package main

import (
	"encoding/csv"
	"fmt"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
)

// OrgMapping 组织架构映射
type OrgMapping struct {
	UserID       string
	UserName     string
	Org1         string
	Org2         string
	Org3         string
	Org4         string
	GitUserName  string
	GitUserEmail string
}

// orgMappings 全局组织映射表，key=user_id
var orgMappings map[string]*OrgMapping

// LoadOrgMapping 从 CSV 加载组织映射到全局 map
func LoadOrgMapping(csvPath string) error {
	orgMappings = make(map[string]*OrgMapping)

	f, err := os.Open(csvPath)
	if err != nil {
		// 文件不存在或无法打开，不报错，map 为空即可
		return nil
	}
	defer f.Close()

	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		return nil
	}

	// 跳过表头行
	for i := 1; i < len(records); i++ {
		row := records[i]
		if len(row) < 8 {
			continue
		}
		userID := strings.TrimSpace(row[0])
		if userID == "" {
			continue
		}
		orgMappings[userID] = &OrgMapping{
			UserID:       userID,
			UserName:     strings.TrimSpace(row[1]),
			Org1:         strings.TrimSpace(row[2]),
			Org2:         strings.TrimSpace(row[3]),
			Org3:         strings.TrimSpace(row[4]),
			Org4:         strings.TrimSpace(row[5]),
			GitUserName:  strings.TrimSpace(row[6]),
			GitUserEmail: strings.TrimSpace(row[7]),
		}
	}

	return nil
}

// getOrgValue 根据 level 获取 OrgMapping 中对应的 org 值
func getOrgValue(m *OrgMapping, level string) string {
	switch level {
	case "org1":
		return m.Org1
	case "org2":
		return m.Org2
	case "org3":
		return m.Org3
	case "org4":
		return m.Org4
	default:
		return ""
	}
}

// filterUsersByParent 根据 parent 路径筛选用户
func filterUsersByParent(parent string) []*OrgMapping {
	var result []*OrgMapping
	if parent == "" {
		for _, m := range orgMappings {
			result = append(result, m)
		}
		return result
	}

	parts := strings.Split(parent, "/")
	for _, m := range orgMappings {
		match := true
		for i, p := range parts {
			level := fmt.Sprintf("org%d", i+1)
			if getOrgValue(m, level) != p {
				match = false
				break
			}
		}
		if match {
			result = append(result, m)
		}
	}
	return result
}

