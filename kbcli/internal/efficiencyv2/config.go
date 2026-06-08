package efficiencyv2

import (
	"log"
	"strings"
)

// 本文件持有 efficiency-v2 的配置类型与默认值逻辑（从 main 的 config.go 迁入）。
// main Config.EfficiencyV2 引用 efficiencyv2.EfficiencyV2Config；配置加载后调用 ApplyDefaults。

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
	OutlierEfficiencyRatioMax    float64 `yaml:"outlier_efficiency_ratio_max"`
	OutlierEfficiencyRatioMin    float64 `yaml:"outlier_efficiency_ratio_min"`
	OutlierLocPerCalendarMinMax  float64 `yaml:"outlier_loc_per_calendar_min_max"`
}

// 极端值排除类别枚举：哪些异常类别会被 outlier_flag 标记（=隐藏+不计入聚合）。
const (
	efficiencyV2ExclusionEfficiencyRatio  = "efficiency_ratio"
	efficiencyV2ExclusionLocRate          = "loc_rate"
	efficiencyV2ExclusionActualToBaseline = "actual_to_baseline"
)

// efficiencyV2DefaultExclusionScope 未显式配置 exclusion 时的默认范围：三类全排（=保持历史行为）。
var efficiencyV2DefaultExclusionScope = []string{
	efficiencyV2ExclusionEfficiencyRatio,
	efficiencyV2ExclusionLocRate,
	efficiencyV2ExclusionActualToBaseline,
}

// EfficiencyV2ExclusionConfig 配置哪些异常类别真正"隐藏+不计入聚合"。
// RawScope 用 *[]string 区分"yaml 未配 scope"(nil → 给默认全开) 与"显式 scope: []"(非 nil 空切片 → none)。
// Scope 是 ApplyDefaults 解析(去非法值/去重/补默认)后的实际生效集合，供运行时读取。
type EfficiencyV2ExclusionConfig struct {
	RawScope *[]string `yaml:"scope"`
	Scope    []string  `yaml:"-"`
}

type EfficiencyV2BaselineDefaults struct {
	WeightAlgo      float64 `yaml:"weight_algo"`
	WeightKNN       float64 `yaml:"weight_knn"`
	WeightLLM       float64 `yaml:"weight_llm"`
	TeamWorkDensity float64 `yaml:"team_work_density"`
}

// EfficiencyV2BaselineAlgoOverrides 覆盖算法基线(古法估时)系数；>0 时生效，0=用内置默认。
type EfficiencyV2BaselineAlgoOverrides struct {
	ThinkTurnMin     float64 `yaml:"think_turn_min"`      // 古法每对话轮思考分钟数，默认 5
	ExecFileCoordMin float64 `yaml:"exec_file_coord_min"` // 古法每文件协调分钟数，默认 30
}

type EfficiencyV2Config struct {
	TeamProfile       string `yaml:"team_profile"`
	IdleThresholdDays int    `yaml:"idle_threshold_days"`
	MaxNeedSpanDays   int    `yaml:"max_need_span_days"`
	// BaselineCalendarCalibration 缩放基线日历(=融合工作量/团队密度)，仅作用于日历口径提效比。
	BaselineCalendarCalibration float64                           `yaml:"baseline_calendar_calibration"`
	VerificationCommandPatterns []string                          `yaml:"verification_command_patterns"`
	Stage                       EfficiencyV2StageConfig           `yaml:"stage"`
	UncoveredCommit             EfficiencyV2UncoveredCommitConfig `yaml:"uncovered_commit"`
	ConfidenceThresholds        EfficiencyV2ConfidenceThresholds  `yaml:"confidence_thresholds"`
	Exclusion                   EfficiencyV2ExclusionConfig       `yaml:"exclusion"`
	BaselineDefaults            EfficiencyV2BaselineDefaults      `yaml:"baseline_defaults"`
	BaselineAlgo                EfficiencyV2BaselineAlgoOverrides `yaml:"baseline_algo"`
	// AnchorSetCSV kNN 锚点母表 CSV 路径，供 import-anchor 命令灌入 anchor_set。
	AnchorSetCSV string `yaml:"anchor_set_csv"`
}

