package technical

import (
	"math"
	"time"

	"comdigger/core/kline"
)

// StrategyType 策略类型
type StrategyType = string

const (
	StrategyTechScore   StrategyType = "tech_score"   // A: 纯技术评分
	StrategyTrendFilter StrategyType = "trend_filter" // B: 趋势过滤（推荐）
	StrategyMACross     StrategyType = "ma_cross"     // C: 均线金叉死叉
	StrategyBreakout    StrategyType = "breakout"     // D: 突破策略
	StrategyTrendStop   StrategyType = "trend_stop"   // E: 趋势+移动止损

	StrategyMA520      StrategyType = "ma_5_20"       // F: 双线规则（陈小群）
	StrategyMA713      StrategyType = "ma_7_13"       // G: 短线共振（章建平）
	StrategyMA52060    StrategyType = "ma_5_20_60"    // H: 五二零组合
	StrategyMAVolMACD  StrategyType = "ma_vol_macd"   // I: 均线+量+MACD
	StrategyMAVolKDJ   StrategyType = "ma_vol_kdj"    // J: 均线+量+KDJ
	StrategyMA20MACDRS StrategyType = "ma20_macd_rsi" // K: 20线+MACD+RSI
)

// StrategyConfig 策略配置
type StrategyConfig struct {
	Type StrategyType
	Name string
	Desc string
	// 策略A参数
	BuyThreshold  float64 // 买入评分阈值（默认30）
	SellThreshold float64 // 卖出评分阈值（默认-30，传入负值）
	// 策略B/E参数
	MAFast       int     // 快线周期（默认20）
	MASlow       int     // 慢线周期（默认60）
	DrawdownStop float64 // 最大回撤止损比例（默认0.20）
	// 策略C参数
	MACrossShort int // 金叉短线（默认5）
	MACrossLong  int // 金叉长线（默认20）
	// 策略D参数
	BreakoutBuyN  int // 突破买入N日新高（默认20）
	BreakoutSellM int // 突破卖出M日新低（默认10）
	// 策略E参数
	TrendStopLoss float64 // 移动止损比例（默认0.15）
	// 策略I/J参数
	VolumeRatio float64 // 放量倍数阈值（策略I默认1.0，策略J默认1.5）
	// Sizer 仓位管理
	SizerType  string  // 仓位类型："fixed"=固定手数/"percent"=资金百分比/"allin"=全仓（默认）
	SizerParam float64 // 仓位参数：fixed时为手数，percent时为百分比0-100，allin时忽略
	// A股交易规则配置
	EnableT1      bool    // 是否启用T+1规则（默认true）
	LimitPct      float64 // 涨跌停幅度%（默认9.5；科创板/创业板用19.5）
	SlippagePct   float64 // 滑点%（买入价×(1+SlippagePct/100)，卖出价×(1-SlippagePct/100)，默认0.1）
	CommissionPct float64 // 佣金%，买卖双向（默认0.03）
	StampDutyPct  float64 // 印花税%，仅卖出时收（默认0.1）
}

// DefaultStrategies 返回5种预设策略配置
func DefaultStrategies() []StrategyConfig {
	return []StrategyConfig{
		{Type: StrategyTechScore, Name: "A.技术评分", Desc: "综合技术指标评分择时，震荡市有效",
			BuyThreshold: 30, SellThreshold: -30,
			EnableT1: true, LimitPct: 9.5, SlippagePct: 0.1, CommissionPct: 0.03, StampDutyPct: 0.1},
		{Type: StrategyTrendFilter, Name: "B.趋势过滤", Desc: "MA20>MA60趋势向上才买入，趋势破坏或回撤20%卖出",
			MAFast: 20, MASlow: 60, BuyThreshold: 10, DrawdownStop: 0.20,
			EnableT1: true, LimitPct: 9.5, SlippagePct: 0.1, CommissionPct: 0.03, StampDutyPct: 0.1},
		{Type: StrategyMACross, Name: "C.均线金叉", Desc: "MA5上穿MA20金叉买入，死叉卖出",
			MACrossShort: 5, MACrossLong: 20,
			EnableT1: true, LimitPct: 9.5, SlippagePct: 0.1, CommissionPct: 0.03, StampDutyPct: 0.1},
		{Type: StrategyBreakout, Name: "D.突破策略", Desc: "突破20日新高买入，跌破10日新低卖出",
			BreakoutBuyN: 20, BreakoutSellM: 10,
			EnableT1: true, LimitPct: 9.5, SlippagePct: 0.1, CommissionPct: 0.03, StampDutyPct: 0.1},
		{Type: StrategyTrendStop, Name: "E.趋势止损", Desc: "趋势向上且评分>0买入，从最高点回撤15%止损",
			MAFast: 20, MASlow: 60, TrendStopLoss: 0.15,
			EnableT1: true, LimitPct: 9.5, SlippagePct: 0.1, CommissionPct: 0.03, StampDutyPct: 0.1},
		{Type: StrategyMA520, Name: "F.双线规则", Desc: "20日向上，5日回踩不破或金叉买；死叉或放量跌破20日卖（陈小群）",
			EnableT1: true, LimitPct: 9.5, SlippagePct: 0.1, CommissionPct: 0.03, StampDutyPct: 0.1},
		{Type: StrategyMA713, Name: "G.短线共振", Desc: "7日金叉13日且双线向上买；死叉卖（章建平）",
			EnableT1: true, LimitPct: 9.5, SlippagePct: 0.1, CommissionPct: 0.03, StampDutyPct: 0.1},
		{Type: StrategyMA52060, Name: "H.五二零", Desc: "5>20>60多头排列且均线向上买；排列破坏卖",
			EnableT1: true, LimitPct: 9.5, SlippagePct: 0.1, CommissionPct: 0.03, StampDutyPct: 0.1},
		{Type: StrategyMAVolMACD, Name: "I.量线MACD", Desc: "四线多头+放量+MACD零轴上金叉买；排列破坏/死叉卖", VolumeRatio: 1.0,
			EnableT1: true, LimitPct: 9.5, SlippagePct: 0.1, CommissionPct: 0.03, StampDutyPct: 0.1},
		{Type: StrategyMAVolKDJ, Name: "J.量线KDJ", Desc: "三线多头+爆量+KDJ低位金叉买；排列破坏/KDJ死叉卖", VolumeRatio: 1.5,
			EnableT1: true, LimitPct: 9.5, SlippagePct: 0.1, CommissionPct: 0.03, StampDutyPct: 0.1},
		{Type: StrategyMA20MACDRS, Name: "K.稳健波段", Desc: "股价在20日线上+MACD零轴上+RSI 30~80买；跌破20日/MACD水下/RSI>80卖",
			EnableT1: true, LimitPct: 9.5, SlippagePct: 0.1, CommissionPct: 0.03, StampDutyPct: 0.1},
	}
}

