package kline

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"comdigger/core/httputil"
)

// GetPriceTencent 从腾讯获取K线数据
func GetPriceTencent(code string, count int, freq string) ([]KlineBar, error) {
	unitMap := map[string]string{
		FreqDay:   "day",
		FreqWeek:  "week",
		FreqMonth: "month",
	}
	unit, ok := unitMap[freq]
	if !ok {
		unit = "day"
	}

	url := fmt.Sprintf("http://web.ifzq.gtimg.cn/appstock/app/fqkline/get?param=%s,%s,,,%d,qfq",
		code, unit, count)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败：%w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := httputil.FastClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败：%w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP 状态码：%d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败：%w", err)
	}

	// 使用 interface{} 解析，因为结构动态
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("JSON 解析失败：%w", err)
	}

	dataMap, ok := raw["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("data 字段格式错误")
	}

	stockData, ok := dataMap[code].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("未找到股票代码 %s 的数据", code)
	}

	// 优先使用前复权数据
	qfqKey := "qfq" + unit
	var klineRaw interface{}
	if v, exists := stockData[qfqKey]; exists {
		klineRaw = v
	} else if v, exists := stockData[unit]; exists {
		klineRaw = v
	} else {
		return nil, fmt.Errorf("未找到K线数据")
	}

	klineArr, ok := klineRaw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("K线数据格式错误")
	}

	result := make([]KlineBar, 0, len(klineArr))
	for _, item := range klineArr {
		row, ok := item.([]interface{})
		if !ok || len(row) < 6 {
			continue
		}

		timeStr, ok := row[0].(string)
		if !ok {
			continue
		}
		t, err := parseTencentTime(timeStr)
		if err != nil {
			continue
		}

		getFloat := func(idx int) (float64, error) {
			switch v := row[idx].(type) {
			case float64:
				return v, nil
			case string:
				return strconv.ParseFloat(v, 64)
			default:
				return 0, fmt.Errorf("索引 %d 类型错误：%T", idx, v)
			}
		}

		// 腾讯列顺序：[时间, 开盘, 收盘, 最高, 最低, 成交量]
		open, err := getFloat(1)
		if err != nil {
			continue
		}
		closeVal, err := getFloat(2)
		if err != nil {
			continue
		}
		high, err := getFloat(3)
		if err != nil {
			continue
		}
		low, err := getFloat(4)
		if err != nil {
			continue
		}
		vol, err := getFloat(5)
		if err != nil {
			continue
		}

		result = append(result, KlineBar{
			Time:   t,
			Open:   open,
			High:   high,
			Low:    low,
			Close:  closeVal,
			Volume: int64(vol),
		})
	}

	// 用 qt 字段更新最后一条K线的实时收盘价
	if qtRaw, exists := stockData["qt"]; exists {
		if qtMap, ok := qtRaw.(map[string]interface{}); ok {
			if qtArr, ok := qtMap[code].([]interface{}); ok && len(qtArr) > 3 {
				if priceStr, ok := qtArr[3].(string); ok {
					if latestPrice, err := strconv.ParseFloat(priceStr, 64); err == nil {
						if len(result) > 0 {
							result[len(result)-1].Close = latestPrice
						}
					}
				}
			}
		}
	}

	return result, nil
}

func parseTencentTime(timeStr string) (time.Time, error) {
	timeStr = strings.TrimSpace(timeStr)
	if len(timeStr) == 12 {
		t, err := time.Parse("200601021504", timeStr)
		if err == nil {
			return t, nil
		}
	}
	if len(timeStr) == 8 {
		t, err := time.Parse("20060102", timeStr)
		if err == nil {
			return t, nil
		}
	}
	if len(timeStr) == 10 {
		t, err := time.Parse("2006-01-02", timeStr)
		if err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("不支持的时间格式：%s", timeStr)
}
