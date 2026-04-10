// Package sentiment 提供市场情绪与舆情数据采集
package sentiment

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"comdigger/core/httputil"
)

// HotStock 热股榜数据
type HotStock struct {
	Rank         int      `json:"rank"`
	StockCode    string   `json:"stock_code"`
	ShortName    string   `json:"short_name"`
	ChangePct    float64  `json:"change_pct"`
	HotValue     int64    `json:"hot_value"`
	PopTag       string   `json:"pop_tag"`
	ConceptTags  []string `json:"concept_tags"`
	Analyse      string   `json:"analyse"`
	AnalyseTitle string   `json:"analyse_title"`
	TradeDate    string   `json:"trade_date"`
}

// HotConcept 热门概念榜数据
type HotConcept struct {
	Rank        int     `json:"rank"`
	ConceptCode string  `json:"concept_code"`
	ConceptName string  `json:"concept_name"`
	ChangePct   float64 `json:"change_pct"`
	HotValue    int64   `json:"hot_value"`
	HotTag      string  `json:"hot_tag"`
	EtfName     string  `json:"etf_name"`
	TradeDate   string  `json:"trade_date"`
}

// FetchHotStock 获取同花顺热股100强（含AI分析原因）
func FetchHotStock() ([]HotStock, error) {
	url := "https://dq.10jqka.com.cn/fuyao/hot_list_data/out/hot_list/v1/stock?stock_type=a&type=hour&list_type=normal"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Host", "dq.10jqka.com.cn")
	req.Header.Set("Referer", "https://www.10jqka.com.cn/")

	resp, err := httputil.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取热股榜失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 解析响应，rate字段可能是string或float64，用interface{}兼容
	var result struct {
		StatusCode int `json:"status_code"`
		Data       struct {
			StockList []struct {
				Order        int         `json:"order"`
				Code         string      `json:"code"`
				Name         string      `json:"name"`
				RiseFall     interface{} `json:"rise_and_fall"`
				Rate         interface{} `json:"rate"`
				Analyse      string      `json:"analyse"`
				AnalyseTitle string      `json:"analyse_title"`
				Tag          struct {
					ConceptTag    interface{} `json:"concept_tag"` // 可能是[]string或null
					PopularityTag string      `json:"popularity_tag"`
				} `json:"tag"`
			} `json:"stock_list"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析热股榜响应失败: %w", err)
	}

	today := time.Now().Format("2006-01-02")
	var stocks []HotStock
	for _, item := range result.Data.StockList {
		changePct := parseToFloat64(item.RiseFall)
		hotValue := int64(parseToFloat64(item.Rate))

		// 解析concept_tag（可能是[]string或nil）
		var conceptTags []string
		if item.Tag.ConceptTag != nil {
			switch v := item.Tag.ConceptTag.(type) {
			case []interface{}:
				for _, t := range v {
					if s, ok := t.(string); ok {
						conceptTags = append(conceptTags, s)
					}
				}
			case string:
				if v != "" {
					conceptTags = []string{v}
				}
			}
		}

		stocks = append(stocks, HotStock{
			Rank:         item.Order,
			StockCode:    item.Code,
			ShortName:    item.Name,
			ChangePct:    changePct,
			HotValue:     hotValue,
			PopTag:       item.Tag.PopularityTag,
			ConceptTags:  conceptTags,
			Analyse:      item.Analyse,
			AnalyseTitle: item.AnalyseTitle,
			TradeDate:    today,
		})
	}

	return stocks, nil
}

// FetchHotConcept 获取同花顺热门概念20强
func FetchHotConcept() ([]HotConcept, error) {
	url := "https://dq.10jqka.com.cn/fuyao/hot_list_data/out/hot_list/v1/plate?type=concept"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Host", "dq.10jqka.com.cn")
	req.Header.Set("Referer", "https://www.10jqka.com.cn/")

	resp, err := httputil.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取热门概念榜失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var result struct {
		StatusCode int `json:"status_code"`
		Data       struct {
			PlateList []struct {
				Order    int         `json:"order"`
				Code     string      `json:"code"`
				Name     string      `json:"name"`
				RiseFall interface{} `json:"rise_and_fall"`
				Rate     interface{} `json:"rate"`
				HotTag   string      `json:"hot_tag"`
				EtfName  string      `json:"etf_name"`
			} `json:"plate_list"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析热门概念榜响应失败: %w", err)
	}

	today := time.Now().Format("2006-01-02")
	var concepts []HotConcept
	for _, item := range result.Data.PlateList {
		changePct := parseToFloat64(item.RiseFall)
		hotValue := int64(parseToFloat64(item.Rate))

		concepts = append(concepts, HotConcept{
			Rank:        item.Order,
			ConceptCode: item.Code,
			ConceptName: item.Name,
			ChangePct:   changePct,
			HotValue:    hotValue,
			HotTag:      item.HotTag,
			EtfName:     item.EtfName,
			TradeDate:   today,
		})
	}

	return concepts, nil
}

// SaveHotStockToDB 保存热股榜数据到数据库
func SaveHotStockToDB(db *sql.DB, data []HotStock) error {
	if len(data) == 0 {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback()

	for _, item := range data {
		conceptTag := strings.Join(item.ConceptTags, ",")
		_, err := tx.Exec(`
			INSERT INTO sentiment_hot_rank
			(rank, stock_code, short_name, change_pct, hot_value, pop_tag, concept_tag, analyse_text, analyse_title, source, trade_date)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
			ON CONFLICT ON CONSTRAINT sentiment_hot_rank_unique DO UPDATE SET
				stock_code=EXCLUDED.stock_code, short_name=EXCLUDED.short_name,
				change_pct=EXCLUDED.change_pct, hot_value=EXCLUDED.hot_value,
				pop_tag=EXCLUDED.pop_tag, concept_tag=EXCLUDED.concept_tag,
				analyse_text=EXCLUDED.analyse_text, analyse_title=EXCLUDED.analyse_title
		`, item.Rank, item.StockCode, item.ShortName, item.ChangePct, item.HotValue,
			item.PopTag, conceptTag, item.Analyse, item.AnalyseTitle, "hot-stock", item.TradeDate)
		if err != nil {
			return fmt.Errorf("保存热股榜数据失败: %w", err)
		}
	}
	return tx.Commit()
}

// SaveHotConceptToDB 保存热门概念榜数据到数据库
func SaveHotConceptToDB(db *sql.DB, data []HotConcept) error {
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
			INSERT INTO sentiment_hot_rank
			(rank, stock_code, short_name, change_pct, hot_value, pop_tag, concept_tag, analyse_text, analyse_title, source, trade_date)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
			ON CONFLICT ON CONSTRAINT sentiment_hot_rank_unique DO UPDATE SET
				stock_code=EXCLUDED.stock_code, short_name=EXCLUDED.short_name,
				change_pct=EXCLUDED.change_pct, hot_value=EXCLUDED.hot_value,
				pop_tag=EXCLUDED.pop_tag, concept_tag=EXCLUDED.concept_tag
		`, item.Rank, item.ConceptCode, item.ConceptName, item.ChangePct, item.HotValue,
			item.HotTag, item.EtfName, "", "", "hot-concept", item.TradeDate)
		if err != nil {
			return fmt.Errorf("保存热门概念榜数据失败: %w", err)
		}
	}
	return tx.Commit()
}

// parseToFloat64 将interface{}（string/float64/int等）转为float64
func parseToFloat64(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(val), 64)
		if err != nil {
			return 0
		}
		return f
	default:
		return 0
	}
}
