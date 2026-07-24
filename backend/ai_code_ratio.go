package main

import (
	"fmt"

	"kanban/core/models"

	"gorm.io/gorm"
)

type needAICodeAgg struct {
	AICoveredLoc int64 `gorm:"column:ai_covered_loc"`
	TotalLocNet  int64 `gorm:"column:total_loc_net"`
}

type userNeedAICodeAgg struct {
	UserId       string `gorm:"column:primary_user_id"`
	AICoveredLoc int64  `gorm:"column:ai_covered_loc"`
	TotalLocNet  int64  `gorm:"column:total_loc_net"`
}

type repoNeedAICodeAgg struct {
	RepoAddr     string `gorm:"column:repo_addr"`
	RepoBranch   string `gorm:"column:repo_branch"`
	AICoveredLoc int64  `gorm:"column:ai_covered_loc"`
	TotalLocNet  int64  `gorm:"column:total_loc_net"`
}

type repoNeedAICodeAggKey struct {
	RepoAddr   string
	RepoBranch string
}

func calcNeedAICodeRatio(aiCoveredLoc, totalLocNet int64) *float64 {
	if totalLocNet <= 0 {
		return nil
	}
	r := float64(aiCoveredLoc) / float64(totalLocNet)
	return &r
}

func calcRepoNeedAICodeRatio(aggs map[repoNeedAICodeAggKey]needAICodeAgg, repoAddr, repoBranch string) *float64 {
	agg, ok := aggs[repoNeedAICodeAggKey{RepoAddr: repoAddr, RepoBranch: repoBranch}]
	if !ok {
		return nil
	}
	return calcNeedAICodeRatio(agg.AICoveredLoc, agg.TotalLocNet)
}

// silicaAgg 是含硅量（指纹口径）的聚合中间量：分子 = Σ(silica × diff_lines) 即匹配行数，
// 分母 = Σ(diff_lines) 即总新增行数。
//
// ⚠️ 必须先还原成「匹配行 / 总行」再求比，不能对各 commit 的 silica 值直接求平均——
// 否则 3 行的小 commit 和 300 行的大 commit 权重相同，整体被拉偏（与 queryRepoNeedAICodeAggsByAddr
// 的跨分支 rollup 同理）。
//
// 权重用 diff_lines（原始新增行）而非 GetEffectiveDiffLines()：commit.silica 的分母在
// analyzeCommitSilica 里就是原始 diff 行数（cmd_import_repo.go「silica 分母 = 原始 diff 行数，
// 护栏不改口径」），用治理后行数加权会让分子分母口径错位。治理排除的 commit 直接在
// WHERE 里滤掉，不进聚合。
type silicaAgg struct {
	SilicaWeighted float64 `gorm:"column:silica_weighted"`
	SilicaWeight   int64   `gorm:"column:silica_weight"`
}

type userSilicaAgg struct {
	UserId         string  `gorm:"column:user_id"`
	SilicaWeighted float64 `gorm:"column:silica_weighted"`
	SilicaWeight   int64   `gorm:"column:silica_weight"`
}

// calcSilicaRatio 把聚合中间量还原成比值；无有效行数时返回 nil（前端渲染 '-'，
// 与「无数据」而非「真 0」区分开）。
func calcSilicaRatio(weighted float64, weight int64) *float64 {
	if weight <= 0 {
		return nil
	}
	r := weighted / float64(weight)
	return &r
}

// silicaAggSelect 是含硅量聚合的 SELECT 片段。
//
// 口径说明：含硅量是「AI 产出的代码行有多少落到了 commit 里」的原始度量，直接取自
// commits.silica（对话 diff 指纹 ∩ commit diff 指纹），**不经过 need 边界**。因此这里
// 不用 applyNeedCaliberFilter——那套过滤（status/主干分支/软件用户）是 Need 口径的门槛，
// 而含硅量在 commit 级就已成立。这也是它能覆盖「AI 会话在别分支/别仓库、need 配不上」
// 那部分工作的原因：silica 的指纹分组键是 (repo_addr, user_id) 与 (work_dir_id, user_id)，
// 不含分支。与 needAICodeAggSelect 解绑置信度门槛的思路一致。
func silicaAggSelect() string {
	return `COALESCE(SUM(silica * diff_lines), 0) AS silica_weighted,
		COALESCE(SUM(diff_lines), 0) AS silica_weight`
}

