package main

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"kanban/core/models"
)

func TestClassifyEfficiencyV2Event(t *testing.T) {
	cfg := efficiencyV2StageTestConfig()
	base := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)
	cases := []struct {
		name  string
		event models.ConversationEvent
		want  string
	}{
		{
			name:  "message event kind",
			event: efficiencyV2StageTestEvent("e-message", "s1", "message", "", "", base, 30),
			want:  efficiencyV2EventClassMessage,
		},
		{
			name:  "read tool",
			event: efficiencyV2StageTestEvent("e-read", "s1", "tool", "Read", "", base, 30),
			want:  efficiencyV2EventClassRead,
		},
		{
			name:  "grep tool",
			event: efficiencyV2StageTestEvent("e-grep", "s1", "tool", "Grep", "", base, 30),
			want:  efficiencyV2EventClassRead,
		},
		{
			name:  "search tool",
			event: efficiencyV2StageTestEvent("e-search", "s1", "tool", "Search", "", base, 30),
			want:  efficiencyV2EventClassRead,
		},
		{
			name:  "edit tool",
			event: efficiencyV2StageTestEvent("e-edit", "s1", "tool", "Edit", "", base, 30),
			want:  efficiencyV2EventClassEdit,
		},
		{
			name:  "write tool",
			event: efficiencyV2StageTestEvent("e-write", "s1", "tool", "Write", "", base, 30),
			want:  efficiencyV2EventClassEdit,
		},
		{
			name:  "multiedit tool",
			event: efficiencyV2StageTestEvent("e-multiedit", "s1", "tool", "MultiEdit", "", base, 30),
			want:  efficiencyV2EventClassEdit,
		},
		{
			name:  "apply patch tool",
			event: efficiencyV2StageTestEvent("e-apply-patch", "s1", "tool", "ApplyPatch", "", base, 30),
			want:  efficiencyV2EventClassEdit,
		},
		{
			name:  "edit event kind",
			event: efficiencyV2StageTestEvent("e-edit-kind", "s1", "edit", "", "", base, 30),
			want:  efficiencyV2EventClassEdit,
		},
		{
			name:  "conversation diff source",
			event: efficiencyV2StageTestEventWithSource("e-diff", "s1", "other", "", "", base, 30, "conversation_diff", "degraded"),
			want:  efficiencyV2EventClassEdit,
		},
		{
			name:  "shell go test verification",
			event: efficiencyV2StageTestEvent("e-go-test", "s1", "shell", "Bash", "go test ./...", base, 30),
			want:  efficiencyV2EventClassVerify,
		},
		{
			name:  "shell npm build verification",
			event: efficiencyV2StageTestEvent("e-npm-build", "s1", "shell", "Bash", "npm run build", base, 30),
			want:  efficiencyV2EventClassVerify,
		},
		{
			name:  "shell typecheck verification",
			event: efficiencyV2StageTestEvent("e-tsc", "s1", "shell", "Bash", "pnpm exec tsc --noEmit", base, 30),
			want:  efficiencyV2EventClassVerify,
		},
		{
			name:  "shell patch command",
			event: efficiencyV2StageTestEvent("e-git-apply", "s1", "shell", "Bash", "git apply fix.patch", base, 30),
			want:  efficiencyV2EventClassEdit,
		},
		{
			name:  "shell non verification command",
			event: efficiencyV2StageTestEvent("e-git-status", "s1", "shell", "Bash", "git status --short", base, 30),
			want:  efficiencyV2EventClassOther,
		},
		{
			name:  "shell install command",
			event: efficiencyV2StageTestEvent("e-install", "s1", "shell", "Bash", "npm install", base, 30),
			want:  efficiencyV2EventClassOther,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyEfficiencyV2Event(tc.event, cfg)
			if got != tc.want {
				t.Fatalf("class: want %s, got %s", tc.want, got)
			}
		})
	}
}

