package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// getDashboardSummary GET /api/v2/dashboard/summary
func getDashboardSummary(c *gin.Context) {
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

	// SQL 1: 从 tasks 聚合
	taskQuery := `SELECT
		COUNT(task_id) as total_tasks,
		COUNT(DISTINCT user_id) as total_users,
		COUNT(DISTINCT work_dir_id) as total_repos,
		COALESCE(SUM(cost), 0) as total_cost,
		COALESCE(SUM(upstream_tokens + downstream_tokens), 0) as total_tokens,
		COALESCE(SUM(task_ancient_minutes), 0) as total_ai_days,
		COALESCE(SUM(COALESCE(task_real_minutes_manual, task_real_minutes)), 0) as total_real_minutes,
		AVG(CASE WHEN COALESCE(task_real_minutes_manual, task_real_minutes) > 0 AND COALESCE(task_ancient_minutes_manual, task_ancient_minutes) > 0 THEN COALESCE(task_ancient_minutes_manual, task_ancient_minutes) / COALESCE(task_real_minutes_manual, task_real_minutes) * 100 END) as avg_efficiency_ratio
		FROM tasks`

	var taskConditions []string
	var taskArgs []interface{}
	taskArgIdx := 1
	if startTime != "" {
		taskConditions = append(taskConditions, fmt.Sprintf("start_time >= $%d", taskArgIdx))
		taskArgs = append(taskArgs, startTime)
		taskArgIdx++
	}
	if endTime != "" {
		taskConditions = append(taskConditions, fmt.Sprintf("end_time <= $%d", taskArgIdx))
		taskArgs = append(taskArgs, endTime)
		taskArgIdx++
	}
	if len(taskConditions) > 0 {
		taskQuery += " WHERE " + strings.Join(taskConditions, " AND ")
	}

	var totalTasks, totalUsers, totalRepos int
	var totalCost, totalAIDays float64
	var totalTokens int64
	var totalRealMinutes float64
	var avgEfficiencyRatio *float64
	err := statDB.QueryRow(taskQuery, taskArgs...).Scan(&totalTasks, &totalUsers, &totalRepos, &totalCost, &totalTokens, &totalAIDays, &totalRealMinutes, &avgEfficiencyRatio)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询 tasks 聚合失败: " + err.Error()})
		return
	}

	// SQL 2: 从 commits 聚合
	commitQuery := `SELECT
		COUNT(*) as total_commits,
		COALESCE(SUM(diff_lines), 0) as total_diff_lines
		FROM commits`

	var commitConditions []string
	var commitArgs []interface{}
	commitArgIdx := 1
	if startTime != "" {
		commitConditions = append(commitConditions, fmt.Sprintf("commit_time >= $%d", commitArgIdx))
		commitArgs = append(commitArgs, startTime)
		commitArgIdx++
	}
	if endTime != "" {
		commitConditions = append(commitConditions, fmt.Sprintf("commit_time <= $%d", commitArgIdx))
		commitArgs = append(commitArgs, endTime)
		commitArgIdx++
	}
	if len(commitConditions) > 0 {
		commitQuery += " WHERE " + strings.Join(commitConditions, " AND ")
	}

	var totalCommits int
	var totalDiffLines int64
	err = statDB.QueryRow(commitQuery, commitArgs...).Scan(&totalCommits, &totalDiffLines)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询 commits 聚合失败: " + err.Error()})
		return
	}

	// SQL 3: 从 commits 聚合去重 repo
	var totalWorkDirs int
	err = statDB.QueryRow("SELECT COUNT(*) FROM (SELECT DISTINCT repo_addr, repo_branch FROM commits WHERE repo_addr IS NOT NULL AND repo_addr != '') sub").Scan(&totalWorkDirs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询 work_dirs 聚合失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total_tasks":                totalTasks,
		"total_users":                totalUsers,
		"total_repos":                totalRepos,
		"total_commits":              totalCommits,
		"total_work_dirs":            totalWorkDirs,
		"total_cost":                 totalCost,
		"total_tokens":               totalTokens,
		"total_diff_lines":           totalDiffLines,
		"total_task_ancient_minutes": totalAIDays,
		"total_real_minutes":         totalRealMinutes,
		"avg_efficiency_ratio":       avgEfficiencyRatio,
	})
}
