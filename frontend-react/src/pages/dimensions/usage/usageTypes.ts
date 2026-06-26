// 使用看板（部门树·视角切换）页面局部类型 —— 对齐后端 zhaoshang-show-data
// （/chat-indicator-statistics/api/v1/stats/departments/* 与 /stats/users/*）。
// 仅本目录消费，不进 api/types.ts（沿用 platformUserData「页面局部 interface」惯例）。
// 后端 n+1 双计数：除 success_rate/error_rate 外，下列指标均已排除失败请求（clean 口径）。

// ============================ 部门聚合 ============================

/** /stats/departments/:dept_id/overview —— 部门全指标聚合 */
export interface DeptOverviewResp {
  dept_id: string
  include_children: boolean
  active_users: number
  total_requests: number
  avg_requests_per_user: number
  sum_prompt_tokens: number
  sum_completion_tokens: number
  sum_total_tokens: number
  avg_prompt_tokens_per_user: number
  avg_completion_tokens_per_user: number
  avg_total_tokens_per_user: number
  total_sessions: number
  avg_ttft_ms: number
  avg_token_output_speed: number
  avg_duration_ms: number
  success_rate: number
  error_rate: number
}

export interface ActiveUsersDailyPoint {
  date: string
  dau: number
  wau: number
  mau: number
  dau_wau_ratio: number
}

/** /stats/departments/:dept_id/active-users —— DAU/WAU/MAU + 粘性 */
export interface DeptActiveUsersResp {
  dau: number
  wau: number
  mau: number
  dau_wau_ratio: number
  daily_trend: ActiveUsersDailyPoint[]
}

/** /stats/departments/:dept_id/trend —— 按天趋势（请求量 / token / 活跃用户） */
export interface DeptTrendPoint {
  date: string
  request_count: number
  prompt_tokens: number
  completion_tokens: number
  active_users: number
}
export interface DeptTrendResp {
  trend: DeptTrendPoint[]
}

export interface DeptModelItem {
  model: string
  request_count: number
  request_pct: number
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  token_pct: number
  input_output_ratio: number
  success_rate: number
  estimated_total_cost: number
}

/** Auto 路由分流（字段后端可能为 model/routed_model、count/request_count，宽松） */
export interface AutoRoutingItem {
  model?: string
  routed_model?: string
  count?: number
  request_count?: number
  pct?: number
  request_pct?: number
}

/** /stats/departments/:dept_id/models/usage —— 各模型用量与占比 */
export interface DeptModelsResp {
  models: DeptModelItem[]
  auto_routing: AutoRoutingItem[]
}

export interface WeekdayItem {
  weekday: number
  weekday_name: string
  request_count: number
}

/** /stats/departments/:dept_id/distribution/weekly —— 按星期分布 */
export interface DeptWeeklyResp {
  weekdays: WeekdayItem[]
}

export interface ResultModelItem {
  model: string
  total_requests: number
  error_requests: number
  success_rate: number
  error_rate: number
}

/** /stats/departments/:dept_id/results —— 请求结果（成功/失败/各模型成功率） */
export interface DeptResultsResp {
  total_requests: number
  success_requests: number
  error_requests: number
  success_rate: number
  error_rate: number
  models: ResultModelItem[]
}

export interface PeriodSpan {
  start: string
  end: string
  total_requests: number
  sum_total_tokens: number
}

/** /stats/departments/:dept_id/usage/period-compare —— 环比变化 */
export interface DeptPeriodCompareResp {
  current_period: PeriodSpan
  previous_period: PeriodSpan
  request_change_pct: number
  token_change_pct: number
}

export interface DeptMemberItem {
  universal_id: string
  username?: string
  user_id?: string // 工号（可读标识，区别于 universal_id 用户唯一 id）
  total_requests: number
  sum_prompt_tokens?: number
  sum_completion_tokens?: number
  sum_total_tokens: number
  success_rate: number
  avg_duration_ms?: number
  active_days: number
  estimated_total_cost?: number
}

/** /stats/departments/:dept_id/members —— 部门下人员分页列表 */
export interface DeptMembersResp {
  dept_id: string
  total: number
  page: number
  page_size: number
  members: DeptMemberItem[]
}

// ============================ 个人 ============================

export interface UserDetailRow {
  universal_id: string
  username?: string
  total_requests: number
  success_requests: number
  error_requests: number
  success_rate: number
  error_rate: number
  sum_prompt_tokens: number
  sum_completion_tokens: number
  sum_total_tokens: number
  sum_cache_tokens: number
  total_sessions: number
  active_days: number
  avg_duration_ms: number
  avg_ttft_ms: number
  avg_token_output_speed: number
  model_preference?: string
  estimated_total_cost: number
}

export interface UserDeptItem {
  user_id?: string
  username?: string
  dept_id: string
  dept_name: string
  is_main: number
}

/** /stats/users/:uid/detail —— 个人全维度详情 */
export interface UserDetailResp {
  user_detail: UserDetailRow
  models: DeptModelItem[]
  auto_routing: AutoRoutingItem[]
  departments: UserDeptItem[]
}

/** /stats/users/:uid/trend —— 个人按天趋势（字段对齐 daily_user_metrics_summary） */
export interface UserTrendPoint {
  date: string
  total_requests?: number
  success_requests?: number
  error_requests?: number
  sum_prompt_tokens?: number
  sum_completion_tokens?: number
  sum_total_tokens?: number
  sum_cache_tokens?: number
  unique_task_count?: number
  avg_duration_ms?: number | null
  avg_first_token_duration_ms?: number | null
  estimated_total_cost?: number | null
  estimated_input_cost?: number | null
  estimated_output_cost?: number | null
  estimated_cache_cost?: number | null
  estimated_request_cost?: number | null
  model_preference?: string | null
  auto_router_breakdown?: string | null
}
