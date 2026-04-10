package aiagent

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"sync"

	"comdigger/core/globalmarket"
	"comdigger/core/kline"
	"comdigger/core/technical"
)

// Agent 分析师接口
type Agent interface {
	Name() string
	Analyze(ctx context.Context, db *sql.DB, companyID, stockName string, client *GLMClient) (*AgentResult, error)
}

// AgentResult 分析师结果
type AgentResult struct {
	AgentName  string
	Analysis   string
	Suggestion string
	RiskLevel  string
	Confidence int
	Err        error
}

// agentResponse AI返回的JSON结构
type agentResponse struct {
	Analysis   string `json:"analysis"`
	Suggestion string `json:"suggestion"`
	RiskLevel  string `json:"risk_level"`
	Confidence int    `json:"confidence"`
}

// parseAgentResponse 解析AI返回的JSON，失败时降级
func parseAgentResponse(raw, agentName string) *AgentResult {
	result := &AgentResult{
		AgentName:  agentName,
		Analysis:   raw,
		Suggestion: "请参考分析内容",
		RiskLevel:  "中性",
		Confidence: 50,
	}

	// 尝试提取JSON块
	jsonStr := raw
	if idx := strings.Index(raw, "{"); idx >= 0 {
		if end := strings.LastIndex(raw, "}"); end > idx {
			jsonStr = raw[idx : end+1]
		}
	}

	var resp agentResponse
	if err := json.Unmarshal([]byte(jsonStr), &resp); err == nil {
		if resp.Analysis != "" {
			result.Analysis = resp.Analysis
		}
		if resp.Suggestion != "" {
			result.Suggestion = resp.Suggestion
		}
		if resp.RiskLevel != "" {
			result.RiskLevel = resp.RiskLevel
		}
		if resp.Confidence > 0 {
			result.Confidence = resp.Confidence
		}
	}
	return result
}

// ─────────────────────────────────────────────────────────────────────────────
// TechnicalAgent 技术分析师
// ─────────────────────────────────────────────────────────────────────────────

type TechnicalAgent struct{}

func (a *TechnicalAgent) Name() string { return "技术分析师" }

func (a *TechnicalAgent) Analyze(ctx context.Context, db *sql.DB, companyID, stockName string, client *GLMClient) (*AgentResult, error) {
	bars, err := kline.LoadKlineFromDB(db, companyID, 120, "1d")
	if err != nil || len(bars) < 30 {
		return &AgentResult{
			AgentName:  a.Name(),
			Analysis:   "K线数据不足，无法进行技术分析",
			RiskLevel:  "中性",
			Confidence: 30,
		}, nil
	}

	data := technical.CalcAllIndicators(bars)
	result := technical.GenerateSignals(data)

	// 构建技术数据摘要
	lastBar := bars[len(bars)-1]
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("股票: %s (%s)\n", stockName, companyID))
	sb.WriteString(fmt.Sprintf("最新收盘: %.2f  日期: %s\n", lastBar.Close, lastBar.Time.Format("2006-01-02")))
	sb.WriteString(fmt.Sprintf("综合信号: %s  得分: %.1f\n", string(result.Overall), result.Score))
	sb.WriteString("各指标信号:\n")
	for _, sig := range result.Signals {
		sb.WriteString(fmt.Sprintf("  %s: %s (强度%d星) - %s\n", sig.Name, string(sig.Signal), sig.Strength, sig.Reason))
	}
	if result.ATRValue > 0 {
		sb.WriteString(fmt.Sprintf("ATR波动率: %.2f%%\n", result.ATRPercent))
	}

	systemPrompt := "你是一位专业的股票技术分析师，擅长通过技术指标判断股票走势。请基于提供的技术指标数据进行分析，给出专业的技术面判断。"
	userPrompt := fmt.Sprintf(`请对以下股票的技术指标进行分析：

%s

请以JSON格式输出分析结果：
{
  "analysis": "技术面综合分析（200字以内）",
  "suggestion": "操作建议（买入/持有/观望/减仓/卖出）",
  "risk_level": "高/中/低",
  "confidence": 置信度(0-100的整数)
}`, sb.String())

	raw, err := client.Chat(ctx, systemPrompt, userPrompt)
	if err != nil {
		return &AgentResult{AgentName: a.Name(), Err: err}, err
	}
	return parseAgentResponse(raw, a.Name()), nil
}

