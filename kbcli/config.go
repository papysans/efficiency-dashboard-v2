package main

import (
	"fmt"
	"kanban/kbcli/internal/logx"
	"os"
	"strings"

	"kanban/core/config"
	"kanban/core/storage"
	"kanban/kbcli/internal/efficiencyv2"
	"kanban/kbcli/internal/estimator"
	"kanban/kbcli/internal/llm"

	"gopkg.in/yaml.v3"
)

// ModelPrice 模型价格配置
type ModelPrice struct {
	InPrice  float64 `yaml:"in_price"`
	OutPrice float64 `yaml:"out_price"`
}

type TaskTimeStatistics struct {
	GapThresholdMinutes int `yaml:"gap_threshold_minutes"`
	ExtensionMinutes    int `yaml:"extension_minutes"`
}

type TaskCreateConfig struct {
	SilicaMaxDays    int  `yaml:"silica_max_days"`    // 计算task和commit相关性/硅含量时的最大关联天数
	CreatePseudoTask bool `yaml:"create_pseudo_task"` // 为所有session创建伪任务
}

// CrontabConfig 定时任务配置
type CrontabConfig struct {
	Schedule string                 `yaml:"schedule"`
	Command  string                 `yaml:"command"`
	Params   map[string]interface{} `yaml:"params"`
}

type CommandConfig struct {
	Command string                 `yaml:"command"`
	Params  map[string]interface{} `yaml:"params"`
}

// ServeConfig HTTP服务配置
type ServeConfig struct {
	Port    int             `yaml:"port"`
	Init    CommandConfig   `yaml:"init"`
	Crontab []CrontabConfig `yaml:"crontab"`
}

// Config 全局配置结构
type Config struct {
	ModelPrices    map[string]ModelPrice           `yaml:"model_prices"`
	TaskDir        string                          `yaml:"task_dir"`
	RepoDir        string                          `yaml:"repo_dir"`
	AnalysedDir    string                          `yaml:"analysed_dir"`
	OrgCSVFile     string                          `yaml:"org_csv_file"`
	AIEstimation   llm.AIEstimationConfig          `yaml:"ai_estimation"`
	BackendURL     string                          `yaml:"backend_url"`
	HTTPProxy      string                          `yaml:"http_proxy"`
	EfficiencyV2   efficiencyv2.EfficiencyV2Config `yaml:"efficiency_v2"`
	StatDatabase   config.DatabaseConfig           `yaml:"stat_database"`
	OrgDSN         string                          `yaml:"org_dsn"`
	DeptSync       DeptSyncConfig                  `yaml:"dept_sync"`
	AlgoEstimation estimator.EstimateConfig        `yaml:"algo_estimation"`
	TaskCreate     TaskCreateConfig                `yaml:"task_create"`
	Serve          ServeConfig                     `yaml:"serve"`
	TaskStatistics TaskTimeStatistics              `yaml:"task_statistics"`
	// Storage 存储后端配置。task_dir/repo_dir/analysed_dir 等路径以 s3:// 开头时
	// 走 S3 兼容对象存储（需配置 storage.s3），否则走本地磁盘，允许混搭。
	Storage storage.Config `yaml:"storage"`
	// AnalysisStartDate 全局分析起始日下界（YYYYMMDD，默认空=不设下界）。
	// 未显式传 --start-date / start_date（且未传 date）时，按日期取数/计算的命令自动用它作为
	// 起始下界，从而永不处理该日期之前的数据。显式传 start-date 时以显式为准，不被覆盖。
	AnalysisStartDate string `yaml:"analysis_start_date"`
}

// DeptSyncConfig dept-sync 部门同步服务对接配置（import-dept 使用）
type DeptSyncConfig struct {
	BaseURL  string `yaml:"base_url"`  // dept-sync 服务地址，如 http://127.0.0.1:8080（不含路由前缀）
	QueryKey string `yaml:"query_key"` // 数据接口鉴权头 X-Query-Key 的值
	// FallbackOrgName 投影时 universal_id/工号 都未命中 dept-sync 的看板用户的兜底 org1（默认空=不兜底，保持原样）。
	// 内网建议填 "深信服科技股份有限公司"。
	FallbackOrgName string `yaml:"fallback_org_name"`
	// FallbackDeptName 兜底 org2（默认 "未知部门"），仅在 FallbackOrgName 非空时生效。
	FallbackDeptName string `yaml:"fallback_dept_name"`
}

// applyAnalysisFloor 当未显式指定 start-date 时，用全局 analysis_start_date 作为起始下界。
// 仅在 startDate 为空且 cfg.AnalysisStartDate 非空时生效；调用方需保证 date（单日）语义不被破坏
// （date 与 start-date 互斥的命令应只在未传 date 时调用本函数）。
func applyAnalysisFloor(startDate string) string {
	if strings.TrimSpace(startDate) == "" && cfg != nil && strings.TrimSpace(cfg.AnalysisStartDate) != "" {
		logx.Infof("应用分析起始日下界 analysis_start_date=%s", cfg.AnalysisStartDate)
		return cfg.AnalysisStartDate
	}
	return startDate
}

