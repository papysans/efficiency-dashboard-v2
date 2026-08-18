// 用户 UUID → 真实用户名映射。
// 单一权威源：后端 /v2/user-names（user_id==universal_id → 真名+工号），展示「真名(工号)」。
// 该接口后端已做全量收敛：dept-sync 优先，未覆盖的用户用 commits 的 git_user_name 兜底，
// 永不受分页截断影响。前端因此不再单独拉 commit 明细建映射（旧 pageSize:250 兜底已下线，
// commits 涨到 2000+ 后会截断 76% 用户，导致贡献者显示 UUID）。
import { useCallback } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getUserNamesV2 } from '@/api/endpoints'

/**
 * 返回 resolveName(userId)：把用户 UUID 解析为「真名(工号)」。
 * 空 userId → '-'；命中映射 → 真名(工号)/真名；否则回退原 userId。
 * options.enabled=false → 懒查询（不发请求，resolveName 原样回退 userId），
 * 供「默认不展示人名、切到人维度才需要」的场景（如总览 Top 榜的人 tab）。
 */
export function useUserNameMap(options?: { enabled?: boolean }) {
  const nameQuery = useQuery({
    queryKey: ['user-name-map'],
    queryFn: () => getUserNamesV2(),
    staleTime: 5 * 60_000,
    enabled: options?.enabled ?? true,
  })

  // user_id → 「真名(工号)」（工号为空则只显真名）
  const nameMap: Record<string, string> = {}
  for (const u of nameQuery.data || []) {
    if (u.user_id && u.real_name && !nameMap[u.user_id]) {
      nameMap[u.user_id] = u.emp_no ? `${u.real_name}(${u.emp_no})` : u.real_name
    }
  }

  const resolveName = useCallback(
    (userId?: string): string => {
      if (!userId) return '-'
      return nameMap[userId] ?? userId
    },
    // map 每次渲染重建，内容由 query.data 决定，依赖它即可
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [nameQuery.data],
  )

  return { resolveName, isLoading: nameQuery.isLoading }
}
