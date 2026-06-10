package governance

import (
	"testing"
	"time"

	"kanban/core/models"
)

func mustCompileDownweightRules(t *testing.T, rules []DownweightRule) []compiledDownweightRule {
	t.Helper()
	compiled, err := compileDownweightRules(rules)
	if err != nil {
		t.Fatalf("compile downweight rules: %v", err)
	}
	return compiled
}

func assertEffective(t *testing.T, got *int64, want int64) {
	t.Helper()
	if got == nil {
		t.Fatalf("effective = nil, want %d", want)
	}
	if *got != want {
		t.Fatalf("effective = %d, want %d", *got, want)
	}
}

func TestComputeCommitEffectiveDiffLines_SoftcapBoundary(t *testing.T) {
	// 等于 softcap → 不命中（规则是 diff_lines>softcap），用原值
	if got := computeCommitEffectiveDiffLines(3000, "feat: x", 3000, nil); got != nil {
		t.Fatalf("diff==softcap should not hit, got %d", *got)
	}
	// 超过 softcap → 截到 softcap
	assertEffective(t, computeCommitEffectiveDiffLines(3001, "feat: x", 3000, nil), 3000)
	assertEffective(t, computeCommitEffectiveDiffLines(112938, "feat: x", 3000, nil), 3000)
	// softcap=0 关闭 → 巨型提交也不命中
	if got := computeCommitEffectiveDiffLines(112938, "feat: x", 0, nil); got != nil {
		t.Fatalf("softcap=0 should be off, got %d", *got)
	}
}

func TestComputeCommitEffectiveDiffLines_DownweightStacksOnSoftcap(t *testing.T) {
	rules := mustCompileDownweightRules(t, []DownweightRule{
		{Pattern: "(?i)merge|sync|format|style|scaffold|init", Factor: 0.2},
	})
	// softcap+降权叠加：5000 → 截到 3000 → ×0.2 = 600
	assertEffective(t, computeCommitEffectiveDiffLines(5000, "Merge branch 'main' into dev", 3000, rules), 600)
	// 仅降权（不超 softcap）：1000 × 0.2 = 200
	assertEffective(t, computeCommitEffectiveDiffLines(1000, "format: gofmt all", 3000, rules), 200)
	// round 取整：333 × 0.2 = 66.6 → 67
	assertEffective(t, computeCommitEffectiveDiffLines(333, "sync upstream", 3000, rules), 67)
	// comment 不命中且不超 softcap → nil（用原值）
	if got := computeCommitEffectiveDiffLines(1000, "feat: real work", 3000, rules); got != nil {
		t.Fatalf("no rule hit should return nil, got %d", *got)
	}
}

func TestComputeCommitEffectiveDiffLines_MultiPatternTakesMinFactor(t *testing.T) {
	rules := mustCompileDownweightRules(t, []DownweightRule{
		{Pattern: "(?i)merge", Factor: 0.5},
		{Pattern: "(?i)sync", Factor: 0.2},
		{Pattern: "(?i)format", Factor: 0.8},
	})
	// 同时命中 merge(0.5)+sync(0.2) → 取最小 0.2：1000→200
	assertEffective(t, computeCommitEffectiveDiffLines(1000, "Merge sync upstream", 3000, rules), 200)
	// 只命中 format(0.8)：1000→800
	assertEffective(t, computeCommitEffectiveDiffLines(1000, "format code", 3000, rules), 800)
}

func TestComputeCommitRuleResults_ReplayKeepsEarliest(t *testing.T) {
	base := time.Date(2026, 5, 18, 9, 0, 0, 0, time.UTC)
	commits := []models.Commit{
		{CommitId: "c-late", UserId: "u1", Comment: " rebase replay done ", DiffLines: 500, CommitTime: base.Add(2 * time.Hour)},
		{CommitId: "c-early", UserId: "u1", Comment: "rebase replay done", DiffLines: 500, CommitTime: base},
		{CommitId: "c-mid", UserId: "u1", Comment: "rebase replay done", DiffLines: 500, CommitTime: base.Add(time.Hour)},
	}
	results := computeCommitRuleResults(commits, CommitRulesConfig{ReplayDedup: true}, nil)

	// 组键用 trim(comment)，三条同组；按 commit_time 升序保最早
	if r := results["c-early"]; r.replayOf != "" || r.effectiveDiffLines != nil {
		t.Fatalf("earliest should be kept untouched, got replay_of=%q effective=%v", r.replayOf, r.effectiveDiffLines)
	}
	for _, id := range []string{"c-mid", "c-late"} {
		r := results[id]
		if r.replayOf != "c-early" {
			t.Fatalf("%s replay_of = %q, want c-early", id, r.replayOf)
		}
		assertEffective(t, r.effectiveDiffLines, 0)
	}
}

