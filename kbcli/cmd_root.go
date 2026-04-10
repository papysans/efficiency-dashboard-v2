package main

import (
	"fmt"
	"os"
	"strings"
)

// RunCLI 是 kbcli 命令行的入口，根据第一个参数分发到对应子命令
func RunCLI(config *Config) {
	args := os.Args[1:]
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	subCmd := args[0]
	subArgs := args[1:]

	switch subCmd {
	case "reindex":
		runReindex(config, subArgs)
	case "reload-org":
		orgProvider, err := NewOrgProvider(config.OrgCSVFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "加载组织信息失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("组织信息加载成功，共 %d 条记录\n", orgProvider.Count())
	case "validate-org":
		csvFile := parseFlag(subArgs, "csv-file", config.OrgCSVFile)
		orgProvider, err := NewOrgProvider(csvFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "验证失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("CSV 文件验证通过: %s\n", csvFile)
		fmt.Printf("  user_id 条目数: %d\n", len(orgProvider.userIDMap))
		fmt.Printf("  user_name 条目数: %d\n", len(orgProvider.userNameMap))
	case "analyze":
		runAnalyze(config, subArgs)
	case "correct":
		runCorrect(config, subArgs)
	case "cat-analysis":
		runCatAnalysis(config, subArgs)
	case "import-tasks":
		runImportTasks(config, subArgs)
	default:
		fmt.Fprintf(os.Stderr, "未知子命令: %s\n\n", subCmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`用法: kbcli <子命令> [参数...]

子命令:
  reindex        将指定日期的 rawdata 写入 Elasticsearch
  reload-org     重新加载组织信息 CSV 文件
  validate-org   验证组织信息 CSV 文件格式
  analyze        分析命令（支持 git 子命令和维度分析）
  correct        纠错命令，修正分析结果中的字段值
  cat-analysis   查看分析过程文件内容
  import-tasks   导入 task 数据到 costrict_stat 数据库

示例:
  kbcli reindex --date=20260331               # 将2026-03-31的数据写入ES
  kbcli reindex --date=20260101               # 将2026-01-01的数据写入ES
  kbcli reload-org                            # 重新加载组织信息
  kbcli validate-org --csv-file=./org.csv     # 验证 CSV 文件
  kbcli analyze git --repo-id=https://github.com/xxx/yyy.git --start-date=20260301 --end-date=20260331
  kbcli analyze --dimension=project --id=xxx --start-date=20260301 --end-date=20260331
  kbcli analyze --dimension=repo --all --start-date=20260301 --end-date=20260331 --force
  kbcli correct --dimension=project --id=xxx --field=task_ancient_minutes --value=50.5 --reason="修正估时" --by="admin"
  kbcli cat-analysis --dimension=project --id=xxx --date=20260331
  kbcli import-tasks --task-dir=./task        # 导入 task 数据到数据库`)
}

// parseFlag 解析 --key=value 或 --key value 格式的参数
func parseFlag(args []string, key string, defaultVal string) string {
	prefix := "--" + key + "="
	for i, a := range args {
		if strings.HasPrefix(a, prefix) {
			return strings.TrimPrefix(a, prefix)
		}
		if a == "--"+key && i+1 < len(args) {
			return args[i+1]
		}
	}
	return defaultVal
}

// parseBoolFlag 解析 --flag 布尔参数
func parseBoolFlag(args []string, key string) bool {
	for _, a := range args {
		if a == "--"+key {
			return true
		}
	}
	return false
}
