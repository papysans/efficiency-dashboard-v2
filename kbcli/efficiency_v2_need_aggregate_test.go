package main

import (
	"encoding/json"
	"kanban/kbcli/internal/estimator"
	"math"
	"strings"
	"testing"
	"time"

	"kanban/core/models"
)

func TestAggregateEfficiencyV2NeedActuals_PersonAndWallAndCalendar(t *testing.T) {
	base := time.Date(2026, 5, 18, 9, 0, 0, 0, time.UTC)

	// Two contributors with overlapping sessions on the same Need.
	sessionA := efficiencyV2AggTestMetric("s-a", "u-alice", base, base.Add(60*time.Minute), 60)
	sessionA.ThinkActiveMin = 20
	sessionA.ExecutionActiveMin = 30
	sessionA.VerificationActiveMin = 10
	sessionB := efficiencyV2AggTestMetric("s-b", "u-bob", base.Add(30*time.Minute), base.Add(90*time.Minute), 60)
	sessionB.ThinkActiveMin = 15
	sessionB.ExecutionActiveMin = 35
	sessionB.VerificationActiveMin = 10

	need := efficiencyV2AggTestNeed("need-1", efficiencyV2BoundaryBranch, efficiencyV2ConfidenceHigh, "u-alice", []string{"s-a", "s-b"}, nil)

	cfg := EfficiencyV2Config{}
	algo := estimator.EstimateConfig{}
	updated := AggregateEfficiencyV2NeedActuals([]models.Need{need}, []models.SessionStageMetric{sessionA, sessionB}, nil, cfg, algo)
	if len(updated) != 1 {
		t.Fatalf("need count = %d, want 1", len(updated))
	}
	got := updated[0]
	if got.TotalSessionActivePersonMin != 120 {
		t.Fatalf("person_min = %.2f, want 120 (60+60)", got.TotalSessionActivePersonMin)
	}
	if got.TotalWallMin != 90 {
		t.Fatalf("wall_min = %.2f, want 90 (union of [0,60]+[30,90])", got.TotalWallMin)
	}
	if got.TotalCalendarMin != 90 {
		t.Fatalf("calendar_min = %.2f, want 90", got.TotalCalendarMin)
	}
	if got.ThinkActiveMin != 35 || got.ExecutionActiveMin != 65 || got.VerificationActiveMin != 20 {
		t.Fatalf("stage totals: think=%.2f exec=%.2f verify=%.2f, want 35/65/20", got.ThinkActiveMin, got.ExecutionActiveMin, got.VerificationActiveMin)
	}
	if got.TotalActiveWorkCorrectedMin != 120 {
		t.Fatalf("active_work_corrected_min = %.2f, want 120 (no uncovered)", got.TotalActiveWorkCorrectedMin)
	}
}

func TestAggregateEfficiencyV2NeedActuals_ExcludesLongIdleGap(t *testing.T) {
	base := time.Date(2026, 5, 18, 9, 0, 0, 0, time.UTC)

	// First session day 1; second session 5 days later. Gap > 3 day default threshold.
	sessionA := efficiencyV2AggTestMetric("s-a", "u-alice", base, base.Add(60*time.Minute), 60)
	sessionB := efficiencyV2AggTestMetric("s-b", "u-alice", base.Add(5*24*time.Hour), base.Add(5*24*time.Hour+60*time.Minute), 60)

	need := efficiencyV2AggTestNeed("need-idle", efficiencyV2BoundaryBranch, efficiencyV2ConfidenceHigh, "u-alice", []string{"s-a", "s-b"}, nil)

	cfg := EfficiencyV2Config{}
	algo := estimator.EstimateConfig{}
	updated := AggregateEfficiencyV2NeedActuals([]models.Need{need}, []models.SessionStageMetric{sessionA, sessionB}, nil, cfg, algo)
	got := updated[0]

	// total_calendar_min should be only the union of two active windows (120 min) because the gap is excluded.
	if got.TotalCalendarMin != 120 {
		t.Fatalf("calendar_min = %.2f, want 120 (excludes long idle gap)", got.TotalCalendarMin)
	}
	if got.TotalWallMin != 120 {
		t.Fatalf("wall_min = %.2f, want 120 (no overlap)", got.TotalWallMin)
	}
}

