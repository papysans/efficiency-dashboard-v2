package efficiencyv2

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
	EfficiencyV2DefaultTeamID     = "default"
)

type EfficiencyV2FusionInputs struct {
	AlgoMin *float64
	KNNMin  *float64
	LLMMin  *float64
	// LLMCalendarMin = LLM 直接估的「同等团队无 AI 自然日历工期」（块3工期维度）。
	// >0 时作为日历基线锚定（绕 work/density 换算）；nil/<=0 回退 fused/density。
	LLMCalendarMin *float64
	Weights        EfficiencyV2BaselineDefaults
	TeamDensity    float64
}

type EfficiencyV2FusionResult struct {
	FusedWorkMin    *float64
	SpreadWorkMin   *float64
	CalendarMin     *float64
	EfficiencyRatio *float64
	EfficiencyLow   *float64
	EfficiencyHigh  *float64
	WorkEfficiency  *float64
	ConfidenceLevel string
	// CalendarSource = "llm_elapsed"（LLM 直接估工期锚定）| "density"（回退 fused/density 派生）。
	CalendarSource string
	OutlierFlag    bool // 派生 = CalendarOutlierFlag || WorkOutlierFlag
	// 按口径拆分异常隔离：日历提效与工作量提效分别判 outlier，避免单口径极端值
	// 把同一 need 另一口径的合理提效一并隐藏（详见 design.md）。
	CalendarOutlierFlag bool
	WorkOutlierFlag     bool
	Reasons             []string
	TeamDensityUsed     *float64
	BaselineSources     []string
}

