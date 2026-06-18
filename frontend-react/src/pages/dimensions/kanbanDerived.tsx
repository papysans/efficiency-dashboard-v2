// 项目/仓库（project/repo）维度的「看板派生」视图 —— 平台源无项目/仓库字段，这两个主体的
// 「使用」「成本」格**填不了平台**，故走看板派生口径：
//   使用：project=AI 渗透率/贡献者(ProjectList/Detail) · repo=AI 代码占比(RepoList/Detail)。
//   成本：project=Need 费用(v1.3.8) · repo=会话费用。**单卡看板费用**（非 user/org 的双卡），
//        并显式注明「平台无项目/仓库口径」。
// 聚合态=各对象排行（点行下钻独立详情）；聚焦态=embed 详情（壳保留面包屑/对象选择器）。
// ⚠️ 这两个主体不发任何 chatStats 请求（与 user/org 平台分支严格区分）。口径：
//   project need_* 字段=小数口径 → RatioPill；repo efficiency/ai=百分比/小数口径见各列。
import { useMemo } from 'react'
import { useNavigate } from 'react-router'
import { useProjectList, useRepos } from '@/api/queries'
import { MetricCard } from '@/components/ui/MetricCard'
import { RatioPill } from '@/components/ui/RatioPill'
import { ChartCard, EmptyHint } from '@/pages/platform/platformShared'
import { fmtCost, formatNumber } from '@/lib/formatters'
import { formatDateParam } from '@/lib/date'
import { sortRows } from '@/lib/sort'
import ProjectDetail from '@/pages/projects/ProjectDetail'
import RepoDetail from '@/pages/repos/RepoDetail'
import type { ProjectListItem, RepoListItem } from '@/api/types'

const TH = 'px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_NUM = 'px-3 py-2 text-right font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_CENTER = 'px-3 py-2 text-center font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TD = 'px-3 py-2 align-middle text-gray-700 dark:text-gray-200 whitespace-nowrap'
const TD_NUM = 'px-3 py-2 align-middle text-right tabular-nums text-gray-700 dark:text-gray-200'

/** 平台口径缺位提示（project/repo 的成本/使用都显式标注，避免误以为是平台¥）。 */
function PlatformNoCaliberNote({ what }: { what: string }) {
  return (
    <p className="text-xs text-gray-400 dark:text-gray-500">
      平台（chat-stats）源无项目 / 仓库维度，{what}为<b className="font-medium">看板派生口径</b>，非平台客观¥。
    </p>
  )
}

// ==================================== 使用（看板派生） ====================================
export function KanbanUsage({
  entity,
  object,
  focused,
  timeRange,
}: {
  entity: 'project' | 'repo'
  object: string
  objectLabel: string
  focused: boolean
  timeRange: [string, string]
}) {
  // objectLabel 仅聚焦态详情用（详情组件自取），本组件不消费。
  if (focused) {
    return (
      <div className="flex flex-col gap-4">
        <PlatformNoCaliberNote what="使用口径（AI 渗透 / AI 代码占比）" />
        {entity === 'project' ? (
          <ProjectDetail projectIdProp={object} embedded />
        ) : (
          <RepoDetail repoAddrProp={object} dateRangeProp={timeRange} embedded />
        )}
      </div>
    )
  }
  return entity === 'project' ? <ProjectUsageAggregate timeRange={timeRange} /> : <RepoUsageAggregate timeRange={timeRange} />
}

