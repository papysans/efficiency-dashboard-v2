// 7×24 请求热力图工具 —— 平台无 weekday×hour 合成端点（distribution/hourly 压平星期、
// distribution/weekly 压平小时），故前端拉 /stats/hourly 原始行按 (weekday,hour) 分桶纯算，
// 零平台改动（依据 research/platform-realtime-and-recheck.md #3）。分桶函数与 option 分离，便于单测。
import type { EChartsOption } from 'echarts'
import type { ChartPalette } from '@/components/charts/chartTheme'

/** /stats/hourly 原始行（HourlyMetricsSummary，date_hour 为本地 Asia/Shanghai 时字符串）。 */
export interface HourlyRow {
  date_hour: string // 'YYYY-MM-DDTHH:00:00'
  total_requests: number
  total_users?: number
  error_requests?: number
}

// 周一..周日（ISO 周序，y 轴自上而下）
export const WEEKDAY_LABELS = ['周一', '周二', '周三', '周四', '周五', '周六', '周日']
export const HOUR_LABELS = Array.from({ length: 24 }, (_, h) => `${h}时`)

/**
 * 把小时原始行按 (weekday,hour) 累加 total_requests。
 * weekday 输出 0=周一..6=周日（由 JS getDay() 的 0=周日 映射而来）。
 * date_hour 用字符串切片解析，仅日期部分进 Date 算星期，避免 datetime 时区歧义。
 * 返回 echarts heatmap 需要的 [hour, weekday, value][] 与最大值（喂 visualMap）。
 */
export function bucketHourlyToHeatmap(rows: HourlyRow[]): { data: [number, number, number][]; max: number } {
  const grid = new Map<string, number>() // key = `${hour}_${weekday}`
  for (const r of rows) {
    const dh = r?.date_hour
    if (!dh || dh.length < 13) continue
    const y = Number(dh.slice(0, 4))
    const mo = Number(dh.slice(5, 7))
    const d = Number(dh.slice(8, 10))
    const hour = Number(dh.slice(11, 13))
    if (!Number.isFinite(y) || !Number.isFinite(mo) || !Number.isFinite(d) || !Number.isFinite(hour)) continue
    const jsDow = new Date(y, mo - 1, d).getDay() // 0=周日..6=周六
    const wd = (jsDow + 6) % 7 // → 0=周一..6=周日
    const key = `${hour}_${wd}`
    grid.set(key, (grid.get(key) || 0) + (Number(r.total_requests) || 0))
  }
  const data: [number, number, number][] = []
  let max = 0
  for (let h = 0; h < 24; h++) {
    for (let wd = 0; wd < 7; wd++) {
      const v = grid.get(`${h}_${wd}`) || 0
      data.push([h, wd, v])
      if (v > max) max = v
    }
  }
  return { data, max }
}

/** 7×24 请求量热力图 option（x=0-23 时，y=周一..周日）。 */
export function heatmapOption(p: ChartPalette, data: [number, number, number][], max: number): EChartsOption {
  return {
    animation: false,
    tooltip: {
      position: 'top',
      backgroundColor: p.tooltipBg,
      borderColor: p.tooltipBorder,
      borderWidth: 1,
      textStyle: { color: p.tooltipText },
      formatter: (pt: unknown) => {
        const v = (pt as { value: [number, number, number] }).value
        return `${WEEKDAY_LABELS[v[1]]} ${HOUR_LABELS[v[0]]}<br/>请求量：${v[2].toLocaleString()}`
      },
    },
    grid: { left: 44, right: 12, top: 8, bottom: 56, containLabel: true },
    xAxis: {
      type: 'category',
      data: HOUR_LABELS,
      splitArea: { show: true },
      axisLine: { lineStyle: { color: p.axisColor } },
      axisLabel: { color: p.textColor, fontSize: 10, interval: 1 },
      axisTick: { show: false },
    },
    yAxis: {
      type: 'category',
      data: WEEKDAY_LABELS,
      splitArea: { show: true },
      axisLine: { lineStyle: { color: p.axisColor } },
      axisLabel: { color: p.textColor, fontSize: 11 },
      axisTick: { show: false },
    },
    visualMap: {
      min: 0,
      max: Math.max(max, 1),
      calculable: true,
      orient: 'horizontal',
      left: 'center',
      bottom: 4,
      inRange: { color: ['#e6f2ff', '#0071e3', '#003a78'] },
      textStyle: { color: p.textColor },
    },
    series: [
      {
        type: 'heatmap',
        data,
        progressive: 0,
        label: { show: false },
        emphasis: { itemStyle: { shadowBlur: 8, shadowColor: 'rgba(0,0,0,0.3)' } },
      },
    ],
  }
}
