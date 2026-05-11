package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"kanban/core/utils"

	"github.com/gin-gonic/gin"
)

type TaskListItem struct {
	TaskID                         string     `json:"task_id"`
	Title                          string     `json:"title"`
	UserID                         string     `json:"user_id"`
	UserName                       string     `json:"user_name"`
	ClientID                       string     `json:"client_id"`
	ClientIDE                      string     `json:"client_ide"`
	ClientVersion                  string     `json:"client_version"`
	ClientOS                       string     `json:"client_os"`
	ClientOSVersion                string     `json:"client_os_version"`
	Caller                         string     `json:"caller"`
	RepoAddr                       string     `json:"repo_addr"`
	RepoBranch                     string     `json:"repo_branch"`
	WorkDir                        string     `json:"work_dir"`
	WorkDirID                      string     `json:"work_dir_id"`
	StartTime                      *time.Time `json:"start_time"`
	EndTime                        *time.Time `json:"end_time"`
	UpstreamTokens                 int64      `json:"upstream_tokens"`
	DownstreamTokens               int64      `json:"downstream_tokens"`
	Cost                           float64    `json:"cost"`
	DiffLines                      int        `json:"diff_lines"`
	TaskAncientMinutes             *float64   `json:"task_ancient_minutes"`
	TaskAncientMinutesReason       string     `json:"task_ancient_minutes_reason"`
	TaskAncientMinutesManual       *float64   `json:"task_ancient_minutes_manual"`
	TaskAncientMinutesReasonManual string     `json:"task_ancient_minutes_reason_manual"`
	TaskRealMinutes                *float64   `json:"task_real_minutes"`
	TaskRealMinutesReason          string     `json:"task_real_minutes_reason"`
	TaskRealMinutesManual          *float64   `json:"task_real_minutes_manual"`
	TaskRealMinutesReasonManual    string     `json:"task_real_minutes_reason_manual"`
	CreatedAt                      time.Time  `json:"created_at"`
	UpdatedAt                      time.Time  `json:"updated_at"`
	EfficiencyRatio                *float64   `json:"efficiency_ratio"`
	Org1                           string     `json:"org1"`
	Org2                           string     `json:"org2"`
	Org3                           string     `json:"org3"`
	Org4                           string     `json:"org4"`
	Org5                           string     `json:"org5"`
	Org6                           string     `json:"org6"`
	Org7                           string     `json:"org7"`
	Org8                           string     `json:"org8"`
	Org9                           string     `json:"org9"`
	OrgDisplay                     string     `json:"org_display"`
}

type TaskListResponse struct {
	Total    int            `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"pageSize"`
	Data     []TaskListItem `json:"data"`
}

type TaskDetailResponse struct {
	Task            *StatTask              `json:"task"`
	Conversations   []StatTaskConversation `json:"conversations"`
	EfficiencyRatio *float64               `json:"efficiency_ratio"`
}

type UpdateTaskManualRequest struct {
	TaskRealMinutesManual          *float64 `json:"task_real_minutes_manual"`
	TaskRealMinutesReasonManual    *string  `json:"task_real_minutes_reason_manual"`
	TaskAncientMinutesManual       *float64 `json:"task_ancient_minutes_manual"`
	TaskAncientMinutesReasonManual *string  `json:"task_ancient_minutes_reason_manual"`
}

