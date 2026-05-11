package main

import (
	"encoding/json"
	"kanban/core/utils"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type RepoListItem struct {
	RepoAddr          string   `json:"repo_addr"`
	RepoBranch        string   `json:"repo_branch"`
	CommitCount       int      `json:"commit_count"`
	StartTime         string   `json:"start_time"`
	EndTime           string   `json:"end_time"`
	SumAncientMinutes *float64 `json:"sum_ancient_minutes"`
	SumRealMinutes    *float64 `json:"sum_real_minutes"`
	TaskCount         int      `json:"task_count"`
	EfficiencyRatio   *float64 `json:"efficiency_ratio"`
}

type ReposListResponse struct {
	Total    int            `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"pageSize"`
	Data     []RepoListItem `json:"data"`
}

type RepoEfficiency struct {
	RepoAncientMinutes       float64  `json:"repo_ancient_minutes"`
	RepoRealMinutes          float64  `json:"repo_real_minutes"`
	EfficiencyRatio          *float64 `json:"efficiency_ratio"`
	RepoAncientMinutesReason string   `json:"repo_ancient_minutes_reason"`
	RepoRealMinutesReason    string   `json:"repo_real_minutes_reason"`
}

type RepoSummary struct {
	CommitCount int `json:"commit_count"`
	TaskCount   int `json:"task_count"`
}

type RepoCommitItem struct {
	CommitID                         string          `json:"commit_id"`
	CommitTime                       time.Time       `json:"commit_time"`
	RepoAddr                         string          `json:"repo_addr"`
	RepoBranch                       string          `json:"repo_branch"`
	GitUserName                      string          `json:"git_user_name"`
	GitUserEmail                     string          `json:"git_user_email"`
	UserID                           string          `json:"user_id"`
	UserName                         string          `json:"user_name"`
	ClientID                         string          `json:"client_id"`
	WorkDir                          string          `json:"work_dir"`
	WorkDirID                        string          `json:"work_dir_id"`
	DiffLines                        *int            `json:"diff_lines"`
	CommitAncientMinutes             *float64        `json:"commit_ancient_minutes"`
	CommitAncientMinutesReason       string          `json:"commit_ancient_minutes_reason"`
	CommitAncientMinutesManual       *float64        `json:"commit_ancient_minutes_manual"`
	CommitAncientMinutesReasonManual string          `json:"commit_ancient_minutes_reason_manual"`
	TaskIDs                          json.RawMessage `json:"task_ids" swaggertype:"string" example:"[\"task1\"]"`
	TaskIDsSilica                    json.RawMessage `json:"task_ids_silica" swaggertype:"string" example:"[\"1.0\"]"`
	CommitRealMinutes                *float64        `json:"commit_real_minutes"`
	CommitRealMinutesReason          string          `json:"commit_real_minutes_reason"`
	CommitRealMinutesManual          *float64        `json:"commit_real_minutes_manual"`
	CommitRealMinutesReasonManual    string          `json:"commit_real_minutes_reason_manual"`
	CommitRealAIMinutes              *float64        `json:"commit_real_ai_minutes"`
	CommitRealAncientMinutes         *float64        `json:"commit_real_ancient_minutes"`
	Comment                          string          `json:"comment"`
	CreatedAt                        *time.Time      `json:"created_at"`
	UpdatedAt                        *time.Time      `json:"updated_at"`
	Cost                             float64         `json:"cost"`
	UpstreamTokens                   int64           `json:"upstream_tokens"`
	DownstreamTokens                 int64           `json:"downstream_tokens"`
	Silica                           *float64        `json:"silica"`
	EfficiencyRatio                  *float64        `json:"efficiency_ratio"`
}

type RepoDetailResponse struct {
	RepoAddr   string           `json:"repo_addr"`
	RepoBranch string           `json:"repo_branch"`
	Branches   []string         `json:"branches"`
	Commits    []RepoCommitItem `json:"commits"`
	Tasks      []StatTask       `json:"tasks"`
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
		if agg.StartTime != nil {
			ri.StartTime = agg.StartTime.Format("2006-01-02")
		}
		if agg.EndTime != nil {
			ri.EndTime = agg.EndTime.Format("2006-01-02")
		}
		ri.SumAncientMinutes = agg.SumAncientMinutes
		ri.SumRealMinutes = agg.SumRealMinutes
		ri.TaskCount = agg.TaskCount
		ri.EfficiencyRatio = agg.EfficiencyRatio
		items = append(items, ri)
	}

	// 内存分页
	page := getDefaultInt(c, "page", 1)
	pageSize := getDefaultInt(c, "pageSize", DefaultPageSize)

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
	repoAddr := c.Query("repoAddr")
	if repoAddr == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "repoAddr is required"})
		return
	}
	repoBranch := c.Query("repoBranch")
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")

	filter := CommitFilter{
		RepoAddr:    repoAddr,
		RepoBranch:  repoBranch,
		GitUserName: strings.TrimSpace(c.Query("gitUserName")),
		UserID:      strings.TrimSpace(c.Query("userId")),
		UserName:    strings.TrimSpace(c.Query("userName")),
		ClientID:    strings.TrimSpace(c.Query("clientId")),
		WorkDir:     strings.TrimSpace(c.Query("workDir")),
		WorkDirID:   strings.TrimSpace(c.Query("workDirId")),
		Org1:        strings.TrimSpace(c.Query("org1")),
		Org2:        strings.TrimSpace(c.Query("org2")),
		Org3:        strings.TrimSpace(c.Query("org3")),
		Org4:        strings.TrimSpace(c.Query("org4")),
		Org5:        strings.TrimSpace(c.Query("org5")),
		Org6:        strings.TrimSpace(c.Query("org6")),
		Org7:        strings.TrimSpace(c.Query("org7")),
		Org8:        strings.TrimSpace(c.Query("org8")),
		Org9:        strings.TrimSpace(c.Query("org9")),
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
	commits, err := ListStatCommits(statDB, filter, 1, 10000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "查询 commits 失败: " + err.Error()})
		return
	}

	// 步骤 2：从 commits 的 task_ids 解析去重所有 taskID，批量获取关联 tasks
	taskIDSet := make(map[string]struct{})
	for _, cm := range commits {
		var ids []string
		if len(cm.TaskIDs) > 0 && string(cm.TaskIDs) != "null" && string(cm.TaskIDs) != "[]" {
			json.Unmarshal(cm.TaskIDs, &ids)
		}
		for _, id := range ids {
			taskIDSet[id] = struct{}{}
		}
	}
	var tasks []StatTask
	for tid := range taskIDSet {
		t, err := GetStatTask(statDB, tid)
		if err != nil || t == nil {
			continue
		}
		tasks = append(tasks, *t)
	}
	// 步骤 3：实时计算 repo 级别效率评估
	var ancientReasons, realReasons []string
	var repoAncientMinutes, repoRealMinutes float64
	for _, cm := range commits {
		ancient := cm.CommitAncientMinutes
		if cm.CommitAncientMinutesManual != nil {
			ancient = cm.CommitAncientMinutesManual
		}
		if ancient != nil {
			repoAncientMinutes += *ancient
		}

		real := cm.CommitRealMinutes
		if cm.CommitRealMinutesManual != nil {
			real = cm.CommitRealMinutesManual
		}
		if real != nil {
			repoRealMinutes += *real
		}

		// 收集 reason
		ancientReason := cm.CommitAncientMinutesReason
		if cm.CommitAncientMinutesReasonManual != "" {
			ancientReason = cm.CommitAncientMinutesReasonManual
		}
		if ancientReason != "" {
			ancientReasons = append(ancientReasons, cm.CommitID[:8]+": "+ancientReason)
		}

		realReason := cm.CommitRealMinutesReason
		if cm.CommitRealMinutesReasonManual != "" {
			realReason = cm.CommitRealMinutesReasonManual
		}
		if realReason != "" {
			realReasons = append(realReasons, cm.CommitID[:8]+": "+realReason)
		}
	}
	var efficiencyRatio *float64
	if repoAncientMinutes > 0 && repoRealMinutes > 0 {
		ratio := utils.CalcEfficiencyRatio(repoAncientMinutes, repoRealMinutes)
		efficiencyRatio = &ratio
	}

	// 步骤 4：获取分支列表
	branches, err := ListBranchesByRepoAddr(statDB, repoAddr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "查询分支列表失败: " + err.Error()})
		return
	}

	// 步骤 4.5：为每个 commit 附加聚合 cost/tokens
	commitItems := make([]RepoCommitItem, 0, len(commits))
	for _, cm := range commits {
		item := RepoCommitItem{
			CommitID:                         cm.CommitID,
			CommitTime:                       cm.CommitTime,
			RepoAddr:                         cm.RepoAddr,
			RepoBranch:                       cm.RepoBranch,
			GitUserName:                      cm.GitUserName,
			GitUserEmail:                     cm.GitUserEmail,
			UserID:                           cm.UserID,
			UserName:                         cm.UserName,
			ClientID:                         cm.ClientID,
			WorkDir:                          cm.WorkDir,
			WorkDirID:                        cm.WorkDirID,
			DiffLines:                        toIntPtr(cm.DiffLines),
			CommitAncientMinutes:             cm.CommitAncientMinutes,
			CommitAncientMinutesReason:       cm.CommitAncientMinutesReason,
			CommitAncientMinutesManual:       cm.CommitAncientMinutesManual,
			CommitAncientMinutesReasonManual: cm.CommitAncientMinutesReasonManual,
			TaskIDs:                          cm.TaskIDs,
			TaskIDsSilica:                    cm.TaskIDsSilica,
			CommitRealMinutes:                cm.CommitRealMinutes,
			CommitRealMinutesReason:          cm.CommitRealMinutesReason,
			CommitRealMinutesManual:          cm.CommitRealMinutesManual,
			CommitRealMinutesReasonManual:    cm.CommitRealMinutesReasonManual,
			CommitRealAIMinutes:              cm.CommitRealAIMinutes,
			CommitRealAncientMinutes:         cm.CommitRealAncientMinutes,
			Comment:                          cm.Comment,
			CreatedAt:                        cm.CreatedAt,
			UpdatedAt:                        cm.UpdatedAt,
			Cost:                             cm.Cost,
			UpstreamTokens:                   cm.UpstreamTokens,
			DownstreamTokens:                 cm.DownstreamTokens,
		}
		if cm.Silica != nil && *cm.Silica > 0 {
			s := math.Round(*cm.Silica*1000) / 10
			item.Silica = &s
		}
		// 计算单条 commit 的效率比率
		commitAncient := cm.CommitAncientMinutes
		if cm.CommitAncientMinutesManual != nil {
			commitAncient = cm.CommitAncientMinutesManual
		}
		commitReal := cm.CommitRealMinutes
		if cm.CommitRealMinutesManual != nil {
			commitReal = cm.CommitRealMinutesManual
		}
		if commitAncient != nil && commitReal != nil && *commitAncient > 0 && *commitReal > 0 {
			ratio := utils.CalcEfficiencyRatio(*commitAncient, *commitReal)
			item.EfficiencyRatio = &ratio
		}
		commitItems = append(commitItems, item)
	}

	// 步骤 5：返回结果
	c.JSON(http.StatusOK, RepoDetailResponse{
		RepoAddr:   repoAddr,
		RepoBranch: repoBranch,
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

func strValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

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
			if ti != nil && tj != nil {
				return ti.After(*tj)
			}
			if ti != nil {
				return true
			}
			if tj != nil {
				return false
			}
			return true
		})
		result = append(result, aggs[0])
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].RepoAddr < result[j].RepoAddr
	})

	return result
}
