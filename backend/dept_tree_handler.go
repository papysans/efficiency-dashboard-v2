package main

import (
	"encoding/json"
	"fmt"
	"io"
	"kanban/backend/internal/appconfig"
	"kanban/core/models"
	"kanban/core/utils"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
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
	DeptId              string   `json:"dept_id"` // 成员直属部门 id（成本树按此归桶算各部门直属，避免逐部门 N× members）
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
	AICodeRatio         *float64 `json:"ai_code_ratio"`
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
	AICodeRatio         *float64 `json:"ai_code_ratio"`
}

type DeptMembersResponse struct {
	Summary DeptMembersSummary `json:"summary"`
	Members []DeptMember       `json:"members"`
}

// DeptRankingItem 一级子部门的整棵子树汇总（复用 members 接口的 DeptMembersSummary 口径）。
type DeptRankingItem struct {
	DeptId   string             `json:"dept_id"`
	DeptName string             `json:"dept_name"`
	Summary  DeptMembersSummary `json:"summary"`
}

// DeptRankingResponse /api/v2/dept-tree/ranking 顶层响应：parent 的各直接子部门汇总排行。
// self 为 parent 节点【自身整棵子树】的守恒汇总（同 items 口径，Σbaseline/Σactual），
// 供前端给 parent 根节点本身挂提效比/费用（OrgTree 公司根节点用），不破坏 items 列表结构。
type DeptRankingResponse struct {
	ParentDeptId string              `json:"parent_dept_id"`
	Self         *DeptMembersSummary `json:"self"`
	Items        []DeptRankingItem   `json:"items"`
}

// DeptTreeNodeWithSummary /api/v2/dept-tree/overview 节点：树结构 + 本节点整棵子树守恒提效汇总
// （一次性返回，替代前端组织树逐展开节点 N× 调 /ranking 的 N+1）。
type DeptTreeNodeWithSummary struct {
	DeptId         string                    `json:"dept_id"`
	DeptName       string                    `json:"dept_name"`
	ParentDeptId   string                    `json:"parent_dept_id"`
	DeptPath       string                    `json:"dept_path"`
	DeptLevel      int                       `json:"dept_level"`
	OrderNum       int                       `json:"order_num"`
	ChildDeptCount int                       `json:"child_dept_count"`
	Status         int                       `json:"status"`
	Summary        DeptMembersSummary        `json:"summary"`
	Children       []DeptTreeNodeWithSummary `json:"children"`
}