// Trade 单笔交易记录
type Trade struct {
	BuyDate   time.Time
	SellDate  time.Time
	BuyPrice  float64
	SellPrice float64
	Shares    float64
	PnL       float64
	PnLPct    float64
	HoldDays  int
}

// DailyRecord 逐日回测记录
type DailyRecord struct {
	Date           time.Time
	Close          float64
	Score          float64
	Signal         string
	Position       bool
	PortfolioValue float64
	BenchmarkValue float64
}

// BacktestResult 回测结果汇总
type BacktestResult struct {
	StrategyName  string        `json:"strategy_name"`
	StrategyDesc  string        `json:"strategy_desc"`
	Trades        []Trade       `json:"trades"`
	DailyRecords  []DailyRecord `json:"daily_records"`
	TotalReturn   float64       `json:"total_return"`
	AnnualReturn  float64       `json:"annual_return"`
	MaxDrawdown   float64       `json:"max_drawdown"`
	WinRate       float64       `json:"win_rate"`
	TotalTrades   int           `json:"total_trades"`
	BuyHoldReturn float64       `json:"buy_hold_return"`
	InitCapital   float64       `json:"init_capital"`
	FinalCapital  float64       `json:"final_capital"`
	FromDate      time.Time     `json:"from_date"`
	ToDate        time.Time     `json:"to_date"`
	// AKQuant 风格绩效指标
	SharpeRatio     float64 `json:"sharpe_ratio"`
	SortinoRatio    float64 `json:"sortino_ratio"`
	CalmarRatio     float64 `json:"calmar_ratio"`
	MaxDrawdownDays int     `json:"max_drawdown_days"`
	ProfitFactor    float64 `json:"profit_factor"`
	AvgPnLPct       float64 `json:"avg_pnl_pct"`
	Volatility      float64 `json:"volatility"`
	ExposureDays    int     `json:"exposure_days"`
	ExposurePct     float64 `json:"exposure_pct"`
	// 新增绩效指标
	VaR95            float64 `json:"var_95"`
	VaR99            float64 `json:"var_99"`
	CVaR95           float64 `json:"cvar_95"`
	CVaR99           float64 `json:"cvar_99"`
	UlcerIndex       float64 `json:"ulcer_index"`
	AvgMAE           float64 `json:"avg_mae"`
	AvgMFE           float64 `json:"avg_mfe"`
	TotalCommission  float64 `json:"total_commission"`
	TotalStampDuty   float64 `json:"total_stamp_duty"`   // 总印花税（元）
	TotalSlippage    float64 `json:"total_slippage"`     // 总滑点成本（元）
	TotalTradingCost float64 `json:"total_trading_cost"` // 总交易成本=佣金+印花税+滑点（元）
	T1Blocked        int     `json:"t1_blocked"`         // 因T+1规则被阻止的卖出次数
	LimitBlocked     int     `json:"limit_blocked"`      // 因涨跌停被阻止的买卖次数
	MaxLossStreak    int     `json:"max_loss_streak"`    // 最大连续亏损次数
	AvgHoldDays      float64 `json:"avg_hold_days"`      // 平均持仓天数
}

// safeGet 安全获取序列第i个元素，越界或NaN返回0
func safeGet(s []float64, i int) float64 {
	if i < 0 || i >= len(s) {
		return 0
	}
	v := s[i]
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}

// maxClose 返回bars切片中的最高收盘价
func maxClose(bars []kline.KlineBar) float64 {
	max := 0.0
	for _, b := range bars {
		if b.Close > max {
			max = b.Close
		}
	}
	return max
}

