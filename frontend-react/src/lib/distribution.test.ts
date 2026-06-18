import { describe, it, expect } from 'vitest'
import {
  computeDistribution,
  computeQuantiles,
  computeExclusionReasons,
  computeLocBands,
  computeRatioHistogram,
  MIN_BINS,
  MAX_BINS,
  type DistInput,
} from './distribution'

/** 构造一行，默认是「计入前置满足」的干净行；按需覆盖。 */
function row(over: Partial<DistInput> = {}): DistInput {
  return {
    efficiency_ratio: 1,
    work_efficiency_ratio: 1,
    calendar_outlier_flag: false,
    work_outlier_flag: false,
    outlier_flag: false,
    coverage_eligible: true,
    reason: '',
    total_loc_net: null,
    total_calendar_min: null,
    ...over,
  }
}

describe('computeDistribution — kept/excluded 口径（对齐 backend db.go:1525）', () => {
  it('入桶前置 = coverage_eligible 且 ratio 有限', () => {
    const rows = [
      row({ coverage_eligible: false }), // 非 eligible，排除
      row({ efficiency_ratio: null }), // ratio null，排除
      row({ efficiency_ratio: Infinity }), // 非有限，排除
      row({ efficiency_ratio: 1.2 }), // 计入
    ]
    const d = computeDistribution(rows, 'calendar', 6)
    expect(d.keptCount).toBe(1)
    expect(d.excludedCount).toBe(0)
  })

  it('kept = NOT outlier；excluded = outlier（日历用 calendar_outlier_flag）', () => {
    const rows = [
      row({ efficiency_ratio: 1, calendar_outlier_flag: false }),
      row({ efficiency_ratio: 8, calendar_outlier_flag: true }), // 隔离
    ]
    const d = computeDistribution(rows, 'calendar', 6)
    expect(d.keptCount).toBe(1)
    expect(d.excludedCount).toBe(1)
  })

  it('切到 work 口径用 work_efficiency_ratio + work_outlier_flag', () => {
    const rows = [
      // 日历正常但工作量异常：日历口径 kept，工作量口径 excluded
      row({
        efficiency_ratio: 1,
        calendar_outlier_flag: false,
        work_efficiency_ratio: 1,
        work_outlier_flag: true,
      }),
    ]
    expect(computeDistribution(rows, 'calendar', 6).keptCount).toBe(1)
    const work = computeDistribution(rows, 'work', 6)
    expect(work.keptCount).toBe(0)
    expect(work.excludedCount).toBe(1)
  })

  it('口径 flag 缺失时回退派生 outlier_flag', () => {
    const rows = [row({ efficiency_ratio: 1, calendar_outlier_flag: undefined, outlier_flag: true })]
    const d = computeDistribution(rows, 'calendar', 6)
    expect(d.excludedCount).toBe(1)
    expect(d.keptCount).toBe(0)
  })

  it('特殊桶：负提效落首桶，>=600% 落尾桶，主区间等宽', () => {
    const rows = [
      row({ efficiency_ratio: -0.5 }), // 负提效 → bucket 0
      row({ efficiency_ratio: 0.5 }), // 主区间
      row({ efficiency_ratio: 7 }), // >600% → 尾桶
    ]
    const d = computeDistribution(rows, 'calendar', 6)
    const first = d.histogram[0]
    const last = d.histogram[d.histogram.length - 1]
    expect(first.label).toBe('负提效')
    expect(first.kept).toBe(1)
    expect(last.kept).toBe(1)
    // 主区间桶数 = binCount；含首尾共 binCount + 2
    expect(d.histogram.length).toBe(6 + 2)
  })

  it('binCount 被钳到 [MIN_BINS, MAX_BINS] 并四舍五入', () => {
    expect(computeDistribution([], 'calendar', 1).binCount).toBe(MIN_BINS)
    expect(computeDistribution([], 'calendar', 999).binCount).toBe(MAX_BINS)
    expect(computeDistribution([], 'calendar', 11.6).binCount).toBe(12)
  })
})

