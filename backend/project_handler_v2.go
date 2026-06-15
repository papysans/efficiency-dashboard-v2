package main

import (
	"encoding/json"
	"fmt"
	"kanban/core/models"
	"kanban/core/utils"
	"log"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type ProjectListItem struct {
	ProjectId                       string          `json:"project_id"`
	Name                            string          `json:"name"`
	Description                     string          `json:"description"`
	Repos                           json.RawMessage `json:"repos" swaggertype:"string"`
	TaskIds                         json.RawMessage `json:"task_ids" swaggertype:"string"`
	TaskIdsSilica                   json.RawMessage `json:"task_ids_silica" swaggertype:"string"`
	StartTime                       *time.Time      `json:"start_time"`
	EndTime                         *time.Time      `json:"end_time"`
	StartTimeManual                 *time.Time      `json:"start_time_manual"`
	EndTimeManual                   *time.Time      `json:"end_time_manual"`
	UpstreamTokens                  *int64          `json:"upstream_tokens"`
	DownstreamTokens                *int64          `json:"downstream_tokens"`
	Cost                            *float64        `json:"cost"`
	ProjectAncientMinutes           *float64        `json:"project_ancient_minutes"`
	ProjectAncientReason            string          `json:"project_ancient_minutes_reason"`
	ProjectAncientMinutesManual     *float64        `json:"project_ancient_minutes_manual"`
	ProjectAncientReasonManual      string          `json:"project_ancient_minutes_reason_manual"`
	ProjectRealProcessMinutes       *float64        `json:"project_real_process_minutes"`
	ProjectRealProcessReason        string          `json:"project_real_process_minutes_reason"`
	ProjectRealProcessMinutesManual *float64        `json:"project_real_process_minutes_manual"`
	ProjectRealProcessReasonManual  string          `json:"project_real_process_minutes_reason_manual"`
	ProjectRealLeadMinutes          *float64        `json:"project_real_lead_minutes"`
	ProjectRealLeadReason           string          `json:"project_real_lead_minutes_reason"`
	ProjectRealLeadMinutesManual    *float64        `json:"project_real_lead_minutes_manual"`
	ProjectRealLeadReasonManual     string          `json:"project_real_lead_minutes_reason_manual"`
	CreatedAt                       time.Time       `json:"created_at"`
	UpdatedAt                       time.Time       `json:"updated_at"`
	RepoCount                       int             `json:"repo_count"`
	TaskCount                       int             `json:"task_count"`
	UserCount                       int             `json:"user_count"`
	TotalCodeLines                  int64           `json:"total_code_lines"`
	ActualLinesPerDay               *float64        `json:"actual_lines_per_day"`
	EfficiencyRatio                 *float64        `json:"efficiency_ratio"`
}

type ProjectListResponse struct {
	Data []ProjectListItem `json:"data"`
}

type ProjectCommitItem struct {
	CommitId                   string    `json:"commit_id"`
	UserId                     string    `json:"user_id"`
	CommitTime                 time.Time `json:"commit_time"`
	RepoAddr                   string    `json:"repo_addr"`
	RepoBranch                 string    `json:"repo_branch"`
	UserName                   string    `json:"user_name"`
	GitUserName                string    `json:"git_user_name"`
	DiffLines                  int       `json:"diff_lines"`
	Comment                    string    `json:"comment"`
	CommitAncientMinutes       float64   `json:"commit_ancient_minutes"`
	CommitAncientMinutesManual *float64  `json:"commit_ancient_minutes_manual"`
	CommitRealMinutes          float64   `json:"commit_real_minutes"`
	CommitRealMinutesManual    *float64  `json:"commit_real_minutes_manual"`
	Silica                     float64   `json:"silica"`
}

type ProjectTaskItem struct {
	TaskId                   string    `json:"task_id"`
	UserId                   string    `json:"user_id"`
	UserName                 string    `json:"user_name"`
	StartTime                time.Time `json:"start_time"`
	EndTime                  time.Time `json:"end_time"`
	UpstreamTokens           int64     `json:"upstream_tokens"`
	DownstreamTokens         int64     `json:"downstream_tokens"`
	Cost                     float64   `json:"cost"`
	TaskAncientMinutes       float64   `json:"task_ancient_minutes"`
	TaskAncientMinutesManual *float64  `json:"task_ancient_minutes_manual"`
	TaskRealMinutes          float64   `json:"task_real_minutes"`
	TaskRealMinutesManual    *float64  `json:"task_real_minutes_manual"`
	Title                    string    `json:"title"`
	WorkDir                  string    `json:"work_dir"`
	DiffLines                int       `json:"diff_lines"`
	Silica                   float64   `json:"silica"`
	AcceptRatio              float64   `json:"accept_ratio"`
}

type ProjectDetailResponse struct {
	Project         *models.Project     `json:"project"`
	Commits         []ProjectCommitItem `json:"commits"`
	Tasks           []ProjectTaskItem   `json:"tasks"`
	Members         []UserDetail        `json:"members"`
	EfficiencyRatio float64             `json:"efficiency_ratio"`
	UserCount       int                 `json:"user_count"`

	// —— Need(branch) 口径项目指标 ——
	// 与上方 commit 古法口径（EfficiencyRatio 等）并列、互不替代，前端需明确区分标注。
	// 比值为"分子分母守恒"原始倍数（非百分比、无上界、无 ×100），口径=日历主/工作量下钻，
	// 只计 coverage_eligible 且按口径非 outlier 的干净 Need，并套全局看板口径（排主干分支）。
	NeedCalendarEfficiencyRatio *float64 `json:"need_calendar_efficiency_ratio"` // 日历口径提效比（主）
	NeedWorkEfficiencyRatio     *float64 `json:"need_work_efficiency_ratio"`     // 工作量口径提效比（下钻）
	NeedAICodeRatio             *float64 `json:"need_ai_code_ratio"`             // AI 代码占比（0~1）
	NeedActualCalendarMin       float64  `json:"need_actual_calendar_min"`
	NeedBaselineCalendarMin     float64  `json:"need_baseline_calendar_min"`
	NeedActualWorkMin           float64  `json:"need_actual_work_min"`
	NeedBaselineWorkMin         float64  `json:"need_baseline_work_min"`
	NeedEligibleCount           int      `json:"need_eligible_count"` // 已选且干净、计入指标的 Need 数
	NeedExcludedCount           int      `json:"need_excluded_count"` // 已选但因日历口径 outlier 被自动剔除的 Need 数
	NeedTotalCount              int      `json:"need_total_count"`    // 候选池总数（看板口径，含未选/已排除/不合格）；与 /needs 列表行数同源
	NeedTotalLocNet             int64    `json:"need_total_loc_net"`  // 已选干净 Need 的净 LOC 之和（生成代码量，AI 占比同源口径）
}

type ProjectConflict struct {
	CommitId    string `json:"commit_id"`
	ProjectId   string `json:"project_id"`
	ProjectName string `json:"project_name"`
}

type ProjectConflictsResponse struct {
	Conflicts []ProjectConflict `json:"conflicts"`
}

type CreateProjectRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

type UpdateProjectRequest struct {
	Name          string          `json:"name"`
	Description   *string         `json:"description"`
	Repos         json.RawMessage `json:"repos" swaggertype:"string"`
	TaskIds       json.RawMessage `json:"task_ids" swaggertype:"string"`
	TaskIdsSilica json.RawMessage `json:"task_ids_silica" swaggertype:"string"`
}

type AddTasksRequest struct {
	TaskIds       []string  `json:"task_ids"`
	TaskIdsSilica []float64 `json:"task_ids_silica"`
}

type RemoveTasksRequest struct {
	TaskIds []string `json:"task_ids"`
}

type UpdateTaskSilicaRequest struct {
	TaskId string  `json:"task_id"`
	Silica float64 `json:"silica"`
}

type CheckProjectConflictsRequest struct {
	CommitIds []string `json:"commit_ids"`
}

type UpdateProjectManualRequest struct {
	ProjectAncientMinutesManual     *float64   `json:"project_ancient_minutes_manual"`
	ProjectAncientReasonManual      *string    `json:"project_ancient_minutes_reason_manual"`
	ProjectRealProcessMinutesManual *float64   `json:"project_real_process_minutes_manual"`
	ProjectRealProcessReasonManual  *string    `json:"project_real_process_minutes_reason_manual"`
	ProjectRealLeadMinutesManual    *float64   `json:"project_real_lead_minutes_manual"`
	ProjectRealLeadReasonManual     *string    `json:"project_real_lead_minutes_reason_manual"`
	StartTimeManual                 *time.Time `json:"start_time_manual"`
	EndTimeManual                   *time.Time `json:"end_time_manual"`
}

// 添加到Project中的Repo条件
type ProjectRepo struct {
	RepoAddr           string   `json:"repo_addr"`
	RepoBranch         string   `json:"repo_branch"`
	StartTime          *string  `json:"start_time"`
	EndTime            *string  `json:"end_time"`
	ExcludeCommits     []string `json:"exclude_commits"`
	IncludeOnlyCommits []string `json:"include_only_commits"`
	// Need 维度白/黑名单（need_id）：仅作用于"项目按 Need(branch) 聚合"口径，
	// 与 commit 级的 ExcludeCommits/IncludeOnlyCommits 各自独立、互不影响。
	// 默认全收该 (repo,branch) 下通过看板口径的 Need；可黑掉个别噪声 Need 或只白名单几个。
	ExcludeNeeds     []string `json:"exclude_needs"`
	IncludeOnlyNeeds []string `json:"include_only_needs"`
}

// collectProjectCommits 根据 project 的 repos 配置收集去重后的 commits
func collectProjectCommits(project *models.Project) (map[string]*models.Commit, error) {
	var repos []ProjectRepo
	if len(project.Repos) > 0 && string(project.Repos) != "null" && string(project.Repos) != "[]" {
		if err := json.Unmarshal([]byte(project.Repos), &repos); err != nil {
			return nil, fmt.Errorf("解析 repos 失败: %w", err)
		}
	}

	commitMap := map[string]*models.Commit{}
	for _, rf := range repos {
		startTime := ""
		endTime := ""
		if rf.StartTime != nil {
			startTime = *rf.StartTime
		}
		if rf.EndTime != nil {
			endTime = *rf.EndTime
		}

		// 与项目列表（ListCommitLightByRepoRange 已剔除治理 commit）同口径，避免列表/详情分裂
		commits, _, err := ListCommits(statDB, CommitFilter{
			ExcludeGoverned: true,
			RepoAddr:        rf.RepoAddr,
			RepoBranch:      rf.RepoBranch,
			StartTime:       startTime,
			EndTime:         endTime,
		}, 1, 999999, "commit_time DESC")
		if err != nil {
			return nil, fmt.Errorf("查询 commits 失败: %w", err)
		}

		if len(rf.IncludeOnlyCommits) > 0 {
			whiteSet := make(map[string]bool, len(rf.IncludeOnlyCommits))
			for _, id := range rf.IncludeOnlyCommits {
				whiteSet[id] = true
			}
			for i := range commits {
				if whiteSet[commits[i].CommitId] {
					commitMap[commits[i].CommitId] = &commits[i]
				}
			}
		} else {
			blackSet := make(map[string]bool, len(rf.ExcludeCommits))
			for _, id := range rf.ExcludeCommits {
				blackSet[id] = true
			}
			for i := range commits {
				if !blackSet[commits[i].CommitId] {
					commitMap[commits[i].CommitId] = &commits[i]
				}
			}
		}
	}
	return commitMap, nil
}

func collectProjectTasks(project *models.Project, commitMap map[string]*models.Commit) ([]ProjectTaskItem, error) {
	taskSilicaMap := map[string]float64{}

	// 从 commits 查关联 tasks
	commitIDs := make([]string, 0, len(commitMap))
	for cid := range commitMap {
		commitIDs = append(commitIDs, cid)
	}
	if len(commitIDs) > 0 {
		var autoTaskIDs []string
		if err := statDB.Model(&models.Task{}).Where("commit_id IN ?", commitIDs).Pluck("task_id", &autoTaskIDs).Error; err != nil {
			log.Printf("批量查询 commit tasks 失败: %v", err)
		}
		for _, id := range autoTaskIDs {
			taskSilicaMap[id] = 1.0
		}
	}

	// 从 project_tasks 关联表获取手动添加的 tasks
	projectTasks, err := ListProjectTasks(statDB, project.ProjectId)
	if err != nil {
		log.Printf("查询 project_tasks 失败: %v", err)
	}
	for _, pt := range projectTasks {
		taskSilicaMap[pt.TaskId] = pt.Silica
	}

	allTaskIDs := make([]string, 0, len(taskSilicaMap))
	for id := range taskSilicaMap {
		allTaskIDs = append(allTaskIDs, id)
	}

	taskDetailMap, err := BatchGetTasks(statDB, allTaskIDs)
	if err != nil {
		log.Printf("批量查询 tasks 失败: %v", err)
		taskDetailMap = make(map[string]*models.Task)
	}

	var tasks []ProjectTaskItem
	for taskID, silica := range taskSilicaMap {
		task := taskDetailMap[taskID]
		if task == nil {
			continue
		}
		tasks = append(tasks, ProjectTaskItem{
			TaskId:                   task.TaskId,
			UserId:                   task.UserId,
			UserName:                 task.UserName,
			StartTime:                task.StartTime,
			EndTime:                  task.EndTime,
			UpstreamTokens:           task.UpstreamTokens,
			DownstreamTokens:         task.DownstreamTokens,
			Cost:                     task.Cost,
			TaskAncientMinutes:       task.TaskAncientMinutes,
			TaskAncientMinutesManual: task.TaskAncientMinutesManual,
			TaskRealMinutes:          task.TaskRealMinutes,
			TaskRealMinutesManual:    task.TaskRealMinutesManual,
			Title:                    task.Title,
			WorkDir:                  task.WorkDir,
			Silica:                   silica,
		})
	}

	return tasks, nil
}

func collectProjectUsers(project *models.Project, commitMap map[string]*models.Commit, tasks []ProjectTaskItem) []UserDetail {
	userSet := map[string]bool{}
	for _, cm := range commitMap {
		if cm.UserId != "" {
			userSet[cm.UserId] = true
		}
	}
	for _, task := range tasks {
		if task.UserId != "" {
			userSet[task.UserId] = true
		}
	}

	userIDs := make([]string, 0, len(userSet))
	for uid := range userSet {
		userIDs = append(userIDs, uid)
	}

	var startTime, endTime string
	if !project.StartTime.IsZero() {
		startTime = project.StartTime.Format(time.RFC3339)
	}
	if !project.EndTime.IsZero() {
		endTime = project.EndTime.Format(time.RFC3339)
	}

	return GetProductivityByIds(statDB, userIDs, startTime, endTime)
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
	commitIDs := make([]string, 0, len(commitMap))
	for cid := range commitMap {
		commitIDs = append(commitIDs, cid)
	}
	tasksets := NewTaskIdSet()
	if len(commitIDs) > 0 {
		var autoTaskIDs []string
		if err := statDB.Model(&models.Task{}).Where("commit_id IN ?", commitIDs).Pluck("task_id", &autoTaskIDs).Error; err != nil {
			log.Printf("批量查询 commit tasks 失败: %v", err)
		}
		for _, tid := range autoTaskIDs {
			tasksets.tasks[tid] = true
		}
	}
	// 从 project_tasks 关联表追加手动 task
	projectTasks, err := ListProjectTasks(statDB, project.ProjectId)
	if err != nil {
		log.Printf("查询 project_tasks 失败: %v", err)
	}
	for _, pt := range projectTasks {
		tasksets.tasks[pt.TaskId] = true
	}

	var upstreamTokens, downstreamTokens int64
	var cost float64
	var ancientMinutes, realProcessMinutes float64
	var minTime, maxTime *time.Time

	// 批量聚合 tasks
	taskIDs := tasksets.GetTaskIds()

	if len(taskIDs) > 0 {
		// taskIDs := make([]string, 0, len(taskIDSet))
		// for tid := range taskIDSet {
		// 	taskIDs = append(taskIDs, tid)
		// }
		var taskAgg struct {
			UpstreamTokens   int64
			DownstreamTokens int64
			Cost             float64
			AncientMinutes   float64
			RealMinutes      float64
			MinTime          *time.Time
			MaxTime          *time.Time
		}
		if err := statDB.Model(&models.Task{}).
			Select(`COALESCE(SUM(upstream_tokens), 0) as upstream_tokens,
				COALESCE(SUM(downstream_tokens), 0) as downstream_tokens,
				COALESCE(SUM(cost), 0) as cost,
				COALESCE(SUM(COALESCE(task_ancient_minutes_manual, task_ancient_minutes)), 0) as ancient_minutes,
				COALESCE(SUM(COALESCE(task_real_minutes_manual, task_real_minutes)), 0) as real_minutes,
				MIN(start_time) as min_time,
				MAX(end_time) as max_time`).
			Where("task_id IN ?", taskIDs).
			Scan(&taskAgg).Error; err != nil {
			return fmt.Errorf("批量聚合 tasks 失败: %w", err)
		}
		upstreamTokens = taskAgg.UpstreamTokens
		downstreamTokens = taskAgg.DownstreamTokens
		cost = taskAgg.Cost
		ancientMinutes = taskAgg.AncientMinutes
		realProcessMinutes = taskAgg.RealMinutes
		minTime = taskAgg.MinTime
		maxTime = taskAgg.MaxTime
	}

	// 遍历 commits
	commitIDsForDerive := make([]string, 0, len(commitMap))
	for _, commit := range commitMap {
		if commit.CommitAncientMinutes <= 0 || commit.CommitRealMinutes <= 0 {
			commitIDsForDerive = append(commitIDsForDerive, commit.CommitId)
		}
	}
	derivedAncient, derivedReal, err := deriveCommitWorkMinutesBatch(statDB, commitIDsForDerive)
	if err != nil {
		log.Printf("批量派生项目 commit 工时失败: %v", err)
	}
	for _, commit := range commitMap {
		if commit.CommitAncientMinutesManual != nil {
			ancientMinutes += *commit.CommitAncientMinutesManual
		} else if commit.CommitAncientMinutes > 0 {
			ancientMinutes += commit.CommitAncientMinutes
		} else {
			ancientMinutes += derivedAncient[commit.CommitId]
		}
		if commit.CommitRealMinutesManual != nil {
			realProcessMinutes += *commit.CommitRealMinutesManual
		} else if commit.CommitRealMinutes > 0 {
			realProcessMinutes += commit.CommitRealMinutes
		} else {
			realProcessMinutes += derivedReal[commit.CommitId]
		}
		if !commit.CommitTime.IsZero() {
			if minTime == nil || commit.CommitTime.Before(*minTime) {
				t := commit.CommitTime
				minTime = &t
			}
			if maxTime == nil || commit.CommitTime.After(*maxTime) {
				t := commit.CommitTime
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
	ancientReason := fmt.Sprintf("tasks:%d commits:%d", len(taskIDs), len(commitMap))
	realReason := fmt.Sprintf("tasks:%d commits:%d", len(taskIDs), len(commitMap))
	leadReason := ""
	if minTime != nil && maxTime != nil {
		leadReason = fmt.Sprintf("%s ~ %s", minTime.Format("2006-01-02"), maxTime.Format("2006-01-02"))
	}

	agg := &ProjectAggregates{
		StartTime:                 minTime,
		EndTime:                   maxTime,
		UpstreamTokens:            upstreamTokens,
		DownstreamTokens:          downstreamTokens,
		Cost:                      cost,
		ProjectAncientMinutes:     &ancientMinutes,
		ProjectAncientReason:      ancientReason,
		ProjectRealProcessMinutes: &realProcessMinutes,
		ProjectRealProcessReason:  realReason,
		ProjectRealLeadMinutes:    leadMinutes,
		ProjectRealLeadReason:     leadReason,
	}
	return UpdateProjectAggregates(statDB, projectID, agg)
}

// createProjectV2 POST /api/v2/projects
// @Summary 创建项目
// @Description 创建新项目
// @Tags Projects
// @Accept json
// @Produce json
// @Param project body CreateProjectRequest true "项目信息"
// @Success 200 {object} Project
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/projects [post]
func createProjectV2(c *gin.Context) {
	var req CreateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "name 不能为空"})
		return
	}

	desc := ""
	if req.Description != nil {
		desc = *req.Description
	}
	p := &models.Project{
		Name:        req.Name,
		Description: desc,
	}
	projectID, err := CreateProject(statDB, p)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	project, err := GetProject(statDB, projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, project)
}

// listProjectsV2 GET /api/v2/projects
// @Summary 获取项目列表
// @Description 获取所有项目列表
// @Tags Projects
// @Produce json
// @Success 200 {object} ProjectListResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/projects [get]
func listProjectsV2(c *gin.Context) {
	orderField, orderDir := parseOrderParam(strings.TrimSpace(c.Query("order")))
	if orderField != "" && !isAllowedField(orderField, projectSortFields) {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "不支持的排序字段: " + orderField})
		return
	}

	list, err := ListProjects(statDB)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	results := make([]ProjectListItem, len(list))
	for i, p := range list {
		// repo_count
		repoCount := 0
		if len(p.Repos) > 0 && string(p.Repos) != "null" && string(p.Repos) != "[]" {
			var repos []json.RawMessage
			if err := json.Unmarshal([]byte(p.Repos), &repos); err == nil {
				repoCount = len(repos)
			}
		}
		// task_count 和手动 task IDs 从 project_tasks 关联表获取
		projectTasks, _ := ListProjectTasks(statDB, p.ProjectId)
		taskCount := len(projectTasks)
		taskIDs := make([]string, len(projectTasks))
		for i, pt := range projectTasks {
			taskIDs[i] = pt.TaskId
		}

		// user_count & total_code_lines: 轻量查询 commits
		userSet := map[string]bool{}
		var totalCodeLines int64
		var repos []ProjectRepo
		if len(p.Repos) > 0 && string(p.Repos) != "null" && string(p.Repos) != "[]" {
			_ = json.Unmarshal([]byte(p.Repos), &repos)
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
				if lc.UserName != "" {
					userSet[lc.UserName] = true
				}
				if lc.DiffLines > 0 {
					totalCodeLines += int64(lc.DiffLines)
				}
				if lc.AncientMinutes > 0 {
					p.ProjectAncientMinutes += lc.AncientMinutes
				}
				if lc.RealMinutes > 0 {
					p.ProjectRealProcessMinutes += lc.RealMinutes
				}
			}
		}
		// 也从 tasks 中收集用户
		if len(taskIDs) > 0 {
			taskDetailMap, err := BatchGetTasks(statDB, taskIDs)
			if err == nil {
				for _, task := range taskDetailMap {
					if task.UserName != "" {
						userSet[task.UserName] = true
					}
				}
			}
		}
		userCount := len(userSet)

		// actual_lines_per_day
		effectiveReal := p.ProjectRealProcessMinutes
		if p.ProjectRealProcessMinutesManual != nil {
			effectiveReal = *p.ProjectRealProcessMinutesManual
		}
		var actualLinesPerDay *float64
		if effectiveReal > 0 && totalCodeLines > 0 {
			days := effectiveReal / 480.0
			v := math.Round(float64(totalCodeLines) / days)
			actualLinesPerDay = &v
		}

		// efficiency_ratio
		effectiveAncient := p.ProjectAncientMinutes
		if p.ProjectAncientMinutesManual != nil {
			effectiveAncient = *p.ProjectAncientMinutesManual
		}
		var effRatio *float64
		if effectiveAncient > 0 && effectiveReal > 0 {
			ratio := utils.CalcEfficiencyRatio(effectiveAncient, effectiveReal)
			effRatio = &ratio
		}

		results[i] = ProjectListItem{
			ProjectId:                       p.ProjectId,
			Name:                            p.Name,
			Description:                     p.Description,
			Repos:                           json.RawMessage(p.Repos),
			TaskIds:                         json.RawMessage([]byte("[]")),
			TaskIdsSilica:                   json.RawMessage([]byte("[]")),
			StartTime:                       &p.StartTime,
			EndTime:                         &p.EndTime,
			StartTimeManual:                 p.StartTimeManual,
			EndTimeManual:                   p.EndTimeManual,
			UpstreamTokens:                  &p.UpstreamTokens,
			DownstreamTokens:                &p.DownstreamTokens,
			Cost:                            &p.Cost,
			ProjectAncientMinutes:           &p.ProjectAncientMinutes,
			ProjectAncientReason:            p.ProjectAncientReason,
			ProjectAncientMinutesManual:     p.ProjectAncientMinutesManual,
			ProjectAncientReasonManual:      p.ProjectAncientReasonManual,
			ProjectRealProcessMinutes:       &p.ProjectRealProcessMinutes,
			ProjectRealProcessReason:        p.ProjectRealProcessReason,
			ProjectRealProcessMinutesManual: p.ProjectRealProcessMinutesManual,
			ProjectRealProcessReasonManual:  p.ProjectRealProcessReasonManual,
			ProjectRealLeadMinutes:          &p.ProjectRealLeadMinutes,
			ProjectRealLeadReason:           p.ProjectRealLeadReason,
			ProjectRealLeadMinutesManual:    p.ProjectRealLeadMinutesManual,
			ProjectRealLeadReasonManual:     p.ProjectRealLeadReasonManual,
			CreatedAt:                       p.CreatedAt,
			UpdatedAt:                       p.UpdatedAt,
			RepoCount:                       repoCount,
			TaskCount:                       taskCount,
			UserCount:                       userCount,
			TotalCodeLines:                  totalCodeLines,
			ActualLinesPerDay:               actualLinesPerDay,
			EfficiencyRatio:                 effRatio,
		}
	}
	sortProjectData(results, orderField, orderDir)

	c.JSON(http.StatusOK, ProjectListResponse{Data: results})
}

