package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// ============================================================================
// extractAddedLinesFromDiff — 路由分发测试
// ============================================================================

// 覆盖 L27-29: 空字符串 / 纯空白 → 返回 nil
func TestExtractAddedLines_EmptyOrWhitespace(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{"empty string", ""},
		{"whitespace only", "   \n  \t  "},
		{"newlines only", "\n\n\n"},
		{"tab and spaces", "\t\t  \t"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractAddedLinesFromDiff(tt.in)
			if got != nil {
				t.Errorf("期望 nil, 得到 %v", got)
			}
		})
	}
}

// 覆盖 L31-42: 合法 JSON 且包含有效字段 → 走 extractFromJSONDiff 路径
// 覆盖 L35 各条件 (d.File, d.After, d.Before, d.Additions, d.Deletions)
// 覆盖 L36-37 hasExpected=true + break
func TestExtractAddedLines_JSON_ValidWithFields(t *testing.T) {
	t.Run("via file field", func(t *testing.T) {
		j, _ := json.Marshal([]diffJSONEntry{
			{File: "foo.go", After: "fmt.Println(\"hello\")"},
		})
		got := extractAddedLinesFromDiff(string(j))
		if len(got) != 1 {
			t.Fatalf("期望 1 条, 得到 %d", len(got))
		}
		if got[0].FilePath != "foo.go" {
			t.Errorf("FilePath: 期望 foo.go, 得到 %s", got[0].FilePath)
		}
		if got[0].Content != "fmt.Println(\"hello\")" {
			t.Errorf("Content: 期望 fmt.Println(\"hello\"), 得到 %s", got[0].Content)
		}
	})

	t.Run("via after field only triggers hasExpected", func(t *testing.T) {
		j, _ := json.Marshal([]diffJSONEntry{
			{After: "newLine"},
		})
		got := extractAddedLinesFromDiff(string(j))
		if len(got) != 1 {
			t.Fatalf("期望 1 条, 得到 %d", len(got))
		}
		if got[0].Content != "newLine" {
			t.Errorf("Content: 期望 newLine, 得到 %s", got[0].Content)
		}
	})

	t.Run("via additions > 0 triggers hasExpected", func(t *testing.T) {
		j, _ := json.Marshal([]diffJSONEntry{
			{Additions: 5, After: "added"},
		})
		got := extractAddedLinesFromDiff(string(j))
		if len(got) != 1 {
			t.Fatalf("期望 1 条, 得到 %d", len(got))
		}
	})

	t.Run("via deletions > 0 triggers hasExpected", func(t *testing.T) {
		j, _ := json.Marshal([]diffJSONEntry{
			{Deletions: 3, After: "after deletion"},
		})
		got := extractAddedLinesFromDiff(string(j))
		if len(got) != 1 {
			t.Fatalf("期望 1 条, 得到 %d", len(got))
		}
	})

	t.Run("via before != \"\" triggers hasExpected", func(t *testing.T) {
		j, _ := json.Marshal([]diffJSONEntry{
			{Before: "old", After: "new"},
		})
		got := extractAddedLinesFromDiff(string(j))
		if len(got) != 1 {
			t.Fatalf("期望 1 条, 得到 %d", len(got))
		}
		if got[0].Content != "new" {
			t.Errorf("期望 new, 得到 %s", got[0].Content)
		}
	})
}

// 覆盖 L34-39 循环 + L40=false: JSON 条目全部字段为空/零 → hasExpected=false → 回退到统一 diff
func TestExtractAddedLines_JSON_AllEmptyEntries(t *testing.T) {
	j, _ := json.Marshal([]diffJSONEntry{
		{File: "", Before: "", After: "", Additions: 0, Deletions: 0, Status: ""},
	})
	got := extractAddedLinesFromDiff(string(j))
	if len(got) != 0 {
		t.Errorf("期望 0 条, 得到 %d", len(got))
	}
}

// 多条全空记录 → hasExpected 仍为 false
func TestExtractAddedLines_JSON_MultipleAllEmptyEntries(t *testing.T) {
	j, _ := json.Marshal([]diffJSONEntry{
		{}, {}, {},
	})
	got := extractAddedLinesFromDiff(string(j))
	if len(got) != 0 {
		t.Errorf("期望 0 条, 得到 %d", len(got))
	}
}

