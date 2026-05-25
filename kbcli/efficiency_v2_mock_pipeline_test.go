//go:build mockrun

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"kanban/core/models"

	"gorm.io/gorm/clause"
)

// TestEfficiencyV2MockSeedToDB seeds the synthesised mock rows into a live
// Postgres so the kbcli efficiency-v2 binary can run against real data.
// Gate via env: MOCK_SEED_DSN="host=... port=... user=... password=... dbname=... sslmode=disable"
func TestEfficiencyV2MockSeedToDB(t *testing.T) {
	dsn := os.Getenv("MOCK_SEED_DSN")
	if dsn == "" {
		t.Skip("MOCK_SEED_DSN not set; skipping DB seed")
	}
	root := os.Getenv("MOCK_DATA_ROOT")
	if root == "" {
		root = filepath.Join("..", "工时估算数据")
	}
	tasks := loadMockTasks(t, root)
	sessions, conversations, commits, users := synthesiseMockSupportRows(tasks)

	db, err := models.OpenGormDB(dsn)
	if err != nil {
		t.Fatalf("open gorm db: %v", err)
	}
	t.Logf("seeding %d sessions / %d conversations / %d commits / %d user_org rows to %s",
		len(sessions), len(conversations), len(commits), len(users), maskDSN(dsn))

	if err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"user_name", "org1"}),
	}).Create(&users).Error; err != nil {
		t.Fatalf("seed user_org: %v", err)
	}
	if err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "session_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"create_time", "user_id", "user_name", "client_id", "client_ide",
			"client_version", "client_os", "client_os_version", "session_date", "conversation_date",
		}),
	}).CreateInBatches(&sessions, 100).Error; err != nil {
		t.Fatalf("seed sessions: %v", err)
	}
	// Conversations: unique by (session_id, request_id). Use the same as upserts.
	if err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "session_id"}, {Name: "request_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"task_id", "sender", "mode", "model", "start_time", "end_time",
			"upstream_tokens", "downstream_tokens", "cost", "diff_lines",
			"repo_addr", "repo_branch", "work_dir", "work_dir_id", "user_input",
			"request_content", "response_content",
		}),
	}).CreateInBatches(&conversations, 100).Error; err != nil {
		t.Fatalf("seed conversations: %v", err)
	}
	if err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "commit_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"commit_time", "repo_addr", "repo_branch", "git_user_name", "git_user_email",
			"user_id", "user_name", "client_id", "work_dir", "work_dir_id", "diff_lines",
			"silica", "comment",
		}),
	}).CreateInBatches(&commits, 100).Error; err != nil {
		t.Fatalf("seed commits: %v", err)
	}
	t.Logf("seed complete; now run: .local/bin/kbcli --config .local/kbcli-config.yaml efficiency-v2 --start-date 20260413 --end-date 20260515")
}

func maskDSN(dsn string) string {
	parts := strings.Split(dsn, " ")
	for i, p := range parts {
		if strings.HasPrefix(p, "password=") {
			parts[i] = "password=***"
		}
	}
	return strings.Join(parts, " ")
}

