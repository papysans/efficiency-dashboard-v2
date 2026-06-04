// 后端返回类型 —— 字段对齐 backend Go struct json tag，见 research/api-contract.md §2。
// PR0 先定义高管大屏/核心列表会用到的类型；其余 detail/mutation 类型在对应 PR 补。

/** 通用分页响应包（多数 list 端点） */
export interface ApiList<T> {
  total: number
  page: number
  pageSize: number
  data: T[]
}

/** 无分页响应包（Org/Project 列表） */
export interface ApiData<T> {
  data: T[]
}

/** /v2/config */
export interface GlobalConfig {
  traditional_dev_lines_per_day: number
}

/** /v2/dashboard/summary（§2.9 / §5） */
export interface DashboardSummary {
  total_tasks: number
  total_users: number
  total_repos: number
  total_commits: number
  total_branchs: number
  total_work_dirs: number
  total_cost: number
  total_tokens: number
  total_task_lines: number
  total_commit_lines: number
  total_diff_lines: number
  total_real_minutes: number
  avg_efficiency_ratio: number
  total_task_ancient_minutes: number
  total_task_real_minutes: number
  task_efficiency_ratio: number
  total_commit_ancient_minutes: number
  total_commit_real_minutes: number
  commit_efficiency_ratio: number
  total_users_v2: number
  total_needs: number
  merged_needs: number
  eligible_needs: number
  need_actual_calendar_min: number
  need_baseline_calendar_min: number
  need_calendar_ratio: number | null // 小数口径
  need_work_ratio: number | null // 小数口径
}

/** /v2/needs 列表项（§2.1，小数口径） */
export interface NeedsV2Summary {
  need_id: string
  boundary_source: string
  // 后端返回字符串枚举（如 'high'），curl 已证实，非数值
  boundary_confidence?: string | null
  status: string
  repo_addr: string
  repo_branch: string
  primary_user_id: string
  dev_start_ts: string
  dev_end_ts: string
  merge_ts: string
  total_calendar_min: number
  baseline_calendar_min: number | null
  total_active_work_corrected_min: number
  baseline_fused_work_min: number | null
  efficiency_ratio: number | null // 日历提效，小数口径
  efficiency_band_low: number | null
  efficiency_band_high: number | null
  work_efficiency_ratio: number | null // 工作量提效，小数口径
  confidence_level?: string
  outlier_flag: boolean
  coverage_eligible: boolean
  total_think_min: number
  total_exec_min: number
  total_verify_min: number
  reason: string
}

/**
 * /v2/needs/{id} 的 need 对象（§3.1.1，全部 snake_case；指针字段普遍可 null）。
 * 仅声明详情页用到的字段；其余字段后端有但页面未消费，留 index 兜底。
 */
export interface NeedDetail {
  need_id: string
  status?: string
  boundary_source?: string
  boundary_confidence?: string | null
  boundary_key?: string
  repo_addr?: string
  repo_branch?: string
  primary_user_id?: string
  contributor_user_ids?: string[] | null
  touched_files?: string[] | string | null
  team_profile_used?: string
  dev_start_ts?: string | null
  dev_end_ts?: string | null
  merge_ts?: string | null
  dev_duration_min?: number | null
  wait_for_review_min?: number | null
  total_session_active_person_min?: number | null
  estimate_uncovered_human_min?: number | null
  total_active_work_corrected_min?: number | null
  total_calendar_min?: number | null
  total_think_min?: number | null
  total_exec_min?: number | null
  total_verify_min?: number | null
  total_other_min?: number | null
  commit_count?: number | null
  total_loc_net?: number | null
  total_files_touched?: number | null
  ai_covered_loc?: number | null
  uncovered_loc?: number | null
  uncovered_work_ratio?: number | null
  ai_code_ratio?: number | null
  silica?: number | null
  churn_ratio?: number | null
  duplication_ratio?: number | null
  revert_count?: number | null
  revert_rate?: number | null
  post_generation_deletion_ratio?: number | null
  feature_dependency_risk?: string
  silica_signal?: string
  ai_code_ratio_signal?: string
  uncovered_work_signal?: string
  efficiency_ratio?: number | null
  efficiency_band_low?: number | null
  efficiency_band_high?: number | null
  work_efficiency_ratio?: number | null
  confidence_level?: string
  outlier_flag?: boolean
  coverage_eligible?: boolean
  baseline_fused_work_min?: number | null
  baseline_calendar_min?: number | null
  reason?: string
  [k: string]: unknown
}