// getProjectDetailV2 GET /api/v2/projects/:projectId
// @Summary 获取项目详情
// @Description 根据项目ID获取项目详细信息
// @Tags Projects
// @Produce json
// @Param projectId path string true "项目ID"
// @Success 200 {object} ProjectDetailResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/projects/{projectId} [get]
func getProjectDetailV2(c *gin.Context) {
	projectID := c.Param("projectId")

	project, err := GetProject(statDB, projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	if project == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "project not found"})
		return
	}

	// 收集 commits
	commitMap, err := collectProjectCommits(project)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	// 收集 tasks
	tasks, err := collectProjectTasks(project, commitMap)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].EndTime.IsZero() && tasks[j].EndTime.IsZero() {
			return false
		}
		if tasks[i].EndTime.IsZero() {
			return false
		}
		if tasks[j].EndTime.IsZero() {
			return true
		}
		return tasks[i].EndTime.After(tasks[j].EndTime)
	})

	// 构建 commits 列表
	commitItems := make([]ProjectCommitItem, 0, len(commitMap))
	for _, cm := range commitMap {
		commitItems = append(commitItems, ProjectCommitItem{
			CommitId:                   cm.CommitId,
			UserId:                     cm.UserId,
			CommitTime:                 cm.CommitTime,
			RepoAddr:                   cm.RepoAddr,
			RepoBranch:                 cm.RepoBranch,
			UserName:                   cm.UserName,
			GitUserName:                cm.GitUserName,
			DiffLines:                  cm.DiffLines,
			Comment:                    cm.Comment,
			CommitAncientMinutes:       cm.CommitAncientMinutes,
			CommitAncientMinutesManual: cm.CommitAncientMinutesManual,
			CommitRealMinutes:          cm.CommitRealMinutes,
			CommitRealMinutesManual:    cm.CommitRealMinutesManual,
			Silica:                     cm.Silica,
		})
	}
	sort.Slice(commitItems, func(i, j int) bool {
		return commitItems[i].CommitTime.After(commitItems[j].CommitTime)
	})

	effRatio := utils.CalcEfficiencyRatioManual(project.ProjectAncientMinutes,
		project.ProjectRealProcessMinutes,
		project.ProjectAncientMinutesManual,
		project.ProjectRealProcessMinutesManual)

	members := collectProjectUsers(project, commitMap, tasks)

	resp := ProjectDetailResponse{
		Project:         project,
		Commits:         commitItems,
		Tasks:           tasks,
		Members:         members,
		EfficiencyRatio: effRatio,
		UserCount:       len(members),
	}

	// Need(branch) 口径项目指标：解析 project.Repos 拿 (repo,branch) 候选池 →
	// needs 表分子分母守恒聚合（只计干净 Need、套看板口径）。失败仅记日志、不影响古法字段返回。
	if scopes, scopeErr := collectProjectRepoBranches(project); scopeErr != nil {
		log.Printf("解析 project %s repo 范围失败: %v", projectID, scopeErr)
	} else if agg, aggErr := queryProjectNeedAgg(statDB, scopes); aggErr != nil {
		log.Printf("查询 project %s Need 口径聚合失败: %v", projectID, aggErr)
	} else {
		// 复用全站共享 helper（actual<=0→nil；baseline=0,actual>0→-100%），口径与 dashboard/部门/组织详情一致。
		resp.NeedCalendarEfficiencyRatio = efficiencyV2Ratio(agg.BaselineCalendarMin, agg.ActualCalendarMin)
		resp.NeedWorkEfficiencyRatio = efficiencyV2Ratio(agg.BaselineWorkMin, agg.ActualWorkMin)
		resp.NeedAICodeRatio = calcNeedAICodeRatio(agg.AICoveredLoc, agg.TotalLocNet)
		resp.NeedActualCalendarMin = agg.ActualCalendarMin
		resp.NeedBaselineCalendarMin = agg.BaselineCalendarMin
		resp.NeedActualWorkMin = agg.ActualWorkMin
		resp.NeedBaselineWorkMin = agg.BaselineWorkMin
		resp.NeedEligibleCount = agg.EligibleNeeds
		resp.NeedExcludedCount = agg.ExcludedNeeds
		resp.NeedTotalCount = agg.TotalNeeds
		resp.NeedTotalLocNet = agg.TotalLocNet
	}

	c.JSON(http.StatusOK, resp)
}

