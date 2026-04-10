package main

import (
	"strings"
	"testing"
)

func TestNewGitAnalyzer(t *testing.T) {
	cacheDir := "/tmp/git_cache"

	// 验证 LocalPath 包含 hash
	a := NewGitAnalyzer("https://github.com/xxx/yyy.git", cacheDir, "")
	if !strings.HasPrefix(a.LocalPath, cacheDir+"/") {
		t.Errorf("LocalPath 应以 cacheDir 为前缀, got %s", a.LocalPath)
	}
	hashPart := strings.TrimPrefix(a.LocalPath, cacheDir+"/")
	if len(hashPart) != 12 {
		t.Errorf("LocalPath hash 部分应为12位, got %d (%s)", len(hashPart), hashPart)
	}

	// 验证 CacheDir 正确设置
	if a.CacheDir != cacheDir {
		t.Errorf("CacheDir = %s, want %s", a.CacheDir, cacheDir)
	}

	// 验证不同 URL 生成不同 LocalPath
	b := NewGitAnalyzer("https://github.com/aaa/bbb.git", cacheDir, "")
	if a.LocalPath == b.LocalPath {
		t.Errorf("不同 URL 应生成不同 LocalPath, 都是 %s", a.LocalPath)
	}
}

func TestFormatDate(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"20260301", "2026-03-01", false},
		{"20261231", "2026-12-31", false},
		{"2026031", "", true},   // 长度7
		{"202603011", "", true},  // 长度9
		{"", "", true},           // 空字符串
	}

	for _, tt := range tests {
		got, err := formatDate(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("formatDate(%q) 应返回错误", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("formatDate(%q) 意外错误: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("formatDate(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestMakeSafeID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://github.com/xxx/yyy.git", "https___github_com_xxx_yyy_git"},
		{"abc123", "abc123"},
		{"a-b.c/d", "a_b_c_d"},
	}

	for _, tt := range tests {
		got := makeSafeID(tt.input)
		if got != tt.want {
			t.Errorf("makeSafeID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
