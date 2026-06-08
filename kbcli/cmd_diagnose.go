package main

import (
	"fmt"
	"kanban/core/models"
	"kanban/kbcli/internal/logx"

	"github.com/spf13/cobra"
)

var diagnoseCmd = &cobra.Command{
	Use:   "diagnose",
	Short: "诊断 user_productivity 表中数据为空的原因",
	Long:  "诊断指定用户在指定日期的 tasks 和 commits 数据，找出为什么 task_ids, commit_ids 为空",
	RunE: func(cmd *cobra.Command, args []string) error {
		UserId, _ := cmd.Flags().GetString("user-id")
		dateStr, _ := cmd.Flags().GetString("date")

		// 如果不指定 user-id 和 date，则执行 find_users.sql 中的逻辑
		if UserId == "" && dateStr == "" {
			return runFindUsers()
		}

		// 如果指定了 user-id 或 date 中的任意一个，则要求两个都必须指定
		if UserId == "" || dateStr == "" {
			return fmt.Errorf("--user-id 和 --date 必须同时指定或同时不指定")
		}

		return runDiagnose(UserId, dateStr)
	},
}

func runFindUsers() error {
	db, err := models.OpenGormDB(cfg.StatDatabase.DSN())
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	logx.Info("===== 查找数据库中实际存在的 user_id 和日期 =====")

	// 1. tasks 表中唯一的 user_id（前20个）
	logx.Info("1. tasks 表中唯一的 user_id（前20个）")
	type taskUser struct {
		UserId       string
		TaskCount    int64
		EarliestDate string
		LatestDate   string
	}
	var taskUsers []taskUser
	db.Raw(`
		SELECT
			user_id,
			COUNT(*) as task_count,
			TO_CHAR(MIN(start_time), 'YYYY-MM-DD') as earliest_date,
			TO_CHAR(MAX(start_time), 'YYYY-MM-DD') as latest_date
		FROM tasks
		WHERE user_id IS NOT NULL AND user_id != ''
		GROUP BY user_id
		ORDER BY task_count DESC
		LIMIT 20
	`).Scan(&taskUsers)
	logx.Infof("   找到 %d 个用户:", len(taskUsers))
	for _, tu := range taskUsers {
		logx.Infof("   - UserId: %s, TaskCount: %d, 日期范围: %s ~ %s",
			tu.UserId, tu.TaskCount, tu.EarliestDate, tu.LatestDate)
	}
	logx.Info("")

	// 2. commits 表中唯一的 user_id（前20个）
	logx.Info("2. commits 表中唯一的 user_id（前20个）")
	type commitUser struct {
		UserId       string
		CommitCount  int64
		EarliestDate string
		LatestDate   string
	}
	var commitUsers []commitUser
	db.Raw(`
		SELECT
			user_id,
			COUNT(*) as commit_count,
			TO_CHAR(MIN(commit_time), 'YYYY-MM-DD') as earliest_date,
			TO_CHAR(MAX(commit_time), 'YYYY-MM-DD') as latest_date
		FROM commits
		WHERE user_id IS NOT NULL AND user_id != ''
		GROUP BY user_id
		ORDER BY commit_count DESC
		LIMIT 20
	`).Scan(&commitUsers)
	logx.Infof("   找到 %d 个用户:", len(commitUsers))
	for _, cu := range commitUsers {
		logx.Infof("   - UserId: %s, CommitCount: %d, 日期范围: %s ~ %s",
			cu.UserId, cu.CommitCount, cu.EarliestDate, cu.LatestDate)
	}
	logx.Info("")

	// 3. 最新的 tasks 和 commits 日期
	logx.Info("3. 最新的 tasks 和 commits 日期")
	type tableDateInfo struct {
		TableName     string
		LatestDateStr string
		LatestDate    string
		TotalCount    int64
	}
	var tableDates []tableDateInfo
	db.Raw(`
		SELECT
			'tasks' as table_name,
			TO_CHAR(DATE(MAX(start_time)), 'YYYYMMDD') as latest_date_str,
			DATE(MAX(start_time)) as latest_date,
			COUNT(*) as total_count
		FROM tasks
		UNION ALL
		SELECT
			'commits' as table_name,
			TO_CHAR(DATE(MAX(commit_time)), 'YYYYMMDD') as latest_date_str,
			DATE(MAX(commit_time)) as latest_date,
			COUNT(*) as total_count
		FROM commits
	`).Scan(&tableDates)
	for _, td := range tableDates {
		logx.Infof("   - 表: %s, 最新日期: %s (%s), 总记录数: %d",
			td.TableName, td.LatestDateStr, td.LatestDate, td.TotalCount)
	}
	logx.Info("")

	// 4. 建议诊断参数
	logx.Info("4. 建议诊断参数")
	if len(taskUsers) > 0 {
		suggestedUserId := taskUsers[0].UserId
		var latestDate string
		db.Raw(`
			SELECT TO_CHAR(DATE(MAX(start_time)), 'YYYYMMDD') as date_str
			FROM tasks
		`).Scan(&latestDate)
		logx.Info("   建议使用的诊断命令:")
		logx.Infof("   kbcli diagnose --user-id %s --date %s", suggestedUserId, latestDate)
	}
	logx.Info("")

	logx.Info("===== 查询完成 =====")
	return nil
}

