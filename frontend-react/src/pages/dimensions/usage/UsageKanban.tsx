// 使用看板统一页：左部门树（含「未划分」虚拟部门）+ 右主区域（视角切换 + include_children + 内容）。
// 视角（?view=）：aggregate=部门聚合 / members=本部门人员列表 / member=个人详情。
// URL query 单一数据源：object=当前部门 dept_id（始终，member 视角也保持，部门树高亮不变）；
//   member=个人 universal_id（仅 member 视角）；include_children=包含子部门开关。
// 时间范围用全局 useViewState()（AppShell 顶部 DateRangePicker）。状态全走 URL query（深链可还原）。
import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { useSearchParams } from 'react-router'
import { useDeptTree, useGlobalConfig } from '@/api/queries'
import { useViewState } from '@/store/viewState'
import type { DeptTreeNode } from '@/api/types'
import { DimSkeleton, PlatformNotConnected } from '../platformDimShared'
import { DeptTreePanel, UNASSIGNED_DEPT_ID } from './DeptTreePanel'
import { DeptAggregateView } from './DeptAggregateView'
import { MembersView } from './MembersView'
import { DeptCompareView } from './DeptCompareView'
import { UserDetailModal } from './UserDetailModal'

type View = 'aggregate' | 'members' | 'compare'

function findDeptName(nodes: DeptTreeNode[], id: string): string {
  for (const n of nodes) {
    if (n.dept_id === id) return n.dept_name
    if (n.children?.length) {
      const hit = findDeptName(n.children, id)
      if (hit) return hit
    }
  }
  return ''
}

export default function UsageKanban() {
  const [sp, setSp] = useSearchParams()
  const { timeRange } = useViewState()
  const { data: gc, isLoading: gcLoading } = useGlobalConfig()
  const chatEnabled = gc?.chat_stats_enabled === true

  const deptId = sp.get('object') || ''
  const viewRaw = sp.get('view')
  const view: View = viewRaw === 'members' ? 'members' : viewRaw === 'compare' ? 'compare' : 'aggregate'
  const includeChildren = sp.get('include_children') !== 'false' // 默认勾选子部门

  const [selectedUser, setSelectedUser] = useState<{ uid: string; username: string } | null>(null)

  const deptQ = useDeptTree()
  const deptNodes = useMemo(() => deptQ.data || [], [deptQ.data])
  const rootDeptId = deptNodes[0]?.dept_id || ''

  // 默认落位：未选部门时自动选根部门（公司整体视角）。
  useEffect(() => {
    if (!chatEnabled || !rootDeptId || deptId) return
    const next = new URLSearchParams(sp)
    next.set('object', rootDeptId)
    next.set('view', 'aggregate')
    setSp(next, { replace: true })
  }, [chatEnabled, rootDeptId, deptId, sp, setSp])

  const patch = (p: Record<string, string | null>) => {
    const next = new URLSearchParams(sp)
    Object.entries(p).forEach(([k, v]) => (v == null ? next.delete(k) : next.set(k, v)))
    setSp(next, { replace: false })
  }

  if (gcLoading) return <DimSkeleton />
  if (!chatEnabled) return <PlatformNotConnected reason="disabled" />

  const deptName = deptId === UNASSIGNED_DEPT_ID ? '未划分' : findDeptName(deptNodes, deptId)

  return (
    <div className="flex flex-col gap-5">
      <div className="min-w-0">
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white">使用</h1>
        <nav aria-label="面包屑" className="mt-1 text-xs text-gray-400 dark:text-gray-500 flex items-center gap-1.5 select-none">
          <span>使用</span>
          {deptName && (
            <>
              <span aria-hidden="true">›</span>
              <button
                type="button"
                onClick={() => patch({ view: 'aggregate' })}
                className="bg-transparent border-none p-0 cursor-pointer hover:text-apple-blue focus:outline-none focus-visible:underline"
              >
                {deptName}
              </button>
            </>
          )}
        </nav>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-[18rem_1fr] gap-5 items-start">
        <DeptTreePanel selectedId={deptId} onSelect={(id) => patch({ object: id, view: 'aggregate' })} />

        <div className="min-w-0 flex flex-col gap-4">
          {/* 视角切换 + include_children 开关 */}
          <div className="glass rounded-2xl px-4 py-2.5 flex flex-wrap items-center justify-between gap-3">
            <div className="flex items-center gap-1" role="tablist" aria-label="视角">
              <ViewTab active={view === 'aggregate'} onClick={() => patch({ view: 'aggregate' })}>
                部门聚合
              </ViewTab>
              <ViewTab active={view === 'compare'} onClick={() => patch({ view: 'compare' })}>
                子部门对比
              </ViewTab>
              <ViewTab active={view === 'members'} onClick={() => patch({ view: 'members' })}>
                本部门人员
              </ViewTab>
            </div>
            <label className="inline-flex items-center gap-2 text-sm text-gray-600 dark:text-gray-300 cursor-pointer select-none">
              <button
                type="button"
                role="switch"
                aria-checked={includeChildren}
                aria-label="包含子部门"
                onClick={() => patch({ include_children: includeChildren ? 'false' : null })}
                className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue ${
                  includeChildren ? 'bg-apple-blue' : 'bg-gray-300 dark:bg-gray-600'
                }`}
              >
                <span
                  className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
                    includeChildren ? 'translate-x-6' : 'translate-x-1'
                  }`}
                />
              </button>
              包含子部门
            </label>
          </div>

          {/* 内容区：按视角分发 */}
          {view === 'members' ? (
            <MembersView
              deptId={deptId}
              start={timeRange[0]}
              end={timeRange[1]}
              includeChildren={includeChildren}
              onRowClick={(uid) => setSelectedUser({ uid, username: '' })}
            />
          ) : view === 'compare' ? (
            <DeptCompareView
              deptId={deptId}
              start={timeRange[0]}
              end={timeRange[1]}
              includeChildren={includeChildren}
              deptNodes={deptNodes}
              onSelectDept={(id) => patch({ object: id, view: 'aggregate' })}
            />
          ) : (
            <DeptAggregateView deptId={deptId} start={timeRange[0]} end={timeRange[1]} includeChildren={includeChildren} />
          )}
        </div>
        </div>

        {/* 用户详情弹窗 */}
        {selectedUser && (
          <UserDetailModal
            uid={selectedUser.uid}
            username={selectedUser.username}
            start={timeRange[0]}
            end={timeRange[1]}
            onClose={() => setSelectedUser(null)}
          />
        )}
      </div>
    )
}

function ViewTab({ active, onClick, children }: { active: boolean; onClick: () => void; children: ReactNode }) {
  return (
    <button
      type="button"
      role="tab"
      aria-selected={active}
      onClick={onClick}
      className={`px-3 py-1.5 rounded-lg text-sm font-medium no-underline transition-colors cursor-pointer border-none focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue ${
        active
          ? 'bg-apple-blue text-white'
          : 'bg-transparent text-gray-600 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white hover:bg-white/50 dark:hover:bg-white/10'
      }`}
    >
      {children}
    </button>
  )
}
