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

type convMeta struct {
	taskID    string
	requestID string
	endTime   time.Time
}

type groupKey struct {
	repoAddr string
	userID   string
}

type groupIndexer struct {
	Lines map[string]int // 代码行指纹 -> Convs数组索引，映射到生成该行的对话
	Convs []convMeta     // 同属于该组的对话
}

type conversationsIndexer struct {
	groups    map[groupKey]groupIndexer
	convCount int
	hashCount int
}

type commitParser struct {
	fpHashs          []string
	taskIDs          []string
	taskSilicas      []float64
	totalLines       int
	totalMatchLines  int
	totalSilica      float64
	aiMinutes        float64
	ancientMinutes   float64
	upstreamTokens   int64
	downstreamTokens int64
	cost             float64
}

func buildCommitParser(fpPath string) *commitParser {
	fpHashs, err := loadFPHashes(fpPath)
	if err != nil {
		return nil
	}
	return &commitParser{
		fpHashs: fpHashs,
	}
}

func buildConversationsIndexer(taskFPDir string) (*conversationsIndexer, error) {
	idx := &conversationsIndexer{
		groups: make(map[groupKey]groupIndexer),
	}

	if _, err := os.Stat(taskFPDir); os.IsNotExist(err) {
		return idx, nil
	}

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

	for i, silicaFile := range silicaFiles {
		logPromptProgress(i, 50)

		tsd, err := loadTaskSilicaFile(silicaFile)
		if err != nil {
			logWarnf("读取task silica文件失败 [%s]: %v", silicaFile, err)
			continue
		}
		if tsd.TaskID == "" || tsd.RepoAddr == "" {
			continue
		}

		gk := groupKey{repoAddr: tsd.RepoAddr, userID: tsd.UserID}

		gi, exists := idx.groups[gk]
		if !exists {
			gi = groupIndexer{
				Lines: make(map[string]int),
				Convs: make([]convMeta, 0),
			}
		}

		for _, conv := range tsd.Conversations {
			if conv.RequestID == "" {
				continue
			}

			var endTime time.Time
			if conv.EndTime != "" {
				endTime, _ = time.Parse(time.RFC3339, conv.EndTime)
			}

			cm := convMeta{
				taskID:    tsd.TaskID,
				requestID: conv.RequestID,
				endTime:   endTime,
			}

			convIdx := len(gi.Convs)
			gi.Convs = append(gi.Convs, cm)

			for _, h := range conv.Fingerprints {
				prevIdx, prevExists := gi.Lines[h]
				if !prevExists || endTime.After(gi.Convs[prevIdx].endTime) {
					gi.Lines[h] = convIdx
					if !prevExists {
						idx.hashCount++
					}
				}
			}

			idx.convCount++
		}

		idx.groups[gk] = gi
	}

	return idx, nil
}

func loadTaskSilicaFile(path string) (*taskSilicaData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var tsd taskSilicaData
	if err := json.Unmarshal(data, &tsd); err != nil {
		return nil, err
	}
	return &tsd, nil
}

