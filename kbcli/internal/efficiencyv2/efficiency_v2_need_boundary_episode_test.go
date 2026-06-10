package efficiencyv2

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"kanban/kbcli/internal/estimator"

	"kanban/core/models"
)

// 默认 idle_threshold_days = 3：桶内间隙 > 3 天即切新段。

func TestEfficiencyV2EpisodeSplitGapCreatesSuffixedNeeds(t *testing.T) {
	base := time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC)
	repo := "git@example.com/acme/app.git"
	branch := "feature/epic-x"
	metrics := []models.SessionStageMetric{
		efficiencyV2NeedTestMetric("s-ep1-a", "u-amy", repo, branch, base, base.Add(time.Hour)),
		efficiencyV2NeedTestMetric("s-ep1-b", "u-amy", repo, branch, base.Add(24*time.Hour), base.Add(25*time.Hour)),
		// 间隙 = 10d-25h > 3d → 切新段
		efficiencyV2NeedTestMetric("s-ep2-a", "u-amy", repo, branch, base.Add(10*24*time.Hour), base.Add(10*24*time.Hour+time.Hour)),
	}

	needs := ResolveEfficiencyV2Needs(metrics, nil, nil, EfficiencyV2Config{})
	if len(needs) != 2 {
		t.Fatalf("need count: want 2 episodes, got %d: %#v", len(needs), needs)
	}
	wantIDs := []string{
		"branch:" + repo + ":" + branch + "@2026-04-01",
		"branch:" + repo + ":" + branch + "@2026-04-11",
	}
	gotIDs := []string{needs[0].NeedId, needs[1].NeedId}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("episode need ids: want %v, got %v", wantIDs, gotIDs)
	}
	for i, need := range needs {
		if need.BoundarySource != efficiencyV2BoundaryBranch {
			t.Fatalf("episode %d boundary source: want branch, got %s", i, need.BoundarySource)
		}
		if !strings.Contains(string(need.BoundaryEvidence), wantIDs[i]) {
			t.Fatalf("episode %d evidence should keep full key %q, got %s", i, wantIDs[i], need.BoundaryEvidence)
		}
	}
	// 各段 session 不串段
	if !strings.Contains(string(needs[0].SessionIds), "s-ep1-a") || strings.Contains(string(needs[0].SessionIds), "s-ep2-a") {
		t.Fatalf("episode 0 sessions wrong: %s", needs[0].SessionIds)
	}
	if !strings.Contains(string(needs[1].SessionIds), "s-ep2-a") || strings.Contains(string(needs[1].SessionIds), "s-ep1-a") {
		t.Fatalf("episode 1 sessions wrong: %s", needs[1].SessionIds)
	}
}

func TestEfficiencyV2EpisodeSingleSegmentKeepsLegacyNeedID(t *testing.T) {
	base := time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC)
	repo := "git@example.com/acme/app.git"
	branch := "feature/short"
	metrics := []models.SessionStageMetric{
		efficiencyV2NeedTestMetric("s-one-a", "u-amy", repo, branch, base, base.Add(time.Hour)),
		// 间隙 2 天 < 3 天阈值，不切
		efficiencyV2NeedTestMetric("s-one-b", "u-amy", repo, branch, base.Add(2*24*time.Hour), base.Add(2*24*time.Hour+time.Hour)),
	}

	needs := ResolveEfficiencyV2Needs(metrics, nil, nil, EfficiencyV2Config{})
	if len(needs) != 1 {
		t.Fatalf("need count: want 1, got %d", len(needs))
	}
	if needs[0].NeedId != "branch:"+repo+":"+branch {
		t.Fatalf("single-segment bucket must keep legacy need_id without suffix, got %q", needs[0].NeedId)
	}
}

