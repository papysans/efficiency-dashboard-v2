package sina

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// ErrOnlyAttachment 表示财报页面仅包含附件下载链接，无正文内容
var ErrOnlyAttachment = errors.New("仅附件，无正文内容")

const defaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// ReportType 财报类型定义
type ReportType struct {
	Key      string // anna/h1/q1/q3
	PageType string // ndbg/zqbg/yjdbg/sjdbg
	Name     string // 中文名
	PathName string // 新浪 URL 路径段
}

// allReportTypes 四种财报类型
// PathName 来自用户提供的原始 URL：
//
//	年报：  vCB_Bulletin      page_type=ndbg
//	中报：  vCB_BulletinZhong page_type=zqbg
//	一季报：vCB_BulletinYi    page_type=yjdbg
//	三季报：vCB_BulletinSan   page_type=sjdbg
var allReportTypes = []ReportType{
	{Key: "anna", PageType: "ndbg", Name: "年报", PathName: "Bulletin"},
	{Key: "h1", PageType: "zqbg", Name: "半年报", PathName: "BulletinZhong"},
	{Key: "q1", PageType: "yjdbg", Name: "一季报", PathName: "BulletinYi"},
	{Key: "q3", PageType: "sjdbg", Name: "三季报", PathName: "BulletinSan"},
}

// FinReportClient 新浪财报浏览器客户端
// 复用单个 page 串行访问，避免多 page 并发时的状态干扰
type FinReportClient struct {
	browser  *rod.Browser
	launcher *launcher.Launcher
	page     *rod.Page
	timeout  time.Duration
}

// NewFinReportClient 创建新浪财报客户端（启动 headless Chrome）
func NewFinReportClient(timeout time.Duration) (*FinReportClient, error) {
	l := launcher.New().Headless(true)
	u, err := l.Launch()
	if err != nil {
		l.Kill()
		return nil, fmt.Errorf("启动浏览器失败: %w", err)
	}
	browser := rod.New().ControlURL(u)
	if err = browser.Connect(); err != nil {
		l.Kill()
		return nil, fmt.Errorf("连接浏览器失败: %w", err)
	}

	page, err := browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		browser.Close()
		l.Kill()
		return nil, fmt.Errorf("创建页面失败: %w", err)
	}
	_ = page.SetUserAgent(&proto.NetworkSetUserAgentOverride{
		UserAgent: defaultUserAgent,
	})
	// 在页面任何脚本执行前注入，隐藏 webdriver 标志（与 durl browser.go 一致）
	_, _ = page.EvalOnNewDocument(`Object.defineProperty(navigator, 'webdriver', {get: () => undefined});`)

	return &FinReportClient{
		browser:  browser,
		launcher: l,
		page:     page,
		timeout:  timeout,
	}, nil
}

// Close 释放浏览器资源
func (c *FinReportClient) Close() {
	if c.page != nil {
		_ = c.page.Close()
	}
	if c.browser != nil {
		_ = c.browser.Close()
	}
	if c.launcher != nil {
		c.launcher.Kill()
	}
}

// navigateAndWait 导航到 URL 并等待页面完全加载
func (c *FinReportClient) navigateAndWait(url string) error {
	if err := c.page.Timeout(c.timeout).Navigate(url); err != nil {
		return fmt.Errorf("导航失败: %w", err)
	}
	if err := c.page.Timeout(c.timeout).WaitLoad(); err != nil {
		return fmt.Errorf("等待页面加载失败: %w", err)
	}
	// 额外等待，确保页面内容完全渲染
	time.Sleep(2 * time.Second)
	return nil
}

// evalJSString 执行 JS（JS 必须返回 string 类型），返回 Go string
// go-rod MarshalJSON 对 JS string 返回 "\"...\""，Unmarshal 去掉外层引号
func (c *FinReportClient) evalJSString(js string) (string, error) {
	result, err := c.page.Eval(js)
	if err != nil {
		return "", err
	}
	b, _ := result.Value.MarshalJSON()
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return "", fmt.Errorf("解析 JS string 失败: %w (raw: %s)", err, string(b))
	}
	return s, nil
}