// DeptOverviewResponse /api/v2/dept-tree/overview 顶层响应：森林（多根）+ 每节点子树守恒汇总。
type DeptOverviewResponse struct {
	Nodes []DeptTreeNodeWithSummary `json:"nodes"`
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
	tree, err := cachedRebuiltDeptTree(baseURL, appconfig.Cfg.DeptSync.QueryKey)
	if err != nil {
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "获取 dept-sync 部门树失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, tree)
}

// rebuildSingleRootTree 把 dept-sync /department/tree 的森林重建成嵌套树。
// 步骤：①递归拍平拿全部节点（去重，children 嵌得对不对都先扁平化）；
// ②按 parent_dept_id 建邻接（parent → []child），child 按 order_num 再 dept_id 排序；
// ③按配置找根：RootDeptId 命中 → 该 ID；否则 RootDeptName 命中 → 该名；都留空/未命中 → 不指定根；
// ④嵌套树构建器；⑤配置命中 → 单根子树 [root]（过滤 parent 链断裂的孤儿脏数据）；
//   留空 → 全森林（所有「parent 空/悬挂」的顶层根各建一棵，按 order_num 排）。
// 语义（需求1）：root_dept_id/root_dept_name 都留空 = 展示全部数据（多根森林，含 dept-sync 孤儿部门）。
// 兜底：连一个顶层根都判不出 → logWarn 退回原始 data 透传，不让页面空白。
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

	// ③ 找配置指定的根 dept_id（仅看 RootDeptId / RootDeptName；都留空/未命中 → rootID 保持空，走全森林）。
	rootID := ""
	if cfgID := strings.TrimSpace(appconfig.Cfg.DeptSync.RootDeptId); cfgID != "" {
		if _, ok := flat[cfgID]; ok {
			rootID = cfgID
		}
	}
	if rootID == "" {
		if rootName := strings.TrimSpace(appconfig.Cfg.DeptSync.RootDeptName); rootName != "" {
			for _, id := range order {
				if flat[id].DeptName == rootName {
					rootID = id
					break
				}
			}
		}
	}

	// ④ 嵌套树构建器（用邻接表把 parent 链够得到根的节点挂回；child_dept_count 用实际子节点数）。
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

	// ⑤ 配置命中 → 单根子树 [root]（过滤孤儿脏数据）。
	if rootID != "" {
		return []DeptTreeNode{build(rootID)}
	}

	// ⑤' 留空 → 全森林：收集所有「parent 空/悬挂（父不在表内）」的顶层根，各建一棵，按 order_num→dept_id 排序。
	// 取舍：dept-sync /tree 含 parent 链断裂的孤儿脏数据，留空态会把它们也作为根列出（即「展示全部数据」）；
	// 要过滤孤儿请配置 root_dept_id / root_dept_name 锁单根。
	roots := make([]string, 0)
	for _, id := range order {
		p := strings.TrimSpace(flat[id].ParentDeptId)
		if _, parentExists := flat[p]; p == "" || !parentExists {
			roots = append(roots, id)
		}
	}
	if len(roots) == 0 {
		log.Printf("[WARN] dept-tree 重建未找到任何顶层根（root_dept_id=%q/root_dept_name=%q 均未命中且无悬挂根），退回原始森林透传",
			appconfig.Cfg.DeptSync.RootDeptId, appconfig.Cfg.DeptSync.RootDeptName)
		return forest
	}
	sort.SliceStable(roots, func(i, j int) bool {
		a, b := flat[roots[i]], flat[roots[j]]
		if a.OrderNum != b.OrderNum {
			return a.OrderNum < b.OrderNum
		}
		return a.DeptId < b.DeptId
	})
	out := make([]DeptTreeNode, 0, len(roots))
	for _, id := range roots {
		out = append(out, build(id))
	}
	return out
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

	// 2. 拉看板全量 V2 指标，按 user_id 建 map（user_id == universal_id）。走窗口缓存（同页多次复用一次聚合）。
	v2rows, err := cachedAggregateUsersV2(c.Query("startDate"), c.Query("endDate"))
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
	var totalAICoveredLoc, totalLocNet int64
	for _, src := range membersResp.Data {
		m := DeptMember{
			UniversalId: src.UniversalId,
			RealName:    src.Username,
			EmpNo:       src.UserId,
			DeptId:      src.DeptId,
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
				m.AICodeRatio = row.AICodeRatio

				summary.KanbanMemberCount++
				summary.MergedNeedCount += row.MergedNeedCount
				summary.ActualCalendarMin += row.ActualCalendarMin
				summary.BaselineCalendarMin += row.BaselineCalendarMin
				summary.CommitCount += row.CommitCount
				summary.CommitDiffLines += row.CommitDiffLines
				summary.Cost += row.Cost
				totalActualWork += row.ActualWorkMin
				totalBaselineWork += row.BaselineWorkMin
				totalAICoveredLoc += row.aiCoveredLoc
				totalLocNet += row.totalLocNet
			}
		}
		members = append(members, m)
	}
	summary.MemberCount = len(members)
	// 合计提效比按汇总基线/实际重算（小数口径，与 listOrgsV2Native / aggregateUsersV2 一致）。
	summary.CalendarRatio = efficiencyV2Ratio(summary.BaselineCalendarMin, summary.ActualCalendarMin)
	summary.WorkRatio = efficiencyV2Ratio(totalBaselineWork, totalActualWork)
	summary.AICodeRatio = calcNeedAICodeRatio(totalAICoveredLoc, totalLocNet)

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

// findDeptNode 在单根嵌套树里按 dept_id DFS 定位节点（命中返回该节点指针，未命中 nil）。
func findDeptNode(nodes []DeptTreeNode, id string) *DeptTreeNode {
	for i := range nodes {
		if nodes[i].DeptId == id {
			return &nodes[i]
		}
		if found := findDeptNode(nodes[i].Children, id); found != nil {
			return found
		}
	}
	return nil
}

