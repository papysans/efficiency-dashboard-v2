package main

// 纯逻辑单测（不依赖 dept-sync/DB）：根筛选（单根/森林，需求1）+ 子树守恒 rollup（overview/forest-ranking 共用 computeDeptSubtreeAccums）。

import (
	"testing"

	"kanban/backend/internal/appconfig"
)

// nestedForest 构造 dept-sync /department/tree 形状的嵌套森林：R(根)→{A→A1, B}，外加独立顶层根 X。
func nestedForest() []DeptTreeNode {
	return []DeptTreeNode{
		{DeptId: "R", DeptName: "RootCo", ParentDeptId: "", OrderNum: 0, Children: []DeptTreeNode{
			{DeptId: "A", DeptName: "DeptA", ParentDeptId: "R", OrderNum: 1, Children: []DeptTreeNode{
				{DeptId: "A1", DeptName: "DeptA1", ParentDeptId: "A", OrderNum: 1},
			}},
			{DeptId: "B", DeptName: "DeptB", ParentDeptId: "R", OrderNum: 2},
		}},
		{DeptId: "X", DeptName: "OtherTop", ParentDeptId: "", OrderNum: 5},
	}
}

func TestRebuildSingleRootTree_EmptyConfig_ReturnsForest(t *testing.T) {
	appconfig.Cfg.DeptSync = appconfig.DeptSyncConfig{} // 留空 → 全森林

	got := rebuildSingleRootTree(nestedForest())
	if len(got) != 2 {
		t.Fatalf("留空应返回全森林(2 根 R+X)，实际 %d 根", len(got))
	}
	if got[0].DeptId != "R" || got[1].DeptId != "X" {
		t.Fatalf("森林根顺序应为 [R,X]，实际 [%s,%s]", got[0].DeptId, got[1].DeptId)
	}
	if len(got[0].Children) != 2 || got[0].ChildDeptCount != 2 {
		t.Fatalf("R 应有 2 子部门(A,B)，实际 children=%d count=%d", len(got[0].Children), got[0].ChildDeptCount)
	}
}

func TestRebuildSingleRootTree_RootDeptId_SingleSubtree(t *testing.T) {
	appconfig.Cfg.DeptSync = appconfig.DeptSyncConfig{RootDeptId: "A"}

	got := rebuildSingleRootTree(nestedForest())
	if len(got) != 1 || got[0].DeptId != "A" {
		t.Fatalf("配置 root_dept_id=A 应返回单根子树 [A]，实际 len=%d", len(got))
	}
	if len(got[0].Children) != 1 || got[0].Children[0].DeptId != "A1" {
		t.Fatalf("A 子树应含 A1，实际 children=%v", got[0].Children)
	}
}

func TestRebuildSingleRootTree_RootDeptName_SingleSubtree(t *testing.T) {
	appconfig.Cfg.DeptSync = appconfig.DeptSyncConfig{RootDeptName: "RootCo"}

	got := rebuildSingleRootTree(nestedForest())
	if len(got) != 1 || got[0].DeptId != "R" {
		t.Fatalf("配置 root_dept_name=RootCo 应返回单根子树 [R]，实际 len=%d", len(got))
	}
}

func approxV2(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 1e-9
}

