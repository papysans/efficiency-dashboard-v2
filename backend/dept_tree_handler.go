package main

import (
	"encoding/json"
	"fmt"
	"io"
	"kanban/backend/internal/appconfig"
	"log"
	"net/http"
	"sort"
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
	// 120s：根部门 include_children=true 一次返回全量(~11154 人/2.1MB)，对齐 kbcli 超时口径。
	client := &http.Client{Timeout: 120 * time.Second}
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
	baseURL := strings.TrimSpace(appconfig.Cfg.DeptSync.BaseURL)
	if baseURL == "" {
		return "", fmt.Errorf("未配置 dept-sync 服务地址（server 配置 dept_sync.base_url）")
	}
	return baseURL, nil
}

// getDeptTreeV2 GET /api/v2/dept-tree
// 代理 dept-sync /department/tree，按 parent_dept_id 重建为「单根公司子树」后返回。
// dept-sync /department/tree 在内网返回的是【森林】：除真正的公司根（深信服科技股份有限公司），
// 还并列了一堆 parent 链断裂的脏数据孤儿部门（产业研究院/NaaS产品线/资质组…）。
// 这里递归拍平 → 按 parent_dept_id 重建邻接 → 从配置指定的根 dept_id/dept_name 起递归重建嵌套树，
// 只返回 [根节点]（单根数组）：真正属于公司、只是 /tree 没嵌好的部门按 parent 链挂回根下，孤儿/脏数据被排除。
// dept-sync 自带 300s 缓存，这里不再加进程内缓存。
func getDeptTreeV2(c *gin.Context) {
	baseURL, err := deptSyncConfigured()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: err.Error()})
		return
	}
	var resp deptSyncTreeResp
	if err := deptSyncGet(baseURL, appconfig.Cfg.DeptSync.QueryKey, "/department/tree", &resp); err != nil {
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "获取 dept-sync 部门树失败: " + err.Error()})
		return
	}
	if resp.Data == nil {
		resp.Data = []DeptTreeNode{}
	}
	c.JSON(http.StatusOK, rebuildSingleRootTree(resp.Data))
}

// rebuildSingleRootTree 把 dept-sync /department/tree 的森林重建成单根公司子树。
// 步骤：①递归拍平拿全部节点（去重，children 嵌得对不对都先扁平化）；
// ②按 parent_dept_id 建邻接（parent → []child），child 按 order_num 再 dept_id 排序；
// ③找根：优先 cfg.DeptSync.RootDeptId，否则按 RootDeptName 匹配 dept_name；
// ④从根 dept_id 递归重建嵌套树，返回 [根节点]。
// 兜底：找不到根（id/name 都没命中）→ logWarn 并退回原始 data（全部透传），不让页面空白。
func rebuildSingleRootTree(forest []DeptTreeNode) []DeptTreeNode {
	if len(forest) == 0 {
		return []DeptTreeNode{}
	}

	// ① 递归拍平成全部节点的扁平 map（按 dept_id 去重，保留各自 parent_dept_id 等字段，丢弃原 children）。
	flat := make(map[string]DeptTreeNode)
	var order []string // 保持首次出现顺序，便于稳定查根
	var walk func(nodes []DeptTreeNode)
	walk = func(nodes []DeptTreeNode) {
		for _, n := range nodes {
			children := n.Children
			if _, seen := flat[n.DeptId]; !seen {
				n.Children = nil
				flat[n.DeptId] = n
				order = append(order, n.DeptId)
			}
			if len(children) > 0 {
				walk(children)
			}
		}
	}
	walk(forest)

	// ② 邻接表：parent_dept_id → []child dept_id，按 order_num 再 dept_id 排序。
	childIDs := make(map[string][]string)
	for _, id := range order {
		n := flat[id]
		childIDs[n.ParentDeptId] = append(childIDs[n.ParentDeptId], id)
	}
	for parent := range childIDs {
		ids := childIDs[parent]
		sort.SliceStable(ids, func(i, j int) bool {
			a, b := flat[ids[i]], flat[ids[j]]
			if a.OrderNum != b.OrderNum {
				return a.OrderNum < b.OrderNum
			}
			return a.DeptId < b.DeptId
		})
	}

	// ③ 找根 dept_id。
	rootID := ""
	if cfgID := strings.TrimSpace(appconfig.Cfg.DeptSync.RootDeptId); cfgID != "" {
		if _, ok := flat[cfgID]; ok {
			rootID = cfgID
		}
	}
	if rootID == "" {
		rootName := strings.TrimSpace(appconfig.Cfg.DeptSync.RootDeptName)
		if rootName == "" {
			rootName = appconfig.DefaultRootDeptName
		}
		for _, id := range order {
			if flat[id].DeptName == rootName {
				rootID = id
				break
			}
		}
	}

	// ⑤ 兜底：找不到根 → 退回原始 data（全部透传）。
	if rootID == "" {
		log.Printf("[WARN] dept-tree 单根重建失败：未按 root_dept_id(%q)/root_dept_name(%q) 找到根节点，退回原始森林透传",
			appconfig.Cfg.DeptSync.RootDeptId, appconfig.Cfg.DeptSync.RootDeptName)
		return forest
	}

	// ④ 从根递归重建嵌套树（用邻接表把 parent 链够得到根的节点挂回；child_dept_count 用实际子节点数）。
	var build func(id string) DeptTreeNode
	build = func(id string) DeptTreeNode {
		n := flat[id]
		kids := childIDs[id]
		n.Children = make([]DeptTreeNode, 0, len(kids))
		for _, cid := range kids {
			n.Children = append(n.Children, build(cid))
		}
		n.ChildDeptCount = len(kids)
		return n
	}
	return []DeptTreeNode{build(rootID)}
}

