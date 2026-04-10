package xueqiu

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// reportDef report configuration
var reportDefs = []struct {
	id   string
	name string
}{
	{id: "ZCFZB", name: "Balance Sheet"},
	{id: "GSLRB", name: "Income Statement"},
	{id: "XJLLB", name: "Cash Flow Statement"},
}

// FinReportTable financial report table
type FinReportTable struct {
	Type    string   // "Balance Sheet" / "Income Statement" / "Cash Flow Statement"
	Headers []string // Period column headers (excluding "Indicator" column), e.g. ["2025 Q3", "2025 H1", ...]
	Rows    []FinReportRow
}

// FinReportRow financial report row
type FinReportRow struct {
	Name   string   // Indicator name
	Values []string // Values for each period, corresponds to Headers
}

// FinReportClient financial report browser client
// Each instance holds an independent browser for concurrent scraping
type FinReportClient struct {
	browser *Browser
}

// NewFinReportClient creates a new FinReportClient instance
func NewFinReportClient(b *Browser) *FinReportClient {
	return &FinReportClient{browser: b}
}

// pageWorker scraping unit for a single report, holds independent page
type pageWorker struct {
	page *rod.Page
}

// initPage creates new page and completes Xueqiu anti-scraping initialization
func (c *FinReportClient) initPage(timeout time.Duration) (*pageWorker, error) {
	page, err := c.browser.NewPage()
	if err != nil {
		return nil, fmt.Errorf("failed to create page: %w", err)
	}

	_ = page.SetUserAgent(&proto.NetworkSetUserAgentOverride{
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	})
	_, _ = page.EvalOnNewDocument(`Object.defineProperty(navigator, 'webdriver', {get: () => undefined});`)

	_ = page.Timeout(timeout).Navigate("https://xueqiu.com")
	time.Sleep(4 * time.Second)

	if _, err := page.Timeout(10 * time.Second).Eval(`() => document.title`); err != nil {
		_ = page.Close()
		return nil, fmt.Errorf("xueqiu home not available: %w", err)
	}
	return &pageWorker{page: page}, nil
}

// FetchAllReports concurrently scrapes three financial reports, each with independent page, finally aggregates and returns
func (c *FinReportClient) FetchAllReports(code string, timeout time.Duration) ([]FinReportTable, error) {
	type result struct {
		idx   int
		table FinReportTable
		err   error
	}

	results := make([]result, len(reportDefs))
	var wg sync.WaitGroup
	ch := make(chan result, len(reportDefs))

	for i, def := range reportDefs {
		wg.Add(1)
		go func(idx int, id, name string) {
			defer wg.Done()

			worker, err := c.initPage(timeout)
			if err != nil {
				ch <- result{idx: idx, err: fmt.Errorf("init page for %s: %w", name, err)}
				return
			}
			defer worker.page.Close()

			table, err := worker.fetchOneReport(code, id, name, timeout)
			ch <- result{idx: idx, table: table, err: err}
		}(i, def.id, def.name)
	}

	// Wait for all goroutines to complete before closing channel
	go func() {
		wg.Wait()
		close(ch)
	}()

	var firstErr error
	for r := range ch {
		if r.err != nil {
			// 单张表失败只记录警告，不中断其他表的数据保存
			log.Printf("[xueqiu] 警告：%s 抓取失败（跳过此表）: %v", reportDefs[r.idx].name, r.err)
			if firstErr == nil {
				firstErr = r.err
			}
			continue
		}
		results[r.idx] = r
	}

	tables := make([]FinReportTable, len(reportDefs))
	successCount := 0
	for i, r := range results {
		tables[i] = r.table
		if r.table.Type != "" {
			successCount++
		}
	}

	// 全部失败才返回 error；部分成功则继续处理已有数据
	if successCount == 0 {
		if firstErr != nil {
			return nil, fmt.Errorf("所有报表抓取均失败: %w", firstErr)
		}
		return nil, fmt.Errorf("所有报表抓取均失败")
	}
	return tables, nil
}

