package main

import (
	"math"
	"net/http"
	"strings"
	"time"

	"kanban/core/models"
	"kanban/core/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// 项目/仓库主体的「按周时间线」端点。平台 chat-stats 源无项目/仓库维度，user×week 周表也带不了
// 这两个维度（见 EfficiencyDimension 简报）——故这里从底层数据现聚合：
//   仓库 → commits 按 ISO 周分桶，提效比 = CalcEfficiencyRatio(ΣancientΣreal)（百分比口径，已×100）。
//   项目 → 干净 Need 按 dev_end_ts 的 ISO 周分桶，提效比 = AVG(needs.efficiency_ratio)（小数口径→×100 归一为百分比）。
// 两者最终 EfficiencyPct 统一为「百分比」量纲，前端直接画，绝不再 ×100（避免 RepoList/DimensionTrend 的口径雷区）。
// repoAddr / projectId 为空 → 聚合态（全部仓库 / 全部干净 Need 的周趋势）；非空 → 该对象聚焦态。

// EntityTrendPoint 一个 ISO 周的聚合点。EfficiencyPct 已是百分比（300=300%）。
type EntityTrendPoint struct {
	WeekStart     string  `json:"week_start"` // 该周周一（YYYY-MM-DD，date-only 防 UTC 误标）
	EfficiencyPct float64 `json:"efficiency_pct"`
	CommitCount   int     `json:"commit_count"` // 仓库口径：本周提交数
	DiffLines     int     `json:"diff_lines"`   // 仓库口径：本周代码行
	NeedCount     int     `json:"need_count"`   // 项目口径：本周干净 Need 数
	Loc           int64   `json:"loc"`          // 项目口径：本周生成代码净行
}

type EntityTrendResponse struct {
	Data []EntityTrendPoint `json:"data"`
}

// parseTrendDateRange 解析 startDate/endDate（YYYYMMDD），endTime 含当日 23:59:59，与 listReposV2 同口径。
func parseTrendDateRange(c *gin.Context) (startTime, endTime string, err error) {
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	if startDate != "" {
		startT, perr := parseDateParam(startDate)
		if perr != nil {
			return "", "", perr
		}
		startTime = startT.Format(time.RFC3339)
	}
	if endDate != "" {
		endT, perr := parseDateParam(endDate)
		if perr != nil {
			return "", "", perr
		}
		endTime = endT.Add(23*time.Hour + 59*time.Minute + 59*time.Second).Format(time.RFC3339)
	}
	return startTime, endTime, nil
}

// getRepoTrendV2 GET /api/v2/repo-trend
// @Summary 仓库按周提效/提交时间线
// @Description commits 按 ISO 周分桶；repoAddr 为空=全部仓库聚合，非空=单仓库（跨全部分支）
// @Tags Repos
// @Produce json
// @Param repoAddr query string false "仓库地址(空=全部仓库)"
// @Param startDate query string false "开始日期(YYYYMMDD)"
// @Param endDate query string false "结束日期(YYYYMMDD)"
// @Success 200 {object} EntityTrendResponse
// @Router /api/v2/repo-trend [get]
func getRepoTrendV2(c *gin.Context) {
	repoAddr := strings.TrimSpace(c.Query("repoAddr"))
	startTime, endTime, err := parseTrendDateRange(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "日期格式错误: " + err.Error()})
		return
	}
	points, err := listRepoWeeklyTrend(statDB, repoAddr, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "查询仓库周趋势失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, EntityTrendResponse{Data: points})
}

// listRepoWeeklyTrend commits 按 ISO 周（date_trunc('week')）聚合：提交数 / 代码行 / Σ古法 / Σ实际，
// 提效比按周用 CalcEfficiencyRatio 现算。剔除治理 excluded_flag 与列表/详情同口径。
func listRepoWeeklyTrend(db *gorm.DB, repoAddr, startTime, endTime string) ([]EntityTrendPoint, error) {
	type weekRow struct {
		WeekStart  time.Time `gorm:"column:week_start"`
		CommitCnt  int       `gorm:"column:commit_cnt"`
		DiffLines  int       `gorm:"column:diff_lines"`
		SumAncient float64   `gorm:"column:sum_ancient"`
		SumReal    float64   `gorm:"column:sum_real"`
	}
	var rows []weekRow
	q := db.Model(&models.Commit{}).
		Select(`date_trunc('week', commits.commit_time) AS week_start,
			COUNT(*) AS commit_cnt,
			COALESCE(SUM(commits.diff_lines), 0) AS diff_lines,
			COALESCE(SUM(commits.commit_ancient_minutes), 0) AS sum_ancient,
			COALESCE(SUM(commits.commit_real_minutes), 0) AS sum_real`).
		Where("commits.repo_addr IS NOT NULL AND commits.repo_addr != ''").
		Where("commits.excluded_flag = false")
	if repoAddr != "" {
		q = q.Where("commits.repo_addr = ?", repoAddr)
	}
	if startTime != "" {
		q = q.Where("commits.commit_time >= ?", startTime)
	}
	if endTime != "" {
		q = q.Where("commits.commit_time <= ?", endTime)
	}
	if err := q.Group("week_start").Order("week_start").Scan(&rows).Error; err != nil {
		return nil, err
	}
	points := make([]EntityTrendPoint, 0, len(rows))
	for _, r := range rows {
		points = append(points, EntityTrendPoint{
			WeekStart:     r.WeekStart.Format("2006-01-02"),
			EfficiencyPct: utils.CalcEfficiencyRatio(r.SumAncient, r.SumReal),
			CommitCount:   r.CommitCnt,
			DiffLines:     r.DiffLines,
		})
	}
	return points, nil
}

