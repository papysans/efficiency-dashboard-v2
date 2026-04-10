package main

import (
	"math"
	"testing"
	"time"
)

// --- helper ---

func mkTime(s string) time.Time {
	t, err := time.Parse("2006-01-02 15:04:05", s)
	if err != nil {
		panic(err)
	}
	return t
}

func convWithStart(startTimes ...*time.Time) []StatTaskConversation {
	var out []StatTaskConversation
	for i, st := range startTimes {
		out = append(out, StatTaskConversation{
			ID:        i + 1,
			TaskID:    "test-task",
			RequestID: "req-" + time.Now().Format("150405"),
			StartTime: st,
		})
	}
	return out
}

func assertFloat(t *testing.T, name string, got, want, epsilon float64) {
	t.Helper()
	if math.Abs(got-want) > epsilon {
		t.Errorf("%s = %.4f, want %.4f (epsilon=%.4f)", name, got, want, epsilon)
	}
}

// ============================================================
// 测试点 1: calculateTaskRealMinutes 核心算法
// ============================================================

// 1.1 全部对话 start_time 为 nil → 0 条有效对话
func TestCalculateTaskRealMinutes_NoValidConversations(t *testing.T) {
	convs := convWithStart(nil, nil, nil)
	mins, reason, segs := calculateTaskRealMinutes(convs, 30, 5)
	assertFloat(t, "minutes", mins, 0, 0.01)
	if reason != "无有效对话" {
		t.Errorf("reason = %q, want %q", reason, "无有效对话")
	}
	if segs != nil {
		t.Errorf("segments should be nil, got %v", segs)
	}
}

// 1.2 空切片
func TestCalculateTaskRealMinutes_EmptySlice(t *testing.T) {
	mins, reason, segs := calculateTaskRealMinutes(nil, 30, 5)
	assertFloat(t, "minutes", mins, 0, 0.01)
	if reason != "无有效对话" {
		t.Errorf("reason = %q, want %q", reason, "无有效对话")
	}
	if segs != nil {
		t.Errorf("segments should be nil")
	}
}

// 1.3 仅 1 条有效对话 → 返回 extension 分钟
func TestCalculateTaskRealMinutes_SingleConversation(t *testing.T) {
	t1 := mkTime("2026-04-01 10:00:00")
	convs := convWithStart(ptrTime(t1))
	mins, reason, segs := calculateTaskRealMinutes(convs, 30, 5)
	assertFloat(t, "minutes", mins, 5, 0.01)
	if reason != "仅1条对话，默认5分钟" {
		t.Errorf("reason = %q", reason)
	}
	if len(segs) != 1 {
		t.Fatalf("segments count = %d, want 1", len(segs))
	}
	if segs[0].ConvCount != 1 {
		t.Errorf("seg[0].ConvCount = %d, want 1", segs[0].ConvCount)
	}
	expectedEnd := t1.Add(5 * time.Minute)
	if !segs[0].End.Equal(expectedEnd) {
		t.Errorf("seg[0].End = %v, want %v", segs[0].End, expectedEnd)
	}
}

// 1.4 1 条有效 + 若干 nil → 同上，仅计入有效对话
func TestCalculateTaskRealMinutes_SingleValidAmongNils(t *testing.T) {
	t1 := mkTime("2026-04-01 10:00:00")
	convs := convWithStart(nil, ptrTime(t1), nil)
	mins, _, segs := calculateTaskRealMinutes(convs, 30, 5)
	assertFloat(t, "minutes", mins, 5, 0.01)
	if len(segs) != 1 {
		t.Fatalf("segments count = %d, want 1", len(segs))
	}
}

// 1.5 两条连续对话（间隔 < 30 分钟）→ 归入同一片段
func TestCalculateTaskRealMinutes_TwoContinuous(t *testing.T) {
	t1 := mkTime("2026-04-01 10:00:00")
	t2 := mkTime("2026-04-01 10:20:00") // 间隔 20 分钟
	convs := convWithStart(ptrTime(t1), ptrTime(t2))
	mins, _, segs := calculateTaskRealMinutes(convs, 30, 5)
	// 片段: 10:00 ~ 10:20+5min=10:25 → 25 分钟
	assertFloat(t, "minutes", mins, 25, 0.01)
	if len(segs) != 1 {
		t.Fatalf("segments count = %d, want 1", len(segs))
	}
	if segs[0].ConvCount != 2 {
		t.Errorf("seg[0].ConvCount = %d, want 2", segs[0].ConvCount)
	}
}

