package utils

import (
	"strings"
	"unicode/utf8"
)

// SanitizeText 清理文本字段，移除无效的 UTF-8 字符（特别是 null 字节 0x00）
// 这样可以避免 PostgreSQL 的 "invalid byte sequence for encoding UTF8" 错误
func SanitizeText(s string) string {
	if s == "" {
		return s
	}

	// 移除所有 null 字节（0x00）
	result := strings.ReplaceAll(s, "\x00", "")

	// 如果包含其他无效的 UTF-8 字符，移除它们
	if !utf8.ValidString(result) {
		valid := make([]rune, 0, len(result))
		for i, r := range result {
			if r == utf8.RuneError {
				// 检查是否真的是无效字符
				_, size := utf8.DecodeRuneInString(result[i:])
				if size == 1 {
					// 真正的无效字符，跳过
					continue
				}
			}
			valid = append(valid, r)
		}
		result = string(valid)
	}

	return result
}

// RemoveWhitespace 移除字符串中的所有空白字符（空格、制表符、换行符、回车符）
func RemoveWhitespace(s string) string {
	return strings.Map(func(r rune) rune {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			return -1
		}
		return r
	}, s)
}
