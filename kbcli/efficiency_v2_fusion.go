package main

import (
	"fmt"
	"math"
	"strings"
	"time"

	"kanban/core/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	efficiencyV2ConfidenceUnknown = "unknown"
	efficiencyV2DefaultTeamID     = "default"
)

type EfficiencyV2FusionInputs struct {
	AlgoMin   *float64
	KNNMin    *float64
	LLMMin    *float64
	Weights   EfficiencyV2BaselineDefaults
	TeamDensity float64
}

type EfficiencyV2FusionResult struct {
	FusedWorkMin     *float64
	SpreadWorkMin    *float64
	CalendarMin      *float64
	EfficiencyRatio  *float64
	EfficiencyLow    *float64
	EfficiencyHigh   *float64
	WorkEfficiency   *float64
	ConfidenceLevel  string
	OutlierFlag      bool
	Reasons          []string
	TeamDensityUsed  *float64
	BaselineSources  []string
}

// EnsureEfficiencyV2FusionWeightSnapshot creates a cold-start fusion weight
// snapshot when no rows exist for the requested team and week.
func EnsureEfficiencyV2FusionWeightSnapshot(db *gorm.DB, teamID string, weekStart time.Time, defaults EfficiencyV2BaselineDefaults) error {
	if teamID == "" {
		teamID = efficiencyV2DefaultTeamID
	}
	// Find existing rows for this team+week. If any are learned (not cold_start),
	// preserve them — those are real Bayesian updates. If they're all cold_start,
	// delete them so we can refresh with current config (handles yaml tuning).
	var existing []models.BaselineFusionWeight
	if err := db.Where("team_id = ? AND week_start = ?", teamID, weekStart).Find(&existing).Error; err != nil {
		return fmt.Errorf("query fusion weights: %w", err)
	}
	if len(existing) > 0 {
		allColdStart := true
		for _, row := range existing {
			if !row.ColdStartDefault {
				allColdStart = false
				break
			}
		}
		if !allColdStart {
			return nil
		}
		// All existing rows are cold_start; replace them with current config.
		if err := db.Where("team_id = ? AND week_start = ? AND cold_start_default = ?", teamID, weekStart, true).
			Delete(&models.BaselineFusionWeight{}).Error; err != nil {
			return fmt.Errorf("clear stale cold_start fusion weights: %w", err)
		}
	}
	row := models.BaselineFusionWeight{
		TeamId:           teamID,
		SnapshotTs:       time.Now().UTC(),
		WeekStart:        weekStart,
		WeightAlgo:       defaults.WeightAlgo,
		WeightKNN:        defaults.WeightKNN,
		WeightLLM:        defaults.WeightLLM,
		TeamWorkDensity:  defaults.TeamWorkDensity,
		DensitySource:    "cold_start",
		ColdStartDefault: true,
		SampleCount:      0,
		Reason:           "cold_start_default",
		Metadata:         models.ObjectJSON("{}"),
	}
	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "team_id"}, {Name: "snapshot_ts"}},
		DoNothing: true,
	}).Create(&row).Error
}

// LookupEfficiencyV2FusionWeights returns the most recent fusion weights for
// the team. Cold-start defaults are returned when no row exists.
func LookupEfficiencyV2FusionWeights(db *gorm.DB, teamID string, defaults EfficiencyV2BaselineDefaults) (EfficiencyV2BaselineDefaults, float64, string, error) {
	if teamID == "" {
		teamID = efficiencyV2DefaultTeamID
	}
	var row models.BaselineFusionWeight
	err := db.Where("team_id = ?", teamID).Order("snapshot_ts DESC").First(&row).Error
	if err != nil {
		return defaults, defaults.TeamWorkDensity, "cold_start_default_fallback", nil
	}
	weights := EfficiencyV2BaselineDefaults{
		WeightAlgo:      row.WeightAlgo,
		WeightKNN:       row.WeightKNN,
		WeightLLM:       row.WeightLLM,
		TeamWorkDensity: row.TeamWorkDensity,
	}
	source := row.DensitySource
	if row.ColdStartDefault {
		source = "cold_start"
	}
	return weights, row.TeamWorkDensity, source, nil
}

