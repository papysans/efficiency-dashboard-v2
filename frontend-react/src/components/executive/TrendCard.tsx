import { useMemo } from 'react'
import * as echarts from 'echarts'
import type { EChartsOption } from 'echarts'
import { useAllNeeds } from '@/api/queries'
import { useTheme } from '@/hooks/useTheme'
import { EChart } from '@/components/charts/EChart'
import { getPalette } from '@/components/charts/chartTheme'
import { isoWeekOf, weekLabel } from '@/lib/week'
import type { NeedsV2Summary } from '@/api/types'

interface TrendCardProps {
  startDate: string
  endDate: string
}

interface WeekPoint {
  key: string
  label: string
  monday: number
  avgPct: number
  count: number
}

/** 把 eligible needs 按 ISO 周聚合为「每周平均提效率(%)」点列（design-pr1 §1②）。 */
function aggregateByWeek(rows: NeedsV2Summary[]): WeekPoint[] {
  const buckets = new Map<string, { sum: number; count: number; label: string; monday: number }>()
  for (const r of rows) {
    if (!r.coverage_eligible || r.efficiency_ratio == null) continue
    // merge_ts 多为 null，回退到 dev_end_ts 以保证有时间锚点
    const ts = r.merge_ts || r.dev_end_ts
    const wk = isoWeekOf(ts)
    if (!wk) continue
    const cur = buckets.get(wk.key) || { sum: 0, count: 0, label: weekLabel(wk.monday), monday: wk.monday.getTime() }
    cur.sum += r.efficiency_ratio
    cur.count += 1
    buckets.set(wk.key, cur)
  }
  return Array.from(buckets.entries())
    .map(([key, v]) => ({ key, label: v.label, monday: v.monday, avgPct: (v.sum / v.count) * 100, count: v.count }))
    .sort((a, b) => a.monday - b.monday)
}

export function TrendCard({ startDate, endDate }: TrendCardProps) {
  const { theme } = useTheme()
  const { data, isLoading, error } = useAllNeeds({ startDate, endDate })

  const points = useMemo(() => aggregateByWeek(data ?? []), [data])

  const option = useMemo<EChartsOption>(() => {
    const p = getPalette(theme)
    return {
      animation: true,
      grid: { left: 8, right: 16, top: 24, bottom: 8, containLabel: true },
      tooltip: {
        trigger: 'axis',
        backgroundColor: p.tooltipBg,
        borderColor: p.tooltipBorder,
        borderWidth: 1,
        textStyle: { color: p.tooltipText },
        formatter: (params: unknown) => {
          const arr = params as Array<{ dataIndex: number; value: number; axisValue: string }>
          const item = arr[0]
          if (!item) return ''
          const pt = points[item.dataIndex]
          return `周一 ${item.axisValue}<br/>平均提效率：<b>${item.value.toFixed(1)}%</b><br/>样本需求：${pt?.count ?? 0} 个`
        },
      },
      xAxis: {
        type: 'category',
        data: points.map((pt) => pt.label),
        boundaryGap: false,
        axisLine: { lineStyle: { color: p.axisColor } },
        axisLabel: { color: p.textColor },
        axisTick: { show: false },
      },
      yAxis: {
        type: 'value',
        axisLabel: { color: p.textColor, formatter: '{value}%' },
        splitLine: { lineStyle: { color: p.splitLineColor } },
      },
      series: [
        {
          type: 'line',
          smooth: true,
          symbol: 'circle',
          symbolSize: 7,
          data: points.map((pt) => Number(pt.avgPct.toFixed(2))),
          lineStyle: { color: p.brand, width: 3 },
          itemStyle: { color: p.brand },
          areaStyle: {
            color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
              { offset: 0, color: p.areaTop },
              { offset: 1, color: p.areaBottom },
            ]),
          },
        },
      ],
    }
  }, [points, theme])

  return (
    <div className="glass rounded-2xl p-5 md:p-6 min-h-[20rem] hover:shadow-lg transition-shadow flex flex-col">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-sm font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide">提效趋势</h2>
        <span className="text-xs text-gray-400 dark:text-gray-500">按 ISO 周 · 可计入需求平均日历提效</span>
      </div>

      {error ? (
        <Centered>加载失败：{(error as Error).message}</Centered>
      ) : isLoading ? (
        <div className="flex-1 skeleton rounded-xl min-h-[16rem]" />
      ) : points.length < 2 ? (
        <Centered>
          <div className="flex flex-col items-center gap-2 text-center">
            <svg className="w-10 h-10 text-gray-300 dark:text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M3 13.125C3 12.504 3.504 12 4.125 12h2.25c.621 0 1.125.504 1.125 1.125v6.75C7.5 20.496 6.996 21 6.375 21h-2.25A1.125 1.125 0 013 19.875v-6.75zM9.75 8.625c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125v11.25c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V8.625zM16.5 4.125c0-.621.504-1.125 1.125-1.125h2.25C20.496 3 21 3.504 21 4.125v15.75c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V4.125z" />
            </svg>
            <p className="text-sm text-gray-500 dark:text-gray-400">趋势数据积累中（当前样本较少）</p>
            {points.length === 1 && (
              <p className="text-xs text-gray-400 dark:text-gray-500">
                本期可计入需求集中在单周（{points[0].label} 起 · {points[0].count} 个 · 平均 {points[0].avgPct.toFixed(1)}%）
              </p>
            )}
          </div>
        </Centered>
      ) : (
        <div className="flex-1">
          <EChart option={option} height={260} />
        </div>
      )}
    </div>
  )
}

function Centered({ children }: { children: React.ReactNode }) {
  return <div className="flex-1 flex items-center justify-center text-sm text-gray-500 dark:text-gray-400 min-h-[16rem]">{children}</div>
}