func TestComputeDeptSubtreeAccums_Conservation(t *testing.T) {
	appconfig.Cfg.DeptSync = appconfig.DeptSyncConfig{} // 森林（含 R 全树）
	tree := rebuildSingleRootTree(nestedForest())

	// 成员（dept-sync 花名册）：A1=u1, A=u2, B=u3（universal_id 锚点匹配 rowByUser）。X 无成员。
	members := []deptSyncMemberNode{
		{UserId: "e1", UniversalId: "u1", DeptId: "A1"},
		{UserId: "e2", UniversalId: "u2", DeptId: "A"},
		{UserId: "e3", UniversalId: "u3", DeptId: "B"},
	}
	rowByUser := map[string]UserV2Row{
		"u1": {UserId: "u1", BaselineCalendarMin: 100, ActualCalendarMin: 50},
		"u2": {UserId: "u2", BaselineCalendarMin: 200, ActualCalendarMin: 100},
		"u3": {UserId: "u3", BaselineCalendarMin: 60, ActualCalendarMin: 30},
	}

	accs := computeDeptSubtreeAccums(tree, members, rowByUser)

	// A 子树 = u2(直属) + A1(u1) = baseline 300 / actual 150；成员 2 人。
	a := accs["A"]
	if a == nil || !approxV2(a.summary.BaselineCalendarMin, 300) || !approxV2(a.summary.ActualCalendarMin, 150) {
		t.Fatalf("A 子树守恒和错误：%+v", a)
	}
	if a.summary.MemberCount != 2 || a.summary.KanbanMemberCount != 2 {
		t.Fatalf("A 子树成员计数错误：member=%d kanban=%d", a.summary.MemberCount, a.summary.KanbanMemberCount)
	}
	// R 子树 = A + B = baseline 360 / actual 180，提效比 (360-180)/180 = 1.0，成员 3 人。
	r := accs["R"]
	if r == nil || !approxV2(r.summary.BaselineCalendarMin, 360) || !approxV2(r.summary.ActualCalendarMin, 180) {
		t.Fatalf("R 子树守恒和错误：%+v", r)
	}
	if r.summary.MemberCount != 3 {
		t.Fatalf("R 子树成员数应为 3，实际 %d", r.summary.MemberCount)
	}
	if r.summary.CalendarRatio == nil || !approxV2(*r.summary.CalendarRatio, 1.0) {
		t.Fatalf("R 子树日历提效比应≈1.0，实际 %v", r.summary.CalendarRatio)
	}
}

// 含硅量的子树 rollup 必须按「Σ匹配行 / Σ总行」守恒重算，不能对成员比值求平均。
// 场景刻意做成"小仓全中 + 大仓几乎没中"，两种算法差 20 倍以上，改坏必炸。
func TestComputeDeptSubtreeAccums_SilicaWeightedConservation(t *testing.T) {
	appconfig.Cfg.DeptSync = appconfig.DeptSyncConfig{}
	tree := rebuildSingleRootTree(nestedForest())

	members := []deptSyncMemberNode{
		{UserId: "e1", UniversalId: "u1", DeptId: "A1"},
		{UserId: "e2", UniversalId: "u2", DeptId: "A"},
		{UserId: "e3", UniversalId: "u3", DeptId: "B"},
	}
	// u1: 3 行全中(1.0)；u2: 300 行几乎没中(0.01)；u3: 无可计入 commit。
	rowByUser := map[string]UserV2Row{
		"u1": {UserId: "u1", silicaWeighted: 3 * 1.0, silicaWeight: 3},
		"u2": {UserId: "u2", silicaWeighted: 300 * 0.01, silicaWeight: 300},
		"u3": {UserId: "u3"},
	}

	accs := computeDeptSubtreeAccums(tree, members, rowByUser)

	// A 子树 = u2(直属) + A1(u1) → (3.0 + 3.0) / 303
	a := accs["A"]
	if a == nil || a.summary.Silica == nil {
		t.Fatalf("A 子树含硅量不应为 nil：%+v", a)
	}
	if want := 6.0 / 303.0; !approxV2(*a.summary.Silica, want) {
		t.Fatalf("A 子树含硅量 got %v, want %v（疑似退回比值平均）", *a.summary.Silica, want)
	}
	// 守住与「比值平均」(1.0+0.01)/2=0.505 的距离
	if *a.summary.Silica > 0.1 {
		t.Fatalf("A 子树含硅量 %v 过高，加权口径可能被改成平均", *a.summary.Silica)
	}

	// R 子树 = A + B，B 的 u3 无权重不改变分母
	r := accs["R"]
	if r == nil || r.summary.Silica == nil {
		t.Fatalf("R 子树含硅量不应为 nil：%+v", r)
	}
	if want := 6.0 / 303.0; !approxV2(*r.summary.Silica, want) {
		t.Fatalf("R 子树含硅量 got %v, want %v", *r.summary.Silica, want)
	}

	// X 无成员 → nil（无数据），不能是 0（会被读成"AI 一行没写"）
	if x := accs["X"]; x == nil || x.summary.Silica != nil {
		t.Fatalf("X 无成员时含硅量应为 nil，got %+v", x)
	}
}
