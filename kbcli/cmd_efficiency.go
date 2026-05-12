package main

import (
	"fmt"
	"time"

	"kanban/core/models"
	"kanban/core/utils"

	"github.com/spf13/cobra"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type userTaskAgg struct {
	UserId             string
	TaskIds            models.StringJSON
	WorkDirIds         models.StringJSON
	TaskDiffLines      int64
	UpstreamTokens     int64
	DownstreamTokens   int64
	Cost               float64
	TaskRealMinutes    float64
	TaskAncientMinutes float64
}

type userCommitAgg struct {
	UserId                   string
	CommitIds                models.StringJSON
	CommitDiffLines          int64
	CommitAncientMinutes     float64
	commitRealAiMinutes      float64
	CommitRealAncientMinutes float64
	CommitRealMinutes        float64
}

func getAllDates(db *gorm.DB) ([]string, error) {
	var dates []string
	if err := db.Raw(`
		SELECT DISTINCT dt FROM (
			SELECT TO_CHAR(DATE(start_time), 'YYYYMMDD') AS dt FROM tasks WHERE start_time IS NOT NULL
			UNION
			SELECT TO_CHAR(DATE(commit_time), 'YYYYMMDD') AS dt FROM commits WHERE commit_time IS NOT NULL
		) sub
		ORDER BY dt
	`).Scan(&dates).Error; err != nil {
		return nil, fmt.Errorf("查询日期列表失败: %w", err)
	}
	return dates, nil
}

func calculateUserProductivity(db *gorm.DB, dateStr string, userNameMap, taskUserNameMap, commitUserNameMap map[string]string) (int, error) {
	taskAggMap, err := aggregateTasksByUser(db, dateStr)
	if err != nil {
		return 0, fmt.Errorf("聚合task数据失败: %w", err)
	}
	logInfof("聚合task数据: %d 个用户", len(taskAggMap))

	commitAggMap, err := aggregateCommitsByUser(db, dateStr)
	if err != nil {
		return 0, fmt.Errorf("聚合commit数据失败: %w", err)
	}
	logInfof("聚合commit数据: %d 个用户", len(commitAggMap))

	allUserIDs := make(map[string]bool)
	for uid := range taskAggMap {
		allUserIDs[uid] = true
	}
	for uid := range commitAggMap {
		allUserIDs[uid] = true
	}

	if len(allUserIDs) == 0 {
		logInfo("没有找到有task或commit数据的用户")
		return 0, nil
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		for uid := range allUserIDs {
			ta := taskAggMap[uid]
			ca := commitAggMap[uid]

			userName := userNameMap[uid]
			if userName == "" {
				userName = taskUserNameMap[uid]
			}
			if userName == "" {
				userName = commitUserNameMap[uid]
			}

			var TaskIdsJSON, WorkDirIdsJson, commitIDsJSON []byte
			var taskDiffLines, upstreamTokens, downstreamTokens int64
			var cost, taskRealMinutes, taskAncientMinutes float64
			var commitDiffLines int64
			var commitAncientMinutes, commitRealAiMinutes, commitRealAncientMinutes, commitRealMinutes float64

			if ta != nil {
				TaskIdsJSON = defaultSliceJSON(ta.TaskIds)
				WorkDirIdsJson = defaultSliceJSON(ta.WorkDirIds)
				taskDiffLines = ta.TaskDiffLines
				upstreamTokens = ta.UpstreamTokens
				downstreamTokens = ta.DownstreamTokens
				cost = ta.Cost
				taskRealMinutes = ta.TaskRealMinutes
				taskAncientMinutes = ta.TaskAncientMinutes
			}
			if ca != nil {
				commitIDsJSON = defaultSliceJSON(ca.CommitIds)
				commitDiffLines = ca.CommitDiffLines
				commitAncientMinutes = ca.CommitAncientMinutes
				commitRealAiMinutes = ca.commitRealAiMinutes
				commitRealAncientMinutes = ca.CommitRealAncientMinutes
				commitRealMinutes = ca.CommitRealMinutes
			}

			taskEffRatio := utils.CalcEfficiencyRatio(taskAncientMinutes, taskRealMinutes)
			commitEffRatio := utils.CalcEfficiencyRatio(commitAncientMinutes, commitRealMinutes)

			if TaskIdsJSON == nil {
				TaskIdsJSON = []byte("[]")
			}
			if WorkDirIdsJson == nil {
				WorkDirIdsJson = []byte("[]")
			}
			if commitIDsJSON == nil {
				commitIDsJSON = []byte("[]")
			}

			createTime, err := time.Parse("20060102", dateStr)
			if err != nil {
				logWarnf("解析日期字符串失败 [%s]: %v", dateStr, err)
			}
			createTime = time.Date(createTime.Year(), createTime.Month(), createTime.Day(), 0, 0, 0, 0, time.UTC)

			up := models.UserProductivity{
				UserProductivityId:       uid + "_" + dateStr,
				CreateTime:               createTime,
				UserId:                   uid,
				UserName:                 userName,
				TaskIds:                  models.StringJSON(TaskIdsJSON),
				WorkDirIds:               models.StringJSON(WorkDirIdsJson),
				TaskDiffLines:            int(taskDiffLines),
				UpstreamTokens:           upstreamTokens,
				DownstreamTokens:         downstreamTokens,
				Cost:                     cost,
				TaskRealMinutes:          taskRealMinutes,
				TaskAncientMinutes:       taskAncientMinutes,
				TaskEfficiencyRatio:      taskEffRatio,
				CommitIds:                models.StringJSON(commitIDsJSON),
				CommitDiffLines:          int(commitDiffLines),
				CommitAncientMinutes:     commitAncientMinutes,
				CommitRealAiMinutes:      commitRealAiMinutes,
				CommitRealAncientMinutes: commitRealAncientMinutes,
				CommitRealMinutes:        commitRealMinutes,
				CommitEfficiencyRatio:    commitEffRatio,
			}

			result := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "user_productivity_id"}},
				DoUpdates: clause.AssignmentColumns([]string{
					"user_name", "task_ids", "work_dir_ids", "task_diff_lines",
					"upstream_tokens", "downstream_tokens", "cost",
					"task_real_minutes", "task_ancient_minutes", "task_efficiency_ratio",
					"commit_ids", "commit_diff_lines", "commit_ancient_minutes",
					"commit_real_ai_minutes", "commit_real_ancient_minutes", "commit_real_minutes",
					"commit_efficiency_ratio", "updated_at",
				}),
			}).Create(&up)
			if result.Error != nil {
				return fmt.Errorf("写入user_productivity失败 [user_id=%s]: %w", uid, result.Error)
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

	return len(allUserIDs), nil
}

