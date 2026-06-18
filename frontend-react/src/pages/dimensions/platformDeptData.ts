// 组织维度（org）平台数据接入的数据层 —— 平台无部门字段，故在看板侧做映射聚合：
//   universal_id(chatStats /stats/users/ranking) == 看板 user_id（同源已实锤）→ 经 dept-sync 部门花名册
//   (/v2/dept-tree/members，按 universal_id keyed) 归到部门。
//
// ⚠️ dept-tree/members 后端代理 dept-sync /department/{id}/users，**只返直属成员（非递归）**。
//   故部门聚合口径 = 仅该部门直属成员（子部门 rollup 不在范围内），UI 须诚实标注「仅直属成员」。
// ⚠️ 降级护栏：dept-tree 端点在本地 eval 环境不可达（dept-sync 未连通）→ 各查询 retry:false，
//   调用方据 error 优雅占位（PlatformNotConnected），绝不崩。chat_stats_enabled=false 时调用方根本不进来。
//
// 设计取舍：平台是「用户」三维客观源，无部门桶端点。聚合态需「各部门把成员平台指标 client-side 求和」，
//   因此：先 dept-tree 取根的直接子部门列表 → 各子部门拉直属成员名单（useQueries 并行）→
//   全量平台用户排行只拉一次（大页）→ 按 universal_id 命中的部门把行求和。
//   聚焦态：单部门成员名单 → universal_id 集合 → filter 平台排行行求和。
import { useMemo } from 'react'
import { useQuery, useQueries } from '@tanstack/react-query'
import { getDeptTreeV2, getDeptTreeMembersV2 } from '@/api/endpoints'
import { chatGet } from '@/api/client'
import type { DeptTreeNode } from '@/api/types'
import { sliceWeekWindows } from '@/lib/weekWindows'
import {
  errorRateOf,
  isTruncated,
  AGG_PAGE_SIZE,
  type ChatUserRankingRow,
  type ChatUsersRankingResp,
  type AggWindow,
} from './platformUserData'

/** 一个部门的平台指标聚合（直属成员命中的平台排行行求和）。 */
export interface DeptPlatformAgg {
  deptId: string
  deptName: string
  /** 该部门直属成员人数（dept-sync 名册）。 */
  memberCount: number
  /** 其中在区间内有平台调用记录的人数。 */
  activePlatformUsers: number
  totalRequests: number
  errorRequests: number
  errorRate: number | null
  uniqueTaskCount: number
  sumTotalTokens: number
  sumCacheTokens: number
  estimatedTotalCost: number
  /** 按请求数加权的平均时延（ms）。 */
  avgDurationMs: number | null
}

/** 把若干平台排行行求和成部门聚合（按请求数加权时延）。 */
function aggregateRows(
  deptId: string,
  deptName: string,
  memberCount: number,
  rows: ChatUserRankingRow[],
): DeptPlatformAgg {
  let totalRequests = 0
  let successRequests = 0
  let errorRequests = 0
  let uniqueTaskCount = 0
  let sumTotalTokens = 0
  let sumCacheTokens = 0
  let estimatedTotalCost = 0
  let weightedDur = 0
  let activePlatformUsers = 0
  for (const r of rows) {
    if ((r.total_requests || 0) > 0 || (r.error_requests || 0) > 0) activePlatformUsers += 1
    totalRequests += r.total_requests || 0
    successRequests += r.success_requests || 0
    errorRequests += r.error_requests || 0
    uniqueTaskCount += r.unique_task_count || 0
    sumTotalTokens += r.sum_total_tokens || 0
    sumCacheTokens += r.sum_cache_tokens || 0
    estimatedTotalCost += r.estimated_total_cost || 0
    weightedDur += (r.avg_duration_ms || 0) * (r.total_requests || 0)
  }
  return {
    deptId,
    deptName,
    memberCount,
    activePlatformUsers,
    totalRequests,
    errorRequests,
    // 统一口径：error/(success+error)（见 platformUserData.errorRateOf）。
    errorRate: errorRateOf(successRequests, errorRequests),
    uniqueTaskCount,
    sumTotalTokens,
    sumCacheTokens,
    estimatedTotalCost,
    avgDurationMs: totalRequests > 0 ? weightedDur / totalRequests : null,
  }
}

/** dept-tree 根的直接子部门（一级部门），聚合态部门 PK 榜的对象集。 */
function topLevelDepts(tree: DeptTreeNode[]): DeptTreeNode[] {
  // dept-tree 返回的就是根的子树数组；其每个元素即一级部门。
  return tree ?? []
}

/** 全量平台用户排行一次拉回（大页，前端按部门命中求和）。区间聚合（吃 start/end）。 */
function useFullRanking(params: { startDate: string; endDate: string }, enabled: boolean) {
  const { startDate, endDate } = params
  return useQuery({
    queryKey: ['dim-dept-full-ranking', startDate, endDate],
    queryFn: () =>
      chatGet<ChatUsersRankingResp>('/stats/users/ranking', {
        start_date: startDate,
        end_date: endDate,
        sort_by: 'total_requests',
        page: 1,
        page_size: AGG_PAGE_SIZE,
      }),
    enabled,
    retry: false,
    refetchOnWindowFocus: false,
    staleTime: 60_000,
  })
}

