package sentiment

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"comdigger/core/httputil"
)

// Margin 融资融券历史数据
type Margin struct {
	TradeDate string  `json:"trade_date"`
	Rzye      float64 `json:"rzye"`     // 融资余额（万元）
	Rqye      float64 `json:"rqye"`     // 融券余额（万元）
	Rzrqye    float64 `json:"rzrqye"`   // 融资融券余额（万元）
	Rzrqyecz  float64 `json:"rzrqyecz"` // 融资融券余额差值（万元）
}

// FetchMargin 获取融资融券历史数据（东方财富）
// days: 获取最近N条记录
func FetchMargin(days int) ([]Margin, error) {
	if days <= 0 {
		days = 30
	}

	url := fmt.Sprintf(
		"https://datacenter-web.eastmoney.com/api/data/v1/get?reportName=RPTA_RZRQ_LSHJ&columns=ALL&pageSize=%d&pageNumber=1&sortTypes=-1&sortColumns=DIM_DATE",
		days,
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://data.eastmoney.com/")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	req.Header.Set("Origin", "https://data.eastmoney.com")

	resp, err := httputil.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取融资融券数据失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var result struct {
		Result struct {
			Data []struct {
				DimDate  string   `json:"DIM_DATE"`
				Rzye     *float64 `json:"RZYE"`
				Rqye     *float64 `json:"RQYE"`
				Rzrqye   *float64 `json:"RZRQYE"`
				Rzrqyecz *float64 `json:"RZRQYECZ"`
			} `json:"data"`
		} `json:"result"`
		Success bool `json:"success"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析融资融券响应失败: %w", err)
	}

	var margins []Margin
	for _, item := range result.Result.Data {
		date := item.DimDate
		if len(date) > 10 {
			date = date[:10]
		}
		// 原始单位为元，转换为万元
		margins = append(margins, Margin{
			TradeDate: date,
			Rzye:      derefFloat(item.Rzye) / 10000,
			Rqye:      derefFloat(item.Rqye) / 10000,
			Rzrqye:    derefFloat(item.Rzrqye) / 10000,
			Rzrqyecz:  derefFloat(item.Rzrqyecz) / 10000,
		})
	}
	return margins, nil
}

// SaveMarginToDB 保存融资融券数据到数据库
func SaveMarginToDB(db *sql.DB, data []Margin) error {
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
			INSERT INTO sentiment_margin
			(trade_date, rzye, rqye, rzrqye, rzrqyecz)
			VALUES ($1,$2,$3,$4,$5)
			ON CONFLICT ON CONSTRAINT sentiment_margin_unique DO UPDATE SET
				rzye=EXCLUDED.rzye, rqye=EXCLUDED.rqye,
				rzrqye=EXCLUDED.rzrqye, rzrqyecz=EXCLUDED.rzrqyecz
		`, item.TradeDate, item.Rzye, item.Rqye, item.Rzrqye, item.Rzrqyecz)
		if err != nil {
			return fmt.Errorf("保存融资融券数据失败: %w", err)
		}
	}
	return tx.Commit()
}
