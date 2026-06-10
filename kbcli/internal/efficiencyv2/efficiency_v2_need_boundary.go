package efficiencyv2

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
	"time"

	"kanban/core/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const efficiencyV2DefaultMaxNeedSpanDays = 30

var (
	efficiencyV2PRPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bpull request\s*#?([0-9]+)\b`),
		regexp.MustCompile(`(?i)\bpr\s*#?([0-9]+)\b`),
	}
	efficiencyV2IssuePattern = regexp.MustCompile(`\b[A-Z][A-Z0-9]+-[0-9]+\b`)
)

type efficiencyV2BoundaryCandidate struct {
	sessionID  string
	commitID   string
	userID     string
	repoAddr   string
	branch     string
	start      time.Time
	end        time.Time
	needID     string
	source     string
	confidence string
	status     string
	prID       string
	issueID    string
	files      []string
	comment    string
}

type efficiencyV2NeedBucket struct {
	source     string
	confidence string
	key        string
	candidates []efficiencyV2BoundaryCandidate
}

type efficiencyV2PayloadBoundary struct {
	NeedID             string                       `json:"need_id"`
	BoundarySource     string                       `json:"boundary_source"`
	BoundaryConfidence string                       `json:"boundary_confidence"`
	Status             string                       `json:"status"`
	BoundaryEvidence   EfficiencyV2BoundaryEvidence `json:"boundary_evidence"`
	MockFiles          []string                     `json:"mock_files"`
}

// ResolveAndUpsertEfficiencyV2Needs 解析并落库全量 Need 边界，返回 needs 与
// prune 删除的旧 need 行数。删除数 >0 说明 key 换代，调用方必须全量重写
// user_productivity_v2（见 RebuildEfficiencyV2UserProductivityAll）：窗口化重算
// 只重写窗口内的周，窗口外历史周的 need_ids 会悬挂引用已删 need。
// commits 必须全量加载、不带日期窗：边界解析要看全量历史，否则分析窗切到
// episode 中间会让段首日期漂移、产生重复 need 行；日期窗只约束下游聚合。
func ResolveAndUpsertEfficiencyV2Needs(db *gorm.DB, cfg EfficiencyV2Config) ([]models.Need, int, error) {
	var metrics []models.SessionStageMetric
	if err := db.Order("session_id ASC").Find(&metrics).Error; err != nil {
		return nil, 0, fmt.Errorf("query session stage metrics: %w", err)
	}
	var events []models.ConversationEvent
	if err := db.Order("session_id ASC").Order("event_start_ts ASC").Order("event_id ASC").Find(&events).Error; err != nil {
		return nil, 0, fmt.Errorf("query conversation events: %w", err)
	}
	var commits []models.Commit
	if err := db.Order("commit_time ASC").Order("commit_id ASC").Find(&commits).Error; err != nil {
		return nil, 0, fmt.Errorf("query commits: %w", err)
	}

	needs := ResolveEfficiencyV2Needs(metrics, events, commits, cfg)
	if len(needs) == 0 {
		return needs, 0, nil
	}
	if err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "need_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"boundary_source",
			"boundary_confidence",
			"boundary_key",
			"boundary_evidence",
			"status",
			"repo_addr",
			"repo_branch",
			"primary_user_id",
			"contributor_user_ids",
			"session_ids",
			"commit_ids",
			"touched_files",
			"dev_start_ts",
			"dev_end_ts",
			"dev_duration_min",
			"coverage_eligible",
			"reason",
			"updated_at",
		}),
	}).CreateInBatches(&needs, 500).Error; err != nil {
		return nil, 0, fmt.Errorf("upsert needs: %w", err)
	}
	if err := updateEfficiencyV2StageMetricNeedIDs(db, needs); err != nil {
		return nil, 0, err
	}
	pruned, err := pruneEfficiencyV2StaleNeeds(db, needs)
	if err != nil {
		return nil, 0, err
	}
	return needs, pruned, nil
}

