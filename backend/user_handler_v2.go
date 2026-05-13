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
	UserId                string  `json:"user_id"`
	UserName              string  `json:"user_name"`
	DayCount              int     `json:"day_count"`
	UpstreamTokens        int64   `json:"upstream_tokens"`
	DownstreamTokens      int64   `json:"downstream_tokens"`
	Cost                  float64 `json:"cost"`
	TaskCount             int     `json:"task_count"`
	TaskDiffLines         int     `json:"task_diff_lines"`
	TaskRealMinutes       float64 `json:"task_real_minutes"`
	TaskAncientMinutes    float64 `json:"task_ancient_minutes"`
	TaskEfficiencyRatio   float64 `json:"task_efficiency_ratio"`
	CommitCount           int     `json:"commit_count"`
	CommitDiffLines       int     `json:"commit_diff_lines"`
	CommitRealMinutes     float64 `json:"commit_real_minutes"`
	CommitAncientMinutes  float64 `json:"commit_ancient_minutes"`
	CommitEfficiencyRatio float64 `json:"commit_efficiency_ratio"`
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
	IsVirtualGroup        bool    `json:"is_virtual_group"`
	OrgName               string  `json:"org_name"`
	GroupID               string  `json:"group_id,omitempty"`
}

type UserSeriesPoint struct {
	Period                string  `json:"period"`
	TotalTokens           int64   `json:"total_tokens"`
	TotalCost             float64 `json:"total_cost"`
	TaskCount             int     `json:"task_count"`
	TaskDiffLines         int     `json:"task_diff_lines"`
	TaskEfficiencyRatio   float64 `json:"task_efficiency_ratio"`
	TaskRealMinutes       float64 `json:"task_real_minutes"`
	TaskAncientMinutes    float64 `json:"task_ancient_minutes"`
	CommitCount           int     `json:"commit_count"`
	CommitDiffLines       int     `json:"commit_diff_lines"`
	CommitEfficiencyRatio float64 `json:"commit_efficiency_ratio"`
	CommitRealMinutes     float64 `json:"commit_real_minutes"`
	CommitAncientMinutes  float64 `json:"commit_ancient_minutes"`
}

