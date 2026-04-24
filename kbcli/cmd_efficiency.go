package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type userTaskAgg struct {
	UserID             string
	TaskIDs            StringJSON
	WorkDirIDs         StringJSON
	TaskDiffLines      int64
	UpstreamTokens     int64
	DownstreamTokens   int64
	Cost               float64
	TaskRealMinutes    float64
	TaskAncientMinutes float64
}

type userCommitAgg struct {
	UserID                   string
	CommitIDs                StringJSON
	CommitDiffLines          int64
	CommitAncientMinutes     float64
	CommitRealAIMinutes      float64
	CommitRealAncientMinutes float64
	CommitRealMinutes        float64
}

func runEfficiency(dateStr string) error {
	if dateStr != "" && len(dateStr) != 8 {
		return fmt.Errorf("--date 格式应为 YYYYMMDD，当前: %s", dateStr)
	}

	db, err := openGormDB(cfg.StatDatabase.DSN())
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	var dates []string
	if dateStr != "" {
		dates = []string{dateStr}
	} else {
		dates, err = getAllDatesGorm(db)
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
		userCount, err := calculateUserProductivityGorm(db, d)
		if err != nil {
			return fmt.Errorf("计算用户生产力失败 [date=%s]: %w", d, err)
		}
		fmt.Printf("用户生产力计算完成: %d 条记录 (日期=%s)\n", userCount, d)
		totalUserCount += userCount

		orgCount, err := calculateOrgProductivityGorm(db, d)
		if err != nil {
			return fmt.Errorf("计算组织生产力失败 [date=%s]: %w", d, err)
		}
		fmt.Printf("组织生产力计算完成: %d 条记录 (日期=%s)\n", orgCount, d)
		totalOrgCount += orgCount
	}

	fmt.Printf("\n全部完成: 用户 %d 条, 组织 %d 条\n", totalUserCount, totalOrgCount)
	return nil
}

var efficiencyCmd = &cobra.Command{
	Use:   "efficiency",
	Short: "按日计算用户和组织效能数据",
	Long:  "根据已导入的task、commit、user_org数据，按日计算各用户和组织（分级）的生产力数据，写入user_productivity和org_productivity表。如有--date参数，则只处理该日期的数据，否则处理所有日期数据",
	RunE: func(cmd *cobra.Command, args []string) error {
		dateStr, _ := cmd.Flags().GetString("date")
		return runEfficiency(dateStr)
	},
}

func init() {
	efficiencyCmd.Flags().String("date", "", "聚合日期，格式YYYYMMDD，不指定则处理所有日期")
	rootCmd.AddCommand(efficiencyCmd)
}

