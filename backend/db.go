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

// EnsureStatSchema 确保 costrict_stat 数据库的表结构存在
// 使用 CREATE TABLE IF NOT EXISTS 实现幂等，每次启动时调用
func EnsureStatSchema(db *sql.DB) error {
	// 启用 pgcrypto 扩展（gen_random_uuid 函数需要）
	if _, err := db.Exec(`CREATE EXTENSION IF NOT EXISTS pgcrypto`); err != nil {
		return fmt.Errorf("启用 pgcrypto 扩展失败: %w", err)
	}

	stmts := []string{
		// user_org 表
		`CREATE TABLE IF NOT EXISTS user_org (
			user_id VARCHAR(255) PRIMARY KEY,
			user_name VARCHAR(255),
			org1 VARCHAR(255),
			org2 VARCHAR(255),
			org3 VARCHAR(255),
			org4 VARCHAR(255),
			org5 VARCHAR(255),
			org6 VARCHAR(255),
			org7 VARCHAR(255),
			org8 VARCHAR(255),
			org9 VARCHAR(255),
			git_user_name VARCHAR(255),
			git_user_email VARCHAR(255),
			created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_user_org_user_name ON user_org(user_name)`,
		`CREATE INDEX IF NOT EXISTS idx_user_org_git_user_name ON user_org(git_user_name)`,
		`CREATE INDEX IF NOT EXISTS idx_user_org_git_user_email ON user_org(git_user_email)`,

		// tasks 表
		`CREATE TABLE IF NOT EXISTS tasks (
			task_id VARCHAR(500) PRIMARY KEY,
			user_id VARCHAR(255),
			user_name VARCHAR(255),
			client_id VARCHAR(255),
			client_ide VARCHAR(100),
			client_version VARCHAR(100),
			client_os VARCHAR(100),
			client_os_version VARCHAR(100),
			caller VARCHAR(100),
			repo_addr TEXT,
			repo_branch VARCHAR(500),
			work_dir TEXT,
			work_dir_id VARCHAR(500),
			diff_lines INT,
			start_time TIMESTAMPTZ,
			end_time TIMESTAMPTZ,
			upstream_tokens BIGINT,
			downstream_tokens BIGINT,
			cost FLOAT8,
			task_real_minutes FLOAT8,
			task_real_minutes_reason TEXT,
			task_real_minutes_manual FLOAT8,
			task_real_minutes_reason_manual TEXT,
			task_ancient_minutes FLOAT8,
			task_ancient_minutes_reason TEXT,
			task_ancient_minutes_manual FLOAT8,
			task_ancient_minutes_reason_manual TEXT,
			efficiency_ratio FLOAT8,
			title VARCHAR(200),
			created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_user_id ON tasks(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_work_dir_id ON tasks(work_dir_id)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_start_time ON tasks(start_time)`,

		// task_conversations 表
		`CREATE TABLE IF NOT EXISTS task_conversations (
			id SERIAL PRIMARY KEY,
			task_id VARCHAR(500) NOT NULL,
			request_id VARCHAR(500) NOT NULL,
			sender VARCHAR(50),
			prompt_mode VARCHAR(50),
			mode VARCHAR(100),
			model VARCHAR(200),
			start_time TIMESTAMPTZ,
			end_time TIMESTAMPTZ,
			process_time BIGINT,
			process_ttft BIGINT,
			upstream_tokens BIGINT,
			downstream_tokens BIGINT,
			cost FLOAT8,
			request_content TEXT,
			response_content TEXT,
			user_input TEXT,
			diff TEXT,
			diff_lines BIGINT,
			error_code VARCHAR(100),
			error_reason TEXT,
			created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(task_id, request_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_task_conversations_task_id ON task_conversations(task_id)`,
		`CREATE INDEX IF NOT EXISTS idx_task_conversations_start_time ON task_conversations(start_time)`,

		// commits 表
		`CREATE TABLE IF NOT EXISTS commits (
			commit_id VARCHAR(500) PRIMARY KEY,
			commit_time TIMESTAMPTZ,
			repo_addr TEXT,
			repo_branch VARCHAR(500),
			git_user_name VARCHAR(255),
			git_user_email VARCHAR(255),
			user_id VARCHAR(255),
			user_name VARCHAR(255),
			client_id VARCHAR(255),
			work_dir TEXT,
			diff_lines INT,
			commit_ancient_minutes FLOAT8,
			commit_ancient_minutes_reason TEXT,
			commit_ancient_minutes_manual FLOAT8,
			commit_ancient_minutes_reason_manual TEXT,
			task_ids JSONB,
			task_ids_silica JSONB,
			upstream_tokens BIGINT,
			downstream_tokens BIGINT,
			cost FLOAT8,
			silica FLOAT8,
			commit_real_ai_minutes FLOAT8,
			commit_real_ancient_minutes FLOAT8,
			commit_real_minutes FLOAT8,
			commit_real_minutes_reason TEXT,
			commit_real_minutes_manual FLOAT8,
			commit_real_minutes_reason_manual TEXT,
			comment VARCHAR(150),
			created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_commits_repo_addr ON commits(repo_addr)`,
		`CREATE INDEX IF NOT EXISTS idx_commits_repo_addr_branch ON commits(repo_addr, repo_branch)`,
		`CREATE INDEX IF NOT EXISTS idx_commits_user_id ON commits(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_commits_commit_time ON commits(commit_time)`,

		// projects 表（虚拟项目）
		`CREATE TABLE IF NOT EXISTS projects (
			project_id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
			name VARCHAR(500) NOT NULL,
			description TEXT,
			repos JSONB DEFAULT '[]',
			task_ids JSONB DEFAULT '[]',
			task_ids_silica JSONB DEFAULT '[]',
			start_time TIMESTAMPTZ,
			end_time TIMESTAMPTZ,
			start_time_manual TIMESTAMPTZ,
			end_time_manual TIMESTAMPTZ,
			upstream_tokens BIGINT DEFAULT 0,
			downstream_tokens BIGINT DEFAULT 0,
			cost FLOAT8 DEFAULT 0,
			project_ancient_minutes FLOAT8,
			project_ancient_minutes_reason TEXT,
			project_ancient_minutes_manual FLOAT8,
			project_ancient_minutes_reason_manual TEXT,
			project_real_process_minutes FLOAT8,
			project_real_process_minutes_reason TEXT,
			project_real_process_minutes_manual FLOAT8,
			project_real_process_minutes_reason_manual TEXT,
			project_real_lead_minutes FLOAT8,
			project_real_lead_minutes_reason TEXT,
			project_real_lead_minutes_manual FLOAT8,
			project_real_lead_minutes_reason_manual TEXT,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_projects_name ON projects(name)`,
		`CREATE INDEX IF NOT EXISTS idx_projects_updated_at ON projects(updated_at)`,

		// user_productivity 表（用户生产力预聚合）
		`CREATE TABLE IF NOT EXISTS user_productivity (
			user_productivity_id VARCHAR(500) PRIMARY KEY,
			create_time TIMESTAMPTZ,
			user_id VARCHAR(255),
			user_name VARCHAR(255),
			task_ids JSONB,
			work_dir_ids JSONB,
			task_diff_lines INT,
			upstream_tokens BIGINT,
			downstream_tokens BIGINT,
			cost FLOAT8,
			task_real_minutes FLOAT8,
			task_ancient_minutes FLOAT8,
			task_efficiency_ratio FLOAT8,
			commit_ids JSONB,
			commit_diff_lines INT,
			commit_ancient_minutes FLOAT8,
			commit_real_ai_minutes FLOAT8,
			commit_real_ancient_minutes FLOAT8,
			commit_real_minutes FLOAT8,
			commit_efficiency_ratio FLOAT8,
			created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_user_productivity_user_id ON user_productivity(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_user_productivity_create_time ON user_productivity(create_time)`,

		// user_groups 表（虚拟用户组）
		`CREATE TABLE IF NOT EXISTS user_groups (
			group_id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
			name VARCHAR(500) NOT NULL,
			org_name VARCHAR(200) DEFAULT '',
			user_ids JSONB DEFAULT '[]',
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_user_groups_name ON user_groups(name)`,
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("EnsureStatSchema 执行失败 [%s]: %w", stmt[:60], err)
		}
	}
	return nil
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
	diff_lines,
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
		&m.DiffLines,
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
	WorkDir                          *string         `json:"work_dir"`
	DiffLines                        *int            `json:"diff_lines"`
	CommitAncientMinutes             *float64        `json:"commit_ancient_minutes"`
	CommitAncientMinutesReason       *string         `json:"commit_ancient_minutes_reason"`
	CommitAncientMinutesManual       *float64        `json:"commit_ancient_minutes_manual"`
	CommitAncientMinutesReasonManual *string         `json:"commit_ancient_minutes_reason_manual"`
	TaskIDs                          json.RawMessage `json:"task_ids" swaggertype:"string" example:"[\"task1\", \"task2\"]"`
	TaskIDsSilica                    json.RawMessage `json:"task_ids_silica" swaggertype:"string" example:"[\"task1\", \"task2\"]"`
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
	git_user_name, git_user_email, user_id, user_name, client_id, work_dir,
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
		&m.GitUserName, &m.GitUserEmail, &m.UserID, &m.UserName, &m.ClientID, &m.WorkDir,
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
	Repos                                 json.RawMessage `json:"repos" swaggertype:"string" example:"[{\"repo_addr\":\"https://github.com/example/repo\"}]"`
	TaskIDs                               json.RawMessage `json:"task_ids" swaggertype:"string" example:"[\"task1\", \"task2\"]"`
	TaskIDsSilica                         json.RawMessage `json:"task_ids_silica" swaggertype:"string" example:"[\"1.0\", \"0.5\"]"`
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
func UpdateProjectManual(db *sql.DB, projectID string, req UpdateProjectManualRequest) error {
	var setClauses []string
	var args []interface{}
	args = append(args, projectID)
	argIdx := 2

	if req.ProjectAncientMinutesManual != nil {
		setClauses = append(setClauses, fmt.Sprintf("project_ancient_minutes_manual = $%d", argIdx))
		args = append(args, *req.ProjectAncientMinutesManual)
		argIdx++
	}
	if req.ProjectAncientMinutesReasonManual != nil {
		setClauses = append(setClauses, fmt.Sprintf("project_ancient_minutes_reason_manual = $%d", argIdx))
		args = append(args, *req.ProjectAncientMinutesReasonManual)
		argIdx++
	}
	if req.ProjectRealProcessMinutesManual != nil {
		setClauses = append(setClauses, fmt.Sprintf("project_real_process_minutes_manual = $%d", argIdx))
		args = append(args, *req.ProjectRealProcessMinutesManual)
		argIdx++
	}
	if req.ProjectRealProcessMinutesReasonManual != nil {
		setClauses = append(setClauses, fmt.Sprintf("project_real_process_minutes_reason_manual = $%d", argIdx))
		args = append(args, *req.ProjectRealProcessMinutesReasonManual)
		argIdx++
	}
	if req.ProjectRealLeadMinutesManual != nil {
		setClauses = append(setClauses, fmt.Sprintf("project_real_lead_minutes_manual = $%d", argIdx))
		args = append(args, *req.ProjectRealLeadMinutesManual)
		argIdx++
	}
	if req.ProjectRealLeadMinutesReasonManual != nil {
		setClauses = append(setClauses, fmt.Sprintf("project_real_lead_minutes_reason_manual = $%d", argIdx))
		args = append(args, *req.ProjectRealLeadMinutesReasonManual)
		argIdx++
	}
	if req.StartTimeManual != nil {
		setClauses = append(setClauses, fmt.Sprintf("start_time_manual = $%d", argIdx))
		args = append(args, *req.StartTimeManual)
		argIdx++
	}
	if req.EndTimeManual != nil {
		setClauses = append(setClauses, fmt.Sprintf("end_time_manual = $%d", argIdx))
		args = append(args, *req.EndTimeManual)
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
	TaskIDs                  json.RawMessage `json:"task_ids" swaggertype:"string" example:"[\"task1\"]"`
	WorkDirIDs               json.RawMessage `json:"work_dir_ids" swaggertype:"string" example:"[\"dir1\"]"`
	TaskDiffLines            *int            `json:"task_diff_lines"`
	UpstreamTokens           *int64          `json:"upstream_tokens"`
	DownstreamTokens         *int64          `json:"downstream_tokens"`
	Cost                     *float64        `json:"cost"`
	TaskRealMinutes          *float64        `json:"task_real_minutes"`
	TaskAncientMinutes       *float64        `json:"task_ancient_minutes"`
	TaskEfficiencyRatio      *float64        `json:"task_efficiency_ratio"`
	CommitIDs                json.RawMessage `json:"commit_ids" swaggertype:"string" example:"[\"commit1\"]"`
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

// --- user_groups 表结构 ---

type UserGroup struct {
	GroupID   string          `json:"group_id"`
	Name      string          `json:"name"`
	OrgName   string          `json:"org_name"`
	UserIDs   json.RawMessage `json:"user_ids" swaggertype:"string" example:"[\"user1\", \"user2\"]"`
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
