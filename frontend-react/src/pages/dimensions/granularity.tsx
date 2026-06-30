// 趋势粒度（天/周/月）共享：useGranularity 随所选区间重置默认；GranularityToggle 段控（每页一个统一控制）。
import { useEffect, useMemo, useState } from 'react'
import {
  type Granularity,
  GRANULARITY_CN,
  availableGranularities,
  defaultGranularity,
  rangeDays,
} from '@/lib/timeBucket'

/** 接所选区间，返回当前粒度 + 可选项；切换区间（start/end 任一变）即重置为该区间默认值。 */
export function useGranularity(start: string, end: string) {
  const span = rangeDays(start, end)
  const options = useMemo(() => availableGranularities(span), [span])
  const [gran, setGran] = useState<Granularity>(() => defaultGranularity(span))
  // 绑 start/end：切区间必重置（即便两区间默认粒度相同）；用户手动选择只在同一区间内保留。
  useEffect(() => {
    setGran(defaultGranularity(rangeDays(start, end)))
  }, [start, end])
  return { gran, setGran, options }
}

/** 段控：天/周/月。可选项 < 2（区间 < 14 天）时不渲染。 */
export function GranularityToggle({
  value,
  options,
  onChange,
}: {
  value: Granularity
  options: Granularity[]
  onChange: (g: Granularity) => void
}) {
  if (options.length < 2) return null
  return (
    <div
      className="inline-flex items-center rounded-lg bg-gray-100/70 dark:bg-white/5 p-0.5"
      role="group"
      aria-label="趋势粒度"
    >
      {options.map((g) => {
        const active = g === value
        return (
          <button
            key={g}
            type="button"
            aria-pressed={active}
            onClick={() => onChange(g)}
            className={`px-2.5 py-1 rounded-md text-xs font-medium transition-colors cursor-pointer border-none focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue ${
              active
                ? 'bg-apple-blue text-white'
                : 'bg-transparent text-gray-500 dark:text-gray-400 hover:text-gray-800 dark:hover:text-white'
            }`}
          >
            {GRANULARITY_CN[g]}
          </button>
        )
      })}
    </div>
  )
}
