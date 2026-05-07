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

func sendToRemote(remoteURL, taskType string, params map[string]interface{}) error {
	remoteURL = strings.TrimRight(remoteURL, "/")
	url := fmt.Sprintf("%s/api/tasks/%s", remoteURL, taskType)

	body, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("序列化请求参数失败: %w", err)
	}

	logInfof("发送远程请求: POST %s", url)
	logDebugf("请求体: %s", string(body))

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("发送远程请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取远程响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("远程服务器返回错误 (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		TaskID string `json:"task_id"`
		Status string `json:"status"`
		Type   string `json:"type"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		logInfof("远程响应: %s", string(respBody))
		return nil
	}

	logInfof("远程任务已提交: task_id=%s, status=%s, type=%s", result.TaskID, result.Status, result.Type)
	logInfof("可通过 GET %s/api/tasks/%s 查询任务状态", remoteURL, result.TaskID)
	return nil
}
