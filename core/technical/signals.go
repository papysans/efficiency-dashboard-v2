package technical

import (
	"fmt"
	"math"

	"comdigger/core/kline"
)

// SignalType 信号类型
type SignalType string

const (
	SignalBuy  SignalType = "买入"
	SignalSell SignalType = "卖出"
	SignalHold SignalType = "观望"
)

// SignalStrength 信号强度（1-5星）
type SignalStrength int

// StrengthStars 将强度转为星形字符串
func StrengthStars(s SignalStrength) string {
	if s < 1 {
		s = 1
	}
	if s > 5 {
		s = 5
	}
	filled := string([]rune("★★★★★")[:s])
	empty := string([]rune("☆☆☆☆☆")[:5-s])
	return filled + empty
}

// IndicatorSignal 单个指标信号
type IndicatorSignal struct {
	Name     string
	Signal   SignalType
	Strength SignalStrength
	Reason   string
}

// SignalResult 综合信号结果
type SignalResult struct {
	Signals []IndicatorSignal
	Overall SignalType
	// Score：综合得分，范围 -100 ~ +100
	// 计算方式：Σ(固定权重 × 强度买卖值) / 有方向信号权重之和 × 100
	// 强度买卖值：买入1★=+0.2 … 5★=+1.0；卖出对称为负；观望不参与分母
	// +100=所有有方向指标全部5★买入，-100=全部5★卖出，0=多空完全平衡
	Score float64
	// ActiveWeight：参与计分的有方向信号权重之和（分母），用于评估得分可信度
	// 越接近 26.5（总权重）说明越多指标给出了明确方向
	ActiveWeight float64
	Summary      string
	// ATR 参考信息（不参与信号聚合）
	ATRValue   float64
	ATRPercent float64 // ATR / 收盘价 * 100，波动率百分比
}

// TechnicalData 各指标计算结果
type TechnicalData struct {
	// MACD
	DIF  []float64
	DEA  []float64
	MACD []float64
	// KDJ
	K []float64
	D []float64
	J []float64
	// RSI
	RSI []float64
	// BOLL
	BollUpper []float64
	BollMid   []float64
	BollLower []float64
	// DMI
	PDI []float64
	MDI []float64
	ADX []float64
	// VR
	VR []float64
	// ROC
	ROC   []float64
	MAROC []float64
	// CCI
	CCI []float64
	// BRAR
	AR []float64
	BR []float64
	// BIAS（6,12,24）
	BIAS1 []float64
	BIAS2 []float64
	BIAS3 []float64
	// ATR（参考，不参与信号）
	ATR []float64
	// 原始价格序列（ATR百分比计算用）
	Close []float64
	// 均线（供策略回测使用）
	MA5  []float64
	MA20 []float64
	MA60 []float64
	MA7  []float64
	MA10 []float64
	MA13 []float64
	// 成交量均线（供策略判断放量/爆量）
	VolumeMA5  []float64
	VolumeMA10 []float64
	// 原始成交量（float64，供策略判断）
	Volume []float64
}

// CalcAllIndicators 计算所有技术指标
func CalcAllIndicators(bars []kline.KlineBar) *TechnicalData {
	n := len(bars)
	if n == 0 {
		return &TechnicalData{}
	}

	close := make([]float64, n)
	open := make([]float64, n)
	high := make([]float64, n)
	low := make([]float64, n)
	volume := make([]float64, n)
	for i, bar := range bars {
		close[i] = bar.Close
		open[i] = bar.Open
		high[i] = bar.High
		low[i] = bar.Low
		volume[i] = float64(bar.Volume)
	}

	data := &TechnicalData{Close: close}

	// MACD(12,26,9)
	data.DIF, data.DEA, data.MACD = MACD(close, 12, 26, 9)
	// KDJ(9,3,3)
	data.K, data.D, data.J = KDJ(close, high, low, 9, 3, 3)
	// RSI(6)
	data.RSI = RSI(close, 6)
	// BOLL(20,2)
	data.BollUpper, data.BollMid, data.BollLower = BOLL(close, 20, 2)
	// DMI(14,6)
	data.PDI, data.MDI, data.ADX, _ = DMI(close, high, low, 14, 6)
	// VR(26)
	data.VR = VR(close, volume, 26)
	// ROC(12,6)
	data.ROC, data.MAROC = ROC(close, 12, 6)
	// CCI(14)
	data.CCI = CCI(close, high, low, 14)
	// BRAR(26)
	data.AR, data.BR = BRAR(open, close, high, low, 26)
	// BIAS(6,12,24)
	data.BIAS1, data.BIAS2, data.BIAS3 = BIAS(close, 6, 12, 24)
	// ATR(14)
	data.ATR = ATR(close, high, low, 14)

	data.MA5 = MA(close, 5)
	data.MA20 = MA(close, 20)
	data.MA60 = MA(close, 60)
	data.MA7 = MA(close, 7)
	data.MA10 = MA(close, 10)
	data.MA13 = MA(close, 13)
	data.Volume = volume
	data.VolumeMA5 = MA(volume, 5)
	data.VolumeMA10 = MA(volume, 10)

	return data
}

