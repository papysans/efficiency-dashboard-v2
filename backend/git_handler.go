package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// getGitAnalysis GET /api/analysis/git
// @Summary 获取Git分析结果
// @Description 根据仓库ID查询Git分析结果，包含提交统计和AI估算信息
// @Tags Git
// @Produce json
// @Param repo_id query string true "仓库ID"
// @Param startDate query string false "开始日期(YYYYMMDD)"
// @Param endDate query string false "结束日期(YYYYMMDD)"
// @Success 200 {object} GitAnalysisResponse
// @Failure 400 {object} ErrorResponse
// @Router /analysis/git [get]
func getGitAnalysis(c *gin.Context) {
	repoID := c.Query("repo_id")
	if repoID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "repo_id 为必填参数"})
		return
	}

	startDate := c.Query("startDate")
	endDate := c.Query("endDate")

	var m *RepoMetrics
	var err error

	if startDate != "" && endDate != "" {
		startDateFmt, e := parseDateParam(startDate)
		if e != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "startDate 格式错误，需要 YYYYMMDD"})
			return
		}
		endDateFmt, e := parseDateParam(endDate)
		if e != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "endDate 格式错误，需要 YYYYMMDD"})
			return
		}
		analysisDate := formatDateYMD(time.Now())
		m, err = GetRepoMetrics(db, repoID, analysisDate, formatDateYMD(startDateFmt), formatDateYMD(endDateFmt))
	} else {
		m, err = GetLatestRepoMetrics(db, repoID)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	if m == nil {
		c.JSON(http.StatusOK, GitAnalysisResponse{RepoID: repoID, AnalysisDate: ""})
		return
	}

	c.JSON(http.StatusOK, buildGitAnalysisResponse(m))
}

func buildGitAnalysisResponse(m *RepoMetrics) GitAnalysisResponse {
	gitStats := &GitStatsInfo{}
	if m.GitCommitCount != nil {
		gitStats.CommitCount = m.GitCommitCount
	}
	if m.GitContributorCount != nil {
		gitStats.ContributorCount = m.GitContributorCount
	}
	if m.GitLinesAdded != nil {
		gitStats.LinesAdded = m.GitLinesAdded
	}
	if m.GitLinesDeleted != nil {
		gitStats.LinesDeleted = m.GitLinesDeleted
	}
	if m.GitFilesChanged != nil {
		gitStats.FilesChanged = m.GitFilesChanged
	}

	estimation := &EstimationInfo{}
	if m.RawAIEstimatedDaysFromTask != nil {
		estimation.FromTask = m.RawAIEstimatedDaysFromTask
	}
	if m.RawAIEstimatedDaysFromGit != nil {
		estimation.FromGit = m.RawAIEstimatedDaysFromGit
	}
	if m.RawAIEstimatedDaysFinal != nil {
		estimation.Final = m.RawAIEstimatedDaysFinal
	}

	resp := GitAnalysisResponse{
		RepoID:       m.RepoID,
		AnalysisDate: formatDateYMD(m.AnalysisDate),
		GitStats:     gitStats,
		Estimation:   estimation,
	}
	if m.GitAnalysisFilePath != nil {
		resp.GitAnalysisFile = m.GitAnalysisFilePath
	}
	return resp
}
