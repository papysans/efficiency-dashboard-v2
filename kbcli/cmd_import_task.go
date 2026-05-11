package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"kanban/core/models"
	"kanban/core/utils"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/spf13/cobra"
)

type taskSummary struct {
	TaskID          string `json:"task_id"`
	UserID          string `json:"user_id"`
	UserName        string `json:"user_name"`
	ClientID        string `json:"client_id"`
	ClientIDE       string `json:"client_ide"`
	ClientVersion   string `json:"client_version"`
	ClientOS        string `json:"client_os"`
	ClientOSVersion string `json:"client_os_version"`
	Caller          string `json:"caller"`
	RepoAddr        string `json:"repo_addr"`
	RepoBranch      string `json:"repo_branch"`
	WorkDir         string `json:"work_dir"`
	Diff            string `json:"diff"`
	DiffLines       int    `json:"diff_lines"`
}

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

	addedLines []addedLine
	startTime  time.Time
	endTime    time.Time
}

type taskSilicaData struct {
	TaskID          string                   `json:"task_id"`
	RepoAddr        string                   `json:"repo_addr"`
	UserID          string                   `json:"user_id"`
	Size            int64                    `json:"size"`
	ConversationNum int                      `json:"conversation_num"`
	Conversations   []taskSilicaConversation `json:"conversations"`
}

type taskSilicaConversation struct {
	RequestID    string   `json:"request_id"`
	EndTime      string   `json:"end_time"`
	Fingerprints []string `json:"fingerprints"`
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

func calcTaskRecord(summary *taskSummary, conversations []taskConversation) models.Task {
	rec := models.Task{
		TaskID:          summary.TaskID,
		UserID:          summary.UserID,
		UserName:        summary.UserName,
		ClientID:        summary.ClientID,
		ClientIDE:       summary.ClientIDE,
		ClientVersion:   summary.ClientVersion,
		ClientOS:        summary.ClientOS,
		ClientOSVersion: summary.ClientOSVersion,
		Caller:          summary.Caller,
		RepoAddr:        summary.RepoAddr,
		RepoBranch:      summary.RepoBranch,
		WorkDir:         summary.WorkDir,
		WorkDirID:       utils.GenerateWorkDirID(summary.ClientID, summary.WorkDir),
	}

	var startTime, endTime *time.Time
	var totalUpstream, totalDownstream int64
	var totalCost float64
	var totalLines int64

	for _, conv := range conversations {
		if conv.StartTime == "" {
			logWarnf("conversation [%s-%s] 缺少start_time字段", summary.TaskID, conv.RequestID)
			continue
		}
		if conv.EndTime == "" {
			logWarnf("conversation [%s-%s] 缺少end_time字段", summary.TaskID, conv.RequestID)
			continue
		}
		t1, err := time.Parse(time.RFC3339, conv.StartTime)
		if err != nil {
			logWarnf("conversation [%s-%s] start_time字段解析错误: %v", summary.TaskID, conv.RequestID, err)
			continue
		}
		t2, err := time.Parse(time.RFC3339, conv.EndTime)
		if err != nil {
			logWarnf("conversation [%s-%s] end_time字段解析错误: %v", summary.TaskID, conv.RequestID, err)
			continue
		}
		if startTime == nil || t1.Before(*startTime) {
			startTime = &t1
		}
		if endTime == nil || t2.After(*endTime) {
			endTime = &t2
		}
		totalUpstream += conv.UpstreamTokens
		totalDownstream += conv.DownstreamTokens
		totalCost += conv.Cost
		totalLines += conv.DiffLines
	}

	rec.StartTime = startTime
	rec.EndTime = endTime
	rec.UpstreamTokens = totalUpstream
	rec.DownstreamTokens = totalDownstream
	rec.Cost = totalCost
	rec.DiffLines = int(totalLines)

	minutes, reason := calcTaskRealMinutes(conversations, 30, 5)
	rec.TaskRealMinutes = minutes
	rec.TaskRealMinutesReason = reason

	minutes, reason = estimateTaskAncientMinutes(cfg.AlgoEstimation, conversations, rec.TaskRealMinutes)
	rec.TaskAncientMinutes = minutes
	rec.TaskAncientMinutesReason = reason
	return rec
}

func importSingleTask(db *gorm.DB, summaryPath, conversationPath, silicaPath string) error {
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
	if summary.UserID == "" {
		return fmt.Errorf("user_id为空")
	}

	conversations, err := parseConversationFile(conversationPath)
	if err != nil {
		return fmt.Errorf("解析conversation文件失败: %w", err)
	}

	task := calcTaskRecord(&summary, conversations)

	if err := generateTaskSilicaFile(&summary, conversations, conversationPath, silicaPath); err != nil {
		logWarnf("生成task silica文件失败 [%s]: %v", summary.TaskID, err)
	}

	result := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "task_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"user_id", "user_name",
			"client_id", "client_ide", "client_version",
			"client_os", "client_os_version", "caller",
			"repo_addr", "repo_branch", "work_dir", "work_dir_id",
			"diff_lines",
			"start_time", "end_time",
			"upstream_tokens", "downstream_tokens", "cost",
			"task_real_minutes", "task_real_minutes_reason",
			"task_ancient_minutes", "task_ancient_minutes_reason",
			"updated_at",
		}),
	}).Create(&task)
	if result.Error != nil {
		return fmt.Errorf("写入tasks表失败: %w", result.Error)
	}

	if task.TaskRealMinutes > 0 {
		logDebugf("  task_real_minutes=%.1f (%s)", task.TaskRealMinutes, task.TaskRealMinutesReason)
	}
	if task.TaskAncientMinutes > 0 {
		logDebugf("  task_ancient_minutes=%.1f (%s)", task.TaskAncientMinutes, task.TaskAncientMinutesReason)
	}

	if len(conversations) > 0 {
		if err := saveConversations(db, task.TaskID, conversations); err != nil {
			return fmt.Errorf("保存conversations失败: %w", err)
		}
	}

	logDebugf("导入成功: %s", task.TaskID)
	return nil
}

