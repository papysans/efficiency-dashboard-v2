package main

import (
	"strings"

	"github.com/gin-gonic/gin"

	"kanban/backend/internal/appconfig"
)

// ConfigResponse 全局配置响应结构
type ConfigResponse struct {
	TraditionalDevLinesPerDay int `json:"traditional_dev_lines_per_day" example:"100"`
	// CostPerPersonDay 人天单价（¥/人天），前端高管大屏把节省人天折算成节省成本用；yaml 缺省默认 2000。
	CostPerPersonDay     float64 `json:"cost_per_person_day" example:"2000"`
	DashboardTitlePrefix string  `json:"dashboard_title_prefix" example:"Costrict"`
	// ChatStatsEnabled chat-indicator-statistics 代理是否启用（chat_stats.base_url 非空）；前端据此显隐「平台」分组。
	ChatStatsEnabled bool `json:"chat_stats_enabled" example:"false"`
}

// getConfigV2 返回前端所需的全局配置
// @Summary 获取全局配置
// @Description 获取前端所需的全局配置信息
// @Tags Config
// @Produce json
// @Success 200 {object} ConfigResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/config [get]
func getConfigV2(c *gin.Context) {
	c.JSON(200, ConfigResponse{
		TraditionalDevLinesPerDay: appconfig.Cfg.TraditionalDevLinesPerDay,
		CostPerPersonDay:          appconfig.Cfg.CostPerPersonDay,
		DashboardTitlePrefix:      appconfig.Cfg.DashboardTitlePrefix,
		ChatStatsEnabled:          strings.TrimSpace(appconfig.Cfg.ChatStats.BaseURL) != "",
	})
}
