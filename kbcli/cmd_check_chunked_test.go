package main

import (
	"testing"

	"kanban/core/rawdump"
	"kanban/core/storage"
)

// TestCheckScanConversations_GroupsChunks 锁定 check 的 scanConversations 对分片布局按
// sessionId 归组(而非把每个 00000N.jsonl 当独立会话), 避免巡检误报 orphan/missing-conversation。
func TestCheckScanConversations_GroupsChunks(t *testing.T) {
	taskDir := t.TempDir()
	convDir := storage.Join(taskDir, "conversation")

	// 旧单文件
	if err := storage.WriteFile(storage.Join(convDir, "2026", "05", "13", "sid-old.jsonl"),
		[]byte(`{"request_id":"r1"}`+"\n")); err != nil {
		t.Fatal(err)
	}
	// 新分片(2 片)
	if err := storage.WriteFile(storage.Join(convDir, "2026", "05", "14", "sid-new", "000001.jsonl"),
		[]byte(`{"request_id":"r1"}`+"\n")); err != nil {
		t.Fatal(err)
	}
	if err := storage.WriteFile(storage.Join(convDir, "2026", "05", "14", "sid-new", "000002.jsonl"),
		[]byte(`{"request_id":"r2"}`+"\n")); err != nil {
		t.Fatal(err)
	}

	ctx := &checkContext{taskDir: taskDir, convMap: make(map[string]rawdump.ConversationRef)}
	if err := ctx.scanConversations(); err != nil {
		t.Fatalf("scanConversations: %v", err)
	}
	if len(ctx.convMap) != 2 {
		t.Fatalf("期望按 sessionId 归出 2 个会话, 得到 %d: %v", len(ctx.convMap), ctx.convMap)
	}
	if got := ctx.convMap["sid-new"].ChunkCount(); got != 2 {
		t.Fatalf("sid-new 应聚合 2 片, 得到 %d", got)
	}
	if got := ctx.convMap["sid-old"].ChunkCount(); got != 1 {
		t.Fatalf("sid-old 应 1 片, 得到 %d", got)
	}
}
