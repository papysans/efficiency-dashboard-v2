// TanStack Query hooks —— 统一 loading/error/缓存。PR0 先做高管大屏/Need 用到的；其余随页面加。
import { useQuery } from '@tanstack/react-query'
import {
  getAllNeedsV2,
  getDashboardSummary,
  getGlobalConfig,
  getNeedDetailV2,
  getNeedsV2,
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

export function useUsers(params: { startDate?: string; endDate?: string }) {
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
