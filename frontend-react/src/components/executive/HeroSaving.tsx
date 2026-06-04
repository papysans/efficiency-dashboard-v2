import { useMemo } from 'react'
import { useDashboardSummary } from '@/api/queries'
import { useCountUp } from '@/hooks/useCountUp'
import { useConfigStore } from '@/stores/config'
import { formatNumber, personDaysValue, PERSON_DAY_MINUTES } from '@/lib/formatters'

interface HeroSavingProps {
  startDate: string
  endDate: string
}

/**
 * Hero：省人天 & ROI + 综合提效。数据 useDashboardSummary（design-pr1 §1①）。
 * - savedMin = max(0, baseline − actual)；省人天 = savedMin/480；省成本 = personDays × 单价
 * - 综合日历提效 = need_calendar_ratio（小数口径×100）
 * - 三大数字用 useCountUp 滚动（reduce-motion 时直接显终值）
 */
export function HeroSaving({ startDate, endDate }: HeroSavingProps) {
  const { data, isLoading, error } = useDashboardSummary({ startDate, endDate })
  const costPerPersonDay = useConfigStore((s) => s.costPerPersonDay)

  const savedMin = Math.max(0, (data?.need_baseline_calendar_min || 0) - (data?.need_actual_calendar_min || 0))
  const savedDays = savedMin / PERSON_DAY_MINUTES
  const savedCost = personDaysValue(savedMin) * costPerPersonDay
  // 综合日历提效（小数口径）→ 百分比数值用于滚动；null 时按 0 处理但展示加保护
  const ratio = data?.need_calendar_ratio
  const ratioPct = ratio == null ? 0 : ratio * 100
  const ratioAvailable = ratio != null && Number.isFinite(Number(ratio))

  const daysCount = useCountUp(savedDays)
  const costCount = useCountUp(Math.round(savedCost))
  const ratioCount = useCountUp(ratioPct)

  const period = useMemo(() => fmtPeriod(startDate, endDate), [startDate, endDate])

  if (error) {
    return (
      <div className="glass rounded-2xl p-6 text-rose-600 dark:text-rose-400">
        加载失败：{(error as Error).message}
      </div>
    )
  }

  if (isLoading || !data) {
    return (
      <div className="glass rounded-2xl p-6 md:p-8 min-h-[15rem]">
        <div className="skeleton w-48 h-7 mb-2" />
        <div className="skeleton w-64 h-4 mb-8" />
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-8">
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="skeleton h-20" />
          ))}
        </div>
      </div>
    )
  }

  const ratioTone = ratioPct < 0 ? 'text-rose-600 dark:text-rose-400' : 'text-emerald-600 dark:text-emerald-400'

  return (
    <div className="glass rounded-2xl p-6 md:p-8 min-h-[15rem] hover:shadow-lg transition-shadow flex flex-col">
      <div className="flex flex-wrap items-start justify-between gap-3 mb-6 md:mb-8">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white mb-1">AI 提效总览</h1>
          <p className="text-sm text-gray-500 dark:text-gray-400">
            按 ¥{formatNumber(costPerPersonDay)}/人天估算 · 基于可计入需求（merged &amp; eligible）
          </p>
        </div>
        <span className="text-xs px-3 py-1 rounded-full bg-white/50 dark:bg-white/10 text-gray-500 dark:text-gray-400 whitespace-nowrap">
          {period}
        </span>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-3 gap-8 flex-1">
        <BigStat
          label="为团队节省"
          value={savedDays > 0 ? daysCount.toFixed(1) : '-'}
          unit="人天"
          tone="text-emerald-600 dark:text-emerald-400"
        />
        <BigStat
          label="折合节省成本"
          value={savedCost > 0 ? `¥${formatNumber(Math.round(costCount))}` : '-'}
          unit=""
          tone="text-emerald-600 dark:text-emerald-400"
        />
        <BigStat
          label="综合日历提效"
          value={ratioAvailable ? `${ratioCount.toFixed(1)}%` : '-'}
          unit=""
          tone={ratioTone}
        />
      </div>
    </div>
  )
}

function BigStat({ label, value, unit, tone }: { label: string; value: string; unit: string; tone: string }) {
  return (
    <div>
      <div className="text-sm font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide mb-2">{label}</div>
      <div className="flex items-baseline gap-1.5">
        <span className={`text-5xl md:text-6xl font-black tabular-nums leading-none ${tone}`}>{value}</span>
        {unit && <span className="text-lg text-gray-400">{unit}</span>}
      </div>
    </div>
  )
}

/** 'YYYYMMDD'×2 → '2026/03/06 ~ 2026/06/04' */
function fmtPeriod(start: string, end: string): string {
  const f = (s: string) => (s.length === 8 ? `${s.slice(0, 4)}/${s.slice(4, 6)}/${s.slice(6, 8)}` : s)
  return `${f(start)} ~ ${f(end)}`
}
