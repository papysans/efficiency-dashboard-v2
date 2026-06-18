// 项目/仓库主体的「按周时间线」图 —— 数据来自 /v2/repo-trend、/v2/project-trend（后端从底层 commits/needs
// 现聚合好的周点 EntityTrendPoint[]）。与 DimensionTrend（user×week 周表、ratio 需 ×100）刻意分开：
//   ⚠️ 这里 efficiency_pct 后端已归一为百分比（300=300%），前端直接画，绝不再 ×100。
// 一个组件按 metric 选画：提效率(%) / 提交数 / 代码行 / 需求数——复用同款玻璃卡 + EChart 主题。
import { useMemo } from 'react'
import * as echarts from 'echarts'
import type { EChartsOption } from 'echarts'
import { useTheme } from '@/hooks/useTheme'
import { EChart } from '@/components/charts/EChart'
import { getPalette } from '@/components/charts/chartTheme'
import { formatNumber } from '@/lib/formatters'
import type { EntityTrendPoint } from '@/api/types'

export type TrendMetric = 'efficiency' | 'commits' | 'loc' | 'needs' | 'cost'

interface MetricCfg {
  pick: (p: EntityTrendPoint) => number
  label: string
  fmt: (v: number) => string
}
const METRIC_CFG: Record<TrendMetric, MetricCfg> = {
  efficiency: { pick: (p) => p.efficiency_pct, label: '平均提效率', fmt: (v) => `${v.toFixed(1)}%` },
  commits: { pick: (p) => p.commit_count, label: '提交数', fmt: (v) => formatNumber(v) },
  loc: { pick: (p) => p.diff_lines, label: '代码行', fmt: (v) => `${formatNumber(v)} 行` },
  needs: { pick: (p) => p.need_count, label: '需求数', fmt: (v) => formatNumber(v) },
  cost: { pick: (p) => p.cost ?? 0, label: '会话费用', fmt: (v) => `¥${formatNumber(v, 2)}` },
}

interface Props {
  points: EntityTrendPoint[] | undefined
  loading?: boolean
  error?: string | null
  title?: string
  subtitle?: string
  metric?: TrendMetric
}

export function EntityWeeklyTrend({
  points,
  loading = false,
  error = null,
  title = '提效趋势',
  subtitle,
  metric = 'efficiency',
}: Props) {
  const { theme } = useTheme()
  const cfg = METRIC_CFG[metric]
  const rows = useMemo(() => points ?? [], [points])

  const option = useMemo<EChartsOption>(() => {
    const p = getPalette(theme)
    const isPct = metric === 'efficiency'
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
          const arr = params as Array<{ dataIndex: number; axisValue: string }>
          const item = arr[0]
          if (!item) return ''
          const pt = rows[item.dataIndex]
          if (!pt) return ''
          return `周一 ${item.axisValue}<br/>${cfg.label}：<b>${cfg.fmt(cfg.pick(pt))}</b>`
        },
      },
      xAxis: {
        type: 'category',
        data: rows.map((pt) => pt.week_start.slice(5)),
        boundaryGap: false,
        axisLine: { lineStyle: { color: p.axisColor } },
        axisLabel: { color: p.textColor },
        axisTick: { show: false },
      },
      yAxis: {
        type: 'value',
        axisLabel: { color: p.textColor, formatter: isPct ? '{value}%' : '{value}' },
        splitLine: { lineStyle: { color: p.splitLineColor } },
      },
      series: [
        {
          type: 'line',
          smooth: true,
          symbol: 'circle',
          symbolSize: 7,
          data: rows.map((pt) => cfg.pick(pt)),
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
  }, [rows, theme, cfg, metric])

  return (
    <div className="glass rounded-2xl p-5 md:p-6 min-h-[20rem] hover:shadow-lg transition-shadow flex flex-col">
      <div className="flex items-center justify-between mb-4 gap-3">
        <h2 className="text-sm font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide">{title}</h2>
        {subtitle && <span className="text-xs text-gray-400 dark:text-gray-500 text-right">{subtitle}</span>}
      </div>

      {error ? (
        <Centered>加载失败：{error}</Centered>
      ) : loading ? (
        <div className="flex-1 skeleton rounded-xl min-h-[16rem]" />
      ) : rows.length < 2 ? (
        <Centered>
          <div className="flex flex-col items-center gap-2 text-center">
            <TrendIcon />
            <p className="text-sm text-gray-500 dark:text-gray-400">趋势数据积累中（当前样本较少）</p>
            {rows.length === 1 && (
              <p className="text-xs text-gray-400 dark:text-gray-500">
                本期数据集中在单周（{rows[0].week_start} 起 · {cfg.label} {cfg.fmt(cfg.pick(rows[0]))}）
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
  return (
    <div className="flex-1 flex items-center justify-center text-sm text-gray-500 dark:text-gray-400 min-h-[16rem]">
      {children}
    </div>
  )
}

function TrendIcon() {
  return (
    <svg className="w-10 h-10 text-gray-300 dark:text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M3 13.125C3 12.504 3.504 12 4.125 12h2.25c.621 0 1.125.504 1.125 1.125v6.75C7.5 20.496 6.996 21 6.375 21h-2.25A1.125 1.125 0 013 19.875v-6.75zM9.75 8.625c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125v11.25c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V8.625zM16.5 4.125c0-.621.504-1.125 1.125-1.125h2.25C20.496 3 21 3.504 21 4.125v15.75c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V4.125z" />
    </svg>
  )
}