// ─────────────────────────────────────────────────────────────────────────────
// FundamentalAgent 基本面分析师
// ─────────────────────────────────────────────────────────────────────────────

type FundamentalAgent struct{}

func (a *FundamentalAgent) Name() string { return "基本面分析师" }

func (a *FundamentalAgent) Analyze(ctx context.Context, db *sql.DB, companyID, stockName string, client *GLMClient) (*AgentResult, error) {
	// 查询最近8期年报财务数据（年报 report_date 月份=12、日=31）
	rows, err := db.QueryContext(ctx, `
		SELECT item_field, item_value, report_date
		FROM fin
		WHERE company_id = $1
		  AND EXTRACT(MONTH FROM report_date) = 12
		  AND EXTRACT(DAY FROM report_date) = 31
		ORDER BY report_date DESC
		LIMIT 200
	`, companyID)
	if err != nil {
		return &AgentResult{AgentName: a.Name(), Analysis: "财务数据查询失败", RiskLevel: "中性", Confidence: 30}, nil
	}
	defer rows.Close()

	// 整理财务数据
	type finRow struct {
		field string
		value float64
		date  string
	}
	var finData []finRow
	for rows.Next() {
		var f finRow
		var val sql.NullFloat64
		if err := rows.Scan(&f.field, &val, &f.date); err == nil && val.Valid {
			f.value = val.Float64
			finData = append(finData, f)
		}
	}

	if len(finData) == 0 {
		return &AgentResult{
			AgentName:  a.Name(),
			Analysis:   "暂无财务数据",
			RiskLevel:  "中性",
			Confidence: 30,
		}, nil
	}

	// 构建财务摘要（按日期分组，取关键字段）
	keyFields := map[string]string{
		"REVENUE": "营收(元)", "NETPROFIT": "净利润(元)",
		"ROE": "ROE(%)", "TOTLIAB_TOASSET": "资产负债率(%)",
		"OPERATECASHFLOW": "经营现金流(元)", "GROSSMARGIN": "毛利率(%)",
	}
	dateData := make(map[string]map[string]float64)
	for _, f := range finData {
		if _, ok := keyFields[f.field]; ok {
			if dateData[f.date] == nil {
				dateData[f.date] = make(map[string]float64)
			}
			dateData[f.date][f.field] = f.value
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("股票: %s (%s)\n", stockName, companyID))
	sb.WriteString("近年财务数据（单位：元）:\n")
	// 取最近4年
	dates := make([]string, 0)
	for d := range dateData {
		dates = append(dates, d)
	}
	// 简单排序（字符串年份降序）
	for i := 0; i < len(dates)-1; i++ {
		for j := i + 1; j < len(dates); j++ {
			if dates[i] < dates[j] {
				dates[i], dates[j] = dates[j], dates[i]
			}
		}
	}
	limit := 4
	if len(dates) < limit {
		limit = len(dates)
	}
	for _, d := range dates[:limit] {
		year := d
		if len(d) >= 4 {
			year = d[:4] // 取 YYYY 部分
		}
		sb.WriteString(fmt.Sprintf("  %s年: ", year))
		for field, label := range keyFields {
			if v, ok := dateData[d][field]; ok {
				sb.WriteString(fmt.Sprintf("%s=%.2f ", label, v))
			}
		}
		sb.WriteString("\n")
	}

	systemPrompt := "你是一位专业的股票基本面分析师，擅长通过财务数据评估公司价值和投资潜力。"
	userPrompt := fmt.Sprintf(`请对以下股票的基本面进行分析：

%s

请以JSON格式输出分析结果：
{
  "analysis": "基本面综合分析（200字以内）",
  "suggestion": "操作建议（买入/持有/观望/减仓/卖出）",
  "risk_level": "高/中/低",
  "confidence": 置信度(0-100的整数)
}`, sb.String())

	raw, err := client.Chat(ctx, systemPrompt, userPrompt)
	if err != nil {
		return &AgentResult{AgentName: a.Name(), Err: err}, err
	}
	return parseAgentResponse(raw, a.Name()), nil
}

// ─────────────────────────────────────────────────────────────────────────────
// FundFlowAgent 资金面分析师（龙虎榜）
// ─────────────────────────────────────────────────────────────────────────────

type FundFlowAgent struct{}

func (a *FundFlowAgent) Name() string { return "资金面分析师" }

func (a *FundFlowAgent) Analyze(ctx context.Context, db *sql.DB, companyID, stockName string, client *GLMClient) (*AgentResult, error) {
	// 从龙虎榜记录中查找该股票（companyID格式如 sz300454，stock_code 格式如 300454）
	code := companyID
	if len(code) > 2 {
		code = code[2:] // 去掉市场前缀
	}
	since := time.Now().AddDate(0, 0, -30).Format("2006-01-02")

	rows, err := db.QueryContext(ctx, `
		SELECT report_date, youzi_name, yingye_bu, buy_amount, sell_amount, net_amount
		FROM longhubang_records
		WHERE stock_code = $1 AND report_date >= $2
		ORDER BY report_date DESC
		LIMIT 50
	`, code, since)

	if err != nil || rows == nil {
		return &AgentResult{
			AgentName:  a.Name(),
			Analysis:   "近期未出现在龙虎榜，资金面无特殊信号",
			RiskLevel:  "中性",
			Confidence: 40,
		}, nil
	}
	defer rows.Close()

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("股票: %s (%s) 近30天龙虎榜记录:\n", stockName, companyID))
	count := 0
	for rows.Next() {
		var date time.Time
		var youziName, yyb string
		var buy, sell, net float64
		if err := rows.Scan(&date, &youziName, &yyb, &buy, &sell, &net); err == nil {
			sb.WriteString(fmt.Sprintf("  %s: %s(%s) 买入%.0f万 卖出%.0f万 净流入%.0f万\n",
				date.Format("01-02"), youziName, yyb, buy/10000, sell/10000, net/10000))
			count++
		}
	}

	if count == 0 {
		return &AgentResult{
			AgentName:  a.Name(),
			Analysis:   "近30天未出现在龙虎榜，资金面无特殊信号",
			RiskLevel:  "中性",
			Confidence: 40,
		}, nil
	}

	systemPrompt := "你是一位专业的股票资金面分析师，擅长通过龙虎榜数据判断主力资金动向。"
	userPrompt := fmt.Sprintf(`请对以下股票的资金面进行分析：

%s

请以JSON格式输出分析结果：
{
  "analysis": "资金面综合分析（200字以内）",
  "suggestion": "操作建议（买入/持有/观望/减仓/卖出）",
  "risk_level": "高/中/低",
  "confidence": 置信度(0-100的整数)
}`, sb.String())

	raw, err := client.Chat(ctx, systemPrompt, userPrompt)
	if err != nil {
		return &AgentResult{AgentName: a.Name(), Err: err}, err
	}
	return parseAgentResponse(raw, a.Name()), nil
}

