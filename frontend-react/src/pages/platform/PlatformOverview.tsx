// 平台·总览 —— chat-indicator-statistics 历史汇总表的玻璃拟态重写（design §2.2 /platform/overview 行，
// 对照 zhaoshang-show-data Dashboard.jsx 的交互）。
// 3 个 Tab：全局趋势 / 模型与成本 / 用户分析。
// 数据全部经 /api/v2/chat 代理走 chatGet。
import { useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { chatGet } from '@/api/client'
import { chatStats } from '@/api/endpoints'
import { useGlobalConfig } from '@/api/queries'
import { useTheme } from '@/hooks/useTheme'
import { useUserNameMap } from '@/hooks/useUserNameMap'
import { EChart } from '@/components/charts/EChart'
import { getPalette } from '@/components/charts/chartTheme'
import { formatNumber } from '@/lib/formatters'
import { Modal } from '@/components/ui/Modal'
import SettingsLayout, { BTN_GLASS, ChatDisabledNotice } from '@/pages/settings/SettingsLayout'
import { ChartCard, ChatUserCell, EmptyHint, PIE_COLORS, multiAreaOption, shortToken } from './platformShared'
import PerformanceTab from './PerformanceTab'
import TimeDistributionTab from './TimeDistributionTab'

// ---- 页面局部类型 ----

interface ChatDailyGlobal {
  date: string
  total_requests: number
  total_users: number
  total_error_requests: number
  error_rate: number | null
  unique_task_count: number
  total_requests_including_errors: number
  sum_prompt_tokens: number
  sum_completion_tokens: number
  sum_total_tokens: number
  sum_cache_tokens: number
  avg_duration_ms: number | null
  avg_first_token_duration_ms: number | null
  avg_token_output_speed: number | null
  estimated_total_cost: number | null
  auto_router_breakdown_global?: string | null
}

interface ChatCostTrendRow {
  date: string
  total_cost: number
  input_cost: number
  output_cost: number
  cache_cost: number
  request_cost: number
  total_requests: number
  model?: string
}

interface ChatCacheHitRateRow {
  date: string
  sum_cache_tokens: number
  sum_prompt_tokens: number
  cache_hit_rate_pct: number
}

interface ChatModelCostRow {
  model: string
  total_requests: number
  total_input_tokens: number
  total_output_tokens: number
  total_cost: number
}

interface ChatDimensionRow {
  dimension_value: string
  total_requests: number
  total_users: number
  total_prompt_tokens: number
  total_completion_tokens: number
  total_cache_tokens: number
  avg_first_token_duration_ms: number | null
  avg_duration_ms: number | null
  avg_token_output_speed: number | null
  error_rate: number | null
}

interface ChatUserRankingRow {
  universal_id: string
  username: string | null
  total_requests: number
  success_requests: number
  error_requests: number
  sum_prompt_tokens: number
  sum_completion_tokens: number
  sum_total_tokens: number
  sum_cache_tokens: number
  unique_task_count: number
  active_days: number
  estimated_total_cost: number
  avg_duration_ms: number
  error_rate: number
  max_duration_ms: number
  avg_token_output_speed: number
}

interface ChatUsersRankingResp {
  total: number
  page: number
  page_size: number
  data: ChatUserRankingRow[]
}

interface ChatModelUsageItem {
  model: string
  request_count: number
  request_pct: number
  total_tokens: number
  token_pct: number
}
interface ChatModelsUsageResp {
  models: ChatModelUsageItem[]
}

// /stats/dimension?dimension_type=error_code —— 错误码维度（平台扩 ETL 后产出；error_code 行 total_requests 恒 0，按 error_requests 计分布）
interface ChatErrorCodeRow {
  dimension_value: string
  error_requests: number
  total_requests_including_errors: number
  error_rate: number | null
}

// ---- 日期工具 ----

function pad2(n: number): string { return String(n).padStart(2, '0') }
function toDateStr(d: Date): string { return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}` }
function rangeForDays(days: number): { start: string; end: string } {
  const end = new Date()
  const start = new Date()
  start.setDate(end.getDate() - (days - 1))
  return { start: toDateStr(start), end: toDateStr(end) }
}
function shortDate(s: string): string { return s.slice(5, 10) }

// ---- 常量 ----

const PRESETS = [
  { label: '近7天', days: 7 },
  { label: '近30天', days: 30 },
  { label: '近90天', days: 90 },
]

const USER_SORTS = [
  { value: 'sum_total_tokens', label: 'Token 总量' },
  { value: 'total_requests', label: '请求数' },
  { value: 'estimated_total_cost', label: '成本' },
]

const TABS = [
  { key: 'global', label: '全局趋势' },
  { key: 'perf', label: '请求性能' },
  { key: 'timedist', label: '时段分布' },
  { key: 'models', label: '模型与成本' },
  { key: 'users', label: '用户分析' },
] as const

type TabKey = (typeof TABS)[number]['key']

const TH = 'px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_NUM = 'px-3 py-2 text-right font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TD = 'px-3 py-2 align-middle text-gray-700 dark:text-gray-200 whitespace-nowrap'
const TD_NUM = 'px-3 py-2 align-middle text-right tabular-nums text-gray-700 dark:text-gray-200'

const INPUT_CLS =
  'glass rounded-lg px-3 py-1.5 text-sm bg-transparent text-gray-900 dark:text-white placeholder-gray-400 ' +
  'focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue'

const fmtMs = (v: number | null | undefined) => (v != null ? `${Number(v).toFixed(0)} ms` : '-')
const fmtPct = (v: number | null | undefined) => (v != null ? `${(v * 100).toFixed(2)}%` : '-')

export default function PlatformOverview() {
  const { theme } = useTheme()
  const { data: gc } = useGlobalConfig()
  const chatEnabled = gc?.chat_stats_enabled === true
  const chatDisabled = !!gc && !chatEnabled

  const [{ start, end }, setRange] = useState(() => rangeForDays(30))
  const [presetDays, setPresetDays] = useState<number | null>(30)
  const rangeValid = !!start && !!end && start <= end

  // Tab 状态
  const [tab, setTab] = useState<TabKey>('global')

  // 自动刷新（Q4 实时·轻方案：开启后各查询按间隔 refetch；30s ≥ realtime 全局 10s 限频）
  const [autoRefresh, setAutoRefresh] = useState(false)
  const refetchInterval: number | false = autoRefresh ? 30000 : false

  // 成本趋势模型筛选
  const [costModel, setCostModel] = useState('all')

  // 模型趋势多选
  const [trendModels, setTrendModels] = useState<string[]>([])

  // 用户排行搜索 + 排序
  const [userSort, setUserSort] = useState('sum_total_tokens')
  const [searchInput, setSearchInput] = useState('')
  const [search, setSearch] = useState('')
  useEffect(() => {
    const id = window.setTimeout(() => setSearch(searchInput.trim()), 400)
    return () => window.clearTimeout(id)
  }, [searchInput])

  // 用户详情 Modal
  const [userModal, setUserModal] = useState<{ open: boolean; uid: string; username: string }>({ open: false, uid: '', username: '' })

  const enabled = chatEnabled && rangeValid

  const dailyQ = useQuery({
    queryKey: ['chat-overview-daily', start, end],
    queryFn: () => chatGet<ChatDailyGlobal[]>('/stats/global/daily', { start_date: start, end_date: end }),
    enabled,
    refetchInterval,
  })
  const costQ = useQuery({
    queryKey: ['chat-overview-cost-trend', start, end, costModel],
    queryFn: () => chatGet<ChatCostTrendRow[]>('/stats/cost-trend', { start_date: start, end_date: end, model: costModel }),
    enabled: enabled && (tab === 'global' || tab === 'models'),
    refetchInterval,
  })
  const cacheQ = useQuery({
    queryKey: ['chat-overview-cache-rate', start, end],
    queryFn: () => chatGet<ChatCacheHitRateRow[]>('/stats/cache-hit-rate', { start_date: start, end_date: end }),
    enabled: enabled && tab === 'global',
    refetchInterval,
  })
  const rankQ = useQuery({
    queryKey: ['chat-overview-model-ranking', start, end],
    queryFn: () => chatGet<ChatModelCostRow[]>('/stats/models/cost-ranking', { start_date: start, end_date: end }),
    enabled: enabled && (tab === 'global' || tab === 'models'),
    refetchInterval,
  })
  const dimQ = useQuery({
    queryKey: ['chat-overview-dimension', start, end],
    queryFn: () => chatGet<ChatDimensionRow[]>('/stats/dimension', { start_date: start, end_date: end, dimension_type: 'routed_model' }),
    enabled: enabled && tab === 'models',
    refetchInterval,
  })
  const modelTrendQ = useQuery({
    queryKey: ['chat-overview-model-trend', start, end, trendModels],
    queryFn: () => chatStats.modelTrend({ start_date: start, end_date: end, models: trendModels.join(',') }),
    enabled: enabled && tab === 'models' && trendModels.length > 0,
    refetchInterval,
  })
  const modelsUsageQ = useQuery({
    queryKey: ['chat-overview-models-usage', start, end],
    queryFn: () => chatGet<ChatModelsUsageResp>('/stats/models/usage', { start_date: start, end_date: end }),
    enabled: enabled && tab === 'models',
    refetchInterval,
  })
  const errorCodeQ = useQuery({
    queryKey: ['chat-overview-error-codes', start, end],
    queryFn: () =>
      chatGet<ChatErrorCodeRow[]>('/stats/dimension', {
        start_date: start,
        end_date: end,
        dimension_type: 'error_code',
        sort_order: 'desc',
      }),
    enabled: enabled && tab === 'models',
    refetchInterval,
  })
  const usersQ = useQuery({
    queryKey: ['chat-overview-users-ranking', start, end, userSort, search],
    queryFn: () => chatGet<ChatUsersRankingResp>('/stats/users/ranking', {
      start_date: start, end_date: end, sort_by: userSort, page: 1, page_size: 50,
      ...(search ? { search } : {}),
    }),
    enabled: enabled && tab === 'users',
    refetchInterval,
  })

  const { resolveName } = useUserNameMap()
  const p = useMemo(() => getPalette(theme), [theme])
  const daily = useMemo(() => dailyQ.data ?? [], [dailyQ.data])

  // ---- KPI 区间合计 ----
  const agg = useMemo(() => {
    const sum = (fn: (r: ChatDailyGlobal) => number | null | undefined) => daily.reduce((s, r) => s + (fn(r) || 0), 0)
    const requests = sum((r) => r.total_requests)
    const requestsIncErr = sum((r) => r.total_requests_including_errors > 0 ? r.total_requests_including_errors : r.total_requests)
    const errors = sum((r) => r.total_error_requests)
    return {
      requests, errors,
      errorRate: requestsIncErr > 0 ? errors / requestsIncErr : null,
      promptTokens: sum((r) => r.sum_prompt_tokens),
      completionTokens: sum((r) => r.sum_completion_tokens),
      cacheTokens: sum((r) => r.sum_cache_tokens),
      cost: sum((r) => r.estimated_total_cost),
      avgUsers: daily.length > 0 ? Math.round(sum((r) => r.total_users) / daily.length) : 0,
      peakUsers: daily.reduce((m, r) => Math.max(m, r.total_users), 0),
      avgRequests: daily.length > 0 ? Math.round(requests / daily.length) : 0,
    }
  }, [daily])

  const kpis = [
    { title: '总请求', value: formatNumber(agg.requests), sub: `日均 ${formatNumber(agg.avgRequests)}` },
    { title: '活跃用户（日均）', value: formatNumber(agg.avgUsers), sub: `单日峰值 ${formatNumber(agg.peakUsers)}` },
    { title: '输入 Token', value: shortToken(agg.promptTokens), full: formatNumber(agg.promptTokens) },
    { title: '输出 Token', value: shortToken(agg.completionTokens), full: formatNumber(agg.completionTokens) },
    { title: '缓存 Token', value: shortToken(agg.cacheTokens), full: formatNumber(agg.cacheTokens) },
    { title: '错误率', value: fmtPct(agg.errorRate), sub: `错误请求 ${formatNumber(agg.errors)}`, alert: (agg.errorRate ?? 0) > 0.05 },
    { title: '总成本', value: `¥${agg.cost.toFixed(2)}`, sub: '估算（按价格表）' },
  ]

  // ---- 图表 option ----
  const costOpt = useMemo(() => {
    const rows = costQ.data ?? []
    return multiAreaOption(p,
      rows.map((r) => shortDate(r.date)),
      [
        { name: '总成本', color: '#ff3b30', data: rows.map((r) => +r.total_cost.toFixed(2)) },
        { name: '输入成本', color: '#0071e3', data: rows.map((r) => +r.input_cost.toFixed(2)) },
        { name: '输出成本', color: '#34c759', data: rows.map((r) => +r.output_cost.toFixed(2)) },
        { name: '缓存成本', color: '#af52de', data: rows.map((r) => +r.cache_cost.toFixed(2)) },
      ],
      { yFmt: (v) => `¥${shortToken(v)}` },
    )
  }, [costQ.data, p])

  const tokenOpt = useMemo(() => multiAreaOption(p,
    daily.map((r) => shortDate(r.date)),
    [
      { name: '输入 Token', color: '#0071e3', data: daily.map((r) => r.sum_prompt_tokens) },
      { name: '输出 Token', color: '#34c759', data: daily.map((r) => r.sum_completion_tokens) },
      { name: '缓存 Token', color: '#5ac8fa', data: daily.map((r) => r.sum_cache_tokens) },
    ],
    { yFmt: (v) => shortToken(v) },
  ), [daily, p])

  const requestOpt = useMemo(() => multiAreaOption(p,
    daily.map((r) => shortDate(r.date)),
    [
      { name: '请求量', color: '#ff9500', data: daily.map((r) => r.total_requests) },
      { name: '错误请求', color: '#ff3b30', data: daily.map((r) => r.total_error_requests) },
    ],
    { yFmt: (v) => shortToken(v) },
  ), [daily, p])

  const cacheOpt = useMemo(() => {
    const rows = cacheQ.data ?? []
    return multiAreaOption(p,
      rows.map((r) => shortDate(r.date)),
      [{ name: '缓存命中率', color: '#34c759', data: rows.map((r) => +r.cache_hit_rate_pct.toFixed(1)) }],
      { yFmt: (v) => `${v}%`, yMax: 100 },
    )
  }, [cacheQ.data, p])

  // ---- 模型与成本 ----

  // Auto 路由合并
  const mergedAutoRouter = useMemo(() => {
    const merged: Record<string, number> = {}
    for (const d of daily) {
      if (!d.auto_router_breakdown_global) continue
      try {
        const prefs = JSON.parse(d.auto_router_breakdown_global)
        for (const [model, count] of Object.entries(prefs)) {
          merged[model] = (merged[model] || 0) + (count as number)
        }
      } catch { /* ignore */ }
    }
    const entries = Object.entries(merged).sort((a, b) => b[1] - a[1])
    return entries.length > 0 ? entries : null
  }, [daily])

  const autoRouterPieOpt = useMemo(() => {
    if (!mergedAutoRouter) return null
    return {
      tooltip: { trigger: 'item' as const },
      legend: { bottom: 0, textStyle: { fontSize: 11 } },
      series: [{
        type: 'pie' as const, radius: ['45%', '72%'], center: ['50%', '45%'],
        label: { formatter: '{b}\n{d}%', fontSize: 10 },
        data: mergedAutoRouter.map(([name, value], i) => ({ name, value, itemStyle: { color: PIE_COLORS[i % PIE_COLORS.length] } })),
      }],
    }
  }, [mergedAutoRouter])

  // 模型请求占比 vs Token 占比 并列双饼（/stats/models/usage）
  const modelsUsagePies = useMemo(() => {
    const ms = modelsUsageQ.data?.models ?? []
    const pie = (valueKey: 'request_count' | 'total_tokens') => ({
      tooltip: { trigger: 'item' as const, formatter: '{b}<br/>{c} ({d}%)' },
      legend: { bottom: 0, type: 'scroll' as const, textStyle: { fontSize: 10 } },
      series: [{
        type: 'pie' as const, radius: ['45%', '72%'], center: ['50%', '45%'],
        label: { formatter: '{b}\n{d}%', fontSize: 10 },
        data: ms.map((m, i) => ({ name: m.model, value: m[valueKey], itemStyle: { color: PIE_COLORS[i % PIE_COLORS.length] } })),
      }],
    })
    return ms.length === 0 ? { req: null, tok: null } : { req: pie('request_count'), tok: pie('total_tokens') }
  }, [modelsUsageQ.data])

  // 错误码分布（#12）：error_code 维度 total_requests 恒 0，按 error_requests 降序取 Top 15。
  const errorCodeOpt = useMemo(() => {
    const rows = [...(errorCodeQ.data ?? [])].sort((a, b) => b.error_requests - a.error_requests).slice(0, 15)
    if (rows.length === 0) return null
    return {
      tooltip: { trigger: 'axis' as const, axisPointer: { type: 'shadow' as const } },
      grid: { left: 8, right: 16, top: 8, bottom: 24, containLabel: true },
      xAxis: {
        type: 'category' as const,
        data: rows.map((r) => r.dimension_value || '未知'),
        axisLabel: { color: p.textColor, fontSize: 11 },
      },
      yAxis: {
        type: 'value' as const,
        axisLabel: { color: p.textColor },
        splitLine: { lineStyle: { color: p.splitLineColor } },
      },
      series: [
        {
          name: '错误次数',
          type: 'bar' as const,
          data: rows.map((r) => r.error_requests),
          itemStyle: { color: '#ff3b30', borderRadius: [3, 3, 0, 0] },
        },
      ],
    }
  }, [errorCodeQ.data, p])

  // 模型趋势 option
  const modelTrendOpt = useMemo(() => {
    const series = modelTrendQ.data ?? []
    if (series.length === 0) return null
    const dateSet = new Set<string>()
    series.forEach(s => (s.data || []).forEach(d => dateSet.add(shortDate(d.date))))
    const dates = Array.from(dateSet).sort()
    const seriesOpt = series.map((s, i) => ({
      name: `${s.model} 请求`,
      type: 'line' as const,
      data: dates.map(date => {
        const row = (s.data || []).find(d => shortDate(d.date) === date)
        return row?.total_requests ?? 0
      }),
      smooth: true,
      symbol: 'circle',
      symbolSize: 4,
      lineStyle: { width: 2 },
      itemStyle: { color: PIE_COLORS[i % PIE_COLORS.length] },
    }))
    return {
      tooltip: { trigger: 'axis' as const },
      legend: { type: 'scroll' as const, bottom: 0, textStyle: { fontSize: 10 } },
      grid: { left: 50, right: 16, top: 8, bottom: 40 },
      xAxis: { type: 'category' as const, data: dates, axisLabel: { fontSize: 10 } },
      yAxis: { type: 'value' as const, axisLabel: { formatter: (v: number) => shortToken(v) } },
      series: seriesOpt,
    }
  }, [modelTrendQ.data])

  const costModelOptions = useMemo(() => ['all', ...(rankQ.data ?? []).map((r) => r.model).filter(Boolean)], [rankQ.data])
  const availableModels = useMemo(() => {
    const set = new Set<string>()
    ;(dimQ.data ?? []).forEach(d => { if (d.dimension_value) set.add(d.dimension_value) })
    return Array.from(set).sort()
  }, [dimQ.data])

  const userRows = usersQ.data?.data ?? []

  const loading = enabled && dailyQ.isLoading
  const queries = [dailyQ, costQ, cacheQ, rankQ, dimQ, modelTrendQ, modelsUsageQ, errorCodeQ, usersQ]
  const errors = queries.filter((q) => q.isError).map((q) => (q.error as Error).message)

  const presetBtn = (active: boolean) =>
    `px-3 py-1.5 rounded-lg text-sm font-medium cursor-pointer transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue ${
      active ? 'bg-apple-blue text-white' : 'glass text-gray-600 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white'
    }`

  const tabBtn = (t: TabKey) =>
    `px-4 py-2 text-sm font-medium rounded-lg cursor-pointer transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue ${
      tab === t ? 'bg-apple-blue text-white' : 'glass text-gray-600 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white'
    }`

  const header = (
    <header>
      <h2 className="text-lg font-semibold text-gray-900 dark:text-white">平台总览</h2>
      <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
        基于日汇总表的历史指标：请求 / token / 成本 / 错误 / 模型与用户排行。
      </p>
    </header>
  )

  if (chatDisabled) {
    return (
      <SettingsLayout>
        <div className="space-y-5">{header}<ChatDisabledNotice /></div>
      </SettingsLayout>
    )
  }

  return (
    <SettingsLayout>
      <div className="space-y-5">
        {header}

        {/* 工具栏 */}
        <div className="flex flex-wrap items-center gap-2">
          {PRESETS.map((o) => (
            <button key={o.days} type="button" onClick={() => { setPresetDays(o.days); setRange(rangeForDays(o.days)) }}
              className={presetBtn(presetDays === o.days)} aria-pressed={presetDays === o.days}>
              {o.label}
            </button>
          ))}
          <label className="flex items-center gap-1.5 text-sm text-gray-500 dark:text-gray-400 ml-2">
            <span>从</span>
            <input type="date" value={start} max={end || undefined}
              onChange={(e) => { setPresetDays(null); setRange((r) => ({ ...r, start: e.target.value })) }}
              className={INPUT_CLS} aria-label="开始日期" />
            <span>至</span>
            <input type="date" value={end} min={start || undefined}
              onChange={(e) => { setPresetDays(null); setRange((r) => ({ ...r, end: e.target.value })) }}
              className={INPUT_CLS} aria-label="结束日期" />
          </label>
          <label className="flex items-center gap-1.5 text-sm text-gray-500 dark:text-gray-400 ml-auto cursor-pointer select-none whitespace-nowrap">
            <input type="checkbox" checked={autoRefresh} onChange={(e) => setAutoRefresh(e.target.checked)} className="accent-apple-blue" />
            自动刷新{autoRefresh ? '（30s）' : ''}
          </label>
          {!rangeValid && <span className="text-sm text-rose-600 dark:text-rose-400">请选择有效的起止日期（开始 ≤ 结束）</span>}
        </div>

        {errors.length > 0 && (
          <div className="glass rounded-xl px-4 py-3 text-sm text-rose-600 dark:text-rose-400 bg-rose-50/50 dark:bg-rose-900/20">
            {[...new Set(errors)].join('；')}
          </div>
        )}

        {loading ? (
          <OverviewSkeleton />
        ) : (
          <>
            {/* KPI 卡行 */}
            <div className="grid grid-cols-2 sm:grid-cols-4 xl:grid-cols-7 gap-3">
              {kpis.map((k) => (
                <div key={k.title} className="glass rounded-2xl p-4 hover:shadow-lg transition-shadow">
                  <div className="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide mb-1">{k.title}</div>
                  <div className={`text-2xl font-bold tabular-nums ${k.alert ? 'text-rose-600 dark:text-rose-400' : 'text-gray-900 dark:text-white'}`} title={k.full}>
                    {k.value}
                  </div>
                  {k.sub && <div className="text-xs text-gray-400 dark:text-gray-500 mt-1">{k.sub}</div>}
                </div>
              ))}
            </div>

            {/* Tab 导航 */}
            <div className="flex gap-2">
              {TABS.map((t) => (
                <button key={t.key} type="button" onClick={() => setTab(t.key)} className={tabBtn(t.key)}>{t.label}</button>
              ))}
            </div>

            {/* ---- 全局趋势 Tab ---- */}
            {tab === 'global' && (
              <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                <ChartCard title="成本趋势" sub="按日 · 估算" extra={
                  <select value={costModel} onChange={(e) => setCostModel(e.target.value)} className={INPUT_CLS}>
                    <option value="all">全部模型</option>
                    {costModelOptions.filter(m => m !== 'all').map(m => <option key={m} value={m}>{m}</option>)}
                  </select>
                }>
                  {(costQ.data ?? []).length > 0 ? <EChart option={costOpt} height={280} /> : <EmptyHint />}
                </ChartCard>
                <ChartCard title="Token 趋势" sub="按日 · 输入 / 输出 / 缓存">
                  {daily.length > 0 ? <EChart option={tokenOpt} height={280} /> : <EmptyHint />}
                </ChartCard>
                <ChartCard title="请求量趋势" sub="按日 · 含错误请求">
                  {daily.length > 0 ? <EChart option={requestOpt} height={260} /> : <EmptyHint />}
                </ChartCard>
                <ChartCard title="缓存命中率趋势" sub="按日 · cache / prompt">
                  {(cacheQ.data ?? []).length > 0 ? <EChart option={cacheOpt} height={260} /> : <EmptyHint />}
                </ChartCard>
              </div>
            )}

            {/* ---- 请求性能 Tab ---- */}
            {tab === 'perf' && (
              <PerformanceTab start={start} end={end} enabled={!!enabled} refetchInterval={refetchInterval} />
            )}

            {/* ---- 时段分布 Tab ---- */}
            {tab === 'timedist' && (
              <TimeDistributionTab start={start} end={end} enabled={!!enabled} refetchInterval={refetchInterval} />
            )}

            {/* ---- 模型与成本 Tab ---- */}
            {tab === 'models' && (
              <div className="space-y-4">
                {/* 模型请求占比 vs Token 占比 并列 */}
                <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                  <ChartCard title="模型请求占比" sub="按请求次数">
                    {modelsUsagePies.req ? <EChart option={modelsUsagePies.req} height={300} /> : <EmptyHint />}
                  </ChartCard>
                  <ChartCard title="模型 Token 占比" sub="按总 Token">
                    {modelsUsagePies.tok ? <EChart option={modelsUsagePies.tok} height={300} /> : <EmptyHint />}
                  </ChartCard>
                </div>

                {/* 成本变化曲线 */}
                <ChartCard title="成本变化曲线" sub="每日总成本 + 构成分析" extra={
                  <select value={costModel} onChange={(e) => setCostModel(e.target.value)} className={INPUT_CLS}>
                    <option value="all">总体</option>
                    {availableModels.map(m => <option key={m} value={m}>{m}</option>)}
                  </select>
                }>
                  {(costQ.data ?? []).length > 0 ? <EChart option={costOpt} height={260} /> : <EmptyHint />}
                </ChartCard>

                {/* 模型请求/Token 趋势 */}
                <ChartCard title="模型请求量趋势" sub="选择模型对比" extra={
                  <select multiple value={trendModels}
                    onChange={(e) => setTrendModels(Array.from(e.target.selectedOptions, o => o.value))}
                    className={`${INPUT_CLS} min-w-[200px]`} size={Math.min(4, availableModels.length || 1)}>
                    {availableModels.map(m => <option key={m} value={m}>{m}</option>)}
                  </select>
                }>
                  {modelTrendQ.isLoading ? (
                    <div className="py-8 text-center text-sm text-gray-400">加载中...</div>
                  ) : trendModels.length === 0 ? (
                    <div className="py-8 text-center text-sm text-gray-400">请在上方选择模型以查看趋势对比</div>
                  ) : modelTrendOpt ? (
                    <EChart option={modelTrendOpt} height={280} />
                  ) : (
                    <EmptyHint />
                  )}
                </ChartCard>

                <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                  {/* Auto 路由分布 */}
                  <ChartCard title="Auto 路由实际命中模型分布" sub="区间合并">
                    {autoRouterPieOpt ? <EChart option={autoRouterPieOpt} height={300} /> : <EmptyHint />}
                  </ChartCard>

                  {/* 按路由模型汇总表 */}
                  <ChartCard title="按路由模型汇总" sub="可排序">
                    <div className="overflow-x-auto max-h-[360px] overflow-y-auto">
                      <table className="w-full text-sm border-collapse">
                        <thead className="sticky top-0 bg-white/70 dark:bg-gray-900/70 backdrop-blur">
                          <tr className="border-b border-gray-200/50 dark:border-white/10">
                            <th className={TH}>模型</th>
                            <th className={TH_NUM}>请求数</th>
                            <th className={TH_NUM}>用户数</th>
                            <th className={TH_NUM}>输入Token</th>
                            <th className={TH_NUM}>输出Token</th>
                            <th className={TH_NUM}>时延</th>
                            <th className={TH_NUM}>错误率</th>
                          </tr>
                        </thead>
                        <tbody>
                          {(dimQ.data ?? []).length === 0 ? (
                            <tr><td colSpan={7}><EmptyHint compact /></td></tr>
                          ) : (
                            (dimQ.data ?? []).map((d) => (
                              <tr key={d.dimension_value} className="border-b border-gray-100/50 dark:border-white/5">
                                <td className={TD}>{d.dimension_value || '-'}</td>
                                <td className={TD_NUM}>{formatNumber(d.total_requests)}</td>
                                <td className={TD_NUM}>{formatNumber(d.total_users)}</td>
                                <td className={TD_NUM} title={formatNumber(d.total_prompt_tokens)}>{shortToken(d.total_prompt_tokens)}</td>
                                <td className={TD_NUM} title={formatNumber(d.total_completion_tokens)}>{shortToken(d.total_completion_tokens)}</td>
                                <td className={TD_NUM}>{fmtMs(d.avg_duration_ms)}</td>
                                <td className={`${TD_NUM} ${(d.error_rate ?? 0) > 0.05 ? 'text-rose-600' : ''}`}>{fmtPct(d.error_rate)}</td>
                              </tr>
                            ))
                          )}
                        </tbody>
                      </table>
                    </div>
                  </ChartCard>
                </div>

                {/* 错误码分布（#12，需平台扩 error_code ETL 维度 + 重跑 sync 回填后有数据） */}
                <ChartCard title="错误码分布" sub="按错误次数 · Top 15 · 需平台 ETL 含 error_code 维度">
                  {errorCodeOpt ? <EChart option={errorCodeOpt} height={280} /> : <EmptyHint />}
                </ChartCard>

                {/* 模型成本明细表 */}
                <ChartCard title="模型成本明细" sub="按累计成本降序">
                  <div className="overflow-x-auto">
                    <table className="w-full text-sm border-collapse">
                      <thead>
                        <tr className="border-b border-gray-200/50 dark:border-white/10">
                          <th className={TH_NUM}>排名</th>
                          <th className={TH}>模型</th>
                          <th className={TH_NUM}>请求数</th>
                          <th className={TH_NUM}>输入 Token</th>
                          <th className={TH_NUM}>输出 Token</th>
                          <th className={TH_NUM}>总成本（¥）</th>
                        </tr>
                      </thead>
                      <tbody>
                        {(rankQ.data ?? []).length === 0 ? (
                          <tr><td colSpan={6}><EmptyHint compact /></td></tr>
                        ) : (
                          (rankQ.data ?? []).map((m, i) => (
                            <tr key={m.model || i} className="border-b border-gray-100/50 dark:border-white/5">
                              <td className={TD_NUM}>{i + 1}</td>
                              <td className={TD}>{m.model || '-'}</td>
                              <td className={TD_NUM}>{formatNumber(m.total_requests)}</td>
                              <td className={TD_NUM} title={formatNumber(m.total_input_tokens)}>{shortToken(m.total_input_tokens)}</td>
                              <td className={TD_NUM} title={formatNumber(m.total_output_tokens)}>{shortToken(m.total_output_tokens)}</td>
                              <td className={TD_NUM}>{m.total_cost.toFixed(2)}</td>
                            </tr>
                          ))
                        )}
                      </tbody>
                    </table>
                  </div>
                </ChartCard>
              </div>
            )}

            {/* ---- 用户分析 Tab ---- */}
            {tab === 'users' && (
              <ChartCard
                title="用户排行"
                sub={`区间聚合 · Top 50${usersQ.data ? ` · 共 ${formatNumber(usersQ.data.total)} 人` : ''}`}
                extra={
                  <>
                    <input type="search" value={searchInput} onChange={(e) => setSearchInput(e.target.value)}
                      placeholder="搜索 ID / 用户名" className={INPUT_CLS} aria-label="搜索用户" />
                    <select value={userSort} onChange={(e) => setUserSort(e.target.value)} className={INPUT_CLS} aria-label="用户排行排序字段">
                      {USER_SORTS.map(s => <option key={s.value} value={s.value}>按{s.label}</option>)}
                    </select>
                  </>
                }
              >
                <div className="overflow-x-auto max-h-[520px] overflow-y-auto">
                  <table className="w-full text-sm border-collapse">
                    <thead className="sticky top-0 bg-white/70 dark:bg-gray-900/70 backdrop-blur">
                      <tr className="border-b border-gray-200/50 dark:border-white/10">
                        <th className={TH_NUM}>排名</th>
                        <th className={TH}>Universal ID</th>
                        <th className={TH}>用户名</th>
                        <th className={TH_NUM}>请求数</th>
                        <th className={TH_NUM}>总 Token</th>
                        <th className={TH_NUM}>输入 Token</th>
                        <th className={TH_NUM}>输出 Token</th>
                        <th className={TH_NUM}>缓存 Token</th>
                        <th className={TH_NUM}>成本（¥）</th>
                        <th className={TH_NUM}>会话数</th>
                        <th className={TH_NUM}>活跃天数</th>
                        <th className={TH_NUM}>平均时延</th>
                        <th className={TH_NUM}>错误率</th>
                      </tr>
                    </thead>
                    <tbody>
                      {usersQ.isFetching && userRows.length === 0 ? (
                        <tr><td colSpan={13} className="py-10 text-center text-sm text-gray-400">加载中…</td></tr>
                      ) : userRows.length === 0 ? (
                        <tr><td colSpan={13}><EmptyHint compact /></td></tr>
                      ) : (
                        userRows.map((u, i) => (
                          <tr key={u.universal_id || i} className="border-b border-gray-100/50 dark:border-white/5">
                            <td className={TD_NUM}>{i + 1}</td>
                            <td className={`${TD} font-mono text-xs`}>
                              <button type="button"
                                onClick={() => setUserModal({ open: true, uid: u.universal_id, username: u.username || u.universal_id })}
                                className="text-apple-blue hover:underline font-medium cursor-pointer bg-transparent border-none p-0">
                                {u.universal_id || '-'}
                              </button>
                            </td>
                            <td className={TD}>
                              <div className="max-w-[180px] truncate">
                                <ChatUserCell universalId={u.universal_id} chatUsername={u.username} resolveName={resolveName} />
                              </div>
                            </td>
                            <td className={TD_NUM}>{formatNumber(u.total_requests)}</td>
                            <td className={TD_NUM} title={formatNumber(u.sum_total_tokens)}>{shortToken(u.sum_total_tokens)}</td>
                            <td className={TD_NUM} title={formatNumber(u.sum_prompt_tokens)}>{shortToken(u.sum_prompt_tokens)}</td>
                            <td className={TD_NUM} title={formatNumber(u.sum_completion_tokens)}>{shortToken(u.sum_completion_tokens)}</td>
                            <td className={TD_NUM} title={formatNumber(u.sum_cache_tokens)}>{shortToken(u.sum_cache_tokens)}</td>
                            <td className={TD_NUM}>{u.estimated_total_cost.toFixed(2)}</td>
                            <td className={TD_NUM}>{formatNumber(u.unique_task_count)}</td>
                            <td className={TD_NUM}>{formatNumber(u.active_days)}</td>
                            <td className={TD_NUM}>{fmtMs(u.avg_duration_ms)}</td>
                            <td className={`${TD_NUM} ${u.error_rate > 0.05 ? 'text-rose-600 dark:text-rose-400' : ''}`}>{fmtPct(u.error_rate)}</td>
                          </tr>
                        ))
                      )}
                    </tbody>
                  </table>
                </div>
              </ChartCard>
            )}
          </>
        )}

        {/* User Detail Modal */}
        {userModal.open && (
          <UserDetailModal
            uid={userModal.uid}
            username={userModal.username}
            startDate={start}
            endDate={end}
            onClose={() => setUserModal({ open: false, uid: '', username: '' })}
          />
        )}
      </div>
    </SettingsLayout>
  )
}

// ---- User Detail Modal ----

function UserDetailModal({ uid, username, startDate, endDate, onClose }: {
  uid: string; username: string; startDate: string; endDate: string; onClose: () => void
}) {
  const { theme } = useTheme()
  const p = useMemo(() => getPalette(theme), [theme])

  const { data: rows, isLoading } = useQuery({
    queryKey: ['chat-user-trend', uid, startDate, endDate],
    queryFn: () => chatStats.userTrend(uid, { start_date: startDate, end_date: endDate }),
    enabled: !!uid,
  })

  const userData = rows ?? []

  // 区间合计
  const total = useMemo(() => userData.reduce((s, r) => ({
    requests: s.requests + (r.total_requests || 0),
    tokens: s.tokens + (r.sum_total_tokens || 0),
    cost: s.cost + (r.estimated_total_cost || 0),
    prompt: s.prompt + (r.sum_prompt_tokens || 0),
    completion: s.completion + (r.sum_completion_tokens || 0),
    cache: s.cache + (r.sum_cache_tokens || 0),
    sessions: s.sessions + (r.unique_task_count || 0),
    errors: s.errors + (r.error_requests || 0),
    avgDuration: s.avgDuration + (r.avg_duration_ms || 0) * (r.total_requests || 0),
    avgTTFT: s.avgTTFT + (r.avg_first_token_duration_ms || 0) * (r.total_requests || 0),
    modelPrefs: r.model_preference ? s.modelPrefs.concat(r.model_preference) : s.modelPrefs,
  }), { requests: 0, tokens: 0, cost: 0, prompt: 0, completion: 0, cache: 0, sessions: 0, errors: 0, avgDuration: 0, avgTTFT: 0, modelPrefs: [] as string[] }), [userData])

  // 合并模型偏好
  const mergedModelPref = useMemo(() => {
    const merged: Record<string, number> = {}
    for (const d of userData) {
      if (!d.model_preference) continue
      try {
        const prefs = JSON.parse(d.model_preference)
        for (const [model, count] of Object.entries(prefs)) {
          merged[model] = (merged[model] || 0) + (count as number)
        }
      } catch { /* ignore */ }
    }
    return Object.keys(merged).length > 0 ? merged : null
  }, [userData])

  // 用户 KPI
  const ukpis = [
    { title: '总请求', value: formatNumber(total.requests) },
    { title: '总 Token', value: shortToken(total.tokens), full: formatNumber(total.tokens) },
    { title: '总成本', value: `¥${total.cost.toFixed(2)}` },
    { title: '缓存命中率', value: total.prompt > 0 ? `${(total.cache / total.prompt * 100).toFixed(1)}%` : '-' },
    { title: '日均请求', value: formatNumber(Math.round(total.requests / Math.max(userData.length, 1))) },
    { title: '会话数', value: formatNumber(total.sessions) },
    { title: '平均 TTFT', value: total.requests > 0 ? fmtMs(total.avgTTFT / total.requests) : '-' },
    { title: '平均时延', value: total.requests > 0 ? fmtMs(total.avgDuration / total.requests) : '-' },
  ]

  // 模型偏好饼图
  const modelPrefPieOpt = useMemo(() => {
    if (!mergedModelPref) return null
    const entries = Object.entries(mergedModelPref).sort((a, b) => b[1] - a[1])
    return {
      tooltip: { trigger: 'item' as const },
      legend: { bottom: 0, textStyle: { fontSize: 10 } },
      series: [{
        type: 'pie' as const, radius: ['45%', '72%'], center: ['50%', '45%'],
        label: { formatter: '{b}\n{d}%', fontSize: 10 },
        data: entries.map(([name, value], i) => ({ name, value, itemStyle: { color: PIE_COLORS[i % PIE_COLORS.length] } })),
      }],
    }
  }, [mergedModelPref])

  // 请求+Token 趋势图
  const trendOpt = useMemo(() => multiAreaOption(p,
    userData.map((d) => shortDate(d.date)),
    [
      { name: '请求数', color: '#0071e3', data: userData.map((d) => d.total_requests) },
      { name: 'Token消耗', color: '#34c759', data: userData.map((d) => d.sum_total_tokens) },
    ],
    { yFmt: (v) => shortToken(v) },
  ), [userData, p])

  // 成本趋势图
  const costTrendOpt = useMemo(() => multiAreaOption(p,
    userData.map((d) => shortDate(d.date)),
    [
      { name: '总成本', color: '#ff3b30', data: userData.map((d) => +(d.estimated_total_cost || 0).toFixed(2)) },
      { name: '输入成本', color: '#0071e3', data: userData.map((d) => +(d.estimated_input_cost || 0).toFixed(2)) },
      { name: '输出成本', color: '#34c759', data: userData.map((d) => +(d.estimated_output_cost || 0).toFixed(2)) },
    ],
    { yFmt: (v) => `¥${shortToken(v)}` },
  ), [userData, p])

  return (
    <Modal
      open={true}
      title={`${username} · ${uid}`}
      maxWidth={1100}
      onClose={onClose}
      footer={<button type="button" className={BTN_GLASS} onClick={onClose}>关闭</button>}
    >
      <div className="space-y-4 max-h-[70vh] overflow-y-auto pr-1">
        {isLoading ? (
          <div className="py-12 text-center text-sm text-gray-400">加载中...</div>
        ) : userData.length === 0 ? (
          <div className="py-12 text-center text-sm text-gray-400">暂无数据</div>
        ) : (
          <>
            {/* KPI 行 */}
            <div className="grid grid-cols-4 gap-3">
              {ukpis.slice(0, 4).map((k) => (
                <div key={k.title} className="glass rounded-xl p-3">
                  <div className="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase">{k.title}</div>
                  <div className="text-xl font-bold tabular-nums text-gray-900 dark:text-white" title={k.full}>{k.value}</div>
                </div>
              ))}
            </div>
            <div className="grid grid-cols-4 gap-3">
              {ukpis.slice(4).map((k) => (
                <div key={k.title} className="glass rounded-xl p-3">
                  <div className="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase">{k.title}</div>
                  <div className="text-xl font-bold tabular-nums text-gray-900 dark:text-white" title={k.full}>{k.value}</div>
                </div>
              ))}
            </div>

            <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
              {/* 模型偏好 */}
              {modelPrefPieOpt && (
                <ChartCard title="模型偏好（使用次数）">
                  <EChart option={modelPrefPieOpt} height={280} />
                </ChartCard>
              )}

              {/* 请求+Token */}
              <ChartCard title="请求量 & Token 趋势">
                <EChart option={trendOpt} height={280} />
              </ChartCard>

              {/* 成本趋势 */}
              <ChartCard title="成本变化趋势">
                <EChart option={costTrendOpt} height={260} />
              </ChartCard>
            </div>

            {/* 每日明细表 */}
            <div className="overflow-x-auto max-h-[360px] overflow-y-auto">
              <table className="w-full text-sm border-collapse">
                <thead className="sticky top-0 bg-white/70 dark:bg-gray-900/70 backdrop-blur">
                  <tr className="border-b border-gray-200/50 dark:border-white/10">
                    <th className={TH}>日期</th>
                    <th className={TH_NUM}>请求</th>
                    <th className={TH_NUM}>输入Token</th>
                    <th className={TH_NUM}>输出Token</th>
                    <th className={TH_NUM}>缓存Token</th>
                    <th className={TH_NUM}>成本</th>
                    <th className={TH_NUM}>会话</th>
                    <th className={TH_NUM}>TTFT</th>
                    <th className={TH_NUM}>时延</th>
                  </tr>
                </thead>
                <tbody>
                  {userData.map((d, i) => (
                    <tr key={d.date || i} className="border-b border-gray-100/50 dark:border-white/5">
                      <td className={TD}>{shortDate(d.date)}</td>
                      <td className={TD_NUM}>{formatNumber(d.total_requests)}</td>
                      <td className={TD_NUM} title={formatNumber(d.sum_prompt_tokens)}>{shortToken(d.sum_prompt_tokens)}</td>
                      <td className={TD_NUM} title={formatNumber(d.sum_completion_tokens)}>{shortToken(d.sum_completion_tokens)}</td>
                      <td className={TD_NUM} title={formatNumber(d.sum_cache_tokens)}>{shortToken(d.sum_cache_tokens)}</td>
                      <td className={TD_NUM}>¥{d.estimated_total_cost.toFixed(2)}</td>
                      <td className={TD_NUM}>{formatNumber(d.unique_task_count)}</td>
                      <td className={TD_NUM}>{fmtMs(d.avg_first_token_duration_ms)}</td>
                      <td className={TD_NUM}>{fmtMs(d.avg_duration_ms)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </>
        )}
      </div>
    </Modal>
  )
}

// ---- Skeleton ----

function OverviewSkeleton() {
  return (
    <div className="space-y-5">
      <div className="grid grid-cols-2 sm:grid-cols-4 xl:grid-cols-7 gap-3">
        {Array.from({ length: 7 }).map((_, i) => (
          <div key={i} className="skeleton h-24 rounded-2xl" />
        ))}
      </div>
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        {Array.from({ length: 4 }).map((_, i) => (
          <div key={i} className="skeleton h-72 rounded-2xl" />
        ))}
      </div>
    </div>
  )
}
