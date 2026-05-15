package main

import (
	"encoding/json"
	"fmt"
	"kanban/core/models"
	"kanban/core/utils"
	"sort"
	"time"

	"gorm.io/gorm"
)

type UserDetail struct {
	UserId                string  `json:"user_id"`
	UserName              string  `json:"user_name"`
	Org1                  string  `json:"org1"`
	Org2                  string  `json:"org2"`
	Org3                  string  `json:"org3"`
	Org4                  string  `json:"org4"`
	Org5                  string  `json:"org5"`
	Org6                  string  `json:"org6"`
	Org7                  string  `json:"org7"`
	Org8                  string  `json:"org8"`
	Org9                  string  `json:"org9"`
	OrgDisplay            string  `json:"org_display"`
	TaskDiffLines         int     `json:"task_diff_lines"`
	TaskRealMinutes       float64 `json:"task_real_minutes"`
	TaskAncientMinutes    float64 `json:"task_ancient_minutes"`
	TaskEfficiencyRatio   float64 `json:"task_efficiency_ratio"`
	CommitDiffLines       int     `json:"commit_diff_lines"`
	CommitRealMinutes     float64 `json:"commit_real_minutes"`
	CommitAncientMinutes  float64 `json:"commit_ancient_minutes"`
	CommitEfficiencyRatio float64 `json:"commit_efficiency_ratio"`
	UpstreamTokens        int64   `json:"upstream_tokens"`
	DownstreamTokens      int64   `json:"downstream_tokens"`
	Cost                  float64 `json:"cost"`
}

type PeriodStatistics struct {
	PeriodKey             string  `json:"period_key"`
	PeriodLabel           string  `json:"period_label"`
	CommitCount           int     `json:"commit_count"`
	CommitDiffLines       int     `json:"commit_diff_lines"`
	CommitRealMinutes     float64 `json:"commit_real_minutes"`
	CommitAncientMinutes  float64 `json:"commit_ancient_minutes"`
	CommitEfficiencyRatio float64 `json:"commit_efficiency_ratio"`
	TaskCount             int     `json:"task_count"`
	TaskDiffLines         int     `json:"task_diff_lines"`
	TaskRealMinutes       float64 `json:"task_real_minutes"`
	TaskAncientMinutes    float64 `json:"task_ancient_minutes"`
	TaskEfficiencyRatio   float64 `json:"task_efficiency_ratio"`
	UpstreamTokens        int64   `json:"upstream_tokens"`
	DownstreamTokens      int64   `json:"downstream_tokens"`
	Cost                  float64 `json:"cost"`
}

type CommitTimeSeriesItem struct {
	PeriodKey             string  `json:"period_key"`
	PeriodLabel           string  `json:"period_label"`
	CommitCount           int     `json:"commit_count"`
	CommitDiffLines       int     `json:"commit_diff_lines"`
	CommitRealMinutes     float64 `json:"commit_real_minutes"`
	CommitAncientMinutes  float64 `json:"commit_ancient_minutes"`
	CommitEfficiencyRatio float64 `json:"commit_efficiency_ratio"`
	UpstreamTokens        int64   `json:"upstream_tokens"`
	DownstreamTokens      int64   `json:"downstream_tokens"`
	Cost                  float64 `json:"cost"`
}

type TaskTimeSeriesItem struct {
	PeriodKey           string  `json:"period_key"`
	PeriodLabel         string  `json:"period_label"`
	TaskCount           int     `json:"task_count"`
	TaskDiffLines       int     `json:"task_diff_lines"`
	TaskRealMinutes     float64 `json:"task_real_minutes"`
	TaskAncientMinutes  float64 `json:"task_ancient_minutes"`
	TaskEfficiencyRatio float64 `json:"task_efficiency_ratio"`
	UpstreamTokens      int64   `json:"upstream_tokens"`
	DownstreamTokens    int64   `json:"downstream_tokens"`
	Cost                float64 `json:"cost"`
}

// GetUsersProductivity 根据用户列表查询用户生产力数据，并构建 UserDetail 数组
func GetUsersProductivity(db *gorm.DB, matchedUsers []*models.UserOrg, startTime, endTime string) []UserDetail {
	userIDs := make([]string, 0, len(matchedUsers))
	for _, u := range matchedUsers {
		userIDs = append(userIDs, u.UserId)
	}
	return GetProductivityByIds(db, userIDs, startTime, endTime)
}

// GetProductivityByIds 根据用户列表查询用户生产力数据，并构建 UserDetail 数组
func GetProductivityByIds(db *gorm.DB, userids []string, startTime, endTime string) []UserDetail {
	var members []UserDetail
	daily, _, err := ListUserProductivity(db, UserFilter{UserIds: userids, StartTime: startTime, EndTime: endTime}, 1, 100000, "")
	if err != nil {
		return members
	}
	memberMap := make(map[string]*UserDetail)
	for _, d := range daily {
		if d.CreateTime.IsZero() {
			continue
		}
		ma := memberMap[d.UserId]
		if ma == nil {
			ma = &UserDetail{UserId: d.UserId, UserName: d.UserName}
			if om, ok := orgMappings[d.UserId]; ok {
				ma.UserName = om.UserName
				ma.Org1 = om.Org1
				ma.Org2 = om.Org2
				ma.Org3 = om.Org3
				ma.Org4 = om.Org4
				ma.Org5 = om.Org5
				ma.Org6 = om.Org6
				ma.Org7 = om.Org7
				ma.Org8 = om.Org8
				ma.Org9 = om.Org9
				ma.OrgDisplay = getOrgDisplay(om.Org1, om.Org2, om.Org3, om.Org4, om.Org5, om.Org6, om.Org7, om.Org8, om.Org9)
			}
		}
		ma.TaskDiffLines += d.TaskDiffLines
		ma.TaskRealMinutes += d.TaskRealMinutes
		ma.TaskAncientMinutes += d.TaskAncientMinutes
		ma.CommitDiffLines += d.CommitDiffLines
		ma.CommitRealMinutes += d.CommitRealMinutes
		ma.CommitAncientMinutes += d.CommitAncientMinutes
		ma.UpstreamTokens += d.UpstreamTokens
		ma.DownstreamTokens += d.DownstreamTokens
		ma.Cost += d.Cost
		memberMap[d.UserId] = ma
	}

	for _, ma := range memberMap {
		ma.TaskEfficiencyRatio = utils.CalcEfficiencyRatio(ma.TaskAncientMinutes, ma.TaskRealMinutes)
		ma.CommitEfficiencyRatio = utils.CalcEfficiencyRatio(ma.CommitAncientMinutes, ma.CommitRealMinutes)
		members = append(members, *ma)
	}

	return members
}

