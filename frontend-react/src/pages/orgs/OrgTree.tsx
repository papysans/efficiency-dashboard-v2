// 组织树页：左侧部门树导航 + 右侧组织详情。
// 数据源 = /v2/orgs/tree（基于 user_org 有数据的 org1..org9 层级）；
// 右栏用 OrgTreeDetailPanel（V2 口径，调 /v2/orgs/tree-detail → aggregateUsersV2，成员/指标真实可用）。
// 选中 org_path + 日期同步到 URL query。
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router'
import { useOrgTree } from '@/api/queries'
import type { OrgTreeNode } from '@/api/types'
import { getDefaultDateRangeWide } from '@/lib/date'
import { DateRangePicker } from '@/components/ui/DateRangePicker'
import { OrgTreeDetailPanel } from './OrgTreeDetailPanel'

const GRANULARITY: Array<{ label: string; value: string }> = [
  { label: '天', value: 'day' },
  { label: '周', value: 'week' },
  { label: '月', value: 'month' },
  { label: '年', value: 'year' },
]

function normalizeDateQuery(value: string | null): string {
  if (!value) return ''
  const s = String(value).trim()
  if (/^\d{8}$/.test(s)) return `${s.slice(0, 4)}-${s.slice(4, 6)}-${s.slice(6, 8)}`
  if (/^\d{4}-\d{2}-\d{2}$/.test(s)) return s
  return ''
}

// 取首条「有子节点的分支」上从根到叶的所有 org_path（用于默认展开）。
// 顶层第一个节点可能是无子节点的扁平组织（如 cleaned-import），故选首个有 children 的根。
function firstBranchPaths(nodes: OrgTreeNode[]): string[] {
  const root = nodes.find((n) => n.children && n.children.length > 0) || nodes[0]
  const paths: string[] = []
  let cur: OrgTreeNode | undefined = root
  while (cur) {
    paths.push(cur.org_path)
    cur = cur.children?.[0]
  }
  return paths
}

interface TreeNodeProps {
  node: OrgTreeNode
  depth: number
  selectedPath: string
  expanded: Set<string>
  onToggle: (path: string) => void
  onSelect: (path: string) => void
}

function TreeNode({ node, depth, selectedPath, expanded, onToggle, onSelect }: TreeNodeProps) {
  const hasChildren = node.children && node.children.length > 0
  const isOpen = expanded.has(node.org_path)
  const isSelected = selectedPath === node.org_path

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
              onToggle(node.org_path)
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
          onClick={() => onSelect(node.org_path)}
          className="flex-1 min-w-0 inline-flex items-center justify-between gap-2 text-left text-sm bg-transparent border-none p-0 cursor-pointer text-inherit focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue rounded"
        >
          <span className="truncate" title={node.name}>{node.name}</span>
          <span className="shrink-0 text-xs px-1.5 py-0.5 rounded-full bg-gray-100 dark:bg-white/10 text-gray-500 dark:text-gray-400 tabular-nums">
            {node.user_count}
          </span>
        </button>
      </div>
      {hasChildren && isOpen && (
        <ul role="group" className="list-none m-0 p-0">
          {node.children.map((ch) => (
            <TreeNode
              key={ch.org_path}
              node={ch}
              depth={depth + 1}
              selectedPath={selectedPath}
              expanded={expanded}
              onToggle={onToggle}
              onSelect={onSelect}
            />
          ))}
        </ul>
      )}
    </li>
  )
}