// pruneEfficiencyV2StaleNeeds 删除本轮重算未重新生成的 need 行并返回删除数：
// key 方案变更（如 episode 后缀）后旧 need_id 残留——upsert 只增改不删。
// 幂等：第二次跑 stale 集为空。session_stage_metrics.need_id 已由
// updateEfficiencyV2StageMetricNeedIDs 全量覆盖刷新（每个 session 必然落入本轮
// 某个 need），不会留下悬挂引用。调用方保证 needs 非空，不会出现"空轮清空全表"。
func pruneEfficiencyV2StaleNeeds(db *gorm.DB, current []models.Need) (int, error) {
	var existing []string
	if err := db.Model(&models.Need{}).Pluck("need_id", &existing).Error; err != nil {
		return 0, fmt.Errorf("list need ids for prune: %w", err)
	}
	currentIDs := make([]string, 0, len(current))
	for _, need := range current {
		currentIDs = append(currentIDs, need.NeedId)
	}
	stale := efficiencyV2StaleIDs(existing, currentIDs)
	if len(stale) == 0 {
		log.Printf("efficiency-v2 prune: no stale need rows")
		return 0, nil
	}
	const batch = 500
	for i := 0; i < len(stale); i += batch {
		end := i + batch
		if end > len(stale) {
			end = len(stale)
		}
		if err := db.Where("need_id IN ?", stale[i:end]).Delete(&models.Need{}).Error; err != nil {
			return 0, fmt.Errorf("prune stale needs: %w", err)
		}
	}
	log.Printf("efficiency-v2 prune: deleted %d stale need rows", len(stale))
	return len(stale), nil
}

// efficiencyV2StaleIDs 返回 existing 中不在 current 里的 id（排序稳定）。
// 供 need 表与 user_productivity_v2 表的 prune 共用。
func efficiencyV2StaleIDs(existing, current []string) []string {
	keep := make(map[string]bool, len(current))
	for _, id := range current {
		keep[id] = true
	}
	stale := make([]string, 0)
	for _, id := range existing {
		if !keep[id] {
			stale = append(stale, id)
		}
	}
	stale = EfficiencyV2SortedUnique(stale)
	return stale
}

func updateEfficiencyV2StageMetricNeedIDs(db *gorm.DB, needs []models.Need) error {
	for _, need := range needs {
		sessionIDs := EfficiencyV2StringsFromJSON(need.SessionIds)
		if len(sessionIDs) == 0 {
			continue
		}
		if err := db.Model(&models.SessionStageMetric{}).
			Where("session_id IN ?", sessionIDs).
			Update("need_id", need.NeedId).Error; err != nil {
			return fmt.Errorf("update stage metric need_id %s: %w", need.NeedId, err)
		}
	}
	return nil
}

func ResolveEfficiencyV2Needs(stageMetrics []models.SessionStageMetric, events []models.ConversationEvent, commits []models.Commit, cfg EfficiencyV2Config) []models.Need {
	cfg = NormalizeEfficiencyV2Config(cfg)
	if cfg.MaxNeedSpanDays == 0 {
		cfg.MaxNeedSpanDays = efficiencyV2DefaultMaxNeedSpanDays
	}

	eventsBySession := make(map[string][]models.ConversationEvent)
	for _, event := range events {
		eventsBySession[event.SessionId] = append(eventsBySession[event.SessionId], event)
	}

	var candidates []efficiencyV2BoundaryCandidate
	for _, metric := range stageMetrics {
		candidates = append(candidates, efficiencyV2CandidateFromStageMetric(metric, eventsBySession[metric.SessionId]))
	}
	for _, commit := range commits {
		candidates = append(candidates, efficiencyV2CandidateFromCommit(commit))
	}
	efficiencyV2PropagateCommitBoundaryClues(candidates)

	bucketsByKey := make(map[string]*efficiencyV2NeedBucket)
	for _, candidate := range candidates {
		source, confidence, key := efficiencyV2ResolveBoundary(candidate)
		bucketKey := source + "\x00" + key
		bucket := bucketsByKey[bucketKey]
		if bucket == nil {
			bucket = &efficiencyV2NeedBucket{source: source, confidence: confidence, key: key}
			bucketsByKey[bucketKey] = bucket
		}
		bucket.candidates = append(bucket.candidates, candidate)
	}

	bucketKeys := make([]string, 0, len(bucketsByKey))
	for key := range bucketsByKey {
		bucketKeys = append(bucketKeys, key)
	}
	sort.Strings(bucketKeys)

	idleThreshold := efficiencyV2IdleThreshold(cfg)
	needs := make([]models.Need, 0, len(bucketKeys))
	for _, key := range bucketKeys {
		for _, episode := range efficiencyV2SplitBucketEpisodes(*bucketsByKey[key], idleThreshold) {
			needs = append(needs, efficiencyV2BuildNeed(episode, cfg))
		}
	}
	return needs
}

