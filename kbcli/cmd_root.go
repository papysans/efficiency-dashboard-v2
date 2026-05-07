package main

import (
	"fmt"
	"os"

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
			fmt.Printf("load config [%s] failed: %v\n", configPath, err)
			// fallback: 尝试上级目录的 config.yaml
			loadedCfg, err = LoadConfig("../config.yaml")
			if err != nil {
				return fmt.Errorf("加载配置文件失败: %w", err)
			}
		}
		cfg = loadedCfg
		fmt.Printf("load config [%s] ok, cfg: %+v\n", configPath, cfg)

		consoleLevel, _ := cmd.Flags().GetString("console")
		logFile, _ := cmd.Flags().GetString("logfile")
		fileLevel, _ := cmd.Flags().GetString("loglevel")
		if err := InitLogger(consoleLevel, logFile, fileLevel); err != nil {
			return err
		}

		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().String("config", "config.yaml", "配置文件路径")
	rootCmd.PersistentFlags().String("console", "warn", "控制台日志级别 (debug/info/warn/error)")
	rootCmd.PersistentFlags().String("logfile", "", "日志文件路径")
	rootCmd.PersistentFlags().String("loglevel", "debug", "日志文件级别 (debug/info/warn/error)")
}

// Execute 执行根命令，供 main.go 调用
func Execute() {
	err := rootCmd.Execute()
	if logger != nil {
		logger.Close()
	}
	if err != nil {
		os.Exit(1)
	}
}
