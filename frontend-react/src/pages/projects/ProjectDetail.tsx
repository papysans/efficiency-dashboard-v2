// 项目详情页（ProjectDetailV2 的 React + 玻璃拟态迁移）—— PR4b 最复杂页。
// 分区 + 8 管理操作 1:1 按 research/pr4-project-commit-workdir.md §2.2；视觉换玻璃拟态。
//
// ⚠️ 口径：
//   - userStats 行 task/commit_efficiency_ratio = **百分比口径** PercentPill（前端 reduce 算出 ancient/real*100）。
//   - ProjectCommit 表 silica 展示为 AI 代码占比，直接当百分比（不 ×100，阈值 80/50）。
//   - 顶部 devEfficiencyRatio/e2eEfficiencyRatio 是**内部小数**（如 3.0），显示 Math.round(*100)+'%'，
//     着色 percentTextClass(*100)。这是 PR4 唯一「内部小数、显示×100」例外，仍走 300/150 阈值。
//
// 8 管理操作：编辑 / 删除 / 人工调整 / 加 Task / 移 Task / 改 Task AI 代码权重 / 加 Repo（含编辑=删旧+加新）/ 移 Repo。
// ⚠️ updateProject 必须回传 repos/task_ids/task_ids_silica 原值，否则后端清空。
// ⚠️ 加 Repo 用数组 index，编辑/删后 index 漂移 → 操作后必 loadData（invalidate）。
import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { useNavigate, useParams } from 'react-router'
import { useQueryClient } from '@tanstack/react-query'
import {
  addRepoToProject,
  addTasksToProject,
  deleteProject,
  getReposV2,
  getTasksV2,
  removeRepoFromProject,
  removeTasksFromProject,
  updateProject,
  updateProjectManual,
  updateTaskSilicaInProject,
} from '@/api/endpoints'
import { useGlobalConfig, useProjectDetail } from '@/api/queries'
import type {
  AddRepoRequest,
  ProjectCommitItem,
  ProjectModel,
  ProjectRepo,
  ProjectTaskItem,
  RepoListItem,
  TaskListItem,
} from '@/api/types'
import { fmtCost, formatDuration, formatLocalTime } from '@/lib/formatters'
import { useUserNameMap } from '@/hooks/useUserNameMap'
import { Tag } from '@/components/ui/Tag'
import { PercentPill, percentTextClass } from '@/components/ui/PercentPill'
import { Modal } from '@/components/ui/Modal'

// ---- 派生工具 ----
function effAncient(r: ProjectModel): number | null {
  return r.project_ancient_minutes_manual ?? r.project_ancient_minutes ?? null
}
function effProcess(r: ProjectModel): number | null {
  return r.project_real_process_minutes_manual ?? r.project_real_process_minutes ?? null
}
function effLead(r: ProjectModel): number | null {
  return r.project_real_lead_minutes_manual ?? r.project_real_lead_minutes ?? null
}
function isZeroTime(s: string | null | undefined): boolean {
  return !s || String(s).startsWith('0001-')
}
function taskEff(t: ProjectTaskItem): number | null {
  return t.task_ancient_minutes_manual ?? t.task_ancient_minutes ?? null
}
function taskReal(t: ProjectTaskItem): number | null {
  return t.task_real_minutes_manual ?? t.task_real_minutes ?? null
}
function commitEffMin(c: ProjectCommitItem): number | null {
  return c.commit_ancient_minutes_manual ?? c.commit_ancient_minutes ?? null
}
function commitRealMin(c: ProjectCommitItem): number | null {
  return c.commit_real_minutes_manual ?? c.commit_real_minutes ?? null
}

interface UserStat {
  user_id: string
  task_count: number
  commit_count: number
  commit_diff_lines: number
  task_ancient_minutes: number
  task_real_minutes: number
  commit_ancient_minutes: number
  commit_real_minutes: number
  cost: number
  task_efficiency_ratio: number
  commit_efficiency_ratio: number
}

