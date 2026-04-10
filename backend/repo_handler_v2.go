package main

import (
	"encoding/json"
	"log"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// listReposV2 GET /api/v2/repos
func listReposV2(c *gin.Context) {
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")

	var startTime, endTime string
	if startDate != "" {
		startT, err := parseDateParam(startDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "startDate 格式错误: " + err.Error()})
			return
		}
		startTime = startT.Format(time.RFC3339)
	}
	if endDate != "" {
		endT, err := parseDateParam(endDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "endDate 格式错误: " + err.Error()})
			return
		}
		endTime = endT.Add(23*time.Hour + 59*time.Minute + 59*time.Second).Format(time.RFC3339)
	}

	aggregates, err := ListRepoAggregates(statDB, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询仓库聚合失败: " + err.Error()})
		return
	}

	// 格式化时间字段
	for _, item := range aggregates {
		if st, ok := item["start_time"].(*time.Time); ok && st != nil {
			item["start_time"] = st.Format("2006-01-02")
		} else {
			item["start_time"] = ""
		}
		if et, ok := item["end_time"].(*time.Time); ok && et != nil {
			item["end_time"] = et.Format("2006-01-02")
		} else {
			item["end_time"] = ""
		}
	}

	// 内存分页
	page := getDefaultInt(c, "page", 1)
	pageSize := getDefaultInt(c, "pageSize", DefaultPageSize)

	total := len(aggregates)
	offset := (page - 1) * pageSize
	if offset > total {
		offset = total
	}
	end := offset + pageSize
	if end > total {
		end = total
	}
	pagedSlice := aggregates[offset:end]

	c.JSON(http.StatusOK, gin.H{
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
		"data":     pagedSlice,
	})
}

// getRepoDetailV2 GET /api/v2/repos/detail
func getRepoDetailV2(c *gin.Context) {
	repoAddr := c.Query("repoAddr")
	if repoAddr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "repoAddr is required"})
		return
	}
	repoBranch := c.Query("repoBranch")
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")

	var startTime, endTime string
	if startDate != "" {
		startT, err := parseDateParam(startDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "startDate 格式错误: " + err.Error()})
			return
		}
		startTime = startT.Format(time.RFC3339)
	}
	if endDate != "" {
		endT, err := parseDateParam(endDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "endDate 格式错误: " + err.Error()})
			return
		}
		endTime = endT.Add(23*time.Hour + 59*time.Minute + 59*time.Second).Format(time.RFC3339)
	}

	// 步骤 1：获取 commits
	commits, err := ListStatCommits(statDB, repoAddr, repoBranch, "", startTime, endTime, 1, 10000, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询 commits 失败: " + err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询分支列表失败: " + err.Error()})
		return
	}

	// 步骤 4.5：为每个 commit 附加聚合 cost/tokens
	commitItems := make([]gin.H, 0, len(commits))
	for _, cm := range commits {
		raw, err := json.Marshal(cm)
		if err != nil {
			log.Printf("序列化 commit %s 失败: %v", cm.CommitID, err)
			continue
		}
		var item gin.H
		if err := json.Unmarshal(raw, &item); err != nil {
			log.Printf("反序列化 commit %s 失败: %v", cm.CommitID, err)
			continue
		}
		var taskIDs []string
		if len(cm.TaskIDs) > 0 && string(cm.TaskIDs) != "null" && string(cm.TaskIDs) != "[]" {
			if err := json.Unmarshal(cm.TaskIDs, &taskIDs); err != nil {
				log.Printf("解析 commit %s task_ids 失败: %v", cm.CommitID, err)
			}
		}
		var silicaList []float64
		if len(cm.TaskIDsSilica) > 0 && string(cm.TaskIDsSilica) != "null" && string(cm.TaskIDsSilica) != "[]" {
			if err := json.Unmarshal(cm.TaskIDsSilica, &silicaList); err != nil {
				log.Printf("解析 commit %s task_ids_silica 失败: %v", cm.CommitID, err)
			}
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
		item["cost"] = totalCost
		item["upstream_tokens"] = upstreamTokens
		item["downstream_tokens"] = downstreamTokens
		item["silica"] = overallSilica
		commitItems = append(commitItems, item)
	}

	// 步骤 5：返回结果
	c.JSON(http.StatusOK, gin.H{
		"repo_addr":   repoAddr,
		"repo_branch": repoBranch,
		"branches":    branches,
		"commits":     commitItems,
		"tasks":       tasks,
		"efficiency": gin.H{
			"repo_ancient_minutes":        repoAncientMinutes,
			"repo_real_minutes":           repoRealMinutes,
			"efficiency_ratio":            efficiencyRatio,
			"repo_ancient_minutes_reason": strings.Join(ancientReasons, "; "),
			"repo_real_minutes_reason":    strings.Join(realReasons, "; "),
		},
		"summary": gin.H{
			"commit_count": len(commits),
			"task_count":   len(tasks),
		},
	})
}

// listRepoBranchesV2 GET /api/v2/repos/branches
func listRepoBranchesV2(c *gin.Context) {
	repoAddr := c.Query("repoAddr")
	if repoAddr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "repoAddr is required"})
		return
	}

	branches, err := ListBranchesByRepoAddr(statDB, repoAddr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询分支列表失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"branches": branches})
}
