package efficiencyv2

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"kanban/core/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	efficiencyV2EventClassMessage = "message"
	efficiencyV2EventClassRead    = "read"
	efficiencyV2EventClassEdit    = "edit"
	efficiencyV2EventClassVerify  = "verify"
	efficiencyV2EventClassOther   = "other"

	efficiencyV2StageThink  = "think"
	efficiencyV2StageExec   = "exec"
	efficiencyV2StageVerify = "verify"
	efficiencyV2StageOther  = "other"

	efficiencyV2StageConfidenceHigh    = "high"
	efficiencyV2StageConfidenceMedium  = "medium"
	efficiencyV2StageConfidenceLow     = "low"
	efficiencyV2StageConfidenceVeryLow = "very_low"
)

type efficiencyV2ClassifiedEvent struct {
	event       models.ConversationEvent
	class       string
	durationSec int64
	endTs       time.Time
}

// ClassifyEfficiencyV2Event maps a normalized conversation event to the stage
// splitter classes required by the v2 pipeline.
func ClassifyEfficiencyV2Event(event models.ConversationEvent, cfg EfficiencyV2Config) string {
	cfg = NormalizeEfficiencyV2Config(cfg)
	tool := normalizeEfficiencyV2Identifier(event.ToolName)
	kind := normalizeEfficiencyV2Identifier(event.EventKind)
	source := normalizeEfficiencyV2Identifier(event.Source)
	command := strings.TrimSpace(event.CommandText)

	if isEfficiencyV2EditTool(tool) ||
		isEfficiencyV2EditKind(kind) ||
		source == "conversationdiff" ||
		isEfficiencyV2PatchCommand(command) {
		return efficiencyV2EventClassEdit
	}
	if command != "" && isEfficiencyV2VerificationCommand(command, cfg.VerificationCommandPatterns) {
		return efficiencyV2EventClassVerify
	}
	if isEfficiencyV2ReadTool(tool) || isEfficiencyV2ReadKind(kind) {
		return efficiencyV2EventClassRead
	}
	if isEfficiencyV2MessageKind(kind) {
		return efficiencyV2EventClassMessage
	}
	return efficiencyV2EventClassOther
}

// InferEfficiencyV2EventDurationSec returns deterministic active seconds for an
// event using explicit end timestamps, next-event inference, then class defaults.
func InferEfficiencyV2EventDurationSec(event models.ConversationEvent, next *models.ConversationEvent, class string, cfg EfficiencyV2StageConfig) int64 {
	cfg = NormalizeEfficiencyV2Config(EfficiencyV2Config{Stage: cfg}).Stage
	if event.EventEndTs != nil && event.EventEndTs.After(event.EventStartTs) {
		return ceilEfficiencyV2DurationSeconds(event.EventEndTs.Sub(event.EventStartTs))
	}
	if next != nil {
		gap := next.EventStartTs.Sub(event.EventStartTs)
		maxGap := time.Duration(cfg.MaxInferredDurationGapMinutes) * time.Minute
		if gap > 0 && gap <= maxGap {
			return ceilEfficiencyV2DurationSeconds(gap)
		}
	}
	return defaultEfficiencyV2DurationSec(event, class, cfg)
}

// BuildEfficiencyV2SessionStageMetrics groups events by session and returns one
// deterministic metric row per session, ready for coordinator-level upsert.
func BuildEfficiencyV2SessionStageMetrics(events []models.ConversationEvent, cfg EfficiencyV2Config) []models.SessionStageMetric {
	if len(events) == 0 {
		return nil
	}
	bySession := make(map[string][]models.ConversationEvent)
	for _, event := range events {
		bySession[event.SessionId] = append(bySession[event.SessionId], event)
	}
	sessionIDs := make([]string, 0, len(bySession))
	for sessionID := range bySession {
		sessionIDs = append(sessionIDs, sessionID)
	}
	sort.Strings(sessionIDs)

	metrics := make([]models.SessionStageMetric, 0, len(sessionIDs))
	for _, sessionID := range sessionIDs {
		metrics = append(metrics, BuildEfficiencyV2SessionStageMetric(bySession[sessionID], cfg))
	}
	return metrics
}

