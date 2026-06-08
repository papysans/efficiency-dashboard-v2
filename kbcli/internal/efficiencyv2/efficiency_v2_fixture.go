package efficiencyv2

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"kanban/core/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	efficiencyV2BoundaryPR          = "lv1_pr"
	efficiencyV2BoundaryBranch      = "lv2_branch"
	efficiencyV2BoundaryIssue       = "lv3_issue"
	efficiencyV2BoundaryFileCluster = "lv4_cluster"
	efficiencyV2BoundaryOrphan      = "lv5_orphan"

	efficiencyV2ConfidenceHigh    = "high"
	efficiencyV2ConfidenceMedium  = "medium"
	efficiencyV2ConfidenceLow     = "low"
	efficiencyV2ConfidenceVeryLow = "very_low"
)

const efficiencyV2FixtureManifestTable = "efficiency_v2_fixture_manifest"

// EfficiencyV2Fixture describes deterministic mock data for the v2 E2E spine.
// It is intentionally independent from v2 tables so the fixture can exist
// before the schema implementation lands.
type EfficiencyV2Fixture struct {
	BaseTime     time.Time
	Scenarios    []EfficiencyV2FixtureScenario
	Anchors      []EfficiencyV2FixtureAnchor
	Coefficients map[string]float64
	Assertions   []EfficiencyV2FixtureAssertion
}

type EfficiencyV2FixtureScenario struct {
	Name                     string
	NeedID                   string
	BoundarySource           string
	BoundaryConfidence       string
	Status                   string
	RepoAddr                 string
	RepoBranch               string
	PrimaryUserID            string
	ContributorUserIDs       []string
	SessionIDs               []string
	CommitIDs                []string
	BoundaryEvidence         EfficiencyV2BoundaryEvidence
	MockFiles                []string
	BaselineVariant          EfficiencyV2BaselineVariant
	HasNoEdit                bool
	HasEditTestEdit          bool
	HasUncoveredCommit       bool
	HasLowAIParticipation    bool
	HasMultiContributor      bool
	HasLongIdleGap           bool
	HasWaitForReview         bool
	HasBaselineFailure       bool
	HasHighEfficiencyOutlier bool
	HasLowEfficiencyOutlier  bool
}

type EfficiencyV2BoundaryEvidence struct {
	PRID           string
	IssueID        string
	BranchName     string
	CommitMessages []string
	FilePaths      []string
	IsOrphan       bool
}

type EfficiencyV2BaselineVariant struct {
	AnchorMode         string
	LLMMode            string
	AvailableMethods   []string
	ExpectedConfidence string
}

type EfficiencyV2FixtureAnchor struct {
	AnchorID         string
	Source           string
	WithoutAIMinutes float64
	FeatureVector    map[string]float64
}

type EfficiencyV2FixtureAssertion struct {
	Name   string
	Target string
	Field  string
	Want   string
}

type EfficiencyV2FixtureManifestRecord struct {
	Kind    string
	Name    string
	Payload string
}

type EfficiencyV2E2EStep struct {
	Name        string
	Description string
	Required    bool
}

type EfficiencyV2E2EHarness struct {
	Fixture EfficiencyV2Fixture
	Steps   []EfficiencyV2E2EStep
}

