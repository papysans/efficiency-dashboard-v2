package main

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
			NeedId: "n1", PrimaryUserId: "u1", Status: "merged",
			BoundaryConfidence: efficiencyV2ConfidenceHigh,
			TotalCalendarMin:   240, BaselineCalendarMin: &baseHigh,
			BaselineFusedWorkMin: ptrFloat(100), TotalActiveWorkCorrectedMin: 100,
			DevEndTs: ptrTime(devEnd),
		},
		{
			NeedId: "n2", PrimaryUserId: "u1", Status: "merged",
			BoundaryConfidence: efficiencyV2ConfidenceHigh,
			TotalCalendarMin:   360, BaselineCalendarMin: &baseHigh2,
			BaselineFusedWorkMin: ptrFloat(200), TotalActiveWorkCorrectedMin: 200,
			DevEndTs: ptrTime(devEnd),
		},
	}
	rows := AggregateEfficiencyV2UserProductivity(needs, EfficiencyV2Config{})
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

func TestAggregateEfficiencyV2UserProductivity_LowConfidenceNeedsDontEnterRatio(t *testing.T) {
	weekStart := time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC)
	devEnd := weekStart.Add(48 * time.Hour)
	baseHigh := 400.0
	needs := []models.Need{
		{
			NeedId: "n1", PrimaryUserId: "u1", Status: "merged",
			BoundaryConfidence: efficiencyV2ConfidenceHigh,
			TotalCalendarMin:   200, BaselineCalendarMin: &baseHigh,
			TotalActiveWorkCorrectedMin: 80, BaselineFusedWorkMin: ptrFloat(100),
			DevEndTs: ptrTime(devEnd),
		},
		{
			NeedId: "n2", PrimaryUserId: "u1", Status: "merged",
			BoundaryConfidence: efficiencyV2ConfidenceLow,
			TotalCalendarMin:   500, BaselineCalendarMin: ptrFloat(900),
			TotalActiveWorkCorrectedMin: 200,
			DevEndTs: ptrTime(devEnd),
		},
	}
	rows := AggregateEfficiencyV2UserProductivity(needs, EfficiencyV2Config{})
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
			NeedId: "n1", PrimaryUserId: "u1", Status: "merged",
			BoundaryConfidence: efficiencyV2ConfidenceHigh,
			TotalCalendarMin:   100, BaselineCalendarMin: &baseHigh,
			TotalActiveWorkCorrectedMin: 60, BaselineFusedWorkMin: ptrFloat(50),
			DevEndTs: ptrTime(devEnd),
		},
		{
			NeedId: "n2", PrimaryUserId: "u1", Status: "abandoned",
			BoundaryConfidence: efficiencyV2ConfidenceHigh,
			TotalCalendarMin:   500, BaselineCalendarMin: ptrFloat(900),
			TotalActiveWorkCorrectedMin: 200,
			DevEndTs: ptrTime(devEnd),
		},
	}
	rows := AggregateEfficiencyV2UserProductivity(needs, EfficiencyV2Config{})
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
			NeedId: "n1", PrimaryUserId: "u1", Status: "active",
			BoundaryConfidence: efficiencyV2ConfidenceVeryLow,
			TotalActiveWorkCorrectedMin: 500,
			DevEndTs: ptrTime(devEnd),
		},
	}
	rows := AggregateEfficiencyV2UserProductivity(needs, EfficiencyV2Config{})
	row := rows[0]
	if !row.ConfidenceLimited {
		t.Fatalf("confidence_limited should be true when no eligible baseline; reason=%q", row.ConfidenceReason)
	}
	if !strings.Contains(row.ConfidenceReason, "no_eligible_baseline") {
		t.Fatalf("reason should mention no_eligible_baseline, got %q", row.ConfidenceReason)
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

func ptrFloat(v float64) *float64 { return &v }
func ptrTime(t time.Time) *time.Time { return &t }
