package main

import (
	"encoding/json"
	"math"
	"os"
	"testing"
	"time"
)

func mkRFC3339(s string) string {
	t, err := time.Parse("2006-01-02 15:04:05", s)
	if err != nil {
		panic(err)
	}
	return t.Format(time.RFC3339)
}

func convWithStart(startTimes ...*string) []time.Time {
	var out []time.Time
	for _, st := range startTimes {
		if st != nil {
			t, err := time.Parse(time.RFC3339, *st)
			if err != nil {
				panic(err)
			}
			out = append(out, t)
		}
	}
	return out
}

func ptrStr(s string) *string { return &s }

func assertFloat(t *testing.T, name string, got, want, epsilon float64) {
	t.Helper()
	if math.Abs(got-want) > epsilon {
		t.Errorf("%s = %.4f, want %.4f (epsilon=%.4f)", name, got, want, epsilon)
	}
}

// ============================================================
// 测试点: calcTaskRealMinutes 核心算法
// ============================================================

// 1.1 全部对话 start_time 为空 → 0 条有效对话
func TestCalcTaskRealMinutes_NoValidConversations(t *testing.T) {
	convs := convWithStart(nil, nil, nil)
	mins, reason := calcTaskRealMinutes(convs, 30, 5)
	assertFloat(t, "minutes", mins, 0, 0.01)
	if reason != "无有效对话" {
		t.Errorf("reason = %q, want %q", reason, "无有效对话")
	}
}

// 1.2 空切片
func TestCalcTaskRealMinutes_EmptySlice(t *testing.T) {
	mins, reason := calcTaskRealMinutes(nil, 30, 5)
	assertFloat(t, "minutes", mins, 0, 0.01)
	if reason != "无有效对话" {
		t.Errorf("reason = %q, want %q", reason, "无有效对话")
	}
}

// 1.3 仅 1 条有效对话 → 返回 extension 分钟
func TestCalcTaskRealMinutes_SingleConversation(t *testing.T) {
	t1 := mkRFC3339("2026-04-01 10:00:00")
	convs := convWithStart(ptrStr(t1))
	mins, reason := calcTaskRealMinutes(convs, 30, 5)
	assertFloat(t, "minutes", mins, 5, 0.01)
	if reason != "仅1条对话，默认5分钟" {
		t.Errorf("reason = %q, want %q", reason, "仅1条对话，默认5分钟")
	}
}

// 1.4 1 条有效 + 若干空start_time → 同上，仅计入有效对话
func TestCalcTaskRealMinutes_SingleValidAmongEmpty(t *testing.T) {
	t1 := mkRFC3339("2026-04-01 10:00:00")
	convs := convWithStart(nil, ptrStr(t1), nil)
	mins, reason := calcTaskRealMinutes(convs, 30, 5)
	assertFloat(t, "minutes", mins, 5, 0.01)
	if reason != "仅1条对话，默认5分钟" {
		t.Errorf("reason = %q, want %q", reason, "仅1条对话，默认5分钟")
	}
}

// 1.5 两条连续对话（间隔 < 30 分钟）→ 归入同一片段
func TestCalcTaskRealMinutes_TwoContinuous(t *testing.T) {
	t1 := mkRFC3339("2026-04-01 10:00:00")
	t2 := mkRFC3339("2026-04-01 10:20:00")
	convs := convWithStart(ptrStr(t1), ptrStr(t2))
	mins, _ := calcTaskRealMinutes(convs, 30, 5)
	assertFloat(t, "minutes", mins, 25, 0.01)
}

// 1.6 两条对话间隔 > 30 分钟 → 断开为 2 个片段
func TestCalcTaskRealMinutes_GapBreak(t *testing.T) {
	t1 := mkRFC3339("2026-04-01 10:00:00")
	t2 := mkRFC3339("2026-04-01 11:00:00")
	convs := convWithStart(ptrStr(t1), ptrStr(t2))
	mins, _ := calcTaskRealMinutes(convs, 30, 5)
	assertFloat(t, "minutes", mins, 10, 0.01)
}

// 1.7 恰好 30 分钟间隔（边界值）→ 应归入同一片段（<= gapThreshold）
func TestCalcTaskRealMinutes_ExactThreshold30Min(t *testing.T) {
	t1 := mkRFC3339("2026-04-01 10:00:00")
	t2 := mkRFC3339("2026-04-01 10:30:00")
	convs := convWithStart(ptrStr(t1), ptrStr(t2))
	mins, _ := calcTaskRealMinutes(convs, 30, 5)
	assertFloat(t, "minutes", mins, 35, 0.01)
}

