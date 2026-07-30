import { describe, expect, it } from 'vitest'
import {
  DEFAULT_REALTIME_QUERY_SORT,
  REALTIME_QUERY_SORT_OPTIONS,
  parseRealtimeQuerySort,
} from './realtimeQuerySort'

describe('realtime query sort', () => {
  it('provides one mutually exclusive option for every field and direction', () => {
    expect(REALTIME_QUERY_SORT_OPTIONS).toHaveLength(12)
    expect(new Set(REALTIME_QUERY_SORT_OPTIONS.map((option) => option.value)).size).toBe(12)
  })

  it('maps a selected option to the API sort parameters', () => {
    expect(parseRealtimeQuerySort('token_output_speed_e2e:asc')).toEqual({
      sort_by: 'token_output_speed_e2e',
      order: 'asc',
    })
  })

  it('falls back to time descending for an invalid value', () => {
    expect(parseRealtimeQuerySort('token_output_speed:sideways')).toEqual(
      parseRealtimeQuerySort(DEFAULT_REALTIME_QUERY_SORT),
    )
  })
})
