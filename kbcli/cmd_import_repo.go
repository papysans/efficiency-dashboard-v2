package main

import (
	"encoding/json"
	"errors"
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
	CommitId     string `json:"commit_id"`           // Git commit hash
	CommitTime   string `json:"commit_time"`         // 提交时间，RFC3339 格式
	RepoAddr     string `json:"repo_addr"`           // 仓库地址
	RepoBranch   string `json:"repo_branch"`         // 分支名称
	GitUserName  string `json:"git_user_name"`       // Git 用户名
	GitUserEmail string `json:"git_user_email"`      // Git 邮箱
	UserId       string `json:"user_id"`             // 系统用户ID
	UserName     string `json:"user_name"`           // 系统用户名
	ClientId     string `json:"client_id"`           // 客户端ID
	WorkPath     string `json:"work_path,omitempty"` // 工作路径（旧字段，兼容用）
	WorkDir      string `json:"work_dir,omitempty"`  // 工作目录（新字段）
	Comment      string `json:"comment"`             // 提交注释
	DiffLines    int    `json:"diff_lines"`          // 新增代码行数
	Diff         string `json:"diff"`                // 完整的 diff 文本
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
//   - analysedDir: 已处理数据的输出根目录，用于存放生成的 .fp 指纹文件
//   - force: 是否强制重新导入；为 false 时会跳过已存在对应 .fp 文件的 commit
//
// 返回值:
//   - []repoFileMeta: 待导入的文件元信息列表
//   - int: 因已处理而被跳过的文件数量
//   - error: 扫描过程中发生的错误
//
// 关键技术原理:
//  1. 使用 filepath.Walk 递归遍历目录树，跳过 analysedDir 子树避免循环处理
//  2. 通过正则表达式 reRepoPath 校验并拆解文件路径，确保只有符合约定格式的文件才会被纳入
//  3. 幂等性控制：非 force 模式下，若目标 .fp 文件已存在，则认为该 commit 已导入，直接跳过
func scanRepoDir(repoDir, analysedDir string, force bool) ([]repoFileMeta, int, error) {
	var files []repoFileMeta
	skipCount := 0

	// 递归遍历 repoDir 下的所有文件和目录
	err := filepath.Walk(repoDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 遇到目录时，判断是否为 analysedDir 本身或其子目录，如果是则跳过整个子树
		// 避免将已处理过程中生成的中间文件再次纳入扫描
		if info.IsDir() {
			relPath, err := filepath.Rel(repoDir, path)
			if err != nil {
				return err
			}
			analysedDirRel, err := filepath.Rel(repoDir, analysedDir)
			if err == nil {
				if relPath == analysedDirRel || strings.HasPrefix(relPath, analysedDirRel+string(filepath.Separator)) {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// 只处理以 .json 结尾的文件，其他后缀直接忽略
		if !strings.HasSuffix(info.Name(), ".json") {
			return nil
		}

		// 将文件绝对路径转为相对于 repoDir 的相对路径
		relPath, err := filepath.Rel(repoDir, path)
		if err != nil {
			return err
		}

		// 统一路径分隔符为正斜杠，以便正则匹配（Windows 下 filepath.Rel 返回反斜杠）
		relPath = filepath.ToSlash(relPath)

		// 使用正则校验路径格式，不符合约定的不处理
		matches := reRepoPath.FindStringSubmatch(relPath)
		if matches == nil {
			return nil
		}

		// 从正则匹配结果中提取元信息
		meta := repoFileMeta{
			Path:     path,
			RelPath:  relPath,
			Repo:     matches[1], // 仓库名
			Branch:   matches[2], // 分支名
			Year:     matches[3], // 年
			Month:    matches[4], // 月
			Day:      matches[5], // 日
			CommitId: matches[6], // commit ID（不含 .json 后缀）
		}

		// 非强制模式下，检查目标指纹文件是否已存在，存在则表示已处理过，直接跳过
		if !force {
			fpPath := fpPathForMeta(analysedDir, meta)
			if _, err := os.Stat(fpPath); err == nil {
				logDebugf("跳过(已处理): %s", path)
				skipCount++
				return nil
			}
		}

		// 通过所有校验，将该文件加入待导入列表
		files = append(files, meta)
		return nil
	})

	return files, skipCount, err
}

// importCommitFile 将单个 commit JSON 文件解析并写入数据库 commits 表，同时生成新增代码行的指纹文件。
//
// 参数:
//   - db: GORM 数据库连接，用于写入 commits 表
//   - meta: 该 JSON 文件的元信息，包含路径、仓库、分支、日期、commit ID 等
//   - analysedDir: 指纹文件输出根目录，用于生成 .fp 文件的存放路径
//
// 返回值:
//   - error: 读取、解析、入库或写指纹文件过程中发生的错误
//
// 关键技术原理:
//  1. 使用 json.Unmarshal 将 JSON 文件反序列化为 RepoCommitData
//  2. 字段校验：user_id、commit_id、commit_time 为必填字段，缺失则直接返回错误
//  3. commit_time 必须按 RFC3339 标准解析，保证时间精度与一致性
//  4. 从 diff 文本中提取新增行（extractAddedLinesFromDiff），并统计新增行数写入 diff_lines
//  5. 调用 estimateCommitAncientMinutes 根据新增行数估算 commit 的 "Ancient Minutes"（原始人分钟）
//  6. 使用 GORM 的 clause.OnConflict 实现 UPSERT：以 commit_id 为唯一键，冲突时更新除主键外的所有字段
//  7. 将新增行的指纹逐行写入 .fp 文件，用于后续代码相似度/抄袭检测等场景
func importCommitFile(db *gorm.DB, meta repoFileMeta, analysedDir string, startDate, endDate *time.Time) error {
	// 读取磁盘上的 JSON 文件内容
	data, err := os.ReadFile(meta.Path)
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}

	// 解析 JSON 数据到 RepoCommitData 结构体
	var commitData RepoCommitData
	if err := json.Unmarshal(data, &commitData); err != nil {
		return fmt.Errorf("解析JSON失败: %w", err)
	}

	// 必填字段校验：user_id 不能为空
	if commitData.UserId == "" {
		return fmt.Errorf("user_id为空")
	}
	// 必填字段校验：commit_id 不能为空
	if commitData.CommitId == "" {
		return fmt.Errorf("commit_id为空")
	}
	// 必填字段校验：commit_time 不能为空
	if commitData.CommitTime == "" {
		return fmt.Errorf("commit_time为空")
	}

	// 按 RFC3339 格式解析提交时间，例如 2024-01-02T15:04:05Z07:00
	commitTime, err := time.Parse(time.RFC3339, commitData.CommitTime)
	if err != nil {
		return fmt.Errorf("解析commit_time失败: %w", err)
	}

	// 日期范围过滤
	if !isActiveTimeInRange(commitTime, startDate, endDate) {
		return errSkipTask
	}

	// 兼容新旧字段：优先使用 work_dir，不存在时回退到 work_path
	workDir := commitData.WorkDir
	if workDir == "" {
		workDir = commitData.WorkPath
	}

	// 从 diff 文本中提取新增代码行，并统计新增行数
	addedLines := extractAddedLinesFromDiff(commitData.Diff)
	commitData.DiffLines = len(addedLines)

	// 根据新增行数估算 commit 原始人分钟（Ancient Minutes）及估算原因
	ancientMinutes, ancientReason := estimateCommitAncientMinutes(commitData.DiffLines)
	logDebugf("  commit_ancient_minutes=%.1f (%s)", ancientMinutes, ancientReason)

	// 构造数据库模型对象，准备写入 commits 表
	commit := models.Commit{
		CommitId:             commitData.CommitId,
		CommitTime:           commitTime,
		RepoAddr:             commitData.RepoAddr,
		RepoBranch:           commitData.RepoBranch,
		GitUserName:          commitData.GitUserName,
		GitUserEmail:         commitData.GitUserEmail,
		UserId:               commitData.UserId,
		UserName:             commitData.UserName,
		ClientId:             commitData.ClientId,
		WorkDir:              workDir,
		WorkDirId:            utils.GenerateWorkDirID(commitData.ClientId, workDir),
		Comment:              commitData.Comment,
		DiffLines:            commitData.DiffLines,
		TaskIds:              models.StringJSON("[]"), // 初始化为空 JSON 数组，后续由 task 关联逻辑填充
		TaskIdsSilica:        models.StringJSON("[]"), // 同上，Silica 关联任务列表
		TaskAcceptRatios:     models.StringJSON("[]"), // 代码采纳率数组，与 task_ids 一一对应
		CommitAncientMinutes: &ancientMinutes,         // 指针类型，允许数据库 NULL
		CommitAncientReason:  ancientReason,           // 估算原因描述
	}

	// UPSERT 写入数据库：以 commit_id 为唯一键，冲突时更新指定字段
	result := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "commit_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"commit_time", "repo_addr", "repo_branch",
			"git_user_name", "git_user_email", "user_id", "user_name",
			"client_id", "work_dir", "work_dir_id", "comment", "diff_lines", "updated_at",
			"commit_ancient_minutes", "commit_ancient_reason",
		}),
	}).Create(&commit)
	if result.Error != nil {
		return fmt.Errorf("写入commits表失败: %w", result.Error)
	}

	// 生成对应 .fp 指纹文件路径并写入新增行指纹
	fpPath := fpPathForMeta(analysedDir, meta)
	if err := writeFingerprintsToFile(addedLines, fpPath); err != nil {
		return fmt.Errorf("写入fp文件失败 [%s]: %w", fpPath, err)
	}

	logDebugf("导入成功: %s (新增行指纹: %d)", commitData.CommitId, len(addedLines))
	return nil
}

