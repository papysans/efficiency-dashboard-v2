// 仓库详情页（RepoDetailV2 的 React + 玻璃拟态迁移）。
// 分区/列/口径 1:1 按 research/pr3-user-repo-org.md §Repo-5；⚠️ 百分比口径 → PercentPill（不 ×100）。
// 「添加到 Project」弹窗为复杂二级功能（getProjects/createProject/addRepoToProject/checkProjectConflicts），
// 本 PR 先做只读详情（research 明确可分期）。
//
// 提效比计算：commitEffRatio/taskEffRatio = (ancientₘ / realₘ) * 100（manual 优先，both>0 才算，否则 0，>0 才显示）。
// 表格客户端排序（manual 优先 / 计算值），null/0 沉底。
import { useCallback, useMemo, useState, type ReactNode } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router'
import { useRepoBranches, useRepoDetail } from '@/api/queries'
import type { RepoCommitItem, TaskListItem } from '@/api/types'
import { formatDuration, formatLocalTime, formatNumber } from '@/lib/formatters'
import { getDefaultDateRangeWide } from '@/lib/date'
import { parseOrder, sortRows, toOrder } from '@/lib/sort'
import { PercentPill, percentTextClass } from '@/components/ui/PercentPill'
import { Tag } from '@/components/ui/Tag'
import { SortableTh } from '@/components/ui/SortableTh'
import { DateRangePicker } from '@/components/ui/DateRangePicker'

// manual 优先口径（commit/task 提效比 = ancient/real*100，both>0 才算）。
function commitEffRatio(row: RepoCommitItem): number {
  const ancient = row.commit_ancient_minutes_manual ?? row.commit_ancient_minutes
  const real = row.commit_real_minutes_manual ?? row.commit_real_minutes
  if (ancient != null && real != null && ancient > 0 && real > 0) return (ancient / real) * 100
  return 0
}
function taskEffRatio(row: TaskListItem): number {
  const ancient = row.task_ancient_minutes_manual ?? row.task_ancient_minutes
  const real = row.task_real_minutes_manual ?? row.task_real_minutes
  if (ancient != null && real != null && ancient > 0 && real > 0) return (ancient / real) * 100
  return 0
}
function commitReal(row: RepoCommitItem): number | null | undefined {
  return row.commit_real_minutes_manual ?? row.commit_real_minutes
}
function commitAncient(row: RepoCommitItem): number | null | undefined {
  return row.commit_ancient_minutes_manual ?? row.commit_ancient_minutes
}
function taskReal(row: TaskListItem): number | null | undefined {
  return row.task_real_minutes_manual ?? row.task_real_minutes
}
function taskAncient(row: TaskListItem): number | null | undefined {
  return row.task_ancient_minutes_manual ?? row.task_ancient_minutes
}
function tokenSum(up?: number | null, down?: number | null): number {
  return (up || 0) + (down || 0)
}

/** 硅含量 tag tone（>=80 success / >=50 primary / 其余 info）。 */
function silicaTone(v: number): 'success' | 'primary' | 'info' {
  if (v >= 80) return 'success'
  if (v >= 50) return 'primary'
  return 'info'
}