func LoadFirstConfig(files []string) (*Config, error) {
	for _, fname := range files {
		if fname == "" {
			continue
		}
		if _, err := os.Stat(fname); os.IsNotExist(err) {
			continue
		}
		loadedCfg, err := LoadConfig(fname)
		if err == nil {
			logx.Debugf("load config [%s] ok, cfg: %+v\n", fname, loadedCfg)
			return loadedCfg, nil
		}
		logx.Warnf("load config [%s] failed: %v\n", fname, err)
	}
	return nil, fmt.Errorf("load config [%s] failed", strings.Join(files, ","))
}

const (
	DefaultPageSize = 50
)

// LoadConfig 从 YAML 文件加载配置
func LoadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	if c.Serve.Port == 0 {
		c.Serve.Port = 8080
	}
	if c.TaskDir == "" {
		c.TaskDir = "task"
	}
	if c.RepoDir == "" {
		c.RepoDir = "repo"
	}
	if c.AnalysedDir == "" {
		c.AnalysedDir = "analysed"
	}
	if c.OrgCSVFile == "" {
		c.OrgCSVFile = "analysed/org_mapping.csv"
	}
	if c.AIEstimation.TimeoutMS == 0 {
		c.AIEstimation.TimeoutMS = 300000
	}
	if c.AIEstimation.Model == "" {
		c.AIEstimation.Model = "claude-3-5-sonnet-20241022"
	}
	if c.AIEstimation.APIFormat == "" {
		c.AIEstimation.APIFormat = "anthropic"
	}
	if c.BackendURL == "" {
		c.BackendURL = "http://localhost:9990"
	}
	efficiencyv2.ApplyDefaults(&c.EfficiencyV2)
	if c.StatDatabase.Host == "" {
		c.StatDatabase.Host = "localhost"
	}
	if c.StatDatabase.Port == 0 {
		c.StatDatabase.Port = 5432
	}
	if c.StatDatabase.User == "" {
		c.StatDatabase.User = "postgres"
	}
	if c.StatDatabase.Password == "" {
		c.StatDatabase.Password = os.Getenv("STAT_DB_PASSWORD")
	}
	if c.StatDatabase.DBName == "" {
		c.StatDatabase.DBName = "costrict_stat"
	}
	if c.StatDatabase.SSLMode == "" {
		c.StatDatabase.SSLMode = "disable"
	}
	if c.AlgoEstimation.MaxInputChars == 0 {
		c.AlgoEstimation.MaxInputChars = 300000
	}
	if c.AlgoEstimation.MaxRatio == 0 {
		c.AlgoEstimation.MaxRatio = 10
	}
	if c.AlgoEstimation.MaxFactor == 0 {
		c.AlgoEstimation.MaxFactor = 1.0
	}
	if c.AlgoEstimation.MinFactor == 0 {
		c.AlgoEstimation.MinFactor = 0.2
	}
	if c.AlgoEstimation.IncharsPerMinutes == 0 {
		c.AlgoEstimation.IncharsPerMinutes = 20
	}
	if c.AlgoEstimation.LinesPerMinutes == 0 {
		c.AlgoEstimation.LinesPerMinutes = 2
	}
	if c.AlgoEstimation.MinMinutes == 0 {
		c.AlgoEstimation.MinMinutes = 5
	}
	if c.AlgoEstimation.CommitMinutesPerLine > 0 {
		c.AlgoEstimation.CommitLinePerMinutes = 1 / c.AlgoEstimation.CommitMinutesPerLine
	} else if c.AlgoEstimation.CommitLinePerMinutes == 0 {
		c.AlgoEstimation.CommitLinePerMinutes = estimator.DefaultTraditionalDevLinesPerDay / 480.0
		c.AlgoEstimation.CommitMinutesPerLine = 1 / c.AlgoEstimation.CommitLinePerMinutes
	} else {
		c.AlgoEstimation.CommitMinutesPerLine = 1 / c.AlgoEstimation.CommitLinePerMinutes
	}
	if c.TaskStatistics.ExtensionMinutes == 0 {
		c.TaskStatistics.ExtensionMinutes = 5
	}
	if c.TaskStatistics.GapThresholdMinutes == 0 {
		c.TaskStatistics.GapThresholdMinutes = 10
	}
	if c.TaskCreate.SilicaMaxDays == 0 {
		c.TaskCreate.SilicaMaxDays = 7
	}
	if c.DeptSync.FallbackDeptName == "" {
		c.DeptSync.FallbackDeptName = "未知部门"
	}

	return &c, nil
}