// FetchListPage 获取财报列表页，返回去重后的报告条目
// 用 JS 在浏览器内提取结构化数据，避免 GBK outerHTML 乱码问题
func (c *FinReportClient) FetchListPage(stockCode string, rt ReportType) ([]ReportEntry, error) {
	url := fmt.Sprintf(
		"https://vip.stock.finance.sina.com.cn/corp/go.php/vCB_%s/stockid/%s/page_type/%s.phtml",
		rt.PathName, stockCode, rt.PageType,
	)

	if err := c.navigateAndWait(url); err != nil {
		return nil, fmt.Errorf("加载%s列表页失败: %w", rt.Name, err)
	}

	// 提取 datelist 内所有 <a> 的 href 和文本
	// 浏览器内部 JS 字符串是 UTF-16，能正确处理中文，无 GBK 乱码问题
	jsonStr, err := c.evalJSString(`() => {
		const container = document.querySelector('.datelist');
		if (!container) return '[]';
		const links = Array.from(container.querySelectorAll('a'));
		const items = links.map(a => ({
			href: a.getAttribute('href') || '',
			text: a.textContent.trim()
		})).filter(item => item.href && item.text);
		return JSON.stringify(items);
	}`)
	if err != nil {
		return nil, fmt.Errorf("提取%s列表失败: %w", rt.Name, err)
	}

	var items []struct {
		Href string `json:"href"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &items); err != nil {
		return nil, fmt.Errorf("解析%s列表 JSON 失败: %w", rt.Name, err)
	}

	// 提取日期（从 <a> 前的文本节点）
	dateJsonStr, _ := c.evalJSString(`() => {
		const container = document.querySelector('.datelist');
		if (!container) return '[]';
		const dates = [];
		container.querySelectorAll('a').forEach(a => {
			let prev = a.previousSibling;
			let dateStr = '';
			while (prev) {
				if (prev.nodeType === 3) {
					const t = prev.textContent.replace(/\u00a0/g, ' ').trim();
					if (/^\d{4}-\d{2}-\d{2}$/.test(t)) {
						dateStr = t;
						break;
					}
				}
				prev = prev.previousSibling;
			}
			dates.push(dateStr);
		});
		return JSON.stringify(dates);
	}`)
	var dates []string
	json.Unmarshal([]byte(dateJsonStr), &dates)

	const sinaBase = "https://vip.stock.finance.sina.com.cn"
	var entries []ReportEntry
	for i, item := range items {
		href := item.Href
		if strings.HasPrefix(href, "/") {
			href = sinaBase + href
		}
		date := ""
		if i < len(dates) {
			date = dates[i]
		}
		period := ParsePeriodFromTitle(item.Text, rt.Key)
		entries = append(entries, ReportEntry{
			Title:  item.Text,
			URL:    href,
			Date:   date,
			Period: period,
		})
	}

	return DeduplicateByPeriod(entries), nil
}

// FetchReportContent 下载财报详情页，提取 #content 区域并转为 Markdown
// 同时返回原始 innerHTML，供调用方保存为 .html 文件
func (c *FinReportClient) FetchReportContent(reportURL string) (markdown, innerHTML string, err error) {
	if err = c.navigateAndWait(reportURL); err != nil {
		return "", "", fmt.Errorf("加载财报详情页失败: %w", err)
	}

	// 用 JS 提取 #content 的 innerHTML（浏览器内部正确处理中文编码）
	innerHTML, err = c.evalJSString(`() => {
		const el = document.querySelector('#content');
		return el ? el.innerHTML : '';
	}`)
	if err != nil {
		return "", "", fmt.Errorf("提取 #content 失败: %w", err)
	}

	if innerHTML == "" {
		return "", "", fmt.Errorf("未找到 #content 元素或内容为空")
	}

	// 检测无效内容：内容过短或仅包含附件说明，无需继续处理
	if len(innerHTML) < 500 || strings.Contains(innerHTML, "公告内容详见附件") {
		return "", "", ErrOnlyAttachment
	}

	// 策略：先用占位符替换所有 <table>，html-to-markdown 处理其余 HTML（段落/标题等），
	// 最后把占位符替换回原始 HTML <table>（Markdown 渲染器支持内嵌 HTML，
	// 可完整保留 colspan/rowspan 等属性）
	processed, tables := extractTables(innerHTML)

	converter := md.NewConverter("", true, nil)
	markdown, err = converter.ConvertString(processed)
	if err != nil {
		return "", "", fmt.Errorf("HTML 转 Markdown 失败: %w", err)
	}

	// 将占位符替换回 HTML 表格
	// html-to-markdown 会对 _ 进行转义，需同时处理转义后的形式
	// 必须倒序替换：MDTABLE_PLACEHOLDER_1 是 MDTABLE_PLACEHOLDER_10 的子串，
	// 若正序替换，_1 会提前错误匹配 _10/_11/.../_19/_100/... 等，导致内容错位
	for i := len(tables) - 1; i >= 0; i-- {
		plain := fmt.Sprintf("MDTABLE_PLACEHOLDER_%d", i)
		escaped := fmt.Sprintf("MDTABLE\\_PLACEHOLDER\\_%d", i)
		markdown = strings.ReplaceAll(markdown, escaped, "\n"+tables[i])
		markdown = strings.ReplaceAll(markdown, plain, "\n"+tables[i])
	}

	return markdown, innerHTML, nil
}

// extractTables 将 HTML 中所有 <table> 替换为占位符，同时返回对应的 HTML 表格字符串列表。
// 表格以原始 HTML 形式保留（含 colspan/rowspan），Markdown 渲染器支持内嵌 HTML 表格。
func extractTables(htmlContent string) (string, []string) {
	// 预处理：合并相邻的同列数表格（由 PDF 截图切割导致的分裂表格）
	htmlContent = mergeAdjacentSameColTables(htmlContent)

	re := regexp.MustCompile(`(?is)<table\b[^>]*>.*?</table>`)
	var tables []string
	result := re.ReplaceAllStringFunc(htmlContent, func(tableHTML string) string {
		idx := len(tables)
		tables = append(tables, cleanTableHTML(tableHTML))
		// 用纯文本占位符替换 table，html-to-markdown 会将其原样保留
		return fmt.Sprintf("<p>MDTABLE_PLACEHOLDER_%d</p>", idx)
	})
	return result, tables
}

// cleanTableHTML 清理 HTML 表格：去掉 width/style/border 等排版属性，保留结构和内容。
// 保留 colspan/rowspan，使 Markdown 渲染器能正确显示跨列/跨行单元格。
func cleanTableHTML(tableHTML string) string {
	// 去掉 width、style、align、border 等纯排版属性，保留 colspan/rowspan
	attrRe := regexp.MustCompile(`(?i)\s+(width|style|align|valign|bgcolor|cellpadding|cellspacing|height|border)\s*=\s*("[^"]*"|'[^']*'|\S+)`)
	cleaned := attrRe.ReplaceAllString(tableHTML, "")
	// 统一加上 border 样式，让表格在 Markdown 预览中有边框线
	cleaned = strings.Replace(cleaned, "<table", `<table border="1" style="border-collapse:collapse"`, 1)
	return cleaned
}

// mergeAdjacentSameColTables 预处理 HTML：将相邻且列数完全相同的 <table> 合并为一个。
//
// 背景：新浪财报 HTML 由 PDF 截图转换，同一逻辑表格可能被切割成多个连续的
// <table>...</table> 块，中间没有任何文字内容。
func mergeAdjacentSameColTables(htmlContent string) string {
	// 匹配两个相邻表格之间的"缝合点"：
	//   </tr></tbody></table>  （可含空白、可选的 </div>、可选的 <div class="table-wrap">）  <table...><tbody>
	// 重要：两个表格之间不能有 <p> 等实质内容标签，只能有空白和div包裹
	sepRe := regexp.MustCompile(`(?is)</tr>\s*</tbody>\s*</table>\s*(?:</div>\s*){0,3}(?:<div[^>]*class="table-wrap"[^>]*>\s*)?<table[^>]*>\s*<tbody>`)

	// firstTrColCount 提取一段 HTML 中第一个 <tr> 的实际列数（考虑 colspan）
	firstTrRe := regexp.MustCompile(`(?is)<tr\b[^>]*>(.*?)</tr>`)
	cellRe := regexp.MustCompile(`(?i)<t[dh]\b[^>]*>`)
	colspanRe := regexp.MustCompile(`(?i)colspan\s*=\s*["']?(\d+)["']?`)

	firstTrColCount := func(html string) int {
		m := firstTrRe.FindStringSubmatch(html)
		if m == nil {
			return -1
		}
		trContent := m[1]
		cells := cellRe.FindAllString(trContent, -1)
		colCount := 0
		for _, cell := range cells {
			colspan := 1
			if match := colspanRe.FindStringSubmatch(cell); len(match) > 1 {
				if n, err := fmt.Sscanf(match[1], "%d", &colspan); err == nil && n == 1 && colspan > 0 {
				} else {
					colspan = 1
				}
			}
			colCount += colspan
		}
		return colCount
	}

	// 反复合并，直到一轮下来没有任何替换
	for {
		locs := sepRe.FindAllStringIndex(htmlContent, -1)
		if len(locs) == 0 {
			break
		}

		replaced := false
		for _, loc := range locs {
			sepStart, sepEnd := loc[0], loc[1]

			leftPart := htmlContent[:sepStart]
			rightPart := htmlContent[sepEnd:]

			// 计算左侧最后一个表格的第一行列数
			// 需要找到 leftPart 中最后一个 <table 的位置
			leftTableStart := strings.LastIndex(leftPart, "<table")
			leftColCount := -1
			if leftTableStart >= 0 {
				leftColCount = firstTrColCount(leftPart[leftTableStart:])
			}

			// 计算右侧第一个表格的第一行列数
			rightColCount := firstTrColCount(rightPart)

			// 检查分隔符之间是否有文字内容
			separatorContent := htmlContent[sepStart:sepEnd]
			textRe := regexp.MustCompile(`<[^>]+>`)
			separatorText := strings.TrimSpace(textRe.ReplaceAllString(separatorContent, ""))
			hasTextBetween := separatorText != ""

			// 唯一合并条件：精确列数完全相同且无文字
			if leftColCount == rightColCount && leftColCount > 0 && !hasTextBetween {
				htmlContent = htmlContent[:sepStart] + htmlContent[sepEnd:]
				replaced = true
				break
			}
		}

		if !replaced {
			break
		}
	}

	return htmlContent
}
