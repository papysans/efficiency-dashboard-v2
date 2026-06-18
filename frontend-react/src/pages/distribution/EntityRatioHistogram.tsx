// 「各对象提效比」直方图（项目/仓库/个人 主体专用）。横轴 = 提效比分档（复用 distribution 等宽分桶），
// 纵轴 = 对象个数。与 DistributionOverview 的「全量需求分布」区分：那是需求口径（组织/公司），这里是
// 「每个对象一个 ratio」的对象口径。算法复用 lib/distribution.computeRatioHistogram（不重写分桶）。
//
// scale='decimal'（项目 need_calendar_efficiency_ratio / 个人 calendar_ratio，小数口径）
// scale='percent'（仓库 efficiency_ratio，百分比口径）—— 口径绝不混：percent 在 computeRatioHistogram 内 /100。
import { useMemo, useState } from 'react'
import type { EChartsOption } from 'echarts'
import { useTheme } from '@/hooks/useTheme'
import { computeRatioHistogram, GRANULARITY_PRESETS } from '@/lib/distribution'
import { formatNumber } from '@/lib/formatters'
import { Glass } from '@/components/ui/Glass'
import { EChart } from '@/components/charts/EChart'
import { getPalette, type ChartTheme } from '@/components/charts/chartTheme'

const DEFAULT_BINS = 6

/** 单系列柱状图 option（对象个数）。与 DistributionOverview.histogramOption 同风格，但单系列不堆叠。 */
function ratioBarOption(theme: ChartTheme, labels: string[], counts: number[]): EChartsOption {
  const p = getPalette(theme)
  return {
    animation: true,
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' },
      backgroundColor: p.tooltipBg,
      borderColor: p.tooltipBorder,
      borderWidth: 1,
      textStyle: { color: p.tooltipText },
    },
    grid: { left: '3%', right: '4%', top: 16, bottom: 24, containLabel: true },
    xAxis: {
      type: 'category',
      data: labels,
      axisLine: { lineStyle: { color: p.axisColor } },
      axisLabel: { color: p.textColor, fontSize: 11, hideOverlap: true, rotate: labels.length > 10 ? 30 : 0 },
    },
    yAxis: {
      type: 'value',
      minInterval: 1,
      axisLabel: { color: p.textColor },
      splitLine: { lineStyle: { color: p.splitLineColor } },
    },
    series: [{ name: '对象数', type: 'bar', data: counts, itemStyle: { color: p.brand, borderRadius: [4, 4, 0, 0] } }],
  }
}

/** 粒度分段控件（与 DistributionOverview 同视觉，组件内自持 bins 不入 URL）。 */
function GranularitySegmented({ value, onChange }: { value: number; onChange: (v: number) => void }) {
  return (
    <div className="inline-flex glass rounded-lg p-0.5 gap-0.5">
      {GRANULARITY_PRESETS.map((g) => (
        <button
          key={g.bins}
          type="button"
          onClick={() => onChange(g.bins)}
          className={`px-3 py-1 rounded-md text-sm font-medium cursor-pointer transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue ${
            g.bins === value ? 'bg-apple-blue text-white' : 'text-gray-600 dark:text-gray-300 hover:text-apple-blue'
          }`}
        >
          {g.label} {g.bins}
        </button>
      ))}
    </div>
  )
}

interface EntityRatioHistogramProps {
  /** 各对象的提效比标量（小数或百分比，由 scale 指定）。 */
  ratios: Array<number | null | undefined>
  /** 'decimal' = 小数口径(0.25=25%) / 'percent' = 百分比口径(300=300%)。 */
  scale: 'decimal' | 'percent'
  /** 分布对象名（如「项目」「仓库」「用户」），用于标题/纵轴说明/空态。 */
  entityLabel: string
  /** 口径说明（标题旁小字）。 */
  caliberNote: string
  loading?: boolean
  error?: string | null
}

/** 各对象提效比直方图：手调粒度即时重算（纯前端，不查后端），数据稀疏走友好空态。 */
export function EntityRatioHistogram({
  ratios,
  scale,
  entityLabel,
  caliberNote,
  loading = false,
  error = null,
}: EntityRatioHistogramProps) {
  const { theme } = useTheme()
  const [bins, setBins] = useState(DEFAULT_BINS)
  const result = useMemo(() => computeRatioHistogram(ratios, bins, scale), [ratios, bins, scale])
  const option = useMemo(
    () =>
      ratioBarOption(
        theme,
        result.histogram.map((b) => b.label),
        result.histogram.map((b) => b.count),
      ),
    [theme, result],
  )

  return (
    <Glass className="p-5 space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <h2 className="text-sm font-semibold text-gray-700 dark:text-gray-200">
            各{entityLabel}提效比分布
          </h2>
          <p className="text-xs text-gray-400 dark:text-gray-500 mt-0.5">
            横轴=提效比分档 · 纵轴={entityLabel}个数 · {caliberNote} · 共 {formatNumber(result.total)} 个{entityLabel}
          </p>
        </div>
        <div className="flex items-center gap-1.5">
          <span className="text-xs text-gray-400 dark:text-gray-500">粒度</span>
          <GranularitySegmented value={bins} onChange={setBins} />
        </div>
      </div>

      {error ? (
        <div className="h-[300px] flex items-center justify-center text-sm text-rose-600 dark:text-rose-400">
          加载失败：{error}
        </div>
      ) : loading ? (
        <div className="skeleton h-[300px] rounded-xl" />
      ) : result.total === 0 ? (
        <div className="h-[300px] flex items-center justify-center text-sm text-gray-400 dark:text-gray-500">
          该时间段暂无可计入分布的{entityLabel}提效数据
        </div>
      ) : (
        <EChart option={option} height={300} />
      )}
    </Glass>
  )
}
