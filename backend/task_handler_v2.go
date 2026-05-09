package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"kanban/core/utils"

	"github.com/gin-gonic/gin"
)

type TaskListItem struct {
	TaskID                         string     `json:"task_id"`
	Title                          *string    `json:"title"`
	UserID                         *string    `json:"user_id"`
	UserName                       *string    `json:"user_name"`
	ClientID                       *string    `json:"client_id"`
	ClientIDE                      *string    `json:"client_ide"`
	ClientVersion                  *string    `json:"client_version"`
	ClientOS                       *string    `json:"client_os"`
	ClientOSVersion                *string    `json:"client_os_version"`
	Caller                         *string    `json:"caller"`
	RepoAddr                       *string    `json:"repo_addr"`
	RepoBranch                     *string    `json:"repo_branch"`
	WorkDir                        *string    `json:"work_dir"`
	WorkDirID                      *string    `json:"work_dir_id"`
	StartTime                      *time.Time `json:"start_time"`
	EndTime                        *time.Time `json:"end_time"`
	UpstreamTokens                 *int64     `json:"upstream_tokens"`
	DownstreamTokens               *int64     `json:"downstream_tokens"`
	Cost                           *float64   `json:"cost"`
	DiffLines                      *int       `json:"diff_lines"`
	TaskAncientMinutes             *float64   `json:"task_ancient_minutes"`
	TaskAncientMinutesReason       *string    `json:"task_ancient_minutes_reason"`
	TaskAncientMinutesManual       *float64   `json:"task_ancient_minutes_manual"`
	TaskAncientMinutesReasonManual *string    `json:"task_ancient_minutes_reason_manual"`
	TaskRealMinutes                *float64   `json:"task_real_minutes"`
	TaskRealMinutesReason          *string    `json:"task_real_minutes_reason"`
	TaskRealMinutesManual          *float64   `json:"task_real_minutes_manual"`
	TaskRealMinutesReasonManual    *string    `json:"task_real_minutes_reason_manual"`
	CreatedAt                      *time.Time `json:"created_at"`
	UpdatedAt                      *time.Time `json:"updated_at"`
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
	TimeSegments    []TimeSegment          `json:"time_segments"`
	EfficiencyRatio *float64               `json:"efficiency_ratio"`
}

type EstimateAncientResult struct {
	TaskID  string  `json:"task_id"`
	Minutes float64 `json:"minutes"`
	Reason  string  `json:"reason"`
	Error   string  `json:"error,omitempty"`
}

type EstimateAncientResponse struct {
	Status  string                  `json:"status"`
	Total   int                     `json:"total"`
	Success int                     `json:"success"`
	Results []EstimateAncientResult `json:"results"`
}

type UpdateTaskManualRequest struct {
	TaskRealMinutesManual          *float64 `json:"task_real_minutes_manual"`
	TaskRealMinutesReasonManual    *string  `json:"task_real_minutes_reason_manual"`
	TaskAncientMinutesManual       *float64 `json:"task_ancient_minutes_manual"`
	TaskAncientMinutesReasonManual *string  `json:"task_ancient_minutes_reason_manual"`
}

// TimeSegment 表示一个连续的工作时间片段
type TimeSegment struct {
	Start     time.Time `json:"start"`
	End       time.Time `json:"end"`
	ConvCount int       `json:"conv_count"`
}

// calculateTaskRealMinutes 根据对话时间戳计算实际工作时长
func calculateTaskRealMinutes(conversations []StatTaskConversation, gapThreshold int, extensionMin int) (float64, string, []TimeSegment) {
	var validTimes []time.Time
	for _, c := range conversations {
		if c.StartTime != nil {
			validTimes = append(validTimes, *c.StartTime)
		}
	}

	if len(validTimes) == 0 {
		return 0, "无有效对话", nil
	}

	ext := time.Duration(extensionMin) * time.Minute

	if len(validTimes) == 1 {
		seg := TimeSegment{
			Start:     validTimes[0],
			End:       validTimes[0].Add(ext),
			ConvCount: 1,
		}
		return float64(extensionMin), fmt.Sprintf("仅1条对话，默认%d分钟", extensionMin), []TimeSegment{seg}
	}

	sort.Slice(validTimes, func(i, j int) bool {
		return validTimes[i].Before(validTimes[j])
	})

	gapDur := time.Duration(gapThreshold) * time.Minute

	segments := []TimeSegment{{Start: validTimes[0], End: validTimes[0], ConvCount: 1}}

	for i := 1; i < len(validTimes); i++ {
		cur := &segments[len(segments)-1]
		gap := validTimes[i].Sub(cur.End)
		if gap <= gapDur {
			cur.End = validTimes[i]
			cur.ConvCount++
		} else {
			cur.End = cur.End.Add(ext)
			segments = append(segments, TimeSegment{Start: validTimes[i], End: validTimes[i], ConvCount: 1})
		}
	}
	segments[len(segments)-1].End = segments[len(segments)-1].End.Add(ext)

	var totalMinutes float64
	var parts []string
	for _, seg := range segments {
		mins := seg.End.Sub(seg.Start).Minutes()
		totalMinutes += mins
		parts = append(parts, fmt.Sprintf("%s~%s(%d条对话)",
			seg.Start.Format("2006-01-02 15:04"),
			seg.End.Format("2006-01-02 15:04"),
			seg.ConvCount))
	}

	reason := fmt.Sprintf("%d个时间片段: [%s]", len(segments), joinStrings(parts, ", "))
	return totalMinutes, reason, segments
}

