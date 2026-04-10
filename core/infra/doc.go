// Package infra 提供项目基础设施：数据库连接、日志、配置加载。
//
// 唯一职责：
//   - InitDB：初始化数据库连接（全项目唯一入口）
//   - UpsertFin：财务数据写入（全项目唯一入口）
//   - Company/Fin 数据模型定义
//   - CustomLogger：统一日志
//   - SearchCompanyInDB：按代码/名称查找公司
//
// 禁止事项：
//   - 禁止在 comdig/ 或 backend/ 中重新定义 Company、Fin 结构体
//   - 禁止在其他地方重新实现数据库连接逻辑
//   - 禁止在其他地方重新实现公司搜索逻辑（已有 SearchCompanyInDB）
package infra
