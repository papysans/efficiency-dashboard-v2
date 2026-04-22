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
	totalCodeLines int64) EfficiencyResponse {

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
	userList := make([]ActualTimeUser, 0, len(users))
	for _, u := range users {
		userList = append(userList, ActualTimeUser{
			UserID:        u.UserID,
			UserName:      u.UserName,
			StartTime:     u.StartTime.Format(time.RFC3339),
			EndTime:       u.EndTime.Format(time.RFC3339),
			LeadTimeMs:    u.LeadTimeMs,
			ProcessTimeMs: u.ProcessTime,
		})
	}

	// 处理 reasons
	aiReasons := reasons
	if aiReasons == nil {
		aiReasons = []string{}
	}

	return EfficiencyResponse{
		Dimension:    dimension,
		DimensionID:  dimensionID,
		AnalysisDate: analysisDate,
		AIEstimated: AIEstimatedInfo{
			RawDays:       rawDays,
			CorrectedDays: correctedDays,
			IsCorrected:   correctedDays != nil,
			Reasons:       aiReasons,
		},
		ActualTime: ActualTimeInfo{
			TotalLeadTimeMs:    totalLeadTimeMs,
			TotalProcessTimeMs: totalProcessTimeMs,
			TotalCodeLines:     totalCodeLines,
			UserCount:          userCount,
			StartTime:          startTime.Format(time.RFC3339),
			EndTime:            endTime.Format(time.RFC3339),
			Users:              userList,
		},
		Efficiency: EfficiencyInfo{
			RatioLead:    ratioLead,
			RatioProcess: ratioProcess,
			Reason:       efficiencyReason,
		},
		Cost: CostInfo{
			APICost:    apiCost,
			DailyRate:  dailyRate,
			CostSaving: costSaving,
			ROI:        roi,
		},
		AnalysisFile: analysisFile,
	}
}

