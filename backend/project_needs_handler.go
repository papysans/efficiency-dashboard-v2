package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"kanban/core/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ============================================================
// 项目按 Need(branch) 聚合 —— 与 commit 古法口径并行的 Need 口径聚合路径
//
// 关联路径：project.Repos(JSONB) → 解析出 (repo_addr, repo_branch) 候选池
//   → needs 表按 (repo,branch) IN 反查（needs 自带这两列，无需 project_id）。
// 口径：套全局看板口径 applyNeedCaliberFilter（排 active + 主干分支），
//   提效比用分子分母守恒（先 SUM baseline/actual 再相除，绝不取各 Need 均值），
//   只计 coverage_eligible 且按口径非 outlier 的干净 Need（与 dashboard/distribution 一致）。
// ============================================================

// projectNeedScope 一个 (repo,branch) 候选条件 + 该条目的 Need 白/黑名单与时间窗。
type projectNeedScope struct {
	RepoAddr         string
	RepoBranch       string
	StartTime        string
	EndTime          string
	ExcludeNeeds     []string
	IncludeOnlyNeeds []string
}

// collectProjectRepoBranches 从 project.Repos 解析出按 Need 聚合所需的候选池（仅取 repo/branch/窗口/Need 名单）。
// 复用与 collectProjectCommits 同款 JSONB 解析逻辑，但不查 commits。
func collectProjectRepoBranches(project *models.Project) ([]projectNeedScope, error) {
	var repos []ProjectRepo
	if len(project.Repos) > 0 && string(project.Repos) != "null" && string(project.Repos) != "[]" {
		if err := json.Unmarshal([]byte(project.Repos), &repos); err != nil {
			return nil, fmt.Errorf("解析 repos 失败: %w", err)
		}
	}
	scopes := make([]projectNeedScope, 0, len(repos))
	for _, rf := range repos {
		s := projectNeedScope{
			RepoAddr:         rf.RepoAddr,
			RepoBranch:       rf.RepoBranch,
			ExcludeNeeds:     rf.ExcludeNeeds,
			IncludeOnlyNeeds: rf.IncludeOnlyNeeds,
		}
		if rf.StartTime != nil {
			s.StartTime = *rf.StartTime
		}
		if rf.EndTime != nil {
			s.EndTime = *rf.EndTime
		}
		scopes = append(scopes, s)
	}
	return scopes, nil
}

// buildProjectNeedScopeClause 把候选池拼成 needs 表的行选择条件：
// 每个 (repo_addr, repo_branch) 一个 AND 组，组内叠加时间窗（dev_end_ts ∈ [start,end]），
// applyNeedFilter=true 时再叠加该条目的 Need 白/黑名单；多组用 OR 连接。
// scopes 为空（或全部缺 repo_addr）时返回 ("", nil)，调用方据此短路为空结果。
func buildProjectNeedScopeClause(scopes []projectNeedScope, applyNeedFilter bool) (string, []interface{}) {
	var groups []string
	var args []interface{}
	for _, s := range scopes {
		if strings.TrimSpace(s.RepoAddr) == "" {
			continue
		}
		cond := "(repo_addr = ?"
		args = append(args, s.RepoAddr)
		if strings.TrimSpace(s.RepoBranch) != "" {
			cond += " AND repo_branch = ?"
			args = append(args, s.RepoBranch)
		}
		if s.StartTime != "" {
			cond += " AND dev_end_ts >= ?"
			args = append(args, s.StartTime)
		}
		if s.EndTime != "" {
			cond += " AND dev_end_ts <= ?"
			args = append(args, s.EndTime)
		}
		if applyNeedFilter {
			if len(s.IncludeOnlyNeeds) > 0 {
				cond += " AND need_id IN ?"
				args = append(args, s.IncludeOnlyNeeds)
			} else if len(s.ExcludeNeeds) > 0 {
				cond += " AND need_id NOT IN ?"
				args = append(args, s.ExcludeNeeds)
			}
		}
		cond += ")"
		groups = append(groups, cond)
	}
	if len(groups) == 0 {
		return "", nil
	}
	return strings.Join(groups, " OR "), args
}

// projectNeedAgg 项目 Need 口径聚合标量（分子分母守恒所需的 SUM + 干净/剔除计数）。
type projectNeedAgg struct {
	TotalNeeds          int
	EligibleNeeds       int
	ExcludedNeeds       int
	ActualCalendarMin   float64
	BaselineCalendarMin float64
	ActualWorkMin       float64
	BaselineWorkMin     float64
	AICoveredLoc        int64
	TotalLocNet         int64
}

