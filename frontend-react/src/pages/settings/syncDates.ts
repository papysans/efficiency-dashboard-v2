const DATE_RE = /^(\d{4})-(\d{2})-(\d{2})$/

function parseDate(value: string): [number, number, number] | null {
  const match = DATE_RE.exec(value)
  if (!match) return null
  const year = Number(match[1])
  const month = Number(match[2])
  const day = Number(match[3])
  const date = new Date(Date.UTC(year, month - 1, day))
  if (date.getUTCFullYear() !== year || date.getUTCMonth() !== month - 1 || date.getUTCDate() !== day) return null
  return [year, month, day]
}

export function addCalendarDays(value: string, days: number): string | null {
  const parsed = parseDate(value)
  if (!parsed) return null
  const date = new Date(Date.UTC(parsed[0], parsed[1] - 1, parsed[2] + days))
  const year = date.getUTCFullYear()
  const month = String(date.getUTCMonth() + 1).padStart(2, '0')
  const day = String(date.getUTCDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

/** 看板日期范围为首尾日期均包含；同步 API 的 end_time 是排他边界。 */
export function toShanghaiSyncRange(startDate: string, endDate: string): { start_time: string; end_time: string } | null {
  if (!parseDate(startDate) || !parseDate(endDate) || startDate > endDate) return null
  const exclusiveEnd = addCalendarDays(endDate, 1)
  if (!exclusiveEnd) return null
  return {
    start_time: `${startDate}T00:00:00+08:00`,
    end_time: `${exclusiveEnd}T00:00:00+08:00`,
  }
}

function shanghaiDate(instant: Date): string {
  return new Date(instant.getTime() + 8 * 60 * 60 * 1000).toISOString().slice(0, 10)
}

/** 将后端半开区间显示为首尾日期均包含的北京时间范围；非整日范围返回 null。 */
export function formatShanghaiDayRange(startTime: string, endTime: string): string | null {
  const start = new Date(startTime)
  const end = new Date(endTime)
  if (Number.isNaN(start.getTime()) || Number.isNaN(end.getTime()) || start >= end) return null
  const startShanghai = new Date(start.getTime() + 8 * 60 * 60 * 1000)
  const endShanghai = new Date(end.getTime() + 8 * 60 * 60 * 1000)
  const isMidnight = (date: Date) =>
    date.getUTCHours() === 0 && date.getUTCMinutes() === 0 && date.getUTCSeconds() === 0 && date.getUTCMilliseconds() === 0
  if (!isMidnight(startShanghai) || !isMidnight(endShanghai)) return null
  return `${shanghaiDate(start)} ~ ${shanghaiDate(new Date(end.getTime() - 1))}`
}
