package main

import (
	"encoding/json"
	"fmt"
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
	commitsQuery := db.Order("commit_time ASC").Order("commit_id ASC")
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
	return needs, nil
}

func updateEfficiencyV2StageMetricNeedIDs(db *gorm.DB, needs []models.Need) error {
	for _, need := range needs {
		sessionIDs := efficiencyV2StringsFromJSON(need.SessionIds)
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
	cfg = normalizeEfficiencyV2Config(cfg)
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
		candidate.files = append(candidate.files, efficiencyV2StringsFromJSON(event.TouchedFiles)...)
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
	files := efficiencyV2SortedUnique(candidate.files)
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
		"commit_messages": efficiencyV2SortedUnique(commitMessages),
		"span_exceeded":   spanExceeded,
	}
	if spanExceeded {
		evidence["max_need_span_days"] = cfg.MaxNeedSpanDays
	}

	need := models.Need{
		NeedId:             bucket.key,
		BoundarySource:     bucket.source,
		BoundaryConfidence: bucket.confidence,
		BoundaryKey:        bucket.key,
		BoundaryEvidence:   efficiencyV2NeedObjectJSON(evidence),
		Status:             status,
		RepoAddr:           repoAddr,
		RepoBranch:         branch,
		PrimaryUserId:      primaryUser,
		ContributorUserIds: efficiencyV2StringJSON(contributorIDs),
		SessionIds:         efficiencyV2StringJSON(efficiencyV2SortedMapKeys(sessions)),
		CommitIds:          efficiencyV2StringJSON(efficiencyV2SortedMapKeys(commits)),
		TouchedFiles:       efficiencyV2StringJSON(efficiencyV2SortedMapKeys(files)),
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
	files = efficiencyV2SortedUnique(files)
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
	parts = efficiencyV2SortedUnique(parts)
	if len(parts) == 0 {
		return "unknown"
	}
	return strings.Join(parts, "-")
}

func efficiencyV2StringJSON(values []string) models.StringJSON {
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

func efficiencyV2StringsFromJSON(value models.StringJSON) []string {
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

func efficiencyV2SortedUnique(values []string) []string {
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			seen[value] = true
		}
	}
	return efficiencyV2SortedMapKeys(seen)
}
