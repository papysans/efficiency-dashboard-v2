// 使用看板数据层 —— 一对一对接后端 zhaoshang-show-data（走 /api/v2/chat 代理，chatGet 解包 {success,data}）。
// 部门聚合 /stats/departments/:dept_id/* ；个人 /stats/users/:uid/*。
// enabled = !!deptId / !!uid（chat_stats_enabled 关闭时上层 UsageKanban 整页不渲染子视图，不发请求）。
import { useQuery, keepPreviousData } from '@tanstack/react-query'
import { chatGet } from '@/api/client'
import type {
  DeptOverviewResp,
  DeptActiveUsersResp,
  DeptTrendResp,
  DeptModelsResp,
  DeptWeeklyResp,
  DeptResultsResp,
  DeptPeriodCompareResp,
  DeptMembersResp,
  UserDetailResp,
  UserTrendPoint,
} from './usageTypes'

/** 部门接口通用入参（include_children 显式转字符串，后端按 ==\"true\" 判）。 */
export interface DeptQuery {
  deptId: string
  start: string
  end: string
  includeChildren: boolean
}

function baseParams(q: DeptQuery) {
  return { start_date: q.start, end_date: q.end, include_children: q.includeChildren ? 'true' : 'false' }
}

function deptUrl(deptId: string, suffix: string) {
  return `/stats/departments/${encodeURIComponent(deptId)}/${suffix}`
}

// ---- 日期运算：环比「上期」= 本期向前平移一个等长窗口 ----
function addDays(dateStr: string, delta: number): string {
  const d = new Date(dateStr + 'T00:00:00')
  d.setDate(d.getDate() + delta)
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}
function computePreviousRange(start: string, end: string): [string, string] {
  const span =
    Math.round((new Date(end + 'T00:00:00').getTime() - new Date(start + 'T00:00:00').getTime()) / 86_400_000) + 1
  return [addDays(start, -span), addDays(start, -1)]
}

// ============================ 部门聚合 ============================

export function useUsageDeptOverview(q: DeptQuery) {
  return useQuery({
    queryKey: ['usage-dept-overview', q.deptId, q.start, q.end, q.includeChildren],
    queryFn: () => chatGet<DeptOverviewResp>(deptUrl(q.deptId, 'overview'), baseParams(q)),
    enabled: !!q.deptId,
  })
}

export function useUsageDeptActiveUsers(q: DeptQuery) {
  return useQuery({
    queryKey: ['usage-dept-active-users', q.deptId, q.start, q.end, q.includeChildren],
    queryFn: () => chatGet<DeptActiveUsersResp>(deptUrl(q.deptId, 'active-users'), baseParams(q)),
    enabled: !!q.deptId,
  })
}

export function useUsageDeptTrend(q: DeptQuery) {
  return useQuery({
    queryKey: ['usage-dept-trend', q.deptId, q.start, q.end, q.includeChildren],
    queryFn: () => chatGet<DeptTrendResp>(deptUrl(q.deptId, 'trend'), baseParams(q)),
    enabled: !!q.deptId,
  })
}

export function useUsageDeptModels(q: DeptQuery) {
  return useQuery({
    queryKey: ['usage-dept-models', q.deptId, q.start, q.end, q.includeChildren],
    queryFn: () => chatGet<DeptModelsResp>(deptUrl(q.deptId, 'models/usage'), baseParams(q)),
    enabled: !!q.deptId,
  })
}

export function useUsageDeptWeekly(q: DeptQuery) {
  return useQuery({
    queryKey: ['usage-dept-weekly', q.deptId, q.start, q.end, q.includeChildren],
    queryFn: () => chatGet<DeptWeeklyResp>(deptUrl(q.deptId, 'distribution/weekly'), baseParams(q)),
    enabled: !!q.deptId,
  })
}

export function useUsageDeptResults(q: DeptQuery) {
  return useQuery({
    queryKey: ['usage-dept-results', q.deptId, q.start, q.end, q.includeChildren],
    queryFn: () => chatGet<DeptResultsResp>(deptUrl(q.deptId, 'results'), baseParams(q)),
    enabled: !!q.deptId,
  })
}

export function useUsagePeriodCompare(q: DeptQuery) {
  const [prevStart, prevEnd] = computePreviousRange(q.start, q.end)
  return useQuery({
    queryKey: ['usage-dept-period-compare', q.deptId, q.start, q.end, q.includeChildren],
    queryFn: () =>
      chatGet<DeptPeriodCompareResp>(deptUrl(q.deptId, 'usage/period-compare'), {
        current_start: q.start,
        current_end: q.end,
        previous_start: prevStart,
        previous_end: prevEnd,
        include_children: q.includeChildren ? 'true' : 'false',
      }),
    enabled: !!q.deptId,
  })
}

export type MemberSortBy =
  | 'sum_total_tokens'
  | 'total_requests'
  | 'sum_prompt_tokens'
  | 'sum_completion_tokens'
  | 'active_days'
  | 'success_rate'

export interface MembersQuery extends DeptQuery {
  page: number
  pageSize: number
  sortBy: MemberSortBy
  sortOrder: 'asc' | 'desc'
  search: string
}

export function useUsageDeptMembers(q: MembersQuery) {
  return useQuery({
    queryKey: ['usage-dept-members', q.deptId, q.start, q.end, q.includeChildren, q.page, q.pageSize, q.sortBy, q.sortOrder, q.search],
    queryFn: () =>
      chatGet<DeptMembersResp>(deptUrl(q.deptId, 'members'), {
        ...baseParams(q),
        page: q.page,
        page_size: q.pageSize,
        sort_by: q.sortBy,
        sort_order: q.sortOrder,
        search: q.search || undefined,
      }),
    enabled: !!q.deptId,
    placeholderData: keepPreviousData,
  })
}

// ============================ 个人 ============================

export function useUsageUserDetail(uid: string, start: string, end: string) {
  return useQuery({
    queryKey: ['usage-user-detail', uid, start, end],
    queryFn: () =>
      chatGet<UserDetailResp>(`/stats/users/${encodeURIComponent(uid)}/detail`, {
        start_date: start,
        end_date: end,
      }),
    enabled: !!uid,
  })
}

/** 个人按天趋势：后端可能返回裸数组或 {trend:[]}，统一归一为数组。 */
export function useUsageUserTrend(uid: string, start: string, end: string) {
  return useQuery({
    queryKey: ['usage-user-trend', uid, start, end],
    queryFn: async () => {
      const r = await chatGet<unknown>(`/stats/users/${encodeURIComponent(uid)}/trend`, {
        start_date: start,
        end_date: end,
      })
      return Array.isArray(r) ? (r as UserTrendPoint[]) : ((r as { trend?: UserTrendPoint[] })?.trend ?? [])
    },
    enabled: !!uid,
  })
}