/** SessionStageMetric（§3.1.2，详情用到的字段） */
export interface NeedSession {
  session_id: string
  user_id?: string
  session_start_ts?: string | null
  session_end_ts?: string | null
  total_active_min?: number | null
  think_active_min?: number | null
  exec_active_min?: number | null
  verify_active_min?: number | null
  stage_confidence?: string
  summary?: string
  [k: string]: unknown
}

/** 关联 Commit（§3.1.3，详情用到的字段） */
export interface NeedCommit {
  commit_id: string
  commit_time?: string | null
  user_name?: string
  diff_lines?: number | null
  silica?: number | null
  comment?: string
  touched_files?: string[] | string | null
  [k: string]: unknown
}

/** baseline_components（§3.1.4，指针可 null） */
export interface NeedBaselineComponents {
  algo_think_min?: number | null
  algo_exec_min?: number | null
  algo_verify_min?: number | null
  algo_total_min?: number | null
  anchor_knn_min?: number | null
  anchor_knn_reason?: string | null
  llm_think_min?: number | null
  llm_exec_min?: number | null
  llm_verify_min?: number | null
  llm_total_min?: number | null
  llm_confidence?: string | null
  llm_reason?: string | null
  fused_work_min?: number | null
  spread_work_min?: number | null
  calendar_min?: number | null
  team_work_density?: number | null
}

/** /v2/needs/{id} 顶层响应（§3.1） */
export interface NeedsV2DetailResponse {
  need: NeedDetail
  sessions?: NeedSession[]
  commits?: NeedCommit[]
  stage_metrics?: NeedSession[]
  baseline_components?: NeedBaselineComponents
  confidence_signals?: Record<string, unknown>
  quality_signals?: Record<string, unknown> & { reason?: string }
}

/** /v2/users 列表项（§2.2，小数口径） */
export interface UserV2Row {
  user_id: string
  user_name: string
  week_count: number
  merged_need_count: number
  active_need_count: number
  abandoned_need_count: number
  actual_calendar_min: number
  baseline_calendar_min: number
  calendar_ratio: number | null // 小数口径
  actual_work_min: number
  baseline_work_min: number
  work_ratio: number | null // 小数口径
  commit_count: number
  commit_diff_lines: number
  cost: number
  tokens: number
  confidence_limited: boolean
  confidence_reason?: string
}

/**
 * /v2/users/{id} 周明细项（models.UserProductivityV2，小数口径）。
 * 仅声明 UserDetail 周表/趋势用到的字段；其余后端有但页面未消费，留 index 兜底。
 */
export interface UserProductivityV2 {
  user_productivity_v2_id?: string
  week_start: string
  user_id?: string
  user_name?: string
  merged_need_count?: number
  active_need_count?: number
  abandoned_need_count?: number
  efficiency_ratio?: number | null // 小数口径
  work_efficiency_ratio?: number | null // 小数口径
  commit_count?: number
  commit_diff_lines?: number
  confidence_limited?: boolean
  confidence_reason?: string
  cost?: number
  upstream_tokens?: number
  downstream_tokens?: number
  [k: string]: unknown
}

/** /v2/users/{id} 顶层响应（§User-2，summary 小数口径） */
export interface UserV2DetailResponse {
  summary: UserV2Row
  weeks: UserProductivityV2[]
  needs: NeedsV2Summary[]
  commits: NeedCommit[]
}

/** /v2/user-groups/{id} 成员/汇总项（§User-3，⚠️ 百分比口径） */
export interface UserGroupMember {
  user_id: string
  user_name: string
  day_count: number
  task_count: number
  commit_count: number
  task_diff_lines: number
  upstream_tokens: number
  downstream_tokens: number
  cost: number
  task_real_minutes: number
  task_ancient_minutes: number
  task_efficiency_ratio: number | null // 百分比口径
  commit_diff_lines: number
  commit_ancient_minutes: number
  commit_real_minutes: number
  commit_efficiency_ratio: number | null // 百分比口径
}