// fetchOneReport scrapes all paginated data for a single report
func (w *pageWorker) fetchOneReport(code, reportID, reportName string, timeout time.Duration) (FinReportTable, error) {
	targetURL := fmt.Sprintf("https://xueqiu.com/snowman/S/%s/detail#/%s", code, reportID)

	if err := w.page.Timeout(30 * time.Second).Navigate(targetURL); err != nil {
		return FinReportTable{}, fmt.Errorf("navigate to %s: %w", targetURL, err)
	}
	if _, err := w.page.Timeout(timeout).Element(".stock-info-content"); err != nil {
		return FinReportTable{}, fmt.Errorf(".stock-info-content not found for %s: %w", reportName, err)
	}
	time.Sleep(2 * time.Second)

	table := FinReportTable{Type: reportName}

	for {
		periods, rows, err := w.extractTableData()
		if err != nil {
			return FinReportTable{}, fmt.Errorf("extract table data for %s: %w", reportName, err)
		}

		if len(table.Headers) == 0 {
			// First page: initialize directly
			table.Headers = periods
			for _, row := range rows {
				if len(row) == 0 {
					continue
				}
				table.Rows = append(table.Rows, FinReportRow{
					Name:   row[0],
					Values: append([]string{}, row[1:]...),
				})
			}
		} else {
			// Subsequent pages: append after skipping columns overlapping with previous page end
			skipCols := 0
			for i, p := range periods {
				tailIdx := len(table.Headers) - len(periods) + i
				if tailIdx >= 0 && tailIdx < len(table.Headers) && table.Headers[tailIdx] == p {
					skipCols++
				} else {
					break
				}
			}
			newPeriods := periods[skipCols:]
			if len(newPeriods) == 0 {
				break
			}
			table.Headers = append(table.Headers, newPeriods...)
			for rowIdx, row := range rows {
				if rowIdx >= len(table.Rows) {
					break
				}
				newValues := row[1+skipCols:]
				table.Rows[rowIdx].Values = append(table.Rows[rowIdx].Values, newValues...)
			}
		}

		hasNextResult, err := w.page.Timeout(5 * time.Second).Eval(`() => {
			const btn = Array.from(document.querySelectorAll('a,button,span,li')).find(
				el => el.textContent.trim().includes('下一页') && el.offsetParent !== null
			);
			if (!btn) return false;
			if (btn.getAttribute('disabled') !== null) return false;
			return true;
		}`)
		if err != nil {
			break
		}
		hasNext, _ := hasNextResult.Value.MarshalJSON()
		if string(hasNext) != "true" {
			break
		}

		firstPeriodBefore := w.getFirstPeriodCellText()

		_, _ = w.page.Timeout(5 * time.Second).Eval(`() => {
			const btn = Array.from(document.querySelectorAll('a,button,span,li')).find(
				el => el.textContent.trim().includes('下一页') &&
				      el.offsetParent !== null &&
				      el.getAttribute('disabled') === null
			);
			if (btn) btn.click();
		}`)

		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			time.Sleep(300 * time.Millisecond)
			if w.getFirstPeriodCellText() != firstPeriodBefore {
				break
			}
		}
	}

	return table, nil
}

// getFirstPeriodCellText gets text of first data cell in first period column (second column)
func (w *pageWorker) getFirstPeriodCellText() string {
	result, err := w.page.Timeout(3 * time.Second).Eval(`() => {
		const table = document.querySelector('.stock-info-content table');
		if (!table) return '';
		const rows = table.querySelectorAll('tbody tr');
		for (let i = 0; i < rows.length; i++) {
			const cells = rows[i].querySelectorAll('td');
			if (cells.length > 1) return cells[1].textContent.trim();
		}
		return '';
	}`)
	if err != nil {
		return ""
	}
	val, err := result.Value.MarshalJSON()
	if err != nil {
		return ""
	}
	s := string(val)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	return s
}

// extractTableData extracts current page table data
func (w *pageWorker) extractTableData() (periods []string, rows [][]string, err error) {
	result, err := w.page.Timeout(10 * time.Second).Eval(`() => {
		const table = document.querySelector('.stock-info-content table');
		if (!table) return {periods: [], rows: []};

		const theadThs = Array.from(table.querySelectorAll('thead th'));
		const periods = theadThs.slice(1).map(th => th.textContent.trim());

		const rows = Array.from(table.querySelectorAll('tbody tr')).map(tr => {
			return Array.from(tr.querySelectorAll('td')).map(td => td.textContent.trim());
		}).filter(r => r.length > 0);

		return {periods, rows};
	}`)
	if err != nil {
		return nil, nil, fmt.Errorf("eval table extraction: %w", err)
	}

	var parsed struct {
		Periods []string   `json:"periods"`
		Rows    [][]string `json:"rows"`
	}
	jsonBytes, err := result.Value.MarshalJSON()
	if err != nil {
		return nil, nil, fmt.Errorf("marshal table JSON: %w", err)
	}
	if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
		return nil, nil, fmt.Errorf("parse table data: %w", err)
	}

	return parsed.Periods, parsed.Rows, nil
}
