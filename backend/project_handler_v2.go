package main

import (
	"encoding/json"
	"fmt"
	"kanban/core/models"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type ProjectListItem struct {
	ProjectId                       string          `json:"project_id"`
	Name                            string          `json:"name"`
	Description                     string          `json:"description"`
	Repos                           json.RawMessage `json:"repos" swaggertype:"string"`
	TaskIds                         json.RawMessage `json:"task_ids" swaggertype:"string"`
	TaskIdsSilica                   json.RawMessage `json:"task_ids_silica" swaggertype:"string"`
	StartTime                       *time.Time      `json:"start_time"`
	EndTime                         *time.Time      `json:"end_time"`
	StartTimeManual                 *time.Time      `json:"start_time_manual"`
	EndTimeManual                   *time.Time      `json:"end_time_manual"`
	UpstreamTokens                  *int64          `json:"upstream_tokens"`
	DownstreamTokens                *int64          `json:"downstream_tokens"`
	Cost                            *float64        `json:"cost"`
	ProjectAncientMinutes           *float64        `json:"project_ancient_minutes"`
	ProjectAncientReason            string          `json:"project_ancient_minutes_reason"`
	ProjectAncientMinutesManual     *float64        `json:"project_ancient_minutes_manual"`
	ProjectAncientReasonManual      string          `json:"project_ancient_minutes_reason_manual"`
	ProjectRealProcessMinutes       *float64        `json:"project_real_process_minutes"`
	ProjectRealProcessReason        string          `json:"project_real_process_minutes_reason"`
	ProjectRealProcessMinutesManual *float64        `json:"project_real_process_minutes_manual"`
	ProjectRealProcessReasonManual  string          `json:"project_real_process_minutes_reason_manual"`
	ProjectRealLeadMinutes          *float64        `json:"project_real_lead_minutes"`
	ProjectRealLeadReason           string          `json:"project_real_lead_minutes_reason"`
	ProjectRealLeadMinutesManual    *float64        `json:"project_real_lead_minutes_manual"`
	ProjectRealLeadReasonManual     string          `json:"project_real_lead_minutes_reason_manual"`
	CreatedAt                       time.Time       `json:"created_at"`
	UpdatedAt                       time.Time       `json:"updated_at"`
	RepoCount                       int             `json:"repo_count"`
	TaskCount                       int             `json:"task_count"`
	UserCount                       int             `json:"user_count"`
	TotalCodeLines                  int64           `json:"total_code_lines"`
	ActualLinesPerDay               *float64        `json:"actual_lines_per_day"`
	EfficiencyRatio                 *float64        `json:"efficiency_ratio"`
	// —— Need(branch) 口径（小数倍数，与项目详情页同源；列表已迁此口径，古法字段保留兼容不再展示）——
	NeedCalendarEfficiencyRatio *float64 `json:"need_calendar_efficiency_ratio"`
	NeedWorkEfficiencyRatio     *float64 `json:"need_work_efficiency_ratio"`
	NeedAICodeRatio             *float64 `json:"need_ai_code_ratio"`
	NeedTotalLocNet             int64    `json:"need_total_loc_net"`
	NeedActualWorkMin           float64  `json:"need_actual_work_min"`
	NeedCost                    float64  `json:"need_cost"`
	NeedEligibleCount           int      `json:"need_eligible_count"`
	NeedTotalCount              int      `json:"need_total_count"`
}

type ProjectListResponse struct {
	Data []ProjectListItem `json:"data"`
}

// ProjectDetailResponse 项目详情（纯 Need(branch) 口径）。项目=一组 Need，所有指标从已选干净 Need 派生。
// 比值为"分子分母守恒"原始倍数（小数，非百分比，前端用 RatioPill）；只计 coverage_eligible 且按口径
// 非 outlier 的干净 Need，并套全局看板口径（排主干分支）。
type ProjectDetailResponse struct {
	Project                     *models.Project `json:"project"`
	NeedCalendarEfficiencyRatio *float64        `json:"need_calendar_efficiency_ratio"` // 日历口径提效比（主）
	NeedWorkEfficiencyRatio     *float64        `json:"need_work_efficiency_ratio"`     // 工作量口径提效比（下钻）
	NeedAICodeRatio             *float64        `json:"need_ai_code_ratio"`             // AI 代码占比（0~1）
	NeedActualCalendarMin       float64         `json:"need_actual_calendar_min"`
	NeedBaselineCalendarMin     float64         `json:"need_baseline_calendar_min"`
	NeedActualWorkMin           float64         `json:"need_actual_work_min"`
	NeedBaselineWorkMin         float64         `json:"need_baseline_work_min"`
	NeedEligibleCount           int             `json:"need_eligible_count"` // 已选且干净、计入指标的 Need 数
	NeedExcludedCount           int             `json:"need_excluded_count"` // 已选但因日历口径 outlier 被自动剔除的 Need 数
	NeedTotalCount              int             `json:"need_total_count"`    // 候选池总数（看板口径，含未选/已排除/不合格）；与 /needs 列表行数同源
	NeedTotalLocNet             int64           `json:"need_total_loc_net"`  // 已选干净 Need 的净 LOC 之和（生成代码量）
	NeedCost                    float64         `json:"need_cost"`           // 选中 Need 会话的成本之和（按 session 去重）
	NeedUpstreamTokens          int64           `json:"need_upstream_tokens"`
	NeedDownstreamTokens        int64           `json:"need_downstream_tokens"`
}

type ProjectConflict struct {
	CommitId    string `json:"commit_id"`
	ProjectId   string `json:"project_id"`
	ProjectName string `json:"project_name"`
}

type ProjectConflictsResponse struct {
	Conflicts []ProjectConflict `json:"conflicts"`
}

type CreateProjectRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

type UpdateProjectRequest struct {
	Name          string          `json:"name"`
	Description   *string         `json:"description"`
	Repos json.RawMessage `json:"repos" swaggertype:"string"`
}

type CheckProjectConflictsRequest struct {
	CommitIds []string `json:"commit_ids"`
}

type UpdateProjectManualRequest struct {
	ProjectAncientMinutesManual     *float64   `json:"project_ancient_minutes_manual"`
	ProjectAncientReasonManual      *string    `json:"project_ancient_minutes_reason_manual"`
	ProjectRealProcessMinutesManual *float64   `json:"project_real_process_minutes_manual"`
	ProjectRealProcessReasonManual  *string    `json:"project_real_process_minutes_reason_manual"`
	ProjectRealLeadMinutesManual    *float64   `json:"project_real_lead_minutes_manual"`
	ProjectRealLeadReasonManual     *string    `json:"project_real_lead_minutes_reason_manual"`
	StartTimeManual                 *time.Time `json:"start_time_manual"`
	EndTimeManual                   *time.Time `json:"end_time_manual"`
}

// 添加到Project中的Repo条件
type ProjectRepo struct {
	RepoAddr           string   `json:"repo_addr"`
	RepoBranch         string   `json:"repo_branch"`
	StartTime          *string  `json:"start_time"`
	EndTime            *string  `json:"end_time"`
	ExcludeCommits     []string `json:"exclude_commits"`
	IncludeOnlyCommits []string `json:"include_only_commits"`
	// Need 维度白/黑名单（need_id）：仅作用于"项目按 Need(branch) 聚合"口径，
	// 与 commit 级的 ExcludeCommits/IncludeOnlyCommits 各自独立、互不影响。
	// 默认全收该 (repo,branch) 下通过看板口径的 Need；可黑掉个别噪声 Need 或只白名单几个。
	ExcludeNeeds     []string `json:"exclude_needs"`
	IncludeOnlyNeeds []string `json:"include_only_needs"`
}

// collectProjectCommits 根据 project 的 repos 配置收集去重后的 commits
func collectProjectCommits(project *models.Project) (map[string]*models.Commit, error) {
	var repos []ProjectRepo
	if len(project.Repos) > 0 && string(project.Repos) != "null" && string(project.Repos) != "[]" {
		if err := json.Unmarshal([]byte(project.Repos), &repos); err != nil {
			return nil, fmt.Errorf("解析 repos 失败: %w", err)
		}
	}

	commitMap := map[string]*models.Commit{}
	for _, rf := range repos {
		startTime := ""
		endTime := ""
		if rf.StartTime != nil {
			startTime = *rf.StartTime
		}
		if rf.EndTime != nil {
			endTime = *rf.EndTime
		}

		// 与项目列表（ListCommitLightByRepoRange 已剔除治理 commit）同口径，避免列表/详情分裂
		commits, _, err := ListCommits(statDB, CommitFilter{
			ExcludeGoverned: true,
			RepoAddr:        rf.RepoAddr,
			RepoBranch:      rf.RepoBranch,
			StartTime:       startTime,
			EndTime:         endTime,
		}, 1, 999999, "commit_time DESC")
		if err != nil {
			return nil, fmt.Errorf("查询 commits 失败: %w", err)
		}

		if len(rf.IncludeOnlyCommits) > 0 {
			whiteSet := make(map[string]bool, len(rf.IncludeOnlyCommits))
			for _, id := range rf.IncludeOnlyCommits {
				whiteSet[id] = true
			}
			for i := range commits {
				if whiteSet[commits[i].CommitId] {
					commitMap[commits[i].CommitId] = &commits[i]
				}
			}
		} else {
			blackSet := make(map[string]bool, len(rf.ExcludeCommits))
			for _, id := range rf.ExcludeCommits {
				blackSet[id] = true
			}
			for i := range commits {
				if !blackSet[commits[i].CommitId] {
					commitMap[commits[i].CommitId] = &commits[i]
				}
			}
		}
	}
	return commitMap, nil
}

// createProjectV2 POST /api/v2/projects
// @Summary 创建项目
// @Description 创建新项目
// @Tags Projects
// @Accept json
// @Produce json
// @Param project body CreateProjectRequest true "项目信息"
// @Success 200 {object} Project
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/projects [post]
func createProjectV2(c *gin.Context) {
	var req CreateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "name 不能为空"})
		return
	}

	desc := ""
	if req.Description != nil {
		desc = *req.Description
	}
	p := &models.Project{
		Name:        req.Name,
		Description: desc,
	}
	projectID, err := CreateProject(statDB, p)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	project, err := GetProject(statDB, projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, project)
}

