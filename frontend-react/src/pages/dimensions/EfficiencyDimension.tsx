// 「效率」维度内容（4 主体共用，壳内 <Outlet> 渲染）。骨架：时间线 → KPI → 排行/明细。
// 两态由壳的聚焦对象（useEntityFocus().object）决定：
//   聚合态(object 空)：时间线(整体) → 排行/列表(现有 OrgTree/UserList/ProjectList/RepoList)。
//   聚焦态(选定对象)：时间线(该对象) → 该对象详情(复用 UserDetail/ProjectDetail/RepoDetail/DeptMembersPanel，embedded)。
// 聚合态额外提供「概览 / 分布」次级 tab：分布并入效率（DistributionOverview，全局 Need 分布，复用不重写）。
//
// 时间线数据策略（见返回简报）：
//   org/user → /v2/efficiency 周聚合行（一次拉回全部周，前端按周分桶；user 聚焦传 userId）。该端点是
//     user×week 表，无 project/repo 维度。
//   org 聚焦(单部门) → 该端点无「按部门」桶，且部门成员过滤需额外成员名册，本段诚实标注「按部门的周趋势建设中」，
//     部门 KPI/成员明细照常由 DeptMembersPanel 给出。
//   project/repo → 该端点无对应维度 → 时间线诚实标注不适用；KPI/明细走各自看板派生口径。
import { useMemo } from 'react'
import { useSearchParams } from 'react-router'
import { useEfficiencyV2, useProjectList, useAllUsers, useRepos, useRepoTrend, useProjectTrend } from '@/api/queries'
import type { ProjectListItem } from '@/api/types'
import { useViewState } from '@/store/viewState'
import { formatDateParam } from '@/lib/date'
import { fmtCost, formatNumber, formatV2Ratio } from '@/lib/formatters'
import { useEntityFocus } from '@/components/layout/EntityDimensionLayout'
import { DimensionTrend } from '@/components/executive/DimensionTrend'
import { EntityWeeklyTrend } from '@/components/executive/EntityWeeklyTrend'
import { MetricCard } from '@/components/ui/MetricCard'
import OrgTree from '@/pages/orgs/OrgTree'
import UserList from '@/pages/users/UserList'
import UserDetail from '@/pages/users/UserDetail'
import ProjectList from '@/pages/projects/ProjectList'
import ProjectDetail from '@/pages/projects/ProjectDetail'
import RepoList from '@/pages/repos/RepoList'
import RepoDetail from '@/pages/repos/RepoDetail'
import DistributionOverview from '@/pages/distribution/DistributionOverview'
import { EntityRatioHistogram } from '@/pages/distribution/EntityRatioHistogram'

type SubView = 'overview' | 'distribution'

export default function EfficiencyDimension() {
  const { entity, object, objectLabel } = useEntityFocus()
  const { timeRange } = useViewState()
  const [searchParams, setSearchParams] = useSearchParams()
  const focused = object !== ''

  // 聚合态次级 tab：概览 / 分布（分布=全局 Need 分布，仅聚合态展示）。URL ?sub= 驱动（深链/distribution-v2 重定向落得到）。
  const subView: SubView = searchParams.get('sub') === 'distribution' ? 'distribution' : 'overview'
  const setSubView = (v: SubView) => {
    const next = new URLSearchParams(searchParams)
    if (v === 'distribution') next.set('sub', 'distribution')
    else next.delete('sub')
    setSearchParams(next, { replace: true })
  }

  return (
    <div className="flex flex-col gap-5">
      {/* 页首主角：时间线 */}
      <EfficiencyTrend entity={entity} object={object} objectLabel={objectLabel} timeRange={timeRange} />

      {/* 聚合态：概览/分布 次级 tab；聚焦态不展示分布 */}
      {!focused && (
        <div className="flex items-center gap-1" role="tablist" aria-label="效率子视图">
          <SubTab active={subView === 'overview'} onClick={() => setSubView('overview')}>
            概览
          </SubTab>
          <SubTab active={subView === 'distribution'} onClick={() => setSubView('distribution')}>
            分布
          </SubTab>
        </div>
      )}

      {/* KPI + 排行/明细。聚焦优先于 subView（与 user/project/repo 一致），避免聚焦态停在全局分布且无控件切回。
          org 例外：聚焦/聚合都渲染 OrgTree（它本身是「树(浏览) + 右栏成员花名册(选中部门=聚焦)」复合视图，
          点树节点不该把树藏掉）；仅在聚合态(未选部门)下才允许切到全局分布。 */}
      {focused ? (
        entity === 'org' ? (
          <OrgTree />
        ) : (
          <FocusContent entity={entity} object={object} objectLabel={objectLabel} timeRange={timeRange} />
        )
      ) : subView === 'distribution' ? (
        <EntityDistribution entity={entity} timeRange={timeRange} />
      ) : entity === 'org' ? (
        <OrgTree />
      ) : (
        <AggregateContent entity={entity} />
      )}
    </div>
  )
}

