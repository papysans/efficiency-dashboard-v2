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

type CommitListItem struct {
	CommitID                         string          `json:"commit_id"`
	CommitTime                       *time.Time      `json:"commit_time"`
	RepoAddr                         *string         `json:"repo_addr"`
	RepoBranch                       *string         `json:"repo_branch"`
	GitUserName                      *string         `json:"git_user_name"`
	GitUserEmail                     *string         `json:"git_user_email"`
	UserID                           *string         `json:"user_id"`
	UserName                         *string         `json:"user_name"`
	ClientID                         *string         `json:"client_id"`
	WorkDir                          *string         `json:"work_dir"`
	DiffLines                        *int            `json:"diff_lines"`
	CommitAncientMinutes             *float64        `json:"commit_ancient_minutes"`
	CommitAncientMinutesReason       *string         `json:"commit_ancient_minutes_reason"`
	CommitAncientMinutesManual       *float64        `json:"commit_ancient_minutes_manual"`
	CommitAncientMinutesReasonManual *string         `json:"commit_ancient_minutes_reason_manual"`
	CommitRealMinutes                *float64        `json:"commit_real_minutes"`
	CommitRealMinutesReason          *string         `json:"commit_real_minutes_reason"`
	CommitRealMinutesManual          *float64        `json:"commit_real_minutes_manual"`
	CommitRealMinutesReasonManual    *string         `json:"commit_real_minutes_reason_manual"`
	CommitRealAIMinutes              *float64        `json:"commit_real_ai_minutes"`
	CommitRealAncientMinutes         *float64        `json:"commit_real_ancient_minutes"`
	TaskIDs                          json.RawMessage `json:"task_ids" swaggertype:"string" example:"[\"task1\"]"`
	TaskIDsSilica                    json.RawMessage `json:"task_ids_silica" swaggertype:"string" example:"[\"1.0\"]"`
	Comment                          *string         `json:"comment"`
	CreatedAt                        *time.Time      `json:"created_at"`
	UpdatedAt                        *time.Time      `json:"updated_at"`
	Cost                             float64         `json:"cost"`
	UpstreamTokens                   int64           `json:"upstream_tokens"`
	DownstreamTokens                 int64           `json:"downstream_tokens"`
	Silica                           *float64        `json:"silica"`
	EfficiencyRatio                  *float64        `json:"efficiency_ratio"`
	Org1                             string          `json:"org1"`
	Org2                             string          `json:"org2"`
	Org3                             string          `json:"org3"`
	Org4                             string          `json:"org4"`
	OrgDisplay                       string          `json:"org_display"`
}

