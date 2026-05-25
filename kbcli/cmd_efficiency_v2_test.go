package main

import (
	"testing"
)

func TestParseEfficiencyV2DateParams_DateOnlyExpandsToRange(t *testing.T) {
	start, end, err := ParseEfficiencyV2DateParams("", "", "20260518")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if start != "2026-05-18" || end != "2026-05-18" {
		t.Fatalf("date-only should produce range [2026-05-18, 2026-05-18], got [%s, %s]", start, end)
	}
}

func TestParseEfficiencyV2DateParams_StartEndDifferentFormats(t *testing.T) {
	start, end, err := ParseEfficiencyV2DateParams("20260101", "2026-12-31", "")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if start != "2026-01-01" || end != "2026-12-31" {
		t.Fatalf("got [%s, %s]", start, end)
	}
}

func TestParseEfficiencyV2DateParams_RangeInvalidOrder(t *testing.T) {
	if _, _, err := ParseEfficiencyV2DateParams("20261231", "20260101", ""); err == nil {
		t.Fatalf("expected error when start > end")
	}
}

func TestParseEfficiencyV2DateParams_InvalidDateFormatErrors(t *testing.T) {
	if _, _, err := ParseEfficiencyV2DateParams("", "", "not-a-date"); err == nil {
		t.Fatalf("expected error for bad date")
	}
}

func TestEfficiencyV2CommandRegistered(t *testing.T) {
	if !validTaskTypes["efficiency-v2"] {
		t.Fatalf("efficiency-v2 should be a registered task type")
	}
}
