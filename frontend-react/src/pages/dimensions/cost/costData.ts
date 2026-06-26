// 成本看板数据层 —— 一对一对接后端 zhaoshang-show-data 的 10 个部门成本接口
// （走 /api/v2/chat 代理，chatGet 解包 {success,data}）。
// 部门接口统一前缀 /stats/departments/:dept_id/cost/*；include_children 显式转字符串（后端按 =="true" 判）。
// enabled = !!deptId（chat_stats_enabled 关闭时上层 CostKanban 整页不渲染子视图，不发请求）。
import { useQuery, keepPreviousData } from '@tanstack/react-query'
import { chatGet } from '@/api/client'
import type {
  CostAnomalyResp,
  CostModelCompositionResp,
  CostModelTrendResp,
  CostModelsResp,
  CostOverviewResp,
  CostPeriodCompareResp,
  CostSubDeptResp,
  CostTeamCompositionResp,
  CostTeamTrendResp,
  CostUsersResp,
} from './costTypes'

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

function costUrl(deptId: string, suffix: string) {
  return `/stats/departments/${encodeURIComponent(deptId)}/cost/${suffix}`
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

/** 总成本 / Token 成本 / 缓存成本 / 人均日均千Token */
export function useCostOverview(q: DeptQuery) {
  return useQuery({
    queryKey: ['cost-overview', q.deptId, q.start, q.end, q.includeChildren],
    queryFn: () => chatGet<CostOverviewResp>(costUrl(q.deptId, 'overview'), baseParams(q)),
    enabled: !!q.deptId,
  })
}

/** 费用环比（current/previous 双窗口） */
export function useCostPeriodCompare(q: DeptQuery) {
  const [prevStart, prevEnd] = computePreviousRange(q.start, q.end)
  return useQuery({
    queryKey: ['cost-period-compare', q.deptId, q.start, q.end, q.includeChildren],
    queryFn: () =>
      chatGet<CostPeriodCompareResp>(costUrl(q.deptId, 'period-compare'), {
        current_start: q.start,
        current_end: q.end,
        previous_start: prevStart,
        previous_end: prevEnd,
        include_children: q.includeChildren ? 'true' : 'false',
      }),
    enabled: !!q.deptId,
  })
}

/** 各模型费用 / 占比 / 单价 / 实际平均成本 */
export function useCostModels(q: DeptQuery) {
  return useQuery({
    queryKey: ['cost-models', q.deptId, q.start, q.end, q.includeChildren],
    queryFn: () => chatGet<CostModelsResp>(costUrl(q.deptId, 'models'), baseParams(q)),
    enabled: !!q.deptId,
  })
}

/** 各模型每日费用（堆叠面积图） */
export function useCostModelTrend(q: DeptQuery) {
  return useQuery({
    queryKey: ['cost-model-trend', q.deptId, q.start, q.end, q.includeChildren],
    queryFn: () => chatGet<CostModelTrendResp>(costUrl(q.deptId, 'model-trend'), baseParams(q)),
    enabled: !!q.deptId,
  })
}

/** 模型费用构成占比（饼图） */
export function useCostModelComposition(q: DeptQuery) {
  return useQuery({
    queryKey: ['cost-model-composition', q.deptId, q.start, q.end, q.includeChildren],
    queryFn: () => chatGet<CostModelCompositionResp>(costUrl(q.deptId, 'composition/models'), baseParams(q)),
    enabled: !!q.deptId,
  })
}

/** 异常检测：单日突增 / 单用户突增 / 费用为 0 活跃用户 */
export function useCostAnomaly(q: DeptQuery) {
  return useQuery({
    queryKey: ['cost-anomaly', q.deptId, q.start, q.end, q.includeChildren],
    queryFn: () => chatGet<CostAnomalyResp>(costUrl(q.deptId, 'anomaly'), baseParams(q)),
    enabled: !!q.deptId,
  })
}

// ============================ 子部门对比（团队） ============================

/** 各团队（直接子部门）费用对比 */
export function useCostSubDepts(q: DeptQuery) {
  return useQuery({
    queryKey: ['cost-sub-depts', q.deptId, q.start, q.end, q.includeChildren],
    queryFn: () => chatGet<CostSubDeptResp>(costUrl(q.deptId, 'sub-departments'), baseParams(q)),
    enabled: !!q.deptId,
  })
}

/** 各团队每日费用（折线） */
export function useCostTeamTrend(q: DeptQuery) {
  return useQuery({
    queryKey: ['cost-team-trend', q.deptId, q.start, q.end, q.includeChildren],
    queryFn: () => chatGet<CostTeamTrendResp>(costUrl(q.deptId, 'team-trend'), baseParams(q)),
    enabled: !!q.deptId,
  })
}

/** 团队费用构成占比（饼图） */
export function useCostTeamComposition(q: DeptQuery) {
  return useQuery({
    queryKey: ['cost-team-composition', q.deptId, q.start, q.end, q.includeChildren],
    queryFn: () => chatGet<CostTeamCompositionResp>(costUrl(q.deptId, 'composition/teams'), baseParams(q)),
    enabled: !!q.deptId,
  })
}

// ============================ 用户成本 ============================

export type CostMemberSortBy = 'total_cost' | 'input_cost' | 'output_cost' | 'total_tokens' | 'request_count'

export interface CostMembersQuery extends DeptQuery {
  page: number
  pageSize: number
  sortBy: CostMemberSortBy
  sortOrder: 'asc' | 'desc'
  search: string
}

/** 部门内各用户成本（分页/排序/搜索） */
export function useCostMembers(q: CostMembersQuery) {
  return useQuery({
    queryKey: ['cost-members', q.deptId, q.start, q.end, q.includeChildren, q.page, q.pageSize, q.sortBy, q.sortOrder, q.search],
    queryFn: () =>
      chatGet<CostUsersResp>(costUrl(q.deptId, 'users'), {
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
