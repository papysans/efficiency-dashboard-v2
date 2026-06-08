package efficiencyv2

import (
	"encoding/json"
	"fmt"
	"kanban/kbcli/internal/estimator"
	"sort"
	"strings"
	"time"

	"kanban/core/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	efficiencyV2SignalLow     = "low"
	efficiencyV2SignalOK      = "ok"
	efficiencyV2SignalUnknown = "unknown"
)

var efficiencyV2DefaultLinesPerMinute = float64(estimator.DefaultTraditionalDevLinesPerDay) / 480.0

// AggregateAndUpsertEfficiencyV2NeedActuals computes and persists actual-time,
// stage totals, uncovered-commit, and quality/confidence signal fields for the
// supplied Needs. It reads associated session_stage_metrics and commits from
// the database.
func AggregateAndUpsertEfficiencyV2NeedActuals(db *gorm.DB, needs []models.Need, cfg EfficiencyV2Config, algo estimator.EstimateConfig) ([]models.Need, error) {
	if len(needs) == 0 {
		return needs, nil
	}
	sessionIDs := make(map[string]bool)
	commitIDs := make(map[string]bool)
	for _, need := range needs {
		for _, id := range EfficiencyV2StringsFromJSON(need.SessionIds) {
			sessionIDs[id] = true
		}
		for _, id := range EfficiencyV2StringsFromJSON(need.CommitIds) {
			commitIDs[id] = true
		}
	}

	var metrics []models.SessionStageMetric
	if len(sessionIDs) > 0 {
		keys := efficiencyV2SortedMapKeys(sessionIDs)
		if err := db.Where("session_id IN ?", keys).Find(&metrics).Error; err != nil {
			return nil, fmt.Errorf("load stage metrics: %w", err)
		}
	}
	var commits []models.Commit
	if len(commitIDs) > 0 {
		keys := efficiencyV2SortedMapKeys(commitIDs)
		if err := db.Where("commit_id IN ?", keys).Find(&commits).Error; err != nil {
			return nil, fmt.Errorf("load commits: %w", err)
		}
	}

	updated := AggregateEfficiencyV2NeedActuals(needs, metrics, commits, cfg, algo)
	if err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "need_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"total_session_active_person_min",
			"estimate_uncovered_human_min",
			"total_active_work_corrected_min",
			"total_wall_min",
			"total_calendar_min",
			"total_think_min",
			"total_exec_min",
			"total_verify_min",
			"total_other_min",
			"commit_count",
			"total_loc_net",
			"total_files_touched",
			"ai_covered_loc",
			"uncovered_commit_ids",
			"uncovered_loc",
			"uncovered_human_min",
			"uncovered_work_ratio",
			"ai_code_ratio",
			"silica",
			"revert_count",
			"revert_rate",
			"quality_signals",
			"confidence_signals",
			"silica_signal",
			"ai_code_ratio_signal",
			"uncovered_work_signal",
			"coverage_eligible",
			"updated_at",
		}),
	}).CreateInBatches(&updated, 500).Error; err != nil {
		return nil, fmt.Errorf("upsert need actuals: %w", err)
	}
	return updated, nil
}

// AggregateEfficiencyV2NeedActuals computes Need-level actual time, stage
// totals, uncovered-commit correction, and quality/confidence signals from
// session stage metrics and commit membership. It returns updated copies of
// the input Needs.
func AggregateEfficiencyV2NeedActuals(needs []models.Need, metrics []models.SessionStageMetric, commits []models.Commit, cfg EfficiencyV2Config, algo estimator.EstimateConfig) []models.Need {
	cfg = NormalizeEfficiencyV2Config(cfg)
	algo = NormalizeEfficiencyV2AlgoConfig(algo)

	metricsBySession := make(map[string]models.SessionStageMetric, len(metrics))
	for _, m := range metrics {
		metricsBySession[m.SessionId] = m
	}
	commitsByID := make(map[string]models.Commit, len(commits))
	for _, c := range commits {
		commitsByID[c.CommitId] = c
	}

	updated := make([]models.Need, 0, len(needs))
	for _, need := range needs {
		updated = append(updated, aggregateOneEfficiencyV2Need(need, metricsBySession, commitsByID, cfg, algo))
	}
	return updated
}

