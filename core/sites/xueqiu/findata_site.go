package xueqiu

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	infra "comdigger/core/infra"
	"comdigger/core/sites"
)

// xueqiuTitleToField 雪球中文字段名 → 数据库字段名映射
// 与 comdig/field_mapping.go 中的 XueqiuTitleToField 内容一致
var xueqiuTitleToField = map[string]string{
	"扣除非经常性损益后的净利润":       "DEDUNETPROFIT",
	"子公司吸收少数股东投资收到的现金":    "GETSUBSIDIARYINVRECE",
	"取得子公司及其他营业单位支付的现金净额": "GETSUBSIDIARYPAY",
	"收到其他与筹资活动有关的现金":      "OTHERFINCASHIN",
	"收到其他与投资活动有关的现金":      "OTHERINVESTRECE",
	"支付其他与投资活动有关的现金":      "OTHERINVESTOUT",
	"发行债券收到的现金":           "ISSUEDBONDRECE",
	"子公司支付给少数股东的股利":       "SUBSIDIARYMINORITYPAY",
}

// reportTypeMap 报表类型映射：FinReportTable.Type → 数据库 report_type 字段值
var reportTypeMap = map[string]string{
	"Balance Sheet":       "fzb",
	"Income Statement":    "lrb",
	"Cash Flow Statement": "llb",
}

// XueqiuFinDataSite 实现 Site 接口，抓取雪球财报数据并存入数据库
type XueqiuFinDataSite struct{}

// Name 返回插件名称
func (s *XueqiuFinDataSite) Name() string {
	return "xueqiu.findata"
}

// Fetch 抓取雪球财报数据并存入数据库
func (s *XueqiuFinDataSite) Fetch(ctx context.Context, opts sites.FetchOptions) error {
	stockCode := opts.StockCode

	// 若 StockCode 为空，则创建 Browser 并调用 ResolveStockCode 搜索
	if stockCode == "" {
		b, err := New(Config{Headless: true, ProxyURL: opts.ProxyURL})
		if err != nil {
			return fmt.Errorf("创建浏览器失败: %w", err)
		}
		defer b.Close()

		page, err := b.NewPage()
		if err != nil {
			return fmt.Errorf("创建页面失败: %w", err)
		}
		defer page.Close()

		code, _, err := ResolveStockCode(opts.CompanyID, page)
		if err != nil {
			return fmt.Errorf("解析股票代码失败: %w", err)
		}
		stockCode = code
	}

	// 创建 Browser 和 FinReportClient
	b, err := New(Config{Headless: true, ProxyURL: opts.ProxyURL})
	if err != nil {
		return fmt.Errorf("创建浏览器失败: %w", err)
	}
	defer b.Close()

	client := NewFinReportClient(b)

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 2 * 60 * 1000000000 // 2 minutes in nanoseconds
	}

	// 抓取三张财报表
	tables, err := client.FetchAllReports(stockCode, timeout)
	if err != nil {
		return fmt.Errorf("抓取财报失败: %w", err)
	}

	// 遍历三张表
	for _, table := range tables {
		reportType, ok := reportTypeMap[table.Type]
		if !ok {
			infra.Logger.Warn("未知报表类型: %s", table.Type)
			continue
		}

		// 遍历每张表的所有行
		for _, row := range table.Rows {
			fieldName, ok := xueqiuTitleToField[row.Name]
			if !ok {
				// 不在映射表中的行跳过
				continue
			}

			// 遍历每个期间列
			for i, header := range table.Headers {
				if i >= len(row.Values) {
					break
				}

				// 解析期间列头
				reportDate := parseFinReportDate(header)
				if reportDate == "" {
					continue
				}

				// 解析数值
				rawVal := row.Values[i]
				if rawVal == "" || rawVal == "--" {
					continue
				}

				value, ok := parseFinReportValue(rawVal)
				if !ok {
					infra.Logger.Warn("无法解析雪球值: field=%s, date=%s, raw=%s", fieldName, reportDate, rawVal)
					continue
				}

				// 构造 ID
				dateForID := strings.ReplaceAll(reportDate, "-", "")
				recordID := fmt.Sprintf("%s_%s_%s", opts.CompanyID, dateForID, fieldName)

				// 只在不存在时 INSERT（不覆盖已有数据）
				inserted, err := insertFinDataIfNotExists(opts.DB, opts.CompanyID, reportDate, reportType, fieldName, recordID, value)
				if err != nil {
					infra.Logger.ErrorDetailed(fmt.Sprintf("保存雪球财务数据失败 [%s:%s:%s]", opts.CompanyID, reportDate, fieldName), err)
					continue
				}
				if inserted {
					infra.Logger.Info("补充雪球字段: %s %s %s = %v", opts.CompanyID, reportDate, fieldName, value)
				}
			}
		}
	}

	return nil
}