func UpsertEfficiencyV2SessionStageMetrics(db *gorm.DB, metrics []models.SessionStageMetric) error {
	if len(metrics) == 0 {
		return nil
	}
	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "session_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"need_id",
			"user_id",
			"repo_addr",
			"repo_branch",
			"work_dir_id",
			"session_start_ts",
			"session_end_ts",
			"first_edit_ts",
			"last_edit_ts",
			"total_active_min",
			"total_wall_min",
			"think_active_min",
			"exec_active_min",
			"verify_active_min",
			"other_active_min",
			"think_wall_min",
			"exec_wall_min",
			"verify_wall_min",
			"other_wall_min",
			"message_event_count",
			"read_event_count",
			"edit_event_count",
			"verify_event_count",
			"other_event_count",
			"degraded_event_count",
			"event_kind_counts",
			"ai_token_ratio",
			"re_prompt_count",
			"revert_count",
			"compaction_count",
			"total_cost_usd",
			"stage_confidence",
			"confidence_reason",
			"summary",
			"summary_source",
			"updated_at",
		}),
	}).CreateInBatches(&metrics, 1000).Error
}

func BuildAndUpsertEfficiencyV2SessionStageMetrics(db *gorm.DB, events []models.ConversationEvent, cfg EfficiencyV2Config) ([]models.SessionStageMetric, error) {
	metrics := BuildEfficiencyV2SessionStageMetrics(events, cfg)
	if err := UpsertEfficiencyV2SessionStageMetrics(db, metrics); err != nil {
		return nil, err
	}
	return metrics, nil
}

