// 用户列表页（UserViewV2 的 React + 玻璃拟态迁移）。
// 逻辑/列/口径/交互 1:1 按 research/pr3-user-repo-org.md §User-1；视觉换玻璃拟态（不照搬 .kanban-native）。
// 关键：小数口径 → RatioPill（×100）；**纯客户端排序**（snake_case 字段 + sortRows，不传 order 给后端）；
//       keyword 客户端过滤 + 客户端分页；URL 同步 startDate/endDate/order。
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router'
import { getAllUsersV2 } from '@/api/endpoints'
import type { UserV2Row } from '@/api/types'
import { useUserNameMap } from '@/hooks/useUserNameMap'
import { formatDuration, formatNumber } from '@/lib/formatters'
import { formatDateParam, getDefaultDateRangeWide } from '@/lib/date'
import { parseOrder, sortRows, toOrder } from '@/lib/sort'
import { RatioPill } from '@/components/ui/RatioPill'
import { SortableTh } from '@/components/ui/SortableTh'
import { Pagination } from '@/components/ui/Pagination'
import { DateRangePicker } from '@/components/ui/DateRangePicker'
import { MetricCard } from '@/components/ui/MetricCard'

// ---- 纯函数（移植 UserViewV2）----

/** 显示名截断（>20 字截断加 …）。入参为已解析的真实用户名。 */
function shortName(name: string): string {
  const n = name || '-'
  return n.length > 20 ? `${n.slice(0, 20)}…` : n
}

// 数值列字段集合（排序时数值化；非数值列 org_name/user 走文本）。snake_case，与 native 一致（不传后端）。
const NUMERIC_FIELDS = new Set<string>([
  'merged_need_count',
  'active_need_count',
  'abandoned_need_count',
  'actual_calendar_min',
  'baseline_calendar_min',
  'calendar_ratio',
  'actual_work_min',
  'work_ratio',
  'ai_code_ratio',
  'commit_count',
  'commit_diff_lines',
  'week_count',
])

