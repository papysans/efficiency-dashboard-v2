package aiagent

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// SaveAnalysis 将AI分析结果持久化到 ai_analysis_records 表
// 注意：现有表使用 symbol 字段（而非 company_id），与 companyID 对应
func SaveAnalysis(db *sql.DB, report *AnalysisReport) error {
	agentsJSON, err := json.Marshal(report.AgentResults)
	if err != nil {
		return fmt.Errorf("序列化分析师结果失败: %w", err)
	}

	decisionJSON, err := json.Marshal(report.Decision)
	if err != nil {
		return fmt.Errorf("序列化决策结果失败: %w", err)
	}

	analysisDate := report.AnalysisDate.Format("2006-01-02")

	query := `
		INSERT INTO ai_analysis_records (symbol, stock_name, analysis_date, agents_results, discussion_result, final_decision)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (symbol, analysis_date) DO UPDATE SET
			stock_name        = EXCLUDED.stock_name,
			agents_results    = EXCLUDED.agents_results,
			discussion_result = EXCLUDED.discussion_result,
			final_decision    = EXCLUDED.final_decision,
			created_at        = CURRENT_TIMESTAMP
	`

	_, err = db.Exec(query,
		report.CompanyID,
		report.StockName,
		analysisDate,
		string(agentsJSON),
		report.Discussion,
		string(decisionJSON),
	)
	if err != nil {
		return fmt.Errorf("写入AI分析记录失败: %w", err)
	}
	return nil
}

// LoadLatestAnalysis 加载指定公司的最新AI分析结果
func LoadLatestAnalysis(db *sql.DB, companyID string) (*AnalysisReport, error) {
	query := `
		SELECT symbol, stock_name, analysis_date, agents_results, discussion_result, final_decision
		FROM ai_analysis_records
		WHERE symbol = $1
		ORDER BY analysis_date DESC
		LIMIT 1
	`

	row := db.QueryRow(query, companyID)

	var symbol, stockName string
	var analysisDate sql.NullTime
	var agentsJSON, discussion, decisionJSON sql.NullString

	err := row.Scan(&symbol, &stockName, &analysisDate, &agentsJSON, &discussion, &decisionJSON)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询AI分析记录失败: %w", err)
	}

	report := &AnalysisReport{
		CompanyID: symbol,
		StockName: stockName,
	}
	if analysisDate.Valid {
		report.AnalysisDate = analysisDate.Time
	}
	if discussion.Valid {
		report.Discussion = discussion.String
	}

	// 解析分析师结果
	if agentsJSON.Valid && agentsJSON.String != "" {
		var results []AgentResult
		if err := json.Unmarshal([]byte(agentsJSON.String), &results); err == nil {
			report.AgentResults = results
		}
	}

	// 解析决策
	if decisionJSON.Valid && decisionJSON.String != "" {
		var decision FinalDecision
		if err := json.Unmarshal([]byte(decisionJSON.String), &decision); err == nil {
			report.Decision = &decision
		}
	}

	return report, nil
}
