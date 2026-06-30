import { describe, it, expect } from 'vitest'
import {
  rangeDays,
  availableGranularities,
  defaultGranularity,
  buildBuckets,
} from './timeBucket'

describe('rangeDays — 含端点天数跨度', () => {
  it('单日=1，整周=7，逆序/非法=0', () => {
    expect(rangeDays('2026-06-17', '2026-06-17')).toBe(1)
    expect(rangeDays('2026-06-01', '2026-06-07')).toBe(7)
    expect(rangeDays('2026-06-10', '2026-06-01')).toBe(0)
    expect(rangeDays('x', '2026-06-01')).toBe(0)
  })
})

describe('粒度阈值（<14 天 / ≥14 周 / ≥60 月）', () => {
  it('available + default 与阈值一致', () => {
    expect(availableGranularities(7)).toEqual(['day'])
    expect(defaultGranularity(7)).toBe('day')
    expect(availableGranularities(14)).toEqual(['day', 'week'])
    expect(defaultGranularity(14)).toBe('week')
    expect(availableGranularities(59)).toEqual(['day', 'week'])
    expect(defaultGranularity(59)).toBe('week')
    expect(availableGranularities(60)).toEqual(['day', 'week', 'month'])
    expect(defaultGranularity(60)).toBe('month')
  })
})

describe('buildBuckets — 天粒度', () => {
  it('每日一桶，去重+升序，rangeText=当日，spanDays=1', () => {
    const b = buildBuckets(['2026-06-02', '2026-06-01', '2026-06-01'], 'day')
    expect(b.map((x) => x.key)).toEqual(['2026-06-01', '2026-06-02'])
    expect(b[0].label).toBe('06/01')
    expect(b[0].rangeText).toBe('2026-06-01')
    expect(b[0].spanDays).toBe(1)
  })
})

describe('buildBuckets — 周粒度（周日为首天，首尾按 clamp 裁剪）', () => {
  // 2026-06-07 是周日；其周 = 06-07(周日)~06-13(周六)。2026-06-14 又是下一个周日。
  it('按周日分桶，key=周日', () => {
    const b = buildBuckets(['2026-06-07', '2026-06-10', '2026-06-14'], 'week')
    expect(b.map((x) => x.key)).toEqual(['2026-06-07', '2026-06-14'])
    expect(b[0].dates).toEqual(['2026-06-07', '2026-06-10'])
    expect(b[1].dates).toEqual(['2026-06-14'])
  })

  it('label=周日 MM/DD；rangeText 整周 周日~周六', () => {
    const b = buildBuckets(['2026-06-10'], 'week')
    expect(b[0].key).toBe('2026-06-07')
    expect(b[0].label).toBe('06/07')
    expect(b[0].rangeText).toBe('2026-06-07 ~ 2026-06-13')
    expect(b[0].spanDays).toBe(7)
  })

  it('clamp 裁剪首尾桶 rangeText 与 spanDays', () => {
    // 数据落在 06-10，但区间从 06-09 开始、06-11 结束 → 该周桶被裁到 06-09~06-11
    const b = buildBuckets(['2026-06-10'], 'week', { start: '2026-06-09', end: '2026-06-11' })
    expect(b[0].rangeText).toBe('2026-06-09 ~ 2026-06-11')
    expect(b[0].spanDays).toBe(3)
  })
})

describe('buildBuckets — 月粒度', () => {
  it('按 YYYY-MM 分桶，label=M月，rangeText=月首~月末', () => {
    const b = buildBuckets(['2026-06-30', '2026-07-01', '2026-06-15'], 'month')
    expect(b.map((x) => x.key)).toEqual(['2026-06', '2026-07'])
    expect(b[0].label).toBe('6月')
    expect(b[0].rangeText).toBe('2026-06-01 ~ 2026-06-30')
    expect(b[1].rangeText).toBe('2026-07-01 ~ 2026-07-31')
  })

  it('clamp 裁剪月桶边界', () => {
    const b = buildBuckets(['2026-06-15'], 'month', { start: '2026-06-10', end: '2026-06-20' })
    expect(b[0].rangeText).toBe('2026-06-10 ~ 2026-06-20')
    expect(b[0].spanDays).toBe(11)
  })
})

describe('buildBuckets — 边界', () => {
  it('空/全非法返回空数组', () => {
    expect(buildBuckets([], 'week')).toEqual([])
    expect(buildBuckets(['nope', ''], 'month')).toEqual([])
  })
})
