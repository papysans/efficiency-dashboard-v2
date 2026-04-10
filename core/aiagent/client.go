package aiagent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	defaultBaseURL    = "https://api.open.bigmodel.cn/api/paas/v4"
	defaultModel      = "glm-4"
	defaultMaxTokens  = 4000
	defaultTimeout    = 120 * time.Second
	defaultMaxRetries = 3
)

// Message 对话消息
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest 请求结构
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
}

// Choice 响应选项
type Choice struct {
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// ChatResponse 响应结构
type ChatResponse struct {
	ID      string   `json:"id"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
}

// APIError API错误结构
type APIError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// GLMClient GLM API 客户端
type GLMClient struct {
	BaseURL    string
	APIKey     string
	Model      string
	MaxTokens  int
	Timeout    time.Duration
	MaxRetries int
	httpClient *http.Client
}

// NewGLMClient 创建 GLM 客户端
// apiKey: API密钥（必填）
// baseURL: API基础URL（空串则使用默认值）
// model: 模型名称（空串则使用默认值）
func NewGLMClient(apiKey, baseURL, model string) *GLMClient {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	if model == "" {
		model = defaultModel
	}
	c := &GLMClient{
		BaseURL:    baseURL,
		APIKey:     apiKey,
		Model:      model,
		MaxTokens:  defaultMaxTokens,
		Timeout:    defaultTimeout,
		MaxRetries: defaultMaxRetries,
	}
	c.httpClient = &http.Client{Timeout: c.Timeout}
	return c
}

// Chat 发送对话请求，返回模型回复内容
func (c *GLMClient) Chat(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	reqBody := ChatRequest{
		Model:       c.Model,
		Messages:    messages,
		MaxTokens:   c.MaxTokens,
		Temperature: 0.7,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt < c.MaxRetries; attempt++ {
		if attempt > 0 {
			sleepDur := time.Duration(1<<uint(attempt-1)) * time.Second
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(sleepDur):
			}
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		result, err := c.doRequest(ctx, jsonData)
		if err != nil {
			lastErr = err
			// 判断是否可重试
			if isRetryable(err) {
				continue
			}
			return "", err
		}
		return result, nil
	}

	return "", fmt.Errorf("重试 %d 次后仍失败: %w", c.MaxRetries, lastErr)
}

// doRequest 执行单次HTTP请求
func (c *GLMClient) doRequest(ctx context.Context, jsonData []byte) (string, error) {
	url := c.BaseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode >= 500 {
		return "", fmt.Errorf("服务器错误 %d: %s", resp.StatusCode, string(body))
	}

	if resp.StatusCode != 200 {
		var apiErr APIError
		if json.Unmarshal(body, &apiErr) == nil && apiErr.Error.Message != "" {
			return "", fmt.Errorf("API错误 %d: %s", resp.StatusCode, apiErr.Error.Message)
		}
		return "", fmt.Errorf("HTTP错误 %d: %s", resp.StatusCode, string(body))
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("响应中没有选项")
	}

	return chatResp.Choices[0].Message.Content, nil
}

// isRetryable 判断错误是否可重试
func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return contains(msg, "timeout") ||
		contains(msg, "connection refused") ||
		contains(msg, "EOF") ||
		contains(msg, "服务器错误 5")
}

// NewClientFromEnv 从环境变量读取配置创建 GLM 客户端
// 优先读取 OPENAI_API_KEY，兼容 GLM_API_KEY
// 优先读取 OPENAI_BASE_URL，兼容 GLM_BASE_URL
// model 参数为空时读取 OPENAI_MODEL/GLM_MODEL 环境变量
func NewClientFromEnv(model string) (*GLMClient, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("GLM_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("AI API key not set: set OPENAI_API_KEY or GLM_API_KEY")
	}
	baseURL := os.Getenv("OPENAI_BASE_URL")
	if baseURL == "" {
		baseURL = os.Getenv("GLM_BASE_URL")
	}
	if model == "" {
		model = os.Getenv("OPENAI_MODEL")
		if model == "" {
			model = os.Getenv("GLM_MODEL")
		}
	}
	return NewGLMClient(apiKey, baseURL, model), nil
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
