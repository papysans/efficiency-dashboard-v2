import { createBrowserRouter, Navigate, useLocation, useParams } from 'react-router'
import AppShell from '@/components/layout/AppShell'
import Overview from '@/pages/Overview'
import DistributionOverview from '@/pages/distribution/DistributionOverview'
import Placeholder from '@/pages/Placeholder'
import NeedList from '@/pages/needs/NeedList'
import NeedDetail from '@/pages/needs/NeedDetail'
import TaskList from '@/pages/tasks/TaskList'
import TaskDetail from '@/pages/tasks/TaskDetail'
import UserList from '@/pages/users/UserList'
import UserDetail from '@/pages/users/UserDetail'
import UserGroupDetail from '@/pages/users/UserGroupDetail'
import RepoList from '@/pages/repos/RepoList'
import RepoDetail from '@/pages/repos/RepoDetail'
import OrgTree from '@/pages/orgs/OrgTree'
import CommitList from '@/pages/commits/CommitList'
import CommitDetail from '@/pages/commits/CommitDetail'
import WorkDirDetail from '@/pages/workdir/WorkDirDetail'
import ProjectList from '@/pages/projects/ProjectList'
import ProjectDetail from '@/pages/projects/ProjectDetail'
import PlatformOverview from '@/pages/platform/PlatformOverview'
import RealtimeReport from '@/pages/platform/RealtimeReport'
import RealtimeQuery from '@/pages/platform/RealtimeQuery'
import Pricing from '@/pages/settings/Pricing'
import Datasources from '@/pages/settings/Datasources'
import SyncTasks from '@/pages/settings/SyncTasks'
import SystemConfig from '@/pages/settings/SystemConfig'

// 路由表对齐 Vue frontend/src/router/index.js（见 research/api-contract.md §6）。
// PR0：总览页真实落地，其余 24 路由先用 Placeholder 占位（可点不 404），后续 PR 替换。
// 旧路由重定向：PR4c 完善为「保留 query/search + /kanban/need/:needId」三条精确重定向。

/**
 * 旧路由重定向（保留 query/search，兼容旧 opencode 链接）。
 * 对齐 Vue redirect: to => ({ path, query: to.query })（api-contract.md §6）。
 * needId 走 path 时按 encodeURIComponent（need_id 可能含斜杠/特殊字符）。
 */
function RedirectWithQuery({ to }: { to: string }) {
  const { search } = useLocation()
  const { needId } = useParams<{ needId?: string }>()
  const target = needId != null ? `/needs/${encodeURIComponent(needId)}` : to
  return <Navigate to={`${target}${search}`} replace />
}

export const router = createBrowserRouter([
  {
    path: '/',
    element: <AppShell />,
    children: [
      { index: true, element: <Overview /> },
      { path: 'distribution-v2', element: <DistributionOverview /> },

      { path: 'needs-v2', element: <NeedList /> },
      { path: 'needs/:needId', element: <NeedDetail /> },
      { path: 'task-v2', element: <TaskList /> },
      { path: 'task/:taskId', element: <TaskDetail /> },
      { path: 'user-v2', element: <UserList /> },
      { path: 'user/group/:groupId', element: <UserGroupDetail /> },
      { path: 'user/:userId', element: <UserDetail /> },
      { path: 'repo-v2', element: <RepoList /> },
      { path: 'repo/:repoAddr/:repoBranch?', element: <RepoDetail /> },
      { path: 'org-tree-v2', element: <OrgTree /> },
      { path: 'project-v2', element: <ProjectList /> },
      { path: 'project/:projectId', element: <ProjectDetail /> },
      { path: 'commit-v2', element: <CommitList /> },
      { path: 'commit/:commitId', element: <CommitDetail /> },
      { path: 'workdir/:workDirId', element: <WorkDirDetail /> },

      // 平台客观指标（chat-indicator-statistics 代理）+ 设置区（占位骨架，T3/T4 填充）
      { path: 'platform/overview', element: <PlatformOverview /> },
      { path: 'platform/realtime', element: <RealtimeReport /> },
      { path: 'platform/realtime/query', element: <RealtimeQuery /> },
      { path: 'settings/pricing', element: <Pricing /> },
      { path: 'settings/datasources', element: <Datasources /> },
      { path: 'settings/sync', element: <SyncTasks /> },
      { path: 'settings/config', element: <SystemConfig /> },

      // 旧路由重定向（保留 query/search，兼容旧 opencode 链接，api-contract.md §6）
      { path: 'cloud/kanban', element: <RedirectWithQuery to="/needs-v2" /> },
      { path: 'kanban/need', element: <RedirectWithQuery to="/needs-v2" /> },
      { path: 'kanban/need/:needId', element: <RedirectWithQuery to="/needs-v2" /> },

      { path: '*', element: <Placeholder title="页面不存在" /> },
    ],
  },
], { basename: '/kanban' })
