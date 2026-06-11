package governance

import "testing"

func TestIsMergeComment(t *testing.T) {
	cases := []struct {
		desc    string
		comment string
		want    bool
	}{
		// git 规范 merge 文案命中（内网实测 comment 以 merge 开头 376 个，avg 314 行 max 20304 行）
		{"Merge branch", "Merge branch 'feature/x' into main", true},
		{"Merge pull request", "Merge pull request #123 from a/b", true},
		{"Merge remote-tracking", "Merge remote-tracking branch 'origin/main'", true},
		{"Merge tag", "Merge tag 'v1.2.0' into release", true},
		{"Merge commit", "Merge commit 'abc123' into dev", true},
		{"全大写", "MERGE BRANCH DEV", true},
		// 兜底只认 git 规范文案：人写的行首 merge 误杀清零 LOC 代价大于漏认（真 merge 主路径是 parent_ids）
		{"小写 merge 冒号不命中", "merge: 同步主干代码", false},
		{"仅 merge 一词不命中", "merge", false},
		{"merge后跟中文不命中", "Merge分支dev到main", false},
		{"祈使句 merge 不命中", "merge search results with local cache", false},
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
