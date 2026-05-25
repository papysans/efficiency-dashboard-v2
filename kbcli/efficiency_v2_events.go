package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"kanban/core/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	efficiencyV2EventSourceRawTool          = "raw_tool"
	efficiencyV2EventSourceConversationDiff = "conversation_diff"
	efficiencyV2EventSourceSynthetic        = "synthetic"

	efficiencyV2ParseQualityExact    = "exact"
	efficiencyV2ParseQualityDegraded = "degraded"
)

type efficiencyV2ConversationEventQuery struct {
	Date      string
	StartDate string
	EndDate   string
	SessionID string
}

// NormalizeEfficiencyV2ConversationEvents builds normalized v2 event rows from
// already imported legacy conversations. It does not modify legacy rows.
func NormalizeEfficiencyV2ConversationEvents(conversations []models.Conversation) ([]models.ConversationEvent, error) {
	events := make([]models.ConversationEvent, 0, len(conversations))
	for _, conv := range conversations {
		event, ok, err := buildEfficiencyV2ConversationEvent(conv)
		if err != nil {
			return nil, err
		}
		if ok {
			events = append(events, event)
		}
	}
	return events, nil
}

func buildEfficiencyV2ConversationEvent(conv models.Conversation) (models.ConversationEvent, bool, error) {
	if toolEvent, ok, err := extractEfficiencyV2RawToolEvent(conv); ok || err != nil {
		return toolEvent, ok, err
	}
	if conv.DiffLines > 0 {
		return buildEfficiencyV2FallbackEvent(conv, "edit", efficiencyV2EventSourceConversationDiff, map[string]interface{}{
			"diff_lines": conv.DiffLines,
		})
	}
	if conversationHasActivity(conv) {
		return buildEfficiencyV2FallbackEvent(conv, "message", efficiencyV2EventSourceSynthetic, map[string]interface{}{
			"fallback_reason": "conversation_activity",
		})
	}
	return models.ConversationEvent{}, false, nil
}

func extractEfficiencyV2RawToolEvent(conv models.Conversation) (models.ConversationEvent, bool, error) {
	for _, raw := range []string{conv.RequestContent, conv.ResponseContent} {
		payload, ok, err := parseEfficiencyV2JSONObject(raw)
		if err != nil {
			return models.ConversationEvent{}, false, err
		}
		if !ok || !hasEfficiencyV2ToolMarker(payload) {
			continue
		}

		eventKind := firstString(payload, "event_kind", "fixture_event_kind", "kind")
		if eventKind == "" {
			eventKind = "other"
		}
		toolName := firstString(payload, "tool_name", "tool", "name")
		command := firstString(payload, "command", "command_text")
		touchedFiles := stringSliceValue(payload["touched_files"])
		if len(touchedFiles) == 0 {
			touchedFiles = stringSliceValue(payload["mock_files"])
		}

		return buildEfficiencyV2Event(conv, eventKind, efficiencyV2EventSourceRawTool, efficiencyV2ParseQualityExact, toolName, command, touchedFiles, payload)
	}
	return models.ConversationEvent{}, false, nil
}

func buildEfficiencyV2FallbackEvent(conv models.Conversation, eventKind, source string, extraPayload map[string]interface{}) (models.ConversationEvent, bool, error) {
	payload := map[string]interface{}{
		"request_id": conv.RequestId,
		"session_id": conv.SessionId,
		"source":     source,
	}
	for k, v := range extraPayload {
		payload[k] = v
	}
	return buildEfficiencyV2Event(conv, eventKind, source, efficiencyV2ParseQualityDegraded, "", "", nil, payload)
}

func buildEfficiencyV2Event(conv models.Conversation, eventKind, source, parseQuality, toolName, commandText string, touchedFiles []string, payload map[string]interface{}) (models.ConversationEvent, bool, error) {
	start := conv.StartTime
	if start.IsZero() {
		if conv.CreatedAt.IsZero() {
			return models.ConversationEvent{}, false, nil
		}
		start = conv.CreatedAt
	}

	var end *time.Time
	durationSec := int64(0)
	if !conv.EndTime.IsZero() && !conv.EndTime.Before(start) {
		endValue := conv.EndTime
		end = &endValue
		durationSec = int64(endValue.Sub(start).Seconds())
	}

	touchedFilesJSON, err := marshalEfficiencyV2JSONArray(touchedFiles)
	if err != nil {
		return models.ConversationEvent{}, false, err
	}
	payloadJSON, err := marshalEfficiencyV2JSONObject(payload)
	if err != nil {
		return models.ConversationEvent{}, false, err
	}

	event := models.ConversationEvent{
		EventId:      BuildEfficiencyV2ConversationEventID(conv, eventKind, source, toolName, start),
		SessionId:    conv.SessionId,
		RequestId:    conv.RequestId,
		TaskId:       conv.TaskId,
		RepoAddr:     conv.RepoAddr,
		RepoBranch:   conv.RepoBranch,
		WorkDirId:    conv.WorkDirId,
		EventStartTs: start,
		EventEndTs:   end,
		DurationSec:  durationSec,
		EventKind:    eventKind,
		ToolName:     toolName,
		CommandText:  commandText,
		TouchedFiles: models.StringJSON(touchedFilesJSON),
		Payload:      models.ObjectJSON(payloadJSON),
		Source:       source,
		ParseQuality: parseQuality,
	}
	return event, true, nil
}

