package analyst

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"comdigger/core/httputil"
)

// extractCode 从 "sz300454" 提取 "300454"
func extractCode(companyCode string) string {
	lower := strings.ToLower(companyCode)
	for _, prefix := range []string{"sz", "sh", "bj"} {
		if strings.HasPrefix(lower, prefix) {
			return companyCode[2:]
		}
	}
	return companyCode
}

func getFloat(m map[string]interface{}, key string) float64 {
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	if f, ok := v.(float64); ok {
		return f
	}
	// API sometimes returns numeric strings
	if s, ok := v.(string); ok && s != "" {
		var f float64
		if _, err := fmt.Sscanf(s, "%f", &f); err == nil {
			return f
		}
	}
	return 0
}

func getString(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// ratingToValue 将评级名称转为数值
func ratingToValue(name string) int {
	switch {
	case strings.Contains(name, "买入") || strings.Contains(name, "强烈推荐"):
		return RatingBuy
	case strings.Contains(name, "增持") || strings.Contains(name, "推荐") || strings.Contains(name, "跑赢"):
		return RatingOutperform
	case strings.Contains(name, "中性") || strings.Contains(name, "持有") || strings.Contains(name, "观望"):
		return RatingNeutral
	case strings.Contains(name, "卖出") || strings.Contains(name, "减持") || strings.Contains(name, "回避"):
		return RatingSell
	default:
		return RatingNeutral
	}
}

// FetchAnalystReports 获取机构研报数据
// stockCode 格式：sz300454
// days: 获取最近多少天的研报
func FetchAnalystReports(stockCode string, days int) ([]AnalystReport, error) {
	code := extractCode(stockCode)
	endTime := time.Now().Format("2006-01-02")
	beginTime := time.Now().AddDate(0, 0, -days).Format("2006-01-02")

	var allReports []AnalystReport
	page := 1
	pageSize := 100

	for {
		url := fmt.Sprintf(
			"https://reportapi.eastmoney.com/report/list?cb=datatable&code=%s&beginTime=%s&endTime=%s&pageSize=%d&pageNo=%d&qType=0",
			code, beginTime, endTime, pageSize, page,
		)

		body, err := httputil.FetchURL(context.Background(), url, map[string]string{
			"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Referer":    "https://data.eastmoney.com/report/stock.jshtml",
		})
		if err != nil {
			return nil, fmt.Errorf("获取研报数据失败: %w", err)
		}

		// 解析 JSONP：datatable({...}) 或 datatable{...}
		bodyStr := strings.TrimSpace(string(body))
		start := strings.Index(bodyStr, "{")
		end := strings.LastIndex(bodyStr, "}")
		if start < 0 || end < 0 || end <= start {
			truncLen := len(bodyStr)
			if truncLen > 200 {
				truncLen = 200
			}
			return nil, fmt.Errorf("JSONP 格式解析失败，响应: %s", bodyStr[:truncLen])
		}
		jsonStr := bodyStr[start : end+1]

		var raw map[string]interface{}
		if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
			return nil, fmt.Errorf("解析研报JSON失败: %w", err)
		}

		dataArr, ok := raw["data"].([]interface{})
		if !ok || len(dataArr) == 0 {
			break
		}

		for _, item := range dataArr {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			dateStr := getString(m, "publishDate")
			if len(dateStr) > 10 {
				dateStr = dateStr[:10]
			}
			t, _ := time.Parse("2006-01-02", dateStr)

			ratingName := getString(m, "emRatingName")
			report := AnalystReport{
				CompanyID:          stockCode,
				StockCode:          code,
				StockName:          getString(m, "stockName"),
				OrgName:            getString(m, "orgName"),
				PublishDate:        t,
				Title:              getString(m, "title"),
				RatingName:         ratingName,
				RatingValue:        ratingToValue(ratingName),
				PredictThisYearEPS: getFloat(m, "predictThisYearEps"),
				PredictNextYearEPS: getFloat(m, "predictNextYearEps"),
				PredictThisYearPE:  getFloat(m, "predictThisYearPe"),
				PredictNextYearPE:  getFloat(m, "predictNextYearPe"),
				InfoCode:           getString(m, "infoCode"),
			}
			allReports = append(allReports, report)
		}

		totalPage, _ := raw["TotalPage"].(float64)
		if page >= int(totalPage) || len(dataArr) < pageSize {
			break
		}
		page++
	}

	return allReports, nil
}

// SummarizeReports 统计研报汇总
func SummarizeReports(reports []AnalystReport) AnalystSummary {
	var summary AnalystSummary
	if len(reports) == 0 {
		return summary
	}

	var totalEPS1, totalEPS2 float64
	var epsCount1, epsCount2 int
	now := time.Now()
	thirtyDaysAgo := now.AddDate(0, 0, -30)

	for _, r := range reports {
		switch r.RatingValue {
		case RatingBuy:
			summary.BuyCount++
		case RatingOutperform:
			summary.HoldCount++
		case RatingNeutral:
			summary.NeutralCount++
		case RatingSell:
			summary.SellCount++
		}

		if r.PredictThisYearEPS > 0 {
			totalEPS1 += r.PredictThisYearEPS
			epsCount1++
		}
		if r.PredictNextYearEPS > 0 {
			totalEPS2 += r.PredictNextYearEPS
			epsCount2++
		}

		// 近30天评级变化（简单统计买入/增持为上调，卖出/减持为下调）
		if r.PublishDate.After(thirtyDaysAgo) {
			if r.RatingValue <= RatingOutperform {
				summary.RecentUpgrades++
			} else if r.RatingValue >= RatingSell {
				summary.RecentDowngrades++
			}
		}
	}

	if epsCount1 > 0 {
		summary.AvgThisYearEPS = totalEPS1 / float64(epsCount1)
	}
	if epsCount2 > 0 {
		summary.AvgNextYearEPS = totalEPS2 / float64(epsCount2)
	}

	// 目标价（用今年EPS × 今年PE估算）
	var prices []float64
	for _, r := range reports {
		if r.PredictThisYearEPS > 0 && r.PredictThisYearPE > 0 {
			price := r.PredictThisYearEPS * r.PredictThisYearPE
			prices = append(prices, price)
		}
	}
	if len(prices) > 0 {
		total := 0.0
		minP := math.MaxFloat64
		maxP := 0.0
		for _, p := range prices {
			total += p
			if p < minP {
				minP = p
			}
			if p > maxP {
				maxP = p
			}
		}
		summary.AvgTargetPrice = total / float64(len(prices))
		summary.MinTargetPrice = minP
		summary.MaxTargetPrice = maxP
	}

	return summary
}
