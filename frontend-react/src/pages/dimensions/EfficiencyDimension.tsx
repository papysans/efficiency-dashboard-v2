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
import { useEfficiencyV2 } from '@/api/queries'
import { useViewState } from '@/store/viewState'
import { formatDateParam } from '@/lib/date'
import { useEntityFocus } from '@/components/layout/EntityDimensionLayout'
import { DimensionTrend } from '@/components/executive/DimensionTrend'
import OrgTree from '@/pages/orgs/OrgTree'
import UserList from '@/pages/users/UserList'
import UserDetail from '@/pages/users/UserDetail'
import ProjectList from '@/pages/projects/ProjectList'
import ProjectDetail from '@/pages/projects/ProjectDetail'
import RepoList from '@/pages/repos/RepoList'
import RepoDetail from '@/pages/repos/RepoDetail'
import DistributionOverview from '@/pages/distribution/DistributionOverview'

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
        <DistributionOverview />
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

  if (entity === 'project' || entity === 'repo') {
    return (
      <DimensionTrend
        rows={[]}
        unavailable
        title="提效趋势"
        subtitle={`${entity === 'project' ? '项目' : '仓库'}口径`}
        unavailableNote="项目/仓库暂无按周的提效时间线（周表按 用户×周 聚合，无此维度）。下方为看板派生口径的提效与明细。"
      />
    )
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
