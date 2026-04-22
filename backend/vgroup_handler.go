package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
)

type VirtualGroupResponse struct {
	ID         int      `json:"id"`
	Name       string   `json:"name"`
	Dimension  string   `json:"dimension"`
	MemberKeys []string `json:"member_keys"`
}

type AggregateVirtualGroupResponse struct {
	Key             string  `json:"key"`
	Name            string  `json:"name"`
	UserInChars     float64 `json:"user_in_chars"`
	CodeLines       float64 `json:"code_lines"`
	APICount        float64 `json:"api_count"`
	APICost         float64 `json:"api_cost"`
	APIInTokens     float64 `json:"api_in_tokens"`
	APIOutTokens    float64 `json:"api_out_tokens"`
	TaskCount       float64 `json:"task_count"`
	AIEstimatedDays float64 `json:"ai_estimated_days"`
	StartTime       string  `json:"start_time"`
	EndTime         string  `json:"end_time"`
	LeadTime        float64 `json:"lead_time"`
	ProcessTime     float64 `json:"process_time"`
}

type FavoriteResponse struct {
	ID          int    `json:"id"`
	Dimension   string `json:"dimension"`
	ItemKey     string `json:"item_key"`
	DisplayName string `json:"display_name"`
}

type FavoriteItem struct {
	ID             int    `json:"id"`
	Dimension      string `json:"dimension"`
	ItemKey        string `json:"item_key"`
	DisplayName    string `json:"display_name"`
	VirtualGroupID *int64 `json:"virtual_group_id"`
	IsVirtual      bool   `json:"is_virtual"`
}

type CreateVirtualGroupRequest struct {
	Name       string   `json:"name" example:"前端组"`
	Dimension  string   `json:"dimension" example:"work_dir"`
	MemberKeys []string `json:"member_keys"`
}

type CreateFavoriteRequest struct {
	Dimension   string `json:"dimension" example:"work_dir"`
	ItemKey     string `json:"item_key" example:"project1"`
	DisplayName string `json:"display_name" example:"项目1"`
}

// createVirtualGroup 创建虚拟组
// @Summary 创建虚拟组
// @Description 创建新的虚拟组
// @Tags VirtualGroups
// @Accept json
// @Produce json
// @Param group body CreateVirtualGroupRequest true "虚拟组信息"
// @Success 201 {object} VirtualGroupResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/virtual-groups [post]
func createVirtualGroup(c *gin.Context) {
	var req CreateVirtualGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "请求参数解析失败"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Dimension = strings.TrimSpace(req.Dimension)
	if req.Name == "" || req.Dimension == "" || len(req.MemberKeys) == 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "name、dimension、member_keys 不能为空"})
		return
	}
	if !validDimensions[req.Dimension] {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: fmt.Sprintf("无效的 dimension: %s", req.Dimension)})
		return
	}

	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "开启事务失败"})
		return
	}
	defer tx.Rollback()

	var id int
	err = tx.QueryRow(
		`INSERT INTO virtual_groups (name, dimension, member_keys) VALUES ($1, $2, $3) RETURNING id`,
		req.Name, req.Dimension, pq.Array(req.MemberKeys),
	).Scan(&id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: fmt.Sprintf("创建虚拟组失败: %v", err)})
		return
	}

	itemKey := fmt.Sprintf("vg_%d", id)
	_, err = tx.Exec(
		`INSERT INTO favorites (dimension, item_key, display_name, virtual_group_id) VALUES ($1, $2, $3, $4)`,
		req.Dimension, itemKey, req.Name, id,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: fmt.Sprintf("创建收藏记录失败: %v", err)})
		return
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "提交事务失败"})
		return
	}

	c.JSON(http.StatusCreated, VirtualGroupResponse{ID: id, Name: req.Name, Dimension: req.Dimension, MemberKeys: req.MemberKeys})
}

