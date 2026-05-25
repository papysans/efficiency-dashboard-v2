package main

import (
	"math"
	"strings"
	"testing"

	"kanban/core/models"
)

func TestComputeEfficiencyV2Fusion_SingleBaselineLowConfidence(t *testing.T) {
	algo := 200.0
	cfg := EfficiencyV2Config{}
	need := models.Need{NeedId: "n-1", TotalCalendarMin: 480, TotalActiveWorkCorrectedMin: 200}
	result := ComputeEfficiencyV2Fusion(need, EfficiencyV2FusionInputs{
		AlgoMin: &algo,
		Weights: EfficiencyV2BaselineDefaults{WeightAlgo: 1, TeamWorkDensity: 0.25},
	}, cfg)
	if result.FusedWorkMin == nil || *result.FusedWorkMin != 200 {
		t.Fatalf("fused = %v, want 200", result.FusedWorkMin)
	}
	if result.ConfidenceLevel != efficiencyV2StageConfidenceLow {
		t.Fatalf("confidence = %q, want low (single baseline)", result.ConfidenceLevel)
	}
	joined := strings.Join(result.Reasons, "|")
	if !strings.Contains(joined, "single_baseline") {
		t.Fatalf("reason should mention single_baseline: %v", result.Reasons)
	}
}

func TestComputeEfficiencyV2Fusion_TwoBaselinesAverages(t *testing.T) {
	algo := 200.0
	knn := 300.0
	need := models.Need{NeedId: "n-1", TotalCalendarMin: 480}
	result := ComputeEfficiencyV2Fusion(need, EfficiencyV2FusionInputs{
		AlgoMin: &algo,
		KNNMin:  &knn,
		Weights: EfficiencyV2BaselineDefaults{WeightAlgo: 0.5, WeightKNN: 0.5, TeamWorkDensity: 0.25},
	}, EfficiencyV2Config{})
	if result.FusedWorkMin == nil || *result.FusedWorkMin != 250 {
		t.Fatalf("fused = %v, want 250", result.FusedWorkMin)
	}
	if result.SpreadWorkMin == nil || *result.SpreadWorkMin != 100 {
		t.Fatalf("spread = %v, want 100", result.SpreadWorkMin)
	}
}

func TestComputeEfficiencyV2Fusion_ThreeBaselinesFusedWithWeights(t *testing.T) {
	algo := 100.0
	knn := 200.0
	llm := 400.0
	result := ComputeEfficiencyV2Fusion(models.Need{}, EfficiencyV2FusionInputs{
		AlgoMin: &algo,
		KNNMin:  &knn,
		LLMMin:  &llm,
		Weights: EfficiencyV2BaselineDefaults{WeightAlgo: 0.3, WeightKNN: 0.45, WeightLLM: 0.25, TeamWorkDensity: 0.25},
	}, EfficiencyV2Config{})
	// expected = 100*0.3 + 200*0.45 + 400*0.25 = 30 + 90 + 100 = 220
	if math.Abs(*result.FusedWorkMin-220) > 1e-6 {
		t.Fatalf("fused = %.3f, want 220", *result.FusedWorkMin)
	}
}

func TestComputeEfficiencyV2Fusion_AllBaselinesMissingReturnsReason(t *testing.T) {
	result := ComputeEfficiencyV2Fusion(models.Need{}, EfficiencyV2FusionInputs{
		Weights: EfficiencyV2BaselineDefaults{WeightAlgo: 0.3, WeightKNN: 0.45, WeightLLM: 0.25, TeamWorkDensity: 0.25},
	}, EfficiencyV2Config{})
	if result.FusedWorkMin != nil {
		t.Fatalf("fused should be nil when no baselines available")
	}
	if result.ConfidenceLevel != efficiencyV2ConfidenceUnknown {
		t.Fatalf("confidence = %q, want unknown", result.ConfidenceLevel)
	}
	joined := strings.Join(result.Reasons, "|")
	if !strings.Contains(joined, "no_baselines") {
		t.Fatalf("reasons should mention no_baselines: %v", result.Reasons)
	}
}

