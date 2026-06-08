package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"kanban/kbcli/internal/logx"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"kanban/core/models"
	"kanban/core/utils"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/spf13/cobra"
)

// taskSession 表示一个session，是对话记录的上下文。
type taskSession struct {
	SessionId       string `json:"task_id"`
	UserId          string `json:"user_id"`
	UserName        string `json:"user_name"`
	ClientId        string `json:"client_id"`
	ClientIde       string `json:"client_ide"`
	ClientVersion   string `json:"client_version"`
	ClientOs        string `json:"client_os"`
	ClientOsVersion string `json:"client_os_version"`
	Caller          string `json:"caller"`
	RepoAddr        string `json:"repo_addr"`
	RepoBranch      string `json:"repo_branch"`
	WorkDir         string `json:"work_dir"`
	StartTime       string `json:"start_time"`
}

// taskConversation 表示一次任务中的单次对话记录，对应 conversation 目录下 JSONL 的每一行。
type taskConversation struct {
	Sender           string     `json:"sender"`
	RequestId        string     `json:"request_id"`
	Caller           string     `json:"caller"`
	RepoAddr         string     `json:"repo_addr"`
	RepoBranch       string     `json:"repo_branch"`
	WorkDir          string     `json:"work_dir"`
	PromptMode       string     `json:"prompt_mode"`
	Mode             string     `json:"mode"`
	Model            string     `json:"model"`
	StartTime        string     `json:"start_time"`
	EndTime          string     `json:"end_time"`
	ProcessTime      flexInt64  `json:"process_time"`
	ProcessTtft      flexInt64  `json:"process_ttft"`
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

	// addedLines 为解析 Diff 后提取的新增代码行，用于后续生成 silica 指纹。
	sessionId  string
	clientId   string
	workDirId  string
	addedLines []addedLine
	startTime  time.Time
	endTime    time.Time
}

// flexString 是一个灵活的字符串类型，用于兼容 JSON 中字段可能为字符串或数字的场景。
type flexString string

// UnmarshalJSON 实现 json.Unmarshaler 接口。
// 功能：支持将 JSON 中的 null、字符串、数字统一反序列化为字符串。
// 参数：
//   - data: JSON 字节的原始内容。
//
// 返回值：反序列化过程中发生的错误。
// 关键技术原理：优先尝试按 string 解析；失败时尝试 json.Number（即数字类型），并将其转为字符串；均失败则报错。
func (f *flexString) UnmarshalJSON(data []byte) error {
	// 处理 JSON null 值，直接置为空字符串
	if string(data) == "null" {
		*f = ""
		return nil
	}
	// 先尝试按普通字符串解析
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*f = flexString(s)
		return nil
	}
	// 字符串解析失败时，尝试按数字类型解析（防止上游将数字以字符串形式输出）
	var n json.Number
	if err := json.Unmarshal(data, &n); err == nil {
		*f = flexString(n.String())
		return nil
	}
	return fmt.Errorf("flexString: cannot unmarshal %s", string(data))
}

// flexInt64 是一个灵活的整数类型，用于兼容 JSON number 为整数或浮点的场景。
// 背景：上游某次起把 process_time / process_ttft 写成小数（如 18.848），
// 而 Go encoding/json 无法把浮点直接解进 int64，会导致整行 conversation 解析失败被跳过，
// 进而丢失该对话的全部字段。改用本类型后，整数与浮点都能解析，按四舍五入存为 int64。
type flexInt64 int64

// UnmarshalJSON 实现 json.Unmarshaler 接口。
// 功能：把 JSON number（整数或浮点）按 math.Round 四舍五入转为 int64；
//
//	空值 / null 容错为 0；负值原样承载（cmd_check 仍可检出负值）。
//
// 参数：
//   - data: JSON 字节的原始内容。
//
// 返回值：反序列化过程中发生的错误。
func (f *flexInt64) UnmarshalJSON(data []byte) error {
	s := strings.TrimSpace(string(data))
	// 空值或 null 容错为 0
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	// 统一按浮点解析，再四舍五入；这样整数与小数都能正确承载
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return err
	}
	*f = flexInt64(math.Round(v))
	return nil
}

// correctConversations 根据任务摘要和对话列表计算完整的 Task 模型记录。
// 功能：汇总对话的时间范围、Token 消耗、成本、代码行数，并调用估算算法得到实际工作时长和预估原始时长。
// 参数：
//   - summary: 任务摘要信息，包含任务基本元数据。
//   - conversations: 该任务下的所有对话列表。
//
// 返回值：组装好的 models.Task 结构体，可直接写入数据库。
// 关键技术原理：
//   - 遍历所有对话，取最早 StartTime 和最晚 EndTime 作为任务整体时间窗口。
//   - 累加 UpstreamTokens、DownstreamTokens、Cost、DiffLines 得到任务级指标。
//   - calcTaskRealMinutes 基于对话时间分布，用“时间片段合并”算法计算真实工作时长。
//   - estimateTaskAncientMinutes 基于输入字符数和代码行数，用线性因子模型估算原始工作量。
var errSkipTask = errors.New("task skipped by date filter")