// listProjectsV2 GET /api/v2/projects
// @Summary 获取项目列表
// @Description 获取所有项目列表
// @Tags Projects
// @Produce json
// @Success 200 {object} ProjectListResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/projects [get]
func listProjectsV2(c *gin.Context) {
	orderField, orderDir := parseOrderParam(strings.TrimSpace(c.Query("order")))
	if orderField != "" && !isAllowedField(orderField, projectSortFields) {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "不支持的排序字段: " + orderField})
		return
	}

	list, err := ListProjects(statDB)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	results := make([]ProjectListItem, len(list))
	for i, p := range list {
		repoCount := 0
		if len(p.Repos) > 0 && string(p.Repos) != "null" && string(p.Repos) != "[]" {
			var repos []json.RawMessage
			if err := json.Unmarshal([]byte(p.Repos), &repos); err == nil {
				repoCount = len(repos)
			}
		}

		// 项目=一组 Need：per-project Need 口径聚合 + 费用（与详情页 queryProjectNeedAgg/Cost 同源）。
		var needCalR, needWorkR, needAIR *float64
		var needLoc int64
		var needWorkMin, needCost float64
		var needEligible, needTotal int
		if scopes, serr := collectProjectRepoBranches(&p); serr == nil {
			if agg, aerr := queryProjectNeedAgg(statDB, scopes); aerr == nil {
				needCalR = efficiencyV2Ratio(agg.BaselineCalendarMin, agg.ActualCalendarMin)
				needWorkR = efficiencyV2Ratio(agg.BaselineWorkMin, agg.ActualWorkMin)
				needAIR = calcNeedAICodeRatio(agg.AICoveredLoc, agg.TotalLocNet)
				needLoc = agg.TotalLocNet
				needWorkMin = agg.ActualWorkMin
				needEligible = agg.EligibleNeeds
				needTotal = agg.TotalNeeds
			} else {
				log.Printf("listProjectsV2: project %s Need 聚合失败: %v", p.ProjectId, aerr)
			}
			if cost, cerr := queryProjectNeedCost(statDB, scopes); cerr == nil {
				needCost = cost.Cost
			}
		} else {
			log.Printf("listProjectsV2: project %s 解析 repos 失败: %v", p.ProjectId, serr)
		}

		results[i] = ProjectListItem{
			ProjectId:                   p.ProjectId,
			Name:                        p.Name,
			Description:                 p.Description,
			Repos:                       json.RawMessage(p.Repos),
			StartTime:                   &p.StartTime,
			EndTime:                     &p.EndTime,
			StartTimeManual:             p.StartTimeManual,
			EndTimeManual:               p.EndTimeManual,
			CreatedAt:                   p.CreatedAt,
			UpdatedAt:                   p.UpdatedAt,
			RepoCount:                   repoCount,
			NeedCalendarEfficiencyRatio: needCalR,
			NeedWorkEfficiencyRatio:     needWorkR,
			NeedAICodeRatio:             needAIR,
			NeedTotalLocNet:             needLoc,
			NeedActualWorkMin:           needWorkMin,
			NeedCost:                    needCost,
			NeedEligibleCount:           needEligible,
			NeedTotalCount:              needTotal,
		}
	}
	sortProjectData(results, orderField, orderDir)

	c.JSON(http.StatusOK, ProjectListResponse{Data: results})
}

