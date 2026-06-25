package efficiencyv2

import (
	"testing"
	"time"

	"kanban/core/models"
)

// 锁住 design.md「激活硬门槛」#2/#6：证明正文卸载后
//
//	(a) 缺 A6 回读 → efficiency-v2 工具事件静默退化（exact→degraded），即 A6 是承重的；
//	(b) 接 A6 回读(HydrateContent) → 解析逐字节一致、0 退化。
//
// 纯函数路径（NormalizeEfficiencyV2ConversationEvents），不依赖 DB。
func TestEfficiencyV2Offload_A6HydrateRoundtrip(t *testing.T) {
	dir := t.TempDir()
	start := time.Date(2026, 5, 18, 9, 0, 0, 0, time.UTC)
	newConv := func() models.Conversation {
		return models.Conversation{
			SessionId:       "session-offload",
			RequestId:       "request-offload",
			TaskId:          "task-offload",
			StartTime:       start,
			EndTime:         start.Add(90 * time.Second),
			RepoAddr:        "git@example.com/acme/api.git",
			RepoBranch:      "feature/offload",
			WorkDirId:       "workdir-offload",
			UserInput:       "改一下 handler",
			ResponseContent: `{"tool_name":"Edit","event_kind":"edit","command":"apply_patch","touched_files":["api/handler.go"],"payload":{"bytes":42}}`,
		}
	}

	// 基线：未卸载，应得 exact raw_tool 事件
	baseConv := newConv()
	baseEvents, err := NormalizeEfficiencyV2ConversationEvents([]models.Conversation{baseConv})
	if err != nil {
		t.Fatalf("baseline normalize: %v", err)
	}
	if len(baseEvents) != 1 || baseEvents[0].Source != "raw_tool" || baseEvents[0].ParseQuality != "exact" {
		t.Fatalf("基线应为单条 exact raw_tool 事件, got %d 条 source=%v quality=%v",
			len(baseEvents), eventField(baseEvents, "Source"), eventField(baseEvents, "ParseQuality"))
	}

	// 卸载：正文落盘 + 三列置空 + 写 ContentLocation
	offConv := newConv()
	if err := offConv.Offload(dir); err != nil {
		t.Fatalf("offload: %v", err)
	}
	if offConv.ResponseContent != "" || offConv.RequestContent != "" || offConv.UserInput != "" {
		t.Fatal("卸载后三列应置空")
	}
	if offConv.ContentLocation == "" {
		t.Fatal("卸载后 ContentLocation 应非空")
	}
	if offConv.UserInputChars != len("改一下 handler") {
		t.Fatalf("卸载应保住 UserInputChars=%d, got %d", len("改一下 handler"), offConv.UserInputChars)
	}

	// (a) 缺 A6 回读：空正文 → 工具事件退化。承重断言写成「不应有任何 exact 事件」，
	// 不钉死 source（防 exact 发射点未来扩散导致门槛假绿）。
	noHydrate := offConv // 值拷贝，不调用 HydrateContent
	degEvents, err := NormalizeEfficiencyV2ConversationEvents([]models.Conversation{noHydrate})
	if err != nil {
		t.Fatalf("degraded normalize: %v", err)
	}
	for _, e := range degEvents {
		if e.ParseQuality == efficiencyV2ParseQualityExact {
			t.Fatalf("缺 A6 回读不应得 exact 事件(source=%s)——门槛测试失效/卸载会污染主指标", e.Source)
		}
	}

	// (b) 接 A6 回读：HydrateContent 还原正文 → 解析与基线逐字节一致
	if err := offConv.HydrateContent(); err != nil {
		t.Fatalf("hydrate: %v", err)
	}
	if offConv.ResponseContent != baseConv.ResponseContent || offConv.UserInput != "改一下 handler" {
		t.Fatal("回读后正文应逐字节还原")
	}
	restored, err := NormalizeEfficiencyV2ConversationEvents([]models.Conversation{offConv})
	if err != nil {
		t.Fatalf("restored normalize: %v", err)
	}
	if len(restored) != len(baseEvents) {
		t.Fatalf("回读后事件数 %d != 基线 %d", len(restored), len(baseEvents))
	}
	b, r := baseEvents[0], restored[0]
	if r.EventId != b.EventId || r.Source != b.Source || r.ParseQuality != b.ParseQuality ||
		r.EventKind != b.EventKind || r.ToolName != b.ToolName || r.CommandText != b.CommandText ||
		string(r.TouchedFiles) != string(b.TouchedFiles) || string(r.Payload) != string(b.Payload) {
		t.Fatalf("回读后事件与基线不一致:\n base=%#v\n rest=%#v", b, r)
	}
}

func eventField(events []models.ConversationEvent, f string) string {
	if len(events) == 0 {
		return "<none>"
	}
	switch f {
	case "Source":
		return events[0].Source
	case "ParseQuality":
		return events[0].ParseQuality
	}
	return ""
}
