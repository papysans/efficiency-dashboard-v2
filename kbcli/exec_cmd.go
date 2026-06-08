package main

import (
	"fmt"
	"kanban/kbcli/internal/logx"
	"time"
)

// resolveStartDateByDays 支持 cron 增量：params 指定 days>0 且未显式给 start_date/date 时，
// 运行时把起始日设为 today-days（YYYYMMDD），只处理最近 N 天而非全量重拉/重算。
func resolveStartDateByDays(params map[string]interface{}, startDate, date string) string {
	days := getIntParam(params, "days", 0)
	if days > 0 && startDate == "" && date == "" {
		return time.Now().AddDate(0, 0, -days).Format("20060102")
	}
	return startDate
}

// createTaskExecutor 根据任务类型创建对应的执行回调函数
func createTaskExecutor(taskType string, params map[string]interface{}) (func() error, error) {
	switch taskType {
	case "import":
		return func() error { return executeImport(params) }, nil
	case "import-conv":
		return func() error { return executeImportConv(params) }, nil
	case "import-repo":
		return func() error { return executeImportRepo(params) }, nil
	case "import-org":
		return func() error { return executeImportOrg(params) }, nil
	case "efficiency-v2":
		return func() error { return executeEfficiencyV2(params) }, nil
	case "fix-task":
		return func() error { return executeFixTask(params) }, nil
	case "fix-commit":
		return func() error { return executeFixCommit(params) }, nil
	default:
		return nil, fmt.Errorf("未知任务类型: %s", taskType)
	}
}

func getStringParam(params map[string]interface{}, key string, defaultVal string) string {
	if val, ok := params[key]; ok {
		if s, ok := val.(string); ok && s != "" {
			return s
		}
	}
	return defaultVal
}

func getBoolParam(params map[string]interface{}, key string, defaultVal bool) bool {
	if val, ok := params[key]; ok {
		switch v := val.(type) {
		case bool:
			return v
		case string:
			return v == "true" || v == "1"
		case int:
			return v != 0
		case float64:
			return v != 0
		}
	}
	return defaultVal
}

func getIntParam(params map[string]interface{}, key string, defaultVal int) int {
	if val, ok := params[key]; ok {
		switch v := val.(type) {
		case int:
			if v == 0 {
				return defaultVal
			}
			return v
		case float64:
			if v == 0.0 {
				return defaultVal
			}
			return int(v)
		case string:
			if v == "" {
				return defaultVal
			}
			var result int
			fmt.Sscanf(v, "%d", &result)
			return result
		}
	}
	return defaultVal
}

func executeImportConv(params map[string]interface{}) error {
	taskDir := getStringParam(params, "task_dir", cfg.TaskDir)
	analysedDir := getStringParam(params, "analysed_dir", cfg.AnalysedDir)
	force := getBoolParam(params, "force", false)
	startDate := getStringParam(params, "start_date", "")
	endDate := getStringParam(params, "end_date", "")
	date := getStringParam(params, "date", "")
	createPseudo := getBoolParam(params, "create_pseudo", cfg.TaskCreate.CreatePseudoTask)
	if date == "" {
		startDate = applyAnalysisFloor(startDate)
	}
	return runImportConv(taskDir, analysedDir, force, startDate, endDate, date, createPseudo)
}

func executeImportRepo(params map[string]interface{}) error {
	repoDir := getStringParam(params, "repo_dir", cfg.RepoDir)
	analysedDir := getStringParam(params, "analysed_dir", cfg.AnalysedDir)
	force := getBoolParam(params, "force", false)
	maxDays := getIntParam(params, "max_days", cfg.TaskCreate.SilicaMaxDays)
	startDate := getStringParam(params, "start_date", "")
	endDate := getStringParam(params, "end_date", "")
	date := getStringParam(params, "date", "")
	if date == "" {
		startDate = applyAnalysisFloor(startDate)
	}
	return runImportRepo(repoDir, analysedDir, force, maxDays, startDate, endDate, date)
}

func executeImportOrg(params map[string]interface{}) error {
	fromDB := getStringParam(params, "from_db", cfg.OrgDSN)
	fromCSV := getStringParam(params, "from_csv", "")
	toCSV := getStringParam(params, "to_csv", "")
	return runImportOrg(fromDB, fromCSV, toCSV)
}