// collectDescendantDeptIDs 收集 node 整棵子树（含自身）的全部 dept_id。
func collectDescendantDeptIDs(node DeptTreeNode, acc map[string]struct{}) {
	acc[node.DeptId] = struct{}{}
	for _, child := range node.Children {
		collectDescendantDeptIDs(child, acc)
	}
}

// getDeptRankingV2 GET /api/v2/dept-tree/ranking?parent_dept_id=&startDate=&endDate=
// 返回 parent_dept_id 的【直接子部门】各自整棵子树的汇总指标（复用 members 接口的 DeptMembersSummary 口径），
// 供首页部门 PK 排行一次性消费——替代「逐子部门各调一次 members（每次都跑全表 aggregateUsersV2）」的 N× 全表聚合。
//
// 性能要点（一次聚合替代 N×）：
//   - 只调一次 dept-sync 花名册（GET /department/{parentId}/users?include_children=true，parent 为根即全公司全量）；
//   - 只调一次 aggregateUsersV2(startDate, endDate, "") 建 rowByUser map；
//   - 把每名成员按其直属 dept_id 归到「一级祖先目标子部门」的桶，累加进各桶 summary（累加/重算逻辑与 members 接口完全一致）。
//
// parent_dept_id 为空时默认用配置根（rebuildSingleRootTree 的单根节点），即排「全公司一级部门」。
func getDeptRankingV2(c *gin.Context) {
	if statDB == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: "数据库未连接"})
		return
	}
	baseURL, err := deptSyncConfigured()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: err.Error()})
		return
	}

	// 1. 取单根嵌套部门树（缓存）。
	tree, err := cachedRebuiltDeptTree(baseURL, appconfig.Cfg.DeptSync.QueryKey)
	if err != nil {
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "获取 dept-sync 部门树失败: " + err.Error()})
		return
	}

	// 全森林顶层排行（需求1 留空多根态）：parent 空且非单根 → 森林各根作为 items，self = 全森林守恒汇总。
	parentDeptID := strings.TrimSpace(c.Query("parent_dept_id"))
	if parentDeptID == "" && len(tree) != 1 {
		resp, ferr := deptRankingForForest(baseURL, c.Query("startDate"), c.Query("endDate"), tree)
		if ferr != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "获取部门排行失败: " + ferr.Error()})
			return
		}
		c.JSON(http.StatusOK, resp)
		return
	}

	// 定位 parent 节点：空 → 取单根；否则按 dept_id DFS 查。
	var parent *DeptTreeNode
	if parentDeptID == "" {
		if len(tree) > 0 {
			parent = &tree[0]
			parentDeptID = parent.DeptId
		}
	} else {
		parent = findDeptNode(tree, parentDeptID)
	}
	if parent == nil {
		// parent 不存在（dept_id 在树里找不到）：返回空排行且无 self（前端显示「该层级暂无可计入部门数据」）。
		c.JSON(http.StatusOK, DeptRankingResponse{ParentDeptId: parentDeptID, Items: []DeptRankingItem{}})
		return
	}
	// 注意：parent 是叶子（无子部门）时不再提前返回——下方按 parent 整棵子树(即它自己+其直属成员)
	// 累加 selfBucket、重算守恒比值，Items 自然为空（无一级子部门桶）。前端聚焦叶子部门时仍能拿到
	// self.cost / self 提效比，成本卡不再回落「建设中」。仅当 parent==nil 才无 self。

	// 2. 预计算：descendantDeptId -> 目标一级子部门 dept_id（含子部门自身），用于把成员按一级祖先归桶。
	bucketByDeptID := make(map[string]string)
	for _, child := range parent.Children {
		descendants := make(map[string]struct{})
		collectDescendantDeptIDs(child, descendants)
		for did := range descendants {
			bucketByDeptID[did] = child.DeptId
		}
	}
	// parent 自身整棵子树的全部 dept_id（含 parent 本级、含直属 parent 而不归任何一级子部门的成员），
	// 用于第 6 步另外累加一个「parent 自身整树 rollup」(self)。
	parentDescendants := make(map[string]struct{})
	collectDescendantDeptIDs(*parent, parentDescendants)

	// 3. 只调一次 dept-sync 花名册：parent 整棵子树全量成员（parent 为根即全公司全量，~11154 人，靠 120s 超时兜住）。
	var membersResp deptSyncMembersResp
	if err := deptSyncGet(baseURL, appconfig.Cfg.DeptSync.QueryKey, "/department/"+parentDeptID+"/users?include_children=true", &membersResp); err != nil {
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "获取 dept-sync 部门成员失败: " + err.Error()})
		return
	}

	// 4. 只调一次 aggregateUsersV2（窗口缓存），建 rowByUser map（user_id == universal_id）。
	v2rows, err := cachedAggregateUsersV2(c.Query("startDate"), c.Query("endDate"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "聚合看板 V2 指标失败: " + err.Error()})
		return
	}
	rowByUser := make(map[string]UserV2Row, len(v2rows))
	for _, r := range v2rows {
		rowByUser[r.UserId] = r
	}

	// 5. 每个目标子部门一个累加桶（summary + 提效比/AI占比重算所需的中间量）。
	type deptBucket struct {
		summary           DeptMembersSummary
		totalActualWork   float64
		totalBaselineWork float64
		totalAICoveredLoc int64
		totalLocNet       int64
	}
	buckets := make(map[string]*deptBucket, len(parent.Children))
	for _, child := range parent.Children {
		buckets[child.DeptId] = &deptBucket{summary: DeptMembersSummary{DeptId: child.DeptId}}
	}
	// parent 自身整树 rollup 桶（覆盖 parent 整棵子树全部成员，含直属 parent 本级、不归任何一级子部门者）。
	selfBucket := &deptBucket{summary: DeptMembersSummary{DeptId: parentDeptID}}

	// 6. 遍历花名册：按成员直属 dept_id 定位一级祖先目标子部门，左连看板指标累加（逻辑与 members 接口一致）。
	//    同一名成员若在 parent 整棵子树内，再额外累加进 self 桶（口径与各子部门桶完全一致）。
	for _, src := range membersResp.Data {
		bucketID, hasBucket := bucketByDeptID[src.DeptId]
		_, inParentSubtree := parentDescendants[src.DeptId]
		if !hasBucket && !inParentSubtree {
			continue // 既不在任何一级子部门子树内，也不在 parent 整棵子树内 → 跳过。
		}
		var b *deptBucket
		if hasBucket {
			b = buckets[bucketID]
			b.summary.MemberCount++
		}
		if inParentSubtree {
			selfBucket.summary.MemberCount++
		}
		if src.UniversalId == "" {
			continue
		}
		row, ok := rowByUser[src.UniversalId]
		if !ok {
			continue
		}
		accumulateDeptBucket := func(t *deptBucket) {
			t.summary.KanbanMemberCount++
			t.summary.MergedNeedCount += row.MergedNeedCount
			t.summary.ActualCalendarMin += row.ActualCalendarMin
			t.summary.BaselineCalendarMin += row.BaselineCalendarMin
			t.summary.CommitCount += row.CommitCount
			t.summary.CommitDiffLines += row.CommitDiffLines
			t.summary.Cost += row.Cost
			t.totalActualWork += row.ActualWorkMin
			t.totalBaselineWork += row.BaselineWorkMin
			t.totalAICoveredLoc += row.aiCoveredLoc
			t.totalLocNet += row.totalLocNet
		}
		if hasBucket {
			accumulateDeptBucket(b)
		}
		if inParentSubtree {
			accumulateDeptBucket(selfBucket)
		}
	}

	// 7. 重算各桶提效比/AI占比（小数口径，与 members 接口一致），按部门树 order 输出（稳定序，前端再排）。
	items := make([]DeptRankingItem, 0, len(parent.Children))
	for _, child := range parent.Children {
		b := buckets[child.DeptId]
		b.summary.CalendarRatio = efficiencyV2Ratio(b.summary.BaselineCalendarMin, b.summary.ActualCalendarMin)
		b.summary.WorkRatio = efficiencyV2Ratio(b.totalBaselineWork, b.totalActualWork)
		b.summary.AICodeRatio = calcNeedAICodeRatio(b.totalAICoveredLoc, b.totalLocNet)
		items = append(items, DeptRankingItem{
			DeptId:   child.DeptId,
			DeptName: child.DeptName,
			Summary:  b.summary,
		})
	}

	// parent 自身整树 rollup（守恒口径，同 items）：给前端把 parent 根节点也挂上提效比/费用。
	selfBucket.summary.CalendarRatio = efficiencyV2Ratio(selfBucket.summary.BaselineCalendarMin, selfBucket.summary.ActualCalendarMin)
	selfBucket.summary.WorkRatio = efficiencyV2Ratio(selfBucket.totalBaselineWork, selfBucket.totalActualWork)
	selfBucket.summary.AICodeRatio = calcNeedAICodeRatio(selfBucket.totalAICoveredLoc, selfBucket.totalLocNet)
	self := selfBucket.summary

	c.JSON(http.StatusOK, DeptRankingResponse{ParentDeptId: parentDeptID, Self: &self, Items: items})
}

