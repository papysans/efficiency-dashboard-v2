package efficiencyv2

import (
	"fmt"
	"sort"
	"time"

	"kanban/core/models"

	"gorm.io/gorm"
)

const (
	efficiencyV2AttributionSolo     = "solo"
	efficiencyV2AttributionSplit    = "split"
	efficiencyV2AttributionResidual = "residual"
)

// ComputeEfficiencyV2NeedEmpAttribution 把单个 need 的交付物(commit LOC/AI)与努力(会话
// 人时)按稳定人身份工号(emp_no)拆成 (need × 工号) 归属行。是纯函数（无 DB），便于单测。
//
// 交付物：need 的 commits 按 committer 工号分组(empMap.EmpForCommit，仅 dept_user 在册的算
// 工号；不在册 / 空邮箱 → emp_no="" 的 residual 行)。每工号 loc_net=Σ GetEffectiveDiffLines、
// commit_count、ai_covered_loc=Σ 落会话窗口的 commit effective lines。
//
// 努力：need 的 sessions 按 empMap.EmpForUID(user_id) 分组(orphan / 共享账号 → "");每工号
// active_work_min = efficiencyV2NeedPersonMinutes(该工号的 sessions, cfg)，复用其按 user_id 的
// 并行会话去重。EmpForUID 为空的 session 计入 residual 行 active_work_min。
//
// 合并：同 need 内按 emp_no outer-join 交付物行与努力行(一个工号可能只有 commit 没 session、
// 或反之)。attribution_kind 按**实际产出的行集**逐行判(不用 need.EmpNoCount，它走 EmpForUID 而
// 行集走 EmpForCommit，两条身份路径不一致)：emp_no="" → residual；否则按非空 emp_no 行数 R，
// R<=1 → solo、R>=2 → split。LOC 守恒：Σ 所有行(含 residual) loc_net == Σ need 内 commit 的 GetEffectiveDiffLines。
func ComputeEfficiencyV2NeedEmpAttribution(need models.Need, commits []models.Commit, sessions []models.SessionStageMetric, empMap *EfficiencyV2UserEmpMap, cfg EfficiencyV2Config) []models.NeedEmpAttribution {
	cfg = NormalizeEfficiencyV2Config(cfg)

	// 交付物侧：按 committer 工号分组累加 LOC / commit / ai 覆盖行。
	windows := efficiencyV2NeedCoverageWindows(sessions, cfg)
	type deliverAccum struct {
		commitCount  int64
		locNet       int64
		aiCoveredLoc int64
	}
	deliverByEmp := make(map[string]*deliverAccum)
	for _, commit := range commits {
		emp := empMap.EmpForCommit(commit.GitUserEmail)
		acc := deliverByEmp[emp]
		if acc == nil {
			acc = &deliverAccum{}
			deliverByEmp[emp] = acc
		}
		lines := commit.GetEffectiveDiffLines()
		acc.commitCount++
		acc.locNet += lines
		if isEfficiencyV2CommitCovered(commit, windows) {
			acc.aiCoveredLoc += lines
		}
	}

	// 努力侧：按 user_id 映射到工号分组，组内复用 need 级并行会话去重算人时。
	sessionsByEmp := make(map[string][]models.SessionStageMetric)
	for _, s := range sessions {
		emp := empMap.EmpForUID(s.UserId)
		sessionsByEmp[emp] = append(sessionsByEmp[emp], s)
	}
	effortByEmp := make(map[string]float64, len(sessionsByEmp))
	for emp, group := range sessionsByEmp {
		effortByEmp[emp] = efficiencyV2NeedPersonMinutes(group, cfg)
	}

	// outer-join：交付物与努力的工号并集。
	empSet := make(map[string]bool, len(deliverByEmp)+len(effortByEmp))
	for emp := range deliverByEmp {
		empSet[emp] = true
	}
	for emp := range effortByEmp {
		empSet[emp] = true
	}

	emps := make([]string, 0, len(empSet))
	for emp := range empSet {
		emps = append(emps, emp)
	}
	sort.Strings(emps) // 固定行顺序，保证确定性

	// attribution_kind 必须按**实际产出的行集**逐行判，不能用 need.EmpNoCount——后者按
	// EmpForUID(user_id) 算，而本行集按 EmpForCommit(git_user_email) 算，两条身份路径不一致：
	//   (a) 单工号 need 里非在册/空邮箱的 residual 行（emp_no=""）会被误盖 solo，应是 residual；
	//   (b) 共享账号 committer：EmpForUID 返回 "" 但 EmpForCommit 返回有效在册工号，故
	//       EmpNoCount==1 的 need 可能产出 ≥2 个在册工号行（设计身份模型的承重 case），应 split。
	// 口径：emp_no=="" → residual；否则按非空 emp_no 行数 R，R<=1 → solo、R>=2 → split。
	registeredEmpCount := 0
	for _, emp := range emps {
		if emp != "" {
			registeredEmpCount++
		}
	}

	rows := make([]models.NeedEmpAttribution, 0, len(emps))
	for _, emp := range emps {
		var kind string
		switch {
		case emp == "":
			kind = efficiencyV2AttributionResidual
		case registeredEmpCount >= 2:
			kind = efficiencyV2AttributionSplit
		default:
			kind = efficiencyV2AttributionSolo
		}
		row := models.NeedEmpAttribution{
			NeedId:          need.NeedId,
			EmpNo:           emp,
			ActiveWorkMin:   effortByEmp[emp],
			AttributionKind: kind,
		}
		if acc := deliverByEmp[emp]; acc != nil {
			row.CommitCount = acc.commitCount
			row.LocNet = acc.locNet
			row.AICoveredLoc = acc.aiCoveredLoc
		}
		rows = append(rows, row)
	}
	return rows
}