// ComputeEfficiencyV2Fusion fuses available baselines, derives the calendar
// efficiency ratio, computes the confidence level, and flags outliers.
func ComputeEfficiencyV2Fusion(need models.Need, inputs EfficiencyV2FusionInputs, cfg EfficiencyV2Config) EfficiencyV2FusionResult {
	cfg = normalizeEfficiencyV2Config(cfg)
	result := EfficiencyV2FusionResult{}

	type baseline struct {
		name  string
		value float64
		weight float64
	}
	available := make([]baseline, 0, 3)
	if inputs.AlgoMin != nil && *inputs.AlgoMin >= 0 {
		available = append(available, baseline{"algo", *inputs.AlgoMin, inputs.Weights.WeightAlgo})
	}
	if inputs.KNNMin != nil && *inputs.KNNMin >= 0 {
		available = append(available, baseline{"knn", *inputs.KNNMin, inputs.Weights.WeightKNN})
	}
	if inputs.LLMMin != nil && *inputs.LLMMin >= 0 {
		available = append(available, baseline{"llm", *inputs.LLMMin, inputs.Weights.WeightLLM})
	}
	for _, b := range available {
		result.BaselineSources = append(result.BaselineSources, b.name)
	}

	if len(available) == 0 {
		result.ConfidenceLevel = efficiencyV2ConfidenceUnknown
		result.Reasons = append(result.Reasons, "fusion:no_baselines")
		return result
	}

	// Normalise weights for available baselines only.
	totalWeight := 0.0
	for _, b := range available {
		w := b.weight
		if w <= 0 {
			w = 1.0 / float64(len(available))
		}
		totalWeight += w
	}
	if totalWeight <= 0 {
		totalWeight = float64(len(available))
	}
	var fused float64
	values := make([]float64, 0, len(available))
	for _, b := range available {
		w := b.weight
		if w <= 0 {
			w = 1.0 / float64(len(available))
		}
		fused += b.value * (w / totalWeight)
		values = append(values, b.value)
	}
	result.FusedWorkMin = &fused

	if len(values) >= 2 {
		spread := efficiencyV2MaxMinSpread(values)
		result.SpreadWorkMin = &spread
	} else {
		zero := 0.0
		result.SpreadWorkMin = &zero
		result.Reasons = append(result.Reasons, "fusion:single_baseline")
	}

	density := inputs.TeamDensity
	if density <= 0 {
		density = cfg.BaselineDefaults.TeamWorkDensity
	}
	if density <= 0 {
		density = 0.25
	}
	result.TeamDensityUsed = &density

	calendar := fused / density
	result.CalendarMin = &calendar

	if need.TotalCalendarMin > 0 && calendar > 0 {
		ratio := (calendar - need.TotalCalendarMin) / need.TotalCalendarMin
		result.EfficiencyRatio = &ratio

		if result.SpreadWorkMin != nil && *result.SpreadWorkMin > 0 {
			// 设计 §Step 6：下界 = baseline 偏低时的 eff = (baseline-spread/2 - actual) / actual
			//              上界 = baseline 偏高时的 eff = (baseline+spread/2 - actual) / actual
			baselineCalendarLow := (fused - *result.SpreadWorkMin/2) / density
			baselineCalendarHigh := (fused + *result.SpreadWorkMin/2) / density
			if baselineCalendarLow > 0 {
				low := (baselineCalendarLow - need.TotalCalendarMin) / need.TotalCalendarMin
				result.EfficiencyLow = &low
			} else {
				result.Reasons = append(result.Reasons, "band:spread_exceeds_fused")
			}
			if baselineCalendarHigh > 0 {
				high := (baselineCalendarHigh - need.TotalCalendarMin) / need.TotalCalendarMin
				result.EfficiencyHigh = &high
			}
		}
	} else {
		result.Reasons = append(result.Reasons, "fusion:missing_actual_calendar")
	}

	if need.TotalActiveWorkCorrectedMin > 0 && fused > 0 {
		wer := (fused - need.TotalActiveWorkCorrectedMin) / need.TotalActiveWorkCorrectedMin
		result.WorkEfficiency = &wer
	}

	thresh := cfg.ConfidenceThresholds
	if fused > 0 && need.TotalActiveWorkCorrectedMin > 0 {
		actualRatio := need.TotalActiveWorkCorrectedMin / fused
		if actualRatio > thresh.OutlierActualToBaselineMax || (actualRatio > 0 && actualRatio < thresh.OutlierActualToBaselineMin) {
			result.OutlierFlag = true
			result.Reasons = append(result.Reasons, fmt.Sprintf("outlier:actual_to_baseline=%.3f", actualRatio))
		}
	}

	// 设计 §10.2.5：calendar 提效比落在极端区间（>0.9 或 <-0.5）必须可发现。
	// 不 clip（§2.3.10），只打 outlier_flag + reason，业务方看聚合数字、UI 标红。
	if result.EfficiencyRatio != nil && (*result.EfficiencyRatio > 0.9 || *result.EfficiencyRatio < -0.5) {
		if !result.OutlierFlag {
			result.OutlierFlag = true
		}
		result.Reasons = append(result.Reasons, fmt.Sprintf("outlier:efficiency_ratio=%.3f", *result.EfficiencyRatio))
	}

	result.ConfidenceLevel = classifyEfficiencyV2Confidence(need, result, len(available), cfg)
	if result.ConfidenceLevel == "" {
		result.ConfidenceLevel = efficiencyV2ConfidenceUnknown
	}

	return result
}

