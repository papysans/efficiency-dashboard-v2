// 平台运维 ·「请求性能」Tab —— 接平台现成性能端点（research/platform-realtime-and-recheck.md）：
//   /stats/performance/overview（区间加权均值）· /performance/by-model（各模型）· /global/daily（按日趋势）。
// 评审要求：TTFT/输出速度/端到端耗时三卡须标注口径+单位+样本量；成功率按日趋势。纯前端，零平台改动。
import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import type { EChartsOption } from 'echarts'
import { chatGet } from '@/api/client'
import { useTheme } from '@/hooks/useTheme'
import { getPalette, type ChartPalette } from '@/components/charts/chartTheme'
import { EChart } from '@/components/charts/EChart'
import { formatNumber } from '@/lib/formatters'
import { ChartCard, EmptyHint, multiAreaOption } from './platformShared'

// ---- 页面局部类型（字段实证自平台 web-ui/MetricsDashboard.jsx）----
interface PerfOverview {
  avg_ttft_ms: number | null
  avg_token_output_speed: number | null
  avg_duration_ms: number | null
}
interface PerfModel {
  model: string
  avg_ttft_ms: number | null
  avg_token_output_speed: number | null
  avg_duration_ms: number | null
}
interface PerfByModelResp {
  models: PerfModel[]
}
interface DailyPerfRow {
  date: string
  avg_duration_ms: number | null
  total_requests: number
  total_error_requests: number
  total_requests_including_errors: number
}

const fmtMs = (v: number | null | undefined) => (v != null ? `${Number(v).toFixed(0)} ms` : '-')
const shortDate = (s: string) => s.slice(5, 10)

/** 横向柱状（模型名做 y 轴，避免 x 轴文字拥挤）。unit 决定 tooltip 精度（ms 取整 / t/s 一位）。 */
function hbarOption(p: ChartPalette, labels: string[], values: number[], unit: string, color: string): EChartsOption {
  return {
    animation: true,
    grid: { left: 8, right: 24, top: 8, bottom: 8, containLabel: true },
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' },
      backgroundColor: p.tooltipBg,
      borderColor: p.tooltipBorder,
      borderWidth: 1,
      textStyle: { color: p.tooltipText },
      valueFormatter: (v) => `${Number(v ?? 0).toFixed(unit === 'ms' ? 0 : 1)} ${unit}`,
    },
    xAxis: {
      type: 'value',
      axisLabel: { color: p.textColor },
      splitLine: { lineStyle: { color: p.splitLineColor } },
    },
    yAxis: {
      type: 'category',
      data: labels,
      axisLine: { lineStyle: { color: p.axisColor } },
      axisLabel: { color: p.textColor, fontSize: 11 },
      axisTick: { show: false },
    },
    series: [{ type: 'bar', data: values, itemStyle: { color, borderRadius: [0, 4, 4, 0] }, barMaxWidth: 18 }],
  }
}

