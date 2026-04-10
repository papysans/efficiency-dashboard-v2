package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEstimateTaskMinutes_PromptTemplate 测试占位符替换
func TestEstimateTaskMinutes_PromptTemplate(t *testing.T) {
	var capturedBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf, _ := io.ReadAll(r.Body)
		capturedBody = string(buf)
		w.WriteHeader(200)
		resp := `{"content":[{"text":"{\"task_ancient_minutes\":60.0,\"task_ancient_minutes_reason\":\"test\"}"}]}`
		w.Write([]byte(resp))
	}))
	defer server.Close()

	config := AIEstimationConfig{
		BaseURL:   server.URL,
		APIKey:    "test-key",
		Model:     "test-model",
		TimeoutMS: 5000,
		Prompt:    "inputs:{{user_inputs}} code:{{code_outputs}} chars:{{total_chars}} lines:{{total_lines}}",
	}
	taskContent := &TaskContentFile{
		TotalUserInChars: 42,
		TotalCodeLines:   100,
		Conversations: []ConversationEntry{
			{
				Timestamp: "2026-03-31T09:00:00Z",
				UserInput: "implement login",
				CodeOutputs: []CodeOutput{
					{Path: "login.go", Code: "package main"},
				},
			},
		},
	}

	_, _, err := EstimateTaskMinutes(config, taskContent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(capturedBody, "implement login") {
		t.Error("prompt 中应包含 user_inputs 内容")
	}
	if !strings.Contains(capturedBody, "login.go") {
		t.Error("prompt 中应包含 code_outputs 内容")
	}
	if !strings.Contains(capturedBody, "42") {
		t.Error("prompt 中应包含 total_chars")
	}
	if !strings.Contains(capturedBody, "100") {
		t.Error("prompt 中应包含 total_lines")
	}
}

// TestEstimateTaskMinutes_NormalResponse 正常响应
func TestEstimateTaskMinutes_NormalResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		resp := `{"content":[{"text":"{\"task_ancient_minutes\":210.0,\"task_ancient_minutes_reason\":\"medium complexity\"}"}]}`
		w.Write([]byte(resp))
	}))
	defer server.Close()

	config := AIEstimationConfig{
		BaseURL:   server.URL,
		APIKey:    "test-key",
		Model:     "test-model",
		TimeoutMS: 5000,
	}
	taskContent := &TaskContentFile{
		Conversations: []ConversationEntry{
			{Timestamp: "2026-03-31T09:00:00Z", UserInput: "hello"},
		},
	}

	minutes, reason, err := EstimateTaskMinutes(config, taskContent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if minutes != 210.0 {
		t.Errorf("minutes: want 210.0, got %f", minutes)
	}
	if reason != "medium complexity" {
		t.Errorf("reason: want 'medium complexity', got %s", reason)
	}
}

// TestEstimateTaskMinutes_MarkdownCodeBlock 返回含 markdown 代码块的响应
func TestEstimateTaskMinutes_MarkdownCodeBlock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		// AI 返回 markdown 包裹的 JSON
		text := "```json\n{\"task_ancient_minutes\":120.0,\"task_ancient_minutes_reason\":\"simple task\"}\n```"
		respObj := map[string]interface{}{
			"content": []map[string]string{{"text": text}},
		}
		json.NewEncoder(w).Encode(respObj)
	}))
	defer server.Close()

	config := AIEstimationConfig{
		BaseURL:   server.URL,
		APIKey:    "test-key",
		Model:     "test-model",
		TimeoutMS: 5000,
	}
	taskContent := &TaskContentFile{
		Conversations: []ConversationEntry{
			{Timestamp: "2026-03-31T09:00:00Z", UserInput: "hello"},
		},
	}

	minutes, reason, err := EstimateTaskMinutes(config, taskContent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if minutes != 120.0 {
		t.Errorf("minutes: want 120.0, got %f", minutes)
	}
	if reason != "simple task" {
		t.Errorf("reason: want 'simple task', got %s", reason)
	}
}

