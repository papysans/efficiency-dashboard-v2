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

func init() {
	rootCmd.PersistentFlags().String("config", "config.yaml", "配置文件路径")
}

// Execute 执行根命令，供 main.go 调用
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		// cobra 已经输出了错误信息，这里只需设置退出码
		// 但由于我们在 main 中调用，可以通过 os.Exit 处理
		// 为简洁起见，让 cobra 自行处理
	}
}
