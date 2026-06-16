package main

import (
	"kanban/core/utils"
	"math"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type DashboardSummaryResponse struct {
	TotalTasks                int     `json:"total_tasks"`
	TotalUsers                int     `json:"total_users"`
	TotalRepos                int     `json:"total_repos"`
	TotalCommits              int     `json:"total_commits"`
	TotalBranchs              int     `json:"total_branchs"`
	TotalWorkDirs             int     `json:"total_work_dirs"`
	TotalCost                 float64 `json:"total_cost"`
	TotalTokens               int64   `json:"total_tokens"`
	TotalTaskLines            int64   `json:"total_task_lines"`
	TotalCommitLines          int64   `json:"total_commit_lines"`
	TotalDiffLines            int64   `json:"total_diff_lines"`
	TotalRealMinutes          float64 `json:"total_real_minutes"`
	AvgEfficiencyRatio        float64 `json:"avg_efficiency_ratio"`
	TotalTaskAncientMinutes   float64 `json:"total_task_ancient_minutes"`
	TotalTaskRealMinutes      float64 `json:"total_task_real_minutes"`
	TaskEfficiencyRatio       float64 `json:"task_efficiency_ratio"`
	TotalCommitAncientMinutes float64 `json:"total_commit_ancient_minutes"`
	TotalCommitRealMinutes    float64 `json:"total_commit_real_minutes"`
	CommitEfficiencyRatio     float64 `json:"commit_efficiency_ratio"`

	// v2（Need 维度）派生指标：当只跑了 v2 管道、tasks 表为空时，首页用这些字段展示真实数据。
	TotalUsersV2            int      `json:"total_users_v2"`
	TotalNeeds              int      `json:"total_needs"`
	MergedNeeds             int      `json:"merged_needs"`
	EligibleNeeds           int      `json:"eligible_needs"`
	NeedActualCalendarMin   float64  `json:"need_actual_calendar_min"`
	NeedBaselineCalendarMin float64  `json:"need_baseline_calendar_min"`
	NeedCalendarRatio       *float64 `json:"need_calendar_ratio"`
	NeedWorkRatio           *float64 `json:"need_work_ratio"`
	AICodeRatio             *float64 `json:"ai_code_ratio"`
}

// efficiencyV2Ratio 返回 v2 小数口径提效比 (baseline - actual) / actual；actual<=0 返回 nil。
func efficiencyV2Ratio(baseline, actual float64) *float64 {
	if actual <= 0 {
		return nil
	}
	r := (baseline - actual) / actual
	return &r
}

// DashboardTrendPoint 首页趋势单周点。efficiency_ratio 小数口径（前端用 RatioPill），actual<=0 时为 null。
type DashboardTrendPoint struct {
	WeekStart       string   `json:"week_start"` // YYYY-MM-DD（ISO 周一）
	EfficiencyRatio *float64 `json:"efficiency_ratio"`
	ActiveUsers     int64    `json:"active_users"`
	MergedNeedCount int64    `json:"merged_need_count"`
	Cost            float64  `json:"cost"`
	CommitDiffLines int64    `json:"commit_diff_lines"`
}

// DashboardTrendDelta 单维度"本期 vs 等长上期"环比。delta_pct 在上期为 0 时为 null（前端不画箭头）。
type DashboardTrendDelta struct {
	Current  float64  `json:"current"`
	Previous float64  `json:"previous"`
	DeltaPct *float64 `json:"delta_pct"`
}

// DashboardTrendsResponse 首页 4 维趋势 + 环比。compare 键：efficiency/usage/cost/contribution。
type DashboardTrendsResponse struct {
	Granularity string                         `json:"granularity"`
	Points      []DashboardTrendPoint          `json:"points"`
	Compare     map[string]DashboardTrendDelta `json:"compare"`
}

// trendDelta 组装单维度环比；上期非 0 时给出相对变化率(有符号)，否则 delta_pct 为 nil。
func trendDelta(current, previous float64) DashboardTrendDelta {
	d := DashboardTrendDelta{Current: current, Previous: previous}
	if previous != 0 {
		p := (current - previous) / math.Abs(previous)
		d.DeltaPct = &p
	}
	return d
}

// windowRatio 整窗提效比：先汇总分钟再求比；actual<=0 返回 0（环比按 0 处理，不画箭头由 previous 决定）。
func windowRatio(agg *dashboardTrendWindowAgg) float64 {
	if agg == nil || agg.ActualCalendarMin <= 0 {
		return 0
	}
	return (agg.BaselineCalendarMin - agg.ActualCalendarMin) / agg.ActualCalendarMin
}

// getDashboardTrends GET /api/v2/dashboard/trends
// @Summary 首页 4 维周趋势 + 环比
// @Description 跨用户按周聚合 user_productivity_v2，返回使用/效率/成本/贡献的周序列(sparkline)与本期vs等长上期环比
// @Tags Dashboard
// @Produce json
// @Param startDate query string false "开始日期(YYYYMMDD 或 YYYY-MM-DD)"
// @Param endDate query string false "结束日期(YYYYMMDD 或 YYYY-MM-DD)"
// @Success 200 {object} DashboardTrendsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/dashboard/trends [get]
func getDashboardTrends(c *gin.Context) {
	start, err := parseStartDate(c.Query("startDate"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "startDate 格式错误: " + err.Error()})
		return
	}
	end, err := parseEndDate(c.Query("endDate"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "endDate 格式错误: " + err.Error()})
		return
	}

	rows, err := queryDashboardTrendWeekly(statDB, start, end)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	points := make([]DashboardTrendPoint, 0, len(rows))
	for _, r := range rows {
		points = append(points, DashboardTrendPoint{
			WeekStart:       r.WeekStart.Format("2006-01-02"),
			EfficiencyRatio: efficiencyV2Ratio(r.BaselineCalendarMin, r.ActualCalendarMin),
			ActiveUsers:     r.ActiveUsers,
			MergedNeedCount: r.MergedNeedCount,
			Cost:            r.Cost,
			CommitDiffLines: r.CommitDiffLines,
		})
	}

	// 环比：本期 vs 等长前一区间；start/end 都有才计算（否则窗口长度不定）。
	compare := map[string]DashboardTrendDelta{}
	if start != nil && end != nil {
		cur, err := queryDashboardTrendWindowAgg(statDB, start, end)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			return
		}
		dur := end.Sub(*start)
		// 上期 = 紧邻当前窗口、等长且不重叠的前一区间。prevEnd 必须严格早于 start，
		// 否则 week_start==start 的那一周会同时落进本期(week_start>=start)与上期(week_start<=prevEnd)被双算。
		prevEnd := start.Add(-time.Second)
		prevStart := prevEnd.Add(-dur)
		prev, err := queryDashboardTrendWindowAgg(statDB, &prevStart, &prevEnd)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			return
		}
		compare["usage"] = trendDelta(float64(cur.DistinctUsers), float64(prev.DistinctUsers))
		compare["efficiency"] = trendDelta(windowRatio(cur), windowRatio(prev))
		compare["cost"] = trendDelta(cur.Cost, prev.Cost)
		compare["contribution"] = trendDelta(float64(cur.CommitDiffLines), float64(prev.CommitDiffLines))
	}

	c.JSON(http.StatusOK, DashboardTrendsResponse{Granularity: "week", Points: points, Compare: compare})
}