func extractDateFromPath(baseDir, filePath string) string {
	relPath, err := filepath.Rel(baseDir, filePath)
	if err != nil {
		return ""
	}
	dir := filepath.ToSlash(filepath.Dir(relPath))
	if _, err := time.Parse("2006/01/02", dir); err == nil {
		return dir
	}
	return ""
}

func saveSession(db *gorm.DB, ss *taskSession, sessionDate, conversationDate string) error {
	var startTime time.Time = time.Now().UTC()
	var err error
	if ss.StartTime != "" {
		if startTime, err = time.Parse(time.RFC3339, ss.StartTime); err != nil {
			logx.Warnf("session [%s] 缺少start_time字段", ss.SessionId)
		}
	}
	// 初始化 Task 基础字段，WorkDirId 通过工具函数根据 ClientId 和 WorkDir 生成唯一标识
	rec := models.Session{
		SessionId:        ss.SessionId,
		CreateTime:       startTime,
		UserId:           ss.UserId,
		UserName:         ss.UserName,
		ClientId:         ss.ClientId,
		ClientIde:        ss.ClientIde,
		ClientVersion:    ss.ClientVersion,
		ClientOs:         ss.ClientOs,
		ClientOsVersion:  ss.ClientOsVersion,
		SessionDate:      sessionDate,
		ConversationDate: conversationDate,
	}

	// 使用 UPSERT 写入 tasks 表：task_id 冲突时更新除主键外的业务字段
	result := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "session_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"user_id", "user_name", "create_time",
			"client_id", "client_ide", "client_version",
			"client_os", "client_os_version",
			"session_date", "conversation_date",
			"updated_at",
		}),
	}).Create(&rec)
	if result.Error != nil {
		logx.Errorf("session [%s] 保存失败: %v", ss.SessionId, result.Error)
		return fmt.Errorf("写入session表失败: %w", result.Error)
	}
	return nil
}

func correctConversations(ss *taskSession, conversations []taskConversation) {
	// 遍历所有对话，解析时间并累加指标；时间解析失败则跳过该对话并记录警告
	for i, conv := range conversations {
		if conv.StartTime == "" {
			logx.Warnf("conversation [%s-%s] 缺少start_time字段", ss.SessionId, conv.RequestId)
			continue
		}
		if conv.EndTime == "" {
			logx.Warnf("conversation [%s-%s] 缺少end_time字段", ss.SessionId, conv.RequestId)
			continue
		}
		_, err := time.Parse(time.RFC3339, conv.StartTime)
		if err != nil {
			logx.Warnf("conversation [%s-%s] start_time字段解析错误: %v", ss.SessionId, conv.RequestId, err)
			continue
		}
		_, err = time.Parse(time.RFC3339, conv.EndTime)
		if err != nil {
			logx.Warnf("conversation [%s-%s] end_time字段解析错误: %v", ss.SessionId, conv.RequestId, err)
			continue
		}
		if conv.Caller == "" {
			conversations[i].Caller = ss.Caller
		}
		if conv.RepoAddr == "" {
			conversations[i].RepoAddr = ss.RepoAddr
		}
		if conv.RepoBranch == "" {
			conversations[i].RepoBranch = ss.RepoBranch
		}
		if conv.WorkDir == "" {
			conversations[i].WorkDir = ss.WorkDir
		}
		conversations[i].sessionId = ss.SessionId
		conversations[i].clientId = ss.ClientId
		conversations[i].workDirId = utils.GenerateWorkDirID(ss.ClientId, conversations[i].WorkDir)
		// 计算并校验字段；校验失败不返回错误，仅记录警告并返回 nil，避免单条坏数据阻断整批导入

		// 若成本缺失且存在 Token 和模型信息，自动计算成本
		if conv.Cost == 0 && conv.UpstreamTokens > 0 && conv.Model != "" {
			conversations[i].Cost = calculateCost(conv.Model, conv.UpstreamTokens, conv.DownstreamTokens, cfg.ModelPrices)
		}
		// 去除用户输入的包装标签
		conversations[i].UserInput = parseUserInput(conv.UserInput)
		// 解析 Diff 文本，提取新增代码行
		if strings.TrimSpace(conv.Diff) != "" {
			conversations[i].addedLines = extractAddedLinesFromDiff(conv.Diff)
		}
		// 统计新增代码行数，并清空 Diff 原文以释放内存
		// 注意：必须读 conversations[i]（切片元素），conv 是 range 的值拷贝，addedLines 不会同步。
		conversations[i].DiffLines = int64(len(conversations[i].addedLines))
		conversations[i].Diff = ""
	}
}

