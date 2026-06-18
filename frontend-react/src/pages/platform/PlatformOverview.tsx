// 平台·总览 —— chat-indicator-statistics 历史汇总表的玻璃拟态重写（design §2.2 /platform/overview 行，
// 对照其 web-ui Dashboard.jsx 的交互，不抄 AntD）。
// 数据全部经 /api/v2/chat 代理走 chatGet（信封 {success,code,data} 由 chatHttp 统一解包）；
// 类型为页面局部 interface（字段对照 chat 侧 pkg/http/handler/stats.go + pkg/model/models.go，不动 src/api/）。
// 注意：汇总表 date 经 GORM 序列化为 RFC3339（"2026-06-04T00:00:00Z"），展示前 slice 成日期。
import { useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { chatGet } from '@/api/client'
import { useGlobalConfig } from '@/api/queries'
import { useTheme } from '@/hooks/useTheme'
import { useUserNameMap } from '@/hooks/useUserNameMap'
import { EChart } from '@/components/charts/EChart'
import { getPalette } from '@/components/charts/chartTheme'
import { formatNumber } from '@/lib/formatters'
import SettingsLayout, { ChatDisabledNotice } from '@/pages/settings/SettingsLayout'
import { ChartCard, ChatUserCell, EmptyHint, multiAreaOption, shortToken } from './platformShared'

// ---- 页面局部类型（GET /stats/* 响应，字段以 chat 侧 stats.go / models.go 为准） ----

/** /stats/global/daily 行（daily_global_summary，仅取页面用到的字段）。 */
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
}

/** /stats/cost-trend 行（stats.go costRow）。 */
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

/** /stats/cache-hit-rate 行（stats.go rateRow）。 */
interface ChatCacheHitRateRow {
  date: string
  sum_cache_tokens: number
  sum_prompt_tokens: number
  cache_hit_rate_pct: number
}

/** /stats/models/cost-ranking 行（stats.go rankingRow，json tag 已映射成 input/output）。 */
interface ChatModelCostRow {
  model: string
  total_requests: number
  total_input_tokens: number
  total_output_tokens: number
  total_cost: number
}

/** /stats/users/ranking 单行（stats.go UsersRanking row）。 */
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

/** /stats/users/ranking 信封内分页体。 */
interface ChatUsersRankingResp {
  total: number
  page: number
  page_size: number
  data: ChatUserRankingRow[]
}

// ---- 日期工具 ----

function pad2(n: number): string {
  return String(n).padStart(2, '0')
}

