// 个人维度（user）平台数据接入的数据层 —— 复用 chatGet（/api/v2/chat/* 代理，信封解包）。
// 平台是「用户(universal_id)/模型/时间」三维客观源；universal_id 与看板 user_id 同源，
// 故个人维度（聚合=用户排行 / 聚焦=单用户）直接走 /stats/users/ranking。
//
// 时间线：/stats/users/ranking 吃日期范围但**无按用户的时间桶** → 用 sliceWeekWindows 把全局
// timeRange 切成 N 个周窗口，各查一次（按需聚焦单用户）拼成「按周」序列（design：零后端切窗）。
//
// ⚠️ 类型为本层局部 interface（字段对照 chat 侧 stats.go / models.go，对齐 PlatformOverview 的
// ChatUserRankingRow），不进 src/api/types.ts（避免与并行任务冲突 + 维持平台类型页面局部惯例）。
// ⚠️ 所有查询都受 enabled 控制：chat_stats_enabled=false 时调用方传 enabled=false，绝不发请求。
import { useQuery, useQueries } from '@tanstack/react-query'
import { chatGet } from '@/api/client'
import { sliceWeekWindows, type WeekWindow } from '@/lib/weekWindows'

/** 聚合按周拉单窗的上限（无翻页拉全 → 超出时数据被截断，须 UI 标注，见 P1-2）。 */
export const AGG_PAGE_SIZE = 500

/**
 * 截断检测（P1-2）：聚合只拉前 AGG_PAGE_SIZE 名，区间真实人数(total)更大时数据被截断。
 * total 未知（如周窗未返回 total）→ 视为未截断（保守，不误报）。
 */
export function isTruncated(total: number | undefined, returnedRows: number): boolean {
  return total != null && total > returnedRows
}

/** /stats/users/ranking 单行（对齐 PlatformOverview ChatUserRankingRow / chat 侧 stats.go）。 */
export interface ChatUserRankingRow {
  universal_id: string
  username: string | null
  total_requests: number
  success_requests: number
  error_requests: number
  sum_prompt_tokens: number
  sum_completion_tokens: number
  sum_total_tokens: number
  sum_cache_tokens: number
  unique_task_count: number
  active_days: number
  estimated_total_cost: number
  avg_duration_ms: number
  error_rate: number
  max_duration_ms: number
  avg_token_output_speed: number
}

/** /stats/users/ranking 信封内分页体。 */
export interface ChatUsersRankingResp {
  total: number
  page: number
  page_size: number
  data: ChatUserRankingRow[]
}

/** 用户排行排序字段（个人维度使用/成本各维默认不同）。 */
export type UserRankingSort =
  | 'sum_total_tokens'
  | 'total_requests'
  | 'estimated_total_cost'
  | 'active_days'
  | 'error_rate'

/**
 * 个人使用/质量/成本「聚合态」共享数据源：区间聚合 Top N 用户排行。
 * sortBy 不同维度传不同；search 已 trim（调用方负责防抖）。
 */
export function useUserRanking(
  params: { startDate: string; endDate: string; sortBy: UserRankingSort; search?: string; pageSize?: number },
  enabled: boolean,
) {
  const { startDate, endDate, sortBy, search = '', pageSize = 50 } = params
  return useQuery({
    queryKey: ['dim-user-ranking', startDate, endDate, sortBy, search, pageSize],
    queryFn: () =>
      chatGet<ChatUsersRankingResp>('/stats/users/ranking', {
        start_date: startDate,
        end_date: endDate,
        sort_by: sortBy,
        page: 1,
        page_size: pageSize,
        ...(search ? { search } : {}),
      }),
    enabled,
    retry: false,
    refetchOnWindowFocus: false,
  })
}

/** 单用户区间聚合（聚焦态 KPI 用）：传 universal_id（== 看板 user_id），取该用户单行。 */
export function useUserRankingFocused(
  params: { startDate: string; endDate: string; universalId: string },
  enabled: boolean,
) {
  const { startDate, endDate, universalId } = params
  return useQuery({
    queryKey: ['dim-user-ranking-focused', startDate, endDate, universalId],
    queryFn: () =>
      chatGet<ChatUsersRankingResp>('/stats/users/ranking', {
        start_date: startDate,
        end_date: endDate,
        search: universalId,
        page: 1,
        page_size: 50,
      }),
    enabled: enabled && !!universalId,
    retry: false,
    refetchOnWindowFocus: false,
    // 排行 search 是模糊匹配，命中后从结果里挑 universal_id 严格相等的那行（见 pickFocusedRow）。
  })
}

/** 从聚焦查询结果里挑出 universal_id 严格相等的那行（search 是模糊匹配，需精确二筛）。 */
export function pickFocusedRow(
  resp: ChatUsersRankingResp | undefined,
  universalId: string,
): ChatUserRankingRow | null {
  if (!resp?.data?.length || !universalId) return null
  return resp.data.find((r) => r.universal_id === universalId) ?? null
}

/** 周时间线一个点（按维度抽取不同度量）。 */
export interface WeekSeriesPoint {
  key: string
  monday: Date
  row: ChatUserRankingRow | null
}

