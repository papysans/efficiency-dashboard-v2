package backtest

import (
	"fmt"
	"os"
	"strings"
	"unicode"

	"comdigger/core/technical"
)

// colorString applies ANSI color to a string (simplified version for terminal output)
func colorString(color, text string) string {
	if os.Getenv("NO_COLOR") != "" {
		return text
	}
	// Check if stdout is a terminal
	fi, err := os.Stdout.Stat()
	if err != nil || (fi.Mode()&os.ModeCharDevice) == 0 {
		return text
	}

	const reset = "\033[0m"
	switch strings.ToLower(color) {
	case "red", "error":
		return "\033[31m" + text + reset
	case "green", "success":
		return "\033[32m" + text + reset
	case "yellow", "warning":
		return "\033[33m" + text + reset
	case "blue", "info":
		return "\033[34m" + text + reset
	case "cyan":
		return "\033[36m" + text + reset
	case "bold":
		return "\033[1m" + text + reset
	default:
		return text
	}
}

// chineseWidth 计算字符串在终端中的显示宽度（中文字符占2，ASCII占1）
func chineseWidth(s string) int {
	w := 0
	for _, r := range s {
		if unicode.Is(unicode.Han, r) || (r >= 0xFF01 && r <= 0xFF60) || (r >= 0x3000 && r <= 0x303F) {
			w += 2
		} else {
			w += 1
		}
	}
	return w
}

// padRight 按显示宽度右填充空格（处理中文对齐）
func padRight(s string, width int) string {
	sw := chineseWidth(s)
	if sw >= width {
		return s
	}
	return s + strings.Repeat(" ", width-sw)
}

// padLeft 按显示宽度左填充空格（处理中文对齐，用于右对齐数字）
func padLeft(s string, width int) string {
	sw := chineseWidth(s)
	if sw >= width {
		return s
	}
	return strings.Repeat(" ", width-sw) + s
}

