package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func parseDateParam(dateStr string) (time.Time, error) {
	return time.Parse("20060102", dateStr)
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

func parseOrderParam(order string) (field, dir string) {
	order = strings.TrimSpace(order)
	if order == "" {
		return "", "DESC"
	}
	if strings.HasPrefix(order, "-") {
		return order[1:], "DESC"
	}
	return order, "ASC"
}

func isAllowedField(field string, allowed []string) bool {
	for _, a := range allowed {
		if a == field {
			return true
		}
	}
	return false
}
