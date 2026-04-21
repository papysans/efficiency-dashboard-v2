package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// RepoFilter repos JSONB 中每个元素的结构
type RepoFilter struct {
	RepoAddr           string   `json:"repo_addr"`
	RepoBranch         string   `json:"repo_branch"`
	StartTime          *string  `json:"start_time"`
	EndTime            *string  `json:"end_time"`
	ExcludeCommits     []string `json:"exclude_commits"`
	IncludeOnlyCommits []string `json:"include_only_commits"`
}

// collectProjectCommits 根据 project 的 repos 配置收集去重后的 commits
func collectProjectCommits(project *Project) (map[string]*StatCommit, error) {
	var repos []RepoFilter
	if len(project.Repos) > 0 && string(project.Repos) != "null" && string(project.Repos) != "[]" {
		if err := json.Unmarshal(project.Repos, &repos); err != nil {
			return nil, fmt.Errorf("解析 repos 失败: %w", err)
		}
	}

	commitMap := map[string]*StatCommit{}
	for _, rf := range repos {
		startTime := ""
		endTime := ""
		if rf.StartTime != nil {
			startTime = *rf.StartTime
		}
		if rf.EndTime != nil {
			endTime = *rf.EndTime
		}

		commits, err := ListStatCommits(statDB, rf.RepoAddr, rf.RepoBranch, "", startTime, endTime, 1, 999999, nil)
		if err != nil {
			return nil, fmt.Errorf("查询 commits 失败: %w", err)
		}

		if len(rf.IncludeOnlyCommits) > 0 {
			whiteSet := make(map[string]bool, len(rf.IncludeOnlyCommits))
			for _, id := range rf.IncludeOnlyCommits {
				whiteSet[id] = true
			}
			for i := range commits {
				if whiteSet[commits[i].CommitID] {
					commitMap[commits[i].CommitID] = &commits[i]
				}
			}
		} else {
			blackSet := make(map[string]bool, len(rf.ExcludeCommits))
			for _, id := range rf.ExcludeCommits {
				blackSet[id] = true
			}
			for i := range commits {
				if !blackSet[commits[i].CommitID] {
					commitMap[commits[i].CommitID] = &commits[i]
				}
			}
		}
	}
	return commitMap, nil
}

