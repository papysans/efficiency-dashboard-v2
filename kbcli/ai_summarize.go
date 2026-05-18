package main

import (
	"encoding/json"
	"fmt"
	"kanban/core/models"
	"log"
	"strings"
	"time"

	"gorm.io/gorm"
)

// callAIForTaskTitle 调用 AI 从对话记录提取任务标题（不超过100字符）
func callAIForTaskTitle(db *gorm.DB, taskID string, userInputs []string) (string, error) {
	aiCfg := cfg.AIEstimation
	if !aiCfg.Enabled || aiCfg.APIKey == "" {
		return "", fmt.Errorf("AI estimation not enabled or API key missing")
	}

	prompt := fmt.Sprintf(`请根据以下用户与AI助手的对话记录，用一句简短的中文描述这个任务的目标，不超过100个字符。
只输出标题文本，不要任何额外格式或引号。

用户输入记录：
%s`, truncateSlice(userInputs, 3000))

	messages := []chatMessage{
		{Role: "system", Content: "请回答问题"},
		{Role: "user", Content: prompt},
	}
	content, err := callLLM(aiCfg, messages, 256)
	if err != nil {
		return "", err
	}

	title := strings.TrimSpace(content)
	title = strings.Trim(title, "\"'`")
	runes := []rune(title)
	if len(runes) > 100 {
		title = string(runes[:100])
	}
	if title == "" {
		return "", fmt.Errorf("AI返回空标题")
	}

	return title, nil
}

func UpdateTaskTitle(db *gorm.DB, taskID string, title string) error {
	result := db.Model(&models.Task{}).Where("task_id = ?", taskID).
		Updates(map[string]interface{}{"title": title, "updated_at": time.Now()})
	if result.Error != nil {
		log.Printf("回写title失败: %v", result.Error)
		return result.Error
	}
	return nil
}

// callAIForAncientEstimation 调用 AI 估算传统开发时长
func callAIForAncientEstimation(userInputs []string, codeOutputs []string, totalChars int64, totalLines int64) (float64, string, error) {
	aiCfg := cfg.AIEstimation
	prompt := fmt.Sprintf(`你是一个经验丰富的软件项目经理，擅长评估软件开发工作量。

请根据以下用户与 AI 的对话记录，分析用户的需求复杂度，并估算如果由传统人工开发（不使用AI），实现该需求所需的**分钟数**。

重点关注：
1. 用户需求的复杂程度
2. 涉及的功能模块数量
3. 技术难度（如是否需要处理并发、安全、性能等问题）
4. 代码量规模

用户输入记录（按时间顺序）：
%s

AI 生成的代码片段：
%s

总输入字符数：%d
总代码行数：%d

请输出 JSON 格式：
{
  "task_ancient_minutes": 270,
  "task_ancient_minutes_reason": "估算理由..."
}`,
		truncateSlice(userInputs, 5000),
		truncateSlice(codeOutputs, 8000),
		totalChars,
		totalLines,
	)

	messages := []chatMessage{
		{Role: "system", Content: "请回答问题"},
		{Role: "user", Content: prompt},
	}
	content, err := callLLM(aiCfg, messages, 1024)
	if err != nil {
		return 0, "", err
	}

	jsonText := extractJSON(content)
	var result struct {
		Minutes float64 `json:"task_ancient_minutes"`
		Reason  string  `json:"task_ancient_minutes_reason"`
	}
	if err := json.Unmarshal([]byte(jsonText), &result); err != nil {
		return 0, "", fmt.Errorf("解析估时结果失败: %w, text: %s", err, content)
	}

	if result.Minutes < 0 || result.Minutes > 100000 {
		return 0, "", fmt.Errorf("估时结果异常: %.2f", result.Minutes)
	}

	return result.Minutes, result.Reason, nil
}

// truncateSlice 将字符串切片拼接后截断到 maxLen 字符
func truncateSlice(items []string, maxLen int) string {
	var sb strings.Builder
	for i, s := range items {
		if sb.Len()+len(s) > maxLen {
			remaining := maxLen - sb.Len()
			if remaining > 0 {
				sb.WriteString(s[:remaining])
				sb.WriteString("...(截断)")
			}
			break
		}
		if i > 0 {
			sb.WriteString("\n---\n")
		}
		sb.WriteString(s)
	}
	return sb.String()
}

// EstimateAncientResult AI 估时单条结果
type EstimateAncientResult struct {
	TaskId  string  `json:"task_id"`
	Minutes float64 `json:"minutes"`
	Reason  string  `json:"reason"`
	Error   string  `json:"error,omitempty"`
}

// EstimateAncientResponse AI 估时批量结果
type EstimateAncientResponse struct {
	Status  string                  `json:"status"`
	Total   int                     `json:"total"`
	Success int                     `json:"success"`
	Results []EstimateAncientResult `json:"results"`
}

// RunAncientMinutesEstimation 批量执行 AI 估时，对应原后端 HTTP handler 的核心逻辑
// specificTaskID 为空时，自动取最近 50 个未估算任务
func RunAncientMinutesEstimation(db *gorm.DB, aiCfg AIEstimationConfig, specificTaskID string) *EstimateAncientResponse {
	if !aiCfg.Enabled || aiCfg.APIKey == "" {
		return &EstimateAncientResponse{Status: "error", Total: 0, Success: 0, Results: []EstimateAncientResult{
			{TaskId: "", Error: "AI estimation not enabled or API key missing"},
		}}
	}

	taskIDs, err := GetTaskIDsForEstimation(db, specificTaskID)
	if err != nil {
		return &EstimateAncientResponse{Status: "error", Total: 0, Success: 0, Results: []EstimateAncientResult{
			{TaskId: "", Error: err.Error()},
		}}
	}

	if len(taskIDs) == 0 {
		return &EstimateAncientResponse{Status: "ok", Total: 0, Success: 0}
	}

	var results []EstimateAncientResult

	for _, tid := range taskIDs {
		userInputs, codeOutputs, totalChars, totalLines, err := GetConvInputForEstimation(db, tid)
		if err != nil {
			results = append(results, EstimateAncientResult{TaskId: tid, Error: err.Error()})
			continue
		}

		if len(userInputs) == 0 {
			results = append(results, EstimateAncientResult{TaskId: tid, Error: "no conversation data"})
			continue
		}

		minutes, reason, err := callAIForAncientEstimation(userInputs, codeOutputs, totalChars, totalLines)
		if err != nil {
			results = append(results, EstimateAncientResult{TaskId: tid, Error: err.Error()})
			continue
		}

		if err := UpdateTaskAncientEstimation(db, tid, minutes, reason); err != nil {
			results = append(results, EstimateAncientResult{TaskId: tid, Minutes: minutes, Reason: reason, Error: "db update failed: " + err.Error()})
			continue
		}

		results = append(results, EstimateAncientResult{TaskId: tid, Minutes: minutes, Reason: reason})
		logInfof("AI估时完成: task=%s, minutes=%.1f", tid, minutes)
	}

	successCount := 0
	for _, r := range results {
		if r.Error == "" {
			successCount++
		}
	}

	return &EstimateAncientResponse{Status: "ok", Total: len(taskIDs), Success: successCount, Results: results}
}