// getProjectDetailV2 GET /api/v2/projects/:projectId
// @Summary 获取项目详情
// @Description 根据项目ID获取项目详细信息
// @Tags Projects
// @Produce json
// @Param projectId path string true "项目ID"
// @Success 200 {object} ProjectDetailResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/projects/{projectId} [get]
func getProjectDetailV2(c *gin.Context) {
	projectID := c.Param("projectId")

	project, err := GetProject(statDB, projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	if project == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "project not found"})
		return
	}

	// 项目=一组 Need：解析 project.Repos → (repo,branch) 候选池 → needs 表分子分母守恒聚合 + 费用派生。
	resp := ProjectDetailResponse{Project: project}
	scopes, scopeErr := collectProjectRepoBranches(project)
	if scopeErr != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: scopeErr.Error()})
		return
	}
	if agg, aggErr := queryProjectNeedAgg(statDB, scopes); aggErr != nil {
		log.Printf("查询 project %s Need 口径聚合失败: %v", projectID, aggErr)
	} else {
		// 复用全站共享 helper（actual<=0→nil；baseline=0,actual>0→-100%），口径与 dashboard/部门/组织详情一致。
		resp.NeedCalendarEfficiencyRatio = efficiencyV2Ratio(agg.BaselineCalendarMin, agg.ActualCalendarMin)
		resp.NeedWorkEfficiencyRatio = efficiencyV2Ratio(agg.BaselineWorkMin, agg.ActualWorkMin)
		resp.NeedAICodeRatio = calcNeedAICodeRatio(agg.AICoveredLoc, agg.TotalLocNet)
		resp.NeedActualCalendarMin = agg.ActualCalendarMin
		resp.NeedBaselineCalendarMin = agg.BaselineCalendarMin
		resp.NeedActualWorkMin = agg.ActualWorkMin
		resp.NeedBaselineWorkMin = agg.BaselineWorkMin
		resp.NeedEligibleCount = agg.EligibleNeeds
		resp.NeedExcludedCount = agg.ExcludedNeeds
		resp.NeedTotalCount = agg.TotalNeeds
		resp.NeedTotalLocNet = agg.TotalLocNet
	}
	if cost, costErr := queryProjectNeedCost(statDB, scopes); costErr != nil {
		log.Printf("查询 project %s 费用聚合失败: %v", projectID, costErr)
	} else {
		resp.NeedCost = cost.Cost
		resp.NeedUpstreamTokens = cost.UpstreamTokens
		resp.NeedDownstreamTokens = cost.DownstreamTokens
	}

	c.JSON(http.StatusOK, resp)
}

