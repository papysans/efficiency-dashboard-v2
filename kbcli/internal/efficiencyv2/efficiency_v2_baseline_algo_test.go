package efficiencyv2

import (
	"strings"
	"testing"
	"time"

	"kanban/core/models"
)

func TestComputeEfficiencyV2BaselineA_Think_FromUserCharsReadsTurns(t *testing.T) {
	need := efficiencyV2AggTestNeed("need-think", efficiencyV2BoundaryBranch, efficiencyV2ConfidenceHigh, "u-alice", []string{"s-1"}, nil)
	session := models.SessionStageMetric{SessionId: "s-1", ReadEventCount: 4, MessageEventCount: 8}
	event := models.ConversationEvent{
		EventId: "e-1", SessionId: "s-1", EventKind: "message",
		EventStartTs: time.Now(), Payload: models.ObjectJSON(`{"user_input":"please refactor the auth module to support OIDC with comprehensive tests"}`),
	}

	result := ComputeEfficiencyV2BaselineA(need, []models.SessionStageMetric{session}, []models.ConversationEvent{event}, nil, EfficiencyV2BaselineACoefficients{})
	if result.ThinkMin == nil {
		t.Fatalf("think_min should be set, got nil")
	}
	// chars * 0.02 + 4 * 5.0 + 8 * 5.0 = 78*0.02 + 20 + 40 = 61.56
	// （古法系数：ThinkFilesReadMin=5, ThinkTurnMin=5 — 见
	// DefaultEfficiencyV2BaselineACoefficients 的设计意图说明）
	if *result.ThinkMin <= 30 || *result.ThinkMin > 100 {
		t.Fatalf("think_min = %.2f, expected in (30, 100]", *result.ThinkMin)
	}
}

func TestComputeEfficiencyV2BaselineA_Think_MissingFeatureFloor(t *testing.T) {
	need := efficiencyV2AggTestNeed("need-think-none", efficiencyV2BoundaryBranch, efficiencyV2ConfidenceHigh, "u-alice", []string{"s-1"}, nil)
	session := models.SessionStageMetric{SessionId: "s-1"}
	result := ComputeEfficiencyV2BaselineA(need, []models.SessionStageMetric{session}, nil, nil, EfficiencyV2BaselineACoefficients{})
	if result.ThinkMin == nil || *result.ThinkMin != 5 {
		t.Fatalf("think_min default floor = %v, want 5", result.ThinkMin)
	}
}

func TestComputeEfficiencyV2BaselineA_Exec_FilterLockFiles(t *testing.T) {
	need := efficiencyV2AggTestNeed("need-exec-lock", efficiencyV2BoundaryBranch, efficiencyV2ConfidenceHigh, "u-alice", []string{"s-1"}, []string{"c1"})
	need.TouchedFiles = EfficiencyV2StringJSON([]string{"package-lock.json", "yarn.lock"})
	session := models.SessionStageMetric{SessionId: "s-1"}
	commit := models.Commit{CommitId: "c1", DiffLines: 5000, Comment: "chore: bump deps"}

	result := ComputeEfficiencyV2BaselineA(need, []models.SessionStageMetric{session}, nil, []models.Commit{commit}, EfficiencyV2BaselineACoefficients{})
	if result.ExecMin == nil {
		t.Fatalf("exec_min should be set, got nil")
	}
	if *result.ExecMin > 100 {
		t.Fatalf("exec_min = %.2f, expected near floor (all files filtered)", *result.ExecMin)
	}
	joined := strings.Join(result.Reasons, "|")
	if !strings.Contains(joined, "filtered_files") {
		t.Fatalf("reasons should mention filtered_files, got %v", result.Reasons)
	}
}

func TestComputeEfficiencyV2BaselineA_Exec_FilterGeneratedFiles(t *testing.T) {
	need := efficiencyV2AggTestNeed("need-exec-gen", efficiencyV2BoundaryBranch, efficiencyV2ConfidenceHigh, "u-alice", []string{"s-1"}, []string{"c1"})
	need.TouchedFiles = EfficiencyV2StringJSON([]string{"proto/api.pb.go", "service/types_generated.go"})
	commit := models.Commit{CommitId: "c1", DiffLines: 8000, Comment: "regenerate proto"}

	result := ComputeEfficiencyV2BaselineA(need, []models.SessionStageMetric{{SessionId: "s-1"}}, nil, []models.Commit{commit}, EfficiencyV2BaselineACoefficients{})
	if *result.ExecMin > 100 {
		t.Fatalf("exec_min = %.2f, expected near floor", *result.ExecMin)
	}
}

func TestComputeEfficiencyV2BaselineA_Exec_FormatterOnlyFlagged(t *testing.T) {
	need := efficiencyV2AggTestNeed("need-exec-fmt", efficiencyV2BoundaryBranch, efficiencyV2ConfidenceHigh, "u-alice", []string{"s-1"}, []string{"c1"})
	need.TouchedFiles = EfficiencyV2StringJSON([]string{"src/app.go", "src/auth.go"})
	commits := []models.Commit{
		{CommitId: "c1", DiffLines: 400, Comment: "style: gofmt"},
		{CommitId: "c2", DiffLines: 200, Comment: "format: prettier"},
	}
	result := ComputeEfficiencyV2BaselineA(need, []models.SessionStageMetric{{SessionId: "s-1"}}, nil, commits, EfficiencyV2BaselineACoefficients{})
	joined := strings.Join(result.Reasons, "|")
	if !strings.Contains(joined, "formatter_only_commits") {
		t.Fatalf("reasons should mention formatter_only_commits, got %v", result.Reasons)
	}
	if *result.ExecMin != 5 {
		t.Fatalf("exec_min = %.2f, want 5 (formatter floor)", *result.ExecMin)
	}
}

