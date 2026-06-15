// 项目列表页（纯 Need(branch) 口径，与项目详情页对齐）。
// 提效比/AI占比为**小数口径**，用 RatioPill（绝不用 PercentPill 百分比口径）。
// 无分页（getProjects 返回全量 {data:[]}）；筛选(name/仅未结束) + 全客户端排序（need_* 字段已在响应里）。
import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { useNavigate, useSearchParams } from 'react-router'
import { useQueryClient } from '@tanstack/react-query'
import { createProject, deleteProject } from '@/api/endpoints'
import { useProjectList } from '@/api/queries'
import type { ProjectListItem } from '@/api/types'
import { fmtCost } from '@/lib/formatters'
import { parseOrder, sortRows, toOrder } from '@/lib/sort'
import { SortableTh } from '@/components/ui/SortableTh'
import { RatioPill } from '@/components/ui/RatioPill'
import { Modal } from '@/components/ui/Modal'

// 全客户端排序 getter（按显示值，所见即所排；null 由 sortRows 沉底）。
const CLIENT_GETTERS: Record<string, (r: ProjectListItem) => number | null | undefined> = {
  needCount: (r) => r.need_total_count,
  needCalRatio: (r) => r.need_calendar_efficiency_ratio,
  needWorkRatio: (r) => r.need_work_efficiency_ratio,
  needAiRatio: (r) => r.need_ai_code_ratio,
  needLoc: (r) => r.need_total_loc_net,
  needWorkMin: (r) => r.need_actual_work_min,
  needCost: (r) => r.need_cost,
}

function projectEndTime(r: ProjectListItem): string | null | undefined {
  return r.end_time_manual ?? r.end_time
}
function isZeroTime(s: string | null | undefined): boolean {
  return !s || String(s).startsWith('0001-')
}
function isOngoing(r: ProjectListItem): boolean {
  return isZeroTime(projectEndTime(r))
}
function projectStartTime(r: ProjectListItem): string | null | undefined {
  return r.start_time_manual ?? r.start_time
}
function fmtDate(s: string | null | undefined): string {
  return isZeroTime(s) ? '-' : String(s).slice(0, 10)
}

const TH = 'px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_NUM = 'px-3 py-2 text-right font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_CENTER = 'px-3 py-2 text-center font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TD = 'px-3 py-2 align-middle text-gray-700 dark:text-gray-200'
const TD_NUM = 'px-3 py-2 align-middle text-right tabular-nums text-gray-700 dark:text-gray-200'
const COLSPAN = 10

export default function ProjectList() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [searchParams, setSearchParams] = useSearchParams()

  const order = searchParams.get('order') || ''
  const parsedOrder = useMemo(() => parseOrder(order), [order])
  const { data, isLoading, error, refetch } = useProjectList()

  const rows = useMemo(() => data?.data || [], [data])

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

  // 三态循环：none→asc→desc→none（全客户端排序）。
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

  const [createOpen, setCreateOpen] = useState(false)

  async function handleCreate(name: string, description: string) {
    await createProject({ name: name.trim(), description: (description || '').trim() })
    setCreateOpen(false)
    await refetch()
  }

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
            项目 = 一组 Need(branch)。提效比 / AI占比为 Need 小数口径（守恒聚合、只计干净 Need），点项目进详情查看组成与贡献者。
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
          <span className="text-sm font-semibold text-gray-700 dark:text-gray-200">项目列表（{filteredRows.length}）</span>
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
                <th className={TH_NUM}>
                  <SortableTh field="needCount" label="Needs" numeric active={isSortActive('needCount')} desc={isSortDesc('needCount')} onSort={onSortChange} />
                </th>
                <th className={TH_CENTER}>
                  <span className="inline-flex justify-center">
                    <SortableTh field="needCalRatio" label="日历提效比" active={isSortActive('needCalRatio')} desc={isSortDesc('needCalRatio')} onSort={onSortChange} />
                  </span>
                </th>
                <th className={TH_CENTER}>
                  <span className="inline-flex justify-center">
                    <SortableTh field="needWorkRatio" label="工作量提效比" active={isSortActive('needWorkRatio')} desc={isSortDesc('needWorkRatio')} onSort={onSortChange} />
                  </span>
                </th>
                <th className={TH_CENTER}>
                  <span className="inline-flex justify-center">
                    <SortableTh field="needAiRatio" label="AI占比" active={isSortActive('needAiRatio')} desc={isSortDesc('needAiRatio')} onSort={onSortChange} />
                  </span>
                </th>
                <th className={TH_NUM}>
                  <SortableTh field="needLoc" label="生成代码" numeric active={isSortActive('needLoc')} desc={isSortDesc('needLoc')} onSort={onSortChange} />
                </th>
                <th className={TH_NUM}>
                  <SortableTh field="needWorkMin" label="实际工时" numeric active={isSortActive('needWorkMin')} desc={isSortDesc('needWorkMin')} onSort={onSortChange} />
                </th>
                <th className={TH_NUM}>
                  <SortableTh field="needCost" label="费用" numeric active={isSortActive('needCost')} desc={isSortDesc('needCost')} onSort={onSortChange} />
                </th>
                <th className={`${TH} min-w-[170px]`}>时间</th>
                <th className={TH_CENTER}>操作</th>
              </tr>
            </thead>
            <tbody>
              {isLoading ? (
                Array.from({ length: 6 }).map((_, i) => (
                  <tr key={i} className="border-b border-gray-100/50 dark:border-white/5">
                    <td className={TD} colSpan={COLSPAN}>
                      <div className="skeleton h-6 rounded" />
                    </td>
                  </tr>
                ))
              ) : displayRows.length === 0 ? (
                <tr>
                  <td colSpan={COLSPAN}>
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
                    <td className={TD_NUM} title="合格 / 候选 Need">
                      {row.need_eligible_count ?? 0} <span className="text-gray-400 dark:text-gray-500">/ {row.need_total_count ?? 0}</span>
                    </td>
                    <td className="px-3 py-2 align-middle text-center"><RatioPill value={row.need_calendar_efficiency_ratio ?? null} /></td>
                    <td className="px-3 py-2 align-middle text-center"><RatioPill value={row.need_work_efficiency_ratio ?? null} /></td>
                    <td className="px-3 py-2 align-middle text-center"><RatioPill value={row.need_ai_code_ratio ?? null} /></td>
                    <td className={TD_NUM}>{row.need_total_loc_net && row.need_total_loc_net > 0 ? `${row.need_total_loc_net.toLocaleString()} 行` : '-'}</td>
                    <td className={TD_NUM}>{row.need_actual_work_min && row.need_actual_work_min > 0 ? `${(row.need_actual_work_min / 480).toFixed(1)} 人天` : '-'}</td>
                    <td className={TD_NUM}>{row.need_cost != null && row.need_cost > 0 ? `¥${fmtCost(row.need_cost)}` : '¥0'}</td>
                    <td className={TD}>
                      {isZeroTime(projectStartTime(row)) ? (
                        '-'
                      ) : (
                        <span className="text-xs">
                          {fmtDate(projectStartTime(row))} ~ {isOngoing(row) ? <span className="text-emerald-600 dark:text-emerald-400">进行中</span> : fmtDate(projectEndTime(row))}
                        </span>
                      )}
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
