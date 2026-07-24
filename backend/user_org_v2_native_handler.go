package main

import (
	"net/http"
	"sort"
	"strings"
	"time"

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
	CommitCount         int64    `json:"commit_count"`      // commits 直聚，与含硅量同过滤、同 commit_time 窗口
	CommitDiffLines     int64    `json:"commit_diff_lines"` // commits.diff_lines 直聚，与含硅量分母同源
	Cost                float64  `json:"cost"`
	Tokens              int64    `json:"tokens"`
	ConfidenceLimited   bool     `json:"confidence_limited"`
	ConfidenceReason    string   `json:"confidence_reason"`
	AICodeRatio         *float64 `json:"ai_code_ratio"`
	Silica              *float64 `json:"silica"`
	aiCoveredLoc        int64
	totalLocNet         int64
	silicaWeighted      float64
	silicaWeight        int64
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
	AICodeRatio         *float64 `json:"ai_code_ratio"`
}

type OrgsV2NativeResponse struct {
	Data         []OrgV2Row `json:"data"`
	NoOrgMapping bool       `json:"no_org_mapping"`
}

// parseCommitTimeWindow 把 YYYY-MM-DD 的起止日期转成 RFC3339 时间串，供 commits 侧
// （含硅量聚合 / 用户详情 commit 列表）共用，保证同一请求下各处窗口完全一致。
// 解析失败或为空 → 返回空串，调用方按「不限」处理。
func parseCommitTimeWindow(startDate, endDate string) (string, string) {
	var startTime, endTime string
	if start, err := parseStartDate(startDate); err == nil && start != nil {
		startTime = start.Format(time.RFC3339)
	}
	if end, err := parseEndDate(endDate); err == nil && end != nil {
		endTime = end.Format(time.RFC3339)
	}
	return startTime, endTime
}

// aggregateUsersV2 把 user_productivity_v2 的周行按用户聚合；提交数与代码行改由 commits 直聚。
func aggregateUsersV2(startDate, endDate, userID string) ([]UserV2Row, error) {
	agg, err := QueryEfficiencyV2Aggregate(statDB, startDate, endDate, userID)
	if err != nil {
		return nil, err
	}
	startTime, endTime := parseCommitTimeWindow(startDate, endDate)
	aiAggs, err := queryUserNeedAICodeAggs(statDB, startTime, endTime, strings.TrimSpace(userID))
	if err != nil {
		return nil, err
	}
	// 含硅量、提交数与代码行走同一次 commits 直聚（不经 need 边界/caliber 过滤）；
	// AI 代码占比仍是独立的 need 时间窗归因口径。
	silicaAggs, err := queryUserSilicaAggs(statDB, startTime, endTime, strings.TrimSpace(userID))
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
		if agg, ok := aiAggs[uid]; ok {
			r.aiCoveredLoc = agg.AICoveredLoc
			r.totalLocNet = agg.TotalLocNet
			r.AICodeRatio = calcNeedAICodeRatio(agg.AICoveredLoc, agg.TotalLocNet)
		}
		if agg, ok := silicaAggs[uid]; ok {
			r.CommitCount = agg.CommitCount
			r.CommitDiffLines = agg.SilicaWeight
			r.silicaWeighted = agg.SilicaWeighted
			r.silicaWeight = agg.SilicaWeight
			r.Silica = calcSilicaRatio(agg.SilicaWeighted, agg.SilicaWeight)
		}
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
	orderField, orderDir := parseOrderParam(strings.TrimSpace(c.Query("order")))
	if orderField != "" && !isAllowedField(orderField, userSortFields) {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "不支持的排序字段: " + orderField})
		return
	}
	rows, err := aggregateUsersV2(c.Query("startDate"), c.Query("endDate"), "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	sortUserV2Data(rows, orderField, orderDir)
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

	// 该用户的 Commit（按时间倒序取最近 100 条）。
	// ⚠️ 过滤必须与 queryUserSilicaAggs 完全一致（复用 applySilicaCommitFilter，别各写各的）：
	// 顶部「Commit / 代码行」卡与含硅量都走那套口径，本列表若不同口径，就会出现
	// 「下面列了 commit、上面卡显示 0」的自相矛盾（治理排除 / 0 行的 commit 正是此类）。
	cStart, cEnd := parseCommitTimeWindow(startDate, endDate)
	cq := applySilicaCommitFilter(statDB.Model(&models.Commit{})).Where("user_id = ?", userID)
	cq = applySilicaDateFilter(cq, cStart, cEnd)
	_ = cq.Order("commit_time DESC").Limit(100).Find(&resp.Commits).Error

	c.JSON(http.StatusOK, resp)
}

