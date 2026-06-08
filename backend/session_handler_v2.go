package main

import (
	"kanban/backend/internal/appconfig"
	"net/http"
	"strings"
	"time"

	"kanban/core/models"

	"github.com/gin-gonic/gin"
)

type SessionListResponse struct {
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"pageSize"`
	Data     []models.Session `json:"data"`
}

// listSessionsV2 GET /api/v2/sessions
// @Summary 获取Session列表
// @Description 按条件查询Session列表，支持分页
// @Tags Sessions
// @Produce json
// @Param userId query string false "用户ID"
// @Param userIds query string false "用户ID列表，逗号分隔"
// @Param userName query string false "用户名"
// @Param clientId query string false "客户端ID"
// @Param clientIde query string false "IDE类型"
// @Param clientVersion query string false "客户端版本"
// @Param clientOs query string false "操作系统"
// @Param startDate query string false "开始日期(YYYYMMDD)"
// @Param endDate query string false "结束日期(YYYYMMDD)"
// @Param order query string false "排序字段，如 createTime 或 -createTime"
// @Param page query int false "页码" default(1)
// @Param pageSize query int false "每页数量" default(20)
// @Success 200 {object} SessionListResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/sessions [get]
func listSessionsV2(c *gin.Context) {
	page := getDefaultInt(c, "page", 1)
	pageSize := getDefaultInt(c, "pageSize", appconfig.DefaultPageSize)

	filter := SessionFilter{}

	if v := strings.TrimSpace(c.Query("userId")); v != "" {
		filter.UserId = v
	}
	if v := strings.TrimSpace(c.Query("userIds")); v != "" {
		filter.UserIds = strings.Split(v, ",")
	}
	if v := strings.TrimSpace(c.Query("userName")); v != "" {
		filter.UserName = v
	}
	if v := strings.TrimSpace(c.Query("clientId")); v != "" {
		filter.ClientId = v
	}
	if v := strings.TrimSpace(c.Query("clientIde")); v != "" {
		filter.ClientIde = v
	}
	if v := strings.TrimSpace(c.Query("clientVersion")); v != "" {
		filter.ClientVersion = v
	}
	if v := strings.TrimSpace(c.Query("clientOs")); v != "" {
		filter.ClientOs = v
	}

	startDate := strings.TrimSpace(c.Query("startDate"))
	endDate := strings.TrimSpace(c.Query("endDate"))
	if startDate != "" || endDate != "" {
		if startDate != "" {
			startT, err := parseDateParam(startDate)
			if err != nil {
				c.JSON(http.StatusBadRequest, ErrorResponse{Error: "startDate 格式错误: " + err.Error()})
				return
			}
			filter.StartTime = startT.Format(time.RFC3339)
		}
		if endDate != "" {
			endT, err := parseDateParam(endDate)
			if err != nil {
				c.JSON(http.StatusBadRequest, ErrorResponse{Error: "endDate 格式错误: " + err.Error()})
				return
			}
			filter.EndTime = endT.Add(23*time.Hour + 59*time.Minute + 59*time.Second).Format(time.RFC3339)
		}
	}

	orderField, orderDir := parseOrderParam(strings.TrimSpace(c.Query("order")))
	if orderField != "" && !isAllowedField(orderField, sessionSortFields) {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "不支持的排序字段: " + orderField})
		return
	}
	orderClause := buildSessionOrder(orderField, orderDir)

	sessions, total, err := ListSessions(statDB, filter, page, pageSize, orderClause)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, SessionListResponse{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Data:     sessions,
	})
}

// getSessionDetailV2 GET /api/v2/sessions/:session_id
// @Summary 获取Session详情
// @Description 根据session_id获取Session详细信息
// @Tags Sessions
// @Produce json
// @Param session_id path string true "Session ID"
// @Success 200 {object} models.Session
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/sessions/{session_id} [get]
func getSessionDetailV2(c *gin.Context) {
	sessionId := c.Param("session_id")

	session, err := GetSession(statDB, sessionId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	if session == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "session not found"})
		return
	}

	c.JSON(http.StatusOK, session)
}
