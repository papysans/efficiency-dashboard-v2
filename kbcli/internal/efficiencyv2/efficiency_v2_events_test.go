package efficiencyv2

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"kanban/core/models"
)

func TestEfficiencyV2NormalizeConversationEvents_ExactToolEvent(t *testing.T) {
	start := time.Date(2026, 5, 18, 9, 0, 0, 0, time.UTC)
	end := start.Add(90 * time.Second)
	conv := models.Conversation{
		SessionId:       "session-exact",
		RequestId:       "request-exact",
		TaskId:          "task-exact",
		StartTime:       start,
		EndTime:         end,
		RepoAddr:        "git@example.com/acme/api.git",
		RepoBranch:      "feature/tool-event",
		WorkDirId:       "workdir-exact",
		ResponseContent: `{"tool_name":"Edit","event_kind":"edit","command":"apply_patch","touched_files":["api/handler.go"],"payload":{"bytes":42}}`,
	}

	events, err := NormalizeEfficiencyV2ConversationEvents([]models.Conversation{conv})
	if err != nil {
		t.Fatalf("normalize exact event: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("event count = %d, want 1", len(events))
	}

	event := events[0]
	if event.EventId == "" {
		t.Fatal("event id should be deterministic and non-empty")
	}
	if event.SessionId != conv.SessionId || event.RequestId != conv.RequestId || event.TaskId != conv.TaskId {
		t.Fatalf("event identity not copied from conversation: %#v", event)
	}
	if event.Source != "raw_tool" || event.ParseQuality != "exact" {
		t.Fatalf("source/quality = %s/%s, want raw_tool/exact", event.Source, event.ParseQuality)
	}
	if event.EventKind != "edit" || event.ToolName != "Edit" || event.CommandText != "apply_patch" {
		t.Fatalf("tool fields not normalized: %#v", event)
	}
	if event.DurationSec != 90 {
		t.Fatalf("duration = %d, want 90", event.DurationSec)
	}
	assertJSONStringArray(t, string(event.TouchedFiles), []string{"api/handler.go"})
	assertJSONObjectField(t, string(event.Payload), "tool_name", "Edit")
}

func TestEfficiencyV2NormalizeConversationEvents_DiffFallback(t *testing.T) {
	start := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	conv := models.Conversation{
		SessionId:      "session-diff",
		RequestId:      "request-diff",
		TaskId:         "task-diff",
		StartTime:      start,
		EndTime:        start.Add(2 * time.Minute),
		DiffLines:      27,
		RepoAddr:       "git@example.com/acme/billing.git",
		RepoBranch:     "feature/diff",
		WorkDirId:      "workdir-diff",
		UserInput:      "update billing export",
		RequestContent: `{"note":"plain json without tool markers"}`,
	}

	events, err := NormalizeEfficiencyV2ConversationEvents([]models.Conversation{conv})
	if err != nil {
		t.Fatalf("normalize diff fallback: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("event count = %d, want 1", len(events))
	}

	event := events[0]
	if event.EventKind != "edit" || event.Source != "conversation_diff" || event.ParseQuality != "degraded" {
		t.Fatalf("diff fallback fields = kind:%s source:%s quality:%s", event.EventKind, event.Source, event.ParseQuality)
	}
	assertJSONObjectNumber(t, string(event.Payload), "diff_lines", float64(27))
}

func TestEfficiencyV2NormalizeConversationEvents_ConversationOnlyFallback(t *testing.T) {
	start := time.Date(2026, 5, 18, 11, 0, 0, 0, time.UTC)
	conv := models.Conversation{
		SessionId:       "session-message",
		RequestId:       "request-message",
		TaskId:          "task-message",
		StartTime:       start,
		UserInput:       "please inspect this issue",
		ResponseContent: "I will inspect the code path.",
	}

	events, err := NormalizeEfficiencyV2ConversationEvents([]models.Conversation{conv})
	if err != nil {
		t.Fatalf("normalize conversation-only fallback: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("event count = %d, want 1", len(events))
	}

	event := events[0]
	if event.EventKind != "message" || event.Source != "synthetic" || event.ParseQuality != "degraded" {
		t.Fatalf("conversation-only fields = kind:%s source:%s quality:%s", event.EventKind, event.Source, event.ParseQuality)
	}
	assertJSONObjectField(t, string(event.Payload), "fallback_reason", "conversation_activity")
}

func TestBuildEfficiencyV2ConversationEventID_IsDeterministic(t *testing.T) {
	start := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	conv := models.Conversation{
		SessionId:       "session-idempotent",
		RequestId:       "request-idempotent",
		StartTime:       start,
		ResponseContent: `{"tool_name":"Write","event_kind":"edit","touched_files":["main.go"]}`,
	}

	first, err := NormalizeEfficiencyV2ConversationEvents([]models.Conversation{conv})
	if err != nil {
		t.Fatalf("first normalize: %v", err)
	}
	second, err := NormalizeEfficiencyV2ConversationEvents([]models.Conversation{conv})
	if err != nil {
		t.Fatalf("second normalize: %v", err)
	}
	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("event counts = %d/%d, want 1/1", len(first), len(second))
	}
	if first[0].EventId != second[0].EventId {
		t.Fatalf("event IDs differ across reruns: %q vs %q", first[0].EventId, second[0].EventId)
	}
}

func TestNormalizeEfficiencyV2ConversationEvents_IdempotentOutput(t *testing.T) {
	start := time.Date(2026, 5, 18, 13, 0, 0, 0, time.UTC)
	convs := []models.Conversation{
		{
			SessionId:       "session-idem",
			RequestId:       "request-tool",
			StartTime:       start,
			ResponseContent: `{"tool_name":"Read","event_kind":"read","touched_files":["README.md"]}`,
		},
		{
			SessionId: "session-idem",
			RequestId: "request-diff",
			StartTime: start.Add(5 * time.Minute),
			DiffLines: 12,
		},
	}

	first, err := NormalizeEfficiencyV2ConversationEvents(convs)
	if err != nil {
		t.Fatalf("first normalize: %v", err)
	}
	second, err := NormalizeEfficiencyV2ConversationEvents(convs)
	if err != nil {
		t.Fatalf("second normalize: %v", err)
	}
	if !reflect.DeepEqual(eventIDs(first), eventIDs(second)) {
		t.Fatalf("event ids differ across reruns: %v vs %v", eventIDs(first), eventIDs(second))
	}
}

func assertJSONStringArray(t *testing.T, raw string, want []string) {
	t.Helper()
	var got []string
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unmarshal JSON array %q: %v", raw, err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("JSON array = %#v, want %#v", got, want)
	}
}

func assertJSONObjectField(t *testing.T, raw, key, want string) {
	t.Helper()
	var got map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unmarshal JSON object %q: %v", raw, err)
	}
	if got[key] != want {
		t.Fatalf("JSON field %q = %#v, want %q", key, got[key], want)
	}
}

func assertJSONObjectNumber(t *testing.T, raw, key string, want float64) {
	t.Helper()
	var got map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unmarshal JSON object %q: %v", raw, err)
	}
	if got[key] != want {
		t.Fatalf("JSON field %q = %#v, want %#v", key, got[key], want)
	}
}

func eventIDs(events []models.ConversationEvent) []string {
	ids := make([]string, 0, len(events))
	for _, event := range events {
		ids = append(ids, event.EventId)
	}
	return ids
}