func aggregateOneEfficiencyV2Need(need models.Need, metricsBySession map[string]models.SessionStageMetric, commitsByID map[string]models.Commit, cfg EfficiencyV2Config, algo estimator.EstimateConfig) models.Need {
	sessionIDs := EfficiencyV2StringsFromJSON(need.SessionIds)
	commitIDs := EfficiencyV2StringsFromJSON(need.CommitIds)

	sessions := make([]models.SessionStageMetric, 0, len(sessionIDs))
	for _, id := range sessionIDs {
		if m, ok := metricsBySession[id]; ok {
			sessions = append(sessions, m)
		}
	}
	needCommits := make([]models.Commit, 0, len(commitIDs))
	for _, id := range commitIDs {
		if c, ok := commitsByID[id]; ok {
			needCommits = append(needCommits, c)
		}
	}

	aggregateEfficiencyV2NeedTime(&need, sessions, cfg)
	aggregateEfficiencyV2NeedCommits(&need, sessions, needCommits, cfg, algo)
	aggregateEfficiencyV2NeedSignals(&need, needCommits, cfg)

	idleThreshold := efficiencyV2IdleThreshold(cfg)
	need.TotalCalendarMin = computeEfficiencyV2DevCalendarMinutes(need.DevStartTs, need.DevEndTs, sessions, needCommits, idleThreshold)

	// 提效比依赖"真实可测的开发日历"。无真实时间信号的 Need（无会话、单 commit/零跨度）
	// total_calendar_min<=0，标为数据受限、排除出提效计算——不用 commit 估时硬填，
	// 经仿真验证硬填 30min 对上千分钟 baseline 会产出 40x~700x 的离谱比值。
	// 多 commit 时间跨度 (>0) 仍保留，是有意义的实际耗时。
	if need.CoverageEligible && need.TotalCalendarMin <= 0 {
		need.CoverageEligible = false
	}
	return need
}

func efficiencyV2IdleThreshold(cfg EfficiencyV2Config) time.Duration {
	idle := time.Duration(cfg.IdleThresholdDays) * 24 * time.Hour
	if idle <= 0 {
		idle = 3 * 24 * time.Hour
	}
	return idle
}

// computeEfficiencyV2DevCalendarMinutes returns the dev-period calendar span
// (dev_end_ts - dev_start_ts) minus any idle gaps longer than idleThreshold.
// Gaps are detected from the union of session edges and commit times that fall
// within [dev_start_ts, dev_end_ts]. This matches the design doc's
// "T_actual_calendar = dev_end_ts - dev_start_ts - 长搁置" — calendar time
// includes eat/sleep/weekends, only > 3-day stalls are excluded.
func computeEfficiencyV2DevCalendarMinutes(devStart, devEnd *time.Time, sessions []models.SessionStageMetric, commits []models.Commit, idleThreshold time.Duration) float64 {
	start, end := devStart, devEnd
	if start == nil || end == nil {
		// Fallback for fixtures / Needs without boundary resolution: derive from
		// first/last activity timestamp. Production needs always have dev_start/end.
		var ts []time.Time
		for _, s := range sessions {
			if s.SessionStartTs != nil {
				ts = append(ts, *s.SessionStartTs)
			}
			if s.SessionEndTs != nil {
				ts = append(ts, *s.SessionEndTs)
			}
		}
		for _, c := range commits {
			ts = append(ts, c.CommitTime)
		}
		if len(ts) == 0 {
			return 0
		}
		sort.Slice(ts, func(i, j int) bool { return ts[i].Before(ts[j]) })
		if start == nil {
			s := ts[0]
			start = &s
		}
		if end == nil {
			e := ts[len(ts)-1]
			end = &e
		}
	}
	span := end.Sub(*start).Minutes()
	if span <= 0 {
		return 0
	}
	points := []time.Time{*start, *end}
	inWindow := func(t time.Time) bool {
		return !t.Before(*start) && !t.After(*end)
	}
	for _, s := range sessions {
		if s.SessionStartTs != nil && inWindow(*s.SessionStartTs) {
			points = append(points, *s.SessionStartTs)
		}
		if s.SessionEndTs != nil && inWindow(*s.SessionEndTs) {
			points = append(points, *s.SessionEndTs)
		}
	}
	for _, c := range commits {
		if inWindow(c.CommitTime) {
			points = append(points, c.CommitTime)
		}
	}
	sort.Slice(points, func(i, j int) bool { return points[i].Before(points[j]) })
	for i := 1; i < len(points); i++ {
		gap := points[i].Sub(points[i-1])
		if gap > idleThreshold {
			span -= gap.Minutes()
		}
	}
	if span < 0 {
		return 0
	}
	return span
}

