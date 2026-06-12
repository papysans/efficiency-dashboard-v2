package main

import (
	"kanban/core/utils"
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