// 1.8 间隔 30 分钟 1 秒 → 应断开（> gapThreshold）
func TestCalcTaskRealMinutes_JustOverThreshold(t *testing.T) {
	ts := mkRFC3339("2026-04-01 10:00:00")
	ts2 := mkRFC3339("2026-04-01 10:30:01")
	convs := convWithStart(ptrStr(ts), ptrStr(ts2))
	mins, _ := calcTaskRealMinutes(convs, 30, 5)
	assertFloat(t, "minutes", mins, 10, 0.01)
}

// 1.9 乱序输入 → 应排序后正确计算
func TestCalcTaskRealMinutes_UnorderedInput(t *testing.T) {
	t1 := mkRFC3339("2026-04-01 10:00:00")
	t2 := mkRFC3339("2026-04-01 10:10:00")
	t3 := mkRFC3339("2026-04-01 10:05:00")
	convs := convWithStart(ptrStr(t1), ptrStr(t2), ptrStr(t3))
	mins, _ := calcTaskRealMinutes(convs, 30, 5)
	assertFloat(t, "minutes", mins, 15, 0.01)
}

// 1.10 多个片段（3 个时间段）
func TestCalcTaskRealMinutes_ThreeSegments(t *testing.T) {
	t1 := mkRFC3339("2026-04-01 09:00:00")
	t2 := mkRFC3339("2026-04-01 09:15:00")
	t3 := mkRFC3339("2026-04-01 10:30:00")
	t4 := mkRFC3339("2026-04-01 10:45:00")
	t5 := mkRFC3339("2026-04-01 12:00:00")
	convs := convWithStart(ptrStr(t1), ptrStr(t2), ptrStr(t3), ptrStr(t4), ptrStr(t5))
	mins, _ := calcTaskRealMinutes(convs, 30, 5)
	assertFloat(t, "minutes", mins, 45, 0.01)
}

// 1.11 自定义 gapThreshold=10, extension=3 参数
func TestCalcTaskRealMinutes_CustomParams(t *testing.T) {
	t1 := mkRFC3339("2026-04-01 10:00:00")
	t2 := mkRFC3339("2026-04-01 10:08:00")
	t3 := mkRFC3339("2026-04-01 10:25:00")
	convs := convWithStart(ptrStr(t1), ptrStr(t2), ptrStr(t3))
	mins, _ := calcTaskRealMinutes(convs, 10, 3)
	assertFloat(t, "minutes", mins, 14, 0.01)
}

// ============================================================
// 测试点: splitConversations
// ============================================================

