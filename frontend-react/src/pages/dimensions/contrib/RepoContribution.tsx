// 仓库「贡献」维度 —— 看板派生口径（平台 chat-stats 源无仓库维度 → 贡献用看板：提交数 / 代码行 / 分支 / 贡献者）。
// 与 kanbanDerived.tsx 的 Repo* 视图同构（同一壳两态、同一表格 class、同一玻璃拟态），仅维度=贡献。
//
// ⚠️ 数据现实（核对 api/types.ts，不编造字段）：
//   · 聚合态用 useRepos（GET /v2/repos，服务端分页，pageSize 拉大客户端聚合）。RepoListItem 含
//     commit_count / task_count / ai_code_ratio / efficiency_ratio / start_time，**无 diff_lines（代码行）
//     与 branch_count（分支数）** → 聚合排行只展示 commit/task/AI 占比，代码行与分支数走聚焦态明细。
//   · 聚焦态用 useRepoDetail（GET /v2/repos/detail），从 commits[].diff_lines / git_user_name 现算
//     代码行总数、贡献者数、分支数（来自 branches[]），并按 git_user_name 拆「按贡献者」小表（提交数 / 代码行）。
//   · 时间线：仓库无按周的贡献时序端点（周表是 用户×周 聚合，无仓库维度，见 EfficiencyDimension）→
//     DimensionTrend 走 unavailable 诚实空态，不假装有趋势。
// 口径：efficiency_ratio 百分比口径 → PercentPill；ai_code_ratio 小数口径 → RatioPill。绝不互换。
import { useMemo } from 'react'
import { useRepos, useRepoBranches, useRepoDetail } from '@/api/queries'
import { useEntityFocus } from '@/components/layout/EntityDimensionLayout'
import { useViewState } from '@/store/viewState'
import { MetricCard } from '@/components/ui/MetricCard'
import { RatioPill } from '@/components/ui/RatioPill'
import { PercentPill } from '@/components/ui/PercentPill'
import { DimensionTrend } from '@/components/executive/DimensionTrend'
import { ChartCard, EmptyHint } from '@/pages/platform/platformShared'
import { formatNumber } from '@/lib/formatters'
import { formatDateParam } from '@/lib/date'
import { sortRows } from '@/lib/sort'
import type { RepoListItem, RepoCommitItem } from '@/api/types'

const TH = 'px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_NUM = 'px-3 py-2 text-right font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_CENTER = 'px-3 py-2 text-center font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TD = 'px-3 py-2 align-middle text-gray-700 dark:text-gray-200 whitespace-nowrap'
const TD_NUM = 'px-3 py-2 align-middle text-right tabular-nums text-gray-700 dark:text-gray-200'

/** 平台口径缺位提示（贡献维度同样标注：仓库贡献为看板派生，非平台 tokens 消耗量）。 */
function DerivedContribNote() {
  return (
    <p className="text-xs text-gray-400 dark:text-gray-500">
      平台（chat-stats）源无仓库维度，贡献为<b className="font-medium">看板派生口径</b>（提交数 / 代码行 / 分支 / 贡献者），
      非平台 tokens 消耗量。
    </p>
  )
}

export default function RepoContribution() {
  const { object, objectLabel } = useEntityFocus()
  const { timeRange } = useViewState()
  const focused = object !== ''

  return (
    <div className="flex flex-col gap-5">
      {/* 页首主角：时间线（仓库贡献无周时序 → 诚实空态） */}
      <DimensionTrend
        rows={[]}
        unavailable
        title="贡献趋势"
        subtitle={focused ? `仓库 · ${objectLabel || object}` : '仓库口径'}
        unavailableNote="仓库暂无按周的贡献时间线（周表按 用户×周 聚合，无仓库维度）。下方为看板派生口径的贡献排行与明细。"
      />

      {focused ? (
        <RepoContribFocus repoAddr={object} timeRange={timeRange} />
      ) : (
        <RepoContribAggregate timeRange={timeRange} />
      )}
    </div>
  )
}

