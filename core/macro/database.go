package macro

import (
	"database/sql"
	"fmt"
)

// UpsertRMBLoan 增量写入人民币贷款数据，返回写入/更新条数
func UpsertRMBLoan(db *sql.DB, data []RMBLoanData) (int, error) {
	query := `
		INSERT INTO macro_rmb_loan (report_date, time_label, new_loan, loan_yoy, loan_seq, accumulate, acc_yoy, source)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (report_date) DO UPDATE SET
			time_label = EXCLUDED.time_label,
			new_loan   = EXCLUDED.new_loan,
			loan_yoy   = EXCLUDED.loan_yoy,
			loan_seq   = EXCLUDED.loan_seq,
			accumulate = EXCLUDED.accumulate,
			acc_yoy    = EXCLUDED.acc_yoy,
			source     = EXCLUDED.source
	`

	total := 0
	var lastErr error
	for _, d := range data {
		result, err := db.Exec(query,
			d.ReportDate,
			d.Time,
			d.NewLoan,
			d.LoanYoY,
			d.LoanSeq,
			d.Accumulate,
			d.AccYoY,
			"eastmoney",
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

// LoadRMBLoanFromDB 从DB加载人民币贷款数据（时间正序）
// limit=0 表示返回全部
func LoadRMBLoanFromDB(db *sql.DB, limit int) ([]RMBLoanData, error) {
	var rows *sql.Rows
	var err error

	if limit > 0 {
		rows, err = db.Query(`
			SELECT report_date, time_label, new_loan, loan_yoy, loan_seq, accumulate, acc_yoy
			FROM macro_rmb_loan
			ORDER BY report_date DESC
			LIMIT $1
		`, limit)
	} else {
		rows, err = db.Query(`
			SELECT report_date, time_label, new_loan, loan_yoy, loan_seq, accumulate, acc_yoy
			FROM macro_rmb_loan
			ORDER BY report_date DESC
		`)
	}
	if err != nil {
		return nil, fmt.Errorf("查询人民币贷款数据失败：%w", err)
	}
	defer rows.Close()

	var result []RMBLoanData
	for rows.Next() {
		var d RMBLoanData
		var reportDate string
		err := rows.Scan(&reportDate, &d.Time, &d.NewLoan, &d.LoanYoY, &d.LoanSeq, &d.Accumulate, &d.AccYoY)
		if err != nil {
			return nil, fmt.Errorf("扫描人民币贷款数据失败：%w", err)
		}
		d.ReportDate = reportDate
		result = append(result, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历人民币贷款数据失败：%w", err)
	}

	// 反转切片，保证时间正序（ASC）
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result, nil
}
