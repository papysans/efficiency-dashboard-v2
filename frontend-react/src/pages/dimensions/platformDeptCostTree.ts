// 组织「成本树」数据层 —— 把整棵 dept-tree 的每个部门「直属成本」算出来，再纯函数 rollup 成子树成本。
//
// 与 platformDeptData 的「一级部门 PK 榜」差异：那个只拉根的**直接子部门**(topLevelDepts)；本 hook 要
// 整棵树**每个**部门的直属成员，故扁平化树取全部 dept_id（flattenDeptIds）后 useQueries 并行拉 members。
//   ⚠️ 这是 N 个 /v2/dept-tree/members 请求（N=树节点数，可能上千）。可接受但量大：
//      - 各 query staleTime 5min + retry:false，切日期才重拉；同 dept_id 跨 hook 共享 queryKey 命中缓存。
//      - 真实树请在内网验（本地 dept-tree 503 → treeQ.error → 调用方走 PlatformNotConnected 降级，0 个 members 请求）。
//   复用 platformDeptData.useFullRanking（全量 ranking 一次拉）+ universal_id→cost 映射，不重写。
//
// 降级：dept-tree 503 / 平台 503 / chat 关 → error 非空或 enabled=false，调用方优雅占位，绝不崩。
import { useMemo } from 'react'
import { useQuery, useQueries } from '@tanstack/react-query'
import { getDeptTreeV2, getDeptTreeMembersV2 } from '@/api/endpoints'
import type { DeptTreeNode } from '@/api/types'
import {
  isTruncated,
  type ChatUserRankingRow,
} from './platformUserData'
import { useFullRanking } from './platformDeptData'
import { rollupCostTree, flattenDeptIds, type CostTreeNode, type DeptDirectAgg } from './costTreeRollup'

export interface DeptCostTreeResult {
  /** rollup 后的成本树（每节点带 subtreeCost/subtreeMembers，根为一级部门/公司根）。 */
  nodes: CostTreeNode[]
  loading: boolean
  error: string | null
  /** 全量 ranking total>AGG_PAGE_SIZE → 直属成本按 Top N 命中求和（排行外漏算），子树成本整体偏小 → UI 标注。 */
  truncated: boolean
  rankingTotal?: number
  /** 拉取了多少个部门 members 请求（量级提示，便于 UI/诊断）。 */
  deptRequestCount: number
}

/**
 * 组织「成本树」聚合态数据源：
 *   1. dept-tree 整棵树（useDeptTree 同 queryKey）。
 *   2. 全量平台 ranking 一次（useFullRanking）→ universal_id → estimated_total_cost 映射。
 *   3. 扁平化树取所有 dept_id → useQueries 并行拉各部门**直属成员**名单。
 *   4. 每部门直属成本 = Σ(直属成员命中 ranking 的 cost)；纯函数 rollupCostTree 自底向上求子树成本。
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

  // 扁平化取全树所有 dept_id（N 个），各拉一次直属成员（非递归）。N 可能上千 → 见文件头注。
  const allDeptIds = useMemo(() => flattenDeptIds(tree), [tree])

  const memberQueries = useQueries({
    queries: allDeptIds.map((id) => ({
      // 与 platformDeptData 的 ['dim-dept-members', id, start, end] 同 key → 一级部门 members 命中缓存复用。
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
    const map = new Map<string, DeptDirectAgg>()
    allDeptIds.forEach((id, i) => {
      const members = memberQueries[i]?.data?.members ?? []
      let directCost = 0
      let directActive = 0
      for (const m of members) {
        const hit = costByUid.get(m.universal_id)
        if (hit) {
          directCost += hit.estimated_total_cost || 0
          if ((hit.total_requests || 0) > 0 || (hit.error_requests || 0) > 0) directActive += 1
        }
      }
      map.set(id, { directCost, directMembers: members.length, directActive })
    })
    return map
  }, [allDeptIds, memberQueries, rankingQ.data])

  const nodes = useMemo(() => rollupCostTree(tree, directAggByDept), [tree, directAggByDept])

  const loading =
    enabled && (treeQ.isLoading || rankingQ.isLoading || memberQueries.some((q) => q.isLoading))

  const treeErr = treeQ.error as Error | undefined
  const rankErr = rankingQ.error as Error | undefined
  // 单个部门 members 失败不致命（该部门直属成本算 0，子树仍 rollup）；只在关键源（树/平台）失败时占位。
  const error = treeErr ? treeErr.message : rankErr ? rankErr.message : null

  const rankingTotal = rankingQ.data?.total
  const truncated = isTruncated(rankingTotal, rankingQ.data?.data?.length ?? 0)

  return {
    nodes,
    loading,
    error,
    truncated,
    rankingTotal,
    deptRequestCount: allDeptIds.length,
  }
}