export default function ProjectDetail() {
  const { projectId } = useParams<{ projectId: string }>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { data, isLoading, error } = useProjectDetail(projectId)
  const { data: globalConfig } = useGlobalConfig()
  // task/commit 的 user_name 多为 UUID，用 commits 的 git_user_name 解析真实名。
  const { resolveName } = useUserNameMap()

  const project = data?.project
  const tasks = useMemo<ProjectTaskItem[]>(() => data?.tasks || [], [data])
  const commits = useMemo<ProjectCommitItem[]>(() => data?.commits || [], [data])
  const userCount = data?.user_count ?? 0
  const repos = useMemo<ProjectRepo[]>(() => project?.repos || [], [project])

  const reload = useCallback(async () => {
    await queryClient.invalidateQueries({ queryKey: ['project-detail', projectId] })
  }, [queryClient, projectId])

  // ---- 前端自算派生指标（§2.2）----
  const totalTokens = (project?.upstream_tokens || 0) + (project?.downstream_tokens || 0)
  const totalCodeLines = useMemo(() => commits.reduce((s, r) => s + (r.diff_lines || 0), 0), [commits])
  const actualWorkDays = useMemo(() => {
    const m = project ? effProcess(project) : null
    return m != null && m > 0 ? m / 480 : null
  }, [project])
  const ancientWorkDays = useMemo(() => {
    const m = project ? effAncient(project) : null
    return m != null && m > 0 ? m / 480 : null
  }, [project])
  const leadWorkDays = useMemo(() => {
    const m = project ? effLead(project) : null
    return m != null && m > 0 ? m / 480 : null
  }, [project])
  const actualLinesPerDay = actualWorkDays && actualWorkDays > 0 ? totalCodeLines / actualWorkDays : null
  const traditionalLinesPerDay = ancientWorkDays && ancientWorkDays > 0 ? totalCodeLines / ancientWorkDays : null
  const traditionalDevLinesPerDay = globalConfig?.traditional_dev_lines_per_day || 100
  // 内部小数（显示 ×100）
  const devEfficiencyRatio = ancientWorkDays && actualWorkDays && actualWorkDays > 0 ? ancientWorkDays / actualWorkDays : null
  const e2eEfficiencyRatio = ancientWorkDays && leadWorkDays && leadWorkDays > 0 ? ancientWorkDays / leadWorkDays : null

  // ---- userStats（用户视角聚合）----
  const userStats = useMemo<UserStat[]>(() => {
    const map = new Map<string, UserStat>()
    const ensure = (userId: string): UserStat => {
      let s = map.get(userId)
      if (!s) {
        s = {
          user_id: userId,
          task_count: 0,
          commit_count: 0,
          commit_diff_lines: 0,
          task_ancient_minutes: 0,
          task_real_minutes: 0,
          commit_ancient_minutes: 0,
          commit_real_minutes: 0,
          cost: 0,
          task_efficiency_ratio: 0,
          commit_efficiency_ratio: 0,
        }
        map.set(userId, s)
      }
      return s
    }
    // 分组键改为 user_id（user_name 多为 UUID，不宜当分组 key/显示）；空 → '未知'。显示走 resolveName。
    for (const t of tasks) {
      const s = ensure(t.user_id || '未知')
      s.task_count += 1
      s.task_ancient_minutes += taskEff(t) || 0
      s.task_real_minutes += taskReal(t) || 0
      s.cost += t.cost || 0
    }
    for (const c of commits) {
      const s = ensure(c.user_id || '未知')
      s.commit_count += 1
      s.commit_diff_lines += c.diff_lines || 0
      s.commit_ancient_minutes += commitEffMin(c) || 0
      s.commit_real_minutes += commitRealMin(c) || 0
      s.cost += c.cost || 0
    }
    const rows = Array.from(map.values())
    for (const s of rows) {
      s.task_efficiency_ratio = s.task_real_minutes > 0 ? (s.task_ancient_minutes / s.task_real_minutes) * 100 : 0
      s.commit_efficiency_ratio = s.commit_real_minutes > 0 ? (s.commit_ancient_minutes / s.commit_real_minutes) * 100 : 0
    }
    rows.sort((a, b) => b.commit_diff_lines - a.commit_diff_lines)
    return rows
  }, [tasks, commits])

  // ---- dialog 状态 ----
  const [editOpen, setEditOpen] = useState(false)
  const [manualOpen, setManualOpen] = useState(false)
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [taskOpen, setTaskOpen] = useState(false)
  const [silicaTask, setSilicaTask] = useState<ProjectTaskItem | null>(null)
  const [removeTask, setRemoveTask] = useState<ProjectTaskItem | null>(null)
  const [repoEdit, setRepoEdit] = useState<{ index: number; repo: ProjectRepo } | null>(null)
  const [repoAddOpen, setRepoAddOpen] = useState(false)
  const [removeRepo, setRemoveRepo] = useState<{ index: number; repo: ProjectRepo } | null>(null)

  async function confirmDelete() {
    setDeleting(true)
    try {
      await deleteProject(projectId as string)
      await queryClient.invalidateQueries({ queryKey: ['project-list'] })
      navigate('/project-v2')
    } finally {
      setDeleting(false)
    }
  }

  async function doRemoveTask() {
    if (!removeTask) return
    await removeTasksFromProject(projectId as string, { task_ids: [removeTask.task_id] })
    setRemoveTask(null)
    await reload()
  }

  async function doRemoveRepo() {
    if (!removeRepo) return
    await removeRepoFromProject(projectId as string, removeRepo.index)
    setRemoveRepo(null)
    await reload()
  }

  if (error) {
    return (
      <div className="glass rounded-2xl p-8 text-center text-sm text-rose-600 dark:text-rose-400">
        {(error as Error).message || '获取项目详情失败'}
      </div>
    )
  }

  return (
    <div className={`space-y-5 ${isLoading ? 'opacity-60 pointer-events-none' : ''}`}>
      {/* ① 标题栏 */}
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          <BackButton onClick={() => navigate(-1)} />
          <div>
            <h1 className="text-xl font-bold text-gray-900 dark:text-white">{project?.name || '项目详情'}</h1>
            {project?.description && <p className="text-sm text-gray-500 dark:text-gray-400">{project.description}</p>}
          </div>
        </div>
        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={() => setManualOpen(true)}
            className="inline-flex items-center gap-1.5 bg-amber-500 hover:bg-amber-600 text-white rounded-lg px-3 py-1.5 text-sm font-medium cursor-pointer transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-amber-400"
          >
            人工调整
          </button>
          <button
            type="button"
            onClick={() => setEditOpen(true)}
            className="inline-flex items-center gap-1.5 bg-apple-blue hover:bg-apple-blue-hover text-white rounded-lg px-3 py-1.5 text-sm font-medium cursor-pointer transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue"
          >
            编辑
          </button>
          <button
            type="button"
            onClick={() => setDeleteOpen(true)}
            className="inline-flex items-center gap-1.5 glass text-rose-500 rounded-lg px-3 py-1.5 text-sm font-medium cursor-pointer transition-colors hover:text-rose-600 focus:outline-none focus-visible:ring-2 focus-visible:ring-rose-400"
          >
            删除
          </button>
        </div>
      </header>

      {/* ② 基础信息 */}
      <Panel title="基础信息">
        <KvGrid>
          <Kv label="项目ID" mono>{project?.project_id || '-'}</Kv>
          <Kv label="起始时间">
            <ManualTime manual={project?.start_time_manual} original={project?.start_time} />
          </Kv>
          <Kv label="结束时间">
            {isZeroTime(project?.end_time_manual ?? project?.end_time) ? (
              <span className="text-emerald-600 dark:text-emerald-400">尚未结束</span>
            ) : (
              <ManualTime manual={project?.end_time_manual} original={project?.end_time} />
            )}
          </Kv>
          <Kv label="Repo数">{repos.length}</Kv>
          <Kv label="Task数">{tasks.length}</Kv>
          <Kv label="Commit数">{commits.length}</Kv>
          <Kv label="参与人数">{userCount}</Kv>
        </KvGrid>
      </Panel>

      {/* ③ 度量信息 */}
      <Panel title="度量信息">
        <KvGrid>
          <Kv label="传统开发预估" title={`汇聚项目内所有 Task 和 Commit 的传统开发预估时间之和${project?.project_ancient_minutes_reason ? `：${project.project_ancient_minutes_reason}` : ''}`}>
            {formatDuration(project ? effAncient(project) : null)}
          </Kv>
          <Kv label="实际处理耗时" title={`项目内实际 AI 处理耗时之和（不含等待时间）${project?.project_real_process_minutes_reason ? `：${project.project_real_process_minutes_reason}` : ''}`}>
            {formatDuration(project ? effProcess(project) : null)}
          </Kv>
          <Kv label="项目周期" title={project?.project_real_lead_minutes_reason || undefined}>
            {formatDuration(project ? effLead(project) : null)}
          </Kv>
          <Kv label="总Tokens" title={`上行 ${project?.upstream_tokens || 0} / 下行 ${project?.downstream_tokens || 0}`}>
            {totalTokens > 0 ? totalTokens.toLocaleString() : '-'}
          </Kv>
          <Kv label="总费用">{project?.cost != null && project.cost > 0 ? `${fmtCost(project.cost)} 元` : '-'}</Kv>
          <Kv label="生成代码量" title="所有 Commit diff_lines 之和">
            {totalCodeLines > 0 ? `${totalCodeLines.toLocaleString()} 行` : '-'}
          </Kv>
          <Kv label="实际耗时">{actualWorkDays != null ? `${actualWorkDays.toFixed(2)} 人天` : '-'}</Kv>
          <Kv label="实际人天代码量">{actualLinesPerDay != null ? `${actualLinesPerDay.toFixed(0)} 行/人天` : '-'}</Kv>
          <Kv label="传统开发人天代码量" title={`传统开发基准：${traditionalDevLinesPerDay} 行/人天`}>
            {traditionalLinesPerDay != null ? `${traditionalLinesPerDay.toFixed(0)} 行/人天` : '-'}
          </Kv>
          <Kv label="开发提效比" title="传统开发预估 ÷ 实际耗时">
            <span className={`text-xl font-bold tabular-nums ${percentTextClass(devEfficiencyRatio != null ? devEfficiencyRatio * 100 : null)}`}>
              {devEfficiencyRatio != null ? `${Math.round(devEfficiencyRatio * 100)}%` : '-'}
            </span>
          </Kv>
          <Kv label="端到端提效比" title="传统开发预估 ÷ 项目周期">
            <span className={`text-xl font-bold tabular-nums ${percentTextClass(e2eEfficiencyRatio != null ? e2eEfficiencyRatio * 100 : null)}`}>
              {e2eEfficiencyRatio != null ? `${Math.round(e2eEfficiencyRatio * 100)}%` : '-'}
            </span>
          </Kv>
        </KvGrid>
      </Panel>

      {/* ④ 用户视角 */}
      <Panel title="用户视角" hint={`${userStats.length} 人`}>
        {userStats.length === 0 ? (
          <Empty>暂无数据</Empty>
        ) : (
          <TableWrap>
            <thead>
              <tr className="border-b border-gray-200/50 dark:border-white/10">
                <th className={TH}>用户</th>
                <th className={TH_NUM}>Task数</th>
                <th className={TH_NUM}>Commit数</th>
                <th className={TH_NUM}>代码行数</th>
                <th className={TH_NUM}>Task传统预估</th>
                <th className={TH_NUM}>Task实际耗时</th>
                <th className={TH_CENTER}>Task提效比</th>
                <th className={TH_NUM}>Commit传统预估</th>
                <th className={TH_NUM}>Commit实际耗时</th>
                <th className={TH_CENTER}>Commit提效比</th>
                <th className={TH_NUM}>费用</th>
              </tr>
            </thead>
            <tbody>
              {userStats.map((s) => (
                <tr key={s.user_id} className="border-b border-gray-100/50 dark:border-white/5 hover:bg-apple-blue/5 dark:hover:bg-white/5 transition-colors">
                  <td className={TD} title={resolveName(s.user_id)}>
                    {s.user_id && s.user_id !== '未知' ? (
                      <LinkBtn onClick={() => navigate(`/user/${encodeURIComponent(s.user_id)}`)}>
                        {resolveName(s.user_id)}
                      </LinkBtn>
                    ) : (
                      resolveName(s.user_id)
                    )}
                  </td>
                  <td className={TD_NUM}>{s.task_count}</td>
                  <td className={TD_NUM}>{s.commit_count}</td>
                  <td className={TD_NUM}>{s.commit_diff_lines.toLocaleString()}</td>
                  <td className={TD_NUM}>{formatDuration(s.task_ancient_minutes)}</td>
                  <td className={TD_NUM}>{formatDuration(s.task_real_minutes)}</td>
                  <td className="px-3 py-2 align-middle text-center">
                    {s.task_efficiency_ratio > 0 ? <PercentPill value={s.task_efficiency_ratio} /> : <Tag tone="info">-</Tag>}
                  </td>
                  <td className={TD_NUM}>{formatDuration(s.commit_ancient_minutes)}</td>
                  <td className={TD_NUM}>{formatDuration(s.commit_real_minutes)}</td>
                  <td className="px-3 py-2 align-middle text-center">
                    {s.commit_efficiency_ratio > 0 ? <PercentPill value={s.commit_efficiency_ratio} /> : <Tag tone="info">-</Tag>}
                  </td>
                  <td className={TD_NUM}>{s.cost > 0 ? fmtCost(s.cost) : '-'}</td>
                </tr>
              ))}
            </tbody>
          </TableWrap>
        )}
      </Panel>

      {/* ⑤ Repos */}
      <Panel
        title={`Repos (${repos.length})`}
        action={
          <PanelAddButton onClick={() => setRepoAddOpen(true)}>添加 Repo</PanelAddButton>
        }
      >
        {repos.length === 0 ? (
          <Empty>暂无 Repo 配置</Empty>
        ) : (
          <TableWrap>
            <thead>
              <tr className="border-b border-gray-200/50 dark:border-white/10">
                <th className={TH}>仓库地址</th>
                <th className={TH}>分支</th>
                <th className={TH}>开始时间</th>
                <th className={TH}>结束时间</th>
                <th className={TH_NUM}>白名单commits</th>
                <th className={TH_NUM}>排除commits</th>
                <th className={TH_CENTER}>操作</th>
              </tr>
            </thead>
            <tbody>
              {repos.map((r, i) => (
                <tr key={`${r.repo_addr}#${r.repo_branch}#${i}`} className="border-b border-gray-100/50 dark:border-white/5 hover:bg-apple-blue/5 dark:hover:bg-white/5 transition-colors">
                  <td className={TD}>
                    <LinkBtn onClick={() => navigate(`/repo/${encodeURIComponent(r.repo_addr)}${r.repo_branch ? `/${encodeURIComponent(r.repo_branch)}` : ''}`)}>
                      {r.repo_addr}
                    </LinkBtn>
                  </td>
                  <td className={TD}>{r.repo_branch || '-'}</td>
                  <td className={TD}>{isZeroTime(r.start_time) ? '-' : formatLocalTime(r.start_time)}</td>
                  <td className={TD}>{isZeroTime(r.end_time) ? '-' : formatLocalTime(r.end_time)}</td>
                  <td className={TD_NUM}>{r.include_only_commits?.length || 0}</td>
                  <td className={TD_NUM}>{r.exclude_commits?.length || 0}</td>
                  <td className="px-3 py-2 align-middle text-center whitespace-nowrap">
                    <button type="button" onClick={() => setRepoEdit({ index: i, repo: r })} className="text-apple-blue hover:text-apple-blue-hover cursor-pointer bg-transparent border-none p-0 text-sm mr-3 focus:outline-none focus-visible:underline">编辑</button>
                    <button type="button" onClick={() => setRemoveRepo({ index: i, repo: r })} className="text-rose-500 hover:text-rose-600 cursor-pointer bg-transparent border-none p-0 text-sm focus:outline-none focus-visible:underline">删除</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </TableWrap>
        )}
      </Panel>

      {/* ⑥ Tasks */}
      <Panel
        title={`Tasks (${tasks.length})`}
        action={<PanelAddButton onClick={() => setTaskOpen(true)}>添加 Task</PanelAddButton>}
      >
        {tasks.length === 0 ? (
          <Empty>暂无数据</Empty>
        ) : (
          <TableWrap>
            <thead>
              <tr className="border-b border-gray-200/50 dark:border-white/10">
                <th className={TH}>Task ID</th>
                <th className={TH}>用户</th>
                <th className={TH}>开始时间</th>
                <th className={TH_NUM}>传统预估</th>
                <th className={TH_NUM}>实际耗时</th>
                <th className={TH_NUM}>AI 代码权重</th>
                <th className={TH_NUM}>费用</th>
                <th className={TH_CENTER}>操作</th>
              </tr>
            </thead>
            <tbody>
              {tasks.map((t) => (
                <tr
                  key={t.task_id}
                  onClick={() => navigate(`/task/${encodeURIComponent(t.task_id)}`)}
                  className="border-b border-gray-100/50 dark:border-white/5 cursor-pointer hover:bg-apple-blue/5 dark:hover:bg-white/5 transition-colors"
                >
                  <td className={TD}>
                    <span className="font-mono text-apple-blue">{t.task_id.substring(0, 8)}</span>
                  </td>
                  <td className={TD}>{resolveName(t.user_id)}</td>
                  <td className={TD}>{formatLocalTime(t.start_time)}</td>
                  <td className={TD_NUM}>{formatDuration(taskEff(t))}</td>
                  <td className={TD_NUM}>{formatDuration(taskReal(t))}</td>
                  <td className={TD_NUM}>{(t.silica ?? 1.0).toFixed(2)}</td>
                  <td className={TD_NUM}>{t.cost != null && t.cost > 0 ? fmtCost(t.cost) : '-'}</td>
                  <td className="px-3 py-2 align-middle text-center whitespace-nowrap">
                    <button
                      type="button"
                      onClick={(e) => { e.stopPropagation(); setSilicaTask(t) }}
                      className="text-apple-blue hover:text-apple-blue-hover cursor-pointer bg-transparent border-none p-0 text-sm mr-3 focus:outline-none focus-visible:underline"
                    >
                      编辑
                    </button>
                    <button
                      type="button"
                      onClick={(e) => { e.stopPropagation(); setRemoveTask(t) }}
                      className="text-rose-500 hover:text-rose-600 cursor-pointer bg-transparent border-none p-0 text-sm focus:outline-none focus-visible:underline"
                    >
                      删除
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </TableWrap>
        )}
      </Panel>

      {/* ⑦ Commits */}
      <Panel title={`Commits (${commits.length})`}>
        {commits.length === 0 ? (
          <Empty>暂无数据</Empty>
        ) : (
          <TableWrap>
            <thead>
              <tr className="border-b border-gray-200/50 dark:border-white/10">
                <th className={TH}>Commit ID</th>
                <th className={TH}>用户</th>
                <th className={TH}>时间</th>
                <th className={TH}>说明</th>
                <th className={TH_NUM}>代码行数</th>
                <th className={TH_NUM}>传统预估</th>
                <th className={TH_NUM}>实际耗时</th>
                <th className={TH_CENTER}>AI 代码占比</th>
              </tr>
            </thead>
            <tbody>
              {commits.map((c) => (
                <tr
                  key={c.commit_id}
                  onClick={() => navigate(`/commit/${encodeURIComponent(c.commit_id)}`)}
                  className="border-b border-gray-100/50 dark:border-white/5 cursor-pointer hover:bg-apple-blue/5 dark:hover:bg-white/5 transition-colors"
                >
                  <td className={TD}><span className="font-mono text-apple-blue">{c.commit_id.substring(0, 8)}</span></td>
                  <td className={TD}>{commitUserName(c, resolveName)}</td>
                  <td className={TD}>{formatLocalTime(c.commit_time)}</td>
                  <td className={TD}><div className="max-w-[260px] truncate" title={c.comment}>{c.comment || '-'}</div></td>
                  <td className={TD_NUM}>{c.diff_lines ?? '-'}</td>
                  <td className={TD_NUM}>{formatDuration(commitEffMin(c))}</td>
                  <td className={TD_NUM}>{formatDuration(commitRealMin(c))}</td>
                  <td className="px-3 py-2 align-middle text-center">
                    {c.silica != null ? <Tag tone={projectSilicaTone(c.silica)}>{c.silica.toFixed(1)}%</Tag> : '-'}
                  </td>
                </tr>
              ))}
            </tbody>
          </TableWrap>
        )}
      </Panel>

      {/* ---- Dialogs ---- */}
      {project && (
        <>
          <EditModal open={editOpen} project={project} repos={repos} onClose={() => setEditOpen(false)} onSaved={async () => { setEditOpen(false); await reload() }} projectId={projectId as string} />
          <ManualModal open={manualOpen} project={project} onClose={() => setManualOpen(false)} onSaved={async () => { setManualOpen(false); await reload() }} projectId={projectId as string} />
          <AddTaskModal open={taskOpen} onClose={() => setTaskOpen(false)} onSaved={async () => { setTaskOpen(false); await reload() }} projectId={projectId as string} />
          <SilicaModal task={silicaTask} onClose={() => setSilicaTask(null)} onSaved={async () => { setSilicaTask(null); await reload() }} projectId={projectId as string} />
          <RepoModal open={repoAddOpen || !!repoEdit} edit={repoEdit} onClose={() => { setRepoAddOpen(false); setRepoEdit(null) }} onSaved={async () => { setRepoAddOpen(false); setRepoEdit(null); await reload() }} projectId={projectId as string} />
        </>
      )}

      <ConfirmModal
        open={deleteOpen}
        title="确认删除"
        message={`确定要删除项目「${project?.name || ''}」吗？此操作不可撤销。`}
        confirmLabel="删除"
        loading={deleting}
        onClose={() => setDeleteOpen(false)}
        onConfirm={confirmDelete}
      />
      <ConfirmModal
        open={!!removeTask}
        title="移除 Task"
        message="确定要从项目中移除此 Task 吗？"
        confirmLabel="移除"
        onClose={() => setRemoveTask(null)}
        onConfirm={doRemoveTask}
      />
      <ConfirmModal
        open={!!removeRepo}
        title="移除 Repo"
        message={`确定要从项目中移除 Repo「${removeRepo?.repo.repo_addr || ''}」吗？`}
        confirmLabel="移除"
        onClose={() => setRemoveRepo(null)}
        onConfirm={doRemoveRepo}
      />
    </div>
  )
}

/**
 * Commit 行用户名：优先按 user_id 解析真实名（resolveName）；
 * 未命中（无 user_id 或映射缺失，resolveName 回退原 user_id）时退回 git_user_name（本就是真实名）。
 */
function commitUserName(c: ProjectCommitItem, resolveName: (id?: string) => string): string {
  if (c.user_id) {
    const resolved = resolveName(c.user_id)
    if (resolved !== c.user_id) return resolved
  }
  return c.git_user_name || c.user_name || c.user_id || '-'
}

/** ProjectCommit AI 代码占比 tag tone（直接当百分比，不 ×100，阈值 80/50）。 */
function projectSilicaTone(v: number): 'success' | 'primary' | 'info' {
  if (v >= 80) return 'success'
  if (v >= 50) return 'primary'
  return 'info'
}

// ============ 编辑项目 dialog ============
function EditModal({
  open,
  project,
  repos,
  projectId,
  onClose,
  onSaved,
}: {
  open: boolean
  project: ProjectModel
  repos: ProjectRepo[]
  projectId: string
  onClose: () => void
  onSaved: () => Promise<void>
}) {
  const [name, setName] = useState('')
  const [desc, setDesc] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [err, setErr] = useState('')

  useEffect(() => {
    if (!open) return
    setName(project.name || '')
    setDesc(project.description || '')
    setErr('')
  }, [open, project])

  async function handleSubmit() {
    if (!name.trim()) {
      setErr('请输入项目名称')
      return
    }
    setSubmitting(true)
    setErr('')
    try {
      // ⚠️ 必须回传 repos/task_ids/task_ids_silica 原值，否则后端清空。
      await updateProject(projectId, {
        name: name.trim(),
        description: (desc || '').trim(),
        repos,
        task_ids: project.task_ids || [],
        task_ids_silica: project.task_ids_silica || [],
      })
      await onSaved()
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : '保存失败')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <FormModal open={open} title="编辑项目" maxWidth={500} submitting={submitting} onClose={onClose} onSubmit={handleSubmit}>
      {err && <div className="text-sm text-rose-600 dark:text-rose-400">{err}</div>}
      <Field label="项目名称">
        <input type="text" value={name} onChange={(e) => setName(e.target.value)} className={INPUT} />
      </Field>
      <Field label="描述">
        <textarea rows={3} value={desc} onChange={(e) => setDesc(e.target.value)} className={`${INPUT} resize-y`} />
      </Field>
    </FormModal>
  )
}

// ============ 人工调整 dialog ============
function ManualModal({
  open,
  project,
  projectId,
  onClose,
  onSaved,
}: {
  open: boolean
  project: ProjectModel
  projectId: string
  onClose: () => void
  onSaved: () => Promise<void>
}) {
  const [ancient, setAncient] = useState('')
  const [ancientReason, setAncientReason] = useState('')
  const [process, setProcess] = useState('')
  const [processReason, setProcessReason] = useState('')
  const [lead, setLead] = useState('')
  const [leadReason, setLeadReason] = useState('')
  const [startTime, setStartTime] = useState('')
  const [endTime, setEndTime] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [err, setErr] = useState('')

  useEffect(() => {
    if (!open) return
    const num = (m: number | null | undefined) => (m == null ? '' : String(m))
    setAncient(num(project.project_ancient_minutes_manual ?? project.project_ancient_minutes))
    setAncientReason(project.project_ancient_minutes_reason_manual || '')
    setProcess(num(project.project_real_process_minutes_manual ?? project.project_real_process_minutes))
    setProcessReason(project.project_real_process_minutes_reason_manual || '')
    setLead(num(project.project_real_lead_minutes_manual ?? project.project_real_lead_minutes))
    setLeadReason(project.project_real_lead_minutes_reason_manual || '')
    setStartTime(toLocalInput(project.start_time_manual))
    setEndTime(toLocalInput(project.end_time_manual))
    setErr('')
  }, [open, project])

  async function handleSubmit() {
    setSubmitting(true)
    setErr('')
    try {
      await updateProjectManual(projectId, {
        project_ancient_minutes_manual: ancient === '' ? null : Number(ancient),
        project_ancient_minutes_reason_manual: ancientReason,
        project_real_process_minutes_manual: process === '' ? null : Number(process),
        project_real_process_minutes_reason_manual: processReason,
        project_real_lead_minutes_manual: lead === '' ? null : Number(lead),
        project_real_lead_minutes_reason_manual: leadReason,
        start_time_manual: startTime === '' ? null : new Date(startTime).toISOString(),
        end_time_manual: endTime === '' ? null : new Date(endTime).toISOString(),
      })
      await onSaved()
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : '保存失败')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <FormModal open={open} title="人工调整" maxWidth={650} submitting={submitting} onClose={onClose} onSubmit={handleSubmit}>
      {err && <div className="text-sm text-rose-600 dark:text-rose-400">{err}</div>}
      <Field label="传统开发预估（分钟）">
        <input type="number" step={10} value={ancient} onChange={(e) => setAncient(e.target.value)} className={INPUT} />
      </Field>
      <Field label="传统开发预估理由">
        <textarea rows={2} value={ancientReason} onChange={(e) => setAncientReason(e.target.value)} className={`${INPUT} resize-y`} />
      </Field>
      <Field label="实际处理耗时（分钟）">
        <input type="number" step={10} value={process} onChange={(e) => setProcess(e.target.value)} className={INPUT} />
      </Field>
      <Field label="实际处理耗时理由">
        <textarea rows={2} value={processReason} onChange={(e) => setProcessReason(e.target.value)} className={`${INPUT} resize-y`} />
      </Field>
      <Field label="项目周期（分钟）">
        <input type="number" step={10} value={lead} onChange={(e) => setLead(e.target.value)} className={INPUT} />
      </Field>
      <Field label="项目周期理由">
        <textarea rows={2} value={leadReason} onChange={(e) => setLeadReason(e.target.value)} className={`${INPUT} resize-y`} />
      </Field>
      <div className="grid grid-cols-2 gap-3">
        <Field label="开始时间">
          <input type="datetime-local" value={startTime} onChange={(e) => setStartTime(e.target.value)} className={INPUT} />
        </Field>
        <Field label="结束时间">
          <input type="datetime-local" value={endTime} onChange={(e) => setEndTime(e.target.value)} className={INPUT} />
        </Field>
      </div>
    </FormModal>
  )
}

// ============ 添加 Task dialog ============
function AddTaskModal({
  open,
  projectId,
  onClose,
  onSaved,
}: {
  open: boolean
  projectId: string
  onClose: () => void
  onSaved: () => Promise<void>
}) {
  const [keyword, setKeyword] = useState('')
  const [options, setOptions] = useState<TaskListItem[]>([])
  const [selected, setSelected] = useState<TaskListItem[]>([])
  const [silica, setSilica] = useState('1.0')
  const [searching, setSearching] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [err, setErr] = useState('')

  useEffect(() => {
    if (!open) return
    setKeyword('')
    setOptions([])
    setSelected([])
    setSilica('1.0')
    setErr('')
  }, [open])

  async function search() {
    setSearching(true)
    try {
      const today = new Date()
      const end = `${today.getFullYear()}${String(today.getMonth() + 1).padStart(2, '0')}${String(today.getDate()).padStart(2, '0')}`
      const res = await getTasksV2({ pageSize: 50, startDate: '20250101', endDate: end })
      const kw = keyword.trim().toLowerCase()
      const rows = (res.data || []).filter((t) =>
        !kw ||
        t.task_id.toLowerCase().includes(kw) ||
        (t.user_name || '').toLowerCase().includes(kw) ||
        (t.work_dir || '').toLowerCase().includes(kw) ||
        (t.title || '').toLowerCase().includes(kw),
      )
      setOptions(rows)
    } finally {
      setSearching(false)
    }
  }

  function toggle(t: TaskListItem) {
    setSelected((prev) => (prev.some((x) => x.task_id === t.task_id) ? prev.filter((x) => x.task_id !== t.task_id) : [...prev, t]))
  }

  async function handleSubmit() {
    if (selected.length === 0) {
      setErr('请至少选择 1 个 Task')
      return
    }
    setSubmitting(true)
    setErr('')
    try {
      const w = Number(silica)
      await addTasksToProject(projectId, {
        task_ids: selected.map((t) => t.task_id),
        task_ids_silica: selected.map(() => (Number.isFinite(w) ? w : 1.0)),
      })
      await onSaved()
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : '添加失败')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <FormModal open={open} title="添加 Task" maxWidth={560} submitting={submitting} onClose={onClose} onSubmit={handleSubmit}>
      {err && <div className="text-sm text-rose-600 dark:text-rose-400">{err}</div>}
      <Field label="搜索 Task（ID / 用户 / 工作目录 / 标题）">
        <div className="flex gap-2">
          <input
            type="text"
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); search() } }}
            className={INPUT}
            placeholder="输入关键词后点搜索"
          />
          <button type="button" onClick={search} disabled={searching} className="shrink-0 bg-apple-blue hover:bg-apple-blue-hover text-white rounded-lg px-3 py-1.5 text-sm font-medium cursor-pointer transition-colors disabled:opacity-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue">
            {searching ? '搜索中...' : '搜索'}
          </button>
        </div>
      </Field>
      {options.length > 0 && (
        <div className="max-h-56 overflow-y-auto glass rounded-lg p-1">
          {options.map((t) => {
            const checked = selected.some((x) => x.task_id === t.task_id)
            return (
              <label key={t.task_id} className="flex items-start gap-2 px-2 py-1.5 rounded-md hover:bg-apple-blue/5 dark:hover:bg-white/5 cursor-pointer">
                <input type="checkbox" checked={checked} onChange={() => toggle(t)} className="mt-1 accent-apple-blue cursor-pointer" />
                <div className="min-w-0">
                  <div className="text-sm text-gray-800 dark:text-gray-100 truncate">
                    <span className="font-mono text-apple-blue">{t.task_id.substring(0, 8)}</span>
                    {t.user_name ? ` · ${t.user_name}` : ''}
                  </div>
                  <div className="text-xs text-gray-400 dark:text-gray-500 truncate">{t.title || t.work_dir || ''}</div>
                </div>
              </label>
            )
          })}
        </div>
      )}
      <Field label="AI 代码权重">
        <input type="number" step={0.1} min={0} value={silica} onChange={(e) => setSilica(e.target.value)} className={INPUT} />
      </Field>
      <div className="text-xs text-gray-400 dark:text-gray-500">已选 {selected.length} 个 Task</div>
    </FormModal>
  )
}

// ============ 改 Task AI 代码权重 dialog ============
function SilicaModal({
  task,
  projectId,
  onClose,
  onSaved,
}: {
  task: ProjectTaskItem | null
  projectId: string
  onClose: () => void
  onSaved: () => Promise<void>
}) {
  const [silica, setSilica] = useState('1.0')
  const [submitting, setSubmitting] = useState(false)
  const [err, setErr] = useState('')

  useEffect(() => {
    if (!task) return
    setSilica(String(task.silica ?? 1.0))
    setErr('')
  }, [task])

  async function handleSubmit() {
    if (!task) return
    setSubmitting(true)
    setErr('')
    try {
      await updateTaskSilicaInProject(projectId, { task_id: task.task_id, silica: Number(silica) })
      await onSaved()
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : '保存失败')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <FormModal open={!!task} title="编辑 AI 代码权重" maxWidth={400} submitting={submitting} onClose={onClose} onSubmit={handleSubmit}>
      {err && <div className="text-sm text-rose-600 dark:text-rose-400">{err}</div>}
      <Field label="AI 代码权重">
        <input type="number" step={0.1} min={0} value={silica} onChange={(e) => setSilica(e.target.value)} className={INPUT} />
      </Field>
    </FormModal>
  )
}

// ============ 添加/编辑 Repo dialog ============
function RepoModal({
  open,
  edit,
  projectId,
  onClose,
  onSaved,
}: {
  open: boolean
  edit: { index: number; repo: ProjectRepo } | null
  projectId: string
  onClose: () => void
  onSaved: () => Promise<void>
}) {
  const [repoAddr, setRepoAddr] = useState('')
  const [repoBranch, setRepoBranch] = useState('')
  const [addrOptions, setAddrOptions] = useState<string[]>([])
  const [endIsNow, setEndIsNow] = useState(false)
  const [startTime, setStartTime] = useState('')
  const [endTime, setEndTime] = useState('')
  const [whitelist, setWhitelist] = useState(false)
  const [includeText, setIncludeText] = useState('')
  const [excludeText, setExcludeText] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [err, setErr] = useState('')

  useEffect(() => {
    if (!open) return
    // 加载 repo 地址选项
    getReposV2({ pageSize: 1000 })
      .then((res) => {
        const addrs = Array.from(new Set((res.data || []).map((r: RepoListItem) => r.repo_addr).filter(Boolean)))
        setAddrOptions(addrs)
      })
      .catch(() => setAddrOptions([]))
    if (edit) {
      const r = edit.repo
      setRepoAddr(r.repo_addr || '')
      setRepoBranch(r.repo_branch || '')
      setEndIsNow(isZeroTime(r.end_time))
      setStartTime(toLocalInput(r.start_time))
      setEndTime(isZeroTime(r.end_time) ? '' : toLocalInput(r.end_time))
      const inc = r.include_only_commits || []
      setWhitelist(inc.length > 0)
      setIncludeText(inc.join('\n'))
      setExcludeText((r.exclude_commits || []).join('\n'))
    } else {
      setRepoAddr('')
      setRepoBranch('')
      setEndIsNow(false)
      setStartTime('')
      setEndTime('')
      setWhitelist(false)
      setIncludeText('')
      setExcludeText('')
    }
    setErr('')
  }, [open, edit])

  async function handleSubmit() {
    if (!repoAddr.trim() || !repoBranch.trim()) {
      setErr('仓库地址和分支必填')
      return
    }
    setSubmitting(true)
    setErr('')
    const splitLines = (s: string) => s.split('\n').map((x) => x.trim()).filter(Boolean)
    const body: AddRepoRequest = {
      repo_addr: repoAddr.trim(),
      repo_branch: repoBranch.trim(),
      start_time: startTime === '' ? null : new Date(startTime).toISOString(),
      end_time: endIsNow || endTime === '' ? null : new Date(endTime).toISOString(),
      include_only_commits: whitelist ? splitLines(includeText) : [],
      exclude_commits: whitelist ? [] : splitLines(excludeText),
    }
    try {
      // ⚠️ 编辑 = 先按 index 删旧 + 再 add 新（index 漂移，操作后必 reload）。
      if (edit) await removeRepoFromProject(projectId, edit.index)
      await addRepoToProject(projectId, body)
      await onSaved()
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : '保存失败')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <FormModal open={open} title={edit ? '编辑 Repo' : '添加 Repo'} maxWidth={560} submitting={submitting} onClose={onClose} onSubmit={handleSubmit}>
      {err && <div className="text-sm text-rose-600 dark:text-rose-400">{err}</div>}
      <Field label="仓库地址">
        <input type="text" list="repo-addr-options" value={repoAddr} onChange={(e) => setRepoAddr(e.target.value)} className={INPUT} placeholder="选择或输入仓库地址" />
        <datalist id="repo-addr-options">
          {addrOptions.map((a) => <option key={a} value={a} />)}
        </datalist>
      </Field>
      <Field label="分支">
        <input type="text" value={repoBranch} onChange={(e) => setRepoBranch(e.target.value)} className={INPUT} placeholder="如 main" />
      </Field>
      <div className="grid grid-cols-2 gap-3">
        <Field label="开始时间">
          <input type="datetime-local" value={startTime} onChange={(e) => setStartTime(e.target.value)} className={INPUT} />
        </Field>
        <Field label="结束时间">
          <input type="datetime-local" value={endTime} onChange={(e) => setEndTime(e.target.value)} disabled={endIsNow} className={`${INPUT} disabled:opacity-50`} />
        </Field>
      </div>
      <label className="inline-flex items-center gap-1.5 text-sm text-gray-600 dark:text-gray-300 cursor-pointer select-none">
        <input type="checkbox" checked={endIsNow} onChange={(e) => setEndIsNow(e.target.checked)} className="accent-apple-blue cursor-pointer" />
        结束时间至今（now）
      </label>
      <label className="inline-flex items-center gap-1.5 text-sm text-gray-600 dark:text-gray-300 cursor-pointer select-none">
        <input type="checkbox" checked={whitelist} onChange={(e) => setWhitelist(e.target.checked)} className="accent-apple-blue cursor-pointer" />
        仅包含指定 Commits（白名单模式）
      </label>
      {whitelist ? (
        <Field label="包含的 Commit IDs（每行一个）">
          <textarea rows={3} value={includeText} onChange={(e) => setIncludeText(e.target.value)} className={`${INPUT} resize-y font-mono`} />
        </Field>
      ) : (
        <Field label="排除的 Commit IDs（每行一个，可选）">
          <textarea rows={3} value={excludeText} onChange={(e) => setExcludeText(e.target.value)} className={`${INPUT} resize-y font-mono`} />
        </Field>
      )}
    </FormModal>
  )
}

// ============ 通用子组件 ============
const INPUT =
  'glass rounded-lg px-3 py-1.5 text-sm w-full bg-transparent text-gray-900 dark:text-white ' +
  'focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue'

const TH = 'px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_NUM = 'px-3 py-2 text-right font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_CENTER = 'px-3 py-2 text-center font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TD = 'px-3 py-2 align-middle text-gray-700 dark:text-gray-200'
const TD_NUM = 'px-3 py-2 align-middle text-right tabular-nums text-gray-700 dark:text-gray-200'

/** ISO/零时间 → datetime-local 输入值（本地）；零/空 → ''。 */
function toLocalInput(iso: string | null | undefined): string {
  if (isZeroTime(iso)) return ''
  const d = new Date(iso as string)
  if (Number.isNaN(d.getTime())) return ''
  const p = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}T${p(d.getHours())}:${p(d.getMinutes())}`
}

function BackButton({ onClick }: { onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="inline-flex items-center gap-1 text-sm text-gray-500 dark:text-gray-400 hover:text-apple-blue cursor-pointer bg-transparent border-none p-0 transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue rounded"
    >
      <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
      </svg>
      返回
    </button>
  )
}

function PanelAddButton({ onClick, children }: { onClick: () => void; children: ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="inline-flex items-center gap-1 text-apple-blue hover:text-apple-blue-hover cursor-pointer bg-transparent border-none p-0 text-sm font-medium focus:outline-none focus-visible:underline"
    >
      <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
      </svg>
      {children}
    </button>
  )
}

function Panel({ title, hint, action, children }: { title: string; hint?: string; action?: ReactNode; children: ReactNode }) {
  return (
    <section className="glass rounded-2xl overflow-hidden">
      <div className="flex items-center justify-between px-5 py-3 border-b border-gray-200/50 dark:border-white/10">
        <div className="flex items-center gap-2">
          <span className="text-sm font-semibold text-gray-700 dark:text-gray-200">{title}</span>
          {hint ? <span className="text-xs text-gray-400 dark:text-gray-500">{hint}</span> : null}
        </div>
        {action}
      </div>
      <div className="p-5">{children}</div>
    </section>
  )
}

function TableWrap({ children }: { children: ReactNode }) {
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm border-collapse">{children}</table>
    </div>
  )
}

function Empty({ children }: { children: ReactNode }) {
  return <div className="py-6 text-center text-sm text-gray-400 dark:text-gray-500">{children}</div>
}

function KvGrid({ children }: { children: ReactNode }) {
  return <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-x-6 gap-y-3">{children}</div>
}

function Kv({ label, children, mono = false, title }: { label: string; children: ReactNode; mono?: boolean; title?: string }) {
  return (
    <div className="flex flex-col gap-0.5">
      <span className="text-xs text-gray-400 dark:text-gray-500" title={title}>
        {label}
        {title && (
          <span className="ml-1 text-gray-300 dark:text-gray-600 cursor-help align-middle" aria-hidden="true">
            <svg className="w-3 h-3 inline" fill="currentColor" viewBox="0 0 24 24">
              <path d="M12 2a10 10 0 100 20 10 10 0 000-20zm0 15a1 1 0 110-2 1 1 0 010 2zm1.07-7.75l-.9.92c-.5.51-.67.95-.67 1.83h-2v-.5c0-.66.27-1.26.67-1.67l1.24-1.26c.37-.36.59-.86.59-1.41a2 2 0 10-4 0H6a4 4 0 118 0c0 .73-.3 1.4-.83 1.99z" />
            </svg>
          </span>
        )}
      </span>
      <span className={`text-sm text-gray-800 dark:text-gray-100 break-words ${mono ? 'font-mono' : ''}`}>{children}</span>
    </div>
  )
}

/** 起止时间 manual 优先 + 删除线原值。 */
function ManualTime({ manual, original }: { manual?: string | null; original?: string | null }) {
  if (!isZeroTime(manual)) {
    return (
      <span className="inline-flex items-center gap-1.5 flex-wrap">
        <span>{formatLocalTime(manual)}</span>
        {!isZeroTime(original) && <span className="line-through text-gray-400 dark:text-gray-500">{formatLocalTime(original)}</span>}
      </span>
    )
  }
  return <span>{isZeroTime(original) ? '-' : formatLocalTime(original)}</span>
}

function LinkBtn({ onClick, children }: { onClick: () => void; children: ReactNode }) {
  return (
    <button
      type="button"
      onClick={(e) => { e.stopPropagation(); onClick() }}
      className="text-apple-blue hover:text-apple-blue-hover cursor-pointer bg-transparent border-none p-0 text-left break-all focus:outline-none focus-visible:underline"
    >
      {children}
    </button>
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

/** 表单 Modal 壳（取消/提交 footer + 内容滚动）。 */
function FormModal({
  open,
  title,
  maxWidth,
  submitting,
  onClose,
  onSubmit,
  children,
}: {
  open: boolean
  title: string
  maxWidth?: number
  submitting: boolean
  onClose: () => void
  onSubmit: () => void
  children: ReactNode
}) {
  return (
    <Modal
      open={open}
      title={title}
      maxWidth={maxWidth}
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
            onClick={onSubmit}
            disabled={submitting}
            className="bg-apple-blue hover:bg-apple-blue-hover text-white rounded-lg px-4 py-1.5 text-sm font-medium cursor-pointer transition-colors disabled:opacity-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue"
          >
            {submitting ? '保存中...' : '保存'}
          </button>
        </>
      }
    >
      <div className="space-y-3">{children}</div>
    </Modal>
  )
}

/** 确认 Modal（删除/移除 通用）。 */
function ConfirmModal({
  open,
  title,
  message,
  confirmLabel,
  loading = false,
  onClose,
  onConfirm,
}: {
  open: boolean
  title: string
  message: string
  confirmLabel: string
  loading?: boolean
  onClose: () => void
  onConfirm: () => void | Promise<void>
}) {
  const [busy, setBusy] = useState(false)
  async function handle() {
    setBusy(true)
    try {
      await onConfirm()
    } finally {
      setBusy(false)
    }
  }
  const disabled = busy || loading
  return (
    <Modal
      open={open}
      title={title}
      maxWidth={420}
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
            onClick={handle}
            disabled={disabled}
            className="bg-rose-500 hover:bg-rose-600 text-white rounded-lg px-4 py-1.5 text-sm font-medium cursor-pointer transition-colors disabled:opacity-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-rose-400"
          >
            {disabled ? '处理中...' : confirmLabel}
          </button>
        </>
      }
    >
      <p className="text-sm text-gray-700 dark:text-gray-200">{message}</p>
    </Modal>
  )
}