func TestAggregateEfficiencyV2NeedActuals_IncludesShortGapInCalendar(t *testing.T) {
	base := time.Date(2026, 5, 18, 9, 0, 0, 0, time.UTC)

	// Two sessions in the same day with a 30-minute gap (well below threshold).
	sessionA := efficiencyV2AggTestMetric("s-a", "u-alice", base, base.Add(60*time.Minute), 60)
	sessionB := efficiencyV2AggTestMetric("s-b", "u-alice", base.Add(90*time.Minute), base.Add(150*time.Minute), 60)

	need := efficiencyV2AggTestNeed("need-short-gap", efficiencyV2BoundaryBranch, efficiencyV2ConfidenceHigh, "u-alice", []string{"s-a", "s-b"}, nil)

	cfg := EfficiencyV2Config{}
	algo := estimator.EstimateConfig{}
	updated := AggregateEfficiencyV2NeedActuals([]models.Need{need}, []models.SessionStageMetric{sessionA, sessionB}, nil, cfg, algo)
	got := updated[0]

	// calendar = full span 150 min (short gap kept)
	if got.TotalCalendarMin != 150 {
		t.Fatalf("calendar_min = %.2f, want 150 (short gap kept)", got.TotalCalendarMin)
	}
}

func TestAggregateEfficiencyV2NeedActuals_UncoveredCommitsByTemporal(t *testing.T) {
	base := time.Date(2026, 5, 18, 9, 0, 0, 0, time.UTC)

	session := efficiencyV2AggTestMetric("s-1", "u-alice", base, base.Add(60*time.Minute), 60)

	// Covered commit: within session end + 60min post-margin
	coveredCommit := models.Commit{
		CommitId:   "c-covered",
		UserId:     "u-alice",
		RepoAddr:   "repo",
		RepoBranch: "feature/x",
		CommitTime: base.Add(90 * time.Minute), // 30 min after session end
		DiffLines:  100,
		Silica:     0.8,
		Comment:    "feat: add x",
	}
	// Uncovered commit: > 60 min after session end
	uncoveredCommit := models.Commit{
		CommitId:   "c-uncovered",
		UserId:     "u-alice",
		RepoAddr:   "repo",
		RepoBranch: "feature/x",
		CommitTime: base.Add(5 * time.Hour), // far outside post-margin
		DiffLines:  60,
		Silica:     0,
		Comment:    "manual: hotfix",
	}

	need := efficiencyV2AggTestNeed("need-cov", efficiencyV2BoundaryBranch, efficiencyV2ConfidenceHigh, "u-alice", []string{"s-1"}, []string{"c-covered", "c-uncovered"})

	cfg := EfficiencyV2Config{}
	algo := estimator.EstimateConfig{LinesPerMinutes: 2, CommitLinePerMinutes: 100.0 / 480.0, MinMinutes: 5}
	updated := AggregateEfficiencyV2NeedActuals([]models.Need{need}, []models.SessionStageMetric{session}, []models.Commit{coveredCommit, uncoveredCommit}, cfg, algo)
	got := updated[0]

	if got.CommitCount != 2 {
		t.Fatalf("commit_count = %d, want 2", got.CommitCount)
	}
	if got.ChangedLoc != 160 {
		t.Fatalf("total_loc_net = %d, want 160", got.ChangedLoc)
	}
	if got.AICoveredLoc != 100 {
		t.Fatalf("ai_covered_loc = %d, want 100", got.AICoveredLoc)
	}
	if got.UncoveredLoc != 60 {
		t.Fatalf("uncovered_loc = %d, want 60", got.UncoveredLoc)
	}
	uncoveredIDs := efficiencyV2StringsFromJSON(got.UncoveredCommitIds)
	if len(uncoveredIDs) != 1 || uncoveredIDs[0] != "c-uncovered" {
		t.Fatalf("uncovered_commit_ids = %v, want [c-uncovered]", uncoveredIDs)
	}
	// uncovered_human_min = 60 LOC / commit_line_per_minutes ≈ 60 / 0.2083 ≈ 288 min
	wantUncoveredMin := 60.0 / (100.0 / 480.0)
	if math.Abs(got.UncoveredHumanMin-wantUncoveredMin) > 0.01 {
		t.Fatalf("uncovered_human_min = %.2f, want %.2f", got.UncoveredHumanMin, wantUncoveredMin)
	}
	if got.EstimateUncoveredHumanMin != got.UncoveredHumanMin {
		t.Fatalf("estimate_uncovered_human_min = %.2f, want %.2f", got.EstimateUncoveredHumanMin, got.UncoveredHumanMin)
	}
	wantCorrected := 60.0 + wantUncoveredMin
	if math.Abs(got.TotalActiveWorkCorrectedMin-wantCorrected) > 0.01 {
		t.Fatalf("active_work_corrected_min = %.2f, want %.2f", got.TotalActiveWorkCorrectedMin, wantCorrected)
	}
}

