package kline

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"comdigger/core/httputil"
)

// RealtimeQuote 实时报价
type RealtimeQuote struct {
	Price     float64 // 现价（元）
	ChangePct float64 // 涨跌幅%
	Volume    int64   // 成交量（手）
}

// GetRealtimeQuote 通过东方财富API获取单股实时价格和涨跌幅
// companyID 格式：sz300454 / sh601318
func GetRealtimeQuote(companyID string) (RealtimeQuote, error) {
	// companyID 转 secid
	var secid string
	if strings.HasPrefix(companyID, "sz") {
		secid = "0." + companyID[2:]
	} else if strings.HasPrefix(companyID, "sh") {
		secid = "1." + companyID[2:]
	} else {
		return RealtimeQuote{}, fmt.Errorf("unsupported companyID prefix: %s", companyID)
	}

	url := fmt.Sprintf("https://push2.eastmoney.com/api/qt/stock/get?secid=%s&fields=f43,f170,f47&cb=jQuery112309_1", secid)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return RealtimeQuote{}, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://quote.eastmoney.com/")

	resp, err := httputil.FastClient.Do(req)
	if err != nil {
		// 非交易时间可能 EOF，静默返回零值
		return RealtimeQuote{}, nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return RealtimeQuote{}, nil
	}
	bodyStr := string(body)

	// 剥离 JSONP 包装
	start := strings.Index(bodyStr, "(")
	end := strings.LastIndex(bodyStr, ")")
	if start < 0 || end <= start {
		return RealtimeQuote{}, nil
	}
	jsonStr := bodyStr[start+1 : end]

	// 解析 JSON
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return RealtimeQuote{}, nil
	}

	dataRaw, ok := raw["data"]
	if !ok || dataRaw == nil {
		// 非交易时间 data 为 null，静默返回零值
		return RealtimeQuote{}, nil
	}
	dataMap, ok := dataRaw.(map[string]interface{})
	if !ok {
		return RealtimeQuote{}, nil
	}

	var quote RealtimeQuote

	// f43: 现价（单位：分，需除以100）
	if v, ok := dataMap["f43"].(float64); ok {
		quote.Price = v / 100.0
	}
	// f170: 涨跌幅（单位：百分比×100，需除以100）
	if v, ok := dataMap["f170"].(float64); ok {
		quote.ChangePct = v / 100.0
	}
	// f47: 成交量（手）
	if v, ok := dataMap["f47"].(float64); ok {
		quote.Volume = int64(v)
	}

	return quote, nil
}
