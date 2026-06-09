import { useMemo } from 'react'
import type { EChartsOption } from 'echarts'
import { useDashboardSummary } from '@/api/queries'
import { useTheme } from '@/hooks/useTheme'
import { EChart } from '@/components/charts/EChart'
import { getPalette } from '@/components/charts/chartTheme'
import { formatNumber, formatV2Ratio } from '@/lib/formatters'

interface AdoptionCardProps {
  startDate: string
  endDate: string
}

const ADOPTION_TIP =
  '按可计入且非异常 Need 聚合：AI 覆盖代码行 / 净代码行。'

/**
 * AI 代码占比（design-pr1 §1④）：小数口径，中心大字百分比。
 */
export function AdoptionCard({ startDate, endDate }: AdoptionCardProps) {
  const { theme } = useTheme()
  const { data, isLoading, error } = useDashboardSummary({ startDate, endDate })

  const total = data?.total_needs ?? 0
  const eligible = data?.eligible_needs ?? 0
  const merged = data?.merged_needs ?? 0
  const ratio = data?.ai_code_ratio ?? null
  const pct = ratio == null ? 0 : Math.max(0, Math.min(100, ratio * 100))

  const option = useMemo<EChartsOption>(() => {
    const p = getPalette(theme)
    const ringTrack = theme === 'dark' ? '#2a2a35' : '#e5e7eb'
    return {
      animation: true,
      series: [
        {
          type: 'pie',
          radius: ['70%', '90%'],
          center: ['50%', '50%'],
          startAngle: 90,
          silent: true,
          label: { show: false },
          labelLine: { show: false },
          data: [
            { value: pct, itemStyle: { color: p.brand } },
            { value: Math.max(0, 100 - pct), itemStyle: { color: ringTrack } },
          ],
        },
      ],
    }
  }, [theme, pct])

  return (
    <div className="glass rounded-2xl p-5 md:p-6 min-h-[20rem] hover:shadow-lg transition-shadow flex flex-col">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-sm font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide">AI 代码占比</h2>
        <span className="text-gray-400 cursor-help inline-flex" title={ADOPTION_TIP} aria-label={ADOPTION_TIP}>
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        </span>
      </div>

      {error ? (
        <Centered>加载失败：{(error as Error).message}</Centered>
      ) : isLoading || !data ? (
        <div className="flex-1 skeleton rounded-xl min-h-[16rem]" />
      ) : (
        <div className="flex-1 flex flex-col items-center justify-center">
          <div className="relative w-[180px] h-[180px]">
            <EChart option={option} height={180} />
            <div className="absolute inset-0 flex flex-col items-center justify-center pointer-events-none">
              <span className="text-4xl font-black tabular-nums text-gray-900 dark:text-white">{ratio == null ? '-' : formatV2Ratio(ratio, 0)}</span>
              <span className="text-xs text-gray-500 dark:text-gray-400 mt-1">AI 覆盖代码占比</span>
            </div>
          </div>
          <div className="mt-5 w-full space-y-1.5 text-sm">
            <Row label="可计入需求" value={`${formatNumber(eligible)} / ${formatNumber(total)}`} />
            <Row label="已合并需求" value={formatNumber(merged)} />
          </div>
        </div>
      )}
    </div>
  )
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between">
      <span className="text-gray-500 dark:text-gray-400">{label}</span>
      <span className="font-semibold tabular-nums text-gray-900 dark:text-white">{value}</span>
    </div>
  )
}

function Centered({ children }: { children: React.ReactNode }) {
  return <div className="flex-1 flex items-center justify-center text-sm text-gray-500 dark:text-gray-400 min-h-[16rem]">{children}</div>
}
