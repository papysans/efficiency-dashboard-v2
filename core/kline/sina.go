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

// GetPriceSina 从新浪获取K线数据
func GetPriceSina(code string, count int, freq string) ([]KlineBar, error) {
	scaleMap := map[string]int{
		FreqDay:   240,
		FreqWeek:  1200,
		FreqMonth: 7200,
	}
	scale, ok := scaleMap[freq]
	if !ok {
		scale = 240
	}

	url := fmt.Sprintf("http://money.finance.sina.com.cn/quotes_service/api/json_v2.php/CN_MarketData.getKLineData?symbol=%s&scale=%d&ma=5&datalen=%d",
		code, scale, count)

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

	var rawData []map[string]interface{}
	if err := json.Unmarshal(body, &rawData); err != nil {
		return nil, fmt.Errorf("JSON 解析失败：%w", err)
	}

	result := make([]KlineBar, 0, len(rawData))
	for _, item := range rawData {
		dayStr, ok := item["day"].(string)
		if !ok {
			continue
		}
		t, err := parseSinaTime(dayStr)
		if err != nil {
			continue
		}

		getFloat := func(key string) (float64, error) {
			switch v := item[key].(type) {
			case float64:
				return v, nil
			case string:
				return strconv.ParseFloat(v, 64)
			default:
				return 0, fmt.Errorf("%s 字段类型错误：%T", key, v)
			}
		}

		open, err := getFloat("open")
		if err != nil {
			continue
		}
		high, err := getFloat("high")
		if err != nil {
			continue
		}
		low, err := getFloat("low")
		if err != nil {
			continue
		}
		close, err := getFloat("close")
		if err != nil {
			continue
		}
		vol, err := getFloat("volume")
		if err != nil {
			continue
		}

		result = append(result, KlineBar{
			Time:   t,
			Open:   open,
			High:   high,
			Low:    low,
			Close:  close,
			Volume: int64(vol),
		})
	}

	return result, nil
}

func parseSinaTime(dayStr string) (time.Time, error) {
	dayStr = strings.TrimSpace(dayStr)
	if len(dayStr) > 10 {
		t, err := time.Parse("2006-01-02 15:04:05", dayStr)
		if err == nil {
			return t, nil
		}
	}
	t, err := time.Parse("2006-01-02", dayStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("时间格式不支持：%s", dayStr)
	}
	return t, nil
}