// listTasksV2 GET /api/v2/tasks
// @Summary 获取任务列表
// @Description 按日期范围查询任务列表，支持分页
// @Tags Tasks
// @Produce json
// @Param startDate query string true "开始日期(YYYYMMDD)"
// @Param endDate query string true "结束日期(YYYYMMDD)"
// @Param userId query string false "用户ID"
// @Param userName query string false "用户名"
// @Param clientId query string false "客户端ID"
// @Param clientIde query string false "IDE类型"
// @Param clientOs query string false "操作系统"
// @Param caller query string false "调用方"
// @Param repoAddr query string false "仓库地址"
// @Param repoBranch query string false "分支"
// @Param workDirId query string false "工作目录ID"
// @Param org1 query string false "一级组织"
// @Param org2 query string false "二级组织"
// @Param org3 query string false "三级组织"
// @Param org4 query string false "四级组织"
// @Param org5 query string false "五级组织"
// @Param org6 query string false "六级组织"
// @Param org7 query string false "七级组织"
// @Param org8 query string false "八级组织"
// @Param org9 query string false "九级组织"
// @Param page query int false "页码" default(1)
// @Param pageSize query int false "每页数量" default(20)
// @Success 200 {object} TaskListResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/tasks [get]
func listTasksV2(c *gin.Context) {
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	if startDate == "" || endDate == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "startDate 和 endDate 为必填参数"})
		return
	}

	startT, err := parseDateParam(startDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "startDate 格式错误: " + err.Error()})
		return
	}
	endT, err := parseDateParam(endDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "endDate 格式错误: " + err.Error()})
		return
	}

	page := getDefaultInt(c, "page", 1)
	pageSize := getDefaultInt(c, "pageSize", DefaultPageSize)

	filter := TaskFilter{
		UserID:     strings.TrimSpace(c.Query("userId")),
		UserName:   strings.TrimSpace(c.Query("userName")),
		ClientID:   strings.TrimSpace(c.Query("clientId")),
		ClientIDE:  strings.TrimSpace(c.Query("clientIde")),
		ClientOS:   strings.TrimSpace(c.Query("clientOs")),
		Caller:     strings.TrimSpace(c.Query("caller")),
		RepoAddr:   strings.TrimSpace(c.Query("repoAddr")),
		RepoBranch: strings.TrimSpace(c.Query("repoBranch")),
		WorkDirID:  strings.TrimSpace(c.Query("workDirId")),
		StartTime:  startT.Format(time.RFC3339),
		EndTime:    endT.Add(23*time.Hour + 59*time.Minute + 59*time.Second).Format(time.RFC3339),
		Org1:       strings.TrimSpace(c.Query("org1")),
		Org2:       strings.TrimSpace(c.Query("org2")),
		Org3:       strings.TrimSpace(c.Query("org3")),
		Org4:       strings.TrimSpace(c.Query("org4")),
		Org5:       strings.TrimSpace(c.Query("org5")),
		Org6:       strings.TrimSpace(c.Query("org6")),
		Org7:       strings.TrimSpace(c.Query("org7")),
		Org8:       strings.TrimSpace(c.Query("org8")),
		Org9:       strings.TrimSpace(c.Query("org9")),
	}

	total, err := CountStatTasks(statDB, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	list, err := ListStatTasks(statDB, filter, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	results := make([]TaskListItem, len(list))
	for i, t := range list {
		item := TaskListItem{
			TaskID:                         t.TaskID,
			Title:                          t.Title,
			UserID:                         t.UserID,
			UserName:                       t.UserName,
			ClientID:                       t.ClientID,
			ClientIDE:                      t.ClientIDE,
			ClientVersion:                  t.ClientVersion,
			ClientOS:                       t.ClientOS,
			ClientOSVersion:                t.ClientOSVersion,
			Caller:                         t.Caller,
			RepoAddr:                       t.RepoAddr,
			RepoBranch:                     t.RepoBranch,
			WorkDir:                        t.WorkDir,
			WorkDirID:                      t.WorkDirID,
			StartTime:                      t.StartTime,
			EndTime:                        t.EndTime,
			UpstreamTokens:                 t.UpstreamTokens,
			DownstreamTokens:               t.DownstreamTokens,
			Cost:                           t.Cost,
			DiffLines:                      t.DiffLines,
			TaskAncientMinutes:             t.TaskAncientMinutes,
			TaskAncientMinutesReason:       t.TaskAncientMinutesReason,
			TaskAncientMinutesManual:       t.TaskAncientMinutesManual,
			TaskAncientMinutesReasonManual: t.TaskAncientMinutesReasonManual,
			TaskRealMinutes:                t.TaskRealMinutes,
			TaskRealMinutesReason:          t.TaskRealMinutesReason,
			TaskRealMinutesManual:          t.TaskRealMinutesManual,
			TaskRealMinutesReasonManual:    t.TaskRealMinutesReasonManual,
			CreatedAt:                      t.CreatedAt,
			UpdatedAt:                      t.UpdatedAt,
		}

		effectiveAncient := t.TaskAncientMinutes
		if t.TaskAncientMinutesManual != nil {
			effectiveAncient = t.TaskAncientMinutesManual
		}
		effectiveReal := t.TaskRealMinutes
		if t.TaskRealMinutesManual != nil {
			effectiveReal = t.TaskRealMinutesManual
		}
		if effectiveAncient != nil && effectiveReal != nil && *effectiveReal > 0 && *effectiveAncient > 0 {
			ratio := utils.CalcEfficiencyRatio(*effectiveAncient, *effectiveReal)
			item.EfficiencyRatio = &ratio
		}

		if t.UserID != "" {
			if om, ok := orgMappings[t.UserID]; ok {
				item.Org1 = om.Org1
				item.Org2 = om.Org2
				item.Org3 = om.Org3
				item.Org4 = om.Org4
				item.Org5 = om.Org5
				item.Org6 = om.Org6
				item.Org7 = om.Org7
				item.Org8 = om.Org8
				item.Org9 = om.Org9
				item.OrgDisplay = getOrgDisplay(om.Org1, om.Org2, om.Org3, om.Org4,
					om.Org5, om.Org6, om.Org7, om.Org8, om.Org9)
			}
		}
		results[i] = item
	}

	c.JSON(http.StatusOK, TaskListResponse{Total: total, Page: page, PageSize: pageSize, Data: results})
}

// getTaskDetailV2 GET /api/v2/tasks/:taskId
// @Summary 获取任务详情
// @Description 根据任务ID获取任务详细信息及关联对话
// @Tags Tasks
// @Produce json
// @Param taskId path string true "任务ID"
// @Success 200 {object} TaskDetailResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/tasks/{taskId} [get]
func getTaskDetailV2(c *gin.Context) {
	taskId := c.Param("taskId")

	task, err := GetStatTask(statDB, taskId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	if task == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "task not found"})
		return
	}

	convs, err := ListStatTaskConversations(statDB, taskId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	efficiencyRatio := utils.CalcEfficiencyRatioManual(task.TaskAncientMinutes,
		task.TaskAncientMinutesManual, task.TaskRealMinutes, task.TaskRealMinutesManual)

	c.JSON(http.StatusOK, TaskDetailResponse{Task: task, Conversations: convs, EfficiencyRatio: efficiencyRatio})
}