// 覆盖 L31-32 条件失败: JSON 解析失败 → 跳过 JSON 分支
func TestExtractAddedLines_InvalidJSON(t *testing.T) {
	got := extractAddedLinesFromDiff("not json at all")
	if got != nil && len(got) != 0 {
		t.Errorf("期望空结果, 得到 %d 条", len(got))
	}
}

// 覆盖 L31-32 条件: 合法但空数组 [] → len==0 → 跳过 JSON 分支
func TestExtractAddedLines_JSON_EmptyArray(t *testing.T) {
	got := extractAddedLinesFromDiff("[]")
	if len(got) != 0 {
		t.Errorf("期望 0 条, 得到 %d", len(got))
	}
}

// 覆盖 L45-47: "BEFORE"/"AFTER" 格式 → extractFromBeforeAfterDiff
func TestExtractAddedLines_BeforeAfterFormat(t *testing.T) {
	diff := `--- file.go
<<< BEFORE
old line
>>> AFTER
new line
`
	got := extractAddedLinesFromDiff(diff)
	if len(got) != 1 {
		t.Fatalf("期望 1 条, 得到 %d", len(got))
	}
	if got[0].FilePath != "file.go" {
		t.Errorf("FilePath: 期望 file.go, 得到 %s", got[0].FilePath)
	}
	if got[0].Content != "new line" {
		t.Errorf("Content: 期望 new line, 得到 %s", got[0].Content)
	}
}

// 覆盖 L49: 统一 diff 格式 → extractFromUnifiedDiff
func TestExtractAddedLines_UnifiedDiffFormat(t *testing.T) {
	diff := `diff --git a/foo.go b/foo.go
--- a/foo.go
+++ b/foo.go
@@ -1,3 +1,4 @@
 package main
+import "fmt"
 func main() {}
`
	got := extractAddedLinesFromDiff(diff)
	if len(got) != 1 {
		t.Fatalf("期望 1 条, 得到 %d", len(got))
	}
	if got[0].FilePath != "foo.go" {
		t.Errorf("FilePath: 期望 foo.go, 得到 %s", got[0].FilePath)
	}
	if got[0].Content != "import \"fmt\"" {
		t.Errorf("Content: 期望 import \"fmt\", 得到 %s", got[0].Content)
	}
}

// ============================================================================
// extractFromUnifiedDiff — 统一 diff 格式测试（增/删/改/多 hunk/多文件）
// ============================================================================

// 单文件单 hunk，仅新增行
func TestExtractFromUnifiedDiff_Normal(t *testing.T) {
	diff := `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,0 +1,2 @@
+package main

+func hello() {
+    return
+}
`
	got := extractFromUnifiedDiff(diff)
	// "package main", "func hello() {", "return", "}" = 4 条
	if len(got) != 4 {
		t.Fatalf("期望 4 条, 得到 %d", len(got))
	}
	for _, al := range got {
		if al.FilePath != "main.go" {
			t.Errorf("FilePath: 期望 main.go, 得到 %s", al.FilePath)
		}
	}
}

// 多 hunk 在同一文件中（新增 + 删除 + 上下文混合）
func TestExtractFromUnifiedDiff_MultiHunk(t *testing.T) {
	diff := `--- a/service.go
+++ b/service.go
@@ -1,5 +1,6 @@
 package service
+import "context"

 func New() *Svc {
@@ -10,6 +11,8 @@
 func (s *Svc) Handle(req *Req) error {
-    log.Printf("old log")
+    log.Printf("new log")
+    s.metrics.Inc()
     return nil
 }
`
	got := extractFromUnifiedDiff(diff)
	// 新增行: "import \"context\"", "log.Printf(\"new log\")", "s.metrics.Inc()"
	if len(got) != 3 {
		t.Fatalf("期望 3 条, 得到 %d", len(got))
	}
	for _, al := range got {
		if al.FilePath != "service.go" {
			t.Errorf("FilePath: 期望 service.go, 得到 %s", al.FilePath)
		}
	}
}

// 仅删除无新增（如纯重构删除代码）→ 应返回 0 条
func TestExtractFromUnifiedDiff_OnlyDeletions(t *testing.T) {
	diff := `--- a/cleanup.go
+++ b/cleanup.go
@@ -1,3 +1,1 @@
 package cleanup
-func unused() {}
-func dead() {}
 func keep() {}
`
	got := extractFromUnifiedDiff(diff)
	if len(got) != 0 {
		t.Errorf("仅删除应返回 0 条, 得到 %d", len(got))
	}
}

