package main

import (
	"fmt"
	"log"
	"time"

	"kanban/core/models"

	"gorm.io/gorm"
)

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

func UpdateTaskTitle(db *gorm.DB, taskID string, title string) error {
	result := db.Model(&models.Task{}).Where("task_id = ?", taskID).
		Updates(map[string]interface{}{"title": title, "updated_at": time.Now()})
	if result.Error != nil {
		log.Printf("回写title失败: %v", result.Error)
		return result.Error
	}
	return nil
}
