package main

import (
	"kanban/backend/internal/appconfig"
	"kanban/core/models"
	"kanban/core/utils"
	"log"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type RepoListItem struct {
	RepoAddr          string  `json:"repo_addr"`
	RepoBranch        string  `json:"repo_branch"`
	CommitCount       int     `json:"commit_count"`
	StartTime         string  `json:"start_time"`
	EndTime           string  `json:"end_time"`
	SumAncientMinutes float64 `json:"sum_ancient_minutes"`
	SumRealMinutes    float64 `json:"sum_real_minutes"`
	TaskCount         int     `json:"task_count"`
	EfficiencyRatio   float64 `json:"efficiency_ratio"`
}

type ReposListResponse struct {
	Total    int            `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"pageSize"`
	Data     []RepoListItem `json:"data"`
}

type RepoEfficiency struct {
	RepoAncientMinutes       float64 `json:"repo_ancient_minutes"`
	RepoRealMinutes          float64 `json:"repo_real_minutes"`
	EfficiencyRatio          float64 `json:"efficiency_ratio"`
	RepoAncientMinutesReason string  `json:"repo_ancient_minutes_reason"`
	RepoRealMinutesReason    string  `json:"repo_real_minutes_reason"`
}

type RepoSummary struct {
	CommitCount int `json:"commit_count"`
	TaskCount   int `json:"task_count"`
}

type RepoCommitItem struct {
	CommitId                   string    `json:"commit_id"`
	CommitTime                 time.Time `json:"commit_time"`
	RepoAddr                   string    `json:"repo_addr"`
	RepoBranch                 string    `json:"repo_branch"`
	GitUserName                string    `json:"git_user_name"`
	GitUserEmail               string    `json:"git_user_email"`
	UserId                     string    `json:"user_id"`
	UserName                   string    `json:"user_name"`
	ClientId                   string    `json:"client_id"`
	WorkDir                    string    `json:"work_dir"`
	WorkDirId                  string    `json:"work_dir_id"`
	DiffLines                  int       `json:"diff_lines"`
	CommitAncientMinutes       float64   `json:"commit_ancient_minutes"`
	CommitAncientReason        string    `json:"commit_ancient_minutes_reason"`
	CommitAncientMinutesManual *float64  `json:"commit_ancient_minutes_manual"`
	CommitAncientReasonManual  string    `json:"commit_ancient_minutes_reason_manual"`
	CommitRealMinutes          float64   `json:"commit_real_minutes"`
	CommitRealReason           string    `json:"commit_real_minutes_reason"`
	CommitRealMinutesManual    *float64  `json:"commit_real_minutes_manual"`
	CommitRealReasonManual     string    `json:"commit_real_minutes_reason_manual"`
	CommitRealAiMinutes        float64   `json:"commit_real_ai_minutes"`
	CommitRealAncientMinutes   float64   `json:"commit_real_ancient_minutes"`
	Comment                    string    `json:"comment"`
	CreatedAt                  time.Time `json:"created_at"`
	UpdatedAt                  time.Time `json:"updated_at"`
	Cost                       float64   `json:"cost"`
	UpstreamTokens             int64     `json:"upstream_tokens"`
	DownstreamTokens           int64     `json:"downstream_tokens"`
	Silica                     float64   `json:"silica"`
	EfficiencyRatio            float64   `json:"efficiency_ratio"`
}

type RepoDetailResponse struct {
	RepoAddr   string           `json:"repo_addr"`
	RepoBranch string           `json:"repo_branch"`
	Branches   []string         `json:"branches"`
	Commits    []RepoCommitItem `json:"commits"`
	Tasks      []TaskListItem   `json:"tasks"`
	Efficiency RepoEfficiency   `json:"efficiency"`
	Summary    RepoSummary      `json:"summary"`
}

type RepoBranchesResponse struct {
	Branches []string `json:"branches"`
}

// listReposV2 GET /api/v2/repos
// @Summary 获取仓库列表
// @Description 按条件查询仓库列表
// @Tags Repos
// @Produce json
// @Param startDate query string false "开始日期(YYYYMMDD)"
// @Param endDate query string false "结束日期(YYYYMMDD)"
// @Param page query int false "页码" default(1)
// @Param pageSize query int false "每页数量" default(20)
// @Success 200 {object} ReposListResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/repos [get]
func listReposV2(c *gin.Context) {
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	orderField, orderDir := parseOrderParam(strings.TrimSpace(c.Query("order")))
	if orderField != "" && !isAllowedField(orderField, repoSortFields) {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "不支持的排序字段: " + orderField})
		return
	}

	var startTime, endTime string
	if startDate != "" {
		startT, err := parseDateParam(startDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "startDate 格式错误: " + err.Error()})
			return
		}
		startTime = startT.Format(time.RFC3339)
	}
	if endDate != "" {
		endT, err := parseDateParam(endDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "endDate 格式错误: " + err.Error()})
			return
		}
		endTime = endT.Add(23*time.Hour + 59*time.Minute + 59*time.Second).Format(time.RFC3339)
	}

	aggregates, err := ListRepoAggregates(statDB, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "查询仓库聚合失败: " + err.Error()})
		return
	}
	aggregates = filterPreferredBranchAggregates(aggregates)

	// 转换 RepoAggregate 为 RepoListItem
	items := make([]RepoListItem, 0, len(aggregates))
	for _, agg := range aggregates {
		var ri RepoListItem
		ri.RepoAddr = agg.RepoAddr
		ri.RepoBranch = agg.RepoBranch
		ri.CommitCount = agg.CommitCount
		ri.StartTime = agg.StartTime.Format("2006-01-02")
		ri.EndTime = agg.EndTime.Format("2006-01-02")
		ri.SumAncientMinutes = agg.SumAncientMinutes
		ri.SumRealMinutes = agg.SumRealMinutes
		ri.TaskCount = agg.TaskCount
		ri.EfficiencyRatio = utils.CalcEfficiencyRatio(agg.SumAncientMinutes, agg.SumRealMinutes)
		items = append(items, ri)
	}
	sortRepoData(items, orderField, orderDir)

	// 内存分页
	page := getDefaultInt(c, "page", 1)
	pageSize := getDefaultInt(c, "pageSize", appconfig.DefaultPageSize)

	total := len(items)
	offset := (page - 1) * pageSize
	if offset > total {
		offset = total
	}
	end := offset + pageSize
	if end > total {
		end = total
	}
	pagedItems := items[offset:end]

	c.JSON(http.StatusOK, ReposListResponse{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Data:     pagedItems,
	})
}

