// 组织「成本树」数据层 —— 把整棵 dept-tree 每个部门的「直属成本」算出来，再纯函数 rollup 成子树成本。
//
// 取数（一次性，治 N+1）：对每个顶层根调一次 /v2/dept-tree/members（整棵子树花名册，含子部门，每名成员带
//   dept_id）→ 客户端按成员**直属 dept_id** 归桶，算各部门「直属成员/直属成本」；rollupCostTree 再自底向上加子树。
//   ⚠️ 旧实现：扁平化树取全部 dept_id，**逐部门**各调一次 members → N（树节点数，可能上千）个请求，浏览器并发被
//      打爆（ERR_INSUFFICIENT_RESOURCES）；且逐部门用 include_children（整子树）当「直属」，rollup 时重复累加
//      （子树人数/活跃虚高）。新实现按真·直属归桶（一根一次取数），顺带修正该重复计数（成本¥0 时数值不变）。
//
// 复用 platformDeptData.useFullRanking（全量平台 ranking 一次）+ universal_id→cost 映射，不重写。
// 降级：dept-tree 503 / 平台 503 / chat 关 → error 非空或 enabled=false，调用方优雅占位，绝不崩。
import { useMemo } from 'react'
import { useQuery, useQueries } from '@tanstack/react-query'
import { getDeptTreeV2, getDeptTreeMembersV2 } from '@/api/endpoints'
import type { DeptTreeNode, DeptMember } from '@/api/types'
import {
  isTruncated,
  type ChatUserRankingRow,
} from './platformUserData'
import { useFullRanking } from './platformDeptData'
import { rollupCostTree, type CostTreeNode, type DeptDirectAgg } from './costTreeRollup'

export interface DeptCostTreeResult {
  /** rollup 后的成本树（每节点带 subtreeCost/subtreeMembers，根为一级部门/公司根）。 */
  nodes: CostTreeNode[]
  loading: boolean
  error: string | null
  /** 全量 ranking total>AGG_PAGE_SIZE → 直属成本按 Top N 命中求和（排行外漏算），子树成本整体偏小 → UI 标注。 */
  truncated: boolean
  rankingTotal?: number
  /** 拉取了多少个根花名册请求（量级提示，便于 UI/诊断）。 */
  deptRequestCount: number
}

/**
 * 组织「成本树」聚合态数据源：
 *   1. dept-tree 整棵树（useDeptTree 同 queryKey）。
 *   2. 全量平台 ranking 一次（useFullRanking）→ universal_id → estimated_total_cost 映射。
 *   3. 各顶层根一次性拉整棵子树花名册（含 dept_id）→ 按直属 dept_id 归桶算各部门直属成本/人数。
 *   4. 纯函数 rollupCostTree 自底向上求子树成本。
 * 任一关键源失败（dept-tree 不可达 / 平台 503）→ error 非空，调用方优雅占位。
 */
export function useDeptCostTree(
  params: { startDate: string; endDate: string },
  enabled: boolean,
): DeptCostTreeResult {
  const { startDate, endDate } = params

  const treeQ = useQuery({
    queryKey: ['dept-tree'],
    queryFn: () => getDeptTreeV2(),
    enabled,
    retry: false,
    staleTime: 5 * 60_000,
  })
  const tree = useMemo<DeptTreeNode[]>(() => treeQ.data ?? [], [treeQ.data])

  // 顶层根（通常 1 个；留空配置=森林多根）。对每个根整棵子树一次性拉花名册（含子部门），替代逐部门 N× members。
  const rootIds = useMemo(() => tree.map((n) => n.dept_id).filter(Boolean), [tree])

  const rosterQueries = useQueries({
    queries: rootIds.map((id) => ({
      // 与 platformDeptData 的 ['dim-dept-members', id, start, end] 同 key → 成本页同根花名册命中缓存复用（去重）。
      queryKey: ['dim-dept-members', id, startDate, endDate],
      queryFn: () => getDeptTreeMembersV2({ deptId: id, startDate, endDate }),
      enabled: enabled && !!id,
      retry: false,
      refetchOnWindowFocus: false,
      staleTime: 5 * 60_000,
    })),
  })

  const rankingQ = useFullRanking({ startDate, endDate }, enabled)

  const directAggByDept = useMemo(() => {
    // universal_id → estimated_total_cost（复用 platformUserData 行类型；空 universal_id 跳过）。
    const costByUid = new Map<string, ChatUserRankingRow>()
    for (const r of rankingQ.data?.data ?? []) {
      if (r.universal_id) costByUid.set(r.universal_id, r)
    }
    // 按成员**直属 dept_id** 归桶：每名成员在根花名册里只出现一次 → directMembers 即真直属人数（rollup 再加子树，不重复）。
    const map = new Map<string, DeptDirectAgg>()
    const collect = (members: DeptMember[]) => {
      for (const m of members) {
        const deptId = m.dept_id
        if (!deptId) continue
        let agg = map.get(deptId)
        if (!agg) {
          agg = { directCost: 0, directMembers: 0, directActive: 0 }
          map.set(deptId, agg)
        }
        agg.directMembers += 1
        const hit = costByUid.get(m.universal_id)
        if (hit) {
          agg.directCost += hit.estimated_total_cost || 0
          if ((hit.total_requests || 0) > 0 || (hit.error_requests || 0) > 0) agg.directActive += 1
        }
      }
    }
    for (const q of rosterQueries) collect(q.data?.members ?? [])
    return map
  }, [rosterQueries, rankingQ.data])

  const nodes = useMemo(() => rollupCostTree(tree, directAggByDept), [tree, directAggByDept])

  const loading =
    enabled && (treeQ.isLoading || rankingQ.isLoading || rosterQueries.some((q) => q.isLoading))

  const treeErr = treeQ.error as Error | undefined
  const rankErr = rankingQ.error as Error | undefined
  // 单个根花名册失败不致命（该根直属成本算 0，其余仍 rollup）；只在关键源（树/平台）失败时占位。
  const error = treeErr ? treeErr.message : rankErr ? rankErr.message : null

  const rankingTotal = rankingQ.data?.total
  const truncated = isTruncated(rankingTotal, rankingQ.data?.data?.length ?? 0)

  return {
    nodes,
    loading,
    error,
    truncated,
    rankingTotal,
    deptRequestCount: rootIds.length,
  }
}
