package screen

import "testing"

func pf64(v float64) *float64 { return &v }

func TestCalculateSanhuScore(t *testing.T) {
	// 测试理想公司（所有指标满分）
	ideal := &SanhuScreenResult{
		ROE: 30, ROA: 29.5, DividendYield: pf64(50), DebtRatio: 20,
		CAGR5: pf64(30), CashFlowRatio: pf64(1.2), PB: pf64(0.8),
	}
	score := CalculateSanhuScore(ideal)
	t.Logf("理想公司得分: %.2f", score)
	if score < 90 {
		t.Errorf("理想公司应该得分>=90，实际%.2f", score)
	}

	// 测试典型白马股
	typical := &SanhuScreenResult{
		ROE: 20, ROA: 15, DividendYield: pf64(35), DebtRatio: 45,
		CAGR5: pf64(15), CashFlowRatio: pf64(0.9), PB: pf64(2.5),
	}
	score = CalculateSanhuScore(typical)
	t.Logf("典型白马股得分: %.2f", score)
	if score < 50 || score > 80 {
		t.Errorf("典型白马股得分应在50-80之间，实际%.2f", score)
	}

	// 测试高风险公司
	risky := &SanhuScreenResult{
		ROE: 5, ROA: 2, DividendYield: pf64(0), DebtRatio: 80,
		CAGR5: pf64(-10), CashFlowRatio: pf64(0.3), PB: pf64(8),
	}
	score = CalculateSanhuScore(risky)
	t.Logf("高风险公司得分: %.2f", score)
	if score > 30 {
		t.Errorf("高风险公司应该得分<=30，实际%.2f", score)
	}

	// 测试nil数据
	score = CalculateSanhuScore(nil)
	if score != 0 {
		t.Errorf("nil数据应该返回0分，实际%.2f", score)
	}
}

func TestLinearMap(t *testing.T) {
	// 正常映射
	result := linearMap(5, 0, 10, 0, 100)
	if result != 50 {
		t.Errorf("linearMap(5, 0, 10, 0, 100) = %.2f, want 50", result)
	}

	// 边界外值
	result = linearMap(-5, 0, 10, 0, 100)
	if result != -50 {
		t.Errorf("linearMap(-5, 0, 10, 0, 100) = %.2f, want -50", result)
	}

	// 相等边界
	result = linearMap(5, 10, 10, 0, 100)
	if result != 50 {
		t.Errorf("linearMap with equal bounds should return 50, got %.2f", result)
	}
}

func TestPassThresholdFilter(t *testing.T) {
	params := SanhuScreenParams{
		MinROE: 15, MinDividendYield: 30, MaxDebtRatio: 50,
		MinCAGR5: 10, MinCashFlowRatio: 0.8, MaxPB: 5,
	}

	// 满足所有条件
	good := SanhuScreenResult{ROE: 20, DividendYield: pf64(35), DebtRatio: 40, CAGR5: pf64(15), CashFlowRatio: pf64(1.0), PB: pf64(3)}
	if !passThresholdFilter(good, params) {
		t.Error("应该通过筛选")
	}

	// ROE不足
	badROE := SanhuScreenResult{ROE: 10, DividendYield: pf64(35), DebtRatio: 40, CAGR5: pf64(15), CashFlowRatio: pf64(1.0), PB: pf64(3)}
	if passThresholdFilter(badROE, params) {
		t.Error("ROE不足应该被过滤")
	}

	// 负债率过高
	badDebt := SanhuScreenResult{ROE: 20, DividendYield: pf64(35), DebtRatio: 60, CAGR5: pf64(15), CashFlowRatio: pf64(1.0), PB: pf64(3)}
	if passThresholdFilter(badDebt, params) {
		t.Error("负债率过高应该被过滤")
	}
}