// getRepoDetailV2 GET /api/v2/repos/detail
// @Summary 获取仓库详情
// @Description 根据仓库地址获取仓库详细信息
// @Tags Repos
// @Produce json
// @Param repoAddr query string true "仓库地址"
// @Param repoBranch query string false "分支名"
// @Param gitUserName query string false "git用户名"
// @Param userId query string false "用户ID"
// @Param userName query string false "用户名"
// @Param clientId query string false "客户端ID"
// @Param workDir query string false "工作目录"
// @Param workDirId query string false "工作目录ID"
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
// @Success 200 {object} RepoDetailResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/repos/detail [get]
func getRepoDetailV2(c *gin.Context) {
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	filter := CommitFilter{
		RepoAddr:    strings.TrimSpace(c.Query("repoAddr")),
		RepoBranch:  strings.TrimSpace(c.Query("repoBranch")),
		GitUserName: strings.TrimSpace(c.Query("gitUserName")),
		UserId:      strings.TrimSpace(c.Query("userId")),
		UserName:    strings.TrimSpace(c.Query("userName")),
		ClientId:    strings.TrimSpace(c.Query("clientId")),
		WorkDir:     strings.TrimSpace(c.Query("workDir")),
		WorkDirId:   strings.TrimSpace(c.Query("workDirId")),
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

	var startTime, endTime string
	if startDate != "" {
		startT, err := parseDateParam(startDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "startDate 格式错误: " + err.Error()})
			return
		}
		startTime = startT.Format(time.RFC3339)
	}
	if endDate != "" {
		endT, err := parseDateParam(endDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "endDate 格式错误: " + err.Error()})
			return
		}
		endTime = endT.Add(23*time.Hour + 59*time.Minute + 59*time.Second).Format(time.RFC3339)
	}
	filter.StartTime = startTime
	filter.EndTime = endTime

	// 步骤 1：获取 commits
	commits, _, err := ListCommits(statDB, filter, 1, 10000, "commit_time DESC")
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "查询 commits 失败: " + err.Error()})
		return
	}

	// 步骤 1.5：为缺失工时的 commit 派生 实际耗时（提交间隔法）/ 传统耗时（Need 平摊）。
	deriveIDs := make([]string, 0, len(commits))
	for i := range commits {
		if commits[i].CommitAncientMinutes <= 0 || commits[i].CommitRealMinutes <= 0 {
			deriveIDs = append(deriveIDs, commits[i].CommitId)
		}
	}
	if len(deriveIDs) > 0 {
		dAnc, dReal, derr := deriveCommitWorkMinutesBatch(statDB, deriveIDs)
		if derr != nil {
			log.Printf("repo 详情派生 commit 工时失败: %v", derr)
		} else {
			for i := range commits {
				if commits[i].CommitAncientMinutes <= 0 && dAnc[commits[i].CommitId] > 0 {
					commits[i].CommitAncientMinutes = dAnc[commits[i].CommitId]
				}
				if commits[i].CommitRealMinutes <= 0 && dReal[commits[i].CommitId] > 0 {
					commits[i].CommitRealMinutes = dReal[commits[i].CommitId]
				}
			}
		}
	}

	// 步骤 2：从 commits 查关联 tasks
	commitIDs := make([]string, len(commits))
	for i, cm := range commits {
		commitIDs[i] = cm.CommitId
	}
	var taskIDs []string
	if len(commitIDs) > 0 {
		if err := statDB.Model(&models.Task{}).Where("commit_id IN ?", commitIDs).Pluck("task_id", &taskIDs).Error; err != nil {
			log.Printf("查询 commits 关联 tasks 失败: %v", err)
		}
	}

	rawTasks, _, _ := ListTasks(statDB, TaskFilter{
		TaskIds: taskIDs,
	}, 0, 0, "")
	tasks := toTaskListItemSlice(rawTasks)

	// 步骤 3：实时计算 repo 级别效率评估
	var ancientReasons, realReasons []string
	var repoAncientMinutes, repoRealMinutes float64
	for _, cm := range commits {
		ancient := cm.CommitAncientMinutes
		if cm.CommitAncientMinutesManual != nil {
			ancient = *cm.CommitAncientMinutesManual
		}
		repoAncientMinutes += ancient

		real := cm.CommitRealMinutes
		if cm.CommitRealMinutesManual != nil {
			real = *cm.CommitRealMinutesManual
		}
		repoRealMinutes += real

		// 收集 reason
		ancientReason := cm.CommitAncientReason
		if cm.CommitAncientReasonManual != "" {
			ancientReason = cm.CommitAncientReasonManual
		}
		if ancientReason != "" {
			ancientReasons = append(ancientReasons, cm.CommitId[:8]+": "+ancientReason)
		}

		realReason := cm.CommitRealReason
		if cm.CommitRealReasonManual != "" {
			realReason = cm.CommitRealReasonManual
		}
		if realReason != "" {
			realReasons = append(realReasons, cm.CommitId[:8]+": "+realReason)
		}
	}
	efficiencyRatio := utils.CalcEfficiencyRatio(repoAncientMinutes, repoRealMinutes)

	// 步骤 4：获取分支列表
	branches, err := ListBranchesByRepoAddr(statDB, filter.RepoAddr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "查询分支列表失败: " + err.Error()})
		return
	}

	// 步骤 4.5：为每个 commit 附加聚合 cost/tokens
	commitItems := make([]RepoCommitItem, 0, len(commits))
	for _, cm := range commits {
		item := RepoCommitItem{
			CommitId:                   cm.CommitId,
			CommitTime:                 cm.CommitTime,
			RepoAddr:                   cm.RepoAddr,
			RepoBranch:                 cm.RepoBranch,
			GitUserName:                cm.GitUserName,
			GitUserEmail:               cm.GitUserEmail,
			UserId:                     cm.UserId,
			UserName:                   cm.UserName,
			ClientId:                   cm.ClientId,
			WorkDir:                    cm.WorkDir,
			WorkDirId:                  cm.WorkDirId,
			DiffLines:                  cm.DiffLines,
			CommitAncientMinutes:       cm.CommitAncientMinutes,
			CommitAncientReason:        cm.CommitAncientReason,
			CommitAncientMinutesManual: cm.CommitAncientMinutesManual,
			CommitAncientReasonManual:  cm.CommitAncientReasonManual,
			CommitRealMinutes:          cm.CommitRealMinutes,
			CommitRealReason:           cm.CommitRealReason,
			CommitRealMinutesManual:    cm.CommitRealMinutesManual,
			CommitRealReasonManual:     cm.CommitRealReasonManual,
			CommitRealAiMinutes:        cm.CommitRealAiMinutes,
			CommitRealAncientMinutes:   cm.CommitRealNonAiMinutes,
			Comment:                    cm.Comment,
			CreatedAt:                  cm.CreatedAt,
			UpdatedAt:                  cm.UpdatedAt,
			Cost:                       cm.Cost,
			UpstreamTokens:             cm.UpstreamTokens,
			DownstreamTokens:           cm.DownstreamTokens,
		}
		if cm.Silica > 0 {
			item.Silica = math.Round(cm.Silica*1000) / 10
		}
		// 计算单条 commit 的效率比率
		ancient := cm.CommitAncientMinutes
		real := cm.CommitRealMinutes
		item.EfficiencyRatio = utils.CalcEfficiencyRatioManual(ancient,
			real,
			cm.CommitAncientMinutesManual,
			cm.CommitRealMinutesManual)
		commitItems = append(commitItems, item)
	}

	// 步骤 5：返回结果
	c.JSON(http.StatusOK, RepoDetailResponse{
		RepoAddr:   filter.RepoAddr,
		RepoBranch: filter.RepoBranch,
		Branches:   branches,
		Commits:    commitItems,
		Tasks:      tasks,
		Efficiency: RepoEfficiency{
			RepoAncientMinutes:       repoAncientMinutes,
			RepoRealMinutes:          repoRealMinutes,
			EfficiencyRatio:          efficiencyRatio,
			RepoAncientMinutesReason: strings.Join(ancientReasons, "; "),
			RepoRealMinutesReason:    strings.Join(realReasons, "; "),
		},
		Summary: RepoSummary{
			CommitCount: len(commits),
			TaskCount:   len(tasks),
		},
	})
}

