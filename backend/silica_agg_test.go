package main

import "testing"

func TestCalcSilicaRatio_NilWhenNoWeight(t *testing.T) {
	// 无可计入行 → nil（前端渲染 '-'），与「真 0」区分。
	if got := calcSilicaRatio(0, 0); got != nil {
		t.Fatalf("weight=0 应返回 nil，got %v", *got)
	}
	if got := calcSilicaRatio(12.5, 0); got != nil {
		t.Fatalf("weight=0 即使有分子也应返回 nil，got %v", *got)
	}
	if got := calcSilicaRatio(0, -3); got != nil {
		t.Fatalf("weight<0 应返回 nil，got %v", *got)
	}
}

// 含硅量必须先还原「匹配行/总行」再求比。本例是最容易出错的场景：
// 一个 3 行的小 commit 全中(1.0)，一个 300 行的大 commit 几乎没中(0.01)。
// 正确口径 = (3*1.0 + 300*0.01) / 303 = 6/303 ≈ 0.0198
// 错误口径（对比值直接平均）= (1.0 + 0.01)/2 = 0.505 —— 会把整体虚高 25 倍。
func TestCalcSilicaRatio_WeightedNotAveraged(t *testing.T) {
	weighted := 3*1.0 + 300*0.01
	weight := int64(3 + 300)

	got := calcSilicaRatio(weighted, weight)
	if got == nil {
		t.Fatal("有权重时不应返回 nil")
	}
	want := 6.0 / 303.0
	if diff := *got - want; diff > 1e-9 || diff < -1e-9 {
		t.Fatalf("加权口径错误: got %v, want %v", *got, want)
	}
	// 守住与「比值平均」的差距，防有人改回简单平均。
	if naive := (1.0 + 0.01) / 2; *got > naive/10 {
		t.Fatalf("结果 %v 接近比值平均 %v，加权口径可能被改坏", *got, naive)
	}
}

func TestCalcSilicaRatio_FullAndZeroMatch(t *testing.T) {
	// 全中 → 1.0
	if got := calcSilicaRatio(240*1.0, 240); got == nil || *got != 1.0 {
		t.Fatalf("全中应为 1.0，got %v", got)
	}
	// 一行没中 → 0（**不是 nil**）：有 commit 有代码行但指纹全不匹配，
	// 是"AI 没写"的事实，不是"无数据"，必须与 nil 区分。
	got := calcSilicaRatio(0, 240)
	if got == nil {
		t.Fatal("有行数但零匹配应为 0，不应是 nil（会被误显示成无数据）")
	}
	if *got != 0 {
		t.Fatalf("零匹配应为 0，got %v", *got)
	}
}