// queryProjectNeedAgg 对候选池内"已选"的 Need 做分子分母守恒聚合。
// FILTER 口径与 queryDashboardNeedAgg 完全一致：日历 SUM 用 NOT calendar_outlier_flag、
// 工作量 SUM 用 NOT work_outlier_flag、AI 占比用 needAICodeAggSelect。
func queryProjectNeedAgg(db *gorm.DB, scopes []projectNeedScope) (*projectNeedAgg, error) {
	var agg projectNeedAgg

	// ① 已选聚合（套 need 白/黑名单）：eligible/excluded 计数与各 SUM 只统计"已选"Need，分子分母守恒。
	selClause, selArgs := buildProjectNeedScopeClause(scopes, true)
	if selClause == "" {
		return &agg, nil // 空候选池：全 0
	}
	q := applyNeedCaliberFilter(db.Model(&models.Need{})).Select(`COUNT(*) FILTER (WHERE coverage_eligible AND NOT outlier_flag) as eligible_needs,
		COUNT(*) FILTER (WHERE coverage_eligible AND calendar_outlier_flag) as excluded_needs,
		COALESCE(SUM(total_calendar_min) FILTER (WHERE coverage_eligible AND NOT calendar_outlier_flag), 0) as actual_calendar_min,
		COALESCE(SUM(baseline_calendar_min) FILTER (WHERE coverage_eligible AND NOT calendar_outlier_flag), 0) as baseline_calendar_min,
		COALESCE(SUM(total_active_work_corrected_min) FILTER (WHERE coverage_eligible AND NOT work_outlier_flag), 0) as actual_work_min,
		COALESCE(SUM(baseline_fused_work_min) FILTER (WHERE coverage_eligible AND NOT work_outlier_flag), 0) as baseline_work_min,
		`+needAICodeAggSelect()).Where(selClause, selArgs...)
	if err := q.Scan(&agg).Error; err != nil {
		return nil, fmt.Errorf("查询项目 Need 聚合失败: %w", err)
	}

	// ② 候选池全量计数（不套 need 名单）：TotalNeeds 语义=候选池总数（含未选/已排除/不合格），
	// 与 getProjectNeedsV2 列表行数同源，避免"全部排除被误判为无 Need"的空态假阴性。
	// 必须放在 ① 的 Scan 之后赋值——Scan 会把未映射的 struct 字段清零。
	poolClause, poolArgs := buildProjectNeedScopeClause(scopes, false)
	var poolCount int64
	if err := applyNeedCaliberFilter(db.Model(&models.Need{})).Where(poolClause, poolArgs...).Count(&poolCount).Error; err != nil {
		return nil, fmt.Errorf("查询项目 Need 候选池计数失败: %w", err)
	}
	agg.TotalNeeds = int(poolCount)
	return &agg, nil
}

// projectNeedCost 项目费用聚合：选中 Need 的会话所产生的 token / 成本。
type projectNeedCost struct {
	Cost             float64
	UpstreamTokens   int64
	DownstreamTokens int64
}

// queryProjectNeedCost 按 Need→sessions→tasks 聚合选中【干净】Need 的费用/tokens。
// 口径与 queryProjectNeedAgg/AI占比一致：只计 coverage_eligible 且非 outlier 的干净 Need，
// 使费用与同页提效比/LOC/工时分母对齐（可与合格 Need 数对账）。
// 关键：先对干净 Need 的 session_ids 去重展开，再对这些 session 的 task 求和——
// 一个 session 可能被多个 Need 引用，按 session 去重可避免跨 Need 重复计数。
func queryProjectNeedCost(db *gorm.DB, scopes []projectNeedScope) (*projectNeedCost, error) {
	var c projectNeedCost
	selClause, selArgs := buildProjectNeedScopeClause(scopes, true)
	if selClause == "" {
		return &c, nil
	}
	sub := applyNeedCaliberFilter(db.Model(&models.Need{})).
		Select("DISTINCT jsonb_array_elements_text(session_ids) AS sid").
		Where(selClause, selArgs...).
		Where("coverage_eligible AND NOT outlier_flag")
	if err := db.Model(&models.Task{}).
		Select("COALESCE(SUM(cost),0) AS cost, COALESCE(SUM(upstream_tokens),0) AS upstream_tokens, COALESCE(SUM(downstream_tokens),0) AS downstream_tokens").
		Where("session_id IN (?)", sub).
		Scan(&c).Error; err != nil {
		return nil, fmt.Errorf("查询项目 Need 费用聚合失败: %w", err)
	}
	return &c, nil
}

