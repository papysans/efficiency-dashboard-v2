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

// getIndices ?? ES ?????? request/task ??
// @Summary ??ES????
// @Description ??Elasticsearch???????request/task??
// @Tags ES
// @Produce json
// @Success 200 {object} object
// @Router /api/indices [get]
func getIndices(c *gin.Context) {
	indices, err := esClient.GetIndices(ESIndexPattern)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

	c.JSON(http.StatusOK, gin.H{
		"request": requestIndices,
		"task":    taskIndices,
	})
}

// getRawData ????????
// @Summary ????????
// @Description ?Elasticsearch????????
// @Tags ES
// @Produce json
// @Param startDate query string true "????"
// @Param endDate query string false "????"
// @Success 200 {object} object
// @Router /api/requests [get]
func getRawData(c *gin.Context) {
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	if startDate == "" || endDate == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "startDate ? endDate ?????"})
		return
	}

	page := getDefaultInt(c, "page", 1)
	pageSize := getDefaultInt(c, "pageSize", DefaultPageSize)
	sortField := getDefaultString(c, "sortField", "@timestamp")
	sortOrder := getDefaultString(c, "sortOrder", "desc")

	indexNames, err := generateIndexNames(ESRequestIndexPrefix, startDate, endDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	query := map[string]interface{}{
		"match_all": map[string]interface{}{},
	}

	result, err := esClient.Search(indexNames, query, (page-1)*pageSize, pageSize, sortField, sortOrder)
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