func TestAggregateEfficiencyV2NeedActuals_CommitWithinPreMarginCovered(t *testing.T) {
	base := time.Date(2026, 5, 18, 9, 0, 0, 0, time.UTC)
	session := efficiencyV2AggTestMetric("s-1", "u-alice", base.Add(30*time.Minute), base.Add(60*time.Minute), 30)
	// Commit before session start but within pre-margin (30 min default)
	commit := models.Commit{
		CommitId:   "c-pre",
		UserId:     "u-alice",
		RepoAddr:   "repo",
		RepoBranch: "feature/x",
		CommitTime: base.Add(10 * time.Minute), // 20 min before session start
		DiffLines:  50,
	}
	need := efficiencyV2AggTestNeed("need-pre", efficiencyV2BoundaryBranch, efficiencyV2ConfidenceHigh, "u-alice", []string{"s-1"}, []string{"c-pre"})

	cfg := EfficiencyV2Config{}
	updated := AggregateEfficiencyV2NeedActuals([]models.Need{need}, []models.SessionStageMetric{session}, []models.Commit{commit}, cfg, estimator.EstimateConfig{})
	got := updated[0]
	if got.UncoveredLoc != 0 {
		t.Fatalf("commit within pre-margin should be covered, uncovered_loc = %d", got.UncoveredLoc)
	}
	if got.AICoveredLoc != 50 {
		t.Fatalf("ai_covered_loc = %d, want 50", got.AICoveredLoc)
	}
}

