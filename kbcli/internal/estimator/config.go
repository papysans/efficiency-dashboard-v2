// Package estimator 持有古法（算法）估时的共享配置类型，供 main 的 algo_estimator
// 与 efficiency-v2 需求聚合(uncovered 人工估时)共用，避免二者跨包形成 import 环。
package estimator

// EstimateConfig 古法估时系数（从 config.go 迁入；main Config.AlgoEstimation 引用本类型）。
type EstimateConfig struct {
	MaxInputChars        float64 `yaml:"max_input_chars"`         //最大输入字符数
	MaxRatio             float64 `yaml:"max_ratio"`               //工作量的最大倍数(相比real_minutes)
	MaxFactor            float64 `yaml:"max_factor"`              //最大的加权系数
	MinFactor            float64 `yaml:"min_factor"`              //最小的加权系数
	IncharsPerMinutes    float64 `yaml:"inchars_per_minutes"`     //人每分钟输入20个字
	LinesPerMinutes      float64 `yaml:"lines_per_minutes"`       //人每分钟输入2行代码
	MinMinutes           float64 `yaml:"min_minutes"`             //最小分钟数
	CommitLinePerMinutes float64 `yaml:"commit_line_per_minutes"` //传统开发人天代码量基准值（行/人天），默认值100行/人天
	CommitMinutesPerLine float64 `yaml:"commit_minutes_per_line"` //传统开发每行代码耗时；优先级高于 commit_line_per_minutes
}