// BuildEfficiencyV2SessionStageMetric computes the v2 stage metric for one
// logical session from normalized events.
func BuildEfficiencyV2SessionStageMetric(events []models.ConversationEvent, cfg EfficiencyV2Config) models.SessionStageMetric {
	cfg = NormalizeEfficiencyV2Config(cfg)
	if len(events) == 0 {
		return models.SessionStageMetric{
			EventKindCounts:  models.ObjectJSON("{}"),
			StageConfidence:  efficiencyV2StageConfidenceVeryLow,
			ConfidenceReason: "no_events",
			Summary:          "stage split: no events",
			SummarySource:    "rule",
		}
	}

	sortedEvents := append([]models.ConversationEvent(nil), events...)
	sortEfficiencyV2Events(sortedEvents)
	classified := classifyEfficiencyV2Events(sortedEvents, cfg)

	metric := models.SessionStageMetric{
		SessionId:       sortedEvents[0].SessionId,
		UserId:          firstNonEmptyEfficiencyV2Field(sortedEvents, func(e models.ConversationEvent) string { return e.UserId }),
		RepoAddr:        firstNonEmptyEfficiencyV2Field(sortedEvents, func(e models.ConversationEvent) string { return e.RepoAddr }),
		RepoBranch:      firstNonEmptyEfficiencyV2Field(sortedEvents, func(e models.ConversationEvent) string { return e.RepoBranch }),
		WorkDirId:       firstNonEmptyEfficiencyV2Field(sortedEvents, func(e models.ConversationEvent) string { return e.WorkDirId }),
		EventKindCounts: models.ObjectJSON("{}"),
		SummarySource:   "rule",
	}

	sessionStart := sortedEvents[0].EventStartTs
	sessionEnd := classified[0].endTs
	metric.SessionStartTs = &sessionStart

	eventKindCounts := make(map[string]int64)
	hasEdit := false
	degradedCount := int64(0)
	multipleSessionIDs := false
	for _, ce := range classified {
		if ce.event.SessionId != metric.SessionId {
			multipleSessionIDs = true
		}
		if ce.endTs.After(sessionEnd) {
			sessionEnd = ce.endTs
		}
		eventKindCounts[efficiencyV2EventKindKey(ce.event)]++
		if isEfficiencyV2DegradedEvent(ce.event) {
			degradedCount++
		}
		switch ce.class {
		case efficiencyV2EventClassMessage:
			metric.MessageEventCount++
		case efficiencyV2EventClassRead:
			metric.ReadEventCount++
		case efficiencyV2EventClassEdit:
			metric.EditEventCount++
			hasEdit = true
			editStart := ce.event.EventStartTs
			if metric.FirstEditTs == nil || editStart.Before(*metric.FirstEditTs) {
				metric.FirstEditTs = &editStart
			}
			if metric.LastEditTs == nil || editStart.After(*metric.LastEditTs) {
				metric.LastEditTs = &editStart
			}
		case efficiencyV2EventClassVerify:
			metric.VerifyEventCount++
		default:
			metric.OtherEventCount++
		}
	}
	metric.SessionEndTs = &sessionEnd
	metric.TotalWallMin = minutesBetweenEfficiencyV2(sessionStart, sessionEnd)
	metric.DegradedEventCount = degradedCount
	metric.EventKindCounts = efficiencyV2ObjectJSON(eventKindCounts)

	currentStage := efficiencyV2StageThink
	for _, ce := range classified {
		minutes := float64(ce.durationSec) / 60
		metric.TotalActiveMin += minutes
		stage := currentStage
		if ce.class == efficiencyV2EventClassOther {
			stage = efficiencyV2StageOther
		} else if !hasEdit {
			stage = efficiencyV2StageThink
		} else {
			switch ce.class {
			case efficiencyV2EventClassEdit:
				currentStage = efficiencyV2StageExec
				stage = efficiencyV2StageExec
			case efficiencyV2EventClassVerify:
				currentStage = efficiencyV2StageVerify
				stage = efficiencyV2StageVerify
			default:
				stage = currentStage
			}
		}
		addEfficiencyV2StageMinutes(&metric, stage, minutes)
	}

	reasons := []string{"rule_stage_split"}
	metric.StageConfidence = efficiencyV2StageConfidenceHigh
	if !hasEdit {
		reasons = append(reasons, "no_edit_session")
	}
	if degradedCount > 0 {
		reasons = append(reasons, fmt.Sprintf("degraded_input_events=%d", degradedCount))
		if degradedCount == int64(len(sortedEvents)) {
			metric.StageConfidence = efficiencyV2StageConfidenceLow
		} else {
			metric.StageConfidence = efficiencyV2StageConfidenceMedium
		}
	}
	if multipleSessionIDs {
		reasons = append(reasons, "multiple_session_ids")
		metric.StageConfidence = efficiencyV2StageConfidenceLow
	}
	metric.ConfidenceReason = strings.Join(reasons, "; ")
	metric.Summary = fmt.Sprintf(
		"stage split: think=%.2f exec=%.2f verify=%.2f other=%.2f; events message=%d read=%d edit=%d verify=%d other=%d",
		metric.ThinkActiveMin,
		metric.ExecutionActiveMin,
		metric.VerificationActiveMin,
		metric.OtherActiveMin,
		metric.MessageEventCount,
		metric.ReadEventCount,
		metric.EditEventCount,
		metric.VerifyEventCount,
		metric.OtherEventCount,
	)

	return metric
}

func NormalizeEfficiencyV2Config(cfg EfficiencyV2Config) EfficiencyV2Config {
	ApplyDefaults(&cfg)
	return cfg
}

func classifyEfficiencyV2Events(events []models.ConversationEvent, cfg EfficiencyV2Config) []efficiencyV2ClassifiedEvent {
	classified := make([]efficiencyV2ClassifiedEvent, 0, len(events))
	for i, event := range events {
		class := ClassifyEfficiencyV2Event(event, cfg)
		var next *models.ConversationEvent
		if i+1 < len(events) {
			next = &events[i+1]
		}
		durationSec := InferEfficiencyV2EventDurationSec(event, next, class, cfg.Stage)
		classified = append(classified, efficiencyV2ClassifiedEvent{
			event:       event,
			class:       class,
			durationSec: durationSec,
			endTs:       event.EventStartTs.Add(time.Duration(durationSec) * time.Second),
		})
	}
	return classified
}