// preparedImportTask 是一个 session 解析完成、待写库的中间结果。
// 解析（读文件 / JSON / 对话）在事务外完成，写库阶段只做 upsert，从而支持批量提交。
type preparedImportTask struct {
	ss               *taskSession
	conversations    []taskConversation
	sessionDate      string
	conversationDate string
	conversationPath string
	silicaPath       string
}

// prepareImportTask 读取并解析单个任务的 summary 与 conversation 文件，算好待写入的记录。
// 仅做文件 IO 与解析、不触库；这样所有易失败的解析步骤都在事务外，不会污染批量事务。
func prepareImportTask(summaryPath, conversationPath, silicaPath string) (*preparedImportTask, error) {
	data, err := os.ReadFile(summaryPath)
	if err != nil {
		return nil, fmt.Errorf("读取summary文件失败: %w", err)
	}

	var ss taskSession
	if err := json.Unmarshal(data, &ss); err != nil {
		return nil, fmt.Errorf("解析summary JSON失败: %w", err)
	}

	// 校验关键字段，防止写入无效数据
	if ss.SessionId == "" {
		return nil, fmt.Errorf("session_id为空")
	}
	if ss.UserId == "" {
		return nil, fmt.Errorf("user_id为空")
	}

	// 解析对话文件，得到该任务下的所有对话列表
	conversations, err := parseConversationFile(conversationPath)
	if err != nil {
		return nil, fmt.Errorf("解析conversation文件失败: %w", err)
	}

	// 根据 ss 和 conversations 计算完整的 Task 记录
	summaryDir := filepath.Dir(filepath.Dir(summaryPath))
	summaryDir = filepath.Dir(filepath.Dir(summaryDir))
	sessionDate := extractDateFromPath(summaryDir, summaryPath)
	conversationDir := filepath.Dir(filepath.Dir(conversationPath))
	conversationDir = filepath.Dir(filepath.Dir(conversationDir))
	conversationDate := extractDateFromPath(conversationDir, conversationPath)

	// 解析对话内的时间/指标；saveConversations 依赖此处填充的 startTime/endTime
	correctConversations(&ss, conversations)

	return &preparedImportTask{
		ss:               &ss,
		conversations:    conversations,
		sessionDate:      sessionDate,
		conversationDate: conversationDate,
		conversationPath: conversationPath,
		silicaPath:       silicaPath,
	}, nil
}

// writeImportTask 在给定（事务）连接上写入一个已解析任务：session + 其全部 conversation。
// 用 clause.OnConflict 保证幂等。供批量事务与逐条回退共用。
func writeImportTask(tx *gorm.DB, p *preparedImportTask) error {
	if err := saveSession(tx, p.ss, p.sessionDate, p.conversationDate); err != nil {
		return err
	}
	if err := saveConversations(tx, p.conversations); err != nil {
		return fmt.Errorf("保存conversations失败: %w", err)
	}
	return nil
}

// finalizeImportTaskSilica 写库成功后生成 task silica 文件（用于下次增量检测）；失败仅告警，不阻断。
func finalizeImportTaskSilica(p *preparedImportTask) {
	if err := generateTaskSilicaFile(p.ss, p.conversations, p.conversationPath, p.silicaPath); err != nil {
		logx.Warnf("生成task silica文件失败 [%s]: %v", p.ss.SessionId, err)
	}
}

// flushImportConvBatch 把一批已解析任务在单个事务内写入：将「每 session 一次提交(一次 fsync)」
// 摊薄成「每批一次」，大幅提升大批量导入吞吐。整批事务失败时回退为逐条单事务，
// 隔离坏记录、保留其余成功项。返回本批成功/失败计数。
func flushImportConvBatch(db *gorm.DB, batch []*preparedImportTask) (success, fail int) {
	if len(batch) == 0 {
		return 0, 0
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		for _, p := range batch {
			if err := writeImportTask(tx, p); err != nil {
				return err
			}
		}
		return nil
	})
	if err == nil {
		for _, p := range batch {
			finalizeImportTaskSilica(p)
		}
		return len(batch), 0
	}

	// 整批失败（多为 DB/schema 级错误）：逐条单事务重试，定位并隔离坏记录
	logx.Warnf("批量写入失败，回退逐条重试: %v", err)
	for _, p := range batch {
		if e := db.Transaction(func(tx *gorm.DB) error { return writeImportTask(tx, p) }); e != nil {
			logx.Warnf("导入失败 [%s]: %v", p.ss.SessionId, e)
			fail++
			continue
		}
		finalizeImportTaskSilica(p)
		success++
	}
	return success, fail
}

