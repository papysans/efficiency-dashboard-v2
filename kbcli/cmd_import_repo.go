package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	_ "github.com/lib/pq"

	"github.com/spf13/cobra"
)

// RepoCommitData 客户端上报的commit数据解析结构
type RepoCommitData struct {
	CommitID     string `json:"commit_id"`
	CommitTime   string `json:"commit_time"`
	RepoAddr     string `json:"repo_addr"`
	RepoBranch   string `json:"repo_branch"`
	GitUserName  string `json:"git_user_name"`
	GitUserEmail string `json:"git_user_email"`
	UserID       string `json:"user_id"`
	UserName     string `json:"user_name"`
	ClientID     string `json:"client_id"`
	WorkPath     string `json:"work_path,omitempty"`
	WorkDir      string `json:"work_dir,omitempty"` // 兼容字段
	Comment      string `json:"comment"`
	DiffLines    int    `json:"diff_lines"`
	Diff         string `json:"diff"` // 不入库
}

// repoFileMeta 扫描到的文件元信息
type repoFileMeta struct {
	Path     string // 绝对路径
	RelPath  string // 相对于repoDir的路径
	Repo     string
	Branch   string
	Year     string
	Month    string
	Day      string
	CommitID string
}

var (
	reRepoPath = regexp.MustCompile(`^([^/]+)/([^/]+)/(\d{4})/(\d{2})/(\d{2})/([^/]+)\.json$`)
)
var importRepoCmd = &cobra.Command{
	Use:   "import-repo",
	Short: "导入客户端上报的repo数据到commits表",
	Long:  "扫描指定repo目录下的commit JSON文件，批量写入commits表",
	RunE: func(cmd *cobra.Command, args []string) error {
		repoDir, _ := cmd.Flags().GetString("repo-dir")
		outputDir, _ := cmd.Flags().GetString("output-dir")

		// 验证目录存在性
		if _, err := os.Stat(repoDir); os.IsNotExist(err) {
			return fmt.Errorf("repo目录不存在: %s", repoDir)
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

		// 自动建表：确保 commits 和 repos 表存在
		if err := ensureImportRepoTables(db); err != nil {
			return fmt.Errorf("自动建表失败: %w", err)
		}

		// 扫描待导入文件
		files, skipCount, err := scanRepoDir(repoDir, outputDir)
		if err != nil {
			return fmt.Errorf("扫描repo目录失败: %w", err)
		}

		if len(files) == 0 {
			fmt.Println("没有找到待导入的commit文件")
			return nil
		}

		fmt.Printf("找到 %d 个待导入的commit文件\n", len(files))

		successCount := 0
		failCount := 0

		for _, fileMeta := range files {
			if err := importCommitFile(db, fileMeta, outputDir); err != nil {
				fmt.Fprintf(os.Stderr, "导入失败 [%s]: %v\n", fileMeta.Path, err)
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
	importRepoCmd.Flags().String("repo-dir", "./repo", "repo 目录路径（必需）")
	importRepoCmd.Flags().String("output-dir", "./analysed", "已处理文件的输出目录")
	if err := importRepoCmd.MarkFlagRequired("repo-dir"); err != nil {
		panic(err)
	}
	rootCmd.AddCommand(importRepoCmd)
}

// scanRepoDir 扫描repo目录，返回符合条件的commit文件列表
// 路径格式: <repo-dir>/<repo>/<branch>/<year>/<month>/<day>/<commit-id>.json
func scanRepoDir(repoDir, outputDir string) ([]repoFileMeta, int, error) {
	var files []repoFileMeta
	skipCount := 0

	err := filepath.Walk(repoDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过outputDir目录（可能是analysed或其他用户指定的目录）
		if info.IsDir() {
			relPath, err := filepath.Rel(repoDir, path)
			if err != nil {
				return err
			}
			outputDirRel, err := filepath.Rel(repoDir, outputDir)
			if err == nil {
				// 检查当前目录是否是outputDir或其子目录
				if relPath == outputDirRel || strings.HasPrefix(relPath, outputDirRel+string(filepath.Separator)) {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// 只处理.json文件
		if !strings.HasSuffix(info.Name(), ".json") {
			return nil
		}

		// 解析路径格式
		relPath, err := filepath.Rel(repoDir, path)
		if err != nil {
			return err
		}

		// 转换路径分隔符为/以匹配正则
		relPath = filepath.ToSlash(relPath)

		matches := reRepoPath.FindStringSubmatch(relPath)
		if matches == nil {
			return nil // 路径格式不匹配，忽略
		}

		// 检查outputDir下是否已有对应文件，有则跳过
		analysedPath := filepath.Join(outputDir, relPath)
		if _, err := os.Stat(analysedPath); err == nil {
			fmt.Printf("跳过(已处理): %s\n", path)
			skipCount++
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
			CommitID: matches[6],
		}

		files = append(files, meta)
		return nil
	})

	return files, skipCount, err
}

func importEstimateCommitAncientMinutes(diffLines int) (float64, string) {
	if diffLines <= 0 {
		return 5, "默认估算:无代码变更"
	}
	minutes := float64(diffLines) * 1.5
	if minutes < 5 {
		minutes = 5
	}
	return minutes, fmt.Sprintf("基于diff_lines=%d估算(1.5分钟/行)", diffLines)
}

// importCommitFile 导入单个commit文件
func importCommitFile(db *sql.DB, meta repoFileMeta, outputDir string) error {
	// 读取并解析JSON文件
	data, err := os.ReadFile(meta.Path)
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}

	var commitData RepoCommitData
	if err := json.Unmarshal(data, &commitData); err != nil {
		return fmt.Errorf("解析JSON失败: %w", err)
	}

	// 验证必填字段
	if commitData.CommitID == "" {
		return fmt.Errorf("commit_id为空")
	}
	if commitData.CommitTime == "" {
		return fmt.Errorf("commit_time为空")
	}

	// 兼容处理 work_path/work_dir 字段
	workDir := commitData.WorkDir
	if workDir == "" {
		workDir = commitData.WorkPath
	}

	// 解析commit_time
	commitTime, err := time.Parse(time.RFC3339, commitData.CommitTime)
	if err != nil {
		return fmt.Errorf("解析commit_time失败: %w", err)
	}

	// 执行UPSERT插入commits表
	_, err = db.Exec(`INSERT INTO commits (
		commit_id, commit_time, repo_addr, repo_branch,
		git_user_name, git_user_email, user_id, user_name,
		client_id, work_dir, comment, diff_lines,
		updated_at
	) VALUES (
		$1, $2, $3, $4,
		$5, $6, $7, $8,
		$9, $10, $11, $12,
		CURRENT_TIMESTAMP
	) ON CONFLICT (commit_id) DO UPDATE SET
		commit_time = EXCLUDED.commit_time,
		comment = EXCLUDED.comment,
		diff_lines = EXCLUDED.diff_lines,
		updated_at = CURRENT_TIMESTAMP`,
		commitData.CommitID, commitTime, commitData.RepoAddr, commitData.RepoBranch,
		commitData.GitUserName, commitData.GitUserEmail, commitData.UserID, commitData.UserName,
		commitData.ClientID, workDir, commitData.Comment, commitData.DiffLines,
	)

	if err != nil {
		return fmt.Errorf("写入commits表失败: %w", err)
	}

	// 估算 commit_ancient_minutes（基于diff_lines的启发式算法）
	if commitData.DiffLines > 0 {
		ancientMinutes, ancientReason := importEstimateCommitAncientMinutes(commitData.DiffLines)
		if _, err := db.Exec(`UPDATE commits SET commit_ancient_minutes = $1, commit_ancient_minutes_reason = $2, updated_at = CURRENT_TIMESTAMP WHERE commit_id = $3 AND commit_ancient_minutes IS NULL AND commit_ancient_minutes_manual IS NULL`,
			ancientMinutes, ancientReason, commitData.CommitID); err != nil {
			fmt.Fprintf(os.Stderr, "警告: 更新commit_ancient_minutes失败 [%s]: %v\n", commitData.CommitID, err)
		} else {
			fmt.Printf("  commit_ancient_minutes=%.1f (%s)\n", ancientMinutes, ancientReason)
		}
	}

	// 导入成功，拷贝文件到outputDir目录
	analysedPath := filepath.Join(outputDir, meta.RelPath)
	analysedParent := filepath.Dir(analysedPath)

	if err := os.MkdirAll(analysedParent, 0755); err != nil {
		// 创建目录失败不影响主流程，仅记录错误
		fmt.Fprintf(os.Stderr, "创建output目录失败 [%s]: %v\n", analysedParent, err)
		return nil
	}

	if err := os.WriteFile(analysedPath, data, 0644); err != nil {
		// 拼接文件失败不影响主流程，仅记录错误
		fmt.Fprintf(os.Stderr, "拷贝文件到output目录失败 [%s]: %v\n", analysedPath, err)
		return nil
	}

	fmt.Printf("导入成功: %s\n", commitData.CommitID)
	return nil
}

// ensureImportRepoTables 确保导入repo数据所需的数据库表存在，不存在则自动创建
func ensureImportRepoTables(db *sql.DB) error {
	// 创建 commits 表
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS commits (
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
	)`)
	if err != nil {
		return fmt.Errorf("创建commits表失败: %w", err)
	}

	// 兼容旧表：将 work_path 列重命名为 work_dir
	db.Exec(`ALTER TABLE commits RENAME COLUMN work_path TO work_dir`)

	// 创建 repos 表
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS repos (
		repo_id VARCHAR(500) PRIMARY KEY,
		repo_addr TEXT NOT NULL,
		repo_branch VARCHAR(500) NOT NULL,
		start_time TIMESTAMPTZ,
		end_time TIMESTAMPTZ,
		commit_ids JSONB DEFAULT '[]',
		created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return fmt.Errorf("创建repos表失败: %w", err)
	}

	// 创建索引（IF NOT EXISTS 保证幂等）
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_commits_repo_addr ON commits(repo_addr)`,
		`CREATE INDEX IF NOT EXISTS idx_commits_repo_addr_branch ON commits(repo_addr, repo_branch)`,
		`CREATE INDEX IF NOT EXISTS idx_commits_user_id ON commits(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_commits_commit_time ON commits(commit_time)`,
		`CREATE INDEX IF NOT EXISTS idx_repos_repo_addr ON repos(repo_addr)`,
		`CREATE INDEX IF NOT EXISTS idx_repos_repo_addr_branch ON repos(repo_addr, repo_branch)`,
	}
	for _, idx := range indexes {
		if _, err := db.Exec(idx); err != nil {
			fmt.Fprintf(os.Stderr, "警告: 创建索引失败(可忽略): %v\n", err)
		}
	}

	return nil
}
