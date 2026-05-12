package main

import (
	"fmt"
	"kanban/core/models"
	"kanban/core/utils"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type OrgDataItem struct {
	OrgName               string  `json:"org_name"`
	UserCount             int     `json:"user_count"`
	TotalTokens           int64   `json:"total_tokens"`
	TotalCost             float64 `json:"total_cost"`
	TaskCount             int     `json:"task_count"`
	TaskDiffLines         int     `json:"task_diff_lines"`
	TaskEfficiencyRatio   float64 `json:"task_efficiency_ratio"`
	CommitCount           int     `json:"commit_count"`
	CommitDiffLines       int     `json:"commit_diff_lines"`
	CommitEfficiencyRatio float64 `json:"commit_efficiency_ratio"`
}

type OrgSeriesPoint struct {
	Period                string  `json:"period"`
	UserCount             int     `json:"user_count"`
	TotalTokens           int64   `json:"total_tokens"`
	TotalCost             float64 `json:"total_cost"`
	TaskCount             int     `json:"task_count"`
	TaskDiffLines         int     `json:"task_diff_lines"`
	TaskEfficiencyRatio   float64 `json:"task_efficiency_ratio"`
	CommitCount           int     `json:"commit_count"`
	CommitDiffLines       int     `json:"commit_diff_lines"`
	CommitEfficiencyRatio float64 `json:"commit_efficiency_ratio"`
}

type OrgSeriesItem struct {
	OrgName string           `json:"org_name"`
	Periods []string         `json:"periods"`
	Points  []OrgSeriesPoint `json:"points"`
}

type OrgListResponse struct {
	Data    []OrgDataItem   `json:"data"`
	Series  []OrgSeriesItem `json:"series"`
	Periods []string        `json:"periods,omitempty"`
}

type OrgSummary struct {
	UserCount             int     `json:"user_count"`
	TaskDiffLines         int     `json:"task_diff_lines"`
	CommitDiffLines       int     `json:"commit_diff_lines"`
	TaskRealMinutes       float64 `json:"task_real_minutes"`
	TaskAncientMinutes    float64 `json:"task_ancient_minutes"`
	TaskEfficiencyRatio   float64 `json:"task_efficiency_ratio"`
	CommitRealMinutes     float64 `json:"commit_real_minutes"`
	CommitAncientMinutes  float64 `json:"commit_ancient_minutes"`
	CommitEfficiencyRatio float64 `json:"commit_efficiency_ratio"`
	UpstreamTokens        int64   `json:"upstream_tokens"`
	DownstreamTokens      int64   `json:"downstream_tokens"`
	Cost                  float64 `json:"cost"`
}

type OrgMemberItem struct {
	UserId                string  `json:"user_id"`
	UserName              string  `json:"user_name"`
	Org1                  string  `json:"org1"`
	Org2                  string  `json:"org2"`
	Org3                  string  `json:"org3"`
	Org4                  string  `json:"org4"`
	OrgDisplay            string  `json:"org_display"`
	TaskDiffLines         int     `json:"task_diff_lines"`
	CommitDiffLines       int     `json:"commit_diff_lines"`
	TaskRealMinutes       float64 `json:"task_real_minutes"`
	TaskAncientMinutes    float64 `json:"task_ancient_minutes"`
	TaskEfficiencyRatio   float64 `json:"task_efficiency_ratio"`
	CommitRealMinutes     float64 `json:"commit_real_minutes"`
	CommitAncientMinutes  float64 `json:"commit_ancient_minutes"`
	CommitEfficiencyRatio float64 `json:"commit_efficiency_ratio"`
	UpstreamTokens        int64   `json:"upstream_tokens"`
	DownstreamTokens      int64   `json:"downstream_tokens"`
	Cost                  float64 `json:"cost"`
}

type CommitTimeSeriesItem struct {
	PeriodKey             string  `json:"period_key"`
	PeriodLabel           string  `json:"period_label"`
	CommitCount           int     `json:"commit_count"`
	CommitDiffLines       int     `json:"commit_diff_lines"`
	CommitRealMinutes     float64 `json:"commit_real_minutes"`
	CommitAncientMinutes  float64 `json:"commit_ancient_minutes"`
	CommitEfficiencyRatio float64 `json:"commit_efficiency_ratio"`
	UpstreamTokens        int64   `json:"upstream_tokens"`
	DownstreamTokens      int64   `json:"downstream_tokens"`
	Cost                  float64 `json:"cost"`
}

type TaskTimeSeriesItem struct {
	PeriodKey           string  `json:"period_key"`
	PeriodLabel         string  `json:"period_label"`
	TaskCount           int     `json:"task_count"`
	TaskDiffLines       int     `json:"task_diff_lines"`
	TaskRealMinutes     float64 `json:"task_real_minutes"`
	TaskAncientMinutes  float64 `json:"task_ancient_minutes"`
	TaskEfficiencyRatio float64 `json:"task_efficiency_ratio"`
	UpstreamTokens      int64   `json:"upstream_tokens"`
	DownstreamTokens    int64   `json:"downstream_tokens"`
	Cost                float64 `json:"cost"`
}

type OrgDetailResponse struct {
	OrgPath     string                 `json:"org_path"`
	Summary     OrgSummary             `json:"summary"`
	Commits     []CommitTimeSeriesItem `json:"commits"`
	Tasks       []TaskTimeSeriesItem   `json:"tasks"`
	Members     []OrgMemberItem        `json:"members"`
	Granularity string                 `json:"granularity"`
}

type GroupSummary struct {
	TaskDiffLines         int     `json:"task_diff_lines"`
	CommitDiffLines       int     `json:"commit_diff_lines"`
	TaskRealMinutes       float64 `json:"task_real_minutes"`
	TaskAncientMinutes    float64 `json:"task_ancient_minutes"`
	TaskEfficiencyRatio   float64 `json:"task_efficiency_ratio"`
	CommitRealMinutes     float64 `json:"commit_real_minutes"`
	CommitAncientMinutes  float64 `json:"commit_ancient_minutes"`
	CommitEfficiencyRatio float64 `json:"commit_efficiency_ratio"`
	UpstreamTokens        int64   `json:"upstream_tokens"`
	DownstreamTokens      int64   `json:"downstream_tokens"`
	Cost                  float64 `json:"cost"`
}

type DailyDataItem struct {
	Date                  string  `json:"date"`
	TaskDiffLines         int     `json:"task_diff_lines"`
	CommitDiffLines       int     `json:"commit_diff_lines"`
	TaskRealMinutes       float64 `json:"task_real_minutes"`
	TaskAncientMinutes    float64 `json:"task_ancient_minutes"`
	TaskEfficiencyRatio   float64 `json:"task_efficiency_ratio"`
	CommitRealMinutes     float64 `json:"commit_real_minutes"`
	CommitAncientMinutes  float64 `json:"commit_ancient_minutes"`
	CommitEfficiencyRatio float64 `json:"commit_efficiency_ratio"`
	UpstreamTokens        int64   `json:"upstream_tokens"`
	DownstreamTokens      int64   `json:"downstream_tokens"`
	Cost                  float64 `json:"cost"`
}

type GroupMemberItem struct {
	UserId                string  `json:"user_id"`
	UserName              string  `json:"user_name"`
	Org1                  string  `json:"org1"`
	Org2                  string  `json:"org2"`
	Org3                  string  `json:"org3"`
	Org4                  string  `json:"org4"`
	OrgDisplay            string  `json:"org_display"`
	TaskDiffLines         int     `json:"task_diff_lines"`
	CommitDiffLines       int     `json:"commit_diff_lines"`
	TaskEfficiencyRatio   float64 `json:"task_efficiency_ratio"`
	CommitEfficiencyRatio float64 `json:"commit_efficiency_ratio"`
	Cost                  float64 `json:"cost"`
}

type GroupDetailResponse struct {
	OrgPath string            `json:"org_path"`
	Summary GroupSummary      `json:"summary"`
	Daily   []DailyDataItem   `json:"daily"`
	Members []GroupMemberItem `json:"members"`
}

// orgMappings 全局组织映射表，key=user_id
var orgMappings map[string]*models.UserOrg

func getOrgDisplay(org1, org2, org3, org4, org5, org6, org7, org8, org9 string) string {
	parts := []string{}
	for _, v := range []string{org1, org2, org3, org4, org5, org6, org7, org8, org9} {
		if v != "" {
			parts = append(parts, v)
		}
	}
	return strings.Join(parts, "/")
}

// getOrgValue 根据 level 获取 models.UserOrg 中对应的 org 值
func getOrgValue(m *models.UserOrg, level string) string {
	switch level {
	case "org1":
		return m.Org1
	case "org2":
		return m.Org2
	case "org3":
		return m.Org3
	case "org4":
		return m.Org4
	case "org5":
		return m.Org5
	case "org6":
		return m.Org6
	case "org7":
		return m.Org7
	case "org8":
		return m.Org8
	case "org9":
		return m.Org9
	default:
		return ""
	}
}

// filterUsersByParent 根据 parent 路径筛选用户
func filterUsersByParent(parent string) []*models.UserOrg {
	var result []*models.UserOrg
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
//
// @Summary 获取组织列表
// @Description 按组织层级查询各组织的效率汇总数据，可选时间序列
// @Tags Orgs
// @Produce json
// @Param level query string false "组织层级(org1/org2/org3/org4)" default(org1)
// @Param parent query string false "父级组织路径(用/分隔)"
// @Param startDate query string false "开始日期(YYYYMMDD)"
// @Param endDate query string false "结束日期(YYYYMMDD)"
// @Param granularity query string false "时间粒度(day/week/month/year)"
// @Success 200 {object} OrgListResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/orgs [get]
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
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "startDate 格式错误: " + err.Error()})
			return
		}
		startTime = startT.Format(time.RFC3339)
	}
	if endDate != "" {
		endT, err := parseDateParam(endDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "endDate 格式错误: " + err.Error()})
			return
		}
		endTime = endT.Add(23*time.Hour + 59*time.Minute + 59*time.Second).Format(time.RFC3339)
	}
	if orgMappings == nil {
		maps, err := LoadUserOrgs(statDB)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			return
		}
		orgMappings = maps
	}

	// 从 orgMappings 筛选符合 parent 条件的用户
	users := filterUsersByParent(parent)
	if len(users) == 0 {
		c.JSON(http.StatusOK, OrgListResponse{Data: []OrgDataItem{}, Series: []OrgSeriesItem{}})
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
			g.userIDs = append(g.userIDs, u.UserId)
		} else {
			groupMap[orgName] = &orgGroup{orgName: orgName, userIDs: []string{u.UserId}}
		}
	}

	if len(groupMap) == 0 {
		c.JSON(http.StatusOK, OrgListResponse{Data: []OrgDataItem{}, Series: []OrgSeriesItem{}})
		return
	}

	// 收集所有 user_id
	var allUserIDs []string
	for _, g := range groupMap {
		allUserIDs = append(allUserIDs, g.userIDs...)
	}

	userAggMap := make(map[string]*userProdAggRow)

	prodRows, err := QueryUserProdAggForIDs(statDB, allUserIDs, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "查询 user_productivity 聚合失败: " + err.Error()})
		return
	}

	for _, row := range prodRows {
		userAggMap[row.UserId] = &row
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
		taskEffRatio     float64
		commitEffRatio   float64
	}
	orgAggMap := make(map[string]*orgAgg)
	for orgName, g := range groupMap {
		oa := &orgAgg{userCount: len(g.userIDs)}
		for _, uid := range g.userIDs {
			if ua, ok := userAggMap[uid]; ok {
				oa.taskCount += ua.TaskCount
				oa.commitCount += ua.CommitCount
				oa.taskDiffLines += ua.TaskDiffLines
				oa.commitDiffLines += ua.CommitDiffLines
				oa.upTokens += ua.UpstreamTokens
				oa.downTokens += ua.DownstreamTokens
				oa.cost += ua.Cost
				oa.taskRealMin += ua.TaskRealMinutes
				oa.taskAncientMin += ua.TaskAncientMinutes
				oa.commitRealMin += ua.CommitRealMinutes
				oa.commitAncientMin += ua.CommitAncientMinutes
			}
		}
		if oa.taskRealMin > 0 {
			oa.taskEffRatio = utils.CalcEfficiencyRatio(oa.taskAncientMin, oa.taskRealMin)
		}
		if oa.commitRealMin > 0 {
			oa.commitEffRatio = utils.CalcEfficiencyRatio(oa.commitAncientMin, oa.commitRealMin)
		}
		orgAggMap[orgName] = oa
	}

	data := make([]OrgDataItem, 0, len(orgAggMap))
	for orgName, oa := range orgAggMap {
		data = append(data, OrgDataItem{
			OrgName: orgName, UserCount: oa.userCount,
			TaskCount: oa.taskCount, CommitCount: oa.commitCount,
			TaskDiffLines: oa.taskDiffLines, CommitDiffLines: oa.commitDiffLines,
			TaskEfficiencyRatio:   oa.taskEffRatio,
			CommitEfficiencyRatio: oa.commitEffRatio,
			TotalTokens:           oa.upTokens + oa.downTokens,
			TotalCost:             oa.cost,
		})
	}

	// 如果不需要时间序列，直接返回
	if granularity == "" {
		c.JSON(http.StatusOK, OrgListResponse{Data: data, Series: []OrgSeriesItem{}})
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

	sRows, err := QueryUserProdTimeSeries(statDB, allUserIDs, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusOK, OrgListResponse{Data: data, Series: []OrgSeriesItem{}})
		return
	}

	for _, row := range sRows {
		orgName, ok := uidToOrg[row.UserId]
		if !ok {
			continue
		}
		dateStr := row.CreateTime.Format("2006-01-02")
		key := dayOrgKey{date: dateStr, orgName: orgName}
		if _, ok := dayOrgMap[key]; !ok {
			dayOrgMap[key] = &dayOrgAgg{users: make(map[string]bool)}
		}
		da := dayOrgMap[key]
		da.users[row.UserId] = true
		da.taskCount += row.TaskCount
		da.commitCount += row.CommitCount
		da.taskDiffLines += row.TaskDiffLines
		da.commitDiffLines += row.CommitDiffLines
		da.upTokens += row.UpstreamTokens
		da.downTokens += row.DownstreamTokens
		da.cost += row.Cost
		da.taskRealMin += row.TaskRealMinutes
		da.taskAncientMin += row.TaskAncientMinutes
		da.commitRealMin += row.CommitRealMinutes
		da.commitAncientMin += row.CommitAncientMinutes
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
	series := make([]OrgSeriesItem, 0, len(orgNames))
	for _, orgName := range orgNames {
		points := make([]OrgSeriesPoint, 0, len(allPeriods))
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
				taskEffRatio = utils.CalcEfficiencyRatio(pa.taskAncientMin, pa.taskRealMin)
				commitEffRatio = utils.CalcEfficiencyRatio(pa.commitAncientMin, pa.commitRealMin)
			}
			points = append(points, OrgSeriesPoint{
				Period:                period,
				UserCount:             userCount,
				TotalTokens:           upTok + downTok,
				TotalCost:             cost,
				TaskCount:             taskCount,
				TaskDiffLines:         taskDiff,
				TaskEfficiencyRatio:   taskEffRatio,
				CommitCount:           commitCount,
				CommitDiffLines:       commitDiff,
				CommitEfficiencyRatio: commitEffRatio,
			})
		}
		series = append(series, OrgSeriesItem{
			OrgName: orgName,
			Periods: allPeriods,
			Points:  points,
		})
	}

	c.JSON(http.StatusOK, OrgListResponse{Data: data, Series: series, Periods: allPeriods})
}