/**
 * 个人维度「按周时间线」——把 timeRange 切成周窗口，各窗口查一次 /stats/users/ranking。
 * universalId 非空 → 单用户该周序列（聚焦态）；空 → 全量该周聚合（聚合态，取 total/合计度量）。
 * 用 useQueries 并行各窗口；任一窗口失败不抛（retry:false + 兜底空点）。
 *
 * 返回 { windows, points, loading, error, hasAny }：
 *   - points[i].row：聚焦态=该用户单行；聚合态=null（聚合度量由调用方从 aggResp 自取，见下）。
 * 为减少请求，聚合态周序列我们只取每窗口的整体汇总（total_requests 等），不需要逐用户，
 * 故聚合态也走 page_size=1 + sort，但平台无「整窗合计行」端点 → 聚合态周序列用 Top1 不准。
 * 因此：聚合态周线我们改为对每窗口拉一页（pageSize 较大）在前端求和（见 aggregateWindow）。
 */
export function useUserWeekSeries(
  params: { startDate: string; endDate: string; universalId?: string },
  enabled: boolean,
): {
  windows: WeekWindow[]
  points: WeekSeriesPoint[]
  /** 聚合态各周整体汇总（universalId 为空时填充；聚焦态为空 Map）。 */
  aggByKey: Map<string, AggWindow>
  loading: boolean
  error: string | null
  hasAny: boolean
  /** 聚合态某窗 total>AGG_PAGE_SIZE（数据被截断，UI 应标注，见 P1-2）；聚焦态恒 false。 */
  truncated: boolean
  /** 聚合态各窗区间真实人数(total)的最大值（截断标注用文案，未知为 undefined）。 */
  maxWindowTotal?: number
} {
  const { startDate, endDate, universalId } = params
  const focused = !!universalId
  const windows = sliceWeekWindows(startDate, endDate)

  const queries = useQueries({
    queries: windows.map((w) => ({
      queryKey: ['dim-user-week', w.startDate, w.endDate, universalId ?? ''],
      queryFn: () =>
        chatGet<ChatUsersRankingResp>('/stats/users/ranking', {
          start_date: w.startDate,
          end_date: w.endDate,
          // 聚焦：search 该用户取单行；聚合：拉一大页前端求和（无整窗合计端点）。
          ...(focused
            ? { search: universalId, page: 1, page_size: 50 }
            : { sort_by: 'sum_total_tokens', page: 1, page_size: AGG_PAGE_SIZE }),
        }),
      enabled,
      retry: false,
      refetchOnWindowFocus: false,
      staleTime: 60_000,
    })),
  })

  const points: WeekSeriesPoint[] = windows.map((w, i) => {
    const resp = queries[i]?.data
    return {
      key: w.key,
      monday: w.monday,
      row: focused ? pickFocusedRow(resp, universalId!) : null,
    }
  })

  const aggByKey = new Map<string, AggWindow>()
  if (!focused) {
    windows.forEach((w, i) => {
      const resp = queries[i]?.data
      if (resp?.data) aggByKey.set(w.key, aggregateWindow(resp.data))
    })
  }

  const loading = enabled && queries.some((q) => q.isLoading)
  const firstErr = queries.find((q) => q.error)?.error as Error | undefined
  const hasAny = focused
    ? points.some((p) => p.row != null)
    : Array.from(aggByKey.values()).some((a) => a.totalRequests > 0)

  // 截断检测（聚合态）：任一周窗区间真实人数 total 超过本窗返回行数 → 该窗求和被截断。
  let truncated = false
  let maxWindowTotal: number | undefined
  if (!focused) {
    queries.forEach((q) => {
      const total = q.data?.total
      const returned = q.data?.data?.length ?? 0
      if (isTruncated(total, returned)) truncated = true
      if (total != null) maxWindowTotal = Math.max(maxWindowTotal ?? 0, total)
    })
  }

  return {
    windows,
    points,
    aggByKey,
    loading,
    error: firstErr ? firstErr.message : null,
    hasAny,
    truncated,
    maxWindowTotal,
  }
}

/** 聚合态单窗口整体汇总（前端对该周排行行求和）。 */
export interface AggWindow {
  totalRequests: number
  totalErrors: number
  errorRate: number | null
  activeUsers: number
  cost: number
  totalTokens: number
}

function aggregateWindow(rows: ChatUserRankingRow[]): AggWindow {
  let totalRequests = 0
  let totalSuccess = 0
  let totalErrors = 0
  let cost = 0
  let totalTokens = 0
  for (const r of rows) {
    totalRequests += r.total_requests || 0
    totalSuccess += r.success_requests || 0
    totalErrors += r.error_requests || 0
    cost += r.estimated_total_cost || 0
    totalTokens += r.sum_total_tokens || 0
  }
  return {
    totalRequests,
    totalErrors,
    // 统一口径：错误率 = error/(success+error)（分母不含 total，对 total 含不含错误都稳健）。
    errorRate: errorRateOf(totalSuccess, totalErrors),
    activeUsers: rows.length,
    cost,
    totalTokens,
  }
}

/**
 * 统一错误率口径（P1-1）：错误率 = error/(success+error)；分母为 0 → null（显示 '-'）。
 * 全工程聚焦/聚合/周序列共用此函数，杜绝两态分歧 / total_requests 含错误双算。
 */
export function errorRateOf(successRequests: number, errorRequests: number): number | null {
  const denom = (successRequests || 0) + (errorRequests || 0)
  return denom > 0 ? (errorRequests || 0) / denom : null
}

/** 单行的统一错误率（聚焦态用，替代后端 row.error_rate）。 */
export function rowErrorRate(row: { success_requests: number; error_requests: number }): number | null {
  return errorRateOf(row.success_requests, row.error_requests)
}
