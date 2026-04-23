package main

import (
	"bufio"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	_ "github.com/lib/pq"

	"github.com/spf13/cobra"
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
	Sender           string     `json:"sender"`
	RequestID        string     `json:"request_id"`
	PromptMode       string     `json:"prompt_mode"`
	Mode             string     `json:"mode"`
	Model            string     `json:"model"`
	StartTime        string     `json:"start_time"`
	EndTime          string     `json:"end_time"`
	ProcessTime      int64      `json:"process_time"`
	ProcessTTFT      int64      `json:"process_ttft"`
	UpstreamTokens   int64      `json:"upstream_tokens"`
	DownstreamTokens int64      `json:"downstream_tokens"`
	Cost             float64    `json:"cost"`
	RequestContent   string     `json:"request_content"`
	ResponseContent  string     `json:"response_content"`
	UserInput        string     `json:"user_input"`
	Diff             string     `json:"diff"`
	DiffLines        int64      `json:"diff_lines"`
	ErrorCode        flexString `json:"error_code"`
	ErrorReason      flexString `json:"error_reason"`
	// 内部使用的计算字段
	calculatedCost float64 // 计算得出的cost（如果原始cost为0或缺失）
}

type flexString string

func (f *flexString) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*f = ""
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*f = flexString(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(data, &n); err == nil {
		*f = flexString(n.String())
		return nil
	}
	return fmt.Errorf("flexString: cannot unmarshal %s", string(data))
}

func flexStrPtr(s flexString) *string {
	if s == "" {
		return nil
	}
	str := string(s)
	return &str
}

type taskRecord struct {
	TaskID                string
	UserID                string
	UserName              string
	ClientID              string
	ClientIDE             string
	ClientVersion         string
	ClientOS              string
	ClientOSVer           string
	Caller                string
	RepoAddr              string
	RepoBranch            string
	WorkDir               string
	WorkDirID             string
	DiffLines             int
	StartTime             *time.Time
	EndTime               *time.Time
	UpstreamTokens        int64
	DownstreamTokens      int64
	Cost                  float64
	TaskRealMinutes       float64
	TaskRealMinutesRsn    string
	TaskAncientMinutes    float64
	TaskAncientMinutesRsn string
}

func calcTaskRecord(summary *taskSummary, conversations []taskConversation) taskRecord {
	rec := taskRecord{
		TaskID:        summary.TaskID,
		UserID:        summary.UserID,
		UserName:      summary.UserName,
		ClientID:      summary.ClientID,
		ClientIDE:     summary.ClientIDE,
		ClientVersion: summary.ClientVersion,
		ClientOS:      summary.ClientOS,
		ClientOSVer:   summary.ClientOSVer,
		Caller:        summary.Caller,
		RepoAddr:      summary.RepoAddr,
		RepoBranch:    summary.RepoBranch,
		WorkDir:       summary.WorkDir,
		WorkDirID:     importGenerateWorkDirID(summary.ClientID, summary.WorkDir),
		DiffLines:     summary.DiffLines,
	}

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

	rec.StartTime = startTime
	rec.EndTime = endTime
	rec.UpstreamTokens = totalUpstream
	rec.DownstreamTokens = totalDownstream
	rec.Cost = totalCost

	realMinutes, realReason := calculateImportTaskRealMinutes(conversations, 30, 5)
	rec.TaskRealMinutes = realMinutes
	rec.TaskRealMinutesRsn = realReason

	if summary.DiffLines > 0 || len(conversations) > 0 {
		ancientMinutes, ancientReason := importEstimateAncientMinutes(summary.DiffLines)
		rec.TaskAncientMinutes = ancientMinutes
		rec.TaskAncientMinutesRsn = ancientReason
	}

	return rec
}

