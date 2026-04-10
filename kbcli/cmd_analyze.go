package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

var safeIDRegex = regexp.MustCompile("[^a-zA-Z0-9]")

func makeSafeID(id string) string {
	return safeIDRegex.ReplaceAllString(id, "_")
}

// runAnalyze 执行 analyze 命令
// 使用方式：kbcli analyze git --repo-id=xxx --start-date=20260301 --end-date=20260331
//
//	kbcli analyze --dimension=project --id=xxx --start-date=20260301 --end-date=20260331
func runAnalyze(config *Config, args []string) {
	if len(args) == 0 {
		fmt.Println("用法: kbcli analyze <子命令> [参数...]")
		fmt.Println("子命令: git")
		fmt.Println()
		fmt.Println("或按维度分析:")
		fmt.Println("  kbcli analyze --dimension=project|repo --id=xxx --start-date=YYYYMMDD --end-date=YYYYMMDD [--force]")
		fmt.Println("  kbcli analyze --dimension=project|repo --all --start-date=YYYYMMDD --end-date=YYYYMMDD [--force]")
		os.Exit(1)
	}

	subCmd := args[0]
	if subCmd == "git" {
		runAnalyzeGit(config, args[1:])
		return
	}

	// 非 git 子命令，走维度分析
	runAnalyzeDimension(config, args)
}

// runAnalyzeDimension 按维度触发提效分析
func runAnalyzeDimension(config *Config, args []string) {
	dimension := parseFlag(args, "dimension", "")
	id := parseFlag(args, "id", "")
	startDate := parseFlag(args, "start-date", "")
	endDate := parseFlag(args, "end-date", "")
	all := parseBoolFlag(args, "all")
	force := parseBoolFlag(args, "force")

	if dimension == "" || startDate == "" || endDate == "" {
		fmt.Println("用法: kbcli analyze --dimension=project|repo --id=xxx --start-date=YYYYMMDD --end-date=YYYYMMDD [--force]")
		fmt.Println("  kbcli analyze --dimension=project|repo --all --start-date=YYYYMMDD --end-date=YYYYMMDD [--force]")
		fmt.Println()
		fmt.Println("参数:")
		fmt.Println("  --dimension    分析维度: project 或 repo（必填）")
		fmt.Println("  --id           维度 ID（与 --all 互斥）")
		fmt.Println("  --start-date   开始日期 YYYYMMDD（必填）")
		fmt.Println("  --end-date     结束日期 YYYYMMDD（必填）")
		fmt.Println("  --all          批量分析所有")
		fmt.Println("  --force        强制重新分析")
		os.Exit(1)
	}

	if dimension != "project" && dimension != "repo" {
		fmt.Fprintf(os.Stderr, "错误: --dimension 必须是 project 或 repo，当前值: %s\n", dimension)
		os.Exit(1)
	}

	if id == "" && !all {
		fmt.Fprintf(os.Stderr, "错误: 必须提供 --id 或 --all 参数\n")
		os.Exit(1)
	}

	bc := NewBackendClient(config.BackendURL)

	if all {
		// 从 ES 获取所有唯一的 dimension ID
		esClient, err := NewESClient(config)
		if err != nil {
			fmt.Fprintf(os.Stderr, "创建 ES 客户端失败: %v\n", err)
			os.Exit(1)
		}

		indexNames := generateESIndexNames("costrict_chat_task_", startDate, endDate)
		field := dimension + "_id"
		ids, err := esClient.GetUniqueDimensionIDs(indexNames, field)
		if err != nil {
			fmt.Fprintf(os.Stderr, "获取 %s 列表失败: %v\n", dimension, err)
			os.Exit(1)
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
			fmt.Fprintf(os.Stderr, "分析失败: %v\n", err)
			os.Exit(1)
		}
		printAnalysisResult(result)
		fmt.Println("[Analyze] 分析完成")
	}
}

// printAnalysisResult 打印分析结果摘要
func printAnalysisResult(result map[string]interface{}) {
	for key, val := range result {
		fmt.Printf("  %s: %v\n", key, val)
	}
}

func runAnalyzeGit(config *Config, args []string) {
	repoID := parseFlag(args, "repo-id", "")
	startDate := parseFlag(args, "start-date", "")
	endDate := parseFlag(args, "end-date", "")
	projectID := parseFlag(args, "project-id", "")

	if repoID == "" || startDate == "" || endDate == "" {
		fmt.Println("用法: kbcli analyze git --repo-id=<仓库URL> --start-date=<开始日期> --end-date=<结束日期> [--project-id=<项目ID>]")
		fmt.Println("  --repo-id       仓库URL（必填）")
		fmt.Println("  --start-date    开始日期，格式 20260301（必填）")
		fmt.Println("  --end-date      结束日期，格式 20260331（必填）")
		fmt.Println("  --project-id    项目ID（可选，用于过滤 task）")
		os.Exit(1)
	}

	// 创建 GitAnalyzer
	cacheDir := config.RawDataDir + "/git_cache"
	analyzer := NewGitAnalyzer(repoID, cacheDir, config.HTTPProxy)

	// 确保仓库存在
	fmt.Printf("[Analyze] 确保仓库存在: %s\n", repoID)
	if err := analyzer.EnsureRepo(); err != nil {
		fmt.Fprintf(os.Stderr, "确保仓库存在失败: %v\n", err)
		os.Exit(1)
	}

	// 分析 commits
	fmt.Printf("[Analyze] 分析 commits: %s ~ %s\n", startDate, endDate)
	gitResult, err := analyzer.AnalyzeCommits(startDate, endDate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "分析 commits 失败: %v\n", err)
		os.Exit(1)
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

	orgProvider, err := NewOrgProvider(config.OrgCSVFile)
	if err != nil {
		fmt.Printf("[Analyze] 警告: 加载组织信息失败: %v，跳过关联分析\n", err)
	} else {
		fmt.Printf("[Analyze] 组织信息加载成功，共 %d 条记录\n", orgProvider.Count())

		tasks := loadTasksFromES(config, repoID, projectID, startDate, endDate)
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
	if config.AIEstimation.Enabled {
		fmt.Printf("[Analyze] 正在进行 AI 估时...\n")
		taskSummary := map[string]interface{}{
			"start_time": startDate,
			"end_time":   endDate,
		}
		minutes, reason, err := EstimateFromGit(config.AIEstimation, gitResult, taskSummary)
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
	dirPath := fmt.Sprintf("%s/%d-%02d/analysis", config.RawDataDir, now.Year(), now.Month())
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "创建输出目录失败: %v\n", err)
		os.Exit(1)
	}

	safeRepoID := makeSafeID(repoID)
	filePath := fmt.Sprintf("%s/git_analysis_%s_%s.json", dirPath, safeRepoID, endDate)

	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "序列化 JSON 失败: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "写入文件失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("[Analyze] 分析结果已保存到: %s\n", filePath)

	// 尝试保存分析结果到 backend
	if config.BackendURL != "" {
		bc := NewBackendClient(config.BackendURL)
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
	if config.BackendURL != "" && len(gitResult.Commits) > 0 {
		bc := NewBackendClient(config.BackendURL)
		pgCommits := MapCommitDetailsToPG(gitResult.Commits, toPathSafeID(repoID), orgProvider)
		if err := bc.SaveCommitsToPG(pgCommits); err != nil {
			fmt.Printf("[Analyze] 警告: commit 写入 PG 失败: %v\n", err)
		} else {
			fmt.Printf("[Analyze] %d 条 commit 已写入 PG\n", len(pgCommits))
		}
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