// listProjectNeeds 列出候选池内全部 Need（套看板口径，但不套 Need 白/黑名单——
// 全展示并由调用方逐个标记 excluded，方便用户挑选）。
func listProjectNeeds(db *gorm.DB, scopes []projectNeedScope) ([]models.Need, error) {
	clause, args := buildProjectNeedScopeClause(scopes, false)
	if clause == "" {
		return nil, nil
	}
	var rows []models.Need
	q := applyNeedCaliberFilter(db.Model(&models.Need{})).Where(clause, args...).
		Order("CASE WHEN boundary_source = 'lv5_orphan' OR boundary_confidence = 'very_low' THEN 1 ELSE 0 END ASC").
		Order("dev_end_ts DESC NULLS LAST").
		Order("need_id ASC")
	if err := q.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("查询项目 Need 列表失败: %w", err)
	}
	return rows, nil
}

// ProjectNeedItem 项目 Need 列表的一行：复用 NeedsV2Summary 全部字段 + 当前是否被项目排除。
type ProjectNeedItem struct {
	NeedsV2Summary
	Excluded bool `json:"excluded"` // true=被项目配置排除（不计入项目 Need 口径指标）
}

type ProjectNeedsResponse struct {
	Data          []ProjectNeedItem `json:"data"`
	TotalCount    int               `json:"total_count"`    // 候选池内（看板口径）Need 总数（含未选/已排除/不合格）
	EligibleCount int               `json:"eligible_count"` // 已选且干净、计入指标的 Need 数
	ExcludedCount int               `json:"excluded_count"` // 已选但因日历 outlier 自动剔除的 Need 数
	StaleCount    int               `json:"stale_count"`    // 配置里 exclude/include 名单中已不在候选池的陈旧 need_id 数（重算漂移）
}

