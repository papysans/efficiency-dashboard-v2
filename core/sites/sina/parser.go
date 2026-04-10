package sina

import (
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// ReportEntry 财报条目
type ReportEntry struct {
	Title  string
	URL    string
	Date   string
	Period string
}

// ParseReportList 解析新浪财报列表页 HTML，提取报告条目
// 查找 class 含 "datelist" 的容器元素下的所有 <a> 标签
func ParseReportList(htmlContent string) ([]ReportEntry, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, err
	}

	var entries []ReportEntry
	var inDatelist bool

	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode {
			// 检查是否进入 datelist 容器
			if hasClass(n, "datelist") {
				inDatelist = true
			}
			// 在 datelist 内提取 <a> 标签
			if inDatelist && n.Data == "a" {
				href := getAttr(n, "href")
				text := strings.TrimSpace(getTextContent(n))
				if href == "" || text == "" {
					// 跳过空链接
				} else {
					// 补全为绝对 URL
					if strings.HasPrefix(href, "/") {
						href = "https://vip.stock.finance.sina.com.cn" + href
					}
					// 解析日期和标题（格式："YYYY-MM-DD 标题文字"）
					date := ""
					title := text
					if len(text) > 10 && text[4] == '-' && text[7] == '-' {
						date = text[:10]
						title = strings.TrimSpace(text[10:])
					}
					entries = append(entries, ReportEntry{
						Title: title,
						URL:   href,
						Date:  date,
					})
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
		// 离开 datelist 容器
		if n.Type == html.ElementNode && hasClass(n, "datelist") {
			inDatelist = false
		}
	}
	traverse(doc)

	return entries, nil
}

// ParsePeriodFromTitle 从标题和报告类型 key 提取期间代码
// reportTypeKey: anna/h1/q1/q3
// 示例："深信服：2023年一季度报告" + "q1" → "2023q1"
func ParsePeriodFromTitle(title string, reportTypeKey string) string {
	re := regexp.MustCompile(`(\d{4})年`)
	m := re.FindStringSubmatch(title)
	if m == nil {
		return ""
	}
	return m[1] + reportTypeKey
}

// DeduplicateByPeriod 按 Period 去重，保留第一次出现的条目（新浪列表按日期降序，第一条为最新版本）
// Period 为空的条目直接跳过
func DeduplicateByPeriod(entries []ReportEntry) []ReportEntry {
	seen := map[string]bool{}
	var result []ReportEntry
	for _, e := range entries {
		if e.Period == "" {
			continue
		}
		if !seen[e.Period] {
			seen[e.Period] = true
			result = append(result, e)
		}
	}
	return result
}

// hasClass 检查节点是否包含指定 class
func hasClass(n *html.Node, class string) bool {
	for _, a := range n.Attr {
		if a.Key == "class" {
			for _, c := range strings.Fields(a.Val) {
				if c == class {
					return true
				}
			}
		}
	}
	return false
}

// getAttr 获取节点属性值
func getAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// getTextContent 递归获取节点的文本内容
func getTextContent(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		sb.WriteString(getTextContent(c))
	}
	return sb.String()
}
