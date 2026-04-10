package sentiment

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"comdigger/core/httputil"
)

// StockSmallcap 小市值成长股数据
type StockSmallcap struct {
	Code         string  // 股票代码（f12）
	Name         string  // 股票名称（f14）
	Price        float64 // 最新价（f2）
	ChangePct    float64 // 涨跌幅%（f3）
	MarketCap    float64 // 总市值元（f20）
	NetProfitYoY float64 // 净利润同比增长率%（f46）
	RevenueYoY   float64 // 营收同比增长率%（f41）
	ROE          float64 // 加权ROE%（f37）
	PE           float64 // PE-TTM（f9）
	PB           float64 // 市净率（f23）
	Industry     string  // 所属行业（f100）
	GrossMargin  float64 // 毛利率%（f49，备用，不参与过滤）
	DebtRatio    float64 // 资产负债率%（f57，备用，不参与过滤和评分）
	Turnover     float64 // 换手率%（f8，备用，不参与过滤和评分）
	Score        float64 // 综合评分0-100
}

// FetchSmallcapRank 获取小市值排行数据（按总市值升序）
// pz: 拉取条数（建议500）
func FetchSmallcapRank(pz int) ([]StockSmallcap, error) {
	url := fmt.Sprintf(
		"https://push2.eastmoney.com/api/qt/clist/get?cb=jQuery112309_1&fid=f20&po=1&pz=%d&pn=1&np=1&fltt=2&invt=2&fs=m:0+t:6,m:0+t:13,m:0+t:80,m:1+t:2,m:1+t:23&fields=f12,f14,f2,f3,f20,f46,f41,f37,f9,f23,f8,f100,f49,f57",
		pz,
	)

	bodyBytes, err := httputil.FetchURL(context.Background(), url, emHeaders())
	if err != nil {
		return nil, fmt.Errorf("获取小市值排行失败: %w", err)
	}

	// 剥离 JSONP 回调
	bodyStr := string(bodyBytes)
	start := strings.Index(bodyStr, "(")
	end := strings.LastIndex(bodyStr, ")")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("解析小市值排行响应格式异常")
	}
	jsonStr := bodyStr[start+1 : end]

	var result struct {
		Data struct {
			Diff []map[string]interface{} `json:"diff"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("解析小市值排行响应失败: %w", err)
	}

	if len(result.Data.Diff) == 0 {
		return nil, nil
	}

	var stocks []StockSmallcap
	for _, item := range result.Data.Diff {
		code, ok := item["f12"].(string)
		if !ok || code == "" {
			continue
		}
		name, ok := item["f14"].(string)
		if !ok || name == "" {
			continue
		}
		industry, _ := item["f100"].(string)

		stocks = append(stocks, StockSmallcap{
			Code:         code,
			Name:         name,
			Price:        parseToFloat64(item["f2"]),
			ChangePct:    parseToFloat64(item["f3"]),
			MarketCap:    parseToFloat64(item["f20"]),
			NetProfitYoY: parseToFloat64(item["f46"]),
			RevenueYoY:   parseToFloat64(item["f41"]),
			ROE:          parseToFloat64(item["f37"]),
			PE:           parseToFloat64(item["f9"]),
			PB:           parseToFloat64(item["f23"]),
			Industry:     industry,
			GrossMargin:  parseToFloat64(item["f49"]),
			DebtRatio:    parseToFloat64(item["f57"]),
			Turnover:     parseToFloat64(item["f8"]),
		})
	}
	return stocks, nil
}

// FilterSmallcapStocks 过滤小市值成长股
// capLimit: 市值上限，单位亿元（如 50.0）
func FilterSmallcapStocks(stocks []StockSmallcap, capLimit float64) []StockSmallcap {
	result := make([]StockSmallcap, 0, len(stocks))
	for _, s := range stocks {
		// 排除ST股票
		if strings.Contains(strings.ToUpper(s.Name), "ST") {
			continue
		}
		// 排除科创板（代码以688开头）
		if strings.HasPrefix(s.Code, "688") {
			continue
		}
		// 排除负PE或零PE（亏损或无效）
		if s.PE <= 0 {
			continue
		}
		// 市值过滤（capLimit亿元 → 元）
		if s.MarketCap > capLimit*1e8 {
			continue
		}
		// 增长过滤
		if s.NetProfitYoY < 20 || s.RevenueYoY < 10 || s.ROE < 5 || s.PE > 80 {
			continue
		}
		result = append(result, s)
	}
	return result
}

// ScoreSmallcapStocks 对小市值成长股评分并排序（降序）
// Score = 0.35×Norm(NetProfitYoY) + 0.25×Norm(RevenueYoY) + 0.20×Norm(ROE) + 0.10×NormInv(MarketCap) + 0.10×Norm(GrossMargin)
func ScoreSmallcapStocks(stocks []StockSmallcap) []StockSmallcap {
	if len(stocks) == 0 {
		return stocks
	}

	// 找各维度 min/max
	minNPY, maxNPY := stocks[0].NetProfitYoY, stocks[0].NetProfitYoY
	minRVY, maxRVY := stocks[0].RevenueYoY, stocks[0].RevenueYoY
	minROE, maxROE := stocks[0].ROE, stocks[0].ROE
	minGM, maxGM := stocks[0].GrossMargin, stocks[0].GrossMargin
	minMC, maxMC := stocks[0].MarketCap, stocks[0].MarketCap

	for _, s := range stocks[1:] {
		if s.NetProfitYoY < minNPY {
			minNPY = s.NetProfitYoY
		}
		if s.NetProfitYoY > maxNPY {
			maxNPY = s.NetProfitYoY
		}
		if s.RevenueYoY < minRVY {
			minRVY = s.RevenueYoY
		}
		if s.RevenueYoY > maxRVY {
			maxRVY = s.RevenueYoY
		}
		if s.ROE < minROE {
			minROE = s.ROE
		}
		if s.ROE > maxROE {
			maxROE = s.ROE
		}
		if s.GrossMargin < minGM {
			minGM = s.GrossMargin
		}
		if s.GrossMargin > maxGM {
			maxGM = s.GrossMargin
		}
		if s.MarketCap < minMC {
			minMC = s.MarketCap
		}
		if s.MarketCap > maxMC {
			maxMC = s.MarketCap
		}
	}

	// 归一化辅助函数
	norm := func(x, min, max float64) float64 {
		if max == min {
			return 0.5
		}
		return (x - min) / (max - min)
	}
	normInv := func(x, min, max float64) float64 {
		if max == min {
			return 0.5
		}
		return 1 - (x-min)/(max-min)
	}

	// 计算评分
	for i := range stocks {
		s := &stocks[i]
		score := 0.35*norm(s.NetProfitYoY, minNPY, maxNPY) +
			0.25*norm(s.RevenueYoY, minRVY, maxRVY) +
			0.20*norm(s.ROE, minROE, maxROE) +
			0.10*normInv(s.MarketCap, minMC, maxMC) +
			0.10*norm(s.GrossMargin, minGM, maxGM)
		s.Score = score * 100
	}

	// 按Score降序排序
	sort.Slice(stocks, func(i, j int) bool {
		return stocks[i].Score > stocks[j].Score
	})

	return stocks
}