// getProjectTrendV2 GET /api/v2/project-trend
// @Summary 项目按周提效时间线
// @Description 干净 Need 按 dev_end_ts 的 ISO 周分桶；projectId 为空=全部干净 Need 聚合，非空=该项目候选池（已选）
// @Tags Projects
// @Produce json
// @Param projectId query string false "项目ID(空=全部)"
// @Param startDate query string false "开始日期(YYYYMMDD)"
// @Param endDate query string false "结束日期(YYYYMMDD)"
// @Success 200 {object} EntityTrendResponse
// @Router /api/v2/project-trend [get]
func getProjectTrendV2(c *gin.Context) {
	projectID := strings.TrimSpace(c.Query("projectId"))
	startTime, endTime, err := parseTrendDateRange(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "日期格式错误: " + err.Error()})
		return
	}

	var scopes []projectNeedScope
	scoped := false
	if projectID != "" {
		project, perr := GetProject(statDB, projectID)
		if perr != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: perr.Error()})
			return
		}
		if project == nil {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "project not found"})
			return
		}
		scopes, perr = collectProjectRepoBranches(project)
		if perr != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: perr.Error()})
			return
		}
		scoped = true
	}

	points, err := listProjectWeeklyTrend(statDB, scopes, scoped, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "查询项目周趋势失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, EntityTrendResponse{Data: points})
}

// listProjectWeeklyTrend 干净 Need（coverage_eligible 且非 outlier）按 dev_end_ts 的 ISO 周聚合：
// Need 数 / 生成代码净行 / AVG(efficiency_ratio)。efficiency_ratio 是小数口径 → ×100 归一为百分比。
// scoped=true 时套项目候选池（已选 Need 名单）；clause 为空（项目无 repo）→ 空结果。scoped=false=全部干净 Need。
func listProjectWeeklyTrend(db *gorm.DB, scopes []projectNeedScope, scoped bool, startTime, endTime string) ([]EntityTrendPoint, error) {
	type weekRow struct {
		WeekStart time.Time `gorm:"column:week_start"`
		NeedCnt   int       `gorm:"column:need_cnt"`
		Loc       int64     `gorm:"column:loc"`
		AvgEff    *float64  `gorm:"column:avg_eff"`
	}
	q := applyNeedCaliberFilter(db.Model(&models.Need{})).
		Select(`date_trunc('week', dev_end_ts) AS week_start,
			COUNT(*) AS need_cnt,
			COALESCE(SUM(total_loc_net), 0) AS loc,
			AVG(efficiency_ratio) AS avg_eff`).
		Where("coverage_eligible AND NOT outlier_flag").
		Where("dev_end_ts IS NOT NULL AND efficiency_ratio IS NOT NULL")
	if scoped {
		clause, args := buildProjectNeedScopeClause(scopes, true)
		if clause == "" {
			return []EntityTrendPoint{}, nil // 项目无 repo 候选池 → 空
		}
		q = q.Where(clause, args...)
	}
	if startTime != "" {
		q = q.Where("dev_end_ts >= ?", startTime)
	}
	if endTime != "" {
		q = q.Where("dev_end_ts <= ?", endTime)
	}
	var rows []weekRow
	if err := q.Group("week_start").Order("week_start").Scan(&rows).Error; err != nil {
		return nil, err
	}
	points := make([]EntityTrendPoint, 0, len(rows))
	for _, r := range rows {
		eff := 0.0
		if r.AvgEff != nil {
			eff = math.Round(*r.AvgEff*100*10) / 10 // 小数→百分比，保留 1 位
		}
		points = append(points, EntityTrendPoint{
			WeekStart:     r.WeekStart.Format("2006-01-02"),
			EfficiencyPct: eff,
			NeedCount:     r.NeedCnt,
			Loc:           r.Loc,
		})
	}
	return points, nil
}