// recalculateProjectAggregates 重算虚拟项目的聚合数据
func recalculateProjectAggregates(projectID string) error {
	project, err := GetProject(statDB, projectID)
	if err != nil {
		return fmt.Errorf("获取 project 失败: %w", err)
	}
	if project == nil {
		return fmt.Errorf("project %s 不存在", projectID)
	}

	commitMap, err := collectProjectCommits(project)
	if err != nil {
		return err
	}

	// 从 commits 提取 task_ids
	taskIDSet := map[string]bool{}
	for _, commit := range commitMap {
		if len(commit.TaskIDs) > 0 && string(commit.TaskIDs) != "null" && string(commit.TaskIDs) != "[]" {
			var ids []string
			if err := json.Unmarshal(commit.TaskIDs, &ids); err == nil {
				for _, id := range ids {
					taskIDSet[id] = true
				}
			}
		}
	}

	// 从 project.TaskIDs 追加
	if len(project.TaskIDs) > 0 && string(project.TaskIDs) != "null" && string(project.TaskIDs) != "[]" {
		var ids []string
		if err := json.Unmarshal(project.TaskIDs, &ids); err == nil {
			for _, id := range ids {
				taskIDSet[id] = true
			}
		}
	}

	var upstreamTokens, downstreamTokens int64
	var cost float64
	var ancientMinutes, realProcessMinutes float64
	var minTime, maxTime *time.Time

	// 遍历 tasks
	for taskID := range taskIDSet {
		task, err := GetStatTask(statDB, taskID)
		if err != nil {
			log.Printf("查询 task %s 失败: %v", taskID, err)
			continue
		}
		if task == nil {
			continue
		}
		if task.UpstreamTokens != nil {
			upstreamTokens += *task.UpstreamTokens
		}
		if task.DownstreamTokens != nil {
			downstreamTokens += *task.DownstreamTokens
		}
		if task.Cost != nil {
			cost += *task.Cost
		}
		if task.TaskAncientMinutesManual != nil {
			ancientMinutes += *task.TaskAncientMinutesManual
		} else if task.TaskAncientMinutes != nil {
			ancientMinutes += *task.TaskAncientMinutes
		}
		if task.TaskRealMinutesManual != nil {
			realProcessMinutes += *task.TaskRealMinutesManual
		} else if task.TaskRealMinutes != nil {
			realProcessMinutes += *task.TaskRealMinutes
		}
		if task.StartTime != nil {
			if minTime == nil || task.StartTime.Before(*minTime) {
				t := *task.StartTime
				minTime = &t
			}
		}
		if task.EndTime != nil {
			if maxTime == nil || task.EndTime.After(*maxTime) {
				t := *task.EndTime
				maxTime = &t
			}
		}
	}

	// 遍历 commits
	for _, commit := range commitMap {
		if commit.CommitAncientMinutesManual != nil {
			ancientMinutes += *commit.CommitAncientMinutesManual
		} else if commit.CommitAncientMinutes != nil {
			ancientMinutes += *commit.CommitAncientMinutes
		}
		if commit.CommitRealMinutesManual != nil {
			realProcessMinutes += *commit.CommitRealMinutesManual
		} else if commit.CommitRealMinutes != nil {
			realProcessMinutes += *commit.CommitRealMinutes
		}
		if commit.CommitTime != nil {
			if minTime == nil || commit.CommitTime.Before(*minTime) {
				t := *commit.CommitTime
				minTime = &t
			}
			if maxTime == nil || commit.CommitTime.After(*maxTime) {
				t := *commit.CommitTime
				maxTime = &t
			}
		}
	}

	// lead minutes
	var leadMinutes *float64
	if minTime != nil && maxTime != nil {
		m := maxTime.Sub(*minTime).Minutes()
		leadMinutes = &m
	}

	// reason
	ancientReason := fmt.Sprintf("tasks:%d commits:%d", len(taskIDSet), len(commitMap))
	realReason := fmt.Sprintf("tasks:%d commits:%d", len(taskIDSet), len(commitMap))
	leadReason := ""
	if minTime != nil && maxTime != nil {
		leadReason = fmt.Sprintf("%s ~ %s", minTime.Format("2006-01-02"), maxTime.Format("2006-01-02"))
	}

	agg := &ProjectAggregates{
		StartTime:                       minTime,
		EndTime:                         maxTime,
		UpstreamTokens:                  upstreamTokens,
		DownstreamTokens:                downstreamTokens,
		Cost:                            cost,
		ProjectAncientMinutes:           &ancientMinutes,
		ProjectAncientMinutesReason:     ancientReason,
		ProjectRealProcessMinutes:       &realProcessMinutes,
		ProjectRealProcessMinutesReason: realReason,
		ProjectRealLeadMinutes:          leadMinutes,
		ProjectRealLeadMinutesReason:    leadReason,
	}
	return UpdateProjectAggregates(statDB, projectID, agg)
}

// createProjectV2 POST /api/v2/projects
// @Summary 创建项目
// @Description 创建新项目
// @Tags Projects
// @Accept json
// @Produce json
// @Param project body object true "项目信息"
// @Success 200 {object} object
// @Failure 400 {object} object
// @Router /api/v2/projects [post]
func createProjectV2(c *gin.Context) {
	var req struct {
		Name        string  `json:"name"`
		Description *string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name 不能为空"})
		return
	}

	p := &Project{
		Name:        req.Name,
		Description: req.Description,
	}
	projectID, err := CreateProject(statDB, p)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	project, err := GetProject(statDB, projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, project)
}

