/**
 * 列表排序共享工具（移植自 opencode kanban FilterTable）
 *
 * order 约定（与后端 utils.go parseOrderParam 一致）：
 *   '-foo' → 降序，按 foo
 *   'foo'  → 升序，按 foo
 *   空串/null/undefined → 无排序
 */

/**
 * 解析 order 字符串。
 * @param {string|undefined|null} order
 * @returns {{field: string, desc: boolean}|null} 空时返回 null
 */
export function parseOrder(order) {
  const t = (order || '').trim()
  if (!t) return null
  return t.startsWith('-')
    ? { field: t.slice(1), desc: true }
    : { field: t, desc: false }
}

/**
 * 由 field + desc 构造 order 字符串。
 * @param {string|undefined|null} field 为空 → undefined（清除排序）
 * @param {boolean} desc
 * @returns {string|undefined}
 */
export function toOrder(field, desc) {
  if (!field) return undefined
  return desc ? `-${field}` : field
}

/**
 * 非空值比较器：
 *   number  → 数值比较
 *   boolean → false < true
 *   其它    → String(a).localeCompare(String(b))
 * @returns {number} <0 / 0 / >0
 */
export function compareValue(a, b) {
  if (typeof a === 'number' && typeof b === 'number') return a - b
  if (typeof a === 'boolean' && typeof b === 'boolean') return a === b ? 0 : a ? 1 : -1
  return String(a).localeCompare(String(b))
}

/**
 * 客户端稳定排序。
 * - getter(row) == null 的行恒末尾（升降都末尾，不随方向翻转）
 * - 同值按原始 index 稳定保序
 * @param {Array} rows
 * @param {(row:any)=>any} getter 取排序值
 * @param {boolean} desc
 * @returns {Array} 排序后的新数组（不修改入参）
 */
export function sortRows(rows, getter, desc) {
  const idx = rows.map((row, i) => ({ row, i }))
  idx.sort((A, B) => {
    const av = getter(A.row)
    const bv = getter(B.row)
    const ae = av == null
    const be = bv == null
    if (ae || be) {
      if (ae && be) return A.i - B.i
      return ae ? 1 : -1
    }
    const c = compareValue(av, bv)
    if (c !== 0) return desc ? -c : c
    return A.i - B.i
  })
  return idx.map(x => x.row)
}
