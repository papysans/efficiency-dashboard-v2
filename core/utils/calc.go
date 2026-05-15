package utils

import "math"

// CalcEfficiencyRatio 计算效能比
// ancientMinutes: 传统开发工时
// realMinutes: 实际AI辅助工时
// 返回: (ancientMinutes - realMinutes) / realMinutes * 100，保留1位小数
func CalcEfficiencyRatio(ancientMinutes, realMinutes float64) float64 {
	if ancientMinutes <= 0 || realMinutes <= 0 || math.IsInf(realMinutes, 0) {
		return 0
	}
	percent := ((ancientMinutes - realMinutes) / realMinutes) * 100
	return math.Round(percent*10) / 10
}

func CalcEfficiencyRatioManual(ancientMinutes, realMinutes float64,
	ancientMinutesManual, realMinutesManual *float64) float64 {

	effectiveAncient := ancientMinutes
	if ancientMinutesManual != nil {
		effectiveAncient = *ancientMinutesManual
	}
	effectiveReal := realMinutes
	if realMinutesManual != nil {
		effectiveReal = *realMinutesManual
	}
	return CalcEfficiencyRatio(effectiveAncient, effectiveReal)
}
