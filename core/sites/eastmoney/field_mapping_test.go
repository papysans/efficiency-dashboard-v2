package eastmoney

import (
	"testing"
)

func TestParseReportDate(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		// 正常场景：标准东方财富日期格式
		{name: "annual report", input: "2025-12-31 00:00:00", want: "2025-12-31"},
		{name: "semi-annual report", input: "2024-06-30 00:00:00", want: "2024-06-30"},
		{name: "Q1 report", input: "2024-03-31 00:00:00", want: "2024-03-31"},
		{name: "Q3 report", input: "2024-09-30 00:00:00", want: "2024-09-30"},
		// 边界场景：刚好 10 字符（无时间部分）
		{name: "date only no time", input: "2025-12-31", want: "2025-12-31"},
		// 边界场景：超长字符串
		{name: "extra trailing text", input: "2025-12-31 00:00:00.000+08:00", want: "2025-12-31"},

		// 异常场景：空字符串
		{name: "empty string", input: "", wantErr: true},
		// 异常场景：长度不足 10
		{name: "too short year only", input: "2025", wantErr: true},
		{name: "too short partial", input: "2025-12", wantErr: true},
		// 异常场景：非日期字符串但长度 < 10
		{name: "short garbage", input: "abc", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseReportDate(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseReportDate(%q) expected error, got %q", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Errorf("parseReportDate(%q) unexpected error: %v", tt.input, err)
				return
			}
			if got != tt.want {
				t.Errorf("parseReportDate(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFieldMappingCompleteness(t *testing.T) {
	// 利润表关键字段
	t.Run("income critical fields", func(t *testing.T) {
		critical := []string{"BIZTOTINCO", "NETPROFIT", "DEDUNETPROFIT", "OPERPROFIT", "BASICEPS", "DILUTEDEPS"}
		for _, dbField := range critical {
			found := false
			for _, v := range incomeFieldMap {
				if v == dbField {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("incomeFieldMap missing critical DB field: %s", dbField)
			}
		}
	})

	// 资产负债表关键字段
	t.Run("balance critical fields", func(t *testing.T) {
		critical := []string{"TOTASSET", "TOTLIAB", "TOTSHAREQUI", "CURFDS", "CAPISTOCK", "UNDISTPROFIT"}
		for _, dbField := range critical {
			found := false
			for _, v := range balanceFieldMap {
				if v == dbField {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("balanceFieldMap missing critical DB field: %s", dbField)
			}
		}
	})

	// 现金流量表关键字段
	t.Run("cashflow critical fields", func(t *testing.T) {
		critical := []string{"MANANETR", "INVNETCASHFLOW", "FINNETCFLOW", "CASHNETR", "CASHENDOFPER"}
		for _, dbField := range critical {
			found := false
			for _, v := range cashflowFieldMap {
				if v == dbField {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("cashflowFieldMap missing critical DB field: %s", dbField)
			}
		}
	})

	// 检查同一 map 内无重复 DB 字段名
	t.Run("no duplicates within incomeFieldMap", func(t *testing.T) {
		checkNoDuplicateValues(t, "incomeFieldMap", incomeFieldMap)
	})
	t.Run("no duplicates within balanceFieldMap", func(t *testing.T) {
		checkNoDuplicateValues(t, "balanceFieldMap", balanceFieldMap)
	})
	t.Run("no duplicates within cashflowFieldMap", func(t *testing.T) {
		checkNoDuplicateValues(t, "cashflowFieldMap", cashflowFieldMap)
	})

	// 检查跨 map 的重复（允许的例外除外）
	t.Run("cross-map duplicate check", func(t *testing.T) {
		allowed := map[string]bool{
			"OTHERCOMPINCO":  true, // 出现在 income + balance
			"MINORITYINCO_B": true, // 出现在 income + balance
		}

		allMaps := map[string]map[string]string{
			"income":   incomeFieldMap,
			"balance":  balanceFieldMap,
			"cashflow": cashflowFieldMap,
		}

		// 收集所有 DB 字段 → 来源 map 列表
		fieldSources := make(map[string][]string)
		for mapName, m := range allMaps {
			for _, dbField := range m {
				fieldSources[dbField] = append(fieldSources[dbField], mapName)
			}
		}

		for dbField, sources := range fieldSources {
			if len(sources) > 1 && !allowed[dbField] {
				t.Errorf("DB field %q appears in multiple maps: %v (not in allowed list)", dbField, sources)
			}
		}
	})

	// 检查字段总数合理性
	t.Run("field count sanity", func(t *testing.T) {
		if len(incomeFieldMap) < 30 {
			t.Errorf("incomeFieldMap has only %d fields, expected >= 30", len(incomeFieldMap))
		}
		if len(balanceFieldMap) < 45 {
			t.Errorf("balanceFieldMap has only %d fields, expected >= 45", len(balanceFieldMap))
		}
		if len(cashflowFieldMap) < 25 {
			t.Errorf("cashflowFieldMap has only %d fields, expected >= 25", len(cashflowFieldMap))
		}
	})
}

// checkNoDuplicateValues 检查 map 中是否有重复的 value
func checkNoDuplicateValues(t *testing.T, mapName string, m map[string]string) {
	t.Helper()
	seen := make(map[string]string) // dbField → emField
	for emField, dbField := range m {
		if prevEM, exists := seen[dbField]; exists {
			t.Errorf("%s: duplicate DB field %q mapped from both %q and %q", mapName, dbField, prevEM, emField)
		}
		seen[dbField] = emField
	}
}
