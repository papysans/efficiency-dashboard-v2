package main

import (
	"log"
	"net/http"
	"strings"
	"time"

	"kanban/core/models"
	"kanban/core/utils"

	"github.com/gin-gonic/gin"
)

type CommitListResponse struct {
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"pageSize"`
	Data     []CommitListItem `json:"data"`
}

type UpdateCommitManualRequest struct {
	CommitAncientMinutesManual *float64 `json:"commit_ancient_minutes_manual"`
	CommitAncientReasonManual  *string  `json:"commit_ancient_minutes_reason_manual"`
	CommitRealMinutesManual    *float64 `json:"commit_real_minutes_manual"`
	CommitRealReasonManual     *string  `json:"commit_real_minutes_reason_manual"`
}

type CommitDetailResponse struct {
	Commit           *models.Commit `json:"commit"`
	RelatedTasks     []RelatedTask  `json:"related_tasks"`
	EfficiencyRatio  float64        `json:"efficiency_ratio"`
	TotalCost        float64        `json:"total_cost"`
	Silica           float64        `json:"silica"`
	UpstreamTokens   int64          `json:"upstream_tokens"`
	DownstreamTokens int64          `json:"downstream_tokens"`
}

// listCommitsV2 GET /api/v2/commits
// @Summary 获取提交列表
// @Description 按条件查询提交列表，支持分页
// @Tags Commits
// @Produce json
// @Param repoAddr query string false "仓库地址"
// @Param repoBranch query string false "分支名"
// @Param gitUserName query string false "git用户名"
// @Param userId query string false "用户ID"
// @Param userName query string false "用户名"
// @Param clientId query string false "客户端ID"
// @Param workDir query string false "工作目录"
// @Param startDate query string false "开始日期(YYYYMMDD)"
// @Param endDate query string false "结束日期(YYYYMMDD)"
// @Param org1 query string false "一级组织"
// @Param org2 query string false "二级组织"
// @Param org3 query string false "三级组织"
// @Param org4 query string false "四级组织"
// @Param org5 query string false "五级组织"
// @Param org6 query string false "六级组织"
// @Param org7 query string false "七级组织"
// @Param org8 query string false "八级组织"
// @Param org9 query string false "九级组织"
// @Param page query int false "页码" default(1)
// @Param pageSize query int false "每页数量" default(20)
// @Success 200 {object} CommitListResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/commits [get]
func listCommitsV2(c *gin.Context) {
	filter := CommitFilter{
		RepoAddr:    strings.TrimSpace(c.Query("repoAddr")),
		RepoBranch:  strings.TrimSpace(c.Query("repoBranch")),
		GitUserName: strings.TrimSpace(c.Query("gitUserName")),
		UserId:      strings.TrimSpace(c.Query("userId")),
		UserName:    strings.TrimSpace(c.Query("userName")),
		ClientId:    strings.TrimSpace(c.Query("clientId")),
		WorkDir:     strings.TrimSpace(c.Query("workDir")),
		OrgsFilter: OrgsFilter{
			Org1: strings.TrimSpace(c.Query("org1")),
			Org2: strings.TrimSpace(c.Query("org2")),
			Org3: strings.TrimSpace(c.Query("org3")),
			Org4: strings.TrimSpace(c.Query("org4")),
			Org5: strings.TrimSpace(c.Query("org5")),
			Org6: strings.TrimSpace(c.Query("org6")),
			Org7: strings.TrimSpace(c.Query("org7")),
			Org8: strings.TrimSpace(c.Query("org8")),
			Org9: strings.TrimSpace(c.Query("org9")),
		},
	}

	startDate := strings.TrimSpace(c.Query("startDate"))
	endDate := strings.TrimSpace(c.Query("endDate"))
	if startDate != "" {
		startT, err := parseDateParam(startDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "startDate 格式错误: " + err.Error()})
			return
		}
		filter.StartTime = startT.Format(time.RFC3339)
	}
	if endDate != "" {
		endT, err := parseDateParam(endDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "endDate 格式错误: " + err.Error()})
			return
		}
		filter.EndTime = endT.Add(23*time.Hour + 59*time.Minute + 59*time.Second).Format(time.RFC3339)
	}

	page := getDefaultInt(c, "page", 1)
	pageSize := getDefaultInt(c, "pageSize", DefaultPageSize)
	orderField, orderDir := parseOrderParam(strings.TrimSpace(c.Query("order")))
	if orderField != "" && !isAllowedField(orderField, commitSortFields) {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "不支持的排序字段: " + orderField})
		return
	}
	orderClause := buildCommitOrder(orderField, orderDir)
	list, total, err := ListCommits(statDB, filter, page, pageSize, orderClause)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	results := toCommitListItemSlice(list)

	c.JSON(http.StatusOK, CommitListResponse{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Data:     results,
	})
}

// getCommitDetailV2 GET /api/v2/commits/:commitId
// @Summary 获取提交详情
// @Description 根据提交ID获取提交详细信息及关联任务
// @Tags Commits
// @Produce json
// @Param commitId path string true "提交ID"
// @Success 200 {object} CommitDetailResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/commits/{commitId} [get]
func getCommitDetailV2(c *gin.Context) {
	commitID := c.Param("commitId")

	commit, err := GetCommitByID(statDB, commitID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	if commit == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "commit not found"})
		return
	}

	var tasks []models.Task
	if err := statDB.Where("commit_id = ?", commitID).Find(&tasks).Error; err != nil {
		log.Printf("查询 commit %s 关联 tasks 失败: %v", commitID, err)
	}

	taskIds := make([]string, len(tasks))
	silicaList := make([]float64, len(tasks))
	for i, t := range tasks {
		taskIds[i] = t.TaskId
		silicaList[i] = t.Silica
	}
	relatedTasks := GetRelatedTasks(statDB, taskIds, silicaList)

	ancient := commit.CommitAncientMinutes
	real := commit.CommitRealMinutes
	if ancient <= 0 || real <= 0 {
		derivedAncient, derivedReal, err := deriveCommitWorkMinutes(statDB, commit.CommitId)
		if err != nil {
			log.Printf("派生 commit %s 详情工时失败: %v", commit.CommitId, err)
		}
		if ancient <= 0 && derivedAncient > 0 {
			ancient = derivedAncient
			commit.CommitAncientMinutes = derivedAncient
		}
		if real <= 0 && derivedReal > 0 {
			real = derivedReal
			commit.CommitRealMinutes = derivedReal
		}
	}
	efficiencyRatio := utils.CalcEfficiencyRatioManual(ancient,
		real,
		commit.CommitAncientMinutesManual,
		commit.CommitRealMinutesManual)

	c.JSON(http.StatusOK, CommitDetailResponse{
		Commit:           commit,
		RelatedTasks:     relatedTasks,
		EfficiencyRatio:  efficiencyRatio,
		TotalCost:        commit.Cost,
		Silica:           commit.Silica,
		UpstreamTokens:   commit.UpstreamTokens,
		DownstreamTokens: commit.DownstreamTokens,
	})
}

// updateCommitManualV2 PUT /api/v2/commits/:commitId/manual
// @Summary 更新提交人工数据
// @Description 更新提交的人工修改数据
// @Tags Commits
// @Accept json
// @Produce json
// @Param commitId path string true "提交ID"
// @Param data body UpdateCommitManualRequest true "人工数据"
// @Success 200 {object} StatusResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/commits/{commitId}/manual [put]
func updateCommitManualV2(c *gin.Context) {
	commitId := c.Param("commitId")

	var req UpdateCommitManualRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	if err := UpdateCommitManual(statDB, commitId, req.CommitAncientMinutesManual, req.CommitAncientReasonManual, req.CommitRealMinutesManual, req.CommitRealReasonManual); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, StatusResponse{Status: "ok"})
}
