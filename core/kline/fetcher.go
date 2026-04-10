package kline

import (
	"fmt"
	"time"
)

// FetchKline 统一入口，先试主数据源，失败自动切换备用源
func FetchKline(site string, code string, count int, freq string) ([]KlineBar, error) {
	var primary, fallback func(string, int, string) ([]KlineBar, error)

	if site == SiteTencent {
		primary = GetPriceTencent
		fallback = GetPriceSina
	} else {
		// 默认 SiteSina
		primary = GetPriceSina
		fallback = GetPriceTencent
	}

	// 尝试主数据源（最多3次）
	var lastErr error
	for i := 0; i < 3; i++ {
		bars, err := primary(code, count, freq)
		if err == nil && len(bars) > 0 {
			return bars, nil
		}
		lastErr = err
		if i < 2 {
			time.Sleep(1 * time.Second)
		}
	}

	// 切换备用数据源（最多3次）
	for i := 0; i < 3; i++ {
		bars, err := fallback(code, count, freq)
		if err == nil && len(bars) > 0 {
			return bars, nil
		}
		lastErr = err
		if i < 2 {
			time.Sleep(1 * time.Second)
		}
	}

	return nil, fmt.Errorf("所有数据源获取失败：%w", lastErr)
}
