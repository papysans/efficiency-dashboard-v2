package factor

import "math"

// TsMean N期均值，数据不足时返回0
func TsMean(values []float64, i, n int) float64 {
	if i < 0 || n <= 0 || i-n+1 < 0 {
		return 0
	}
	start := i - n + 1
	if start < 0 {
		return 0
	}
	sum := 0.0
	for j := start; j <= i; j++ {
		sum += values[j]
	}
	return sum / float64(n)
}

// TsStd N期样本标准差，数据不足时返回0
func TsStd(values []float64, i, n int) float64 {
	if i < 0 || n < 2 || i-n+1 < 0 {
		return 0
	}
	start := i - n + 1
	if start < 0 {
		return 0
	}
	mean := TsMean(values, i, n)
	variance := 0.0
	for j := start; j <= i; j++ {
		diff := values[j] - mean
		variance += diff * diff
	}
	variance /= float64(n - 1)
	return math.Sqrt(variance)
}

// Delay N期前的值，数据不足时返回0
func Delay(values []float64, i, n int) float64 {
	idx := i - n
	if idx < 0 || idx >= len(values) {
		return 0
	}
	return values[idx]
}

// Delta N期变化量（values[i] - values[i-n]），数据不足时返回0
func Delta(values []float64, i, n int) float64 {
	if i < 0 || i >= len(values) {
		return 0
	}
	prev := Delay(values, i, n)
	if prev == 0 && i-n < 0 {
		return 0
	}
	return values[i] - prev
}

// TsMax N期内最大值，数据不足时返回0
func TsMax(values []float64, i, n int) float64 {
	if i < 0 || n <= 0 || i-n+1 < 0 {
		return 0
	}
	start := i - n + 1
	if start < 0 {
		return 0
	}
	max := values[start]
	for j := start + 1; j <= i; j++ {
		if values[j] > max {
			max = values[j]
		}
	}
	return max
}

// TsMin N期内最小值，数据不足时返回0
func TsMin(values []float64, i, n int) float64 {
	if i < 0 || n <= 0 || i-n+1 < 0 {
		return 0
	}
	start := i - n + 1
	if start < 0 {
		return 0
	}
	min := values[start]
	for j := start + 1; j <= i; j++ {
		if values[j] < min {
			min = values[j]
		}
	}
	return min
}

// TsRank 当前值在N期内的时序百分位（0-1），数据不足时返回0
func TsRank(values []float64, i, n int) float64 {
	if i < 0 || n <= 0 || i-n+1 < 0 {
		return 0
	}
	start := i - n + 1
	if start < 0 {
		return 0
	}
	current := values[i]
	count := 0
	total := 0
	for j := start; j <= i; j++ {
		total++
		if current > values[j] {
			count++
		}
	}
	if total == 0 {
		return 0
	}
	return float64(count) / float64(total)
}

// LogVal 自然对数，v<=0时返回0
func LogVal(v float64) float64 {
	if v <= 0 {
		return 0
	}
	return math.Log(v)
}

// AbsVal 绝对值
func AbsVal(v float64) float64 {
	return math.Abs(v)
}

// SignVal 符号：正1/负-1/零0
func SignVal(v float64) float64 {
	if v > 0 {
		return 1
	} else if v < 0 {
		return -1
	}
	return 0
}

// TsCorr 两个序列在位置 i 的最近 n 期皮尔逊相关系数，数据不足或标准差为0时返回0
func TsCorr(values1, values2 []float64, i, n int) float64 {
	if i < n-1 || i >= len(values1) || i >= len(values2) {
		return 0
	}
	start := i - n + 1
	mean1 := 0.0
	mean2 := 0.0
	for j := start; j <= i; j++ {
		mean1 += values1[j]
		mean2 += values2[j]
	}
	mean1 /= float64(n)
	mean2 /= float64(n)

	cov := 0.0
	var1 := 0.0
	var2 := 0.0
	for j := start; j <= i; j++ {
		d1 := values1[j] - mean1
		d2 := values2[j] - mean2
		cov += d1 * d2
		var1 += d1 * d1
		var2 += d2 * d2
	}
	denom := math.Sqrt(var1 * var2)
	if denom == 0 {
		return 0
	}
	return cov / denom
}
