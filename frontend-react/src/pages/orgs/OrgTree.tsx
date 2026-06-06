// 组织树页：左树来自 dept-sync 权威全量树（/v2/dept-tree），客户端懒渲染（只渲染展开分支，
// 5334 节点不一次性全渲染 DOM）；点部门 → 右侧 DeptMembersPanel 调 /v2/dept-tree/members 拿该部门
// 直属成员花名册（按 universal_id 左连看板 V2 指标，无看板数据的成员也列出）。
// 选中 dept_id + 日期同步 URL query。
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router'
import { useDeptTree } from '@/api/queries'
import type { DeptTreeNode } from '@/api/types'
import { getDefaultDateRangeWide } from '@/lib/date'
import { DateRangePicker } from '@/components/ui/DateRangePicker'
import { DeptMembersPanel } from './DeptMembersPanel'

function normalizeDateQuery(value: string | null): string {
  if (!value) return ''
  const s = String(value).trim()
  if (/^\d{8}$/.test(s)) return `${s.slice(0, 4)}-${s.slice(4, 6)}-${s.slice(6, 8)}`
  if (/^\d{4}-\d{2}-\d{2}$/.test(s)) return s
  return ''
}

// 取根 → 首条有子节点分支的所有 dept_id（默认展开第一条分支，方便一进页面就看到层级）。
function firstBranchIds(nodes: DeptTreeNode[]): string[] {
  const root = nodes.find((n) => n.children && n.children.length > 0) || nodes[0]
  const ids: string[] = []
  let cur: DeptTreeNode | undefined = root
  while (cur) {
    ids.push(cur.dept_id)
    cur = cur.children?.[0]
  }
  return ids
}

// 在树中按 dept_id 找节点（用于选中态标题展示）。
function findNodeById(nodes: DeptTreeNode[], deptId: string): DeptTreeNode | undefined {
  for (const n of nodes) {
    if (n.dept_id === deptId) return n
    if (n.children?.length) {
      const hit = findNodeById(n.children, deptId)
      if (hit) return hit
    }
  }
  return undefined
}

interface TreeNodeProps {
  node: DeptTreeNode
  depth: number
  selectedId: string
  expanded: Set<string>
  onToggle: (id: string) => void
  onSelect: (id: string) => void
}

function TreeNode({ node, depth, selectedId, expanded, onToggle, onSelect }: TreeNodeProps) {
  // dept-sync child_dept_count 标识有子部门；children 已嵌套在树里，懒渲染只看 isOpen。
  const hasChildren = (node.children && node.children.length > 0) || node.child_dept_count > 0
  const isOpen = expanded.has(node.dept_id)
  const isSelected = selectedId === node.dept_id

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
          {node.child_dept_count > 0 && (
            <span className="shrink-0 text-xs px-1.5 py-0.5 rounded-full bg-gray-100 dark:bg-white/10 text-gray-500 dark:text-gray-400 tabular-nums">
              {node.child_dept_count}
            </span>
          )}
        </button>
      </div>
      {/* 懒渲染：折叠节点不渲染子节点 DOM */}
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
}

export default function OrgTree() {
  const [searchParams, setSearchParams] = useSearchParams()
  const { data: tree, isLoading, error } = useDeptTree()

  const nodes: DeptTreeNode[] = useMemo(() => tree || [], [tree])

  const selectedId = searchParams.get('dept_id') || ''
  const selectedNode = useMemo(() => (selectedId ? findNodeById(nodes, selectedId) : undefined), [nodes, selectedId])

  const dateRange = useMemo<[string, string]>(() => {
    const start = normalizeDateQuery(searchParams.get('startDate'))
    const end = normalizeDateQuery(searchParams.get('endDate'))
    if (start && end) return [start, end]
    return getDefaultDateRangeWide()
  }, [searchParams])

  const [expanded, setExpanded] = useState<Set<string>>(new Set())

  // 默认展开第一条有子部门的分支（只在首次树到达时设一次）。
  useEffect(() => {
    if (!nodes.length) return
    setExpanded((prev) => {
      if (prev.size > 0) return prev
      return new Set<string>(firstBranchIds(nodes))
    })
  }, [nodes])

  const toggle = useCallback((id: string) => {
    setExpanded((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }, [])

  const select = useCallback(
    (id: string) => {
      const next = new URLSearchParams(searchParams)
      next.set('dept_id', id)
      setSearchParams(next, { replace: true })
    },
    [searchParams, setSearchParams],
  )

  const onDateChange = useCallback(
    (range: [string, string]) => {
      const next = new URLSearchParams(searchParams)
      next.set('startDate', range[0].replace(/-/g, ''))
      next.set('endDate', range[1].replace(/-/g, ''))
      setSearchParams(next, { replace: true })
    },
    [searchParams, setSearchParams],
  )

  return (
    <div className="space-y-5">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">组织</h1>
          <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
            左树为 dept-sync 权威部门树；点部门查看其直属成员花名册（按 universal_id 对到看板指标，无活动成员也列出）。
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <DateRangePicker value={dateRange} onChange={onDateChange} />
        </div>
      </header>

      <div className="grid grid-cols-1 lg:grid-cols-[20rem_1fr] gap-5 items-start">
        {/* 左：dept-sync 部门树（懒渲染） */}
        <aside className="glass rounded-2xl overflow-hidden lg:sticky lg:top-20">
          <div className="px-4 py-3 border-b border-gray-200/50 dark:border-white/10">
            <span className="text-sm font-semibold text-gray-700 dark:text-gray-200">部门导航</span>
          </div>
          <div className="p-2 max-h-[72vh] overflow-y-auto">
            {error ? (
              <div className="px-3 py-6 text-center text-sm text-rose-600 dark:text-rose-400">
                {(error as Error).message || '获取部门树失败'}
              </div>
            ) : isLoading ? (
              <div className="space-y-2 p-2">
                {Array.from({ length: 5 }).map((_, i) => (
                  <div key={i} className="skeleton h-7 rounded" />
                ))}
              </div>
            ) : !nodes.length ? (
              <div className="px-3 py-6 text-center text-sm text-gray-400 dark:text-gray-500">暂无部门数据</div>
            ) : (
              <ul role="tree" aria-label="dept-sync 部门树" className="list-none m-0 p-0">
                {nodes.map((n) => (
                  <TreeNode
                    key={n.dept_id}
                    node={n}
                    depth={0}
                    selectedId={selectedId}
                    expanded={expanded}
                    onToggle={toggle}
                    onSelect={select}
                  />
                ))}
              </ul>
            )}
          </div>
        </aside>

        {/* 右：部门成员（dept-sync 花名册 + 看板 V2 指标） */}
        <div className="min-w-0">
          <DeptMembersPanel deptId={selectedId} deptName={selectedNode?.dept_name || ''} dateRange={dateRange} />
        </div>
      </div>
    </div>
  )
}
