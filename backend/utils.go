package main

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

var safeIDRegex = regexp.MustCompile("[^a-zA-Z0-9]")

func makeSafeID(id string) string {
	return safeIDRegex.ReplaceAllString(id, "_")
}

func parseDateParam(dateStr string) (time.Time, error) {
	return time.Parse("20060102", dateStr)
}

func formatDateYMD(t time.Time) string {
	return t.Format("2006-01-02")
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
