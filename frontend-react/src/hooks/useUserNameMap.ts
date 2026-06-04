// 用户 UUID → 真实用户名映射。
// 背景：needs 列表只有 primary_user_id=UUID；/v2/users 的 user_name 反而多为 UUID（没用）。
// commits 每行带 user_id + git_user_name（最真实，如 IronRookieCoder/林凯90331）+ user_name（多数真实，偶尔 UUID）。
// 故从 commits 建映射：git_user_name 优先 > user_name 次选 > 回退 user_id。
// 当前数据集 136 commits / 9 users，pageSize:250 一页覆盖。
import { useCallback } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getCommitsV2 } from '@/api/endpoints'

/**
 * 返回 resolveName(userId)：把用户 UUID 解析为真实用户名。
 * 空 userId → '-'；未命中（映射还没建好或该 id 无 commit）→ 回退原 userId。
 */
export function useUserNameMap() {
  const query = useQuery({
    queryKey: ['user-name-map'],
    queryFn: () => getCommitsV2({ pageSize: 250 }),
    staleTime: 5 * 60_000,
  })

  const map: Record<string, string> = {}
  for (const c of query.data?.data || []) {
    if (c.user_id && !map[c.user_id]) {
      map[c.user_id] = c.git_user_name || c.user_name || c.user_id
    }
  }

  const resolveName = useCallback(
    (userId?: string): string => {
      if (!userId) return '-'
      return map[userId] ?? userId
    },
    // map 每次渲染重建，但内容由 query.data 决定，依赖它即可
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [query.data],
  )

  return { resolveName, isLoading: query.isLoading }
}
