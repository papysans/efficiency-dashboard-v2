// 项目列表页（ProjectViewV2 的 React + 玻璃拟态迁移）。
// 逻辑/列/口径/混合排序 1:1 按 research/pr4-project-commit-workdir.md §2.1；视觉换玻璃拟态。
//
// ⚠️ efficiency_ratio 是**百分比口径**（300=300%，不 ×100），用 PercentPill，绝不用 RatioPill。
// 无分页（getProjects 返回全量 {data:[]}），客户端筛选（name/开始范围/结束范围/仅未结束）。
// 混合排序：
//   服务端列 userCount/repoCount/taskCount/totalCodeLines/actualLinesPerDay/cost（order camelCase，变 order 重拉）；
//   客户端列 projectAncientMinutes/projectRealProcessMinutes/efficiencyRatio（sortRows，manual 优先值，null 沉底）。
import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { useNavigate, useSearchParams } from 'react-router'
import { useQueryClient } from '@tanstack/react-query'
import { createProject, deleteProject } from '@/api/endpoints'
import { useProjectList } from '@/api/queries'
import type { ProjectListItem } from '@/api/types'
import { fmtCost, formatDuration, formatLocalTime } from '@/lib/formatters'
import { parseOrder, sortRows, toOrder } from '@/lib/sort'
import { SortableTh } from '@/components/ui/SortableTh'
import { PercentPill } from '@/components/ui/PercentPill'
import { Modal } from '@/components/ui/Modal'

// 服务端排序白名单（backend sort.go projectSortFields 子集，本页声明 sortField 的六列）。
const SERVER_FIELDS = new Set(['userCount', 'repoCount', 'taskCount', 'totalCodeLines', 'actualLinesPerDay', 'cost'])

// 客户端列 sortRows getter（manual 优先 / 原值，按显示值，所见即所排）。
function effAncient(r: ProjectListItem): number | null {
  return r.project_ancient_minutes_manual ?? r.project_ancient_minutes ?? null
}
function effProcess(r: ProjectListItem): number | null {
  return r.project_real_process_minutes_manual ?? r.project_real_process_minutes ?? null
}
function effLead(r: ProjectListItem): number | null {
  return r.project_real_lead_minutes_manual ?? r.project_real_lead_minutes ?? null
}
const CLIENT_GETTERS: Record<string, (r: ProjectListItem) => number | null | undefined> = {
  projectAncientMinutes: effAncient,
  projectRealProcessMinutes: effProcess,
  efficiencyRatio: (r) => r.efficiency_ratio,
}

/** end_time（manual 优先），无 → 进行中。 */
function projectEndTime(r: ProjectListItem): string | null | undefined {
  return r.end_time_manual ?? r.end_time
}
/** 0001 零时间（后端零值）算作未设置 → 进行中。 */
function isZeroTime(s: string | null | undefined): boolean {
  return !s || String(s).startsWith('0001-')
}
function isOngoing(r: ProjectListItem): boolean {
  return isZeroTime(projectEndTime(r))
}
function projectStartTime(r: ProjectListItem): string | null | undefined {
  return r.start_time_manual ?? r.start_time
}

const TH = 'px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_NUM = 'px-3 py-2 text-right font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_CENTER = 'px-3 py-2 text-center font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TD = 'px-3 py-2 align-middle text-gray-700 dark:text-gray-200'
const TD_NUM = 'px-3 py-2 align-middle text-right tabular-nums text-gray-700 dark:text-gray-200'

