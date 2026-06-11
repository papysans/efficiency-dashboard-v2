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
  /** 人天单价（¥/人天），高管大屏节省成本折算用；缺省时前端兜底 2000。 */
  cost_per_person_day?: number
  dashboard_title_prefix: string
  /** chat-indicator-statistics 代理是否启用（backend chat_stats.base_url 非空）。false/缺省时「平台」导航组不渲染。 */
  chat_stats_enabled?: boolean
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
  ai_code_ratio?: number | null // 小数口径
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
  total_calendar_min: number
  baseline_calendar_min: number | null
  total_active_work_corrected_min: number
  baseline_fused_work_min: number | null
  efficiency_ratio: number | null // 日历提效，小数口径
  efficiency_band_low: number | null
  efficiency_band_high: number | null
  work_efficiency_ratio: number | null // 工作量提效，小数口径
  total_loc_net?: number | null
  ai_covered_loc?: number | null
  ai_code_ratio?: number | null
  confidence_level?: string
  outlier_flag: boolean // 派生 = 任一口径异常
  calendar_outlier_flag?: boolean // 日历提效口径异常
  work_outlier_flag?: boolean // 工作量提效口径异常
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
  dev_duration_min?: number | null
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
  outlier_flag?: boolean // 派生 = 任一口径异常
  calendar_outlier_flag?: boolean // 日历提效口径异常
  work_outlier_flag?: boolean // 工作量提效口径异常
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
  ai_code_ratio?: number | null // 小数口径
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
  ai_code_ratio?: number | null // 小数口径
  commit_count: number
  commit_diff_lines: number
  cost: number
}

/** /v2/repos 列表项（§Repo-4，⚠️ 百分比口径 efficiency_ratio = CalcEfficiencyRatio(ancient,real)）。 */
export interface RepoListItem {
  repo_addr: string
  repo_branch: string
  commit_count: number
  start_time: string
  end_time: string
  sum_ancient_minutes: number
  sum_real_minutes: number
  task_count: number
  efficiency_ratio: number // 百分比口径
  ai_code_ratio?: number | null // 小数口径
}

/** /v2/repos/detail 的 efficiency 块（百分比口径）。 */
export interface RepoEfficiency {
  repo_ancient_minutes: number
  repo_real_minutes: number
  efficiency_ratio: number // 百分比口径
  repo_ancient_minutes_reason?: string
  repo_real_minutes_reason?: string
}

/** /v2/repos/detail 的 commits 项（§Repo-5，commit_*_manual 优先；silica 为 Commit 级 AI 代码占比，小数口径）。 */
export interface RepoCommitItem {
  commit_id: string
  commit_time?: string | null
  git_user_name?: string
  comment?: string
  diff_lines?: number | null
  commit_real_minutes?: number | null
  commit_real_minutes_manual?: number | null
  commit_ancient_minutes?: number | null
  commit_ancient_minutes_manual?: number | null
  silica?: number | null
  cost?: number | null
  upstream_tokens?: number | null
  downstream_tokens?: number | null
  efficiency_ratio?: number | null // 百分比口径
  [k: string]: unknown
}

/** /v2/repos/detail 顶层响应（§Repo-5）。 */
export interface RepoDetailResponse {
  repo_addr: string
  repo_branch: string
  branches: string[]
  commits: RepoCommitItem[]
  tasks: TaskListItem[]
  efficiency: RepoEfficiency
  summary?: { commit_count?: number; task_count?: number; ai_code_ratio?: number | null }
}

/** /v2/repos/branches 响应。 */
export interface RepoBranchesResponse {
  branches: string[]
}

/** /v2/orgs/detail 汇总块（§Org-7，⚠️ 百分比口径）。 */
export interface OrgSummary {
  user_count: number
  task_diff_lines: number
  task_real_minutes: number
  task_ancient_minutes: number
  task_efficiency_ratio: number // 百分比口径
  commit_diff_lines: number
  commit_real_minutes: number
  commit_ancient_minutes: number
  commit_efficiency_ratio: number // 百分比口径
  upstream_tokens: number
  downstream_tokens: number
  cost: number
}

