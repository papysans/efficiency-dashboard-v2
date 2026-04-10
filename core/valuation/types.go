package valuation

import "time"

// ValuationRecord 单条历史估值记录
type ValuationRecord struct {
	CompanyID      string
	TradeDate      time.Time
	PETTM          float64
	PELar          float64
	PBMRQ          float64
	PSTTM          float64
	PCFOcfTTM      float64
	TotalMarketCap float64
}

// ValuationStats 估值统计（历史分位数）
type ValuationStats struct {
	Current ValuationRecord
	// PE-TTM 历史分位数（0-100，越低越便宜）
	PEPct1Y float64
	PEPct3Y float64
	PEPct5Y float64
	// PB 历史分位数
	PBPct1Y float64
	PBPct3Y float64
	PBPct5Y float64
	// PS 历史分位数
	PSPct1Y float64
	PSPct3Y float64
	PSPct5Y float64
}
