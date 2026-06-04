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
  boundary_confidence?: number | null
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
