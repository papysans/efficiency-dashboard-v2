package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"kanban/kbcli/internal/logx"
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

	logx.Infof("发送远程请求: POST %s", url)
	logx.Debugf("请求体: %s", string(body))

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
		TaskId string `json:"task_id"`
		Status string `json:"status"`
		Type   string `json:"type"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		logx.Infof("远程响应: %s", string(respBody))
		return nil
	}

	logx.Infof("远程任务已提交: task_id=%s, status=%s, type=%s", result.TaskId, result.Status, result.Type)
	logx.Infof("可通过 curl %s/api/tasks/%s 查询任务状态", remoteURL, result.TaskId)
	return nil
}
