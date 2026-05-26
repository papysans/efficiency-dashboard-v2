package main

import (
	"os"
	"reflect"
	"testing"
)

// TP-53: 正常加载配置文件
func TestLoadConfig_Normal(t *testing.T) {
	yaml := `
model_prices:
  GLM-4.7:
    in_price: 0.5
    out_price: 1.0
  Auto:
    in_price: 0.0
    out_price: 0.0
`
	f, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}
	defer os.Remove(f.Name())
	f.WriteString(yaml)
	f.Close()

	cfg, err := LoadConfig(f.Name())
	if err != nil {
		t.Fatalf("LoadConfig 返回错误: %v", err)
	}

	if len(cfg.ModelPrices) != 2 {
		t.Errorf("ModelPrices count: want 2, got %d", len(cfg.ModelPrices))
	}
	glm, ok := cfg.ModelPrices["GLM-4.7"]
	if !ok {
		t.Error("ModelPrices 应包含 GLM-4.7")
	} else {
		if glm.InPrice != 0.5 {
			t.Errorf("GLM-4.7 InPrice: want 0.5, got %f", glm.InPrice)
		}
		if glm.OutPrice != 1.0 {
			t.Errorf("GLM-4.7 OutPrice: want 1.0, got %f", glm.OutPrice)
		}
	}
}

// TP-54: 空配置时使用默认值
func TestLoadConfig_Defaults(t *testing.T) {
	yaml := `
model_prices: {}
`
	f, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}
	defer os.Remove(f.Name())
	f.WriteString(yaml)
	f.Close()

	_, err = LoadConfig(f.Name())
	if err != nil {
		t.Fatalf("LoadConfig 返回错误: %v", err)
	}
}

// TP-55: 文件不存在 → 返回 error
func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("期望返回 error，但未返回")
	}
}

// TP-56: YAML 格式不合法 → 返回 error
func TestLoadConfig_InvalidYAML(t *testing.T) {
	f, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}
	defer os.Remove(f.Name())
	f.WriteString("invalid: yaml: content: [unclosed")
	f.Close()

	_, err = LoadConfig(f.Name())
	if err == nil {
		t.Error("期望返回 error，但未返回")
	}
}

// TP-57: model_prices 为空 map 时 map 非 nil
func TestLoadConfig_EmptyModelPrices(t *testing.T) {
	yaml := `
model_prices: {}
`
	f, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}
	defer os.Remove(f.Name())
	f.WriteString(yaml)
	f.Close()

	cfg, err := LoadConfig(f.Name())
	if err != nil {
		t.Fatalf("LoadConfig 返回错误: %v", err)
	}
	if cfg.ModelPrices == nil {
		t.Error("空 model_prices 应返回非 nil map")
	}
	if len(cfg.ModelPrices) != 0 {
		t.Errorf("空 model_prices 应长度为0, got %d", len(cfg.ModelPrices))
	}
}

// TP-58: 多个模型价格正确加载
func TestLoadConfig_MultipleModelPrices(t *testing.T) {
	yaml := `
model_prices:
  GLM-4.7:
    in_price: 0.5
    out_price: 1.0
  GLM-5:
    in_price: 1.0
    out_price: 2.0
  Kimi-K2.5-Moonshot:
    in_price: 1.0
    out_price: 2.0
  Auto:
    in_price: 0.0
    out_price: 0.0
`
	f, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}
	defer os.Remove(f.Name())
	f.WriteString(yaml)
	f.Close()

	cfg, err := LoadConfig(f.Name())
	if err != nil {
		t.Fatalf("LoadConfig 返回错误: %v", err)
	}
	if len(cfg.ModelPrices) != 4 {
		t.Errorf("ModelPrices count: want 4, got %d", len(cfg.ModelPrices))
	}
	kimi, ok := cfg.ModelPrices["Kimi-K2.5-Moonshot"]
	if !ok {
		t.Error("应包含 Kimi-K2.5-Moonshot")
	} else if kimi.OutPrice != 2.0 {
		t.Errorf("Kimi OutPrice: want 2.0, got %f", kimi.OutPrice)
	}
}