describe('computeRatioHistogram — 各对象提效比分桶（复用等宽分桶，纵轴=对象个数）', () => {
  it('小数口径：负提效首桶 / 主区间 / >=600% 尾桶；total = 有限值个数', () => {
    const r = computeRatioHistogram([-0.5, 0.5, 7, null, undefined, NaN], 6, 'decimal')
    expect(r.total).toBe(3) // null/undefined/NaN 跳过
    expect(r.histogram[0].label).toBe('负提效')
    expect(r.histogram[0].count).toBe(1) // -0.5
    expect(r.histogram[r.histogram.length - 1].count).toBe(1) // 7 → 尾桶
    // 主区间桶数 = bins，含首尾共 bins + 2
    expect(r.histogram.length).toBe(6 + 2)
  })

  it('百分比口径 /100 后与小数口径落同一桶（口径不混）', () => {
    const dec = computeRatioHistogram([2.5], 6, 'decimal') // 250%
    const pct = computeRatioHistogram([250], 6, 'percent') // 250% → /100 = 2.5
    const decIdx = dec.histogram.findIndex((b) => b.count === 1)
    const pctIdx = pct.histogram.findIndex((b) => b.count === 1)
    expect(decIdx).toBe(pctIdx)
    expect(decIdx).toBeGreaterThan(0)
  })

  it('binCount 钳到 [MIN_BINS, MAX_BINS]；空输入 total=0', () => {
    expect(computeRatioHistogram([], 1, 'decimal').binCount).toBe(MIN_BINS)
    expect(computeRatioHistogram([], 999, 'decimal').binCount).toBe(MAX_BINS)
    expect(computeRatioHistogram([], 6, 'decimal').total).toBe(0)
  })
})

describe('computeQuantiles — 仅 kept 集，线性插值（等价 PG PERCENTILE_CONT）', () => {
  it('四个有限值的 P25/中位/P75', () => {
    const rows = [1, 2, 3, 4].map((v) => row({ efficiency_ratio: v }))
    const q = computeQuantiles(rows, 'calendar')
    // 升序 [1,2,3,4]，PERCENTILE_CONT: p25=1.75 median=2.5 p75=3.25
    expect(q.count).toBe(4)
    expect(q.p25).toBeCloseTo(1.75, 10)
    expect(q.median).toBeCloseTo(2.5, 10)
    expect(q.p75).toBeCloseTo(3.25, 10)
  })

  it('outlier 与非 eligible 不计入分位数样本', () => {
    const rows = [
      row({ efficiency_ratio: 1 }),
      row({ efficiency_ratio: 99, calendar_outlier_flag: true }),
      row({ efficiency_ratio: 99, coverage_eligible: false }),
    ]
    const q = computeQuantiles(rows, 'calendar')
    expect(q.count).toBe(1)
    expect(q.median).toBe(1)
  })

  it('空集 → 全 null', () => {
    const q = computeQuantiles([], 'calendar')
    expect(q).toEqual({ p25: null, median: null, p75: null, count: 0 })
  })
})

describe('computeExclusionReasons — reason 子串计数，可重叠（对齐 reason_loc/eff/atb）', () => {
  it('一行命中两个子串则两类各 +1（与后端独立 FILTER 一致）', () => {
    const rows = [
      row({ outlier_flag: true, reason: 'impossible_loc_rate; efficiency_ratio' }),
      row({ outlier_flag: true, reason: 'actual_to_baseline' }),
      row({ outlier_flag: false, reason: 'efficiency_ratio' }), // 非 outlier，不计
      row({ outlier_flag: true, coverage_eligible: false, reason: 'efficiency_ratio' }), // 非 eligible，不计
    ]
    const byKey = Object.fromEntries(computeExclusionReasons(rows).map((r) => [r.reason, r.count]))
    expect(byKey['impossible_loc_rate']).toBe(1)
    expect(byKey['efficiency_ratio']).toBe(1)
    expect(byKey['actual_to_baseline']).toBe(1)
  })
})

describe('computeLocBands — 行/分钟分档（对齐后端 lb1..lb4 边界）', () => {
  it('边界 7/21/50 归属（<=7 / >7&<=21 / >21&<=50 / >50）', () => {
    const rows = [
      row({ total_loc_net: 7, total_calendar_min: 1 }), // 7.0 → lb1
      row({ total_loc_net: 21, total_calendar_min: 1 }), // 21.0 → lb2
      row({ total_loc_net: 50, total_calendar_min: 1 }), // 50.0 → lb3
      row({ total_loc_net: 51, total_calendar_min: 1 }), // 51.0 → lb4
    ]
    const counts = computeLocBands(rows).map((b) => b.count)
    expect(counts).toEqual([1, 1, 1, 1])
  })

  it('total_loc_net 为 null 或 calendar<=0 不计入任何档（对齐 db.go:1552 NULL 语义）', () => {
    const rows = [
      row({ total_loc_net: null, total_calendar_min: 10 }),
      row({ total_loc_net: 100, total_calendar_min: 0 }),
      row({ total_loc_net: 100, total_calendar_min: null }),
      row({ total_loc_net: 100, total_calendar_min: 10, coverage_eligible: false }),
    ]
    expect(computeLocBands(rows).reduce((s, b) => s + b.count, 0)).toBe(0)
  })
})