func saveConversations(db *gorm.DB, taskID string, conversations []taskConversation) error {
	return db.Transaction(func(tx *gorm.DB) error {
		for _, conv := range conversations {
			tc := models.TaskConversation{
				TaskID:           taskID,
				RequestID:        conv.RequestID,
				Sender:           conv.Sender,
				PromptMode:       conv.PromptMode,
				Mode:             conv.Mode,
				Model:            conv.Model,
				StartTime:        conv.startTime,
				EndTime:          conv.endTime,
				ProcessTime:      conv.ProcessTime,
				ProcessTTFT:      conv.ProcessTTFT,
				UpstreamTokens:   conv.UpstreamTokens,
				DownstreamTokens: conv.DownstreamTokens,
				Cost:             conv.Cost,
				RequestContent:   utils.SanitizeText(conv.RequestContent),
				ResponseContent:  utils.SanitizeText(conv.ResponseContent),
				UserInput:        utils.SanitizeText(conv.UserInput),
				DiffLines:        conv.DiffLines,
				ErrorCode:        string(conv.ErrorCode),
				ErrorReason:      utils.SanitizeText(string(conv.ErrorReason)),
			}

			result := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "task_id"}, {Name: "request_id"}},
				DoNothing: true,
			}).Create(&tc)
			if result.Error != nil {
				return fmt.Errorf("写入task_conversations表失败: %w", result.Error)
			}
		}
		return nil
	})
}

func estimateTaskAncientMinutes(cfg EstimateConfig, convs []taskConversation, realMinutes float64) (float64, string) {
	var totalInchars int64
	var totalDiffLines int64

	for _, conv := range convs {
		totalInchars += int64(len(conv.UserInput))
		totalDiffLines += conv.DiffLines
	}

	inchars := float64(totalInchars)
	diffLines := float64(totalDiffLines)

	if inchars >= cfg.MaxInputChars {
		inchars = cfg.MaxInputChars
	}

	factor := cfg.MinFactor + (inchars/cfg.MaxInputChars)*(cfg.MaxFactor-cfg.MinFactor)
	workload := (diffLines / cfg.LinesPerMinutes) * factor

	maxWorkload := cfg.MaxRatio * realMinutes
	minWorkload := max(cfg.MinMinutes, realMinutes)

	if workload > maxWorkload {
		workload = maxWorkload
	}
	if workload < minWorkload {
		workload = minWorkload
	}

	return workload, fmt.Sprintf(
		"基于diff_lines=%.0f, user_input=%.0f字符, factor=%.2f, real_minutes=%.2f估算",
		diffLines, float64(totalInchars), factor, realMinutes,
	)
}

