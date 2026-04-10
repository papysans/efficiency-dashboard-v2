package screen

// ScreenParams 筛选参数
type ScreenParams struct {
	MinROE             float64
	MinNetProfitGrowth float64
	MinRevenueGrowth   float64
	MinGrossMargin     float64
	MaxPE              float64
	MaxPB              float64
	MinCashFlowRatio   float64
	Market             string
	ReportType         string
	TopN               int
	Mode               string              // 筛选模式："standard"/"sanhu"/"growth"/"value"
	SanhuParams        *SanhuScreenParams  // 散户乙筛选参数，Mode="sanhu"时使用，可为nil
}

// ScreenResult 筛选结果
type ScreenResult struct {
	CompanyID       string
	CompanyName     string
	ReportDate      string
	ROE             float64
	NetProfit       float64
	Revenue         float64
	GrossMargin     float64
	NetProfitGrowth float64
	RevenueGrowth   float64
	PETTM           float64
	PBMRQ           float64
	PSTTM           float64
	CashFlowRatio   float64
	Score           float64
}

// 筛选模式常量
const (
	ModeStandard = "standard" // 标准筛选模式
	ModeSanhu    = "sanhu"    // 散户乙筛选模式
	ModeGrowth   = "growth"   // 成长型筛选模式
	ModeValue    = "value"    // 价值型筛选模式
)

// 默认筛选参数常量
const (
	DefaultMinROE = 15.0
	DefaultMaxPE  = 30.0
	DefaultTopN   = 50
)
