// 工作目录详情页（WorkDirDetailV2 的 React + 玻璃拟态迁移）。
// 分区 1:1 按 research/pr4-project-commit-workdir.md §3.1；视觉换玻璃拟态。
//
// ⚠️ 复用 repo 详情 API：getRepoDetailV2(workDirId, '', {})（branch 传空字符串）。
// ⚠️ 重大 caveat（§6.1）：当前后端 RepoDetailResponse 不返回这些字段，安全降级（**不补后端**）：
//   summary.user_count / total_cost / task_ancient_minutes → '-'
//   commit.silica_reason / matched_tasks → 展开区「暂无关联 Task」
//   silica_entries → AI 代码占比图表不渲染
// 本页无 efficiency_ratio 列，唯一比例是 silica（展示为 AI 代码占比，0~1 小数口径）。
// 全部客户端排序（client sortRows，null/0 沉底）。
import { Fragment, useCallback, useMemo, useState, type ReactNode } from 'react'
import { useNavigate, useParams } from 'react-router'
import { useRepoDetail } from '@/api/queries'
import type { RepoCommitItem, TaskListItem } from '@/api/types'
import { formatLocalTime, formatNumber, formatV2Ratio } from '@/lib/formatters'
import { parseOrder, sortRows, toOrder } from '@/lib/sort'
import { SortableTh } from '@/components/ui/SortableTh'

/** AI 代码占比进度条颜色（0~1 小数口径）。 */
function silicaBarColor(silica: number): string {
  const pct = silica * 100
  if (pct >= 80) return 'bg-emerald-500'
  if (pct >= 50) return 'bg-sky-500'
  return 'bg-amber-500'
}

// WorkDir commit 行可能含 matched_tasks（当前后端不返回 → undefined，安全降级）。
interface WorkDirCommit extends RepoCommitItem {
  silica_reason?: string
  matched_tasks?: Array<{ task_id: string; user_name?: string; user_id?: string; silica?: number | null }>
}

const TH = 'px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_NUM = 'px-3 py-2 text-right font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TD = 'px-3 py-2 align-middle text-gray-700 dark:text-gray-200'
const TD_NUM = 'px-3 py-2 align-middle text-right tabular-nums text-gray-700 dark:text-gray-200'