// 1.6 两条对话间隔 > 30 分钟 → 断开为 2 个片段
func TestCalculateTaskRealMinutes_GapBreak(t *testing.T) {
	t1 := mkTime("2026-04-01 10:00:00")
	t2 := mkTime("2026-04-01 11:00:00") // 间隔 60 分钟
	convs := convWithStart(ptrTime(t1), ptrTime(t2))
	mins, _, segs := calculateTaskRealMinutes(convs, 30, 5)
	// 片段1: 10:00 ~ 10:00+5min=10:05 → 5 分钟
	// 片段2: 11:00 ~ 11:00+5min=11:05 → 5 分钟
	assertFloat(t, "minutes", mins, 10, 0.01)
	if len(segs) != 2 {
		t.Fatalf("segments count = %d, want 2", len(segs))
	}
	if segs[0].ConvCount != 1 || segs[1].ConvCount != 1 {
		t.Errorf("conv counts = [%d, %d], want [1, 1]", segs[0].ConvCount, segs[1].ConvCount)
	}
}

// 1.7 恰好 30 分钟间隔（边界值）→ 应归入同一片段（<= gapThreshold）
func TestCalculateTaskRealMinutes_ExactThreshold30Min(t *testing.T) {
	t1 := mkTime("2026-04-01 10:00:00")
	t2 := mkTime("2026-04-01 10:30:00") // 恰好 30 分钟
	convs := convWithStart(ptrTime(t1), ptrTime(t2))
	mins, _, segs := calculateTaskRealMinutes(convs, 30, 5)
	// 同一片段: 10:00 ~ 10:30+5min=10:35 → 35 分钟
	assertFloat(t, "minutes", mins, 35, 0.01)
	if len(segs) != 1 {
		t.Fatalf("segments count = %d, want 1 (gap exactly at threshold should merge)", len(segs))
	}
}

// 1.8 间隔 30 分钟 1 秒 → 应断开（> gapThreshold）
func TestCalculateTaskRealMinutes_JustOverThreshold(t *testing.T) {
	t1 := mkTime("2026-04-01 10:00:00")
	t2 := t1.Add(30*time.Minute + 1*time.Second) // 30分01秒
	convs := convWithStart(ptrTime(t1), ptrTime(t2))
	mins, _, segs := calculateTaskRealMinutes(convs, 30, 5)
	// 断开为2个片段，各5分钟 = 10分钟
	assertFloat(t, "minutes", mins, 10, 0.01)
	if len(segs) != 2 {
		t.Fatalf("segments count = %d, want 2 (gap just over threshold should split)", len(segs))
	}
}

// 1.9 乱序输入 → 应排序后正确计算
func TestCalculateTaskRealMinutes_UnorderedInput(t *testing.T) {
	t1 := mkTime("2026-04-01 10:00:00")
	t2 := mkTime("2026-04-01 10:10:00")
	t3 := mkTime("2026-04-01 10:05:00")
	// 输入顺序: t1, t2, t3（乱序）
	convs := convWithStart(ptrTime(t1), ptrTime(t2), ptrTime(t3))
	mins, _, segs := calculateTaskRealMinutes(convs, 30, 5)
	// 排序后: t1(10:00), t3(10:05), t2(10:10)
	// 同一片段: 10:00 ~ 10:10+5min=10:15 → 15 分钟
	assertFloat(t, "minutes", mins, 15, 0.01)
	if len(segs) != 1 {
		t.Fatalf("segments count = %d, want 1", len(segs))
	}
	if segs[0].ConvCount != 3 {
		t.Errorf("seg[0].ConvCount = %d, want 3", segs[0].ConvCount)
	}
}

// 1.10 多个片段（3 个时间段）
func TestCalculateTaskRealMinutes_ThreeSegments(t *testing.T) {
	t1 := mkTime("2026-04-01 09:00:00")
	t2 := mkTime("2026-04-01 09:15:00") // 间隔15min < 30 → 合并
	t3 := mkTime("2026-04-01 10:30:00") // 间隔75min > 30 → 断开
	t4 := mkTime("2026-04-01 10:45:00") // 间隔15min < 30 → 合并
	t5 := mkTime("2026-04-01 12:00:00") // 间隔75min > 30 → 断开
	convs := convWithStart(ptrTime(t1), ptrTime(t2), ptrTime(t3), ptrTime(t4), ptrTime(t5))
	mins, _, segs := calculateTaskRealMinutes(convs, 30, 5)
	// 片段1: 09:00~09:15+5=09:20 → 20min (2条对话)
	// 片段2: 10:30~10:45+5=10:50 → 20min (2条对话)
	// 片段3: 12:00~12:00+5=12:05 → 5min (1条对话)
	// 总计: 45 分钟
	assertFloat(t, "minutes", mins, 45, 0.01)
	if len(segs) != 3 {
		t.Fatalf("segments count = %d, want 3", len(segs))
	}
	if segs[0].ConvCount != 2 || segs[1].ConvCount != 2 || segs[2].ConvCount != 1 {
		t.Errorf("conv counts = [%d, %d, %d], want [2, 2, 1]",
			segs[0].ConvCount, segs[1].ConvCount, segs[2].ConvCount)
	}
}

