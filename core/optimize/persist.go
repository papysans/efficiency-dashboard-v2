package optimize

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"comdigger/core/technical"
)

// generateJobID 生成优化任务ID：companyID_strategy_YYYYMMDD_HHMMSS
func generateJobID(companyID, strategy string) string {
	return fmt.Sprintf("%s_%s_%s", companyID, strategy, time.Now().Format("20060102_150405"))
}

// saveOptimizeResult 将单个参数组合的回测结果写入 optimize_results 表（幂等写入）
func saveOptimizeResult(db *sql.DB, jobID, companyID, strategy string, params map[string]interface{}, r *technical.BacktestResult) error {
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return err
	}
	if r == nil {
		return nil
	}
	_, err = db.Exec(`INSERT INTO optimize_results
		(job_id, company_id, strategy, params, total_return, annual_return, sharpe_ratio, calmar_ratio, max_drawdown, win_rate, total_trades)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (job_id, params) DO UPDATE SET
			total_return=$5, annual_return=$6, sharpe_ratio=$7, calmar_ratio=$8,
			max_drawdown=$9, win_rate=$10, total_trades=$11`,
		jobID, companyID, strategy, string(paramsJSON),
		r.TotalReturn, r.AnnualReturn, r.SharpeRatio, r.CalmarRatio,
		r.MaxDrawdown, r.WinRate, r.TotalTrades)
	return err
}

// loadLatestJobID 查询指定公司+策略最近一次的 JobID（用于断点续传自动复用）
func loadLatestJobID(db *sql.DB, companyID, strategy string) (string, error) {
	var jobID string
	err := db.QueryRow(`
		SELECT job_id FROM optimize_results
		WHERE company_id=$1 AND strategy=$2
		ORDER BY created_at DESC
		LIMIT 1`, companyID, strategy).Scan(&jobID)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return jobID, err
}

// loadCompletedParams 从 optimize_results 表加载已完成的参数组合集合（用于断点续传）
func loadCompletedParams(db *sql.DB, jobID string) (map[string]bool, error) {
	rows, err := db.Query(`SELECT params FROM optimize_results WHERE job_id=$1`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]bool)
	for rows.Next() {
		var paramsStr string
		if err := rows.Scan(&paramsStr); err != nil {
			continue
		}
		// 规范化 JSON：解析后重新序列化，消除 PostgreSQL JSONB 格式差异（空格、键顺序等）
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(paramsStr), &parsed); err == nil {
			if normalized, err := json.Marshal(parsed); err == nil {
				result[string(normalized)] = true
				continue
			}
		}
		result[paramsStr] = true
	}
	return result, rows.Err()
}

// loadResultsFromDB 从 DB 加载指定 job 的优化结果（用于断点续传全部跳过时展示）
func loadResultsFromDB(db *sql.DB, jobID, sortBy string, topN int) ([]OptimizeResult, error) {
	orderCol := "sharpe_ratio"
	switch sortBy {
	case "total_return":
		orderCol = "total_return"
	case "calmar_ratio":
		orderCol = "calmar_ratio"
	case "annual_return":
		orderCol = "annual_return"
	}
	if topN <= 0 {
		topN = 10
	}
	rows, err := db.Query(fmt.Sprintf(`
		SELECT params, total_return, annual_return, sharpe_ratio, calmar_ratio, max_drawdown, win_rate, total_trades
		FROM optimize_results
		WHERE job_id=$1
		ORDER BY %s DESC NULLS LAST
		LIMIT $2`, orderCol), jobID, topN)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []OptimizeResult
	for rows.Next() {
		var paramsStr string
		var r technical.BacktestResult
		var totalReturn, annualReturn, sharpe, calmar, maxDD, winRate sql.NullFloat64
		var totalTrades sql.NullInt64
		if err := rows.Scan(&paramsStr, &totalReturn, &annualReturn, &sharpe, &calmar, &maxDD, &winRate, &totalTrades); err != nil {
			continue
		}
		if totalReturn.Valid {
			r.TotalReturn = totalReturn.Float64
		}
		if annualReturn.Valid {
			r.AnnualReturn = annualReturn.Float64
		}
		if sharpe.Valid {
			r.SharpeRatio = sharpe.Float64
		}
		if calmar.Valid {
			r.CalmarRatio = calmar.Float64
		}
		if maxDD.Valid {
			r.MaxDrawdown = maxDD.Float64
		}
		if winRate.Valid {
			r.WinRate = winRate.Float64
		}
		if totalTrades.Valid {
			r.TotalTrades = int(totalTrades.Int64)
		}
		var params map[string]interface{}
		_ = json.Unmarshal([]byte(paramsStr), &params)
		results = append(results, OptimizeResult{
			Params: params,
			Result: &r,
			SortBy: sortBy,
		})
	}
	return results, rows.Err()
}
