package main

import (
	"sort"
	"strings"
)

// ===================== Needs =====================

var needSortFields = []string{"devEndTs", "efficiencyRatio", "workEfficiencyRatio", "totalCalendarMin", "baselineCalendarMin"}

func buildNeedOrder(field, dir string) string {
	switch field {
	case "devEndTs":
		return "dev_end_ts " + dir + " NULLS LAST"
	case "efficiencyRatio":
		return "efficiency_ratio " + dir + " NULLS LAST"
	case "workEfficiencyRatio":
		return "work_efficiency_ratio " + dir + " NULLS LAST"
	case "totalCalendarMin":
		return "total_calendar_min " + dir + " NULLS LAST"
	case "baselineCalendarMin":
		return "baseline_calendar_min " + dir + " NULLS LAST"
	default:
		return "dev_end_ts DESC NULLS LAST"
	}
}

// ===================== Sessions =====================

var sessionSortFields = []string{
	"createTime",
}

func buildSessionOrder(field, dir string) string {
	switch field {
	case "createTime":
		return "create_time " + dir
	default:
		return "create_time DESC"
	}
}

// ===================== Commits =====================

var commitSortFields = []string{
	"commitTime", "diffLines", "cost", "silica",
	"efficiencyRatio", "commitAncientMinutes", "commitRealMinutes",
}

func buildCommitOrder(field, dir string) string {
	switch field {
	case "commitTime":
		return "commit_time " + dir
	case "diffLines":
		return "diff_lines " + dir
	case "cost":
		return "cost " + dir
	case "silica":
		return "silica " + dir
	case "commitAncientMinutes":
		return "commit_ancient_minutes " + dir
	case "commitRealMinutes":
		return "commit_real_minutes " + dir
	case "efficiencyRatio":
		return "commit_ancient_minutes / NULLIF(commit_real_minutes, 0) " + dir
	default:
		return "commit_time DESC"
	}
}

// ===================== Tasks =====================

var taskSortFields = []string{
	"startTime", "endTime", "cost", "diffLines",
	"taskAncientMinutes", "taskRealMinutes", "efficiencyRatio",
}

func buildTaskOrder(field, dir string) string {
	switch field {
	case "startTime":
		return "start_time " + dir
	case "endTime":
		return "end_time " + dir
	case "cost":
		return "cost " + dir
	case "diffLines":
		return "diff_lines " + dir
	case "taskAncientMinutes":
		return "task_ancient_minutes " + dir
	case "taskRealMinutes":
		return "task_real_minutes " + dir
	case "efficiencyRatio":
		return "task_ancient_minutes / NULLIF(task_real_minutes, 0) " + dir
	default:
		return "start_time DESC"
	}
}

// ===================== Orgs =====================

var orgSortFields = []string{
	"userCount", "totalTokens", "totalCost", "taskCount",
	"taskDiffLines", "taskEfficiencyRatio", "commitCount",
	"commitDiffLines", "commitEfficiencyRatio",
}

