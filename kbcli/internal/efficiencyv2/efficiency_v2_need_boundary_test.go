package efficiencyv2

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"kanban/core/models"
)

func TestResolveEfficiencyV2NeedPrecedence(t *testing.T) {
	base := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)
	cases := []struct {
		name       string
		metric     models.SessionStageMetric
		event      models.ConversationEvent
		commits    []models.Commit
		wantSource string
		wantID     string
		wantConf   string
	}{
		{
			name:   "pr wins over branch issue and files",
			metric: efficiencyV2NeedTestMetric("s-pr", "u-alice", "git@example.com/acme/app.git", "feature/TASK-101-login", base, base.Add(time.Hour)),
			event: efficiencyV2NeedTestEvent("e-pr", "s-pr", "u-alice", "git@example.com/acme/app.git", "feature/TASK-101-login", base, EfficiencyV2BoundaryEvidence{
				PRID:       "55",
				IssueID:    "TASK-101",
				BranchName: "feature/TASK-101-login",
				FilePaths:  []string{"app/login.go", "app/login_test.go"},
			}),
			commits:    []models.Commit{efficiencyV2NeedTestCommit("c-pr", "u-alice", "git@example.com/acme/app.git", "feature/TASK-101-login", base.Add(30*time.Minute), "TASK-101 Merge pull request #55")},
			wantSource: efficiencyV2BoundaryPR,
			wantID:     "pr:55",
			wantConf:   efficiencyV2ConfidenceHigh,
		},
		{
			name:   "branch wins over issue and files",
			metric: efficiencyV2NeedTestMetric("s-branch", "u-bob", "git@example.com/acme/app.git", "feature/TASK-102-export", base, base.Add(time.Hour)),
			event: efficiencyV2NeedTestEvent("e-branch", "s-branch", "u-bob", "git@example.com/acme/app.git", "feature/TASK-102-export", base, EfficiencyV2BoundaryEvidence{
				IssueID:    "TASK-102",
				BranchName: "feature/TASK-102-export",
				FilePaths:  []string{"export/a.go", "export/b.go"},
			}),
			wantSource: efficiencyV2BoundaryBranch,
			wantID:     "branch:git@example.com/acme/app.git:feature/TASK-102-export",
			wantConf:   efficiencyV2ConfidenceHigh,
		},
		{
			name:   "issue wins over file cluster",
			metric: efficiencyV2NeedTestMetric("s-issue", "u-cara", "git@example.com/acme/app.git", "main", base, base.Add(time.Hour)),
			event: efficiencyV2NeedTestEvent("e-issue", "s-issue", "u-cara", "git@example.com/acme/app.git", "main", base, EfficiencyV2BoundaryEvidence{
				IssueID:   "TASK-103",
				FilePaths: []string{"orders/a.go", "orders/b.go"},
			}),
			wantSource: efficiencyV2BoundaryIssue,
			wantID:     "issue:TASK-103",
			wantConf:   efficiencyV2ConfidenceMedium,
		},
		{
			name:   "file cluster wins over orphan",
			metric: efficiencyV2NeedTestMetric("s-cluster", "u-dan", "git@example.com/acme/app.git", "main", base, base.Add(time.Hour)),
			event: efficiencyV2NeedTestEvent("e-cluster", "s-cluster", "u-dan", "git@example.com/acme/app.git", "main", base, EfficiencyV2BoundaryEvidence{
				FilePaths: []string{"docs/a.md", "docs/b.md"},
			}),
			wantSource: efficiencyV2BoundaryFileCluster,
			wantID:     "cluster:u-dan:docs",
			wantConf:   efficiencyV2ConfidenceLow,
		},
		{
			name:       "orphan fallback",
			metric:     efficiencyV2NeedTestMetric("s-orphan", "u-erin", "", "", base, base.Add(time.Hour)),
			event:      efficiencyV2NeedTestEvent("e-orphan", "s-orphan", "u-erin", "", "", base, EfficiencyV2BoundaryEvidence{IsOrphan: true}),
			wantSource: efficiencyV2BoundaryOrphan,
			wantID:     "orphan:u-erin",
			wantConf:   efficiencyV2ConfidenceVeryLow,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			needs := ResolveEfficiencyV2Needs([]models.SessionStageMetric{tc.metric}, []models.ConversationEvent{tc.event}, tc.commits, EfficiencyV2Config{})
			if len(needs) != 1 {
				t.Fatalf("need count: want 1, got %d: %#v", len(needs), needs)
			}
			got := needs[0]
			if got.BoundarySource != tc.wantSource || got.NeedId != tc.wantID || got.BoundaryConfidence != tc.wantConf {
				t.Fatalf("need boundary: want %s %s %s, got %s %s %s", tc.wantSource, tc.wantID, tc.wantConf, got.BoundarySource, got.NeedId, got.BoundaryConfidence)
			}
			if got.PrimaryUserId != tc.metric.UserId {
				t.Fatalf("primary user: want %s, got %s", tc.metric.UserId, got.PrimaryUserId)
			}
			if !strings.Contains(string(got.SessionIds), tc.metric.SessionId) {
				t.Fatalf("session ids should include %s, got %s", tc.metric.SessionId, got.SessionIds)
			}
		})
	}
}