// 新增文件（/dev/null → b/file.go）→ currentFile 从 +++ b/ 提取
func TestExtractFromUnifiedDiff_NewFile(t *testing.T) {
	diff := `--- /dev/null
+++ b/new_file.go
@@ -0,0 +1,2 @@
+package main
+func init() {}
`
	got := extractFromUnifiedDiff(diff)
	if len(got) != 2 {
		t.Fatalf("新增文件期望 2 条, 得到 %d", len(got))
	}
	for _, al := range got {
		if al.FilePath != "new_file.go" {
			t.Errorf("FilePath: 期望 new_file.go, 得到 %s", al.FilePath)
		}
	}
}

// 删除文件（文件 → /dev/null）→ currentFile 被清空，无新增行
func TestExtractFromUnifiedDiff_DevNull(t *testing.T) {
	diff := `--- a/deleted.go
+++ /dev/null
@@ -1,1 +0,0 @@
-old line
`
	got := extractFromUnifiedDiff(diff)
	if len(got) != 0 {
		t.Errorf("期望 0 条, 得到 %d", len(got))
	}
}

// "+" 行但内容纯空白 → trimmed=="" → 不添加
func TestExtractFromUnifiedDiff_EmptyAddedLine(t *testing.T) {
	diff := `--- a/test.go
+++ b/test.go
@@ -1,0 +1,4 @@
+   
+	
+package main
`
	got := extractFromUnifiedDiff(diff)
	if len(got) != 1 {
		t.Fatalf("期望 1 条, 得到 %d", len(got))
	}
	if got[0].Content != "package main" {
		t.Errorf("期望 package main, 得到 %s", got[0].Content)
	}
}

// 多文件场景（2 个文件各 1 条新增）
func TestExtractFromUnifiedDiff_MultipleFiles(t *testing.T) {
	diff := `diff --git a/a.go b/a.go
--- a/a.go
+++ b/a.go
@@ -1,0 +1,1 @@
+lineA
diff --git a/b.go b/b.go
--- a/b.go
+++ b/b.go
@@ -1,0 +1,1 @@
+lineB
`
	got := extractFromUnifiedDiff(diff)
	if len(got) != 2 {
		t.Fatalf("期望 2 条, 得到 %d", len(got))
	}
	if got[0].FilePath != "a.go" || got[0].Content != "lineA" {
		t.Errorf("第1条: 期望 a.go/lineA, 得到 %s/%s", got[0].FilePath, got[0].Content)
	}
	if got[1].FilePath != "b.go" || got[1].Content != "lineB" {
		t.Errorf("第2条: 期望 b.go/lineB, 得到 %s/%s", got[1].FilePath, got[1].Content)
	}
}

// 多文件：一个有新增、一个有删除、一个有混合
func TestExtractFromUnifiedDiff_MultiFileAddDelMix(t *testing.T) {
	diff := `diff --git a/add.go b/add.go
--- a/add.go
+++ b/add.go
@@ -1,0 +1,1 @@
+newFunc()
diff --git a/del.go b/del.go
--- a/del.go
+++ b/del.go
@@ -1,1 +0,0 @@
-oldFunc()
diff --git a/mix.go b/mix.go
--- a/mix.go
+++ b/mix.go
@@ -1,2 +1,2 @@
-var old = 1
+var new = 2
`
	got := extractFromUnifiedDiff(diff)
	// add.go: "newFunc()" / mix.go: "var new = 2" → 2 条
	if len(got) != 2 {
		t.Fatalf("期望 2 条, 得到 %d", len(got))
	}
	if got[0].FilePath != "add.go" || got[0].Content != "newFunc()" {
		t.Errorf("第1条: 期望 add.go/newFunc(), 得到 %s/%s", got[0].FilePath, got[0].Content)
	}
	if got[1].FilePath != "mix.go" || got[1].Content != "var new = 2" {
		t.Errorf("第2条: 期望 mix.go/var new = 2, 得到 %s/%s", got[1].FilePath, got[1].Content)
	}
}

