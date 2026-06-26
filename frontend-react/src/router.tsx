import type { ComponentType } from 'react'
import { createBrowserRouter, Navigate, useLocation, useParams } from 'react-router'
import AppShell from '@/components/layout/AppShell'
import DimensionEntityLayout from '@/components/layout/DimensionEntityLayout'
import EfficiencyDimension from '@/pages/dimensions/EfficiencyDimension'
import UsageLayout from '@/pages/dimensions/usage/UsageLayout'
import UsageKanban from '@/pages/dimensions/usage/UsageKanban'
import CostKanban from '@/pages/dimensions/cost/CostKanban'
import ContributionDimension from '@/pages/dimensions/ContributionDimension'
import {
  DEFAULT_DIMENSION,
  DIMENSIONS,
  ENTITIES,
  type Dimension,
  type Entity,
} from '@/components/layout/matrix'
import Overview from '@/pages/Overview'
import Placeholder from '@/pages/Placeholder'
import NeedList from '@/pages/needs/NeedList'
import NeedDetail from '@/pages/needs/NeedDetail'
import TaskList from '@/pages/tasks/TaskList'
import TaskDetail from '@/pages/tasks/TaskDetail'
// 注：UserList/RepoList/OrgTree/ProjectList 不在 router 直接挂载——已被维度组件内嵌为聚合态内容。
import UserDetail from '@/pages/users/UserDetail'
import UserGroupDetail from '@/pages/users/UserGroupDetail'
import RepoDetail from '@/pages/repos/RepoDetail'
import CommitList from '@/pages/commits/CommitList'
import CommitDetail from '@/pages/commits/CommitDetail'
import WorkDirDetail from '@/pages/workdir/WorkDirDetail'
import ProjectDetail from '@/pages/projects/ProjectDetail'
import PlatformOverview from '@/pages/platform/PlatformOverview'
import PlatformHealth from '@/pages/platform/PlatformHealth'
import RealtimeReport from '@/pages/platform/RealtimeReport'
import RealtimeQuery from '@/pages/platform/RealtimeQuery'
import Pricing from '@/pages/settings/Pricing'
import Datasources from '@/pages/settings/Datasources'
import SyncTasks from '@/pages/settings/SyncTasks'
import SystemConfig from '@/pages/settings/SystemConfig'

// 主体×维度矩阵 IA（维度优先）。一级导航选维度（使用/效率/成本/贡献），
// 进维度后页内一排主体 Tab（组织/个人/项目/仓库）切「看谁」→ /:dim/:entity。
// 4 维度共用 DimensionEntityLayout 壳；entity 走 URL param（壳层 useParams 读 + 守卫脏值）。
//
// 详情叶子页（/user/:userId、/project/:projectId、/repo/:repoAddr… 等）保留在顶层，与维度一级段
// （usage/efficiency/cost/contribution）首段不冲突；下钻深链原样指向这些叶子页，不随轴翻转改动。
//
// 旧「主体优先」链接（/org、/user/efficiency …）+ 旧 -v2 链接全量重定向（保留 query/search），不能 404。

const DIM_COMPONENT: Record<Dimension, ComponentType> = {
  usage: UsageKanban,
  efficiency: EfficiencyDimension,
  cost: CostKanban,
  contribution: ContributionDimension,
}

/**
 * 旧 opencode need 链接重定向（保留 query/search）。
 * needId 走 path 时按 encodeURIComponent（need_id 可能含斜杠/特殊字符）。
 */
function RedirectWithQuery({ to }: { to: string }) {
  const { search } = useLocation()
  const { needId } = useParams<{ needId?: string }>()
  const target = needId != null ? `/needs/${encodeURIComponent(needId)}` : to
  return <Navigate to={`${target}${search}`} replace />
}

/** 简单重定向（保留 query/search）。 */
function SimpleRedirect({ to }: { to: string }) {
  const { search } = useLocation()
  return <Navigate to={`${to}${search}`} replace />
}

/** 旧「主体优先」URL → 新「维度优先」URL（/:entity/:dim → /:dim/:entity），保留 query/search。 */
function FlipRedirect({ dim, entity }: { dim: Dimension; entity: Entity }) {
  const { search } = useLocation()
  return <Navigate to={`/${dim}/${entity}${search}`} replace />
}

/** 旧 /distribution-v2 → 「效率」维度「分布」次级 tab（org 主体；保留原 query 如 caliber/bins）。 */
function DistributionToEfficiency() {
  const { search } = useLocation()
  const sp = new URLSearchParams(search)
  sp.set('sub', 'distribution')
  return <Navigate to={`/efficiency/org?${sp.toString()}`} replace />
}

// 一个维度下的主体子路由（4 维度共用 DimensionEntityLayout 壳；entity 走 param，壳内按 entity 分发口径/数据源）。
//   使用 = UsageKanban（部门树·视角切换统一页：部门聚合 / 本部门人员 / 个人详情；project/repo 已下线，由 UsageLayout 重定向到 org）。
//   效率 = EfficiencyDimension（时间线→KPI→排行/明细，聚合↔聚焦两态，分布并入）。
//   成本 = CostKanban（部门树·视角切换：部门聚合 / 子部门对比 / 用户成本，对接后端 /cost/* 接口）。
//   贡献 = ContributionDimension（全主体看板派生：合并需求/代码行/提交/贡献者，零平台请求）。
// 不带 entity（裸 /usage）或脏值（/usage/garbage）→ 由 DimensionEntityLayout 的守卫统一重定向到默认主体（组织），保留 query。
function dimensionRoute(dim: Dimension) {
  const Comp = DIM_COMPONENT[dim]
  return {
    path: dim,
    element: <DimensionEntityLayout dim={dim} />,
    children: [
      { path: ':entity', element: <Comp /> },
    ],
  }
}

