package main

import (
	"fmt"
	"strings"
	"testing"
)

// defaultTestConfig returns a consistent EstimateConfig for tests.
func defaultTestConfig() *EstimateConfig {
	return &EstimateConfig{
		MaxInputChars:        10000,
		MaxRatio:             3.0,
		MaxFactor:            2.0,
		MinFactor:            0.5,
		IncharsPerMinutes:    100,
		LinesPerMinutes:      10,
		MinMinutes:           5,
		CommitLinePerMinutes: 5,
	}
}

func TestEstimateTaskAncientMinutes_NormalCase(t *testing.T) {
	ec := defaultTestConfig()
	totalInchars := 5000.0
	totalLines := 100.0
	realMinutes := 60.0

	workload, reason := estimateTaskAncientMinutes(ec, totalInchars, totalLines, realMinutes)

	// factor = 0.5 + (5000/10000)*(2.0-0.5) = 0.5 + 0.5*1.5 = 1.25
	// workload = (100/10) * 1.25 = 12.5
	// maxWorkload = 3.0 * 60 = 180
	// minWorkload = max(5, 60) = 60
	// 12.5 < 60, so clamped to 60
	expected := 60.0
	if workload != expected {
		t.Errorf("expected workload %.2f, got %.2f", expected, workload)
	}
	if reason == "" {
		t.Error("expected non-empty reason string")
	}
}

func TestEstimateTaskAncientMinutes_ZeroInchars(t *testing.T) {
	ec := defaultTestConfig()
	totalInchars := 0.0
	totalLines := 100.0
	realMinutes := 60.0

	workload, reason := estimateTaskAncientMinutes(ec, totalInchars, totalLines, realMinutes)

	// factor = MinFactor = 0.5
	// workload = (100/10) * 0.5 = 5.0
	// maxWorkload = 3.0 * 60 = 180
	// minWorkload = max(5, 60) = 60
	// 5.0 < 60, so clamped to 60
	expected := 60.0
	if workload != expected {
		t.Errorf("expected workload %.2f, got %.2f", expected, workload)
	}
	if !strings.Contains(reason, "factor=0.50") {
		t.Errorf("reason should contain factor=0.50, got: %s", reason)
	}
}

func TestEstimateTaskAncientMinutes_MaxInchars(t *testing.T) {
	ec := defaultTestConfig()
	totalInchars := ec.MaxInputChars
	totalLines := 200.0
	realMinutes := 60.0

	workload, reason := estimateTaskAncientMinutes(ec, totalInchars, totalLines, realMinutes)

	// factor = MaxFactor = 2.0
	// workload = (200/10) * 2.0 = 40.0
	// maxWorkload = 3.0 * 60 = 180
	// minWorkload = max(5, 60) = 60
	// 40.0 < 60, so clamped to 60
	expected := 60.0
	if workload != expected {
		t.Errorf("expected workload %.2f, got %.2f", expected, workload)
	}
	if !strings.Contains(reason, "factor=2.00") {
		t.Errorf("reason should contain factor=2.00, got: %s", reason)
	}
}

func TestEstimateTaskAncientMinutes_IncharsExceedsMax(t *testing.T) {
	ec := defaultTestConfig()
	totalInchars := ec.MaxInputChars + 5000
	totalLines := 200.0
	realMinutes := 60.0

	workload, reason := estimateTaskAncientMinutes(ec, totalInchars, totalLines, realMinutes)

	// totalInchars clamped to MaxInputChars
	// factor = MaxFactor = 2.0
	// workload = (200/10) * 2.0 = 40.0
	// maxWorkload = 3.0 * 60 = 180
	// minWorkload = max(5, 60) = 60
	// 40.0 < 60, so clamped to 60
	expected := 60.0
	if workload != expected {
		t.Errorf("expected workload %.2f, got %.2f", expected, workload)
	}
	if !strings.Contains(reason, fmt.Sprintf("user_input=%.0f字符", ec.MaxInputChars)) {
		t.Errorf("reason should contain clamped user_input value %.0f, got: %s", ec.MaxInputChars, reason)
	}
}

func TestEstimateTaskAncientMinutes_ZeroLines(t *testing.T) {
	ec := defaultTestConfig()
	totalInchars := 5000.0
	totalLines := 0.0
	realMinutes := 60.0

	workload, reason := estimateTaskAncientMinutes(ec, totalInchars, totalLines, realMinutes)

	// workload = (0/10) * factor = 0
	// maxWorkload = 3.0 * 60 = 180
	// minWorkload = max(5, 60) = 60
	// 0 < 60, so clamped to 60
	expected := 60.0
	if workload != expected {
		t.Errorf("expected workload %.2f, got %.2f", expected, workload)
	}
	if !strings.Contains(reason, "diff_lines=0") {
		t.Errorf("reason should contain diff_lines=0, got: %s", reason)
	}
}