var (
	reImportNonSafe   = regexp.MustCompile(`[^a-z0-9\-]`)
	reImportMultiDash = regexp.MustCompile(`-{2,}`)
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

var importTasksCmd = &cobra.Command{
	Use:   "import-task",
	Short: "导入 task 数据到 costrict_stat 数据库",
	RunE: func(cmd *cobra.Command, args []string) error {
		taskDir, _ := cmd.Flags().GetString("task-dir")
		analysedDir, _ := cmd.Flags().GetString("analysed-dir")

		summaryDir := filepath.Join(taskDir, "summary")
		conversationDir := filepath.Join(taskDir, "conversation")

		// 检查 summary 目录是否存在
		if _, err := os.Stat(summaryDir); os.IsNotExist(err) {
			return fmt.Errorf("summary目录不存在: %s", summaryDir)
		}

		// 连接数据库
		db, err := sql.Open("postgres", cfg.StatDatabase.DSN())
		if err != nil {
			return fmt.Errorf("连接数据库失败: %w", err)
		}
		defer db.Close()

		if err := db.Ping(); err != nil {
			return fmt.Errorf("数据库连接测试失败: %w", err)
		}

		// 自动建表：确保 tasks 和 task_conversations 表存在
		if err := ensureImportTables(db); err != nil {
			return fmt.Errorf("自动建表失败: %w", err)
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
			return fmt.Errorf("扫描summary目录失败: %w", err)
		}

		if len(summaryFiles) == 0 {
			fmt.Println("没有找到待导入的 summary 文件")
			return nil
		}

		successCount := 0
		failCount := 0
		skipCount := 0

		force, _ := cmd.Flags().GetBool("force")

		for _, summaryPath := range summaryFiles {
			relPath, err := filepath.Rel(summaryDir, summaryPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "计算相对路径失败 [%s]: %v\n", summaryPath, err)
				failCount++
				continue
			}
			fpRelPath := strings.TrimSuffix(relPath, ".json") + ".fp"
			fpPath := filepath.Join(analysedDir, "task", "summary", fpRelPath)
			if !force {
				if _, err := os.Stat(fpPath); err == nil {
					fmt.Printf("跳过(fp已存在): %s\n", summaryPath)
					skipCount++
					continue
				}
			}
			convRelPath := strings.TrimSuffix(relPath, ".json") + ".jsonl"
			conversationPath := filepath.Join(conversationDir, convRelPath)

			if err := importSingleTask(db, summaryPath, conversationPath, fpPath); err != nil {
				fmt.Fprintf(os.Stderr, "导入失败 [%s]: %v\n", summaryPath, err)
				failCount++
			} else {
				successCount++
			}
		}

		fmt.Printf("导入完成: 成功 %d 个，失败 %d 个，跳过 %d 个\n", successCount, failCount, skipCount)
		return nil
	},
}

func init() {
	importTasksCmd.Flags().String("task-dir", "./task", "task 目录路径")
	importTasksCmd.Flags().String("analysed-dir", "./analysed", "输出目录路径")
	importTasksCmd.Flags().BoolP("force", "f", false, "强制重新导入，忽略fp文件的短路判断")
	rootCmd.AddCommand(importTasksCmd)
}

