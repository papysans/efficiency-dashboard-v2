package efficiencyv2

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"kanban/core/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AggregateAndUpsertEfficiencyV2UserProductivity computes user-week v2
// aggregates from the persisted Needs and upserts user_productivity_v2 rows.
// It returns the number of weekly rows upserted.
//
// startDate/endDate are YYYY-MM-DD strings; empty means "no bound". When set,
// only Needs whose dev window overlaps the range are loaded, so reruns on a
// narrow window do not touch other weeks.
func AggregateAndUpsertEfficiencyV2UserProductivity(db *gorm.DB, cfg EfficiencyV2Config, startDate, endDate string) (int, error) {
	rows, err := aggregateAndUpsertEfficiencyV2UserProductivityRows(db, cfg, startDate, endDate)
	return len(rows), err
}

// RebuildEfficiencyV2UserProductivityAll 全量（无日期窗）重写 user_productivity_v2，
// 并删除本轮未重新生成的周行。仅在 need prune 删除过旧行（key 换代）后调用：
// 窗口化重算只重写窗口内的周，窗口外历史周的 need_ids 会悬挂引用已删 need；
// 周锚点（dev_end_ts 所在周）也可能随 episode 切分漂移，留下整行过期的旧周行。
// 幂等：need prune=0 的后续重算走窗口化路径，不会触发本函数。
func RebuildEfficiencyV2UserProductivityAll(db *gorm.DB, cfg EfficiencyV2Config) (int, error) {
	rows, err := aggregateAndUpsertEfficiencyV2UserProductivityRows(db, cfg, "", "")
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		// 与 need prune 同样的安全护栏：空轮不清表。
		return 0, nil
	}
	currentIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		currentIDs = append(currentIDs, row.UserProductivityV2Id)
	}
	var existing []string
	if err := db.Model(&models.UserProductivityV2{}).Pluck("user_productivity_v2_id", &existing).Error; err != nil {
		return 0, fmt.Errorf("list user_productivity_v2 ids for prune: %w", err)
	}
	stale := efficiencyV2StaleIDs(existing, currentIDs)
	if len(stale) > 0 {
		const batch = 500
		for i := 0; i < len(stale); i += batch {
			end := i + batch
			if end > len(stale) {
				end = len(stale)
			}
			if err := db.Where("user_productivity_v2_id IN ?", stale[i:end]).Delete(&models.UserProductivityV2{}).Error; err != nil {
				return 0, fmt.Errorf("prune stale user_productivity_v2 rows: %w", err)
			}
		}
	}
	log.Printf("efficiency-v2 prune: user_productivity_v2 full rebuild wrote %d rows, deleted %d stale rows", len(rows), len(stale))
	return len(rows), nil
}

func aggregateAndUpsertEfficiencyV2UserProductivityRows(db *gorm.DB, cfg EfficiencyV2Config, startDate, endDate string) ([]models.UserProductivityV2, error) {
	cfg = NormalizeEfficiencyV2Config(cfg)
	q := db.Model(&models.Need{}).Order("primary_user_id ASC")
	if startDate != "" {
		q = q.Where("dev_end_ts >= ? OR dev_start_ts >= ?", startDate, startDate)
	}
	if endDate != "" {
		end := endDate + " 23:59:59"
		q = q.Where("dev_start_ts <= ? OR dev_end_ts <= ?", end, end)
	}
	var needs []models.Need
	if err := q.Find(&needs).Error; err != nil {
		return nil, fmt.Errorf("load needs: %w", err)
	}
	rows := AggregateEfficiencyV2UserProductivity(needs, cfg)
	if len(rows) == 0 {
		return nil, nil
	}
	// 与老 user_productivity 对齐：补 token / cost / commit 用量字段。
	// 通过 Need 反查（Need 已经有 PrimaryUserId + 周锚点 + session_ids + commit_ids），
	// 比直接 join sessions 表更稳（sessions 表在 v2 路径下可能未被填充）。
	usage, err := rollupEfficiencyV2UsageFromNeeds(db, needs)
	if err != nil {
		return nil, fmt.Errorf("rollup usage from needs: %w", err)
	}
	for i := range rows {
		key := efficiencyV2UserWeekKey{UserID: rows[i].UserId, WeekStart: rows[i].WeekStart}
		if u, ok := usage[key]; ok {
			rows[i].UpstreamTokens = u.UpstreamTokens
			rows[i].DownstreamTokens = u.DownstreamTokens
			rows[i].Cost = u.Cost
			rows[i].CommitCount = u.CommitCount
			rows[i].CommitDiffLines = u.CommitDiffLines
		}
	}
	if err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "week_start"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"user_name", "need_ids",
			"merged_need_count", "active_need_count", "abandoned_need_count",
			"actual_calendar_min", "baseline_calendar_min", "efficiency_ratio",
			"actual_active_work_corrected_min", "baseline_fused_work_min", "work_efficiency_ratio",
			"coverage_high_confidence", "coverage_medium", "coverage_low_unreported",
			"coverage_abandoned", "coverage_active",
			"confidence_limited", "confidence_reason",
			"upstream_tokens", "downstream_tokens", "cost",
			"commit_count", "commit_diff_lines",
			"updated_at",
		}),
	}).Create(&rows).Error; err != nil {
		return nil, fmt.Errorf("upsert user_productivity_v2: %w", err)
	}
	return rows, nil
}

