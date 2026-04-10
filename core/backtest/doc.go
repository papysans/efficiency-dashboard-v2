// Package backtest 提供策略回测引擎。
//
// 唯一职责：
//   - RunBacktest：执行技术指标策略回测
//   - 回测结果统计和分析
//
// 禁止事项：
//   - 禁止在其他包中重新实现回测逻辑
package backtest
