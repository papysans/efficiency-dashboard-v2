package models

import (
	"encoding/json"
	"fmt"
	"strings"

	"kanban/core/storage"
)

// 正文卸载（WS-A）：把 conversations 的 request_content/response_content/user_input 三列正文
// 从热库卸载到磁盘/对象存储，DB 仅留 ContentLocation 指针，按需回读。
//
// 设计要点（见 .trellis/tasks/06-24-db-conversations-disk/design.md）：
//   - 卸载的是 import 已清洗（StripLargeBase64+SanitizeText）、已拆到 (session_id,request_id) 粒度的字节，
//     回读逐字节一致、口径零漂移——绝不截断（硬截断曾致 ~26% 工具事件解析退化）。
//   - key 下沉到 request_id：正文是 per-request 而非 per-session；三列合写一个 JSON 对象，省 2/3 对象数。
//   - 落盘走 storage（disk/s3 自动分流），disk 后端已 temp+rename 原子写。

// conversationContentBlob 是一条对话三列正文的磁盘载体（一行一对象）。
type conversationContentBlob struct {
	RequestContent  string `json:"request_content"`
	ResponseContent string `json:"response_content"`
	UserInput       string `json:"user_input"`
}

// ConversationContentLocation 构造卸载对象的存储位置（含 scheme，disk/s3 透明）。
// 约定：analysedDir/task/conversation/content/<sessionId>/<requestId>.json。
// 写侧、回读侧、backfill 必须用本函数生成 key，保证三方一致。
// session_id/request_id 来自上游 raw-dump（DB 主键），正常是 UUID 类安全串；
// 但对含路径分隔符/点段的畸形 ID 必须先编码，否则会写出 content 目录外或与他行碰撞 →
// content_location 指向错 blob（Codex review P2）。
func ConversationContentLocation(analysedDir, sessionID, requestID string) string {
	return storage.Join(analysedDir, "task", "conversation", "content",
		sanitizePathComponent(sessionID), sanitizePathComponent(requestID)+".json")
}

// sanitizePathComponent 把 ID 编码成单个路径安全段：转义路径分隔符与 % 自身（保证编码单射、
// 畸形 ID 不与正常 ID 碰撞），并中和会被路径规范化吃成当前/父目录的纯点段（"."/".."）。
// 正常 UUID 类 ID 原样返回（无特殊字符）。
func sanitizePathComponent(s string) string {
	s = strings.ReplaceAll(s, "%", "%25") // 先转义 % 自身，保证下面的替换可逆/单射
	s = strings.ReplaceAll(s, "/", "%2F")
	s = strings.ReplaceAll(s, "\\", "%5C")
	switch s {
	case "":
		return "_"
	case ".":
		return "%2E"
	case "..":
		return "%2E%2E"
	}
	return s
}

// OffloadConversationContent 把一条对话的三列正文写成磁盘对象，返回其 location。
// 只负责「落对象」——调用方须按固定顺序「先落对象成功 → 再写 content_location → 最后置空 DB 列」，
// 任一段失败回退为不卸载（保留 DB 原列），避免 DB 指针与对象不一致产生孤儿。
func OffloadConversationContent(analysedDir, sessionID, requestID, req, resp, userInput string) (string, error) {
	loc := ConversationContentLocation(analysedDir, sessionID, requestID)
	data, err := json.Marshal(conversationContentBlob{
		RequestContent:  req,
		ResponseContent: resp,
		UserInput:       userInput,
	})
	if err != nil {
		return "", fmt.Errorf("序列化对话正文失败(session=%s request=%s): %w", sessionID, requestID, err)
	}
	if err := storage.WriteFile(loc, data); err != nil {
		return "", fmt.Errorf("写对话正文对象失败(%s): %w", loc, err)
	}
	return loc, nil
}

// EffectiveUserInputChars 返回用户输入长度（字节，与历史口径 len(UserInput) 一致）。
// 优先用导入期持久化的 UserInputChars；为 0 时回退 len(UserInput)，兼容旧行未回填 / 本就空输入。
// 不变式：必须在「卸载 user_input（置空 DB 列）」之前回填 UserInputChars，
// 否则卸载后回退到 len("")=0 会塌掉 pseudo_task 古代估时分母。
func (c Conversation) EffectiveUserInputChars() int {
	if c.UserInputChars > 0 {
		return c.UserInputChars
	}
	return len(c.UserInput)
}

// LoadConversationContent 从 location 回读三列正文。供 efficiency-v2 重建事件、backend 看原文懒加载用。
func LoadConversationContent(location string) (req, resp, userInput string, err error) {
	data, err := storage.ReadFile(location)
	if err != nil {
		return "", "", "", fmt.Errorf("读对话正文对象失败(%s): %w", location, err)
	}
	var blob conversationContentBlob
	if err := json.Unmarshal(data, &blob); err != nil {
		return "", "", "", fmt.Errorf("解析对话正文对象失败(%s): %w", location, err)
	}
	return blob.RequestContent, blob.ResponseContent, blob.UserInput, nil
}

// HydrateContent 在 ContentLocation 非空时回灌被卸载（DB 列为空）的正文，逐列只在「DB 空且 blob 非空」时填充。
// 这样 staged cutover（先只卸 response_content）期间，未卸载的列仍用 DB 值、已卸载的列从 blob 回灌，互不干扰。
// 回读失败返回 error，调用方决定是否降级（绝不能静默当空串→会复现解析退化）。
func (c *Conversation) HydrateContent() error {
	if c.ContentLocation == "" {
		return nil
	}
	req, resp, userInput, err := LoadConversationContent(c.ContentLocation)
	if err != nil {
		return err
	}
	if c.RequestContent == "" && req != "" {
		c.RequestContent = req
	}
	if c.ResponseContent == "" && resp != "" {
		c.ResponseContent = resp
	}
	if c.UserInput == "" && userInput != "" {
		c.UserInput = userInput
	}
	return nil
}
