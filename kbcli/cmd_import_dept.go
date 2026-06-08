package main

import (
	"encoding/json"
	"fmt"
	"io"
	"kanban/core/models"
	"kanban/kbcli/internal/appconfig"
	"kanban/kbcli/internal/logx"
	"kanban/kbcli/internal/util"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/spf13/cobra"
)

// dept-sync 数据接口路由前缀（前缀 /costrict-dept-info + /api/v1）
const deptSyncAPIPrefix = "/costrict-dept-info/api/v1"

// ---- dept-sync HTTP API 响应结构（实测契约见 task research/dept-sync-full-api.md）----

type deptSyncTreeResp struct {
	Code    string         `json:"code"`
	Success bool           `json:"success"`
	Data    []deptSyncNode `json:"data"`
}

// deptSyncNode 部门树节点（嵌套 children）。null 字段反序列化为零值。
type deptSyncNode struct {
	DeptId         string         `json:"dept_id"`
	DeptName       string         `json:"dept_name"`
	ParentDeptId   string         `json:"parent_dept_id"`
	DeptPath       string         `json:"dept_path"`
	DeptLevel      int            `json:"dept_level"`
	OrderNum       int            `json:"order_num"`
	LeaderId       string         `json:"leader_id"`
	ChildDeptCount int            `json:"child_dept_count"`
	Status         int            `json:"status"`
	Children       []deptSyncNode `json:"children"`
}

// deptSyncDeptUsersResp 是 GET /api/v1/department/{dept_id}/users?include_children=true 的响应。
// 对公司根部门调用一次即拿全量人员，字段最全（含 username 权威真名 + universal_id + 工号）。
type deptSyncDeptUsersResp struct {
	Code    string                 `json:"code"`
	Success bool                   `json:"success"`
	Data    []deptSyncDeptUserNode `json:"data"`
}

// deptSyncDeptUserNode 一名人员。UserId 是工号；Username 是权威真名；UniversalId 可能为空串。
// 注意：此响应无 dept_path，org1..9 需用 DeptId 去 dept 表 deptPathByID 查。
type deptSyncDeptUserNode struct {
	UserId      string `json:"user_id"` // 工号
	Username    string `json:"username"`
	UniversalId string `json:"universal_id"`
	DeptId      string `json:"dept_id"`
	DeptName    string `json:"dept_name"`
	IsMain      int    `json:"is_main"`
	Position    string `json:"position"`
	EntryTime   string `json:"entry_time"`
	Status      int    `json:"status"`
}

// deptSyncGet 调 dept-sync 数据接口（带 X-Query-Key），解析到 out。
func deptSyncGet(baseURL, queryKey, path string, out interface{}) error {
	url := strings.TrimRight(baseURL, "/") + deptSyncAPIPrefix + path
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Query-Key", queryKey)
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

// flattenDeptTree 递归把嵌套部门树拍平成 []models.Dept。
func flattenDeptTree(nodes []deptSyncNode, out *[]models.Dept) {
	for _, n := range nodes {
		*out = append(*out, models.Dept{
			DeptId:         n.DeptId,
			DeptName:       n.DeptName,
			ParentDeptId:   n.ParentDeptId,
			DeptPath:       n.DeptPath,
			DeptLevel:      n.DeptLevel,
			OrderNum:       n.OrderNum,
			LeaderId:       n.LeaderId,
			ChildDeptCount: n.ChildDeptCount,
			Status:         n.Status,
		})
		if len(n.Children) > 0 {
			flattenDeptTree(n.Children, out)
		}
	}
}

// findRootDeptIDs 找根部门：parent_dept_id 为空，或 parent 不在所有 dept_id 集合里（悬挂父引用）。
// 通常只有一个（深信服=49），为稳健支持多根。
func findRootDeptIDs(depts []models.Dept) []string {
	idSet := make(map[string]bool, len(depts))
	for _, d := range depts {
		idSet[d.DeptId] = true
	}
	var roots []string
	for _, d := range depts {
		if strings.TrimSpace(d.ParentDeptId) == "" || !idSet[d.ParentDeptId] {
			roots = append(roots, d.DeptId)
		}
	}
	return roots
}

// splitDeptPath 把物化路径 "/A/B/C" 拆成 ["A","B","C"]（去空段）。
func splitDeptPath(path string) []string {
	var segs []string
	for _, p := range strings.Split(path, "/") {
		p = strings.TrimSpace(p)
		if p != "" {
			segs = append(segs, p)
		}
	}
	return segs
}

// assignOrgFields 把部门层级段写入 UserOrg 的 Org1..Org9。
func assignOrgFields(uo *models.UserOrg, segs []string) {
	fields := []*string{&uo.Org1, &uo.Org2, &uo.Org3, &uo.Org4, &uo.Org5, &uo.Org6, &uo.Org7, &uo.Org8, &uo.Org9}
	for i, s := range segs {
		if i >= len(fields) {
			break
		}
		*fields[i] = s
	}
}

// deriveRealName 从 git_user_name 派生真名：去掉尾部连续工号数字（如 "林凯90331"→"林凯"），
// 再 TrimSpace。若去掉数字后为空（纯数字名）或原本无尾部数字，则回退原值 trim。
// 仅在工号次选且 dept-sync person 缺名时兜底（权威真名优先用 dept-sync username）。
func deriveRealName(gitUserName string) string {
	name := strings.TrimSpace(gitUserName)
	trimmed := strings.TrimRight(name, "0123456789")
	trimmed = strings.TrimSpace(trimmed)
	if trimmed == "" {
		return name
	}
	return trimmed
}

func saveDepts(db *gorm.DB, rows []models.Dept) error {
	return db.Transaction(func(tx *gorm.DB) error {
		for i := range rows {
			r := rows[i]
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "dept_id"}},
				DoUpdates: clause.AssignmentColumns([]string{
					"dept_name", "parent_dept_id", "dept_path", "dept_level",
					"order_num", "leader_id", "child_dept_count", "status", "updated_at",
				}),
			}).Create(&r).Error; err != nil {
				return fmt.Errorf("写入 dept 失败 [dept_id=%s]: %w", r.DeptId, err)
			}
		}
		return nil
	})
}