// buildResponseFromProjectMetrics 从 PG ProjectMetrics 构建响应
func buildResponseFromProjectMetrics(m *ProjectMetrics) EfficiencyResponse {
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
func buildResponseFromRepoMetrics(m *RepoMetrics) EfficiencyResponse {
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
// @Summary 获取提效分析结果
// @Description 根据维度和ID查询提效分析结果，优先从PG缓存获取，无缓存时从ES实时计算
// @Tags Efficiency
// @Produce json
// @Param dimension query string true "维度(work_dir或repo)"
// @Param id query string true "维度ID"
// @Param startDate query string true "开始日期(YYYYMMDD)"
// @Param endDate query string true "结束日期(YYYYMMDD)"
// @Success 200 {object} EfficiencyResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /analysis/efficiency [get]
func getEfficiency(c *gin.Context) {
	dimension := c.Query("dimension")
	id := c.Query("id")
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")

	if dimension != "work_dir" && dimension != "repo" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "dimension 必须是 work_dir 或 repo"})
		return
	}
	if id == "" || startDate == "" || endDate == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "id, startDate, endDate 为必填参数"})
		return
	}

	startDateFmt, err := parseDateParam(startDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "startDate 格式错误，需要 YYYYMMDD"})
		return
	}
	endDateFmt, err := parseDateParam(endDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "endDate 格式错误，需要 YYYYMMDD"})
		return
	}

	analysisDate := formatDateYMD(time.Now())
	startDateStr := formatDateYMD(startDateFmt)
	endDateStr := formatDateYMD(endDateFmt)

	// 查 PG 缓存
	if dimension == "work_dir" {
		m, err := GetProjectMetrics(db, id, analysisDate, startDateStr, endDateStr)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			return
		}
		if m != nil {
			c.JSON(http.StatusOK, buildResponseFromProjectMetrics(m))
			return
		}
	} else {
		m, err := GetRepoMetrics(db, id, analysisDate, startDateStr, endDateStr)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
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
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
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
// @Summary 计算提效分析
// @Description 强制重新计算指定维度和ID的提效分析结果，包含AI综合评估，并持久化到PG
// @Tags Efficiency
// @Accept json
// @Produce json
// @Param data body CalculateEfficiencyRequest true "计算参数"
// @Success 200 {object} EfficiencyResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /analysis/efficiency/calculate [post]
func calculateEfficiency(c *gin.Context) {
	var req CalculateEfficiencyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "请求参数解析失败"})
		return
	}

	if req.Dimension != "work_dir" && req.Dimension != "repo" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "dimension 必须是 work_dir 或 repo"})
		return
	}
	if req.ID == "" || req.StartDate == "" || req.EndDate == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "id, startDate, endDate 为必填参数"})
		return
	}

	startDateFmt, err := parseDateParam(req.StartDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "startDate 格式错误，需要 YYYYMMDD"})
		return
	}
	endDateFmt, err := parseDateParam(req.EndDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "endDate 格式错误，需要 YYYYMMDD"})
		return
	}

	analysisDate := time.Now()
	analysisDateStr := formatDateYMD(analysisDate)

	// 强制重新计算
	ar, err := computeFromES(req.Dimension, req.ID, req.StartDate, req.EndDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
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
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: fmt.Sprintf("创建分析目录失败: %v", err)})
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
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: fmt.Sprintf("序列化分析结果失败: %v", err)})
		return
	}
	if err := os.WriteFile(analysisFilePath, fileData, 0644); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: fmt.Sprintf("写入分析文件失败: %v", err)})
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
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
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
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
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
// @Summary 纠正提效分析数据
// @Description 纠正AI估算天数等字段，记录纠正历史，并重新计算提效比例
// @Tags Efficiency
// @Accept json
// @Produce json
// @Param data body CorrectionRequest true "纠正请求"
// @Success 200 {object} EfficiencyResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /analysis/efficiency/correct [put]
func correctEfficiency(c *gin.Context) {
	var req CorrectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "请求参数解析失败"})
		return
	}

	if req.Field != "ai_estimated_days" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "field 仅支持 ai_estimated_days"})
		return
	}
	if strings.TrimSpace(req.Reason) == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "reason 不能为空"})
		return
	}
	if strings.TrimSpace(req.By) == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "by 不能为空"})
		return
	}
	if req.Dimension != "work_dir" && req.Dimension != "repo" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "dimension 必须是 work_dir 或 repo"})
		return
	}

	startDateFmt, err := parseDateParam(req.StartDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "startDate 格式错误，需要 YYYYMMDD"})
		return
	}
	endDateFmt, err := parseDateParam(req.EndDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "endDate 格式错误，需要 YYYYMMDD"})
		return
	}

	analysisDate := formatDateYMD(time.Now())
	startDateStr := formatDateYMD(startDateFmt)
	endDateStr := formatDateYMD(endDateFmt)
	now := time.Now()

	if req.Dimension == "work_dir" {
		m, err := GetProjectMetrics(db, req.ID, analysisDate, startDateStr, endDateStr)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			return
		}
		if m == nil {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "未找到对应的分析记录"})
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
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
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
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			return
		}

		c.JSON(http.StatusOK, buildResponseFromProjectMetrics(m))
	} else {
		m, err := GetRepoMetrics(db, req.ID, analysisDate, startDateStr, endDateStr)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			return
		}
		if m == nil {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "未找到对应的分析记录"})
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
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
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
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			return
		}

		c.JSON(http.StatusOK, buildResponseFromRepoMetrics(m))
	}
}

