package models

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestOffloadAndLoadConversationContent_ByteIdentical(t *testing.T) {
	dir := t.TempDir()
	// 含工具事件 JSON + 占位符 + 多字节，模拟真实清洗后正文，验证逐字节一致（绝不截断）。
	req := `{"tool_name":"edit_file","command":"go build","touched_files":["a.go"]}` + strings.Repeat("中文", 1000)
	resp := `[base64 12345B omitted]` + "\n响应正文\t带制表符"
	userInput := "用户提问：为什么 14GB？"

	loc, err := OffloadConversationContent(dir, "sess-1", "req-1", req, resp, userInput)
	if err != nil {
		t.Fatalf("offload 失败: %v", err)
	}
	want := filepath.Join(dir, "task", "conversation", "content", "sess-1", "req-1.json")
	if loc != want {
		t.Fatalf("location 不符:\n got %q\nwant %q", loc, want)
	}

	gotReq, gotResp, gotUser, err := LoadConversationContent(loc)
	if err != nil {
		t.Fatalf("load 失败: %v", err)
	}
	if gotReq != req || gotResp != resp || gotUser != userInput {
		t.Fatalf("回读非逐字节一致:\nreq %v\nresp %v\nuser %v",
			gotReq == req, gotResp == resp, gotUser == userInput)
	}
}

func TestHydrateContent_StagedOnlyFillsEmpty(t *testing.T) {
	dir := t.TempDir()
	loc, err := OffloadConversationContent(dir, "s", "r", "REQ", "RESP", "UIN")
	if err != nil {
		t.Fatalf("offload 失败: %v", err)
	}

	// staged cutover：只卸了 response_content（DB 列置空），request/user_input 仍在 DB。
	c := Conversation{
		ContentLocation: loc,
		RequestContent:  "DB_REQ_未卸载",
		ResponseContent: "", // 已卸载置空
		UserInput:       "DB_UIN_未卸载",
	}
	if err := c.HydrateContent(); err != nil {
		t.Fatalf("hydrate 失败: %v", err)
	}
	if c.RequestContent != "DB_REQ_未卸载" {
		t.Errorf("未卸载列被覆盖: %q", c.RequestContent)
	}
	if c.ResponseContent != "RESP" {
		t.Errorf("已卸载列未回灌: %q", c.ResponseContent)
	}
	if c.UserInput != "DB_UIN_未卸载" {
		t.Errorf("未卸载列被覆盖: %q", c.UserInput)
	}
}

func TestConversationContentLocation_PathSafe(t *testing.T) {
	dir := "/base"
	// 畸形 ID 含分隔符/点段：不得逃逸 content 目录，且与正常 ID 不碰撞。
	cases := []struct{ sid, rid string }{
		{"../../etc", "passwd"},
		{"s", "../../../tmp/x"},
		{"..", ".."},
		{"a/b", "c\\d"},
	}
	contentRoot := filepath.Join(dir, "task", "conversation", "content")
	prefix := contentRoot + string(filepath.Separator)
	seen := map[string]bool{}
	for _, c := range cases {
		loc := ConversationContentLocation(dir, c.sid, c.rid)
		// 真正的安全属性：规范化后仍在 content 根下，且无任何路径段是真正的 ./.. （分隔符已转义→畸形 ID 退化为单个字面段）
		clean := filepath.Clean(loc)
		if !strings.HasPrefix(clean, prefix) {
			t.Errorf("逃逸 content 目录: sid=%q rid=%q → %q", c.sid, c.rid, clean)
		}
		for _, seg := range strings.Split(strings.TrimPrefix(clean, prefix), string(filepath.Separator)) {
			if seg == "." || seg == ".." {
				t.Errorf("含可遍历段 %q: %q", seg, clean)
			}
		}
		if seen[loc] {
			t.Errorf("不同 ID 碰撞到同一 loc: %q", loc)
		}
		seen[loc] = true
	}
	// 单射：正常 ID 与「编码后恰好等于正常 ID」的畸形 ID 不得碰撞
	if ConversationContentLocation(dir, "a", "b") == ConversationContentLocation(dir, "a", "b%2F") {
		t.Error("编码非单射，存在碰撞")
	}
	// 正常 UUID 类 ID 原样（不被编码污染）
	if got := ConversationContentLocation(dir, "sess-1", "req-1"); !strings.HasSuffix(got, "/sess-1/req-1.json") {
		t.Errorf("正常 ID 不应被编码: %q", got)
	}
}

func TestHydrateContent_NoLocationNoop(t *testing.T) {
	c := Conversation{RequestContent: "x"}
	if err := c.HydrateContent(); err != nil {
		t.Fatalf("无 location 应 no-op: %v", err)
	}
	if c.RequestContent != "x" {
		t.Errorf("无 location 不应改动: %q", c.RequestContent)
	}
}
