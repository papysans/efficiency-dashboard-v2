package efficiencyv2

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"kanban/core/models"
	"kanban/kbcli/internal/governance"

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
	mergeTs    *time.Time
	mergeOnly  bool
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

func ResolveAndUpsertEfficiencyV2Needs(db *gorm.DB, cfg EfficiencyV2Config, startDate, endDate string) ([]models.Need, error) {
	var metrics []models.SessionStageMetric
	if err := db.Order("session_id ASC").Find(&metrics).Error; err != nil {
		return nil, fmt.Errorf("query session stage metrics: %w", err)
	}
	var events []models.ConversationEvent
	if err := db.Order("session_id ASC").Order("event_start_ts ASC").Order("event_id ASC").Find(&events).Error; err != nil {
		return nil, fmt.Errorf("query conversation events: %w", err)
	}
	// 治理排除的 commit 不参与 Need 边界构建（双保险：聚合口径处也按 effective 记 0）
	commitsQuery := db.Where("excluded_flag = false").Order("commit_time ASC").Order("commit_id ASC")
	if startDate != "" {
		commitsQuery = commitsQuery.Where("DATE(commit_time) >= ?", startDate)
	}
	if endDate != "" {
		commitsQuery = commitsQuery.Where("DATE(commit_time) <= ?", endDate)
	}
	var commits []models.Commit
	if err := commitsQuery.Find(&commits).Error; err != nil {
		return nil, fmt.Errorf("query commits: %w", err)
	}

	needs := ResolveEfficiencyV2Needs(metrics, events, commits, cfg)
	if len(needs) == 0 {
		return needs, nil
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
			"merge_ts",
			"dev_duration_min",
			"wait_for_review_min",
			"coverage_eligible",
			"reason",
			"updated_at",
		}),
	}).CreateInBatches(&needs, 500).Error; err != nil {
		return nil, fmt.Errorf("upsert needs: %w", err)
	}
	if err := updateEfficiencyV2StageMetricNeedIDs(db, needs); err != nil {
		return nil, err
	}
	if cfg.RepoAddrCanon {
		if err := cleanupEfficiencyV2PreCanonNeeds(db, needs); err != nil {
			return nil, err
		}
	}
	if err := cleanupEfficiencyV2FullyExcludedNeeds(db); err != nil {
		return nil, err
	}
	return needs, nil
}

// cleanupEfficiencyV2FullyExcludedNeeds 清理"内容物已被治理全量排除"的残留 need：
// 无任何 session 且 commit 全部 excluded_flag=true 的 need，本轮边界重算不会再生成它
// （排除的 commit 不进候选），旧行会带着治理前的统计永久残留在列表里。
// 删除幂等且可恢复：若黑名单回滚（commit 不再 excluded），下轮重算会原样重建该 need。
// 保护条件：必须有 commit（纯会话 need 不碰）、session_ids 为空、所有 commit 均被排除。
func cleanupEfficiencyV2FullyExcludedNeeds(db *gorm.DB) error {
	var candidates []models.Need
	if err := db.Where("jsonb_array_length(session_ids) = 0").
		Where("jsonb_array_length(commit_ids) > 0").
		Find(&candidates).Error; err != nil {
		return fmt.Errorf("query commit-only needs: %w", err)
	}
	if len(candidates) == 0 {
		return nil
	}
	commitIDs := make(map[string]bool)
	for _, need := range candidates {
		for _, id := range EfficiencyV2StringsFromJSON(need.CommitIds) {
			commitIDs[id] = true
		}
	}
	var excludedIDs []string
	if err := db.Model(&models.Commit{}).
		Where("commit_id IN ? AND excluded_flag = true", efficiencyV2SortedMapKeys(commitIDs)).
		Pluck("commit_id", &excludedIDs).Error; err != nil {
		return fmt.Errorf("query excluded commits: %w", err)
	}
	excludedSet := make(map[string]bool, len(excludedIDs))
	for _, id := range excludedIDs {
		excludedSet[id] = true
	}
	staleIDs := selectFullyExcludedNeedIDs(candidates, excludedSet)
	if len(staleIDs) == 0 {
		return nil
	}
	if err := db.Where("need_id IN ?", staleIDs).Delete(&models.Need{}).Error; err != nil {
		return fmt.Errorf("delete fully-excluded needs: %w", err)
	}
	return nil
}

