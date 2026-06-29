import { describe, it, expect } from 'vitest'
import { bucketHourlyToHeatmap, type HourlyRow } from './monitoringHeatmap'

// 7×24 热力图分桶纯逻辑回归网：把 /stats/hourly 原始行(date_hour)按 (weekday,hour) 累加请求量。
// 重点防回归：weekday 映射(JS getDay 0=周日 → 输出 0=周一..6=周日) 与 datetime 字符串切片解析。
const cell = (data: [number, number, number][], hour: number, wd: number) =>
  data.find((d) => d[0] === hour && d[1] === wd)?.[2]

describe('bucketHourlyToHeatmap', () => {
  it('空输入产出 168 个全 0 格子，max 为 0', () => {
    const { data, max } = bucketHourlyToHeatmap([])
    expect(data).toHaveLength(24 * 7)
    expect(max).toBe(0)
    expect(data.every((d) => d[2] === 0)).toBe(true)
  })

  it('weekday 映射：周一(2024-01-01)落 wd=0，周日(2024-01-07)落 wd=6', () => {
    const rows: HourlyRow[] = [
      { date_hour: '2024-01-01T09:00:00', total_requests: 10 }, // 周一 9 时
      { date_hour: '2024-01-07T14:00:00', total_requests: 5 }, // 周日 14 时
    ]
    const { data, max } = bucketHourlyToHeatmap(rows)
    expect(cell(data, 9, 0)).toBe(10)
    expect(cell(data, 14, 6)).toBe(5)
    expect(max).toBe(10)
  })

  it('同一 (weekday,hour) 跨周累加', () => {
    const rows: HourlyRow[] = [
      { date_hour: '2024-01-01T09:00:00', total_requests: 10 }, // 周一 9 时
      { date_hour: '2024-01-08T09:00:00', total_requests: 7 }, // 下周一 9 时
    ]
    const { data, max } = bucketHourlyToHeatmap(rows)
    expect(cell(data, 9, 0)).toBe(17)
    expect(max).toBe(17)
  })

  it('非法/空 date_hour 跳过，不污染网格', () => {
    const rows: HourlyRow[] = [
      { date_hour: '', total_requests: 99 },
      { date_hour: 'bad', total_requests: 99 },
      { date_hour: '2024-01-01T09:00:00', total_requests: 3 },
    ]
    const { data, max } = bucketHourlyToHeatmap(rows)
    expect(cell(data, 9, 0)).toBe(3)
    expect(max).toBe(3)
  })
})