func TestAggregateEfficiencyV2NeedActuals_SignalsAndRatios(t *testing.T) {
	base := time.Date(2026, 5, 18, 9, 0, 0, 0, time.UTC)
	session := efficiencyV2AggTestMetric("s-1", "u-alice", base, base.Add(60*time.Minute), 60)

	commits := []models.Commit{
		{CommitId: "c1", UserId: "u-alice", RepoAddr: "r", RepoBranch: "feature/x", CommitTime: base.Add(80 * time.Minute), DiffLines: 100, Silica: 0.8, Comment: "feat"},
		{CommitId: "c2", UserId: "u-alice", RepoAddr: "r", RepoBranch: "feature/x", CommitTime: base.Add(6 * time.Hour), DiffLines: 50, Silica: 0, Comment: "manual"},
		{CommitId: "c3", UserId: "u-alice", RepoAddr: "r", RepoBranch: "feature/x", CommitTime: base.Add(85 * time.Minute), DiffLines: 25, Silica: 0.5, Comment: "Revert \"feat\""},
	}

	need := efficiencyV2AggTestNeed("need-signals", efficiencyV2BoundaryBranch, efficiencyV2ConfidenceHigh, "u-alice", []string{"s-1"}, []string{"c1", "c2", "c3"})

	algo := estimator.EstimateConfig{LinesPerMinutes: 2, CommitLinePerMinutes: 100.0 / 480.0, MinMinutes: 5}
	updated := AggregateEfficiencyV2NeedActuals([]models.Need{need}, []models.SessionStageMetric{session}, commits, EfficiencyV2Config{}, algo)
	got := updated[0]

	if got.ChangedLoc != 175 {
		t.Fatalf("total_loc_net = %d, want 175", got.ChangedLoc)
	}
	// covered: c1 (100), c3 (25) → 125. uncovered: c2 (50)
	if got.AICoveredLoc != 125 {
		t.Fatalf("ai_covered_loc = %d, want 125", got.AICoveredLoc)
	}
	if got.UncoveredLoc != 50 {
		t.Fatalf("uncovered_loc = %d, want 50", got.UncoveredLoc)
	}
	if got.AICodeRatio == nil {
		t.Fatalf("ai_code_ratio should be populated")
	}
	if math.Abs(*got.AICodeRatio-125.0/175.0) > 1e-6 {
		t.Fatalf("ai_code_ratio = %.4f, want %.4f", *got.AICodeRatio, 125.0/175.0)
	}
	if got.UncoveredWorkRatio == nil {
		t.Fatalf("uncovered_work_ratio should be populated")
	}
	if math.Abs(*got.UncoveredWorkRatio-50.0/175.0) > 1e-6 {
		t.Fatalf("uncovered_work_ratio = %.4f, want %.4f", *got.UncoveredWorkRatio, 50.0/175.0)
	}
	if got.Silica == nil {
		t.Fatalf("silica should be populated")
	}
	// weighted by LOC: (0.8*100 + 0*50 + 0.5*25) / 175 = (80 + 0 + 12.5)/175 = 92.5/175
	wantSilica := (0.8*100 + 0*50 + 0.5*25) / 175.0
	if math.Abs(*got.Silica-wantSilica) > 1e-6 {
		t.Fatalf("silica = %.4f, want %.4f", *got.Silica, wantSilica)
	}
	if got.RevertCount != 1 {
		t.Fatalf("revert_count = %d, want 1", got.RevertCount)
	}
	if got.RevertRate == nil {
		t.Fatalf("revert_rate should be populated")
	}
	if math.Abs(*got.RevertRate-1.0/3.0) > 1e-6 {
		t.Fatalf("revert_rate = %.4f, want %.4f", *got.RevertRate, 1.0/3.0)
	}
}

func TestAggregateEfficiencyV2NeedActuals_DegradedReasonsWhenNoCommits(t *testing.T) {
	base := time.Date(2026, 5, 18, 9, 0, 0, 0, time.UTC)
	session := efficiencyV2AggTestMetric("s-1", "u-alice", base, base.Add(30*time.Minute), 30)
	need := efficiencyV2AggTestNeed("need-nocommit", efficiencyV2BoundaryFileCluster, efficiencyV2ConfidenceLow, "u-alice", []string{"s-1"}, nil)

	updated := AggregateEfficiencyV2NeedActuals([]models.Need{need}, []models.SessionStageMetric{session}, nil, EfficiencyV2Config{}, estimator.EstimateConfig{})
	got := updated[0]

	if got.AICodeRatio != nil {
		t.Fatalf("ai_code_ratio should be nil when no commits, got %v", *got.AICodeRatio)
	}
	if got.Silica != nil {
		t.Fatalf("silica should be nil when no commits, got %v", *got.Silica)
	}
	if got.UncoveredWorkRatio != nil {
		t.Fatalf("uncovered_work_ratio should be nil when no commits, got %v", *got.UncoveredWorkRatio)
	}

	signals := decodeEfficiencyV2JSONObject(t, got.QualitySignals)
	reason, _ := signals["reason"].(string)
	if !strings.Contains(reason, "no_commits") {
		t.Fatalf("quality_signals reason should mention no_commits, got %q", reason)
	}
}

