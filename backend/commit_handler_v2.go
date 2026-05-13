package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"kanban/core/utils"

	"github.com/gin-gonic/gin"
)

type CommitListItem struct {
	CommitId                         string          `json:"commit_id"`
	CommitTime                       time.Time       `json:"commit_time"`
	RepoAddr                         string          `json:"repo_addr"`
	RepoBranch                       string          `json:"repo_branch"`
	GitUserName                      string          `json:"git_user_name"`
	GitUserEmail                     string          `json:"git_user_email"`
	UserId                           string          `json:"user_id"`
	UserName                         string          `json:"user_name"`
	ClientId                         string          `json:"client_id"`
	WorkDir                          string          `json:"work_dir"`
	DiffLines                        int             `json:"diff_lines"`
	CommitAncientMinutes             *float64        `json:"commit_ancient_minutes"`
	CommitAncientMinutesReason       string          `json:"commit_ancient_minutes_reason"`
	CommitAncientMinutesManual       *float64        `json:"commit_ancient_minutes_manual"`
	CommitAncientMinutesReasonManual string          `json:"commit_ancient_minutes_reason_manual"`
	CommitRealMinutes                *float64        `json:"commit_real_minutes"`
	CommitRealMinutesReason          string          `json:"commit_real_minutes_reason"`
	CommitRealMinutesManual          *float64        `json:"commit_real_minutes_manual"`
	CommitRealMinutesReasonManual    string          `json:"commit_real_minutes_reason_manual"`
	CommitRealAiMinutes              *float64        `json:"commit_real_ai_minutes"`
	CommitRealAncientMinutes         *float64        `json:"commit_real_ancient_minutes"`
	TaskIds                          json.RawMessage `json:"task_ids" swaggertype:"string" example:"[\"task1\"]"`
	TaskIdsSilica                    json.RawMessage `json:"task_ids_silica" swaggertype:"string" example:"[\"1.0\"]"`
	TaskAcceptRatios                 json.RawMessage `json:"task_accept_ratios" swaggertype:"string" example:"[\"0.5\"]"`
	Comment                          string          `json:"comment"`
	CreatedAt                        time.Time       `json:"created_at"`
	UpdatedAt                        time.Time       `json:"updated_at"`
	Cost                             float64         `json:"cost"`
	UpstreamTokens                   int64           `json:"upstream_tokens"`
	DownstreamTokens                 int64           `json:"downstream_tokens"`
	Silica                           *float64        `json:"silica"`
	EfficiencyRatio                  *float64        `json:"efficiency_ratio"`
	Org1                             string          `json:"org1"`
	Org2                             string          `json:"org2"`
	Org3                             string          `json:"org3"`
	Org4                             string          `json:"org4"`
	Org5                             string          `json:"org5"`
	Org6                             string          `json:"org6"`
	Org7                             string          `json:"org7"`
	Org8                             string          `json:"org8"`
	Org9                             string          `json:"org9"`
	OrgDisplay                       string          `json:"org_display"`
}