// TestEfficiencyV2MockPipeline runs the v2 pipeline end-to-end in memory
// against synthesised input data loaded from the project's "工时估算数据" JSON
// dump. Run with: go test -tags mockrun -run TestEfficiencyV2MockPipeline -v ./...
func TestEfficiencyV2MockPipeline(t *testing.T) {
	root := os.Getenv("MOCK_DATA_ROOT")
	if root == "" {
		root = filepath.Join("..", "工时估算数据")
	}
	tasks := loadMockTasks(t, root)
	t.Logf("loaded %d task rows from %s", len(tasks), root)

	sessions, conversations, commits, users := synthesiseMockSupportRows(tasks)
	t.Logf("synthesised: sessions=%d conversations=%d commits=%d users=%d",
		len(sessions), len(conversations), len(commits), len(users))

	cfg := EfficiencyV2Config{}
	cfg = normalizeEfficiencyV2Config(cfg)
	algo := EstimateConfig{CommitLinePerMinutes: efficiencyV2DefaultLinesPerMinute, MinMinutes: 5}

	// ---- Step 1: normalise conversation events ----
	events, err := NormalizeEfficiencyV2ConversationEvents(conversations)
	if err != nil {
		t.Fatalf("normalize events: %v", err)
	}
	t.Logf("step1 conversation_events: %d (degraded=%d)", len(events), countDegradedEvents(events))

	// Attach user id from session list (mock substitute for hydrateEfficiencyV2EventUsers).
	usersBySession := map[string]string{}
	for _, s := range sessions {
		usersBySession[s.SessionId] = s.UserId
	}
	for i := range events {
		if events[i].UserId == "" {
			events[i].UserId = usersBySession[events[i].SessionId]
		}
	}

	// ---- Step 2: session stage metrics ----
	metrics := BuildEfficiencyV2SessionStageMetrics(events, cfg)
	t.Logf("step2 session_stage_metrics: %d", len(metrics))
	sumStageMinutes(t, metrics)

	// ---- Step 3: resolve Need boundaries ----
	needs := ResolveEfficiencyV2Needs(metrics, events, commits, cfg)
	t.Logf("step3 needs (raw boundary): %d", len(needs))
	logNeedBoundaryDistribution(t, needs)

	// ---- Step 4: aggregate Need actuals (time + uncovered + signals) ----
	needs = AggregateEfficiencyV2NeedActuals(needs, metrics, commits, cfg, algo)
	logNeedActualStats(t, needs)

	// ---- Step 5: baselines A, B, C + fusion ----
	coefs := DefaultEfficiencyV2BaselineACoefficients()
	anchors := []EfficiencyV2KNNAnchor{} // empty: simulates cold-start with no METR data
	weights := cfg.BaselineDefaults

	snapshots := make([]mockBaselineSnapshot, 0, len(needs))

	for i := range needs {
		need := &needs[i]
		sessionIDs := efficiencyV2StringsFromJSON(need.SessionIds)
		commitIDs := efficiencyV2StringsFromJSON(need.CommitIds)
		var needSessions []models.SessionStageMetric
		for _, m := range metrics {
			for _, id := range sessionIDs {
				if m.SessionId == id {
					needSessions = append(needSessions, m)
					break
				}
			}
		}
		var needCommits []models.Commit
		for _, c := range commits {
			for _, id := range commitIDs {
				if c.CommitId == id {
					needCommits = append(needCommits, c)
					break
				}
			}
		}

		algoResult := ComputeEfficiencyV2BaselineA(*need, needSessions, nil, needCommits, coefs)
		PersistEfficiencyV2BaselineAOnNeed(need, algoResult)

		knnResult := ComputeEfficiencyV2BaselineB(BuildEfficiencyV2NeedFeatureVector(*need, needSessions), anchors, efficiencyV2KNNDefaultK)
		PersistEfficiencyV2BaselineBOnNeed(need, knnResult)

		// LLM: disabled by config (no API key). The path produces reason="llm:disabled".
		llmResult := CallAIForNeedEstimationV4(BuildEfficiencyV2NeedStructuredSummary(*need, needSessions, needCommits, nil), AIEstimationConfig{})
		PersistEfficiencyV2BaselineCOnNeed(need, llmResult)

		fusion := ComputeEfficiencyV2Fusion(*need, EfficiencyV2FusionInputs{
			AlgoMin:     algoResult.TotalMin,
			KNNMin:      knnResult.Estimate,
			LLMMin:      llmResult.TotalMin,
			Weights:     weights,
			TeamDensity: weights.TeamWorkDensity,
		}, cfg)
		PersistEfficiencyV2FusionOnNeed(need, fusion, cfg)

		snapshots = append(snapshots, mockBaselineSnapshot{
			need:    need.NeedId,
			algoMin: algoResult.TotalMin,
			knnMin:  knnResult.Estimate,
			llmMin:  llmResult.TotalMin,
			fused:   fusion.FusedWorkMin,
			ratio:   fusion.EfficiencyRatio,
			conf:    fusion.ConfidenceLevel,
			outlier: fusion.OutlierFlag,
		})
	}
	logBaselineSummary(t, snapshots)

	// ---- Step 6: user-week aggregate ----
	weekly := AggregateEfficiencyV2UserProductivity(needs, cfg)
	t.Logf("step6 user_productivity_v2 rows: %d (users=%d)", len(weekly), countWeeklyUsers(weekly))
	logUserWeekSample(t, weekly, 5)

	// ---- Cross-check against legacy ancient_minutes (ground-truth style) ----
	logLegacyVsBaselineAComparison(t, tasks, snapshots)
}

// ---------- JSON ingest ----------