// importSingleTask 导入单个 task
func importSingleTask(db *sql.DB, summaryPath, conversationPath, fpPath string) error {
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

	var conversations []taskConversation
	if _, err := os.Stat(conversationPath); err == nil {
		conversations, err = parseConversationFile(conversationPath, cfg.ModelPrices)
		if err != nil {
			return fmt.Errorf("解析conversation文件失败: %w", err)
		}
	}

	rec := calcTaskRecord(&summary, conversations)

	_, err = db.Exec(`INSERT INTO tasks (
		task_id, user_id, user_name, client_id, client_ide, client_version,
		client_os, client_os_version, caller,
		repo_addr, repo_branch, work_dir, work_dir_id,
		diff_lines,
		start_time, end_time, upstream_tokens, downstream_tokens, cost,
		task_real_minutes, task_real_minutes_reason,
		task_ancient_minutes, task_ancient_minutes_reason,
		efficiency_ratio,
		updated_at
	) VALUES (
		$1, $2, $3, $4, $5, $6,
		$7, $8, $9,
		$10, $11, $12, $13,
		$14,
		$15, $16, $17, $18, $19,
		$20, $21,
		$22, $23,
		CASE WHEN $20 > 0 AND $22 > 0 THEN $22 / $20 * 100 ELSE NULL END,
		CURRENT_TIMESTAMP
	) ON CONFLICT (task_id) DO UPDATE SET
		user_id = EXCLUDED.user_id, user_name = EXCLUDED.user_name,
		client_id = EXCLUDED.client_id, client_ide = EXCLUDED.client_ide,
		client_version = EXCLUDED.client_version,
		client_os = EXCLUDED.client_os, client_os_version = EXCLUDED.client_os_version,
		caller = EXCLUDED.caller,
		repo_addr = EXCLUDED.repo_addr, repo_branch = EXCLUDED.repo_branch,
		work_dir = EXCLUDED.work_dir, work_dir_id = EXCLUDED.work_dir_id,
		diff_lines = EXCLUDED.diff_lines,
		start_time = EXCLUDED.start_time, end_time = EXCLUDED.end_time,
		upstream_tokens = EXCLUDED.upstream_tokens, downstream_tokens = EXCLUDED.downstream_tokens,
		cost = EXCLUDED.cost,
		task_real_minutes = CASE WHEN tasks.task_real_minutes IS NULL AND tasks.task_real_minutes_manual IS NULL THEN EXCLUDED.task_real_minutes ELSE tasks.task_real_minutes END,
		task_real_minutes_reason = CASE WHEN tasks.task_real_minutes IS NULL AND tasks.task_real_minutes_manual IS NULL THEN EXCLUDED.task_real_minutes_reason ELSE tasks.task_real_minutes_reason END,
		task_ancient_minutes = CASE WHEN tasks.task_ancient_minutes IS NULL AND tasks.task_ancient_minutes_manual IS NULL THEN EXCLUDED.task_ancient_minutes ELSE tasks.task_ancient_minutes END,
		task_ancient_minutes_reason = CASE WHEN tasks.task_ancient_minutes IS NULL AND tasks.task_ancient_minutes_manual IS NULL THEN EXCLUDED.task_ancient_minutes_reason ELSE tasks.task_ancient_minutes_reason END,
		efficiency_ratio = CASE
			WHEN COALESCE(tasks.task_real_minutes_manual, tasks.task_real_minutes) > 0 AND COALESCE(tasks.task_ancient_minutes_manual, tasks.task_ancient_minutes) > 0
			THEN COALESCE(tasks.task_ancient_minutes_manual, tasks.task_ancient_minutes) / COALESCE(tasks.task_real_minutes_manual, tasks.task_real_minutes) * 100
			ELSE NULL
		END,
		updated_at = CURRENT_TIMESTAMP`,
		rec.TaskID, rec.UserID, rec.UserName,
		rec.ClientID, rec.ClientIDE, rec.ClientVersion,
		rec.ClientOS, rec.ClientOSVer, rec.Caller,
		rec.RepoAddr, rec.RepoBranch, rec.WorkDir, rec.WorkDirID,
		rec.DiffLines,
		rec.StartTime, rec.EndTime, rec.UpstreamTokens, rec.DownstreamTokens, rec.Cost,
		rec.TaskRealMinutes, rec.TaskRealMinutesRsn,
		rec.TaskAncientMinutes, rec.TaskAncientMinutesRsn,
	)
	if err != nil {
		return fmt.Errorf("写入tasks表失败: %w", err)
	}

	if rec.TaskRealMinutes > 0 {
		fmt.Printf("  task_real_minutes=%.1f (%s)\n", rec.TaskRealMinutes, rec.TaskRealMinutesRsn)
	}
	if rec.TaskAncientMinutes > 0 {
		fmt.Printf("  task_ancient_minutes=%.1f (%s)\n", rec.TaskAncientMinutes, rec.TaskAncientMinutesRsn)
	}

	if len(conversations) > 0 {
		if err := saveConversations(db, rec.TaskID, conversations); err != nil {
			return fmt.Errorf("保存conversations失败: %w", err)
		}
	}

	if err := generateFingerprintFile(&summary, conversations, fpPath); err != nil {
		fmt.Fprintf(os.Stderr, "警告: 生成指纹文件失败 [%s]: %v\n", rec.TaskID, err)
	}

	silicaJSONPath := strings.TrimSuffix(fpPath, ".fp") + ".json"
	if err := generateSilicaSummaryFile(&summary, rec.StartTime, silicaJSONPath); err != nil {
		fmt.Fprintf(os.Stderr, "警告: 生成silica摘要文件失败 [%s]: %v\n", rec.TaskID, err)
	}

	fmt.Printf("导入成功: %s\n", rec.TaskID)
	return nil
}

