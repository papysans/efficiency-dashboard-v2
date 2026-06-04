// 三态升降表头按钮（移植自 Vue components/native/SortableTh.vue，玻璃风格）。
// 父组件负责 none→asc→desc→none 的三态循环（onSort）；本组件只负责渲染 + aria-sort。
// 见 research/pr2-need-pages.md §4。

interface SortableThProps {
  field: string
  label: string
  active?: boolean
  desc?: boolean
  /** 数字列右对齐 */
  numeric?: boolean
  onSort: (field: string) => void
}

export function SortableTh({ field, label, active = false, desc = false, numeric = false, onSort }: SortableThProps) {
  const ariaSort = active ? (desc ? 'descending' : 'ascending') : 'none'
  const upOn = active && !desc
  const downOn = active && desc

  return (
    <button
      type="button"
      onClick={() => onSort(field)}
      aria-sort={ariaSort}
      className={`inline-flex items-center gap-1 cursor-pointer bg-transparent border-none p-0 font-semibold transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue rounded ${
        numeric ? 'flex-row-reverse' : ''
      } ${active ? 'text-apple-blue' : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200'}`}
    >
      <span>{label}</span>
      <span className="inline-flex flex-col leading-none" aria-hidden="true">
        <svg className={`w-2.5 h-2.5 -mb-0.5 ${upOn ? 'text-apple-blue' : 'text-gray-300 dark:text-gray-600'}`} viewBox="0 0 12 12" fill="currentColor">
          <path d="M6 2.5L9.5 7.5h-7z" />
        </svg>
        <svg className={`w-2.5 h-2.5 ${downOn ? 'text-apple-blue' : 'text-gray-300 dark:text-gray-600'}`} viewBox="0 0 12 12" fill="currentColor">
          <path d="M6 9.5L2.5 4.5h7z" />
        </svg>
      </span>
    </button>
  )
}