func TestComputeEfficiencyV2BaselineA_Verify_Coefficients(t *testing.T) {
	need := efficiencyV2AggTestNeed("need-verify", efficiencyV2BoundaryBranch, efficiencyV2ConfidenceHigh, "u-alice", []string{"s-1"}, nil)
	session := models.SessionStageMetric{SessionId: "s-1", VerifyEventCount: 3, RePromptCount: 2, ReadEventCount: 4, EditEventCount: 4}
	result := ComputeEfficiencyV2BaselineA(need, []models.SessionStageMetric{session}, nil, nil, EfficiencyV2BaselineACoefficients{})
	if result.VerifyMin == nil {
		t.Fatalf("verify_min should be set")
	}
	// 3*5 + 2*5 + review_reads = 15 + 10 + (4 * 3/(3+4)) * 1 = 15+10+1.71 ≈ 26.71
	if *result.VerifyMin < 25 || *result.VerifyMin > 30 {
		t.Fatalf("verify_min = %.2f, expected ~26.71", *result.VerifyMin)
	}
}

func TestComputeEfficiencyV2BaselineA_Verify_NoSignalsUsesFloor(t *testing.T) {
	need := efficiencyV2AggTestNeed("need-verify-none", efficiencyV2BoundaryBranch, efficiencyV2ConfidenceHigh, "u-alice", []string{"s-1"}, nil)
	session := models.SessionStageMetric{SessionId: "s-1"}
	result := ComputeEfficiencyV2BaselineA(need, []models.SessionStageMetric{session}, nil, nil, EfficiencyV2BaselineACoefficients{})
	if *result.VerifyMin != 5 {
		t.Fatalf("verify_min = %.2f, want 5", *result.VerifyMin)
	}
}

func TestComputeEfficiencyV2BaselineA_TotalIsSumOfComponents(t *testing.T) {
	need := efficiencyV2AggTestNeed("need-total", efficiencyV2BoundaryBranch, efficiencyV2ConfidenceHigh, "u-alice", []string{"s-1"}, []string{"c1"})
	need.TouchedFiles = EfficiencyV2StringJSON([]string{"src/main.go"})
	session := models.SessionStageMetric{SessionId: "s-1", VerifyEventCount: 2, MessageEventCount: 4, ReadEventCount: 3}
	commit := models.Commit{CommitId: "c1", DiffLines: 200}
	result := ComputeEfficiencyV2BaselineA(need, []models.SessionStageMetric{session}, nil, []models.Commit{commit}, EfficiencyV2BaselineACoefficients{})
	if result.TotalMin == nil {
		t.Fatalf("total_min should be set")
	}
	expectedSum := 0.0
	if result.ThinkMin != nil {
		expectedSum += *result.ThinkMin
	}
	if result.ExecMin != nil {
		expectedSum += *result.ExecMin
	}
	if result.VerifyMin != nil {
		expectedSum += *result.VerifyMin
	}
	if *result.TotalMin != expectedSum {
		t.Fatalf("total = %.4f, want sum %.4f", *result.TotalMin, expectedSum)
	}
}

func TestPersistEfficiencyV2BaselineAOnNeed_FieldsAssigned(t *testing.T) {
	need := models.Need{NeedId: "n-1"}
	think, exec, verify, total := 5.0, 10.0, 15.0, 30.0
	PersistEfficiencyV2BaselineAOnNeed(&need, EfficiencyV2BaselineAResult{
		ThinkMin: &think, ExecMin: &exec, VerifyMin: &verify, TotalMin: &total,
	})
	if need.BaselineAlgoThinkWorkMin == nil || *need.BaselineAlgoThinkWorkMin != 5 {
		t.Fatalf("think field not assigned: %v", need.BaselineAlgoThinkWorkMin)
	}
	if need.BaselineAlgoExecutionWorkMin == nil || *need.BaselineAlgoExecutionWorkMin != 10 {
		t.Fatalf("exec field not assigned: %v", need.BaselineAlgoExecutionWorkMin)
	}
	if need.BaselineAlgoVerificationWorkMin == nil || *need.BaselineAlgoVerificationWorkMin != 15 {
		t.Fatalf("verify field not assigned: %v", need.BaselineAlgoVerificationWorkMin)
	}
	if need.BaselineAlgoTotalWorkMin == nil || *need.BaselineAlgoTotalWorkMin != 30 {
		t.Fatalf("total field not assigned: %v", need.BaselineAlgoTotalWorkMin)
	}
}

func TestDefaultEfficiencyV2BaselineACoefficients_StableDefaults(t *testing.T) {
	defaults := DefaultEfficiencyV2BaselineACoefficients()
	if defaults.Version == "" {
		t.Fatalf("default version must be set")
	}
	if defaults.MinThinkMin <= 0 || defaults.MinExecMin <= 0 || defaults.MinVerifyMin <= 0 {
		t.Fatalf("min floors must be positive: %+v", defaults)
	}
	if defaults.ExecLinesPerMin <= 0 {
		t.Fatalf("exec lines per minute must be positive")
	}
}