func TestComputeEfficiencyV2Fusion_RatioNotClipped(t *testing.T) {
	algo := 600.0
	knn := 600.0
	need := models.Need{TotalCalendarMin: 12000, TotalActiveWorkCorrectedMin: 100} // wildly exceeds baseline
	result := ComputeEfficiencyV2Fusion(need, EfficiencyV2FusionInputs{
		AlgoMin: &algo,
		KNNMin:  &knn,
		Weights: EfficiencyV2BaselineDefaults{WeightAlgo: 0.5, WeightKNN: 0.5, TeamWorkDensity: 0.25},
	}, EfficiencyV2Config{})
	if result.EfficiencyRatio == nil {
		t.Fatalf("ratio should be set")
	}
	// fused = 600, calendar = 600/0.25 = 2400. ratio = (2400 - 12000)/12000 = -0.8
	if *result.EfficiencyRatio > -0.7 || *result.EfficiencyRatio < -0.9 {
		t.Fatalf("ratio = %.3f, want about -0.8 (not clipped)", *result.EfficiencyRatio)
	}
}

func TestComputeEfficiencyV2Fusion_OutlierHighFlag(t *testing.T) {
	algo := 100.0
	knn := 100.0
	need := models.Need{TotalCalendarMin: 60, TotalActiveWorkCorrectedMin: 600} // 6x baseline
	result := ComputeEfficiencyV2Fusion(need, EfficiencyV2FusionInputs{
		AlgoMin: &algo,
		KNNMin:  &knn,
		Weights: EfficiencyV2BaselineDefaults{WeightAlgo: 0.5, WeightKNN: 0.5, TeamWorkDensity: 0.25},
	}, EfficiencyV2Config{})
	if !result.OutlierFlag {
		t.Fatalf("outlier flag should be true for >5x baseline")
	}
}

func TestComputeEfficiencyV2Fusion_OutlierLowFlag(t *testing.T) {
	algo := 100.0
	knn := 100.0
	need := models.Need{TotalCalendarMin: 60, TotalActiveWorkCorrectedMin: 5} // 0.05 of baseline
	result := ComputeEfficiencyV2Fusion(need, EfficiencyV2FusionInputs{
		AlgoMin: &algo,
		KNNMin:  &knn,
		Weights: EfficiencyV2BaselineDefaults{WeightAlgo: 0.5, WeightKNN: 0.5, TeamWorkDensity: 0.25},
	}, EfficiencyV2Config{})
	if !result.OutlierFlag {
		t.Fatalf("outlier flag should be true for <0.1x baseline")
	}
}

func TestClassifyEfficiencyV2Confidence_LowOnLowSilica(t *testing.T) {
	silica := 0.10
	need := models.Need{Silica: &silica}
	fused := 100.0
	spread := 5.0
	result := EfficiencyV2FusionResult{FusedWorkMin: &fused, SpreadWorkMin: &spread}
	cfg := EfficiencyV2Config{}
	got := classifyEfficiencyV2Confidence(need, result, 3, cfg)
	if got != efficiencyV2StageConfidenceLow {
		t.Fatalf("confidence = %q, want low (silica below threshold)", got)
	}
}

func TestClassifyEfficiencyV2Confidence_HighOnHealthySignals(t *testing.T) {
	silica := 0.8
	aiRatio := 0.7
	uncov := 0.05
	need := models.Need{Silica: &silica, AICodeRatio: &aiRatio, UncoveredWorkRatio: &uncov}
	fused := 100.0
	spread := 5.0 // 5% — under HighSpreadRatioMax 15%
	result := EfficiencyV2FusionResult{FusedWorkMin: &fused, SpreadWorkMin: &spread}
	got := classifyEfficiencyV2Confidence(need, result, 3, EfficiencyV2Config{})
	if got != efficiencyV2StageConfidenceHigh {
		t.Fatalf("confidence = %q, want high", got)
	}
}

func TestClassifyEfficiencyV2Confidence_MediumOnModerateSpread(t *testing.T) {
	silica := 0.8
	aiRatio := 0.7
	uncov := 0.05
	need := models.Need{Silica: &silica, AICodeRatio: &aiRatio, UncoveredWorkRatio: &uncov}
	fused := 100.0
	spread := 20.0 // 20% — within MediumSpreadRatioMax 30%
	result := EfficiencyV2FusionResult{FusedWorkMin: &fused, SpreadWorkMin: &spread}
	got := classifyEfficiencyV2Confidence(need, result, 3, EfficiencyV2Config{})
	if got != efficiencyV2StageConfidenceMedium {
		t.Fatalf("confidence = %q, want medium", got)
	}
}