func TestEfficiencyV2SplitBucketEpisodesZeroStartJoinsFirstSegment(t *testing.T) {
	base := time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC)
	bucket := efficiencyV2NeedBucket{
		source:     efficiencyV2BoundaryBranch,
		confidence: efficiencyV2ConfidenceHigh,
		key:        "branch:repo:feature/x",
		candidates: []efficiencyV2BoundaryCandidate{
			{sessionID: "s-undated"},
			{sessionID: "s-seg1", start: base, end: base.Add(time.Hour)},
			{sessionID: "s-seg2", start: base.Add(10 * 24 * time.Hour), end: base.Add(10*24*time.Hour + time.Hour)},
		},
	}
	episodes := efficiencyV2SplitBucketEpisodes(bucket, 3*24*time.Hour)
	if len(episodes) != 2 {
		t.Fatalf("episode count: want 2, got %d", len(episodes))
	}
	if episodes[0].key != "branch:repo:feature/x@2026-04-01" || episodes[1].key != "branch:repo:feature/x@2026-04-11" {
		t.Fatalf("episode keys wrong: %q / %q", episodes[0].key, episodes[1].key)
	}
	if len(episodes[0].candidates) != 2 || episodes[0].candidates[0].sessionID != "s-undated" {
		t.Fatalf("zero-start candidate should join first segment, got %#v", episodes[0].candidates)
	}
	if len(episodes[1].candidates) != 1 || episodes[1].candidates[0].sessionID != "s-seg2" {
		t.Fatalf("second segment wrong: %#v", episodes[1].candidates)
	}
}

func TestEfficiencyV2SplitBucketEpisodesAllZeroStartReturnsBucketAsIs(t *testing.T) {
	bucket := efficiencyV2NeedBucket{
		key:        "orphan:u-x:undated",
		candidates: []efficiencyV2BoundaryCandidate{{sessionID: "s-a"}, {sessionID: "s-b"}},
	}
	episodes := efficiencyV2SplitBucketEpisodes(bucket, 3*24*time.Hour)
	if len(episodes) != 1 || episodes[0].key != bucket.key || len(episodes[0].candidates) != 2 {
		t.Fatalf("all-zero-start bucket should pass through unchanged, got %#v", episodes)
	}
}

func TestEfficiencyV2ClampNeedKeyPreservesEpisodeSuffix(t *testing.T) {
	longBase := "cluster:u-long:" + strings.Repeat("dir-", 40) // >100 runes
	keyA := longBase + "@2026-04-01"
	keyB := longBase + "@2026-04-11"

	clampedA := efficiencyV2ClampNeedKey(keyA)
	clampedB := efficiencyV2ClampNeedKey(keyB)
	for _, pair := range []struct{ key, clamped, suffix string }{
		{keyA, clampedA, "@2026-04-01"},
		{keyB, clampedB, "@2026-04-11"},
	} {
		if got := len([]rune(pair.clamped)); got > 100 {
			t.Fatalf("clamped key %q length %d > 100", pair.clamped, got)
		}
		if !strings.HasSuffix(pair.clamped, pair.suffix) {
			t.Fatalf("clamped key %q must keep episode suffix %q", pair.clamped, pair.suffix)
		}
	}
	if clampedA == clampedB {
		t.Fatalf("different episode suffixes must not collide after clamp: %q", clampedA)
	}

	// 不同超长主体（前缀相同）不能截成同一个 key
	otherBase := "cluster:u-long:" + strings.Repeat("dir-", 39) + "tail-xx"
	if efficiencyV2ClampNeedKey(otherBase+"@2026-04-01") == clampedA {
		t.Fatal("different long bases must not collide after clamp")
	}

	// 无后缀的超长 key 保持原行为：87 前缀 + "~" + 12 哈希 = 100
	noSuffix := efficiencyV2ClampNeedKey(longBase)
	if got := len([]rune(noSuffix)); got != 100 {
		t.Fatalf("no-suffix clamp length: want 100, got %d (%q)", got, noSuffix)
	}
	// 短 key 原样返回
	if efficiencyV2ClampNeedKey("pr:42@2026-04-01") != "pr:42@2026-04-01" {
		t.Fatal("short key must be returned unchanged")
	}
}