// updateProjectV2 PUT /api/v2/projects/:projectId
// @Summary 更新项目
// @Description 更新项目信息
// @Tags Projects
// @Accept json
// @Produce json
// @Param projectId path string true "项目ID"
// @Param project body UpdateProjectRequest true "项目信息"
// @Success 200 {object} StatusResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/projects/{projectId} [put]
func updateProjectV2(c *gin.Context) {
	projectID := c.Param("projectId")

	var req UpdateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	project, err := GetProject(statDB, projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	if project == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "project not found"})
		return
	}

	// 校验 repos 必须是 []ProjectRepo 形状再落库，避免坏值入 JSONB 后读侧
	// collectProjectRepoBranches 反序列化失败（详情/needs 整页 500）。
	if len(req.Repos) > 0 && string(req.Repos) != "null" {
		var probe []ProjectRepo
		if err := json.Unmarshal(req.Repos, &probe); err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "repos 必须是 [{repo_addr,repo_branch,...}] 数组: " + err.Error()})
			return
		}
	}

	updates := map[string]interface{}{
		"name":        req.Name,
		"description": project.Description,
		"repos":       models.StringJSON(req.Repos),
		"updated_at":  time.Now(),
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	result := statDB.Model(&models.Project{}).Where("project_id = ?", projectID).Updates(updates)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: result.Error.Error()})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "project not found"})
		return
	}

	// 注：项目=一组 Need；task_ids 已不属于项目模型（v1 遗留），不再写 project_tasks。
	c.JSON(http.StatusOK, StatusResponse{Status: "ok"})
}

