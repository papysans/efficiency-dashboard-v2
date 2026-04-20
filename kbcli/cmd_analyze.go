package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/spf13/cobra"
)

var safeIDRegex = regexp.MustCompile("[^a-zA-Z0-9]")

func makeSafeID(id string) string {
	return safeIDRegex.ReplaceAllString(id, "_")
}

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "分析命令（支持 git 子命令和维度分析）",
	RunE: func(cmd *cobra.Command, args []string) error {
		dimension, _ := cmd.Flags().GetString("dimension")
		id, _ := cmd.Flags().GetString("id")
		startDate, _ := cmd.Flags().GetString("start-date")
		endDate, _ := cmd.Flags().GetString("end-date")
		all, _ := cmd.Flags().GetBool("all")
		force, _ := cmd.Flags().GetBool("force")

		if dimension == "" || startDate == "" || endDate == "" {
			return fmt.Errorf("必须提供 --dimension, --start-date, --end-date 参数")
		}

		if dimension != "project" && dimension != "repo" {
			return fmt.Errorf("--dimension 必须是 project 或 repo，当前值: %s", dimension)
		}

		if id == "" && !all {
			return fmt.Errorf("必须提供 --id 或 --all 参数")
		}

		bc := NewBackendClient(cfg.BackendURL)

		if all {
			// 从 ES 获取所有唯一的 dimension ID
			esClient, err := NewESClient(cfg)
			if err != nil {
				return fmt.Errorf("创建 ES 客户端失败: %w", err)
			}

			indexNames := generateESIndexNames("costrict_chat_task_", startDate, endDate)
			field := dimension + "_id"
			ids, err := esClient.GetUniqueDimensionIDs(indexNames, field)
			if err != nil {
				return fmt.Errorf("获取 %s 列表失败: %w", dimension, err)
			}

			fmt.Printf("[Analyze] 共找到 %d 个 %s，开始逐个分析...\n", len(ids), dimension)
			var successCount, failCount int
			for i, dimID := range ids {
				fmt.Printf("[Analyze] (%d/%d) 分析 %s=%s\n", i+1, len(ids), dimension, dimID)
				result, err := bc.CalculateEfficiency(dimension, dimID, startDate, endDate, force)
				if err != nil {
					fmt.Printf("  失败: %v\n", err)
					failCount++
					continue
				}
				successCount++
				printAnalysisResult(result)
			}
			fmt.Printf("[Analyze] 批量分析完成: 成功 %d, 失败 %d\n", successCount, failCount)
		} else {
			fmt.Printf("[Analyze] 分析 %s=%s (%s ~ %s)\n", dimension, id, startDate, endDate)
			result, err := bc.CalculateEfficiency(dimension, id, startDate, endDate, force)
			if err != nil {
				return fmt.Errorf("分析失败: %w", err)
			}
			printAnalysisResult(result)
			fmt.Println("[Analyze] 分析完成")
		}
		return nil
	},
}