// listOrgV2 GET /api/v2/orgs
// 支持两种模式：
//   - 不带 granularity（或 granularity=""）：返回 data[]（各组织汇总）
//   - 带 granularity：额外返回 series（各组织×时间段的时间序列，用于图表）
func listOrgV2(c *gin.Context) {
	level := c.Query("level")
	parent := c.Query("parent")
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	granularity := c.Query("granularity")

	if level == "" {
		level = "org1"
	}

	var startTime, endTime string
	if startDate != "" {
		startT, err := parseDateParam(startDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "startDate 格式错误: " + err.Error()})
			return
		}
		startTime = startT.Format(time.RFC3339)
	}
	if endDate != "" {
		endT, err := parseDateParam(endDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "endDate 格式错误: " + err.Error()})
			return
		}
		endTime = endT.Add(23*time.Hour + 59*time.Minute + 59*time.Second).Format(time.RFC3339)
	}

	// 从 orgMappings 筛选符合 parent 条件的用户
	users := filterUsersByParent(parent)
	if len(users) == 0 {
		c.JSON(http.StatusOK, gin.H{"data": []gin.H{}, "series": []gin.H{}})
		return
	}

	// 按当前 level 的 org 值分组
	type orgGroup struct {
		orgName string
		userIDs []string
	}
	groupMap := make(map[string]*orgGroup)
	for _, u := range users {
		orgName := getOrgValue(u, level)
		if orgName == "" {
			continue
		}
		if g, ok := groupMap[orgName]; ok {
			g.userIDs = append(g.userIDs, u.UserID)
		} else {
			groupMap[orgName] = &orgGroup{orgName: orgName, userIDs: []string{u.UserID}}
		}
	}

	if len(groupMap) == 0 {
		c.JSON(http.StatusOK, gin.H{"data": []gin.H{}, "series": []gin.H{}})
		return
	}

	// 从 user_productivity 聚合每个用户的汇总数据
	type userProdAgg struct {
		taskCount        int
		commitCount      int
		taskDiffLines    int
		commitDiffLines  int
		upTokens         int64
		downTokens       int64
		cost             float64
		taskRealMin      float64
		taskAncientMin   float64
		commitRealMin    float64
		commitAncientMin float64
	}

	// 收集所有 user_id
	var allUserIDs []string
	for _, g := range groupMap {
		allUserIDs = append(allUserIDs, g.userIDs...)
	}

	userAggMap := make(map[string]*userProdAgg)

	prodQuery := `SELECT user_id,
		COALESCE(SUM(jsonb_array_length(task_ids)), 0),
		COALESCE(SUM(jsonb_array_length(commit_ids)), 0),
		COALESCE(SUM(task_diff_lines), 0),
		COALESCE(SUM(commit_diff_lines), 0),
		COALESCE(SUM(upstream_tokens), 0),
		COALESCE(SUM(downstream_tokens), 0),
		COALESCE(SUM(cost), 0),
		COALESCE(SUM(task_real_minutes), 0),
		COALESCE(SUM(task_ancient_minutes), 0),
		COALESCE(SUM(commit_real_minutes), 0),
		COALESCE(SUM(commit_ancient_minutes), 0)
		FROM user_productivity
		WHERE user_id = ANY($1::text[])`

	prodArgs := []interface{}{pq.Array(allUserIDs)}
	prodArgIdx := 2
	if startTime != "" {
		prodQuery += fmt.Sprintf(" AND create_time >= $%d", prodArgIdx)
		prodArgs = append(prodArgs, startTime)
		prodArgIdx++
	}
	if endTime != "" {
		prodQuery += fmt.Sprintf(" AND create_time <= $%d", prodArgIdx)
		prodArgs = append(prodArgs, endTime)
		prodArgIdx++
	}
	prodQuery += " GROUP BY user_id"

	prodRows, err := statDB.Query(prodQuery, prodArgs...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询 user_productivity 聚合失败: " + err.Error()})
		return
	}
	defer prodRows.Close()

	for prodRows.Next() {
		var uid string
		var taskCount, commitCount, taskDiffLines, commitDiffLines int
		var upTokens, downTokens int64
		var cost, taskRealMin, taskAncientMin, commitRealMin, commitAncientMin float64
		if err := prodRows.Scan(&uid, &taskCount, &commitCount, &taskDiffLines, &commitDiffLines,
			&upTokens, &downTokens, &cost, &taskRealMin, &taskAncientMin, &commitRealMin, &commitAncientMin); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "扫描 user_productivity 行失败: " + err.Error()})
			return
		}
		userAggMap[uid] = &userProdAgg{
			taskCount: taskCount, commitCount: commitCount,
			taskDiffLines: taskDiffLines, commitDiffLines: commitDiffLines,
			upTokens: upTokens, downTokens: downTokens, cost: cost,
			taskRealMin: taskRealMin, taskAncientMin: taskAncientMin,
			commitRealMin: commitRealMin, commitAncientMin: commitAncientMin,
		}
	}

	// 按 org 分组汇总 → data[]
	type orgAgg struct {
		userCount        int
		taskCount        int
		commitCount      int
		taskDiffLines    int
		commitDiffLines  int
		upTokens         int64
		downTokens       int64
		cost             float64
		taskRealMin      float64
		taskAncientMin   float64
		commitRealMin    float64
		commitAncientMin float64
	}
	orgAggMap := make(map[string]*orgAgg)
	for orgName, g := range groupMap {
		oa := &orgAgg{userCount: len(g.userIDs)}
		for _, uid := range g.userIDs {
			if ua, ok := userAggMap[uid]; ok {
				oa.taskCount += ua.taskCount
				oa.commitCount += ua.commitCount
				oa.taskDiffLines += ua.taskDiffLines
				oa.commitDiffLines += ua.commitDiffLines
				oa.upTokens += ua.upTokens
				oa.downTokens += ua.downTokens
				oa.cost += ua.cost
				oa.taskRealMin += ua.taskRealMin
				oa.taskAncientMin += ua.taskAncientMin
				oa.commitRealMin += ua.commitRealMin
				oa.commitAncientMin += ua.commitAncientMin
			}
		}
		orgAggMap[orgName] = oa
	}

	data := make([]gin.H, 0, len(orgAggMap))
	for orgName, oa := range orgAggMap {
		var taskEffRatio, commitEffRatio float64
		if oa.taskRealMin > 0 {
			taskEffRatio = math.Round(oa.taskAncientMin / oa.taskRealMin * 100)
		}
		if oa.commitRealMin > 0 {
			commitEffRatio = math.Round(oa.commitAncientMin / oa.commitRealMin * 100)
		}
		data = append(data, gin.H{
			"org_name":                orgName,
			"user_count":              oa.userCount,
			"task_count":              oa.taskCount,
			"commit_count":            oa.commitCount,
			"task_diff_lines":         oa.taskDiffLines,
			"commit_diff_lines":       oa.commitDiffLines,
			"task_efficiency_ratio":   taskEffRatio,
			"commit_efficiency_ratio": commitEffRatio,
			"total_tokens":            oa.upTokens + oa.downTokens,
			"total_cost":              oa.cost,
		})
	}

	// 如果不需要时间序列，直接返回
	if granularity == "" {
		c.JSON(http.StatusOK, gin.H{"data": data, "series": []gin.H{}})
		return
	}

	// 构建时间序列：各组织 × 时间段
	// 从 user_productivity 按天查每个用户，再按 org 分组聚合
	type dayOrgKey struct {
		date    string
		orgName string
	}
	type dayOrgAgg struct {
		userCount        int
		taskCount        int
		commitCount      int
		taskDiffLines    int
		commitDiffLines  int
		upTokens         int64
		downTokens       int64
		cost             float64
		taskRealMin      float64
		taskAncientMin   float64
		commitRealMin    float64
		commitAncientMin float64
		users            map[string]bool
	}

	// 建立 uid → orgName 映射
	uidToOrg := make(map[string]string)
	for orgName, g := range groupMap {
		for _, uid := range g.userIDs {
			uidToOrg[uid] = orgName
		}
	}

	dayOrgMap := make(map[dayOrgKey]*dayOrgAgg)

	seriesQuery := `SELECT user_id, create_time,
		COALESCE(jsonb_array_length(task_ids), 0),
		COALESCE(jsonb_array_length(commit_ids), 0),
		COALESCE(task_diff_lines, 0),
		COALESCE(commit_diff_lines, 0),
		COALESCE(upstream_tokens, 0),
		COALESCE(downstream_tokens, 0),
		COALESCE(cost, 0),
		COALESCE(task_real_minutes, 0),
		COALESCE(task_ancient_minutes, 0),
		COALESCE(commit_real_minutes, 0),
		COALESCE(commit_ancient_minutes, 0)
		FROM user_productivity
		WHERE user_id = ANY($1::text[])`

	seriesArgs := []interface{}{pq.Array(allUserIDs)}
	seriesArgIdx := 2
	if startTime != "" {
		seriesQuery += fmt.Sprintf(" AND create_time >= $%d", seriesArgIdx)
		seriesArgs = append(seriesArgs, startTime)
		seriesArgIdx++
	}
	if endTime != "" {
		seriesQuery += fmt.Sprintf(" AND create_time <= $%d", seriesArgIdx)
		seriesArgs = append(seriesArgs, endTime)
		seriesArgIdx++
	}
	seriesQuery += " ORDER BY create_time"

	sRows, err := statDB.Query(seriesQuery, seriesArgs...)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": data, "series": []gin.H{}})
		return
	}
	defer sRows.Close()

	for sRows.Next() {
		var uid string
		var ct time.Time
		var taskCnt, commitCnt, taskDiff, commitDiff int
		var upTok, downTok int64
		var cost, taskReal, taskAnc, commitReal, commitAnc float64
		if err := sRows.Scan(&uid, &ct, &taskCnt, &commitCnt, &taskDiff, &commitDiff,
			&upTok, &downTok, &cost, &taskReal, &taskAnc, &commitReal, &commitAnc); err != nil {
			continue
		}
		orgName, ok := uidToOrg[uid]
		if !ok {
			continue
		}
		dateStr := ct.Format("2006-01-02")
		key := dayOrgKey{date: dateStr, orgName: orgName}
		if _, ok := dayOrgMap[key]; !ok {
			dayOrgMap[key] = &dayOrgAgg{users: make(map[string]bool)}
		}
		da := dayOrgMap[key]
		da.users[uid] = true
		da.taskCount += taskCnt
		da.commitCount += commitCnt
		da.taskDiffLines += taskDiff
		da.commitDiffLines += commitDiff
		da.upTokens += upTok
		da.downTokens += downTok
		da.cost += cost
		da.taskRealMin += taskReal
		da.taskAncientMin += taskAnc
		da.commitRealMin += commitReal
		da.commitAncientMin += commitAnc
	}

	// 收集所有日期，排序
	dateSet := make(map[string]bool)
	for k := range dayOrgMap {
		dateSet[k.date] = true
	}
	var allDates []string
	for d := range dateSet {
		allDates = append(allDates, d)
	}
	sort.Strings(allDates)

	// 按 granularity 聚合日期 → period_label
	// 计算某日期是所在月的第几周（以周一为周起始）
	getWeekOfMonth := func(t time.Time) int {
		// 获取当月第一天
		firstDay := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
		// 计算当月第一天是周几（周一=1, 周日=0）
		firstWeekday := int(firstDay.Weekday())
		if firstWeekday == 0 {
			firstWeekday = 7
		}
		// 计算是该月的第几周
		return (t.Day()+firstWeekday-2)/7 + 1
	}

	periodOf := func(dateStr string) string {
		t, _ := time.Parse("2006-01-02", dateStr)
		switch granularity {
		case "week":
			// 格式：年月第几周，如 "202608第2周"
			weekOfMonth := getWeekOfMonth(t)
			return fmt.Sprintf("%d%02d第%d周", t.Year(), t.Month(), weekOfMonth)
		case "month":
			return t.Format("2006-01")
		case "year":
			return t.Format("2006")
		default:
			return dateStr
		}
	}

	// 按 orgName × period 聚合
	type periodOrgKey struct {
		period  string
		orgName string
	}
	type periodOrgAgg struct {
		taskCount        int
		commitCount      int
		taskDiffLines    int
		commitDiffLines  int
		upTokens         int64
		downTokens       int64
		cost             float64
		taskRealMin      float64
		taskAncientMin   float64
		commitRealMin    float64
		commitAncientMin float64
		users            map[string]bool
	}
	periodOrgMap := make(map[periodOrgKey]*periodOrgAgg)
	periodSet := make(map[string]bool)

	for _, dateStr := range allDates {
		period := periodOf(dateStr)
		periodSet[period] = true
		for orgName := range groupMap {
			key := dayOrgKey{date: dateStr, orgName: orgName}
			da, ok := dayOrgMap[key]
			if !ok {
				continue
			}
			pk := periodOrgKey{period: period, orgName: orgName}
			if _, ok := periodOrgMap[pk]; !ok {
				periodOrgMap[pk] = &periodOrgAgg{users: make(map[string]bool)}
			}
			pa := periodOrgMap[pk]
			for uid := range da.users {
				pa.users[uid] = true
			}
			pa.taskCount += da.taskCount
			pa.commitCount += da.commitCount
			pa.taskDiffLines += da.taskDiffLines
			pa.commitDiffLines += da.commitDiffLines
			pa.upTokens += da.upTokens
			pa.downTokens += da.downTokens
			pa.cost += da.cost
			pa.taskRealMin += da.taskRealMin
			pa.taskAncientMin += da.taskAncientMin
			pa.commitRealMin += da.commitRealMin
			pa.commitAncientMin += da.commitAncientMin
		}
	}

	// 收集并排序 periods
	var allPeriods []string
	for p := range periodSet {
		allPeriods = append(allPeriods, p)
	}
	sort.Strings(allPeriods)

	// 收集并排序 orgNames
	var orgNames []string
	for orgName := range groupMap {
		orgNames = append(orgNames, orgName)
	}
	sort.Strings(orgNames)

	// 构建 series：每个 org 一条记录，包含按 period 排列的数据点
	series := make([]gin.H, 0, len(orgNames))
	for _, orgName := range orgNames {
		points := make([]gin.H, 0, len(allPeriods))
		for _, period := range allPeriods {
			pk := periodOrgKey{period: period, orgName: orgName}
			pa := periodOrgMap[pk]
			var taskEffRatio, commitEffRatio float64
			var userCount, taskCount, commitCount, taskDiff, commitDiff int
			var upTok, downTok int64
			var cost float64
			if pa != nil {
				userCount = len(pa.users)
				taskCount = pa.taskCount
				commitCount = pa.commitCount
				taskDiff = pa.taskDiffLines
				commitDiff = pa.commitDiffLines
				upTok = pa.upTokens
				downTok = pa.downTokens
				cost = pa.cost
				if pa.taskRealMin > 0 {
					taskEffRatio = math.Round(pa.taskAncientMin / pa.taskRealMin * 100)
				}
				if pa.commitRealMin > 0 {
					commitEffRatio = math.Round(pa.commitAncientMin / pa.commitRealMin * 100)
				}
			}
			points = append(points, gin.H{
				"period":                  period,
				"user_count":              userCount,
				"task_count":              taskCount,
				"commit_count":            commitCount,
				"task_diff_lines":         taskDiff,
				"commit_diff_lines":       commitDiff,
				"task_efficiency_ratio":   taskEffRatio,
				"commit_efficiency_ratio": commitEffRatio,
				"total_tokens":            upTok + downTok,
				"total_cost":              cost,
			})
		}
		series = append(series, gin.H{
			"org_name": orgName,
			"periods":  allPeriods,
			"points":   points,
		})
	}

	c.JSON(http.StatusOK, gin.H{"data": data, "series": series, "periods": allPeriods})
}

