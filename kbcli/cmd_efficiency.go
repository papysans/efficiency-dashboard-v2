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

// userTaskAgg 存储按用户聚合的 task 统计数据，用于后续计算用户生产力。
//
// 字段说明:
//   - TaskIds / WorkDirIds: 使用 PostgreSQL array_to_json 聚合得到的 JSON 数组字符串
//   - TaskDiffLines: 该用户当日所有 task 的新增代码行数总和
//   - UpstreamTokens / DownstreamTokens: AI 对话上下文的 token 消耗统计
//   - Cost: AI 服务调用成本汇总
//   - TaskRealMinutes: 用户实际耗时分钟数（优先取人工修正值 task_real_minutes_manual）
//   - TaskAncientMinutes: 原始人分钟估算值（优先取人工修正值 task_ancient_minutes_manual）
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

// userCommitAgg 存储按用户聚合的 commit 统计数据，用于后续计算用户生产力。
//
// 字段说明:
//   - CommitIds: 该用户当日所有 commit ID 的 JSON 数组字符串
//   - CommitDiffLines: 新增代码行数总和
//   - CommitAncientMinutes: 原始人分钟估算值（优先取人工修正值 commit_ancient_minutes_manual）
//   - commitRealAiMinutes: AI 实际耗时分钟数
//   - CommitRealAncientMinutes: 原始人实际耗时分钟数
//   - CommitRealMinutes: 用户实际耗时分钟数（优先取人工修正值 commit_real_minutes_manual）
type userCommitAgg struct {
	UserId                   string
	CommitIds                models.StringJSON
	CommitDiffLines          int64
	CommitAncientMinutes     float64
	commitRealAiMinutes      float64
	CommitRealAncientMinutes float64
	CommitRealMinutes        float64
}