// BuildEfficiencyV2Fixture returns the local source-of-truth mock catalog used
// by section-1 E2E tests and later pipeline integration tests.
func BuildEfficiencyV2Fixture() EfficiencyV2Fixture {
	base := time.Date(2026, 5, 18, 9, 0, 0, 0, time.UTC)
	return EfficiencyV2Fixture{
		BaseTime: base,
		Scenarios: []EfficiencyV2FixtureScenario{
			{
				Name:               "pr-happy-edit-test-edit",
				NeedID:             "pr:101",
				BoundarySource:     efficiencyV2BoundaryPR,
				BoundaryConfidence: efficiencyV2ConfidenceHigh,
				Status:             "merged",
				RepoAddr:           "git@example.com/acme/auth.git",
				RepoBranch:         "feature/forgot-password",
				PrimaryUserID:      "u-alice",
				ContributorUserIDs: []string{"u-alice"},
				SessionIDs:         []string{"s-pr-101-a"},
				CommitIDs:          []string{"c-pr-101-a"},
				BoundaryEvidence: EfficiencyV2BoundaryEvidence{
					PRID:           "101",
					BranchName:     "feature/forgot-password",
					CommitMessages: []string{"Merge pull request #101 from feature/forgot-password"},
					FilePaths:      []string{"backend/auth/reset.go", "frontend/auth/reset.ts"},
				},
				MockFiles:        []string{"backend/auth/reset.go", "frontend/auth/reset.ts"},
				BaselineVariant:  baselineVariant("seeded", "success", []string{"A", "B", "C"}, efficiencyV2ConfidenceHigh),
				HasEditTestEdit:  true,
				HasWaitForReview: true,
			},
			{
				Name:               "branch-low-ai-uncovered",
				NeedID:             "branch:git@example.com/acme/billing.git:feature/invoice-export",
				BoundarySource:     efficiencyV2BoundaryBranch,
				BoundaryConfidence: efficiencyV2ConfidenceHigh,
				Status:             "merged",
				RepoAddr:           "git@example.com/acme/billing.git",
				RepoBranch:         "feature/invoice-export",
				PrimaryUserID:      "u-bob",
				ContributorUserIDs: []string{"u-bob"},
				SessionIDs:         []string{"s-branch-201-a"},
				CommitIDs:          []string{"c-branch-201-a", "c-branch-201-uncovered"},
				BoundaryEvidence: EfficiencyV2BoundaryEvidence{
					BranchName:     "feature/invoice-export",
					CommitMessages: []string{"add invoice export", "manual uncovered export formatting"},
					FilePaths:      []string{"billing/export.go", "billing/export_test.go"},
				},
				MockFiles:             []string{"billing/export.go", "billing/export_test.go"},
				BaselineVariant:       baselineVariant("seeded", "disabled", []string{"A", "B"}, efficiencyV2ConfidenceMedium),
				HasUncoveredCommit:    true,
				HasLowAIParticipation: true,
			},
			{
				Name:               "issue-multi-contributor-long-idle",
				NeedID:             "issue:TASK-302",
				BoundarySource:     efficiencyV2BoundaryIssue,
				BoundaryConfidence: efficiencyV2ConfidenceMedium,
				Status:             "merged",
				RepoAddr:           "git@example.com/acme/orders.git",
				RepoBranch:         "main",
				PrimaryUserID:      "u-cara",
				ContributorUserIDs: []string{"u-cara", "u-dan"},
				SessionIDs:         []string{"s-issue-302-a", "s-issue-302-b"},
				CommitIDs:          []string{"c-issue-302-a", "c-issue-302-b"},
				BoundaryEvidence: EfficiencyV2BoundaryEvidence{
					IssueID:        "TASK-302",
					BranchName:     "main",
					CommitMessages: []string{"TASK-302 refactor state machine", "TASK-302 add transition tests"},
					FilePaths:      []string{"orders/state.go", "orders/state_test.go", "orders/migrate.go"},
				},
				MockFiles:           []string{"orders/state.go", "orders/state_test.go", "orders/migrate.go"},
				BaselineVariant:     baselineVariant("seeded", "success", []string{"A", "B", "C"}, efficiencyV2ConfidenceMedium),
				HasMultiContributor: true,
				HasLongIdleGap:      true,
			},
			{
				Name:               "file-cluster-active-no-edit",
				NeedID:             "cluster:u-erin:2026w21:auth-docs",
				BoundarySource:     efficiencyV2BoundaryFileCluster,
				BoundaryConfidence: efficiencyV2ConfidenceLow,
				Status:             "active",
				RepoAddr:           "git@example.com/acme/docs.git",
				RepoBranch:         "main",
				PrimaryUserID:      "u-erin",
				ContributorUserIDs: []string{"u-erin"},
				SessionIDs:         []string{"s-cluster-401-a"},
				CommitIDs:          []string{},
				BoundaryEvidence: EfficiencyV2BoundaryEvidence{
					BranchName:     "main",
					CommitMessages: []string{},
					FilePaths:      []string{"docs/auth.md", "docs/login.md", "docs/reset.md"},
				},
				MockFiles:       []string{"docs/auth.md", "docs/login.md", "docs/reset.md"},
				BaselineVariant: baselineVariant("empty", "disabled", []string{"A"}, efficiencyV2ConfidenceLow),
				HasNoEdit:       true,
			},
			{
				Name:               "orphan-abandoned-baseline-failure",
				NeedID:             "orphan:u-frank:2026w21",
				BoundarySource:     efficiencyV2BoundaryOrphan,
				BoundaryConfidence: efficiencyV2ConfidenceVeryLow,
				Status:             "abandoned",
				RepoAddr:           "",
				RepoBranch:         "",
				PrimaryUserID:      "u-frank",
				ContributorUserIDs: []string{"u-frank"},
				SessionIDs:         []string{"s-orphan-501-a"},
				CommitIDs:          []string{},
				BoundaryEvidence: EfficiencyV2BoundaryEvidence{
					IsOrphan:  true,
					FilePaths: []string{},
				},
				MockFiles:               []string{},
				BaselineVariant:         baselineVariant("empty", "failed", []string{}, efficiencyV2ConfidenceLow),
				HasBaselineFailure:      true,
				HasLowEfficiencyOutlier: true,
			},
			{
				Name:               "branch-fast-outlier",
				NeedID:             "branch:git@example.com/acme/ux.git:feature/one-line-fix",
				BoundarySource:     efficiencyV2BoundaryBranch,
				BoundaryConfidence: efficiencyV2ConfidenceHigh,
				Status:             "merged",
				RepoAddr:           "git@example.com/acme/ux.git",
				RepoBranch:         "feature/one-line-fix",
				PrimaryUserID:      "u-gina",
				ContributorUserIDs: []string{"u-gina"},
				SessionIDs:         []string{"s-outlier-601-a"},
				CommitIDs:          []string{"c-outlier-601-a"},
				BoundaryEvidence: EfficiencyV2BoundaryEvidence{
					BranchName:     "feature/one-line-fix",
					CommitMessages: []string{"tiny ui fix"},
					FilePaths:      []string{"ux/button.ts"},
				},
				MockFiles:                []string{"ux/button.ts"},
				BaselineVariant:          baselineVariant("seeded", "failed", []string{"A", "B"}, efficiencyV2ConfidenceLow),
				HasHighEfficiencyOutlier: true,
			},
		},
		Anchors: []EfficiencyV2FixtureAnchor{
			{
				AnchorID:         "anchor-small-feature",
				Source:           "local_fixture",
				WithoutAIMinutes: 240,
				FeatureVector: map[string]float64{
					"loc":   120,
					"files": 4,
					"turns": 35,
				},
			},
			{
				AnchorID:         "anchor-refactor",
				Source:           "local_fixture",
				WithoutAIMinutes: 720,
				FeatureVector: map[string]float64{
					"loc":   360,
					"files": 9,
					"turns": 80,
				},
			},
		},
		Coefficients: map[string]float64{
			"think_user_chars":    0.02,
			"think_files_read":    30,
			"exec_lines_per_min":  0.2,
			"exec_file_coord_min": 30,
			"verify_test_count":   25,
			"team_work_density":   0.25,
			"ai_code_ratio_min":   0.3,
			"uncovered_work_max":  0.3,
		},
		Assertions: []EfficiencyV2FixtureAssertion{
			{Name: "has-pr-boundary", Target: "needs", Field: "boundary_source", Want: efficiencyV2BoundaryPR},
			{Name: "has-branch-boundary", Target: "needs", Field: "boundary_source", Want: efficiencyV2BoundaryBranch},
			{Name: "has-issue-boundary", Target: "needs", Field: "boundary_source", Want: efficiencyV2BoundaryIssue},
			{Name: "has-low-confidence-coverage", Target: "user_productivity_v2", Field: "coverage_low_unreported", Want: ">0"},
			{Name: "has-abandoned-coverage", Target: "user_productivity_v2", Field: "coverage_abandoned", Want: ">0"},
		},
	}
}

