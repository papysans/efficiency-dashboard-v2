package main

import (
	"fmt"
	"time"

	"kanban/core/models"

	"gorm.io/gorm"
)

// GetTaskIDsForEstimation 获取需要 AI 估算的任务 ID 列表
// specificTaskID 为空时，返回最近 50 个未估算的任务
func GetTaskIDsForEstimation(db *gorm.DB, specificTaskID string) ([]string, error) {
	var taskIDs []string
	q := db.Model(&models.Task{}).Select("task_id").
		Where("task_ancient_minutes IS NULL AND task_ancient_minutes_manual IS NULL")
	if specificTaskID != "" {
		q = q.Where("task_id = ?", specificTaskID)
	} else {
		q = q.Order("start_time DESC").Limit(50)
	}
	if err := q.Pluck("task_id", &taskIDs).Error; err != nil {
		return nil, err
	}
	return taskIDs, nil
}

// GetConvInputForEstimation 获取任务的对话输入和代码输出数据，用于 AI 估算
func GetConvInputForEstimation(db *gorm.DB, taskID string) ([]string, []string, int64, int64, error) {
	type row struct {
		UserInput        *string
		Diff             *string
		DiffLines        *int64
		UpstreamTokens   *int64
		DownstreamTokens *int64
	}
	var rows []row
	if err := db.Model(&models.TaskConversation{}).
		Select("user_input, diff, diff_lines, upstream_tokens, downstream_tokens").
		Where("task_id = ?", taskID).
		Order("start_time").
		Scan(&rows).Error; err != nil {
		return nil, nil, 0, 0, err
	}

	var userInputs []string
	var codeOutputs []string
	var totalLines int64
	var totalChars int64
	for _, r := range rows {
		if r.UserInput != nil && *r.UserInput != "" {
			userInputs = append(userInputs, *r.UserInput)
			totalChars += int64(len(*r.UserInput))
		}
		if r.Diff != nil && *r.Diff != "" {
			codeOutputs = append(codeOutputs, *r.Diff)
		}
		if r.DiffLines != nil {
			totalLines += *r.DiffLines
		}
	}
	return userInputs, codeOutputs, totalChars, totalLines, nil
}

// UpdateTaskAncientEstimation 将 AI 估算结果写回数据库
func UpdateTaskAncientEstimation(db *gorm.DB, taskID string, minutes float64, reason string) error {
	result := db.Model(&models.Task{}).Where("task_id = ?", taskID).
		Updates(map[string]interface{}{
			"task_ancient_minutes":        minutes,
			"task_ancient_minutes_reason": reason,
			"updated_at":                  time.Now(),
		})
	if result.Error != nil {
		return fmt.Errorf("db update failed: %w", result.Error)
	}
	return nil
}