func (idx *conversationsIndexer) buildCandidateHashIndex(gk groupKey, commitTime time.Time, maxDays int) map[string]convMeta {
	gi, exists := idx.groups[gk]
	if !exists || len(gi.Lines) == 0 {
		return nil
	}

	maxDuration := time.Duration(maxDays) * 24 * time.Hour
	candidateIndices := make(map[int]bool)
	for i, m := range gi.Convs {
		if m.endTime.IsZero() {
			continue
		}
		if m.endTime.Before(commitTime) && commitTime.Sub(m.endTime) <= maxDuration {
			candidateIndices[i] = true
		}
	}

	if len(candidateIndices) == 0 {
		return nil
	}

	result := make(map[string]convMeta)
	for hash, convIdx := range gi.Lines {
		if candidateIndices[convIdx] {
			result[hash] = gi.Convs[convIdx]
		}
	}
	return result
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

func (p *commitParser) computeCommitSilica(hashToConv map[string]convMeta) {
	convMatchedLines := make(map[string]float64)
	p.totalLines = len(p.fpHashs)
	matchedLines := 0

	for _, hash := range p.fpHashs {
		if cm, ok := hashToConv[hash]; ok {
			key := cm.taskID + "|" + cm.requestID
			convMatchedLines[key] += 1.0
			matchedLines++
		}
	}

	p.totalMatchLines = matchedLines
	p.taskIDs = make([]string, 0)
	p.taskSilicas = make([]float64, 0)
	p.totalSilica = 0

	if p.totalLines == 0 || len(convMatchedLines) == 0 {
		return
	}

	taskSilicaMap := make(map[string]float64)
	for convKey, matched := range convMatchedLines {
		silica := matched / float64(p.totalLines)
		parts := strings.SplitN(convKey, "|", 2)
		taskID := parts[0]
		taskSilicaMap[taskID] += silica
	}

	for taskID, silica := range taskSilicaMap {
		p.taskIDs = append(p.taskIDs, taskID)
		p.taskSilicas = append(p.taskSilicas, silica)
		p.totalSilica += silica
	}
}

func (p *commitParser) calcCommitDerivedMinutes(db *gorm.DB) error {
	for i, taskID := range p.taskIDs {
		var task Task
		if err := db.Select("COALESCE(task_real_minutes_manual, task_real_minutes) as task_real_minutes, COALESCE(task_ancient_minutes_manual, task_ancient_minutes) as task_ancient_minutes, upstream_tokens, downstream_tokens, cost").
			Where("task_id = ?", taskID).First(&task).Error; err != nil {
			if err != gorm.ErrRecordNotFound {
				logWarnf("查询task数据失败 [%s]: %v", taskID, err)
			}
			continue
		}
		silica := p.taskSilicas[i]
		p.aiMinutes += task.TaskRealMinutes * silica
		p.ancientMinutes += task.TaskAncientMinutes * (1 - silica)
		p.upstreamTokens += task.UpstreamTokens
		p.downstreamTokens += task.DownstreamTokens
		p.cost += task.Cost
	}
	return nil
}

func runSilica(analysedDir string, force bool, maxDays int) error {
	startTime := time.Now()
	taskFPDir := filepath.Join(analysedDir, "task", "summary")
	repoFPDir := filepath.Join(analysedDir, "repo")

	if _, err := os.Stat(taskFPDir); os.IsNotExist(err) {
		recordCommandRun("silica", startTime, 0, 0, 0, err)
		return fmt.Errorf("task指纹目录不存在: %s", taskFPDir)
	}

	db, err := openGormDB(cfg.StatDatabase.DSN())
	if err != nil {
		recordCommandRun("silica", startTime, 0, 0, 0, err)
		return fmt.Errorf("连接数据库失败: %w", err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	idx, err := buildConversationsIndexer(taskFPDir)
	if err != nil {
		recordCommandRun("silica", startTime, 0, 0, 0, err)
		return fmt.Errorf("构建conversation指纹索引失败: %w", err)
	}
	logInfof("已加载conversation指纹索引: %d个conversation, %d个分组, %d个哈希, max_days=%d", idx.convCount, len(idx.groups), idx.hashCount, maxDays)

	commitFPFiles, err := scanCommitFPFiles(repoFPDir)
	if err != nil {
		recordCommandRun("silica", startTime, 0, 0, 0, err)
		return fmt.Errorf("扫描commit指纹文件失败: %w", err)
	}
	logInfof("找到%d个commit指纹文件", len(commitFPFiles))

	successCount := 0
	skipCount := 0
	failCount := 0

	for i, fpFile := range commitFPFiles {
		logPromptProgress(i, 50)

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
		commitPs := buildCommitParser(fpFile)
		if commitPs == nil {
			logWarnf("读取commit指纹文件失败 [%s]", fpFile)
			skipCount++
			continue
		}

		var commit Commit
		if err := db.Select("repo_addr, user_id, commit_time").Where("commit_id = ?", commitID).First(&commit).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				logWarnf("commit不存在于数据库 [%s]", commitID)
				skipCount++
				continue
			}
			logWarnf("查询commit元数据失败 [%s]: %v", commitID, err)
			failCount++
			continue
		}

		gk := groupKey{repoAddr: commit.RepoAddr, userID: commit.UserID}
		if commit.CommitTime == nil {
			logErrorf("Commit [%s] 缺少提交时间", commitID)
			continue
		}
		candidateHashes := idx.buildCandidateHashIndex(gk, *commit.CommitTime, maxDays)
		commitPs.computeCommitSilica(candidateHashes)

		if len(commitPs.taskIDs) == 0 {
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
				logWarnf("更新commits表失败 [%s]: %v", commitID, err)
				failCount++
			} else {
				successCount++
			}
			continue
		}

		if err := commitPs.calcCommitDerivedMinutes(db); err != nil {
			logWarnf("计算衍生数据失败 [%s]: %v", commitID, err)
			failCount++
			continue
		}
		commitRealMinutes := commitPs.aiMinutes + commitPs.ancientMinutes

		taskIDsJSON, _ := json.Marshal(commitPs.taskIDs)
		silicaJSON, _ := json.Marshal(commitPs.taskSilicas)

		if err := db.Model(&Commit{}).Where("commit_id = ?", commitID).Updates(map[string]interface{}{
			"task_ids":                    string(taskIDsJSON),
			"task_ids_silica":             string(silicaJSON),
			"silica":                      commitPs.totalSilica,
			"commit_real_ai_minutes":      commitPs.aiMinutes,
			"commit_real_ancient_minutes": commitPs.ancientMinutes,
			"commit_real_minutes":         commitRealMinutes,
			"upstream_tokens":             commitPs.upstreamTokens,
			"downstream_tokens":           commitPs.downstreamTokens,
			"cost":                        commitPs.cost,
		}).Error; err != nil {
			logWarnf("更新commits表失败 [%s]: %v", commitID, err)
			failCount++
		} else {
			logDebugf("  %s: silica=%.4f (%d/%d行匹配), ai=%.1fmin, ancient=%.1fmin, total=%.1fmin",
				commitID, commitPs.totalSilica, commitPs.totalMatchLines, commitPs.totalLines, commitPs.aiMinutes, commitPs.ancientMinutes, commitRealMinutes)
			successCount++
		}
	}

	logInfof("含硅量计算完成: 成功 %d 个，跳过 %d 个，失败 %d 个", successCount, skipCount, failCount)
	recordCommandRun("silica", startTime, successCount, failCount, skipCount, nil)
	return nil
}

