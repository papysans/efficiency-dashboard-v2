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

func needAICodeAggSelect() string {
	return `COALESCE(SUM(ai_covered_loc) FILTER (WHERE coverage_eligible AND NOT outlier_flag AND total_loc_net > 0), 0) AS ai_covered_loc,
		COALESCE(SUM(total_loc_net) FILTER (WHERE coverage_eligible AND NOT outlier_flag AND total_loc_net > 0), 0) AS total_loc_net`
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
