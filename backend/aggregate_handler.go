package main

import (
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// 维度到 ES 字段的映射（普通 terms）
var dimensionFieldMap = map[string]string{
	"work_dir": "project_id",
	"repo":     "repo_id",
	"user":     "user_id",
	"org1":     "org1",
}

// 维度到 ES script 的映射（script terms）
var dimensionScriptMap = map[string]string{
	"org2": "doc['org1'].value + '_' + doc['org2'].value",
	"org3": "doc['org1'].value + '_' + doc['org2'].value + '_' + doc['org3'].value",
	"org4": "doc['org1'].value + '_' + doc['org2'].value + '_' + doc['org3'].value + '_' + doc['org4'].value",
}

// script 维度对应的 org 字段列表，用于拆分 key 构建 term 查询
var scriptDimensionFields = map[string][]string{
	"org2": {"org1", "org2"},
	"org3": {"org1", "org2", "org3"},
	"org4": {"org1", "org2", "org3", "org4"},
}

var validDimensions = map[string]bool{
	"work_dir": true, "repo": true, "user": true,
	"org1": true, "org2": true, "org3": true, "org4": true,
}

// buildSubAggs 构建子聚合
func buildSubAggs() map[string]interface{} {
	return map[string]interface{}{
		"sum_user_in_chars":     map[string]interface{}{"sum": map[string]interface{}{"field": "user_in_chars"}},
		"sum_code_lines":        map[string]interface{}{"sum": map[string]interface{}{"field": "assistant_out_code_lines"}},
		"sum_api_count":         map[string]interface{}{"sum": map[string]interface{}{"field": "api_count"}},
		"sum_api_cost":          map[string]interface{}{"sum": map[string]interface{}{"field": "api_cost"}},
		"sum_api_in_tokens":     map[string]interface{}{"sum": map[string]interface{}{"field": "api_in_tokens"}},
		"sum_api_out_tokens":    map[string]interface{}{"sum": map[string]interface{}{"field": "api_out_tokens"}},
		"sum_ai_estimated_days": map[string]interface{}{"sum": map[string]interface{}{"field": "ai_estimated_days"}},
		"task_count":            map[string]interface{}{"value_count": map[string]interface{}{"field": "task_id"}},
		"min_start_time":        map[string]interface{}{"min": map[string]interface{}{"field": "api_request_time"}},
		"max_end_time":          map[string]interface{}{"max": map[string]interface{}{"field": "api_end_time"}},
	}
}

// buildDimensionAgg 根据维度构建 terms aggregation
func buildDimensionAgg(dimension string, size int) map[string]interface{} {
	subAggs := buildSubAggs()

	if esField, ok := dimensionFieldMap[dimension]; ok {
		return map[string]interface{}{
			"dimension_agg": map[string]interface{}{
				"terms": map[string]interface{}{"field": esField, "size": size},
				"aggs":  subAggs,
			},
		}
	}

	scriptSource := dimensionScriptMap[dimension]
	return map[string]interface{}{
		"dimension_agg": map[string]interface{}{
			"terms": map[string]interface{}{
				"script": map[string]interface{}{
					"source": scriptSource,
					"lang":   "painless",
				},
				"size": size,
			},
			"aggs": subAggs,
		},
	}
}

// getAggValue 从聚合 bucket 中提取子聚合的 value
func getAggValue(bucket map[string]interface{}, key string) float64 {
	if agg, ok := bucket[key].(map[string]interface{}); ok {
		if v, ok := agg["value"].(float64); ok {
			return v
		}
	}
	return 0
}

// getAggTimeString 从聚合 bucket 中提取时间字符串
func getAggTimeString(bucket map[string]interface{}, key string) string {
	if agg, ok := bucket[key].(map[string]interface{}); ok {
		if s, ok := agg["value_as_string"].(string); ok {
			return s
		}
		if v, ok := agg["value"].(float64); ok && !math.IsInf(v, 0) && !math.IsNaN(v) {
			t := time.UnixMilli(int64(v))
			return t.UTC().Format(time.RFC3339)
		}
	}
	return ""
}

// buildBucketQuery 为指定维度和 key 构建二次查询的 query
func buildBucketQuery(dimension string, key string) map[string]interface{} {
	if esField, ok := dimensionFieldMap[dimension]; ok {
		return map[string]interface{}{
			"bool": map[string]interface{}{
				"filter": []map[string]interface{}{
					{"term": map[string]interface{}{esField: key}},
				},
			},
		}
	}

	// script 维度：拆分 key 构建多个 term 条件
	fields := scriptDimensionFields[dimension]
	parts := strings.SplitN(key, "_", len(fields))
	var filters []map[string]interface{}
	for i, field := range fields {
		val := ""
		if i < len(parts) {
			val = parts[i]
		}
		filters = append(filters, map[string]interface{}{
			"term": map[string]interface{}{field: val},
		})
	}
	return map[string]interface{}{
		"bool": map[string]interface{}{
			"filter": filters,
		},
	}
}

// calcProcessTime 根据 task 列表计算合并后的 process_time（毫秒）
func calcProcessTime(hits []map[string]interface{}) float64 {
	if len(hits) == 0 {
		return 0
	}
	// 提取时间戳对 [start, end, start, end, ...]
	var timestamps []float64
	for _, hit := range hits {
		start, ok1 := hit["api_request_time"].(float64)
		end, ok2 := hit["api_end_time"].(float64)
		if ok1 && ok2 {
			timestamps = append(timestamps, start, end)
		}
	}
	return calcProcessTimeMs(timestamps)
}

// getAggregate 维度聚合查询
func getAggregate(c *gin.Context) {
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	dimension := c.Query("dimension")
	if startDate == "" || endDate == "" || dimension == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "startDate, endDate 和 dimension 为必填参数"})
		return
	}
	if !validDimensions[dimension] {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("无效的 dimension: %s，有效值为 work_dir, repo, user, org1, org2, org3, org4", dimension)})
		return
	}

	page := getDefaultInt(c, "page", 1)
	pageSize := getDefaultInt(c, "pageSize", 20)

	indexNames, err := generateIndexNames(ESTaskIndexPrefix, startDate, endDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	aggsQuery := buildDimensionAgg(dimension, pageSize)

	aggregations, err := esClient.Aggregate(indexNames, aggsQuery)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 解析 buckets
	var buckets []interface{}
	if dimAgg, ok := aggregations["dimension_agg"].(map[string]interface{}); ok {
		if b, ok := dimAgg["buckets"].([]interface{}); ok {
			buckets = b
		}
	}

	items := make([]map[string]interface{}, 0, len(buckets))
	for _, raw := range buckets {
		bucket, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}

		key := fmt.Sprintf("%v", bucket["key"])
		minStart := getAggValue(bucket, "min_start_time")
		maxEnd := getAggValue(bucket, "max_end_time")
		leadTime := float64(0)
		if maxEnd > 0 && minStart > 0 {
			leadTime = maxEnd - minStart
		}

		// 二次查询计算 process_time
		query := buildBucketQuery(dimension, key)
		searchResult, err := esClient.Search(indexNames, query, 0, ESMaxSearchSize, "api_request_time", "asc")
		processTime := float64(0)
		if err == nil && len(searchResult.Hits) > 0 {
			processTime = calcProcessTime(searchResult.Hits)
		}

		item := map[string]interface{}{
			"key":               key,
			"user_in_chars":     getAggValue(bucket, "sum_user_in_chars"),
			"code_lines":        getAggValue(bucket, "sum_code_lines"),
			"api_count":         getAggValue(bucket, "sum_api_count"),
			"api_cost":          getAggValue(bucket, "sum_api_cost"),
			"api_in_tokens":     getAggValue(bucket, "sum_api_in_tokens"),
			"api_out_tokens":    getAggValue(bucket, "sum_api_out_tokens"),
			"task_count":        getAggValue(bucket, "task_count"),
			"ai_estimated_days": getAggValue(bucket, "sum_ai_estimated_days"),
			"start_time":        getAggTimeString(bucket, "min_start_time"),
			"end_time":          getAggTimeString(bucket, "max_end_time"),
			"lead_time":         leadTime,
			"process_time":      processTime,
		}
		items = append(items, item)
	}

	c.JSON(http.StatusOK, gin.H{
		"total":    len(buckets),
		"page":     page,
		"pageSize": pageSize,
		"items":    items,
	})
}

