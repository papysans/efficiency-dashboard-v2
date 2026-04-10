package optimize

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"comdigger/core/kline"
	"comdigger/core/technical"
)

// RunGridSearch 并发遍历所有参数组合，找到最优策略参数
func RunGridSearch(bars []kline.KlineBar, data *technical.TechnicalData, from time.Time, initCapital float64, cfg OptimizeConfig) ([]OptimizeResult, error) {
	if cfg.MaxWorkers <= 0 {
		cfg.MaxWorkers = 4
	}
	if cfg.TopN <= 0 {
		cfg.TopN = 10
	}
	if cfg.SortBy == "" {
		cfg.SortBy = "sharpe_ratio"
	}

	// 持久化初始化
	if cfg.DB != nil && cfg.JobID == "" {
		if cfg.Resume {
			// 断点续传：优先复用最近一次同公司+策略的 JobID
			if latestJobID, err := loadLatestJobID(cfg.DB, cfg.CompanyID, cfg.Strategy); err == nil && latestJobID != "" {
				cfg.JobID = latestJobID
			} else {
				cfg.JobID = generateJobID(cfg.CompanyID, cfg.Strategy)
			}
		} else {
			cfg.JobID = generateJobID(cfg.CompanyID, cfg.Strategy)
		}
	}
	var completedSet map[string]bool
	if cfg.Resume && cfg.DB != nil {
		var err error
		completedSet, err = loadCompletedParams(cfg.DB, cfg.JobID)
		if err != nil {
			fmt.Printf("  警告：加载已完成参数失败: %v，将重新执行所有参数组合\n", err)
		}
	}

	// 生成所有参数组合（笛卡尔积）
	type paramSet struct {
		buyThreshold  float64
		sellThreshold float64
		maFast        int
		maSlow        int
	}

	var combos []paramSet

	buyThresholds := cfg.Grid.BuyThresholds
	if len(buyThresholds) == 0 {
		buyThresholds = []float64{0}
	}
	sellThresholds := cfg.Grid.SellThresholds
	if len(sellThresholds) == 0 {
		sellThresholds = []float64{0}
	}
	maFasts := cfg.Grid.MAFasts
	if len(maFasts) == 0 {
		maFasts = []int{0}
	}
	maSlows := cfg.Grid.MASlows
	if len(maSlows) == 0 {
		maSlows = []int{0}
	}

	for _, bt := range buyThresholds {
		for _, st := range sellThresholds {
			for _, mf := range maFasts {
				for _, ms := range maSlows {
					// 断点续传：跳过已完成的参数组合
					if cfg.Resume && completedSet != nil {
						paramKey := map[string]interface{}{
							"buy_threshold":  bt,
							"sell_threshold": st,
							"ma_fast":        mf,
							"ma_slow":        ms,
						}
						if keyBytes, err := json.Marshal(paramKey); err == nil {
							if completedSet[string(keyBytes)] {
								continue
							}
						}
					}
					combos = append(combos, paramSet{bt, st, mf, ms})
				}
			}
		}
	}

	total := len(combos)
	results := make([]OptimizeResult, 0, total)
	var mu sync.Mutex
	var wg sync.WaitGroup

	sem := make(chan struct{}, cfg.MaxWorkers)
	completed := 0

	for _, combo := range combos {
		wg.Add(1)
		sem <- struct{}{}
		go func(p paramSet) {
			defer wg.Done()
			defer func() { <-sem }()

			strategyCfg := technical.StrategyConfig{
				Type:          cfg.Strategy,
				Name:          cfg.Strategy,
				BuyThreshold:  p.buyThreshold,
				SellThreshold: p.sellThreshold,
				MAFast:        p.maFast,
				MASlow:        p.maSlow,
				DrawdownStop:  0.20,
			}
			// 补充策略默认值
			switch cfg.Strategy {
			case technical.StrategyTrendFilter:
				if strategyCfg.MAFast == 0 {
					strategyCfg.MAFast = 20
				}
				if strategyCfg.MASlow == 0 {
					strategyCfg.MASlow = 60
				}
			}

			btResult := technical.RunBacktest(bars, data, from, strategyCfg, initCapital)
			params := map[string]interface{}{
				"buy_threshold":  p.buyThreshold,
				"sell_threshold": p.sellThreshold,
				"ma_fast":        p.maFast,
				"ma_slow":        p.maSlow,
			}

			// 持久化（在 mu.Lock 之前执行，确保 wg.Wait 返回前所有结果已写入 DB）
			if cfg.DB != nil {
				_ = saveOptimizeResult(cfg.DB, cfg.JobID, cfg.CompanyID, cfg.Strategy, params, btResult)
			}

			mu.Lock()
			results = append(results, OptimizeResult{
				Params: params,
				Result: btResult,
				SortBy: cfg.SortBy,
			})
			completed++
			if total > 0 && completed%(total/10+1) == 0 {
				pct := float64(completed) / float64(total) * 100
				fmt.Printf("  优化进度: %d/%d (%.0f%%)\n", completed, total, pct)
			}
			mu.Unlock()
		}(combo)
	}

	wg.Wait()

	// 断点续传且本次无新计算时，从 DB 加载已有结果
	if cfg.Resume && cfg.DB != nil && len(results) == 0 && len(completedSet) > 0 {
		dbResults, err := loadResultsFromDB(cfg.DB, cfg.JobID, cfg.SortBy, cfg.TopN)
		if err == nil && len(dbResults) > 0 {
			if cfg.DB != nil {
				fmt.Printf("  优化结果已保存到 DB，JobID: %s\n", cfg.JobID)
			}
			return dbResults, nil
		}
	}

	// 按 SortBy 降序排序
	sort.Slice(results, func(i, j int) bool {
		return getMetric(results[i].Result, cfg.SortBy) > getMetric(results[j].Result, cfg.SortBy)
	})

	if len(results) > cfg.TopN {
		results = results[:cfg.TopN]
	}

	if cfg.DB != nil {
		fmt.Printf("  优化结果已保存到 DB，JobID: %s\n", cfg.JobID)
	}

	return results, nil
}

// getMetric 从 BacktestResult 中获取指定指标值
func getMetric(r *technical.BacktestResult, metric string) float64 {
	if r == nil {
		return 0
	}
	switch metric {
	case "sharpe_ratio":
		return r.SharpeRatio
	case "total_return":
		return r.TotalReturn
	case "calmar_ratio":
		return r.CalmarRatio
	case "annual_return":
		return r.AnnualReturn
	case "win_rate":
		return r.WinRate
	default:
		return r.SharpeRatio
	}
}
