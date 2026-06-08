package efficiencyv2

import "testing"

func TestBuildEfficiencyV2Fixture_CoversRequiredDimensions(t *testing.T) {
	fixture := BuildEfficiencyV2Fixture()
	if err := fixture.Validate(); err != nil {
		t.Fatalf("fixture should be valid: %v", err)
	}
	if len(fixture.Scenarios) < 6 {
		t.Fatalf("expected at least 6 scenarios, got %d", len(fixture.Scenarios))
	}
}

func TestBuildEfficiencyV2FixtureManifest_IncludesSeedableMetadata(t *testing.T) {
	fixture := BuildEfficiencyV2Fixture()
	records, err := BuildEfficiencyV2FixtureManifest(fixture)
	if err != nil {
		t.Fatalf("manifest should build: %v", err)
	}
	counts := map[string]int{}
	for _, record := range records {
		counts[record.Kind]++
		if record.Name == "" {
			t.Fatalf("manifest record has empty name: %#v", record)
		}
		if record.Payload == "" || record.Payload == "{}" {
			t.Fatalf("manifest record has empty payload: %#v", record)
		}
	}
	for _, kind := range []string{"scenario", "boundary_evidence", "baseline_variant", "anchor", "coefficient", "assertion"} {
		if counts[kind] == 0 {
			t.Fatalf("manifest missing kind %q; counts=%v", kind, counts)
		}
	}
}

func TestBuildEfficiencyV2Fixture_ContainsRawBoundaryEvidence(t *testing.T) {
	fixture := BuildEfficiencyV2Fixture()
	if err := fixture.Validate(); err != nil {
		t.Fatalf("fixture should be valid: %v", err)
	}
	seen := map[string]bool{}
	for _, scenario := range fixture.Scenarios {
		switch scenario.BoundarySource {
		case efficiencyV2BoundaryPR:
			seen["pr"] = scenario.BoundaryEvidence.PRID != ""
		case efficiencyV2BoundaryBranch:
			seen["branch"] = scenario.BoundaryEvidence.BranchName != "" && !isMainlineBranch(scenario.BoundaryEvidence.BranchName)
		case efficiencyV2BoundaryIssue:
			seen["issue"] = scenario.BoundaryEvidence.IssueID != ""
		case efficiencyV2BoundaryFileCluster:
			seen["file-cluster"] = len(scenario.BoundaryEvidence.FilePaths) >= 2
		case efficiencyV2BoundaryOrphan:
			seen["orphan"] = scenario.BoundaryEvidence.IsOrphan
		}
	}
	for _, key := range []string{"pr", "branch", "issue", "file-cluster", "orphan"} {
		if !seen[key] {
			t.Fatalf("missing raw boundary evidence for %s: %#v", key, seen)
		}
	}
}

func TestBuildEfficiencyV2Fixture_CoversBaselineVariants(t *testing.T) {
	fixture := BuildEfficiencyV2Fixture()
	if err := fixture.Validate(); err != nil {
		t.Fatalf("fixture should be valid: %v", err)
	}
	var seededAnchor, emptyAnchor, llmSuccess, llmDisabled, llmFailed, aOnly, ab, abc bool
	for _, scenario := range fixture.Scenarios {
		seededAnchor = seededAnchor || scenario.BaselineVariant.AnchorMode == "seeded"
		emptyAnchor = emptyAnchor || scenario.BaselineVariant.AnchorMode == "empty"
		llmSuccess = llmSuccess || scenario.BaselineVariant.LLMMode == "success"
		llmDisabled = llmDisabled || scenario.BaselineVariant.LLMMode == "disabled"
		llmFailed = llmFailed || scenario.BaselineVariant.LLMMode == "failed"
		aOnly = aOnly || stringSetEqual(scenario.BaselineVariant.AvailableMethods, []string{"A"})
		ab = ab || stringSetEqual(scenario.BaselineVariant.AvailableMethods, []string{"A", "B"})
		abc = abc || stringSetEqual(scenario.BaselineVariant.AvailableMethods, []string{"A", "B", "C"})
	}
	for name, ok := range map[string]bool{
		"seeded anchor": seededAnchor,
		"empty anchor":  emptyAnchor,
		"llm success":   llmSuccess,
		"llm disabled":  llmDisabled,
		"llm failed":    llmFailed,
		"A only":        aOnly,
		"A+B":           ab,
		"A+B+C":         abc,
	} {
		if !ok {
			t.Fatalf("missing baseline variant %s", name)
		}
	}
}

func TestEfficiencyV2E2EHarnessPlan_Shape(t *testing.T) {
	steps := EfficiencyV2E2EHarnessPlan()
	want := []string{
		"fixture-setup",
		"legacy-efficiency",
		"run-efficiency-v2",
		"db-assertions",
		"api-assertions",
	}
	if len(steps) != len(want) {
		t.Fatalf("expected %d steps, got %d", len(want), len(steps))
	}
	for i, step := range steps {
		if step.Name != want[i] {
			t.Fatalf("step[%d] = %q, want %q", i, step.Name, want[i])
		}
		if !step.Required {
			t.Fatalf("step %q should be required", step.Name)
		}
		if step.Description == "" {
			t.Fatalf("step %q should have a description", step.Name)
		}
	}
}

func TestEfficiencyV2CurrentE2EFailurePoint(t *testing.T) {
	got := EfficiencyV2CurrentE2EFailurePoint()
	if got != "run-efficiency-v2" {
		t.Fatalf("current failure point = %q, want run-efficiency-v2", got)
	}
}

func TestEfficiencyV2E2EHarness_ValidateSpine(t *testing.T) {
	harness := NewEfficiencyV2E2EHarness()
	if err := harness.ValidateSpine(); err != nil {
		t.Fatalf("harness spine should be valid: %v", err)
	}
}
