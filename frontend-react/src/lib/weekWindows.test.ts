import { describe, it, expect } from 'vitest'
import { sliceWeekWindows, weekWindowLabel } from './weekWindows'

describe('sliceWeekWindows — 周窗口切分（周一为界，端点裁剪）', () => {
  it('非法/空区间返回空数组', () => {
    expect(sliceWeekWindows('', '')).toEqual([])
    expect(sliceWeekWindows('2026-06-10', '2026-06-01')).toEqual([]) // start > end
    expect(sliceWeekWindows('not-a-date', '2026-06-10')).toEqual([])
  })

  it('单日区间 → 1 个窗口，起止被裁剪到该日', () => {
    const ws = sliceWeekWindows('2026-06-17', '2026-06-17') // 周三
    expect(ws).toHaveLength(1)
    expect(ws[0].startDate).toBe('2026-06-17')
    expect(ws[0].endDate).toBe('2026-06-17')
    // 该周周一 = 2026-06-15
    expect(weekWindowLabel(ws[0].monday)).toBe('06/15')
  })

  it('跨多周区间 → 首尾窗口被 range 端点裁剪，中间窗口为整周（周一→周日）', () => {
    // 2026-06-10(周三) ~ 2026-06-24(周三)：跨 W24/W25/W26 三周
    const ws = sliceWeekWindows('2026-06-10', '2026-06-24')
    expect(ws.length).toBe(3)
    // 首窗：起点裁剪到 06-10，结束到该周周日 06-14
    expect(ws[0].startDate).toBe('2026-06-10')
    expect(ws[0].endDate).toBe('2026-06-14')
    // 中窗：整周 06-15(周一) ~ 06-21(周日)
    expect(ws[1].startDate).toBe('2026-06-15')
    expect(ws[1].endDate).toBe('2026-06-21')
    // 尾窗：从周一 06-22 起，结束裁剪到 06-24
    expect(ws[2].startDate).toBe('2026-06-22')
    expect(ws[2].endDate).toBe('2026-06-24')
  })

  it('窗口按周一时间升序', () => {
    const ws = sliceWeekWindows('2026-05-01', '2026-06-24')
    for (let i = 1; i < ws.length; i += 1) {
      expect(ws[i].monday.getTime()).toBeGreaterThan(ws[i - 1].monday.getTime())
    }
  })

  it('区间过大用 maxWindows 兜底（取最近 N 周）', () => {
    const ws = sliceWeekWindows('2025-01-01', '2026-06-24', 8)
    expect(ws.length).toBe(8)
    // 末窗仍裁剪到 end
    expect(ws[ws.length - 1].endDate).toBe('2026-06-24')
  })

  it('key 形如 YYYY-Wxx', () => {
    const ws = sliceWeekWindows('2026-06-17', '2026-06-17')
    expect(ws[0].key).toMatch(/^\d{4}-W\d{2}$/)
  })
})
