package main

const (
	MsPerWorkDay         = 28800000 // 8小时/天(毫秒)
	ProcessTimeGapMs     = 600000   // process_time合并间隔(10分钟)
	DefaultDailyRate     = 400.0    // 日费率(元)
	ESMaxSearchSize      = 10000    // ES搜索最大size
	ESTaskIndexPrefix    = "costrict_chat_task_"
	ESRequestIndexPrefix = "costrict_chat_request_"
	ESIndexPattern       = "costrict_chat_*"
	DefaultPageSize      = 50
	AIMaxTokens          = 1024
	EfficiencyRatioMin   = 0.1
	EfficiencyRatioMax   = 10000.0

	DefaultTraditionalDevLinesPerDay = 100 // 传统开发人天代码量基准值（行/人天）
)