export default function OrgTree() {
  const [searchParams, setSearchParams] = useSearchParams()
  const { data: tree, isLoading, error } = useOrgTree()

  const nodes: OrgTreeNode[] = useMemo(() => tree || [], [tree])

  const selectedPath = searchParams.get('org_path') || ''
  const granularity = searchParams.get('granularity') || 'day'

  const dateRange = useMemo<[string, string]>(() => {
    const start = normalizeDateQuery(searchParams.get('startDate'))
    const end = normalizeDateQuery(searchParams.get('endDate'))
    if (start && end) return [start, end]
    return getDefaultDateRangeWide()
  }, [searchParams])

  const [expanded, setExpanded] = useState<Set<string>>(new Set())

  // 默认展开第一条有数据路径；若 URL 已带 org_path，展开其祖先链。
  useEffect(() => {
    if (!nodes.length) return
    setExpanded((prev) => {
      if (prev.size > 0) return prev
      const next = new Set<string>(firstBranchPaths(nodes))
      if (selectedPath) {
        const segs = selectedPath.split('/').filter(Boolean)
        const acc: string[] = []
        for (const s of segs) {
          acc.push(s)
          next.add(acc.join('/'))
        }
      }
      return next
    })
  }, [nodes, selectedPath])

  const toggle = useCallback((path: string) => {
    setExpanded((prev) => {
      const next = new Set(prev)
      if (next.has(path)) next.delete(path)
      else next.add(path)
      return next
    })
  }, [])

  const select = useCallback(
    (path: string) => {
      const next = new URLSearchParams(searchParams)
      next.set('org_path', path)
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

  const onGranularityChange = useCallback(
    (val: string) => {
      const next = new URLSearchParams(searchParams)
      next.set('granularity', val)
      setSearchParams(next, { replace: true })
    },
    [searchParams, setSearchParams],
  )

  const selectCls =
    'glass rounded-lg px-2 py-1.5 text-sm bg-transparent cursor-pointer text-gray-700 dark:text-gray-200 ' +
    'focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue min-w-[80px]'

  return (
    <div className="space-y-5">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">组织树</h1>
          <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
            按真实部门层级下钻；左树选择组织，右侧查看该组织详情（提效比为百分比口径）。
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <DateRangePicker value={dateRange} onChange={onDateChange} />
          <select value={granularity} onChange={(e) => onGranularityChange(e.target.value)} className={selectCls} aria-label="粒度">
            {GRANULARITY.map((g) => (
              <option key={g.value} value={g.value}>{g.label}</option>
            ))}
          </select>
        </div>
      </header>

      <div className="grid grid-cols-1 lg:grid-cols-[18rem_1fr] gap-5 items-start">
        {/* 左：部门树 */}
        <aside className="glass rounded-2xl overflow-hidden lg:sticky lg:top-20">
          <div className="px-4 py-3 border-b border-gray-200/50 dark:border-white/10">
            <span className="text-sm font-semibold text-gray-700 dark:text-gray-200">部门导航</span>
          </div>
          <div className="p-2 max-h-[70vh] overflow-y-auto">
            {error ? (
              <div className="px-3 py-6 text-center text-sm text-rose-600 dark:text-rose-400">
                {(error as Error).message || '获取组织树失败'}
              </div>
            ) : isLoading ? (
              <div className="space-y-2 p-2">
                {Array.from({ length: 5 }).map((_, i) => (
                  <div key={i} className="skeleton h-7 rounded" />
                ))}
              </div>
            ) : !nodes.length ? (
              <div className="px-3 py-6 text-center text-sm text-gray-400 dark:text-gray-500">暂无组织数据</div>
            ) : (
              <ul role="tree" aria-label="组织部门树" className="list-none m-0 p-0">
                {nodes.map((n) => (
                  <TreeNode
                    key={n.org_path}
                    node={n}
                    depth={0}
                    selectedPath={selectedPath}
                    expanded={expanded}
                    onToggle={toggle}
                    onSelect={select}
                  />
                ))}
              </ul>
            )}
          </div>
        </aside>

        {/* 右：组织详情（V2 口径，OrgTreeDetailPanel） */}
        <div className="min-w-0">
          <OrgTreeDetailPanel orgPath={selectedPath} dateRange={dateRange} />
        </div>
      </div>
    </div>
  )
}
