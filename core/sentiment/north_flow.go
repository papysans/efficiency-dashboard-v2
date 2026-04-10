package sentiment

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"comdigger/core/httputil"
)

// NorthFlow 北向资金每日成交数据
// 注：2024年8月19日起，沪深港通不再每日披露买入/卖出/净买入明细，
// 仅披露每日成交总额；净买入数据改为季度披露。
type NorthFlow struct {
	TradeDate    string  `json:"trade_date"`
	DealAmtHgt   float64 `json:"deal_amt_hgt"`   // 沪股通成交总额（万元）
	DealNumHgt   int64   `json:"deal_num_hgt"`   // 沪股通成交笔数
	DealAmtSgt   float64 `json:"deal_amt_sgt"`   // 深股通成交总额（万元）
	DealNumSgt   int64   `json:"deal_num_sgt"`   // 深股通成交笔数
	DealAmtTotal float64 `json:"deal_amt_total"` // 北向合计成交额（万元）
	QuotaStatus  string  `json:"quota_status"`   // 额度状态（"额度充足"等）
}

// NorthFlowMin 北向资金分时数据（今日，全为0表示非交易时间或数据已停播）
type NorthFlowMin struct {
	TradeTime string  `json:"trade_time"`
	NetHgt    float64 `json:"net_hgt"` // 沪股通累计净流入（万元，注：2024年8月后此字段可能全为0）
	NetSgt    float64 `json:"net_sgt"` // 深股通累计净流入（万元）
	NetTgt    float64 `json:"net_tgt"` // 合计净流入（万元）
}

// FetchNorthFlowHistory 获取北向资金每日成交历史（东方财富）
// days: 获取最近N条记录
// 注：2024年8月19日起，BUY_AMT/SELL_AMT/NET_DEAL_AMT 字段均为 null，
// 仅 DEAL_AMT（成交总额）和 DEAL_NUM（成交笔数）有数据
func FetchNorthFlowHistory(days int) ([]NorthFlow, error) {
	if days <= 0 {
		days = 30
	}

	// 分别请求沪股通(001)和深股通(003)
	hgtData, err := fetchNorthFlowByType("001", days)
	if err != nil {
		return nil, fmt.Errorf("获取沪股通数据失败: %w", err)
	}
	sgtData, err := fetchNorthFlowByType("003", days)
	if err != nil {
		return nil, fmt.Errorf("获取深股通数据失败: %w", err)
	}

	// 按日期合并
	sgtMap := make(map[string]northDayItem)
	for date, item := range sgtData {
		sgtMap[date] = item
	}

	var result []NorthFlow
	for date, hgt := range hgtData {
		sgt := sgtMap[date]
		result = append(result, NorthFlow{
			TradeDate:    date,
			DealAmtHgt:   hgt.DealAmt,
			DealNumHgt:   hgt.DealNum,
			DealAmtSgt:   sgt.DealAmt,
			DealNumSgt:   sgt.DealNum,
			DealAmtTotal: hgt.DealAmt + sgt.DealAmt,
			QuotaStatus:  hgt.Quota,
		})
	}
	return result, nil
}

// northDayItem 内部合并用
type northDayItem struct {
	DealAmt float64
	DealNum int64
	Quota   string
}

