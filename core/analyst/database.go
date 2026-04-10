package analyst

import (
	"database/sql"
	"fmt"
	"time"
)

// SaveReports 批量写入研报数据（按 info_code 去重）
func SaveReports(db *sql.DB, reports []AnalystReport) error {
	for _, r := range reports {
		if r.InfoCode == "" {
			continue
		}
		_, err := db.Exec(`
			INSERT INTO analyst_reports 
			(company_id, stock_code, stock_name, org_name, publish_date, title, rating_name, rating_value,
			 predict_this_year_eps, predict_next_year_eps, predict_this_year_pe, predict_next_year_pe, info_code)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
			ON CONFLICT (info_code) DO NOTHING`,
			r.CompanyID, r.StockCode, r.StockName, r.OrgName, r.PublishDate,
			r.Title, r.RatingName, r.RatingValue,
			r.PredictThisYearEPS, r.PredictNextYearEPS, r.PredictThisYearPE, r.PredictNextYearPE,
			r.InfoCode,
		)
		if err != nil {
			return fmt.Errorf("保存研报失败 [%s]: %w", r.InfoCode, err)
		}
	}
	return nil
}

// LoadRecentReports 查询最近 days 天的研报
func LoadRecentReports(db *sql.DB, companyID string, days int) ([]AnalystReport, error) {
	since := time.Now().AddDate(0, 0, -days)
	rows, err := db.Query(`
		SELECT company_id, stock_code, stock_name, org_name, publish_date, title, 
		       rating_name, rating_value, predict_this_year_eps, predict_next_year_eps,
		       predict_this_year_pe, predict_next_year_pe, info_code
		FROM analyst_reports
		WHERE company_id = $1 AND publish_date >= $2
		ORDER BY publish_date DESC`, companyID, since)
	if err != nil {
		return nil, fmt.Errorf("查询研报失败: %w", err)
	}
	defer rows.Close()

	var reports []AnalystReport
	for rows.Next() {
		var r AnalystReport
		if err := rows.Scan(
			&r.CompanyID, &r.StockCode, &r.StockName, &r.OrgName, &r.PublishDate,
			&r.Title, &r.RatingName, &r.RatingValue,
			&r.PredictThisYearEPS, &r.PredictNextYearEPS, &r.PredictThisYearPE, &r.PredictNextYearPE,
			&r.InfoCode,
		); err != nil {
			return nil, err
		}
		reports = append(reports, r)
	}
	return reports, nil
}