// getDeptTreeMembersV2 GET /api/v2/dept-tree/members?dept_id=&startDate=&endDate=
// 代理 dept-sync /department/{dept_id}/users?include_children=true 拿该部门【整棵子树】成员花名册
// （非直属：中间/顶层部门直属常为 0，必须带 include_children 才能看到其下所有人），
// 按 universal_id 左连看板 V2 指标（看板 user_id == universal_id）。
// 没匹配到看板数据的成员也照样返回（has_kanban_data=false，指标置零/nil）。
// summary 按返回的全量成员（含子部门）合计。点根公司会返回全量(~11154 人)，靠 deptSyncGet 的 120s 超时兜住。
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

	// 1. 取该部门【整棵子树】成员花名册（include_children=true，含所有子部门成员）。
	var membersResp deptSyncMembersResp
	if err := deptSyncGet(baseURL, appconfig.Cfg.DeptSync.QueryKey, "/department/"+deptID+"/users?include_children=true", &membersResp); err != nil {
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

	// 有看板活动的成员上提、无活动的下沉；活跃组内按合并需求数→提交数→实际日历分钟降序（活跃在前，
	// 与 aggregateUsersV2 的"活跃在前"口径一致）；无活动组内按真名升序（RealName 为空退用 EmpNo）。
	sort.SliceStable(members, func(i, j int) bool {
		if members[i].HasKanbanData != members[j].HasKanbanData {
			return members[i].HasKanbanData
		}
		if members[i].HasKanbanData {
			if members[i].MergedNeedCount != members[j].MergedNeedCount {
				return members[i].MergedNeedCount > members[j].MergedNeedCount
			}
			if members[i].CommitCount != members[j].CommitCount {
				return members[i].CommitCount > members[j].CommitCount
			}
			return members[i].ActualCalendarMin > members[j].ActualCalendarMin
		}
		ni := members[i].RealName
		if ni == "" {
			ni = members[i].EmpNo
		}
		nj := members[j].RealName
		if nj == "" {
			nj = members[j].EmpNo
		}
		return ni < nj
	})

	c.JSON(http.StatusOK, DeptMembersResponse{Summary: summary, Members: members})
}
