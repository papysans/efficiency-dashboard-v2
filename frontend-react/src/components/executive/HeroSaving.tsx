import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useDashboardSummary, useGlobalConfig } from '@/api/queries'
import { chatGet } from '@/api/client'
import { useCountUp } from '@/hooks/useCountUp'
import { formatNumber, personDaysValue, PERSON_DAY_MINUTES } from '@/lib/formatters'

interface HeroSavingProps {
  startDate: string
  endDate: string
}

/** /stats/global/daily 行（仅取本卡用到的成本字段）。 */
interface ChatDailyCostRow {
  estimated_total_cost: number | null
}

/** 人天单价兜底（后端 /v2/config cost_per_person_day 缺省时） */
const FALLBACK_COST_PER_PERSON_DAY = 2000

/**
 * Hero：省人天 & 净节省 + 综合提效。数据 useDashboardSummary（design-pr1 §1①）。
 * - savedMin = max(0, baseline − actual)；省人天 = savedMin/480；毛节省 = personDays × 单价（/v2/config 下发，缺省 2000）
 * - chat_stats 启用且同区间日汇总拉取成功时升级为净节省口径（T8）：
 *   省人天 / AI 花费（全平台口径，按价格表估算）/ 净节省（毛−AI，负数标红）/ 综合提效（毛节省只参与计算不展示）
 * - chat 未启用 / 加载中 / 失败：降级回三格（省人天 / 折合节省成本 / 综合提效），chat 请求绝不拖垮主数据
 * - 大数字用 useCountUp 滚动（reduce-motion 时直接显终值）
 */
export function HeroSaving({ startDate, endDate }: HeroSavingProps) {
  const { data, isLoading, error } = useDashboardSummary({ startDate, endDate })
  const { data: gc } = useGlobalConfig()
  const costPerPersonDay =
    gc?.cost_per_person_day && gc.cost_per_person_day > 0 ? gc.cost_per_person_day : FALLBACK_COST_PER_PERSON_DAY
  const chatEnabled = gc?.chat_stats_enabled === true

  // AI 花费：chat 侧同日期区间 estimated_total_cost 求和（局部 useQuery，失败静默降级）
  const chatQ = useQuery({
    queryKey: ['hero-chat-daily-cost', startDate, endDate],
    queryFn: () =>
      chatGet<ChatDailyCostRow[]>('/stats/global/daily', {
        start_date: ymd8ToDash(startDate),
        end_date: ymd8ToDash(endDate),
      }),
    enabled: chatEnabled,
    retry: 1,
    staleTime: 5 * 60_000,
  })
  const aiCost = useMemo(
    () => (chatQ.data ?? []).reduce((s, r) => s + (r.estimated_total_cost || 0), 0),
    [chatQ.data],
  )
  const aiAvailable = chatEnabled && chatQ.isSuccess

  const savedMin = Math.max(0, (data?.need_baseline_calendar_min || 0) - (data?.need_actual_calendar_min || 0))
  const savedDays = savedMin / PERSON_DAY_MINUTES
  const grossSaving = personDaysValue(savedMin) * costPerPersonDay
  const netSaving = grossSaving - aiCost
  // 综合日历提效（小数口径）→ 百分比数值用于滚动；null 时按 0 处理但展示加保护
  const ratio = data?.need_calendar_ratio
  const ratioPct = ratio == null ? 0 : ratio * 100
  const ratioAvailable = ratio != null && Number.isFinite(Number(ratio))

  const daysCount = useCountUp(savedDays)
  const grossCount = useCountUp(Math.round(grossSaving))
  const aiCount = useCountUp(Math.round(aiCost))
  const netCount = useCountUp(Math.round(netSaving))
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

  const emerald = 'text-emerald-600 dark:text-emerald-400'
  const rose = 'text-rose-600 dark:text-rose-400'
  const neutral = 'text-gray-900 dark:text-white'
  const ratioTone = ratioPct < 0 ? rose : emerald

  return (
    <div className="glass rounded-2xl p-6 md:p-8 min-h-[15rem] hover:shadow-lg transition-shadow flex flex-col">
      <div className="flex flex-wrap items-start justify-between gap-3 mb-6 md:mb-8">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white mb-1">AI 提效总览</h1>
          <p className="text-sm text-gray-500 dark:text-gray-400">
            按 ¥{formatNumber(costPerPersonDay)}/人天估算 · 基于可计入需求（merged &amp; eligible）
            {aiAvailable && ' · AI 花费为全平台口径（按价格表估算）'}
          </p>
        </div>
        <span className="text-xs px-3 py-1 rounded-full bg-white/50 dark:bg-white/10 text-gray-500 dark:text-gray-400 whitespace-nowrap">
          {period}
        </span>
      </div>

      {aiAvailable ? (
        // 净节省口径四格：省人天 / AI 花费 / 净节省（毛−AI）/ 综合提效
        <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-4 gap-x-8 gap-y-6 flex-1">
          <BigStat label="为团队节省" value={savedDays > 0 ? daysCount.toFixed(1) : '-'} unit="人天" tone={emerald} compact />
          <BigStat label="AI 花费" value={`¥${formatNumber(Math.round(aiCount))}`} unit="" tone={neutral} compact />
          <BigStat
            label="净节省"
            value={`¥${formatNumber(Math.round(netCount))}`}
            unit=""
            tone={netSaving < 0 ? rose : emerald}
            compact
          />
          <BigStat label="综合日历提效" value={ratioAvailable ? `${ratioCount.toFixed(1)}%` : '-'} unit="" tone={ratioTone} compact />
        </div>
      ) : (
        // 降级三格（chat 未启用 / 加载中 / 失败）：保持原布局
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-8 flex-1">
          <BigStat label="为团队节省" value={savedDays > 0 ? daysCount.toFixed(1) : '-'} unit="人天" tone={emerald} />
          <BigStat
            label="折合节省成本"
            value={grossSaving > 0 ? `¥${formatNumber(Math.round(grossCount))}` : '-'}
            unit=""
            tone={emerald}
          />
          <BigStat label="综合日历提效" value={ratioAvailable ? `${ratioCount.toFixed(1)}%` : '-'} unit="" tone={ratioTone} />
        </div>
      )}
    </div>
  )
}

function BigStat({
  label,
  value,
  unit,
  tone,
  compact = false,
}: {
  label: string
  value: string
  unit: string
  tone: string
  compact?: boolean
}) {
  const size = compact ? 'text-4xl md:text-5xl' : 'text-5xl md:text-6xl'
  return (
    <div>
      <div className="text-sm font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide mb-2">{label}</div>
      <div className="flex items-baseline gap-1.5">
        <span className={`${size} font-black tabular-nums leading-none ${tone}`}>{value}</span>
        {unit && <span className="text-lg text-gray-400">{unit}</span>}
      </div>
    </div>
  )
}

/** 'YYYYMMDD' → 'YYYY-MM-DD'（chat 侧日期参数格式）；非 8 位原样透传。 */
function ymd8ToDash(s: string): string {
  return s.length === 8 ? `${s.slice(0, 4)}-${s.slice(4, 6)}-${s.slice(6, 8)}` : s
}

/** 'YYYYMMDD'×2 → '2026/03/06 ~ 2026/06/04' */
function fmtPeriod(start: string, end: string): string {
  const f = (s: string) => (s.length === 8 ? `${s.slice(0, 4)}/${s.slice(4, 6)}/${s.slice(6, 8)}` : s)
  return `${f(start)} ~ ${f(end)}`
}
