// Task 列表页（TaskViewV2 的 React + 玻璃拟态迁移）。
// 逻辑/列/口径/双模式排序 1:1 按 research/pr2-task-pages.md；视觉换玻璃拟态（不照搬 .kanban-native）。
//
// Task 维度不展示提效比（单 Task 大多算不出，提效看 Need/用户/组织层；详情页保留）。
// 双模式排序：服务端列 startTime/diffLines/cost（order camelCase）；客户端列 实际耗时/传统预估
// （sortRows，manual 优先值 manual ?? original，null 沉底）。
// manual 优先：列表显示/排序三处一致用 manual ?? original。
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router'
import { getTasksV2 } from '@/api/endpoints'
import type { TaskListItem } from '@/api/types'
import { useUserNameMap } from '@/hooks/useUserNameMap'
import { fmtCost, formatDuration, formatLocalTime } from '@/lib/formatters'
import { formatDateParam, getDefaultDateRangeWide } from '@/lib/date'
import { parseOrder, sortRows, toOrder } from '@/lib/sort'
import { SortableTh } from '@/components/ui/SortableTh'
import { Pagination } from '@/components/ui/Pagination'
import { DateRangePicker } from '@/components/ui/DateRangePicker'

// ---- manual 优先口径（§3.2，显示/排序一致）----
function getEffectiveReal(row: TaskListItem): number | null {
  return row.task_real_minutes_manual ?? row.task_real_minutes ?? null
}
function getEffectiveAncient(row: TaskListItem): number | null {
  return row.task_ancient_minutes_manual ?? row.task_ancient_minutes ?? null
}
function tokenSum(row: TaskListItem): number {
  return (row.upstream_tokens || 0) + (row.downstream_tokens || 0)
}
function orgPath(row: TaskListItem): string {
  return [row.org1, row.org2, row.org3, row.org4].filter(Boolean).join('/')
}

// 服务端排序白名单（backend sort.go taskSortFields）。列表只暴露这三列服务端排序。
const SERVER_FIELDS = new Set(['startTime', 'diffLines', 'cost'])
// 客户端列 sortRows getter（manual 优先 / 原值）
const CLIENT_GETTERS: Record<string, (r: TaskListItem) => number | null | undefined> = {
  taskRealMinutes: getEffectiveReal,
  taskAncientMinutes: getEffectiveAncient,
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
  page: number
  pageSize: number
  order: string
  userName: string
  org1: string
  org2: string
  org3: string
  org4: string
}

const DEFAULT_PAGE_SIZE = 250

function stateFromParams(sp: URLSearchParams): PageState {
  const start = normalizeDateQuery(sp.get('startDate'))
  const end = normalizeDateQuery(sp.get('endDate'))
  const dateRange: [string, string] = start && end ? [start, end] : getDefaultDateRangeWide()
  const page = Number(sp.get('page')) > 0 ? Number(sp.get('page')) : 1
  const pageSize = Number(sp.get('pageSize')) > 0 ? Number(sp.get('pageSize')) : DEFAULT_PAGE_SIZE
  return {
    dateRange,
    page,
    pageSize,
    order: sp.get('order') || '',
    userName: (sp.get('userName') || '').trim(),
    org1: (sp.get('org1') || '').trim(),
    org2: (sp.get('org2') || '').trim(),
    org3: (sp.get('org3') || '').trim(),
    org4: (sp.get('org4') || '').trim(),
  }
}

/** URL 仅同步 startDate/endDate/userName/org1~4/order（§4.3），省略默认值。 */
function buildQuery(s: PageState): Record<string, string> {
  const [start, end] = s.dateRange
  const q: Record<string, string> = {
    startDate: formatDateParam(start),
    endDate: formatDateParam(end),
  }
  if (s.page !== 1) q.page = String(s.page)
  if (s.pageSize !== DEFAULT_PAGE_SIZE) q.pageSize = String(s.pageSize)
  if (s.userName) q.userName = s.userName
  if (s.org1) q.org1 = s.org1
  if (s.org2) q.org2 = s.org2
  if (s.org3) q.org3 = s.org3
  if (s.org4) q.org4 = s.org4
  if (s.order) q.order = s.order
  return q
}