// AggregateEfficiencyV2UserProductivity buckets Needs by (user, week) and
// produces deterministic user-week aggregate rows.
func AggregateEfficiencyV2UserProductivity(needs []models.Need, cfg EfficiencyV2Config) []models.UserProductivityV2 {
	cfg = NormalizeEfficiencyV2Config(cfg)
	type bucketKey struct {
		userID    string
		weekStart time.Time
	}
	type bucket struct {
		needIDs []string
		needs   []models.Need
	}
	buckets := map[bucketKey]*bucket{}
	for _, need := range needs {
		anchor := efficiencyV2WeekAnchorForNeed(need)
		key := bucketKey{userID: need.PrimaryUserId, weekStart: anchor}
		b := buckets[key]
		if b == nil {
			b = &bucket{}
			buckets[key] = b
		}
		b.needIDs = append(b.needIDs, need.NeedId)
		b.needs = append(b.needs, need)
	}

	rows := make([]models.UserProductivityV2, 0, len(buckets))
	for key, b := range buckets {
		if key.userID == "" {
			continue
		}
		row := buildEfficiencyV2UserWeekRow(key.userID, key.weekStart, b.needs, cfg)
		row.NeedIds = EfficiencyV2StringJSON(sortedStrings(b.needIDs))
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].UserId != rows[j].UserId {
			return rows[i].UserId < rows[j].UserId
		}
		return rows[i].WeekStart.Before(rows[j].WeekStart)
	})
	return rows
}