func sortOrgData(data []OrgDataItem, field, dir string) {
	if field == "" {
		return
	}
	asc := dir == "ASC"
	sort.Slice(data, func(i, j int) bool {
		a, b := &data[i], &data[j]
		switch field {
		case "userCount":
			if a.UserCount == b.UserCount {
				return strings.Compare(a.OrgName, b.OrgName) < 0
			}
			if asc {
				return a.UserCount < b.UserCount
			}
			return a.UserCount > b.UserCount
		case "totalTokens":
			if a.TotalTokens == b.TotalTokens {
				return strings.Compare(a.OrgName, b.OrgName) < 0
			}
			if asc {
				return a.TotalTokens < b.TotalTokens
			}
			return a.TotalTokens > b.TotalTokens
		case "totalCost":
			if a.TotalCost == b.TotalCost {
				return strings.Compare(a.OrgName, b.OrgName) < 0
			}
			if asc {
				return a.TotalCost < b.TotalCost
			}
			return a.TotalCost > b.TotalCost
		case "taskCount":
			if a.TaskCount == b.TaskCount {
				return strings.Compare(a.OrgName, b.OrgName) < 0
			}
			if asc {
				return a.TaskCount < b.TaskCount
			}
			return a.TaskCount > b.TaskCount
		case "taskDiffLines":
			if a.TaskDiffLines == b.TaskDiffLines {
				return strings.Compare(a.OrgName, b.OrgName) < 0
			}
			if asc {
				return a.TaskDiffLines < b.TaskDiffLines
			}
			return a.TaskDiffLines > b.TaskDiffLines
		case "taskEfficiencyRatio":
			if a.TaskEfficiencyRatio == b.TaskEfficiencyRatio {
				return strings.Compare(a.OrgName, b.OrgName) < 0
			}
			if asc {
				return a.TaskEfficiencyRatio < b.TaskEfficiencyRatio
			}
			return a.TaskEfficiencyRatio > b.TaskEfficiencyRatio
		case "commitCount":
			if a.CommitCount == b.CommitCount {
				return strings.Compare(a.OrgName, b.OrgName) < 0
			}
			if asc {
				return a.CommitCount < b.CommitCount
			}
			return a.CommitCount > b.CommitCount
		case "commitDiffLines":
			if a.CommitDiffLines == b.CommitDiffLines {
				return strings.Compare(a.OrgName, b.OrgName) < 0
			}
			if asc {
				return a.CommitDiffLines < b.CommitDiffLines
			}
			return a.CommitDiffLines > b.CommitDiffLines
		case "commitEfficiencyRatio":
			if a.CommitEfficiencyRatio == b.CommitEfficiencyRatio {
				return strings.Compare(a.OrgName, b.OrgName) < 0
			}
			if asc {
				return a.CommitEfficiencyRatio < b.CommitEfficiencyRatio
			}
			return a.CommitEfficiencyRatio > b.CommitEfficiencyRatio
		default:
			return strings.Compare(a.OrgName, b.OrgName) < 0
		}
	})
}

// ===================== Projects =====================

var projectSortFields = []string{
	"cost", "projectAncientMinutes", "projectRealProcessMinutes",
	"repoCount", "taskCount", "userCount", "totalCodeLines",
	"actualLinesPerDay", "efficiencyRatio",
}

func sortProjectData(data []ProjectListItem, field, dir string) {
	if field == "" {
		return
	}
	asc := dir == "ASC"
	sort.Slice(data, func(i, j int) bool {
		a, b := &data[i], &data[j]
		switch field {
		case "cost":
			va, vb := 0.0, 0.0
			if a.Cost != nil {
				va = *a.Cost
			}
			if b.Cost != nil {
				vb = *b.Cost
			}
			if va == vb {
				return strings.Compare(a.Name, b.Name) < 0
			}
			if asc {
				return va < vb
			}
			return va > vb
		case "projectAncientMinutes":
			va, vb := 0.0, 0.0
			if a.ProjectAncientMinutes != nil {
				va = *a.ProjectAncientMinutes
			}
			if b.ProjectAncientMinutes != nil {
				vb = *b.ProjectAncientMinutes
			}
			if va == vb {
				return strings.Compare(a.Name, b.Name) < 0
			}
			if asc {
				return va < vb
			}
			return va > vb
		case "projectRealProcessMinutes":
			va, vb := 0.0, 0.0
			if a.ProjectRealProcessMinutes != nil {
				va = *a.ProjectRealProcessMinutes
			}
			if b.ProjectRealProcessMinutes != nil {
				vb = *b.ProjectRealProcessMinutes
			}
			if va == vb {
				return strings.Compare(a.Name, b.Name) < 0
			}
			if asc {
				return va < vb
			}
			return va > vb
		case "repoCount":
			if a.RepoCount == b.RepoCount {
				return strings.Compare(a.Name, b.Name) < 0
			}
			if asc {
				return a.RepoCount < b.RepoCount
			}
			return a.RepoCount > b.RepoCount
		case "taskCount":
			if a.TaskCount == b.TaskCount {
				return strings.Compare(a.Name, b.Name) < 0
			}
			if asc {
				return a.TaskCount < b.TaskCount
			}
			return a.TaskCount > b.TaskCount
		case "userCount":
			if a.UserCount == b.UserCount {
				return strings.Compare(a.Name, b.Name) < 0
			}
			if asc {
				return a.UserCount < b.UserCount
			}
			return a.UserCount > b.UserCount
		case "totalCodeLines":
			if a.TotalCodeLines == b.TotalCodeLines {
				return strings.Compare(a.Name, b.Name) < 0
			}
			if asc {
				return a.TotalCodeLines < b.TotalCodeLines
			}
			return a.TotalCodeLines > b.TotalCodeLines
		case "actualLinesPerDay":
			va, vb := 0.0, 0.0
			if a.ActualLinesPerDay != nil {
				va = *a.ActualLinesPerDay
			}
			if b.ActualLinesPerDay != nil {
				vb = *b.ActualLinesPerDay
			}
			if va == vb {
				return strings.Compare(a.Name, b.Name) < 0
			}
			if asc {
				return va < vb
			}
			return va > vb
		case "efficiencyRatio":
			va, vb := 0.0, 0.0
			if a.EfficiencyRatio != nil {
				va = *a.EfficiencyRatio
			}
			if b.EfficiencyRatio != nil {
				vb = *b.EfficiencyRatio
			}
			if va == vb {
				return strings.Compare(a.Name, b.Name) < 0
			}
			if asc {
				return va < vb
			}
			return va > vb
		default:
			return strings.Compare(a.Name, b.Name) < 0
		}
	})
}

