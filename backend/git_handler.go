package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
)

// getGitAnalysis GET /api/analysis/git
func getGitAnalysis(c *gin.Context) {
	repoID := c.Query("repo_id")
	if repoID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "repo_id 为必填参数"})
		return
	}

	startDate := c.Query("startDate")
	endDate := c.Query("endDate")

	var m *RepoMetrics
	var err error

	if startDate != "" && endDate != "" {
		startDateFmt, e := parseDateParam(startDate)
		if e != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "startDate 格式错误，需要 YYYYMMDD"})
			return
		}
		endDateFmt, e := parseDateParam(endDate)
		if e != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "endDate 格式错误，需要 YYYYMMDD"})
			return
		}
		analysisDate := formatDateYMD(time.Now())
		m, err = GetRepoMetrics(db, repoID, analysisDate, formatDateYMD(startDateFmt), formatDateYMD(endDateFmt))
	} else {
		m, err = GetLatestRepoMetrics(db, repoID)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if m == nil {
		c.JSON(http.StatusOK, gin.H{
			"repo_id":       repoID,
			"analysis_date": nil,
			"git_stats":     nil,
			"estimation":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, buildGitAnalysisResponse(m))
}

// buildGitAnalysisResponse 构建 git 分析响应
func buildGitAnalysisResponse(m *RepoMetrics) gin.H {
	gitStats := gin.H{}
	if m.GitCommitCount != nil {
		gitStats["commit_count"] = *m.GitCommitCount
	}
	if m.GitContributorCount != nil {
		gitStats["contributor_count"] = *m.GitContributorCount
	}
	if m.GitLinesAdded != nil {
		gitStats["lines_added"] = *m.GitLinesAdded
	}
	if m.GitLinesDeleted != nil {
		gitStats["lines_deleted"] = *m.GitLinesDeleted
	}
	if m.GitFilesChanged != nil {
		gitStats["files_changed"] = *m.GitFilesChanged
	}

	estimation := gin.H{}
	if m.RawAIEstimatedDaysFromTask != nil {
		estimation["from_task"] = *m.RawAIEstimatedDaysFromTask
	}
	if m.RawAIEstimatedDaysFromGit != nil {
		estimation["from_git"] = *m.RawAIEstimatedDaysFromGit
	}
	if m.RawAIEstimatedDaysFinal != nil {
		estimation["final"] = *m.RawAIEstimatedDaysFinal
	}

	resp := gin.H{
		"repo_id":       m.RepoID,
		"analysis_date": formatDateYMD(m.AnalysisDate),
		"git_stats":     gitStats,
		"estimation":    estimation,
	}
	if m.GitAnalysisFilePath != nil {
		resp["git_analysis_file"] = *m.GitAnalysisFilePath
	}
	return resp
}

// triggerGitAnalysis POST /api/analysis/git/analyze
func triggerGitAnalysis(c *gin.Context) {
	var req struct {
		RepoID                   string   `json:"repo_id"`
		StartDate                string   `json:"start_date"`
		EndDate                  string   `json:"end_date"`
		CommitCount              *int     `json:"commit_count"`
		ContributorCount         *int     `json:"contributor_count"`
		LinesAdded               *int64   `json:"lines_added"`
		LinesDeleted             *int64   `json:"lines_deleted"`
		FilesChanged             *int     `json:"files_changed"`
		AIEstimatedDaysFromGit   *float64 `json:"ai_estimated_days_from_git"`
		AIEstimatedReason        *string  `json:"ai_estimated_reason"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("请求体解析失败: %v", err)})
		return
	}
	if req.RepoID == "" || req.StartDate == "" || req.EndDate == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "repo_id, start_date, end_date 为必填参数"})
		return
	}

	startDateFmt, err := parseDateParam(req.StartDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "start_date 格式错误，需要 YYYYMMDD"})
		return
	}
	endDateFmt, err := parseDateParam(req.EndDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "end_date 格式错误，需要 YYYYMMDD"})
		return
	}

	analysisDate := time.Now()
	analysisDateStr := formatDateYMD(analysisDate)
	startDateStr := formatDateYMD(startDateFmt)
	endDateStr := formatDateYMD(endDateFmt)

	// 查询已有记录，保留非 git 字段
	existing, err := GetRepoMetrics(db, req.RepoID, analysisDateStr, startDateStr, endDateStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var m RepoMetrics
	if existing != nil {
		m = *existing
	} else {
		m.RepoID = req.RepoID
		m.AnalysisDate = analysisDate
		m.QueryStartDate = startDateFmt
		m.QueryEndDate = endDateFmt
	}

	// 更新 git 分析字段
	m.GitCommitCount = req.CommitCount
	m.GitContributorCount = req.ContributorCount
	m.GitLinesAdded = req.LinesAdded
	m.GitLinesDeleted = req.LinesDeleted
	m.GitFilesChanged = req.FilesChanged
	m.RawAIEstimatedDaysFromGit = req.AIEstimatedDaysFromGit

	// 保存 git 分析文件
	safeID := makeSafeID(req.RepoID)
	dirName := endDateFmt.Format("2006-01") + "/analysis"
	fileName := fmt.Sprintf("git_commits_%s_%s.json", safeID, req.EndDate)
	gitFilePath := filepath.Join(appConfig.RawDataDir, dirName, fileName)
	m.GitAnalysisFilePath = ptrString(gitFilePath)

	if err := UpsertRepoMetrics(db, &m); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "git 分析数据已保存",
		"repo_id": req.RepoID,
	})
}

// getGitCommits GET /api/analysis/git/commits
func getGitCommits(c *gin.Context) {
	repoID := c.Query("repo_id")
	if repoID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "repo_id 为必填参数"})
		return
	}

	endDate := c.Query("endDate")
	if endDate == "" {
		endDate = time.Now().Format("20060102")
	}

	endDateFmt, err := parseDateParam(endDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "endDate 格式错误，需要 YYYYMMDD"})
		return
	}

	safeID := makeSafeID(repoID)
	dirName := endDateFmt.Format("2006-01") + "/analysis"
	fileName := fmt.Sprintf("git_commits_%s_%s.json", safeID, endDate)
	filePath := filepath.Join(appConfig.RawDataDir, dirName, fileName)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusOK, gin.H{"commits": []interface{}{}, "repo_id": repoID})
		return
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("读取 commit 文件失败: %v", err)})
		return
	}

	var commits interface{}
	if err := json.Unmarshal(data, &commits); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("解析 commit 文件失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"commits": commits, "repo_id": repoID})
}