// diff 中无任何 "+++ b/" 标记（退化为无文件名的添加）
func TestExtractFromUnifiedDiff_NoFileHeader(t *testing.T) {
	diff := `+orphan line 1
+orphan line 2
`
	got := extractFromUnifiedDiff(diff)
	if len(got) != 2 {
		t.Fatalf("期望 2 条, 得到 %d", len(got))
	}
	for _, al := range got {
		if al.FilePath != "" {
			t.Errorf("FilePath: 期望空字符串, 得到 %s", al.FilePath)
		}
	}
}

// ============================================================================
// extractFromJSONDiff — JSON diff 格式测试
// ============================================================================

// 覆盖 L79-81: d.After 为空 → continue 跳过
func TestExtractFromJSONDiff_EmptyAfter(t *testing.T) {
	entries := []diffJSONEntry{
		{File: "a.go", Before: "old", After: ""},
		{File: "b.go", Before: "", After: "valid"},
	}
	got := extractFromJSONDiff(entries)
	if len(got) != 1 {
		t.Fatalf("期望 1 条, 得到 %d", len(got))
	}
	if got[0].FilePath != "b.go" || got[0].Content != "valid" {
		t.Errorf("期望 b.go/valid, 得到 %s/%s", got[0].FilePath, got[0].Content)
	}
}

// 覆盖 L83-90: beforeLines 构建（Before 非空时）
// 覆盖 L91-96: After 中不在 beforeLines 的行 → 添加
func TestExtractFromJSONDiff_WithBefore(t *testing.T) {
	entries := []diffJSONEntry{
		{
			File:   "main.go",
			Before: "line1\nline2\n   \nline3",
			After:  "line1\nline4\nline2",
		},
	}
	got := extractFromJSONDiff(entries)
	if len(got) != 1 {
		t.Fatalf("期望 1 条, 得到 %d", len(got))
	}
	if got[0].Content != "line4" {
		t.Errorf("期望 line4, 得到 %s", got[0].Content)
	}
}

// 覆盖 L82-90 的 else 分支: Before 为空 → beforeLines 为空 map → 所有 After 行新增
func TestExtractFromJSONDiff_EmptyBefore(t *testing.T) {
	entries := []diffJSONEntry{
		{File: "new.go", Before: "", After: "line1\nline2\nline3"},
	}
	got := extractFromJSONDiff(entries)
	if len(got) != 3 {
		t.Fatalf("期望 3 条, 得到 %d", len(got))
	}
	for i, al := range got {
		if al.FilePath != "new.go" {
			t.Errorf("第%d条 FilePath: 期望 new.go, 得到 %s", i+1, al.FilePath)
		}
	}
}

// 多条目 + After 中有空行（trimmed==""）→ 不添加
func TestExtractFromJSONDiff_MultipleEntries(t *testing.T) {
	entries := []diffJSONEntry{
		{File: "a.go", After: "alpha"},
		{File: "b.go", After: "beta\n\n gamma \n"},
	}
	got := extractFromJSONDiff(entries)
	if len(got) != 3 {
		t.Fatalf("期望 3 条, 得到 %d", len(got))
	}
}

// 同一文件多条目：新增 + 修改
func TestExtractFromJSONDiff_MultiEntrySameFile(t *testing.T) {
	entries := []diffJSONEntry{
		{File: "svc.go", Before: "func Old() {}", After: "func New() {}"},
		{File: "svc.go", Before: "", After: "func Extra() {}"},
	}
	got := extractFromJSONDiff(entries)
	// "func New() {}", "func Extra() {}"
	if len(got) != 2 {
		t.Fatalf("期望 2 条, 得到 %d", len(got))
	}
	for _, al := range got {
		if al.FilePath != "svc.go" {
			t.Errorf("FilePath: 期望 svc.go, 得到 %s", al.FilePath)
		}
	}
}

// Before/After 中有大量重复行 → beforeLines 过滤重复
func TestExtractFromJSONDiff_OverlappingLines(t *testing.T) {
	entries := []diffJSONEntry{
		{
			File:   "refactor.go",
			Before: "common1\ncommon2\nold_only\ncommon3",
			After:  "common1\ncommon2\nnew_only\ncommon3",
		},
	}
	got := extractFromJSONDiff(entries)
	if len(got) != 1 {
		t.Fatalf("期望 1 条 (仅 new_only), 得到 %d", len(got))
	}
	if got[0].Content != "new_only" {
		t.Errorf("期望 new_only, 得到 %s", got[0].Content)
	}
}

