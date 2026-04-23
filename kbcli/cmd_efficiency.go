package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"github.com/lib/pq"

	"github.com/spf13/cobra"
)

type userTaskAgg struct {
	UserID             string
	TaskIDs            []string
	WorkDirIDs         []string
	TaskDiffLines      int64
	UpstreamTokens     int64
	DownstreamTokens   int64
	Cost               float64
	TaskRealMinutes    float64
	TaskAncientMinutes float64
}

type userCommitAgg struct {
	UserID                   string
	CommitIDs                []string
	CommitDiffLines          int64
	CommitAncientMinutes     float64
	CommitRealAIMinutes      float64
	CommitRealAncientMinutes float64
	CommitRealMinutes        float64
}

var efficiencyCmd = &cobra.Command{
	Use:   "efficiency",
	Short: "按日计算用户和组织效能数据",
	Long:  "根据已导入的task、commit、user_org数据，按日计算各用户和组织（分级）的生产力数据，写入user_productivity和org_productivity表。如有--date参数，则只处理该日期的数据，否则处理所有日期数据",
	RunE: func(cmd *cobra.Command, args []string) error {
		dateStr, _ := cmd.Flags().GetString("date")
		if dateStr != "" && len(dateStr) != 8 {
			return fmt.Errorf("--date 格式应为 YYYYMMDD，当前: %s", dateStr)
		}

		db, err := sql.Open("postgres", cfg.StatDatabase.DSN())
		if err != nil {
			return fmt.Errorf("连接数据库失败: %w", err)
		}
		defer db.Close()

		if err := db.Ping(); err != nil {
			return fmt.Errorf("数据库连接测试失败: %w", err)
		}

		if err := ensureEfficiencyTables(db); err != nil {
			return fmt.Errorf("确保表存在失败: %w", err)
		}

		var dates []string
		if dateStr != "" {
			dates = []string{dateStr}
		} else {
			dates, err = getAllDates(db)
			if err != nil {
				return fmt.Errorf("获取日期列表失败: %w", err)
			}
			if len(dates) == 0 {
				fmt.Println("没有找到任何task或commit数据")
				return nil
			}
			fmt.Printf("共发现 %d 个日期需要处理\n", len(dates))
		}

		totalUserCount := 0
		totalOrgCount := 0
		for _, d := range dates {
			fmt.Printf("\n=== 处理日期: %s ===\n", d)
			userCount, err := calculateUserProductivity(db, d)
			if err != nil {
				return fmt.Errorf("计算用户生产力失败 [date=%s]: %w", d, err)
			}
			fmt.Printf("用户生产力计算完成: %d 条记录 (日期=%s)\n", userCount, d)
			totalUserCount += userCount

			orgCount, err := calculateOrgProductivity(db, d)
			if err != nil {
				return fmt.Errorf("计算组织生产力失败 [date=%s]: %w", d, err)
			}
			fmt.Printf("组织生产力计算完成: %d 条记录 (日期=%s)\n", orgCount, d)
			totalOrgCount += orgCount
		}

		fmt.Printf("\n全部完成: 用户 %d 条, 组织 %d 条\n", totalUserCount, totalOrgCount)
		return nil
	},
}

func init() {
	efficiencyCmd.Flags().String("date", "", "聚合日期，格式YYYYMMDD，不指定则处理所有日期")
	rootCmd.AddCommand(efficiencyCmd)
}

func getAllDates(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`
		SELECT DISTINCT dt FROM (
			SELECT TO_CHAR(DATE(start_time), 'YYYYMMDD') AS dt FROM tasks WHERE start_time IS NOT NULL
			UNION
			SELECT TO_CHAR(DATE(commit_time), 'YYYYMMDD') AS dt FROM commits WHERE commit_time IS NOT NULL
		) sub
		ORDER BY dt
	`)
	if err != nil {
		return nil, fmt.Errorf("查询日期列表失败: %w", err)
	}
	defer rows.Close()

	var dates []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, fmt.Errorf("读取日期失败: %w", err)
		}
		dates = append(dates, d)
	}
	return dates, rows.Err()
}

