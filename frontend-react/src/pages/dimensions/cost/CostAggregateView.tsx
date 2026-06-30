// 成本维度·部门聚合视图：对接后端 7 个部门成本接口。
//   总成本(Token/缓存) · 每日总费用趋势(由 model-trend 聚合) · 各模型成本(表+饼)
//   各模型每日堆叠面积(model-trend) · 异常检测(anomaly)。
// 复用 ChartCard/MetricCard/EmptyHint/EChart + platformShared 的 PIE_COLORS/baseTooltip/multiAreaOption。
// 完全复刻 usage/DeptAggregateView 的结构：Skeleton 卡、错误/空态、ChangeBadge、tabular-nums 表格。
import { useMemo, type ReactNode } from 'react'
import * as echarts from 'echarts'
import type { EChartsOption } from 'echarts'
import { useTheme } from '@/hooks/useTheme'
import { getPalette, type ChartPalette } from '@/components/charts/chartTheme'
import { EChart } from '@/components/charts/EChart'
import { MetricCard } from '@/components/ui/MetricCard'
import { ChartCard, EmptyHint, PIE_COLORS, baseTooltip, shortToken, useZeroRequestFilter, ZeroRequestToggle } from '@/pages/platform/platformShared'
import { useGranularity, GranularityToggle } from '../granularity'
import { buildBuckets, GRANULARITY_CN, type Granularity } from '@/lib/timeBucket'
import { fmtCost, formatNumber } from '@/lib/formatters'
import {
  useCostOverview,
  useCostPeriodCompare,
  useCostModels,
  useCostModelTrend,
  useCostModelComposition,
  useCostAnomaly,
} from './costData'
import type { CostModelItem, CostModelTrendSeries } from './costTypes'

const PCT = (v: number | null | undefined) => (v == null || !Number.isFinite(v) ? '-' : `${v.toFixed(1)}%`)

/** 单价(null 显 '-')。 */
const PRICE = (v: number | null | undefined) => (v == null || !Number.isFinite(v) ? '-' : fmtCost(v))

/** 环比箭头：正绿负红，0 灰。prev==0 时后端可能返 100/特殊值，按有限数处理即可。 */
function ChangeBadge({ pct }: { pct: number }) {
  if (!Number.isFinite(pct)) return <span className="text-gray-400">环比 —</span>
  const up = pct > 0
  const flat = pct === 0
  const color = flat ? 'text-gray-400' : up ? 'text-emerald-600 dark:text-emerald-400' : 'text-rose-600 dark:text-rose-400'
  const arrow = flat ? '·' : up ? '▲' : '▼'
  return (
    <span className={color}>
      环比 {arrow} {PCT(Math.abs(pct))}
    </span>
  )
}

const PURPLE = '#af52de'
const GREEN = '#34c759'