/**
 * 组织维度「聚合态」：一级部门平台 PK 榜（各部门直属成员的平台指标 client-side 求和）。
 * 数据 = dept-tree（一级部门 + 各部门直属成员名单 useQueries） × 全量平台排行（按 universal_id 命中求和）。
 * 任一关键源失败（dept-tree 不可达 / 平台 503）→ error 非空，调用方优雅占位。
 */
export function useDeptPlatformRanking(
  params: { startDate: string; endDate: string },
  enabled: boolean,
): {
  items: DeptPlatformAgg[]
  loading: boolean
  error: string | null
  /** 全量平台排行 total>AGG_PAGE_SIZE → 部门求和按 Top N 截断（漏掉排行外成员），UI 应标注（P1-2）。 */
  truncated: boolean
  /** 区间内平台真实总人数（截断文案用，未知为 undefined）。 */
  rankingTotal?: number
} {
  const treeQ = useQuery({
    queryKey: ['dept-tree'],
    queryFn: () => getDeptTreeV2(),
    enabled,
    retry: false,
    staleTime: 5 * 60_000,
  })
  const depts = useMemo(() => topLevelDepts(treeQ.data ?? []), [treeQ.data])

  // 各一级部门直属成员名单（useQueries 并行；只需 universal_id，不依赖日期 → members 端点吃日期但我们只取名册）。
  const memberQueries = useQueries({
    queries: depts.map((d) => ({
      queryKey: ['dim-dept-members', d.dept_id, params.startDate, params.endDate],
      queryFn: () => getDeptTreeMembersV2({ deptId: d.dept_id, startDate: params.startDate, endDate: params.endDate }),
      enabled: enabled && !!d.dept_id,
      retry: false,
      refetchOnWindowFocus: false,
      staleTime: 5 * 60_000,
    })),
  })

  const rankingQ = useFullRanking(params, enabled)

  const items = useMemo<DeptPlatformAgg[]>(() => {
    const rankByUid = new Map<string, ChatUserRankingRow>()
    for (const r of rankingQ.data?.data ?? []) {
      if (r.universal_id) rankByUid.set(r.universal_id, r)
    }
    return depts.map((d, i) => {
      const members = memberQueries[i]?.data?.members ?? []
      const memberCount = members.length
      const rows: ChatUserRankingRow[] = []
      for (const m of members) {
        const hit = rankByUid.get(m.universal_id)
        if (hit) rows.push(hit)
      }
      return aggregateRows(d.dept_id, d.dept_name, memberCount, rows)
    })
  }, [depts, memberQueries, rankingQ.data])

  const loading =
    enabled && (treeQ.isLoading || rankingQ.isLoading || memberQueries.some((q) => q.isLoading))
  const treeErr = treeQ.error as Error | undefined
  const rankErr = rankingQ.error as Error | undefined
  const error = treeErr ? treeErr.message : rankErr ? rankErr.message : null

  const rankingTotal = rankingQ.data?.total
  const truncated = isTruncated(rankingTotal, rankingQ.data?.data?.length ?? 0)

  return { items, loading, error, truncated, rankingTotal }
}

/**
 * 组织维度「聚焦态」：单部门平台聚合（该部门直属成员命中平台排行求和）。
 * deptId 空 → 不查。任一源失败 → error 非空。
 */
export function useDeptPlatformFocused(
  params: { startDate: string; endDate: string; deptId: string },
  enabled: boolean,
): {
  agg: DeptPlatformAgg | null
  /** 该部门直属成员逐人平台行（命中的，供成员排行表）。 */
  memberRows: Array<{ universal_id: string; username: string | null; row: ChatUserRankingRow }>
  loading: boolean
  error: string | null
  /** 全量平台排行 total>AGG_PAGE_SIZE → 该部门成员可能落在排行外漏算，UI 应标注（P1-2）。 */
  truncated: boolean
  rankingTotal?: number
} {
  const { startDate, endDate, deptId } = params
  const on = enabled && !!deptId

  const membersQ = useQuery({
    queryKey: ['dim-dept-members-focus', deptId, startDate, endDate],
    queryFn: () => getDeptTreeMembersV2({ deptId, startDate, endDate }),
    enabled: on,
    retry: false,
    refetchOnWindowFocus: false,
    staleTime: 5 * 60_000,
  })

  const rankingQ = useFullRanking({ startDate, endDate }, on)

  const result = useMemo(() => {
    const members = membersQ.data?.members ?? []
    const rankByUid = new Map<string, ChatUserRankingRow>()
    for (const r of rankingQ.data?.data ?? []) {
      if (r.universal_id) rankByUid.set(r.universal_id, r)
    }
    const rows: ChatUserRankingRow[] = []
    const memberRows: Array<{ universal_id: string; username: string | null; row: ChatUserRankingRow }> = []
    for (const m of members) {
      const hit = rankByUid.get(m.universal_id)
      if (hit) {
        rows.push(hit)
        memberRows.push({ universal_id: m.universal_id, username: hit.username, row: hit })
      }
    }
    const agg = aggregateRows(deptId, deptId, members.length, rows)
    return { agg, memberRows }
  }, [membersQ.data, rankingQ.data, deptId])

  const loading = on && (membersQ.isLoading || rankingQ.isLoading)
  const memErr = membersQ.error as Error | undefined
  const rankErr = rankingQ.error as Error | undefined
  const error = memErr ? memErr.message : rankErr ? rankErr.message : null

  const rankingTotal = rankingQ.data?.total
  const truncated = isTruncated(rankingTotal, rankingQ.data?.data?.length ?? 0)

  return { agg: result.agg, memberRows: result.memberRows, loading, error, truncated, rankingTotal }
}