func TestPersistEfficiencyV2FusionOnNeed_AssignsBandFields(t *testing.T) {
	need := models.Need{NeedId: "n-1"}
	fused := 100.0
	calendar := 400.0
	ratio := 0.5
	low := 0.4
	high := 0.6
	result := EfficiencyV2FusionResult{
		FusedWorkMin:    &fused,
		CalendarMin:     &calendar,
		EfficiencyRatio: &ratio,
		EfficiencyLow:   &low,
		EfficiencyHigh:  &high,
		ConfidenceLevel: efficiencyV2StageConfidenceHigh,
	}
	PersistEfficiencyV2FusionOnNeed(&need, result, EfficiencyV2Config{})
	if need.EfficiencyRatio == nil || *need.EfficiencyRatio != 0.5 {
		t.Fatalf("efficiency_ratio = %v, want 0.5", need.EfficiencyRatio)
	}
	if need.EfficiencyLowerBand == nil || *need.EfficiencyLowerBand != 0.4 {
		t.Fatalf("low band = %v, want 0.4", need.EfficiencyLowerBand)
	}
	if need.ConfidenceLevel != efficiencyV2StageConfidenceHigh {
		t.Fatalf("confidence level = %q, want high", need.ConfidenceLevel)
	}
}

func TestPersistEfficiencyV2FusionOnNeed_IdempotentReasonRerun(t *testing.T) {
	need := models.Need{NeedId: "n-1", Reason: "need span exceeds max_need_span_days=30"}
	result := EfficiencyV2FusionResult{Reasons: []string{"fusion:single_baseline", "outlier:actual_to_baseline=8.5"}}
	PersistEfficiencyV2FusionOnNeed(&need, result, EfficiencyV2Config{})
	first := need.Reason
	// Rerun: simulates reload from DB followed by fresh fusion.
	PersistEfficiencyV2FusionOnNeed(&need, result, EfficiencyV2Config{})
	if need.Reason != first {
		t.Fatalf("rerun changed reason:\nfirst=%q\nsecond=%q", first, need.Reason)
	}
}

func TestComputeEfficiencyV2Fusion_SpreadExceedsFusedHandlesGracefully(t *testing.T) {
	// Three baselines with extreme spread so spread/2 > fused.
	algo := 10.0
	knn := 100.0
	llm := 5000.0 // spread = 4990, fused ≈ 1370 with equal weights
	need := models.Need{TotalCalendarMin: 240}
	result := ComputeEfficiencyV2Fusion(need, EfficiencyV2FusionInputs{
		AlgoMin: &algo,
		KNNMin:  &knn,
		LLMMin:  &llm,
		Weights: EfficiencyV2BaselineDefaults{WeightAlgo: 1.0 / 3, WeightKNN: 1.0 / 3, WeightLLM: 1.0 / 3, TeamWorkDensity: 0.25},
	}, EfficiencyV2Config{})
	// 设计 §Step 6：下界 = baseline 偏低时的 eff，spread > fused 时
	// baseline_low = fused - spread/2 < 0 → 下界 nil
	if result.EfficiencyLow != nil {
		t.Fatalf("efficiency_band_low should be nil when spread exceeds fused, got %v", *result.EfficiencyLow)
	}
	joined := strings.Join(result.Reasons, "|")
	if !strings.Contains(joined, "spread_exceeds_fused") {
		t.Fatalf("reasons should explain missing band, got %v", result.Reasons)
	}
}

func TestComputeEfficiencyV2WeeklyHoldOutError_WithoutAnchorsReturnsNaN(t *testing.T) {
	algo, knn, llm := ComputeEfficiencyV2WeeklyHoldOutError(nil)
	if !math.IsNaN(algo) || !math.IsNaN(knn) || !math.IsNaN(llm) {
		t.Fatalf("expected NaN for empty anchors, got %v/%v/%v", algo, knn, llm)
	}
}

func TestComputeEfficiencyV2WeeklyHoldOutError_PlaceholderReturnsNaN(t *testing.T) {
	// Until anchor partitioning is implemented the function is a placeholder
	// returning NaN so callers persist NULL rather than misleading numbers.
	anchors := []EfficiencyV2KNNAnchor{
		{AnchorID: "a1", WithoutAIMinutes: 100},
		{AnchorID: "a2", WithoutAIMinutes: 200},
	}
	algo, knn, llm := ComputeEfficiencyV2WeeklyHoldOutError(anchors)
	if !math.IsNaN(algo) || !math.IsNaN(knn) || !math.IsNaN(llm) {
		t.Fatalf("placeholder should return NaN, got %v/%v/%v", algo, knn, llm)
	}
}
