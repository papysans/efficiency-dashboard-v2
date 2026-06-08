package util

import (
	"strings"

	"kanban/kbcli/internal/appconfig"
)

// CalculateCost 根据模型和 Token 数计算调用成本。
// 功能：通过模型前缀匹配价格表，计算 (inputTokens * InPrice + outputTokens * OutPrice) / 1e6 的总成本。
// 参数：
//   - model: 模型名称（大小写不敏感）。
//   - inTokens: 上游（输入）Token 数量。
//   - outTokens: 下游（输出）Token 数量。
//   - prices: 模型价格映射表，key 为模型前缀或 "default"。
//
// 返回值：计算后的成本（货币单位）。
// 关键技术原理：前缀最长匹配策略——遍历价格表，找所有为 model 前缀的 key，取长度最长的一个；无匹配则回退到 default；仍未找到则返回 0。
func CalculateCost(model string, inTokens, outTokens int64, prices map[string]appconfig.ModelPrice) float64 {
	model = strings.ToLower(model)
	var price appconfig.ModelPrice
	// 前缀匹配：找 prices 中为 model 前缀的 key，取最长匹配
	var bestKey string
	for k := range prices {
		if k != "default" && strings.HasPrefix(model, k) {
			if len(k) > len(bestKey) {
				bestKey = k
			}
		}
	}
	if bestKey != "" {
		price = prices[bestKey]
	} else {
		// 无匹配前缀时回退到 default 价格
		var ok bool
		price, ok = prices["default"]
		if !ok {
			return 0
		}
	}

	// 成本公式：按百万 Token 计价
	return (float64(inTokens)/1e6)*price.InPrice + (float64(outTokens)/1e6)*price.OutPrice
}