// minClose 返回bars切片中的最低收盘价
func minClose(bars []kline.KlineBar) float64 {
	min := math.MaxFloat64
	for _, b := range bars {
		if b.Close < min {
			min = b.Close
		}
	}
	if min == math.MaxFloat64 {
		return 0
	}
	return min
}

// shouldBuy 根据策略判断是否买入
func shouldBuy(cfg StrategyConfig, data *TechnicalData, i int, sig *SignalResult, bars []kline.KlineBar) bool {
	switch cfg.Type {
	case StrategyTechScore:
		return sig.Score > cfg.BuyThreshold
	case StrategyTrendFilter:
		ma20 := safeGet(data.MA20, i)
		ma60 := safeGet(data.MA60, i)
		return ma20 > 0 && ma60 > 0 && ma20 > ma60 && sig.Score > cfg.BuyThreshold
	case StrategyMACross:
		if i < 1 {
			return false
		}
		ma5Prev := safeGet(data.MA5, i-1)
		ma20Prev := safeGet(data.MA20, i-1)
		ma5Cur := safeGet(data.MA5, i)
		ma20Cur := safeGet(data.MA20, i)
		return ma5Prev > 0 && ma20Prev > 0 && ma5Prev < ma20Prev && ma5Cur >= ma20Cur
	case StrategyBreakout:
		if i < cfg.BreakoutBuyN {
			return false
		}
		return bars[i].Close > maxClose(bars[i-cfg.BreakoutBuyN:i])
	case StrategyTrendStop:
		ma20 := safeGet(data.MA20, i)
		ma60 := safeGet(data.MA60, i)
		return ma20 > 0 && ma60 > 0 && ma20 > ma60 && sig.Score > 0

	case StrategyMA520: // F: 双线规则（5+20，陈小群）
		if i < 1 {
			return false
		}
		ma5Cur := safeGet(data.MA5, i)
		ma5Prev := safeGet(data.MA5, i-1)
		ma20Cur := safeGet(data.MA20, i)
		ma20Prev := safeGet(data.MA20, i-1)
		if ma5Cur == 0 || ma20Cur == 0 || ma20Prev == 0 {
			return false
		}
		ma20Up := ma20Cur > ma20Prev                           // 20日向上
		goldenCross := ma5Prev < ma20Prev && ma5Cur >= ma20Cur // 5日金叉20日
		// 5日回踩20日不破：5日在20日上方，且距离<3%（回踩贴近）
		pullback := ma5Cur > ma20Cur && ma5Cur/ma20Cur < 1.03 &&
			ma5Prev > ma20Prev && ma5Prev/ma20Prev >= 1.03
		return ma20Up && (goldenCross || pullback)

	case StrategyMA713: // G: 短线共振（7+13，章建平）
		if i < 1 {
			return false
		}
		ma7Cur := safeGet(data.MA7, i)
		ma7Prev := safeGet(data.MA7, i-1)
		ma13Cur := safeGet(data.MA13, i)
		ma13Prev := safeGet(data.MA13, i-1)
		if ma7Cur == 0 || ma13Cur == 0 || ma7Prev == 0 || ma13Prev == 0 {
			return false
		}
		goldenCross := ma7Prev < ma13Prev && ma7Cur >= ma13Cur
		ma7Up := ma7Cur > ma7Prev
		ma13Up := ma13Cur > ma13Prev
		return goldenCross && ma7Up && ma13Up

	case StrategyMA52060: // H: 五二零（5+20+60多头排列）
		if i < 1 {
			return false
		}
		ma5 := safeGet(data.MA5, i)
		ma5Prev := safeGet(data.MA5, i-1)
		ma20 := safeGet(data.MA20, i)
		ma20Prev := safeGet(data.MA20, i-1)
		ma60 := safeGet(data.MA60, i)
		ma60Prev := safeGet(data.MA60, i-1)
		if ma5 == 0 || ma20 == 0 || ma60 == 0 {
			return false
		}
		bullAlign := ma5 > ma20 && ma20 > ma60                       // 多头排列
		allUp := ma5 > ma5Prev && ma20 > ma20Prev && ma60 > ma60Prev // 三线向上
		return bullAlign && allUp

	case StrategyMAVolMACD: // I: 均线+量+MACD（四线多头+放量+MACD零轴上金叉）
		if i < 1 {
			return false
		}
		ma5 := safeGet(data.MA5, i)
		ma10 := safeGet(data.MA10, i)
		ma20 := safeGet(data.MA20, i)
		ma60 := safeGet(data.MA60, i)
		if ma5 == 0 || ma10 == 0 || ma20 == 0 || ma60 == 0 {
			return false
		}
		bullAlign := ma5 > ma10 && ma10 > ma20 && ma20 > ma60
		vol := safeGet(data.Volume, i)
		volMA := safeGet(data.VolumeMA10, i)
		volOK := volMA > 0 && vol > volMA*cfg.VolumeRatio // 放量
		dif := safeGet(data.DIF, i)
		difPrev := safeGet(data.DIF, i-1)
		dea := safeGet(data.DEA, i)
		deaPrev := safeGet(data.DEA, i-1)
		macdOK := dif > 0 && difPrev < deaPrev && dif >= dea // 零轴上方金叉
		return bullAlign && volOK && macdOK

	case StrategyMAVolKDJ: // J: 均线+量+KDJ（三线多头+爆量+KDJ低位金叉）
		if i < 1 {
			return false
		}
		ma5 := safeGet(data.MA5, i)
		ma10 := safeGet(data.MA10, i)
		ma20 := safeGet(data.MA20, i)
		if ma5 == 0 || ma10 == 0 || ma20 == 0 {
			return false
		}
		bullAlign := ma5 > ma10 && ma10 > ma20
		vol := safeGet(data.Volume, i)
		volMA := safeGet(data.VolumeMA5, i)
		volOK := volMA > 0 && vol > volMA*cfg.VolumeRatio // 爆量
		k := safeGet(data.K, i)
		kPrev := safeGet(data.K, i-1)
		d := safeGet(data.D, i)
		dPrev := safeGet(data.D, i-1)
		kdjOK := k < 50 && kPrev < dPrev && k >= d // 低位金叉
		return bullAlign && volOK && kdjOK

	case StrategyMA20MACDRS: // K: 20线+MACD+RSI（稳健波段）
		if i < 1 {
			return false
		}
		ma20 := safeGet(data.MA20, i)
		ma20Prev := safeGet(data.MA20, i-1)
		if ma20 == 0 {
			return false
		}
		priceAboveMA20 := bars[i].Close > ma20
		ma20Up := ma20 > ma20Prev
		dif := safeGet(data.DIF, i)
		dea := safeGet(data.DEA, i)
		macdOK := dif > 0 && dea > 0
		rsi := safeGet(data.RSI, i)
		rsiOK := rsi > 30 && rsi < 80
		return priceAboveMA20 && ma20Up && macdOK && rsiOK
	}
	return false
}

