package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// callLLM 统一调用大模型入口，根据配置选择 anthropic 或 openai 格式，非流式
func callLLM(config AIEstimationConfig, messages []chatMessage, maxTokens int) (string, error) {
	switch config.APIFormat {
	case "anthropic":
		return callAnthropicChat(config, messages, maxTokens)
	default:
		return callOpenAIChat(config, messages, maxTokens)
	}
}

// callOpenAIChat 调用 OpenAI 兼容格式 API（chat.completions）
func callOpenAIChat(config AIEstimationConfig, messages []chatMessage, maxTokens int) (string, error) {
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
func callAnthropicChat(config AIEstimationConfig, messages []chatMessage, maxTokens int) (string, error) {
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