// aggregateVirtualGroup 聚合虚拟组数据
// @Summary 聚合虚拟组数据
// @Description 根据虚拟组ID聚合查询ES数据，返回虚拟组的汇总统计指标
// @Tags VirtualGroups
// @Produce json
// @Param id path string true "虚拟组ID"
// @Param startDate query string true "开始日期(YYYYMMDD)"
// @Param endDate query string true "结束日期(YYYYMMDD)"
// @Success 200 {object} AggregateVirtualGroupResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/virtual-groups/{id}/aggregate [get]
func aggregateVirtualGroup(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "无效的虚拟组 ID"})
		return
	}

	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	if startDate == "" || endDate == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "startDate 和 endDate 为必填参数"})
		return
	}

	// 查询虚拟组信息
	var name, dimension string
	var memberKeys []string
	err = db.QueryRow(
		`SELECT name, dimension, member_keys FROM virtual_groups WHERE id=$1`, id,
	).Scan(&name, &dimension, pq.Array(&memberKeys))
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "虚拟组不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: fmt.Sprintf("查询虚拟组失败: %v", err)})
		return
	}

	indexNames, err := generateIndexNames(ESTaskIndexPrefix, startDate, endDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
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
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: fmt.Sprintf("ES 聚合查询失败: %v", err)})
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

	c.JSON(http.StatusOK, AggregateVirtualGroupResponse{Key: fmt.Sprintf("vg_%d", id), Name: name, UserInChars: getAggValue(aggregations, "sum_user_in_chars"), CodeLines: getAggValue(aggregations, "sum_code_lines"), APICount: getAggValue(aggregations, "sum_api_count"), APICost: getAggValue(aggregations, "sum_api_cost"), APIInTokens: getAggValue(aggregations, "sum_api_in_tokens"), APIOutTokens: getAggValue(aggregations, "sum_api_out_tokens"), TaskCount: getAggValue(aggregations, "task_count"), AIEstimatedDays: getAggValue(aggregations, "sum_ai_estimated_days"), StartTime: getAggTimeString(aggregations, "min_start_time"), EndTime: getAggTimeString(aggregations, "max_end_time"), LeadTime: leadTime, ProcessTime: processTime})
}

// createFavorite 添加收藏
// @Summary 添加收藏
// @Description 添加新的收藏
// @Tags Favorites
// @Accept json
// @Produce json
// @Param favorite body CreateFavoriteRequest true "收藏信息"
// @Success 201 {object} FavoriteResponse
// @Failure 400 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/favorites [post]
func createFavorite(c *gin.Context) {
	var req CreateFavoriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "请求参数解析失败"})
		return
	}
	req.Dimension = strings.TrimSpace(req.Dimension)
	req.ItemKey = strings.TrimSpace(req.ItemKey)
	if req.Dimension == "" || req.ItemKey == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "dimension 和 item_key 不能为空"})
		return
	}

	var id int
	err := db.QueryRow(
		`INSERT INTO favorites (dimension, item_key, display_name) VALUES ($1, $2, $3) RETURNING id`,
		req.Dimension, req.ItemKey, req.DisplayName,
	).Scan(&id)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			c.JSON(http.StatusConflict, ErrorResponse{Error: "该收藏已存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: fmt.Sprintf("添加收藏失败: %v", err)})
		return
	}

	c.JSON(http.StatusCreated, FavoriteResponse{ID: id, Dimension: req.Dimension, ItemKey: req.ItemKey, DisplayName: req.DisplayName})
}

// listFavorites 查询收藏列表
// @Summary 获取收藏列表
// @Description 按条件查询收藏列表
// @Tags Favorites
// @Produce json
// @Param dimension query string false "维度过滤"
// @Success 200 {array} FavoriteItem
// @Failure 500 {object} ErrorResponse
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
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: fmt.Sprintf("查询收藏列表失败: %v", err)})
		return
	}
	defer rows.Close()

	items := make([]FavoriteItem, 0)
	for rows.Next() {
		var id int
		var dim, itemKey string
		var displayName sql.NullString
		var virtualGroupID sql.NullInt64
		if err := rows.Scan(&id, &dim, &itemKey, &displayName, &virtualGroupID); err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: fmt.Sprintf("扫描收藏数据失败: %v", err)})
			return
		}

		item := FavoriteItem{
			ID:             id,
			Dimension:      dim,
			ItemKey:        itemKey,
			DisplayName:    displayName.String,
			VirtualGroupID: nil,
			IsVirtual:      false,
		}
		if virtualGroupID.Valid {
			vgID := virtualGroupID.Int64
			item.VirtualGroupID = &vgID
			item.IsVirtual = true
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: fmt.Sprintf("遍历收藏数据失败: %v", err)})
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
// @Success 200 {object} StatusMessageResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/favorites/{id} [delete]
func deleteFavorite(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "无效的收藏 ID"})
		return
	}

	var virtualGroupID sql.NullInt64
	err = db.QueryRow(`SELECT virtual_group_id FROM favorites WHERE id=$1`, id).Scan(&virtualGroupID)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "收藏记录不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: fmt.Sprintf("查询收藏记录失败: %v", err)})
		return
	}

	if virtualGroupID.Valid {
		// 删除虚拟组（级联删除 favorites）
		_, err = db.Exec(`DELETE FROM virtual_groups WHERE id=$1`, virtualGroupID.Int64)
	} else {
		_, err = db.Exec(`DELETE FROM favorites WHERE id=$1`, id)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: fmt.Sprintf("取消收藏失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, StatusMessageResponse{Message: "取消收藏成功"})
}
