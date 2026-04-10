package news

import (
	"sort"
	"strings"
	"unicode"
)

// CalcFlowScore 计算多平台流量分数，返回市场情绪综合结果
func CalcFlowScore(platformData map[string][]NewsItem) MarketSentiment {
	var flowScores []FlowScore
	var totalRaw float64
	var allItems []NewsItem

	for platform, items := range platformData {
		cfg, ok := PlatformConfigs[platform]
		if !ok {
			continue
		}
		count := len(items)
		raw := cfg.Weight * cfg.CategoryWeight * float64(count)
		totalRaw += raw
		flowScores = append(flowScores, FlowScore{
			Platform: cfg.Name,
			Score:    raw,
			Count:    count,
		})
		allItems = append(allItems, items...)
	}

	// 归一化到 0-1000
	normalized := totalRaw / 50.0
	if normalized > 1000 {
		normalized = 1000
	}

	// 等级划分
	level := scoreLevel(normalized)

	// 按分数降序排列平台
	sort.Slice(flowScores, func(i, j int) bool {
		return flowScores[i].Score > flowScores[j].Score
	})
	// 为每个平台设置等级
	for i := range flowScores {
		flowScores[i].Level = scoreLevel(flowScores[i].Score / 50.0)
	}

	// 提取热词
	keywords := ExtractHotKeywords(allItems)

	return MarketSentiment{
		TotalScore:   normalized,
		Level:        level,
		TopPlatforms: flowScores,
		HotKeywords:  keywords,
	}
}

// scoreLevel 根据归一化分数返回等级
func scoreLevel(score float64) string {
	switch {
	case score >= 800:
		return "极高"
	case score >= 500:
		return "高"
	case score >= 200:
		return "中"
	default:
		return "低"
	}
}

// FindStockRelatedNews 从新闻列表中过滤与股票相关的新闻
func FindStockRelatedNews(items []NewsItem, stockName string) []NewsItem {
	if stockName == "" {
		return nil
	}
	var result []NewsItem
	for _, item := range items {
		if strings.Contains(item.Title, stockName) || strings.Contains(item.Content, stockName) {
			result = append(result, item)
		}
	}
	return result
}

// ExtractHotKeywords 从新闻标题中提取高频关键词（前20个）
// 简单按标点/空格切分，统计2字及以上词语频率，不依赖jieba
func ExtractHotKeywords(allItems []NewsItem) []string {
	freq := make(map[string]int)

	for _, item := range allItems {
		words := tokenize(item.Title)
		for _, w := range words {
			if len([]rune(w)) >= 2 {
				freq[w]++
			}
		}
	}

	// 按频率排序
	type kv struct {
		Key   string
		Value int
	}
	var sorted []kv
	for k, v := range freq {
		if v >= 2 { // 至少出现2次才算热词
			sorted = append(sorted, kv{k, v})
		}
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Value > sorted[j].Value
	})

	// 取前20个
	limit := 20
	if len(sorted) < limit {
		limit = len(sorted)
	}
	result := make([]string, limit)
	for i := 0; i < limit; i++ {
		result[i] = sorted[i].Key
	}
	return result
}

// tokenize 简单分词：按标点、空格、数字分割，保留中文词和英文词
func tokenize(text string) []string {
	var words []string
	var current strings.Builder

	runes := []rune(text)
	for _, r := range runes {
		if unicode.IsLetter(r) || unicode.Is(unicode.Han, r) {
			current.WriteRune(r)
		} else {
			if current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
		}
	}
	if current.Len() > 0 {
		words = append(words, current.String())
	}
	return words
}
