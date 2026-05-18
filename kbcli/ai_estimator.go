package main

import (
	"encoding/json"
	"regexp"
	"strings"
)

// extractJSON 从 AI 响应文本中提取 JSON 对象
// 支持三种格式：纯 JSON / markdown 代码块 / 中文分析+JSON 混合
func extractJSON(text string) string {
	text = strings.TrimSpace(text)

	// 1. 直接尝试解析
	if json.Valid([]byte(text)) {
		return text
	}

	// 2. 尝试提取 markdown 代码块内的 JSON
	re := regexp.MustCompile("(?s)```(?:json)?\\s*\\n?(.*?)\\n?```")
	if matches := re.FindStringSubmatch(text); len(matches) > 1 {
		candidate := strings.TrimSpace(matches[1])
		if json.Valid([]byte(candidate)) {
			return candidate
		}
	}

	// 3. 查找文本中最后一个完整的 {...} JSON 对象（模型可能先输出分析再输出 JSON）
	lastBrace := strings.LastIndex(text, "}")
	if lastBrace >= 0 {
		// 从最后的 } 往前找匹配的 {
		depth := 0
		for i := lastBrace; i >= 0; i-- {
			if text[i] == '}' {
				depth++
			} else if text[i] == '{' {
				depth--
				if depth == 0 {
					candidate := strings.TrimSpace(text[i : lastBrace+1])
					if json.Valid([]byte(candidate)) {
						return candidate
					}
				}
			}
		}
	}

	return text
}
