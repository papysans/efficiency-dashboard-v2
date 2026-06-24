package utils

import (
	"strings"
	"testing"
)

func TestStripLargeBase64(t *testing.T) {
	b64 := strings.Repeat("A", 300) // 300 个连续 base64 字符(模拟图片 blob)

	tests := []struct {
		name      string
		in        string
		wantHas   string // 期望包含
		wantNoHas string // 期望不包含
	}{
		{"短文本原样", "你好 hello world", "你好 hello world", ""},
		{"普通文本不误伤", "this is a normal sentence with words.", "normal sentence", "omitted"},
		{"长base64被剥离", "前缀文本 " + b64 + " 后缀文本", "前缀文本", b64},
		{"剥离后留占位符", "img:" + b64, "[base64 300B omitted]", b64},
		{"data URI 图片的 base64 被剥", "data:image/jpeg;base64," + b64, "data:image/jpeg;base64,", b64},
		{"短base64(<200)保留", "abc123+/==", "abc123+/==", "omitted"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripLargeBase64(tt.in)
			if tt.wantHas != "" && !strings.Contains(got, tt.wantHas) {
				t.Errorf("结果应含 %q，实际: %q", tt.wantHas, got)
			}
			if tt.wantNoHas != "" && strings.Contains(got, tt.wantNoHas) {
				t.Errorf("结果不应含被剥内容 %q，实际: %q", tt.wantNoHas, got)
			}
		})
	}
}
