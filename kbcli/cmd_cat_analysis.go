package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// runCatAnalysis 查看分析文件
func runCatAnalysis(config *Config, args []string) {
	dimension := parseFlag(args, "dimension", "")
	id := parseFlag(args, "id", "")
	date := parseFlag(args, "date", "")

	if dimension == "" || id == "" || date == "" {
		fmt.Println("用法: kbcli cat-analysis --dimension=project|repo --id=xxx --date=YYYYMMDD")
		fmt.Println()
		fmt.Println("参数:")
		fmt.Println("  --dimension    维度: project 或 repo（必填）")
		fmt.Println("  --id           维度 ID（必填）")
		fmt.Println("  --date         日期 YYYYMMDD（必填）")
		os.Exit(1)
	}

	if dimension != "project" && dimension != "repo" {
		fmt.Fprintf(os.Stderr, "错误: --dimension 必须是 project 或 repo，当前值: %s\n", dimension)
		os.Exit(1)
	}

	if len(date) != 8 {
		fmt.Fprintf(os.Stderr, "错误: --date 格式必须是 YYYYMMDD，当前值: %s\n", date)
		os.Exit(1)
	}

	// 构建文件路径: {RawDataDir}/{YYYY-MM}/analysis/{dimension}_{safeID}_{date}.json
	yearMonth := date[:4] + "-" + date[4:6]
	safeID := makeSafeID(id)
	filePath := fmt.Sprintf("%s/%s/analysis/%s_%s_%s.json", config.RawDataDir, yearMonth, dimension, safeID, date)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "文件不存在: %s\n", filePath)
		} else {
			fmt.Fprintf(os.Stderr, "读取文件失败: %v\n", err)
		}
		os.Exit(1)
	}

	// 尝试格式化输出 JSON
	var parsed interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		// JSON 解析失败，直接输出原始内容
		fmt.Print(string(data))
		return
	}

	formatted, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		fmt.Print(string(data))
		return
	}
	fmt.Println(string(formatted))
}
