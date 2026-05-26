/**
 * 格式化费用（保留2位小数）
 * 兼容 el-table :formatter 签名
 */
export function fmtCost(row, col, value) {
  if (value == null) return ''
  return Number(value).toFixed(2)
}

/**
 * v2 提效比格式化。
 * v2 ratio 是小数口径：0.25 表示 25%，不同于 legacy 300=300%。
 */
export function formatV2Ratio(value, digits = 1) {
  if (value == null || value === '') return '-'
  const num = Number(value)
  if (!Number.isFinite(num)) return '-'
  return `${(num * 100).toFixed(digits)}%`
}

/**
 * 数值千分位格式化。
 */
export function formatNumber(value, digits = 0) {
  if (value == null || value === '') return '-'
  const num = Number(value)
  if (!Number.isFinite(num)) return '-'
  return num.toLocaleString('zh-CN', {
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  })
}

/**
 * AI预估人天（保留1位小数，0 返回 '-'）
 * 兼容 el-table :formatter 签名
 */
export function fmtDays(row, col, value) {
  if (value == null || value === 0) return '-'
  return Number(value).toFixed(1)
}

/**
 * 毫秒转分钟显示（保留1位小数，后缀 ' min'）
 * 兼容 el-table :formatter 签名
 */
export function fmtMsToMin(row, col, value) {
  if (value == null) return ''
  const minutes = Number(value) / 60000
  return minutes.toFixed(1) + ' min'
}

/**
 * ISO 8601 字符串转本地时间 YYYY-MM-DD HH:mm:ss
 */
export function formatLocalTime(isoStr) {
  if (!isoStr) return '-'
  const d = new Date(isoStr)
  if (isNaN(d.getTime())) return '-'
  const Y = d.getFullYear()
  const M = String(d.getMonth() + 1).padStart(2, '0')
  const D = String(d.getDate()).padStart(2, '0')
  const h = String(d.getHours()).padStart(2, '0')
  const m = String(d.getMinutes()).padStart(2, '0')
  const s = String(d.getSeconds()).padStart(2, '0')
  return `${Y}-${M}-${D} ${h}:${m}:${s}`
}

/**
 * 分钟数自适应显示：分钟 / 小时分钟 / 人天
 */
export function formatDuration(minutes) {
  if (minutes == null || minutes === 0) return '-'
  const m = Math.round(Number(minutes))
  if (m < 60) return `${m}分钟`
  if (m <= 480) {
    const h = Math.floor(m / 60)
    const rem = m % 60
    return rem === 0 ? `${h}小时` : `${h}小时${rem}分钟`
  }
  return (m / 480).toFixed(1) + '人天'
}
