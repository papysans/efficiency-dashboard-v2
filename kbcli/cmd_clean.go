package main

import (
	"fmt"
	"time"

	"kanban/core/models"

	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "清洗过期或有问题的数据",
	Long:  "根据指定条件清洗 commits、tasks、task_conversations、user_productivity 表中的过期数据。",
	RunE: func(cmd *cobra.Command, args []string) error {
		beforeStr, _ := cmd.Flags().GetString("before")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		batchSize, _ := cmd.Flags().GetInt("batch-size")
		if beforeStr == "" {
			return fmt.Errorf("必须指定 --before 参数")
		}
		if batchSize <= 0 {
			batchSize = 1000
		}

		before, err := parseCleanTime(beforeStr)
		if err != nil {
			return fmt.Errorf("解析 --before 参数失败: %w", err)
		}

		db, err := models.OpenGormDB(cfg.StatDatabase.DSN())
		if err != nil {
			return fmt.Errorf("连接数据库失败: %w", err)
		}
		sqlDB, _ := db.DB()
		defer sqlDB.Close()

		tables := []struct {
			name      string
			field     string
			condition string
			model     interface{}
		}{
			{"commits", "CommitTime", "commit_time < ?", &models.Commit{}},
			{"tasks", "EndTime", "end_time < ?", &models.Task{}},
			{"task_conversations", "EndTime", "end_time < ?", &models.Conversation{}},
			{"user_productivity", "CreateTime", "create_time < ?", &models.UserProductivity{}},
		}

		for _, t := range tables {
			logInfof("[clean] 处理 %s 表中 %s 早于 %s 的记录...", t.name, t.field, before.Format("2006-01-02 15:04:05"))
			if dryRun {
				var cnt int64
				if err := db.Table(t.name).Where(t.condition, before).Count(&cnt).Error; err != nil {
					logErrorf("[clean] 统计 %s 失败: %v", t.name, err)
					continue
				}
				logInfof("[clean] %s 表将有 %d 条记录被删除（dry-run）", t.name, cnt)
				continue
			}
			deleted, err := batchDelete(db, t.name, t.condition, before, batchSize)
			if err != nil {
				logErrorf("[clean] 清洗 %s 失败: %v", t.name, err)
				continue
			}
			logInfof("[clean] %s 表已删除 %d 条记录", t.name, deleted)
		}

		logInfo("[clean] 全部清洗完成")
		return nil
	},
}

func batchDelete(db *gorm.DB, table, condition string, before time.Time, batchSize int) (int64, error) {
	var total int64
	for {
		sql := fmt.Sprintf(
			"DELETE FROM %s WHERE ctid IN (SELECT ctid FROM %s WHERE %s LIMIT ?)",
			table, table, condition,
		)
		result := db.Exec(sql, before, batchSize)
		if result.Error != nil {
			return total, result.Error
		}
		total += result.RowsAffected
		if result.RowsAffected == 0 {
			break
		}
	}
	return total, nil
}

func parseCleanTime(s string) (time.Time, error) {
	layouts := []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("无法解析时间: %s", s)
}

func init() {
	cleanCmd.Flags().String("before", "", "清洗早于该日期的数据，格式如 2024-01-01 或 2024-01-01 12:00:00")
	cleanCmd.Flags().Bool("dry-run", false, "仅预览将要删除的记录数，不实际执行删除")
	cleanCmd.Flags().Int("batch-size", 1000, "每批次删除的记录数，避免一次性删除大量数据导致锁表")
	rootCmd.AddCommand(cleanCmd)
}
