package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"kanban/core/models"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/spf13/cobra"
)

// convMeta 存储单个对话（conversation）的元数据，用于在组内标识一次AI交互。
type convMeta struct {
	sessionId      string    // 所属的session
	requestId      string    // 对话请求ID，在一个session中，是唯一的，标识一次AI请求
	DiffLines      int       // 对话生成的代码行数
	UserInputChars int       // 对话的用户输入字符数
	startTime      time.Time // 对话开始时间
	endTime        time.Time // 对话结束时间，用于与commit时间进行先后比较
}

type sessionMeta struct {
	sessionId     string
	conversations []*convMeta
}

// groupKey 定义分组的复合键，将同一仓库下同一用户的对话归为一组。
// 原理：AI对话产生的代码行通常与特定仓库和特定开发者强相关，
// 按(repoAddr, userID)分组可避免跨仓库/跨用户的指纹误匹配。
type groupKey struct {
	repoAddr string // 仓库地址
	userID   string // 用户ID
}

// groupIndexer 管理同一分组内的对话列表及其产生的代码行指纹索引。
// 关键技术原理： Lines 采用 map[string]int 结构，键为代码行指纹（hash），
// 值为该指纹在 Convs 切片中的索引。当同一指纹被多个对话生成时，
// 保留 endTime 最晚的对话（见 buildConversationsIndexer 中的冲突处理逻辑），
// 确保“最接近commit时间”的对话获得该指纹的归属权。
type groupIndexer struct {
	Lines    map[string]*convMeta    // 代码行指纹 -> Convs数组索引，映射到生成该行的对话
	Sessions map[string]*sessionMeta //sessionId -> sessionMeta对象
}

// conversationsIndexer 全局conversation指纹索引，聚合所有分组的数据。
// 用于快速判断某行代码是否由某次AI对话生成。
type conversationsIndexer struct {
	groups map[groupKey]groupIndexer // 分组索引
}

// commitParser 用于解析单个commit的指纹文件并计算含硅量及相关衍生指标。
type commitParser struct {
	commitId         string    // commit的唯一ID
	fpHashs          []string  // commit指纹文件中的全部代码行指纹
	taskIds          []string  // 匹配的taskId列表（去重后）
	taskSilicas      []float64 // 每个task对当前commit的含硅量贡献值
	taskAcceptRatios []float64 // 每个task被当前commit采纳的代码行占该task代码行的比例
	totalLines       int       // commit总行数（指纹总数）
	totalMatchLines  int       // 与AI对话指纹匹配的行数
	totalSilica      float64   // 总含硅量（所有task贡献值之和）
	aiMinutes        float64   // AI生成代码对应的实际耗时（分钟）
	ancientMinutes   float64   // 非AI代码（传统编码）的估算耗时（分钟）
	realMinutes      float64   // 总实际耗时 = aiMinutes + ancientMinutes
	realReason       string    // 实际耗时的计算说明文本
	upstreamTokens   int64     // 上游token消耗总数
	downstreamTokens int64     // 下游token消耗总数
	cost             float64   // AI调用成本总数
}

// buildCommitParser 从指定的指纹文件路径构建commitParser实例。
//
// 参数:
//   - fpPath: commit指纹文件路径，每行一个代码行指纹（hash）。
//
// 返回值:
//   - *commitParser: 解析器实例，包含已加载的指纹列表。
//   - nil: 如果指纹文件读取失败。
func buildCommitParser(commitId, fpPath string) *commitParser {
	// 加载commit指纹文件中的所有hash值
	fpHashs, err := loadFPHashes(fpPath)
	if err != nil {
		return nil
	}
	return &commitParser{
		commitId: commitId,
		fpHashs:  fpHashs,
	}
}