// 旧「主体优先」重定向路由（静态段，优先级高于 /user/:userId 等动态详情段，故 /user/efficiency 命中重定向而非详情）。
//   裸主体 /org → /usage/org（默认维度）；/:entity/:dim → /:dim/:entity。
const legacyEntityRedirects = ENTITIES.flatMap(({ key: entity }) => [
  { path: entity, element: <FlipRedirect dim={DEFAULT_DIMENSION} entity={entity} /> },
  ...DIMENSIONS.map(({ key: dim }) => ({
    path: `${entity}/${dim}`,
    element: <FlipRedirect dim={dim} entity={entity} />,
  })),
])

export const router = createBrowserRouter([
  {
    path: '/',
    element: <AppShell />,
    children: [
      { index: true, element: <Overview /> },

      // ---- 维度×主体矩阵（4 维度共用壳） ----
      // usage 维度：IA 改为「部门树·视角切换」统一页，独立 UsageLayout（不走 4 维度共用壳 DimensionEntityLayout）。
      // 保留 /usage/:entity 路径形状以兼容旧链（/usage/org、/usage/user、org-tree-v2 等）与 AppShell.entityFromPath。
      // project/repo 主体在 usage 已下线，由 UsageLayout 守卫重定向到 /usage/org。
      {
        path: 'usage',
        element: <UsageLayout />,
        children: [
          { path: ':entity', element: <UsageKanban /> },
          { index: true, element: <UsageKanban /> },
        ],
      },
      dimensionRoute('efficiency'),
      // cost 维度：独立「部门树·视角切换」页（与 usage 同构），对接后端 10 个 /cost/* 接口。
      // 保留 /cost/:entity 路径形状以兼容 AppShell 顶部导航生成的 /cost/org（CostKanban 不读 entity）。
      {
        path: 'cost',
        children: [
          { index: true, element: <CostKanban /> },
          { path: ':entity', element: <CostKanban /> },
        ],
      },
      dimensionRoute('contribution'),

      // ---- 需求（保留顶级，几乎不动） ----
      { path: 'needs-v2', element: <NeedList /> },
      { path: 'needs/:needId', element: <NeedDetail /> },

      // ---- 详情/叶子页（从主体表格下钻进入，配返回按钮；保留原路径不动，不随轴翻转改） ----
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
      // 分布：已并入「效率」维度子视图（efficiency/org 内「分布」次级 tab）。旧 /distribution-v2 → 重定向（保留 query）。
      { path: 'distribution-v2', element: <DistributionToEfficiency /> },

      // 设置区（含平台运维三级页）。平台原始监控页已从顶部「平台」一级入口下沉到设置下。
      { path: 'settings/pricing', element: <Pricing /> },
      { path: 'settings/datasources', element: <Datasources /> },
      { path: 'settings/sync', element: <SyncTasks /> },
      { path: 'settings/config', element: <SystemConfig /> },
      { path: 'settings/platform/overview', element: <PlatformOverview /> },
      { path: 'settings/platform/health', element: <PlatformHealth /> },
      { path: 'settings/platform/realtime', element: <RealtimeReport /> },
      { path: 'settings/platform/realtime/query', element: <RealtimeQuery /> },

      // 旧 /platform/* → /settings/platform/*（保留 query/search，旧链接/书签不 404）
      { path: 'platform/overview', element: <SimpleRedirect to="/settings/platform/overview" /> },
      { path: 'platform/realtime', element: <SimpleRedirect to="/settings/platform/realtime" /> },
      { path: 'platform/realtime/query', element: <SimpleRedirect to="/settings/platform/realtime/query" /> },

      // ---- 旧「主体优先」(/org、/user/efficiency …) → 新「维度优先」(保留 query/search，不 404) ----
      ...legacyEntityRedirects,

      // ---- 旧 -v2 路由 → 新维度优先（默认维度=使用，保留 query/search） ----
      { path: 'user-v2', element: <SimpleRedirect to="/usage/user" /> },
      { path: 'org-tree-v2', element: <SimpleRedirect to="/usage/org" /> },
      { path: 'project-v2', element: <SimpleRedirect to="/usage/project" /> },
      { path: 'repo-v2', element: <SimpleRedirect to="/usage/repo" /> },

      // 旧 opencode 链接重定向（保留 query/search）
      { path: 'cloud/kanban', element: <RedirectWithQuery to="/needs-v2" /> },
      { path: 'kanban/need', element: <RedirectWithQuery to="/needs-v2" /> },
      { path: 'kanban/need/:needId', element: <RedirectWithQuery to="/needs-v2" /> },

      { path: '*', element: <Placeholder title="页面不存在" /> },
    ],
  },
], { basename: '/kanban' })
