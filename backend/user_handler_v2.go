package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"kanban/core/utils"

	"github.com/gin-gonic/gin"
)

type UserListItem struct {
	UserID                string  `json:"user_id"`
	UserName              string  `json:"user_name"`
	DayCount              int     `json:"day_count"`
	TaskCount             int     `json:"task_count"`
	CommitCount           int     `json:"commit_count"`
	TaskDiffLines         int     `json:"task_diff_lines"`
	CommitDiffLines       int     `json:"commit_diff_lines"`
	UpstreamTokens        int64   `json:"upstream_tokens"`
	DownstreamTokens      int64   `json:"downstream_tokens"`
	Cost                  float64 `json:"cost"`
	TaskRealMinutes       float64 `json:"task_real_minutes"`
	TaskAncientMinutes    float64 `json:"task_ancient_minutes"`
	TaskEfficiencyRatio   float64 `json:"task_efficiency_ratio"`
	CommitRealMinutes     float64 `json:"commit_real_minutes"`
	CommitAncientMinutes  float64 `json:"commit_ancient_minutes"`
	CommitEfficiencyRatio float64 `json:"commit_efficiency_ratio"`
	Org1                  string  `json:"org1"`
	Org2                  string  `json:"org2"`
	Org3                  string  `json:"org3"`
	Org4                  string  `json:"org4"`
	OrgDisplay            string  `json:"org_display"`
	IsVirtualGroup        bool    `json:"is_virtual_group"`
	OrgName               string  `json:"org_name"`
	GroupID               string  `json:"group_id,omitempty"`
}

type UserSeriesPoint struct {
	Period                string  `json:"period"`
	TaskCount             int     `json:"task_count"`
	CommitCount           int     `json:"commit_count"`
	TaskDiffLines         int     `json:"task_diff_lines"`
	CommitDiffLines       int     `json:"commit_diff_lines"`
	TaskEfficiencyRatio   float64 `json:"task_efficiency_ratio"`
	CommitEfficiencyRatio float64 `json:"commit_efficiency_ratio"`
	TotalTokens           int64   `json:"total_tokens"`
	TotalCost             float64 `json:"total_cost"`
	TaskRealMinutes       float64 `json:"task_real_minutes"`
	TaskAncientMinutes    float64 `json:"task_ancient_minutes"`
	CommitRealMinutes     float64 `json:"commit_real_minutes"`
	CommitAncientMinutes  float64 `json:"commit_ancient_minutes"`
}

type UserSeriesItem struct {
	UserID   string            `json:"user_id"`
	UserName string            `json:"user_name"`
	Periods  []string          `json:"periods"`
	Points   []UserSeriesPoint `json:"points"`
}

