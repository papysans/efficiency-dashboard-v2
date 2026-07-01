// 使用维度共享的 ECharts 趋势图 option 构建器。
// 双Y轴折线图：左轴=请求量，右轴=Token（防大数压扁）或单轴按需。
// 按天粒度（替代旧的按周 PlatformWeekTrend）。

import * as echarts from 'echarts'
import type { EChartsOption } from 'echarts'
import type { ChartPalette } from '@/components/charts/chartTheme'
import { formatNumber } from '@/lib/formatters'

export interface TrendSeriesItem {
  name: string
  color: string
  data: number[]
  /** 图形类型：line=折线(默认,左轴带面积) / bar=柱状。组合图用「量」柱 +「人」线。 */
  type?: 'bar' | 'line'
  /** 落在哪个 Y 轴：left=主轴(面积) / right=次轴 / third=第三轴(独立刻度，如使用率%)。 */
  axis?: 'left' | 'right' | 'third'
  /** tooltip 里该序列值的格式化（缺省千分位）；如使用率传 v=>`${v.toFixed(1)}%`。 */
  tipFmt?: (v: number) => string
}

/**
 * 构建多Y轴折线图 option（粒度无关：x 轴标签由调用方按天/周/月给定）。
 * 左(area)/右(虚线)各自独立刻度；series 标 axis:'third' 时启用第三轴（独立刻度，右侧外移，如使用率%）。
 * headers 提供时（按周/月聚合），tooltip 头部用 headers[dataIndex]（日期范围）替代 x 轴标签。
 */
export function buildDualAxisTrendOption(
  p: ChartPalette,
  labels: string[],
  series: TrendSeriesItem[],
  opts: {
    leftFmt?: (v: number) => string
    rightFmt?: (v: number) => string
    thirdFmt?: (v: number) => string
    thirdMax?: number
    headers?: string[]
    /** 追加到 tooltip 末尾的额外行（如「使用率」——它由活跃用户派生，不单独占一条线）。返回空串则不加。 */
    extraTooltip?: (dataIndex: number) => string
  } = {},
): EChartsOption {
  const headers = opts.headers
  const hasThird = series.some((s) => s.axis === 'third')
  const hasBar = series.some((s) => s.type === 'bar')
  const fmtByName = new Map(series.map((s) => [s.name, s.tipFmt ?? formatNumber]))
  const axisIndex = (s: TrendSeriesItem) => (s.axis === 'third' ? 2 : s.axis === 'right' ? 1 : 0)

  const yAxis: EChartsOption['yAxis'] = [
    {
      type: 'value',
      axisLabel: { color: p.textColor, formatter: opts.leftFmt },
      splitLine: { lineStyle: { color: p.splitLineColor } },
    },
    {
      type: 'value',
      axisLabel: { color: p.textColor, formatter: opts.rightFmt },
      splitLine: { show: false },
    },
  ]
  if (hasThird) {
    yAxis.push({
      type: 'value',
      position: 'right',
      offset: 52,
      min: 0,
      max: opts.thirdMax,
      axisLabel: { color: p.textColor, formatter: opts.thirdFmt },
      splitLine: { show: false },
    })
  }

  return {
    animation: true,
    grid: { left: 8, right: hasThird ? 76 : 16, top: 36, bottom: 8, containLabel: true },
    tooltip: {
      trigger: 'axis',
      backgroundColor: p.tooltipBg,
      borderColor: p.tooltipBorder,
      borderWidth: 1,
      textStyle: { color: p.tooltipText },
      formatter: (params: unknown) => {
        const arr = params as { dataIndex: number; seriesName: string; value: number; marker: string; axisValue: string }[]
        const idx = arr[0]?.dataIndex
        const head = headers ? (headers[idx] ?? arr[0]?.axisValue ?? '') : (arr[0]?.axisValue ?? '')
        const body = arr
          .map((it) => `${it.marker}${it.seriesName}: ${(fmtByName.get(it.seriesName) ?? formatNumber)(it.value)}`)
          .join('<br/>')
        const extra = opts.extraTooltip ? opts.extraTooltip(idx) : ''
        return `${head}<br/>${body}${extra ? `<br/>${extra}` : ''}`
      },
    },
    legend: {
      top: 0,
      left: 'center',
      textStyle: { color: p.textColor },
      itemWidth: 14,
      itemHeight: 8,
    },
    xAxis: {
      type: 'category',
      data: labels,
      // 含柱状时留边（柱居格内、首尾不贴边）；纯折线/面积则贴边铺满。
      boundaryGap: hasBar,
      axisLine: { lineStyle: { color: p.axisColor } },
      axisLabel: { color: p.textColor, hideOverlap: true },
      axisTick: { show: false },
    },
    yAxis,
    series: series.map((s) => {
      const yi = axisIndex(s)
      const onLeft = yi === 0
      if (s.type === 'bar') {
        return {
          name: s.name,
          type: 'bar',
          yAxisIndex: yi,
          barMaxWidth: 28,
          data: s.data,
          itemStyle: {
            borderRadius: [4, 4, 0, 0],
            color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
              { offset: 0, color: rgba(s.color, 0.85) },
              { offset: 1, color: rgba(s.color, 0.35) },
            ]),
          },
        }
      }
      return {
        name: s.name,
        type: 'line',
        yAxisIndex: yi,
        smooth: true,
        symbol: 'circle',
        symbolSize: 6,
        data: s.data,
        lineStyle: { color: s.color, width: 2 },
        itemStyle: { color: s.color },
        // 与柱同图时折线不铺面积（避免盖住柱）；纯折线图左轴序列仍铺渐变面积。
        ...(onLeft && !hasBar
          ? {
              areaStyle: {
                color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
                  { offset: 0, color: rgba(s.color, 0.25) },
                  { offset: 1, color: rgba(s.color, 0) },
                ]),
              },
            }
          : {}),
      }
    }),
  }
}

function rgba(hex: string, alpha: number): string {
  const r = parseInt(hex.slice(1, 3), 16)
  const g = parseInt(hex.slice(3, 5), 16)
  const b = parseInt(hex.slice(5, 7), 16)
  return `rgba(${r},${g},${b},${alpha})`
}
