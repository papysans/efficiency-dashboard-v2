package efficiencyv2

import (
	"math"
	"strings"
	"testing"
	"time"

	"kanban/core/models"
)

func TestAggregateEfficiencyV2UserProductivity_RatioFromSums(t *testing.T) {
	weekStart := time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC)
	devEnd := weekStart.Add(72 * time.Hour)

	baseHigh := 400.0
	baseHigh2 := 800.0
	needs := []models.Need{
		{
			NeedId: "n1", PrimaryUserId: "u1", SessionIds: EfficiencyV2StringJSON([]string{"s1"}), Status: "merged",
			BoundaryConfidence: efficiencyV2ConfidenceHigh, CoverageEligible: true,
			TotalCalendarMin: 240, BaselineCalendarMin: &baseHigh,
			BaselineFusedWorkMin: ptrFloat(100), TotalActiveWorkCorrectedMin: 100,
			DevEndTs: ptrTime(devEnd),
		},
		{
			NeedId: "n2", PrimaryUserId: "u1", SessionIds: EfficiencyV2StringJSON([]string{"s1"}), Status: "merged",
			BoundaryConfidence: efficiencyV2ConfidenceHigh, CoverageEligible: true,
			TotalCalendarMin: 360, BaselineCalendarMin: &baseHigh2,
			BaselineFusedWorkMin: ptrFloat(200), TotalActiveWorkCorrectedMin: 200,
			DevEndTs: ptrTime(devEnd),
		},
	}
	rows := AggregateEfficiencyV2UserProductivity(needs, EfficiencyV2Config{}, efficiencyV2AIUsersFromNeeds(needs))
	if len(rows) != 1 {
		t.Fatalf("expected 1 user-week row, got %d", len(rows))
	}
	row := rows[0]
	if row.ActualCalendarMin != 600 || row.BaselineCalendarMin != 1200 {
		t.Fatalf("sums actual=%.2f baseline=%.2f, want 600/1200", row.ActualCalendarMin, row.BaselineCalendarMin)
	}
	// ratio = (1200 - 600) / 600 = 1.0
	if row.EfficiencyRatio == nil || math.Abs(*row.EfficiencyRatio-1.0) > 1e-6 {
		t.Fatalf("ratio = %v, want 1.0", row.EfficiencyRatio)
	}
}

func TestAggregateEfficiencyV2UserProductivity_BlockedPrimaryUserSkipped(t *testing.T) {
	weekStart := time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC)
	devEnd := weekStart.Add(48 * time.Hour)
	baseHigh := 400.0
	needs := []models.Need{
		{
			NeedId: "n1", PrimaryUserId: "u1", SessionIds: EfficiencyV2StringJSON([]string{"s1"}), Status: "merged",
			BoundaryConfidence: efficiencyV2ConfidenceHigh, CoverageEligible: true,
			TotalCalendarMin: 240, BaselineCalendarMin: &baseHigh,
			BaselineFusedWorkMin: ptrFloat(100), TotalActiveWorkCorrectedMin: 100,
			DevEndTs: ptrTime(devEnd),
		},
		{
			// blocked 账号的残留 need（清理跑前的旧行）不得串进周表
			NeedId: "n2", PrimaryUserId: "blocked-user", Status: "merged",
			BoundaryConfidence: efficiencyV2ConfidenceHigh, CoverageEligible: true,
			TotalCalendarMin: 999, BaselineCalendarMin: ptrFloat(9999),
			BaselineFusedWorkMin: ptrFloat(999), TotalActiveWorkCorrectedMin: 999,
			DevEndTs: ptrTime(devEnd),
		},
	}
	rows := AggregateEfficiencyV2UserProductivity(needs, EfficiencyV2Config{
		BlockedUserIds: []string{"blocked-user"},
	}, efficiencyV2AIUsersFromNeeds(needs))
	if len(rows) != 1 {
		t.Fatalf("blocked primary_user 的 need 应被跳过，期望 1 行，得到 %d 行", len(rows))
	}
	if rows[0].UserId != "u1" || rows[0].ActualCalendarMin != 240 {
		t.Fatalf("仅 u1 进周表（actual=240），得到 user=%q actual=%.2f", rows[0].UserId, rows[0].ActualCalendarMin)
	}
}