type rawTaskRow struct {
	TaskId            string  `json:"task_id"`
	SessionId         string  `json:"session_id"`
	CommitId          string  `json:"commit_id"`
	UserId            string  `json:"user_id"`
	UserName          string  `json:"user_name"`
	ClientId          string  `json:"client_id"`
	ClientIde         string  `json:"client_ide"`
	ClientVersion     string  `json:"client_version"`
	ClientOs          string  `json:"client_os"`
	ClientOsVersion   string  `json:"client_os_version"`
	StartTime         string  `json:"start_time"`
	EndTime           string  `json:"end_time"`
	RepoAddr          string  `json:"repo_addr"`
	RepoBranch        string  `json:"repo_branch"`
	WorkDir           string  `json:"work_dir"`
	WorkDirId         string  `json:"work_dir_id"`
	DiffLines         int     `json:"diff_lines"`
	UpstreamTokens    int64   `json:"upstream_tokens"`
	DownstreamTokens  int64   `json:"downstream_tokens"`
	Cost              float64 `json:"cost"`
	Silica            float64 `json:"silica"`
	AcceptRatio       float64 `json:"accept_ratio"`
	TaskAncientMin    float64 `json:"task_ancient_minutes"`
	TaskRealMinutes   float64 `json:"task_real_minutes"`
	TaskAncientReason string  `json:"task_ancient_reason"`
	Title             string  `json:"title"`
	SessionDate       string  `json:"session_date"`
	ConversationDate  string  `json:"conversation_date"`
}

func loadMockTasks(t *testing.T, root string) []rawTaskRow {
	t.Helper()
	files := []string{"ai_le_algo.json", "task_ancient_data.json"}
	var all []rawTaskRow
	for _, f := range files {
		path := filepath.Join(root, f)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var parsed struct {
			Rows []rawTaskRow `json:"rows"`
		}
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		all = append(all, parsed.Rows...)
	}
	return all
}

// synthesiseMockSupportRows fabricates the sessions/conversations/commits/users
// rows the v2 pipeline needs. Only fields actually consumed by v2 algorithms
// are populated; everything else is left zero.
func synthesiseMockSupportRows(tasks []rawTaskRow) (sessions []models.Session, conversations []models.Conversation, commits []models.Commit, users []models.UserOrg) {
	sessionFirstSeen := map[string]rawTaskRow{}
	commitFirstSeen := map[string]rawTaskRow{}
	userSeen := map[string]string{}

	for _, task := range tasks {
		if _, ok := sessionFirstSeen[task.SessionId]; !ok || parseMockTime(task.StartTime).Before(parseMockTime(sessionFirstSeen[task.SessionId].StartTime)) {
			sessionFirstSeen[task.SessionId] = task
		}
		if task.CommitId != "" {
			if _, ok := commitFirstSeen[task.CommitId]; !ok {
				commitFirstSeen[task.CommitId] = task
			}
		}
		if _, ok := userSeen[task.UserId]; !ok {
			userSeen[task.UserId] = task.UserName
		}
	}

	for sessID, task := range sessionFirstSeen {
		sessions = append(sessions, models.Session{
			SessionId:       sessID,
			CreateTime:      parseMockTime(task.StartTime),
			UserId:          task.UserId,
			UserName:        task.UserName,
			ClientId:        task.ClientId,
			ClientIde:       task.ClientIde,
			ClientVersion:   task.ClientVersion,
			ClientOs:        task.ClientOs,
			ClientOsVersion: task.ClientOsVersion,
			SessionDate:     task.SessionDate,
			ConversationDate: task.ConversationDate,
		})
	}

	// One synthetic conversation per task. v2 normalizer will emit edit events
	// (when diff_lines > 0) plus message events when activity exists.
	for i, task := range tasks {
		reqID := fmt.Sprintf("mockreq-%04d", i)
		conv := models.Conversation{
			SessionId:        task.SessionId,
			RequestId:        reqID,
			TaskId:           task.TaskId,
			Sender:           "assistant",
			Mode:             "mock",
			Model:            "mock-model",
			StartTime:        parseMockTime(task.StartTime),
			EndTime:          parseMockTime(task.EndTime),
			ProcessTime:      0,
			UpstreamTokens:   task.UpstreamTokens,
			DownstreamTokens: task.DownstreamTokens,
			Cost:             task.Cost,
			DiffLines:        int64(task.DiffLines),
			RepoAddr:         task.RepoAddr,
			RepoBranch:       task.RepoBranch,
			WorkDir:          task.WorkDir,
			WorkDirId:        task.WorkDirId,
			UserInput:        firstNonEmpty(task.Title, "mock conversation"),
			RequestContent:   "",
			ResponseContent:  "",
		}
		conversations = append(conversations, conv)
	}

	for commitID, task := range commitFirstSeen {
		commits = append(commits, models.Commit{
			CommitId:    commitID,
			CommitTime:  parseMockTime(task.EndTime),
			RepoAddr:    task.RepoAddr,
			RepoBranch:  task.RepoBranch,
			GitUserName: task.UserName,
			GitUserEmail: task.UserName + "@example.invalid",
			UserId:      task.UserId,
			UserName:    task.UserName,
			ClientId:    task.ClientId,
			WorkDir:     task.WorkDir,
			WorkDirId:   task.WorkDirId,
			DiffLines:   task.DiffLines,
			Silica:      task.Silica,
			Comment:     firstNonEmpty(task.Title, "mock commit"),
		})
	}

	for uid, name := range userSeen {
		users = append(users, models.UserOrg{
			UserId:   uid,
			UserName: name,
			Org1:     "mock-org",
		})
	}

	sort.SliceStable(sessions, func(i, j int) bool { return sessions[i].SessionId < sessions[j].SessionId })
	sort.SliceStable(commits, func(i, j int) bool { return commits[i].CommitTime.Before(commits[j].CommitTime) })
	sort.SliceStable(conversations, func(i, j int) bool {
		if conversations[i].SessionId != conversations[j].SessionId {
			return conversations[i].SessionId < conversations[j].SessionId
		}
		return conversations[i].StartTime.Before(conversations[j].StartTime)
	})
	return
}

func parseMockTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	// Sample format: "2026-04-23 11:30:40.977+08"
	layouts := []string{
		"2006-01-02 15:04:05.999-07",
		"2006-01-02 15:04:05.999999-07",
		"2006-01-02 15:04:05-07",
		"2006-01-02 15:04:05.999999-0700",
		"2006-01-02T15:04:05Z07:00",
		time.RFC3339Nano,
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// ---------- Telemetry helpers ----------

func countDegradedEvents(events []models.ConversationEvent) int {
	n := 0
	for _, e := range events {
		if strings.ToLower(e.ParseQuality) != "exact" {
			n++
		}
	}
	return n
}

func sumStageMinutes(t *testing.T, metrics []models.SessionStageMetric) {
	var think, exec, verify, other float64
	confidence := map[string]int{}
	for _, m := range metrics {
		think += m.ThinkActiveMin
		exec += m.ExecutionActiveMin
		verify += m.VerificationActiveMin
		other += m.OtherActiveMin
		confidence[m.StageConfidence]++
	}
	t.Logf("  stage totals (person-minutes): think=%.1f exec=%.1f verify=%.1f other=%.1f", think, exec, verify, other)
	t.Logf("  stage confidence: %v", confidence)
}

func logNeedBoundaryDistribution(t *testing.T, needs []models.Need) {
	by := map[string]int{}
	conf := map[string]int{}
	for _, n := range needs {
		by[n.BoundarySource]++
		conf[n.BoundaryConfidence]++
	}
	t.Logf("  boundary_source: %v", by)
	t.Logf("  boundary_confidence: %v", conf)
}

func logNeedActualStats(t *testing.T, needs []models.Need) {
	withCommits := 0
	withUncovered := 0
	totalPerson := 0.0
	totalCalendar := 0.0
	for _, n := range needs {
		if n.CommitCount > 0 {
			withCommits++
		}
		if n.UncoveredLoc > 0 {
			withUncovered++
		}
		totalPerson += n.TotalSessionActivePersonMin
		totalCalendar += n.TotalCalendarMin
	}
	t.Logf("step4 actuals: needs_with_commits=%d uncovered_needs=%d total_person_min=%.1f total_calendar_min=%.1f",
		withCommits, withUncovered, totalPerson, totalCalendar)
}

type mockBaselineSnapshot struct {
	need    string
	algoMin *float64
	knnMin  *float64
	llmMin  *float64
	fused   *float64
	ratio   *float64
	conf    string
	outlier bool
}

func logBaselineSummary(t *testing.T, snapshots []mockBaselineSnapshot) {
	t.Helper()
	withAlgo, withKNN, withLLM, withFused := 0, 0, 0, 0
	confCount := map[string]int{}
	outliers := 0
	for _, s := range snapshots {
		if s.algoMin != nil {
			withAlgo++
		}
		if s.knnMin != nil {
			withKNN++
		}
		if s.llmMin != nil {
			withLLM++
		}
		if s.fused != nil {
			withFused++
		}
		confCount[s.conf]++
		if s.outlier {
			outliers++
		}
	}
	t.Logf("step5 baseline coverage: algo=%d knn=%d llm=%d fused=%d outliers=%d", withAlgo, withKNN, withLLM, withFused, outliers)
	t.Logf("step5 confidence distribution: %v", confCount)

	// First 5 needs with most extreme ratios (informational).
	sort.SliceStable(snapshots, func(i, j int) bool {
		ri := -1e18
		rj := -1e18
		if snapshots[i].ratio != nil {
			ri = *snapshots[i].ratio
		}
		if snapshots[j].ratio != nil {
			rj = *snapshots[j].ratio
		}
		return ri > rj
	})
	limit := 5
	if limit > len(snapshots) {
		limit = len(snapshots)
	}
	for i := 0; i < limit; i++ {
		s := snapshots[i]
		t.Logf("  top%d need=%s algo=%s knn=%s llm=%s fused=%s ratio=%s conf=%s outlier=%v",
			i+1, s.need,
			floatStr(s.algoMin), floatStr(s.knnMin), floatStr(s.llmMin),
			floatStr(s.fused), floatStr(s.ratio), s.conf, s.outlier,
		)
	}
}

func floatStr(p *float64) string {
	if p == nil {
		return "nil"
	}
	return fmt.Sprintf("%.2f", *p)
}

func countWeeklyUsers(rows []models.UserProductivityV2) int {
	users := map[string]struct{}{}
	for _, r := range rows {
		users[r.UserId] = struct{}{}
	}
	return len(users)
}

func logUserWeekSample(t *testing.T, rows []models.UserProductivityV2, n int) {
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].ActualActiveWorkCorrectedMin > rows[j].ActualActiveWorkCorrectedMin
	})
	if n > len(rows) {
		n = len(rows)
	}
	for i := 0; i < n; i++ {
		r := rows[i]
		ratio := "nil"
		if r.EfficiencyRatio != nil {
			ratio = fmt.Sprintf("%.3f", *r.EfficiencyRatio)
		}
		t.Logf("  user=%s week=%s merged=%d active=%d abandoned=%d actual_cal=%.1f base_cal=%.1f ratio=%s conf_limited=%v reason=%q",
			r.UserId, r.WeekStart.Format("2006-01-02"),
			r.MergedNeedCount, r.ActiveNeedCount, r.AbandonedNeedCount,
			r.ActualCalendarMin, r.BaselineCalendarMin, ratio, r.ConfidenceLimited, r.ConfidenceReason)
	}
}

