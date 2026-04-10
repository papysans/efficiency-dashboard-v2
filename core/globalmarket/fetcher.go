package globalmarket

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// FetchStockData 从 stooq.com 抓取指定股票的K线数据
// 返回 nil, nil 表示数据不足（不视为错误）
func FetchStockData(symbol string, name string, days int) (*StockSummary, error) {
	d2 := time.Now()
	d1 := d2.AddDate(0, 0, -(days*2 + 10))
	d1Str := d1.Format("20060102")
	d2Str := d2.Format("20060102")

	url := fmt.Sprintf("https://stooq.com/q/d/l/?s=%s&i=d&d1=%s&d2=%s", symbol, d1Str, d2Str)

	client := &http.Client{Timeout: 10 * time.Second}

	var (
		resp *http.Response
		err  error
	)
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * time.Second)
		}
		resp, err = client.Get(url)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", symbol, err)
	}

	reader := csv.NewReader(resp.Body)
	records, parseErr := reader.ReadAll()
	resp.Body.Close()
	if parseErr != nil {
		return nil, fmt.Errorf("parse csv %s: %w", symbol, parseErr)
	}

	// 跳过表头行
	if len(records) < 2 {
		return nil, nil
	}
	records = records[1:]

	var bars []StockBar
	for _, row := range records {
		if len(row) < 6 {
			continue
		}
		date, err := time.Parse("2006-01-02", strings.TrimSpace(row[0]))
		if err != nil {
			continue
		}
		open, err := strconv.ParseFloat(strings.TrimSpace(row[1]), 64)
		if err != nil {
			continue
		}
		high, err := strconv.ParseFloat(strings.TrimSpace(row[2]), 64)
		if err != nil {
			continue
		}
		low, err := strconv.ParseFloat(strings.TrimSpace(row[3]), 64)
		if err != nil {
			continue
		}
		close, err := strconv.ParseFloat(strings.TrimSpace(row[4]), 64)
		if err != nil {
			continue
		}
		var volume int64
		volStr := strings.TrimSpace(row[5])
		if volStr != "" {
			volume, _ = strconv.ParseInt(volStr, 10, 64)
		}
		bars = append(bars, StockBar{
			Date:   date,
			Open:   open,
			High:   high,
			Low:    low,
			Close:  close,
			Volume: volume,
		})
	}

	// 取最后 days 行
	if len(bars) > days {
		bars = bars[len(bars)-days:]
	}

	if len(bars) < 2 {
		return nil, nil
	}

	last := len(bars) - 1
	dayChangePct := (bars[last].Close - bars[last-1].Close) / bars[last-1].Close * 100
	fiveDayChangePct := (bars[last].Close - bars[0].Close) / bars[0].Close * 100

	return &StockSummary{
		Symbol:           symbol,
		Name:             name,
		LatestClose:      bars[last].Close,
		DayChangePct:     dayChangePct,
		FiveDayChangePct: fiveDayChangePct,
		Bars:             bars,
	}, nil
}

// FetchMultiple 并发抓取多只股票数据
func FetchMultiple(symbols []string, names []string, days int) []StockSummary {
	sem := make(chan struct{}, 5)
	var wg sync.WaitGroup
	mu := sync.Mutex{}
	var results []StockSummary

	for i, sym := range symbols {
		name := ""
		if i < len(names) {
			name = names[i]
		}
		wg.Add(1)
		go func(s, n string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			summary, err := FetchStockData(s, n, days)
			if err != nil || summary == nil {
				return
			}
			mu.Lock()
			results = append(results, *summary)
			mu.Unlock()
		}(sym, name)
	}

	wg.Wait()
	return results
}

// FormatChangePct 格式化涨跌幅显示
func FormatChangePct(pct float64) string {
	if pct > 0 {
		return fmt.Sprintf("↑+%.2f%%", pct)
	} else if pct < 0 {
		return fmt.Sprintf("↓%.2f%%", pct)
	}
	return fmt.Sprintf("→0.00%%")
}
