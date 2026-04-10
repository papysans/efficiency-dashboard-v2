package sector

import (
	"context"
	"fmt"
	"strings"

	"comdigger/core/aiagent"
)

// SectorAgentResult 单位分析师的分析结果
type SectorAgentResult struct {
	AgentName string
	Analysis  string
}

// SectorSummary 聚合所有输入数据
type SectorSummary struct {
	MarketOverviews   []MarketOverview // 4个指数概况
	TopRisingSectors  []SectorInfo     // 涨幅前15行业板块
	TopFallingSectors []SectorInfo     // 跌幅前15行业板块
	TopConceptSectors []SectorInfo     // 涨幅前15概念板块
	TopInflowSectors  []SectorFundFlow // 主力净流入前15行业
	TopOutflowSectors []SectorFundFlow // 主力净流出前10行业
	LatestNews        []FinNews        // 最新20条财经新闻
}

// RunMacroStrategist 宏观策略师
func RunMacroStrategist(ctx context.Context, summary SectorSummary, client *aiagent.GLMClient) SectorAgentResult {
	systemPrompt := "你是专业的宏观策略师，专注于宏观环境分析、政策导向解读和板块轮动预判。请基于市场数据给出简洁、专业的分析。"

	var sb strings.Builder
	sb.WriteString("今日市场指数概况：\n")
	for _, m := range summary.MarketOverviews {
		sb.WriteString(fmt.Sprintf("  %s(%s): 最新价%.2f 涨跌幅%.2f%% 振幅%.2f%%\n",
			m.Name, m.Code, m.Price, m.ChangePct, m.Amplitude))
	}

	sb.WriteString("\n最新财经公告（前20条标题）：\n")
	for i, n := range summary.LatestNews {
		if i >= 20 {
			break
		}
		sb.WriteString(fmt.Sprintf("  [%s] %s\n", n.Time, n.Title))
	}

	sb.WriteString("\n请分析：\n1. 宏观环境判断\n2. 政策导向解读\n3. 板块轮动预判")

	analysis, err := client.Chat(ctx, systemPrompt, sb.String())
	if err != nil {
		analysis = "宏观分析暂不可用: " + err.Error()
	}
	return SectorAgentResult{AgentName: "宏观策略师", Analysis: analysis}
}

// RunSectorDiagnostician 板块诊断师
func RunSectorDiagnostician(ctx context.Context, summary SectorSummary, client *aiagent.GLMClient) SectorAgentResult {
	systemPrompt := "你是专业的板块诊断师，专注于板块强弱分析、轮动特征识别和主线方向判断。"

	var sb strings.Builder
	sb.WriteString("今日行业板块涨幅前15：\n")
	for i, s := range summary.TopRisingSectors {
		sb.WriteString(fmt.Sprintf("  %d. %s 涨跌幅:%.2f%% 领涨股:%s 上涨:%d家 下跌:%d家\n",
			i+1, s.Name, s.ChangePct, s.LeadStockName, s.RisingCount, s.FallingCount))
	}

	sb.WriteString("\n今日行业板块跌幅前15：\n")
	for i, s := range summary.TopFallingSectors {
		sb.WriteString(fmt.Sprintf("  %d. %s 涨跌幅:%.2f%% 领涨股:%s 上涨:%d家 下跌:%d家\n",
			i+1, s.Name, s.ChangePct, s.LeadStockName, s.RisingCount, s.FallingCount))
	}

	sb.WriteString("\n今日概念板块涨幅前15：\n")
	for i, s := range summary.TopConceptSectors {
		sb.WriteString(fmt.Sprintf("  %d. %s 涨跌幅:%.2f%% 领涨股:%s\n",
			i+1, s.Name, s.ChangePct, s.LeadStockName))
	}

	sb.WriteString("\n请分析：\n1. 板块强弱对比\n2. 轮动特征识别\n3. 今日主线方向")

	analysis, err := client.Chat(ctx, systemPrompt, sb.String())
	if err != nil {
		analysis = "板块诊断暂不可用: " + err.Error()
	}
	return SectorAgentResult{AgentName: "板块诊断师", Analysis: analysis}
}

