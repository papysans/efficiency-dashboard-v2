// TanStack Query hooks —— 统一 loading/error/缓存。PR0 先做高管大屏/Need 用到的；其余随页面加。
import { useQuery } from '@tanstack/react-query'
import { getAllNeedsV2, getDashboardSummary, getGlobalConfig, getNeedsV2, getUsersV2 } from './endpoints'
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