func TestLoadConfig_EfficiencyV2Defaults(t *testing.T) {
	yaml := `
model_prices: {}
`
	f, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}
	defer os.Remove(f.Name())
	f.WriteString(yaml)
	f.Close()

	cfg, err := LoadConfig(f.Name())
	if err != nil {
		t.Fatalf("LoadConfig 返回错误: %v", err)
	}

	if cfg.EfficiencyMode != "legacy" {
		t.Errorf("EfficiencyMode: want legacy, got %s", cfg.EfficiencyMode)
	}
	if cfg.EfficiencyV2.TeamProfile != "balanced" {
		t.Errorf("EfficiencyV2.TeamProfile: want balanced, got %s", cfg.EfficiencyV2.TeamProfile)
	}
	if cfg.EfficiencyV2.IdleThresholdDays != 3 {
		t.Errorf("EfficiencyV2.IdleThresholdDays: want 3, got %d", cfg.EfficiencyV2.IdleThresholdDays)
	}
	if cfg.EfficiencyV2.MaxNeedSpanDays != 30 {
		t.Errorf("EfficiencyV2.MaxNeedSpanDays: want 30, got %d", cfg.EfficiencyV2.MaxNeedSpanDays)
	}
	if cfg.EfficiencyV2.Stage.GapThresholdMinutes != 5 {
		t.Errorf("EfficiencyV2.Stage.GapThresholdMinutes: want 5, got %d", cfg.EfficiencyV2.Stage.GapThresholdMinutes)
	}
	if cfg.EfficiencyV2.Stage.ExtensionMinutes != 2 {
		t.Errorf("EfficiencyV2.Stage.ExtensionMinutes: want 2, got %d", cfg.EfficiencyV2.Stage.ExtensionMinutes)
	}
	if cfg.EfficiencyV2.Stage.MaxInferredDurationGapMinutes != 5 {
		t.Errorf("EfficiencyV2.Stage.MaxInferredDurationGapMinutes: want 5, got %d", cfg.EfficiencyV2.Stage.MaxInferredDurationGapMinutes)
	}
	if cfg.EfficiencyV2.Stage.DefaultEditDurationSeconds != 30 {
		t.Errorf("EfficiencyV2.Stage.DefaultEditDurationSeconds: want 30, got %d", cfg.EfficiencyV2.Stage.DefaultEditDurationSeconds)
	}
	if cfg.EfficiencyV2.Stage.DefaultReadDurationSeconds != 10 {
		t.Errorf("EfficiencyV2.Stage.DefaultReadDurationSeconds: want 10, got %d", cfg.EfficiencyV2.Stage.DefaultReadDurationSeconds)
	}
	if cfg.EfficiencyV2.Stage.DefaultCommandDurationSeconds != 30 {
		t.Errorf("EfficiencyV2.Stage.DefaultCommandDurationSeconds: want 30, got %d", cfg.EfficiencyV2.Stage.DefaultCommandDurationSeconds)
	}
	if cfg.EfficiencyV2.Stage.DefaultMessageCharsPerMinute != 300 {
		t.Errorf("EfficiencyV2.Stage.DefaultMessageCharsPerMinute: want 300, got %d", cfg.EfficiencyV2.Stage.DefaultMessageCharsPerMinute)
	}
	if cfg.EfficiencyV2.Stage.DefaultOtherDurationSeconds != 10 {
		t.Errorf("EfficiencyV2.Stage.DefaultOtherDurationSeconds: want 10, got %d", cfg.EfficiencyV2.Stage.DefaultOtherDurationSeconds)
	}
	if cfg.EfficiencyV2.UncoveredCommit.PreMarginMinutes != 30 {
		t.Errorf("EfficiencyV2.UncoveredCommit.PreMarginMinutes: want 30, got %d", cfg.EfficiencyV2.UncoveredCommit.PreMarginMinutes)
	}
	if cfg.EfficiencyV2.UncoveredCommit.PostMarginMinutes != 60 {
		t.Errorf("EfficiencyV2.UncoveredCommit.PostMarginMinutes: want 60, got %d", cfg.EfficiencyV2.UncoveredCommit.PostMarginMinutes)
	}

	thresholds := cfg.EfficiencyV2.ConfidenceThresholds
	if thresholds.HighSpreadRatioMax != 0.15 {
		t.Errorf("HighSpreadRatioMax: want 0.15, got %f", thresholds.HighSpreadRatioMax)
	}
	if thresholds.MediumSpreadRatioMax != 0.30 {
		t.Errorf("MediumSpreadRatioMax: want 0.30, got %f", thresholds.MediumSpreadRatioMax)
	}
	if thresholds.SilicaSignalMin != 0.30 {
		t.Errorf("SilicaSignalMin: want 0.30, got %f", thresholds.SilicaSignalMin)
	}
	if thresholds.AICodeRatioMin != 0.30 {
		t.Errorf("AICodeRatioMin: want 0.30, got %f", thresholds.AICodeRatioMin)
	}
	if thresholds.UncoveredWorkRatioMax != 0.30 {
		t.Errorf("UncoveredWorkRatioMax: want 0.30, got %f", thresholds.UncoveredWorkRatioMax)
	}
	if thresholds.SingleFeatureContributionMax != 0.80 {
		t.Errorf("SingleFeatureContributionMax: want 0.80, got %f", thresholds.SingleFeatureContributionMax)
	}
	if thresholds.ChurnRatioMax != 0.30 {
		t.Errorf("ChurnRatioMax: want 0.30, got %f", thresholds.ChurnRatioMax)
	}
	if thresholds.RevertRatioMax != 0.20 {
		t.Errorf("RevertRatioMax: want 0.20, got %f", thresholds.RevertRatioMax)
	}
	if thresholds.PostGenerationDeleteRatioMax != 0.15 {
		t.Errorf("PostGenerationDeleteRatioMax: want 0.15, got %f", thresholds.PostGenerationDeleteRatioMax)
	}
	if thresholds.DuplicationRatioMax != 0.40 {
		t.Errorf("DuplicationRatioMax: want 0.40, got %f", thresholds.DuplicationRatioMax)
	}
	if thresholds.OutlierActualToBaselineMax != 5 {
		t.Errorf("OutlierActualToBaselineMax: want 5, got %f", thresholds.OutlierActualToBaselineMax)
	}
	if thresholds.OutlierActualToBaselineMin != 0.10 {
		t.Errorf("OutlierActualToBaselineMin: want 0.10, got %f", thresholds.OutlierActualToBaselineMin)
	}

	baseline := cfg.EfficiencyV2.BaselineDefaults
	if baseline.WeightAlgo != 0.30 {
		t.Errorf("BaselineDefaults.WeightAlgo: want 0.30, got %f", baseline.WeightAlgo)
	}
	if baseline.WeightKNN != 0.45 {
		t.Errorf("BaselineDefaults.WeightKNN: want 0.45, got %f", baseline.WeightKNN)
	}
	if baseline.WeightLLM != 0.25 {
		t.Errorf("BaselineDefaults.WeightLLM: want 0.25, got %f", baseline.WeightLLM)
	}
	if baseline.TeamWorkDensity != 0.25 {
		t.Errorf("BaselineDefaults.TeamWorkDensity: want 0.25, got %f", baseline.TeamWorkDensity)
	}
	if cfg.AlgoEstimation.CommitLinePerMinutes != 100.0/480.0 {
		t.Errorf("AlgoEstimation.CommitLinePerMinutes: want %v, got %v", 100.0/480.0, cfg.AlgoEstimation.CommitLinePerMinutes)
	}
	if cfg.AlgoEstimation.CommitMinutesPerLine != 480.0/100.0 {
		t.Errorf("AlgoEstimation.CommitMinutesPerLine: want %v, got %v", 480.0/100.0, cfg.AlgoEstimation.CommitMinutesPerLine)
	}

	expectedPatterns := []string{
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
	if !reflect.DeepEqual(cfg.EfficiencyV2.VerificationCommandPatterns, expectedPatterns) {
		t.Errorf("VerificationCommandPatterns mismatch:\nwant %#v\ngot  %#v", expectedPatterns, cfg.EfficiencyV2.VerificationCommandPatterns)
	}
}