func baselineVariant(anchorMode, llmMode string, methods []string, confidence string) EfficiencyV2BaselineVariant {
	return EfficiencyV2BaselineVariant{
		AnchorMode:         anchorMode,
		LLMMode:            llmMode,
		AvailableMethods:   methods,
		ExpectedConfidence: confidence,
	}
}

func (f EfficiencyV2Fixture) Validate() error {
	if f.BaseTime.IsZero() {
		return fmt.Errorf("base time is required")
	}
	if len(f.Scenarios) == 0 {
		return fmt.Errorf("at least one scenario is required")
	}
	requiredBoundaries := map[string]bool{
		efficiencyV2BoundaryPR:          false,
		efficiencyV2BoundaryBranch:      false,
		efficiencyV2BoundaryIssue:       false,
		efficiencyV2BoundaryFileCluster: false,
		efficiencyV2BoundaryOrphan:      false,
	}
	var hasNoEdit, hasEditTestEdit, hasUncovered, hasLowAI, hasMulti, hasIdle, hasWait, hasBaselineFailure, hasHighOutlier, hasLowOutlier bool
	var hasSeededAnchor, hasEmptyAnchor, hasLLMSuccess, hasLLMDisabled, hasLLMFailed, hasAOnly, hasAB, hasABC bool
	for _, s := range f.Scenarios {
		if s.Name == "" || s.NeedID == "" || s.PrimaryUserID == "" {
			return fmt.Errorf("scenario %q is missing required identity fields", s.Name)
		}
		if _, ok := requiredBoundaries[s.BoundarySource]; ok {
			requiredBoundaries[s.BoundarySource] = true
		}
		hasNoEdit = hasNoEdit || s.HasNoEdit
		hasEditTestEdit = hasEditTestEdit || s.HasEditTestEdit
		hasUncovered = hasUncovered || s.HasUncoveredCommit
		hasLowAI = hasLowAI || s.HasLowAIParticipation
		hasMulti = hasMulti || s.HasMultiContributor
		hasIdle = hasIdle || s.HasLongIdleGap
		hasWait = hasWait || s.HasWaitForReview
		hasBaselineFailure = hasBaselineFailure || s.HasBaselineFailure
		hasHighOutlier = hasHighOutlier || s.HasHighEfficiencyOutlier
		hasLowOutlier = hasLowOutlier || s.HasLowEfficiencyOutlier
		hasSeededAnchor = hasSeededAnchor || s.BaselineVariant.AnchorMode == "seeded"
		hasEmptyAnchor = hasEmptyAnchor || s.BaselineVariant.AnchorMode == "empty"
		hasLLMSuccess = hasLLMSuccess || s.BaselineVariant.LLMMode == "success"
		hasLLMDisabled = hasLLMDisabled || s.BaselineVariant.LLMMode == "disabled"
		hasLLMFailed = hasLLMFailed || s.BaselineVariant.LLMMode == "failed"
		hasAOnly = hasAOnly || stringSetEqual(s.BaselineVariant.AvailableMethods, []string{"A"})
		hasAB = hasAB || stringSetEqual(s.BaselineVariant.AvailableMethods, []string{"A", "B"})
		hasABC = hasABC || stringSetEqual(s.BaselineVariant.AvailableMethods, []string{"A", "B", "C"})
		if err := validateBoundaryEvidence(s); err != nil {
			return err
		}
	}
	for boundary, ok := range requiredBoundaries {
		if !ok {
			return fmt.Errorf("missing boundary scenario: %s", boundary)
		}
	}
	requiredFlags := map[string]bool{
		"no-edit":           hasNoEdit,
		"edit-test-edit":    hasEditTestEdit,
		"uncovered-commit":  hasUncovered,
		"low-ai":            hasLowAI,
		"multi-contributor": hasMulti,
		"long-idle":         hasIdle,
		"wait-for-review":   hasWait,
		"baseline-failure":  hasBaselineFailure,
		"high-outlier":      hasHighOutlier,
		"low-outlier":       hasLowOutlier,
		"seeded-anchor":     hasSeededAnchor,
		"empty-anchor":      hasEmptyAnchor,
		"llm-success":       hasLLMSuccess,
		"llm-disabled":      hasLLMDisabled,
		"llm-failed":        hasLLMFailed,
		"a-only-baseline":   hasAOnly,
		"ab-baseline":       hasAB,
		"abc-baseline":      hasABC,
	}
	for name, ok := range requiredFlags {
		if !ok {
			return fmt.Errorf("missing fixture dimension: %s", name)
		}
	}
	if len(f.Anchors) == 0 {
		return fmt.Errorf("at least one anchor is required")
	}
	if len(f.Assertions) == 0 {
		return fmt.Errorf("at least one expected assertion is required")
	}
	return nil
}