// saveConversations 将任务对话列表批量保存到数据库，使用事务保证原子性。
// 功能：逐条将 taskConversation 转换为 models.Conversation 后插入，冲突时忽略（DoNothing）。
// 参数：
//   - db: GORM 数据库连接。
//   - conversations: 需要保存的对话列表。
//
// 返回值：事务执行过程中发生的错误。
// 关键技术原理：通过 db.Transaction 开启事务，确保一批对话要么全部写入成功，要么全部回滚；
// 复合唯一键 (task_id, request_id) 冲突时忽略插入，避免重复数据报错。
func saveConversations(db *gorm.DB, conversations []taskConversation) error {
	if len(conversations) == 0 {
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		for _, conv := range conversations {
			// 转换字段并对文本内容进行清洗，防止非法字符入库
			tc := models.Conversation{
				TaskId:           "",
				SessionId:        conv.sessionId,
				RequestId:        conv.RequestId,
				Sender:           conv.Sender,
				PromptMode:       conv.PromptMode,
				Mode:             conv.Mode,
				Model:            conv.Model,
				StartTime:        conv.startTime,
				EndTime:          conv.endTime,
				ProcessTime:      int64(conv.ProcessTime),
				ProcessTtft:      int64(conv.ProcessTtft),
				UpstreamTokens:   conv.UpstreamTokens,
				DownstreamTokens: conv.DownstreamTokens,
				Cost:             conv.Cost,
				RequestContent:   utils.SanitizeText(conv.RequestContent),
				ResponseContent:  utils.SanitizeText(conv.ResponseContent),
				UserInput:        utils.SanitizeText(conv.UserInput),
				DiffLines:        conv.DiffLines,
				ErrorCode:        string(conv.ErrorCode),
				ErrorReason:      utils.SanitizeText(string(conv.ErrorReason)),
				RepoAddr:         conv.RepoAddr,
				RepoBranch:       conv.RepoBranch,
				WorkDir:          conv.WorkDir,
				WorkDirId:        conv.workDirId,
			}

			// 复合主键冲突时忽略，避免同一对话重复导入导致事务失败
			result := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "session_id"}, {Name: "request_id"}},
				DoNothing: true,
			}).Create(&tc)
			if result.Error != nil {
				return fmt.Errorf("写入task_conversations表失败: %w", result.Error)
			}
		}
		return nil
	})
}

// parseUserInput 解析并提取用户输入中的实际内容。
// 功能：去除 <user_message> 和 </user_message> 包装标签，返回内部文本。
// 参数：
//   - userInput: 原始用户输入字符串，可能包含 XML 风格的包装标签。
//
// 返回值：去掉包装后的纯用户输入文本；若标签不匹配或格式异常，则返回原字符串。
func parseUserInput(userInput string) string {
	const prefix = "<user_message>"
	const suffix = "</user_message>"

	// 若不以指定前缀开头，直接返回原内容
	if !strings.HasPrefix(userInput, prefix) {
		return userInput
	}

	startIdx := len(prefix)
	// 在前缀之后查找后缀位置
	endIdx := strings.Index(userInput[startIdx:], suffix)
	if endIdx == -1 {
		return userInput
	}

	// 截取前缀和后缀之间的内容
	return userInput[startIdx : startIdx+endIdx]
}

// checkConversation 对单个对话进行字段合法性检查
func checkConversation(conv *taskConversation) error {
	// 基础字段校验
	if conv.RequestId == "" {
		return fmt.Errorf("对话缺失request_id字段")
	}
	if conv.StartTime == "" {
		return fmt.Errorf("对话[%s]缺失start_time字段", conv.RequestId)
	}
	if t, err := time.Parse(time.RFC3339, conv.StartTime); err != nil {
		return err
	} else {
		conv.startTime = t
	}

	if conv.EndTime == "" {
		return fmt.Errorf("对话[%s]缺失end_time字段", conv.RequestId)
	}
	if t, err := time.Parse(time.RFC3339, conv.EndTime); err != nil {
		return err
	} else {
		conv.endTime = t
	}

	return nil
}

// skeletonize 对长字符串进行头部保留 + 尾部保留的截断处理，中间用 "..." 代替。
// 功能：当 content 长度超过 maxSize 时，保留头部 head 个字符和尾部剩余字符，中间替换为省略号。
// 参数：
//   - content: 原始字符串。
//   - head: 头部保留的字符数。
//   - maxSize: 截断后的最大总长度（包含省略号）。
//
// 返回值：截断后的字符串；若 content 长度不超过 maxSize，则原样返回。
func skeletonize(content string, head, maxSize int) string {
	// 长度未超限，无需截断
	if len(content) <= maxSize {
		return content
	}
	// 防止 head 超过内容长度导致越界
	if head > len(content) {
		head = len(content)
	}
	// 计算尾部可保留长度，需为 "..." 预留 3 个字符
	tail := maxSize - head - 3
	if tail < 0 {
		tail = 0
	}
	return content[:head] + "..." + content[len(content)-tail:]
}

