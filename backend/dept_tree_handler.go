package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// 组织树页（dept-sync 权威全量树 + API 懒加载）后端代理。
// 树结构与部门成员花名册全部经 HTTP API 取自 dept-sync（不直读其 PG）；
// 成员按 universal_id 左连看板 V2 指标（看板 user_id == dept-sync universal_id，内网 98.6% 命中）。
// 鉴权头 X-Query-Key 来自 server 配置 dept_sync.query_key。client 写法照搬 kbcli/cmd_import_dept.go deptSyncGet。

// dept-sync 数据接口路由前缀（/costrict-dept-info + /api/v1），与 kbcli deptSyncAPIPrefix 一致。
const deptSyncAPIPrefix = "/costrict-dept-info/api/v1"

// ---- dept-sync HTTP API 响应结构（实测契约见 task research/dept-sync-api.md）----

// DeptTreeNode 嵌套部门树节点（透传给前端，字段与 dept-sync /department/tree 对齐）。
type DeptTreeNode struct {
	DeptId         string         `json:"dept_id"`
	DeptName       string         `json:"dept_name"`
	ParentDeptId   string         `json:"parent_dept_id"`
	DeptPath       string         `json:"dept_path"`
	DeptLevel      int            `json:"dept_level"`
	OrderNum       int            `json:"order_num"`
	ChildDeptCount int            `json:"child_dept_count"`
	Status         int            `json:"status"`
	Children       []DeptTreeNode `json:"children"`
}

type deptSyncTreeResp struct {
	Code    string         `json:"code"`
	Success bool           `json:"success"`
	Data    []DeptTreeNode `json:"data"`
}

// deptSyncMemberNode 是 GET /department/{dept_id}/users 返回的一名直属成员。
// user_id 是工号；username 是真名；universal_id 用于对到看板用户（== 看板 user_id）。
type deptSyncMemberNode struct {
	UserId      string `json:"user_id"`
	Username    string `json:"username"`
	UniversalId string `json:"universal_id"`
	DeptId      string `json:"dept_id"`
	DeptName    string `json:"dept_name"`
	Position    string `json:"position"`
	IsMain      int    `json:"is_main"`
	Status      int    `json:"status"`
}

type deptSyncMembersResp struct {
	Code    string               `json:"code"`
	Success bool                 `json:"success"`
	Data    []deptSyncMemberNode `json:"data"`
}

// DeptMember 是 /api/v2/dept-tree/members 返回的一行：dept-sync 花名册 + 左连的看板 V2 指标。
// 没匹配到看板数据的成员也返回（has_kanban_data=false，指标为零值/nil）。
type DeptMember struct {
	UniversalId         string   `json:"universal_id"`
	RealName            string   `json:"real_name"`
	EmpNo               string   `json:"emp_no"`
	Position            string   `json:"position"`
	IsMain              int      `json:"is_main"`
	HasKanbanData       bool     `json:"has_kanban_data"`
	MergedNeedCount     int64    `json:"merged_need_count"`
	ActualCalendarMin   float64  `json:"actual_calendar_min"`
	BaselineCalendarMin float64  `json:"baseline_calendar_min"`
	CalendarRatio       *float64 `json:"calendar_ratio"`
	WorkRatio           *float64 `json:"work_ratio"`
	CommitCount         int64    `json:"commit_count"`
	CommitDiffLines     int64    `json:"commit_diff_lines"`
	Cost                float64  `json:"cost"`
}

// DeptMembersSummary 该部门直属成员的合计（仅匹配到看板数据的计入指标合计；提效比按合计基线/实际重算）。
type DeptMembersSummary struct {
	DeptId              string   `json:"dept_id"`
	MemberCount         int      `json:"member_count"`
	KanbanMemberCount   int      `json:"kanban_member_count"`
	MergedNeedCount     int64    `json:"merged_need_count"`
	ActualCalendarMin   float64  `json:"actual_calendar_min"`
	BaselineCalendarMin float64  `json:"baseline_calendar_min"`
	CalendarRatio       *float64 `json:"calendar_ratio"`
	WorkRatio           *float64 `json:"work_ratio"`
	CommitCount         int64    `json:"commit_count"`
	CommitDiffLines     int64    `json:"commit_diff_lines"`
	Cost                float64  `json:"cost"`
}

type DeptMembersResponse struct {
	Summary DeptMembersSummary `json:"summary"`
	Members []DeptMember       `json:"members"`
}

// deptSyncGet 调 dept-sync 数据接口（带 X-Query-Key），解析统一包到 out。
// 照搬 kbcli/cmd_import_dept.go 的同名实现（backend 不依赖 kbcli 包，故各自一份）。
func deptSyncGet(baseURL, queryKey, path string, out interface{}) error {
	url := strings.TrimRight(baseURL, "/") + deptSyncAPIPrefix + path
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Query-Key", queryKey)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("请求 dept-sync 失败 [%s]: %w", path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("dept-sync 返回非 200 [%s]: %d %s", path, resp.StatusCode, string(body))
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("解析 dept-sync 响应失败 [%s]: %w", path, err)
	}
	return nil
}