// getOrgDetailV2 GET /api/v2/orgs/detail
// 重构：复用 user_productivity 聚合，与 getGroupDetailV2 逻辑对齐，并支持 granularity
func getOrgDetailV2(c *gin.Context) {
	orgPath := c.Query("org_path")
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	granularity := c.Query("granularity")
	if granularity == "" {
		granularity = "day"
	}

	if orgPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "org_path 不能为空"})
		return
	}

	var startTime, endTime string
	if startDate != "" {
		startT, err := parseDateParam(startDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "startDate 格式错误: " + err.Error()})
			return
		}
		startTime = startT.Format(time.RFC3339)
	}
	if endDate != "" {
		endT, err := parseDateParam(endDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "endDate 格式错误: " + err.Error()})
			return
		}
		endTime = endT.Add(23*time.Hour + 59*time.Minute + 59*time.Second).Format(time.RFC3339)
	}

	// 解析 org_path，按 "/" 分级匹配 orgMappings
	parts := strings.Split(orgPath, "/")
	var matchedUsers []*OrgMapping
	for _, m := range orgMappings {
		match := true
		for i, p := range parts {
			level := fmt.Sprintf("org%d", i+1)
			if getOrgValue(m, level) != p {
				match = false
				break
			}
		}
		if match {
			matchedUsers = append(matchedUsers, m)
		}
	}

	empty := gin.H{
		"org_path":    orgPath,
		"summary":     gin.H{},
		"commits":     []gin.H{},
		"tasks":       []gin.H{},
		"members":     []gin.H{},
		"granularity": granularity,
	}
	if len(matchedUsers) == 0 {
		c.JSON(http.StatusOK, empty)
		return
	}

	// 聚合结构
	type dailyAgg struct {
		taskDiffLines    int
		commitDiffLines  int
		taskRealMin      float64
		taskAncientMin   float64
		commitRealMin    float64
		commitAncientMin float64
		upTokens         int64
		downTokens       int64
		cost             float64
	}
	type memberAgg struct {
		userID                 string
		userName               string
		org1, org2, org3, org4 string
		taskDiffLines          int
		commitDiffLines        int
		taskRealMin            float64
		taskAncientMin         float64
		commitRealMin          float64
		commitAncientMin       float64
		upTokens               int64
		downTokens             int64
		cost                   float64
	}

	dailyMap := make(map[string]*dailyAgg)
	memberMap := make(map[string]*memberAgg)
	for _, u := range matchedUsers {
		memberMap[u.UserID] = &memberAgg{
			userID: u.UserID, userName: u.UserName,
			org1: u.Org1, org2: u.Org2, org3: u.Org3, org4: u.Org4,
		}
	}

	// 从 user_productivity 逐用户聚合
	for _, u := range matchedUsers {
		daily, err := ListUserProductivity(statDB, u.UserID, startTime, endTime, 1, 100000)
		if err != nil {
			continue
		}
		for _, d := range daily {
			if d.CreateTime == nil {
				continue
			}
			dateStr := d.CreateTime.Format("2006-01-02")
			if _, ok := dailyMap[dateStr]; !ok {
				dailyMap[dateStr] = &dailyAgg{}
			}
			da := dailyMap[dateStr]
			ma := memberMap[u.UserID]

			if d.TaskDiffLines != nil {
				da.taskDiffLines += *d.TaskDiffLines
				ma.taskDiffLines += *d.TaskDiffLines
			}
			if d.CommitDiffLines != nil {
				da.commitDiffLines += *d.CommitDiffLines
				ma.commitDiffLines += *d.CommitDiffLines
			}
			if d.TaskRealMinutes != nil {
				da.taskRealMin += *d.TaskRealMinutes
				ma.taskRealMin += *d.TaskRealMinutes
			}
			if d.TaskAncientMinutes != nil {
				da.taskAncientMin += *d.TaskAncientMinutes
				ma.taskAncientMin += *d.TaskAncientMinutes
			}
			if d.CommitRealMinutes != nil {
				da.commitRealMin += *d.CommitRealMinutes
				ma.commitRealMin += *d.CommitRealMinutes
			}
			if d.CommitAncientMinutes != nil {
				da.commitAncientMin += *d.CommitAncientMinutes
				ma.commitAncientMin += *d.CommitAncientMinutes
			}
			if d.UpstreamTokens != nil {
				da.upTokens += *d.UpstreamTokens
				ma.upTokens += *d.UpstreamTokens
			}
			if d.DownstreamTokens != nil {
				da.downTokens += *d.DownstreamTokens
				ma.downTokens += *d.DownstreamTokens
			}
			if d.Cost != nil {
				da.cost += *d.Cost
				ma.cost += *d.Cost
			}
		}
	}

	// 将 dailyMap 转为 UserProductivity 切片，复用 aggregateDailyByGranularity
	var dates []string
	for date := range dailyMap {
		dates = append(dates, date)
	}
	sort.Strings(dates)

	dailySlice := make([]UserProductivity, 0, len(dates))
	for _, date := range dates {
		da := dailyMap[date]
		t, _ := time.Parse("2006-01-02", date)
		dailySlice = append(dailySlice, UserProductivity{
			CreateTime:           ptrTime(t),
			TaskDiffLines:        ptrInt(da.taskDiffLines),
			CommitDiffLines:      ptrInt(da.commitDiffLines),
			TaskRealMinutes:      ptrFloat64(da.taskRealMin),
			TaskAncientMinutes:   ptrFloat64(da.taskAncientMin),
			CommitRealMinutes:    ptrFloat64(da.commitRealMin),
			CommitAncientMinutes: ptrFloat64(da.commitAncientMin),
			UpstreamTokens:       ptrInt64(da.upTokens),
			DownstreamTokens:     ptrInt64(da.downTokens),
			Cost:                 ptrFloat64(da.cost),
		})
	}
	commitsList, tasksList := aggregateDailyByGranularity(dailySlice, granularity)

	// 汇总 summary
	var sumTaskDiffLines, sumCommitDiffLines int
	var sumTaskRealMin, sumTaskAncientMin, sumCommitRealMin, sumCommitAncientMin float64
	var sumUpTokens, sumDownTokens int64
	var sumCost float64
	for _, da := range dailyMap {
		sumTaskDiffLines += da.taskDiffLines
		sumCommitDiffLines += da.commitDiffLines
		sumTaskRealMin += da.taskRealMin
		sumTaskAncientMin += da.taskAncientMin
		sumCommitRealMin += da.commitRealMin
		sumCommitAncientMin += da.commitAncientMin
		sumUpTokens += da.upTokens
		sumDownTokens += da.downTokens
		sumCost += da.cost
	}
	var sumTaskEffRatio, sumCommitEffRatio float64
	if sumTaskRealMin > 0 {
		sumTaskEffRatio = math.Round(sumTaskAncientMin / sumTaskRealMin * 100)
	}
	if sumCommitRealMin > 0 {
		sumCommitEffRatio = math.Round(sumCommitAncientMin / sumCommitRealMin * 100)
	}
	summary := gin.H{
		"user_count":              len(matchedUsers),
		"task_diff_lines":         sumTaskDiffLines,
		"commit_diff_lines":       sumCommitDiffLines,
		"task_real_minutes":       sumTaskRealMin,
		"task_ancient_minutes":    sumTaskAncientMin,
		"task_efficiency_ratio":   sumTaskEffRatio,
		"commit_real_minutes":     sumCommitRealMin,
		"commit_ancient_minutes":  sumCommitAncientMin,
		"commit_efficiency_ratio": sumCommitEffRatio,
		"upstream_tokens":         sumUpTokens,
		"downstream_tokens":       sumDownTokens,
		"cost":                    sumCost,
	}

	// 构建 members 列表
	membersResult := make([]gin.H, 0, len(memberMap))
	for _, ma := range memberMap {
		var taskEffRatio, commitEffRatio float64
		if ma.taskRealMin > 0 {
			taskEffRatio = math.Round(ma.taskAncientMin / ma.taskRealMin * 100)
		}
		if ma.commitRealMin > 0 {
			commitEffRatio = math.Round(ma.commitAncientMin / ma.commitRealMin * 100)
		}
		membersResult = append(membersResult, gin.H{
			"user_id":                 ma.userID,
			"user_name":               ma.userName,
			"org1":                    ma.org1,
			"org2":                    ma.org2,
			"org3":                    ma.org3,
			"org4":                    ma.org4,
			"task_diff_lines":         ma.taskDiffLines,
			"commit_diff_lines":       ma.commitDiffLines,
			"task_real_minutes":       ma.taskRealMin,
			"task_ancient_minutes":    ma.taskAncientMin,
			"task_efficiency_ratio":   taskEffRatio,
			"commit_real_minutes":     ma.commitRealMin,
			"commit_ancient_minutes":  ma.commitAncientMin,
			"commit_efficiency_ratio": commitEffRatio,
			"upstream_tokens":         ma.upTokens,
			"downstream_tokens":       ma.downTokens,
			"cost":                    ma.cost,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"org_path":    orgPath,
		"summary":     summary,
		"commits":     commitsList,
		"tasks":       tasksList,
		"members":     membersResult,
		"granularity": granularity,
	})
}