func aggregateEfficiencyV2NeedTime(need *models.Need, sessions []models.SessionStageMetric, cfg EfficiencyV2Config) {
	var personMin, think, exec, verify, other float64
	intervals := make([][2]time.Time, 0, len(sessions))
	for _, s := range sessions {
		personMin += s.TotalActiveMin
		think += s.ThinkActiveMin
		exec += s.ExecutionActiveMin
		verify += s.VerificationActiveMin
		other += s.OtherActiveMin
		if s.SessionStartTs == nil || s.SessionEndTs == nil {
			continue
		}
		intervals = append(intervals, [2]time.Time{*s.SessionStartTs, *s.SessionEndTs})
	}

	need.TotalSessionActivePersonMin = personMin
	need.ThinkActiveMin = think
	need.ExecutionActiveMin = exec
	need.VerificationActiveMin = verify
	need.OtherActiveMin = other

	merged := mergeEfficiencyV2Intervals(intervals)
	wallMin := 0.0
	for _, iv := range merged {
		wallMin += iv[1].Sub(iv[0]).Minutes()
	}
	need.TotalWallMin = wallMin
	// total_calendar_min is set in aggregateOneEfficiencyV2Need after commits
	// are aggregated, using dev_start_ts/dev_end_ts per design.
}

func aggregateEfficiencyV2NeedCommits(need *models.Need, sessions []models.SessionStageMetric, commits []models.Commit, cfg EfficiencyV2Config, algo estimator.EstimateConfig) {
	preMargin := time.Duration(cfg.UncoveredCommit.PreMarginMinutes) * time.Minute
	postMargin := time.Duration(cfg.UncoveredCommit.PostMarginMinutes) * time.Minute

	windows := make([][2]time.Time, 0, len(sessions))
	for _, s := range sessions {
		if s.SessionStartTs == nil || s.SessionEndTs == nil {
			continue
		}
		windows = append(windows, [2]time.Time{
			s.SessionStartTs.Add(-preMargin),
			s.SessionEndTs.Add(postMargin),
		})
	}

	var changedLoc, aiCoveredLoc, uncoveredLoc int64
	var uncoveredCommitIDs []string
	var revertCount int64
	for _, commit := range commits {
		changedLoc += int64(commit.DiffLines)
		if isEfficiencyV2RevertCommit(commit.Comment) {
			revertCount++
		}
		// covered-rule: temporal-only. Commits lack a touched_files list in
		// models.Commit; once that arrives the rule should also require file
		// overlap with a session's touched files.
		if isEfficiencyV2CommitCovered(commit, windows) {
			aiCoveredLoc += int64(commit.DiffLines)
		} else {
			uncoveredLoc += int64(commit.DiffLines)
			uncoveredCommitIDs = append(uncoveredCommitIDs, commit.CommitId)
		}
	}
	sort.Strings(uncoveredCommitIDs)

	need.CommitCount = int64(len(commits))
	need.ChangedLoc = changedLoc
	need.AICoveredLoc = aiCoveredLoc
	need.UncoveredLoc = uncoveredLoc
	need.UncoveredCommitIds = EfficiencyV2StringJSON(uncoveredCommitIDs)

	uncoveredMin := estimateEfficiencyV2UncoveredHumanMinutes(uncoveredLoc, algo)
	need.UncoveredHumanMin = uncoveredMin
	need.EstimateUncoveredHumanMin = uncoveredMin
	need.TotalActiveWorkCorrectedMin = need.TotalSessionActivePersonMin + uncoveredMin

	need.RevertCount = revertCount
	if len(commits) > 0 {
		rate := float64(revertCount) / float64(len(commits))
		need.RevertRate = &rate
	} else {
		need.RevertRate = nil
	}

	// FileCount mirrors len(need.TouchedFiles); the touched-files list itself is
	// owned by Need boundary resolution (efficiency_v2_need_boundary.go).
	files := EfficiencyV2StringsFromJSON(need.TouchedFiles)
	need.FileCount = int64(len(files))
}