func buildEfficiencyV2UserWeekRow(userID string, weekStart time.Time, needs []models.Need, cfg EfficiencyV2Config) models.UserProductivityV2 {
	row := models.UserProductivityV2{
		UserProductivityV2Id: efficiencyV2UserWeekID(userID, weekStart),
		UserId:               userID,
		WeekStart:            weekStart,
	}
	var (
		merged, active, abandoned                           int64
		actualCalSum, baseCalSum                            float64
		actualWorkSum, baseWorkSum                          float64
		coverageActive, coverageAbandoned                   float64
		coverageLowUnreported, coverageHigh, coverageMedium float64
		userName                                            string
	)
	for _, need := range needs {
		if userName == "" && need.PrimaryUserId == userID {
			userName = need.PrimaryUserId
		}
		switch strings.ToLower(strings.TrimSpace(need.Status)) {
		case "merged":
			merged++
		case "abandoned":
			abandoned++
		default:
			active++
		}

		// 工作量覆盖度分桶（仅展示 + 置信度判定用，与提效比口径无关）。
		switch need.Status {
		case "merged":
			switch need.BoundaryConfidence {
			case efficiencyV2ConfidenceHigh:
				coverageHigh += need.TotalActiveWorkCorrectedMin
			case efficiencyV2ConfidenceMedium:
				coverageMedium += need.TotalActiveWorkCorrectedMin
			default:
				coverageLowUnreported += need.TotalActiveWorkCorrectedMin
			}
		case "abandoned":
			coverageAbandoned += need.TotalActiveWorkCorrectedMin
		default:
			coverageActive += need.TotalActiveWorkCorrectedMin
		}

		// 提效比累加口径必须与 dashboard 完全一致：coverage_eligible(merged+高/中置信
		// +有可测日历) 且非 outlier。按口径分别判 outlier：日历提效只看 calendar_outlier_flag、
		// 工作量提效只看 work_outlier_flag，避免单口径极端值(如 actual_to_baseline)把同一 need
		// 合理的日历提效一并隐藏(详见 06-06-outlier-3000-need/design.md §3)。
		if need.CoverageEligible {
			if !need.CalendarOutlierFlag {
				actualCalSum += need.TotalCalendarMin
				if need.BaselineCalendarMin != nil {
					baseCalSum += *need.BaselineCalendarMin
				}
			}
			if !need.WorkOutlierFlag {
				actualWorkSum += need.TotalActiveWorkCorrectedMin
				if need.BaselineFusedWorkMin != nil {
					baseWorkSum += *need.BaselineFusedWorkMin
				}
			}
		}
	}
	row.UserName = userName
	row.MergedNeedCount = merged
	row.ActiveNeedCount = active
	row.AbandonedNeedCount = abandoned

	row.ActualCalendarMin = actualCalSum
	row.BaselineCalendarMin = baseCalSum
	if baseCalSum > 0 && actualCalSum > 0 {
		ratio := (baseCalSum - actualCalSum) / actualCalSum
		row.EfficiencyRatio = &ratio
	}
	row.ActualActiveWorkCorrectedMin = actualWorkSum
	row.BaselineFusedWorkMin = baseWorkSum
	if baseWorkSum > 0 && actualWorkSum > 0 {
		wer := (baseWorkSum - actualWorkSum) / actualWorkSum
		row.WorkEfficiencyRatio = &wer
	}
	row.CoverageHigh = coverageHigh
	row.CoverageMedium = coverageMedium
	row.CoverageLowUnreported = coverageLowUnreported
	row.CoverageAbandoned = coverageAbandoned
	row.CoverageActive = coverageActive

	totalCoverage := coverageHigh + coverageMedium + coverageLowUnreported + coverageAbandoned + coverageActive
	confidenceLimited := false
	reasons := []string{}
	if totalCoverage > 0 {
		lowRatio := (coverageLowUnreported + coverageAbandoned) / totalCoverage
		if lowRatio > 0.5 {
			confidenceLimited = true
			reasons = append(reasons, fmt.Sprintf("low_unreported_ratio=%.3f", lowRatio))
		}
		highRatio := coverageHigh / totalCoverage
		if highRatio < 0.1 {
			confidenceLimited = true
			reasons = append(reasons, fmt.Sprintf("high_confidence_ratio=%.3f", highRatio))
		}
	}
	if baseCalSum == 0 {
		confidenceLimited = true
		reasons = append(reasons, "no_eligible_baseline")
	}
	row.ConfidenceLimited = confidenceLimited
	row.ConfidenceReason = strings.Join(reasons, "; ")
	return row
}

func efficiencyV2WeekAnchorForNeed(need models.Need) time.Time {
	anchor := time.Time{}
	if need.DevEndTs != nil {
		anchor = *need.DevEndTs
	} else if need.DevStartTs != nil {
		anchor = *need.DevStartTs
	}
	if anchor.IsZero() {
		anchor = time.Now().UTC()
	}
	return EfficiencyV2MondayAnchor(anchor)
}

