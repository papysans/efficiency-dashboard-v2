package main

import (
	"kanban/kbcli/internal/llm"
	"strings"
	"testing"

	"kanban/core/models"
)

func TestBuildEfficiencyV2NeedStructuredSummary_NoRawTranscripts(t *testing.T) {
	need := models.Need{
		NeedId:                      "n-1",
		BoundarySource:              efficiencyV2BoundaryPR,
		Status:                      "merged",
		ChangedLoc:                  500,
		FileCount:                   8,
		CommitCount:                 4,
		UncoveredLoc:                30,
		ThinkActiveMin:              25,
		ExecutionActiveMin:          70,
		VerificationActiveMin:       12,
		TotalSessionActivePersonMin: 110,
		UncoveredHumanMin:           45,
		RevertCount:                 1,
	}
	session := models.SessionStageMetric{SessionId: "s-1", UserId: "u-alice", ThinkActiveMin: 5, EditEventCount: 3, VerifyEventCount: 2}

	commits := []models.Commit{
		{CommitId: "c1", Comment: "feat: add OIDC reset"},
		{CommitId: "c2", Comment: "fix: handle empty token"},
	}
	tasks := []models.Task{
		{TaskId: "t1", SessionId: "s-1", Title: "实现密码重置流程"},
		{TaskId: "t2", SessionId: "s-1", Title: "实现密码重置流程"}, // duplicate dedup
		{TaskId: "t3", SessionId: "s-1", Title: ""},         // empty skipped
	}
	summary := BuildEfficiencyV2NeedStructuredSummary(need, []models.SessionStageMetric{session}, commits, tasks)
	if summary.NeedID != "n-1" {
		t.Fatalf("need id = %q", summary.NeedID)
	}
	if len(summary.SessionSummaries) != 1 || summary.SessionSummaries[0] == "" {
		t.Fatalf("session summaries should be populated, got %v", summary.SessionSummaries)
	}
	if strings.Contains(summary.SessionSummaries[0], "<user_message>") {
		t.Fatalf("session summary must not include raw user message, got %q", summary.SessionSummaries[0])
	}
	if len(summary.CommitMessages) != 2 {
		t.Fatalf("commit_messages = %v, want 2 entries", summary.CommitMessages)
	}
	if len(summary.TaskTitles) != 1 || summary.TaskTitles[0] != "实现密码重置流程" {
		t.Fatalf("task_titles = %v, want 1 dedup'd entry", summary.TaskTitles)
	}
}

func TestBuildEfficiencyV2LLMPrompt_DoesNotIncludeRawTranscripts(t *testing.T) {
	summary := EfficiencyV2NeedStructuredSummary{NeedID: "n-1", BoundarySource: efficiencyV2BoundaryBranch, ChangedLOC: 200}
	prompt, err := BuildEfficiencyV2LLMPrompt(summary)
	if err != nil {
		t.Fatalf("build prompt: %v", err)
	}
	for _, forbidden := range []string{"raw_transcript", "<user_message>", "<assistant_message>"} {
		if strings.Contains(prompt, forbidden) {
			t.Fatalf("prompt must not include %q", forbidden)
		}
	}
	if !strings.Contains(prompt, "严格 JSON") {
		t.Fatalf("prompt should require strict JSON output, got: %s", prompt[:200])
	}
}

func TestCallAIForNeedEstimationV4_DisabledReturnsNullWithReason(t *testing.T) {
	result := CallAIForNeedEstimationV4(EfficiencyV2NeedStructuredSummary{NeedID: "n-1"}, llm.AIEstimationConfig{Enabled: false})
	if result.TotalMin != nil {
		t.Fatalf("total should be nil when disabled, got %v", *result.TotalMin)
	}
	if !strings.Contains(result.Reason, "disabled") {
		t.Fatalf("reason should mention disabled, got %q", result.Reason)
	}
}

func TestCallAIForNeedEstimationV4_MissingAPIKeyReturnsDisabled(t *testing.T) {
	result := CallAIForNeedEstimationV4(EfficiencyV2NeedStructuredSummary{NeedID: "n-1"}, llm.AIEstimationConfig{Enabled: true, APIKey: ""})
	if result.TotalMin != nil {
		t.Fatalf("total should be nil when API key missing")
	}
	if !strings.Contains(result.Reason, "disabled") {
		t.Fatalf("reason should mention disabled, got %q", result.Reason)
	}
}

func TestParseEfficiencyV2LLMResponse_ValidJSON(t *testing.T) {
	raw := `{"think_min": 30, "exec_min": 90, "verify_min": 15, "total_min": 135, "confidence": "medium", "reason": "Looks like a small feature"}`
	result := parseEfficiencyV2LLMResponse(raw)
	if result.TotalMin == nil || *result.TotalMin != 135 {
		t.Fatalf("total = %v, want 135", result.TotalMin)
	}
	if result.Confidence != "medium" {
		t.Fatalf("confidence = %q, want medium", result.Confidence)
	}
}

func TestParseEfficiencyV2LLMResponse_InvalidJSONReturnsReason(t *testing.T) {
	result := parseEfficiencyV2LLMResponse("not json")
	if result.TotalMin != nil {
		t.Fatalf("total should be nil for invalid JSON")
	}
	if !strings.Contains(result.Reason, "no_json") && !strings.Contains(result.Reason, "invalid_json") {
		t.Fatalf("reason should mention json error, got %q", result.Reason)
	}
}

func TestParseEfficiencyV2LLMResponse_NegativeMinutesRejected(t *testing.T) {
	result := parseEfficiencyV2LLMResponse(`{"think_min": -5, "exec_min": 10, "verify_min": 5, "total_min": 10}`)
	if result.TotalMin != nil {
		t.Fatalf("total should be nil for negative values")
	}
	if !strings.Contains(result.Reason, "negative") {
		t.Fatalf("reason should mention negative_minutes, got %q", result.Reason)
	}
}

func TestParseEfficiencyV2LLMResponse_TotalAutoComputedFromComponents(t *testing.T) {
	result := parseEfficiencyV2LLMResponse(`{"think_min": 10, "exec_min": 20, "verify_min": 5, "confidence": "high", "reason": "ok"}`)
	if result.TotalMin == nil {
		t.Fatalf("total should be auto-computed")
	}
	if *result.TotalMin != 35 {
		t.Fatalf("total = %.2f, want 35", *result.TotalMin)
	}
}

func TestPersistEfficiencyV2BaselineCOnNeed_NullableFields(t *testing.T) {
	need := models.Need{NeedId: "n-1"}
	PersistEfficiencyV2BaselineCOnNeed(&need, EfficiencyV2LLMResult{Confidence: "low", Reason: "llm:disabled"})
	if need.BaselineLLMTotalWorkMin != nil {
		t.Fatalf("total should remain nil")
	}
	if need.BaselineLLMConfidence != "low" {
		t.Fatalf("confidence = %q, want low", need.BaselineLLMConfidence)
	}
	if need.BaselineLLMReason != "llm:disabled" {
		t.Fatalf("reason = %q, want llm:disabled", need.BaselineLLMReason)
	}
}