func calcTaskRealMinutes(conversations []taskConversation, gapThreshold, extensionMin int) (float64, string) {
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

func calculateCost(model string, inTokens, outTokens int64, prices map[string]ModelPrice) float64 {
	model = strings.ToLower(model)
	var price ModelPrice
	// 前缀匹配：找 prices 中为 model 前缀的 key，取最长匹配
	var bestKey string
	for k := range prices {
		if k != "default" && strings.HasPrefix(model, k) {
			if len(k) > len(bestKey) {
				bestKey = k
			}
		}
	}
	if bestKey != "" {
		price = prices[bestKey]
	} else {
		var ok bool
		price, ok = prices["default"]
		if !ok {
			return 0
		}
	}

	return (float64(inTokens)/1e6)*price.InPrice + (float64(outTokens)/1e6)*price.OutPrice
}

func parseUserInput(userInput string) string {
	const prefix = "<user_message>"
	const suffix = "</user_message>"

	if !strings.HasPrefix(userInput, prefix) {
		return userInput
	}

	startIdx := len(prefix)
	endIdx := strings.Index(userInput[startIdx:], suffix)
	if endIdx == -1 {
		return userInput
	}

	return userInput[startIdx : startIdx+endIdx]
}

func calcConversation(conv *taskConversation) error {
	if conv.RequestID == "" {
		return fmt.Errorf("对话缺失request_id字段")
	}
	if conv.StartTime == "" {
		return fmt.Errorf("对话[%s]缺失start_time字段", conv.RequestID)
	}
	if t, err := time.Parse(time.RFC3339, conv.StartTime); err != nil {
		return err
	} else {
		conv.startTime = t
	}

	if conv.EndTime == "" {
		return fmt.Errorf("对话[%s]缺失end_time字段", conv.RequestID)
	}
	if t, err := time.Parse(time.RFC3339, conv.EndTime); err != nil {
		return err
	} else {
		conv.endTime = t
	}

	if conv.Cost == 0 && conv.UpstreamTokens > 0 && conv.Model != "" {
		conv.Cost = calculateCost(conv.Model, conv.UpstreamTokens, conv.DownstreamTokens, cfg.ModelPrices)
	}
	conv.UserInput = parseUserInput(conv.UserInput)
	if strings.TrimSpace(conv.Diff) != "" {
		conv.addedLines = extractAddedLinesFromDiff(conv.Diff)
	}
	conv.DiffLines = int64(len(conv.addedLines))
	conv.Diff = ""
	return nil
}

func skeletonize(content string, head, maxSize int) string {
	if len(content) <= maxSize {
		return content
	}
	if head > len(content) {
		head = len(content)
	}
	tail := maxSize - head - 3
	if tail < 0 {
		tail = 0
	}
	return content[:head] + "..." + content[len(content)-tail:]
}

func parseConversation(path string, lineNum int, content []byte, ignoreUnmarshalWarning bool) (*taskConversation, error) {
	var conv taskConversation
	if err := json.Unmarshal(content, &conv); err != nil {
		if !ignoreUnmarshalWarning {
			logWarnf("解析[%s:%d]失败: %v, 内容: %s", path, lineNum, err, skeletonize(string(content), 40, 64))
		}
		return nil, err
	}
	if err := calcConversation(&conv); err != nil {
		logWarnf("解析[%s:%d]中的对话发生错误: %v", path, lineNum, err)
		return nil, nil
	}
	return &conv, nil
}

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
		if c, err := parseConversation(path, lineNum, []byte(line), true); err == nil {
			if c != nil {
				convs = append(convs, *c)
			}
			continue
		}
		// 容错：尝试将单行按独立的JSON对象拆分解析
		// 处理上游写入时缺少换行符的情况: {"a":1}{"a":2}
		parts, splitErr := splitConversations(line)
		if splitErr == nil && len(parts) > 0 {
			for _, part := range parts {
				if c, err := parseConversation(path, lineNum, []byte(part), false); err == nil && c != nil {
					convs = append(convs, *c)
				}
			}
			continue
		}

		return nil, fmt.Errorf("第%d行JSON解析失败: %w, 内容: %s", lineNum, err, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}
	return convs, nil
}

