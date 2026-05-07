package main

import (
	"github.com/spf13/cobra"
)

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "顺序执行完整的导入流程: import-task → import-repo → import-org → silica → efficiency",
	Long: `顺序执行完整的导入流程:
  1. import-task: 导入task数据
  2. import-repo: 导入repo/commit数据
  3. import-org: 导入用户组织信息
  4. silica: 计算commit含硅量
  5. efficiency: 计算用户和组织效能数据

所有子命令的参数均可在import命令中使用，参数含义与各子命令一致。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		taskDir, _ := cmd.Flags().GetString("task-dir")
		repoDir, _ := cmd.Flags().GetString("repo-dir")
		analysedDir, _ := cmd.Flags().GetString("analysed-dir")
		force, _ := cmd.Flags().GetBool("force")
		fromDB, _ := cmd.Flags().GetString("from-db")
		fromCSV, _ := cmd.Flags().GetString("from-csv")
		dateStr, _ := cmd.Flags().GetString("date")
		maxDays, _ := cmd.Flags().GetInt("max-days")
		remote, _ := cmd.Flags().GetString("remote")

		// 如果指定了远程地址，发送到远程 kbcli 服务执行
		if remote != "" {
			return sendToRemote(remote, "import", map[string]interface{}{
				"task_dir":     taskDir,
				"repo_dir":     repoDir,
				"analysed_dir": analysedDir,
				"force":        force,
				"from_db":      fromDB,
				"from_csv":     fromCSV,
				"date":         dateStr,
				"max_days":     maxDays,
			})
		}
		if taskDir == "" {
			taskDir = cfg.TaskDir
		}
		if repoDir == "" {
			repoDir = cfg.RepoDir
		}
		if analysedDir == "" {
			analysedDir = cfg.AnalysedDir
		}
		if fromDB == "" {
			fromDB = cfg.OrgDSN
		}
		if maxDays <= 0 {
			maxDays = cfg.SilicaMaxDays
		}

		steps := []struct {
			name string
			fn   func() error
		}{
			{"import-task", func() error { return runImportTask(taskDir, analysedDir, force) }},
			{"import-repo", func() error { return runImportRepo(repoDir, analysedDir, force) }},
			{"import-org", func() error { return runImportOrg(fromDB, fromCSV, "") }},
			{"silica", func() error { return runSilica(analysedDir, force, maxDays) }},
			{"efficiency", func() error { return runEfficiency(dateStr) }},
		}

		for _, step := range steps {
			logInfof("========== [import] 步骤: %s ==========", step.name)
			if err := step.fn(); err != nil {
				logErrorf("步骤 %s 失败: %w", step.name, err)
			}
		}

		logInfo("========== [import] 全部步骤完成 ==========")
		return nil
	},
}

func init() {
	importCmd.Flags().SortFlags = false
	importCmd.Flags().String("task-dir", "", "task 目录路径")
	importCmd.Flags().String("repo-dir", "", "repo 目录路径")
	importCmd.Flags().String("analysed-dir", "", "已处理文件的输出目录")
	importCmd.Flags().BoolP("force", "f", false, "强制重新导入和计算，覆盖已存在数据")
	importCmd.Flags().String("from-db", "", "源数据库DSN（import-org用）")
	importCmd.Flags().String("to-csv", "", "导出CSV文件路径（import-org用，可选，不指定则不导出）")
	importCmd.Flags().String("from-csv", "", "从指定的CSV文件加载UserOrg数据，替代从数据库加载（import-org用）")
	importCmd.Flags().String("date", "", "聚合日期，格式YYYYMMDD，不指定则处理所有日期（efficiency用）")
	importCmd.Flags().Int("max-days", 0, "对话结束后多少天内的commit算相关（silica用，默认从config读取）")
	importCmd.Flags().String("remote", "", "远程kbcli服务地址（如 http://127.0.0.1:8080），指定后命令将发送到远程执行")

	rootCmd.AddCommand(importCmd)
}
