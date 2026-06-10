// typed API 端点 —— 只对齐后端真实注册的 /api/v2/*（见 research/api-contract.md §2）。
// 丢弃 es.js legacy 死端点（/requests、/aggregate*、/analysis/*、/favorites、/virtual-groups）。
// PR0 覆盖 dashboard/config + 各维度 list/detail（GET）；mutation 在对应 PR 补。

import { apiDelete, apiGet, apiPost, apiPut, chatDelete, chatGet, chatPost, chatPut } from './client'
import type {
  ChatDatasource,
  ChatDatasourceTestResult,
  ChatDatasourceUpsert,
  ChatDetailQueryReq,
  ChatDetailQueryResponse,
  ChatRealtimeResponse,
  ChatSyncSubmitReq,
  ChatSyncSubmitResponse,
  ChatSyncTaskListResponse,
  ChatSyncTaskStatus,
  ChatSystemConfig,
  ModelPricing,
  ModelPricingUpsert,
  AddRepoRequest,
  AddTasksRequest,
  ApiData,
  ApiList,
  CheckConflictsResponse,
  CommitDetailResponse,
  CommitListItem,
  CreateProjectRequest,
  CreateProjectResponse,
  DashboardSummary,
  DeptMembersResponse,
  DeptTreeNode,
  EstimateAncientResponse,
  GlobalConfig,
  ListParams,
  NeedsV2DetailResponse,
  NeedsV2Summary,
  OrgDetailResponse,
  ProjectDetailResponse,
  ProjectListItem,
  RepoBranchesResponse,
  RepoDetailResponse,
  RepoListItem,
  TaskDetailResponse,
  TaskListItem,
  UpdateCommitManualRequest,
  UpdateProjectManualRequest,
  UpdateProjectRequest,
  UpdateTaskManualRequest,
  UpdateTaskSilicaRequest,
  UserGroupDetailResponse,
  UserV2DetailResponse,
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
// /v2/users 默认 pageSize=50 且服务端切片，列表页一次性拉全（pageSize:1000）再客户端分页/排序/过滤，对齐 Vue。
export function getUsersV2(params: { startDate?: string; endDate?: string; page?: number; pageSize?: number }) {
  return apiGet<ApiList<UserV2Row>>('/v2/users', params)
}

export function getUserDetailV2(userId: string, params: { startDate?: string; endDate?: string }) {
  return apiGet<UserV2DetailResponse>(`/v2/users/${encodeURIComponent(userId)}`, params)
}

/**
 * 翻页拉全 users：/v2/users 服务端按 pageSize 切片，单次 pageSize:1000 在 total>1000 时静默截断
 * （实测内网 1462 用户被截到 1000）。本页分页/排序/过滤全客户端做，必须先拉全量。
 * 先拉第 1 页读 total，不足则继续翻页合并。MAX_PAGES 兜底防死循环。
 */
export async function getAllUsersV2(params: { startDate?: string; endDate?: string }): Promise<UserV2Row[]> {
  const MAX_PAGES = 50
  const PAGE_SIZE = 500
  const all: UserV2Row[] = []
  let total = Infinity
  for (let page = 1; page <= MAX_PAGES; page += 1) {
    const res = await getUsersV2({ ...params, page, pageSize: PAGE_SIZE })
    const rows = res.data ?? []
    all.push(...rows)
    if (typeof res.total === 'number') total = res.total
    if (rows.length === 0 || all.length >= total) break
  }
  return all
}

// 用户 user_id(==dept-sync universal_id) → 权威真名+工号（来自 dept_user）。供 useUserNameMap 解析「真名(工号)」。
export interface UserNameRow {
  user_id: string
  real_name: string
  emp_no: string
}
export function getUserNamesV2() {
  return apiGet<UserNameRow[]>('/v2/user-names')
}

// ---- Orgs（列表小数口径 / 详情百分比口径） ----
// ⚠️ 后端读取的是 org_path（snake_case），传 orgPath 会报「org_path 不能为空」。对齐 Vue OrgDetailV2。
export function getOrgDetailV2(params: { orgPath: string; startDate?: string; endDate?: string; granularity?: string }) {
  const { orgPath, ...rest } = params
  return apiGet<OrgDetailResponse>('/v2/orgs/detail', { org_path: orgPath, ...rest })
}

// 组织树（dept-sync 权威全量嵌套部门树，后端代理 dept-sync /department/tree；与日期无关）。
export function getDeptTreeV2() {
  return apiGet<DeptTreeNode[]>('/v2/dept-tree')
}

// 部门成员（dept-sync 花名册 + 看板 V2 指标，按 universal_id 左连）。无看板数据的成员也返回。
// 后端代理 dept-sync /department/{dept_id}/users（只返直属成员，非递归）。
export function getDeptTreeMembersV2(params: { deptId: string; startDate?: string; endDate?: string }) {
  const { deptId, ...rest } = params
  return apiGet<DeptMembersResponse>('/v2/dept-tree/members', { dept_id: deptId, ...rest })
}

// ---- Commits（⚠️ 百分比口径：efficiency_ratio 300=300%，不 ×100） ----
export function getCommitsV2(params: ListParams) {
  return apiGet<ApiList<CommitListItem>>('/v2/commits', params)
}

// commit_id 是 hash 串（对齐 Vue：CommitViewV2 行内不 encode；这里 encode 更安全且不改变 hash）。
export function getCommitDetailV2(commitId: string) {
  return apiGet<CommitDetailResponse>(`/v2/commits/${encodeURIComponent(commitId)}`)
}

/** 人工调整：PUT /v2/commits/{id}/manual（传统预估 / 实际耗时 各值+理由 4 字段）。 */
export function updateCommitManualV2(commitId: string, body: UpdateCommitManualRequest) {
  return apiPut<unknown>(`/v2/commits/${encodeURIComponent(commitId)}/manual`, body)
}

// ---- Repos（⚠️ 百分比口径：efficiency_ratio = CalcEfficiencyRatio(ancient,real)，不 ×100） ----
export function getReposV2(params: ListParams) {
  return apiGet<ApiList<RepoListItem>>('/v2/repos', params)
}

export function getRepoDetailV2(params: { repoAddr: string; repoBranch?: string; startDate?: string; endDate?: string }) {
  return apiGet<RepoDetailResponse>('/v2/repos/detail', params)
}

export function getRepoBranches(repoAddr: string) {
  return apiGet<RepoBranchesResponse>('/v2/repos/branches', { repoAddr })
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

// ---- Projects（⚠️ 百分比口径 efficiency_ratio；列表无分页 {data:[]}） ----
export function getProjects(params?: ListParams) {
  return apiGet<ApiData<ProjectListItem>>('/v2/projects', params)
}

export function getProjectDetail(projectId: string) {
  return apiGet<ProjectDetailResponse>(`/v2/projects/${encodeURIComponent(projectId)}`)
}

/** 创建 Project：POST /v2/projects（返回含 project_id）。 */
export function createProject(body: CreateProjectRequest) {
  return apiPost<CreateProjectResponse>('/v2/projects', body)
}

/** 编辑 Project：PUT /v2/projects/{id}。⚠️ 必须回传 repos/task_ids/task_ids_silica 原值，否则后端清空。 */
export function updateProject(projectId: string, body: UpdateProjectRequest) {
  return apiPut<unknown>(`/v2/projects/${encodeURIComponent(projectId)}`, body)
}

/** 删除 Project：DELETE /v2/projects/{id}。 */
export function deleteProject(projectId: string) {
  return apiDelete<{ status?: string }>(`/v2/projects/${encodeURIComponent(projectId)}`)
}

/** 人工调整：PUT /v2/projects/{id}/manual（3 组 minutes/reason + start/end_time_manual）。 */
export function updateProjectManual(projectId: string, body: UpdateProjectManualRequest) {
  return apiPut<unknown>(`/v2/projects/${encodeURIComponent(projectId)}/manual`, body)
}

/** 加 Task：POST /v2/projects/{id}/tasks（task_ids + 同长 silica 数组）。 */
export function addTasksToProject(projectId: string, body: AddTasksRequest) {
  return apiPost<unknown>(`/v2/projects/${encodeURIComponent(projectId)}/tasks`, body)
}

/** 移 Task：DELETE /v2/projects/{id}/tasks（body 传 task_ids）。 */
export function removeTasksFromProject(projectId: string, body: { task_ids: string[] }) {
  return apiDelete<unknown>(`/v2/projects/${encodeURIComponent(projectId)}/tasks`, { data: body })
}

/** 改 Task silica 权重：PUT /v2/projects/{id}/tasks/silica。 */
export function updateTaskSilicaInProject(projectId: string, body: UpdateTaskSilicaRequest) {
  return apiPut<unknown>(`/v2/projects/${encodeURIComponent(projectId)}/tasks/silica`, body)
}

/** 加 Repo filter：POST /v2/projects/{id}/repos。 */
export function addRepoToProject(projectId: string, body: AddRepoRequest) {
  return apiPost<unknown>(`/v2/projects/${encodeURIComponent(projectId)}/repos`, body)
}

/** 移 Repo filter：DELETE /v2/projects/{id}/repos/{index}（数组下标，删后会漂移需 reload）。 */
export function removeRepoFromProject(projectId: string, index: number) {
  return apiDelete<unknown>(`/v2/projects/${encodeURIComponent(projectId)}/repos/${index}`)
}

/** 冲突检测：POST /v2/projects/check-conflicts → conflicts[{commit_id,project_id,project_name}]。 */
export function checkProjectConflicts(body: { commit_ids: string[] }) {
  return apiPost<CheckConflictsResponse>('/v2/projects/check-conflicts', body)
}

// ---- User Groups（⚠️ 百分比口径；后端无列表端点，只有 detail/delete） ----
export function getUserGroupDetail(groupId: string, params: { startDate?: string; endDate?: string }) {
  return apiGet<UserGroupDetailResponse>(`/v2/user-groups/${encodeURIComponent(groupId)}`, params)
}

/** 删除用户组：DELETE /v2/user-groups/{groupId}（删除后跳回用户列表）。 */
export function deleteUserGroup(groupId: string) {
  return apiDelete<{ status?: string }>(`/v2/user-groups/${encodeURIComponent(groupId)}`)
}

// ---- Chat Stats（chat-indicator-statistics 代理，/api/v2/chat/*） ----
// ⚠️ 全部走 chatGet/chatPost/...（信封 {success,code,data} 解包），不要混用 apiGet。
// chat_stats_enabled=false 时后端 503，调用方应先看 GlobalConfig 开关再发请求。
export const chatStats = {
  /** 实时态势聚合（直查源库，服务端 10 秒限频；range ∈ 30m|1h|3h）。 */
  getRealtime(params: { range: '30m' | '1h' | '3h'; datasource_id?: string; max_rows?: number }) {
    return chatGet<ChatRealtimeResponse>('/stats/realtime', params)
  },

  /** 明细点查（最多 100 条；时间 ISO 8601 必填）。 */
  queryDetail(body: ChatDetailQueryReq) {
    return chatPost<ChatDetailQueryResponse>('/stats/detail/query', body)
  },

  // -- 模型价格 CRUD --
  listPricing() {
    return chatGet<ModelPricing[]>('/pricing/models')
  },
  createPricing(body: ModelPricingUpsert) {
    return chatPost<ModelPricing>('/pricing/models', body)
  },
  updatePricing(id: number, body: ModelPricingUpsert) {
    return chatPut<ModelPricing>(`/pricing/models/${id}`, body)
  },
  deletePricing(id: number) {
    return chatDelete<void>(`/pricing/models/${id}`)
  },

  // -- 数据源管理 --
  listDatasources() {
    return chatGet<ChatDatasource[]>('/datasources')
  },
  createDatasource(body: ChatDatasourceUpsert) {
    return chatPost<ChatDatasource>('/datasources', body)
  },
  updateDatasource(id: number, body: ChatDatasourceUpsert) {
    return chatPut<ChatDatasource>(`/datasources/${id}`, body)
  },
  deleteDatasource(id: number) {
    return chatDelete<void>(`/datasources/${id}`)
  },
  /** 连接测试（⚠️ 失败也是 HTTP 200，看返回的 success/message）。 */
  testDatasource(id: number) {
    return chatPost<ChatDatasourceTestResult>(`/datasources/${id}/test`)
  },

  // -- 同步任务 --
  listSyncTasks() {
    return chatGet<ChatSyncTaskListResponse>('/sync/tasks')
  },
  getSyncTask(taskId: string) {
    return chatGet<ChatSyncTaskStatus>(`/sync/tasks/${encodeURIComponent(taskId)}`)
  },
  submitSyncTask(body: ChatSyncSubmitReq) {
    return chatPost<ChatSyncSubmitResponse>('/sync/tasks', body)
  },
  retrySyncTask(taskId: string) {
    return chatPost<{ task_id: string; status: string }>(`/sync/tasks/${encodeURIComponent(taskId)}/retry`)
  },
  cancelSyncTask(taskId: string) {
    return chatPost<{ task_id: string; status: string }>(`/sync/tasks/${encodeURIComponent(taskId)}/cancel`)
  },

  // -- 系统配置（KV：system_currency 等） --
  getConfig() {
    return chatGet<ChatSystemConfig>('/config')
  },
  updateConfig(body: ChatSystemConfig) {
    return chatPut<void>('/config', body)
  },
}
