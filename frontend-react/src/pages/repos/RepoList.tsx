// 仓库列表页（RepoViewV2 的 React + 玻璃拟态迁移）。
// 逻辑/列/口径/排序 1:1 按 research/pr3-user-repo-org.md §Repo-4；视觉换玻璃拟态（不照搬 KbFilterTable）。
//
// ⚠️ 口径：efficiency_ratio 是**百分比口径**（不 ×100），用 PercentPill，绝不用 RatioPill。
// 排序混合：
//   服务端列 commitCount/taskCount/startTime（order camelCase，后端分页+排序），变 order → page=1 重拉；
//   客户端列 sumAncientMinutes/sumRealMinutes/efficiencyRatio（sortRows，所见即所排，null 沉底），只本地重排不请求。
// 服务端分页（后端 total）。URL 同步 startDate/endDate/order/page/pageSize。
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router'
import { getReposV2 } from '@/api/endpoints'
import type { RepoListItem } from '@/api/types'
import { useViewState } from '@/store/viewState'
import { formatDuration } from '@/lib/formatters'
import { formatDateParam } from '@/lib/date'
import { parseOrder, sortRows, toOrder } from '@/lib/sort'
import { SortableTh } from '@/components/ui/SortableTh'
import { Pagination } from '@/components/ui/Pagination'
import { PercentPill } from '@/components/ui/PercentPill'
import { RatioPill } from '@/components/ui/RatioPill'

// 服务端排序白名单（backend sort.go repoSortFields 子集，本页声明 sortField 的三列）。
const SERVER_FIELDS = new Set(['commitCount', 'taskCount', 'startTime', 'aiCodeRatio'])
// 客户端列 sortRows getter（按显示值 = 封顶后口径，所见即所排）。
const CLIENT_GETTERS: Record<string, (r: RepoListItem) => number | null | undefined> = {
  sumAncientMinutes: (r) => r.sum_ancient_minutes,
  sumRealMinutes: (r) => r.sum_real_minutes,
  efficiencyRatio: (r) => r.efficiency_ratio,
}

interface PageState {
  page: number
  pageSize: number
  order: string
}

const DEFAULT_PAGE_SIZE = 250

function stateFromParams(sp: URLSearchParams): PageState {
  const page = Number(sp.get('page')) > 0 ? Number(sp.get('page')) : 1
  const pageSize = Number(sp.get('pageSize')) > 0 ? Number(sp.get('pageSize')) : DEFAULT_PAGE_SIZE
  return { page, pageSize, order: sp.get('order') || '' }
}

/** URL 省略默认值（page=1/pageSize=250/无 order 不入 URL）。日期走全局 timeRange，不入 URL。 */
function buildQuery(s: PageState): Record<string, string> {
  const q: Record<string, string> = {}
  if (s.page !== 1) q.page = String(s.page)
  if (s.pageSize !== DEFAULT_PAGE_SIZE) q.pageSize = String(s.pageSize)
  if (s.order) q.order = s.order
  return q
}

/** 请求参数：仅服务端列才下发 order（其余客户端列后端不消费）。日期由调用方传入（全局 timeRange）。 */
function buildParams(s: PageState, dateParams: { startDate: string; endDate: string }): Record<string, string | number> {
  const params: Record<string, string | number> = {
    startDate: dateParams.startDate,
    endDate: dateParams.endDate,
    page: s.page,
    pageSize: s.pageSize,
  }
  const parsed = parseOrder(s.order)
  if (parsed && SERVER_FIELDS.has(parsed.field)) params.order = s.order
  return params
}

const TH = 'px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_NUM = 'px-3 py-2 text-right font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_CENTER = 'px-3 py-2 text-center font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TD = 'px-3 py-2 align-middle text-gray-700 dark:text-gray-200'
const TD_NUM = 'px-3 py-2 align-middle text-right tabular-nums text-gray-700 dark:text-gray-200'