// getDashboardSummary GET /api/v2/dashboard/summary
// @Summary 获取仪表盘汇总信息
// @Description 获取仪表盘的汇总统计数据
// @Tags Dashboard
// @Produce json
// @Param startDate query string false "开始日期"
// @Param endDate query string false "结束日期"
// @Success 200 {object} DashboardSummaryResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/dashboard/summary [get]
func getDashboardSummary(c *gin.Context) {
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

	taskAgg, err := QueryDashboardTaskAgg(statDB, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	commitAgg, err := QueryDashboardCommitAgg(statDB, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	needAgg, err := QueryDashboardNeedAgg(statDB, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	taskRatio := utils.CalcEfficiencyRatio(taskAgg.TotalAncientMinutes, taskAgg.TotalRealMinutes)
	commitRatio := utils.CalcEfficiencyRatio(commitAgg.TotalAncientMinutes, commitAgg.TotalRealMinutes)

	// Need 维度综合提效：小数口径 (baseline - actual) / actual，仅当实际值>0时有意义。
	needCalendarRatio := efficiencyV2Ratio(needAgg.BaselineCalendarMin, needAgg.ActualCalendarMin)
	needWorkRatio := efficiencyV2Ratio(needAgg.BaselineWorkMin, needAgg.ActualWorkMin)

	// 总用户数：tasks 为空时回退到 commits 去重用户，保证首页有真实值。
	totalUsers := taskAgg.TotalUsers
	if totalUsers == 0 {
		totalUsers = commitAgg.TotalUsers
	}

	c.JSON(http.StatusOK, DashboardSummaryResponse{
		// 名实相符：前端卡片 hint 为「需求口径参与者」，取 need 维度 primary_user_id 去重（看板口径+日期窗）
		TotalUsersV2:              needAgg.TotalUsers,
		TotalNeeds:                needAgg.TotalNeeds,
		MergedNeeds:               needAgg.MergedNeeds,
		EligibleNeeds:             needAgg.EligibleNeeds,
		NeedActualCalendarMin:     needAgg.ActualCalendarMin,
		NeedBaselineCalendarMin:   needAgg.BaselineCalendarMin,
		NeedCalendarRatio:         needCalendarRatio,
		NeedWorkRatio:             needWorkRatio,
		AICodeRatio:               calcNeedAICodeRatio(needAgg.AICoveredLoc, needAgg.TotalLocNet),
		TotalTasks:                taskAgg.TotalTasks,
		TotalUsers:                totalUsers,
		TotalWorkDirs:             taskAgg.TotalWorkDirs,
		TotalCost:                 taskAgg.TotalCost,
		TotalTokens:               taskAgg.TotalTokens,
		TotalTaskLines:            taskAgg.TotalLines,
		TotalRealMinutes:          taskAgg.TotalRealMinutes,
		TotalTaskAncientMinutes:   taskAgg.TotalAncientMinutes,
		TotalTaskRealMinutes:      taskAgg.TotalRealMinutes,
		TaskEfficiencyRatio:       taskRatio,
		TotalCommitAncientMinutes: commitAgg.TotalAncientMinutes,
		TotalCommitRealMinutes:    commitAgg.TotalRealMinutes,
		CommitEfficiencyRatio:     commitRatio,
		TotalCommits:              commitAgg.TotalCommits,
		TotalRepos:                commitAgg.TotalRepos,
		TotalBranchs:              commitAgg.TotalBranchs,
		TotalCommitLines:          commitAgg.TotalDiffLines,

		TotalDiffLines:     commitAgg.TotalDiffLines,
		AvgEfficiencyRatio: taskRatio,
	})
}
