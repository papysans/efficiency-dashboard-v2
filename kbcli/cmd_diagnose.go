package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var diagnoseCmd = &cobra.Command{
	Use:   "diagnose",
	Short: "诊断 user_productivity 表中数据为空的原因",
	Long:  "诊断指定用户在指定日期的 tasks 和 commits 数据，找出为什么 task_ids, commit_ids, work_dir_ids 为空",
	RunE: func(cmd *cobra.Command, args []string) error {
		userID, _ := cmd.Flags().GetString("user-id")
		dateStr, _ := cmd.Flags().GetString("date")

		// 如果不指定 user-id 和 date，则执行 find_users.sql 中的逻辑
		if userID == "" && dateStr == "" {
			return runFindUsers()
		}

		// 如果指定了 user-id 或 date 中的任意一个，则要求两个都必须指定
		if userID == "" || dateStr == "" {
			return fmt.Errorf("--user-id 和 --date 必须同时指定或同时不指定")
		}

		return runDiagnose(userID, dateStr)
	},
}

func runFindUsers() error {
	db, err := openGormDB(cfg.StatDatabase.DSN())
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	logInfo("===== 查找数据库中实际存在的 user_id 和日期 =====")

	// 1. tasks 表中唯一的 user_id（前20个）
	logInfo("1. tasks 表中唯一的 user_id（前20个）")
	type taskUser struct {
		UserID       string
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
	logInfof("   找到 %d 个用户:", len(taskUsers))
	for _, tu := range taskUsers {
		logInfof("   - UserID: %s, TaskCount: %d, 日期范围: %s ~ %s",
			tu.UserID, tu.TaskCount, tu.EarliestDate, tu.LatestDate)
	}
	logInfo("")

	// 2. commits 表中唯一的 user_id（前20个）
	logInfo("2. commits 表中唯一的 user_id（前20个）")
	type commitUser struct {
		UserID       string
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
	logInfof("   找到 %d 个用户:", len(commitUsers))
	for _, cu := range commitUsers {
		logInfof("   - UserID: %s, CommitCount: %d, 日期范围: %s ~ %s",
			cu.UserID, cu.CommitCount, cu.EarliestDate, cu.LatestDate)
	}
	logInfo("")

	// 3. 最新的 tasks 和 commits 日期
	logInfo("3. 最新的 tasks 和 commits 日期")
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
		logInfof("   - 表: %s, 最新日期: %s (%s), 总记录数: %d",
			td.TableName, td.LatestDateStr, td.LatestDate, td.TotalCount)
	}
	logInfo("")

	// 4. 建议诊断参数
	logInfo("4. 建议诊断参数")
	if len(taskUsers) > 0 {
		suggestedUserID := taskUsers[0].UserID
		var latestDate string
		db.Raw(`
			SELECT TO_CHAR(DATE(MAX(start_time)), 'YYYYMMDD') as date_str
			FROM tasks
		`).Scan(&latestDate)
		logInfo("   建议使用的诊断命令:")
		logInfof("   kbcli diagnose --user-id %s --date %s", suggestedUserID, latestDate)
	}
	logInfo("")

	logInfo("===== 查询完成 =====")
	return nil
}

func runDiagnose(userID, dateStr string) error {
	db, err := openGormDB(cfg.StatDatabase.DSN())
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	logInfof("===== 诊断用户: %s, 日期: %s =====", userID, dateStr)

	// 1. 检查数据库时区
	logInfo("1. 检查数据库时区设置")
	var timezone string
	db.Raw("SHOW timezone").Scan(&timezone)
	logInfof("   数据库时区: %s", timezone)

	// 2. 检查 user_id 字段
	logInfo("2. user_id 字段检查")
	type userIDCheck struct {
		UserID         string
		UserIDLength   int
		IsTrimmed      bool
		FirstCharASCII int
	}
	var uidCheck userIDCheck
	db.Raw(`
		SELECT 
			user_id,
			LENGTH(user_id) as user_id_length,
			user_id = TRIM(user_id) as is_trimmed,
			ASCII(SUBSTRING(user_id, 1, 1)) as first_char_ascii
		FROM tasks 
		WHERE user_id = $1
		LIMIT 1
	`, userID).Scan(&uidCheck)
	logInfof("   UserID: %s", uidCheck.UserID)
	logInfof("   长度: %d", uidCheck.UserIDLength)
	logInfof("   是否已去除首尾空格: %t", uidCheck.IsTrimmed)
	logInfof("   第一个字符ASCII码: %d", uidCheck.FirstCharASCII)

	// 2. 检查 tasks 表中该用户的所有记录
	logInfof("3. tasks 表中用户 %s 的所有记录（不限日期）", userID)
	type taskInfo struct {
		UserID        string
		TaskID        string
		StartTime     string
		StartDate     string
		DateStrFormat string
		WorkDirID     string
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
	`, userID).Scan(&tasks)
	logInfof("   找到 %d 条记录:", len(tasks))
	for _, t := range tasks {
		logInfof("   - TaskID: %s, StartTime: %s, WorkDirID: %s", t.TaskID, t.StartTime, t.WorkDirID)
	}
	logInfo("")

	// 3. 检查 tasks 表中该用户在指定日期的记录
	logInfof("4. tasks 表中用户 %s 在指定日期 %s 的记录", userID, dateStr)
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
	`, userID, dateStr).Scan(&tasksOnDate)
	logInfof("   找到 %d 条记录:", len(tasksOnDate))
	for _, t := range tasksOnDate {
		logInfof("   - TaskID: %s, StartTime: %s, WorkDirID: %s", t.TaskID, t.StartTime, t.WorkDirID)
	}
	logInfo("")

	// 5. 模拟 tasks 聚合查询
	logInfof("5. 模拟 tasks 聚合查询（用户: %s，日期: %s）", userID, dateStr)
	type taskAgg struct {
		UserID             string
		TaskIDs            string
		WorkDirIDs         string
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
			COALESCE(array_to_json(array_agg(DISTINCT work_dir_id) FILTER (WHERE work_dir_id IS NOT NULL AND work_dir_id != '')), '[]') as work_dir_ids,
			COALESCE(SUM(diff_lines), 0) as task_diff_lines,
			COALESCE(SUM(upstream_tokens), 0) as upstream_tokens,
			COALESCE(SUM(downstream_tokens), 0) as downstream_tokens,
			COALESCE(SUM(cost), 0) as cost,
			COALESCE(SUM(COALESCE(task_real_minutes_manual, task_real_minutes)), 0) as task_real_minutes,
			COALESCE(SUM(COALESCE(task_ancient_minutes_manual, task_ancient_minutes)), 0) as task_ancient_minutes
		FROM tasks
		WHERE user_id  = $1 AND DATE(start_time) = $2
		GROUP BY user_id
	`, userID, dateStr).Scan(&taskAggs)
	logInfof("   聚合结果数量: %d", len(taskAggs))
	for _, ta := range taskAggs {
		logInfof("   - UserID: %s", ta.UserID)
		logInfof("     TaskIDs: %s", ta.TaskIDs)
		logInfof("     WorkDirIDs: %s", ta.WorkDirIDs)
		logInfof("     TaskDiffLines: %d, TaskRealMinutes: %.2f", ta.TaskDiffLines, ta.TaskRealMinutes)
	}
	logInfo("")

	// 6. 检查 commits 表中该用户的所有记录
	logInfof("6. commits 表中用户 %s 的所有记录（不限日期）", userID)
	type commitInfo struct {
		UserID        string
		CommitID      string
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
	`, userID).Scan(&commits)
	logInfof("   找到 %d 条记录:", len(commits))
	for _, c := range commits {
		logInfof("   - CommitID: %s, CommitTime: %s", c.CommitID, c.CommitTime)
	}
	logInfo("")

	// 7. 检查 commits 表中该用户在指定日期的记录
	logInfof("7. commits 表中用户 %s 在指定日期 %s 的记录", userID, dateStr)
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
	`, userID, dateStr).Scan(&commitsOnDate)
	logInfof("   找到 %d 条记录:", len(commitsOnDate))
	for _, c := range commitsOnDate {
		logInfof("   - CommitID: %s, CommitTime: %s", c.CommitID, c.CommitTime)
	}
	logInfo("")

	// 8. 模拟 commits 聚合查询
	logInfof("8. 模拟 commits 聚合查询（用户: %s，日期: %s）", userID, dateStr)
	type commitAgg struct {
		UserID                   string
		CommitIDs                string
		CommitDiffLines          int64
		CommitAncientMinutes     float64
		CommitRealAIMinutes      float64
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
			COALESCE(SUM(commit_real_ancient_minutes), 0) as commit_real_ancient_minutes,
			COALESCE(SUM(COALESCE(commit_real_minutes_manual, commit_real_minutes)), 0) as commit_real_minutes
		FROM commits
		WHERE user_id = $1 AND DATE(commit_time) = $2
		GROUP BY user_id
	`, userID, dateStr).Scan(&commitAggs)
	logInfof("   聚合结果数量: %d", len(commitAggs))
	for _, ca := range commitAggs {
		logInfof("   - UserID: %s", ca.UserID)
		logInfof("     CommitIDs: %s", ca.CommitIDs)
		logInfof("     CommitDiffLines: %d, CommitRealMinutes: %.2f", ca.CommitDiffLines, ca.CommitRealMinutes)
	}
	logInfo("")

	// 9. 检查 user_productivity 表中的记录
	logInfof("9. user_productivity 表中的记录（用户: %s, 日期: %s）", userID, dateStr)
	type userProd struct {
		UserProductivityID string
		UserID             string
		CreateTime         string
		TaskIDs            string
		WorkDirIDs         string
		CommitIDs          string
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
			work_dir_ids,
			commit_ids,
			task_diff_lines,
			commit_diff_lines,
			task_real_minutes,
			commit_real_minutes
		FROM user_productivity
		WHERE user_id = $1 AND TO_CHAR(create_time, 'YYYYMMDD') = $2
	`, userID, dateStr).Scan(&userProds)
	logInfof("   找到 %d 条记录:", len(userProds))
	for _, up := range userProds {
		logInfof("   - ID: %s, CreateTime: %s", up.UserProductivityID, up.CreateTime)
		logInfof("     TaskIDs: %s", up.TaskIDs)
		logInfof("     WorkDirIDs: %s", up.WorkDirIDs)
		logInfof("     CommitIDs: %s", up.CommitIDs)
	}
	logInfo("")

	// 10. 检查该用户的所有日期分布
	logInfo("10. 该用户的所有日期分布（tasks表）")
	type dateDist struct {
		UserID    string
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
	`, userID).Scan(&dateDists)
	for _, dd := range dateDists {
		logInfof("   - UserID: %s", dd.UserID)
		logInfof("     TaskCount: %d", dd.TaskCount)
		logInfof("     最早: %s", dd.Earliest)
		logInfof("     最晚: %s", dd.Latest)
		logInfof("     所有日期: %s", dd.AllDates)
	}
	logInfo("")

	// 诊断结论
	logInfo("===== 诊断结论 =====")
	if len(tasksOnDate) == 0 && len(commitsOnDate) == 0 {
		logInfof("❌ 问题找到：用户 %s 在日期 %s 的 tasks 和 commits 表中都没有数据！", userID, dateStr)
		if len(tasks) > 0 || len(commits) > 0 {
			logInfo("   但是该用户在其他日期有数据，可能是日期不匹配。")
			logInfo("   可能原因：")
			logInfo("   1. 时区问题：数据中的时间戳在数据库时区下属于不同的日期")
			logInfo("   2. 日期格式问题：指定日期格式不正确")
			logInfo("   3. 数据确实不在该日期")
		}
	} else if len(taskAggs) == 0 && len(tasksOnDate) > 0 {
		logInfo("❌ 问题找到：tasks 表中有数据，但聚合查询返回空！")
		logInfo("   可能原因：")
		logInfo("   1. user_id 在数据中有空格或特殊字符")
		logInfo("   2. user_id 为 NULL 或空字符串")
		logInfo("   3. DATE(start_time) 函数返回的日期不匹配")
	} else if len(commitAggs) == 0 && len(commitsOnDate) > 0 {
		logInfo("❌ 问题找到：commits 表中有数据，但聚合查询返回空！")
		logInfo("   可能原因：")
		logInfo("   1. user_id 在数据中有空格或特殊字符")
		logInfo("   2. user_id 为 NULL 或空字符串")
		logInfo("   3. DATE(commit_time) 函数返回的日期不匹配")
	} else {
		logInfof("✓ 数据正常：用户 %s 在日期 %s 有数据，聚合查询也返回了结果。", userID, dateStr)
		if len(userProds) == 0 {
			logInfo("   但是 user_productivity 表中没有记录，可能 efficiency 命令还没有运行。")
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
