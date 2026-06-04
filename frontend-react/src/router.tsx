import { createBrowserRouter, Navigate } from 'react-router'
import AppShell from '@/components/layout/AppShell'
import Overview from '@/pages/Overview'
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
import OrgList from '@/pages/orgs/OrgList'
import OrgDetail from '@/pages/orgs/OrgDetail'
import CommitList from '@/pages/commits/CommitList'
import CommitDetail from '@/pages/commits/CommitDetail'
import WorkDirDetail from '@/pages/workdir/WorkDirDetail'
import ProjectList from '@/pages/projects/ProjectList'
import ProjectDetail from '@/pages/projects/ProjectDetail'

// 路由表对齐 Vue frontend/src/router/index.js（见 research/api-contract.md §6）。
// PR0：总览页真实落地，其余 24 路由先用 Placeholder 占位（可点不 404），后续 PR 替换。
// 旧路由重定向：PR4 完善为「保留 query + /kanban/need/:needId」三条精确重定向。
export const router = createBrowserRouter([
  {
    path: '/',
    element: <AppShell />,
    children: [
      { index: true, element: <Overview /> },

      { path: 'needs-v2', element: <NeedList /> },
      { path: 'needs/:needId', element: <NeedDetail /> },
      { path: 'task-v2', element: <TaskList /> },
      { path: 'task/:taskId', element: <TaskDetail /> },
      { path: 'user-v2', element: <UserList /> },
      { path: 'user/group/:groupId', element: <UserGroupDetail /> },
      { path: 'user/:userId', element: <UserDetail /> },
      { path: 'repo-v2', element: <RepoList /> },
      { path: 'repo/:repoAddr/:repoBranch?', element: <RepoDetail /> },
      { path: 'org-v2', element: <OrgList /> },
      { path: 'org/:orgPath', element: <OrgDetail /> },
      { path: 'project-v2', element: <ProjectList /> },
      { path: 'project/:projectId', element: <ProjectDetail /> },
      { path: 'commit-v2', element: <CommitList /> },
      { path: 'commit/:commitId', element: <CommitDetail /> },
      { path: 'workdir/:workDirId', element: <WorkDirDetail /> },

      // 旧路由重定向（PR4 完善 query 保留）
      { path: 'cloud/kanban', element: <Navigate to="/needs-v2" replace /> },
      { path: 'kanban/need', element: <Navigate to="/needs-v2" replace /> },

      { path: '*', element: <Placeholder title="页面不存在" /> },
    ],
  },
])