function ProjectUsageAggregate({ timeRange }: { timeRange: [string, string] }) {
  const navigate = useNavigate()
  const goToProject = useProjectNav(navigate)
  const dateParams = useMemo(
    () => ({ startDate: formatDateParam(timeRange[0]), endDate: formatDateParam(timeRange[1]) }),
    [timeRange],
  )
  const { data, isLoading, error } = useProjectList(dateParams)
  const rows = useMemo<ProjectListItem[]>(() => data?.data ?? [], [data])

  // 按 AI 占比降序（null 沉底）——使用维度看 AI 渗透。
  const ranked = useMemo(() => sortRows(rows, (r) => r.need_ai_code_ratio, true), [rows])

  const kpi = useMemo(() => {
    const projects = rows.length
    const needs = rows.reduce((s, r) => s + (r.need_total_count || 0), 0)
    const aiProjects = rows.filter((r) => r.need_ai_code_ratio != null).length
    // LOC 加权守恒：Σ(ratio_i × loc_i) / Σ(loc_i)，仅累加 ratio 有限且 loc>0 的项目。
    let aiNum = 0
    let aiDen = 0
    for (const r of rows) {
      const ratio = Number(r.need_ai_code_ratio)
      const loc = Number(r.need_total_loc_net)
      if (Number.isFinite(ratio) && Number.isFinite(loc) && loc > 0) {
        aiNum += ratio * loc
        aiDen += loc
      }
    }
    const avgAi = aiDen > 0 ? aiNum / aiDen : null
    return { projects, needs, aiProjects, avgAi }
  }, [rows])

  if (error) return <DerivedError msg={(error as Error).message} />

  return (
    <div className="flex flex-col gap-5">
      <PlatformNoCaliberNote what="使用口径（AI 渗透 / 活动量）" />
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        <MetricCard label="项目数" value={formatNumber(kpi.projects)} />
        <MetricCard label="有 AI 数据项目数" value={formatNumber(kpi.aiProjects)} />
        <MetricCard label="需求总数" value={formatNumber(kpi.needs)} />
        <MetricCard label="平均 AI 占比" value={<RatioPill value={kpi.avgAi} />} hint="按生成代码行加权" />
      </div>
      <ChartCard title="项目 AI 渗透排行（看板派生）" sub="按需求 AI 代码占比倒序 · 点行下钻">
        <table className="w-full text-sm border-collapse">
          <thead>
            <tr className="border-b border-gray-200/50 dark:border-white/10">
              <th className={TH_NUM}>排名</th>
              <th className={TH}>项目</th>
              <th className={TH_CENTER}>AI 占比</th>
              <th className={TH_NUM}>需求数</th>
            </tr>
          </thead>
          <tbody>
            {isLoading ? (
              <SkeletonRows cols={4} />
            ) : ranked.length === 0 ? (
              <tr>
                <td colSpan={4}>
                  <EmptyHint compact />
                </td>
              </tr>
            ) : (
              ranked.map((r, i) => (
                <tr
                  key={r.project_id}
                  onClick={() => goToProject(r.project_id)}
                  className="border-b border-gray-100/50 dark:border-white/5 cursor-pointer hover:bg-apple-blue/5 dark:hover:bg-white/5 transition-colors"
                >
                  <td className={TD_NUM}>{i + 1}</td>
                  <td className={TD}>
                    <ProjectNameButton name={r.name} onClick={() => goToProject(r.project_id)} />
                  </td>
                  <td className="px-3 py-2 align-middle text-center"><RatioPill value={r.need_ai_code_ratio ?? null} /></td>
                  <td className={TD_NUM}>{formatNumber(r.need_total_count)}</td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </ChartCard>
    </div>
  )
}

function RepoUsageAggregate({ timeRange }: { timeRange: [string, string] }) {
  const navigate = useNavigate()
  const goToRepo = useRepoNav(navigate)
  const dateParams = useMemo(
    () => ({ startDate: formatDateParam(timeRange[0]), endDate: formatDateParam(timeRange[1]) }),
    [timeRange],
  )
  const { data, isLoading, error } = useRepos({ ...dateParams, page: 1, pageSize: 1000 })
  const rows = useMemo<RepoListItem[]>(() => data?.data ?? [], [data])

  const ranked = useMemo(() => sortRows(rows, (r) => r.ai_code_ratio, true), [rows])

  const kpi = useMemo(() => {
    const repos = rows.length
    const commits = rows.reduce((s, r) => s + (r.commit_count || 0), 0)
    // commit 加权守恒：Σ(ratio_i × commit_i) / Σ(commit_i)，仅累加 ratio 有限且 commit>0 的仓库。
    let aiNum = 0
    let aiDen = 0
    for (const r of rows) {
      const ratio = Number(r.ai_code_ratio)
      const cc = Number(r.commit_count)
      if (Number.isFinite(ratio) && Number.isFinite(cc) && cc > 0) {
        aiNum += ratio * cc
        aiDen += cc
      }
    }
    const avgAi = aiDen > 0 ? aiNum / aiDen : null
    return { repos, commits, avgAi }
  }, [rows])

  if (error) return <DerivedError msg={(error as Error).message} />

  return (
    <div className="flex flex-col gap-5">
      <PlatformNoCaliberNote what="使用口径（AI 代码占比）" />
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        <MetricCard label="仓库数" value={formatNumber(kpi.repos)} />
        <MetricCard label="Commit 总数" value={formatNumber(kpi.commits)} />
        <MetricCard label="平均 AI 占比" value={<RatioPill value={kpi.avgAi} />} hint="按 Commit 数加权" />
        <MetricCard label="有 AI 数据仓库" value={formatNumber(rows.filter((r) => r.ai_code_ratio != null).length)} />
      </div>
      <ChartCard title="仓库 AI 占比排行（看板派生）" sub="按 AI 代码占比倒序（整仓跨全部分支聚合）· 点行下钻进整仓详情">
        <div className="overflow-x-auto max-h-[520px] overflow-y-auto">
          <table className="w-full text-sm border-collapse">
            <thead className="sticky top-0 bg-white/70 dark:bg-gray-900/70 backdrop-blur">
              <tr className="border-b border-gray-200/50 dark:border-white/10">
                <th className={TH_NUM}>排名</th>
                <th className={TH}>仓库</th>
                <th className={TH_NUM}>分支</th>
                <th className={TH_CENTER}>AI 占比</th>
                <th className={TH_NUM}>Commit数</th>
                <th className={TH_NUM}>Task数</th>
              </tr>
            </thead>
            <tbody>
              {isLoading ? (
                <SkeletonRows cols={6} />
              ) : ranked.length === 0 ? (
                <tr>
                  <td colSpan={6}>
                    <EmptyHint compact />
                  </td>
                </tr>
              ) : (
                ranked.map((r, i) => (
                  <tr
                    key={r.repo_addr}
                    onClick={() => goToRepo(r.repo_addr)}
                    className="border-b border-gray-100/50 dark:border-white/5 cursor-pointer hover:bg-apple-blue/5 dark:hover:bg-white/5 transition-colors"
                  >
                    <td className={TD_NUM}>{i + 1}</td>
                    <td className={TD}>
                      <RepoAddrButton addr={r.repo_addr} onClick={() => goToRepo(r.repo_addr)} />
                    </td>
                    <td className={TD_NUM}>{r.branch_count ? `${formatNumber(r.branch_count)} 支` : '-'}</td>
                    <td className="px-3 py-2 align-middle text-center"><RatioPill value={r.ai_code_ratio ?? null} /></td>
                    <td className={TD_NUM}>{formatNumber(r.commit_count)}</td>
                    <td className={TD_NUM}>{formatNumber(r.task_count)}</td>
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

// ==================================== 成本（看板派生·单卡） ====================================
export function KanbanCost({
  entity,
  object,
  focused,
  timeRange,
}: {
  entity: 'project' | 'repo'
  object: string
  objectLabel: string
  focused: boolean
  timeRange: [string, string]
}) {
  // objectLabel 仅聚焦态详情用（详情组件自取），本组件不消费。
  if (focused) {
    return (
      <div className="flex flex-col gap-4">
        <SingleCostNotice />
        {entity === 'project' ? (
          <ProjectDetail projectIdProp={object} embedded />
        ) : (
          <RepoDetail repoAddrProp={object} dateRangeProp={timeRange} embedded />
        )}
      </div>
    )
  }
  return entity === 'project' ? <ProjectCostAggregate timeRange={timeRange} /> : <RepoCostAggregate timeRange={timeRange} />
}

/** 单卡成本顶部说明（与 user/org 双卡区分：project/repo 平台无口径，只有看板费用单卡）。 */
function SingleCostNotice() {
  return (
    <div className="glass rounded-xl px-4 py-3 flex items-start gap-2 text-sm border-l-4" style={{ borderLeftColor: '#af52de' }} role="note">
      <svg className="w-5 h-5 shrink-0 text-purple-500" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
      </svg>
      <span className="text-gray-600 dark:text-gray-300">
        成本 = <b className="text-gray-900 dark:text-white">看板费用</b>（会话调用花费聚合）。平台（chat-stats）无项目 / 仓库口径，
        故此处为<b>单卡看板费用</b>，不提供 user/org 的「平台AI花费 ‖ 人天」双卡。
      </span>
    </div>
  )
}

function ProjectCostAggregate({ timeRange }: { timeRange: [string, string] }) {
  const navigate = useNavigate()
  const goToProject = useProjectNav(navigate)
  const dateParams = useMemo(
    () => ({ startDate: formatDateParam(timeRange[0]), endDate: formatDateParam(timeRange[1]) }),
    [timeRange],
  )
  const { data, isLoading, error } = useProjectList(dateParams)
  const rows = useMemo<ProjectListItem[]>(() => data?.data ?? [], [data])
  const ranked = useMemo(() => sortRows(rows, (r) => r.need_cost, true), [rows])
  const totalCost = useMemo(() => rows.reduce((s, r) => s + (r.need_cost || 0), 0), [rows])
  const totalLoc = useMemo(() => rows.reduce((s, r) => s + (r.need_total_loc_net || 0), 0), [rows])
  // ¥/完成需求：分母用各项目已完成（status='merged'）需求数合计，比候选池总数更贴近「已交付成本」。
  const totalDone = useMemo(() => rows.reduce((s, r) => s + (r.need_done_count || 0), 0), [rows])
  const costPerDone = totalDone > 0 ? totalCost / totalDone : null

  if (error) return <DerivedError msg={(error as Error).message} />

  return (
    <div className="flex flex-col gap-5">
      <SingleCostNotice />
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        <MetricCard label="总费用" value={`¥${fmtCost(totalCost)}`} hint="各项目需求费用合计" />
        <MetricCard label="¥/完成需求" value={costPerDone != null ? `¥${fmtCost(costPerDone)}` : '-'} hint="总费用 / 已完成(merged)需求数" />
        <MetricCard label="平均单价" value={totalLoc > 0 ? `¥${fmtCost((totalCost / totalLoc) * 1000)} /千行` : '-'} hint="费用 / 生成代码千行" />
        <MetricCard label="生成代码(合计)" value={totalLoc > 0 ? `${formatNumber(totalLoc)} 行` : '-'} />
      </div>
      <ChartCard title="项目费用排行（看板派生·单卡）" sub="按需求费用倒序 · 点行下钻">
        <table className="w-full text-sm border-collapse">
          <thead>
            <tr className="border-b border-gray-200/50 dark:border-white/10">
              <th className={TH_NUM}>排名</th>
              <th className={TH}>项目</th>
              <th className={TH_NUM}>费用（¥）</th>
              <th className={TH_NUM}>生成代码</th>
            </tr>
          </thead>
          <tbody>
            {isLoading ? (
              <SkeletonRows cols={4} />
            ) : ranked.length === 0 ? (
              <tr>
                <td colSpan={4}>
                  <EmptyHint compact />
                </td>
              </tr>
            ) : (
              ranked.map((r, i) => (
                <tr
                  key={r.project_id}
                  onClick={() => goToProject(r.project_id)}
                  className="border-b border-gray-100/50 dark:border-white/5 cursor-pointer hover:bg-apple-blue/5 dark:hover:bg-white/5 transition-colors"
                >
                  <td className={TD_NUM}>{i + 1}</td>
                  <td className={TD}>
                    <ProjectNameButton name={r.name} onClick={() => goToProject(r.project_id)} />
                  </td>
                  <td className={TD_NUM}>{r.need_cost != null && r.need_cost > 0 ? fmtCost(r.need_cost) : '0.00'}</td>
                  <td className={TD_NUM}>{r.need_total_loc_net && r.need_total_loc_net > 0 ? `${formatNumber(r.need_total_loc_net)} 行` : '-'}</td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </ChartCard>
    </div>
  )
}

function RepoCostAggregate({ timeRange }: { timeRange: [string, string] }) {
  const navigate = useNavigate()
  const goToRepo = useRepoNav(navigate)
  const dateParams = useMemo(
    () => ({ startDate: formatDateParam(timeRange[0]), endDate: formatDateParam(timeRange[1]) }),
    [timeRange],
  )
  const { data, isLoading, error } = useRepos({ ...dateParams, page: 1, pageSize: 1000 })
  const rows = useMemo<RepoListItem[]>(() => data?.data ?? [], [data])
  // 费用从底层聚合：Need→session→tasks.cost（跨分支按 repo_addr 合并），与项目侧 need_cost 同口径。
  const ranked = useMemo(() => sortRows(rows, (r) => r.cost, true), [rows])
  const totalCost = useMemo(() => rows.reduce((s, r) => s + (r.cost || 0), 0), [rows])
  const totalCommits = useMemo(() => rows.reduce((s, r) => s + (r.commit_count || 0), 0), [rows])
  const costPerCommit = totalCommits > 0 ? totalCost / totalCommits : null

  if (error) return <DerivedError msg={(error as Error).message} />

  return (
    <div className="flex flex-col gap-5">
      <SingleCostNotice />
      <p className="text-xs text-gray-400 dark:text-gray-500">
        仓库费用 = 该仓库<b className="font-medium">干净需求的会话花费</b>（Need→session→task 链，按 session 去重，跨全部分支聚合），与项目侧同口径；
        无 tasks 成本数据的库显示 ¥0。
      </p>
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        <MetricCard label="总费用" value={`¥${fmtCost(totalCost)}`} hint="各仓库会话费用合计" />
        <MetricCard label="仓库数" value={formatNumber(rows.length)} />
        <MetricCard label="¥/Commit" value={costPerCommit != null ? `¥${fmtCost(costPerCommit)}` : '-'} hint="总费用 / Commit 总数" />
        <MetricCard label="Commit 总数" value={formatNumber(totalCommits)} />
      </div>
      <ChartCard title="仓库费用排行（看板派生·单卡）" sub="按会话费用倒序（整仓跨全部分支聚合）· 点行下钻进整仓详情">
        <div className="overflow-x-auto max-h-[520px] overflow-y-auto">
          <table className="w-full text-sm border-collapse">
            <thead className="sticky top-0 bg-white/70 dark:bg-gray-900/70 backdrop-blur">
              <tr className="border-b border-gray-200/50 dark:border-white/10">
                <th className={TH_NUM}>排名</th>
                <th className={TH}>仓库</th>
                <th className={TH_NUM}>分支</th>
                <th className={TH_NUM}>费用（¥）</th>
                <th className={TH_NUM}>Commit数</th>
              </tr>
            </thead>
            <tbody>
              {isLoading ? (
                <SkeletonRows cols={5} />
              ) : ranked.length === 0 ? (
                <tr>
                  <td colSpan={5}>
                    <EmptyHint compact />
                  </td>
                </tr>
              ) : (
                ranked.map((r, i) => (
                  <tr
                    key={r.repo_addr}
                    onClick={() => goToRepo(r.repo_addr)}
                    className="border-b border-gray-100/50 dark:border-white/5 cursor-pointer hover:bg-apple-blue/5 dark:hover:bg-white/5 transition-colors"
                  >
                    <td className={TD_NUM}>{i + 1}</td>
                    <td className={TD}>
                      <RepoAddrButton addr={r.repo_addr} onClick={() => goToRepo(r.repo_addr)} />
                    </td>
                    <td className={TD_NUM}>{r.branch_count ? `${formatNumber(r.branch_count)} 支` : '-'}</td>
                    <td className={TD_NUM}>{r.cost != null && r.cost > 0 ? fmtCost(r.cost) : '0.00'}</td>
                    <td className={TD_NUM}>{formatNumber(r.commit_count)}</td>
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

// ==================================== 共用小件 ====================================
// 行下钻导航助手（project/repo 排行点行进独立详情，与 ProjectList/RepoList 同址）。
type NavFn = ReturnType<typeof useNavigate>
function useProjectNav(navigate: NavFn) {
  return (projectId?: string) => {
    if (!projectId) return
    navigate(`/project/${encodeURIComponent(projectId)}`)
  }
}
function useRepoNav(navigate: NavFn) {
  return (repoAddr?: string, repoBranch?: string) => {
    if (!repoAddr) return
    navigate(`/repo/${encodeURIComponent(repoAddr)}/${encodeURIComponent(repoBranch || '')}`)
  }
}

/** 项目名链接按钮（截断 + 互链样式，行内点击 stopPropagation 防双触发）。 */
function ProjectNameButton({ name, onClick }: { name?: string; onClick: () => void }) {
  return (
    <button
      type="button"
      className="max-w-[240px] truncate text-left font-medium text-apple-blue hover:text-apple-blue-hover bg-transparent border-none p-0 cursor-pointer focus:outline-none focus-visible:underline"
      title={name}
      onClick={(e) => {
        e.stopPropagation()
        onClick()
      }}
    >
      {name || '-'}
    </button>
  )
}

/** 仓库地址链接按钮（截断 + 互链样式）。 */
function RepoAddrButton({ addr, onClick }: { addr?: string; onClick: () => void }) {
  return (
    <button
      type="button"
      className="max-w-[320px] truncate text-left text-apple-blue hover:text-apple-blue-hover bg-transparent border-none p-0 cursor-pointer focus:outline-none focus-visible:underline"
      title={addr}
      onClick={(e) => {
        e.stopPropagation()
        onClick()
      }}
    >
      {addr || '-'}
    </button>
  )
}

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
