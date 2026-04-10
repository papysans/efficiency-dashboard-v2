package screen

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// SaveScreenResults 批量写入 screen_results 表
func SaveScreenResults(db *sql.DB, results []ScreenResult, params ScreenParams) error {
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("序列化筛选参数失败: %w", err)
	}

	for _, r := range results {
		_, err := db.Exec(`
			INSERT INTO screen_results 
			(company_id, company_name, report_date, roe, net_profit, revenue, gross_margin, pe_ttm, pb_mrq, ps_ttm, cash_flow_ratio, score, screen_params)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
			r.CompanyID, r.CompanyName, r.ReportDate,
			r.ROE, r.NetProfit, r.Revenue, r.GrossMargin,
			r.PETTM, r.PBMRQ, r.PSTTM, r.CashFlowRatio,
			r.Score, string(paramsJSON),
		)
		if err != nil {
			return fmt.Errorf("保存筛选结果失败 [%s]: %w", r.CompanyID, err)
		}
	}
	return nil
}