func getAllDatesGorm(db *gorm.DB) ([]string, error) {
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

func calculateUserProductivityGorm(db *gorm.DB, dateStr string) (int, error) {
	userNameMap, err := loadUserNamesGorm(db)
	if err != nil {
		return 0, fmt.Errorf("加载用户名称失败: %w", err)
	}

	taskUserNameMap, err := loadUserNamesFromTasksGorm(db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "警告: 加载task用户名失败: %v\n", err)
	}
	commitUserNameMap, err := loadUserNamesFromCommitsGorm(db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "警告: 加载commit用户名失败: %v\n", err)
	}

	taskAggMap, err := aggregateTasksByUserGorm(db, dateStr)
	if err != nil {
		return 0, fmt.Errorf("聚合task数据失败: %w", err)
	}
	fmt.Printf("聚合task数据: %d 个用户\n", len(taskAggMap))

	commitAggMap, err := aggregateCommitsByUserGorm(db, dateStr)
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

			var taskIDsJSON, workDirIDsJSON, commitIDsJSON []byte
			var taskDiffLines, upstreamTokens, downstreamTokens int64
			var cost, taskRealMinutes, taskAncientMinutes float64
			var commitDiffLines int64
			var commitAncientMinutes, commitRealAIMinutes, commitRealAncientMinutes, commitRealMinutes float64

			if ta != nil {
				taskIDsJSON = defaultSliceJSON(ta.TaskIDs)
				workDirIDsJSON = defaultSliceJSON(ta.WorkDirIDs)
				taskDiffLines = ta.TaskDiffLines
				upstreamTokens = ta.UpstreamTokens
				downstreamTokens = ta.DownstreamTokens
				cost = ta.Cost
				taskRealMinutes = ta.TaskRealMinutes
				taskAncientMinutes = ta.TaskAncientMinutes
			}
			if ca != nil {
				commitIDsJSON = defaultSliceJSON(ca.CommitIDs)
				commitDiffLines = ca.CommitDiffLines
				commitAncientMinutes = ca.CommitAncientMinutes
				commitRealAIMinutes = ca.CommitRealAIMinutes
				commitRealAncientMinutes = ca.CommitRealAncientMinutes
				commitRealMinutes = ca.CommitRealMinutes
			}

			taskEffRatio := calcEfficiencyRatio(taskAncientMinutes, taskRealMinutes)
			commitEffRatio := calcEfficiencyRatio(commitAncientMinutes, commitRealMinutes)

			if taskIDsJSON == nil {
				taskIDsJSON = []byte("[]")
			}
			if workDirIDsJSON == nil {
				workDirIDsJSON = []byte("[]")
			}
			if commitIDsJSON == nil {
				commitIDsJSON = []byte("[]")
			}

			createTime, _ := time.Parse("20060102", dateStr)
			createTime = time.Date(createTime.Year(), createTime.Month(), createTime.Day(), 0, 0, 0, 0, time.UTC)

			up := UserProductivity{
				UserProductivityID:   uid + "_" + dateStr,
				CreateTime:           &createTime,
				UserID:               uid,
				UserName:             userName,
				TaskIDs:              StringJSON(taskIDsJSON),
				WorkDirIDs:           StringJSON(workDirIDsJSON),
				TaskDiffLines:        int(taskDiffLines),
				UpstreamTokens:       upstreamTokens,
				DownstreamTokens:     downstreamTokens,
				Cost:                 cost,
				TaskRealMinutes:      taskRealMinutes,
				TaskAncientMinutes:   taskAncientMinutes,
				TaskEfficiencyRatio:  taskEffRatio,
				CommitIDs:            StringJSON(commitIDsJSON),
				CommitDiffLines:      int(commitDiffLines),
				CommitAncientMinutes: commitAncientMinutes,
				CommitRealAIMinutes:  commitRealAIMinutes,
				CommitRealAncMin:     commitRealAncientMinutes,
				CommitRealMinutes:    commitRealMinutes,
				CommitEfficiencyRtio: commitEffRatio,
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

func calculateOrgProductivityGorm(db *gorm.DB, dateStr string) (int, error) {
	orgTree, err := buildOrgTreeGorm(db)
	if err != nil {
		return 0, fmt.Errorf("构建组织树失败: %w", err)
	}

	if len(orgTree) == 0 {
		fmt.Println("没有组织数据，跳过组织生产力计算")
		return 0, nil
	}

	createTime, _ := time.Parse("20060102", dateStr)
	createTime = time.Date(createTime.Year(), createTime.Month(), createTime.Day(), 0, 0, 0, 0, time.UTC)

	err = db.Transaction(func(tx *gorm.DB) error {
		for _, node := range orgTree {
			if len(node.UserIDs) == 0 {
				continue
			}

			var agg struct {
				TaskDiffLines      int64
				UpstreamTokens     int64
				DownstreamTokens   int64
				Cost               float64
				TaskRealMinutes    float64
				TaskAncientMinutes float64
				CommitDiffLines    int64
				CommitAncMinutes   float64
				CommitRealAIMin    float64
				CommitRealAncMin   float64
				CommitRealMin      float64
			}

			if err := tx.Raw(`
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
				WHERE user_id = ANY(?) AND create_time = ?
			`, pq.Array(node.UserIDs), createTime).Scan(&agg).Error; err != nil {
				return fmt.Errorf("查询组织聚合数据失败 [%s]: %w", node.OrgName, err)
			}

			taskEffRatio := calcEfficiencyRatio(agg.TaskAncientMinutes, agg.TaskRealMinutes)
			commitEffRatio := calcEfficiencyRatio(agg.CommitAncMinutes, agg.CommitRealMin)

			userIDsJSON, _ := json.Marshal(node.UserIDs)
			userNamesJSON, _ := json.Marshal(node.UserNames)

			op := OrgProductivity{
				OrgID:                newUUID(),
				OrgName:              node.OrgName,
				OrgLevel:             node.Level,
				CreateTime:           &createTime,
				UserIDs:              StringJSON(userIDsJSON),
				UserNames:            StringJSON(userNamesJSON),
				TaskDiffLines:        int(agg.TaskDiffLines),
				UpstreamTokens:       agg.UpstreamTokens,
				DownstreamTokens:     agg.DownstreamTokens,
				Cost:                 agg.Cost,
				TaskRealMinutes:      agg.TaskRealMinutes,
				TaskAncientMinutes:   agg.TaskAncientMinutes,
				TaskEfficiencyRatio:  taskEffRatio,
				CommitDiffLines:      int(agg.CommitDiffLines),
				CommitAncientMinutes: agg.CommitAncMinutes,
				CommitRealAIMinutes:  agg.CommitRealAIMin,
				CommitRealAncMin:     agg.CommitRealAncMin,
				CommitRealMinutes:    agg.CommitRealMin,
				CommitEfficiencyRtio: commitEffRatio,
				UserCount:            len(node.UserIDs),
			}

			result := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "org_name"}, {Name: "create_time"}},
				DoUpdates: clause.AssignmentColumns([]string{
					"org_level", "user_ids", "user_names",
					"task_diff_lines", "upstream_tokens", "downstream_tokens", "cost",
					"task_real_minutes", "task_ancient_minutes", "task_efficiency_ratio",
					"commit_diff_lines", "commit_ancient_minutes",
					"commit_real_ai_minutes", "commit_real_ancient_minutes", "commit_real_minutes",
					"commit_efficiency_ratio", "user_count", "updated_at",
				}),
			}).Create(&op)
			if result.Error != nil {
				return fmt.Errorf("写入org_productivity失败 [%s]: %w", node.OrgName, result.Error)
			}

			fmt.Printf("  组织 [%s] (L%d): %d个用户, task_eff=%.2f, commit_eff=%.2f\n",
				node.OrgName, node.Level, len(node.UserIDs), taskEffRatio, commitEffRatio)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

	return len(orgTree), nil
}