// ───────────────────────────── Overview（整棵森林 + 每节点子树守恒汇总，一次性返回） ─────────────────────────────

// deptBucketAccum 部门桶累加器：summary + 提效比/AI占比守恒重算所需的工作量/LOC 中间量（口径同 getDeptRankingV2 内联桶）。
type deptBucketAccum struct {
	summary           DeptMembersSummary
	totalActualWork   float64
	totalBaselineWork float64
	totalAICoveredLoc int64
	totalLocNet       int64
}

// accumulate 把一名命中看板数据的成员指标累加进桶（口径与 members/ranking 累加完全一致）。
func (a *deptBucketAccum) accumulate(row UserV2Row) {
	a.summary.KanbanMemberCount++
	a.summary.MergedNeedCount += row.MergedNeedCount
	a.summary.ActualCalendarMin += row.ActualCalendarMin
	a.summary.BaselineCalendarMin += row.BaselineCalendarMin
	a.summary.CommitCount += row.CommitCount
	a.summary.CommitDiffLines += row.CommitDiffLines
	a.summary.Cost += row.Cost
	a.totalActualWork += row.ActualWorkMin
	a.totalBaselineWork += row.BaselineWorkMin
	a.totalAICoveredLoc += row.aiCoveredLoc
	a.totalLocNet += row.totalLocNet
}