export default function WorkDirDetail() {
  const { workDirId: rawId } = useParams<{ workDirId: string }>()
  const navigate = useNavigate()
  // 路由 param 已由 React Router decode（对齐 Vue decodeURIComponent）。
  const workDirId = rawId || ''

  // 复用 repo 详情 API，branch 传空字符串。
  const { data: detailData, isLoading, error } = useRepoDetail({ repoAddr: workDirId, repoBranch: '' })

  const commits: WorkDirCommit[] = useMemo(() => (detailData?.commits || []) as WorkDirCommit[], [detailData?.commits])
  const tasks: TaskListItem[] = useMemo(() => detailData?.tasks || [], [detailData?.tasks])
  const summary = detailData?.summary
  const repoAddr = detailData?.repo_addr || workDirId || '-'

  // 参与者聚合（§3.1 ④）：从 tasks 按 user_id 聚 task_count；再用 name→uid 映射把 commit.git_user_name 归 commit_count。
  const participants = useMemo(() => {
    const map = new Map<string, { user_id: string; user_name: string; task_count: number; commit_count: number }>()
    const nameToId = new Map<string, string>()
    tasks.forEach((t) => {
      const uid = t.user_id || ''
      if (!uid) return
      if (t.user_name) nameToId.set(t.user_name, uid)
      const cur = map.get(uid) || { user_id: uid, user_name: t.user_name || uid, task_count: 0, commit_count: 0 }
      cur.task_count += 1
      map.set(uid, cur)
    })
    commits.forEach((c) => {
      const name = c.git_user_name || ''
      const uid = nameToId.get(name)
      if (!uid) return
      const cur = map.get(uid)
      if (cur) cur.commit_count += 1
    })
    return Array.from(map.values())
  }, [tasks, commits])

  // 客户端排序（commits / participants 各自维护 order）
  const [commitOrder, setCommitOrder] = useState('')
  const [partOrder, setPartOrder] = useState('')
  const parsedCommitOrder = useMemo(() => parseOrder(commitOrder), [commitOrder])
  const parsedPartOrder = useMemo(() => parseOrder(partOrder), [partOrder])

  const COMMIT_GETTERS: Record<string, (r: WorkDirCommit) => unknown> = useMemo(
    () => ({
      commitId: (r) => r.commit_id,
      gitUserName: (r) => r.git_user_name,
      commitTime: (r) => (r.commit_time ? new Date(r.commit_time).getTime() : null),
      diffLines: (r) => r.diff_lines,
      silica: (r) => r.silica,
      matchedTasks: (r) => r.matched_tasks?.length ?? null,
    }),
    [],
  )
  const PART_GETTERS: Record<string, (r: (typeof participants)[number]) => unknown> = useMemo(
    () => ({
      taskCount: (r) => r.task_count,
      commitCount: (r) => r.commit_count,
    }),
    [],
  )

  const sortedCommits = useMemo(() => {
    if (parsedCommitOrder && COMMIT_GETTERS[parsedCommitOrder.field]) {
      return sortRows(commits, COMMIT_GETTERS[parsedCommitOrder.field], parsedCommitOrder.desc)
    }
    return commits
  }, [commits, parsedCommitOrder, COMMIT_GETTERS])

  const sortedParticipants = useMemo(() => {
    if (parsedPartOrder && PART_GETTERS[parsedPartOrder.field]) {
      return sortRows(participants, PART_GETTERS[parsedPartOrder.field], parsedPartOrder.desc)
    }
    return participants
  }, [participants, parsedPartOrder, PART_GETTERS])

  const cycle = useCallback((cur: ReturnType<typeof parseOrder>, field: string): string => {
    if (!cur || cur.field !== field) return toOrder(field, false) || ''
    if (!cur.desc) return toOrder(field, true) || ''
    return ''
  }, [])

  // 可展开行
  const [expanded, setExpanded] = useState<Set<string>>(new Set())
  function toggleRow(id: string) {
    setExpanded((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  if (error) {
    return (
      <div className="glass rounded-2xl p-8 text-center text-sm text-rose-600 dark:text-rose-400">
        {(error as Error).message || '获取工作目录详情失败'}
      </div>
    )
  }

  return (
    <div className={`space-y-5 ${isLoading ? 'opacity-60 pointer-events-none' : ''}`}>
      {/* ① 标题栏 */}
      <header className="flex flex-wrap items-center gap-3">
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
        <h1 className="text-xl font-bold text-gray-900 dark:text-white break-all">工作目录详情: {repoAddr}</h1>
      </header>

      {/* ② 仓库概览（部分字段后端未返回 → 安全降级为 '-'） */}
      <section className="glass rounded-2xl p-5">
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-x-6 gap-y-3 text-sm">
          <KV label="仓库地址" wide value={<span className="font-mono break-all">{detailData?.repo_addr || '-'}</span>} />
          <KV label="分支" value={detailData?.repo_branch || '-'} />
          <KV label="用户数" value="-" />
          <KV label="关联Task数" value={summary?.task_count != null ? formatNumber(summary.task_count) : '-'} />
          <KV label="关联Commit数" value={summary?.commit_count != null ? formatNumber(summary.commit_count) : '-'} />
          <KV label="总费用" value="-" />
          <KV label="传统开发时长预估" value="-" />
        </div>
      </section>

      {/* ③ Commit 列表（可展开；matched_tasks 后端不返回 → 展开区「暂无关联 Task」） */}
      {commits.length > 0 && (
        <section className="glass rounded-2xl overflow-hidden">
          <div className="flex items-center justify-between px-5 py-3 border-b border-gray-200/50 dark:border-white/10">
            <span className="text-sm font-semibold text-gray-700 dark:text-gray-200">Commit 列表</span>
            <span className="text-xs text-gray-400 dark:text-gray-500">{commits.length} 条</span>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full text-sm border-collapse">
              <thead>
                <tr className="border-b border-gray-200/50 dark:border-white/10">
                  <th className="px-3 py-2 w-8" />
                  <th className={TH}>
                    <SortableTh field="commitId" label="Commit ID" active={parsedCommitOrder?.field === 'commitId'} desc={parsedCommitOrder?.field === 'commitId' && parsedCommitOrder.desc} onSort={(f) => setCommitOrder(cycle(parsedCommitOrder, f))} />
                  </th>
                  <th className={TH}>
                    <SortableTh field="gitUserName" label="提交者" active={parsedCommitOrder?.field === 'gitUserName'} desc={parsedCommitOrder?.field === 'gitUserName' && parsedCommitOrder.desc} onSort={(f) => setCommitOrder(cycle(parsedCommitOrder, f))} />
                  </th>
                  <th className={TH}>
                    <SortableTh field="commitTime" label="提交时间" active={parsedCommitOrder?.field === 'commitTime'} desc={parsedCommitOrder?.field === 'commitTime' && parsedCommitOrder.desc} onSort={(f) => setCommitOrder(cycle(parsedCommitOrder, f))} />
                  </th>
                  <th className={TH_NUM}>
                    <SortableTh field="diffLines" label="Diff行数" numeric active={parsedCommitOrder?.field === 'diffLines'} desc={parsedCommitOrder?.field === 'diffLines' && parsedCommitOrder.desc} onSort={(f) => setCommitOrder(cycle(parsedCommitOrder, f))} />
                  </th>
                  <th className={TH}>
                    <SortableTh field="silica" label="AI 代码占比" active={parsedCommitOrder?.field === 'silica'} desc={parsedCommitOrder?.field === 'silica' && parsedCommitOrder.desc} onSort={(f) => setCommitOrder(cycle(parsedCommitOrder, f))} />
                  </th>
                  <th className={TH_NUM}>
                    <SortableTh field="matchedTasks" label="关联Task数" numeric active={parsedCommitOrder?.field === 'matchedTasks'} desc={parsedCommitOrder?.field === 'matchedTasks' && parsedCommitOrder.desc} onSort={(f) => setCommitOrder(cycle(parsedCommitOrder, f))} />
                  </th>
                </tr>
              </thead>
              <tbody>
                {sortedCommits.map((c) => {
                  const isOpen = expanded.has(c.commit_id)
                  const pct = Math.round((c.silica ?? 0) * 100)
                  const matched = c.matched_tasks || []
                  return (
                    <Fragment key={c.commit_id}>
                      <tr className="border-b border-gray-100/50 dark:border-white/5 hover:bg-apple-blue/5 dark:hover:bg-white/5 transition-colors">
                        <td className="px-3 py-2 align-middle">
                          <button
                            type="button"
                            onClick={() => toggleRow(c.commit_id)}
                            aria-label={isOpen ? '收起' : '展开'}
                            aria-expanded={isOpen}
                            className="text-gray-400 hover:text-apple-blue cursor-pointer bg-transparent border-none p-0 transition-transform focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue rounded"
                          >
                            <svg className={`w-4 h-4 transition-transform ${isOpen ? 'rotate-90' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
                              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                            </svg>
                          </button>
                        </td>
                        <td className={TD}>
                          <div className="font-mono max-w-[180px] truncate" title={c.commit_id}>{c.commit_id}</div>
                        </td>
                        <td className={TD}>{c.git_user_name || '-'}</td>
                        <td className={TD}>{formatLocalTime(c.commit_time)}</td>
                        <td className={TD_NUM}>{c.diff_lines ?? 0}</td>
                        <td className={TD}>
                          <div className="flex items-center gap-2 min-w-[120px]">
                            <div className="flex-1 h-1.5 rounded-full bg-gray-200 dark:bg-white/10 overflow-hidden">
                              <div className={`h-full rounded-full ${silicaBarColor(c.silica ?? 0)}`} style={{ width: `${pct}%` }} />
                            </div>
                            <span className="text-xs tabular-nums text-gray-500 dark:text-gray-400 w-10 text-right">{formatV2Ratio(c.silica, 0)}</span>
                          </div>
                        </td>
                        <td className={TD_NUM}>{matched.length || '-'}</td>
                      </tr>
                      {isOpen && (
                        <tr className="bg-gray-50/60 dark:bg-white/[0.02]">
                          <td colSpan={7} className="px-5 py-3">
                            {c.silica_reason && (
                              <p className="text-xs text-gray-500 dark:text-gray-400 mb-2">{c.silica_reason}</p>
                            )}
                            {matched.length === 0 ? (
                              <div className="text-xs text-gray-400 dark:text-gray-500">暂无关联 Task</div>
                            ) : (
                              <div className="space-y-1.5">
                                {matched.map((mt) => (
                                  <div key={mt.task_id} className="flex items-center gap-3 text-xs">
                                    <button
                                      type="button"
                                      className="font-mono text-apple-blue hover:text-apple-blue-hover cursor-pointer bg-transparent border-none p-0 focus:outline-none focus-visible:underline"
                                      onClick={() => navigate(`/task/${encodeURIComponent(mt.task_id)}`)}
                                    >
                                      {mt.task_id}
                                    </button>
                                    <span className="text-gray-500 dark:text-gray-400">{mt.user_name || mt.user_id || '-'}</span>
                                    <span className="tabular-nums text-gray-500 dark:text-gray-400">{formatV2Ratio(mt.silica, 0)}</span>
                                  </div>
                                ))}
                              </div>
                            )}
                          </td>
                        </tr>
                      )}
                    </Fragment>
                  )
                })}
              </tbody>
            </table>
          </div>
        </section>
      )}

      {/* ④ 参与者列表（tasks 为空时 participants 为空，不渲染该卡） */}
      {participants.length > 0 && (
        <section className="glass rounded-2xl overflow-hidden">
          <div className="flex items-center justify-between px-5 py-3 border-b border-gray-200/50 dark:border-white/10">
            <span className="text-sm font-semibold text-gray-700 dark:text-gray-200">参与者</span>
            <span className="text-xs text-gray-400 dark:text-gray-500">{participants.length} 人</span>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full text-sm border-collapse">
              <thead>
                <tr className="border-b border-gray-200/50 dark:border-white/10">
                  <th className={TH}>用户名</th>
                  <th className={TH_NUM}>
                    <SortableTh field="taskCount" label="Task数" numeric active={parsedPartOrder?.field === 'taskCount'} desc={parsedPartOrder?.field === 'taskCount' && parsedPartOrder.desc} onSort={(f) => setPartOrder(cycle(parsedPartOrder, f))} />
                  </th>
                  <th className={TH_NUM}>
                    <SortableTh field="commitCount" label="Commit数" numeric active={parsedPartOrder?.field === 'commitCount'} desc={parsedPartOrder?.field === 'commitCount' && parsedPartOrder.desc} onSort={(f) => setPartOrder(cycle(parsedPartOrder, f))} />
                  </th>
                </tr>
              </thead>
              <tbody>
                {sortedParticipants.map((p) => (
                  <tr key={p.user_id} className="border-b border-gray-100/50 dark:border-white/5 hover:bg-apple-blue/5 dark:hover:bg-white/5 transition-colors">
                    <td className={TD}>
                      <button
                        type="button"
                        className="text-apple-blue hover:text-apple-blue-hover cursor-pointer bg-transparent border-none p-0 focus:outline-none focus-visible:underline"
                        onClick={() => navigate(`/user/${encodeURIComponent(p.user_id)}`)}
                      >
                        {p.user_name}
                      </button>
                    </td>
                    <td className={TD_NUM}>{p.task_count}</td>
                    <td className={TD_NUM}>{p.commit_count}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      )}

      {/* ⑤ AI 代码占比图表：silica_entries 后端不返回 → 不渲染（照搬 Vue 现状） */}
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
