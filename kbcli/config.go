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
	Enabled      bool              `yaml:"enabled"`
	APIKey       string            `yaml:"api_key"`
	XApiKey      string            `yaml:"x_api_key"`
	BaseURL      string            `yaml:"base_url"`
	Model        string            `yaml:"model"`
	TimeoutMS    int               `yaml:"timeout_ms"`
	HTTPProxy    string            `yaml:"http_proxy"`
	Prompt       string            `yaml:"prompt"`
	APIFormat    string            `yaml:"api_format"`
	ExtraHeaders map[string]string `yaml:"extra_headers"`
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

type TaskCreateConfig struct {
	SilicaMaxDays    int  `yaml:"silica_max_days"`    // 计算task和commit相关性/硅含量时的最大关联天数
	CreatePseudoTask bool `yaml:"create_pseudo_task"` // 为所有session创建伪任务
}

type EfficiencyV2StageConfig struct {
	GapThresholdMinutes           int `yaml:"gap_threshold_minutes"`
	ExtensionMinutes              int `yaml:"extension_minutes"`
	MaxInferredDurationGapMinutes int `yaml:"max_inferred_duration_gap_minutes"`
	DefaultEditDurationSeconds    int `yaml:"default_edit_duration_seconds"`
	DefaultReadDurationSeconds    int `yaml:"default_read_duration_seconds"`
	DefaultCommandDurationSeconds int `yaml:"default_command_duration_seconds"`
	DefaultMessageCharsPerMinute  int `yaml:"default_message_chars_per_minute"`
	DefaultOtherDurationSeconds   int `yaml:"default_other_duration_seconds"`
}

type EfficiencyV2UncoveredCommitConfig struct {
	PreMarginMinutes  int `yaml:"pre_margin_minutes"`
	PostMarginMinutes int `yaml:"post_margin_minutes"`
}

type EfficiencyV2ConfidenceThresholds struct {
	HighSpreadRatioMax           float64 `yaml:"high_spread_ratio_max"`
	MediumSpreadRatioMax         float64 `yaml:"medium_spread_ratio_max"`
	SilicaSignalMin              float64 `yaml:"silica_signal_min"`
	AICodeRatioMin               float64 `yaml:"ai_code_ratio_min"`
	UncoveredWorkRatioMax        float64 `yaml:"uncovered_work_ratio_max"`
	SingleFeatureContributionMax float64 `yaml:"single_feature_contribution_max"`
	ChurnRatioMax                float64 `yaml:"churn_ratio_max"`
	RevertRatioMax               float64 `yaml:"revert_ratio_max"`
	PostGenerationDeleteRatioMax float64 `yaml:"post_generation_delete_ratio_max"`
	DuplicationRatioMax          float64 `yaml:"duplication_ratio_max"`
	OutlierActualToBaselineMax   float64 `yaml:"outlier_actual_to_baseline_max"`
	OutlierActualToBaselineMin   float64 `yaml:"outlier_actual_to_baseline_min"`
}

type EfficiencyV2BaselineDefaults struct {
	WeightAlgo      float64 `yaml:"weight_algo"`
	WeightKNN       float64 `yaml:"weight_knn"`
	WeightLLM       float64 `yaml:"weight_llm"`
	TeamWorkDensity float64 `yaml:"team_work_density"`
}