// getOrgDetailV2 GET /api/v2/orgs/detail
// 重构：复用 user_productivity 聚合，与 getGroupDetailV2 逻辑对齐，并支持 granularity
// @Summary 获取组织详情
// @Description 根据组织路径查询组织详情，包含汇总数据、提交/任务时间序列、成员列表
// @Tags Orgs
// @Produce json
// @Param org_path query string true "组织路径(用/分隔，如 org1/org2)"
// @Param startDate query string false "开始日期(YYYYMMDD)"
// @Param endDate query string false "结束日期(YYYYMMDD)"
// @Param granularity query string false "时间粒度(day/week/month/year)" default(day)
// @Success 200 {object} OrgDetailResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/orgs/detail [get]
func getOrgDetailV2(c *gin.Context) {
	orgPath := c.Query("org_path")
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	granularity := c.Query("granularity")
	if granularity == "" {
		granularity = "day"
	}

	if orgPath == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "org_path 不能为空"})
		return
	}

	var startTime, endTime string
	if startDate != "" {
		startT, err := parseDateParam(startDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "startDate 格式错误: " + err.Error()})
			return
		}
		startTime = startT.Format(time.RFC3339)
	}
	if endDate != "" {
		endT, err := parseDateParam(endDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "endDate 格式错误: " + err.Error()})
			return
		}
		endTime = endT.Add(23*time.Hour + 59*time.Minute + 59*time.Second).Format(time.RFC3339)
	}
	if orgMappings == nil {
		maps, err := LoadUserOrgs(statDB)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			return
		}
		orgMappings = maps
	}

	// 解析 org_path，按 "/" 分级匹配 orgMappings
	parts := strings.Split(orgPath, "/")
	var matchedUsers []*models.UserOrg
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

	empty := OrgDetailResponse{
		OrgPath:     orgPath,
		Summary:     OrgSummary{},
		Commits:     []CommitTimeSeriesItem{},
		Tasks:       []TaskTimeSeriesItem{},
		Members:     []OrgMemberItem{},
		Granularity: granularity,
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
		memberMap[u.UserId] = &memberAgg{
			userID: u.UserId, userName: u.UserName,
			org1: u.Org1, org2: u.Org2, org3: u.Org3, org4: u.Org4,
		}
	}

	// 从 user_productivity 逐用户聚合
	for _, u := range matchedUsers {
		daily, err := ListUserProductivity(statDB, u.UserId, startTime, endTime, 1, 100000)
		if err != nil {
			continue
		}
		for _, d := range daily {
			if d.CreateTime.IsZero() {
				continue
			}
			dateStr := d.CreateTime.Format("2006-01-02")
			if _, ok := dailyMap[dateStr]; !ok {
				dailyMap[dateStr] = &dailyAgg{}
			}
			da := dailyMap[dateStr]
			ma := memberMap[u.UserId]

			da.taskDiffLines += d.TaskDiffLines
			ma.taskDiffLines += d.TaskDiffLines
			da.commitDiffLines += d.CommitDiffLines
			ma.commitDiffLines += d.CommitDiffLines
			da.taskRealMin += d.TaskRealMinutes
			ma.taskRealMin += d.TaskRealMinutes
			da.taskAncientMin += d.TaskAncientMinutes
			ma.taskAncientMin += d.TaskAncientMinutes
			da.commitRealMin += d.CommitRealMinutes
			ma.commitRealMin += d.CommitRealMinutes
			da.commitAncientMin += d.CommitAncientMinutes
			ma.commitAncientMin += d.CommitAncientMinutes
			da.upTokens += d.UpstreamTokens
			ma.upTokens += d.UpstreamTokens
			da.downTokens += d.DownstreamTokens
			ma.downTokens += d.DownstreamTokens
			da.cost += d.Cost
			ma.cost += d.Cost
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
			CreateTime:           t,
			TaskDiffLines:        da.taskDiffLines,
			CommitDiffLines:      da.commitDiffLines,
			TaskRealMinutes:      da.taskRealMin,
			TaskAncientMinutes:   da.taskAncientMin,
			CommitRealMinutes:    da.commitRealMin,
			CommitAncientMinutes: da.commitAncientMin,
			UpstreamTokens:       da.upTokens,
			DownstreamTokens:     da.downTokens,
			Cost:                 da.cost,
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
	sumTaskEffRatio := utils.CalcEfficiencyRatio(sumTaskAncientMin, sumTaskRealMin)
	sumCommitEffRatio := utils.CalcEfficiencyRatio(sumCommitAncientMin, sumCommitRealMin)
	summary := OrgSummary{
		UserCount:             len(matchedUsers),
		TaskDiffLines:         sumTaskDiffLines,
		CommitDiffLines:       sumCommitDiffLines,
		TaskRealMinutes:       sumTaskRealMin,
		TaskAncientMinutes:    sumTaskAncientMin,
		TaskEfficiencyRatio:   sumTaskEffRatio,
		CommitRealMinutes:     sumCommitRealMin,
		CommitAncientMinutes:  sumCommitAncientMin,
		CommitEfficiencyRatio: sumCommitEffRatio,
		UpstreamTokens:        sumUpTokens,
		DownstreamTokens:      sumDownTokens,
		Cost:                  sumCost,
	}

	// 构建 members 列表
	membersResult := make([]OrgMemberItem, 0, len(memberMap))
	for _, ma := range memberMap {
		taskEffRatio := utils.CalcEfficiencyRatio(ma.taskAncientMin, ma.taskRealMin)
		commitEffRatio := utils.CalcEfficiencyRatio(ma.commitAncientMin, ma.commitRealMin)

		membersResult = append(membersResult, OrgMemberItem{
			UserId:                ma.userID,
			UserName:              ma.userName,
			Org1:                  ma.org1,
			Org2:                  ma.org2,
			Org3:                  ma.org3,
			Org4:                  ma.org4,
			OrgDisplay:            getOrgDisplay(ma.org1, ma.org2, ma.org3, ma.org4, "", "", "", "", ""),
			TaskDiffLines:         ma.taskDiffLines,
			CommitDiffLines:       ma.commitDiffLines,
			TaskRealMinutes:       ma.taskRealMin,
			TaskAncientMinutes:    ma.taskAncientMin,
			TaskEfficiencyRatio:   taskEffRatio,
			CommitRealMinutes:     ma.commitRealMin,
			CommitAncientMinutes:  ma.commitAncientMin,
			CommitEfficiencyRatio: commitEffRatio,
			UpstreamTokens:        ma.upTokens,
			DownstreamTokens:      ma.downTokens,
			Cost:                  ma.cost,
		})
	}

	c.JSON(http.StatusOK, OrgDetailResponse{
		OrgPath:     orgPath,
		Summary:     summary,
		Commits:     commitsList,
		Tasks:       tasksList,
		Members:     membersResult,
		Granularity: granularity,
	})
}

// getGroupDetailV2 GET /api/v2/group
// @Summary 获取组织分组详情
// @Description 根据org1-org4维度查询组织分组详情，包含汇总、每日数据和成员列表
// @Tags Orgs
// @Produce json
// @Param org1 query string false "一级组织"
// @Param org2 query string false "二级组织"
// @Param org3 query string false "三级组织"
// @Param org4 query string false "四级组织"
// @Param startDate query string false "开始日期(YYYYMMDD)"
// @Param endDate query string false "结束日期(YYYYMMDD)"
// @Success 200 {object} GroupDetailResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/group [get]
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
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "startDate 格式错误: " + err.Error()})
			return
		}
		startTime = startT.Format(time.RFC3339)
	}
	if endDate != "" {
		endT, err := parseDateParam(endDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "endDate 格式错误: " + err.Error()})
			return
		}
		endTime = endT.Add(23*time.Hour + 59*time.Minute + 59*time.Second).Format(time.RFC3339)
	}

	// 按组织层级从 orgMappings 筛选用户，逐级匹配非空参数
	var matchedUsers []*models.UserOrg
	orgParams := []string{org1, org2, org3, org4}
	for _, m := range orgMappings {
		match := true
		for i, p := range orgParams {
			if p != "" && getOrgValue(m, fmt.Sprintf("org%d", i+1)) != p {
				match = false
				break
			}
		}
		if match {
			matchedUsers = append(matchedUsers, m)
		}
	}

	// 拼接 org_path（过滤空值后用" / "连接）
	var orgParts []string
	for _, v := range orgParams {
		if v != "" {
			orgParts = append(orgParts, v)
		}
	}
	orgPath := strings.Join(orgParts, " / ")

	if len(matchedUsers) == 0 {
		c.JSON(http.StatusOK, GroupDetailResponse{
			OrgPath: orgPath,
			Summary: GroupSummary{},
			Daily:   []DailyDataItem{},
			Members: []GroupMemberItem{},
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
		memberMap[u.UserId] = &memberAgg{
			userID:   u.UserId,
			userName: u.UserName,
			org1:     u.Org1,
			org2:     u.Org2,
			org3:     u.Org3,
			org4:     u.Org4,
		}
	}

	for _, u := range matchedUsers {
		daily, err := ListUserProductivity(statDB, u.UserId, startTime, endTime, 1, 100000)
		if err != nil {
			continue
		}

		for _, d := range daily {
			if d.CreateTime.IsZero() {
				continue
			}
			dateStr := d.CreateTime.Format("2006-01-02")

			if _, ok := dailyMap[dateStr]; !ok {
				dailyMap[dateStr] = &dailyAgg{}
			}
			da := dailyMap[dateStr]
			ma := memberMap[u.UserId]

			da.taskDiffLines += d.TaskDiffLines
			ma.taskDiffLines += d.TaskDiffLines
			da.commitDiffLines += d.CommitDiffLines
			ma.commitDiffLines += d.CommitDiffLines
			da.taskRealMin += d.TaskRealMinutes
			ma.taskRealMin += d.TaskRealMinutes
			da.taskAncientMin += d.TaskAncientMinutes
			ma.taskAncientMin += d.TaskAncientMinutes
			da.commitRealMin += d.CommitRealMinutes
			ma.commitRealMin += d.CommitRealMinutes
			da.commitAncientMin += d.CommitAncientMinutes
			ma.commitAncientMin += d.CommitAncientMinutes
			da.upTokens += d.UpstreamTokens
			da.downTokens += d.DownstreamTokens
			da.cost += d.Cost
			ma.cost += d.Cost
		}
	}

	// 构建 daily 数组（按日期排序）
	var dates []string
	for date := range dailyMap {
		dates = append(dates, date)
	}
	sort.Strings(dates)

	dailyResult := make([]DailyDataItem, 0, len(dates))
	for _, date := range dates {
		da := dailyMap[date]
		var taskEffRatio, commitEffRatio float64
		if da.taskRealMin > 0 {
			taskEffRatio = utils.CalcEfficiencyRatio(da.taskAncientMin, da.taskRealMin)
		}
		if da.commitRealMin > 0 {
			commitEffRatio = utils.CalcEfficiencyRatio(da.commitAncientMin, da.commitRealMin)
		}
		dailyResult = append(dailyResult, DailyDataItem{
			Date:                  date,
			TaskDiffLines:         da.taskDiffLines,
			CommitDiffLines:       da.commitDiffLines,
			TaskRealMinutes:       da.taskRealMin,
			TaskAncientMinutes:    da.taskAncientMin,
			TaskEfficiencyRatio:   taskEffRatio,
			CommitRealMinutes:     da.commitRealMin,
			CommitAncientMinutes:  da.commitAncientMin,
			CommitEfficiencyRatio: commitEffRatio,
			UpstreamTokens:        da.upTokens,
			DownstreamTokens:      da.downTokens,
			Cost:                  da.cost,
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
	sumTaskEffRatio := utils.CalcEfficiencyRatio(sumTaskAncientMin, sumTaskRealMin)
	sumCommitEffRatio := utils.CalcEfficiencyRatio(sumCommitAncientMin, sumCommitRealMin)
	summary := GroupSummary{
		TaskDiffLines:         sumTaskDiffLines,
		CommitDiffLines:       sumCommitDiffLines,
		TaskRealMinutes:       sumTaskRealMin,
		TaskAncientMinutes:    sumTaskAncientMin,
		TaskEfficiencyRatio:   sumTaskEffRatio,
		CommitRealMinutes:     sumCommitRealMin,
		CommitAncientMinutes:  sumCommitAncientMin,
		CommitEfficiencyRatio: sumCommitEffRatio,
		UpstreamTokens:        sumUpTokens,
		DownstreamTokens:      sumDownTokens,
		Cost:                  sumCost,
	}

	// 构建 members 列表
	membersResult := make([]GroupMemberItem, 0, len(memberMap))
	for _, ma := range memberMap {
		var taskEffRatio, commitEffRatio float64
		if ma.taskRealMin > 0 {
			taskEffRatio = utils.CalcEfficiencyRatio(ma.taskAncientMin, ma.taskRealMin)
		}
		if ma.commitRealMin > 0 {
			commitEffRatio = utils.CalcEfficiencyRatio(ma.commitAncientMin, ma.commitRealMin)
		}
		membersResult = append(membersResult, GroupMemberItem{
			UserId:                ma.userID,
			UserName:              ma.userName,
			Org1:                  ma.org1,
			Org2:                  ma.org2,
			Org3:                  ma.org3,
			Org4:                  ma.org4,
			OrgDisplay:            getOrgDisplay(ma.org1, ma.org2, ma.org3, ma.org4, "", "", "", "", ""),
			TaskDiffLines:         ma.taskDiffLines,
			CommitDiffLines:       ma.commitDiffLines,
			TaskEfficiencyRatio:   taskEffRatio,
			CommitEfficiencyRatio: commitEffRatio,
			Cost:                  ma.cost,
		})
	}

	c.JSON(http.StatusOK, GroupDetailResponse{
		OrgPath: orgPath,
		Summary: summary,
		Daily:   dailyResult,
		Members: membersResult,
	})
}

type RefreshOrgMappingResponse struct {
	Count int    `json:"count"`
	Msg   string `json:"msg"`
}

// refreshOrgMappingV2 POST /api/v2/orgs/refresh
// @Summary 刷新组织结构查找表
// @Description 从 user_org 表重新加载组织结构映射到内存
// @Tags Orgs
// @Produce json
// @Success 200 {object} RefreshOrgMappingResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/orgs/refresh [post]
func refreshOrgMappingV2(c *gin.Context) {
	if statDB == nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "数据库未连接"})
		return
	}
	maps, err := LoadUserOrgs(statDB)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "刷新组织映射失败: " + err.Error()})
		return
	}
	orgMappings = maps
	c.JSON(http.StatusOK, RefreshOrgMappingResponse{
		Count: len(orgMappings),
		Msg:   "组织映射刷新成功",
	})
}