func validateBoundaryEvidence(s EfficiencyV2FixtureScenario) error {
	switch s.BoundarySource {
	case efficiencyV2BoundaryPR:
		if s.BoundaryEvidence.PRID == "" {
			return fmt.Errorf("PR scenario %q is missing PR evidence", s.Name)
		}
	case efficiencyV2BoundaryBranch:
		if s.BoundaryEvidence.BranchName == "" || isMainlineBranch(s.BoundaryEvidence.BranchName) {
			return fmt.Errorf("branch scenario %q is missing non-main branch evidence", s.Name)
		}
	case efficiencyV2BoundaryIssue:
		if s.BoundaryEvidence.IssueID == "" {
			return fmt.Errorf("issue scenario %q is missing issue evidence", s.Name)
		}
	case efficiencyV2BoundaryFileCluster:
		if len(s.BoundaryEvidence.FilePaths) < 2 {
			return fmt.Errorf("file-cluster scenario %q needs multiple file evidence paths", s.Name)
		}
	case efficiencyV2BoundaryOrphan:
		if !s.BoundaryEvidence.IsOrphan {
			return fmt.Errorf("orphan scenario %q must mark orphan evidence", s.Name)
		}
	}
	return nil
}

func isMainlineBranch(branch string) bool {
	switch branch {
	case "main", "master", "develop":
		return true
	default:
		return false
	}
}

