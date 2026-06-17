package efficiencyv2

import (
	"reflect"
	"testing"
)

// 实测语义：工号 25163 一个人用 3 个 user_id（碎片）→ 三者都映射到 25163、无共享标记；
// 共享账号一个 user_id 挂 2 工号 → 标共享、不进 EmpByUID。
func TestBuildEfficiencyV2UserEmpMapFromRows(t *testing.T) {
	rows := []efficiencyV2EmpCommitRow{
		// add-kb-tyh-B 实测：3 user_id 同工号 25163（碎片）
		{UserId: "16112b61", EmpNo: "25163"},
		{UserId: "70b3222c", EmpNo: "25163"},
		{UserId: "ba4ec4d1", EmpNo: "25163"},
		// 普通单人单账号
		{UserId: "u-solo", EmpNo: "90331"},
		// 共享账号：一个 user_id 两个工号（BUG018 形态）
		{UserId: "u-shared", EmpNo: "11910"},
		{UserId: "u-shared", EmpNo: "33844"},
		// 脏行：空 user_id / 空 emp_no 应忽略
		{UserId: "", EmpNo: "99999"},
		{UserId: "u-x", EmpNo: ""},
		// 重复行（DISTINCT 兜底）
		{UserId: "u-solo", EmpNo: "90331"},
	}
	m := BuildEfficiencyV2UserEmpMapFromRows(rows)

	for _, uid := range []string{"16112b61", "70b3222c", "ba4ec4d1"} {
		if got := m.EmpForUID(uid); got != "25163" {
			t.Fatalf("碎片 user_id %s 应映射工号 25163，得 %q", uid, got)
		}
	}
	if got := m.EmpForUID("u-solo"); got != "90331" {
		t.Fatalf("单账号应映射 90331，得 %q", got)
	}
	if !m.SharedAccountUIDs["u-shared"] {
		t.Fatalf("u-shared 应标共享账号")
	}
	if got := m.EmpForUID("u-shared"); got != "" {
		t.Fatalf("共享账号 EmpForUID 应空，得 %q", got)
	}
	if got := m.EmpForUID("u-orphan-unknown"); got != "" {
		t.Fatalf("未知 user_id 应空，得 %q", got)
	}
	if _, ok := m.EmpByUID["u-x"]; ok {
		t.Fatalf("空 emp_no 行不应进 map")
	}
}

// DistinctEmpNos：碎片去重（3 user_id → 1 工号）、丢弃共享账号与 orphan、排序确定。
func TestEfficiencyV2DistinctEmpNos(t *testing.T) {
	m := BuildEfficiencyV2UserEmpMapFromRows([]efficiencyV2EmpCommitRow{
		{UserId: "a1", EmpNo: "25163"},
		{UserId: "a2", EmpNo: "25163"},
		{UserId: "a3", EmpNo: "25163"},
		{UserId: "b1", EmpNo: "90331"},
		{UserId: "sh", EmpNo: "11910"},
		{UserId: "sh", EmpNo: "33844"},
	})
	// add-kb-tyh-B 实测：5 个 user_id（含 2 个 orphan session-only）→ 应只剩 1 个工号
	got := m.DistinctEmpNos([]string{"a1", "a2", "a3", "orphan1", "orphan2"})
	if !reflect.DeepEqual(got, []string{"25163"}) {
		t.Fatalf("碎片 5 user_id 应去重为 [25163]，得 %v", got)
	}
	// 含共享账号：sh 不计入
	got = m.DistinctEmpNos([]string{"a1", "b1", "sh"})
	if !reflect.DeepEqual(got, []string{"25163", "90331"}) {
		t.Fatalf("应去重为 [25163 90331]（排序、丢共享），得 %v", got)
	}
	// 全 orphan → 空
	if got := m.DistinctEmpNos([]string{"x", "y"}); len(got) != 0 {
		t.Fatalf("全 orphan 应空，得 %v", got)
	}
	// nil receiver 安全
	var nilMap *EfficiencyV2UserEmpMap
	if got := nilMap.EmpForUID("a1"); got != "" {
		t.Fatalf("nil map EmpForUID 应空")
	}
}

// EmpForCommit：按 git_user_email 前缀取工号 + dept_user 在册校验。共享账号的 committer
// 工号(33844 无干净 user_id 指向)只要在册仍认；不在册前缀 / 空邮箱 / nil map 一律空。
func TestEfficiencyV2EmpForCommit(t *testing.T) {
	m := BuildEfficiencyV2UserEmpMapFromRows([]efficiencyV2EmpCommitRow{
		{UserId: "a1", EmpNo: "25163"},
		{UserId: "sh", EmpNo: "11910"},
		{UserId: "sh", EmpNo: "33844"}, // 共享账号下的在册工号，无干净 user_id
	})
	cases := []struct {
		email string
		want  string
	}{
		{"25163@sangfor.com", "25163"},     // 在册
		{"33844@sundray.com", "33844"},     // 在册（即便其 user_id 是共享账号）
		{"99999@sangfor.com", ""},          // 前缀不在册
		{"noreply@github.com", ""},         // 外部贡献者 → 不在册
		{"", ""},                           // 空邮箱
		{"  25163@sangfor.com  ", "25163"}, // 两侧空白
	}
	for _, c := range cases {
		if got := m.EmpForCommit(c.email); got != c.want {
			t.Fatalf("EmpForCommit(%q) = %q, want %q", c.email, got, c.want)
		}
	}
	var nilMap *EfficiencyV2UserEmpMap
	if got := nilMap.EmpForCommit("25163@sangfor.com"); got != "" {
		t.Fatalf("nil map EmpForCommit 应空，得 %q", got)
	}
}
