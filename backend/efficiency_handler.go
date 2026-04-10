package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// userTaskInfo 单个用户的任务聚合信息
type userTaskInfo struct {
	UserID      string
	UserName    string
	StartTime   time.Time
	EndTime     time.Time
	LeadTimeMs  int64
	ProcessTime int64
	Tasks       []map[string]interface{}
}

// analysisResult 实时计算的分析结果
type analysisResult struct {
	AIEstimatedDays    float64
	AIEstimatedReasons []string
	EfficiencyReason   string
	TotalCodeLines     int64
	APICost            float64
	TotalLeadTimeMs    int64
	TotalProcessTimeMs int64
	UserCount          int
	StartTime          time.Time
	EndTime            time.Time
	Users              []userTaskInfo
	TaskCount          int
}

// computeFromES 从 ES task 数据实时计算分析结果
func computeFromES(dimension, id, startDate, endDate string) (*analysisResult, error) {
	indexNames, err := generateIndexNames(ESTaskIndexPrefix, startDate, endDate)
	if err != nil {
		return nil, err
	}

	esField := "project_id"
	if dimension == "repo" {
		esField = "repo_id"
	}

	query := map[string]interface{}{
		"bool": map[string]interface{}{
			"filter": []map[string]interface{}{
				{"term": map[string]interface{}{esField: id}},
			},
		},
	}

	result, err := esClient.Search(indexNames, query, 0, ESMaxSearchSize, "api_request_time", "asc")
	if err != nil {
		return nil, fmt.Errorf("查询ES失败: %w", err)
	}

	if len(result.Hits) == 0 {
		return nil, nil
	}

	// 按 user_id 分组
	userMap := make(map[string]*userTaskInfo)
	var userOrder []string
	for _, hit := range result.Hits {
		uid := ""
		if v, ok := hit["user_id"]; ok {
			uid = fmt.Sprintf("%v", v)
		}
		if uid == "" {
			uid = "unknown"
		}

		info, exists := userMap[uid]
		if !exists {
			uname := uid
			if v, ok := hit["user_name"]; ok && v != nil {
				uname = fmt.Sprintf("%v", v)
			}
			info = &userTaskInfo{UserID: uid, UserName: uname}
			userMap[uid] = info
			userOrder = append(userOrder, uid)
		}
		info.Tasks = append(info.Tasks, hit)
	}

	var totalAIEstimatedDays, totalAPICost float64
	var totalLeadTimeMs, totalProcessTimeMs int64
	var globalStart, globalEnd time.Time
	var allUsers []userTaskInfo
	var allReasons []string
	var totalCodeLines int64

	for _, uid := range userOrder {
		info := userMap[uid]
		tasks := info.Tasks

		var userStart, userEnd time.Time
		var userAIEst, userAPICost float64
		var userReasons []string
		var userCodeLines int64

		for i, task := range tasks {
			reqTime, okReq := parseESTime(task["api_request_time"])
			endTime, okEnd := parseESTime(task["api_end_time"])

			if okReq {
				if i == 0 || reqTime.Before(userStart) {
					userStart = reqTime
				}
			}
			if okEnd {
				if i == 0 || endTime.After(userEnd) {
					userEnd = endTime
				}
			}

			userAIEst += getFloat64(task, "ai_estimated_days")
			userAPICost += getFloat64(task, "api_cost")

			// 提取reason
			if reason, ok := task["ai_estimated_reason"]; ok && reason != nil {
				if r := fmt.Sprintf("%v", reason); r != "" && r != "<nil>" {
					userReasons = append(userReasons, r)
				}
			}
			// 累加代码行数
			userCodeLines += int64(getFloat64(task, "code_lines"))
		}

		userLeadTimeMs := userEnd.Sub(userStart).Milliseconds()

		// process_time 合并算法
		type taskTime struct {
			reqTime time.Time
			endTime time.Time
		}
		var taskTimes []taskTime
		for _, task := range tasks {
			reqTime, okReq := parseESTime(task["api_request_time"])
			endTime, okEnd := parseESTime(task["api_end_time"])
			if okReq && okEnd {
				taskTimes = append(taskTimes, taskTime{reqTime, endTime})
			}
		}
		sort.Slice(taskTimes, func(i, j int) bool {
			return taskTimes[i].reqTime.Before(taskTimes[j].reqTime)
		})

		var userProcessTimeMs int64
		if len(taskTimes) > 0 {
			segStart := taskTimes[0].reqTime
			segEnd := taskTimes[0].endTime
			for k := 1; k < len(taskTimes); k++ {
				gap := taskTimes[k].reqTime.Sub(segEnd).Milliseconds()
				if gap <= int64(ProcessTimeGapMs) {
					if taskTimes[k].endTime.After(segEnd) {
						segEnd = taskTimes[k].endTime
					}
				} else {
					userProcessTimeMs += segEnd.Sub(segStart).Milliseconds()
					segStart = taskTimes[k].reqTime
					segEnd = taskTimes[k].endTime
				}
			}
			userProcessTimeMs += segEnd.Sub(segStart).Milliseconds()
		}

		userInfo := userTaskInfo{
			UserID:      uid,
			UserName:    info.UserName,
			StartTime:   userStart,
			EndTime:     userEnd,
			LeadTimeMs:  userLeadTimeMs,
			ProcessTime: userProcessTimeMs,
		}
		allUsers = append(allUsers, userInfo)

		totalAIEstimatedDays += userAIEst
		totalAPICost += userAPICost
		totalLeadTimeMs += userLeadTimeMs
		totalProcessTimeMs += userProcessTimeMs
		allReasons = append(allReasons, userReasons...)
		totalCodeLines += userCodeLines

		if globalStart.IsZero() || userStart.Before(globalStart) {
			globalStart = userStart
		}
		if globalEnd.IsZero() || userEnd.After(globalEnd) {
			globalEnd = userEnd
		}
	}

	return &analysisResult{
		AIEstimatedDays:    totalAIEstimatedDays,
		AIEstimatedReasons: allReasons,
		TotalCodeLines:     totalCodeLines,
		APICost:            totalAPICost,
		TotalLeadTimeMs:    totalLeadTimeMs,
		TotalProcessTimeMs: totalProcessTimeMs,
		UserCount:          len(allUsers),
		StartTime:          globalStart,
		EndTime:            globalEnd,
		Users:              allUsers,
		TaskCount:          len(result.Hits),
	}, nil
}

