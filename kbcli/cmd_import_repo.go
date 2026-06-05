package main

import (
	"encoding/json"
	"fmt"
	"kanban/core/models"
	"kanban/core/utils"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/spf13/cobra"
)

// RepoCommitData 表示单个 commit JSON 文件的数据结构，对应客户端上报的代码提交信息
type RepoCommitData struct {
	CommitId     string   `json:"commit_id"`           // Git commit hash
	CommitTime   string   `json:"commit_time"`         // 提交时间，RFC3339 格式
	RepoAddr     string   `json:"repo_addr"`           // 仓库地址
	RepoBranch   string   `json:"repo_branch"`         // 分支名称
	GitUserName  string   `json:"git_user_name"`       // Git 用户名
	GitUserEmail string   `json:"git_user_email"`      // Git 邮箱
	UserId       string   `json:"user_id"`             // 系统用户ID
	UserName     string   `json:"user_name"`           // 系统用户名
	ClientId     string   `json:"client_id"`           // 客户端ID
	WorkPath     string   `json:"work_path,omitempty"` // 工作路径（旧字段，兼容用）
	WorkDir      string   `json:"work_dir,omitempty"`  // 工作目录（新字段）
	Comment      string   `json:"comment"`             // 提交注释
	DiffLines    int      `json:"diff_lines"`          // 新增代码行数
	Diff         string   `json:"diff"`                // 完整的 diff 文本
	Files        []string `json:"files,omitempty"`     // 本次提交涉及的文件路径
}

// repoFileMeta 记录扫描到的单个 commit JSON 文件的元信息，用于后续解析路径和去重
type repoFileMeta struct {
	Path     string // 文件绝对路径
	RelPath  string // 相对于 repoDir 的相对路径（统一使用正斜杠）
	Repo     string // 仓库名，从路径中提取
	Branch   string // 分支名，从路径中提取
	Year     string // 年，从路径中提取
	Month    string // 月，从路径中提取
	Day      string // 日，从路径中提取
	CommitId string // commit ID，从路径中提取（即文件名）
}

// reRepoPath 匹配 commit JSON 文件的相对路径格式: repo/branch/YYYY/MM/DD/commit_id.json
// 使用正则从路径中拆解出仓库、分支、日期和 commit ID，用于后续构造指纹文件路径
var reRepoPath = regexp.MustCompile(`^([^/]+)/([^/]+)/(\d{4})/(\d{2})/(\d{2})/([^/]+)\.json$`)

// scanRepoDir 递归扫描 repo 目录，收集所有待导入的 commit JSON 文件元信息。
//
// 参数:
//   - repoDir: 客户端上报数据的根目录，按 repo/branch/YYYY/MM/DD/*.json 层级存放
//   - db: 数据库连接，用于 force 模式下的存在性检查
//   - force: 是否强制重新导入；为 false 时会跳过数据库中已存在的 commit
//   - startDate, endDate: 日期范围过滤
//
// 返回值:
//   - []repoFileMeta: 待导入的文件元信息列表
//   - int: 因已处理或日期过滤而被跳过的文件数量
//   - error: 扫描过程中发生的错误
//
// 关键技术原理:
//  1. 使用 filepath.Walk 递归遍历目录树
//  2. 通过正则表达式 reRepoPath 校验并拆解文件路径，确保只有符合约定格式的文件才会被纳入
//  3. 日期过滤：根据文件路径中的日期进行范围判断
//  4. 幂等性控制：非 force 模式下，若数据库已有该 commit 记录，则直接跳过
func scanRepoDir(repoDir string, db *gorm.DB, force bool, startDate, endDate *time.Time) ([]repoFileMeta, int, error) {
	var files []repoFileMeta
	skipCount := 0

	err := filepath.Walk(repoDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if !strings.HasSuffix(info.Name(), ".json") {
			return nil
		}

		relPath, err := filepath.Rel(repoDir, path)
		if err != nil {
			return err
		}

		relPath = filepath.ToSlash(relPath)

		matches := reRepoPath.FindStringSubmatch(relPath)
		if matches == nil {
			return nil
		}

		meta := repoFileMeta{
			Path:     path,
			RelPath:  relPath,
			Repo:     matches[1],
			Branch:   matches[2],
			Year:     matches[3],
			Month:    matches[4],
			Day:      matches[5],
			CommitId: matches[6],
		}

		// 日期范围过滤：根据文件路径中的日期判断
		fileDate, err := time.Parse("20060102", meta.Year+meta.Month+meta.Day)
		if err != nil {
			return nil
		}
		if !isActiveTimeInRange(fileDate, startDate, endDate) {
			logDebugf("跳过(日期范围过滤): %s", path)
			skipCount++
			return nil
		}

		// force 逻辑：非 force 模式下，若数据库已有该 commit 则跳过
		if !force {
			var count int64
			if err := db.Model(&models.Commit{}).Where("commit_id = ?", meta.CommitId).Count(&count).Error; err == nil && count > 0 {
				logDebugf("跳过(已存在): %s", path)
				skipCount++
				return nil
			}
		}

		files = append(files, meta)
		return nil
	})

	return files, skipCount, err
}

