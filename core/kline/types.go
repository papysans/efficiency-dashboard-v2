package kline

import "time"

// KlineBar K线数据条目
type KlineBar struct {
	Time   time.Time
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume int64
}

// 频率常量
const (
	FreqDay   = "1d"
	FreqWeek  = "1w"
	FreqMonth = "1M"
)

// 数据源常量
const (
	SiteSina    = "k.sina"
	SiteTencent = "k.tencent"
)