// parseConversation 解析单行 JSON 数据为 taskConversation 结构体。
// 功能：尝试将一行 JSONL 内容反序列化为 taskConversation。
// 参数：
//   - path: 源文件路径，仅用于日志输出。
//   - lineNum: 当前行号，仅用于日志输出。
//   - content: 该行的原始字节内容。
//   - ignoreUnmarshalWarning: 若为 true，则在 JSON 反序列化失败时不输出警告日志。
//
// 返回值：
//   - *taskConversation: 解析成功且校验通过的对话指针；若校验失败返回 nil。
//   - error: JSON 解析失败时返回错误；校验失败时返回 nil（不阻断外层批量处理）。
func parseConversation(path string, lineNum int, content []byte, ignoreUnmarshalWarning bool) (*taskConversation, error) {
	var conv taskConversation
	// 反序列化 JSON
	if err := json.Unmarshal(content, &conv); err != nil {
		if !ignoreUnmarshalWarning {
			logx.Warnf("解析[%s:%d]失败: %v, 内容: %s", path, lineNum, err, skeletonize(string(content), 40, 64))
		}
		return nil, err
	}
	if err := checkConversation(&conv); err != nil {
		logx.Warnf("解析[%s:%d]发现错误: %v", path, lineNum, err)
		return nil, nil
	}
	return &conv, nil
}