// buildConversationsIndexer 扫描task指纹目录，构建全局conversation索引。
//
// 参数:
//   - taskFPDir: task/conversation指纹目录，预期包含若干 .silica.json 文件。
//
// 返回值:
//   - *conversationsIndexer: 构建完成的索引对象。
//   - error: 目录扫描或文件解析过程中的错误。
//
// 关键技术原理:
//  1. 遍历 taskFPDir 下所有 .silica.json 文件，每个文件记录了一个task的所有AI对话及其生成的代码行指纹。
//  2. 按 (repoAddr, userID) 将对话分组，因为同一开发者在同一仓库中的对话最可能与该仓库后续commit相关。
//  3. 在 groupIndexer.Lines 中维护“指纹 -> 对话索引”的映射。当同一指纹出现在多个对话中时，
//     采用 endTime 最晚的对话作为归属（时间最近优先原则），这基于一个假设：
//     越接近commit时间的对话，其生成代码被直接采纳的概率越高。
//  4. 索引构建完成后，可通过 buildCandidateHashs 按时间窗口快速筛选候选指纹。
func buildConversationsIndexer(taskFPDir string) (*conversationsIndexer, error) {
	idx := &conversationsIndexer{
		groups: make(map[groupKey]groupIndexer),
	}

	// 目录不存在时返回空索引，不报错（兼容增量分析场景）
	if _, err := os.Stat(taskFPDir); os.IsNotExist(err) {
		return idx, nil
	}

	// 第一步：递归扫描所有 .silica.json 文件
	var silicaFiles []string
	err := filepath.Walk(taskFPDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".silica.json") {
			silicaFiles = append(silicaFiles, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("扫描task silica目录失败: %w", err)
	}

	// 第二步：逐个解析文件并构建索引
	var hashCount int
	var skipCount int
	var convCount int
	var missRepoCount int
	var missStartTimeCount int
	var missEndTimeCount int
	for i, silicaFile := range silicaFiles {
		logPromptProgress(i, 50)

		// 加载单个task的silica数据文件
		ss, err := loadTaskSilicaFile(silicaFile)
		if err != nil {
			logWarnf("读取task silica文件失败 [%s]: %v", silicaFile, err)
			skipCount++
			continue
		}
		// 跳过缺少关键字段的文件，避免索引中出现无效分组
		if ss.SessionId == "" {
			logWarnf("文件[%s]缺失字段[task_id]", silicaFile)
			skipCount++
			continue
		}

		session := &sessionMeta{
			sessionId: ss.SessionId,
		}

		// 第三步：遍历该task下的所有对话，将指纹注册到分组索引中
		for i, conv := range ss.Conversations {
			if conv.RequestId == "" {
				logWarnf("文件[%s]对话[%d]缺失字段[request_id]", silicaFile, i)
				continue
			}
			if conv.RepoAddr == "" {
				missRepoCount++
				continue
			}
			// 解析对话结束时间，用于后续的时间窗口筛选和冲突仲裁
			var startTime, endTime time.Time
			if conv.EndTime != "" {
				endTime, _ = time.Parse(time.RFC3339, conv.EndTime)
			} else {
				missStartTimeCount++
			}
			if conv.StartTime != "" {
				startTime, _ = time.Parse(time.RFC3339, conv.StartTime)
			} else {
				missEndTimeCount++
			}

			// 按 repoAddr + userID 确定分组键
			gk := groupKey{repoAddr: conv.RepoAddr, userID: ss.UserId}

			// 获取或初始化该分组索引器
			gi, exists := idx.groups[gk]
			if !exists {
				gi = groupIndexer{
					Lines:    make(map[string]*convMeta),
					Sessions: make(map[string]*sessionMeta),
				}
				idx.groups[gk] = gi
			}
			gi.Sessions[ss.SessionId] = session

			// 创建对话元数据对象
			cm := &convMeta{
				sessionId:      ss.SessionId,
				requestId:      conv.RequestId,
				DiffLines:      len(conv.Fingerprints),
				UserInputChars: conv.UserInputChars,
				startTime:      startTime,
				endTime:        endTime,
			}
			session.conversations = append(session.conversations, cm)

			gi.Sessions[ss.SessionId] = session

			// 第四步：注册该对话产生的所有代码行指纹
			for _, h := range conv.Fingerprints {
				prevConv, prevExists := gi.Lines[h]
				// 冲突仲裁：如果该指纹已存在，仅当当前对话的 endTime 更晚时才替换归属
				// 这确保“最接近commit”的对话优先获得指纹归属权
				if !prevExists || endTime.After(prevConv.endTime) {
					gi.Lines[h] = cm
					if !prevExists {
						hashCount++ // 仅当新增唯一指纹时计数
					}
				}
			}
		}
		convCount += len(ss.Conversations)
	}
	logInfof("加载[%d]个对话文件，忽略其中[%d]个文件", len(silicaFiles), skipCount)
	logInfof("共[%d]组对话,缺失repo_addr[%d]个,缺失start_time[%d]个,缺失end_time[%d]个",
		convCount, missRepoCount, missStartTimeCount, missStartTimeCount)
	logInfof("共[%d]个分组, [%d]个唯一哈希", len(idx.groups), hashCount)
	return idx, nil
}

// loadTaskSilicaFile 读取并解析单个task的silica数据文件。
//
// 参数:
//   - path: .silica.json 文件的完整路径。
//
// 返回值:
//   - *taskSilicaData: 解析后的数据结构。
//   - error: 文件读取或JSON解析错误。
func loadTaskSilicaFile(path string) (*taskSilicaData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var ss taskSilicaData
	if err := json.Unmarshal(data, &ss); err != nil {
		return nil, err
	}
	return &ss, nil
}

// buildCandidateHashs 根据分组键和commit时间，构建候选指纹索引。
//
// 参数:
//   - gk: 分组键（repoAddr + userID），用于定位对应的分组数据。
//   - commitTime: commit的提交时间。
//   - maxDays: 最大时间窗口（天数），仅选取在commit提交前 maxDays 天内结束的对话。
//
// 返回值:
//   - map[string]*convMeta: 符合条件的指纹到对话元数据的映射。
//   - nil: 如果分组不存在或无符合条件的对话。
//
// 关键技术原理:
//
//	采用“时间窗口过滤”策略。一个对话结束后，其生成的代码可能在短期内被开发者采纳并提交。
//	超过 maxDays 的对话被认为与当前commit的关联性过低，予以排除。
//	先筛选出符合条件的对话索引集合，再遍历 Lines 映射过滤出属于这些对话的指纹。
func (gi *groupIndexer) buildCandidateHashs(commitTime time.Time, maxDays int) map[string]*convMeta {
	// 计算允许的最大时间差：maxDays 转换为 Duration
	maxDuration := time.Duration(maxDays) * 24 * time.Hour
	hashs := make(map[string]*convMeta)
	for i, m := range gi.Lines {
		if m.endTime.IsZero() {
			continue // 跳过无结束时间的对话（无法判断时间关联性）
		}
		// 对话必须在commit之前结束，且与commit的时间差不超过maxDuration
		if m.endTime.Before(commitTime) && commitTime.Sub(m.endTime) <= maxDuration {
			hashs[i] = m
		}
	}
	return hashs
}

// loadFPHashes 从指纹文件中逐行读取所有非空指纹。
//
// 参数:
//   - fpPath: 指纹文件路径，预期每行一个hash字符串。
//
// 返回值:
//   - []string: 指纹字符串切片。
//   - error: 文件打开或读取错误。
func loadFPHashes(fpPath string) ([]string, error) {
	f, err := os.Open(fpPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var hashes []string
	scanner := bufio.NewScanner(f)
	// 扩大扫描缓冲区以支持较大的指纹文件（最大10MB行）
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue // 跳过空行
		}
		hashes = append(hashes, line)
	}
	return hashes, scanner.Err()
}

