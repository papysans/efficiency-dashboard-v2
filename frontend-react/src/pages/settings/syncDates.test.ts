import { describe, expect, it } from 'vitest'
import { addCalendarDays, formatShanghaiDayRange, toShanghaiSyncRange } from './syncDates'

describe('toShanghaiSyncRange', () => {
  it('converts an inclusive date range to a Shanghai half-open interval', () => {
    expect(toShanghaiSyncRange('2026-07-20', '2026-07-21')).toEqual({
      start_time: '2026-07-20T00:00:00+08:00',
      end_time: '2026-07-22T00:00:00+08:00',
    })
  })

  it('handles month boundaries without using the browser timezone', () => {
    expect(addCalendarDays('2026-07-31', 1)).toBe('2026-08-01')
  })

  it('rejects invalid or reversed ranges', () => {
    expect(toShanghaiSyncRange('2026-07-21', '2026-07-20')).toBeNull()
    expect(toShanghaiSyncRange('2026-02-30', '2026-03-01')).toBeNull()
  })
})

describe('formatShanghaiDayRange', () => {
  it('shows an exclusive backend boundary as an inclusive Shanghai date range', () => {
    expect(formatShanghaiDayRange('2026-07-19T16:00:00Z', '2026-07-21T16:00:00Z')).toBe('2026-07-20 ~ 2026-07-21')
  })

  it('falls back for legacy partial-day tasks', () => {
    expect(formatShanghaiDayRange('2026-07-18T10:59:00Z', '2026-07-21T10:59:00Z')).toBeNull()
  })
})
