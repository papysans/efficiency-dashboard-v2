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

/**
 * 验证时长采集缺失提示文案。
 * 上游 rawdata 只记录用户↔AI 一问一答文本，不记录工具调用 / bash 命令执行，
 * 后端按命令白名单判定验证阶段 → verify 恒为 0。值为 0/空时显示「—」并以此文案说明。
 */
export const VERIFY_UNAVAILABLE_TIP = '当前采集口径未记录命令执行（bash / 测试 / 编译），验证时长不可用'

/**
 * 思考 / 执行阶段的粗略口径提示文案。
 */
export const STAGE_ESTIMATE_TIP = '思考 / 执行为粗略口径：基于对话轮与代码 diff 推断，含时长估算'

/**
 * 实际采集维度的验证时长格式化：0/空显示「—」（采集未覆盖），非 0 时正常显示。
 * 注意：仅用于实际采集的验证时长（total_verify_min / verify_active_min），
 * 不适用于基线估算维度（baselineRows 里的理论验证段）。
 */
export function formatVerifyMin(minutes) {
  if (Number(minutes || 0) === 0) return '—'
  return formatDuration(minutes)
}