var analyzeGitCmd = &cobra.Command{
	Use:   "git",
	Short: "分析 Git 仓库提交数据",
	RunE: func(cmd *cobra.Command, args []string) error {
		repoID, _ := cmd.Flags().GetString("repo-id")
		startDate, _ := cmd.Flags().GetString("start-date")
		endDate, _ := cmd.Flags().GetString("end-date")
		projectID, _ := cmd.Flags().GetString("project-id")

		if repoID == "" || startDate == "" || endDate == "" {
			return fmt.Errorf("必须提供 --repo-id, --start-date, --end-date 参数")
		}

		// 创建 GitAnalyzer
		cacheDir := cfg.RawDataDir + "/git_cache"
		analyzer := NewGitAnalyzer(repoID, cacheDir, cfg.HTTPProxy)

		// 确保仓库存在
		fmt.Printf("[Analyze] 确保仓库存在: %s\n", repoID)
		if err := analyzer.EnsureRepo(); err != nil {
			return fmt.Errorf("确保仓库存在失败: %w", err)
		}

		// 分析 commits
		fmt.Printf("[Analyze] 分析 commits: %s ~ %s\n", startDate, endDate)
		gitResult, err := analyzer.AnalyzeCommits(startDate, endDate)
		if err != nil {
			return fmt.Errorf("分析 commits 失败: %w", err)
		}

		// 打印统计结果
		fmt.Printf("[Analyze] Git 分析结果:\n")
		fmt.Printf("  Commit 数量: %d\n", gitResult.CommitCount)
		fmt.Printf("  贡献者数量: %d\n", gitResult.ContributorCount)
		fmt.Printf("  新增行数: %d\n", gitResult.LinesAdded)
		fmt.Printf("  删除行数: %d\n", gitResult.LinesDeleted)
		fmt.Printf("  变更文件数: %d\n", gitResult.FilesChanged)

		// === 关联分析步骤 ===
		var matches []TaskCommitMatch
		var classifications []CommitClassification
		var attributions []CodeAttribution
		var attributionSummary *AttributionSummary

		orgProvider, err := NewOrgProvider(cfg.OrgCSVFile)
		if err != nil {
			fmt.Printf("[Analyze] 警告: 加载组织信息失败: %v，跳过关联分析\n", err)
		} else {
			fmt.Printf("[Analyze] 组织信息加载成功，共 %d 条记录\n", orgProvider.Count())

			tasks := loadTasksFromES(cfg, repoID, projectID, startDate, endDate)
			fmt.Printf("[Analyze] 从 ES 加载到 %d 个 task\n", len(tasks))

			if len(gitResult.Commits) > 0 {
				matches, classifications = MatchTasksToCommits(gitResult.Commits, tasks, orgProvider)
				fmt.Printf("[Analyze] 关联结果: %d 个匹配, %d 个分类\n", len(matches), len(classifications))

				// 按 commit hash 索引 matched tasks
				commitTasksMap := make(map[string][]TaskContentFile)
				for _, m := range matches {
					for _, t := range tasks {
						if t.TaskID == m.TaskID {
							commitTasksMap[m.CommitHash] = append(commitTasksMap[m.CommitHash], t)
						}
					}
				}

				for _, cls := range classifications {
					if cls.CodeSource == CodeSourceAICurrent {
						matchedTasks := commitTasksMap[cls.CommitHash]
						attr, err := AnalyzeCodeAttribution(cls.CommitHash, matchedTasks, analyzer.LocalPath)
						if err != nil {
							fmt.Printf("[Analyze] 警告: commit %s 归因分析失败: %v\n", cls.CommitHash[:8], err)
							continue
						}
						attributions = append(attributions, *attr)
					}
				}

				if len(attributions) > 0 {
					attributionSummary = SummarizeAttributions(attributions)
					fmt.Printf("[Analyze] 代码归因: AI代码行 %d, 人工代码行 %d\n",
						attributionSummary.TotalOurAILines, attributionSummary.TotalHumanLines)
				}

				// 汇总代码来源统计
				var ourAILines, humanLines, aiOtherLines, unknownLines int64
				var mappedTaskCount int
				for _, cls := range classifications {
					switch cls.CodeSource {
					case CodeSourceAICurrent:
						ourAILines += cls.LinesAdded
						mappedTaskCount++
					case CodeSourceHuman:
						humanLines += cls.LinesAdded
					case CodeSourceAIOther:
						aiOtherLines += cls.LinesAdded
					case CodeSourceUnknown:
						unknownLines += cls.LinesAdded
					}
				}

				fmt.Printf("[Analyze] 代码来源统计:\n")
				fmt.Printf("  AI(我们): %d 行 (%d commits)\n", ourAILines, mappedTaskCount)
				fmt.Printf("  人工: %d 行\n", humanLines)
				fmt.Printf("  AI(其他): %d 行\n", aiOtherLines)
				fmt.Printf("  未知: %d 行\n", unknownLines)
			}
		}

		// 构建输出结果
		output := map[string]interface{}{
			"repo_id":    repoID,
			"start_date": startDate,
			"end_date":   endDate,
			"git_result": gitResult,
		}

		if len(matches) > 0 {
			output["task_commit_matches"] = matches
		}
		if len(classifications) > 0 {
			output["commit_classifications"] = classifications
		}
		if attributionSummary != nil {
			output["code_attribution"] = attributionSummary
		}

		// AI 估时
		if cfg.AIEstimation.Enabled {
			fmt.Printf("[Analyze] 正在进行 AI 估时...\n")
			taskSummary := map[string]interface{}{
				"start_time": startDate,
				"end_time":   endDate,
			}
			minutes, reason, err := EstimateFromGit(cfg.AIEstimation, gitResult, taskSummary)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[Analyze] AI 估时失败: %v\n", err)
			} else {
				fmt.Printf("[Analyze] AI 估时结果: %.2f 分钟\n", minutes)
				fmt.Printf("[Analyze] 估时理由: %s\n", reason)
				output["commit_ancient_minutes"] = minutes
				output["commit_ancient_minutes_reason"] = reason
			}
		}

		// 保存分析结果到 JSON 文件
		now := time.Now()
		dirPath := fmt.Sprintf("%s/%d-%02d/analysis", cfg.RawDataDir, now.Year(), now.Month())
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return fmt.Errorf("创建输出目录失败: %w", err)
		}

		safeRepoID := makeSafeID(repoID)
		filePath := fmt.Sprintf("%s/git_analysis_%s_%s.json", dirPath, safeRepoID, endDate)

		jsonData, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return fmt.Errorf("序列化 JSON 失败: %w", err)
		}

		if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
			return fmt.Errorf("写入文件失败: %w", err)
		}

		fmt.Printf("[Analyze] 分析结果已保存到: %s\n", filePath)

		// 尝试保存分析结果到 backend
		if cfg.BackendURL != "" {
			bc := NewBackendClient(cfg.BackendURL)
			var aiEstDays *float64
			if minutes, ok := output["commit_ancient_minutes"].(float64); ok {
				aiEstDays = &minutes
			}
			if err := bc.SaveGitAnalysis(repoID, startDate, endDate, gitResult, aiEstDays); err != nil {
				fmt.Printf("[Analyze] 警告: 保存到 backend 失败: %v\n", err)
			} else {
				fmt.Printf("[Analyze] 分析结果已保存到 backend\n")
			}
		}

		// 尝试将 commits 写入 PG
		if cfg.BackendURL != "" && len(gitResult.Commits) > 0 {
			bc := NewBackendClient(cfg.BackendURL)
			pgCommits := MapCommitDetailsToPG(gitResult.Commits, toPathSafeID(repoID), orgProvider)
			if err := bc.SaveCommitsToPG(pgCommits); err != nil {
				fmt.Printf("[Analyze] 警告: commit 写入 PG 失败: %v\n", err)
			} else {
				fmt.Printf("[Analyze] %d 条 commit 已写入 PG\n", len(pgCommits))
			}
		}

		return nil
	},
}

