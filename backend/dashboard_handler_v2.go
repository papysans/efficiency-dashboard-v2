package main

import (
	"net/http"
	"time"

	"kanban/core/utils"

	"github.com/gin-gonic/gin"
)

type DashboardSummaryResponse struct {
	TotalTasks              int     `json:"total_tasks"`
	TotalUsers              int     `json:"total_users"`
	TotalRepos              int     `json:"total_repos"`
	TotalCommits            int     `json:"total_commits"`
	TotalWorkDirs           int     `json:"total_work_dirs"`
	TotalCost               float64 `json:"total_cost"`
	TotalTokens             int64   `json:"total_tokens"`
	TotalDiffLines          int64   `json:"total_diff_lines"`
	TotalTaskAncientMinutes float64 `json:"total_task_ancient_minutes"`
	TotalRealMinutes        float64 `json:"total_real_minutes"`
	AvgEfficiencyRatio      float64 `json:"avg_efficiency_ratio"`
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

	totalWorkDirs, err := QueryDistinctWorkDirs(statDB)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	avgEfficiencyRatio := utils.CalcEfficiencyRatio(taskAgg.TotalAncientMinutes, taskAgg.TotalRealMinutes)

	c.JSON(http.StatusOK, DashboardSummaryResponse{
		TotalTasks:              taskAgg.TotalTasks,
		TotalUsers:              taskAgg.TotalUsers,
		TotalRepos:              taskAgg.TotalRepos,
		TotalCommits:            commitAgg.TotalCommits,
		TotalWorkDirs:           totalWorkDirs,
		TotalCost:               taskAgg.TotalCost,
		TotalTokens:             taskAgg.TotalTokens,
		TotalDiffLines:          commitAgg.TotalDiffLines,
		TotalTaskAncientMinutes: taskAgg.TotalAiDays,
		TotalRealMinutes:        taskAgg.TotalRealMinutes,
		AvgEfficiencyRatio:      avgEfficiencyRatio,
	})
}
