package main

import (
	"math"
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

func convWithStart(startTimes ...*string) []taskConversation {
	var out []taskConversation
	for _, st := range startTimes {
		c := taskConversation{
			RequestId: "req-test",
		}
		if st != nil {
			c.StartTime = *st
		}
		out = append(out, c)
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

// 1.12 start_time 格式异常 → 跳过无效解析
func TestCalcTaskRealMinutes_InvalidTimeFormat(t *testing.T) {
	t1 := mkRFC3339("2026-04-01 10:00:00")
	convs := []taskConversation{
		{StartTime: "not-a-valid-time", RequestId: "r1"},
		{StartTime: t1, RequestId: "r2"},
	}
	mins, reason := calcTaskRealMinutes(convs, 30, 5)
	assertFloat(t, "minutes", mins, 5, 0.01)
	if reason != "仅1条对话，默认5分钟" {
		t.Errorf("reason = %q", reason)
	}
}
