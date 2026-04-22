package main

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type RawDataHit struct {
	ID        string                 `json:"_id"`
	Source    map[string]interface{} `json:"_source"`
	Score     float64                `json:"_score"`
	Index     string                 `json:"_index"`
	Type      string                 `json:"_type"`
	Timestamp interface{}            `json:"@timestamp,omitempty"`
}

type RawDataResponse struct {
	Total    int64        `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"pageSize"`
	Hits     []RawDataHit `json:"hits"`
}

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

// getRawData 查询原始请求数据
// @Summary 查询原始请求数据
// @Description 从Elasticsearch中查询原始请求数据列表
// @Tags ES
// @Produce json
// @Param startDate query string true "开始日期(YYYYMMDD)"
// @Param endDate query string false "结束日期(YYYYMMDD)"
// @Param page query int false "页码" default(1)
// @Param pageSize query int false "每页数量" default(20)
// @Param sortField query string false "排序字段" default(@timestamp)
// @Param sortOrder query string false "排序方向" default(desc)
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

	hits := make([]RawDataHit, len(result.Hits))
	for i, h := range result.Hits {
		hits[i] = RawDataHit{
			Source: h,
		}
	}
	c.JSON(http.StatusOK, RawDataResponse{Total: int64(result.Total), Page: page, PageSize: pageSize, Hits: hits})
}