func init() {
	analyzeCmd.Flags().String("dimension", "", "分析维度: project 或 repo")
	analyzeCmd.Flags().String("id", "", "维度 ID（与 --all 互斥）")
	analyzeCmd.Flags().Bool("all", false, "批量分析所有")
	analyzeCmd.Flags().String("start-date", "", "开始日期 YYYYMMDD")
	analyzeCmd.Flags().String("end-date", "", "结束日期 YYYYMMDD")
	analyzeCmd.Flags().Bool("force", false, "强制重新分析")

	analyzeGitCmd.Flags().String("repo-id", "", "仓库URL（必填）")
	analyzeGitCmd.Flags().String("start-date", "", "开始日期，格式 20260301（必填）")
	analyzeGitCmd.Flags().String("end-date", "", "结束日期，格式 20260331（必填）")
	analyzeGitCmd.Flags().String("project-id", "", "项目ID（可选，用于过滤 task）")

	analyzeCmd.AddCommand(analyzeGitCmd)
	rootCmd.AddCommand(analyzeCmd)
}

// printAnalysisResult 打印分析结果摘要
func printAnalysisResult(result map[string]interface{}) {
	for key, val := range result {
		fmt.Printf("  %s: %v\n", key, val)
	}
}

// loadTasksFromES 从 ES 查询 task 列表并读取 task 内容文件
func loadTasksFromES(config *Config, repoID, projectID, startDate, endDate string) []TaskContentFile {
	esClient, err := NewESClient(config)
	if err != nil {
		fmt.Printf("[Analyze] 警告: 创建ES客户端失败: %v\n", err)
		return nil
	}

	indexNames := generateESIndexNames("costrict_chat_task_", startDate, endDate)

	// 构建 ES 查询
	var filterClauses []map[string]interface{}
	if projectID != "" {
		filterClauses = append(filterClauses, map[string]interface{}{
			"term": map[string]interface{}{"project_id": projectID},
		})
	}
	if repoID != "" {
		filterClauses = append(filterClauses, map[string]interface{}{
			"term": map[string]interface{}{"repo_id": repoID},
		})
	}

	var query map[string]interface{}
	if len(filterClauses) > 0 {
		query = map[string]interface{}{
			"bool": map[string]interface{}{
				"filter": filterClauses,
			},
		}
	} else {
		query = map[string]interface{}{"match_all": map[string]interface{}{}}
	}

	hits, err := esClient.SearchTasks(indexNames, query, 1000)
	if err != nil {
		fmt.Printf("[Analyze] 警告: ES查询task失败: %v\n", err)
		return nil
	}

	var tasks []TaskContentFile
	for _, hit := range hits {
		sourceFile, _ := hit["source_file"].(string)
		if sourceFile != "" {
			fullPath := filepath.Join(config.RawDataDir, sourceFile)
			data, err := os.ReadFile(fullPath)
			if err != nil {
				fmt.Printf("[Analyze] 警告: 读取task文件失败 %s: %v\n", fullPath, err)
			} else {
				var tcf TaskContentFile
				if err := json.Unmarshal(data, &tcf); err != nil {
					fmt.Printf("[Analyze] 警告: 解析task文件失败 %s: %v\n", fullPath, err)
				} else {
					tasks = append(tasks, tcf)
					continue
				}
			}
		}

		// source_file 不存在或读取失败，从 ES 文档构建简化的 TaskContentFile
		taskID, _ := hit["task_id"].(string)
		if taskID == "" {
			continue
		}
		userID, _ := hit["user_id"].(string)
		userName, _ := hit["user_name"].(string)
		projID, _ := hit["project_id"].(string)
		apiReqTime, _ := hit["api_request_time"].(string)
		apiEndTime, _ := hit["api_end_time"].(string)
		tasks = append(tasks, TaskContentFile{
			TaskID:    taskID,
			UserID:    userID,
			UserName:  userName,
			ProjectID: projID,
			StartTime: apiReqTime,
			EndTime:   apiEndTime,
		})
	}

	return tasks
}
