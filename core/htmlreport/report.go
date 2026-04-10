package htmlreport

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"text/template"

	"comdigger/core/technical"
)

// ReportData HTML报告模板数据
type ReportData struct {
	Title             string
	EquityCurveData   string // JSON
	MonthlyReturns    string // JSON
	TradesData        string // JSON
	StrategyCompare   string // JSON
	RollingSharpeData string // JSON: [{date, sharpe}]，60日滚动Sharpe序列
	BubbleData        string // JSON: [{name, totalReturn, maxDD, sharpe}]，策略气泡图
	PnLHistData       string // JSON: [{bin, count}]，盈亏%分布直方图
	HoldDaysHistData  string // JSON: [{bin, count}]，持仓天数分布直方图
}

// equityPoint 权益曲线数据点
type equityPoint struct {
	Date           string  `json:"date"`
	Value          float64 `json:"value"`
	Drawdown       float64 `json:"drawdown"`
	BenchmarkValue float64 `json:"benchmarkValue"`
}

// monthlyReturn 月度收益数据点
type monthlyReturn struct {
	Year   int     `json:"year"`
	Month  int     `json:"month"`
	Return float64 `json:"return"`
}

// tradePoint 交易散点数据
type tradePoint struct {
	HoldDays     int     `json:"holdDays"`
	PnLPct       float64 `json:"pnlPct"`
	StrategyName string  `json:"strategyName"`
}

// strategyCompare 策略对比数据
type strategyCompare struct {
	Name        string  `json:"name"`
	TotalReturn float64 `json:"totalReturn"`
	Sharpe      float64 `json:"sharpe"`
	MaxDD       float64 `json:"maxDD"`
}

