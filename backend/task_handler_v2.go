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
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// TimeSegment 表示一个连续的工作时间片段
type TimeSegment struct {
	Start     time.Time `json:"start"`
	End       time.Time `json:"end"`
	ConvCount int       `json:"conv_count"`
}

// calculateTaskRealMinutes 根据对话时间戳计算实际工作时长
func calculateTaskRealMinutes(conversations []StatTaskConversation, gapThreshold int, extensionMin int) (float64, string, []TimeSegment) {
	// 过滤出有效的 start_time
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

	// 初始化第一个片段
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
	// 最后一个片段加 extension
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

// upsertTaskV2 POST /api/v2/tasks
// @Summary 创建或更新任务
// @Description 创建新任务或更新已有任务信息
// @Tags Tasks
// @Accept json
// @Produce json
// @Param task body StatTask true "任务信息"
// @Success 200 {object} object
// @Failure 400 {object} object
// @Router /api/v2/tasks [post]
func upsertTaskV2(c *gin.Context) {
	var task StatTask
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 验证必填字段
	if strings.TrimSpace(task.TaskID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "task_id 不能为空"})
		return
	}

	// 幂等性保障：先查询记录是否存在
	existingTask, err := GetStatTask(statDB, task.TaskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询任务失败: " + err.Error()})
		return
	}

	// 执行 upsert 操作
	if err := UpsertStatTask(statDB, &task); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 返回操作结果
	if existingTask != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "updated",
			"task_id": task.TaskID,
			"action":  "记录已更新",
		})
	} else {
		c.JSON(http.StatusCreated, gin.H{
			"status":  "created",
			"task_id": task.TaskID,
			"action":  "新记录已创建",
		})
	}
}

// batchUpsertConversationsV2 POST /api/v2/tasks/conversations/batch
// @Summary 批量添加或更新任务对话
// @Description 批量添加或更新任务的对话记录
// @Tags Tasks
// @Accept json
// @Produce json
// @Param conversations body []StatTaskConversation true "对话记录列表"
// @Success 200 {object} object
// @Failure 400 {object} object
// @Router /api/v2/tasks/conversations/batch [post]
func batchUpsertConversationsV2(c *gin.Context) {
	var convs []StatTaskConversation
	if err := c.ShouldBindJSON(&convs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(convs) == 0 {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "count": 0})
		return
	}
	if err := BatchInsertStatTaskConversations(statDB, convs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "count": len(convs)})
}