func TestEstimateTaskAncientMinutes_ZeroRealMinutes(t *testing.T) {
	ec := defaultTestConfig()
	totalInchars := 5000.0
	totalLines := 100.0
	realMinutes := 0.0

	workload, reason := estimateTaskAncientMinutes(ec, totalInchars, totalLines, realMinutes)

	// factor = 1.25
	// workload = (100/10) * 1.25 = 12.5
	// maxWorkload = 3.0 * 0 = 0
	// minWorkload = max(5, 0) = 5
	// workload > maxWorkload (12.5 > 0), clamped to 0
	// then 0 < minWorkload (0 < 5), clamped to 5
	expected := 5.0
	if workload != expected {
		t.Errorf("expected workload %.2f, got %.2f", expected, workload)
	}
	if !strings.Contains(reason, "real_minutes=0.00") {
		t.Errorf("reason should contain real_minutes=0.00, got: %s", reason)
	}
}

func TestEstimateTaskAncientMinutes_WorkloadExceedsMax(t *testing.T) {
	ec := defaultTestConfig()
	totalInchars := 5000.0
	totalLines := 1000.0
	realMinutes := 10.0

	workload, _ := estimateTaskAncientMinutes(ec, totalInchars, totalLines, realMinutes)

	// factor = 1.25
	// workload = (1000/10) * 1.25 = 125.0
	// maxWorkload = 3.0 * 10 = 30
	// minWorkload = max(5, 10) = 10
	// 125 > 30, clamped to 30
	expected := 30.0
	if workload != expected {
		t.Errorf("expected workload %.2f, got %.2f", expected, workload)
	}
}

func TestEstimateTaskAncientMinutes_WorkloadBelowMin(t *testing.T) {
	ec := defaultTestConfig()
	totalInchars := 1000.0
	totalLines := 10.0
	realMinutes := 120.0

	workload, _ := estimateTaskAncientMinutes(ec, totalInchars, totalLines, realMinutes)

	// factor = 0.5 + (1000/10000)*1.5 = 0.5 + 0.15 = 0.65
	// workload = (10/10) * 0.65 = 0.65
	// maxWorkload = 3.0 * 120 = 360
	// minWorkload = max(5, 120) = 120
	// 0.65 < 120, clamped to 120
	expected := 120.0
	if workload != expected {
		t.Errorf("expected workload %.2f, got %.2f", expected, workload)
	}
}

func TestEstimateTaskAncientMinutes_LargeTotalLines(t *testing.T) {
	ec := defaultTestConfig()
	totalInchars := 5000.0
	totalLines := 10000.0
	realMinutes := 30.0

	workload, _ := estimateTaskAncientMinutes(ec, totalInchars, totalLines, realMinutes)

	// factor = 1.25
	// workload = (10000/10) * 1.25 = 1250.0
	// maxWorkload = 3.0 * 30 = 90
	// minWorkload = max(5, 30) = 30
	// 1250 > 90, clamped to 90
	expected := 90.0
	if workload != expected {
		t.Errorf("expected workload %.2f, got %.2f", expected, workload)
	}
}

func TestEstimateTaskAncientMinutes_SmallRealMinutesLargeWorkload(t *testing.T) {
	ec := defaultTestConfig()
	totalInchars := 5000.0
	totalLines := 500.0
	realMinutes := 2.0

	workload, _ := estimateTaskAncientMinutes(ec, totalInchars, totalLines, realMinutes)

	// factor = 1.25
	// workload = (500/10) * 1.25 = 62.5
	// maxWorkload = 3.0 * 2 = 6
	// minWorkload = max(5, 2) = 5
	// 62.5 > 6, clamped to 6
	// 6 > 5, no further clamping
	expected := 6.0
	if workload != expected {
		t.Errorf("expected workload %.2f, got %.2f", expected, workload)
	}
}

func TestEstimateTaskAncientMinutes_ReasonStringFormat(t *testing.T) {
	ec := defaultTestConfig()
	totalInchars := 5000.0
	totalLines := 100.0
	realMinutes := 60.0

	_, reason := estimateTaskAncientMinutes(ec, totalInchars, totalLines, realMinutes)

	expectedParts := []string{
		"基于diff_lines=100",
		"user_input=5000字符",
		"factor=1.25",
		"real_minutes=60.00",
	}
	for _, part := range expectedParts {
		if !strings.Contains(reason, part) {
			t.Errorf("reason should contain %q, got: %s", part, reason)
		}
	}
}

func TestEstimateTaskAncientMinutes_RealMinutesBelowMinMinutes(t *testing.T) {
	ec := defaultTestConfig()
	totalInchars := 5000.0
	totalLines := 100.0
	realMinutes := 3.0

	workload, _ := estimateTaskAncientMinutes(ec, totalInchars, totalLines, realMinutes)

	// factor = 1.25
	// workload = (100/10) * 1.25 = 12.5
	// maxWorkload = 3.0 * 3 = 9
	// minWorkload = max(5, 3) = 5
	// 12.5 > 9, clamped to 9
	// 9 > 5, no further clamping
	expected := 9.0
	if workload != expected {
		t.Errorf("expected workload %.2f, got %.2f", expected, workload)
	}
}