// buildEfficiencyResponse 构建提效分析 JSON 响应
// aiRatioProcess > 0 时优先使用 AI 综合评估的提效比例
func buildEfficiencyResponse(dimension, dimensionID, analysisDate string,
	rawDays float64, correctedDays *float64,
	totalLeadTimeMs, totalProcessTimeMs int64,
	userCount int, startTime, endTime time.Time,
	users []userTaskInfo, apiCost float64, analysisFile string,
	reasons []string, efficiencyReason string, aiRatioProcess float64,
	totalCodeLines int64) gin.H {

	isCorrected := correctedDays != nil
	var correctedVal interface{}
	if correctedDays != nil {
		correctedVal = *correctedDays
	}

	// 计算提效比例
	aiDays := rawDays
	if correctedDays != nil {
		aiDays = *correctedDays
	}
	var ratioLead, ratioProcess float64
	if aiRatioProcess > 0 {
		// AI 综合评估：优先使用 AI 给出的提效比例
		ratioProcess = aiRatioProcess
		if totalLeadTimeMs > 0 && totalProcessTimeMs > 0 {
			ratioLead = ratioProcess * float64(totalProcessTimeMs) / float64(totalLeadTimeMs)
		}
	} else {
		// 降级：简单计算 + clamp 防止极端值
		if totalLeadTimeMs > 0 {
			ratioLead = math.Round(aiDays/float64(totalLeadTimeMs)*MsPerWorkDay*1000) / 10
			ratioLead = math.Max(EfficiencyRatioMin, math.Min(EfficiencyRatioMax, ratioLead))
		}
		if totalProcessTimeMs > 0 {
			ratioProcess = math.Round(aiDays/float64(totalProcessTimeMs)*MsPerWorkDay*1000) / 10
			ratioProcess = math.Max(EfficiencyRatioMin, math.Min(EfficiencyRatioMax, ratioProcess))
		}
	}

	// 成本
	dailyRate := DefaultDailyRate
	costSaving := math.Round((aiDays*dailyRate-apiCost)*10) / 10
	var roi float64
	if apiCost > 0 {
		roi = math.Round(costSaving/apiCost*1000) / 10
		roi = math.Max(-1000, math.Min(100000, roi))
	}

	// 用户列表
	userList := make([]gin.H, 0, len(users))
	for _, u := range users {
		userList = append(userList, gin.H{
			"user_id":         u.UserID,
			"user_name":       u.UserName,
			"start_time":      u.StartTime.Format(time.RFC3339),
			"end_time":        u.EndTime.Format(time.RFC3339),
			"lead_time_ms":    u.LeadTimeMs,
			"process_time_ms": u.ProcessTime,
		})
	}

	// 处理 reasons
	aiReasons := reasons
	if aiReasons == nil {
		aiReasons = []string{}
	}

	return gin.H{
		"dimension":     dimension,
		"dimension_id":  dimensionID,
		"analysis_date": analysisDate,
		"ai_estimated": gin.H{
			"raw_days":       rawDays,
			"corrected_days": correctedVal,
			"is_corrected":   isCorrected,
			"reasons":        aiReasons,
		},
		"actual_time": gin.H{
			"total_lead_time_ms":    totalLeadTimeMs,
			"total_process_time_ms": totalProcessTimeMs,
			"total_code_lines":      totalCodeLines,
			"user_count":            userCount,
			"start_time":            startTime.Format(time.RFC3339),
			"end_time":              endTime.Format(time.RFC3339),
			"users":                 userList,
		},
		"efficiency": gin.H{
			"ratio_lead":    ratioLead,
			"ratio_process": ratioProcess,
			"reason":        efficiencyReason,
		},
		"cost": gin.H{
			"api_cost":    apiCost,
			"daily_rate":  dailyRate,
			"cost_saving": costSaving,
			"roi":         roi,
		},
		"analysis_file": analysisFile,
	}
}

