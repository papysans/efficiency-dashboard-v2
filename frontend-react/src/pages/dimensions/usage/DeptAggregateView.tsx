// 部门聚合视角：对接后端 7 个部门接口，覆盖需求 1/2/3/4/6 的部门口径指标。
//   活跃用户(active-users) · 使用概览+环比(overview+period-compare) · 按天趋势(trend)
//   各模型用量(models/usage) · 按星期(distribution/weekly) · 请求结果(results)。
// 趋势用 trendOptions.buildDualAxisTrendOption（按天，左轴请求量+右轴活跃用户）；token 趋势用 multiAreaOption。
import { useMemo, type ReactNode } from 'react'
import type { EChartsOption } from 'echarts'
import { useTheme } from '@/hooks/useTheme'
import { getPalette, type ChartPalette } from '@/components/charts/chartTheme'
import { EChart } from '@/components/charts/EChart'
import { MetricCard } from '@/components/ui/MetricCard'
import { ChartCard, EmptyHint, PIE_COLORS, baseTooltip, multiAreaOption, shortToken } from '@/pages/platform/platformShared'
import { buildDualAxisTrendOption, type TrendSeriesItem } from '../trendOptions'
import { formatNumber } from '@/lib/formatters'
import {
  useUsageDeptActiveUsers,
  useUsageDeptModels,
  useUsageDeptOverview,
  useUsageDeptResults,
  useUsageDeptTrend,
  useUsageDeptWeekly,
  useUsagePeriodCompare,
} from './usageData'

const PCT = (v: number | null | undefined) => (v == null || !Number.isFinite(v) ? '-' : `${v.toFixed(1)}%`)

/** 环比箭头：正绿负红，0 灰。用于「总请求」「总 Token」卡的 hint。 */
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

