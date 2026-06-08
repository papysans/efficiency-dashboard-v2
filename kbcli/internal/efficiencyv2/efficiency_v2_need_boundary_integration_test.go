//go:build integration

package efficiencyv2

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"

	"kanban/core/models"

	"gorm.io/gorm"
)

func TestEfficiencyV2NeedBoundaryResolution_PersistsFixtureNeeds(t *testing.T) {
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
		t.Fatalf("resolve/upsert needs: %v", err)
	}
	if len(needs) == 0 {
		t.Fatal("expected resolved needs")
	}
	assertEfficiencyV2FixtureNeeds(t, db, fixture)

	firstNeedIDsByBoundary := loadEfficiencyV2NeedIDsByBoundary(t, db)
	needsAgain, err := ResolveAndUpsertEfficiencyV2Needs(db, cfg.EfficiencyV2, "", "")
	if err != nil {
		t.Fatalf("rerun resolve/upsert needs: %v", err)
	}
	if len(needsAgain) != len(needs) {
		t.Fatalf("rerun resolved %d needs, want %d", len(needsAgain), len(needs))
	}
	assertEfficiencyV2FixtureNeeds(t, db, fixture)
	assertEfficiencyV2RowsUnique(t, db, "needs", "need_id")
	secondNeedIDsByBoundary := loadEfficiencyV2NeedIDsByBoundary(t, db)
	if !reflect.DeepEqual(secondNeedIDsByBoundary, firstNeedIDsByBoundary) {
		t.Fatalf("rerun changed need ids by boundary:\nfirst=%#v\nsecond=%#v", firstNeedIDsByBoundary, secondNeedIDsByBoundary)
	}
}

func assertEfficiencyV2FixtureNeeds(t *testing.T, db *gorm.DB, fixture EfficiencyV2Fixture) {
	t.Helper()
	for _, scenario := range fixture.Scenarios {
		need := loadEfficiencyV2Need(t, db, scenario.BoundarySource, scenario.NeedID)
		if need.NeedId != scenario.NeedID {
			t.Fatalf("%s need_id = %q, want %q", scenario.Name, need.NeedId, scenario.NeedID)
		}
		if need.BoundaryConfidence != scenario.BoundaryConfidence {
			t.Fatalf("%s boundary_confidence = %q, want %q", scenario.Name, need.BoundaryConfidence, scenario.BoundaryConfidence)
		}
		if need.Status != scenario.Status {
			t.Fatalf("%s status = %q, want %q", scenario.Name, need.Status, scenario.Status)
		}
		if need.RepoAddr != scenario.RepoAddr {
			t.Fatalf("%s repo_addr = %q, want %q", scenario.Name, need.RepoAddr, scenario.RepoAddr)
		}
		if need.RepoBranch != scenario.RepoBranch {
			t.Fatalf("%s repo_branch = %q, want %q", scenario.Name, need.RepoBranch, scenario.RepoBranch)
		}
		if need.PrimaryUserId != scenario.PrimaryUserID {
			t.Fatalf("%s primary_user_id = %q, want %q", scenario.Name, need.PrimaryUserId, scenario.PrimaryUserID)
		}
		assertEfficiencyV2StringJSONSet(t, scenario.Name, "session_ids", need.SessionIds, scenario.SessionIDs)
		assertEfficiencyV2StringJSONSet(t, scenario.Name, "commit_ids", need.CommitIds, scenario.CommitIDs)
		assertEfficiencyV2StringJSONSet(t, scenario.Name, "contributor_user_ids", need.ContributorUserIds, scenario.ContributorUserIDs)
		wantCoverageEligible := scenario.Status == "merged" &&
			(scenario.BoundaryConfidence == efficiencyV2ConfidenceHigh || scenario.BoundaryConfidence == efficiencyV2ConfidenceMedium)
		if need.CoverageEligible != wantCoverageEligible {
			t.Fatalf("%s coverage_eligible = %v, want %v", scenario.Name, need.CoverageEligible, wantCoverageEligible)
		}
	}
}

func loadEfficiencyV2Need(t *testing.T, db *gorm.DB, boundarySource, needID string) models.Need {
	t.Helper()
	var need models.Need
	if err := db.Where("boundary_source = ? AND need_id = ?", boundarySource, needID).First(&need).Error; err != nil {
		t.Fatalf("load need %s/%s: %v", boundarySource, needID, err)
	}
	return need
}

func loadEfficiencyV2NeedIDsByBoundary(t *testing.T, db *gorm.DB) map[string][]string {
	t.Helper()
	var rows []struct {
		BoundarySource string
		NeedID         string
	}
	if err := db.Table("needs").
		Select("boundary_source, need_id").
		Order("boundary_source ASC").
		Order("need_id ASC").
		Scan(&rows).Error; err != nil {
		t.Fatalf("load need ids by boundary: %v", err)
	}
	result := map[string][]string{}
	for _, row := range rows {
		result[row.BoundarySource] = append(result[row.BoundarySource], row.NeedID)
	}
	return result
}

func assertEfficiencyV2StringJSONSet(t *testing.T, scenarioName, field string, got models.StringJSON, want []string) {
	t.Helper()
	gotValues := decodeEfficiencyV2StringJSON(t, got)
	sort.Strings(gotValues)
	wantValues := append([]string(nil), want...)
	if wantValues == nil {
		wantValues = []string{}
	}
	sort.Strings(wantValues)
	if !reflect.DeepEqual(gotValues, wantValues) {
		t.Fatalf("%s %s = %#v, want %#v", scenarioName, field, gotValues, wantValues)
	}
}

func decodeEfficiencyV2StringJSON(t *testing.T, value models.StringJSON) []string {
	t.Helper()
	var result []string
	if err := json.Unmarshal([]byte(value), &result); err != nil {
		t.Fatalf("decode string json %q: %v", string(value), err)
	}
	return result
}
