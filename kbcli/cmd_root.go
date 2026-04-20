package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// cfg 是全局配置，在 rootCmd 的 PersistentPreRunE 中加载
var cfg *Config

// rootCmd 是 kbcli 的根 cobra 命令
var rootCmd = &cobra.Command{
	Use:   "kbcli",
	Short: "效率看板 CLI 工具",
	Long:  "kbcli 是效率看板的命令行工具，用于数据索引、分析、纠错等操作。",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("config")
		loadedCfg, err := LoadConfig(configPath)
		if err != nil {
			// fallback: 尝试上级目录的 config.yaml
			loadedCfg, err = LoadConfig("../config.yaml")
			if err != nil {
				return fmt.Errorf("加载配置文件失败: %w", err)
			}
		}
		cfg = loadedCfg
		return nil
	},
}

// reloadOrgCmd 重新加载组织信息 CSV 文件
var reloadOrgCmd = &cobra.Command{
	Use:   "reload-org",
	Short: "重新加载组织信息 CSV 文件",
	RunE: func(cmd *cobra.Command, args []string) error {
		orgProvider, err := NewOrgProvider(cfg.OrgCSVFile)
		if err != nil {
			return fmt.Errorf("加载组织信息失败: %w", err)
		}
		fmt.Printf("组织信息加载成功，共 %d 条记录\n", orgProvider.Count())
		return nil
	},
}

// validateOrgCmd 验证组织信息 CSV 文件格式
var validateOrgCmd = &cobra.Command{
	Use:   "validate-org",
	Short: "验证组织信息 CSV 文件格式",
	RunE: func(cmd *cobra.Command, args []string) error {
		csvFile, _ := cmd.Flags().GetString("csv-file")
		if csvFile == "" {
			csvFile = cfg.OrgCSVFile
		}
		orgProvider, err := NewOrgProvider(csvFile)
		if err != nil {
			return fmt.Errorf("验证失败: %w", err)
		}
		fmt.Printf("CSV 文件验证通过: %s\n", csvFile)
		fmt.Printf("  user_id 条目数: %d\n", len(orgProvider.userIDMap))
		fmt.Printf("  user_name 条目数: %d\n", len(orgProvider.userNameMap))
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().String("config", "config.yaml", "配置文件路径")
	validateOrgCmd.Flags().String("csv-file", "", "CSV 文件路径（默认使用配置文件中的路径）")
	// rootCmd.AddCommand(analyzeCmd)
	rootCmd.AddCommand(reloadOrgCmd)
	rootCmd.AddCommand(validateOrgCmd)
}

// Execute 执行根命令，供 main.go 调用
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		// cobra 已经输出了错误信息，这里只需设置退出码
		// 但由于我们在 main 中调用，可以通过 os.Exit 处理
		// 为简洁起见，让 cobra 自行处理
	}
}
