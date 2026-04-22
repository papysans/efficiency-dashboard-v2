package main

import (
	"encoding/json"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

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

	// 转换 map 为 RepoListItem
	items := make([]RepoListItem, 0, len(aggregates))
	for _, m := range aggregates {
		var ri RepoListItem
		ri.RepoAddr, _ = m["repo_addr"].(*string)
		ri.RepoBranch, _ = m["repo_branch"].(*string)
		if v, ok := m["commit_count"].(int); ok {
			ri.CommitCount = v
		}
		if st, ok := m["start_time"].(*time.Time); ok && st != nil {
			ri.StartTime = st.Format("2006-01-02")
		} else {
			ri.StartTime = ""
		}
		if et, ok := m["end_time"].(*time.Time); ok && et != nil {
			ri.EndTime = et.Format("2006-01-02")
		} else {
			ri.EndTime = ""
		}
		ri.SumAncientMinutes, _ = m["sum_ancient_minutes"].(*float64)
		ri.SumRealMinutes, _ = m["sum_real_minutes"].(*float64)
		if v, ok := m["task_count"].(int); ok {
			ri.TaskCount = v
		}
		if v, ok := m["efficiency_ratio"].(float64); ok {
			ri.EfficiencyRatio = &v
		}
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
// @Param startDate query string false "开始日期(YYYYMMDD)"
// @Param endDate query string false "结束日期(YYYYMMDD)"
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

	// 步骤 1：获取 commits
	commits, err := ListStatCommits(statDB, repoAddr, repoBranch, "", startTime, endTime, 1, 10000, nil)
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
	taskMap := make(map[string]*StatTask)
	for i := range tasks {
		taskMap[tasks[i].TaskID] = &tasks[i]
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
		if cm.CommitAncientMinutesReasonManual != nil {
			ancientReason = cm.CommitAncientMinutesReasonManual
		}
		if ancientReason != nil && *ancientReason != "" {
			ancientReasons = append(ancientReasons, cm.CommitID[:8]+": "+*ancientReason)
		}

		realReason := cm.CommitRealMinutesReason
		if cm.CommitRealMinutesReasonManual != nil {
			realReason = cm.CommitRealMinutesReasonManual
		}
		if realReason != nil && *realReason != "" {
			realReasons = append(realReasons, cm.CommitID[:8]+": "+*realReason)
		}
	}
	var efficiencyRatio *float64
	if repoAncientMinutes > 0 && repoRealMinutes > 0 {
		ratio := (repoAncientMinutes / repoRealMinutes) * 100
		ratio = math.Round(ratio*10) / 10
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
			DiffLines:                        cm.DiffLines,
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
		}
		var taskIDs []string
		if len(cm.TaskIDs) > 0 && string(cm.TaskIDs) != "null" && string(cm.TaskIDs) != "[]" {
			json.Unmarshal(cm.TaskIDs, &taskIDs)
		}
		var silicaList []float64
		if len(cm.TaskIDsSilica) > 0 && string(cm.TaskIDsSilica) != "null" && string(cm.TaskIDsSilica) != "[]" {
			json.Unmarshal(cm.TaskIDsSilica, &silicaList)
		}
		var totalCost float64
		var upstreamTokens, downstreamTokens int64
		var weightedSilicaSum, silicaWeightSum float64
		for j, taskID := range taskIDs {
			task := taskMap[taskID]
			if task != nil {
				if task.Cost != nil {
					totalCost += *task.Cost
				}
				silica := 0.0
				if j < len(silicaList) {
					silica = silicaList[j]
				}
				if task.UpstreamTokens != nil {
					upstreamTokens += int64(math.Round(float64(*task.UpstreamTokens) * silica))
				}
				if task.DownstreamTokens != nil {
					downstreamTokens += int64(math.Round(float64(*task.DownstreamTokens) * silica))
				}
				// 加权硅含量：Σ(silica_i * task_diff_i) / Σ(task_diff_i)
				if task.DiffLines != nil && *task.DiffLines > 0 {
					weightedSilicaSum += float64(*task.DiffLines) * silica
					silicaWeightSum += float64(*task.DiffLines)
				}
			}
		}
		// 计算整体硅含量（百分比，1位小数）
		var overallSilica *float64
		if silicaWeightSum > 0 {
			s := math.Round(weightedSilicaSum/silicaWeightSum*1000) / 10
			overallSilica = &s
		}
		item.Cost = totalCost
		item.UpstreamTokens = upstreamTokens
		item.DownstreamTokens = downstreamTokens
		item.Silica = overallSilica
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
			r := (*commitAncient / *commitReal) * 100
			r = math.Round(r*10) / 10
			item.EfficiencyRatio = &r
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