func runDiagnose(UserId, dateStr string) error {
	db, err := models.OpenGormDB(cfg.StatDatabase.DSN())
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	logx.Infof("===== 诊断用户: %s, 日期: %s =====", UserId, dateStr)

	// 1. 检查数据库时区
	logx.Info("1. 检查数据库时区设置")
	var timezone string
	db.Raw("SHOW timezone").Scan(&timezone)
	logx.Infof("   数据库时区: %s", timezone)

	// 2. 检查 user_id 字段
	logx.Info("2. user_id 字段检查")
	type UserIdCheck struct {
		UserId         string
		UserIdLength   int
		IsTrimmed      bool
		FirstCharAscii int
	}
	var uidCheck UserIdCheck
	db.Raw(`
		SELECT 
			user_id,
			LENGTH(user_id) as user_id_length,
			user_id = TRIM(user_id) as is_trimmed,
			ASCII(SUBSTRING(user_id, 1, 1)) as first_char_ascii
		FROM tasks 
		WHERE user_id = $1
		LIMIT 1
	`, UserId).Scan(&uidCheck)
	logx.Infof("   UserId: %s", uidCheck.UserId)
	logx.Infof("   长度: %d", uidCheck.UserIdLength)
	logx.Infof("   是否已去除首尾空格: %t", uidCheck.IsTrimmed)
	logx.Infof("   第一个字符ASCII码: %d", uidCheck.FirstCharAscii)

	// 2. 检查 tasks 表中该用户的所有记录
	logx.Infof("3. tasks 表中用户 %s 的所有记录（不限日期）", UserId)
	type taskInfo struct {
		UserId        string
		TaskId        string
		StartTime     string
		StartDate     string
		DateStrFormat string
		WorkDirId     string
	}
	var tasks []taskInfo
	db.Raw(`
		SELECT 
			user_id,
			task_id,
			TO_CHAR(start_time, 'YYYY-MM-DD HH24:MI:SS') as start_time,
			TO_CHAR(DATE(start_time), 'YYYY-MM-DD') as start_date,
			TO_CHAR(DATE(start_time), 'YYYYMMDD') as date_str_format,
			work_dir_id
		FROM tasks 
		WHERE user_id = $1
		ORDER BY start_time DESC
	`, UserId).Scan(&tasks)
	logx.Infof("   找到 %d 条记录:", len(tasks))
	for _, t := range tasks {
		logx.Infof("   - TaskId: %s, StartTime: %s, WorkDirId: %s", t.TaskId, t.StartTime, t.WorkDirId)
	}
	logx.Info("")

	// 3. 检查 tasks 表中该用户在指定日期的记录
	logx.Infof("4. tasks 表中用户 %s 在指定日期 %s 的记录", UserId, dateStr)
	var tasksOnDate []taskInfo
	db.Raw(`
		SELECT 
			user_id,
			task_id,
			TO_CHAR(start_time, 'YYYY-MM-DD HH24:MI:SS') as start_time,
			TO_CHAR(DATE(start_time), 'YYYY-MM-DD') as start_date,
			TO_CHAR(DATE(start_time), 'YYYYMMDD') as date_str_format,
			work_dir_id
		FROM tasks 
		WHERE user_id = $1 AND DATE(start_time) = $2
		ORDER BY start_time
	`, UserId, dateStr).Scan(&tasksOnDate)
	logx.Infof("   找到 %d 条记录:", len(tasksOnDate))
	for _, t := range tasksOnDate {
		logx.Infof("   - TaskId: %s, StartTime: %s, WorkDirId: %s", t.TaskId, t.StartTime, t.WorkDirId)
	}
	logx.Info("")

	// 5. 模拟 tasks 聚合查询
	logx.Infof("5. 模拟 tasks 聚合查询（用户: %s，日期: %s）", UserId, dateStr)
	type taskAgg struct {
		UserId             string
		TaskIds            string
		TaskDiffLines      int64
		UpstreamTokens     int64
		DownstreamTokens   int64
		Cost               float64
		TaskRealMinutes    float64
		TaskAncientMinutes float64
	}
	var taskAggs []taskAgg
	db.Raw(`
		SELECT
			user_id,
			COALESCE(array_to_json(array_agg(task_id)), '[]') as task_ids,
			COALESCE(SUM(diff_lines), 0) as task_diff_lines,
			COALESCE(SUM(upstream_tokens), 0) as upstream_tokens,
			COALESCE(SUM(downstream_tokens), 0) as downstream_tokens,
			COALESCE(SUM(cost), 0) as cost,
			COALESCE(SUM(COALESCE(task_real_minutes_manual, task_real_minutes)), 0) as task_real_minutes,
			COALESCE(SUM(COALESCE(task_ancient_minutes_manual, task_ancient_minutes)), 0) as task_ancient_minutes
		FROM tasks
		WHERE user_id  = $1 AND DATE(start_time) = $2
		GROUP BY user_id
	`, UserId, dateStr).Scan(&taskAggs)
	logx.Infof("   聚合结果数量: %d", len(taskAggs))
	for _, ta := range taskAggs {
		logx.Infof("   - UserId: %s", ta.UserId)
		logx.Infof("     TaskIds: %s", ta.TaskIds)
		logx.Infof("     TaskDiffLines: %d, TaskRealMinutes: %.2f", ta.TaskDiffLines, ta.TaskRealMinutes)
	}
	logx.Info("")

	// 6. 检查 commits 表中该用户的所有记录
	logx.Infof("6. commits 表中用户 %s 的所有记录（不限日期）", UserId)
	type commitInfo struct {
		UserId        string
		CommitId      string
		CommitTime    string
		CommitDate    string
		DateStrFormat string
	}
	var commits []commitInfo
	db.Raw(`
		SELECT 
			user_id,
			commit_id,
			TO_CHAR(commit_time, 'YYYY-MM-DD HH24:MI:SS') as commit_time,
			TO_CHAR(DATE(commit_time), 'YYYY-MM-DD') as commit_date,
			TO_CHAR(DATE(commit_time), 'YYYYMMDD') as date_str_format
		FROM commits 
		WHERE user_id = $1
		ORDER BY commit_time DESC
		LIMIT 10
	`, UserId).Scan(&commits)
	logx.Infof("   找到 %d 条记录:", len(commits))
	for _, c := range commits {
		logx.Infof("   - CommitId: %s, CommitTime: %s", c.CommitId, c.CommitTime)
	}
	logx.Info("")

	// 7. 检查 commits 表中该用户在指定日期的记录
	logx.Infof("7. commits 表中用户 %s 在指定日期 %s 的记录", UserId, dateStr)
	var commitsOnDate []commitInfo
	db.Raw(`
		SELECT 
			user_id,
			commit_id,
			TO_CHAR(commit_time, 'YYYY-MM-DD HH24:MI:SS') as commit_time,
			TO_CHAR(DATE(commit_time), 'YYYY-MM-DD') as commit_date,
			TO_CHAR(DATE(commit_time), 'YYYYMMDD') as date_str_format
		FROM commits 
		WHERE user_id = $1 AND DATE(commit_time) = $2
		ORDER BY commit_time
	`, UserId, dateStr).Scan(&commitsOnDate)
	logx.Infof("   找到 %d 条记录:", len(commitsOnDate))
	for _, c := range commitsOnDate {
		logx.Infof("   - CommitId: %s, CommitTime: %s", c.CommitId, c.CommitTime)
	}
	logx.Info("")

	// 8. 模拟 commits 聚合查询
	logx.Infof("8. 模拟 commits 聚合查询（用户: %s，日期: %s）", UserId, dateStr)
	type commitAgg struct {
		UserId                   string
		CommitIds                string
		CommitDiffLines          int64
		CommitAncientMinutes     float64
		CommitRealAiMinutes      float64
		CommitRealAncientMinutes float64
		CommitRealMinutes        float64
	}
	var commitAggs []commitAgg
	db.Raw(`
		SELECT
			user_id,
			COALESCE(array_to_json(array_agg(commit_id)), '[]') as commit_ids,
			COALESCE(SUM(diff_lines), 0) as commit_diff_lines,
			COALESCE(SUM(COALESCE(commit_ancient_minutes_manual, commit_ancient_minutes)), 0) as commit_ancient_minutes,
			COALESCE(SUM(commit_real_ai_minutes), 0) as commit_real_ai_minutes,
			COALESCE(SUM(commit_real_non_ai_minutes), 0) as commit_real_ancient_minutes,
			COALESCE(SUM(COALESCE(commit_real_minutes_manual, commit_real_minutes)), 0) as commit_real_minutes
		FROM commits
		WHERE user_id = $1 AND DATE(commit_time) = $2
		GROUP BY user_id
	`, UserId, dateStr).Scan(&commitAggs)
	logx.Infof("   聚合结果数量: %d", len(commitAggs))
	for _, ca := range commitAggs {
		logx.Infof("   - UserId: %s", ca.UserId)
		logx.Infof("     CommitIds: %s", ca.CommitIds)
		logx.Infof("     CommitDiffLines: %d, CommitRealMinutes: %.2f", ca.CommitDiffLines, ca.CommitRealMinutes)
	}
	logx.Info("")

	// 9. 检查 user_productivity 表中的记录
	logx.Infof("9. user_productivity 表中的记录（用户: %s, 日期: %s）", UserId, dateStr)
	type userProd struct {
		UserProductivityId string
		UserId             string
		CreateTime         string
		TaskIds            string
		CommitIds          string
		TaskDiffLines      int
		CommitDiffLines    int
		TaskRealMinutes    float64
		CommitRealMinutes  float64
	}
	var userProds []userProd
	db.Raw(`
		SELECT 
			user_productivity_id,
			user_id,
			TO_CHAR(create_time, 'YYYY-MM-DD HH24:MI:SS') as create_time,
			task_ids,
			commit_ids,
			task_diff_lines,
			commit_diff_lines,
			task_real_minutes,
			commit_real_minutes
		FROM user_productivity
		WHERE user_id = $1 AND TO_CHAR(create_time, 'YYYYMMDD') = $2
	`, UserId, dateStr).Scan(&userProds)
	logx.Infof("   找到 %d 条记录:", len(userProds))
	for _, up := range userProds {
		logx.Infof("   - ID: %s, CreateTime: %s", up.UserProductivityId, up.CreateTime)
		logx.Infof("     TaskIds: %s", up.TaskIds)
		logx.Infof("     CommitIds: %s", up.CommitIds)
	}
	logx.Info("")

	// 10. 检查该用户的所有日期分布
	logx.Info("10. 该用户的所有日期分布（tasks表）")
	type dateDist struct {
		UserId    string
		TaskCount int
		Earliest  string
		Latest    string
		AllDates  string
	}
	var dateDists []dateDist
	db.Raw(`
		SELECT 
			user_id,
			COUNT(*) as task_count,
			TO_CHAR(MIN(start_time), 'YYYY-MM-DD HH24:MI:SS') as earliest,
			TO_CHAR(MAX(start_time), 'YYYY-MM-DD HH24:MI:SS') as latest,
			STRING_AGG(DISTINCT TO_CHAR(DATE(start_time), 'YYYYMMDD'), ', ') as all_dates
		FROM tasks 
		WHERE user_id = $1
		GROUP BY user_id
	`, UserId).Scan(&dateDists)
	for _, dd := range dateDists {
		logx.Infof("   - UserId: %s", dd.UserId)
		logx.Infof("     TaskCount: %d", dd.TaskCount)
		logx.Infof("     最早: %s", dd.Earliest)
		logx.Infof("     最晚: %s", dd.Latest)
		logx.Infof("     所有日期: %s", dd.AllDates)
	}
	logx.Info("")

	// 诊断结论
	logx.Info("===== 诊断结论 =====")
	if len(tasksOnDate) == 0 && len(commitsOnDate) == 0 {
		logx.Infof("❌ 问题找到：用户 %s 在日期 %s 的 tasks 和 commits 表中都没有数据！", UserId, dateStr)
		if len(tasks) > 0 || len(commits) > 0 {
			logx.Info("   但是该用户在其他日期有数据，可能是日期不匹配。")
			logx.Info("   可能原因：")
			logx.Info("   1. 时区问题：数据中的时间戳在数据库时区下属于不同的日期")
			logx.Info("   2. 日期格式问题：指定日期格式不正确")
			logx.Info("   3. 数据确实不在该日期")
		}
	} else if len(taskAggs) == 0 && len(tasksOnDate) > 0 {
		logx.Info("❌ 问题找到：tasks 表中有数据，但聚合查询返回空！")
		logx.Info("   可能原因：")
		logx.Info("   1. user_id 在数据中有空格或特殊字符")
		logx.Info("   2. user_id 为 NULL 或空字符串")
		logx.Info("   3. DATE(start_time) 函数返回的日期不匹配")
	} else if len(commitAggs) == 0 && len(commitsOnDate) > 0 {
		logx.Info("❌ 问题找到：commits 表中有数据，但聚合查询返回空！")
		logx.Info("   可能原因：")
		logx.Info("   1. user_id 在数据中有空格或特殊字符")
		logx.Info("   2. user_id 为 NULL 或空字符串")
		logx.Info("   3. DATE(commit_time) 函数返回的日期不匹配")
	} else {
		logx.Infof("✓ 数据正常：用户 %s 在日期 %s 有数据，聚合查询也返回了结果。", UserId, dateStr)
		if len(userProds) == 0 {
			logx.Info("   但是 user_productivity 表中没有记录，可能 efficiency 命令还没有运行。")
		}
	}

	return nil
}

func init() {
	diagnoseCmd.Flags().SortFlags = false
	diagnoseCmd.Flags().String("user-id", "", "要诊断的用户ID")
	diagnoseCmd.Flags().String("date", "", "要诊断的日期，格式为 YYYYMMDD")
	rootCmd.AddCommand(diagnoseCmd)
}