/**
 * 组织维度「按周时间线」—— 把 timeRange 切成周窗口，各窗口拉全量平台排行 + 该范围部门成员，
 * 按部门成员命中求和成「按周」整体序列（聚焦=单部门；聚合=全部一级部门合计）。
 * 子部门 rollup 不在范围（members 非递归）。任一窗口失败不抛（retry:false + 兜底空点）。
 */
export function useDeptWeekSeries(
  params: { startDate: string; endDate: string; deptId?: string },
  enabled: boolean,
): {
  windows: ReturnType<typeof sliceWeekWindows>
  aggByKey: Map<string, AggWindow>
  loading: boolean
  error: string | null
  hasAny: boolean
} {
  const { startDate, endDate, deptId } = params
  const focused = !!deptId
  const windows = sliceWeekWindows(startDate, endDate)

  // 部门成员 universal_id 集合（聚焦=单部门；聚合=全部一级部门并集）。与周窗无关，整窗取一次。
  const treeQ = useQuery({
    queryKey: ['dept-tree'],
    queryFn: () => getDeptTreeV2(),
    enabled,
    retry: false,
    staleTime: 5 * 60_000,
  })
  const depts = useMemo(() => topLevelDepts(treeQ.data ?? []), [treeQ.data])
  const targetDeptIds = useMemo(
    () => (focused ? [deptId!] : depts.map((d) => d.dept_id)),
    [focused, deptId, depts],
  )

  const memberQueries = useQueries({
    queries: targetDeptIds.map((id) => ({
      queryKey: ['dim-dept-members', id, startDate, endDate],
      queryFn: () => getDeptTreeMembersV2({ deptId: id, startDate, endDate }),
      enabled: enabled && !!id,
      retry: false,
      refetchOnWindowFocus: false,
      staleTime: 5 * 60_000,
    })),
  })

  const memberUidSet = useMemo(() => {
    const s = new Set<string>()
    for (const q of memberQueries) {
      for (const m of q.data?.members ?? []) {
        if (m.universal_id) s.add(m.universal_id)
      }
    }
    return s
  }, [memberQueries])

  // 各周窗口拉全量平台排行（大页），前端按成员命中求和。
  const weekQueries = useQueries({
    queries: windows.map((w) => ({
      queryKey: ['dim-dept-week', w.startDate, w.endDate],
      queryFn: () =>
        chatGet<ChatUsersRankingResp>('/stats/users/ranking', {
          start_date: w.startDate,
          end_date: w.endDate,
          sort_by: 'total_requests',
          page: 1,
          page_size: 500,
        }),
      enabled,
      retry: false,
      refetchOnWindowFocus: false,
      staleTime: 60_000,
    })),
  })

  const aggByKey = useMemo(() => {
    const map = new Map<string, AggWindow>()
    windows.forEach((w, i) => {
      const rows = weekQueries[i]?.data?.data
      if (!rows) return
      let totalRequests = 0
      let totalSuccess = 0
      let totalErrors = 0
      let cost = 0
      let totalTokens = 0
      let activeUsers = 0
      for (const r of rows) {
        if (!memberUidSet.has(r.universal_id)) continue
        if ((r.total_requests || 0) > 0 || (r.error_requests || 0) > 0) activeUsers += 1
        totalRequests += r.total_requests || 0
        totalSuccess += r.success_requests || 0
        totalErrors += r.error_requests || 0
        cost += r.estimated_total_cost || 0
        totalTokens += r.sum_total_tokens || 0
      }
      map.set(w.key, {
        totalRequests,
        totalErrors,
        // 统一口径：error/(success+error)（见 platformUserData.errorRateOf）。
        errorRate: errorRateOf(totalSuccess, totalErrors),
        activeUsers,
        cost,
        totalTokens,
      })
    })
    return map
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [windows, weekQueries, memberUidSet])

  const loading =
    enabled &&
    (treeQ.isLoading || memberQueries.some((q) => q.isLoading) || weekQueries.some((q) => q.isLoading))
  const firstErr =
    (treeQ.error as Error | undefined) ||
    (memberQueries.find((q) => q.error)?.error as Error | undefined) ||
    (weekQueries.find((q) => q.error)?.error as Error | undefined)
  const hasAny = Array.from(aggByKey.values()).some((a) => a.totalRequests > 0)

  return { windows, aggByKey, loading, error: firstErr ? firstErr.message : null, hasAny }
}
