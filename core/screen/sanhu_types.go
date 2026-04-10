package screen

// SanhuScreenParams 散户乙筛选参数
// 散户乙选股标准：重视ROE、股息率、负债率、扣非增长、现金流质量
// 符合"三高一低"原则：高ROE、高股息、高成长、低负债
type SanhuScreenParams struct {
	MinROE           float64 // 最小ROE（多年平均）
	MinDividendYield float64 // 最小股息率（年度，%）
	MaxDebtRatio     float64 // 最大负债率（%）
	MinCAGR5         float64 // 最小5年扣非净利润复合增长率（%）
	MinCashFlowRatio float64 // 最小经营现金流/净利润比率
	MaxPB            float64 // 最大市净率（PB）
	TopN             int     // 返回数量限制
}

// SanhuScreenResult 散户乙筛选结果
// 包含散户乙选股模型关注的核心财务指标和综合评分
// 可能缺失数据的字段用指针类型，JSON序列化时NULL→null，前端显示为"-"
type SanhuScreenResult struct {
	CompanyID        string   // 公司代码
	CompanyName      string   // 公司名称
	ROE              float64  // 净资产收益率（%）
	ROA              float64  // 总资产收益率（%）
	DividendYield    *float64 // 股息率（年度，%），可能为null
	DividendYieldTTM *float64 // 股息率（TTM，%），可能为null
	DebtRatio        float64  // 负债率（%）
	CAGR5            *float64 // 扣非净利润5年复合增长率（%），可能为null
	CashFlowRatio    *float64 // 货币资金/总资产（%），可能为null
	PB               *float64 // 市净率，可能为null
	Score            float64  // 综合得分（满分100）
}

// 散户乙评分权重常量
// 总分 = 100分，权重分配体现散户乙投资理念
const (
	WeightROE      = 0.20 // ROE权重：20%，最核心指标
	WeightROAProx  = 0.10 // ROA替代权重：10%，反映资产质量
	WeightDividend = 0.25 // 股息率权重：25%，重视股东回报
	WeightDebt     = 0.10 // 负债率权重：10%，风险控制
	WeightGrowth   = 0.20 // 增长权重：20%，成长性要求
	WeightCashFlow = 0.10 // 现金流权重：10%，盈利质量验证
	WeightPB       = 0.05 // PB权重：5%，估值参考
)

// 散户乙默认筛选参数
// 采用散户乙典型的选股标准作为默认值
const (
	DefaultSanhuMinROE           = 15.0 // 默认最小ROE 15%
	DefaultSanhuMinDividendYield = 2.0  // 默认最小股息率 2%
	DefaultSanhuMaxDebtRatio     = 60.0 // 默认最大负债率 60%
	DefaultSanhuMinCAGR5         = 10.0 // 默认最小5年增长 10%
	DefaultSanhuMinCashFlowRatio = 0.8  // 默认最小现金流利润比 0.8
	DefaultSanhuMaxPB            = 5.0  // 默认最大PB 5倍
	DefaultSanhuTopN             = 30   // 默认返回30家
)
