package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// taskSummary task_summary.json 解析结构
type taskSummary struct {
	TaskID        string `json:"task_id"`
	UserID        string `json:"user_id"`
	UserName      string `json:"user_name"`
	ClientID      string `json:"client_id"`
	ClientIDE     string `json:"client_ide"`
	ClientVersion string `json:"client_version"`
	ClientOS      string `json:"client_os"`
	ClientOSVer   string `json:"client_os_version"`
	Caller        string `json:"caller"`
	RepoAddr      string `json:"repo_addr"`
	RepoBranch    string `json:"repo_branch"`
	WorkDir       string `json:"work_dir"`
	Diff          string `json:"diff"`
	DiffLines     int    `json:"diff_lines"`
}

// taskConversation task_conversation.jsonl 每行解析结构
type taskConversation struct {
	Sender           string   `json:"sender"`
	RequestID        string   `json:"request_id"`
	PromptMode       string   `json:"prompt_mode"`
	Mode             string   `json:"mode"`
	Model            string   `json:"model"`
	StartTime        string   `json:"start_time"`
	EndTime          string   `json:"end_time"`
	ProcessTime      int64    `json:"process_time"`
	ProcessTTFT      int64    `json:"process_ttft"`
	UpstreamTokens   int64    `json:"upstream_tokens"`
	DownstreamTokens int64    `json:"downstream_tokens"`
	Cost             float64  `json:"cost"`
	RequestContent   string   `json:"request_content"`
	ResponseContent  string   `json:"response_content"`
	UserInput        string   `json:"user_input"`
	Diff             string   `json:"diff"`
	DiffLines        int64    `json:"diff_lines"`
	ErrorCode        *string  `json:"error_code"`
	ErrorReason      *string  `json:"error_reason"`
}

var (
	reImportNonSafe    = regexp.MustCompile(`[^a-z0-9\-.]`)
	reImportMultiDash  = regexp.MustCompile(`-{2,}`)
)

// importGenerateWorkDirID 根据 clientID 和 workDir 生成工作目录唯一标识
// 算法与 backend/id_utils.go 中的 generateWorkDirID 一致
func importGenerateWorkDirID(clientID, workDir string) string {
	prefix := clientID
	if len(prefix) > 6 {
		prefix = prefix[:6]
	}

	suffix := workDir
	if suffix != "" {
		suffix = strings.ToLower(suffix)
		suffix = reImportNonSafe.ReplaceAllString(suffix, "-")
		suffix = reImportMultiDash.ReplaceAllString(suffix, "-")
		suffix = strings.Trim(suffix, "-")
	}

	if prefix == "" && suffix == "" {
		return ""
	}
	if prefix == "" {
		return suffix
	}
	if suffix == "" {
		return prefix
	}
	return prefix + "-" + suffix
}

// runImportTasks 执行 import-tasks 命令，扫描本地 task 目录并导入到 costrict_stat 数据库
func runImportTasks(config *Config, args []string) {
	taskDir := parseFlag(args, "task-dir", "./task")

	summaryDir := filepath.Join(taskDir, "summary")
	conversationDir := filepath.Join(taskDir, "conversation")
	analysedDir := filepath.Join(taskDir, "analysed")

	// 检查 summary 目录是否存在
	if _, err := os.Stat(summaryDir); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "错误: summary目录不存在: %s\n", summaryDir)
		os.Exit(1)
	}

	// 连接数据库
	db, err := sql.Open("postgres", config.StatDatabase.DSN())
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: 连接数据库失败: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		fmt.Fprintf(os.Stderr, "错误: 数据库连接测试失败: %v\n", err)
		os.Exit(1)
	}

	// 扫描所有 summary JSON 文件
	var summaryFiles []string
	err = filepath.Walk(summaryDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".json") {
			summaryFiles = append(summaryFiles, path)
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: 扫描summary目录失败: %v\n", err)
		os.Exit(1)
	}

	if len(summaryFiles) == 0 {
		fmt.Println("没有找到待导入的 summary 文件")
		return
	}

	successCount := 0
	failCount := 0

	for _, summaryPath := range summaryFiles {
		if err := importSingleTask(db, summaryPath, summaryDir, conversationDir, analysedDir); err != nil {
			fmt.Fprintf(os.Stderr, "导入失败 [%s]: %v\n", summaryPath, err)
			failCount++
		} else {
			successCount++
		}
	}

	fmt.Printf("导入完成: 成功 %d 个，失败 %d 个\n", successCount, failCount)
}