// efficiencyV2SplitBucketEpisodes 把一个静态身份桶（branch/PR/issue/cluster/orphan）
// 按活动间隙二级切分成多个 episode 段，一段 = 一个 need：候选按 start 升序扫描，
// 下一候选 start 与当前段 max(end) 的间隙超过 idleThreshold 即切新段。
// 单段桶原样返回（need_id 不加后缀，保持存量兼容）；多段时每段 key 追加
// "@YYYY-MM-DD"（段首候选 start 的 UTC 日期，段间间隙 > idleThreshold 保证日期互异）。
// start 为零值的候选无法排序，归入第一段（cluster/orphan 的零值候选已在 key 层
// 分流到独立 undated 桶，不会进入多段切分）。
func efficiencyV2SplitBucketEpisodes(bucket efficiencyV2NeedBucket, idleThreshold time.Duration) []efficiencyV2NeedBucket {
	var undated, timed []efficiencyV2BoundaryCandidate
	for _, candidate := range bucket.candidates {
		if candidate.start.IsZero() {
			undated = append(undated, candidate)
		} else {
			timed = append(timed, candidate)
		}
	}
	if len(timed) == 0 {
		return []efficiencyV2NeedBucket{bucket}
	}
	sort.SliceStable(timed, func(i, j int) bool { return timed[i].start.Before(timed[j].start) })

	var segments [][]efficiencyV2BoundaryCandidate
	var current []efficiencyV2BoundaryCandidate
	var maxEnd time.Time
	for _, candidate := range timed {
		if len(current) > 0 && candidate.start.Sub(maxEnd) > idleThreshold {
			segments = append(segments, current)
			current = nil
			// 重置间隙基准：新段的 max(end) 只看本段候选，防止脏数据
			// （end 早于 start 的候选）让上一段的 maxEnd 残留进新段。
			maxEnd = time.Time{}
		}
		current = append(current, candidate)
		end := candidate.end
		if end.IsZero() {
			end = candidate.start
		}
		if end.After(maxEnd) {
			maxEnd = end
		}
	}
	segments = append(segments, current)

	if len(segments) == 1 {
		return []efficiencyV2NeedBucket{bucket}
	}

	episodes := make([]efficiencyV2NeedBucket, 0, len(segments))
	for i, segment := range segments {
		episode := bucket
		episode.key = bucket.key + "@" + segment[0].start.UTC().Format("2006-01-02")
		if i == 0 && len(undated) > 0 {
			segment = append(append([]efficiencyV2BoundaryCandidate(nil), undated...), segment...)
		}
		episode.candidates = segment
		episodes = append(episodes, episode)
	}
	return episodes
}

func efficiencyV2PropagateCommitBoundaryClues(candidates []efficiencyV2BoundaryCandidate) {
	type clue struct {
		prID    string
		issueID string
	}
	cluesByRepoBranch := make(map[string]clue)
	for _, candidate := range candidates {
		if candidate.repoAddr == "" || candidate.branch == "" {
			continue
		}
		key := candidate.repoAddr + "\x00" + candidate.branch
		current := cluesByRepoBranch[key]
		if current.prID == "" && candidate.prID != "" {
			current.prID = candidate.prID
		}
		if current.issueID == "" && candidate.issueID != "" {
			current.issueID = candidate.issueID
		}
		cluesByRepoBranch[key] = current
	}
	for i := range candidates {
		if candidates[i].repoAddr == "" || candidates[i].branch == "" {
			continue
		}
		key := candidates[i].repoAddr + "\x00" + candidates[i].branch
		clue := cluesByRepoBranch[key]
		if candidates[i].prID == "" {
			candidates[i].prID = clue.prID
		}
		if candidates[i].issueID == "" {
			candidates[i].issueID = clue.issueID
		}
	}
}