func loadUserNames(db *gorm.DB) (map[string]string, error) {
	var userOrgs []models.UserOrg
	if err := db.Select("user_id, user_name").Where("user_name IS NOT NULL AND user_name != ''").Find(&userOrgs).Error; err != nil {
		return nil, fmt.Errorf("查询user_org用户名失败: %w", err)
	}
	result := make(map[string]string)
	for _, uo := range userOrgs {
		result[uo.UserId] = uo.UserName
	}
	return result, nil
}

func aggregateTasksByUser(db *gorm.DB, dateStr string) (map[string]*userTaskAgg, error) {
	var rows []userTaskAgg
	if err := db.Raw(`
		SELECT
			user_id,
			COALESCE(array_to_json(array_agg(task_id)), '[]') as task_ids,
			COALESCE(array_to_json(array_agg(DISTINCT work_dir_id) FILTER (WHERE work_dir_id IS NOT NULL AND work_dir_id != '')), '[]') as work_dir_ids,
			COALESCE(SUM(diff_lines), 0) as task_diff_lines,
			COALESCE(SUM(upstream_tokens), 0) as upstream_tokens,
			COALESCE(SUM(downstream_tokens), 0) as downstream_tokens,
			COALESCE(SUM(cost), 0) as cost,
			COALESCE(SUM(COALESCE(task_real_minutes_manual, task_real_minutes)), 0) as task_real_minutes,
			COALESCE(SUM(COALESCE(task_ancient_minutes_manual, task_ancient_minutes)), 0) as task_ancient_minutes
		FROM tasks
		WHERE user_id IS NOT NULL AND user_id != '' AND DATE(start_time) = $1
		GROUP BY user_id
	`, dateStr).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("查询task聚合数据失败: %w", err)
	}

	result := make(map[string]*userTaskAgg)
	for i := range rows {
		result[rows[i].UserId] = &rows[i]
	}
	return result, nil
}

func aggregateCommitsByUser(db *gorm.DB, dateStr string) (map[string]*userCommitAgg, error) {
	var rows []userCommitAgg
	if err := db.Raw(`
		SELECT
			user_id,
			COALESCE(array_to_json(array_agg(commit_id)), '[]') as commit_ids,
			COALESCE(SUM(diff_lines), 0) as commit_diff_lines,
			COALESCE(SUM(COALESCE(commit_ancient_minutes_manual, commit_ancient_minutes)), 0) as commit_ancient_minutes,
			COALESCE(SUM(commit_real_ai_minutes), 0) as commit_real_ai_minutes,
			COALESCE(SUM(commit_real_ancient_minutes), 0) as commit_real_ancient_minutes,
			COALESCE(SUM(COALESCE(commit_real_minutes_manual, commit_real_minutes)), 0) as commit_real_minutes
		FROM commits
		WHERE user_id IS NOT NULL AND user_id != '' AND DATE(commit_time) = $1
		GROUP BY user_id
	`, dateStr).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("查询commit聚合数据失败: %w", err)
	}

	result := make(map[string]*userCommitAgg)
	for i := range rows {
		result[rows[i].UserId] = &rows[i]
	}
	return result, nil
}

