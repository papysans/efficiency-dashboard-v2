package main

import "testing"

// TestBuildProjectNeedScopeClause_CanonicalizesRepoAddr 验证候选池 SQL 的 repo_addr 参数
// 被规范化为 needs.repo_addr 的写法（原始 scp/端口地址 → canon），否则精确等值恒空（候选池 bug 根因）。
func TestBuildProjectNeedScopeClause_CanonicalizesRepoAddr(t *testing.T) {
	cases := []struct {
		name     string
		rawAddr  string
		wantArg0 string
	}{
		{"scp 多级路径", "git@cs.devops.sangfor.org:SDS/SRC/EDS/phxrep.git", "cs.devops.sangfor.org/sds/src/eds/phxrep"},
		{"带端口冒号保留", "git@cs.devops.sangfor.org:19670/iap.git", "cs.devops.sangfor.org:19670/iap"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clause, args := buildProjectNeedScopeClause([]projectNeedScope{{RepoAddr: tc.rawAddr}}, false)
			if clause != "(repo_addr = ?)" {
				t.Fatalf("clause = %q, want %q", clause, "(repo_addr = ?)")
			}
			if len(args) != 1 {
				t.Fatalf("len(args) = %d, want 1", len(args))
			}
			if got, _ := args[0].(string); got != tc.wantArg0 {
				t.Fatalf("args[0] = %q, want %q (规范化未生效)", got, tc.wantArg0)
			}
		})
	}
}

// TestIsNeedExcludedByScopes_CanonicalMatch 验证勾选判定按规范化地址匹配：
// 配置存原始地址、入参是 needs 行的规范化地址，仍能匹配上对应 scope 并让黑名单生效。
func TestIsNeedExcludedByScopes_CanonicalMatch(t *testing.T) {
	rawAddr := "git@cs.devops.sangfor.org:SDS/SRC/EDS/phxrep.git"
	canonAddr := "cs.devops.sangfor.org/sds/src/eds/phxrep" // needs 行实际写法
	branch := "EDS5.3.2-dev"

	// 无名单：规范化匹配成功 → 默认纳入（not excluded）。
	if excluded := isNeedExcludedByScopes([]projectNeedScope{{RepoAddr: rawAddr}}, canonAddr, branch, "n-1"); excluded {
		t.Fatalf("无名单时应纳入(false)，got excluded=true（规范化匹配失败）")
	}

	// 黑名单含该 need：只有规范化匹配成功才会判定为排除(true)；若 canon 失效则 matched=false→保守纳入(false)。
	scopesExcl := []projectNeedScope{{RepoAddr: rawAddr, ExcludeNeeds: []string{"n-1"}}}
	if excluded := isNeedExcludedByScopes(scopesExcl, canonAddr, branch, "n-1"); !excluded {
		t.Fatalf("黑名单命中且地址规范化匹配时应排除(true)，got false（规范化匹配失败）")
	}
	// 黑名单不含的其它 need：匹配成功但未被排除 → 纳入(false)。
	if excluded := isNeedExcludedByScopes(scopesExcl, canonAddr, branch, "n-2"); excluded {
		t.Fatalf("黑名单未命中应纳入(false)，got true")
	}
}