func joinStrings(ss []string, sep string) string {
	if len(ss) == 0 {
		return ""
	}
	result := ss[0]
	for i := 1; i < len(ss); i++ {
		result += sep + ss[i]
	}
	return result
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

		if t.UserID != nil {
			if om, ok := orgMappings[*t.UserID]; ok {
				item.Org1 = om.Org1
				item.Org2 = om.Org2
				item.Org3 = om.Org3
				item.Org4 = om.Org4
				item.Org5 = om.Org5
				item.Org6 = om.Org6
				item.Org7 = om.Org7
				item.Org8 = om.Org8
				item.Org9 = om.Org9
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

	realMinutes, realReason, segments := calculateTaskRealMinutes(convs, appConfig.TaskRealMinutes.GapThresholdMinutes, appConfig.TaskRealMinutes.ExtensionMinutes)
	task.TaskRealMinutes = &realMinutes
	task.TaskRealMinutesReason = &realReason

	go func(taskID string, rm float64, rr string) {
		UpdateTaskRealMinutes(statDB, taskID, rm, rr)
	}(task.TaskID, realMinutes, realReason)

	if task.Title == nil || *task.Title == "" {
		var userInputs []string
		for _, conv := range convs {
			if conv.UserInput != nil && *conv.UserInput != "" {
				userInputs = append(userInputs, *conv.UserInput)
			}
		}
		if len(userInputs) > 0 {
			go callAIForTaskTitle(task.TaskID, userInputs)
		}
	}

	var efficiencyRatio *float64
	effectiveAncient := task.TaskAncientMinutes
	if task.TaskAncientMinutesManual != nil {
		effectiveAncient = task.TaskAncientMinutesManual
	}
	effectiveReal := task.TaskRealMinutes
	if task.TaskRealMinutesManual != nil {
		effectiveReal = task.TaskRealMinutesManual
	}
	if effectiveAncient != nil && effectiveReal != nil && *effectiveReal > 0 && *effectiveAncient > 0 {
		ratio := utils.CalcEfficiencyRatio(*effectiveAncient, *effectiveReal)
		efficiencyRatio = &ratio
	}

	c.JSON(http.StatusOK, TaskDetailResponse{Task: task, Conversations: convs, TimeSegments: segments, EfficiencyRatio: efficiencyRatio})
}

// callAIForTaskTitle 调用 AI 从对话记录提取任务标题（不超过100字符）
func callAIForTaskTitle(taskID string, userInputs []string) {
	cfg := appConfig.AIEstimation
	if !cfg.Enabled || cfg.APIKey == "" {
		return
	}

	prompt := fmt.Sprintf(`请根据以下用户与AI助手的对话记录，用一句简短的中文描述这个任务的目标，不超过100个字符。
只输出标题文本，不要任何额外格式或引号。

用户输入记录：
%s`, truncateSlice(userInputs, 3000))

	reqBody := map[string]interface{}{
		"model":      cfg.Model,
		"max_tokens": 256,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	url := strings.TrimRight(cfg.BaseURL, "/") + "/v1/messages"
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		log.Printf("创建AI请求失败(title): %v", err)
		return
	}
	httpReq.Header.Set("x-api-key", cfg.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: time.Duration(cfg.TimeoutMS) * time.Millisecond}
	resp, err := client.Do(httpReq)
	if err != nil {
		log.Printf("AI请求失败(title): %v", err)
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		log.Printf("AI返回非200(title): %d, %s", resp.StatusCode, string(respBody))
		return
	}

	var anthropicResp struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBody, &anthropicResp); err != nil || len(anthropicResp.Content) == 0 {
		log.Printf("解析AI响应失败(title): %v", err)
		return
	}

	title := strings.TrimSpace(anthropicResp.Content[0].Text)
	title = strings.Trim(title, "\"'`")
	runes := []rune(title)
	if len(runes) > 100 {
		title = string(runes[:100])
	}
	if title == "" {
		return
	}

	UpdateTaskTitle(statDB, taskID, title)
	log.Printf("AI提取title完成: task=%s, title=%s", taskID, title)
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

// estimateAncientMinutes POST /api/v2/tasks/estimate-ancient
// @Summary AI估算任务耗时
// @Description 使用AI估算指定任务或未估算任务的耗时
// @Tags Tasks
// @Produce json
// @Param taskId query string false "指定任务ID(为空则批量估算最近50个未估算任务)"
// @Success 200 {object} EstimateAncientResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/tasks/estimate-ancient [post]
func estimateAncientMinutes(c *gin.Context) {
	cfg := appConfig.AIEstimation
	if !cfg.Enabled || cfg.APIKey == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "AI estimation not enabled or API key missing"})
		return
	}

	taskId := c.Query("taskId")

	taskIDs, err := GetTaskIDsForEstimation(statDB, taskId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	if len(taskIDs) == 0 {
		c.JSON(http.StatusOK, EstimateAncientResponse{Status: "ok", Total: 0, Success: 0})
		return
	}

	var results []EstimateAncientResult

	for _, tid := range taskIDs {
		userInputs, codeOutputs, totalChars, totalLines, err := GetConvInputForEstimation(statDB, tid)
		if err != nil {
			results = append(results, EstimateAncientResult{TaskID: tid, Error: err.Error()})
			continue
		}

		if len(userInputs) == 0 {
			results = append(results, EstimateAncientResult{TaskID: tid, Error: "no conversation data"})
			continue
		}

		minutes, reason, err := callAIForAncientEstimation(userInputs, codeOutputs, totalChars, totalLines)
		if err != nil {
			results = append(results, EstimateAncientResult{TaskID: tid, Error: err.Error()})
			continue
		}

		if err := UpdateTaskAncientEstimation(statDB, tid, minutes, reason); err != nil {
			results = append(results, EstimateAncientResult{TaskID: tid, Minutes: minutes, Reason: reason, Error: "db update failed: " + err.Error()})
			continue
		}

		results = append(results, EstimateAncientResult{TaskID: tid, Minutes: minutes, Reason: reason})
		log.Printf("AI估时完成: task=%s, minutes=%.1f", tid, minutes)
	}

	successCount := 0
	for _, r := range results {
		if r.Error == "" {
			successCount++
		}
	}

	c.JSON(http.StatusOK, EstimateAncientResponse{Status: "ok", Total: len(taskIDs), Success: successCount, Results: results})
}

