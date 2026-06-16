package main

import (
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"kanban/core/models"
	"kanban/core/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type NeedsV2ListResponse struct {
	Total       int64            `json:"total"`
	FoldedCount int64            `json:"folded_count"` // 默认折叠掉的(coverage_eligible=false)条数,供需求页"已折叠N个"提示;includeAll 时为 0
	Page        int              `json:"page"`
	PageSize    int              `json:"pageSize"`
	Data        []NeedsV2Summary `json:"data"`
}

type NeedsV2Summary struct {
	NeedId               string     `json:"need_id"`
	BoundarySource       string     `json:"boundary_source"`
	BoundaryConfidence   string     `json:"boundary_confidence"`
	Status               string     `json:"status"`
	RepoAddr             string     `json:"repo_addr"`
	RepoBranch           string     `json:"repo_branch"`
	PrimaryUserId        string     `json:"primary_user_id"`
	DevStartTs           *time.Time `json:"dev_start_ts"`
	DevEndTs             *time.Time `json:"dev_end_ts"`
	TotalCalendarMin     float64    `json:"total_calendar_min"`
	BaselineCalendarMin  *float64   `json:"baseline_calendar_min"`
	TotalActiveWorkMin   float64    `json:"total_active_work_corrected_min"`
	BaselineFusedWorkMin *float64   `json:"baseline_fused_work_min"`
	EfficiencyRatio      *float64   `json:"efficiency_ratio"`
	EfficiencyBandLow    *float64   `json:"efficiency_band_low"`
	EfficiencyBandHigh   *float64   `json:"efficiency_band_high"`
	WorkEfficiencyRatio  *float64   `json:"work_efficiency_ratio"`
	ConfidenceLevel      string     `json:"confidence_level"`
	OutlierFlag          bool       `json:"outlier_flag"`          // 派生 = 任一口径异常
	CalendarOutlierFlag  bool       `json:"calendar_outlier_flag"` // 日历提效口径异常
	WorkOutlierFlag      bool       `json:"work_outlier_flag"`     // 工作量提效口径异常
	CoverageEligible     bool       `json:"coverage_eligible"`
	TotalLocNet          int64      `json:"total_loc_net"`
	AICoveredLoc         int64      `json:"ai_covered_loc"`
	AICodeRatio          *float64   `json:"ai_code_ratio"`
	TotalThinkMin        float64    `json:"total_think_min"`
	TotalExecMin         float64    `json:"total_exec_min"`
	TotalVerifyMin       float64    `json:"total_verify_min"`
	Reason               string     `json:"reason"`
}

type NeedsV2DetailResponse struct {
	Need               models.Need                 `json:"need"`
	Sessions           []models.SessionStageMetric `json:"sessions"`
	Commits            []models.Commit             `json:"commits"`
	StageMetrics       []models.SessionStageMetric `json:"stage_metrics"`
	BaselineComponents NeedsV2BaselineComponents   `json:"baseline_components"`
	ConfidenceSignals  models.ObjectJSON           `json:"confidence_signals"`
	QualitySignals     models.ObjectJSON           `json:"quality_signals"`
}

type NeedsV2BaselineComponents struct {
	AlgoThinkMin    *float64 `json:"algo_think_min"`
	AlgoExecMin     *float64 `json:"algo_exec_min"`
	AlgoVerifyMin   *float64 `json:"algo_verify_min"`
	AlgoTotalMin    *float64 `json:"algo_total_min"`
	AnchorKnnMin    *float64 `json:"anchor_knn_min"`
	AnchorKnnReason string   `json:"anchor_knn_reason"`
	LLMThinkMin     *float64 `json:"llm_think_min"`
	LLMExecMin      *float64 `json:"llm_exec_min"`
	LLMVerifyMin    *float64 `json:"llm_verify_min"`
	LLMTotalMin     *float64 `json:"llm_total_min"`
	LLMConfidence   string   `json:"llm_confidence"`
	LLMReason       string   `json:"llm_reason"`
	FusedWorkMin    *float64 `json:"fused_work_min"`
	SpreadWorkMin   *float64 `json:"spread_work_min"`
	CalendarMin     *float64 `json:"calendar_min"`
	TeamWorkDensity *float64 `json:"team_work_density"`
}

