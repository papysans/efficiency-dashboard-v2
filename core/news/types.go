package news

// PlatformConfig 平台配置
type PlatformConfig struct {
	Name           string
	Weight         float64
	Category       string
	CategoryWeight float64
}

// NewsItem 单条新闻
type NewsItem struct {
	Title       string  `json:"title"`
	URL         string  `json:"url"`
	Content     string  `json:"content"`
	Source      string  `json:"source"`
	PublishTime string  `json:"publish_time"`
	Score       float64 `json:"score"`
	Rank        int     `json:"rank"`
}

// APIResponse newsapi.ws4.cn 响应结构
type APIResponse struct {
	Status string     `json:"status"`
	Data   []NewsItem `json:"data"`
	Msg    string     `json:"msg"`
}

// FlowScore 平台流量得分
type FlowScore struct {
	Platform string
	Score    float64
	Level    string
	Count    int
}

// MarketSentiment 市场情绪综合结果
type MarketSentiment struct {
	TotalScore   float64
	Level        string
	TopPlatforms []FlowScore
	HotKeywords  []string
	StockRelated []NewsItem
}

// 类别权重
const (
	WeightFinance = 1.5
	WeightSocial  = 1.2
	WeightNews    = 1.0
	WeightTech    = 0.8
)

// PlatformConfigs 14个平台的权重配置
// 支持平台：eastmoney, xueqiu, cls, sina_finance, weibo, 36kr,
//
//	bilibili, douyin, tieba, juejin, jinritoutiao, tenxunwang, baidu, zhihu
var PlatformConfigs = map[string]PlatformConfig{
	// 财经平台（权重最高）
	"eastmoney":    {Name: "东方财富", Weight: 9, Category: "finance", CategoryWeight: WeightFinance},
	"sina_finance": {Name: "新浪财经", Weight: 9, Category: "finance", CategoryWeight: WeightFinance},
	"xueqiu":       {Name: "雪球", Weight: 8, Category: "finance", CategoryWeight: WeightFinance},
	"cls":          {Name: "财联社", Weight: 8, Category: "finance", CategoryWeight: WeightFinance},
	// 社交平台
	"weibo":    {Name: "微博热搜", Weight: 10, Category: "social", CategoryWeight: WeightSocial},
	"douyin":   {Name: "抖音热点", Weight: 9, Category: "social", CategoryWeight: WeightSocial},
	"zhihu":    {Name: "知乎热榜", Weight: 7, Category: "social", CategoryWeight: WeightSocial},
	"bilibili": {Name: "B站热门", Weight: 6, Category: "social", CategoryWeight: WeightSocial},
	"tieba":    {Name: "百度贴吧", Weight: 5, Category: "social", CategoryWeight: WeightSocial},
	// 新闻平台
	"baidu":        {Name: "百度热搜", Weight: 8, Category: "news", CategoryWeight: WeightNews},
	"jinritoutiao": {Name: "今日头条", Weight: 7, Category: "news", CategoryWeight: WeightNews},
	"tenxunwang":   {Name: "腾讯新闻", Weight: 6, Category: "news", CategoryWeight: WeightNews},
	// 科技平台
	"36kr":   {Name: "36氪", Weight: 6, Category: "tech", CategoryWeight: WeightTech},
	"juejin": {Name: "掘金", Weight: 5, Category: "tech", CategoryWeight: WeightTech},
}

// AllPlatforms 返回所有支持的平台 key 列表
func AllPlatforms() []string {
	platforms := make([]string, 0, len(PlatformConfigs))
	for k := range PlatformConfigs {
		platforms = append(platforms, k)
	}
	return platforms
}
