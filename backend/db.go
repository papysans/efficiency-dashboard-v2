package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

type rowScanner interface {
	Scan(dest ...interface{}) error
}

// InitDB 初始化 PostgreSQL 数据库连接
func InitDB(cfg DatabaseConfig) (*sql.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开数据库连接失败: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("数据库连接验证失败: %w", err)
	}
	return db, nil
}

// ProjectMetrics 项目维度指标
type ProjectMetrics struct {
	ID                       int
	ProjectID                string
	AnalysisDate             time.Time
	QueryStartDate           time.Time
	QueryEndDate             time.Time
	RawAIEstimatedDays       *float64
	RawTotalCost             *float64
	RawTotalCodeLines        *int64
	RawTaskCount             *int
	CorrectedAIEstimatedDays *float64
	CorrectionReason         *string
	CorrectedBy              *string
	CorrectedAt              *time.Time
	ActualStartTime          *time.Time
	ActualEndTime            *time.Time
	TotalLeadTimeMs          *int64
	TotalProcessTimeMs       *int64
	UserCount                *int
	EfficiencyRatioLead      *float64
	EfficiencyRatioProcess   *float64
	APICost                  *float64
	DailyRate                *float64
	CostSaving               *float64
	ROI                      *float64
	AnalysisFilePath         *string
	CreatedAt                *time.Time
	UpdatedAt                *time.Time
	OurAICodeLines           *int64
	HumanCodeLines           *int64
	UserManualDays           *float64
	UserManualDaysReason     *string
	UserManualDaysBy         *string
	UserManualDaysAt         *time.Time
}

// RepoMetrics 仓库维度指标
type RepoMetrics struct {
	ID                         int
	RepoID                     string
	AnalysisDate               time.Time
	QueryStartDate             time.Time
	QueryEndDate               time.Time
	GitCommitCount             *int
	GitContributorCount        *int
	GitLinesAdded              *int64
	GitLinesDeleted            *int64
	GitFilesChanged            *int
	RawAIEstimatedDaysFromTask *float64
	RawAIEstimatedDaysFromGit  *float64
	RawAIEstimatedDaysFinal    *float64
	CorrectedAIEstimatedDays   *float64
	CorrectionReason           *string
	CorrectedBy                *string
	CorrectedAt                *time.Time
	ActualStartTime            *time.Time
	ActualEndTime              *time.Time
	TotalLeadTimeMs            *int64
	TotalProcessTimeMs         *int64
	EfficiencyRatioLead        *float64
	EfficiencyRatioProcess     *float64
	APICost                    *float64
	DailyRate                  *float64
	CostSaving                 *float64
	ROI                        *float64
	AnalysisFilePath           *string
	GitAnalysisFilePath        *string
	CreatedAt                  *time.Time
	UpdatedAt                  *time.Time
	OurAICodeLines             *int64
	HumanCodeLines             *int64
	AIOtherCodeLines           *int64
	UnknownCodeLines           *int64
	MappedTaskCount            *int
	UserManualDays             *float64
	UserManualDaysReason       *string
	UserManualDaysBy           *string
	UserManualDaysAt           *time.Time
}

// CorrectionHistory 纠正历史记录
type CorrectionHistory struct {
	ID           int
	Dimension    string
	DimensionID  string
	AnalysisDate time.Time
	FieldName    string
	OldValue     *string
	NewValue     *string
	Reason       *string
	CorrectedBy  *string
	CorrectedAt  *time.Time
}

// TaskCommitMapping task_commit_mapping 表结构
type TaskCommitMapping struct {
	ID           int
	RepoID       string
	TaskID       string
	CommitHash   string
	UserID       *string
	MatchScore   *float64
	MatchReason  *string
	CodeSource   string
	AnalysisDate time.Time
	CreatedAt    *time.Time
}

// CodeAttributionRow code_attribution 表结构
type CodeAttributionRow struct {
	ID              int
	RepoID          string
	CommitHash      string
	TaskID          *string
	OurAICodeLines  int64
	HumanCodeLines  int64
	TotalAddedLines int64
	AnalysisDate    time.Time
	CreatedAt       *time.Time
}

// --- project_metrics 列名与 scan 辅助 ---

var projectMetricsSelectColumns = `id, project_id, analysis_date, query_start_date, query_end_date,
	raw_ai_estimated_days, raw_total_cost, raw_total_code_lines, raw_task_count,
	corrected_ai_estimated_days, correction_reason, corrected_by, corrected_at,
	actual_start_time, actual_end_time, total_lead_time_ms, total_process_time_ms,
	user_count, efficiency_ratio_lead, efficiency_ratio_process,
	api_cost, daily_rate, cost_saving, roi,
	analysis_file_path, created_at, updated_at,
	our_ai_code_lines, human_code_lines,
	user_manual_days, user_manual_days_reason, user_manual_days_by, user_manual_days_at`

func scanProjectMetrics(s rowScanner, m *ProjectMetrics) error {
	return s.Scan(
		&m.ID, &m.ProjectID, &m.AnalysisDate, &m.QueryStartDate, &m.QueryEndDate,
		&m.RawAIEstimatedDays, &m.RawTotalCost, &m.RawTotalCodeLines, &m.RawTaskCount,
		&m.CorrectedAIEstimatedDays, &m.CorrectionReason, &m.CorrectedBy, &m.CorrectedAt,
		&m.ActualStartTime, &m.ActualEndTime, &m.TotalLeadTimeMs, &m.TotalProcessTimeMs,
		&m.UserCount, &m.EfficiencyRatioLead, &m.EfficiencyRatioProcess,
		&m.APICost, &m.DailyRate, &m.CostSaving, &m.ROI,
		&m.AnalysisFilePath, &m.CreatedAt, &m.UpdatedAt,
		&m.OurAICodeLines, &m.HumanCodeLines,
		&m.UserManualDays, &m.UserManualDaysReason, &m.UserManualDaysBy, &m.UserManualDaysAt,
	)
}

// --- project_metrics CRUD ---

// UpsertProjectMetrics 插入或更新项目指标
func UpsertProjectMetrics(db *sql.DB, m *ProjectMetrics) error {
	_, err := db.Exec(`
		INSERT INTO project_metrics (
			project_id, analysis_date, query_start_date, query_end_date,
			raw_ai_estimated_days, raw_total_cost, raw_total_code_lines, raw_task_count,
			corrected_ai_estimated_days, correction_reason, corrected_by, corrected_at,
			actual_start_time, actual_end_time, total_lead_time_ms, total_process_time_ms,
			user_count, efficiency_ratio_lead, efficiency_ratio_process,
			api_cost, daily_rate, cost_saving, roi,
			analysis_file_path,
			our_ai_code_lines, human_code_lines,
			user_manual_days, user_manual_days_reason, user_manual_days_by, user_manual_days_at,
			updated_at
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8,
			$9, $10, $11, $12,
			$13, $14, $15, $16,
			$17, $18, $19,
			$20, $21, $22, $23,
			$24,
			$25, $26,
			$27, $28, $29, $30,
			CURRENT_TIMESTAMP
		)
		ON CONFLICT (project_id, analysis_date, query_start_date, query_end_date)
		DO UPDATE SET
			raw_ai_estimated_days = $5, raw_total_cost = $6,
			raw_total_code_lines = $7, raw_task_count = $8,
			corrected_ai_estimated_days = $9, correction_reason = $10,
			corrected_by = $11, corrected_at = $12,
			actual_start_time = $13, actual_end_time = $14,
			total_lead_time_ms = $15, total_process_time_ms = $16,
			user_count = $17, efficiency_ratio_lead = $18, efficiency_ratio_process = $19,
			api_cost = $20, daily_rate = $21, cost_saving = $22, roi = $23,
			analysis_file_path = $24,
			our_ai_code_lines = $25, human_code_lines = $26,
			user_manual_days = $27, user_manual_days_reason = $28,
			user_manual_days_by = $29, user_manual_days_at = $30,
			updated_at = CURRENT_TIMESTAMP`,
		m.ProjectID, m.AnalysisDate, m.QueryStartDate, m.QueryEndDate,
		m.RawAIEstimatedDays, m.RawTotalCost, m.RawTotalCodeLines, m.RawTaskCount,
		m.CorrectedAIEstimatedDays, m.CorrectionReason, m.CorrectedBy, m.CorrectedAt,
		m.ActualStartTime, m.ActualEndTime, m.TotalLeadTimeMs, m.TotalProcessTimeMs,
		m.UserCount, m.EfficiencyRatioLead, m.EfficiencyRatioProcess,
		m.APICost, m.DailyRate, m.CostSaving, m.ROI,
		m.AnalysisFilePath,
		m.OurAICodeLines, m.HumanCodeLines,
		m.UserManualDays, m.UserManualDaysReason, m.UserManualDaysBy, m.UserManualDaysAt,
	)
	if err != nil {
		return fmt.Errorf("upsert project_metrics 失败: %w", err)
	}
	return nil
}

// GetProjectMetrics 查询单条项目指标，不存在返回 nil, nil
func GetProjectMetrics(db *sql.DB, projectID string, analysisDate string, startDate string, endDate string) (*ProjectMetrics, error) {
	var m ProjectMetrics
	err := scanProjectMetrics(db.QueryRow(fmt.Sprintf(`
		SELECT %s FROM project_metrics
		WHERE project_id = $1 AND analysis_date = $2
			AND query_start_date = $3 AND query_end_date = $4`,
		projectMetricsSelectColumns),
		projectID, analysisDate, startDate, endDate,
	), &m)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询 project_metrics 失败: %w", err)
	}
	return &m, nil
}

