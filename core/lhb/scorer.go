package lhb

import (
	"sort"
	"strings"
)

// IsTopYouzi 判断是否为顶级游资
func IsTopYouzi(name string) bool {
	for _, youzi := range TopYouziList {
		if strings.Contains(name, youzi) {
			return true
		}
	}
	return false
}

// IsFamousYouzi 判断是否为知名游资
func IsFamousYouzi(name string) bool {
	for _, youzi := range FamousYouziList {
		if strings.Contains(name, youzi) {
			return true
		}
	}
	return false
}

// IsInstitution 判断是否为机构
func IsInstitution(name string) bool {
	for _, kw := range InstitutionKeywords {
		if strings.Contains(name, kw) {
			return true
		}
	}
	return false
}

// ScoreStocks 对龙虎榜数据按股票分组评分，返回降序排列的评分列表
func ScoreStocks(records []LHBRecord) []StockScore {
	// 按股票代码分组
	groups := make(map[string][]LHBRecord)
	nameMap := make(map[string]string)
	for _, r := range records {
		groups[r.Symbol] = append(groups[r.Symbol], r)
		if r.Name != "" {
			nameMap[r.Symbol] = r.Name
		}
	}

	scores := make([]StockScore, 0, len(groups))
	for symbol, recs := range groups {
		qs := scoreQuality(recs)
		is := scoreInflow(recs)
		ss := scoreSellPressure(recs)
		inst := scoreInstitution(recs)
		bs := scoreBonus(recs)
		total := qs + is + ss + inst + bs

		// 收集顶级游资名称
		topNames := collectTopYouzi(recs)
		hasInst := hasInstitution(recs)

		scores = append(scores, StockScore{
			Symbol:           symbol,
			Name:             nameMap[symbol],
			TotalScore:       total,
			QualityScore:     qs,
			InflowScore:      is,
			SellScore:        ss,
			InstitutionScore: inst,
			BonusScore:       bs,
			TopYouziNames:    topNames,
			HasInstitution:   hasInst,
			SeatCount:        len(recs),
			Records:          recs,
		})
	}

	// 按总分降序
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].TotalScore > scores[j].TotalScore
	})
	return scores
}

// scoreQuality 资金质量分（0-30分）
func scoreQuality(records []LHBRecord) float64 {
	var score float64
	for _, r := range records {
		if r.BuyAmount <= 0 {
			continue
		}
		combined := r.YouziName + " " + r.YYB
		if IsTopYouzi(combined) {
			score += 10.0
		} else if IsFamousYouzi(combined) {
			score += 5.0
		} else {
			score += 1.5
		}
	}
	if score > 30 {
		score = 30
	}
	return score
}

// scoreInflow 净流入分（0-25分），金额单位：元，转万元计算
func scoreInflow(records []LHBRecord) float64 {
	var totalNet float64
	for _, r := range records {
		if r.NetInflow > 0 {
			totalNet += r.NetInflow
		}
	}
	wan := totalNet / 10000.0 // 转万元

	var score float64
	switch {
	case wan < 1000:
		score = (wan / 1000.0) * 10.0
	case wan < 5000:
		score = 10.0 + ((wan-1000.0)/4000.0)*8.0
	case wan < 10000:
		score = 18.0 + ((wan-5000.0)/5000.0)*4.0
	default:
		extra := (wan - 10000.0) / 10000.0
		if extra > 1 {
			extra = 1
		}
		score = 22.0 + extra*3.0
	}
	if score > 25 {
		score = 25
	}
	return score
}

// scoreSellPressure 卖出压力分（0-20分）
func scoreSellPressure(records []LHBRecord) float64 {
	var totalBuy, totalSell float64
	for _, r := range records {
		totalBuy += r.BuyAmount
		totalSell += r.SellAmount
	}
	if totalBuy+totalSell <= 0 {
		return 10.0 // 无数据时给中间分
	}

	sellRatio := totalSell / (totalBuy + totalSell)
	var score float64
	switch {
	case sellRatio < 0.1:
		score = 20.0
	case sellRatio < 0.3:
		score = 20.0 - (sellRatio-0.1)/0.2*5.0
	case sellRatio < 0.5:
		score = 15.0 - (sellRatio-0.3)/0.2*5.0
	case sellRatio < 0.8:
		score = 10.0 - (sellRatio-0.5)/0.3*5.0
	default:
		remain := (sellRatio - 0.8) / 0.2
		if remain > 1 {
			remain = 1
		}
		score = 5.0 - remain*5.0
	}
	if score < 0 {
		score = 0
	}
	return score
}

// scoreInstitution 机构参与分（0-15分）
func scoreInstitution(records []LHBRecord) float64 {
	var institutionCount, youziCount int
	hasInst := false
	hasYouzi := false

	for _, r := range records {
		combined := r.YouziName + " " + r.YYB
		if IsInstitution(combined) {
			institutionCount++
			hasInst = true
		} else if r.BuyAmount > 0 {
			youziCount++
			hasYouzi = true
		}
	}

	if hasInst && hasYouzi {
		return 15.0
	}
	if hasInst {
		score := 8.0 + float64(institutionCount)*2.0
		if score > 15 {
			score = 15
		}
		return score
	}
	if hasYouzi {
		score := 5.0 + float64(youziCount)
		if score > 15 {
			score = 15
		}
		return score
	}
	return 0
}

// scoreBonus Bonus分（0-10分）
func scoreBonus(records []LHBRecord) float64 {
	var score float64

	// 1. 席位集中度：同一游资多次出现
	youziCount := make(map[string]int)
	for _, r := range records {
		if r.YouziName != "" {
			youziCount[r.YouziName]++
		}
	}
	for _, cnt := range youziCount {
		if cnt >= 2 {
			score += 2.0
			break
		}
	}

	// 2. 热门概念匹配
	for _, r := range records {
		for _, kw := range HotConceptKeywords {
			if strings.Contains(r.Concepts, kw) {
				score += 2.0
				goto conceptDone
			}
		}
	}
conceptDone:

	// 3. 买卖比
	var totalBuy, totalSell float64
	for _, r := range records {
		totalBuy += r.BuyAmount
		totalSell += r.SellAmount
	}
	if totalSell > 0 && totalBuy/totalSell > 0.7 {
		score += 2.0
	} else if totalSell == 0 && totalBuy > 0 {
		score += 2.0 // 只买不卖
	}

	if score > 10 {
		score = 10
	}
	return score
}

// collectTopYouzi 收集记录中的顶级游资名称
func collectTopYouzi(records []LHBRecord) []string {
	seen := make(map[string]bool)
	var result []string
	for _, r := range records {
		combined := r.YouziName + " " + r.YYB
		if IsTopYouzi(combined) && !seen[r.YouziName] {
			seen[r.YouziName] = true
			result = append(result, r.YouziName)
		}
	}
	return result
}

// hasInstitution 判断记录中是否有机构参与
func hasInstitution(records []LHBRecord) bool {
	for _, r := range records {
		combined := r.YouziName + " " + r.YYB
		if IsInstitution(combined) {
			return true
		}
	}
	return false
}