type EfficiencyV2Config struct {
	TeamProfile                 string                            `yaml:"team_profile"`
	IdleThresholdDays           int                               `yaml:"idle_threshold_days"`
	MaxNeedSpanDays             int                               `yaml:"max_need_span_days"`
	VerificationCommandPatterns []string                          `yaml:"verification_command_patterns"`
	Stage                       EfficiencyV2StageConfig           `yaml:"stage"`
	UncoveredCommit             EfficiencyV2UncoveredCommitConfig `yaml:"uncovered_commit"`
	ConfidenceThresholds        EfficiencyV2ConfidenceThresholds  `yaml:"confidence_thresholds"`
	BaselineDefaults            EfficiencyV2BaselineDefaults      `yaml:"baseline_defaults"`
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
	ModelPrices    map[string]ModelPrice `yaml:"model_prices"`
	TaskDir        string                `yaml:"task_dir"`
	RepoDir        string                `yaml:"repo_dir"`
	AnalysedDir    string                `yaml:"analysed_dir"`
	OrgCSVFile     string                `yaml:"org_csv_file"`
	AIEstimation   AIEstimationConfig    `yaml:"ai_estimation"`
	BackendURL     string                `yaml:"backend_url"`
	HTTPProxy      string                `yaml:"http_proxy"`
	EfficiencyMode string                `yaml:"efficiency_mode"`
	EfficiencyV2   EfficiencyV2Config    `yaml:"efficiency_v2"`
	StatDatabase   config.DatabaseConfig `yaml:"stat_database"`
	OrgDSN         string                `yaml:"org_dsn"`
	AlgoEstimation EstimateConfig        `yaml:"algo_estimation"`
	TaskCreate     TaskCreateConfig      `yaml:"task_create"`
	Serve          ServeConfig           `yaml:"serve"`
	TaskStatistics TaskTimeStatistics    `yaml:"task_statistics"`
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

var defaultEfficiencyV2VerificationCommandPatterns = []string{
	"go test",
	"npm test",
	"npm run test",
	"yarn test",
	"pnpm test",
	"pytest",
	"jest",
	"cargo test",
	"mvn test",
	"gradle test",
	"./gradlew test",
	"go build",
	"npm run build",
	"yarn build",
	"pnpm build",
	"make build",
	"cargo build",
	"mvn package",
	"gradle build",
	"./gradlew build",
	"tsc",
	"npm run typecheck",
	"yarn typecheck",
	"pnpm typecheck",
	"mypy",
	"go vet",
	"cargo check",
	"eslint",
	"npm run lint",
	"yarn lint",
	"pnpm lint",
	"golangci-lint",
	"ruff",
	"rubocop",
	"pylint",
	"rustfmt --check",
	"npm run check",
	"yarn check",
	"pnpm check",
	"make check",
	"gradle check",
	"./gradlew check",
}

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
	applyEfficiencyV2Defaults(&c)
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
	if c.TaskCreate.SilicaMaxDays == 0 {
		c.TaskCreate.SilicaMaxDays = 7
	}

	return &c, nil
}

