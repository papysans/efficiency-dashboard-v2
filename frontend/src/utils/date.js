/**
 * 获取最近7天的日期范围
 * @returns {[string, string]} [startStr, endStr] 格式 YYYY-MM-DD
 */
export function getDefaultDateRange() {
  const end = new Date()
  const start = new Date()
  start.setDate(start.getDate() - 6)
  const fmt = (d) => {
    const y = d.getFullYear()
    const m = String(d.getMonth() + 1).padStart(2, '0')
    const day = String(d.getDate()).padStart(2, '0')
    return `${y}-${m}-${day}`
  }
  return [fmt(start), fmt(end)]
}

/**
 * 将 'YYYY-MM-DD' 格式转为 'YYYYMMDD'
 * @param {string} dateStr
 * @returns {string}
 */
export function formatDateParam(dateStr) {
  return dateStr.replace(/-/g, '')
}

/**
 * 获取最近N天的日期范围（默认90天），用于数据跨度较大的页面
 * @param {number} days 天数，默认90
 * @returns {[string, string]} [startStr, endStr] 格式 YYYY-MM-DD
 */
export function getDefaultDateRangeWide(days = 90) {
  const end = new Date()
  const start = new Date()
  start.setDate(start.getDate() - (days - 1))
  const fmt = (d) => {
    const y = d.getFullYear()
    const m = String(d.getMonth() + 1).padStart(2, '0')
    const day = String(d.getDate()).padStart(2, '0')
    return `${y}-${m}-${day}`
  }
  return [fmt(start), fmt(end)]
}
