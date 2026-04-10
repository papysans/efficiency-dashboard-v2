package sentiment

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"comdigger/core/httputil"
)

// StockGrowth 个股成长数据
type StockGrowth struct {
	Code         string  // 股票代码（f12）
	Name         string  // 股票名称（f14）
	Price        float64 // 最新价（f2）
	ChangePct    float64 // 涨跌幅%（f3）
	NetProfitYoY float64 // 净利润同比增长率%（f46）
	RevenueYoY   float64 // 营收同比增长率%（f41）
	ROE          float64 // 加权ROE%（f37）
	PE           float64 // PE-TTM（f9）
	PB           float64 // 市净率（f23）
	MarketCap    float64 // 总市值元（f20）
	Industry     string  // 所属行业（f100）
	GrossMargin  float64 // 毛利率%（f49，备用，不参与过滤）
	DebtRatio    float64 // 资产负债率%（f57，备用，不参与过滤和评分）
	Turnover     float64 // 换手率%（f8，备用，不参与过滤和评分）
	Score        float64 // 综合评分0-100
}

// FetchGrowthRank 获取成长排行数据
// mode: "profit"（净利润增长降序）/ "revenue"（营收增长降序）/ "roe"（ROE降序）
// pz: 拉取条数（建议500）
func FetchGrowthRank(mode string, pz int) ([]StockGrowth, error) {
	var fid string
	switch mode {
	case "revenue":
		fid = "f41"
	case "roe":
		fid = "f37"
	default: // "profit"
		fid = "f46"
	}

	url := fmt.Sprintf(
		"https://push2.eastmoney.com/api/qt/clist/get?cb=jQuery112309_1&fid=%s&po=1&pz=%d&pn=1&np=1&fltt=2&invt=2&fs=m:0+t:6,m:0+t:13,m:0+t:80,m:1+t:2,m:1+t:23&fields=f12,f14,f2,f3,f46,f41,f37,f9,f23,f20,f100,f49,f57,f8",
		fid, pz,
	)

	bodyBytes, err := httputil.FetchURL(context.Background(), url, emHeaders())
	if err != nil {
		return nil, fmt.Errorf("获取成长排行失败: %w", err)
	}

	// 剥离 JSONP 回调：jQuery112309_1({...})
	bodyStr := string(bodyBytes)
	start := strings.Index(bodyStr, "(")
	end := strings.LastIndex(bodyStr, ")")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("解析成长排行响应格式异常")
	}
	jsonStr := bodyStr[start+1 : end]

	var result struct {
		Data struct {
			Diff []map[string]interface{} `json:"diff"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("解析成长排行响应失败: %w", err)
	}

	if len(result.Data.Diff) == 0 {
		return nil, nil
	}

	var stocks []StockGrowth
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

		stocks = append(stocks, StockGrowth{
			Code:         code,
			Name:         name,
			Price:        parseToFloat64(item["f2"]),
			ChangePct:    parseToFloat64(item["f3"]),
			NetProfitYoY: parseToFloat64(item["f46"]),
			RevenueYoY:   parseToFloat64(item["f41"]),
			ROE:          parseToFloat64(item["f37"]),
			PE:           parseToFloat64(item["f9"]),
			PB:           parseToFloat64(item["f23"]),
			MarketCap:    parseToFloat64(item["f20"]),
			Industry:     industry,
			GrossMargin:  parseToFloat64(item["f49"]),
			DebtRatio:    parseToFloat64(item["f57"]),
			Turnover:     parseToFloat64(item["f8"]),
		})
	}
	return stocks, nil
}

// FilterGrowthStocks 过滤成长股
// mode: "profit" / "revenue" / "roe"
func FilterGrowthStocks(stocks []StockGrowth, mode string) []StockGrowth {
	result := make([]StockGrowth, 0, len(stocks))
	for _, s := range stocks {
		// 硬过滤（所有模式通用）
		// 排除ST股票
		if strings.Contains(strings.ToUpper(s.Name), "ST") {
			continue
		}
		// 排除科创板（代码以688开头）
		if strings.HasPrefix(s.Code, "688") {
			continue
		}
		// 排除负PE或零PE
		if s.PE <= 0 {
			continue
		}

		// 模式过滤
		switch mode {
		case "revenue":
			// 营收增长模式：营收增长≥15%，净利润增长≥0%，ROE≥3%，PE≤100
			if s.RevenueYoY < 15 || s.NetProfitYoY < 0 || s.ROE < 3 || s.PE > 100 {
				continue
			}
		case "roe":
			// ROE优选模式：ROE≥15%，净利润增长≥5%，PE≤60
			if s.ROE < 15 || s.NetProfitYoY < 5 || s.PE > 60 {
				continue
			}
		default: // "profit" 净利润增长模式
			// 净利润增长≥10%，营收增长≥0%，ROE≥5%，PE≤80
			if s.NetProfitYoY < 10 || s.RevenueYoY < 0 || s.ROE < 5 || s.PE > 80 {
				continue
			}
		}

		result = append(result, s)
	}
	return result
}

// ScoreGrowthStocks 对成长股评分并排序（降序）
// Score = 40%×Norm(NetProfitYoY) + 30%×Norm(RevenueYoY) + 20%×Norm(ROE) + 10%×Norm(GrossMargin)
func ScoreGrowthStocks(stocks []StockGrowth) []StockGrowth {
	if len(stocks) == 0 {
		return stocks
	}

	// 找各维度 min/max
	minNPY, maxNPY := stocks[0].NetProfitYoY, stocks[0].NetProfitYoY
	minRVY, maxRVY := stocks[0].RevenueYoY, stocks[0].RevenueYoY
	minROE, maxROE := stocks[0].ROE, stocks[0].ROE
	minGM, maxGM := stocks[0].GrossMargin, stocks[0].GrossMargin

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
	}

	// 归一化辅助函数
	norm := func(x, min, max float64) float64 {
		if max == min {
			return 0.5
		}
		return (x - min) / (max - min)
	}

	// 计算评分
	for i := range stocks {
		s := &stocks[i]
		score := 0.40*norm(s.NetProfitYoY, minNPY, maxNPY) +
			0.30*norm(s.RevenueYoY, minRVY, maxRVY) +
			0.20*norm(s.ROE, minROE, maxROE) +
			0.10*norm(s.GrossMargin, minGM, maxGM)
		s.Score = score * 100
	}

	// 按Score降序排序
	sort.Slice(stocks, func(i, j int) bool {
		return stocks[i].Score > stocks[j].Score
	})

	return stocks
}
