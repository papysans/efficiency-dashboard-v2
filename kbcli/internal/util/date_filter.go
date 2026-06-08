package main

import (
	"fmt"
	"time"
)

// parseDateRange 解析 startDate、endDate、date 三个字符串参数为日期范围。
// 若指定了 date，则范围限定为该日期一天之内，startDate 和 endDate 被忽略。
// 返回的 startDate 为当天 00:00:00（若设置），endDate 为次日 00:00:00（若设置），
// 用于半开区间比较 [startDate, endDate)。
func parseDateRange(startDateStr, endDateStr, dateStr string) (startDate, endDate *time.Time, err error) {
	if dateStr != "" {
		t, err := time.Parse("20060102", dateStr)
		if err != nil {
			return nil, nil, fmt.Errorf("date格式错误，应为YYYYMMDD: %w", err)
		}
		endNext := t.AddDate(0, 0, 1)
		return &t, &endNext, nil
	}

	if startDateStr != "" {
		t, err := time.Parse("20060102", startDateStr)
		if err != nil {
			return nil, nil, fmt.Errorf("startDate格式错误，应为YYYYMMDD: %w", err)
		}
		startDate = &t
	}

	if endDateStr != "" {
		t, err := time.Parse("20060102", endDateStr)
		if err != nil {
			return nil, nil, fmt.Errorf("endDate格式错误，应为YYYYMMDD: %w", err)
		}
		endNext := t.AddDate(0, 0, 1)
		endDate = &endNext
	}

	return startDate, endDate, nil
}

// isActiveTimeInRange 检查活跃时间是否在日期范围内。
// startDate 为 nil 表示不限下限；endDate 为 nil 表示不限上限。
// endDate 为次日零点，活跃时间 < endDate 视为在范围内。
func isActiveTimeInRange(activeTime time.Time, startDate, endDate *time.Time) bool {
	if startDate != nil && activeTime.Before(*startDate) {
		return false
	}
	if endDate != nil && !activeTime.Before(*endDate) {
		return false
	}
	return true
}