// PrintBacktestResult 输出单次回测结果
func PrintBacktestResult(stockName, companyID string, result *technical.BacktestResult) {
	sep := colorString("bold", "═══════════════════════════════════════════════════════════")
	thinSep := strings.Repeat("─", 65)

	fmt.Printf("\n%s\n", sep)
	fmt.Printf("  回测结果  %s (%s)  %s ~ %s\n",
		stockName, companyID,
		result.FromDate.Format("2006-01-02"),
		result.ToDate.Format("2006-01-02"))
	if result.StrategyName != "" {
		fmt.Printf("  策略: %s  %s\n", result.StrategyName, result.StrategyDesc)
	}
	fmt.Printf("  初始资金: %.2f 元\n", result.InitCapital)
	fmt.Printf("%s\n", sep)

	// ── 逐日信号记录表 ──────────────────────────────────────────────────
	fmt.Printf("\n%s\n", colorString("cyan", "【逐日信号记录】"))
	// 表头：日期(10) 收盘(10) 评分(8) 信号(6) 持仓(6) 组合价值(14)
	fmt.Printf("%s %s %s %s %s %s\n",
		padRight("日期", 10),
		padLeft("收盘", 10),
		padLeft("评分", 8),
		padRight("信号", 6),
		padRight("持仓", 6),
		padLeft("组合价值", 14))
	fmt.Printf("%s\n", thinSep)

	for _, rec := range result.DailyRecords {
		var sigColor string
		switch rec.Signal {
		case string(technical.SignalBuy):
			sigColor = "green"
		case string(technical.SignalSell):
			sigColor = "red"
		default:
			sigColor = "yellow"
		}
		// 信号和持仓：ANSI转义码不占显示宽度，需用原始字符串计算宽度再填充
		rawSig := rec.Signal // 原始文字，用于计算宽度
		rawPos := "空仓"
		if rec.Position {
			rawPos = "持仓"
		}
		sigStr := colorString(sigColor, rawSig)
		posStr := rawPos
		if rec.Position {
			posStr = colorString("green", rawPos)
		}
		sigPad := strings.Repeat(" ", 6-chineseWidth(rawSig)) // 目标宽6
		posPad := strings.Repeat(" ", 6-chineseWidth(rawPos)) // 目标宽6
		fmt.Printf("%s %s %s %s %s %s\n",
			rec.Date.Format("2006-01-02"),
			padLeft(fmt.Sprintf("%.2f", rec.Close), 10),
			padLeft(fmt.Sprintf("%+.1f", rec.Score), 8),
			sigStr+sigPad,
			posStr+posPad,
			padLeft(fmt.Sprintf("%.2f", rec.PortfolioValue), 14))
	}
	fmt.Printf("%s\n", thinSep)

	// ── 交易明细表 ──────────────────────────────────────────────────────
	fmt.Printf("\n%s\n", colorString("cyan", "【交易明细】"))
	if len(result.Trades) == 0 {
		fmt.Println("  无交易记录")
	} else {
		// 表头
		fmt.Printf("%s %s  %s %s  %s  %s  %s\n",
			padRight("买入日期", 10),
			padLeft("买入价", 10),
			padRight("卖出日期", 10),
			padLeft("卖出价", 10),
			padLeft("持仓天", 6),
			padLeft("盈亏(元)", 12),
			padLeft("盈亏率", 8))
		fmt.Printf("%s\n", thinSep)
		for _, t := range result.Trades {
			pnlStr := fmt.Sprintf("%+.2f", t.PnL)
			pnlPctStr := fmt.Sprintf("%+.2f%%", t.PnLPct)
			color := "green"
			if t.PnL < 0 {
				color = "red"
			}
			fmt.Printf("%s %s  %s %s  %s  %s  %s\n",
				t.BuyDate.Format("2006-01-02"),
				padLeft(fmt.Sprintf("%.2f", t.BuyPrice), 10),
				t.SellDate.Format("2006-01-02"),
				padLeft(fmt.Sprintf("%.2f", t.SellPrice), 10),
				padLeft(fmt.Sprintf("%d", t.HoldDays), 6),
				colorString(color, padLeft(pnlStr, 12)),
				colorString(color, padLeft(pnlPctStr, 8)))
		}
		fmt.Printf("%s\n", thinSep)
	}

	// ── 汇总统计 ────────────────────────────────────────────────────────
	fmt.Printf("\n%s\n", colorString("cyan", "【汇总统计】"))

	retColor := "green"
	if result.TotalReturn < 0 {
		retColor = "red"
	}
	annColor := "green"
	if result.AnnualReturn < 0 {
		annColor = "red"
	}
	bhColor := "green"
	if result.BuyHoldReturn < 0 {
		bhColor = "red"
	}

	fmt.Printf("  总收益率:   %s\n",
		colorString(retColor, fmt.Sprintf("%+.2f%%", result.TotalReturn)))
	fmt.Printf("  年化收益率: %s\n",
		colorString(annColor, fmt.Sprintf("%+.2f%%", result.AnnualReturn)))
	fmt.Printf("  最大回撤:   %.2f%%\n", result.MaxDrawdown)
	fmt.Printf("  胜率:       %.1f%%  （%d 笔交易）\n", result.WinRate, result.TotalTrades)
	fmt.Printf("  最终资金:   %.2f 元\n", result.FinalCapital)
	fmt.Printf("  夏普比率:   %.2f  索提诺: %.2f  卡玛: %.2f\n", result.SharpeRatio, result.SortinoRatio, result.CalmarRatio)
	fmt.Printf("  年化波动率: %.1f%%  最大回撤持续: %d天  盈亏比: %.2f\n", result.Volatility, result.MaxDrawdownDays, result.ProfitFactor)
	fmt.Printf("  持仓时间:   %.1f%%（%d天）  平均每笔: %+.2f%%\n", result.ExposurePct, result.ExposureDays, result.AvgPnLPct)
	fmt.Printf("  VaR(95%%):   %.2f%%  VaR(99%%): %.2f%%  CVaR(95%%): %.2f%%\n", result.VaR95, result.VaR99, result.CVaR95)
	fmt.Printf("  溃疡指数:   %.4f  MAE均值: %.2f%%  MFE均值: %.2f%%\n", result.UlcerIndex, result.AvgMAE, result.AvgMFE)
	fmt.Printf("  交易成本:   佣金%.2f元 + 印花税%.2f元 + 滑点%.2f元 = 合计%.2f元\n", result.TotalCommission, result.TotalStampDuty, result.TotalSlippage, result.TotalTradingCost)
	fmt.Printf("  规则阻止:   T+1阻止%d次  涨跌停阻止%d次\n", result.T1Blocked, result.LimitBlocked)
	fmt.Printf("  平均持仓:   %.1f天  最大连续亏损:%d次\n", result.AvgHoldDays, result.MaxLossStreak)

	// 月度收益率表格
	if len(result.DailyRecords) > 0 {
		fmt.Printf("\n%s\n", colorString("cyan", "【月度收益率】"))

		type monthKey struct{ year, month int }
		type monthData struct{ startVal, endVal float64 }
		monthMap := make(map[monthKey]monthData)

		for i, rec := range result.DailyRecords {
			t := rec.Date
			key := monthKey{t.Year(), int(t.Month())}
			md := monthMap[key]
			md.endVal = rec.PortfolioValue
			if _, exists := monthMap[key]; !exists {
				if i > 0 {
					md.startVal = result.DailyRecords[i-1].PortfolioValue
				} else {
					md.startVal = rec.PortfolioValue
				}
			}
			monthMap[key] = md
		}

		yearSet := make(map[int]bool)
		for k := range monthMap {
			yearSet[k.year] = true
		}
		years := make([]int, 0, len(yearSet))
		for y := range yearSet {
			years = append(years, y)
		}
		for i := 1; i < len(years); i++ {
			for j := i; j > 0 && years[j] < years[j-1]; j-- {
				years[j], years[j-1] = years[j-1], years[j]
			}
		}

		monthAbbr := []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}

		for _, year := range years {
			line := fmt.Sprintf("  %d:", year)
			for m := 1; m <= 12; m++ {
				key := monthKey{year, m}
				md, ok := monthMap[key]
				if !ok {
					continue
				}
				var ret float64
				if md.startVal > 0 {
					ret = (md.endVal - md.startVal) / md.startVal * 100
				}
				retStr := fmt.Sprintf("%+.1f%%", ret)
				if ret >= 0 {
					line += " " + monthAbbr[m-1] + colorString("green", retStr)
				} else {
					line += " " + monthAbbr[m-1] + colorString("red", retStr)
				}
			}
			fmt.Println(line)
		}
	}

	// ── 与买入持有对比 ──────────────────────────────────────────────────
	fmt.Printf("\n%s\n", colorString("cyan", "【策略 vs 买入持有】"))
	diff := result.TotalReturn - result.BuyHoldReturn
	diffColor := "green"
	beatStr := "跑赢"
	if diff < 0 {
		diffColor = "red"
		beatStr = "跑输"
	}

	// 对比表格：列1=指标名(16宽)，列2=本策略(10宽)，列3=买入持有(10宽)
	// 表头行：中文字符各占2宽，需用 padLeft 补齐到数字列宽度
	col2W := 10
	col3W := 10
	fmt.Printf("  %s  %s  %s\n",
		padRight("", 16),
		padLeft("本策略", col2W),  // "本策略"=6宽，填充4空格 → 10宽
		padLeft("买入持有", col3W)) // "买入持有"=8宽，填充2空格 → 10宽
	fmt.Printf("  %s  %s  %s\n",
		padRight("总收益率", 16),
		colorString(retColor, padLeft(fmt.Sprintf("%+.2f%%", result.TotalReturn), col2W)),
		colorString(bhColor, padLeft(fmt.Sprintf("%+.2f%%", result.BuyHoldReturn), col3W)))
	fmt.Printf("  %s  %s  %s\n",
		padRight("年化收益率", 16),
		colorString(annColor, padLeft(fmt.Sprintf("%+.2f%%", result.AnnualReturn), col2W)),
		padLeft("-", col3W))

	// 超额收益结论
	fmt.Printf("\n  结论: 策略%s买入持有 %s\n",
		beatStr,
		colorString(diffColor, fmt.Sprintf("%+.2f%%", diff)))
	if diff < 0 {
		fmt.Printf("  提示: 该股票本身涨幅 %s，技术择时未能跑赢自然趋势\n",
			colorString(bhColor, fmt.Sprintf("%+.2f%%", result.BuyHoldReturn)))
	} else {
		fmt.Printf("  提示: 技术择时有效，相比持有节省了 %.2f%% 的最大回撤风险\n",
			result.MaxDrawdown)
	}

	fmt.Printf("%s\n\n", sep)
}

