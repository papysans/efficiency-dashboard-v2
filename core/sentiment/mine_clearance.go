package sentiment

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"comdigger/core/httputil"
)

// MineClearance 个股扫雷避险结果
type MineClearance struct {
	StockCode string
	ShortName string
	Score     int
	FType     string
	SType     string
	TType     string
	Reason    string
}

// FetchMineClearance 获取个股扫雷数据（通达信）
// stockCode 为纯6位代码，如 "300454"
func FetchMineClearance(stockCode string) ([]MineClearance, error) {
	url := fmt.Sprintf("http://page3.tdx.com.cn:7615/site/pcwebcall_static/bxb/json/%s.json", stockCode)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := httputil.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取扫雷数据失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var raw struct {
		Name string `json:"name"`
		Data []struct {
			Name  string `json:"name"`
			SType string `json:"s_type"`
			Rows  []struct {
				Name       string          `json:"name"`
				Trig       int             `json:"trig"`
				Fs         int             `json:"fs"`
				CommonLxid json.RawMessage `json:"commonlxid"`
			} `json:"rows"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("解析扫雷数据失败: %w", err)
	}

	shortName := raw.Name
	score := 100
	deductedSTypes := make(map[string]bool)
	var items []MineClearance

	for _, cat := range raw.Data {
		fType := cat.Name
		sType := cat.SType

		for _, row := range cat.Rows {
			// 尝试解析 commonlxid 为子项数组
			var subItems []struct {
				Name   string `json:"name"`
				Trig   int    `json:"trig"`
				Fs     int    `json:"fs"`
				Reason string `json:"reason"`
			}

			hasSubItems := false
			if len(row.CommonLxid) > 0 {
				// 检查是否为非 null
				trimmed := strings.TrimSpace(string(row.CommonLxid))
				if trimmed != "null" && trimmed != "" {
					if err := json.Unmarshal(row.CommonLxid, &subItems); err == nil && len(subItems) > 0 {
						hasSubItems = true
					}
				}
			}

			if hasSubItems {
				for _, sub := range subItems {
					if sub.Trig == 1 {
						reason := sub.Reason
						if reason == "" {
							reason = sub.Name
						}
						items = append(items, MineClearance{
							StockCode: stockCode,
							ShortName: shortName,
							FType:     fType,
							SType:     sType,
							TType:     sub.Name,
							Reason:    reason,
						})
						if !deductedSTypes[sType] {
							score -= sub.Fs
							deductedSTypes[sType] = true
						}
					}
				}
			} else if row.Trig == 1 {
				reason := row.Name
				items = append(items, MineClearance{
					StockCode: stockCode,
					ShortName: shortName,
					FType:     fType,
					SType:     sType,
					TType:     row.Name,
					Reason:    reason,
				})
				if !deductedSTypes[sType] {
					score -= row.Fs
					deductedSTypes[sType] = true
				}
			}
		}
	}

	if score < 1 {
		score = 1
	}

	// 退市股判断
	if strings.HasSuffix(shortName, "退") && len(items) == 0 {
		return []MineClearance{{
			StockCode: stockCode,
			ShortName: shortName,
			Score:     -1,
		}}, nil
	}

	// 无风险项
	if len(items) == 0 {
		return []MineClearance{{
			StockCode: stockCode,
			ShortName: shortName,
			Score:     100,
			FType:     "暂无风险项",
		}}, nil
	}

	// 为所有结果设置 Score
	for i := range items {
		items[i].Score = score
	}

	return items, nil
}

// SaveMineClearanceToDB 保存扫雷数据到数据库
func SaveMineClearanceToDB(db *sql.DB, data []MineClearance) error {
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
			INSERT INTO sentiment_mine_clearance
			(stock_code, short_name, score, f_type, s_type, t_type, reason)
			VALUES ($1,$2,$3,$4,$5,$6,$7)
			ON CONFLICT ON CONSTRAINT sentiment_mine_clearance_unique DO UPDATE SET
				score=EXCLUDED.score, reason=EXCLUDED.reason, fetched_at=NOW()
		`, item.StockCode, item.ShortName, item.Score, item.FType, item.SType, item.TType, item.Reason)
		if err != nil {
			return fmt.Errorf("保存扫雷数据失败: %w", err)
		}
	}
	return tx.Commit()
}
