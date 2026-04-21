package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
)

// createVirtualGroup 创建虚拟组
// @Summary 创建虚拟组
// @Description 创建新的虚拟组
// @Tags VirtualGroups
// @Accept json
// @Produce json
// @Param group body object true "虚拟组信息"
// @Success 200 {object} object
// @Failure 400 {object} object
// @Router /api/virtual-groups [post]
func createVirtualGroup(c *gin.Context) {
	var req struct {
		Name       string   `json:"name"`
		Dimension  string   `json:"dimension"`
		MemberKeys []string `json:"member_keys"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数解析失败"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Dimension = strings.TrimSpace(req.Dimension)
	if req.Name == "" || req.Dimension == "" || len(req.MemberKeys) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name、dimension、member_keys 不能为空"})
		return
	}
	if !validDimensions[req.Dimension] {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("无效的 dimension: %s", req.Dimension)})
		return
	}

	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "开启事务失败"})
		return
	}
	defer tx.Rollback()

	var id int
	err = tx.QueryRow(
		`INSERT INTO virtual_groups (name, dimension, member_keys) VALUES ($1, $2, $3) RETURNING id`,
		req.Name, req.Dimension, pq.Array(req.MemberKeys),
	).Scan(&id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("创建虚拟组失败: %v", err)})
		return
	}

	itemKey := fmt.Sprintf("vg_%d", id)
	_, err = tx.Exec(
		`INSERT INTO favorites (dimension, item_key, display_name, virtual_group_id) VALUES ($1, $2, $3, $4)`,
		req.Dimension, itemKey, req.Name, id,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("创建收藏记录失败: %v", err)})
		return
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "提交事务失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":          id,
		"name":        req.Name,
		"dimension":   req.Dimension,
		"member_keys": req.MemberKeys,
	})
}

// listVirtualGroups 查询虚拟组列表
// @Summary 获取虚拟组列表
// @Description 按条件查询虚拟组列表
// @Tags VirtualGroups
// @Produce json
// @Param dimension query string false "维度过滤"
// @Success 200 {object} object
// @Router /api/virtual-groups [get]
func listVirtualGroups(c *gin.Context) {
	dimension := strings.TrimSpace(c.Query("dimension"))

	var rows *sql.Rows
	var err error
	if dimension != "" {
		rows, err = db.Query(
			`SELECT id, name, dimension, member_keys, created_at, updated_at FROM virtual_groups WHERE dimension=$1 ORDER BY id`,
			dimension,
		)
	} else {
		rows, err = db.Query(
			`SELECT id, name, dimension, member_keys, created_at, updated_at FROM virtual_groups ORDER BY id`,
		)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("查询虚拟组失败: %v", err)})
		return
	}
	defer rows.Close()

	items := make([]gin.H, 0)
	for rows.Next() {
		var id int
		var name, dim string
		var memberKeys []string
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&id, &name, &dim, pq.Array(&memberKeys), &createdAt, &updatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("扫描虚拟组数据失败: %v", err)})
			return
		}
		items = append(items, gin.H{
			"id":          id,
			"name":        name,
			"dimension":   dim,
			"member_keys": memberKeys,
			"created_at":  createdAt,
			"updated_at":  updatedAt,
		})
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("遍历虚拟组数据失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, items)
}

// deleteVirtualGroup 删除虚拟组
// @Summary 删除虚拟组
// @Description 根据虚拟组ID删除虚拟组
// @Tags VirtualGroups
// @Produce json
// @Param id path string true "虚拟组ID"
// @Success 200 {object} object
// @Failure 404 {object} object
// @Router /api/virtual-groups/{id} [delete]
func deleteVirtualGroup(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的虚拟组 ID"})
		return
	}

	result, err := db.Exec(`DELETE FROM virtual_groups WHERE id=$1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("删除虚拟组失败: %v", err)})
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "虚拟组不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// aggregateVirtualGroup 聚合虚拟组数据
func aggregateVirtualGroup(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的虚拟组 ID"})
		return
	}

	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	if startDate == "" || endDate == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "startDate 和 endDate 为必填参数"})
		return
	}

	// 查询虚拟组信息
	var name, dimension string
	var memberKeys []string
	err = db.QueryRow(
		`SELECT name, dimension, member_keys FROM virtual_groups WHERE id=$1`, id,
	).Scan(&name, &dimension, pq.Array(&memberKeys))
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "虚拟组不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("查询虚拟组失败: %v", err)})
		return
	}

	indexNames, err := generateIndexNames(ESTaskIndexPrefix, startDate, endDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 构建 bool query，should 子句包含所有 member_keys
	shouldClauses := make([]interface{}, 0, len(memberKeys))
	for _, key := range memberKeys {
		shouldClauses = append(shouldClauses, buildBucketQuery(dimension, key))
	}
	query := map[string]interface{}{
		"bool": map[string]interface{}{
			"should":               shouldClauses,
			"minimum_should_match": 1,
		},
	}

	// 聚合查询（不分组，直接汇总）
	aggsQuery := buildSubAggs()
	aggregations, err := esClient.AggregateWithQuery(indexNames, query, aggsQuery)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("ES 聚合查询失败: %v", err)})
		return
	}

	// 搜索查询计算 process_time
	searchResult, err := esClient.Search(indexNames, query, 0, 10000, "api_request_time", "asc")
	processTime := float64(0)
	if err == nil && len(searchResult.Hits) > 0 {
		processTime = calcProcessTime(searchResult.Hits)
	}

	minStart := getAggValue(aggregations, "min_start_time")
	maxEnd := getAggValue(aggregations, "max_end_time")
	leadTime := float64(0)
	if maxEnd > 0 && minStart > 0 {
		leadTime = maxEnd - minStart
	}

	c.JSON(http.StatusOK, gin.H{
		"key":               fmt.Sprintf("vg_%d", id),
		"name":              name,
		"user_in_chars":     getAggValue(aggregations, "sum_user_in_chars"),
		"code_lines":        getAggValue(aggregations, "sum_code_lines"),
		"api_count":         getAggValue(aggregations, "sum_api_count"),
		"api_cost":          getAggValue(aggregations, "sum_api_cost"),
		"api_in_tokens":     getAggValue(aggregations, "sum_api_in_tokens"),
		"api_out_tokens":    getAggValue(aggregations, "sum_api_out_tokens"),
		"task_count":        getAggValue(aggregations, "task_count"),
		"ai_estimated_days": getAggValue(aggregations, "sum_ai_estimated_days"),
		"start_time":        getAggTimeString(aggregations, "min_start_time"),
		"end_time":          getAggTimeString(aggregations, "max_end_time"),
		"lead_time":         leadTime,
		"process_time":      processTime,
	})
}

