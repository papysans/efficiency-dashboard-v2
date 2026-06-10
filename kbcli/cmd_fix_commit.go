package main

import (
	"encoding/json"
	"fmt"
	"kanban/kbcli/internal/llm"
	"kanban/kbcli/internal/logx"
	"path/filepath"
	"time"

	"kanban/core/models"
	"kanban/core/storage"
	"kanban/core/utils"

	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

var fixCommitCmd = &cobra.Command{
	Use:   "fix-commit",
	Short: "使用AI为Commit记录补充传统耗时估算",
	Long:  `扫描repo目录，对commit_ancient_minutes_manual为空的commit进行AI耗时估算。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		repoDir, _ := cmd.Flags().GetString("repo-dir")
		startDate, _ := cmd.Flags().GetString("start-date")
		endDate, _ := cmd.Flags().GetString("end-date")
		date, _ := cmd.Flags().GetString("date")
		commitID, _ := cmd.Flags().GetString("commit")
		max, _ := cmd.Flags().GetInt("max")

		if repoDir == "" {
			repoDir = cfg.RepoDir
		}

		// 未显式传 start-date 且非单日(date)/单提交(commit)模式时，套全局分析起始日下界。
		if date == "" && commitID == "" {
			startDate = applyAnalysisFloor(startDate)
		}

		return runFixCommit(repoDir, startDate, endDate, date, commitID, max)
	},
}

func runFixCommit(repoDir, startDateStr, endDateStr, dateStr, commitID string, max int) error {
	db, err := models.OpenGormDB(cfg.StatDatabase.DSN())
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	startDate, endDate, err := parseDateRange(startDateStr, endDateStr, dateStr)
	if err != nil {
		return err
	}

	logx.Info("===== 开始处理commit估算 =====")
	if err := fixCommits(db, repoDir, startDate, endDate, commitID, max); err != nil {
		return fmt.Errorf("处理commit失败: %w", err)
	}

	logx.Info("===== fix-commit 命令执行完成 =====")
	return nil
}

func fixCommits(db *gorm.DB, repoDir string, startDate, endDate *time.Time, commitID string, max int) error {
	var commits []models.Commit
	query := db.Where("commit_ancient_minutes_manual IS NULL")
	if commitID != "" {
		query = db.Where("commit_id = ?", commitID)
	}
	if startDate != nil {
		query = query.Where("commit_time >= ?", *startDate)
	}
	if endDate != nil {
		query = query.Where("commit_time < ?", *endDate)
	}
	if err := query.Find(&commits).Error; err != nil {
		return fmt.Errorf("查询commit失败: %w", err)
	}

	if len(commits) == 0 {
		logx.Info("没有需要处理的commit记录")
		return nil
	}

	logx.Infof("找到 %d 个commit记录待处理", len(commits))

	var checked, success int
	for _, commit := range commits {
		if max > 0 && checked >= max {
			logx.Infof("已达到最大处理数量 %d，停止", max)
			break
		}
		checked++

		relPath := filepath.Join(
			utils.ToPathSafeID(commit.RepoAddr),
			utils.ToPathSafeID(commit.RepoBranch),
			commit.CommitTime.Format("2006"),
			commit.CommitTime.Format("01"),
			commit.CommitTime.Format("02"),
			commit.CommitId+".json",
		)
		path := storage.Join(repoDir, relPath)

		data, err := storage.ReadFile(path)
		if err != nil {
			logx.Errorf("读取commit文件失败: %s, %v", path, err)
			continue
		}

		var commitData RepoCommitData
		if err := json.Unmarshal(data, &commitData); err != nil {
			logx.Errorf("解析commit JSON失败: %s, %v", path, err)
			continue
		}

		minutes, reason, err := callAIForCommitEstimation(commitData.Comment, commitData.Diff, commitData.DiffLines)
		if err != nil {
			logx.Errorf("AI估算commit失败: %s, %v", commit.CommitId, err)
			continue
		}

		if err := updateCommitAncientEstimation(db, commit.CommitId, minutes, reason); err != nil {
			logx.Errorf("更新commit估算结果失败: %s, %v", commit.CommitId, err)
			continue
		}

		success++
		logx.Infof("commit估时完成: %s, minutes=%.1f", commit.CommitId, minutes)
	}

	logx.Infof("commit处理完成: 检查 %d, 成功 %d", checked, success)
	return nil
}

func callAIForCommitEstimation(comment, diff string, diffLines int) (float64, string, error) {
	aiCfg := cfg.AIEstimation
	if !aiCfg.Enabled || aiCfg.APIKey == "" {
		return 0, "", fmt.Errorf("AI estimation not enabled or API key missing")
	}

	prompt := fmt.Sprintf(`你是一个经验丰富的软件项目经理，擅长评估代码审查和合并的开发工作量。

请根据以下 Git commit 信息，估算如果由传统人工方式（不使用AI）完成该提交所需的**分钟数**。

重点关注：
1. 提交说明中描述的修改复杂度
2. 涉及的功能模块数量
3. 技术难度（如是否需要处理并发、安全、性能等问题）
4. 代码变更规模

提交说明：
%s

代码变更（diff）：
%s

总变更代码行数：%d

请输出 JSON 格式：
{
  "ancient_minutes": 30,
  "ancient_reason": "估算理由..."
}`,
		truncateString(comment, 2000),
		truncateString(diff, 8000),
		diffLines,
	)

	messages := []llm.ChatMessage{
		{Role: "system", Content: "请回答问题"},
		{Role: "user", Content: prompt},
	}
	content, err := llm.CallLLM(aiCfg, messages, 1024)
	if err != nil {
		return 0, "", err
	}

	jsonText := llm.ExtractJSON(content)
	var result struct {
		Minutes float64 `json:"ancient_minutes"`
		Reason  string  `json:"ancient_reason"`
	}
	if err := json.Unmarshal([]byte(jsonText), &result); err != nil {
		return 0, "", fmt.Errorf("解析估时结果失败: %w, text: %s", err, content)
	}

	if result.Minutes < 0 || result.Minutes > 100000 {
		return 0, "", fmt.Errorf("估时结果异常: %.2f", result.Minutes)
	}

	return result.Minutes, result.Reason, nil
}

func updateCommitAncientEstimation(db *gorm.DB, commitID string, minutes float64, reason string) error {
	result := db.Model(&models.Commit{}).Where("commit_id = ?", commitID).
		Updates(map[string]interface{}{
			"commit_ancient_minutes_manual": minutes,
			"commit_ancient_reason_manual":  reason,
			"updated_at":                    time.Now(),
		})
	return result.Error
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "...(截断)"
}

func init() {
	fixCommitCmd.Flags().SortFlags = false
	fixCommitCmd.Flags().String("repo-dir", "", "repo 目录路径，默认从配置文件获取")
	fixCommitCmd.Flags().String("start-date", "", "限定起始日期，格式 YYYYMMDD，为空则不限")
	fixCommitCmd.Flags().String("end-date", "", "限定结束日期，格式 YYYYMMDD，为空则不限")
	fixCommitCmd.Flags().String("date", "", "限定日期，格式 YYYYMMDD，限定活跃时间在该日期之内（与start-date/end-date互斥）)")
	fixCommitCmd.Flags().String("commit", "", "指定CommitId，只处理该Commit")
	fixCommitCmd.Flags().Int("max", 0, "最多处理多少个Commit，0表示不限制")
	rootCmd.AddCommand(fixCommitCmd)
}