// 1.11 自定义 gapThreshold=10, extension=3 参数
func TestCalculateTaskRealMinutes_CustomParams(t *testing.T) {
	t1 := mkTime("2026-04-01 10:00:00")
	t2 := mkTime("2026-04-01 10:08:00") // 间隔8min < 10 → 合并
	t3 := mkTime("2026-04-01 10:25:00") // 间隔17min > 10 → 断开
	convs := convWithStart(ptrTime(t1), ptrTime(t2), ptrTime(t3))
	mins, _, segs := calculateTaskRealMinutes(convs, 10, 3)
	// 片段1: 10:00~10:08+3=10:11 → 11min
	// 片段2: 10:25~10:25+3=10:28 → 3min
	// 总计: 14 分钟
	assertFloat(t, "minutes", mins, 14, 0.01)
	if len(segs) != 2 {
		t.Fatalf("segments count = %d, want 2", len(segs))
	}
}

// ============================================================
// 测试点 2: efficiency_ratio 计算逻辑（模拟 getTaskDetailV2 中的计算）
// ============================================================

// computeEfficiencyRatio 复制 getTaskDetailV2 中的 efficiency_ratio 计算逻辑供单元测试
func computeEfficiencyRatio(ancient, ancientManual, real, realManual *float64) *float64 {
	effectiveAncient := ancient
	if ancientManual != nil {
		effectiveAncient = ancientManual
	}
	effectiveReal := real
	if realManual != nil {
		effectiveReal = realManual
	}
	if effectiveAncient != nil && effectiveReal != nil && *effectiveReal > 0 && *effectiveAncient > 0 {
		ratio := (*effectiveAncient / *effectiveReal) * 100
		return &ratio
	}
	return nil
}

func ptrFloat(v float64) *float64 { return &v }

// 2.1 正常情况: ancient=480, real=120 → ratio=400%
func TestEfficiencyRatio_Normal(t *testing.T) {
	ratio := computeEfficiencyRatio(ptrFloat(480), nil, ptrFloat(120), nil)
	if ratio == nil {
		t.Fatal("ratio should not be nil")
	}
	assertFloat(t, "ratio", *ratio, 400.0, 0.01)
}

// 2.2 manual 优先: ancientManual=960, realManual=120 → 800%
func TestEfficiencyRatio_ManualPriority(t *testing.T) {
	ratio := computeEfficiencyRatio(ptrFloat(480), ptrFloat(960), ptrFloat(240), ptrFloat(120))
	if ratio == nil {
		t.Fatal("ratio should not be nil")
	}
	// 960 / 120 * 100 = 800
	assertFloat(t, "ratio", *ratio, 800.0, 0.01)
}

// 2.3 ancient 为 nil → ratio 为 nil
func TestEfficiencyRatio_AncientNil(t *testing.T) {
	ratio := computeEfficiencyRatio(nil, nil, ptrFloat(120), nil)
	if ratio != nil {
		t.Errorf("ratio should be nil when ancient is nil, got %f", *ratio)
	}
}

// 2.4 real=0 → ratio 为 nil
func TestEfficiencyRatio_RealZero(t *testing.T) {
	ratio := computeEfficiencyRatio(ptrFloat(480), nil, ptrFloat(0), nil)
	if ratio != nil {
		t.Errorf("ratio should be nil when real=0, got %f", *ratio)
	}
}

// 2.5 real 为 nil → ratio 为 nil
func TestEfficiencyRatio_RealNil(t *testing.T) {
	ratio := computeEfficiencyRatio(ptrFloat(480), nil, nil, nil)
	if ratio != nil {
		t.Errorf("ratio should be nil when real is nil, got %f", *ratio)
	}
}

// 2.6 both nil → ratio 为 nil
func TestEfficiencyRatio_BothNil(t *testing.T) {
	ratio := computeEfficiencyRatio(nil, nil, nil, nil)
	if ratio != nil {
		t.Errorf("ratio should be nil when both nil, got %f", *ratio)
	}
}

// 2.7 ancient=0 → ratio 为 nil (ancient > 0 条件不满足)
func TestEfficiencyRatio_AncientZero(t *testing.T) {
	ratio := computeEfficiencyRatio(ptrFloat(0), nil, ptrFloat(120), nil)
	if ratio != nil {
		t.Errorf("ratio should be nil when ancient=0, got %f", *ratio)
	}
}

// 2.8 仅 ancientManual 存在, real 正常
func TestEfficiencyRatio_OnlyAncientManual(t *testing.T) {
	ratio := computeEfficiencyRatio(nil, ptrFloat(600), ptrFloat(120), nil)
	if ratio == nil {
		t.Fatal("ratio should not be nil")
	}
	assertFloat(t, "ratio", *ratio, 500.0, 0.01)
}
