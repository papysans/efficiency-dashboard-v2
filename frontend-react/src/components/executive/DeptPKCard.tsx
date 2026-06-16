import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router'
import { useDeptTree, useDeptRanking } from '@/api/queries'
import { RatioPill } from '@/components/ui/RatioPill'
import { formatNumber } from '@/lib/formatters'
import { glossaryTip } from '@/lib/glossary'
import type { DeptTreeNode } from '@/api/types'

// 部门 PK Top5（Q3：只在首页放 Top5；Q1：前端可切层级）。
// 数据源：dept-tree(树，仅出层级下拉) + dept-tree/ranking?parent_dept_id=（一次聚合返回各直接子部门含子树的提效/成本/AI汇总，小数口径）。
// 默认排"全公司一级部门"（parent_dept_id 传空，交后端取配置根）；下拉可切到任意有子部门的父节点，排其直接子部门。
// ⚠️ ranking 接口代理远程 dept-sync，本地 eval 环境不可达 → 优雅降级为"暂不可用"。
// 性能：替代旧 useQueries 的 N× members 全表聚合，改为单次 ranking 端点（后端只跑一次 aggregateUsersV2）。

interface DeptPKCardProps {
  startDate: string
  endDate: string
}

const ROOT = '__root__'
const RANK_BADGE = [
  'bg-amber-400 text-white',
  'bg-gray-300 text-gray-700 dark:bg-gray-400 dark:text-gray-900',
  'bg-orange-300 text-white dark:bg-orange-400',
]
const RANK_DEFAULT = 'bg-gray-100 text-gray-500 dark:bg-gray-700 dark:text-gray-300'

/** 收集所有"有子部门"的节点，做层级下拉选项（缩进体现层级）。 */
function collectParents(nodes: DeptTreeNode[], depth = 0, acc: { id: string; label: string }[] = []) {
  for (const n of nodes) {
    if (n.children && n.children.length > 0) {
      acc.push({ id: n.dept_id, label: `${'　'.repeat(depth)}${n.dept_name}` })
      collectParents(n.children, depth + 1, acc)
    }
  }
  return acc
}

export function DeptPKCard({ startDate, endDate }: DeptPKCardProps) {
  const navigate = useNavigate()
  const treeQ = useDeptTree()
  const tree = treeQ.data ?? []
  const [parentId, setParentId] = useState<string>(ROOT)

  const parentOptions = useMemo(() => collectParents(tree), [tree])

  // 一次聚合：ROOT → parent_dept_id 传空（后端默认取配置根，排「全公司一级部门」）；否则传选中父节点 id。
  const rankingQ = useDeptRanking({
    parentDeptId: parentId === ROOT ? undefined : parentId,
    startDate,
    endDate,
  })

  // 按 calendar_ratio 降序（null 沉底）取 Top5。
  const top5 = useMemo(() => {
    const rows = (rankingQ.data?.items ?? []).filter((it) => it.summary?.calendar_ratio != null)
    rows.sort((a, b) => (b.summary.calendar_ratio ?? -Infinity) - (a.summary.calendar_ratio ?? -Infinity))
    return rows.slice(0, 5)
  }, [rankingQ.data])

  const loading = treeQ.isLoading || rankingQ.isLoading
  const errored = treeQ.isError || rankingQ.isError

  return (
    <div className="glass rounded-2xl p-5 md:p-6 hover:shadow-lg transition-shadow flex flex-col">
      <div className="flex items-center justify-between mb-4 gap-3">
        <div className="flex items-center gap-1 min-w-0">
          <h2 className="text-sm font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide truncate">部门 PK</h2>
          <span className="text-gray-400 cursor-help inline-flex" title={glossaryTip('dept_efficiency')} aria-label={glossaryTip('dept_efficiency')}>
            <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
          </span>
        </div>
        <select
          value={parentId}
          onChange={(e) => setParentId(e.target.value)}
          aria-label="选择部门层级"
          className="shrink-0 text-xs rounded-lg bg-gray-100/70 dark:bg-white/5 text-gray-700 dark:text-gray-200 px-2 py-1 border-none cursor-pointer max-w-[10rem] truncate"
        >
          <option value={ROOT}>全公司（一级部门）</option>
          {parentOptions.map((o) => (
            <option key={o.id} value={o.id}>{o.label}</option>
          ))}
        </select>
      </div>

      {errored ? (
        <div className="flex-1 flex items-center justify-center text-center text-sm text-gray-500 dark:text-gray-400 min-h-[14rem] px-4">
          部门数据暂不可用（需 dept-sync 服务连通）
        </div>
      ) : loading ? (
        <ul className="flex-1 space-y-2">
          {Array.from({ length: 5 }).map((_, i) => (
            <li key={i} className="skeleton h-11 rounded-xl" />
          ))}
        </ul>
      ) : top5.length === 0 ? (
        <div className="flex-1 flex items-center justify-center text-sm text-gray-500 dark:text-gray-400 min-h-[14rem]">
          该层级暂无可计入部门数据
        </div>
      ) : (
        <ul className="flex-1 space-y-2">
          {top5.map((r, i) => {
            const badge = i < 3 ? RANK_BADGE[i] : RANK_DEFAULT
            const sum = r.summary
            return (
              <li
                key={r.dept_id}
                onClick={() => navigate('/org-tree-v2')}
                role="button"
                tabIndex={0}
                onKeyDown={(e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), navigate('/org-tree-v2'))}
                aria-label={`${r.dept_name}，点击查看部门`}
                className="flex items-center gap-3 rounded-xl px-2 py-1.5 cursor-pointer hover:bg-white/40 dark:hover:bg-white/5 transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-sky-400"
              >
                <span className={`shrink-0 w-6 h-6 rounded-full flex items-center justify-center text-xs font-bold tabular-nums ${badge}`}>
                  {i + 1}
                </span>
                <div className="min-w-0 flex-1">
                  <div className="text-sm font-medium text-gray-900 dark:text-white truncate" title={r.dept_name}>
                    {r.dept_name}
                  </div>
                  <div className="text-xs text-gray-400 dark:text-gray-500 truncate">
                    {formatNumber(sum.kanban_member_count)} 人 · 需求 {formatNumber(sum.merged_need_count)}
                  </div>
                </div>
                <span className="shrink-0">
                  <RatioPill value={sum.calendar_ratio} />
                </span>
              </li>
            )
          })}
        </ul>
      )}
    </div>
  )
}