func TestResolveEfficiencyV2NeedMainlineBranchFiltering(t *testing.T) {
	base := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)
	for _, branch := range []string{"main", "master", "develop", "release", "release/2026.05", "develop/v2"} {
		t.Run(branch, func(t *testing.T) {
			metric := efficiencyV2NeedTestMetric("s-"+strings.ReplaceAll(branch, "/", "-"), "u-main", "git@example.com/acme/app.git", branch, base, base.Add(time.Hour))
			commit := efficiencyV2NeedTestCommit("c-"+strings.ReplaceAll(branch, "/", "-"), "u-main", "git@example.com/acme/app.git", branch, base.Add(30*time.Minute), "TASK-204 fix mainline regression")

			needs := ResolveEfficiencyV2Needs([]models.SessionStageMetric{metric}, nil, []models.Commit{commit}, EfficiencyV2Config{})
			if len(needs) != 1 {
				t.Fatalf("need count: want 1, got %d", len(needs))
			}
			if needs[0].BoundarySource != efficiencyV2BoundaryIssue {
				t.Fatalf("mainline branch %q should fall through to issue boundary, got %s", branch, needs[0].BoundarySource)
			}
			if needs[0].BoundarySource == efficiencyV2BoundaryBranch {
				t.Fatalf("mainline branch %q must not become lv2 branch", branch)
			}
		})
	}
}

func TestResolveEfficiencyV2NeedCommitTouchedFilesCreateFileCluster(t *testing.T) {
	base := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)
	commit := efficiencyV2NeedTestCommit("c-files", "u-files", "git@example.com/acme/app.git", "main", base, "sync generated report")
	commit.TouchedFiles = EfficiencyV2StringJSON([]string{"reports/monthly.go", "reports/monthly_test.go"})

	needs := ResolveEfficiencyV2Needs(nil, nil, []models.Commit{commit}, EfficiencyV2Config{})
	if len(needs) != 1 {
		t.Fatalf("need count: want 1, got %d", len(needs))
	}
	if needs[0].BoundarySource != efficiencyV2BoundaryFileCluster || needs[0].NeedId != "cluster:u-files:reports" {
		t.Fatalf("commit touched files should create file cluster, got %s/%s", needs[0].BoundarySource, needs[0].NeedId)
	}
	if !strings.Contains(string(needs[0].TouchedFiles), "reports/monthly.go") {
		t.Fatalf("need touched files should include commit files, got %s", needs[0].TouchedFiles)
	}
}