// listProjectsV2 GET /api/v2/projects
// @Summary 获取项目列表
// @Description 获取所有项目列表
// @Tags Projects
// @Produce json
// @Success 200 {object} object
// @Router /api/v2/projects [get]
func listProjectsV2(c *gin.Context) {
	list, err := ListProjects(statDB)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	results := make([]gin.H, len(list))
	for i, p := range list {
		// repo_count
		repoCount := 0
		if len(p.Repos) > 0 && string(p.Repos) != "null" && string(p.Repos) != "[]" {
			var repos []json.RawMessage
			if err := json.Unmarshal(p.Repos, &repos); err == nil {
				repoCount = len(repos)
			}
		}
		// task_count
		taskCount := 0
		var taskIDs []string
		if len(p.TaskIDs) > 0 && string(p.TaskIDs) != "null" && string(p.TaskIDs) != "[]" {
			if err := json.Unmarshal(p.TaskIDs, &taskIDs); err == nil {
				taskCount = len(taskIDs)
			}
		}

		// user_count & total_code_lines: 轻量查询 commits
		userSet := map[string]bool{}
		var totalCodeLines int64
		var repos []RepoFilter
		if len(p.Repos) > 0 && string(p.Repos) != "null" && string(p.Repos) != "[]" {
			_ = json.Unmarshal(p.Repos, &repos)
		}
		for _, rf := range repos {
			startT := ""
			endT := ""
			if rf.StartTime != nil {
				startT = *rf.StartTime
			}
			if rf.EndTime != nil {
				endT = *rf.EndTime
			}
			lights, err := ListCommitLightByRepoRange(statDB, rf.RepoAddr, rf.RepoBranch, startT, endT)
			if err != nil {
				log.Printf("listProjectsV2: light query for repo %s failed: %v", rf.RepoAddr, err)
				continue
			}
			for _, lc := range lights {
				if lc.UserName != nil && *lc.UserName != "" {
					userSet[*lc.UserName] = true
				}
				if lc.DiffLines != nil {
					totalCodeLines += int64(*lc.DiffLines)
				}
			}
		}
		// 也从 tasks 中收集用户
		if len(taskIDs) > 0 {
			taskDetailMap, err := BatchGetStatTasks(statDB, taskIDs)
			if err == nil {
				for _, task := range taskDetailMap {
					if task.UserName != nil && *task.UserName != "" {
						userSet[*task.UserName] = true
					}
				}
			}
		}
		userCount := len(userSet)

		// actual_lines_per_day
		effectiveReal := p.ProjectRealProcessMinutes
		if p.ProjectRealProcessMinutesManual != nil {
			effectiveReal = p.ProjectRealProcessMinutesManual
		}
		var actualLinesPerDay interface{}
		if effectiveReal != nil && *effectiveReal > 0 && totalCodeLines > 0 {
			days := *effectiveReal / 480.0
			actualLinesPerDay = math.Round(float64(totalCodeLines) / days)
		}

		// efficiency_ratio
		effectiveAncient := p.ProjectAncientMinutes
		if p.ProjectAncientMinutesManual != nil {
			effectiveAncient = p.ProjectAncientMinutesManual
		}
		var effRatio interface{}
		if effectiveAncient != nil && effectiveReal != nil && *effectiveReal > 0 && *effectiveAncient > 0 {
			ratio := (*effectiveAncient / *effectiveReal) * 100
			effRatio = math.Round(ratio*10) / 10
		}

		results[i] = gin.H{
			"project_id":                            p.ProjectID,
			"name":                                  p.Name,
			"description":                           p.Description,
			"repos":                                 p.Repos,
			"task_ids":                              p.TaskIDs,
			"task_ids_silica":                       p.TaskIDsSilica,
			"start_time":                            p.StartTime,
			"end_time":                              p.EndTime,
			"start_time_manual":                     p.StartTimeManual,
			"end_time_manual":                       p.EndTimeManual,
			"upstream_tokens":                       p.UpstreamTokens,
			"downstream_tokens":                     p.DownstreamTokens,
			"cost":                                  p.Cost,
			"project_ancient_minutes":               p.ProjectAncientMinutes,
			"project_ancient_minutes_reason":        p.ProjectAncientMinutesReason,
			"project_ancient_minutes_manual":        p.ProjectAncientMinutesManual,
			"project_ancient_minutes_reason_manual": p.ProjectAncientMinutesReasonManual,
			"project_real_process_minutes":          p.ProjectRealProcessMinutes,
			"project_real_process_minutes_reason":   p.ProjectRealProcessMinutesReason,
			"project_real_process_minutes_manual":   p.ProjectRealProcessMinutesManual,
			"project_real_process_minutes_reason_manual": p.ProjectRealProcessMinutesReasonManual,
			"project_real_lead_minutes":                  p.ProjectRealLeadMinutes,
			"project_real_lead_minutes_reason":           p.ProjectRealLeadMinutesReason,
			"project_real_lead_minutes_manual":           p.ProjectRealLeadMinutesManual,
			"project_real_lead_minutes_reason_manual":    p.ProjectRealLeadMinutesReasonManual,
			"created_at":           p.CreatedAt,
			"updated_at":           p.UpdatedAt,
			"repo_count":           repoCount,
			"task_count":           taskCount,
			"user_count":           userCount,
			"total_code_lines":     totalCodeLines,
			"actual_lines_per_day": actualLinesPerDay,
			"efficiency_ratio":     effRatio,
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": results})
}

// getProjectDetailV2 GET /api/v2/projects/:projectId
// @Summary 获取项目详情
// @Description 根据项目ID获取项目详细信息
// @Tags Projects
// @Produce json
// @Param projectId path string true "项目ID"
// @Success 200 {object} object
// @Failure 404 {object} object
// @Router /api/v2/projects/{projectId} [get]
func getProjectDetailV2(c *gin.Context) {
	projectID := c.Param("projectId")

	project, err := GetProject(statDB, projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if project == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}

	// 收集 commits
	commitMap, err := collectProjectCommits(project)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 收集 tasks（含 silica 权重映射）
	// 先解析直接配置的 task_ids 和对应的 silica
	taskSilicaMap := map[string]float64{} // task_id -> silica
	if len(project.TaskIDs) > 0 && string(project.TaskIDs) != "null" && string(project.TaskIDs) != "[]" {
		var ids []string
		var silicas []float64
		if err := json.Unmarshal(project.TaskIDs, &ids); err == nil {
			if len(project.TaskIDsSilica) > 0 && string(project.TaskIDsSilica) != "null" {
				_ = json.Unmarshal(project.TaskIDsSilica, &silicas)
			}
			for i, id := range ids {
				s := 1.0
				if i < len(silicas) {
					s = silicas[i]
				}
				taskSilicaMap[id] = s
			}
		}
	}
	// 再收集 commits 关联的 task_ids（silica 默认 1.0，不覆盖已有配置）
	for _, commit := range commitMap {
		if len(commit.TaskIDs) > 0 && string(commit.TaskIDs) != "null" && string(commit.TaskIDs) != "[]" {
			var ids []string
			if err := json.Unmarshal(commit.TaskIDs, &ids); err == nil {
				for _, id := range ids {
					if _, exists := taskSilicaMap[id]; !exists {
						taskSilicaMap[id] = 1.0
					}
				}
			}
		}
	}

	// 批量查询所有关联 tasks（用于 commits 的 silica 计算）
	allTaskIDs := make([]string, 0, len(taskSilicaMap))
	for id := range taskSilicaMap {
		allTaskIDs = append(allTaskIDs, id)
	}
	taskDetailMap, err := BatchGetStatTasks(statDB, allTaskIDs)
	if err != nil {
		log.Printf("批量查询 tasks 失败: %v", err)
		taskDetailMap = make(map[string]*StatTask)
	}

	// 构建 commits 列表，计算每条 commit 的加权硅含量
	commitItems := make([]gin.H, 0, len(commitMap))
	for _, cm := range commitMap {
		var taskIDs []string
		if len(cm.TaskIDs) > 0 && string(cm.TaskIDs) != "null" && string(cm.TaskIDs) != "[]" {
			_ = json.Unmarshal(cm.TaskIDs, &taskIDs)
		}
		var silicaList []float64
		if len(cm.TaskIDsSilica) > 0 && string(cm.TaskIDsSilica) != "null" && string(cm.TaskIDsSilica) != "[]" {
			_ = json.Unmarshal(cm.TaskIDsSilica, &silicaList)
		}
		var weightedSilicaSum, silicaWeightSum float64
		for j, taskID := range taskIDs {
			task := taskDetailMap[taskID]
			if task != nil && task.DiffLines != nil && *task.DiffLines > 0 {
				silica := 0.0
				if j < len(silicaList) {
					silica = silicaList[j]
				}
				weightedSilicaSum += float64(*task.DiffLines) * silica
				silicaWeightSum += float64(*task.DiffLines)
			}
		}
		var overallSilica *float64
		if silicaWeightSum > 0 {
			s := math.Round(weightedSilicaSum/silicaWeightSum*1000) / 10
			overallSilica = &s
		}
		commitItems = append(commitItems, gin.H{
			"commit_id":                     cm.CommitID,
			"commit_time":                   cm.CommitTime,
			"repo_addr":                     cm.RepoAddr,
			"repo_branch":                   cm.RepoBranch,
			"user_name":                     cm.UserName,
			"git_user_name":                 cm.GitUserName,
			"diff_lines":                    cm.DiffLines,
			"comment":                       cm.Comment,
			"commit_ancient_minutes":        cm.CommitAncientMinutes,
			"commit_ancient_minutes_manual": cm.CommitAncientMinutesManual,
			"commit_real_minutes":           cm.CommitRealMinutes,
			"commit_real_minutes_manual":    cm.CommitRealMinutesManual,
			"silica":                        overallSilica,
		})
	}

	var tasks []gin.H
	for taskID, silica := range taskSilicaMap {
		task := taskDetailMap[taskID]
		if task == nil {
			continue
		}
		tasks = append(tasks, gin.H{
			"task_id":                     task.TaskID,
			"user_name":                   task.UserName,
			"start_time":                  task.StartTime,
			"end_time":                    task.EndTime,
			"upstream_tokens":             task.UpstreamTokens,
			"downstream_tokens":           task.DownstreamTokens,
			"cost":                        task.Cost,
			"task_ancient_minutes":        task.TaskAncientMinutes,
			"task_ancient_minutes_manual": task.TaskAncientMinutesManual,
			"task_real_minutes":           task.TaskRealMinutes,
			"task_real_minutes_manual":    task.TaskRealMinutesManual,
			"title":                       task.Title,
			"work_dir":                    task.WorkDir,
			"silica":                      silica,
		})
	}

	// efficiency_ratio
	effectiveAncient := project.ProjectAncientMinutes
	if project.ProjectAncientMinutesManual != nil {
		effectiveAncient = project.ProjectAncientMinutesManual
	}
	effectiveReal := project.ProjectRealProcessMinutes
	if project.ProjectRealProcessMinutesManual != nil {
		effectiveReal = project.ProjectRealProcessMinutesManual
	}
	var effRatio interface{}
	if effectiveAncient != nil && effectiveReal != nil && *effectiveReal > 0 && *effectiveAncient > 0 {
		ratio := (*effectiveAncient / *effectiveReal) * 100
		effRatio = math.Round(ratio*10) / 10
	}

	// user_count: 统计参与的用户数
	userSet := map[string]bool{}
	for _, cm := range commitMap {
		if cm.UserName != nil && *cm.UserName != "" {
			userSet[*cm.UserName] = true
		} else if cm.GitUserName != nil && *cm.GitUserName != "" {
			userSet[*cm.GitUserName] = true
		}
	}
	for _, task := range taskDetailMap {
		if task.UserName != nil && *task.UserName != "" {
			userSet[*task.UserName] = true
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"project":          project,
		"commits":          commitItems,
		"tasks":            tasks,
		"efficiency_ratio": effRatio,
		"user_count":       len(userSet),
	})
}

// updateProjectV2 PUT /api/v2/projects/:projectId
// @Summary 更新项目
// @Description 更新项目信息
// @Tags Projects
// @Accept json
// @Produce json
// @Param projectId path string true "项目ID"
// @Param project body object true "项目信息"
// @Success 200 {object} object
// @Failure 400 {object} object
// @Router /api/v2/projects/{projectId} [put]
func updateProjectV2(c *gin.Context) {
	projectID := c.Param("projectId")

	var req struct {
		Name          string          `json:"name"`
		Description   *string         `json:"description"`
		Repos         json.RawMessage `json:"repos"`
		TaskIDs       json.RawMessage `json:"task_ids"`
		TaskIDsSilica json.RawMessage `json:"task_ids_silica"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	project, err := GetProject(statDB, projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if project == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}

	project.Name = req.Name
	project.Description = req.Description
	project.Repos = req.Repos
	project.TaskIDs = req.TaskIDs
	project.TaskIDsSilica = req.TaskIDsSilica

	if err := UpdateProject(statDB, project); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := recalculateProjectAggregates(projectID); err != nil {
		log.Printf("重算 project %s 聚合数据失败: %v", projectID, err)
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// deleteProjectV2 DELETE /api/v2/projects/:projectId
// @Summary 删除项目
// @Description 根据项目ID删除项目
// @Tags Projects
// @Produce json
// @Param projectId path string true "项目ID"
// @Success 200 {object} object
// @Failure 404 {object} object
// @Router /api/v2/projects/{projectId} [delete]
func deleteProjectV2(c *gin.Context) {
	projectID := c.Param("projectId")

	if err := DeleteProject(statDB, projectID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// updateProjectManualV2 PUT /api/v2/projects/:projectId/manual
// @Summary 更新项目人工数据
// @Description 更新项目的人工修改数据
// @Tags Projects
// @Accept json
// @Produce json
// @Param projectId path string true "项目ID"
// @Param data body object true "人工数据"
// @Success 200 {object} object
// @Failure 400 {object} object
// @Router /api/v2/projects/{projectId}/manual [put]
func updateProjectManualV2(c *gin.Context) {
	projectID := c.Param("projectId")

	var fields map[string]interface{}
	if err := c.ShouldBindJSON(&fields); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := UpdateProjectManual(statDB, projectID, fields); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// addTasksToProjectV2 POST /api/v2/projects/:projectId/tasks
// @Summary 为项目添加任务
// @Description 向项目添加多个任务
// @Tags Projects
// @Accept json
// @Produce json
// @Param projectId path string true "项目ID"
// @Param tasks body []string true "任务ID列表"
// @Success 200 {object} object
// @Failure 400 {object} object
// @Router /api/v2/projects/{projectId}/tasks [post]
func addTasksToProjectV2(c *gin.Context) {
	projectID := c.Param("projectId")

	var req struct {
		TaskIDs       []string  `json:"task_ids"`
		TaskIDsSilica []float64 `json:"task_ids_silica"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	project, err := GetProject(statDB, projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if project == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}

	// 解析现有 task_ids
	var existingIDs []string
	if len(project.TaskIDs) > 0 && string(project.TaskIDs) != "null" && string(project.TaskIDs) != "[]" {
		json.Unmarshal(project.TaskIDs, &existingIDs)
	}
	// 解析现有 task_ids_silica
	var existingSilica []float64
	if len(project.TaskIDsSilica) > 0 && string(project.TaskIDsSilica) != "null" && string(project.TaskIDsSilica) != "[]" {
		json.Unmarshal(project.TaskIDsSilica, &existingSilica)
	}

	// 去重追加
	existSet := make(map[string]bool, len(existingIDs))
	for _, id := range existingIDs {
		existSet[id] = true
	}
	for i, id := range req.TaskIDs {
		if !existSet[id] {
			existingIDs = append(existingIDs, id)
			silica := 0.0
			if i < len(req.TaskIDsSilica) {
				silica = req.TaskIDsSilica[i]
			}
			existingSilica = append(existingSilica, silica)
			existSet[id] = true
		}
	}

	idsJSON, _ := json.Marshal(existingIDs)
	silicaJSON, _ := json.Marshal(existingSilica)
	project.TaskIDs = idsJSON
	project.TaskIDsSilica = silicaJSON

	if err := UpdateProject(statDB, project); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := recalculateProjectAggregates(projectID); err != nil {
		log.Printf("重算 project %s 聚合数据失败: %v", projectID, err)
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// addRepoToProjectV2 POST /api/v2/projects/:projectId/repos
// @Summary 为项目添加仓库
// @Description 向项目添加仓库
// @Tags Projects
// @Accept json
// @Produce json
// @Param projectId path string true "项目ID"
// @Param repo body object true "仓库信息"
// @Success 200 {object} object
// @Failure 400 {object} object
// @Router /api/v2/projects/{projectId}/repos [post]
func addRepoToProjectV2(c *gin.Context) {
	projectID := c.Param("projectId")

	var req RepoFilter
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	project, err := GetProject(statDB, projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if project == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}

	var repos []RepoFilter
	if len(project.Repos) > 0 && string(project.Repos) != "null" && string(project.Repos) != "[]" {
		json.Unmarshal(project.Repos, &repos)
	}
	repos = append(repos, req)

	reposJSON, _ := json.Marshal(repos)
	project.Repos = reposJSON

	if err := UpdateProject(statDB, project); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := recalculateProjectAggregates(projectID); err != nil {
		log.Printf("重算 project %s 聚合数据失败: %v", projectID, err)
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// removeRepoFromProjectV2 DELETE /api/v2/projects/:projectId/repos/:index
// @Summary 从项目移除仓库
// @Description 从项目中移除指定索引的仓库
// @Tags Projects
// @Produce json
// @Param projectId path string true "项目ID"
// @Param index path int true "仓库索引"
// @Success 200 {object} object
// @Failure 404 {object} object
// @Router /api/v2/projects/{projectId}/repos/{index} [delete]
func removeRepoFromProjectV2(c *gin.Context) {
	projectID := c.Param("projectId")
	indexStr := c.Param("index")
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "index 必须为整数"})
		return
	}

	project, err := GetProject(statDB, projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if project == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}

	var repos []RepoFilter
	if len(project.Repos) > 0 && string(project.Repos) != "null" && string(project.Repos) != "[]" {
		json.Unmarshal(project.Repos, &repos)
	}

	if index < 0 || index >= len(repos) {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("index %d 超出范围 [0, %d)", index, len(repos))})
		return
	}

	repos = append(repos[:index], repos[index+1:]...)
	reposJSON, _ := json.Marshal(repos)
	project.Repos = reposJSON

	if err := UpdateProject(statDB, project); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := recalculateProjectAggregates(projectID); err != nil {
		log.Printf("重算 project %s 聚合数据失败: %v", projectID, err)
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// checkProjectConflictsV2 POST /api/v2/projects/check-conflicts
// @Summary 检查项目冲突
// @Description 检查项目中任务或仓库的冲突情况
// @Tags Projects
// @Accept json
// @Produce json
// @Param data body object true "检查数据"
// @Success 200 {object} object
// @Router /api/v2/projects/check-conflicts [post]
func checkProjectConflictsV2(c *gin.Context) {
	var req struct {
		CommitIDs []string `json:"commit_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	projects, err := ListProjects(statDB)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	checkSet := make(map[string]bool, len(req.CommitIDs))
	for _, id := range req.CommitIDs {
		checkSet[id] = true
	}

	type conflict struct {
		CommitID    string `json:"commit_id"`
		ProjectID   string `json:"project_id"`
		ProjectName string `json:"project_name"`
	}
	var conflicts []conflict

	for _, p := range projects {
		commitMap, err := collectProjectCommits(&p)
		if err != nil {
			continue
		}
		for commitID := range commitMap {
			if checkSet[commitID] {
				conflicts = append(conflicts, conflict{
					CommitID:    commitID,
					ProjectID:   p.ProjectID,
					ProjectName: p.Name,
				})
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"conflicts": conflicts})
}

// removeTasksFromProjectV2 DELETE /api/v2/projects/:projectId/tasks
// @Summary 从项目移除任务
// @Description 从项目中移除所有任务
// @Tags Projects
// @Produce json
// @Param projectId path string true "项目ID"
// @Success 200 {object} object
// @Failure 404 {object} object
// @Router /api/v2/projects/{projectId}/tasks [delete]
func removeTasksFromProjectV2(c *gin.Context) {
	projectID := c.Param("projectId")

	var req struct {
		TaskIDs []string `json:"task_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.TaskIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "task_ids 不能为空"})
		return
	}

	project, err := GetProject(statDB, projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if project == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}

	// 解析现有 task_ids
	var existingIDs []string
	if len(project.TaskIDs) > 0 && string(project.TaskIDs) != "null" && string(project.TaskIDs) != "[]" {
		json.Unmarshal(project.TaskIDs, &existingIDs)
	}
	// 解析现有 task_ids_silica
	var existingSilica []float64
	if len(project.TaskIDsSilica) > 0 && string(project.TaskIDsSilica) != "null" && string(project.TaskIDsSilica) != "[]" {
		json.Unmarshal(project.TaskIDsSilica, &existingSilica)
	}

	// 构建 remove set
	removeSet := make(map[string]bool, len(req.TaskIDs))
	for _, id := range req.TaskIDs {
		removeSet[id] = true
	}

	// 过滤掉要删除的 task
	var newIDs []string
	var newSilica []float64
	for i, id := range existingIDs {
		if !removeSet[id] {
			newIDs = append(newIDs, id)
			if i < len(existingSilica) {
				newSilica = append(newSilica, existingSilica[i])
			} else {
				newSilica = append(newSilica, 0)
			}
		}
	}

	idsJSON, _ := json.Marshal(newIDs)
	silicaJSON, _ := json.Marshal(newSilica)
	project.TaskIDs = idsJSON
	project.TaskIDsSilica = silicaJSON

	if err := UpdateProject(statDB, project); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := recalculateProjectAggregates(projectID); err != nil {
		log.Printf("重算 project %s 聚合数据失败: %v", projectID, err)
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// updateTaskSilicaInProjectV2 PUT /api/v2/projects/:projectId/tasks/silica
// @Summary 更新项目任务二氧化硅数据
// @Description 更新项目中任务的二氧化硅数据
// @Tags Projects
// @Accept json
// @Produce json
// @Param projectId path string true "项目ID"
// @Param data body object true "二氧化硅数据"
// @Success 200 {object} object
// @Failure 400 {object} object
// @Router /api/v2/projects/{projectId}/tasks/silica [put]
func updateTaskSilicaInProjectV2(c *gin.Context) {
	projectID := c.Param("projectId")

	var req struct {
		TaskID string  `json:"task_id"`
		Silica float64 `json:"silica"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.TaskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "task_id 不能为空"})
		return
	}

	project, err := GetProject(statDB, projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if project == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}

	// 解析现有 task_ids
	var existingIDs []string
	if len(project.TaskIDs) > 0 && string(project.TaskIDs) != "null" && string(project.TaskIDs) != "[]" {
		json.Unmarshal(project.TaskIDs, &existingIDs)
	}
	// 解析现有 task_ids_silica
	var existingSilica []float64
	if len(project.TaskIDsSilica) > 0 && string(project.TaskIDsSilica) != "null" && string(project.TaskIDsSilica) != "[]" {
		json.Unmarshal(project.TaskIDsSilica, &existingSilica)
	}

	// 找到 task_id 的索引并更新 silica
	found := false
	for i, id := range existingIDs {
		if id == req.TaskID {
			if i < len(existingSilica) {
				existingSilica[i] = req.Silica
			} else {
				// 补齐 silica 数组
				for j := len(existingSilica); j < i; j++ {
					existingSilica = append(existingSilica, 0)
				}
				existingSilica = append(existingSilica, req.Silica)
			}
			found = true
			break
		}
	}
	if !found {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("task_id %s 不在此 project 中", req.TaskID)})
		return
	}

	silicaJSON, _ := json.Marshal(existingSilica)
	project.TaskIDsSilica = silicaJSON

	if err := UpdateProject(statDB, project); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := recalculateProjectAggregates(projectID); err != nil {
		log.Printf("重算 project %s 聚合数据失败: %v", projectID, err)
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
