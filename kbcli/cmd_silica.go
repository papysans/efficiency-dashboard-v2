package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/lib/pq"

	"github.com/spf13/cobra"
)

type silicaTaskSummary struct {
	TaskID    string `json:"task_id"`
	RepoAddr  string `json:"repo_addr"`
	UserID    string `json:"user_id"`
	StartTime string `json:"start_time"`
}

type taskMeta struct {
	taskID    string
	startTime time.Time
}

type groupKey struct {
	repoAddr string
	userID   string
}

type silicaIndex struct {
	groupHashes map[groupKey]map[string]map[string]bool
	taskMetas   map[groupKey][]taskMeta
	taskCount   int
	hashCount   int
}

var silicaCmd = &cobra.Command{
	Use:   "silica",
	Short: "计算commit的含硅量（task贡献比例），更新commits表的task_ids和task_ids_silica",
	RunE: func(cmd *cobra.Command, args []string) error {
		analysedDir, _ := cmd.Flags().GetString("analysed-dir")
		reprocess, _ := cmd.Flags().GetBool("reprocess")

		taskFPDir := filepath.Join(analysedDir, "task", "summary")
		repoFPDir := filepath.Join(analysedDir, "repo")

		db, err := sql.Open("postgres", cfg.StatDatabase.DSN())
		if err != nil {
			return fmt.Errorf("连接数据库失败: %w", err)
		}
		defer db.Close()

		if err := db.Ping(); err != nil {
			return fmt.Errorf("数据库连接测试失败: %w", err)
		}

		if err := ensureSilicaColumns(db); err != nil {
			return fmt.Errorf("确保commits表字段失败: %w", err)
		}

		idx, err := buildSilicaIndex(taskFPDir)
		if err != nil {
			return fmt.Errorf("构建task指纹索引失败: %w", err)
		}
		fmt.Printf("已加载task指纹索引: %d个task, %d个分组, %d个哈希\n", idx.taskCount, len(idx.groupHashes), idx.hashCount)

		commitFPFiles, err := scanCommitFPFiles(repoFPDir)
		if err != nil {
			return fmt.Errorf("扫描commit指纹文件失败: %w", err)
		}
		fmt.Printf("找到%d个commit指纹文件\n", len(commitFPFiles))

		successCount := 0
		skipCount := 0
		failCount := 0

		for _, fpFile := range commitFPFiles {
			commitID := strings.TrimSuffix(filepath.Base(fpFile), ".fp")

			if !reprocess {
				var taskIDsJSON string
				err := db.QueryRow(`SELECT task_ids FROM commits WHERE commit_id = $1`, commitID).Scan(&taskIDsJSON)
				if err == nil && taskIDsJSON != "" && taskIDsJSON != "null" && taskIDsJSON != "[]" {
					skipCount++
					continue
				}
			}

			var repoAddr, userID string
			var commitTime time.Time
			err := db.QueryRow(`SELECT repo_addr, user_id, commit_time FROM commits WHERE commit_id = $1`, commitID).Scan(&repoAddr, &userID, &commitTime)
			if err != nil {
				if err == sql.ErrNoRows {
					fmt.Fprintf(os.Stderr, "警告: commit不存在于数据库 [%s]\n", commitID)
					skipCount++
					continue
				}
				fmt.Fprintf(os.Stderr, "查询commit元数据失败 [%s]: %v\n", commitID, err)
				failCount++
				continue
			}

			gk := groupKey{repoAddr: repoAddr, userID: userID}
			candidateHashes := idx.buildCandidateHashIndex(gk, commitTime)

			taskIDSilica, totalLines, err := computeCommitSilica(fpFile, candidateHashes)
			if err != nil {
				fmt.Fprintf(os.Stderr, "计算含硅量失败 [%s]: %v\n", commitID, err)
				failCount++
				continue
			}

			if len(taskIDSilica) == 0 {
				if _, err := db.Exec(`UPDATE commits SET task_ids = '[]'::jsonb, task_ids_silica = '[]'::jsonb, silica = 0, updated_at = CURRENT_TIMESTAMP WHERE commit_id = $1`, commitID); err != nil {
					fmt.Fprintf(os.Stderr, "更新commits表失败 [%s]: %v\n", commitID, err)
					failCount++
				} else {
					successCount++
				}
				continue
			}

			var taskIDList []string
			var silicaList []float64
			var totalSilica float64
			for _, ts := range taskIDSilica {
				taskIDList = append(taskIDList, ts.TaskID)
				silicaList = append(silicaList, ts.Silica)
				totalSilica += ts.Silica
			}

			taskIDsJSON, _ := json.Marshal(taskIDList)
			silicaJSON, _ := json.Marshal(silicaList)

			if _, err := db.Exec(`UPDATE commits SET task_ids = $1::jsonb, task_ids_silica = $2::jsonb, silica = $3, updated_at = CURRENT_TIMESTAMP WHERE commit_id = $4`,
				string(taskIDsJSON), string(silicaJSON), totalSilica, commitID); err != nil {
				fmt.Fprintf(os.Stderr, "更新commits表失败 [%s]: %v\n", commitID, err)
				failCount++
			} else {
				matched := countMatchedLines(fpFile, candidateHashes)
				fmt.Printf("  %s: silica=%.4f (%d/%d行匹配), candidates=%d, tasks=%v\n", commitID, totalSilica, matched, totalLines, len(idx.taskMetas[gk]), taskIDList)
				successCount++
			}
		}

		fmt.Printf("含硅量计算完成: 成功 %d 个，跳过 %d 个，失败 %d 个\n", successCount, skipCount, failCount)
		return nil
	},
}

type taskSilica struct {
	TaskID string
	Silica float64
}

