package main

import "comdigger/core"

// toPathSafeID 将原始字符串转换为 path-safe 格式（别名，保持向后兼容）
func toPathSafeID(raw string) string {
	return core.ToPathSafeID(raw)
}

// generateWorkDirID 根据 clientID 和 workDir 生成工作目录唯一标识
// 算法：clientID前6位 + "-" + workDir路径安全化
func generateWorkDirID(clientID, workDir string) string {
	prefix := clientID
	if len(prefix) > 6 {
		prefix = prefix[:6]
	}

	suffix := toPathSafeID(workDir)

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