// getEfficiencyHistory GET /api/analysis/efficiency/history
// @Summary 获取纠正历史
// @Description 查询指定维度和ID的纠正历史记录
// @Tags Efficiency
// @Produce json
// @Param dimension query string true "维度(work_dir或repo)"
// @Param id query string true "维度ID"
// @Success 200 {object} EfficiencyHistoryResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /analysis/efficiency/history [get]
func getEfficiencyHistory(c *gin.Context) {
	dimension := c.Query("dimension")
	id := c.Query("id")

	if dimension == "" || id == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "dimension 和 id 为必填参数"})
		return
	}

	list, err := ListCorrectionHistory(db, dimension, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	items := make([]CorrectionHistoryItem, 0, len(list))
	for _, h := range list {
		item := CorrectionHistoryItem{
			FieldName: h.FieldName,
		}
		if h.OldValue != nil {
			item.OldValue = h.OldValue
		}
		if h.NewValue != nil {
			item.NewValue = h.NewValue
		}
		if h.Reason != nil {
			item.Reason = h.Reason
		}
		if h.CorrectedBy != nil {
			item.CorrectedBy = h.CorrectedBy
		}
		if h.CorrectedAt != nil {
			correctedAtStr := h.CorrectedAt.Format(time.RFC3339)
			item.CorrectedAt = &correctedAtStr
		}
		items = append(items, item)
	}

	c.JSON(http.StatusOK, EfficiencyHistoryResponse{Items: items})
}

// getEfficiencyFile GET /api/analysis/efficiency/file
// @Summary 获取提效分析文件
// @Description 根据维度、ID和日期获取提效分析的原始JSON文件内容
// @Tags Efficiency
// @Produce json
// @Param dimension query string true "维度(work_dir或repo)"
// @Param id query string true "维度ID"
// @Param date query string true "日期(YYYYMMDD)"
// @Success 200 {object} EfficiencyResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /analysis/efficiency/file [get]
func getEfficiencyFile(c *gin.Context) {
	dimension := c.Query("dimension")
	id := c.Query("id")
	date := c.Query("date")

	if dimension == "" || id == "" || date == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "dimension, id, date 为必填参数"})
		return
	}

	dateParsed, err := parseDateParam(date)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "date 格式错误，需要 YYYYMMDD"})
		return
	}

	safeID := makeSafeID(id)
	dirName := dateParsed.Format("2006-01") + "/analysis"
	fileName := fmt.Sprintf("%s_%s_%s.json", dimension, safeID, date)
	filePath := filepath.Join(appConfig.RawDataDir, dirName, fileName)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "分析文件不存在"})
		return
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: fmt.Sprintf("读取分析文件失败: %v", err)})
		return
	}

	c.Data(http.StatusOK, "application/json", data)
}

// updateUserManualDays PUT /api/analysis/efficiency/manual-days
// @Summary 更新用户人工天数
// @Description 更新指定维度和ID的用户人工天数，需提供原因和操作人
// @Tags Efficiency
// @Accept json
// @Produce json
// @Param data body UpdateManualDaysRequest true "更新参数"
// @Success 200 {object} ManualDaysResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /analysis/efficiency/manual-days [put]
func updateUserManualDays(c *gin.Context) {
	var req UpdateManualDaysRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "请求参数解析失败"})
		return
	}

	if req.Dimension != "work_dir" && req.Dimension != "repo" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "dimension 必须是 work_dir 或 repo"})
		return
	}
	if req.ID == "" || req.StartDate == "" || req.EndDate == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "id, startDate, endDate 为必填参数"})
		return
	}
	if strings.TrimSpace(req.Reason) == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "reason 不能为空"})
		return
	}
	if strings.TrimSpace(req.By) == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "by 不能为空"})
		return
	}

	startDateFmt, err := parseDateParam(req.StartDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "startDate 格式错误，需要 YYYYMMDD"})
		return
	}
	endDateFmt, err := parseDateParam(req.EndDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "endDate 格式错误，需要 YYYYMMDD"})
		return
	}

	analysisDate := formatDateYMD(time.Now())
	if err := UpdateUserManualDays(db, req.Dimension, req.ID, analysisDate,
		formatDateYMD(startDateFmt), formatDateYMD(endDateFmt),
		req.Value, req.Reason, req.By); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, ManualDaysResponse{
		Message:        "ok",
		UserManualDays: req.Value,
	})
}