/** 请求参数：仅服务端列才下发 order（§3.4 serverOrderParam）。userName 走服务端多口径反查（全量过滤而非当前页）；org 仍客户端筛选。 */
function buildParams(s: PageState): Record<string, string | number> {
  const [start, end] = s.dateRange
  const params: Record<string, string | number> = {
    startDate: formatDateParam(start),
    endDate: formatDateParam(end),
    page: s.page,
    pageSize: s.pageSize,
  }
  if (s.userName) params.userName = s.userName
  const parsed = parseOrder(s.order)
  if (parsed && SERVER_FIELDS.has(parsed.field)) params.order = s.order
  return params
}

const TH = 'px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_NUM = 'px-3 py-2 text-right font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TD = 'px-3 py-2 align-middle text-gray-700 dark:text-gray-200'
const TD_NUM = 'px-3 py-2 align-middle text-right tabular-nums text-gray-700 dark:text-gray-200'

export default function TaskList() {
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  // task user_name 多为 UUID，用 commits 的 git_user_name 解析真实名。
  const { resolveName } = useUserNameMap()

  const state = useMemo(() => stateFromParams(searchParams), [searchParams])
  const parsedOrder = useMemo(() => parseOrder(state.order), [state.order])

  // 草稿筛选（点查询/回车才写 URL）
  const [draftRange, setDraftRange] = useState<[string, string]>(state.dateRange)
  const [draftUserName, setDraftUserName] = useState(state.userName)

  const [rows, setRows] = useState<TaskListItem[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [errMsg, setErrMsg] = useState('')

  const commit = useCallback(
    (next: PageState) => {
      setSearchParams(buildQuery(next), { replace: true })
    },
    [setSearchParams],
  )

  // URL 变化重新拉数据（同时清空选择，避免跨页/跨筛选残留选中项）
  useEffect(() => {
    let aborted = false
    const s = stateFromParams(searchParams)
    setLoading(true)
    setErrMsg('')
    getTasksV2(buildParams(s))
      .then((res) => {
        if (aborted) return
        setRows(res.data || [])
        setTotal(res.total || 0)
      })
      .catch((err: unknown) => {
        if (aborted) return
        setRows([])
        setTotal(0)
        setErrMsg(err instanceof Error ? err.message : '获取 Task 列表失败')
      })
      .finally(() => {
        if (!aborted) setLoading(false)
      })
    return () => {
      aborted = true
    }
  }, [searchParams])

  // URL 外部变化时回填草稿
  useEffect(() => {
    setDraftRange(state.dateRange)
    setDraftUserName(state.userName)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [searchParams])

  // 客户端筛选（仅 org；userName 已改走服务端多口径反查，避免只过滤当前页）。
  const filteredRows = useMemo(() => {
    let out = rows
    const orgs = [state.org1, state.org2, state.org3, state.org4]
    if (orgs.some(Boolean)) {
      out = out.filter((r) => {
        const cells = [r.org1, r.org2, r.org3, r.org4]
        return orgs.every((o, i) => !o || cells[i] === o)
      })
    }
    return out
  }, [rows, state.org1, state.org2, state.org3, state.org4])

  // 客户端列排序（命中 CLIENT_GETTERS 才在前端排；服务端列后端已排，原样）。
  const displayRows = useMemo(() => {
    if (parsedOrder && CLIENT_GETTERS[parsedOrder.field]) {
      return sortRows(filteredRows, CLIENT_GETTERS[parsedOrder.field], parsedOrder.desc)
    }
    return filteredRows
  }, [filteredRows, parsedOrder])

  function applyFilters() {
    commit({ ...state, dateRange: draftRange, userName: draftUserName.trim(), page: 1 })
  }
  function resetFilters() {
    const range = getDefaultDateRangeWide()
    setDraftRange(range)
    setDraftUserName('')
    commit({ dateRange: range, page: 1, pageSize: DEFAULT_PAGE_SIZE, order: '', userName: '', org1: '', org2: '', org3: '', org4: '' })
  }
  function onDateChange(range: [string, string]) {
    setDraftRange(range)
    commit({ ...state, dateRange: range, userName: draftUserName.trim(), page: 1 })
  }
  function handleSizeChange(size: number) {
    commit({ ...state, dateRange: draftRange, userName: draftUserName.trim(), pageSize: size, page: 1 })
  }
  function handlePageChange(p: number) {
    commit({ ...state, dateRange: draftRange, userName: draftUserName.trim(), page: p })
  }

  // 三态循环：服务端列变 order 即重拉（page=1）；客户端列只改 order（本地 displayRows 重排），不动 page。
  function onSortChange(field: string) {
    const cur = parsedOrder
    let nextOrder: string | undefined
    if (!cur || cur.field !== field) nextOrder = toOrder(field, false)
    else if (!cur.desc) nextOrder = toOrder(field, true)
    else nextOrder = undefined
    const isServer = SERVER_FIELDS.has(field)
    commit({
      ...state,
      dateRange: draftRange,
      userName: draftUserName.trim(),
      order: nextOrder || '',
      page: isServer ? 1 : state.page,
    })
  }

  const isSortActive = (field: string) => parsedOrder?.field === field
  const isSortDesc = (field: string) => parsedOrder?.field === field && parsedOrder?.desc === true

  function onFilterKey(e: React.KeyboardEvent) {
    if (e.key === 'Enter') applyFilters()
  }

  function goToTask(row: TaskListItem) {
    if (!row?.task_id) return
    navigate(`/task/${encodeURIComponent(row.task_id)}`)
  }
  function goToUser(row: TaskListItem, e: React.MouseEvent) {
    e.stopPropagation()
    const id = row.user_id || row.user_name
    if (!id) return
    navigate({
      pathname: `/user/${encodeURIComponent(id)}`,
      search: `?startDate=${formatDateParam(state.dateRange[0])}&endDate=${formatDateParam(state.dateRange[1])}`,
    })
  }
  const inputCls =
    'glass rounded-lg px-3 py-1.5 text-sm bg-transparent text-gray-900 dark:text-white placeholder-gray-400 ' +
    'focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue'

  return (
    <div className="space-y-5">
      <header className="space-y-3">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">任务 Task</h1>
          <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
            按任务查看 AI 使用明细（耗时、代码量、费用）；提效比请看需求 / 用户 / 组织层。
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <DateRangePicker value={draftRange} onChange={onDateChange} />
          <input
            value={draftUserName}
            onChange={(e) => setDraftUserName(e.target.value)}
            onKeyDown={onFilterKey}
            placeholder="用户名"
            className={`${inputCls} w-[160px]`}
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

      <section className="glass rounded-2xl overflow-hidden">
        <div className="flex flex-wrap items-center justify-between gap-2 px-5 py-3 border-b border-gray-200/50 dark:border-white/10">
          <span className="text-sm font-semibold text-gray-700 dark:text-gray-200">Task 列表</span>
        </div>

        {errMsg && (
          <div className="px-5 py-2 text-sm text-gray-600 dark:text-gray-300 bg-apple-blue/5 dark:bg-white/5">{errMsg}</div>
        )}

        <div className="overflow-x-auto">
          <table className="w-full text-sm border-collapse">
            <thead>
              <tr className="border-b border-gray-200/50 dark:border-white/10">
                <th className={TH}>Task ID</th>
                <th className={TH}>
                  <SortableTh field="startTime" label="时间" active={isSortActive('startTime')} desc={isSortDesc('startTime')} onSort={onSortChange} />
                </th>
                <th className={TH}>组织</th>
                <th className={TH}>用户</th>
                <th className={TH}>说明</th>
                <th className={TH_NUM}>
                  <SortableTh field="diffLines" label="代码量" numeric active={isSortActive('diffLines')} desc={isSortDesc('diffLines')} onSort={onSortChange} />
                </th>
                <th className={TH_NUM}>
                  <SortableTh field="taskRealMinutes" label="实际耗时" numeric active={isSortActive('taskRealMinutes')} desc={isSortDesc('taskRealMinutes')} onSort={onSortChange} />
                </th>
                <th className={TH_NUM}>
                  <SortableTh field="taskAncientMinutes" label="传统耗时预估" numeric active={isSortActive('taskAncientMinutes')} desc={isSortDesc('taskAncientMinutes')} onSort={onSortChange} />
                </th>
                <th className={TH_NUM}>Tokens消耗</th>
                <th className={TH_NUM}>费用</th>
              </tr>
            </thead>
            <tbody>
              {loading ? (
                Array.from({ length: 8 }).map((_, i) => (
                  <tr key={i} className="border-b border-gray-100/50 dark:border-white/5">
                    <td className={TD} colSpan={10}>
                      <div className="skeleton h-6 rounded" />
                    </td>
                  </tr>
                ))
              ) : displayRows.length === 0 ? (
                <tr>
                  <td colSpan={10}>
                    <div className="py-12 text-center text-sm text-gray-400 dark:text-gray-500">暂无数据</div>
                  </td>
                </tr>
              ) : (
                displayRows.map((row) => {
                  const tokens = tokenSum(row)
                  const hasOrg = !!row.org1
                  return (
                    <tr
                      key={row.task_id}
                      onClick={() => goToTask(row)}
                      className="border-b border-gray-100/50 dark:border-white/5 cursor-pointer hover:bg-apple-blue/5 dark:hover:bg-white/5 transition-colors"
                    >
                      <td className={TD}>
                        <button
                          type="button"
                          className="font-mono text-apple-blue hover:text-apple-blue-hover cursor-pointer bg-transparent border-none p-0 focus:outline-none focus-visible:underline"
                          onClick={(e) => {
                            e.stopPropagation()
                            goToTask(row)
                          }}
                        >
                          {(row.task_id || '').substring(0, 6)}
                        </button>
                      </td>
                      <td className={TD}>{formatLocalTime(row.start_time)}</td>
                      <td className={TD}>
                        {hasOrg ? (
                          <span
                            className="max-w-[200px] truncate inline-block align-bottom"
                            title={orgPath(row)}
                          >
                            {orgPath(row)}
                          </span>
                        ) : (
                          '-'
                        )}
                      </td>
                      <td className={TD}>
                        {row.user_id || row.user_name ? (
                          <button
                            type="button"
                            className="text-apple-blue hover:text-apple-blue-hover cursor-pointer bg-transparent border-none p-0 focus:outline-none focus-visible:underline"
                            title={resolveName(row.user_id || row.user_name)}
                            onClick={(e) => goToUser(row, e)}
                          >
                            {resolveName(row.user_id || row.user_name)}
                          </button>
                        ) : (
                          '-'
                        )}
                      </td>
                      <td className={TD}>
                        <div className="max-w-[280px] truncate" title={row.title}>{row.title || '-'}</div>
                      </td>
                      <td className={TD_NUM}>{row.diff_lines ?? '-'}</td>
                      <td className={TD_NUM}>{formatDuration(getEffectiveReal(row))}</td>
                      <td className={TD_NUM}>{formatDuration(getEffectiveAncient(row))}</td>
                      <td className={TD_NUM}>{tokens > 0 ? tokens.toLocaleString() : '-'}</td>
                      <td className={TD_NUM}>{row.cost != null ? fmtCost(row.cost) : '-'}</td>
                    </tr>
                  )
                })
              )}
            </tbody>
          </table>
        </div>

        <div className="px-5 py-3 border-t border-gray-200/50 dark:border-white/10">
          <Pagination
            page={state.page}
            pageSize={state.pageSize}
            total={total}
            pageSizes={[250, 500, 1000]}
            onPageChange={handlePageChange}
            onSizeChange={handleSizeChange}
          />
        </div>
      </section>
    </div>
  )
}
