package utils

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// base64BlobRe 匹配「≥200 个连续 base64 字符(+可选 = 填充)」。粘贴的图片/二进制(data:image;base64,...
// 或 {"type":"base64","data":"..."})、base64 编码的元数据 JSON({"username":...} 编码后)都是这种长串；
// 正常文本/代码/URL/哈希(hex≤64,UUID 带连字符,JWT 带 . 分隔)不会有 200 连续标准 base64 串，故安全。
var base64BlobRe = regexp.MustCompile(`[A-Za-z0-9+/]{200,}={0,2}`)

// StripLargeBase64 把内容里的长 base64 串(图片/二进制 blob/编码 JSON)替换为短占位符 [base64 NB omitted]。
// 用途：导入对话时 request/response_content/user_input 常含用户粘贴的图片 base64,几十~几百 KB 一张,
// 直接入库会把 conversations 表撑爆(DB 已 200MB)。剥离后只留文本,大幅瘦身;编码字节本就无指标/展示价值。
func StripLargeBase64(s string) string {
	if len(s) < 200 {
		return s
	}
	return base64BlobRe.ReplaceAllStringFunc(s, func(m string) string {
		return fmt.Sprintf("[base64 %dB omitted]", len(m))
	})
}

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