// importSingleTask 导入单个 task
func importSingleTask(db *sql.DB, summaryPath, summaryDir, conversationDir, analysedDir string) error {
	// 解析 summary JSON
	data, err := os.ReadFile(summaryPath)
	if err != nil {
		return fmt.Errorf("读取summary文件失败: %w", err)
	}

	var summary taskSummary
	if err := json.Unmarshal(data, &summary); err != nil {
		return fmt.Errorf("解析summary JSON失败: %w", err)
	}

	if summary.TaskID == "" {
		return fmt.Errorf("task_id为空")
	}

	// 查找对应的 conversation 文件
	relPath, err := filepath.Rel(summaryDir, summaryPath)
	if err != nil {
		return fmt.Errorf("计算相对路径失败: %w", err)
	}
	convRelPath := strings.TrimSuffix(relPath, ".json") + ".jsonl"
	convPath := filepath.Join(conversationDir, convRelPath)

	// 解析 conversation
	var conversations []taskConversation
	if _, err := os.Stat(convPath); err == nil {
		conversations, err = parseConversationFile(convPath)
		if err != nil {
			return fmt.Errorf("解析conversation文件失败: %w", err)
		}
	}

	// 从 conversation 累加计算聚合字段
	var startTime, endTime *time.Time
	var totalUpstream, totalDownstream int64
	var totalCost float64

	for _, conv := range conversations {
		if conv.StartTime != "" {
			if t, err := time.Parse(time.RFC3339, conv.StartTime); err == nil {
				if startTime == nil || t.Before(*startTime) {
					startTime = &t
				}
			}
		}
		if conv.EndTime != "" {
			if t, err := time.Parse(time.RFC3339, conv.EndTime); err == nil {
				if endTime == nil || t.After(*endTime) {
					endTime = &t
				}
			}
		}
		totalUpstream += conv.UpstreamTokens
		totalDownstream += conv.DownstreamTokens
		totalCost += conv.Cost
	}

	// 生成 work_dir_id
	workDirID := importGenerateWorkDirID(summary.ClientID, summary.WorkDir)

	// 写入 tasks 表
	_, err = db.Exec(`INSERT INTO tasks (
		task_id, user_id, user_name, client_id, client_ide, client_version,
		client_os, client_os_version, caller,
		repo_addr, repo_branch, work_dir, work_dir_id,
		diff, diff_lines,
		start_time, end_time, upstream_tokens, downstream_tokens, cost,
		updated_at
	) VALUES (
		$1, $2, $3, $4, $5, $6,
		$7, $8, $9,
		$10, $11, $12, $13,
		$14, $15,
		$16, $17, $18, $19, $20,
		CURRENT_TIMESTAMP
	) ON CONFLICT (task_id) DO UPDATE SET
		user_id = EXCLUDED.user_id, user_name = EXCLUDED.user_name,
		client_id = EXCLUDED.client_id, client_ide = EXCLUDED.client_ide,
		client_version = EXCLUDED.client_version,
		client_os = EXCLUDED.client_os, client_os_version = EXCLUDED.client_os_version,
		caller = EXCLUDED.caller,
		repo_addr = EXCLUDED.repo_addr, repo_branch = EXCLUDED.repo_branch,
		work_dir = EXCLUDED.work_dir, work_dir_id = EXCLUDED.work_dir_id,
		diff = EXCLUDED.diff, diff_lines = EXCLUDED.diff_lines,
		start_time = EXCLUDED.start_time, end_time = EXCLUDED.end_time,
		upstream_tokens = EXCLUDED.upstream_tokens, downstream_tokens = EXCLUDED.downstream_tokens,
		cost = EXCLUDED.cost,
		updated_at = CURRENT_TIMESTAMP`,
		summary.TaskID, summary.UserID, summary.UserName,
		summary.ClientID, summary.ClientIDE, summary.ClientVersion,
		summary.ClientOS, summary.ClientOSVer, summary.Caller,
		summary.RepoAddr, summary.RepoBranch, summary.WorkDir, workDirID,
		summary.Diff, summary.DiffLines,
		startTime, endTime, totalUpstream, totalDownstream, totalCost,
	)
	if err != nil {
		return fmt.Errorf("写入tasks表失败: %w", err)
	}

	// 事务批量写入 task_conversations 表
	if len(conversations) > 0 {
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("开启事务失败: %w", err)
		}

		for _, conv := range conversations {
			var convStartTime, convEndTime *time.Time
			if conv.StartTime != "" {
				if t, err := time.Parse(time.RFC3339, conv.StartTime); err == nil {
					convStartTime = &t
				}
			}
			if conv.EndTime != "" {
				if t, err := time.Parse(time.RFC3339, conv.EndTime); err == nil {
					convEndTime = &t
				}
			}

			_, err = tx.Exec(`INSERT INTO task_conversations (
				task_id, request_id, sender, prompt_mode, mode, model,
				start_time, end_time, process_time, process_ttft,
				upstream_tokens, downstream_tokens, cost,
				request_content, response_content, user_input,
				diff, diff_lines,
				error_code, error_reason
			) VALUES (
				$1, $2, $3, $4, $5, $6,
				$7, $8, $9, $10,
				$11, $12, $13,
				$14, $15, $16,
				$17, $18,
				$19, $20
			) ON CONFLICT (task_id, request_id) DO NOTHING`,
				summary.TaskID, conv.RequestID, conv.Sender, conv.PromptMode, conv.Mode, conv.Model,
				convStartTime, convEndTime, conv.ProcessTime, conv.ProcessTTFT,
				conv.UpstreamTokens, conv.DownstreamTokens, conv.Cost,
				conv.RequestContent, conv.ResponseContent, conv.UserInput,
				conv.Diff, conv.DiffLines,
				conv.ErrorCode, conv.ErrorReason,
			)
			if err != nil {
				tx.Rollback()
				return fmt.Errorf("写入task_conversations表失败: %w", err)
			}
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("提交事务失败: %w", err)
		}
	}

	// 导入成功，移动 summary 文件到 analysed 目录
	analysedPath := filepath.Join(analysedDir, relPath)
	analysedParent := filepath.Dir(analysedPath)
	if err := os.MkdirAll(analysedParent, 0755); err != nil {
		return fmt.Errorf("创建analysed目录失败: %w", err)
	}
	if err := os.Rename(summaryPath, analysedPath); err != nil {
		return fmt.Errorf("移动summary文件到analysed失败: %w", err)
	}

	fmt.Printf("导入成功: %s\n", summary.TaskID)
	return nil
}

// parseConversationFile 解析 .jsonl 文件为 conversation 列表
func parseConversationFile(path string) ([]taskConversation, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var convs []taskConversation
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var conv taskConversation
		if err := json.Unmarshal([]byte(line), &conv); err != nil {
			return nil, fmt.Errorf("第%d行JSON解析失败: %w", lineNum, err)
		}
		convs = append(convs, conv)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}
	return convs, nil
}