func aggregateEfficiencyV2NeedSignals(need *models.Need, commits []models.Commit, cfg EfficiencyV2Config) {
	reasons := make([]string, 0, 4)
	// Note v2 first cut: covered-rule is temporal-only. Surfaced so downstream
	// consumers can see the rule even when no signal is degraded.
	reasons = append(reasons, "covered_rule=temporal_only")

	switch {
	case len(commits) == 0:
		need.AICodeRatio = nil
		need.Silica = nil
		need.UncoveredWorkRatio = nil
		need.AICodeRatioSignal = efficiencyV2SignalUnknown
		need.SilicaSignal = efficiencyV2SignalUnknown
		need.UncoveredWorkSignal = efficiencyV2SignalUnknown
		reasons = append(reasons, "no_commits")
	case need.ChangedLoc <= 0:
		need.AICodeRatio = nil
		need.Silica = nil
		need.UncoveredWorkRatio = nil
		need.AICodeRatioSignal = efficiencyV2SignalUnknown
		need.SilicaSignal = efficiencyV2SignalUnknown
		need.UncoveredWorkSignal = efficiencyV2SignalUnknown
		reasons = append(reasons, "zero_loc_commits")
	default:
		aiRatio := float64(need.AICoveredLoc) / float64(need.ChangedLoc)
		need.AICodeRatio = &aiRatio
		uncovRatio := float64(need.UncoveredLoc) / float64(need.ChangedLoc)
		need.UncoveredWorkRatio = &uncovRatio

		var weightedSilica, totalWeight float64
		anyPositiveSilica := false
		for _, commit := range commits {
			weight := float64(commit.DiffLines)
			if weight <= 0 {
				weight = 1
			}
			weightedSilica += commit.Silica * weight
			totalWeight += weight
			if commit.Silica > 0 {
				anyPositiveSilica = true
			}
		}
		// 当所有 commit.silica = 0 时，是 silica 计算未跑（cleaned seed 路径没有
		// raw diff fingerprint）而不是 AI 写了 0 行。按"数据缺失 → unknown"处理，
		// 避免无差别拉低所有 need 的置信度。
		if totalWeight > 0 && anyPositiveSilica {
			silica := weightedSilica / totalWeight
			need.Silica = &silica
		} else {
			need.Silica = nil
		}

		thresh := cfg.ConfidenceThresholds
		need.AICodeRatioSignal = efficiencyV2SignalOK
		if aiRatio < thresh.AICodeRatioMin {
			need.AICodeRatioSignal = efficiencyV2SignalLow
			reasons = append(reasons, fmt.Sprintf("ai_code_ratio=%.3f<%.3f", aiRatio, thresh.AICodeRatioMin))
		}
		need.UncoveredWorkSignal = efficiencyV2SignalOK
		if uncovRatio > thresh.UncoveredWorkRatioMax {
			need.UncoveredWorkSignal = efficiencyV2SignalLow
			reasons = append(reasons, fmt.Sprintf("uncovered_work_ratio=%.3f>%.3f", uncovRatio, thresh.UncoveredWorkRatioMax))
		}
		if need.Silica != nil {
			need.SilicaSignal = efficiencyV2SignalOK
			if *need.Silica < thresh.SilicaSignalMin {
				need.SilicaSignal = efficiencyV2SignalLow
				reasons = append(reasons, fmt.Sprintf("silica=%.3f<%.3f", *need.Silica, thresh.SilicaSignalMin))
			}
		} else {
			need.SilicaSignal = efficiencyV2SignalUnknown
		}
	}

	signals := map[string]interface{}{
		"ai_code_ratio":          efficiencyV2FloatOrNil(need.AICodeRatio),
		"silica":                 efficiencyV2FloatOrNil(need.Silica),
		"uncovered_work_ratio":   efficiencyV2FloatOrNil(need.UncoveredWorkRatio),
		"revert_count":           need.RevertCount,
		"revert_rate":            efficiencyV2FloatOrNil(need.RevertRate),
		"commit_count":           need.CommitCount,
		"uncovered_commit_count": len(EfficiencyV2StringsFromJSON(need.UncoveredCommitIds)),
		"total_loc":              need.ChangedLoc,
		"uncovered_loc":          need.UncoveredLoc,
		"ai_covered_loc":         need.AICoveredLoc,
		"reason":                 strings.Join(reasons, "; "),
	}
	confSignals := map[string]interface{}{
		"silica_signal":         need.SilicaSignal,
		"ai_code_ratio_signal":  need.AICodeRatioSignal,
		"uncovered_work_signal": need.UncoveredWorkSignal,
	}

	need.QualitySignals = efficiencyV2ObjectJSONAny(signals)
	need.ConfidenceSignals = efficiencyV2ObjectJSONAny(confSignals)
}