func TestAggregateEfficiencyV2NeedActuals_LowAISignal(t *testing.T) {
	base := time.Date(2026, 5, 18, 9, 0, 0, 0, time.UTC)
	session := efficiencyV2AggTestMetric("s-1", "u-alice", base, base.Add(60*time.Minute), 60)
	commits := []models.Commit{
		{CommitId: "c1", UserId: "u-alice", RepoAddr: "r", RepoBranch: "feature/x", CommitTime: base.Add(80 * time.Minute), DiffLines: 100, Silica: 0.05},
	}
	need := efficiencyV2AggTestNeed("need-low-ai", efficiencyV2BoundaryBranch, efficiencyV2ConfidenceHigh, "u-alice", []string{"s-1"}, []string{"c1"})

	updated := AggregateEfficiencyV2NeedActuals([]models.Need{need}, []models.SessionStageMetric{session}, commits, EfficiencyV2Config{}, estimator.EstimateConfig{LinesPerMinutes: 2, CommitLinePerMinutes: 100.0 / 480.0, MinMinutes: 5})
	got := updated[0]

	if got.SilicaSignal != "low" {
		t.Fatalf("silica_signal = %q, want low (silica %.2f below threshold)", got.SilicaSignal, *got.Silica)
	}
}

func TestAggregateEfficiencyV2NeedActuals_HighUncoveredFlagsSignal(t *testing.T) {
	base := time.Date(2026, 5, 18, 9, 0, 0, 0, time.UTC)
	session := efficiencyV2AggTestMetric("s-1", "u-alice", base, base.Add(60*time.Minute), 60)
	commits := []models.Commit{
		{CommitId: "c1", UserId: "u-alice", RepoAddr: "r", RepoBranch: "feature/x", CommitTime: base.Add(80 * time.Minute), DiffLines: 30, Silica: 0.8},
		{CommitId: "c2", UserId: "u-alice", RepoAddr: "r", RepoBranch: "feature/x", CommitTime: base.Add(6 * time.Hour), DiffLines: 70, Silica: 0},
	}
	need := efficiencyV2AggTestNeed("need-uncov", efficiencyV2BoundaryBranch, efficiencyV2ConfidenceHigh, "u-alice", []string{"s-1"}, []string{"c1", "c2"})

	updated := AggregateEfficiencyV2NeedActuals([]models.Need{need}, []models.SessionStageMetric{session}, commits, EfficiencyV2Config{}, estimator.EstimateConfig{LinesPerMinutes: 2, CommitLinePerMinutes: 100.0 / 480.0, MinMinutes: 5})
	got := updated[0]

	// uncovered ratio = 70/100 = 0.7 > 0.3 default threshold → low signal
	if got.UncoveredWorkSignal != "low" {
		t.Fatalf("uncovered_work_signal = %q, want low (ratio %.2f)", got.UncoveredWorkSignal, *got.UncoveredWorkRatio)
	}
}

