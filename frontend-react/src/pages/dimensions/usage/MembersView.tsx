// 本部门人员列表视角：对接 /stats/departments/:dept_id/members（分页/排序/搜索）。
// 点行 → onRowClick(universal_id)（由父组件写 ?member=<uid>&view=member 进个人详情）。
// 复用 Pagination / SortableTh / ChatUserCell。搜索 400ms debounce + trim（CLAUDE.md 输入规范）。
import { useEffect, useState } from 'react'
import { ChartCard, ChatUserCell, EmptyHint, shortToken } from '@/pages/platform/platformShared'
import { Pagination } from '@/components/ui/Pagination'
import { SortableTh } from '@/components/ui/SortableTh'
import { useUserNameMap } from '@/hooks/useUserNameMap'
import { fmtCost, formatNumber } from '@/lib/formatters'
import { useUsageDeptMembers, type MemberSortBy } from './usageData'

const TH_NUM = 'px-3 py-2 text-right font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TD_NUM = 'px-3 py-2 text-right align-middle tabular-nums text-gray-700 dark:text-gray-200 whitespace-nowrap'
const TD = 'px-3 py-2 align-middle text-gray-700 dark:text-gray-200 whitespace-nowrap'
const INPUT_CLS =
  'glass rounded-lg px-3 py-1.5 text-sm bg-transparent text-gray-900 dark:text-white placeholder-gray-400 ' +
  'focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue'

const SORT_COLS: { field: MemberSortBy; label: string }[] = [
  { field: 'total_requests', label: '请求数' },
  { field: 'sum_total_tokens', label: '总 Token' },
  { field: 'success_rate', label: '成功率' },
  { field: 'active_days', label: '活跃天数' },
]

export function MembersView({
  deptId,
  start,
  end,
  includeChildren,
  onRowClick,
}: {
  deptId: string
  start: string
  end: string
  includeChildren: boolean
  onRowClick: (uid: string) => void
}) {
  const { resolveName } = useUserNameMap()
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [sortBy, setSortBy] = useState<MemberSortBy>('sum_total_tokens')
  const [sortOrder, setSortOrder] = useState<'asc' | 'desc'>('desc')
  const [searchInput, setSearchInput] = useState('')
  const [search, setSearch] = useState('')

  useEffect(() => {
    const id = window.setTimeout(() => setSearch(searchInput.trim()), 400)
    return () => window.clearTimeout(id)
  }, [searchInput])

  // 切部门/子部门开关/搜索时回到第一页。
  useEffect(() => {
    setPage(1)
  }, [deptId, includeChildren, search])

  const q = { deptId, start, end, includeChildren, page, pageSize, sortBy, sortOrder, search }
  const membersQ = useUsageDeptMembers(q)
  const rows = membersQ.data?.members ?? []
  const total = membersQ.data?.total ?? 0

  const handleSort = (field: string) => {
    const f = field as MemberSortBy
    if (sortBy === f) {
      setSortOrder((o) => (o === 'desc' ? 'asc' : 'desc'))
    } else {
      setSortBy(f)
      setSortOrder('desc')
    }
    setPage(1)
  }

  return (
    <ChartCard
      title="本部门人员"
      sub={`按 ${sortBy} ${sortOrder === 'desc' ? '降序' : '升序'}${includeChildren ? ' · 含子部门' : ' · 仅直属'}`}
      extra={
        <input
          type="search"
          value={searchInput}
          onChange={(e) => setSearchInput(e.target.value)}
          placeholder="搜索 ID / 用户名"
          className={INPUT_CLS}
          aria-label="搜索成员"
        />
      }
    >
      <div className="overflow-x-auto">
        <table className="w-full text-sm border-collapse">
          <thead>
            <tr className="border-b border-gray-200/50 dark:border-white/10">
              <th className={TH_NUM}>#</th>
              <th className="px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap">用户名</th>
              <th className="px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap">工号</th>
              <th className="px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap">用户 ID</th>
              {SORT_COLS.map((c) => (
                <th key={c.field} className={TH_NUM}>
                  <SortableTh
                    field={c.field}
                    label={c.label}
                    numeric
                    active={sortBy === c.field}
                    desc={sortOrder === 'desc'}
                    onSort={handleSort}
                  />
                </th>
              ))}
              <th className={TH_NUM}>预估花费</th>
            </tr>
          </thead>
          <tbody>
            {membersQ.isLoading && rows.length === 0 ? (
              <tr>
                <td colSpan={9} className="py-10 text-center text-sm text-gray-400 dark:text-gray-500">加载中…</td>
              </tr>
            ) : rows.length === 0 ? (
              <tr>
                <td colSpan={9}>
                  <EmptyHint compact />
                </td>
              </tr>
            ) : (
              rows.map((m, i) => {
                const uid = m.universal_id || ''
                return (
                  <tr
                    key={uid || i}
                    onClick={uid ? () => onRowClick(uid) : undefined}
                    className={`border-b border-gray-100/50 dark:border-white/5 ${uid ? 'cursor-pointer hover:bg-apple-blue/5 dark:hover:bg-white/5 transition-colors' : ''}`}
                  >
                    <td className={TD_NUM}>{(page - 1) * pageSize + i + 1}</td>
                    <td className={TD}>
                      <div className="max-w-[220px] truncate">
                        <ChatUserCell universalId={m.universal_id} chatUsername={m.username} resolveName={resolveName} />
                      </div>
                    </td>
                    <td className={TD} title={m.user_id || ''}>{m.user_id || '-'}</td>
                    <td className={TD}>
                      <span className="text-xs text-gray-400 font-mono" title={m.universal_id || ''}>{m.universal_id ? `${m.universal_id.slice(0, 12)}…` : '-'}</span>
                    </td>
                    <td className={TD_NUM}>{formatNumber(m.total_requests)}</td>
                    <td className={TD_NUM} title={formatNumber(m.sum_total_tokens)}>{shortToken(m.sum_total_tokens)}</td>
                    <td className={TD_NUM}>{m.success_rate.toFixed(1)}%</td>
                    <td className={TD_NUM}>{formatNumber(m.active_days)}</td>
                    <td className={TD_NUM}>{m.estimated_total_cost != null ? fmtCost(m.estimated_total_cost) : '-'}</td>
                  </tr>
                )
              })
            )}
          </tbody>
        </table>
      </div>

      <div className="mt-4 flex justify-end">
        <Pagination
          page={page}
          pageSize={pageSize}
          total={total}
          onPageChange={setPage}
          onSizeChange={(s) => {
            setPageSize(s)
            setPage(1)
          }}
        />
      </div>
    </ChartCard>
  )
}
