// 提效分布前端分桶/统计纯函数（提效分布页的计算地基，无 React 依赖、可独立测试）。
//
// 计数 / 分位数 / 诊断 / LOC 速率档 严格对齐后端 backend/db.go:1525 queryNeedsDistributionAgg：
//   入桶前置 = coverage_eligible 且 ratio 有限
//   kept     = 前置 且 NOT <caliber>_outlier_flag
//   excluded = 前置 且 <caliber>_outlier_flag
//   分位数仅对 kept 集（PERCENTILE_CONT 线性插值，与 PG 一致）
//   诊断(剔除原因/LOC 速率)按全局 outlier_flag，不随口径切换（与后端 reason_*/lb* 一致）
//
// 直方图档位为「前端可调等宽 bin」(手调粒度，非后端语义 6 档) —— 主区间固定 [0,600%]，
//   负提效(<0)、>600% 各单列特殊桶；后端的语义 6 档仅作概览，本页不依赖。

export type Caliber = 'calendar' | 'work'

/** 分桶所需的最小行结构（NeedsV2Summary 结构兼容，解耦便于测试）。 */
export interface DistInput {
  efficiency_ratio: number | null
  work_efficiency_ratio: number | null
  calendar_outlier_flag?: boolean
  work_outlier_flag?: boolean
  outlier_flag: boolean
  coverage_eligible: boolean
  reason?: string
  total_loc_net?: number | null
  total_calendar_min?: number | null
}

export interface HistBucket {
  label: string
  lo: number // 下界(含)；负提效桶 = -Infinity
  hi: number // 上界(不含)；尾桶 = Infinity
  kept: number
  excluded: number
}

export interface Quantiles {
  p25: number | null
  median: number | null
  p75: number | null
  count: number // kept 样本数
}

export interface ReasonCount {
  reason: string
  label: string
  count: number
}

export interface LocBand {
  label: string
  lo: number
  hi: number
  count: number
}

export interface DistributionResult {
  caliber: Caliber
  binCount: number
  keptCount: number
  excludedCount: number
  histogram: HistBucket[]
  quantiles: Quantiles
}

// 主区间 [0, MAIN_HI]，按 binCount 等宽细分；负提效与 >MAIN_HI 各单列。
// 取 6（600%）使预设粒度的档宽为整：6→100% / 12→50% / 24→25%。
const MAIN_HI = 6

/** 手调粒度预设（主区间 bin 数）。 */
export const GRANULARITY_PRESETS = [
  { label: '粗', bins: 6 },
  { label: '中', bins: 12 },
  { label: '细', bins: 24 },
] as const

export const MIN_BINS = 4
export const MAX_BINS = 50

function pick(row: DistInput, caliber: Caliber): { ratio: number | null; outlier: boolean } {
  if (caliber === 'calendar') {
    return { ratio: row.efficiency_ratio, outlier: row.calendar_outlier_flag ?? row.outlier_flag }
  }
  return { ratio: row.work_efficiency_ratio, outlier: row.work_outlier_flag ?? row.outlier_flag }
}

/** 比值 → 百分比 label（整数优先，否则一位小数）。 */
function fmtPct(v: number): string {
  const p = v * 100
  return Number.isInteger(p) ? `${p}%` : `${p.toFixed(1)}%`
}

function clampBins(binCount: number): number {
  return Math.max(MIN_BINS, Math.min(MAX_BINS, Math.round(binCount)))
}

function emptyBuckets(binCount: number): HistBucket[] {
  const step = MAIN_HI / binCount
  const buckets: HistBucket[] = [{ label: '负提效', lo: -Infinity, hi: 0, kept: 0, excluded: 0 }]
  for (let i = 0; i < binCount; i += 1) {
    const lo = i * step
    const hi = (i + 1) * step
    buckets.push({ label: `${fmtPct(lo)}~${fmtPct(hi)}`, lo, hi, kept: 0, excluded: 0 })
  }
  buckets.push({ label: `>${fmtPct(MAIN_HI)}`, lo: MAIN_HI, hi: Infinity, kept: 0, excluded: 0 })
  return buckets
}

/** ratio 落到哪个桶下标：0=负提效，1..binCount=主区间，binCount+1=尾桶。 */
function bucketIndex(ratio: number, binCount: number): number {
  if (ratio < 0) return 0
  if (ratio >= MAIN_HI) return binCount + 1
  const step = MAIN_HI / binCount
  return 1 + Math.floor(ratio / step)
}

/** 升序数组的分位数（线性插值，等价 PG PERCENTILE_CONT）。 */
function quantile(sorted: number[], q: number): number | null {
  const n = sorted.length
  if (n === 0) return null
  if (n === 1) return sorted[0]
  const pos = (n - 1) * q
  const base = Math.floor(pos)
  const rest = pos - base
  const lo = sorted[base]
  const hi = sorted[base + 1]
  return hi === undefined ? lo : lo + rest * (hi - lo)
}

/** 当前口径下 kept 集的分位数（健康横幅双口径中位用）。 */
export function computeQuantiles(rows: DistInput[], caliber: Caliber): Quantiles {
  const kept: number[] = []
  for (const row of rows) {
    if (!row.coverage_eligible) continue
    const { ratio, outlier } = pick(row, caliber)
    if (ratio == null || !Number.isFinite(ratio) || outlier) continue
    kept.push(ratio)
  }
  kept.sort((a, b) => a - b)
  return { p25: quantile(kept, 0.25), median: quantile(kept, 0.5), p75: quantile(kept, 0.75), count: kept.length }
}