// updateProjectV2 PUT /api/v2/projects/:projectId
// @Summary 更新项目
// @Description 更新项目信息
// @Tags Projects
// @Accept json
// @Produce json
// @Param projectId path string true "项目ID"
// @Param project body UpdateProjectRequest true "项目信息"
// @Success 200 {object} StatusResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/projects/{projectId} [put]
func updateProjectV2(c *gin.Context) {
	projectID := c.Param("projectId")

	var req UpdateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	project, err := GetProject(statDB, projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	if project == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "project not found"})
		return
	}

	updates := map[string]interface{}{
		"name":        req.Name,
		"description": project.Description,
		"repos":       models.StringJSON(req.Repos),
		"updated_at":  time.Now(),
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	result := statDB.Model(&models.Project{}).Where("project_id = ?", projectID).Updates(updates)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: result.Error.Error()})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "project not found"})
		return
	}

	// 解析请求中的 task_ids 和 silica，替换 project_tasks
	var taskIDs []string
	var silicas []float64
	if len(req.TaskIds) > 0 && string(req.TaskIds) != "null" && string(req.TaskIds) != "[]" {
		json.Unmarshal(req.TaskIds, &taskIDs)
	}
	if len(req.TaskIdsSilica) > 0 && string(req.TaskIdsSilica) != "null" && string(req.TaskIdsSilica) != "[]" {
		json.Unmarshal(req.TaskIdsSilica, &silicas)
	}
	if err := ReplaceProjectTasks(statDB, projectID, taskIDs, silicas); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	if err := recalculateProjectAggregates(projectID); err != nil {
		log.Printf("重算 project %s 聚合数据失败: %v", projectID, err)
	}

	c.JSON(http.StatusOK, StatusResponse{Status: "ok"})
}

