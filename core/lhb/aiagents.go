package lhb

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"comdigger/core/aiagent"
)

// LHBSummary 龙虎榜分析摘要
type LHBSummary struct {
	Date        string
	TopStocks   []StockScore   // 评分TOP20
	TopYouzi    map[string]int // 游资名→出现次数，前10名
	HotConcepts []string       // 命中HotConceptKeywords的关键词，去重
	RiskStocks  []StockScore   // SellScore < 8 的股票（卖压>50%）
	TotalCount  int            // len(scores)
}

// BuildSummary 构建龙虎榜分析摘要
func BuildSummary(scores []StockScore, records []LHBRecord) LHBSummary {
	summary := LHBSummary{
		TotalCount: len(scores),
		TopYouzi:   make(map[string]int),
	}

	// TopStocks：取前20
	if len(scores) <= 20 {
		summary.TopStocks = scores
	} else {
		summary.TopStocks = scores[:20]
	}

	// TopYouzi：统计游资频次，取前10
	youziCount := make(map[string]int)
	for _, r := range records {
		if r.YouziName != "" {
			youziCount[r.YouziName]++
		}
	}
	type youziEntry struct {
		name  string
		count int
	}
	entries := make([]youziEntry, 0, len(youziCount))
	for name, cnt := range youziCount {
		entries = append(entries, youziEntry{name, cnt})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].count > entries[j].count
	})
	limit := 10
	if len(entries) < limit {
		limit = len(entries)
	}
	for _, e := range entries[:limit] {
		summary.TopYouzi[e.name] = e.count
	}

	// HotConcepts：从 records 的 Name 字段检查热门概念关键词，去重
	conceptSeen := make(map[string]bool)
	for _, r := range records {
		for _, kw := range HotConceptKeywords {
			if !conceptSeen[kw] && strings.Contains(r.Name, kw) {
				conceptSeen[kw] = true
				summary.HotConcepts = append(summary.HotConcepts, kw)
			}
		}
	}

	// RiskStocks：SellScore < 8
	for _, s := range scores {
		if s.SellScore < 8 {
			summary.RiskStocks = append(summary.RiskStocks, s)
		}
	}

	return summary
}

// LHBAgentResult 单位分析师的分析结果
type LHBAgentResult struct {
	AgentName string
	Analysis  string
}

// RunYouziAnalyst 游资行为分析师
func RunYouziAnalyst(ctx context.Context, summary LHBSummary, client *aiagent.GLMClient) LHBAgentResult {
	systemPrompt := "你是专业的游资行为分析师，擅长分析龙虎榜中游资的操作意图和联合行为。"

	var sb strings.Builder
	sb.WriteString("今日龙虎榜游资频次统计（前10名）：\n")
	// 按频次排序输出
	type youziEntry struct {
		name  string
		count int
	}
	entries := make([]youziEntry, 0, len(summary.TopYouzi))
	for name, cnt := range summary.TopYouzi {
		entries = append(entries, youziEntry{name, cnt})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].count > entries[j].count
	})
	for _, e := range entries {
		sb.WriteString(fmt.Sprintf("  %s: %d次\n", e.name, e.count))
	}

	sb.WriteString("\n各股票顶级游资情况：\n")
	for _, s := range summary.TopStocks {
		if len(s.TopYouziNames) > 0 {
			sb.WriteString(fmt.Sprintf("  %s(%s): %s\n", s.Name, s.Symbol, strings.Join(s.TopYouziNames, "、")))
		}
	}

	sb.WriteString("\n请分析：\n1. 游资操作意图\n2. 联合操作识别\n3. 今日游资风格评估")

	analysis, err := client.Chat(ctx, systemPrompt, sb.String())
	if err != nil {
		analysis = "游资分析暂不可用: " + err.Error()
	}
	return LHBAgentResult{AgentName: "游资行为分析师", Analysis: analysis}
}

// RunPotentialAnalyst 个股潜力分析师
func RunPotentialAnalyst(ctx context.Context, summary LHBSummary, client *aiagent.GLMClient) LHBAgentResult {
	systemPrompt := "你是专业的个股潜力分析师，擅长从龙虎榜数据挖掘次日潜力股，给出具体的买入区间、目标价和止损位。"

	var sb strings.Builder
	sb.WriteString("今日龙虎榜TOP20股票数据：\n")
	for _, s := range summary.TopStocks {
		institutionStr := "否"
		if s.HasInstitution {
			institutionStr = "是"
		}
		youziStr := strings.Join(s.TopYouziNames, "、")
		if youziStr == "" {
			youziStr = "无"
		}
		sb.WriteString(fmt.Sprintf("  %s(%s) 总分:%.1f 席位数:%d 机构:%s 游资:%s\n",
			s.Name, s.Symbol, s.TotalScore, s.SeatCount, institutionStr, youziStr))
	}
	sb.WriteString("\n请给出TOP5-8次日潜力股，每只股票包含：买入区间、目标价、止损位。")

	analysis, err := client.Chat(ctx, systemPrompt, sb.String())
	if err != nil {
		analysis = "个股潜力分析暂不可用: " + err.Error()
	}
	return LHBAgentResult{AgentName: "个股潜力分析师", Analysis: analysis}
}

