package kline

import (
	"database/sql"
	"fmt"
	"time"
)

// UpsertKlineData 增量写入K线数据，返回写入/更新条数
func UpsertKlineData(db *sql.DB, companyID string, bars []KlineBar, freq string, source string) (int, error) {
	query := `
		INSERT INTO kline (company_id, trade_date, open, high, low, close, volume, freq, source)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (company_id, trade_date, freq) DO UPDATE SET
			open = EXCLUDED.open,
			high = EXCLUDED.high,
			low = EXCLUDED.low,
			close = EXCLUDED.close,
			volume = EXCLUDED.volume,
			source = EXCLUDED.source
	`

	total := 0
	var lastErr error
	for _, bar := range bars {
		result, err := db.Exec(query,
			companyID,
			bar.Time.Format("2006-01-02"),
			bar.Open,
			bar.High,
			bar.Low,
			bar.Close,
			bar.Volume,
			freq,
			source,
		)
		if err != nil {
			lastErr = err
			continue
		}
		n, _ := result.RowsAffected()
		total += int(n)
	}

	return total, lastErr
}

// LoadKlineFromDB 从DB加载K线数据（时间正序）
func LoadKlineFromDB(db *sql.DB, companyID string, count int, freq string) ([]KlineBar, error) {
	query := `
		SELECT trade_date, open, high, low, close, volume
		FROM kline
		WHERE company_id = $1 AND freq = $2
		ORDER BY trade_date DESC
		LIMIT $3
	`

	rows, err := db.Query(query, companyID, freq, count)
	if err != nil {
		return nil, fmt.Errorf("查询K线数据失败：%w", err)
	}
	defer rows.Close()

	var bars []KlineBar
	for rows.Next() {
		var bar KlineBar
		err := rows.Scan(&bar.Time, &bar.Open, &bar.High, &bar.Low, &bar.Close, &bar.Volume)
		if err != nil {
			return nil, fmt.Errorf("扫描K线数据失败：%w", err)
		}
		bars = append(bars, bar)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历K线数据失败：%w", err)
	}

	// 反转切片，保证时间正序（ASC）
	for i, j := 0, len(bars)-1; i < j; i, j = i+1, j-1 {
		bars[i], bars[j] = bars[j], bars[i]
	}

	return bars, nil
}

// CountKlineInDB 统计DB中已有的K线数据条数
func CountKlineInDB(db *sql.DB, companyID string, freq string) (int, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM kline WHERE company_id = $1 AND freq = $2`, companyID, freq).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("统计K线数据失败：%w", err)
	}
	return count, nil
}

// GetLatestKlineDate 查询DB中某公司最新K线日期，供增量判断使用
// 若DB中无数据（MAX返回NULL），返回零值 time.Time{} 和 nil error
func GetLatestKlineDate(db *sql.DB, companyID string, freq string) (time.Time, error) {
	var t *time.Time
	err := db.QueryRow(`SELECT MAX(trade_date) FROM kline WHERE company_id = $1 AND freq = $2`, companyID, freq).Scan(&t)
	if err != nil {
		return time.Time{}, fmt.Errorf("查询最新K线日期失败：%w", err)
	}
	if t == nil {
		return time.Time{}, nil
	}
	return *t, nil
}