func isEfficiencyV2CommitCovered(commit models.Commit, windows [][2]time.Time) bool {
	if len(windows) == 0 {
		return false
	}
	for _, w := range windows {
		if !commit.CommitTime.Before(w[0]) && !commit.CommitTime.After(w[1]) {
			return true
		}
	}
	return false
}

func isEfficiencyV2RevertCommit(comment string) bool {
	normalized := strings.TrimSpace(strings.ToLower(comment))
	if normalized == "" {
		return false
	}
	if strings.HasPrefix(normalized, "revert ") ||
		strings.HasPrefix(normalized, "revert:") ||
		strings.HasPrefix(normalized, "revert\"") {
		return true
	}
	if strings.Contains(normalized, "this reverts commit") {
		return true
	}
	// Chinese variants: 回滚 / 撤销 / 还原
	original := strings.TrimSpace(comment)
	for _, prefix := range []string{"回滚", "撤销", "还原"} {
		if strings.HasPrefix(original, prefix) {
			return true
		}
	}
	return false
}

func estimateEfficiencyV2UncoveredHumanMinutes(loc int64, algo estimator.EstimateConfig) float64 {
	if loc <= 0 {
		return 0
	}
	rate := algo.CommitLinePerMinutes
	if rate <= 0 {
		rate = efficiencyV2DefaultLinesPerMinute
	}
	minutes := float64(loc) / rate
	if minutes < algo.MinMinutes {
		minutes = algo.MinMinutes
	}
	return minutes
}

func mergeEfficiencyV2Intervals(intervals [][2]time.Time) [][2]time.Time {
	if len(intervals) == 0 {
		return nil
	}
	sorted := make([][2]time.Time, len(intervals))
	copy(sorted, intervals)
	sort.SliceStable(sorted, func(i, j int) bool {
		if !sorted[i][0].Equal(sorted[j][0]) {
			return sorted[i][0].Before(sorted[j][0])
		}
		return sorted[i][1].Before(sorted[j][1])
	})
	merged := [][2]time.Time{sorted[0]}
	for _, iv := range sorted[1:] {
		last := &merged[len(merged)-1]
		if !iv[0].After(last[1]) {
			if iv[1].After(last[1]) {
				last[1] = iv[1]
			}
			continue
		}
		merged = append(merged, iv)
	}
	return merged
}

func NormalizeEfficiencyV2AlgoConfig(algo estimator.EstimateConfig) estimator.EstimateConfig {
	if algo.CommitMinutesPerLine > 0 {
		algo.CommitLinePerMinutes = 1 / algo.CommitMinutesPerLine
	} else if algo.CommitLinePerMinutes <= 0 {
		algo.CommitLinePerMinutes = efficiencyV2DefaultLinesPerMinute
		algo.CommitMinutesPerLine = 1 / algo.CommitLinePerMinutes
	} else {
		algo.CommitMinutesPerLine = 1 / algo.CommitLinePerMinutes
	}
	if algo.MinMinutes < 0 {
		algo.MinMinutes = 0
	}
	return algo
}

func efficiencyV2FloatOrNil(value *float64) interface{} {
	if value == nil {
		return nil
	}
	return *value
}

// efficiencyV2ObjectJSONAny marshals a heterogeneous map for jsonb storage. It
// complements efficiencyV2ObjectJSON (which is specialised for int64 counts).
func efficiencyV2ObjectJSONAny(payload map[string]interface{}) models.ObjectJSON {
	data, err := json.Marshal(payload)
	if err != nil {
		return models.ObjectJSON("{}")
	}
	return models.ObjectJSON(data)
}
