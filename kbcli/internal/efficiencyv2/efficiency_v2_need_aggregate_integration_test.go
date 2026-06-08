//go:build integration

package efficiencyv2

import (
	"testing"
)

func TestEfficiencyV2NeedActualsAggregation_PopulatesActualSignalFields(t *testing.T) {
	db := openEfficiencyV2IntegrationDB(t)
	fixture := BuildEfficiencyV2Fixture()
	cfg := Config{}
	applyEfficiencyV2Defaults(&cfg)

	if err := SeedEfficiencyV2Fixture(db, fixture); err != nil {
		t.Fatalf("seed fixture: %v", err)
	}
	events, err := NormalizeAndUpsertEfficiencyV2ConversationEvents(db, EfficiencyV2ConversationEventQuery{
		StartDate: "2026-05-18",
		EndDate:   "2026-05-24",
	})
	if err != nil {
		t.Fatalf("normalize/upsert events: %v", err)
	}
	if _, err := BuildAndUpsertEfficiencyV2SessionStageMetrics(db, events, cfg.EfficiencyV2); err != nil {
		t.Fatalf("build/upsert stage metrics: %v", err)
	}
	needs, err := ResolveAndUpsertEfficiencyV2Needs(db, cfg.EfficiencyV2, "", "")
	if err != nil {
		t.Fatalf("resolve needs: %v", err)
	}
	if len(needs) == 0 {
		t.Fatal("expected needs from fixture")
	}

	updated, err := AggregateAndUpsertEfficiencyV2NeedActuals(db, needs, cfg.EfficiencyV2, cfg.AlgoEstimation)
	if err != nil {
		t.Fatalf("aggregate need actuals: %v", err)
	}
	if len(updated) != len(needs) {
		t.Fatalf("aggregated need count = %d, want %d", len(updated), len(needs))
	}

	branchNeed := loadEfficiencyV2Need(t, db, efficiencyV2BoundaryBranch, "branch:git@example.com/acme/billing.git:feature/invoice-export")
	if branchNeed.CommitCount != 2 {
		t.Fatalf("branch need commit_count = %d, want 2", branchNeed.CommitCount)
	}
	if branchNeed.UncoveredLoc <= 0 {
		t.Fatalf("expected uncovered_loc > 0 for branch-low-ai-uncovered, got %d", branchNeed.UncoveredLoc)
	}
	uncoveredIDs := EfficiencyV2StringsFromJSON(branchNeed.UncoveredCommitIds)
	if len(uncoveredIDs) != 1 || uncoveredIDs[0] != "c-branch-201-uncovered" {
		t.Fatalf("uncovered ids = %v, want [c-branch-201-uncovered]", uncoveredIDs)
	}
	if branchNeed.UncoveredHumanMin <= 0 {
		t.Fatalf("uncovered_human_min should be > 0, got %.2f", branchNeed.UncoveredHumanMin)
	}
	if branchNeed.TotalActiveWorkCorrectedMin <= branchNeed.TotalSessionActivePersonMin {
		t.Fatalf("active_work_corrected (%.2f) should exceed person time (%.2f) when uncovered work exists", branchNeed.TotalActiveWorkCorrectedMin, branchNeed.TotalSessionActivePersonMin)
	}
	if branchNeed.UncoveredWorkSignal != efficiencyV2SignalLow {
		t.Fatalf("uncovered_work_signal = %q, want %q", branchNeed.UncoveredWorkSignal, efficiencyV2SignalLow)
	}
	if branchNeed.SilicaSignal != efficiencyV2SignalLow {
		t.Fatalf("silica_signal = %q, want low for low-ai scenario", branchNeed.SilicaSignal)
	}

	prNeed := loadEfficiencyV2Need(t, db, efficiencyV2BoundaryPR, "pr:101")
	if prNeed.TotalSessionActivePersonMin <= 0 {
		t.Fatalf("pr need person_min should be > 0, got %.2f", prNeed.TotalSessionActivePersonMin)
	}
	if prNeed.TotalWallMin <= 0 {
		t.Fatalf("pr need wall_min should be > 0, got %.2f", prNeed.TotalWallMin)
	}
	if prNeed.TotalCalendarMin <= 0 {
		t.Fatalf("pr need calendar_min should be > 0, got %.2f", prNeed.TotalCalendarMin)
	}
	if prNeed.UncoveredLoc != 0 {
		t.Fatalf("pr need uncovered_loc = %d, want 0 (covered commit)", prNeed.UncoveredLoc)
	}

	// Rerun must remain idempotent.
	if _, err := AggregateAndUpsertEfficiencyV2NeedActuals(db, needs, cfg.EfficiencyV2, cfg.AlgoEstimation); err != nil {
		t.Fatalf("rerun aggregate: %v", err)
	}
	if err := assertEfficiencyV2RowsUnique(t, db, "needs", "need_id"); err != nil {
		t.Fatal(err)
	}
}
