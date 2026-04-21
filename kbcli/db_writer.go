package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// BackendClient 封装 backend API 调用
type BackendClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewBackendClient 创建 backend API 客户端
func NewBackendClient(baseURL string) *BackendClient {
	return &BackendClient{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// // SaveGitAnalysis 通过 POST /api/analysis/git/analyze 保存 git 分析结果到 PG
// func (c *BackendClient) SaveGitAnalysis(repoID, startDate, endDate string, result *GitAnalysisResult, aiEstDays *float64) error {
// 	body := map[string]interface{}{
// 		"repo_id":    repoID,
// 		"start_date": startDate,
// 		"end_date":   endDate,
// 	}
// 	if result != nil {
// 		body["commit_count"] = result.CommitCount
// 		body["contributor_count"] = result.ContributorCount
// 		body["lines_added"] = result.LinesAdded
// 		body["lines_deleted"] = result.LinesDeleted
// 		body["files_changed"] = result.FilesChanged
// 	}
// 	if aiEstDays != nil {
// 		body["commit_ancient_minutes_from_git"] = *aiEstDays
// 	}

// 	jsonData, err := json.Marshal(body)
// 	if err != nil {
// 		return fmt.Errorf("序列化请求体失败: %w", err)
// 	}

// 	resp, err := c.HTTPClient.Post(c.BaseURL+"/api/analysis/git/analyze", "application/json", bytes.NewReader(jsonData))
// 	if err != nil {
// 		return fmt.Errorf("调用 backend API 失败: %w", err)
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode != 200 {
// 		respBody, _ := io.ReadAll(resp.Body)
// 		return fmt.Errorf("backend API 返回错误: %d, %s", resp.StatusCode, string(respBody))
// 	}
// 	return nil
// }

// CalculateEfficiency 调用后端 API 触发提效分析计算
func (c *BackendClient) CalculateEfficiency(dimension, id, startDate, endDate string, force bool) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"dimension":  dimension,
		"id":         id,
		"start_date": startDate,
		"end_date":   endDate,
		"force":      force,
	}
	jsonData, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("序列化请求体失败: %w", err)
	}

	resp, err := c.HTTPClient.Post(c.BaseURL+"/api/analysis/efficiency/calculate", "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("调用 backend API 失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("backend API 返回错误: %d, %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	return result, nil
}

// CorrectEfficiency 调用后端 API 执行纠错
func (c *BackendClient) CorrectEfficiency(dimension, id, field string, value float64, reason, by string) error {
	body := map[string]interface{}{
		"dimension": dimension,
		"id":        id,
		"field":     field,
		"value":     value,
		"reason":    reason,
		"by":        by,
	}
	jsonData, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("序列化请求体失败: %w", err)
	}

	req, err := http.NewRequest("PUT", c.BaseURL+"/api/analysis/efficiency/correct", bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("调用 backend API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("backend API 返回错误: %d, %s", resp.StatusCode, string(respBody))
	}
	return nil
}
