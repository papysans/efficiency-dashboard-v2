package main

// 部门维度进程内缓存（需求2：组织树短期不变 → 加缓存）。
//
// 缓存三类输入，全部短 TTL（默认 60s）：
//   ① dept 树（HTTP 取 dept-sync /department/tree + 单根/森林重建结果）—— dept-sync 自带 300s 缓存，
//      这里再加进程内 60s 把一次组织页加载里 tree/ranking/trend/overview 的多次重建收敛成一次。
//   ② 全森林成员花名册（所有根的 include_children 并集，与日期无关）—— overview / forest-ranking 用。
//   ③ aggregateUsersV2 按 (startDate,endDate) 窗口 —— 最重（看板全表聚合），旧实现每个 members/ranking
//      都重跑一次；同一次组织页加载里所有节点共享同一时间窗 → 缓存后只算 1 次。
//
// 并发：各缓存独立 Mutex；TTL 内并发首次回源可能短暂重复算一次，可接受，不引 singleflight 依赖。
// 缓存的树/花名册被 handler 只读消费（不就地改），故跨请求共享安全。

import (
	"sync"
	"time"
)

// deptCacheTTL 部门维度缓存有效期。组织树/花名册/聚合短期不变，60s 足够把一次页面加载的并发调用收敛成一次回源。
var deptCacheTTL = 60 * time.Second

// ───────────────────────────── ① dept 树（重建结果）缓存 ─────────────────────────────
var (
	deptTreeMu  sync.Mutex
	deptTreeVal []DeptTreeNode
	deptTreeExp time.Time
)

// cachedRebuiltDeptTree 取缓存的 dept 树（HTTP 取 + rebuildSingleRootTree）；过期/未命中则回源并写缓存。
func cachedRebuiltDeptTree(baseURL, queryKey string) ([]DeptTreeNode, error) {
	deptTreeMu.Lock()
	if deptTreeVal != nil && time.Now().Before(deptTreeExp) {
		v := deptTreeVal
		deptTreeMu.Unlock()
		return v, nil
	}
	deptTreeMu.Unlock()

	var resp deptSyncTreeResp
	if err := deptSyncGet(baseURL, queryKey, "/department/tree", &resp); err != nil {
		return nil, err
	}
	if resp.Data == nil {
		resp.Data = []DeptTreeNode{}
	}
	tree := rebuildSingleRootTree(resp.Data)

	deptTreeMu.Lock()
	deptTreeVal = tree
	deptTreeExp = time.Now().Add(deptCacheTTL)
	deptTreeMu.Unlock()
	return tree, nil
}

// ───────────────────────────── ② 全森林成员花名册缓存 ─────────────────────────────
var (
	deptMembersMu  sync.Mutex
	deptMembersVal []deptSyncMemberNode
	deptMembersExp time.Time
)

// cachedAllDeptMembers 取缓存的「所有根整棵子树成员并集」（各根 include_children 花名册拼接）；过期/未命中则回源。
// 与日期无关（dept-sync 组织花名册），故全局缓存。overview / forest-ranking 共用。
func cachedAllDeptMembers(baseURL, queryKey string, roots []DeptTreeNode) ([]deptSyncMemberNode, error) {
	deptMembersMu.Lock()
	if deptMembersVal != nil && time.Now().Before(deptMembersExp) {
		v := deptMembersVal
		deptMembersMu.Unlock()
		return v, nil
	}
	deptMembersMu.Unlock()

	all := make([]deptSyncMemberNode, 0, 256)
	for _, root := range roots {
		var resp deptSyncMembersResp
		if err := deptSyncGet(baseURL, queryKey, "/department/"+root.DeptId+"/users?include_children=true", &resp); err != nil {
			return nil, err
		}
		all = append(all, resp.Data...)
	}

	deptMembersMu.Lock()
	deptMembersVal = all
	deptMembersExp = time.Now().Add(deptCacheTTL)
	deptMembersMu.Unlock()
	return all, nil
}

// ───────────────────────────── ③ aggregateUsersV2 按窗口缓存 ─────────────────────────────
var (
	aggUsersMu    sync.Mutex
	aggUsersCache = map[string]aggUsersEntry{}
)

type aggUsersEntry struct {
	rows []UserV2Row
	exp  time.Time
}

// cachedAggregateUsersV2 取缓存的 aggregateUsersV2(全表聚合)，按 (startDate,endDate) 窗口；过期/未命中则回源。
// 这是部门端点里最重的一步，缓存收益最大（同窗口的 members/ranking/overview 复用一次聚合）。
func cachedAggregateUsersV2(startDate, endDate string) ([]UserV2Row, error) {
	key := startDate + "|" + endDate

	aggUsersMu.Lock()
	if e, ok := aggUsersCache[key]; ok && time.Now().Before(e.exp) {
		rows := e.rows
		aggUsersMu.Unlock()
		return rows, nil
	}
	aggUsersMu.Unlock()

	rows, err := aggregateUsersV2(startDate, endDate, "")
	if err != nil {
		return nil, err
	}

	aggUsersMu.Lock()
	aggUsersCache[key] = aggUsersEntry{rows: rows, exp: time.Now().Add(deptCacheTTL)}
	aggUsersMu.Unlock()
	return rows, nil
}
