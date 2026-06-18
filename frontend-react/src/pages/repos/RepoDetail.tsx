// 仓库详情页（RepoDetailV2 的 React + 玻璃拟态迁移）。
// 分区/列/口径 1:1 按 research/pr3-user-repo-org.md §Repo-5；⚠️ 百分比口径 → PercentPill（不 ×100）。
// 「添加到 Project」（PR4c §4.2）：选 Project（或新建）→ 两段式冲突检测（checkProjectConflicts，
// 有冲突展示让用户确认）→ addRepoToProject（加 repo filter，可选白名单 commits）。
//
// 提效比计算：commitEffRatio/taskEffRatio = (ancientₘ − realₘ) / realₘ * 100（提升百分比，与后端 CalcEfficiencyRatio 及同页汇总卡一致；manual 优先，ancient/real 都>0 才可算，返回值含 0/负的真实提效；不可算返回 null，渲染/排序按缺失处理）。
// 表格客户端排序（manual 优先 / 计算值），null/0 沉底。
import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router'
import { useQueryClient } from '@tanstack/react-query'
import { useRepoBranches, useRepoDetail } from '@/api/queries'
import { addRepoToProject, checkProjectConflicts, createProject, getProjects } from '@/api/endpoints'
import type { ProjectConflict, ProjectListItem, RepoCommitItem, TaskListItem } from '@/api/types'
import { formatDuration, formatLocalTime, formatNumber, formatV2Ratio } from '@/lib/formatters'
import { getDefaultDateRangeWide } from '@/lib/date'
import { parseOrder, sortRows, toOrder } from '@/lib/sort'
import { PercentPill, percentTextClass } from '@/components/ui/PercentPill'
import { Tag } from '@/components/ui/Tag'
import { SortableTh } from '@/components/ui/SortableTh'
import { DateRangePicker } from '@/components/ui/DateRangePicker'
import { Modal } from '@/components/ui/Modal'