type UserSeriesItem struct {
	UserId   string            `json:"user_id"`
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
	UserId                string  `json:"user_id"`
	UserName              string  `json:"user_name"`
	DayCount              int     `json:"day_count"`
	UpstreamTokens        int64   `json:"upstream_tokens"`
	DownstreamTokens      int64   `json:"downstream_tokens"`
	Cost                  float64 `json:"cost"`
	TaskCount             int     `json:"task_count"`
	TaskDiffLines         int     `json:"task_diff_lines"`
	TaskRealMinutes       float64 `json:"task_real_minutes"`
	TaskAncientMinutes    float64 `json:"task_ancient_minutes"`
	TaskEfficiencyRatio   float64 `json:"task_efficiency_ratio"`
	CommitCount           int     `json:"commit_count"`
	CommitDiffLines       int     `json:"commit_diff_lines"`
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
// @Param org5 query string false "五级组织"
// @Param org6 query string false "六级组织"
// @Param org7 query string false "七级组织"
// @Param org8 query string false "八级组织"
// @Param org9 query string false "九级组织"
// @Param page query int false "页码" default(1)
// @Param pageSize query int false "每页数量" default(20)
// @Success 200 {object} UsersListResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/users [get]
func listUsersV2(c *gin.Context) {
	filter := UserFilter{
		Granularity: strings.TrimSpace(c.Query("granularity")),
		OrgsFilter: OrgsFilter{
			Org1: strings.TrimSpace(c.Query("org1")),
			Org2: strings.TrimSpace(c.Query("org2")),
			Org3: strings.TrimSpace(c.Query("org3")),
			Org4: strings.TrimSpace(c.Query("org4")),
			Org5: strings.TrimSpace(c.Query("org5")),
			Org6: strings.TrimSpace(c.Query("org6")),
			Org7: strings.TrimSpace(c.Query("org7")),
			Org8: strings.TrimSpace(c.Query("org8")),
			Org9: strings.TrimSpace(c.Query("org9")),
		},
	}
	orderField, orderDir := parseOrderParam(strings.TrimSpace(c.Query("order")))
	if orderField != "" && !isAllowedField(orderField, userSortFields) {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "不支持的排序字段: " + orderField})
		return
	}

	startDate := strings.TrimSpace(c.Query("startDate"))
	endDate := strings.TrimSpace(c.Query("endDate"))
	if startDate != "" {
		startT, err := parseDateParam(startDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "startDate 格式错误: " + err.Error()})
			return
		}
		filter.StartTime = startT.Format(time.RFC3339)
	}
	if endDate != "" {
		endT, err := parseDateParam(endDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "endDate 格式错误: " + err.Error()})
			return
		}
		filter.EndTime = endT.Add(23*time.Hour + 59*time.Minute + 59*time.Second).Format(time.RFC3339)
	}

	aggRows, err := QueryUserProdAgg(statDB, filter.StartTime, filter.EndTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	all := make([]UserListItem, 0)
	for _, row := range aggRows {
		om, matched := filter.MatchOrg(row.UserId)
		if !matched {
			continue
		}

		orgDisplay := filter.OrgDisplay(om)
		item := UserListItem{
			UserId:                row.UserId,
			UserName:              row.UserName,
			DayCount:              row.DayCount,
			UpstreamTokens:        row.UpstreamTokens,
			DownstreamTokens:      row.DownstreamTokens,
			Cost:                  row.Cost,
			TaskCount:             row.TaskCount,
			TaskDiffLines:         row.TaskDiffLines,
			TaskRealMinutes:       row.TaskRealMinutes,
			TaskAncientMinutes:    row.TaskAncientMinutes,
			TaskEfficiencyRatio:   utils.CalcEfficiencyRatio(row.TaskAncientMinutes, row.TaskRealMinutes),
			CommitCount:           row.CommitCount,
			CommitDiffLines:       row.CommitDiffLines,
			CommitRealMinutes:     row.CommitRealMinutes,
			CommitAncientMinutes:  row.CommitAncientMinutes,
			CommitEfficiencyRatio: utils.CalcEfficiencyRatio(row.CommitAncientMinutes, row.CommitRealMinutes),
			OrgDisplay:            orgDisplay,
			IsVirtualGroup:        false,
			OrgName:               "",
		}
		if om != nil {
			item.Org1 = om.Org1
			item.Org2 = om.Org2
			item.Org3 = om.Org3
			item.Org4 = om.Org4
			item.Org5 = om.Org5
			item.Org6 = om.Org6
			item.Org7 = om.Org7
			item.Org8 = om.Org8
			item.Org9 = om.Org9
		}
		all = append(all, item)
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
				daily, _, err := ListUserProductivity(statDB, UserFilter{UserIds: []string{uid}, StartTime: filter.StartTime, EndTime: filter.EndTime}, 1, 100000, "")
				if err != nil {
					continue
				}
				dayCount += len(daily)
				for _, d := range daily {
					if d.TaskIds != nil {
						var ids []interface{}
						if json.Unmarshal(d.TaskIds, &ids) == nil {
							taskCount += len(ids)
						}
					}
					if d.CommitIds != nil {
						var ids []interface{}
						if json.Unmarshal(d.CommitIds, &ids) == nil {
							commitCount += len(ids)
						}
					}
					taskDiffLines += d.TaskDiffLines
					commitDiffLines += d.CommitDiffLines
					upTokens += d.UpstreamTokens
					downTokens += d.DownstreamTokens
					cost += d.Cost
					taskRealMin += d.TaskRealMinutes
					taskAncientMin += d.TaskAncientMinutes
					commitRealMin += d.CommitRealMinutes
					commitAncientMin += d.CommitAncientMinutes
				}
			}

			taskEffRatio := utils.CalcEfficiencyRatio(taskAncientMin, taskRealMin)
			commitEffRatio := utils.CalcEfficiencyRatio(commitAncientMin, commitRealMin)

			all = append(all, UserListItem{
				UserId:                group.GroupID,
				UserName:              group.Name,
				GroupID:               group.GroupID,
				DayCount:              dayCount,
				UpstreamTokens:        upTokens,
				DownstreamTokens:      downTokens,
				Cost:                  cost,
				TaskCount:             taskCount,
				TaskDiffLines:         taskDiffLines,
				TaskRealMinutes:       taskRealMin,
				TaskAncientMinutes:    taskAncientMin,
				TaskEfficiencyRatio:   taskEffRatio,
				CommitCount:           commitCount,
				CommitDiffLines:       commitDiffLines,
				CommitRealMinutes:     commitRealMin,
				CommitAncientMinutes:  commitAncientMin,
				CommitEfficiencyRatio: commitEffRatio,
				IsVirtualGroup:        true,
				OrgDisplay:            group.OrgName,
				OrgName:               group.OrgName,
			})
		}
	}
	sortUserData(all, orderField, orderDir)

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
	if filter.Granularity != "" && len(all) > 0 {
		var allUserIDs []string
		for _, u := range all {
			if !u.IsVirtualGroup {
				allUserIDs = append(allUserIDs, u.UserId)
			}
		}

		if len(allUserIDs) > 0 {
			sRows, err := QueryUserProdTimeSeries(statDB, allUserIDs, filter.StartTime, filter.EndTime)
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
					key := dayUserKey{date: dateStr, userID: row.UserId}
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
					switch filter.Granularity {
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
					uid := u.UserId
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
							Period:                period,
							TotalTokens:           upTok + downTok,
							TotalCost:             cost,
							TaskCount:             taskCount,
							TaskDiffLines:         taskDiff,
							TaskEfficiencyRatio:   taskEffRatio,
							TaskRealMinutes:       taskRealMin,
							TaskAncientMinutes:    taskAncientMin,
							CommitCount:           commitCount,
							CommitDiffLines:       commitDiff,
							CommitEfficiencyRatio: commitEffRatio,
							CommitRealMinutes:     commitRealMin,
							CommitAncientMinutes:  commitAncientMin,
						})
					}
					series = append(series, UserSeriesItem{
						UserId: uid, UserName: userName,
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
		if d.CreateTime.IsZero() {
			continue
		}
		key, label := periodKeyForTime(d.CreateTime, granularity)
		pd, exists := periodMap[key]
		if !exists {
			pd = &periodData{key: key, label: label}
			periodMap[key] = pd
			orderKeys = append(orderKeys, key)
		}
		if d.TaskIds != nil {
			var ids []interface{}
			if json.Unmarshal(d.TaskIds, &ids) == nil {
				pd.taskCount += len(ids)
			}
		}
		if d.CommitIds != nil {
			var ids []interface{}
			if json.Unmarshal(d.CommitIds, &ids) == nil {
				pd.commitCount += len(ids)
			}
		}
		pd.taskDiffLines += d.TaskDiffLines
		pd.commitDiffLines += d.CommitDiffLines
		pd.upTokens += d.UpstreamTokens
		pd.downTokens += d.DownstreamTokens
		pd.cost += d.Cost
		pd.taskRealMin += d.TaskRealMinutes
		pd.taskAncientMin += d.TaskAncientMinutes
		pd.commitRealMin += d.CommitRealMinutes
		pd.commitAncientMin += d.CommitAncientMinutes
	}

	commitsList := make([]CommitTimeSeriesItem, 0, len(orderKeys))
	tasksList := make([]TaskTimeSeriesItem, 0, len(orderKeys))

	for _, key := range orderKeys {
		pd := periodMap[key]
		commitEffRatio := utils.CalcEfficiencyRatio(pd.commitAncientMin, pd.commitRealMin)
		taskEffRatio := utils.CalcEfficiencyRatio(pd.taskAncientMin, pd.taskRealMin)

		commitsList = append(commitsList, CommitTimeSeriesItem{
			PeriodKey:             key,
			PeriodLabel:           pd.label,
			CommitCount:           pd.commitCount,
			CommitDiffLines:       pd.commitDiffLines,
			CommitRealMinutes:     pd.commitRealMin,
			CommitAncientMinutes:  pd.commitAncientMin,
			CommitEfficiencyRatio: commitEffRatio,
			UpstreamTokens:        pd.upTokens,
			DownstreamTokens:      pd.downTokens,
			Cost:                  pd.cost,
		})

		tasksList = append(tasksList, TaskTimeSeriesItem{
			PeriodKey:           key,
			PeriodLabel:         pd.label,
			TaskCount:           pd.taskCount,
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

	daily, _, err := ListUserProductivity(statDB, UserFilter{UserIds: []string{userID}, StartTime: startTime, EndTime: endTime}, 1, 10000, "")
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
		if i == 0 {
			userName = d.UserName
		}
		if d.TaskIds != nil {
			var ids []interface{}
			if json.Unmarshal(d.TaskIds, &ids) == nil {
				taskCount += len(ids)
			}
		}
		if d.CommitIds != nil {
			var ids []interface{}
			if json.Unmarshal(d.CommitIds, &ids) == nil {
				commitCount += len(ids)
			}
		}
		taskDiffLines += d.TaskDiffLines
		commitDiffLines += d.CommitDiffLines
		upTokens += d.UpstreamTokens
		downTokens += d.DownstreamTokens
		cost += d.Cost
		taskRealMin += d.TaskRealMinutes
		taskAncientMin += d.TaskAncientMinutes
		commitRealMin += d.CommitRealMinutes
		commitAncientMin += d.CommitAncientMinutes
	}

	taskEffRatio := utils.CalcEfficiencyRatio(taskAncientMin, taskRealMin)
	commitEffRatio := utils.CalcEfficiencyRatio(commitAncientMin, commitRealMin)
	commitsList, tasksList := aggregateDailyByGranularity(daily, granularity)

	summary := UserDetailSummary{
		UserId:                userID,
		UserName:              userName,
		DayCount:              dayCount,
		TaskCount:             taskCount,
		CommitCount:           commitCount,
		TaskDiffLines:         taskDiffLines,
		CommitDiffLines:       commitDiffLines,
		UpstreamTokens:        upTokens,
		DownstreamTokens:      downTokens,
		Cost:                  cost,
		TaskRealMinutes:       taskRealMin,
		TaskAncientMinutes:    taskAncientMin,
		TaskEfficiencyRatio:   taskEffRatio,
		CommitRealMinutes:     commitRealMin,
		CommitAncientMinutes:  commitAncientMin,
		CommitEfficiencyRatio: commitEffRatio,
	}

	c.JSON(http.StatusOK, UserDetailResponse{
		Summary:     summary,
		Daily:       daily,
		Commits:     commitsList,
		Tasks:       tasksList,
		Total:       dayCount,
		Granularity: granularity,
	})
}