func sortEfficiencyV2Events(events []models.ConversationEvent) {
	sort.SliceStable(events, func(i, j int) bool {
		if !events[i].EventStartTs.Equal(events[j].EventStartTs) {
			return events[i].EventStartTs.Before(events[j].EventStartTs)
		}
		if events[i].EventId != events[j].EventId {
			return events[i].EventId < events[j].EventId
		}
		return events[i].RequestId < events[j].RequestId
	})
}

func addEfficiencyV2StageMinutes(metric *models.SessionStageMetric, stage string, minutes float64) {
	switch stage {
	case efficiencyV2StageThink:
		metric.ThinkActiveMin += minutes
		metric.ThinkWallMin += minutes
	case efficiencyV2StageExec:
		metric.ExecutionActiveMin += minutes
		metric.ExecutionWallMin += minutes
	case efficiencyV2StageVerify:
		metric.VerificationActiveMin += minutes
		metric.VerificationWallMin += minutes
	default:
		metric.OtherActiveMin += minutes
		metric.OtherWallMin += minutes
	}
}

func defaultEfficiencyV2DurationSec(event models.ConversationEvent, class string, cfg EfficiencyV2StageConfig) int64 {
	switch class {
	case efficiencyV2EventClassEdit:
		return positiveEfficiencyV2Default(cfg.DefaultEditDurationSeconds)
	case efficiencyV2EventClassRead:
		return positiveEfficiencyV2Default(cfg.DefaultReadDurationSeconds)
	case efficiencyV2EventClassVerify:
		return positiveEfficiencyV2Default(cfg.DefaultCommandDurationSeconds)
	case efficiencyV2EventClassMessage:
		return defaultEfficiencyV2MessageDurationSec(event, cfg)
	default:
		return positiveEfficiencyV2Default(cfg.DefaultOtherDurationSeconds)
	}
}

func defaultEfficiencyV2MessageDurationSec(event models.ConversationEvent, cfg EfficiencyV2StageConfig) int64 {
	charsPerMinute := cfg.DefaultMessageCharsPerMinute
	if charsPerMinute <= 0 {
		charsPerMinute = 300
	}
	chars := efficiencyV2PayloadCharCount(event.Payload)
	if chars <= 0 {
		chars = charsPerMinute
	}
	seconds := int64(math.Ceil(float64(chars) * 60 / float64(charsPerMinute)))
	if seconds < 1 {
		return 1
	}
	return seconds
}

func efficiencyV2PayloadCharCount(payload models.ObjectJSON) int {
	if payload == "" {
		return 0
	}
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		return 0
	}
	for _, key := range []string{"char_count", "user_input_chars", "input_chars", "message_chars"} {
		if count, ok := numericEfficiencyV2PayloadValue(raw[key]); ok && count > 0 {
			return int(math.Ceil(count))
		}
	}
	for _, key := range []string{"user_input", "message", "text", "content", "request_content", "prompt"} {
		if value, ok := raw[key].(string); ok && value != "" {
			return utf8.RuneCountInString(value)
		}
	}
	return 0
}