export function CostAggregateView({
  deptId,
  start,
  end,
  includeChildren,
}: {
  deptId: string
  start: string
  end: string
  includeChildren: boolean
}) {
  const q = { deptId, start, end, includeChildren }
  const { theme } = useTheme()
  const p = getPalette(theme)

  const overviewQ = useCostOverview(q)
  const compareQ = useCostPeriodCompare(q)
  const modelsQ = useCostModels(q)
  const trendQ = useCostModelTrend(q)
  const compositionQ = useCostModelComposition(q)
  const anomalyQ = useCostAnomaly(q)

  // 趋势粒度（每页统一控制，随区间重置默认）。
  const { gran, setGran, options: granOptions } = useGranularity(start, end)

  if (!deptId) {
    return <div className="glass rounded-2xl p-10 text-center text-sm text-gray-400 dark:text-gray-500">请在左侧选择部门</div>
  }

  const fatalErr = [overviewQ, modelsQ, trendQ, anomalyQ].find((h) => h.error)?.error
  if (fatalErr) {
    return (
      <div className="glass rounded-2xl p-10 text-center text-sm text-rose-600 dark:text-rose-400">
        加载部门成本指标失败：{(fatalErr as Error).message}
      </div>
    )
  }

  const ov = overviewQ.data
  const cmp = compareQ.data
  const anyLoading = overviewQ.isLoading && !ov

  // 后端对无活动部门返回精简对象（total_cost=0 / active_users=0）。整体空态。
  if (ov && ov.total_cost === 0 && (ov.active_users === 0 || ov.active_users == null)) {
    return (
      <div className="glass rounded-2xl p-10 text-center text-sm text-gray-400 dark:text-gray-500">
        该部门在所选区间内无成本记录。
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-4">
      {/* 区块1 总成本卡 */}
      <ChartCard title="总成本" sub="实际扣费 · 含费用环比">
        {ov ? (
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
            <MetricCard label="总费用" value={`¥${fmtCost(ov.total_cost)}`} tone="pos" accent={PURPLE} />
            <MetricCard label="日均费用" value={`¥${fmtCost(ov.daily_avg_cost)}`} />
            <MetricCard label="人均费用" value={`¥${fmtCost(ov.per_user_avg_cost)}`} hint={`活跃 ${formatNumber(ov.active_users)} 人`} />
            <MetricCard label="每千Token成本" value={`¥${fmtCost(ov.per_1k_token_cost)}`} tip="总费用 / 总Token × 1000" accent={GREEN} />
            <MetricCard label="活跃用户" value={formatNumber(ov.active_users)} />
          </div>
        ) : (
          <Skeleton4 />
        )}
        {cmp && (
          <div className="mt-3 flex flex-wrap items-center gap-4 text-xs">
            <ChangeBadge pct={cmp.cost_change_pct} />
            <span className="text-gray-400 dark:text-gray-500">
              上期 {cmp.previous_period.start} ~ {cmp.previous_period.end}（¥{fmtCost(cmp.previous_period.total_cost)}）
            </span>
          </div>
        )}
      </ChartCard>

      {/* 区块2 Token 成本卡 */}
      <ChartCard title="Token 成本" sub="输入 / 输出费用拆分">
        {ov ? (
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
            <MetricCard label="输入Token费用" value={`¥${fmtCost(ov.input_cost)}`} />
            <MetricCard label="输出Token费用" value={`¥${fmtCost(ov.output_cost)}`} />
            <MetricCard label="输入费用占比" value={PCT(ov.input_cost_pct)} />
            <MetricCard label="输出费用占比" value={PCT(ov.output_cost_pct)} />
            <MetricCard label="总Token" value={shortToken(ov.total_tokens)} hint={formatNumber(ov.total_tokens)} />
          </div>
        ) : (
          <Skeleton4 />
        )}
      </ChartCard>

      {/* 区块3 缓存成本卡 */}
      <ChartCard title="缓存成本" sub="命中 / 未命中 · 节省费用">
        {ov ? (
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
            <MetricCard label="缓存命中输入Token" value={shortToken(ov.cache.hit_input_tokens)} hint={formatNumber(ov.cache.hit_input_tokens)} />
            <MetricCard label="缓存命中输入费用" value={`¥${fmtCost(ov.cache.hit_input_cost)}`} />
            <MetricCard label="缓存未命中输入Token" value={shortToken(ov.cache.miss_input_tokens)} hint={formatNumber(ov.cache.miss_input_tokens)} />
            <MetricCard label="缓存未命中输入费用" value={`¥${fmtCost(ov.cache.miss_input_cost)}`} />
            <MetricCard label="缓存命中率" value={PCT(ov.cache.hit_rate_pct)} />
            <MetricCard label="缓存节省费用" value={`¥${fmtCost(ov.cache.savings)}`} tone="pos" accent={GREEN} />
          </div>
        ) : (
          <Skeleton4 />
        )}
      </ChartCard>

      {/* 区块4 总费用趋势（由 model-trend 各模型按 date 聚合）· 粒度可切（每页统一控制） */}
      <DailyCostTrendBlock
        loading={anyLoading}
        series={trendQ.data?.series}
        palette={p}
        gran={gran}
        start={start}
        end={end}
        granControl={<GranularityToggle value={gran} options={granOptions} onChange={setGran} />}
      />

      {/* 区块5 各模型成本（表 + 饼） */}
      <ModelsCostBlock
        loading={modelsQ.isLoading}
        models={modelsQ.data?.models}
        composition={compositionQ.data?.items}
        palette={p}
      />

      {/* 区块6 各模型费用趋势堆叠面积（同粒度） */}
      <ModelTrendStackBlock
        loading={trendQ.isLoading}
        series={trendQ.data?.series}
        palette={p}
        gran={gran}
        start={start}
        end={end}
        granControl={<GranularityToggle value={gran} options={granOptions} onChange={setGran} />}
      />

      {/* 区块7 异常检测 */}
      <AnomalyBlock data={anomalyQ.data} />
    </div>
  )
}

// ============================ 总费用趋势（聚合各模型 data）·粒度可切 ============================
// 费用可加：按桶求和。
function DailyCostTrendBlock({
  loading,
  series,
  palette: p,
  gran,
  start,
  end,
  granControl,
}: {
  loading: boolean
  series?: CostModelTrendSeries[]
  palette: ChartPalette
  gran: Granularity
  start: string
  end: string
  granControl: ReactNode
}) {
  // 各模型 series.data 先按 date 汇总成每日 total_cost，再按粒度分桶求和。
  const { labels, headers, totals } = useMemo(() => {
    const dateMap = new Map<string, number>()
    for (const s of series || []) {
      for (const pt of s.data) dateMap.set(pt.date, (dateMap.get(pt.date) || 0) + (pt.total_cost || 0))
    }
    const buckets = buildBuckets(Array.from(dateMap.keys()), gran, { start, end })
    return {
      labels: buckets.map((b) => b.label),
      headers: buckets.map((b) => b.rangeText),
      totals: buckets.map((b) => b.dates.reduce((acc, d) => acc + (dateMap.get(d) || 0), 0)),
    }
  }, [series, gran, start, end])

  const opt = useMemo<EChartsOption | null>(() => {
    if (!labels.length) return null
    return {
      animation: true,
      grid: { left: 8, right: 16, top: 24, bottom: 8, containLabel: true },
      tooltip: {
        trigger: 'axis',
        ...baseTooltip(p),
        formatter: (params: unknown) => {
          const arr = params as { dataIndex: number; value: number; marker: string }[]
          const head = headers[arr[0]?.dataIndex] ?? ''
          return `${head}<br/>${arr.map((it) => `${it.marker}总费用: ¥${fmtCost(it.value)}`).join('<br/>')}`
        },
      },
      xAxis: {
        type: 'category',
        data: labels,
        boundaryGap: false,
        axisLine: { lineStyle: { color: p.axisColor } },
        axisLabel: { color: p.textColor, hideOverlap: true },
        axisTick: { show: false },
      },
      yAxis: {
        type: 'value',
        axisLabel: { color: p.textColor, formatter: (v: number) => `¥${shortToken(v)}` },
        splitLine: { lineStyle: { color: p.splitLineColor } },
      },
      series: [
        {
          name: '总费用',
          type: 'line',
          smooth: true,
          symbol: 'circle',
          symbolSize: 6,
          data: totals,
          lineStyle: { color: PURPLE, width: 2 },
          itemStyle: { color: PURPLE },
          areaStyle: {
            color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
              { offset: 0, color: rgba(PURPLE, 0.25) },
              { offset: 1, color: rgba(PURPLE, 0) },
            ]),
          },
        },
      ],
    }
  }, [labels, headers, totals, p])

  const title = `总费用趋势（${GRANULARITY_CN[gran]}）`
  if (loading) return <SkeletonCard title={title} />
  if (!opt) {
    return (
      <ChartCard title={title} sub="由各模型费用聚合（后端无 cost/daily-trend）" extra={granControl}>
        <EmptyHint />
      </ChartCard>
    )
  }
  return (
    <ChartCard title={title} sub="由各模型费用聚合（后端无 cost/daily-trend）" extra={granControl}>
      <EChart option={opt} height={280} />
    </ChartCard>
  )
}