// applySilicaCommitFilter 是含硅量聚合的公共过滤：治理排除的 commit 不计，
// 无新增行的 commit 不计（避免 0 权重行混入，也防分母为 0）。
func applySilicaCommitFilter(q *gorm.DB) *gorm.DB {
	return q.Where("NOT excluded_flag").Where("diff_lines > 0")
}

// applySilicaDateFilter 按 commit_time 过滤。注意与 applyNeedAICodeDateFilter 的 dev_end_ts
// 不同：含硅量挂在 commit 上，没有 need 的开发周期概念，用提交时刻即可。
func applySilicaDateFilter(q *gorm.DB, startTime, endTime string) *gorm.DB {
	if startTime != "" {
		q = q.Where("commit_time >= ?", startTime)
	}
	if endTime != "" {
		q = q.Where("commit_time <= ?", endTime)
	}
	return q
}

// queryUserSilicaAggs 按 commits.user_id 聚合含硅量。
// userID 非空时只查该用户；返回 map 缺 key 表示该用户窗口内无可计入 commit（比值为 nil）。
func queryUserSilicaAggs(db *gorm.DB, startTime, endTime, userID string) (map[string]silicaAgg, error) {
	var rows []userSilicaAgg
	q := applySilicaCommitFilter(db.Model(&models.Commit{})).
		Select("user_id, " + silicaAggSelect()).
		Where("user_id <> ''")
	q = applySilicaDateFilter(q, startTime, endTime)
	if userID != "" {
		q = q.Where("user_id = ?", userID)
	}
	if err := q.Group("user_id").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("查询用户含硅量聚合失败: %w", err)
	}
	result := make(map[string]silicaAgg, len(rows))
	for _, row := range rows {
		result[row.UserId] = silicaAgg{SilicaWeighted: row.SilicaWeighted, SilicaWeight: row.SilicaWeight}
	}
	return result, nil
}

// AI 代码占比是「AI 覆盖行 / 总变更行」的原始度量，只依赖 commit 的有效行数（need_aggregate
// 里按 commit 直接汇总 ai_covered_loc/total_loc_net，与 boundary 置信度无关）。因此这里【不】用
// coverage_eligible 过滤——coverage_eligible = merged && 置信度(高/中)，那是提效比/基线是否可信的
// 门槛，会把只有裸 commit（Low 置信度、未匹配 PR/branch/issue）的 need 整条滤掉，导致这些成员
// AI 代码占比显示「-」。占比是事实数据，置信度高低无所谓（用户要求），故只保留 NOT outlier_flag
// （防极端 LOC dump 拉偏）与 total_loc_net > 0（防除零）。提效比/日历口径仍在各自 SUM 里按
// coverage_eligible 过滤，不受本改动影响。
func needAICodeAggSelect() string {
	return `COALESCE(SUM(ai_covered_loc) FILTER (WHERE NOT outlier_flag AND total_loc_net > 0), 0) AS ai_covered_loc,
		COALESCE(SUM(total_loc_net) FILTER (WHERE NOT outlier_flag AND total_loc_net > 0), 0) AS total_loc_net`
}

func applyNeedAICodeDateFilter(q *gorm.DB, startTime, endTime string) *gorm.DB {
	if startTime != "" {
		q = q.Where("dev_end_ts >= ?", startTime)
	}
	if endTime != "" {
		q = q.Where("dev_end_ts <= ?", endTime)
	}
	return q
}

func queryUserNeedAICodeAggs(db *gorm.DB, startTime, endTime, userID string) (map[string]needAICodeAgg, error) {
	var rows []userNeedAICodeAgg
	q := applyNeedCaliberFilter(db.Model(&models.Need{})).
		Select("primary_user_id, " + needAICodeAggSelect()).
		Where("primary_user_id <> ''")
	q = applyNeedAICodeDateFilter(q, startTime, endTime)
	if userID != "" {
		q = q.Where("primary_user_id = ?", userID)
	}
	if err := q.Group("primary_user_id").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("查询用户 Need AI 代码占比聚合失败: %w", err)
	}
	result := make(map[string]needAICodeAgg, len(rows))
	for _, row := range rows {
		result[row.UserId] = needAICodeAgg{AICoveredLoc: row.AICoveredLoc, TotalLocNet: row.TotalLocNet}
	}
	return result, nil
}