func saveConversations(db *sql.DB, taskID string, conversations []taskConversation) error {
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
			'', $17,
			$18, $19
		) ON CONFLICT (task_id, request_id) DO NOTHING`,
			taskID, conv.RequestID, conv.Sender, conv.PromptMode, conv.Mode, conv.Model,
			convStartTime, convEndTime, conv.ProcessTime, conv.ProcessTTFT,
			conv.UpstreamTokens, conv.DownstreamTokens, conv.Cost,
			conv.RequestContent, conv.ResponseContent, conv.UserInput,
			conv.DiffLines,
			flexStrPtr(conv.ErrorCode), flexStrPtr(conv.ErrorReason),
		)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("写入task_conversations表失败: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}
	return nil
}

// ensureImportTables 确保导入所需的数据库表存在，不存在则自动创建
func ensureImportTables(db *sql.DB) error {
	// 创建 tasks 表
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS tasks (
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
	)`)
	if err != nil {
		return fmt.Errorf("创建tasks表失败: %w", err)
	}

	// 创建 task_conversations 表
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS task_conversations (
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
	)`)
	if err != nil {
		return fmt.Errorf("创建task_conversations表失败: %w", err)
	}

	// 创建索引（IF NOT EXISTS 保证幂等）
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_tasks_user_id ON tasks(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_work_dir_id ON tasks(work_dir_id)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_start_time ON tasks(start_time)`,
		`CREATE INDEX IF NOT EXISTS idx_task_conversations_task_id ON task_conversations(task_id)`,
		`CREATE INDEX IF NOT EXISTS idx_task_conversations_start_time ON task_conversations(start_time)`,
	}
	for _, idx := range indexes {
		if _, err := db.Exec(idx); err != nil {
			fmt.Fprintf(os.Stderr, "警告: 创建索引失败(可忽略): %v\n", err)
		}
	}

	return nil
}

func importEstimateAncientMinutes(diffLines int) (float64, string) {
	if diffLines <= 0 {
		return 5, "默认估算:无代码变更"
	}
	minutes := float64(diffLines) * 1.5
	if minutes < 5 {
		minutes = 5
	}
	return minutes, fmt.Sprintf("基于diff_lines=%d估算(1.5分钟/行)", diffLines)
}

func calculateImportTaskRealMinutes(conversations []taskConversation, gapThreshold, extensionMin int) (float64, string) {
	var validTimes []time.Time
	for _, conv := range conversations {
		if conv.StartTime != "" {
			if t, err := time.Parse(time.RFC3339, conv.StartTime); err == nil {
				validTimes = append(validTimes, t)
			}
		}
	}
	if len(validTimes) == 0 {
		return 0, "无有效对话"
	}
	if len(validTimes) == 1 {
		return float64(extensionMin), fmt.Sprintf("仅1条对话，默认%d分钟", extensionMin)
	}
	sort.Slice(validTimes, func(i, j int) bool {
		return validTimes[i].Before(validTimes[j])
	})
	gapDur := time.Duration(gapThreshold) * time.Minute
	ext := time.Duration(extensionMin) * time.Minute
	type timeSeg struct {
		start     time.Time
		end       time.Time
		convCount int
	}
	segments := []timeSeg{{start: validTimes[0], end: validTimes[0], convCount: 1}}
	for i := 1; i < len(validTimes); i++ {
		cur := &segments[len(segments)-1]
		gap := validTimes[i].Sub(cur.end)
		if gap <= gapDur {
			cur.end = validTimes[i]
			cur.convCount++
		} else {
			cur.end = cur.end.Add(ext)
			segments = append(segments, timeSeg{start: validTimes[i], end: validTimes[i], convCount: 1})
		}
	}
	segments[len(segments)-1].end = segments[len(segments)-1].end.Add(ext)
	var totalMinutes float64
	var parts []string
	for _, seg := range segments {
		mins := seg.end.Sub(seg.start).Minutes()
		totalMinutes += mins
		parts = append(parts, fmt.Sprintf("%s~%s(%d条对话)",
			seg.start.Format("2006-01-02 15:04"),
			seg.end.Format("2006-01-02 15:04"),
			seg.convCount))
	}
	reason := fmt.Sprintf("%d个时间片段: [%s]", len(segments), strings.Join(parts, ", "))
	return totalMinutes, reason
}

