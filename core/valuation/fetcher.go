package valuation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"comdigger/core/httputil"
)

type emValuationResp struct {
	Result struct {
		Data  []map[string]interface{} `json:"data"`
		Count int                      `json:"count"`
		Pages int                      `json:"pages"`
	} `json:"result"`
	Success bool `json:"success"`
}

// parseMarketAndCode 将 "sz300454" 拆分为 market前缀数字 和 6位代码
// sz→0, sh→1, bj→0（默认）
func parseMarketAndCode(companyCode string) (marketNum string, code string) {
	lower := strings.ToLower(companyCode)
	if strings.HasPrefix(lower, "sz") {
		return "0", companyCode[2:]
	} else if strings.HasPrefix(lower, "sh") {
		return "1", companyCode[2:]
	} else if strings.HasPrefix(lower, "bj") {
		return "0", companyCode[2:]
	}
	// 纯数字代码
	return "0", companyCode
}

func getFloat(m map[string]interface{}, key string) float64 {
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	if f, ok := v.(float64); ok {
		return f
	}
	return 0
}

func getString(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// FetchValuationHistory 获取个股历史估值数据
// companyCode 格式：sz300454
// days: 获取最近多少天的数据（默认250）
func FetchValuationHistory(companyCode string, days int) ([]ValuationRecord, error) {
	marketNum, code := parseMarketAndCode(companyCode)
	secid := marketNum + "." + code

	var allRecords []ValuationRecord
	page := 1
	pageSize := 100
	maxPages := (days / pageSize) + 2

	for page <= maxPages {
		url := fmt.Sprintf(
			"https://datacenter-web.eastmoney.com/api/data/v1/get?reportName=RPT_VALUEANALYSIS_DET&columns=SECURITY_CODE,TRADE_DATE,PE_TTM,PE_LAR,PB_MRQ,PS_TTM,PCF_OCF_TTM,TOTAL_MARKET_CAP&sortColumns=TRADE_DATE&sortTypes=-1&pageSize=%d&pageNumber=%d&secid=%s&filter=(SECURITY_CODE=%%22%s%%22)",
			pageSize, page, secid, code,
		)

		body, err := httputil.FetchURL(context.Background(), url, map[string]string{
			"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Referer":    "https://finance.eastmoney.com/",
		})
		if err != nil {
			return nil, fmt.Errorf("获取历史估值数据失败: %w", err)
		}

		var resp emValuationResp
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("解析历史估值数据失败: %w", err)
		}

		if !resp.Success || len(resp.Result.Data) == 0 {
			break
		}

		for _, item := range resp.Result.Data {
			dateStr := getString(item, "TRADE_DATE")
			if dateStr == "" {
				continue
			}
			// 日期格式：2024-12-31T00:00:00 或 2024-12-31
			if len(dateStr) > 10 {
				dateStr = dateStr[:10]
			}
			t, err := time.Parse("2006-01-02", dateStr)
			if err != nil {
				continue
			}
			rec := ValuationRecord{
				CompanyID:      companyCode,
				TradeDate:      t,
				PETTM:          getFloat(item, "PE_TTM"),
				PELar:          getFloat(item, "PE_LAR"),
				PBMRQ:          getFloat(item, "PB_MRQ"),
				PSTTM:          getFloat(item, "PS_TTM"),
				PCFOcfTTM:      getFloat(item, "PCF_OCF_TTM"),
				TotalMarketCap: getFloat(item, "TOTAL_MARKET_CAP"),
			}
			allRecords = append(allRecords, rec)
		}

		if len(allRecords) >= days || len(resp.Result.Data) < pageSize {
			break
		}
		page++
	}

	return allRecords, nil
}

// CalcPercentile 计算当前值在历史记录中的百分位（0-100）
// field: "pe", "pb", "ps", "pcf"
// window: 最多使用多少条历史记录
func CalcPercentile(records []ValuationRecord, field string, window int) float64 {
	if len(records) == 0 {
		return 50
	}
	n := len(records)
	if n > window {
		n = window
	}
	subset := records[:n]
	if len(subset) == 0 {
		return 50
	}

	// 当前值（records[0] 是最新的）
	var current float64
	switch field {
	case "pe":
		current = records[0].PETTM
	case "pb":
		current = records[0].PBMRQ
	case "ps":
		current = records[0].PSTTM
	case "pcf":
		current = records[0].PCFOcfTTM
	default:
		return 50
	}

	if current <= 0 {
		return 50
	}

	// 统计有多少历史值低于当前值
	count := 0
	total := 0
	for _, r := range subset {
		var v float64
		switch field {
		case "pe":
			v = r.PETTM
		case "pb":
			v = r.PBMRQ
		case "ps":
			v = r.PSTTM
		case "pcf":
			v = r.PCFOcfTTM
		}
		if v > 0 {
			total++
			if current > v {
				count++
			}
		}
	}
	if total == 0 {
		return 50
	}
	return float64(count) / float64(total) * 100
}
