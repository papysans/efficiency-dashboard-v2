package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
)

// listUsersV2 GET /api/v2/users
func listUsersV2(c *gin.Context) {
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	granularity := c.Query("granularity")

	var startTime, endTime string
	if startDate != "" {
		startT, err := parseDateParam(startDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "startDate 格式错误: " + err.Error()})
			return
		}
		startTime = startT.Format(time.RFC3339)
	}
	if endDate != "" {
		endT, err := parseDateParam(endDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "endDate 格式错误: " + err.Error()})
			return
		}
		endTime = endT.Add(23*time.Hour + 59*time.Minute + 59*time.Second).Format(time.RFC3339)
	}

	filterOrg1 := c.Query("org1")
	filterOrg2 := c.Query("org2")
	filterOrg3 := c.Query("org3")
	filterOrg4 := c.Query("org4")

	// 从 user_productivity 表聚合
	query := `SELECT user_id,
		COALESCE(MAX(user_name), '') as user_name,
		COUNT(*) as day_count,
		COALESCE(SUM(jsonb_array_length(task_ids)), 0) as task_count,
		COALESCE(SUM(jsonb_array_length(commit_ids)), 0) as commit_count,
		COALESCE(SUM(task_diff_lines), 0) as task_diff_lines,
		COALESCE(SUM(commit_diff_lines), 0) as commit_diff_lines,
		COALESCE(SUM(upstream_tokens), 0) as upstream_tokens,
		COALESCE(SUM(downstream_tokens), 0) as downstream_tokens,
		COALESCE(SUM(cost), 0) as cost,
		COALESCE(SUM(task_real_minutes), 0) as task_real_minutes,
		COALESCE(SUM(task_ancient_minutes), 0) as task_ancient_minutes,
		COALESCE(SUM(commit_real_minutes), 0) as commit_real_minutes,
		COALESCE(SUM(commit_ancient_minutes), 0) as commit_ancient_minutes
		FROM user_productivity`

	var conditions []string
	var args []interface{}
	argIdx := 1
	if startTime != "" {
		conditions = append(conditions, fmt.Sprintf("create_time >= $%d", argIdx))
		args = append(args, startTime)
		argIdx++
	}
	if endTime != "" {
		conditions = append(conditions, fmt.Sprintf("create_time <= $%d", argIdx))
		args = append(args, endTime)
		argIdx++
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " GROUP BY user_id"

	rows, err := statDB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询 user_productivity 聚合失败: " + err.Error()})
		return
	}
	defer rows.Close()

	all := make([]gin.H, 0)
	for rows.Next() {
		var uid, userName string
		var dayCount, taskCount, commitCount int
		var taskDiffLines, commitDiffLines int
		var upTokens, downTokens int64
		var cost, taskRealMin, taskAncientMin, commitRealMin, commitAncientMin float64

		if err := rows.Scan(&uid, &userName, &dayCount, &taskCount, &commitCount,
			&taskDiffLines, &commitDiffLines, &upTokens, &downTokens, &cost,
			&taskRealMin, &taskAncientMin, &commitRealMin, &commitAncientMin); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "扫描 user_productivity 聚合行失败: " + err.Error()})
			return
		}

		var taskEffRatio, commitEffRatio float64
		if taskRealMin > 0 {
			taskEffRatio = math.Round(taskAncientMin / taskRealMin * 100)
		}
		if commitRealMin > 0 {
			commitEffRatio = math.Round(commitAncientMin / commitRealMin * 100)
		}

		var org1, org2, org3, org4 string
		if om, ok := orgMappings[uid]; ok {
			org1 = om.Org1
			org2 = om.Org2
			org3 = om.Org3
			org4 = om.Org4
		}

		// org 过滤
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

		// 拼接 org_display
		var orgParts []string
		for _, v := range []string{org1, org2, org3, org4} {
			if v != "" {
				orgParts = append(orgParts, v)
			}
		}
		orgDisplay := strings.Join(orgParts, "/")

		all = append(all, gin.H{
			"user_id":                 uid,
			"user_name":               userName,
			"day_count":               dayCount,
			"task_count":              taskCount,
			"commit_count":            commitCount,
			"task_diff_lines":         taskDiffLines,
			"commit_diff_lines":       commitDiffLines,
			"upstream_tokens":         upTokens,
			"downstream_tokens":       downTokens,
			"cost":                    cost,
			"task_real_minutes":       taskRealMin,
			"task_ancient_minutes":    taskAncientMin,
			"task_efficiency_ratio":   taskEffRatio,
			"commit_real_minutes":     commitRealMin,
			"commit_ancient_minutes":  commitAncientMin,
			"commit_efficiency_ratio": commitEffRatio,
			"org1":                    org1,
			"org2":                    org2,
			"org3":                    org3,
			"org4":                    org4,
			"org_display":             orgDisplay,
			"is_virtual_group":        false,
			"org_name":                "",
		})
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "遍历 user_productivity 聚合结果失败: " + err.Error()})
		return
	}

	// 追加虚拟组数据
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
				taskEffRatio = math.Round(taskAncientMin / taskRealMin * 100)
			}
			if commitRealMin > 0 {
				commitEffRatio = math.Round(commitAncientMin / commitRealMin * 100)
			}

			all = append(all, gin.H{
				"user_id":                 group.GroupID,
				"user_name":               group.Name,
				"group_id":                group.GroupID,
				"day_count":               dayCount,
				"task_count":              taskCount,
				"commit_count":            commitCount,
				"task_diff_lines":         taskDiffLines,
				"commit_diff_lines":       commitDiffLines,
				"upstream_tokens":         upTokens,
				"downstream_tokens":       downTokens,
				"cost":                    cost,
				"task_real_minutes":       taskRealMin,
				"task_ancient_minutes":    taskAncientMin,
				"task_efficiency_ratio":   taskEffRatio,
				"commit_real_minutes":     commitRealMin,
				"commit_ancient_minutes":  commitAncientMin,
				"commit_efficiency_ratio": commitEffRatio,
				"is_virtual_group":        true,
				"org_display":             group.OrgName,
				"org_name":                group.OrgName,
				"org1":                    "",
				"org2":                    "",
				"org3":                    "",
				"org4":                    "",
			})
		}
	}

	// 内存分页
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

	// 构建时间序列 series（仅当 granularity 非空时）
	series := []gin.H{}
	allPeriods := []string{}
	if granularity != "" && len(all) > 0 {
		// 收集所有 user_id（非虚拟组）
		var allUserIDs []string
		for _, u := range all {
			if isVirtual, _ := u["is_virtual_group"].(bool); !isVirtual {
				if uid, ok := u["user_id"].(string); ok {
					allUserIDs = append(allUserIDs, uid)
				}
			}
		}

		if len(allUserIDs) > 0 {
			seriesQuery := `SELECT user_id, create_time,
				COALESCE(jsonb_array_length(task_ids), 0),
				COALESCE(jsonb_array_length(commit_ids), 0),
				COALESCE(task_diff_lines, 0),
				COALESCE(commit_diff_lines, 0),
				COALESCE(upstream_tokens, 0),
				COALESCE(downstream_tokens, 0),
				COALESCE(cost, 0),
				COALESCE(task_real_minutes, 0),
				COALESCE(task_ancient_minutes, 0),
				COALESCE(commit_real_minutes, 0),
				COALESCE(commit_ancient_minutes, 0)
				FROM user_productivity
				WHERE user_id = ANY($1::text[])`

			seriesArgs := []interface{}{pq.Array(allUserIDs)}
			seriesArgIdx := 2
			if startTime != "" {
				seriesQuery += fmt.Sprintf(" AND create_time >= $%d", seriesArgIdx)
				seriesArgs = append(seriesArgs, startTime)
				seriesArgIdx++
			}
			if endTime != "" {
				seriesQuery += fmt.Sprintf(" AND create_time <= $%d", seriesArgIdx)
				seriesArgs = append(seriesArgs, endTime)
				seriesArgIdx++
			}
			seriesQuery += " ORDER BY create_time"

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

			sRows, err := statDB.Query(seriesQuery, seriesArgs...)
			if err == nil {
				defer sRows.Close()
				for sRows.Next() {
					var uid string
					var ct time.Time
					var taskCnt, commitCnt, taskDiff, commitDiff int
					var upTok, downTok int64
					var cost, taskReal, taskAnc, commitReal, commitAnc float64
					if err := sRows.Scan(&uid, &ct, &taskCnt, &commitCnt, &taskDiff, &commitDiff,
						&upTok, &downTok, &cost, &taskReal, &taskAnc, &commitReal, &commitAnc); err != nil {
						continue
					}
					dateStr := ct.Format("2006-01-02")
					key := dayUserKey{date: dateStr, userID: uid}
					if _, ok := dayUserMap[key]; !ok {
						dayUserMap[key] = &dayUserAgg{}
					}
					da := dayUserMap[key]
					da.taskCount += taskCnt
					da.commitCount += commitCnt
					da.taskDiffLines += taskDiff
					da.commitDiffLines += commitDiff
					da.upTokens += upTok
					da.downTokens += downTok
					da.cost += cost
					da.taskRealMin += taskReal
					da.taskAncientMin += taskAnc
					da.commitRealMin += commitReal
					da.commitAncientMin += commitAnc
				}
			}

			// 收集所有日期并排序
			dateSet := make(map[string]bool)
			for k := range dayUserMap {
				dateSet[k.date] = true
			}
			var allDates []string
			for d := range dateSet {
				allDates = append(allDates, d)
			}
			sort.Strings(allDates)

			// 计算某日期是所在月的第几周（以周一为周起始）
			getWeekOfMonth := func(t time.Time) int {
				// 获取当月第一天
				firstDay := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
				// 计算当月第一天是周几（周一=1, 周日=0）
				firstWeekday := int(firstDay.Weekday())
				if firstWeekday == 0 {
					firstWeekday = 7
				}
				// 计算是该月的第几周
				return (t.Day()+firstWeekday-2)/7 + 1
			}

			periodOf := func(dateStr string) string {
				t, _ := time.Parse("2006-01-02", dateStr)
				switch granularity {
				case "week":
					// 格式：年月第几周，如 "202608第2周"
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

			// 按 userID × period 聚合
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

			// 收集并排序 periods
			for p := range periodSet {
				allPeriods = append(allPeriods, p)
			}
			sort.Strings(allPeriods)

			// 构建 series：每个 user 一条记录
			// 用 all 中的顺序（已排序），只取非虚拟组
			for _, u := range all {
				if isVirtual, _ := u["is_virtual_group"].(bool); isVirtual {
					continue
				}
				uid, _ := u["user_id"].(string)
				userName, _ := u["user_name"].(string)
				if userName == "" {
					userName = uid
				}
				points := make([]gin.H, 0, len(allPeriods))
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
							taskEffRatio = math.Round(pa.taskAncientMin / pa.taskRealMin * 100)
						}
						if pa.commitRealMin > 0 {
							commitEffRatio = math.Round(pa.commitAncientMin / pa.commitRealMin * 100)
						}
					}
					points = append(points, gin.H{
						"period":                  period,
						"task_count":              taskCount,
						"commit_count":            commitCount,
						"task_diff_lines":         taskDiff,
						"commit_diff_lines":       commitDiff,
						"task_efficiency_ratio":   taskEffRatio,
						"commit_efficiency_ratio": commitEffRatio,
						"total_tokens":            upTok + downTok,
						"total_cost":              cost,
						"task_real_minutes":       taskRealMin,
						"task_ancient_minutes":    taskAncientMin,
						"commit_real_minutes":     commitRealMin,
						"commit_ancient_minutes":  commitAncientMin,
					})
				}
				series = append(series, gin.H{
					"user_id":   uid,
					"user_name": userName,
					"periods":   allPeriods,
					"points":    points,
				})
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
		"data":     pagedSlice,
		"series":   series,
		"periods":  allPeriods,
	})
}