func EfficiencyV2MondayAnchor(t time.Time) time.Time {
	weekStart := t.UTC()
	weekday := int(weekStart.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	weekStart = weekStart.AddDate(0, 0, -(weekday - 1))
	return time.Date(weekStart.Year(), weekStart.Month(), weekStart.Day(), 0, 0, 0, 0, time.UTC)
}

func efficiencyV2UserWeekID(userID string, weekStart time.Time) string {
	parts := userID + "|" + weekStart.UTC().Format("2006-01-02")
	sum := sha256.Sum256([]byte(parts))
	return "uw_" + hex.EncodeToString(sum[:12])
}

func sortedStrings(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}

type efficiencyV2UserWeekKey struct {
	UserID    string
	WeekStart time.Time
}

type efficiencyV2UserWeekUsage struct {
	UpstreamTokens   int64
	DownstreamTokens int64
	Cost             float64
	CommitCount      int64
	CommitDiffLines  int64
}

// rollupEfficiencyV2UsageFromNeeds 通过 Need 反查 tokens/cost/commits 用量。
// Need 已经有 PrimaryUserId + dev_end_ts(确定周锚点) + session_ids + commit_ids，
// 所以可以避开 sessions 表（v2 路径不一定填充它）直接聚合。
func rollupEfficiencyV2UsageFromNeeds(db *gorm.DB, needs []models.Need) (map[efficiencyV2UserWeekKey]efficiencyV2UserWeekUsage, error) {
	out := map[efficiencyV2UserWeekKey]efficiencyV2UserWeekUsage{}
	if len(needs) == 0 {
		return out, nil
	}

	// 收集所有 session_ids 和 commit_ids，去重一次性查
	sessionSet := map[string]bool{}
	commitSet := map[string]bool{}
	for _, n := range needs {
		for _, s := range EfficiencyV2StringsFromJSON(n.SessionIds) {
			sessionSet[s] = true
		}
		for _, c := range EfficiencyV2StringsFromJSON(n.CommitIds) {
			commitSet[c] = true
		}
	}

	// session_id → (upstream, downstream, cost)
	type convSum struct {
		SessionId        string
		UpstreamTokens   int64
		DownstreamTokens int64
		Cost             float64
	}
	convAgg := map[string]convSum{}
	if len(sessionSet) > 0 {
		sessionIDs := make([]string, 0, len(sessionSet))
		for s := range sessionSet {
			sessionIDs = append(sessionIDs, s)
		}
		var rows []convSum
		if err := db.Model(&models.Conversation{}).
			Select("session_id, COALESCE(SUM(upstream_tokens),0) AS upstream_tokens, "+
				"COALESCE(SUM(downstream_tokens),0) AS downstream_tokens, "+
				"COALESCE(SUM(cost),0) AS cost").
			Where("session_id IN ?", sessionIDs).
			Group("session_id").Find(&rows).Error; err != nil {
			return nil, fmt.Errorf("agg conversations by session: %w", err)
		}
		for _, r := range rows {
			convAgg[r.SessionId] = r
		}
	}

	// commit_id → diff_lines
	commitAgg := map[string]int64{}
	if len(commitSet) > 0 {
		commitIDs := make([]string, 0, len(commitSet))
		for c := range commitSet {
			commitIDs = append(commitIDs, c)
		}
		type commitRow struct {
			CommitId  string
			DiffLines int64
		}
		var rows []commitRow
		if err := db.Model(&models.Commit{}).
			Select("commit_id, diff_lines").
			Where("commit_id IN ?", commitIDs).Find(&rows).Error; err != nil {
			return nil, fmt.Errorf("agg commits: %w", err)
		}
		for _, r := range rows {
			commitAgg[r.CommitId] = r.DiffLines
		}
	}

	// 按 (user_id, week_start) bucket
	for _, n := range needs {
		if n.PrimaryUserId == "" {
			continue
		}
		key := efficiencyV2UserWeekKey{UserID: n.PrimaryUserId, WeekStart: efficiencyV2WeekAnchorForNeed(n)}
		u := out[key]
		for _, sid := range EfficiencyV2StringsFromJSON(n.SessionIds) {
			if cs, ok := convAgg[sid]; ok {
				u.UpstreamTokens += cs.UpstreamTokens
				u.DownstreamTokens += cs.DownstreamTokens
				u.Cost += cs.Cost
			}
		}
		for _, cid := range EfficiencyV2StringsFromJSON(n.CommitIds) {
			if dl, ok := commitAgg[cid]; ok {
				u.CommitCount++
				u.CommitDiffLines += dl
			}
		}
		out[key] = u
	}
	return out, nil
}
