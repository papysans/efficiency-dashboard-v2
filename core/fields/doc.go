// Package fields 提供字段元数据管理（字段定义、别名、公式）。
//
// 唯一职责：
//   - FieldTitleManager：字段元数据的内存缓存和 DB 操作（单例）
//   - GetManager()：获取全局单例
//   - 字段别名映射（alias.go）
//   - 公式解析（formula.go）
//
// 禁止事项：
//   - 禁止在 comdig/ 或 backend/ 中直接查询 field_title 表
//   - 所有字段元数据操作必须通过 GetManager()
package fields