// scanCommitFPFiles 递归扫描仓库指纹目录下的所有 .fp 文件。
//
// 参数:
//   - repoFPDir: 仓库指纹文件根目录。
//
// 返回值:
//   - []string: .fp 文件的完整路径列表。
//   - error: 目录遍历过程中的错误。
func scanCommitFPFiles(repoFPDir string) ([]string, error) {
	var files []string
	if _, err := os.Stat(repoFPDir); os.IsNotExist(err) {
		return files, nil // 目录不存在时返回空列表，不报错
	}

	err := filepath.Walk(repoFPDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".fp") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

type sessionConvs struct {
	SessionId  string
	convs      []*convMeta
	startTime  time.Time
	endTime    time.Time
	matchLines int
}

// computeCommitSilica 构建对该Commit产生贡献的Task，并计算每个Task的代码贡献率(即含硅量silica),以及Commit的总含硅量。
//
// 参数:
//   - gi: 和该Commit相关的分组索引
//   - hashs: 候选指纹索引，键为指纹，值为生成该指纹的对话元数据。
//
// 定义:
//
//	Task: 同一个会话(Session)下的一组为该提交(Commit)贡献了代码的对话(Conversation)
//
// 关键技术原理:
//
// 含硅量定义为：commit中与AI对话指纹匹配的代码行所占的比例。
// 代码采纳率定义为：commit中匹配的某task代码行占该task总代码行的比例。
// 具体计算步骤：
//  1. 遍历commit的所有指纹，统计在 hashs 中存在的匹配行数，并对匹配到的convMeta按sessionId分组，同一个会话为一组,保存在map[string]sessionConvs中，sessionConvs记录一个会话的所有对话convMeta
//  2. 按convMeta聚合匹配行数，实现对话级精确归因。该匹配行数也累加到convMeta所在的sessionConvs中。
//  3. 将一个会话匹配到的所有convMeta按EndTime排序，得到最早一个convMeta(firstConv)和最晚一个convMeta(lastConv).
//  4. 从session中获取从firstConv到lastConv之间的所有对话，构建一个models.Task（newTask），
//     newTask的task_id是随机生成的uuid，设这些对话的TaskId为newTask的TaskId(即这些对话从属于newTask)
//  5. 计算每个models.Task的各项数据：
//     含硅量(Silica) = 该session下所有对话匹配行数 / commit总行数。
//     代码接受率(AcceptRatio)=该task下所有对话匹配行数 / 该task总代码行数
//     TaskRealMinutes(调用calcTaskRealMinutes)
//     TaskAncientMinutes(调用estimateTaskAncientMinutes)，
//     开始时间，结束时间等
//  6. Commit的总含硅量 totalSilica 为各task含硅量之和，理论上限为1.0（100%）。
//
// 注意：
//
//	同一task可能包含多个对话，这些对话的匹配行数会合并到该task的silica中。
func (p *commitParser) computeCommitSilica(gi *groupIndexer, hashs map[string]*convMeta) ([]models.Task, map[string][]string) {
	p.totalLines = len(p.fpHashs)
	matchedLines := 0

	// 第一步：遍历commit的所有指纹，统计在hashs中存在的匹配行数
	// convMatchedLines 用于按对话维度聚合匹配行数，键格式为 "sessionId|requestId"
	convMatchedLines := make(map[string]int)
	// sessionConvsMap 记录每个session的匹配情况
	sessionConvsMap := make(map[string]*sessionConvs)

	for _, hash := range p.fpHashs {
		if cm, ok := hashs[hash]; ok {
			key := cm.sessionId + "|" + cm.requestId
			convMatchedLines[key]++
			matchedLines++

			sc, exists := sessionConvsMap[cm.sessionId]
			if !exists {
				sc = &sessionConvs{
					SessionId: cm.sessionId,
				}
				sessionConvsMap[cm.sessionId] = sc
			}
			sc.matchLines++
		}
	}

	p.totalMatchLines = matchedLines
	p.taskIds = make([]string, 0)
	p.taskSilicas = make([]float64, 0)
	p.taskAcceptRatios = make([]float64, 0)
	p.totalSilica = 0

	// 无匹配或commit为空时直接返回零值
	if p.totalLines == 0 || len(sessionConvsMap) == 0 {
		return nil, nil
	}

	var tasks []models.Task
	// taskConvMap 记录每个task包含的conversation request_id列表，用于后续更新conversations表的task_id
	taskConvMap := make(map[string][]string)

	// 遍历每个session，构建Task
	for sessionId, sc := range sessionConvsMap {
		session, ok := gi.Sessions[sessionId]
		if !ok {
			continue
		}

		// 收集该session下所有匹配到的convMeta
		var matchedConvs []*convMeta
		for _, cm := range session.conversations {
			key := cm.sessionId + "|" + cm.requestId
			if _, ok := convMatchedLines[key]; ok {
				matchedConvs = append(matchedConvs, cm)
			}
		}

		if len(matchedConvs) == 0 {
			continue
		}

		// 第三步：将匹配到的convMeta按EndTime排序
		sort.Slice(matchedConvs, func(i, j int) bool {
			return matchedConvs[i].endTime.Before(matchedConvs[j].endTime)
		})

		firstConv := matchedConvs[0]
		lastConv := matchedConvs[len(matchedConvs)-1]

		// 第四步：从session中获取从firstConv到lastConv之间的所有对话
		var validTimes []time.Time
		var taskTotalLines int64
		var taskTotalInchars int
		var taskMatchLines int
		var taskRequestIds []string

		for _, cm := range session.conversations {
			if (!cm.endTime.Before(firstConv.endTime) && !cm.endTime.After(lastConv.endTime)) ||
				cm.endTime.Equal(firstConv.endTime) || cm.endTime.Equal(lastConv.endTime) {
				taskRequestIds = append(taskRequestIds, cm.requestId)
				if !cm.startTime.IsZero() {
					validTimes = append(validTimes, cm.startTime)
				}
				taskTotalLines += int64(cm.DiffLines)
				taskTotalInchars += cm.UserInputChars

				key := cm.sessionId + "|" + cm.requestId
				if lines, ok := convMatchedLines[key]; ok {
					taskMatchLines += lines
				}
			}
		}

		// task_id 由 session_id + commit_id 确定，保证同一组合始终对应同一条记录
		taskId := sessionId + "|" + p.commitId

		// 第五步：计算TaskRealMinutes和TaskAncientMinutes
		realMinutes, realReason := calcTaskRealMinutes(validTimes, cfg.TaskStatistics.GapThresholdMinutes, cfg.TaskStatistics.ExtensionMinutes)
		ancientMinutes, ancientReason := estimateTaskAncientMinutes(&cfg.AlgoEstimation, float64(taskTotalInchars), float64(taskTotalLines), realMinutes)

		// 构建models.Task
		task := models.Task{
			TaskId:             taskId,
			CommitId:           p.commitId,
			SessionId:          sessionId,
			UserName:           "",
			ClientId:           "",
			ClientIde:          "",
			StartTime:          firstConv.startTime,
			EndTime:            lastConv.endTime,
			DiffLines:          int(taskTotalLines),
			Silica:             float64(sc.matchLines) / float64(p.totalLines),
			TaskRealMinutes:    realMinutes,
			TaskRealReason:     realReason,
			TaskAncientMinutes: ancientMinutes,
			TaskAncientReason:  ancientReason,
		}

		if taskTotalLines > 0 {
			task.AcceptRatio = float64(taskMatchLines) / float64(taskTotalLines)
		}

		tasks = append(tasks, task)
		taskConvMap[taskId] = taskRequestIds

		p.taskIds = append(p.taskIds, taskId)
		p.taskSilicas = append(p.taskSilicas, task.Silica)
		p.taskAcceptRatios = append(p.taskAcceptRatios, task.AcceptRatio)
		p.totalSilica += task.Silica
	}

	return tasks, taskConvMap
}

// 参数:
//   - db: GORM数据库连接，用于查询task表中的原始数据。
//
// 返回值:
//   - error: 数据库查询过程中的错误（当前实现中关键错误仅记录日志，不阻断流程）。
//
// 关键技术原理:
//  1. AI耗时（aiMinutes）: 累加各匹配task的 task_real_minutes（优先取手动修正值）。
//     这基于一个假设：commit中匹配到的AI代码所对应的开发耗时，可近似由各task的实际耗时按silica比例分摊。
//  2. 远古耗时（ancientMinutes）: 对于未匹配到的代码行（ancientLines = totalLines * (1 - totalSilica)），
//     调用 estimateCommitAncientMinutes 基于代码行数进行经验估算。
//  3. 总实际耗时（realMinutes）= aiMinutes + ancientMinutes，代表该commit的完整人力成本估算。
//  4. Token与Cost：直接累加各匹配task的 upstream_tokens、downstream_tokens 和 cost。
func (p *commitParser) calcCommitDerivedMinutes(tasks []models.Task) error {
	// 遍历所有匹配到的task，累加AI相关指标
	for _, t := range tasks {
		p.aiMinutes += t.TaskRealMinutes
		p.upstreamTokens += t.UpstreamTokens
		p.downstreamTokens += t.DownstreamTokens
		p.cost += t.Cost
	}

	// 计算非AI代码行数（传统编码），基于总含硅量的反比
	ancientLines := int((1 - p.totalSilica) * float64(p.totalLines))

	// 估算传统编码的开发耗时
	var ancientReason string
	p.ancientMinutes = 0
	if ancientLines > 0 {
		p.ancientMinutes, ancientReason = estimateCommitAncientMinutes(ancientLines)
	} else {
		ancientReason = "纯AI生成" // 全部代码均匹配AI指纹
	}

	// 汇总总实际耗时并生成计算说明文本
	p.realMinutes = p.aiMinutes + p.ancientMinutes
	p.realReason = fmt.Sprintf("AI耗时%.1f分钟 + 剩余代码%.1f分钟(%s) = %.1f分钟", p.aiMinutes, p.ancientMinutes, ancientReason, p.realMinutes)

	return nil
}

func saveCommit(db *gorm.DB, commitPs *commitParser) error {
	// 将taskID列表、含硅量列表和代码采纳率列表序列化为JSON字符串
	taskIdsJSON, _ := json.Marshal(commitPs.taskIds)
	silicaJSON, _ := json.Marshal(commitPs.taskSilicas)
	acceptRatiosJSON, _ := json.Marshal(commitPs.taskAcceptRatios)

	// 构建Commit对象
	commit := models.Commit{
		CommitId:                 commitPs.commitId,
		TaskIds:                  models.StringJSON(taskIdsJSON),
		TaskIdsSilica:            models.StringJSON(silicaJSON),
		TaskAcceptRatios:         models.StringJSON(acceptRatiosJSON),
		Silica:                   &commitPs.totalSilica,
		CommitRealAiMinutes:      &commitPs.aiMinutes,
		CommitRealAncientMinutes: &commitPs.ancientMinutes,
		CommitRealMinutes:        &commitPs.realMinutes,
		CommitRealReason:         commitPs.realReason,
		UpstreamTokens:           &commitPs.upstreamTokens,
		DownstreamTokens:         &commitPs.downstreamTokens,
		Cost:                     &commitPs.cost,
	}

	// Upsert写入数据表
	if err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "commit_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"task_ids", "task_ids_silica", "task_accept_ratios", "silica", "commit_real_ai_minutes", "commit_real_ancient_minutes", "commit_real_minutes", "commit_real_reason", "upstream_tokens", "downstream_tokens", "cost"}),
	}).Create(&commit).Error; err != nil {
		logWarnf("保存commits表失败 [%s]: %v", commitPs.commitId, err)
		return err
	}

	// 调试日志：输出该commit的核心计算结果
	logDebugf("  %s: silica=%.4f (%d/%d行匹配), ai=%.1fmin, ancient=%.1fmin, total=%.1fmin",
		commitPs.commitId, commitPs.totalSilica, commitPs.totalMatchLines, commitPs.totalLines, commitPs.aiMinutes, commitPs.ancientMinutes, commitPs.realMinutes)
	return nil
}