func periodKeyForTime(t time.Time, granularity string) (key string, label string) {
	switch granularity {
	case "week":
		weekday := int(t.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		monday := t.AddDate(0, 0, -(weekday - 1))
		firstDay := time.Date(monday.Year(), monday.Month(), 1, 0, 0, 0, 0, t.Location())
		firstWeekday := int(firstDay.Weekday())
		if firstWeekday == 0 {
			firstWeekday = 7
		}
		weekNum := (monday.Day()+firstWeekday-2)/7 + 1
		if weekNum <= 0 {
			weekNum = 1
		}
		key = fmt.Sprintf("%d%02d第%d周", monday.Year(), int(monday.Month()), weekNum)
		label = key
	case "month":
		key = t.Format("2006-01")
		label = fmt.Sprintf("%d年%02d月", t.Year(), int(t.Month()))
	case "year":
		key = t.Format("2006")
		label = fmt.Sprintf("%d年", t.Year())
	default:
		key = t.Format("2006-01-02")
		label = t.Format("2006-01-02")
	}
	return
}

func AggregateDailyByGranularity(daily []models.UserProductivity, granularity string) ([]CommitTimeSeriesItem, []TaskTimeSeriesItem) {
	orderKeys := make([]string, 0)
	periodMap := make(map[string]*PeriodStatistics)

	for _, d := range daily {
		if d.CreateTime.IsZero() {
			continue
		}
		key, label := periodKeyForTime(d.CreateTime, granularity)
		pd, exists := periodMap[key]
		if !exists {
			pd = &PeriodStatistics{PeriodKey: key, PeriodLabel: label}
			periodMap[key] = pd
			orderKeys = append(orderKeys, key)
		}
		if d.TaskIds != "" {
			var ids []interface{}
			if json.Unmarshal([]byte(d.TaskIds), &ids) == nil {
				pd.TaskCount += len(ids)
			}
		}
		if d.CommitIds != "" {
			var ids []interface{}
			if json.Unmarshal([]byte(d.CommitIds), &ids) == nil {
				pd.CommitCount += len(ids)
			}
		}
		pd.TaskDiffLines += d.TaskDiffLines
		pd.CommitDiffLines += d.CommitDiffLines
		pd.UpstreamTokens += d.UpstreamTokens
		pd.DownstreamTokens += d.DownstreamTokens
		pd.Cost += d.Cost
		pd.TaskRealMinutes += d.TaskRealMinutes
		pd.TaskAncientMinutes += d.TaskAncientMinutes
		pd.CommitRealMinutes += d.CommitRealMinutes
		pd.CommitAncientMinutes += d.CommitAncientMinutes
	}

	sort.Slice(orderKeys, func(i, j int) bool {
		return orderKeys[i] > orderKeys[j]
	})
	commitsList := make([]CommitTimeSeriesItem, 0, len(orderKeys))
	tasksList := make([]TaskTimeSeriesItem, 0, len(orderKeys))
	for _, key := range orderKeys {
		pd := periodMap[key]
		commitEffRatio := utils.CalcEfficiencyRatio(pd.CommitAncientMinutes, pd.CommitRealMinutes)
		taskEffRatio := utils.CalcEfficiencyRatio(pd.TaskAncientMinutes, pd.TaskRealMinutes)

		commitsList = append(commitsList, CommitTimeSeriesItem{
			PeriodKey:             key,
			PeriodLabel:           pd.PeriodLabel,
			CommitCount:           pd.CommitCount,
			CommitDiffLines:       pd.CommitDiffLines,
			CommitRealMinutes:     pd.CommitRealMinutes,
			CommitAncientMinutes:  pd.CommitAncientMinutes,
			CommitEfficiencyRatio: commitEffRatio,
			UpstreamTokens:        pd.UpstreamTokens,
			DownstreamTokens:      pd.DownstreamTokens,
			Cost:                  pd.Cost,
		})

		tasksList = append(tasksList, TaskTimeSeriesItem{
			PeriodKey:           key,
			PeriodLabel:         pd.PeriodLabel,
			TaskCount:           pd.TaskCount,
			TaskDiffLines:       pd.TaskDiffLines,
			TaskRealMinutes:     pd.TaskRealMinutes,
			TaskAncientMinutes:  pd.TaskAncientMinutes,
			TaskEfficiencyRatio: taskEffRatio,
			UpstreamTokens:      pd.UpstreamTokens,
			DownstreamTokens:    pd.DownstreamTokens,
			Cost:                pd.Cost,
		})
	}
	return commitsList, tasksList
}
