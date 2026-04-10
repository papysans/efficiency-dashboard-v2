package technical

import (
	"math"
	"time"

	"comdigger/core/kline"
)

// PortfolioBacktestResult 多标的组合回测结果
type PortfolioBacktestResult struct {
	Stocks       []string                   // 股票代码列表
	StrategyName string                     // 策略名称
	TotalReturn  float64                    // 组合总收益率%
	AnnualReturn float64                    // 组合年化收益率%
	MaxDrawdown  float64                    // 组合最大回撤%
	SharpeRatio  float64                    // 组合夏普比率
	DailyValues  []PortfolioDailyValue      // 组合总价值时序
	StockResults map[string]*BacktestResult // 各股票单独结果
}

// PortfolioDailyValue 组合每日价值记录
type PortfolioDailyValue struct {
	Date  time.Time
	Value float64
}

// RunPortfolioBacktest 多标的等权组合回测
// stockBars: 各股票K线数据（key=公司ID）
// stockData: 各股票技术指标数据（key=公司ID）
// from: 回测起始日期
// cfg: 策略配置（对每只股票使用相同策略）
// initCapital: 总初始资金（等权分配给每只股票）
func RunPortfolioBacktest(stockBars map[string][]kline.KlineBar, stockData map[string]*TechnicalData, from time.Time, cfg StrategyConfig, initCapital float64) *PortfolioBacktestResult {
	n := len(stockBars)
	if n == 0 {
		return &PortfolioBacktestResult{}
	}

	// 等权分配资金
	perCapital := initCapital / float64(n)

	// 收集股票代码列表
	stocks := make([]string, 0, n)
	for id := range stockBars {
		stocks = append(stocks, id)
	}

	// 对每只股票独立运行回测
	stockResults := make(map[string]*BacktestResult, n)
	for _, id := range stocks {
		bars := stockBars[id]
		data := stockData[id]
		if data == nil || len(bars) == 0 {
			continue
		}
		stockResults[id] = RunBacktest(bars, data, from, cfg, perCapital)
	}

	// 合并各股票的 DailyRecords，按日期对齐，计算组合总价值
	// 收集所有日期
	dateSet := make(map[string]time.Time)
	for _, r := range stockResults {
		for _, rec := range r.DailyRecords {
			key := rec.Date.Format("2006-01-02")
			dateSet[key] = rec.Date
		}
	}

	// 按日期升序排序
	sortedDates := make([]time.Time, 0, len(dateSet))
	for _, d := range dateSet {
		sortedDates = append(sortedDates, d)
	}
	// 简单插入排序
	for i := 1; i < len(sortedDates); i++ {
		for j := i; j > 0 && sortedDates[j].Before(sortedDates[j-1]); j-- {
			sortedDates[j], sortedDates[j-1] = sortedDates[j-1], sortedDates[j]
		}
	}

	// 建立各股票的日期→组合价值映射
	stockDailyMap := make(map[string]map[string]float64)
	for id, r := range stockResults {
		m := make(map[string]float64, len(r.DailyRecords))
		for _, rec := range r.DailyRecords {
			m[rec.Date.Format("2006-01-02")] = rec.PortfolioValue
		}
		stockDailyMap[id] = m
	}

	// 按日期计算组合总价值
	dailyValues := make([]PortfolioDailyValue, 0, len(sortedDates))
	for _, d := range sortedDates {
		key := d.Format("2006-01-02")
		total := 0.0
		for _, id := range stocks {
			if m, ok := stockDailyMap[id]; ok {
				if v, ok2 := m[key]; ok2 {
					total += v
				} else {
					// 该股票当日无数据，用该股票分配的资金作为估值
					total += perCapital
				}
			}
		}
		dailyValues = append(dailyValues, PortfolioDailyValue{Date: d, Value: total})
	}

	result := &PortfolioBacktestResult{
		Stocks:       stocks,
		StrategyName: cfg.Name,
		DailyValues:  dailyValues,
		StockResults: stockResults,
	}

	// 基于组合总价值序列计算组合级绩效指标
	if len(dailyValues) > 0 {
		lastVal := dailyValues[len(dailyValues)-1].Value
		if initCapital > 0 {
			result.TotalReturn = (lastVal - initCapital) / initCapital * 100
		}

		// 年化收益率
		days := dailyValues[len(dailyValues)-1].Date.Sub(dailyValues[0].Date).Hours() / 24
		if days > 0 {
			years := days / 365.0
			result.AnnualReturn = (math.Pow(1+result.TotalReturn/100, 1/years) - 1) * 100
		}

		// 最大回撤
		peak := dailyValues[0].Value
		maxDD := 0.0
		for _, dv := range dailyValues {
			if dv.Value > peak {
				peak = dv.Value
			}
			if peak > 0 {
				dd := (peak - dv.Value) / peak * 100
				if dd > maxDD {
					maxDD = dd
				}
			}
		}
		result.MaxDrawdown = maxDD

		// 夏普比率（基于组合日收益率）
		portfolioReturns := make([]float64, 0, len(dailyValues)-1)
		for i := 1; i < len(dailyValues); i++ {
			prev := dailyValues[i-1].Value
			curr := dailyValues[i].Value
			if prev > 0 {
				portfolioReturns = append(portfolioReturns, (curr-prev)/prev)
			}
		}
		result.SharpeRatio = calcSharpeRatio(portfolioReturns, 0.02)
	}

	return result
}
