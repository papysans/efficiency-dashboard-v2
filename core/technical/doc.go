// Package technical 提供技术指标计算和交易信号生成。
//
// 唯一职责：
//   - CalcMACD/CalcKDJ/CalcRSI 等：各技术指标计算
//   - GenerateSignals：生成综合交易信号
//   - Score：综合评分（-100 到 +100）
//
// 禁止事项：
//   - 禁止在其他包中重新实现 MACD、KDJ、RSI 等指标算法
//   - 所有技术分析必须通过此包
package technical
