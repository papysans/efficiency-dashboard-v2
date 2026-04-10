package optimize

import (
	"fmt"
	"time"

	"comdigger/core/kline"
	"comdigger/core/technical"
)

// WFOWindowResult 单个WFO窗口的结果
type WFOWindowResult struct {
	WindowIdx   int
	TrainFrom   time.Time
	TrainTo     time.Time
	TestFrom    time.Time
	TestTo      time.Time
	BestParams  map[string]interface{}
	TrainResult *technical.BacktestResult
	TestResult  *technical.BacktestResult
}

// RunWFO 执行 Walk-Forward Optimization
func RunWFO(bars []kline.KlineBar, data *technical.TechnicalData, initCapital float64, cfg WFOConfig) ([]WFOWindowResult, error) {
	if cfg.TrainBars <= 0 {
		cfg.TrainBars = 200
	}
	if cfg.TestBars <= 0 {
		cfg.TestBars = 60
	}

	n := len(bars)
	if n < cfg.TrainBars+cfg.TestBars {
		return nil, fmt.Errorf("K线数据不足，需要至少 %d 条，当前 %d 条", cfg.TrainBars+cfg.TestBars, n)
	}

	var windows []WFOWindowResult
	windowIdx := 0

	for start := 0; start+cfg.TrainBars+cfg.TestBars <= n; start += cfg.TestBars {
		trainEnd := start + cfg.TrainBars
		testEnd := trainEnd + cfg.TestBars
		if testEnd > n {
			testEnd = n
		}

		trainBars := bars[start:trainEnd]
		testBars := bars[start:testEnd] // 测试集包含训练集数据（用于指标预热）

		// 训练集数据和指标
		trainData := technical.CalcAllIndicators(trainBars)
		testData := technical.CalcAllIndicators(testBars)

		// 在训练集上做 GridSearch
		trainFrom := trainBars[0].Time
		optResults, err := RunGridSearch(trainBars, trainData, trainFrom, initCapital, cfg.OptimizeConfig)
		if err != nil || len(optResults) == 0 {
			windowIdx++
			continue
		}

		bestResult := optResults[0]
		bestParams := bestResult.Params

		// 用最优参数在测试集上回测
		testFrom := bars[trainEnd].Time
		testCfg := technical.StrategyConfig{
			Type:          cfg.Strategy,
			Name:          fmt.Sprintf("WFO窗口%d", windowIdx+1),
			BuyThreshold:  toFloat64(bestParams["buy_threshold"]),
			SellThreshold: toFloat64(bestParams["sell_threshold"]),
			MAFast:        toInt(bestParams["ma_fast"]),
			MASlow:        toInt(bestParams["ma_slow"]),
			DrawdownStop:  0.20,
		}
		if testCfg.MAFast == 0 && cfg.Strategy == technical.StrategyTrendFilter {
			testCfg.MAFast = 20
		}
		if testCfg.MASlow == 0 && cfg.Strategy == technical.StrategyTrendFilter {
			testCfg.MASlow = 60
		}

		testResult := technical.RunBacktest(testBars, testData, testFrom, testCfg, initCapital)

		window := WFOWindowResult{
			WindowIdx:   windowIdx + 1,
			TrainFrom:   trainBars[0].Time,
			TrainTo:     trainBars[len(trainBars)-1].Time,
			TestFrom:    testFrom,
			TestTo:      testBars[len(testBars)-1].Time,
			BestParams:  bestParams,
			TrainResult: bestResult.Result,
			TestResult:  testResult,
		}
		windows = append(windows, window)

		fmt.Printf("  WFO窗口%d: 训练[%s~%s] 测试[%s~%s] 最优参数: buy=%.0f sell=%.0f 样本外收益: %+.2f%%\n",
			windowIdx+1,
			window.TrainFrom.Format("2006-01-02"),
			window.TrainTo.Format("2006-01-02"),
			window.TestFrom.Format("2006-01-02"),
			window.TestTo.Format("2006-01-02"),
			toFloat64(bestParams["buy_threshold"]),
			toFloat64(bestParams["sell_threshold"]),
			testResult.TotalReturn)

		windowIdx++
	}

	return windows, nil
}

func toFloat64(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	}
	return 0
}

func toInt(v interface{}) int {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case int:
		return val
	case float64:
		return int(val)
	}
	return 0
}
