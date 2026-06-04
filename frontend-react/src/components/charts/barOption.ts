// ECharts 柱状图 option 工厂 —— 移植自 Vue frontend/src/utils/kanbanChart.js，
// 叠加 chartTheme 亮暗适配（坐标轴/网格/文字/tooltip 色）。Need 详情基线/阶段两图用。
import type { EChartsOption } from 'echarts'
import { getPalette, type ChartTheme } from './chartTheme'

export interface BarSeries {
  name: string
  data: number[]
  /** 覆盖默认 series 类型（默认 bar） */
  type?: 'bar' | 'line'
}

export interface BarOptionOpts {
  /** 标题字号（默认 13） */
  titleSize?: number
  /** tooltip 数值格式化（如 formatDuration） */
  format?: (v: number) => string
  /** 默认 series 类型 */
  type?: 'bar' | 'line'
}

/**
 * 生成柱状图 option（与 kanbanChart.js 逻辑一致）：
 * - 标题含「提效 / Efficiency / Ratio」时 y 轴自动 `{value}%`。
 * - format 存在时 tooltip 用自定义 axis formatter。
 */
export function barOption(
  theme: ChartTheme,
  title: string,
  labels: string[],
  list: BarSeries[],
  opts: BarOptionOpts = {},
): EChartsOption {
  const p = getPalette(theme)
  const interval = labels.length > 18 ? Math.ceil(labels.length / 18) - 1 : 0
  const defaultType = opts.type ?? 'bar'
  const isRatio = title.includes('提效') || title.includes('Efficiency') || title.includes('Ratio')

  return {
    animation: true,
    title: {
      text: title,
      top: 10,
      left: 'center',
      textStyle: { fontSize: opts.titleSize ?? 13, fontWeight: 'bold', color: p.textColor },
    },
    tooltip: opts.format
      ? {
          trigger: 'axis',
          backgroundColor: p.tooltipBg,
          borderColor: p.tooltipBorder,
          borderWidth: 1,
          textStyle: { color: p.tooltipText },
          formatter: (items: unknown) => {
            const rows = (Array.isArray(items) ? items : [items]) as Array<{
              axisValue: string
              marker: string
              seriesName: string
              value: number
            }>
            return rows.reduce(
              (txt, item, index) =>
                `${txt}${index === 0 ? `${item.axisValue}<br/>` : ''}${item.marker}${item.seriesName}: ${opts.format!(
                  Number(item.value ?? 0),
                )}<br/>`,
              '',
            )
          },
        }
      : {
          trigger: 'axis',
          backgroundColor: p.tooltipBg,
          borderColor: p.tooltipBorder,
          borderWidth: 1,
          textStyle: { color: p.tooltipText },
        },
    legend: {
      data: list.map((item) => item.name),
      top: 40,
      left: 20,
      right: 20,
      type: 'scroll',
      textStyle: { color: p.textColor },
    },
    grid: { left: '5%', right: '5%', top: list.length > 1 ? 92 : 56, bottom: 44, containLabel: true },
    xAxis: {
      type: 'category',
      data: labels,
      axisLine: { lineStyle: { color: p.axisColor } },
      axisLabel: { rotate: 0, fontSize: 11, margin: 12, hideOverlap: true, interval, color: p.textColor },
    },
    yAxis: isRatio
      ? {
          type: 'value',
          axisLabel: { formatter: '{value}%', color: p.textColor },
          splitLine: { lineStyle: { color: p.splitLineColor } },
        }
      : {
          type: 'value',
          axisLabel: { color: p.textColor },
          splitLine: { lineStyle: { color: p.splitLineColor } },
        },
    series: list.map((item) => {
      const next = item.type ?? defaultType
      return {
        name: item.name,
        type: next,
        smooth: next === 'line',
        data: item.data,
        itemStyle: { color: p.brand },
      }
    }),
  }
}