// deleteProjectV2 DELETE /api/v2/projects/:projectId
// @Summary 删除项目
// @Description 根据项目ID删除项目
// @Tags Projects
// @Produce json
// @Param projectId path string true "项目ID"
// @Success 200 {object} StatusResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/projects/{projectId} [delete]
func deleteProjectV2(c *gin.Context) {
	projectID := c.Param("projectId")

	if err := DeleteProject(statDB, projectID); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, StatusResponse{Status: "ok"})
}

// updateProjectManualV2 PUT /api/v2/projects/:projectId/manual
// @Summary 更新项目人工数据
// @Description 更新项目的人工修改数据
// @Tags Projects
// @Accept json
// @Produce json
// @Param projectId path string true "项目ID"
// @Param data body UpdateProjectManualRequest true "人工数据"
// @Success 200 {object} StatusResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/projects/{projectId}/manual [put]
func updateProjectManualV2(c *gin.Context) {
	projectID := c.Param("projectId")

	var fields UpdateProjectManualRequest
	if err := c.ShouldBindJSON(&fields); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	if err := UpdateProjectManual(statDB, projectID, fields); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, StatusResponse{Status: "ok"})
}

// addTasksToProjectV2 POST /api/v2/projects/:projectId/tasks
// @Summary 为项目添加任务
// @Description 向项目添加多个任务
// @Tags Projects
// @Accept json
// @Produce json
// @Param projectId path string true "项目ID"
// @Param tasks body AddTasksRequest true "任务ID列表"
// @Success 200 {object} StatusResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/projects/{projectId}/tasks [post]
func addTasksToProjectV2(c *gin.Context) {
	projectID := c.Param("projectId")

	var req AddTasksRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	project, err := GetProject(statDB, projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	if project == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "project not found"})
		return
	}

	// 去重追加到 project_tasks
	if err := AddProjectTasks(statDB, projectID, req.TaskIds, req.TaskIdsSilica); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	if err := recalculateProjectAggregates(projectID); err != nil {
		log.Printf("重算 project %s 聚合数据失败: %v", projectID, err)
	}

	c.JSON(http.StatusOK, StatusResponse{Status: "ok"})
}

