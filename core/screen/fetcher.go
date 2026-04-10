package screen

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"comdigger/core/httputil"
)

// eastmoneyResponse 东方财富API响应结构
// 注意：东方财富 datacenter-web 接口的顶层字段是 "result"，不是 "data"
type eastmoneyResponse struct {
	Result struct {
		Data  []map[string]interface{} `json:"data"`
		Count int                      `json:"count"`
		Pages int                      `json:"pages"`
	} `json:"result"`
	Success bool `json:"success"`
}

// convertCompanyID 将 "300454.SZ" 格式转为 "sz300454"
func convertCompanyID(secuCode string) string {
	parts := strings.Split(secuCode, ".")
	if len(parts) != 2 {
		return strings.ToLower(secuCode)
	}
	code := parts[0]
	market := strings.ToLower(parts[1])
	return market + code
}

// getFloat 从 map 中安全获取 float64
func getFloat(m map[string]interface{}, key string) float64 {
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	}
	return 0
}

// getString 从 map 中安全获取 string
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

// FetchFinancialScreen 从东方财富获取全市场财务数据
// 只拉取沪深A股（主板+创业板+科创板），过滤ST/北交所/三板
func FetchFinancialScreen(params ScreenParams) ([]ScreenResult, error) {
	var allResults []ScreenResult
	page := 1
	pageSize := 100
	// 沪主板+深创业板+深主板+科创板 市场代码
	// TRADE_MARKET_ZJG: 0101=沪A, 0102=深A(含创业板), 0103=科创板
	// 使用 %22 URL编码双引号（在 PowerShell 单引号字符串中安全传递）
	marketFilter := `(TRADE_MARKET_ZJG+in+(%220101%22,%220102%22,%220103%22))`

	for {
		url := fmt.Sprintf(
			"https://datacenter-web.eastmoney.com/api/data/v1/get?reportName=RPT_LICO_FN_CPD&columns=SECURITY_CODE,SECUCODE,SECURITY_NAME_ABBR,WEIGHTAVG_ROE,PARENT_NETPROFIT,TOTAL_OPERATE_INCOME,XSMLL,YSTZ,SJLTZ,MGJYXJJE,QDATE,ISNEW,TRADE_MARKET_ZJG&sortColumns=WEIGHTAVG_ROE&sortTypes=-1&pageSize=%d&pageNumber=%d&filter=(ISNEW=%%221%%22)%s",
			pageSize, page, marketFilter,
		)

		body, err := httputil.FetchURL(context.Background(), url, map[string]string{
			"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Referer":    "https://finance.eastmoney.com/",
		})
		if err != nil {
			return nil, fmt.Errorf("获取财务筛选数据失败: %w", err)
		}

		var resp eastmoneyResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("解析财务筛选数据失败: %w", err)
		}

		if !resp.Success || resp.Result.Data == nil {
			break
		}

		for _, item := range resp.Result.Data {
			name := getString(item, "SECURITY_NAME_ABBR")
			// 过滤ST股票
			if strings.Contains(strings.ToUpper(name), "ST") {
				continue
			}

			secuCode := getString(item, "SECUCODE")
			if secuCode == "" {
				secuCode = getString(item, "SECURITY_CODE")
			}
			result := ScreenResult{
				CompanyID:       convertCompanyID(secuCode),
				CompanyName:     name,
				ReportDate:      getString(item, "QDATE"),
				ROE:             getFloat(item, "WEIGHTAVG_ROE"),
				NetProfit:       getFloat(item, "PARENT_NETPROFIT"),
				Revenue:         getFloat(item, "TOTAL_OPERATE_INCOME"),
				GrossMargin:     getFloat(item, "XSMLL"),
				RevenueGrowth:   getFloat(item, "YSTZ"),
				NetProfitGrowth: getFloat(item, "SJLTZ"),
				CashFlowRatio:   getFloat(item, "MGJYXJJE"),
			}
			allResults = append(allResults, result)
		}

		// 拉取足够多的数据用于后续过滤（params.TopN * 10）
		if len(allResults) >= params.TopN || len(resp.Result.Data) < pageSize {
			break
		}
		page++
	}

	// 如果需要PE/PB过滤，批量获取估值数据
	if params.MaxPE > 0 || params.MaxPB > 0 {
		allResults = enrichWithValuation(allResults)
	}

	return allResults, nil
}

// enrichWithValuation 批量获取PE/PB数据（今日估值）
// 通过 RPT_VALUEANALYSIS_DET 接口，按市场获取最新估值
func enrichWithValuation(results []ScreenResult) []ScreenResult {
	if len(results) == 0 {
		return results
	}

	// 构建代码→索引映射
	codeIndex := make(map[string]int)
	for i, r := range results {
		// 从 sz300454 提取 300454
		code := r.CompanyID
		if len(code) > 2 {
			code = code[2:]
		}
		codeIndex[code] = i
	}

	// 分批获取估值（每批100个）
	codes := make([]string, 0, len(results))
	for code := range codeIndex {
		codes = append(codes, code)
	}

	batchSize := 100
	for i := 0; i < len(codes); i += batchSize {
		end := i + batchSize
		if end > len(codes) {
			end = len(codes)
		}
		batch := codes[i:end]

		// 构建批量过滤条件
		var filterParts []string
		for _, code := range batch {
			filterParts = append(filterParts, fmt.Sprintf("%%22%s%%22", code))
		}
		filterStr := fmt.Sprintf("(SECURITY_CODE+in+(%s))", strings.Join(filterParts, ","))

		url := fmt.Sprintf(
			"https://datacenter-web.eastmoney.com/api/data/v1/get?reportName=RPT_VALUEANALYSIS_DET&columns=SECURITY_CODE,PE_TTM,PB_MRQ,PS_TTM&sortColumns=TRADE_DATE&sortTypes=-1&pageSize=%d&pageNumber=1&filter=%s",
			batchSize, filterStr,
		)

		body, err := httputil.FetchURL(context.Background(), url, map[string]string{
			"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Referer":    "https://finance.eastmoney.com/",
		})
		if err != nil {
			continue
		}

		var resp eastmoneyResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			continue
		}

		if !resp.Success || resp.Result.Data == nil {
			continue
		}

		// 填充PE/PB（每个代码只取第一条，即最新）
		seen := make(map[string]bool)
		for _, item := range resp.Result.Data {
			code := getString(item, "SECURITY_CODE")
			if seen[code] {
				continue
			}
			seen[code] = true
			if idx, ok := codeIndex[code]; ok {
				results[idx].PETTM = getFloat(item, "PE_TTM")
				results[idx].PBMRQ = getFloat(item, "PB_MRQ")
				results[idx].PSTTM = getFloat(item, "PS_TTM")
			}
		}
	}

	return results
}