type orgNode struct {
	OrgName   string
	Level     int
	UserIDs   []string
	UserNames []string
}

func buildOrgTreeGorm(db *gorm.DB) ([]*orgNode, error) {
	var userOrgs []UserOrg
	if err := db.Where("org1 IS NOT NULL AND org1 != ''").Find(&userOrgs).Error; err != nil {
		return nil, fmt.Errorf("查询user_org失败: %w", err)
	}

	type userOrgInfo struct {
		UserID   string
		UserName string
		Orgs     []string
	}

	var infos []userOrgInfo
	for _, uo := range userOrgs {
		var nonEmptyOrgs []string
		for _, o := range []string{uo.Org1, uo.Org2, uo.Org3, uo.Org4, uo.Org5, uo.Org6, uo.Org7, uo.Org8, uo.Org9} {
			if o != "" {
				nonEmptyOrgs = append(nonEmptyOrgs, o)
			} else {
				break
			}
		}
		if len(nonEmptyOrgs) > 0 {
			infos = append(infos, userOrgInfo{UserID: uo.UserID, UserName: uo.UserName, Orgs: nonEmptyOrgs})
		}
	}

	orgMap := make(map[string]*orgNode)
	for _, uo := range infos {
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

func loadUserNamesGorm(db *gorm.DB) (map[string]string, error) {
	var userOrgs []UserOrg
	if err := db.Select("user_id, user_name").Where("user_name IS NOT NULL AND user_name != ''").Find(&userOrgs).Error; err != nil {
		return nil, fmt.Errorf("查询user_org用户名失败: %w", err)
	}
	result := make(map[string]string)
	for _, uo := range userOrgs {
		result[uo.UserID] = uo.UserName
	}
	return result, nil
}

func aggregateTasksByUserGorm(db *gorm.DB, dateStr string) (map[string]*userTaskAgg, error) {
	type row struct {
		UserID             string
		TaskIDs            StringJSON
		WorkDirIDs         StringJSON
		TaskDiffLines      int64
		UpstreamTokens     int64
		DownstreamTokens   int64
		Cost               float64
		TaskRealMinutes    float64
		TaskAncientMinutes float64
	}

	var rows []row
	if err := db.Raw(`
		SELECT
			user_id,
			COALESCE(array_to_json(array_agg(task_id)), '[]'),
			COALESCE(array_to_json(array_agg(DISTINCT work_dir_id) FILTER (WHERE work_dir_id IS NOT NULL AND work_dir_id != '')), '[]'),
			COALESCE(SUM(diff_lines), 0),
			COALESCE(SUM(upstream_tokens), 0),
			COALESCE(SUM(downstream_tokens), 0),
			COALESCE(SUM(cost), 0),
			COALESCE(SUM(COALESCE(task_real_minutes_manual, task_real_minutes)), 0),
			COALESCE(SUM(COALESCE(task_ancient_minutes_manual, task_ancient_minutes)), 0)
		FROM tasks
		WHERE user_id IS NOT NULL AND user_id != '' AND DATE(start_time) = $1
		GROUP BY user_id
	`, dateStr).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("查询task聚合数据失败: %w", err)
	}

	result := make(map[string]*userTaskAgg)
	for i := range rows {
		result[rows[i].UserID] = &userTaskAgg{
			UserID:             rows[i].UserID,
			TaskIDs:            rows[i].TaskIDs,
			WorkDirIDs:         rows[i].WorkDirIDs,
			TaskDiffLines:      rows[i].TaskDiffLines,
			UpstreamTokens:     rows[i].UpstreamTokens,
			DownstreamTokens:   rows[i].DownstreamTokens,
			Cost:               rows[i].Cost,
			TaskRealMinutes:    rows[i].TaskRealMinutes,
			TaskAncientMinutes: rows[i].TaskAncientMinutes,
		}
	}
	return result, nil
}

func aggregateCommitsByUserGorm(db *gorm.DB, dateStr string) (map[string]*userCommitAgg, error) {
	type row struct {
		UserID                   string
		CommitIDs                StringJSON
		CommitDiffLines          int64
		CommitAncientMinutes     float64
		CommitRealAIMinutes      float64
		CommitRealAncientMinutes float64
		CommitRealMinutes        float64
	}

	var rows []row
	if err := db.Raw(`
		SELECT
			user_id,
			COALESCE(array_to_json(array_agg(commit_id)), '[]'),
			COALESCE(SUM(diff_lines), 0),
			COALESCE(SUM(COALESCE(commit_ancient_minutes_manual, commit_ancient_minutes)), 0),
			COALESCE(SUM(commit_real_ai_minutes), 0),
			COALESCE(SUM(commit_real_ancient_minutes), 0),
			COALESCE(SUM(COALESCE(commit_real_minutes_manual, commit_real_minutes)), 0)
		FROM commits
		WHERE user_id IS NOT NULL AND user_id != '' AND DATE(commit_time) = $1
		GROUP BY user_id
	`, dateStr).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("查询commit聚合数据失败: %w", err)
	}

	result := make(map[string]*userCommitAgg)
	for i := range rows {
		result[rows[i].UserID] = &userCommitAgg{
			UserID:                   rows[i].UserID,
			CommitIDs:                rows[i].CommitIDs,
			CommitDiffLines:          rows[i].CommitDiffLines,
			CommitAncientMinutes:     rows[i].CommitAncientMinutes,
			CommitRealAIMinutes:      rows[i].CommitRealAIMinutes,
			CommitRealAncientMinutes: rows[i].CommitRealAncientMinutes,
			CommitRealMinutes:        rows[i].CommitRealMinutes,
		}
	}
	return result, nil
}

