// 组织列表页（OrgViewV2 的 React + 玻璃拟态迁移）。
// 逻辑/列/口径/排序 1:1 按 research/pr3-user-repo-org.md §Org-6；视觉换玻璃拟态（不照搬 .kanban-native）。
//
// ⚠️ 口径：calendar_ratio/work_ratio 是**小数口径** → RatioPill（×100）。
// 排序：native **纯客户端 sortRows**（snake_case 原字段名，**不传 order 给后端**），三态 + URL order 同步。
// 照搬陷阱：①/v2/orgs 命中 native handler（忽略 level/parent/order）。②**无分页**（全量展示）。
//          ③**无行跳转**（native <tr> 无 click，照搬不加）。④no_org_mapping 黄色警告条。
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router'
import { getOrgV2 } from '@/api/endpoints'
import type { OrgV2Row } from '@/api/types'
import { formatDuration, formatNumber } from '@/lib/formatters'
import { formatDateParam, getDefaultDateRangeWide } from '@/lib/date'
import { parseOrder, sortRows, toOrder } from '@/lib/sort'
import { RatioPill } from '@/components/ui/RatioPill'
import { SortableTh } from '@/components/ui/SortableTh'
import { DateRangePicker } from '@/components/ui/DateRangePicker'
import { MetricCard } from '@/components/ui/MetricCard'

// 数值列字段集合（snake_case 原字段名，与 native 一致，不传后端）。org_name 为文本列。
const NUMERIC_FIELDS = new Set<string>([
  'user_count',
  'merged_need_count',
  'actual_calendar_min',
  'baseline_calendar_min',
  'calendar_ratio',
  'work_ratio',
  'commit_count',
  'commit_diff_lines',
])

/** 排序取值器：数值列 Number（非有限 → null 沉底）；文本列 String。 */
function getterFor(field: string): (row: OrgV2Row) => unknown {
  if (NUMERIC_FIELDS.has(field)) {
    return (row) => {
      const v = (row as unknown as Record<string, unknown>)[field]
      const num = Number(v)
      return Number.isFinite(num) ? num : null
    }
  }
  return (row) => String((row as unknown as Record<string, unknown>)[field] ?? '')
}

function normalizeDateQuery(value: string | null): string {
  if (!value) return ''
  const s = String(value).trim()
  if (/^\d{8}$/.test(s)) return `${s.slice(0, 4)}-${s.slice(4, 6)}-${s.slice(6, 8)}`
  if (/^\d{4}-\d{2}-\d{2}$/.test(s)) return s
  return ''
}

interface PageState {
  dateRange: [string, string]
  order: string
}

function stateFromParams(sp: URLSearchParams): PageState {
  const start = normalizeDateQuery(sp.get('startDate'))
  const end = normalizeDateQuery(sp.get('endDate'))
  const dateRange: [string, string] = start && end ? [start, end] : getDefaultDateRangeWide()
  return { dateRange, order: sp.get('order') || '' }
}

function buildQuery(s: PageState): Record<string, string> {
  const [start, end] = s.dateRange
  const q: Record<string, string> = { startDate: formatDateParam(start), endDate: formatDateParam(end) }
  if (s.order) q.order = s.order
  return q
}

const TH = 'px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_NUM = 'px-3 py-2 text-right font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TD = 'px-3 py-2 align-middle text-gray-700 dark:text-gray-200'
const TD_NUM = 'px-3 py-2 align-middle text-right tabular-nums text-gray-700 dark:text-gray-200'