/** 时间线分发：org/user 用周表；org-focus/project/repo 诚实标注。 */
function EfficiencyTrend({
  entity,
  object,
  objectLabel,
  timeRange,
}: {
  entity: string
  object: string
  objectLabel: string
  timeRange: [string, string]
}) {
  const focused = object !== ''
  // user：聚焦传 userId（单用户周趋势）；聚合不传（全量）。仅 org/user 启用周表查询。
  const enableTrend = entity === 'user' || (entity === 'org' && !focused)
  const userId = entity === 'user' && focused ? object : undefined
  const dateParams = useMemo(
    () => ({ startDate: formatDateParam(timeRange[0]), endDate: formatDateParam(timeRange[1]) }),
    [timeRange],
  )

  const trendQ = useEfficiencyV2({ ...dateParams, userId }, enableTrend)

  // 项目：聚焦态走「该项目干净 Need 按周提效」时间线（/v2/project-trend）；聚合态仍用项目级聚合 KPI 概览。
  if (entity === 'project') {
    return focused ? (
      <ProjectFocusTrend object={object} objectLabel={objectLabel} dateParams={dateParams} />
    ) : (
      <ProjectAggregateSummary />
    )
  }

  // 仓库：两态都走「commits 按周提效」时间线（/v2/repo-trend）。聚合=全部仓库，聚焦=单仓跨全部分支。
  if (entity === 'repo') {
    return <RepoEfficiencyTrend object={object} objectLabel={objectLabel} dateParams={dateParams} focused={focused} />
  }

  if (entity === 'org' && focused) {
    return (
      <DimensionTrend
        rows={[]}
        unavailable
        title="提效趋势"
        subtitle={`部门 · ${objectLabel || object}`}
        unavailableNote="按部门的周趋势建设中（周表按 用户×周 聚合，未带部门维度）。下方为该部门成员花名册与汇总指标。"
      />
    )
  }

  return (
    <DimensionTrend
      rows={trendQ.data?.data}
      loading={trendQ.isLoading}
      error={trendQ.error ? (trendQ.error as Error).message : null}
      title="提效趋势"
      subtitle={
        entity === 'user' && focused
          ? `个人 · ${objectLabel || object} · 按 ISO 周日历提效`
          : entity === 'user'
            ? '全部用户 · 按 ISO 周日历提效'
            : '全公司 · 按 ISO 周日历提效'
      }
    />
  )
}

/** 仓库提效时间线：commits 按 ISO 周现聚合（/v2/repo-trend，efficiency_pct 已百分比口径）。
 *  聚合态(repoAddr 空)=全部仓库；聚焦态=单仓跨全部分支。 */
function RepoEfficiencyTrend({
  object,
  objectLabel,
  dateParams,
  focused,
}: {
  object: string
  objectLabel: string
  dateParams: { startDate: string; endDate: string }
  focused: boolean
}) {
  const q = useRepoTrend({ repoAddr: focused ? object : undefined, ...dateParams })
  return (
    <EntityWeeklyTrend
      points={q.data?.data}
      loading={q.isLoading}
      error={q.error ? (q.error as Error).message : null}
      title="提效趋势"
      subtitle={focused ? `仓库 · ${objectLabel || object} · 按 ISO 周` : '全部仓库 · 按 ISO 周（commits 聚合）'}
      metric="efficiency"
    />
  )
}

/** 项目聚焦态提效时间线：该项目干净 Need 按 dev_end_ts 的 ISO 周现聚合（/v2/project-trend）。 */
function ProjectFocusTrend({
  object,
  objectLabel,
  dateParams,
}: {
  object: string
  objectLabel: string
  dateParams: { startDate: string; endDate: string }
}) {
  const q = useProjectTrend({ projectId: object, ...dateParams })
  return (
    <EntityWeeklyTrend
      points={q.data?.data}
      loading={q.isLoading}
      error={q.error ? (q.error as Error).message : null}
      title="提效趋势"
      subtitle={`项目 · ${objectLabel || object} · 按 ISO 周（干净需求聚合）`}
      metric="efficiency"
    />
  )
}