func TestEfficiencyV2IntegrationFlowDowngradesConfidence(t *testing.T) {
	base := time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC)
	repo := "git@example.com/acme/app.git"
	branch := "feature/integration-train"
	// 8 天跨度、3 个贡献者，段内间隙都 < 3 天（不触发 episode 切分）
	metrics := []models.SessionStageMetric{
		efficiencyV2NeedTestMetric("s-if-a", "u-a", repo, branch, base, base.Add(time.Hour)),
		efficiencyV2NeedTestMetric("s-if-b", "u-b", repo, branch, base.Add(3*24*time.Hour), base.Add(3*24*time.Hour+time.Hour)),
		efficiencyV2NeedTestMetric("s-if-c", "u-c", repo, branch, base.Add(6*24*time.Hour), base.Add(6*24*time.Hour+time.Hour)),
	}
	commit := efficiencyV2NeedTestCommit("c-if", "u-a", repo, branch, base.Add(8*24*time.Hour), "merge integration train")

	needs := ResolveEfficiencyV2Needs(metrics, nil, []models.Commit{commit}, EfficiencyV2Config{})
	if len(needs) != 1 {
		t.Fatalf("need count: want 1, got %d", len(needs))
	}
	need := needs[0]
	if need.BoundaryConfidence != efficiencyV2ConfidenceLow {
		t.Fatalf("integration-flow need confidence: want low, got %s", need.BoundaryConfidence)
	}
	if need.CoverageEligible {
		t.Fatal("integration-flow need must not be coverage eligible")
	}
	if !strings.Contains(need.Reason, "integration flow") {
		t.Fatalf("reason should record integration flow downgrade, got %q", need.Reason)
	}
	if !strings.Contains(string(need.BoundaryEvidence), `"integration_flow":true`) {
		t.Fatalf("evidence should flag integration_flow, got %s", need.BoundaryEvidence)
	}

	// 对照：同跨度但只有 2 个贡献者，不降级
	metrics2 := []models.SessionStageMetric{
		efficiencyV2NeedTestMetric("s-2c-a", "u-a", repo, "feature/two-devs", base, base.Add(time.Hour)),
		efficiencyV2NeedTestMetric("s-2c-b", "u-b", repo, "feature/two-devs", base.Add(3*24*time.Hour), base.Add(3*24*time.Hour+time.Hour)),
		efficiencyV2NeedTestMetric("s-2c-c", "u-a", repo, "feature/two-devs", base.Add(6*24*time.Hour), base.Add(6*24*time.Hour+time.Hour)),
	}
	commit2 := efficiencyV2NeedTestCommit("c-2c", "u-a", repo, "feature/two-devs", base.Add(8*24*time.Hour), "merge two devs")
	needs2 := ResolveEfficiencyV2Needs(metrics2, nil, []models.Commit{commit2}, EfficiencyV2Config{})
	if len(needs2) != 1 || needs2[0].BoundaryConfidence != efficiencyV2ConfidenceHigh {
		t.Fatalf("two-contributor need must stay high confidence, got %#v", needs2)
	}

	// 对照：3 个贡献者但跨度 ≤ 7d，不降级
	metrics3 := []models.SessionStageMetric{
		efficiencyV2NeedTestMetric("s-3s-a", "u-a", repo, "feature/short-train", base, base.Add(time.Hour)),
		efficiencyV2NeedTestMetric("s-3s-b", "u-b", repo, "feature/short-train", base.Add(2*24*time.Hour), base.Add(2*24*time.Hour+time.Hour)),
		efficiencyV2NeedTestMetric("s-3s-c", "u-c", repo, "feature/short-train", base.Add(4*24*time.Hour), base.Add(4*24*time.Hour+time.Hour)),
	}
	needs3 := ResolveEfficiencyV2Needs(metrics3, nil, nil, EfficiencyV2Config{})
	if len(needs3) != 1 || needs3[0].BoundaryConfidence != efficiencyV2ConfidenceHigh {
		t.Fatalf("short-span multi-contributor need must stay high confidence, got %#v", needs3)
	}
}