var silicaCmd = &cobra.Command{
	Use:   "silica",
	Short: "计算commit的含硅量（task贡献比例），更新commits表的task_ids和task_ids_silica",
	RunE: func(cmd *cobra.Command, args []string) error {
		analysedDir, _ := cmd.Flags().GetString("analysed-dir")
		force, _ := cmd.Flags().GetBool("force")
		maxDays, _ := cmd.Flags().GetInt("max-days")
		remote, _ := cmd.Flags().GetString("remote")

		if remote != "" {
			return sendToRemote(remote, "silica", map[string]interface{}{
				"analysed_dir": analysedDir,
				"force":        force,
				"max_days":     maxDays,
			})
		}

		if analysedDir == "" {
			analysedDir = cfg.AnalysedDir
		}
		if maxDays <= 0 {
			maxDays = cfg.SilicaMaxDays
		}

		return runSilica(analysedDir, force, maxDays)
	},
}

func init() {
	silicaCmd.Flags().SortFlags = false
	silicaCmd.Flags().String("analysed-dir", "", "已分析文件目录路径")
	silicaCmd.Flags().BoolP("force", "f", false, "强制重新计算，覆盖已存在数据")
	silicaCmd.Flags().Int("max-days", 0, "对话结束后多少天内的commit算相关（默认从config读取）")
	silicaCmd.Flags().String("remote", "", "远程kbcli服务地址（如 http://127.0.0.1:8080），指定后命令将发送到远程执行")
	rootCmd.AddCommand(silicaCmd)
}
