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
      { path: 'repo-v2', element: <Placeholder title="仓库列表" /> },
      { path: 'repo/:repoAddr/:repoBranch?', element: <Placeholder title="仓库详情" /> },
      { path: 'org-v2', element: <Placeholder title="组织列表" /> },
      { path: 'org/:orgPath', element: <Placeholder title="组织详情" /> },
      { path: 'project-v2', element: <Placeholder title="项目列表" /> },
      { path: 'project/:projectId', element: <Placeholder title="项目详情" /> },
      { path: 'commit-v2', element: <Placeholder title="提交列表" /> },
      { path: 'commit/:commitId', element: <Placeholder title="提交详情" /> },
      { path: 'workdir/:workDirId', element: <Placeholder title="工作目录详情" /> },

      // 旧路由重定向（PR4 完善 query 保留）
      { path: 'cloud/kanban', element: <Navigate to="/needs-v2" replace /> },
      { path: 'kanban/need', element: <Navigate to="/needs-v2" replace /> },

      { path: '*', element: <Placeholder title="页面不存在" /> },
    ],
  },
])
