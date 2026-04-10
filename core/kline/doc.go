// Package kline 提供 A股 K线数据的获取、存储和查询。
//
// 唯一职责：
//   - FetchKline/FetchKlineFromSina/FetchKlineFromTencent：从数据源获取 K线
//   - UpsertKlineData：增量写入 K线数据到 DB
//   - LoadKlineFromDB：从 DB 加载 K线数据
//   - KlineBar：K线数据结构体
//
// 禁止事项：
//   - 禁止在其他包中直接调用新浪/腾讯 K线 API
//   - 所有 K线数据操作必须通过此包
package kline
