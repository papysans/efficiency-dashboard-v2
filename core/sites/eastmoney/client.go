package eastmoney

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"comdigger/core/httputil"
	"comdigger/core/infra"
)

// eastmoneyResponse 东方财富 datacenter-web API 响应结构
type eastmoneyResponse struct {
	Result struct {
		Data  []map[string]interface{} `json:"data"`
		Count int                      `json:"count"`
		Pages int                      `json:"pages"`
	} `json:"result"`
	Success bool   `json:"success"`
	Message string `json:"message"`
}

var defaultHeaders = map[string]string{
	"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Referer":    "https://emweb.eastmoney.com/",
}

// FetchIncome 获取利润表数据
func FetchIncome(ctx context.Context, stockCode string) ([]map[string]interface{}, error) {
	return fetchReport(ctx, "RPT_F10_FINANCE_GINCOME", stockCode)
}

// FetchBalance 获取资产负债表数据
func FetchBalance(ctx context.Context, stockCode string) ([]map[string]interface{}, error) {
	return fetchReport(ctx, "RPT_F10_FINANCE_GBALANCE", stockCode)
}

// FetchCashflow 获取现金流量表数据
func FetchCashflow(ctx context.Context, stockCode string) ([]map[string]interface{}, error) {
	return fetchReport(ctx, "RPT_F10_FINANCE_GCASHFLOW", stockCode)
}

// fetchReport 统一请求东方财富 datacenter-web API
func fetchReport(ctx context.Context, reportName, stockCode string) ([]map[string]interface{}, error) {
	var allData []map[string]interface{}
	page := 1

	for {
		url := fmt.Sprintf(
			"https://datacenter-web.eastmoney.com/api/data/v1/get?sortColumns=REPORT_DATE&sortTypes=-1&pageSize=200&pageNumber=%d&reportName=%s&columns=ALL&filter=(SECUCODE=%%22%s%%22)",
			page, reportName, stockCode,
		)

		start := time.Now()
		body, err := httputil.FetchURL(ctx, url, defaultHeaders)
		elapsed := time.Since(start)

		if err != nil {
			return nil, fmt.Errorf("请求东方财富API失败 [%s page=%d]: %w", reportName, page, err)
		}

		infra.Logger.Info("[eastmoney] %s page=%d 耗时=%v", reportName, page, elapsed)

		var resp eastmoneyResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			preview := string(body)
			if len(preview) > 200 {
				preview = preview[:200]
			}
			return nil, fmt.Errorf("解析东方财富响应失败 [%s]: %w, body=%s", reportName, err, preview)
		}

		if !resp.Success {
			return nil, fmt.Errorf("东方财富API返回失败 [%s]: %s", reportName, resp.Message)
		}

		if resp.Result.Data == nil {
			infra.Logger.Info("[eastmoney] %s 返回数据为空", reportName)
			return []map[string]interface{}{}, nil
		}

		allData = append(allData, resp.Result.Data...)
		infra.Logger.Info("[eastmoney] %s page=%d 返回%d条，共%d页", reportName, page, len(resp.Result.Data), resp.Result.Pages)

		if page >= resp.Result.Pages {
			break
		}
		page++
	}

	return allData, nil
}
