// 子部门对比（PK）视角：列出当前部门的直接子部门，横向对比关键使用指标。
// 对每个子部门复用 /stats/departments/:id/overview（与聚合视角 queryKey 一致 → 缓存共享）。
// include_children 控制每个子部门的统计是否含其更深层子部门（孙部门）。
// 点行 → onSelectDept(childId) 切到该子部门的聚合视图。
import { useMemo, useState } from 'react'
import { useQueries } from '@tanstack/react-query'
import type { DeptTreeNode } from '@/api/types'
import { ChartCard, EmptyHint, shortToken } from '@/pages/platform/platformShared'
import { SortableTh } from '@/components/ui/SortableTh'
import { formatNumber } from '@/lib/formatters'
import { chatGet } from '@/api/client'
import type { DeptOverviewResp } from './usageTypes'

const PCT = (v: number | null | undefined) => (v == null || !Number.isFinite(v) ? '-' : `${v.toFixed(1)}%`)
const TH = 'px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_NUM = 'px-3 py-2 text-right font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TD = 'px-3 py-2 align-middle text-gray-700 dark:text-gray-200 whitespace-nowrap'
const TD_NUM = 'px-3 py-2 text-right align-middle tabular-nums text-gray-700 dark:text-gray-200 whitespace-nowrap'

type SortKey = 'active_users' | 'total_requests' | 'sum_total_tokens' | 'success_rate' | 'error_rate' | 'total_sessions'

/** 递归找 deptId 的直接子部门。 */
function findChildren(nodes: DeptTreeNode[], deptId: string): DeptTreeNode[] {
  for (const n of nodes) {
    if (n.dept_id === deptId) return n.children || []
    if (n.children?.length) {
      const hit = findChildren(n.children, deptId)
      if (hit.length) return hit
    }
  }
  return []
}

export function DeptCompareView({
  deptId,
  start,
  end,
  includeChildren,
  deptNodes,
  onSelectDept,
}: {
  deptId: string
  start: string
  end: string
  includeChildren: boolean
  deptNodes: DeptTreeNode[]
  onSelectDept: (deptId: string) => void
}) {
  const [sortBy, setSortBy] = useState<SortKey>('total_requests')
  const [sortOrder, setSortOrder] = useState<'asc' | 'desc'>('desc')

  const children = useMemo(() => findChildren(deptNodes, deptId), [deptNodes, deptId])

  // 对每个子部门并发取 overview（queryKey 与 useUsageDeptOverview 一致 → 缓存共享）。
  const results = useQueries({
    queries: children.map((ch) => ({
      queryKey: ['usage-dept-overview', ch.dept_id, start, end, includeChildren],
      queryFn: () =>
        chatGet<DeptOverviewResp>(`/stats/departments/${encodeURIComponent(ch.dept_id)}/overview`, {
          start_date: start,
          end_date: end,
          include_children: includeChildren ? 'true' : 'false',
        }),
      enabled: !!ch.dept_id,
    })),
  })

  // 合并子部门 + overview，按 sortBy 排序。
  const rows = useMemo(() => {
    const merged = children.map((ch, i) => ({ dept: ch, ov: results[i].data as DeptOverviewResp | undefined }))
    const get = (ov: DeptOverviewResp | undefined, k: SortKey) => (ov ? Number(ov[k] ?? 0) : 0)
    merged.sort((a, b) => {
      const diff = get(a.ov, sortBy) - get(b.ov, sortBy)
      return sortOrder === 'desc' ? -diff : diff
    })
    return merged
  }, [children, results, sortBy, sortOrder])

  const loading = results.some((r) => r.isLoading)
  const handleSort = (field: string) => {
    const f = field as SortKey
    if (sortBy === f) setSortOrder((o) => (o === 'desc' ? 'asc' : 'desc'))
    else {
      setSortBy(f)
      setSortOrder('desc')
    }
  }

  if (!children.length) {
    return (
      <ChartCard title="子部门对比" sub="该部门下无子部门可对比">
        <EmptyHint compact />
      </ChartCard>
    )
  }

  const cols: { key: SortKey; label: string }[] = [
    { key: 'active_users', label: '活跃用户' },
    { key: 'total_requests', label: '总请求' },
    { key: 'sum_total_tokens', label: '总 Token' },
    { key: 'total_sessions', label: '会话数' },
    { key: 'success_rate', label: '成功率' },
    { key: 'error_rate', label: '失败率' },
  ]

  return (
    <ChartCard title="子部门对比（PK）" sub={`${children.length} 个子部门 · ${includeChildren ? '含各子部门下级（整棵子树）' : '仅各子部门直属'} · 点行下钻`}>
      <div className="overflow-x-auto">
        <table className="w-full text-sm border-collapse">
          <thead>
            <tr className="border-b border-gray-200/50 dark:border-white/10">
              <th className={TH_NUM}>#</th>
              <th className={TH}>子部门</th>
              {cols.map((c) => (
                <th key={c.key} className={TH_NUM}>
                  <SortableTh field={c.key} label={c.label} numeric active={sortBy === c.key} desc={sortOrder === 'desc'} onSort={handleSort} />
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {loading && rows.every((r) => !r.ov) ? (
              <tr>
                <td colSpan={8} className="py-10 text-center text-sm text-gray-400 dark:text-gray-500">加载中…</td>
              </tr>
            ) : (
              rows.map((r, i) => (
                <tr
                  key={r.dept.dept_id}
                  onClick={() => onSelectDept(r.dept.dept_id)}
                  className="border-b border-gray-100/50 dark:border-white/5 cursor-pointer hover:bg-apple-blue/5 dark:hover:bg-white/5 transition-colors"
                >
                  <td className={TD_NUM}>{i + 1}</td>
                  <td className={TD}>
                    <span className="max-w-[220px] truncate inline-block align-middle text-apple-blue hover:text-apple-blue-hover" title={r.dept.dept_name}>
                      {r.dept.dept_name}
                    </span>
                    {r.dept.child_dept_count > 0 && (
                      <span className="ml-2 text-xs px-1.5 py-0.5 rounded-full bg-gray-100 dark:bg-white/10 text-gray-500 dark:text-gray-400">{r.dept.child_dept_count}</span>
                    )}
                  </td>
                  <td className={TD_NUM}>{formatNumber(r.ov?.active_users)}</td>
                  <td className={TD_NUM}>{formatNumber(r.ov?.total_requests)}</td>
                  <td className={TD_NUM} title={formatNumber(r.ov?.sum_total_tokens)}>{shortToken(r.ov?.sum_total_tokens)}</td>
                  <td className={TD_NUM}>{formatNumber(r.ov?.total_sessions)}</td>
                  <td className={TD_NUM}>{PCT(r.ov?.success_rate)}</td>
                  <td className={TD_NUM}>{PCT(r.ov?.error_rate)}</td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </ChartCard>
  )
}