// buildCommitTasks 为单个Commit构建关联Task，计算含硅量及相关衍生指标，并更新数据库。
func buildCommitTasks(db *gorm.DB, commitFpFile string, idx *conversationsIndexer, force bool, maxDays int, startDate, endDate *time.Time) string {
	// 从文件名中提取commitId（去掉 .fp 后缀）
	commitId := strings.TrimSuffix(filepath.Base(commitFpFile), ".fp")

	// 非强制模式下，跳过已计算过含硅量的commit
	if !force {
		var commit models.Commit
		if err := db.Select("task_ids").Where("commit_id = ?", commitId).First(&commit).Error; err == nil {
			// 如果task_ids已存在且非空（非 "null" 或 "[]"），则视为已处理
			if string(commit.TaskIds) != "" && string(commit.TaskIds) != "null" && string(commit.TaskIds) != "[]" {
				return "skip"
			}
		}
	}

	// 加载commit指纹
	commitPs := buildCommitParser(commitId, commitFpFile)
	if commitPs == nil {
		logWarnf("读取commit指纹文件失败 [%s]", commitFpFile)
		return "skip"
	}

	// 查询commit的元数据（仓库地址、用户ID、提交时间）
	var commit models.Commit
	if err := db.Select("repo_addr, user_id, commit_time").Where("commit_id = ?", commitId).First(&commit).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			logWarnf("commit不存在于数据库 [%s]", commitId)
			return "skip"
		}
		logWarnf("查询commit元数据失败 [%s]: %v", commitId, err)
		return "fail"
	}
	if commit.CommitTime.IsZero() {
		logErrorf("Commit [%s] 缺少提交时间", commitId)
		return "skip"
	}

	// 日期范围过滤
	if !isActiveTimeInRange(commit.CommitTime, startDate, endDate) {
		logDebugf("跳过(日期范围过滤): %s", commitId)
		return "skip"
	}

	// 使用commit的仓库地址和用户ID构建分组键
	gk := groupKey{repoAddr: commit.RepoAddr, userID: commit.UserId}
	gi, exists := idx.groups[gk]
	if !exists {
		return "skip"
	}

	// 在时间窗口内筛选候选指纹，并计算含硅量
	candidateHashes := gi.buildCandidateHashs(commit.CommitTime, maxDays)
	tasks, taskConvMap := commitPs.computeCommitSilica(&gi, candidateHashes)

	// 保存计算出的tasks到数据库，按 task_id 做 upsert
	if len(tasks) > 0 {
		if err := db.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "task_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"user_id", "user_name", "client_id", "client_ide",
				"client_version", "client_os", "client_os_version", "caller",
				"repo_addr", "repo_branch", "work_dir", "work_dir_id",
				"start_time", "end_time", "diff_lines", "silica", "accept_ratio",
				"upstream_tokens", "downstream_tokens", "cost",
				"task_real_minutes", "task_real_reason",
				"task_ancient_minutes", "task_ancient_reason",
				"title",
			}),
		}).Create(&tasks).Error; err != nil {
			logWarnf("批量upsert task失败: %v", err)
		}
	}

	// 更新属于该task的所有conversation记录，设置task_id
	for _, task := range tasks {
		if requestIds, ok := taskConvMap[task.TaskId]; ok && len(requestIds) > 0 {
			if err := db.Model(&models.TaskConversation{}).
				Where("session_id = ? AND request_id IN ?", task.SessionId, requestIds).
				Update("task_id", task.TaskId).Error; err != nil {
				logWarnf("更新conversation的task_id失败 [%s]: %v", task.TaskId, err)
			}
		}
	}
	// 计算衍生指标：AI耗时、传统耗时、token、cost等
	if err := commitPs.calcCommitDerivedMinutes(tasks); err != nil {
		logWarnf("计算衍生数据失败 [%s]: %v", commitId, err)
		return "fail"
	}
	if err := saveCommit(db, commitPs); err != nil {
		return "fail"
	}
	return "success"
}