// shouldSell 根据策略判断是否卖出
func shouldSell(cfg StrategyConfig, data *TechnicalData, i int, sig *SignalResult, bars []kline.KlineBar, maxHighAfterBuy float64) bool {
	switch cfg.Type {
	case StrategyTechScore:
		return sig.Score < cfg.SellThreshold
	case StrategyTrendFilter:
		ma20 := safeGet(data.MA20, i)
		ma60 := safeGet(data.MA60, i)
		trendBroken := ma20 > 0 && ma60 > 0 && ma20 < ma60
		drawdownHit := maxHighAfterBuy > 0 && bars[i].Close < maxHighAfterBuy*(1-cfg.DrawdownStop)
		return trendBroken || drawdownHit
	case StrategyMACross:
		if i < 1 {
			return false
		}
		ma5Prev := safeGet(data.MA5, i-1)
		ma20Prev := safeGet(data.MA20, i-1)
		ma5Cur := safeGet(data.MA5, i)
		ma20Cur := safeGet(data.MA20, i)
		return ma5Prev > 0 && ma20Prev > 0 && ma5Prev > ma20Prev && ma5Cur <= ma20Cur
	case StrategyBreakout:
		if i < cfg.BreakoutSellM {
			return false
		}
		return bars[i].Close < minClose(bars[i-cfg.BreakoutSellM:i])
	case StrategyTrendStop:
		return maxHighAfterBuy > 0 && bars[i].Close < maxHighAfterBuy*(1-cfg.TrendStopLoss)

	case StrategyMA520: // F: 死叉 OR 放量跌破20日
		if i < 1 {
			return false
		}
		ma5Cur := safeGet(data.MA5, i)
		ma5Prev := safeGet(data.MA5, i-1)
		ma20Cur := safeGet(data.MA20, i)
		ma20Prev := safeGet(data.MA20, i-1)
		deadCross := ma5Prev > ma20Prev && ma5Cur <= ma20Cur
		vol := safeGet(data.Volume, i)
		volMA := safeGet(data.VolumeMA5, i)
		breakWithVol := bars[i].Close < ma20Cur && volMA > 0 && vol > volMA*1.5
		return deadCross || breakWithVol

	case StrategyMA713: // G: 7日死叉13日
		if i < 1 {
			return false
		}
		ma7Cur := safeGet(data.MA7, i)
		ma7Prev := safeGet(data.MA7, i-1)
		ma13Cur := safeGet(data.MA13, i)
		ma13Prev := safeGet(data.MA13, i-1)
		return ma7Prev > 0 && ma13Prev > 0 && ma7Prev > ma13Prev && ma7Cur <= ma13Cur

	case StrategyMA52060: // H: 任一层排列破坏
		ma5 := safeGet(data.MA5, i)
		ma20 := safeGet(data.MA20, i)
		ma60 := safeGet(data.MA60, i)
		return (ma5 > 0 && ma20 > 0 && ma5 < ma20) || (ma20 > 0 && ma60 > 0 && ma20 < ma60)

	case StrategyMAVolMACD: // I: 排列破坏 OR MACD死叉 OR DIF跌破零轴
		if i < 1 {
			return false
		}
		ma5 := safeGet(data.MA5, i)
		ma20 := safeGet(data.MA20, i)
		alignBroken := ma5 > 0 && ma20 > 0 && ma5 < ma20
		dif := safeGet(data.DIF, i)
		difPrev := safeGet(data.DIF, i-1)
		dea := safeGet(data.DEA, i)
		deaPrev := safeGet(data.DEA, i-1)
		deadCross := difPrev >= deaPrev && dif < dea
		difBelow0 := dif < 0
		return alignBroken || deadCross || difBelow0

	case StrategyMAVolKDJ: // J: 排列破坏 OR KDJ死叉
		if i < 1 {
			return false
		}
		ma5 := safeGet(data.MA5, i)
		ma20 := safeGet(data.MA20, i)
		alignBroken := ma5 > 0 && ma20 > 0 && ma5 < ma20
		k := safeGet(data.K, i)
		kPrev := safeGet(data.K, i-1)
		d := safeGet(data.D, i)
		dPrev := safeGet(data.D, i-1)
		deadCross := kPrev >= dPrev && k < d
		return alignBroken || deadCross

	case StrategyMA20MACDRS: // K: 跌破20日 OR MACD水下 OR RSI>80
		ma20 := safeGet(data.MA20, i)
		priceBelowMA20 := ma20 > 0 && bars[i].Close < ma20
		dif := safeGet(data.DIF, i)
		difBelow0 := dif < 0
		rsi := safeGet(data.RSI, i)
		rsiOverbought := rsi > 80
		return priceBelowMA20 || difBelow0 || rsiOverbought
	}
	return false
}

