package main

import (
	"net/http"
	"sort"
	"strings"

	"kanban/core/models"

	"github.com/gin-gonic/gin"
)

// 本文件提供基于 v2 数据源（user_productivity_v2 / needs / commits）的用户与组织聚合，
// 取代依赖空的 v1 user_productivity 表的老 handler。提效比为小数口径 (baseline-actual)/actual。

type UserV2Row struct {
	UserId              string   `json:"user_id"`
	UserName            string   `json:"user_name"`
	WeekCount           int      `json:"week_count"`
	MergedNeedCount     int64    `json:"merged_need_count"`
	ActiveNeedCount     int64    `json:"active_need_count"`
	AbandonedNeedCount  int64    `json:"abandoned_need_count"`
	ActualCalendarMin   float64  `json:"actual_calendar_min"`
	BaselineCalendarMin float64  `json:"baseline_calendar_min"`
	CalendarRatio       *float64 `json:"calendar_ratio"`
	ActualWorkMin       float64  `json:"actual_work_min"`
	BaselineWorkMin     float64  `json:"baseline_work_min"`
	WorkRatio           *float64 `json:"work_ratio"`
	CommitCount         int64    `json:"commit_count"`
	CommitDiffLines     int64    `json:"commit_diff_lines"`
	Cost                float64  `json:"cost"`
	Tokens              int64    `json:"tokens"`
	ConfidenceLimited   bool     `json:"confidence_limited"`
	ConfidenceReason    string   `json:"confidence_reason"`
}

type UsersV2NativeResponse struct {
	Total    int         `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"pageSize"`
	Data     []UserV2Row `json:"data"`
}

type UserV2DetailResponse struct {
	Summary UserV2Row                   `json:"summary"`
	Weeks   []models.UserProductivityV2 `json:"weeks"`
	Needs   []NeedsV2Summary            `json:"needs"`
	Commits []models.Commit             `json:"commits"`
}

type OrgV2Row struct {
	OrgName             string   `json:"org_name"`
	UserCount           int      `json:"user_count"`
	MergedNeedCount     int64    `json:"merged_need_count"`
	ActualCalendarMin   float64  `json:"actual_calendar_min"`
	BaselineCalendarMin float64  `json:"baseline_calendar_min"`
	CalendarRatio       *float64 `json:"calendar_ratio"`
	WorkRatio           *float64 `json:"work_ratio"`
	CommitCount         int64    `json:"commit_count"`
	CommitDiffLines     int64    `json:"commit_diff_lines"`
	Cost                float64  `json:"cost"`
}

type OrgsV2NativeResponse struct {
	Data         []OrgV2Row `json:"data"`
	NoOrgMapping bool       `json:"no_org_mapping"`
}

// aggregateUsersV2 把 user_productivity_v2 的周行按用户聚合。
func aggregateUsersV2(startDate, endDate, userID string) ([]UserV2Row, error) {
	agg, err := QueryEfficiencyV2Aggregate(statDB, startDate, endDate, userID)
	if err != nil {
		return nil, err
	}
	byUser := make(map[string]*UserV2Row)
	order := make([]string, 0)
	for _, w := range agg.Data {
		r, ok := byUser[w.UserId]
		if !ok {
			r = &UserV2Row{UserId: w.UserId, UserName: w.UserName}
			byUser[w.UserId] = r
			order = append(order, w.UserId)
		}
		if r.UserName == "" {
			r.UserName = w.UserName
		}
		r.WeekCount++
		r.MergedNeedCount += w.MergedNeedCount
		r.ActiveNeedCount += w.ActiveNeedCount
		r.AbandonedNeedCount += w.AbandonedNeedCount
		r.ActualCalendarMin += w.ActualCalendarMin
		r.BaselineCalendarMin += w.BaselineCalendarMin
		r.ActualWorkMin += w.ActualActiveWorkCorrectedMin
		r.BaselineWorkMin += w.BaselineFusedWorkMin
		r.CommitCount += w.CommitCount
		r.CommitDiffLines += w.CommitDiffLines
		r.Cost += w.Cost
		r.Tokens += w.UpstreamTokens + w.DownstreamTokens
		if w.ConfidenceLimited {
			r.ConfidenceLimited = true
			// 合并各周的受限原因（去重），供前端在"受限"tag 上展示。
			for _, tok := range strings.Split(w.ConfidenceReason, ";") {
				tok = strings.TrimSpace(tok)
				if tok == "" || strings.Contains(r.ConfidenceReason, tok) {
					continue
				}
				if r.ConfidenceReason != "" {
					r.ConfidenceReason += "; "
				}
				r.ConfidenceReason += tok
			}
		}
	}
	rows := make([]UserV2Row, 0, len(order))
	for _, uid := range order {
		r := byUser[uid]
		r.CalendarRatio = efficiencyV2Ratio(r.BaselineCalendarMin, r.ActualCalendarMin)
		r.WorkRatio = efficiencyV2Ratio(r.BaselineWorkMin, r.ActualWorkMin)
		rows = append(rows, *r)
	}
	// 正常(非受限)上提、受限下沉；同组内按合并需求数、其次实际工作量降序，让活跃用户在前。
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].ConfidenceLimited != rows[j].ConfidenceLimited {
			return !rows[i].ConfidenceLimited
		}
		if rows[i].MergedNeedCount != rows[j].MergedNeedCount {
			return rows[i].MergedNeedCount > rows[j].MergedNeedCount
		}
		return rows[i].ActualWorkMin > rows[j].ActualWorkMin
	})
	return rows, nil
}