func efficiencyV2CandidateFromStageMetric(metric models.SessionStageMetric, events []models.ConversationEvent) efficiencyV2BoundaryCandidate {
	candidate := efficiencyV2BoundaryCandidate{
		sessionID: metric.SessionId,
		userID:    metric.UserId,
		repoAddr:  metric.RepoAddr,
		branch:    metric.RepoBranch,
	}
	if metric.SessionStartTs != nil {
		candidate.start = *metric.SessionStartTs
	}
	if metric.SessionEndTs != nil {
		candidate.end = *metric.SessionEndTs
	}
	if candidate.end.IsZero() {
		candidate.end = candidate.start
	}

	for _, event := range events {
		if candidate.repoAddr == "" {
			candidate.repoAddr = event.RepoAddr
		}
		if candidate.branch == "" {
			candidate.branch = event.RepoBranch
		}
		if candidate.userID == "" {
			candidate.userID = event.UserId
		}
		if candidate.start.IsZero() || event.EventStartTs.Before(candidate.start) {
			candidate.start = event.EventStartTs
		}
		eventEnd := event.EventStartTs
		if event.EventEndTs != nil {
			eventEnd = *event.EventEndTs
		}
		if candidate.end.IsZero() || eventEnd.After(candidate.end) {
			candidate.end = eventEnd
		}
		payload := efficiencyV2BoundaryPayloadFromEvent(event.Payload)
		evidence := payload.BoundaryEvidence
		if candidate.needID == "" {
			candidate.needID = strings.TrimSpace(payload.NeedID)
		}
		if candidate.source == "" {
			candidate.source = strings.TrimSpace(payload.BoundarySource)
		}
		if candidate.confidence == "" {
			candidate.confidence = strings.TrimSpace(payload.BoundaryConfidence)
		}
		if candidate.status == "" {
			candidate.status = strings.TrimSpace(payload.Status)
		}
		if candidate.prID == "" {
			candidate.prID = strings.TrimSpace(evidence.PRID)
		}
		if candidate.issueID == "" {
			candidate.issueID = strings.TrimSpace(evidence.IssueID)
		}
		if candidate.branch == "" {
			candidate.branch = strings.TrimSpace(evidence.BranchName)
		}
		candidate.files = append(candidate.files, evidence.FilePaths...)
		candidate.files = append(candidate.files, EfficiencyV2StringsFromJSON(event.TouchedFiles)...)
	}

	return candidate
}

func efficiencyV2CandidateFromCommit(commit models.Commit) efficiencyV2BoundaryCandidate {
	candidate := efficiencyV2BoundaryCandidate{
		commitID: commit.CommitId,
		userID:   commit.UserId,
		repoAddr: commit.RepoAddr,
		branch:   commit.RepoBranch,
		start:    commit.CommitTime,
		end:      commit.CommitTime,
		comment:  commit.Comment,
		files:    EfficiencyV2StringsFromJSON(commit.TouchedFiles),
	}
	candidate.prID = efficiencyV2ExtractPRID(commit.Comment)
	candidate.issueID = efficiencyV2ExtractIssueID(commit.Comment)
	if candidate.issueID == "" {
		candidate.issueID = efficiencyV2ExtractIssueID(commit.RepoBranch)
	}
	return candidate
}

func efficiencyV2BoundaryPayloadFromEvent(payload models.ObjectJSON) efficiencyV2PayloadBoundary {
	var decoded efficiencyV2PayloadBoundary
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		return efficiencyV2PayloadBoundary{}
	}
	decoded.BoundaryEvidence.FilePaths = append(decoded.BoundaryEvidence.FilePaths, decoded.MockFiles...)
	return decoded
}

func efficiencyV2ResolveBoundary(candidate efficiencyV2BoundaryCandidate) (string, string, string) {
	source, confidence, key := efficiencyV2ResolveNaturalBoundary(candidate)
	if candidate.source == source {
		if candidate.confidence != "" {
			confidence = candidate.confidence
		}
		if candidate.needID != "" {
			key = candidate.needID
		}
	}
	return source, confidence, key
}