// periodKeyForTime 根据聚合粒度返回时间分组 key 和展示标签
func periodKeyForTime(t time.Time, granularity string) (key string, label string) {
	switch granularity {
	case "week":
		// 计算该周第一天（周一）
		weekday := int(t.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		monday := t.AddDate(0, 0, -(weekday - 1))
		// 第几周：该月1日是第几周（以周一为周起始）
		firstDay := time.Date(monday.Year(), monday.Month(), 1, 0, 0, 0, 0, t.Location())
		firstWeekday := int(firstDay.Weekday())
		if firstWeekday == 0 {
			firstWeekday = 7
		}
		weekNum := (monday.Day()+firstWeekday-2)/7 + 1
		if weekNum <= 0 {
			weekNum = 1
		}
		// 格式：年月第几周，如 "202608第2周"
		key = fmt.Sprintf("%d%02d第%d周", monday.Year(), int(monday.Month()), weekNum)
		label = key
	case "month":
		key = t.Format("2006-01")
		label = fmt.Sprintf("%d年%02d月", t.Year(), int(t.Month()))
	case "year":
		key = t.Format("2006")
		label = fmt.Sprintf("%d年", t.Year())
	default: // day
		key = t.Format("2006-01-02")
		label = t.Format("2006-01-02")
	}
	return
}

// aggregateDailyByGranularity 将按天数据聚合为指定粒度的 commits 和 tasks 两个列表
func aggregateDailyByGranularity(daily []UserProductivity, granularity string) (commitsList []gin.H, tasksList []gin.H) {
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

	commitsList = make([]gin.H, 0, len(orderKeys))
	tasksList = make([]gin.H, 0, len(orderKeys))

	for _, key := range orderKeys {
		pd := periodMap[key]
		var commitEffRatio, taskEffRatio float64
		if pd.commitRealMin > 0 {
			commitEffRatio = math.Round(pd.commitAncientMin / pd.commitRealMin * 100)
		}
		if pd.taskRealMin > 0 {
			taskEffRatio = math.Round(pd.taskAncientMin / pd.taskRealMin * 100)
		}

		commitsList = append(commitsList, gin.H{
			"period_key":              key,
			"period_label":            pd.label,
			"commit_count":            pd.commitCount,
			"task_count":              pd.taskCount,
			"commit_diff_lines":       pd.commitDiffLines,
			"commit_real_minutes":     pd.commitRealMin,
			"commit_ancient_minutes":  pd.commitAncientMin,
			"commit_efficiency_ratio": commitEffRatio,
			"upstream_tokens":         pd.upTokens,
			"downstream_tokens":       pd.downTokens,
			"cost":                    pd.cost,
		})

		tasksList = append(tasksList, gin.H{
			"period_key":            key,
			"period_label":          pd.label,
			"task_count":            pd.taskCount,
			"commit_count":          pd.commitCount,
			"task_diff_lines":       pd.taskDiffLines,
			"task_real_minutes":     pd.taskRealMin,
			"task_ancient_minutes":  pd.taskAncientMin,
			"task_efficiency_ratio": taskEffRatio,
			"upstream_tokens":       pd.upTokens,
			"downstream_tokens":     pd.downTokens,
			"cost":                  pd.cost,
		})
	}
	return
}

// getUserDetailV2 GET /api/v2/users/:userId
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
			c.JSON(http.StatusBadRequest, gin.H{"error": "startDate 格式错误: " + err.Error()})
			return
		}
		startTime = startT.Format(time.RFC3339)
	}
	if endDate != "" {
		endT, err := parseDateParam(endDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "endDate 格式错误: " + err.Error()})
			return
		}
		endTime = endT.Add(23*time.Hour + 59*time.Minute + 59*time.Second).Format(time.RFC3339)
	}

	// 获取按天明细
	daily, err := ListUserProductivity(statDB, userID, startTime, endTime, 1, 10000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询 user_productivity 失败: " + err.Error()})
		return
	}

	// 计算汇总
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
		taskEffRatio = math.Round(taskAncientMin / taskRealMin * 100)
	}
	if commitRealMin > 0 {
		commitEffRatio = math.Round(commitAncientMin / commitRealMin * 100)
	}

	summary := gin.H{
		"user_id":                 userID,
		"user_name":               userName,
		"day_count":               dayCount,
		"task_count":              taskCount,
		"commit_count":            commitCount,
		"task_diff_lines":         taskDiffLines,
		"commit_diff_lines":       commitDiffLines,
		"upstream_tokens":         upTokens,
		"downstream_tokens":       downTokens,
		"cost":                    cost,
		"task_real_minutes":       taskRealMin,
		"task_ancient_minutes":    taskAncientMin,
		"task_efficiency_ratio":   taskEffRatio,
		"commit_real_minutes":     commitRealMin,
		"commit_ancient_minutes":  commitAncientMin,
		"commit_efficiency_ratio": commitEffRatio,
	}

	commitsList, tasksList := aggregateDailyByGranularity(daily, granularity)

	c.JSON(http.StatusOK, gin.H{
		"summary":     summary,
		"daily":       daily,
		"commits":     commitsList,
		"tasks":       tasksList,
		"total":       dayCount,
		"granularity": granularity,
	})
}