// createFavorite 添加收藏
// @Summary 添加收藏
// @Description 添加新的收藏
// @Tags Favorites
// @Accept json
// @Produce json
// @Param favorite body object true "收藏信息"
// @Success 200 {object} object
// @Failure 400 {object} object
// @Router /api/favorites [post]
func createFavorite(c *gin.Context) {
	var req struct {
		Dimension   string `json:"dimension"`
		ItemKey     string `json:"item_key"`
		DisplayName string `json:"display_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数解析失败"})
		return
	}
	req.Dimension = strings.TrimSpace(req.Dimension)
	req.ItemKey = strings.TrimSpace(req.ItemKey)
	if req.Dimension == "" || req.ItemKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "dimension 和 item_key 不能为空"})
		return
	}

	var id int
	err := db.QueryRow(
		`INSERT INTO favorites (dimension, item_key, display_name) VALUES ($1, $2, $3) RETURNING id`,
		req.Dimension, req.ItemKey, req.DisplayName,
	).Scan(&id)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			c.JSON(http.StatusConflict, gin.H{"error": "该收藏已存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("添加收藏失败: %v", err)})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":           id,
		"dimension":    req.Dimension,
		"item_key":     req.ItemKey,
		"display_name": req.DisplayName,
	})
}

// listFavorites 查询收藏列表
// @Summary 获取收藏列表
// @Description 按条件查询收藏列表
// @Tags Favorites
// @Produce json
// @Param dimension query string false "维度过滤"
// @Success 200 {object} object
// @Router /api/favorites [get]
func listFavorites(c *gin.Context) {
	dimension := strings.TrimSpace(c.Query("dimension"))

	var rows *sql.Rows
	var err error
	if dimension != "" {
		rows, err = db.Query(
			`SELECT id, dimension, item_key, display_name, virtual_group_id FROM favorites WHERE dimension=$1 ORDER BY id`,
			dimension,
		)
	} else {
		rows, err = db.Query(
			`SELECT id, dimension, item_key, display_name, virtual_group_id FROM favorites ORDER BY id`,
		)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("查询收藏列表失败: %v", err)})
		return
	}
	defer rows.Close()

	items := make([]gin.H, 0)
	for rows.Next() {
		var id int
		var dim, itemKey string
		var displayName sql.NullString
		var virtualGroupID sql.NullInt64
		if err := rows.Scan(&id, &dim, &itemKey, &displayName, &virtualGroupID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("扫描收藏数据失败: %v", err)})
			return
		}

		item := gin.H{
			"id":               id,
			"dimension":        dim,
			"item_key":         itemKey,
			"display_name":     displayName.String,
			"virtual_group_id": nil,
			"is_virtual":       false,
		}
		if virtualGroupID.Valid {
			item["virtual_group_id"] = virtualGroupID.Int64
			item["is_virtual"] = true
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("遍历收藏数据失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, items)
}

// deleteFavorite 取消收藏
// @Summary 取消收藏
// @Description 根据收藏ID取消收藏
// @Tags Favorites
// @Produce json
// @Param id path string true "收藏ID"
// @Success 200 {object} object
// @Failure 404 {object} object
// @Router /api/favorites/{id} [delete]
func deleteFavorite(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的收藏 ID"})
		return
	}

	var virtualGroupID sql.NullInt64
	err = db.QueryRow(`SELECT virtual_group_id FROM favorites WHERE id=$1`, id).Scan(&virtualGroupID)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "收藏记录不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("查询收藏记录失败: %v", err)})
		return
	}

	if virtualGroupID.Valid {
		// 删除虚拟组（级联删除 favorites）
		_, err = db.Exec(`DELETE FROM virtual_groups WHERE id=$1`, virtualGroupID.Int64)
	} else {
		_, err = db.Exec(`DELETE FROM favorites WHERE id=$1`, id)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("取消收藏失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "取消收藏成功"})
}