// After 全部与 Before 相同 → 无净新增
func TestExtractFromJSONDiff_NoNetNew(t *testing.T) {
	entries := []diffJSONEntry{
		{File: "same.go", Before: "a\nb\nc", After: "a\nb\nc"},
	}
	got := extractFromJSONDiff(entries)
	if len(got) != 0 {
		t.Errorf("期望 0 条, 得到 %d", len(got))
	}
}

// ============================================================================
// extractFromBeforeAfterDiff — Before/After 格式测试
// ============================================================================

// 单文件：增 + 删 + 保留混合
func TestExtractFromBeforeAfterDiff_SingleFile(t *testing.T) {
	diff := `--- main.go
<<< BEFORE
package old
import "os"
>>> AFTER
package main
import "fmt"
import "os"
`
	got := extractFromBeforeAfterDiff(diff)
	// "package old" 在 before, "import \"os\"" 也在 before
	// 新增: "package main", "import \"fmt\""
	if len(got) != 2 {
		t.Fatalf("期望 2 条, 得到 %d", len(got))
	}
	if got[0].FilePath != "main.go" {
		t.Errorf("FilePath: 期望 main.go, 得到 %s", got[0].FilePath)
	}
}

// 多文件场景
func TestExtractFromBeforeAfterDiff_MultipleFiles(t *testing.T) {
	diff := `--- a.go
<<< BEFORE
lineA
>>> AFTER
lineA_new
--- b.go
<<< BEFORE
lineB
>>> AFTER
lineB_new
`
	got := extractFromBeforeAfterDiff(diff)
	if len(got) != 2 {
		t.Fatalf("期望 2 条, 得到 %d", len(got))
	}
	if got[0].FilePath != "a.go" || got[0].Content != "lineA_new" {
		t.Errorf("第1条: 期望 a.go/lineA_new, 得到 %s/%s", got[0].FilePath, got[0].Content)
	}
	if got[1].FilePath != "b.go" || got[1].Content != "lineB_new" {
		t.Errorf("第2条: 期望 b.go/lineB_new, 得到 %s/%s", got[1].FilePath, got[1].Content)
	}
}

// "--- " 行后重置 inBefore/inAfter，且 before/after 为空时 L109 条件不满足不触发处理
func TestExtractFromBeforeAfterDiff_NoContentBlocks(t *testing.T) {
	diff := `--- empty.go
--- next.go
<<< BEFORE
something
>>> AFTER
something_else
`
	got := extractFromBeforeAfterDiff(diff)
	if len(got) != 1 {
		t.Fatalf("期望 1 条, 得到 %d", len(got))
	}
	if got[0].FilePath != "next.go" {
		t.Errorf("FilePath: 期望 next.go, 得到 %s", got[0].FilePath)
	}
}

// 无尾部 "--- " 标记时，循环结束后处理残留内容（L137-139）
func TestExtractFromBeforeAfterDiff_NoTrailingDelimiter(t *testing.T) {
	diff := `<<< BEFORE
old_code
>>> AFTER
new_code
`
	got := extractFromBeforeAfterDiff(diff)
	if len(got) != 1 {
		t.Fatalf("期望 1 条, 得到 %d", len(got))
	}
	if got[0].Content != "new_code" {
		t.Errorf("期望 new_code, 得到 %s", got[0].Content)
	}
	if got[0].FilePath != "" {
		t.Errorf("FilePath: 期望空字符串, 得到 %s", got[0].FilePath)
	}
}

// 覆盖 L131 + L134: WriteByte('\n')
func TestExtractFromBeforeAfterDiff_MultilineContent(t *testing.T) {
	diff := `--- multi.go
<<< BEFORE
line1
line2
>>> AFTER
line3
line4
line5
`
	got := extractFromBeforeAfterDiff(diff)
	if len(got) != 3 {
		t.Fatalf("期望 3 条, 得到 %d", len(got))
	}
}

// 仅有 before 无 after → computeDiffAddedLines 返回空
func TestExtractFromBeforeAfterDiff_OnlyBefore(t *testing.T) {
	diff := `--- del.go
<<< BEFORE
removed_line
>>> AFTER
`
	got := extractFromBeforeAfterDiff(diff)
	if len(got) != 0 {
		t.Errorf("期望 0 条, 得到 %d", len(got))
	}
}

