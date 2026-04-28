package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/spf13/cobra"
)

type taskAddressing struct {
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
	groupHashes map[groupKey]map[string]string
	taskMetas   map[groupKey][]taskMeta
	taskCount   int
	hashCount   int
}

type taskSilica struct {
	TaskID string
	Silica float64
}

func buildSilicaIndex(taskFPDir string) (*silicaIndex, error) {
	idx := &silicaIndex{
		groupHashes: make(map[groupKey]map[string]string),
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
		addr, err := extractTaskAddressingFromJSON(jsonFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "警告: 读取task摘要失败 [%s]: %v\n", jsonFile, err)
			continue
		}
		if addr.TaskID == "" {
			continue
		}

		hashes, err := loadFPHashes(fpFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "警告: 读取task指纹文件失败 [%s]: %v\n", fpFile, err)
			continue
		}

		gk := groupKey{repoAddr: addr.RepoAddr, userID: addr.UserID}

		if idx.groupHashes[gk] == nil {
			idx.groupHashes[gk] = make(map[string]string)
		}
		for _, h := range hashes {
			if idx.groupHashes[gk][h] == "" {
				idx.groupHashes[gk][h] = addr.TaskID
				idx.hashCount++
			}
		}

		var startTime time.Time
		if addr.StartTime != "" {
			startTime, _ = time.Parse(time.RFC3339, addr.StartTime)
		}
		idx.taskMetas[gk] = append(idx.taskMetas[gk], taskMeta{
			taskID:    addr.TaskID,
			startTime: startTime,
		})
		idx.taskCount++
	}

	return idx, nil
}

func (idx *silicaIndex) buildCandidateHashIndex(gk groupKey, commitTime time.Time) map[string]string {
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

	result := make(map[string]string)
	for hash, taskID := range groupHashes {
		if candidateTaskIDs[taskID] {
			result[hash] = taskID
		}
	}
	return result
}

func extractTaskAddressingFromJSON(jsonPath string) (*taskAddressing, error) {
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, err
	}
	var addr taskAddressing
	if err := json.Unmarshal(data, &addr); err != nil {
		return nil, err
	}
	return &addr, nil
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

func computeCommitSilica(fpPath string, hashToTaskID map[string]string) ([]taskSilica, int, error) {
	f, err := os.Open(fpPath)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	taskMatchedLines := make(map[string]float64)
	totalLines := 0

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		totalLines++

		if taskID, ok := hashToTaskID[line]; ok {
			taskMatchedLines[taskID] += 1.0
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
		silica := matchedLines / float64(totalLines)
		result = append(result, taskSilica{TaskID: taskID, Silica: silica})
	}

	return result, totalLines, nil
}

func calcCommitDerivedMinutes(db *gorm.DB, taskIDSilica []taskSilica) (float64, float64, int64, int64, float64) {
	var aiMinutes, ancientMinutes float64
	var upstreamTokens, downstreamTokens int64
	var cost float64
	for _, ts := range taskIDSilica {
		var task Task
		if err := db.Select("COALESCE(task_real_minutes_manual, task_real_minutes) as task_real_minutes, COALESCE(task_ancient_minutes_manual, task_ancient_minutes) as task_ancient_minutes, upstream_tokens, downstream_tokens, cost").
			Where("task_id = ?", ts.TaskID).First(&task).Error; err != nil {
			if err != gorm.ErrRecordNotFound {
				fmt.Fprintf(os.Stderr, "查询task数据失败 [%s]: %v\n", ts.TaskID, err)
			}
			continue
		}
		aiMinutes += task.TaskRealMinutes * ts.Silica
		ancientMinutes += task.TaskAncientMinutes * (1 - ts.Silica)
		upstreamTokens += task.UpstreamTokens
		downstreamTokens += task.DownstreamTokens
		cost += task.Cost
	}
	return aiMinutes, ancientMinutes, upstreamTokens, downstreamTokens, cost
}

func countMatchedLines(fpPath string, hashToTaskID map[string]string) int {
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
		if _, ok := hashToTaskID[line]; ok {
			matched++
		}
	}
	return matched
}