// runSilica 执行含硅量计算的主流程。
//
// 参数:
//   - analysedDir: 已分析数据的根目录，预期包含 task/conversation/ 和 repo/ 子目录。
//   - force: 是否强制重新计算。为false时跳过已存在task_ids的commit。
//   - maxDays: 对话与commit的最大关联时间窗口（天数）。
//
// 返回值:
//   - error: 流程中断错误（如目录不存在、数据库连接失败等）。
//
// 主流程说明:
//  1. 构建全局conversation指纹索引。
//  2. 扫描所有commit的 .fp 指纹文件。
//  3. 对每个commit：查询数据库元数据 -> 筛选候选指纹 -> 计算含硅量 -> 计算衍生指标 -> 更新数据库。
//  4. 汇总统计成功/跳过/失败的数量。
func runSilica(analysedDir string, force bool, maxDays int, startDateStr, endDateStr, dateStr string) error {
	startTime := time.Now()
	// 构建task指纹目录和仓库指纹目录的路径
	taskFPDir := filepath.Join(analysedDir, "task", "conversation")
	repoFPDir := filepath.Join(analysedDir, "repo")

	// 前置校验：task指纹目录必须存在
	if _, err := os.Stat(taskFPDir); os.IsNotExist(err) {
		recordCommandRun("silica", startTime, 0, 0, 0, err)
		return fmt.Errorf("task指纹目录不存在: %s", taskFPDir)
	}

	// 解析日期范围
	startDate, endDate, err := parseDateRange(startDateStr, endDateStr, dateStr)
	if err != nil {
		recordCommandRun("silica", startTime, 0, 0, 0, err)
		return err
	}

	// 建立数据库连接
	db, err := models.OpenGormDB(cfg.StatDatabase.DSN())
	if err != nil {
		recordCommandRun("silica", startTime, 0, 0, 0, err)
		return fmt.Errorf("连接数据库失败: %w", err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close() // 确保函数退出时关闭数据库连接

	// 构建conversation指纹索引
	idx, err := buildConversationsIndexer(taskFPDir)
	if err != nil {
		recordCommandRun("silica", startTime, 0, 0, 0, err)
		return fmt.Errorf("构建conversation指纹索引失败: %w", err)
	}

	// 扫描所有commit指纹文件
	commitFPFiles, err := scanCommitFPFiles(repoFPDir)
	if err != nil {
		recordCommandRun("silica", startTime, 0, 0, 0, err)
		return fmt.Errorf("扫描commit指纹文件失败: %w", err)
	}
	logInfof("找到%d个commit指纹文件", len(commitFPFiles))

	// 统计计数器
	successCount := 0
	skipCount := 0
	failCount := 0

	// 逐文件处理每个commit
	for i, commitFpFile := range commitFPFiles {
		logPromptProgress(i, 50) // 每50个文件打印进度

		status := buildCommitTasks(db, commitFpFile, idx, force, maxDays, startDate, endDate)
		switch status {
		case "success":
			successCount++
		case "skip":
			skipCount++
		case "fail":
			failCount++
		}
	}

	// 打印最终统计并记录命令执行历史
	logInfof("含硅量计算完成: 成功 %d 个，跳过 %d 个，失败 %d 个", successCount, skipCount, failCount)
	recordCommandRun("silica", startTime, successCount, failCount, skipCount, nil)
	return nil
}

// silicaCmd 定义了 "silica" 子命令的Cobra命令对象。
// 功能：计算commit的含硅量（task贡献比例），并更新commits表的task_ids和task_ids_silica字段。
var silicaCmd = &cobra.Command{
	Use:   "silica",
	Short: "计算commit的含硅量（task贡献比例），更新commits表的task_ids和task_ids_silica",
	RunE: func(cmd *cobra.Command, args []string) error {
		// 解析命令行参数
		analysedDir, _ := cmd.Flags().GetString("analysed-dir")
		force, _ := cmd.Flags().GetBool("force")
		maxDays, _ := cmd.Flags().GetInt("max-days")
		remote, _ := cmd.Flags().GetString("remote")
		startDate, _ := cmd.Flags().GetString("start-date")
		endDate, _ := cmd.Flags().GetString("end-date")
		date, _ := cmd.Flags().GetString("date")

		// 如果指定了remote地址，将命令转发到远程kbcli服务执行
		if remote != "" {
			return sendToRemote(remote, "silica", map[string]interface{}{
				"analysed_dir": analysedDir,
				"force":        force,
				"max_days":     maxDays,
				"start_date":   startDate,
				"end_date":     endDate,
				"date":         date,
			})
		}

		// 参数默认值处理：空值时从全局配置读取
		if analysedDir == "" {
			analysedDir = cfg.AnalysedDir
		}
		if maxDays <= 0 {
			maxDays = cfg.SilicaMaxDays
		}

		return runSilica(analysedDir, force, maxDays, startDate, endDate, date)
	},
}

// init 注册silica命令及其命令行标志到rootCmd。
func init() {
	silicaCmd.Flags().SortFlags = false // 保持标志定义顺序，便于文档生成
	silicaCmd.Flags().String("analysed-dir", "", "已分析文件目录路径")
	silicaCmd.Flags().BoolP("force", "f", false, "强制重新计算，覆盖已存在数据")
	silicaCmd.Flags().String("start-date", "", "限定起始日期，格式 YYYYMMDD，为空则不限")
	silicaCmd.Flags().String("end-date", "", "限定结束日期，格式 YYYYMMDD，为空则不限")
	silicaCmd.Flags().String("date", "", "限定日期，格式 YYYYMMDD，限定活跃时间在该日期之内（与start-date/end-date互斥）")
	silicaCmd.Flags().Int("max-days", 0, "对话结束后多少天内的commit算相关（默认从config读取）")
	silicaCmd.Flags().String("remote", "", "远程kbcli服务地址（如 http://127.0.0.1:8080），指定后命令将发送到远程执行")
	rootCmd.AddCommand(silicaCmd)
}