// importCommitFile 将单个 commit JSON 文件解析并写入数据库 commits 表，同时计算 silica 指标。
//
// 参数:
//   - db: GORM 数据库连接，用于写入 commits 表
//   - meta: 该 JSON 文件的元信息，包含路径、仓库、分支、日期、commit ID 等
//   - idx: conversation 指纹索引，用于计算 silica
//   - maxDays: 对话与 commit 的最大关联时间窗口（天数）
//
// 返回值:
//   - error: 读取、解析、入库过程中发生的错误
//
// 关键技术原理:
//  1. 使用 json.Unmarshal 将 JSON 文件反序列化为 RepoCommitData
//  2. 字段校验：user_id、commit_id、commit_time 为必填字段，缺失则直接返回错误
//  3. commit_time 必须按 RFC3339 标准解析，保证时间精度与一致性
//  4. 从 diff 文本中提取新增行（extractAddedLinesFromDiff），并统计新增行数
//  5. 计算新增行指纹，调用 analyzeCommitSilica 计算含硅量及相关指标
//  6. 使用 GORM 的 clause.OnConflict 实现 UPSERT：一次性写入 commit 基本信息和 silica 相关字段
//  7. 保存关联的 tasks 并更新 conversations 的 task_id
func importCommitFile(db *gorm.DB, meta repoFileMeta, idx *conversationsIndexer, maxDays int) error {
	data, err := os.ReadFile(meta.Path)
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}

	var commitData RepoCommitData
	if err := json.Unmarshal(data, &commitData); err != nil {
		return fmt.Errorf("解析JSON失败: %w", err)
	}

	if commitData.UserId == "" {
		return fmt.Errorf("user_id为空")
	}
	if commitData.CommitId == "" {
		return fmt.Errorf("commit_id为空")
	}
	if commitData.CommitTime == "" {
		return fmt.Errorf("commit_time为空")
	}

	commitTime, err := time.Parse(time.RFC3339, commitData.CommitTime)
	if err != nil {
		return fmt.Errorf("解析commit_time失败: %w", err)
	}

	if commitData.WorkDir == "" {
		commitData.WorkDir = commitData.WorkPath
	}

	addedLines := extractAddedLinesFromDiff(commitData.Diff)
	commitData.DiffLines = len(addedLines)

	// 计算指纹列表
	fpHashes := make([]string, len(addedLines))
	for i, al := range addedLines {
		fpHashes[i] = calcLineFingerprint(al)
	}

	// 计算 silica
	p, tms := analyzeCommitSilica(
		commitData.CommitId,
		commitData.RepoAddr,
		commitData.UserId,
		commitTime,
		fpHashes,
		idx,
		maxDays,
	)

	// 填充 commit 关联字段到 tasks
	for _, tm := range tms {
		if tm.task == nil {
			continue
		}
		tm.task.RepoAddr = commitData.RepoAddr
		tm.task.RepoBranch = commitData.RepoBranch
		tm.task.UserId = commitData.UserId
		tm.task.UserName = commitData.UserName
		tm.task.ClientId = commitData.ClientId
		tm.task.WorkDir = commitData.WorkDir
	}

	// 保存 tasks 和更新 conversations
	if err := saveTasksAndConv(db, tms); err != nil {
		logWarnf("保存Commit[%s]关联数据失败: %v", commitData.CommitId, err)
	}
	if err := saveCommit(db, &commitData, p, commitTime); err != nil {
		logWarnf("保存Commit[%s]失败: %v", commitData.CommitId, err)
	}

	logDebugf("导入成功: %s (新增行: %d, silica: %.4f)", commitData.CommitId, len(addedLines), p.totalSilica)
	logDebugf("  commit_ancient_minutes=%.1f (%s)", p.ancientMinutes, p.ancientReason)
	// 调试日志：输出该commit的核心计算结果
	logDebugf("  %s: silica=%.4f (%d/%d行匹配), ai=%.1fmin, ancient=%.1fmin, total=%.1fmin",
		p.commitId, p.totalSilica, p.totalMatchLines, p.totalLines, p.aiMinutes, p.nonAiMinutes, p.realMinutes)
	return nil
}