export interface UserGroupSummary {
  day_count: number
  task_count: number
  commit_count: number
  task_diff_lines: number
  upstream_tokens: number
  downstream_tokens: number
  cost: number
  task_real_minutes: number
  task_ancient_minutes: number
  task_efficiency_ratio: number | null // 百分比口径
  commit_diff_lines: number
  commit_ancient_minutes: number
  commit_real_minutes: number
  commit_efficiency_ratio: number | null // 百分比口径
}

export interface UserGroup {
  group_id: string
  name: string
  org_name?: string
  user_ids?: unknown
  created_at?: string
  updated_at?: string
}

/** /v2/user-groups/{id} 顶层响应（§User-3） */
export interface UserGroupDetailResponse {
  group: UserGroup | null
  summary: UserGroupSummary
  members: UserGroupMember[]
}

/** /v2/orgs 列表项（§2.3，小数口径） */
export interface OrgV2Row {
  org_name: string
  user_count: number
  merged_need_count: number
  actual_calendar_min: number
  baseline_calendar_min: number
  calendar_ratio: number | null // 小数口径
  work_ratio: number | null // 小数口径
  commit_count: number
  commit_diff_lines: number
  cost: number
}

/** 通用列表 query 参数 */
export interface ListParams {
  startDate?: string
  endDate?: string
  page?: number
  pageSize?: number
  order?: string
  [k: string]: unknown
}

/**
 * /v2/tasks 列表项 & 详情 task 对象（§6，backend db.go TaskListItem）。
 * ⚠️ efficiency_ratio 是**百分比口径**（300=300%），后端 ((ancient-real)/real)*100 算出，含 manual 覆盖。
 * 与 Need 小数口径相反，前端展示**不 ×100**，绝不用 formatV2Ratio/RatioPill。
 */
export interface TaskListItem {
  task_id: string
  session_id?: string
  commit_id?: string
  title?: string
  user_id?: string
  user_name?: string
  client_id?: string
  client_ide?: string
  client_version?: string
  client_os?: string
  client_os_version?: string
  caller?: string
  repo_addr?: string
  repo_branch?: string
  work_dir?: string
  work_dir_id?: string
  start_time?: string | null
  end_time?: string | null
  upstream_tokens?: number
  downstream_tokens?: number
  cost?: number
  silica?: number
  accept_ratio?: number
  diff_lines?: number
  task_ancient_minutes?: number | null
  task_ancient_minutes_reason?: string
  task_ancient_minutes_manual?: number | null
  task_ancient_minutes_reason_manual?: string
  task_real_minutes?: number | null
  task_real_minutes_reason?: string
  task_real_minutes_manual?: number | null
  task_real_minutes_reason_manual?: string
  efficiency_ratio?: number | null // 百分比口径
  org1?: string
  org2?: string
  org3?: string
  org4?: string
  org5?: string
  org6?: string
  org7?: string
  org8?: string
  org9?: string
  org_display?: string
  [k: string]: unknown
}

/** Conversation（§7.5，core/models/models.go Conversation，详情对话历史用） */
export interface Conversation {
  id?: number
  session_id?: string
  request_id?: string
  task_id?: string
  sender?: string
  prompt_mode?: string
  mode?: string
  model?: string
  start_time?: string | null
  end_time?: string | null
  process_time?: number | null
  process_ttft?: number | null
  upstream_tokens?: number | null
  downstream_tokens?: number | null
  cost?: number | null
  diff_lines?: number | null
  user_input?: string
  error_code?: string
  error_reason?: string
  [k: string]: unknown
}

/** /v2/tasks/{id} 顶层响应（§7.1，无 time_segments，那是死代码） */
export interface TaskDetailResponse {
  task: TaskListItem
  conversations?: Conversation[]
  efficiency_ratio?: number | null // 顶层再给一份，百分比口径
}

/** PUT /v2/tasks/{id}/manual 请求体（§7.6） */
export interface UpdateTaskManualRequest {
  task_real_minutes_manual: number | null
  task_real_minutes_reason_manual: string
  task_ancient_minutes_manual: number | null
  task_ancient_minutes_reason_manual: string
}

/** POST /v2/tasks/estimate-ancient 响应（§5.2，前端只读这两个字段） */
export interface EstimateAncientResponse {
  success: number
  total: number
}