// PrintAllBacktestsComparison 输出多策略回测横向对比表
func PrintAllBacktestsComparison(results []*technical.BacktestResult) {
	if len(results) == 0 {
		return
	}

	sep := colorString("bold", "═══════════════════════════════════════════════════════════════")
	thinSep := strings.Repeat("─", 67)

	// 取第一个有效结果的日期范围
	fromStr := results[0].FromDate.Format("2006-01-02")
	toStr := results[0].ToDate.Format("2006-01-02")

	fmt.Printf("\n%s\n", sep)
	fmt.Printf("  多策略回测对比  %s ~ %s\n", fromStr, toStr)
	fmt.Printf("%s\n", sep)

	// 表头
	fmt.Printf("%s  %s  %s  %s  %s  %s\n",
		padRight("策略名称", 14),
		padLeft("总收益率", 10),
		padLeft("年化收益率", 10),
		padLeft("最大回撤", 10),
		padLeft("胜率", 8),
		padLeft("交易次数", 8))
	fmt.Printf("%s\n", thinSep)

	// 找最优策略（总收益率最高）
	bestIdx := 0
	for i, r := range results {
		if r.TotalReturn > results[bestIdx].TotalReturn {
			bestIdx = i
		}
	}

	for _, r := range results {
		retStr := fmt.Sprintf("%+.2f%%", r.TotalReturn)
		retColor := "green"
		if r.TotalReturn < 0 {
			retColor = "red"
		}
		winRateStr := "-"
		if r.TotalTrades > 0 {
			winRateStr = fmt.Sprintf("%.1f%%", r.WinRate)
		}
		annStr := fmt.Sprintf("%+.2f%%", r.AnnualReturn)
		fmt.Printf("%s  %s  %s  %s  %s  %s\n",
			padRight(r.StrategyName, 14),
			colorString(retColor, padLeft(retStr, 10)),
			padLeft(annStr, 10),
			padLeft(fmt.Sprintf("%.2f%%", r.MaxDrawdown), 10),
			padLeft(winRateStr, 8),
			padLeft(fmt.Sprintf("%d", r.TotalTrades), 8))
	}

	// 买入持有行（取第一个结果的BuyHoldReturn）
	bhReturn := results[0].BuyHoldReturn
	bhColor := "green"
	if bhReturn < 0 {
		bhColor = "red"
	}
	fmt.Printf("%s  %s  %s  %s  %s  %s\n",
		padRight("买入持有", 14),
		colorString(bhColor, padLeft(fmt.Sprintf("%+.2f%%", bhReturn), 10)),
		padLeft("-", 10),
		padLeft("-", 10),
		padLeft("-", 8),
		padLeft("-", 8))

	fmt.Printf("%s\n", thinSep)
	fmt.Printf("最优策略: %s（总收益率 %+.2f%%）\n",
		colorString("green", results[bestIdx].StrategyName),
		results[bestIdx].TotalReturn)
	fmt.Printf("%s\n\n", sep)
}
