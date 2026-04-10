package technical

import (
	"math"
)

// RD 四舍五入取 D 位小数
func RD(val float64, digits int) float64 {
	pow := math.Pow(10, float64(digits))
	return math.Round(val*pow) / pow
}

// RET 返回序列倒数第 N 个值，默认返回最后一个
func RET(series []float64, n int) float64 {
	if len(series) == 0 {
		return 0
	}
	if n <= 0 {
		n = 1
	}
	if n > len(series) {
		n = len(series)
	}
	return series[len(series)-n]
}

// ABS 返回绝对值
func ABS(val float64) float64 {
	return math.Abs(val)
}

// MAX 返回两个值中的最大值
func MAX(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// MAXSeries 返回两个序列对应位置的最大值
func MAXSeries(s1, s2 []float64) []float64 {
	if len(s1) == 0 || len(s2) == 0 {
		return []float64{}
	}
	result := make([]float64, len(s1))
	for i := range result {
		if i >= len(s2) {
			result[i] = s1[i]
		} else {
			result[i] = MAX(s1[i], s2[i])
		}
	}
	return result
}

// MIN 返回两个值中的最小值
func MIN(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// MINSeries 返回两个序列对应位置的最小值
func MINSeries(s1, s2 []float64) []float64 {
	if len(s1) == 0 || len(s2) == 0 {
		return []float64{}
	}
	result := make([]float64, len(s1))
	for i := range result {
		if i >= len(s2) {
			result[i] = s1[i]
		} else {
			result[i] = MIN(s1[i], s2[i])
		}
	}
	return result
}

// MA 求序列的 N 日平均值
func MA(series []float64, n int) []float64 {
	if len(series) == 0 || n <= 0 {
		return []float64{}
	}
	result := make([]float64, len(series))
	for i := range result {
		if i < n-1 {
			result[i] = math.NaN()
		} else {
			sum := 0.0
			for j := i - n + 1; j <= i; j++ {
				if math.IsNaN(series[j]) {
					result[i] = math.NaN()
					break
				}
				sum += series[j]
			}
			if !math.IsNaN(result[i]) {
				result[i] = sum / float64(n)
			}
		}
	}
	return result
}

// REF 对序列整体下移动 N
func REF(series []float64, n int) []float64 {
	if len(series) == 0 || n <= 0 {
		return []float64{}
	}
	result := make([]float64, len(series))
	for i := range result {
		if i < n {
			result[i] = math.NaN()
		} else {
			result[i] = series[i-n]
		}
	}
	return result
}

// DIFF 前一个值减后一个值
func DIFF(series []float64, n int) []float64 {
	if len(series) == 0 || n <= 0 {
		return []float64{}
	}
	result := make([]float64, len(series))
	for i := range result {
		if i < n {
			result[i] = math.NaN()
		} else {
			if math.IsNaN(series[i]) || math.IsNaN(series[i-n]) {
				result[i] = math.NaN()
			} else {
				result[i] = series[i] - series[i-n]
			}
		}
	}
	return result
}

// STD 求序列的 N 日标准差（总体标准差，ddof=0）
func STD(series []float64, n int) []float64 {
	if len(series) == 0 || n <= 0 {
		return []float64{}
	}
	result := make([]float64, len(series))
	for i := range result {
		if i < n-1 {
			result[i] = math.NaN()
		} else {
			sum := 0.0
			hasNaN := false
			for j := i - n + 1; j <= i; j++ {
				if math.IsNaN(series[j]) {
					hasNaN = true
					break
				}
				sum += series[j]
			}
			if hasNaN {
				result[i] = math.NaN()
			} else {
				mean := sum / float64(n)
				variance := 0.0
				for j := i - n + 1; j <= i; j++ {
					diff := series[j] - mean
					variance += diff * diff
				}
				variance /= float64(n)
				result[i] = math.Sqrt(variance)
			}
		}
	}
	return result
}

// IF 序列布尔判断
func IF(condition []bool, trueVal, falseVal []float64) []float64 {
	if len(condition) == 0 {
		return []float64{}
	}
	result := make([]float64, len(condition))
	for i := range result {
		if condition[i] {
			if i < len(trueVal) {
				result[i] = trueVal[i]
			}
		} else {
			if i < len(falseVal) {
				result[i] = falseVal[i]
			}
		}
	}
	return result
}

// SUM 对序列求 N 天累计和
func SUM(series []float64, n int) []float64 {
	if len(series) == 0 || n <= 0 {
		return []float64{}
	}
	result := make([]float64, len(series))
	for i := range result {
		if i < n-1 {
			result[i] = math.NaN()
		} else {
			sum := 0.0
			hasNaN := false
			for j := i - n + 1; j <= i; j++ {
				if math.IsNaN(series[j]) {
					hasNaN = true
					break
				}
				sum += series[j]
			}
			if hasNaN {
				result[i] = math.NaN()
			} else {
				result[i] = sum
			}
		}
	}
	return result
}

// HHV 最近 N 天最高值
func HHV(series []float64, n int) []float64 {
	if len(series) == 0 || n <= 0 {
		return []float64{}
	}
	result := make([]float64, len(series))
	for i := range result {
		if i < n-1 {
			result[i] = math.NaN()
		} else {
			maxVal := series[i-n+1]
			hasNaN := false
			for j := i - n + 1; j <= i; j++ {
				if math.IsNaN(series[j]) {
					hasNaN = true
					break
				}
				if series[j] > maxVal {
					maxVal = series[j]
				}
			}
			if hasNaN {
				result[i] = math.NaN()
			} else {
				result[i] = maxVal
			}
		}
	}
	return result
}

// LLV 最近 N 天最低值
func LLV(series []float64, n int) []float64 {
	if len(series) == 0 || n <= 0 {
		return []float64{}
	}
	result := make([]float64, len(series))
	for i := range result {
		if i < n-1 {
			result[i] = math.NaN()
		} else {
			minVal := series[i-n+1]
			hasNaN := false
			for j := i - n + 1; j <= i; j++ {
				if math.IsNaN(series[j]) {
					hasNaN = true
					break
				}
				if series[j] < minVal {
					minVal = series[j]
				}
			}
			if hasNaN {
				result[i] = math.NaN()
			} else {
				result[i] = minVal
			}
		}
	}
	return result
}

// EMA 指数移动平均（α=2/(n+1)）
func EMA(series []float64, n int) []float64 {
	if len(series) == 0 || n <= 0 {
		return []float64{}
	}
	result := make([]float64, len(series))
	alpha := 2.0 / float64(n+1)
	for i := range result {
		if i == 0 {
			result[i] = series[0]
		} else {
			if math.IsNaN(series[i]) {
				result[i] = result[i-1]
			} else {
				if math.IsNaN(result[i-1]) {
					foundValid := false
					for j := i - 1; j >= 0; j-- {
						if !math.IsNaN(series[j]) {
							result[i-1] = series[j]
							foundValid = true
							break
						}
					}
					if !foundValid {
						result[i] = series[i]
					} else {
						result[i] = series[i]*alpha + result[i-1]*(1-alpha)
					}
				} else {
					result[i] = series[i]*alpha + result[i-1]*(1-alpha)
				}
			}
		}
	}
	return result
}

// SMA 中国式的 SMA：SMA(i) = SMA(i-1)*(n-m)/n + x(i)*m/n
func SMA(series []float64, n int, m int) []float64 {
	if len(series) == 0 || n <= 0 {
		return []float64{}
	}
	if m <= 0 {
		m = 1
	}
	result := make([]float64, len(series))
	sum := 0.0
	hasNaN := false
	for i := 0; i < n && i < len(series); i++ {
		if math.IsNaN(series[i]) {
			hasNaN = true
		}
		sum += series[i]
	}
	if hasNaN {
		for i := 0; i < n && i < len(series); i++ {
			result[i] = math.NaN()
		}
	} else {
		for i := 0; i < n && i < len(series); i++ {
			result[i] = sum / float64(i+1)
		}
	}
	for i := n; i < len(series); i++ {
		if math.IsNaN(series[i]) {
			result[i] = result[i-1]
		} else {
			if math.IsNaN(result[i-1]) {
				s := 0.0
				count := 0
				for j := i; j >= 0 && count < n; j-- {
					if !math.IsNaN(series[j]) {
						s += series[j]
						count++
					}
				}
				if count == n {
					result[i] = s / float64(n)
				} else {
					result[i] = math.NaN()
				}
			} else {
				result[i] = (float64(m)*series[i] + float64(n-m)*result[i-1]) / float64(n)
			}
		}
	}
	return result
}

// AVEDEV 平均绝对偏差
func AVEDEV(series []float64, n int) []float64 {
	if len(series) == 0 || n <= 0 {
		return []float64{}
	}
	result := make([]float64, len(series))
	for i := range result {
		if i < n-1 {
			result[i] = math.NaN()
		} else {
			sum := 0.0
			hasNaN := false
			for j := i - n + 1; j <= i; j++ {
				if math.IsNaN(series[j]) {
					hasNaN = true
					break
				}
				sum += series[j]
			}
			if hasNaN {
				result[i] = math.NaN()
			} else {
				mean := sum / float64(n)
				absSum := 0.0
				for j := i - n + 1; j <= i; j++ {
					absSum += math.Abs(series[j] - mean)
				}
				result[i] = absSum / float64(n)
			}
		}
	}
	return result
}

// ADDSeries 序列相加
func ADDSeries(s1, s2 []float64) []float64 {
	if len(s1) == 0 || len(s2) == 0 {
		return []float64{}
	}
	result := make([]float64, len(s1))
	for i := range result {
		if i >= len(s2) {
			result[i] = s1[i]
		} else {
			result[i] = s1[i] + s2[i]
		}
	}
	return result
}