// EnsureEfficiencyV2FusionWeightSnapshot creates a cold-start fusion weight
// snapshot when no rows exist for the requested team and week.
func EnsureEfficiencyV2FusionWeightSnapshot(db *gorm.DB, teamID string, weekStart time.Time, defaults EfficiencyV2BaselineDefaults) error {
	if teamID == "" {
		teamID = EfficiencyV2DefaultTeamID
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
		teamID = EfficiencyV2DefaultTeamID
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
	cfg = NormalizeEfficiencyV2Config(cfg)
	result := EfficiencyV2FusionResult{}

	type baseline struct {
		name   string
		value  float64
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

	// 基线日历标定：仅缩放"基线日历"这一估计量，把偏大的日历口径拉下来。
	// 实际时间跨度(need.TotalCalendarMin)与 density 语义均不受影响。
	calib := cfg.BaselineCalendarCalibration
	if calib <= 0 {
		calib = 1.0
	}
	// 块3工期维度：日历基线优先用 LLM 直接估的「同等团队无 AI 自然工期(elapsed)」，
	// 它对多人 need 有独立锚定（绕开 work/density 单流换算虚高，见 PRD）。
	// LLM 未给/无效(<=0)时回退 (fused/density)*calib，并打 reason 标低置信。
	// LLM 工期不经 calib（calib 是给 density 换算用的缩放，LLM 已直接是日历量纲）。
	densityCalendar := (fused / density) * calib
	calendar := densityCalendar
	calendarSource := "density"
	if inputs.LLMCalendarMin != nil && *inputs.LLMCalendarMin > 0 {
		calendar = *inputs.LLMCalendarMin
		calendarSource = "llm_elapsed"
	} else {
		result.Reasons = append(result.Reasons, "calendar:fallback_density")
	}
	result.CalendarMin = &calendar
	result.CalendarSource = calendarSource

	if need.TotalCalendarMin > 0 && calendar > 0 {
		ratio := (calendar - need.TotalCalendarMin) / need.TotalCalendarMin
		result.EfficiencyRatio = &ratio

		// 误差带只在密度派生口径下可算（spread 是融合工作量口径的离散度）。
		// LLM 工期是单点估计、无 spread，故 LLM 锚定时不给带（band:llm_no_spread）。
		if calendarSource == "llm_elapsed" {
			result.Reasons = append(result.Reasons, "band:llm_no_spread")
		} else if result.SpreadWorkMin != nil && *result.SpreadWorkMin > 0 {
			// 设计 §Step 6：下界 = baseline 偏低时的 eff = (baseline-spread/2 - actual) / actual
			//              上界 = baseline 偏高时的 eff = (baseline+spread/2 - actual) / actual
			baselineCalendarLow := (fused - *result.SpreadWorkMin/2) / density * calib
			baselineCalendarHigh := (fused + *result.SpreadWorkMin/2) / density * calib
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

	// 异常探测：reason 文本始终 append（诊断保留），但口径 flag 仅当该类别 ∈ exclusion.scope
	// 时才置 true。flag 语义 = "撞了配置范围内的异常类别" = "应从对应口径聚合中隐藏"。
	// 空 scope ⇒ 永不置 flag（全部计入，含极端值）。
	// 按口径拆分：actual_to_baseline→工作量侧；efficiency_ratio→日历侧；loc_rate→两侧都打
	// （LOC 虚高污染算法基线本身，日历与工作量提效都从该脏基线派生）。
	thresh := cfg.ConfidenceThresholds
	if fused > 0 && need.TotalActiveWorkCorrectedMin > 0 {
		actualRatio := need.TotalActiveWorkCorrectedMin / fused
		if actualRatio > thresh.OutlierActualToBaselineMax || (actualRatio > 0 && actualRatio < thresh.OutlierActualToBaselineMin) {
			if efficiencyV2ScopeExcludes(cfg, efficiencyV2ExclusionActualToBaseline) {
				result.WorkOutlierFlag = true
			}
			result.Reasons = append(result.Reasons, fmt.Sprintf("outlier:actual_to_baseline=%.3f", actualRatio))
		}
	}

	// 设计 §10.2.5：calendar 提效比落在极端区间必须可发现（阈值可配，默认 >10.0 或 <-2.0）。
	// 不 clip（§2.3.10），只打日历侧 flag + reason，业务方看聚合数字、UI 标 tag。
	if result.EfficiencyRatio != nil && (*result.EfficiencyRatio > thresh.OutlierEfficiencyRatioMax || *result.EfficiencyRatio < thresh.OutlierEfficiencyRatioMin) {
		if efficiencyV2ScopeExcludes(cfg, efficiencyV2ExclusionEfficiencyRatio) {
			result.CalendarOutlierFlag = true
		}
		result.Reasons = append(result.Reasons, fmt.Sprintf("outlier:efficiency_ratio=%.3f", *result.EfficiencyRatio))
	}

	// LOC 速率物理不可能：实际日历内净写入行数过多(>~7行/分≈一天1w行)，多为机器生成/vendored/锁文件。
	// LOC 虚高污染算法基线 → 日历+工作量两口径都打 flag(从聚合统计剔除)，不 clip、不影响单任务展示。
	if thresh.OutlierLocPerCalendarMinMax > 0 && need.TotalCalendarMin > 0 && need.ChangedLoc > 0 {
		locRate := float64(need.ChangedLoc) / need.TotalCalendarMin
		if locRate > thresh.OutlierLocPerCalendarMinMax {
			if efficiencyV2ScopeExcludes(cfg, efficiencyV2ExclusionLocRate) {
				result.CalendarOutlierFlag = true
				result.WorkOutlierFlag = true
			}
			result.Reasons = append(result.Reasons, fmt.Sprintf("outlier:impossible_loc_rate=%.1f", locRate))
		}
	}

	// 派生：任一口径异常 ⇒ outlier_flag=true（供前端「异常」tag/筛选、原因诊断计数兼容）。
	result.OutlierFlag = result.CalendarOutlierFlag || result.WorkOutlierFlag

	result.ConfidenceLevel = classifyEfficiencyV2Confidence(need, result, len(available), cfg)
	if result.ConfidenceLevel == "" {
		result.ConfidenceLevel = efficiencyV2ConfidenceUnknown
	}

	return result
}

func classifyEfficiencyV2Confidence(need models.Need, result EfficiencyV2FusionResult, baselineCount int, cfg EfficiencyV2Config) string {
	cfg = NormalizeEfficiencyV2Config(cfg)
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
	cfg = NormalizeEfficiencyV2Config(cfg)
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
	need.CalendarOutlierFlag = result.CalendarOutlierFlag
	need.WorkOutlierFlag = result.WorkOutlierFlag
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
	"fusion:", "outlier:", "band:", "calendar:",
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

// efficiencyV2ScopeExcludes 判断给定异常类别是否在配置的排除范围内（=该类别撞线时应置 outlier_flag）。
// cfg 须已 normalize（Exclusion.Scope 已解析）；空 scope ⇒ 任何类别都不排。
func efficiencyV2ScopeExcludes(cfg EfficiencyV2Config, category string) bool {
	for _, c := range cfg.Exclusion.Scope {
		if c == category {
			return true
		}
	}
	return false
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
