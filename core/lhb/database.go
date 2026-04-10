package lhb

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// SaveRecords 将龙虎榜原始数据 Upsert 到 longhubang_records 表
func SaveRecords(db *sql.DB, records []LHBRecord) error {
	query := `
		INSERT INTO longhubang_records 
			(report_date, stock_code, stock_name, youzi_name, yingye_bu, list_type, buy_amount, sell_amount, net_amount, concepts)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (report_date, stock_code, youzi_name, yingye_bu) DO UPDATE SET
			buy_amount  = EXCLUDED.buy_amount,
			sell_amount = EXCLUDED.sell_amount,
			net_amount  = EXCLUDED.net_amount,
			list_type   = EXCLUDED.list_type,
			concepts    = EXCLUDED.concepts
	`

	var lastErr error
	count := 0
	for _, r := range records {
		// 解析日期
		date, err := time.Parse("2006-01-02", r.Date)
		if err != nil {
			lastErr = fmt.Errorf("日期格式错误 [%s]: %w", r.Date, err)
			continue
		}

		_, err = db.Exec(query,
			date,
			r.Symbol,
			r.Name,
			r.YouziName,
			r.YYB,
			r.ListType,
			r.BuyAmount,
			r.SellAmount,
			r.NetInflow,
			r.Concepts,
		)
		if err != nil {
			lastErr = fmt.Errorf("写入龙虎榜记录失败 [%s %s]: %w", r.Date, r.Symbol, err)
			continue
		}
		count++
	}

	if lastErr != nil {
		return fmt.Errorf("部分写入失败（成功%d条）: %w", count, lastErr)
	}
	return nil
}

// SaveScores 将龙虎榜评分结果 Upsert 到 longhubang_scores 表
func SaveScores(db *sql.DB, date string, scores []StockScore) error {
	query := `
		INSERT INTO longhubang_scores
			(report_date, stock_code, stock_name, total_score, quality_score, inflow_score, sell_score, institution_score, bonus_score, top_youzi_names, has_institution, seat_count)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (report_date, stock_code) DO UPDATE SET
			stock_name        = EXCLUDED.stock_name,
			total_score       = EXCLUDED.total_score,
			quality_score     = EXCLUDED.quality_score,
			inflow_score      = EXCLUDED.inflow_score,
			sell_score        = EXCLUDED.sell_score,
			institution_score = EXCLUDED.institution_score,
			bonus_score       = EXCLUDED.bonus_score,
			top_youzi_names   = EXCLUDED.top_youzi_names,
			has_institution   = EXCLUDED.has_institution,
			seat_count        = EXCLUDED.seat_count
	`

	reportDate, err := time.Parse("2006-01-02", date)
	if err != nil {
		return fmt.Errorf("日期格式错误: %w", err)
	}

	var lastErr error
	count := 0
	for _, s := range scores {
		topYouziStr := strings.Join(s.TopYouziNames, ",")
		_, err := db.Exec(query,
			reportDate,
			s.Symbol,
			s.Name,
			s.TotalScore,
			s.QualityScore,
			s.InflowScore,
			s.SellScore,
			s.InstitutionScore,
			s.BonusScore,
			topYouziStr,
			s.HasInstitution,
			s.SeatCount,
		)
		if err != nil {
			lastErr = fmt.Errorf("写入评分失败 [%s]: %w", s.Symbol, err)
			continue
		}
		count++
	}

	if lastErr != nil {
		return fmt.Errorf("部分写入失败（成功%d条）: %w", count, lastErr)
	}
	return nil
}

// LoadRecordsByDate 从数据库加载指定日期的龙虎榜记录
func LoadRecordsByDate(db *sql.DB, date string) ([]LHBRecord, error) {
	query := `
		SELECT report_date, stock_code, stock_name, youzi_name, yingye_bu, list_type, buy_amount, sell_amount, net_amount, concepts
		FROM longhubang_records
		WHERE report_date = $1
		ORDER BY net_amount DESC
	`

	rows, err := db.Query(query, date)
	if err != nil {
		return nil, fmt.Errorf("查询龙虎榜数据失败: %w", err)
	}
	defer rows.Close()

	var records []LHBRecord
	for rows.Next() {
		var r LHBRecord
		var reportDate time.Time
		err := rows.Scan(
			&reportDate, &r.Symbol, &r.Name, &r.YouziName, &r.YYB,
			&r.ListType, &r.BuyAmount, &r.SellAmount, &r.NetInflow, &r.Concepts,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描龙虎榜数据失败: %w", err)
		}
		r.Date = reportDate.Format("2006-01-02")
		records = append(records, r)
	}
	return records, rows.Err()
}

// LoadScoresByDate 从数据库加载指定日期的评分结果
func LoadScoresByDate(db *sql.DB, date string) ([]StockScore, error) {
	query := `
		SELECT stock_code, stock_name, total_score, quality_score, inflow_score, sell_score, institution_score, bonus_score, top_youzi_names, has_institution, seat_count
		FROM longhubang_scores
		WHERE report_date = $1
		ORDER BY total_score DESC
	`

	rows, err := db.Query(query, date)
	if err != nil {
		return nil, fmt.Errorf("查询评分数据失败: %w", err)
	}
	defer rows.Close()

	var scores []StockScore
	for rows.Next() {
		var s StockScore
		var topYouziStr string
		err := rows.Scan(
			&s.Symbol, &s.Name, &s.TotalScore, &s.QualityScore,
			&s.InflowScore, &s.SellScore, &s.InstitutionScore, &s.BonusScore,
			&topYouziStr, &s.HasInstitution, &s.SeatCount,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描评分数据失败: %w", err)
		}
		if topYouziStr != "" {
			s.TopYouziNames = strings.Split(topYouziStr, ",")
		}
		scores = append(scores, s)
	}
	return scores, rows.Err()
}
