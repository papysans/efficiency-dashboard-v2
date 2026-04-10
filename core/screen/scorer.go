package screen

import "sort"

// CalcGrowthRate 计算同比增长率，prev 为 0 时返回 0
func CalcGrowthRate(current, prev float64) float64 {
	if prev == 0 {
		return 0
	}
	return (current - prev) / prev * 100
}

// normalize 将值归一化到 0-100
func normalize(val, min, max float64) float64 {
	if max <= min {
		return 50
	}
	n := (val - min) / (max - min) * 100
	if n < 0 {
		return 0
	}
	if n > 100 {
		return 100
	}
	return n
}

// ScoreScreenResults 综合评分并降序排序
// 评分权重：ROE(30%) + 净利润增长(25%) + 营收增长(20%) + 毛利率(15%) + 估值(10%)
func ScoreScreenResults(results []ScreenResult) []ScreenResult {
	if len(results) == 0 {
		return results
	}

	// 计算各指标的最大最小值
	minROE, maxROE := results[0].ROE, results[0].ROE
	minNPG, maxNPG := results[0].NetProfitGrowth, results[0].NetProfitGrowth
	minRG, maxRG := results[0].RevenueGrowth, results[0].RevenueGrowth
	minGM, maxGM := results[0].GrossMargin, results[0].GrossMargin
	minPE, maxPE := results[0].PETTM, results[0].PETTM

	for _, r := range results {
		if r.ROE < minROE {
			minROE = r.ROE
		}
		if r.ROE > maxROE {
			maxROE = r.ROE
		}
		if r.NetProfitGrowth < minNPG {
			minNPG = r.NetProfitGrowth
		}
		if r.NetProfitGrowth > maxNPG {
			maxNPG = r.NetProfitGrowth
		}
		if r.RevenueGrowth < minRG {
			minRG = r.RevenueGrowth
		}
		if r.RevenueGrowth > maxRG {
			maxRG = r.RevenueGrowth
		}
		if r.GrossMargin < minGM {
			minGM = r.GrossMargin
		}
		if r.GrossMargin > maxGM {
			maxGM = r.GrossMargin
		}
		if r.PETTM < minPE {
			minPE = r.PETTM
		}
		if r.PETTM > maxPE {
			maxPE = r.PETTM
		}
	}

	for i := range results {
		r := &results[i]
		roeScore := normalize(r.ROE, minROE, maxROE)
		npgScore := normalize(r.NetProfitGrowth, minNPG, maxNPG)
		rgScore := normalize(r.RevenueGrowth, minRG, maxRG)
		gmScore := normalize(r.GrossMargin, minGM, maxGM)
		// PE估值：越低越好，反向归一化
		peScore := 100 - normalize(r.PETTM, minPE, maxPE)
		if r.PETTM <= 0 {
			peScore = 50 // PE为负或0时给中性分
		}
		r.Score = roeScore*0.30 + npgScore*0.25 + rgScore*0.20 + gmScore*0.15 + peScore*0.10
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}
