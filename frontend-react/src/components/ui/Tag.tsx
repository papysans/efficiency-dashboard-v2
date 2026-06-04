import type { ReactNode } from 'react'

// 玻璃语义标签（替代 Vue 的 .kn-tag--*）。亮暗两套 Tailwind 类。
export type TagTone = 'neutral' | 'primary' | 'success' | 'warning' | 'error' | 'info'

const TONE_CLASS: Record<TagTone, string> = {
  neutral: 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300',
  primary: 'bg-blue-100 text-blue-700 dark:bg-blue-900/50 dark:text-blue-300',
  success: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/50 dark:text-emerald-300',
  warning: 'bg-amber-100 text-amber-700 dark:bg-amber-900/50 dark:text-amber-300',
  error: 'bg-rose-100 text-rose-700 dark:bg-rose-900/50 dark:text-rose-300',
  info: 'bg-sky-100 text-sky-700 dark:bg-sky-900/50 dark:text-sky-300',
}

interface TagProps {
  tone?: TagTone
  mono?: boolean
  title?: string
  children: ReactNode
}

export function Tag({ tone = 'neutral', mono = false, title, children }: TagProps) {
  return (
    <span
      title={title}
      className={`inline-block text-xs px-2 py-0.5 rounded-full font-medium ${mono ? 'font-mono' : ''} ${TONE_CLASS[tone]}`}
    >
      {children}
    </span>
  )
}