func stringSetEqual(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	seen := make(map[string]int, len(got))
	for _, v := range got {
		seen[v]++
	}
	for _, v := range want {
		if seen[v] == 0 {
			return false
		}
		seen[v]--
	}
	return true
}

func EfficiencyV2E2EHarnessPlan() []EfficiencyV2E2EStep {
	return []EfficiencyV2E2EStep{
		{Name: "fixture-setup", Description: "generate deterministic raw-ish fixture data", Required: true},
		{Name: "legacy-efficiency", Description: "run legacy efficiency where applicable", Required: true},
		{Name: "run-efficiency-v2", Description: "run kbcli efficiency-v2 for the fixture date range", Required: true},
		{Name: "db-assertions", Description: "assert conversation_events, session_stage_metrics, needs, and user_productivity_v2", Required: true},
		{Name: "api-assertions", Description: "assert /api/v2/needs and /api/v2/efficiency responses", Required: true},
	}
}

func EfficiencyV2CurrentE2EFailurePoint() string {
	return "run-efficiency-v2"
}

func NewEfficiencyV2E2EHarness() EfficiencyV2E2EHarness {
	return EfficiencyV2E2EHarness{
		Fixture: BuildEfficiencyV2Fixture(),
		Steps:   EfficiencyV2E2EHarnessPlan(),
	}
}

func (h EfficiencyV2E2EHarness) ValidateSpine() error {
	if err := h.Fixture.Validate(); err != nil {
		return err
	}
	if len(h.Steps) == 0 {
		return fmt.Errorf("at least one E2E step is required")
	}
	var hasFailurePoint bool
	for _, step := range h.Steps {
		if step.Name == "" || step.Description == "" {
			return fmt.Errorf("E2E step is missing name or description")
		}
		if step.Name == EfficiencyV2CurrentE2EFailurePoint() {
			hasFailurePoint = true
		}
	}
	if !hasFailurePoint {
		return fmt.Errorf("E2E harness is missing current failure point %q", EfficiencyV2CurrentE2EFailurePoint())
	}
	return nil
}

func BuildEfficiencyV2FixtureManifest(fixture EfficiencyV2Fixture) ([]EfficiencyV2FixtureManifestRecord, error) {
	if err := fixture.Validate(); err != nil {
		return nil, err
	}
	var records []EfficiencyV2FixtureManifestRecord
	for _, s := range fixture.Scenarios {
		records = append(records, manifestRecord("scenario", s.Name, s))
		records = append(records, manifestRecord("boundary_evidence", s.Name, s.BoundaryEvidence))
		records = append(records, manifestRecord("baseline_variant", s.Name, s.BaselineVariant))
	}
	for _, anchor := range fixture.Anchors {
		records = append(records, manifestRecord("anchor", anchor.AnchorID, anchor))
	}
	coefficientNames := make([]string, 0, len(fixture.Coefficients))
	for name := range fixture.Coefficients {
		coefficientNames = append(coefficientNames, name)
	}
	sort.Strings(coefficientNames)
	for _, name := range coefficientNames {
		value := fixture.Coefficients[name]
		records = append(records, manifestRecord("coefficient", name, map[string]float64{name: value}))
	}
	for _, assertion := range fixture.Assertions {
		records = append(records, manifestRecord("assertion", assertion.Name, assertion))
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("fixture manifest is empty")
	}
	return records, nil
}