func TestEfficiencyV2UndatedCandidatesGetSeparateBucket(t *testing.T) {
	base := time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC)
	// orphan：无 repo/branch/files。一个有时间，一个完全无时间信号。
	dated := efficiencyV2NeedTestMetric("s-dated", "u-zed", "", "", base, base.Add(time.Hour))
	undated := models.SessionStageMetric{SessionId: "s-undated", UserId: "u-zed"}

	needs := ResolveEfficiencyV2Needs([]models.SessionStageMetric{dated, undated}, nil, nil, EfficiencyV2Config{})
	if len(needs) != 2 {
		t.Fatalf("need count: want 2 (dated + undated buckets), got %d: %#v", len(needs), needs)
	}
	byID := map[string]models.Need{}
	for _, need := range needs {
		byID[need.NeedId] = need
	}
	if _, ok := byID["orphan:u-zed"]; !ok {
		t.Fatalf("missing dated orphan bucket, got %#v", byID)
	}
	un, ok := byID["orphan:u-zed:undated"]
	if !ok {
		t.Fatalf("missing undated orphan bucket, got %#v", byID)
	}
	if un.BoundaryConfidence != efficiencyV2ConfidenceVeryLow {
		t.Fatalf("undated bucket confidence: want very_low, got %s", un.BoundaryConfidence)
	}

	// cluster：零 start + ≥2 files → undated cluster 桶，confidence very_low
	source, confidence, key := efficiencyV2ResolveNaturalBoundary(efficiencyV2BoundaryCandidate{
		userID: "u-zed",
		files:  []string{"pkg/a.go", "pkg/b.go"},
	})
	if source != efficiencyV2BoundaryFileCluster || confidence != efficiencyV2ConfidenceVeryLow || key != "cluster:u-zed:undated:pkg" {
		t.Fatalf("undated cluster boundary: got %s/%s/%s", source, confidence, key)
	}
}

func TestEfficiencyV2StaleIDs(t *testing.T) {
	existing := []string{"a", "b@2026-04-01", "b", "c"}
	current := []string{"b@2026-04-01", "c", "d"}
	got := efficiencyV2StaleIDs(existing, current)
	want := []string{"a", "b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("stale ids: want %v, got %v", want, got)
	}
	if len(efficiencyV2StaleIDs(current, current)) != 0 {
		t.Fatal("identical sets must yield no stale ids (idempotent rerun)")
	}
}

func TestEfficiencyV2ZeroCalendarDeliverableGetsAuditReason(t *testing.T) {
	base := time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC)
	repo := "git@example.com/acme/app.git"
	branch := "feature/single-moment"
	// 单时刻大 commit：merged + high → coverage_eligible=true，但零日历
	commit := efficiencyV2NeedTestCommit("c-zero-cal", "u-amy", repo, branch, base, "big drop")

	needs := ResolveEfficiencyV2Needs(nil, nil, []models.Commit{commit}, EfficiencyV2Config{})
	if len(needs) != 1 {
		t.Fatalf("need count: want 1, got %d", len(needs))
	}
	if !needs[0].CoverageEligible {
		t.Fatalf("precondition: boundary-stage need should be eligible, got %#v", needs[0])
	}
	updated := AggregateEfficiencyV2NeedActuals(needs, nil, []models.Commit{commit}, EfficiencyV2Config{}, estimator.EstimateConfig{})
	if len(updated) != 1 {
		t.Fatalf("updated count: want 1, got %d", len(updated))
	}
	need := updated[0]
	if need.CoverageEligible {
		t.Fatal("zero-calendar need must be excluded from coverage")
	}
	if !strings.Contains(need.Reason, "episode has deliverable but zero calendar") {
		t.Fatalf("zero-calendar exclusion must carry audit reason, got %q", need.Reason)
	}
	// 幂等：重复聚合不重复追加 reason
	again := AggregateEfficiencyV2NeedActuals(updated, nil, []models.Commit{commit}, EfficiencyV2Config{}, estimator.EstimateConfig{})
	if strings.Count(again[0].Reason, "episode has deliverable but zero calendar") != 1 {
		t.Fatalf("audit reason must not duplicate on re-aggregation, got %q", again[0].Reason)
	}
}