func defaultSliceJSON(j models.StringJSON) []byte {
	if j == "" || j == "null" {
		return []byte("[]")
	}
	return []byte(j)
}

func loadUserNamesFromTasks(db *gorm.DB) (map[string]string, error) {
	type row struct {
		UserId   string
		UserName string
	}
	var rows []row
	if err := db.Raw(`SELECT DISTINCT user_id, user_name FROM tasks WHERE user_id IS NOT NULL AND user_id != '' AND user_name IS NOT NULL AND user_name != ''`).Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for _, r := range rows {
		if _, exists := result[r.UserId]; !exists {
			result[r.UserId] = r.UserName
		}
	}
	return result, nil
}

func loadUserNamesFromCommits(db *gorm.DB) (map[string]string, error) {
	type row struct {
		UserId   string
		UserName string
	}
	var rows []row
	if err := db.Raw(`SELECT DISTINCT user_id, user_name FROM commits WHERE user_id IS NOT NULL AND user_id != '' AND user_name IS NOT NULL AND user_name != ''`).Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for _, r := range rows {
		if _, exists := result[r.UserId]; !exists {
			result[r.UserId] = r.UserName
		}
	}
	return result, nil
}

func runEfficiency(dateStr string) error {
	startTime := time.Now()

	if dateStr != "" {
		if _, err := time.Parse("20060102", dateStr); err != nil {
			recordCommandRun("efficiency", startTime, 0, 0, 0, err)
			return fmt.Errorf("--date 格式应为 YYYYMMDD，当前: %s, 详情: %w", dateStr, err)
		}
	}

	db, err := models.OpenGormDB(cfg.StatDatabase.DSN())
	if err != nil {
		recordCommandRun("efficiency", startTime, 0, 0, 0, err)
		return fmt.Errorf("连接数据库失败: %w", err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	var dates []string
	if dateStr != "" {
		dates = []string{dateStr}
	} else {
		dates, err = getAllDates(db)
		if err != nil {
			recordCommandRun("efficiency", startTime, 0, 0, 0, err)
			return fmt.Errorf("获取日期列表失败: %w", err)
		}
		if len(dates) == 0 {
			logInfo("没有找到任何task或commit数据")
			recordCommandRun("efficiency", startTime, 0, 0, 0, nil)
			return nil
		}
		logInfof("共发现 %d 个日期需要处理", len(dates))
	}
	userNameMap, err := loadUserNames(db)
	if err != nil {
		recordCommandRun("efficiency", startTime, 0, 0, 0, err)
		return fmt.Errorf("加载用户名称失败: %w", err)
	}

	taskUserNameMap, err := loadUserNamesFromTasks(db)
	if err != nil {
		logWarnf("加载task用户名失败: %v", err)
	}
	commitUserNameMap, err := loadUserNamesFromCommits(db)
	if err != nil {
		logWarnf("加载commit用户名失败: %v", err)
	}

	totalUserCount := 0
	for _, d := range dates {
		logInfof("=== 处理日期: %s ===", d)
		userCount, err := calculateUserProductivity(db, d, userNameMap, taskUserNameMap, commitUserNameMap)
		if err != nil {
			recordCommandRun("efficiency", startTime, totalUserCount, 0, 0, err)
			return fmt.Errorf("计算用户生产力失败 [date=%s]: %w", d, err)
		}
		logInfof("用户生产力计算完成: %d 条记录 (日期=%s)", userCount, d)
		totalUserCount += userCount
	}

	logInfof("全部完成: 用户 %d 条", totalUserCount)
	recordCommandRun("efficiency", startTime, totalUserCount, 0, 0, nil)
	return nil
}

var efficiencyCmd = &cobra.Command{
	Use:   "efficiency",
	Short: "按日计算用户和组织效能数据",
	Long:  "根据已导入的task、commit、user_org数据，按日计算各用户的生产力数据，写入user_productivity表。如有--date参数，则只处理该日期的数据，否则处理所有日期数据",
	RunE: func(cmd *cobra.Command, args []string) error {
		dateStr, _ := cmd.Flags().GetString("date")
		remote, _ := cmd.Flags().GetString("remote")

		if remote != "" {
			return sendToRemote(remote, "efficiency", map[string]interface{}{
				"date": dateStr,
			})
		}

		return runEfficiency(dateStr)
	},
}

func init() {
	efficiencyCmd.Flags().SortFlags = false
	efficiencyCmd.Flags().String("date", "", "聚合日期，格式YYYYMMDD，不指定则处理所有日期")
	efficiencyCmd.Flags().String("remote", "", "远程kbcli服务地址（如 http://127.0.0.1:8080），指定后命令将发送到远程执行")
	rootCmd.AddCommand(efficiencyCmd)
}