func TestResolveEfficiencyV2NeedMainlineExplicitBranchMetadataIgnored(t *testing.T) {
	base := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)
	metric := efficiencyV2NeedTestMetric("s-main-explicit", "u-main", "git@example.com/acme/app.git", "main", base, base.Add(time.Hour))
	event := efficiencyV2NeedTestEvent("e-main-explicit", "s-main-explicit", "u-main", "git@example.com/acme/app.git", "main", base, EfficiencyV2BoundaryEvidence{
		IssueID: "TASK-205",
	})
	event.Payload = models.ObjectJSON(fixtureJSON(map[string]interface{}{
		"need_id":             "branch:git@example.com/acme/app.git:main",
		"boundary_source":     efficiencyV2BoundaryBranch,
		"boundary_confidence": efficiencyV2ConfidenceHigh,
		"boundary_evidence": EfficiencyV2BoundaryEvidence{
			IssueID: "TASK-205",
		},
	}))

	needs := ResolveEfficiencyV2Needs([]models.SessionStageMetric{metric}, []models.ConversationEvent{event}, nil, EfficiencyV2Config{})
	if len(needs) != 1 {
		t.Fatalf("need count: want 1, got %d", len(needs))
	}
	if needs[0].BoundarySource != efficiencyV2BoundaryIssue || needs[0].NeedId != "issue:TASK-205" {
		t.Fatalf("explicit mainline branch metadata should fall through to issue, got %s/%s", needs[0].BoundarySource, needs[0].NeedId)
	}
}

func TestResolveEfficiencyV2NeedExplicitBoundaryDoesNotBypassHigherPrecedence(t *testing.T) {
	base := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)
	metric := efficiencyV2NeedTestMetric("s-pr-over-branch", "u-alice", "git@example.com/acme/app.git", "feature/TASK-210-login", base, base.Add(time.Hour))
	event := efficiencyV2NeedTestEvent("e-pr-over-branch", "s-pr-over-branch", "u-alice", "git@example.com/acme/app.git", "feature/TASK-210-login", base, EfficiencyV2BoundaryEvidence{})
	event.Payload = models.ObjectJSON(fixtureJSON(map[string]interface{}{
		"need_id":             "branch:git@example.com/acme/app.git:feature/TASK-210-login",
		"boundary_source":     efficiencyV2BoundaryBranch,
		"boundary_confidence": efficiencyV2ConfidenceHigh,
		"boundary_evidence": EfficiencyV2BoundaryEvidence{
			PRID:       "210",
			IssueID:    "TASK-210",
			BranchName: "feature/TASK-210-login",
		},
	}))

	needs := ResolveEfficiencyV2Needs([]models.SessionStageMetric{metric}, []models.ConversationEvent{event}, nil, EfficiencyV2Config{})
	if len(needs) != 1 {
		t.Fatalf("need count: want 1, got %d", len(needs))
	}
	if needs[0].BoundarySource != efficiencyV2BoundaryPR || needs[0].NeedId != "pr:210" {
		t.Fatalf("PR evidence should outrank explicit branch metadata, got %s/%s", needs[0].BoundarySource, needs[0].NeedId)
	}
}

func TestResolveEfficiencyV2NeedOverMaxSpanFlag(t *testing.T) {
	base := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
	metric := efficiencyV2NeedTestMetric("s-long", "u-long", "git@example.com/acme/app.git", "feature/long", base, base.Add(31*24*time.Hour))

	needs := ResolveEfficiencyV2Needs([]models.SessionStageMetric{metric}, nil, nil, EfficiencyV2Config{})
	if len(needs) != 1 {
		t.Fatalf("need count: want 1, got %d", len(needs))
	}
	if !strings.Contains(needs[0].Reason, "max_need_span_days=30") {
		t.Fatalf("reason should flag max span, got %q", needs[0].Reason)
	}
	var evidence map[string]interface{}
	if err := json.Unmarshal([]byte(needs[0].BoundaryEvidence), &evidence); err != nil {
		t.Fatalf("boundary evidence should be valid JSON: %v", err)
	}
	if evidence["span_exceeded"] != true {
		t.Fatalf("boundary evidence should flag span_exceeded, got %#v", evidence)
	}
}