type EfficiencyV2AggregateResponse struct {
	Total int64                       `json:"total"`
	Data  []models.UserProductivityV2 `json:"data"`
}

// listNeedsV2 GET /api/v2/needs
// @Summary List v2 Needs
// @Tags NeedsV2
// @Produce json
// @Success 200 {object} NeedsV2ListResponse
// @Router /api/v2/needs [get]
func listNeedsV2(c *gin.Context) {
	if statDB == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: "数据库未连接"})
		return
	}
	orderField, orderDir := parseOrderParam(strings.TrimSpace(c.Query("order")))
	if orderField != "" && !isAllowedField(orderField, needSortFields) {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "不支持的排序字段: " + orderField})
		return
	}
	resp, err := QueryNeedsV2(statDB, NeedsV2Filter{
		StartDate:          c.Query("startDate"),
		EndDate:            c.Query("endDate"),
		RepoAddr:           c.Query("repoAddr"),
		RepoBranch:         c.Query("repoBranch"),
		UserId:             c.Query("userId"),
		Status:             c.Query("status"),
		BoundarySource:     c.Query("boundarySource"),
		BoundaryConfidence: c.Query("boundaryConfidence"),
		ConfidenceLevel:    c.Query("confidenceLevel"),
		OutlierOnly:        c.Query("outlierOnly") == "true",
		IncludeAll:         c.Query("includeAll") == "true",
		FoldNonEligible:    true, // 列表页：默认折叠 coverage_eligible=false 的 need（受 includeAll 放开）
		Page:               parsePage(c.Query("page")),
		PageSize:           parsePageSize(c.Query("pageSize")),
		OrderField:         orderField,
		OrderDir:           orderDir,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// getNeedV2 GET /api/v2/needs/:needId
// @Summary Get v2 Need detail
// @Tags NeedsV2
// @Produce json
// @Param needId path string true "Need ID"
// @Success 200 {object} NeedsV2DetailResponse
// @Router /api/v2/needs/{needId} [get]
func getNeedV2(c *gin.Context) {
	if statDB == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: "数据库未连接"})
		return
	}
	needID := strings.TrimPrefix(c.Param("needId"), "/")
	// /needs 用 catch-all 路由 *needId（gin 不允许同级再注册静态 /needs/distribution，会 panic），
	// 故 distribution 子路径在此分发，复用同一通配入口。
	if needID == "distribution" {
		getNeedsDistributionV2(c)
		return
	}
	resp, err := QueryNeedV2Detail(statDB, needID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "need not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// getEfficiencyV2Aggregate GET /api/v2/efficiency
// @Summary Get v2 user-week aggregate
// @Tags NeedsV2
// @Produce json
// @Success 200 {object} EfficiencyV2AggregateResponse
// @Router /api/v2/efficiency [get]
func getEfficiencyV2Aggregate(c *gin.Context) {
	if statDB == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: "数据库未连接"})
		return
	}
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	userID := c.Query("userId")

	resp, err := QueryEfficiencyV2Aggregate(statDB, startDate, endDate, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// NeedsDistributionHistogramBucket 提效比直方图单桶：同一区间内计入(kept)与剔除(excluded)分别计数。
type NeedsDistributionHistogramBucket struct {
	Label    string `json:"label"`
	Kept     int64  `json:"kept"`
	Excluded int64  `json:"excluded"`
}

// NeedsDistributionExclusionReason 剔除原因计数(reason 文本含子串，原因可能重叠)。
type NeedsDistributionExclusionReason struct {
	Reason string `json:"reason"`
	Label  string `json:"label"`
	Count  int64  `json:"count"`
}

// NeedsDistributionLocBand LOC 速率分档计数(eligible 且 calendar>0)。
type NeedsDistributionLocBand struct {
	Label string `json:"label"`
	Count int64  `json:"count"`
}

type NeedsDistributionWindow struct {
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

// NeedsDistributionResponse 提效分布面板响应：计数 + 分位数 + 直方图 + 剔除原因 + LOC 速率分档。
type NeedsDistributionResponse struct {
	Window           NeedsDistributionWindow            `json:"window"`
	KeptCount        int64                              `json:"kept_count"`
	ExcludedCount    int64                              `json:"excluded_count"`
	CalendarMedian   *float64                           `json:"calendar_median"`
	CalendarP25      *float64                           `json:"calendar_p25"`
	CalendarP75      *float64                           `json:"calendar_p75"`
	WorkMedian       *float64                           `json:"work_median"`
	Histogram        []NeedsDistributionHistogramBucket `json:"histogram"`
	ExclusionReasons []NeedsDistributionExclusionReason `json:"exclusion_reasons"`
	LocRateBands     []NeedsDistributionLocBand         `json:"loc_rate_bands"`
}

// getNeedsDistributionV2 GET /api/v2/needs/distribution
// @Summary v2 提效分布聚合(供前端"提效分布"面板)
// @Tags NeedsV2
// @Produce json
// @Param startDate query string false "开始日期 YYYYMMDD 或 YYYY-MM-DD"
// @Param endDate query string false "结束日期 YYYYMMDD 或 YYYY-MM-DD"
// @Success 200 {object} NeedsDistributionResponse
// @Router /api/v2/needs/distribution [get]
func getNeedsDistributionV2(c *gin.Context) {
	if statDB == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: "数据库未连接"})
		return
	}
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")

	// 复用本文件 parseStartDate/parseEndDate：窗口为 dev_end_ts ∈ [startDate, endDate+24h)。
	var startTime, endTime string
	if start, err := parseStartDate(startDate); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "startDate 格式错误: " + err.Error()})
		return
	} else if start != nil {
		startTime = start.Format(time.RFC3339)
	}
	if end, err := parseEndDate(endDate); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "endDate 格式错误: " + err.Error()})
		return
	} else if end != nil {
		endTime = end.Format(time.RFC3339)
	}

	agg, err := queryNeedsDistributionAgg(statDB, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	resp := NeedsDistributionResponse{
		Window:         NeedsDistributionWindow{StartDate: startDate, EndDate: endDate},
		KeptCount:      agg.KeptCount,
		ExcludedCount:  agg.ExcludedCount,
		CalendarMedian: agg.CalendarMedian,
		CalendarP25:    agg.CalendarP25,
		CalendarP75:    agg.CalendarP75,
		WorkMedian:     agg.WorkMedian,
		Histogram: []NeedsDistributionHistogramBucket{
			{Label: "负提效", Kept: agg.H0Kept, Excluded: agg.H0Excl},
			{Label: "0-50%", Kept: agg.H1Kept, Excluded: agg.H1Excl},
			{Label: "50-100%", Kept: agg.H2Kept, Excluded: agg.H2Excl},
			{Label: "100-200%", Kept: agg.H3Kept, Excluded: agg.H3Excl},
			{Label: "200-500%", Kept: agg.H4Kept, Excluded: agg.H4Excl},
			{Label: ">500%", Kept: agg.H5Kept, Excluded: agg.H5Excl},
		},
		ExclusionReasons: []NeedsDistributionExclusionReason{
			{Reason: "impossible_loc_rate", Label: "物理不可能(>1w行/日)", Count: agg.ReasonLoc},
			{Reason: "efficiency_ratio", Label: "极端提效(>1000%)", Count: agg.ReasonEff},
			{Reason: "actual_to_baseline", Label: "工作量异常", Count: agg.ReasonAtb},
		},
		LocRateBands: []NeedsDistributionLocBand{
			{Label: "≤7 人力可达", Count: agg.Lb1},
			{Label: "7-21", Count: agg.Lb2},
			{Label: "21-50", Count: agg.Lb3},
			{Label: ">50 bulk", Count: agg.Lb4},
		},
	}
	c.JSON(http.StatusOK, resp)
}