// ─────────────────────────────────────────────────────────────────────────────
// RiskAgent 风险管理师
// ─────────────────────────────────────────────────────────────────────────────

type RiskAgent struct{}

func (a *RiskAgent) Name() string { return "风险管理师" }

func (a *RiskAgent) Analyze(ctx context.Context, db *sql.DB, companyID, stockName string, client *GLMClient) (*AgentResult, error) {
	// 从fin表读取风险相关字段
	rows, err := db.QueryContext(ctx, `
		SELECT item_field, item_value, report_date
		FROM fin
		WHERE company_id = $1
		  AND EXTRACT(MONTH FROM report_date) = 12
		  AND EXTRACT(DAY FROM report_date) = 31
		  AND item_field IN ('TOTLIAB_TOASSET','GOODWILL','ACCOUNTSREC','OPERATECASHFLOW','NETPROFIT')
		ORDER BY report_date DESC
		LIMIT 50
	`, companyID)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("股票: %s (%s) 风险指标:\n", stockName, companyID))

	if err == nil && rows != nil {
		defer rows.Close()
		for rows.Next() {
			var field string
			var val sql.NullFloat64
			var date string
			if err := rows.Scan(&field, &val, &date); err == nil && val.Valid {
				year := date
				if len(date) >= 4 {
					year = date[:4]
				}
				sb.WriteString(fmt.Sprintf("  %s年 %s=%.2f\n", year, field, val.Float64))
			}
		}
	} else {
		sb.WriteString("  财务数据暂不可用\n")
	}

	systemPrompt := "你是一位专业的股票风险管理师，擅长识别财务风险、经营风险和市场风险。"
	userPrompt := fmt.Sprintf(`请对以下股票进行风险评估：

%s

请以JSON格式输出分析结果：
{
  "analysis": "风险综合评估（200字以内）",
  "suggestion": "风险建议（持有/减仓/卖出/观望）",
  "risk_level": "高/中/低",
  "confidence": 置信度(0-100的整数)
}`, sb.String())

	raw, err := client.Chat(ctx, systemPrompt, userPrompt)
	if err != nil {
		return &AgentResult{AgentName: a.Name(), Err: err}, err
	}
	return parseAgentResponse(raw, a.Name()), nil
}