// RunBacktest 执行回测
// bars: K线数据（按时间升序）
// data: 已计算好的技术指标数据（与 bars 等长）
// from: 回测起始日期
// cfg: 策略配置
// initCapital: 初始资金
func RunBacktest(bars []kline.KlineBar, data *TechnicalData, from time.Time, cfg StrategyConfig, initCapital float64) *BacktestResult {
	result := &BacktestResult{
		InitCapital:  initCapital,
		FromDate:     from,
		StrategyName: cfg.Name,
		StrategyDesc: cfg.Desc,
	}

	n := len(bars)
	if n == 0 {
		result.FinalCapital = initCapital
		return result
	}

	// 找到第一个 >= from 的起始索引
	startIdx := n // 默认超出范围
	for i, bar := range bars {
		if !bar.Time.Before(from) {
			startIdx = i
			break
		}
	}
	// 至少需要 30 条历史数据用于指标预热
	if startIdx < 30 {
		startIdx = 30
	}
	if startIdx >= n {
		result.FinalCapital = initCapital
		result.ToDate = from
		return result
	}

	// 记录起始收盘价（用于计算买入持有收益）
	startClose := bars[startIdx].Close

	// 买入持有基准：以 startIdx 收盘价买入，计算持有份额
	benchmarkShares := 0.0
	if bars[startIdx].Close > 0 {
		benchmarkShares = initCapital / bars[startIdx].Close
	}

	// 回测状态
	cash := initCapital
	shares := 0.0
	hasPosition := false
	var currentTrade Trade
	maxHighAfterBuy := 0.0 // 持仓期间最高收盘价
	var buyDate time.Time  // 记录最近买入的执行日期（用于T+1检查）
	prevClose := 0.0       // 前一日收盘价（用于涨跌停检查）

	for i := startIdx; i < n; i++ {
		bar := bars[i]
		// 维护前一日收盘价
		if i > startIdx {
			prevClose = bars[i-1].Close
		}
		sig := GenerateSignalsAt(data, i)

		// 更新持仓期间最高收盘价
		if hasPosition && bar.Close > maxHighAfterBuy {
			maxHighAfterBuy = bar.Close
		}

		// 当前组合价值 = 现金 + 持仓市值
		portfolioValue := cash
		if hasPosition {
			portfolioValue = cash + shares*bar.Close
		}

		// 信号字符串
		signalStr := string(sig.Overall)

		// 记录逐日数据
		result.DailyRecords = append(result.DailyRecords, DailyRecord{
			Date:           bar.Time,
			Close:          bar.Close,
			Score:          sig.Score,
			Signal:         signalStr,
			Position:       hasPosition,
			PortfolioValue: portfolioValue,
			BenchmarkValue: benchmarkShares * bar.Close,
		})

		// 若明天有数据，判断是否交易
		if i+1 < n {
			nextBar := bars[i+1]
			if !hasPosition && shouldBuy(cfg, data, i, sig, bars) {
				// 涨跌停检查：当日涨停（changePct >= LimitPct），买单无法成交
				changePct := 0.0
				if prevClose > 0 {
					changePct = (bar.Close - prevClose) / prevClose * 100
				}
				if cfg.LimitPct > 0 && changePct >= cfg.LimitPct {
					result.LimitBlocked++
				} else {
					// 买入实际成交价（含滑点）
					buyPrice := nextBar.Open * (1 + cfg.SlippagePct/100)
					shares = calcShares(cfg.SizerType, cfg.SizerParam, cash, buyPrice)
					if shares > 0 {
						commission := buyPrice * shares * cfg.CommissionPct / 100
						slippage := math.Abs(buyPrice-nextBar.Open) * shares
						cash = cash - shares*buyPrice - commission
						result.TotalCommission += commission
						result.TotalSlippage += slippage
						result.TotalTradingCost += commission + slippage
						hasPosition = true
						buyDate = nextBar.Time
						maxHighAfterBuy = buyPrice
						currentTrade = Trade{
							BuyDate:  nextBar.Time,
							BuyPrice: buyPrice,
							Shares:   shares,
						}
					}
				}
			} else if hasPosition && shouldSell(cfg, data, i, sig, bars, maxHighAfterBuy) {
				// T+1规则：买入当日不能卖出
				if cfg.EnableT1 && !buyDate.IsZero() && bar.Time.Equal(buyDate) {
					result.T1Blocked++
				} else {
					// 涨跌停检查：当日跌停（changePct <= -LimitPct），卖单无法成交
					changePct := 0.0
					if prevClose > 0 {
						changePct = (bar.Close - prevClose) / prevClose * 100
					}
					if cfg.LimitPct > 0 && changePct <= -cfg.LimitPct {
						result.LimitBlocked++
					} else {
						// 卖出实际成交价（含滑点）
						sellPrice := nextBar.Open * (1 - cfg.SlippagePct/100)
						pnl := (sellPrice - currentTrade.BuyPrice) * shares
						pnlPct := (sellPrice - currentTrade.BuyPrice) / currentTrade.BuyPrice * 100
						holdDays := int(nextBar.Time.Sub(currentTrade.BuyDate).Hours() / 24)
						commission := sellPrice * shares * cfg.CommissionPct / 100
						stampDuty := sellPrice * shares * cfg.StampDutyPct / 100
						slippage := math.Abs(nextBar.Open-sellPrice) * shares
						cash = cash + shares*sellPrice - commission - stampDuty
						result.TotalCommission += commission
						result.TotalStampDuty += stampDuty
						result.TotalSlippage += slippage
						result.TotalTradingCost += commission + stampDuty + slippage
						currentTrade.SellDate = nextBar.Time
						currentTrade.SellPrice = sellPrice
						currentTrade.PnL = pnl
						currentTrade.PnLPct = pnlPct
						currentTrade.HoldDays = holdDays
						result.Trades = append(result.Trades, currentTrade)
						shares = 0
						hasPosition = false
						maxHighAfterBuy = 0
						buyDate = time.Time{} // 重置买入日期
					}
				}
			}
		}
	}

	// 末尾若仍持仓：A股T+1规则，不强制平仓，以最后收盘价计算浮盈计入最终资金
	// 持仓中的未平仓交易不记入 Trades（因为尚未真实卖出）
	if hasPosition && n > 0 {
		lastBar := bars[n-1]
		cash = cash + shares*lastBar.Close // 现金 + 持仓市值（以最后收盘价估值）
	}

	// 计算最大连续亏损次数
	streak := 0
	maxStreak := 0
	for _, t := range result.Trades {
		if t.PnL < 0 {
			streak++
			if streak > maxStreak {
				maxStreak = streak
			}
		} else {
			streak = 0
		}
	}
	result.MaxLossStreak = maxStreak

	// 计算平均持仓天数
	if len(result.Trades) > 0 {
		totalDays := 0
		for _, t := range result.Trades {
			totalDays += t.HoldDays
		}
		result.AvgHoldDays = float64(totalDays) / float64(len(result.Trades))
	}

	result.FinalCapital = cash
	result.TotalTrades = len(result.Trades)

	// 总收益率
	result.TotalReturn = (result.FinalCapital - initCapital) / initCapital * 100

	// 年化收益率
	if len(bars) > 0 {
		result.ToDate = bars[n-1].Time
		days := result.ToDate.Sub(bars[startIdx].Time).Hours() / 24
		if days > 0 {
			years := days / 365.0
			result.AnnualReturn = (math.Pow(1+result.TotalReturn/100, 1/years) - 1) * 100
		}
	}

	// 最大回撤（逐日组合价值序列）
	if len(result.DailyRecords) > 0 {
		peak := result.DailyRecords[0].PortfolioValue
		maxDD := 0.0
		for _, rec := range result.DailyRecords {
			if rec.PortfolioValue > peak {
				peak = rec.PortfolioValue
			}
			if peak > 0 {
				dd := (peak - rec.PortfolioValue) / peak * 100
				if dd > maxDD {
					maxDD = dd
				}
			}
		}
		result.MaxDrawdown = maxDD
	}

	// 胜率
	if result.TotalTrades > 0 {
		wins := 0
		for _, t := range result.Trades {
			if t.PnL > 0 {
				wins++
			}
		}
		result.WinRate = float64(wins) / float64(result.TotalTrades) * 100
	}

	// 计算 AKQuant 风格绩效指标
	dailyReturns := calcDailyReturns(result.DailyRecords)
	result.SharpeRatio = calcSharpeRatio(dailyReturns, 0.02)
	result.SortinoRatio = calcSortinoRatio(dailyReturns, 0.02)
	result.Volatility = calcVolatility(dailyReturns)
	result.MaxDrawdownDays = calcMaxDrawdownDays(result.DailyRecords)
	result.ProfitFactor = calcProfitFactor(result.Trades)
	if result.MaxDrawdown > 0 {
		result.CalmarRatio = result.AnnualReturn / result.MaxDrawdown
	}
	// 平均每笔盈亏%
	if result.TotalTrades > 0 {
		totalPnLPct := 0.0
		for _, t := range result.Trades {
			totalPnLPct += t.PnLPct
		}
		result.AvgPnLPct = totalPnLPct / float64(result.TotalTrades)
	}
	// 持仓天数和占比
	for _, rec := range result.DailyRecords {
		if rec.Position {
			result.ExposureDays++
		}
	}
	if len(result.DailyRecords) > 0 {
		result.ExposurePct = float64(result.ExposureDays) / float64(len(result.DailyRecords)) * 100
	}

	// 买入持有收益率
	lastClose := bars[n-1].Close
	if startClose > 0 {
		result.BuyHoldReturn = (lastClose - startClose) / startClose * 100
	}

	// 新增绩效指标计算
	result.VaR95 = calcVaR(dailyReturns, 0.95)
	result.VaR99 = calcVaR(dailyReturns, 0.99)
	result.CVaR95 = calcCVaR(dailyReturns, 0.95)
	result.CVaR99 = calcCVaR(dailyReturns, 0.99)
	result.UlcerIndex = calcUlcerIndex(result.DailyRecords)
	result.AvgMAE = calcMAE(bars, result.Trades)
	result.AvgMFE = calcMFE(bars, result.Trades)

	return result
}