var defaultEfficiencyV2VerificationCommandPatterns = []string{
	"go test", "npm test", "npm run test", "yarn test", "pnpm test",
	"pytest", "jest", "cargo test", "mvn test", "gradle test", "./gradlew test",
	"go build", "npm run build", "yarn build", "pnpm build", "make build",
	"cargo build", "mvn package", "gradle build", "./gradlew build",
	"tsc", "npm run typecheck", "yarn typecheck", "pnpm typecheck", "mypy",
	"go vet", "cargo check", "eslint", "npm run lint", "yarn lint", "pnpm lint",
	"golangci-lint", "ruff", "rubocop", "pylint", "rustfmt --check",
	"npm run check", "yarn check", "pnpm check", "make check", "gradle check", "./gradlew check",
}

// ApplyDefaults 给 EfficiencyV2Config 补默认值（原 main applyEfficiencyV2Defaults 的 V2 部分迁入）。
func ApplyDefaults(cfg *EfficiencyV2Config) {
	if cfg.TeamProfile == "" {
		cfg.TeamProfile = "balanced"
	}
	if cfg.IdleThresholdDays == 0 {
		cfg.IdleThresholdDays = 3
	}
	if cfg.MaxNeedSpanDays == 0 {
		cfg.MaxNeedSpanDays = 30
	}
	if len(cfg.VerificationCommandPatterns) == 0 {
		cfg.VerificationCommandPatterns = append([]string(nil), defaultEfficiencyV2VerificationCommandPatterns...)
	}
	if cfg.Stage.GapThresholdMinutes == 0 {
		cfg.Stage.GapThresholdMinutes = 5
	}
	if cfg.Stage.ExtensionMinutes == 0 {
		cfg.Stage.ExtensionMinutes = 2
	}
	if cfg.Stage.MaxInferredDurationGapMinutes == 0 {
		cfg.Stage.MaxInferredDurationGapMinutes = 5
	}
	if cfg.Stage.DefaultEditDurationSeconds == 0 {
		cfg.Stage.DefaultEditDurationSeconds = 30
	}
	if cfg.Stage.DefaultReadDurationSeconds == 0 {
		cfg.Stage.DefaultReadDurationSeconds = 10
	}
	if cfg.Stage.DefaultCommandDurationSeconds == 0 {
		cfg.Stage.DefaultCommandDurationSeconds = 30
	}
	if cfg.Stage.DefaultMessageCharsPerMinute == 0 {
		cfg.Stage.DefaultMessageCharsPerMinute = 300
	}
	if cfg.Stage.DefaultOtherDurationSeconds == 0 {
		cfg.Stage.DefaultOtherDurationSeconds = 10
	}
	if cfg.UncoveredCommit.PreMarginMinutes == 0 {
		cfg.UncoveredCommit.PreMarginMinutes = 30
	}
	if cfg.UncoveredCommit.PostMarginMinutes == 0 {
		cfg.UncoveredCommit.PostMarginMinutes = 60
	}
	if cfg.ConfidenceThresholds.HighSpreadRatioMax == 0 {
		cfg.ConfidenceThresholds.HighSpreadRatioMax = 0.15
	}
	if cfg.ConfidenceThresholds.MediumSpreadRatioMax == 0 {
		cfg.ConfidenceThresholds.MediumSpreadRatioMax = 0.30
	}
	if cfg.ConfidenceThresholds.SilicaSignalMin == 0 {
		cfg.ConfidenceThresholds.SilicaSignalMin = 0.30
	}
	if cfg.ConfidenceThresholds.AICodeRatioMin == 0 {
		cfg.ConfidenceThresholds.AICodeRatioMin = 0.30
	}
	if cfg.ConfidenceThresholds.UncoveredWorkRatioMax == 0 {
		cfg.ConfidenceThresholds.UncoveredWorkRatioMax = 0.30
	}
	if cfg.ConfidenceThresholds.SingleFeatureContributionMax == 0 {
		cfg.ConfidenceThresholds.SingleFeatureContributionMax = 0.80
	}
	if cfg.ConfidenceThresholds.ChurnRatioMax == 0 {
		cfg.ConfidenceThresholds.ChurnRatioMax = 0.30
	}
	if cfg.ConfidenceThresholds.RevertRatioMax == 0 {
		cfg.ConfidenceThresholds.RevertRatioMax = 0.20
	}
	if cfg.ConfidenceThresholds.PostGenerationDeleteRatioMax == 0 {
		cfg.ConfidenceThresholds.PostGenerationDeleteRatioMax = 0.15
	}
	if cfg.ConfidenceThresholds.DuplicationRatioMax == 0 {
		cfg.ConfidenceThresholds.DuplicationRatioMax = 0.40
	}
	if cfg.ConfidenceThresholds.OutlierActualToBaselineMax == 0 {
		cfg.ConfidenceThresholds.OutlierActualToBaselineMax = 5
	}
	if cfg.ConfidenceThresholds.OutlierActualToBaselineMin == 0 {
		cfg.ConfidenceThresholds.OutlierActualToBaselineMin = 0.10
	}
	if cfg.ConfidenceThresholds.OutlierEfficiencyRatioMax == 0 {
		cfg.ConfidenceThresholds.OutlierEfficiencyRatioMax = 10.0
	}
	if cfg.ConfidenceThresholds.OutlierEfficiencyRatioMin == 0 {
		cfg.ConfidenceThresholds.OutlierEfficiencyRatioMin = -2.0
	}
	if cfg.ConfidenceThresholds.OutlierLocPerCalendarMinMax == 0 {
		cfg.ConfidenceThresholds.OutlierLocPerCalendarMinMax = 7.0
	}
	cfg.Exclusion.Scope = resolveEfficiencyV2ExclusionScope(cfg.Exclusion.RawScope)
	if cfg.BaselineCalendarCalibration <= 0 {
		cfg.BaselineCalendarCalibration = 1.0
	}
	if strings.TrimSpace(cfg.AnchorSetCSV) == "" {
		cfg.AnchorSetCSV = "docs/data/efficiency_v2_anchor_set.csv"
	}
	if cfg.BaselineDefaults.WeightAlgo == 0 {
		cfg.BaselineDefaults.WeightAlgo = 0.30
	}
	if cfg.BaselineDefaults.WeightKNN == 0 {
		cfg.BaselineDefaults.WeightKNN = 0.45
	}
	if cfg.BaselineDefaults.WeightLLM == 0 {
		cfg.BaselineDefaults.WeightLLM = 0.25
	}
	if cfg.BaselineDefaults.TeamWorkDensity == 0 {
		cfg.BaselineDefaults.TeamWorkDensity = 0.25
	}
}

// resolveEfficiencyV2ExclusionScope 把 yaml 原始 scope 解析为生效集合：
//   - raw == nil（未配 exclusion 段或未配 scope key）→ 默认三类全开（保持历史行为）
//   - raw != nil（含显式 scope: []）→ 尊重其值；空切片即 none（全不排）
//
// 非法枚举值忽略并告警；重复值去重，顺序保持稳定。
func resolveEfficiencyV2ExclusionScope(raw *[]string) []string {
	if raw == nil {
		return append([]string(nil), efficiencyV2DefaultExclusionScope...)
	}
	seen := make(map[string]bool, len(*raw))
	out := make([]string, 0, len(*raw))
	for _, v := range *raw {
		cat := strings.TrimSpace(v)
		switch cat {
		case efficiencyV2ExclusionEfficiencyRatio, efficiencyV2ExclusionLocRate, efficiencyV2ExclusionActualToBaseline:
			if !seen[cat] {
				seen[cat] = true
				out = append(out, cat)
			}
		default:
			log.Printf("efficiency_v2.exclusion.scope ignores invalid category %q (allowed: efficiency_ratio, loc_rate, actual_to_baseline)\n", v)
		}
	}
	return out
}
