package eastmoney

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"comdigger/core/infra"
	"comdigger/core/sites"
)

// EastMoneyFinDataSite 东方财富财务数据源
type EastMoneyFinDataSite struct{}

// Name 返回插件名称
func (s *EastMoneyFinDataSite) Name() string {
	return "eastmoney.findata"
}

// Fetch 抓取东方财富财务数据并存入数据库
func (s *EastMoneyFinDataSite) Fetch(ctx context.Context, opts sites.FetchOptions) error {
	if opts.StockCode == "" {
		return fmt.Errorf("StockCode 不能为空")
	}

	emCode, err := convertStockCode(opts.StockCode)
	if err != nil {
		return fmt.Errorf("转换股票代码失败: %w", err)
	}

	// 抓取利润表
	incomeRecords, err := FetchIncome(ctx, emCode)
	if err != nil {
		return fmt.Errorf("抓取利润表失败: %w", err)
	}
	processRecords(opts.DB, opts.CompanyID, "lrb", incomeRecords, incomeFieldMap)

	// 抓取资产负债表
	balanceRecords, err := FetchBalance(ctx, emCode)
	if err != nil {
		return fmt.Errorf("抓取资产负债表失败: %w", err)
	}
	processRecords(opts.DB, opts.CompanyID, "fzb", balanceRecords, balanceFieldMap)

	// 抓取现金流量表
	cashflowRecords, err := FetchCashflow(ctx, emCode)
	if err != nil {
		return fmt.Errorf("抓取现金流量表失败: %w", err)
	}
	processRecords(opts.DB, opts.CompanyID, "llb", cashflowRecords, cashflowFieldMap)

	return nil
}

// convertStockCode 将雪球格式 SZ300454 转为东方财富格式 300454.SZ
func convertStockCode(code string) (string, error) {
	if len(code) < 3 {
		return "", fmt.Errorf("股票代码格式不正确: %q，期望如 SZ300454", code)
	}

	prefix := code[:2]
	num := code[2:]
	upper := strings.ToUpper(prefix)

	if upper != "SZ" && upper != "SH" && upper != "BJ" {
		return "", fmt.Errorf("不支持的市场前缀: %q，期望 SZ/SH/BJ", prefix)
	}
	if len(num) == 0 {
		return "", fmt.Errorf("股票代码缺少数字部分: %q", code)
	}

	return num + "." + upper, nil
}

// processRecords 处理一张表的所有记录并入库
func processRecords(db *sql.DB, companyID, reportType string, records []map[string]interface{}, fieldMap map[string]string) {
	inserted := 0
	skipped := 0

	for _, record := range records {
		// 提取报告日期
		dateRaw, ok := record[reportDateField]
		if !ok {
			continue
		}
		dateStr, ok := dateRaw.(string)
		if !ok {
			continue
		}
		reportDate, err := parseReportDate(dateStr)
		if err != nil {
			infra.Logger.Warn("[eastmoney] 解析日期失败: %v", err)
			continue
		}

		dateForID := strings.ReplaceAll(reportDate, "-", "")

		for emField, dbField := range fieldMap {
			raw, ok := record[emField]
			if !ok || raw == nil {
				continue
			}

			value, ok := raw.(float64)
			if !ok {
				continue
			}

			// 检查同比字段
			var tongbi *float64
			if yoyRaw, ok := record[emField+yoySuffix]; ok && yoyRaw != nil {
				if yoyVal, ok := yoyRaw.(float64); ok {
					tongbi = &yoyVal
				}
			}

			recordID := fmt.Sprintf("%s_%s_%s", companyID, dateForID, dbField)

			ok2, err := insertIfNotExists(db, companyID, reportDate, reportType, dbField, recordID, value, tongbi)
			if err != nil {
				infra.Logger.Warn("[eastmoney] 入库失败 [%s:%s:%s]: %v", companyID, reportDate, dbField, err)
				continue
			}
			if ok2 {
				inserted++
			} else {
				skipped++
			}
		}
	}

	infra.Logger.Info("[eastmoney] %s %s: 新增%d条，跳过%d条", companyID, reportType, inserted, skipped)
}

// insertIfNotExists 仅在记录不存在时插入
func insertIfNotExists(db *sql.DB, companyID, reportDate, reportType, itemField, id string, value float64, tongbi *float64) (bool, error) {
	var existingID string
	err := db.QueryRow(
		`SELECT id FROM fin WHERE company_id=$1 AND report_date=$2 AND item_field=$3`,
		companyID, reportDate, itemField,
	).Scan(&existingID)
	if err == nil {
		return false, nil
	}
	if err != sql.ErrNoRows {
		return false, fmt.Errorf("查询fin记录失败: %w", err)
	}

	_, err = db.Exec(`
		INSERT INTO fin (
			id, company_id, report_date, report_type, item_field,
			item_value, item_display_type, item_group_no, item_tongbi
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, id, companyID, reportDate, reportType, itemField, value, 0, 0, tongbi)
	if err != nil {
		return false, fmt.Errorf("插入财务数据失败: %w", err)
	}
	return true, nil
}

func init() {
	sites.Register(&EastMoneyFinDataSite{})
}
