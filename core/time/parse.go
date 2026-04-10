package coretime

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ParseTimeString 解析单个时间字符串，返回 "YYYYMMDD" 格式。
// 支持格式：
//   - 2023Q1/2023Q2/2023Q3/2023Q4 → 季度末日期
//   - 2023H1/2023H2               → 半年度末日期
//   - 2023                        → 年报（20231231）
//   - 20231231                    → 原样返回（验证有效性）
//   - 2023-12-31                  → 转为 20231231
func ParseTimeString(timeStr string) (string, error) {
	timeStr = strings.TrimSpace(timeStr)

	quarterRegex := regexp.MustCompile(`^(\d{4})[Qq]([1-4])$`)
	if matches := quarterRegex.FindStringSubmatch(timeStr); matches != nil {
		year, _ := strconv.Atoi(matches[1])
		quarter, _ := strconv.Atoi(matches[2])
		var month, day int
		switch quarter {
		case 1:
			month, day = 3, 31
		case 2:
			month, day = 6, 30
		case 3:
			month, day = 9, 30
		case 4:
			month, day = 12, 31
		}
		return fmt.Sprintf("%04d%02d%02d", year, month, day), nil
	}

	hRegex := regexp.MustCompile(`^(\d{4})[Hh]([1-2])$`)
	if matches := hRegex.FindStringSubmatch(timeStr); matches != nil {
		year, _ := strconv.Atoi(matches[1])
		h, _ := strconv.Atoi(matches[2])
		var month, day int
		switch h {
		case 1:
			month, day = 6, 30
		case 2:
			month, day = 12, 31
		}
		return fmt.Sprintf("%04d%02d%02d", year, month, day), nil
	}

	yearRegex := regexp.MustCompile(`^(\d{4})$`)
	if matches := yearRegex.FindStringSubmatch(timeStr); matches != nil {
		year, _ := strconv.Atoi(matches[1])
		return fmt.Sprintf("%04d1231", year), nil
	}

	if regexp.MustCompile(`^(\d{8})$`).MatchString(timeStr) {
		if _, err := time.Parse("20060102", timeStr); err != nil {
			return "", fmt.Errorf("无效的日期: %s", timeStr)
		}
		return timeStr, nil
	}

	if regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})$`).MatchString(timeStr) {
		t, err := time.Parse("2006-01-02", timeStr)
		if err != nil {
			return "", fmt.Errorf("无效的日期: %s", timeStr)
		}
		return t.Format("20060102"), nil
	}

	return "", fmt.Errorf("不支持的时间格式，支持格式: 2023Q1, 2023H1, 2023, 20231231, 2023-12-31")
}

// ParseTimeRange 解析时间范围，返回 "YYYYMMDD" 格式的起止日期。
// to 为空默认今天，from 为空默认 to 倒推 3 年。
func ParseTimeRange(from, to string) (startDate, endDate string, err error) {
	if to == "" {
		endDate = time.Now().Format("20060102")
	} else {
		endDate, err = ParseTimeString(to)
		if err != nil {
			return "", "", fmt.Errorf("无效的结束时间格式 [%s]: %w", to, err)
		}
	}

	if from == "" {
		endTime, _ := time.Parse("20060102", endDate)
		startDate = endTime.AddDate(-3, 0, 0).Format("20060102")
	} else {
		startDate, err = ParseTimeString(from)
		if err != nil {
			return "", "", fmt.Errorf("无效的起始时间格式 [%s]: %w", from, err)
		}
	}

	return startDate, endDate, nil
}
