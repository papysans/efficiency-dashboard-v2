package sector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"comdigger/core/httputil"
)

// httpClient 共享 HTTP 客户端（用于市场概况等非 clist 接口）
var httpClient = &http.Client{Timeout: 15 * time.Second}

// doGet 发送 GET 请求并返回响应体（用于非 clist 接口，如市场概况、财经新闻）
func doGet(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://data.eastmoney.com/")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	return body, nil
}

// fetchClist 通过 httputil.FetchURL + JSONP 获取 clist 接口数据
// push2 clist 接口需要 cb=jQuery 参数，且非交易时间会 EOF
func fetchClist(url string) ([]map[string]interface{}, error) {
	bodyBytes, err := httputil.FetchURL(context.Background(), url, map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Referer":    "https://finance.eastmoney.com/",
	})
	if err != nil {
		// 非交易时间可能返回 EOF 或其他错误，返回空数组而不是错误
		fmt.Printf("⚠️ 板块数据请求失败（可能是非交易时间）: %v\n", err)
		return []map[string]interface{}{}, nil
	}

	// 检查响应是否为空
	if len(bodyBytes) == 0 {
		fmt.Printf("⚠️ 板块数据响应为空（可能是非交易时间）\n")
		return []map[string]interface{}{}, nil
	}

	// 剥离 JSONP：jQuery112309_1({...})
	bodyStr := string(bodyBytes)
	start := strings.Index(bodyStr, "(")
	end := strings.LastIndex(bodyStr, ")")
	if start < 0 || end <= start {
		// 非 JSONP 格式，返回空数组而不是错误
		fmt.Printf("⚠️ 板块数据响应格式异常（非 JSONP 格式），长度: %d\n", len(bodyBytes))
		return []map[string]interface{}{}, nil
	}
	jsonStr := bodyStr[start+1 : end]

	var result struct {
		Data struct {
			Diff []map[string]interface{} `json:"diff"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		fmt.Printf("⚠️ 板块数据解析失败: %v\n", err)
		return []map[string]interface{}{}, nil
	}

	// 如果没有数据，返回空数组
	if result.Data.Diff == nil {
		return []map[string]interface{}{}, nil
	}

	return result.Data.Diff, nil
}

// extractFloat 从 map 中提取 float64
func extractFloat(m map[string]interface{}, key string) float64 {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	}
	return 0
}

// extractInt 从 map 中提取 int
func extractInt(m map[string]interface{}, key string) int {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return int(val)
	}
	return 0
}

// fetchSectorList 通用板块列表请求（使用 JSONP 模式）
func fetchSectorList(url string, top int) ([]SectorInfo, error) {
	diff, err := fetchClist(url)
	if err != nil {
		return nil, err
	}
	if len(diff) == 0 {
		return []SectorInfo{}, nil
	}

	var sectors []SectorInfo
	for _, item := range diff {
		code, _ := item["f12"].(string)
		name, _ := item["f14"].(string)
		leadName, _ := item["f128"].(string)
		leadCode, _ := item["f140"].(string)
		sectors = append(sectors, SectorInfo{
			Code:           code,
			Name:           name,
			ChangePct:      extractFloat(item, "f3"),
			ChangeAmt:      extractFloat(item, "f4"),
			TotalMarketCap: extractFloat(item, "f20"),
			LeadStockName:  leadName,
			LeadStockCode:  leadCode,
			RisingCount:    extractInt(item, "f104"),
			FallingCount:   extractInt(item, "f105"),
		})
		if len(sectors) >= top {
			break
		}
	}
	return sectors, nil
}

// FetchIndustrySectors 获取行业板块涨跌幅排行（降序）
func FetchIndustrySectors(top int) ([]SectorInfo, error) {
	url := "https://push2.eastmoney.com/api/qt/clist/get?cb=jQuery112309_1&fid=f3&po=1&pz=100&pn=1&np=1&fltt=2&invt=2&fs=m:90+t:2&fields=f12,f14,f3,f4,f20,f128,f140,f104,f105"
	return fetchSectorList(url, top)
}

// FetchConceptSectors 获取概念板块涨跌幅排行（降序）
func FetchConceptSectors(top int) ([]SectorInfo, error) {
	url := "https://push2.eastmoney.com/api/qt/clist/get?cb=jQuery112309_1&fid=f3&po=1&pz=100&pn=1&np=1&fltt=2&invt=2&fs=m:90+t:3&fields=f12,f14,f3,f4,f20,f128,f140,f104,f105"
	return fetchSectorList(url, top)
}

// FetchIndustryFundFlow 获取行业板块资金流向（主力净流入降序）
func FetchIndustryFundFlow(top int) ([]SectorFundFlow, error) {
	url := "https://push2.eastmoney.com/api/qt/clist/get?cb=jQuery112309_1&fid=f62&po=1&pz=50&pn=1&np=1&fltt=2&invt=2&fs=m:90+t:2&fields=f12,f14,f3,f62,f184,f66,f69,f72,f75,f128,f140"
	diff, err := fetchClist(url)
	if err != nil {
		return nil, err
	}
	if len(diff) == 0 {
		return []SectorFundFlow{}, nil
	}

	var flows []SectorFundFlow
	for _, item := range diff {
		code, _ := item["f12"].(string)
		name, _ := item["f14"].(string)
		leadName, _ := item["f128"].(string)
		leadCode, _ := item["f140"].(string)
		flows = append(flows, SectorFundFlow{
			Code:               code,
			Name:               name,
			ChangePct:          extractFloat(item, "f3"),
			MainNetInflow:      extractFloat(item, "f62"),
			MainNetInflowRate:  extractFloat(item, "f184"),
			SuperNetInflow:     extractFloat(item, "f66"),
			SuperNetInflowRate: extractFloat(item, "f69"),
			BigNetInflow:       extractFloat(item, "f72"),
			BigNetInflowRate:   extractFloat(item, "f75"),
			LeadStockCode:      leadCode,
			LeadStockName:      leadName,
		})
		if len(flows) >= top {
			break
		}
	}
	return flows, nil
}

// FetchMarketOverview 获取4大指数概况
func FetchMarketOverview() ([]MarketOverview, error) {
	secids := []struct {
		secid string
	}{
		{"1.000001"}, // 上证指数
		{"0.399001"}, // 深证成指
		{"0.399006"}, // 创业板指
		{"1.000688"}, // 科创50
	}

	overviews := []MarketOverview{}
	for _, s := range secids {
		url := fmt.Sprintf(
			"https://push2.eastmoney.com/api/qt/stock/get?secid=%s&fields=f43,f57,f58,f169,f170,f171,f47,f48&fltt=2",
			s.secid,
		)
		body, err := doGet(url)
		if err != nil {
			fmt.Printf("获取指数 %s 失败: %v\n", s.secid, err)
			continue
		}

		var result struct {
			Data map[string]interface{} `json:"data"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			fmt.Printf("解析指数 %s 失败: %v\n", s.secid, err)
			continue
		}
		if result.Data == nil {
			continue
		}

		code, _ := result.Data["f57"].(string)
		name, _ := result.Data["f58"].(string)
		volume := int64(0)
		if v, ok := result.Data["f47"].(float64); ok {
			volume = int64(v)
		}
		overviews = append(overviews, MarketOverview{
			Code:      code,
			Name:      name,
			Price:     extractFloat(result.Data, "f43"),
			ChangeAmt: extractFloat(result.Data, "f169"),
			ChangePct: extractFloat(result.Data, "f170"),
			Amplitude: extractFloat(result.Data, "f171"),
			Volume:    volume,
			Amount:    extractFloat(result.Data, "f48"),
		})
	}
	return overviews, nil
}