func TestEfficiencyV2StageDurationInference(t *testing.T) {
	cfg := efficiencyV2StageTestConfig()
	base := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)

	explicit := efficiencyV2StageTestEvent("e-explicit", "s1", "tool", "Edit", "", base, 90)
	if got := InferEfficiencyV2EventDurationSec(explicit, nil, efficiencyV2EventClassEdit, cfg.Stage); got != 90 {
		t.Fatalf("explicit duration: want 90, got %d", got)
	}

	withoutEnd := efficiencyV2StageTestOpenEvent("e-open", "s1", "tool", "Edit", "", base)
	nextWithinGap := efficiencyV2StageTestEvent("e-next", "s1", "message", "", "", base.Add(2*time.Minute), 30)
	if got := InferEfficiencyV2EventDurationSec(withoutEnd, &nextWithinGap, efficiencyV2EventClassEdit, cfg.Stage); got != 120 {
		t.Fatalf("next-event inferred duration: want 120, got %d", got)
	}

	nextOutsideGap := efficiencyV2StageTestEvent("e-far", "s1", "message", "", "", base.Add(9*time.Minute), 30)
	if got := InferEfficiencyV2EventDurationSec(withoutEnd, &nextOutsideGap, efficiencyV2EventClassEdit, cfg.Stage); got != 40 {
		t.Fatalf("default edit duration: want 40, got %d", got)
	}

	message := efficiencyV2StageTestOpenEvent("e-message", "s1", "message", "", "", base)
	message.Payload = models.ObjectJSON(`{"text":"abcdefghij"}`)
	if got := InferEfficiencyV2EventDurationSec(message, nil, efficiencyV2EventClassMessage, cfg.Stage); got != 6 {
		t.Fatalf("message char duration: want 6, got %d", got)
	}
}

func TestSplitEfficiencyV2StageNoEditSession(t *testing.T) {
	cfg := efficiencyV2StageTestConfig()
	base := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)
	events := []models.ConversationEvent{
		efficiencyV2StageTestEvent("e-message", "s-no-edit", "message", "", "", base, 60),
		efficiencyV2StageTestEvent("e-read", "s-no-edit", "tool", "Read", "", base.Add(time.Minute), 120),
		efficiencyV2StageTestEvent("e-test", "s-no-edit", "shell", "Bash", "go test ./...", base.Add(3*time.Minute), 30),
		efficiencyV2StageTestEvent("e-other", "s-no-edit", "shell", "Bash", "git status", base.Add(4*time.Minute), 30),
	}

	got := BuildEfficiencyV2SessionStageMetric(events, cfg)

	assertEfficiencyV2Float(t, "think active", got.ThinkActiveMin, 3.5)
	assertEfficiencyV2Float(t, "exec active", got.ExecutionActiveMin, 0)
	assertEfficiencyV2Float(t, "verify active", got.VerificationActiveMin, 0)
	assertEfficiencyV2Float(t, "other active", got.OtherActiveMin, 0.5)
	assertEfficiencyV2Float(t, "total active", got.TotalActiveMin, 4.0)
	if got.EditEventCount != 0 || got.VerifyEventCount != 1 || got.OtherEventCount != 1 {
		t.Fatalf("counts: edit=%d verify=%d other=%d", got.EditEventCount, got.VerifyEventCount, got.OtherEventCount)
	}
	if got.StageConfidence != efficiencyV2StageConfidenceHigh {
		t.Fatalf("confidence: want high, got %s", got.StageConfidence)
	}
	if !strings.Contains(got.ConfidenceReason, "no_edit_session") {
		t.Fatalf("confidence reason should mention no_edit_session, got %q", got.ConfidenceReason)
	}
}

func TestSplitEfficiencyV2StageFirstMessageEditSession(t *testing.T) {
	cfg := efficiencyV2StageTestConfig()
	base := time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC)
	events := []models.ConversationEvent{
		efficiencyV2StageTestEvent("e-edit", "s-first-edit", "tool", "Edit", "", base, 120),
		efficiencyV2StageTestEvent("e-message", "s-first-edit", "message", "", "", base.Add(2*time.Minute), 60),
	}

	got := BuildEfficiencyV2SessionStageMetric(events, cfg)

	assertEfficiencyV2Float(t, "think active", got.ThinkActiveMin, 0)
	assertEfficiencyV2Float(t, "exec active", got.ExecutionActiveMin, 3.0)
	assertEfficiencyV2Float(t, "verify active", got.VerificationActiveMin, 0)
	if got.FirstEditTs == nil || !got.FirstEditTs.Equal(base) {
		t.Fatalf("first edit ts: want %s, got %v", base, got.FirstEditTs)
	}
	if got.LastEditTs == nil || !got.LastEditTs.Equal(base) {
		t.Fatalf("last edit ts: want %s, got %v", base, got.LastEditTs)
	}
}