type UsersListResponse struct {
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"pageSize"`
	Data     []UserListItem   `json:"data"`
	Series   []UserSeriesItem `json:"series"`
	Periods  []string         `json:"periods"`
}

type UserDetailSummary struct {
	UserID                string  `json:"user_id"`
	UserName              string  `json:"user_name"`
	DayCount              int     `json:"day_count"`
	TaskCount             int     `json:"task_count"`
	CommitCount           int     `json:"commit_count"`
	TaskDiffLines         int     `json:"task_diff_lines"`
	CommitDiffLines       int     `json:"commit_diff_lines"`
	UpstreamTokens        int64   `json:"upstream_tokens"`
	DownstreamTokens      int64   `json:"downstream_tokens"`
	Cost                  float64 `json:"cost"`
	TaskRealMinutes       float64 `json:"task_real_minutes"`
	TaskAncientMinutes    float64 `json:"task_ancient_minutes"`
	TaskEfficiencyRatio   float64 `json:"task_efficiency_ratio"`
	CommitRealMinutes     float64 `json:"commit_real_minutes"`
	CommitAncientMinutes  float64 `json:"commit_ancient_minutes"`
	CommitEfficiencyRatio float64 `json:"commit_efficiency_ratio"`
}

type UserDetailResponse struct {
	Summary     UserDetailSummary      `json:"summary"`
	Daily       []UserProductivity     `json:"daily"`
	Commits     []CommitTimeSeriesItem `json:"commits"`
	Tasks       []TaskTimeSeriesItem   `json:"tasks"`
	Total       int                    `json:"total"`
	Granularity string                 `json:"granularity"`
}

// listUsersV2 GET /api/v2/users
// @Summary 获取用户列表
// @Description 按日期范围查询用户效率汇总列表，支持分页、组织筛选和时间序列
// @Tags Users
// @Produce json
// @Param startDate query string false "开始日期(YYYYMMDD)"
// @Param endDate query string false "结束日期(YYYYMMDD)"
// @Param granularity query string false "时间粒度(day/week/month/year)"
// @Param org1 query string false "一级组织"
// @Param org2 query string false "二级组织"
// @Param org3 query string false "三级组织"
// @Param org4 query string false "四级组织"
// @Param page query int false "页码" default(1)
// @Param pageSize query int false "每页数量" default(20)
// @Success 200 {object} UsersListResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/users [get]
func listUsersV2(c *gin.Context) {
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	granularity := c.Query("granularity")

	var startTime, endTime string
	if startDate != "" {
		startT, err := parseDateParam(startDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "startDate 格式错误: " + err.Error()})
			return
		}
		startTime = startT.Format(time.RFC3339)
	}
	if endDate != "" {
		endT, err := parseDateParam(endDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "endDate 格式错误: " + err.Error()})
			return
		}
		endTime = endT.Add(23*time.Hour + 59*time.Minute + 59*time.Second).Format(time.RFC3339)
	}

	filterOrg1 := c.Query("org1")
	filterOrg2 := c.Query("org2")
	filterOrg3 := c.Query("org3")
	filterOrg4 := c.Query("org4")

	aggRows, err := QueryUserProdAgg(statDB, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	all := make([]UserListItem, 0)
	for _, row := range aggRows {
		var taskEffRatio, commitEffRatio float64
		if row.TaskRealMinutes > 0 {
			taskEffRatio = utils.CalcEfficiencyRatio(row.TaskAncientMinutes, row.TaskRealMinutes)
		}
		if row.CommitRealMinutes > 0 {
			commitEffRatio = utils.CalcEfficiencyRatio(row.CommitAncientMinutes, row.CommitRealMinutes)
		}

		var org1, org2, org3, org4 string
		if om, ok := orgMappings[row.UserID]; ok {
			org1 = om.Org1
			org2 = om.Org2
			org3 = om.Org3
			org4 = om.Org4
		}

		if filterOrg1 != "" && org1 != filterOrg1 {
			continue
		}
		if filterOrg2 != "" && org2 != filterOrg2 {
			continue
		}
		if filterOrg3 != "" && org3 != filterOrg3 {
			continue
		}
		if filterOrg4 != "" && org4 != filterOrg4 {
			continue
		}

		var orgParts []string
		for _, v := range []string{org1, org2, org3, org4} {
			if v != "" {
				orgParts = append(orgParts, v)
			}
		}
		orgDisplay := strings.Join(orgParts, "/")

		all = append(all, UserListItem{
			UserID: row.UserID, UserName: row.UserName, DayCount: row.DayCount,
			TaskCount: row.TaskCount, CommitCount: row.CommitCount,
			TaskDiffLines: row.TaskDiffLines, CommitDiffLines: row.CommitDiffLines,
			UpstreamTokens: row.UpstreamTokens, DownstreamTokens: row.DownstreamTokens,
			Cost: row.Cost, TaskRealMinutes: row.TaskRealMinutes,
			TaskAncientMinutes:    row.TaskAncientMinutes,
			TaskEfficiencyRatio:   taskEffRatio,
			CommitRealMinutes:     row.CommitRealMinutes,
			CommitAncientMinutes:  row.CommitAncientMinutes,
			CommitEfficiencyRatio: commitEffRatio,
			Org1:                  org1, Org2: org2, Org3: org3, Org4: org4,
			OrgDisplay: orgDisplay, IsVirtualGroup: false,
			OrgName: "",
		})
	}

	groups, err := ListUserGroups(statDB)
	if err == nil {
		for _, group := range groups {
			var userIDs []string
			if err := json.Unmarshal(group.UserIDs, &userIDs); err != nil {
				continue
			}

			var taskDiffLines, commitDiffLines int
			var upTokens, downTokens int64
			var cost, taskRealMin, taskAncientMin, commitRealMin, commitAncientMin float64
			var dayCount, taskCount, commitCount int

			for _, uid := range userIDs {
				daily, err := ListUserProductivity(statDB, uid, startTime, endTime, 1, 100000)
				if err != nil {
					continue
				}
				dayCount += len(daily)
				for _, d := range daily {
					if d.TaskIDs != nil {
						var ids []interface{}
						if json.Unmarshal(d.TaskIDs, &ids) == nil {
							taskCount += len(ids)
						}
					}
					if d.CommitIDs != nil {
						var ids []interface{}
						if json.Unmarshal(d.CommitIDs, &ids) == nil {
							commitCount += len(ids)
						}
					}
					if d.TaskDiffLines != nil {
						taskDiffLines += *d.TaskDiffLines
					}
					if d.CommitDiffLines != nil {
						commitDiffLines += *d.CommitDiffLines
					}
					if d.UpstreamTokens != nil {
						upTokens += *d.UpstreamTokens
					}
					if d.DownstreamTokens != nil {
						downTokens += *d.DownstreamTokens
					}
					if d.Cost != nil {
						cost += *d.Cost
					}
					if d.TaskRealMinutes != nil {
						taskRealMin += *d.TaskRealMinutes
					}
					if d.TaskAncientMinutes != nil {
						taskAncientMin += *d.TaskAncientMinutes
					}
					if d.CommitRealMinutes != nil {
						commitRealMin += *d.CommitRealMinutes
					}
					if d.CommitAncientMinutes != nil {
						commitAncientMin += *d.CommitAncientMinutes
					}
				}
			}

			var taskEffRatio, commitEffRatio float64
			if taskRealMin > 0 {
				taskEffRatio = utils.CalcEfficiencyRatio(taskAncientMin, taskRealMin)
			}
			if commitRealMin > 0 {
				commitEffRatio = utils.CalcEfficiencyRatio(commitAncientMin, commitRealMin)
			}

			all = append(all, UserListItem{
				UserID: group.GroupID, UserName: group.Name,
				GroupID:  group.GroupID,
				DayCount: dayCount, TaskCount: taskCount,
				CommitCount:   commitCount,
				TaskDiffLines: taskDiffLines, CommitDiffLines: commitDiffLines,
				UpstreamTokens: upTokens, DownstreamTokens: downTokens,
				Cost: cost, TaskRealMinutes: taskRealMin,
				TaskAncientMinutes:    taskAncientMin,
				TaskEfficiencyRatio:   taskEffRatio,
				CommitRealMinutes:     commitRealMin,
				CommitAncientMinutes:  commitAncientMin,
				CommitEfficiencyRatio: commitEffRatio,
				IsVirtualGroup:        true,
				OrgDisplay:            group.OrgName, OrgName: group.OrgName,
			})
		}
	}

	page := getDefaultInt(c, "page", 1)
	pageSize := getDefaultInt(c, "pageSize", DefaultPageSize)

	total := len(all)
	offset := (page - 1) * pageSize
	if offset > total {
		offset = total
	}
	end := offset + pageSize
	if end > total {
		end = total
	}
	pagedSlice := all[offset:end]

	series := []UserSeriesItem{}
	allPeriods := []string{}
	if granularity != "" && len(all) > 0 {
		var allUserIDs []string
		for _, u := range all {
			if !u.IsVirtualGroup {
				allUserIDs = append(allUserIDs, u.UserID)
			}
		}

		if len(allUserIDs) > 0 {
			sRows, err := QueryUserProdTimeSeries(statDB, allUserIDs, startTime, endTime)
			if err == nil {
				type dayUserAgg struct {
					taskCount        int
					commitCount      int
					taskDiffLines    int
					commitDiffLines  int
					upTokens         int64
					downTokens       int64
					cost             float64
					taskRealMin      float64
					taskAncientMin   float64
					commitRealMin    float64
					commitAncientMin float64
				}
				type dayUserKey struct {
					date   string
					userID string
				}
				dayUserMap := make(map[dayUserKey]*dayUserAgg)

				for _, row := range sRows {
					dateStr := row.CreateTime.Format("2006-01-02")
					key := dayUserKey{date: dateStr, userID: row.UserID}
					if _, ok := dayUserMap[key]; !ok {
						dayUserMap[key] = &dayUserAgg{}
					}
					da := dayUserMap[key]
					da.taskCount += row.TaskCount
					da.commitCount += row.CommitCount
					da.taskDiffLines += row.TaskDiffLines
					da.commitDiffLines += row.CommitDiffLines
					da.upTokens += row.UpstreamTokens
					da.downTokens += row.DownstreamTokens
					da.cost += row.Cost
					da.taskRealMin += row.TaskRealMinutes
					da.taskAncientMin += row.TaskAncientMinutes
					da.commitRealMin += row.CommitRealMinutes
					da.commitAncientMin += row.CommitAncientMinutes
				}

				dateSet := make(map[string]bool)
				for k := range dayUserMap {
					dateSet[k.date] = true
				}
				var allDates []string
				for d := range dateSet {
					allDates = append(allDates, d)
				}
				sort.Strings(allDates)

				getWeekOfMonth := func(t time.Time) int {
					firstDay := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
					firstWeekday := int(firstDay.Weekday())
					if firstWeekday == 0 {
						firstWeekday = 7
					}
					return (t.Day()+firstWeekday-2)/7 + 1
				}

				periodOf := func(dateStr string) string {
					t, _ := time.Parse("2006-01-02", dateStr)
					switch granularity {
					case "week":
						weekOfMonth := getWeekOfMonth(t)
						return fmt.Sprintf("%d%02d第%d周", t.Year(), t.Month(), weekOfMonth)
					case "month":
						return t.Format("2006-01")
					case "year":
						return t.Format("2006")
					default:
						return dateStr
					}
				}

				type periodUserKey struct {
					period string
					userID string
				}
				type periodUserAgg struct {
					taskCount        int
					commitCount      int
					taskDiffLines    int
					commitDiffLines  int
					upTokens         int64
					downTokens       int64
					cost             float64
					taskRealMin      float64
					taskAncientMin   float64
					commitRealMin    float64
					commitAncientMin float64
				}
				periodUserMap := make(map[periodUserKey]*periodUserAgg)
				periodSet := make(map[string]bool)

				for _, dateStr := range allDates {
					period := periodOf(dateStr)
					periodSet[period] = true
					for _, uid := range allUserIDs {
						key := dayUserKey{date: dateStr, userID: uid}
						da, ok := dayUserMap[key]
						if !ok {
							continue
						}
						pk := periodUserKey{period: period, userID: uid}
						if _, ok := periodUserMap[pk]; !ok {
							periodUserMap[pk] = &periodUserAgg{}
						}
						pa := periodUserMap[pk]
						pa.taskCount += da.taskCount
						pa.commitCount += da.commitCount
						pa.taskDiffLines += da.taskDiffLines
						pa.commitDiffLines += da.commitDiffLines
						pa.upTokens += da.upTokens
						pa.downTokens += da.downTokens
						pa.cost += da.cost
						pa.taskRealMin += da.taskRealMin
						pa.taskAncientMin += da.taskAncientMin
						pa.commitRealMin += da.commitRealMin
						pa.commitAncientMin += da.commitAncientMin
					}
				}

				for p := range periodSet {
					allPeriods = append(allPeriods, p)
				}
				sort.Strings(allPeriods)

				for _, u := range all {
					if u.IsVirtualGroup {
						continue
					}
					uid := u.UserID
					userName := u.UserName
					if userName == "" {
						userName = uid
					}
					points := make([]UserSeriesPoint, 0, len(allPeriods))
					for _, period := range allPeriods {
						pk := periodUserKey{period: period, userID: uid}
						pa := periodUserMap[pk]
						var taskEffRatio, commitEffRatio float64
						var taskCount, commitCount, taskDiff, commitDiff int
						var upTok, downTok int64
						var cost float64
						var taskRealMin, taskAncientMin, commitRealMin, commitAncientMin float64
						if pa != nil {
							taskCount = pa.taskCount
							commitCount = pa.commitCount
							taskDiff = pa.taskDiffLines
							commitDiff = pa.commitDiffLines
							upTok = pa.upTokens
							downTok = pa.downTokens
							cost = pa.cost
							taskRealMin = pa.taskRealMin
							taskAncientMin = pa.taskAncientMin
							commitRealMin = pa.commitRealMin
							commitAncientMin = pa.commitAncientMin
							if pa.taskRealMin > 0 {
								taskEffRatio = utils.CalcEfficiencyRatio(pa.taskAncientMin, pa.taskRealMin)
							}
							if pa.commitRealMin > 0 {
								commitEffRatio = utils.CalcEfficiencyRatio(pa.commitAncientMin, pa.commitRealMin)
							}
						}
						points = append(points, UserSeriesPoint{
							Period:    period,
							TaskCount: taskCount, CommitCount: commitCount,
							TaskDiffLines: taskDiff, CommitDiffLines: commitDiff,
							TaskEfficiencyRatio:   taskEffRatio,
							CommitEfficiencyRatio: commitEffRatio,
							TotalTokens:           upTok + downTok,
							TotalCost:             cost,
							TaskRealMinutes:       taskRealMin,
							TaskAncientMinutes:    taskAncientMin,
							CommitRealMinutes:     commitRealMin,
							CommitAncientMinutes:  commitAncientMin,
						})
					}
					series = append(series, UserSeriesItem{
						UserID: uid, UserName: userName,
						Periods: allPeriods,
						Points:  points,
					})
				}
			}
		}
	}

	c.JSON(http.StatusOK, UsersListResponse{
		Total: total, Page: page, PageSize: pageSize,
		Data: pagedSlice, Series: series, Periods: allPeriods,
	})
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

func aggregateDailyByGranularity(daily []UserProductivity, granularity string) ([]CommitTimeSeriesItem, []TaskTimeSeriesItem) {
	type periodData struct {
		label            string
		key              string
		commitCount      int
		taskCount        int
		taskDiffLines    int
		commitDiffLines  int
		upTokens         int64
		downTokens       int64
		cost             float64
		taskRealMin      float64
		taskAncientMin   float64
		commitRealMin    float64
		commitAncientMin float64
	}

	orderKeys := make([]string, 0)
	periodMap := make(map[string]*periodData)

	for _, d := range daily {
		if d.CreateTime == nil {
			continue
		}
		key, label := periodKeyForTime(*d.CreateTime, granularity)
		pd, exists := periodMap[key]
		if !exists {
			pd = &periodData{key: key, label: label}
			periodMap[key] = pd
			orderKeys = append(orderKeys, key)
		}
		if d.TaskIDs != nil {
			var ids []interface{}
			if json.Unmarshal(d.TaskIDs, &ids) == nil {
				pd.taskCount += len(ids)
			}
		}
		if d.CommitIDs != nil {
			var ids []interface{}
			if json.Unmarshal(d.CommitIDs, &ids) == nil {
				pd.commitCount += len(ids)
			}
		}
		if d.TaskDiffLines != nil {
			pd.taskDiffLines += *d.TaskDiffLines
		}
		if d.CommitDiffLines != nil {
			pd.commitDiffLines += *d.CommitDiffLines
		}
		if d.UpstreamTokens != nil {
			pd.upTokens += *d.UpstreamTokens
		}
		if d.DownstreamTokens != nil {
			pd.downTokens += *d.DownstreamTokens
		}
		if d.Cost != nil {
			pd.cost += *d.Cost
		}
		if d.TaskRealMinutes != nil {
			pd.taskRealMin += *d.TaskRealMinutes
		}
		if d.TaskAncientMinutes != nil {
			pd.taskAncientMin += *d.TaskAncientMinutes
		}
		if d.CommitRealMinutes != nil {
			pd.commitRealMin += *d.CommitRealMinutes
		}
		if d.CommitAncientMinutes != nil {
			pd.commitAncientMin += *d.CommitAncientMinutes
		}
	}

	commitsList := make([]CommitTimeSeriesItem, 0, len(orderKeys))
	tasksList := make([]TaskTimeSeriesItem, 0, len(orderKeys))

	for _, key := range orderKeys {
		pd := periodMap[key]
		var commitEffRatio, taskEffRatio float64
		if pd.commitRealMin > 0 {
			commitEffRatio = utils.CalcEfficiencyRatio(pd.commitAncientMin, pd.commitRealMin)
		}
		if pd.taskRealMin > 0 {
			taskEffRatio = utils.CalcEfficiencyRatio(pd.taskAncientMin, pd.taskRealMin)
		}

		commitsList = append(commitsList, CommitTimeSeriesItem{
			PeriodKey: key, PeriodLabel: pd.label,
			CommitCount: pd.commitCount, TaskCount: pd.taskCount,
			CommitDiffLines:       pd.commitDiffLines,
			CommitRealMinutes:     pd.commitRealMin,
			CommitAncientMinutes:  pd.commitAncientMin,
			CommitEfficiencyRatio: commitEffRatio,
			UpstreamTokens:        pd.upTokens,
			DownstreamTokens:      pd.downTokens,
			Cost:                  pd.cost,
		})

		tasksList = append(tasksList, TaskTimeSeriesItem{
			PeriodKey: key, PeriodLabel: pd.label,
			TaskCount: pd.taskCount, CommitCount: pd.commitCount,
			TaskDiffLines:       pd.taskDiffLines,
			TaskRealMinutes:     pd.taskRealMin,
			TaskAncientMinutes:  pd.taskAncientMin,
			TaskEfficiencyRatio: taskEffRatio,
			UpstreamTokens:      pd.upTokens,
			DownstreamTokens:    pd.downTokens,
			Cost:                pd.cost,
		})
	}
	return commitsList, tasksList
}

// getUserDetailV2 GET /api/v2/users/:userId
// @Summary 获取用户详情
// @Description 根据用户ID获取用户效率详情，包含每日数据、提交和任务时间序列
// @Tags Users
// @Produce json
// @Param userId path string true "用户ID"
// @Param startDate query string false "开始日期(YYYYMMDD)"
// @Param endDate query string false "结束日期(YYYYMMDD)"
// @Param granularity query string false "时间粒度(day/week/month/year)" default(day)
// @Success 200 {object} UserDetailResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/users/{userId} [get]
func getUserDetailV2(c *gin.Context) {
	userID := c.Param("userId")
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	granularity := c.Query("granularity")
	if granularity == "" {
		granularity = "day"
	}

	var startTime, endTime string
	if startDate != "" {
		startT, err := parseDateParam(startDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "startDate 格式错误: " + err.Error()})
			return
		}
		startTime = startT.Format(time.RFC3339)
	}
	if endDate != "" {
		endT, err := parseDateParam(endDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "endDate 格式错误: " + err.Error()})
			return
		}
		endTime = endT.Add(23*time.Hour + 59*time.Minute + 59*time.Second).Format(time.RFC3339)
	}

	daily, err := ListUserProductivity(statDB, userID, startTime, endTime, 1, 10000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "查询 user_productivity 失败: " + err.Error()})
		return
	}

	var dayCount, taskCount, commitCount int
	var taskDiffLines, commitDiffLines int
	var upTokens, downTokens int64
	var cost, taskRealMin, taskAncientMin, commitRealMin, commitAncientMin float64
	var userName string

	dayCount = len(daily)
	for i, d := range daily {
		if i == 0 && d.UserName != nil {
			userName = *d.UserName
		}
		if d.TaskIDs != nil {
			var ids []interface{}
			if json.Unmarshal(d.TaskIDs, &ids) == nil {
				taskCount += len(ids)
			}
		}
		if d.CommitIDs != nil {
			var ids []interface{}
			if json.Unmarshal(d.CommitIDs, &ids) == nil {
				commitCount += len(ids)
			}
		}
		if d.TaskDiffLines != nil {
			taskDiffLines += *d.TaskDiffLines
		}
		if d.CommitDiffLines != nil {
			commitDiffLines += *d.CommitDiffLines
		}
		if d.UpstreamTokens != nil {
			upTokens += *d.UpstreamTokens
		}
		if d.DownstreamTokens != nil {
			downTokens += *d.DownstreamTokens
		}
		if d.Cost != nil {
			cost += *d.Cost
		}
		if d.TaskRealMinutes != nil {
			taskRealMin += *d.TaskRealMinutes
		}
		if d.TaskAncientMinutes != nil {
			taskAncientMin += *d.TaskAncientMinutes
		}
		if d.CommitRealMinutes != nil {
			commitRealMin += *d.CommitRealMinutes
		}
		if d.CommitAncientMinutes != nil {
			commitAncientMin += *d.CommitAncientMinutes
		}
	}

	var taskEffRatio, commitEffRatio float64
	if taskRealMin > 0 {
		taskEffRatio = utils.CalcEfficiencyRatio(taskAncientMin, taskRealMin)
	}
	if commitRealMin > 0 {
		commitEffRatio = utils.CalcEfficiencyRatio(commitAncientMin, commitRealMin)
	}

	summary := UserDetailSummary{
		UserID: userID, UserName: userName,
		DayCount: dayCount, TaskCount: taskCount,
		CommitCount:   commitCount,
		TaskDiffLines: taskDiffLines, CommitDiffLines: commitDiffLines,
		UpstreamTokens: upTokens, DownstreamTokens: downTokens,
		Cost: cost, TaskRealMinutes: taskRealMin,
		TaskAncientMinutes:    taskAncientMin,
		TaskEfficiencyRatio:   taskEffRatio,
		CommitRealMinutes:     commitRealMin,
		CommitAncientMinutes:  commitAncientMin,
		CommitEfficiencyRatio: commitEffRatio,
	}

	commitsList, tasksList := aggregateDailyByGranularity(daily, granularity)

	c.JSON(http.StatusOK, UserDetailResponse{
		Summary:     summary,
		Daily:       daily,
		Commits:     commitsList,
		Tasks:       tasksList,
		Total:       dayCount,
		Granularity: granularity,
	})
}