// calculateCost 计算 API 调用费用
func calculateCost(model string, inTokens, outTokens int64, prices map[string]ModelPrice) float64 {
	price, ok := prices[model]
	if !ok {
		return 0
	}
	return (float64(inTokens)/1e6)*price.InPrice + (float64(outTokens)/1e6)*price.OutPrice
}

// parseConversationFile 解析 .jsonl 文件为 conversation 列表，并计算cost（如果缺失）
func parseConversationFile(path string, modelPrices map[string]ModelPrice) ([]taskConversation, error) {
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

		// 计算cost：如果原始cost为0或缺失，则根据tokens计算
		if conv.Cost == 0 && conv.UpstreamTokens > 0 && conv.Model != "" {
			conv.calculatedCost = calculateCost(conv.Model, conv.UpstreamTokens, conv.DownstreamTokens, modelPrices)
			if conv.calculatedCost > 0 {
				conv.Cost = conv.calculatedCost
			}
		} else {
			conv.calculatedCost = conv.Cost
		}

		convs = append(convs, conv)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}
	return convs, nil
}

type addedLine struct {
	FilePath string
	Content  string
}

type diffJSONEntry struct {
	File      string `json:"file"`
	Before    string `json:"before"`
	After     string `json:"after"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Status    string `json:"status"`
}

func extractAddedLinesFromDiff(diffText string) []addedLine {
	if strings.TrimSpace(diffText) == "" {
		return nil
	}

	var jsonDiff []diffJSONEntry
	if err := json.Unmarshal([]byte(diffText), &jsonDiff); err == nil && len(jsonDiff) > 0 {
		hasExpected := false
		for _, d := range jsonDiff {
			if d.File != "" || d.After != "" || d.Before != "" || d.Additions > 0 || d.Deletions > 0 {
				hasExpected = true
				break
			}
		}
		if hasExpected {
			return extractFromJSONDiff(jsonDiff)
		}
	}

	if strings.Contains(diffText, "<<< BEFORE") && strings.Contains(diffText, ">>> AFTER") {
		return extractFromBeforeAfterDiff(diffText)
	}

	return extractFromUnifiedDiff(diffText)
}

func extractFromUnifiedDiff(diffText string) []addedLine {
	var result []addedLine
	var currentFile string

	for _, line := range strings.Split(diffText, "\n") {
		if strings.HasPrefix(line, "+++ b/") {
			currentFile = line[6:]
			continue
		}
		if strings.HasPrefix(line, "+++ /dev/null") {
			currentFile = ""
			continue
		}
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			content := line[1:]
			trimmed := strings.TrimSpace(content)
			if trimmed != "" {
				result = append(result, addedLine{FilePath: currentFile, Content: trimmed})
			}
		}
	}
	return result
}

func extractFromJSONDiff(jsonDiff []diffJSONEntry) []addedLine {
	var result []addedLine
	for _, d := range jsonDiff {
		if d.After == "" {
			continue
		}
		beforeLines := make(map[string]bool)
		if d.Before != "" {
			for _, line := range strings.Split(d.Before, "\n") {
				trimmed := strings.TrimSpace(line)
				if trimmed != "" {
					beforeLines[trimmed] = true
				}
			}
		}
		for _, line := range strings.Split(d.After, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && !beforeLines[trimmed] {
				result = append(result, addedLine{FilePath: d.File, Content: trimmed})
			}
		}
	}
	return result
}

func extractFromBeforeAfterDiff(diffText string) []addedLine {
	var result []addedLine
	var currentFile string
	var beforeContent, afterContent strings.Builder
	var inBefore, inAfter bool

	for _, line := range strings.Split(diffText, "\n") {
		if strings.HasPrefix(line, "--- ") {
			if afterContent.Len() > 0 || beforeContent.Len() > 0 {
				result = append(result, computeDiffAddedLines(currentFile, beforeContent.String(), afterContent.String())...)
				beforeContent.Reset()
				afterContent.Reset()
			}
			currentFile = strings.TrimSpace(line[4:])
			inBefore = false
			inAfter = false
			continue
		}
		if strings.TrimSpace(line) == "<<< BEFORE" {
			inBefore = true
			inAfter = false
			continue
		}
		if strings.TrimSpace(line) == ">>> AFTER" {
			inBefore = false
			inAfter = true
			continue
		}
		if inBefore {
			beforeContent.WriteString(line)
			beforeContent.WriteByte('\n')
		} else if inAfter {
			afterContent.WriteString(line)
			afterContent.WriteByte('\n')
		}
	}
	if afterContent.Len() > 0 || beforeContent.Len() > 0 {
		result = append(result, computeDiffAddedLines(currentFile, beforeContent.String(), afterContent.String())...)
	}
	return result
}

func computeDiffAddedLines(filePath, before, after string) []addedLine {
	beforeLines := make(map[string]bool)
	for _, line := range strings.Split(before, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			beforeLines[trimmed] = true
		}
	}
	var result []addedLine
	for _, line := range strings.Split(after, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !beforeLines[trimmed] {
			result = append(result, addedLine{FilePath: filePath, Content: trimmed})
		}
	}
	return result
}

func generateSilicaSummaryFile(summary *taskSummary, startTime *time.Time, jsonPath string) error {
	s := silicaTaskSummary{
		TaskID:   summary.TaskID,
		RepoAddr: summary.RepoAddr,
		UserID:   summary.UserID,
	}
	if startTime != nil {
		s.StartTime = startTime.Format(time.RFC3339)
	}

	data, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("序列化silica摘要失败: %w", err)
	}

	if err := os.WriteFile(jsonPath, data, 0644); err != nil {
		return fmt.Errorf("写入silica摘要文件失败: %w", err)
	}

	fmt.Printf("  silica摘要已生成: %s\n", jsonPath)
	return nil
}

func generateFingerprintFile(summary *taskSummary, conversations []taskConversation, fpPath string) error {
	diffText := summary.Diff
	source := "summary"

	if strings.TrimSpace(diffText) == "" {
		var diffs []string
		for _, conv := range conversations {
			if strings.TrimSpace(conv.Diff) != "" {
				diffs = append(diffs, conv.Diff)
			}
		}
		if len(diffs) > 0 {
			diffText = strings.Join(diffs, "\n")
			source = "conversation"
		}
	}

	dir := filepath.Dir(fpPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建fp目录失败: %w", err)
	}

	if strings.TrimSpace(diffText) == "" {
		if err := os.WriteFile(fpPath, []byte{}, 0644); err != nil {
			return fmt.Errorf("写入fp文件失败: %w", err)
		}
		fmt.Printf("  fp文件已生成(无diff数据, 来源:%s): %s\n", source, fpPath)
		return nil
	}

	addedLines := extractAddedLinesFromDiff(diffText)
	if len(addedLines) == 0 {
		if err := os.WriteFile(fpPath, []byte{}, 0644); err != nil {
			return fmt.Errorf("写入fp文件失败: %w", err)
		}
		fmt.Printf("  fp文件已生成(无新增行, 来源:%s): %s\n", source, fpPath)
		return nil
	}

	var sb strings.Builder
	for _, al := range addedLines {
		hash := sha256.Sum256([]byte(removeWhitespace(al.FilePath + al.Content)))
		sb.WriteString(hex.EncodeToString(hash[:]))
		sb.WriteByte('\n')
	}

	if err := os.WriteFile(fpPath, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("写入fp文件失败: %w", err)
	}

	fmt.Printf("  fp文件已生成(来源:%s, %d行指纹): %s\n", source, len(addedLines), fpPath)
	return nil
}
