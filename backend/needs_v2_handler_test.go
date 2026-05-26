package main

import (
	"testing"

	"kanban/core/models"
)

func TestSummarizeNeed_CopiesPersistedFields(t *testing.T) {
	ratio := 0.25
	bandLow := 0.15
	bandHigh := 0.4
	work := 0.30
	calendar := 400.0
	fused := 260.0
	need := models.Need{
		NeedId:                      "n-1",
		BoundarySource:              "lv2_branch",
		BoundaryConfidence:          "high",
		Status:                      "merged",
		RepoAddr:                    "git@example.com/x.git",
		RepoBranch:                  "feature/x",
		PrimaryUserId:               "u-1",
		TotalCalendarMin:            300,
		BaselineCalendarMin:         &calendar,
		TotalActiveWorkCorrectedMin: 210,
		BaselineFusedWorkMin:        &fused,
		EfficiencyRatio:             &ratio,
		EfficiencyLowerBand:         &bandLow,
		EfficiencyUpperBand:         &bandHigh,
		WorkEfficiencyRatio:         &work,
		ConfidenceLevel:             "high",
		OutlierFlag:                 false,
		CoverageEligible:            true,
		ThinkActiveMin:              20,
		ExecutionActiveMin:          40,
		VerificationActiveMin:       10,
		Reason:                      "ok",
	}
	got := summarizeNeed(need)
	if got.NeedId != "n-1" || got.EfficiencyRatio == nil || *got.EfficiencyRatio != 0.25 {
		t.Fatalf("summary missing fields: %+v", got)
	}
	if got.EfficiencyBandLow == nil || *got.EfficiencyBandLow != 0.15 {
		t.Fatalf("low band not copied")
	}
	if got.WorkEfficiencyRatio == nil || *got.WorkEfficiencyRatio != 0.30 {
		t.Fatalf("work efficiency not copied")
	}
	if got.TotalActiveWorkMin != 210 || got.BaselineFusedWorkMin == nil || *got.BaselineFusedWorkMin != 260 {
		t.Fatalf("work terms not copied")
	}
}

func TestBuildEfficiencyV2BaselineComponents_NullSafe(t *testing.T) {
	need := models.Need{NeedId: "n-1"}
	got := buildEfficiencyV2BaselineComponents(need)
	if got.AlgoTotalMin != nil || got.AnchorKnnMin != nil || got.LLMTotalMin != nil {
		t.Fatalf("baseline components should be nil for fresh need")
	}
}

func TestEfficiencyV2DecodeJSONStringSlice_HandlesEmpty(t *testing.T) {
	if out := efficiencyV2DecodeJSONStringSlice(""); out != nil {
		t.Fatalf("empty should decode to nil")
	}
	if out := efficiencyV2DecodeJSONStringSlice(models.StringJSON("[]")); out != nil {
		t.Fatalf("empty array should decode to nil")
	}
	out := efficiencyV2DecodeJSONStringSlice(models.StringJSON(`["a","b"]`))
	if len(out) != 2 || out[0] != "a" || out[1] != "b" {
		t.Fatalf("decode failed: %#v", out)
	}
}

func TestParsePagePageSize_Bounds(t *testing.T) {
	if parsePage("") != 1 {
		t.Fatalf("empty page should default to 1")
	}
	if parsePage("3") != 3 {
		t.Fatalf("page 3 should pass through")
	}
	if parsePage("-2") != 1 {
		t.Fatalf("negative page should default to 1")
	}
	if parsePageSize("") != 20 {
		t.Fatalf("empty pageSize should default to 20")
	}
	if parsePageSize("500") != 200 {
		t.Fatalf("pageSize > 200 should be clamped to 200")
	}
	if parsePageSize("0") != 20 {
		t.Fatalf("pageSize 0 should default to 20")
	}
}