func TestAggregateEfficiencyV2NeedActuals_ThreeIntervalCalendar(t *testing.T) {
	base := time.Date(2026, 5, 18, 9, 0, 0, 0, time.UTC)
	// Cluster A (day 0), then 5-day idle gap, then cluster B (two short sessions day 5)
	sessionA := efficiencyV2AggTestMetric("s-a", "u-alice", base, base.Add(60*time.Minute), 60)
	sessionB := efficiencyV2AggTestMetric("s-b", "u-alice", base.Add(5*24*time.Hour), base.Add(5*24*time.Hour+30*time.Minute), 30)
	sessionC := efficiencyV2AggTestMetric("s-c", "u-alice", base.Add(5*24*time.Hour+90*time.Minute), base.Add(5*24*time.Hour+150*time.Minute), 60)

	need := efficiencyV2AggTestNeed("need-3int", efficiencyV2BoundaryBranch, efficiencyV2ConfidenceHigh, "u-alice", []string{"s-a", "s-b", "s-c"}, nil)
	updated := AggregateEfficiencyV2NeedActuals([]models.Need{need}, []models.SessionStageMetric{sessionA, sessionB, sessionC}, nil, EfficiencyV2Config{}, estimator.EstimateConfig{})
	got := updated[0]

	// Wall = 60 + 30 + 60 = 150 (no overlap)
	if got.TotalWallMin != 150 {
		t.Fatalf("wall_min = %.2f, want 150", got.TotalWallMin)
	}
	// Calendar = full span (5d 150min from start) minus long gap (5d - 1h between A and B).
	// Cluster B spans 30+60min sessions with a 60-min short gap kept; sessionB starts at base+5d, ends base+5d+30min;
	// sessionC starts base+5d+90min ends base+5d+150min. Short gap (60min) kept. Long gap excluded.
	// Expected = 60 (A) + (150 - 0) at day 5 (B start to C end = 150 min) = 60 + 150 = 210.
	if got.TotalCalendarMin != 210 {
		t.Fatalf("calendar_min = %.2f, want 210", got.TotalCalendarMin)
	}
}

func TestAggregateEfficiencyV2NeedActuals_NoSessionsAllUncovered(t *testing.T) {
	base := time.Date(2026, 5, 18, 9, 0, 0, 0, time.UTC)
	commits := []models.Commit{
		{CommitId: "c1", UserId: "u-alice", RepoAddr: "r", RepoBranch: "feature/x", CommitTime: base, DiffLines: 50},
	}
	need := efficiencyV2AggTestNeed("need-nosess", efficiencyV2BoundaryBranch, efficiencyV2ConfidenceHigh, "u-alice", nil, []string{"c1"})

	updated := AggregateEfficiencyV2NeedActuals([]models.Need{need}, nil, commits, EfficiencyV2Config{}, estimator.EstimateConfig{CommitLinePerMinutes: 100.0 / 480.0, MinMinutes: 5})
	got := updated[0]
	if got.UncoveredLoc != 50 {
		t.Fatalf("uncovered_loc = %d, want 50 (no sessions = all uncovered)", got.UncoveredLoc)
	}
	if got.AICoveredLoc != 0 {
		t.Fatalf("ai_covered_loc = %d, want 0", got.AICoveredLoc)
	}
}

func TestAggregateEfficiencyV2NeedActuals_RevertChinese(t *testing.T) {
	base := time.Date(2026, 5, 18, 9, 0, 0, 0, time.UTC)
	session := efficiencyV2AggTestMetric("s-1", "u-alice", base, base.Add(60*time.Minute), 60)
	commits := []models.Commit{
		{CommitId: "c1", UserId: "u-alice", CommitTime: base.Add(80 * time.Minute), DiffLines: 30, Comment: "回滚 之前的修改"},
		{CommitId: "c2", UserId: "u-alice", CommitTime: base.Add(85 * time.Minute), DiffLines: 25, Comment: "撤销 ftp 改动"},
		{CommitId: "c3", UserId: "u-alice", CommitTime: base.Add(90 * time.Minute), DiffLines: 25, Comment: "normal commit"},
	}
	need := efficiencyV2AggTestNeed("need-rev-cn", efficiencyV2BoundaryBranch, efficiencyV2ConfidenceHigh, "u-alice", []string{"s-1"}, []string{"c1", "c2", "c3"})

	updated := AggregateEfficiencyV2NeedActuals([]models.Need{need}, []models.SessionStageMetric{session}, commits, EfficiencyV2Config{}, estimator.EstimateConfig{})
	got := updated[0]
	if got.RevertCount != 2 {
		t.Fatalf("revert_count = %d, want 2", got.RevertCount)
	}
}