// parseConversationFile 解析整个 conversation JSONL 文件。
// 功能：按行读取文件，逐行解析为 taskConversation；若整行解析失败，则尝试按多个独立 JSON 对象拆分后再次解析。
// 参数：
//   - path: conversation JSONL 文件路径。
//
// 返回值：解析成功的对话列表；若发生不可恢复错误则返回错误。
// 关键技术原理：
//  1. 使用 bufio.Scanner 逐行读取，单行单 JSON 对象为正常情况。
//  2. 容错机制：当单行整体验证失败时，尝试调用 splitConversations 将连续拼接的 JSON 对象（如 {"a":1}{"a":2}）拆分为多个对象分别解析。
//  3. scanner.Buffer 显式设置缓冲区初始大小和最大大小，防止超长行导致扫描器默认缓冲区溢出。
func parseConversationFile(path string) ([]taskConversation, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var convs []taskConversation
	// 用 bufio.Reader 逐行读取，对单行长度不设硬上限：
	// 避免个别超长 JSON 行（>10MB，如内嵌大段 base64/上下文）触发 bufio.Scanner 的
	// token too long，进而导致整个 conversation 文件被判失败、对应 session 整条丢失。
	reader := bufio.NewReader(f)
	lineNum := 0
	for {
		lineStr, readErr := reader.ReadString('\n')
		if len(lineStr) > 0 {
			lineNum++
		}
		line := strings.TrimSpace(lineStr)
		if line != "" {
			// 先尝试将整行作为一个 JSON 对象解析；ignoreUnmarshalWarning=true 减少冗余日志
			c, parseErr := parseConversation(path, lineNum, []byte(line), true)
			if parseErr == nil {
				if c != nil {
					convs = append(convs, *c)
				}
			} else if parts, splitErr := splitConversations(line); splitErr == nil && len(parts) > 0 {
				// 容错：处理上游缺少换行符把多个对象拼到一行的情况 {"a":1}{"a":2}
				for _, part := range parts {
					if pc, e := parseConversation(path, lineNum, []byte(part), false); e == nil && pc != nil {
						convs = append(convs, *pc)
					}
				}
			} else {
				return nil, fmt.Errorf("第%d行JSON解析失败: %w, 内容: %s", lineNum, parseErr, line)
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			return nil, fmt.Errorf("读取文件失败: %w", readErr)
		}
	}
	return convs, nil
}

// splitConversations 尝试将单行字符串按顶层 JSON 对象拆分，返回JSON字符串列表。
// 功能：处理多个 JSON 对象连续拼接在同一行的情况（缺少换行符），逐层识别嵌套大括号后切分。
// 参数：
//   - line: 可能包含多个 JSON 对象的单行字符串。
//
// 返回值：拆分出的独立 JSON 字符串列表；若未找到有效对象则返回错误。
// 关键技术原理：基于状态机的括号匹配算法，逐字符扫描并维护 depth（嵌套深度）和 inString（是否在字符串内），
// 遇到未转义的 '"' 时切换 inString 状态，在 inString 为 false 时统计 '{' 和 '}' 来判定顶层对象边界。
func splitConversations(line string) ([]string, error) {
	var parts []string
	start := 0
	for start < len(line) {
		// 非 '{' 开头则终止拆分
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
		// 若括号未匹配或超出字符串范围，说明不是合法 JSON 对象序列
		if depth != 0 || end >= len(line) {
			break
		}

		objStr := strings.TrimSpace(line[start : end+1])
		if objStr == "" {
			break
		}
		parts = append(parts, objStr)

		// 跳过空白和可能的分隔符，准备解析下一个对象
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

// generateTaskSilicaFile 为指定任务生成 silica 摘要文件。
// 功能：汇总任务的 conversation 信息，提取每条对话新增代码行的指纹，序列化为 JSON 后写入磁盘。
// 参数：
//   - ss: 任务摘要，用于填充 silica 中的基础信息。
//   - conversations: 该任务的所有对话列表。
//   - conversationPath: 原始 conversation 文件路径，用于获取文件大小作为增量检测依据。
//   - silicaPath: 输出 silica 文件的目标路径。
//
// 返回值：文件写入过程中发生的错误。
// 关键技术原理：calcLineFingerprint 为每行新增代码生成稳定指纹，用于后续任务间的代码相似度分析。
func generateTaskSilicaFile(ss *taskSession, conversations []taskConversation, conversationPath, silicaPath string) error {
	// 获取 conversation 文件大小，用于后续增量导入时判断是否需要更新
	var fileSize int64
	if info, err := os.Stat(conversationPath); err == nil {
		fileSize = info.Size()
	}

	// 组装 taskSilicaData 基础信息
	tsd := taskSilicaData{
		SessionId:       ss.SessionId,
		UserId:          ss.UserId,
		Size:            fileSize,
		ConversationNum: len(conversations),
	}

	// 逐条对话提取新增代码行的指纹
	for _, conv := range conversations {
		// 无新增代码行的对话无需记录指纹
		if len(conv.addedLines) == 0 {
			continue
		}

		var fingerprints []string
		if ss.RepoAddr != "" {
			for _, al := range conv.addedLines {
				fingerprints = append(fingerprints, calcLineFingerprint(al))
			}
		}
		tsc := taskSilicaConversation{
			RequestId:      conv.RequestId,
			StartTime:      conv.StartTime,
			EndTime:        conv.EndTime,
			RepoAddr:       conv.RepoAddr,
			UserInputChars: len(conv.UserInput),
			Fingerprints:   fingerprints,
		}
		tsd.Conversations = append(tsd.Conversations, tsc)
	}

	// 确保目标目录存在
	dir := filepath.Dir(silicaPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 序列化为 JSON
	data, err := json.Marshal(tsd)
	if err != nil {
		return fmt.Errorf("序列化taskSilicaData失败: %w", err)
	}

	// 写入 silica 文件
	if err := os.WriteFile(silicaPath, data, 0644); err != nil {
		return fmt.Errorf("写入task silica文件失败: %w", err)
	}

	logx.Debugf("  task silica文件已生成(%d个conversation): %s", len(tsd.Conversations), silicaPath)
	return nil
}

// scanConversationFiles 扫描 conversation 目录下所有 .jsonl 文件，建立 sessionId -> 文件路径 映射。
// 功能：递归遍历目录，收集以 .jsonl 结尾的文件；若同一 sessionId 对应多个路径，保留字典序更大的路径。
// 参数：
//   - conversationDir: conversation 文件所在根目录。
//   - startDate: 开始日期（含），nil 表示不限。
//   - endDate: 结束日期（不含），nil 表示不限。
//
// 返回值：taskID 到文件路径的映射；扫描失败时返回错误。
func scanConversationFiles(conversationDir string, startDate, endDate *time.Time) (map[string]string, error) {
	convMap := make(map[string]string)
	// 目录不存在时返回空映射，不做报错处理
	if _, err := os.Stat(conversationDir); os.IsNotExist(err) {
		return convMap, nil
	}

	err := filepath.Walk(conversationDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// 仅处理 .jsonl 文件
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".jsonl") {
			// 从相对路径中提取日期信息 YYYY/MM/DD
			relPath, err := filepath.Rel(conversationDir, path)
			if err != nil {
				return err
			}
			dateStr := filepath.ToSlash(filepath.Dir(relPath))
			fileDate, err := time.Parse("2006/01/02", dateStr)
			if err != nil {
				// 路径格式不符合预期，跳过该文件
				return nil
			}
			// 日期范围过滤：不在 [startDate, endDate) 范围内时跳过
			if !isActiveTimeInRange(fileDate, startDate, endDate) {
				return nil
			}

			sessionId := strings.TrimSuffix(info.Name(), ".jsonl")
			// 若存在同名 sessionId 的多个文件，保留字典序更大的路径（通常表示更晚生成或更优先的版本）
			if existing, ok := convMap[sessionId]; !ok || path > existing {
				convMap[sessionId] = path
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("扫描conversation目录失败: %w", err)
	}
	return convMap, nil
}

// scanSessionFiles 扫描 ss 目录下所有 .json 文件，建立 sessionId -> 文件路径 映射。
// 功能：递归遍历目录，收集以 .json 结尾的文件；若同一 sessionId 对应多个路径，保留字典序更大的路径。
// 参数：
//   - summaryDir: ss 文件所在根目录。
//
// 返回值：taskID 到文件路径的映射；扫描失败时返回错误。
func scanSessionFiles(summaryDir string) (map[string]string, error) {
	sessionMap := make(map[string]string)
	// 目录不存在时返回空映射
	if _, err := os.Stat(summaryDir); os.IsNotExist(err) {
		return sessionMap, nil
	}
	err := filepath.Walk(summaryDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// 仅处理 .json 文件
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".json") {
			sessionId := strings.TrimSuffix(info.Name(), ".json")
			// 同名冲突时保留字典序更大的路径
			if existing, ok := sessionMap[sessionId]; !ok || path > existing {
				sessionMap[sessionId] = path
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("扫描summary目录失败: %w", err)
	}
	return sessionMap, nil
}

// needUpdateConversations 判断是否需要重新导入某任务的 conversation 数据。
// 功能：基于 silica 文件中记录的 conversation 文件大小，与当前文件实际大小比对，实现增量检测。
// 参数：
//   - conversationPath: 当前 conversation 文件路径。
//   - silicaPath: 上次生成的 silica 文件路径。
//   - force: 若 force 为 true，则跳过检测直接返回 true（强制重新导入）。
//
// 返回值：需要更新时返回 true，否则返回 false。
func needUpdateConversations(conversationPath, silicaPath string, force bool) bool {
	// 强制模式直接返回需要更新
	if force {
		return true
	}
	// 读取并解析已有的 silica 文件
	data, err := os.ReadFile(silicaPath)
	if err != nil {
		return true
	}
	var tsd taskSilicaData
	if err := json.Unmarshal(data, &tsd); err != nil {
		return true
	}
	// 获取当前 conversation 文件大小并与 silica 中记录的值比较
	info, err := os.Stat(conversationPath)
	if err != nil {
		return true
	}
	return info.Size() != tsd.Size
}

// runImportConv 执行完整的 task 批量导入流程。
// 功能：扫描 summary 和 conversation 目录，配对后逐任务导入；支持增量检测和强制重导。
// 参数：
//   - taskDir: 任务数据根目录，内部包含 summary/ 和 conversation/ 子目录。
//   - analysedDir: 分析结果输出目录，用于存放 silica 文件。
//   - force: 是否强制重新导入所有任务。
//
// 返回值：导入流程中发生的不可恢复错误。
// 关键技术原理：
//  1. 分别扫描 summary 和 conversation 目录，按 sessionId 建立映射。
//  2. 对每个 conversation 文件，先检查是否存在对应 summary；再调用 needUpdateConversations 进行增量检测。
//  3. 调用 importSingleTask 完成单任务导入，并统计成功/失败/跳过数量。
//  4. 通过 recordCommandRun 记录命令执行结果，便于运维监控和审计。
func runImportConv(taskDir, analysedDir string, force bool, startDateStr, endDateStr, dateStr string, createPseudo bool) error {
	startTime := time.Now()
	summaryDir := filepath.Join(taskDir, "summary")
	conversationDir := filepath.Join(taskDir, "conversation")

	// 校验 summary 目录必须存在
	if _, err := os.Stat(summaryDir); os.IsNotExist(err) {
		recordCommandRun("import-conv", startTime, 0, 0, 0, err)
		return fmt.Errorf("summary目录不存在: %s", summaryDir)
	}

	// 解析日期范围
	startDate, endDate, err := parseDateRange(startDateStr, endDateStr, dateStr)
	if err != nil {
		recordCommandRun("import-conv", startTime, 0, 0, 0, err)
		return err
	}

	// 连接数据库
	db, err := models.OpenGormDB(cfg.StatDatabase.DSN())
	if err != nil {
		recordCommandRun("import-conv", startTime, 0, 0, 0, err)
		return fmt.Errorf("连接数据库失败: %w", err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	// 扫描 conversation 和 summary 文件
	convMap, err := scanConversationFiles(conversationDir, startDate, endDate)
	if err != nil {
		recordCommandRun("import-conv", startTime, 0, 0, 0, err)
		return err
	}

	sessionMap, err := scanSessionFiles(summaryDir)
	if err != nil {
		recordCommandRun("import-conv", startTime, 0, 0, 0, err)
		return err
	}

	// 无 conversation 文件时直接结束
	if len(convMap) == 0 {
		logx.Info("没有找到待导入的 conversation 文件")
		recordCommandRun("import-conv", startTime, 0, 0, 0, nil)
		return nil
	}

	successCount := 0
	failCount := 0
	skipCount := 0

	// 遍历所有 conversation 文件，匹配 summary 并执行导入
	totalConv := len(convMap)
	processed := 0
	// 累积成批写入：每 importConvBatchSize 个 session 合并到一个事务提交，
	// 把「单 session 一次 fsync」摊薄成「每批一次」，显著提速大批量导入。
	const importConvBatchSize = 200
	batch := make([]*preparedImportTask, 0, importConvBatchSize)
	flush := func() {
		s, f := flushImportConvBatch(db, batch)
		successCount += s
		failCount += f
		batch = batch[:0]
	}
	for sessionId, conversationPath := range convMap {
		processed++
		logx.Progress("[import-conv] 导入对话", processed, totalConv, 50)
		summaryPath, ok := sessionMap[sessionId]
		if !ok {
			logx.Debugf("跳过(无对应summary): %s", sessionId)
			skipCount++
			continue
		}

		// 构造 silica 文件路径并判断是否需要增量更新
		silicaPath := filepath.Join(analysedDir, "task", "conversation", sessionId+".silica.json")
		if !needUpdateConversations(conversationPath, silicaPath, force) {
			logx.Debugf("跳过(conversation未更新): %s", sessionId)
			skipCount++
			continue
		}

		// 解析放在事务外：解析失败（如缺字段）只跳过当前 session，不污染批量事务
		p, err := prepareImportTask(summaryPath, conversationPath, silicaPath)
		if err != nil {
			if errors.Is(err, errSkipTask) {
				logx.Debugf("跳过(日期范围过滤): %s", sessionId)
				skipCount++
				continue
			}
			logx.Warnf("导入失败 [%s]: %v", sessionId, err)
			failCount++
			continue
		}
		batch = append(batch, p)
		if len(batch) >= importConvBatchSize {
			flush()
		}
	}
	flush() // 写入尾批

	logx.Infof("导入完成: 成功 %d 个，失败 %d 个，跳过 %d 个", successCount, failCount, skipCount)

	if createPseudo {
		if err := createPseudoTasks(db); err != nil {
			logx.Warnf("创建伪任务失败: %v", err)
		}
	}

	recordCommandRun("import-conv", startTime, successCount, failCount, skipCount, nil)
	return nil
}

// importConvCmd 定义了 "import-conv" CLI 子命令，用于将本地 task 数据导入到统计数据库。
// 功能：支持本地导入和远程执行两种模式；本地模式下自动从配置或命令行参数获取目录路径。
var importConvCmd = &cobra.Command{
	Use:   "import-conv",
	Short: "导入 task 数据到 costrict_stat 数据库",
	RunE: func(cmd *cobra.Command, args []string) error {
		// 从命令行参数获取输入目录、输出目录、强制标志和远程地址
		taskDir, _ := cmd.Flags().GetString("task-dir")
		analysedDir, _ := cmd.Flags().GetString("analysed-dir")
		force, _ := cmd.Flags().GetBool("force")
		remote, _ := cmd.Flags().GetString("remote")
		startDate, _ := cmd.Flags().GetString("start-date")
		endDate, _ := cmd.Flags().GetString("end-date")
		date, _ := cmd.Flags().GetString("date")
		createPseudo, _ := cmd.Flags().GetBool("create-pseudo")
		if !cmd.Flags().Changed("create-pseudo") {
			createPseudo = cfg.TaskCreate.CreatePseudoTask
		}

		// 若指定了 remote 地址，则将命令参数序列化后发送到远程 kbcli 服务执行
		if remote != "" {
			return sendToRemote(remote, "import-conv", map[string]interface{}{
				"task_dir":      taskDir,
				"analysed_dir":  analysedDir,
				"force":         force,
				"start_date":    startDate,
				"end_date":      endDate,
				"date":          date,
				"create_pseudo": createPseudo,
			})
		}
		// 若未显式指定目录，回退到配置文件中的默认值
		if taskDir == "" {
			taskDir = cfg.TaskDir
		}
		if analysedDir == "" {
			analysedDir = cfg.AnalysedDir
		}
		// 未显式传 start-date 且非单日(date)模式时，套全局分析起始日下界。
		if date == "" {
			startDate = applyAnalysisFloor(startDate)
		}

		return runImportConv(taskDir, analysedDir, force, startDate, endDate, date, createPseudo)
	},
}

// init 注册 import-conv 子命令及其命令行参数到 rootCmd。
// 功能：在程序初始化阶段完成 Cobra 命令和 Flag 的绑定。
func init() {
	importConvCmd.Flags().SortFlags = false
	importConvCmd.Flags().String("task-dir", "", "task 目录路径")
	importConvCmd.Flags().String("analysed-dir", "", "输出目录路径")
	importConvCmd.Flags().BoolP("force", "f", false, "强制重新导入，覆盖已存在数据")
	importConvCmd.Flags().String("start-date", "", "限定起始日期，格式 YYYYMMDD，为空则不限")
	importConvCmd.Flags().String("end-date", "", "限定结束日期，格式 YYYYMMDD，为空则不限")
	importConvCmd.Flags().String("date", "", "限定日期，格式 YYYYMMDD，限定活跃时间在该日期之内（与start-date/end-date互斥）")
	importConvCmd.Flags().Bool("create-pseudo", false, "为所有session创建伪任务（默认从config读取）")
	importConvCmd.Flags().String("remote", "", "远程kbcli服务地址（如 http://127.0.0.1:8080），指定后命令将发送到远程执行")
	rootCmd.AddCommand(importConvCmd)
}
