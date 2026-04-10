package xueqiu

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/go-rod/rod"
)

// ResolveStockCode resolves stock code (package-level function, accepts page parameter)
// Rules:
// 1. SZ/SH/HK prefix + number → return directly in uppercase
// 2. 6-digit pure number → add SH/SZ prefix based on first digit
// 3. 2-5 digit pure number → pad to 5 digits
// 4. Others → call searchStockByPage to search
func ResolveStockCode(query string, page *rod.Page) (code, name string, err error) {
	query = strings.TrimSpace(query)
	upper := strings.ToUpper(query)

	if regexp.MustCompile(`^(SZ|SH|HK)\d+$`).MatchString(upper) {
		return upper, "", nil
	}

	if regexp.MustCompile(`^\d{6}$`).MatchString(query) {
		prefix := "SH"
		ch := query[0]
		if ch == '0' || ch == '3' || ch == '1' {
			prefix = "SZ"
		}
		return prefix + query, "", nil
	}

	if regexp.MustCompile(`^\d{2,5}$`).MatchString(query) {
		padded := fmt.Sprintf("%05s", query)
		return padded, "", nil
	}

	return searchStockByPage(page, query)
}

// searchStockByPage searches stock code by page (private function)
func searchStockByPage(page *rod.Page, query string) (string, string, error) {
	searchURL := "https://xueqiu.com/k?q=" + url.QueryEscape(query)

	if err := page.Timeout(20 * time.Second).Navigate(searchURL); err != nil {
		return "", "", fmt.Errorf("failed to navigate to search: %w", err)
	}
	time.Sleep(4 * time.Second)

	result, err := page.Timeout(10 * time.Second).Eval(`() => {
        const items = [];
        document.querySelectorAll('table tr a, .stock-item a').forEach(function(a) {
            const href = a.href || '';
            const match = href.match(/\/S\/([A-Z0-9]+)$/);
            if (match) {
                const row = a.closest('tr') || a.closest('.stock-item');
                items.push({code: match[1], name: row ? row.innerText.substring(0,50) : a.innerText});
            }
        });
        return items;
    }`)
	if err != nil {
		return "", "", fmt.Errorf("failed to extract search results: %w", err)
	}

	type stockItem struct {
		Code string `json:"code"`
		Name string `json:"name"`
	}
	var items []stockItem
	jsonBytes, err2 := result.Value.MarshalJSON()
	if err2 != nil {
		return "", "", fmt.Errorf("no stock found for: %s", query)
	}
	if err2 = json.Unmarshal(jsonBytes, &items); err2 != nil || len(items) == 0 {
		return "", "", fmt.Errorf("no stock found for: %s", query)
	}

	first := items[0]
	name := strings.Fields(first.Name)
	stockName := ""
	if len(name) > 0 {
		stockName = name[0]
	}
	return first.Code, stockName, nil
}
