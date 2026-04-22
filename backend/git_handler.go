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

// triggerGitAnalysis POST /api/analysis/git/analyze
// @Summary 触发Git分析
// @Description 提交Git分析数据（提交数、贡献者数、代码行数等），保存到PG
// @Tags Git
// @Accept json
// @Produce json
// @Param data body GitAnalysisRequest true "Git分析数据"
// @Success 200 {object} GitAnalysisSaveResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /analysis/git/analyze [post]
type GitAnalysisRequest struct {
	RepoID                 string   `json:"repo_id" example:"repo1"`
	StartDate              string   `json:"start_date" example:"20260101"`
	EndDate                string   `json:"end_date" example:"20260331"`
	CommitCount            *int     `json:"commit_count,omitempty" example:"100"`
	ContributorCount       *int     `json:"contributor_count,omitempty" example:"5"`
	LinesAdded             *int64   `json:"lines_added,omitempty" example:"1000"`
	LinesDeleted           *int64   `json:"lines_deleted,omitempty" example:"500"`
	FilesChanged           *int     `json:"files_changed,omitempty" example:"20"`
	AIEstimatedDaysFromGit *float64 `json:"ai_estimated_days_from_git,omitempty" example:"10.5"`
	AIEstimatedReason      *string  `json:"ai_estimated_reason,omitempty"`
}

func triggerGitAnalysis(c *gin.Context) {
	var req GitAnalysisRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: fmt.Sprintf("请求体解析失败: %v", err)})
		return
	}
	if req.RepoID == "" || req.StartDate == "" || req.EndDate == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "repo_id, start_date, end_date 为必填参数"})
		return
	}

	startDateFmt, err := parseDateParam(req.StartDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "start_date 格式错误，需要 YYYYMMDD"})
		return
	}
	endDateFmt, err := parseDateParam(req.EndDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "end_date 格式错误，需要 YYYYMMDD"})
		return
	}

	analysisDate := time.Now()
	analysisDateStr := formatDateYMD(analysisDate)
	startDateStr := formatDateYMD(startDateFmt)
	endDateStr := formatDateYMD(endDateFmt)

	// 查询已有记录，保留非 git 字段
	existing, err := GetRepoMetrics(db, req.RepoID, analysisDateStr, startDateStr, endDateStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
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
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, GitAnalysisSaveResponse{Message: "git 分析数据已保存", RepoID: req.RepoID})
}

// getGitCommits GET /api/analysis/git/commits
// @Summary 获取Git提交列表
// @Description 根据仓库ID和日期获取Git提交记录列表
// @Tags Git
// @Produce json
// @Param repo_id query string true "仓库ID"
// @Param endDate query string false "结束日期(YYYYMMDD)，默认今天"
// @Success 200 {object} GitCommitsResponse
// @Failure 400 {object} ErrorResponse
// @Router /analysis/git/commits [get]
func getGitCommits(c *gin.Context) {
	repoID := c.Query("repo_id")
	if repoID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "repo_id 为必填参数"})
		return
	}

	endDate := c.Query("endDate")
	if endDate == "" {
		endDate = time.Now().Format("20060102")
	}

	endDateFmt, err := parseDateParam(endDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "endDate 格式错误，需要 YYYYMMDD"})
		return
	}

	safeID := makeSafeID(repoID)
	dirName := endDateFmt.Format("2006-01") + "/analysis"
	fileName := fmt.Sprintf("git_commits_%s_%s.json", safeID, endDate)
	filePath := filepath.Join(appConfig.RawDataDir, dirName, fileName)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusOK, GitCommitsResponse{Commits: []GitCommitItem{}, RepoID: repoID})
		return
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: fmt.Sprintf("读取 commit 文件失败: %v", err)})
		return
	}

	var commits []GitCommitItem
	if err := json.Unmarshal(data, &commits); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: fmt.Sprintf("解析 commit 文件失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, GitCommitsResponse{Commits: commits, RepoID: repoID})
}