/**
 * #2 项目聚合态时间线位的替代：项目级聚合 KPI（项目维度天然无按周时序，聚合信息更有用）。
 * 全用 useProjectList()(ProjectListItem) 现成字段（与项目列表/详情同源、纯 Need(branch) 口径）求和/取均，
 * 口径不混：提效比/AI占比为**小数口径**（formatV2Ratio ×100 / RatioPill），费用为 ¥。
 * 平均日历提效比 = 各项目 need_calendar_efficiency_ratio 的算术均值（仅计有限值，与项目排行同源）。
 */
function ProjectAggregateSummary() {
  const { data, isLoading, error } = useProjectList()
  const rows = useMemo<ProjectListItem[]>(() => data?.data ?? [], [data])

  const agg = useMemo(() => {
    let eligible = 0
    let total = 0
    let cost = 0
    let loc = 0
    let users = 0
    const calRatios: number[] = []
    const aiRatios: number[] = []
    for (const r of rows) {
      eligible += r.need_eligible_count ?? 0
      total += r.need_total_count ?? 0
      cost += r.need_cost ?? 0
      loc += r.need_total_loc_net ?? 0
      users += r.user_count ?? 0
      const cal = Number(r.need_calendar_efficiency_ratio)
      if (r.need_calendar_efficiency_ratio != null && Number.isFinite(cal)) calRatios.push(cal)
      const ai = Number(r.need_ai_code_ratio)
      if (r.need_ai_code_ratio != null && Number.isFinite(ai)) aiRatios.push(ai)
    }
    const mean = (xs: number[]) => (xs.length > 0 ? xs.reduce((s, v) => s + v, 0) / xs.length : null)
    return {
      projectCount: rows.length,
      eligible,
      total,
      cost,
      loc,
      users,
      avgCalRatio: mean(calRatios),
      avgAiRatio: mean(aiRatios),
    }
  }, [rows])

  const eligiblePct = agg.total > 0 ? (agg.eligible / agg.total) * 100 : 0

  return (
    <div className="glass rounded-2xl p-5 md:p-6 space-y-4">
      <div className="flex items-center justify-between gap-3">
        <h2 className="text-sm font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide">项目提效概览</h2>
        <span className="text-xs text-gray-400 dark:text-gray-500 text-right">
          项目=一组需求(branch) · 纯 Need 口径（守恒聚合、只计干净需求）
        </span>
      </div>

      {error ? (
        <div className="text-sm text-rose-600 dark:text-rose-400">加载失败：{(error as Error).message}</div>
      ) : isLoading ? (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
          {Array.from({ length: 8 }).map((_, i) => (
            <div key={i} className="skeleton h-20 rounded-2xl" />
          ))}
        </div>
      ) : (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
          <MetricCard label="项目数" value={formatNumber(agg.projectCount)} accent="#0071e3" />
          <MetricCard
            label="合格需求"
            value={formatNumber(agg.eligible)}
            hint={`合格/候选 ${formatNumber(agg.eligible)} / ${formatNumber(agg.total)} · ${eligiblePct.toFixed(1)}%`}
          />
          <MetricCard
            label="平均日历提效比"
            value={formatV2Ratio(agg.avgCalRatio)}
            tip="各项目日历提效比（小数口径）的算术均值"
            tone={agg.avgCalRatio != null && agg.avgCalRatio < 0 ? 'neg' : 'pos'}
          />
          <MetricCard label="平均 AI 占比" value={formatV2Ratio(agg.avgAiRatio)} tip="各项目 AI 代码占比（小数口径）均值" />
          <MetricCard label="费用合计" value={`¥${fmtCost(agg.cost)}`} hint="各项目干净需求费用之和" />
          <MetricCard
            label="生成代码合计"
            value={agg.loc > 0 ? `${formatNumber(agg.loc)} 行` : '-'}
            hint="need_total_loc_net 之和"
          />
          <MetricCard label="贡献者合计" value={formatNumber(agg.users)} hint="各项目贡献者人次之和" />
        </div>
      )}
    </div>
  )
}

/** 聚合态：现有列表/排行（点行下钻进独立详情；壳内选对象则进聚焦态）。org 由上层单独渲染 OrgTree。 */
function AggregateContent({ entity }: { entity: string }) {
  switch (entity) {
    case 'user':
      return <UserList />
    case 'project':
      return <ProjectList />
    case 'repo':
      return <RepoList />
    default:
      return null
  }
}