// ===================== Repos =====================

var repoSortFields = []string{
	"commitCount", "startTime", "endTime",
	"sumAncientMinutes", "sumRealMinutes", "taskCount", "efficiencyRatio",
}

func sortRepoData(data []RepoListItem, field, dir string) {
	if field == "" {
		return
	}
	asc := dir == "ASC"
	sort.Slice(data, func(i, j int) bool {
		a, b := &data[i], &data[j]
		switch field {
		case "commitCount":
			if a.CommitCount == b.CommitCount {
				return strings.Compare(a.RepoAddr, b.RepoAddr) < 0
			}
			if asc {
				return a.CommitCount < b.CommitCount
			}
			return a.CommitCount > b.CommitCount
		case "startTime":
			cmp := strings.Compare(a.StartTime, b.StartTime)
			if cmp == 0 {
				return strings.Compare(a.RepoAddr, b.RepoAddr) < 0
			}
			if asc {
				return cmp < 0
			}
			return cmp > 0
		case "endTime":
			cmp := strings.Compare(a.EndTime, b.EndTime)
			if cmp == 0 {
				return strings.Compare(a.RepoAddr, b.RepoAddr) < 0
			}
			if asc {
				return cmp < 0
			}
			return cmp > 0
		case "sumAncientMinutes":
			va, vb := 0.0, 0.0
			va = a.SumAncientMinutes
			vb = b.SumAncientMinutes
			if va == vb {
				return strings.Compare(a.RepoAddr, b.RepoAddr) < 0
			}
			if asc {
				return va < vb
			}
			return va > vb
		case "sumRealMinutes":
			va, vb := 0.0, 0.0
			va = a.SumRealMinutes
			vb = b.SumRealMinutes
			if va == vb {
				return strings.Compare(a.RepoAddr, b.RepoAddr) < 0
			}
			if asc {
				return va < vb
			}
			return va > vb
		case "taskCount":
			if a.TaskCount == b.TaskCount {
				return strings.Compare(a.RepoAddr, b.RepoAddr) < 0
			}
			if asc {
				return a.TaskCount < b.TaskCount
			}
			return a.TaskCount > b.TaskCount
		case "efficiencyRatio":
			va, vb := 0.0, 0.0
			va = a.EfficiencyRatio
			vb = b.EfficiencyRatio
			if va == vb {
				return strings.Compare(a.RepoAddr, b.RepoAddr) < 0
			}
			if asc {
				return va < vb
			}
			return va > vb
		default:
			return strings.Compare(a.RepoAddr, b.RepoAddr) < 0
		}
	})
}

