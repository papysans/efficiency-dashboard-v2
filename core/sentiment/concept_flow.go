package sentiment

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"comdigger/core/httputil"
)

// ConceptFlow 概念板块资金流向
type ConceptFlow struct {
	IndexCode         string
	IndexName         string
	ChangePct         float64
	MainNetInflow     float64 // 主力净流入（元）
	MainNetInflowRate float64 // 主力净流入占比（%）
	MaxNetInflow      float64 // 超大单净流入（元）
	MaxNetInflowRate  float64 // 超大单净流入占比（%）
	LgNetInflow       float64 // 大单净流入（元）
	LgNetInflowRate   float64 // 大单净流入占比（%）
	MidNetInflow      float64 // 中单净流入（元）
	MidNetInflowRate  float64 // 中单净流入占比（%）
	SmNetInflow       float64 // 小单净流入（元）
	SmNetInflowRate   float64 // 小单净流入占比（%）
	LeadStockCode     string  // 领涨股代码
	LeadStockName     string  // 领涨股名称
}

// FetchConceptFlow 获取概念板块资金流向（东方财富）
// daysType: 1=今日, 5=近5日, 10=近10日
func FetchConceptFlow(daysType int) ([]ConceptFlow, error) {
	type dayConfig struct {
		fid    string
		fields string
	}
	configs := map[int]dayConfig{
		1:  {fid: "f62", fields: "f12,f14,f2,f3,f62,f184,f66,f69,f72,f75,f78,f81,f84,f87,f124,f136"},
		5:  {fid: "f164", fields: "f12,f14,f2,f3,f164,f165,f166,f167,f168,f169,f170,f171,f172,f173,f124,f136"},
		10: {fid: "f174", fields: "f12,f14,f2,f3,f174,f175,f176,f177,f178,f179,f180,f181,f182,f183,f124,f136"},
	}
	cfg, ok := configs[daysType]
	if !ok {
		cfg = configs[1]
	}

	var all []ConceptFlow

	for page := 1; ; page++ {
		// fs 参数：m:90%20t:3（空格用%20编码，冒号直接传，PowerShell 单引号字符串安全）
		url := fmt.Sprintf(
			"https://push2.eastmoney.com/api/qt/clist/get?cb=jQuery112309_1&fid=%s&po=1&pz=50&pn=%d&np=1&fltt=2&invt=2&fs=m:90%%20t:3&fields=%s",
			cfg.fid, page, cfg.fields,
		)

		bodyBytes, err := httputil.FetchURL(context.Background(), url, emHeaders())
		if err != nil {
			// 非交易时间 clist 接口会关闭连接，第一页失败时给出友好提示
			if page == 1 {
				return nil, fmt.Errorf("获取概念资金流向失败（可能为非交易时间，该接口仅交易时段有数据）: %w", err)
			}
			break
		}

		bodyStr := string(bodyBytes)
		start := strings.Index(bodyStr, "(")
		end := strings.LastIndex(bodyStr, ")")
		if start < 0 || end <= start {
			break
		}
		jsonStr := bodyStr[start+1 : end]

		var result struct {
			Data struct {
				Diff []map[string]interface{} `json:"diff"`
			} `json:"data"`
		}
		if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
			return nil, fmt.Errorf("解析概念资金流向响应失败: %w", err)
		}
		if len(result.Data.Diff) == 0 {
			break
		}

		// 根据 daysType 确定各字段的 key
		var (
			mainKey, mainRateKey string
			maxKey, maxRateKey   string
			lgKey, lgRateKey     string
			midKey, midRateKey   string
			smKey, smRateKey     string
		)
		switch daysType {
		case 5:
			mainKey, mainRateKey = "f164", "f165"
			maxKey, maxRateKey = "f166", "f167"
			lgKey, lgRateKey = "f168", "f169"
			midKey, midRateKey = "f170", "f171"
			smKey, smRateKey = "f172", "f173"
		case 10:
			mainKey, mainRateKey = "f174", "f175"
			maxKey, maxRateKey = "f176", "f177"
			lgKey, lgRateKey = "f178", "f179"
			midKey, midRateKey = "f180", "f181"
			smKey, smRateKey = "f182", "f183"
		default: // 1
			mainKey, mainRateKey = "f62", "f184"
			maxKey, maxRateKey = "f66", "f69"
			lgKey, lgRateKey = "f72", "f75"
			midKey, midRateKey = "f78", "f81"
			smKey, smRateKey = "f84", "f87"
		}

		for _, item := range result.Data.Diff {
			mainNet, ok1 := extractFloat(item, mainKey)
			if !ok1 {
				continue
			}

			indexCode, _ := item["f12"].(string)
			indexName, _ := item["f14"].(string)

			changePct, _ := extractFloat(item, "f3")
			mainRate, _ := extractFloat(item, mainRateKey)
			maxNet, _ := extractFloat(item, maxKey)
			maxRate, _ := extractFloat(item, maxRateKey)
			lgNet, _ := extractFloat(item, lgKey)
			lgRate, _ := extractFloat(item, lgRateKey)
			midNet, _ := extractFloat(item, midKey)
			midRate, _ := extractFloat(item, midRateKey)
			smNet, _ := extractFloat(item, smKey)
			smRate, _ := extractFloat(item, smRateKey)

			leadCode, _ := item["f124"].(string)
			leadName, _ := item["f136"].(string)

			all = append(all, ConceptFlow{
				IndexCode:         indexCode,
				IndexName:         indexName,
				ChangePct:         changePct,
				MainNetInflow:     mainNet,
				MainNetInflowRate: mainRate,
				MaxNetInflow:      maxNet,
				MaxNetInflowRate:  maxRate,
				LgNetInflow:       lgNet,
				LgNetInflowRate:   lgRate,
				MidNetInflow:      midNet,
				MidNetInflowRate:  midRate,
				SmNetInflow:       smNet,
				SmNetInflowRate:   smRate,
				LeadStockCode:     leadCode,
				LeadStockName:     leadName,
			})
		}
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].MainNetInflow > all[j].MainNetInflow
	})

	return all, nil
}