// writeFingerprintsToFile 将新增代码行的指纹逐行写入 .fp 文件。
//
// 参数:
//   - addedLines: 新增代码行列表，由 extractAddedLinesFromDiff 提取得到
//   - fpPath: 指纹文件的完整输出路径
//
// 返回值:
//   - error: 目录创建或文件写入过程中发生的错误
//
// 关键技术原理:
//  1. 自动递归创建目标目录，确保多级路径存在
//  2. 对每一行新增代码调用 calcLineFingerprint 计算指纹（通常是哈希或归一化表示）
//  3. 使用 strings.Builder 批量拼接，减少内存分配，最后一次性写入文件，提升 I/O 效率
func writeFingerprintsToFile(addedLines []addedLine, fpPath string) error {
	// 获取指纹文件所在目录，若目录不存在则递归创建
	fpDir := filepath.Dir(fpPath)
	if err := os.MkdirAll(fpDir, 0755); err != nil {
		return fmt.Errorf("创建fp目录失败: %w", err)
	}

	// 使用 strings.Builder 拼接所有指纹，减少中间字符串分配
	var sb strings.Builder
	for _, al := range addedLines {
		// 计算单行指纹并追加换行符
		sb.WriteString(calcLineFingerprint(al))
		sb.WriteByte('\n')
	}

	// 将拼接结果一次性写入文件，权限设为 0644（所有者可读写，其他用户只读）
	if err := os.WriteFile(fpPath, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("写入fp文件失败: %w", err)
	}
	return nil
}

