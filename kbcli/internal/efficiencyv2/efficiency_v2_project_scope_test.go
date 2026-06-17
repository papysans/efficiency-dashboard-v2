package efficiencyv2

import (
	"strings"
	"testing"
)

// 项目级 need 解析命中：配置侧原始 repo 地址须被 canon 规范化后才能匹配 needs.repo_addr。
func TestBuildEfficiencyV2ProjectNeedScopeClause_CanonicalizesRepoAddr(t *testing.T) {
	scopes := []EfficiencyV2ProjectNeedScope{
		{RepoAddr: "git@example.com:Acme/Billing.git", RepoBranch: "feature/x"},
	}
	clause, args := buildEfficiencyV2ProjectNeedScopeClause(scopes)
	if clause == "" {
		t.Fatalf("clause should not be empty for a valid repo scope")
	}
	if !strings.Contains(clause, "repo_addr = ?") || !strings.Contains(clause, "repo_branch = ?") {
		t.Fatalf("clause missing repo/branch predicates: %q", clause)
	}
	// 配置原始地址 git@example.com:Acme/Billing.git → canon = example.com/acme/billing
	if len(args) < 1 || args[0] != "example.com/acme/billing" {
		t.Fatalf("repo_addr arg should be canonicalized, got %#v", args)
	}
	if len(args) != 2 || args[1] != "feature/x" {
		t.Fatalf("args = %#v, want [canonAddr, branch]", args)
	}
}

func TestBuildEfficiencyV2ProjectNeedScopeClause_WindowAndIncludeOnly(t *testing.T) {
	scopes := []EfficiencyV2ProjectNeedScope{
		{
			RepoAddr:         "https://gitlab/acme/web.git",
			StartTime:        "2026-06-01",
			EndTime:          "2026-06-30",
			IncludeOnlyNeeds: []string{"need-1", "need-2"},
		},
	}
	clause, args := buildEfficiencyV2ProjectNeedScopeClause(scopes)
	if !strings.Contains(clause, "dev_end_ts >= ?") || !strings.Contains(clause, "dev_end_ts <= ?") {
		t.Fatalf("clause should carry the time window: %q", clause)
	}
	if !strings.Contains(clause, "need_id IN ?") {
		t.Fatalf("include-only should add need_id IN ?: %q", clause)
	}
	// args: canonAddr, start, end, includeOnlySlice
	if args[0] != "gitlab/acme/web" {
		t.Fatalf("canon addr = %v, want gitlab/acme/web", args[0])
	}
}

func TestBuildEfficiencyV2ProjectNeedScopeClause_ExcludeNeeds(t *testing.T) {
	scopes := []EfficiencyV2ProjectNeedScope{
		{RepoAddr: "git@h:o/r.git", ExcludeNeeds: []string{"bad-1"}},
	}
	clause, _ := buildEfficiencyV2ProjectNeedScopeClause(scopes)
	if !strings.Contains(clause, "need_id NOT IN ?") {
		t.Fatalf("exclude needs should add NOT IN: %q", clause)
	}
}

func TestBuildEfficiencyV2ProjectNeedScopeClause_MultipleScopesOredAndEmptySkipped(t *testing.T) {
	scopes := []EfficiencyV2ProjectNeedScope{
		{RepoAddr: "git@h:o/a.git"},
		{RepoAddr: "  "}, // 空地址跳过
		{RepoAddr: "git@h:o/b.git", RepoBranch: "dev"},
	}
	clause, _ := buildEfficiencyV2ProjectNeedScopeClause(scopes)
	if strings.Count(clause, " OR ") != 1 {
		t.Fatalf("two valid scopes should be OR-joined exactly once: %q", clause)
	}
}

func TestBuildEfficiencyV2ProjectNeedScopeClause_EmptyWhenNoValidRepo(t *testing.T) {
	clause, args := buildEfficiencyV2ProjectNeedScopeClause([]EfficiencyV2ProjectNeedScope{{RepoAddr: ""}})
	if clause != "" || args != nil {
		t.Fatalf("empty/blank repo scopes should yield empty clause, got %q / %#v", clause, args)
	}
}
