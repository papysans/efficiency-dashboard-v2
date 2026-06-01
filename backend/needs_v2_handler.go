package main

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"kanban/core/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type NeedsV2ListResponse struct {
	Total    int64            `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"pageSize"`
	Data     []NeedsV2Summary `json:"data"`
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
	MergeTs              *time.Time `json:"merge_ts"`
	TotalCalendarMin     float64    `json:"total_calendar_min"`
	BaselineCalendarMin  *float64   `json:"baseline_calendar_min"`
	TotalActiveWorkMin   float64    `json:"total_active_work_corrected_min"`
	BaselineFusedWorkMin *float64   `json:"baseline_fused_work_min"`
	EfficiencyRatio      *float64   `json:"efficiency_ratio"`
	EfficiencyBandLow    *float64   `json:"efficiency_band_low"`
	EfficiencyBandHigh   *float64   `json:"efficiency_band_high"`
	WorkEfficiencyRatio  *float64   `json:"work_efficiency_ratio"`
	ConfidenceLevel      string     `json:"confidence_level"`
	OutlierFlag          bool       `json:"outlier_flag"`
	CoverageEligible     bool       `json:"coverage_eligible"`
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
		q = q.Where("dev_end_ts >= ? OR merge_ts >= ?", *start, *start)
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
	if filter.UserId != "" {
		q = q.Where("primary_user_id = ?", strings.TrimSpace(filter.UserId))
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
	if filter.OutlierOnly {
		q = q.Where("outlier_flag = TRUE")
	}

	if err := q.Count(&resp.Total).Error; err != nil {
		return resp, err
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
		MergeTs:              n.MergeTs,
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
		CoverageEligible:     n.CoverageEligible,
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