/** 排序取值器：数值列 Number（非有限 → null 沉底）；文本列 String。 */
function getterFor(field: string): (row: UserV2Row) => unknown {
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

/** 中位数（用于「日历提效中位」MetricCard）。 */
function median(values: number[]): number | null {
  const arr = values.filter((v) => Number.isFinite(v)).sort((a, b) => a - b)
  if (!arr.length) return null
  const mid = Math.floor(arr.length / 2)
  return arr.length % 2 ? arr[mid] : (arr[mid - 1] + arr[mid]) / 2
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

export default function UserList() {
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()

  // user_name 字段多为 UUID（无用），用 commits 的 git_user_name 建映射解析真实名。
  const { resolveName } = useUserNameMap()

  const state = useMemo(() => stateFromParams(searchParams), [searchParams])
  const parsedOrder = useMemo(() => parseOrder(state.order), [state.order])

  const [draftRange, setDraftRange] = useState<[string, string]>(state.dateRange)
  const [keyword, setKeyword] = useState('')
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(50)

  const [allRows, setAllRows] = useState<UserV2Row[]>([])
  const [loading, setLoading] = useState(false)
  const [errMsg, setErrMsg] = useState('')

  const commit = useCallback(
    (next: PageState) => {
      setSearchParams(buildQuery(next), { replace: true })
    },
    [setSearchParams],
  )

  // URL 变化时一次性全量拉（pageSize 1000，分页/排序/过滤全客户端）。
  useEffect(() => {
    let aborted = false
    const s = stateFromParams(searchParams)
    setLoading(true)
    setErrMsg('')
    // 翻页拉全（getAllUsersV2 内部循环到 total）：单次 pageSize:1000 会在 total>1000 时截断（内网 1462 被截到 1000）。
    // 本页分页/排序/过滤全在客户端做，必须先把全量拉回来。
    getAllUsersV2({ startDate: formatDateParam(s.dateRange[0]), endDate: formatDateParam(s.dateRange[1]) })
      .then((rows) => {
        if (aborted) return
        setAllRows(rows || [])
      })
      .catch((err: unknown) => {
        if (aborted) return
        setAllRows([])
        setErrMsg(err instanceof Error ? err.message : '获取用户列表失败')
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

  // 改 keyword 时回到第 1 页
  useEffect(() => {
    setPage(1)
  }, [keyword])

  const filteredRows = useMemo(() => {
    const kw = keyword.trim().toLowerCase()
    if (!kw) return allRows
    // 过滤同时匹配真实名（resolveName）、原始 user_name 与 user_id。
    return allRows.filter((r) =>
      `${resolveName(r.user_id)}${r.user_name || ''}${r.user_id || ''}`.toLowerCase().includes(kw),
    )
  }, [allRows, keyword, resolveName])

  const sortedRows = useMemo(() => {
    if (!parsedOrder) return filteredRows
    return sortRows(filteredRows, getterFor(parsedOrder.field), parsedOrder.desc)
  }, [filteredRows, parsedOrder])

  const pagedRows = useMemo(
    () => sortedRows.slice((page - 1) * pageSize, (page - 1) * pageSize + pageSize),
    [sortedRows, page, pageSize],
  )

  // 顶部 4 张 MetricCard 统计
  const stats = useMemo(() => {
    const merged = allRows.reduce((sum, r) => sum + (r.merged_need_count || 0), 0)
    const commits = allRows.reduce((sum, r) => sum + (r.commit_count || 0), 0)
    const ratios = allRows.map((r) => Number(r.calendar_ratio)).filter((v) => Number.isFinite(v))
    return { merged, commits, medianRatio: median(ratios) }
  }, [allRows])

  function applyFilters() {
    commit({ dateRange: draftRange, order: state.order })
    setPage(1)
  }

  function resetFilters() {
    const range = getDefaultDateRangeWide()
    setDraftRange(range)
    setKeyword('')
    setPage(1)
    commit({ dateRange: range, order: '' })
  }

  // 仅更新草稿日期；真正查询走「查询」按钮（applyFilters），对齐 Vue UserViewV2（DateRangePicker 只 v-model）。
  function onDateChange(range: [string, string]) {
    setDraftRange(range)
  }

  // 三态循环：无→升→降→无。换列从升序开始。回到第 1 页。
  function onSortChange(field: string) {
    const cur = parsedOrder
    let nextOrder: string | undefined
    if (!cur || cur.field !== field) nextOrder = toOrder(field, false)
    else if (!cur.desc) nextOrder = toOrder(field, true)
    else nextOrder = undefined
    commit({ dateRange: draftRange, order: nextOrder || '' })
    setPage(1)
  }

  const isSortActive = (field: string) => parsedOrder?.field === field
  const isSortDesc = (field: string) => parsedOrder?.field === field && parsedOrder?.desc === true

  function goToDetail(row: UserV2Row) {
    if (!row?.user_id) return
    const [start, end] = state.dateRange
    navigate({
      pathname: `/user/${encodeURIComponent(row.user_id)}`,
      search: `?${new URLSearchParams({ startDate: formatDateParam(start), endDate: formatDateParam(end) }).toString()}`,
    })
  }

  const inputCls =
    'glass rounded-lg px-3 py-1.5 text-sm bg-transparent text-gray-900 dark:text-white placeholder-gray-400 ' +
    'focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue'

  return (
    <div className="space-y-5">
      <header className="space-y-3">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">用户看板</h1>
          <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
            按用户聚合需求提效，日历提效看交付周期缩短了多少，人力提效看人工投入节省了多少。
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <DateRangePicker value={draftRange} onChange={onDateChange} />
          <input
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            placeholder="用户名/ID 过滤"
            className={`${inputCls} w-[200px]`}
          />
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

      {/* 4 张 MetricCard */}
      <section className="grid grid-cols-2 lg:grid-cols-4 gap-3">
        <MetricCard label="用户数" value={formatNumber(filteredRows.length)} />
        <MetricCard label="合并需求总数" value={formatNumber(stats.merged)} />
        <MetricCard label="Commit 总数" value={formatNumber(stats.commits)} />
        <MetricCard label="日历提效中位" value={<RatioPill value={stats.medianRatio} />} />
      </section>

      {/* table card */}
      <section className="glass rounded-2xl overflow-hidden">
        <div className="flex items-center justify-between px-5 py-3 border-b border-gray-200/50 dark:border-white/10">
          <span className="text-sm font-semibold text-gray-700 dark:text-gray-200">用户列表</span>
          <span className="text-xs text-gray-400 dark:text-gray-500">按用户汇总可计入需求</span>
        </div>

        {errMsg && (
          <div className="px-5 py-2 text-sm text-rose-600 dark:text-rose-400 bg-rose-50/50 dark:bg-rose-900/20">{errMsg}</div>
        )}

        <div className="overflow-x-auto">
          <table className="w-full text-sm border-collapse">
            <thead>
              <tr className="border-b border-gray-200/50 dark:border-white/10">
                <th className={TH}>用户</th>
                <th className={TH_NUM}>
                  <span className="inline-flex justify-end w-full">
                    <SortableTh field="merged_need_count" label="合并需求" numeric active={isSortActive('merged_need_count')} desc={isSortDesc('merged_need_count')} onSort={onSortChange} />
                  </span>
                </th>
                <th className={TH_NUM}>
                  <span className="inline-flex justify-end w-full">
                    <SortableTh field="active_need_count" label="活跃" numeric active={isSortActive('active_need_count')} desc={isSortDesc('active_need_count')} onSort={onSortChange} />
                  </span>
                </th>
                <th className={TH_NUM}>
                  <span className="inline-flex justify-end w-full">
                    <SortableTh field="abandoned_need_count" label="废弃" numeric active={isSortActive('abandoned_need_count')} desc={isSortDesc('abandoned_need_count')} onSort={onSortChange} />
                  </span>
                </th>
                <th className={TH_NUM}>
                  <span className="inline-flex justify-end w-full">
                    <SortableTh field="actual_calendar_min" label="实际周期" numeric active={isSortActive('actual_calendar_min')} desc={isSortDesc('actual_calendar_min')} onSort={onSortChange} />
                  </span>
                </th>
                <th className={TH_NUM}>
                  <span className="inline-flex justify-end w-full">
                    <SortableTh field="baseline_calendar_min" label="传统周期预估" numeric active={isSortActive('baseline_calendar_min')} desc={isSortDesc('baseline_calendar_min')} onSort={onSortChange} />
                  </span>
                </th>
                <th className={TH}>
                  <SortableTh field="calendar_ratio" label="日历提效" active={isSortActive('calendar_ratio')} desc={isSortDesc('calendar_ratio')} onSort={onSortChange} />
                </th>
                <th className={TH_NUM}>
                  <span className="inline-flex justify-end w-full">
                    <SortableTh field="actual_work_min" label="实际人力" numeric active={isSortActive('actual_work_min')} desc={isSortDesc('actual_work_min')} onSort={onSortChange} />
                  </span>
                </th>
                <th className={TH}>
                  <SortableTh field="work_ratio" label="人力提效" active={isSortActive('work_ratio')} desc={isSortDesc('work_ratio')} onSort={onSortChange} />
                </th>
                <th className={TH}>
                  <SortableTh field="ai_code_ratio" label="AI 代码占比" active={isSortActive('ai_code_ratio')} desc={isSortDesc('ai_code_ratio')} onSort={onSortChange} />
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
                <th className={TH_NUM}>
                  <span className="inline-flex justify-end w-full">
                    <SortableTh field="week_count" label="活跃周" numeric active={isSortActive('week_count')} desc={isSortDesc('week_count')} onSort={onSortChange} />
                  </span>
                </th>
              </tr>
            </thead>
            <tbody>
              {loading ? (
                Array.from({ length: 8 }).map((_, i) => (
                  <tr key={i} className="border-b border-gray-100/50 dark:border-white/5">
                    <td className={TD} colSpan={13}>
                      <div className="skeleton h-6 rounded" />
                    </td>
                  </tr>
                ))
              ) : filteredRows.length === 0 ? (
                <tr>
                  <td colSpan={13}>
                    <div className="py-12 text-center text-sm text-gray-400 dark:text-gray-500">暂无用户数据</div>
                  </td>
                </tr>
              ) : (
                pagedRows.map((row) => (
                  <tr
                    key={row.user_id}
                    onClick={() => goToDetail(row)}
                    className="border-b border-gray-100/50 dark:border-white/5 cursor-pointer hover:bg-apple-blue/5 dark:hover:bg-white/5 transition-colors"
                  >
                    <td className={TD}>
                      <button
                        type="button"
                        className="text-apple-blue hover:text-apple-blue-hover cursor-pointer bg-transparent border-none p-0 focus:outline-none focus-visible:underline"
                        title={resolveName(row.user_id)}
                        onClick={(e) => {
                          e.stopPropagation()
                          goToDetail(row)
                        }}
                      >
                        {shortName(resolveName(row.user_id))}
                      </button>
                    </td>
                    <td className={TD_NUM}>{row.merged_need_count}</td>
                    <td className={TD_NUM}>{row.active_need_count}</td>
                    <td className={TD_NUM}>{row.abandoned_need_count}</td>
                    <td className={TD_NUM}>{formatDuration(row.actual_calendar_min)}</td>
                    <td className={TD_NUM}>{formatDuration(row.baseline_calendar_min)}</td>
                    <td className={TD}><RatioPill value={row.calendar_ratio} /></td>
                    <td className={TD_NUM}>{formatDuration(row.actual_work_min)}</td>
                    <td className={TD}><RatioPill value={row.work_ratio} /></td>
                    <td className={TD}><RatioPill value={row.ai_code_ratio} /></td>
                    <td className={TD_NUM}>{row.commit_count}</td>
                    <td className={TD_NUM}>{formatNumber(row.commit_diff_lines, 0)}</td>
                    <td className={TD_NUM}>{row.week_count}</td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>

        <div className="px-5 py-3 border-t border-gray-200/50 dark:border-white/10">
          <Pagination
            page={page}
            pageSize={pageSize}
            total={filteredRows.length}
            pageSizes={[20, 50, 100]}
            onPageChange={setPage}
            onSizeChange={(s) => {
              setPageSize(s)
              setPage(1)
            }}
          />
        </div>
      </section>
    </div>
  )
}