// TestEstimateTaskMinutes_Non200 API 返回非 200 状态码
func TestEstimateTaskMinutes_Non200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer server.Close()

	config := AIEstimationConfig{
		BaseURL:   server.URL,
		APIKey:    "test-key",
		Model:     "test-model",
		TimeoutMS: 5000,
	}
	taskContent := &TaskContentFile{
		Conversations: []ConversationEntry{
			{Timestamp: "2026-03-31T09:00:00Z", UserInput: "hello"},
		},
	}

	_, _, err := EstimateTaskMinutes(config, taskContent)
	if err == nil {
		t.Error("期望返回 error, 但未返回")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error 应包含状态码 500, got: %s", err.Error())
	}
}

// TestEstimateTaskMinutes_InvalidJSON JSON 解析失败
func TestEstimateTaskMinutes_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		resp := `{"content":[{"text":"not valid json"}]}`
		w.Write([]byte(resp))
	}))
	defer server.Close()

	config := AIEstimationConfig{
		BaseURL:   server.URL,
		APIKey:    "test-key",
		Model:     "test-model",
		TimeoutMS: 5000,
	}
	taskContent := &TaskContentFile{
		Conversations: []ConversationEntry{
			{Timestamp: "2026-03-31T09:00:00Z", UserInput: "hello"},
		},
	}

	_, _, err := EstimateTaskMinutes(config, taskContent)
	if err == nil {
		t.Error("期望返回 error, 但未返回")
	}
}

// TestEstimateTaskMinutes_InvalidMinutes task_ancient_minutes 为负数或 >100000
func TestEstimateTaskMinutes_InvalidMinutes(t *testing.T) {
	tests := []struct {
		name    string
		minutes float64
	}{
		{"negative", -1.0},
		{"too_large", 100001.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
				respObj := map[string]interface{}{
					"content": []map[string]interface{}{
						{"text": `{"task_ancient_minutes":` + strings.Replace(strings.Replace(
							func() string { b, _ := json.Marshal(tt.minutes); return string(b) }(),
							"\n", "", -1), " ", "", -1) + `,"task_ancient_minutes_reason":"test"}`},
					},
				}
				json.NewEncoder(w).Encode(respObj)
			}))
			defer server.Close()

			config := AIEstimationConfig{
				BaseURL:   server.URL,
				APIKey:    "test-key",
				Model:     "test-model",
				TimeoutMS: 5000,
			}
			taskContent := &TaskContentFile{
				Conversations: []ConversationEntry{
					{Timestamp: "2026-03-31T09:00:00Z", UserInput: "hello"},
				},
			}

			_, _, err := EstimateTaskMinutes(config, taskContent)
			if err == nil {
				t.Errorf("minutes=%.1f 时期望返回 error, 但未返回", tt.minutes)
			}
		})
	}
}

// TestUpdateTaskContentWithEstimation 测试回写功能
func TestUpdateTaskContentWithEstimation(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "task_content.json")

	content := &TaskContentFile{
		TaskID:   "task-001",
		UserName: "test-user",
	}

	err := UpdateTaskContentWithEstimation(content, 210.0, "medium complexity", filePath)
	if err != nil {
		t.Fatalf("UpdateTaskContentWithEstimation 返回错误: %v", err)
	}

	// 验证文件已写入
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("读取文件失败: %v", err)
	}

	var result TaskContentFile
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("JSON 解析失败: %v", err)
	}
	if result.TaskAncientMinutes != 210.0 {
		t.Errorf("TaskAncientMinutes: want 210.0, got %f", result.TaskAncientMinutes)
	}
	if result.TaskAncientMinutesReason != "medium complexity" {
		t.Errorf("TaskAncientMinutesReason: want 'medium complexity', got %s", result.TaskAncientMinutesReason)
	}
	if result.TaskID != "task-001" {
		t.Errorf("TaskID: want task-001, got %s", result.TaskID)
	}
}