// manual 优先口径（commit/task 提效比 = (ancient−real)/real*100 提升百分比，与后端 CalcEfficiencyRatio 及同页汇总卡一致）。
// ancient/real 都>0 才可算（返回值含 0/负的真实提效，无提升即 0 或负，照常显示）；不可算返回 null → 显示 `-`、排序沉底。
function commitEffRatio(row: RepoCommitItem): number | null {
  const ancient = row.commit_ancient_minutes_manual ?? row.commit_ancient_minutes
  const real = row.commit_real_minutes_manual ?? row.commit_real_minutes
  if (ancient != null && real != null && ancient > 0 && real > 0) return ((ancient - real) / real) * 100
  return null
}
function taskEffRatio(row: TaskListItem): number | null {
  const ancient = row.task_ancient_minutes_manual ?? row.task_ancient_minutes
  const real = row.task_real_minutes_manual ?? row.task_real_minutes
  if (ancient != null && real != null && ancient > 0 && real > 0) return ((ancient - real) / real) * 100
  return null
}
function commitReal(row: RepoCommitItem): number | null | undefined {
  return row.commit_real_minutes_manual ?? row.commit_real_minutes
}
/** 取 commit 所属分支（后端逐条返回 repo_branch；经 RepoCommitItem 索引签名取出，归一为字符串）。 */
function commitBranch(row: RepoCommitItem): string {
  const b = (row as { repo_branch?: unknown }).repo_branch
  return typeof b === 'string' ? b : ''
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

/** Commit 级 AI 代码占比 tag tone（小数口径）。 */
function aiCodeRatioTone(v: number): 'success' | 'primary' | 'info' {
  if (v >= 0.8) return 'success'
  if (v >= 0.5) return 'primary'
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

/**
 * RepoDetail 既是独立路由页（/repo/:repoAddr/:repoBranch?），也可被「仓库·效率」聚焦态壳内嵌
 * （传 repoAddrProp + dateRangeProp + embedded）。嵌入态：日期用全局 timeRange、分支走内部 state 不离开壳。
 */
interface RepoDetailProps {
  repoAddrProp?: string
  dateRangeProp?: [string, string]
  embedded?: boolean
}

export default function RepoDetail({ repoAddrProp, dateRangeProp, embedded = false }: RepoDetailProps = {}) {
  const { repoAddr: repoAddrRaw, repoBranch: repoBranchRaw } = useParams<{ repoAddr: string; repoBranch?: string }>()
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()

  // 路由 param 由 React Router 已 decode；嵌入态用 prop。
  const repoAddr = repoAddrProp ?? repoAddrRaw ?? ''

  // 日期：嵌入态用 prop（全局 timeRange）；独立页取 URL（YYYYMMDD/YYYY-MM-DD）；缺则近 90 天。
  const dateRange = useMemo<[string, string]>(() => {
    if (dateRangeProp) return dateRangeProp
    const start = normalizeDateQuery(searchParams.get('startDate'))
    const end = normalizeDateQuery(searchParams.get('endDate'))
    if (start && end) return [start, end]
    return getDefaultDateRangeWide()
  }, [searchParams, dateRangeProp])

  const params = useMemo(
    () => ({ startDate: dateRange[0].replace(/-/g, ''), endDate: dateRange[1].replace(/-/g, '') }),
    [dateRange],
  )

  const { data: branchesData } = useRepoBranches(repoAddr)
  const branches = branchesData?.branches || []

  // 嵌入态分支用内部 state（不导航离开壳）；独立页分支走 URL param。
  // 默认空串 = 整仓口径（后端 repoBranch 传空 → 不过滤 → 返回该仓库所有分支的 commits）。
  // 不再 fallback branches[0]，否则会强制单分支过滤，看不到其他分支（bug #4 根因）。
  const [embeddedBranch, setEmbeddedBranch] = useState('')
  const currentBranch = embedded ? embeddedBranch : repoBranchRaw || ''

  const { data, isLoading, error } = useRepoDetail({
    repoAddr,
    repoBranch: currentBranch || undefined,
    ...params,
  })

  const commits: RepoCommitItem[] = useMemo(() => data?.commits || [], [data?.commits])
  const tasks: TaskListItem[] = useMemo(() => data?.tasks || [], [data?.tasks])
  const efficiency = data?.efficiency

  // 「添加到 Project」对话框
  const [addOpen, setAddOpen] = useState(false)

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
      efficiencyRatio: (r) => commitEffRatio(r),
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
      efficiencyRatio: (r) => taskEffRatio(r),
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

  // 分支一览（类树/总览）：整仓 commits 客户端按 repo_branch 分组聚合。
  // 提效比口径=守恒（Σ古法 / Σ实际，百分比 (Σ古法−Σ实际)/Σ实际*100，对齐同页汇总卡 / 后端 CalcEfficiencyRatio）。
  // 按 commit 数倒序。空时返回 []，渲染侧据此隐藏卡片。
  const branchSummary = useMemo(() => {
    const map = new Map<string, { branch: string; count: number; diffLines: number; realMin: number; ancientMin: number }>()
    for (const c of commits) {
      const b = commitBranch(c)
      const key = b || '(未标注分支)'
      let row = map.get(key)
      if (!row) {
        row = { branch: b, count: 0, diffLines: 0, realMin: 0, ancientMin: 0 }
        map.set(key, row)
      }
      row.count += 1
      row.diffLines += c.diff_lines || 0
      row.realMin += commitReal(c) || 0
      row.ancientMin += commitAncient(c) || 0
    }
    return Array.from(map.values())
      .map((r) => ({
        ...r,
        // 守恒提效比：仅 Σ实际>0 才可算（与 commit 级口径一致，不可算返回 null）。
        effRatio: r.realMin > 0 ? ((r.ancientMin - r.realMin) / r.realMin) * 100 : null,
      }))
      .sort((a, b) => b.count - a.count)
  }, [commits])

  function handleBranchChange(branch: string) {
    if (embedded) {
      setEmbeddedBranch(branch)
      return
    }
    const q = new URLSearchParams({ startDate: params.startDate, endDate: params.endDate })
    // 空分支 = 整仓口径：导航到不带 branch 段的路由（repoBranch? 可选），避免空段匹配问题。
    const pathname = branch
      ? `/repo/${encodeURIComponent(repoAddr)}/${encodeURIComponent(branch)}`
      : `/repo/${encodeURIComponent(repoAddr)}`
    navigate({ pathname, search: `?${q.toString()}` })
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
          {!embedded && (
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
          )}
          {!embedded && <h1 className="text-2xl font-bold text-gray-900 dark:text-white">仓库详情</h1>}
        </div>
        <div className="flex flex-wrap items-center gap-2">
          {branches.length > 0 && (
            <select
              value={currentBranch}
              onChange={(e) => handleBranchChange(e.target.value)}
              className="glass rounded-lg px-3 py-1.5 text-sm bg-transparent cursor-pointer text-gray-700 dark:text-gray-200 focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue"
              aria-label="切换分支"
            >
              {/* 默认整仓口径（空值=不过滤=所有分支 commits）；其下为各具体分支。 */}
              <option value="">全部分支（整仓）</option>
              {branches.map((b) => (
                <option key={b} value={b}>
                  {b}
                </option>
              ))}
            </select>
          )}
          {!embedded && <DateRangePicker value={dateRange} onChange={onDateChange} />}
          <button
            type="button"
            onClick={() => setAddOpen(true)}
            className="inline-flex items-center gap-1.5 bg-apple-blue hover:bg-apple-blue-hover text-white rounded-lg px-3 py-1.5 text-sm font-medium cursor-pointer transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
            </svg>
            添加到 Project
          </button>
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
          <div>
            <div className="text-gray-500 dark:text-gray-400 mb-1">AI 代码占比</div>
            <div className="text-xl font-bold text-emerald-600 dark:text-emerald-400 tabular-nums">
              {formatV2Ratio(data?.summary?.ai_code_ratio)}
            </div>
          </div>
          <KV label="代码行数" value={`${totalDiffLines.toLocaleString()} 行`} />
          <KV label="总费用（Tasks）" value={totalCost > 0 ? `${totalCost.toFixed(2)} 元` : '-'} />
          <KV label="贡献者" value={`${contributorCount} 人`} />
        </div>
      </section>

      {/* 分支一览（仅整仓态显示）：整仓 commits 按 repo_branch 分组的树/总览，点行下钻到该分支。 */}
      {currentBranch === '' && branchSummary.length > 0 && (
        <section className="glass rounded-2xl overflow-hidden">
          <div className="flex items-center justify-between px-5 py-3 border-b border-gray-200/50 dark:border-white/10">
            <span className="text-sm font-semibold text-gray-700 dark:text-gray-200">分支一览</span>
            <span className="text-xs text-gray-400 dark:text-gray-500">{branchSummary.length} 个分支</span>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full text-sm border-collapse">
              <thead>
                <tr className="border-b border-gray-200/50 dark:border-white/10">
                  <th className={TH}>分支</th>
                  <th className={TH_NUM}>提交数</th>
                  <th className={TH_NUM}>代码行数</th>
                  <th className={TH_NUM}>实际耗时</th>
                  <th className={TH_CENTER}>提效比</th>
                </tr>
              </thead>
              <tbody>
                {branchSummary.map((b) => (
                  <tr
                    key={b.branch || '__unlabeled__'}
                    onClick={() => b.branch && handleBranchChange(b.branch)}
                    className={`border-b border-gray-100/50 dark:border-white/5 transition-colors ${
                      b.branch ? 'cursor-pointer hover:bg-apple-blue/5 dark:hover:bg-white/5' : ''
                    }`}
                  >
                    <td className={TD}>
                      <span className="font-mono break-all text-apple-blue">{b.branch || '(未标注分支)'}</span>
                    </td>
                    <td className={TD_NUM}>{formatNumber(b.count)}</td>
                    <td className={TD_NUM}>{b.diffLines.toLocaleString()}</td>
                    <td className={TD_NUM}>{formatDuration(b.realMin)}</td>
                    <td className="px-3 py-2 align-middle text-center">
                      {b.effRatio != null ? <PercentPill value={b.effRatio} /> : '-'}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      )}

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
                {/* 整仓态加「分支」列，标明每条 commit 属哪个分支。 */}
                {currentBranch === '' && <th className={TH}>分支</th>}
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
                    <SortableTh field="silica" label="AI 代码占比" active={parsedCommitOrder?.field === 'silica'} desc={parsedCommitOrder?.field === 'silica' && parsedCommitOrder.desc} onSort={(f) => setCommitOrder(cycle(parsedCommitOrder, f))} />
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
                  <td colSpan={currentBranch === '' ? 12 : 11}>
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
                      {currentBranch === '' && (
                        <td className={TD}>
                          <div className="max-w-[160px] truncate font-mono text-xs" title={commitBranch(c)}>{commitBranch(c) || '-'}</div>
                        </td>
                      )}
                      <td className={TD}><div className="max-w-[260px] truncate" title={c.comment}>{c.comment || '-'}</div></td>
                      <td className={TD_NUM}>{c.diff_lines ?? 0}</td>
                      <td className={TD_NUM}>{formatDuration(commitReal(c))}</td>
                      <td className={TD_NUM}>{formatDuration(commitAncient(c))}</td>
                      <td className="px-3 py-2 align-middle text-center">
                        {c.silica != null ? <Tag tone={aiCodeRatioTone(c.silica)}>{formatV2Ratio(c.silica)}</Tag> : '-'}
                      </td>
                      <td className="px-3 py-2 align-middle text-center">
                        {eff != null ? <PercentPill value={eff} /> : '-'}
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
                        {eff != null ? <PercentPill value={eff} /> : '-'}
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

      <AddRepoToProjectModal
        open={addOpen}
        repoAddr={repoAddr}
        repoBranch={currentBranch}
        commits={commits}
        startDate={params.startDate}
        endDate={params.endDate}
        onClose={() => setAddOpen(false)}
      />
    </div>
  )
}

const NEW_PROJECT = '__new__'

/**
 * RepoDetail「添加到 Project」（§4.2，加 repo filter）。
 * 两段式：先 checkProjectConflicts(目标 commit_ids)；有冲突则展示 conflicts 让用户「仍然添加」确认；
 * 无冲突直接 addRepoToProject。可选白名单（仅包含勾选的 commits），否则按日期范围过滤目标 commits。
 */
function AddRepoToProjectModal({
  open,
  repoAddr,
  repoBranch,
  commits,
  startDate,
  endDate,
  onClose,
}: {
  open: boolean
  repoAddr: string
  repoBranch: string
  commits: RepoCommitItem[]
  startDate: string
  endDate: string
  onClose: () => void
}) {
  const queryClient = useQueryClient()
  const [projects, setProjects] = useState<ProjectListItem[]>([])
  const [selectedProjectId, setSelectedProjectId] = useState('')
  const [newName, setNewName] = useState('')
  const [newDesc, setNewDesc] = useState('')
  const [whitelistMode, setWhitelistMode] = useState(false)
  const [whitelist, setWhitelist] = useState<Set<string>>(new Set())
  const [conflicts, setConflicts] = useState<ProjectConflict[]>([])
  const [conflictsChecked, setConflictsChecked] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [err, setErr] = useState('')

  useEffect(() => {
    if (!open) return
    setSelectedProjectId('')
    setNewName('')
    setNewDesc('')
    setWhitelistMode(false)
    setWhitelist(new Set())
    setConflicts([])
    setConflictsChecked(false)
    setErr('')
    getProjects()
      .then((res) => setProjects(res.data || []))
      .catch(() => setProjects([]))
  }, [open])

  // 改了目标范围/白名单则需重新检测冲突
  function resetConflictCheck() {
    setConflicts([])
    setConflictsChecked(false)
  }

  function toggleWhitelist(id: string) {
    setWhitelist((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
    resetConflictCheck()
  }

  // 目标 commit_ids：白名单模式取勾选；否则按日期范围过滤（commit_time 前 10 位在 [start,end]）。
  function getTargetCommitIds(): string[] {
    if (whitelistMode) return commits.filter((c) => whitelist.has(c.commit_id)).map((c) => c.commit_id)
    const s = startDate ? `${startDate.slice(0, 4)}-${startDate.slice(4, 6)}-${startDate.slice(6, 8)}` : ''
    const e = endDate ? `${endDate.slice(0, 4)}-${endDate.slice(4, 6)}-${endDate.slice(6, 8)}` : ''
    return commits
      .filter((c) => {
        const d = (c.commit_time || '').slice(0, 10)
        if (!d) return false
        if (s && d < s) return false
        if (e && d > e) return false
        return true
      })
      .map((c) => c.commit_id)
  }

  async function doAdd() {
    setSubmitting(true)
    setErr('')
    try {
      let projectId = selectedProjectId
      if (selectedProjectId === NEW_PROJECT) {
        if (!newName.trim()) {
          setErr('请输入新项目名称')
          setSubmitting(false)
          return
        }
        const created = await createProject({ name: newName.trim(), description: newDesc.trim() })
        projectId = created.project_id
      }
      await addRepoToProject(projectId, {
        repo_addr: repoAddr,
        repo_branch: repoBranch,
        start_time: null,
        end_time: null,
        include_only_commits: whitelistMode ? getTargetCommitIds() : [],
        exclude_commits: [],
      })
      // 加 Repo filter 改变了 Project 构成 → 失效项目列表/该项目详情缓存。
      await queryClient.invalidateQueries({ queryKey: ['project-list'] })
      await queryClient.invalidateQueries({ queryKey: ['project-detail', projectId] })
      onClose()
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : '添加失败')
    } finally {
      setSubmitting(false)
    }
  }

  async function handleConfirm() {
    if (!selectedProjectId) {
      setErr('请选择目标 Project')
      return
    }
    if (selectedProjectId === NEW_PROJECT && !newName.trim()) {
      setErr('请输入新项目名称')
      return
    }
    // 第一段：先检测冲突
    if (!conflictsChecked) {
      const targets = getTargetCommitIds()
      if (targets.length === 0) {
        setErr('没有可添加的 Commits')
        return
      }
      setSubmitting(true)
      setErr('')
      try {
        const res = await checkProjectConflicts({ commit_ids: targets })
        const found = res.conflicts || []
        setConflicts(found)
        setConflictsChecked(true)
        if (found.length > 0) {
          // 有冲突 → 停下等用户「仍然添加」
          setSubmitting(false)
          return
        }
      } catch (e: unknown) {
        setErr(e instanceof Error ? e.message : '冲突检测失败')
        setSubmitting(false)
        return
      }
      setSubmitting(false)
    }
    // 第二段：无冲突（或已确认）→ 添加
    await doAdd()
  }

  const inputCls =
    'glass rounded-lg px-3 py-1.5 text-sm w-full bg-transparent text-gray-900 dark:text-white ' +
    'focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue'
  const hasConflict = conflictsChecked && conflicts.length > 0

  return (
    <Modal
      open={open}
      title="添加到 Project"
      maxWidth={750}
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
          {hasConflict ? (
            <button
              type="button"
              onClick={doAdd}
              disabled={submitting}
              className="bg-amber-500 hover:bg-amber-600 text-white rounded-lg px-4 py-1.5 text-sm font-medium cursor-pointer transition-colors disabled:opacity-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-amber-500"
            >
              {submitting ? '添加中...' : '仍然添加'}
            </button>
          ) : (
            <button
              type="button"
              onClick={handleConfirm}
              disabled={submitting}
              className="bg-apple-blue hover:bg-apple-blue-hover text-white rounded-lg px-4 py-1.5 text-sm font-medium cursor-pointer transition-colors disabled:opacity-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue"
            >
              {submitting ? '处理中...' : '确认'}
            </button>
          )}
        </>
      }
    >
      <div className="space-y-3">
        {err && <div className="text-sm text-rose-600 dark:text-rose-400">{err}</div>}
        <RepoModalField label="目标 Project">
          <select
            value={selectedProjectId}
            onChange={(e) => {
              setSelectedProjectId(e.target.value)
              resetConflictCheck()
            }}
            className={`${inputCls} cursor-pointer`}
          >
            <option value="">请选择…</option>
            <option value={NEW_PROJECT}>+ 新建 Project</option>
            {projects.map((p) => (
              <option key={p.project_id} value={p.project_id}>
                {p.name}
              </option>
            ))}
          </select>
        </RepoModalField>
        {selectedProjectId === NEW_PROJECT && (
          <>
            <RepoModalField label="名称">
              <input type="text" value={newName} onChange={(e) => setNewName(e.target.value)} className={inputCls} />
            </RepoModalField>
            <RepoModalField label="描述">
              <input type="text" value={newDesc} onChange={(e) => setNewDesc(e.target.value)} className={inputCls} />
            </RepoModalField>
          </>
        )}
        <label className="inline-flex items-center gap-2 text-sm text-gray-600 dark:text-gray-300 cursor-pointer select-none">
          <input
            type="checkbox"
            checked={whitelistMode}
            onChange={(e) => {
              setWhitelistMode(e.target.checked)
              resetConflictCheck()
            }}
            className="accent-apple-blue cursor-pointer"
          />
          仅包含指定 Commits（白名单）
        </label>

        {whitelistMode && (
          <div className="glass rounded-xl overflow-hidden max-h-[300px] overflow-y-auto">
            <table className="w-full text-sm border-collapse">
              <thead className="sticky top-0">
                <tr className="border-b border-gray-200/50 dark:border-white/10 bg-white/60 dark:bg-white/5">
                  <th className="px-3 py-2 w-10"></th>
                  <th className="px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400">Commit ID</th>
                  <th className="px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400">说明</th>
                  <th className="px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400">用户</th>
                  <th className="px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400">时间</th>
                  <th className="px-3 py-2 text-right font-semibold text-gray-500 dark:text-gray-400">代码行数</th>
                </tr>
              </thead>
              <tbody>
                {commits.map((c) => (
                  <tr key={c.commit_id} className="border-b border-gray-100/50 dark:border-white/5">
                    <td className="px-3 py-2 text-center">
                      <input
                        type="checkbox"
                        checked={whitelist.has(c.commit_id)}
                        onChange={() => toggleWhitelist(c.commit_id)}
                        aria-label={`选择 ${c.commit_id}`}
                        className="accent-apple-blue cursor-pointer align-middle"
                      />
                    </td>
                    <td className="px-3 py-2 font-mono text-gray-700 dark:text-gray-200">{(c.commit_id || '').substring(0, 8)}</td>
                    <td className="px-3 py-2 text-gray-700 dark:text-gray-200"><div className="max-w-[200px] truncate" title={c.comment}>{c.comment || '-'}</div></td>
                    <td className="px-3 py-2 text-gray-700 dark:text-gray-200">{c.git_user_name || '-'}</td>
                    <td className="px-3 py-2 text-gray-700 dark:text-gray-200">{formatLocalTime(c.commit_time)}</td>
                    <td className="px-3 py-2 text-right tabular-nums text-gray-700 dark:text-gray-200">{c.diff_lines ?? 0}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {hasConflict && (
          <div className="rounded-xl border border-amber-400/60 bg-amber-50/60 dark:bg-amber-900/20 p-3 text-sm">
            <div className="font-medium text-amber-700 dark:text-amber-300 mb-1">以下 Commits 已属于其他 Project：</div>
            <ul className="space-y-0.5 text-amber-700 dark:text-amber-300">
              {conflicts.map((c) => (
                <li key={c.commit_id} className="font-mono text-xs">
                  {(c.commit_id || '').substring(0, 8)} → {c.project_name}
                </li>
              ))}
            </ul>
          </div>
        )}
      </div>
    </Modal>
  )
}

function RepoModalField({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="block">
      <span className="block text-xs text-gray-500 dark:text-gray-400 mb-1">{label}</span>
      {children}
    </label>
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