// callAIForAncientEstimation 调用 AI 估算传统开发时长
func callAIForAncientEstimation(userInputs []string, codeOutputs []string, totalChars int64, totalLines int64) (float64, string, error) {
	cfg := appConfig.AIEstimation
	prompt := fmt.Sprintf(`你是一个经验丰富的软件项目经理，擅长评估软件开发工作量。

请根据以下用户与 AI 的对话记录，分析用户的需求复杂度，并估算如果由传统人工开发（不使用AI），实现该需求所需的**分钟数**。

重点关注：
1. 用户需求的复杂程度
2. 涉及的功能模块数量
3. 技术难度（如是否需要处理并发、安全、性能等问题）
4. 代码量规模

用户输入记录（按时间顺序）：
%s

AI 生成的代码片段：
%s

总输入字符数：%d
总代码行数：%d

请输出 JSON 格式：
{
  "task_ancient_minutes": 270,
  "task_ancient_minutes_reason": "估算理由..."
}`,
		truncateSlice(userInputs, 5000),
		truncateSlice(codeOutputs, 8000),
		totalChars,
		totalLines,
	)

	reqBody := map[string]interface{}{
		"model":      cfg.Model,
		"max_tokens": 1024,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	url := strings.TrimRight(cfg.BaseURL, "/") + "/v1/messages"
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return 0, "", err
	}
	httpReq.Header.Set("x-api-key", cfg.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: time.Duration(cfg.TimeoutMS) * time.Millisecond}
	resp, err := client.Do(httpReq)
	if err != nil {
		return 0, "", fmt.Errorf("AI API 请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return 0, "", fmt.Errorf("AI API 返回 %d: %s", resp.StatusCode, string(respBody))
	}

	var anthropicResp struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBody, &anthropicResp); err != nil {
		return 0, "", fmt.Errorf("解析响应失败: %w", err)
	}
	if len(anthropicResp.Content) == 0 {
		return 0, "", fmt.Errorf("AI 响应 content 为空")
	}

	jsonText := extractJSON(anthropicResp.Content[0].Text)
	var result struct {
		Minutes float64 `json:"task_ancient_minutes"`
		Reason  string  `json:"task_ancient_minutes_reason"`
	}
	if err := json.Unmarshal([]byte(jsonText), &result); err != nil {
		return 0, "", fmt.Errorf("解析估时结果失败: %w, text: %s", err, anthropicResp.Content[0].Text)
	}

	if result.Minutes < 0 || result.Minutes > 100000 {
		return 0, "", fmt.Errorf("估时结果异常: %.2f", result.Minutes)
	}

	return result.Minutes, result.Reason, nil
}

// truncateSlice 将字符串切片拼接后截断到 maxLen 字符
func truncateSlice(items []string, maxLen int) string {
	var sb strings.Builder
	for i, s := range items {
		if sb.Len()+len(s) > maxLen {
			remaining := maxLen - sb.Len()
			if remaining > 0 {
				sb.WriteString(s[:remaining])
				sb.WriteString("...(截断)")
			}
			break
		}
		if i > 0 {
			sb.WriteString("\n---\n")
		}
		sb.WriteString(s)
	}
	return sb.String()
}
