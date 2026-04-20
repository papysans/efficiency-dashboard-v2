package core

import (
	"regexp"
	"strings"
)

var (
	reNonPathSafe    = regexp.MustCompile(`[^a-z0-9\-]`)
	reMultipleDashes = regexp.MustCompile(`-{2,}`)
)

// ToPathSafeID 将原始字符串转换为 path-safe 格式
// - 转换为小写
// - 只保留字母、数字、点号和连字符
// - 多个连字符合并为一个
// - 移除首尾连字符
func ToPathSafeID(raw string) string {
	if raw == "" {
		return ""
	}
	result := strings.ToLower(raw)
	result = reNonPathSafe.ReplaceAllString(result, "-")
	result = reMultipleDashes.ReplaceAllString(result, "-")
	result = strings.Trim(result, "-")
	return result
}