func saveDeptUsers(db *gorm.DB, rows []models.DeptUser) error {
	return db.Transaction(func(tx *gorm.DB) error {
		for i := range rows {
			r := rows[i]
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "emp_no"}},
				DoUpdates: clause.AssignmentColumns([]string{
					"real_name", "universal_id", "dept_id", "position",
					"is_main", "entry_time", "status", "updated_at",
				}),
			}).Create(&r).Error; err != nil {
				return fmt.Errorf("写入 dept_user 失败 [emp_no=%s]: %w", r.EmpNo, err)
			}
		}
		return nil
	})
}

// saveUserOrgDeptProjection 投影专用 UPSERT：只更新 user_name + org1..org9，
// 不碰 git_user_name/git_user_email（避免投影时空值覆盖已有 git 字段；故未复用 saveUserOrgs）。
func saveUserOrgDeptProjection(db *gorm.DB, rows []models.UserOrg) error {
	return db.Transaction(func(tx *gorm.DB) error {
		for i := range rows {
			r := rows[i]
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "user_id"}},
				DoUpdates: clause.AssignmentColumns([]string{
					"user_name", "org1", "org2", "org3", "org4", "org5", "org6", "org7", "org8", "org9", "updated_at",
				}),
			}).Create(&r).Error; err != nil {
				return fmt.Errorf("投影写入 user_org 失败 [user_id=%s]: %w", r.UserId, err)
			}
		}
		return nil
	})
}

