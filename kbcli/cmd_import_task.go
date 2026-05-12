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

// taskSummary 表示单个任务的基本摘要信息，对应 summary 目录下的 JSON 文件。
type taskSummary struct {
	TaskId          string `json:"task_id"`
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
	Diff            string `json:"diff"`
	DiffLines       int    `json:"diff_lines"`
}

// taskConversation 表示一次任务中的单次对话记录，对应 conversation 目录下 JSONL 的每一行。
type taskConversation struct {
	Sender           string     `json:"sender"`
	RequestId        string     `json:"request_id"`
	PromptMode       string     `json:"prompt_mode"`
	Mode             string     `json:"mode"`
	Model            string     `json:"model"`
	StartTime        string     `json:"start_time"`
	EndTime          string     `json:"end_time"`
	ProcessTime      int64      `json:"process_time"`
	ProcessTtft      int64      `json:"process_ttft"`
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
	addedLines []addedLine
	// startTime、endTime 为解析后的 time.Time 类型时间，用于数据库写入和时长计算。
	startTime time.Time
	endTime   time.Time
}

// taskSilicaData 用于生成任务级别的 silica 摘要文件，保存到 analysedDir 中。
type taskSilicaData struct {
	TaskId          string                   `json:"task_id"`
	RepoAddr        string                   `json:"repo_addr"`
	UserId          string                   `json:"user_id"`
	Size            int64                    `json:"size"`
	ConversationNum int                      `json:"conversation_num"`
	Conversations   []taskSilicaConversation `json:"conversations"`
}