// ─────────────────────────────────────────────────────────────────────────────
// SentimentAgent 市场情绪分析师
// ─────────────────────────────────────────────────────────────────────────────

type SentimentAgent struct{}

func (a *SentimentAgent) Name() string { return "市场情绪分析师" }

func (a *SentimentAgent) Analyze(ctx context.Context, db *sql.DB, companyID, stockName string, client *GLMClient) (*AgentResult, error) {
	since := time.Now().AddDate(0, 0, -7).Format("2006-01-02")
	rows, err := db.QueryContext(ctx, `
		SELECT title, platform, score, publish_time
		FROM news_flow_records
		WHERE (title LIKE $1 OR content LIKE $1)
		  AND publish_time >= $2
		ORDER BY score DESC
		LIMIT 20
	`, "%"+stockName+"%", since)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("股票: %s 近7天市场情绪数据:\n", stockName))

	newsCount := 0
	if err == nil && rows != nil {
		defer rows.Close()
		for rows.Next() {
			var title, platform string
			var score float64
			var publishTime time.Time
			if err := rows.Scan(&title, &platform, &score, &publishTime); err == nil {
				sb.WriteString(fmt.Sprintf("  [%s][%s] %s\n", platform, publishTime.Format("01-02"), title))
				newsCount++
			}
		}
	}

	if newsCount == 0 {
		return &AgentResult{
			AgentName:  a.Name(),
			Analysis:   "近7天暂无相关新闻，市场情绪中性",
			RiskLevel:  "中性",
			Confidence: 40,
		}, nil
	}

	systemPrompt := "你是一位专业的市场情绪分析师，擅长通过新闻舆情判断市场对个股的情绪倾向。"
	userPrompt := fmt.Sprintf(`请分析以下股票的市场情绪：

%s

请以JSON格式输出分析结果：
{
  "analysis": "市场情绪综合分析（200字以内）",
  "suggestion": "操作建议（买入/持有/观望/减仓/卖出）",
  "risk_level": "高/中/低",
  "confidence": 置信度(0-100的整数)
}`, sb.String())

	raw, err := client.Chat(ctx, systemPrompt, userPrompt)
	if err != nil {
		return &AgentResult{AgentName: a.Name(), Err: err}, err
	}
	return parseAgentResponse(raw, a.Name()), nil
}

// ─────────────────────────────────────────────────────────────────────────────
// NewsAgent 新闻分析师
// ─────────────────────────────────────────────────────────────────────────────

type NewsAgent struct{}

func (a *NewsAgent) Name() string { return "新闻分析师" }

