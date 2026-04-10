package technical

import "math"

// MACD 指数平滑异同移动平均线
// 返回：DIF, DEA, MACD
func MACD(close []float64, short, long, m int) (dif, dea, macd []float64) {
	if len(close) == 0 {
		return []float64{}, []float64{}, []float64{}
	}
	emaShort := EMA(close, short)
	emaLong := EMA(close, long)
	dif = make([]float64, len(close))
	for i := range dif {
		if math.IsNaN(emaShort[i]) || math.IsNaN(emaLong[i]) {
			dif[i] = math.NaN()
		} else {
			dif[i] = emaShort[i] - emaLong[i]
		}
	}
	dea = EMA(dif, m)
	macd = make([]float64, len(close))
	for i := range macd {
		if math.IsNaN(dif[i]) || math.IsNaN(dea[i]) {
			macd[i] = math.NaN()
		} else {
			macd[i] = (dif[i] - dea[i]) * 2
		}
	}
	return
}

// KDJ 随机指标
// 返回：K, D, J
func KDJ(close, high, low []float64, n, m1, m2 int) (k, d, j []float64) {
	if len(close) == 0 || len(high) == 0 || len(low) == 0 {
		return []float64{}, []float64{}, []float64{}
	}
	llv := LLV(low, n)
	hhv := HHV(high, n)
	rsv := make([]float64, len(close))
	for i := range rsv {
		if math.IsNaN(llv[i]) || math.IsNaN(hhv[i]) {
			rsv[i] = math.NaN()
		} else {
			denominator := hhv[i] - llv[i]
			if denominator == 0 {
				rsv[i] = 0
			} else {
				rsv[i] = (close[i] - llv[i]) / denominator * 100
			}
		}
	}
	k = EMA(rsv, m1*2-1)
	d = EMA(k, m2*2-1)
	j = make([]float64, len(close))
	for i := range j {
		if math.IsNaN(k[i]) || math.IsNaN(d[i]) {
			j[i] = math.NaN()
		} else {
			j[i] = k[i]*3 - d[i]*2
		}
	}
	return
}

// RSI 相对强弱指标
func RSI(close []float64, n int) []float64 {
	if len(close) == 0 {
		return []float64{}
	}
	dif := DIFF(close, 1)
	maxDif := make([]float64, len(dif))
	absDif := make([]float64, len(dif))
	for i := range dif {
		if math.IsNaN(dif[i]) {
			maxDif[i] = math.NaN()
			absDif[i] = math.NaN()
		} else {
			if dif[i] > 0 {
				maxDif[i] = dif[i]
			} else {
				maxDif[i] = 0
			}
			absDif[i] = math.Abs(dif[i])
		}
	}
	smaMax := SMA(maxDif, n, 1)
	smaAbs := SMA(absDif, n, 1)
	result := make([]float64, len(close))
	for i := range result {
		if math.IsNaN(smaMax[i]) || math.IsNaN(smaAbs[i]) || smaAbs[i] == 0 {
			result[i] = math.NaN()
		} else {
			result[i] = smaMax[i] / smaAbs[i] * 100
		}
	}
	return result
}

// BOLL 布林带指标
// 返回：UPPER, MID, LOWER
func BOLL(close []float64, n, p int) (upper, mid, lower []float64) {
	if len(close) == 0 {
		return []float64{}, []float64{}, []float64{}
	}
	mid = MA(close, n)
	std := STD(close, n)
	upper = make([]float64, len(close))
	lower = make([]float64, len(close))
	for i := range upper {
		if math.IsNaN(mid[i]) || math.IsNaN(std[i]) {
			upper[i] = math.NaN()
			lower[i] = math.NaN()
		} else {
			upper[i] = mid[i] + std[i]*float64(p)
			lower[i] = mid[i] - std[i]*float64(p)
		}
	}
	return
}