function fmtDate(ms: number): string {
  const d = new Date(ms)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

function normalizeDateQuery(value: string | null): string {
  if (!value) return ''
  const s = String(value).trim()
  if (/^\d{8}$/.test(s)) return `${s.slice(0, 4)}-${s.slice(4, 6)}-${s.slice(6, 8)}`
  if (/^\d{4}-\d{2}-\d{2}$/.test(s)) return s
  return ''
}

const TH = 'px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_NUM = 'px-3 py-2 text-right font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_CENTER = 'px-3 py-2 text-center font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TD = 'px-3 py-2 align-middle text-gray-700 dark:text-gray-200'
const TD_NUM = 'px-3 py-2 align-middle text-right tabular-nums text-gray-700 dark:text-gray-200'

export default function RepoDetail() {
  const { repoAddr: repoAddrRaw, repoBranch: repoBranchRaw } = useParams<{ repoAddr: string; repoBranch?: string }>()
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()

  // 路由 param 由 React Router 已 decode。
  const repoAddr = repoAddrRaw || ''

  // 日期取 URL（YYYYMMDD/YYYY-MM-DD）；缺则近 90 天。
  const dateRange = useMemo<[string, string]>(() => {
    const start = normalizeDateQuery(searchParams.get('startDate'))
    const end = normalizeDateQuery(searchParams.get('endDate'))
    if (start && end) return [start, end]
    return getDefaultDateRangeWide()
  }, [searchParams])

  const params = useMemo(
    () => ({ startDate: dateRange[0].replace(/-/g, ''), endDate: dateRange[1].replace(/-/g, '') }),
    [dateRange],
  )

  const { data: branchesData } = useRepoBranches(repoAddr)
  const branches = branchesData?.branches || []

  // 当前分支：URL param 优先；空则取 branches[0]（待加载）。
  const currentBranch = repoBranchRaw || branches[0] || ''

  const { data, isLoading, error } = useRepoDetail({
    repoAddr,
    repoBranch: currentBranch || undefined,
    ...params,
  })

  const commits: RepoCommitItem[] = useMemo(() => data?.commits || [], [data?.commits])
  const tasks: TaskListItem[] = useMemo(() => data?.tasks || [], [data?.tasks])
  const efficiency = data?.efficiency

  // 客户端排序（commits / tasks 各自维护 order）
  const [commitOrder, setCommitOrder] = useState('')
  const [taskOrder, setTaskOrder] = useState('')
  const parsedCommitOrder = useMemo(() => parseOrder(commitOrder), [commitOrder])
  const parsedTaskOrder = useMemo(() => parseOrder(taskOrder), [taskOrder])

  const COMMIT_GETTERS: Record<string, (r: RepoCommitItem) => unknown> = useMemo(
    () => ({
      commitTime: (r) => (r.commit_time ? new Date(r.commit_time).getTime() : null),
      diffLines: (r) => r.diff_lines,
      commitReal: (r) => commitReal(r),
      commitAncient: (r) => commitAncient(r),
      silica: (r) => r.silica,
      efficiencyRatio: (r) => (commitEffRatio(r) > 0 ? commitEffRatio(r) : null),
      cost: (r) => r.cost,
      tokens: (r) => {
        const t = tokenSum(r.upstream_tokens, r.downstream_tokens)
        return t > 0 ? t : null
      },
    }),
    [],
  )
  const TASK_GETTERS: Record<string, (r: TaskListItem) => unknown> = useMemo(
    () => ({
      startTime: (r) => (r.start_time ? new Date(r.start_time).getTime() : null),
      diffLines: (r) => r.diff_lines,
      taskReal: (r) => taskReal(r),
      taskAncient: (r) => taskAncient(r),
      efficiencyRatio: (r) => (taskEffRatio(r) > 0 ? taskEffRatio(r) : null),
      cost: (r) => r.cost,
      tokens: (r) => {
        const t = tokenSum(r.upstream_tokens, r.downstream_tokens)
        return t > 0 ? t : null
      },
    }),
    [],
  )

  const sortedCommits = useMemo(() => {
    if (parsedCommitOrder && COMMIT_GETTERS[parsedCommitOrder.field]) {
      return sortRows(commits, COMMIT_GETTERS[parsedCommitOrder.field], parsedCommitOrder.desc)
    }
    return commits
  }, [commits, parsedCommitOrder, COMMIT_GETTERS])

  const sortedTasks = useMemo(() => {
    if (parsedTaskOrder && TASK_GETTERS[parsedTaskOrder.field]) {
      return sortRows(tasks, TASK_GETTERS[parsedTaskOrder.field], parsedTaskOrder.desc)
    }
    return tasks
  }, [tasks, parsedTaskOrder, TASK_GETTERS])

  // 三态循环（commits / tasks 各自）
  const cycle = useCallback((cur: ReturnType<typeof parseOrder>, field: string): string => {
    if (!cur || cur.field !== field) return toOrder(field, false) || ''
    if (!cur.desc) return toOrder(field, true) || ''
    return ''
  }, [])

  // 派生汇总（对齐 Vue computed）
  const totalDiffLines = useMemo(() => commits.reduce((s, c) => s + (c.diff_lines || 0), 0), [commits])
  const contributorCount = useMemo(() => {
    const names = new Set<string>()
    commits.forEach((c) => c.git_user_name && names.add(c.git_user_name))
    tasks.forEach((t) => t.user_name && names.add(t.user_name))
    return names.size
  }, [commits, tasks])
  const totalTokens = useMemo(() => tasks.reduce((s, t) => s + (t.upstream_tokens || 0) + (t.downstream_tokens || 0), 0), [tasks])
  const totalCost = useMemo(() => tasks.reduce((s, t) => s + (t.cost || 0), 0), [tasks])
  const activityRange = useMemo(() => {
    const times = commits.map((c) => c.commit_time).filter(Boolean).map((t) => new Date(t as string).getTime())
    if (!times.length) return '-'
    return `${fmtDate(Math.min(...times))} ~ ${fmtDate(Math.max(...times))}`
  }, [commits])

  function handleBranchChange(branch: string) {
    const q = new URLSearchParams({ startDate: params.startDate, endDate: params.endDate })
    navigate({
      pathname: `/repo/${encodeURIComponent(repoAddr)}/${encodeURIComponent(branch)}`,
      search: `?${q.toString()}`,
    })
  }

  function onDateChange(range: [string, string]) {
    const next = new URLSearchParams(searchParams)
    next.set('startDate', range[0].replace(/-/g, ''))
    next.set('endDate', range[1].replace(/-/g, ''))
    setSearchParams(next, { replace: true })
  }

  if (error) {
    return (
      <div className="glass rounded-2xl p-8 text-center text-sm text-rose-600 dark:text-rose-400">
        {(error as Error).message || '获取仓库详情失败'}
      </div>
    )
  }

  return (
    <div className={`space-y-5 ${isLoading ? 'opacity-60 pointer-events-none' : ''}`}>
      {/* 标题栏 */}
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-3 min-w-0">
          <button
            type="button"
            onClick={() => navigate(-1)}
            className="inline-flex items-center gap-1 text-sm text-gray-500 dark:text-gray-400 hover:text-apple-blue cursor-pointer bg-transparent border-none p-0 transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue rounded"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
            </svg>
            返回
          </button>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">仓库详情</h1>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          {branches.length > 0 && (
            <select
              value={currentBranch}
              onChange={(e) => handleBranchChange(e.target.value)}
              className="glass rounded-lg px-3 py-1.5 text-sm bg-transparent cursor-pointer text-gray-700 dark:text-gray-200 focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue"
              aria-label="切换分支"
            >
              {branches.map((b) => (
                <option key={b} value={b}>
                  {b}
                </option>
              ))}
            </select>
          )}
          <DateRangePicker value={dateRange} onChange={onDateChange} />
        </div>
      </header>

      {/* 基础信息 */}
      <section className="glass rounded-2xl p-5">
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-x-6 gap-y-3 text-sm">
          <KV label="仓库地址" wide value={<span className="font-mono break-all">{repoAddr || '-'}</span>} />
          <KV label="分支" value={<span className="font-mono break-all">{currentBranch || '-'}</span>} />
          <KV label="活跃时间" value={activityRange} />
          <KV label="提交数" value={formatNumber(commits.length)} />
          <KV label="任务数" value={formatNumber(tasks.length)} />
          <KV label="总 Tokens" value={totalTokens.toLocaleString()} />
        </div>
      </section>

      {/* 度量信息 */}
      <section className="glass rounded-2xl p-5">
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-x-6 gap-y-4 text-sm">
          <div>
            <div className="text-gray-500 dark:text-gray-400 mb-1">传统开发时长预估</div>
            <div className="text-xl font-bold text-amber-600 dark:text-amber-400 tabular-nums" title={efficiency?.repo_ancient_minutes_reason}>
              {formatDuration(efficiency?.repo_ancient_minutes)}
            </div>
          </div>
          <div>
            <div className="text-gray-500 dark:text-gray-400 mb-1">实际耗时</div>
            <div className="text-xl font-bold text-sky-600 dark:text-sky-400 tabular-nums" title={efficiency?.repo_real_minutes_reason}>
              {formatDuration(efficiency?.repo_real_minutes)}
            </div>
          </div>
          <div>
            <div className="text-gray-500 dark:text-gray-400 mb-1">提效比</div>
            <div className={`text-xl font-bold tabular-nums ${percentTextClass(efficiency?.efficiency_ratio)}`}>
              {efficiency?.efficiency_ratio != null ? `${Math.round(efficiency.efficiency_ratio)}%` : '-'}
            </div>
          </div>
          <KV label="代码行数" value={`${totalDiffLines.toLocaleString()} 行`} />
          <KV label="总费用（Tasks）" value={totalCost > 0 ? `${totalCost.toFixed(2)} 元` : '-'} />
          <KV label="贡献者" value={`${contributorCount} 人`} />
        </div>
      </section>

      {/* Commits 表 */}
      <section className="glass rounded-2xl overflow-hidden">
        <div className="flex items-center justify-between px-5 py-3 border-b border-gray-200/50 dark:border-white/10">
          <span className="text-sm font-semibold text-gray-700 dark:text-gray-200">Commits</span>
          <span className="text-xs text-gray-400 dark:text-gray-500">{commits.length} 条</span>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-sm border-collapse">
            <thead>
              <tr className="border-b border-gray-200/50 dark:border-white/10">
                <th className={TH}>Commit ID</th>
                <th className={TH}>
                  <SortableTh field="commitTime" label="时间" active={parsedCommitOrder?.field === 'commitTime'} desc={parsedCommitOrder?.field === 'commitTime' && parsedCommitOrder.desc} onSort={(f) => setCommitOrder(cycle(parsedCommitOrder, f))} />
                </th>
                <th className={TH}>用户</th>
                <th className={TH}>说明</th>
                <th className={TH_NUM}>
                  <SortableTh field="diffLines" label="代码行数" numeric active={parsedCommitOrder?.field === 'diffLines'} desc={parsedCommitOrder?.field === 'diffLines' && parsedCommitOrder.desc} onSort={(f) => setCommitOrder(cycle(parsedCommitOrder, f))} />
                </th>
                <th className={TH_NUM}>
                  <SortableTh field="commitReal" label="实际耗时" numeric active={parsedCommitOrder?.field === 'commitReal'} desc={parsedCommitOrder?.field === 'commitReal' && parsedCommitOrder.desc} onSort={(f) => setCommitOrder(cycle(parsedCommitOrder, f))} />
                </th>
                <th className={TH_NUM}>
                  <SortableTh field="commitAncient" label="传统开发时长预估" numeric active={parsedCommitOrder?.field === 'commitAncient'} desc={parsedCommitOrder?.field === 'commitAncient' && parsedCommitOrder.desc} onSort={(f) => setCommitOrder(cycle(parsedCommitOrder, f))} />
                </th>
                <th className={TH_CENTER}>
                  <span className="inline-flex justify-center">
                    <SortableTh field="silica" label="硅含量" active={parsedCommitOrder?.field === 'silica'} desc={parsedCommitOrder?.field === 'silica' && parsedCommitOrder.desc} onSort={(f) => setCommitOrder(cycle(parsedCommitOrder, f))} />
                  </span>
                </th>
                <th className={TH_CENTER}>
                  <span className="inline-flex justify-center">
                    <SortableTh field="efficiencyRatio" label="提效比" active={parsedCommitOrder?.field === 'efficiencyRatio'} desc={parsedCommitOrder?.field === 'efficiencyRatio' && parsedCommitOrder.desc} onSort={(f) => setCommitOrder(cycle(parsedCommitOrder, f))} />
                  </span>
                </th>
                <th className={TH_NUM}>
                  <SortableTh field="cost" label="费用" numeric active={parsedCommitOrder?.field === 'cost'} desc={parsedCommitOrder?.field === 'cost' && parsedCommitOrder.desc} onSort={(f) => setCommitOrder(cycle(parsedCommitOrder, f))} />
                </th>
                <th className={TH_NUM}>
                  <SortableTh field="tokens" label="Tokens消耗" numeric active={parsedCommitOrder?.field === 'tokens'} desc={parsedCommitOrder?.field === 'tokens' && parsedCommitOrder.desc} onSort={(f) => setCommitOrder(cycle(parsedCommitOrder, f))} />
                </th>
              </tr>
            </thead>
            <tbody>
              {!sortedCommits.length ? (
                <tr>
                  <td colSpan={11}>
                    <div className="py-10 text-center text-sm text-gray-400 dark:text-gray-500">暂无数据</div>
                  </td>
                </tr>
              ) : (
                sortedCommits.map((c) => {
                  const eff = commitEffRatio(c)
                  const tokens = tokenSum(c.upstream_tokens, c.downstream_tokens)
                  return (
                    <tr
                      key={c.commit_id}
                      onClick={() => navigate(`/commit/${encodeURIComponent(c.commit_id)}`)}
                      className="border-b border-gray-100/50 dark:border-white/5 cursor-pointer hover:bg-apple-blue/5 dark:hover:bg-white/5 transition-colors"
                    >
                      <td className={TD}>
                        <button
                          type="button"
                          className="font-mono text-apple-blue hover:text-apple-blue-hover cursor-pointer bg-transparent border-none p-0 focus:outline-none focus-visible:underline"
                          onClick={(e) => {
                            e.stopPropagation()
                            navigate(`/commit/${encodeURIComponent(c.commit_id)}`)
                          }}
                          title={c.commit_id}
                        >
                          {(c.commit_id || '').substring(0, 8)}
                        </button>
                      </td>
                      <td className={TD}>{formatLocalTime(c.commit_time)}</td>
                      <td className={TD}><div className="max-w-[140px] truncate" title={c.git_user_name}>{c.git_user_name || '-'}</div></td>
                      <td className={TD}><div className="max-w-[260px] truncate" title={c.comment}>{c.comment || '-'}</div></td>
                      <td className={TD_NUM}>{c.diff_lines ?? 0}</td>
                      <td className={TD_NUM}>{formatDuration(commitReal(c))}</td>
                      <td className={TD_NUM}>{formatDuration(commitAncient(c))}</td>
                      <td className="px-3 py-2 align-middle text-center">
                        {c.silica != null ? <Tag tone={silicaTone(c.silica)}>{c.silica.toFixed(1)}%</Tag> : '-'}
                      </td>
                      <td className="px-3 py-2 align-middle text-center">
                        {eff > 0 ? <PercentPill value={eff} /> : '-'}
                      </td>
                      <td className={TD_NUM}>{c.cost != null && c.cost > 0 ? c.cost.toFixed(2) : '-'}</td>
                      <td className={TD_NUM}>{tokens > 0 ? tokens.toLocaleString() : '-'}</td>
                    </tr>
                  )
                })
              )}
            </tbody>
          </table>
        </div>
      </section>

      {/* Tasks 表（仅有数据才显示） */}
      {tasks.length > 0 && (
        <section className="glass rounded-2xl overflow-hidden">
          <div className="flex items-center justify-between px-5 py-3 border-b border-gray-200/50 dark:border-white/10">
            <span className="text-sm font-semibold text-gray-700 dark:text-gray-200">Tasks</span>
            <span className="text-xs text-gray-400 dark:text-gray-500">{tasks.length} 条</span>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full text-sm border-collapse">
              <thead>
                <tr className="border-b border-gray-200/50 dark:border-white/10">
                  <th className={TH}>Task ID</th>
                  <th className={TH}>
                    <SortableTh field="startTime" label="时间" active={parsedTaskOrder?.field === 'startTime'} desc={parsedTaskOrder?.field === 'startTime' && parsedTaskOrder.desc} onSort={(f) => setTaskOrder(cycle(parsedTaskOrder, f))} />
                  </th>
                  <th className={TH}>用户</th>
                  <th className={TH}>说明</th>
                  <th className={TH_NUM}>
                    <SortableTh field="diffLines" label="代码行数" numeric active={parsedTaskOrder?.field === 'diffLines'} desc={parsedTaskOrder?.field === 'diffLines' && parsedTaskOrder.desc} onSort={(f) => setTaskOrder(cycle(parsedTaskOrder, f))} />
                  </th>
                  <th className={TH_NUM}>
                    <SortableTh field="taskReal" label="实际耗时" numeric active={parsedTaskOrder?.field === 'taskReal'} desc={parsedTaskOrder?.field === 'taskReal' && parsedTaskOrder.desc} onSort={(f) => setTaskOrder(cycle(parsedTaskOrder, f))} />
                  </th>
                  <th className={TH_NUM}>
                    <SortableTh field="taskAncient" label="传统开发时长预估" numeric active={parsedTaskOrder?.field === 'taskAncient'} desc={parsedTaskOrder?.field === 'taskAncient' && parsedTaskOrder.desc} onSort={(f) => setTaskOrder(cycle(parsedTaskOrder, f))} />
                  </th>
                  <th className={TH_CENTER}>
                    <span className="inline-flex justify-center">
                      <SortableTh field="efficiencyRatio" label="提效比" active={parsedTaskOrder?.field === 'efficiencyRatio'} desc={parsedTaskOrder?.field === 'efficiencyRatio' && parsedTaskOrder.desc} onSort={(f) => setTaskOrder(cycle(parsedTaskOrder, f))} />
                    </span>
                  </th>
                  <th className={TH_NUM}>
                    <SortableTh field="cost" label="费用" numeric active={parsedTaskOrder?.field === 'cost'} desc={parsedTaskOrder?.field === 'cost' && parsedTaskOrder.desc} onSort={(f) => setTaskOrder(cycle(parsedTaskOrder, f))} />
                  </th>
                  <th className={TH_NUM}>
                    <SortableTh field="tokens" label="Tokens消耗" numeric active={parsedTaskOrder?.field === 'tokens'} desc={parsedTaskOrder?.field === 'tokens' && parsedTaskOrder.desc} onSort={(f) => setTaskOrder(cycle(parsedTaskOrder, f))} />
                  </th>
                </tr>
              </thead>
              <tbody>
                {sortedTasks.map((t) => {
                  const eff = taskEffRatio(t)
                  const tokens = tokenSum(t.upstream_tokens, t.downstream_tokens)
                  return (
                    <tr
                      key={t.task_id}
                      onClick={() => navigate(`/task/${encodeURIComponent(t.task_id)}`)}
                      className="border-b border-gray-100/50 dark:border-white/5 cursor-pointer hover:bg-apple-blue/5 dark:hover:bg-white/5 transition-colors"
                    >
                      <td className={TD}>
                        <button
                          type="button"
                          className="font-mono text-apple-blue hover:text-apple-blue-hover cursor-pointer bg-transparent border-none p-0 focus:outline-none focus-visible:underline"
                          onClick={(e) => {
                            e.stopPropagation()
                            navigate(`/task/${encodeURIComponent(t.task_id)}`)
                          }}
                          title={t.task_id}
                        >
                          {(t.task_id || '').substring(0, 8)}
                        </button>
                      </td>
                      <td className={TD}>{formatLocalTime(t.start_time)}</td>
                      <td className={TD}><div className="max-w-[140px] truncate" title={t.user_name}>{t.user_name || '-'}</div></td>
                      <td className={TD}><div className="max-w-[260px] truncate" title={t.title}>{t.title || '-'}</div></td>
                      <td className={TD_NUM}>{t.diff_lines ?? 0}</td>
                      <td className={TD_NUM}>{formatDuration(taskReal(t))}</td>
                      <td className={TD_NUM}>{formatDuration(taskAncient(t))}</td>
                      <td className="px-3 py-2 align-middle text-center">
                        {eff > 0 ? <PercentPill value={eff} /> : '-'}
                      </td>
                      <td className={TD_NUM}>{t.cost != null && t.cost > 0 ? t.cost.toFixed(2) : '-'}</td>
                      <td className={TD_NUM}>{tokens > 0 ? tokens.toLocaleString() : '-'}</td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        </section>
      )}
    </div>
  )
}

function KV({ label, value, wide = false }: { label: string; value: ReactNode; wide?: boolean }) {
  return (
    <div className={wide ? 'sm:col-span-2 lg:col-span-3' : ''}>
      <div className="text-gray-500 dark:text-gray-400">{label}</div>
      <div className="text-gray-800 dark:text-gray-100 mt-0.5">{value}</div>
    </div>
  )
}