func runSilica(analysedDir string, force bool) error {
	taskFPDir := filepath.Join(analysedDir, "task", "summary")
	repoFPDir := filepath.Join(analysedDir, "repo")

	db, err := openGormDB(cfg.StatDatabase.DSN())
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

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

		if !force {
			var commit Commit
			if err := db.Select("task_ids").Where("commit_id = ?", commitID).First(&commit).Error; err == nil {
				if string(commit.TaskIDs) != "" && string(commit.TaskIDs) != "null" && string(commit.TaskIDs) != "[]" {
					skipCount++
					continue
				}
			}
		}

		var commit Commit
		if err := db.Select("repo_addr, user_id, commit_time").Where("commit_id = ?", commitID).First(&commit).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				fmt.Fprintf(os.Stderr, "警告: commit不存在于数据库 [%s]\n", commitID)
				skipCount++
				continue
			}
			fmt.Fprintf(os.Stderr, "查询commit元数据失败 [%s]: %v\n", commitID, err)
			failCount++
			continue
		}

		gk := groupKey{repoAddr: commit.RepoAddr, userID: commit.UserID}
		commitTime := time.Time{}
		if commit.CommitTime != nil {
			commitTime = *commit.CommitTime
		}
		candidateHashes := idx.buildCandidateHashIndex(gk, commitTime)

		taskIDSilica, totalLines, err := computeCommitSilica(fpFile, candidateHashes)
		if err != nil {
			fmt.Fprintf(os.Stderr, "计算含硅量失败 [%s]: %v\n", commitID, err)
			failCount++
			continue
		}

		if len(taskIDSilica) == 0 {
			if err := db.Model(&Commit{}).Where("commit_id = ?", commitID).Updates(map[string]interface{}{
				"task_ids":                    "[]",
				"task_ids_silica":             "[]",
				"silica":                      0,
				"commit_real_ai_minutes":      0,
				"commit_real_ancient_minutes": gorm.Expr("COALESCE(commit_ancient_minutes, 0)"),
				"commit_real_minutes":         gorm.Expr("COALESCE(commit_ancient_minutes, 0)"),
				"upstream_tokens":             0,
				"downstream_tokens":           0,
				"cost":                        0,
			}).Error; err != nil {
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

		commitRealAIMinutes, commitRealAncientMinutes, upstreamTokens, downstreamTokens, cost := calcCommitDerivedMinutes(db, taskIDSilica)
		commitRealMinutes := commitRealAIMinutes + commitRealAncientMinutes

		taskIDsJSON, _ := json.Marshal(taskIDList)
		silicaJSON, _ := json.Marshal(silicaList)

		if err := db.Model(&Commit{}).Where("commit_id = ?", commitID).Updates(map[string]interface{}{
			"task_ids":                    string(taskIDsJSON),
			"task_ids_silica":             string(silicaJSON),
			"silica":                      totalSilica,
			"commit_real_ai_minutes":      commitRealAIMinutes,
			"commit_real_ancient_minutes": commitRealAncientMinutes,
			"commit_real_minutes":         commitRealMinutes,
			"upstream_tokens":             upstreamTokens,
			"downstream_tokens":           downstreamTokens,
			"cost":                        cost,
		}).Error; err != nil {
			fmt.Fprintf(os.Stderr, "更新commits表失败 [%s]: %v\n", commitID, err)
			failCount++
		} else {
			matched := countMatchedLines(fpFile, candidateHashes)
			fmt.Printf("  %s: silica=%.4f (%d/%d行匹配), ai=%.1fmin, ancient=%.1fmin, total=%.1fmin\n",
				commitID, totalSilica, matched, totalLines, commitRealAIMinutes, commitRealAncientMinutes, commitRealMinutes)
			successCount++
		}
	}

	fmt.Printf("含硅量计算完成: 成功 %d 个，跳过 %d 个，失败 %d 个\n", successCount, skipCount, failCount)
	return nil
}

var silicaCmd = &cobra.Command{
	Use:   "silica",
	Short: "计算commit的含硅量（task贡献比例），更新commits表的task_ids和task_ids_silica",
	RunE: func(cmd *cobra.Command, args []string) error {
		analysedDir, _ := cmd.Flags().GetString("analysed-dir")
		force, _ := cmd.Flags().GetBool("force")

		if analysedDir == "" {
			analysedDir = cfg.AnalysedDir
		}

		return runSilica(analysedDir, force)
	},
}

func init() {
	silicaCmd.Flags().SortFlags = false
	silicaCmd.Flags().String("analysed-dir", "", "已分析文件目录路径")
	silicaCmd.Flags().Bool("force", false, "强制重新计算，覆盖已存在数据")
	rootCmd.AddCommand(silicaCmd)
}
