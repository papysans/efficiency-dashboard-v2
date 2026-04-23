package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestImportTasksCostCalculation 测试import-tasks命令的cost计算逻辑
func TestImportTasksCostCalculation(t *testing.T) {
	// 准备测试数据
	testDir := t.TempDir()

	// 创建测试用的conversation.jsonl文件
	conversationData := []map[string]interface{}{
		{
			"sender":            "user",
			"request_id":        "req-001",
			"prompt_mode":       "auto",
			"mode":              "chat",
			"model":             "GLM-4.7",
			"start_time":        "2025-01-15T10:00:00Z",
			"end_time":          "2025-01-15T10:01:00Z",
			"process_time":      60000,
			"process_ttft":      500,
			"upstream_tokens":   1000000, // 1M tokens
			"downstream_tokens": 500000,  // 0.5M tokens
			"cost":              0,       // cost为0，需要计算
			"request_content":   "test request",
			"response_content":  "test response",
			"user_input":        "hello",
			"diff":              "test diff",
			"diff_lines":        10,
		},
		{
			"sender":            "assistant",
			"request_id":        "req-002",
			"prompt_mode":       "manual",
			"mode":              "code",
			"model":             "GLM-5",
			"start_time":        "2025-01-15T10:01:00Z",
			"end_time":          "2025-01-15T10:02:00Z",
			"process_time":      60000,
			"process_ttft":      600,
			"upstream_tokens":   2000000, // 2M tokens
			"downstream_tokens": 1000000, // 1M tokens
			"cost":              0,       // cost为0，需要计算
			"request_content":   "test request 2",
			"response_content":  "test response 2",
			"user_input":        "help",
			"diff":              "test diff 2",
			"diff_lines":        20,
		},
		{
			"sender":            "user",
			"request_id":        "req-003",
			"prompt_mode":       "auto",
			"mode":              "chat",
			"model":             "GLM-4.7",
			"start_time":        "2025-01-15T10:02:00Z",
			"end_time":          "2025-01-15T10:03:00Z",
			"process_time":      60000,
			"process_ttft":      700,
			"upstream_tokens":   500000, // 0.5M tokens
			"downstream_tokens": 250000, // 0.25M tokens
			"cost":              0.5,    // cost已设置，应该使用这个值
			"request_content":   "test request 3",
			"response_content":  "test response 3",
			"user_input":        "thanks",
			"diff":              "test diff 3",
			"diff_lines":        5,
		},
	}

	conversationPath := filepath.Join(testDir, "conversation.jsonl")
	f, err := os.Create(conversationPath)
	assert.NoError(t, err)
	defer f.Close()

	for _, data := range conversationData {
		line, _ := json.Marshal(data)
		f.Write(append(line, '\n'))
	}

	// 准备modelPrices配置
	modelPrices := map[string]ModelPrice{
		"GLM-4.7": {
			InPrice:  0.5, // 0.5元/百万token
			OutPrice: 1.0, // 1.0元/百万token
		},
		"GLM-5": {
			InPrice:  1.0, // 1.0元/百万token
			OutPrice: 2.0, // 2.0元/百万token
		},
	}

	// 解析conversation文件并验证cost计算
	conversations, err := parseConversationFile(conversationPath, modelPrices)
	assert.NoError(t, err)
	assert.Len(t, conversations, 3)

	// 验证第一个conversation：GLM-4.7, 1M in, 0.5M out
	// cost = (1M/1M)*0.5 + (0.5M/1M)*1.0 = 0.5 + 0.5 = 1.0元
	assert.Equal(t, int64(1000000), conversations[0].UpstreamTokens)
	assert.Equal(t, int64(500000), conversations[0].DownstreamTokens)
	assert.InDelta(t, 1.0, conversations[0].Cost, 0.01)
	assert.InDelta(t, 1.0, conversations[0].calculatedCost, 0.01)

	// 验证第二个conversation：GLM-5, 2M in, 1M out
	// cost = (2M/1M)*1.0 + (1M/1M)*2.0 = 2.0 + 2.0 = 4.0元
	assert.Equal(t, int64(2000000), conversations[1].UpstreamTokens)
	assert.Equal(t, int64(1000000), conversations[1].DownstreamTokens)
	assert.InDelta(t, 4.0, conversations[1].Cost, 0.01)
	assert.InDelta(t, 4.0, conversations[1].calculatedCost, 0.01)

	// 验证第三个conversation：cost已设置为0.5，应该保持不变
	assert.Equal(t, int64(500000), conversations[2].UpstreamTokens)
	assert.Equal(t, int64(250000), conversations[2].DownstreamTokens)
	assert.InDelta(t, 0.5, conversations[2].Cost, 0.01)
	assert.InDelta(t, 0.5, conversations[2].calculatedCost, 0.01)

	// 验证总cost：1.0 + 4.0 + 0.5 = 5.5元
	totalCost := 0.0
	for _, conv := range conversations {
		totalCost += conv.Cost
	}
	assert.InDelta(t, 5.5, totalCost, 0.01)
}