// buildResponseFromProjectMetrics 从 PG ProjectMetrics 构建响应
func buildResponseFromProjectMetrics(m *ProjectMetrics) gin.H {
	var rawDays, apiCost float64
	var totalLeadTimeMs, totalProcessTimeMs int64
	var userCount int
	var startTime, endTime time.Time
	var analysisFile string

	if m.RawAIEstimatedDays != nil {
		rawDays = *m.RawAIEstimatedDays
	}
	if m.APICost != nil {
		apiCost = *m.APICost
	}
	if m.TotalLeadTimeMs != nil {
		totalLeadTimeMs = *m.TotalLeadTimeMs
	}
	if m.TotalProcessTimeMs != nil {
		totalProcessTimeMs = *m.TotalProcessTimeMs
	}
	if m.UserCount != nil {
		userCount = *m.UserCount
	}
	if m.ActualStartTime != nil {
		startTime = *m.ActualStartTime
	}
	if m.ActualEndTime != nil {
		endTime = *m.ActualEndTime
	}
	if m.AnalysisFilePath != nil {
		analysisFile = *m.AnalysisFilePath
	}
	var totalCodeLines int64
	if m.RawTotalCodeLines != nil {
		totalCodeLines = *m.RawTotalCodeLines
	}

	return buildEfficiencyResponse("work_dir", m.ProjectID, formatDateYMD(m.AnalysisDate),
		rawDays, m.CorrectedAIEstimatedDays,
		totalLeadTimeMs, totalProcessTimeMs,
		userCount, startTime, endTime,
		nil, apiCost, analysisFile,
		nil, "", 0,
		totalCodeLines)
}