// getAggregateSummary 维度聚合汇总
func getAggregateSummary(c *gin.Context) {
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	dimension := c.Query("dimension")
	if startDate == "" || endDate == "" || dimension == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "startDate, endDate 和 dimension 为必填参数"})
		return
	}
	if !validDimensions[dimension] {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("无效的 dimension: %s，有效值为 work_dir, repo, user, org1, org2, org3, org4", dimension)})
		return
	}

	indexNames, err := generateIndexNames(ESTaskIndexPrefix, startDate, endDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	aggsQuery := buildDimensionAgg(dimension, ESMaxSearchSize)

	aggregations, err := esClient.Aggregate(indexNames, aggsQuery)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var buckets []interface{}
	if dimAgg, ok := aggregations["dimension_agg"].(map[string]interface{}); ok {
		if b, ok := dimAgg["buckets"].([]interface{}); ok {
			buckets = b
		}
	}

	totalTaskCount := float64(0)
	totalApiCount := float64(0)
	totalApiCost := float64(0)
	totalAiEstimatedDays := float64(0)

	for _, raw := range buckets {
		bucket, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		totalTaskCount += getAggValue(bucket, "task_count")
		totalApiCount += getAggValue(bucket, "sum_api_count")
		totalApiCost += getAggValue(bucket, "sum_api_cost")
		totalAiEstimatedDays += getAggValue(bucket, "sum_ai_estimated_days")
	}

	c.JSON(http.StatusOK, gin.H{
		"dimension":               dimension,
		"bucket_count":            len(buckets),
		"total_task_count":        totalTaskCount,
		"total_api_count":         totalApiCount,
		"total_api_cost":          totalApiCost,
		"total_ai_estimated_days": totalAiEstimatedDays,
	})
}