// ============================ 各模型成本（表 + 饼） ============================
function ModelsCostBlock({
  loading,
  models,
  composition,
  palette: p,
}: {
  loading: boolean
  models?: CostModelItem[]
  composition?: { model: string; total_cost: number; cost_pct: number }[]
  palette: ChartPalette
}) {
  const { showZero, setShowZero, visible: visibleModels, hiddenCount } = useZeroRequestFilter(models)
  const pieOpt = useMemo<EChartsOption | null>(() => {
    if (!visibleModels.length) return null
    // 饼图与表格同口径：只保留未被隐藏（request_count>0 或已展开）的模型。
    const visibleNames = new Set(visibleModels.map((m) => m.model))
    const items = (composition && composition.length ? composition : visibleModels).filter((m) =>
      visibleNames.has((m as { model: string }).model),
    )
    if (!items.length) return null
    return {
      tooltip: { trigger: 'item', ...baseTooltip(p), formatter: '{b}: ¥{c} ({d}%)' },
      legend: { type: 'scroll', bottom: 0, textStyle: { color: p.textColor } },
      series: [
        {
          type: 'pie',
          radius: ['38%', '68%'],
          center: ['50%', '46%'],
          itemStyle: { borderColor: p.tooltipBg, borderWidth: 2 },
          label: { color: p.textColor, formatter: '{d}%' },
          data: items.map((m, i) => ({
            name: (m as { model: string }).model,
            value: m.total_cost,
            itemStyle: { color: PIE_COLORS[i % PIE_COLORS.length] },
          })),
        },
      ],
    }
  }, [composition, visibleModels, p])

  if (loading) return <SkeletonCard title="各模型成本" />
  if (!models || !models.length) {
    return (
      <ChartCard title="各模型成本" sub="费用 / 占比 / 单价 / 实际平均成本">
        <EmptyHint />
      </ChartCard>
    )
  }
  return (
    <ChartCard
      title="各模型成本"
      sub="按实际命中模型拆分（后端按 total_cost 降序）"
      extra={<ZeroRequestToggle showZero={showZero} onToggle={setShowZero} hiddenCount={hiddenCount} />}
    >
      {visibleModels.length === 0 ? (
        <EmptyHint />
      ) : (
        <div className="grid grid-cols-1 lg:grid-cols-[20rem_1fr] gap-4 items-start">
          {pieOpt && <EChart option={pieOpt} height={300} />}
          <div className="overflow-x-auto">
            <table className="w-full text-sm border-collapse">
              <thead>
                <tr className="border-b border-gray-200/50 dark:border-white/10 text-gray-500 dark:text-gray-400">
                  <th className="px-3 py-2 text-left whitespace-nowrap">模型</th>
                  <th className="px-3 py-2 text-right whitespace-nowrap">费用</th>
                  <th className="px-3 py-2 text-right whitespace-nowrap">费用占比</th>
                  <th className="px-3 py-2 text-right whitespace-nowrap">输入单价/千</th>
                  <th className="px-3 py-2 text-right whitespace-nowrap">输出单价/千</th>
                  <th className="px-3 py-2 text-right whitespace-nowrap">实际平均成本/千</th>
                  <th className="px-3 py-2 text-right whitespace-nowrap">请求数</th>
                </tr>
              </thead>
              <tbody>
                {visibleModels.map((m, i) => (
                <tr key={m.model || i} className="border-b border-gray-100/50 dark:border-white/5">
                  <td className="px-3 py-2 text-gray-700 dark:text-gray-200">
                    <span className="inline-flex items-center gap-2">
                      <span className="w-2.5 h-2.5 rounded-full" style={{ background: PIE_COLORS[i % PIE_COLORS.length] }} />
                      <span className="truncate max-w-[180px]" title={m.model}>{m.model || '-'}</span>
                    </span>
                  </td>
                  <td className="px-3 py-2 text-right tabular-nums text-gray-700 dark:text-gray-200">¥{fmtCost(m.total_cost)}</td>
                  <td className="px-3 py-2 text-right tabular-nums text-gray-700 dark:text-gray-200">{PCT(m.cost_pct)}</td>
                  <td className="px-3 py-2 text-right tabular-nums text-gray-700 dark:text-gray-200">{PRICE(m.unit_price.input_per_1k)}</td>
                  <td className="px-3 py-2 text-right tabular-nums text-gray-700 dark:text-gray-200">{PRICE(m.unit_price.output_per_1k)}</td>
                  <td className="px-3 py-2 text-right tabular-nums text-gray-700 dark:text-gray-200">¥{fmtCost(m.actual_avg_cost_per_1k)}</td>
                  <td className="px-3 py-2 text-right tabular-nums text-gray-700 dark:text-gray-200">{formatNumber(m.request_count)}</td>
                </tr>
              ))}
            </tbody>
          </table>
          </div>
        </div>
      )}
    </ChartCard>
  )
}