// DMI 动向指标
// 返回：PDI, MDI, ADX, ADXR
func DMI(close, high, low []float64, m1, m2 int) (pdi, mdi, adx, adxr []float64) {
	if len(close) == 0 || len(high) == 0 || len(low) == 0 {
		return []float64{}, []float64{}, []float64{}, []float64{}
	}
	refClose := REF(close, 1)
	trValues := make([]float64, len(close))
	for i := range trValues {
		val1 := high[i] - low[i]
		val2 := math.Abs(high[i] - refClose[i])
		val3 := math.Abs(low[i] - refClose[i])
		trValues[i] = MAX(val1, MAX(val2, val3))
	}
	tr := SUM(trValues, m1)
	refHigh := REF(high, 1)
	hd := make([]float64, len(high))
	for i := range hd {
		hd[i] = high[i] - refHigh[i]
	}
	refLow := REF(low, 1)
	ld := make([]float64, len(low))
	for i := range ld {
		ld[i] = refLow[i] - low[i]
	}
	hdFloat := make([]float64, len(hd))
	ldFloat := make([]float64, len(ld))
	for i := range hd {
		if hd[i] > 0 && hd[i] > ld[i] {
			hdFloat[i] = hd[i]
		}
		if ld[i] > 0 && ld[i] > hd[i] {
			ldFloat[i] = ld[i]
		}
	}
	dmp := SUM(hdFloat, m1)
	dmm := SUM(ldFloat, m1)
	pdi = make([]float64, len(close))
	mdi = make([]float64, len(close))
	for i := range pdi {
		if math.IsNaN(tr[i]) || tr[i] == 0 {
			pdi[i] = math.NaN()
			mdi[i] = math.NaN()
		} else {
			pdi[i] = dmp[i] * 100 / tr[i]
			mdi[i] = dmm[i] * 100 / tr[i]
		}
	}
	adxValues := make([]float64, len(close))
	for i := range adxValues {
		if math.IsNaN(pdi[i]) || math.IsNaN(mdi[i]) {
			adxValues[i] = math.NaN()
		} else {
			sum := pdi[i] + mdi[i]
			if sum == 0 {
				adxValues[i] = 0
			} else {
				adxValues[i] = math.Abs(mdi[i]-pdi[i]) / sum * 100
			}
		}
	}
	adx = MA(adxValues, m2)
	refAdx := REF(adx, m2)
	adxr = make([]float64, len(close))
	for i := range adxr {
		if math.IsNaN(adx[i]) || math.IsNaN(refAdx[i]) {
			adxr[i] = math.NaN()
		} else {
			adxr[i] = (adx[i] + refAdx[i]) / 2
		}
	}
	return
}

// VR 容量比率
func VR(close, volume []float64, m1 int) []float64 {
	if len(close) == 0 || len(volume) == 0 {
		return []float64{}
	}
	lc := REF(close, 1)
	upVol := make([]float64, len(close))
	downVol := make([]float64, len(close))
	for i := range close {
		if math.IsNaN(lc[i]) {
			upVol[i] = 0
			downVol[i] = 0
		} else {
			if close[i] > lc[i] {
				upVol[i] = volume[i]
				downVol[i] = 0
			} else {
				upVol[i] = 0
				downVol[i] = volume[i]
			}
		}
	}
	upSum := SUM(upVol, m1)
	downSum := SUM(downVol, m1)
	result := make([]float64, len(close))
	for i := range result {
		if math.IsNaN(upSum[i]) || math.IsNaN(downSum[i]) || downSum[i] == 0 {
			result[i] = math.NaN()
		} else {
			result[i] = upSum[i] / downSum[i] * 100
		}
	}
	return result
}

// CCI 顺势指标
// 参数：CLOSE 收盘价，HIGH 最高价，LOW 最低价，N 周期（默认 14）
// 返回：CCI 值序列；>100 超买，<-100 超卖
func CCI(close, high, low []float64, n int) []float64 {
	if len(close) == 0 || len(high) == 0 || len(low) == 0 {
		return []float64{}
	}
	// TP = (HIGH + LOW + CLOSE) / 3
	tp := make([]float64, len(close))
	for i := range tp {
		tp[i] = (high[i] + low[i] + close[i]) / 3
	}
	// CCI = (TP - MA(TP, N)) / (0.015 * AVEDEV(TP, N))
	maTp := MA(tp, n)
	avedevTp := AVEDEV(tp, n)
	result := make([]float64, len(close))
	for i := range result {
		if math.IsNaN(maTp[i]) || math.IsNaN(avedevTp[i]) || avedevTp[i] == 0 {
			result[i] = math.NaN()
		} else {
			result[i] = (tp[i] - maTp[i]) / (0.015 * avedevTp[i])
		}
	}
	return result
}

