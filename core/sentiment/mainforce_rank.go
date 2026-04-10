package sentiment

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"comdigger/core/httputil"
)

// StockFundFlow 个股主力资金流向数据
type StockFundFlow struct {
	Code              string  // 股票代码
	Name              string  // 股票名称
	Price             float64 // 最新价
	ChangePct         float64 // 涨跌幅(%)
	MainInflow1d      float64 // 今日主力净流入(元)
	MainInflow1dRate  float64 // 今日主力净流入占比(%)
	SuperInflow1d     float64 // 超大单净流入(元)
	SuperInflow1dRate float64 // 超大单占比(%)
	MainInflow5d      float64 // 5日主力净流入(元)
	MainInflow10d     float64 // 10日主力净流入(元)
	ContinuousAcc     float64 // 连续流入累计(元，f267)
	Score             float64 // 综合评分0-100
}

// fetchStockFundFlowRank 内部辅助函数，获取主力资金排行
// 使用 FetchWithPowerShell + JSONP 解析（与 concept_flow.go 相同模式）
// 注意：push2 clist 接口仅交易时段有数据，非交易时间会返回 EOF
func fetchStockFundFlowRank(fid, fields string) ([]StockFundFlow, error) {
	url := fmt.Sprintf(
		"https://push2.eastmoney.com/api/qt/clist/get?cb=jQuery112309_1&fid=%s&po=1&pz=200&pn=1&np=1&fltt=2&invt=2&fs=m:0+t:6,m:0+t:13,m:0+t:80,m:1+t:2,m:1+t:23&fields=%s",
		fid, fields,
	)

	bodyBytes, err := httputil.FetchURL(context.Background(), url, emHeaders())
	if err != nil {
		return nil, fmt.Errorf("获取主力资金排行失败（可能为非交易时间，该接口仅交易时段有数据）: %w", err)
	}

	// 剥离 JSONP 回调：jQuery112309_1({...})
	bodyStr := string(bodyBytes)
	start := strings.Index(bodyStr, "(")
	end := strings.LastIndex(bodyStr, ")")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("解析主力资金排行响应格式异常")
	}
	jsonStr := bodyStr[start+1 : end]

	var result struct {
		Data struct {
			Diff []map[string]interface{} `json:"diff"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("解析主力资金排行响应失败: %w", err)
	}

	if len(result.Data.Diff) == 0 {
		return nil, nil
	}

	var stocks []StockFundFlow
	for _, item := range result.Data.Diff {
		code, _ := item["f12"].(string)
		name, _ := item["f14"].(string)
		if code == "" || name == "" {
			continue
		}
		stocks = append(stocks, StockFundFlow{
			Code:              code,
			Name:              name,
			Price:             parseToFloat64(item["f2"]),
			ChangePct:         parseToFloat64(item["f3"]),
			MainInflow1d:      parseToFloat64(item["f62"]),
			MainInflow1dRate:  parseToFloat64(item["f184"]),
			SuperInflow1d:     parseToFloat64(item["f66"]),
			SuperInflow1dRate: parseToFloat64(item["f69"]),
			MainInflow5d:      parseToFloat64(item["f164"]),
			MainInflow10d:     parseToFloat64(item["f174"]),
			ContinuousAcc:     parseToFloat64(item["f267"]),
		})
	}
	return stocks, nil
}

// FetchMainForceRank1d 获取今日主力净流入排行（按f62降序）
func FetchMainForceRank1d(top int) ([]StockFundFlow, error) {
	stocks, err := fetchStockFundFlowRank("f62", "f12,f14,f2,f3,f62,f184,f66,f69,f164,f174,f267")
	if err != nil {
		return nil, err
	}
	if top > 0 && len(stocks) > top {
		stocks = stocks[:top]
	}
	return stocks, nil
}

// FetchMainForceRank5d 获取5日主力净流入排行（按f164降序）
func FetchMainForceRank5d(top int) ([]StockFundFlow, error) {
	stocks, err := fetchStockFundFlowRank("f164", "f12,f14,f2,f3,f62,f184,f66,f69,f164,f165,f174,f267")
	if err != nil {
		return nil, err
	}
	if top > 0 && len(stocks) > top {
		stocks = stocks[:top]
	}
	return stocks, nil
}

// FetchMainForceRank10d 获取10日主力净流入排行（按f174降序）
func FetchMainForceRank10d(top int) ([]StockFundFlow, error) {
	stocks, err := fetchStockFundFlowRank("f174", "f12,f14,f2,f3,f62,f184,f66,f69,f164,f174,f175,f267")
	if err != nil {
		return nil, err
	}
	if top > 0 && len(stocks) > top {
		stocks = stocks[:top]
	}
	return stocks, nil
}

// FilterAndScore 过滤并评分
// maxChange: 今日涨幅上限（默认9.5%）
func FilterAndScore(stocks []StockFundFlow, maxChange float64) []StockFundFlow {
	// 硬过滤
	filtered := make([]StockFundFlow, 0, len(stocks))
	for _, s := range stocks {
		// 排除ST股票
		if strings.Contains(strings.ToUpper(s.Name), "ST") {
			continue
		}
		// 排除今日涨幅超过上限
		if s.ChangePct > maxChange {
			continue
		}
		// 排除今日主力净流入<=0
		if s.MainInflow1d <= 0 {
			continue
		}
		filtered = append(filtered, s)
	}

	if len(filtered) == 0 {
		return filtered
	}

	// 找各维度最大值
	var maxMain1d, maxMain5d, maxMain10d, maxSuper1d, maxContAcc float64
	for _, s := range filtered {
		if s.MainInflow1d > maxMain1d {
			maxMain1d = s.MainInflow1d
		}
		if s.MainInflow5d > maxMain5d {
			maxMain5d = s.MainInflow5d
		}
		if s.MainInflow10d > maxMain10d {
			maxMain10d = s.MainInflow10d
		}
		if s.SuperInflow1d > maxSuper1d {
			maxSuper1d = s.SuperInflow1d
		}
		if s.ContinuousAcc > maxContAcc {
			maxContAcc = s.ContinuousAcc
		}
	}

	// 计算评分
	for i := range filtered {
		s := &filtered[i]
		score := 0.0
		if maxMain1d > 0 {
			score += 0.30 * (s.MainInflow1d / maxMain1d)
		}
		if maxMain5d > 0 {
			score += 0.25 * (s.MainInflow5d / maxMain5d)
		}
		if maxMain10d > 0 {
			score += 0.20 * (s.MainInflow10d / maxMain10d)
		}
		if maxSuper1d > 0 {
			score += 0.15 * (s.SuperInflow1d / maxSuper1d)
		}
		if maxContAcc > 0 {
			score += 0.10 * (s.ContinuousAcc / maxContAcc)
		}
		s.Score = score * 100
	}

	// 按Score降序排序
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Score > filtered[j].Score
	})

	return filtered
}