// getAllDates 从 tasks 和 commits 两张表中提取所有不重复的日期，按升序返回。
//
// 参数:
//   - db: GORM 数据库连接
//
// 返回值:
//   - []string: 日期字符串列表，格式为 YYYYMMDD
//   - error: 查询执行失败时返回错误
//
// 关键技术原理:
//  1. 使用 SQL UNION 合并 tasks.start_time 和 commits.commit_time 两个时间来源的去重日期
//  2. 使用 PostgreSQL 的 TO_CHAR(DATE(...), 'YYYYMMDD') 将时间戳转为统一格式的日期字符串
//  3. 外层排序确保日期按时间先后顺序处理，避免乱序导致下游统计异常
func getAllDates(db *gorm.DB) ([]string, error) {
	var dates []string
	// 通过原生 SQL 查询所有涉及任务的日期，UNION 去重后按日期升序排列
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

// calculateUserProductivity 计算指定日期下所有用户的生产力数据，并写入 user_productivity 表。
//
// 参数:
//   - db: GORM 数据库连接
//   - dateStr: 目标日期，格式 YYYYMMDD
//   - userNameMap: 从 user_org 表加载的用户 ID -> 用户名映射（优先级最高）
//   - taskUserNameMap: 从 tasks 表提取的用户 ID -> 用户名映射（优先级次之）
//   - commitUserNameMap: 从 commits 表提取的用户 ID -> 用户名映射（优先级最低）
//
// 返回值:
//   - int: 成功写入/更新的用户记录数
//   - error: 聚合或写入过程中发生的错误
//
// 关键技术原理:
//  1. 分别从 tasks 和 commits 聚合当日数据，得到两个 map[string]*agg，key 均为 user_id
//  2. 合并两个 map 的 key 得到全量用户列表，保证只有 task 或只有 commit 的用户也能被统计
//  3. 用户名三级回退策略：user_org > tasks > commits，尽可能保证用户名不缺失
//  4. 使用 utils.CalcEfficiencyRatio 分别计算 task 效能比和 commit 效能比
//     效能比 = AncientMinutes / RealMinutes，反映 AI 辅助相对于纯人工的效率提升
//  5. 在数据库事务中逐用户 UPSERT 写入 user_productivity 表，确保单日数据的原子性
//  6. 使用 clause.OnConflict 以 user_productivity_id（user_id + "_" + dateStr）为唯一键，
//     冲突时更新全部业务字段，实现增量更新和重跑兼容
func calculateUserProductivity(db *gorm.DB, dateStr string, userNameMap, taskUserNameMap, commitUserNameMap map[string]string) (int, error) {
	// 按用户聚合当日的 task 数据
	taskAggMap, err := aggregateTasksByUser(db, dateStr)
	if err != nil {
		return 0, fmt.Errorf("聚合task数据失败: %w", err)
	}
	logInfof("聚合task数据: %d 个用户", len(taskAggMap))

	// 按用户聚合当日的 commit 数据
	commitAggMap, err := aggregateCommitsByUser(db, dateStr)
	if err != nil {
		return 0, fmt.Errorf("聚合commit数据失败: %w", err)
	}
	logInfof("聚合commit数据: %d 个用户", len(commitAggMap))

	// 合并 task 和 commit 的用户集合，得到当日全量用户列表
	allUserIDs := make(map[string]bool)
	for uid := range taskAggMap {
		allUserIDs[uid] = true
	}
	for uid := range commitAggMap {
		allUserIDs[uid] = true
	}

	// 当日没有任何数据的用户，直接返回
	if len(allUserIDs) == 0 {
		logInfo("没有找到有task或commit数据的用户")
		return 0, nil
	}

	// 在事务中逐用户计算并写入 user_productivity 表
	err = db.Transaction(func(tx *gorm.DB) error {
		for uid := range allUserIDs {
			ta := taskAggMap[uid]   // 该用户的 task 聚合数据，可能为 nil
			ca := commitAggMap[uid] // 该用户的 commit 聚合数据，可能为 nil

			// 用户名三级回退策略，尽量保证有可读名称
			userName := userNameMap[uid]
			if userName == "" {
				userName = taskUserNameMap[uid]
			}
			if userName == "" {
				userName = commitUserNameMap[uid]
			}

			// 声明所有需要写入 user_productivity 的字段，默认零值
			var TaskIdsJSON, WorkDirIdsJson, commitIDsJSON []byte
			var taskDiffLines, upstreamTokens, downstreamTokens int64
			var cost, taskRealMinutes, taskAncientMinutes float64
			var commitDiffLines int64
			var commitAncientMinutes, commitRealAiMinutes, commitRealAncientMinutes, commitRealMinutes float64

			// 如果该用户有 task 数据，提取对应指标
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
			// 如果该用户有 commit 数据，提取对应指标
			if ca != nil {
				commitIDsJSON = defaultSliceJSON(ca.CommitIds)
				commitDiffLines = ca.CommitDiffLines
				commitAncientMinutes = ca.CommitAncientMinutes
				commitRealAiMinutes = ca.commitRealAiMinutes
				commitRealAncientMinutes = ca.CommitRealAncientMinutes
				commitRealMinutes = ca.CommitRealMinutes
			}

			// 计算 task 和 commit 的效能比：原始人分钟 / 实际分钟
			taskEffRatio := utils.CalcEfficiencyRatio(taskAncientMinutes, taskRealMinutes)
			commitEffRatio := utils.CalcEfficiencyRatio(commitAncientMinutes, commitRealMinutes)

			// 兜底：确保 JSON 字段不为空，统一为 "[]"
			if TaskIdsJSON == nil {
				TaskIdsJSON = []byte("[]")
			}
			if WorkDirIdsJson == nil {
				WorkDirIdsJson = []byte("[]")
			}
			if commitIDsJSON == nil {
				commitIDsJSON = []byte("[]")
			}

			// 将 YYYYMMDD 字符串解析为 time.Time，并统一设置为 UTC 零点
			createTime, err := time.Parse("20060102", dateStr)
			if err != nil {
				logWarnf("解析日期字符串失败 [%s]: %v", dateStr, err)
			}
			createTime = time.Date(createTime.Year(), createTime.Month(), createTime.Day(), 0, 0, 0, 0, time.UTC)

			// 构造 user_productivity 模型对象
			up := models.UserProductivity{
				UserProductivityId:       uid + "_" + dateStr, // 复合主键：user_id + "_" + dateStr
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

			// UPSERT：以 user_productivity_id 为唯一键，冲突时更新全部业务字段
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

// loadUserNames 从 user_org 表中加载所有已设置 user_name 的用户映射。
//
// 参数:
//   - db: GORM 数据库连接
//
// 返回值:
//   - map[string]string: user_id -> user_name 的映射表
//   - error: 查询失败时返回错误
//
// 说明:
//
//	user_org 表中的用户名通常由管理员维护，优先级最高，用于统一各业务表中的用户显示名称。
func loadUserNames(db *gorm.DB) (map[string]string, error) {
	var userOrgs []models.UserOrg
	// 只查询 user_name 非空的记录，避免将空值覆盖有效名称
	if err := db.Select("user_id, user_name").Where("user_name IS NOT NULL AND user_name != ''").Find(&userOrgs).Error; err != nil {
		return nil, fmt.Errorf("查询user_org用户名失败: %w", err)
	}
	result := make(map[string]string)
	for _, uo := range userOrgs {
		result[uo.UserId] = uo.UserName
	}
	return result, nil
}

// aggregateTasksByUser 按用户聚合指定日期的 task 统计数据。
//
// 参数:
//   - db: GORM 数据库连接
//   - dateStr: 目标日期，格式 YYYYMMDD
//
// 返回值:
//   - map[string]*userTaskAgg: user_id -> task 聚合数据的映射
//   - error: SQL 执行失败时返回错误
//
// 关键技术原理:
//  1. 使用 PostgreSQL 原生 SQL 聚合：array_to_json(array_agg(...)) 将多个 task_id 转为 JSON 数组字符串
//  2. array_agg(DISTINCT ...) FILTER 用于去空值后聚合 work_dir_id
//  3. COALESCE(SUM(...), 0) 保证无数据时返回 0 而非 NULL，避免下游空指针
//  4. 实际耗时和原始人分钟优先取人工修正值（xxx_manual），未修正时使用算法估算值
//  5. 日期匹配使用 DATE(start_time) = $1，利用 PostgreSQL 的日期类型自动转换和索引优化
func aggregateTasksByUser(db *gorm.DB, dateStr string) (map[string]*userTaskAgg, error) {
	var rows []userTaskAgg
	// 原生 SQL 按 user_id 分组聚合 task 数据
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

	// 将结果转为 map，key 为 user_id，便于 O(1) 查找
	result := make(map[string]*userTaskAgg)
	for i := range rows {
		result[rows[i].UserId] = &rows[i]
	}
	return result, nil
}

// aggregateCommitsByUser 按用户聚合指定日期的 commit 统计数据。
//
// 参数:
//   - db: GORM 数据库连接
//   - dateStr: 目标日期，格式 YYYYMMDD
//
// 返回值:
//   - map[string]*userCommitAgg: user_id -> commit 聚合数据的映射
//   - error: SQL 执行失败时返回错误
//
// 关键技术原理:
//  1. 与 aggregateTasksByUser 类似，使用 PostgreSQL 原生聚合函数
//  2. commit_ancient_minutes 和 commit_real_minutes 同样采用人工修正值优先策略（xxx_manual）
//  3. commit_real_ai_minutes 和 commit_real_ancient_minutes 直接求和，不涉及人工修正逻辑
//  4. 日期匹配使用 DATE(commit_time) = $1，确保跨时区场景下按自然日统计
func aggregateCommitsByUser(db *gorm.DB, dateStr string) (map[string]*userCommitAgg, error) {
	var rows []userCommitAgg
	// 原生 SQL 按 user_id 分组聚合 commit 数据
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

	// 将结果转为 map，key 为 user_id
	result := make(map[string]*userCommitAgg)
	for i := range rows {
		result[rows[i].UserId] = &rows[i]
	}
	return result, nil
}

// defaultSliceJSON 将 StringJSON 值兜底为 JSON 数组字符串 "[]"。
//
// 参数:
//   - j: 来自数据库聚合的 StringJSON 值，可能为空字符串或 "null"
//
// 返回值:
//   - []byte: 有效的 JSON 数组字节切片，确保下游序列化不会出错
//
// 说明:
//
//	PostgreSQL 聚合空集合时可能返回 "" 或 "null"，而非 "[]"，
//	该函数用于统一兜底，保证写入 user_productivity 的 JSON 字段始终合法。
func defaultSliceJSON(j models.StringJSON) []byte {
	if j == "" || j == "null" {
		return []byte("[]")
	}
	return []byte(j)
}

// loadUserNamesFromTasks 从 tasks 表中提取有用户名的 user_id -> user_name 映射。
//
// 参数:
//   - db: GORM 数据库连接
//
// 返回值:
//   - map[string]string: user_id -> user_name 映射
//   - error: 查询失败时返回错误
//
// 说明:
//
//	由于一个 user_id 可能对应多条 task 记录，取第一条遇到的非空 user_name 即可。
//	该映射作为 calculateUserProductivity 中用户名的第二优先级来源。
func loadUserNamesFromTasks(db *gorm.DB) (map[string]string, error) {
	type row struct {
		UserId   string
		UserName string
	}
	var rows []row
	// 去重查询 tasks 表中同时有 user_id 和 user_name 的记录
	if err := db.Raw(`SELECT DISTINCT user_id, user_name FROM tasks WHERE user_id IS NOT NULL AND user_id != '' AND user_name IS NOT NULL AND user_name != ''`).Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for _, r := range rows {
		// 若该 user_id 尚未设置名称，则使用当前记录的值
		if _, exists := result[r.UserId]; !exists {
			result[r.UserId] = r.UserName
		}
	}
	return result, nil
}

// loadUserNamesFromCommits 从 commits 表中提取有用户名的 user_id -> user_name 映射。
//
// 参数:
//   - db: GORM 数据库连接
//
// 返回值:
//   - map[string]string: user_id -> user_name 映射
//   - error: 查询失败时返回错误
//
// 说明:
//
//	该映射作为 calculateUserProductivity 中用户名的第三优先级来源，
//	用于兜底那些既不在 user_org 也不在 tasks 中有名称的用户。
func loadUserNamesFromCommits(db *gorm.DB) (map[string]string, error) {
	type row struct {
		UserId   string
		UserName string
	}
	var rows []row
	// 去重查询 commits 表中同时有 user_id 和 user_name 的记录
	if err := db.Raw(`SELECT DISTINCT user_id, user_name FROM commits WHERE user_id IS NOT NULL AND user_id != '' AND user_name IS NOT NULL AND user_name != ''`).Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for _, r := range rows {
		// 若该 user_id 尚未设置名称，则使用当前记录的值
		if _, exists := result[r.UserId]; !exists {
			result[r.UserId] = r.UserName
		}
	}
	return result, nil
}

// runEfficiency 执行效能计算的主流程，支持按指定日期或全量日期计算用户生产力。
//
// 参数:
//   - startDateStr: 限定起始日期，格式 2006-01-02
//   - endDateStr: 限定结束日期，格式 2006-01-02
//   - dateStr: 限定日期，格式 2006-01-02；与 startDateStr/endDateStr 互斥
//
// 返回值:
//   - error: 参数校验、数据库连接、数据聚合或写入过程中发生的错误
//
// 关键技术原理:
//  1. 参数校验：解析日期参数为统一格式，避免无效日期导致全表扫描
//  2. 数据源：从 tasks 和 commits 两张表提取有数据的日期，支持全量自动发现和单日期定点计算两种模式
//  3. 用户名加载：分别加载 user_org、tasks、commits 三个来源的用户名，形成三级回退策略
//  4. 逐日处理：循环 dates 列表，每天独立调用 calculateUserProductivity，日志按日期分段，便于排查
//  5. 容错设计：loadUserNamesFromTasks / loadUserNamesFromCommits 失败时仅输出警告，不中断主流程
//  6. 命令埋点：通过 recordCommandRun 记录执行结果，用于监控和告警
func runEfficiency(startDateStr, endDateStr, dateStr string) error {
	startTime := time.Now()

	// 解析日期范围
	startDate, endDate, err := parseDateRange(startDateStr, endDateStr, dateStr)
	if err != nil {
		recordCommandRun("efficiency", startTime, 0, 0, 0, err)
		return err
	}

	// 建立数据库连接，并在函数退出时关闭底层连接
	db, err := models.OpenGormDB(cfg.StatDatabase.DSN())
	if err != nil {
		recordCommandRun("efficiency", startTime, 0, 0, 0, err)
		return fmt.Errorf("连接数据库失败: %w", err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	// 确定需要处理的日期列表
	var dates []string
	if dateStr != "" {
		// 单日期模式：直接处理指定日期
		t, _ := time.Parse("2006-01-02", dateStr)
		dates = []string{t.Format("20060102")}
	} else {
		// 全量模式：从数据库中提取所有有数据的日期
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
		// 根据日期范围过滤
		if startDate != nil || endDate != nil {
			var filtered []string
			for _, d := range dates {
				t, err := time.Parse("20060102", d)
				if err != nil {
					continue
				}
				if isActiveTimeInRange(t, startDate, endDate) {
					filtered = append(filtered, d)
				}
			}
			dates = filtered
		}
		if len(dates) == 0 {
			logInfo("日期过滤后没有待处理的日期")
			recordCommandRun("efficiency", startTime, 0, 0, 0, nil)
			return nil
		}
		logInfof("共发现 %d 个日期需要处理", len(dates))
	}

	// 加载三个来源的用户名映射
	userNameMap, err := loadUserNames(db)
	if err != nil {
		recordCommandRun("efficiency", startTime, 0, 0, 0, err)
		return fmt.Errorf("加载用户名称失败: %w", err)
	}

	// 从 tasks 和 commits 加载用户名作为备用，失败仅警告不阻断
	taskUserNameMap, err := loadUserNamesFromTasks(db)
	if err != nil {
		logWarnf("加载task用户名失败: %v", err)
	}
	commitUserNameMap, err := loadUserNamesFromCommits(db)
	if err != nil {
		logWarnf("加载commit用户名失败: %v", err)
	}

	// 逐日计算用户生产力并汇总总用户数
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

	// 输出最终统计并记录命令执行结果
	logInfof("全部完成: 用户 %d 条", totalUserCount)
	recordCommandRun("efficiency", startTime, totalUserCount, 0, 0, nil)
	return nil
}

// efficiencyCmd 定义了 "efficiency" Cobra 子命令，用于按日计算用户和组织效能数据。
//
// 用法: kbcli efficiency [--date YYYYMMDD] [--remote URL]
//
// 标志说明:
//   - date: 指定聚合日期，格式 YYYYMMDD；不指定则自动处理所有有数据的日期
//   - remote: 若指定远程 kbcli 服务地址，则将命令参数发送到远程执行，本地不处理
//
// 功能说明:
//
//	根据已导入的 task、commit、user_org 数据，按日计算各用户的生产力数据并写入 user_productivity 表。
//	支持单日期重跑和全量历史计算两种模式，使用 UPSERT 机制保证可重复执行。
var efficiencyCmd = &cobra.Command{
	Use:   "efficiency",
	Short: "按日计算用户和组织效能数据",
	Long:  "根据已导入的task、commit、user_org数据，按日计算各用户的生产力数据，写入user_productivity表。如有--date参数，则只处理该日期的数据，否则处理所有日期数据",
	RunE: func(cmd *cobra.Command, args []string) error {
		// 从命令行标志读取参数
		dateStr, _ := cmd.Flags().GetString("date")
		startDate, _ := cmd.Flags().GetString("start-date")
		endDate, _ := cmd.Flags().GetString("end-date")
		remote, _ := cmd.Flags().GetString("remote")

		// 若指定了 remote，将参数发送到远程执行
		if remote != "" {
			return sendToRemote(remote, "efficiency", map[string]interface{}{
				"date":       dateStr,
				"start_date": startDate,
				"end_date":   endDate,
			})
		}

		// 本地执行效能计算
		return runEfficiency(startDate, endDate, dateStr)
	},
}

// init 注册 efficiency 命令及其命令行标志到 rootCmd。
//
// 说明:
//
//	SortFlags 设为 false 保持标志按定义顺序显示，提升可读性。
func init() {
	efficiencyCmd.Flags().SortFlags = false
	efficiencyCmd.Flags().String("date", "", "限定日期，格式 YYYYMMDD，限定活跃时间在该日期之内（与start-date/end-date互斥）")
	efficiencyCmd.Flags().String("start-date", "", "限定起始日期，格式 YYYYMMDD，为空则不限")
	efficiencyCmd.Flags().String("end-date", "", "限定结束日期，格式 YYYYMMDD，为空则不限")
	efficiencyCmd.Flags().String("remote", "", "远程kbcli服务地址（如 http://127.0.0.1:8080），指定后命令将发送到远程执行")
	rootCmd.AddCommand(efficiencyCmd)
}
