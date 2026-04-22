package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type TaskCommitMappingItem struct {
	TaskID      string   `json:"task_id"`
	CommitHash  string   `json:"commit_hash"`
	CodeSource  string   `json:"code_source"`
	UserID      *string  `json:"user_id,omitempty"`
	MatchScore  *float64 `json:"match_score,omitempty"`
	MatchReason *string  `json:"match_reason,omitempty"`
}

type TaskCommitMappingsResponse struct {
	Items []TaskCommitMappingItem `json:"items"`
}

type CodeAttributionSummary struct {
	TotalOurAILines int64 `json:"total_our_ai_lines"`
	TotalHumanLines int64 `json:"total_human_lines"`
}

type CodeAttributionDetail struct {
	CommitHash string  `json:"commit_hash"`
	OurAILines int64   `json:"our_ai_lines"`
	HumanLines int64   `json:"human_lines"`
	TaskID     *string `json:"task_id,omitempty"`
}

type CodeAttributionResponse struct {
	Summary CodeAttributionSummary  `json:"summary"`
	Details []CodeAttributionDetail `json:"details"`
}

type CodeSourceItem struct {
	Lines      int64   `json:"lines"`
	Percentage float64 `json:"percentage"`
}

type CodeSourceGroup struct {
	AICurrent CodeSourceItem `json:"ai_current"`
	Human     CodeSourceItem `json:"human"`
	AIOther   CodeSourceItem `json:"ai_other"`
	Unknown   CodeSourceItem `json:"unknown"`
}

type CodeSourceStatsResponse struct {
	CodeSource      CodeSourceGroup `json:"code_source"`
	MappedTaskCount int             `json:"mapped_task_count"`
}

// getTaskCommitMappings GET /api/analysis/task-commits
// @Summary 获取任务-提交映射
// @Description 查询指定仓库和日期范围内的任务与提交的映射关系
// @Tags Attribution
// @Produce json
// @Param repo_id query string true "仓库ID"
// @Param startDate query string true "开始日期(YYYYMMDD)"
// @Param endDate query string true "结束日期(YYYYMMDD)"
// @Success 200 {object} TaskCommitMappingsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /analysis/task-commits [get]
func getTaskCommitMappings(c *gin.Context) {
	repoID := c.Query("repo_id")
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")

	if repoID == "" || startDate == "" || endDate == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "repo_id, startDate, endDate 为必填参数"})
		return
	}

	startDateFmt, err := parseDateParam(startDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "startDate 格式错误，需要 YYYYMMDD"})
		return
	}
	endDateFmt, err := parseDateParam(endDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "endDate 格式错误，需要 YYYYMMDD"})
		return
	}

	list, err := ListTaskCommitMappings(db, repoID, formatDateYMD(startDateFmt), formatDateYMD(endDateFmt))
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	items := make([]TaskCommitMappingItem, 0, len(list))
	for _, m := range list {
		item := TaskCommitMappingItem{
			TaskID:     m.TaskID,
			CommitHash: m.CommitHash,
			CodeSource: m.CodeSource,
		}
		if m.UserID != nil {
			item.UserID = m.UserID
		}
		if m.MatchScore != nil {
			item.MatchScore = m.MatchScore
		}
		if m.MatchReason != nil {
			item.MatchReason = m.MatchReason
		}
		items = append(items, item)
	}
	c.JSON(http.StatusOK, TaskCommitMappingsResponse{Items: items})
}

