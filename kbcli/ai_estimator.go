package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

// extractJSON 从 AI 响应文本中提取 JSON 对象
// 支持三种格式：纯 JSON / markdown 代码块 / 中文分析+JSON 混合
func extractJSON(text string) string {
	text = strings.TrimSpace(text)

	// 1. 直接尝试解析
	if json.Valid([]byte(text)) {
		return text
	}

	// 2. 尝试提取 markdown 代码块内的 JSON
	re := regexp.MustCompile("(?s)```(?:json)?\\s*\\n?(.*?)\\n?```")
	if matches := re.FindStringSubmatch(text); len(matches) > 1 {
		candidate := strings.TrimSpace(matches[1])
		if json.Valid([]byte(candidate)) {
			return candidate
		}
	}

	// 3. 查找文本中最后一个完整的 {...} JSON 对象（模型可能先输出分析再输出 JSON）
	lastBrace := strings.LastIndex(text, "}")
	if lastBrace >= 0 {
		// 从最后的 } 往前找匹配的 {
		depth := 0
		for i := lastBrace; i >= 0; i-- {
			if text[i] == '}' {
				depth++
			} else if text[i] == '{' {
				depth--
				if depth == 0 {
					candidate := strings.TrimSpace(text[i : lastBrace+1])
					if json.Valid([]byte(candidate)) {
						return candidate
					}
				}
			}
		}
	}

	return text
}

const defaultEstimationPrompt = `你是一个经验丰富的软件项目经理，擅长评估软件开发工作量。

请根据以下用户与 AI 的对话记录，分析用户的需求复杂度，并估算实现该需求所需的**分钟数**。

重点关注：
1. 用户需求的复杂程度
2. 涉及的功能模块数量
3. 技术难度（如是否需要处理并发、安全、性能等问题）
4. 代码量规模

用户输入记录（按时间顺序）：
{{user_inputs}}

AI 生成的代码：
{{code_outputs}}

总输入字符数：{{total_chars}}
总代码行数：{{total_lines}}

请输出 JSON 格式：
{
  "task_ancient_minutes": 270,
  "task_ancient_minutes_reason": "估算理由..."
}`

// EstimateTaskMinutes 调用 AI 接口估算任务工作量（分钟）
func EstimateTaskMinutes(config AIEstimationConfig, taskContent *TaskContentFile) (float64, string, error) {
	// 构建占位符内容
	var userInputs strings.Builder
	var codeOutputs strings.Builder
	for _, conv := range taskContent.Conversations {
		fmt.Fprintf(&userInputs, "[%s] %s\n", conv.Timestamp, conv.UserInput)
		for _, co := range conv.CodeOutputs {
			fmt.Fprintf(&codeOutputs, "%s:\n%s\n", co.Path, co.Code)
		}
	}

	promptTemplate := config.Prompt
	if promptTemplate == "" {
		promptTemplate = defaultEstimationPrompt
	}

	prompt := promptTemplate
	prompt = strings.ReplaceAll(prompt, "{{user_inputs}}", userInputs.String())
	prompt = strings.ReplaceAll(prompt, "{{code_outputs}}", codeOutputs.String())
	prompt = strings.ReplaceAll(prompt, "{{total_chars}}", fmt.Sprintf("%d", taskContent.TotalUserInChars))
	prompt = strings.ReplaceAll(prompt, "{{total_lines}}", fmt.Sprintf("%d", taskContent.TotalCodeLines))

	// 构建 HTTP 请求（Anthropic Messages API）
	reqBody := map[string]interface{}{
		"model":      config.Model,
		"max_tokens": 1024,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return 0, "", fmt.Errorf("序列化请求体失败: %w", err)
	}

	url := config.BaseURL + "/v1/messages"
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return 0, "", fmt.Errorf("创建HTTP请求失败: %w", err)
	}
	httpReq.Header.Set("x-api-key", config.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: time.Duration(config.TimeoutMS) * time.Millisecond,
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return 0, "", fmt.Errorf("AI API 请求失败（可能超时）: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, "", fmt.Errorf("读取AI响应失败: %w", err)
	}

	if resp.StatusCode != 200 {
		return 0, "", fmt.Errorf("AI API 返回非200状态码: %d, 响应: %s", resp.StatusCode, string(respBody))
	}

	// 解析 Anthropic 响应
	var anthropicResp struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBody, &anthropicResp); err != nil {
		return 0, "", fmt.Errorf("解析AI响应JSON失败: %w, 原始响应: %s", err, string(respBody))
	}
	if len(anthropicResp.Content) == 0 {
		return 0, "", fmt.Errorf("AI响应content为空, 原始响应: %s", string(respBody))
	}

	text := anthropicResp.Content[0].Text

	// 从响应文本中提取 JSON 对象
	// 模型可能返回：纯 JSON / markdown 代码块包裹的 JSON / 中文分析 + JSON 混合格式
	jsonText := extractJSON(text)

	// 解析估时结果
	var result struct {
		TaskAncientMinutes       float64 `json:"task_ancient_minutes"`
		TaskAncientMinutesReason string  `json:"task_ancient_minutes_reason"`
	}
	if err := json.Unmarshal([]byte(jsonText), &result); err != nil {
		return 0, "", fmt.Errorf("解析AI估时结果JSON失败: %w, 原始文本: %s", err, text)
	}

	if result.TaskAncientMinutes < 0 || result.TaskAncientMinutes > 100000 {
		return 0, "", fmt.Errorf("AI估时结果异常: %.2f（应在0-100000之间）", result.TaskAncientMinutes)
	}

	return result.TaskAncientMinutes, result.TaskAncientMinutesReason, nil
}

// UpdateTaskContentWithEstimation 将估时结果回写到 TaskContentFile 并保存
func UpdateTaskContentWithEstimation(content *TaskContentFile, minutes float64, reason string, filePath string) error {
	content.TaskAncientMinutes = minutes
	content.TaskAncientMinutesReason = reason

	data, err := json.MarshalIndent(content, "", "  ")
	if err != nil {
		return fmt.Errorf("JSON序列化失败: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	return nil
}
