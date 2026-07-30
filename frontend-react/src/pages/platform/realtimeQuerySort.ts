import type { ChatDetailSortBy, ChatSortOrder } from '@/api/types'

export type RealtimeQuerySortValue = `${ChatDetailSortBy}:${ChatSortOrder}`

export interface RealtimeQuerySortOption {
  label: string
  value: RealtimeQuerySortValue
}

export const DEFAULT_REALTIME_QUERY_SORT: RealtimeQuerySortValue = 'ts:desc'

export const REALTIME_QUERY_SORT_OPTIONS: RealtimeQuerySortOption[] = [
  { label: '时间倒序（新到旧）', value: 'ts:desc' },
  { label: '时间正序（旧到新）', value: 'ts:asc' },
  { label: 'TTFT 正序（低到高）', value: 'first_token_duration:asc' },
  { label: 'TTFT 逆序（高到低）', value: 'first_token_duration:desc' },
  { label: 'Token 输出速度正序（低到高）', value: 'token_output_speed:asc' },
  { label: 'Token 输出速度逆序（高到低）', value: 'token_output_speed:desc' },
  { label: 'Token E2E 输出速度正序（低到高）', value: 'token_output_speed_e2e:asc' },
  { label: 'Token E2E 输出速度逆序（高到低）', value: 'token_output_speed_e2e:desc' },
  { label: 'Prompt Tokens 正序（低到高）', value: 'prompt_tokens:asc' },
  { label: 'Prompt Tokens 逆序（高到低）', value: 'prompt_tokens:desc' },
  { label: 'Completion Tokens 正序（低到高）', value: 'completion_tokens:asc' },
  { label: 'Completion Tokens 逆序（高到低）', value: 'completion_tokens:desc' },
]

const validSortValues = new Set(REALTIME_QUERY_SORT_OPTIONS.map((option) => option.value))

export function parseRealtimeQuerySort(value: string): {
  sort_by: ChatDetailSortBy
  order: ChatSortOrder
} {
  const safeValue = validSortValues.has(value as RealtimeQuerySortValue)
    ? value as RealtimeQuerySortValue
    : DEFAULT_REALTIME_QUERY_SORT
  const [sortBy, order] = safeValue.split(':') as [ChatDetailSortBy, ChatSortOrder]
  return { sort_by: sortBy, order }
}