// ==================================== 聚合态：仓库贡献排行 ====================================
function RepoContribAggregate({ timeRange }: { timeRange: [string, string] }) {
  const dateParams = useMemo(
    () => ({ startDate: formatDateParam(timeRange[0]), endDate: formatDateParam(timeRange[1]) }),
    [timeRange],
  )
  const { data, isLoading, error } = useRepos({ ...dateParams, page: 1, pageSize: 1000 })
  const rows = useMemo<RepoListItem[]>(() => data?.data ?? [], [data])

  // 贡献维度按提交数降序（null 沉底）——提交量是仓库贡献的主排序口径。
  const ranked = useMemo(() => sortRows(rows, (r) => r.commit_count, true), [rows])

  const kpi = useMemo(() => {
    const repos = rows.length
    const commits = rows.reduce((s, r) => s + (r.commit_count || 0), 0)
    const tasks = rows.reduce((s, r) => s + (r.task_count || 0), 0)
    const aiVals = rows.map((r) => Number(r.ai_code_ratio)).filter((v) => Number.isFinite(v))
    const avgAi = aiVals.length ? aiVals.reduce((a, b) => a + b, 0) / aiVals.length : null
    return { repos, commits, tasks, avgAi }
  }, [rows])

  if (error) return <DerivedError msg={(error as Error).message} />

  return (
    <div className="flex flex-col gap-5">
      <DerivedContribNote />
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        <MetricCard label="仓库数" value={formatNumber(kpi.repos)} />
        <MetricCard label="Commit 总数" value={formatNumber(kpi.commits)} hint="各仓库提交数合计" />
        <MetricCard label="Task 总数" value={formatNumber(kpi.tasks)} hint="各仓库任务数合计" />
        <MetricCard label="平均 AI 占比" value={<RatioPill value={kpi.avgAi} />} hint="各仓库 ai_code_ratio 均值" />
      </div>
      <ChartCard title="仓库贡献排行（看板派生）" sub="按 Commit 数倒序（每仓库取首选分支）">
        <p className="text-xs text-gray-400 dark:text-gray-500 mb-2">
          仓库列表口径无代码行 / 分支数字段 → 进单仓库详情可见按贡献者拆分的代码行与分支明细。
        </p>
        <div className="overflow-x-auto max-h-[520px] overflow-y-auto">
          <table className="w-full text-sm border-collapse">
            <thead className="sticky top-0 bg-white/70 dark:bg-gray-900/70 backdrop-blur">
              <tr className="border-b border-gray-200/50 dark:border-white/10">
                <th className={TH_NUM}>排名</th>
                <th className={TH}>仓库</th>
                <th className={TH}>分支</th>
                <th className={TH_NUM}>Commit数</th>
                <th className={TH_NUM}>Task数</th>
                <th className={TH_CENTER}>AI 占比</th>
                <th className={TH_CENTER}>提效比</th>
              </tr>
            </thead>
            <tbody>
              {isLoading ? (
                <SkeletonRows cols={7} />
              ) : ranked.length === 0 ? (
                <tr>
                  <td colSpan={7}>
                    <EmptyHint compact />
                  </td>
                </tr>
              ) : (
                ranked.map((r, i) => (
                  <tr key={`${r.repo_addr}#${r.repo_branch}`} className="border-b border-gray-100/50 dark:border-white/5">
                    <td className={TD_NUM}>{i + 1}</td>
                    <td className={TD}>
                      <div className="max-w-[320px] truncate" title={r.repo_addr}>{r.repo_addr || '-'}</div>
                    </td>
                    <td className={TD}>
                      <div className="max-w-[160px] truncate" title={r.repo_branch}>{r.repo_branch || '-'}</div>
                    </td>
                    <td className={TD_NUM}>{formatNumber(r.commit_count)}</td>
                    <td className={TD_NUM}>{formatNumber(r.task_count)}</td>
                    <td className="px-3 py-2 align-middle text-center"><RatioPill value={r.ai_code_ratio ?? null} /></td>
                    <td className="px-3 py-2 align-middle text-center"><PercentPill value={r.efficiency_ratio} /></td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </ChartCard>
    </div>
  )
}

// ==================================== 聚焦态：单仓库贡献明细（按贡献者拆分） ====================================
interface ContribRow {
  name: string
  commits: number
  diffLines: number
}

function RepoContribFocus({ repoAddr, timeRange }: { repoAddr: string; timeRange: [string, string] }) {
  // 仓库详情按首选分支取（与 RepoDetail embedded 一致：未指定 repoBranch 后端回首选分支）。
  const { data: branchesData } = useRepoBranches(repoAddr)
  const branches = useMemo<string[]>(() => branchesData?.branches ?? [], [branchesData])

  const params = useMemo(
    () => ({
      repoAddr,
      startDate: timeRange[0].replace(/-/g, ''),
      endDate: timeRange[1].replace(/-/g, ''),
    }),
    [repoAddr, timeRange],
  )
  const { data, isLoading, error } = useRepoDetail(params)
  const commits = useMemo<RepoCommitItem[]>(() => data?.commits ?? [], [data])

  // 按贡献者（git_user_name）拆：提交数 + 代码行合计。无名归「(未署名)」。
  const contributors = useMemo<ContribRow[]>(() => {
    const map = new Map<string, ContribRow>()
    for (const c of commits) {
      const name = (c.git_user_name || '').trim() || '(未署名)'
      const cur = map.get(name) || { name, commits: 0, diffLines: 0 }
      cur.commits += 1
      cur.diffLines += c.diff_lines || 0
      map.set(name, cur)
    }
    return Array.from(map.values()).sort((a, b) => b.commits - a.commits || b.diffLines - a.diffLines)
  }, [commits])

  const kpi = useMemo(() => {
    const totalCommits = commits.length
    const totalDiff = commits.reduce((s, c) => s + (c.diff_lines || 0), 0)
    const branchCount = branches.length
    const contributorCount = contributors.length
    return { totalCommits, totalDiff, branchCount, contributorCount }
  }, [commits, branches, contributors])

  if (error) return <DerivedError msg={(error as Error).message} />

  return (
    <div className="flex flex-col gap-5">
      <DerivedContribNote />
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        <MetricCard label="贡献者" value={isLoading ? '…' : `${formatNumber(kpi.contributorCount)} 人`} hint="按 commit 署名去重" />
        <MetricCard label="提交数" value={isLoading ? '…' : formatNumber(kpi.totalCommits)} />
        <MetricCard label="代码行数" value={isLoading ? '…' : (kpi.totalDiff > 0 ? `${formatNumber(kpi.totalDiff)} 行` : '-')} hint="各 commit diff 行合计" />
        <MetricCard label="分支数" value={isLoading ? '…' : formatNumber(kpi.branchCount)} />
      </div>
      <ChartCard title="按贡献者拆分（看板派生）" sub="按提交数倒序">
        <table className="w-full text-sm border-collapse">
          <thead>
            <tr className="border-b border-gray-200/50 dark:border-white/10">
              <th className={TH_NUM}>排名</th>
              <th className={TH}>贡献者</th>
              <th className={TH_NUM}>提交数</th>
              <th className={TH_NUM}>代码行数</th>
              <th className={TH_NUM}>提交占比</th>
            </tr>
          </thead>
          <tbody>
            {isLoading ? (
              <SkeletonRows cols={5} />
            ) : contributors.length === 0 ? (
              <tr>
                <td colSpan={5}>
                  <EmptyHint compact />
                </td>
              </tr>
            ) : (
              contributors.map((r, i) => (
                <tr key={r.name} className="border-b border-gray-100/50 dark:border-white/5">
                  <td className={TD_NUM}>{i + 1}</td>
                  <td className={TD}>
                    <div className="max-w-[260px] truncate font-medium text-gray-900 dark:text-white" title={r.name}>{r.name}</div>
                  </td>
                  <td className={TD_NUM}>{formatNumber(r.commits)}</td>
                  <td className={TD_NUM}>{r.diffLines > 0 ? `${formatNumber(r.diffLines)} 行` : '-'}</td>
                  <td className={TD_NUM}>
                    {kpi.totalCommits > 0 ? `${((r.commits / kpi.totalCommits) * 100).toFixed(1)}%` : '-'}
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </ChartCard>
    </div>
  )
}

// ==================================== 共用小件 ====================================
function SkeletonRows({ cols }: { cols: number }) {
  return (
    <>
      {Array.from({ length: 6 }).map((_, i) => (
        <tr key={i} className="border-b border-gray-100/50 dark:border-white/5">
          <td colSpan={cols} className="px-3 py-2">
            <div className="skeleton h-6 rounded" />
          </td>
        </tr>
      ))}
    </>
  )
}

function DerivedError({ msg }: { msg?: string }) {
  return (
    <div className="glass rounded-2xl p-8 text-center text-sm text-rose-600 dark:text-rose-400">
      {msg || '获取看板派生数据失败'}
    </div>
  )
}