func manifestRecord(kind, name string, payload interface{}) EfficiencyV2FixtureManifestRecord {
	return EfficiencyV2FixtureManifestRecord{
		Kind:    kind,
		Name:    name,
		Payload: fixtureJSON(payload),
	}
}

// SeedEfficiencyV2Fixture seeds both raw-ish legacy data and fixture metadata.
// The manifest table is an E2E/test support table used until dedicated v2
// anchor/coefficient/assertion tables are available.
func SeedEfficiencyV2Fixture(db *gorm.DB, fixture EfficiencyV2Fixture) error {
	if err := SeedEfficiencyV2RawFixture(db, fixture); err != nil {
		return err
	}
	return SeedEfficiencyV2FixtureManifest(db, fixture)
}

func SeedEfficiencyV2FixtureManifest(db *gorm.DB, fixture EfficiencyV2Fixture) error {
	records, err := BuildEfficiencyV2FixtureManifest(fixture)
	if err != nil {
		return err
	}
	if err := db.Exec(fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			kind text NOT NULL,
			name text NOT NULL,
			payload jsonb NOT NULL,
			created_at timestamptz NOT NULL DEFAULT now(),
			updated_at timestamptz NOT NULL DEFAULT now(),
			PRIMARY KEY (kind, name)
		)`, efficiencyV2FixtureManifestTable)).Error; err != nil {
		return fmt.Errorf("create fixture manifest table: %w", err)
	}
	for _, record := range records {
		if err := db.Exec(fmt.Sprintf(`
			INSERT INTO %s (kind, name, payload, updated_at)
			VALUES ($1, $2, $3::jsonb, now())
			ON CONFLICT (kind, name)
			DO UPDATE SET payload = EXCLUDED.payload, updated_at = now()
		`, efficiencyV2FixtureManifestTable), record.Kind, record.Name, record.Payload).Error; err != nil {
			return fmt.Errorf("seed fixture manifest %s/%s: %w", record.Kind, record.Name, err)
		}
	}
	return nil
}

// SeedEfficiencyV2RawFixture seeds existing legacy tables. Later sections extend
// the E2E harness to assert the v2 tables generated from this raw-ish data.
func SeedEfficiencyV2RawFixture(db *gorm.DB, fixture EfficiencyV2Fixture) error {
	if err := fixture.Validate(); err != nil {
		return err
	}
	for idx, s := range fixture.Scenarios {
		if err := seedFixtureUsers(db, s); err != nil {
			return err
		}
		if err := seedFixtureSessionsAndConversations(db, fixture.BaseTime, idx, s); err != nil {
			return err
		}
		if err := seedFixtureCommits(db, fixture.BaseTime, idx, s); err != nil {
			return err
		}
	}
	return nil
}

func seedFixtureUsers(db *gorm.DB, s EfficiencyV2FixtureScenario) error {
	for _, userID := range s.ContributorUserIDs {
		user := models.UserOrg{
			UserId:   userID,
			UserName: "fixture-" + userID,
			Org1:     "Efficiency V2 Fixture",
			Org2:     s.BoundarySource,
		}
		if err := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"user_name", "org1", "org2", "updated_at"}),
		}).Create(&user).Error; err != nil {
			return fmt.Errorf("seed user %s: %w", userID, err)
		}
	}
	return nil
}

func seedFixtureSessionsAndConversations(db *gorm.DB, base time.Time, scenarioIndex int, s EfficiencyV2FixtureScenario) error {
	start := base.Add(time.Duration(scenarioIndex*24) * time.Hour)
	for sessionIndex, sessionID := range s.SessionIDs {
		userID := s.PrimaryUserID
		if sessionIndex < len(s.ContributorUserIDs) {
			userID = s.ContributorUserIDs[sessionIndex]
		}
		sessionStart := start.Add(time.Duration(sessionIndex*15) * time.Minute)
		session := models.Session{
			SessionId:        sessionID,
			CreateTime:       sessionStart,
			UserId:           userID,
			UserName:         "fixture-" + userID,
			ClientId:         "fixture-client",
			ClientIde:        "codex",
			ClientVersion:    "fixture",
			ClientOs:         "linux",
			ClientOsVersion:  "fixture",
			SessionDate:      sessionStart.Format("2006/01/02"),
			ConversationDate: sessionStart.Format("2006/01/02"),
		}
		if err := db.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "session_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"create_time", "user_id", "user_name", "client_id", "client_ide",
				"client_version", "client_os", "client_os_version", "session_date",
				"conversation_date", "updated_at",
			}),
		}).Create(&session).Error; err != nil {
			return fmt.Errorf("seed session %s: %w", sessionID, err)
		}
		if err := seedFixtureConversationRows(db, sessionID, userID, sessionStart, s); err != nil {
			return err
		}
	}
	return nil
}

func seedFixtureConversationRows(db *gorm.DB, sessionID, userID string, start time.Time, s EfficiencyV2FixtureScenario) error {
	rows := []models.Conversation{
		fixtureConversation(sessionID, "r1", userID, start, s, "message", 0),
	}
	if !s.HasNoEdit {
		rows = append(rows, fixtureConversation(sessionID, "r2", userID, start.Add(5*time.Minute), s, "edit", 24))
		if s.HasEditTestEdit {
			rows = append(rows,
				fixtureConversation(sessionID, "r3", userID, start.Add(10*time.Minute), s, "verify", 0),
				fixtureConversation(sessionID, "r4", userID, start.Add(15*time.Minute), s, "edit", 12),
			)
		}
		rows = append(rows, fixtureConversation(sessionID, "r5", userID, start.Add(20*time.Minute), s, "verify", 0))
	}
	for _, row := range rows {
		if err := db.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "session_id"}, {Name: "request_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"task_id", "sender", "prompt_mode", "mode", "model", "start_time",
				"end_time", "process_time", "upstream_tokens", "downstream_tokens",
				"cost", "diff_lines", "repo_addr", "repo_branch", "work_dir",
				"work_dir_id", "user_input", "request_content", "response_content",
			}),
		}).Create(&row).Error; err != nil {
			return fmt.Errorf("seed conversation %s/%s: %w", sessionID, row.RequestId, err)
		}
	}
	task := models.Task{
		TaskId:             "task-" + sessionID,
		SessionId:          sessionID,
		UserId:             userID,
		UserName:           "fixture-" + userID,
		ClientId:           "fixture-client",
		ClientIde:          "codex",
		RepoAddr:           s.RepoAddr,
		RepoBranch:         s.RepoBranch,
		WorkDir:            "/fixture/" + s.Name,
		WorkDirId:          "fixture-workdir-" + s.Name,
		StartTime:          start,
		EndTime:            start.Add(25 * time.Minute),
		DiffLines:          fixtureScenarioDiffLines(s),
		Silica:             fixtureScenarioSilica(s),
		UpstreamTokens:     1000,
		DownstreamTokens:   2000,
		Cost:               0.01,
		TaskRealMinutes:    25,
		TaskAncientMinutes: 120,
		Title:              s.Name,
		SessionDate:        start.Format("2006/01/02"),
		ConversationDate:   start.Format("2006/01/02"),
	}
	if err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "task_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"session_id", "user_id", "user_name", "client_id", "client_ide",
			"repo_addr", "repo_branch", "work_dir", "work_dir_id", "start_time",
			"end_time", "diff_lines", "silica", "upstream_tokens",
			"downstream_tokens", "cost", "task_real_minutes",
			"task_ancient_minutes", "title", "session_date", "conversation_date",
			"updated_at",
		}),
	}).Create(&task).Error; err != nil {
		return fmt.Errorf("seed task %s: %w", task.TaskId, err)
	}
	return nil
}

func fixtureConversation(sessionID, requestID, userID string, start time.Time, s EfficiencyV2FixtureScenario, kind string, diffLines int64) models.Conversation {
	payload := map[string]interface{}{
		"fixture_event_kind":  kind,
		"scenario":            s.Name,
		"need_id":             s.NeedID,
		"boundary_source":     s.BoundarySource,
		"boundary_confidence": s.BoundaryConfidence,
		"status":              s.Status,
		"boundary_evidence":   s.BoundaryEvidence,
		"mock_files":          s.MockFiles,
	}
	if kind == "verify" {
		payload["command"] = "go test ./..."
	}
	response := fixtureJSON(payload)
	return models.Conversation{
		SessionId:        sessionID,
		RequestId:        requestID,
		TaskId:           "task-" + sessionID,
		Sender:           "assistant",
		PromptMode:       "default",
		Mode:             "fixture",
		Model:            "fixture-model",
		StartTime:        start,
		EndTime:          start.Add(2 * time.Minute),
		ProcessTime:      120000,
		UpstreamTokens:   100,
		DownstreamTokens: 200,
		Cost:             0.001,
		DiffLines:        diffLines,
		RepoAddr:         s.RepoAddr,
		RepoBranch:       s.RepoBranch,
		WorkDir:          "/fixture/" + s.Name,
		WorkDirId:        "fixture-workdir-" + s.Name,
		UserInput:        fmt.Sprintf("<user_message>%s %s</user_message>", s.Name, kind),
		RequestContent:   response,
		ResponseContent:  response,
	}
}

func seedFixtureCommits(db *gorm.DB, base time.Time, scenarioIndex int, s EfficiencyV2FixtureScenario) error {
	start := base.Add(time.Duration(scenarioIndex*24+1) * time.Hour)
	for i, commitID := range s.CommitIDs {
		userID := s.PrimaryUserID
		if i < len(s.ContributorUserIDs) {
			userID = s.ContributorUserIDs[i]
		}
		commitTime := start.Add(time.Duration(i*20) * time.Minute)
		// Push uncovered scenarios outside the configured post-session margin so
		// the aggregator can classify the last commit as uncovered. The offset is
		// kept conservative (> 4h) — far beyond the default 60-min post margin —
		// so the test stays correct even if the default rises later.
		if s.HasUncoveredCommit && i == len(s.CommitIDs)-1 && len(s.CommitIDs) > 1 {
			commitTime = start.Add(6 * time.Hour)
		}
		commit := models.Commit{
			CommitId:             commitID,
			CommitTime:           commitTime,
			RepoAddr:             s.RepoAddr,
			RepoBranch:           s.RepoBranch,
			GitUserName:          "fixture-" + userID,
			GitUserEmail:         userID + "@example.com",
			UserId:               userID,
			UserName:             "fixture-" + userID,
			ClientId:             "fixture-client",
			WorkDir:              "/fixture/" + s.Name,
			WorkDirId:            "fixture-workdir-" + s.Name,
			DiffLines:            fixtureScenarioDiffLines(s),
			CommitAncientMinutes: 120,
			CommitRealAiMinutes:  20,
			CommitRealMinutes:    35,
			Silica:               fixtureScenarioSilica(s),
			Comment:              fixtureCommitComment(s),
		}
		if err := db.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "commit_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"commit_time", "repo_addr", "repo_branch", "git_user_name",
				"git_user_email", "user_id", "user_name", "client_id",
				"work_dir", "work_dir_id", "diff_lines", "commit_ancient_minutes",
				"commit_real_ai_minutes", "commit_real_minutes", "silica",
				"comment", "updated_at",
			}),
		}).Create(&commit).Error; err != nil {
			return fmt.Errorf("seed commit %s: %w", commitID, err)
		}
	}
	return nil
}

func fixtureScenarioDiffLines(s EfficiencyV2FixtureScenario) int {
	switch {
	case s.HasNoEdit:
		return 0
	case s.HasHighEfficiencyOutlier:
		return 5
	case s.HasLongIdleGap:
		return 360
	default:
		return 120
	}
}

func fixtureScenarioSilica(s EfficiencyV2FixtureScenario) float64 {
	if s.HasLowAIParticipation {
		return 0.1
	}
	if s.HasNoEdit {
		return 0
	}
	return 0.75
}

func fixtureCommitComment(s EfficiencyV2FixtureScenario) string {
	if len(s.BoundaryEvidence.CommitMessages) > 0 {
		return s.BoundaryEvidence.CommitMessages[0]
	}
	if s.BoundarySource == efficiencyV2BoundaryIssue {
		return "TASK-302 fixture commit"
	}
	if s.HasUncoveredCommit {
		return "fixture uncovered manual commit"
	}
	return "fixture commit for " + s.Name
}

func fixtureJSON(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(data)
}
