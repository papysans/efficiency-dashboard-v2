package main

import (
	"github.com/gin-gonic/gin"

	"kanban/backend/internal/appconfig"
)

// ConfigResponse 全局配置响应结构
type ConfigResponse struct {
	TraditionalDevLinesPerDay int `json:"traditional_dev_lines_per_day" example:"100"`
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
	})
}