// RunFundFlowAnalyst 资金流向分析师
func RunFundFlowAnalyst(ctx context.Context, summary SectorSummary, client *aiagent.GLMClient) SectorAgentResult {
	systemPrompt := "你是专业的资金流向分析师，专注于主力资金意图分析和资金轮动路径追踪。"

	var sb strings.Builder
	sb.WriteString("今日行业主力净流入前15（单位：亿元）：\n")
	for i, f := range summary.TopInflowSectors {
		sb.WriteString(fmt.Sprintf("  %d. %s 主力净流入:%.2f亿 占比:%.2f%% 领涨股:%s\n",
			i+1, f.Name, f.MainNetInflow/1e8, f.MainNetInflowRate, f.LeadStockName))
	}

	sb.WriteString("\n今日行业主力净流出前10（单位：亿元）：\n")
	for i, f := range summary.TopOutflowSectors {
		sb.WriteString(fmt.Sprintf("  %d. %s 主力净流入:%.2f亿 占比:%.2f%% 领涨股:%s\n",
			i+1, f.Name, f.MainNetInflow/1e8, f.MainNetInflowRate, f.LeadStockName))
	}

	sb.WriteString("\n请分析：\n1. 主力资金意图\n2. 资金轮动路径\n3. 重点关注板块")

	analysis, err := client.Chat(ctx, systemPrompt, sb.String())
	if err != nil {
		analysis = "资金分析暂不可用: " + err.Error()
	}
	return SectorAgentResult{AgentName: "资金流向分析师", Analysis: analysis}
}

// RunSentimentDecoder 市场情绪解码员
func RunSentimentDecoder(ctx context.Context, summary SectorSummary, client *aiagent.GLMClient) SectorAgentResult {
	systemPrompt := "你是专业的市场情绪解码员，专注于市场情绪量化评估、恐贪指数分析和短期情绪预判。"

	var sb strings.Builder
	sb.WriteString("今日市场指数涨跌：\n")
	for _, m := range summary.MarketOverviews {
		sb.WriteString(fmt.Sprintf("  %s: %.2f%%\n", m.Name, m.ChangePct))
	}

	// 统计上涨/下跌板块数量
	risingCount := 0
	fallingCount := 0
	for _, s := range summary.TopRisingSectors {
		risingCount += s.RisingCount
		fallingCount += s.FallingCount
	}
	for _, s := range summary.TopFallingSectors {
		risingCount += s.RisingCount
		fallingCount += s.FallingCount
	}
	sb.WriteString(fmt.Sprintf("\n板块内个股统计：上涨 %d 家，下跌 %d 家\n", risingCount, fallingCount))

	// 极端板块（涨跌幅>5%或<-5%）
	sb.WriteString("\n极端涨跌板块（>5%或<-5%）：\n")
	extremeCount := 0
	for _, s := range summary.TopRisingSectors {
		if s.ChangePct > 5 {
			sb.WriteString(fmt.Sprintf("  强势: %s +%.2f%%\n", s.Name, s.ChangePct))
			extremeCount++
		}
	}
	for _, s := range summary.TopFallingSectors {
		if s.ChangePct < -5 {
			sb.WriteString(fmt.Sprintf("  弱势: %s %.2f%%\n", s.Name, s.ChangePct))
			extremeCount++
		}
	}
	if extremeCount == 0 {
		sb.WriteString("  （无极端涨跌板块）\n")
	}

	sb.WriteString("\n请给出：\n1. 情绪评分(0-100)\n2. 恐贪指数\n3. 短期情绪预判")

	analysis, err := client.Chat(ctx, systemPrompt, sb.String())
	if err != nil {
		analysis = "情绪分析暂不可用: " + err.Error()
	}
	return SectorAgentResult{AgentName: "市场情绪解码员", Analysis: analysis}
}

// RunSectorAnalysis 顺序执行4位分析师
func RunSectorAnalysis(ctx context.Context, summary SectorSummary, client *aiagent.GLMClient, progressFn func(string)) []SectorAgentResult {
	results := make([]SectorAgentResult, 0, 4)

	r1 := RunMacroStrategist(ctx, summary, client)
	results = append(results, r1)
	if progressFn != nil {
		progressFn("[1/4] 宏观策略师 完成")
	}

	r2 := RunSectorDiagnostician(ctx, summary, client)
	results = append(results, r2)
	if progressFn != nil {
		progressFn("[2/4] 板块诊断师 完成")
	}

	r3 := RunFundFlowAnalyst(ctx, summary, client)
	results = append(results, r3)
	if progressFn != nil {
		progressFn("[3/4] 资金流向分析师 完成")
	}

	r4 := RunSentimentDecoder(ctx, summary, client)
	results = append(results, r4)
	if progressFn != nil {
		progressFn("[4/4] 市场情绪解码员 完成")
	}

	return results
}

// RunAIAnalysis 执行板块 AI 分析（HTTP handler 使用的入口）
func RunAIAnalysis() ([]SectorAgentResult, error) {
	// 1. 获取板块数据
	summary, err := FetchSector()
	if err != nil {
		return nil, fmt.Errorf("获取板块数据失败: %w", err)
	}

	// 2. 初始化 AI 客户端
	client, err := aiagent.NewClientFromEnv("")
	if err != nil {
		return nil, fmt.Errorf("初始化 AI 客户端失败: %w", err)
	}

	// 3. 执行分析
	ctx := context.Background()
	results := RunSectorAnalysis(ctx, *summary, client, nil)

	return results, nil
}
