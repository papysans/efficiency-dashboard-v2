import { formatPercent } from '@/lib/formatters'

// 百分比口径提效比胶囊（输入已是百分比值：300 => '300.0%'，**不 ×100**）。
// 用于 Task/Commit/Repo/Project 等古法 ancient/real 维度。与 Need 的 RatioPill（小数口径×100）相反，
// 绝不混用：把 300 喂给 RatioPill 会显示成 30000%。见 research/pr2-task-pages.md「头号差异」。
//
// 着色阈值**直接比原值**（不放大）：null/非有限 => info（对齐 Vue el-tag(info) -）；
// >=300 => pos(绿) / >=150 => info(蓝) / 其余 => neutral。
type Tone = 'pos' | 'info' | 'neutral'

function toneOf(value: number | string | null | undefined): Tone {
  const num = Number(value)
  if (value == null || value === '' || !Number.isFinite(num)) return 'info'
  if (num >= 300) return 'pos'
  if (num >= 150) return 'info'
  return 'neutral'
}

const TONE_CLASS: Record<Tone, string> = {
  pos: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/50 dark:text-emerald-300',
  info: 'bg-sky-100 text-sky-700 dark:bg-sky-900/50 dark:text-sky-300',
  neutral: 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300',
}

interface PercentPillProps {
  value: number | string | null | undefined
  digits?: number
}

export function PercentPill({ value, digits = 1 }: PercentPillProps) {
  return (
    <span className={`inline-block text-xs px-2 py-0.5 rounded-full font-medium tabular-nums ${TONE_CLASS[toneOf(value)]}`}>
      {formatPercent(value, digits)}
    </span>
  )
}

/**
 * 百分比口径文字色（详情大数字用，对齐 Vue efficiencyColor）：
 * null => 灰 / >=300 => 绿 / >=150 => 蓝 / 其余 => 灰。
 */
export function percentTextClass(value: number | null | undefined): string {
  if (value == null || !Number.isFinite(Number(value))) return 'text-gray-400 dark:text-gray-500'
  const num = Number(value)
  if (num >= 300) return 'text-emerald-600 dark:text-emerald-400'
  if (num >= 150) return 'text-sky-600 dark:text-sky-400'
  return 'text-gray-400 dark:text-gray-500'
}