func TestAggregateEfficiencyV2UserProductivity_LowConfidenceNeedsDontEnterRatio(t *testing.T) {
	weekStart := time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC)
	devEnd := weekStart.Add(48 * time.Hour)
	baseHigh := 400.0
	needs := []models.Need{
		{
			NeedId: "n1", PrimaryUserId: "u1", SessionIds: EfficiencyV2StringJSON([]string{"s1"}), Status: "merged",
			BoundaryConfidence: efficiencyV2ConfidenceHigh, CoverageEligible: true,
			TotalCalendarMin: 200, BaselineCalendarMin: &baseHigh,
			TotalActiveWorkCorrectedMin: 80, BaselineFusedWorkMin: ptrFloat(100),
			DevEndTs: ptrTime(devEnd),
		},
		{
			// low-confidence → 非 coverage_eligible，不进提效比口径。
			NeedId: "n2", PrimaryUserId: "u1", SessionIds: EfficiencyV2StringJSON([]string{"s1"}), Status: "merged",
			BoundaryConfidence: efficiencyV2ConfidenceLow,
			TotalCalendarMin:   500, BaselineCalendarMin: ptrFloat(900),
			TotalActiveWorkCorrectedMin: 200,
			DevEndTs:                    ptrTime(devEnd),
		},
	}
	rows := AggregateEfficiencyV2UserProductivity(needs, EfficiencyV2Config{}, efficiencyV2AIUsersFromNeeds(needs))
	row := rows[0]
	// Only high-confidence n1 contributes to ratio terms.
	if row.ActualCalendarMin != 200 || row.BaselineCalendarMin != 400 {
		t.Fatalf("ratio terms = %.2f / %.2f, want 200 / 400", row.ActualCalendarMin, row.BaselineCalendarMin)
	}
	if row.CoverageLowUnreported != 200 {
		t.Fatalf("low coverage = %.2f, want 200", row.CoverageLowUnreported)
	}
}

func TestAggregateEfficiencyV2UserProductivity_AbandonedNotInRatio(t *testing.T) {
	weekStart := time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC)
	devEnd := weekStart.Add(48 * time.Hour)
	baseHigh := 200.0
	needs := []models.Need{
		{
			NeedId: "n1", PrimaryUserId: "u1", SessionIds: EfficiencyV2StringJSON([]string{"s1"}), Status: "merged",
			BoundaryConfidence: efficiencyV2ConfidenceHigh, CoverageEligible: true,
			TotalCalendarMin: 100, BaselineCalendarMin: &baseHigh,
			TotalActiveWorkCorrectedMin: 60, BaselineFusedWorkMin: ptrFloat(50),
			DevEndTs: ptrTime(devEnd),
		},
		{
			NeedId: "n2", PrimaryUserId: "u1", SessionIds: EfficiencyV2StringJSON([]string{"s1"}), Status: "abandoned",
			BoundaryConfidence: efficiencyV2ConfidenceHigh,
			TotalCalendarMin:   500, BaselineCalendarMin: ptrFloat(900),
			TotalActiveWorkCorrectedMin: 200,
			DevEndTs:                    ptrTime(devEnd),
		},
	}
	rows := AggregateEfficiencyV2UserProductivity(needs, EfficiencyV2Config{}, efficiencyV2AIUsersFromNeeds(needs))
	row := rows[0]
	if row.ActualCalendarMin != 100 {
		t.Fatalf("abandoned needs must not enter ratio (actual=%.2f, want 100)", row.ActualCalendarMin)
	}
	if row.CoverageAbandoned != 200 {
		t.Fatalf("coverage_abandoned = %.2f, want 200", row.CoverageAbandoned)
	}
	if row.AbandonedNeedCount != 1 {
		t.Fatalf("abandoned_need_count = %d, want 1", row.AbandonedNeedCount)
	}
}

