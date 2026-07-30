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
  ChatLogPreviewResponse,
  ChatModelTrendSeries,
  ChatRealtimeResponse,
  ChatSyncSubmitReq,
  ChatSyncSubmitResponse,
  ChatSyncTaskListResponse,
  ChatSyncTaskStatus,
  ChatSystemConfig,
  ChatTraceLogResponse,
  ChatUserTrendRow,
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
  DashboardTrends,
  DeptMembersResponse,
  DeptOverviewResponse,
  DeptRankingResponse,
  DeptTreeNode,
  EfficiencyV2AggregateResponse,
  EntityTrendResponse,
  GlobalConfig,
  ListParams,
  NeedRepoOption,
  NeedsV2DetailResponse,
  NeedsV2Summary,
  OrgDetailResponse,
  ProjectDetailResponse,
  ProjectListItem,
  ProjectNeedsResponse,
  RepoBranchesResponse,
  RepoDetailResponse,
  RepoListItem,
  TaskDetailResponse,
  TaskListItem,
  UpdateCommitManualRequest,
  UpdateProjectManualRequest,
  UpdateProjectNeedSelectionRequest,
  UpdateProjectRequest,
  UpdateTaskManualRequest,
  UserGroupDetailResponse,
  UserV2DetailResponse,
  UserV2Row,
} from './types'

// ---- Dashboard & Config ----
export function getDashboardSummary(params: { startDate?: string; endDate?: string }) {
  return apiGet<DashboardSummary>('/v2/dashboard/summary', params)
}