func queryRepoNeedAICodeAggs(db *gorm.DB, startTime, endTime string) (map[repoNeedAICodeAggKey]needAICodeAgg, error) {
	var rows []repoNeedAICodeAgg
	q := applyNeedCaliberFilter(db.Model(&models.Need{})).
		Select("repo_addr, repo_branch, " + needAICodeAggSelect()).
		Where("repo_addr <> ''")
	q = applyNeedAICodeDateFilter(q, startTime, endTime)
	if err := q.Group("repo_addr, repo_branch").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("查询仓库 Need AI 代码占比聚合失败: %w", err)
	}
	result := make(map[repoNeedAICodeAggKey]needAICodeAgg, len(rows))
	for _, row := range rows {
		key := repoNeedAICodeAggKey{RepoAddr: row.RepoAddr, RepoBranch: row.RepoBranch}
		result[key] = needAICodeAgg{AICoveredLoc: row.AICoveredLoc, TotalLocNet: row.TotalLocNet}
	}
	return result, nil
}

// queryRepoNeedAICodeAggsByAddr 按 repo_addr 聚合（跨全部分支合并）——仓库级 AI 代码占比用。
// ⚠️ 必须先对 covered/total LOC 求和再求比（calcNeedAICodeRatio），不能对各分支比值求平均，
// 否则小分支会把整仓占比拉偏。与 listReposV2 的跨分支 rollup 同口径（一仓一行）。
func queryRepoNeedAICodeAggsByAddr(db *gorm.DB, startTime, endTime string) (map[string]needAICodeAgg, error) {
	type addrRow struct {
		RepoAddr     string `gorm:"column:repo_addr"`
		AICoveredLoc int64  `gorm:"column:ai_covered_loc"`
		TotalLocNet  int64  `gorm:"column:total_loc_net"`
	}
	var rows []addrRow
	q := applyNeedCaliberFilter(db.Model(&models.Need{})).
		Select("repo_addr, " + needAICodeAggSelect()).
		Where("repo_addr <> ''")
	q = applyNeedAICodeDateFilter(q, startTime, endTime)
	if err := q.Group("repo_addr").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("查询仓库 Need AI 代码占比聚合(按仓库)失败: %w", err)
	}
	result := make(map[string]needAICodeAgg, len(rows))
	for _, row := range rows {
		result[row.RepoAddr] = needAICodeAgg{AICoveredLoc: row.AICoveredLoc, TotalLocNet: row.TotalLocNet}
	}
	return result, nil
}

// queryRepoNeedCostAggs 按 repo_addr 聚合看板派生费用：干净 Need→session 去重→tasks.cost 求和。
// 口径与项目侧 queryProjectNeedCost 一致（同一条 Need→session→task 链，按 session 去重避免跨 Need 重复计费）。
// ⚠️ 费用来源是 tasks.cost；archive/无 tasks 数据的库返回 0，生产库才有真实 ¥（非 bug，是数据完整度）。
func queryRepoNeedCostAggs(db *gorm.DB, startTime, endTime string) (map[string]float64, error) {
	type costRow struct {
		RepoAddr string  `gorm:"column:repo_addr"`
		Cost     float64 `gorm:"column:cost"`
	}
	// 子查询：干净 Need 的 (repo_addr, session_id) 去重对（一 session 被同仓多 Need 引用只计一次）。
	sub := applyNeedCaliberFilter(db.Model(&models.Need{})).
		Select("DISTINCT repo_addr, jsonb_array_elements_text(session_ids) AS sid").
		Where("repo_addr <> '' AND coverage_eligible AND NOT outlier_flag")
	sub = applyNeedAICodeDateFilter(sub, startTime, endTime)
	var rows []costRow
	if err := db.Table("(?) AS rs", sub).
		Select("rs.repo_addr, COALESCE(SUM(t.cost), 0) AS cost").
		Joins("JOIN tasks t ON t.session_id = rs.sid").
		Group("rs.repo_addr").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("查询仓库 Need 费用聚合失败: %w", err)
	}
	result := make(map[string]float64, len(rows))
	for _, row := range rows {
		result[row.RepoAddr] = row.Cost
	}
	return result, nil
}

func queryRepoNeedAICodeAgg(db *gorm.DB, startTime, endTime, repoAddr, repoBranch string) (*needAICodeAgg, error) {
	var agg needAICodeAgg
	q := applyNeedCaliberFilter(db.Model(&models.Need{})).
		Select(needAICodeAggSelect()).
		Where("repo_addr = ?", repoAddr)
	q = applyNeedAICodeDateFilter(q, startTime, endTime)
	if repoBranch != "" {
		q = q.Where("repo_branch = ?", repoBranch)
	}
	if err := q.Scan(&agg).Error; err != nil {
		return nil, fmt.Errorf("查询仓库详情 Need AI 代码占比聚合失败: %w", err)
	}
	return &agg, nil
}