// listRepoBranchesV2 GET /api/v2/repos/branches
// @Summary 获取仓库分支列表
// @Description 根据仓库地址获取所有分支
// @Tags Repos
// @Produce json
// @Param repoAddr query string true "仓库地址"
// @Success 200 {object} RepoBranchesResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/repos/branches [get]
func listRepoBranchesV2(c *gin.Context) {
	repoAddr := c.Query("repoAddr")
	if repoAddr == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "repoAddr is required"})
		return
	}

	branches, err := ListBranchesByRepoAddr(statDB, repoAddr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "查询分支列表失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, RepoBranchesResponse{Branches: branches})
}

func branchPriorityScore(branch string) int {
	switch strings.ToLower(branch) {
	case "main":
		return 4
	case "master":
		return 3
	case "dev":
		return 2
	case "develop":
		return 1
	default:
		return 0
	}
}

// filterPreferredBranchAggregates 从同一仓库的多分支聚合数据中筛选出最优分支的记录。
//
// 业务背景：一个仓库通常会在多个分支（如 main、master、dev）上产生代码统计聚合数据，
// 但在仓库级看板中只需展示一条代表性记录，避免重复统计。
//
// 筛选规则（按优先级降序）：
//  1. 分支优先级：main > master > dev > develop > 其他分支
//  2. 若分支优先级相同，则取 EndTime 最新（最近结束）的记录
//
// 最终返回结果按 RepoAddr 字典序排列，每个仓库仅保留一条记录。
func filterPreferredBranchAggregates(aggregates []RepoAggregate) []RepoAggregate {
	repoMap := make(map[string][]RepoAggregate)
	for _, agg := range aggregates {
		addr := agg.RepoAddr
		repoMap[addr] = append(repoMap[addr], agg)
	}

	result := make([]RepoAggregate, 0, len(repoMap))
	for _, aggs := range repoMap {
		sort.Slice(aggs, func(i, j int) bool {
			scoreI := branchPriorityScore(aggs[i].RepoBranch)
			scoreJ := branchPriorityScore(aggs[j].RepoBranch)
			if scoreI != scoreJ {
				return scoreI > scoreJ
			}
			ti, tj := aggs[i].EndTime, aggs[j].EndTime
			return ti.After(tj)
		})
		result = append(result, aggs[0])
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].RepoAddr < result[j].RepoAddr
	})

	return result
}
