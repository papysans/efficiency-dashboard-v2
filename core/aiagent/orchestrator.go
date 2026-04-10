package aiagent

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

const maxConcurrency = 3

// FinalDecision 最终投资决策
type FinalDecision struct {
	Rating          string `json:"rating"`           // 强烈买入/买入/持有/卖出/强烈卖出
	TargetPrice     string `json:"target_price"`     // 目标价
	EntryRange      string `json:"entry_range"`      // 进场区间
	TakeProfit      string `json:"take_profit"`      // 止盈位
	StopLoss        string `json:"stop_loss"`        // 止损位
	HoldingPeriod   string `json:"holding_period"`   // 持有周期
	PositionSize    string `json:"position_size"`    // 建议仓位
	RiskWarning     string `json:"risk_warning"`     // 风险提示
	OperationAdvice string `json:"operation_advice"` // 操作建议
	Confidence      int    `json:"confidence"`       // 置信度 0-100
}

// AnalysisReport 完整分析报告
type AnalysisReport struct {
	CompanyID    string
	StockName    string
	AnalysisDate time.Time
	AgentResults []AgentResult
	Discussion   string
	Decision     *FinalDecision
}

// RunAnalysis 并行运行7位分析师，生成完整分析报告
func RunAnalysis(ctx context.Context, db *sql.DB, companyID, stockName string, client *GLMClient, progressFn func(string)) (*AnalysisReport, error) {
	agents := []Agent{
		&TechnicalAgent{},
		&FundamentalAgent{},
		&FundFlowAgent{},
		&RiskAgent{},
		&SentimentAgent{},
		&NewsAgent{},
		&GlobalMarketAgent{},
	}

	// 任务队列模式：semaphore 控制并发数
	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make([]AgentResult, 0, len(agents))

	for _, agent := range agents {
		wg.Add(1)
		go func(a Agent) {
			defer wg.Done()
			// 获取信号量
			sem <- struct{}{}
			defer func() { <-sem }()

			select {
			case <-ctx.Done():
				mu.Lock()
				results = append(results, AgentResult{AgentName: a.Name(), Err: ctx.Err()})
				mu.Unlock()
				return
			default:
			}

			if progressFn != nil {
				progressFn(fmt.Sprintf("正在运行 %s...", a.Name()))
			}

			result, err := a.Analyze(ctx, db, companyID, stockName, client)
			if result == nil {
				result = &AgentResult{AgentName: a.Name()}
			}
			if err != nil {
				result.Err = err
			}

			if progressFn != nil {
				progressFn(fmt.Sprintf("%s 完成", a.Name()))
			}

			mu.Lock()
			results = append(results, *result)
			mu.Unlock()
		}(agent)
	}

	wg.Wait()

	// 团队讨论
	if progressFn != nil {
		progressFn("正在进行团队讨论...")
	}
	discussion, err := conductDiscussion(ctx, results, stockName, client)
	if err != nil {
		discussion = "团队讨论失败: " + err.Error()
	}

	// 最终决策
	if progressFn != nil {
		progressFn("正在生成最终决策...")
	}
	decision, err := makeFinalDecision(ctx, discussion, stockName, client)
	if err != nil {
		decision = fallbackDecision(results)
	}

	return &AnalysisReport{
		CompanyID:    companyID,
		StockName:    stockName,
		AnalysisDate: time.Now(),
		AgentResults: results,
		Discussion:   discussion,
		Decision:     decision,
	}, nil
}