// ===================== Users =====================

var userSortFields = []string{
	"cost", "taskCount", "taskDiffLines", "taskRealMinutes",
	"taskAncientMinutes", "taskEfficiencyRatio", "commitCount",
	"commitDiffLines", "commitRealMinutes", "commitAncientMinutes",
	"commitEfficiencyRatio",
}

func sortUserData(data []UserListItem, field, dir string) {
	if field == "" {
		return
	}
	asc := dir == "ASC"
	sort.Slice(data, func(i, j int) bool {
		a, b := &data[i], &data[j]
		switch field {
		case "cost":
			if a.Cost == b.Cost {
				return strings.Compare(a.UserId, b.UserId) < 0
			}
			if asc {
				return a.Cost < b.Cost
			}
			return a.Cost > b.Cost
		case "taskCount":
			if a.TaskCount == b.TaskCount {
				return strings.Compare(a.UserId, b.UserId) < 0
			}
			if asc {
				return a.TaskCount < b.TaskCount
			}
			return a.TaskCount > b.TaskCount
		case "taskDiffLines":
			if a.TaskDiffLines == b.TaskDiffLines {
				return strings.Compare(a.UserId, b.UserId) < 0
			}
			if asc {
				return a.TaskDiffLines < b.TaskDiffLines
			}
			return a.TaskDiffLines > b.TaskDiffLines
		case "taskRealMinutes":
			if a.TaskRealMinutes == b.TaskRealMinutes {
				return strings.Compare(a.UserId, b.UserId) < 0
			}
			if asc {
				return a.TaskRealMinutes < b.TaskRealMinutes
			}
			return a.TaskRealMinutes > b.TaskRealMinutes
		case "taskAncientMinutes":
			if a.TaskAncientMinutes == b.TaskAncientMinutes {
				return strings.Compare(a.UserId, b.UserId) < 0
			}
			if asc {
				return a.TaskAncientMinutes < b.TaskAncientMinutes
			}
			return a.TaskAncientMinutes > b.TaskAncientMinutes
		case "taskEfficiencyRatio":
			if a.TaskEfficiencyRatio == b.TaskEfficiencyRatio {
				return strings.Compare(a.UserId, b.UserId) < 0
			}
			if asc {
				return a.TaskEfficiencyRatio < b.TaskEfficiencyRatio
			}
			return a.TaskEfficiencyRatio > b.TaskEfficiencyRatio
		case "commitCount":
			if a.CommitCount == b.CommitCount {
				return strings.Compare(a.UserId, b.UserId) < 0
			}
			if asc {
				return a.CommitCount < b.CommitCount
			}
			return a.CommitCount > b.CommitCount
		case "commitDiffLines":
			if a.CommitDiffLines == b.CommitDiffLines {
				return strings.Compare(a.UserId, b.UserId) < 0
			}
			if asc {
				return a.CommitDiffLines < b.CommitDiffLines
			}
			return a.CommitDiffLines > b.CommitDiffLines
		case "commitRealMinutes":
			if a.CommitRealMinutes == b.CommitRealMinutes {
				return strings.Compare(a.UserId, b.UserId) < 0
			}
			if asc {
				return a.CommitRealMinutes < b.CommitRealMinutes
			}
			return a.CommitRealMinutes > b.CommitRealMinutes
		case "commitAncientMinutes":
			if a.CommitAncientMinutes == b.CommitAncientMinutes {
				return strings.Compare(a.UserId, b.UserId) < 0
			}
			if asc {
				return a.CommitAncientMinutes < b.CommitAncientMinutes
			}
			return a.CommitAncientMinutes > b.CommitAncientMinutes
		case "commitEfficiencyRatio":
			if a.CommitEfficiencyRatio == b.CommitEfficiencyRatio {
				return strings.Compare(a.UserId, b.UserId) < 0
			}
			if asc {
				return a.CommitEfficiencyRatio < b.CommitEfficiencyRatio
			}
			return a.CommitEfficiencyRatio > b.CommitEfficiencyRatio
		default:
			return strings.Compare(a.UserId, b.UserId) < 0
		}
	})
}