func TestAggregateEfficiencyV2UserProductivity_CoverageLimitedFlag(t *testing.T) {
	weekStart := time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC)
	devEnd := weekStart.Add(48 * time.Hour)
	needs := []models.Need{
		{
			NeedId: "n1", PrimaryUserId: "u1", SessionIds: EfficiencyV2StringJSON([]string{"s1"}), Status: "active",
			BoundaryConfidence:          efficiencyV2ConfidenceVeryLow,
			TotalActiveWorkCorrectedMin: 500,
			DevEndTs:                    ptrTime(devEnd),
		},
	}
	rows := AggregateEfficiencyV2UserProductivity(needs, EfficiencyV2Config{}, efficiencyV2AIUsersFromNeeds(needs))
	row := rows[0]
	if !row.ConfidenceLimited {
		t.Fatalf("confidence_limited should be true when no eligible baseline; reason=%q", row.ConfidenceReason)
	}
	if !strings.Contains(row.ConfidenceReason, "no_eligible_baseline") {
		t.Fatalf("reason should mention no_eligible_baseline, got %q", row.ConfidenceReason)
	}
}

// 核心回归：工作量侧 outlier 不应连累同一 need 的日历提效（本任务修复目标）。
// 对照旧行为：单一 outlier_flag 会把 n1 的日历提效一并隐藏，导致用户「有 commit 没提效」。
func TestAggregateEfficiencyV2UserProductivity_CaliberSplitOutlier(t *testing.T) {
	weekStart := time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC)
	devEnd := weekStart.Add(48 * time.Hour)
	needs := []models.Need{
		{
			// 仅工作量侧 outlier：日历提效合理(应计入)，工作量提效极端(应剔除)。
			NeedId: "n1", PrimaryUserId: "u1", SessionIds: EfficiencyV2StringJSON([]string{"s1"}), Status: "merged",
			BoundaryConfidence: efficiencyV2ConfidenceHigh, CoverageEligible: true,
			WorkOutlierFlag: true, OutlierFlag: true, // 派生 outlier_flag=true
			TotalCalendarMin: 200, BaselineCalendarMin: ptrFloat(500),
			TotalActiveWorkCorrectedMin: 5, BaselineFusedWorkMin: ptrFloat(8000),
			DevEndTs: ptrTime(devEnd),
		},
		{
			// 仅日历侧 outlier：工作量提效合理(应计入)，日历提效极端(应剔除)。
			NeedId: "n2", PrimaryUserId: "u1", SessionIds: EfficiencyV2StringJSON([]string{"s1"}), Status: "merged",
			BoundaryConfidence: efficiencyV2ConfidenceHigh, CoverageEligible: true,
			CalendarOutlierFlag: true, OutlierFlag: true,
			TotalCalendarMin: 10, BaselineCalendarMin: ptrFloat(9000),
			TotalActiveWorkCorrectedMin: 100, BaselineFusedWorkMin: ptrFloat(300),
			DevEndTs: ptrTime(devEnd),
		},
	}
	rows := AggregateEfficiencyV2UserProductivity(needs, EfficiencyV2Config{}, efficiencyV2AIUsersFromNeeds(needs))
	row := rows[0]
	// 日历口径：只 n1 计入(n2 被 calendar_outlier 剔除)。
	if row.ActualCalendarMin != 200 || row.BaselineCalendarMin != 500 {
		t.Fatalf("calendar terms = %.2f / %.2f, want 200 / 500 (only n1)", row.ActualCalendarMin, row.BaselineCalendarMin)
	}
	// 工作量口径：只 n2 计入(n1 被 work_outlier 剔除)。
	if row.ActualActiveWorkCorrectedMin != 100 || row.BaselineFusedWorkMin != 300 {
		t.Fatalf("work terms = %.2f / %.2f, want 100 / 300 (only n2)", row.ActualActiveWorkCorrectedMin, row.BaselineFusedWorkMin)
	}
	// n1 的日历提效复活 = (500-200)/200 = 1.5
	if row.EfficiencyRatio == nil || math.Abs(*row.EfficiencyRatio-1.5) > 1e-6 {
		t.Fatalf("calendar ratio = %v, want 1.5 (n1 revived)", row.EfficiencyRatio)
	}
}