func init() {
	silicaCmd.Flags().String("analysed-dir", "./analysed", "已分析文件目录路径")
	silicaCmd.Flags().Bool("reprocess", false, "重新处理已计算过的commit")
	rootCmd.AddCommand(silicaCmd)
}

func ensureSilicaColumns(db *sql.DB) error {
	alterations := []string{
		`DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'commits' AND column_name = 'task_ids') THEN ALTER TABLE commits ADD COLUMN task_ids JSONB; END IF; END $$`,
		`DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'commits' AND column_name = 'task_ids_silica') THEN ALTER TABLE commits ADD COLUMN task_ids_silica JSONB; END IF; END $$`,
		`DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'commits' AND column_name = 'silica') THEN ALTER TABLE commits ADD COLUMN silica FLOAT8 DEFAULT 0; END IF; END $$`,
	}
	for _, sql := range alterations {
		if _, err := db.Exec(sql); err != nil {
			return fmt.Errorf("执行ALTER TABLE失败: %w", err)
		}
	}
	return nil
}

func buildSilicaIndex(taskFPDir string) (*silicaIndex, error) {
	idx := &silicaIndex{
		groupHashes: make(map[groupKey]map[string]map[string]bool),
		taskMetas:   make(map[groupKey][]taskMeta),
	}

	if _, err := os.Stat(taskFPDir); os.IsNotExist(err) {
		return idx, nil
	}

	var fpFiles []string
	err := filepath.Walk(taskFPDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".fp") {
			fpFiles = append(fpFiles, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("扫描task指纹目录失败: %w", err)
	}

	for _, fpFile := range fpFiles {
		jsonFile := strings.TrimSuffix(fpFile, ".fp") + ".json"
		summary, err := extractTaskSummaryFromJSON(jsonFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "警告: 读取task摘要失败 [%s]: %v\n", jsonFile, err)
			continue
		}
		if summary.TaskID == "" {
			continue
		}

		hashes, err := loadFPHashes(fpFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "警告: 读取task指纹文件失败 [%s]: %v\n", fpFile, err)
			continue
		}

		gk := groupKey{repoAddr: summary.RepoAddr, userID: summary.UserID}

		if idx.groupHashes[gk] == nil {
			idx.groupHashes[gk] = make(map[string]map[string]bool)
		}
		for _, h := range hashes {
			if idx.groupHashes[gk][h] == nil {
				idx.groupHashes[gk][h] = make(map[string]bool)
			}
			idx.groupHashes[gk][h][summary.TaskID] = true
			idx.hashCount++
		}

		var startTime time.Time
		if summary.StartTime != "" {
			startTime, _ = time.Parse(time.RFC3339, summary.StartTime)
		}
		idx.taskMetas[gk] = append(idx.taskMetas[gk], taskMeta{
			taskID:    summary.TaskID,
			startTime: startTime,
		})
		idx.taskCount++
	}

	return idx, nil
}

func (idx *silicaIndex) buildCandidateHashIndex(gk groupKey, commitTime time.Time) map[string]map[string]bool {
	groupHashes := idx.groupHashes[gk]
	if len(groupHashes) == 0 {
		return nil
	}

	candidateTaskIDs := make(map[string]bool)
	metas := idx.taskMetas[gk]
	sevenDays := 7 * 24 * time.Hour
	for _, m := range metas {
		if m.startTime.IsZero() {
			continue
		}
		if m.startTime.Before(commitTime) && commitTime.Before(m.startTime.Add(sevenDays)) {
			candidateTaskIDs[m.taskID] = true
		}
	}

	if len(candidateTaskIDs) == 0 {
		return nil
	}

	result := make(map[string]map[string]bool)
	for hash, taskIDs := range groupHashes {
		for tid := range taskIDs {
			if candidateTaskIDs[tid] {
				if result[hash] == nil {
					result[hash] = make(map[string]bool)
				}
				result[hash][tid] = true
			}
		}
	}
	return result
}

func extractTaskSummaryFromJSON(jsonPath string) (*silicaTaskSummary, error) {
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, err
	}
	var summary silicaTaskSummary
	if err := json.Unmarshal(data, &summary); err != nil {
		return nil, err
	}
	return &summary, nil
}

func loadFPHashes(fpPath string) ([]string, error) {
	f, err := os.Open(fpPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var hashes []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		hashes = append(hashes, line)
	}
	return hashes, scanner.Err()
}

func scanCommitFPFiles(repoFPDir string) ([]string, error) {
	var files []string
	if _, err := os.Stat(repoFPDir); os.IsNotExist(err) {
		return files, nil
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

func computeCommitSilica(fpPath string, hashToTaskIDs map[string]map[string]bool) ([]taskSilica, int, error) {
	f, err := os.Open(fpPath)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	taskMatchedLines := make(map[string]int)
	totalLines := 0

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		totalLines++

		if taskIDs, ok := hashToTaskIDs[line]; ok {
			for taskID := range taskIDs {
				taskMatchedLines[taskID]++
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, 0, err
	}

	if totalLines == 0 || len(taskMatchedLines) == 0 {
		return nil, totalLines, nil
	}

	var result []taskSilica
	for taskID, matchedLines := range taskMatchedLines {
		silica := float64(matchedLines) / float64(totalLines)
		result = append(result, taskSilica{TaskID: taskID, Silica: silica})
	}

	return result, totalLines, nil
}

func countMatchedLines(fpPath string, hashToTaskIDs map[string]map[string]bool) int {
	f, err := os.Open(fpPath)
	if err != nil {
		return 0
	}
	defer f.Close()

	matched := 0
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if _, ok := hashToTaskIDs[line]; ok {
			matched++
		}
	}
	return matched
}
