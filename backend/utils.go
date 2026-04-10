package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

func ptrFloat64(v float64) *float64 { return &v }
func ptrInt64(v int64) *int64       { return &v }
func ptrInt(v int) *int             { return &v }
func ptrString(v string) *string    { return &v }
func ptrTime(v time.Time) *time.Time { return &v }

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

func parseESTime(val interface{}) (time.Time, bool) {
	switch v := val.(type) {
	case float64:
		return time.UnixMilli(int64(v)), true
	case string:
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			t, err = time.Parse("2006-01-02T15:04:05-07:00", v)
		}
		if err != nil {
			t, err = time.Parse("2006-01-02T15:04:05Z", v)
		}
		return t, err == nil
	}
	return time.Time{}, false
}

func getFloat64(m map[string]interface{}, key string) float64 {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case float64:
			return val
		case int:
			return float64(val)
		}
	}
	return 0
}

// extractJSON 从 AI 响应文本中提取 JSON 对象
func extractJSON(text string) string {
	text = strings.TrimSpace(text)
	if json.Valid([]byte(text)) {
		return text
	}
	re := regexp.MustCompile("(?s)```(?:json)?\\s*\\n?(.*?)\\n?```")
	if matches := re.FindStringSubmatch(text); len(matches) > 1 {
		candidate := strings.TrimSpace(matches[1])
		if json.Valid([]byte(candidate)) {
			return candidate
		}
	}
	lastBrace := strings.LastIndex(text, "}")
	if lastBrace >= 0 {
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

// generateIndexNames 根据日期范围生成索引名列表，格式: prefix_YYYYMMDD
func generateIndexNames(prefix string, startDate, endDate string) ([]string, error) {
	start, err := time.Parse("20060102", startDate)
	if err != nil {
		return nil, fmt.Errorf("startDate 格式错误，需要 YYYYMMDD: %w", err)
	}
	end, err := time.Parse("20060102", endDate)
	if err != nil {
		return nil, fmt.Errorf("endDate 格式错误，需要 YYYYMMDD: %w", err)
	}

	var names []string
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		names = append(names, prefix+d.Format("20060102"))
	}
	return names, nil
}

// calcProcessTimeMs 统一的 process_time 计算算法
// timestamps 为已排序的毫秒级时间戳切片，格式: [start1, end1, start2, end2, ...]
func calcProcessTimeMs(timestamps []float64) float64 {
	if len(timestamps) < 2 {
		return 0
	}
	segStart := timestamps[0]
	segEnd := timestamps[1]
	var total float64
	for i := 2; i+1 < len(timestamps); i += 2 {
		gap := timestamps[i] - segEnd
		if gap <= ProcessTimeGapMs {
			if timestamps[i+1] > segEnd {
				segEnd = timestamps[i+1]
			}
		} else {
			total += segEnd - segStart
			segStart = timestamps[i]
			segEnd = timestamps[i+1]
		}
	}
	total += segEnd - segStart
	return total
}