// RunAllBacktests 对所有预设策略执行回测
func RunAllBacktests(bars []kline.KlineBar, data *TechnicalData, from time.Time, initCapital float64, sizerType string, sizerParam float64, enableT1 bool, limitPct, slippagePct, commissionPct float64) []*BacktestResult {
	strategies := DefaultStrategies()
	results := make([]*BacktestResult, len(strategies))
	for i, cfg := range strategies {
		cfg.SizerType = sizerType
		cfg.SizerParam = sizerParam
		cfg.EnableT1 = enableT1
		cfg.LimitPct = limitPct
		cfg.SlippagePct = slippagePct
		cfg.CommissionPct = commissionPct
		results[i] = RunBacktest(bars, data, from, cfg, initCapital)
	}
	return results
}

// calcDailyReturns 从逐日 PortfolioValue 序列计算每日收益率
func calcDailyReturns(records []DailyRecord) []float64 {
	if len(records) < 2 {
		return nil
	}
	returns := make([]float64, 0, len(records)-1)
	for i := 1; i < len(records); i++ {
		prev := records[i-1].PortfolioValue
		curr := records[i].PortfolioValue
		if prev > 0 {
			returns = append(returns, (curr-prev)/prev)
		}
	}
	return returns
}

// calcSharpeRatio 年化夏普比率（无风险利率 riskFreeRate 为年化，如 0.02）
func calcSharpeRatio(dailyReturns []float64, riskFreeRate float64) float64 {
	if len(dailyReturns) < 2 {
		return 0
	}
	dailyRF := riskFreeRate / 252.0
	sum := 0.0
	for _, r := range dailyReturns {
		sum += r
	}
	mean := sum / float64(len(dailyReturns))
	excessMean := mean - dailyRF

	variance := 0.0
	for _, r := range dailyReturns {
		diff := r - mean
		variance += diff * diff
	}
	variance /= float64(len(dailyReturns) - 1)
	stdDev := math.Sqrt(variance)
	if stdDev == 0 {
		return 0
	}
	return excessMean / stdDev * math.Sqrt(252)
}

