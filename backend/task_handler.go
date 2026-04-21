package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// getTasks 查询 Task 列表
func getTasks(c *gin.Context) {
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	if startDate == "" || endDate == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "startDate 和 endDate 为必填参数"})
		return
	}

	page := getDefaultInt(c, "page", 1)
	pageSize := getDefaultInt(c, "pageSize", DefaultPageSize)

	indexNames, err := generateIndexNames(ESTaskIndexPrefix, startDate, endDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total":    result.Total,
		"page":     page,
		"pageSize": pageSize,
		"hits":     result.Hits,
	})
}

// getTasksSummary 获取 Task 汇总统计
func getTasksSummary(c *gin.Context) {
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	if startDate == "" || endDate == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "startDate 和 endDate 为必填参数"})
		return
	}

	indexNames, err := generateIndexNames(ESTaskIndexPrefix, startDate, endDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	getValue := func(key string) interface{} {
		if agg, ok := aggregations[key].(map[string]interface{}); ok {
			return agg["value"]
		}
		return 0
	}

	c.JSON(http.StatusOK, gin.H{
		"task_count":             getValue("task_count"),
		"total_api_count":       getValue("total_api_count"),
		"total_api_cost":        getValue("total_api_cost"),
		"total_ai_estimated_days": getValue("total_ai_estimated_days"),
	})
}