// SaveConceptFlowToDB 保存概念资金流向到数据库
func SaveConceptFlowToDB(db *sql.DB, daysType int, data []ConceptFlow) error {
	if len(data) == 0 {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback()

	fetchedDate := time.Now().Format("2006-01-02")
	for _, item := range data {
		_, err := tx.Exec(`
			INSERT INTO sentiment_concept_flow
			(index_code, index_name, days_type, change_pct, main_net_inflow, main_net_inflow_rate,
			 max_net_inflow, max_net_inflow_rate, lg_net_inflow, lg_net_inflow_rate,
			 mid_net_inflow, mid_net_inflow_rate, sm_net_inflow, sm_net_inflow_rate,
			 lead_stock_code, lead_stock_name, fetched_date)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
			ON CONFLICT ON CONSTRAINT sentiment_concept_flow_unique DO UPDATE SET
				main_net_inflow=EXCLUDED.main_net_inflow,
				main_net_inflow_rate=EXCLUDED.main_net_inflow_rate,
				max_net_inflow=EXCLUDED.max_net_inflow,
				max_net_inflow_rate=EXCLUDED.max_net_inflow_rate,
				lg_net_inflow=EXCLUDED.lg_net_inflow,
				lg_net_inflow_rate=EXCLUDED.lg_net_inflow_rate,
				mid_net_inflow=EXCLUDED.mid_net_inflow,
				mid_net_inflow_rate=EXCLUDED.mid_net_inflow_rate,
				sm_net_inflow=EXCLUDED.sm_net_inflow,
				sm_net_inflow_rate=EXCLUDED.sm_net_inflow_rate,
				lead_stock_code=EXCLUDED.lead_stock_code,
				lead_stock_name=EXCLUDED.lead_stock_name,
				change_pct=EXCLUDED.change_pct
		`, item.IndexCode, item.IndexName, daysType, item.ChangePct,
			item.MainNetInflow, item.MainNetInflowRate,
			item.MaxNetInflow, item.MaxNetInflowRate,
			item.LgNetInflow, item.LgNetInflowRate,
			item.MidNetInflow, item.MidNetInflowRate,
			item.SmNetInflow, item.SmNetInflowRate,
			item.LeadStockCode, item.LeadStockName, fetchedDate)
		if err != nil {
			return fmt.Errorf("保存概念资金流向失败: %w", err)
		}
	}
	return tx.Commit()
}

// extractFloat 从 map 中提取 float64，若值为 "-" 则返回 (0, false)
func extractFloat(m map[string]interface{}, key string) (float64, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch val := v.(type) {
	case float64:
		return val, true
	case string:
		if val == "-" {
			return 0, false
		}
		return 0, true
	}
	return 0, true
}
