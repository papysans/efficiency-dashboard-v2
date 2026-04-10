package main

import (
	"fmt"
	"os"
	"strconv"
)

// runCorrect 执行纠错命令
func runCorrect(config *Config, args []string) {
	dimension := parseFlag(args, "dimension", "")
	id := parseFlag(args, "id", "")
	field := parseFlag(args, "field", "")
	valueStr := parseFlag(args, "value", "")
	reason := parseFlag(args, "reason", "")
	by := parseFlag(args, "by", "")

	if dimension == "" || id == "" || field == "" || valueStr == "" || reason == "" || by == "" {
		fmt.Println("用法: kbcli correct --dimension=project|repo --id=xxx --field=字段名 --value=数值 --reason=\"原因\" --by=\"操作人\"")
		fmt.Println()
		fmt.Println("参数:")
		fmt.Println("  --dimension    维度: project 或 repo（必填）")
		fmt.Println("  --id           维度 ID（必填）")
		fmt.Println("  --field        要纠错的字段名（必填）")
		fmt.Println("  --value        纠错后的值（必填，数值）")
		fmt.Println("  --reason       纠错原因（必填）")
		fmt.Println("  --by           操作人（必填）")
		os.Exit(1)
	}

	if dimension != "project" && dimension != "repo" {
		fmt.Fprintf(os.Stderr, "错误: --dimension 必须是 project 或 repo，当前值: %s\n", dimension)
		os.Exit(1)
	}

	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: --value 必须是数值，当前值: %s\n", valueStr)
		os.Exit(1)
	}

	bc := NewBackendClient(config.BackendURL)
	fmt.Printf("[Correct] 纠错 %s=%s, 字段=%s, 值=%.2f\n", dimension, id, field, value)

	if err := bc.CorrectEfficiency(dimension, id, field, value, reason, by); err != nil {
		fmt.Fprintf(os.Stderr, "纠错失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("[Correct] 纠错成功")
}
