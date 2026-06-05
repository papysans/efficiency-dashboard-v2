// TanStack Query hooks —— 统一 loading/error/缓存。PR0 先做高管大屏/Need 用到的；其余随页面加。
import { useQuery } from '@tanstack/react-query'
import {
  getAllNeedsV2,
  getCommitDetailV2,
  getDashboardSummary,
  getGlobalConfig,
  getNeedDetailV2,
  getNeedsV2,
  getOrgDetailV2,
  getOrgTreeV2,
  getProjectDetail,
  getProjects,
  getRepoBranches,
  getRepoDetailV2,
  getTaskDetailV2,
  getUserDetailV2,
  getUserGroupDetail,
  getUsersV2,
} from './endpoints'
import type { ListParams } from './types'

export function useDashboardSummary(params: { startDate?: string; endDate?: string }) {
  return useQuery({
    queryKey: ['dashboard-summary', params],
    queryFn: () => getDashboardSummary(params),
  })
}

export function useGlobalConfig() {
  return useQuery({
    queryKey: ['global-config'],
    queryFn: () => getGlobalConfig(),
    staleTime: 5 * 60_000,
  })
}

export function useNeeds(params: ListParams) {
  return useQuery({
    queryKey: ['needs', params],
    queryFn: () => getNeedsV2(params),
  })
}

/** Need 详情（needId 由 endpoints 内部 encodeURIComponent）。 */
export function useNeedDetail(needId: string | undefined) {
  return useQuery({
    queryKey: ['need-detail', needId],
    queryFn: () => getNeedDetailV2(needId as string),
    enabled: !!needId,
  })
}

/** 翻页拉全 needs（绕过后端 200 pageSize cap），高管大屏趋势/Top 榜用。 */
export function useAllNeeds(params: ListParams) {
  return useQuery({
    queryKey: ['needs-all', params],
    queryFn: () => getAllNeedsV2(params),
  })
}

export function useUsers(params: { startDate?: string; endDate?: string; pageSize?: number }) {
  return useQuery({
    queryKey: ['users', params],
    queryFn: () => getUsersV2(params),
  })
}

/** 用户详情（userId 由 endpoints 内部 encodeURIComponent；小数口径）。 */
export function useUserDetail(userId: string | undefined, params: { startDate?: string; endDate?: string }) {
  return useQuery({
    queryKey: ['user-detail', userId, params],
    queryFn: () => getUserDetailV2(userId as string, params),
    enabled: !!userId,
  })
}

/** 用户组详情（百分比口径；后端无列表端点，仅 detail by groupId）。 */
export function useUserGroupDetail(groupId: string | undefined, params: { startDate?: string; endDate?: string }) {
  return useQuery({
    queryKey: ['user-group-detail', groupId, params],
    queryFn: () => getUserGroupDetail(groupId as string, params),
    enabled: !!groupId,
  })
}

/** Task 详情（taskId 由 endpoints 内部 encodeURIComponent）。 */
export function useTaskDetail(taskId: string | undefined) {
  return useQuery({
    queryKey: ['task-detail', taskId],
    queryFn: () => getTaskDetailV2(taskId as string),
    enabled: !!taskId,
  })
}

/** Commit 详情（百分比口径；commitId 由 endpoints 内部 encodeURIComponent）。 */
export function useCommitDetail(commitId: string | undefined) {
  return useQuery({
    queryKey: ['commit-detail', commitId],
    queryFn: () => getCommitDetailV2(commitId as string),
    enabled: !!commitId,
  })
}

/** 仓库详情（百分比口径）。repoAddr/repoBranch 调用方已 decode；endpoints 走 query 传参。 */
export function useRepoDetail(params: { repoAddr: string; repoBranch?: string; startDate?: string; endDate?: string }) {
  return useQuery({
    queryKey: ['repo-detail', params],
    queryFn: () => getRepoDetailV2(params),
    enabled: !!params.repoAddr,
  })
}

/** 仓库分支列表（分支切换下拉用）。 */
export function useRepoBranches(repoAddr: string | undefined) {
  return useQuery({
    queryKey: ['repo-branches', repoAddr],
    queryFn: () => getRepoBranches(repoAddr as string),
    enabled: !!repoAddr,
  })
}

/** 组织详情（百分比口径；org_path 由 endpoints 转 snake_case）。orgPath 空时不请求。 */
export function useOrgDetail(params: { orgPath: string; startDate?: string; endDate?: string; granularity?: string }) {
  return useQuery({
    queryKey: ['org-detail', params],
    queryFn: () => getOrgDetailV2(params),
    enabled: !!params.orgPath,
  })
}

/** 组织树（基于 user_org 有数据的层级；与日期无关，长缓存）。 */
export function useOrgTree() {
  return useQuery({
    queryKey: ['org-tree'],
    queryFn: () => getOrgTreeV2(),
    staleTime: 5 * 60_000,
  })
}

/** 项目列表（百分比口径；无分页，客户端筛选/排序）。 */
export function useProjectList(params?: { order?: string }) {
  return useQuery({
    queryKey: ['project-list', params],
    queryFn: () => getProjects(params),
  })
}

/** 项目详情（百分比口径；含 commits/tasks/members + 8 管理操作）。 */
export function useProjectDetail(projectId: string | undefined) {
  return useQuery({
    queryKey: ['project-detail', projectId],
    queryFn: () => getProjectDetail(projectId as string),
    enabled: !!projectId,
  })
}
