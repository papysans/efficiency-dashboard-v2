package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type codeOutput struct {
	Path string `json:"path"`
	Code string `json:"code"`
}

type taskConvContent struct {
	Timestamp   time.Time    `json:"timestamp"`
	UserInput   string       `json:"user_input"`
	CodeOutputs []codeOutput `json:"code_outputs"`
}

type taskContent struct {
	TaskAncientMinutes float64           `json:"task_ancient_minutes"`
	TaskAncientReason  string            `json:"task_ancient_minutes_reason"`
	Conversations      []taskConvContent `json:"conversations"`
	TotalUserInChars   int               `json:"total_user_inchars"`
	TotalCodeLines     int               `json:"total_code_lines"`
}

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
func EstimateTaskMinutes(config AIEstimationConfig, taskContent *taskContent) (float64, string, error) {
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

	messages := []chatMessage{
		{Role: "system", Content: "请回答问题"},
		{Role: "user", Content: prompt},
	}
	content, err := callLLM(config, messages, 1024)
	if err != nil {
		return 0, "", err
	}

	// 从响应文本中提取 JSON 对象
	// 模型可能返回：纯 JSON / markdown 代码块包裹的 JSON / 中文分析 + JSON 混合格式
	jsonText := extractJSON(content)

	// 解析估时结果
	var result struct {
		TaskAncientMinutes float64 `json:"task_ancient_minutes"`
		TaskAncientReason  string  `json:"task_ancient_minutes_reason"`
	}
	if err := json.Unmarshal([]byte(jsonText), &result); err != nil {
		return 0, "", fmt.Errorf("解析AI估时结果JSON失败: %w, 原始文本: %s", err, content)
	}

	if result.TaskAncientMinutes < 0 || result.TaskAncientMinutes > 100000 {
		return 0, "", fmt.Errorf("AI估时结果异常: %.2f（应在0-100000之间）", result.TaskAncientMinutes)
	}

	return result.TaskAncientMinutes, result.TaskAncientReason, nil
}