func TestComputeCommitRuleResults_ReplayThresholdsAndScope(t *testing.T) {
	base := time.Date(2026, 5, 18, 9, 0, 0, 0, time.UTC)
	commits := []models.Commit{
		// diff_lines=100 不满足 >100，不参与去重
		{CommitId: "small-1", UserId: "u1", Comment: "duplicate comment", DiffLines: 100, CommitTime: base},
		{CommitId: "small-2", UserId: "u1", Comment: "duplicate comment", DiffLines: 100, CommitTime: base.Add(time.Minute)},
		// trim(comment) 长度 5 不满足 >5，不参与去重
		{CommitId: "short-1", UserId: "u1", Comment: " abcde ", DiffLines: 500, CommitTime: base},
		{CommitId: "short-2", UserId: "u1", Comment: "abcde", DiffLines: 500, CommitTime: base.Add(time.Minute)},
		// 不同 user 不同组，不互相判重
		{CommitId: "user-a", UserId: "u1", Comment: "rebase replay done", DiffLines: 500, CommitTime: base},
		{CommitId: "user-b", UserId: "u2", Comment: "rebase replay done", DiffLines: 500, CommitTime: base.Add(time.Minute)},
	}
	results := computeCommitRuleResults(commits, CommitRulesConfig{ReplayDedup: true}, nil)
	for id, r := range results {
		if r.replayOf != "" || r.effectiveDiffLines != nil {
			t.Fatalf("%s should not be deduped, got replay_of=%q effective=%v", id, r.replayOf, r.effectiveDiffLines)
		}
	}

	// replay_dedup=false 时同组完全重复也不去重
	dup := []models.Commit{
		{CommitId: "d1", UserId: "u1", Comment: "rebase replay done", DiffLines: 500, CommitTime: base},
		{CommitId: "d2", UserId: "u1", Comment: "rebase replay done", DiffLines: 500, CommitTime: base.Add(time.Minute)},
	}
	results = computeCommitRuleResults(dup, CommitRulesConfig{ReplayDedup: false}, nil)
	if r := results["d2"]; r.replayOf != "" || r.effectiveDiffLines != nil {
		t.Fatalf("replay_dedup=false should not dedup, got replay_of=%q effective=%v", r.replayOf, r.effectiveDiffLines)
	}
}

func TestComputeCommitRuleResults_ReplayOverridesDownweightAndSameSecondTieBreak(t *testing.T) {
	base := time.Date(2026, 5, 18, 9, 0, 0, 0, time.UTC)
	rules := mustCompileDownweightRules(t, []DownweightRule{{Pattern: "(?i)sync", Factor: 0.2}})
	commits := []models.Commit{
		// 同秒重放：commit_time 相同时按 commit_id 兜底，c-a 为"最早"
		{CommitId: "c-b", UserId: "u1", Comment: "sync upstream code", DiffLines: 500, CommitTime: base},
		{CommitId: "c-a", UserId: "u1", Comment: "sync upstream code", DiffLines: 500, CommitTime: base},
	}
	results := computeCommitRuleResults(commits, CommitRulesConfig{DiffLinesSoftcap: 3000, ReplayDedup: true}, rules)

	// 保留者吃降权：500 × 0.2 = 100
	keeper := results["c-a"]
	if keeper.replayOf != "" {
		t.Fatalf("keeper replay_of = %q, want empty", keeper.replayOf)
	}
	assertEffective(t, keeper.effectiveDiffLines, 100)
	// 重复者 effective 被重放去重覆盖为 0
	dup := results["c-b"]
	if dup.replayOf != "c-a" {
		t.Fatalf("dup replay_of = %q, want c-a", dup.replayOf)
	}
	assertEffective(t, dup.effectiveDiffLines, 0)
}
