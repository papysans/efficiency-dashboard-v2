package main

import (
	"fmt"
	"strings"
	"time"
)

const (
	CodeSourceAICurrent = "ai_current"
	CodeSourceHuman     = "human"
	CodeSourceAIOther   = "ai_other"
	CodeSourceUnknown   = "unknown"
)

// TaskCommitMatch Task 与 Commit 的关联匹配
type TaskCommitMatch struct {
	TaskID      string  `json:"task_id"`
	CommitHash  string  `json:"commit_hash"`
	UserID      string  `json:"user_id"`
	MatchScore  float64 `json:"match_score"`
	MatchReason string  `json:"match_reason"`
}

// CommitClassification Commit 的代码来源分类
type CommitClassification struct {
	CommitHash     string   `json:"commit_hash"`
	CodeSource     string   `json:"code_source"`
	MatchedTaskIDs []string `json:"matched_task_ids"`
	UserID         string   `json:"user_id"`
	LinesAdded     int64    `json:"lines_added"`
	LinesDeleted   int64    `json:"lines_deleted"`
}

// MatchTasksToCommits 建立 Task-Commit 关联关系并分类代码来源
func MatchTasksToCommits(commits []CommitDetail, tasks []TaskContentFile, orgProvider *OrgProvider) ([]TaskCommitMatch, []CommitClassification) {
	var matches []TaskCommitMatch
	var classifications []CommitClassification

	// 按 user_id 索引 tasks
	userTasks := make(map[string][]TaskContentFile)
	for _, task := range tasks {
		userTasks[task.UserID] = append(userTasks[task.UserID], task)
	}

	// 记录每个 commit 关联的 task IDs
	commitTaskMap := make(map[string][]string)

	for _, commit := range commits {
		userID, found := orgProvider.LookupByGitAuthor(commit.AuthorName, commit.AuthorEmail)

		if found && len(tasks) > 0 {
			// 查找该用户的时间窗口匹配 task
			for _, task := range userTasks[userID] {
				taskStart, errStart := time.Parse(time.RFC3339, task.StartTime)
				taskEnd, errEnd := time.Parse(time.RFC3339, task.EndTime)
				if errStart != nil || errEnd != nil {
					if errStart != nil {
						fmt.Printf("警告: 解析 task %s 的 StartTime 失败: %v\n", task.TaskID, errStart)
					}
					if errEnd != nil {
						fmt.Printf("警告: 解析 task %s 的 EndTime 失败: %v\n", task.TaskID, errEnd)
					}
					continue
				}

				windowStart := taskStart.Add(-1 * time.Hour)
				windowEnd := taskEnd.Add(24 * time.Hour)

				if commit.Timestamp.Before(windowStart) || commit.Timestamp.After(windowEnd) {
					continue
				}

				// 时间窗口匹配，计算文件路径交集
				taskPaths := collectTaskFilePaths(task)

				if len(taskPaths) == 0 {
					// 纯 chat task，无代码输出
					matches = append(matches, TaskCommitMatch{
						TaskID:      task.TaskID,
						CommitHash:  commit.Hash,
						UserID:      userID,
						MatchScore:  0.5,
						MatchReason: "时间窗口匹配(无代码输出)",
					})
					commitTaskMap[commit.Hash] = append(commitTaskMap[commit.Hash], task.TaskID)
					continue
				}

				// 计算文件路径交集
				intersectCount := 0
				for _, f := range commit.FilesChanged {
					if taskPaths[f] {
						intersectCount++
					}
				}

				if intersectCount > 0 {
					denominator := len(commit.FilesChanged)
					if len(taskPaths) > denominator {
						denominator = len(taskPaths)
					}
					score := float64(intersectCount) / float64(denominator)
					matches = append(matches, TaskCommitMatch{
						TaskID:      task.TaskID,
						CommitHash:  commit.Hash,
						UserID:      userID,
						MatchScore:  score,
						MatchReason: "时间窗口+文件路径匹配",
					})
					commitTaskMap[commit.Hash] = append(commitTaskMap[commit.Hash], task.TaskID)
				} else {
					// 时间匹配但无文件交集
					matches = append(matches, TaskCommitMatch{
						TaskID:      task.TaskID,
						CommitHash:  commit.Hash,
						UserID:      userID,
						MatchScore:  0.3,
						MatchReason: "仅时间窗口匹配",
					})
					commitTaskMap[commit.Hash] = append(commitTaskMap[commit.Hash], task.TaskID)
				}
			}
		}

		// 代码来源分类
		classification := CommitClassification{
			CommitHash:   commit.Hash,
			UserID:       userID,
			LinesAdded:   commit.LinesAdded,
			LinesDeleted: commit.LinesDeleted,
		}

		if taskIDs, ok := commitTaskMap[commit.Hash]; ok && len(taskIDs) > 0 {
			classification.CodeSource = CodeSourceAICurrent
			classification.MatchedTaskIDs = taskIDs
		} else if found {
			classification.CodeSource = CodeSourceHuman
		} else if containsAIKeywords(commit.Message) {
			classification.CodeSource = CodeSourceAIOther
		} else {
			classification.CodeSource = CodeSourceUnknown
		}

		classifications = append(classifications, classification)
	}

	return matches, classifications
}

// collectTaskFilePaths 收集 task 中所有代码输出的文件路径
func collectTaskFilePaths(task TaskContentFile) map[string]bool {
	paths := make(map[string]bool)
	for _, conv := range task.Conversations {
		for _, co := range conv.CodeOutputs {
			if co.Path != "" {
				paths[co.Path] = true
			}
		}
	}
	return paths
}

// containsAIKeywords 检查 commit message 中是否包含 AI 相关关键词
func containsAIKeywords(message string) bool {
	lower := strings.ToLower(message)
	keywords := []string{"ai", "copilot", "gpt", "chatgpt", "claude"}
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}