/** 主入口：当前口径 + 粒度下的直方图（kept/excluded 堆叠）+ 计数 + 分位数。 */
export function computeDistribution(rows: DistInput[], caliber: Caliber, binCount: number): DistributionResult {
  const bins = clampBins(binCount)
  const histogram = emptyBuckets(bins)
  const keptRatios: number[] = []
  let keptCount = 0
  let excludedCount = 0
  for (const row of rows) {
    if (!row.coverage_eligible) continue
    const { ratio, outlier } = pick(row, caliber)
    if (ratio == null || !Number.isFinite(ratio)) continue
    const idx = bucketIndex(ratio, bins)
    if (outlier) {
      histogram[idx].excluded += 1
      excludedCount += 1
    } else {
      histogram[idx].kept += 1
      keptCount += 1
      keptRatios.push(ratio)
    }
  }
  keptRatios.sort((a, b) => a - b)
  return {
    caliber,
    binCount: bins,
    keptCount,
    excludedCount,
    histogram,
    quantiles: {
      p25: quantile(keptRatios, 0.25),
      median: quantile(keptRatios, 0.5),
      p75: quantile(keptRatios, 0.75),
      count: keptCount,
    },
  }
}

/** 各对象提效比直方图的单桶（counts=落该桶的对象个数）。 */
export interface RatioHistBucket {
  label: string
  lo: number
  hi: number
  count: number
}

export interface RatioHistogramResult {
  binCount: number
  total: number // 计入直方图的对象个数（有限 ratio）
  histogram: RatioHistBucket[]
}

/**
 * 「各对象提效比」直方图：纵轴 = 对象个数（项目/仓库/用户 数），不分 kept/excluded。
 * 复用 computeDistribution 的等宽分桶（emptyBuckets/bucketIndex/clampBins，共享 [0,600%] 主区间 +
 * 负提效首桶 + 尾桶 + fmtPct 标签），仅把「需求行」换成「对象的 ratio 标量」。
 *
 * scale: 'decimal' 输入是小数口径（0.25=25%，项目/个人用）；'percent' 输入已是百分比数值
 * （300=300%，仓库用）→ 先 /100 归一到小数再入桶，使两口径共享同一档边界。空/非有限值跳过。
 */
export function computeRatioHistogram(
  ratios: Array<number | null | undefined>,
  binCount: number,
  scale: 'decimal' | 'percent' = 'decimal',
): RatioHistogramResult {
  const bins = clampBins(binCount)
  const base = emptyBuckets(bins)
  const histogram: RatioHistBucket[] = base.map((b) => ({ label: b.label, lo: b.lo, hi: b.hi, count: 0 }))
  let total = 0
  for (const raw of ratios) {
    if (raw == null) continue
    const num = Number(raw)
    if (!Number.isFinite(num)) continue
    const ratio = scale === 'percent' ? num / 100 : num
    histogram[bucketIndex(ratio, bins)].count += 1
    total += 1
  }
  return { binCount: bins, total, histogram }
}

// 剔除原因（reason 文本含子串，与后端 reason_loc/reason_eff/reason_atb 同口径；原因可重叠计数）。
const REASON_DEFS: Array<{ key: string; label: string }> = [
  { key: 'impossible_loc_rate', label: '物理不可能(>1w行/日)' },
  { key: 'efficiency_ratio', label: '极端提效(>1000%)' },
  { key: 'actual_to_baseline', label: '工作量异常' },
]

export function computeExclusionReasons(rows: DistInput[]): ReasonCount[] {
  const counts = REASON_DEFS.map((d) => ({ reason: d.key, label: d.label, count: 0 }))
  for (const row of rows) {
    if (!row.coverage_eligible || !row.outlier_flag) continue
    const reason = row.reason ?? ''
    for (let i = 0; i < REASON_DEFS.length; i += 1) {
      if (reason.includes(REASON_DEFS[i].key)) counts[i].count += 1
    }
  }
  return counts
}

// LOC 速率分档（行/分钟，与后端 lb1..lb4 同口径：<=7 / >7&<=21 / >21&<=50 / >50）。
const LOC_BANDS: Array<{ label: string; lo: number; hi: number }> = [
  { label: '≤7 人力可达', lo: 0, hi: 7 },
  { label: '7-21', lo: 7, hi: 21 },
  { label: '21-50', lo: 21, hi: 50 },
  { label: '>50 bulk', lo: 50, hi: Infinity },
]

export function computeLocBands(rows: DistInput[]): LocBand[] {
  const out = LOC_BANDS.map((b) => ({ label: b.label, lo: b.lo, hi: b.hi, count: 0 }))
  for (const row of rows) {
    if (!row.coverage_eligible) continue
    // 与后端 db.go:1552 一致：total_loc_net 为 NULL 的行不计入任何档（不当 0 处理）。
    if (row.total_loc_net == null) continue
    const cal = row.total_calendar_min ?? 0
    const loc = row.total_loc_net
    if (cal <= 0) continue
    const rate = loc / cal
    if (rate <= 7) out[0].count += 1
    else if (rate <= 21) out[1].count += 1
    else if (rate <= 50) out[2].count += 1
    else out[3].count += 1
  }
  return out
}