func applyEfficiencyV2Defaults(c *Config) {
	if c.EfficiencyMode == "" {
		c.EfficiencyMode = "legacy"
	}
	if c.EfficiencyV2.TeamProfile == "" {
		c.EfficiencyV2.TeamProfile = "balanced"
	}
	if c.EfficiencyV2.IdleThresholdDays == 0 {
		c.EfficiencyV2.IdleThresholdDays = 3
	}
	if c.EfficiencyV2.MaxNeedSpanDays == 0 {
		c.EfficiencyV2.MaxNeedSpanDays = 30
	}
	if len(c.EfficiencyV2.VerificationCommandPatterns) == 0 {
		c.EfficiencyV2.VerificationCommandPatterns = append([]string(nil), defaultEfficiencyV2VerificationCommandPatterns...)
	}
	if c.EfficiencyV2.Stage.GapThresholdMinutes == 0 {
		c.EfficiencyV2.Stage.GapThresholdMinutes = 5
	}
	if c.EfficiencyV2.Stage.ExtensionMinutes == 0 {
		c.EfficiencyV2.Stage.ExtensionMinutes = 2
	}
	if c.EfficiencyV2.Stage.MaxInferredDurationGapMinutes == 0 {
		c.EfficiencyV2.Stage.MaxInferredDurationGapMinutes = 5
	}
	if c.EfficiencyV2.Stage.DefaultEditDurationSeconds == 0 {
		c.EfficiencyV2.Stage.DefaultEditDurationSeconds = 30
	}
	if c.EfficiencyV2.Stage.DefaultReadDurationSeconds == 0 {
		c.EfficiencyV2.Stage.DefaultReadDurationSeconds = 10
	}
	if c.EfficiencyV2.Stage.DefaultCommandDurationSeconds == 0 {
		c.EfficiencyV2.Stage.DefaultCommandDurationSeconds = 30
	}
	if c.EfficiencyV2.Stage.DefaultMessageCharsPerMinute == 0 {
		c.EfficiencyV2.Stage.DefaultMessageCharsPerMinute = 300
	}
	if c.EfficiencyV2.Stage.DefaultOtherDurationSeconds == 0 {
		c.EfficiencyV2.Stage.DefaultOtherDurationSeconds = 10
	}
	if c.EfficiencyV2.UncoveredCommit.PreMarginMinutes == 0 {
		c.EfficiencyV2.UncoveredCommit.PreMarginMinutes = 30
	}
	if c.EfficiencyV2.UncoveredCommit.PostMarginMinutes == 0 {
		c.EfficiencyV2.UncoveredCommit.PostMarginMinutes = 60
	}
	if c.EfficiencyV2.ConfidenceThresholds.HighSpreadRatioMax == 0 {
		c.EfficiencyV2.ConfidenceThresholds.HighSpreadRatioMax = 0.15
	}
	if c.EfficiencyV2.ConfidenceThresholds.MediumSpreadRatioMax == 0 {
		c.EfficiencyV2.ConfidenceThresholds.MediumSpreadRatioMax = 0.30
	}
	if c.EfficiencyV2.ConfidenceThresholds.SilicaSignalMin == 0 {
		c.EfficiencyV2.ConfidenceThresholds.SilicaSignalMin = 0.30
	}
	if c.EfficiencyV2.ConfidenceThresholds.AICodeRatioMin == 0 {
		c.EfficiencyV2.ConfidenceThresholds.AICodeRatioMin = 0.30
	}
	if c.EfficiencyV2.ConfidenceThresholds.UncoveredWorkRatioMax == 0 {
		c.EfficiencyV2.ConfidenceThresholds.UncoveredWorkRatioMax = 0.30
	}
	if c.EfficiencyV2.ConfidenceThresholds.SingleFeatureContributionMax == 0 {
		c.EfficiencyV2.ConfidenceThresholds.SingleFeatureContributionMax = 0.80
	}
	if c.EfficiencyV2.ConfidenceThresholds.ChurnRatioMax == 0 {
		c.EfficiencyV2.ConfidenceThresholds.ChurnRatioMax = 0.30
	}
	if c.EfficiencyV2.ConfidenceThresholds.RevertRatioMax == 0 {
		c.EfficiencyV2.ConfidenceThresholds.RevertRatioMax = 0.20
	}
	if c.EfficiencyV2.ConfidenceThresholds.PostGenerationDeleteRatioMax == 0 {
		c.EfficiencyV2.ConfidenceThresholds.PostGenerationDeleteRatioMax = 0.15
	}
	if c.EfficiencyV2.ConfidenceThresholds.DuplicationRatioMax == 0 {
		c.EfficiencyV2.ConfidenceThresholds.DuplicationRatioMax = 0.40
	}
	if c.EfficiencyV2.ConfidenceThresholds.OutlierActualToBaselineMax == 0 {
		c.EfficiencyV2.ConfidenceThresholds.OutlierActualToBaselineMax = 5
	}
	if c.EfficiencyV2.ConfidenceThresholds.OutlierActualToBaselineMin == 0 {
		c.EfficiencyV2.ConfidenceThresholds.OutlierActualToBaselineMin = 0.10
	}
	if c.EfficiencyV2.BaselineDefaults.WeightAlgo == 0 {
		c.EfficiencyV2.BaselineDefaults.WeightAlgo = 0.30
	}
	if c.EfficiencyV2.BaselineDefaults.WeightKNN == 0 {
		c.EfficiencyV2.BaselineDefaults.WeightKNN = 0.45
	}
	if c.EfficiencyV2.BaselineDefaults.WeightLLM == 0 {
		c.EfficiencyV2.BaselineDefaults.WeightLLM = 0.25
	}
	if c.EfficiencyV2.BaselineDefaults.TeamWorkDensity == 0 {
		c.EfficiencyV2.BaselineDefaults.TeamWorkDensity = 0.25
	}
}