func ensureEfficiencyTables(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS user_productivity (
		user_productivity_id VARCHAR(100) PRIMARY KEY,
		create_time TIMESTAMPTZ,
		user_id VARCHAR(100),
		user_name VARCHAR(500),
		task_ids JSONB,
		work_dir_ids JSONB,
		task_diff_lines INT,
		upstream_tokens BIGINT,
		downstream_tokens BIGINT,
		cost FLOAT8,
		task_real_minutes FLOAT8,
		task_ancient_minutes FLOAT8,
		task_efficiency_ratio FLOAT8,
		commit_ids JSONB,
		commit_diff_lines INT,
		commit_ancient_minutes FLOAT8,
		commit_real_ai_minutes FLOAT8,
		commit_real_ancient_minutes FLOAT8,
		commit_real_minutes FLOAT8,
		commit_efficiency_ratio FLOAT8,
		updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return fmt.Errorf("创建user_productivity表失败: %w", err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS org_productivity (
			org_id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
			org_name VARCHAR(500),
			org_level INT DEFAULT 1,
			create_time TIMESTAMPTZ,
			user_ids JSONB,
			user_names JSONB,
			task_diff_lines INT,
			upstream_tokens BIGINT,
			downstream_tokens BIGINT,
			cost FLOAT8,
			task_real_minutes FLOAT8,
			task_ancient_minutes FLOAT8,
			task_efficiency_ratio FLOAT8,
			commit_diff_lines INT,
			commit_ancient_minutes FLOAT8,
			commit_real_ai_minutes FLOAT8,
			commit_real_ancient_minutes FLOAT8,
			commit_real_minutes FLOAT8,
			commit_efficiency_ratio FLOAT8,
			user_count INT DEFAULT 0,
			created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
		)`)
	if err != nil {
		return fmt.Errorf("创建org_productivity表失败: %w", err)
	}

	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_org_productivity_org_name ON org_productivity(org_name)`,
		`CREATE INDEX IF NOT EXISTS idx_org_productivity_org_level ON org_productivity(org_level)`,
		`CREATE INDEX IF NOT EXISTS idx_org_productivity_create_time ON org_productivity(create_time)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uniq_org_productivity_name_time ON org_productivity(org_name, create_time)`,
	}
	for _, idx := range indexes {
		if _, err := db.Exec(idx); err != nil {
			fmt.Fprintf(os.Stderr, "警告: 创建索引失败(可忽略): %v\n", err)
		}
	}

	return nil
}

func calculateUserProductivity(db *sql.DB, dateStr string) (int, error) {
	userNameMap, err := loadUserNames(db)
	if err != nil {
		return 0, fmt.Errorf("加载用户名称失败: %w", err)
	}

	taskUserNameMap, err := loadUserNamesFromTasks(db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "警告: 加载task用户名失败: %v\n", err)
	}
	commitUserNameMap, err := loadUserNamesFromCommits(db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "警告: 加载commit用户名失败: %v\n", err)
	}

	taskAggMap, err := aggregateTasksByUser(db, dateStr)
	if err != nil {
		return 0, fmt.Errorf("聚合task数据失败: %w", err)
	}
	fmt.Printf("聚合task数据: %d 个用户\n", len(taskAggMap))

	commitAggMap, err := aggregateCommitsByUser(db, dateStr)
	if err != nil {
		return 0, fmt.Errorf("聚合commit数据失败: %w", err)
	}
	fmt.Printf("聚合commit数据: %d 个用户\n", len(commitAggMap))

	allUserIDs := make(map[string]bool)
	for uid := range taskAggMap {
		allUserIDs[uid] = true
	}
	for uid := range commitAggMap {
		allUserIDs[uid] = true
	}

	if len(allUserIDs) == 0 {
		fmt.Println("没有找到有task或commit数据的用户")
		return 0, nil
	}

	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("开启事务失败: %w", err)
	}

	stmt, err := tx.Prepare(`
		INSERT INTO user_productivity (
			user_productivity_id, create_time, user_id, user_name,
			task_ids, work_dir_ids, task_diff_lines,
			upstream_tokens, downstream_tokens, cost,
			task_real_minutes, task_ancient_minutes, task_efficiency_ratio,
			commit_ids, commit_diff_lines, commit_ancient_minutes,
			commit_real_ai_minutes, commit_real_ancient_minutes, commit_real_minutes,
			commit_efficiency_ratio,
			updated_at
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7,
			$8, $9, $10,
			$11, $12, $13,
			$14, $15, $16,
			$17, $18, $19,
			$20,
			CURRENT_TIMESTAMP
		) ON CONFLICT (user_productivity_id) DO UPDATE SET
			user_name = EXCLUDED.user_name,
			task_ids = EXCLUDED.task_ids,
			work_dir_ids = EXCLUDED.work_dir_ids,
			task_diff_lines = EXCLUDED.task_diff_lines,
			upstream_tokens = EXCLUDED.upstream_tokens,
			downstream_tokens = EXCLUDED.downstream_tokens,
			cost = EXCLUDED.cost,
			task_real_minutes = EXCLUDED.task_real_minutes,
			task_ancient_minutes = EXCLUDED.task_ancient_minutes,
			task_efficiency_ratio = EXCLUDED.task_efficiency_ratio,
			commit_ids = EXCLUDED.commit_ids,
			commit_diff_lines = EXCLUDED.commit_diff_lines,
			commit_ancient_minutes = EXCLUDED.commit_ancient_minutes,
			commit_real_ai_minutes = EXCLUDED.commit_real_ai_minutes,
			commit_real_ancient_minutes = EXCLUDED.commit_real_ancient_minutes,
			commit_real_minutes = EXCLUDED.commit_real_minutes,
			commit_efficiency_ratio = EXCLUDED.commit_efficiency_ratio,
			updated_at = CURRENT_TIMESTAMP
	`)
	if err != nil {
		tx.Rollback()
		return 0, fmt.Errorf("预处理语句失败: %w", err)
	}
	defer stmt.Close()

	count := 0
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

		var taskIDs, workDirIDs, commitIDs []string
		var taskDiffLines, upstreamTokens, downstreamTokens int64
		var cost, taskRealMinutes, taskAncientMinutes float64
		var commitDiffLines int64
		var commitAncientMinutes, commitRealAIMinutes, commitRealAncientMinutes, commitRealMinutes float64

		if ta != nil {
			taskIDs = ta.TaskIDs
			workDirIDs = ta.WorkDirIDs
			taskDiffLines = ta.TaskDiffLines
			upstreamTokens = ta.UpstreamTokens
			downstreamTokens = ta.DownstreamTokens
			cost = ta.Cost
			taskRealMinutes = ta.TaskRealMinutes
			taskAncientMinutes = ta.TaskAncientMinutes
		}
		if ca != nil {
			commitIDs = ca.CommitIDs
			commitDiffLines = ca.CommitDiffLines
			commitAncientMinutes = ca.CommitAncientMinutes
			commitRealAIMinutes = ca.CommitRealAIMinutes
			commitRealAncientMinutes = ca.CommitRealAncientMinutes
			commitRealMinutes = ca.CommitRealMinutes
		}

		taskEffRatio := calcEfficiencyRatio(taskAncientMinutes, taskRealMinutes)
		commitEffRatio := calcEfficiencyRatio(commitAncientMinutes, commitRealMinutes)

		taskIDsJSON, _ := json.Marshal(defaultSlice(taskIDs))
		workDirIDsJSON, _ := json.Marshal(defaultSlice(workDirIDs))
		commitIDsJSON, _ := json.Marshal(defaultSlice(commitIDs))

		createTime, _ := time.Parse("20060102", dateStr)
		// 设置为当天的起始时间
		createTime = time.Date(createTime.Year(), createTime.Month(), createTime.Day(), 0, 0, 0, 0, time.UTC)

		upID := uid + "_" + dateStr

		_, err := stmt.Exec(
			upID, createTime, uid, userName,
			string(taskIDsJSON), string(workDirIDsJSON), taskDiffLines,
			upstreamTokens, downstreamTokens, cost,
			taskRealMinutes, taskAncientMinutes, taskEffRatio,
			string(commitIDsJSON), commitDiffLines, commitAncientMinutes,
			commitRealAIMinutes, commitRealAncientMinutes, commitRealMinutes,
			commitEffRatio,
		)
		if err != nil {
			tx.Rollback()
			return 0, fmt.Errorf("写入user_productivity失败 [user_id=%s]: %w", uid, err)
		}
		count++
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("提交事务失败: %w", err)
	}

	return count, nil
}

