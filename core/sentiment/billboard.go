package sentiment

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"comdigger/core/httputil"
)

// Billboard 龙虎榜数据
type Billboard struct {
	TradeDate        string  `json:"trade_date"`
	StockCode        string  `json:"stock_code"`
	StockName        string  `json:"stock_name"`
	ClosePrice       float64 `json:"close_price"`
	ChangeRate       float64 `json:"change_rate"`
	BillboardNetAmt  float64 `json:"billboard_net_amt"`
	BillboardBuyAmt  float64 `json:"billboard_buy_amt"`
	BillboardSellAmt float64 `json:"billboard_sell_amt"`
	DealAmountRatio  float64 `json:"deal_amount_ratio"`
	Explain          string  `json:"explain"`
	TradeMarket      string  `json:"trade_market"`
}

// FetchBillboard 获取龙虎榜数据（东方财富）
// date 格式 YYYY-MM-DD，为空时自动获取最近交易日
func FetchBillboard(date string) ([]Billboard, error) {
	if date == "" {
		var err error
		date, err = GetLastTradeDay()
		if err != nil {
			return nil, fmt.Errorf("获取最近交易日失败: %w", err)
		}
	}

	url := fmt.Sprintf(
		"https://datacenter-web.eastmoney.com/api/data/v1/get?reportName=RPT_DAILYBILLBOARD_DETAILSNEW&columns=ALL&filter=(TRADE_DATE<='%s')(TRADE_DATE>='%s')&pageNumber=1&pageSize=100&sortTypes=-1&sortColumns=CHANGE_RATE",
		date, date,
	)

	body, err := httputil.FetchURL(context.Background(), url, emHeaders())
	if err != nil {
		return nil, fmt.Errorf("获取龙虎榜失败: %w", err)
	}

	var result struct {
		Result struct {
			Data []struct {
				TradeDate        string   `json:"TRADE_DATE"`
				SecurityCode     string   `json:"SECURITY_CODE"`
				SecurityName     string   `json:"SECURITY_NAME_ABBR"`
				ClosePrice       *float64 `json:"CLOSE_PRICE"`
				ChangeRate       *float64 `json:"CHANGE_RATE"`
				BillboardNetAmt  *float64 `json:"BILLBOARD_NET_AMT"`
				BillboardBuyAmt  *float64 `json:"BILLBOARD_BUY_AMT"`
				BillboardSellAmt *float64 `json:"BILLBOARD_SELL_AMT"`
				DealAmountRatio  *float64 `json:"DEAL_AMOUNT_RATIO"`
				Explain          string   `json:"EXPLAIN"`
				TradeMarket      string   `json:"TRADE_MARKET"`
			} `json:"data"`
		} `json:"result"`
		Success bool `json:"success"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析龙虎榜响应失败: %w", err)
	}

	var boards []Billboard
	for _, item := range result.Result.Data {
		// 日期截取前10位 YYYY-MM-DD
		tradeDate := item.TradeDate
		if len(tradeDate) > 10 {
			tradeDate = tradeDate[:10]
		}
		boards = append(boards, Billboard{
			TradeDate:        tradeDate,
			StockCode:        item.SecurityCode,
			StockName:        item.SecurityName,
			ClosePrice:       derefFloat(item.ClosePrice),
			ChangeRate:       derefFloat(item.ChangeRate),
			BillboardNetAmt:  derefFloat(item.BillboardNetAmt),
			BillboardBuyAmt:  derefFloat(item.BillboardBuyAmt),
			BillboardSellAmt: derefFloat(item.BillboardSellAmt),
			DealAmountRatio:  derefFloat(item.DealAmountRatio),
			Explain:          item.Explain,
			TradeMarket:      item.TradeMarket,
		})
	}
	return boards, nil
}

// SaveBillboardToDB 保存龙虎榜数据到数据库
func SaveBillboardToDB(db *sql.DB, data []Billboard) error {
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
			INSERT INTO sentiment_billboard
			(trade_date, stock_code, stock_name, close_price, change_rate,
			 billboard_net_amt, billboard_buy_amt, billboard_sell_amt, deal_amount_ratio, explain_text, trade_market)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
			ON CONFLICT ON CONSTRAINT sentiment_billboard_unique DO UPDATE SET
				stock_name=EXCLUDED.stock_name, close_price=EXCLUDED.close_price,
				change_rate=EXCLUDED.change_rate, billboard_net_amt=EXCLUDED.billboard_net_amt,
				billboard_buy_amt=EXCLUDED.billboard_buy_amt, billboard_sell_amt=EXCLUDED.billboard_sell_amt,
				deal_amount_ratio=EXCLUDED.deal_amount_ratio, explain_text=EXCLUDED.explain_text,
				trade_market=EXCLUDED.trade_market
		`, item.TradeDate, item.StockCode, item.StockName, item.ClosePrice, item.ChangeRate,
			item.BillboardNetAmt, item.BillboardBuyAmt, item.BillboardSellAmt,
			item.DealAmountRatio, item.Explain, item.TradeMarket)
		if err != nil {
			return fmt.Errorf("保存龙虎榜数据失败: %w", err)
		}
	}
	return tx.Commit()
}