// fetchNorthFlowByType 获取单一互联互通类型的每日成交数据
// mutualType: "001"=沪股通, "003"=深股通
func fetchNorthFlowByType(mutualType string, days int) (map[string]northDayItem, error) {
	// 双引号用 %22 URL 编码，避免 PowerShell 单引号字符串中的解析问题
	url := fmt.Sprintf(
		"https://datacenter-web.eastmoney.com/api/data/v1/get?reportName=RPT_MUTUAL_DEAL_HISTORY&columns=ALL&filter=(MUTUAL_TYPE=%%22%s%%22)&pageNumber=1&pageSize=%d&sortTypes=-1&sortColumns=TRADE_DATE",
		mutualType, days,
	)

	body, err := httputil.FetchURL(context.Background(), url, emHeaders())
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}

	var result struct {
		Result struct {
			Data []struct {
				TradeDate string   `json:"TRADE_DATE"`
				DealAmt   *float64 `json:"DEAL_AMT"`           // 成交总额（万元）
				DealNum   *int64   `json:"DEAL_NUM"`           // 成交笔数
				QuotaText string   `json:"QUOTA_BALANCE_TEXT"` // 额度状态
				// 以下字段2024年8月19日起均为null
				// NetDealAmt  *float64 `json:"NET_DEAL_AMT"`
				// BuyAmt      *float64 `json:"BUY_AMT"`
				// SellAmt     *float64 `json:"SELL_AMT"`
			} `json:"data"`
		} `json:"result"`
		Success bool `json:"success"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	data := make(map[string]northDayItem)
	for _, item := range result.Result.Data {
		date := item.TradeDate
		if len(date) > 10 {
			date = date[:10]
		}
		var dealAmt float64
		var dealNum int64
		if item.DealAmt != nil {
			dealAmt = *item.DealAmt
		}
		if item.DealNum != nil {
			dealNum = *item.DealNum
		}
		data[date] = northDayItem{
			DealAmt: dealAmt,
			DealNum: dealNum,
			Quota:   item.QuotaText,
		}
	}
	return data, nil
}

// FetchNorthFlowMin 获取北向资金今日分时数据（东方财富）
// 注：2024年8月19日后，分时净流入数据可能全为0（政策限制）
func FetchNorthFlowMin() ([]NorthFlowMin, error) {
	url := "https://push2.eastmoney.com/api/qt/kamt.rtmin/get?fields1=f1,f3&fields2=f51,f52,f54,f56"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://data.eastmoney.com/")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")

	resp, err := httputil.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取北向分时失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var result struct {
		Data struct {
			S2N     []string `json:"s2n"`     // 格式: "时间,沪股通(万元),深股通(万元),合计(万元)"
			S2NDate string   `json:"s2nDate"` // 数据日期 MM-DD
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析北向分时响应失败: %w", err)
	}

	var flows []NorthFlowMin
	for _, line := range result.Data.S2N {
		parts := strings.Split(line, ",")
		if len(parts) < 4 {
			continue
		}
		flows = append(flows, NorthFlowMin{
			TradeTime: parts[0],
			NetHgt:    parseToFloat64(parts[1]),
			NetSgt:    parseToFloat64(parts[2]),
			NetTgt:    parseToFloat64(parts[3]),
		})
	}
	return flows, nil
}

// SaveNorthFlowToDB 保存北向资金成交数据到数据库
func SaveNorthFlowToDB(db *sql.DB, data []NorthFlow) error {
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
			INSERT INTO sentiment_north_flow
			(trade_date, deal_amt_hgt, deal_num_hgt, deal_amt_sgt, deal_num_sgt, deal_amt_total, quota_status)
			VALUES ($1,$2,$3,$4,$5,$6,$7)
			ON CONFLICT ON CONSTRAINT sentiment_north_flow_unique DO UPDATE SET
				deal_amt_hgt=EXCLUDED.deal_amt_hgt, deal_num_hgt=EXCLUDED.deal_num_hgt,
				deal_amt_sgt=EXCLUDED.deal_amt_sgt, deal_num_sgt=EXCLUDED.deal_num_sgt,
				deal_amt_total=EXCLUDED.deal_amt_total, quota_status=EXCLUDED.quota_status
		`, item.TradeDate, item.DealAmtHgt, item.DealNumHgt,
			item.DealAmtSgt, item.DealNumSgt, item.DealAmtTotal, item.QuotaStatus)
		if err != nil {
			return fmt.Errorf("保存北向资金数据失败: %w", err)
		}
	}
	return tx.Commit()
}
