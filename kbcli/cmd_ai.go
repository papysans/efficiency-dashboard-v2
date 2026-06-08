package main

import (
	"encoding/json"
	"fmt"
	"kanban/kbcli/internal/appconfig"
	"kanban/kbcli/internal/llm"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var aiCmd = &cobra.Command{
	Use:   "ai",
	Short: "使用AI对指定文件进行estimation或summarize",
	Long:  `加载指定的conversation jsonl或commit json文件，调用AI进行estimation或summarize，直接输出结果。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		path, _ := cmd.Flags().GetString("path")
		fileType, _ := cmd.Flags().GetString("type")
		op, _ := cmd.Flags().GetString("op")

		if path == "" {
			return fmt.Errorf("--path 必须指定")
		}
		if fileType != "task" && fileType != "commit" {
			return fmt.Errorf("--type 必须是 task 或 commit")
		}
		if op != "estimation" && op != "summarize" {
			return fmt.Errorf("--op 必须是 estimation 或 summarize")
		}

		return runAI(path, fileType, op)
	},
}

func runAI(path, fileType, op string) error {
	switch fileType {
	case "task":
		return runAITask(path, op)
	case "commit":
		return runAICommit(path, op)
	}
	return fmt.Errorf("未知的type: %s", fileType)
}

func runAITask(path, op string) error {
	convs, err := parseConversationFile(path)
	if err != nil {
		return fmt.Errorf("解析conversation文件失败: %w", err)
	}

	var userInputs []string
	var codeOutputs []string
	var totalChars, totalLines int64
	for _, c := range convs {
		if c.UserInput != "" {
			userInputs = append(userInputs, c.UserInput)
			totalChars += int64(len(c.UserInput))
		}
		if c.Diff != "" {
			codeOutputs = append(codeOutputs, c.Diff)
		}
		totalLines += c.DiffLines
	}

	if op == "summarize" {
		if len(userInputs) == 0 {
			return fmt.Errorf("无用户输入，无法summarize")
		}
		title, err := callAIForTaskTitle(nil, "", userInputs)
		if err != nil {
			return fmt.Errorf("AI summarize失败: %w", err)
		}
		fmt.Printf("Title: %s\n", title)
		return nil
	}

	if op == "estimation" {
		if len(userInputs) == 0 {
			return fmt.Errorf("无用户输入，无法estimation")
		}
		title, err := callAIForTaskTitle(nil, "", userInputs)
		if err != nil {
			return fmt.Errorf("AI 取标题失败: %w", err)
		}
		minutes, reason, err := callAIForAncientEstimation(title, int(totalLines), totalChars)
		if err != nil {
			return fmt.Errorf("AI estimation失败: %w", err)
		}
		fmt.Printf("Title: %s\nAncient Minutes: %.1f\nReason: %s\n", title, minutes, reason)
		return nil
	}

	return fmt.Errorf("未知的op: %s", op)
}

func runAICommit(path, op string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取commit文件失败: %w", err)
	}

	var commitData RepoCommitData
	if err := json.Unmarshal(data, &commitData); err != nil {
		return fmt.Errorf("解析commit JSON失败: %w", err)
	}

	if op == "summarize" {
		summary, err := callAIForCommitSummarize(commitData.Comment, commitData.Diff, commitData.DiffLines)
		if err != nil {
			return fmt.Errorf("AI summarize失败: %w", err)
		}
		fmt.Printf("Summary: %s\n", summary)
		return nil
	}

	if op == "estimation" {
		minutes, reason, err := callAIForCommitEstimation(commitData.Comment, commitData.Diff, commitData.DiffLines)
		if err != nil {
			return fmt.Errorf("AI estimation失败: %w", err)
		}
		fmt.Printf("Ancient Minutes: %.1f\nReason: %s\n", minutes, reason)
		return nil
	}

	return fmt.Errorf("未知的op: %s", op)
}

// 调用AI生成commit摘要
func callAIForCommitSummarize(comment, diff string, diffLines int) (string, error) {
	aiCfg := appconfig.Cfg.AIEstimation
	if !aiCfg.Enabled || aiCfg.APIKey == "" {
		return "", fmt.Errorf("AI estimation not enabled or API key missing")
	}

	prompt := fmt.Sprintf(`你是一个经验丰富的软件工程师。请根据以下Git commit信息，生成一段简短的中文摘要（不超过200字），概括这次提交的主要修改内容和目的。

提交说明：
%s

代码变更（diff）：
%s

总变更代码行数：%d

只输出摘要文本，不要任何额外格式。`,
		truncateString(comment, 2000),
		truncateString(diff, 8000),
		diffLines,
	)

	messages := []llm.ChatMessage{
		{Role: "system", Content: "请回答问题"},
		{Role: "user", Content: prompt},
	}
	content, err := llm.CallLLM(aiCfg, messages, 512)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(content), nil
}

func init() {
	aiCmd.Flags().SortFlags = false
	aiCmd.Flags().String("path", "", "需要加载的文件路径（conversation jsonl 或 commit json）")
	aiCmd.Flags().String("type", "", "文件格式：task 或 commit")
	aiCmd.Flags().String("op", "", "AI操作：estimation 或 summarize")
	_ = aiCmd.MarkFlagRequired("path")
	_ = aiCmd.MarkFlagRequired("type")
	_ = aiCmd.MarkFlagRequired("op")
	rootCmd.AddCommand(aiCmd)
}