type CommitListResponse struct {
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"pageSize"`
	Data     []CommitListItem `json:"data"`
}

type RelatedTask struct {
	TaskID          string     `json:"task_id"`
	UserName        *string    `json:"user_name"`
	StartTime       *time.Time `json:"start_time"`
	TaskRealMinutes *float64   `json:"task_real_minutes"`
	Silica          *float64   `json:"silica"`
	Cost            *float64   `json:"cost"`
	DiffLines       *int       `json:"diff_lines"`
}

type UpdateCommitManualRequest struct {
	CommitAncientMinutesManual       *float64 `json:"commit_ancient_minutes_manual"`
	CommitAncientMinutesReasonManual *string  `json:"commit_ancient_minutes_reason_manual"`
	CommitRealMinutesManual          *float64 `json:"commit_real_minutes_manual"`
	CommitRealMinutesReasonManual    *string  `json:"commit_real_minutes_reason_manual"`
}

type CommitDetailResponse struct {
	Commit           *StatCommit   `json:"commit"`
	RelatedTasks     []RelatedTask `json:"related_tasks"`
	EfficiencyRatio  *float64      `json:"efficiency_ratio"`
	TotalCost        float64       `json:"total_cost"`
	Silica           *float64      `json:"silica"`
	UpstreamTokens   int64         `json:"upstream_tokens"`
	DownstreamTokens int64         `json:"downstream_tokens"`
}

// listCommitsV2 GET /api/v2/commits
// @Summary 获取提交列表
// @Description 按条件查询提交列表
// @Tags Commits
// @Produce json
// @Param repoAddr query string true "仓库地址"
// @Param repoBranch query string false "分支名"
// @Param userId query string false "用户ID"
// @Param org1 query string false "一级组织"
// @Param org2 query string false "二级组织"
// @Param org3 query string false "三级组织"
// @Param org4 query string false "四级组织"
// @Param startDate query string false "开始日期(YYYYMMDD)"
// @Param endDate query string false "结束日期(YYYYMMDD)"
// @Param page query int false "页码" default(1)
// @Param pageSize query int false "每页数量" default(20)
// @Success 200 {object} CommitListResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/commits [get]
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
			c.JSON(http.StatusOK, CommitListResponse{Total: 0, Page: 1, PageSize: 250, Data: []CommitListItem{}})
			return
		}
	}

	page := getDefaultInt(c, "page", 1)
	pageSize := getDefaultInt(c, "pageSize", DefaultPageSize)

	total, err := CountStatCommits(statDB, repoAddr, repoBranch, userID, startTime, endTime, orgFilterUserIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	list, err := ListStatCommits(statDB, repoAddr, repoBranch, userID, startTime, endTime, page, pageSize, orgFilterUserIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
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
	results := make([]CommitListItem, len(list))
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

		item := CommitListItem{
			CommitID:                         commit.CommitID,
			CommitTime:                       commit.CommitTime,
			RepoAddr:                         commit.RepoAddr,
			RepoBranch:                       commit.RepoBranch,
			GitUserName:                      commit.GitUserName,
			GitUserEmail:                     commit.GitUserEmail,
			UserID:                           commit.UserID,
			UserName:                         commit.UserName,
			ClientID:                         commit.ClientID,
			WorkDir:                          commit.WorkDir,
			DiffLines:                        commit.DiffLines,
			CommitAncientMinutes:             commit.CommitAncientMinutes,
			CommitAncientMinutesReason:       commit.CommitAncientMinutesReason,
			CommitAncientMinutesManual:       commit.CommitAncientMinutesManual,
			CommitAncientMinutesReasonManual: commit.CommitAncientMinutesReasonManual,
			CommitRealMinutes:                commit.CommitRealMinutes,
			CommitRealMinutesReason:          commit.CommitRealMinutesReason,
			CommitRealMinutesManual:          commit.CommitRealMinutesManual,
			CommitRealMinutesReasonManual:    commit.CommitRealMinutesReasonManual,
			CommitRealAIMinutes:              commit.CommitRealAIMinutes,
			CommitRealAncientMinutes:         commit.CommitRealAncientMinutes,
			TaskIDs:                          commit.TaskIDs,
			TaskIDsSilica:                    commit.TaskIDsSilica,
			Comment:                          commit.Comment,
			CreatedAt:                        commit.CreatedAt,
			UpdatedAt:                        commit.UpdatedAt,
			Cost:                             totalCost,
			UpstreamTokens:                   upstreamTokens,
			DownstreamTokens:                 downstreamTokens,
			Silica:                           overallSilica,
		}

		effectiveAncient := commit.CommitAncientMinutes
		if commit.CommitAncientMinutesManual != nil {
			effectiveAncient = commit.CommitAncientMinutesManual
		}
		effectiveReal := commit.CommitRealMinutes
		if commit.CommitRealMinutesManual != nil {
			effectiveReal = commit.CommitRealMinutesManual
		}
		var efficiencyRatio *float64
		if effectiveAncient != nil && effectiveReal != nil && *effectiveReal > 0 && *effectiveAncient > 0 {
			ratio := calcEfficiencyRatio(*effectiveAncient, *effectiveReal)
			efficiencyRatio = &ratio
		}
		item.EfficiencyRatio = efficiencyRatio

		// 补充 org 字段
		if commit.UserID != nil {
			if om, ok := orgMappings[*commit.UserID]; ok {
				item.Org1 = om.Org1
				item.Org2 = om.Org2
				item.Org3 = om.Org3
				item.Org4 = om.Org4
				parts := []string{}
				for _, v := range []string{om.Org1, om.Org2, om.Org3, om.Org4} {
					if v != "" {
						parts = append(parts, v)
					}
				}
				item.OrgDisplay = strings.Join(parts, "/")
			}
		}

		results[i] = item
	}

	c.JSON(http.StatusOK, CommitListResponse{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Data:     results,
	})
}

// getCommitDetailV2 GET /api/v2/commits/:commitId
// @Summary 获取提交详情
// @Description 根据提交ID获取提交详细信息
// @Tags Commits
// @Produce json
// @Param commitId path string true "提交ID"
// @Success 200 {object} CommitDetailResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/commits/{commitId} [get]
func getCommitDetailV2(c *gin.Context) {
	commitID := c.Param("commitId")

	commit, err := GetStatCommitByID(statDB, commitID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	if commit == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "commit not found"})
		return
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
		ratio := calcEfficiencyRatio(*effectiveAncient, *effectiveReal)
		efficiencyRatio = &ratio
	}

	// 计算整体硅含量
	var overallSilica *float64
	if silicaWeightSum > 0 {
		s := math.Round(weightedSilicaSum/silicaWeightSum*1000) / 10 // 百分比，1位小数
		overallSilica = &s
	}

	c.JSON(http.StatusOK, CommitDetailResponse{
		Commit:           commit,
		RelatedTasks:     relatedTasks,
		EfficiencyRatio:  efficiencyRatio,
		TotalCost:        math.Round(totalCost*10000) / 10000,
		Silica:           overallSilica,
		UpstreamTokens:   upstreamTokens,
		DownstreamTokens: downstreamTokens,
	})
}

// updateCommitManualV2 PUT /api/v2/commits/:commitId/manual
// @Summary 更新提交人工数据
// @Description 更新提交的人工修改数据
// @Tags Commits
// @Accept json
// @Produce json
// @Param commitId path string true "提交ID"
// @Param data body UpdateCommitManualRequest true "人工数据"
// @Success 200 {object} StatusResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/commits/{commitId}/manual [put]
func updateCommitManualV2(c *gin.Context) {
	commitId := c.Param("commitId")

	var req UpdateCommitManualRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	if err := UpdateStatCommitManual(statDB, commitId, req.CommitAncientMinutesManual, req.CommitAncientMinutesReasonManual, req.CommitRealMinutesManual, req.CommitRealMinutesReasonManual); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, StatusResponse{Status: "ok"})
}