// deleteProjectV2 DELETE /api/v2/projects/:projectId
// @Summary 删除项目
// @Description 根据项目ID删除项目
// @Tags Projects
// @Produce json
// @Param projectId path string true "项目ID"
// @Success 200 {object} StatusResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/projects/{projectId} [delete]
func deleteProjectV2(c *gin.Context) {
	projectID := c.Param("projectId")

	if err := DeleteProject(statDB, projectID); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, StatusResponse{Status: "ok"})
}

// updateProjectManualV2 PUT /api/v2/projects/:projectId/manual
// @Summary 更新项目人工数据
// @Description 更新项目的人工修改数据
// @Tags Projects
// @Accept json
// @Produce json
// @Param projectId path string true "项目ID"
// @Param data body UpdateProjectManualRequest true "人工数据"
// @Success 200 {object} StatusResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/projects/{projectId}/manual [put]
func updateProjectManualV2(c *gin.Context) {
	projectID := c.Param("projectId")

	var fields UpdateProjectManualRequest
	if err := c.ShouldBindJSON(&fields); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	if err := UpdateProjectManual(statDB, projectID, fields); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, StatusResponse{Status: "ok"})
}

// addRepoToProjectV2 POST /api/v2/projects/:projectId/repos
// @Summary 为项目添加仓库
// @Description 向项目添加仓库
// @Tags Projects
// @Accept json
// @Produce json
// @Param projectId path string true "项目ID"
// @Param repo body ProjectRepo true "仓库信息"
// @Success 200 {object} StatusResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/projects/{projectId}/repos [post]
func addRepoToProjectV2(c *gin.Context) {
	projectID := c.Param("projectId")

	var req ProjectRepo
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	project, err := GetProject(statDB, projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	if project == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "project not found"})
		return
	}

	var repos []ProjectRepo
	if len(project.Repos) > 0 && string(project.Repos) != "null" && string(project.Repos) != "[]" {
		json.Unmarshal([]byte(project.Repos), &repos)
	}
	repos = append(repos, req)

	reposJSON, _ := json.Marshal(repos)
	project.Repos = models.StringJSON(reposJSON)

	if err := UpdateProject(statDB, project); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, StatusResponse{Status: "ok"})
}

// removeRepoFromProjectV2 DELETE /api/v2/projects/:projectId/repos/:index
// @Summary 从项目移除仓库
// @Description 从项目中移除指定索引的仓库
// @Tags Projects
// @Produce json
// @Param projectId path string true "项目ID"
// @Param index path int true "仓库索引"
// @Success 200 {object} StatusResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v2/projects/{projectId}/repos/{index} [delete]
func removeRepoFromProjectV2(c *gin.Context) {
	projectID := c.Param("projectId")
	indexStr := c.Param("index")
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "index 必须为整数"})
		return
	}

	project, err := GetProject(statDB, projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	if project == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "project not found"})
		return
	}

	var repos []ProjectRepo
	if len(project.Repos) > 0 && string(project.Repos) != "null" && string(project.Repos) != "[]" {
		json.Unmarshal([]byte(project.Repos), &repos)
	}

	if index < 0 || index >= len(repos) {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: fmt.Sprintf("index %d 超出范围 [0, %d)", index, len(repos))})
		return
	}

	repos = append(repos[:index], repos[index+1:]...)
	reposJSON, _ := json.Marshal(repos)
	project.Repos = models.StringJSON(reposJSON)

	if err := UpdateProject(statDB, project); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, StatusResponse{Status: "ok"})
}

// checkProjectConflictsV2 POST /api/v2/projects/check-conflicts
// @Summary 检查项目冲突
// @Description 检查项目中任务或仓库的冲突情况
// @Tags Projects
// @Accept json
// @Produce json
// @Param data body CheckProjectConflictsRequest true "检查数据"
// @Success 200 {object} ProjectConflictsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/projects/check-conflicts [post]
func checkProjectConflictsV2(c *gin.Context) {
	var req CheckProjectConflictsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	projects, err := ListProjects(statDB)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	checkSet := make(map[string]bool, len(req.CommitIds))
	for _, id := range req.CommitIds {
		checkSet[id] = true
	}

	var conflicts []ProjectConflict

	for _, p := range projects {
		commitMap, err := collectProjectCommits(&p)
		if err != nil {
			continue
		}
		for commitID := range commitMap {
			if checkSet[commitID] {
				conflicts = append(conflicts, ProjectConflict{
					CommitId:    commitID,
					ProjectId:   p.ProjectId,
					ProjectName: p.Name,
				})
			}
		}
	}

	c.JSON(http.StatusOK, ProjectConflictsResponse{Conflicts: conflicts})
}
