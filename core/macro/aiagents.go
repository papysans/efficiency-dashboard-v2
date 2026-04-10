package macro

import (
	"context"
	"fmt"
	"strings"

	"comdigger/core/aiagent"
)

// MacroAgentResult 单位分析师的分析结果
type MacroAgentResult struct {
	AgentName string
	Analysis  string
}

// RunKondratieffAnalyst 康波周期分析师
func RunKondratieffAnalyst(ctx context.Context, summary MacroSummary, client *aiagent.GLMClient) MacroAgentResult {
	systemPrompt := "你是专业的康波周期研究员，擅长60年长波理论和技术革命周期分析。请基于宏观经济数据判断当前所处的康波阶段，并给出战略资产配置建议。"

	var sb strings.Builder
	sb.WriteString("GDP历史数据（最近10条，年份+同比%）：\n")
	gdps := summary.GDPs
	if len(gdps) > 10 {
		gdps = gdps[:10]
	}
	for _, d := range gdps {
		sb.WriteString(fmt.Sprintf("  %s: GDP同比=%.2f%% 总量=%.0f亿元\n", d.ReportDate, d.GDPYoY, d.TotalGDP))
	}

	sb.WriteString("\nPPI历史数据（最近10条，时间+同比%）：\n")
	ppis := summary.PPIs
	if len(ppis) > 10 {
		ppis = ppis[:10]
	}
	for _, d := range ppis {
		sb.WriteString(fmt.Sprintf("  %s: PPI同比=%.2f%%\n", d.ReportDate, d.PPIYoY))
	}

	sb.WriteString("\nM2历史数据（最近10条，时间+M2同比%）：\n")
	m2s := summary.M2s
	if len(m2s) > 10 {
		m2s = m2s[:10]
	}
	for _, d := range m2s {
		sb.WriteString(fmt.Sprintf("  %s: M2同比=%.2f%% M1同比=%.2f%%\n", d.ReportDate, d.M2Yoy, d.M1Yoy))
	}

	sb.WriteString("\n请分析：\n")
	sb.WriteString("①第五轮信息技术康波所处阶段（回升/繁荣/衰退/萧条）\n")
	sb.WriteString("②AI/新能源是否开启第六轮康波\n")
	sb.WriteString("③战略资产配置建议（股票/债券/商品/现金比例）")

	analysis, err := client.Chat(ctx, systemPrompt, sb.String())
	if err != nil {
		return MacroAgentResult{AgentName: "康波周期分析师", Analysis: "康波分析暂不可用: " + err.Error()}
	}
	return MacroAgentResult{AgentName: "康波周期分析师", Analysis: analysis}
}

// RunMerrillClockAnalyst 美林时钟分析师
func RunMerrillClockAnalyst(ctx context.Context, summary MacroSummary, client *aiagent.GLMClient) MacroAgentResult {
	systemPrompt := "你是专业的美林投资时钟分析师，擅长经济周期与资产轮动分析。请基于GDP/CPI/PMI/PPI数据判断当前美林时钟所处象限，并给出各类资产配置比例建议。"

	var sb strings.Builder

	sb.WriteString("GDP数据（最近5条）：\n")
	gdps := summary.GDPs
	if len(gdps) > 5 {
		gdps = gdps[:5]
	}
	for _, d := range gdps {
		sb.WriteString(fmt.Sprintf("  %s: GDP同比=%.2f%%\n", d.ReportDate, d.GDPYoY))
	}

	sb.WriteString("\nCPI数据（最近5条）：\n")
	cpis := summary.CPIs
	if len(cpis) > 5 {
		cpis = cpis[:5]
	}
	for _, d := range cpis {
		sb.WriteString(fmt.Sprintf("  %s: CPI同比=%.2f%% 环比=%.2f%%\n", d.ReportDate, d.NationalYoY, d.NationalSeq))
	}

	sb.WriteString("\nPMI数据（最近5条）：\n")
	pmis := summary.PMIs
	if len(pmis) > 5 {
		pmis = pmis[:5]
	}
	for _, d := range pmis {
		sb.WriteString(fmt.Sprintf("  %s: 制造业PMI=%.1f 非制造业PMI=%.1f\n", d.ReportDate, d.ManufacturingPMI, d.NonManufacturingPMI))
	}

	sb.WriteString("\nPPI数据（最近5条）：\n")
	ppis := summary.PPIs
	if len(ppis) > 5 {
		ppis = ppis[:5]
	}
	for _, d := range ppis {
		sb.WriteString(fmt.Sprintf("  %s: PPI同比=%.2f%%\n", d.ReportDate, d.PPIYoY))
	}

	sb.WriteString("\n请分析：\n")
	sb.WriteString("①经济增长方向（GDP趋势+PMI荣枯线50判断）\n")
	sb.WriteString("②通胀方向（CPI/PPI同比趋势）\n")
	sb.WriteString("③当前象限（复苏→股票/过热→商品/滞胀→现金/衰退→债券）\n")
	sb.WriteString("④各类资产配置比例")

	analysis, err := client.Chat(ctx, systemPrompt, sb.String())
	if err != nil {
		return MacroAgentResult{AgentName: "美林时钟分析师", Analysis: "美林时钟分析暂不可用: " + err.Error()}
	}
	return MacroAgentResult{AgentName: "美林时钟分析师", Analysis: analysis}
}