// derefFloat 安全解引用*float64，nil时返回0
func derefFloat(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

// BillboardDetail 龙虎榜营业部明细
type BillboardDetail struct {
	TradeDate   string
	StockCode   string
	Side        string // "buy" 或 "sell"
	OperateCode string
	OperateName string
	BuyAmt      float64 // 买入额（元）
	SellAmt     float64 // 卖出额（元）
	NetAmt      float64 // 净额（元）
	BuyAmtRate  float64 // 买入额占当日总成交额比例
	SellAmtRate float64 // 卖出额占当日总成交额比例
	Reason      string
}

// FetchBillboardDetail 获取龙虎榜营业部明细（东方财富）
func FetchBillboardDetail(date, stockCode string) ([]BillboardDetail, error) {
	type apiItem struct {
		TradeDate    string   `json:"TRADE_DATE"`
		SecurityCode string   `json:"SECURITY_CODE"`
		OperateCode  string   `json:"OPERATEDEPT_CODE"`
		OperateName  string   `json:"OPERATEDEPT_NAME"`
		Buy          *float64 `json:"BUY"`
		Sell         *float64 `json:"SELL"`
		Net          *float64 `json:"NET"`
		TotalBuyRio  *float64 `json:"TOTAL_BUYRIO"`
		TotalSellRio *float64 `json:"TOTAL_SELLRIO"`
		Explanation  string   `json:"EXPLANATION"`
	}
	type apiResp struct {
		Result struct {
			Data []apiItem `json:"data"`
		} `json:"result"`
		Success bool `json:"success"`
	}

	fetchSide := func(reportName, side string) ([]BillboardDetail, error) {
		url := fmt.Sprintf(
			"https://datacenter-web.eastmoney.com/api/data/v1/get?reportName=%s&columns=ALL&filter=(TRADE_DATE='%s')(SECURITY_CODE=%%22%s%%22)&pageNumber=1&pageSize=50&sortTypes=-1&sortColumns=BUY&source=WEB&client=WEB",
			reportName, date, stockCode,
		)
		body, err := httputil.FetchURL(context.Background(), url, emHeaders())
		if err != nil {
			return nil, fmt.Errorf("获取营业部明细(%s)失败: %w", side, err)
		}
		var resp apiResp
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("解析营业部明细(%s)响应失败: %w", side, err)
		}
		var result []BillboardDetail
		for _, item := range resp.Result.Data {
			tradeDate := item.TradeDate
			if len(tradeDate) > 10 {
				tradeDate = tradeDate[:10]
			}
			result = append(result, BillboardDetail{
				TradeDate:   tradeDate,
				StockCode:   item.SecurityCode,
				Side:        side,
				OperateCode: item.OperateCode,
				OperateName: item.OperateName,
				BuyAmt:      derefFloat(item.Buy),
				SellAmt:     derefFloat(item.Sell),
				NetAmt:      derefFloat(item.Net),
				BuyAmtRate:  derefFloat(item.TotalBuyRio),
				SellAmtRate: derefFloat(item.TotalSellRio),
				Reason:      item.Explanation,
			})
		}
		return result, nil
	}

	buyDetails, err := fetchSide("RPT_BILLBOARD_DAILYDETAILSBUY", "buy")
	if err != nil {
		return nil, err
	}
	sellDetails, err := fetchSide("RPT_BILLBOARD_DAILYDETAILSSELL", "sell")
	if err != nil {
		return nil, err
	}
	return append(buyDetails, sellDetails...), nil
}

// SaveBillboardDetailToDB 保存龙虎榜营业部明细到数据库
func SaveBillboardDetailToDB(db *sql.DB, data []BillboardDetail) error {
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
			INSERT INTO sentiment_billboard_detail
			(trade_date, stock_code, side, operate_code, operate_name, buy_amt, sell_amt, net_amt, buy_amt_rate, sell_amt_rate, reason)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
			ON CONFLICT ON CONSTRAINT sentiment_billboard_detail_unique DO UPDATE SET
				buy_amt=EXCLUDED.buy_amt, sell_amt=EXCLUDED.sell_amt, net_amt=EXCLUDED.net_amt,
				buy_amt_rate=EXCLUDED.buy_amt_rate, sell_amt_rate=EXCLUDED.sell_amt_rate, reason=EXCLUDED.reason
		`, item.TradeDate, item.StockCode, item.Side, item.OperateCode, item.OperateName,
			item.BuyAmt, item.SellAmt, item.NetAmt, item.BuyAmtRate, item.SellAmtRate, item.Reason)
		if err != nil {
			return fmt.Errorf("保存营业部明细失败: %w", err)
		}
	}
	return tx.Commit()
}