export default function OrgList() {
  const [searchParams, setSearchParams] = useSearchParams()

  const state = useMemo(() => stateFromParams(searchParams), [searchParams])
  const parsedOrder = useMemo(() => parseOrder(state.order), [state.order])

  const [draftRange, setDraftRange] = useState<[string, string]>(state.dateRange)
  const [rows, setRows] = useState<OrgV2Row[]>([])
  const [noOrgMapping, setNoOrgMapping] = useState(false)
  const [loading, setLoading] = useState(false)
  const [errMsg, setErrMsg] = useState('')

  const commit = useCallback(
    (next: PageState) => {
      setSearchParams(buildQuery(next), { replace: true })
    },
    [setSearchParams],
  )

  useEffect(() => {
    let aborted = false
    const s = stateFromParams(searchParams)
    setLoading(true)
    setErrMsg('')
    getOrgV2({ startDate: formatDateParam(s.dateRange[0]), endDate: formatDateParam(s.dateRange[1]) })
      .then((res) => {
        if (aborted) return
        setRows(res.data || [])
        setNoOrgMapping(res.no_org_mapping === true)
      })
      .catch((err: unknown) => {
        if (aborted) return
        setRows([])
        setNoOrgMapping(false)
        setErrMsg(err instanceof Error ? err.message : '获取组织列表失败')
      })
      .finally(() => {
        if (!aborted) setLoading(false)
      })
    return () => {
      aborted = true
    }
  }, [searchParams])

  useEffect(() => {
    setDraftRange(state.dateRange)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [searchParams])

  const sortedRows = useMemo(() => {
    if (!parsedOrder) return rows
    return sortRows(rows, getterFor(parsedOrder.field), parsedOrder.desc)
  }, [rows, parsedOrder])

  // 顶部 4 张 MetricCard
  const stats = useMemo(() => {
    const users = rows.reduce((sum, r) => sum + (r.user_count || 0), 0)
    const merged = rows.reduce((sum, r) => sum + (r.merged_need_count || 0), 0)
    const totalBaseline = rows.reduce((sum, r) => sum + (r.baseline_calendar_min || 0), 0)
    const totalActual = rows.reduce((sum, r) => sum + (r.actual_calendar_min || 0), 0)
    const overallRatio = totalActual > 0 ? (totalBaseline - totalActual) / totalActual : null
    return { users, merged, overallRatio }
  }, [rows])

  function applyFilters() {
    commit({ dateRange: draftRange, order: state.order })
  }
  function resetFilters() {
    const range = getDefaultDateRangeWide()
    setDraftRange(range)
    commit({ dateRange: range, order: '' })
  }
  function onDateChange(range: [string, string]) {
    setDraftRange(range)
  }

  // 三态循环：无→升→降→无（无分页）。
  function onSortChange(field: string) {
    const cur = parsedOrder
    let nextOrder: string | undefined
    if (!cur || cur.field !== field) nextOrder = toOrder(field, false)
    else if (!cur.desc) nextOrder = toOrder(field, true)
    else nextOrder = undefined
    commit({ dateRange: draftRange, order: nextOrder || '' })
  }

  const isSortActive = (field: string) => parsedOrder?.field === field
  const isSortDesc = (field: string) => parsedOrder?.field === field && parsedOrder?.desc === true

  return (
    <div className="space-y-5">
      <header className="space-y-3">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">组织看板</h1>
          <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
            按组织聚合需求提效，提效比为小数口径（已 ×100 展示为百分比）。
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <DateRangePicker value={draftRange} onChange={onDateChange} />
          <button
            type="button"
            onClick={applyFilters}
            disabled={loading}
            className="bg-apple-blue hover:bg-apple-blue-hover text-white rounded-lg px-4 py-1.5 text-sm font-medium cursor-pointer transition-colors disabled:opacity-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue"
          >
            查询
          </button>
          <button
            type="button"
            onClick={resetFilters}
            className="glass rounded-lg px-4 py-1.5 text-sm text-gray-700 dark:text-gray-200 cursor-pointer hover:text-apple-blue transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue"
          >
            重置
          </button>
        </div>
      </header>

      {/* no_org_mapping 警告条 */}
      {noOrgMapping && (
        <div className="glass rounded-2xl px-5 py-3 flex items-start gap-2 border-l-4 border-amber-400">
          <svg className="w-5 h-5 shrink-0 text-amber-500 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z" />
          </svg>
          <span className="text-sm text-amber-700 dark:text-amber-300">
            当前数据集缺少完整的用户↔组织映射（user_org 多数为空），未映射用户已归入「未分组」。组织维度仅供参考。
          </span>
        </div>
      )}

      {/* 4 张 MetricCard */}
      <section className="grid grid-cols-2 lg:grid-cols-4 gap-3">
        <MetricCard label="组织数" value={formatNumber(rows.length)} />
        <MetricCard label="覆盖用户" value={formatNumber(stats.users)} />
        <MetricCard label="合并需求总数" value={formatNumber(stats.merged)} />
        <MetricCard label="整体日历提效" value={<RatioPill value={stats.overallRatio} />} />
      </section>

      <section className="glass rounded-2xl overflow-hidden">
        <div className="flex items-center justify-between px-5 py-3 border-b border-gray-200/50 dark:border-white/10">
          <span className="text-sm font-semibold text-gray-700 dark:text-gray-200">组织列表</span>
          <span className="text-xs text-gray-400 dark:text-gray-500">提效比按百分比展示（小数口径 ×100）</span>
        </div>

        {errMsg && (
          <div className="px-5 py-2 text-sm text-rose-600 dark:text-rose-400 bg-rose-50/50 dark:bg-rose-900/20">{errMsg}</div>
        )}

        <div className="overflow-x-auto">
          <table className="w-full text-sm border-collapse">
            <thead>
              <tr className="border-b border-gray-200/50 dark:border-white/10">
                <th className={TH}>
                  <SortableTh field="org_name" label="组织" active={isSortActive('org_name')} desc={isSortDesc('org_name')} onSort={onSortChange} />
                </th>
                <th className={TH_NUM}>
                  <span className="inline-flex justify-end w-full">
                    <SortableTh field="user_count" label="用户数" numeric active={isSortActive('user_count')} desc={isSortDesc('user_count')} onSort={onSortChange} />
                  </span>
                </th>
                <th className={TH_NUM}>
                  <span className="inline-flex justify-end w-full">
                    <SortableTh field="merged_need_count" label="合并需求" numeric active={isSortActive('merged_need_count')} desc={isSortDesc('merged_need_count')} onSort={onSortChange} />
                  </span>
                </th>
                <th className={TH_NUM}>
                  <span className="inline-flex justify-end w-full">
                    <SortableTh field="actual_calendar_min" label="实际日历" numeric active={isSortActive('actual_calendar_min')} desc={isSortDesc('actual_calendar_min')} onSort={onSortChange} />
                  </span>
                </th>
                <th className={TH_NUM}>
                  <span className="inline-flex justify-end w-full">
                    <SortableTh field="baseline_calendar_min" label="基线日历" numeric active={isSortActive('baseline_calendar_min')} desc={isSortDesc('baseline_calendar_min')} onSort={onSortChange} />
                  </span>
                </th>
                <th className={TH}>
                  <SortableTh field="calendar_ratio" label="日历提效" active={isSortActive('calendar_ratio')} desc={isSortDesc('calendar_ratio')} onSort={onSortChange} />
                </th>
                <th className={TH}>
                  <SortableTh field="work_ratio" label="工作量提效" active={isSortActive('work_ratio')} desc={isSortDesc('work_ratio')} onSort={onSortChange} />
                </th>
                <th className={TH_NUM}>
                  <span className="inline-flex justify-end w-full">
                    <SortableTh field="commit_count" label="Commit" numeric active={isSortActive('commit_count')} desc={isSortDesc('commit_count')} onSort={onSortChange} />
                  </span>
                </th>
                <th className={TH_NUM}>
                  <span className="inline-flex justify-end w-full">
                    <SortableTh field="commit_diff_lines" label="代码行" numeric active={isSortActive('commit_diff_lines')} desc={isSortDesc('commit_diff_lines')} onSort={onSortChange} />
                  </span>
                </th>
              </tr>
            </thead>
            <tbody>
              {loading ? (
                Array.from({ length: 6 }).map((_, i) => (
                  <tr key={i} className="border-b border-gray-100/50 dark:border-white/5">
                    <td className={TD} colSpan={9}>
                      <div className="skeleton h-6 rounded" />
                    </td>
                  </tr>
                ))
              ) : !rows.length ? (
                <tr>
                  <td colSpan={9}>
                    <div className="py-12 text-center text-sm text-gray-400 dark:text-gray-500">暂无组织数据</div>
                  </td>
                </tr>
              ) : (
                sortedRows.map((row) => (
                  <tr key={row.org_name} className="border-b border-gray-100/50 dark:border-white/5">
                    <td className={TD}>{row.org_name}</td>
                    <td className={TD_NUM}>{row.user_count}</td>
                    <td className={TD_NUM}>{row.merged_need_count}</td>
                    <td className={TD_NUM}>{formatDuration(row.actual_calendar_min)}</td>
                    <td className={TD_NUM}>{formatDuration(row.baseline_calendar_min)}</td>
                    <td className={TD}><RatioPill value={row.calendar_ratio} /></td>
                    <td className={TD}><RatioPill value={row.work_ratio} /></td>
                    <td className={TD_NUM}>{row.commit_count}</td>
                    <td className={TD_NUM}>{formatNumber(row.commit_diff_lines, 0)}</td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  )
}