// RunChinaPolicyAnalyst 中国政策分析师
func RunChinaPolicyAnalyst(ctx context.Context, summary MacroSummary, client *aiagent.GLMClient) MacroAgentResult {
	systemPrompt := "你是专注中国货币政策、财政政策和产业政策的分析师。请基于M2/信贷/CPI数据判断当前政策取向，并指出受益板块方向。"

	var sb strings.Builder

	sb.WriteString("M2货币供应数据（最近5条）：\n")
	m2s := summary.M2s
	if len(m2s) > 5 {
		m2s = m2s[:5]
	}
	for _, d := range m2s {
		sb.WriteString(fmt.Sprintf("  %s: M2同比=%.2f%% M1同比=%.2f%%\n", d.ReportDate, d.M2Yoy, d.M1Yoy))
	}

	sb.WriteString("\n人民币贷款数据（最近5条）：\n")
	loans := summary.Loans
	if len(loans) > 5 {
		loans = loans[:5]
	}
	for _, d := range loans {
		sb.WriteString(fmt.Sprintf("  %s: 新增贷款=%.0f亿元 同比=%.2f%%\n", d.ReportDate, d.NewLoan, d.LoanYoY))
	}

	sb.WriteString("\nCPI数据（最近3条）：\n")
	cpis := summary.CPIs
	if len(cpis) > 3 {
		cpis = cpis[:3]
	}
	for _, d := range cpis {
		sb.WriteString(fmt.Sprintf("  %s: CPI同比=%.2f%%\n", d.ReportDate, d.NationalYoY))
	}

	sb.WriteString("\n请分析：\n")
	sb.WriteString("①货币政策取向（M2/M1增速、信贷扩张/收缩）\n")
	sb.WriteString("②财政政策力度\n")
	sb.WriteString("③政策受益板块方向")

	analysis, err := client.Chat(ctx, systemPrompt, sb.String())
	if err != nil {
		return MacroAgentResult{AgentName: "中国政策分析师", Analysis: "政策分析暂不可用: " + err.Error()}
	}
	return MacroAgentResult{AgentName: "中国政策分析师", Analysis: analysis}
}

// RunChiefMacroStrategist 首席宏观策略师
func RunChiefMacroStrategist(ctx context.Context, summary MacroSummary, prev3 []MacroAgentResult, client *aiagent.GLMClient) MacroAgentResult {
	systemPrompt := "你是首席宏观策略师，负责综合多位分析师的报告，给出最终的宏观评分和大类资产配置建议。"

	var sb strings.Builder
	sb.WriteString("以下是三位分析师的报告：\n\n")
	for _, r := range prev3 {
		sb.WriteString(fmt.Sprintf("【%s】\n%s\n\n", r.AgentName, r.Analysis))
	}

	sb.WriteString("请综合以上分析给出：\n")
	sb.WriteString("①宏观评分(0-100，0=极度悲观，100=极度乐观)\n")
	sb.WriteString("②大类资产配置建议（股票/债券/商品/现金比例及理由）\n")
	sb.WriteString("③近3-6个月操作策略")

	analysis, err := client.Chat(ctx, systemPrompt, sb.String())
	if err != nil {
		return MacroAgentResult{AgentName: "首席宏观策略师", Analysis: "首席策略分析暂不可用: " + err.Error()}
	}
	return MacroAgentResult{AgentName: "首席宏观策略师", Analysis: analysis}
}

// RunMacroAnalysis 顺序执行4位分析师
func RunMacroAnalysis(ctx context.Context, summary MacroSummary, client *aiagent.GLMClient, progressFn func(string)) []MacroAgentResult {
	results := make([]MacroAgentResult, 0, 4)

	r1 := RunKondratieffAnalyst(ctx, summary, client)
	results = append(results, r1)
	if progressFn != nil {
		progressFn("[1/4] 康波周期分析师 完成")
	}

	r2 := RunMerrillClockAnalyst(ctx, summary, client)
	results = append(results, r2)
	if progressFn != nil {
		progressFn("[2/4] 美林时钟分析师 完成")
	}

	r3 := RunChinaPolicyAnalyst(ctx, summary, client)
	results = append(results, r3)
	if progressFn != nil {
		progressFn("[3/4] 中国政策分析师 完成")
	}

	r4 := RunChiefMacroStrategist(ctx, summary, results, client)
	results = append(results, r4)
	if progressFn != nil {
		progressFn("[4/4] 首席宏观策略师 完成")
	}

	return results
}
