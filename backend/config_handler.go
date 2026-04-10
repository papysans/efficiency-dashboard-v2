package main

import "github.com/gin-gonic/gin"

// getConfigV2 返回前端所需的全局配置
func getConfigV2(c *gin.Context) {
	c.JSON(200, gin.H{
		"traditional_dev_lines_per_day": appConfig.TraditionalDevLinesPerDay,
	})
}
