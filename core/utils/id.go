package utils

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

// GenerateWorkDirID 根据 clientID 和 workDir 生成工作目录唯一标识
// 算法：clientID前6位 + "-" + workDir路径安全化
func GenerateWorkDirID(clientID, workDir string) string {
	prefix := clientID
	if len(prefix) > 6 {
		prefix = prefix[:6]
	}

	suffix := ToPathSafeID(workDir)

	if prefix == "" && suffix == "" {
		return ""
	}
	if prefix == "" {
		return suffix
	}
	if suffix == "" {
		return prefix
	}
	return prefix + "-" + suffix
}
