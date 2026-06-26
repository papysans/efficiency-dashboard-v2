// 成本看板（部门树·视角切换）页面局部类型 —— 对齐后端 zhaoshang-show-data
// （/chat-indicator-statistics/api/v1/stats/departments/:dept_id/cost/*）。
// 仅本目录消费，不进 api/types.ts（沿用 usage 目录「页面局部 interface」惯例）。
// 字段严格对齐后端 handler（dept_cost_overview/models/team/user.go）的 gin.H 返回。

// ============================ 部门成本总览 ============================

/** /stats/departments/:dept_id/cost/overview —— 总成本 / Token 成本 / 缓存成本 / 人均日均千Token */
export interface CostOverviewResp {
  dept_id: string
  include_children: boolean
  total_cost: number // 总费用（实际扣费）
  input_cost: number // 输入 Token 费用
  output_cost: number // 输出 Token 费用
  cache_cost: number // 缓存费用
  request_cost: number // 请求费用（request/hybrid 计费模式）
  input_cost_pct: number // 输入费用占比
  output_cost_pct: number // 输出费用占比
  daily_avg_cost: number // 日均费用
  per_user_avg_cost: number // 人均费用
  per_1k_token_cost: number // 每千 Token 平均成本
  active_users: number
  total_tokens: number
  prompt_tokens: number
  completion_tokens: number
  cache_tokens: number
  period_days: number
  cache: {
    hit_input_tokens: number // 缓存命中输入 Token 量
    hit_input_cost: number // 缓存命中输入费用
    miss_input_tokens: number // 缓存未命中输入 Token 量
    miss_input_cost: number // 缓存未命中输入费用
    hit_rate_pct: number // 缓存命中率
    savings: number // 缓存节省费用
  }
}

/** /stats/departments/:dept_id/cost/period-compare —— 费用环比 */
export interface CostPeriodSpan {
  start: string
  end: string
  total_cost: number
  input_cost: number
  output_cost: number
  cache_cost: number
}
export interface CostPeriodCompareResp {
  current_period: CostPeriodSpan
  previous_period: CostPeriodSpan
  cost_change_pct: number // 总费用环比
  input_cost_change_pct: number
  output_cost_change_pct: number
}

// ============================ 模型成本 ============================

/** 模型单价（每千 Token，从计费系统 model_pricing 同步，按区间最新生效价） */
export interface CostUnitPrice {
  input_per_1k: number | null
  output_per_1k: number | null
  cache_per_1k: number | null
}

/** /stats/departments/:dept_id/cost/models —— 各模型费用 / 占比 / 单价 / 实际平均成本 */
export interface CostModelItem {
  model: string
  total_cost: number // 各模型费用
  input_cost: number
  output_cost: number
  cache_cost: number
  request_cost: number
  cost_pct: number // 各模型费用占比
  request_count: number
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  cache_tokens: number
  pricing_mode: string | null // token / request / hybrid
  unit_price: CostUnitPrice // 各模型单价
  actual_avg_cost_per_1k: number // 各模型实际平均成本 = total_cost/total_tokens*1000
}
export interface CostModelsResp {
  models: CostModelItem[]
}

export interface CostTrendPoint {
  date: string
  total_cost: number
}

/** /stats/departments/:dept_id/cost/model-trend —— 各模型每日费用（堆叠面积图） */
export interface CostModelTrendSeries {
  model: string
  data: CostTrendPoint[]
}
export interface CostModelTrendResp {
  series: CostModelTrendSeries[]
}

/** /stats/departments/:dept_id/cost/composition/models —— 模型费用构成占比（饼图） */
export interface CostCompositionItem {
  model: string
  total_cost: number
  cost_pct: number
}
export interface CostModelCompositionResp {
  items: CostCompositionItem[]
}

// ============================ 团队（子部门）成本 ============================

/** /stats/departments/:dept_id/cost/sub-departments —— 各团队（直接子部门）费用对比 */
export interface CostSubDeptItem {
  dept_id: string
  dept_name: string // 团队名
  total_cost: number // 各团队费用
  input_cost: number
  output_cost: number
  cache_cost: number
  cost_pct: number // 各团队费用占比
  active_users: number // 团队活跃用户数（团队人均 = total_cost/active_users 前端自算）
  total_tokens: number
}
export interface CostSubDeptResp {
  parent_dept_id: string
  items: CostSubDeptItem[]
}

/** /stats/departments/:dept_id/cost/team-trend —— 各团队每日费用（折线） */
export interface CostTeamTrendSeries {
  dept_id: string
  dept_name: string
  data: CostTrendPoint[]
}
export interface CostTeamTrendResp {
  series: CostTeamTrendSeries[]
}

/** /stats/departments/:dept_id/cost/composition/teams —— 团队费用构成占比（饼图） */
export interface CostTeamCompositionItem {
  dept_id: string
  dept_name: string
  total_cost: number
  cost_pct: number
}
export interface CostTeamCompositionResp {
  items: CostTeamCompositionItem[]
}

// ============================ 用户成本 ============================

/** /stats/departments/:dept_id/cost/users —— 部门内各用户成本（分页） */
export interface CostUserItem {
  universal_id: string
  username: string | null
  user_id?: string // 工号（可读标识，区别于 universal_id 用户唯一 id）
  total_cost: number // 各用户费用
  input_cost: number
  output_cost: number
  cache_cost: number
  request_cost: number
  cost_pct: number // 各用户费用占比
  request_count: number
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  cache_tokens: number
  active_days: number
}
export interface CostUsersResp {
  dept_id: string
  total: number
  page: number
  page_size: number
  users: CostUserItem[]
}

/** /stats/departments/:dept_id/cost/anomaly —— 异常检测 */
export interface CostAnomalyResp {
  dept_id: string
  daily_spike_count: number // 单日费用突增次数
  user_spike_count: number // 单用户费用突增次数（去重用户数）
  zero_cost_active_users: number // 费用为 0 的活跃用户数
  daily_spike_threshold: number
  user_spike_threshold: number
}