// getGroupDetailV2 GET /api/v2/group
func getGroupDetailV2(c *gin.Context) {
	org1 := c.Query("org1")
	org2 := c.Query("org2")
	org3 := c.Query("org3")
	org4 := c.Query("org4")
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")

	var startTime, endTime string
	if startDate != "" {
		startT, err := parseDateParam(startDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "startDate 格式错误: " + err.Error()})
			return
		}
		startTime = startT.Format(time.RFC3339)
	}
	if endDate != "" {
		endT, err := parseDateParam(endDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "endDate 格式错误: " + err.Error()})
			return
		}
		endTime = endT.Add(23*time.Hour + 59*time.Minute + 59*time.Second).Format(time.RFC3339)
	}

	// 按 org1/org2/org3/org4 从 orgMappings 筛选用户，逐级匹配非空参数
	var matchedUsers []*OrgMapping
	for _, m := range orgMappings {
		match := true
		if org1 != "" && m.Org1 != org1 {
			match = false
		}
		if org2 != "" && m.Org2 != org2 {
			match = false
		}
		if org3 != "" && m.Org3 != org3 {
			match = false
		}
		if org4 != "" && m.Org4 != org4 {
			match = false
		}
		if match {
			matchedUsers = append(matchedUsers, m)
		}
	}

	// 拼接 org_path（过滤空值后用" / "连接）
	var orgParts []string
	for _, v := range []string{org1, org2, org3, org4} {
		if v != "" {
			orgParts = append(orgParts, v)
		}
	}
	orgPath := strings.Join(orgParts, " / ")

	if len(matchedUsers) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"org_path": orgPath,
			"summary":  gin.H{},
			"daily":    []gin.H{},
			"members":  []gin.H{},
		})
		return
	}

	type dailyAgg struct {
		taskDiffLines    int
		commitDiffLines  int
		taskRealMin      float64
		taskAncientMin   float64
		commitRealMin    float64
		commitAncientMin float64
		upTokens         int64
		downTokens       int64
		cost             float64
	}
	dailyMap := make(map[string]*dailyAgg)

	type memberAgg struct {
		userID                 string
		userName               string
		org1, org2, org3, org4 string
		taskDiffLines          int
		commitDiffLines        int
		taskRealMin            float64
		taskAncientMin         float64
		commitRealMin          float64
		commitAncientMin       float64
		cost                   float64
	}
	memberMap := make(map[string]*memberAgg)

	for _, u := range matchedUsers {
		memberMap[u.UserID] = &memberAgg{
			userID:   u.UserID,
			userName: u.UserName,
			org1:     u.Org1,
			org2:     u.Org2,
			org3:     u.Org3,
			org4:     u.Org4,
		}
	}

	for _, u := range matchedUsers {
		daily, err := ListUserProductivity(statDB, u.UserID, startTime, endTime, 1, 100000)
		if err != nil {
			continue
		}

		for _, d := range daily {
			if d.CreateTime == nil {
				continue
			}
			dateStr := d.CreateTime.Format("2006-01-02")

			if _, ok := dailyMap[dateStr]; !ok {
				dailyMap[dateStr] = &dailyAgg{}
			}
			da := dailyMap[dateStr]
			ma := memberMap[u.UserID]

			if d.TaskDiffLines != nil {
				da.taskDiffLines += *d.TaskDiffLines
				ma.taskDiffLines += *d.TaskDiffLines
			}
			if d.CommitDiffLines != nil {
				da.commitDiffLines += *d.CommitDiffLines
				ma.commitDiffLines += *d.CommitDiffLines
			}
			if d.TaskRealMinutes != nil {
				da.taskRealMin += *d.TaskRealMinutes
				ma.taskRealMin += *d.TaskRealMinutes
			}
			if d.TaskAncientMinutes != nil {
				da.taskAncientMin += *d.TaskAncientMinutes
				ma.taskAncientMin += *d.TaskAncientMinutes
			}
			if d.CommitRealMinutes != nil {
				da.commitRealMin += *d.CommitRealMinutes
				ma.commitRealMin += *d.CommitRealMinutes
			}
			if d.CommitAncientMinutes != nil {
				da.commitAncientMin += *d.CommitAncientMinutes
				ma.commitAncientMin += *d.CommitAncientMinutes
			}
			if d.UpstreamTokens != nil {
				da.upTokens += *d.UpstreamTokens
			}
			if d.DownstreamTokens != nil {
				da.downTokens += *d.DownstreamTokens
			}
			if d.Cost != nil {
				da.cost += *d.Cost
				ma.cost += *d.Cost
			}
		}
	}

	// 构建 daily 数组（按日期排序）
	var dates []string
	for date := range dailyMap {
		dates = append(dates, date)
	}
	sort.Strings(dates)

	dailyResult := make([]gin.H, 0, len(dates))
	for _, date := range dates {
		da := dailyMap[date]
		var taskEffRatio, commitEffRatio float64
		if da.taskRealMin > 0 {
			taskEffRatio = math.Round(da.taskAncientMin / da.taskRealMin * 100)
		}
		if da.commitRealMin > 0 {
			commitEffRatio = math.Round(da.commitAncientMin / da.commitRealMin * 100)
		}
		dailyResult = append(dailyResult, gin.H{
			"date":                    date,
			"task_diff_lines":         da.taskDiffLines,
			"commit_diff_lines":       da.commitDiffLines,
			"task_real_minutes":       da.taskRealMin,
			"task_ancient_minutes":    da.taskAncientMin,
			"task_efficiency_ratio":   taskEffRatio,
			"commit_real_minutes":     da.commitRealMin,
			"commit_ancient_minutes":  da.commitAncientMin,
			"commit_efficiency_ratio": commitEffRatio,
			"upstream_tokens":         da.upTokens,
			"downstream_tokens":       da.downTokens,
			"cost":                    da.cost,
		})
	}

	// 汇总 summary
	var sumTaskDiffLines, sumCommitDiffLines int
	var sumTaskRealMin, sumTaskAncientMin, sumCommitRealMin, sumCommitAncientMin float64
	var sumUpTokens, sumDownTokens int64
	var sumCost float64
	for _, da := range dailyMap {
		sumTaskDiffLines += da.taskDiffLines
		sumCommitDiffLines += da.commitDiffLines
		sumTaskRealMin += da.taskRealMin
		sumTaskAncientMin += da.taskAncientMin
		sumCommitRealMin += da.commitRealMin
		sumCommitAncientMin += da.commitAncientMin
		sumUpTokens += da.upTokens
		sumDownTokens += da.downTokens
		sumCost += da.cost
	}
	var sumTaskEffRatio, sumCommitEffRatio float64
	if sumTaskRealMin > 0 {
		sumTaskEffRatio = math.Round(sumTaskAncientMin / sumTaskRealMin * 100)
	}
	if sumCommitRealMin > 0 {
		sumCommitEffRatio = math.Round(sumCommitAncientMin / sumCommitRealMin * 100)
	}

	summary := gin.H{
		"task_diff_lines":         sumTaskDiffLines,
		"commit_diff_lines":       sumCommitDiffLines,
		"task_real_minutes":       sumTaskRealMin,
		"task_ancient_minutes":    sumTaskAncientMin,
		"task_efficiency_ratio":   sumTaskEffRatio,
		"commit_real_minutes":     sumCommitRealMin,
		"commit_ancient_minutes":  sumCommitAncientMin,
		"commit_efficiency_ratio": sumCommitEffRatio,
		"upstream_tokens":         sumUpTokens,
		"downstream_tokens":       sumDownTokens,
		"cost":                    sumCost,
	}

	// 构建 members 列表
	membersResult := make([]gin.H, 0, len(memberMap))
	for _, ma := range memberMap {
		var taskEffRatio, commitEffRatio float64
		if ma.taskRealMin > 0 {
			taskEffRatio = math.Round(ma.taskAncientMin / ma.taskRealMin * 100)
		}
		if ma.commitRealMin > 0 {
			commitEffRatio = math.Round(ma.commitAncientMin / ma.commitRealMin * 100)
		}
		membersResult = append(membersResult, gin.H{
			"user_id":                 ma.userID,
			"user_name":               ma.userName,
			"org1":                    ma.org1,
			"org2":                    ma.org2,
			"org3":                    ma.org3,
			"org4":                    ma.org4,
			"task_diff_lines":         ma.taskDiffLines,
			"commit_diff_lines":       ma.commitDiffLines,
			"task_efficiency_ratio":   taskEffRatio,
			"commit_efficiency_ratio": commitEffRatio,
			"cost":                    ma.cost,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"org_path": orgPath,
		"summary":  summary,
		"daily":    dailyResult,
		"members":  membersResult,
	})
}
