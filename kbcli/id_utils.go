package main

import "comdigger/core"

// toPathSafeID 将原始字符串转换为 path-safe 格式（别名，保持向后兼容）
func toPathSafeID(raw string) string {
	return core.ToPathSafeID(raw)
}