func saveCommit(db *gorm.DB, commitData *RepoCommitData, p *commitParser, commitTime time.Time) error {
	commit := models.Commit{
		CommitId:               commitData.CommitId,
		CommitTime:             commitTime,
		RepoAddr:               commitData.RepoAddr,
		RepoBranch:             commitData.RepoBranch,
		GitUserName:            commitData.GitUserName,
		GitUserEmail:           commitData.GitUserEmail,
		UserId:                 commitData.UserId,
		UserName:               commitData.UserName,
		ClientId:               commitData.ClientId,
		WorkDir:                commitData.WorkDir,
		WorkDirId:              utils.GenerateWorkDirID(commitData.ClientId, commitData.WorkDir),
		Comment:                commitData.Comment,
		DiffLines:              commitData.DiffLines,
		TouchedFiles:           efficiencyV2StringJSON(efficiencyV2SortedUnique(commitData.Files)),
		Silica:                 p.totalSilica,
		CommitRealAiMinutes:    p.aiMinutes,
		CommitRealNonAiMinutes: p.nonAiMinutes,
		CommitRealMinutes:      p.realMinutes,
		CommitRealReason:       p.realReason,
		UpstreamTokens:         p.upstreamTokens,
		DownstreamTokens:       p.downstreamTokens,
		Cost:                   p.cost,
		CommitAncientMinutes:   p.ancientMinutes,
		CommitAncientReason:    p.ancientReason,
	}

	result := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "commit_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"commit_time", "repo_addr", "repo_branch",
			"git_user_name", "git_user_email", "user_id", "user_name",
			"client_id", "work_dir", "work_dir_id", "comment", "diff_lines", "updated_at",
			"touched_files",
			"commit_ancient_minutes", "commit_ancient_reason",
			"silica",
			"commit_real_ai_minutes", "commit_real_non_ai_minutes", "commit_real_minutes", "commit_real_reason",
			"upstream_tokens", "downstream_tokens", "cost",
		}),
	}).Create(&commit)
	if result.Error != nil {
		return fmt.Errorf("写入commits表失败: %w", result.Error)
	}
	return nil
}

// saveTasksAndConv 将计算出的tasks保存到数据库，并更新关联的conversation记录。
func saveTasksAndConv(db *gorm.DB, tms []taskMatched) error {
	if len(tms) == 0 {
		return nil
	}

	// 收集所有 session_id
	sessionIDs := make([]string, 0, len(tms))
	for _, tm := range tms {
		if tm.task != nil {
			sessionIDs = append(sessionIDs, tm.task.SessionId)
		}
	}
	if len(sessionIDs) == 0 {
		return nil
	}

	// 查询 sessions 表补全元数据
	var sessions []models.Session
	sessionMap := make(map[string]models.Session)
	if err := db.Where("session_id IN ?", sessionIDs).Find(&sessions).Error; err != nil {
		logWarnf("查询sessions表失败: %v", err)
	} else {
		for _, s := range sessions {
			sessionMap[s.SessionId] = s
		}
	}

	// 查询 conversations 表，按 session_id 聚合 token 和 cost
	type convAgg struct {
		SessionId        string
		UpstreamTokens   int64
		DownstreamTokens int64
		Cost             float64
	}
	var aggs []convAgg
	convAggMap := make(map[string]convAgg)
	if err := db.Model(&models.Conversation{}).
		Select("session_id, SUM(upstream_tokens) as upstream_tokens, SUM(downstream_tokens) as downstream_tokens, SUM(cost) as cost").
		Where("session_id IN ?", sessionIDs).
		Group("session_id").
		Find(&aggs).Error; err != nil {
		logWarnf("查询conversations表失败: %v", err)
	} else {
		for _, a := range aggs {
			convAggMap[a.SessionId] = a
		}
	}

	// 填充 task 缺失字段并收集待保存对象
	var tasks []models.Task
	for _, tm := range tms {
		if tm.task == nil {
			continue
		}
		t := tm.task
		sessionID := t.SessionId
		if s, ok := sessionMap[sessionID]; ok {
			if t.UserId == "" {
				t.UserId = s.UserId
			}
			if t.UserName == "" {
				t.UserName = s.UserName
			}
			t.ClientIde = s.ClientIde
			t.ClientVersion = s.ClientVersion
			t.ClientOs = s.ClientOs
			t.ClientOsVersion = s.ClientOsVersion
			if t.ClientId == "" {
				t.ClientId = s.ClientId
			}
			if t.WorkDir != "" {
				t.WorkDirId = utils.GenerateWorkDirID(t.ClientId, t.WorkDir)
			}
			if t.SessionDate == "" {
				t.SessionDate = s.SessionDate
			}
			if t.ConversationDate == "" {
				t.ConversationDate = s.ConversationDate
			}
		}
		if a, ok := convAggMap[sessionID]; ok {
			t.UpstreamTokens = a.UpstreamTokens
			t.DownstreamTokens = a.DownstreamTokens
			t.Cost = a.Cost
		}
		tasks = append(tasks, *t)
	}

	if len(tasks) == 0 {
		return nil
	}

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
			"title", "session_date", "conversation_date",
		}),
	}).Create(&tasks).Error; err != nil {
		return err
	}

	for _, tm := range tms {
		if tm.task == nil || len(tm.convs) == 0 {
			continue
		}
		var requestIds []string
		for _, c := range tm.convs {
			requestIds = append(requestIds, c.conv.requestId)
		}
		if len(requestIds) > 0 {
			if err := db.Model(&models.Conversation{}).
				Where("session_id = ? AND request_id IN ?", tm.task.SessionId, requestIds).
				Update("task_id", tm.task.TaskId).Error; err != nil {
				logWarnf("更新conversation的task_id失败 [%s]: %v", tm.task.TaskId, err)
			}
		}
	}
	return nil
}

