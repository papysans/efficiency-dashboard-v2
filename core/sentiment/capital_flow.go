package sentiment

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"comdigger/core/httputil"
)

// CapitalFlow 个股资金流向分时数据
type CapitalFlow struct {
	CompanyID      string `json:"company_id"`
	TradeTime      string `json:"trade_time"`
	MainNetInflow  int64  `json:"main_net_inflow"`  // 主力净流入（元）
	SuperNetInflow int64  `json:"super_net_inflow"` // 超大单净流入（元）
	BigNetInflow   int64  `json:"big_net_inflow"`   // 大单净流入（元）
	SmallNetInflow int64  `json:"small_net_inflow"` // 小单净流入（元）
}

// FetchCapitalFlow 获取个股分时资金流向（东方财富）
// companyID 格式：sz300454 / sh601318
func FetchCapitalFlow(companyID string) ([]CapitalFlow, error) {
	if len(companyID) < 8 {
		return nil, fmt.Errorf("companyID格式错误: %s，应为 sz300454 或 sh601318", companyID)
	}

	market := strings.ToLower(companyID[:2])
	code := companyID[2:]

	// 市场映射：sz→0, sh→1
	marketNum := "0"
	if market == "sh" {
		marketNum = "1"
	}
	secid := fmt.Sprintf("%s.%s", marketNum, code)

	url := fmt.Sprintf(
		"https://push2.eastmoney.com/api/qt/stock/fflow/kline/get?secid=%s&lmt=0&klt=1&fields1=f1,f2,f3,f7&fields2=f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61,f62,f63,f64,f65",
		secid,
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://data.eastmoney.com/")

	resp, err := httputil.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取资金流向失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var result struct {
		Data struct {
			Klines []string `json:"klines"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析资金流向响应失败: %w", err)
	}

	var flows []CapitalFlow
	for _, line := range result.Data.Klines {
		parts := strings.Split(line, ",")
		if len(parts) < 5 {
			continue
		}
		// 格式：时间,主力净流入,超大单净流入,大单净流入,小单净流入,...
		tradeTime := parts[0]
		mainNet := parseInt64(parts[1])
		superNet := parseInt64(parts[2])
		bigNet := parseInt64(parts[3])
		smallNet := parseInt64(parts[4])

		// 将浮点字符串转int64（原始数据可能带小数点）
		flows = append(flows, CapitalFlow{
			CompanyID:      companyID,
			TradeTime:      tradeTime,
			MainNetInflow:  mainNet,
			SuperNetInflow: superNet,
			BigNetInflow:   bigNet,
			SmallNetInflow: smallNet,
		})
	}
	return flows, nil
}

// SaveCapitalFlowToDB 保存资金流向数据到数据库
func SaveCapitalFlowToDB(db *sql.DB, data []CapitalFlow) error {
	if len(data) == 0 {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback()

	for _, item := range data {
		_, err := tx.Exec(`
			INSERT INTO sentiment_capital_flow
			(company_id, trade_time, main_net_inflow, super_net_inflow, big_net_inflow, small_net_inflow)
			VALUES ($1,$2,$3,$4,$5,$6)
			ON CONFLICT ON CONSTRAINT sentiment_capital_flow_unique DO UPDATE SET
				main_net_inflow=EXCLUDED.main_net_inflow,
				super_net_inflow=EXCLUDED.super_net_inflow,
				big_net_inflow=EXCLUDED.big_net_inflow,
				small_net_inflow=EXCLUDED.small_net_inflow
		`, item.CompanyID, item.TradeTime, item.MainNetInflow,
			item.SuperNetInflow, item.BigNetInflow, item.SmallNetInflow)
		if err != nil {
			return fmt.Errorf("保存资金流向数据失败: %w", err)
		}
	}
	return tx.Commit()
}

// parseInt64 将字符串（可能含小数）转为int64
func parseInt64(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return 0
	}
	// 先尝试直接解析int64
	if v, err := strconv.ParseInt(s, 10, 64); err == nil {
		return v
	}
	// 再尝试float64后转int64
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return int64(f)
	}
	return 0
}

// CapitalFlowDaily 个股每日资金流向历史
type CapitalFlowDaily struct {
	TradeDate      string
	MainNetInflow  int64 // 主力净流入（元）
	SuperNetInflow int64 // 超大单净流入（元）
	BigNetInflow   int64 // 大单净流入（元）
	SmallNetInflow int64 // 小单净流入（元）
}

// FetchCapitalFlowHistory 获取个股每日资金流向历史（东方财富）
func FetchCapitalFlowHistory(companyID string) ([]CapitalFlowDaily, error) {
	if len(companyID) < 8 {
		return nil, fmt.Errorf("companyID格式错误: %s，应为 sz300454 或 sh601318", companyID)
	}

	market := strings.ToLower(companyID[:2])
	code := companyID[2:]
	marketNum := "0"
	if market == "sh" {
		marketNum = "1"
	}

	url := fmt.Sprintf(
		"https://push2his.eastmoney.com/api/qt/stock/fflow/daykline/get?lmt=0&klt=101&fields1=f1,f2,f3,f7&fields2=f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61&secid=%s.%s",
		marketNum, code,
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://data.eastmoney.com/")

	resp, err := httputil.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取资金流向历史失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var result struct {
		Data struct {
			Klines []string `json:"klines"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("获取资金流向历史失败: %w", err)
	}

	var flows []CapitalFlowDaily
	for _, line := range result.Data.Klines {
		parts := strings.Split(line, ",")
		if len(parts) < 5 {
			continue
		}
		flows = append(flows, CapitalFlowDaily{
			TradeDate:      parts[0],
			MainNetInflow:  parseInt64(parts[1]),
			SuperNetInflow: parseInt64(parts[2]),
			BigNetInflow:   parseInt64(parts[3]),
			SmallNetInflow: parseInt64(parts[4]),
		})
	}
	return flows, nil
}

// SaveCapitalFlowDailyToDB 保存个股每日资金流向历史到数据库
func SaveCapitalFlowDailyToDB(db *sql.DB, companyID string, data []CapitalFlowDaily) error {
	if len(data) == 0 {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback()

	for _, item := range data {
		_, err := tx.Exec(`
			INSERT INTO sentiment_capital_flow_daily
			(company_id, trade_date, main_net_inflow, super_net_inflow, big_net_inflow, small_net_inflow)
			VALUES ($1,$2,$3,$4,$5,$6)
			ON CONFLICT ON CONSTRAINT sentiment_capital_flow_daily_unique DO UPDATE SET
				main_net_inflow=EXCLUDED.main_net_inflow,
				super_net_inflow=EXCLUDED.super_net_inflow,
				big_net_inflow=EXCLUDED.big_net_inflow,
				small_net_inflow=EXCLUDED.small_net_inflow
		`, companyID, item.TradeDate, item.MainNetInflow,
			item.SuperNetInflow, item.BigNetInflow, item.SmallNetInflow)
		if err != nil {
			return fmt.Errorf("保存每日资金流向失败: %w", err)
		}
	}
	return tx.Commit()
}