/** /v2/orgs/detail commits 时序项（§Org-7，百分比口径）。 */
export interface CommitTimeSeriesItem {
  period_key: string
  period_label: string
  commit_count: number
  commit_diff_lines: number
  commit_real_minutes: number
  commit_ancient_minutes: number
  commit_efficiency_ratio: number // 百分比口径
  upstream_tokens: number
  downstream_tokens: number
  cost: number
  [k: string]: unknown
}

/** /v2/orgs/detail tasks 时序项（§Org-7，百分比口径）。 */
export interface TaskTimeSeriesItem {
  period_key: string
  period_label: string
  task_count: number
  task_diff_lines: number
  task_real_minutes: number
  task_ancient_minutes: number
  task_efficiency_ratio: number // 百分比口径
  upstream_tokens: number
  downstream_tokens: number
  cost: number
  [k: string]: unknown
}

/** /v2/orgs/detail members 项（UserDetail，§Org-7，百分比口径）。 */
export interface OrgMember {
  user_id: string
  user_name: string
  org1?: string
  org2?: string
  org3?: string
  org4?: string
  org_display?: string
  task_diff_lines: number
  task_real_minutes: number
  task_ancient_minutes: number
  task_efficiency_ratio: number // 百分比口径
  commit_diff_lines: number
  commit_real_minutes: number
  commit_ancient_minutes: number
  commit_efficiency_ratio: number // 百分比口径
  upstream_tokens: number
  downstream_tokens: number
  cost: number
  [k: string]: unknown
}

/** /v2/dept-tree 节点（递归）。来自 dept-sync 权威全量部门树（透传）。 */
export interface DeptTreeNode {
  dept_id: string
  dept_name: string
  parent_dept_id: string
  dept_path: string
  dept_level: number
  order_num: number
  child_dept_count: number
  status: number
  children: DeptTreeNode[]
}

/** /v2/dept-tree/members 一名成员：dept-sync 花名册 + 左连看板 V2 指标（按 universal_id）。 */
export interface DeptMember {
  universal_id: string
  real_name: string
  emp_no: string
  position: string
  is_main: number
  has_kanban_data: boolean
  merged_need_count: number
  actual_calendar_min: number
  baseline_calendar_min: number
  calendar_ratio: number | null // 小数口径
  work_ratio: number | null // 小数口径
  ai_code_ratio?: number | null // 小数口径
  commit_count: number
  commit_diff_lines: number
  cost: number
}

/** /v2/dept-tree/members summary（该部门直属成员合计，小数提效比）。 */
export interface DeptMembersSummary {
  dept_id: string
  member_count: number
  kanban_member_count: number
  merged_need_count: number
  actual_calendar_min: number
  baseline_calendar_min: number
  calendar_ratio: number | null // 小数口径
  work_ratio: number | null // 小数口径
  ai_code_ratio?: number | null // 小数口径
  commit_count: number
  commit_diff_lines: number
  cost: number
}

/** /v2/dept-tree/members 顶层响应。 */
export interface DeptMembersResponse {
  summary: DeptMembersSummary
  members: DeptMember[]
}