// logLegacyVsBaselineAComparison compares Baseline A's algorithmic estimate
// to the legacy `task_ancient_minutes` field that already exists in the dump.
// This gives a directional sanity check, not a precise validation: legacy
// numbers are task-level, v2 numbers are Need-level after aggregation.
func logLegacyVsBaselineAComparison(t *testing.T, tasks []rawTaskRow, snapshots []mockBaselineSnapshot) {
	legacyTotal := 0.0
	for _, task := range tasks {
		legacyTotal += task.TaskAncientMin
	}
	algoTotal := 0.0
	algoNeedCount := 0
	for _, s := range snapshots {
		if s.algoMin != nil {
			algoTotal += *s.algoMin
			algoNeedCount++
		}
	}
	t.Logf("legacy total task_ancient_minutes = %.1f over %d tasks", legacyTotal, len(tasks))
	t.Logf("v2 Baseline A total work_min = %.1f over %d needs", algoTotal, algoNeedCount)
	if legacyTotal > 0 {
		t.Logf("v2/legacy ratio = %.2fx (rough — units differ: task vs Need)", algoTotal/legacyTotal)
	}
}

// Hash helpers (kept for completeness; used if we needed deterministic mock IDs).
func mockHashID(prefix, key string) string {
	sum := sha256.Sum256([]byte(prefix + "|" + key))
	return prefix + "_" + hex.EncodeToString(sum[:8])
}
