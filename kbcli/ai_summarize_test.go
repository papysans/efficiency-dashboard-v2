package main

import "testing"

func TestTruncateSlice(t *testing.T) {
	tests := []struct {
		name     string
		items    []string
		maxLen   int
		expected string
	}{
		{
			name:     "empty slice",
			items:    []string{},
			maxLen:   100,
			expected: "",
		},
		{
			name:     "single item shorter than maxLen",
			items:    []string{"hello"},
			maxLen:   10,
			expected: "hello",
		},
		{
			name:     "single item exactly at maxLen boundary",
			items:    []string{"hello"},
			maxLen:   5,
			expected: "hello",
		},
		{
			name:     "single item exactly 1 char over maxLen",
			items:    []string{"hello"},
			maxLen:   4,
			expected: "hell...(截断)",
		},
		{
			name:     "single item longer than maxLen",
			items:    []string{"hello world"},
			maxLen:   5,
			expected: "hello...(截断)",
		},
		{
			name:     "multiple items total fits in maxLen",
			items:    []string{"abc", "def", "ghi"},
			maxLen:   20,
			expected: "abc\n---\ndef\n---\nghi",
		},
		{
			name:     "multiple items second would exceed maxLen",
			items:    []string{"hello", "worldwide"},
			maxLen:   10,
			expected: "helloworld...(截断)",
		},
		{
			name:     "multiple items first alone exceeds maxLen",
			items:    []string{"hello world", "foo"},
			maxLen:   5,
			expected: "hello...(截断)",
		},
		{
			name:     "maxLen zero with non-empty item",
			items:    []string{"abc"},
			maxLen:   0,
			expected: "",
		},
		{
			name:     "maxLen zero with empty item",
			items:    []string{""},
			maxLen:   0,
			expected: "",
		},
		{
			name:     "maxLen less than suffix length",
			items:    []string{"abcdef"},
			maxLen:   3,
			expected: "abc...(截断)",
		},
		{
			name:     "maxLen negative with non-empty item",
			items:    []string{"abc"},
			maxLen:   -1,
			expected: "",
		},
		{
			name:     "maxLen negative with empty item",
			items:    []string{""},
			maxLen:   -1,
			expected: "",
		},
		{
			name:     "separator pushes total over maxLen",
			items:    []string{"abc", "def"},
			maxLen:   7,
			expected: "abc\n---\ndef",
		},
		{
			name:     "separator exactly fits",
			items:    []string{"abc", "def"},
			maxLen:   9,
			expected: "abc\n---\ndef",
		},
		{
			name:     "empty string in middle",
			items:    []string{"abc", "", "def"},
			maxLen:   20,
			expected: "abc\n---\n\n---\ndef",
		},
		{
			name:     "multiple items all empty",
			items:    []string{"", "", ""},
			maxLen:   10,
			expected: "\n---\n\n---\n",
		},
		{
			name:     "unicode item shorter than maxLen",
			items:    []string{"你好世界"},
			maxLen:   12,
			expected: "你好世界",
		},
		{
			name:     "unicode item truncated by byte length",
			items:    []string{"你好世界"},
			maxLen:   6,
			expected: "你好...(截断)",
		},
		{
			name:     "maxLen 1 with single char item",
			items:    []string{"a"},
			maxLen:   1,
			expected: "a",
		},
		{
			name:     "maxLen 1 with multi-char item",
			items:    []string{"ab"},
			maxLen:   1,
			expected: "a...(截断)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateSlice(tt.items, tt.maxLen)
			if got != tt.expected {
				t.Errorf("truncateSlice(%q, %d) = %q, want %q", tt.items, tt.maxLen, got, tt.expected)
			}
		})
	}
}
