package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"strings"
	"time"
)

// aiEfficiencyResult AI综合评估结果
type aiEfficiencyResult struct {
	EfficiencyRatio  float64 `json:"efficiency_ratio"`
	EfficiencyReason string  `json:"efficiency_reason"`
}

// callAIForEfficiencyAssessment 调用AI综合评估提效比例
func callAIForEfficiencyAssessment(aiEstDays float64, processTimeMs, leadTimeMs int64, totalCodeLines int64, taskCount int, apiCallCount int, reasons []string) (*aiEfficiencyResult, error) {
	cfg := appConfig.AIEstimation
	if !cfg.Enabled || cfg.APIKey == "" {
		return nil, fmt.Errorf("AI estimation not enabled or API key missing")
	}

	processTimeDays := float64(processTimeMs) / float64(MsPerWorkDay)
	leadTimeDays := float64(leadTimeMs) / float64(MsPerWorkDay)

	reasonSummary := "无"
	if len(reasons) > 0 {
		maxReasons := reasons
		if len(maxReasons) > 10 {
			maxReasons = maxReasons[:10]
		}
		reasonSummary = strings.Join(maxReasons, "\n- ")
	}

	prompt := fmt.Sprintf(`你是一位资深的软件工程效率分析师。请根据以下数据，综合评估 AI 编码助手的提效倍率。

## 数据概览
- AI预估（如果由人工完成）需要: %.1f 人天
- 实际使用AI后的处理时间(process_time): %.2f 人天 (%.0f 分钟)
- 实际使用AI后的前置时间(lead_time): %.2f 人天 (%.0f 分钟)
- 生成的总代码行数: %d 行
- 总任务数(task): %d 个
- 总API调用次数: %d 次

## AI预估理由摘要
- %s

## 评估要求
1. 提效倍率 = AI预估人天 / 实际使用AI所耗人天（基于process_time）
2. 但简单的除法可能不合理（比如用户只发了几个请求，process_time很短但任务未必完成），你需要综合判断
3. 考虑因素：代码行数是否合理、任务复杂度、API调用频次等
4. 提效倍率通常在 1x-50x 之间，超出此范围要特别谨慎并说明理由
5. 如果数据不足以判断（如process_time过短、代码量极少），请给出保守估计

请输出JSON格式：
{
  "efficiency_ratio": 3.5,
  "efficiency_reason": "综合分析理由..."
}`,
		aiEstDays,
		processTimeDays, float64(processTimeMs)/60000.0,
		leadTimeDays, float64(leadTimeMs)/60000.0,
		totalCodeLines,
		taskCount,
		apiCallCount,
		reasonSummary,
	)

	reqBody := map[string]interface{}{
		"model":      cfg.Model,
		"max_tokens": AIMaxTokens,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("序列化请求体失败: %w", err)
	}

	url := strings.TrimRight(cfg.BaseURL, "/") + "/v1/messages"
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("创建HTTP请求失败: %w", err)
	}
	httpReq.Header.Set("x-api-key", cfg.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("Content-Type", "application/json")

	transport := &http.Transport{}
	if cfg.HTTPProxy != "" {
		proxyURL, err := neturl.Parse(cfg.HTTPProxy)
		if err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}
	client := &http.Client{
		Timeout:   time.Duration(cfg.TimeoutMS) * time.Millisecond,
		Transport: transport,
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("AI API请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取AI响应失败: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("AI API返回非200: %d, %s", resp.StatusCode, string(respBody))
	}

	var anthropicResp struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBody, &anthropicResp); err != nil {
		return nil, fmt.Errorf("解析AI响应失败: %w, resp body: %s", err, string(respBody))
	}
	if len(anthropicResp.Content) == 0 {
		return nil, fmt.Errorf("AI响应content为空")
	}

	jsonText := extractJSON(anthropicResp.Content[0].Text)
	var result aiEfficiencyResult
	if err := json.Unmarshal([]byte(jsonText), &result); err != nil {
		return nil, fmt.Errorf("解析AI效率评估结果失败: %w", err)
	}

	return &result, nil
}