// updateTaskManualV2 PUT /api/v2/tasks/:taskId/manual
// @Summary 更新任务人工数据
// @Description 更新任务的人工修改数据
// @Tags Tasks
// @Accept json
// @Produce json
// @Param taskId path string true "任务ID"
// @Param data body UpdateTaskManualRequest true "人工数据"
// @Success 200 {object} StatusResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/tasks/{taskId}/manual [put]
func updateTaskManualV2(c *gin.Context) {
	taskId := c.Param("taskId")
	if taskId == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "taskId 不能为空"})
		return
	}

	var req UpdateTaskManualRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	if err := UpdateStatTaskManual(statDB, taskId, req.TaskRealMinutesManual, req.TaskRealMinutesReasonManual, req.TaskAncientMinutesManual, req.TaskAncientMinutesReasonManual); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, StatusResponse{Status: "ok"})
}

// getTaskFile GET /api/v2/tasks/file
// @Summary 获取任务文件内容
// @Description 根据任务ID和日期获取任务的summary或conversation文件内容
// @Tags Tasks
// @Produce json
// @Param type query string true "文件类型(summary/conversation)"
// @Param taskId query string true "任务ID"
// @Param date query string true "日期(YYYYMMDD)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/tasks/file [get]
func getTaskFile(c *gin.Context) {
	typ := c.Query("type")
	taskId := c.Query("taskId")
	date := c.Query("date")

	if typ != "summary" && typ != "conversation" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "type must be summary or conversation"})
		return
	}
	if taskId == "" || date == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "taskId and date are required"})
		return
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "date must be YYYY-MM-DD format"})
		return
	}
	if strings.Contains(taskId, "..") || strings.Contains(taskId, "/") || strings.Contains(taskId, "\\") ||
		strings.Contains(date, "..") || strings.Contains(date, "/") || strings.Contains(date, "\\") {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid characters in parameters"})
		return
	}

	parts := strings.Split(date, "-")
	yyyy, mm, dd := parts[0], parts[1], parts[2]

	var filePath string
	var contentType string

	if typ == "summary" {
		filePath = filepath.Join(appConfig.AnalysedDir, "analysed", yyyy, mm, dd, taskId+".json")
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			filePath = filepath.Join(appConfig.TaskDir, "summary", yyyy, mm, dd, taskId+".json")
		}
		contentType = "application/json"
	} else {
		filePath = filepath.Join(appConfig.TaskDir, "conversation", yyyy, mm, dd, taskId+".jsonl")
		contentType = "text/plain; charset=utf-8"
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "文件不存在"})
		return
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.Data(http.StatusOK, contentType, data)
}
