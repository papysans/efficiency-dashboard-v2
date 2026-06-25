// 使用看板左侧部门树：本服务 /v2/dept-tree（dept-sync 单根树），前端追加「未划分」虚拟顶级节点。
// 选中写 ?object=<dept_id>（由父组件 patch URL，连同 view=aggregate）。懒渲染（仅展开分支入 DOM）。
// unassigned dept_id 直接透传后端（后端已特判为「无部门用户」）。
import { memo, useCallback, useEffect, useMemo, useState } from 'react'
import { useDeptTree } from '@/api/queries'
import type { DeptTreeNode } from '@/api/types'

/** 虚拟部门：「未划分」（无部门归属的用户）。与后端 deptclient.UnassignedDeptID 同值。 */
export const UNASSIGNED_DEPT_ID = 'unassigned'

const UNASSIGNED_NODE: DeptTreeNode = {
  dept_id: UNASSIGNED_DEPT_ID,
  dept_name: '未划分',
  parent_dept_id: '',
  dept_path: '',
  dept_level: 0,
  order_num: 9999,
  child_dept_count: 0,
  status: 1,
  children: [],
}

// 默认展开：单根（公司根）→ 根 + 第一层；多根 → 所有顶层。与 OrgTree 同范式。
function initialExpandedIds(nodes: DeptTreeNode[]): string[] {
  const ids: string[] = []
  let firstLevel = nodes
  if (nodes.length === 1 && nodes[0].children?.length) {
    ids.push(nodes[0].dept_id)
    firstLevel = nodes[0].children
  }
  for (const n of firstLevel) ids.push(n.dept_id)
  return ids
}

interface TreeNodeProps {
  node: DeptTreeNode
  depth: number
  selectedId: string
  expanded: Set<string>
  onToggle: (id: string) => void
  onSelect: (id: string) => void
}

const TreeNode = memo(function TreeNode({ node, depth, selectedId, expanded, onToggle, onSelect }: TreeNodeProps) {
  const hasChildren = (node.children && node.children.length > 0) || node.child_dept_count > 0
  const isOpen = expanded.has(node.dept_id)
  const isSelected = selectedId === node.dept_id
  const isUnassigned = node.dept_id === UNASSIGNED_DEPT_ID
  return (
    <li role="treeitem" aria-expanded={hasChildren ? isOpen : undefined} aria-selected={isSelected}>
      <div
        className={`flex items-center gap-1 rounded-lg pr-2 py-1.5 cursor-pointer transition-colors ${
          isSelected
            ? 'bg-apple-blue/15 text-apple-blue'
            : 'text-gray-700 dark:text-gray-200 hover:bg-white/50 dark:hover:bg-white/10'
        }`}
        style={{ paddingLeft: `${depth * 16 + 8}px` }}
      >
        {hasChildren ? (
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation()
              onToggle(node.dept_id)
            }}
            aria-label={isOpen ? '收起' : '展开'}
            className="shrink-0 w-5 h-5 inline-flex items-center justify-center rounded text-gray-400 hover:text-apple-blue bg-transparent border-none cursor-pointer focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue"
          >
            <svg className={`w-3.5 h-3.5 transition-transform ${isOpen ? 'rotate-90' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
            </svg>
          </button>
        ) : (
          <span className="shrink-0 w-5 h-5" aria-hidden="true" />
        )}
        <button
          type="button"
          onClick={() => onSelect(node.dept_id)}
          className="flex-1 min-w-0 inline-flex items-center justify-between gap-2 text-left text-sm bg-transparent border-none p-0 cursor-pointer text-inherit focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue rounded"
        >
          <span className="truncate" title={node.dept_name}>{node.dept_name}</span>
          <span className="shrink-0 flex items-center gap-1.5">
            {isUnassigned && (
              <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-amber-100 dark:bg-amber-500/20 text-amber-700 dark:text-amber-300" title="无部门归属用户的虚拟部门">
                虚拟
              </span>
            )}
            {node.child_dept_count > 0 && (
              <span className="text-xs px-1.5 py-0.5 rounded-full bg-gray-100 dark:bg-white/10 text-gray-500 dark:text-gray-400 tabular-nums">
                {node.child_dept_count}
              </span>
            )}
          </span>
        </button>
      </div>
      {hasChildren && isOpen && node.children?.length ? (
        <ul role="group" className="list-none m-0 p-0">
          {node.children.map((ch) => (
            <TreeNode
              key={ch.dept_id}
              node={ch}
              depth={depth + 1}
              selectedId={selectedId}
              expanded={expanded}
              onToggle={onToggle}
              onSelect={onSelect}
            />
          ))}
        </ul>
      ) : null}
    </li>
  )
})

export function DeptTreePanel({
  selectedId,
  onSelect,
}: {
  selectedId: string
  onSelect: (deptId: string) => void
}) {
  const { data, isLoading, error } = useDeptTree()
  // 追加「未划分」虚拟顶级节点（与公司根并列）。
  const nodes = useMemo<DeptTreeNode[]>(() => [...(data || []), UNASSIGNED_NODE], [data])

  const [expanded, setExpanded] = useState<Set<string>>(new Set())
  useEffect(() => {
    if (!nodes.length) return
    setExpanded((prev) => (prev.size > 0 ? prev : new Set(initialExpandedIds(nodes))))
  }, [nodes])

  const toggle = useCallback((id: string) => {
    setExpanded((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }, [])

  return (
    <aside className="glass rounded-2xl overflow-hidden lg:sticky lg:top-20">
      <div className="px-4 py-3 border-b border-gray-200/50 dark:border-white/10 flex items-center justify-between">
        <span className="text-sm font-semibold text-gray-700 dark:text-gray-200">部门导航</span>
        <span className="text-xs text-gray-400 dark:text-gray-500">点部门看其使用指标</span>
      </div>
      <div className="p-2 max-h-[72vh] overflow-y-auto">
        {error ? (
          <div className="px-3 py-6 text-center text-sm text-rose-600 dark:text-rose-400">
            {(error as Error).message || '获取部门树失败'}
          </div>
        ) : isLoading ? (
          <div className="space-y-2 p-2">
            {Array.from({ length: 6 }).map((_, i) => (
              <div key={i} className="skeleton h-7 rounded" />
            ))}
          </div>
        ) : !nodes.length ? (
          <div className="px-3 py-6 text-center text-sm text-gray-400 dark:text-gray-500">暂无部门数据</div>
        ) : (
          <ul role="tree" aria-label="部门树" className="list-none m-0 p-0">
            {nodes.map((n) => (
              <TreeNode
                key={n.dept_id}
                node={n}
                depth={0}
                selectedId={selectedId}
                expanded={expanded}
                onToggle={toggle}
                onSelect={onSelect}
              />
            ))}
          </ul>
        )}
      </div>
    </aside>
  )
}