// RunThemeAnalyst 题材追踪分析师
func RunThemeAnalyst(ctx context.Context, summary LHBSummary, client *aiagent.GLMClient) LHBAgentResult {
	systemPrompt := "你是专业的题材追踪分析师，擅长识别龙虎榜中的热点题材和板块轮动方向。"

	var sb strings.Builder
	sb.WriteString("今日龙虎榜命中热门概念关键词：\n")
	for _, kw := range summary.HotConcepts {
		// 反查哪些TopStocks的Name命中了该关键词
		var matched []string
		for _, s := range summary.TopStocks {
			if strings.Contains(s.Name, kw) {
				matched = append(matched, s.Name)
			}
		}
		if len(matched) > 0 {
			sb.WriteString(fmt.Sprintf("  【%s】: %s\n", kw, strings.Join(matched, "、")))
		} else {
			sb.WriteString(fmt.Sprintf("  【%s】\n", kw))
		}
	}
	if len(summary.HotConcepts) == 0 {
		sb.WriteString("  （今日无明显热门概念命中）\n")
	}

	sb.WriteString("\n请分析：\n1. 热门题材排名\n2. 板块轮动方向\n3. 萌芽期题材识别")

	analysis, err := client.Chat(ctx, systemPrompt, sb.String())
	if err != nil {
		analysis = "题材分析暂不可用: " + err.Error()
	}
	return LHBAgentResult{AgentName: "题材追踪分析师", Analysis: analysis}
}

// RunRiskAnalyst 风险控制专家
func RunRiskAnalyst(ctx context.Context, summary LHBSummary, client *aiagent.GLMClient) LHBAgentResult {
	systemPrompt := "你是专业的风险控制专家，擅长识别龙虎榜中的出货信号和异常卖压，保护投资者资金安全。"

	var sb strings.Builder
	sb.WriteString("今日龙虎榜高卖压风险股票（卖压分<8）：\n")
	for _, s := range summary.RiskStocks {
		sb.WriteString(fmt.Sprintf("  %s(%s) 卖压分:%.1f 总分:%.1f\n",
			s.Name, s.Symbol, s.SellScore, s.TotalScore))
	}
	if len(summary.RiskStocks) == 0 {
		sb.WriteString("  （今日无明显高卖压风险股票）\n")
	}

	sb.WriteString("\n请给出：\n1. TOP3-5风险股\n2. 出货信号识别\n3. 仓位建议")

	analysis, err := client.Chat(ctx, systemPrompt, sb.String())
	if err != nil {
		analysis = "风险分析暂不可用: " + err.Error()
	}
	return LHBAgentResult{AgentName: "风险控制专家", Analysis: analysis}
}

// RunChiefStrategist 首席策略师（综合前4位分析师意见）
func RunChiefStrategist(ctx context.Context, summary LHBSummary, prev4 []LHBAgentResult, client *aiagent.GLMClient) LHBAgentResult {
	systemPrompt := "你是首席投资策略师，综合多位专业分析师的意见，给出最终的市场判断和投资决策建议。"

	var sb strings.Builder
	sb.WriteString("以下是各位专业分析师的分析报告：\n\n")
	for _, r := range prev4 {
		sb.WriteString(fmt.Sprintf("【%s】\n%s\n\n", r.AgentName, r.Analysis))
	}

	sb.WriteString(fmt.Sprintf("今日整体统计：\n  上榜股票总数: %d\n  TOP股票数: %d\n  高风险股票数: %d\n\n",
		summary.TotalCount, len(summary.TopStocks), len(summary.RiskStocks)))

	sb.WriteString("请综合以上分析，给出：\n1. 市场情绪评分(0-100)\n2. 最终TOP5推荐股（含完整交易参数：买入区间/目标价/止损位）")

	analysis, err := client.Chat(ctx, systemPrompt, sb.String())
	if err != nil {
		analysis = "首席策略分析暂不可用: " + err.Error()
	}
	return LHBAgentResult{AgentName: "首席策略师", Analysis: analysis}
}

// RunLHBAnalysis 顺序执行5位分析师，返回分析结果切片
func RunLHBAnalysis(ctx context.Context, scores []StockScore, records []LHBRecord, client *aiagent.GLMClient, progressFn func(string)) []LHBAgentResult {
	summary := BuildSummary(scores, records)

	results := make([]LHBAgentResult, 0, 5)

	r1 := RunYouziAnalyst(ctx, summary, client)
	results = append(results, r1)
	if progressFn != nil {
		progressFn("[1/5] 游资行为分析师 完成")
	}

	r2 := RunPotentialAnalyst(ctx, summary, client)
	results = append(results, r2)
	if progressFn != nil {
		progressFn("[2/5] 个股潜力分析师 完成")
	}

	r3 := RunThemeAnalyst(ctx, summary, client)
	results = append(results, r3)
	if progressFn != nil {
		progressFn("[3/5] 题材追踪分析师 完成")
	}

	r4 := RunRiskAnalyst(ctx, summary, client)
	results = append(results, r4)
	if progressFn != nil {
		progressFn("[4/5] 风险控制专家 完成")
	}

	r5 := RunChiefStrategist(ctx, summary, results, client)
	results = append(results, r5)
	if progressFn != nil {
		progressFn("[5/5] 首席策略师 完成")
	}

	return results
}