func TestSplitEfficiencyV2StageEditTestEditLoopFinalVerification(t *testing.T) {
	cfg := efficiencyV2StageTestConfig()
	base := time.Date(2026, 5, 21, 11, 0, 0, 0, time.UTC)
	events := []models.ConversationEvent{
		efficiencyV2StageTestEvent("e-message", "s-loop", "message", "", "", base, 60),
		efficiencyV2StageTestEvent("e-edit-1", "s-loop", "tool", "Edit", "", base.Add(time.Minute), 60),
		efficiencyV2StageTestEvent("e-test", "s-loop", "shell", "Bash", "go test ./...", base.Add(2*time.Minute), 120),
		efficiencyV2StageTestEvent("e-edit-2", "s-loop", "tool", "MultiEdit", "", base.Add(4*time.Minute), 60),
		efficiencyV2StageTestEvent("e-build", "s-loop", "shell", "Bash", "npm run build", base.Add(5*time.Minute), 180),
	}

	got := BuildEfficiencyV2SessionStageMetric(events, cfg)

	assertEfficiencyV2Float(t, "think active", got.ThinkActiveMin, 1.0)
	assertEfficiencyV2Float(t, "exec active", got.ExecutionActiveMin, 2.0)
	assertEfficiencyV2Float(t, "verify active", got.VerificationActiveMin, 5.0)
	assertEfficiencyV2Float(t, "other active", got.OtherActiveMin, 0)
	if got.EditEventCount != 2 || got.VerifyEventCount != 2 {
		t.Fatalf("counts: edit=%d verify=%d", got.EditEventCount, got.VerifyEventCount)
	}
	if got.FirstEditTs == nil || !got.FirstEditTs.Equal(base.Add(time.Minute)) {
		t.Fatalf("first edit ts: want %s, got %v", base.Add(time.Minute), got.FirstEditTs)
	}
	if got.LastEditTs == nil || !got.LastEditTs.Equal(base.Add(4*time.Minute)) {
		t.Fatalf("last edit ts: want %s, got %v", base.Add(4*time.Minute), got.LastEditTs)
	}
}

func TestSplitEfficiencyV2StageDegradedConfidence(t *testing.T) {
	cfg := efficiencyV2StageTestConfig()
	base := time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC)
	events := []models.ConversationEvent{
		efficiencyV2StageTestEventWithSource("e-message", "s-degraded", "message", "", "", base, 60, "synthetic", "exact"),
		efficiencyV2StageTestEventWithSource("e-diff", "s-degraded", "other", "", "", base.Add(time.Minute), 60, "conversation_diff", "degraded"),
	}

	got := BuildEfficiencyV2SessionStageMetric(events, cfg)

	if got.StageConfidence == efficiencyV2StageConfidenceHigh {
		t.Fatalf("confidence should be lowered for degraded input")
	}
	if got.DegradedEventCount != 1 {
		t.Fatalf("degraded count: want 1, got %d", got.DegradedEventCount)
	}
	if !strings.Contains(got.ConfidenceReason, "degraded_input_events=1") {
		t.Fatalf("confidence reason should mention degraded input, got %q", got.ConfidenceReason)
	}
	if got.EditEventCount != 1 {
		t.Fatalf("conversation_diff event should classify as edit, got edit count %d", got.EditEventCount)
	}
}

func TestSplitEfficiencyV2StageMetricsAreDeterministic(t *testing.T) {
	cfg := efficiencyV2StageTestConfig()
	base := time.Date(2026, 5, 21, 13, 0, 0, 0, time.UTC)
	events := []models.ConversationEvent{
		efficiencyV2StageTestEvent("s2-edit", "s2", "tool", "Edit", "", base.Add(2*time.Minute), 60),
		efficiencyV2StageTestEvent("s1-message", "s1", "message", "", "", base, 60),
		efficiencyV2StageTestEvent("s2-test", "s2", "shell", "Bash", "go test ./...", base.Add(3*time.Minute), 60),
		efficiencyV2StageTestEvent("s1-read", "s1", "tool", "Read", "", base.Add(time.Minute), 60),
	}
	reversed := []models.ConversationEvent{events[3], events[2], events[1], events[0]}

	first := BuildEfficiencyV2SessionStageMetrics(events, cfg)
	second := BuildEfficiencyV2SessionStageMetrics(reversed, cfg)

	if len(first) != 2 {
		t.Fatalf("metric count: want 2, got %d", len(first))
	}
	if first[0].SessionId != "s1" || first[1].SessionId != "s2" {
		t.Fatalf("session order: got %s, %s", first[0].SessionId, first[1].SessionId)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("stage metric generation should be deterministic for reordered input\nfirst=%#v\nsecond=%#v", first, second)
	}
}