func TestAggregateEfficiencyV2NeedActuals_AlwaysIncludesCoveredRuleReason(t *testing.T) {
	base := time.Date(2026, 5, 18, 9, 0, 0, 0, time.UTC)
	session := efficiencyV2AggTestMetric("s-1", "u-alice", base, base.Add(60*time.Minute), 60)
	commit := models.Commit{CommitId: "c1", UserId: "u-alice", RepoAddr: "r", RepoBranch: "feature/x", CommitTime: base.Add(80 * time.Minute), DiffLines: 100, Silica: 0.8}
	need := efficiencyV2AggTestNeed("need-rule", efficiencyV2BoundaryBranch, efficiencyV2ConfidenceHigh, "u-alice", []string{"s-1"}, []string{"c1"})

	updated := AggregateEfficiencyV2NeedActuals([]models.Need{need}, []models.SessionStageMetric{session}, []models.Commit{commit}, EfficiencyV2Config{}, estimator.EstimateConfig{CommitLinePerMinutes: 100.0 / 480.0, MinMinutes: 5})
	signals := decodeEfficiencyV2JSONObject(t, updated[0].QualitySignals)
	reason, _ := signals["reason"].(string)
	if !strings.Contains(reason, "covered_rule=temporal_only") {
		t.Fatalf("quality_signals reason should include covered_rule=temporal_only, got %q", reason)
	}
}

func TestNormalizeEfficiencyV2AlgoConfigCommitMinutesPerLineOverridesLineRate(t *testing.T) {
	algo := normalizeEfficiencyV2AlgoConfig(estimator.EstimateConfig{
		CommitLinePerMinutes: 0.2,
		CommitMinutesPerLine: 2,
	})

	if algo.CommitMinutesPerLine != 2 {
		t.Fatalf("CommitMinutesPerLine = %v, want 2", algo.CommitMinutesPerLine)
	}
	if algo.CommitLinePerMinutes != 0.5 {
		t.Fatalf("CommitLinePerMinutes = %v, want 0.5", algo.CommitLinePerMinutes)
	}
}

func efficiencyV2AggTestMetric(sessionID, userID string, start, end time.Time, activeMin float64) models.SessionStageMetric {
	startCopy := start
	endCopy := end
	return models.SessionStageMetric{
		SessionId:      sessionID,
		UserId:         userID,
		SessionStartTs: &startCopy,
		SessionEndTs:   &endCopy,
		TotalActiveMin: activeMin,
		TotalWallMin:   end.Sub(start).Minutes(),
	}
}

func efficiencyV2AggTestNeed(needID, source, confidence, primaryUser string, sessions, commits []string) models.Need {
	contributors := []string{primaryUser}
	return models.Need{
		NeedId:             needID,
		BoundarySource:     source,
		BoundaryConfidence: confidence,
		BoundaryKey:        needID,
		BoundaryEvidence:   models.ObjectJSON("{}"),
		Status:             "merged",
		PrimaryUserId:      primaryUser,
		ContributorUserIds: efficiencyV2StringJSON(contributors),
		SessionIds:         efficiencyV2StringJSON(sessions),
		CommitIds:          efficiencyV2StringJSON(commits),
		TouchedFiles:       models.StringJSON("[]"),
		UncoveredCommitIds: models.StringJSON("[]"),
		QualitySignals:     models.ObjectJSON("{}"),
		ConfidenceSignals:  models.ObjectJSON("{}"),
	}
}

func decodeEfficiencyV2JSONObject(t *testing.T, payload models.ObjectJSON) map[string]interface{} {
	t.Helper()
	result := map[string]interface{}{}
	if payload == "" || payload == "null" || payload == "{}" {
		return result
	}
	if err := json.Unmarshal([]byte(payload), &result); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	return result
}