// calcSortinoRatio 索提诺比率（只计下行波动）
func calcSortinoRatio(dailyReturns []float64, riskFreeRate float64) float64 {
	if len(dailyReturns) < 2 {
		return 0
	}
	dailyRF := riskFreeRate / 252.0
	sum := 0.0
	for _, r := range dailyReturns {
		sum += r
	}
	mean := sum / float64(len(dailyReturns))
	excessMean := mean - dailyRF

	downVariance := 0.0
	downCount := 0
	for _, r := range dailyReturns {
		if r < dailyRF {
			diff := r - dailyRF
			downVariance += diff * diff
			downCount++
		}
	}
	if downCount == 0 {
		return 0
	}
	downStdDev := math.Sqrt(downVariance / float64(downCount))
	if downStdDev == 0 {
		return 0
	}
	return excessMean / downStdDev * math.Sqrt(252)
}

// calcMaxDrawdownDays 统计最大回撤期间的持续天数
func calcMaxDrawdownDays(records []DailyRecord) int {
	if len(records) == 0 {
		return 0
	}
	peak := records[0].PortfolioValue
	peakIdx := 0
	maxDays := 0

	for i, rec := range records {
		if rec.PortfolioValue > peak {
			peak = rec.PortfolioValue
			peakIdx = i
		}
		days := i - peakIdx
		if days > maxDays {
			maxDays = days
		}
	}
	return maxDays
}

// calcVolatility 年化波动率（%）
func calcVolatility(dailyReturns []float64) float64 {
	if len(dailyReturns) < 2 {
		return 0
	}
	sum := 0.0
	for _, r := range dailyReturns {
		sum += r
	}
	mean := sum / float64(len(dailyReturns))
	variance := 0.0
	for _, r := range dailyReturns {
		diff := r - mean
		variance += diff * diff
	}
	variance /= float64(len(dailyReturns) - 1)
	return math.Sqrt(variance) * math.Sqrt(252) * 100
}

