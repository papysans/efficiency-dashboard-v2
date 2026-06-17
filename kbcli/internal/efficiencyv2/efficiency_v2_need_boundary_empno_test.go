package efficiencyv2

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"kanban/core/models"
)

// 工号去重前提下的集成流改判：贡献者数按工号(emp_no)而非碎片 user_id。
// 实测一人多账号（工号 25163 挂 3 个 user_id）旧口径会被 user_id 计数误判 ≥3 人
// 而错误降级；改数工号后单工号 need 不再触发 integration_flow、恢复 eligible。

// 工号 emp map：3 个碎片 user_id 同工号 25163，外加两个独立工号供多工号用例。
func efficiencyV2EmpNoTestMap() *EfficiencyV2UserEmpMap {
	return BuildEfficiencyV2UserEmpMapFromRows([]efficiencyV2EmpCommitRow{
		// add-kb-tyh-B 形态：一个人(25163)三个账号
		{UserId: "u-a", EmpNo: "25163"},
		{UserId: "u-b", EmpNo: "25163"},
		{UserId: "u-c", EmpNo: "25163"},
		// 另外两个真实独立工号
		{UserId: "u-x", EmpNo: "90331"},
		{UserId: "u-y", EmpNo: "70811"},
	})
}

// 碎片：3 个不同 user_id 同工号 + 跨度 8 天 → 有效贡献者 = 1 工号 < 3 → 不降级、
// emp_no_count==1、contributor_emp_nos=[25163]、仍 eligible。
func TestEfficiencyV2IntegrationFlowEmpNoFragmentNotDowngraded(t *testing.T) {
	base := time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC)
	repo := "git@example.com/acme/app.git"
	branch := "feature/add-kb-tyh-B"
	// 三段间隙都 < 3 天（不触发 episode 切分），总跨度 8 天 > IntegrationFlowSpanDays(7)
	metrics := []models.SessionStageMetric{
		efficiencyV2NeedTestMetric("s-frag-a", "u-a", repo, branch, base, base.Add(time.Hour)),
		efficiencyV2NeedTestMetric("s-frag-b", "u-b", repo, branch, base.Add(3*24*time.Hour), base.Add(3*24*time.Hour+time.Hour)),
		efficiencyV2NeedTestMetric("s-frag-c", "u-c", repo, branch, base.Add(6*24*time.Hour), base.Add(6*24*time.Hour+time.Hour)),
	}
	commit := efficiencyV2NeedTestCommit("c-frag", "u-a", repo, branch, base.Add(8*24*time.Hour), "merge add-kb-tyh-B")

	needs := ResolveEfficiencyV2NeedsWithEmpMap(metrics, nil, []models.Commit{commit}, EfficiencyV2Config{}, efficiencyV2EmpNoTestMap())
	if len(needs) != 1 {
		t.Fatalf("need count: want 1, got %d", len(needs))
	}
	need := needs[0]
	if need.BoundaryConfidence != efficiencyV2ConfidenceHigh {
		t.Fatalf("单工号碎片 need 不应被集成流降级，confidence = %s want high", need.BoundaryConfidence)
	}
	if !need.CoverageEligible {
		t.Fatal("单工号碎片 need 应保持 coverage eligible（不再被误降级排除）")
	}
	if strings.Contains(need.Reason, "integration flow") {
		t.Fatalf("不应记录 integration flow 降级，reason = %q", need.Reason)
	}
	if need.EmpNoCount != 1 {
		t.Fatalf("emp_no_count = %d, want 1（3 个 user_id 折叠为 1 工号）", need.EmpNoCount)
	}
	if got := EfficiencyV2StringsFromJSON(need.ContributorEmpNos); !reflect.DeepEqual(got, []string{"25163"}) {
		t.Fatalf("contributor_emp_nos = %v, want [25163]", got)
	}
	// 三个碎片 user_id 仍完整保留在 contributor_user_ids（工号去重不改原始贡献者集）
	if got := EfficiencyV2StringsFromJSON(need.ContributorUserIds); len(got) != 3 {
		t.Fatalf("contributor_user_ids 应保留 3 个原始 user_id，got %v", got)
	}
}

