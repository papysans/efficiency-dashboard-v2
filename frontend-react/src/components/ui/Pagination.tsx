// 玻璃风格分页（移植 Vue el-pagination 的 total/sizes/prev/pager/next 行为）。
// pageSize 选项 [20,50,100,200]，对齐 Need 列表（research/pr2-need-pages.md §2.9）。

interface PaginationProps {
  page: number
  pageSize: number
  total: number
  pageSizes?: number[]
  onPageChange: (page: number) => void
  onSizeChange: (size: number) => void
}

const ICON_BTN =
  'inline-flex items-center justify-center w-8 h-8 rounded-lg text-sm transition-colors cursor-pointer ' +
  'disabled:opacity-40 disabled:cursor-not-allowed focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue'

/** 生成页码列表：始终含首尾，当前页前后各 1，超出用 -1 占位省略号。 */
function pageList(current: number, totalPages: number): number[] {
  if (totalPages <= 7) return Array.from({ length: totalPages }, (_, i) => i + 1)
  const set = new Set<number>([1, totalPages, current, current - 1, current + 1])
  const pages = Array.from(set)
    .filter((p) => p >= 1 && p <= totalPages)
    .sort((a, b) => a - b)
  const out: number[] = []
  let prev = 0
  for (const p of pages) {
    if (p - prev > 1) out.push(-1)
    out.push(p)
    prev = p
  }
  return out
}

export function Pagination({
  page,
  pageSize,
  total,
  pageSizes = [20, 50, 100, 200],
  onPageChange,
  onSizeChange,
}: PaginationProps) {
  const totalPages = Math.max(1, Math.ceil(total / pageSize))
  const pages = pageList(page, totalPages)

  return (
    <div className="flex flex-wrap items-center gap-3 text-sm text-gray-600 dark:text-gray-300">
      <span>共 {total} 条</span>

      <label className="flex items-center gap-1">
        <select
          value={pageSize}
          onChange={(e) => onSizeChange(Number(e.target.value))}
          className="glass rounded-lg px-2 py-1 text-sm bg-transparent cursor-pointer focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue"
        >
          {pageSizes.map((s) => (
            <option key={s} value={s}>
              {s}/页
            </option>
          ))}
        </select>
      </label>

      <div className="flex items-center gap-1">
        <button
          type="button"
          className={`glass ${ICON_BTN}`}
          disabled={page <= 1}
          onClick={() => onPageChange(page - 1)}
          aria-label="上一页"
        >
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
          </svg>
        </button>

        {pages.map((p, i) =>
          p === -1 ? (
            <span key={`gap-${i}`} className="px-1 text-gray-400">
              …
            </span>
          ) : (
            <button
              key={p}
              type="button"
              className={`${ICON_BTN} ${p === page ? 'bg-apple-blue text-white' : 'glass hover:text-apple-blue'}`}
              aria-current={p === page ? 'page' : undefined}
              onClick={() => onPageChange(p)}
            >
              {p}
            </button>
          ),
        )}

        <button
          type="button"
          className={`glass ${ICON_BTN}`}
          disabled={page >= totalPages}
          onClick={() => onPageChange(page + 1)}
          aria-label="下一页"
        >
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
          </svg>
        </button>
      </div>
    </div>
  )
}