// insertFinDataIfNotExists 仅在记录不存在时插入，返回是否实际插入
func insertFinDataIfNotExists(db *sql.DB, companyID, reportDate, reportType, itemField, id string, value float64) (bool, error) {
	// 检查是否已存在
	var existingID string
	err := db.QueryRow(
		`SELECT id FROM fin WHERE company_id=$1 AND report_date=$2 AND item_field=$3`,
		companyID, reportDate, itemField,
	).Scan(&existingID)
	if err == nil {
		// 已存在，跳过
		return false, nil
	}
	if err != sql.ErrNoRows {
		return false, fmt.Errorf("查询fin记录失败: %w", err)
	}

	// 不存在则插入
	_, err = db.Exec(`
		INSERT INTO fin (
			id, company_id, report_date, report_type, item_field,
			item_value, item_display_type, item_group_no
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, id, companyID, reportDate, reportType, itemField, value, 0, 0)
	if err != nil {
		return false, fmt.Errorf("插入财务数据失败: %w", err)
	}
	return true, nil
}

// parseFinReportDate 解析雪球期间列头为 YYYY-MM-DD 格式
// 支持中文格式（go-rod 抓取的网页 DOM）：
//
//	"2024年报" → "2024-12-31"
//	"2024三季报" → "2024-09-30"
//	"2024中报" → "2024-06-30"
//	"2024一季报" → "2024-03-31"
//
// 支持英文格式（备用）：
//
//	"YYYY Q1" → "YYYY-03-31"
//	"YYYY Q2" / "YYYY H1" → "YYYY-06-30"
//	"YYYY Q3" → "YYYY-09-30"
//	"YYYY Q4" / "YYYY" → "YYYY-12-31"
func parseFinReportDate(s string) string {
	s = strings.TrimSpace(s)

	// 匹配中文格式：年报/三季报/中报/一季报
	reCN := regexp.MustCompile(`^(\d{4})(年报|三季报|中报|一季报)$`)
	if m := reCN.FindStringSubmatch(s); m != nil {
		year := m[1]
		switch m[2] {
		case "年报":
			return year + "-12-31"
		case "三季报":
			return year + "-09-30"
		case "中报":
			return year + "-06-30"
		case "一季报":
			return year + "-03-31"
		}
	}

	// 匹配英文格式 "YYYY Q1/Q2/Q3/Q4/H1"
	reQuarter := regexp.MustCompile(`^(\d{4})\s+(Q1|Q2|Q3|Q4|H1)$`)
	if m := reQuarter.FindStringSubmatch(s); m != nil {
		year := m[1]
		period := m[2]
		switch period {
		case "Q1":
			return year + "-03-31"
		case "Q2", "H1":
			return year + "-06-30"
		case "Q3":
			return year + "-09-30"
		case "Q4":
			return year + "-12-31"
		}
	}

	// 匹配纯年份 "YYYY" 格式
	reYear := regexp.MustCompile(`^(\d{4})$`)
	if m := reYear.FindStringSubmatch(s); m != nil {
		return m[1] + "-12-31"
	}

	infra.Logger.Warn("无法识别雪球日期格式: %s", s)
	return ""
}

// parseFinReportValue 解析雪球页面数值
// 支持中文单位格式（go-rod 抓取的 DOM）："-1.36亿+79.14%"、"7681.57万-30.45%"
// 支持英文单位格式（备用）："5.41B-32.56%"，B=亿=1e8元、M=百万=1e6元、K=千=1e3元
// 最终存储单位为元（与新浪数据一致）
func parseFinReportValue(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "--" {
		return 0, false
	}

	// 去掉尾部同比部分：[+-]\d[\d.]*%
	reTongbi := regexp.MustCompile(`[+\-]\d[\d.]*%$`)
	s = reTongbi.ReplaceAllString(s, "")

	// 去掉尾部孤立的 "-"
	s = strings.TrimRight(s, "-")
	s = strings.TrimSpace(s)

	if s == "" || s == "--" {
		return 0, false
	}

	// 去掉逗号
	s = strings.ReplaceAll(s, ",", "")

	// 解析中文单位（亿/万/元），数据库存储单位为元
	var multiplier float64 = 1
	if strings.HasSuffix(s, "亿") {
		multiplier = 1e8 // 亿 → 元：×1e8
		s = strings.TrimSuffix(s, "亿")
	} else if strings.HasSuffix(s, "万") {
		multiplier = 1e4 // 万 → 元：×1e4
		s = strings.TrimSuffix(s, "万")
	} else if strings.HasSuffix(s, "元") {
		multiplier = 1 // 元 → 元：×1
		s = strings.TrimSuffix(s, "元")
	} else if strings.HasSuffix(s, "B") {
		// 英文单位：B = 亿 = 1e8元
		multiplier = 1e8
		s = strings.TrimSuffix(s, "B")
	} else if strings.HasSuffix(s, "M") {
		// 英文单位：M = 百万 = 1e6元
		multiplier = 1e6
		s = strings.TrimSuffix(s, "M")
	} else if strings.HasSuffix(s, "K") {
		// 英文单位：K = 千 = 1e3元
		multiplier = 1e3
		s = strings.TrimSuffix(s, "K")
	}

	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}

	return val * multiplier, true
}

func init() {
	sites.Register(&XueqiuFinDataSite{})
}