/** Date → 本地 'YYYY-MM-DD'。 */
function toDateStr(d: Date): string {
  return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}`
}

/** 近 N 天（含今天）的起止日期。 */
function rangeForDays(days: number): { start: string; end: string } {
  const end = new Date()
  const start = new Date()
  start.setDate(end.getDate() - (days - 1))
  return { start: toDateStr(start), end: toDateStr(end) }
}

/** RFC3339/日期串 → 'MM-DD'（x 轴标签）。 */
function shortDate(s: string): string {
  return s.slice(5, 10)
}

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
  // 开关语义与态势/明细页一致：未启用时整页提示（直达 URL 仍可进本页）。
  const { data: gc } = useGlobalConfig()
  const chatEnabled = gc?.chat_stats_enabled === true
  const chatDisabled = !!gc && !chatEnabled

  // 日期范围：快捷档（默认近30天）+ 自定义起止；手动改输入框即脱离快捷档
  const [{ start, end }, setRange] = useState(() => rangeForDays(30))
  const [presetDays, setPresetDays] = useState<number | null>(30)
  const rangeValid = !!start && !!end && start <= end

  // 成本趋势模型筛选（'all' = 全局汇总表；具体模型走维度表，选项来自成本排行）
  const [costModel, setCostModel] = useState('all')
  // 用户排行：排序 + 搜索（防抖 + trim）
  const [userSort, setUserSort] = useState('sum_total_tokens')
  const [searchInput, setSearchInput] = useState('')
  const [search, setSearch] = useState('')
  useEffect(() => {
    const id = window.setTimeout(() => setSearch(searchInput.trim()), 400)
    return () => window.clearTimeout(id)
  }, [searchInput])

  const enabled = chatEnabled && rangeValid

  const dailyQ = useQuery({
    queryKey: ['chat-overview-daily', start, end],
    queryFn: () => chatGet<ChatDailyGlobal[]>('/stats/global/daily', { start_date: start, end_date: end }),
    enabled,
  })
  const costQ = useQuery({
    queryKey: ['chat-overview-cost-trend', start, end, costModel],
    queryFn: () =>
      chatGet<ChatCostTrendRow[]>('/stats/cost-trend', { start_date: start, end_date: end, model: costModel }),
    enabled,
  })
  const cacheQ = useQuery({
    queryKey: ['chat-overview-cache-rate', start, end],
    queryFn: () =>
      chatGet<ChatCacheHitRateRow[]>('/stats/cache-hit-rate', { start_date: start, end_date: end }),
    enabled,
  })
  const rankQ = useQuery({
    queryKey: ['chat-overview-model-ranking', start, end],
    queryFn: () =>
      chatGet<ChatModelCostRow[]>('/stats/models/cost-ranking', { start_date: start, end_date: end }),
    enabled,
  })
  const usersQ = useQuery({
    queryKey: ['chat-overview-users-ranking', start, end, userSort, search],
    queryFn: () =>
      chatGet<ChatUsersRankingResp>('/stats/users/ranking', {
        start_date: start,
        end_date: end,
        sort_by: userSort,
        page: 1,
        page_size: 50,
        ...(search ? { search } : {}),
      }),
    enabled,
  })

  // universal_id 与看板 user_id 同源 → 用户排行解析看板用户名并互链（失败自动回退）。
  const { resolveName } = useUserNameMap()
  const p = useMemo(() => getPalette(theme), [theme])

  const daily = useMemo(() => dailyQ.data ?? [], [dailyQ.data])

  // ---- KPI：区间合计（由 global/daily 前端汇总，对齐 chat 侧 Dashboard.jsx agg 口径） ----
  const agg = useMemo(() => {
    const sum = (fn: (r: ChatDailyGlobal) => number | null | undefined) =>
      daily.reduce((s, r) => s + (fn(r) || 0), 0)
    const requests = sum((r) => r.total_requests)
    const requestsIncErr = sum((r) =>
      r.total_requests_including_errors > 0 ? r.total_requests_including_errors : r.total_requests,
    )
    const errors = sum((r) => r.total_error_requests)
    return {
      requests,
      errors,
      errorRate: requestsIncErr > 0 ? errors / requestsIncErr : null,
      promptTokens: sum((r) => r.sum_prompt_tokens),
      completionTokens: sum((r) => r.sum_completion_tokens),
      cacheTokens: sum((r) => r.sum_cache_tokens),
      cost: sum((r) => r.estimated_total_cost),
      // 活跃用户口径：total_users 是「日」去重，区间不可直接求和 → 取日均，峰值作参考
      avgUsers: daily.length > 0 ? Math.round(sum((r) => r.total_users) / daily.length) : 0,
      peakUsers: daily.reduce((m, r) => Math.max(m, r.total_users), 0),
      avgRequests: daily.length > 0 ? Math.round(requests / daily.length) : 0,
    }
  }, [daily])

  const kpis: Array<{ title: string; value: string; sub?: string; full?: string; alert?: boolean }> = [
    { title: '总请求', value: formatNumber(agg.requests), sub: `日均 ${formatNumber(agg.avgRequests)}` },
    { title: '活跃用户（日均）', value: formatNumber(agg.avgUsers), sub: `单日峰值 ${formatNumber(agg.peakUsers)}` },
    { title: '输入 Token', value: shortToken(agg.promptTokens), full: formatNumber(agg.promptTokens) },
    { title: '输出 Token', value: shortToken(agg.completionTokens), full: formatNumber(agg.completionTokens) },
    { title: '缓存 Token', value: shortToken(agg.cacheTokens), full: formatNumber(agg.cacheTokens) },
    {
      title: '错误率',
      value: fmtPct(agg.errorRate),
      sub: `错误请求 ${formatNumber(agg.errors)}`,
      alert: (agg.errorRate ?? 0) > 0.05,
    },
    { title: '总成本', value: `¥${agg.cost.toFixed(2)}`, sub: '估算（按价格表）' },
  ]

  // ---- 图表 option ----
  const costOpt = useMemo(() => {
    const rows = costQ.data ?? []
    return multiAreaOption(
      p,
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

  const tokenOpt = useMemo(
    () =>
      multiAreaOption(
        p,
        daily.map((r) => shortDate(r.date)),
        [
          { name: '输入 Token', color: '#0071e3', data: daily.map((r) => r.sum_prompt_tokens) },
          { name: '输出 Token', color: '#34c759', data: daily.map((r) => r.sum_completion_tokens) },
          { name: '缓存 Token', color: '#5ac8fa', data: daily.map((r) => r.sum_cache_tokens) },
        ],
        { yFmt: (v) => shortToken(v) },
      ),
    [daily, p],
  )

  const requestOpt = useMemo(
    () =>
      multiAreaOption(
        p,
        daily.map((r) => shortDate(r.date)),
        [
          { name: '请求量', color: '#ff9500', data: daily.map((r) => r.total_requests) },
          { name: '错误请求', color: '#ff3b30', data: daily.map((r) => r.total_error_requests) },
        ],
        { yFmt: (v) => shortToken(v) },
      ),
    [daily, p],
  )

  const cacheOpt = useMemo(() => {
    const rows = cacheQ.data ?? []
    return multiAreaOption(
      p,
      rows.map((r) => shortDate(r.date)),
      [{ name: '缓存命中率', color: '#34c759', data: rows.map((r) => +r.cache_hit_rate_pct.toFixed(1)) }],
      { yFmt: (v) => `${v}%`, yMax: 100 },
    )
  }, [cacheQ.data, p])

  const costModelOptions = useMemo(
    () => ['all', ...(rankQ.data ?? []).map((r) => r.model).filter(Boolean)],
    [rankQ.data],
  )

  const userRows = usersQ.data?.data ?? []
  const queries = [dailyQ, costQ, cacheQ, rankQ, usersQ]
  const errors = queries.filter((q) => q.error).map((q) => (q.error as Error).message)
  const loading = enabled && dailyQ.isLoading

  const presetBtn = (active: boolean) =>
    `px-3 py-1.5 rounded-lg text-sm font-medium cursor-pointer transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue ${
      active
        ? 'bg-apple-blue text-white'
        : 'glass text-gray-600 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white'
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
        <div className="space-y-5">
          {header}
          <ChatDisabledNotice />
        </div>
      </SettingsLayout>
    )
  }

  return (
    <SettingsLayout>
      <div className="space-y-5">
        {header}

        {/* 工具栏：快捷档 + 自定义起止日期 */}
      <div className="flex flex-wrap items-center gap-2">
        {PRESETS.map((o) => (
          <button
            key={o.days}
            type="button"
            onClick={() => {
              setPresetDays(o.days)
              setRange(rangeForDays(o.days))
            }}
            className={presetBtn(presetDays === o.days)}
            aria-pressed={presetDays === o.days}
          >
            {o.label}
          </button>
        ))}
        <label className="flex items-center gap-1.5 text-sm text-gray-500 dark:text-gray-400 ml-2">
          <span>从</span>
          <input
            type="date"
            value={start}
            max={end || undefined}
            onChange={(e) => {
              setPresetDays(null)
              setRange((r) => ({ ...r, start: e.target.value }))
            }}
            className={INPUT_CLS}
            aria-label="开始日期"
          />
          <span>至</span>
          <input
            type="date"
            value={end}
            min={start || undefined}
            onChange={(e) => {
              setPresetDays(null)
              setRange((r) => ({ ...r, end: e.target.value }))
            }}
            className={INPUT_CLS}
            aria-label="结束日期"
          />
        </label>
        {!rangeValid && (
          <span className="text-sm text-rose-600 dark:text-rose-400">请选择有效的起止日期（开始 ≤ 结束）</span>
        )}
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
          {/* KPI 卡行（区间合计） */}
          <div className="grid grid-cols-2 sm:grid-cols-4 xl:grid-cols-7 gap-3">
            {kpis.map((k) => (
              <div key={k.title} className="glass rounded-2xl p-4 hover:shadow-lg transition-shadow">
                <div className="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide mb-1">
                  {k.title}
                </div>
                <div
                  className={`text-2xl font-bold tabular-nums ${
                    k.alert ? 'text-rose-600 dark:text-rose-400' : 'text-gray-900 dark:text-white'
                  }`}
                  title={k.full}
                >
                  {k.value}
                </div>
                {k.sub && <div className="text-xs text-gray-400 dark:text-gray-500 mt-1">{k.sub}</div>}
              </div>
            ))}
          </div>

          {/* 趋势图 */}
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
            <ChartCard
              title="成本趋势"
              sub="按日 · 估算"
              extra={
                <select
                  value={costModel}
                  onChange={(e) => setCostModel(e.target.value)}
                  className={INPUT_CLS}
                  aria-label="成本趋势模型筛选"
                >
                  <option value="all">全部模型</option>
                  {costModelOptions
                    .filter((m) => m !== 'all')
                    .map((m) => (
                      <option key={m} value={m}>
                        {m}
                      </option>
                    ))}
                </select>
              }
            >
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

          {/* 模型成本排行 */}
          <ChartCard title="模型成本排行" sub="按 routed_model · Top 10 · 按总成本倒序">
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
                    <tr>
                      <td colSpan={6}>
                        <EmptyHint compact />
                      </td>
                    </tr>
                  ) : (
                    (rankQ.data ?? []).map((m, i) => (
                      <tr key={m.model || i} className="border-b border-gray-100/50 dark:border-white/5">
                        <td className={TD_NUM}>{i + 1}</td>
                        <td className={TD}>{m.model || '-'}</td>
                        <td className={TD_NUM}>{formatNumber(m.total_requests)}</td>
                        <td className={TD_NUM} title={formatNumber(m.total_input_tokens)}>
                          {shortToken(m.total_input_tokens)}
                        </td>
                        <td className={TD_NUM} title={formatNumber(m.total_output_tokens)}>
                          {shortToken(m.total_output_tokens)}
                        </td>
                        <td className={TD_NUM}>{m.total_cost.toFixed(2)}</td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </ChartCard>

          {/* 用户排行（区间聚合 Top 50） */}
          <ChartCard
            title="用户排行"
            sub={`区间聚合 · Top 50${usersQ.data ? ` · 共 ${formatNumber(usersQ.data.total)} 人` : ''}`}
            extra={
              <>
                <input
                  type="search"
                  value={searchInput}
                  onChange={(e) => setSearchInput(e.target.value)}
                  placeholder="搜索 ID / 用户名"
                  className={INPUT_CLS}
                  aria-label="搜索用户"
                />
                <select
                  value={userSort}
                  onChange={(e) => setUserSort(e.target.value)}
                  className={INPUT_CLS}
                  aria-label="用户排行排序字段"
                >
                  {USER_SORTS.map((s) => (
                    <option key={s.value} value={s.value}>
                      按{s.label}
                    </option>
                  ))}
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
                    <tr>
                      <td colSpan={13} className="py-10 text-center text-sm text-gray-400 dark:text-gray-500">
                        加载中…
                      </td>
                    </tr>
                  ) : userRows.length === 0 ? (
                    <tr>
                      <td colSpan={13}>
                        <EmptyHint compact />
                      </td>
                    </tr>
                  ) : (
                    userRows.map((u, i) => (
                      <tr key={u.universal_id || i} className="border-b border-gray-100/50 dark:border-white/5">
                        <td className={TD_NUM}>{i + 1}</td>
                        <td className={`${TD} font-mono text-xs`}>{u.universal_id || '-'}</td>
                        <td className={TD}>
                          <div className="max-w-[180px] truncate">
                            <ChatUserCell
                              universalId={u.universal_id}
                              chatUsername={u.username}
                              resolveName={resolveName}
                            />
                          </div>
                        </td>
                        <td className={TD_NUM}>{formatNumber(u.total_requests)}</td>
                        <td className={TD_NUM} title={formatNumber(u.sum_total_tokens)}>
                          {shortToken(u.sum_total_tokens)}
                        </td>
                        <td className={TD_NUM} title={formatNumber(u.sum_prompt_tokens)}>
                          {shortToken(u.sum_prompt_tokens)}
                        </td>
                        <td className={TD_NUM} title={formatNumber(u.sum_completion_tokens)}>
                          {shortToken(u.sum_completion_tokens)}
                        </td>
                        <td className={TD_NUM} title={formatNumber(u.sum_cache_tokens)}>
                          {shortToken(u.sum_cache_tokens)}
                        </td>
                        <td className={TD_NUM}>{u.estimated_total_cost.toFixed(2)}</td>
                        <td className={TD_NUM}>{formatNumber(u.unique_task_count)}</td>
                        <td className={TD_NUM}>{formatNumber(u.active_days)}</td>
                        <td className={TD_NUM}>{fmtMs(u.avg_duration_ms)}</td>
                        <td
                          className={`${TD_NUM} ${
                            u.error_rate > 0.05 ? 'text-rose-600 dark:text-rose-400' : ''
                          }`}
                        >
                          {fmtPct(u.error_rate)}
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </ChartCard>
        </>
      )}
      </div>
    </SettingsLayout>
  )
}

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
      <div className="skeleton h-64 rounded-2xl" />
    </div>
  )
}