func classifyEfficiencyV2Confidence(need models.Need, result EfficiencyV2FusionResult, baselineCount int, cfg EfficiencyV2Config) string {
	cfg = normalizeEfficiencyV2Config(cfg)
	if baselineCount == 0 {
		return efficiencyV2ConfidenceUnknown
	}
	if baselineCount == 1 {
		return efficiencyV2StageConfidenceLow
	}
	thresh := cfg.ConfidenceThresholds
	if result.FusedWorkMin == nil || *result.FusedWorkMin <= 0 {
		return efficiencyV2StageConfidenceLow
	}
	spreadRatio := 0.0
	if result.SpreadWorkMin != nil {
		spreadRatio = *result.SpreadWorkMin / *result.FusedWorkMin
	}
	if spreadRatio > thresh.MediumSpreadRatioMax {
		return efficiencyV2StageConfidenceLow
	}
	if need.AICodeRatio != nil && *need.AICodeRatio < thresh.AICodeRatioMin {
		return efficiencyV2StageConfidenceLow
	}
	if need.UncoveredWorkRatio != nil && *need.UncoveredWorkRatio > thresh.UncoveredWorkRatioMax {
		return efficiencyV2StageConfidenceLow
	}
	if need.Silica != nil && *need.Silica < thresh.SilicaSignalMin {
		return efficiencyV2StageConfidenceLow
	}
	// 设计 line 246: feature_dependency_risk = "low" 意为 risk 低 = 好（pass）。
	// 之前代码反了（risk=low 时降级 confidence）；且 FeatureDependencyRisk 当前
	// 未被任何代码填充（churn/revert/post_gen_delete 计算待实现），保留检查
	// 但改成正确语义：风险 high 时降级。
	if need.FeatureDependencyRisk == "high" {
		return efficiencyV2StageConfidenceLow
	}
	if spreadRatio > thresh.HighSpreadRatioMax {
		return efficiencyV2StageConfidenceMedium
	}
	return efficiencyV2StageConfidenceHigh
}

func PersistEfficiencyV2FusionOnNeed(need *models.Need, result EfficiencyV2FusionResult, cfg EfficiencyV2Config) {
	cfg = normalizeEfficiencyV2Config(cfg)
	need.BaselineFusedWorkMin = result.FusedWorkMin
	need.BaselineSpreadWorkMin = result.SpreadWorkMin
	need.BaselineCalendarMin = result.CalendarMin
	need.TeamWorkDensityUsed = result.TeamDensityUsed
	need.TeamProfileUsed = cfg.TeamProfile
	need.EfficiencyRatio = result.EfficiencyRatio
	need.EfficiencyLowerBand = result.EfficiencyLow
	need.EfficiencyUpperBand = result.EfficiencyHigh
	need.WorkEfficiencyRatio = result.WorkEfficiency
	need.ConfidenceLevel = result.ConfidenceLevel
	need.OutlierFlag = result.OutlierFlag
	// Preserve the boundary-side reason (set by Need resolver), append fusion
	// reasons fresh each run. Strip prior fusion reasons so reruns stay idempotent.
	boundaryReason := stripEfficiencyV2FusionReasons(need.Reason)
	parts := []string{}
	if boundaryReason != "" {
		parts = append(parts, boundaryReason)
	}
	parts = append(parts, result.Reasons...)
	need.Reason = strings.TrimSpace(strings.Join(parts, "; "))
}

var efficiencyV2FusionReasonPrefixes = []string{
	"fusion:", "outlier:", "band:",
}

func stripEfficiencyV2FusionReasons(existing string) string {
	if existing == "" {
		return ""
	}
	parts := strings.Split(existing, ";")
	kept := parts[:0]
	for _, raw := range parts {
		p := strings.TrimSpace(raw)
		if p == "" {
			continue
		}
		drop := false
		for _, prefix := range efficiencyV2FusionReasonPrefixes {
			if strings.HasPrefix(p, prefix) {
				drop = true
				break
			}
		}
		if !drop {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, "; ")
}

func efficiencyV2MaxMinSpread(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	lo, hi := values[0], values[0]
	for _, v := range values[1:] {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	return hi - lo
}

// ComputeEfficiencyV2WeeklyHoldOutError is a placeholder for the per-baseline
// hold-out MAE snapshot. A real implementation would partition the anchor set
// and compute MAE per baseline; until that lands, this returns NaN so callers
// can persist NULL rather than misleading numbers. Callers MUST guard against
// NaN before writing to baseline_fusion_weights.HoldOutMAE* fields.
func ComputeEfficiencyV2WeeklyHoldOutError(anchors []EfficiencyV2KNNAnchor) (algo, knn, llm float64) {
	_ = anchors // intentionally unused until anchor partitioning is implemented
	return math.NaN(), math.NaN(), math.NaN()
}