// splitConversations 尝试将单行字符串按顶层JSON对象拆分，返回JSON字符串列表
func splitConversations(line string) ([]string, error) {
	var parts []string
	start := 0
	for start < len(line) {
		if line[start] != '{' {
			break
		}
		// 找到匹配的 }，考虑嵌套
		depth := 0
		end := start
		inString := false
		escaped := false
		for ; end < len(line); end++ {
			ch := line[end]
			if inString {
				if escaped {
					escaped = false
					continue
				}
				if ch == '\\' {
					escaped = true
					continue
				}
				if ch == '"' {
					inString = false
				}
				continue
			}
			if ch == '"' {
				inString = true
				continue
			}
			if ch == '{' {
				depth++
				continue
			}
			if ch == '}' {
				depth--
				if depth == 0 {
					break
				}
			}
		}
		if depth != 0 || end >= len(line) {
			break
		}

		objStr := strings.TrimSpace(line[start : end+1])
		if objStr == "" {
			break
		}
		parts = append(parts, objStr)

		// 跳过空白和可能的分隔符
		next := end + 1
		for next < len(line) && (line[next] == ' ' || line[next] == '\t' || line[next] == '\n' || line[next] == '\r') {
			next++
		}
		start = next
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("未能拆分出有效JSON对象")
	}
	return parts, nil
}

