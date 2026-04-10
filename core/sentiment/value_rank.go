package sentiment

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"comdigger/core/httputil"
)

// StockValuation 个股估值数据
type StockValuation struct {
	Code         string  // 股票代码（f12）
	Name         string  // 股票名称（f14）
	Price        float64 // 最新价（f2）
	ChangePct    float64 // 涨跌幅%（f3）
	PE           float64 // PE-TTM（f9）
	PB           float64 // 市净率（f23）
	DividendRate float64 // 股息率%（f114）
	ROE          float64 // ROE%（f173）
	Industry     string  // 所属行业（f100）
	MarketCap    float64 // 总市值元（f20）
	Turnover     float64 // 换手率%（f8）
	Score        float64 // 综合评分0-100
}

// FetchValuationRank 获取估值排行数据
// mode: "pe"（PE升序）/ "pb"（PB升序）/ "div"（股息率降序）
// pz: 拉取条数（建议500）
func FetchValuationRank(mode string, pz int) ([]StockValuation, error) {
	var fid string
	var po int
	switch mode {
	case "pb":
		fid = "f23"
		po = 1
	case "div":
		fid = "f114"
		po = 0
	default: // "pe"
		fid = "f9"
		po = 1
	}

	url := fmt.Sprintf(
		"https://push2.eastmoney.com/api/qt/clist/get?cb=jQuery112309_1&fid=%s&po=%d&pz=%d&pn=1&np=1&fltt=2&invt=2&fs=m:0+t:6,m:0+t:13,m:0+t:80,m:1+t:2,m:1+t:23&fields=f12,f14,f2,f3,f9,f23,f114,f100,f20,f8,f173",
		fid, po, pz,
	)

	bodyBytes, err := httputil.FetchURL(context.Background(), url, emHeaders())
	if err != nil {
		return nil, fmt.Errorf("获取估值排行失败: %w", err)
	}

	// 剥离 JSONP 回调：jQuery112309_1({...})
	bodyStr := string(bodyBytes)
	start := strings.Index(bodyStr, "(")
	end := strings.LastIndex(bodyStr, ")")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("解析估值排行响应格式异常")
	}
	jsonStr := bodyStr[start+1 : end]

	var result struct {
		Data struct {
			Diff []map[string]interface{} `json:"diff"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("解析估值排行响应失败: %w", err)
	}

	if len(result.Data.Diff) == 0 {
		return nil, nil
	}

	var stocks []StockValuation
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

		stocks = append(stocks, StockValuation{
			Code:         code,
			Name:         name,
			Price:        parseToFloat64(item["f2"]),
			ChangePct:    parseToFloat64(item["f3"]),
			PE:           parseToFloat64(item["f9"]),
			PB:           parseToFloat64(item["f23"]),
			DividendRate: parseToFloat64(item["f114"]),
			ROE:          parseToFloat64(item["f173"]),
			Industry:     industry,
			MarketCap:    parseToFloat64(item["f20"]),
			Turnover:     parseToFloat64(item["f8"]),
		})
	}
	return stocks, nil
}

// FilterValueStocks 过滤低估值股票
// mode: "pe" / "pb" / "div"
func FilterValueStocks(stocks []StockValuation, mode string) []StockValuation {
	result := make([]StockValuation, 0, len(stocks))
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
		// 排除负PB或零PB
		if s.PB <= 0 {
			continue
		}

		// 模式过滤
		switch mode {
		case "pb":
			// 低PB模式：PB≤2，PE≤50，ROE≥3%
			if s.PB > 2 || s.PE > 50 || s.ROE < 3 {
				continue
			}
		case "div":
			// 高股息模式：股息率≥2%，PE≤50，PB≤5
			if s.DividendRate < 2 || s.PE > 50 || s.PB > 5 {
				continue
			}
		default: // "pe" 综合价值模式
			// PE≤30，PB≤3，股息率≥1%，ROE≥5%
			if s.PE > 30 || s.PB > 3 || s.DividendRate < 1 || s.ROE < 5 {
				continue
			}
		}

		result = append(result, s)
	}
	return result
}

// ScoreValueStocks 对估值股票评分并排序（降序）
// Score = 30%×Norm_inv(PE) + 25%×Norm_inv(PB) + 25%×Norm(股息率) + 20%×Norm(ROE)
func ScoreValueStocks(stocks []StockValuation) []StockValuation {
	if len(stocks) == 0 {
		return stocks
	}

	// 找各维度 min/max
	minPE, maxPE := stocks[0].PE, stocks[0].PE
	minPB, maxPB := stocks[0].PB, stocks[0].PB
	minDiv, maxDiv := stocks[0].DividendRate, stocks[0].DividendRate
	minROE, maxROE := stocks[0].ROE, stocks[0].ROE

	for _, s := range stocks[1:] {
		if s.PE < minPE {
			minPE = s.PE
		}
		if s.PE > maxPE {
			maxPE = s.PE
		}
		if s.PB < minPB {
			minPB = s.PB
		}
		if s.PB > maxPB {
			maxPB = s.PB
		}
		if s.DividendRate < minDiv {
			minDiv = s.DividendRate
		}
		if s.DividendRate > maxDiv {
			maxDiv = s.DividendRate
		}
		if s.ROE < minROE {
			minROE = s.ROE
		}
		if s.ROE > maxROE {
			maxROE = s.ROE
		}
	}

	// 归一化辅助函数
	normInv := func(x, min, max float64) float64 {
		if max == min {
			return 0.5
		}
		return 1 - (x-min)/(max-min)
	}
	norm := func(x, min, max float64) float64 {
		if max == min {
			return 0.5
		}
		return (x - min) / (max - min)
	}

	// 计算评分
	for i := range stocks {
		s := &stocks[i]
		score := 0.30*normInv(s.PE, minPE, maxPE) +
			0.25*normInv(s.PB, minPB, maxPB) +
			0.25*norm(s.DividendRate, minDiv, maxDiv) +
			0.20*norm(s.ROE, minROE, maxROE)
		s.Score = score * 100
	}

	// 按Score降序排序
	sort.Slice(stocks, func(i, j int) bool {
		return stocks[i].Score > stocks[j].Score
	})

	return stocks
}
