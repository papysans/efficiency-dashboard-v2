package main

import "fmt"

// createTaskExecutor 根据任务类型创建对应的执行回调函数
func createTaskExecutor(taskType string, params map[string]interface{}) (func() error, error) {
	switch taskType {
	case "import":
		return func() error { return executeImport(params) }, nil
	case "import-task":
		return func() error { return executeImportTask(params) }, nil
	case "import-repo":
		return func() error { return executeImportRepo(params) }, nil
	case "import-org":
		return func() error { return executeImportOrg(params) }, nil
	case "silica":
		return func() error { return executeSilica(params) }, nil
	case "efficiency":
		return func() error { return executeEfficiency(params) }, nil
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

func executeImportTask(params map[string]interface{}) error {
	taskDir := getStringParam(params, "task_dir", cfg.TaskDir)
	analysedDir := getStringParam(params, "analysed_dir", cfg.AnalysedDir)
	force := getBoolParam(params, "force", false)
	startDate := getStringParam(params, "start_date", "")
	endDate := getStringParam(params, "end_date", "")
	date := getStringParam(params, "date", "")
	return runImportTask(taskDir, analysedDir, force, startDate, endDate, date)
}

func executeImportRepo(params map[string]interface{}) error {
	repoDir := getStringParam(params, "repo_dir", cfg.RepoDir)
	analysedDir := getStringParam(params, "analysed_dir", cfg.AnalysedDir)
	force := getBoolParam(params, "force", false)
	startDate := getStringParam(params, "start_date", "")
	endDate := getStringParam(params, "end_date", "")
	date := getStringParam(params, "date", "")
	return runImportRepo(repoDir, analysedDir, force, startDate, endDate, date)
}

func executeImportOrg(params map[string]interface{}) error {
	fromDB := getStringParam(params, "from_db", cfg.OrgDSN)
	fromCSV := getStringParam(params, "from_csv", "")
	toCSV := getStringParam(params, "to_csv", "")
	return runImportOrg(fromDB, fromCSV, toCSV)
}

func executeSilica(params map[string]interface{}) error {
	analysedDir := getStringParam(params, "analysed_dir", cfg.AnalysedDir)
	force := getBoolParam(params, "force", false)
	maxDays := getIntParam(params, "max_days", cfg.SilicaMaxDays)
	startDate := getStringParam(params, "start_date", "")
	endDate := getStringParam(params, "end_date", "")
	date := getStringParam(params, "date", "")
	return runSilica(analysedDir, force, maxDays, startDate, endDate, date)
}

func executeEfficiency(params map[string]interface{}) error {
	startDate := getStringParam(params, "start_date", "")
	endDate := getStringParam(params, "end_date", "")
	date := getStringParam(params, "date", "")
	return runEfficiency(startDate, endDate, date)
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
	maxDays := getIntParam(params, "max_days", cfg.SilicaMaxDays)

	steps := []struct {
		name string
		fn   func() error
	}{
		{"import-task", func() error { return runImportTask(taskDir, analysedDir, force, startDate, endDate, date) }},
		{"import-repo", func() error { return runImportRepo(repoDir, analysedDir, force, startDate, endDate, date) }},
		{"import-org", func() error { return runImportOrg(fromDB, fromCSV, "") }},
		{"silica", func() error { return runSilica(analysedDir, force, maxDays, startDate, endDate, date) }},
		{"efficiency", func() error { return runEfficiency(startDate, endDate, date) }},
	}

	for _, step := range steps {
		logInfof("========== [import] 步骤: %s ==========", step.name)
		if err := step.fn(); err != nil {
			logErrorf("步骤 %s 失败: %v", step.name, err)
		}
	}

	logInfo("========== [import] 全部步骤完成 ==========")
	return nil
}
