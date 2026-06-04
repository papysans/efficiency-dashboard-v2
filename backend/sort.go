package main

import (
	"sort"
	"strings"
)

// ===================== Needs =====================

var needSortFields = []string{"devStartTs", "devEndTs", "efficiencyRatio", "workEfficiencyRatio", "totalCalendarMin", "baselineCalendarMin"}

func buildNeedOrder(field, dir string) string {
	switch field {
	case "devStartTs":
		return "dev_start_ts " + dir + " NULLS LAST"
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
		return "commit_time " + dir + " NULLS LAST"
	case "diffLines":
		return "diff_lines " + dir + " NULLS LAST"
	case "cost":
		return "cost " + dir + " NULLS LAST"
	case "silica":
		return "silica " + dir + " NULLS LAST"
	case "commitAncientMinutes":
		return "commit_ancient_minutes " + dir + " NULLS LAST"
	case "commitRealMinutes":
		return "commit_real_minutes " + dir + " NULLS LAST"
	case "efficiencyRatio":
		return "commit_ancient_minutes / NULLIF(commit_real_minutes, 0) " + dir + " NULLS LAST"
	default:
		return "commit_time DESC NULLS LAST"
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
		return "start_time " + dir + " NULLS LAST"
	case "endTime":
		return "end_time " + dir + " NULLS LAST"
	case "cost":
		return "cost " + dir + " NULLS LAST"
	case "diffLines":
		return "diff_lines " + dir + " NULLS LAST"
	case "taskAncientMinutes":
		return "task_ancient_minutes " + dir + " NULLS LAST"
	case "taskRealMinutes":
		return "task_real_minutes " + dir + " NULLS LAST"
	case "efficiencyRatio":
		return "task_ancient_minutes / NULLIF(task_real_minutes, 0) " + dir + " NULLS LAST"
	default:
		return "start_time DESC NULLS LAST"
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

// ===================== Sort helpers (nil/missing sinks to bottom) =====================

// lessFloatSink reports whether the i-th item should sort before the j-th when
// comparing a *float64 key. Missing (nil) values always sink to the bottom,
// regardless of sort direction; equal/both-missing values fall back to tiebreak.
// The bool return value is the sort.Slice "less" result; the second bool
// reports whether the comparison was decisive (false → callers should fall
// through to their tiebreak comparison).
func lessFloatSink(a, b *float64, asc bool) (less bool, decided bool) {
	am, bm := a == nil, b == nil
	if am || bm {
		if am && bm {
			return false, false // both missing → undecided, fall back to tiebreak
		}
		// exactly one missing → missing one sorts last (independent of direction)
		return bm, true // bm==true means b is missing → a(=i) comes first
	}
	if *a == *b {
		return false, false // equal → fall back to tiebreak
	}
	if asc {
		return *a < *b, true
	}
	return *a > *b, true
}

// lessStringSink compares string keys where empty string is treated as missing
// and sinks to the bottom regardless of direction. Returns (less, decided).
func lessStringSink(a, b string, asc bool) (less bool, decided bool) {
	am, bm := a == "", b == ""
	if am || bm {
		if am && bm {
			return false, false // both missing → fall back to tiebreak
		}
		return bm, true // missing one sorts last
	}
	cmp := strings.Compare(a, b)
	if cmp == 0 {
		return false, false
	}
	if asc {
		return cmp < 0, true
	}
	return cmp > 0, true
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
			if less, decided := lessFloatSink(a.Cost, b.Cost, asc); decided {
				return less
			}
			return strings.Compare(a.Name, b.Name) < 0
		case "projectAncientMinutes":
			if less, decided := lessFloatSink(a.ProjectAncientMinutes, b.ProjectAncientMinutes, asc); decided {
				return less
			}
			return strings.Compare(a.Name, b.Name) < 0
		case "projectRealProcessMinutes":
			if less, decided := lessFloatSink(a.ProjectRealProcessMinutes, b.ProjectRealProcessMinutes, asc); decided {
				return less
			}
			return strings.Compare(a.Name, b.Name) < 0
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
			if less, decided := lessFloatSink(a.ActualLinesPerDay, b.ActualLinesPerDay, asc); decided {
				return less
			}
			return strings.Compare(a.Name, b.Name) < 0
		case "efficiencyRatio":
			if less, decided := lessFloatSink(a.EfficiencyRatio, b.EfficiencyRatio, asc); decided {
				return less
			}
			return strings.Compare(a.Name, b.Name) < 0
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
			if less, decided := lessStringSink(a.StartTime, b.StartTime, asc); decided {
				return less
			}
			return strings.Compare(a.RepoAddr, b.RepoAddr) < 0
		case "endTime":
			if less, decided := lessStringSink(a.EndTime, b.EndTime, asc); decided {
				return less
			}
			return strings.Compare(a.RepoAddr, b.RepoAddr) < 0
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
