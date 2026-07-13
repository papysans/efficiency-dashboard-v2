// 玻璃风格模态框（人工调整 dialog 用）：遮罩 + Esc 关闭 + focus trap + 点击遮罩关闭。
// 尊重 prefers-reduced-motion（渐入动画在 motion-reduce 下禁用）。
import { useCallback, useEffect, useRef, type ReactNode } from 'react'

interface ModalProps {
  open: boolean
  title: string
  onClose: () => void
  children: ReactNode
  footer?: ReactNode
  /** 内容最大宽度，默认 600px */
  maxWidth?: number
  /** 遮罩层级，默认 100；嵌套弹层可提高层级 */
  zIndex?: number
}

const FOCUSABLE =
  'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])'

export function Modal({ open, title, onClose, children, footer, maxWidth = 600, zIndex = 100 }: ModalProps) {
  const panelRef = useRef<HTMLDivElement | null>(null)
  const prevFocus = useRef<HTMLElement | null>(null)

  // Esc 关闭 + Tab 循环（focus trap）
  const onKeyDown = useCallback(
    (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.stopPropagation()
        onClose()
        return
      }
      if (e.key !== 'Tab' || !panelRef.current) return
      const nodes = Array.from(panelRef.current.querySelectorAll<HTMLElement>(FOCUSABLE)).filter(
        (el) => el.offsetParent !== null,
      )
      if (nodes.length === 0) return
      const first = nodes[0]
      const last = nodes[nodes.length - 1]
      const active = document.activeElement as HTMLElement | null
      if (e.shiftKey && active === first) {
        e.preventDefault()
        last.focus()
      } else if (!e.shiftKey && active === last) {
        e.preventDefault()
        first.focus()
      }
    },
    [onClose],
  )

  useEffect(() => {
    if (!open) return
    prevFocus.current = document.activeElement as HTMLElement | null
    document.addEventListener('keydown', onKeyDown, true)
    // 打开时聚焦面板内第一个可聚焦元素
    const t = window.setTimeout(() => {
      const nodes = panelRef.current?.querySelectorAll<HTMLElement>(FOCUSABLE)
      nodes?.[0]?.focus()
    }, 0)
    return () => {
      document.removeEventListener('keydown', onKeyDown, true)
      window.clearTimeout(t)
      prevFocus.current?.focus?.()
    }
  }, [open, onKeyDown])

  if (!open) return null

  return (
    <div
      className="fixed inset-0 flex items-center justify-center p-4 bg-black/40 backdrop-blur-sm"
      style={{ zIndex }}
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose()
      }}
    >
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-label={title}
        style={{ maxWidth }}
        className="glass rounded-2xl w-full max-h-[90vh] overflow-y-auto animate-[fade-in-up_.25s_ease-out_both] motion-reduce:animate-none"
      >
        <div className="flex items-center justify-between px-5 py-3 border-b border-gray-200/50 dark:border-white/10">
          <h2 className="text-base font-semibold text-gray-900 dark:text-white">{title}</h2>
          <button
            type="button"
            onClick={onClose}
            aria-label="关闭"
            className="text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 cursor-pointer bg-transparent border-none p-1 rounded-lg transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue"
          >
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
        <div className="p-5">{children}</div>
        {footer && (
          <div className="flex items-center justify-end gap-2 px-5 py-3 border-t border-gray-200/50 dark:border-white/10">
            {footer}
          </div>
        )}
      </div>
    </div>
  )
}