func generateTaskSilicaFile(summary *taskSummary, conversations []taskConversation, conversationPath, silicaPath string) error {
	var fileSize int64
	if info, err := os.Stat(conversationPath); err == nil {
		fileSize = info.Size()
	}

	tsd := taskSilicaData{
		TaskID:          summary.TaskID,
		RepoAddr:        summary.RepoAddr,
		UserID:          summary.UserID,
		Size:            fileSize,
		ConversationNum: len(conversations),
	}

	for _, conv := range conversations {
		if len(conv.addedLines) == 0 {
			continue
		}

		var fingerprints []string
		for _, al := range conv.addedLines {
			fingerprints = append(fingerprints, calcLineFingerprint(al))
		}

		tsc := taskSilicaConversation{
			RequestID:    conv.RequestID,
			EndTime:      conv.EndTime,
			Fingerprints: fingerprints,
		}
		tsd.Conversations = append(tsd.Conversations, tsc)
	}

	dir := filepath.Dir(silicaPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	data, err := json.Marshal(tsd)
	if err != nil {
		return fmt.Errorf("序列化taskSilicaData失败: %w", err)
	}

	if err := os.WriteFile(silicaPath, data, 0644); err != nil {
		return fmt.Errorf("写入task silica文件失败: %w", err)
	}

	logDebugf("  task silica文件已生成(%d个conversation): %s", len(tsd.Conversations), silicaPath)
	return nil
}

func scanConversationFiles(conversationDir string) (map[string]string, error) {
	convMap := make(map[string]string)
	if _, err := os.Stat(conversationDir); os.IsNotExist(err) {
		return convMap, nil
	}
	err := filepath.Walk(conversationDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".jsonl") {
			taskID := strings.TrimSuffix(info.Name(), ".jsonl")
			if existing, ok := convMap[taskID]; !ok || path > existing {
				convMap[taskID] = path
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("扫描conversation目录失败: %w", err)
	}
	return convMap, nil
}

func scanSummaryFiles(summaryDir string) (map[string]string, error) {
	summaryMap := make(map[string]string)
	if _, err := os.Stat(summaryDir); os.IsNotExist(err) {
		return summaryMap, nil
	}
	err := filepath.Walk(summaryDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".json") {
			taskID := strings.TrimSuffix(info.Name(), ".json")
			if existing, ok := summaryMap[taskID]; !ok || path > existing {
				summaryMap[taskID] = path
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("扫描summary目录失败: %w", err)
	}
	return summaryMap, nil
}

func needUpdateConversations(conversationPath, silicaPath string, force bool) bool {
	if force {
		return true
	}
	data, err := os.ReadFile(silicaPath)
	if err != nil {
		return true
	}
	var tsd taskSilicaData
	if err := json.Unmarshal(data, &tsd); err != nil {
		return true
	}
	info, err := os.Stat(conversationPath)
	if err != nil {
		return true
	}
	return info.Size() != tsd.Size
}

func runImportTask(taskDir, analysedDir string, force bool) error {
	startTime := time.Now()
	summaryDir := filepath.Join(taskDir, "summary")
	conversationDir := filepath.Join(taskDir, "conversation")

	if _, err := os.Stat(summaryDir); os.IsNotExist(err) {
		recordCommandRun("import-task", startTime, 0, 0, 0, err)
		return fmt.Errorf("summary目录不存在: %s", summaryDir)
	}

	db, err := models.OpenGormDB(cfg.StatDatabase.DSN())
	if err != nil {
		recordCommandRun("import-task", startTime, 0, 0, 0, err)
		return fmt.Errorf("连接数据库失败: %w", err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	convMap, err := scanConversationFiles(conversationDir)
	if err != nil {
		recordCommandRun("import-task", startTime, 0, 0, 0, err)
		return err
	}

	summaryMap, err := scanSummaryFiles(summaryDir)
	if err != nil {
		recordCommandRun("import-task", startTime, 0, 0, 0, err)
		return err
	}

	if len(convMap) == 0 {
		logInfo("没有找到待导入的 conversation 文件")
		recordCommandRun("import-task", startTime, 0, 0, 0, nil)
		return nil
	}

	successCount := 0
	failCount := 0
	skipCount := 0

	for taskID, conversationPath := range convMap {
		summaryPath, ok := summaryMap[taskID]
		if !ok {
			logDebugf("跳过(无对应summary): %s", taskID)
			skipCount++
			continue
		}

		silicaPath := filepath.Join(analysedDir, "task", "conversation", taskID+".silica.json")
		if !needUpdateConversations(conversationPath, silicaPath, force) {
			logDebugf("跳过(conversation未更新): %s", taskID)
			skipCount++
			continue
		}

		if err := importSingleTask(db, summaryPath, conversationPath, silicaPath); err != nil {
			logWarnf("导入失败 [%s]: %v", taskID, err)
			failCount++
		} else {
			successCount++
			logPromptProgress(successCount, 50)
		}
	}

	logInfof("导入完成: 成功 %d 个，失败 %d 个，跳过 %d 个", successCount, failCount, skipCount)
	recordCommandRun("import-task", startTime, successCount, failCount, skipCount, nil)
	return nil
}

var importTasksCmd = &cobra.Command{
	Use:   "import-task",
	Short: "导入 task 数据到 costrict_stat 数据库",
	RunE: func(cmd *cobra.Command, args []string) error {
		taskDir, _ := cmd.Flags().GetString("task-dir")
		analysedDir, _ := cmd.Flags().GetString("analysed-dir")
		force, _ := cmd.Flags().GetBool("force")
		remote, _ := cmd.Flags().GetString("remote")

		if remote != "" {
			return sendToRemote(remote, "import-task", map[string]interface{}{
				"task_dir":     taskDir,
				"analysed_dir": analysedDir,
				"force":        force,
			})
		}
		if taskDir == "" {
			taskDir = cfg.TaskDir
		}
		if analysedDir == "" {
			analysedDir = cfg.AnalysedDir
		}

		return runImportTask(taskDir, analysedDir, force)
	},
}

func init() {
	importTasksCmd.Flags().SortFlags = false
	importTasksCmd.Flags().String("task-dir", "", "task 目录路径")
	importTasksCmd.Flags().String("analysed-dir", "", "输出目录路径")
	importTasksCmd.Flags().BoolP("force", "f", false, "强制重新导入，覆盖已存在数据")
	importTasksCmd.Flags().String("remote", "", "远程kbcli服务地址（如 http://127.0.0.1:8080），指定后命令将发送到远程执行")
	rootCmd.AddCommand(importTasksCmd)
}
