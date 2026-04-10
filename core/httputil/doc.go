// Package httputil 提供统一的 HTTP 客户端。
//
// 唯一职责：
//   - DefaultClient：通用 HTTP 客户端（30s 超时），用于大多数 API 请求
//   - FastClient：快速 HTTP 客户端（10s 超时），用于 K线等实时数据
//
// 禁止事项：
//   - 禁止在其他包中直接 new(http.Client) 或 &http.Client{...}
//   - 所有对外 HTTP 请求必须复用此包的客户端实例
package httputil