type NeedsV2Filter struct {
	StartDate          string
	EndDate            string
	RepoAddr           string
	RepoBranch         string
	UserId             string
	Status             string
	BoundarySource     string
	BoundaryConfidence string
	ConfidenceLevel    string
	OutlierOnly        bool
	IncludeAll         bool
	FoldNonEligible    bool // 仅 /v2/needs 列表页设 true：默认折叠 coverage_eligible=false 的 need；其它复用方(用户详情等)不设 → 不折叠，保数据完整
	Page               int
	PageSize           int
	OrderField         string
	OrderDir           string
}

// QueryNeedsV2 lists v2 Needs with filters; returns persisted summary fields
// without recomputing baselines.
func QueryNeedsV2(db *gorm.DB, filter NeedsV2Filter) (NeedsV2ListResponse, error) {
	resp := NeedsV2ListResponse{Page: filter.Page, PageSize: filter.PageSize}
	if resp.Page < 1 {
		resp.Page = 1
	}
	if resp.PageSize <= 0 {
		resp.PageSize = 20
	}
	q := db.Model(&models.Need{})
	if start, err := parseStartDate(filter.StartDate); err == nil && start != nil {
		q = q.Where("dev_end_ts >= ?", *start)
	}
	if end, err := parseEndDate(filter.EndDate); err == nil && end != nil {
		q = q.Where("dev_start_ts <= ? OR dev_end_ts <= ?", *end, *end)
	}
	if filter.RepoAddr != "" {
		q = q.Where("repo_addr = ?", strings.TrimSpace(filter.RepoAddr))
	}
	if filter.RepoBranch != "" {
		q = q.Where("repo_branch = ?", strings.TrimSpace(filter.RepoBranch))
	}
	if strings.TrimSpace(filter.UserId) != "" {
		// 多口径反查:搜索框可输入真名/工号/git名/UUID 任意一种,统一反查成 primary_user_id 候选集。
		// 候选集恒含 term 原值,故已知 UUID 的精确查询向后兼容;反查无命中时退化为仅原值精确匹配。
		q = q.Where("primary_user_id IN ?", resolveUserIdCandidates(db, filter.UserId))
	}
	// 看板口径：非 active(已交付) + 非主干分支。详见 applyNeedCaliberFilter。
	// includeAll=true 时放开口径，显示 active + 主干分支 + 全部需求，便于排查异常。
	if !filter.IncludeAll {
		q = applyNeedCaliberFilter(q)
	}
	if filter.Status != "" && strings.TrimSpace(filter.Status) != "active" {
		q = q.Where("status = ?", strings.TrimSpace(filter.Status))
	}
	if filter.BoundarySource != "" {
		q = q.Where("boundary_source = ?", strings.TrimSpace(filter.BoundarySource))
	}
	if filter.BoundaryConfidence != "" {
		q = q.Where("boundary_confidence = ?", strings.TrimSpace(filter.BoundaryConfidence))
	}
	if filter.ConfidenceLevel != "" {
		q = q.Where("confidence_level = ?", strings.TrimSpace(filter.ConfidenceLevel))
	}
	// outlier_flag = "撞了 exclusion.scope 内的异常类别"(写侧 ComputeEfficiencyV2Fusion 标记)。
	// 默认隐藏被排除项，与首页/分布/用户周聚合口径一致；OutlierOnly=true 反查作为排查 escape hatch，二者互斥。
	if filter.OutlierOnly {
		q = q.Where("outlier_flag = TRUE")
	} else {
		q = q.Where("NOT outlier_flag")
	}

	// 列表展示口径对齐计算口径：仅列表页(FoldNonEligible)默认折叠 coverage_eligible=false 的 need
	// (commit-only/零日历/低置信/非 merged,即不进提效/省人天/AI 占比计算的那批),根治满屏 "-"。纯展示层、
	// 不改任何计算口径;"显示全部"(includeAll)放开;折叠条件不进共用的 applyNeedCaliberFilter,故不影响总览/
	// 渗透率分母。⚠️ 用户详情等复用 QueryNeedsV2 的调用方不设 FoldNonEligible → 不折叠(关联 need 列表完整)。
	fold := filter.FoldNonEligible && !filter.IncludeAll
	if fold {
		base := q.Session(&gorm.Session{}) // clone：base 算口径内全部(折叠前),q 加折叠条件算折叠后
		if err := base.Count(&resp.FoldedCount).Error; err != nil {
			return resp, err
		}
		q = q.Where("coverage_eligible")
	}

	if err := q.Count(&resp.Total).Error; err != nil {
		return resp, err
	}
	if fold {
		resp.FoldedCount -= resp.Total // 折叠数 = 口径内全部 − 折叠后
	}
	offset := (resp.Page - 1) * resp.PageSize
	var rows []models.Need
	// 低价值需求（孤儿边界 lv5_orphan 或 very_low 置信）排到最后，中间排序由 buildNeedOrder 决定（默认 dev_end_ts DESC）。
	if err := q.Order("CASE WHEN boundary_source = 'lv5_orphan' OR boundary_confidence = 'very_low' THEN 1 ELSE 0 END ASC").
		Order(buildNeedOrder(filter.OrderField, filter.OrderDir)).
		Order("need_id ASC").Limit(resp.PageSize).Offset(offset).Find(&rows).Error; err != nil {
		return resp, err
	}
	for _, n := range rows {
		resp.Data = append(resp.Data, summarizeNeed(n))
	}
	return resp, nil
}