// calcProfitFactor 盈亏比 = 总盈利/|总亏损|，无亏损时返回0
func calcProfitFactor(trades []Trade) float64 {
	totalProfit := 0.0
	totalLoss := 0.0
	for _, t := range trades {
		if t.PnL > 0 {
			totalProfit += t.PnL
		} else if t.PnL < 0 {
			totalLoss += -t.PnL
		}
	}
	if totalLoss == 0 {
		return 0
	}
	return totalProfit / totalLoss
}

// calcVaR 计算VaR（风险价值），confidence=0.95或0.99
// 对日收益率升序排序，取第(1-confidence)*N个位置的绝对值（%）
func calcVaR(dailyReturns []float64, confidence float64) float64 {
	if len(dailyReturns) < 10 {
		return 0
	}
	sorted := make([]float64, len(dailyReturns))
	copy(sorted, dailyReturns)
	// 升序排序
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j] < sorted[j-1]; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	idx := int(float64(len(sorted)) * (1 - confidence))
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return math.Abs(sorted[idx]) * 100
}

// calcCVaR 计算CVaR（条件风险价值/期望损失）
// 取低于VaR阈值的所有日收益率的均值绝对值（%）
func calcCVaR(dailyReturns []float64, confidence float64) float64 {
	if len(dailyReturns) < 10 {
		return 0
	}
	sorted := make([]float64, len(dailyReturns))
	copy(sorted, dailyReturns)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j] < sorted[j-1]; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	cutIdx := int(float64(len(sorted)) * (1 - confidence))
	if cutIdx == 0 {
		return 0
	}
	sum := 0.0
	for i := 0; i < cutIdx; i++ {
		sum += sorted[i]
	}
	return math.Abs(sum/float64(cutIdx)) * 100
}

// calcUlcerIndex 计算溃疡指数（回撤深度的RMS，越小越好）
func calcUlcerIndex(records []DailyRecord) float64 {
	if len(records) < 2 {
		return 0
	}
	peak := records[0].PortfolioValue
	sumSq := 0.0
	for _, rec := range records {
		if rec.PortfolioValue > peak {
			peak = rec.PortfolioValue
		}
		if peak > 0 {
			dd := (peak - rec.PortfolioValue) / peak * 100
			sumSq += dd * dd
		}
	}
	return math.Sqrt(sumSq / float64(len(records)))
}

// calcMAE 每笔交易平均最大不利变动%（持仓期间最大浮亏）
func calcMAE(bars []kline.KlineBar, trades []Trade) float64 {
	if len(trades) == 0 {
		return 0
	}
	// 建立日期→索引映射
	dateIdx := make(map[string]int, len(bars))
	for i, b := range bars {
		dateIdx[b.Time.Format("2006-01-02")] = i
	}
	total := 0.0
	count := 0
	for _, t := range trades {
		buyKey := t.BuyDate.Format("2006-01-02")
		sellKey := t.SellDate.Format("2006-01-02")
		startI, okS := dateIdx[buyKey]
		endI, okE := dateIdx[sellKey]
		if !okS || !okE || endI <= startI {
			continue
		}
		minPrice := t.BuyPrice
		for i := startI; i <= endI; i++ {
			if bars[i].Close < minPrice {
				minPrice = bars[i].Close
			}
		}
		mae := (minPrice - t.BuyPrice) / t.BuyPrice * 100
		total += mae
		count++
	}
	if count == 0 {
		return 0
	}
	return total / float64(count)
}

// calcMFE 每笔交易平均最大有利变动%（持仓期间最大浮盈）
func calcMFE(bars []kline.KlineBar, trades []Trade) float64 {
	if len(trades) == 0 {
		return 0
	}
	dateIdx := make(map[string]int, len(bars))
	for i, b := range bars {
		dateIdx[b.Time.Format("2006-01-02")] = i
	}
	total := 0.0
	count := 0
	for _, t := range trades {
		buyKey := t.BuyDate.Format("2006-01-02")
		sellKey := t.SellDate.Format("2006-01-02")
		startI, okS := dateIdx[buyKey]
		endI, okE := dateIdx[sellKey]
		if !okS || !okE || endI <= startI {
			continue
		}
		maxPrice := t.BuyPrice
		for i := startI; i <= endI; i++ {
			if bars[i].Close > maxPrice {
				maxPrice = bars[i].Close
			}
		}
		mfe := (maxPrice - t.BuyPrice) / t.BuyPrice * 100
		total += mfe
		count++
	}
	if count == 0 {
		return 0
	}
	return total / float64(count)
}

// calcShares 根据仓位类型计算买入手数
func calcShares(sizerType string, sizerParam, cash, price float64) float64 {
	if price <= 0 {
		return 0
	}
	switch sizerType {
	case "fixed":
		shares := sizerParam
		if shares*price > cash {
			shares = math.Floor(cash / price)
		}
		return shares
	case "percent":
		return math.Floor(cash * sizerParam / 100.0 / price)
	default: // "allin" 或空字符串
		return math.Floor(cash / price)
	}
}
