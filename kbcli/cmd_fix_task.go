package main

import (
	"fmt"
	"kanban/kbcli/internal/appconfig"
	"kanban/kbcli/internal/logx"
	"kanban/kbcli/internal/util"
	"time"

	"kanban/core/models"

	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

var fixTaskCmd = &cobra.Command{
	Use:   "fix-task",
	Short: "使用AI为Task记录补充标题和传统耗时估算",
	Long: `扫描conversation目录，对title为空或task_ancient_minutes_manual为空的task进行标题提炼和工作量估算。
支持--task指定单个任务ID，此时直接根据Task记录定位conversation文件进行处理。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		taskDir, _ := cmd.Flags().GetString("task-dir")
		startDate, _ := cmd.Flags().GetString("start-date")
		endDate, _ := cmd.Flags().GetString("end-date")
		date, _ := cmd.Flags().GetString("date")
		specificTask, _ := cmd.Flags().GetString("task")

		if taskDir == "" {
			taskDir = appconfig.Cfg.TaskDir
		}

		max, _ := cmd.Flags().GetInt("max")

		// 未显式传 start-date 且非单日(date)/单任务(task)模式时，套全局分析起始日下界。
		if date == "" && specificTask == "" {
			startDate = appconfig.ApplyAnalysisFloor(startDate)
		}

		return runFixTask(taskDir, startDate, endDate, date, specificTask, max)
	},
}

func runFixTask(taskDir, startDateStr, endDateStr, dateStr, specificTaskID string, max int) error {
	db, err := models.OpenGormDB(appconfig.Cfg.StatDatabase.DSN())
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	if specificTaskID != "" {
		logx.Infof("===== 开始处理指定task: %s =====", specificTaskID)
		if err := fixSingleTask(db, taskDir, specificTaskID); err != nil {
			return fmt.Errorf("处理指定task失败: %w", err)
		}
		logx.Info("===== fix-task 命令执行完成 =====")
		return nil
	}

	startDate, endDate, err := util.ParseDateRange(startDateStr, endDateStr, dateStr)
	if err != nil {
		return err
	}

	logx.Info("===== 开始处理task标题和估算 =====")
	if err := fixTasks(db, taskDir, startDate, endDate, max); err != nil {
		return fmt.Errorf("处理task失败: %w", err)
	}

	logx.Info("===== fix-task 命令执行完成 =====")
	return nil
}

func fixSingleTask(db *gorm.DB, taskDir, taskID string) error {
	var task models.Task
	if err := db.Where("task_id = ?", taskID).First(&task).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("task不存在: %s", taskID)
		}
		return fmt.Errorf("查询task失败: %w", err)
	}

	convPath, ok := models.GetConversationFilePath(taskDir, &task)
	if !ok {
		return fmt.Errorf("找不到conversation文件: session=%s", task.SessionId)
	}

	convs, err := parseConversationFile(convPath)
	if err != nil {
		return fmt.Errorf("读取conversation文件失败: %w", err)
	}

	var userInputs []string
	var totalChars int64
	for _, c := range convs {
		if c.UserInput != "" {
			userInputs = append(userInputs, c.UserInput)
			totalChars += int64(len(c.UserInput))
		}
	}

	if task.Title == "" && len(userInputs) > 0 {
		title, err := callAIForTaskTitle(db, task.TaskId, userInputs)
		if err == nil && title != "" {
			UpdateTaskTitle(db, task.TaskId, title)
			task.Title = title
			logx.Infof("AI提取title完成: task=%s, title=%s", task.TaskId, title)
		}
	}

	if task.TaskAncientMinutesManual == nil {
		if len(userInputs) == 0 {
			logx.Warnf("task无对话数据，跳过估时: %s", task.TaskId)
			return nil
		}
		if task.Title == "" {
			logx.Warnf("task无标题，跳过估时: %s", task.TaskId)
			return nil
		}
		minutes, reason, err := callAIForAncientEstimation(task.Title, task.DiffLines, totalChars)
		if err != nil {
			return fmt.Errorf("AI估时失败: %w", err)
		}
		if err := UpdateTaskAncientEstimation(db, task.TaskId, minutes, reason); err != nil {
			return fmt.Errorf("更新task估时结果失败: %w", err)
		}
		logx.Infof("task估时完成: %s, minutes=%.1f", task.TaskId, minutes)
	}

	return nil
}

func fixTasks(db *gorm.DB, taskDir string, startDate, endDate *time.Time, max int) error {
	query := db.Model(&models.Task{})
	if startDate != nil {
		query = query.Where("start_time >= ?", *startDate)
	}
	if endDate != nil {
		query = query.Where("start_time <= ?", *endDate)
	}

	var tasks []models.Task
	if err := query.Find(&tasks).Error; err != nil {
		return fmt.Errorf("查询task失败: %w", err)
	}

	if len(tasks) == 0 {
		logx.Info("没有需要处理的task记录")
		return nil
	}

	logx.Infof("找到 %d 个task记录待检查", len(tasks))

	var checked, skipped, processed int
	for _, task := range tasks {
		if max > 0 && processed >= max {
			logx.Infof("已达到最大处理数量 %d，停止", max)
			break
		}
		checked++
		if task.Title != "" && task.TaskAncientMinutesManual != nil {
			skipped++
			continue
		}
		if err := fixSingleTask(db, taskDir, task.TaskId); err != nil {
			logx.Errorf("处理task失败: %s, %v", task.TaskId, err)
			continue
		}
		processed++
	}

	logx.Infof("task处理完成: 检查 %d 个, 跳过 %d 个, 处理 %d 个", checked, skipped, processed)
	return nil
}

func init() {
	fixTaskCmd.Flags().SortFlags = false
	fixTaskCmd.Flags().String("task", "", "指定任务ID，只处理该任务（与日期筛选互斥）")
	fixTaskCmd.Flags().String("task-dir", "", "task 目录路径（包含summary和conversation子目录），默认从配置文件获取")
	fixTaskCmd.Flags().String("start-date", "", "限定起始日期，格式 YYYYMMDD，为空则不限")
	fixTaskCmd.Flags().String("end-date", "", "限定结束日期，格式 YYYYMMDD，为空则不限")
	fixTaskCmd.Flags().String("date", "", "限定日期，格式 YYYYMMDD，限定活跃时间在该日期之内（与start-date/end-date互斥）")
	fixTaskCmd.Flags().Int("max", 0, "最多处理多少个Task，0表示不限制")
	rootCmd.AddCommand(fixTaskCmd)
}