// merge 把另一桶 b 的全部原始量并入 a（子树/森林汇总用）；不并 ratio 指针（finalize 由并完原始量守恒重算）。
func (a *deptBucketAccum) merge(b *deptBucketAccum) {
	a.summary.MemberCount += b.summary.MemberCount
	a.summary.KanbanMemberCount += b.summary.KanbanMemberCount
	a.summary.MergedNeedCount += b.summary.MergedNeedCount
	a.summary.ActualCalendarMin += b.summary.ActualCalendarMin
	a.summary.BaselineCalendarMin += b.summary.BaselineCalendarMin
	a.summary.CommitCount += b.summary.CommitCount
	a.summary.CommitDiffLines += b.summary.CommitDiffLines
	a.summary.Cost += b.summary.Cost
	a.totalActualWork += b.totalActualWork
	a.totalBaselineWork += b.totalBaselineWork
	a.totalAICoveredLoc += b.totalAICoveredLoc
	a.totalLocNet += b.totalLocNet
}

// finalize 守恒重算各比值（小数口径，与 members/ranking 一致）。
func (a *deptBucketAccum) finalize() {
	a.summary.CalendarRatio = efficiencyV2Ratio(a.summary.BaselineCalendarMin, a.summary.ActualCalendarMin)
	a.summary.WorkRatio = efficiencyV2Ratio(a.totalBaselineWork, a.totalActualWork)
	a.summary.AICodeRatio = calcNeedAICodeRatio(a.totalAICoveredLoc, a.totalLocNet)
}

