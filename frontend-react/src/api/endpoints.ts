// typed API 端点 —— 只对齐后端真实注册的 /api/v2/*（见 research/api-contract.md §2）。
// 丢弃 es.js legacy 死端点（/requests、/aggregate*、/analysis/*、/favorites、/virtual-groups）。
// PR0 覆盖 dashboard/config + 各维度 list/detail（GET）；mutation 在对应 PR 补。

import { apiGet, apiPost, apiPut } from './client'
import type {
  ApiData,
  ApiList,
  DashboardSummary,
  EstimateAncientResponse,
  GlobalConfig,
  ListParams,
  NeedsV2DetailResponse,
  NeedsV2Summary,
  OrgV2Row,
  TaskDetailResponse,
  TaskListItem,
  UpdateTaskManualRequest,
  UserV2Row,
} from './types'

// ---- Dashboard & Config ----
export function getDashboardSummary(params: { startDate?: string; endDate?: string }) {
  return apiGet<DashboardSummary>('/v2/dashboard/summary', params)
}

export function getGlobalConfig() {
  return apiGet<GlobalConfig>('/v2/config')
}

// ---- Needs（小数口径） ----
export function getNeedsV2(params: ListParams) {
  return apiGet<ApiList<NeedsV2Summary>>('/v2/needs', params)
}

export function getNeedDetailV2(needId: string) {
  return apiGet<NeedsV2DetailResponse>(`/v2/needs/${encodeURIComponent(needId)}`)
}

/**
 * 翻页拉全 needs：后端 /v2/needs 的 pageSize 被钳到 200，单页请求会在 total>200 时静默截断。
 * 高管大屏的趋势/Top 榜需要全量，故先拉第 1 页读 total，不足则继续翻页合并 data。
 * 终止条件：累计条数 >= total，或某页 data 为空，或翻页超过 MAX_PAGES（兜底防死循环）。
 * 各页不传 order（后端默认即可，调用方会再 sortRows）。
 */
export async function getAllNeedsV2(params: ListParams): Promise<NeedsV2Summary[]> {
  const MAX_PAGES = 50
  const PAGE_SIZE = 200
  // 抹掉外部可能传入的 page/order，由本函数统一控制翻页。
  const { page: _page, order: _order, pageSize: _pageSize, ...rest } = params
  void _page
  void _order
  void _pageSize

  const all: NeedsV2Summary[] = []
  let total = Infinity
  for (let page = 1; page <= MAX_PAGES; page += 1) {
    const res = await getNeedsV2({ ...rest, page, pageSize: PAGE_SIZE })
    const rows = res.data ?? []
    all.push(...rows)
    if (typeof res.total === 'number') total = res.total
    if (rows.length === 0 || all.length >= total) break
  }
  return all
}

// ---- Users（小数口径） ----
export function getUsersV2(params: { startDate?: string; endDate?: string }) {
  return apiGet<ApiData<UserV2Row>>('/v2/users', params)
}

export function getUserDetailV2(userId: string, params: { startDate?: string; endDate?: string }) {
  return apiGet<Record<string, unknown>>(`/v2/users/${encodeURIComponent(userId)}`, params)
}

// ---- Orgs（列表小数口径 / 详情百分比口径） ----
export function getOrgV2(params: { startDate?: string; endDate?: string }) {
  return apiGet<ApiData<OrgV2Row> & { no_org_mapping?: boolean }>('/v2/orgs', params)
}

export function getOrgDetailV2(params: { orgPath: string; startDate?: string; endDate?: string }) {
  return apiGet<Record<string, unknown>>('/v2/orgs/detail', params)
}

// ---- Commits（百分比口径） ----
export function getCommitsV2(params: ListParams) {
  return apiGet<ApiList<Record<string, unknown>>>('/v2/commits', params)
}

export function getCommitDetailV2(commitId: string) {
  return apiGet<Record<string, unknown>>(`/v2/commits/${encodeURIComponent(commitId)}`)
}

// ---- Repos（百分比口径） ----
export function getReposV2(params: ListParams) {
  return apiGet<ApiList<Record<string, unknown>>>('/v2/repos', params)
}

export function getRepoDetailV2(params: { repoAddr: string; repoBranch?: string; startDate?: string; endDate?: string }) {
  return apiGet<Record<string, unknown>>('/v2/repos/detail', params)
}

export function getRepoBranches(repoAddr: string) {
  return apiGet<{ branches?: string[] } | string[]>('/v2/repos/branches', { repoAddr })
}

// ---- Tasks（⚠️ 百分比口径：efficiency_ratio 300=300%，不 ×100） ----
export function getTasksV2(params: ListParams) {
  return apiGet<ApiList<TaskListItem>>('/v2/tasks', params)
}

// taskId 是 hash 串（对齐 Vue：getTaskDetailV2 未 encodeURIComponent），但 encode 更安全且不改变 hash。
export function getTaskDetailV2(taskId: string) {
  return apiGet<TaskDetailResponse>(`/v2/tasks/${encodeURIComponent(taskId)}`)
}

/** 人工调整：PUT /v2/tasks/{taskId}/manual（实际耗时 / 传统预估 各值+理由 4 字段）。 */
export function updateTaskManualV2(taskId: string, body: UpdateTaskManualRequest) {
  return apiPut<unknown>(`/v2/tasks/${encodeURIComponent(taskId)}/manual`, body)
}

/**
 * 古法估算：POST /v2/tasks/estimate-ancient（重算耗时，timeout 600s）。
 * 前端只读 { success, total }（§5.2）。
 */
export function estimateAncientMinutes() {
  return apiPost<EstimateAncientResponse>('/v2/tasks/estimate-ancient', undefined, { timeout: 600000 })
}

/**
 * 任务文件查看 URL（§7.8）：不走 axios，直接拼 /api 路径供 <a href target=_blank> 打开。
 * type ∈ {'summary','conversation'}；date 是前端拼的冗余参数（后端按 task 自身定位文件，未读 date）。
 */
export function getTaskFileUrl(type: 'summary' | 'conversation', taskId: string, startTime?: string | null): string {
  if (!startTime) return ''
  const d = new Date(startTime)
  if (Number.isNaN(d.getTime())) return ''
  const date = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
  return `/api/v2/tasks/file?type=${type}&taskId=${encodeURIComponent(taskId)}&date=${date}`
}

// ---- Projects（百分比口径，列表无分页） ----
export function getProjects(params?: ListParams) {
  return apiGet<ApiList<Record<string, unknown>>>('/v2/projects', params)
}

export function getProjectDetail(projectId: string) {
  return apiGet<Record<string, unknown>>(`/v2/projects/${encodeURIComponent(projectId)}`)
}

// ---- User Groups ----
export function getUserGroupDetail(groupId: string, params: { startDate?: string; endDate?: string }) {
  return apiGet<Record<string, unknown>>(`/v2/user-groups/${encodeURIComponent(groupId)}`, params)
}
