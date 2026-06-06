// 用户 UUID → 真实用户名映射。
// 主：dept-sync 权威映射 /v2/user-names（user_id==universal_id → 真名+工号），展示「真名(工号)」。
// 兜底：commits 的 git_user_name（dept-sync 没覆盖到的、或无 universal_id 的用户）。
// 背景：needs 只有 primary_user_id=UUID；/v2/users 的 user_name 多为 UUID。内网看板 user_id 即 dept-sync
// universal_id（98.6% 命中），故优先用 dept-sync 权威真名，比 commit 反推更准更全。
import { useCallback } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getCommitsV2, getUserNamesV2 } from '@/api/endpoints'

/**
 * 返回 resolveName(userId)：把用户 UUID 解析为「真名(工号)」。
 * 空 userId → '-'；dept-sync 命中 → 真名(工号)；否则 commit 真名兜底；再否则回退原 userId。
 */
export function useUserNameMap() {
  // dept-sync 权威映射（主）
  const deptQuery = useQuery({
    queryKey: ['user-name-map-dept'],
    queryFn: () => getUserNamesV2(),
    staleTime: 5 * 60_000,
  })
  // commits 真名（兜底）
  const commitQuery = useQuery({
    queryKey: ['user-name-map-commit'],
    queryFn: () => getCommitsV2({ pageSize: 250 }),
    staleTime: 5 * 60_000,
  })

  // dept-sync：user_id → 「真名(工号)」（工号为空则只显真名）
  const deptMap: Record<string, string> = {}
  for (const u of deptQuery.data || []) {
    if (u.user_id && u.real_name && !deptMap[u.user_id]) {
      deptMap[u.user_id] = u.emp_no ? `${u.real_name}(${u.emp_no})` : u.real_name
    }
  }
  // commit 兜底：user_id → git_user_name > user_name
  const commitMap: Record<string, string> = {}
  for (const c of commitQuery.data?.data || []) {
    if (c.user_id && !commitMap[c.user_id]) {
      commitMap[c.user_id] = c.git_user_name || c.user_name || c.user_id
    }
  }

  const resolveName = useCallback(
    (userId?: string): string => {
      if (!userId) return '-'
      return deptMap[userId] ?? commitMap[userId] ?? userId
    },
    // map 每次渲染重建，内容由两个 query.data 决定，依赖它们即可
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [deptQuery.data, commitQuery.data],
  )

  return { resolveName, isLoading: deptQuery.isLoading || commitQuery.isLoading }
}
