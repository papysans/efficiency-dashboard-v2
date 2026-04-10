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

// upsertCommitV2 POST /api/v2/commits
func upsertCommitV2(c *gin.Context) {
	var commit StatCommit
	if err := c.ShouldBindJSON(&commit); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := UpsertStatCommit(statDB, &commit); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// batchUpsertCommitsV2 POST /api/v2/commits/batch
func batchUpsertCommitsV2(c *gin.Context) {
	var commits []StatCommit
	if err := c.ShouldBindJSON(&commits); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(commits) == 0 {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "count": 0})
		return
	}
	if err := BatchUpsertStatCommits(statDB, commits); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "count": len(commits)})
}

// listCommitsV2 GET /api/v2/commits
func listCommitsV2(c *gin.Context) {
	repoAddr := c.Query("repoAddr")
	repoBranch := c.Query("repoBranch")
	userID := c.Query("userId")
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	org1 := c.Query("org1")
	org2 := c.Query("org2")
	org3 := c.Query("org3")
	org4 := c.Query("org4")

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

	// 若指定了 org 参数，通过 orgMappings 过滤出匹配的 user_id 集合
	var orgFilterUserIDs []string
	if org1 != "" || org2 != "" || org3 != "" || org4 != "" {
		for uid, m := range orgMappings {
			if org1 != "" && m.Org1 != org1 {
				continue
			}
			if org2 != "" && m.Org2 != org2 {
				continue
			}
			if org3 != "" && m.Org3 != org3 {
				continue
			}
			if org4 != "" && m.Org4 != org4 {
				continue
			}
			orgFilterUserIDs = append(orgFilterUserIDs, uid)
		}
		// org 有效但无匹配用户 → 直接返回空结果
		if len(orgFilterUserIDs) == 0 {
			c.JSON(http.StatusOK, gin.H{"total": 0, "page": 1, "pageSize": 250, "data": []gin.H{}})
			return
		}
	}

	page := getDefaultInt(c, "page", 1)
	pageSize := getDefaultInt(c, "pageSize", DefaultPageSize)

	total, err := CountStatCommits(statDB, repoAddr, repoBranch, userID, startTime, endTime, orgFilterUserIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	list, err := ListStatCommits(statDB, repoAddr, repoBranch, userID, startTime, endTime, page, pageSize, orgFilterUserIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 批量获取所有关联的 StatTask
	allTaskIDSet := make(map[string]struct{})
	for _, cm := range list {
		var ids []string
		if len(cm.TaskIDs) > 0 && string(cm.TaskIDs) != "null" && string(cm.TaskIDs) != "[]" {
			if err := json.Unmarshal(cm.TaskIDs, &ids); err != nil {
				log.Printf("解析 commit %s task_ids 失败: %v", cm.CommitID, err)
				continue
			}
		}
		for _, id := range ids {
			allTaskIDSet[id] = struct{}{}
		}
	}
	allTaskIDs := make([]string, 0, len(allTaskIDSet))
	for id := range allTaskIDSet {
		allTaskIDs = append(allTaskIDs, id)
	}
	taskMap, err := BatchGetStatTasks(statDB, allTaskIDs)
	if err != nil {
		log.Printf("批量查询 stat tasks 失败: %v", err)
		taskMap = make(map[string]*StatTask)
	}

	// 为每条 commit 计算 efficiency_ratio
	results := make([]gin.H, len(list))
	for i, commit := range list {
		// 聚合 cost/tokens
		var totalCost float64
		var upstreamTokens, downstreamTokens int64
		var taskIDs []string
		if len(commit.TaskIDs) > 0 && string(commit.TaskIDs) != "null" && string(commit.TaskIDs) != "[]" {
			if err := json.Unmarshal(commit.TaskIDs, &taskIDs); err != nil {
				log.Printf("解析 commit %s task_ids 失败: %v", commit.CommitID, err)
			}
		}
		var silicaList []float64
		if len(commit.TaskIDsSilica) > 0 && string(commit.TaskIDsSilica) != "null" && string(commit.TaskIDsSilica) != "[]" {
			if err := json.Unmarshal(commit.TaskIDsSilica, &silicaList); err != nil {
				log.Printf("解析 commit %s task_ids_silica 失败: %v", commit.CommitID, err)
			}
		}
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

		item := gin.H{
			"commit_id":                            commit.CommitID,
			"commit_time":                          commit.CommitTime,
			"repo_addr":                            commit.RepoAddr,
			"repo_branch":                          commit.RepoBranch,
			"git_user_name":                        commit.GitUserName,
			"git_user_email":                       commit.GitUserEmail,
			"user_id":                              commit.UserID,
			"user_name":                            commit.UserName,
			"client_id":                            commit.ClientID,
			"work_path":                            commit.WorkPath,
			"diff_lines":                           commit.DiffLines,
			"commit_ancient_minutes":               commit.CommitAncientMinutes,
			"commit_ancient_minutes_reason":        commit.CommitAncientMinutesReason,
			"commit_ancient_minutes_manual":        commit.CommitAncientMinutesManual,
			"commit_ancient_minutes_reason_manual": commit.CommitAncientMinutesReasonManual,
			"commit_real_minutes":                  commit.CommitRealMinutes,
			"commit_real_minutes_reason":           commit.CommitRealMinutesReason,
			"commit_real_minutes_manual":           commit.CommitRealMinutesManual,
			"commit_real_minutes_reason_manual":    commit.CommitRealMinutesReasonManual,
			"commit_real_ai_minutes":               commit.CommitRealAIMinutes,
			"commit_real_ancient_minutes":          commit.CommitRealAncientMinutes,
			"task_ids":                             commit.TaskIDs,
			"task_ids_silica":                      commit.TaskIDsSilica,
			"comment":                              commit.Comment,
			"created_at":                           commit.CreatedAt,
			"updated_at":                           commit.UpdatedAt,
			"cost":                                 totalCost,
			"upstream_tokens":                      upstreamTokens,
			"downstream_tokens":                    downstreamTokens,
			"silica":                               overallSilica,
		}

		effectiveAncient := commit.CommitAncientMinutes
		if commit.CommitAncientMinutesManual != nil {
			effectiveAncient = commit.CommitAncientMinutesManual
		}
		effectiveReal := commit.CommitRealMinutes
		if commit.CommitRealMinutesManual != nil {
			effectiveReal = commit.CommitRealMinutesManual
		}
		if effectiveAncient != nil && effectiveReal != nil && *effectiveReal > 0 && *effectiveAncient > 0 {
			ratio := (*effectiveAncient / *effectiveReal) * 100
			item["efficiency_ratio"] = math.Round(ratio*10) / 10
		} else {
			item["efficiency_ratio"] = nil
		}

		// 补充 org 字段
		if commit.UserID != nil {
			if om, ok := orgMappings[*commit.UserID]; ok {
				item["org1"] = om.Org1
				item["org2"] = om.Org2
				item["org3"] = om.Org3
				item["org4"] = om.Org4
				parts := []string{}
				for _, v := range []string{om.Org1, om.Org2, om.Org3, om.Org4} {
					if v != "" {
						parts = append(parts, v)
					}
				}
				item["org_display"] = strings.Join(parts, "/")
			} else {
				item["org1"] = ""
				item["org2"] = ""
				item["org3"] = ""
				item["org4"] = ""
				item["org_display"] = ""
			}
		} else {
			item["org1"] = ""
			item["org2"] = ""
			item["org3"] = ""
			item["org4"] = ""
			item["org_display"] = ""
		}

		results[i] = item
	}

	c.JSON(http.StatusOK, gin.H{
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
		"data":     results,
	})
}

// getCommitDetailV2 GET /api/v2/commits/:commitId
func getCommitDetailV2(c *gin.Context) {
	commitID := c.Param("commitId")

	commit, err := GetStatCommitByID(statDB, commitID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if commit == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "commit not found"})
		return
	}

	// 解析关联 task
	type RelatedTask struct {
		TaskID          string     `json:"task_id"`
		UserName        *string    `json:"user_name"`
		StartTime       *time.Time `json:"start_time"`
		TaskRealMinutes *float64   `json:"task_real_minutes"`
		Silica          *float64   `json:"silica"`
		Cost            *float64   `json:"cost"`
		DiffLines       *int       `json:"diff_lines"`
	}

	var relatedTasks []RelatedTask
	var taskIDs []string
	if len(commit.TaskIDs) > 0 && string(commit.TaskIDs) != "null" && string(commit.TaskIDs) != "[]" {
		if err := json.Unmarshal(commit.TaskIDs, &taskIDs); err != nil {
			log.Printf("解析 commit task_ids 失败: %v", err)
		}
	}

	var silicaList []float64
	if len(commit.TaskIDsSilica) > 0 && string(commit.TaskIDsSilica) != "null" && string(commit.TaskIDsSilica) != "[]" {
		if err := json.Unmarshal(commit.TaskIDsSilica, &silicaList); err != nil {
			log.Printf("解析 commit task_ids_silica 失败: %v", err)
		}
	}

	var aiMinutes, ancientMinutes float64
	var totalCost float64
	var upstreamTokens, downstreamTokens int64
	// 加权硅含量: Σ(silica_i * task_diff_i) / Σ(task_diff_i)
	var weightedSilicaSum, silicaWeightSum float64

	for i, taskID := range taskIDs {
		rt := RelatedTask{TaskID: taskID}
		task, err := GetStatTask(statDB, taskID)
		if err != nil {
			log.Printf("查询关联 task %s 失败: %v", taskID, err)
		}
		if task != nil {
			rt.UserName = task.UserName
			rt.StartTime = task.StartTime
			rt.TaskRealMinutes = task.TaskRealMinutes
			rt.Cost = task.Cost
			rt.DiffLines = task.DiffLines
			if task.Cost != nil {
				totalCost += *task.Cost
			}
			// 累加计算 commit_real_ai_minutes 和 commit_real_ancient_minutes
			silica := 0.0
			if i < len(silicaList) {
				silica = silicaList[i]
			}
			if task.TaskRealMinutes != nil {
				aiMinutes += *task.TaskRealMinutes * silica
			}
			if task.TaskAncientMinutes != nil {
				ancientMinutes += *task.TaskAncientMinutes * (1 - silica)
			}
			// tokens: task.tokens * silica
			if task.UpstreamTokens != nil {
				upstreamTokens += int64(math.Round(float64(*task.UpstreamTokens) * silica))
			}
			if task.DownstreamTokens != nil {
				downstreamTokens += int64(math.Round(float64(*task.DownstreamTokens) * silica))
			}
			// 加权硅含量
			if task.DiffLines != nil && *task.DiffLines > 0 {
				w := float64(*task.DiffLines) * silica
				weightedSilicaSum += w
				silicaWeightSum += float64(*task.DiffLines)
			}
		}
		if i < len(silicaList) {
			s := silicaList[i]
			rt.Silica = &s
		}
		relatedTasks = append(relatedTasks, rt)
	}

	// 处理空 task_ids 情况
	if len(taskIDs) == 0 {
		aiMinutes = 0
		if commit.CommitAncientMinutes != nil {
			ancientMinutes = *commit.CommitAncientMinutes
		}
	}
	// commit 实际耗时 = Σ(task_real_minutes * silica)，仅计算 AI 辅助部分的实际耗时
	commitRealMinutes := aiMinutes

	// 赋值到 commit 对象
	commit.CommitRealAIMinutes = &aiMinutes
	commit.CommitRealAncientMinutes = &ancientMinutes
	commit.CommitRealMinutes = &commitRealMinutes

	// 异步写回数据库
	go func(cID string, tIDs, tSilica json.RawMessage, rm, raiM, raM float64) {
		err := UpdateStatCommitTaskAssoc(statDB, cID, tIDs, tSilica, &rm, &raiM, &raM, nil)
		if err != nil {
			log.Printf("异步更新 commit_real_minutes 失败: %v", err)
		}
	}(commit.CommitID, commit.TaskIDs, commit.TaskIDsSilica, commitRealMinutes, aiMinutes, ancientMinutes)

	// 计算 efficiency_ratio
	var efficiencyRatio *float64
	effectiveAncient := commit.CommitAncientMinutes
	if commit.CommitAncientMinutesManual != nil {
		effectiveAncient = commit.CommitAncientMinutesManual
	}
	effectiveReal := commit.CommitRealMinutes
	if commit.CommitRealMinutesManual != nil {
		effectiveReal = commit.CommitRealMinutesManual
	}
	if effectiveAncient != nil && effectiveReal != nil && *effectiveReal > 0 && *effectiveAncient > 0 {
		ratio := (*effectiveAncient / *effectiveReal) * 100
		ratio = math.Round(ratio*10) / 10
		efficiencyRatio = &ratio
	}

	// 计算整体硅含量
	var overallSilica *float64
	if silicaWeightSum > 0 {
		s := math.Round(weightedSilicaSum/silicaWeightSum*1000) / 10 // 百分比，1位小数
		overallSilica = &s
	}

	c.JSON(http.StatusOK, gin.H{
		"commit":            commit,
		"related_tasks":     relatedTasks,
		"efficiency_ratio":  efficiencyRatio,
		"total_cost":        math.Round(totalCost*10000) / 10000,
		"silica":            overallSilica,
		"upstream_tokens":   upstreamTokens,
		"downstream_tokens": downstreamTokens,
	})
}

// updateCommitManualV2 PUT /api/v2/commits/:commitId/manual
func updateCommitManualV2(c *gin.Context) {
	commitId := c.Param("commitId")

	var req struct {
		CommitAncientMinutesManual       *float64 `json:"commit_ancient_minutes_manual"`
		CommitAncientMinutesReasonManual *string  `json:"commit_ancient_minutes_reason_manual"`
		CommitRealMinutesManual          *float64 `json:"commit_real_minutes_manual"`
		CommitRealMinutesReasonManual    *string  `json:"commit_real_minutes_reason_manual"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := UpdateStatCommitManual(statDB, commitId, req.CommitAncientMinutesManual, req.CommitAncientMinutesReasonManual, req.CommitRealMinutesManual, req.CommitRealMinutesReasonManual); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