func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func calcEfficiencyRatio(ancientMinutes, realMinutes float64) float64 {
	if ancientMinutes > 0 && realMinutes > 0 && !math.IsInf(realMinutes, 0) {
		return (ancientMinutes / realMinutes) * 100
	}
	return 0
}

func defaultSliceJSON(j StringJSON) []byte {
	if j == "" || j == "null" {
		return []byte("[]")
	}
	return []byte(j)
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

func loadUserNamesFromTasksGorm(db *gorm.DB) (map[string]string, error) {
	type row struct {
		UserID   string
		UserName string
	}
	var rows []row
	if err := db.Raw(`SELECT DISTINCT user_id, user_name FROM tasks WHERE user_id IS NOT NULL AND user_id != '' AND user_name IS NOT NULL AND user_name != ''`).Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for _, r := range rows {
		if _, exists := result[r.UserID]; !exists {
			result[r.UserID] = r.UserName
		}
	}
	return result, nil
}

func loadUserNamesFromCommitsGorm(db *gorm.DB) (map[string]string, error) {
	type row struct {
		UserID   string
		UserName string
	}
	var rows []row
	if err := db.Raw(`SELECT DISTINCT user_id, user_name FROM commits WHERE user_id IS NOT NULL AND user_id != '' AND user_name IS NOT NULL AND user_name != ''`).Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for _, r := range rows {
		if _, exists := result[r.UserID]; !exists {
			result[r.UserID] = r.UserName
		}
	}
	return result, nil
}
