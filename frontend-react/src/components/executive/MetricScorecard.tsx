import type { ReactNode } from 'react'
import { formatV2Ratio } from '@/lib/formatters'
import type { DashboardTrendDelta } from '@/api/types'

// 首页 4 维记分卡：维度名 + ⓘ名词解释 + 大数值 + 副信息 + 环比箭头 + 周趋势 sparkline。
// 可点下钻（onClick → 该维度最相关现有页）。质量维度本轮无数据，用 placeholder 占位（见 QualityPlaceholder）。

interface MetricScorecardProps {
  /** 维度名（使用/效率/成本/贡献） */
  label: string
  /** 当期值（已格式化），无数据传 null */
  value: ReactNode
  /** 副信息（如 "63% AI" / "ROI 4.2x"） */
  hint?: string
  /** ⓘ 名词解释（取自 glossaryTip） */
  tip: string
  /** 周序列（升序），用于 sparkline；空数组则不画 */
  series: number[]
  /** 环比（本期vs上期）；null/无 delta_pct 时不显示箭头 */
  delta?: DashboardTrendDelta | null
  /** 该维度是否"越高越好"（成本=false）。决定环比箭头配色。默认 true */
  higherIsBetter?: boolean
  /** sparkline / accent 颜色（CSS 颜色值） */
  accent?: string
  /** 下钻点击 */
  onClick?: () => void
  /** 加载态 */
  loading?: boolean
}

export function MetricScorecard({
  label,
  value,
  hint,
  tip,
  series,
  delta,
  higherIsBetter = true,
  accent = '#0071e3',
  onClick,
  loading = false,
}: MetricScorecardProps) {
  const clickable = !!onClick
  return (
    <div
      className={`glass rounded-2xl p-4 flex flex-col gap-2 transition-shadow ${
        clickable ? 'cursor-pointer hover:shadow-lg focus:outline-none focus-visible:ring-2 focus-visible:ring-sky-400' : ''
      }`}
      style={{ borderLeft: `3px solid ${accent}` }}
      onClick={onClick}
      role={clickable ? 'button' : undefined}
      tabIndex={clickable ? 0 : undefined}
      onKeyDown={clickable ? (e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), onClick?.()) : undefined}
      aria-label={clickable ? `${label}，点击下钻` : undefined}
    >
      <div className="flex items-center gap-1">
        <span className="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide">{label}</span>
        <span className="text-gray-400 cursor-help inline-flex" title={tip} aria-label={tip}>
          <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        </span>
      </div>

      {loading ? (
        <div className="skeleton h-7 w-20 rounded" />
      ) : (
        <div className="flex items-baseline gap-2 flex-wrap">
          <span className="text-2xl font-bold tabular-nums text-gray-900 dark:text-white">{value ?? '-'}</span>
          <DeltaArrow delta={delta} higherIsBetter={higherIsBetter} />
        </div>
      )}

      {hint && <div className="text-xs text-gray-400 dark:text-gray-500">{hint}</div>}

      <div className="mt-auto pt-1">
        <Sparkline data={series} color={accent} />
      </div>
    </div>
  )
}

/** 环比箭头：▲/▼ + |变化率|%。delta 缺失或 delta_pct=null 时渲染空（不占位）。 */
function DeltaArrow({ delta, higherIsBetter }: { delta?: DashboardTrendDelta | null; higherIsBetter: boolean }) {
  if (!delta || delta.delta_pct == null) return null
  const pct = delta.delta_pct
  const up = pct >= 0
  const good = higherIsBetter ? up : !up
  const color = good ? 'text-emerald-600 dark:text-emerald-400' : 'text-rose-600 dark:text-rose-400'
  return (
    <span className={`text-xs font-medium tabular-nums ${color}`} title="环比：本期 vs 等长上期">
      {up ? '▲' : '▼'} {formatV2Ratio(Math.abs(pct), 0)}
    </span>
  )
}

/** 极简内联 SVG sparkline（无依赖）。归一化到 viewBox，单点/空数据安全降级。 */
function Sparkline({ data, color }: { data: number[]; color: string }) {
  const w = 100
  const h = 28
  const pts = data.filter((d) => Number.isFinite(d))
  if (pts.length < 2) {
    return <div className="h-7" aria-hidden="true" />
  }
  const min = Math.min(...pts)
  const max = Math.max(...pts)
  const span = max - min || 1
  const step = w / (pts.length - 1)
  const coords = pts.map((d, i) => {
    const x = i * step
    const y = h - ((d - min) / span) * (h - 4) - 2 // 上下各留 2px
    return [x, y] as const
  })
  const line = coords.map(([x, y], i) => `${i === 0 ? 'M' : 'L'}${x.toFixed(1)},${y.toFixed(1)}`).join(' ')
  const area = `${line} L${w},${h} L0,${h} Z`
  const last = coords[coords.length - 1]
  return (
    <svg viewBox={`0 0 ${w} ${h}`} className="w-full h-7" preserveAspectRatio="none" role="img" aria-label="周趋势">
      <path d={area} fill={color} fillOpacity={0.1} stroke="none" />
      <path d={line} fill="none" stroke={color} strokeWidth={1.5} vectorEffect="non-scaling-stroke" />
      <circle cx={last[0]} cy={last[1]} r={2} fill={color} />
    </svg>
  )
}

/** 质量维度占位卡（本轮无可靠数据，标"数据建设中"，与 4 维卡同形）。 */
export function QualityPlaceholder({ tip }: { tip: string }) {
  return (
    <div className="glass rounded-2xl p-4 flex flex-col gap-2 opacity-70" style={{ borderLeft: '3px solid #9ca3af' }}>
      <div className="flex items-center gap-1">
        <span className="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide">质量</span>
        <span className="text-gray-400 cursor-help inline-flex" title={tip} aria-label={tip}>
          <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        </span>
      </div>
      <div className="text-sm text-gray-400 dark:text-gray-500 mt-1">数据建设中</div>
      <div className="text-xs text-gray-400 dark:text-gray-500 mt-auto">质量信号采集完善后开放</div>
    </div>
  )
}
