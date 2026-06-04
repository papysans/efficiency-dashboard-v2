import { formatV2Ratio } from '@/lib/formatters'

// 提效比胶囊（小数口径输入：0.25 => 25%）。分档阈值与 Vue RatioPill 一致，
// 见 research/api-contract.md §4：基于 value*100 判定 <0 红 / >=300 绿 / >=150 蓝 / 其余中性。
type Tone = 'pos' | 'neg' | 'info' | 'neutral'

function toneOf(value: number | string | null | undefined): Tone {
  const num = Number(value)
  if (value == null || value === '' || !Number.isFinite(num)) return 'neutral'
  const pct = num * 100
  if (pct < 0) return 'neg'
  if (pct >= 300) return 'pos'
  if (pct >= 150) return 'info'
  return 'neutral'
}

const TONE_CLASS: Record<Tone, string> = {
  pos: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/50 dark:text-emerald-300',
  neg: 'bg-rose-100 text-rose-700 dark:bg-rose-900/50 dark:text-rose-300',
  info: 'bg-sky-100 text-sky-700 dark:bg-sky-900/50 dark:text-sky-300',
  neutral: 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300',
}

interface RatioPillProps {
  value: number | string | null | undefined
  digits?: number
}

export function RatioPill({ value, digits = 1 }: RatioPillProps) {
  return (
    <span className={`inline-block text-xs px-2 py-0.5 rounded-full font-medium tabular-nums ${TONE_CLASS[toneOf(value)]}`}>
      {formatV2Ratio(value, digits)}
    </span>
  )
}