// BuildReportData 从回测结果构建报告数据
func BuildReportData(stockName string, results []*technical.BacktestResult) ReportData {
	data := ReportData{
		Title: stockName,
	}

	// 权益曲线（取第一个有记录的策略）
	var equityCurve []equityPoint
	for _, r := range results {
		if len(r.DailyRecords) > 0 {
			peak := r.DailyRecords[0].PortfolioValue
			for _, rec := range r.DailyRecords {
				if rec.PortfolioValue > peak {
					peak = rec.PortfolioValue
				}
				dd := 0.0
				if peak > 0 {
					dd = (peak - rec.PortfolioValue) / peak * 100
				}
				equityCurve = append(equityCurve, equityPoint{
					Date:           rec.Date.Format("2006-01-02"),
					Value:          rec.PortfolioValue,
					Drawdown:       dd,
					BenchmarkValue: rec.BenchmarkValue,
				})
			}
			break
		}
	}
	equityJSON, _ := json.Marshal(equityCurve)
	data.EquityCurveData = string(equityJSON)

	// 月度收益（取第一个有记录的策略）
	var monthly []monthlyReturn
	for _, r := range results {
		if len(r.DailyRecords) > 0 {
			type monthKey struct{ year, month int }
			monthMap := make(map[monthKey][]float64)
			for i := 1; i < len(r.DailyRecords); i++ {
				prev := r.DailyRecords[i-1].PortfolioValue
				curr := r.DailyRecords[i].PortfolioValue
				if prev > 0 {
					ret := (curr - prev) / prev * 100
					t := r.DailyRecords[i].Date
					key := monthKey{t.Year(), int(t.Month())}
					monthMap[key] = append(monthMap[key], ret)
				}
			}
			for k, rets := range monthMap {
				sum := 0.0
				for _, v := range rets {
					sum += v
				}
				monthly = append(monthly, monthlyReturn{
					Year:   k.year,
					Month:  k.month,
					Return: sum,
				})
			}
			break
		}
	}
	monthlyJSON, _ := json.Marshal(monthly)
	data.MonthlyReturns = string(monthlyJSON)

	// 交易散点（所有策略合并）
	var trades []tradePoint
	for _, r := range results {
		for _, t := range r.Trades {
			trades = append(trades, tradePoint{
				HoldDays:     t.HoldDays,
				PnLPct:       t.PnLPct,
				StrategyName: r.StrategyName,
			})
		}
	}
	tradesJSON, _ := json.Marshal(trades)
	data.TradesData = string(tradesJSON)

	// 策略对比
	var compares []strategyCompare
	for _, r := range results {
		compares = append(compares, strategyCompare{
			Name:        r.StrategyName,
			TotalReturn: r.TotalReturn,
			Sharpe:      r.SharpeRatio,
			MaxDD:       r.MaxDrawdown,
		})
	}
	compareJSON, _ := json.Marshal(compares)
	data.StrategyCompare = string(compareJSON)

	// RollingSharpeData：60日滚动Sharpe
	type sharpePoint struct {
		Date   string  `json:"date"`
		Sharpe float64 `json:"sharpe"`
	}
	var sharpePoints []sharpePoint
	if len(results) > 0 && len(results[0].DailyRecords) > 0 {
		recs := results[0].DailyRecords
		window := 60
		for i := window; i < len(recs); i++ {
			// 计算窗口内日收益率
			var rets []float64
			for j := i - window + 1; j <= i; j++ {
				prev := recs[j-1].PortfolioValue
				curr := recs[j].PortfolioValue
				if prev > 0 {
					rets = append(rets, (curr-prev)/prev)
				}
			}
			if len(rets) < 2 {
				continue
			}
			// 均值
			sum := 0.0
			for _, r := range rets {
				sum += r
			}
			mean := sum / float64(len(rets))
			// 标准差
			varSum := 0.0
			for _, r := range rets {
				d := r - mean
				varSum += d * d
			}
			std := math.Sqrt(varSum / float64(len(rets)-1))
			if std == 0 {
				continue
			}
			sharpe := mean / std * math.Sqrt(252)
			sharpePoints = append(sharpePoints, sharpePoint{
				Date:   recs[i].Date.Format("2006-01-02"),
				Sharpe: math.Round(sharpe*100) / 100,
			})
		}
	}
	if sharpePoints == nil {
		data.RollingSharpeData = "[]"
	} else {
		b, _ := json.Marshal(sharpePoints)
		data.RollingSharpeData = string(b)
	}

	// BubbleData：各策略气泡图
	type bubblePoint struct {
		Name        string  `json:"name"`
		TotalReturn float64 `json:"totalReturn"`
		MaxDD       float64 `json:"maxDD"`
		Sharpe      float64 `json:"sharpe"`
	}
	var bubbles []bubblePoint
	for _, r := range results {
		bubbles = append(bubbles, bubblePoint{
			Name:        r.StrategyName,
			TotalReturn: math.Round(r.TotalReturn*100) / 100,
			MaxDD:       math.Round(r.MaxDrawdown*100) / 100,
			Sharpe:      math.Round(r.SharpeRatio*100) / 100,
		})
	}
	if bubbles == nil {
		data.BubbleData = "[]"
	} else {
		b, _ := json.Marshal(bubbles)
		data.BubbleData = string(b)
	}

	// PnLHistData：盈亏%分布直方图，bin宽5%，范围-50%~+100%
	type histBin struct {
		Bin   string `json:"bin"`
		Count int    `json:"count"`
	}
	var pnlHist []histBin
	if len(results) > 0 {
		// 定义bins：-50~-45, -45~-40, ..., -5~0, 0~5, ..., 95~100
		binMin, binMax, binWidth := -50, 100, 5
		binCount := (binMax - binMin) / binWidth
		counts := make([]int, binCount)
		for _, t := range results[0].Trades {
			v := t.PnLPct
			if v < float64(binMin) {
				v = float64(binMin)
			}
			if v >= float64(binMax) {
				v = float64(binMax) - 0.001
			}
			idx := int((v - float64(binMin)) / float64(binWidth))
			if idx < 0 {
				idx = 0
			}
			if idx >= binCount {
				idx = binCount - 1
			}
			counts[idx]++
		}
		for i := 0; i < binCount; i++ {
			lo := binMin + i*binWidth
			hi := lo + binWidth
			pnlHist = append(pnlHist, histBin{
				Bin:   fmt.Sprintf("%d%%~%d%%", lo, hi),
				Count: counts[i],
			})
		}
	}
	if pnlHist == nil {
		data.PnLHistData = "[]"
	} else {
		b, _ := json.Marshal(pnlHist)
		data.PnLHistData = string(b)
	}

	// HoldDaysHistData：持仓天数分布直方图，bin宽5天，范围0~100天
	var holdHist []histBin
	if len(results) > 0 {
		binWidth := 5
		maxBin := 100
		binCount := maxBin / binWidth     // 20个bin: 0~5, 5~10, ..., 95~100
		counts := make([]int, binCount+1) // +1 for "100天+"
		for _, t := range results[0].Trades {
			d := t.HoldDays
			if d >= maxBin {
				counts[binCount]++
			} else {
				idx := d / binWidth
				if idx >= binCount {
					idx = binCount - 1
				}
				counts[idx]++
			}
		}
		for i := 0; i < binCount; i++ {
			lo := i * binWidth
			hi := lo + binWidth
			holdHist = append(holdHist, histBin{
				Bin:   fmt.Sprintf("%d~%d天", lo, hi),
				Count: counts[i],
			})
		}
		holdHist = append(holdHist, histBin{Bin: "100天+", Count: counts[binCount]})
	}
	if holdHist == nil {
		data.HoldDaysHistData = "[]"
	} else {
		b, _ := json.Marshal(holdHist)
		data.HoldDaysHistData = string(b)
	}

	return data
}

// GenerateHTML 生成HTML报告文件
func GenerateHTML(data ReportData, outputPath string) error {
	tmpl, err := template.New("report").Parse(reportTemplate)
	if err != nil {
		return fmt.Errorf("解析模板失败: %w", err)
	}
	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer f.Close()
	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("渲染模板失败: %w", err)
	}
	return nil
}