// ListProjectMetrics 按日期范围查询项目指标
func ListProjectMetrics(db *sql.DB, startDate string, endDate string) ([]ProjectMetrics, error) {
	rows, err := db.Query(fmt.Sprintf(`
		SELECT %s FROM project_metrics
		WHERE query_start_date >= $1 AND query_end_date <= $2
		ORDER BY analysis_date DESC, project_id`,
		projectMetricsSelectColumns),
		startDate, endDate,
	)
	if err != nil {
		return nil, fmt.Errorf("查询 project_metrics 列表失败: %w", err)
	}
	defer rows.Close()

	var list []ProjectMetrics
	for rows.Next() {
		var m ProjectMetrics
		if err := scanProjectMetrics(rows, &m); err != nil {
			return nil, fmt.Errorf("扫描 project_metrics 行失败: %w", err)
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

// --- repo_metrics 列名与 scan 辅助 ---

var repoMetricsSelectColumns = `id, repo_id, analysis_date, query_start_date, query_end_date,
	git_commit_count, git_contributor_count, git_lines_added, git_lines_deleted, git_files_changed,
	raw_ai_estimated_days_from_task, raw_ai_estimated_days_from_git, raw_ai_estimated_days_final,
	corrected_ai_estimated_days, correction_reason, corrected_by, corrected_at,
	actual_start_time, actual_end_time, total_lead_time_ms, total_process_time_ms,
	efficiency_ratio_lead, efficiency_ratio_process,
	api_cost, daily_rate, cost_saving, roi,
	analysis_file_path, git_analysis_file_path, created_at, updated_at,
	our_ai_code_lines, human_code_lines, ai_other_code_lines, unknown_code_lines, mapped_task_count,
	user_manual_days, user_manual_days_reason, user_manual_days_by, user_manual_days_at`

func scanRepoMetrics(s rowScanner, m *RepoMetrics) error {
	return s.Scan(
		&m.ID, &m.RepoID, &m.AnalysisDate, &m.QueryStartDate, &m.QueryEndDate,
		&m.GitCommitCount, &m.GitContributorCount, &m.GitLinesAdded, &m.GitLinesDeleted, &m.GitFilesChanged,
		&m.RawAIEstimatedDaysFromTask, &m.RawAIEstimatedDaysFromGit, &m.RawAIEstimatedDaysFinal,
		&m.CorrectedAIEstimatedDays, &m.CorrectionReason, &m.CorrectedBy, &m.CorrectedAt,
		&m.ActualStartTime, &m.ActualEndTime, &m.TotalLeadTimeMs, &m.TotalProcessTimeMs,
		&m.EfficiencyRatioLead, &m.EfficiencyRatioProcess,
		&m.APICost, &m.DailyRate, &m.CostSaving, &m.ROI,
		&m.AnalysisFilePath, &m.GitAnalysisFilePath, &m.CreatedAt, &m.UpdatedAt,
		&m.OurAICodeLines, &m.HumanCodeLines, &m.AIOtherCodeLines, &m.UnknownCodeLines, &m.MappedTaskCount,
		&m.UserManualDays, &m.UserManualDaysReason, &m.UserManualDaysBy, &m.UserManualDaysAt,
	)
}

// --- repo_metrics CRUD ---

// UpsertRepoMetrics 插入或更新仓库指标
func UpsertRepoMetrics(db *sql.DB, m *RepoMetrics) error {
	_, err := db.Exec(`
		INSERT INTO repo_metrics (
			repo_id, analysis_date, query_start_date, query_end_date,
			git_commit_count, git_contributor_count, git_lines_added, git_lines_deleted, git_files_changed,
			raw_ai_estimated_days_from_task, raw_ai_estimated_days_from_git, raw_ai_estimated_days_final,
			corrected_ai_estimated_days, correction_reason, corrected_by, corrected_at,
			actual_start_time, actual_end_time, total_lead_time_ms, total_process_time_ms,
			efficiency_ratio_lead, efficiency_ratio_process,
			api_cost, daily_rate, cost_saving, roi,
			analysis_file_path, git_analysis_file_path,
			our_ai_code_lines, human_code_lines, ai_other_code_lines, unknown_code_lines, mapped_task_count,
			user_manual_days, user_manual_days_reason, user_manual_days_by, user_manual_days_at,
			updated_at
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8, $9,
			$10, $11, $12,
			$13, $14, $15, $16,
			$17, $18, $19, $20,
			$21, $22,
			$23, $24, $25, $26,
			$27, $28,
			$29, $30, $31, $32, $33,
			$34, $35, $36, $37,
			CURRENT_TIMESTAMP
		)
		ON CONFLICT (repo_id, analysis_date, query_start_date, query_end_date)
		DO UPDATE SET
			git_commit_count = $5, git_contributor_count = $6,
			git_lines_added = $7, git_lines_deleted = $8, git_files_changed = $9,
			raw_ai_estimated_days_from_task = $10, raw_ai_estimated_days_from_git = $11,
			raw_ai_estimated_days_final = $12,
			corrected_ai_estimated_days = $13, correction_reason = $14,
			corrected_by = $15, corrected_at = $16,
			actual_start_time = $17, actual_end_time = $18,
			total_lead_time_ms = $19, total_process_time_ms = $20,
			efficiency_ratio_lead = $21, efficiency_ratio_process = $22,
			api_cost = $23, daily_rate = $24, cost_saving = $25, roi = $26,
			analysis_file_path = $27, git_analysis_file_path = $28,
			our_ai_code_lines = $29, human_code_lines = $30,
			ai_other_code_lines = $31, unknown_code_lines = $32, mapped_task_count = $33,
			user_manual_days = $34, user_manual_days_reason = $35,
			user_manual_days_by = $36, user_manual_days_at = $37,
			updated_at = CURRENT_TIMESTAMP`,
		m.RepoID, m.AnalysisDate, m.QueryStartDate, m.QueryEndDate,
		m.GitCommitCount, m.GitContributorCount, m.GitLinesAdded, m.GitLinesDeleted, m.GitFilesChanged,
		m.RawAIEstimatedDaysFromTask, m.RawAIEstimatedDaysFromGit, m.RawAIEstimatedDaysFinal,
		m.CorrectedAIEstimatedDays, m.CorrectionReason, m.CorrectedBy, m.CorrectedAt,
		m.ActualStartTime, m.ActualEndTime, m.TotalLeadTimeMs, m.TotalProcessTimeMs,
		m.EfficiencyRatioLead, m.EfficiencyRatioProcess,
		m.APICost, m.DailyRate, m.CostSaving, m.ROI,
		m.AnalysisFilePath, m.GitAnalysisFilePath,
		m.OurAICodeLines, m.HumanCodeLines, m.AIOtherCodeLines, m.UnknownCodeLines, m.MappedTaskCount,
		m.UserManualDays, m.UserManualDaysReason, m.UserManualDaysBy, m.UserManualDaysAt,
	)
	if err != nil {
		return fmt.Errorf("upsert repo_metrics 失败: %w", err)
	}
	return nil
}

// GetRepoMetrics 查询单条仓库指标，不存在返回 nil, nil
func GetRepoMetrics(db *sql.DB, repoID string, analysisDate string, startDate string, endDate string) (*RepoMetrics, error) {
	var m RepoMetrics
	err := scanRepoMetrics(db.QueryRow(fmt.Sprintf(`
		SELECT %s FROM repo_metrics
		WHERE repo_id = $1 AND analysis_date = $2
			AND query_start_date = $3 AND query_end_date = $4`,
		repoMetricsSelectColumns),
		repoID, analysisDate, startDate, endDate,
	), &m)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询 repo_metrics 失败: %w", err)
	}
	return &m, nil
}

// ListRepoMetrics 按日期范围查询仓库指标
func ListRepoMetrics(db *sql.DB, startDate string, endDate string) ([]RepoMetrics, error) {
	rows, err := db.Query(fmt.Sprintf(`
		SELECT %s FROM repo_metrics
		WHERE query_start_date >= $1 AND query_end_date <= $2
		ORDER BY analysis_date DESC, repo_id`,
		repoMetricsSelectColumns),
		startDate, endDate,
	)
	if err != nil {
		return nil, fmt.Errorf("查询 repo_metrics 列表失败: %w", err)
	}
	defer rows.Close()

	var list []RepoMetrics
	for rows.Next() {
		var m RepoMetrics
		if err := scanRepoMetrics(rows, &m); err != nil {
			return nil, fmt.Errorf("扫描 repo_metrics 行失败: %w", err)
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

// GetLatestRepoMetrics 取指定 repo 最新一条记录
func GetLatestRepoMetrics(db *sql.DB, repoID string) (*RepoMetrics, error) {
	var m RepoMetrics
	err := scanRepoMetrics(db.QueryRow(fmt.Sprintf(`
		SELECT %s FROM repo_metrics
		WHERE repo_id = $1
		ORDER BY analysis_date DESC
		LIMIT 1`,
		repoMetricsSelectColumns),
		repoID,
	), &m)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询最新 repo_metrics 失败: %w", err)
	}
	return &m, nil
}

// --- correction_history CRUD ---

// InsertCorrectionHistory 插入纠正历史记录
func InsertCorrectionHistory(db *sql.DB, h *CorrectionHistory) error {
	_, err := db.Exec(`
		INSERT INTO correction_history (
			dimension, dimension_id, analysis_date, field_name,
			old_value, new_value, reason, corrected_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		h.Dimension, h.DimensionID, h.AnalysisDate, h.FieldName,
		h.OldValue, h.NewValue, h.Reason, h.CorrectedBy,
	)
	if err != nil {
		return fmt.Errorf("插入 correction_history 失败: %w", err)
	}
	return nil
}

// ListCorrectionHistory 按维度和维度ID查询纠正历史，按 corrected_at DESC 排序
func ListCorrectionHistory(db *sql.DB, dimension string, dimensionID string) ([]CorrectionHistory, error) {
	rows, err := db.Query(`
		SELECT id, dimension, dimension_id, analysis_date, field_name,
			old_value, new_value, reason, corrected_by, corrected_at
		FROM correction_history
		WHERE dimension = $1 AND dimension_id = $2
		ORDER BY corrected_at DESC`,
		dimension, dimensionID,
	)
	if err != nil {
		return nil, fmt.Errorf("查询 correction_history 列表失败: %w", err)
	}
	defer rows.Close()

	var list []CorrectionHistory
	for rows.Next() {
		var h CorrectionHistory
		if err := rows.Scan(
			&h.ID, &h.Dimension, &h.DimensionID, &h.AnalysisDate, &h.FieldName,
			&h.OldValue, &h.NewValue, &h.Reason, &h.CorrectedBy, &h.CorrectedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描 correction_history 行失败: %w", err)
		}
		list = append(list, h)
	}
	return list, rows.Err()
}

// --- task_commit_mapping CRUD ---

// UpsertTaskCommitMapping 插入或更新任务-提交映射
func UpsertTaskCommitMapping(db *sql.DB, m *TaskCommitMapping) error {
	_, err := db.Exec(`
		INSERT INTO task_commit_mapping (
			repo_id, task_id, commit_hash, user_id,
			match_score, match_reason, code_source, analysis_date
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (repo_id, task_id, commit_hash)
		DO UPDATE SET
			user_id = $4, match_score = $5, match_reason = $6,
			code_source = $7, analysis_date = $8`,
		m.RepoID, m.TaskID, m.CommitHash, m.UserID,
		m.MatchScore, m.MatchReason, m.CodeSource, m.AnalysisDate,
	)
	if err != nil {
		return fmt.Errorf("upsert task_commit_mapping 失败: %w", err)
	}
	return nil
}

// ListTaskCommitMappings 按 repo_id 和 analysis_date 范围查询任务-提交映射
func ListTaskCommitMappings(db *sql.DB, repoID string, startDate, endDate string) ([]TaskCommitMapping, error) {
	rows, err := db.Query(`
		SELECT id, repo_id, task_id, commit_hash, user_id,
			match_score, match_reason, code_source, analysis_date, created_at
		FROM task_commit_mapping
		WHERE repo_id = $1 AND analysis_date >= $2 AND analysis_date <= $3
		ORDER BY analysis_date DESC, task_id`,
		repoID, startDate, endDate,
	)
	if err != nil {
		return nil, fmt.Errorf("查询 task_commit_mapping 列表失败: %w", err)
	}
	defer rows.Close()

	var list []TaskCommitMapping
	for rows.Next() {
		var m TaskCommitMapping
		if err := rows.Scan(
			&m.ID, &m.RepoID, &m.TaskID, &m.CommitHash, &m.UserID,
			&m.MatchScore, &m.MatchReason, &m.CodeSource, &m.AnalysisDate, &m.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描 task_commit_mapping 行失败: %w", err)
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

// --- code_attribution CRUD ---

// UpsertCodeAttribution 插入或更新代码归因
func UpsertCodeAttribution(db *sql.DB, a *CodeAttributionRow) error {
	_, err := db.Exec(`
		INSERT INTO code_attribution (
			repo_id, commit_hash, task_id,
			our_ai_code_lines, human_code_lines, total_added_lines, analysis_date
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (repo_id, commit_hash, task_id)
		DO UPDATE SET
			our_ai_code_lines = $4, human_code_lines = $5,
			total_added_lines = $6, analysis_date = $7`,
		a.RepoID, a.CommitHash, a.TaskID,
		a.OurAICodeLines, a.HumanCodeLines, a.TotalAddedLines, a.AnalysisDate,
	)
	if err != nil {
		return fmt.Errorf("upsert code_attribution 失败: %w", err)
	}
	return nil
}

// ListCodeAttributions 按 repo_id 和 analysis_date 范围查询代码归因
func ListCodeAttributions(db *sql.DB, repoID string, startDate, endDate string) ([]CodeAttributionRow, error) {
	rows, err := db.Query(`
		SELECT id, repo_id, commit_hash, task_id,
			our_ai_code_lines, human_code_lines, total_added_lines, analysis_date, created_at
		FROM code_attribution
		WHERE repo_id = $1 AND analysis_date >= $2 AND analysis_date <= $3
		ORDER BY analysis_date DESC, commit_hash`,
		repoID, startDate, endDate,
	)
	if err != nil {
		return nil, fmt.Errorf("查询 code_attribution 列表失败: %w", err)
	}
	defer rows.Close()

	var list []CodeAttributionRow
	for rows.Next() {
		var a CodeAttributionRow
		if err := rows.Scan(
			&a.ID, &a.RepoID, &a.CommitHash, &a.TaskID,
			&a.OurAICodeLines, &a.HumanCodeLines, &a.TotalAddedLines, &a.AnalysisDate, &a.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描 code_attribution 行失败: %w", err)
		}
		list = append(list, a)
	}
	return list, rows.Err()
}

// UpdateUserManualDays 更新用户手动填写的人天数，并写入 correction_history 审计记录
func UpdateUserManualDays(db *sql.DB, dimension, id string, analysisDate, startDate, endDate string, days float64, reason, by string) error {
	now := time.Now()
	if dimension == "work_dir" {
		_, err := db.Exec(`
			UPDATE project_metrics
			SET user_manual_days = $1, user_manual_days_reason = $2,
				user_manual_days_by = $3, user_manual_days_at = $4,
				updated_at = CURRENT_TIMESTAMP
			WHERE project_id = $5 AND analysis_date = $6
				AND query_start_date = $7 AND query_end_date = $8`,
			days, reason, by, now,
			id, analysisDate, startDate, endDate,
		)
		if err != nil {
			return fmt.Errorf("更新 project_metrics user_manual_days 失败: %w", err)
		}
	} else {
		_, err := db.Exec(`
			UPDATE repo_metrics
			SET user_manual_days = $1, user_manual_days_reason = $2,
				user_manual_days_by = $3, user_manual_days_at = $4,
				updated_at = CURRENT_TIMESTAMP
			WHERE repo_id = $5 AND analysis_date = $6
				AND query_start_date = $7 AND query_end_date = $8`,
			days, reason, by, now,
			id, analysisDate, startDate, endDate,
		)
		if err != nil {
			return fmt.Errorf("更新 repo_metrics user_manual_days 失败: %w", err)
		}
	}

	h := &CorrectionHistory{
		Dimension:   dimension,
		DimensionID: id,
		FieldName:   "user_manual_days",
		NewValue:    ptrString(fmt.Sprintf("%.2f", days)),
		Reason:      ptrString(reason),
		CorrectedBy: ptrString(by),
	}
	h.AnalysisDate, _ = time.Parse("2006-01-02", analysisDate)
	return InsertCorrectionHistory(db, h)
}

// ============================================================
// costrict_stat 数据库 - StatTask / StatTaskConversation
// ============================================================

// StatTask costrict_stat 数据库 tasks 表结构
type StatTask struct {
	TaskID                         string     `json:"task_id"`
	UserID                         *string    `json:"user_id"`
	UserName                       *string    `json:"user_name"`
	ClientID                       *string    `json:"client_id"`
	ClientIDE                      *string    `json:"client_ide"`
	ClientVersion                  *string    `json:"client_version"`
	ClientOS                       *string    `json:"client_os"`
	ClientOSVersion                *string    `json:"client_os_version"`
	Caller                         *string    `json:"caller"`
	RepoAddr                       *string    `json:"repo_addr"`
	RepoBranch                     *string    `json:"repo_branch"`
	WorkDir                        *string    `json:"work_dir"`
	WorkDirID                      *string    `json:"work_dir_id"`
	Diff                           *string    `json:"diff"`
	DiffLines                      *int       `json:"diff_lines"`
	StartTime                      *time.Time `json:"start_time"`
	EndTime                        *time.Time `json:"end_time"`
	UpstreamTokens                 *int64     `json:"upstream_tokens"`
	DownstreamTokens               *int64     `json:"downstream_tokens"`
	Cost                           *float64   `json:"cost"`
	TaskRealMinutes                *float64   `json:"task_real_minutes"`
	TaskRealMinutesReason          *string    `json:"task_real_minutes_reason"`
	TaskRealMinutesManual          *float64   `json:"task_real_minutes_manual"`
	TaskRealMinutesReasonManual    *string    `json:"task_real_minutes_reason_manual"`
	TaskAncientMinutes             *float64   `json:"task_ancient_minutes"`
	TaskAncientMinutesReason       *string    `json:"task_ancient_minutes_reason"`
	TaskAncientMinutesManual       *float64   `json:"task_ancient_minutes_manual"`
	TaskAncientMinutesReasonManual *string    `json:"task_ancient_minutes_reason_manual"`
	EfficiencyRatio                *float64   `json:"efficiency_ratio"`
	Title                          *string    `json:"title"`
	CreatedAt                      *time.Time `json:"created_at"`
	UpdatedAt                      *time.Time `json:"updated_at"`
}

// StatTaskConversation costrict_stat 数据库 task_conversations 表结构
type StatTaskConversation struct {
	ID               int        `json:"id"`
	TaskID           string     `json:"task_id"`
	RequestID        string     `json:"request_id"`
	Sender           *string    `json:"sender"`
	PromptMode       *string    `json:"prompt_mode"`
	Mode             *string    `json:"mode"`
	Model            *string    `json:"model"`
	StartTime        *time.Time `json:"start_time"`
	EndTime          *time.Time `json:"end_time"`
	ProcessTime      *int64     `json:"process_time"`
	ProcessTTFT      *int64     `json:"process_ttft"`
	UpstreamTokens   *int64     `json:"upstream_tokens"`
	DownstreamTokens *int64     `json:"downstream_tokens"`
	Cost             *float64   `json:"cost"`
	RequestContent   *string    `json:"request_content"`
	ResponseContent  *string    `json:"response_content"`
	UserInput        *string    `json:"user_input"`
	Diff             *string    `json:"diff"`
	DiffLines        *int64     `json:"diff_lines"`
	ErrorCode        *string    `json:"error_code"`
	ErrorReason      *string    `json:"error_reason"`
	CreatedAt        *time.Time `json:"created_at"`
}

// --- stat tasks 列名与 scan 辅助 ---

var statTaskSelectColumns = `task_id, user_id, user_name, client_id, client_ide, client_version, client_os, client_os_version,
	caller, repo_addr, repo_branch, work_dir, work_dir_id,
	diff, diff_lines,
	start_time, end_time, upstream_tokens, downstream_tokens, cost,
	task_real_minutes, task_real_minutes_reason,
	task_real_minutes_manual, task_real_minutes_reason_manual,
	task_ancient_minutes, task_ancient_minutes_reason,
	task_ancient_minutes_manual, task_ancient_minutes_reason_manual,
	efficiency_ratio, title, created_at, updated_at`

func scanStatTask(s rowScanner, m *StatTask) error {
	return s.Scan(
		&m.TaskID, &m.UserID, &m.UserName, &m.ClientID, &m.ClientIDE, &m.ClientVersion, &m.ClientOS, &m.ClientOSVersion,
		&m.Caller, &m.RepoAddr, &m.RepoBranch, &m.WorkDir, &m.WorkDirID,
		&m.Diff, &m.DiffLines,
		&m.StartTime, &m.EndTime, &m.UpstreamTokens, &m.DownstreamTokens, &m.Cost,
		&m.TaskRealMinutes, &m.TaskRealMinutesReason,
		&m.TaskRealMinutesManual, &m.TaskRealMinutesReasonManual,
		&m.TaskAncientMinutes, &m.TaskAncientMinutesReason,
		&m.TaskAncientMinutesManual, &m.TaskAncientMinutesReasonManual,
		&m.EfficiencyRatio, &m.Title, &m.CreatedAt, &m.UpdatedAt,
	)
}

// --- stat task_conversations 列名与 scan 辅助 ---

var statTaskConversationSelectColumns = `id, task_id, request_id, sender, prompt_mode, mode, model,
	start_time, end_time, process_time, process_ttft,
	upstream_tokens, downstream_tokens, cost,
	request_content, response_content, user_input, diff, diff_lines,
	error_code, error_reason, created_at`

func scanStatTaskConversation(s rowScanner, m *StatTaskConversation) error {
	return s.Scan(
		&m.ID, &m.TaskID, &m.RequestID, &m.Sender, &m.PromptMode, &m.Mode, &m.Model,
		&m.StartTime, &m.EndTime, &m.ProcessTime, &m.ProcessTTFT,
		&m.UpstreamTokens, &m.DownstreamTokens, &m.Cost,
		&m.RequestContent, &m.ResponseContent, &m.UserInput, &m.Diff, &m.DiffLines,
		&m.ErrorCode, &m.ErrorReason, &m.CreatedAt,
	)
}

// --- stat tasks CRUD ---

// UpsertStatTask 插入或更新 stat tasks 记录
func UpsertStatTask(db *sql.DB, t *StatTask) error {
	_, err := db.Exec(`
		INSERT INTO tasks (
			task_id, user_id, user_name, client_id, client_ide, client_version, client_os, client_os_version,
			caller, repo_addr, repo_branch, work_dir, work_dir_id,
			diff, diff_lines,
			start_time, end_time, upstream_tokens, downstream_tokens, cost,
			task_real_minutes, task_real_minutes_reason,
			task_real_minutes_manual, task_real_minutes_reason_manual,
			task_ancient_minutes, task_ancient_minutes_reason,
			task_ancient_minutes_manual, task_ancient_minutes_reason_manual,
			efficiency_ratio, title
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			$9, $10, $11, $12, $13,
			$14, $15,
			$16, $17, $18, $19, $20,
			$21, $22,
			$23, $24,
			$25, $26,
			$27, $28,
			$29, $30
		)
		ON CONFLICT(task_id) DO UPDATE SET
			user_id = $2, user_name = $3, client_id = $4, client_ide = $5, client_version = $6,
			client_os = $7, client_os_version = $8, caller = $9,
			repo_addr = $10, repo_branch = $11, work_dir = $12, work_dir_id = $13,
			diff = $14, diff_lines = $15,
			start_time = $16, end_time = $17, upstream_tokens = $18, downstream_tokens = $19,
			cost = $20,
			task_real_minutes = $21, task_real_minutes_reason = $22,
			task_real_minutes_manual = $23, task_real_minutes_reason_manual = $24,
			task_ancient_minutes = $25, task_ancient_minutes_reason = $26,
			task_ancient_minutes_manual = $27, task_ancient_minutes_reason_manual = $28,
			efficiency_ratio = $29, title = COALESCE($30, tasks.title),
			updated_at = CURRENT_TIMESTAMP`,
		t.TaskID, t.UserID, t.UserName, t.ClientID, t.ClientIDE, t.ClientVersion, t.ClientOS, t.ClientOSVersion,
		t.Caller, t.RepoAddr, t.RepoBranch, t.WorkDir, t.WorkDirID,
		t.Diff, t.DiffLines,
		t.StartTime, t.EndTime, t.UpstreamTokens, t.DownstreamTokens, t.Cost,
		t.TaskRealMinutes, t.TaskRealMinutesReason,
		t.TaskRealMinutesManual, t.TaskRealMinutesReasonManual,
		t.TaskAncientMinutes, t.TaskAncientMinutesReason,
		t.TaskAncientMinutesManual, t.TaskAncientMinutesReasonManual,
		t.EfficiencyRatio, t.Title,
	)
	if err != nil {
		return fmt.Errorf("upsert stat tasks 失败: %w", err)
	}
	return nil
}

// GetStatTask 查询单条 stat tasks 记录，不存在返回 nil, nil
func GetStatTask(db *sql.DB, taskID string) (*StatTask, error) {
	var m StatTask
	err := scanStatTask(db.QueryRow(fmt.Sprintf(`
		SELECT %s FROM tasks WHERE task_id = $1`,
		statTaskSelectColumns),
		taskID,
	), &m)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询 stat tasks 失败: %w", err)
	}
	return &m, nil
}

// BatchGetStatTasks 批量查询 stat tasks 记录，返回 map[taskID]*StatTask
func BatchGetStatTasks(db *sql.DB, taskIDs []string) (map[string]*StatTask, error) {
	result := make(map[string]*StatTask)
	if len(taskIDs) == 0 {
		return result, nil
	}
	placeholders := make([]string, len(taskIDs))
	args := make([]interface{}, len(taskIDs))
	for i, id := range taskIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	query := fmt.Sprintf(`SELECT %s FROM tasks WHERE task_id IN (%s)`,
		statTaskSelectColumns, strings.Join(placeholders, ", "))
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("批量查询 stat tasks 失败: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var m StatTask
		if err := scanStatTask(rows, &m); err != nil {
			return nil, fmt.Errorf("批量查询 stat tasks 失败: %w", err)
		}
		result[m.TaskID] = &m
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("批量查询 stat tasks 失败: %w", err)
	}
	return result, nil
}

// ListStatTasks 按条件查询 stat tasks 列表
func ListStatTasks(db *sql.DB, userID, workDirID, startTime, endTime string, page, pageSize int) ([]StatTask, error) {
	var conditions []string
	var args []interface{}
	argIdx := 1

	if userID != "" {
		conditions = append(conditions, fmt.Sprintf("user_id = $%d", argIdx))
		args = append(args, userID)
		argIdx++
	}
	if workDirID != "" {
		conditions = append(conditions, fmt.Sprintf("work_dir_id = $%d", argIdx))
		args = append(args, workDirID)
		argIdx++
	}
	if startTime != "" {
		conditions = append(conditions, fmt.Sprintf("start_time >= $%d", argIdx))
		args = append(args, startTime)
		argIdx++
	}
	if endTime != "" {
		conditions = append(conditions, fmt.Sprintf("start_time <= $%d", argIdx))
		args = append(args, endTime)
		argIdx++
	}

	query := fmt.Sprintf("SELECT %s FROM tasks", statTaskSelectColumns)
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY start_time DESC"
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, pageSize, (page-1)*pageSize)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询 stat tasks 列表失败: %w", err)
	}
	defer rows.Close()

	var list []StatTask
	for rows.Next() {
		var m StatTask
		if err := scanStatTask(rows, &m); err != nil {
			return nil, fmt.Errorf("扫描 stat tasks 行失败: %w", err)
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

// CountStatTasks 按条件统计 stat tasks 总数
func CountStatTasks(db *sql.DB, userID, workDirID, startTime, endTime string) (int, error) {
	var conditions []string
	var args []interface{}
	argIdx := 1

	if userID != "" {
		conditions = append(conditions, fmt.Sprintf("user_id = $%d", argIdx))
		args = append(args, userID)
		argIdx++
	}
	if workDirID != "" {
		conditions = append(conditions, fmt.Sprintf("work_dir_id = $%d", argIdx))
		args = append(args, workDirID)
		argIdx++
	}
	if startTime != "" {
		conditions = append(conditions, fmt.Sprintf("start_time >= $%d", argIdx))
		args = append(args, startTime)
		argIdx++
	}
	if endTime != "" {
		conditions = append(conditions, fmt.Sprintf("start_time <= $%d", argIdx))
		args = append(args, endTime)
		argIdx++
	}

	query := "SELECT count(*) FROM tasks"
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	var count int
	err := db.QueryRow(query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("统计 stat tasks 总数失败: %w", err)
	}
	return count, nil
}

// UpdateStatTaskManual 更新 stat tasks 的人工修正字段
func UpdateStatTaskManual(db *sql.DB, taskID string, realManual *float64, realReasonManual *string, ancientManual *float64, ancientReasonManual *string) error {
	result, err := db.Exec(`
		UPDATE tasks SET
			task_real_minutes_manual = $2,
			task_real_minutes_reason_manual = $3,
			task_ancient_minutes_manual = $4,
			task_ancient_minutes_reason_manual = $5,
			updated_at = CURRENT_TIMESTAMP
		WHERE task_id = $1`,
		taskID, realManual, realReasonManual, ancientManual, ancientReasonManual,
	)
	if err != nil {
		return fmt.Errorf("更新 stat tasks manual 字段失败: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("更新 stat tasks manual 字段失败: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("stat tasks task_id=%s 不存在", taskID)
	}
	return nil
}

// --- stat task_conversations CRUD ---

// BatchInsertStatTaskConversations 批量插入 stat task_conversations，使用事务
func BatchInsertStatTaskConversations(db *sql.DB, convs []StatTaskConversation) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("批量插入 stat task_conversations 失败: %w", err)
	}

	for i := range convs {
		c := &convs[i]
		_, err := tx.Exec(`
			INSERT INTO task_conversations (
				task_id, request_id, sender, prompt_mode, mode, model,
				start_time, end_time, process_time, process_ttft,
				upstream_tokens, downstream_tokens, cost,
				request_content, response_content, user_input, diff, diff_lines,
				error_code, error_reason
			) VALUES (
				$1, $2, $3, $4, $5, $6,
				$7, $8, $9, $10,
				$11, $12, $13,
				$14, $15, $16, $17, $18,
				$19, $20
			)
			ON CONFLICT(task_id, request_id) DO NOTHING`,
			c.TaskID, c.RequestID, c.Sender, c.PromptMode, c.Mode, c.Model,
			c.StartTime, c.EndTime, c.ProcessTime, c.ProcessTTFT,
			c.UpstreamTokens, c.DownstreamTokens, c.Cost,
			c.RequestContent, c.ResponseContent, c.UserInput, c.Diff, c.DiffLines,
			c.ErrorCode, c.ErrorReason,
		)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("批量插入 stat task_conversations 失败: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交批量插入 stat task_conversations 事务失败: %w", err)
	}
	return nil
}

// ListStatTaskConversations 按 task_id 查询 stat task_conversations 列表
func ListStatTaskConversations(db *sql.DB, taskID string) ([]StatTaskConversation, error) {
	rows, err := db.Query(fmt.Sprintf(`
		SELECT %s FROM task_conversations
		WHERE task_id = $1 ORDER BY start_time ASC`,
		statTaskConversationSelectColumns),
		taskID,
	)
	if err != nil {
		return nil, fmt.Errorf("查询 stat task_conversations 列表失败: %w", err)
	}
	defer rows.Close()

	var list []StatTaskConversation
	for rows.Next() {
		var m StatTaskConversation
		if err := scanStatTaskConversation(rows, &m); err != nil {
			return nil, fmt.Errorf("扫描 stat task_conversations 行失败: %w", err)
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

// ============================================================
// costrict_stat 数据库 - StatCommit
// ============================================================

// jsonRawToString pq 驱动会把 []byte 编码为 bytea 格式，导致 jsonb 列无法识别
func jsonRawToString(raw json.RawMessage) interface{} {
	if raw == nil {
		return nil
	}
	return string(raw)
}

// StatCommit costrict_stat 数据库 commits 表结构
type StatCommit struct {
	CommitID                         string          `json:"commit_id"`
	CommitTime                       *time.Time      `json:"commit_time"`
	RepoAddr                         *string         `json:"repo_addr"`
	RepoBranch                       *string         `json:"repo_branch"`
	GitUserName                      *string         `json:"git_user_name"`
	GitUserEmail                     *string         `json:"git_user_email"`
	UserID                           *string         `json:"user_id"`
	UserName                         *string         `json:"user_name"`
	ClientID                         *string         `json:"client_id"`
	WorkPath                         *string         `json:"work_path"`
	DiffLines                        *int            `json:"diff_lines"`
	CommitAncientMinutes             *float64        `json:"commit_ancient_minutes"`
	CommitAncientMinutesReason       *string         `json:"commit_ancient_minutes_reason"`
	CommitAncientMinutesManual       *float64        `json:"commit_ancient_minutes_manual"`
	CommitAncientMinutesReasonManual *string         `json:"commit_ancient_minutes_reason_manual"`
	TaskIDs                          json.RawMessage `json:"task_ids"`
	TaskIDsSilica                    json.RawMessage `json:"task_ids_silica"`
	CommitRealAIMinutes              *float64        `json:"commit_real_ai_minutes"`
	CommitRealAncientMinutes         *float64        `json:"commit_real_ancient_minutes"`
	CommitRealMinutes                *float64        `json:"commit_real_minutes"`
	CommitRealMinutesReason          *string         `json:"commit_real_minutes_reason"`
	CommitRealMinutesManual          *float64        `json:"commit_real_minutes_manual"`
	CommitRealMinutesReasonManual    *string         `json:"commit_real_minutes_reason_manual"`
	Comment                          *string         `json:"comment"`
	CreatedAt                        *time.Time      `json:"created_at"`
	UpdatedAt                        *time.Time      `json:"updated_at"`
}

// --- stat commits 列名与 scan 辅助 ---

var statCommitSelectColumns = `commit_id, commit_time, repo_addr, repo_branch,
	git_user_name, git_user_email, user_id, user_name, client_id, work_path,
	diff_lines, commit_ancient_minutes, commit_ancient_minutes_reason,
	commit_ancient_minutes_manual, commit_ancient_minutes_reason_manual,
	task_ids, task_ids_silica,
	commit_real_ai_minutes, commit_real_ancient_minutes,
	commit_real_minutes, commit_real_minutes_reason,
	commit_real_minutes_manual, commit_real_minutes_reason_manual,
	comment,
	created_at, updated_at`

func scanStatCommit(s rowScanner, m *StatCommit) error {
	var taskIDs, taskIDsSilica *[]byte
	err := s.Scan(
		&m.CommitID, &m.CommitTime, &m.RepoAddr, &m.RepoBranch,
		&m.GitUserName, &m.GitUserEmail, &m.UserID, &m.UserName, &m.ClientID, &m.WorkPath,
		&m.DiffLines, &m.CommitAncientMinutes, &m.CommitAncientMinutesReason,
		&m.CommitAncientMinutesManual, &m.CommitAncientMinutesReasonManual,
		&taskIDs, &taskIDsSilica,
		&m.CommitRealAIMinutes, &m.CommitRealAncientMinutes,
		&m.CommitRealMinutes, &m.CommitRealMinutesReason,
		&m.CommitRealMinutesManual, &m.CommitRealMinutesReasonManual,
		&m.Comment,
		&m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return err
	}
	if taskIDs != nil {
		m.TaskIDs = json.RawMessage(*taskIDs)
	}
	if taskIDsSilica != nil {
		m.TaskIDsSilica = json.RawMessage(*taskIDsSilica)
	}
	return nil
}

// --- stat commits CRUD ---

// UpsertStatCommit 插入或更新 stat commits 记录
func UpsertStatCommit(db *sql.DB, c *StatCommit) error {
	_, err := db.Exec(`
		INSERT INTO commits (
			commit_id, commit_time, repo_addr, repo_branch,
			git_user_name, git_user_email, user_id, user_name, client_id, work_path,
			diff_lines, commit_ancient_minutes, commit_ancient_minutes_reason,
			commit_ancient_minutes_manual, commit_ancient_minutes_reason_manual,
			task_ids, task_ids_silica,
			commit_real_ai_minutes, commit_real_ancient_minutes,
			commit_real_minutes, commit_real_minutes_reason,
			commit_real_minutes_manual, commit_real_minutes_reason_manual,
			comment
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8, $9, $10,
			$11, $12, $13,
			$14, $15,
			$16, $17,
			$18, $19,
			$20, $21,
			$22, $23,
			$24
		)
		ON CONFLICT(commit_id) DO UPDATE SET
			commit_time = $2, repo_addr = $3, repo_branch = $4,
			git_user_name = $5, git_user_email = $6, user_id = $7, user_name = $8,
			client_id = $9, work_path = $10,
			diff_lines = $11, commit_ancient_minutes = $12, commit_ancient_minutes_reason = $13,
			commit_ancient_minutes_manual = $14, commit_ancient_minutes_reason_manual = $15,
			task_ids = $16, task_ids_silica = $17,
			commit_real_ai_minutes = $18, commit_real_ancient_minutes = $19,
			commit_real_minutes = $20, commit_real_minutes_reason = $21,
			commit_real_minutes_manual = $22, commit_real_minutes_reason_manual = $23,
			comment = $24,
			updated_at = CURRENT_TIMESTAMP`,
		c.CommitID, c.CommitTime, c.RepoAddr, c.RepoBranch,
		c.GitUserName, c.GitUserEmail, c.UserID, c.UserName, c.ClientID, c.WorkPath,
		c.DiffLines, c.CommitAncientMinutes, c.CommitAncientMinutesReason,
		c.CommitAncientMinutesManual, c.CommitAncientMinutesReasonManual,
		jsonRawToString(c.TaskIDs), jsonRawToString(c.TaskIDsSilica),
		c.CommitRealAIMinutes, c.CommitRealAncientMinutes,
		c.CommitRealMinutes, c.CommitRealMinutesReason,
		c.CommitRealMinutesManual, c.CommitRealMinutesReasonManual,
		c.Comment,
	)
	if err != nil {
		return fmt.Errorf("upsert stat commits 失败: %w", err)
	}
	return nil
}

// GetStatCommitByID 查询单条 stat commits 记录，不存在返回 nil, nil
func GetStatCommitByID(db *sql.DB, commitID string) (*StatCommit, error) {
	var m StatCommit
	err := scanStatCommit(db.QueryRow(fmt.Sprintf(`
		SELECT %s FROM commits WHERE commit_id = $1`,
		statCommitSelectColumns),
		commitID,
	), &m)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询 stat commits 失败: %w", err)
	}
	return &m, nil
}

// ListStatCommits 按条件查询 stat commits 列表
func ListStatCommits(db *sql.DB, repoAddr, repoBranch, userID, startTime, endTime string, page, pageSize int, orgUserIDs []string) ([]StatCommit, error) {
	var conditions []string
	var args []interface{}
	argIdx := 1

	if repoAddr != "" {
		conditions = append(conditions, fmt.Sprintf("repo_addr = $%d", argIdx))
		args = append(args, repoAddr)
		argIdx++
	}
	if repoBranch != "" {
		conditions = append(conditions, fmt.Sprintf("repo_branch = $%d", argIdx))
		args = append(args, repoBranch)
		argIdx++
	}
	if userID != "" {
		conditions = append(conditions, fmt.Sprintf("user_id = $%d", argIdx))
		args = append(args, userID)
		argIdx++
	}
	if len(orgUserIDs) > 0 {
		placeholders := make([]string, len(orgUserIDs))
		for i, uid := range orgUserIDs {
			placeholders[i] = fmt.Sprintf("$%d", argIdx)
			args = append(args, uid)
			argIdx++
		}
		conditions = append(conditions, "user_id IN ("+strings.Join(placeholders, ",")+")")
	}
	if startTime != "" {
		conditions = append(conditions, fmt.Sprintf("commit_time >= $%d", argIdx))
		args = append(args, startTime)
		argIdx++
	}
	if endTime != "" {
		conditions = append(conditions, fmt.Sprintf("commit_time <= $%d", argIdx))
		args = append(args, endTime)
		argIdx++
	}

	query := fmt.Sprintf("SELECT %s FROM commits", statCommitSelectColumns)
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY commit_time DESC"
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, pageSize, (page-1)*pageSize)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询 stat commits 列表失败: %w", err)
	}
	defer rows.Close()

	var list []StatCommit
	for rows.Next() {
		var m StatCommit
		if err := scanStatCommit(rows, &m); err != nil {
			return nil, fmt.Errorf("扫描 stat commits 行失败: %w", err)
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

// CountStatCommits 按条件统计 stat commits 总数
func CountStatCommits(db *sql.DB, repoAddr, repoBranch, userID, startTime, endTime string, orgUserIDs []string) (int, error) {
	var conditions []string
	var args []interface{}
	argIdx := 1

	if repoAddr != "" {
		conditions = append(conditions, fmt.Sprintf("repo_addr = $%d", argIdx))
		args = append(args, repoAddr)
		argIdx++
	}
	if repoBranch != "" {
		conditions = append(conditions, fmt.Sprintf("repo_branch = $%d", argIdx))
		args = append(args, repoBranch)
		argIdx++
	}
	if userID != "" {
		conditions = append(conditions, fmt.Sprintf("user_id = $%d", argIdx))
		args = append(args, userID)
		argIdx++
	}
	if len(orgUserIDs) > 0 {
		placeholders := make([]string, len(orgUserIDs))
		for i, uid := range orgUserIDs {
			placeholders[i] = fmt.Sprintf("$%d", argIdx)
			args = append(args, uid)
			argIdx++
		}
		conditions = append(conditions, "user_id IN ("+strings.Join(placeholders, ",")+")")
	}
	if startTime != "" {
		conditions = append(conditions, fmt.Sprintf("commit_time >= $%d", argIdx))
		args = append(args, startTime)
		argIdx++
	}
	if endTime != "" {
		conditions = append(conditions, fmt.Sprintf("commit_time <= $%d", argIdx))
		args = append(args, endTime)
		argIdx++
	}

	query := "SELECT count(*) FROM commits"
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	var count int
	err := db.QueryRow(query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("统计 stat commits 总数失败: %w", err)
	}
	return count, nil
}

// BatchUpsertStatCommits 在事务中批量插入或更新 stat commits
func BatchUpsertStatCommits(db *sql.DB, commits []StatCommit) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("批量upsert stat commits 失败: %w", err)
	}

	for i := range commits {
		c := &commits[i]
		_, err := tx.Exec(`
			INSERT INTO commits (
				commit_id, commit_time, repo_addr, repo_branch,
				git_user_name, git_user_email, user_id, user_name, client_id, work_path,
				diff_lines, commit_ancient_minutes, commit_ancient_minutes_reason,
				commit_ancient_minutes_manual, commit_ancient_minutes_reason_manual,
				task_ids, task_ids_silica,
				commit_real_ai_minutes, commit_real_ancient_minutes,
				commit_real_minutes, commit_real_minutes_reason,
				commit_real_minutes_manual, commit_real_minutes_reason_manual,
				comment
			) VALUES (
				$1, $2, $3, $4,
				$5, $6, $7, $8, $9, $10,
				$11, $12, $13,
				$14, $15,
				$16, $17,
				$18, $19,
				$20, $21,
				$22, $23,
				$24
			)
			ON CONFLICT(commit_id) DO UPDATE SET
				commit_time = $2, repo_addr = $3, repo_branch = $4,
				git_user_name = $5, git_user_email = $6, user_id = $7, user_name = $8,
				client_id = $9, work_path = $10,
				diff_lines = $11, commit_ancient_minutes = $12, commit_ancient_minutes_reason = $13,
				commit_ancient_minutes_manual = $14, commit_ancient_minutes_reason_manual = $15,
				task_ids = $16, task_ids_silica = $17,
				commit_real_ai_minutes = $18, commit_real_ancient_minutes = $19,
				commit_real_minutes = $20, commit_real_minutes_reason = $21,
				commit_real_minutes_manual = $22, commit_real_minutes_reason_manual = $23,
				comment = $24,
				updated_at = CURRENT_TIMESTAMP`,
			c.CommitID, c.CommitTime, c.RepoAddr, c.RepoBranch,
			c.GitUserName, c.GitUserEmail, c.UserID, c.UserName, c.ClientID, c.WorkPath,
			c.DiffLines, c.CommitAncientMinutes, c.CommitAncientMinutesReason,
			c.CommitAncientMinutesManual, c.CommitAncientMinutesReasonManual,
			jsonRawToString(c.TaskIDs), jsonRawToString(c.TaskIDsSilica),
			c.CommitRealAIMinutes, c.CommitRealAncientMinutes,
			c.CommitRealMinutes, c.CommitRealMinutesReason,
			c.CommitRealMinutesManual, c.CommitRealMinutesReasonManual,
			c.Comment,
		)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("批量upsert stat commits 失败: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交批量upsert stat commits 事务失败: %w", err)
	}
	return nil
}

// UpdateStatCommitManual 更新 stat commits 的人工修正字段
func UpdateStatCommitManual(db *sql.DB, commitID string, ancientManual *float64, ancientReasonManual *string, realManual *float64, realReasonManual *string) error {
	result, err := db.Exec(`
		UPDATE commits SET
			commit_ancient_minutes_manual = $2,
			commit_ancient_minutes_reason_manual = $3,
			commit_real_minutes_manual = $4,
			commit_real_minutes_reason_manual = $5,
			updated_at = CURRENT_TIMESTAMP
		WHERE commit_id = $1`,
		commitID, ancientManual, ancientReasonManual, realManual, realReasonManual,
	)
	if err != nil {
		return fmt.Errorf("更新 stat commits manual 字段失败: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("更新 stat commits manual 字段失败: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("stat commits commit_id=%s 不存在", commitID)
	}
	return nil
}

// UpdateStatCommitTaskAssoc 更新 stat commits 的 task 关联信息
func UpdateStatCommitTaskAssoc(db *sql.DB, commitID string, taskIDs, taskIDsSilica json.RawMessage, realMinutes *float64, realAIMinutes *float64, realAncientMinutes *float64, realReason *string) error {
	result, err := db.Exec(`
		UPDATE commits SET
			task_ids = $2,
			task_ids_silica = $3,
			commit_real_minutes = $4,
			commit_real_ai_minutes = $5,
			commit_real_ancient_minutes = $6,
			commit_real_minutes_reason = $7,
			updated_at = CURRENT_TIMESTAMP
		WHERE commit_id = $1`,
		commitID, jsonRawToString(taskIDs), jsonRawToString(taskIDsSilica), realMinutes, realAIMinutes, realAncientMinutes, realReason,
	)
	if err != nil {
		return fmt.Errorf("更新 stat commits task 关联信息失败: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("更新 stat commits task 关联信息失败: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("stat commits commit_id=%s 不存在", commitID)
	}
	return nil
}

// ListRepoAggregates 按 (repo_addr, repo_branch) 聚合 stat commits 数据
func ListRepoAggregates(db *sql.DB, startTime, endTime string) ([]map[string]interface{}, error) {
	var conditions []string
	var args []interface{}
	argIdx := 1

	conditions = append(conditions, "repo_addr IS NOT NULL AND repo_addr != ''")

	if startTime != "" {
		conditions = append(conditions, fmt.Sprintf("commit_time >= $%d", argIdx))
		args = append(args, startTime)
		argIdx++
	}
	if endTime != "" {
		conditions = append(conditions, fmt.Sprintf("commit_time <= $%d", argIdx))
		args = append(args, endTime)
		argIdx++
	}

	query := `SELECT repo_addr, repo_branch,
		COUNT(*) AS commit_count,
		MIN(commit_time) AS start_time,
		MAX(commit_time) AS end_time,
		SUM(commit_ancient_minutes) AS sum_ancient_minutes,
		SUM(commit_real_minutes) AS sum_real_minutes,
		SUM(CASE WHEN task_ids IS NOT NULL AND task_ids::text NOT IN ('null', '[]') THEN jsonb_array_length(task_ids) ELSE 0 END) AS task_count
		FROM commits`
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " GROUP BY repo_addr, repo_branch ORDER BY repo_addr, repo_branch"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询 stat commits 聚合失败: %w", err)
	}
	defer rows.Close()

	var list []map[string]interface{}
	for rows.Next() {
		var repoAddr, repoBranch *string
		var commitCount int
		var startT, endT *time.Time
		var sumAncient, sumReal *float64
		var taskCount int
		if err := rows.Scan(&repoAddr, &repoBranch, &commitCount, &startT, &endT, &sumAncient, &sumReal, &taskCount); err != nil {
			return nil, fmt.Errorf("扫描 stat commits 聚合行失败: %w", err)
		}
		item := map[string]interface{}{
			"repo_addr":           repoAddr,
			"repo_branch":         repoBranch,
			"commit_count":        commitCount,
			"start_time":          startT,
			"end_time":            endT,
			"sum_ancient_minutes": sumAncient,
			"sum_real_minutes":    sumReal,
			"task_count":          taskCount,
		}
		if sumAncient != nil && sumReal != nil && *sumReal > 0 {
			ratio := (*sumAncient / *sumReal) * 100
			ratio = math.Round(ratio*10) / 10
			item["efficiency_ratio"] = ratio
		} else {
			item["efficiency_ratio"] = nil
		}
		list = append(list, item)
	}
	return list, rows.Err()
}

// ListBranchesByRepoAddr 查询指定 repo_addr 的所有分支
func ListBranchesByRepoAddr(db *sql.DB, repoAddr string) ([]string, error) {
	rows, err := db.Query(`
		SELECT DISTINCT repo_branch FROM commits
		WHERE repo_addr = $1 AND repo_branch IS NOT NULL AND repo_branch != ''
		ORDER BY repo_branch`, repoAddr)
	if err != nil {
		return nil, fmt.Errorf("查询 stat commits 分支列表失败: %w", err)
	}
	defer rows.Close()

	var list []string
	for rows.Next() {
		var branch string
		if err := rows.Scan(&branch); err != nil {
			return nil, fmt.Errorf("扫描 stat commits 分支行失败: %w", err)
		}
		list = append(list, branch)
	}
	return list, rows.Err()
}

// ============================================================
// costrict_stat 数据库 - Project (虚拟项目)
// ============================================================

// Project costrict_stat 数据库 projects 表结构
type Project struct {
	ProjectID                             string          `json:"project_id"`
	Name                                  string          `json:"name"`
	Description                           *string         `json:"description"`
	Repos                                 json.RawMessage `json:"repos"`
	TaskIDs                               json.RawMessage `json:"task_ids"`
	TaskIDsSilica                         json.RawMessage `json:"task_ids_silica"`
	StartTime                             *time.Time      `json:"start_time"`
	EndTime                               *time.Time      `json:"end_time"`
	StartTimeManual                       *time.Time      `json:"start_time_manual"`
	EndTimeManual                         *time.Time      `json:"end_time_manual"`
	UpstreamTokens                        int64           `json:"upstream_tokens"`
	DownstreamTokens                      int64           `json:"downstream_tokens"`
	Cost                                  float64         `json:"cost"`
	ProjectAncientMinutes                 *float64        `json:"project_ancient_minutes"`
	ProjectAncientMinutesReason           *string         `json:"project_ancient_minutes_reason"`
	ProjectAncientMinutesManual           *float64        `json:"project_ancient_minutes_manual"`
	ProjectAncientMinutesReasonManual     *string         `json:"project_ancient_minutes_reason_manual"`
	ProjectRealProcessMinutes             *float64        `json:"project_real_process_minutes"`
	ProjectRealProcessMinutesReason       *string         `json:"project_real_process_minutes_reason"`
	ProjectRealProcessMinutesManual       *float64        `json:"project_real_process_minutes_manual"`
	ProjectRealProcessMinutesReasonManual *string         `json:"project_real_process_minutes_reason_manual"`
	ProjectRealLeadMinutes                *float64        `json:"project_real_lead_minutes"`
	ProjectRealLeadMinutesReason          *string         `json:"project_real_lead_minutes_reason"`
	ProjectRealLeadMinutesManual          *float64        `json:"project_real_lead_minutes_manual"`
	ProjectRealLeadMinutesReasonManual    *string         `json:"project_real_lead_minutes_reason_manual"`
	CreatedAt                             *time.Time      `json:"created_at"`
	UpdatedAt                             *time.Time      `json:"updated_at"`
}

// ProjectAggregates 项目聚合计算结果
type ProjectAggregates struct {
	StartTime                       *time.Time
	EndTime                         *time.Time
	UpstreamTokens                  int64
	DownstreamTokens                int64
	Cost                            float64
	ProjectAncientMinutes           *float64
	ProjectAncientMinutesReason     string
	ProjectRealProcessMinutes       *float64
	ProjectRealProcessMinutesReason string
	ProjectRealLeadMinutes          *float64
	ProjectRealLeadMinutesReason    string
}

// --- projects 列名与 scan 辅助 ---

var projectSelectColumns = `project_id, name, description,
	repos, task_ids, task_ids_silica,
	start_time, end_time, start_time_manual, end_time_manual,
	upstream_tokens, downstream_tokens, cost,
	project_ancient_minutes, project_ancient_minutes_reason,
	project_ancient_minutes_manual, project_ancient_minutes_reason_manual,
	project_real_process_minutes, project_real_process_minutes_reason,
	project_real_process_minutes_manual, project_real_process_minutes_reason_manual,
	project_real_lead_minutes, project_real_lead_minutes_reason,
	project_real_lead_minutes_manual, project_real_lead_minutes_reason_manual,
	created_at, updated_at`

func scanProject(s rowScanner, m *Project) error {
	var repos, taskIDs, taskIDsSilica *[]byte
	err := s.Scan(
		&m.ProjectID, &m.Name, &m.Description,
		&repos, &taskIDs, &taskIDsSilica,
		&m.StartTime, &m.EndTime, &m.StartTimeManual, &m.EndTimeManual,
		&m.UpstreamTokens, &m.DownstreamTokens, &m.Cost,
		&m.ProjectAncientMinutes, &m.ProjectAncientMinutesReason,
		&m.ProjectAncientMinutesManual, &m.ProjectAncientMinutesReasonManual,
		&m.ProjectRealProcessMinutes, &m.ProjectRealProcessMinutesReason,
		&m.ProjectRealProcessMinutesManual, &m.ProjectRealProcessMinutesReasonManual,
		&m.ProjectRealLeadMinutes, &m.ProjectRealLeadMinutesReason,
		&m.ProjectRealLeadMinutesManual, &m.ProjectRealLeadMinutesReasonManual,
		&m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return err
	}
	if repos != nil {
		m.Repos = json.RawMessage(*repos)
	}
	if taskIDs != nil {
		m.TaskIDs = json.RawMessage(*taskIDs)
	}
	if taskIDsSilica != nil {
		m.TaskIDsSilica = json.RawMessage(*taskIDsSilica)
	}
	return nil
}

// --- projects CRUD ---

// CreateProject 创建虚拟项目，返回新创建的 project_id
func CreateProject(db *sql.DB, p *Project) (string, error) {
	var projectID string
	err := db.QueryRow(`
		INSERT INTO projects (name, description, repos, task_ids, task_ids_silica)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING project_id`,
		p.Name, p.Description,
		jsonRawToString(p.Repos), jsonRawToString(p.TaskIDs), jsonRawToString(p.TaskIDsSilica),
	).Scan(&projectID)
	if err != nil {
		return "", fmt.Errorf("创建 project 失败: %w", err)
	}
	return projectID, nil
}

// GetProject 查询单条 project 记录，不存在返回 nil, nil
func GetProject(db *sql.DB, projectID string) (*Project, error) {
	var m Project
	err := scanProject(db.QueryRow(fmt.Sprintf(`
		SELECT %s FROM projects WHERE project_id = $1`,
		projectSelectColumns),
		projectID,
	), &m)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询 project 失败: %w", err)
	}
	return &m, nil
}

// ListProjects 查询全量 project 列表
func ListProjects(db *sql.DB) ([]Project, error) {
	rows, err := db.Query(fmt.Sprintf(`
		SELECT %s FROM projects ORDER BY updated_at DESC`,
		projectSelectColumns))
	if err != nil {
		return nil, fmt.Errorf("查询 projects 列表失败: %w", err)
	}
	defer rows.Close()

	var list []Project
	for rows.Next() {
		var m Project
		if err := scanProject(rows, &m); err != nil {
			return nil, fmt.Errorf("扫描 projects 行失败: %w", err)
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

// UpdateProject 更新虚拟项目基本信息
func UpdateProject(db *sql.DB, p *Project) error {
	result, err := db.Exec(`
		UPDATE projects SET
			name = $2, description = $3,
			repos = $4, task_ids = $5, task_ids_silica = $6,
			updated_at = CURRENT_TIMESTAMP
		WHERE project_id = $1`,
		p.ProjectID, p.Name, p.Description,
		jsonRawToString(p.Repos), jsonRawToString(p.TaskIDs), jsonRawToString(p.TaskIDsSilica),
	)
	if err != nil {
		return fmt.Errorf("更新 project 失败: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("更新 project 失败: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("project project_id=%s 不存在", p.ProjectID)
	}
	return nil
}

// DeleteProject 删除虚拟项目
func DeleteProject(db *sql.DB, projectID string) error {
	result, err := db.Exec(`DELETE FROM projects WHERE project_id = $1`, projectID)
	if err != nil {
		return fmt.Errorf("删除 project 失败: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("删除 project 失败: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("project project_id=%s 不存在", projectID)
	}
	return nil
}

// UpdateProjectManual 动态更新虚拟项目的人工修正字段
func UpdateProjectManual(db *sql.DB, projectID string, fields map[string]interface{}) error {
	allowed := map[string]bool{
		"project_ancient_minutes_manual":             true,
		"project_ancient_minutes_reason_manual":      true,
		"project_real_process_minutes_manual":        true,
		"project_real_process_minutes_reason_manual": true,
		"project_real_lead_minutes_manual":           true,
		"project_real_lead_minutes_reason_manual":    true,
		"start_time_manual":                          true,
		"end_time_manual":                            true,
	}

	var setClauses []string
	var args []interface{}
	args = append(args, projectID)
	argIdx := 2

	for k, v := range fields {
		if !allowed[k] {
			return fmt.Errorf("不允许更新字段: %s", k)
		}
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", k, argIdx))
		args = append(args, v)
		argIdx++
	}
	if len(setClauses) == 0 {
		return fmt.Errorf("没有需要更新的字段")
	}
	setClauses = append(setClauses, "updated_at = CURRENT_TIMESTAMP")

	query := fmt.Sprintf("UPDATE projects SET %s WHERE project_id = $1",
		strings.Join(setClauses, ", "))
	result, err := db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("更新 project manual 字段失败: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("更新 project manual 字段失败: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("project project_id=%s 不存在", projectID)
	}
	return nil
}

// UpdateProjectAggregates 更新虚拟项目的聚合计算结果
func UpdateProjectAggregates(db *sql.DB, projectID string, agg *ProjectAggregates) error {
	result, err := db.Exec(`
		UPDATE projects SET
			start_time = $2, end_time = $3,
			upstream_tokens = $4, downstream_tokens = $5, cost = $6,
			project_ancient_minutes = $7, project_ancient_minutes_reason = $8,
			project_real_process_minutes = $9, project_real_process_minutes_reason = $10,
			project_real_lead_minutes = $11, project_real_lead_minutes_reason = $12,
			updated_at = CURRENT_TIMESTAMP
		WHERE project_id = $1`,
		projectID,
		agg.StartTime, agg.EndTime,
		agg.UpstreamTokens, agg.DownstreamTokens, agg.Cost,
		agg.ProjectAncientMinutes, agg.ProjectAncientMinutesReason,
		agg.ProjectRealProcessMinutes, agg.ProjectRealProcessMinutesReason,
		agg.ProjectRealLeadMinutes, agg.ProjectRealLeadMinutesReason,
	)
	if err != nil {
		return fmt.Errorf("更新 project 聚合数据失败: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("更新 project 聚合数据失败: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("project project_id=%s 不存在", projectID)
	}
	return nil
}

// --- user_productivity 表结构 ---

type UserProductivity struct {
	UserProductivityID       string          `json:"user_productivity_id"`
	CreateTime               *time.Time      `json:"create_time"`
	UserID                   *string         `json:"user_id"`
	UserName                 *string         `json:"user_name"`
	TaskIDs                  json.RawMessage `json:"task_ids"`
	WorkDirIDs               json.RawMessage `json:"work_dir_ids"`
	TaskDiffLines            *int            `json:"task_diff_lines"`
	UpstreamTokens           *int64          `json:"upstream_tokens"`
	DownstreamTokens         *int64          `json:"downstream_tokens"`
	Cost                     *float64        `json:"cost"`
	TaskRealMinutes          *float64        `json:"task_real_minutes"`
	TaskAncientMinutes       *float64        `json:"task_ancient_minutes"`
	TaskEfficiencyRatio      *float64        `json:"task_efficiency_ratio"`
	CommitIDs                json.RawMessage `json:"commit_ids"`
	CommitDiffLines          *int            `json:"commit_diff_lines"`
	CommitAncientMinutes     *float64        `json:"commit_ancient_minutes"`
	CommitRealAIMinutes      *float64        `json:"commit_real_ai_minutes"`
	CommitRealAncientMinutes *float64        `json:"commit_real_ancient_minutes"`
	CommitRealMinutes        *float64        `json:"commit_real_minutes"`
	CommitEfficiencyRatio    *float64        `json:"commit_efficiency_ratio"`
	CreatedAt                *time.Time      `json:"created_at"`
	UpdatedAt                *time.Time      `json:"updated_at"`
}

var userProductivitySelectColumns = `user_productivity_id, create_time, user_id, user_name,
	task_ids, work_dir_ids, task_diff_lines,
	upstream_tokens, downstream_tokens, cost,
	task_real_minutes, task_ancient_minutes, task_efficiency_ratio,
	commit_ids, commit_diff_lines,
	commit_ancient_minutes, commit_real_ai_minutes, commit_real_ancient_minutes,
	commit_real_minutes, commit_efficiency_ratio,
	created_at, updated_at`

func scanUserProductivity(s rowScanner) (*UserProductivity, error) {
	var m UserProductivity
	var taskIDs, workDirIDs, commitIDs *[]byte
	err := s.Scan(
		&m.UserProductivityID, &m.CreateTime, &m.UserID, &m.UserName,
		&taskIDs, &workDirIDs, &m.TaskDiffLines,
		&m.UpstreamTokens, &m.DownstreamTokens, &m.Cost,
		&m.TaskRealMinutes, &m.TaskAncientMinutes, &m.TaskEfficiencyRatio,
		&commitIDs, &m.CommitDiffLines,
		&m.CommitAncientMinutes, &m.CommitRealAIMinutes, &m.CommitRealAncientMinutes,
		&m.CommitRealMinutes, &m.CommitEfficiencyRatio,
		&m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if taskIDs != nil {
		m.TaskIDs = json.RawMessage(*taskIDs)
	}
	if workDirIDs != nil {
		m.WorkDirIDs = json.RawMessage(*workDirIDs)
	}
	if commitIDs != nil {
		m.CommitIDs = json.RawMessage(*commitIDs)
	}
	return &m, nil
}

// --- user_productivity CRUD ---

// UpsertUserProductivity 插入或更新 user_productivity 记录
func UpsertUserProductivity(db *sql.DB, up *UserProductivity) error {
	_, err := db.Exec(`
		INSERT INTO user_productivity (
			user_productivity_id, create_time, user_id, user_name,
			task_ids, work_dir_ids, task_diff_lines,
			upstream_tokens, downstream_tokens, cost,
			task_real_minutes, task_ancient_minutes, task_efficiency_ratio,
			commit_ids, commit_diff_lines,
			commit_ancient_minutes, commit_real_ai_minutes, commit_real_ancient_minutes,
			commit_real_minutes, commit_efficiency_ratio
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7,
			$8, $9, $10,
			$11, $12, $13,
			$14, $15,
			$16, $17, $18,
			$19, $20
		)
		ON CONFLICT(user_productivity_id) DO UPDATE SET
			create_time = $2, user_id = $3, user_name = $4,
			task_ids = $5, work_dir_ids = $6, task_diff_lines = $7,
			upstream_tokens = $8, downstream_tokens = $9, cost = $10,
			task_real_minutes = $11, task_ancient_minutes = $12, task_efficiency_ratio = $13,
			commit_ids = $14, commit_diff_lines = $15,
			commit_ancient_minutes = $16, commit_real_ai_minutes = $17, commit_real_ancient_minutes = $18,
			commit_real_minutes = $19, commit_efficiency_ratio = $20,
			updated_at = CURRENT_TIMESTAMP`,
		up.UserProductivityID, up.CreateTime, up.UserID, up.UserName,
		jsonRawToString(up.TaskIDs), jsonRawToString(up.WorkDirIDs), up.TaskDiffLines,
		up.UpstreamTokens, up.DownstreamTokens, up.Cost,
		up.TaskRealMinutes, up.TaskAncientMinutes, up.TaskEfficiencyRatio,
		jsonRawToString(up.CommitIDs), up.CommitDiffLines,
		up.CommitAncientMinutes, up.CommitRealAIMinutes, up.CommitRealAncientMinutes,
		up.CommitRealMinutes, up.CommitEfficiencyRatio,
	)
	if err != nil {
		return fmt.Errorf("upsert user_productivity 失败: %w", err)
	}
	return nil
}

// ListUserProductivity 按条件查询 user_productivity 列表
func ListUserProductivity(db *sql.DB, userId, startTime, endTime string, page, pageSize int) ([]UserProductivity, error) {
	var conditions []string
	var args []interface{}
	argIdx := 1

	if userId != "" {
		conditions = append(conditions, fmt.Sprintf("user_id = $%d", argIdx))
		args = append(args, userId)
		argIdx++
	}
	if startTime != "" {
		conditions = append(conditions, fmt.Sprintf("create_time >= $%d", argIdx))
		args = append(args, startTime)
		argIdx++
	}
	if endTime != "" {
		conditions = append(conditions, fmt.Sprintf("create_time <= $%d", argIdx))
		args = append(args, endTime)
		argIdx++
	}

	query := fmt.Sprintf("SELECT %s FROM user_productivity", userProductivitySelectColumns)
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY create_time DESC"
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, pageSize, (page-1)*pageSize)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询 user_productivity 列表失败: %w", err)
	}
	defer rows.Close()

	var list []UserProductivity
	for rows.Next() {
		m, err := scanUserProductivity(rows)
		if err != nil {
			return nil, fmt.Errorf("扫描 user_productivity 行失败: %w", err)
		}
		list = append(list, *m)
	}
	return list, rows.Err()
}

// CountUserProductivity 按条件统计 user_productivity 总数
func CountUserProductivity(db *sql.DB, userId, startTime, endTime string) (int, error) {
	var conditions []string
	var args []interface{}
	argIdx := 1

	if userId != "" {
		conditions = append(conditions, fmt.Sprintf("user_id = $%d", argIdx))
		args = append(args, userId)
		argIdx++
	}
	if startTime != "" {
		conditions = append(conditions, fmt.Sprintf("create_time >= $%d", argIdx))
		args = append(args, startTime)
		argIdx++
	}
	if endTime != "" {
		conditions = append(conditions, fmt.Sprintf("create_time <= $%d", argIdx))
		args = append(args, endTime)
		argIdx++
	}

	query := "SELECT count(*) FROM user_productivity"
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	var count int
	err := db.QueryRow(query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("统计 user_productivity 总数失败: %w", err)
	}
	return count, nil
}

// DeleteUserProductivityByDate 按 create_time 范围删除 user_productivity 数据
func DeleteUserProductivityByDate(db *sql.DB, startDate, endDate string) error {
	_, err := db.Exec(`DELETE FROM user_productivity WHERE create_time >= $1 AND create_time <= $2`,
		startDate, endDate)
	if err != nil {
		return fmt.Errorf("删除 user_productivity 失败: %w", err)
	}
	return nil
}

// --- user_groups 表结构 ---

type UserGroup struct {
	GroupID   string          `json:"group_id"`
	Name      string          `json:"name"`
	OrgName   string          `json:"org_name"`
	UserIDs   json.RawMessage `json:"user_ids"`
	CreatedAt *time.Time      `json:"created_at"`
	UpdatedAt *time.Time      `json:"updated_at"`
}

// --- user_groups CRUD ---

// CreateUserGroup 创建虚拟用户组，返回新创建的 UserGroup
func CreateUserGroup(db *sql.DB, name string, orgName string, userIDs []string) (*UserGroup, error) {
	userIDsJSON, err := json.Marshal(userIDs)
	if err != nil {
		return nil, fmt.Errorf("序列化 user_ids 失败: %w", err)
	}
	var g UserGroup
	var rawUserIDs *[]byte
	err = db.QueryRow(`
		INSERT INTO user_groups (name, org_name, user_ids)
		VALUES ($1, $2, $3)
		RETURNING group_id, name, org_name, user_ids, created_at, updated_at`,
		name, orgName, string(userIDsJSON),
	).Scan(&g.GroupID, &g.Name, &g.OrgName, &rawUserIDs, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("创建 user_group 失败: %w", err)
	}
	if rawUserIDs != nil {
		g.UserIDs = json.RawMessage(*rawUserIDs)
	}
	return &g, nil
}

// ListUserGroups 查询全量 user_groups 列表
func ListUserGroups(db *sql.DB) ([]UserGroup, error) {
	rows, err := db.Query(`SELECT group_id, name, org_name, user_ids, created_at, updated_at FROM user_groups ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("查询 user_groups 列表失败: %w", err)
	}
	defer rows.Close()

	var list []UserGroup
	for rows.Next() {
		var g UserGroup
		var rawUserIDs *[]byte
		if err := rows.Scan(&g.GroupID, &g.Name, &g.OrgName, &rawUserIDs, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, fmt.Errorf("扫描 user_groups 行失败: %w", err)
		}
		if rawUserIDs != nil {
			g.UserIDs = json.RawMessage(*rawUserIDs)
		}
		list = append(list, g)
	}
	return list, rows.Err()
}

// GetUserGroup 查询单条 user_group 记录，不存在返回 nil, nil
func GetUserGroup(db *sql.DB, groupId string) (*UserGroup, error) {
	var g UserGroup
	var rawUserIDs *[]byte
	err := db.QueryRow(`
		SELECT group_id, name, org_name, user_ids, created_at, updated_at
		FROM user_groups WHERE group_id = $1`,
		groupId,
	).Scan(&g.GroupID, &g.Name, &g.OrgName, &rawUserIDs, &g.CreatedAt, &g.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询 user_group 失败: %w", err)
	}
	if rawUserIDs != nil {
		g.UserIDs = json.RawMessage(*rawUserIDs)
	}
	return &g, nil
}

// CommitLightStats 轻量统计结果（用于项目列表）
type CommitLightStats struct {
	UserName  *string
	DiffLines *int
}

// ListCommitLightByRepoRange 轻量查询 commits（只取 user_name, git_user_name, diff_lines）
func ListCommitLightByRepoRange(db *sql.DB, repoAddr, repoBranch, startTime, endTime string) ([]CommitLightStats, error) {
	var conditions []string
	var args []interface{}
	argIdx := 1

	if repoAddr != "" {
		conditions = append(conditions, fmt.Sprintf("repo_addr = $%d", argIdx))
		args = append(args, repoAddr)
		argIdx++
	}
	if repoBranch != "" {
		conditions = append(conditions, fmt.Sprintf("repo_branch = $%d", argIdx))
		args = append(args, repoBranch)
		argIdx++
	}
	if startTime != "" {
		conditions = append(conditions, fmt.Sprintf("commit_time >= $%d", argIdx))
		args = append(args, startTime)
		argIdx++
	}
	if endTime != "" {
		conditions = append(conditions, fmt.Sprintf("commit_time <= $%d", argIdx))
		args = append(args, endTime)
		argIdx++
	}

	query := "SELECT COALESCE(user_name, git_user_name), diff_lines FROM commits"
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("轻量查询 commits 失败: %w", err)
	}
	defer rows.Close()

	var list []CommitLightStats
	for rows.Next() {
		var m CommitLightStats
		if err := rows.Scan(&m.UserName, &m.DiffLines); err != nil {
			return nil, fmt.Errorf("扫描 commit light 失败: %w", err)
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

// DeleteUserGroup 删除虚拟用户组
func DeleteUserGroup(db *sql.DB, groupId string) error {
	result, err := db.Exec(`DELETE FROM user_groups WHERE group_id = $1`, groupId)
	if err != nil {
		return fmt.Errorf("删除 user_group 失败: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("删除 user_group 失败: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("user group not found: %s", groupId)
	}
	return nil
}