/**
 * #5 效率·分布 按主体口径区分：每个主体显「自己对象的提效比分布」，不再共用全局 Need 分布。
 *   org  → DistributionOverview（全量需求提效比分布，原样保留 = 组织/公司口径）。
 *   project/repo/user → 各对象提效比直方图（横轴=提效比分档，纵轴=对象个数），口径分流：
 *     project = need_calendar_efficiency_ratio（小数）｜ repo = efficiency_ratio（百分比）｜ user = calendar_ratio（小数）。
 */
function EntityDistribution({ entity, timeRange }: { entity: string; timeRange: [string, string] }) {
  if (entity === 'project') return <ProjectRatioDistribution />
  if (entity === 'repo') return <RepoRatioDistribution timeRange={timeRange} />
  if (entity === 'user') return <UserRatioDistribution timeRange={timeRange} />
  // 组织：保留全量需求分布（= 组织/公司口径的需求分布）+ 口径说明。
  return (
    <div className="space-y-3">
      <p className="text-xs text-gray-500 dark:text-gray-400">
        组织 = 全量需求提效比分布（公司口径，含双口径与数据质量诊断）。按部门拆分排行需后端支持，暂以全量分布呈现。
      </p>
      <DistributionOverview />
    </div>
  )
}

/** 项目分布：各项目 need_calendar_efficiency_ratio（小数口径）分桶。数据现成（useProjectList，与日期无关）。 */
function ProjectRatioDistribution() {
  const { data, isLoading, error } = useProjectList()
  const ratios = useMemo(() => (data?.data ?? []).map((r) => r.need_calendar_efficiency_ratio), [data])
  return (
    <EntityRatioHistogram
      ratios={ratios}
      scale="decimal"
      entityLabel="项目"
      caliberNote="日历提效比 · 小数口径"
      loading={isLoading}
      error={error ? (error as Error).message : null}
    />
  )
}

/** 仓库分布：各仓库 efficiency_ratio（⚠️百分比口径）分桶。pageSize 拉大客户端一次取回（对齐分布页仓库排行）。 */
function RepoRatioDistribution({ timeRange }: { timeRange: [string, string] }) {
  const startDate = formatDateParam(timeRange[0])
  const endDate = formatDateParam(timeRange[1])
  const { data, isLoading, error } = useRepos({ startDate, endDate, page: 1, pageSize: 1000 })
  const ratios = useMemo(() => (data?.data ?? []).map((r) => r.efficiency_ratio), [data])
  return (
    <EntityRatioHistogram
      ratios={ratios}
      scale="percent"
      entityLabel="仓库"
      caliberNote="提效比 · 百分比口径"
      loading={isLoading}
      error={error ? (error as Error).message : null}
    />
  )
}

/** 个人分布：各用户 calendar_ratio（小数口径）分桶。useAllUsers 翻页拉全（对齐分布页用户排行）。 */
function UserRatioDistribution({ timeRange }: { timeRange: [string, string] }) {
  const startDate = formatDateParam(timeRange[0])
  const endDate = formatDateParam(timeRange[1])
  const { data, isLoading, error } = useAllUsers({ startDate, endDate })
  const ratios = useMemo(() => (data ?? []).map((r) => r.calendar_ratio), [data])
  return (
    <EntityRatioHistogram
      ratios={ratios}
      scale="decimal"
      entityLabel="用户"
      caliberNote="日历提效比 · 小数口径"
      loading={isLoading}
      error={error ? (error as Error).message : null}
    />
  )
}

/** 聚焦态：复用现有详情组件（embedded，去掉返回/标题，壳保留面包屑）。org 不走此路（OrgTree 复合视图）。 */
function FocusContent({
  entity,
  object,
  objectLabel,
  timeRange,
}: {
  entity: string
  object: string
  objectLabel: string
  timeRange: [string, string]
}) {
  // objectLabel 目前仅 org 路径使用，org 不走此函数，保留参数签名以备扩展。
  void objectLabel
  switch (entity) {
    case 'user':
      return <UserDetail userIdProp={object} dateRangeProp={timeRange} embedded />
    case 'project':
      return <ProjectDetail projectIdProp={object} embedded />
    case 'repo':
      return <RepoDetail repoAddrProp={object} dateRangeProp={timeRange} embedded />
    default:
      return null
  }
}

function SubTab({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      type="button"
      role="tab"
      aria-selected={active}
      onClick={onClick}
      className={`px-3 py-1.5 rounded-lg text-sm font-medium cursor-pointer transition-colors border-none bg-transparent ${
        active
          ? 'bg-apple-blue/15 text-apple-blue'
          : 'text-gray-500 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white hover:bg-white/50 dark:hover:bg-white/10'
      } focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue`}
    >
      {children}
    </button>
  )
}
