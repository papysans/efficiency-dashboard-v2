// 玻璃风格日期范围选择（移植 Vue components/DateRangePicker.vue 的快捷项 + 范围交互）。
// 纯实现：input[type=date]×2 + 快捷按钮，不引第三方库。输出 [start,end] YYYY-MM-DD 字符串
// （列表 buildQuery 依赖此格式经 formatDateParam）。见 design-pr2-need.md「新建文件」。
import { useEffect, useRef, useState } from 'react'

interface DateRangePickerProps {
  value: [string, string]
  onChange: (range: [string, string]) => void
}

function fmt(d: Date): string {
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

function rangeFromDays(days: number): [string, string] {
  const end = new Date()
  const start = new Date()
  start.setDate(start.getDate() - (days - 1))
  return [fmt(start), fmt(end)]
}

// 快捷项对齐 Vue（今天 / 1 天前 / 3 天前 / 1 周前 / 1 月前 / 3 月前）。
const SHORTCUTS: Array<{ label: string; days: number }> = [
  { label: '今天', days: 1 },
  { label: '1 天前', days: 2 },
  { label: '3 天前', days: 4 },
  { label: '1 周前', days: 7 },
  { label: '1 月前', days: 30 },
  { label: '3 月前', days: 90 },
]

export function DateRangePicker({ value, onChange }: DateRangePickerProps) {
  const [open, setOpen] = useState(false)
  const rootRef = useRef<HTMLDivElement | null>(null)
  const [start, end] = value

  // 点击外部关闭面板
  useEffect(() => {
    if (!open) return
    function onDocClick(e: MouseEvent) {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', onDocClick)
    return () => document.removeEventListener('mousedown', onDocClick)
  }, [open])

  const inputCls =
    'glass rounded-lg px-2 py-1 text-sm bg-transparent text-gray-900 dark:text-white ' +
    'focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue cursor-pointer [color-scheme:light] dark:[color-scheme:dark]'

  return (
    <div ref={rootRef} className="relative inline-block">
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        className="glass rounded-lg px-3 py-1.5 text-sm flex items-center gap-2 cursor-pointer text-gray-700 dark:text-gray-200 hover:text-apple-blue transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue"
        aria-haspopup="dialog"
        aria-expanded={open}
      >
        <svg className="w-4 h-4 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
        </svg>
        <span className="tabular-nums">{start} ~ {end}</span>
      </button>

      {open && (
        <div className="glass rounded-2xl p-4 absolute z-50 mt-2 left-0 flex gap-4 min-w-[20rem]" role="dialog">
          <div className="flex flex-col gap-1 pr-3 border-r border-gray-200/50 dark:border-white/10">
            {SHORTCUTS.map((sc) => (
              <button
                key={sc.label}
                type="button"
                onClick={() => {
                  onChange(rangeFromDays(sc.days))
                  setOpen(false)
                }}
                className="text-left text-sm px-2 py-1 rounded-lg text-gray-600 dark:text-gray-300 hover:bg-apple-blue hover:text-white transition-colors cursor-pointer whitespace-nowrap focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue"
              >
                {sc.label}
              </button>
            ))}
          </div>
          <div className="flex flex-col gap-2">
            <label className="flex flex-col gap-1 text-xs text-gray-500 dark:text-gray-400">
              开始
              <input
                type="date"
                value={start}
                max={end || undefined}
                onChange={(e) => e.target.value && onChange([e.target.value, end])}
                className={inputCls}
              />
            </label>
            <label className="flex flex-col gap-1 text-xs text-gray-500 dark:text-gray-400">
              结束
              <input
                type="date"
                value={end}
                min={start || undefined}
                onChange={(e) => e.target.value && onChange([start, e.target.value])}
                className={inputCls}
              />
            </label>
          </div>
        </div>
      )}
    </div>
  )
}
