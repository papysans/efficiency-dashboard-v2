package lhb

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const (
	apiBaseURL  = "http://lhb-api.ws4.cn/v1/youzi/all"
	httpTimeout = 10 * time.Second
	maxRetries  = 3
	rateDelay   = 25 * time.Millisecond
)

// flexibleFloat 支持 JSON 数字和字符串双重解析
type flexibleFloat float64

func (f *flexibleFloat) UnmarshalJSON(data []byte) error {
	// 尝试直接解析数字
	var num float64
	if err := json.Unmarshal(data, &num); err == nil {
		*f = flexibleFloat(num)
		return nil
	}
	// 尝试解析字符串
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		if s == "" || s == "null" {
			*f = 0
			return nil
		}
		num, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return fmt.Errorf("无法解析浮点数: %s", s)
		}
		*f = flexibleFloat(num)
		return nil
	}
	*f = 0
	return nil
}

// apiRecord API原始记录（字段名与API一致）
type apiRecord struct {
	Date       string        `json:"rq"`    // YYYY-MM-DD
	Symbol     string        `json:"gpdm"`  // 股票代码
	Name       string        `json:"gpmc"`  // 股票名称
	YouziName  string        `json:"yzmc"`  // 游资名称
	YYB        string        `json:"yyb"`   // 营业部
	ListType   string        `json:"sblx"`  // 榜单类型
	Concepts   string        `json:"gl"`    // 概念
	BuyAmount  flexibleFloat `json:"mrje"`  // 买入金额（元）
	SellAmount flexibleFloat `json:"mcje"`  // 卖出金额（元）
	NetInflow  flexibleFloat `json:"jlrje"` // 净流入金额（元）
}

// apiResponse API响应结构
type apiResponse struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data []apiRecord `json:"data"`
}

// FetchByDate 获取指定日期的龙虎榜数据
func FetchByDate(date string) ([]LHBRecord, error) {
	client := &http.Client{Timeout: httpTimeout}
	url := fmt.Sprintf("%s?date=%s", apiBaseURL, date)

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			sleepDur := time.Duration(1<<uint(attempt-1)) * time.Second
			time.Sleep(sleepDur)
		}

		resp, err := client.Get(url)
		if err != nil {
			lastErr = fmt.Errorf("请求失败 (attempt %d): %w", attempt+1, err)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("读取响应失败 (attempt %d): %w", attempt+1, err)
			continue
		}

		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("服务器错误 %d (attempt %d)", resp.StatusCode, attempt+1)
			continue
		}

		var apiResp apiResponse
		if err := json.Unmarshal(body, &apiResp); err != nil {
			lastErr = fmt.Errorf("解析响应失败: %w", err)
			continue
		}

		// code=40400 表示无数据，不是错误
		if apiResp.Code == 40400 {
			time.Sleep(rateDelay)
			return nil, nil
		}

		if apiResp.Code != 20000 {
			lastErr = fmt.Errorf("API返回错误 code=%d: %s", apiResp.Code, apiResp.Msg)
			continue
		}

		// 转换为 LHBRecord
		records := make([]LHBRecord, 0, len(apiResp.Data))
		for _, r := range apiResp.Data {
			records = append(records, LHBRecord{
				Date:       r.Date,
				Symbol:     r.Symbol,
				Name:       r.Name,
				YouziName:  r.YouziName,
				YYB:        r.YYB,
				ListType:   r.ListType,
				BuyAmount:  float64(r.BuyAmount),
				SellAmount: float64(r.SellAmount),
				NetInflow:  float64(r.NetInflow),
				Concepts:   r.Concepts,
			})
		}

		time.Sleep(rateDelay) // 遵守速率限制
		return records, nil
	}

	return nil, lastErr
}

// FetchRecent 获取最近 days 个交易日的龙虎榜数据（跳过周末）
func FetchRecent(days int) ([]LHBRecord, error) {
	var allRecords []LHBRecord
	checked := 0
	fetched := 0
	current := time.Now()

	for fetched < days && checked < days*3 {
		checked++
		current = current.AddDate(0, 0, -1)

		// 跳过周末
		wd := current.Weekday()
		if wd == time.Saturday || wd == time.Sunday {
			continue
		}

		dateStr := current.Format("2006-01-02")
		records, err := FetchByDate(dateStr)
		if err != nil {
			fmt.Printf("[龙虎榜] %s 获取失败: %v\n", dateStr, err)
			continue
		}
		if len(records) > 0 {
			allRecords = append(allRecords, records...)
			fetched++
		}
	}

	return allRecords, nil
}