// taskSilicaConversation 为 taskSilicaData 中的单条对话摘要，记录指纹信息。
type taskSilicaConversation struct {
	RequestId    string   `json:"request_id"`
	EndTime      string   `json:"end_time"`
	Fingerprints []string `json:"fingerprints"`
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

// calcTaskRecord 根据任务摘要和对话列表计算完整的 Task 模型记录。
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
func calcTaskRecord(summary *taskSummary, conversations []taskConversation) models.Task {
	// 初始化 Task 基础字段，WorkDirId 通过工具函数根据 ClientId 和 WorkDir 生成唯一标识
	rec := models.Task{
		TaskId:          summary.TaskId,
		UserId:          summary.UserId,
		UserName:        summary.UserName,
		ClientId:        summary.ClientId,
		ClientIde:       summary.ClientIde,
		ClientVersion:   summary.ClientVersion,
		ClientOs:        summary.ClientOs,
		ClientOsVersion: summary.ClientOsVersion,
		Caller:          summary.Caller,
		RepoAddr:        summary.RepoAddr,
		RepoBranch:      summary.RepoBranch,
		WorkDir:         summary.WorkDir,
		WorkDirId:       utils.GenerateWorkDirID(summary.ClientId, summary.WorkDir),
	}

	// 聚合变量：时间范围、Token、成本、代码行数
	var startTime, endTime *time.Time
	var totalUpstream, totalDownstream int64
	var totalCost float64
	var totalLines int64

	// 遍历所有对话，解析时间并累加指标；时间解析失败则跳过该对话并记录警告
	for _, conv := range conversations {
		if conv.StartTime == "" {
			logWarnf("conversation [%s-%s] 缺少start_time字段", summary.TaskId, conv.RequestId)
			continue
		}
		if conv.EndTime == "" {
			logWarnf("conversation [%s-%s] 缺少end_time字段", summary.TaskId, conv.RequestId)
			continue
		}
		t1, err := time.Parse(time.RFC3339, conv.StartTime)
		if err != nil {
			logWarnf("conversation [%s-%s] start_time字段解析错误: %v", summary.TaskId, conv.RequestId, err)
			continue
		}
		t2, err := time.Parse(time.RFC3339, conv.EndTime)
		if err != nil {
			logWarnf("conversation [%s-%s] end_time字段解析错误: %v", summary.TaskId, conv.RequestId, err)
			continue
		}
		// 维护任务级别最早开始时间和最晚结束时间
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

	// 将聚合结果写入 Task 记录
	rec.StartTime = startTime
	rec.EndTime = endTime
	rec.UpstreamTokens = totalUpstream
	rec.DownstreamTokens = totalDownstream
	rec.Cost = totalCost
	rec.DiffLines = int(totalLines)

	// 计算真实工作时长（基于时间片段合并算法）
	minutes, reason := calcTaskRealMinutes(conversations, cfg.TaskStatistics.GapThresholdMinutes, cfg.TaskStatistics.ExtensionMinutes)
	rec.TaskRealMinutes = minutes
	rec.TaskRealMinutesReason = reason

	// 估算原始工作量（基于输入字符数和代码行数的因子模型）
	minutes, reason = estimateTaskAncientMinutes(&cfg.AlgoEstimation, conversations, rec.TaskRealMinutes)
	rec.TaskAncientMinutes = minutes
	rec.TaskAncientMinutesReason = reason
	return rec
}

// importSingleTask 导入单个任务到数据库。
// 功能：读取 summary 和 conversation 文件，解析并计算任务记录，生成 silica 文件后写入数据库。
// 参数：
//   - db: GORM 数据库连接，用于写入 tasks 和 task_conversations 表。
//   - summaryPath: summary JSON 文件路径。
//   - conversationPath: conversation JSONL 文件路径。
//   - silicaPath: 输出 silica 文件的路径。
//
// 返回值：导入过程中发生的错误。
// 关键技术原理：使用 GORM 的 clause.OnConflict 实现 UPSERT（冲突时更新指定列），保证多次导入幂等。
func importSingleTask(db *gorm.DB, summaryPath, conversationPath, silicaPath string) error {
	// 读取并解析 summary JSON 文件
	data, err := os.ReadFile(summaryPath)
	if err != nil {
		return fmt.Errorf("读取summary文件失败: %w", err)
	}

	var summary taskSummary
	if err := json.Unmarshal(data, &summary); err != nil {
		return fmt.Errorf("解析summary JSON失败: %w", err)
	}

	// 校验关键字段，防止写入无效数据
	if summary.TaskId == "" {
		return fmt.Errorf("task_id为空")
	}
	if summary.UserId == "" {
		return fmt.Errorf("user_id为空")
	}

	// 解析对话文件，得到该任务下的所有对话列表
	conversations, err := parseConversationFile(conversationPath)
	if err != nil {
		return fmt.Errorf("解析conversation文件失败: %w", err)
	}

	// 根据 summary 和 conversations 计算完整的 Task 记录
	task := calcTaskRecord(&summary, conversations)

	// 生成 task silica 文件用于后续增量检测；失败仅记录警告，不阻断主流程
	if err := generateTaskSilicaFile(&summary, conversations, conversationPath, silicaPath); err != nil {
		logWarnf("生成task silica文件失败 [%s]: %v", summary.TaskId, err)
	}

	// 使用 UPSERT 写入 tasks 表：task_id 冲突时更新除主键外的业务字段
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

	// 调试日志：输出估算时长信息
	if task.TaskRealMinutes > 0 {
		logDebugf("  task_real_minutes=%.1f (%s)", task.TaskRealMinutes, task.TaskRealMinutesReason)
	}
	if task.TaskAncientMinutes > 0 {
		logDebugf("  task_ancient_minutes=%.1f (%s)", task.TaskAncientMinutes, task.TaskAncientMinutesReason)
	}

	// 若存在有效对话，将其保存到 task_conversations 表
	if len(conversations) > 0 {
		if err := saveConversations(db, task.TaskId, conversations); err != nil {
			return fmt.Errorf("保存conversations失败: %w", err)
		}
	}

	logDebugf("导入成功: %s", task.TaskId)
	return nil
}

// saveConversations 将任务对话列表批量保存到数据库，使用事务保证原子性。
// 功能：逐条将 taskConversation 转换为 models.TaskConversation 后插入，冲突时忽略（DoNothing）。
// 参数：
//   - db: GORM 数据库连接。
//   - taskID: 所属任务的唯一标识。
//   - conversations: 需要保存的对话列表。
//
// 返回值：事务执行过程中发生的错误。
// 关键技术原理：通过 db.Transaction 开启事务，确保一批对话要么全部写入成功，要么全部回滚；
// 复合唯一键 (task_id, request_id) 冲突时忽略插入，避免重复数据报错。
func saveConversations(db *gorm.DB, taskID string, conversations []taskConversation) error {
	return db.Transaction(func(tx *gorm.DB) error {
		for _, conv := range conversations {
			// 转换字段并对文本内容进行清洗，防止非法字符入库
			tc := models.TaskConversation{
				TaskId:           taskID,
				RequestId:        conv.RequestId,
				Sender:           conv.Sender,
				PromptMode:       conv.PromptMode,
				Mode:             conv.Mode,
				Model:            conv.Model,
				StartTime:        conv.startTime,
				EndTime:          conv.endTime,
				ProcessTime:      conv.ProcessTime,
				ProcessTtft:      conv.ProcessTtft,
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

			// 复合主键冲突时忽略，避免同一对话重复导入导致事务失败
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

// estimateTaskAncientMinutes 估算任务的“原始工作量”分钟数。
// 功能：基于用户输入字符数和代码新增行数，结合配置参数，通过线性因子模型估算开发者完成该任务所需的原始时间。
// 参数：
//   - cfg: 估算算法的配置参数，包含最大输入字符、因子范围、每行代码耗时等。
//   - convs: 任务下的所有对话列表。
//   - realMinutes: 已计算出的真实工作时长，用于设置上下界约束。
//
// 返回值：
//   - float64: 估算的原始工作量分钟数。
//   - string: 估算依据的说明文本。
//
// 关键技术原理：
//  1. 汇总所有对话的 user_input 长度（totalInchars）和 diff 新增行数（totalDiffLines）。
//  2. 输入字符数经截断后，映射到 [MinFactor, MaxFactor] 区间的线性因子 factor。
//  3. workload = (diffLines / LinesPerMinutes) * factor，反映“按代码量估算的时间”。
//  4. 用 maxWorkload（真实时长的 MaxRatio 倍）和 minWorkload（不小于 MinMinutes 且不小于真实时长）进行上下界裁剪，防止极端偏差。
func estimateTaskAncientMinutes(ec *EstimateConfig, convs []taskConversation, realMinutes float64) (float64, string) {
	var totalInchars int64
	var totalDiffLines int64

	// 汇总所有对话的输入字符数和新增代码行数
	for _, conv := range convs {
		totalInchars += int64(len(conv.UserInput))
		totalDiffLines += conv.DiffLines
	}

	inchars := float64(totalInchars)
	diffLines := float64(totalDiffLines)

	// 输入字符数超过配置上限时截断，避免异常大输入导致 factor 越界
	if inchars >= ec.MaxInputChars {
		inchars = ec.MaxInputChars
	}

	// 线性插值计算因子：输入越多，因子越接近 MaxFactor
	factor := ec.MinFactor + (inchars/ec.MaxInputChars)*(ec.MaxFactor-ec.MinFactor)
	// 基于代码行数和因子计算初步工作量
	workload := (diffLines / ec.LinesPerMinutes) * factor

	// 计算上下界：上限为真实时长的 MaxRatio 倍；下限不小于 MinMinutes 且不小于真实时长
	maxWorkload := ec.MaxRatio * realMinutes
	minWorkload := max(ec.MinMinutes, realMinutes)

	// 对 workload 进行裁剪，确保结果在合理区间内
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

// calcTaskRealMinutes 计算任务的真实工作时长（分钟）。
// 功能：根据所有对话的开始时间，将相邻且间隔不超过阈值的时间合并为连续工作片段，累加各片段时长得到总工作时长。
// 参数：
//   - conversations: 任务下的所有对话列表。
//   - gapThreshold: 两条对话之间允许的最大间隔（分钟），超过则认为进入新的工作片段。
//   - extensionMin: 每个工作片段结束时追加的延展时间（分钟），用于覆盖末尾等待或思考时间。
//
// 返回值：
//   - float64: 真实工作总时长（分钟）。
//   - string: 各时间片段的详细说明文本。
//
// 关键技术原理：
//  1. 提取所有有效开始时间并排序。
//  2. 顺序遍历时间序列：若相邻时间差 <= gapThreshold，则合并到当前片段；否则结束当前片段并追加 extensionMin，同时开启新片段。
//  3. 最后一个片段同样追加 extensionMin。
//  4. 累加各片段 (end - start) 的分钟数得到总时长。
func calcTaskRealMinutes(conversations []taskConversation, gapThreshold, extensionMin int) (float64, string) {
	// 收集所有成功解析的开始时间
	var validTimes []time.Time
	for _, conv := range conversations {
		if conv.StartTime != "" {
			if t, err := time.Parse(time.RFC3339, conv.StartTime); err == nil {
				validTimes = append(validTimes, t)
			}
		}
	}
	// 边界情况：无有效对话或仅一条对话时返回固定默认值
	if len(validTimes) == 0 {
		return 0, "无有效对话"
	}
	if len(validTimes) == 1 {
		return float64(extensionMin), fmt.Sprintf("仅1条对话，默认%d分钟", extensionMin)
	}
	// 按时间先后排序，保证片段合并顺序正确
	sort.Slice(validTimes, func(i, j int) bool {
		return validTimes[i].Before(validTimes[j])
	})
	gapDur := time.Duration(gapThreshold) * time.Minute
	ext := time.Duration(extensionMin) * time.Minute
	// timeSeg 表示一个连续工作片段：start 为片段开始时间，end 为片段结束时间（含追加的延展时间），convCount 为片段内对话数
	type timeSeg struct {
		start     time.Time
		end       time.Time
		convCount int
	}
	// 初始化第一个片段
	segments := []timeSeg{{start: validTimes[0], end: validTimes[0], convCount: 1}}
	for i := 1; i < len(validTimes); i++ {
		cur := &segments[len(segments)-1]
		gap := validTimes[i].Sub(cur.end)
		// 若间隔在阈值内，合并到当前片段
		if gap <= gapDur {
			cur.end = validTimes[i]
			cur.convCount++
		} else {
			// 结束当前片段并追加延展时间，同时开启新片段
			cur.end = cur.end.Add(ext)
			segments = append(segments, timeSeg{start: validTimes[i], end: validTimes[i], convCount: 1})
		}
	}
	// 最后一个片段也需要追加延展时间
	segments[len(segments)-1].end = segments[len(segments)-1].end.Add(ext)
	var totalMinutes float64
	var parts []string
	// 累加所有片段时长并生成说明文本
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

// calculateCost 根据模型和 Token 数计算调用成本。
// 功能：通过模型前缀匹配价格表，计算 (inputTokens * InPrice + outputTokens * OutPrice) / 1e6 的总成本。
// 参数：
//   - model: 模型名称（大小写不敏感）。
//   - inTokens: 上游（输入）Token 数量。
//   - outTokens: 下游（输出）Token 数量。
//   - prices: 模型价格映射表，key 为模型前缀或 "default"。
//
// 返回值：计算后的成本（货币单位）。
// 关键技术原理：前缀最长匹配策略——遍历价格表，找所有为 model 前缀的 key，取长度最长的一个；无匹配则回退到 default；仍未找到则返回 0。
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
		// 无匹配前缀时回退到 default 价格
		var ok bool
		price, ok = prices["default"]
		if !ok {
			return 0
		}
	}

	// 成本公式：按百万 Token 计价
	return (float64(inTokens)/1e6)*price.InPrice + (float64(outTokens)/1e6)*price.OutPrice
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

// calcConversation 对单个对话进行字段解析和指标计算。
// 功能：解析时间字段、补全缺失的成本、提取用户输入、解析 Diff 并统计新增行数。
// 参数：
//   - conv: 指向 taskConversation 的指针，函数会直接修改该结构体。
//
// 返回值：校验或解析失败时返回错误；成功返回 nil。
// 关键技术原理：
//   - 若 Cost 为 0 但有 Token 和模型信息，则调用 calculateCost 自动补全成本。
//   - Diff 通过 extractAddedLinesFromDiff 解析为新增代码行列表；解析后清空 Diff 原文以节省内存。
func calcConversation(conv *taskConversation) error {
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

	// 若成本缺失且存在 Token 和模型信息，自动计算成本
	if conv.Cost == 0 && conv.UpstreamTokens > 0 && conv.Model != "" {
		conv.Cost = calculateCost(conv.Model, conv.UpstreamTokens, conv.DownstreamTokens, cfg.ModelPrices)
	}
	// 去除用户输入的包装标签
	conv.UserInput = parseUserInput(conv.UserInput)
	// 解析 Diff 文本，提取新增代码行
	if strings.TrimSpace(conv.Diff) != "" {
		conv.addedLines = extractAddedLinesFromDiff(conv.Diff)
	}
	// 统计新增代码行数，并清空 Diff 原文以释放内存
	conv.DiffLines = int64(len(conv.addedLines))
	conv.Diff = ""
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
// 功能：尝试将一行 JSONL 内容反序列化为 taskConversation，并调用 calcConversation 完成字段计算和校验。
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
			logWarnf("解析[%s:%d]失败: %v, 内容: %s", path, lineNum, err, skeletonize(string(content), 40, 64))
		}
		return nil, err
	}
	// 计算并校验字段；校验失败不返回错误，仅记录警告并返回 nil，避免单条坏数据阻断整批导入
	if err := calcConversation(&conv); err != nil {
		logWarnf("解析[%s:%d]中的对话发生错误: %v", path, lineNum, err)
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
	// 显式设置 scanner 缓冲区：初始 64KB，最大 10MB，以支持超长 JSON 行
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		// 跳过空行
		if line == "" {
			continue
		}
		// 先尝试将整行作为一个 JSON 对象解析；ignoreUnmarshalWarning=true 减少冗余日志
		if c, err := parseConversation(path, lineNum, []byte(line), true); err == nil {
			if c != nil {
				convs = append(convs, *c)
			}
			continue
		}
		// 容错：尝试将单行按独立的 JSON 对象拆分解析
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
//   - summary: 任务摘要，用于填充 silica 中的基础信息。
//   - conversations: 该任务的所有对话列表。
//   - conversationPath: 原始 conversation 文件路径，用于获取文件大小作为增量检测依据。
//   - silicaPath: 输出 silica 文件的目标路径。
//
// 返回值：文件写入过程中发生的错误。
// 关键技术原理：calcLineFingerprint 为每行新增代码生成稳定指纹，用于后续任务间的代码相似度分析。
func generateTaskSilicaFile(summary *taskSummary, conversations []taskConversation, conversationPath, silicaPath string) error {
	// 获取 conversation 文件大小，用于后续增量导入时判断是否需要更新
	var fileSize int64
	if info, err := os.Stat(conversationPath); err == nil {
		fileSize = info.Size()
	}

	// 组装 taskSilicaData 基础信息
	tsd := taskSilicaData{
		TaskId:          summary.TaskId,
		RepoAddr:        summary.RepoAddr,
		UserId:          summary.UserId,
		Size:            fileSize,
		ConversationNum: len(conversations),
	}
	if summary.RepoAddr == "" {
		logDebugf("任务[%s]无法关联Commit,忽略代码指纹信息生成", summary.TaskId)
	}

	// 逐条对话提取新增代码行的指纹
	for _, conv := range conversations {
		// 无新增代码行的对话无需记录指纹
		if len(conv.addedLines) == 0 {
			continue
		}

		var fingerprints []string
		if summary.RepoAddr != "" {
			for _, al := range conv.addedLines {
				fingerprints = append(fingerprints, calcLineFingerprint(al))
			}
		}
		tsc := taskSilicaConversation{
			RequestId:    conv.RequestId,
			EndTime:      conv.EndTime,
			Fingerprints: fingerprints,
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

	logDebugf("  task silica文件已生成(%d个conversation): %s", len(tsd.Conversations), silicaPath)
	return nil
}

// scanConversationFiles 扫描 conversation 目录下所有 .jsonl 文件，建立 taskID -> 文件路径 映射。
// 功能：递归遍历目录，收集以 .jsonl 结尾的文件；若同一 taskID 对应多个路径，保留字典序更大的路径。
// 参数：
//   - conversationDir: conversation 文件所在根目录。
//
// 返回值：taskID 到文件路径的映射；扫描失败时返回错误。
func scanConversationFiles(conversationDir string) (map[string]string, error) {
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
			taskID := strings.TrimSuffix(info.Name(), ".jsonl")
			// 若存在同名 taskID 的多个文件，保留字典序更大的路径（通常表示更晚生成或更优先的版本）
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

// scanSummaryFiles 扫描 summary 目录下所有 .json 文件，建立 taskID -> 文件路径 映射。
// 功能：递归遍历目录，收集以 .json 结尾的文件；若同一 taskID 对应多个路径，保留字典序更大的路径。
// 参数：
//   - summaryDir: summary 文件所在根目录。
//
// 返回值：taskID 到文件路径的映射；扫描失败时返回错误。
func scanSummaryFiles(summaryDir string) (map[string]string, error) {
	summaryMap := make(map[string]string)
	// 目录不存在时返回空映射
	if _, err := os.Stat(summaryDir); os.IsNotExist(err) {
		return summaryMap, nil
	}
	err := filepath.Walk(summaryDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// 仅处理 .json 文件
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".json") {
			taskID := strings.TrimSuffix(info.Name(), ".json")
			// 同名冲突时保留字典序更大的路径
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

// runImportTask 执行完整的 task 批量导入流程。
// 功能：扫描 summary 和 conversation 目录，配对后逐任务导入；支持增量检测和强制重导。
// 参数：
//   - taskDir: 任务数据根目录，内部包含 summary/ 和 conversation/ 子目录。
//   - analysedDir: 分析结果输出目录，用于存放 silica 文件。
//   - force: 是否强制重新导入所有任务。
//
// 返回值：导入流程中发生的不可恢复错误。
// 关键技术原理：
//  1. 分别扫描 summary 和 conversation 目录，按 taskID 建立映射。
//  2. 对每个 conversation 文件，先检查是否存在对应 summary；再调用 needUpdateConversations 进行增量检测。
//  3. 调用 importSingleTask 完成单任务导入，并统计成功/失败/跳过数量。
//  4. 通过 recordCommandRun 记录命令执行结果，便于运维监控和审计。
func runImportTask(taskDir, analysedDir string, force bool) error {
	startTime := time.Now()
	summaryDir := filepath.Join(taskDir, "summary")
	conversationDir := filepath.Join(taskDir, "conversation")

	// 校验 summary 目录必须存在
	if _, err := os.Stat(summaryDir); os.IsNotExist(err) {
		recordCommandRun("import-task", startTime, 0, 0, 0, err)
		return fmt.Errorf("summary目录不存在: %s", summaryDir)
	}

	// 连接数据库
	db, err := models.OpenGormDB(cfg.StatDatabase.DSN())
	if err != nil {
		recordCommandRun("import-task", startTime, 0, 0, 0, err)
		return fmt.Errorf("连接数据库失败: %w", err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	// 扫描 conversation 和 summary 文件
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

	// 无 conversation 文件时直接结束
	if len(convMap) == 0 {
		logInfo("没有找到待导入的 conversation 文件")
		recordCommandRun("import-task", startTime, 0, 0, 0, nil)
		return nil
	}

	successCount := 0
	failCount := 0
	skipCount := 0

	// 遍历所有 conversation 文件，匹配 summary 并执行导入
	for taskID, conversationPath := range convMap {
		summaryPath, ok := summaryMap[taskID]
		if !ok {
			logDebugf("跳过(无对应summary): %s", taskID)
			skipCount++
			continue
		}

		// 构造 silica 文件路径并判断是否需要增量更新
		silicaPath := filepath.Join(analysedDir, "task", "conversation", taskID+".silica.json")
		if !needUpdateConversations(conversationPath, silicaPath, force) {
			logDebugf("跳过(conversation未更新): %s", taskID)
			skipCount++
			continue
		}

		// 执行单任务导入
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

// importTasksCmd 定义了 "import-task" CLI 子命令，用于将本地 task 数据导入到统计数据库。
// 功能：支持本地导入和远程执行两种模式；本地模式下自动从配置或命令行参数获取目录路径。
var importTasksCmd = &cobra.Command{
	Use:   "import-task",
	Short: "导入 task 数据到 costrict_stat 数据库",
	RunE: func(cmd *cobra.Command, args []string) error {
		// 从命令行参数获取输入目录、输出目录、强制标志和远程地址
		taskDir, _ := cmd.Flags().GetString("task-dir")
		analysedDir, _ := cmd.Flags().GetString("analysed-dir")
		force, _ := cmd.Flags().GetBool("force")
		remote, _ := cmd.Flags().GetString("remote")

		// 若指定了 remote 地址，则将命令参数序列化后发送到远程 kbcli 服务执行
		if remote != "" {
			return sendToRemote(remote, "import-task", map[string]interface{}{
				"task_dir":     taskDir,
				"analysed_dir": analysedDir,
				"force":        force,
			})
		}
		// 若未显式指定目录，回退到配置文件中的默认值
		if taskDir == "" {
			taskDir = cfg.TaskDir
		}
		if analysedDir == "" {
			analysedDir = cfg.AnalysedDir
		}

		return runImportTask(taskDir, analysedDir, force)
	},
}

// init 注册 import-task 子命令及其命令行参数到 rootCmd。
// 功能：在程序初始化阶段完成 Cobra 命令和 Flag 的绑定。
func init() {
	importTasksCmd.Flags().SortFlags = false
	importTasksCmd.Flags().String("task-dir", "", "task 目录路径")
	importTasksCmd.Flags().String("analysed-dir", "", "输出目录路径")
	importTasksCmd.Flags().BoolP("force", "f", false, "强制重新导入，覆盖已存在数据")
	importTasksCmd.Flags().String("remote", "", "远程kbcli服务地址（如 http://127.0.0.1:8080），指定后命令将发送到远程执行")
	rootCmd.AddCommand(importTasksCmd)
}