// deptSyncConfigured 校验 dept-sync 地址是否配置；未配置返回明确错误供 handler 回报。
func deptSyncConfigured() (string, error) {
	baseURL := strings.TrimSpace(appConfig.DeptSync.BaseURL)
	if baseURL == "" {
		return "", fmt.Errorf("未配置 dept-sync 服务地址（server 配置 dept_sync.base_url）")
	}
	return baseURL, nil
}

// getDeptTreeV2 GET /api/v2/dept-tree
// 代理 dept-sync /department/tree，返回全量嵌套部门树。
// dept-sync 自带 300s 缓存，这里简单透传，不再加进程内缓存。
func getDeptTreeV2(c *gin.Context) {
	baseURL, err := deptSyncConfigured()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: err.Error()})
		return
	}
	var resp deptSyncTreeResp
	if err := deptSyncGet(baseURL, appConfig.DeptSync.QueryKey, "/department/tree", &resp); err != nil {
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "获取 dept-sync 部门树失败: " + err.Error()})
		return
	}
	if resp.Data == nil {
		resp.Data = []DeptTreeNode{}
	}
	c.JSON(http.StatusOK, resp.Data)
}

// getDeptTreeMembersV2 GET /api/v2/dept-tree/members?dept_id=&startDate=&endDate=
// 代理 dept-sync /department/{dept_id}/users 拿该部门「直属」成员花名册，
// 按 universal_id 左连看板 V2 指标（看板 user_id == universal_id）。
// 没匹配到看板数据的成员也照样返回（has_kanban_data=false，指标置零/nil）。
func getDeptTreeMembersV2(c *gin.Context) {
	if statDB == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: "数据库未连接"})
		return
	}
	deptID := strings.TrimSpace(c.Query("dept_id"))
	if deptID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "dept_id 不能为空"})
		return
	}
	baseURL, err := deptSyncConfigured()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: err.Error()})
		return
	}

	// 1. 取该部门直属成员花名册（非递归）。
	var membersResp deptSyncMembersResp
	if err := deptSyncGet(baseURL, appConfig.DeptSync.QueryKey, "/department/"+deptID+"/users", &membersResp); err != nil {
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "获取 dept-sync 部门成员失败: " + err.Error()})
		return
	}

	// 2. 拉看板全量 V2 指标，按 user_id 建 map（user_id == universal_id）。
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	v2rows, err := aggregateUsersV2(startDate, endDate, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "聚合看板 V2 指标失败: " + err.Error()})
		return
	}
	rowByUser := make(map[string]UserV2Row, len(v2rows))
	for _, r := range v2rows {
		rowByUser[r.UserId] = r
	}

	// 3. 左连：每个 dept-sync 成员按 universal_id 查看板指标；无则零值（仍列出）。
	members := make([]DeptMember, 0, len(membersResp.Data))
	summary := DeptMembersSummary{DeptId: deptID}
	var totalActualWork, totalBaselineWork float64
	for _, src := range membersResp.Data {
		m := DeptMember{
			UniversalId: src.UniversalId,
			RealName:    src.Username,
			EmpNo:       src.UserId,
			Position:    src.Position,
			IsMain:      src.IsMain,
		}
		if src.UniversalId != "" {
			if row, ok := rowByUser[src.UniversalId]; ok {
				m.HasKanbanData = true
				m.MergedNeedCount = row.MergedNeedCount
				m.ActualCalendarMin = row.ActualCalendarMin
				m.BaselineCalendarMin = row.BaselineCalendarMin
				m.CalendarRatio = row.CalendarRatio
				m.WorkRatio = row.WorkRatio
				m.CommitCount = row.CommitCount
				m.CommitDiffLines = row.CommitDiffLines
				m.Cost = row.Cost

				summary.KanbanMemberCount++
				summary.MergedNeedCount += row.MergedNeedCount
				summary.ActualCalendarMin += row.ActualCalendarMin
				summary.BaselineCalendarMin += row.BaselineCalendarMin
				summary.CommitCount += row.CommitCount
				summary.CommitDiffLines += row.CommitDiffLines
				summary.Cost += row.Cost
				totalActualWork += row.ActualWorkMin
				totalBaselineWork += row.BaselineWorkMin
			}
		}
		members = append(members, m)
	}
	summary.MemberCount = len(members)
	// 合计提效比按汇总基线/实际重算（小数口径，与 listOrgsV2Native / aggregateUsersV2 一致）。
	summary.CalendarRatio = efficiencyV2Ratio(summary.BaselineCalendarMin, summary.ActualCalendarMin)
	summary.WorkRatio = efficiencyV2Ratio(totalBaselineWork, totalActualWork)

	c.JSON(http.StatusOK, DeptMembersResponse{Summary: summary, Members: members})
}