func executeImport(params map[string]interface{}) error {
	taskDir := getStringParam(params, "task_dir", cfg.TaskDir)
	repoDir := getStringParam(params, "repo_dir", cfg.RepoDir)
	analysedDir := getStringParam(params, "analysed_dir", cfg.AnalysedDir)
	force := getBoolParam(params, "force", false)
	fromDB := getStringParam(params, "from_db", cfg.OrgDSN)
	fromCSV := getStringParam(params, "from_csv", "")
	startDate := getStringParam(params, "start_date", "")
	endDate := getStringParam(params, "end_date", "")
	date := getStringParam(params, "date", "")
	maxDays := getIntParam(params, "max_days", cfg.TaskCreate.SilicaMaxDays)
	createPseudo := getBoolParam(params, "create_pseudo", cfg.TaskCreate.CreatePseudoTask)
	// 未显式传 start-date 且非单日(date)模式时，套全局分析起始日下界。
	// 一处生效全流程：import-conv/import-repo/efficiency(-v2) 都用这个 startDate。
	if date == "" {
		startDate = applyAnalysisFloor(startDate)
	}
	// 增量 days 窗【只用于取数】(import-conv/repo)：仅增量加行、安全。
	// efficiency 重算仍用原始 startDate（cron 的 days 不传时=空=全量），避免跨窗 need 被
	// 窗内 commit 部分覆盖（commit_ids 是覆盖更新，会永久丢掉窗外老 commit）。
	ingestStart := resolveStartDateByDays(params, startDate, date)

	steps := []struct {
		name string
		fn   func() error
	}{
		{"import-conv", func() error {
			return runImportConv(taskDir, analysedDir, force, ingestStart, endDate, date, createPseudo)
		}},
		{"import-repo", func() error {
			return runImportRepo(repoDir, analysedDir, force, maxDays, ingestStart, endDate, date)
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
		{"efficiency-v2", func() error { return runEfficiencyV2(startDate, endDate, date) }},
	}

	for _, step := range steps {
		logx.Infof("========== [import] 步骤: %s ==========", step.name)
		if err := step.fn(); err != nil {
			logx.Errorf("步骤 %s 失败: %v", step.name, err)
		}
	}

	logx.Info("========== [import] 全部步骤完成 ==========")
	return nil
}

func executeEfficiencyV2(params map[string]interface{}) error {
	// 注意：efficiency-v2 不应用 days 增量窗——窗内 commit 重解析会覆盖跨窗 need 的 commit_ids、
	// 丢掉窗外老 commit。只接受显式 start_date/end_date/date（调用方明确窗内 need 完整时才用）。
	startDate := getStringParam(params, "start_date", "")
	endDate := getStringParam(params, "end_date", "")
	date := getStringParam(params, "date", "")
	if date == "" {
		startDate = applyAnalysisFloor(startDate)
	}
	return runEfficiencyV2(startDate, endDate, date)
}

func executeFixTask(params map[string]interface{}) error {
	taskDir := getStringParam(params, "task_dir", cfg.TaskDir)
	startDate := getStringParam(params, "start_date", "")
	endDate := getStringParam(params, "end_date", "")
	date := getStringParam(params, "date", "")
	specificTask := getStringParam(params, "task", "")
	max := getIntParam(params, "max", 0)
	if date == "" && specificTask == "" {
		startDate = applyAnalysisFloor(startDate)
	}
	return runFixTask(taskDir, startDate, endDate, date, specificTask, max)
}

func executeFixCommit(params map[string]interface{}) error {
	repoDir := getStringParam(params, "repo_dir", cfg.RepoDir)
	startDate := getStringParam(params, "start_date", "")
	endDate := getStringParam(params, "end_date", "")
	date := getStringParam(params, "date", "")
	commitID := getStringParam(params, "commit", "")
	max := getIntParam(params, "max", 0)
	if date == "" && commitID == "" {
		startDate = applyAnalysisFloor(startDate)
	}
	return runFixCommit(repoDir, startDate, endDate, date, commitID, max)
}
