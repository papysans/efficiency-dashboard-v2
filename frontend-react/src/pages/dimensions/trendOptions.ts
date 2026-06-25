// 使用维度共享的 ECharts 趋势图 option 构建器。
// 双Y轴折线图：左轴=请求量，右轴=Token（防大数压扁）或单轴按需。
// 按天粒度（替代旧的按周 PlatformWeekTrend）。

import * as echarts from 'echarts'
import type { EChartsOption } from 'echarts'
import type { ChartPalette } from '@/components/charts/chartTheme'

export interface TrendSeriesItem {
  name: string
  color: string
  data: number[]
  axis?: 'left' | 'right'
}

/**
 * 构建双Y轴按天折线图 option。
 * 左右轴各自独立刻度，tooltip 两边都显。
 * leftFmt/rightFmt 控制各轴刻度格式（量级缩写等）。
 */
export function buildDualAxisTrendOption(
  p: ChartPalette,
  labels: string[],
  series: TrendSeriesItem[],
  opts: { leftFmt?: (v: number) => string; rightFmt?: (v: number) => string } = {},
): EChartsOption {
  return {
    animation: true,
    grid: { left: 8, right: 16, top: 36, bottom: 8, containLabel: true },
    tooltip: {
      trigger: 'axis',
      backgroundColor: p.tooltipBg,
      borderColor: p.tooltipBorder,
      borderWidth: 1,
      textStyle: { color: p.tooltipText },
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
      boundaryGap: false,
      axisLine: { lineStyle: { color: p.axisColor } },
      axisLabel: { color: p.textColor, hideOverlap: true },
      axisTick: { show: false },
    },
    yAxis: [
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
    ],
    series: series.map((s) => {
      const onRight = s.axis === 'right'
      return {
        name: s.name,
        type: 'line',
        yAxisIndex: onRight ? 1 : 0,
        smooth: true,
        symbol: 'circle',
        symbolSize: 6,
        data: s.data,
        lineStyle: {
          color: s.color,
          width: 2,
          ...(onRight ? { type: 'dashed' as const } : {}),
        },
        itemStyle: { color: s.color },
        ...(onRight
          ? {}
          : {
              areaStyle: {
                color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
                  { offset: 0, color: rgba(s.color, 0.25) },
                  { offset: 1, color: rgba(s.color, 0) },
                ]),
              },
            }),
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