func efficiencyV2ResolveNaturalBoundary(candidate efficiencyV2BoundaryCandidate) (string, string, string) {
	if candidate.prID != "" {
		return efficiencyV2BoundaryPR, efficiencyV2ConfidenceHigh, "pr:" + candidate.prID
	}
	if candidate.branch != "" && !efficiencyV2IsMainlineBranch(candidate.branch) {
		return efficiencyV2BoundaryBranch, efficiencyV2ConfidenceHigh, fmt.Sprintf("branch:%s:%s", candidate.repoAddr, candidate.branch)
	}
	issueID := candidate.issueID
	if issueID == "" {
		issueID = efficiencyV2ExtractIssueID(candidate.branch)
	}
	if issueID == "" {
		issueID = efficiencyV2ExtractIssueID(candidate.comment)
	}
	if issueID != "" {
		return efficiencyV2BoundaryIssue, efficiencyV2ConfidenceMedium, "issue:" + issueID
	}
	files := EfficiencyV2SortedUnique(candidate.files)
	if len(files) >= 2 {
		confidence := efficiencyV2ConfidenceLow
		if candidate.start.IsZero() {
			// 无任何时间信号的候选进独立 undated 桶，置信度再降一档。
			confidence = efficiencyV2ConfidenceVeryLow
		}
		return efficiencyV2BoundaryFileCluster, confidence, efficiencyV2FileClusterKey(candidate.userID, candidate.start, files)
	}
	return efficiencyV2BoundaryOrphan, efficiencyV2ConfidenceVeryLow, efficiencyV2OrphanKey(candidate.userID, candidate.start)
}