// getCodeAttribution GET /api/analysis/code-attribution
// @Summary 获取代码归属分析
// @Description 查询指定仓库和日期范围内的代码归属分析，统计AI和人工代码行数
// @Tags Attribution
// @Produce json
// @Param repo_id query string true "仓库ID"
// @Param startDate query string true "开始日期(YYYYMMDD)"
// @Param endDate query string true "结束日期(YYYYMMDD)"
// @Success 200 {object} CodeAttributionResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /analysis/code-attribution [get]
func getCodeAttribution(c *gin.Context) {
	repoID := c.Query("repo_id")
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")

	if repoID == "" || startDate == "" || endDate == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "repo_id, startDate, endDate 为必填参数"})
		return
	}

	startDateFmt, err := parseDateParam(startDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "startDate 格式错误，需要 YYYYMMDD"})
		return
	}
	endDateFmt, err := parseDateParam(endDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "endDate 格式错误，需要 YYYYMMDD"})
		return
	}

	list, err := ListCodeAttributions(db, repoID, formatDateYMD(startDateFmt), formatDateYMD(endDateFmt))
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	var totalOurAI, totalHuman int64
	details := make([]CodeAttributionDetail, 0, len(list))
	for _, a := range list {
		totalOurAI += a.OurAICodeLines
		totalHuman += a.HumanCodeLines
		item := CodeAttributionDetail{
			CommitHash: a.CommitHash,
			OurAILines: a.OurAICodeLines,
			HumanLines: a.HumanCodeLines,
		}
		if a.TaskID != nil {
			item.TaskID = a.TaskID
		}
		details = append(details, item)
	}
	c.JSON(http.StatusOK, CodeAttributionResponse{
		Summary: CodeAttributionSummary{TotalOurAILines: totalOurAI, TotalHumanLines: totalHuman},
		Details: details,
	})
}

// getCodeSourceStats GET /api/analysis/code-source
// @Summary 获取代码来源统计
// @Description 查询指定仓库的代码来源分布统计（AI当前/Human/AI其他/未知）
// @Tags Attribution
// @Produce json
// @Param repo_id query string true "仓库ID"
// @Param startDate query string true "开始日期(YYYYMMDD)"
// @Param endDate query string true "结束日期(YYYYMMDD)"
// @Success 200 {object} CodeSourceStatsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /analysis/code-source [get]
func getCodeSourceStats(c *gin.Context) {
	repoID := c.Query("repo_id")
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")

	if repoID == "" || startDate == "" || endDate == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "repo_id, startDate, endDate 为必填参数"})
		return
	}

	startDateFmt, err := parseDateParam(startDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "startDate 格式错误，需要 YYYYMMDD"})
		return
	}
	endDateFmt, err := parseDateParam(endDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "endDate 格式错误，需要 YYYYMMDD"})
		return
	}

	analysisDate := formatDateYMD(time.Now())
	m, err := GetRepoMetrics(db, repoID, analysisDate, formatDateYMD(startDateFmt), formatDateYMD(endDateFmt))
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	var aiCurrent, human, aiOther, unknown int64
	var mappedTaskCount int
	if m != nil {
		if m.OurAICodeLines != nil {
			aiCurrent = *m.OurAICodeLines
		}
		if m.HumanCodeLines != nil {
			human = *m.HumanCodeLines
		}
		if m.AIOtherCodeLines != nil {
			aiOther = *m.AIOtherCodeLines
		}
		if m.UnknownCodeLines != nil {
			unknown = *m.UnknownCodeLines
		}
		if m.MappedTaskCount != nil {
			mappedTaskCount = *m.MappedTaskCount
		}
	}

	total := aiCurrent + human + aiOther + unknown
	pct := func(v int64) float64 {
		if total == 0 {
			return 0
		}
		return float64(v) / float64(total) * 100
	}

	c.JSON(http.StatusOK, CodeSourceStatsResponse{
		CodeSource: CodeSourceGroup{
			AICurrent: CodeSourceItem{Lines: aiCurrent, Percentage: pct(aiCurrent)},
			Human:     CodeSourceItem{Lines: human, Percentage: pct(human)},
			AIOther:   CodeSourceItem{Lines: aiOther, Percentage: pct(aiOther)},
			Unknown:   CodeSourceItem{Lines: unknown, Percentage: pct(unknown)},
		},
		MappedTaskCount: mappedTaskCount,
	})
}