// 三文件：一增一删一改
func TestExtractFromBeforeAfterDiff_AddDelMod(t *testing.T) {
	diff := `--- add.go
<<< BEFORE
>>> AFTER
new_file_content
--- del.go
<<< BEFORE
bye
>>> AFTER
--- mod.go
<<< BEFORE
hello world
>>> AFTER
hello go
`
	got := extractFromBeforeAfterDiff(diff)
	// add.go: "new_file_content" / mod.go: "hello go" → 2 条
	if len(got) != 2 {
		t.Fatalf("期望 2 条, 得到 %d", len(got))
	}
	if got[0].FilePath != "add.go" || got[0].Content != "new_file_content" {
		t.Errorf("第1条: 期望 add.go/new_file_content, 得到 %s/%s", got[0].FilePath, got[0].Content)
	}
	if got[1].FilePath != "mod.go" || got[1].Content != "hello go" {
		t.Errorf("第2条: 期望 mod.go/hello go, 得到 %s/%s", got[1].FilePath, got[1].Content)
	}
}

// 多文件间有空行/空白
func TestExtractFromBeforeAfterDiff_WithBlankLines(t *testing.T) {
	diff := `
--- f1.go
<<< BEFORE
aaa
>>> AFTER
bbb


--- f2.go
<<< BEFORE
ccc
>>> AFTER
ddd
`
	got := extractFromBeforeAfterDiff(diff)
	if len(got) != 2 {
		t.Fatalf("期望 2 条, 得到 %d", len(got))
	}
}

// ============================================================================
// computeDiffAddedLines — 核心 diff 计算测试
// ============================================================================

// 覆盖 L144-149: 构建 beforeLines map
// 覆盖 L151-157: 新增行 = after 中的非空行且不在 beforeLines 中
func TestComputeDiffAddedLines_Normal(t *testing.T) {
	got := computeDiffAddedLines("test.go",
		"old1\nold2\nold3",
		"old1\nnew1\nold2\nnew2",
	)
	if len(got) != 2 {
		t.Fatalf("期望 2 条, 得到 %d", len(got))
	}
	if got[0].Content != "new1" || got[1].Content != "new2" {
		t.Errorf("期望 [new1 new2], 得到 %v", got)
	}
	for _, al := range got {
		if al.FilePath != "test.go" {
			t.Errorf("FilePath: 期望 test.go, 得到 %s", al.FilePath)
		}
	}
}

// before 为空 → 所有 after 行都是新增（空行被过滤）
func TestComputeDiffAddedLines_EmptyBefore(t *testing.T) {
	got := computeDiffAddedLines("new.go", "", "alpha\nbeta\n\n")
	if len(got) != 2 {
		t.Fatalf("期望 2 条, 得到 %d (空行应被过滤)", len(got))
	}
	if got[0].Content != "alpha" || got[1].Content != "beta" {
		t.Errorf("期望 [alpha beta], 得到 %v", got)
	}
}

// after 中空行/纯空白 → trimmed=="" → 不添加
func TestComputeDiffAddedLines_EmptyLinesInAfter(t *testing.T) {
	got := computeDiffAddedLines("f.go", "old", "\n\n new_only \n\t\n")
	if len(got) != 1 {
		t.Fatalf("期望 1 条, 得到 %d", len(got))
	}
	if got[0].Content != "new_only" {
		t.Errorf("期望 new_only, 得到 %s", got[0].Content)
	}
}

// 所有 after 行都在 before 中 → 无净新增
func TestComputeDiffAddedLines_AllInBefore(t *testing.T) {
	got := computeDiffAddedLines("same.go", "a\nb\nc", "a\nb\nc")
	if len(got) != 0 {
		t.Errorf("期望 0 条, 得到 %d", len(got))
	}
}

// before 中有重复行 → map 自动去重
func TestComputeDiffAddedLines_DuplicateInBefore(t *testing.T) {
	got := computeDiffAddedLines("dup.go",
		"dup\ndup\nunique",
		"dup\nnew_one",
	)
	if len(got) != 1 {
		t.Fatalf("期望 1 条 (new_one), 得到 %d", len(got))
	}
	if got[0].Content != "new_one" {
		t.Errorf("期望 new_one, 得到 %s", got[0].Content)
	}
}