// resolveUserIdCandidates 把搜索框输入的 term(真名/工号/git 用户名/UUID 任意一种)反查成
// needs.primary_user_id 的候选 UUID 集合。多口径并集,且恒并入 term 原值(兼容已知 UUID 精确查询、
// 以及 dept_user/commits 都未覆盖但 needs 确有的 UUID)。
//
// 口径:三表 user 主键同源,均为 dept-sync universal_id 形态的 UUID —— needs.primary_user_id ==
// commits.user_id == dept_user.universal_id(前端 useUserNameMap 即依赖此命中把 UUID 显示成真名,
// 内网实测约 98.6% 命中;models.go DeptUser 上"universal_id 0% 命中"为早期过时注释)。
//   - 路 A dept_user(内网 dept-sync 权威表):真名子串 / 工号精确 / universal_id 精确 → universal_id
//   - 路 C commits:git_user_name 子串(内网格式"AI_真名工号",真名与工号同时内嵌)→ user_id
//     注意 commits.user_name 实为 UUID(非可读名),不可用于真名模糊,故不纳入。
//
// 性能:commits.git_user_name 子串匹配走全表扫(无法用索引),搜索为低频交互,代价可接受。
func resolveUserIdCandidates(db *gorm.DB, term string) []string {
	term = strings.TrimSpace(term)
	if term == "" {
		return nil
	}
	// 转义 LIKE 元字符,避免用户输入的 % _ \ 被当通配符使匹配面失控(如输入单个 % 命中全表、过滤失效)。
	escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(term)
	like := "%" + escaped + "%"
	set := map[string]struct{}{term: {}}
	// 反查失败记日志并降级(继续用已得候选,搜索保持可用),不静默吞错致真名搜索无声失效、排障无迹。
	var deptIDs []string
	if err := db.Model(&models.DeptUser{}).
		Where("real_name ILIKE ? OR emp_no = ? OR universal_id = ?", like, term, term).
		Distinct().Pluck("universal_id", &deptIDs).Error; err != nil {
		log.Printf("[WARN] resolveUserIdCandidates: dept_user 反查失败(降级,真名/工号搜全度受影响): %v", err)
	}
	var commitIDs []string
	if err := db.Model(&models.Commit{}).
		Where("git_user_name ILIKE ?", like).
		Distinct().Pluck("user_id", &commitIDs).Error; err != nil {
		log.Printf("[WARN] resolveUserIdCandidates: commits 反查失败(降级,真名搜全度受影响): %v", err)
	}
	for _, id := range append(deptIDs, commitIDs...) {
		if strings.TrimSpace(id) != "" {
			set[id] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	return out
}

func QueryNeedV2Detail(db *gorm.DB, needID string) (NeedsV2DetailResponse, error) {
	resp := NeedsV2DetailResponse{}
	var need models.Need
	if err := db.Where("need_id = ?", needID).First(&need).Error; err != nil {
		return resp, err
	}
	resp.Need = need
	resp.QualitySignals = need.QualitySignals
	resp.ConfidenceSignals = need.ConfidenceSignals
	resp.BaselineComponents = buildEfficiencyV2BaselineComponents(need)

	sessionIDs := efficiencyV2DecodeJSONStringSlice(need.SessionIds)
	commitIDs := efficiencyV2DecodeJSONStringSlice(need.CommitIds)
	if len(sessionIDs) > 0 {
		// 按活跃工作量降序：让有执行/实质活动的会话排在前面，避免纯聊天会话（exec=0）占满表头。
		if err := db.Where("session_id IN ?", sessionIDs).
			Order("total_active_min DESC").Order("session_start_ts ASC").
			Find(&resp.Sessions).Error; err != nil {
			return resp, err
		}
		resp.StageMetrics = resp.Sessions
	}
	if len(commitIDs) > 0 {
		if err := db.Where("commit_id IN ?", commitIDs).
			Order("commit_time DESC").Find(&resp.Commits).Error; err != nil {
			return resp, err
		}
	}
	return resp, nil
}

func QueryEfficiencyV2Aggregate(db *gorm.DB, startDate, endDate, userID string) (EfficiencyV2AggregateResponse, error) {
	resp := EfficiencyV2AggregateResponse{}
	q := db.Model(&models.UserProductivityV2{})
	if start, err := parseStartDate(startDate); err == nil && start != nil {
		q = q.Where("week_start >= ?", *start)
	}
	if end, err := parseEndDate(endDate); err == nil && end != nil {
		q = q.Where("week_start <= ?", *end)
	}
	if userID != "" {
		q = q.Where("user_id = ?", strings.TrimSpace(userID))
	}
	if err := q.Count(&resp.Total).Error; err != nil {
		return resp, err
	}
	if err := q.Order("week_start DESC").Order("user_id ASC").Find(&resp.Data).Error; err != nil {
		return resp, err
	}
	return resp, nil
}

func summarizeNeed(n models.Need) NeedsV2Summary {
	return NeedsV2Summary{
		NeedId:               n.NeedId,
		BoundarySource:       n.BoundarySource,
		BoundaryConfidence:   n.BoundaryConfidence,
		Status:               n.Status,
		RepoAddr:             n.RepoAddr,
		RepoBranch:           n.RepoBranch,
		PrimaryUserId:        n.PrimaryUserId,
		DevStartTs:           n.DevStartTs,
		DevEndTs:             n.DevEndTs,
		TotalCalendarMin:     n.TotalCalendarMin,
		BaselineCalendarMin:  n.BaselineCalendarMin,
		TotalActiveWorkMin:   n.TotalActiveWorkCorrectedMin,
		BaselineFusedWorkMin: n.BaselineFusedWorkMin,
		EfficiencyRatio:      n.EfficiencyRatio,
		EfficiencyBandLow:    n.EfficiencyLowerBand,
		EfficiencyBandHigh:   n.EfficiencyUpperBand,
		WorkEfficiencyRatio:  n.WorkEfficiencyRatio,
		ConfidenceLevel:      n.ConfidenceLevel,
		OutlierFlag:          n.OutlierFlag,
		CalendarOutlierFlag:  n.CalendarOutlierFlag,
		WorkOutlierFlag:      n.WorkOutlierFlag,
		CoverageEligible:     n.CoverageEligible,
		TotalLocNet:          n.ChangedLoc,
		AICoveredLoc:         n.AICoveredLoc,
		AICodeRatio:          n.AICodeRatio,
		TotalThinkMin:        n.ThinkActiveMin,
		TotalExecMin:         n.ExecutionActiveMin,
		TotalVerifyMin:       n.VerificationActiveMin,
		Reason:               n.Reason,
	}
}

func buildEfficiencyV2BaselineComponents(n models.Need) NeedsV2BaselineComponents {
	return NeedsV2BaselineComponents{
		AlgoThinkMin:    n.BaselineAlgoThinkWorkMin,
		AlgoExecMin:     n.BaselineAlgoExecutionWorkMin,
		AlgoVerifyMin:   n.BaselineAlgoVerificationWorkMin,
		AlgoTotalMin:    n.BaselineAlgoTotalWorkMin,
		AnchorKnnMin:    n.BaselineAnchorKnnWorkMin,
		AnchorKnnReason: n.BaselineAnchorKnnReason,
		LLMThinkMin:     n.BaselineLLMThinkWorkMin,
		LLMExecMin:      n.BaselineLLMExecutionWorkMin,
		LLMVerifyMin:    n.BaselineLLMVerificationWorkMin,
		LLMTotalMin:     n.BaselineLLMTotalWorkMin,
		LLMConfidence:   n.BaselineLLMConfidence,
		LLMReason:       n.BaselineLLMReason,
		FusedWorkMin:    n.BaselineFusedWorkMin,
		SpreadWorkMin:   n.BaselineSpreadWorkMin,
		CalendarMin:     n.BaselineCalendarMin,
		TeamWorkDensity: n.TeamWorkDensityUsed,
	}
}

func efficiencyV2DecodeJSONStringSlice(value models.StringJSON) []string {
	if value == "" || string(value) == "null" || string(value) == "[]" {
		return nil
	}
	var out []string
	_ = jsonUnmarshalQuiet([]byte(value), &out)
	return out
}

func parseStartDate(s string) (*time.Time, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	t, err := parseEfficiencyV2APIDate(s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func parseEndDate(s string) (*time.Time, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	t, err := parseEfficiencyV2APIDate(s)
	if err != nil {
		return nil, err
	}
	end := t.Add(24*time.Hour - time.Second)
	return &end, nil
}

// parseEfficiencyV2APIDate accepts both YYYYMMDD and YYYY-MM-DD so the v2
// read API stays consistent with the CLI surface.
func parseEfficiencyV2APIDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if t, err := time.Parse("20060102", s); err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02", s)
}

func parsePage(s string) int {
	v, _ := strconv.Atoi(s)
	if v < 1 {
		v = 1
	}
	return v
}

func parsePageSize(s string) int {
	v, _ := strconv.Atoi(s)
	if v <= 0 {
		return 20
	}
	if v > 200 {
		return 200
	}
	return v
}

// ============================================================
// 项目「添加来源」仓库选择器数据源 —— 从 needs 表聚合"可作为项目来源的仓库"
// （规范化 repo_addr，与候选池 buildProjectNeedScopeClause 同源、同口径），
// 取代旧的手填 git 地址 / commits 源下拉：勾选的仓库一定能圈到 Need。
// ============================================================

// NeedRepoBranchOption 仓库下一条特性分支的可选项（候选 Need 计数 + 最近活跃）。
type NeedRepoBranchOption struct {
	RepoBranch string     `json:"repo_branch"`
	NeedCount  int64      `json:"need_count"`
	LastActive *time.Time `json:"last_active"`
}

// NeedRepoOption 一个可作为项目来源的仓库（规范化地址 + 候选 Need 计数 + 最近活跃 + 分支清单）。
type NeedRepoOption struct {
	RepoAddr   string                 `json:"repo_addr"`
	NeedCount  int64                  `json:"need_count"`
	LastActive *time.Time             `json:"last_active"`
	Branches   []NeedRepoBranchOption `json:"branches"`
}

// listNeedRepoOptionsV2 GET /api/v2/need-repo-options
// @Summary 可作为项目来源的仓库清单
// @Description 从 needs 表按看板口径聚合仓库（规范化 repo_addr + 候选 Need 数 + 最近活跃 + 分支清单），供项目「添加来源」选择器；不含主干/未交付（与候选池一致）
// @Tags Projects
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/need-repo-options [get]
func listNeedRepoOptionsV2(c *gin.Context) {
	type aggRow struct {
		RepoAddr   string     `gorm:"column:repo_addr"`
		RepoBranch string     `gorm:"column:repo_branch"`
		NeedCount  int64      `gorm:"column:need_count"`
		LastActive *time.Time `gorm:"column:last_active"`
	}
	var rows []aggRow
	q := applyNeedCaliberFilter(statDB.Model(&models.Need{})).
		Select("repo_addr, repo_branch, COUNT(*) AS need_count, MAX(dev_end_ts) AS last_active").
		Where("repo_addr <> ''").
		Group("repo_addr, repo_branch")
	if err := q.Scan(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "查询仓库选项失败: " + err.Error()})
		return
	}

	// 内存按 CanonRepoAddr 汇总：① 剥离 userinfo token（oauth2/glpat/gho，绝不进 UI）；
	// ② 同仓库多写法（带/不带 token、协议差异）归一合并；③ 返回值即候选池匹配口径，前端选了必命中。
	// 不假设 needs.repo_addr 已规范化（历史脏数据可能含 token），读侧主动归一更稳健。
	type repoAgg struct {
		opt      *NeedRepoOption
		branchIx map[string]int // repo_branch -> opt.Branches 下标，按分支合并去重
	}
	idx := make(map[string]*repoAgg)
	out := make([]*NeedRepoOption, 0)
	for _, r := range rows {
		addr := utils.CanonRepoAddr(r.RepoAddr)
		if addr == "" {
			continue // 退化值（"/"、".git"、裸 scheme 等）canon 后为空，不是有效来源，与 buildProjectNeedScopeClause 空地址跳过一致
		}
		ra := idx[addr]
		if ra == nil {
			ra = &repoAgg{opt: &NeedRepoOption{RepoAddr: addr, Branches: []NeedRepoBranchOption{}}, branchIx: map[string]int{}}
			idx[addr] = ra
			out = append(out, ra.opt)
		}
		ra.opt.NeedCount += r.NeedCount
		if r.LastActive != nil && (ra.opt.LastActive == nil || r.LastActive.After(*ra.opt.LastActive)) {
			ra.opt.LastActive = r.LastActive
		}
		if b := strings.TrimSpace(r.RepoBranch); b != "" {
			if bi, ok := ra.branchIx[b]; ok {
				br := &ra.opt.Branches[bi]
				br.NeedCount += r.NeedCount
				if r.LastActive != nil && (br.LastActive == nil || r.LastActive.After(*br.LastActive)) {
					br.LastActive = r.LastActive
				}
			} else {
				ra.branchIx[b] = len(ra.opt.Branches)
				ra.opt.Branches = append(ra.opt.Branches, NeedRepoBranchOption{RepoBranch: b, NeedCount: r.NeedCount, LastActive: r.LastActive})
			}
		}
	}

	// 排序：最近活跃降序（nil 沉底），活跃相同按 Need 数降序——稳定可读，仓库与分支同口径。
	moreRecent := func(ai, aj *time.Time, ci, cj int64) bool {
		if (ai == nil) != (aj == nil) {
			return aj == nil // 非 nil 在前
		}
		if ai != nil && aj != nil && !ai.Equal(*aj) {
			return ai.After(*aj)
		}
		return ci > cj
	}
	sort.SliceStable(out, func(i, j int) bool {
		return moreRecent(out[i].LastActive, out[j].LastActive, out[i].NeedCount, out[j].NeedCount)
	})
	for _, o := range out {
		b := o.Branches
		sort.SliceStable(b, func(i, j int) bool {
			return moreRecent(b[i].LastActive, b[j].LastActive, b[i].NeedCount, b[j].NeedCount)
		})
	}

	c.JSON(http.StatusOK, gin.H{"data": out})
}
