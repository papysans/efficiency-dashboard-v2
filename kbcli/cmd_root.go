package main

import (
	"kanban/core/storage"
	"kanban/kbcli/internal/appconfig"
	"kanban/kbcli/internal/logx"
	"os"

	"github.com/spf13/cobra"
)

// rootCmd 是 kbcli 的根 cobra 命令
var rootCmd = &cobra.Command{
	Use:   "kbcli",
	Short: "效率看板 CLI 工具",
	Long:  "kbcli 是效率看板的命令行工具，用于数据索引、分析、纠错等操作。",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("config")
		consoleLevel, _ := cmd.Flags().GetString("console")
		logFile, _ := cmd.Flags().GetString("logfile")
		fileLevel, _ := cmd.Flags().GetString("loglevel")

		if err := logx.Init(consoleLevel, logFile, fileLevel); err != nil {
			return err
		}
		loadedCfg, err := appconfig.LoadFirstConfig([]string{configPath, "config.yaml", "configs/kbcli-config.yaml", "kbcli-config.yaml", "../kbcli-config.yaml"})
		if err != nil {
			return err
		}
		appconfig.Cfg = loadedCfg
		// 初始化存储后端，并对配置中的 s3:// 路径做启动期 fail-fast 校验
		if err := storage.Configure(appconfig.Cfg.Storage); err != nil {
			return err
		}
		return storage.ValidateLocations(appconfig.Cfg.TaskDir, appconfig.Cfg.RepoDir, appconfig.Cfg.AnalysedDir, appconfig.Cfg.OrgCSVFile)
	},
}

func init() {
	rootCmd.PersistentFlags().String("config", "", "配置文件路径")
	rootCmd.PersistentFlags().String("console", "info", "控制台日志级别 (debug/info/warn/error)")
	rootCmd.PersistentFlags().String("logfile", "", "日志文件路径")
	rootCmd.PersistentFlags().String("loglevel", "info", "日志文件级别 (debug/info/warn/error)")
}

// Execute 执行根命令，供 main.go 调用
func Execute() {
	err := rootCmd.Execute()
	logx.Close()
	if err != nil {
		os.Exit(1)
	}
}