// buildResponseFromRepoMetrics 从 PG RepoMetrics 构建响应
func buildResponseFromRepoMetrics(m *RepoMetrics) gin.H {
	var rawDays, apiCost float64
	var totalLeadTimeMs, totalProcessTimeMs int64
	var startTime, endTime time.Time
	var analysisFile string

	if m.RawAIEstimatedDaysFinal != nil {
		rawDays = *m.RawAIEstimatedDaysFinal
	}
	if m.APICost != nil {
		apiCost = *m.APICost
	}
	if m.TotalLeadTimeMs != nil {
		totalLeadTimeMs = *m.TotalLeadTimeMs
	}
	if m.TotalProcessTimeMs != nil {
		totalProcessTimeMs = *m.TotalProcessTimeMs
	}
	if m.ActualStartTime != nil {
		startTime = *m.ActualStartTime
	}
	if m.ActualEndTime != nil {
		endTime = *m.ActualEndTime
	}
	if m.AnalysisFilePath != nil {
		analysisFile = *m.AnalysisFilePath
	}
	var totalCodeLines int64
	if m.OurAICodeLines != nil {
		totalCodeLines += *m.OurAICodeLines
	}
	if m.HumanCodeLines != nil {
		totalCodeLines += *m.HumanCodeLines
	}
	if m.AIOtherCodeLines != nil {
		totalCodeLines += *m.AIOtherCodeLines
	}
	if m.UnknownCodeLines != nil {
		totalCodeLines += *m.UnknownCodeLines
	}

	return buildEfficiencyResponse("repo", m.RepoID, formatDateYMD(m.AnalysisDate),
		rawDays, m.CorrectedAIEstimatedDays,
		totalLeadTimeMs, totalProcessTimeMs,
		0, startTime, endTime,
		nil, apiCost, analysisFile,
		nil, "", 0,
		totalCodeLines)
}

// --- Handler ---

// getEfficiency GET /api/analysis/efficiency
func getEfficiency(c *gin.Context) {
	dimension := c.Query("dimension")
	id := c.Query("id")
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")

	if dimension != "work_dir" && dimension != "repo" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "dimension 必须是 work_dir 或 repo"})
		return
	}
	if id == "" || startDate == "" || endDate == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id, startDate, endDate 为必填参数"})
		return
	}

	startDateFmt, err := parseDateParam(startDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "startDate 格式错误，需要 YYYYMMDD"})
		return
	}
	endDateFmt, err := parseDateParam(endDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "endDate 格式错误，需要 YYYYMMDD"})
		return
	}

	analysisDate := formatDateYMD(time.Now())
	startDateStr := formatDateYMD(startDateFmt)
	endDateStr := formatDateYMD(endDateFmt)

	// 查 PG 缓存
	if dimension == "work_dir" {
		m, err := GetProjectMetrics(db, id, analysisDate, startDateStr, endDateStr)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if m != nil {
			c.JSON(http.StatusOK, buildResponseFromProjectMetrics(m))
			return
		}
	} else {
		m, err := GetRepoMetrics(db, id, analysisDate, startDateStr, endDateStr)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if m != nil {
			c.JSON(http.StatusOK, buildResponseFromRepoMetrics(m))
			return
		}
	}

	// 实时从 ES 计算
	ar, err := computeFromES(dimension, id, startDate, endDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if ar == nil {
		c.JSON(http.StatusOK, buildEfficiencyResponse(dimension, id, analysisDate,
			0, nil, 0, 0, 0, time.Time{}, time.Time{}, nil, 0, "",
			nil, "", 0,
			0))
		return
	}

	// AI 综合评估提效比例
	var efficiencyReason string
	var aiRatioProcess float64
	aiResult, aiErr := callAIForEfficiencyAssessment(
		ar.AIEstimatedDays,
		ar.TotalProcessTimeMs, ar.TotalLeadTimeMs,
		ar.TotalCodeLines, ar.TaskCount, ar.TaskCount,
		ar.AIEstimatedReasons,
	)
	if aiErr == nil && aiResult != nil {
		ar.EfficiencyReason = aiResult.EfficiencyReason
		efficiencyReason = aiResult.EfficiencyReason
		aiRatioProcess = aiResult.EfficiencyRatio * 100
	} else {
		log.Printf("GET 提效分析 AI 评估失败: %v", aiErr)
	}

	resp := buildEfficiencyResponse(dimension, id, analysisDate,
		ar.AIEstimatedDays, nil,
		ar.TotalLeadTimeMs, ar.TotalProcessTimeMs,
		ar.UserCount, ar.StartTime, ar.EndTime,
		ar.Users, ar.APICost, "",
		ar.AIEstimatedReasons, efficiencyReason, aiRatioProcess,
		ar.TotalCodeLines)
	c.JSON(http.StatusOK, resp)
}

