package main

import (
	"fmt"
	"testing"

	"kanban/core/storage"
)

// convRecord 造一条最小可过 checkConversation 的对话记录(需 request_id + 合法起止时间)。
func convRecord(reqID string) string {
	return fmt.Sprintf(
		`{"request_id":%q,"start_time":"2026-05-13T10:00:00Z","end_time":"2026-05-13T10:01:00Z","user_input":"hi"}`,
		reqID) + "\n"
}

// TestScanAndReassemble_MixedLayout 验证 import 发现+重组对「旧单文件」与「新目录分片」两种布局都成立。
func TestScanAndReassemble_MixedLayout(t *testing.T) {
	root := t.TempDir()
	convDir := storage.Join(root, "conversation")

	// 旧单文件: conversation/2026/05/13/sid-old.jsonl, 2 条记录
	oldFile := storage.Join(convDir, "2026", "05", "13", "sid-old.jsonl")
	if err := storage.WriteFile(oldFile, []byte(convRecord("old-r1")+convRecord("old-r2"))); err != nil {
		t.Fatal(err)
	}

	// 新分片: conversation/2026/05/14/sid-new/000001..000003.jsonl, 每片一条
	newDir := storage.Join(convDir, "2026", "05", "14", "sid-new")
	// 故意乱序写, 验证重组按数字序
	if err := storage.WriteFile(storage.Join(newDir, "000003.jsonl"), []byte(convRecord("new-r3"))); err != nil {
		t.Fatal(err)
	}
	if err := storage.WriteFile(storage.Join(newDir, "000001.jsonl"), []byte(convRecord("new-r1"))); err != nil {
		t.Fatal(err)
	}
	if err := storage.WriteFile(storage.Join(newDir, "000002.jsonl"), []byte(convRecord("new-r2"))); err != nil {
		t.Fatal(err)
	}

	convMap, err := scanConversationFiles(convDir, nil, nil)
	if err != nil {
		t.Fatalf("scanConversationFiles: %v", err)
	}
	if len(convMap) != 2 {
		t.Fatalf("期望 2 个 session, 得到 %d: %v", len(convMap), convMap)
	}

	// 旧单文件 session
	oldSrc, ok := convMap["sid-old"]
	if !ok {
		t.Fatal("缺 sid-old")
	}
	if oldSrc.date != "2026/05/13" || oldSrc.ref.ChunkCount() != 1 {
		t.Fatalf("sid-old 不符: date=%s chunks=%d", oldSrc.date, oldSrc.ref.ChunkCount())
	}
	oldConvs, err := parseConversations(oldSrc.ref)
	if err != nil {
		t.Fatalf("解析 sid-old: %v", err)
	}
	if len(oldConvs) != 2 || oldConvs[0].RequestId != "old-r1" || oldConvs[1].RequestId != "old-r2" {
		t.Fatalf("sid-old 解析结果不符: %+v", reqIDs(oldConvs))
	}

	// 新分片 session
	newSrc, ok := convMap["sid-new"]
	if !ok {
		t.Fatal("缺 sid-new")
	}
	if newSrc.date != "2026/05/14" || newSrc.ref.ChunkCount() != 3 {
		t.Fatalf("sid-new 不符: date=%s chunks=%d", newSrc.date, newSrc.ref.ChunkCount())
	}
	newConvs, err := parseConversations(newSrc.ref)
	if err != nil {
		t.Fatalf("解析 sid-new: %v", err)
	}
	got := reqIDs(newConvs)
	want := []string{"new-r1", "new-r2", "new-r3"}
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("sid-new 分片重组顺序错误: 得到 %v 期望 %v", got, want)
	}
}

// TestScanConversationFiles_LatestDateWins 验证同一 session 跨日期目录时只保留最新日期那组分片。
func TestScanConversationFiles_LatestDateWins(t *testing.T) {
	root := t.TempDir()
	convDir := storage.Join(root, "conversation")
	// 同一 sid 在 05/13(旧, 1 片) 与 05/14(新, 2 片) 都出现
	if err := storage.WriteFile(storage.Join(convDir, "2026", "05", "13", "sid.jsonl"), []byte(convRecord("d13"))); err != nil {
		t.Fatal(err)
	}
	if err := storage.WriteFile(storage.Join(convDir, "2026", "05", "14", "sid", "000001.jsonl"), []byte(convRecord("d14-r1"))); err != nil {
		t.Fatal(err)
	}
	if err := storage.WriteFile(storage.Join(convDir, "2026", "05", "14", "sid", "000002.jsonl"), []byte(convRecord("d14-r2"))); err != nil {
		t.Fatal(err)
	}

	convMap, err := scanConversationFiles(convDir, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	src, ok := convMap["sid"]
	if !ok {
		t.Fatal("缺 sid")
	}
	if src.date != "2026/05/14" || src.ref.ChunkCount() != 2 {
		t.Fatalf("应只保留最新日期 05/14 的 2 片, 得到 date=%s chunks=%d (paths=%v)",
			src.date, src.ref.ChunkCount(), src.ref.Paths)
	}
}

// TestScanConversationFiles_SameDateSingleFileWins 验证同一 session 同日期新旧布局并存时，
// scanConversationFiles 选单文件（与 rawdump.Resolve 一致），不把两套混入一个 ref。
func TestScanConversationFiles_SameDateSingleFileWins(t *testing.T) {
	root := t.TempDir()
	convDir := storage.Join(root, "conversation")
	if err := storage.WriteFile(storage.Join(convDir, "2026", "05", "13", "sid.jsonl"),
		[]byte(convRecord("single-r1"))); err != nil {
		t.Fatal(err)
	}
	if err := storage.WriteFile(storage.Join(convDir, "2026", "05", "13", "sid", "000001.jsonl"),
		[]byte(convRecord("chunk-r1"))); err != nil {
		t.Fatal(err)
	}

	convMap, err := scanConversationFiles(convDir, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	src, ok := convMap["sid"]
	if !ok {
		t.Fatal("缺 sid")
	}
	if src.ref.ChunkCount() != 1 {
		t.Fatalf("同日期应只取单文件 1 个来源, 得到 %d (paths=%v)", src.ref.ChunkCount(), src.ref.Paths)
	}
	convs, err := parseConversations(src.ref)
	if err != nil {
		t.Fatal(err)
	}
	if len(convs) != 1 || convs[0].RequestId != "single-r1" {
		t.Fatalf("应取单文件记录, 得到 %v", reqIDs(convs))
	}
}

func reqIDs(cs []taskConversation) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.RequestId
	}
	return out
}