// listTasksV2 GET /api/v2/tasks
// @Summary 获取任务列表
// @Description 按条件查询任务列表，支持日期范围过滤
// @Tags Tasks
// @Produce json
// @Param startDate query string false "开始日期"
// @Param endDate query string false "结束日期"
// @Param dimension query string false "维度过滤"
// @Success 200 {object} object
// @Router /api/v2/tasks [get]
func listTasksV2(c *gin.Context) {
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	if startDate == "" || endDate == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "startDate 和 endDate 为必填参数"})
		return
	}

	startT, err := parseDateParam(startDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "startDate 格式错误: " + err.Error()})
		return
	}
	endT, err := parseDateParam(endDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "endDate 格式错误: " + err.Error()})
		return
	}

	startTime := startT.Format(time.RFC3339)
	endTime := endT.Add(23*time.Hour + 59*time.Minute + 59*time.Second).Format(time.RFC3339)

	userID := c.Query("userId")
	workDirID := c.Query("workDirId")
	page := getDefaultInt(c, "page", 1)
	pageSize := getDefaultInt(c, "pageSize", DefaultPageSize)

	total, err := CountStatTasks(statDB, userID, workDirID, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	list, err := ListStatTasks(statDB, userID, workDirID, startTime, endTime, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 为每条 task 计算 efficiency_ratio
	results := make([]gin.H, len(list))
	for i, t := range list {
		item := gin.H{
			"task_id":                            t.TaskID,
			"title":                              t.Title,
			"user_id":                            t.UserID,
			"user_name":                          t.UserName,
			"client_id":                          t.ClientID,
			"client_ide":                         t.ClientIDE,
			"client_version":                     t.ClientVersion,
			"client_os":                          t.ClientOS,
			"client_os_version":                  t.ClientOSVersion,
			"caller":                             t.Caller,
			"repo_addr":                          t.RepoAddr,
			"repo_branch":                        t.RepoBranch,
			"work_dir":                           t.WorkDir,
			"work_dir_id":                        t.WorkDirID,
			"start_time":                         t.StartTime,
			"end_time":                           t.EndTime,
			"upstream_tokens":                    t.UpstreamTokens,
			"downstream_tokens":                  t.DownstreamTokens,
			"cost":                               t.Cost,
			"diff_lines":                         t.DiffLines,
			"task_ancient_minutes":               t.TaskAncientMinutes,
			"task_ancient_minutes_reason":        t.TaskAncientMinutesReason,
			"task_ancient_minutes_manual":        t.TaskAncientMinutesManual,
			"task_ancient_minutes_reason_manual": t.TaskAncientMinutesReasonManual,
			"task_real_minutes":                  t.TaskRealMinutes,
			"task_real_minutes_reason":           t.TaskRealMinutesReason,
			"task_real_minutes_manual":           t.TaskRealMinutesManual,
			"task_real_minutes_reason_manual":    t.TaskRealMinutesReasonManual,
			"created_at":                         t.CreatedAt,
			"updated_at":                         t.UpdatedAt,
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
			ratio := (*effectiveAncient / *effectiveReal) * 100
			item["efficiency_ratio"] = ratio
		} else {
			item["efficiency_ratio"] = nil
		}

		// 补充 org 字段
		if t.UserID != nil {
			if om, ok := orgMappings[*t.UserID]; ok {
				item["org1"] = om.Org1
				item["org2"] = om.Org2
				item["org3"] = om.Org3
				item["org4"] = om.Org4
			} else {
				item["org1"] = ""
				item["org2"] = ""
				item["org3"] = ""
				item["org4"] = ""
			}
		} else {
			item["org1"] = ""
			item["org2"] = ""
			item["org3"] = ""
			item["org4"] = ""
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

// getTaskDetailV2 GET /api/v2/tasks/:taskId
// @Summary 获取任务详情
// @Description 根据任务ID获取任务详细信息
// @Tags Tasks
// @Produce json
// @Param taskId path string true "任务ID"
// @Success 200 {object} object
// @Failure 404 {object} object
// @Router /api/v2/tasks/{taskId} [get]
func getTaskDetailV2(c *gin.Context) {
	taskId := c.Param("taskId")

	task, err := GetStatTask(statDB, taskId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if task == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}

	convs, err := ListStatTaskConversations(statDB, taskId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 计算 task_real_minutes
	realMinutes, realReason, segments := calculateTaskRealMinutes(convs, appConfig.TaskRealMinutes.GapThresholdMinutes, appConfig.TaskRealMinutes.ExtensionMinutes)
	task.TaskRealMinutes = &realMinutes
	task.TaskRealMinutesReason = &realReason

	go func(taskID string, rm float64, rr string) {
		_, err := statDB.Exec("UPDATE tasks SET task_real_minutes = $1, task_real_minutes_reason = $2 WHERE task_id = $3", rm, rr, taskID)
		if err != nil {
			log.Printf("异步更新 task_real_minutes 失败: %v", err)
		}
	}(task.TaskID, realMinutes, realReason)

	// 如果 title 为空，异步调 AI 提取
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

	// 计算 efficiency_ratio
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
		ratio := (*effectiveAncient / *effectiveReal) * 100
		efficiencyRatio = &ratio
	}

	c.JSON(http.StatusOK, gin.H{
		"task":             task,
		"conversations":    convs,
		"time_segments":    segments,
		"efficiency_ratio": efficiencyRatio,
	})
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
	// 去除可能的引号包裹
	title = strings.Trim(title, "\"'`")
	// 截断到100字符
	runes := []rune(title)
	if len(runes) > 100 {
		title = string(runes[:100])
	}
	if title == "" {
		return
	}

	_, err = statDB.Exec("UPDATE tasks SET title = $1, updated_at = NOW() WHERE task_id = $2", title, taskID)
	if err != nil {
		log.Printf("回写title失败: %v", err)
	} else {
		log.Printf("AI提取title完成: task=%s, title=%s", taskID, title)
	}
}

// updateTaskManualV2 PUT /api/v2/tasks/:taskId/manual
// @Summary 更新任务人工数据
// @Description 更新任务的人工修改数据
// @Tags Tasks
// @Accept json
// @Produce json
// @Param taskId path string true "任务ID"
// @Param data body object true "人工数据"
// @Success 200 {object} object
// @Failure 400 {object} object
// @Router /api/v2/tasks/{taskId}/manual [put]
func updateTaskManualV2(c *gin.Context) {
	taskId := c.Param("taskId")
	if taskId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "taskId 不能为空"})
		return
	}

	var req struct {
		TaskRealMinutesManual          *float64 `json:"task_real_minutes_manual"`
		TaskRealMinutesReasonManual    *string  `json:"task_real_minutes_reason_manual"`
		TaskAncientMinutesManual       *float64 `json:"task_ancient_minutes_manual"`
		TaskAncientMinutesReasonManual *string  `json:"task_ancient_minutes_reason_manual"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := UpdateStatTaskManual(statDB, taskId, req.TaskRealMinutesManual, req.TaskRealMinutesReasonManual, req.TaskAncientMinutesManual, req.TaskAncientMinutesReasonManual); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// extractMinutesFromReason 从 reason 文本中提取分钟数
func extractMinutesFromReason(reason string) (float64, string) {
	// a) 综合评估行
	re := regexp.MustCompile(`综合评估[:：]\s*(\d+\.?\d*)\s*人天`)
	if m := re.FindStringSubmatch(reason); m != nil {
		v, _ := strconv.ParseFloat(m[1], 64)
		return v * 480, "综合评估:" + m[1] + "人天"
	}

	// b) 人天格式
	re = regexp.MustCompile(`(\d+\.?\d*)\s*人天`)
	if m := re.FindStringSubmatch(reason); m != nil {
		v, _ := strconv.ParseFloat(m[1], 64)
		return v * 480, m[1] + "人天"
	}

	// c) 工作日格式
	re = regexp.MustCompile(`(\d+\.?\d*)\s*个?工作日`)
	if m := re.FindStringSubmatch(reason); m != nil {
		v, _ := strconv.ParseFloat(m[1], 64)
		return v * 480, m[1] + "工作日"
	}

	// d) 天格式（排除无关词）
	reDays := regexp.MustCompile(`(\d+\.?\d*)\s*天`)
	dayMatches := reDays.FindAllStringSubmatch(reason, -1)
	dayMatchIndices := reDays.FindAllStringIndex(reason, -1)
	var validDayValues []float64
	for i, m := range dayMatches {
		idx := dayMatchIndices[i]
		// 排除"每天"
		if idx[0] > 0 {
			rBefore := []rune(reason[:idx[0]])
			if len(rBefore) > 0 && rBefore[len(rBefore)-1] == '每' {
				continue
			}
		}
		// 排除"天前"、"天内"
		after := reason[idx[1]:]
		rAfter := []rune(after)
		if len(rAfter) > 0 && (rAfter[0] == '前' || rAfter[0] == '内') {
			continue
		}
		v, _ := strconv.ParseFloat(m[1], 64)
		validDayValues = append(validDayValues, v)
	}
	if len(validDayValues) > 0 {
		hasChaijie := strings.Contains(reason, "拆解")
		if hasChaijie || len(validDayValues) > 1 {
			var total float64
			for _, v := range validDayValues {
				total += v
			}
			return total * 480, fmt.Sprintf("工作量拆解:%.1f天", total)
		}
		return validDayValues[0] * 480, fmt.Sprintf("%.1f天", validDayValues[0])
	}

	// e) 小时格式
	re = regexp.MustCompile(`(\d+\.?\d*)\s*小时`)
	if m := re.FindStringSubmatch(reason); m != nil {
		v, _ := strconv.ParseFloat(m[1], 64)
		return v * 60, m[1] + "小时"
	}

	// f) 半小时
	if strings.Contains(reason, "半小时") {
		return 30, "半小时"
	}

	// g) 分钟格式
	re = regexp.MustCompile(`(\d+\.?\d*)\s*分钟`)
	if m := re.FindStringSubmatch(reason); m != nil {
		v, _ := strconv.ParseFloat(m[1], 64)
		return v, m[1] + "分钟"
	}

	// h) 几分钟
	if strings.Contains(reason, "几分钟") {
		return 5, "几分钟"
	}

	// i) 0人天/0天
	if strings.Contains(reason, "0人天") || strings.Contains(reason, "所需人天为 0") || strings.Contains(reason, "所需人天为0") || strings.Contains(reason, "人天数为0") || strings.Contains(reason, "人天数为 0") {
		return 0, "0人天"
	}

	// j) 关键词估算
	if strings.Contains(reason, "极低") || strings.Contains(reason, "极小") || strings.Contains(reason, "极简") {
		return 5, "关键词估算:极小"
	}
	if strings.Contains(reason, "简单") || strings.Contains(reason, "轻量") || strings.Contains(reason, "基础") {
		return 30, "关键词估算:简单"
	}
	if strings.Contains(reason, "低") || strings.Contains(reason, "较小") || strings.Contains(reason, "较少") || strings.Contains(reason, "日常") {
		return 60, "关键词估算:较低"
	}
	if strings.Contains(reason, "中等") || strings.Contains(reason, "适中") {
		return 240, "关键词估算:中等"
	}
	if strings.Contains(reason, "复杂") || strings.Contains(reason, "较高") || strings.Contains(reason, "高") {
		return 480, "关键词估算:复杂"
	}

	return 60, "默认估算"
}

// fixAncientMinutes POST /api/v2/tasks/fix-ancient-minutes
// @Summary 修复古代工时数据
// @Description 从任务古代工时原因中提取工时数值并更新到数据库
// @Tags Tasks
// @Produce json
// @Success 200 {object} object
// @Router /api/v2/tasks/fix-ancient-minutes [post]
func fixAncientMinutes(c *gin.Context) {
	rows, err := statDB.Query(`SELECT task_id, task_ancient_minutes_reason FROM tasks WHERE task_ancient_minutes IS NULL AND task_ancient_minutes_reason IS NOT NULL AND task_ancient_minutes_reason != ''`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type fixResult struct {
		TaskID  string  `json:"task_id"`
		Minutes float64 `json:"minutes"`
		Method  string  `json:"method"`
	}

	var results []fixResult
	for rows.Next() {
		var taskID, reason string
		if err := rows.Scan(&taskID, &reason); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		minutes, method := extractMinutesFromReason(reason)
		if _, err := statDB.Exec(`UPDATE tasks SET task_ancient_minutes = $1 WHERE task_id = $2`, minutes, taskID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("update task %s failed: %v", taskID, err)})
			return
		}
		results = append(results, fixResult{TaskID: taskID, Minutes: minutes, Method: method})
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"count":   len(results),
		"results": results,
	})
}

// getTaskFile GET /api/v2/tasks/file
// @Summary 获取任务文件
// @Description 获取任务的原始文件内容
// @Tags Tasks
// @Produce json
// @Param taskId query string true "任务ID"
// @Param type query string false "文件类型"
// @Success 200 {object} object
// @Failure 404 {object} object
// @Router /api/v2/tasks/file [get]
func getTaskFile(c *gin.Context) {
	typ := c.Query("type")
	taskId := c.Query("taskId")
	date := c.Query("date")

	if typ != "summary" && typ != "conversation" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type must be summary or conversation"})
		return
	}
	if taskId == "" || date == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "taskId and date are required"})
		return
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date must be YYYY-MM-DD format"})
		return
	}
	if strings.Contains(taskId, "..") || strings.Contains(taskId, "/") || strings.Contains(taskId, "\\") ||
		strings.Contains(date, "..") || strings.Contains(date, "/") || strings.Contains(date, "\\") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid characters in parameters"})
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
		c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在"})
		return
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Data(http.StatusOK, contentType, data)
}

// estimateAncientMinutes POST /api/v2/tasks/estimate-ancient
// estimateAncientMinutes POST /api/v2/tasks/estimate-ancient
// @Summary 估算古代工时
// @Description 使用AI从对话记录中估算任务的古代工时
// @Tags Tasks
// @Produce json
// @Success 200 {object} object
// @Failure 500 {object} object
// @Router /api/v2/tasks/estimate-ancient [post]
func estimateAncientMinutes(c *gin.Context) {
	cfg := appConfig.AIEstimation
	if !cfg.Enabled || cfg.APIKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "AI estimation not enabled or API key missing"})
		return
	}

	// 可选参数：指定单个 taskId
	taskId := c.Query("taskId")

	var query string
	var args []interface{}
	if taskId != "" {
		query = `SELECT task_id FROM tasks WHERE task_id = $1 AND task_ancient_minutes IS NULL AND task_ancient_minutes_manual IS NULL`
		args = []interface{}{taskId}
	} else {
		query = `SELECT task_id FROM tasks WHERE task_ancient_minutes IS NULL AND task_ancient_minutes_manual IS NULL ORDER BY start_time DESC LIMIT 50`
	}

	rows, err := statDB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var taskIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		taskIDs = append(taskIDs, id)
	}

	if len(taskIDs) == 0 {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "no tasks need estimation", "count": 0})
		return
	}

	type estimateResult struct {
		TaskID  string  `json:"task_id"`
		Minutes float64 `json:"minutes"`
		Reason  string  `json:"reason"`
		Error   string  `json:"error,omitempty"`
	}

	var results []estimateResult

	for _, tid := range taskIDs {
		// 读取对话记录
		convRows, err := statDB.Query(
			`SELECT user_input, diff, diff_lines, upstream_tokens, downstream_tokens
			 FROM task_conversations WHERE task_id = $1 ORDER BY start_time`, tid)
		if err != nil {
			results = append(results, estimateResult{TaskID: tid, Error: err.Error()})
			continue
		}

		var userInputs []string
		var codeOutputs []string
		var totalLines int64
		var totalChars int64
		for convRows.Next() {
			var userInput, diff *string
			var diffLines, upTokens, downTokens *int64
			if err := convRows.Scan(&userInput, &diff, &diffLines, &upTokens, &downTokens); err != nil {
				continue
			}
			if userInput != nil && *userInput != "" {
				userInputs = append(userInputs, *userInput)
				totalChars += int64(len(*userInput))
			}
			if diff != nil && *diff != "" {
				codeOutputs = append(codeOutputs, *diff)
			}
			if diffLines != nil {
				totalLines += *diffLines
			}
		}
		convRows.Close()

		if len(userInputs) == 0 {
			results = append(results, estimateResult{TaskID: tid, Error: "no conversation data"})
			continue
		}

		// 构建 prompt
		minutes, reason, err := callAIForAncientEstimation(userInputs, codeOutputs, totalChars, totalLines)
		if err != nil {
			results = append(results, estimateResult{TaskID: tid, Error: err.Error()})
			continue
		}

		// 回写 DB
		_, err = statDB.Exec(
			`UPDATE tasks SET task_ancient_minutes = $1, task_ancient_minutes_reason = $2, updated_at = NOW() WHERE task_id = $3`,
			minutes, reason, tid)
		if err != nil {
			results = append(results, estimateResult{TaskID: tid, Minutes: minutes, Reason: reason, Error: "db update failed: " + err.Error()})
			continue
		}

		results = append(results, estimateResult{TaskID: tid, Minutes: minutes, Reason: reason})
		log.Printf("AI估时完成: task=%s, minutes=%.1f", tid, minutes)
	}

	successCount := 0
	for _, r := range results {
		if r.Error == "" {
			successCount++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"total":   len(taskIDs),
		"success": successCount,
		"results": results,
	})
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