func TestResolveEfficiencyV2NeedCommitExtendsDevEnd(t *testing.T) {
	base := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)
	metric := efficiencyV2NeedTestMetric("s-review", "u-review", "git@example.com/acme/app.git", "feature/review-wait", base, base.Add(time.Hour))
	lateCommit := efficiencyV2NeedTestCommit("c-review-late", "u-review", "git@example.com/acme/app.git", "feature/review-wait", base.Add(3*time.Hour), "Merge pull request #88 from feature/review-wait")

	needs := ResolveEfficiencyV2Needs([]models.SessionStageMetric{metric}, nil, []models.Commit{lateCommit}, EfficiencyV2Config{})
	if len(needs) != 1 {
		t.Fatalf("need count: want 1, got %d", len(needs))
	}
	need := needs[0]
	if need.DevEndTs == nil || !need.DevEndTs.Equal(lateCommit.CommitTime) {
		t.Fatalf("dev_end_ts = %v, want commit time %v", need.DevEndTs, lateCommit.CommitTime)
	}
	if need.DevDurationMin != 180 {
		t.Fatalf("dev_duration_min = %.2f, want 180", need.DevDurationMin)
	}
}

func TestResolveEfficiencyV2NeedFutureCommitClamped(t *testing.T) {
	// 采集侧时区双偏移会产生「未来」commit_time（实测内网多用户 +8h），dev_end 应被 clamp 到当前时刻。
	base := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
	metric := efficiencyV2NeedTestMetric("s-future", "u-future", "git@example.com/acme/app.git", "feature/future-commit", base, base.Add(30*time.Minute))
	futureCommit := efficiencyV2NeedTestCommit("c-future", "u-future", "git@example.com/acme/app.git", "feature/future-commit", time.Now().Add(8*time.Hour), "Merge pull request #99 from feature/future-commit")

	needs := ResolveEfficiencyV2Needs([]models.SessionStageMetric{metric}, nil, []models.Commit{futureCommit}, EfficiencyV2Config{})
	if len(needs) != 1 {
		t.Fatalf("need count: want 1, got %d", len(needs))
	}
	need := needs[0]
	if need.DevEndTs == nil {
		t.Fatal("dev_end_ts is nil")
	}
	if need.DevEndTs.After(time.Now()) {
		t.Fatalf("dev_end_ts = %v 仍在未来，未被 clamp", need.DevEndTs)
	}
	if need.DevStartTs == nil || !need.DevStartTs.Equal(base) {
		t.Fatalf("dev_start_ts = %v, want %v", need.DevStartTs, base)
	}
}

func TestResolveEfficiencyV2NeedLowConfidenceCoverageEligibleFalse(t *testing.T) {
	base := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)
	clusterMetric := efficiencyV2NeedTestMetric("s-cluster", "u-cluster", "git@example.com/acme/app.git", "main", base, base.Add(time.Hour))
	clusterEvent := efficiencyV2NeedTestEvent("e-cluster", "s-cluster", "u-cluster", "git@example.com/acme/app.git", "main", base, EfficiencyV2BoundaryEvidence{
		FilePaths: []string{"docs/a.md", "docs/b.md"},
	})
	orphanMetric := efficiencyV2NeedTestMetric("s-orphan", "u-orphan", "", "", base, base.Add(time.Hour))

	needs := ResolveEfficiencyV2Needs([]models.SessionStageMetric{clusterMetric, orphanMetric}, []models.ConversationEvent{clusterEvent}, nil, EfficiencyV2Config{})
	if len(needs) != 2 {
		t.Fatalf("need count: want 2, got %d", len(needs))
	}
	bySource := map[string]models.Need{}
	for _, need := range needs {
		bySource[need.BoundarySource] = need
	}
	for _, source := range []string{efficiencyV2BoundaryFileCluster, efficiencyV2BoundaryOrphan} {
		need, ok := bySource[source]
		if !ok {
			t.Fatalf("missing need source %s in %#v", source, needs)
		}
		if need.CoverageEligible {
			t.Fatalf("%s confidence %s should be persisted but coverage eligible=false", source, need.BoundaryConfidence)
		}
	}
}

