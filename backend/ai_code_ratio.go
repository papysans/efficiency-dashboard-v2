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