// addRepoToProjectV2 POST /api/v2/projects/:projectId/repos
// @Summary 为项目添加仓库
// @Description 向项目添加仓库
// @Tags Projects
// @Accept json
// @Produce json
// @Param projectId path string true "项目ID"
// @Param repo body ProjectRepo true "仓库信息"
// @Success 200 {object} StatusResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/projects/{projectId}/repos [post]
func addRepoToProjectV2(c *gin.Context) {
	projectID := c.Param("projectId")

	var req ProjectRepo
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	project, err := GetProject(statDB, projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	if project == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "project not found"})
		return
	}

	var repos []ProjectRepo
	if len(project.Repos) > 0 && string(project.Repos) != "null" && string(project.Repos) != "[]" {
		json.Unmarshal([]byte(project.Repos), &repos)
	}
	repos = append(repos, req)

	reposJSON, _ := json.Marshal(repos)
	project.Repos = models.StringJSON(reposJSON)

	if err := UpdateProject(statDB, project); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	if err := recalculateProjectAggregates(projectID); err != nil {
		log.Printf("重算 project %s 聚合数据失败: %v", projectID, err)
	}

	c.JSON(http.StatusOK, StatusResponse{Status: "ok"})
}

// removeRepoFromProjectV2 DELETE /api/v2/projects/:projectId/repos/:index
// @Summary 从项目移除仓库
// @Description 从项目中移除指定索引的仓库
// @Tags Projects
// @Produce json
// @Param projectId path string true "项目ID"
// @Param index path int true "仓库索引"
// @Success 200 {object} StatusResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v2/projects/{projectId}/repos/{index} [delete]
func removeRepoFromProjectV2(c *gin.Context) {
	projectID := c.Param("projectId")
	indexStr := c.Param("index")
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "index 必须为整数"})
		return
	}

	project, err := GetProject(statDB, projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	if project == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "project not found"})
		return
	}

	var repos []ProjectRepo
	if len(project.Repos) > 0 && string(project.Repos) != "null" && string(project.Repos) != "[]" {
		json.Unmarshal([]byte(project.Repos), &repos)
	}

	if index < 0 || index >= len(repos) {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: fmt.Sprintf("index %d 超出范围 [0, %d)", index, len(repos))})
		return
	}

	repos = append(repos[:index], repos[index+1:]...)
	reposJSON, _ := json.Marshal(repos)
	project.Repos = models.StringJSON(reposJSON)

	if err := UpdateProject(statDB, project); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	if err := recalculateProjectAggregates(projectID); err != nil {
		log.Printf("重算 project %s 聚合数据失败: %v", projectID, err)
	}

	c.JSON(http.StatusOK, StatusResponse{Status: "ok"})
}