// BRAR 情绪指标
// 参数：OPEN 开盘价，CLOSE 收盘价，HIGH 最高价，LOW 最低价，M1 周期（默认 26）
// 返回：AR（多空力量），BR（市场意愿）
// AR<50 超卖，AR>150 超买；BR<40 超卖，BR>300 超买
func BRAR(open, close, high, low []float64, m1 int) (ar, br []float64) {
	if len(open) == 0 || len(close) == 0 || len(high) == 0 || len(low) == 0 {
		return []float64{}, []float64{}
	}
	// AR = SUM(HIGH - OPEN, M1) / SUM(OPEN - LOW, M1) * 100
	highOpen := make([]float64, len(high))
	openLow := make([]float64, len(low))
	for i := range high {
		highOpen[i] = high[i] - open[i]
		openLow[i] = open[i] - low[i]
	}
	sumHO := SUM(highOpen, m1)
	sumOL := SUM(openLow, m1)
	ar = make([]float64, len(close))
	for i := range ar {
		if math.IsNaN(sumHO[i]) || math.IsNaN(sumOL[i]) || sumOL[i] == 0 {
			ar[i] = math.NaN()
		} else {
			ar[i] = sumHO[i] / sumOL[i] * 100
		}
	}
	// BR = SUM(MAX(0, HIGH - REF(CLOSE, 1)), M1) / SUM(MAX(0, REF(CLOSE, 1) - LOW), M1) * 100
	refClose := REF(close, 1)
	highRef := make([]float64, len(high))
	refLow := make([]float64, len(low))
	for i := range high {
		hc := high[i] - refClose[i]
		if math.IsNaN(hc) || hc < 0 {
			highRef[i] = 0
		} else {
			highRef[i] = hc
		}
		cl := refClose[i] - low[i]
		if math.IsNaN(cl) || cl < 0 {
			refLow[i] = 0
		} else {
			refLow[i] = cl
		}
	}
	sumHR := SUM(highRef, m1)
	sumRL := SUM(refLow, m1)
	br = make([]float64, len(close))
	for i := range br {
		if math.IsNaN(sumHR[i]) || math.IsNaN(sumRL[i]) || sumRL[i] == 0 {
			br[i] = math.NaN()
		} else {
			br[i] = sumHR[i] / sumRL[i] * 100
		}
	}
	return
}

// BIAS 乖离率
// 参数：CLOSE 收盘价，L1/L2/L3 三个周期（默认 6,12,24）
// 返回：BIAS1, BIAS2, BIAS3（百分比）；>3% 超买，<-3% 超卖
func BIAS(close []float64, l1, l2, l3 int) (bias1, bias2, bias3 []float64) {
	if len(close) == 0 {
		return []float64{}, []float64{}, []float64{}
	}
	// BIAS = (CLOSE - MA(CLOSE, N)) / MA(CLOSE, N) * 100
	ma1 := MA(close, l1)
	ma2 := MA(close, l2)
	ma3 := MA(close, l3)
	bias1 = make([]float64, len(close))
	bias2 = make([]float64, len(close))
	bias3 = make([]float64, len(close))
	for i := range close {
		if math.IsNaN(ma1[i]) || ma1[i] == 0 {
			bias1[i] = math.NaN()
		} else {
			bias1[i] = (close[i] - ma1[i]) / ma1[i] * 100
		}
		if math.IsNaN(ma2[i]) || ma2[i] == 0 {
			bias2[i] = math.NaN()
		} else {
			bias2[i] = (close[i] - ma2[i]) / ma2[i] * 100
		}
		if math.IsNaN(ma3[i]) || ma3[i] == 0 {
			bias3[i] = math.NaN()
		} else {
			bias3[i] = (close[i] - ma3[i]) / ma3[i] * 100
		}
	}
	return
}

// ATR 平均真实波幅（波动率参考，不参与买卖信号）
// 参数：CLOSE 收盘价，HIGH 最高价，LOW 最低价，N 周期（默认 14）
// 返回：ATR 值序列（绝对价格波动幅度）
func ATR(close, high, low []float64, n int) []float64 {
	if len(close) == 0 || len(high) == 0 || len(low) == 0 {
		return []float64{}
	}
	// TR = MAX(HIGH-LOW, |REF(CLOSE,1)-HIGH|, |REF(CLOSE,1)-LOW|)
	refClose := REF(close, 1)
	tr := make([]float64, len(close))
	for i := range tr {
		val1 := high[i] - low[i]
		if math.IsNaN(refClose[i]) {
			tr[i] = val1
		} else {
			val2 := math.Abs(refClose[i] - high[i])
			val3 := math.Abs(refClose[i] - low[i])
			tr[i] = MAX(val1, MAX(val2, val3))
		}
	}
	// ATR = MA(TR, N)
	return MA(tr, n)
}

// ROC 变动率指标
// 返回：ROC, MAROC
func ROC(close []float64, n, m int) (roc, maroc []float64) {
	if len(close) == 0 {
		return []float64{}, []float64{}
	}
	refClose := REF(close, n)
	roc = make([]float64, len(close))
	for i := range roc {
		if math.IsNaN(close[i]) || math.IsNaN(refClose[i]) || refClose[i] == 0 {
			roc[i] = math.NaN()
		} else {
			roc[i] = 100 * (close[i] - refClose[i]) / refClose[i]
		}
	}
	maroc = MA(roc, m)
	return
}
