package sentiment

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"comdigger/core/httputil"
)

// FinIndex 东方财富核心财务指标
type FinIndex struct {
	StockCode      string  `json:"stock_code"`
	ReportDate     string  `json:"report_date"`
	ReportType     string  `json:"report_type"`
	EPS            float64 `json:"eps"`              // 基本每股收益
	ROEWtd         float64 `json:"roe_wtd"`          // 加权ROE(%)
	ROA            float64 `json:"roa"`              // 总资产收益率(%)
	GrossMargin    float64 `json:"gross_margin"`     // 销售毛利率(%)
	NetProfit      float64 `json:"net_profit"`       // 归母净利润(元)
	Revenue        float64 `json:"revenue"`          // 营业总收入(元)
	AssetLiabRatio float64 `json:"asset_liab_ratio"` // 资产负债率(%)
	CurrRatio      float64 `json:"curr_ratio"`       // 流动比率
}

// FetchFinIndex 获取东方财富核心财务指标
// stockCode: 纯代码如 "300454"
// market: 大写市场如 "SZ"/"SH"
// reportType: "年报"/"中报"/"三季报"/"一季报"，为空时不过滤
func FetchFinIndex(stockCode, market, reportType string) ([]FinIndex, error) {
	secuCode := fmt.Sprintf("%s.%s", stockCode, strings.ToUpper(market))

	// 双引号用 %22 URL 编码，避免 PowerShell 单引号字符串中的解析问题
	filterStr := fmt.Sprintf("(SECUCODE=%%22%s%%22)", secuCode)
	if reportType != "" {
		filterStr += fmt.Sprintf("(REPORT_TYPE=%%22%s%%22)", reportType)
	}

	url := fmt.Sprintf(
		"https://datacenter.eastmoney.com/securities/api/data/get?type=RPT_F10_FINANCE_MAINFINADATA&sty=APP_F10_MAINFINADATA&filter=%s&p=1&ps=20&sr=-1&st=REPORT_DATE&source=HSF10&client=PC",
		filterStr,
	)

	body, err := httputil.FetchURL(context.Background(), url, emHeaders())
	if err != nil {
		return nil, fmt.Errorf("获取财务指标失败: %w", err)
	}

	var result struct {
		Result struct {
			Data []struct {
				SecurityCode     string   `json:"SECURITY_CODE"`
				ReportDate       string   `json:"REPORT_DATE"`
				ReportType       string   `json:"REPORT_TYPE"`
				EPSJB            *float64 `json:"EPSJB"`            // 基本每股收益
				ROEJQ            *float64 `json:"ROEJQ"`            // 加权ROE
				ZZCJLL           *float64 `json:"ZZCJLL"`           // 总资产净利率(ROA)
				XSMLL            *float64 `json:"XSMLL"`            // 销售毛利率
				PARENTNETPROFIT  *float64 `json:"PARENTNETPROFIT"`  // 归母净利润
				TOTALOPERATEREVE *float64 `json:"TOTALOPERATEREVE"` // 营业总收入
				ZCFZL            *float64 `json:"ZCFZL"`            // 资产负债率
				LD               *float64 `json:"LD"`               // 流动比率
			} `json:"data"`
		} `json:"result"`
		Success bool `json:"success"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析财务指标响应失败: %w", err)
	}

	var indexes []FinIndex
	for _, item := range result.Result.Data {
		reportDate := item.ReportDate
		if len(reportDate) > 10 {
			reportDate = reportDate[:10]
		}
		indexes = append(indexes, FinIndex{
			StockCode:      item.SecurityCode,
			ReportDate:     reportDate,
			ReportType:     item.ReportType,
			EPS:            derefFloat(item.EPSJB),
			ROEWtd:         derefFloat(item.ROEJQ),
			ROA:            derefFloat(item.ZZCJLL),
			GrossMargin:    derefFloat(item.XSMLL),
			NetProfit:      derefFloat(item.PARENTNETPROFIT),
			Revenue:        derefFloat(item.TOTALOPERATEREVE),
			AssetLiabRatio: derefFloat(item.ZCFZL),
			CurrRatio:      derefFloat(item.LD),
		})
	}
	return indexes, nil
}

// SaveFinIndexToDB 保存财务指标到数据库
func SaveFinIndexToDB(db *sql.DB, data []FinIndex) error {
	if len(data) == 0 {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback()

	for _, item := range data {
		_, err := tx.Exec(`
			INSERT INTO sentiment_fin_index
			(stock_code, report_date, report_type, eps, roe_wtd, roa, gross_margin,
			 net_profit, revenue, asset_liab_ratio, curr_ratio)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
			ON CONFLICT ON CONSTRAINT sentiment_fin_index_unique DO UPDATE SET
				eps=EXCLUDED.eps, roe_wtd=EXCLUDED.roe_wtd, roa=EXCLUDED.roa,
				gross_margin=EXCLUDED.gross_margin, net_profit=EXCLUDED.net_profit,
				revenue=EXCLUDED.revenue, asset_liab_ratio=EXCLUDED.asset_liab_ratio,
				curr_ratio=EXCLUDED.curr_ratio
		`, item.StockCode, item.ReportDate, item.ReportType, item.EPS, item.ROEWtd,
			item.ROA, item.GrossMargin, item.NetProfit, item.Revenue,
			item.AssetLiabRatio, item.CurrRatio)
		if err != nil {
			return fmt.Errorf("保存财务指标失败: %w", err)
		}
	}
	return tx.Commit()
}