// calculateEfficiency POST /api/analysis/efficiency/calculate
func calculateEfficiency(c *gin.Context) {
	var req struct {
		Dimension string `json:"dimension"`
		ID        string `json:"id"`
		StartDate string `json:"startDate"`
		EndDate   string `json:"endDate"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数解析失败"})
		return
	}

	if req.Dimension != "work_dir" && req.Dimension != "repo" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "dimension 必须是 work_dir 或 repo"})
		return
	}
	if req.ID == "" || req.StartDate == "" || req.EndDate == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id, startDate, endDate 为必填参数"})
		return
	}

	startDateFmt, err := parseDateParam(req.StartDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "startDate 格式错误，需要 YYYYMMDD"})
		return
	}
	endDateFmt, err := parseDateParam(req.EndDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "endDate 格式错误，需要 YYYYMMDD"})
		return
	}

	analysisDate := time.Now()
	analysisDateStr := formatDateYMD(analysisDate)

	// 强制重新计算
	ar, err := computeFromES(req.Dimension, req.ID, req.StartDate, req.EndDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if ar == nil {
		c.JSON(http.StatusOK, buildEfficiencyResponse(req.Dimension, req.ID, analysisDateStr,
			0, nil, 0, 0, 0, time.Time{}, time.Time{}, nil, 0, "",
			nil, "", 0,
			0))
		return
	}

	// 计算提效比例 - 优先使用AI综合评估
	dailyRate := DefaultDailyRate
	var ratioLead, ratioProcess float64
	var efficiencyReason string
	var aiRatioProcess float64

	aiResult, aiErr := callAIForEfficiencyAssessment(
		ar.AIEstimatedDays,
		ar.TotalProcessTimeMs, ar.TotalLeadTimeMs,
		ar.TotalCodeLines, ar.TaskCount, ar.TaskCount,
		ar.AIEstimatedReasons,
	)

	if aiErr == nil && aiResult != nil {
		aiRatioProcess = aiResult.EfficiencyRatio * 100
		ratioProcess = aiRatioProcess
		efficiencyReason = aiResult.EfficiencyReason
		if ar.TotalLeadTimeMs > 0 && ar.TotalProcessTimeMs > 0 {
			ratioLead = ratioProcess * float64(ar.TotalProcessTimeMs) / float64(ar.TotalLeadTimeMs)
		}
		log.Printf("AI综合评估提效比例: %.1f%%，理由: %s", ratioProcess, efficiencyReason)
	} else {
		if aiErr != nil {
			log.Printf("AI提效评估失败，降级为简单计算: %v", aiErr)
		}
		if ar.TotalLeadTimeMs > 0 {
			ratioLead = ar.AIEstimatedDays / (float64(ar.TotalLeadTimeMs) / float64(MsPerWorkDay)) * 100
		}
		if ar.TotalProcessTimeMs > 0 {
			ratioProcess = ar.AIEstimatedDays / (float64(ar.TotalProcessTimeMs) / float64(MsPerWorkDay)) * 100
		}
		ratioLead = math.Max(10, math.Min(10000, ratioLead))
		ratioProcess = math.Max(10, math.Min(10000, ratioProcess))
		efficiencyReason = "AI评估失败，使用简单计算（已限制范围）"
	}
	costSaving := ar.AIEstimatedDays*dailyRate - ar.APICost
	var roi float64
	if ar.APICost > 0 {
		roi = costSaving / ar.APICost * 100
	}

	// 生成分析文件
	safeID := makeSafeID(req.ID)
	dirName := analysisDate.Format("2006-01") + "/analysis"
	analysisDir := filepath.Join(appConfig.RawDataDir, dirName)
	if err := os.MkdirAll(analysisDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("创建分析目录失败: %v", err)})
		return
	}

	fileName := fmt.Sprintf("%s_%s_%s.json", req.Dimension, safeID, analysisDate.Format("20060102"))
	analysisFilePath := filepath.Join(analysisDir, fileName)

	resp := buildEfficiencyResponse(req.Dimension, req.ID, analysisDateStr,
		ar.AIEstimatedDays, nil,
		ar.TotalLeadTimeMs, ar.TotalProcessTimeMs,
		ar.UserCount, ar.StartTime, ar.EndTime,
		ar.Users, ar.APICost, analysisFilePath,
		ar.AIEstimatedReasons, ar.EfficiencyReason, aiRatioProcess,
		ar.TotalCodeLines)

	fileData, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("序列化分析结果失败: %v", err)})
		return
	}
	if err := os.WriteFile(analysisFilePath, fileData, 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("写入分析文件失败: %v", err)})
		return
	}

	// 写入 PG
	if req.Dimension == "work_dir" {
		m := &ProjectMetrics{
			ProjectID:              req.ID,
			AnalysisDate:           analysisDate,
			QueryStartDate:         startDateFmt,
			QueryEndDate:           endDateFmt,
			RawAIEstimatedDays:     ptrFloat64(ar.AIEstimatedDays),
			RawTotalCost:           ptrFloat64(ar.APICost),
			RawTaskCount:           ptrInt(ar.TaskCount),
			ActualStartTime:        ptrTime(ar.StartTime),
			ActualEndTime:          ptrTime(ar.EndTime),
			TotalLeadTimeMs:        ptrInt64(ar.TotalLeadTimeMs),
			TotalProcessTimeMs:     ptrInt64(ar.TotalProcessTimeMs),
			UserCount:              ptrInt(ar.UserCount),
			EfficiencyRatioLead:    ptrFloat64(ratioLead),
			EfficiencyRatioProcess: ptrFloat64(ratioProcess),
			APICost:                ptrFloat64(ar.APICost),
			DailyRate:              ptrFloat64(dailyRate),
			CostSaving:             ptrFloat64(costSaving),
			ROI:                    ptrFloat64(roi),
			AnalysisFilePath:       ptrString(analysisFilePath),
		}
		if err := UpsertProjectMetrics(db, m); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	} else {
		m := &RepoMetrics{
			RepoID:                     req.ID,
			AnalysisDate:               analysisDate,
			QueryStartDate:             startDateFmt,
			QueryEndDate:               endDateFmt,
			RawAIEstimatedDaysFromTask: ptrFloat64(ar.AIEstimatedDays),
			RawAIEstimatedDaysFinal:    ptrFloat64(ar.AIEstimatedDays),
			ActualStartTime:            ptrTime(ar.StartTime),
			ActualEndTime:              ptrTime(ar.EndTime),
			TotalLeadTimeMs:            ptrInt64(ar.TotalLeadTimeMs),
			TotalProcessTimeMs:         ptrInt64(ar.TotalProcessTimeMs),
			EfficiencyRatioLead:        ptrFloat64(ratioLead),
			EfficiencyRatioProcess:     ptrFloat64(ratioProcess),
			APICost:                    ptrFloat64(ar.APICost),
			DailyRate:                  ptrFloat64(dailyRate),
			CostSaving:                 ptrFloat64(costSaving),
			ROI:                        ptrFloat64(roi),
			AnalysisFilePath:           ptrString(analysisFilePath),
		}
		if err := UpsertRepoMetrics(db, m); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, resp)
}

// CorrectionRequest 纠正请求
type CorrectionRequest struct {
	Dimension string  `json:"dimension"`
	ID        string  `json:"id"`
	StartDate string  `json:"startDate"`
	EndDate   string  `json:"endDate"`
	Field     string  `json:"field"`
	Value     float64 `json:"value"`
	Reason    string  `json:"reason"`
	By        string  `json:"by"`
}

// correctEfficiency PUT /api/analysis/efficiency/correct
func correctEfficiency(c *gin.Context) {
	var req CorrectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数解析失败"})
		return
	}

	if req.Field != "ai_estimated_days" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "field 仅支持 ai_estimated_days"})
		return
	}
	if strings.TrimSpace(req.Reason) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "reason 不能为空"})
		return
	}
	if strings.TrimSpace(req.By) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "by 不能为空"})
		return
	}
	if req.Dimension != "work_dir" && req.Dimension != "repo" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "dimension 必须是 work_dir 或 repo"})
		return
	}

	startDateFmt, err := parseDateParam(req.StartDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "startDate 格式错误，需要 YYYYMMDD"})
		return
	}
	endDateFmt, err := parseDateParam(req.EndDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "endDate 格式错误，需要 YYYYMMDD"})
		return
	}

	analysisDate := formatDateYMD(time.Now())
	startDateStr := formatDateYMD(startDateFmt)
	endDateStr := formatDateYMD(endDateFmt)
	now := time.Now()

	if req.Dimension == "work_dir" {
		m, err := GetProjectMetrics(db, req.ID, analysisDate, startDateStr, endDateStr)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if m == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "未找到对应的分析记录"})
			return
		}

		// 记录旧值
		var oldValue string
		if m.CorrectedAIEstimatedDays != nil {
			oldValue = fmt.Sprintf("%.1f", *m.CorrectedAIEstimatedDays)
		} else if m.RawAIEstimatedDays != nil {
			oldValue = fmt.Sprintf("%.1f", *m.RawAIEstimatedDays)
		}

		// 更新
		m.CorrectedAIEstimatedDays = ptrFloat64(req.Value)
		m.CorrectionReason = ptrString(req.Reason)
		m.CorrectedBy = ptrString(req.By)
		m.CorrectedAt = ptrTime(now)

		// 重新计算提效比例
		if m.TotalLeadTimeMs != nil && *m.TotalLeadTimeMs > 0 {
			m.EfficiencyRatioLead = ptrFloat64(req.Value / (float64(*m.TotalLeadTimeMs) / float64(MsPerWorkDay)) * 100)
		}
		if m.TotalProcessTimeMs != nil && *m.TotalProcessTimeMs > 0 {
			m.EfficiencyRatioProcess = ptrFloat64(req.Value / (float64(*m.TotalProcessTimeMs) / float64(MsPerWorkDay)) * 100)
		}

		if err := UpsertProjectMetrics(db, m); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// 写入纠正历史
		h := &CorrectionHistory{
			Dimension:    "work_dir",
			DimensionID:  req.ID,
			AnalysisDate: m.AnalysisDate,
			FieldName:    req.Field,
			OldValue:     ptrString(oldValue),
			NewValue:     ptrString(fmt.Sprintf("%.1f", req.Value)),
			Reason:       ptrString(req.Reason),
			CorrectedBy:  ptrString(req.By),
		}
		if err := InsertCorrectionHistory(db, h); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, buildResponseFromProjectMetrics(m))
	} else {
		m, err := GetRepoMetrics(db, req.ID, analysisDate, startDateStr, endDateStr)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if m == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "未找到对应的分析记录"})
			return
		}

		var oldValue string
		if m.CorrectedAIEstimatedDays != nil {
			oldValue = fmt.Sprintf("%.1f", *m.CorrectedAIEstimatedDays)
		} else if m.RawAIEstimatedDaysFinal != nil {
			oldValue = fmt.Sprintf("%.1f", *m.RawAIEstimatedDaysFinal)
		}

		m.CorrectedAIEstimatedDays = ptrFloat64(req.Value)
		m.CorrectionReason = ptrString(req.Reason)
		m.CorrectedBy = ptrString(req.By)
		m.CorrectedAt = ptrTime(now)

		if m.TotalLeadTimeMs != nil && *m.TotalLeadTimeMs > 0 {
			m.EfficiencyRatioLead = ptrFloat64(req.Value / (float64(*m.TotalLeadTimeMs) / float64(MsPerWorkDay)) * 100)
		}
		if m.TotalProcessTimeMs != nil && *m.TotalProcessTimeMs > 0 {
			m.EfficiencyRatioProcess = ptrFloat64(req.Value / (float64(*m.TotalProcessTimeMs) / float64(MsPerWorkDay)) * 100)
		}

		if err := UpsertRepoMetrics(db, m); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		h := &CorrectionHistory{
			Dimension:    "repo",
			DimensionID:  req.ID,
			AnalysisDate: m.AnalysisDate,
			FieldName:    req.Field,
			OldValue:     ptrString(oldValue),
			NewValue:     ptrString(fmt.Sprintf("%.1f", req.Value)),
			Reason:       ptrString(req.Reason),
			CorrectedBy:  ptrString(req.By),
		}
		if err := InsertCorrectionHistory(db, h); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, buildResponseFromRepoMetrics(m))
	}
}

// getEfficiencyHistory GET /api/analysis/efficiency/history
func getEfficiencyHistory(c *gin.Context) {
	dimension := c.Query("dimension")
	id := c.Query("id")

	if dimension == "" || id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "dimension 和 id 为必填参数"})
		return
	}

	list, err := ListCorrectionHistory(db, dimension, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	items := make([]gin.H, 0, len(list))
	for _, h := range list {
		item := gin.H{
			"field_name": h.FieldName,
		}
		if h.OldValue != nil {
			item["old_value"] = *h.OldValue
		}
		if h.NewValue != nil {
			item["new_value"] = *h.NewValue
		}
		if h.Reason != nil {
			item["reason"] = *h.Reason
		}
		if h.CorrectedBy != nil {
			item["corrected_by"] = *h.CorrectedBy
		}
		if h.CorrectedAt != nil {
			item["corrected_at"] = h.CorrectedAt.Format(time.RFC3339)
		}
		items = append(items, item)
	}

	c.JSON(http.StatusOK, gin.H{"items": items})
}

// getEfficiencyFile GET /api/analysis/efficiency/file
func getEfficiencyFile(c *gin.Context) {
	dimension := c.Query("dimension")
	id := c.Query("id")
	date := c.Query("date")

	if dimension == "" || id == "" || date == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "dimension, id, date 为必填参数"})
		return
	}

	dateParsed, err := parseDateParam(date)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date 格式错误，需要 YYYYMMDD"})
		return
	}

	safeID := makeSafeID(id)
	dirName := dateParsed.Format("2006-01") + "/analysis"
	fileName := fmt.Sprintf("%s_%s_%s.json", dimension, safeID, date)
	filePath := filepath.Join(appConfig.RawDataDir, dirName, fileName)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "分析文件不存在"})
		return
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("读取分析文件失败: %v", err)})
		return
	}

	c.Data(http.StatusOK, "application/json", data)
}

// updateUserManualDays PUT /api/analysis/efficiency/manual-days
func updateUserManualDays(c *gin.Context) {
	var req struct {
		Dimension string  `json:"dimension"`
		ID        string  `json:"id"`
		StartDate string  `json:"startDate"`
		EndDate   string  `json:"endDate"`
		Value     float64 `json:"value"`
		Reason    string  `json:"reason"`
		By        string  `json:"by"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数解析失败"})
		return
	}

	if req.Dimension != "work_dir" && req.Dimension != "repo" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "dimension 必须是 work_dir 或 repo"})
		return
	}
	if req.ID == "" || req.StartDate == "" || req.EndDate == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id, startDate, endDate 为必填参数"})
		return
	}
	if strings.TrimSpace(req.Reason) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "reason 不能为空"})
		return
	}
	if strings.TrimSpace(req.By) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "by 不能为空"})
		return
	}

	startDateFmt, err := parseDateParam(req.StartDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "startDate 格式错误，需要 YYYYMMDD"})
		return
	}
	endDateFmt, err := parseDateParam(req.EndDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "endDate 格式错误，需要 YYYYMMDD"})
		return
	}

	analysisDate := formatDateYMD(time.Now())
	if err := UpdateUserManualDays(db, req.Dimension, req.ID, analysisDate,
		formatDateYMD(startDateFmt), formatDateYMD(endDateFmt),
		req.Value, req.Reason, req.By); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":          "ok",
		"user_manual_days": req.Value,
	})
}
