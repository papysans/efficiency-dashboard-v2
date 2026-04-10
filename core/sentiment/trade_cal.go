package sentiment

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"comdigger/core/httputil"
)

// TradeCal 交易日历
type TradeCal struct {
	TradeDate  string `json:"trade_date"`
	IsTradeDay bool   `json:"is_trade_day"`
}

// FetchTradeCal 获取某月交易日历（深交所）
// yearMonth 格式：YYYY-MM，为空时取当前年月
func FetchTradeCal(yearMonth string) ([]TradeCal, error) {
	if yearMonth == "" {
		yearMonth = time.Now().Format("2006-01")
	}
	url := fmt.Sprintf("https://www.szse.cn/api/report/exchange/onepersistenthour/monthList?month=%s", yearMonth)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://www.szse.cn/")

	resp, err := httputil.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取交易日历失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var result struct {
		Data []struct {
			Jybz string `json:"jybz"` // "1"=交易日, "0"=非交易日
			Jyrq string `json:"jyrq"` // 日期 YYYY-MM-DD
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析交易日历失败: %w", err)
	}

	var cals []TradeCal
	for _, item := range result.Data {
		cals = append(cals, TradeCal{
			TradeDate:  item.Jyrq,
			IsTradeDay: item.Jybz == "1",
		})
	}
	return cals, nil
}

// IsTradeDay 判断某天是否为交易日（格式 YYYY-MM-DD）
func IsTradeDay(date string) (bool, error) {
	if len(date) < 7 {
		return false, fmt.Errorf("日期格式错误: %s", date)
	}
	yearMonth := date[:7]
	cals, err := FetchTradeCal(yearMonth)
	if err != nil {
		return false, err
	}
	for _, cal := range cals {
		if cal.TradeDate == date {
			return cal.IsTradeDay, nil
		}
	}
	return false, nil
}

// GetLastTradeDay 获取最近一个交易日（从今天向前最多查7天）
func GetLastTradeDay() (string, error) {
	today := time.Now()
	for i := 0; i < 7; i++ {
		d := today.AddDate(0, 0, -i)
		dateStr := d.Format("2006-01-02")
		isT, err := IsTradeDay(dateStr)
		if err != nil {
			continue
		}
		if isT {
			return dateStr, nil
		}
	}
	return "", fmt.Errorf("最近7天内未找到交易日")
}