// fpPathForMeta 根据文件元信息构造对应的指纹文件（.fp）输出路径。
//
// 参数:
//   - analysedDir: 指纹文件根目录
//   - meta: commit 文件的元信息
//
// 返回值:
//   - string: 指纹文件的完整路径，格式为 analysedDir/repo/repo/branch/YYYY/MM/DD/commit_id.fp
//
// 说明:
//
//	路径结构与原始 JSON 文件的层级保持一致，仅将后缀由 .json 替换为 .fp，
//	这样便于通过原始文件路径快速定位对应的指纹文件，也方便扫描时进行幂等性判断。
func fpPathForMeta(analysedDir string, meta repoFileMeta) string {
	return filepath.Join(analysedDir, "repo", meta.Repo, meta.Branch, meta.Year, meta.Month, meta.Day, meta.CommitId+".fp")
}

// runImportRepo 执行 repo 数据导入的主流程，包括目录扫描、批量导入和统计输出。
//
// 参数:
//   - repoDir: 客户端上报 commit JSON 文件的根目录
//   - analysedDir: 指纹文件输出根目录
//   - force: 是否强制重新导入
//
// 返回值:
//   - error: 任何阶段发生的错误都会包装后返回，并通过 recordCommandRun 记录执行结果
//
// 关键技术原理:
//  1. 前置校验：检查 repoDir 是否存在，提前失败避免后续无意义的数据库连接
//  2. 数据库连接：通过 models.OpenGormDB 建立连接，并在函数退出时关闭底层 sqlDB
//  3. 幂等扫描：调用 scanRepoDir 获取待导入列表和跳过计数
//  4. 批量处理：逐个文件调用 importCommitFile，失败时记录日志并计数，不中断整体流程
//  5. 进度提示：每成功导入 50 个文件调用 logPromptProgress 输出进度信息
//  6. 命令埋点：通过 recordCommandRun 记录命令执行时间、成功/失败/跳过数量，用于运维监控
func runImportRepo(repoDir, analysedDir string, force bool, startDateStr, endDateStr, dateStr string) error {
	startTime := time.Now()

	// 检查 repo 目录是否存在，不存在则直接返回错误并记录执行结果
	if _, err := os.Stat(repoDir); os.IsNotExist(err) {
		recordCommandRun("import-repo", startTime, 0, 0, 0, err)
		return fmt.Errorf("repo目录不存在: %s", repoDir)
	}

	// 解析日期范围
	startDate, endDate, err := parseDateRange(startDateStr, endDateStr, dateStr)
	if err != nil {
		recordCommandRun("import-repo", startTime, 0, 0, 0, err)
		return err
	}

	// 打开数据库连接，获取底层 *sql.DB 以便在函数结束时关闭
	db, err := models.OpenGormDB(cfg.StatDatabase.DSN())
	if err != nil {
		recordCommandRun("import-repo", startTime, 0, 0, 0, err)
		return fmt.Errorf("连接数据库失败: %w", err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	// 扫描目录，获取待导入文件列表和已跳过数量
	files, skipCount, err := scanRepoDir(repoDir, analysedDir, force)
	if err != nil {
		recordCommandRun("import-repo", startTime, 0, 0, 0, err)
		return fmt.Errorf("扫描repo目录失败: %w", err)
	}

	// 如果没有待导入文件，记录成功并提前退出
	if len(files) == 0 {
		logInfo("没有找到待导入的commit文件")
		recordCommandRun("import-repo", startTime, 0, 0, skipCount, nil)
		return nil
	}

	logInfof("找到 %d 个待导入的commit文件", len(files))

	successCount := 0
	failCount := 0

	// 逐个导入文件，失败不中断，继续处理下一个
	for _, fileMeta := range files {
		if err := importCommitFile(db, fileMeta, analysedDir, startDate, endDate); err != nil {
			if errors.Is(err, errSkipTask) {
				logDebugf("跳过(日期范围过滤): %s", fileMeta.Path)
				skipCount++
				continue
			}
			logWarnf("导入失败 [%s]: %v", fileMeta.Path, err)
			failCount++
		} else {
			successCount++
			// 每成功处理 50 个输出一次进度提示
			logPromptProgress(successCount, 50)
		}
	}

	// 输出最终统计信息并记录命令执行结果
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
//   - analysed-dir: 指纹文件输出根目录，不指定则使用配置文件中的 AnalysedDir
//   - force: 强制重新导入，忽略已有的 .fp 文件
//   - remote: 若指定远程 kbcli 服务地址，则将命令参数发送到远程执行，本地不处理
var importRepoCmd = &cobra.Command{
	Use:   "import-repo",
	Short: "导入客户端上报的repo数据到commits表",
	Long:  "扫描指定repo目录下的commit JSON文件，批量写入commits表",
	RunE: func(cmd *cobra.Command, args []string) error {
		// 从命令行标志中读取参数值
		repoDir, _ := cmd.Flags().GetString("repo-dir")
		analysedDir, _ := cmd.Flags().GetString("analysed-dir")
		force, _ := cmd.Flags().GetBool("force")
		remote, _ := cmd.Flags().GetString("remote")
		startDate, _ := cmd.Flags().GetString("start-date")
		endDate, _ := cmd.Flags().GetString("end-date")
		date, _ := cmd.Flags().GetString("date")

		// 若指定了 remote，将命令参数序列化后发送到远程 kbcli 服务执行
		if remote != "" {
			return sendToRemote(remote, "import-repo", map[string]interface{}{
				"repo_dir":     repoDir,
				"analysed_dir": analysedDir,
				"force":        force,
				"start_date":   startDate,
				"end_date":     endDate,
				"date":         date,
			})
		}

		// 未指定目录时，回退到配置文件中的默认值
		if repoDir == "" {
			repoDir = cfg.RepoDir
		}
		if analysedDir == "" {
			analysedDir = cfg.AnalysedDir
		}

		// 执行本地导入流程
		return runImportRepo(repoDir, analysedDir, force, startDate, endDate, date)
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
	importRepoCmd.Flags().String("remote", "", "远程kbcli服务地址（如 http://127.0.0.1:8080），指定后命令将发送到远程执行")
	rootCmd.AddCommand(importRepoCmd)
}