export default function PerformanceTab({
  start,
  end,
  enabled,
  refetchInterval,
}: {
  start: string
  end: string
  enabled: boolean
  refetchInterval: number | false
}) {
  const { theme } = useTheme()
  const p = useMemo(() => getPalette(theme), [theme])

  const overviewQ = useQuery({
    queryKey: ['perf-overview', start, end],
    queryFn: () => chatGet<PerfOverview>('/stats/performance/overview', { start_date: start, end_date: end }),
    enabled,
    refetchInterval,
  })
  const byModelQ = useQuery({
    queryKey: ['perf-by-model', start, end],
    queryFn: () => chatGet<PerfByModelResp>('/stats/performance/by-model', { start_date: start, end_date: end }),
    enabled,
    refetchInterval,
  })
  const dailyQ = useQuery({
    queryKey: ['perf-daily', start, end],
    queryFn: () => chatGet<DailyPerfRow[]>('/stats/global/daily', { start_date: start, end_date: end }),
    enabled,
    refetchInterval,
  })

  const po = overviewQ.data
  const daily = useMemo(() => dailyQ.data ?? [], [dailyQ.data])
  const sampleRequests = useMemo(() => daily.reduce((s, r) => s + (r.total_requests || 0), 0), [daily])

  // 各模型性能：过滤掉无数据(null)的废弃/边缘模型，避免一排 0 柱误导。
  // TTFT 升序(快者在上)，输出速度降序(快者在上)。
  const ttftModels = useMemo(
    () =>
      (byModelQ.data?.models ?? [])
        .filter((m) => (m.avg_ttft_ms ?? 0) > 0)
        .sort((a, b) => (a.avg_ttft_ms ?? 0) - (b.avg_ttft_ms ?? 0)),
    [byModelQ.data],
  )
  const speedModels = useMemo(
    () =>
      (byModelQ.data?.models ?? [])
        .filter((m) => (m.avg_token_output_speed ?? 0) > 0)
        .sort((a, b) => (b.avg_token_output_speed ?? 0) - (a.avg_token_output_speed ?? 0)),
    [byModelQ.data],
  )

  const kpis = [
    {
      title: '平均 TTFT',
      value: fmtMs(po?.avg_ttft_ms),
      sub: `首 Token 时延 · 加权均值 · 样本 ${formatNumber(sampleRequests)} 请求`,
    },
    {
      title: '平均 Token 输出速度',
      value: po?.avg_token_output_speed != null ? `${po.avg_token_output_speed.toFixed(1)} t/s` : '-',
      sub: `生成阶段 tokens/秒 · 加权均值 · 样本 ${formatNumber(sampleRequests)} 请求`,
    },
    {
      title: '平均端到端耗时',
      value: fmtMs(po?.avg_duration_ms),
      sub: `请求开始→完整响应 · 加权均值 · 样本 ${formatNumber(sampleRequests)} 请求`,
    },
  ]

  // 端到端耗时趋势（按日）
  const durTrendOpt = useMemo(
    () =>
      multiAreaOption(
        p,
        daily.map((r) => shortDate(r.date)),
        [{ name: '平均端到端耗时', color: '#ff3b30', data: daily.map((r) => Math.round(r.avg_duration_ms ?? 0)) }],
        { yFmt: (v) => `${v} ms` },
      ),
    [daily, p],
  )

  // 成功率趋势（按日）= 成功请求 / 含错误总请求
  const successTrendOpt = useMemo(
    () =>
      multiAreaOption(
        p,
        daily.map((r) => shortDate(r.date)),
        [
          {
            name: '成功率',
            color: '#34c759',
            data: daily.map((r) => {
              const total = r.total_requests_including_errors > 0 ? r.total_requests_including_errors : r.total_requests
              return total > 0 ? +(((total - r.total_error_requests) / total) * 100).toFixed(1) : 0
            }),
          },
        ],
        { yFmt: (v) => `${v}%`, yMax: 100 },
      ),
    [daily, p],
  )

  const ttftOpt = useMemo(
    () => hbarOption(p, ttftModels.map((m) => m.model || '-'), ttftModels.map((m) => Math.round(m.avg_ttft_ms ?? 0)), 'ms', '#0071e3'),
    [ttftModels, p],
  )
  const speedOpt = useMemo(
    () => hbarOption(p, speedModels.map((m) => m.model || '-'), speedModels.map((m) => +(m.avg_token_output_speed ?? 0).toFixed(1)), 't/s', '#af52de'),
    [speedModels, p],
  )

  return (
    <div className="space-y-4">
      {/* 性能 KPI 三卡 */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
        {kpis.map((k) => (
          <div key={k.title} className="glass rounded-2xl p-4 hover:shadow-lg transition-shadow">
            <div className="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide mb-1">
              {k.title}
            </div>
            <div className="text-2xl font-bold tabular-nums text-gray-900 dark:text-white">{k.value}</div>
            <div className="text-xs text-gray-400 dark:text-gray-500 mt-1">{k.sub}</div>
          </div>
        ))}
      </div>

      {/* 趋势：耗时 + 成功率 */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <ChartCard title="平均端到端耗时趋势" sub="按日 · 加权均值">
          {daily.length > 0 ? <EChart option={durTrendOpt} height={260} /> : <EmptyHint />}
        </ChartCard>
        <ChartCard title="请求成功率趋势" sub="按日 · 成功÷含错误总请求">
          {daily.length > 0 ? <EChart option={successTrendOpt} height={260} /> : <EmptyHint />}
        </ChartCard>
      </div>

      {/* 各模型性能对比 */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <ChartCard title="各模型平均 TTFT" sub="首 Token 时延 (ms) · 升序 · 仅含有数据模型">
          {ttftModels.length > 0 ? <EChart option={ttftOpt} height={Math.max(220, ttftModels.length * 28)} /> : <EmptyHint />}
        </ChartCard>
        <ChartCard title="各模型平均输出速度" sub="tokens/秒 · 降序 · 仅含有数据模型">
          {speedModels.length > 0 ? <EChart option={speedOpt} height={Math.max(220, speedModels.length * 28)} /> : <EmptyHint />}
        </ChartCard>
      </div>
    </div>
  )
}
