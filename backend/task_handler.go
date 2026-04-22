package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// getTasks 查询 Task 列表
// @Summary 查询任务列表
// @Description 按日期范围查询任务列表，支持按任务ID、项目ID、用户ID过滤
// @Tags Tasks(v1)
// @Produce json
// @Param startDate query string true "开始日期(YYYYMMDD)"
// @Param endDate query string true "结束日期(YYYYMMDD)"
// @Param taskId query string false "任务ID"
// @Param projectId query string false "项目ID"
// @Param userId query string false "用户ID"
// @Param page query int false "页码" default(1)
// @Param pageSize query int false "每页数量" default(20)
// @Success 200 {object} TasksV1Response
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /tasks [get]
func getTasks(c *gin.Context) {
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	if startDate == "" || endDate == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "startDate 和 endDate 为必填参数"})
		return
	}

	page := getDefaultInt(c, "page", 1)
	pageSize := getDefaultInt(c, "pageSize", DefaultPageSize)

	indexNames, err := generateIndexNames(ESTaskIndexPrefix, startDate, endDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	filters := make(map[string]interface{})
	for _, key := range []string{"taskId", "projectId", "userId"} {
		if val := c.Query(key); val != "" {
			filters[key] = val
		}
	}

	result, err := esClient.SearchWithFilter(indexNames, filters, (page-1)*pageSize, pageSize, "@timestamp", "desc")
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	hits := make([]interface{}, len(result.Hits))
	for i, h := range result.Hits {
		hits[i] = h
	}
	c.JSON(http.StatusOK, TasksV1Response{
		Total:    int64(result.Total),
		Page:     page,
		PageSize: pageSize,
		Hits:     hits,
	})
}

// getTasksSummary 获取 Task 汇总统计
// @Summary 获取任务汇总统计
// @Description 按日期范围统计任务数量、API调用次数、API费用、AI估算天数
// @Tags Tasks(v1)
// @Produce json
// @Param startDate query string true "开始日期(YYYYMMDD)"
// @Param endDate query string true "结束日期(YYYYMMDD)"
// @Success 200 {object} TasksSummaryV1Response
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /tasks/summary [get]
func getTasksSummary(c *gin.Context) {
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	if startDate == "" || endDate == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "startDate 和 endDate 为必填参数"})
		return
	}

	indexNames, err := generateIndexNames(ESTaskIndexPrefix, startDate, endDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	aggsQuery := map[string]interface{}{
		"total_api_count": map[string]interface{}{
			"sum": map[string]interface{}{"field": "api_count"},
		},
		"total_api_cost": map[string]interface{}{
			"sum": map[string]interface{}{"field": "api_cost"},
		},
		"total_ai_estimated_days": map[string]interface{}{
			"sum": map[string]interface{}{"field": "ai_estimated_days"},
		},
		"task_count": map[string]interface{}{
			"value_count": map[string]interface{}{"field": "task_id"},
		},
	}

	aggregations, err := esClient.Aggregate(indexNames, aggsQuery)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	getValue := func(key string) float64 {
		if agg, ok := aggregations[key].(map[string]interface{}); ok {
			if v, ok := agg["value"].(float64); ok {
				return v
			}
		}
		return 0
	}

	c.JSON(http.StatusOK, TasksSummaryV1Response{
		TaskCount:      getValue("task_count"),
		TotalAPICount:  getValue("total_api_count"),
		TotalAPICost:   getValue("total_api_cost"),
		TotalAIEstDays: getValue("total_ai_estimated_days"),
	})
}