export default function RepoList() {
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  // 全局时间范围（顶部统一 DateRangePicker）——本页不再有自己的日期 picker/state。
  const { timeRange } = useViewState()

  const state = useMemo(() => stateFromParams(searchParams), [searchParams])
  const parsedOrder = useMemo(() => parseOrder(state.order), [state.order])

  const dateParams = useMemo(
    () => ({ startDate: formatDateParam(timeRange[0]), endDate: formatDateParam(timeRange[1]) }),
    [timeRange],
  )

  const [rows, setRows] = useState<RepoListItem[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [errMsg, setErrMsg] = useState('')

  const commit = useCallback(
    (next: PageState) => {
      setSearchParams(buildQuery(next), { replace: true })
    },
    [setSearchParams],
  )

  // URL（page/pageSize/order）或全局时间范围变化时重新拉数据。
  useEffect(() => {
    let aborted = false
    const s = stateFromParams(searchParams)
    setLoading(true)
    setErrMsg('')
    getReposV2(buildParams(s, dateParams))
      .then((res) => {
        if (aborted) return
        setRows(res.data || [])
        setTotal(res.total || 0)
      })
      .catch((err: unknown) => {
        if (aborted) return
        setRows([])
        setTotal(0)
        setErrMsg(err instanceof Error ? err.message : '获取仓库列表失败')
      })
      .finally(() => {
        if (!aborted) setLoading(false)
      })
    return () => {
      aborted = true
    }
  }, [searchParams, dateParams])

  // 客户端列排序（命中 CLIENT_GETTERS 才在前端排；服务端列后端已排，原样）。
  const displayRows = useMemo(() => {
    if (parsedOrder && CLIENT_GETTERS[parsedOrder.field]) {
      return sortRows(rows, CLIENT_GETTERS[parsedOrder.field], parsedOrder.desc)
    }
    return rows
  }, [rows, parsedOrder])

  function handleSizeChange(size: number) {
    commit({ ...state, pageSize: size, page: 1 })
  }
  function handlePageChange(p: number) {
    commit({ ...state, page: p })
  }

  // 三态循环：服务端列变 order 即重拉（page=1）；客户端列只改 order（本地重排），不动 page。
  function onSortChange(field: string) {
    const cur = parsedOrder
    let nextOrder: string | undefined
    if (!cur || cur.field !== field) nextOrder = toOrder(field, false)
    else if (!cur.desc) nextOrder = toOrder(field, true)
    else nextOrder = undefined
    const isServer = SERVER_FIELDS.has(field)
    commit({ ...state, order: nextOrder || '', page: isServer ? 1 : state.page })
  }

  const isSortActive = (field: string) => parsedOrder?.field === field
  const isSortDesc = (field: string) => parsedOrder?.field === field && parsedOrder?.desc === true

  function goToRepo(row: RepoListItem) {
    if (!row?.repo_addr) return
    // 整仓口径：repo_branch 已空，下钻进整仓详情（详情页内再切分支）。
    navigate(`/repo/${encodeURIComponent(row.repo_addr)}/`)
  }

  return (
    <div className="space-y-5">
      <header>
        <p className="text-sm text-gray-500 dark:text-gray-400">
          按仓库聚合古法预估 vs 实际耗时，提效比为百分比口径（300 表示提速到 4 倍）。整仓口径：跨该仓库全部分支聚合。
        </p>
      </header>

      <section className="glass rounded-2xl overflow-hidden">
        <div className="flex items-center justify-between px-5 py-3 border-b border-gray-200/50 dark:border-white/10">
          <span className="text-sm font-semibold text-gray-700 dark:text-gray-200">仓库列表</span>
          <span className="text-xs text-gray-400 dark:text-gray-500">提效比为百分比口径（不 ×100，300=300%）</span>
        </div>

        {errMsg && (
          <div className="px-5 py-2 text-sm text-rose-600 dark:text-rose-400 bg-rose-50/50 dark:bg-rose-900/20">{errMsg}</div>
        )}

        <div className="overflow-x-auto">
          <table className="w-full text-sm border-collapse">
            <thead>
              <tr className="border-b border-gray-200/50 dark:border-white/10">
                <th className={`${TH} min-w-[300px]`}>仓库地址</th>
                <th className={`${TH_NUM} min-w-[80px]`}>分支数</th>
                <th className={TH_NUM}>
                  <SortableTh field="commitCount" label="Commit数" numeric active={isSortActive('commitCount')} desc={isSortDesc('commitCount')} onSort={onSortChange} />
                </th>
                <th className={TH_NUM}>
                  <SortableTh field="taskCount" label="Task数" numeric active={isSortActive('taskCount')} desc={isSortDesc('taskCount')} onSort={onSortChange} />
                </th>
                <th className={TH_NUM}>
                  <SortableTh field="sumAncientMinutes" label="传统开发时长预估" numeric active={isSortActive('sumAncientMinutes')} desc={isSortDesc('sumAncientMinutes')} onSort={onSortChange} />
                </th>
                <th className={TH_NUM}>
                  <SortableTh field="sumRealMinutes" label="实际耗时" numeric active={isSortActive('sumRealMinutes')} desc={isSortDesc('sumRealMinutes')} onSort={onSortChange} />
                </th>
                <th className={TH_CENTER}>
                  <span className="inline-flex justify-center">
                    <SortableTh field="efficiencyRatio" label="提效比" active={isSortActive('efficiencyRatio')} desc={isSortDesc('efficiencyRatio')} onSort={onSortChange} />
                  </span>
                </th>
                <th className={TH_CENTER}>
                  <span className="inline-flex justify-center">
                    <SortableTh field="aiCodeRatio" label="AI 代码占比" active={isSortActive('aiCodeRatio')} desc={isSortDesc('aiCodeRatio')} onSort={onSortChange} />
                  </span>
                </th>
                <th className={`${TH} min-w-[150px]`}>
                  <SortableTh field="startTime" label="开始时间" active={isSortActive('startTime')} desc={isSortDesc('startTime')} onSort={onSortChange} />
                </th>
              </tr>
            </thead>
            <tbody>
              {loading ? (
                Array.from({ length: 8 }).map((_, i) => (
                  <tr key={i} className="border-b border-gray-100/50 dark:border-white/5">
                    <td className={TD} colSpan={9}>
                      <div className="skeleton h-6 rounded" />
                    </td>
                  </tr>
                ))
              ) : displayRows.length === 0 ? (
                <tr>
                  <td colSpan={9}>
                    <div className="py-12 text-center text-sm text-gray-400 dark:text-gray-500">暂无仓库数据</div>
                  </td>
                </tr>
              ) : (
                displayRows.map((row) => (
                  <tr
                    key={row.repo_addr}
                    onClick={() => goToRepo(row)}
                    className="border-b border-gray-100/50 dark:border-white/5 cursor-pointer hover:bg-apple-blue/5 dark:hover:bg-white/5 transition-colors"
                  >
                    <td className={TD}>
                      <div className="max-w-[360px] truncate" title={row.repo_addr}>{row.repo_addr || '-'}</div>
                    </td>
                    <td className={TD_NUM}>{row.branch_count ? `${row.branch_count} 支` : '-'}</td>
                    <td className={TD_NUM}>{row.commit_count}</td>
                    <td className={TD_NUM}>{row.task_count}</td>
                    <td className={TD_NUM}>{formatDuration(row.sum_ancient_minutes)}</td>
                    <td className={TD_NUM}>{formatDuration(row.sum_real_minutes)}</td>
                    <td className="px-3 py-2 align-middle text-center">
                      <PercentPill value={row.efficiency_ratio} />
                    </td>
                    <td className="px-3 py-2 align-middle text-center">
                      <RatioPill value={row.ai_code_ratio} />
                    </td>
                    {/* 后端已格式化为 date-only（2006-01-02），再过 formatLocalTime 会解析成 UTC 午夜→假 08:00:00 */}
                    <td className={TD}>{row.start_time || '-'}</td>
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
            pageSizes={[250, 500, 1000]}
            onPageChange={handlePageChange}
            onSizeChange={handleSizeChange}
          />
        </div>
      </section>
    </div>
  )
}
