package main

import (
	"fmt"
	"os/exec"
	"strings"
)

// CodeAttribution 单个 commit 的代码归因分析结果
type CodeAttribution struct {
	CommitHash      string `json:"commit_hash"`
	TaskID          string `json:"task_id"`
	OurAICodeLines  int64  `json:"our_ai_code_lines"`
	HumanCodeLines  int64  `json:"human_code_lines"`
	TotalAddedLines int64  `json:"total_added_lines"`
}

// AttributionSummary 代码归因汇总
type AttributionSummary struct {
	TotalOurAILines int64            `json:"total_our_ai_lines"`
	TotalHumanLines int64            `json:"total_human_lines"`
	CommitCount     int              `json:"commit_count"`
	Details         []CodeAttribution `json:"details"`
}

// AnalyzeCodeAttribution 分析单个 commit 的代码来源归因
// commitHash: commit 哈希
// matchedTasks: 与该 commit 关联的 task 列表
// gitLocalPath: git 仓库本地路径
func AnalyzeCodeAttribution(commitHash string, matchedTasks []TaskContentFile, gitLocalPath string) (*CodeAttribution, error) {
	// 获取 commit diff
	cmd := exec.Command("git", "-C", gitLocalPath, "show", "--no-color", "--unified=0", commitHash)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("执行 git show 失败 (commit %s): %w", commitHash, err)
	}

	// 从 diff 中提取新增行
	var addedLines []string
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			addedLines = append(addedLines, line[1:]) // 去掉行首的 '+'
		}
	}

	// 构建 AI 代码行集合
	aiCodeLines := make(map[string]bool)
	for _, task := range matchedTasks {
		for _, conv := range task.Conversations {
			for _, co := range conv.CodeOutputs {
				if co.Code == "" {
					continue
				}
				for _, codeLine := range strings.Split(co.Code, "\n") {
					trimmed := strings.TrimSpace(codeLine)
					if trimmed != "" {
						aiCodeLines[trimmed] = true
					}
				}
			}
		}
	}

	// 行级比对
	var ourAICount, humanCount int64
	for _, line := range addedLines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if aiCodeLines[trimmed] {
			ourAICount++
		} else {
			humanCount++
		}
	}

	result := &CodeAttribution{
		CommitHash:      commitHash,
		OurAICodeLines:  ourAICount,
		HumanCodeLines:  humanCount,
		TotalAddedLines: ourAICount + humanCount,
	}

	if len(matchedTasks) > 0 {
		result.TaskID = matchedTasks[0].TaskID
	}

	return result, nil
}

// SummarizeAttributions 汇总多个 CodeAttribution 的统计
func SummarizeAttributions(attributions []CodeAttribution) *AttributionSummary {
	summary := &AttributionSummary{
		CommitCount: len(attributions),
		Details:     attributions,
	}
	for _, a := range attributions {
		summary.TotalOurAILines += a.OurAICodeLines
		summary.TotalHumanLines += a.HumanCodeLines
	}
	return summary
}