/** 首页 4 维周趋势(sparkline) + 本期vs上期环比。跨用户聚合 user_productivity_v2 周表。 */
export function getDashboardTrends(params: { startDate?: string; endDate?: string }) {
  return apiGet<DashboardTrends>('/v2/dashboard/trends', params)
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

// ---- Efficiency 周聚合（user_productivity_v2 周表，小数口径 efficiency_ratio）----
// 返回 user×week 行（week_start/efficiency_ratio/work_efficiency_ratio/actual_calendar_min...），
// 支持日期范围 + userId 过滤。供「效率」维度时间线按周聚合（一次拉回全部周，前端分桶，无需切窗）。
export function getEfficiencyV2(params: { startDate?: string; endDate?: string; userId?: string }) {
  return apiGet<EfficiencyV2AggregateResponse>('/v2/efficiency', params)
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

// 组织树总览（整棵森林 + 每节点整棵子树守恒提效汇总，一次性返回）。
// 替代组织树前端逐展开节点 N× 调 /ranking（N+1 → 1）；含日期窗（提效随时间窗变）。
export function getDeptOverviewV2(params: { startDate?: string; endDate?: string }) {
  return apiGet<DeptOverviewResponse>('/v2/dept-tree/overview', { ...params })
}

// 部门成员（dept-sync 花名册 + 看板 V2 指标，按 universal_id 左连）。无看板数据的成员也返回。
// 后端代理 dept-sync /department/{dept_id}/users（只返直属成员，非递归）。
export function getDeptTreeMembersV2(params: { deptId: string; startDate?: string; endDate?: string }) {
  const { deptId, ...rest } = params
  return apiGet<DeptMembersResponse>('/v2/dept-tree/members', { dept_id: deptId, ...rest })
}

// 部门排行（一次聚合）：返回 parentDeptId 的各直接子部门整棵子树汇总，供首页部门 PK 一次性消费。
// parentDeptId 为空 → 后端默认取配置根（排「全公司一级部门」）。替代逐子部门 N× members 全表聚合。
export function getDeptRankingV2(params: { parentDeptId?: string; startDate?: string; endDate?: string }) {
  const { parentDeptId, ...rest } = params
  return apiGet<DeptRankingResponse>('/v2/dept-tree/ranking', { parent_dept_id: parentDeptId, ...rest })
}

// 部门按周时间线（整棵子树成员 user_productivity_v2 周表现聚合）：efficiency_pct(百分比) + need_count/commit_count/diff_lines。
// 复用 repo/project-trend 的 EntityTrendPoint（loc/cost 部门趋势恒 0）。dept-sync 不可达 → 502/503 或 {data:[]}。
export function getDeptTreeTrendV2(params: { deptId?: string; startDate?: string; endDate?: string }) {
  const { deptId, ...rest } = params
  // deptId 空 → dept_id 传空串，后端默认公司根（全公司整树周趋势）。
  return apiGet<EntityTrendResponse>('/v2/dept-tree/trend', { dept_id: deptId ?? '', ...rest })
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

/**
 * 翻页拉全 repos：/v2/repos 服务端按 pageSize 切片，单次 pageSize:1000 在 total>1000 时静默截断
 * （实际仓库 >1000 被截到 1000，修 #6）。仓库聚合排行/分布全在客户端做，必须先拉全量。
 * 先拉第 1 页读 total，不足则继续翻页合并。MAX_PAGES 兜底防死循环。
 */
export async function getAllReposV2(params: { startDate?: string; endDate?: string }): Promise<RepoListItem[]> {
  const MAX_PAGES = 100
  const PAGE_SIZE = 500
  const all: RepoListItem[] = []
  let total = Infinity
  for (let page = 1; page <= MAX_PAGES; page += 1) {
    const res = await getReposV2({ ...params, page, pageSize: PAGE_SIZE })
    const rows = res.data ?? []
    all.push(...rows)
    if (typeof res.total === 'number') total = res.total
    if (rows.length === 0 || all.length >= total) break
  }
  return all
}

/** 项目「添加来源」仓库选择器数据源：GET /v2/need-repo-options（needs 同源、规范化地址，与候选池一致）。 */
export function getNeedRepoOptions() {
  return apiGet<ApiData<NeedRepoOption>>('/v2/need-repo-options')
}

export function getRepoDetailV2(params: { repoAddr: string; repoBranch?: string; startDate?: string; endDate?: string }) {
  return apiGet<RepoDetailResponse>('/v2/repos/detail', params)
}

export function getRepoBranches(repoAddr: string) {
  return apiGet<RepoBranchesResponse>('/v2/repos/branches', { repoAddr })
}

/** 仓库按周时间线（commits 现聚合）：repoAddr 空=全部仓库，非空=单仓跨全部分支。 */
export function getRepoTrendV2(params: { repoAddr?: string; startDate?: string; endDate?: string }) {
  return apiGet<EntityTrendResponse>('/v2/repo-trend', params)
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
 * 任务文件查看 URL（§7.8）：不走 axios，直接拼 /kanban/api 路径供 <a href target=_blank> 打开。
 * type ∈ {'summary','conversation'}；后端按 task 自身定位文件，不需要 date。
 */
export function getTaskFileUrl(type: 'summary' | 'conversation', taskId: string): string {
  return `/kanban/api/v2/tasks/file?type=${type}&taskId=${encodeURIComponent(taskId)}`
}

// ---- Projects（⚠️ 百分比口径 efficiency_ratio；列表无分页 {data:[]}） ----
export function getProjects(params?: ListParams) {
  return apiGet<ApiData<ProjectListItem>>('/v2/projects', params)
}

export function getProjectDetail(projectId: string) {
  return apiGet<ProjectDetailResponse>(`/v2/projects/${encodeURIComponent(projectId)}`)
}

/** 项目按周时间线（干净 Need 按 dev_end_ts 现聚合）：projectId 空=全部干净 Need，非空=该项目候选池。 */
export function getProjectTrendV2(params: { projectId?: string; startDate?: string; endDate?: string }) {
  return apiGet<EntityTrendResponse>('/v2/project-trend', params)
}

/** 创建 Project：POST /v2/projects（返回含 project_id）。 */
export function createProject(body: CreateProjectRequest) {
  return apiPost<CreateProjectResponse>('/v2/projects', body)
}

/** 编辑 Project：PUT /v2/projects/{id}。⚠️ 必须回传 repos 原值，否则后端清空；task_ids 已不属项目模型。 */
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

/** 项目候选池 Need 列表：GET /v2/projects/{id}/needs（⚠️ 小数口径；含 excluded 标记）。 */
export function getProjectNeeds(projectId: string) {
  return apiGet<ProjectNeedsResponse>(`/v2/projects/${encodeURIComponent(projectId)}/needs`)
}

/** 纳入/排除单个 Need：PUT /v2/projects/{id}/needs/selection（写 exclude_needs，不影响 commit 古法口径）。 */
export function updateProjectNeedSelection(projectId: string, body: UpdateProjectNeedSelectionRequest) {
  return apiPut<unknown>(`/v2/projects/${encodeURIComponent(projectId)}/needs/selection`, body)
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

  /** 明细点查（最多 5000 条；时间 ISO 8601 必填）。 */
  queryDetail(body: ChatDetailQueryReq) {
    return chatPost<ChatDetailQueryResponse>('/stats/detail/query', body)
  },
  /** 原始日志预览（local_log_path 会由 chat 服务限制在配置的根目录内）。 */
  previewLog(body: { local_log_path: string }) {
    return chatPost<ChatLogPreviewResponse>('/stats/detail/log-preview', body)
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

  // -- 链路日志查询（Loki 等后端） --
  traceLogs(body: { datasource_id: string; request_id: string; label_selector?: string; start_time: string; end_time: string; limit?: number; cursor?: string }) {
    return chatPost<ChatTraceLogResponse>('/stats/trace-logs', body)
  },

  // -- 单用户趋势 --
  userTrend(uid: string, params: { start_date: string; end_date: string }) {
    return chatGet<ChatUserTrendRow[]>(`/stats/users/${encodeURIComponent(uid)}/trend`, params)
  },

  // -- 模型请求/Token 趋势 --
  modelTrend(params: { start_date: string; end_date: string; models?: string }) {
    return chatGet<ChatModelTrendSeries[]>('/stats/model-trend', params)
  },
}
