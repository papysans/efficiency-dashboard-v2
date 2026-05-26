import { createRouter, createWebHistory } from 'vue-router'

const routes = [
  { path: '/', name: 'Home', component: () => import('@/views/Home.vue') },
  { path: '/repo-v2', name: 'RepoV2', component: () => import('@/views/RepoViewV2.vue') },
  { path: '/repo/:repoAddr/:repoBranch?', name: 'RepoDetail', component: () => import('@/views/RepoDetailV2.vue') },
  { path: '/user-v2', name: 'UserV2', component: () => import('@/views/UserViewV2.vue') },
  { path: '/user/group/:groupId', name: 'UserGroupDetail', component: () => import('@/views/UserGroupDetail.vue') },
  { path: '/user/:userId', name: 'UserDetail', component: () => import('@/views/UserDetailV2.vue') },
  { path: '/org-v2', name: 'OrgV2', component: () => import('@/views/OrgViewV2.vue') },
  { path: '/org/:orgPath', name: 'OrgDetail', component: () => import('@/views/OrgDetailV2.vue') },
  { path: '/needs-v2', name: 'NeedsV2', component: () => import('@/views/NeedViewV2.vue') },
  { path: '/needs/:needId', name: 'NeedDetailV2', component: () => import('@/views/NeedDetailV2.vue') },
  { path: '/task-v2', name: 'TaskV2', component: () => import('@/views/TaskViewV2.vue') },
  { path: '/task/:taskId', name: 'TaskDetail', component: () => import('@/views/TaskDetailV2.vue') },
  { path: '/commit-v2', name: 'CommitV2', component: () => import('@/views/CommitViewV2.vue') },
  { path: '/commit/:commitId', name: 'CommitDetail', component: () => import('@/views/CommitDetailV2.vue') },
  { path: '/workdir/:workDirId', name: 'WorkDirDetail', component: () => import('@/views/WorkDirDetailV2.vue') },
  { path: '/project-v2', name: 'ProjectView', component: () => import('@/views/ProjectViewV2.vue') },
  { path: '/project/:projectId', name: 'ProjectDetail', component: () => import('@/views/ProjectDetailV2.vue') },
  { path: '/cloud/kanban', redirect: to => ({ path: '/needs-v2', query: to.query }) },
  { path: '/kanban/need', redirect: to => ({ path: '/needs-v2', query: to.query }) },
  { path: '/kanban/need/:needId', redirect: to => ({ path: `/needs/${encodeURIComponent(to.params.needId)}`, query: to.query }) },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

export default router