// conductDiscussion 将6位分析师结果汇总，模拟团队讨论
func conductDiscussion(ctx context.Context, results []AgentResult, stockName string, client *GLMClient) (string, error) {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("关于股票「%s」的分析师团队讨论：\n\n", stockName))

	for _, r := range results {
		if r.Err != nil {
			sb.WriteString(fmt.Sprintf("【%s】分析失败: %v\n\n", r.AgentName, r.Err))
			continue
		}
		sb.WriteString(fmt.Sprintf("【%s】\n", r.AgentName))
		sb.WriteString(fmt.Sprintf("分析: %s\n", r.Analysis))
		sb.WriteString(fmt.Sprintf("建议: %s  风险: %s  置信度: %d%%\n\n", r.Suggestion, r.RiskLevel, r.Confidence))
	}

	systemPrompt := "你是一位资深的投资决策协调人，负责综合多位分析师的意见，形成团队共识。"
	userPrompt := sb.String() + "\n请综合以上各位分析师的意见，进行团队讨论，形成对该股票的综合判断（300字以内）。"

	return client.Chat(ctx, systemPrompt, userPrompt)
}

// makeFinalDecision 基于讨论结果生成最终投资决策
func makeFinalDecision(ctx context.Context, discussion, stockName string, client *GLMClient) (*FinalDecision, error) {
	systemPrompt := "你是一位经验丰富的首席投资策略师，负责基于团队讨论结果做出最终投资决策。"
	userPrompt := fmt.Sprintf(`基于以下团队讨论结果，为股票「%s」生成最终投资决策：

%s

请严格以JSON格式输出（不要有其他文字）：
{
  "rating": "买入",
  "target_price": "目标价格或区间",
  "entry_range": "建议进场价格区间",
  "take_profit": "止盈位",
  "stop_loss": "止损位",
  "holding_period": "短期/中期/长期",
  "position_size": "建议仓位比例",
  "risk_warning": "主要风险提示",
  "operation_advice": "具体操作建议",
  "confidence": 综合置信度(0-100的整数)
}`, stockName, discussion)

	raw, err := client.Chat(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	// 提取JSON
	jsonStr := raw
	if idx := strings.Index(raw, "{"); idx >= 0 {
		if end := strings.LastIndex(raw, "}"); end > idx {
			jsonStr = raw[idx : end+1]
		}
	}

	var decision FinalDecision
	if err := json.Unmarshal([]byte(jsonStr), &decision); err != nil {
		// 降级：关键词匹配评级
		return fallbackDecisionFromText(raw), nil
	}
	return &decision, nil
}

// fallbackDecision 基于分析师结果的降级决策
func fallbackDecision(results []AgentResult) *FinalDecision {
	buyCount, sellCount := 0, 0
	for _, r := range results {
		switch r.Suggestion {
		case "买入", "强烈买入":
			buyCount++
		case "卖出", "减仓", "强烈卖出":
			sellCount++
		}
	}

	rating := "持有"
	if buyCount > sellCount && buyCount >= 3 {
		rating = "买入"
	} else if sellCount > buyCount && sellCount >= 3 {
		rating = "卖出"
	}

	return &FinalDecision{
		Rating:          rating,
		TargetPrice:     "参考市价",
		HoldingPeriod:   "中期",
		PositionSize:    "5%-10%",
		RiskWarning:     "市场有风险，投资需谨慎",
		OperationAdvice: "请综合各分析师意见自行判断",
		Confidence:      50,
	}
}

// fallbackDecisionFromText 从文本中关键词匹配评级
func fallbackDecisionFromText(text string) *FinalDecision {
	rating := "持有"
	if strings.Contains(text, "强烈买入") {
		rating = "强烈买入"
	} else if strings.Contains(text, "买入") {
		rating = "买入"
	} else if strings.Contains(text, "强烈卖出") {
		rating = "强烈卖出"
	} else if strings.Contains(text, "卖出") || strings.Contains(text, "减仓") {
		rating = "卖出"
	}
	return &FinalDecision{
		Rating:          rating,
		TargetPrice:     "参考市价",
		HoldingPeriod:   "中期",
		PositionSize:    "5%-10%",
		RiskWarning:     "市场有风险，投资需谨慎",
		OperationAdvice: text,
		Confidence:      50,
	}
}
