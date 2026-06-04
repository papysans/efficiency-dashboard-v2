import type { CSSProperties, ReactNode } from 'react'

interface MetricCardProps {
  label: string
  value: ReactNode
  hint?: string
  /** 口径说明，悬浮在 info 图标上 */
  tip?: string
  tone?: 'pos' | 'neg' | 'neutral'
  /** 左侧强调色条（CSS 颜色值，如 '#0071e3'）。Need 详情 6 张卡用。 */
  accent?: string
}

const VALUE_TONE: Record<string, string> = {
  pos: 'text-emerald-600 dark:text-emerald-400',
  neg: 'text-rose-600 dark:text-rose-400',
  neutral: 'text-gray-900 dark:text-white',
}

/** 玻璃指标卡：标签 + 大数值 + 可选 hint/口径 tip / 左侧 accent 色条 */
export function MetricCard({ label, value, hint, tip, tone = 'neutral', accent }: MetricCardProps) {
  const style: CSSProperties | undefined = accent ? { borderLeft: `3px solid ${accent}` } : undefined
  return (
    <div className="glass rounded-2xl p-4 hover:scale-[1.02] transition-transform" style={style}>
      <div className="flex items-center gap-1 mb-1">
        <span className="text-sm text-gray-500 dark:text-gray-400">{label}</span>
        {tip && (
          <span className="text-gray-400 cursor-help inline-flex" title={tip} aria-label={tip}>
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
          </span>
        )}
      </div>
      <div className={`text-2xl font-bold tabular-nums ${VALUE_TONE[tone]}`}>{value}</div>
      {hint && <div className="text-xs text-gray-400 dark:text-gray-500 mt-1">{hint}</div>}
    </div>
  )
}