// checkProjectConflictsV2 POST /api/v2/projects/check-conflicts
// @Summary 检查项目冲突
// @Description 检查项目中任务或仓库的冲突情况
// @Tags Projects
// @Accept json
// @Produce json
// @Param data body CheckProjectConflictsRequest true "检查数据"
// @Success 200 {object} ProjectConflictsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/projects/check-conflicts [post]
func checkProjectConflictsV2(c *gin.Context) {
	var req CheckProjectConflictsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	projects, err := ListProjects(statDB)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	checkSet := make(map[string]bool, len(req.CommitIds))
	for _, id := range req.CommitIds {
		checkSet[id] = true
	}

	var conflicts []ProjectConflict

	for _, p := range projects {
		commitMap, err := collectProjectCommits(&p)
		if err != nil {
			continue
		}
		for commitID := range commitMap {
			if checkSet[commitID] {
				conflicts = append(conflicts, ProjectConflict{
					CommitId:    commitID,
					ProjectId:   p.ProjectId,
					ProjectName: p.Name,
				})
			}
		}
	}

	c.JSON(http.StatusOK, ProjectConflictsResponse{Conflicts: conflicts})
}

// removeTasksFromProjectV2 DELETE /api/v2/projects/:projectId/tasks
// @Summary 从项目移除任务
// @Description 从项目中移除所有任务
// @Tags Projects
// @Produce json
// @Param projectId path string true "项目ID"
// @Param tasks body RemoveTasksRequest true "任务ID列表"
// @Success 200 {object} StatusResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/projects/{projectId}/tasks [delete]
func removeTasksFromProjectV2(c *gin.Context) {
	projectID := c.Param("projectId")

	var req RemoveTasksRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}
	if len(req.TaskIds) == 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "task_ids 不能为空"})
		return
	}

	project, err := GetProject(statDB, projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	if project == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "project not found"})
		return
	}

	if err := RemoveProjectTasks(statDB, projectID, req.TaskIds); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	if err := recalculateProjectAggregates(projectID); err != nil {
		log.Printf("重算 project %s 聚合数据失败: %v", projectID, err)
	}

	c.JSON(http.StatusOK, StatusResponse{Status: "ok"})
}

// updateTaskSilicaInProjectV2 PUT /api/v2/projects/:projectId/tasks/silica
// @Summary 更新项目任务二氧化硅数据
// @Description 更新项目中任务的二氧化硅数据
// @Tags Projects
// @Accept json
// @Produce json
// @Param projectId path string true "项目ID"
// @Param data body UpdateTaskSilicaRequest true "二氧化硅数据"
// @Success 200 {object} StatusResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/projects/{projectId}/tasks/silica [put]
func updateTaskSilicaInProjectV2(c *gin.Context) {
	projectID := c.Param("projectId")

	var req UpdateTaskSilicaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}
	if req.TaskId == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "task_id 不能为空"})
		return
	}

	project, err := GetProject(statDB, projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	if project == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "project not found"})
		return
	}

	if err := UpdateProjectTaskSilica(statDB, projectID, req.TaskId, req.Silica); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	if err := recalculateProjectAggregates(projectID); err != nil {
		log.Printf("重算 project %s 聚合数据失败: %v", projectID, err)
	}

	c.JSON(http.StatusOK, StatusResponse{Status: "ok"})
}