func efficiencyV2BuildNeed(bucket efficiencyV2NeedBucket, cfg EfficiencyV2Config) models.Need {
	contributors := make(map[string]bool)
	sessions := make(map[string]bool)
	commits := make(map[string]bool)
	files := make(map[string]bool)
	commitMessages := make([]string, 0)
	var repoAddr, branch, primaryUser string
	status := "active"
	var devStart, devEnd time.Time

	for _, candidate := range bucket.candidates {
		if repoAddr == "" {
			repoAddr = candidate.repoAddr
		}
		if branch == "" {
			branch = candidate.branch
		}
		if primaryUser == "" {
			primaryUser = candidate.userID
		}
		if candidate.userID != "" {
			contributors[candidate.userID] = true
		}
		if candidate.sessionID != "" {
			sessions[candidate.sessionID] = true
		}
		if candidate.commitID != "" {
			commits[candidate.commitID] = true
		}
		if candidate.comment != "" {
			commitMessages = append(commitMessages, candidate.comment)
		}
		if candidate.status != "" && status == "active" {
			status = candidate.status
		}
		for _, file := range candidate.files {
			file = strings.TrimSpace(file)
			if file != "" {
				files[file] = true
			}
		}
		if !candidate.start.IsZero() && (devStart.IsZero() || candidate.start.Before(devStart)) {
			devStart = candidate.start
		}
		if !candidate.end.IsZero() && (devEnd.IsZero() || candidate.end.After(devEnd)) {
			devEnd = candidate.end
		}
	}

	// 采集侧存在把本地时间误标 UTC 的 commit（实测多用户 +8h 的「未来」时间戳），
	// 会把 dev span 顶到未来：仅对超过当前时刻的端点 clamp，不做全量纠偏（历史时段内的偏移无法区分）。
	now := time.Now()
	if !devEnd.IsZero() && devEnd.After(now) {
		log.Printf("efficiency-v2 need %s: dev_end %s 晚于当前时刻，已 clamp 到 now（疑似上游时区双偏移）", bucket.key, devEnd.Format(time.RFC3339))
		devEnd = now
	}
	if !devStart.IsZero() && devStart.After(now) {
		devStart = now
	}

	contributorIDs := efficiencyV2SortedMapKeys(contributors)
	if primaryUser == "" && len(contributorIDs) > 0 {
		primaryUser = contributorIDs[0]
	}

	if status == "active" && len(commits) > 0 {
		status = "merged"
	}

	spanExceeded := false
	if !devStart.IsZero() && !devEnd.IsZero() && cfg.MaxNeedSpanDays > 0 {
		spanExceeded = devEnd.Sub(devStart) > time.Duration(cfg.MaxNeedSpanDays)*24*time.Hour
	}

	// 多人集成流降级：episode 切分后仍然跨度长且多人贡献的段（如长命集成分支）
	// 不是单个可归属的需求，个体提效比不可解释——confidence 降为 low，
	// 由既有 eligible 规则（仅 high/medium 可入）自动踢出，不加新 flag。
	confidence := bucket.confidence
	integrationFlow := false
	if (confidence == efficiencyV2ConfidenceHigh || confidence == efficiencyV2ConfidenceMedium) &&
		cfg.IntegrationFlowSpanDays > 0 && cfg.IntegrationFlowMinContributors > 0 &&
		!devStart.IsZero() && !devEnd.IsZero() &&
		devEnd.Sub(devStart) > time.Duration(cfg.IntegrationFlowSpanDays)*24*time.Hour &&
		len(contributorIDs) >= cfg.IntegrationFlowMinContributors {
		confidence = efficiencyV2ConfidenceLow
		integrationFlow = true
	}

	evidence := map[string]interface{}{
		"source":          bucket.source,
		"key":             bucket.key,
		"commit_messages": EfficiencyV2SortedUnique(commitMessages),
		"span_exceeded":   spanExceeded,
	}
	if spanExceeded {
		evidence["max_need_span_days"] = cfg.MaxNeedSpanDays
	}
	if integrationFlow {
		evidence["integration_flow"] = true
	}

	needKey := efficiencyV2ClampNeedKey(bucket.key)
	need := models.Need{
		NeedId:             needKey,
		BoundarySource:     bucket.source,
		BoundaryConfidence: confidence,
		BoundaryKey:        needKey,
		BoundaryEvidence:   efficiencyV2NeedObjectJSON(evidence),
		Status:             status,
		RepoAddr:           repoAddr,
		RepoBranch:         branch,
		PrimaryUserId:      primaryUser,
		ContributorUserIds: EfficiencyV2StringJSON(contributorIDs),
		SessionIds:         EfficiencyV2StringJSON(efficiencyV2SortedMapKeys(sessions)),
		CommitIds:          EfficiencyV2StringJSON(efficiencyV2SortedMapKeys(commits)),
		TouchedFiles:       EfficiencyV2StringJSON(efficiencyV2SortedMapKeys(files)),
		CoverageEligible:   status == "merged" && (confidence == efficiencyV2ConfidenceHigh || confidence == efficiencyV2ConfidenceMedium),
	}
	if !devStart.IsZero() {
		start := devStart
		need.DevStartTs = &start
	}
	if !devEnd.IsZero() {
		end := devEnd
		need.DevEndTs = &end
	}
	if need.DevStartTs != nil && need.DevEndTs != nil {
		need.DevDurationMin = need.DevEndTs.Sub(*need.DevStartTs).Minutes()
	}
	reasons := make([]string, 0, 2)
	if spanExceeded {
		reasons = append(reasons, fmt.Sprintf("need span exceeds max_need_span_days=%d", cfg.MaxNeedSpanDays))
	}
	if integrationFlow {
		reasons = append(reasons, fmt.Sprintf("integration flow: span exceeds %dd with >=%d contributors, confidence downgraded to low",
			cfg.IntegrationFlowSpanDays, cfg.IntegrationFlowMinContributors))
	}
	need.Reason = strings.Join(reasons, "; ")
	return need
}

func efficiencyV2ExtractPRID(text string) string {
	for _, pattern := range efficiencyV2PRPatterns {
		matches := pattern.FindStringSubmatch(text)
		if len(matches) == 2 {
			return matches[1]
		}
	}
	return ""
}

func efficiencyV2ExtractIssueID(text string) string {
	return efficiencyV2IssuePattern.FindString(strings.ToUpper(text))
}

func efficiencyV2ConfidenceForBoundarySource(source string) string {
	switch source {
	case efficiencyV2BoundaryPR, efficiencyV2BoundaryBranch:
		return efficiencyV2ConfidenceHigh
	case efficiencyV2BoundaryIssue:
		return efficiencyV2ConfidenceMedium
	case efficiencyV2BoundaryFileCluster:
		return efficiencyV2ConfidenceLow
	default:
		return efficiencyV2ConfidenceVeryLow
	}
}

func efficiencyV2IsMainlineBranch(branch string) bool {
	branch = strings.TrimSpace(strings.ToLower(branch))
	return branch == "main" || branch == "master" || branch == "develop" || branch == "release" ||
		strings.HasPrefix(branch, "release/") || strings.HasPrefix(branch, "develop/")
}