// safeLastTwo 安全获取序列最后两个值
func safeLastTwo(s []float64) (prev, last float64, ok bool) {
	if len(s) < 2 {
		return 0, 0, false
	}
	prev = s[len(s)-2]
	last = s[len(s)-1]
	if math.IsNaN(prev) || math.IsNaN(last) {
		return 0, 0, false
	}
	return prev, last, true
}

// safeLast 安全获取序列最后一个值
func safeLast(s []float64) (float64, bool) {
	if len(s) == 0 {
		return 0, false
	}
	v := s[len(s)-1]
	if math.IsNaN(v) {
		return 0, false
	}
	return v, true
}

// GenerateSignals 基于技术指标生成买卖信号
func GenerateSignals(data *TechnicalData) *SignalResult {
	result := &SignalResult{}

	// ── MACD ──────────────────────────────────────────────────────────────
	// 优先判断金叉/死叉（穿越事件，最强信号）：
	//   5★：金叉 且 MACD柱同时由负转正（零轴附近双重确认）
	//   4★：金叉（DIF上穿DEA） / 死叉（DIF下穿DEA）
	//
	// 无穿越时，按 DIF/DEA 位置 + MACD柱方向细分四种状态：
	//   DIF>0 DEA>0 DIF>DEA → 多头区域且柱扩大 → 买入3★
	//   DIF>0 DEA>0 DIF<DEA → 多头区域但柱收缩 → 观望2★（多头减弱）
	//   DIF<0 DEA<0 DIF>DEA → 空头区域但柱转正 → 观望2★（空头内多头迹象）
	//   DIF<0 DEA<0 DIF<DEA → 空头区域且柱扩大 → 卖出3★
	{
		crossBuy := CROSS(data.DIF, data.DEA)
		crossSell := CROSS(data.DEA, data.DIF)
		macdPrev, macdLast, hasMacd := safeLastTwo(data.MACD)
		difLast, hasDif := safeLast(data.DIF)
		deaLast, hasDea := safeLast(data.DEA)

		switch {
		case crossBuy:
			strength := SignalStrength(4)
			reason := "DIF上穿DEA，金叉买入"
			if hasMacd && macdPrev < 0 && macdLast >= 0 {
				strength = 5
				reason = "DIF上穿DEA且MACD柱由负转正，强金叉"
			}
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "MACD", Signal: SignalBuy, Strength: strength, Reason: reason,
			})
		case crossSell:
			strength := SignalStrength(4)
			reason := "DIF下穿DEA，死叉卖出"
			if hasMacd && macdPrev > 0 && macdLast <= 0 {
				strength = 5
				reason = "DIF下穿DEA且MACD柱由正转负，强死叉"
			}
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "MACD", Signal: SignalSell, Strength: strength, Reason: reason,
			})
		case hasDif && hasDea && difLast > 0 && deaLast > 0 && difLast > deaLast:
			// 多头区域，DIF在DEA上方，MACD柱为正且扩大
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "MACD", Signal: SignalBuy, Strength: 3,
				Reason: fmt.Sprintf("DIF=%.4f>DEA=%.4f，零轴上方多头持续", difLast, deaLast),
			})
		case hasDif && hasDea && difLast > 0 && deaLast > 0 && difLast <= deaLast:
			// 多头区域，但DIF在DEA下方，MACD柱为负，多头减弱
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "MACD", Signal: SignalHold, Strength: 2,
				Reason: fmt.Sprintf("DIF=%.4f<DEA=%.4f，零轴上方但多头减弱", difLast, deaLast),
			})
		case hasDif && hasDea && difLast < 0 && deaLast < 0 && difLast > deaLast:
			// 空头区域，但DIF已在DEA上方，MACD柱转正，空头内多头迹象
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "MACD", Signal: SignalHold, Strength: 2,
				Reason: fmt.Sprintf("DIF=%.4f>DEA=%.4f，零轴下方但MACD柱转正，空头内多头迹象", difLast, deaLast),
			})
		case hasDif && hasDea && difLast < 0 && deaLast < 0 && difLast <= deaLast:
			// 空头区域，DIF在DEA下方，MACD柱为负，空头持续
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "MACD", Signal: SignalSell, Strength: 3,
				Reason: fmt.Sprintf("DIF=%.4f<DEA=%.4f，零轴下方空头持续", difLast, deaLast),
			})
		case hasMacd:
			_ = macdPrev
			_ = macdLast
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "MACD", Signal: SignalHold, Strength: 1, Reason: "MACD无明显信号",
			})
		}
	}

	// ── KDJ ───────────────────────────────────────────────────────────────
	// 5★：J<0（极度超卖）或 J>100（极度超买）
	// 4★：K<20且D<20 / K>80且D>80
	// 3★：K<30且D<30 / K>70且D>70
	// 2★：KDJ金叉/死叉
	// 1★：中性
	if k, okK := safeLast(data.K); okK {
		if d, okD := safeLast(data.D); okD {
			j, hasJ := safeLast(data.J)
			switch {
			case hasJ && j < 0:
				result.Signals = append(result.Signals, IndicatorSignal{
					Name: "KDJ", Signal: SignalBuy, Strength: 5,
					Reason: fmt.Sprintf("J=%.1f<0，极度超卖", j),
				})
			case hasJ && j > 100:
				result.Signals = append(result.Signals, IndicatorSignal{
					Name: "KDJ", Signal: SignalSell, Strength: 5,
					Reason: fmt.Sprintf("J=%.1f>100，极度超买", j),
				})
			case k < 20 && d < 20:
				result.Signals = append(result.Signals, IndicatorSignal{
					Name: "KDJ", Signal: SignalBuy, Strength: 4,
					Reason: fmt.Sprintf("K=%.1f D=%.1f，超卖区域", k, d),
				})
			case k > 80 && d > 80:
				result.Signals = append(result.Signals, IndicatorSignal{
					Name: "KDJ", Signal: SignalSell, Strength: 4,
					Reason: fmt.Sprintf("K=%.1f D=%.1f，超买区域", k, d),
				})
			case k < 30 && d < 30:
				result.Signals = append(result.Signals, IndicatorSignal{
					Name: "KDJ", Signal: SignalBuy, Strength: 3,
					Reason: fmt.Sprintf("K=%.1f D=%.1f，偏弱区域", k, d),
				})
			case k > 70 && d > 70:
				result.Signals = append(result.Signals, IndicatorSignal{
					Name: "KDJ", Signal: SignalSell, Strength: 3,
					Reason: fmt.Sprintf("K=%.1f D=%.1f，偏强区域", k, d),
				})
			case CROSS(data.K, data.D):
				result.Signals = append(result.Signals, IndicatorSignal{
					Name: "KDJ", Signal: SignalBuy, Strength: 2,
					Reason: fmt.Sprintf("K上穿D金叉，K=%.1f D=%.1f", k, d),
				})
			case CROSS(data.D, data.K):
				result.Signals = append(result.Signals, IndicatorSignal{
					Name: "KDJ", Signal: SignalSell, Strength: 2,
					Reason: fmt.Sprintf("K下穿D死叉，K=%.1f D=%.1f", k, d),
				})
			default:
				result.Signals = append(result.Signals, IndicatorSignal{
					Name: "KDJ", Signal: SignalHold, Strength: 1,
					Reason: fmt.Sprintf("K=%.1f D=%.1f，中性区域", k, d),
				})
			}
		}
	}

	// ── RSI ───────────────────────────────────────────────────────────────
	// 5★：RSI<10 或 RSI>90
	// 4★：RSI<20 或 RSI>80
	// 3★：RSI<30 或 RSI>70
	// 2★：RSI<40 或 RSI>60
	// 1★：中性
	if rsi, ok := safeLast(data.RSI); ok {
		switch {
		case rsi < 10:
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "RSI", Signal: SignalBuy, Strength: 5,
				Reason: fmt.Sprintf("RSI=%.1f<10，极度超卖", rsi),
			})
		case rsi > 90:
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "RSI", Signal: SignalSell, Strength: 5,
				Reason: fmt.Sprintf("RSI=%.1f>90，极度超买", rsi),
			})
		case rsi < 20:
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "RSI", Signal: SignalBuy, Strength: 4,
				Reason: fmt.Sprintf("RSI=%.1f<20，超卖", rsi),
			})
		case rsi > 80:
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "RSI", Signal: SignalSell, Strength: 4,
				Reason: fmt.Sprintf("RSI=%.1f>80，超买", rsi),
			})
		case rsi < 30:
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "RSI", Signal: SignalBuy, Strength: 3,
				Reason: fmt.Sprintf("RSI=%.1f<30，偏弱", rsi),
			})
		case rsi > 70:
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "RSI", Signal: SignalSell, Strength: 3,
				Reason: fmt.Sprintf("RSI=%.1f>70，偏强", rsi),
			})
		case rsi < 40:
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "RSI", Signal: SignalBuy, Strength: 2,
				Reason: fmt.Sprintf("RSI=%.1f，略偏弱", rsi),
			})
		case rsi > 60:
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "RSI", Signal: SignalSell, Strength: 2,
				Reason: fmt.Sprintf("RSI=%.1f，略偏强", rsi),
			})
		default:
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "RSI", Signal: SignalHold, Strength: 1,
				Reason: fmt.Sprintf("RSI=%.1f，中性区间", rsi),
			})
		}
	}

	// ── BOLL ──────────────────────────────────────────────────────────────
	// 5★：突破幅度>带宽50%
	// 4★：收盘<下轨 或 收盘>上轨
	// 3★：收盘在下轨~中轨之间（偏空）或 中轨~上轨之间（偏多）
	// 1★：收盘在中轨附近
	if upper, okU := safeLast(data.BollUpper); okU {
		if lower, okL := safeLast(data.BollLower); okL {
			if mid, okM := safeLast(data.BollMid); okM {
				closePrice, hasClose := safeLast(data.Close)
				if hasClose {
					bandwidth := upper - lower
					switch {
					case bandwidth > 0 && closePrice < lower-(bandwidth*0.5):
						result.Signals = append(result.Signals, IndicatorSignal{
							Name: "BOLL", Signal: SignalBuy, Strength: 5,
							Reason: fmt.Sprintf("收盘%.2f大幅低于下轨%.2f，极度超卖", closePrice, lower),
						})
					case bandwidth > 0 && closePrice > upper+(bandwidth*0.5):
						result.Signals = append(result.Signals, IndicatorSignal{
							Name: "BOLL", Signal: SignalSell, Strength: 5,
							Reason: fmt.Sprintf("收盘%.2f大幅高于上轨%.2f，极度超买", closePrice, upper),
						})
					case closePrice < lower:
						result.Signals = append(result.Signals, IndicatorSignal{
							Name: "BOLL", Signal: SignalBuy, Strength: 4,
							Reason: fmt.Sprintf("收盘%.2f<下轨%.2f，超卖", closePrice, lower),
						})
					case closePrice > upper:
						result.Signals = append(result.Signals, IndicatorSignal{
							Name: "BOLL", Signal: SignalSell, Strength: 4,
							Reason: fmt.Sprintf("收盘%.2f>上轨%.2f，超买", closePrice, upper),
						})
					case closePrice < mid:
						result.Signals = append(result.Signals, IndicatorSignal{
							Name: "BOLL", Signal: SignalSell, Strength: 2,
							Reason: fmt.Sprintf("收盘%.2f在中轨%.2f下方", closePrice, mid),
						})
					case closePrice > mid:
						result.Signals = append(result.Signals, IndicatorSignal{
							Name: "BOLL", Signal: SignalBuy, Strength: 2,
							Reason: fmt.Sprintf("收盘%.2f在中轨%.2f上方", closePrice, mid),
						})
					default:
						result.Signals = append(result.Signals, IndicatorSignal{
							Name: "BOLL", Signal: SignalHold, Strength: 1,
							Reason: fmt.Sprintf("收盘%.2f在布林带中轨附近", closePrice),
						})
					}
				}
			}
		}
	}

	// ── DMI ───────────────────────────────────────────────────────────────
	// 5★：金叉/死叉 且 ADX>40（强趋势确认）
	// 4★：金叉/死叉 且 ADX>25
	// 3★：PDI/MDI 差距明显（>10）
	// 1★：无明显信号
	{
		crossBuy := CROSS(data.PDI, data.MDI)
		crossSell := CROSS(data.MDI, data.PDI)
		adx, hasAdx := safeLast(data.ADX)
		pdi, hasPdi := safeLast(data.PDI)
		mdi, hasMdi := safeLast(data.MDI)

		if crossBuy {
			strength := SignalStrength(3)
			reason := "PDI上穿MDI，多头趋势"
			if hasAdx && adx > 40 {
				strength = 5
				reason = fmt.Sprintf("PDI上穿MDI，ADX=%.1f强趋势确认", adx)
			} else if hasAdx && adx > 25 {
				strength = 4
				reason = fmt.Sprintf("PDI上穿MDI，ADX=%.1f趋势确认", adx)
			}
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "DMI", Signal: SignalBuy, Strength: strength, Reason: reason,
			})
		} else if crossSell {
			strength := SignalStrength(3)
			reason := "MDI上穿PDI，空头趋势"
			if hasAdx && adx > 40 {
				strength = 5
				reason = fmt.Sprintf("MDI上穿PDI，ADX=%.1f强趋势确认", adx)
			} else if hasAdx && adx > 25 {
				strength = 4
				reason = fmt.Sprintf("MDI上穿PDI，ADX=%.1f趋势确认", adx)
			}
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "DMI", Signal: SignalSell, Strength: strength, Reason: reason,
			})
		} else if hasPdi && hasMdi {
			diff := pdi - mdi
			switch {
			case diff > 10:
				result.Signals = append(result.Signals, IndicatorSignal{
					Name: "DMI", Signal: SignalBuy, Strength: 2,
					Reason: fmt.Sprintf("PDI=%.1f>MDI=%.1f，多头占优", pdi, mdi),
				})
			case diff < -10:
				result.Signals = append(result.Signals, IndicatorSignal{
					Name: "DMI", Signal: SignalSell, Strength: 2,
					Reason: fmt.Sprintf("MDI=%.1f>PDI=%.1f，空头占优", mdi, pdi),
				})
			default:
				result.Signals = append(result.Signals, IndicatorSignal{
					Name: "DMI", Signal: SignalHold, Strength: 1,
					Reason: "DMI多空平衡，无明显趋势",
				})
			}
		}
	}

	// ── CCI ───────────────────────────────────────────────────────────────
	// 5★：CCI<-200 或 CCI>200
	// 4★：CCI<-100 或 CCI>100
	// 3★：CCI穿越±100（金叉/死叉）
	// 2★：CCI<-50 或 CCI>50
	// 1★：中性
	if cci, ok := safeLast(data.CCI); ok {
		cciPrev, _, hasPrev := safeLastTwo(data.CCI)
		switch {
		case cci < -200:
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "CCI", Signal: SignalBuy, Strength: 5,
				Reason: fmt.Sprintf("CCI=%.1f<-200，极度超卖", cci),
			})
		case cci > 200:
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "CCI", Signal: SignalSell, Strength: 5,
				Reason: fmt.Sprintf("CCI=%.1f>200，极度超买", cci),
			})
		case cci < -100:
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "CCI", Signal: SignalBuy, Strength: 4,
				Reason: fmt.Sprintf("CCI=%.1f<-100，超卖", cci),
			})
		case cci > 100:
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "CCI", Signal: SignalSell, Strength: 4,
				Reason: fmt.Sprintf("CCI=%.1f>100，超买", cci),
			})
		case hasPrev && cciPrev < -100 && cci >= -100:
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "CCI", Signal: SignalBuy, Strength: 3,
				Reason: fmt.Sprintf("CCI=%.1f从超卖区反弹穿越-100", cci),
			})
		case hasPrev && cciPrev > 100 && cci <= 100:
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "CCI", Signal: SignalSell, Strength: 3,
				Reason: fmt.Sprintf("CCI=%.1f从超买区回落穿越+100", cci),
			})
		case cci < -50:
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "CCI", Signal: SignalBuy, Strength: 2,
				Reason: fmt.Sprintf("CCI=%.1f，偏弱", cci),
			})
		case cci > 50:
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "CCI", Signal: SignalSell, Strength: 2,
				Reason: fmt.Sprintf("CCI=%.1f，偏强", cci),
			})
		default:
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "CCI", Signal: SignalHold, Strength: 1,
				Reason: fmt.Sprintf("CCI=%.1f，中性区间", cci),
			})
		}
	}

	// ── BIAS ──────────────────────────────────────────────────────────────
	// 使用 BIAS1（6日）作为主信号，BIAS2（12日）辅助确认
	// 5★：|BIAS1|>8%
	// 4★：|BIAS1|>5%
	// 3★：|BIAS1|>3%
	// 2★：|BIAS1|>1.5%
	// 1★：中性
	if b1, ok := safeLast(data.BIAS1); ok {
		switch {
		case b1 < -8:
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "BIAS", Signal: SignalBuy, Strength: 5,
				Reason: fmt.Sprintf("BIAS6=%.1f%%<-8%%，严重超卖", b1),
			})
		case b1 > 8:
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "BIAS", Signal: SignalSell, Strength: 5,
				Reason: fmt.Sprintf("BIAS6=%.1f%%>8%%，严重超买", b1),
			})
		case b1 < -5:
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "BIAS", Signal: SignalBuy, Strength: 4,
				Reason: fmt.Sprintf("BIAS6=%.1f%%<-5%%，超卖", b1),
			})
		case b1 > 5:
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "BIAS", Signal: SignalSell, Strength: 4,
				Reason: fmt.Sprintf("BIAS6=%.1f%%>5%%，超买", b1),
			})
		case b1 < -3:
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "BIAS", Signal: SignalBuy, Strength: 3,
				Reason: fmt.Sprintf("BIAS6=%.1f%%<-3%%，偏离均线", b1),
			})
		case b1 > 3:
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "BIAS", Signal: SignalSell, Strength: 3,
				Reason: fmt.Sprintf("BIAS6=%.1f%%>3%%，偏离均线", b1),
			})
		case b1 < -1.5:
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "BIAS", Signal: SignalBuy, Strength: 2,
				Reason: fmt.Sprintf("BIAS6=%.1f%%，轻微偏低", b1),
			})
		case b1 > 1.5:
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "BIAS", Signal: SignalSell, Strength: 2,
				Reason: fmt.Sprintf("BIAS6=%.1f%%，轻微偏高", b1),
			})
		default:
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "BIAS", Signal: SignalHold, Strength: 1,
				Reason: fmt.Sprintf("BIAS6=%.1f%%，贴近均线", b1),
			})
		}
	}

	// ── BRAR ──────────────────────────────────────────────────────────────
	// AR：多空力量对比；BR：市场意愿
	// 5★：AR<50 或 AR>200（极端）
	// 4★：AR<80 且 BR<70（双重超卖）或 AR>150 且 BR>200（双重超买）
	// 3★：AR<80 或 BR<70 / AR>150 或 BR>200
	// 2★：轻微偏离
	// 1★：中性
	if ar, okAr := safeLast(data.AR); okAr {
		br, hasBr := safeLast(data.BR)
		switch {
		case ar < 50:
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "BRAR", Signal: SignalBuy, Strength: 5,
				Reason: fmt.Sprintf("AR=%.1f<50，多空力量极度悲观", ar),
			})
		case ar > 200:
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "BRAR", Signal: SignalSell, Strength: 5,
				Reason: fmt.Sprintf("AR=%.1f>200，多空力量极度乐观", ar),
			})
		case hasBr && ar < 80 && br < 70:
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "BRAR", Signal: SignalBuy, Strength: 4,
				Reason: fmt.Sprintf("AR=%.1f BR=%.1f，双重超卖", ar, br),
			})
		case hasBr && ar > 150 && br > 200:
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "BRAR", Signal: SignalSell, Strength: 4,
				Reason: fmt.Sprintf("AR=%.1f BR=%.1f，双重超买", ar, br),
			})
		case ar < 80:
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "BRAR", Signal: SignalBuy, Strength: 3,
				Reason: fmt.Sprintf("AR=%.1f<80，市场人气偏低", ar),
			})
		case ar > 150:
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "BRAR", Signal: SignalSell, Strength: 3,
				Reason: fmt.Sprintf("AR=%.1f>150，市场人气偏高", ar),
			})
		case hasBr && br < 70:
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "BRAR", Signal: SignalBuy, Strength: 2,
				Reason: fmt.Sprintf("BR=%.1f<70，市场意愿偏弱", br),
			})
		case hasBr && br > 200:
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "BRAR", Signal: SignalSell, Strength: 2,
				Reason: fmt.Sprintf("BR=%.1f>200，市场意愿过热", br),
			})
		default:
			reason := fmt.Sprintf("AR=%.1f，市场情绪中性", ar)
			if hasBr {
				reason = fmt.Sprintf("AR=%.1f BR=%.1f，市场情绪中性", ar, br)
			}
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "BRAR", Signal: SignalHold, Strength: 1, Reason: reason,
			})
		}
	}

	// ── VR ────────────────────────────────────────────────────────────────
	// 5★：VR<25 或 VR>300
	// 4★：VR<40 或 VR>200
	// 3★：VR<60 或 VR>160
	// 1★：正常范围
	if vr, ok := safeLast(data.VR); ok {
		switch {
		case vr < 25:
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "VR", Signal: SignalBuy, Strength: 5,
				Reason: fmt.Sprintf("VR=%.1f<25，量能极度萎缩", vr),
			})
		case vr > 300:
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "VR", Signal: SignalSell, Strength: 5,
				Reason: fmt.Sprintf("VR=%.1f>300，量能极度放大", vr),
			})
		case vr < 40:
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "VR", Signal: SignalBuy, Strength: 4,
				Reason: fmt.Sprintf("VR=%.1f<40，量能萎缩超卖", vr),
			})
		case vr > 200:
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "VR", Signal: SignalSell, Strength: 4,
				Reason: fmt.Sprintf("VR=%.1f>200，量能过大超买", vr),
			})
		case vr < 60:
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "VR", Signal: SignalBuy, Strength: 3,
				Reason: fmt.Sprintf("VR=%.1f<60，量能偏弱", vr),
			})
		case vr > 160:
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "VR", Signal: SignalSell, Strength: 3,
				Reason: fmt.Sprintf("VR=%.1f>160，量能偏大", vr),
			})
		default:
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "VR", Signal: SignalHold, Strength: 1,
				Reason: fmt.Sprintf("VR=%.1f，量能正常", vr),
			})
		}
	}

	// ── ROC ───────────────────────────────────────────────────────────────
	// 金叉/死叉为主，辅以幅度
	if CROSS(data.ROC, data.MAROC) {
		result.Signals = append(result.Signals, IndicatorSignal{
			Name: "ROC", Signal: SignalBuy, Strength: 3,
			Reason: "ROC上穿MAROC，动量转强",
		})
	} else if CROSS(data.MAROC, data.ROC) {
		result.Signals = append(result.Signals, IndicatorSignal{
			Name: "ROC", Signal: SignalSell, Strength: 3,
			Reason: "ROC下穿MAROC，动量转弱",
		})
	} else if roc, ok := safeLast(data.ROC); ok {
		if roc > 0 {
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "ROC", Signal: SignalBuy, Strength: 1,
				Reason: fmt.Sprintf("ROC=%.2f%%>0，动量正向", roc),
			})
		} else {
			result.Signals = append(result.Signals, IndicatorSignal{
				Name: "ROC", Signal: SignalSell, Strength: 1,
				Reason: fmt.Sprintf("ROC=%.2f%%<0，动量负向", roc),
			})
		}
	}

	// ── ATR 参考信息（不参与信号聚合）─────────────────────────────────────
	if atr, ok := safeLast(data.ATR); ok {
		result.ATRValue = atr
		if closePrice, okC := safeLast(data.Close); okC && closePrice > 0 {
			result.ATRPercent = atr / closePrice * 100
		}
	}

	// ── 加权聚合（方案B：强度融入买卖值）────────────────────────────────
	//
	// 固定权重（0-5 范围，按指标独立性和可靠性分配）：
	//   MACD=4.5  趋势族群唯一核心，综合性最强
	//   KDJ =3.5  超买超卖族群最敏感，J值极端信号质量高
	//   BOLL=3.5  价格偏离族群核心，突破上下轨信号直观
	//   RSI =3.0  超买超卖族群，比KDJ稳定但与KDJ相关
	//   DMI =3.0  趋势方向独占，ADX确认强度，独立性高
	//   CCI =2.5  超买超卖族群第三席，对趋势行情互补
	//   BIAS=2.0  价格偏离辅助，与BOLL重叠，降权
	//   BRAR=2.0  情绪独占，但信号周期慢，适度降权
	//   VR  =1.5  量价独占，辅助指标
	//   ROC =1.0  动量辅助，与MACD高度重叠
	//   总权重 = 26.5
	//
	// 强度买卖值（signal_value）：
	//   买入：1★=+0.2, 2★=+0.4, 3★=+0.6, 4★=+0.8, 5★=+1.0
	//   卖出：1★=-0.2, 2★=-0.4, 3★=-0.6, 4★=-0.8, 5★=-1.0
	//   观望：0
	//
	// 综合得分 = Σ(weight_i × signal_value_i) / 26.5 × 100
	// 范围：-100（全部最强卖出）~ +100（全部最强买入）

	weights := map[string]float64{
		"MACD": 4.5,
		"KDJ":  3.5,
		"BOLL": 3.5,
		"RSI":  3.0,
		"DMI":  3.0,
		"CCI":  2.5,
		"BIAS": 2.0,
		"BRAR": 2.0,
		"VR":   1.5,
		"ROC":  1.0,
	}
	const totalWeight = 26.5 // 所有权重之和，用于归一化

	rawScore := 0.0
	activeWeight := 0.0 // 只累加有方向（买入/卖出）信号的权重
	for _, sig := range result.Signals {
		w, ok := weights[sig.Name]
		if !ok {
			continue
		}
		// 强度 → 买卖值：1★=0.2, 2★=0.4, ..., 5★=1.0
		signalValue := float64(sig.Strength) * 0.2
		switch sig.Signal {
		case SignalBuy:
			rawScore += w * signalValue
			activeWeight += w
		case SignalSell:
			rawScore -= w * signalValue
			activeWeight += w
			// 观望：不贡献 rawScore，也不计入 activeWeight
		}
	}
	result.ActiveWeight = activeWeight

	// 分母用有方向信号权重之和，避免观望信号稀释得分
	// 若无任何有方向信号（全部观望），得分为 0
	if activeWeight > 0 {
		result.Score = rawScore / activeWeight * 100
	}

	// 综合方向判断（±10 为观望死区）
	switch {
	case result.Score > 10:
		result.Overall = SignalBuy
	case result.Score < -10:
		result.Overall = SignalSell
	default:
		result.Overall = SignalHold
	}

	buyCount, sellCount := 0, 0
	for _, sig := range result.Signals {
		if sig.Signal == SignalBuy {
			buyCount++
		} else if sig.Signal == SignalSell {
			sellCount++
		}
	}
	holdCount := len(result.Signals) - buyCount - sellCount
	result.Summary = fmt.Sprintf("买入%d 卖出%d 观望%d",
		buyCount, sellCount, holdCount)

	return result
}

