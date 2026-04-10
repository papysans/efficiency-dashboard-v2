// Package query 提供财务数据查询功能。
//
// 唯一职责：
//   - BuildReportTypeFilter：构建报告类型 SQL 过滤条件（全项目唯一入口）
//   - 财务数据的多维度查询
//
// 禁止事项：
//   - 禁止在 comdig/ 或 backend/ 中重新实现报告类型过滤逻辑
package query