export function DeptAggregateView({
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

  const overviewQ = useUsageDeptOverview(q)
  const activeQ = useUsageDeptActiveUsers(q)
  const trendQ = useUsageDeptTrend(q)
  const modelsQ = useUsageDeptModels(q)
  const weeklyQ = useUsageDeptWeekly(q)
  const resultsQ = useUsageDeptResults(q)
  const compareQ = useUsagePeriodCompare(q)

  if (!deptId) {
    return <div className="glass rounded-2xl p-10 text-center text-sm text-gray-400 dark:text-gray-500">请在左侧选择部门</div>
  }

  const fatalErr = [overviewQ, activeQ, trendQ, modelsQ, weeklyQ, resultsQ].find((h) => h.error)?.error
  if (fatalErr) {
    return (
      <div className="glass rounded-2xl p-10 text-center text-sm text-rose-600 dark:text-rose-400">
        加载部门指标失败：{(fatalErr as Error).message}
      </div>
    )
  }

  const ov = overviewQ.data
  const au = activeQ.data
  const cmp = compareQ.data
  const anyLoading = overviewQ.isLoading && !ov

  // 部门无数据：后端对无活动部门返回精简对象（active_users=0 / total_requests=0，其余字段缺）。
  // 整体空态，避免 success_rate / total_sessions 等 undefined 字段在 formatNumber/PCT 里崩溃。
  if (ov && ov.total_requests === 0 && (ov.active_users === 0 || ov.active_users == null)) {
    return (
      <div className="glass rounded-2xl p-10 text-center text-sm text-gray-400 dark:text-gray-500">
        该部门在所选区间内无平台使用记录。
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-4">
      {/* 需求1 活跃用户（部门视角；请求失败的用户也算活跃——后端 clean 口径） */}
      <ChartCard title="活跃用户" sub="DAU/WAU/MAU · DAU/WAU 比值衡量粘性">
        {au ? (
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
            <MetricCard label="DAU 日活" value={formatNumber(au.dau)} hint="区间末日至少 1 次请求的去重用户" />
            <MetricCard label="WAU 周活" value={formatNumber(au.wau)} hint="末日往前 7 天滚动去重" />
            <MetricCard label="MAU 月活" value={formatNumber(au.mau)} hint="末日往前 30 天滚动去重" />
            <MetricCard label="DAU/WAU" value={PCT(au.dau_wau_ratio)} hint="粘性：日活占周活比" accent={p.brand} />
          </div>
        ) : (
          <Skeleton4 />
        )}
      </ChartCard>

      {/* 需求2 用户使用（总请求/人均/会话/token/人均token + 环比）+ 需求6 成功率/失败率 */}
      <ChartCard title="使用概览" sub="除成功率/失败率外，均已排除失败请求">
        {ov ? (
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
            <MetricCard label="总请求" value={formatNumber(ov.total_requests)} tip="统计周期内所有成功 API 请求" />
            <MetricCard label="人均请求" value={ov.active_users ? formatNumber(Math.round(ov.total_requests / ov.active_users)) : '-'} hint={`活跃 ${formatNumber(ov.active_users)} 人`} />
            <MetricCard label="总会话数" value={formatNumber(ov.total_sessions)} hint="unique_task 去重" />
            <MetricCard label="活跃用户" value={formatNumber(ov.active_users)} />
            <MetricCard label="总输入 Token" value={shortToken(ov.sum_prompt_tokens)} hint={formatNumber(ov.sum_prompt_tokens)} />
            <MetricCard label="总输出 Token" value={shortToken(ov.sum_completion_tokens)} hint={formatNumber(ov.sum_completion_tokens)} />
            <MetricCard
              label="总 Token 消耗"
              value={shortToken(ov.sum_total_tokens)}
              hint={formatNumber(ov.sum_total_tokens)}
              tone={cmp && cmp.token_change_pct > 0 ? 'pos' : cmp && cmp.token_change_pct < 0 ? 'neg' : 'neutral'}
            />
            <MetricCard label="人均 Token" value={ov.active_users ? shortToken(Math.round(ov.sum_total_tokens / ov.active_users)) : '-'} />
            <MetricCard label="请求成功率" value={PCT(ov.success_rate)} tone="pos" />
            <MetricCard label="请求失败率" value={PCT(ov.error_rate)} tone={ov.error_rate > 5 ? 'neg' : 'neutral'} />
            <MetricCard label="人均输入 Token" value={ov.active_users ? shortToken(Math.round(ov.sum_prompt_tokens / ov.active_users)) : '-'} />
            <MetricCard label="人均输出 Token" value={ov.active_users ? shortToken(Math.round(ov.sum_completion_tokens / ov.active_users)) : '-'} />
          </div>
        ) : (
          <Skeleton4 />
        )}
        {cmp && (
          <div className="mt-3 flex flex-wrap items-center gap-4 text-xs">
            <ChangeBadge pct={cmp.request_change_pct} />
            <ChangeBadge pct={cmp.token_change_pct} />
            <span className="text-gray-400 dark:text-gray-500">
              上期 {cmp.previous_period.start} ~ {cmp.previous_period.end}
            </span>
          </div>
        )}
      </ChartCard>

      {/* 需求3/4 按天趋势：请求量 + 活跃用户（双轴） */}
      <TrendBlock loading={anyLoading} trend={trendQ.data?.trend} palette={p} />

      {/* 需求3 各模型用量 */}
      <ModelsBlock loading={modelsQ.isLoading} models={modelsQ.data?.models} palette={p} />

      {/* 需求4 按星期分布 */}
      <WeeklyBlock loading={weeklyQ.isLoading} weekdays={weeklyQ.data?.weekdays} palette={p} />

      {/* 需求6 请求结果 + 各模型成功率 */}
      <ResultsBlock loading={resultsQ.isLoading} data={resultsQ.data} palette={p} />
    </div>
  )
}

// ============================ 按天趋势（请求量 + 活跃用户 双轴；Token 两线） ============================
function TrendBlock({
  loading,
  trend,
  palette: p,
}: {
  loading: boolean
  trend?: { date: string; request_count: number; prompt_tokens: number; completion_tokens: number; active_users: number }[]
  palette: ChartPalette
}) {
  const labels = useMemo(() => (trend || []).map((t) => t.date), [trend])
  const reqSeries: TrendSeriesItem[] = useMemo(
    () => [
      { name: '请求量', color: '#ff9500', data: (trend || []).map((t) => t.request_count) },
      { name: '活跃用户', color: '#34c759', axis: 'right', data: (trend || []).map((t) => t.active_users) },
    ],
    [trend],
  )
  const tokenOpt = useMemo(
    () =>
      multiAreaOption(
        p,
        labels,
        [
          { name: '输入 Token', color: '#0071e3', data: (trend || []).map((t) => t.prompt_tokens) },
          { name: '输出 Token', color: '#af52de', data: (trend || []).map((t) => t.completion_tokens) },
        ],
        { yFmt: (v) => shortToken(v) },
      ),
    [p, labels, trend],
  )

  if (loading) return <SkeletonCard title="使用趋势（按天）" />
  if (!trend || !trend.length) {
    return (
      <ChartCard title="使用趋势（按天）" sub="每日请求量 / 活跃用户 / Token 消耗">
        <EmptyHint />
      </ChartCard>
    )
  }
  return (
    <>
      <ChartCard title="使用趋势（按天）" sub="请求量（左轴）· 活跃用户（右轴）">
        <EChart option={buildDualAxisTrendOption(p, labels, reqSeries, { leftFmt: (v) => shortToken(v), rightFmt: (v) => formatNumber(v) })} height={280} />
      </ChartCard>
      <ChartCard title="Token 消耗趋势（按天）" sub="输入 / 输出 Token">
        <EChart option={tokenOpt} height={260} />
      </ChartCard>
    </>
  )
}

// ============================ 各模型用量 ============================
function ModelsBlock({
  loading,
  models,
  palette: p,
}: {
  loading: boolean
  models?: { model: string; request_count: number; request_pct: number; prompt_tokens: number; completion_tokens: number; total_tokens: number; token_pct: number; input_output_ratio: number; success_rate: number; estimated_total_cost: number }[]
  palette: ChartPalette
}) {
  const pieOpt = useMemo<EChartsOption | null>(() => {
    if (!models || !models.length) return null
    return {
      tooltip: { trigger: 'item', ...baseTooltip(p), formatter: '{b}: {c} ({d}%)' },
      legend: { type: 'scroll', bottom: 0, textStyle: { color: p.textColor } },
      series: [
        {
          type: 'pie',
          radius: ['38%', '68%'],
          center: ['50%', '46%'],
          itemStyle: { borderColor: p.tooltipBg, borderWidth: 2 },
          label: { color: p.textColor },
          data: models.map((m, i) => ({ name: m.model, value: m.request_count, itemStyle: { color: PIE_COLORS[i % PIE_COLORS.length] } })),
        },
      ],
    }
  }, [models, p])

  if (loading) return <SkeletonCard title="各模型使用" />
  if (!models || !models.length) {
    return (
      <ChartCard title="各模型使用" sub="请求次数 / 占比 / Token / 成功率">
        <EmptyHint />
      </ChartCard>
    )
  }
  return (
    <ChartCard title="各模型使用" sub="按实际命中模型（routed_model）拆分">
      <div className="grid grid-cols-1 lg:grid-cols-[20rem_1fr] gap-4 items-start">
        {pieOpt && <EChart option={pieOpt} height={260} />}
        <div className="overflow-x-auto">
          <table className="w-full text-sm border-collapse">
            <thead>
              <tr className="border-b border-gray-200/50 dark:border-white/10 text-gray-500 dark:text-gray-400">
                <th className="px-3 py-2 text-left whitespace-nowrap">模型</th>
                <th className="px-3 py-2 text-right whitespace-nowrap">请求次数</th>
                <th className="px-3 py-2 text-right whitespace-nowrap">请求占比</th>
                <th className="px-3 py-2 text-right whitespace-nowrap">输入 Token</th>
                <th className="px-3 py-2 text-right whitespace-nowrap">输出 Token</th>
                <th className="px-3 py-2 text-right whitespace-nowrap">消耗占比</th>
                <th className="px-3 py-2 text-right whitespace-nowrap">输入/输出</th>
                <th className="px-3 py-2 text-right whitespace-nowrap">成功率</th>
              </tr>
            </thead>
            <tbody>
              {models.map((m, i) => (
                <tr key={m.model || i} className="border-b border-gray-100/50 dark:border-white/5">
                  <td className="px-3 py-2 text-gray-700 dark:text-gray-200">
                    <span className="inline-flex items-center gap-2">
                      <span className="w-2.5 h-2.5 rounded-full" style={{ background: PIE_COLORS[i % PIE_COLORS.length] }} />
                      <span className="truncate max-w-[180px]" title={m.model}>{m.model || '-'}</span>
                    </span>
                  </td>
                  <td className="px-3 py-2 text-right tabular-nums text-gray-700 dark:text-gray-200">{formatNumber(m.request_count)}</td>
                  <td className="px-3 py-2 text-right tabular-nums text-gray-700 dark:text-gray-200">{PCT(m.request_pct)}</td>
                  <td className="px-3 py-2 text-right tabular-nums text-gray-700 dark:text-gray-200" title={formatNumber(m.prompt_tokens)}>{shortToken(m.prompt_tokens)}</td>
                  <td className="px-3 py-2 text-right tabular-nums text-gray-700 dark:text-gray-200" title={formatNumber(m.completion_tokens)}>{shortToken(m.completion_tokens)}</td>
                  <td className="px-3 py-2 text-right tabular-nums text-gray-700 dark:text-gray-200">{PCT(m.token_pct)}</td>
                  <td className="px-3 py-2 text-right tabular-nums text-gray-700 dark:text-gray-200">{m.input_output_ratio.toFixed(2)}</td>
                  <td className="px-3 py-2 text-right tabular-nums text-gray-700 dark:text-gray-200">{PCT(m.success_rate)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </ChartCard>
  )
}

// ============================ 按星期分布 ============================
function WeeklyBlock({
  loading,
  weekdays,
  palette: p,
}: {
  loading: boolean
  weekdays?: { weekday: number; weekday_name: string; request_count: number }[]
  palette: ChartPalette
}) {
  const opt = useMemo<EChartsOption | null>(() => {
    if (!weekdays || !weekdays.length) return null
    return {
      grid: { left: 8, right: 16, top: 16, bottom: 8, containLabel: true },
      tooltip: { trigger: 'axis', ...baseTooltip(p) },
      xAxis: {
        type: 'category',
        data: weekdays.map((w) => w.weekday_name),
        axisLine: { lineStyle: { color: p.axisColor } },
        axisLabel: { color: p.textColor },
        axisTick: { show: false },
      },
      yAxis: { type: 'value', axisLabel: { color: p.textColor, formatter: (v: number) => shortToken(v) }, splitLine: { lineStyle: { color: p.splitLineColor } } },
      series: [{ type: 'bar', data: weekdays.map((w) => w.request_count), itemStyle: { color: p.brand }, barMaxWidth: 36 }],
    }
  }, [weekdays, p])

  if (loading) return <SkeletonCard title="按星期请求量分布" />
  if (!opt) {
    return (
      <ChartCard title="按星期请求量分布" sub="一周 7 天各日请求次数">
        <EmptyHint />
      </ChartCard>
    )
  }
  return (
    <ChartCard title="按星期请求量分布" sub="一周 7 天各日请求次数">
      <EChart option={opt} height={240} />
    </ChartCard>
  )
}

// ============================ 请求结果 + 各模型成功率 ============================
function ResultsBlock({
  loading,
  data,
  palette: p,
}: {
  loading: boolean
  data?: { total_requests: number; success_requests: number; error_requests: number; success_rate: number; error_rate: number; models: { model: string; total_requests: number; error_requests: number; success_rate: number; error_rate: number }[] }
  palette: ChartPalette
}) {
  const opt = useMemo<EChartsOption | null>(() => {
    const ms = data?.models || []
    if (!ms.length) return null
    return {
      grid: { left: 8, right: 16, top: 16, bottom: 8, containLabel: true },
      tooltip: { trigger: 'axis', ...baseTooltip(p), formatter: (params: unknown) => {
        const arr = params as { name: string; value: number }[]
        return arr.map((it) => `${it.name}<br/>成功率 ${it.value.toFixed(1)}%`).join('<br/>')
      } },
      xAxis: {
        type: 'category',
        data: ms.map((m) => m.model),
        axisLine: { lineStyle: { color: p.axisColor } },
        axisLabel: { color: p.textColor, rotate: 30, hideOverlap: true },
        axisTick: { show: false },
      },
      yAxis: { type: 'value', max: 100, axisLabel: { color: p.textColor, formatter: '{value}%' }, splitLine: { lineStyle: { color: p.splitLineColor } } },
      series: [{ type: 'bar', data: ms.map((m) => m.success_rate), itemStyle: { color: '#34c759' }, barMaxWidth: 32 }],
    }
  }, [data, p])

  if (loading) return <SkeletonCard title="请求结果" />
  if (!data) {
    return (
      <ChartCard title="请求结果" sub="成功率 / 失败率 / 各模型成功率">
        <EmptyHint />
      </ChartCard>
    )
  }
  return (
    <ChartCard title="请求结果" sub="成功率分母含失败请求（n+1 口径）· 运维重点">
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 mb-4">
        <MetricCard label="成功次数" value={formatNumber(data.success_requests)} tone="pos" />
        <MetricCard label="失败次数" value={formatNumber(data.error_requests)} tone={data.error_requests > 0 ? 'neg' : 'neutral'} />
        <MetricCard label="成功率" value={PCT(data.success_rate)} tone="pos" />
        <MetricCard label="失败率" value={PCT(data.error_rate)} tone={data.error_rate > 5 ? 'neg' : 'neutral'} />
      </div>
      {opt ? <EChart option={opt} height={260} /> : <EmptyHint compact />}
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