func calculateOrgProductivity(db *sql.DB, dateStr string) (int, error) {
	orgTree, err := buildOrgTree(db)
	if err != nil {
		return 0, fmt.Errorf("构建组织树失败: %w", err)
	}

	if len(orgTree) == 0 {
		fmt.Println("没有组织数据，跳过组织生产力计算")
		return 0, nil
	}

	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("开启事务失败: %w", err)
	}

	stmt, err := tx.Prepare(`
		INSERT INTO org_productivity (
			org_name, org_level, create_time,
			user_ids, user_names,
			task_diff_lines, upstream_tokens, downstream_tokens, cost,
			task_real_minutes, task_ancient_minutes, task_efficiency_ratio,
			commit_diff_lines, commit_ancient_minutes,
			commit_real_ai_minutes, commit_real_ancient_minutes, commit_real_minutes,
			commit_efficiency_ratio, user_count,
			updated_at
		) VALUES (
			$1, $2, $3,
			$4, $5,
			$6, $7, $8, $9,
			$10, $11, $12,
			$13, $14,
			$15, $16, $17,
			$18, $19,
			CURRENT_TIMESTAMP
		) ON CONFLICT (org_name, create_time) DO UPDATE SET
			org_level = EXCLUDED.org_level,
			user_ids = EXCLUDED.user_ids,
			user_names = EXCLUDED.user_names,
			task_diff_lines = EXCLUDED.task_diff_lines,
			upstream_tokens = EXCLUDED.upstream_tokens,
			downstream_tokens = EXCLUDED.downstream_tokens,
			cost = EXCLUDED.cost,
			task_real_minutes = EXCLUDED.task_real_minutes,
			task_ancient_minutes = EXCLUDED.task_ancient_minutes,
			task_efficiency_ratio = EXCLUDED.task_efficiency_ratio,
			commit_diff_lines = EXCLUDED.commit_diff_lines,
			commit_ancient_minutes = EXCLUDED.commit_ancient_minutes,
			commit_real_ai_minutes = EXCLUDED.commit_real_ai_minutes,
			commit_real_ancient_minutes = EXCLUDED.commit_real_ancient_minutes,
			commit_real_minutes = EXCLUDED.commit_real_minutes,
			commit_efficiency_ratio = EXCLUDED.commit_efficiency_ratio,
			user_count = EXCLUDED.user_count,
			updated_at = CURRENT_TIMESTAMP
	`)
	if err != nil {
		tx.Rollback()
		return 0, fmt.Errorf("预处理语句失败: %w", err)
	}
	defer stmt.Close()

	createTime, _ := time.Parse("20060102", dateStr)
	// 设置为当天的起始时间
	createTime = time.Date(createTime.Year(), createTime.Month(), createTime.Day(), 0, 0, 0, 0, time.UTC)

	count := 0
	for _, node := range orgTree {
		if len(node.UserIDs) == 0 {
			continue
		}

		var agg struct {
			TaskDiffLines            int64
			UpstreamTokens           int64
			DownstreamTokens         int64
			Cost                     float64
			TaskRealMinutes          float64
			TaskAncientMinutes       float64
			CommitDiffLines          int64
			CommitAncientMinutes     float64
			CommitRealAIMinutes      float64
			CommitRealAncientMinutes float64
			CommitRealMinutes        float64
		}

		err := db.QueryRow(`
			SELECT
				COALESCE(SUM(task_diff_lines), 0),
				COALESCE(SUM(upstream_tokens), 0),
				COALESCE(SUM(downstream_tokens), 0),
				COALESCE(SUM(cost), 0),
				COALESCE(SUM(task_real_minutes), 0),
				COALESCE(SUM(task_ancient_minutes), 0),
				COALESCE(SUM(commit_diff_lines), 0),
				COALESCE(SUM(commit_ancient_minutes), 0),
				COALESCE(SUM(commit_real_ai_minutes), 0),
				COALESCE(SUM(commit_real_ancient_minutes), 0),
				COALESCE(SUM(commit_real_minutes), 0)
			FROM user_productivity
				WHERE user_id = ANY($1) AND create_time = $2
			`, pq.Array(node.UserIDs), createTime).Scan(
			&agg.TaskDiffLines, &agg.UpstreamTokens, &agg.DownstreamTokens, &agg.Cost,
			&agg.TaskRealMinutes, &agg.TaskAncientMinutes,
			&agg.CommitDiffLines, &agg.CommitAncientMinutes,
			&agg.CommitRealAIMinutes, &agg.CommitRealAncientMinutes, &agg.CommitRealMinutes,
		)
		if err != nil {
			tx.Rollback()
			return 0, fmt.Errorf("查询组织聚合数据失败 [%s]: %w", node.OrgName, err)
		}

		taskEffRatio := calcEfficiencyRatio(agg.TaskAncientMinutes, agg.TaskRealMinutes)
		commitEffRatio := calcEfficiencyRatio(agg.CommitAncientMinutes, agg.CommitRealMinutes)

		userIDsJSON, _ := json.Marshal(node.UserIDs)
		userNamesJSON, _ := json.Marshal(node.UserNames)

		_, err = stmt.Exec(
			node.OrgName, node.Level, createTime,
			string(userIDsJSON), string(userNamesJSON),
			agg.TaskDiffLines, agg.UpstreamTokens, agg.DownstreamTokens, agg.Cost,
			agg.TaskRealMinutes, agg.TaskAncientMinutes, taskEffRatio,
			agg.CommitDiffLines, agg.CommitAncientMinutes,
			agg.CommitRealAIMinutes, agg.CommitRealAncientMinutes, agg.CommitRealMinutes,
			commitEffRatio, len(node.UserIDs),
		)
		if err != nil {
			tx.Rollback()
			return 0, fmt.Errorf("写入org_productivity失败 [%s]: %w", node.OrgName, err)
		}

		count++
		fmt.Printf("  组织 [%s] (L%d): %d个用户, task_eff=%.2f, commit_eff=%.2f\n",
			node.OrgName, node.Level, len(node.UserIDs), taskEffRatio, commitEffRatio)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("提交事务失败: %w", err)
	}

	return count, nil
}

