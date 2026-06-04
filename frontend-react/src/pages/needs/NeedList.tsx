// Need 列表页（NeedViewV2 的 React + 玻璃拟态迁移）。
// 逻辑/列/口径/交互 1:1 按 research/pr2-need-pages.md §2；视觉换玻璃拟态（不照搬 .kanban-native）。
// 关键：6 列服务端排序（order camelCase）、URL useSearchParams 同步 + 防回环、buildQuery 省默认值。
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router'
import { getNeedsV2 } from '@/api/endpoints'
import type { NeedsV2Summary } from '@/api/types'
import {
  formatDateTimeNoYear,
  formatDuration,
} from '@/lib/formatters'
import { useUsers } from '@/api/queries'
import { formatDateParam, getDefaultDateRangeWide } from '@/lib/date'
import { parseOrder, toOrder } from '@/lib/sort'
import {
  ACTUAL_CALENDAR_TIP,
  ACTUAL_WORK_TIP,
  BASELINE_CALENDAR_TIP,
  CALENDAR_RATIO_TIP,
  FUSED_BASELINE_WORK_TIP,
  WORK_RATIO_TIP,
} from '@/lib/needMetricTips'
import { RatioPill } from '@/components/ui/RatioPill'
import { SortableTh } from '@/components/ui/SortableTh'
import { Pagination } from '@/components/ui/Pagination'
import { DateRangePicker } from '@/components/ui/DateRangePicker'
import { Tag } from '@/components/ui/Tag'

// ---- 纯函数（移植 NeedViewV2）----

const BOUNDARY_SOURCE_LABELS: Record<string, string> = {
  lv1_pr: 'PR',
  lv2_branch: '分支',
  lv3_issue: '议题',
  lv4_cluster: '聚类',
  lv5_orphan: '孤儿',
  branch: '分支',
  commit: '提交',
  session: '会话',
  manual: '手动',
}
function boundarySourceLabel(src?: string): string {
  if (!src) return '-'
  return BOUNDARY_SOURCE_LABELS[src] || src
}

// 边界来源下拉真实值（对齐后端 boundary_source：lv1_pr/lv2_branch/lv3_issue/lv4_cluster/lv5_orphan）。
// ⚠️ Codex P2 修复：旧值 commit/branch/session/manual 筛不出任何行（后端无此值）→ 列表空。
const BOUNDARY_SOURCE_OPTIONS = ['lv1_pr', 'lv2_branch', 'lv3_issue', 'lv4_cluster', 'lv5_orphan'] as const

function shortNeedId(value?: string): string {
  if (!value) return '-'
  const s = String(value)
  return s.length > 18 ? `${s.slice(0, 18)}…` : s
}

function normalizeDateQuery(value: string | null): string {
  if (!value) return ''
  const s = String(value).trim()
  if (/^\d{8}$/.test(s)) return `${s.slice(0, 4)}-${s.slice(4, 6)}-${s.slice(6, 8)}`
  if (/^\d{4}-\d{2}-\d{2}$/.test(s)) return s
  return ''
}

interface NeedFilters {
  repoAddr: string
  repoBranch: string
  userId: string
  boundarySource: string
  outlierOnly: boolean
  includeAll: boolean
}

interface PageState {
  dateRange: [string, string]
  page: number
  pageSize: number
  order: string
  filters: NeedFilters
}

const DEFAULT_FILTERS: NeedFilters = {
  repoAddr: '',
  repoBranch: '',
  userId: '',
  boundarySource: '',
  outlierOnly: false,
  includeAll: false,
}