func TestLoadConfig_CommitMinutesPerLineOverridesLineRate(t *testing.T) {
	yaml := `
model_prices: {}
algo_estimation:
  commit_line_per_minutes: 0.2
  commit_minutes_per_line: 2
`
	f, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}
	defer os.Remove(f.Name())
	f.WriteString(yaml)
	f.Close()

	cfg, err := LoadConfig(f.Name())
	if err != nil {
		t.Fatalf("LoadConfig 返回错误: %v", err)
	}

	if cfg.AlgoEstimation.CommitMinutesPerLine != 2 {
		t.Errorf("CommitMinutesPerLine: want 2, got %v", cfg.AlgoEstimation.CommitMinutesPerLine)
	}
	if cfg.AlgoEstimation.CommitLinePerMinutes != 0.5 {
		t.Errorf("CommitLinePerMinutes: want 0.5, got %v", cfg.AlgoEstimation.CommitLinePerMinutes)
	}
}

func TestLoadConfig_EfficiencyV2ExplicitValues(t *testing.T) {
	yaml := `
model_prices: {}
efficiency_mode: both
efficiency_v2:
  team_profile: metr_senior
  idle_threshold_days: 9
  max_need_span_days: 21
  verification_command_patterns:
    - custom verify
  stage:
    gap_threshold_minutes: 11
    extension_minutes: 4
    max_inferred_duration_gap_minutes: 12
    default_edit_duration_seconds: 40
    default_read_duration_seconds: 20
    default_command_duration_seconds: 50
    default_message_chars_per_minute: 200
    default_other_duration_seconds: 13
  uncovered_commit:
    pre_margin_minutes: 15
    post_margin_minutes: 25
  confidence_thresholds:
    high_spread_ratio_max: 0.10
    medium_spread_ratio_max: 0.20
    silica_signal_min: 0.40
    ai_code_ratio_min: 0.45
    uncovered_work_ratio_max: 0.35
    single_feature_contribution_max: 0.70
    churn_ratio_max: 0.25
    revert_ratio_max: 0.18
    post_generation_delete_ratio_max: 0.12
    duplication_ratio_max: 0.33
    outlier_actual_to_baseline_max: 4
    outlier_actual_to_baseline_min: 0.20
  baseline_defaults:
    weight_algo: 0.20
    weight_knn: 0.50
    weight_llm: 0.30
    team_work_density: 0.40
`
	f, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}
	defer os.Remove(f.Name())
	f.WriteString(yaml)
	f.Close()

	cfg, err := LoadConfig(f.Name())
	if err != nil {
		t.Fatalf("LoadConfig 返回错误: %v", err)
	}

	if cfg.EfficiencyMode != "both" {
		t.Errorf("EfficiencyMode: want both, got %s", cfg.EfficiencyMode)
	}
	if cfg.EfficiencyV2.TeamProfile != "metr_senior" {
		t.Errorf("EfficiencyV2.TeamProfile: want metr_senior, got %s", cfg.EfficiencyV2.TeamProfile)
	}
	if cfg.EfficiencyV2.IdleThresholdDays != 9 {
		t.Errorf("EfficiencyV2.IdleThresholdDays: want 9, got %d", cfg.EfficiencyV2.IdleThresholdDays)
	}
	if cfg.EfficiencyV2.MaxNeedSpanDays != 21 {
		t.Errorf("EfficiencyV2.MaxNeedSpanDays: want 21, got %d", cfg.EfficiencyV2.MaxNeedSpanDays)
	}
	if !reflect.DeepEqual(cfg.EfficiencyV2.VerificationCommandPatterns, []string{"custom verify"}) {
		t.Errorf("VerificationCommandPatterns: want custom verify, got %#v", cfg.EfficiencyV2.VerificationCommandPatterns)
	}
	if cfg.EfficiencyV2.Stage.GapThresholdMinutes != 11 {
		t.Errorf("Stage.GapThresholdMinutes: want 11, got %d", cfg.EfficiencyV2.Stage.GapThresholdMinutes)
	}
	if cfg.EfficiencyV2.Stage.ExtensionMinutes != 4 {
		t.Errorf("Stage.ExtensionMinutes: want 4, got %d", cfg.EfficiencyV2.Stage.ExtensionMinutes)
	}
	if cfg.EfficiencyV2.Stage.MaxInferredDurationGapMinutes != 12 {
		t.Errorf("Stage.MaxInferredDurationGapMinutes: want 12, got %d", cfg.EfficiencyV2.Stage.MaxInferredDurationGapMinutes)
	}
	if cfg.EfficiencyV2.Stage.DefaultEditDurationSeconds != 40 {
		t.Errorf("Stage.DefaultEditDurationSeconds: want 40, got %d", cfg.EfficiencyV2.Stage.DefaultEditDurationSeconds)
	}
	if cfg.EfficiencyV2.Stage.DefaultReadDurationSeconds != 20 {
		t.Errorf("Stage.DefaultReadDurationSeconds: want 20, got %d", cfg.EfficiencyV2.Stage.DefaultReadDurationSeconds)
	}
	if cfg.EfficiencyV2.Stage.DefaultCommandDurationSeconds != 50 {
		t.Errorf("Stage.DefaultCommandDurationSeconds: want 50, got %d", cfg.EfficiencyV2.Stage.DefaultCommandDurationSeconds)
	}
	if cfg.EfficiencyV2.Stage.DefaultMessageCharsPerMinute != 200 {
		t.Errorf("Stage.DefaultMessageCharsPerMinute: want 200, got %d", cfg.EfficiencyV2.Stage.DefaultMessageCharsPerMinute)
	}
	if cfg.EfficiencyV2.Stage.DefaultOtherDurationSeconds != 13 {
		t.Errorf("Stage.DefaultOtherDurationSeconds: want 13, got %d", cfg.EfficiencyV2.Stage.DefaultOtherDurationSeconds)
	}
	if cfg.EfficiencyV2.UncoveredCommit.PreMarginMinutes != 15 {
		t.Errorf("UncoveredCommit.PreMarginMinutes: want 15, got %d", cfg.EfficiencyV2.UncoveredCommit.PreMarginMinutes)
	}
	if cfg.EfficiencyV2.UncoveredCommit.PostMarginMinutes != 25 {
		t.Errorf("UncoveredCommit.PostMarginMinutes: want 25, got %d", cfg.EfficiencyV2.UncoveredCommit.PostMarginMinutes)
	}

	thresholds := cfg.EfficiencyV2.ConfidenceThresholds
	if thresholds.HighSpreadRatioMax != 0.10 ||
		thresholds.MediumSpreadRatioMax != 0.20 ||
		thresholds.SilicaSignalMin != 0.40 ||
		thresholds.AICodeRatioMin != 0.45 ||
		thresholds.UncoveredWorkRatioMax != 0.35 ||
		thresholds.SingleFeatureContributionMax != 0.70 ||
		thresholds.ChurnRatioMax != 0.25 ||
		thresholds.RevertRatioMax != 0.18 ||
		thresholds.PostGenerationDeleteRatioMax != 0.12 ||
		thresholds.DuplicationRatioMax != 0.33 ||
		thresholds.OutlierActualToBaselineMax != 4 ||
		thresholds.OutlierActualToBaselineMin != 0.20 {
		t.Errorf("ConfidenceThresholds not loaded from explicit config: %+v", thresholds)
	}

	baseline := cfg.EfficiencyV2.BaselineDefaults
	if baseline.WeightAlgo != 0.20 ||
		baseline.WeightKNN != 0.50 ||
		baseline.WeightLLM != 0.30 ||
		baseline.TeamWorkDensity != 0.40 {
		t.Errorf("BaselineDefaults not loaded from explicit config: %+v", baseline)
	}
}
