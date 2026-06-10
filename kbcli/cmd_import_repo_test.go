package main

import (
	"reflect"
	"testing"
)

// TestExtractTouchedFilesFromDiff 覆盖标准 diff / 重命名 / 空 diff 三类场景。
func TestExtractTouchedFilesFromDiff(t *testing.T) {
	t.Run("标准 diff 提取 b 侧路径并去重排序", func(t *testing.T) {
		diff := "diff --git a/core/models/models.go b/core/models/models.go\n" +
			"index 1111111..2222222 100644\n" +
			"--- a/core/models/models.go\n" +
			"+++ b/core/models/models.go\n" +
			"@@ -1,3 +1,4 @@\n" +
			"+// 新增注释\n" +
			"diff --git a/kbcli/main.go b/kbcli/main.go\n" +
			"index 3333333..4444444 100644\n" +
			"--- a/kbcli/main.go\n" +
			"+++ b/kbcli/main.go\n" +
			"@@ -10,0 +11,1 @@\n" +
			"+func added() {}\n"
		got := extractTouchedFilesFromDiff(diff)
		want := []string{"core/models/models.go", "kbcli/main.go"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("extractTouchedFilesFromDiff = %v, want %v", got, want)
		}
	})

	t.Run("重命名取 b 侧新路径", func(t *testing.T) {
		diff := "diff --git a/old/name.go b/new/renamed.go\n" +
			"similarity index 95%\n" +
			"rename from old/name.go\n" +
			"rename to new/renamed.go\n"
		got := extractTouchedFilesFromDiff(diff)
		want := []string{"new/renamed.go"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("extractTouchedFilesFromDiff = %v, want %v", got, want)
		}
	})

	t.Run("空 diff 返回空", func(t *testing.T) {
		if got := extractTouchedFilesFromDiff(""); len(got) != 0 {
			t.Fatalf("extractTouchedFilesFromDiff(\"\") = %v, want empty", got)
		}
	})

	t.Run("非头行的 diff --git 文本不误匹配", func(t *testing.T) {
		// 行中部出现 "diff --git" 字样（如 diff 内容里引用），(?m)^ 锚定行首不应命中
		diff := "+ run: diff --git a/x.go b/x.go\n"
		if got := extractTouchedFilesFromDiff(diff); len(got) != 0 {
			t.Fatalf("行中部文本不应匹配, got %v", got)
		}
	})
}
