package main

import (
	"encoding/json"
	"fmt"
	"kanban/core/models"
	"kanban/core/storage"
	"kanban/kbcli/internal/appconfig"
	"kanban/kbcli/internal/governance"
	"kanban/kbcli/internal/logx"
	"path"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
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

type convKey struct {
	sessionId string // 所属的session
	requestId string // 对话请求ID，在一个session中，是唯一的，标识一次AI请求
}

type convMatched struct {
	conv       *convMeta
	matchLines int
}

type taskMatched struct {
	sessionId    string
	matchLines   int
	totalLines   int64
	totalInchars int
	startTime    time.Time
	endTime      time.Time
	silica       float64
	acceptRatio  float64
	task         *models.Task
	convs        []convMatched
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
	Sessions map[string]*sessionMeta // sessionId -> sessionMeta对象
}

// conversationsIndexer 全局conversation指纹索引，聚合所有分组的数据。
// 用于快速判断某行代码是否由某次AI对话生成。
type conversationsIndexer struct {
	groups   map[groupKey]groupIndexer // 主分组索引，键为 (repoAddr, userID)
	wdGroups map[wdKey]groupIndexer    // 后备分组索引，键为 (work_dir_id, userID)，仅收录缺 repo_addr 的对话
}

// wdKey 是 work_dir_id 后备分组键。保留 userID 维度，避免不同用户因 work_dir_id 碰撞
// （work_dir_id = clientID[:6]+workDir，clientID[:6] 可能是共享前缀）而跨用户误归组——
// 与主分组 (repoAddr,userID) 一样防串户。
type wdKey struct {
	workDirId string
	userID    string
}

// newGroupIndexer 创建空的分组索引器。
func newGroupIndexer() groupIndexer {
	return groupIndexer{
		Lines:    make(map[string]*convMeta),
		Sessions: make(map[string]*sessionMeta),
	}
}

// lookupGroups 为某 commit 取出可匹配的对话分组：主分组 (repoAddr,userID) 与
// work_dir_id 后备分组。两者都存在时合并为一个临时索引（主分组指纹优先，后备仅补充）。
// 后备分组让缺 repo_addr 的对话也能经 work_dir_id 与 commit 匹配。
func (idx *conversationsIndexer) lookupGroups(repoAddr, userId, workDirId string) (groupIndexer, bool) {
	main, hasMain := idx.groups[groupKey{repoAddr: repoAddr, userID: userId}]
	var fb groupIndexer
	hasFb := false
	if workDirId != "" {
		fb, hasFb = idx.wdGroups[wdKey{workDirId: workDirId, userID: userId}]
	}
	switch {
	case !hasMain && !hasFb:
		return groupIndexer{}, false
	case hasMain && !hasFb:
		return main, true
	case !hasMain && hasFb:
		return fb, true
	}
	// 两者都有：合并（主分组优先占据指纹归属，后备仅填补主分组没有的指纹）
	merged := newGroupIndexer()
	for h, cm := range main.Lines {
		merged.Lines[h] = cm
	}
	for h, cm := range fb.Lines {
		if _, ok := merged.Lines[h]; !ok {
			merged.Lines[h] = cm
		}
	}
	for sid, s := range fb.Sessions {
		merged.Sessions[sid] = s
	}
	for sid, s := range main.Sessions {
		merged.Sessions[sid] = s
	}
	return merged, true
}

// commitParser 用于解析单个commit的指纹文件并计算含硅量及相关衍生指标。
type commitParser struct {
	commitId         string    // commit的唯一ID
	commitTime       time.Time // commit的提交时间
	fpHashs          []string  // commit指纹文件中的全部代码行指纹
	taskIds          []string  // 匹配的taskId列表（去重后）
	taskSilicas      []float64 // 每个task对当前commit的含硅量贡献值
	taskAcceptRatios []float64 // 每个task被当前commit采纳的代码行占该task代码行的比例
	totalLines       int       // commit总行数（指纹总数）
	totalMatchLines  int       // 与AI对话指纹匹配的行数
	totalSilica      float64   // 总含硅量（所有task贡献值之和）
	aiMinutes        float64   // AI生成代码对应的实际耗时（分钟）
	nonAiMinutes     float64   // 非AI代码（传统编码）的估算耗时（分钟）
	realMinutes      float64   // 总实际耗时 = aiMinutes + nonAiMinutes
	realReason       string    // 实际耗时的计算说明文本
	ancientMinutes   float64   // 估算的传统方法工作量
	ancientReason    string    // 估算的传统方法工作量的估算说明
	upstreamTokens   int64     // 上游token消耗总数
	downstreamTokens int64     // 下游token消耗总数
	cost             float64   // AI调用成本总数
}

// buildCommitParser 从指纹列表构建 commitParser 实例。
//
// 参数:
//   - commitId: commit 的唯一 ID。
//   - fpHashes: 过滤护栏后的匹配指纹集（仅用于匹配，可能少于原始行数）。
//   - totalLines: commit 原始新增行数（silica 分母 + ancient 估算用），与护栏无关，
//     保证 silica 仍是 "AI 占整个 commit 的比例"，不因护栏剔除样板行而被动抬高。
func buildCommitParser(commitId string, fpHashes []string, totalLines int) *commitParser {
	return &commitParser{
		commitId:   commitId,
		fpHashs:    fpHashes,
		totalLines: totalLines,
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
func buildConversationsIndexer(db *gorm.DB, analysedDir string) (*conversationsIndexer, error) {
	idx := &conversationsIndexer{
		groups:   make(map[groupKey]groupIndexer),
		wdGroups: make(map[wdKey]groupIndexer),
	}

	// 第一步：从 PostgreSQL 索引枚举对象。S3 模式严禁回退到目录扫描，
	// 因为线上对象存储只支持按 key 读写、不支持 ListObjects。
	silicaFiles, err := indexedTaskSilicaFiles(db, analysedDir)
	if err != nil {
		return nil, err
	}

	// 第二步：逐个解析文件并构建索引
	var hashCount int
	var skipCount int
	var convCount int
	var missRepoCount int
	var missStartTimeCount int
	var missEndTimeCount int
	for i, silicaFile := range silicaFiles {
		logx.PromptProgress(i, 50)

		// 加载单个task的silica数据文件
		ss, err := loadTaskSilicaFile(silicaFile)
		if err != nil {
			logx.Warnf("读取task silica文件失败 [%s]: %v", silicaFile, err)
			skipCount++
			continue
		}
		// 跳过缺少关键字段的文件，避免索引中出现无效分组
		if ss.SessionId == "" {
			logx.Warnf("文件[%s]缺失字段[task_id]", silicaFile)
			skipCount++
			continue
		}

		session := &sessionMeta{
			sessionId: ss.SessionId,
		}

		// 第三步：遍历该task下的所有对话，将指纹注册到分组索引中
		for i, conv := range ss.Conversations {
			if conv.RequestId == "" {
				logx.Warnf("文件[%s]对话[%d]缺失字段[request_id]", silicaFile, i)
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

			// 选择分组：有 repo_addr 走主分组 (repoAddr,userID)；缺 repo_addr 时改用 work_dir_id
			// 后备分组，让这部分对话也能与 commit 匹配；两者皆无则无法定位分组，跳过。
			// repo_addr 过 SanitizeRepoAddr：凭据脱敏修复前生成的旧 .silica.json 中 repo_addr
			// 仍带 userinfo，而 commit 侧已入库即脱敏，不清洗会导致 groupKey 精确匹配失配
			// （带凭据仓库的 silica 降级）。幂等，新文件不受影响。
			var gi groupIndexer
			var exists bool
			if conv.RepoAddr != "" {
				gk := groupKey{repoAddr: governance.SanitizeRepoAddr(conv.RepoAddr), userID: ss.UserId}
				gi, exists = idx.groups[gk]
				if !exists {
					gi = newGroupIndexer()
					idx.groups[gk] = gi
				}
			} else if conv.WorkDirId != "" {
				missRepoCount++ // 缺 repo_addr，走 (work_dir_id, userID) 后备分组
				wk := wdKey{workDirId: conv.WorkDirId, userID: ss.UserId}
				gi, exists = idx.wdGroups[wk]
				if !exists {
					gi = newGroupIndexer()
					idx.wdGroups[wk] = gi
				}
			} else {
				missRepoCount++
				continue // 既无 repo_addr 又无 work_dir_id，无法定位分组
			}
			gi.Sessions[ss.SessionId] = session

			// 创建对话元数据对象。DiffLines 用原始新增行数（acceptRatio 分母），
			// 与护栏过滤后的 Fingerprints 数区分；旧 .silica.json 无此字段时回退到指纹数。
			diffLines := conv.DiffLines
			if diffLines == 0 {
				diffLines = len(conv.Fingerprints)
			}
			cm := &convMeta{
				sessionId:      ss.SessionId,
				requestId:      conv.RequestId,
				DiffLines:      diffLines,
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
	logx.Infof("加载[%d]个对话文件，忽略其中[%d]个文件", len(silicaFiles), skipCount)
	logx.Infof("共[%d]组对话,缺失repo_addr[%d]个,缺失start_time[%d]个,缺失end_time[%d]个",
		convCount, missRepoCount, missStartTimeCount, missStartTimeCount)
	logx.Infof("共[%d]个分组, [%d]个唯一哈希", len(idx.groups), hashCount)
	return idx, nil
}

func indexedTaskSilicaFiles(db *gorm.DB, analysedDir string) ([]string, error) {
	var rows []models.SilicaObjectIndex
	if err := db.Order("session_id ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("查询silica_object_index失败: %w", err)
	}
	if len(rows) > 0 {
		files := make([]string, 0, len(rows))
		for _, row := range rows {
			loc, err := indexedTaskSilicaLocation(analysedDir, row.ObjectPath)
			if err != nil {
				return nil, fmt.Errorf("silica_object_index session=%s: %w", row.SessionId, err)
			}
			files = append(files, loc)
		}
		logx.Infof("从silica_object_index加载[%d]个对象路径", len(files))
		return files, nil
	}

	if storage.IsS3(analysedDir) {
		logx.Warnf("silica_object_index为空，S3模式跳过目录枚举；请先运行import-conv生成对象索引")
		return nil, nil
	}

	// 本地旧部署可能已有 silica 文件但尚未生成数据库索引，保留一次兼容扫描。
	taskFPDir := storage.Join(analysedDir, "task", "conversation")
	if ok, err := storage.Exists(taskFPDir); err != nil {
		return nil, err
	} else if !ok {
		return nil, nil
	}
	var files []string
	if err := storage.Walk(taskFPDir, func(filePath string, info storage.FileInfo) error {
		if strings.HasSuffix(info.Name, ".silica.json") {
			files = append(files, filePath)
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("扫描本地task silica目录失败: %w", err)
	}
	sort.Strings(files)
	return files, nil
}

func indexedTaskSilicaLocation(analysedDir, objectPath string) (string, error) {
	rel := strings.TrimSpace(strings.ReplaceAll(objectPath, "\\", "/"))
	if rel == "" || storage.IsS3(rel) || path.IsAbs(rel) {
		return "", fmt.Errorf("object_path必须是analysed_dir下的相对路径: %q", objectPath)
	}
	rel = path.Clean(rel)
	if rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", fmt.Errorf("object_path越出analysed_dir: %q", objectPath)
	}
	if !strings.HasPrefix(rel, "task/conversation/") || !strings.HasSuffix(rel, ".silica.json") {
		return "", fmt.Errorf("object_path不是task silica路径: %q", objectPath)
	}
	return storage.Join(analysedDir, rel), nil
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
	data, err := storage.ReadFile(path)
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

// buildCommitTasks 构建对该Commit产生贡献的Task基础信息，仅做归因分析，不计算任何指标。
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
//  1. 遍历commit的所有指纹，统计在 hashs 中存在的匹配行数。按convMeta聚合匹配行数，实现对话级精确归因。
//  2. 按sessionId分组，将每个会话匹配到的信息汇总到taskMatched中。
//     taskMatched.convs 保存该task涉及的所有conversation及其匹配行数。
//  3. 将一个会话匹配到的所有convMeta按EndTime排序，得到最早一个convMeta(firstConv)和最晚一个convMeta(lastConv).
//  4. 从session中获取从firstConv到lastConv之间的所有对话，纳入taskMatched，填充基础统计字段。
//
// 注意：
//
//	同一task可能包含多个对话，这些对话的匹配行数会合并到该task的matchLines中。
//	具体的含硅量、采纳率、耗时等指标由 calcTaskMetrics 在后续阶段计算。
func (p *commitParser) buildCommitTasks(gi *groupIndexer, hashs map[string]*convMeta) []taskMatched {
	matchedLines := 0

	// 第一步：遍历commit指纹，按对话维度聚合匹配行数
	convMatchLines := make(map[convKey]int)
	taskMap := make(map[string]*taskMatched)

	for _, hash := range p.fpHashs {
		if cm, ok := hashs[hash]; ok {
			key := convKey{sessionId: cm.sessionId, requestId: cm.requestId}
			convMatchLines[key]++
			matchedLines++

			if taskMap[cm.sessionId] == nil {
				taskMap[cm.sessionId] = &taskMatched{sessionId: cm.sessionId}
			}
			taskMap[cm.sessionId].matchLines++
		}
	}

	if p.totalLines == 0 || matchedLines == 0 {
		return nil
	}

	var result []taskMatched

	// 第二步：遍历每个有匹配的session，构建taskMatched基础信息
	for sessionId, tm := range taskMap {
		session, ok := gi.Sessions[sessionId]
		if !ok {
			continue
		}

		// 收集匹配到的convMeta并排序，确定task时间范围
		var matchedConvs []*convMeta
		for _, cm := range session.conversations {
			key := convKey{sessionId: cm.sessionId, requestId: cm.requestId}
			if convMatchLines[key] > 0 {
				matchedConvs = append(matchedConvs, cm)
			}
		}
		if len(matchedConvs) == 0 {
			continue
		}

		sort.Slice(matchedConvs, func(i, j int) bool {
			return matchedConvs[i].endTime.Before(matchedConvs[j].endTime)
		})

		firstConv := matchedConvs[0]
		lastConv := matchedConvs[len(matchedConvs)-1]
		tm.startTime = firstConv.startTime
		tm.endTime = lastConv.endTime

		for _, cm := range session.conversations {
			if (cm.endTime.Before(firstConv.endTime) || cm.endTime.After(lastConv.endTime)) &&
				!cm.endTime.Equal(firstConv.endTime) && !cm.endTime.Equal(lastConv.endTime) {
				continue
			}

			key := convKey{sessionId: cm.sessionId, requestId: cm.requestId}
			lines := convMatchLines[key]

			tm.convs = append(tm.convs, convMatched{conv: cm, matchLines: lines})
			tm.totalLines += int64(cm.DiffLines)
			tm.totalInchars += cm.UserInputChars
		}

		if len(tm.convs) == 0 {
			continue
		}

		result = append(result, *tm)
	}

	return result
}

// calcTaskMetrics 计算每个task的含硅量、采纳率、耗时等指标，并构建models.Task对象。
//
// 参数:
//   - tms: buildCommitTasks 返回的taskMatched基础列表。
//   - commitId: 当前commit的ID，用于生成taskId。
//   - totalLines: commit的总代码行数，用于计算含硅量。
//
// 返回值:
//   - []taskMatched: 填充了指标和task对象的task列表。
func calcTaskMetrics(tms []taskMatched, commitId string, totalLines int) []taskMatched {
	for i := range tms {
		tm := &tms[i]

		tm.silica = float64(tm.matchLines) / float64(totalLines)
		if tm.totalLines > 0 {
			tm.acceptRatio = float64(tm.matchLines) / float64(tm.totalLines)
		}

		var validTimes []time.Time
		for _, c := range tm.convs {
			if !c.conv.startTime.IsZero() {
				validTimes = append(validTimes, c.conv.startTime)
			}
		}

		realMinutes, realReason := calcTaskRealMinutes(validTimes, appconfig.Cfg.TaskStatistics.GapThresholdMinutes, appconfig.Cfg.TaskStatistics.ExtensionMinutes)
		ancientMinutes, ancientReason := estimateTaskAncientMinutes(&appconfig.Cfg.AlgoEstimation, float64(tm.totalInchars), float64(tm.totalLines), realMinutes)

		taskId := tm.sessionId + "|" + commitId
		tm.task = &models.Task{
			TaskId:             taskId,
			CommitId:           commitId,
			SessionId:          tm.sessionId,
			UserName:           "",
			ClientId:           "",
			ClientIde:          "",
			StartTime:          tm.startTime,
			EndTime:            tm.endTime,
			DiffLines:          int(tm.totalLines),
			Silica:             tm.silica,
			AcceptRatio:        tm.acceptRatio,
			TaskRealMinutes:    realMinutes,
			TaskRealReason:     realReason,
			TaskAncientMinutes: ancientMinutes,
			TaskAncientReason:  ancientReason,
		}
	}
	return tms
}

// aggregateCommitTaskStats 聚合task指标到commitParser的统计字段中。
//
// 参数:
//   - tms: 已计算完指标的taskMatched列表。
func (p *commitParser) aggregateCommitTaskStats(tms []taskMatched) {
	p.taskIds = make([]string, 0, len(tms))
	p.taskSilicas = make([]float64, 0, len(tms))
	p.taskAcceptRatios = make([]float64, 0, len(tms))
	p.totalSilica = 0
	p.totalMatchLines = 0

	for _, tm := range tms {
		p.taskIds = append(p.taskIds, tm.task.TaskId)
		p.taskSilicas = append(p.taskSilicas, tm.silica)
		p.taskAcceptRatios = append(p.taskAcceptRatios, tm.acceptRatio)
		p.totalSilica += tm.silica
		p.totalMatchLines += tm.matchLines
	}
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
func (p *commitParser) calcCommitDerivedMinutes(tms []taskMatched) error {
	// 遍历所有匹配到的task，累加AI相关指标
	for _, tm := range tms {
		if tm.task == nil {
			continue
		}
		p.aiMinutes += tm.task.TaskRealMinutes
		p.upstreamTokens += tm.task.UpstreamTokens
		p.downstreamTokens += tm.task.DownstreamTokens
		p.cost += tm.task.Cost
	}

	// 计算非AI代码行数（传统编码），基于总含硅量的反比
	ancientLines := int((1 - p.totalSilica) * float64(p.totalLines))

	// 估算传统编码的开发耗时
	var ancientReason string
	p.nonAiMinutes = 0
	if ancientLines > 0 {
		p.nonAiMinutes, ancientReason = estimateCommitAncientMinutes(ancientLines)
	} else {
		ancientReason = "纯AI生成" // 全部代码均匹配AI指纹
	}

	// 汇总总实际耗时并生成计算说明文本
	p.realMinutes = p.aiMinutes + p.nonAiMinutes
	p.realReason = fmt.Sprintf("AI耗时%.1f分钟 + 剩余代码%.1f分钟(%s) = %.1f分钟", p.aiMinutes, p.nonAiMinutes, ancientReason, p.realMinutes)

	return nil
}

// analyzeCommitSilica 计算单个commit的含硅量及相关指标。
// 纯计算函数，不操作数据库，不读取文件。
func analyzeCommitSilica(
	commitId, repoAddr, userId, workDirId string,
	commitTime time.Time,
	fpHashes []string,
	totalDiffLines int,
	idx *conversationsIndexer,
	maxDays int,
) (*commitParser, []taskMatched) {
	p := buildCommitParser(commitId, fpHashes, totalDiffLines)

	// 主分组 (repoAddr,userID) + work_dir_id 后备分组，覆盖缺 repo_addr 的对话。
	gi, exists := idx.lookupGroups(repoAddr, userId, workDirId)
	if !exists {
		return p, nil
	}

	candidateHashes := gi.buildCandidateHashs(commitTime, maxDays)
	tms := p.buildCommitTasks(&gi, candidateHashes)
	tms = calcTaskMetrics(tms, p.commitId, p.totalLines)
	p.aggregateCommitTaskStats(tms)
	p.calcCommitDerivedMinutes(tms)
	p.ancientMinutes, p.ancientReason = estimateCommitAncientMinutes(totalDiffLines)

	return p, tms
}