// truncateTechnicalData 将 TechnicalData 中所有序列截断到 [:idx+1]
// 用于回测逐日重放时模拟"只看到第 idx 天为止"的数据
func truncateTechnicalData(data *TechnicalData, idx int) *TechnicalData {
	trunc := func(s []float64) []float64 {
		if len(s) == 0 {
			return s
		}
		end := idx + 1
		if end > len(s) {
			end = len(s)
		}
		return s[:end]
	}
	return &TechnicalData{
		DIF:        trunc(data.DIF),
		DEA:        trunc(data.DEA),
		MACD:       trunc(data.MACD),
		K:          trunc(data.K),
		D:          trunc(data.D),
		J:          trunc(data.J),
		RSI:        trunc(data.RSI),
		BollUpper:  trunc(data.BollUpper),
		BollMid:    trunc(data.BollMid),
		BollLower:  trunc(data.BollLower),
		PDI:        trunc(data.PDI),
		MDI:        trunc(data.MDI),
		ADX:        trunc(data.ADX),
		VR:         trunc(data.VR),
		ROC:        trunc(data.ROC),
		MAROC:      trunc(data.MAROC),
		CCI:        trunc(data.CCI),
		AR:         trunc(data.AR),
		BR:         trunc(data.BR),
		BIAS1:      trunc(data.BIAS1),
		BIAS2:      trunc(data.BIAS2),
		BIAS3:      trunc(data.BIAS3),
		ATR:        trunc(data.ATR),
		Close:      trunc(data.Close),
		MA5:        trunc(data.MA5),
		MA20:       trunc(data.MA20),
		MA60:       trunc(data.MA60),
		MA7:        trunc(data.MA7),
		MA10:       trunc(data.MA10),
		MA13:       trunc(data.MA13),
		Volume:     trunc(data.Volume),
		VolumeMA5:  trunc(data.VolumeMA5),
		VolumeMA10: trunc(data.VolumeMA10),
	}
}

// GenerateSignalsAt 对第 idx 条 bar 生成技术信号（截断数据后调用 GenerateSignals）
func GenerateSignalsAt(data *TechnicalData, idx int) *SignalResult {
	truncated := truncateTechnicalData(data, idx)
	return GenerateSignals(truncated)
}