// getAggregateKeys 轻量级接口：只返回指定维度的 key 列表（用于下拉提示）
func getAggregateKeys(c *gin.Context) {
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	dimension := c.Query("dimension")
	keyword := c.Query("keyword") // 可选：前端输入的搜索关键词

	if startDate == "" || endDate == "" || dimension == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "startDate, endDate 和 dimension 为必填参数"})
		return
	}
	if !validDimensions[dimension] {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("无效的 dimension: %s", dimension)})
		return
	}

	indexNames, err := generateIndexNames(ESTaskIndexPrefix, startDate, endDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 只做 terms 聚合，不带子聚合，最多返回 500 个 key
	var aggs map[string]interface{}
	if esField, ok := dimensionFieldMap[dimension]; ok {
		termsConf := map[string]interface{}{"field": esField, "size": 500}
		// 如果有搜索关键词，加 include 正则过滤
		if keyword != "" {
			termsConf["include"] = fmt.Sprintf(".*%s.*", strings.ReplaceAll(keyword, ".", "\\."))
		}
		aggs = map[string]interface{}{
			"keys_agg": map[string]interface{}{
				"terms": termsConf,
			},
		}
	} else if scriptSource, ok := dimensionScriptMap[dimension]; ok {
		aggs = map[string]interface{}{
			"keys_agg": map[string]interface{}{
				"terms": map[string]interface{}{
					"script": map[string]interface{}{"source": scriptSource, "lang": "painless"},
					"size":   500,
				},
			},
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无法构建维度聚合"})
		return
	}

	result, err := esClient.Aggregate(indexNames, aggs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var keys []string
	if keysAgg, ok := result["keys_agg"].(map[string]interface{}); ok {
		if buckets, ok := keysAgg["buckets"].([]interface{}); ok {
			for _, raw := range buckets {
				if bucket, ok := raw.(map[string]interface{}); ok {
					if key, ok := bucket["key"].(string); ok {
						// 如果有关键词且是 script 维度，在 Go 端过滤
						if keyword != "" && strings.Contains(strings.ToLower(key), strings.ToLower(keyword)) {
							keys = append(keys, key)
						} else if keyword == "" {
							keys = append(keys, key)
						}
					}
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"keys": keys, "total": len(keys)})
}