// TestResolveEfficiencyV2NeedRepoAddrCanonMergesWritingStyles 验证 repo_addr_canon 开启时，
// 同一仓库的 git@ 与 https 写法分裂在 branch 边界处合并为同一个 need（key 用归一地址）。
func TestResolveEfficiencyV2NeedRepoAddrCanonMergesWritingStyles(t *testing.T) {
	base := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)
	metric := efficiencyV2NeedTestMetric("s-canon", "u-alice", "git@example.com:acme/app.git", "feature/canon-x", base, base.Add(time.Hour))
	commit := efficiencyV2NeedTestCommit("c-canon", "u-alice", "https://example.com/acme/app", "feature/canon-x", base.Add(30*time.Minute), "feat: canon merge")

	// 开关关闭：两种写法分裂成两个 branch need（旧行为）。
	needsOff := ResolveEfficiencyV2Needs([]models.SessionStageMetric{metric}, nil, []models.Commit{commit}, EfficiencyV2Config{})
	if len(needsOff) != 2 {
		t.Fatalf("canon off: need count = %d, want 2 (写法分裂)", len(needsOff))
	}

	// 开关开启：归一后合并成同一个 need，key 内嵌归一地址。
	needsOn := ResolveEfficiencyV2Needs([]models.SessionStageMetric{metric}, nil, []models.Commit{commit}, EfficiencyV2Config{RepoAddrCanon: true})
	if len(needsOn) != 1 {
		t.Fatalf("canon on: need count = %d, want 1 (写法归一合并)", len(needsOn))
	}
	got := needsOn[0]
	if got.BoundarySource != efficiencyV2BoundaryBranch {
		t.Fatalf("boundary source = %s, want %s", got.BoundarySource, efficiencyV2BoundaryBranch)
	}
	wantKey := "branch:example.com/acme/app:feature/canon-x"
	if got.BoundaryKey != wantKey {
		t.Fatalf("boundary key = %q, want %q", got.BoundaryKey, wantKey)
	}
	if got.RepoAddr != "example.com/acme/app" {
		t.Fatalf("need repo_addr = %q, want 归一地址 example.com/acme/app", got.RepoAddr)
	}
	sessionIDs := EfficiencyV2StringsFromJSON(got.SessionIds)
	commitIDs := EfficiencyV2StringsFromJSON(got.CommitIds)
	if len(sessionIDs) != 1 || len(commitIDs) != 1 {
		t.Fatalf("merged need 应同时含 session 与 commit, got sessions=%v commits=%v", sessionIDs, commitIDs)
	}
}

func efficiencyV2NeedTestMetric(sessionID, userID, repoAddr, branch string, start, end time.Time) models.SessionStageMetric {
	return models.SessionStageMetric{
		SessionId:      sessionID,
		UserId:         userID,
		RepoAddr:       repoAddr,
		RepoBranch:     branch,
		SessionStartTs: &start,
		SessionEndTs:   &end,
		TotalActiveMin: end.Sub(start).Minutes(),
	}
}

func efficiencyV2NeedTestEvent(eventID, sessionID, userID, repoAddr, branch string, start time.Time, evidence EfficiencyV2BoundaryEvidence) models.ConversationEvent {
	end := start.Add(time.Minute)
	payload := fixtureJSON(map[string]interface{}{
		"boundary_evidence": evidence,
	})
	return models.ConversationEvent{
		EventId:      eventID,
		SessionId:    sessionID,
		RequestId:    eventID + "-request",
		UserId:       userID,
		RepoAddr:     repoAddr,
		RepoBranch:   branch,
		EventStartTs: start,
		EventEndTs:   &end,
		EventKind:    "message",
		Payload:      models.ObjectJSON(payload),
		Source:       "test",
		ParseQuality: "exact",
	}
}

func efficiencyV2NeedTestCommit(commitID, userID, repoAddr, branch string, commitTime time.Time, comment string) models.Commit {
	return models.Commit{
		CommitId:   commitID,
		UserId:     userID,
		RepoAddr:   repoAddr,
		RepoBranch: branch,
		CommitTime: commitTime,
		Comment:    comment,
	}
}
