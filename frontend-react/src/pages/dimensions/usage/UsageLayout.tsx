// 使用维度独立布局（路由层分叉）：usage 的 IA 已从「主体 Tab」改为「部门树·视角切换」，
// 不再走 4 维度共用的 DimensionEntityLayout。本组件仅做 entity 守卫，真内容在 UsageKanban。
// 守卫：project/repo 主体在 usage 维度已下线 + entity 脏值 → 重定向 /usage/org（保留 query）；
//       合法 entity（org/user）或裸 /usage → <Outlet/>。
import { Navigate, Outlet, useLocation, useParams } from 'react-router'
import { isEntity } from '@/components/layout/matrix'

export default function UsageLayout() {
  const { entity } = useParams<{ entity?: string }>()
  const { search } = useLocation()
  if (entity === 'project' || entity === 'repo' || (entity && !isEntity(entity))) {
    return <Navigate to={`/usage/org${search}`} replace />
  }
  return <Outlet />
}
