package lhb

// LHBRecord 龙虎榜单条记录
type LHBRecord struct {
	Date       string  // YYYY-MM-DD
	Symbol     string  // 股票代码
	Name       string  // 股票名称
	YouziName  string  // 游资名称
	YYB        string  // 营业部
	ListType   string  // 榜单类型
	BuyAmount  float64 // 买入金额（元）
	SellAmount float64 // 卖出金额（元）
	NetInflow  float64 // 净流入金额（元）
	Concepts   string  // 概念
}

// StockScore 股票龙虎榜评分
type StockScore struct {
	Symbol           string
	Name             string
	TotalScore       float64
	QualityScore     float64
	InflowScore      float64
	SellScore        float64
	InstitutionScore float64
	BonusScore       float64
	TopYouziNames    []string
	HasInstitution   bool
	SeatCount        int
	Records          []LHBRecord
}

// 顶级游资名单（18位，每个+10分）
var TopYouziList = []string{
	"赵老哥", "章盟主", "92科比", "瑞鹤仙", "小鳄鱼",
	"养家心法", "欢乐海岸", "古北路", "成都系", "佛山系",
	"方新侠", "乔帮主", "淮海路", "东方财富",
	"国信深圳", "华泰深圳", "中信杭州", "招商深圳",
}

// 知名游资名单（10位，每个+5分）
var FamousYouziList = []string{
	"深股通", "沪股通", "北向资金",
	"中金公司", "中信证券", "国泰君安", "海通证券",
	"广发证券", "华泰证券", "招商证券",
}

// 机构关键词
var InstitutionKeywords = []string{
	"机构专用", "机构", "基金", "保险", "社保",
	"QFII", "RQFII", "券商", "信托",
}

// 热门概念关键词（Bonus加分用）
var HotConceptKeywords = []string{
	"人工智能", "AI", "算力", "新能源", "芯片", "半导体",
	"军工", "医药", "消费", "5G", "新材料", "量子",
	"光伏", "储能", "锂电池", "汽车", "游戏", "传媒",
	"元宇宙", "机器人",
}