// FetchFinNews 获取最新财经快讯（东方财富快讯）
func FetchFinNews(count int) ([]FinNews, error) {
	url := fmt.Sprintf(
		"https://newsapi.eastmoney.com/kuaixun/v1/getlist_115_ajaxResult_%d_1_.html",
		count,
	)
	body, err := doGet(url)
	if err != nil {
		return nil, err
	}

	// 响应格式：var ajaxResult={...}
	bodyStr := string(body)
	start := strings.Index(bodyStr, "=")
	if start < 0 {
		return nil, fmt.Errorf("无法解析快讯响应格式")
	}
	jsonStr := strings.TrimSpace(bodyStr[start+1:])

	var result struct {
		LivesList []struct {
			Title  string `json:"title"`
			Sort   string `json:"sort"`
			NewsID string `json:"newsid"`
		} `json:"LivesList"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("解析财经快讯失败: %w", err)
	}

	var news []FinNews
	for _, item := range result.LivesList {
		news = append(news, FinNews{
			Title:  item.Title,
			Time:   item.Sort,
			Source: "eastmoney",
		})
	}
	return news, nil
}

// FetchSector 获取完整的板块轮动数据摘要
func FetchSector() (*SectorSummary, error) {
	summary := &SectorSummary{
		MarketOverviews:   []MarketOverview{},
		TopRisingSectors:  []SectorInfo{},
		TopFallingSectors: []SectorInfo{},
		TopConceptSectors: []SectorInfo{},
		TopInflowSectors:  []SectorFundFlow{},
		TopOutflowSectors: []SectorFundFlow{},
		LatestNews:        []FinNews{},
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	errors := make([]error, 0)

	// 1. 获取4大指数
	wg.Add(1)
	go func() {
		defer wg.Done()
		overviews, err := FetchMarketOverview()
		mu.Lock()
		if err != nil {
			errors = append(errors, fmt.Errorf("市场指数: %w", err))
		} else {
			summary.MarketOverviews = overviews
		}
		mu.Unlock()
	}()

	// 2. 获取行业板块（涨幅前15 + 跌幅前15）
	wg.Add(1)
	go func() {
		defer wg.Done()
		// 获取前100个，前15是涨幅榜，后15是跌幅榜
		allIndustries, err := FetchIndustrySectors(100)
		mu.Lock()
		if err != nil {
			errors = append(errors, fmt.Errorf("行业板块: %w", err))
		} else {
			if len(allIndustries) >= 15 {
				summary.TopRisingSectors = allIndustries[:15]
			}
			if len(allIndustries) >= 30 {
				summary.TopFallingSectors = allIndustries[len(allIndustries)-15:]
			}
		}
		mu.Unlock()
	}()

	// 3. 获取概念板块涨幅前15
	wg.Add(1)
	go func() {
		defer wg.Done()
		concepts, err := FetchConceptSectors(15)
		mu.Lock()
		if err != nil {
			errors = append(errors, fmt.Errorf("概念板块: %w", err))
		} else {
			summary.TopConceptSectors = concepts
		}
		mu.Unlock()
	}()

	// 4. 获取资金流向（流入前15 + 流出前10）
	wg.Add(1)
	go func() {
		defer wg.Done()
		// 获取前50个，前15是流入榜，后10是流出榜
		allFlows, err := FetchIndustryFundFlow(50)
		mu.Lock()
		if err != nil {
			errors = append(errors, fmt.Errorf("资金流向: %w", err))
		} else {
			if len(allFlows) >= 15 {
				summary.TopInflowSectors = allFlows[:15]
			}
			if len(allFlows) >= 25 {
				summary.TopOutflowSectors = allFlows[len(allFlows)-10:]
			}
		}
		mu.Unlock()
	}()

	// 5. 获取财经新闻
	wg.Add(1)
	go func() {
		defer wg.Done()
		news, err := FetchFinNews(20)
		mu.Lock()
		if err != nil {
			errors = append(errors, fmt.Errorf("财经新闻: %w", err))
		} else {
			summary.LatestNews = news
		}
		mu.Unlock()
	}()

	wg.Wait()

	// 如果有错误，返回第一个错误
	if len(errors) > 0 {
		return summary, errors[0]
	}

	return summary, nil
}
