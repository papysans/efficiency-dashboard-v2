package efficiencyv2

import (
	"math"
	"testing"

	"kanban/core/models"
)

func TestComputeEfficiencyV2BaselineB_AnchorsAvailable(t *testing.T) {
	anchors := []EfficiencyV2KNNAnchor{
		{AnchorID: "a1", WithoutAIMinutes: 200, Weight: 1, FeatureVector: map[string]float64{"loc": 100, "files": 3, "turns": 30}},
		{AnchorID: "a2", WithoutAIMinutes: 800, Weight: 1, FeatureVector: map[string]float64{"loc": 500, "files": 12, "turns": 90}},
		{AnchorID: "a3", WithoutAIMinutes: 600, Weight: 2, FeatureVector: map[string]float64{"loc": 300, "files": 8, "turns": 60}},
	}
	need := map[string]float64{"loc": 110, "files": 4, "turns": 28}
	result := ComputeEfficiencyV2BaselineB(need, anchors, 2)
	if result.Estimate == nil {
		t.Fatalf("estimate should be set")
	}
	// Closest is a1 (much closer than a2/a3), so estimate should be closer to 200 than 800.
	if *result.Estimate < 100 || *result.Estimate > 500 {
		t.Fatalf("estimate = %.2f, expected near a1=200", *result.Estimate)
	}
	if len(result.NeighborIDs) != 2 {
		t.Fatalf("neighbor count = %d, want 2", len(result.NeighborIDs))
	}
	if result.NeighborIDs[0] != "a1" {
		t.Fatalf("first neighbor = %s, want a1", result.NeighborIDs[0])
	}
}

func TestComputeEfficiencyV2BaselineB_EmptyAnchorsReturnsNullReason(t *testing.T) {
	result := ComputeEfficiencyV2BaselineB(map[string]float64{"loc": 100}, nil, 3)
	if result.Estimate != nil {
		t.Fatalf("estimate should be nil for empty anchors")
	}
	if result.Reason == "" {
		t.Fatalf("reason should be set, got empty")
	}
}

func TestComputeEfficiencyV2BaselineB_InverseDistanceWeighting(t *testing.T) {
	// Two equidistant anchors with different weights should average proportionally.
	anchors := []EfficiencyV2KNNAnchor{
		{AnchorID: "near", WithoutAIMinutes: 100, Weight: 1, FeatureVector: map[string]float64{"x": 10}},
		{AnchorID: "far", WithoutAIMinutes: 500, Weight: 1, FeatureVector: map[string]float64{"x": 20}},
	}
	need := map[string]float64{"x": 10}
	result := ComputeEfficiencyV2BaselineB(need, anchors, 2)
	// near distance = 0, far distance = 10. Weights = 1/1=1 and 1/11 ≈ 0.0909.
	// expected = (100*1 + 500*0.0909) / 1.0909 ≈ 133.33
	if *result.Estimate < 130 || *result.Estimate > 140 {
		t.Fatalf("estimate = %.2f, expected ~133", *result.Estimate)
	}
}

func TestComputeEfficiencyV2BaselineB_TopKSelectsLowestDistance(t *testing.T) {
	anchors := []EfficiencyV2KNNAnchor{
		{AnchorID: "a", WithoutAIMinutes: 100, Weight: 1, FeatureVector: map[string]float64{"x": 0}},
		{AnchorID: "b", WithoutAIMinutes: 200, Weight: 1, FeatureVector: map[string]float64{"x": 100}},
		{AnchorID: "c", WithoutAIMinutes: 300, Weight: 1, FeatureVector: map[string]float64{"x": 200}},
	}
	need := map[string]float64{"x": 5}
	result := ComputeEfficiencyV2BaselineB(need, anchors, 1)
	if len(result.NeighborIDs) != 1 || result.NeighborIDs[0] != "a" {
		t.Fatalf("top-1 neighbor = %v, want [a]", result.NeighborIDs)
	}
}

func TestEfficiencyV2KNNDistance_EuclideanSymmetry(t *testing.T) {
	a := map[string]float64{"x": 3, "y": 4}
	b := map[string]float64{"x": 0, "y": 0}
	if d := efficiencyV2KNNDistance(a, b); math.Abs(d-5) > 1e-9 {
		t.Fatalf("distance(a,b) = %.4f, want 5", d)
	}
	if d := efficiencyV2KNNDistance(b, a); math.Abs(d-5) > 1e-9 {
		t.Fatalf("distance(b,a) = %.4f, want 5", d)
	}
}

func TestBuildEfficiencyV2NeedFeatureVector_IncludesKeys(t *testing.T) {
	need := models.Need{ChangedLoc: 250, FileCount: 5, ThinkActiveMin: 30, ExecutionActiveMin: 60, VerificationActiveMin: 15}
	sessions := []models.SessionStageMetric{{MessageEventCount: 12}, {MessageEventCount: 8}}
	vec := BuildEfficiencyV2NeedFeatureVector(need, sessions)
	for _, key := range []string{"loc", "files", "turns", "think", "exec", "verify"} {
		if _, ok := vec[key]; !ok {
			t.Fatalf("feature vector missing %q: %#v", key, vec)
		}
	}
	if vec["turns"] != 20 {
		t.Fatalf("turns = %.2f, want 20", vec["turns"])
	}
}

func TestPersistEfficiencyV2BaselineBOnNeed_AssignsFields(t *testing.T) {
	need := models.Need{NeedId: "n-1"}
	estimate := 350.0
	PersistEfficiencyV2BaselineBOnNeed(&need, EfficiencyV2KNNResult{Estimate: &estimate, Reason: "knn:k=3"})
	if need.BaselineAnchorKnnWorkMin == nil || *need.BaselineAnchorKnnWorkMin != 350 {
		t.Fatalf("knn estimate not assigned: %v", need.BaselineAnchorKnnWorkMin)
	}
	if need.BaselineAnchorKnnReason != "knn:k=3" {
		t.Fatalf("knn reason = %q, want knn:k=3", need.BaselineAnchorKnnReason)
	}
}

func TestEfficiencyV2OptionalAnchorFetchPlan_HasOfflineFallback(t *testing.T) {
	plan := EfficiencyV2OptionalAnchorFetchPlan()
	if plan.OfflineFallback == "" {
		t.Fatalf("offline fallback must be set so tests can run without network")
	}
	if plan.CachePath == "" {
		t.Fatalf("cache path must be set")
	}
}
