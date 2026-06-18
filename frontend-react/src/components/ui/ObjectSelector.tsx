// 可搜索对象选择器（玻璃拟态 combobox）。聚合态/聚焦态切换的入口。
// 选「全部」=聚合态(value=空)，选某项=聚焦态。受控组件：value/onChange 由 EntityDimensionLayout 持有
// 并同步到 URL ?object=。键盘可达（focus-visible ring + Esc 关 + 回车选中高亮项）。
import { useEffect, useMemo, useRef, useState } from 'react'
import type { EntityOption } from '@/hooks/useEntityObjects'

interface ObjectSelectorProps {
  options: EntityOption[]
  /** 当前选中对象 value（空串=全部/聚合态）。 */
  value: string
  onChange: (value: string) => void
  loading?: boolean
  /** 「全部XX」文案。 */
  allLabel: string
  placeholder?: string
}

export function ObjectSelector({ options, value, onChange, loading = false, allLabel, placeholder = '搜索…' }: ObjectSelectorProps) {
  const [open, setOpen] = useState(false)
  const [kw, setKw] = useState('')
  const [activeIdx, setActiveIdx] = useState(0)
  const boxRef = useRef<HTMLDivElement>(null)

  const selectedLabel = useMemo(() => {
    if (!value) return allLabel
    return options.find((o) => o.value === value)?.label.trim() || value
  }, [value, options, allLabel])

  // 过滤（trim 去空格，匹配 label 与 value）。「全部」选项恒在最前。
  const filtered = useMemo(() => {
    const q = kw.trim().toLowerCase()
    const base: EntityOption[] = [{ value: '', label: allLabel }, ...options]
    if (!q) return base
    return base.filter((o) => `${o.label}${o.value}`.toLowerCase().includes(q))
  }, [kw, options, allLabel])

  useEffect(() => {
    if (!open) return
    function onDocClick(e: MouseEvent) {
      if (boxRef.current && !boxRef.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', onDocClick)
    return () => document.removeEventListener('mousedown', onDocClick)
  }, [open])

  useEffect(() => {
    if (open) setActiveIdx(0)
  }, [open, kw])

  function pick(v: string) {
    onChange(v)
    setOpen(false)
    setKw('')
  }

  function onKeyDown(e: React.KeyboardEvent) {
    if (e.key === 'Escape') {
      setOpen(false)
      return
    }
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      setActiveIdx((i) => Math.min(i + 1, filtered.length - 1))
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      setActiveIdx((i) => Math.max(i - 1, 0))
    } else if (e.key === 'Enter') {
      e.preventDefault()
      const opt = filtered[activeIdx]
      if (opt) pick(opt.value)
    }
  }

  return (
    <div ref={boxRef} className="relative min-w-[180px]">
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        disabled={loading}
        aria-haspopup="listbox"
        aria-expanded={open}
        className="glass rounded-lg px-3 py-1.5 text-sm bg-transparent text-gray-900 dark:text-white inline-flex items-center justify-between gap-2 w-full cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue transition-colors"
      >
        <span className="truncate" title={selectedLabel}>{loading ? '加载中…' : selectedLabel}</span>
        <svg className={`w-4 h-4 shrink-0 text-gray-400 transition-transform ${open ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
        </svg>
      </button>

      {open && (
        <div className="glass absolute right-0 mt-1 z-50 w-72 max-w-[80vw] rounded-lg p-2 shadow-xl">
          <input
            autoFocus
            value={kw}
            onChange={(e) => setKw(e.target.value)}
            onKeyDown={onKeyDown}
            placeholder={placeholder}
            className="w-full rounded-md px-2.5 py-1.5 text-sm bg-white/60 dark:bg-white/10 text-gray-900 dark:text-white placeholder-gray-400 focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue mb-1.5"
          />
          <ul role="listbox" className="list-none m-0 p-0 max-h-64 overflow-y-auto">
            {filtered.length === 0 ? (
              <li className="px-2.5 py-2 text-sm text-gray-400 dark:text-gray-500 text-center">无匹配项</li>
            ) : (
              filtered.map((o, i) => {
                const isSel = o.value === value
                const isActive = i === activeIdx
                return (
                  <li key={o.value || '__all__'} role="option" aria-selected={isSel}>
                    <button
                      type="button"
                      onMouseEnter={() => setActiveIdx(i)}
                      onClick={() => pick(o.value)}
                      className={`w-full text-left px-2.5 py-1.5 rounded-md text-sm cursor-pointer transition-colors truncate ${
                        isSel
                          ? 'bg-apple-blue text-white'
                          : isActive
                            ? 'bg-apple-blue/10 text-gray-900 dark:text-white'
                            : 'text-gray-700 dark:text-gray-200 hover:bg-white/50 dark:hover:bg-white/10'
                      }`}
                      title={o.label.trim() || allLabel}
                    >
                      {o.label}
                    </button>
                  </li>
                )
              })
            )}
          </ul>
        </div>
      )}
    </div>
  )
}
