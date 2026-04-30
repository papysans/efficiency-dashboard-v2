package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/spf13/cobra"
)

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
	WorkDir      string `json:"work_dir,omitempty"`
	Comment      string `json:"comment"`
	DiffLines    int    `json:"diff_lines"`
	Diff         string `json:"diff"`
}

type repoFileMeta struct {
	Path     string
	RelPath  string
	Repo     string
	Branch   string
	Year     string
	Month    string
	Day      string
	CommitID string
}

var reRepoPath = regexp.MustCompile(`^([^/]+)/([^/]+)/(\d{4})/(\d{2})/(\d{2})/([^/]+)\.json$`)

func scanRepoDir(repoDir, analysedDir string, force bool) ([]repoFileMeta, int, error) {
	var files []repoFileMeta
	skipCount := 0

	err := filepath.Walk(repoDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

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
			CommitID: matches[6],
		}

		if !force {
			fpPath := fpPathForMeta(analysedDir, meta)
			if _, err := os.Stat(fpPath); err == nil {
				logDebugf("跳过(已处理): %s", path)
				skipCount++
				return nil
			}
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

func importCommitFile(db *gorm.DB, meta repoFileMeta, analysedDir string) error {
	data, err := os.ReadFile(meta.Path)
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}

	var commitData RepoCommitData
	if err := json.Unmarshal(data, &commitData); err != nil {
		return fmt.Errorf("解析JSON失败: %w", err)
	}

	if commitData.CommitID == "" {
		return fmt.Errorf("commit_id为空")
	}
	if commitData.CommitTime == "" {
		return fmt.Errorf("commit_time为空")
	}

	workDir := commitData.WorkDir
	if workDir == "" {
		workDir = commitData.WorkPath
	}

	commitTime, err := time.Parse(time.RFC3339, commitData.CommitTime)
	if err != nil {
		return fmt.Errorf("解析commit_time失败: %w", err)
	}

	commit := Commit{
		CommitID:      commitData.CommitID,
		CommitTime:    &commitTime,
		RepoAddr:      commitData.RepoAddr,
		RepoBranch:    commitData.RepoBranch,
		GitUserName:   commitData.GitUserName,
		GitUserEmail:  commitData.GitUserEmail,
		UserID:        commitData.UserID,
		UserName:      commitData.UserName,
		ClientID:      commitData.ClientID,
		WorkDir:       workDir,
		Comment:       commitData.Comment,
		DiffLines:     commitData.DiffLines,
		TaskIDs:       StringJSON("[]"),
		TaskIDsSilica: StringJSON("[]"),
	}

	result := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "commit_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"commit_time", "repo_addr", "repo_branch",
			"git_user_name", "git_user_email", "user_id", "user_name",
			"client_id", "work_dir", "comment", "diff_lines", "updated_at",
		}),
	}).Create(&commit)
	if result.Error != nil {
		return fmt.Errorf("写入commits表失败: %w", result.Error)
	}

	ancientMinutes, ancientReason := importEstimateCommitAncientMinutes(commitData.DiffLines)
	ancientMinutesPtr := &ancientMinutes
	ancientReasonPtr := &ancientReason
	if err := db.Model(&Commit{}).
		Where("commit_id = ? AND commit_ancient_minutes IS NULL AND commit_ancient_minutes_manual IS NULL", commitData.CommitID).
		Updates(map[string]interface{}{
			"commit_ancient_minutes":        ancientMinutesPtr,
			"commit_ancient_minutes_reason": ancientReasonPtr,
		}).Error; err != nil {
		logWarnf("更新commit_ancient_minutes失败 [%s]: %v", commitData.CommitID, err)
	} else {
		logDebugf("  commit_ancient_minutes=%.1f (%s)", ancientMinutes, ancientReason)
	}

	addedLines := extractAddedLinesFromDiff(commitData.Diff)
	fpPath := fpPathForMeta(analysedDir, meta)

	if err := writeFingerprintsToFile(addedLines, fpPath); err != nil {
		return fmt.Errorf("写入fp文件失败 [%s]: %w", fpPath, err)
	}

	logInfof("导入成功: %s (新增行指纹: %d)", commitData.CommitID, len(addedLines))
	return nil
}

func removeWhitespace(s string) string {
	return strings.Map(func(r rune) rune {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			return -1
		}
		return r
	}, s)
}

func writeFingerprintsToFile(addedLines []addedLine, fpPath string) error {
	fpDir := filepath.Dir(fpPath)
	if err := os.MkdirAll(fpDir, 0755); err != nil {
		return fmt.Errorf("创建fp目录失败: %w", err)
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
	return nil
}

func fpPathForMeta(analysedDir string, meta repoFileMeta) string {
	return filepath.Join(analysedDir, "repo", meta.Repo, meta.Branch, meta.Year, meta.Month, meta.Day, meta.CommitID+".fp")
}

func runImportRepo(repoDir, analysedDir string, force bool) error {
	if _, err := os.Stat(repoDir); os.IsNotExist(err) {
		return fmt.Errorf("repo目录不存在: %s", repoDir)
	}

	db, err := openGormDB(cfg.StatDatabase.DSN())
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	files, skipCount, err := scanRepoDir(repoDir, analysedDir, force)
	if err != nil {
		return fmt.Errorf("扫描repo目录失败: %w", err)
	}

	if len(files) == 0 {
		logInfo("没有找到待导入的commit文件")
		return nil
	}

	logInfof("找到 %d 个待导入的commit文件", len(files))

	successCount := 0
	failCount := 0

	for _, fileMeta := range files {
		if err := importCommitFile(db, fileMeta, analysedDir); err != nil {
			logWarnf("导入失败 [%s]: %v", fileMeta.Path, err)
			failCount++
		} else {
			successCount++
		}
	}

	logInfof("导入完成: 成功 %d 个，失败 %d 个，跳过 %d 个", successCount, failCount, skipCount)

	return nil
}

var importRepoCmd = &cobra.Command{
	Use:   "import-repo",
	Short: "导入客户端上报的repo数据到commits表",
	Long:  "扫描指定repo目录下的commit JSON文件，批量写入commits表",
	RunE: func(cmd *cobra.Command, args []string) error {
		repoDir, _ := cmd.Flags().GetString("repo-dir")
		analysedDir, _ := cmd.Flags().GetString("analysed-dir")
		force, _ := cmd.Flags().GetBool("force")

		if repoDir == "" {
			repoDir = cfg.RepoDir
		}
		if analysedDir == "" {
			analysedDir = cfg.AnalysedDir
		}

		return runImportRepo(repoDir, analysedDir, force)
	},
}

func init() {
	importRepoCmd.Flags().SortFlags = false
	importRepoCmd.Flags().String("repo-dir", "", "repo 目录路径")
	importRepoCmd.Flags().String("analysed-dir", "", "已处理文件的输出目录")
	importRepoCmd.Flags().BoolP("force", "f", false, "强制重新导入，覆盖已存在数据")
	rootCmd.AddCommand(importRepoCmd)
}