// efficiencyV2EpisodeKeySuffixPattern 匹配 episode 切分追加的 "@YYYY-MM-DD" 段后缀。
var efficiencyV2EpisodeKeySuffixPattern = regexp.MustCompile(`@\d{4}-\d{2}-\d{2}$`)

// efficiencyV2ClampNeedKey 把 need 边界 key 截断到 100 字符以内，用作 need_id / boundary_key。
// 超长（如 file-cluster 把大量顶层目录拼接）时，保留可读前缀 + 全量 key 的短哈希，
// 避免不同长 key 截断成同一前缀后发生主键碰撞、把不同 need 错误合并。
// episode 的 "@YYYY-MM-DD" 后缀必须保住（放在 hash 段之后）：截断把后缀截掉的话，
// 超长桶的多个段会塌缩成同一个 need_id 互相覆盖。
func efficiencyV2ClampNeedKey(key string) string {
	const maxRunes = 100
	if len([]rune(key)) <= maxRunes {
		return key
	}
	suffix := efficiencyV2EpisodeKeySuffixPattern.FindString(key)
	base := strings.TrimSuffix(key, suffix)
	sum := sha1.Sum([]byte(key))
	hash := hex.EncodeToString(sum[:])[:12]
	// 后缀是纯 ASCII，len 即 rune 数；prefix + "~" + hash + suffix = 100。
	prefixLen := maxRunes - len(suffix) - 1 - len(hash)
	prefix := string([]rune(base)[:prefixLen])
	return prefix + "~" + hash + suffix
}

// cluster/orphan key 已去掉 ISO 周组件：跨周硬切与周内乱合改由 episode 二级
// 切分自然分段。start 为零值（无任何时间信号）的候选无法参与 episode 排序，
// 单独归入独立 undated 桶。
func efficiencyV2FileClusterKey(userID string, start time.Time, files []string) string {
	if start.IsZero() {
		return fmt.Sprintf("cluster:%s:undated:%s", userID, efficiencyV2ClusterSlug(files))
	}
	return fmt.Sprintf("cluster:%s:%s", userID, efficiencyV2ClusterSlug(files))
}

func efficiencyV2OrphanKey(userID string, start time.Time) string {
	if start.IsZero() {
		return fmt.Sprintf("orphan:%s:undated", userID)
	}
	return fmt.Sprintf("orphan:%s", userID)
}

func efficiencyV2ClusterSlug(files []string) string {
	files = EfficiencyV2SortedUnique(files)
	if len(files) == 0 {
		return "unknown"
	}
	parts := make([]string, 0, len(files))
	for _, file := range files {
		file = strings.Trim(file, "/")
		if file == "" {
			continue
		}
		piece := file
		if slash := strings.Index(file, "/"); slash >= 0 {
			piece = file[:slash]
		}
		piece = strings.TrimSuffix(piece, ".go")
		piece = strings.TrimSuffix(piece, ".ts")
		piece = strings.TrimSuffix(piece, ".md")
		if piece != "" {
			parts = append(parts, piece)
		}
	}
	parts = EfficiencyV2SortedUnique(parts)
	if len(parts) == 0 {
		return "unknown"
	}
	return strings.Join(parts, "-")
}

func EfficiencyV2StringJSON(values []string) models.StringJSON {
	data, err := json.Marshal(values)
	if err != nil {
		return models.StringJSON("[]")
	}
	return models.StringJSON(data)
}

func efficiencyV2NeedObjectJSON(value interface{}) models.ObjectJSON {
	data, err := json.Marshal(value)
	if err != nil {
		return models.ObjectJSON("{}")
	}
	return models.ObjectJSON(data)
}

func EfficiencyV2StringsFromJSON(value models.StringJSON) []string {
	if value == "" {
		return nil
	}
	var values []string
	if err := json.Unmarshal([]byte(value), &values); err != nil {
		return nil
	}
	return values
}

func efficiencyV2SortedMapKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for value := range values {
		if value != "" {
			keys = append(keys, value)
		}
	}
	sort.Strings(keys)
	return keys
}

func EfficiencyV2SortedUnique(values []string) []string {
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			seen[value] = true
		}
	}
	return efficiencyV2SortedMapKeys(seen)
}
