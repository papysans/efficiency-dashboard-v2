package analyst

import "time"

// 评级值常量
const (
	RatingBuy        = 1 // 买入
	RatingOutperform = 2 // 增持
	RatingNeutral    = 3 // 中性
	RatingSell       = 4 // 卖出
)

// AnalystReport 单条研报记录
type AnalystReport struct {
	CompanyID          string
	StockCode          string
	StockName          string
	OrgName            string
	PublishDate        time.Time
	Title              string
	RatingName         string
	RatingValue        int
	PredictThisYearEPS float64
	PredictNextYearEPS float64
	PredictThisYearPE  float64
	PredictNextYearPE  float64
	InfoCode           string
}

// AnalystSummary 研报汇总统计
type AnalystSummary struct {
	BuyCount         int
	HoldCount        int
	NeutralCount     int
	SellCount        int
	AvgTargetPrice   float64
	MaxTargetPrice   float64
	MinTargetPrice   float64
	AvgThisYearEPS   float64
	AvgNextYearEPS   float64
	RecentUpgrades   int // 近30天评级上调次数
	RecentDowngrades int // 近30天评级下调次数
}
