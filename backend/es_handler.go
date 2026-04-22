package main

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func getDefaultInt(c *gin.Context, key string, defaultVal int) int {
	str := c.Query(key)
	if str == "" {
		return defaultVal
	}
	var val int
	if _, err := fmt.Sscanf(str, "%d", &val); err != nil {
		return defaultVal
	}
	return val
}

func getDefaultString(c *gin.Context, key, defaultVal string) string {
	val := c.Query(key)
	if val == "" {
		return defaultVal
	}
	return val
}

// --- Handler ---

// getIndices 获取 ES 索引列表，区分 request/task 索引
// @Summary 获取ES索引列表
// @Description 获取Elasticsearch中匹配的索引列表，区分request和task索引
// @Tags ES
// @Produce json
// @Success 200 {object} IndicesResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/indices [get]
func getIndices(c *gin.Context) {
	indices, err := esClient.GetIndices(ESIndexPattern)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	var requestIndices, taskIndices []IndexInfo
	for _, idx := range indices {
		if strings.Contains(idx.Name, "_request_") {
			requestIndices = append(requestIndices, idx)
		} else if strings.Contains(idx.Name, "_task_") {
			taskIndices = append(taskIndices, idx)
		}
	}

	c.JSON(http.StatusOK, IndicesResponse{Request: requestIndices, Task: taskIndices})
}

// getRawData 查询原始请求数据
// @Summary 查询原始请求数据
// @Description 从Elasticsearch中查询原始请求数据列表
// @Tags ES
// @Produce json
// @Param startDate query string true "开始日期(YYYYMMDD)"
// @Param endDate query string false "结束日期(YYYYMMDD)"
// @Success 200 {object} RawDataResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/requests [get]
func getRawData(c *gin.Context) {
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	if startDate == "" || endDate == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "startDate 和 endDate 为必填参数"})
		return
	}

	page := getDefaultInt(c, "page", 1)
	pageSize := getDefaultInt(c, "pageSize", DefaultPageSize)
	sortField := getDefaultString(c, "sortField", "@timestamp")
	sortOrder := getDefaultString(c, "sortOrder", "desc")

	indexNames, err := generateIndexNames(ESRequestIndexPrefix, startDate, endDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	query := map[string]interface{}{
		"match_all": map[string]interface{}{},
	}

	result, err := esClient.Search(indexNames, query, (page-1)*pageSize, pageSize, sortField, sortOrder)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	hits := make([]interface{}, len(result.Hits))
	for i, h := range result.Hits {
		hits[i] = h
	}
	c.JSON(http.StatusOK, RawDataResponse{Total: int64(result.Total), Page: page, PageSize: pageSize, Hits: hits})
}
