import { createBrowserRouter, Navigate, useLocation, useParams } from 'react-router'
import AppShell from '@/components/layout/AppShell'
import EntityDimensionLayout from '@/components/layout/EntityDimensionLayout'
import EfficiencyDimension from '@/pages/dimensions/EfficiencyDimension'
import UsageDimension from '@/pages/dimensions/UsageDimension'
import QualityDimension from '@/pages/dimensions/QualityDimension'
import CostDimension from '@/pages/dimensions/CostDimension'
import ContributionDimension from '@/pages/dimensions/ContributionDimension'
import { QualityComingSoon } from '@/components/ui/QualityComingSoon'
import type { Entity } from '@/components/ui/DimensionTabs'
import Overview from '@/pages/Overview'
import Placeholder from '@/pages/Placeholder'
import NeedList from '@/pages/needs/NeedList'
import NeedDetail from '@/pages/needs/NeedDetail'
import TaskList from '@/pages/tasks/TaskList'
import TaskDetail from '@/pages/tasks/TaskDetail'
// 注：UserList/RepoList/OrgTree/ProjectList 不再在 router 直接挂载——已被 EfficiencyDimension 内嵌为聚合态内容。
import UserDetail from '@/pages/users/UserDetail'
import UserGroupDetail from '@/pages/users/UserGroupDetail'
import RepoDetail from '@/pages/repos/RepoDetail'
import CommitList from '@/pages/commits/CommitList'
import CommitDetail from '@/pages/commits/CommitDetail'
import WorkDirDetail from '@/pages/workdir/WorkDirDetail'
import ProjectDetail from '@/pages/projects/ProjectDetail'
import PlatformOverview from '@/pages/platform/PlatformOverview'
import RealtimeReport from '@/pages/platform/RealtimeReport'
import RealtimeQuery from '@/pages/platform/RealtimeQuery'
import Pricing from '@/pages/settings/Pricing'
import Datasources from '@/pages/settings/Datasources'
import SyncTasks from '@/pages/settings/SyncTasks'
import SystemConfig from '@/pages/settings/SystemConfig'

// 主体×维度矩阵 IA（A 主体优先）。一级导航选主体（组织/个人/项目/仓库），
// 进下钻后页内一排维度 Tab（使用/质量/效率/成本/贡献）切「看什么」。
// 4 下钻共用 EntityDimensionLayout 壳，维度走「静态段」子路由（usage|quality|efficiency|cost|contribution）。
//
// 路由冲突处理：维度段用「静态字符串」而非动态参数（dim 是固定枚举），所以与详情叶子
// /user/:userId、/user/group/:groupId 不冲突——React Router 静态段优先级高于动态段，
// /user/efficiency 命中维度路由，/user/<uuid> 命中 :userId 详情。详情叶子全部保留原样。
//
// 旧 -v2 链接全量重定向（保留 query/search），不能 404。

/**
 * 旧路由重定向（保留 query/search，兼容旧 opencode 链接）。
 * needId 走 path 时按 encodeURIComponent（need_id 可能含斜杠/特殊字符）。
 */
function RedirectWithQuery({ to }: { to: string }) {
  const { search } = useLocation()
  const { needId } = useParams<{ needId?: string }>()
  const target = needId != null ? `/needs/${encodeURIComponent(needId)}` : to
  return <Navigate to={`${target}${search}`} replace />
}

/** 简单重定向（保留 query/search），用于 -v2 → 新主体路由。 */
function SimpleRedirect({ to }: { to: string }) {
  const { search } = useLocation()
  return <Navigate to={`${to}${search}`} replace />
}

/** index → usage 重定向，保留 query/search（深链 /user?object=X 不丢聚焦对象）。
 *  默认落「使用」Tab（第一个维度，最符合直觉；project/repo 的 usage 是看板派生有内容，org/user 的 usage 走平台）。 */
function IndexToUsage() {
  const { search } = useLocation()
  return <Navigate to={`usage${search}`} replace />
}

/** 旧 /distribution-v2 → 「效率」维度的「分布」次级 tab（org 主体；保留原 query 如 caliber/bins）。 */
function DistributionToEfficiency() {
  const { search } = useLocation()
  const sp = new URLSearchParams(search)
  sp.set('sub', 'distribution')
  return <Navigate to={`/org/efficiency?${sp.toString()}`} replace />
}

