package globalmarket

import "time"

// StockBar 单日K线数据
type StockBar struct {
	Date   time.Time
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume int64
}

// StockSummary 股票摘要（含涨跌幅计算结果）
type StockSummary struct {
	Symbol           string
	Name             string
	LatestClose      float64
	DayChangePct     float64
	FiveDayChangePct float64
	Bars             []StockBar
}

// IndexDef 全球指数定义
type IndexDef struct {
	Symbol string // stooq代码，如 "%5Espx"
	Name   string // 中文名，如 "标普500"
}

// PeerInfo 对标信息
type PeerInfo struct {
	Peers         []string // stooq美股代码列表，如 ["crwd.us","panw.us"]
	PeerNames     []string // 对应英文名
	SectorETF     string   // 行业ETF stooq代码
	SectorETFName string   // 行业ETF名称
	Category      string   // 行业分类中文名
	DualListedUS  string   // 若该公司在美国双重上市的stooq代码，如 "baba.us"，否则为空串
}

// GlobalMarketData 完整市场数据
type GlobalMarketData struct {
	CompanyPeers     PeerInfo
	PeerSummaries    []StockSummary
	IndexSummaries   []StockSummary
	SectorETFSummary *StockSummary
}
