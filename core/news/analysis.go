package news

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"comdigger/core/aiagent"
)

// AIAnalysisResult AI 分析结果
type AIAnalysisResult struct {
	Summary     string   `json:"summary"`
	Keywords    []string `json:"keywords"`
	Sentiment   string   `json:"sentiment"`
	Suggestions []string `json:"suggestions"`
}

// RunAIAnalysis 对最近新闻进行 AI 分析
func RunAIAnalysis(db *sql.DB, limit int) (*AIAnalysisResult, error) {
	if limit <= 0 {
		limit = 50
	}

	// 加载新闻
	items, err := LoadNews(db, limit, 0)
	if err != nil {
		return nil, fmt.Errorf("加载新闻失败: %w", err)
	}

	if len(items) == 0 {
		return &AIAnalysisResult{
			Summary:     "暂无新闻数据",
			Keywords:    []string{},
			Sentiment:   "中性",
			Suggestions: []string{"建议先运行新闻抓取获取数据"},
		}, nil
	}

	// 构建 AI 分析提示词
	var titles []string
	for i, item := range items {
		if i >= 20 { // 最多使用 20 条新闻
			break
		}
		titles = append(titles, item.Title)
	}

	prompt := fmt.Sprintf(`请分析以下财经新闻标题，总结市场情绪和热点：

%s

请从以下几个维度分析：
1. 市场情绪（积极/中性/消极）
2. 主要热点关键词
3. 简要总结
4. 投资建议

请用 JSON 格式返回，格式如下：
{
  "summary": "简要总结",
  "keywords": ["关键词1", "关键词2"],
  "sentiment": "积极/中性/消极",
  "suggestions": ["建议1", "建议2"]
}`, strings.Join(titles, "\n"))

	// 调用 AI
	client, err := aiagent.NewClientFromEnv("")
	if err != nil {
		return nil, fmt.Errorf("初始化 AI 客户端失败: %w", err)
	}

	ctx := context.Background()
	systemPrompt := "你是一位专业的财经分析师，擅长分析市场新闻和情绪。"
	response, err := client.Chat(ctx, systemPrompt, prompt)
	if err != nil {
		return nil, fmt.Errorf("AI 分析失败: %w", err)
	}

	// 简单解析 JSON 响应（实际项目可能需要更健壮的解析）
	result := &AIAnalysisResult{
		Summary:     response,
		Keywords:    []string{},
		Sentiment:   "中性",
		Suggestions: []string{},
	}

	return result, nil
}
