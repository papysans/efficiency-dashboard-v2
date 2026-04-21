package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// getTaskCommitMappings GET /api/analysis/task-commits
func getTaskCommitMappings(c *gin.Context) {
	repoID := c.Query("repo_id")
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")

	if repoID == "" || startDate == "" || endDate == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "repo_id, startDate, endDate 为必填参数"})
		return
	}

	startDateFmt, err := parseDateParam(startDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "startDate 格式错误，需要 YYYYMMDD"})
		return
	}
	endDateFmt, err := parseDateParam(endDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "endDate 格式错误，需要 YYYYMMDD"})
		return
	}

	list, err := ListTaskCommitMappings(db, repoID, formatDateYMD(startDateFmt), formatDateYMD(endDateFmt))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	items := make([]gin.H, 0, len(list))
	for _, m := range list {
		item := gin.H{
			"task_id":     m.TaskID,
			"commit_hash": m.CommitHash,
			"code_source": m.CodeSource,
		}
		if m.UserID != nil {
			item["user_id"] = *m.UserID
		}
		if m.MatchScore != nil {
			item["match_score"] = *m.MatchScore
		}
		if m.MatchReason != nil {
			item["match_reason"] = *m.MatchReason
		}
		items = append(items, item)
	}

	c.JSON(http.StatusOK, gin.H{"items": items})
}

// getCodeAttribution GET /api/analysis/code-attribution
func getCodeAttribution(c *gin.Context) {
	repoID := c.Query("repo_id")
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")

	if repoID == "" || startDate == "" || endDate == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "repo_id, startDate, endDate 为必填参数"})
		return
	}

	startDateFmt, err := parseDateParam(startDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "startDate 格式错误，需要 YYYYMMDD"})
		return
	}
	endDateFmt, err := parseDateParam(endDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "endDate 格式错误，需要 YYYYMMDD"})
		return
	}

	list, err := ListCodeAttributions(db, repoID, formatDateYMD(startDateFmt), formatDateYMD(endDateFmt))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var totalOurAI, totalHuman int64
	details := make([]gin.H, 0, len(list))
	for _, a := range list {
		totalOurAI += a.OurAICodeLines
		totalHuman += a.HumanCodeLines
		item := gin.H{
			"commit_hash":   a.CommitHash,
			"our_ai_lines":  a.OurAICodeLines,
			"human_lines":   a.HumanCodeLines,
		}
		if a.TaskID != nil {
			item["task_id"] = *a.TaskID
		}
		details = append(details, item)
	}

	c.JSON(http.StatusOK, gin.H{
		"summary": gin.H{
			"total_our_ai_lines": totalOurAI,
			"total_human_lines":  totalHuman,
		},
		"details": details,
	})
}

// getCodeSourceStats GET /api/analysis/code-source
func getCodeSourceStats(c *gin.Context) {
	repoID := c.Query("repo_id")
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")

	if repoID == "" || startDate == "" || endDate == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "repo_id, startDate, endDate 为必填参数"})
		return
	}

	startDateFmt, err := parseDateParam(startDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "startDate 格式错误，需要 YYYYMMDD"})
		return
	}
	endDateFmt, err := parseDateParam(endDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "endDate 格式错误，需要 YYYYMMDD"})
		return
	}

	analysisDate := formatDateYMD(time.Now())
	m, err := GetRepoMetrics(db, repoID, analysisDate, formatDateYMD(startDateFmt), formatDateYMD(endDateFmt))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

	c.JSON(http.StatusOK, gin.H{
		"code_source": gin.H{
			"ai_current":       gin.H{"lines": aiCurrent, "percentage": pct(aiCurrent)},
			"human":            gin.H{"lines": human, "percentage": pct(human)},
			"ai_other":         gin.H{"lines": aiOther, "percentage": pct(aiOther)},
			"unknown":          gin.H{"lines": unknown, "percentage": pct(unknown)},
		},
		"mapped_task_count": mappedTaskCount,
	})
}