// selectFullyExcludedNeedIDs 从候选 need 中挑出 commit 全部在 excludedSet 里的，返回排序后的 need_id。
func selectFullyExcludedNeedIDs(candidates []models.Need, excludedSet map[string]bool) []string {
	var staleIDs []string
	for _, need := range candidates {
		ids := EfficiencyV2StringsFromJSON(need.CommitIds)
		if len(ids) == 0 {
			continue
		}
		allExcluded := true
		for _, id := range ids {
			if !excludedSet[id] {
				allExcluded = false
				break
			}
		}
		if allExcluded {
			staleIDs = append(staleIDs, need.NeedId)
		}
	}
	sort.Strings(staleIDs)
	return staleIDs
}

// cleanupEfficiencyV2PreCanonNeeds 清理 repo 地址归一前残留的旧 need 行。
// 背景：branch 边界 key 内嵌 repo_addr（branch:{repo}:{branch}），开启 repo_addr_canon 后
// 同一边界的 key 变成归一写法，needs 表的 (boundary_source, boundary_key) 唯一索引
// 不会覆盖旧 key 行，留下 canon 前的旧行与新行重复统计同一边界。
// 保护条件（全部满足才删，缺一不可，防止误删正常 need）：
//  1. boundary_source 相同且仅限 lv2_branch——其他 source 的 key 不含 repo_addr，不受归一影响；
//  2. repo_branch 与新 need 相同——同仓库不同分支是不同 need，绝不能互删；
//  3. 旧行 repo_addr 经 Canon 后重构出的边界 key == 本轮某个新 need 的 boundary_key
//     （即 canon 后 repo_addr 相同、boundary_key 不同的 pre-canon 旧写法行），
//     且旧行自身 key 不等于该 canon key——新行本轮已聚合落库，删除旧行不丢数据。
//
// 采用 DELETE 而非 UPDATE 旧行 key 迁移：canon key 的新行本轮已存在，UPDATE 必撞唯一索引；
// 旧行的会话/commit 归属已被新行重新聚合吸收，删除幂等且无信息损失。
func cleanupEfficiencyV2PreCanonNeeds(db *gorm.DB, needs []models.Need) error {
	newKeys := make(map[string]bool)
	branches := make(map[string]bool)
	for _, need := range needs {
		if need.BoundarySource != efficiencyV2BoundaryBranch {
			continue
		}
		newKeys[need.BoundaryKey] = true
		if need.RepoBranch != "" {
			branches[need.RepoBranch] = true
		}
	}
	if len(newKeys) == 0 {
		return nil
	}
	var existing []models.Need
	if err := db.Where("boundary_source = ?", efficiencyV2BoundaryBranch).
		Where("repo_branch IN ?", efficiencyV2SortedMapKeys(branches)).
		Find(&existing).Error; err != nil {
		return fmt.Errorf("query pre-canon needs: %w", err)
	}
	var staleIDs []string
	for _, old := range existing {
		canonKey := efficiencyV2ClampNeedKey(fmt.Sprintf("branch:%s:%s", governance.CanonRepoAddr(old.RepoAddr), old.RepoBranch))
		if canonKey == old.BoundaryKey {
			continue // 已是 canon 写法（含本轮新行自身），不动
		}
		if !newKeys[canonKey] {
			continue // 本轮窗口没有对应的 canon 新行，保守保留
		}
		staleIDs = append(staleIDs, old.NeedId)
	}
	if len(staleIDs) == 0 {
		return nil
	}
	sort.Strings(staleIDs)
	if err := db.Where("need_id IN ?", staleIDs).Delete(&models.Need{}).Error; err != nil {
		return fmt.Errorf("delete pre-canon needs: %w", err)
	}
	return nil
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
	// repo 地址写法归一（治理配置 normalization.repo_addr_canon）：候选统一在此 Canon 一次，
	// 下游所有用到 repoAddr 的点——branch 边界 key 构造（branch:{repo}:{branch}）、
	// session↔commit 按 repo+branch 配对的线索传播与分桶——都用归一地址，
	// 让 git@ 与 https、带不带 .git 等写法分裂的同一仓库合并成同一个 need。
	// conversations/commits 表的 repo_addr 原值不动（治理不改原始数据），仅派生侧归一。
	if cfg.RepoAddrCanon {
		for i := range candidates {
			candidates[i].repoAddr = governance.CanonRepoAddr(candidates[i].repoAddr)
		}
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

	needs := make([]models.Need, 0, len(bucketKeys))
	for _, key := range bucketKeys {
		needs = append(needs, efficiencyV2BuildNeed(*bucketsByKey[key], cfg))
	}
	return needs
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
	if candidate.prID != "" && efficiencyV2IsMergeCommitComment(commit.Comment) {
		mergeTs := commit.CommitTime
		candidate.mergeTs = &mergeTs
		candidate.mergeOnly = true
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
		return efficiencyV2BoundaryFileCluster, efficiencyV2ConfidenceLow, efficiencyV2FileClusterKey(candidate.userID, candidate.start, files)
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
	var mergeTs *time.Time

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
		if candidate.mergeTs != nil && (mergeTs == nil || candidate.mergeTs.After(*mergeTs)) {
			merge := *candidate.mergeTs
			mergeTs = &merge
		}
		if !candidate.mergeOnly && !candidate.start.IsZero() && (devStart.IsZero() || candidate.start.Before(devStart)) {
			devStart = candidate.start
		}
		if !candidate.mergeOnly && !candidate.end.IsZero() && (devEnd.IsZero() || candidate.end.After(devEnd)) {
			devEnd = candidate.end
		}
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

	evidence := map[string]interface{}{
		"source":          bucket.source,
		"key":             bucket.key,
		"commit_messages": EfficiencyV2SortedUnique(commitMessages),
		"span_exceeded":   spanExceeded,
	}
	if spanExceeded {
		evidence["max_need_span_days"] = cfg.MaxNeedSpanDays
	}

	needKey := efficiencyV2ClampNeedKey(bucket.key)
	need := models.Need{
		NeedId:             needKey,
		BoundarySource:     bucket.source,
		BoundaryConfidence: bucket.confidence,
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
		MergeTs:            mergeTs,
		CoverageEligible:   status == "merged" && (bucket.confidence == efficiencyV2ConfidenceHigh || bucket.confidence == efficiencyV2ConfidenceMedium),
	}
	if mergeTs != nil && !devEnd.IsZero() && mergeTs.After(devEnd) {
		need.WaitForReviewMin = mergeTs.Sub(devEnd).Minutes()
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
	if spanExceeded {
		need.Reason = fmt.Sprintf("need span exceeds max_need_span_days=%d", cfg.MaxNeedSpanDays)
	}
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

func efficiencyV2IsMergeCommitComment(text string) bool {
	normalized := strings.ToLower(strings.TrimSpace(text))
	return strings.Contains(normalized, "merge pull request") || strings.HasPrefix(normalized, "merge pr")
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
	return branch == "main" || branch == "master" || branch == "develop" || branch == "release" || strings.HasPrefix(branch, "release/")
}

// efficiencyV2ClampNeedKey 把 need 边界 key 截断到 100 字符以内，用作 need_id / boundary_key。
// 超长（如 file-cluster 把大量顶层目录拼接）时，保留可读前缀 + 全量 key 的短哈希后缀，
// 避免不同长 key 截断成同一前缀后发生主键碰撞、把不同 need 错误合并。
func efficiencyV2ClampNeedKey(key string) string {
	const maxRunes = 100
	runes := []rune(key)
	if len(runes) <= maxRunes {
		return key
	}
	sum := sha1.Sum([]byte(key))
	hash := hex.EncodeToString(sum[:])[:12]
	prefix := string(runes[:maxRunes-1-len(hash)]) // 87 runes + "~" + 12 hash = 100
	return prefix + "~" + hash
}

func efficiencyV2FileClusterKey(userID string, start time.Time, files []string) string {
	return fmt.Sprintf("cluster:%s:%s:%s", userID, efficiencyV2WeekKey(start), efficiencyV2ClusterSlug(files))
}

func efficiencyV2OrphanKey(userID string, start time.Time) string {
	return fmt.Sprintf("orphan:%s:%s", userID, efficiencyV2WeekKey(start))
}

func efficiencyV2WeekKey(ts time.Time) string {
	if ts.IsZero() {
		return "unknown"
	}
	year, week := ts.ISOWeek()
	return fmt.Sprintf("%04dw%02d", year, week)
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
