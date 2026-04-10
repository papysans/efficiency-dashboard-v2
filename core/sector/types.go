package sector

// SectorInfo 板块基本信息（涨跌幅）
type SectorInfo struct {
	Code           string  // 板块代码
	Name           string  // 板块名称
	ChangePct      float64 // 涨跌幅(%)
	ChangeAmt      float64 // 涨跌额
	TotalMarketCap float64 // 总市值(元)
	LeadStockCode  string  // 领涨股代码
	LeadStockName  string  // 领涨股名称
	RisingCount    int     // 上涨家数
	FallingCount   int     // 下跌家数
}

// SectorFundFlow 板块资金流向
type SectorFundFlow struct {
	Code               string  // 板块代码
	Name               string  // 板块名称
	ChangePct          float64 // 涨跌幅(%)
	MainNetInflow      float64 // 主力净流入(元)
	MainNetInflowRate  float64 // 主力净流入占比(%)
	SuperNetInflow     float64 // 超大单净流入(元)
	SuperNetInflowRate float64 // 超大单占比(%)
	BigNetInflow       float64 // 大单净流入(元)
	BigNetInflowRate   float64 // 大单占比(%)
	LeadStockCode      string  // 领涨股代码
	LeadStockName      string  // 领涨股名称
}

// MarketOverview 市场指数概况
type MarketOverview struct {
	Code      string  // 指数代码
	Name      string  // 指数名称
	Price     float64 // 最新价
	ChangeAmt float64 // 涨跌额
	ChangePct float64 // 涨跌幅(%)
	Amplitude float64 // 振幅(%)
	Volume    int64   // 成交量
	Amount    float64 // 成交额(元)
}

// FinNews 财经新闻
type FinNews struct {
	Title  string // 标题
	Time   string // 发布时间
	Source string // 来源
}
