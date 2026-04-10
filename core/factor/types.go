package factor

import "time"

// FactorData 单个公司的因子数据（时序）
type FactorData struct {
	CompanyID string
	// Dates 按升序排列（最早在前）
	Dates []time.Time
	// Values 字段名 → 时序数据（与 Dates 一一对应）
	Values map[string][]float64
}

// FactorResult 单个公司的因子计算结果
type FactorResult struct {
	CompanyID   string
	CompanyName string
	Date        time.Time
	FactorValue float64
	Rank        float64 // 全市场百分位（0-1），1表示最高
}
