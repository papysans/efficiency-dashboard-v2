// 总览·平台客观指标区块（Phase 2）。
// 把已接通的平台（chat-indicator-statistics）全局日粒度客观数据汇总进高管落地页，
// 与上方看板派生的 Hero/趋势/榜单互补——但口径独立（成本双源：平台¥=Token 调用花费 ≠ 看板人天）。
// 数据全部走 chatGet（/api/v2/chat 代理，信封统一解包），参数 snake_case（start_date/end_date），
// 时间用全局 useViewState().timeRange（YYYY-MM-DD，chat 端点直接吃，无需 formatDateParam）。
// 取数方式与字段口径直接复用 PlatformOverview（global/daily 区间求和/平均 + cost-trend 日趋势）。
// 降级护栏：chat_stats_enabled=false → 轻提示（不发请求）；请求失败 → 轻提示（不崩、不影响 Overview 其余）。
import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { chatGet } from '@/api/client'
import { useGlobalConfig } from '@/api/queries'
import { useViewState } from '@/store/viewState'
import { useTheme } from '@/hooks/useTheme'
import { EChart } from '@/components/charts/EChart'
import { getPalette } from '@/components/charts/chartTheme'
import { fmtCost, formatNumber } from '@/lib/formatters'
import { ChartCard, EmptyHint, multiAreaOption, shortToken } from '@/pages/platform/platformShared'
import { useGranularity, GranularityToggle } from '@/pages/dimensions/granularity'
import { buildBuckets, GRANULARITY_CN } from '@/lib/timeBucket'

// ---- 局部类型（对照 PlatformOverview 的 /stats/global/daily、/stats/cost-trend，仅取本块用到的字段） ----

/** /stats/global/daily 行（daily_global_summary）。 */
interface ChatDailyGlobal {
  date: string
  total_requests: number
  total_users: number
  total_error_requests: number
  total_requests_including_errors: number
  sum_prompt_tokens: number
  sum_completion_tokens: number
  sum_total_tokens: number
  sum_cache_tokens: number
  estimated_total_cost: number | null
}

/** /stats/cost-trend 行（全局：model='all'）。 */
interface ChatCostTrendRow {
  date: string
  total_cost: number
  total_requests: number
}

const fmtPct = (v: number | null | undefined) => (v != null ? `${(v * 100).toFixed(2)}%` : '-')

/**
 * 平台客观指标块。chat 开关未就绪时返回 null（不闪占位）；
 * 关闭 / 请求失败 → 轻提示（玻璃拟态薄条），整块不消失但不崩。
 */