func BuildEfficiencyV2ConversationEventID(conv models.Conversation, eventKind, source, toolName string, start time.Time) string {
	parts := []string{
		conv.SessionId,
		conv.RequestId,
		start.UTC().Format(time.RFC3339Nano),
		eventKind,
		source,
		toolName,
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return "cev_" + hex.EncodeToString(sum[:16])
}

func UpsertEfficiencyV2ConversationEvents(db *gorm.DB, events []models.ConversationEvent) error {
	if len(events) == 0 {
		return nil
	}
	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "event_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"session_id",
			"request_id",
			"task_id",
			"user_id",
			"repo_addr",
			"repo_branch",
			"work_dir_id",
			"event_start_ts",
			"event_end_ts",
			"duration_sec",
			"event_kind",
			"tool_name",
			"command_text",
			"touched_files",
			"payload",
			"source",
			"parse_quality",
			"updated_at",
		}),
	}).CreateInBatches(&events, 1000).Error
}

func NormalizeAndUpsertEfficiencyV2ConversationEvents(db *gorm.DB, query efficiencyV2ConversationEventQuery) ([]models.ConversationEvent, error) {
	conversations, err := QueryEfficiencyV2Conversations(db, query)
	if err != nil {
		return nil, err
	}
	events, err := NormalizeEfficiencyV2ConversationEvents(conversations)
	if err != nil {
		return nil, err
	}
	if err := hydrateEfficiencyV2EventUsers(db, events); err != nil {
		return nil, err
	}
	if err := UpsertEfficiencyV2ConversationEvents(db, events); err != nil {
		return nil, err
	}
	return events, nil
}

func QueryEfficiencyV2Conversations(db *gorm.DB, query efficiencyV2ConversationEventQuery) ([]models.Conversation, error) {
	tx := db.Model(&models.Conversation{}).Order("session_id ASC").Order("start_time ASC").Order("request_id ASC")
	if query.SessionID != "" {
		tx = tx.Where("session_id = ?", query.SessionID)
	}
	if query.Date != "" {
		tx = tx.Where("DATE(start_time) = ?", query.Date)
	}
	if query.StartDate != "" {
		tx = tx.Where("DATE(start_time) >= ?", query.StartDate)
	}
	if query.EndDate != "" {
		tx = tx.Where("DATE(start_time) <= ?", query.EndDate)
	}

	var conversations []models.Conversation
	if err := tx.Find(&conversations).Error; err != nil {
		return nil, err
	}
	return conversations, nil
}

func hydrateEfficiencyV2EventUsers(db *gorm.DB, events []models.ConversationEvent) error {
	if len(events) == 0 {
		return nil
	}
	sessionIDs := make([]string, 0, len(events))
	seen := map[string]bool{}
	for _, event := range events {
		if event.SessionId == "" || seen[event.SessionId] {
			continue
		}
		seen[event.SessionId] = true
		sessionIDs = append(sessionIDs, event.SessionId)
	}
	if len(sessionIDs) == 0 {
		return nil
	}
	var sessions []models.Session
	if err := db.Where("session_id IN ?", sessionIDs).Find(&sessions).Error; err != nil {
		return err
	}
	usersBySession := map[string]string{}
	for _, session := range sessions {
		usersBySession[session.SessionId] = session.UserId
	}
	for i := range events {
		if userID := usersBySession[events[i].SessionId]; userID != "" {
			events[i].UserId = userID
		}
	}
	return nil
}

func parseEfficiencyV2JSONObject(raw string) (map[string]interface{}, bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, false, nil
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, false, nil
	}
	return payload, true, nil
}

func hasEfficiencyV2ToolMarker(payload map[string]interface{}) bool {
	for _, key := range []string{"tool_name", "command", "event_kind", "fixture_event_kind", "touched_files", "payload"} {
		if _, ok := payload[key]; ok {
			return true
		}
	}
	return false
}

func firstString(payload map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		value, ok := payload[key]
		if !ok {
			continue
		}
		if s, ok := value.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func stringSliceValue(value interface{}) []string {
	switch v := value.(type) {
	case []string:
		return append([]string(nil), v...)
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		return []string{strings.TrimSpace(v)}
	default:
		return nil
	}
}

func conversationHasActivity(conv models.Conversation) bool {
	if !conv.StartTime.IsZero() || !conv.CreatedAt.IsZero() {
		return true
	}
	return strings.TrimSpace(conv.UserInput) != "" ||
		strings.TrimSpace(conv.RequestContent) != "" ||
		strings.TrimSpace(conv.ResponseContent) != ""
}

func marshalEfficiencyV2JSONArray(values []string) (string, error) {
	if values == nil {
		values = []string{}
	}
	values = append([]string(nil), values...)
	sort.Strings(values)
	data, err := json.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("marshal touched files: %w", err)
	}
	return string(data), nil
}

func marshalEfficiencyV2JSONObject(payload map[string]interface{}) (string, error) {
	if payload == nil {
		payload = map[string]interface{}{}
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}
	return string(data), nil
}