func TestSplitEfficiencyV2StageMetricGenerationIsIdempotent(t *testing.T) {
	cfg := efficiencyV2StageTestConfig()
	base := time.Date(2026, 5, 21, 13, 30, 0, 0, time.UTC)
	events := []models.ConversationEvent{
		efficiencyV2StageTestEvent("e-message", "s-idempotent", "message", "", "", base, 60),
		efficiencyV2StageTestEvent("e-edit", "s-idempotent", "tool", "Edit", "", base.Add(time.Minute), 60),
		efficiencyV2StageTestEvent("e-test", "s-idempotent", "shell", "Bash", "go test ./...", base.Add(2*time.Minute), 60),
	}

	first := BuildEfficiencyV2SessionStageMetrics(events, cfg)
	second := BuildEfficiencyV2SessionStageMetrics(events, cfg)

	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("rerun should build one logical metric row per session, got %d and %d", len(first), len(second))
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("rerun should produce deterministic metrics\nfirst=%#v\nsecond=%#v", first, second)
	}
}

func TestEfficiencyV2StageEventKindCountsJSON(t *testing.T) {
	cfg := efficiencyV2StageTestConfig()
	base := time.Date(2026, 5, 21, 14, 0, 0, 0, time.UTC)
	events := []models.ConversationEvent{
		efficiencyV2StageTestEvent("e-message", "s-counts", "message", "", "", base, 60),
		efficiencyV2StageTestEvent("e-edit", "s-counts", "edit", "", "", base.Add(time.Minute), 60),
	}

	got := BuildEfficiencyV2SessionStageMetric(events, cfg)
	var counts map[string]int64
	if err := json.Unmarshal([]byte(got.EventKindCounts), &counts); err != nil {
		t.Fatalf("event kind counts should be valid JSON: %v", err)
	}
	if counts["message"] != 1 || counts["edit"] != 1 {
		t.Fatalf("event kind counts mismatch: %#v", counts)
	}
}

func efficiencyV2StageTestConfig() EfficiencyV2Config {
	return EfficiencyV2Config{
		VerificationCommandPatterns: []string{
			"go test",
			"npm run build",
			"tsc",
			"custom verify",
		},
		Stage: EfficiencyV2StageConfig{
			MaxInferredDurationGapMinutes: 5,
			DefaultEditDurationSeconds:    40,
			DefaultReadDurationSeconds:    20,
			DefaultCommandDurationSeconds: 50,
			DefaultMessageCharsPerMinute:  100,
			DefaultOtherDurationSeconds:   13,
		},
	}
}

func efficiencyV2StageTestEvent(eventID, sessionID, kind, tool, command string, start time.Time, durationSec int) models.ConversationEvent {
	event := efficiencyV2StageTestOpenEvent(eventID, sessionID, kind, tool, command, start)
	end := start.Add(time.Duration(durationSec) * time.Second)
	event.EventEndTs = &end
	return event
}

func efficiencyV2StageTestOpenEvent(eventID, sessionID, kind, tool, command string, start time.Time) models.ConversationEvent {
	return efficiencyV2StageTestEventWithSource(eventID, sessionID, kind, tool, command, start, -1, "raw_tool", "exact")
}

func efficiencyV2StageTestEventWithSource(eventID, sessionID, kind, tool, command string, start time.Time, durationSec int, source, parseQuality string) models.ConversationEvent {
	event := models.ConversationEvent{
		EventId:      eventID,
		SessionId:    sessionID,
		RequestId:    eventID + "-request",
		TaskId:       "task-" + sessionID,
		UserId:       "user-" + sessionID,
		RepoAddr:     "git@example.com/acme/repo.git",
		RepoBranch:   "feature/stage",
		WorkDirId:    "workdir-" + sessionID,
		EventStartTs: start,
		EventKind:    kind,
		ToolName:     tool,
		CommandText:  command,
		Source:       source,
		ParseQuality: parseQuality,
		Payload:      models.ObjectJSON("{}"),
	}
	if durationSec >= 0 {
		end := start.Add(time.Duration(durationSec) * time.Second)
		event.EventEndTs = &end
	}
	return event
}

func assertEfficiencyV2Float(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.000001 {
		t.Fatalf("%s: want %.6f, got %.6f", name, want, got)
	}
}