// listOrgsV2Native GET /api/v2/orgs
func listOrgsV2Native(c *gin.Context) {
	if statDB == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: "数据库未连接"})
		return
	}
	orderField, orderDir := parseOrderParam(strings.TrimSpace(c.Query("order")))
	if orderField != "" && !isAllowedField(orderField, orgSortFields) {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "不支持的排序字段: " + orderField})
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
		aiCoveredLoc        int64
		totalLocNet         int64
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
		b.aiCoveredLoc += r.aiCoveredLoc
		b.totalLocNet += r.totalLocNet
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
			AICodeRatio:         calcNeedAICodeRatio(b.aiCoveredLoc, b.totalLocNet),
		})
	}
	sort.SliceStable(data, func(i, j int) bool { return data[i].UserCount > data[j].UserCount })
	sortOrgV2Data(data, orderField, orderDir)

	c.JSON(http.StatusOK, OrgsV2NativeResponse{Data: data, NoOrgMapping: !mapped})
}

// UserNameRow 是 /api/v2/user-names 的一行：看板 user_id(==dept-sync universal_id) → 权威真名 + 工号。
type UserNameRow struct {
	UserId   string `json:"user_id"` // == dept-sync universal_id == 看板 user_id
	RealName string `json:"real_name"`
	EmpNo    string `json:"emp_no"`
}

// getUserNamesV2 GET /api/v2/user-names
// 全量权威映射 user_id→真名(+工号)，供前端把 UUID 解析成「真名(工号)」。
// dept-sync 优先（dept_user.universal_id → real_name+emp_no），未覆盖的用户用 commits 的
// git_user_name 兜底（按 user_id 取出现次数最多的非空 git_user_name）。看板 user_id 即 universal_id。
// 收敛到此单一接口后，前端不再单独拉 commit 明细建映射，永不受分页截断影响。
func getUserNamesV2(c *gin.Context) {
	if statDB == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: "数据库未连接"})
		return
	}
	// 1) dept-sync 权威映射（主）
	var rows []models.DeptUser
	if err := statDB.Where("universal_id <> ''").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "查询 dept_user 失败: " + err.Error()})
		return
	}
	out := make([]UserNameRow, 0, len(rows))
	seen := make(map[string]bool, len(rows))
	for _, r := range rows {
		if seen[r.UniversalId] {
			continue
		}
		seen[r.UniversalId] = true
		out = append(out, UserNameRow{UserId: r.UniversalId, RealName: r.RealName, EmpNo: r.EmpNo})
	}
	// 2) commits 兜底：每个 user_id 取出现次数最多的非空 git_user_name（DISTINCT ON + cnt desc）。
	//    dept-sync 已覆盖的 user_id 跳过，emp_no 留空。
	type commitName struct {
		UserId      string `gorm:"column:user_id"`
		GitUserName string `gorm:"column:git_user_name"`
	}
	var cnames []commitName
	if err := statDB.Raw(`
		SELECT DISTINCT ON (user_id) user_id, git_user_name
		FROM commits
		WHERE user_id <> '' AND git_user_name <> ''
		GROUP BY user_id, git_user_name
		ORDER BY user_id, COUNT(*) DESC, git_user_name
	`).Scan(&cnames).Error; err != nil {
		// 兜底失败不致命：仍返回 dept-sync 部分，避免整接口 500。
		c.JSON(http.StatusOK, out)
		return
	}
	for _, cn := range cnames {
		if seen[cn.UserId] {
			continue
		}
		seen[cn.UserId] = true
		out = append(out, UserNameRow{UserId: cn.UserId, RealName: cn.GitUserName, EmpNo: ""})
	}
	c.JSON(http.StatusOK, out)
}
