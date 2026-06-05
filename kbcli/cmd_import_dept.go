package main

import (
	"encoding/json"
	"fmt"
	"io"
	"kanban/core/models"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/spf13/cobra"
)

// deptUserDeptsConcurrency 是逐工号调 /user/{工号}/departments 的并发上限。
const deptUserDeptsConcurrency = 12

// dept-sync 数据接口路由前缀（前缀 /costrict-dept-info + /api/v1）
const deptSyncAPIPrefix = "/costrict-dept-info/api/v1"

// ---- dept-sync HTTP API 响应结构（实测契约见 task research/dept-sync-api.md）----

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

// deptSyncUserDeptsResp 是 GET /api/v1/user/{工号}/departments 的响应。
// 注意：此接口无 username 字段（真名阶段1 从 commits.git_user_name 派生）。
type deptSyncUserDeptsResp struct {
	Code    string                 `json:"code"`
	Success bool                   `json:"success"`
	Data    []deptSyncUserDeptNode `json:"data"`
}

// deptSyncUserDeptNode 某工号的一条部门归属。UserId 是工号（JOIN 锚点）。
type deptSyncUserDeptNode struct {
	UserId      string `json:"user_id"`
	UniversalId string `json:"universal_id"`
	DeptId      string `json:"dept_id"`
	DeptName    string `json:"dept_name"`
	DeptPath    string `json:"dept_path"`
	Position    string `json:"position"`
	IsMain      int    `json:"is_main"`
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

// projectDeptToUserOrg 工号桥接投影：用已构建的 uidsByEmp（工号 → 看板 user_id 列表，
// 来自 commits.git_user_email 前缀）把 deptUsers 按其主部门 dept_path 拆 org1..org9 回填 user_org。
// universal_id 不参与桥接。返回命中并写入的 user_org 行数。
func projectDeptToUserOrg(db *gorm.DB, deptUsers []models.DeptUser, deptPathByID map[string]string, uidsByEmp map[string][]string) (int, error) {
	var userOrgs []models.UserOrg
	for _, du := range deptUsers {
		uids := uidsByEmp[du.EmpNo]
		if len(uids) == 0 {
			continue // 该员工在看板无 sangfor commit，无法桥接，跳过
		}
		segs := splitDeptPath(deptPathByID[du.DeptId])
		for _, uid := range uids {
			uo := models.UserOrg{UserId: uid, UserName: du.RealName}
			assignOrgFields(&uo, segs)
			userOrgs = append(userOrgs, uo)
		}
	}
	if len(userOrgs) == 0 {
		return 0, nil
	}
	return len(userOrgs), saveUserOrgDeptProjection(db, userOrgs)
}

// triggerOrgRefresh 尽力触发 backend 重载 org 映射（失败仅告警，可手动重启 backend）。
func triggerOrgRefresh(backendURL string) {
	if backendURL == "" {
		logWarnf("backend_url 未配置，跳过 orgs/refresh，请手动重启 backend 或调用 POST /api/v2/orgs/refresh 使映射生效")
		return
	}
	url := strings.TrimRight(backendURL, "/") + "/api/v2/orgs/refresh"
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Post(url, "application/json", nil)
	if err != nil {
		logWarnf("调用 orgs/refresh 失败（请手动重启 backend）: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logWarnf("orgs/refresh 返回非 200（请手动重启 backend）: %d %s", resp.StatusCode, string(body))
		return
	}
	logInfo("已触发 backend orgs/refresh，org 映射已重载")
}

func runImportDept(baseURL, queryKey string) error {
	startTime := time.Now()
	if baseURL == "" {
		baseURL = cfg.DeptSync.BaseURL
	}
	if queryKey == "" {
		queryKey = cfg.DeptSync.QueryKey
	}
	if baseURL == "" {
		err := fmt.Errorf("未配置 dept-sync 服务地址（--base-url 或 config dept_sync.base_url）")
		recordCommandRun("import-dept", startTime, 0, 0, 0, err)
		return err
	}

	// 1. 拉全量嵌套部门树，拍平 → 全量落 dept（dept_path 供投影查）
	var treeResp deptSyncTreeResp
	if err := deptSyncGet(baseURL, queryKey, "/department/tree", &treeResp); err != nil {
		recordCommandRun("import-dept", startTime, 0, 0, 0, err)
		return err
	}
	var depts []models.Dept
	flattenDeptTree(treeResp.Data, &depts)
	logInfof("从 dept-sync 拉取到 %d 个部门", len(depts))

	deptPathByID := make(map[string]string, len(depts))
	for _, d := range depts {
		deptPathByID[d.DeptId] = d.DeptPath
	}

	// 2. 从看板 commits 取 distinct 工号 + 真名兜底，构建工号→user_id 桥接与工号→真名映射
	db, err := models.OpenGormDB(cfg.StatDatabase.DSN())
	if err != nil {
		recordCommandRun("import-dept", startTime, 0, 0, 0, err)
		return fmt.Errorf("连接目标数据库失败: %w", err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	logInfo("目标数据库连接成功")

	type empCommit struct {
		EmpNo       string
		UserId      string
		GitUserName string
	}
	var rows []empCommit
	if err := db.Raw(`
		SELECT DISTINCT split_part(git_user_email, '@', 1) AS emp_no, user_id, git_user_name
		FROM commits
		WHERE git_user_email ILIKE '%@sangfor.com' AND user_id IS NOT NULL AND user_id <> ''
	`).Scan(&rows).Error; err != nil {
		recordCommandRun("import-dept", startTime, len(depts), 0, 0, err)
		return fmt.Errorf("从 commits 构建工号桥接失败: %w", err)
	}
	uidsByEmp := make(map[string][]string) // 工号 → 看板 user_id 列表（一人可多 UUID）
	nameByEmp := make(map[string]string)   // 工号 → 派生真名
	for _, r := range rows {
		if r.EmpNo == "" {
			continue
		}
		uidsByEmp[r.EmpNo] = append(uidsByEmp[r.EmpNo], r.UserId)
		if _, ok := nameByEmp[r.EmpNo]; !ok {
			if dn := deriveRealName(r.GitUserName); dn != "" {
				nameByEmp[r.EmpNo] = dn
			}
		}
	}
	logInfof("看板 commits 中 distinct 工号 %d 个", len(uidsByEmp))

	// 3. 工号驱动 + 并发：每工号调 /user/{工号}/departments，取主部门，容忍查无此人
	emps := make([]string, 0, len(uidsByEmp))
	for emp := range uidsByEmp {
		emps = append(emps, emp)
	}
	sort.Strings(emps)

	var (
		mu        sync.Mutex
		deptUsers []models.DeptUser
		skipped   int // dept-sync 查无此人 / 请求失败 / data 为空
		wg        sync.WaitGroup
		sem       = make(chan struct{}, deptUserDeptsConcurrency)
	)
	for _, emp := range emps {
		emp := emp
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			var resp deptSyncUserDeptsResp
			if err := deptSyncGet(baseURL, queryKey, "/user/"+emp+"/departments", &resp); err != nil || len(resp.Data) == 0 {
				mu.Lock()
				skipped++
				mu.Unlock()
				return
			}
			// 取 is_main==1 那条；没有则取第一条
			node := resp.Data[0]
			for _, n := range resp.Data {
				if n.IsMain == 1 {
					node = n
					break
				}
			}
			du := models.DeptUser{
				EmpNo:       emp,
				RealName:    nameByEmp[emp],
				UniversalId: node.UniversalId,
				DeptId:      node.DeptId,
				Position:    node.Position,
				IsMain:      node.IsMain,
				Status:      node.Status,
			}
			mu.Lock()
			deptUsers = append(deptUsers, du)
			mu.Unlock()
		}()
	}
	wg.Wait()

	sort.Slice(deptUsers, func(i, j int) bool { return deptUsers[i].EmpNo < deptUsers[j].EmpNo })
	logInfof("工号驱动查部门：命中 %d / 跳过 %d / 共 %d", len(deptUsers), skipped, len(emps))

	// 4. 落库：全量部门树 + 仅命中的工号
	if err := saveDepts(db, depts); err != nil {
		recordCommandRun("import-dept", startTime, 0, 0, 0, err)
		return err
	}
	if err := saveDeptUsers(db, deptUsers); err != nil {
		recordCommandRun("import-dept", startTime, 0, 0, 0, err)
		return err
	}
	logInfof("已写入 %d 个部门到 dept、%d 名命中人员到 dept_user", len(depts), len(deptUsers))

	// 5. 投影回填 user_org（复用已构建的工号→user_id 桥接）
	matched, err := projectDeptToUserOrg(db, deptUsers, deptPathByID, uidsByEmp)
	if err != nil {
		recordCommandRun("import-dept", startTime, len(depts), 0, 0, err)
		return err
	}
	logInfof("投影回填 user_org：经工号桥接命中并写入 %d 条 user_org 记录", matched)

	// 6. 刷新后端 org 映射（尽力而为）
	triggerOrgRefresh(cfg.BackendURL)

	recordCommandRun("import-dept", startTime, len(depts)+len(deptUsers), 0, 0, nil)
	return nil
}

var importDeptCmd = &cobra.Command{
	Use:   "import-dept",
	Short: "从 dept-sync 服务拉取部门树与人员，落库 dept/dept_user 并投影回填 user_org",
	Long: `调用 dept-sync HTTP API（带 X-Query-Key）：先 /api/v1/department/tree 全量落 dept；
再以看板 commits 中的 distinct 工号驱动，并发调 /api/v1/user/{工号}/departments 取主部门，
落入 costrict_stat 的 dept / dept_user 表（仅命中的工号）。
真名阶段1 从 commits.git_user_name 去尾部工号数字派生；
并以工号经 commits.git_user_email 前缀桥接到看板 user_id，按 dept_path 拆 org1..org9 投影回填 user_org。
universal_id 仅留存，不参与 JOIN。`,
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