// listUsersV2Native GET /api/v2/users
func listUsersV2Native(c *gin.Context) {
	if statDB == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: "数据库未连接"})
		return
	}
	rows, err := aggregateUsersV2(c.Query("startDate"), c.Query("endDate"), "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	page := getDefaultInt(c, "page", 1)
	pageSize := getDefaultInt(c, "pageSize", 50)
	total := len(rows)
	offset := (page - 1) * pageSize
	if offset > total {
		offset = total
	}
	end := offset + pageSize
	if end > total {
		end = total
	}
	c.JSON(http.StatusOK, UsersV2NativeResponse{
		Total: total, Page: page, PageSize: pageSize, Data: rows[offset:end],
	})
}

// getUserV2DetailNative GET /api/v2/users/:userId
func getUserV2DetailNative(c *gin.Context) {
	if statDB == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: "数据库未连接"})
		return
	}
	userID := strings.TrimSpace(c.Param("userId"))
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")

	rows, err := aggregateUsersV2(startDate, endDate, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	resp := UserV2DetailResponse{}
	if len(rows) > 0 {
		resp.Summary = rows[0]
	} else {
		resp.Summary = UserV2Row{UserId: userID}
	}

	// 周明细
	agg, err := QueryEfficiencyV2Aggregate(statDB, startDate, endDate, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	resp.Weeks = agg.Data

	// 该用户的 Need（主用户）
	needResp, err := QueryNeedsV2(statDB, NeedsV2Filter{
		StartDate: startDate, EndDate: endDate, UserId: userID, Page: 1, PageSize: 200,
	})
	if err == nil {
		resp.Needs = needResp.Data
	}

	// 该用户的 Commit（按时间倒序取最近 100 条）
	cq := statDB.Model(&models.Commit{}).Where("user_id = ?", userID)
	if start, e := parseStartDate(startDate); e == nil && start != nil {
		cq = cq.Where("commit_time >= ?", *start)
	}
	if endT, e := parseEndDate(endDate); e == nil && endT != nil {
		cq = cq.Where("commit_time <= ?", *endT)
	}
	_ = cq.Order("commit_time DESC").Limit(100).Find(&resp.Commits).Error

	c.JSON(http.StatusOK, resp)
}

// listOrgsV2Native GET /api/v2/orgs
func listOrgsV2Native(c *gin.Context) {
	if statDB == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: "数据库未连接"})
		return
	}
	rows, err := aggregateUsersV2(c.Query("startDate"), c.Query("endDate"), "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	// 用户 -> 顶层组织映射（user_org 在本数据集为空，则全部归入“未分组”）。
	orgByUser := make(map[string]string)
	var userOrgs []models.UserOrg
	if err := statDB.Find(&userOrgs).Error; err == nil {
		for _, uo := range userOrgs {
			name := strings.TrimSpace(uo.Org1)
			if name != "" {
				orgByUser[uo.UserId] = name
			}
		}
	}

	type acc struct {
		users               int
		merged              int64
		actualCalendarMin   float64
		baselineCalendarMin float64
		actualWorkMin       float64
		baselineWorkMin     float64
		commitCount         int64
		commitDiffLines     int64
		cost                float64
	}
	buckets := make(map[string]*acc)
	orgOrder := make([]string, 0)
	mapped := false
	for _, r := range rows {
		name := orgByUser[r.UserId]
		if name == "" {
			name = "未分组"
		} else {
			mapped = true
		}
		b, ok := buckets[name]
		if !ok {
			b = &acc{}
			buckets[name] = b
			orgOrder = append(orgOrder, name)
		}
		b.users++
		b.merged += r.MergedNeedCount
		b.actualCalendarMin += r.ActualCalendarMin
		b.baselineCalendarMin += r.BaselineCalendarMin
		b.actualWorkMin += r.ActualWorkMin
		b.baselineWorkMin += r.BaselineWorkMin
		b.commitCount += r.CommitCount
		b.commitDiffLines += r.CommitDiffLines
		b.cost += r.Cost
	}

	data := make([]OrgV2Row, 0, len(orgOrder))
	for _, name := range orgOrder {
		b := buckets[name]
		data = append(data, OrgV2Row{
			OrgName:             name,
			UserCount:           b.users,
			MergedNeedCount:     b.merged,
			ActualCalendarMin:   b.actualCalendarMin,
			BaselineCalendarMin: b.baselineCalendarMin,
			CalendarRatio:       efficiencyV2Ratio(b.baselineCalendarMin, b.actualCalendarMin),
			WorkRatio:           efficiencyV2Ratio(b.baselineWorkMin, b.actualWorkMin),
			CommitCount:         b.commitCount,
			CommitDiffLines:     b.commitDiffLines,
			Cost:                b.cost,
		})
	}
	sort.SliceStable(data, func(i, j int) bool { return data[i].UserCount > data[j].UserCount })

	c.JSON(http.StatusOK, OrgsV2NativeResponse{Data: data, NoOrgMapping: !mapped})
}