func numericEfficiencyV2PayloadValue(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case float64:
		return v, true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func positiveEfficiencyV2Default(seconds int) int64 {
	if seconds <= 0 {
		return 1
	}
	return int64(seconds)
}

func ceilEfficiencyV2DurationSeconds(d time.Duration) int64 {
	if d <= 0 {
		return 0
	}
	seconds := int64(math.Ceil(d.Seconds()))
	if seconds < 1 {
		return 1
	}
	return seconds
}

func minutesBetweenEfficiencyV2(start, end time.Time) float64 {
	if end.Before(start) {
		return 0
	}
	return end.Sub(start).Minutes()
}

func isEfficiencyV2EditTool(tool string) bool {
	switch tool {
	case "edit", "write", "multiedit", "applypatch", "patch":
		return true
	default:
		return false
	}
}

func isEfficiencyV2EditKind(kind string) bool {
	switch kind {
	case "edit", "write", "multiedit", "applypatch", "patch", "diff", "syntheticdiff":
		return true
	default:
		return false
	}
}

func isEfficiencyV2ReadTool(tool string) bool {
	switch tool {
	case "read", "grep", "glob", "search", "websearch", "rg", "ripgrep", "find":
		return true
	default:
		return false
	}
}

func isEfficiencyV2ReadKind(kind string) bool {
	switch kind {
	case "read", "grep", "glob", "search", "fileopen", "listfiles":
		return true
	default:
		return false
	}
}

func isEfficiencyV2MessageKind(kind string) bool {
	switch kind {
	case "message", "usermessage", "assistantmessage", "user", "assistant", "conversation", "chat":
		return true
	default:
		return false
	}
}

func isEfficiencyV2PatchCommand(command string) bool {
	normalized := normalizeEfficiencyV2Command(command)
	if normalized == "" {
		return false
	}
	return strings.Contains(normalized, "apply_patch") ||
		strings.Contains(normalized, "*** begin patch") ||
		hasEfficiencyV2CommandPrefix(normalized, "git apply") ||
		hasEfficiencyV2CommandPrefix(normalized, "patch")
}

func isEfficiencyV2VerificationCommand(command string, patterns []string) bool {
	normalizedCommand := normalizeEfficiencyV2Command(command)
	if normalizedCommand == "" {
		return false
	}
	for _, pattern := range patterns {
		normalizedPattern := normalizeEfficiencyV2Command(pattern)
		if normalizedPattern == "" {
			continue
		}
		if containsEfficiencyV2CommandPattern(normalizedCommand, normalizedPattern) {
			return true
		}
	}
	return false
}

func containsEfficiencyV2CommandPattern(command, pattern string) bool {
	if command == pattern || strings.HasPrefix(command, pattern+" ") {
		return true
	}
	index := strings.Index(command, pattern)
	for index >= 0 {
		beforeOK := index == 0 || isEfficiencyV2CommandBoundary(rune(command[index-1]))
		afterIndex := index + len(pattern)
		afterOK := afterIndex == len(command) || isEfficiencyV2CommandBoundary(rune(command[afterIndex]))
		if beforeOK && afterOK {
			return true
		}
		nextIndex := strings.Index(command[index+1:], pattern)
		if nextIndex < 0 {
			return false
		}
		index += nextIndex + 1
	}
	return false
}

func hasEfficiencyV2CommandPrefix(command, prefix string) bool {
	return command == prefix || strings.HasPrefix(command, prefix+" ")
}

func isEfficiencyV2CommandBoundary(r rune) bool {
	return unicode.IsSpace(r) || strings.ContainsRune(";|&()[]{}'\"`:", r)
}

func normalizeEfficiencyV2Command(command string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(command))), " ")
}

func normalizeEfficiencyV2Identifier(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func isEfficiencyV2DegradedEvent(event models.ConversationEvent) bool {
	quality := strings.ToLower(strings.TrimSpace(event.ParseQuality))
	return quality != "" && quality != "exact"
}

func efficiencyV2EventKindKey(event models.ConversationEvent) string {
	kind := strings.TrimSpace(strings.ToLower(event.EventKind))
	if kind == "" {
		return "unknown"
	}
	return kind
}

func efficiencyV2ObjectJSON(counts map[string]int64) models.ObjectJSON {
	if len(counts) == 0 {
		return models.ObjectJSON("{}")
	}
	data, err := json.Marshal(counts)
	if err != nil {
		return models.ObjectJSON("{}")
	}
	return models.ObjectJSON(data)
}

func firstNonEmptyEfficiencyV2Field(events []models.ConversationEvent, field func(models.ConversationEvent) string) string {
	for _, event := range events {
		if value := strings.TrimSpace(field(event)); value != "" {
			return value
		}
	}
	return ""
}