// runImportRepo 执行 repo 数据导入的主流程，包括目录扫描、批量导入和统计输出。
//
// 参数:
//   - repoDir: 客户端上报 commit JSON 文件的根目录
//   - analysedDir: 已分析数据的输出根目录，用于读取 .silica.json 构建索引
//   - force: 是否强制重新导入
//   - maxDays: 对话与 commit 的最大关联时间窗口（天数）
//
// 返回值:
//   - error: 任何阶段发生的错误都会包装后返回，并通过 recordCommandRun 记录执行结果
//
// 关键技术原理:
//  1. 前置校验：检查 repoDir 是否存在，提前失败避免后续无意义的数据库连接
//  2. 数据库连接：通过 models.OpenGormDB 建立连接，并在函数退出时关闭底层 sqlDB
//  3. 先构建 conversation 指纹索引，再扫描和导入 commit
//  4. 幂等扫描：调用 scanRepoDir 获取待导入列表和跳过计数（日期过滤 + 数据库存在性检查）
//  5. 批量处理：逐个文件调用 importCommitFile，失败时记录日志并计数，不中断整体流程
//  6. 进度提示：每成功导入 50 个文件调用 logPromptProgress 输出进度信息
//  7. 命令埋点：通过 recordCommandRun 记录命令执行时间、成功/失败/跳过数量，用于运维监控
func runImportRepo(repoDir, analysedDir string, force bool, maxDays int, startDateStr, endDateStr, dateStr string) error {
	startTime := time.Now()

	if _, err := os.Stat(repoDir); os.IsNotExist(err) {
		recordCommandRun("import-repo", startTime, 0, 0, 0, err)
		return fmt.Errorf("repo目录不存在: %s", repoDir)
	}

	startDate, endDate, err := parseDateRange(startDateStr, endDateStr, dateStr)
	if err != nil {
		recordCommandRun("import-repo", startTime, 0, 0, 0, err)
		return err
	}

	db, err := models.OpenGormDB(cfg.StatDatabase.DSN())
	if err != nil {
		recordCommandRun("import-repo", startTime, 0, 0, 0, err)
		return fmt.Errorf("连接数据库失败: %w", err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	// 先构建 conversation 索引
	taskFPDir := filepath.Join(analysedDir, "task", "conversation")
	idx, err := buildConversationsIndexer(taskFPDir)
	if err != nil {
		recordCommandRun("import-repo", startTime, 0, 0, 0, err)
		return fmt.Errorf("构建conversation指纹索引失败: %w", err)
	}

	// 扫描 repo 目录
	files, skipCount, err := scanRepoDir(repoDir, db, force, startDate, endDate)
	if err != nil {
		recordCommandRun("import-repo", startTime, 0, 0, 0, err)
		return fmt.Errorf("扫描repo目录失败: %w", err)
	}

	if len(files) == 0 {
		logInfo("没有找到待导入的commit文件")
		recordCommandRun("import-repo", startTime, 0, 0, skipCount, nil)
		return nil
	}

	logInfof("找到 %d 个待导入的commit文件", len(files))

	successCount := 0
	failCount := 0

	totalFiles := len(files)
	for i, fileMeta := range files {
		if err := importCommitFile(db, fileMeta, idx, maxDays); err != nil {
			logWarnf("导入失败 [%s]: %v", fileMeta.Path, err)
			failCount++
		} else {
			successCount++
		}
		logProgress("[import-repo] 导入commit", i+1, totalFiles, 50)
	}

	logInfof("导入完成: 成功 %d 个，失败 %d 个，跳过 %d 个", successCount, failCount, skipCount)
	recordCommandRun("import-repo", startTime, successCount, failCount, skipCount, nil)
	return nil
}

// importRepoCmd 定义了 "import-repo" Cobra 子命令，用于将客户端上报的 repo 数据导入数据库。
//
// 用法: kbcli import-repo [--repo-dir PATH] [--analysed-dir PATH] [--force] [--remote URL]
//
// 标志说明:
//   - repo-dir: repo 数据根目录，不指定则使用配置文件中的 RepoDir
//   - analysed-dir: 已分析数据目录，用于读取 .silica.json 构建 conversation 索引
//   - force: 强制重新导入，覆盖数据库中已存在的 commit 记录
//   - remote: 若指定远程 kbcli 服务地址，则将命令参数发送到远程执行，本地不处理
var importRepoCmd = &cobra.Command{
	Use:   "import-repo",
	Short: "导入客户端上报的repo数据到commits表，同时计算silica指标",
	Long:  "扫描指定repo目录下的commit JSON文件，批量写入commits表，并计算各commit的含硅量（task贡献比例）",
	RunE: func(cmd *cobra.Command, args []string) error {
		// 从命令行标志中读取参数值
		repoDir, _ := cmd.Flags().GetString("repo-dir")
		analysedDir, _ := cmd.Flags().GetString("analysed-dir")
		force, _ := cmd.Flags().GetBool("force")
		remote, _ := cmd.Flags().GetString("remote")
		startDate, _ := cmd.Flags().GetString("start-date")
		endDate, _ := cmd.Flags().GetString("end-date")
		date, _ := cmd.Flags().GetString("date")
		maxDays, _ := cmd.Flags().GetInt("max-days")
		// 若指定了 remote，将命令参数序列化后发送到远程 kbcli 服务执行
		if remote != "" {
			return sendToRemote(remote, "import-repo", map[string]interface{}{
				"repo_dir":     repoDir,
				"analysed_dir": analysedDir,
				"force":        force,
				"start_date":   startDate,
				"end_date":     endDate,
				"date":         date,
				"max_days":     maxDays,
			})
		}

		// 未指定目录时，回退到配置文件中的默认值
		if repoDir == "" {
			repoDir = cfg.RepoDir
		}
		if analysedDir == "" {
			analysedDir = cfg.AnalysedDir
		}
		if maxDays <= 0 {
			maxDays = cfg.TaskCreate.SilicaMaxDays
		}
		// 未显式传 start-date 且非单日(date)模式时，套全局分析起始日下界。
		if date == "" {
			startDate = applyAnalysisFloor(startDate)
		}

		// 执行本地导入流程
		return runImportRepo(repoDir, analysedDir, force, maxDays, startDate, endDate, date)
	},
}

// init 注册 import-repo 命令及其命令行标志到 rootCmd。
//
// 说明:
//
//	SortFlags 设为 false 保持标志按定义顺序显示，提升可读性。
func init() {
	importRepoCmd.Flags().SortFlags = false
	importRepoCmd.Flags().String("repo-dir", "", "repo 目录路径")
	importRepoCmd.Flags().String("analysed-dir", "", "已处理文件的输出目录")
	importRepoCmd.Flags().BoolP("force", "f", false, "强制重新导入，覆盖已存在数据")
	importRepoCmd.Flags().String("start-date", "", "限定起始日期，格式 YYYYMMDD，为空则不限")
	importRepoCmd.Flags().String("end-date", "", "限定结束日期，格式 YYYYMMDD，为空则不限")
	importRepoCmd.Flags().String("date", "", "限定日期，格式 YYYYMMDD，限定活跃时间在该日期之内（与start-date/end-date互斥）")
	importRepoCmd.Flags().Int("max-days", 0, "对话结束后多少天内的commit算相关（默认从config读取）")
	importRepoCmd.Flags().String("remote", "", "远程kbcli服务地址（如 http://127.0.0.1:8080），指定后命令将发送到远程执行")
	rootCmd.AddCommand(importRepoCmd)
}