func (a *NewsAgent) Analyze(ctx context.Context, db *sql.DB, companyID, stockName string, client *GLMClient) (*AgentResult, error) {
	since := time.Now().AddDate(0, 0, -7).Format("2006-01-02")
	rows, err := db.QueryContext(ctx, `
		SELECT title, content, platform, publish_time
		FROM news_flow_records
		WHERE (title LIKE $1 OR content LIKE $1)
		  AND publish_time >= $2
		ORDER BY score DESC
		LIMIT 15
	`, "%"+stockName+"%", since)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("股票: %s 近7天相关新闻:\n", stockName))

	newsCount := 0
	if err == nil && rows != nil {
		defer rows.Close()
		for rows.Next() {
			var title, content, platform string
			var publishTime time.Time
			if err := rows.Scan(&title, &content, &platform, &publishTime); err == nil {
				// 截取content前100字
				contentPreview := content
				runes := []rune(content)
				if len(runes) > 100 {
					contentPreview = string(runes[:100]) + "..."
				}
				sb.WriteString(fmt.Sprintf("  [%s][%s] %s\n  摘要: %s\n\n",
					platform, publishTime.Format("01-02"), title, contentPreview))
				newsCount++
			}
		}
	}

	if newsCount == 0 {
		return &AgentResult{
			AgentName:  a.Name(),
			Analysis:   "近7天暂无相关新闻报道",
			RiskLevel:  "中性",
			Confidence: 40,
		}, nil
	}

	systemPrompt := "你是一位专业的财经新闻分析师，擅长从新闻报道中提取对股票投资有价值的信息。"
	userPrompt := fmt.Sprintf(`请分析以下股票的相关新闻对投资的影响：

%s

请以JSON格式输出分析结果：
{
  "analysis": "新闻影响综合分析（200字以内）",
  "suggestion": "操作建议（买入/持有/观望/减仓/卖出）",
  "risk_level": "高/中/低",
  "confidence": 置信度(0-100的整数)
}`, sb.String())

	raw, err := client.Chat(ctx, systemPrompt, userPrompt)
	if err != nil {
		return &AgentResult{AgentName: a.Name(), Err: err}, err
	}
	return parseAgentResponse(raw, a.Name()), nil
}

// ─────────────────────────────────────────────────────────────────────────────
// GlobalMarketAgent 全球市场分析师
// ─────────────────────────────────────────────────────────────────────────────

type GlobalMarketAgent struct{}

func (a *GlobalMarketAgent) Name() string { return "全球市场分析师" }