// getProjectNeedsV2 GET /api/v2/projects/:projectId/needs
// @Summary 项目候选池 Need 列表
// @Description 列出项目 repo 候选池下的全部 Need（含提效比/AI占比/干净度标记/是否被排除），供挑选干净样本
// @Tags Projects
// @Produce json
// @Param projectId path string true "项目ID"
// @Success 200 {object} ProjectNeedsResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/projects/{projectId}/needs [get]
func getProjectNeedsV2(c *gin.Context) {
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

	scopes, err := collectProjectRepoBranches(project)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	rows, err := listProjectNeeds(statDB, scopes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	items := make([]ProjectNeedItem, 0, len(rows))
	poolIDs := make(map[string]bool, len(rows))
	eligible, excludedAuto := 0, 0
	for _, n := range rows {
		poolIDs[n.NeedId] = true
		excluded := isNeedExcludedByScopes(scopes, n.RepoAddr, n.RepoBranch, n.NeedId)
		// 计入指标的计数只统计"已选"Need，与 queryProjectNeedAgg 口径对齐。
		if !excluded {
			if n.CoverageEligible && !n.OutlierFlag {
				eligible++
			}
			if n.CoverageEligible && n.CalendarOutlierFlag {
				excludedAuto++
			}
		}
		items = append(items, ProjectNeedItem{
			NeedsV2Summary: summarizeNeed(n),
			Excluded:       excluded,
		})
	}

	// need_id 漂移对账：配置 exclude_needs∪include_only_needs 中已不在候选池的陈旧 id（重算后 need_id 变了）。
	staleSet := map[string]bool{}
	for _, s := range scopes {
		for _, id := range s.ExcludeNeeds {
			if !poolIDs[id] {
				staleSet[id] = true
			}
		}
		for _, id := range s.IncludeOnlyNeeds {
			if !poolIDs[id] {
				staleSet[id] = true
			}
		}
	}

	c.JSON(http.StatusOK, ProjectNeedsResponse{
		Data:          items,
		TotalCount:    len(items),
		EligibleCount: eligible,
		ExcludedCount: excludedAuto,
		StaleCount:    len(staleSet),
	})
}

// isNeedExcludedByScopes 按 SQL OR 语义判定一个 Need 是否被项目配置排除（不计入 Need 口径指标）：
// 只要存在一个匹配 (repo,branch)（branch 为空=通配该 repo 全部分支）且该条目把它纳入的 scope，即为"已选"。
// 与 buildProjectNeedScopeClause(scopes, true) 的 OR 展开完全同口径，支持通配分支与同 (repo,branch) 重复条目。
func isNeedExcludedByScopes(scopes []projectNeedScope, repoAddr, repoBranch, needID string) bool {
	matched := false
	for _, s := range scopes {
		if s.RepoAddr != repoAddr {
			continue
		}
		if s.RepoBranch != "" && s.RepoBranch != repoBranch {
			continue
		}
		matched = true
		switch {
		case len(s.IncludeOnlyNeeds) > 0:
			if containsString(s.IncludeOnlyNeeds, needID) {
				return false // 被某条目白名单纳入
			}
		case len(s.ExcludeNeeds) > 0:
			if !containsString(s.ExcludeNeeds, needID) {
				return false // 某条目无黑名单命中 → 纳入
			}
		default:
			return false // 某条目无名单 → 默认纳入
		}
	}
	if !matched {
		return false // 理论不可达（rows 来自 scopes）；保守视为纳入，不静默隐藏
	}
	return true // 所有匹配条目都把它挡掉 → 排除
}

func containsString(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// UpdateProjectNeedSelectionRequest 勾选/取消勾选单个 Need（黑名单语义）。
type UpdateProjectNeedSelectionRequest struct {
	RepoAddr   string `json:"repo_addr"`
	RepoBranch string `json:"repo_branch"`
	NeedId     string `json:"need_id"`
	Excluded   bool   `json:"excluded"` // true=排除该 Need（加入黑名单），false=纳入（移出黑名单）
}

// updateProjectNeedSelectionV2 PUT /api/v2/projects/:projectId/needs/selection
// @Summary 更新项目 Need 勾选
// @Description 把单个 Need 纳入/排除项目 Need 口径指标（写入对应 repo 条目的 exclude_needs）
// @Tags Projects
// @Accept json
// @Produce json
// @Param projectId path string true "项目ID"
// @Param data body UpdateProjectNeedSelectionRequest true "勾选数据"
// @Success 200 {object} StatusResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/projects/{projectId}/needs/selection [put]
func updateProjectNeedSelectionV2(c *gin.Context) {
	projectID := c.Param("projectId")

	var req UpdateProjectNeedSelectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}
	if strings.TrimSpace(req.NeedId) == "" || strings.TrimSpace(req.RepoAddr) == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "need_id 与 repo_addr 不能为空"})
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
		if err := json.Unmarshal([]byte(project.Repos), &repos); err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "解析 repos 失败: " + err.Error()})
			return
		}
	}

	matched := false
	for i := range repos {
		if repos[i].RepoAddr != req.RepoAddr {
			continue
		}
		// branch 为空=通配该 repo 全部分支（与 collectProjectCommits/buildProjectNeedScopeClause 同语义）；
		// 否则需精确匹配该 Need 实际所属分支，避免通配配置下勾选必 400。
		if repos[i].RepoBranch != "" && repos[i].RepoBranch != req.RepoBranch {
			continue
		}
		matched = true
		// 按该条目当前语义二选一写：白名单条目只动 IncludeOnlyNeeds、黑名单条目只动 ExcludeNeeds，
		// 避免在白名单条目里累积既不生效又对账不上的死黑名单（白名单耗空回退时会突然生效）。
		if len(repos[i].IncludeOnlyNeeds) > 0 {
			repos[i].IncludeOnlyNeeds = toggleStringSet(repos[i].IncludeOnlyNeeds, req.NeedId, !req.Excluded)
		} else {
			repos[i].ExcludeNeeds = toggleStringSet(repos[i].ExcludeNeeds, req.NeedId, req.Excluded)
		}
	}
	if !matched {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: fmt.Sprintf("(repo=%s, branch=%s) 不在项目配置中", req.RepoAddr, req.RepoBranch)})
		return
	}

	reposJSON, _ := json.Marshal(repos)
	project.Repos = models.StringJSON(reposJSON)
	if err := UpdateProject(statDB, project); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	// 注：Need 勾选只影响 Need 口径指标（详情页实时聚合），不影响 commit 古法聚合，故无需 recalculateProjectAggregates。

	c.JSON(http.StatusOK, StatusResponse{Status: "ok"})
}

// toggleStringSet 在去重字符串集合里增删 id：present=true 确保存在，present=false 确保移除。
func toggleStringSet(list []string, id string, present bool) []string {
	out := make([]string, 0, len(list)+1)
	has := false
	for _, v := range list {
		if v == id {
			has = true
			if present {
				out = append(out, v)
			}
			continue
		}
		out = append(out, v)
	}
	if present && !has {
		out = append(out, id)
	}
	return out
}
