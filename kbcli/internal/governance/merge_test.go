package governance

import "testing"

func TestIsMergeComment(t *testing.T) {
	cases := []struct {
		desc    string
		comment string
		want    bool
	}{
		// 行首 merge 命中（内网实测 comment 以 merge 开头 376 个，avg 314 行 max 20304 行）
		{"Merge branch", "Merge branch 'feature/x' into main", true},
		{"Merge pull request", "Merge pull request #123 from a/b", true},
		{"Merge remote-tracking", "Merge remote-tracking branch 'origin/main'", true},
		{"小写 merge 冒号", "merge: 同步主干代码", true},
		{"全大写", "MERGE BRANCH DEV", true},
		{"仅 merge 一词", "merge", true},
		{"merge后跟中文也算行首单词", "Merge分支dev到main", true},
		// 边界：merged 不命中 \b（merge 后紧跟字母）
		{"merged 不命中", "merged latest upstream changes", false},
		// 边界："merge" 出现在句中不命中行首规则
		{"句中 merge 不命中", "fix: merge two config loaders into one", false},
		{"中文开头句中含 merge", "修复 merge 冲突导致的编译错误", false},
		// 中文 comment 不误伤
		{"纯中文 comment", "合并主干代码并修复冲突", false},
		{"常规 feat comment", "feat(governance): 身份治理子 pass", false},
		{"空 comment", "", false},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			if got := IsMergeComment(c.comment); got != c.want {
				t.Fatalf("IsMergeComment(%q) = %v，期望 %v", c.comment, got, c.want)
			}
		})
	}
}