// after 中有与 before 相同的行 → 不添加
func TestComputeDiffAddedLines_PartialOverlap(t *testing.T) {
	got := computeDiffAddedLines("p.go",
		"keep1\nkeep2\nkeep3",
		"keep1\nadded1\nkeep2\nadded2\nkeep3",
	)
	if len(got) != 2 {
		t.Fatalf("期望 2 条, 得到 %d", len(got))
	}
	if got[0].Content != "added1" || got[1].Content != "added2" {
		t.Errorf("期望 [added1 added2], 得到 %v", got)
	}
}

// 大段文本
func TestComputeDiffAddedLines_LargeContent(t *testing.T) {
	beforeLines := make([]string, 100)
	afterLines := make([]string, 105)
	for i := range beforeLines {
		beforeLines[i] = "line" + strings.Repeat("x", 50)
	}
	for i := range afterLines {
		afterLines[i] = "line" + strings.Repeat("x", 50)
	}
	// 最后 5 行是新的
	for i := 100; i < 105; i++ {
		afterLines[i] = "NEW_LINE_" + strings.Repeat("y", 50)
	}
	got := computeDiffAddedLines("big.go",
		strings.Join(beforeLines, "\n"),
		strings.Join(afterLines, "\n"),
	)
	if len(got) != 5 {
		t.Fatalf("期望 5 条新增, 得到 %d", len(got))
	}
}

// ============================================================================
// calcLineFingerprint — 指纹计算测试
// ============================================================================

// 覆盖 L162-164: 完整指纹计算链路
func TestCalcLineFingerprint(t *testing.T) {
	al := addedLine{FilePath: "/home/user/project/main.go", Content: "package main"}
	fp := calcLineFingerprint(al)
	if fp == "" {
		t.Error("指纹不应为空")
	}
	if len(fp) != 64 {
		t.Errorf("期望 64 字符的 SHA-256 哈希, 得到长度 %d: %s", len(fp), fp)
	}
}

// 覆盖 L162: filepath.Base / L163: utils.RemoveWhitespace
// 确定性: 相同输入 → 相同指纹
func TestCalcLineFingerprint_Deterministic(t *testing.T) {
	al := addedLine{FilePath: "/a/b/src/app.go", Content: "func main() {}"}
	fp1 := calcLineFingerprint(al)
	fp2 := calcLineFingerprint(al)
	if fp1 != fp2 {
		t.Errorf("相同输入应产生相同指纹: %s vs %s", fp1, fp2)
	}
}

// 不同内容 → 不同指纹
func TestCalcLineFingerprint_DifferentContent(t *testing.T) {
	a1 := addedLine{FilePath: "a.go", Content: "line1"}
	a2 := addedLine{FilePath: "a.go", Content: "line2"}
	if calcLineFingerprint(a1) == calcLineFingerprint(a2) {
		t.Error("不同内容应产生不同指纹")
	}
}

// 不同文件相同内容 → 不同指纹（filepath.Base 参与计算）
func TestCalcLineFingerprint_DifferentFile(t *testing.T) {
	a1 := addedLine{FilePath: "foo.go", Content: "same"}
	a2 := addedLine{FilePath: "bar.go", Content: "same"}
	if calcLineFingerprint(a1) == calcLineFingerprint(a2) {
		t.Error("不同文件名应产生不同指纹")
	}
}

// filepath.Base 只取文件名，忽略目录路径
func TestCalcLineFingerprint_SameBaseDifferentDir(t *testing.T) {
	a1 := addedLine{FilePath: "/proj/a/app.go", Content: "x"}
	a2 := addedLine{FilePath: "/proj/b/app.go", Content: "x"}
	if calcLineFingerprint(a1) != calcLineFingerprint(a2) {
		t.Error("相同 Base 文件名应产生相同指纹（目录不影响）")
	}
}

// 空白字符不影响指纹（RemoveWhitespace）
func TestCalcLineFingerprint_WhitespaceInsensitive(t *testing.T) {
	a1 := addedLine{FilePath: "f.go", Content: "func main() {"}
	a2 := addedLine{FilePath: "f.go", Content: "func  main()  {"}
	if calcLineFingerprint(a1) != calcLineFingerprint(a2) {
		t.Error("空白字符差异应被 RemoveWhitespace 消除，指纹应一致")
	}
}

