package efficiencyv2

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
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

func TestEfficiencyV2NormalizeConversationEvents_NewCscToolEventsArray(t *testing.T) {
	start := time.Date(2026, 5, 18, 9, 30, 0, 0, time.UTC)
	conv := models.Conversation{
		SessionId:       "session-tool-array",
		RequestId:       "request-tool-array",
		StartTime:       start,
		EndTime:         start.Add(2 * time.Minute),
		RepoAddr:        "git@example.com/acme/api.git",
		RepoBranch:      "feature/tool-array",
		WorkDirId:       "workdir-tool-array",
		ResponseContent: "plain assistant text",
		ToolEvents:      models.StringJSON(`[{"name":"Edit","event_kind":"edit","tool_use_id":"tu-edit","touched_files":["api/handler.go"],"after":"package api"},{"name":"Bash","event_kind":"command","tool_use_id":"tu-bash","command":"go test ./...","exit_code":0}]`),
	}

	events, err := NormalizeEfficiencyV2ConversationEvents([]models.Conversation{conv})
	if err != nil {
		t.Fatalf("normalize new csc tool_events: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("event count = %d, want 2", len(events))
	}
	if events[0].EventId == events[1].EventId {
		t.Fatalf("tool_events array entries must not collapse to same event_id: %s", events[0].EventId)
	}
	if events[0].ToolName != "Edit" || events[0].EventKind != "edit" {
		t.Fatalf("first event = %s/%s, want Edit/edit", events[0].ToolName, events[0].EventKind)
	}
	if events[1].ToolName != "Bash" || events[1].CommandText != "go test ./..." {
		t.Fatalf("second event = %s command %q, want Bash command", events[1].ToolName, events[1].CommandText)
	}
	assertJSONStringArray(t, string(events[0].TouchedFiles), []string{"api/handler.go"})
	assertJSONObjectField(t, string(events[0].Payload), "tool_use_id", "tu-edit")
}

func TestEfficiencyV2NormalizeConversationEvents_NewCscSameToolEventsStayDistinct(t *testing.T) {
	start := time.Date(2026, 5, 18, 9, 45, 0, 0, time.UTC)
	conv := models.Conversation{
		SessionId:       "session-same-tool-array",
		RequestId:       "request-same-tool-array",
		StartTime:       start,
		EndTime:         start.Add(2 * time.Minute),
		ResponseContent: "plain assistant text",
		ToolEvents:      models.StringJSON(`[{"name":"Edit","event_kind":"edit","tool_use_id":"tu-edit-1","touched_files":["a.go"]},{"name":"Edit","event_kind":"edit","tool_use_id":"tu-edit-2","touched_files":["b.go"]}]`),
	}

	events, err := NormalizeEfficiencyV2ConversationEvents([]models.Conversation{conv})
	if err != nil {
		t.Fatalf("normalize same tool csc tool_events: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("event count = %d, want 2", len(events))
	}
	if events[0].EventId == events[1].EventId {
		t.Fatalf("same-tool events must have distinct event_id: %s", events[0].EventId)
	}
	if events[0].ToolName != "Edit" || events[1].ToolName != "Edit" {
		t.Fatalf("tool names should remain display values, got %q/%q", events[0].ToolName, events[1].ToolName)
	}
	if logicalEventIdentity(events[0]) == logicalEventIdentity(events[1]) {
		t.Fatalf("same-tool events must be distinct for ux_conversation_events_logical")
	}
	assertJSONObjectField(t, string(events[0].Payload), "tool_use_id", "tu-edit-1")
	assertJSONObjectField(t, string(events[1].Payload), "tool_use_id", "tu-edit-2")
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

func TestConversationEventPruneScopes_GroupByConversationRequest(t *testing.T) {
	scopes := conversationEventPruneScopes([]models.ConversationEvent{
		{SessionId: "s1", RequestId: "r1", EventId: "new-2"},
		{SessionId: "s1", RequestId: "r1", EventId: "new-1"},
		{SessionId: "s1", RequestId: "r2", EventId: "other-request"},
		{SessionId: "", RequestId: "r3", EventId: "ignored"},
	})

	if len(scopes) != 2 {
		t.Fatalf("scope count = %d, want 2", len(scopes))
	}
	if scopes[0].SessionId != "s1" || scopes[0].RequestId != "r1" {
		t.Fatalf("first scope key = %s/%s, want s1/r1", scopes[0].SessionId, scopes[0].RequestId)
	}
	if !reflect.DeepEqual(scopes[0].KeepEventIds, []string{"new-1", "new-2"}) {
		t.Fatalf("first scope keep ids = %#v, want sorted new ids", scopes[0].KeepEventIds)
	}
	if scopes[1].SessionId != "s1" || scopes[1].RequestId != "r2" {
		t.Fatalf("second scope key = %s/%s, want s1/r2", scopes[1].SessionId, scopes[1].RequestId)
	}
	if !reflect.DeepEqual(scopes[1].KeepEventIds, []string{"other-request"}) {
		t.Fatalf("second scope keep ids = %#v", scopes[1].KeepEventIds)
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

func logicalEventIdentity(event models.ConversationEvent) string {
	var payload map[string]interface{}
	_ = json.Unmarshal([]byte(event.Payload), &payload)
	toolIdentity := ""
	if v, ok := payload["tool_use_id"].(string); ok {
		toolIdentity = v
	} else if v, ok := payload["event_index"]; ok {
		toolIdentity = jsonNumberString(v)
	}
	if toolIdentity == "" {
		toolIdentity = event.EventId
	}
	return strings.Join([]string{
		event.SessionId,
		event.RequestId,
		event.EventStartTs.UTC().Format(time.RFC3339Nano),
		event.EventKind,
		event.Source,
		event.ToolName,
		toolIdentity,
	}, "\x00")
}

func jsonNumberString(v interface{}) string {
	switch value := v.(type) {
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	case string:
		return value
	default:
		return fmt.Sprint(value)
	}
}

func eventIDs(events []models.ConversationEvent) []string {
	ids := make([]string, 0, len(events))
	for _, event := range events {
		ids = append(ids, event.EventId)
	}
	return ids
}