// ============================ 各模型费用堆叠面积·粒度可切 ============================
function ModelTrendStackBlock({
  loading,
  series,
  palette: p,
  gran,
  start,
  end,
  granControl,
}: {
  loading: boolean
  series?: CostModelTrendSeries[]
  palette: ChartPalette
  gran: Granularity
  start: string
  end: string
  granControl?: ReactNode
}) {
  const opt = useMemo<EChartsOption | null>(() => {
    if (!series || !series.length) return null
    // 取所有日期并集（各模型日期可能不完全对齐），按粒度分桶；各模型在桶内费用求和。
    const dateSet = new Set<string>()
    for (const s of series) for (const pt of s.data) dateSet.add(pt.date)
    const buckets = buildBuckets(Array.from(dateSet), gran, { start, end })
    if (!buckets.length) return null
    const labels = buckets.map((b) => b.label)
    const headers = buckets.map((b) => b.rangeText)
    const byModel = new Map<string, Map<string, number>>()
    for (const s of series) {
      const m = new Map<string, number>()
      for (const pt of s.data) m.set(pt.date, pt.total_cost || 0)
      byModel.set(s.model, m)
    }
    return {
      animation: true,
      grid: { left: 8, right: 16, top: 36, bottom: 8, containLabel: true },
      tooltip: {
        trigger: 'axis',
        ...baseTooltip(p),
        formatter: (params: unknown) => {
          const arr = params as { dataIndex: number; seriesName: string; value: number; marker: string }[]
          const head = headers[arr[0]?.dataIndex] ?? ''
          const total = arr.reduce((acc, it) => acc + (Number(it.value) || 0), 0)
          const lines = arr.map((it) => `${it.marker}${it.seriesName}: ¥${fmtCost(it.value)}`)
          return `${head}<br/>${lines.join('<br/>')}<br/><b>合计: ¥${fmtCost(total)}</b>`
        },
      },
      legend: { type: 'scroll', top: 0, left: 'center', textStyle: { color: p.textColor }, itemWidth: 14, itemHeight: 8 },
      xAxis: {
        type: 'category',
        data: labels,
        boundaryGap: false,
        axisLine: { lineStyle: { color: p.axisColor } },
        axisLabel: { color: p.textColor, hideOverlap: true },
        axisTick: { show: false },
      },
      yAxis: {
        type: 'value',
        axisLabel: { color: p.textColor, formatter: (v: number) => `¥${shortToken(v)}` },
        splitLine: { lineStyle: { color: p.splitLineColor } },
      },
      series: series.map((s, i) => {
        const color = PIE_COLORS[i % PIE_COLORS.length]
        const m = byModel.get(s.model)!
        return {
          name: s.model,
          type: 'line',
          stack: 'cost',
          smooth: true,
          symbol: 'none',
          data: buckets.map((b) => b.dates.reduce((acc, d) => acc + (m.get(d) || 0), 0)),
          lineStyle: { color, width: 1.5 },
          itemStyle: { color },
          areaStyle: {
            color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
              { offset: 0, color: rgba(color, 0.4) },
              { offset: 1, color: rgba(color, 0.05) },
            ]),
          },
        }
      }),
    }
  }, [series, p, gran, start, end])

  const title = `各模型费用趋势（${GRANULARITY_CN[gran]}）`
  if (loading) return <SkeletonCard title={title} />
  if (!opt) {
    return (
      <ChartCard title={title} sub="堆叠面积图" extra={granControl}>
        <EmptyHint />
      </ChartCard>
    )
  }
  return (
    <ChartCard title={title} sub="堆叠面积图" extra={granControl}>
      <EChart option={opt} height={300} />
    </ChartCard>
  )
}