// 2.1 Single valid JSON object → returns [obj]
func TestSplitConversations_SingleValidJSON(t *testing.T) {
	line := `{"a":1}`
	parts, err := splitConversations(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(parts) != 1 || parts[0] != `{"a":1}` {
		t.Errorf("got %v, want [%q]", parts, `{"a":1}`)
	}
}

// 2.2 Multiple valid JSON objects on same line → returns [obj1, obj2]
func TestSplitConversations_MultipleValidJSON(t *testing.T) {
	line := `{"a":1}{"b":2}`
	parts, err := splitConversations(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(parts) != 2 || parts[0] != `{"a":1}` || parts[1] != `{"b":2}` {
		t.Errorf("got %v, want [%q, %q]", parts, `{"a":1}`, `{"b":2}`)
	}
}

// 2.3 Line starting with whitespace then JSON → returns error
func TestSplitConversations_LeadingWhitespace(t *testing.T) {
	line := `   {"a":1}`
	_, err := splitConversations(line)
	if err == nil {
		t.Error("expected error for leading whitespace, got nil")
	}
}

// 2.4 Nested JSON objects → correctly tracks depth
func TestSplitConversations_NestedJSON(t *testing.T) {
	line := `{"a":{"b":1}}`
	parts, err := splitConversations(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(parts) != 1 || parts[0] != `{"a":{"b":1}}` {
		t.Errorf("got %v, want [%q]", parts, `{"a":{"b":1}}`)
	}
}

// 2.5 JSON with strings containing braces → doesn't get confused
func TestSplitConversations_StringWithBraces(t *testing.T) {
	line := `{"a":"{not brace}"}`
	parts, err := splitConversations(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(parts) != 1 || parts[0] != `{"a":"{not brace}"}` {
		t.Errorf("got %v, want [%q]", parts, `{"a":"{not brace}"}`)
	}
}

// 2.6 Invalid: starts with non-{ → returns error
func TestSplitConversations_StartsWithNonBrace(t *testing.T) {
	line := `notjson{"a":1}`
	_, err := splitConversations(line)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

// 2.7 Invalid: unclosed brace → returns error
func TestSplitConversations_UnclosedBrace(t *testing.T) {
	line := `{"a":1`
	_, err := splitConversations(line)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

// 2.8 Empty string → error
func TestSplitConversations_EmptyString(t *testing.T) {
	line := ""
	_, err := splitConversations(line)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

// ============================================================
// 测试点: parseUserInput
// ============================================================

// 3.1 Normal wrapped: <user_message>hello</user_message> → "hello"
func TestParseUserInput_NormalWrapped(t *testing.T) {
	input := `<user_message>hello</user_message>`
	got := parseUserInput(input)
	if got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

// 3.2 No prefix → returns original
func TestParseUserInput_NoPrefix(t *testing.T) {
	input := `hello world`
	got := parseUserInput(input)
	if got != "hello world" {
		t.Errorf("got %q, want %q", got, "hello world")
	}
}

// 3.3 Prefix but no suffix → returns original
func TestParseUserInput_PrefixNoSuffix(t *testing.T) {
	input := `<user_message>hello`
	got := parseUserInput(input)
	if got != `<user_message>hello` {
		t.Errorf("got %q, want %q", got, `<user_message>hello`)
	}
}

// 3.4 Empty string → returns empty
func TestParseUserInput_EmptyString(t *testing.T) {
	input := ""
	got := parseUserInput(input)
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

// 3.5 Multiple occurrences of suffix → takes first after prefix
func TestParseUserInput_MultipleSuffixes(t *testing.T) {
	input := `<user_message>hello</user_message>world</user_message>`
	got := parseUserInput(input)
	if got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

// ============================================================
// 测试点: skeletonize
// ============================================================

// 4.1 content shorter than maxSize → returns as-is
func TestSkeletonize_ShorterThanMaxSize(t *testing.T) {
	content := "hello"
	got := skeletonize(content, 3, 10)
	if got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

// 4.2 content longer than maxSize → head + "..." + tail
func TestSkeletonize_LongerThanMaxSize(t *testing.T) {
	content := "hello world example"
	got := skeletonize(content, 5, 12)
	want := "hello...mple"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// 4.3 head > len(content) → head clamped
func TestSkeletonize_HeadGreaterThanLength(t *testing.T) {
	content := "hi"
	got := skeletonize(content, 10, 5)
	if got != "hi" {
		t.Errorf("got %q, want %q", got, "hi")
	}
}

// 4.4 maxSize < head + 3 → tail can be 0 or negative
func TestSkeletonize_MaxSizeLessThanHeadPlus3(t *testing.T) {
	content := "hello world"
	got := skeletonize(content, 10, 5)
	want := "hello worl..."
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// 4.5 head = 0 → "..." + tail
func TestSkeletonize_HeadZero(t *testing.T) {
	content := "hello world"
	got := skeletonize(content, 0, 8)
	want := "...world"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// 4.6 maxSize = 0 → head clamped, tail = maxSize - head - 3 could be negative
func TestSkeletonize_MaxSizeZero(t *testing.T) {
	content := "hello world"
	got := skeletonize(content, 20, 0)
	want := "hello world..."
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// ============================================================
// 测试点: flexString.UnmarshalJSON
// ============================================================

// 5.1 null → empty string
func TestFlexString_UnmarshalJSON_Null(t *testing.T) {
	var f flexString
	err := f.UnmarshalJSON([]byte("null"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f != "" {
		t.Errorf("got %q, want empty", f)
	}
}

// 5.2 "hello" → "hello"
func TestFlexString_UnmarshalJSON_String(t *testing.T) {
	var f flexString
	err := f.UnmarshalJSON([]byte(`"hello"`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f != "hello" {
		t.Errorf("got %q, want %q", f, "hello")
	}
}

// 5.3 123 → "123" (number to string)
func TestFlexString_UnmarshalJSON_Number(t *testing.T) {
	var f flexString
	err := f.UnmarshalJSON([]byte("123"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f != "123" {
		t.Errorf("got %q, want %q", f, "123")
	}
}

// 5.4 Invalid like { → error
func TestFlexString_UnmarshalJSON_Invalid(t *testing.T) {
	var f flexString
	err := f.UnmarshalJSON([]byte("{"))
	if err == nil {
		t.Error("expected error, got nil")
	}
}

// 5.5 "" → empty string
func TestFlexString_UnmarshalJSON_EmptyString(t *testing.T) {
	var f flexString
	err := f.UnmarshalJSON([]byte(`""`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f != "" {
		t.Errorf("got %q, want empty", f)
	}
}

// ============================================================
// 测试点: needUpdateConversations
// ============================================================

// 6.1 force=true → always returns true
func TestNeedUpdateConversations_ForceTrue(t *testing.T) {
	got := needUpdateConversations("any", "any", true)
	if !got {
		t.Error("expected true when force=true")
	}
}

// 6.2 force=false, silica file doesn't exist → returns true
func TestNeedUpdateConversations_SilicaNotExist(t *testing.T) {
	convFile, err := os.CreateTemp("", "conv_*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(convFile.Name())
	if _, err := convFile.WriteString("data"); err != nil {
		t.Fatal(err)
	}
	convFile.Close()

	got := needUpdateConversations(convFile.Name(), "/nonexistent/path/silica.json", false)
	if !got {
		t.Error("expected true when silica file does not exist")
	}
}

// 6.3 force=false, silica file exists but invalid JSON → returns true
func TestNeedUpdateConversations_SilicaInvalidJSON(t *testing.T) {
	convFile, err := os.CreateTemp("", "conv_*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(convFile.Name())
	if _, err := convFile.WriteString("data"); err != nil {
		t.Fatal(err)
	}
	convFile.Close()

	silicaFile, err := os.CreateTemp("", "silica_*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(silicaFile.Name())
	if _, err := silicaFile.WriteString("not json"); err != nil {
		t.Fatal(err)
	}
	silicaFile.Close()

	got := needUpdateConversations(convFile.Name(), silicaFile.Name(), false)
	if !got {
		t.Error("expected true when silica file has invalid JSON")
	}
}

// 6.4 force=false, silica file exists with valid JSON, file sizes differ → returns true
func TestNeedUpdateConversations_FileSizesDiffer(t *testing.T) {
	convFile, err := os.CreateTemp("", "conv_*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(convFile.Name())
	if _, err := convFile.WriteString("newer data here"); err != nil {
		t.Fatal(err)
	}
	convFile.Close()

	silicaFile, err := os.CreateTemp("", "silica_*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(silicaFile.Name())
	data, _ := json.Marshal(taskSilicaData{Size: 5})
	if _, err := silicaFile.Write(data); err != nil {
		t.Fatal(err)
	}
	silicaFile.Close()

	got := needUpdateConversations(convFile.Name(), silicaFile.Name(), false)
	if !got {
		t.Error("expected true when file sizes differ")
	}
}

// 6.5 force=false, silica file exists with valid JSON, file sizes same → returns false
func TestNeedUpdateConversations_FileSizesSame(t *testing.T) {
	convFile, err := os.CreateTemp("", "conv_*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(convFile.Name())
	content := []byte("some data")
	if _, err := convFile.Write(content); err != nil {
		t.Fatal(err)
	}
	convFile.Close()

	silicaFile, err := os.CreateTemp("", "silica_*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(silicaFile.Name())
	data, _ := json.Marshal(taskSilicaData{Size: int64(len(content))})
	if _, err := silicaFile.Write(data); err != nil {
		t.Fatal(err)
	}
	silicaFile.Close()

	got := needUpdateConversations(convFile.Name(), silicaFile.Name(), false)
	if got {
		t.Error("expected false when file sizes are the same")
	}
}

// ============================================================
// 测试点: countDiffLines
// ============================================================

// 7.1 Empty string → 0
func TestCountDiffLines_EmptyString(t *testing.T) {
	got := countDiffLines("")
	if got != 0 {
		t.Errorf("got %d, want 0", got)
	}
}

// 7.2 Unified diff format → count + and - lines
func TestCountDiffLines_UnifiedDiff(t *testing.T) {
	diff := "--- a/file.go\n+++ b/file.go\n@@ -1,3 +1,3 @@\n line1\n-line2\n+line2modified\n line3"
	got := countDiffLines(diff)
	if got != 2 {
		t.Errorf("got %d, want 2", got)
	}
}

// 7.3 JSON diff format → additions + deletions from entries
func TestCountDiffLines_JSONDiff(t *testing.T) {
	diff := `[{"file":"a.go","before":"","after":"line1\nline2","additions":3,"deletions":1,"status":"modified"}]`
	got := countDiffLines(diff)
	if got != 4 {
		t.Errorf("got %d, want 4", got)
	}
}

// 7.4 Before/After format → count differences
func TestCountDiffLines_BeforeAfter(t *testing.T) {
	diff := "--- file.go\n<<< BEFORE\nold line1\nold line2\n>>> AFTER\nnew line1\nnew line2\nnew line3"
	got := countDiffLines(diff)
	if got != 5 {
		t.Errorf("got %d, want 5", got)
	}
}

// 7.5 Mixed but no recognizable format → unified fallback
func TestCountDiffLines_MixedFallback(t *testing.T) {
	diff := "+added line\n-removed line\n some context"
	got := countDiffLines(diff)
	if got != 2 {
		t.Errorf("got %d, want 2", got)
	}
}