// 一个主体下的 5 维度子路由（4 主体共用同一组维度组件，组件内部按 entity 分支自管口径/数据源）。
//   效率 = EfficiencyDimension（时间线→KPI→排行/明细，聚合↔聚焦两态，分布并入）。
//   使用 = UsageDimension（user→平台金源 / org→平台部门聚合 / project,repo→看板派生）。
//   成本 = CostDimension（user,org→平台AI花费‖人天双卡 / project,repo→看板费用单卡）。
//   贡献 = ContributionDimension（全主体看板派生：合并需求/代码行/提交/贡献者，零平台请求）。
//   质量 = AI服务健康度，只有平台错误率口径 → user/org 给 QualityDimension；project/repo 无该口径
//     → QualityComingSoon 占位（且 DimensionTabs 把这俩的质量 Tab 灰显，不可点）。
// 不带 dim → 默认重定向到 usage（第一个维度「使用」，保留 query）。
function entityRoute(entity: Entity) {
  // 质量维度：仅 user/org 有平台 AI 服务错误率口径；project/repo 无 → 建设中占位。
  const hasPlatformQuality = entity === 'user' || entity === 'org'
  return {
    path: entity,
    element: <EntityDimensionLayout entity={entity} />,
    children: [
      { index: true, element: <IndexToUsage /> },
      { path: 'usage', element: <UsageDimension /> },
      { path: 'quality', element: hasPlatformQuality ? <QualityDimension /> : <QualityComingSoon /> },
      { path: 'efficiency', element: <EfficiencyDimension /> },
      { path: 'cost', element: <CostDimension /> },
      { path: 'contribution', element: <ContributionDimension /> },
    ],
  }
}

export const router = createBrowserRouter([
  {
    path: '/',
    element: <AppShell />,
    children: [
      { index: true, element: <Overview /> },

      // ---- 主体×维度矩阵（4 下钻共用壳） ----
      entityRoute('org'),
      entityRoute('user'),
      entityRoute('project'),
      entityRoute('repo'),

      // ---- 需求（保留顶级，几乎不动） ----
      { path: 'needs-v2', element: <NeedList /> },
      { path: 'needs/:needId', element: <NeedDetail /> },

      // ---- 详情/叶子页（从主体页表格下钻进入，配面包屑；保留原路径不动） ----
      // 注意：/user/group 与 /user/:userId 是静态段优先，与上面的维度静态段互不冲突。
      { path: 'user/group/:groupId', element: <UserGroupDetail /> },
      { path: 'user/:userId', element: <UserDetail /> },
      { path: 'project/:projectId', element: <ProjectDetail /> },
      { path: 'repo/:repoAddr/:repoBranch?', element: <RepoDetail /> },
      { path: 'task/:taskId', element: <TaskDetail /> },
      { path: 'commit/:commitId', element: <CommitDetail /> },
      { path: 'workdir/:workDirId', element: <WorkDirDetail /> },

      // ---- 降级为下钻明细的列表（导航不再占顶部，路由保留可访问） ----
      { path: 'task-v2', element: <TaskList /> },
      { path: 'commit-v2', element: <CommitList /> },
      // 分布：已并入「效率」维度子视图（org 效率页内「分布」次级 tab）。旧 /distribution-v2 → 重定向（保留 query）。
      { path: 'distribution-v2', element: <DistributionToEfficiency /> },

      // 设置区（含平台运维三级页）。平台原始监控页已从顶部「平台」一级入口下沉到设置下，
      // 渲染在 SettingsLayout 壳内（组件自身 wrap SettingsLayout），与价格/数据源同壳。
      { path: 'settings/pricing', element: <Pricing /> },
      { path: 'settings/datasources', element: <Datasources /> },
      { path: 'settings/sync', element: <SyncTasks /> },
      { path: 'settings/config', element: <SystemConfig /> },
      { path: 'settings/platform/overview', element: <PlatformOverview /> },
      { path: 'settings/platform/realtime', element: <RealtimeReport /> },
      { path: 'settings/platform/realtime/query', element: <RealtimeQuery /> },

      // 旧 /platform/* → /settings/platform/*（保留 query/search，旧链接/书签不 404）
      { path: 'platform/overview', element: <SimpleRedirect to="/settings/platform/overview" /> },
      { path: 'platform/realtime', element: <SimpleRedirect to="/settings/platform/realtime" /> },
      { path: 'platform/realtime/query', element: <SimpleRedirect to="/settings/platform/realtime/query" /> },

      // ---- 旧 -v2 路由 → 新主体路由（保留 query/search，不 404） ----
      { path: 'user-v2', element: <SimpleRedirect to="/user" /> },
      { path: 'org-tree-v2', element: <SimpleRedirect to="/org" /> },
      { path: 'project-v2', element: <SimpleRedirect to="/project" /> },
      { path: 'repo-v2', element: <SimpleRedirect to="/repo" /> },

      // 旧 opencode 链接重定向（保留 query/search）
      { path: 'cloud/kanban', element: <RedirectWithQuery to="/needs-v2" /> },
      { path: 'kanban/need', element: <RedirectWithQuery to="/needs-v2" /> },
      { path: 'kanban/need/:needId', element: <RedirectWithQuery to="/needs-v2" /> },

      { path: '*', element: <Placeholder title="页面不存在" /> },
    ],
  },
], { basename: '/kanban' })
