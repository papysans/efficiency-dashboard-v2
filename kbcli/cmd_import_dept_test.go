package main

import (
	"kanban/core/models"
	"testing"
)

func TestSplitDeptPath(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"标准路径去前导空段", "/深信服科技股份有限公司/研发体系/Costrict研发部/开发组",
			[]string{"深信服科技股份有限公司", "研发体系", "Costrict研发部", "开发组"}},
		{"单段", "/深信服科技股份有限公司", []string{"深信服科技股份有限公司"}},
		{"空串", "", nil},
		{"仅斜杠", "///", nil},
		{"无前导斜杠", "A/B", []string{"A", "B"}},
		{"段内空格被裁剪", "/ A / B ", []string{"A", "B"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := splitDeptPath(c.in)
			if len(got) != len(c.want) {
				t.Fatalf("段数不符: got %v want %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("第%d段不符: got %q want %q", i, got[i], c.want[i])
				}
			}
		})
	}
}

func TestAssignOrgFields(t *testing.T) {
	t.Run("四级写入org1..org4", func(t *testing.T) {
		var uo models.UserOrg
		assignOrgFields(&uo, []string{"深信服科技股份有限公司", "研发体系", "Costrict研发部", "开发组"})
		if uo.Org1 != "深信服科技股份有限公司" || uo.Org2 != "研发体系" || uo.Org3 != "Costrict研发部" || uo.Org4 != "开发组" {
			t.Fatalf("org1..4 写入错误: %+v", uo)
		}
		if uo.Org5 != "" {
			t.Fatalf("org5 应为空, got %q", uo.Org5)
		}
	})
	t.Run("超过9级安全截断", func(t *testing.T) {
		var uo models.UserOrg
		segs := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"}
		assignOrgFields(&uo, segs) // 不应 panic
		if uo.Org9 != "i" {
			t.Fatalf("org9 应为第9段 'i', got %q", uo.Org9)
		}
	})
	t.Run("空段列表不写入", func(t *testing.T) {
		var uo models.UserOrg
		assignOrgFields(&uo, nil)
		if uo.Org1 != "" {
			t.Fatalf("org1 应为空, got %q", uo.Org1)
		}
	})
}

func TestDeriveRealName(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"林凯90331", "林凯"},
		{"邓彬84569", "邓彬"},
		{"IronRookieCoder", "IronRookieCoder"},
		{"", ""},
		{"张三", "张三"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			if got := deriveRealName(c.in); got != c.want {
				t.Fatalf("deriveRealName(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestFlattenDeptTree(t *testing.T) {
	tree := []deptSyncNode{
		{DeptId: "49", DeptName: "深信服科技股份有限公司", DeptLevel: 1, Children: []deptSyncNode{
			{DeptId: "1416", DeptName: "研发体系", ParentDeptId: "49", DeptLevel: 2, Children: []deptSyncNode{
				{DeptId: "6560", DeptName: "Costrict研发部", ParentDeptId: "1416", DeptLevel: 3, Children: []deptSyncNode{
					{DeptId: "6571", DeptName: "开发组", ParentDeptId: "6560", DeptLevel: 4},
					{DeptId: "6572", DeptName: "客户成功组", ParentDeptId: "6560", DeptLevel: 4},
				}},
			}},
		}},
	}
	var out []models.Dept
	flattenDeptTree(tree, &out)
	if len(out) != 5 {
		t.Fatalf("拍平后应有 5 个部门, got %d", len(out))
	}
	byID := make(map[string]models.Dept, len(out))
	for _, d := range out {
		byID[d.DeptId] = d
	}
	if byID["6571"].DeptName != "开发组" || byID["6571"].ParentDeptId != "6560" || byID["6571"].DeptLevel != 4 {
		t.Fatalf("叶子节点 6571 字段错误: %+v", byID["6571"])
	}
	if byID["49"].DeptName != "深信服科技股份有限公司" {
		t.Fatalf("根节点 49 错误: %+v", byID["49"])
	}
}
