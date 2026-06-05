// 组织详情共享样式/工具：被 OrgDetail.tsx、OrgDetailPanel.tsx、OrgTree.tsx 复用，避免复制。
import type { ReactNode } from 'react'

// 表格表头/单元格通用 class（玻璃拟态风格，与 OrgList 一致）。
export const TH = 'px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
export const TH_NUM = 'px-3 py-2 text-right font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
export const TH_CENTER = 'px-3 py-2 text-center font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
export const TD = 'px-3 py-2 align-middle text-gray-700 dark:text-gray-200'
export const TD_NUM = 'px-3 py-2 align-middle text-right tabular-nums text-gray-700 dark:text-gray-200'

/** 费用单值（null → '-'，否则 2 位）。 */
export function fmtCostVal(value: number | null | undefined): string {
  if (value == null) return '-'
  return Number(value).toFixed(2)
}

/** token K/M 缩写（对齐 Vue fmtTokens）。 */
export function fmtTokens(up?: number | null, down?: number | null): string {
  const total = (up || 0) + (down || 0)
  if (total === 0) return '-'
  if (total >= 1_000_000) return `${(total / 1_000_000).toFixed(1)}M`
  if (total >= 1000) return `${(total / 1000).toFixed(1)}K`
  return String(total)
}

/** 玻璃拟态分区卡片（标题 + 右上 hint + 内容）。 */
export function Panel({ title, hint, children }: { title: string; hint?: string; children: ReactNode }) {
  return (
    <section className="glass rounded-2xl overflow-hidden">
      <div className="flex items-center justify-between px-5 py-3 border-b border-gray-200/50 dark:border-white/10">
        <span className="text-sm font-semibold text-gray-700 dark:text-gray-200">{title}</span>
        {hint && <span className="text-xs text-gray-400 dark:text-gray-500">{hint}</span>}
      </div>
      <div className="overflow-x-auto p-1">{children}</div>
    </section>
  )
}
