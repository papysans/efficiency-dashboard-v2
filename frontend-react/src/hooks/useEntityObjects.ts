// 对象选择器数据源 —— 按主体复用现有 list query，返回可搜索的 {value,label} 列表。
// org=部门(dept-sync 全量树扁平化) / user=用户(真名映射) / project=项目 / repo=仓库(取首选分支)。
// 「全部」由选择器自身给空 value 表示（聚合态）；选某项 value=该对象 id（聚焦态）。
import { useMemo } from 'react'
import { useDeptTree, useProjectList } from '@/api/queries'
import { useUserNameMap } from '@/hooks/useUserNameMap'
import { useViewState } from '@/store/viewState'
import { getAllUsersV2, getAllReposV2 } from '@/api/endpoints'
import { useQuery } from '@tanstack/react-query'
import { formatDateParam } from '@/lib/date'
import type { DeptTreeNode } from '@/api/types'
import type { Entity } from '@/components/layout/matrix'

export interface EntityOption {
  value: string
  label: string
}

/** 扁平化 dept-sync 树为 {dept_id -> 全路径名}，缩进体现层级。 */
function flattenDeptTree(nodes: DeptTreeNode[], depth = 0, out: EntityOption[] = []): EntityOption[] {
  for (const n of nodes) {
    out.push({ value: n.dept_id, label: `${'　'.repeat(depth)}${n.dept_name}` })
    if (n.children?.length) flattenDeptTree(n.children, depth + 1, out)
  }
  return out
}

/**
 * 返回某主体在当前全局时间范围下的可选对象列表（含 loading）。
 * 选择器组件据此渲染可搜索下拉；不同主体的 query 都按需启用（enabled），切主体不浪费请求。
 */
export function useEntityObjects(entity: Entity): { options: EntityOption[]; loading: boolean } {
  const { timeRange } = useViewState()
  const [start, end] = timeRange
  const dateParams = useMemo(
    () => ({ startDate: formatDateParam(start), endDate: formatDateParam(end) }),
    [start, end],
  )
  const { resolveName } = useUserNameMap()

  // org：dept-sync 全量树（与日期无关，长缓存），扁平化。
  const deptQ = useDeptTree()
  const orgOptions = useMemo<EntityOption[]>(
    () => (entity === 'org' ? flattenDeptTree(deptQ.data || []) : []),
    [entity, deptQ.data],
  )

  // user：一次拉全用户（沿用 getAllUsersV2 翻页拉全），真名映射显示。
  const userQ = useQuery({
    queryKey: ['entity-objects-users', dateParams],
    queryFn: () => getAllUsersV2(dateParams),
    enabled: entity === 'user',
    staleTime: 60_000,
  })
  const userOptions = useMemo<EntityOption[]>(() => {
    if (entity !== 'user') return []
    return (userQ.data || [])
      .filter((u) => u.user_id)
      .map((u) => ({ value: u.user_id, label: resolveName(u.user_id) }))
  }, [entity, userQ.data, resolveName])

  // project：列表无分页，全量返回。
  const projectQ = useProjectList()
  const projectOptions = useMemo<EntityOption[]>(() => {
    if (entity !== 'project') return []
    return (projectQ.data?.data || [])
      .filter((p) => p.project_id)
      .map((p) => ({ value: p.project_id, label: p.name || p.project_id }))
  }, [entity, projectQ.data])

  // repo：服务端分页会在仓库 >1000 时截断（修 #6），故翻页拉全后取仓库地址。
  const repoQ = useQuery({
    queryKey: ['entity-objects-repos', dateParams],
    queryFn: () => getAllReposV2(dateParams),
    enabled: entity === 'repo',
    staleTime: 60_000,
  })
  const repoOptions = useMemo<EntityOption[]>(() => {
    if (entity !== 'repo') return []
    return (repoQ.data || [])
      .filter((r) => r.repo_addr)
      .map((r) => ({ value: r.repo_addr, label: r.repo_addr }))
  }, [entity, repoQ.data])

  switch (entity) {
    case 'org':
      return { options: orgOptions, loading: deptQ.isLoading }
    case 'user':
      return { options: userOptions, loading: userQ.isLoading }
    case 'project':
      return { options: projectOptions, loading: projectQ.isLoading }
    case 'repo':
      return { options: repoOptions, loading: repoQ.isLoading }
    default:
      return { options: [], loading: false }
  }
}