// computeDeptSubtreeAccums 一次遍历算出森林中【每个 dept 节点整棵子树】的守恒累加器（ratio 已 finalize）。
// ① 按成员直属 dept_id 归「直接桶」（universal_id 命中看板指标即累加，未命中仍计 MemberCount）；
// ② 后序 DFS：节点子树累加器 = 自身直接桶 + Σ 子节点子树；③ 末了 finalize。口径与 members/ranking 完全一致。
func computeDeptSubtreeAccums(tree []DeptTreeNode, members []deptSyncMemberNode, rowByUser map[string]UserV2Row) map[string]*deptBucketAccum {
	direct := make(map[string]*deptBucketAccum)
	getDirect := func(id string) *deptBucketAccum {
		a := direct[id]
		if a == nil {
			a = &deptBucketAccum{summary: DeptMembersSummary{DeptId: id}}
			direct[id] = a
		}
		return a
	}
	for _, src := range members {
		a := getDirect(src.DeptId)
		a.summary.MemberCount++
		if src.UniversalId != "" {
			if row, ok := rowByUser[src.UniversalId]; ok {
				a.accumulate(row)
			}
		}
	}
	subtree := make(map[string]*deptBucketAccum)
	var walk func(n DeptTreeNode) *deptBucketAccum
	walk = func(n DeptTreeNode) *deptBucketAccum {
		acc := &deptBucketAccum{summary: DeptMembersSummary{DeptId: n.DeptId}}
		if d, ok := direct[n.DeptId]; ok {
			acc.merge(d)
		}
		for _, ch := range n.Children {
			acc.merge(walk(ch))
		}
		acc.finalize()
		subtree[n.DeptId] = acc
		return acc
	}
	for i := range tree {
		walk(tree[i])
	}
	return subtree
}

// getDeptOverviewV2 GET /api/v2/dept-tree/overview?startDate=&endDate=
// 一次性返回整棵森林 + 每节点整棵子树守恒提效汇总，替代前端组织树逐展开节点 N× 调 /ranking（N+1 → 1）。
// 输入全走进程内短缓存（树/全员花名册/aggregateUsersV2），重计算只在 TTL 过期后首次发生。
func getDeptOverviewV2(c *gin.Context) {
	if statDB == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: "数据库未连接"})
		return
	}
	baseURL, err := deptSyncConfigured()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: err.Error()})
		return
	}
	tree, err := cachedRebuiltDeptTree(baseURL, appconfig.Cfg.DeptSync.QueryKey)
	if err != nil {
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "获取 dept-sync 部门树失败: " + err.Error()})
		return
	}
	if len(tree) == 0 {
		c.JSON(http.StatusOK, DeptOverviewResponse{Nodes: []DeptTreeNodeWithSummary{}})
		return
	}
	members, err := cachedAllDeptMembers(baseURL, appconfig.Cfg.DeptSync.QueryKey, tree)
	if err != nil {
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "获取 dept-sync 部门成员失败: " + err.Error()})
		return
	}
	v2rows, err := cachedAggregateUsersV2(c.Query("startDate"), c.Query("endDate"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "聚合看板 V2 指标失败: " + err.Error()})
		return
	}
	rowByUser := make(map[string]UserV2Row, len(v2rows))
	for _, r := range v2rows {
		rowByUser[r.UserId] = r
	}

	accs := computeDeptSubtreeAccums(tree, members, rowByUser)

	var attach func(n DeptTreeNode) DeptTreeNodeWithSummary
	attach = func(n DeptTreeNode) DeptTreeNodeWithSummary {
		out := DeptTreeNodeWithSummary{
			DeptId:         n.DeptId,
			DeptName:       n.DeptName,
			ParentDeptId:   n.ParentDeptId,
			DeptPath:       n.DeptPath,
			DeptLevel:      n.DeptLevel,
			OrderNum:       n.OrderNum,
			ChildDeptCount: n.ChildDeptCount,
			Status:         n.Status,
			Summary:        DeptMembersSummary{DeptId: n.DeptId},
			Children:       make([]DeptTreeNodeWithSummary, 0, len(n.Children)),
		}
		if a, ok := accs[n.DeptId]; ok {
			out.Summary = a.summary
		}
		for _, ch := range n.Children {
			out.Children = append(out.Children, attach(ch))
		}
		return out
	}
	nodes := make([]DeptTreeNodeWithSummary, 0, len(tree))
	for i := range tree {
		nodes = append(nodes, attach(tree[i]))
	}
	c.JSON(http.StatusOK, DeptOverviewResponse{Nodes: nodes})
}

