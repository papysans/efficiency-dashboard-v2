package eastmoney

import (
	"testing"
)

func TestConvertStockCode(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		// 正常场景：深交所
		{name: "SZ uppercase", input: "SZ300454", want: "300454.SZ"},
		// 正常场景：上交所
		{name: "SH uppercase", input: "SH601318", want: "601318.SH"},
		// 正常场景：北交所
		{name: "BJ uppercase", input: "BJ838171", want: "838171.BJ"},
		// 正常场景：小写输入自动转大写
		{name: "sz lowercase", input: "sz300454", want: "300454.SZ"},
		{name: "sh lowercase", input: "sh601318", want: "601318.SH"},
		{name: "bj lowercase", input: "bj838171", want: "838171.BJ"},
		// 正常场景：混合大小写
		{name: "Sz mixed case", input: "Sz300454", want: "300454.SZ"},

		// 异常场景：空字符串
		{name: "empty string", input: "", wantErr: true},
		// 异常场景：长度不足（<3）
		{name: "single char", input: "X", wantErr: true},
		{name: "two chars only", input: "XX", wantErr: true},
		// 异常场景：不支持的市场前缀
		{name: "HK unsupported", input: "HK00700", wantErr: true},
		{name: "US unsupported", input: "US12345", wantErr: true},
		// 异常场景：仅有市场前缀无数字
		{name: "SZ no number part", input: "SZ", wantErr: true},
		// 边界场景：刚好 3 字符（前缀 + 1 位数字）
		{name: "minimal valid SZ1", input: "SZ1", want: "1.SZ"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convertStockCode(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("convertStockCode(%q) expected error, got %q", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Errorf("convertStockCode(%q) unexpected error: %v", tt.input, err)
				return
			}
			if got != tt.want {
				t.Errorf("convertStockCode(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
