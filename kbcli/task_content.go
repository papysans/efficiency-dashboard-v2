package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// TaskContentFile Task 内容文件结构
type TaskContentFile struct {
	TaskID            string              `json:"task_id"`
	UserID            string              `json:"user_id"`
	UserName          string              `json:"user_name"`
	ProjectID         string              `json:"project_id"`
	StartTime         string              `json:"start_time"`
	EndTime           string              `json:"end_time"`
	TotalUserInChars  int64               `json:"total_user_in_chars"`
	TotalCodeLines    int64               `json:"total_code_lines"`
	Conversations     []ConversationEntry `json:"conversations"`
	TaskAncientMinutes       float64             `json:"task_ancient_minutes,omitempty"`
	TaskAncientMinutesReason string              `json:"task_ancient_minutes_reason,omitempty"`
}

// ConversationEntry 对话条目
type ConversationEntry struct {
	Timestamp   string       `json:"timestamp"`
	RequestID   string       `json:"request_id"`
	UserInput   string       `json:"user_input"`
	CodeOutputs []CodeOutput `json:"code_outputs"`
}

// CodeOutput 代码输出
type CodeOutput struct {
	Path string `json:"path"`
	Code string `json:"code"`
}

// ExtractTaskContent 从 RawDoc 列表提取 Task 内容文件
func ExtractTaskContent(taskID string, rawDocs []RawDoc, rawDataDir string) (*TaskContentFile, error) {
	if len(rawDocs) == 0 {
		return nil, fmt.Errorf("rawDocs 为空")
	}

	// 按 APIRequestTime 升序排序
	sort.Slice(rawDocs, func(i, j int) bool {
		return rawDocs[i].APIRequestTime.Before(rawDocs[j].APIRequestTime)
	})

	first := rawDocs[0]
	last := rawDocs[len(rawDocs)-1]

	tcf := &TaskContentFile{
		TaskID:    taskID,
		UserID:    first.UserID,
		UserName:  first.UserName,
		ProjectID: first.ProjectID,
		StartTime: first.APIRequestTime.Format(time.RFC3339),
		EndTime:   last.APIEndTime.Format(time.RFC3339),
	}

	for _, doc := range rawDocs {
		tcf.TotalUserInChars += doc.UserInChars
		tcf.TotalCodeLines += doc.AssistantOutCodeLines

		// 读取原始 JSON 文件
		fullPath := filepath.Join(rawDataDir, doc.SourcePath)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			fmt.Printf("警告: 读取文件失败 %s: %v\n", fullPath, err)
			continue
		}

		var raw rawJSON
		if err := json.Unmarshal(data, &raw); err != nil {
			fmt.Printf("警告: 解析JSON失败 %s: %v\n", fullPath, err)
			continue
		}

		// 提取 user_input
		userInput := ""
		if msgs := raw.Params.LLMParams.Messages; len(msgs) > 0 {
			content := contentToString(msgs[len(msgs)-1].Content)
			if strings.HasPrefix(content, "<user_message>") {
				start := len("<user_message>")
				end := strings.Index(content, "</user_message>")
				if end < 0 {
					end = len(content)
				}
				userInput = content[start:end]
			} else {
				userInput = content
			}
		}

		// 提取 code_outputs
		var codeOutputs []CodeOutput
		for _, tc := range raw.ResponseContent.ToolCalls {
			if tc.Function.Name != "write_to_file" && tc.Function.Name != "apply_diff" {
				continue
			}
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				continue
			}
			path, _ := args["path"].(string)
			var code string
			if tc.Function.Name == "apply_diff" {
				diff, _ := args["diff"].(string)
				code = extractDiffReplaceContent(diff)
			} else {
				code, _ = args["content"].(string)
			}
			if path != "" {
				codeOutputs = append(codeOutputs, CodeOutput{Path: path, Code: code})
			}
		}

		tcf.Conversations = append(tcf.Conversations, ConversationEntry{
			Timestamp:   doc.APIRequestTime.Format(time.RFC3339),
			RequestID:   doc.RequestID,
			UserInput:   userInput,
			CodeOutputs: codeOutputs,
		})
	}

	return tcf, nil
}

// SaveTaskContent 保存 Task 内容文件到 rawDataDir
func SaveTaskContent(content *TaskContentFile, rawDataDir string) (string, error) {
	t, err := time.Parse(time.RFC3339, content.StartTime)
	if err != nil {
		return "", fmt.Errorf("解析 StartTime 失败: %w", err)
	}

	yearMonth := t.Format("2006-01")
	day := t.Format("02")
	fileName := fmt.Sprintf("task_%s.json", content.TaskID)
	relativePath := filepath.Join(yearMonth, day, fileName)
	fullPath := filepath.Join(rawDataDir, relativePath)

	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return "", fmt.Errorf("创建目录失败: %w", err)
	}

	data, err := json.MarshalIndent(content, "", "  ")
	if err != nil {
		return "", fmt.Errorf("JSON序列化失败: %w", err)
	}

	if err := os.WriteFile(fullPath, data, 0644); err != nil {
		return "", fmt.Errorf("写入文件失败: %w", err)
	}

	return relativePath, nil
}
