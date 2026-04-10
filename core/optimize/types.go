package optimize

import (
	"database/sql"
	"time"

	"comdigger/core/technical"
)

// ParamGrid 参数网格（各维度候选值列表，空时跳过该维度）
type ParamGrid struct {
	BuyThresholds  []float64 // 买入评分阈值候选值
	SellThresholds []float64 // 卖出评分阈值候选值
	MAFasts        []int     // 快线周期候选值
	MASlows        []int     // 慢线周期候选值
}

// OptimizeResult 单个参数组合的优化结果
type OptimizeResult struct {
	Params map[string]interface{}    // 参数键值对
	Result *technical.BacktestResult // 回测结果
	SortBy string                    // 排序指标名
}

// OptimizeConfig 优化配置
type OptimizeConfig struct {
	Strategy   string    // 策略类型名（如 "tech_score"）
	Grid       ParamGrid // 参数网格
	SortBy     string    // 排序指标："sharpe_ratio"/"total_return"/"calmar_ratio"，默认"sharpe_ratio"
	MaxWorkers int       // 并发数，默认4
	TopN       int       // 返回前N个结果，默认10
	// 持久化专用字段（非持久化场景保持零值即可）
	DB        *sql.DB `json:"-"` // 非空时启用持久化
	JobID     string  // 任务ID；空时由 RunGridSearch 自动生成
	CompanyID string  // 公司ID；用于持久化记录
	Resume    bool    // true 时跳过已完成的参数组合，实现断点续传
}

// WFOConfig Walk-Forward Optimization 配置
type WFOConfig struct {
	OptimizeConfig
	TrainBars int // 训练集K线条数，默认200
	TestBars  int // 测试集K线条数，默认60
}

// OptimizeJob 优化任务摘要（用于查询历史）
type OptimizeJob struct {
	JobID     string
	CompanyID string
	Strategy  string
	CreatedAt time.Time
	Count     int // 已完成的参数组合数
}