func TestEfficiencyV2WeekAnchorForNeed_AlignsToMonday(t *testing.T) {
	// 2026-05-22 is a Friday. Anchor should be the Monday of that ISO week (2026-05-18).
	dev := time.Date(2026, 5, 22, 14, 30, 0, 0, time.UTC)
	need := models.Need{DevEndTs: &dev}
	anchor := efficiencyV2WeekAnchorForNeed(need)
	want := time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC)
	if !anchor.Equal(want) {
		t.Fatalf("anchor = %v, want %v", anchor, want)
	}
}

// 人级软件用户口径：从未采集到任何 session 的纯非用户(只有 commit-only need)不进周表；
// 真 AI 用户(有任一带 session 的 need)的全部 need 保留——包括其没关联上 session 的 commit-only need。
func TestAggregateEfficiencyV2UserProductivity_NonSoftwareUserSkipped(t *testing.T) {
	weekStart := time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC)
	devEnd := weekStart.Add(48 * time.Hour)
	baseHigh := 400.0
	needs := []models.Need{
		{
			// AI 用户：有带 session 的 need。
			NeedId: "n1", PrimaryUserId: "ai-user", Status: "merged",
			SessionIds:         EfficiencyV2StringJSON([]string{"s1"}),
			BoundaryConfidence: efficiencyV2ConfidenceHigh, CoverageEligible: true,
			TotalCalendarMin: 240, BaselineCalendarMin: &baseHigh,
			TotalActiveWorkCorrectedMin: 100, BaselineFusedWorkMin: ptrFloat(100),
			DevEndTs: ptrTime(devEnd),
		},
		{
			// 同一 AI 用户的 commit-only need（没关联上 session）也应保留，计入其交付。
			NeedId: "n2", PrimaryUserId: "ai-user", Status: "merged",
			BoundaryConfidence: efficiencyV2ConfidenceHigh,
			TotalCalendarMin:   120,
			DevEndTs:           ptrTime(devEnd),
		},
		{
			// 纯非用户：所有 need 都无 session → 不进周表。
			NeedId: "n3", PrimaryUserId: "non-user", Status: "merged",
			BoundaryConfidence: efficiencyV2ConfidenceHigh, CoverageEligible: true,
			TotalCalendarMin: 999, BaselineCalendarMin: ptrFloat(9999),
			DevEndTs: ptrTime(devEnd),
		},
	}
	rows := AggregateEfficiencyV2UserProductivity(needs, EfficiencyV2Config{}, efficiencyV2AIUsersFromNeeds(needs))
	if len(rows) != 1 {
		t.Fatalf("仅 ai-user 进周表(non-user 纯非用户应跳过)，期望 1 行，得到 %d 行", len(rows))
	}
	if rows[0].UserId != "ai-user" {
		t.Fatalf("期望 ai-user 进表，得到 %q", rows[0].UserId)
	}
	// 关键：ai-user 的 commit-only need(n2)不被误杀，仍计入其交付。
	if rows[0].MergedNeedCount != 2 {
		t.Fatalf("ai-user 的全部 merged need 应保留(含 commit-only n2)，merged_need_count=%d，want 2", rows[0].MergedNeedCount)
	}
}

func ptrFloat(v float64) *float64    { return &v }
func ptrTime(t time.Time) *time.Time { return &t }