// triggerOrgRefresh 尽力触发 backend 重载 org 映射（失败仅告警，可手动重启 backend）。
func triggerOrgRefresh(backendURL string) {
	if backendURL == "" {
		logx.Warnf("backend_url 未配置，跳过 orgs/refresh，请手动重启 backend 或调用 POST /api/v2/orgs/refresh 使映射生效")
		return
	}
	url := strings.TrimRight(backendURL, "/") + "/api/v2/orgs/refresh"
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Post(url, "application/json", nil)
	if err != nil {
		logx.Warnf("调用 orgs/refresh 失败（请手动重启 backend）: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logx.Warnf("orgs/refresh 返回非 200（请手动重启 backend）: %d %s", resp.StatusCode, string(body))
		return
	}
	logx.Info("已触发 backend orgs/refresh，org 映射已重载")
}

func runImportDept(baseURL, queryKey string) error {
	startTime := time.Now()
	if baseURL == "" {
		baseURL = appconfig.Cfg.DeptSync.BaseURL
	}
	if queryKey == "" {
		queryKey = appconfig.Cfg.DeptSync.QueryKey
	}
	if baseURL == "" {
		err := fmt.Errorf("未配置 dept-sync 服务地址（--base-url 或 config dept_sync.base_url）")
		util.RecordCommandRun("import-dept", startTime, 0, 0, 0, err)
		return err
	}

	// 1. 拉全量嵌套部门树，拍平 → 全量落 dept；构建 dept_id → dept_path 供投影查（include_children 响应无 dept_path）
	var treeResp deptSyncTreeResp
	if err := deptSyncGet(baseURL, queryKey, "/department/tree", &treeResp); err != nil {
		util.RecordCommandRun("import-dept", startTime, 0, 0, 0, err)
		return err
	}
	var depts []models.Dept
	flattenDeptTree(treeResp.Data, &depts)
	logx.Infof("从 dept-sync 拉取到 %d 个部门", len(depts))

	deptPathByID := make(map[string]string, len(depts))
	for _, d := range depts {
		deptPathByID[d.DeptId] = d.DeptPath
	}

	// 2. 对每个根部门 include_children=true 批量取全量人员，按工号去重（通常仅 1 根）
	roots := findRootDeptIDs(depts)
	if len(roots) == 0 {
		err := fmt.Errorf("部门树未找到根部门（无 parent_dept_id 为空或悬挂的节点）")
		util.RecordCommandRun("import-dept", startTime, len(depts), 0, 0, err)
		return err
	}
	logx.Infof("识别到 %d 个根部门: %v", len(roots), roots)

	peopleByEmp := make(map[string]deptSyncDeptUserNode) // 工号 → person（跨根去重）
	for _, root := range roots {
		var usersResp deptSyncDeptUsersResp
		if err := deptSyncGet(baseURL, queryKey, "/department/"+root+"/users?include_children=true", &usersResp); err != nil {
			util.RecordCommandRun("import-dept", startTime, len(depts), 0, 0, err)
			return err
		}
		for _, p := range usersResp.Data {
			if p.UserId == "" {
				continue
			}
			peopleByEmp[p.UserId] = p // 同工号多部门：取主部门更稳妥，但 include_children 实测每人单条主部门
		}
	}
	logx.Infof("从根部门 include_children 拿到全量人员 %d 名（按工号去重后）", len(peopleByEmp))

	// 3. 落库：全量部门树 + 全量花名册（dept_user）
	db, err := models.OpenGormDB(appconfig.Cfg.StatDatabase.DSN())
	if err != nil {
		util.RecordCommandRun("import-dept", startTime, 0, 0, 0, err)
		return fmt.Errorf("连接目标数据库失败: %w", err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	logx.Info("目标数据库连接成功")

	deptUsers := make([]models.DeptUser, 0, len(peopleByEmp))
	for _, p := range peopleByEmp {
		deptUsers = append(deptUsers, models.DeptUser{
			EmpNo:       p.UserId,
			RealName:    p.Username,
			UniversalId: p.UniversalId,
			DeptId:      p.DeptId,
			Position:    p.Position,
			IsMain:      p.IsMain,
			EntryTime:   p.EntryTime,
			Status:      p.Status,
		})
	}
	if err := saveDepts(db, depts); err != nil {
		util.RecordCommandRun("import-dept", startTime, 0, 0, 0, err)
		return err
	}
	if err := saveDeptUsers(db, deptUsers); err != nil {
		util.RecordCommandRun("import-dept", startTime, 0, 0, 0, err)
		return err
	}
	logx.Infof("已写入 %d 个部门到 dept、%d 名人员到 dept_user", len(depts), len(deptUsers))

	// 4. 投影回填 user_org（universal_id 直连为主，工号经 commits 桥接次选，未命中兜底）

	// 4a. universal_id → person 映射（只收 universal_id 非空的；锚点 = 看板 user_org.user_id == universal_id）
	personByUniversalID := make(map[string]deptSyncDeptUserNode)
	for _, p := range peopleByEmp {
		if uid := strings.TrimSpace(p.UniversalId); uid != "" {
			personByUniversalID[uid] = p
		}
	}

	// 4b. 工号次选桥接：commits.git_user_email 前缀 → 工号 → 看板 user_id
	type empCommit struct {
		EmpNo       string
		UserId      string
		GitUserName string
	}
	var commitRows []empCommit
	if err := db.Raw(`
		SELECT DISTINCT split_part(git_user_email, '@', 1) AS emp_no, user_id, git_user_name
		FROM commits
		WHERE git_user_email ILIKE '%@sangfor.com' AND user_id IS NOT NULL AND user_id <> ''
	`).Scan(&commitRows).Error; err != nil {
		util.RecordCommandRun("import-dept", startTime, len(depts), 0, 0, err)
		return fmt.Errorf("从 commits 构建工号桥接失败: %w", err)
	}
	empByUID := make(map[string]string)  // 看板 user_id → 工号
	nameByEmp := make(map[string]string) // 工号 → git 派生真名（次选缺名兜底）
	for _, r := range commitRows {
		if r.EmpNo == "" {
			continue
		}
		if _, ok := empByUID[r.UserId]; !ok {
			empByUID[r.UserId] = r.EmpNo
		}
		if _, ok := nameByEmp[r.EmpNo]; !ok {
			if dn := deriveRealName(r.GitUserName); dn != "" {
				nameByEmp[r.EmpNo] = dn
			}
		}
	}

	// 4c. 读看板全部 user_org 行（看板用户全集），逐行回填
	var boardUsers []models.UserOrg
	if err := db.Find(&boardUsers).Error; err != nil {
		util.RecordCommandRun("import-dept", startTime, len(depts), 0, 0, err)
		return fmt.Errorf("读取看板 user_org 失败: %w", err)
	}

	fallbackOrg := strings.TrimSpace(appconfig.Cfg.DeptSync.FallbackOrgName)
	fallbackDept := strings.TrimSpace(appconfig.Cfg.DeptSync.FallbackDeptName)

	var (
		projections    []models.UserOrg
		hitUniversal   int
		hitEmpFallback int
		hitFallbackOrg int
		total          = len(boardUsers)
	)
	for _, bu := range boardUsers {
		// a. 主：user_org.user_id == universal_id 直连
		if p, ok := personByUniversalID[bu.UserId]; ok {
			uo := models.UserOrg{UserId: bu.UserId, UserName: p.Username}
			assignOrgFields(&uo, splitDeptPath(deptPathByID[p.DeptId]))
			projections = append(projections, uo)
			hitUniversal++
			continue
		}
		// b. 次选：看板 user_id → 工号 → people 里的 person
		if emp, ok := empByUID[bu.UserId]; ok {
			if p, ok := peopleByEmp[emp]; ok {
				name := p.Username
				if strings.TrimSpace(name) == "" {
					name = nameByEmp[emp] // person 缺名才用 git 派生真名兜底
				}
				uo := models.UserOrg{UserId: bu.UserId, UserName: name}
				assignOrgFields(&uo, splitDeptPath(deptPathByID[p.DeptId]))
				projections = append(projections, uo)
				hitEmpFallback++
				continue
			}
		}
		// c. 兜底：a/b 都没命中。配置了 fallback_org_name 才回填 org1/org2（其余 org 空，user_name 不动）。
		if fallbackOrg != "" {
			uo := models.UserOrg{UserId: bu.UserId, UserName: bu.UserName, Org1: fallbackOrg, Org2: fallbackDept}
			projections = append(projections, uo)
			hitFallbackOrg++
		}
	}

	if err := saveUserOrgDeptProjection(db, projections); err != nil {
		util.RecordCommandRun("import-dept", startTime, len(depts), 0, 0, err)
		return err
	}
	logx.Infof("投影回填 user_org：universal_id 命中 %d / 工号次选命中 %d / 兜底 %d（共 %d 名看板用户，写入 %d 条）",
		hitUniversal, hitEmpFallback, hitFallbackOrg, total, len(projections))

	// 5. 刷新后端 org 映射（尽力而为）
	triggerOrgRefresh(appconfig.Cfg.BackendURL)

	util.RecordCommandRun("import-dept", startTime, len(depts)+len(deptUsers), 0, 0, nil)
	return nil
}

var importDeptCmd = &cobra.Command{
	Use:   "import-dept",
	Short: "从 dept-sync 服务拉取部门树与全量人员，落库 dept/dept_user 并投影回填 user_org",
	Long: `调用 dept-sync HTTP API（带 X-Query-Key）：
1. GET /api/v1/department/tree 全量落 dept，并构建 dept_id→dept_path 供投影查。
2. 对公司根部门 GET /api/v1/department/{root}/users?include_children=true 一次拿全量人员
   （工号 + 权威真名 username + universal_id + dept_id），落入全量花名册 dept_user。
3. 投影回填 user_org：以 universal_id（== 看板 user_id）直连为主；未命中则经
   commits.git_user_email 前缀的工号桥接为次选；仍未命中且配置了 dept_sync.fallback_org_name
   时兜底到 org1=fallback_org_name / org2=fallback_dept_name。真名优先用 dept-sync username。
4. 尽力触发 POST /api/v2/orgs/refresh 重载映射。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		baseURL, _ := cmd.Flags().GetString("base-url")
		queryKey, _ := cmd.Flags().GetString("query-key")
		return runImportDept(baseURL, queryKey)
	},
}

func init() {
	importDeptCmd.Flags().SortFlags = false
	importDeptCmd.Flags().String("base-url", "", "dept-sync 服务地址（默认取 config dept_sync.base_url）")
	importDeptCmd.Flags().String("query-key", "", "dept-sync X-Query-Key 鉴权密钥（默认取 config dept_sync.query_key）")
	rootCmd.AddCommand(importDeptCmd)
}
