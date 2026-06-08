// Package llm 是统一的大模型调用客户端（从原 kbcli package main 的 call_llm.go 抽出，
// 供 AI 估时(ai_estimator/ai_summarize) 与 efficiency-v2 LLM 基线共用）。
package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// AIEstimationConfig AI 估时 / LLM 调用配置（从 config.go 迁入；main Config.AIEstimation 引用本类型）。
type AIEstimationConfig struct {
	Enabled      bool              `yaml:"enabled"`
	APIKey       string            `yaml:"api_key"`
	XApiKey      string            `yaml:"x_api_key"`
	BaseURL      string            `yaml:"base_url"`
	Model        string            `yaml:"model"`
	TimeoutMS    int               `yaml:"timeout_ms"`
	HTTPProxy    string            `yaml:"http_proxy"`
	Prompt       string            `yaml:"prompt"`
	APIFormat    string            `yaml:"api_format"`
	ExtraHeaders map[string]string `yaml:"extra_headers"`
}

// ChatMessage 一条对话消息（role/content）。
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// CallLLM 统一调用大模型入口，根据配置选择 anthropic 或 openai 格式，非流式。
func CallLLM(config AIEstimationConfig, messages []ChatMessage, maxTokens int) (string, error) {
	switch config.APIFormat {
	case "anthropic":
		return callAnthropicChat(config, messages, maxTokens)
	default:
		return callOpenAIChat(config, messages, maxTokens)
	}
}

// callOpenAIChat 调用 OpenAI 兼容格式 API（chat.completions）
func callOpenAIChat(config AIEstimationConfig, messages []ChatMessage, maxTokens int) (string, error) {
	reqBody := map[string]interface{}{
		"model":       config.Model,
		"messages":    messages,
		"temperature": 0,
		"max_tokens":  maxTokens,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("序列化请求体失败: %w", err)
	}

	url := strings.TrimRight(config.BaseURL, "/") + "/v1/chat/completions"
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+config.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	if config.XApiKey != "" {
		httpReq.Header.Set("x-apikey", config.XApiKey)
	}
	for k, v := range config.ExtraHeaders {
		httpReq.Header.Set(k, v)
	}

	client := &http.Client{Timeout: time.Duration(config.TimeoutMS) * time.Millisecond}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("AI API 请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取AI响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("AI API 返回 %d: %s", resp.StatusCode, string(respBody))
	}

	var openAIResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &openAIResp); err != nil {
		return "", fmt.Errorf("解析AI响应失败: %w, 原始响应: %s", err, string(respBody))
	}
	if len(openAIResp.Choices) == 0 {
		return "", fmt.Errorf("AI响应choices为空, 原始响应: %s", string(respBody))
	}

	return strings.TrimSpace(openAIResp.Choices[0].Message.Content), nil
}

// callAnthropicChat 调用 Anthropic Messages API
func callAnthropicChat(config AIEstimationConfig, messages []ChatMessage, maxTokens int) (string, error) {
	var userContent string
	for _, m := range messages {
		if m.Role == "system" {
			userContent = m.Content + "\n\n" + userContent
		} else {
			if userContent != "" {
				userContent += "\n\n"
			}
			userContent += m.Content
		}
	}

	reqBody := map[string]interface{}{
		"model":      config.Model,
		"max_tokens": maxTokens,
		"messages": []map[string]string{
			{"role": "user", "content": userContent},
		},
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("序列化请求体失败: %w", err)
	}

	url := strings.TrimRight(config.BaseURL, "/") + "/v1/messages"
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	httpReq.Header.Set("x-api-key", config.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("Content-Type", "application/json")
	for k, v := range config.ExtraHeaders {
		httpReq.Header.Set(k, v)
	}

	client := &http.Client{Timeout: time.Duration(config.TimeoutMS) * time.Millisecond}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("AI API 请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取AI响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("AI API 返回 %d: %s", resp.StatusCode, string(respBody))
	}

	var anthropicResp struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBody, &anthropicResp); err != nil {
		return "", fmt.Errorf("解析AI响应失败: %w, 原始响应: %s", err, string(respBody))
	}
	if len(anthropicResp.Content) == 0 {
		return "", fmt.Errorf("AI响应content为空, 原始响应: %s", string(respBody))
	}

	return strings.TrimSpace(anthropicResp.Content[0].Text), nil
}