/** 从 URLSearchParams 读出页面状态（含默认值兜底，pageSize 上限 200）。 */
function stateFromParams(sp: URLSearchParams): PageState {
  const start = normalizeDateQuery(sp.get('startDate'))
  const end = normalizeDateQuery(sp.get('endDate'))
  const dateRange: [string, string] = start && end ? [start, end] : getDefaultDateRangeWide()
  const page = Number(sp.get('page')) > 0 ? Number(sp.get('page')) : 1
  const pageSize = Number(sp.get('pageSize')) > 0 ? Math.min(Number(sp.get('pageSize')), 200) : 20
  return {
    dateRange,
    page,
    pageSize,
    order: sp.get('order') || '',
    filters: {
      repoAddr: (sp.get('repoAddr') || '').trim(),
      repoBranch: (sp.get('repoBranch') || '').trim(),
      userId: (sp.get('userId') || '').trim(),
      boundarySource: (sp.get('boundarySource') || '').trim(),
      outlierOnly: sp.get('outlierOnly') === 'true',
      includeAll: sp.get('includeAll') === 'true',
    },
  }
}

/** 组装 URL query（省略默认值：page=1/pageSize=20/空 filter/无 order 不入 URL）。 */
function buildQuery(s: PageState): Record<string, string> {
  const [start, end] = s.dateRange
  const q: Record<string, string> = {
    startDate: formatDateParam(start),
    endDate: formatDateParam(end),
  }
  if (s.page !== 1) q.page = String(s.page)
  if (s.pageSize !== 20) q.pageSize = String(s.pageSize)
  if (s.filters.repoAddr.trim()) q.repoAddr = s.filters.repoAddr.trim()
  if (s.filters.repoBranch.trim()) q.repoBranch = s.filters.repoBranch.trim()
  if (s.filters.userId.trim()) q.userId = s.filters.userId.trim()
  if (s.filters.boundarySource.trim()) q.boundarySource = s.filters.boundarySource.trim()
  if (s.filters.outlierOnly) q.outlierOnly = 'true'
  if (s.filters.includeAll) q.includeAll = 'true'
  if (s.order) q.order = s.order
  return q
}

/** 请求参数：buildQuery + 始终带 page/pageSize（无 order 时不带）。 */
function buildParams(s: PageState): Record<string, string | number> {
  const q = buildQuery(s)
  const params: Record<string, string | number> = { ...q, page: s.page, pageSize: s.pageSize }
  return params
}

const TH = 'px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_NUM = 'px-3 py-2 text-right font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TD = 'px-3 py-2 align-middle text-gray-700 dark:text-gray-200'
const TD_NUM = 'px-3 py-2 align-middle text-right tabular-nums text-gray-700 dark:text-gray-200'

