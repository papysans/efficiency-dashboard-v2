// 平台运维 ·「时段分布」Tab —— 单日 0-23 小时请求量/活跃用户（/stats/distribution/hourly，单日精确去重）
//  + 7×24 请求热力图（/stats/hourly 区间原始行前端分桶，见 monitoringHeatmap）。纯前端，零平台改动。
import { useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import type { EChartsOption } from 'echarts'
import { chatGet } from '@/api/client'
import { useTheme } from '@/hooks/useTheme'
import { getPalette, type ChartPalette } from '@/components/charts/chartTheme'
import { EChart } from '@/components/charts/EChart'
import { ChartCard, EmptyHint } from './platformShared'
import { bucketHourlyToHeatmap, heatmapOption, HOUR_LABELS, type HourlyRow } from './monitoringHeatmap'

interface HourPoint {
  hour: number
  request_count: number
  active_users: number
}
interface HourlyDistResp {
  hours: HourPoint[]
}

const INPUT_CLS =
  'glass rounded-lg px-3 py-1.5 text-sm bg-transparent text-gray-900 dark:text-white ' +
  'focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue'

/** 0-23 小时竖向柱状。 */
function vbarHourOption(p: ChartPalette, values: number[], color: string, name: string): EChartsOption {
  return {
    animation: true,
    grid: { left: 8, right: 16, top: 8, bottom: 8, containLabel: true },
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' },
      backgroundColor: p.tooltipBg,
      borderColor: p.tooltipBorder,
      borderWidth: 1,
      textStyle: { color: p.tooltipText },
    },
    xAxis: {
      type: 'category',
      data: HOUR_LABELS,
      axisLine: { lineStyle: { color: p.axisColor } },
      axisLabel: { color: p.textColor, fontSize: 10, interval: 1 },
      axisTick: { show: false },
    },
    yAxis: {
      type: 'value',
      axisLabel: { color: p.textColor },
      splitLine: { lineStyle: { color: p.splitLineColor } },
    },
    series: [{ name, type: 'bar', data: values, itemStyle: { color, borderRadius: [3, 3, 0, 0] } }],
  }
}

export default function TimeDistributionTab({
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

  // 单日选择器（小时分布用，默认区间末日）
  const [day, setDay] = useState(end)
  // 区间变化后把越界的单日拉回区间末日（否则会查/显示区间外的某天，与 Tab 其余图口径不一致）
  useEffect(() => {
    if (day < start || day > end) setDay(end)
  }, [start, end, day])
  const dayValid = !!day && day <= end && day >= start

  const hourlyQ = useQuery({
    queryKey: ['timedist-hourly', day],
    queryFn: () => chatGet<HourlyDistResp>('/stats/distribution/hourly', { start_date: day, end_date: day }),
    enabled: enabled && dayValid,
    refetchInterval,
  })
  const heatmapQ = useQuery({
    queryKey: ['timedist-heatmap', start, end],
    queryFn: () => chatGet<HourlyRow[]>('/stats/hourly', { start_hour: `${start}T00:00:00`, end_hour: `${end}T23:00:00` }),
    enabled,
    refetchInterval,
  })

  const hours = useMemo(() => {
    const hs = hourlyQ.data?.hours ?? []
    // 补齐缺失小时为 0，保证 0-23 完整
    const byHour = new Map(hs.map((h) => [h.hour, h]))
    return Array.from({ length: 24 }, (_, h) => byHour.get(h) ?? { hour: h, request_count: 0, active_users: 0 })
  }, [hourlyQ.data])

  const reqOpt = useMemo(() => vbarHourOption(p, hours.map((h) => h.request_count), '#0071e3', '请求量'), [hours, p])
  const userOpt = useMemo(() => vbarHourOption(p, hours.map((h) => h.active_users), '#34c759', '活跃用户'), [hours, p])

  const heat = useMemo(() => bucketHourlyToHeatmap(heatmapQ.data ?? []), [heatmapQ.data])
  const heatOpt = useMemo(() => heatmapOption(p, heat.data, heat.max), [heat, p])
  const hasHourly = (hourlyQ.data?.hours?.length ?? 0) > 0

  return (
    <div className="space-y-4">
      {/* 单日小时分布 */}
      <div className="flex items-center gap-2">
        <label className="text-sm text-gray-500 dark:text-gray-400">单日时段：</label>
        <input
          type="date"
          value={day}
          min={start}
          max={end}
          onChange={(e) => setDay(e.target.value)}
          className={INPUT_CLS}
          aria-label="选择单日查看 0-23 时分布"
        />
        {!dayValid && <span className="text-xs text-rose-500">请选择区间内的日期</span>}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <ChartCard title="按小时请求量" sub={`${day} · 0-23 时`}>
          {hasHourly ? <EChart option={reqOpt} height={280} /> : <EmptyHint />}
        </ChartCard>
        <ChartCard title="按小时活跃用户" sub={`${day} · 0-23 时 · 单日去重`}>
          {hasHourly ? <EChart option={userOpt} height={280} /> : <EmptyHint />}
        </ChartCard>
      </div>

      {/* 7×24 热力图（区间） */}
      <ChartCard title="7×24 请求热力图" sub={`${start} ~ ${end} · 按 (星期, 小时) 聚合请求量`}>
        {heat.max > 0 ? <EChart option={heatOpt} height={320} /> : <EmptyHint />}
      </ChartCard>
    </div>
  )
}
