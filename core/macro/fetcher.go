package macro

import (
	"context"
	"encoding/json"
	"fmt"

	"comdigger/core/httputil"
)

// fetchMacroData 通用宏观数据获取函数
func fetchMacroData(reportName string, pageSize int) ([]map[string]interface{}, error) {
	url := fmt.Sprintf(
		"https://datacenter-web.eastmoney.com/api/data/v1/get?reportName=%s&columns=ALL&pageNumber=1&pageSize=%d&sortTypes=-1&sortColumns=REPORT_DATE",
		reportName, pageSize,
	)
	body, err := httputil.FetchURL(context.Background(), url, map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Referer":    "https://finance.eastmoney.com/",
	})
	if err != nil {
		return nil, fmt.Errorf("获取 %s 失败: %w", reportName, err)
	}

	var resp struct {
		Result *struct {
			Data []map[string]interface{} `json:"data"`
		} `json:"result"`
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("解析 %s 响应失败: %w", reportName, err)
	}
	if resp.Result == nil || len(resp.Result.Data) == 0 {
		return []map[string]interface{}{}, nil
	}
	return resp.Result.Data, nil
}

// extractFloat 从map中安全提取float64值
func extractFloat(m map[string]interface{}, key string) float64 {
	v, ok := m[key]
	if !ok {
		return 0
	}
	if f, ok := v.(float64); ok {
		return f
	}
	return 0
}

// extractString 从map中安全提取string值
func extractString(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// parseReportDate 解析日期时间字符串，截取前10位（YYYY-MM-DD）
// 兼容 "2026-01-01T00:00:00" 和 "2025-12-01 00:00:00" 两种格式
func parseReportDate(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

// FetchGDP 获取GDP数据
func FetchGDP(count int) ([]GDPData, error) {
	rows, err := fetchMacroData("RPT_ECONOMY_GDP", count)
	if err != nil {
		return nil, err
	}
	result := make([]GDPData, 0, len(rows))
	for _, row := range rows {
		result = append(result, GDPData{
			ReportDate: parseReportDate(extractString(row, "REPORT_DATE")),
			TotalGDP:   extractFloat(row, "DOMESTICL_PRODUCT_BASE"),
			GDPYoY:     extractFloat(row, "SUM_SAME"),
			SecondYoY:  extractFloat(row, "SECOND_SAME"),
			ThirdYoY:   extractFloat(row, "THIRD_SAME"),
		})
	}
	return result, nil
}

// FetchCPI 获取CPI数据
func FetchCPI(count int) ([]CPIData, error) {
	rows, err := fetchMacroData("RPT_ECONOMY_CPI", count)
	if err != nil {
		return nil, err
	}
	result := make([]CPIData, 0, len(rows))
	for _, row := range rows {
		result = append(result, CPIData{
			ReportDate:  parseReportDate(extractString(row, "REPORT_DATE")),
			Time:        extractString(row, "TIME"),
			NationalYoY: extractFloat(row, "NATIONAL_SAME"),
			NationalSeq: extractFloat(row, "NATIONAL_SEQUENTIAL"),
		})
	}
	return result, nil
}

// FetchPMI 获取PMI数据
func FetchPMI(count int) ([]PMIData, error) {
	rows, err := fetchMacroData("RPT_ECONOMY_PMI", count)
	if err != nil {
		return nil, err
	}
	result := make([]PMIData, 0, len(rows))
	for _, row := range rows {
		result = append(result, PMIData{
			ReportDate:          parseReportDate(extractString(row, "REPORT_DATE")),
			Time:                extractString(row, "TIME"),
			ManufacturingPMI:    extractFloat(row, "MAKE_INDEX"),
			NonManufacturingPMI: extractFloat(row, "NMAKE_INDEX"),
		})
	}
	return result, nil
}

// FetchPPI 获取PPI数据
func FetchPPI(count int) ([]PPIData, error) {
	rows, err := fetchMacroData("RPT_ECONOMY_PPI", count)
	if err != nil {
		return nil, err
	}
	result := make([]PPIData, 0, len(rows))
	for _, row := range rows {
		result = append(result, PPIData{
			ReportDate: parseReportDate(extractString(row, "REPORT_DATE")),
			Time:       extractString(row, "TIME"),
			PPIYoY:     extractFloat(row, "BASE_SAME"),
		})
	}
	return result, nil
}

// FetchM2 获取M2货币供应数据
func FetchM2(count int) ([]M2Data, error) {
	rows, err := fetchMacroData("RPT_ECONOMY_CURRENCY_SUPPLY", count)
	if err != nil {
		return nil, err
	}
	result := make([]M2Data, 0, len(rows))
	for _, row := range rows {
		result = append(result, M2Data{
			ReportDate: parseReportDate(extractString(row, "REPORT_DATE")),
			Time:       extractString(row, "TIME"),
			M2Yoy:      extractFloat(row, "CURRENCY_SAME"),
			M1Yoy:      extractFloat(row, "FREE_CASH_SAME"),
		})
	}
	return result, nil
}

// FetchRMBLoan 获取人民币贷款数据
func FetchRMBLoan(count int) ([]RMBLoanData, error) {
	rows, err := fetchMacroData("RPT_ECONOMY_RMB_LOAN", count)
	if err != nil {
		return nil, err
	}
	result := make([]RMBLoanData, 0, len(rows))
	for _, row := range rows {
		result = append(result, RMBLoanData{
			ReportDate: parseReportDate(extractString(row, "REPORT_DATE")),
			Time:       extractString(row, "TIME"),
			NewLoan:    extractFloat(row, "RMB_LOAN"),
			LoanYoY:    extractFloat(row, "RMB_LOAN_SAME"),
			LoanSeq:    extractFloat(row, "RMB_LOAN_SEQUENTIAL"),
			Accumulate: extractFloat(row, "RMB_LOAN_ACCUMULATE"),
			AccYoY:     extractFloat(row, "LOAN_ACCUMULATE_SAME"),
		})
	}
	return result, nil
}