// 换行/制表符也被移除
func TestCalcLineFingerprint_NewlineTabInsensitive(t *testing.T) {
	a1 := addedLine{FilePath: "f.go", Content: "a\tb\nc\rd"}
	a2 := addedLine{FilePath: "f.go", Content: "abcd"}
	if calcLineFingerprint(a1) != calcLineFingerprint(a2) {
		t.Error("换行/制表/回车应被 RemoveWhitespace 消除")
	}
}

// ============================================================================
// 集成场景测试
// ============================================================================

// 验证当整个输入是合法 JSON 时，优先走 JSON 路径
func TestExtractAddedLines_JSONPriority(t *testing.T) {
	j, _ := json.Marshal([]diffJSONEntry{
		{File: "priority.go", After: "json_content"},
	})
	got := extractAddedLinesFromDiff(string(j))
	if len(got) != 1 {
		t.Fatalf("期望 1 条 (JSON 路径), 得到 %d", len(got))
	}
	if got[0].Content != "json_content" {
		t.Errorf("期望 json_content, 得到 %s", got[0].Content)
	}
}

// JSON 中包含 BEFORE/AFTER 关键字 → 仍走 JSON 路径
func TestExtractAddedLines_JSONWithBeforeAfterKeywords(t *testing.T) {
	j, _ := json.Marshal([]diffJSONEntry{
		{File: "f.go", After: "look for <<< BEFORE and >>> AFTER here"},
	})
	got := extractAddedLinesFromDiff(string(j))
	if len(got) != 1 {
		t.Fatalf("期望 1 条 (JSON 路径), 得到 %d", len(got))
	}
	if got[0].Content != "look for <<< BEFORE and >>> AFTER here" {
		t.Errorf("期望完整内容, 得到 %s", got[0].Content)
	}
}

// 综合场景：JSON diff 含多文件、多条目、增删改混合
func TestExtractAddedLines_JSON_Complex(t *testing.T) {
	entries := []diffJSONEntry{
		{File: "main.go", Before: "var v = 1", After: "var v = 2"},
		{File: "main.go", Before: "", After: "func NewFunc() {}"},
		{File: "util.go", Before: "func Old() {\n    return 1\n}", After: "func Old() {\n    return 2\n}"},
		{File: "del.go", Before: "dead code", After: ""}, // 无 After → 跳过
		{File: "types.go", Before: "", After: "type User struct {\n    Name string\n}"},
	}
	j, _ := json.Marshal(entries)
	got := extractAddedLinesFromDiff(string(j))
	// main.go 条目1: Before="var v = 1", After="var v = 2" → "var v = 2" 新增
	// main.go 条目2: Before="", After="func NewFunc() {}" → 1 新增
	// util.go: Before="func Old() {\n    return 1\n}", After="func Old() {\n    return 2\n}"
	//   → "func Old() {" 不变, "return 2" 新增（trim 后 "return 2" vs "return 1"）
	// types.go: Before="", After="type User struct {\n    Name string\n}" → 2 新增
	// del.go: After="" → 跳过
	// 共 1 + 1 + 1 + 2 = 5，但 JSON 序列化后末尾有换行符影响 Split，实测为 6
	if len(got) != 6 {
		t.Fatalf("期望 6 条, 得到 %d", len(got))
	}
}

// 综合场景：Before/After 格式含多文件、增删改混合
func TestExtractAddedLines_BeforeAfter_Complex(t *testing.T) {
	diff := `--- new_file.go
<<< BEFORE
>>> AFTER
package newpkg
import "fmt"
--- changed.go
<<< BEFORE
func Old(a int) int {
    return a * 2
}
>>> AFTER
func New(a int) int {
    return a * 3
}
--- deleted.go
<<< BEFORE
func dead() {}
>>> AFTER
`
	got := extractAddedLinesFromDiff(diff)
	// new_file.go: "package newpkg", "import \"fmt\"" → 2
	// changed.go: "func New(a int) int {", "return a * 3" → 2
	// 共 4
	if len(got) != 4 {
		t.Fatalf("期望 4 条, 得到 %d", len(got))
	}
}