type CommitListResponse struct {
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"pageSize"`
	Data     []CommitListItem `json:"data"`
}

type RelatedTask struct {
	TaskId          string    `json:"task_id"`
	UserName        string    `json:"user_name"`
	StartTime       time.Time `json:"start_time"`
	TaskRealMinutes float64   `json:"task_real_minutes"`
	Silica          float64   `json:"silica"`
	Cost            float64   `json:"cost"`
	DiffLines       int       `json:"diff_lines"`
}

type UpdateCommitManualRequest struct {
	CommitAncientMinutesManual       *float64 `json:"commit_ancient_minutes_manual"`
	CommitAncientMinutesReasonManual *string  `json:"commit_ancient_minutes_reason_manual"`
	CommitRealMinutesManual          *float64 `json:"commit_real_minutes_manual"`
	CommitRealMinutesReasonManual    *string  `json:"commit_real_minutes_reason_manual"`
}

type CommitDetailResponse struct {
	Commit           *StatCommit   `json:"commit"`
	RelatedTasks     []RelatedTask `json:"related_tasks"`
	EfficiencyRatio  *float64      `json:"efficiency_ratio"`
	TotalCost        float64       `json:"total_cost"`
	Silica           *float64      `json:"silica"`
	UpstreamTokens   int64         `json:"upstream_tokens"`
	DownstreamTokens int64         `json:"downstream_tokens"`
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
		OrgFilter: OrgFilter{
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

	filter.resolveOrgUserIDs()
	if (filter.Org1 != "" || filter.Org2 != "" || filter.Org3 != "" || filter.Org4 != "" ||
		filter.Org5 != "" || filter.Org6 != "" || filter.Org7 != "" || filter.Org8 != "" || filter.Org9 != "") && len(filter.OrgUserIDs) == 0 {
		c.JSON(http.StatusOK, CommitListResponse{Total: 0, Page: 1, PageSize: pageSize, Data: []CommitListItem{}})
		return
	}

	list, total, err := ListStatCommits(statDB, filter, page, pageSize, orderClause)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	results := make([]CommitListItem, len(list))
	for i, commit := range list {
		item := CommitListItem{
			CommitId:                         commit.CommitId,
			CommitTime:                       commit.CommitTime,
			RepoAddr:                         commit.RepoAddr,
			RepoBranch:                       commit.RepoBranch,
			GitUserName:                      commit.GitUserName,
			GitUserEmail:                     commit.GitUserEmail,
			UserId:                           commit.UserId,
			UserName:                         commit.UserName,
			ClientId:                         commit.ClientId,
			WorkDir:                          commit.WorkDir,
			DiffLines:                        commit.DiffLines,
			CommitAncientMinutes:             commit.CommitAncientMinutes,
			CommitAncientMinutesReason:       commit.CommitAncientMinutesReason,
			CommitAncientMinutesManual:       commit.CommitAncientMinutesManual,
			CommitAncientMinutesReasonManual: commit.CommitAncientMinutesReasonManual,
			CommitRealMinutes:                commit.CommitRealMinutes,
			CommitRealMinutesReason:          commit.CommitRealMinutesReason,
			CommitRealMinutesManual:          commit.CommitRealMinutesManual,
			CommitRealMinutesReasonManual:    commit.CommitRealMinutesReasonManual,
			CommitRealAiMinutes:              commit.CommitRealAiMinutes,
			CommitRealAncientMinutes:         commit.CommitRealAncientMinutes,
			TaskIds:                          commit.TaskIds,
			TaskIdsSilica:                    commit.TaskIdsSilica,
			TaskAcceptRatios:                 commit.TaskAcceptRatios,
			Comment:                          commit.Comment,
			CreatedAt:                        commit.CreatedAt,
			UpdatedAt:                        commit.UpdatedAt,
			Cost:                             commit.Cost,
			UpstreamTokens:                   commit.UpstreamTokens,
			DownstreamTokens:                 commit.DownstreamTokens,
			Silica:                           commit.Silica,
		}
		item.EfficiencyRatio = utils.CalcEfficiencyRatioManual(commit.CommitAncientMinutes,
			commit.CommitAncientMinutesManual,
			commit.CommitRealMinutes,
			commit.CommitRealMinutesManual)

		if commit.UserId != "" {
			if om, ok := orgMappings[commit.UserId]; ok {
				item.Org1 = om.Org1
				item.Org2 = om.Org2
				item.Org3 = om.Org3
				item.Org4 = om.Org4
				item.Org5 = om.Org5
				item.Org6 = om.Org6
				item.Org7 = om.Org7
				item.Org8 = om.Org8
				item.Org9 = om.Org9
				item.OrgDisplay = getOrgDisplay(om.Org1, om.Org2, om.Org3, om.Org4, om.Org5, om.Org6, om.Org7, om.Org8, om.Org9)
			}
		}

		results[i] = item
	}

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

	commit, err := GetStatCommitByID(statDB, commitID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	if commit == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "commit not found"})
		return
	}

	var relatedTasks []RelatedTask
	var taskIDs []string
	if len(commit.TaskIds) > 0 && string(commit.TaskIds) != "null" && string(commit.TaskIds) != "[]" {
		if err := json.Unmarshal(commit.TaskIds, &taskIDs); err != nil {
			log.Printf("解析 commit task_ids 失败: %v", err)
		}
	}

	var silicaList []float64
	if len(commit.TaskIdsSilica) > 0 && string(commit.TaskIdsSilica) != "null" && string(commit.TaskIdsSilica) != "[]" {
		if err := json.Unmarshal(commit.TaskIdsSilica, &silicaList); err != nil {
			log.Printf("解析 commit task_ids_silica 失败: %v", err)
		}
	}

	for i, taskID := range taskIDs {
		rt := RelatedTask{TaskId: taskID}
		task, err := GetStatTask(statDB, taskID)
		if err != nil {
			log.Printf("查询关联 task %s 失败: %v", taskID, err)
		}
		if task != nil {
			rt.UserName = task.UserName
			if task.StartTime != nil {
				rt.StartTime = *task.StartTime
			}
			if task.TaskRealMinutes != nil {
				rt.TaskRealMinutes = *task.TaskRealMinutes
			}
			rt.Cost = task.Cost
			rt.DiffLines = task.DiffLines
		}
		if i < len(silicaList) {
			rt.Silica = silicaList[i]
		}
		relatedTasks = append(relatedTasks, rt)
	}

	efficiencyRatio := utils.CalcEfficiencyRatioManual(commit.CommitAncientMinutes,
		commit.CommitAncientMinutesManual,
		commit.CommitRealMinutes,
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

	if err := UpdateStatCommitManual(statDB, commitId, req.CommitAncientMinutesManual, req.CommitAncientMinutesReasonManual, req.CommitRealMinutesManual, req.CommitRealMinutesReasonManual); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, StatusResponse{Status: "ok"})
}