/** /v2/orgs/detail 顶层响应（§Org-7）。 */
export interface OrgDetailResponse {
  org_path: string
  summary: OrgSummary | null
  commits: CommitTimeSeriesItem[] | null
  tasks: TaskTimeSeriesItem[] | null
  members: OrgMember[] | null
  granularity: string
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
  request_content?: string
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

/**
 * /v2/commits 列表项（PR4 §1.1，backend db.go CommitListItem）。
 * ⚠️ efficiency_ratio 是**百分比口径**（300=300%，直接 .toFixed(1)+'%'，不 ×100）。
 */
export interface CommitListItem {
  commit_id: string
  commit_time?: string | null
  repo_addr?: string
  repo_branch?: string
  git_user_name?: string
  git_user_email?: string
  user_id?: string
  user_name?: string
  client_id?: string
  work_dir?: string
  diff_lines?: number | null
  commit_ancient_minutes?: number | null
  commit_ancient_minutes_manual?: number | null
  commit_real_minutes?: number | null
  commit_real_minutes_manual?: number | null
  commit_real_ai_minutes?: number | null
  commit_real_ancient_minutes?: number | null
  comment?: string
  cost?: number | null
  upstream_tokens?: number | null
  downstream_tokens?: number | null
  silica?: number | null
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

/**
 * /v2/commits/{id} 的 commit 对象（models.Commit，详情用到的字段；指针字段可 null）。
 */
export interface CommitDetail {
  commit_id: string
  commit_time?: string | null
  repo_addr?: string
  repo_branch?: string
  git_user_name?: string
  git_user_email?: string
  user_id?: string
  user_name?: string
  comment?: string
  diff_lines?: number | null
  commit_ancient_minutes?: number | null
  commit_ancient_minutes_reason?: string
  commit_ancient_minutes_manual?: number | null
  commit_ancient_minutes_reason_manual?: string
  commit_real_minutes?: number | null
  commit_real_minutes_reason?: string
  commit_real_minutes_manual?: number | null
  commit_real_minutes_reason_manual?: string
  silica?: number | null
  efficiency_ratio?: number | null // 百分比口径
  [k: string]: unknown
}

/** /v2/commits/{id} 的 related_tasks 项（db.go RelatedTask，silica 0~1 要 ×100）。 */
export interface RelatedTask {
  task_id: string
  user_name?: string
  start_time?: string | null
  task_real_minutes?: number | null
  silica?: number | null // 0~1
  cost?: number | null
  diff_lines?: number | null
  [k: string]: unknown
}

/** /v2/commits/{id} 顶层响应（PR4 §1.2）。 */
export interface CommitDetailResponse {
  commit: CommitDetail
  related_tasks?: RelatedTask[]
  efficiency_ratio?: number | null // 顶层，百分比口径
  total_cost?: number | null
  silica?: number | null
  upstream_tokens?: number | null
  downstream_tokens?: number | null
}

/** PUT /v2/commits/{id}/manual 请求体（PR4 §1.2，4 字段）。 */
export interface UpdateCommitManualRequest {
  commit_ancient_minutes_manual: number | null
  commit_ancient_minutes_reason_manual: string
  commit_real_minutes_manual: number | null
  commit_real_minutes_reason_manual: string
}

// ============ Projects（PR4b，百分比口径；列表无分页） ============

/** 项目内 repo filter 配置（project.repos JSON 数组项）。 */
export interface ProjectRepo {
  repo_addr: string
  repo_branch: string
  start_time?: string | null
  end_time?: string | null
  exclude_commits?: string[] | null
  include_only_commits?: string[] | null
  [k: string]: unknown
}

/**
 * /v2/projects 列表项（PR4 §2.1 / §五，project_handler_v2.go ProjectListItem）。
 * ⚠️ efficiency_ratio 是**百分比口径**（300=300%），用 PercentPill。
 */
export interface ProjectListItem {
  project_id: string
  name: string
  description?: string
  repos?: ProjectRepo[] | null
  task_ids?: string[] | null
  task_ids_silica?: number[] | null
  start_time?: string | null
  end_time?: string | null
  start_time_manual?: string | null
  end_time_manual?: string | null
  upstream_tokens?: number | null
  downstream_tokens?: number | null
  cost?: number | null
  project_ancient_minutes?: number | null
  project_ancient_minutes_reason?: string
  project_ancient_minutes_manual?: number | null
  project_ancient_minutes_reason_manual?: string
  project_real_process_minutes?: number | null
  project_real_process_minutes_reason?: string
  project_real_process_minutes_manual?: number | null
  project_real_process_minutes_reason_manual?: string
  project_real_lead_minutes?: number | null
  project_real_lead_minutes_reason?: string
  project_real_lead_minutes_manual?: number | null
  project_real_lead_minutes_reason_manual?: string
  created_at?: string
  updated_at?: string
  repo_count?: number
  task_count?: number
  user_count?: number
  total_code_lines?: number
  actual_lines_per_day?: number | null
  efficiency_ratio?: number | null // 百分比口径
  [k: string]: unknown
}

/** /v2/projects/{id} 的 project 对象（models.Project，含 repos/task_ids/task_ids_silica）。 */
export interface ProjectModel extends ProjectListItem {
  repos?: ProjectRepo[] | null
  task_ids?: string[] | null
  task_ids_silica?: number[] | null
}

/** /v2/projects/{id} commits 项（ProjectCommitItem，silica 直接当百分比，不 ×100）。 */
export interface ProjectCommitItem {
  commit_id: string
  user_id?: string
  commit_time?: string | null
  repo_addr?: string
  repo_branch?: string
  user_name?: string
  git_user_name?: string
  diff_lines?: number | null
  comment?: string
  commit_ancient_minutes?: number | null
  commit_ancient_minutes_manual?: number | null
  commit_real_minutes?: number | null
  commit_real_minutes_manual?: number | null
  silica?: number | null
  cost?: number | null // 当前后端 ProjectCommitItem 未返回（恒 undefined）；保留供 userStats 1:1 对齐 Vue
  [k: string]: unknown
}

/** /v2/projects/{id} tasks 项（ProjectTaskItem）。 */
export interface ProjectTaskItem {
  task_id: string
  user_id?: string
  user_name?: string
  start_time?: string | null
  end_time?: string | null
  upstream_tokens?: number | null
  downstream_tokens?: number | null
  cost?: number | null
  task_ancient_minutes?: number | null
  task_ancient_minutes_manual?: number | null
  task_real_minutes?: number | null
  task_real_minutes_manual?: number | null
  title?: string
  work_dir?: string
  diff_lines?: number | null
  silica?: number | null
  accept_ratio?: number | null
  [k: string]: unknown
}

/** /v2/projects/{id} 顶层响应（PR4 §2.2）。 */
export interface ProjectDetailResponse {
  project: ProjectModel
  commits: ProjectCommitItem[] | null
  tasks: ProjectTaskItem[] | null
  members: OrgMember[] | null
  efficiency_ratio?: number | null
  user_count?: number
}

/** POST/PUT /v2/projects（创建/编辑） */
export interface CreateProjectRequest {
  name: string
  description?: string
}

/** PUT /v2/projects/{id}（必须回传 repos/task_ids/task_ids_silica 原值，否则后端清空）。 */
export interface UpdateProjectRequest {
  name: string
  description?: string
  repos: ProjectRepo[]
  task_ids: string[]
  task_ids_silica: number[]
}

/** PUT /v2/projects/{id}/manual（6 minutes/reason + start/end_time_manual）。 */
export interface UpdateProjectManualRequest {
  project_ancient_minutes_manual: number | null
  project_ancient_minutes_reason_manual: string
  project_real_process_minutes_manual: number | null
  project_real_process_minutes_reason_manual: string
  project_real_lead_minutes_manual: number | null
  project_real_lead_minutes_reason_manual: string
  start_time_manual: string | null
  end_time_manual: string | null
}

/** POST /v2/projects/{id}/tasks（task_ids + 同长 silica 数组）。 */
export interface AddTasksRequest {
  task_ids: string[]
  task_ids_silica: number[]
}

/** PUT /v2/projects/{id}/tasks/silica。 */
export interface UpdateTaskSilicaRequest {
  task_id: string
  silica: number
}

/** POST /v2/projects/{id}/repos（end_time 白名单 now → null）。 */
export interface AddRepoRequest {
  repo_addr: string
  repo_branch: string
  start_time?: string | null
  end_time?: string | null
  exclude_commits: string[]
  include_only_commits: string[]
}

/** POST /v2/projects/check-conflicts 响应项。 */
export interface ProjectConflict {
  commit_id: string
  project_id: string
  project_name: string
}

export interface CheckConflictsResponse {
  conflicts: ProjectConflict[]
}

/** POST /v2/projects（创建）响应（含 project_id）。 */
export interface CreateProjectResponse {
  project_id: string
  name?: string
  [k: string]: unknown
}

// ============================================================
// chat-indicator-statistics 代理类型（/api/v2/chat/* → chat 服务 /chat-indicator-statistics/api/v1/*）
// 字段对照其源码 pkg/model/models.go + pkg/http/handler/{realtime,handler}.go 的 json tag，勿凭空增删。
// ⚠️ chat 侧响应信封是 {success,code,data}（错误 400 + {error:{code,message,type}}），
//    与看板「裸数据 + {error:string}」不同 —— 走 client.ts 的 chatGet/chatPost/... 解包，不要混用 apiGet。
// ============================================================

/** GET /v2/chat/stats/realtime 响应 summary（realtime.go aggregateRealtime）。 */
export interface ChatRealtimeSummary {
  total_requests: number
  total_users: number
  total_prompt_tokens: number
  total_completion_tokens: number
  total_cache_tokens: number
  total_error_requests: number
  total_cost: number
}

/** token 分钟趋势项（time 形如 "HH:mm"）。 */
export interface ChatTokenTrendItem {
  time: string
  prompt_tokens: number
  completion_tokens: number
  cache_tokens: number
}

/** 缓存命中率分钟趋势项（rate 为百分比 0-100）。 */
export interface ChatCacheRateItem {
  time: string
  cache_tokens: number
  prompt_tokens: number
  rate: number
}

/** 模型请求分布项。 */
export interface ChatModelRequestItem {
  model: string
  request_count: number
  user_count: number
  prompt_tokens: number
  completion_tokens: number
  total_cost: number
}

/** Auto 路由细分项（percentage 为百分比 0-100，保留 1 位）。 */
export interface ChatAutoRouterItem {
  routed_model: string
  request_count: number
  percentage: number
}

/** 请求量分钟趋势项。 */
export interface ChatRequestTrendItem {
  time: string
  request_count: number
}

/** Top50 用户项。 */
export interface ChatTopUserItem {
  universal_id: string
  username: string
  request_count: number
  prompt_tokens: number
  completion_tokens: number
}

/** GET /v2/chat/stats/realtime（range=30m|1h|3h；服务端 10 秒限频）。 */
export interface ChatRealtimeResponse {
  summary: ChatRealtimeSummary
  token_trend: ChatTokenTrendItem[]
  cache_hit_rate: ChatCacheRateItem[]
  model_requests: ChatModelRequestItem[]
  auto_router_breakdown: ChatAutoRouterItem[]
  request_trend: ChatRequestTrendItem[]
  top_users: ChatTopUserItem[]
}

/** POST /v2/chat/stats/detail/query 请求体（realtime.go RawDataQuery；时间 ISO 8601，必填）。 */
export interface ChatDetailQueryReq {
  datasource_id?: string
  start_time: string
  end_time: string
  universal_id?: string
  request_id?: string
  /** true=仅错误，false=仅成功，缺省=全部 */
  has_error?: boolean
  model?: string
  routed_model?: string
  /** 默认/最大 100 */
  limit?: number
  /** 'asc' | 'desc'（默认 desc） */
  order?: string
}

/** 明细行（rawMetricItem；指针字段可为 null）。 */
export interface ChatDetailRow {
  id: number
  request_id: string
  user_id: string
  username: string | null
  universal_id: string | null
  ts: string
  prompt_tokens: number | null
  completion_tokens: number | null
  cache_tokens: number | null
  error_code: string | null
  model: string | null
  routed_model: string | null
  mode: string | null
  duration: number | null
  first_token_duration: number | null
  slow_chunk: number | null
  system_tokens: number | null
  user_tokens: number | null
  request_time: string | null
  end_time: string | null
}

export interface ChatDetailQueryResponse {
  total: number
  items: ChatDetailRow[]
}

/** model_pricing 行（models.go ModelPricing；pricing_mode ∈ token|request|hybrid）。 */
export interface ModelPricing {
  id: number
  model_name: string
  pricing_mode: string
  input_price_per_token: number | null
  output_price_per_token: number | null
  cache_price_per_token: number | null
  request_price: number | null
  currency: string
  exchange_rate: number | null
  original_currency: string | null
  original_input_price: number | null
  original_output_price: number | null
  original_cache_price: number | null
  original_request_price: number | null
  effective_date: string
  end_date: string | null
  notes: string | null
  created_at: string
}

/** 创建/编辑价格请求体（id/created_at 服务端生成）。 */
export type ModelPricingUpsert = Omit<ModelPricing, 'id' | 'created_at'> & { id?: number }

/** source_datasource 行（models.go SourceDatasource；source_type ∈ postgres|elasticsearch）。 */
export interface ChatDatasource {
  id: number
  name: string
  source_type: string
  is_enabled: boolean
  pg_host: string | null
  pg_port: number | null
  pg_database: string | null
  pg_schema: string | null
  pg_table: string | null
  pg_username: string | null
  pg_password: string | null
  pg_ssl_mode: string | null
  es_hosts: string | null
  es_username: string | null
  es_password: string | null
  es_index: string | null
  es_verify_certs: boolean | null
  es_scroll_duration: string | null
  max_open_conns: number | null
  max_idle_conns: number | null
  notes: string | null
  created_at: string
  updated_at: string | null
}

/** 创建/编辑数据源请求体。 */
export type ChatDatasourceUpsert = Partial<Omit<ChatDatasource, 'id' | 'created_at' | 'updated_at'>> &
  Pick<ChatDatasource, 'name' | 'source_type'>

/** POST /v2/chat/datasources/{id}/test 结果（注意：连接失败也是 HTTP 200，看 success）。 */
export interface ChatDatasourceTestResult {
  success: boolean
  message: string
  ping_ms: number
}

/** sync_task 行（models.go SyncTask；status ∈ pending|running|completed|failed|retrying）。 */
export interface ChatSyncTask {
  id: number
  task_id: string
  status: string
  req_start_time: string
  req_end_time: string
  total_gaps: number
  completed_gaps: number
  total_rows_processed: number
  total_rows_written: number
  error_message: string | null
  retry_count: number
  source_name: string
  started_at: string | null
  finished_at: string | null
  created_at: string
}

/** GET /v2/chat/sync/tasks 响应。 */
export interface ChatSyncTaskListResponse {
  total: number
  tasks: ChatSyncTask[]
}

/** POST /v2/chat/sync/tasks 请求体（时间 ISO 8601）。 */
export interface ChatSyncSubmitReq {
  start_time: string
  end_time: string
  source_id?: number
  /** 强制覆盖：预删该范围日期的汇总数据 */
  force?: boolean
}

/** POST /v2/chat/sync/tasks 响应（⚠️ 该端点信封是 {code:0,data}，chatPost 仍按 data 解包）。 */
export interface ChatSyncSubmitResponse {
  task_id: string
  status: string
  gaps: unknown[]
  source_id: number
  source_name: string
}

/** GET /v2/chat/sync/tasks/{task_id} 响应（progress 为百分比 0-100）。 */
export interface ChatSyncTaskStatus {
  task_id: string
  status: string
  progress: number
  total_gaps: number
  completed_gaps: number
  total_rows_processed: number
  total_rows_written: number
  error_message: string | null
  source_name: string
  started_at: string | null
  finished_at: string | null
}

/** GET/PUT /v2/chat/config —— KV 扁平 map（如 system_currency / exchange_rate_usd_cny）。 */
export type ChatSystemConfig = Record<string, string>