// efficiencyV2NeedCoverageWindows 由 need 的会话起止 ±margin 构成的覆盖窗口集合，用于判定
// commit 是否落在 AI 会话窗内（算 ai_covered_loc）。口径与 aggregateEfficiencyV2NeedCommits 一致。
func efficiencyV2NeedCoverageWindows(sessions []models.SessionStageMetric, cfg EfficiencyV2Config) [][2]time.Time {
	preMargin := time.Duration(cfg.UncoveredCommit.PreMarginMinutes) * time.Minute
	postMargin := time.Duration(cfg.UncoveredCommit.PostMarginMinutes) * time.Minute
	windows := make([][2]time.Time, 0, len(sessions))
	for _, s := range sessions {
		if s.SessionStartTs == nil || s.SessionEndTs == nil {
			continue
		}
		windows = append(windows, [2]time.Time{
			s.SessionStartTs.Add(-preMargin),
			s.SessionEndTs.Add(postMargin),
		})
	}
	return windows
}

// AggregateAndUpsertEfficiencyV2NeedEmpAttribution 计算并落库一批 need 的 (need × 工号)
// 归属行。读取每 need 关联的 commits(治理排除的不计) 与 session_stage_metrics，按工号拆分后
// 对每个 need 先删旧行再插新行(防陈旧)。empMap 在管线入口已加载，由调用方传入复用。
func AggregateAndUpsertEfficiencyV2NeedEmpAttribution(db *gorm.DB, needs []models.Need, empMap *EfficiencyV2UserEmpMap, cfg EfficiencyV2Config) (int, error) {
	if len(needs) == 0 {
		return 0, nil
	}
	cfg = NormalizeEfficiencyV2Config(cfg)

	sessionIDs := make(map[string]bool)
	commitIDs := make(map[string]bool)
	for _, need := range needs {
		for _, id := range EfficiencyV2StringsFromJSON(need.SessionIds) {
			sessionIDs[id] = true
		}
		for _, id := range EfficiencyV2StringsFromJSON(need.CommitIds) {
			commitIDs[id] = true
		}
	}

	var metrics []models.SessionStageMetric
	if len(sessionIDs) > 0 {
		keys := efficiencyV2SortedMapKeys(sessionIDs)
		if err := db.Where("session_id IN ?", keys).Find(&metrics).Error; err != nil {
			return 0, fmt.Errorf("load stage metrics: %w", err)
		}
	}
	var commits []models.Commit
	if len(commitIDs) > 0 {
		keys := efficiencyV2SortedMapKeys(commitIDs)
		// 治理排除的 commit 不进归属（GetEffectiveDiffLines 对 excluded 也返回 0，查询过滤是双保险）。
		if err := db.Where("commit_id IN ? AND excluded_flag = false", keys).Find(&commits).Error; err != nil {
			return 0, fmt.Errorf("load commits: %w", err)
		}
	}
	metricsBySession := make(map[string]models.SessionStageMetric, len(metrics))
	for _, m := range metrics {
		metricsBySession[m.SessionId] = m
	}
	commitsByID := make(map[string]models.Commit, len(commits))
	for _, c := range commits {
		commitsByID[c.CommitId] = c
	}

	written := 0
	for _, need := range needs {
		needSessions := make([]models.SessionStageMetric, 0)
		for _, id := range EfficiencyV2StringsFromJSON(need.SessionIds) {
			if m, ok := metricsBySession[id]; ok {
				needSessions = append(needSessions, m)
			}
		}
		needCommits := make([]models.Commit, 0)
		for _, id := range EfficiencyV2StringsFromJSON(need.CommitIds) {
			if c, ok := commitsByID[id]; ok {
				needCommits = append(needCommits, c)
			}
		}

		rows := ComputeEfficiencyV2NeedEmpAttribution(need, needCommits, needSessions, empMap, cfg)

		// 先删该 need 旧行再插新行：key 方案/工号集变化后旧行会悬挂。整体放事务保证原子。
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("need_id = ?", need.NeedId).Delete(&models.NeedEmpAttribution{}).Error; err != nil {
				return fmt.Errorf("prune old attribution rows: %w", err)
			}
			if len(rows) == 0 {
				return nil
			}
			if err := tx.CreateInBatches(&rows, 500).Error; err != nil {
				return fmt.Errorf("insert attribution rows: %w", err)
			}
			return nil
		}); err != nil {
			return written, fmt.Errorf("upsert need %s attribution: %w", need.NeedId, err)
		}
		written += len(rows)
	}

	// 清理悬挂行：need prune（key 换代）删过的旧 need_id 在本表残留的归属行。current 是
	// 本轮全量 need 集，不在其中的 need_id 一律删（幂等：第二次 stale 集为空）。
	if err := pruneEfficiencyV2StaleNeedEmpAttribution(db, needs); err != nil {
		return written, err
	}
	return written, nil
}

// pruneEfficiencyV2StaleNeedEmpAttribution 删除 need_emp_attribution 中 need_id 不在
// current need 集内的行（need prune 后的悬挂归属）。沿用 efficiencyV2StaleIDs 比对口径。
func pruneEfficiencyV2StaleNeedEmpAttribution(db *gorm.DB, current []models.Need) error {
	var existing []string
	if err := db.Model(&models.NeedEmpAttribution{}).Distinct("need_id").Pluck("need_id", &existing).Error; err != nil {
		return fmt.Errorf("list attribution need ids for prune: %w", err)
	}
	currentIDs := make([]string, 0, len(current))
	for _, need := range current {
		currentIDs = append(currentIDs, need.NeedId)
	}
	stale := efficiencyV2StaleIDs(existing, currentIDs)
	if len(stale) == 0 {
		return nil
	}
	const batch = 500
	for i := 0; i < len(stale); i += batch {
		end := i + batch
		if end > len(stale) {
			end = len(stale)
		}
		if err := db.Where("need_id IN ?", stale[i:end]).Delete(&models.NeedEmpAttribution{}).Error; err != nil {
			return fmt.Errorf("prune stale attribution rows: %w", err)
		}
	}
	return nil
}
