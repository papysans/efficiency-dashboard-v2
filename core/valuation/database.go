package valuation

import (
	"database/sql"
	"fmt"
	"sort"
	"time"
)

// UpsertValuation 增量写入历史估值数据
func UpsertValuation(db *sql.DB, records []ValuationRecord) error {
	for _, r := range records {
		_, err := db.Exec(`
			INSERT INTO stock_valuation 
			(company_id, trade_date, pe_ttm, pe_lar, pb_mrq, ps_ttm, pcf_ocf_ttm, total_market_cap)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (company_id, trade_date) DO UPDATE SET
				pe_ttm = EXCLUDED.pe_ttm,
				pe_lar = EXCLUDED.pe_lar,
				pb_mrq = EXCLUDED.pb_mrq,
				ps_ttm = EXCLUDED.ps_ttm,
				pcf_ocf_ttm = EXCLUDED.pcf_ocf_ttm,
				total_market_cap = EXCLUDED.total_market_cap`,
			r.CompanyID, r.TradeDate, r.PETTM, r.PELar, r.PBMRQ, r.PSTTM, r.PCFOcfTTM, r.TotalMarketCap,
		)
		if err != nil {
			return fmt.Errorf("写入估值数据失败 [%s %s]: %w", r.CompanyID, r.TradeDate.Format("2006-01-02"), err)
		}
	}
	return nil
}

// LoadValuationHistory 从数据库查询历史估值
func LoadValuationHistory(db *sql.DB, companyID string, days int) ([]ValuationRecord, error) {
	rows, err := db.Query(`
		SELECT company_id, trade_date, pe_ttm, pe_lar, pb_mrq, ps_ttm, pcf_ocf_ttm, total_market_cap
		FROM stock_valuation
		WHERE company_id = $1
		ORDER BY trade_date DESC
		LIMIT $2`, companyID, days)
	if err != nil {
		return nil, fmt.Errorf("查询估值历史失败: %w", err)
	}
	defer rows.Close()

	var records []ValuationRecord
	for rows.Next() {
		var r ValuationRecord
		if err := rows.Scan(&r.CompanyID, &r.TradeDate, &r.PETTM, &r.PELar, &r.PBMRQ, &r.PSTTM, &r.PCFOcfTTM, &r.TotalMarketCap); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, nil
}

// GetLatestValuationDate 查询DB中某公司最新估值日期，供增量判断使用
// 若DB中无数据（MAX返回NULL），返回零值 time.Time{} 和 nil error
func GetLatestValuationDate(db *sql.DB, companyID string) (time.Time, error) {
	var t *time.Time
	err := db.QueryRow(`SELECT MAX(trade_date) FROM stock_valuation WHERE company_id = $1`, companyID).Scan(&t)
	if err != nil {
		return time.Time{}, fmt.Errorf("查询最新估值日期失败: %w", err)
	}
	if t == nil {
		return time.Time{}, nil
	}
	return *t, nil
}

// CalcValuationStats 计算估值统计（历史分位数）
func CalcValuationStats(records []ValuationRecord) ValuationStats {
	if len(records) == 0 {
		return ValuationStats{}
	}

	// 确保 records[0] 是最新的
	sorted := make([]ValuationRecord, len(records))
	copy(sorted, records)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].TradeDate.After(sorted[j].TradeDate)
	})

	stats := ValuationStats{
		Current: sorted[0],
	}

	// 1年≈250交易日，3年≈750，5年≈1250
	stats.PEPct1Y = CalcPercentile(sorted, "pe", 250)
	stats.PEPct3Y = CalcPercentile(sorted, "pe", 750)
	stats.PEPct5Y = CalcPercentile(sorted, "pe", 1250)
	stats.PBPct1Y = CalcPercentile(sorted, "pb", 250)
	stats.PBPct3Y = CalcPercentile(sorted, "pb", 750)
	stats.PBPct5Y = CalcPercentile(sorted, "pb", 1250)
	stats.PSPct1Y = CalcPercentile(sorted, "ps", 250)
	stats.PSPct3Y = CalcPercentile(sorted, "ps", 750)
	stats.PSPct5Y = CalcPercentile(sorted, "ps", 1250)

	return stats
}