// 真多工号：3 个不同工号 + 跨度 8 天 → 有效贡献者 = 3 工号 ≥ 3 → 仍降级、
// emp_no_count==3、不 eligible。
func TestEfficiencyV2IntegrationFlowMultiEmpNoStillDowngraded(t *testing.T) {
	base := time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC)
	repo := "git@example.com/acme/app.git"
	branch := "feature/real-integration-train"
	metrics := []models.SessionStageMetric{
		efficiencyV2NeedTestMetric("s-multi-a", "u-a", repo, branch, base, base.Add(time.Hour)),                                    // 25163
		efficiencyV2NeedTestMetric("s-multi-x", "u-x", repo, branch, base.Add(3*24*time.Hour), base.Add(3*24*time.Hour+time.Hour)), // 90331
		efficiencyV2NeedTestMetric("s-multi-y", "u-y", repo, branch, base.Add(6*24*time.Hour), base.Add(6*24*time.Hour+time.Hour)), // 70811
	}
	commit := efficiencyV2NeedTestCommit("c-multi", "u-a", repo, branch, base.Add(8*24*time.Hour), "merge real integration train")

	needs := ResolveEfficiencyV2NeedsWithEmpMap(metrics, nil, []models.Commit{commit}, EfficiencyV2Config{}, efficiencyV2EmpNoTestMap())
	if len(needs) != 1 {
		t.Fatalf("need count: want 1, got %d", len(needs))
	}
	need := needs[0]
	if need.BoundaryConfidence != efficiencyV2ConfidenceLow {
		t.Fatalf("真多工号 need 应被集成流降级为 low，got %s", need.BoundaryConfidence)
	}
	if need.CoverageEligible {
		t.Fatal("真多工号 need 应被降级排除（coverage eligible=false）")
	}
	if !strings.Contains(need.Reason, "integration flow") {
		t.Fatalf("应记录 integration flow 降级，reason = %q", need.Reason)
	}
	if need.EmpNoCount != 3 {
		t.Fatalf("emp_no_count = %d, want 3（三个独立工号）", need.EmpNoCount)
	}
	if got := EfficiencyV2StringsFromJSON(need.ContributorEmpNos); !reflect.DeepEqual(got, []string{"25163", "70811", "90331"}) {
		t.Fatalf("contributor_emp_nos = %v, want 排序后 [25163 70811 90331]", got)
	}
}

// 空 map（dept_user 未灌入 / 全 orphan）：回退按 user_id 数判集成流（旧行为保留）。
// 3 个不同 user_id（即便其实同一个人）+ 跨度 8 天 → 仍降级；emp_no_count==0。
func TestEfficiencyV2IntegrationFlowEmptyMapFallsBackToUserIds(t *testing.T) {
	base := time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC)
	repo := "git@example.com/acme/app.git"
	branch := "feature/no-empmap"
	metrics := []models.SessionStageMetric{
		efficiencyV2NeedTestMetric("s-nm-a", "u-a", repo, branch, base, base.Add(time.Hour)),
		efficiencyV2NeedTestMetric("s-nm-b", "u-b", repo, branch, base.Add(3*24*time.Hour), base.Add(3*24*time.Hour+time.Hour)),
		efficiencyV2NeedTestMetric("s-nm-c", "u-c", repo, branch, base.Add(6*24*time.Hour), base.Add(6*24*time.Hour+time.Hour)),
	}
	commit := efficiencyV2NeedTestCommit("c-nm", "u-a", repo, branch, base.Add(8*24*time.Hour), "merge no empmap")

	// 两条等价路径：显式空 map 与不带 map 的入口，行为必须一致（旧 user_id 口径）。
	for name, needs := range map[string][]models.Need{
		"explicit empty map": ResolveEfficiencyV2NeedsWithEmpMap(metrics, nil, []models.Commit{commit}, EfficiencyV2Config{}, &EfficiencyV2UserEmpMap{}),
		"nil map entrypoint": ResolveEfficiencyV2Needs(metrics, nil, []models.Commit{commit}, EfficiencyV2Config{}),
	} {
		if len(needs) != 1 {
			t.Fatalf("[%s] need count: want 1, got %d", name, len(needs))
		}
		need := needs[0]
		if need.BoundaryConfidence != efficiencyV2ConfidenceLow {
			t.Fatalf("[%s] 空 map 应回退按 user_id 数（3≥3）降级，got %s", name, need.BoundaryConfidence)
		}
		if !strings.Contains(need.Reason, "integration flow") {
			t.Fatalf("[%s] 空 map 回退应仍记录 integration flow 降级，reason=%q", name, need.Reason)
		}
		if need.EmpNoCount != 0 {
			t.Fatalf("[%s] 无工号映射时 emp_no_count 应为 0，got %d", name, need.EmpNoCount)
		}
		if got := EfficiencyV2StringsFromJSON(need.ContributorEmpNos); len(got) != 0 {
			t.Fatalf("[%s] 无工号映射时 contributor_emp_nos 应为空，got %v", name, got)
		}
	}
}
