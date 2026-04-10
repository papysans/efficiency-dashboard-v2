package sentiment

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"comdigger/core/httputil"
)

// BondQuote 可转债实时行情
type BondQuote struct {
	BondCode  string  `json:"bond_code"`
	BondName  string  `json:"bond_name"`
	Price     float64 `json:"price"`      // 当前价（元）
	ChangeVal float64 `json:"change_val"` // 涨跌额
	ChangePct float64 `json:"change_pct"` // 涨跌幅(%)
	Volume    int64   `json:"volume"`     // 成交量（手）
	Amount    float64 `json:"amount"`     // 成交额（元）
	PreClose  float64 `json:"pre_close"`  // 昨收价
	TradeTime string  `json:"trade_time"`
}

// FetchBondQuote 获取可转债实时行情（新浪）
// 自动分页获取所有可转债数据
func FetchBondQuote() ([]BondQuote, error) {
	var allBonds []BondQuote
	page := 1
	pageSize := 80

	for {
		bonds, err := fetchBondQuotePage(page, pageSize)
		if err != nil {
			return nil, err
		}
		if len(bonds) == 0 {
			break
		}
		allBonds = append(allBonds, bonds...)
		if len(bonds) < pageSize {
			break
		}
		page++
		// 防止过度请求
		if page > 20 {
			break
		}
	}
	return allBonds, nil
}

// fetchBondQuotePage 获取单页可转债行情
func fetchBondQuotePage(page, num int) ([]BondQuote, error) {
	url := fmt.Sprintf(
		"https://vip.stock.finance.sina.com.cn/quotes_service/api/json_v2.php/Market_Center.getHQNodeDataSimple?page=%d&num=%d&sort=symbol&asc=1&node=hskzz_z",
		page, num,
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://finance.sina.com.cn/")

	resp, err := httputil.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取可转债行情失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 新浪返回格式：[{"symbol":"sz128xxx","name":"xxx","trade":"xxx",...}, ...]
	// trade/pricechange/changepercent/settlement 为字符串，volume/amount 为数字
	var rawItems []struct {
		Symbol        string  `json:"symbol"`
		Name          string  `json:"name"`
		Trade         string  `json:"trade"`         // 当前价（字符串）
		PriceChange   string  `json:"pricechange"`   // 涨跌额（字符串）
		ChangePercent string  `json:"changepercent"` // 涨跌幅（字符串）
		Volume        float64 `json:"volume"`        // 成交量（数字）
		Amount        float64 `json:"amount"`        // 成交额（数字）
		Settlement    string  `json:"settlement"`    // 昨收价（字符串）
	}

	if err := json.Unmarshal(body, &rawItems); err != nil {
		return nil, fmt.Errorf("解析可转债行情响应失败: %w", err)
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	var bonds []BondQuote
	for _, item := range rawItems {
		bonds = append(bonds, BondQuote{
			BondCode:  item.Symbol,
			BondName:  item.Name,
			Price:     parseToFloat64(item.Trade),
			ChangeVal: parseToFloat64(item.PriceChange),
			ChangePct: parseToFloat64(item.ChangePercent),
			Volume:    int64(item.Volume),
			Amount:    item.Amount,
			PreClose:  parseToFloat64(item.Settlement),
			TradeTime: now,
		})
	}
	return bonds, nil
}

// SaveBondQuoteToDB 保存可转债行情到数据库
func SaveBondQuoteToDB(db *sql.DB, data []BondQuote) error {
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
			INSERT INTO sentiment_bond_quote
			(bond_code, bond_name, price, change_val, change_pct, volume, amount, pre_close, trade_time)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			ON CONFLICT ON CONSTRAINT sentiment_bond_quote_unique DO UPDATE SET
				bond_name=EXCLUDED.bond_name, price=EXCLUDED.price,
				change_val=EXCLUDED.change_val, change_pct=EXCLUDED.change_pct,
				volume=EXCLUDED.volume, amount=EXCLUDED.amount,
				pre_close=EXCLUDED.pre_close
		`, item.BondCode, item.BondName, item.Price, item.ChangeVal, item.ChangePct,
			item.Volume, item.Amount, item.PreClose, item.TradeTime)
		if err != nil {
			return fmt.Errorf("保存可转债行情失败: %w", err)
		}
	}
	return tx.Commit()
}
