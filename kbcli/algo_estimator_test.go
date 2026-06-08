package main

import (
	"fmt"
	"kanban/kbcli/internal/estimator"
	"strings"
	"testing"
)

func setupTestCfg(ec estimator.EstimateConfig) func() {
	oldCfg := cfg
	cfg = &Config{AlgoEstimation: ec}
	return func() { cfg = oldCfg }
}

func TestEstimateCommitAncientMinutes_ZeroOrNegative(t *testing.T) {
	defer setupTestCfg(estimator.EstimateConfig{
		MinMinutes:           5,
		CommitLinePerMinutes: 100.0 / 480.0,
	})()

	tests := []struct {
		name      string
		diffLines int
	}{
		{"zero", 0},
		{"negative", -5},
		{"negative large", -100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			minutes, reason := estimateCommitAncientMinutes(tt.diffLines)
			if minutes != 5 {
				t.Errorf("expected minutes=5, got %v", minutes)
			}
			if reason != "默认估算:无代码变更" {
				t.Errorf("expected reason=默认估算:无代码变更, got %s", reason)
			}
		})
	}
}

func TestEstimateCommitAncientMinutes_CalculatedValue(t *testing.T) {
	defer setupTestCfg(estimator.EstimateConfig{
		MinMinutes:           5,
		CommitLinePerMinutes: 100.0 / 480.0, // ≈ 0.20833
	})()

	// diffLines=10, rate=0.20833 → 10 / 0.20833 ≈ 48.0 minutes, > MinMinutes
	minutes, reason := estimateCommitAncientMinutes(10)
	expectedMinutes := 10.0 / (100.0 / 480.0) // = 48.0
	if minutes != expectedMinutes {
		t.Errorf("expected minutes=%v, got %v", expectedMinutes, minutes)
	}
	expectedReason := fmt.Sprintf("基于diff_lines=%d估算(%.2f行/分钟)", 10, 100.0/480.0)
	if reason != expectedReason {
		t.Errorf("expected reason=%s, got %s", expectedReason, reason)
	}
}

func TestEstimateCommitAncientMinutes_ClampsToMinMinutes(t *testing.T) {
	defer setupTestCfg(estimator.EstimateConfig{
		MinMinutes:           5,
		CommitLinePerMinutes: 100.0 / 480.0, // ≈ 0.20833
	})()

	// diffLines=1, rate=0.20833 → 1 / 0.20833 ≈ 4.8 minutes, < MinMinutes(5)
	minutes, reason := estimateCommitAncientMinutes(1)
	if minutes != 5 {
		t.Errorf("expected minutes clamped to 5, got %v", minutes)
	}
	if !strings.HasPrefix(reason, "基于diff_lines=1估算") {
		t.Errorf("unexpected reason format: %s", reason)
	}
}

func TestEstimateCommitAncientMinutes_LargeDiffNoUpperClamp(t *testing.T) {
	defer setupTestCfg(estimator.EstimateConfig{
		MinMinutes:           5,
		CommitLinePerMinutes: 100.0 / 480.0,
	})()

	// Large diff lines should not be clamped at the upper end
	minutes, _ := estimateCommitAncientMinutes(10000)
	expectedMinutes := 10000.0 / (100.0 / 480.0) // = 48000
	if minutes != expectedMinutes {
		t.Errorf("expected minutes=%v, got %v", expectedMinutes, minutes)
	}
}

func TestEstimateCommitAncientMinutes_DifferentRates(t *testing.T) {
	tests := []struct {
		name                 string
		commitLinePerMinutes float64
		diffLines            int
		minMinutes           float64
		expectedMinutes      float64
	}{
		{
			name:                 "fast rate 1 line per minute",
			commitLinePerMinutes: 1.0,
			diffLines:            10,
			minMinutes:           5,
			expectedMinutes:      10.0,
		},
		{
			name:                 "slow rate 0.1 line per minute",
			commitLinePerMinutes: 0.1,
			diffLines:            10,
			minMinutes:           5,
			expectedMinutes:      100.0,
		},
		{
			name:                 "rate exactly at boundary",
			commitLinePerMinutes: 2.0,
			diffLines:            10,
			minMinutes:           5,
			expectedMinutes:      5.0, // 10/2 = 5, exactly at min
		},
		{
			name:                 "rate above boundary",
			commitLinePerMinutes: 5.0,
			diffLines:            10,
			minMinutes:           5,
			expectedMinutes:      5.0, // 10/5 = 2, clamped to MinMinutes=5
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer setupTestCfg(estimator.EstimateConfig{
				MinMinutes:           tt.minMinutes,
				CommitLinePerMinutes: tt.commitLinePerMinutes,
			})()

			minutes, _ := estimateCommitAncientMinutes(tt.diffLines)
			if minutes != tt.expectedMinutes {
				t.Errorf("expected minutes=%v, got %v", tt.expectedMinutes, minutes)
			}
		})
	}
}

func TestEstimateCommitAncientMinutes_ReasonFormat(t *testing.T) {
	defer setupTestCfg(estimator.EstimateConfig{
		MinMinutes:           5,
		CommitLinePerMinutes: 2.5,
	})()

	_, reason := estimateCommitAncientMinutes(42)
	expectedReason := fmt.Sprintf("基于diff_lines=%d估算(%.2f行/分钟)", 42, 2.5)
	if reason != expectedReason {
		t.Errorf("expected reason=%s, got %s", expectedReason, reason)
	}
}

func TestEstimateCommitAncientMinutes_MinMinutesZero(t *testing.T) {
	defer setupTestCfg(estimator.EstimateConfig{
		MinMinutes:           0,
		CommitLinePerMinutes: 1.0,
	})()

	minutes, reason := estimateCommitAncientMinutes(0)
	if minutes != 0 {
		t.Errorf("expected minutes=0, got %v", minutes)
	}
	if reason != "默认估算:无代码变更" {
		t.Errorf("unexpected reason for zero diffLines: %s", reason)
	}

	// Small positive diffLines with MinMinutes=0 should not be clamped up
	minutes, reason = estimateCommitAncientMinutes(1)
	if minutes != 1.0 {
		t.Errorf("expected minutes=1.0, got %v", minutes)
	}
	if strings.HasPrefix(reason, "默认估算") {
		t.Errorf("expected calculated reason, got default: %s", reason)
	}
}
