package query

import "strings"

// BuildReportTypeFilter 根据报告类型生成 SQL 过滤片段（以 AND 开头）
// 统一用 TO_CHAR(report_date, 'MMDD') 方式生成 SQL 片段
// 枚举值：year/q1/h1/q3（兼容 full→year 映射）
// all 或空字符串返回 ""
func BuildReportTypeFilter(reportType string) string {
	// 兼容旧枚举值
	if reportType == "full" {
		reportType = "year"
	}
	switch reportType {
	case "year":
		return "AND TO_CHAR(report_date, 'MMDD') = '1231'"
	case "q1":
		return "AND TO_CHAR(report_date, 'MMDD') = '0331'"
	case "h1":
		return "AND TO_CHAR(report_date, 'MMDD') = '0630'"
	case "q3":
		return "AND TO_CHAR(report_date, 'MMDD') = '0930'"
	default:
		return ""
	}
}

// BuildReportTypeFilterWithAlias 生成带表别名的报告期过滤 SQL 片段
func BuildReportTypeFilterWithAlias(reportType, tableAlias string) string {
	filter := BuildReportTypeFilter(reportType)
	if filter == "" || tableAlias == "" {
		return filter
	}
	return strings.ReplaceAll(filter, "report_date", tableAlias+".report_date")
}
