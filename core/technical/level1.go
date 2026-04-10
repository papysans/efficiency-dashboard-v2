package technical

import "math"

// COUNT 最近 N 天满足条件的天数
func COUNT(condition []bool, n int) []float64 {
	if len(condition) == 0 || n <= 0 {
		return []float64{}
	}
	floatCond := make([]float64, len(condition))
	for i := range floatCond {
		if condition[i] {
			floatCond[i] = 1
		}
	}
	return SUM(floatCond, n)
}

// EVERY 最近 N 天是否都是 True
func EVERY(condition []bool, n int) bool {
	if len(condition) == 0 || n <= 0 {
		return false
	}
	start := len(condition) - n
	if start < 0 {
		start = 0
	}
	for i := start; i < len(condition); i++ {
		if !condition[i] {
			return false
		}
	}
	return true
}

// EXIST N 日内是否存在一天满足条件
func EXIST(condition []bool, n int) bool {
	if len(condition) == 0 || n <= 0 {
		return false
	}
	start := len(condition) - n
	if start < 0 {
		start = 0
	}
	for i := start; i < len(condition); i++ {
		if condition[i] {
			return true
		}
	}
	return false
}

// BARSLAST 上一次条件成立到当前的周期数
func BARSLAST(condition []bool) int {
	if len(condition) == 0 {
		return -1
	}
	for i := len(condition) - 1; i >= 0; i-- {
		if condition[i] {
			return len(condition) - 1 - i
		}
	}
	return -1
}

// CROSS 判断 s1 上穿 s2（昨天 s1<=s2，今天 s1>s2）
func CROSS(series1, series2 []float64) bool {
	if len(series1) < 2 || len(series2) < 2 {
		return false
	}
	yesterday1 := series1[len(series1)-2]
	yesterday2 := series2[len(series2)-2]
	today1 := series1[len(series1)-1]
	today2 := series2[len(series2)-1]
	return yesterday1 <= yesterday2 && today1 > today2
}

// FORCAST 返回 S 序列 N 周期线性回归后的预测值
func FORCAST(series []float64, n int) float64 {
	if len(series) == 0 || n <= 0 {
		return 0
	}
	if n > len(series) {
		n = len(series)
	}
	data := series[len(series)-n:]
	slope, intercept := linearRegression(data)
	return slope*float64(n) + intercept
}

func linearRegression(y []float64) (slope, intercept float64) {
	n := float64(len(y))
	if n == 0 {
		return 0, 0
	}
	sumX := 0.0
	sumY := 0.0
	for i := range y {
		sumX += float64(i)
		if isNaNOrInf(y[i]) {
			return 0, 0
		}
		sumY += y[i]
	}
	meanX := sumX / n
	meanY := sumY / n
	numerator := 0.0
	denominator := 0.0
	for i := range y {
		dx := float64(i) - meanX
		dy := y[i] - meanY
		numerator += dx * dy
		denominator += dx * dx
	}
	if denominator == 0 {
		return 0, meanY
	}
	slope = numerator / denominator
	intercept = meanY - slope*meanX
	return
}

func isNaNOrInf(val float64) bool {
	return val != val || val == math.Inf(1) || val == math.Inf(-1)
}