export default function NeedList() {
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()

  // URL 是单一数据源；筛选输入是本地草稿（点查询/回车才写 URL）。
  const state = useMemo(() => stateFromParams(searchParams), [searchParams])
  const parsedOrder = useMemo(() => parseOrder(state.order), [state.order])

  const [draftFilters, setDraftFilters] = useState<NeedFilters>(state.filters)
  const [draftRange, setDraftRange] = useState<[string, string]>(state.dateRange)

  const [rows, setRows] = useState<NeedsV2Summary[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [errMsg, setErrMsg] = useState('')

  // 主用户列显示 user_name（needs 列表只有 primary_user_id=UUID）。拉用户列表建 user_id→user_name 映射；
  // 优先 user_name，缺失时回退 UUID。当前数据集 user_name 字段填的就是 UUID，故现在仍显示 UUID。
  const usersQuery = useUsers({ pageSize: 1000 })
  const userMap = useMemo(() => {
    const map: Record<string, string> = {}
    for (const u of usersQuery.data?.data || []) {
      if (u.user_id && u.user_name) map[u.user_id] = u.user_name
    }
    return map
  }, [usersQuery.data])

  // 把当前草稿同步到 URL（写 = setSearchParams replace）。
  // 防回环：setSearchParams(replace) 仅在结果 search 串真变时才会触发下方 effect 重拉，
  // 同串写入 react-router 不重渲染，无需额外 flag。
  const commit = useCallback(
    (next: PageState) => {
      setSearchParams(buildQuery(next), { replace: true })
    },
    [setSearchParams],
  )

  // URL 变化时重新拉数据（init + 任意 search 变化）。
  useEffect(() => {
    let aborted = false
    const s = stateFromParams(searchParams)
    setLoading(true)
    setErrMsg('')
    getNeedsV2(buildParams(s))
      .then((res) => {
        if (aborted) return
        setRows(res.data || [])
        setTotal(res.total || 0)
      })
      .catch((err: unknown) => {
        if (aborted) return
        setRows([])
        setTotal(0)
        setErrMsg(err instanceof Error ? err.message : '获取 Need 列表失败')
      })
      .finally(() => {
        if (!aborted) setLoading(false)
      })
    return () => {
      aborted = true
    }
  }, [searchParams])

  // URL 外部变化（如重置/快捷链接）时回填草稿。
  useEffect(() => {
    setDraftFilters(state.filters)
    setDraftRange(state.dateRange)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [searchParams])

  function applyFilters() {
    commit({ ...state, dateRange: draftRange, filters: draftFilters, page: 1 })
  }

  function resetFilters() {
    const range = getDefaultDateRangeWide()
    setDraftFilters({ ...DEFAULT_FILTERS })
    setDraftRange(range)
    commit({ dateRange: range, page: 1, pageSize: 20, order: '', filters: { ...DEFAULT_FILTERS } })
  }

  function onDateChange(range: [string, string]) {
    setDraftRange(range)
    // 与分页/页大小一致：带上当前草稿 filters，避免已输入未应用的筛选被 URL 回填覆盖丢失。
    commit({ ...state, dateRange: range, filters: draftFilters, page: 1 })
  }

  function handleSizeChange(size: number) {
    commit({ ...state, dateRange: draftRange, filters: draftFilters, pageSize: size, page: 1 })
  }

  function handlePageChange(p: number) {
    commit({ ...state, dateRange: draftRange, filters: draftFilters, page: p })
  }

  // 三态循环：无→升→降→无。同列推进，换列从升序开始。
  function onSortChange(field: string) {
    const cur = parsedOrder
    let nextOrder: string | undefined
    if (!cur || cur.field !== field) nextOrder = toOrder(field, false)
    else if (!cur.desc) nextOrder = toOrder(field, true)
    else nextOrder = undefined
    commit({ ...state, dateRange: draftRange, filters: draftFilters, order: nextOrder || '', page: 1 })
  }

  const isSortActive = (field: string) => parsedOrder?.field === field
  const isSortDesc = (field: string) => parsedOrder?.field === field && parsedOrder?.desc === true

  function goToDetail(row: NeedsV2Summary) {
    if (!row?.need_id) return
    navigate({
      pathname: `/needs/${encodeURIComponent(row.need_id)}`,
      search: `?${new URLSearchParams(buildQuery(state)).toString()}`,
    })
  }

  function onFilterKey(e: React.KeyboardEvent) {
    if (e.key === 'Enter') applyFilters()
  }

  const inputCls =
    'glass rounded-lg px-3 py-1.5 text-sm bg-transparent text-gray-900 dark:text-white placeholder-gray-400 ' +
    'focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue'

  return (
    <div className="space-y-5">
      {/* header */}
      <header className="space-y-3">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">需求 Need 提效</h1>
          <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
            按需求边界度量提效比，日历提效为最终业务口径，工作量提效用于诊断。
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <DateRangePicker value={draftRange} onChange={onDateChange} />
          <input
            value={draftFilters.repoAddr}
            onChange={(e) => setDraftFilters((f) => ({ ...f, repoAddr: e.target.value }))}
            onKeyDown={onFilterKey}
            placeholder="仓库地址"
            className={`${inputCls} w-[200px]`}
          />
          <input
            value={draftFilters.repoBranch}
            onChange={(e) => setDraftFilters((f) => ({ ...f, repoBranch: e.target.value }))}
            onKeyDown={onFilterKey}
            placeholder="分支"
            className={`${inputCls} w-[160px]`}
          />
          <input
            value={draftFilters.userId}
            onChange={(e) => setDraftFilters((f) => ({ ...f, userId: e.target.value }))}
            onKeyDown={onFilterKey}
            placeholder="用户 ID"
            className={`${inputCls} w-[150px]`}
          />
          <select
            value={draftFilters.boundarySource}
            onChange={(e) => setDraftFilters((f) => ({ ...f, boundarySource: e.target.value }))}
            className={`${inputCls} w-[140px] cursor-pointer`}
          >
            <option value="">边界来源</option>
            {BOUNDARY_SOURCE_OPTIONS.map((src) => (
              <option key={src} value={src}>
                {boundarySourceLabel(src)}
              </option>
            ))}
          </select>
          <label className="flex items-center gap-1.5 text-sm text-gray-600 dark:text-gray-300 cursor-pointer">
            <input
              type="checkbox"
              checked={draftFilters.outlierOnly}
              onChange={(e) => setDraftFilters((f) => ({ ...f, outlierOnly: e.target.checked }))}
              className="accent-apple-blue cursor-pointer"
            />
            仅异常
          </label>
          <label
            className="flex items-center gap-1.5 text-sm text-gray-600 dark:text-gray-300 cursor-pointer"
            title="放开看板口径：显示 active 未交付 + 主干分支 + 全部需求"
          >
            <input
              type="checkbox"
              checked={draftFilters.includeAll}
              onChange={(e) => setDraftFilters((f) => ({ ...f, includeAll: e.target.checked }))}
              className="accent-apple-blue cursor-pointer"
            />
            显示全部
          </label>
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

      {/* table card */}
      <section className="glass rounded-2xl overflow-hidden">
        <div className="flex items-center justify-between px-5 py-3 border-b border-gray-200/50 dark:border-white/10">
          <span className="text-sm font-semibold text-gray-700 dark:text-gray-200">Need 列表</span>
          <span className="text-xs text-gray-400 dark:text-gray-500">提效比按百分比展示（小数口径 ×100）</span>
        </div>

        {errMsg && (
          <div className="px-5 py-2 text-sm text-rose-600 dark:text-rose-400 bg-rose-50/50 dark:bg-rose-900/20">{errMsg}</div>
        )}

        <div className="overflow-x-auto">
          <table className="w-full text-sm border-collapse">
            <thead>
              <tr className="border-b border-gray-200/50 dark:border-white/10">
                <th className={TH}>Need ID</th>
                <th className={TH}>
                  <span className="inline-flex items-center gap-1">
                    <SortableTh field="efficiencyRatio" label="日历提效" active={isSortActive('efficiencyRatio')} desc={isSortDesc('efficiencyRatio')} onSort={onSortChange} />
                    <InfoMark tip={CALENDAR_RATIO_TIP} />
                  </span>
                </th>
                <th className={TH}>
                  <span className="inline-flex items-center gap-1">
                    <SortableTh field="workEfficiencyRatio" label="工作量提效" active={isSortActive('workEfficiencyRatio')} desc={isSortDesc('workEfficiencyRatio')} onSort={onSortChange} />
                    <InfoMark tip={WORK_RATIO_TIP} />
                  </span>
                </th>
                <th className={TH}>仓库</th>
                <th className={TH}>分支</th>
                <th className={TH}>主用户</th>
                <th className={TH_NUM}>
                  <span className="inline-flex items-center gap-1 justify-end">
                    <SortableTh field="totalCalendarMin" label="实际日历" numeric active={isSortActive('totalCalendarMin')} desc={isSortDesc('totalCalendarMin')} onSort={onSortChange} />
                    <InfoMark tip={ACTUAL_CALENDAR_TIP} />
                  </span>
                </th>
                <th className={TH_NUM}>
                  <span className="inline-flex items-center gap-1 justify-end">
                    <SortableTh field="baselineCalendarMin" label="基线日历" numeric active={isSortActive('baselineCalendarMin')} desc={isSortDesc('baselineCalendarMin')} onSort={onSortChange} />
                    <InfoMark tip={BASELINE_CALENDAR_TIP} />
                  </span>
                </th>
                <th className={TH_NUM}>
                  <span className="inline-flex items-center gap-1 justify-end">实际工作量 <InfoMark tip={ACTUAL_WORK_TIP} /></span>
                </th>
                <th className={TH_NUM}>
                  <span className="inline-flex items-center gap-1 justify-end">基线工作量 <InfoMark tip={FUSED_BASELINE_WORK_TIP} /></span>
                </th>
                <th className={TH}>质量</th>
                <th className={TH}>边界来源</th>
                <th className={TH}>
                  <SortableTh field="devStartTs" label="记录时间" active={isSortActive('devStartTs')} desc={isSortDesc('devStartTs')} onSort={onSortChange} />
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
              ) : rows.length === 0 ? (
                <tr>
                  <td colSpan={13}>
                    <div className="py-12 text-center text-sm text-gray-400 dark:text-gray-500">暂无 Need 数据</div>
                  </td>
                </tr>
              ) : (
                rows.map((row) => (
                  <tr
                    key={row.need_id}
                    onClick={() => goToDetail(row)}
                    className="border-b border-gray-100/50 dark:border-white/5 cursor-pointer hover:bg-apple-blue/5 dark:hover:bg-white/5 transition-colors"
                  >
                    <td className={TD}>
                      <button
                        type="button"
                        className="text-apple-blue hover:text-apple-blue-hover cursor-pointer bg-transparent border-none p-0 focus:outline-none focus-visible:underline"
                        onClick={(e) => {
                          e.stopPropagation()
                          goToDetail(row)
                        }}
                      >
                        {shortNeedId(row.need_id)}
                      </button>
                    </td>
                    <td className={TD}><RatioPill value={row.efficiency_ratio} /></td>
                    <td className={TD}><RatioPill value={row.work_efficiency_ratio} /></td>
                    <td className={TD}><Ellipsis text={row.repo_addr} /></td>
                    <td className={TD}><Ellipsis text={row.repo_branch} /></td>
                    <td className={TD}><Ellipsis text={row.primary_user_id ? userMap[row.primary_user_id] ?? row.primary_user_id : '-'} /></td>
                    <td className={TD_NUM}>{formatDuration(row.total_calendar_min)}</td>
                    <td className={TD_NUM}>{formatDuration(row.baseline_calendar_min)}</td>
                    <td className={TD_NUM}>{formatDuration(row.total_active_work_corrected_min)}</td>
                    <td className={TD_NUM}>{formatDuration(row.baseline_fused_work_min)}</td>
                    <td className={TD}>
                      {row.outlier_flag ? (
                        <Tag tone="error">异常</Tag>
                      ) : row.coverage_eligible ? (
                        <Tag tone="success">可计入</Tag>
                      ) : (
                        <Tag tone="neutral">未计入</Tag>
                      )}
                    </td>
                    <td className={TD}>{boundarySourceLabel(row.boundary_source)}</td>
                    <td className={TD}>{formatDateTimeNoYear(row.dev_start_ts)}</td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>

        <div className="px-5 py-3 border-t border-gray-200/50 dark:border-white/10">
          <Pagination
            page={state.page}
            pageSize={state.pageSize}
            total={total}
            onPageChange={handlePageChange}
            onSizeChange={handleSizeChange}
          />
        </div>
      </section>
    </div>
  )
}

/** 表头 ⓘ 口径提示（内联 SVG，hover/title 展示文案）。 */
function InfoMark({ tip }: { tip: string }) {
  return (
    <span className="text-gray-400 cursor-help inline-flex align-middle" title={tip} aria-label={tip}>
      <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
      </svg>
    </span>
  )
}

/** 省略文本 + title（长文本一行省略，hover 看全）。 */
function Ellipsis({ text, title }: { text?: string | null; title?: string }) {
  const display = text || '-'
  return (
    <div className="max-w-[280px] truncate" title={title ?? (text || undefined)}>
      {display}
    </div>
  )
}