// deptRankingForForest 全森林顶层排行（需求1 留空多根态）：森林各根作为 items（各根整棵子树守恒汇总），
// self = 全森林守恒汇总。复用 computeDeptSubtreeAccums 一次算全树。
func deptRankingForForest(baseURL, startDate, endDate string, tree []DeptTreeNode) (DeptRankingResponse, error) {
	res := DeptRankingResponse{ParentDeptId: "", Items: []DeptRankingItem{}}
	members, err := cachedAllDeptMembers(baseURL, appconfig.Cfg.DeptSync.QueryKey, tree)
	if err != nil {
		return res, err
	}
	v2rows, err := cachedAggregateUsersV2(startDate, endDate)
	if err != nil {
		return res, err
	}
	rowByUser := make(map[string]UserV2Row, len(v2rows))
	for _, r := range v2rows {
		rowByUser[r.UserId] = r
	}
	accs := computeDeptSubtreeAccums(tree, members, rowByUser)
	items := make([]DeptRankingItem, 0, len(tree))
	self := &deptBucketAccum{summary: DeptMembersSummary{DeptId: ""}}
	for i := range tree {
		root := tree[i]
		a, ok := accs[root.DeptId]
		if !ok {
			continue
		}
		items = append(items, DeptRankingItem{DeptId: root.DeptId, DeptName: root.DeptName, Summary: a.summary})
		self.merge(a)
	}
	self.finalize()
	s := self.summary
	res.Self = &s
	res.Items = items
	return res, nil
}

