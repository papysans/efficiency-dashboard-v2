package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ModelPrice 模型价格配置
type ModelPrice struct {
	InPrice  float64 `yaml:"in_price"`
	OutPrice float64 `yaml:"out_price"`
}

// DatabaseConfig 数据库连接配置
type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
	SSLMode  string `yaml:"sslmode"`
}

// DSN 返回 PostgreSQL 连接字符串
func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.DBName, d.SSLMode)
}

// AIEstimationConfig AI 估时配置
type AIEstimationConfig struct {
	Enabled   bool   `yaml:"enabled"`
	APIKey    string `yaml:"api_key"`
	BaseURL   string `yaml:"base_url"`
	Model     string `yaml:"model"`
	TimeoutMS int    `yaml:"timeout_ms"`
	Prompt    string `yaml:"prompt"`
}

type EstimateConfig struct {
	MaxInputChars     float64 `yaml:"max_input_chars"`     //最大输入字符数
	MaxRatio          float64 `yaml:"max_ratio"`           //工作量的最大倍数(相比chars(user_input) / inchars_per_minutes + real_minutes)
	MaxFactor         float64 `yaml:"max_factor"`          //最大的加权系数
	MinFactor         float64 `yaml:"min_factor"`          //最小的加权系数
	IncharsPerMinutes float64 `yaml:"inchars_per_minutes"` //人每分钟输入20个字
	LinesPerMinutes   float64 `yaml:"lines_per_minutes"`   //人每分钟输入2行代码
	MinMinutes        float64 `yaml:"min_minutes"`         //最小分钟数
}

// CrontabConfig 定时任务配置
type CrontabConfig struct {
	Schedule string                 `yaml:"schedule"`
	Command  string                 `yaml:"command"`
	Params   map[string]interface{} `yaml:"params"`
}

// ServeConfig HTTP服务配置
type ServeConfig struct {
	Port    int             `yaml:"port"`
	Crontab []CrontabConfig `yaml:"crontab"`
}

// Config 全局配置结构
type Config struct {
	ModelPrices    map[string]ModelPrice `yaml:"model_prices"`
	TaskDir        string                `yaml:"task_dir"`
	RepoDir        string                `yaml:"repo_dir"`
	AnalysedDir    string                `yaml:"analysed_dir"`
	OrgCSVFile     string                `yaml:"org_csv_file"`
	AIEstimation   AIEstimationConfig    `yaml:"ai_estimation"`
	BackendURL     string                `yaml:"backend_url"`
	HTTPProxy      string                `yaml:"http_proxy"`
	StatDatabase   DatabaseConfig        `yaml:"stat_database"`
	OrgDSN         string                `yaml:"org_dsn"`
	AlgoEstimation EstimateConfig        `yaml:"algo_estimation"`
	Serve          ServeConfig           `yaml:"serve"`
}

// LoadConfig 从 YAML 文件加载配置
func LoadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	// 默认值
	if config.TaskDir == "" {
		config.TaskDir = "task"
	}
	if config.RepoDir == "" {
		config.RepoDir = "repo"
	}
	if config.AnalysedDir == "" {
		config.AnalysedDir = "analysed"
	}
	if config.OrgCSVFile == "" {
		config.OrgCSVFile = "analysed/org_mapping.csv"
	}
	if config.AIEstimation.TimeoutMS == 0 {
		config.AIEstimation.TimeoutMS = 300000
	}
	if config.AIEstimation.Model == "" {
		config.AIEstimation.Model = "claude-3-5-sonnet-20241022"
	}
	if config.BackendURL == "" {
		config.BackendURL = "http://localhost:9990"
	}
	if config.StatDatabase.Host == "" {
		config.StatDatabase.Host = "localhost"
	}
	if config.StatDatabase.Port == 0 {
		config.StatDatabase.Port = 5432
	}
	if config.StatDatabase.User == "" {
		config.StatDatabase.User = "postgres"
	}
	if config.StatDatabase.Password == "" {
		config.StatDatabase.Password = os.Getenv("STAT_DB_PASSWORD")
	}
	if config.StatDatabase.DBName == "" {
		config.StatDatabase.DBName = "costrict_stat"
	}
	if config.StatDatabase.SSLMode == "" {
		config.StatDatabase.SSLMode = "disable"
	}
	if config.AlgoEstimation.MaxInputChars == 0 {
		config.AlgoEstimation.MaxInputChars = 300000
	}
	if config.AlgoEstimation.MaxRatio == 0 {
		config.AlgoEstimation.MaxRatio = 50
	}
	if config.AlgoEstimation.MaxFactor == 0 {
		config.AlgoEstimation.MaxFactor = 1.0
	}
	if config.AlgoEstimation.MinFactor == 0 {
		config.AlgoEstimation.MinFactor = 0.2
	}
	if config.AlgoEstimation.IncharsPerMinutes == 0 {
		config.AlgoEstimation.IncharsPerMinutes = 20
	}
	if config.AlgoEstimation.LinesPerMinutes == 0 {
		config.AlgoEstimation.LinesPerMinutes = 2
	}
	if config.AlgoEstimation.MinMinutes == 0 {
		config.AlgoEstimation.MinMinutes = 5
	}

	return &config, nil
}
