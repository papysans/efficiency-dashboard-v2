package main

import (
	"fmt"
	"os"
	"strings"

	"kanban/core/config"

	"gopkg.in/yaml.v3"
)

// ModelPrice 模型价格配置
type ModelPrice struct {
	InPrice  float64 `yaml:"in_price"`
	OutPrice float64 `yaml:"out_price"`
}

// AIEstimationConfig AI 估时配置
type AIEstimationConfig struct {
	Enabled   bool   `yaml:"enabled"`
	APIKey    string `yaml:"api_key"`
	BaseURL   string `yaml:"base_url"`
	Model     string `yaml:"model"`
	TimeoutMS int    `yaml:"timeout_ms"`
	HTTPProxy string `yaml:"http_proxy"`
	Prompt    string `yaml:"prompt"`
}

type EstimateConfig struct {
	MaxInputChars        float64 `yaml:"max_input_chars"`         //最大输入字符数
	MaxRatio             float64 `yaml:"max_ratio"`               //工作量的最大倍数(相比real_minutes)
	MaxFactor            float64 `yaml:"max_factor"`              //最大的加权系数
	MinFactor            float64 `yaml:"min_factor"`              //最小的加权系数
	IncharsPerMinutes    float64 `yaml:"inchars_per_minutes"`     //人每分钟输入20个字
	LinesPerMinutes      float64 `yaml:"lines_per_minutes"`       //人每分钟输入2行代码
	MinMinutes           float64 `yaml:"min_minutes"`             //最小分钟数
	CommitLinePerMinutes float64 `yaml:"commit_line_per_minutes"` //传统开发人天代码量基准值（行/人天），默认值100行/人天
}

type TaskTimeStatistics struct {
	GapThresholdMinutes int `yaml:"gap_threshold_minutes"`
	ExtensionMinutes    int `yaml:"extension_minutes"`
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
	ModelPrices      map[string]ModelPrice `yaml:"model_prices"`
	TaskDir          string                `yaml:"task_dir"`
	RepoDir          string                `yaml:"repo_dir"`
	AnalysedDir      string                `yaml:"analysed_dir"`
	OrgCSVFile       string                `yaml:"org_csv_file"`
	AIEstimation     AIEstimationConfig    `yaml:"ai_estimation"`
	BackendURL       string                `yaml:"backend_url"`
	HTTPProxy        string                `yaml:"http_proxy"`
	StatDatabase     config.DatabaseConfig `yaml:"stat_database"`
	OrgDSN           string                `yaml:"org_dsn"`
	AlgoEstimation   EstimateConfig        `yaml:"algo_estimation"`
	SilicaMaxDays    int                   `yaml:"silica_max_days"` //计算task和commit相关性/硅含量时的最大关联天数
	Serve            ServeConfig           `yaml:"serve"`
	TaskStatistics   TaskTimeStatistics    `yaml:"task_statistics"`
	CreatePseudoTask bool                  `yaml:"create_pseudo_task"`
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
			logDebugf("load config [%s] ok, cfg: %+v\n", fname, loadedCfg)
			return loadedCfg, nil
		}
		logWarnf("load config [%s] failed: %v\n", fname, err)
	}
	return nil, fmt.Errorf("load config [%s] failed", strings.Join(files, ","))
}

const (
	DefaultPageSize                  = 50
	DefaultTraditionalDevLinesPerDay = 100 // 传统开发人天代码量基准值（行/人天）
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
	if c.BackendURL == "" {
		c.BackendURL = "http://localhost:9990"
	}
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
	if c.AlgoEstimation.CommitLinePerMinutes == 0 {
		c.AlgoEstimation.CommitLinePerMinutes = DefaultTraditionalDevLinesPerDay / 480.0
	}
	if c.TaskStatistics.ExtensionMinutes == 0 {
		c.TaskStatistics.ExtensionMinutes = 5
	}
	if c.TaskStatistics.GapThresholdMinutes == 0 {
		c.TaskStatistics.GapThresholdMinutes = 10
	}
	if c.SilicaMaxDays == 0 {
		c.SilicaMaxDays = 7
	}

	return &c, nil
}