// ============================ 异常检测 ============================
function AnomalyBlock({
  data,
}: {
  data?: { daily_spike_count: number; user_spike_count: number; zero_cost_active_users: number; daily_spike_threshold: number; user_spike_threshold: number }
}) {
  if (!data) {
    return (
      <ChartCard title="异常检测" sub="单日/单用户费用突增 · 0 费用活跃用户">
        <Skeleton4 />
      </ChartCard>
    )
  }
  return (
    <ChartCard title="异常检测" sub="单日/单用户费用突增 · 0 费用活跃用户">
      <div className="grid grid-cols-2 sm:grid-cols-3 gap-3">
        <MetricCard
          label="单日费用突增次数"
          value={formatNumber(data.daily_spike_count)}
          hint={`较前7日日均 +${(data.daily_spike_threshold * 100).toFixed(0)}%`}
          tone={data.daily_spike_count > 0 ? 'neg' : 'neutral'}
        />
        <MetricCard
          label="单用户费用突增次数"
          value={formatNumber(data.user_spike_count)}
          hint={`较个人前7日日均 +${(data.user_spike_threshold * 100).toFixed(0)}%（去重用户）`}
          tone={data.user_spike_count > 0 ? 'neg' : 'neutral'}
        />
        <MetricCard
          label="费用为0的活跃用户数"
          value={formatNumber(data.zero_cost_active_users)}
          tone="neutral"
        />
      </div>
    </ChartCard>
  )
}

// ============================ 小工具 ============================
function Skeleton4() {
  return (
    <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
      {Array.from({ length: 8 }).map((_, i) => (
        <div key={i} className="skeleton h-24 rounded-2xl" />
      ))}
    </div>
  )
}

function SkeletonCard({ title }: { title: ReactNode }) {
  return (
    <ChartCard title={typeof title === 'string' ? title : ''}>
      <div className="h-[240px] skeleton rounded-xl" />
    </ChartCard>
  )
}

function rgba(hex: string, alpha: number): string {
  const r = parseInt(hex.slice(1, 3), 16)
  const g = parseInt(hex.slice(3, 5), 16)
  const b = parseInt(hex.slice(5, 7), 16)
  return `rgba(${r},${g},${b},${alpha})`
}
