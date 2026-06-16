// TanStack Query hooks —— 统一 loading/error/缓存。PR0 先做高管大屏/Need 用到的；其余随页面加。
import { useQuery } from '@tanstack/react-query'
import {
  chatStats,
  getAllNeedsV2,
  getAllUsersV2,
  getCommitDetailV2,
  getDashboardSummary,
  getDashboardTrends,
  getGlobalConfig,
  getDeptRankingV2,
  getDeptTreeV2,
  getNeedDetailV2,
  getProjectDetail,
  getProjectNeeds,
  getProjects,
  getRepoBranches,
  getRepoDetailV2,
  getReposV2,
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

/** 首页 4 维周趋势 + 环比（sparkline / 环比箭头用）。 */
export function useDashboardTrends(params: { startDate?: string; endDate?: string }) {
  return useQuery({
    queryKey: ['dashboard-trends', params],
    queryFn: () => getDashboardTrends(params),
  })
}

export function useGlobalConfig() {
  return useQuery({
    queryKey: ['global-config'],
    queryFn: () => getGlobalConfig(),
    staleTime: 5 * 60_000,
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

/** 翻页拉全 users（绕过服务端切片截断），分布页用户 Top 排行用。 */
export function useAllUsers(params: { startDate?: string; endDate?: string }) {
  return useQuery({
    queryKey: ['users-all', params],
    queryFn: () => getAllUsersV2(params),
  })
}

/** 仓库列表（⚠️ 百分比口径 efficiency_ratio）。分布页仓库 Top 排行用，pageSize 拉大客户端排序。 */
export function useRepos(params: ListParams) {
  return useQuery({
    queryKey: ['repos', params],
    queryFn: () => getReposV2(params),
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

/** 组织树（dept-sync 权威全量树；与日期无关，长缓存）。 */
export function useDeptTree() {
  return useQuery({
    queryKey: ['dept-tree'],
    queryFn: () => getDeptTreeV2(),
    staleTime: 5 * 60_000,
  })
}

/** 部门排行（一次聚合）：parentDeptId 的各直接子部门整棵子树汇总，供首页部门 PK 一次性消费。 */
export function useDeptRanking(params: { parentDeptId?: string; startDate?: string; endDate?: string }) {
  return useQuery({
    queryKey: ['dept-ranking', params],
    queryFn: () => getDeptRankingV2(params),
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

/** 项目候选池 Need 列表（小数口径；供按 branch 挑选干净样本）。 */
export function useProjectNeeds(projectId: string | undefined) {
  return useQuery({
    queryKey: ['project-needs', projectId],
    queryFn: () => getProjectNeeds(projectId as string),
    enabled: !!projectId,
  })
}

// ---- Chat Stats（/api/v2/chat/* 代理；mutation 类按现有惯例由页面直接调 endpoints） ----

/**
 * 实时态势聚合。服务端 10 秒限频 + 直查源库较慢，故：
 * 不自动轮询（refetchInterval 关）、不窗口聚焦重拉、失败不重试（限频错误重试只会继续 400）。
 * 页面用「手动刷新按钮 + 10s 倒计时」触发 refetch（设计 §2.2）。
 */
export function useChatRealtime(
  params: { range: '30m' | '1h' | '3h'; datasource_id?: string },
  enabled = true,
) {
  return useQuery({
    queryKey: ['chat-realtime', params],
    queryFn: () => chatStats.getRealtime(params),
    enabled,
    refetchInterval: false,
    refetchOnWindowFocus: false,
    retry: false,
    staleTime: 10_000,
  })
}

/** 模型价格列表（设置页；增删改后由页面 invalidate ['chat-pricing']）。 */
export function useChatPricing(enabled = true) {
  return useQuery({
    queryKey: ['chat-pricing'],
    queryFn: () => chatStats.listPricing(),
    enabled,
  })
}

/** 数据源列表（设置页）。 */
export function useChatDatasources(enabled = true) {
  return useQuery({
    queryKey: ['chat-datasources'],
    queryFn: () => chatStats.listDatasources(),
    enabled,
  })
}

/** 同步任务列表（设置页；页面可对 running 任务自行加 refetchInterval）。 */
export function useChatSyncTasks(enabled = true) {
  return useQuery({
    queryKey: ['chat-sync-tasks'],
    queryFn: () => chatStats.listSyncTasks(),
    enabled,
  })
}

/** 系统配置 KV（币种/汇率）。 */
export function useChatSystemConfig(enabled = true) {
  return useQuery({
    queryKey: ['chat-system-config'],
    queryFn: () => chatStats.getConfig(),
    enabled,
  })
}