func (a *GlobalMarketAgent) Analyze(ctx context.Context, db *sql.DB, companyID, stockName string, client *GLMClient) (*AgentResult, error) {
	// 步骤1：查预设映射
	peerInfo, found := globalmarket.GetPeerInfo(companyID)

	peers := peerInfo.Peers
	peerNames := peerInfo.PeerNames

	// 步骤2：若未找到预设，调用 AI 推断
	if !found {
		systemPrompt := "你是美股专家，熟悉中美股市对应关系"
		userPrompt := fmt.Sprintf("请列出与%s业务最相似的3-5个美股上市公司ticker代码，只返回JSON数组，例如[\"crwd\",\"panw\"]，不要其他内容", stockName)
		raw, err := client.Chat(ctx, systemPrompt, userPrompt)
		if err == nil {
			// 提取 JSON 数组
			jsonStr := raw
			if idx := strings.Index(raw, "["); idx >= 0 {
				if end := strings.LastIndex(raw, "]"); end > idx {
					jsonStr = raw[idx : end+1]
				}
			}
			var tickers []string
			if jsonErr := json.Unmarshal([]byte(jsonStr), &tickers); jsonErr == nil {
				for _, ticker := range tickers {
					ticker = strings.ToLower(strings.TrimSpace(ticker))
					if ticker != "" {
						peers = append(peers, ticker+".us")
						peerNames = append(peerNames, strings.ToUpper(ticker))
					}
				}
			}
			// 解析失败则 peers 保持为空，只分析大盘指数
		}
	}

	// 步骤3：并发抓取三组数据
	var (
		peerSummaries  []globalmarket.StockSummary
		indexSummaries []globalmarket.StockSummary
		etfSummary     *globalmarket.StockSummary
		wg             sync.WaitGroup
		mu             sync.Mutex
	)

	// 抓取对标个股（最多5个）
	if len(peers) > 0 {
		fetchPeers := peers
		fetchNames := peerNames
		if len(fetchPeers) > 5 {
			fetchPeers = fetchPeers[:5]
			fetchNames = fetchNames[:5]
		}
		// 确保 names 长度与 peers 一致
		for len(fetchNames) < len(fetchPeers) {
			fetchNames = append(fetchNames, "")
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := globalmarket.FetchMultiple(fetchPeers, fetchNames, 5)
			mu.Lock()
			peerSummaries = result
			mu.Unlock()
		}()
	}

	// 抓取全球指数
	wg.Add(1)
	go func() {
		defer wg.Done()
		indexSymbols := make([]string, len(globalmarket.GlobalIndices))
		indexNames := make([]string, len(globalmarket.GlobalIndices))
		for i, idx := range globalmarket.GlobalIndices {
			indexSymbols[i] = idx.Symbol
			indexNames[i] = idx.Name
		}
		result := globalmarket.FetchMultiple(indexSymbols, indexNames, 5)
		mu.Lock()
		indexSummaries = result
		mu.Unlock()
	}()

	// 抓取行业 ETF
	if peerInfo.SectorETF != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, _ := globalmarket.FetchStockData(peerInfo.SectorETF, peerInfo.SectorETFName, 5)
			mu.Lock()
			etfSummary = result
			mu.Unlock()
		}()
	}

	wg.Wait()

	// 步骤4：若三组数据均为空，直接返回降级结果
	if len(peerSummaries) == 0 && len(indexSummaries) == 0 && etfSummary == nil {
		return &AgentResult{
			AgentName:  a.Name(),
			Analysis:   "全球市场数据获取失败，无法分析",
			RiskLevel:  "中性",
			Confidence: 30,
		}, nil
	}

	// 步骤5：构建摘要
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("股票: %s (%s) 全球市场联动分析\n", stockName, companyID))

	if len(peerSummaries) > 0 {
		category := peerInfo.Category
		if category == "" {
			category = "同行业"
		}
		sb.WriteString(fmt.Sprintf("\n【美股对标公司】（%s同行业：%s）\n", stockName, category))
		for _, s := range peerSummaries {
			sb.WriteString(fmt.Sprintf("  %s(%s): 昨日%s 5日%s 最新收盘 %.2f\n",
				s.Name, strings.ToUpper(s.Symbol),
				globalmarket.FormatChangePct(s.DayChangePct),
				globalmarket.FormatChangePct(s.FiveDayChangePct),
				s.LatestClose))
		}
	}

	if etfSummary != nil {
		sb.WriteString(fmt.Sprintf("  行业ETF(%s): 昨日%s 5日%s\n",
			strings.ToUpper(etfSummary.Symbol),
			globalmarket.FormatChangePct(etfSummary.DayChangePct),
			globalmarket.FormatChangePct(etfSummary.FiveDayChangePct)))
	}

	if len(indexSummaries) > 0 {
		sb.WriteString("\n【全球主要指数】\n")
		for _, s := range indexSummaries {
			sb.WriteString(fmt.Sprintf("  %s: 昨日%s 5日%s\n",
				s.Name,
				globalmarket.FormatChangePct(s.DayChangePct),
				globalmarket.FormatChangePct(s.FiveDayChangePct)))
		}
	}

	// 步骤6：调用 GLM 分析
	systemPrompt := "你是一位专业的全球市场联动分析师，擅长通过美股对标公司和全球指数走势预判A股/港股个股当日表现。"
	userPrompt := fmt.Sprintf(`请基于以下全球市场数据，分析对%s的影响：

%s

请以JSON格式输出分析结果：
{
  "analysis": "全球市场联动分析（200字以内）",
  "suggestion": "操作建议（买入/持有/观望/减仓/卖出）",
  "risk_level": "高/中/低",
  "confidence": 置信度(0-100的整数)
}`, stockName, sb.String())

	raw, err := client.Chat(ctx, systemPrompt, userPrompt)
	if err != nil {
		return &AgentResult{AgentName: a.Name(), Err: err}, err
	}
	return parseAgentResponse(raw, a.Name()), nil
}