// getDeptTreeTrendV2 GET /api/v2/dept-tree/trend?dept_id=&startDate=&endDate=
// 返回该部门【整棵子树成员】按 ISO 周的趋势点数组，复用 repo_project_trend.go 的 EntityTrendPoint/EntityTrendResponse，
// 同时给前端「效率」「贡献」两个占位用（efficiency_pct + need_count/commit_count/diff_lines）。
//
// 数据源：user_productivity_v2（user×week 周表，已是 ISO 周聚合，列含 user_id + week_start +
// baseline_calendar_min/actual_calendar_min/merged_need_count/commit_count/commit_diff_lines），
// 直接按 user_id IN (universal_ids) 过滤 + GROUP BY week_start，无需回退基表。
//   - efficiency_pct = utils.CalcEfficiencyRatio(Σbaseline_calendar, Σactual_calendar)（gain% 百分比口径，
//     与 repo-trend/project-trend 统一，前端直接画，绝不再 ×100）。
//   - need_count = Σmerged_need_count；commit_count = Σcommit_count；diff_lines = Σcommit_diff_lines。
//   - loc：周表无 LOC 列（total_loc_net 在 needs 行级），故 Loc 恒 0（部门趋势不消费 LOC）。
//   - cost：本端点不计费用（费用走部门 ranking 的 self/items），Cost 恒 0。
// startDate/endDate（camelCase，与全站一致）按 week_start 做窗口过滤（同 QueryEfficiencyV2Aggregate）。
// dept_id 空 → 默认公司根（全公司整树周趋势，org 贡献聚合态用）。
// dept-sync 不可达 → 502；花名册为空或无 universal_id → {data:[]}（不报错）。
func getDeptTreeTrendV2(c *gin.Context) {
	if statDB == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: "数据库未连接"})
		return
	}
	baseURL, err := deptSyncConfigured()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: err.Error()})
		return
	}

	// 收集目标成员花名册：dept_id 空 → 全公司（所有森林根并集，缓存；需求1 留空多根态也覆盖全部根）；
	// 否则该部门整棵子树（HTTP include_children）。
	deptID := strings.TrimSpace(c.Query("dept_id"))
	var membersData []deptSyncMemberNode
	if deptID == "" {
		tree, terr := cachedRebuiltDeptTree(baseURL, appconfig.Cfg.DeptSync.QueryKey)
		if terr != nil {
			c.JSON(http.StatusBadGateway, ErrorResponse{Error: "获取 dept-sync 部门树失败: " + terr.Error()})
			return
		}
		if len(tree) == 0 {
			c.JSON(http.StatusOK, EntityTrendResponse{Data: []EntityTrendPoint{}})
			return
		}
		membersData, terr = cachedAllDeptMembers(baseURL, appconfig.Cfg.DeptSync.QueryKey, tree)
		if terr != nil {
			c.JSON(http.StatusBadGateway, ErrorResponse{Error: "获取 dept-sync 部门成员失败: " + terr.Error()})
			return
		}
	} else {
		var membersResp deptSyncMembersResp
		if err := deptSyncGet(baseURL, appconfig.Cfg.DeptSync.QueryKey, "/department/"+deptID+"/users?include_children=true", &membersResp); err != nil {
			c.JSON(http.StatusBadGateway, ErrorResponse{Error: "获取 dept-sync 部门成员失败: " + err.Error()})
			return
		}
		membersData = membersResp.Data
	}
	universalIDs := make([]string, 0, len(membersData))
	seen := make(map[string]struct{}, len(membersData))
	for _, src := range membersData {
		uid := strings.TrimSpace(src.UniversalId)
		if uid == "" {
			continue
		}
		if _, ok := seen[uid]; ok {
			continue
		}
		seen[uid] = struct{}{}
		universalIDs = append(universalIDs, uid)
	}
	if len(universalIDs) == 0 {
		c.JSON(http.StatusOK, EntityTrendResponse{Data: []EntityTrendPoint{}})
		return
	}

	// 2. 按 ISO 周守恒聚合 user_productivity_v2（user_id IN 成员 + week_start 窗口 → GROUP BY week_start）。
	points, err := listDeptTreeWeeklyTrend(statDB, universalIDs, c.Query("startDate"), c.Query("endDate"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "查询部门周趋势失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, EntityTrendResponse{Data: points})
}

// listDeptTreeWeeklyTrend 把 user_productivity_v2 周表按 week_start 聚合成趋势点：
// user_id IN (universalIDs)（部门整棵子树成员）+ startDate/endDate 窗口过滤，每周守恒重算 efficiency_pct。
// 周表已是 ISO 周一对齐的 week_start(date)，直接 GROUP BY，无需 date_trunc。
func listDeptTreeWeeklyTrend(db *gorm.DB, universalIDs []string, startDate, endDate string) ([]EntityTrendPoint, error) {
	type weekRow struct {
		WeekStart   time.Time `gorm:"column:week_start"`
		NeedCnt     int64     `gorm:"column:need_cnt"`
		CommitCnt   int64     `gorm:"column:commit_cnt"`
		DiffLines   int64     `gorm:"column:diff_lines"`
		SumBaseline float64   `gorm:"column:sum_baseline"`
		SumActual   float64   `gorm:"column:sum_actual"`
	}
	q := db.Model(&models.UserProductivityV2{}).
		Select(`week_start,
			COALESCE(SUM(merged_need_count), 0) AS need_cnt,
			COALESCE(SUM(commit_count), 0) AS commit_cnt,
			COALESCE(SUM(commit_diff_lines), 0) AS diff_lines,
			COALESCE(SUM(baseline_calendar_min), 0) AS sum_baseline,
			COALESCE(SUM(actual_calendar_min), 0) AS sum_actual`).
		Where("user_id IN ?", universalIDs)
	if start, err := parseStartDate(startDate); err == nil && start != nil {
		q = q.Where("week_start >= ?", *start)
	}
	if end, err := parseEndDate(endDate); err == nil && end != nil {
		q = q.Where("week_start <= ?", *end)
	}
	var rows []weekRow
	if err := q.Group("week_start").Order("week_start").Scan(&rows).Error; err != nil {
		return nil, err
	}
	points := make([]EntityTrendPoint, 0, len(rows))
	for _, r := range rows {
		points = append(points, EntityTrendPoint{
			WeekStart:     r.WeekStart.Format("2006-01-02"),
			EfficiencyPct: utils.CalcEfficiencyRatio(r.SumBaseline, r.SumActual),
			CommitCount:   int(r.CommitCnt),
			DiffLines:     int(r.DiffLines),
			NeedCount:     int(r.NeedCnt),
			// Loc：周表无 LOC 列，恒 0；Cost：部门趋势不计费用，恒 0。
		})
	}
	return points, nil
}
