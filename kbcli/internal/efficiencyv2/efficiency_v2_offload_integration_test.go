//go:build integration

package efficiencyv2

import (
	"testing"
	"time"

	"kanban/core/models"
)

// 集成测试：走真实生产路径 NormalizeAndUpsertEfficiencyV2ConversationEvents
// （内含 QueryEfficiencyV2Conversations 的 A6 回读），证明正文卸载后 efficiency-v2 仍解析出 exact 工具事件。
// 这是「回读 wiring」的承重守卫——纯函数门槛测试手动 HydrateContent，抓不到「Query 漏接 HydrateContent」；
// 本测试若 QueryEfficiencyV2Conversations 未回读，正文为空 → 事件退化 → FAIL，正好拦住该 P0 回归。
// 运行：go test -tags integration（需测试 DB）。
func TestEfficiencyV2Offload_ProductionPathHydrates_Integration(t *testing.T) {
	db := openEfficiencyV2IntegrationDB(t)
	dir := t.TempDir()
	const sid = "session-offload-integ"
	const rid = "request-offload-integ"

	cleanup := func() {
		db.Where("session_id = ?", sid).Delete(&models.Conversation{})
		db.Where("session_id = ?", sid).Delete(&models.ConversationEvent{})
	}
	cleanup()
	t.Cleanup(cleanup)

	start := time.Date(2026, 5, 19, 9, 0, 0, 0, time.UTC)
	conv := models.Conversation{
		SessionId:       sid,
		RequestId:       rid,
		StartTime:       start,
		EndTime:         start.Add(60 * time.Second),
		ResponseContent: `{"tool_name":"Edit","event_kind":"edit","command":"apply_patch","touched_files":["api/handler.go"]}`,
		UserInput:       "改 handler",
	}
	if err := db.Create(&conv).Error; err != nil {
		t.Fatalf("insert conversation: %v", err)
	}

	// 卸载该行：落 blob + UPDATE 写指针 + 置空三列（模拟 offload-content 执行器动作）
	loc, err := models.OffloadConversationContent(dir, sid, rid, conv.RequestContent, conv.ResponseContent, conv.UserInput)
	if err != nil {
		t.Fatalf("offload blob: %v", err)
	}
	if err := db.Model(&models.Conversation{}).Where("session_id = ? AND request_id = ?", sid, rid).
		Updates(map[string]interface{}{
			"content_location": loc,
			"request_content":  "",
			"response_content": "",
			"user_input":       "",
		}).Error; err != nil {
		t.Fatalf("null columns: %v", err)
	}

	// 生产路径：Query(应逐行回读 blob) → Normalize。漏接 A6 则正文为空 → 退化，断言失败。
	events, err := NormalizeAndUpsertEfficiencyV2ConversationEvents(db, EfficiencyV2ConversationEventQuery{SessionID: sid})
	if err != nil {
		t.Fatalf("normalize/upsert: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("event count = %d, want 1", len(events))
	}
	if events[0].ParseQuality != efficiencyV2ParseQualityExact ||
		events[0].Source != efficiencyV2EventSourceRawTool || events[0].ToolName != "Edit" {
		t.Fatalf("卸载后生产路径未回读→事件退化: quality=%s source=%s tool=%s（应 exact/raw_tool/Edit；疑 QueryEfficiencyV2Conversations 漏接 HydrateContent）",
			events[0].ParseQuality, events[0].Source, events[0].ToolName)
	}
}