export default function ProjectList() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [searchParams, setSearchParams] = useSearchParams()

  const order = searchParams.get('order') || ''
  const parsedOrder = useMemo(() => parseOrder(order), [order])
  // 服务端列才把 order 下发后端（其余客户端列不消费）。
  const serverOrder = parsedOrder && SERVER_FIELDS.has(parsedOrder.field) ? order : undefined
  const { data, isLoading, error, refetch } = useProjectList(serverOrder ? { order: serverOrder } : undefined)

  const rows = useMemo(() => data?.data || [], [data])

  // 客户端筛选状态
  const [filterName, setFilterName] = useState('')
  const [filterOngoing, setFilterOngoing] = useState(false)

  const filteredRows = useMemo(() => {
    const kw = filterName.trim().toLowerCase()
    return rows.filter((r) => {
      if (kw && !(r.name || '').toLowerCase().includes(kw)) return false
      if (filterOngoing && !isOngoing(r)) return false
      return true
    })
  }, [rows, filterName, filterOngoing])

  // 客户端列排序（命中 CLIENT_GETTERS 才前端排；服务端列后端已排，原样）。
  const displayRows = useMemo(() => {
    if (parsedOrder && CLIENT_GETTERS[parsedOrder.field]) {
      return sortRows(filteredRows, CLIENT_GETTERS[parsedOrder.field], parsedOrder.desc)
    }
    return filteredRows
  }, [filteredRows, parsedOrder])

  const commitOrder = useCallback(
    (nextOrder: string | undefined) => {
      const next = new URLSearchParams(searchParams)
      if (nextOrder) next.set('order', nextOrder)
      else next.delete('order')
      setSearchParams(next, { replace: true })
    },
    [searchParams, setSearchParams],
  )

  // 三态循环：none→asc→desc→none。服务端列变 order 由 react-query 自动重拉；客户端列本地重排。
  function onSortChange(field: string) {
    const cur = parsedOrder
    let nextOrder: string | undefined
    if (!cur || cur.field !== field) nextOrder = toOrder(field, false)
    else if (!cur.desc) nextOrder = toOrder(field, true)
    else nextOrder = undefined
    commitOrder(nextOrder)
  }
  const isSortActive = (field: string) => parsedOrder?.field === field
  const isSortDesc = (field: string) => parsedOrder?.field === field && parsedOrder?.desc === true

  // 创建 Project
  const [createOpen, setCreateOpen] = useState(false)

  async function handleCreate(name: string, description: string) {
    await createProject({ name: name.trim(), description: (description || '').trim() })
    setCreateOpen(false)
    await refetch()
  }

  // 删除 Project
  const [pendingDelete, setPendingDelete] = useState<ProjectListItem | null>(null)
  const [deleting, setDeleting] = useState(false)

  async function confirmDelete() {
    if (!pendingDelete) return
    setDeleting(true)
    try {
      await deleteProject(pendingDelete.project_id)
      setPendingDelete(null)
      await queryClient.invalidateQueries({ queryKey: ['project-list'] })
      await refetch()
    } finally {
      setDeleting(false)
    }
  }

  function goToProject(row: ProjectListItem) {
    if (!row?.project_id) return
    navigate(`/project/${encodeURIComponent(row.project_id)}`)
  }

  return (
    <div className="space-y-5">
      <header className="space-y-3">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">项目 Project 提效</h1>
          <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
            虚拟项目聚合关联 Repo / Task 的古法预估 vs 实际耗时，提效比为百分比口径（300 表示提速到 4 倍）。
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <input
            type="text"
            value={filterName}
            onChange={(e) => setFilterName(e.target.value)}
            placeholder="项目名称"
            className="glass rounded-lg px-3 py-1.5 text-sm bg-transparent text-gray-900 dark:text-white placeholder-gray-400 focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue"
          />
          <label className="inline-flex items-center gap-1.5 text-sm text-gray-600 dark:text-gray-300 cursor-pointer select-none">
            <input
              type="checkbox"
              checked={filterOngoing}
              onChange={(e) => setFilterOngoing(e.target.checked)}
              className="accent-apple-blue cursor-pointer"
            />
            仅显示尚未结束
          </label>
        </div>
      </header>

      <section className="glass rounded-2xl overflow-hidden">
        <div className="flex items-center justify-between px-5 py-3 border-b border-gray-200/50 dark:border-white/10">
          <span className="text-sm font-semibold text-gray-700 dark:text-gray-200">
            项目列表（{filteredRows.length}）
          </span>
          <button
            type="button"
            onClick={() => setCreateOpen(true)}
            className="inline-flex items-center gap-1.5 bg-apple-blue hover:bg-apple-blue-hover text-white rounded-lg px-3 py-1.5 text-sm font-medium cursor-pointer transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
            </svg>
            创建项目
          </button>
        </div>

        {error && (
          <div className="px-5 py-2 text-sm text-rose-600 dark:text-rose-400 bg-rose-50/50 dark:bg-rose-900/20">
            {(error as Error).message || '获取项目列表失败'}
          </div>
        )}

        <div className="overflow-x-auto">
          <table className="w-full text-sm border-collapse">
            <thead>
              <tr className="border-b border-gray-200/50 dark:border-white/10">
                <th className={`${TH} min-w-[200px]`}>项目名称</th>
                <th className={`${TH} min-w-[150px]`}>开始时间</th>
                <th className={`${TH} min-w-[150px]`}>结束时间</th>
                <th className={TH_NUM}>
                  <SortableTh field="userCount" label="人数" numeric active={isSortActive('userCount')} desc={isSortDesc('userCount')} onSort={onSortChange} />
                </th>
                <th className={TH_NUM}>
                  <SortableTh field="repoCount" label="Repo数" numeric active={isSortActive('repoCount')} desc={isSortDesc('repoCount')} onSort={onSortChange} />
                </th>
                <th className={TH_NUM}>
                  <SortableTh field="taskCount" label="Task数" numeric active={isSortActive('taskCount')} desc={isSortDesc('taskCount')} onSort={onSortChange} />
                </th>
                <th className={TH_NUM}>
                  <SortableTh field="totalCodeLines" label="生成代码量" numeric active={isSortActive('totalCodeLines')} desc={isSortDesc('totalCodeLines')} onSort={onSortChange} />
                </th>
                <th className={TH_NUM}>
                  <SortableTh field="actualLinesPerDay" label="实际人天代码量" numeric active={isSortActive('actualLinesPerDay')} desc={isSortDesc('actualLinesPerDay')} onSort={onSortChange} />
                </th>
                <th className={TH_NUM}>
                  <SortableTh field="cost" label="费用" numeric active={isSortActive('cost')} desc={isSortDesc('cost')} onSort={onSortChange} />
                </th>
                <th className={TH_NUM}>项目周期</th>
                <th className={TH_NUM}>
                  <SortableTh field="projectAncientMinutes" label="传统开发预估" numeric active={isSortActive('projectAncientMinutes')} desc={isSortDesc('projectAncientMinutes')} onSort={onSortChange} />
                </th>
                <th className={TH_NUM}>
                  <SortableTh field="projectRealProcessMinutes" label="实际耗时" numeric active={isSortActive('projectRealProcessMinutes')} desc={isSortDesc('projectRealProcessMinutes')} onSort={onSortChange} />
                </th>
                <th className={TH_CENTER}>
                  <span className="inline-flex justify-center">
                    <SortableTh field="efficiencyRatio" label="提效比" active={isSortActive('efficiencyRatio')} desc={isSortDesc('efficiencyRatio')} onSort={onSortChange} />
                  </span>
                </th>
                <th className={TH_CENTER}>操作</th>
              </tr>
            </thead>
            <tbody>
              {isLoading ? (
                Array.from({ length: 6 }).map((_, i) => (
                  <tr key={i} className="border-b border-gray-100/50 dark:border-white/5">
                    <td className={TD} colSpan={14}>
                      <div className="skeleton h-6 rounded" />
                    </td>
                  </tr>
                ))
              ) : displayRows.length === 0 ? (
                <tr>
                  <td colSpan={14}>
                    <div className="py-12 text-center text-sm text-gray-400 dark:text-gray-500">暂无项目数据，点击「创建项目」开始</div>
                  </td>
                </tr>
              ) : (
                displayRows.map((row) => (
                  <tr
                    key={row.project_id}
                    onClick={() => goToProject(row)}
                    className="border-b border-gray-100/50 dark:border-white/5 cursor-pointer hover:bg-apple-blue/5 dark:hover:bg-white/5 transition-colors"
                  >
                    <td className={TD}>
                      <div className="max-w-[240px] truncate font-medium text-gray-900 dark:text-white" title={row.name}>{row.name || '-'}</div>
                    </td>
                    <td className={TD}>{isZeroTime(projectStartTime(row)) ? '-' : formatLocalTime(projectStartTime(row))}</td>
                    <td className={TD}>
                      {isOngoing(row) ? (
                        <span className="text-emerald-600 dark:text-emerald-400">尚未结束</span>
                      ) : (
                        formatLocalTime(projectEndTime(row))
                      )}
                    </td>
                    <td className={TD_NUM}>{row.user_count ?? '-'}</td>
                    <td className={TD_NUM}>{row.repo_count ?? '-'}</td>
                    <td className={TD_NUM}>{row.task_count ?? '-'}</td>
                    <td className={TD_NUM}>{row.total_code_lines && row.total_code_lines > 0 ? `${row.total_code_lines.toLocaleString()} 行` : '-'}</td>
                    <td className={TD_NUM}>
                      {row.actual_lines_per_day != null ? `${Math.round(row.actual_lines_per_day).toLocaleString()} 行/人天` : '-'}
                    </td>
                    <td className={TD_NUM}>{row.cost != null ? fmtCost(row.cost) : '-'}</td>
                    <td className={TD_NUM}>{formatDuration(effLead(row))}</td>
                    <td className={TD_NUM}>{formatDuration(effAncient(row))}</td>
                    <td className={TD_NUM}>{formatDuration(effProcess(row))}</td>
                    <td className="px-3 py-2 align-middle text-center">
                      <PercentPill value={row.efficiency_ratio} />
                    </td>
                    <td className="px-3 py-2 align-middle text-center">
                      <button
                        type="button"
                        onClick={(e) => {
                          e.stopPropagation()
                          setPendingDelete(row)
                        }}
                        className="text-rose-500 hover:text-rose-600 cursor-pointer bg-transparent border-none p-0 text-sm transition-colors focus:outline-none focus-visible:underline"
                      >
                        删除
                      </button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </section>

      <CreateProjectModal open={createOpen} onClose={() => setCreateOpen(false)} onSubmit={handleCreate} />

      <Modal
        open={!!pendingDelete}
        title="确认删除"
        maxWidth={420}
        onClose={() => setPendingDelete(null)}
        footer={
          <>
            <button
              type="button"
              onClick={() => setPendingDelete(null)}
              className="glass rounded-lg px-4 py-1.5 text-sm text-gray-700 dark:text-gray-200 cursor-pointer hover:text-apple-blue transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue"
            >
              取消
            </button>
            <button
              type="button"
              onClick={confirmDelete}
              disabled={deleting}
              className="bg-rose-500 hover:bg-rose-600 text-white rounded-lg px-4 py-1.5 text-sm font-medium cursor-pointer transition-colors disabled:opacity-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-rose-400"
            >
              {deleting ? '删除中...' : '删除'}
            </button>
          </>
        }
      >
        <p className="text-sm text-gray-700 dark:text-gray-200">
          确定要删除项目「{pendingDelete?.name}」吗？此操作不可撤销。
        </p>
      </Modal>
    </div>
  )
}

function CreateProjectModal({
  open,
  onClose,
  onSubmit,
}: {
  open: boolean
  onClose: () => void
  onSubmit: (name: string, description: string) => Promise<void>
}) {
  const [name, setName] = useState('')
  const [desc, setDesc] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [err, setErr] = useState('')

  useEffect(() => {
    if (!open) return
    setName('')
    setDesc('')
    setErr('')
  }, [open])

  async function handleSubmit() {
    if (!name.trim()) {
      setErr('请输入项目名称')
      return
    }
    setSubmitting(true)
    setErr('')
    try {
      await onSubmit(name, desc)
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : '创建失败')
    } finally {
      setSubmitting(false)
    }
  }

  const inputCls =
    'glass rounded-lg px-3 py-1.5 text-sm w-full bg-transparent text-gray-900 dark:text-white ' +
    'focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue'

  return (
    <Modal
      open={open}
      title="创建项目"
      maxWidth={500}
      onClose={onClose}
      footer={
        <>
          <button
            type="button"
            onClick={onClose}
            className="glass rounded-lg px-4 py-1.5 text-sm text-gray-700 dark:text-gray-200 cursor-pointer hover:text-apple-blue transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue"
          >
            取消
          </button>
          <button
            type="button"
            onClick={handleSubmit}
            disabled={submitting}
            className="bg-apple-blue hover:bg-apple-blue-hover text-white rounded-lg px-4 py-1.5 text-sm font-medium cursor-pointer transition-colors disabled:opacity-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue"
          >
            {submitting ? '创建中...' : '创建'}
          </button>
        </>
      }
    >
      <div className="space-y-3">
        {err && <div className="text-sm text-rose-600 dark:text-rose-400">{err}</div>}
        <Field label="项目名称">
          <input type="text" value={name} onChange={(e) => setName(e.target.value)} className={inputCls} />
        </Field>
        <Field label="描述（可选）">
          <textarea rows={3} value={desc} onChange={(e) => setDesc(e.target.value)} className={`${inputCls} resize-y`} />
        </Field>
      </div>
    </Modal>
  )
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="block">
      <span className="block text-xs text-gray-500 dark:text-gray-400 mb-1">{label}</span>
      {children}
    </label>
  )
}
