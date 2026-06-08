package main

import (
	"github.com/spf13/cobra"
	"kanban/kbcli/internal/logx"
)

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "顺序执行完整的导入流程: import-conv → import-repo → import-org → import-dept → efficiency",
	Long: `顺序执行完整的导入流程:
  1. import-conv: 导入task数据
  2. import-repo: 导入repo/commit数据（含silica计算）
  3. import-org: 导入用户组织信息（兜底"临时组织"占位为非破坏性，不覆盖已有真实 org）
  4. import-dept: 从 dept-sync 刷新真实部门并投影回填 user_org（未配置/不可达时非致命跳过）
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
		startDateStr, _ := cmd.Flags().GetString("start-date")
		endDateStr, _ := cmd.Flags().GetString("end-date")
		maxDays, _ := cmd.Flags().GetInt("max-days")
		createPseudo, _ := cmd.Flags().GetBool("create-pseudo")
		if !cmd.Flags().Changed("create-pseudo") {
			createPseudo = cfg.TaskCreate.CreatePseudoTask
		}
		remote, _ := cmd.Flags().GetString("remote")

		// 如果指定了远程地址，发送到远程 kbcli 服务执行
		if remote != "" {
			return sendToRemote(remote, "import", map[string]interface{}{
				"task_dir":      taskDir,
				"repo_dir":      repoDir,
				"analysed_dir":  analysedDir,
				"force":         force,
				"from_db":       fromDB,
				"from_csv":      fromCSV,
				"date":          dateStr,
				"start_date":    startDateStr,
				"end_date":      endDateStr,
				"max_days":      maxDays,
				"create_pseudo": createPseudo,
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
			maxDays = cfg.TaskCreate.SilicaMaxDays
		}
		// 未显式传 start-date 且非单日(date)模式时，套全局分析起始日下界。
		// 一处生效全流程：import-conv/import-repo/efficiency 都用这个 startDateStr。
		if dateStr == "" {
			startDateStr = applyAnalysisFloor(startDateStr)
		}

		steps := []struct {
			name string
			fn   func() error
		}{
			{"import-conv", func() error {
				return runImportConv(taskDir, analysedDir, force, startDateStr, endDateStr, dateStr, createPseudo)
			}},
			{"import-repo", func() error {
				return runImportRepo(repoDir, analysedDir, force, maxDays, startDateStr, endDateStr, dateStr)
			}},
			{"import-org", func() error { return runImportOrg(fromDB, fromCSV, "") }},
			{"import-dept", func() error {
				// 非致命：未配置 dept_sync.base_url 或 dept-sync 不可达时仅告警跳过，
				// 不阻断后续 efficiency 步骤。import-dept 用真实部门刷新 org，配合
				// import-org 的非破坏性占位，使 org 自动保持正确。
				if err := runImportDept("", ""); err != nil {
					logx.Warnf("import-dept 跳过(非致命): %v", err)
				}
				return nil
			}},
			{"efficiency-v2", func() error { return runEfficiencyV2(startDateStr, endDateStr, dateStr) }},
		}

		for _, step := range steps {
			logx.Infof("========== [import] 步骤: %s ==========", step.name)
			if err := step.fn(); err != nil {
				logx.Errorf("步骤 %s 失败: %v", step.name, err)
			}
		}

		logx.Info("========== [import] 全部步骤完成 ==========")
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
	importCmd.Flags().String("date", "", "限定日期，格式YYYYMMDD，限定活跃时间在该日期之内（与start-date/end-date互斥）")
	importCmd.Flags().String("start-date", "", "限定起始日期，格式YYYYMMDD，限定活跃时间在该日期之后（含）")
	importCmd.Flags().String("end-date", "", "限定结束日期，格式YYYYMMDD，限定活跃时间在该日期之前（含）")
	importCmd.Flags().Int("max-days", 0, "对话结束后多少天内的commit算相关（silica用，默认从config读取）")
	importCmd.Flags().Bool("create-pseudo", false, "为所有session创建伪任务（默认从config读取）")
	importCmd.Flags().String("remote", "", "远程kbcli服务地址（如 http://127.0.0.1:8080），指定后命令将发送到远程执行")

	rootCmd.AddCommand(importCmd)
}