type orgNode struct {
	OrgName   string
	Level     int
	UserIDs   []string
	UserNames []string
}

func buildOrgTree(db *sql.DB) ([]*orgNode, error) {
	rows, err := db.Query(`SELECT user_id, user_name, org1, org2, org3, org4, org5, org6, org7, org8, org9 FROM user_org WHERE org1 IS NOT NULL AND org1 != ''`)
	if err != nil {
		return nil, fmt.Errorf("查询user_org失败: %w", err)
	}
	defer rows.Close()

	type userOrgInfo struct {
		UserID   string
		UserName string
		Orgs     []string
	}

	var userOrgs []userOrgInfo
	for rows.Next() {
		var uid, uname string
		var orgs [9]sql.NullString
		if err := rows.Scan(&uid, &uname, &orgs[0], &orgs[1], &orgs[2], &orgs[3], &orgs[4], &orgs[5], &orgs[6], &orgs[7], &orgs[8]); err != nil {
			return nil, fmt.Errorf("读取user_org数据失败: %w", err)
		}
		var nonEmptyOrgs []string
		for _, o := range orgs {
			if o.Valid && o.String != "" {
				nonEmptyOrgs = append(nonEmptyOrgs, o.String)
			} else {
				break
			}
		}
		if len(nonEmptyOrgs) > 0 {
			userOrgs = append(userOrgs, userOrgInfo{UserID: uid, UserName: uname, Orgs: nonEmptyOrgs})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历user_org数据失败: %w", err)
	}

	orgMap := make(map[string]*orgNode)
	for _, uo := range userOrgs {
		for level := 1; level <= len(uo.Orgs); level++ {
			orgPath := strings.Join(uo.Orgs[:level], "/")
			if _, ok := orgMap[orgPath]; !ok {
				orgMap[orgPath] = &orgNode{
					OrgName: orgPath,
					Level:   level,
				}
			}
			node := orgMap[orgPath]
			node.UserIDs = append(node.UserIDs, uo.UserID)
			node.UserNames = append(node.UserNames, uo.UserName)
		}
	}

	for _, node := range orgMap {
		node.UserIDs = uniqueStrings(node.UserIDs)
		node.UserNames = uniqueStrings(node.UserNames)
	}

	var result []*orgNode
	for _, node := range orgMap {
		result = append(result, node)
	}
	return result, nil
}

func loadUserNames(db *sql.DB) (map[string]string, error) {
	result := make(map[string]string)
	rows, err := db.Query(`SELECT user_id, user_name FROM user_org WHERE user_name IS NOT NULL AND user_name != ''`)
	if err != nil {
		return nil, fmt.Errorf("查询user_org用户名失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var uid, uname string
		if err := rows.Scan(&uid, &uname); err != nil {
			return nil, fmt.Errorf("读取用户名失败: %w", err)
		}
		result[uid] = uname
	}
	return result, rows.Err()
}

func aggregateTasksByUser(db *sql.DB, dateStr string) (map[string]*userTaskAgg, error) {
	rows, err := db.Query(`
		SELECT
			user_id,
			COALESCE(array_agg(task_id), '{}'),
			COALESCE(array_agg(DISTINCT work_dir_id) FILTER (WHERE work_dir_id IS NOT NULL AND work_dir_id != ''), '{}'),
			COALESCE(SUM(diff_lines), 0),
			COALESCE(SUM(upstream_tokens), 0),
			COALESCE(SUM(downstream_tokens), 0),
			COALESCE(SUM(cost), 0),
			COALESCE(SUM(COALESCE(task_real_minutes_manual, task_real_minutes)), 0),
			COALESCE(SUM(COALESCE(task_ancient_minutes_manual, task_ancient_minutes)), 0)
		FROM tasks
		WHERE user_id IS NOT NULL AND user_id != '' AND DATE(start_time) = $1
		GROUP BY user_id
	`, dateStr)
	if err != nil {
		return nil, fmt.Errorf("查询task聚合数据失败: %w", err)
	}
	defer rows.Close()

	result := make(map[string]*userTaskAgg)
	for rows.Next() {
		var agg userTaskAgg
		if err := rows.Scan(
			&agg.UserID, pq.Array(&agg.TaskIDs), pq.Array(&agg.WorkDirIDs),
			&agg.TaskDiffLines, &agg.UpstreamTokens, &agg.DownstreamTokens,
			&agg.Cost, &agg.TaskRealMinutes, &agg.TaskAncientMinutes,
		); err != nil {
			return nil, fmt.Errorf("读取task聚合数据失败: %w", err)
		}
		result[agg.UserID] = &agg
	}
	return result, rows.Err()
}

func aggregateCommitsByUser(db *sql.DB, dateStr string) (map[string]*userCommitAgg, error) {
	rows, err := db.Query(`
		SELECT
			user_id,
			COALESCE(array_agg(commit_id), '{}'),
			COALESCE(SUM(diff_lines), 0),
			COALESCE(SUM(COALESCE(commit_ancient_minutes_manual, commit_ancient_minutes)), 0),
			COALESCE(SUM(commit_real_ai_minutes), 0),
			COALESCE(SUM(commit_real_ancient_minutes), 0),
			COALESCE(SUM(COALESCE(commit_real_minutes_manual, commit_real_minutes)), 0)
		FROM commits
		WHERE user_id IS NOT NULL AND user_id != '' AND DATE(commit_time) = $1
		GROUP BY user_id
	`, dateStr)
	if err != nil {
		return nil, fmt.Errorf("查询commit聚合数据失败: %w", err)
	}
	defer rows.Close()

	result := make(map[string]*userCommitAgg)
	for rows.Next() {
		var agg userCommitAgg
		if err := rows.Scan(
			&agg.UserID, pq.Array(&agg.CommitIDs),
			&agg.CommitDiffLines, &agg.CommitAncientMinutes,
			&agg.CommitRealAIMinutes, &agg.CommitRealAncientMinutes,
			&agg.CommitRealMinutes,
		); err != nil {
			return nil, fmt.Errorf("读取commit聚合数据失败: %w", err)
		}
		result[agg.UserID] = &agg
	}
	return result, rows.Err()
}

func calcEfficiencyRatio(ancientMinutes, realMinutes float64) float64 {
	if ancientMinutes > 0 && realMinutes > 0 && !math.IsInf(realMinutes, 0) {
		return (ancientMinutes / realMinutes) * 100
	}
	return 0
}

func defaultSlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func uniqueStrings(s []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}

func loadUserNamesFromTasks(db *sql.DB) (map[string]string, error) {
	result := make(map[string]string)
	rows, err := db.Query(`SELECT DISTINCT user_id, user_name FROM tasks WHERE user_id IS NOT NULL AND user_id != '' AND user_name IS NOT NULL AND user_name != ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var uid, uname string
		if err := rows.Scan(&uid, &uname); err != nil {
			return nil, err
		}
		if _, exists := result[uid]; !exists {
			result[uid] = uname
		}
	}
	return result, rows.Err()
}

func loadUserNamesFromCommits(db *sql.DB) (map[string]string, error) {
	result := make(map[string]string)
	rows, err := db.Query(`SELECT DISTINCT user_id, user_name FROM commits WHERE user_id IS NOT NULL AND user_id != '' AND user_name IS NOT NULL AND user_name != ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var uid, uname string
		if err := rows.Scan(&uid, &uname); err != nil {
			return nil, err
		}
		if _, exists := result[uid]; !exists {
			result[uid] = uname
		}
	}
	return result, rows.Err()
}