export function PlatformObjectiveCard() {
  const { theme } = useTheme()
  const { timeRange } = useViewState()
  // chat 端点参数 snake_case，时间用全局 timeRange 原始 YYYY-MM-DD（无需 formatDateParam）。
  const [start, end] = timeRange

  const { data: gc, isLoading: gcLoading } = useGlobalConfig()
  const configResolved = !!gc && !gcLoading
  const chatEnabled = gc?.chat_stats_enabled === true
  // 开关 false → 不发任何请求（降级护栏）。
  const enabled = chatEnabled && !!start && !!end && start <= end

  const dailyQ = useQuery({
    queryKey: ['overview-platform-daily', start, end],
    queryFn: () => chatGet<ChatDailyGlobal[]>('/stats/global/daily', { start_date: start, end_date: end }),
    enabled,
  })
  const costQ = useQuery({
    queryKey: ['overview-platform-cost-trend', start, end],
    queryFn: () => chatGet<ChatCostTrendRow[]>('/stats/cost-trend', { start_date: start, end_date: end, model: 'all' }),
    enabled,
  })

  const p = useMemo(() => getPalette(theme), [theme])
  const daily = useMemo(() => dailyQ.data ?? [], [dailyQ.data])
  const costRows = useMemo(() => costQ.data ?? [], [costQ.data])

  // 趋势粒度（随全局时间范围重置默认）。
  const { gran, setGran, options: granOptions } = useGranularity(start, end)

  // ---- KPI：区间合计/平均（对齐 PlatformOverview agg 口径） ----
  const agg = useMemo(() => {
    const sum = (fn: (r: ChatDailyGlobal) => number | null | undefined) =>
      daily.reduce((s, r) => s + (fn(r) || 0), 0)
    const requests = sum((r) => r.total_requests)
    // 成功率口径：1 − 错误率；错误率 = 错误请求 / 含错误总请求（与平台总览/维度页一致）。
    const requestsIncErr = sum((r) =>
      r.total_requests_including_errors > 0 ? r.total_requests_including_errors : r.total_requests,
    )
    const errors = sum((r) => r.total_error_requests)
    const errorRate = requestsIncErr > 0 ? errors / requestsIncErr : null
    return {
      requests,
      errorRate,
      successRate: errorRate != null ? 1 - errorRate : null,
      totalTokens: sum((r) => r.sum_total_tokens),
      cacheTokens: sum((r) => r.sum_cache_tokens),
      promptTokens: sum((r) => r.sum_prompt_tokens),
      // 成本：global/daily 的 estimated_total_cost 区间求和（与 cost-trend 同源价格表）。
      cost: sum((r) => r.estimated_total_cost),
      // 活跃用户：total_users 是「日」去重，区间不可直接求和 → 取单日峰值（注明口径），日均作参考。
      peakUsers: daily.reduce((m, r) => Math.max(m, r.total_users), 0),
      avgUsers: daily.length > 0 ? Math.round(sum((r) => r.total_users) / daily.length) : 0,
    }
  }, [daily])

  // 缓存命中率（区间合计）：缓存 token / 输入 token。
  const cacheHitRate = agg.promptTokens > 0 ? agg.cacheTokens / agg.promptTokens : null

  // ---- 趋势图：优先 cost-trend 的总成本序列；无成本数据时回退请求量序列。两者皆可加，按桶求和 ----
  const trendOpt = useMemo(() => {
    if (costRows.length > 0) {
      const byDate = new Map(costRows.map((r) => [r.date, r]))
      const buckets = buildBuckets(costRows.map((r) => r.date), gran, { start, end })
      const data = buckets.map((b) => +b.dates.reduce((acc, d) => acc + (Number(byDate.get(d)?.total_cost) || 0), 0).toFixed(2))
      return multiAreaOption(
        p,
        buckets.map((b) => b.label),
        [{ name: 'AI 花费（¥）', color: '#ff3b30', data }],
        { yFmt: (v) => `¥${shortToken(v)}`, tipFmt: (v) => `¥${fmtCost(v)}`, headers: buckets.map((b) => b.rangeText) },
      )
    }
    const byDate = new Map(daily.map((r) => [r.date, r]))
    const buckets = buildBuckets(daily.map((r) => r.date), gran, { start, end })
    const data = buckets.map((b) => b.dates.reduce((acc, d) => acc + (byDate.get(d)?.total_requests || 0), 0))
    return multiAreaOption(
      p,
      buckets.map((b) => b.label),
      [{ name: '请求量', color: '#ff9500', data }],
      { yFmt: (v) => shortToken(v), tipFmt: (v) => formatNumber(v), headers: buckets.map((b) => b.rangeText) },
    )
  }, [costRows, daily, p, gran, start, end])

  const hasTrend = costRows.length > 0 || daily.length > 0
  const trendTitle = costRows.length > 0 ? 'AI 花费趋势' : '请求量趋势'
  const trendSub = costRows.length > 0 ? '估算（chat-indicator-statistics）' : '含错误请求'

  // config 未就绪：不渲染（避免误判降级闪提示）。
  if (!configResolved) return null

  const wrap = (children: React.ReactNode) => (
    <section className="glass rounded-2xl p-5 md:p-6 hover:shadow-lg transition-shadow flex flex-col">
      <div className="flex items-center justify-between gap-3 mb-1">
        <h2 className="text-sm font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide">平台客观指标</h2>
        <span className="text-[11px] text-gray-400 dark:text-gray-500">平台客观采集 · chat-indicator-statistics</span>
      </div>
      <p className="text-xs text-gray-400 dark:text-gray-500 mb-4">
        AI 调用真实花费 / 请求 / Token，按全局时间范围聚合。口径独立于上方看板派生（平台¥=Token 调用花费，≠ 看板折算人天）。
      </p>
      {children}
    </section>
  )

  // 降级护栏 ①：开关 false → 轻提示，不发请求、不整块消失。
  if (!chatEnabled) {
    return wrap(
      <div className="flex items-center justify-center min-h-[7rem] text-sm text-gray-400 dark:text-gray-500 text-center px-4">
        当前环境未启用平台指标服务（chat_stats_enabled=false），配置平台源后将自动展示 AI 调用花费 / 请求 / Token 等客观数据。
      </div>,
    )
  }

  // 降级护栏 ②：请求失败 → 轻提示，不崩、不影响 Overview 其余部分。
  const fatalError = dailyQ.error || costQ.error
  if (fatalError) {
    return wrap(
      <div className="flex items-center justify-center min-h-[7rem] text-sm text-gray-400 dark:text-gray-500 text-center px-4">
        平台指标暂不可用（{(fatalError as Error).message}）。恢复后将自动展示。
      </div>,
    )
  }

  const loading = dailyQ.isLoading || costQ.isLoading

  if (loading) {
    return wrap(
      <div className="flex flex-col gap-4">
        <div className="grid grid-cols-2 sm:grid-cols-3 xl:grid-cols-5 gap-3">
          {Array.from({ length: 5 }).map((_, i) => (
            <div key={i} className="skeleton h-20 rounded-2xl" />
          ))}
        </div>
        <div className="skeleton h-64 rounded-2xl" />
      </div>,
    )
  }

  const kpis: Array<{ title: string; value: string; sub?: string; full?: string; alert?: boolean }> = [
    { title: '总 AI 花费', value: `¥${agg.cost.toFixed(2)}`, sub: '估算 · Token 调用花费' },
    { title: '总请求', value: formatNumber(agg.requests), sub: `总 Token ${shortToken(agg.totalTokens)}` },
    { title: '活跃用户（峰值）', value: formatNumber(agg.peakUsers), sub: `日均 ${formatNumber(agg.avgUsers)} · 单日去重` },
    {
      title: '成功率',
      value: fmtPct(agg.successRate),
      sub: agg.errorRate != null ? `错误率 ${fmtPct(agg.errorRate)}` : undefined,
      alert: (agg.errorRate ?? 0) > 0.05,
    },
    { title: '缓存命中率', value: fmtPct(cacheHitRate), sub: `缓存 Token ${shortToken(agg.cacheTokens)}` },
  ]

  return wrap(
    <div className="flex flex-col gap-4">
      <div className="grid grid-cols-2 sm:grid-cols-3 xl:grid-cols-5 gap-3">
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

      <ChartCard title={`${trendTitle}（${GRANULARITY_CN[gran]}）`} sub={trendSub} extra={<GranularityToggle value={gran} options={granOptions} onChange={setGran} />}>
        {hasTrend ? <EChart option={trendOpt} height={280} /> : <EmptyHint />}
      </ChartCard>
    </div>,
  )
}
