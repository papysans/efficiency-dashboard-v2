import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router'
import { useAllNeeds, useUsers } from '@/api/queries'
import { useUserNameMap } from '@/hooks/useUserNameMap'
import { RatioPill } from '@/components/ui/RatioPill'
import { sortRows } from '@/lib/sort'
import type { NeedsV2Summary, UserV2Row } from '@/api/types'

interface TopRankCardProps {
  startDate: string
  endDate: string
}

type Tab = 'need' | 'user'

const RANK_BADGE = [
  'bg-amber-400 text-white', // 1 金
  'bg-gray-300 text-gray-700 dark:bg-gray-400 dark:text-gray-900', // 2 银
  'bg-orange-300 text-white dark:bg-orange-400', // 3 铜
]
const RANK_DEFAULT = 'bg-gray-100 text-gray-500 dark:bg-gray-700 dark:text-gray-300'

/** 截断长 need_id（branch:.../pr:NN）用于展示 */
function shortNeedId(id: string): string {
  const colon = id.lastIndexOf(':')
  const tail = colon >= 0 ? id.slice(colon + 1) : id
  return tail.length > 28 ? `${tail.slice(0, 28)}…` : tail
}

/**
 * Top 提效榜（design-pr1 §1③）：需求 / 人 tab 切换。
 * 后端不支持 efficiency_ratio order → 客户端 sortRows（null 沉底）取 top6。
 */
export function TopRankCard({ startDate, endDate }: TopRankCardProps) {
  const navigate = useNavigate()
  const [tab, setTab] = useState<Tab>('need')
  const needsQ = useAllNeeds({ startDate, endDate })
  // pageSize:1000 一次性全量（对齐 UserList）：/v2/users 默认 pageSize=50 会服务端截断，
  // 人榜需全量再客户端 sortRows 取 top6，否则只在前 50 名里排。
  const usersQ = useUsers({ startDate, endDate, pageSize: 1000 })
  // 人榜 user_name 是 UUID（后端不回真名）→ 复用组织花名册同源的 /v2/user-names 解析「真名(工号)」。
  // 懒查询：只有切到「人」tab 才发请求，默认「需求」tab 不为此拉全量映射。
  const { resolveName } = useUserNameMap({ enabled: tab === 'user' })

  const topNeeds = useMemo<NeedsV2Summary[]>(() => {
    const rows = (needsQ.data ?? []).filter((r) => r.coverage_eligible && r.efficiency_ratio != null)
    return sortRows(rows, (r) => r.efficiency_ratio, true).slice(0, 6)
  }, [needsQ.data])

  const topUsers = useMemo<UserV2Row[]>(() => {
    const rows = (usersQ.data?.data ?? []).filter((r) => r.calendar_ratio != null)
    return sortRows(rows, (r) => r.calendar_ratio, true).slice(0, 6)
  }, [usersQ.data])

  const loading = tab === 'need' ? needsQ.isLoading : usersQ.isLoading
  const error = tab === 'need' ? needsQ.error : usersQ.error
  const empty = tab === 'need' ? topNeeds.length === 0 : topUsers.length === 0

  return (
    <div className="glass rounded-2xl p-5 md:p-6 hover:shadow-lg transition-shadow flex flex-col">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-sm font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide">Top 提效榜</h2>
        <div className="flex items-center gap-1 rounded-lg bg-gray-100/70 dark:bg-white/5 p-0.5" role="tablist" aria-label="榜单维度">
          <TabBtn active={tab === 'need'} onClick={() => setTab('need')} label="需求" />
          <TabBtn active={tab === 'user'} onClick={() => setTab('user')} label="人" />
        </div>
      </div>

      {error ? (
        <div className="flex-1 flex items-center justify-center text-sm text-rose-600 dark:text-rose-400 min-h-[14rem]">
          加载失败：{(error as Error).message}
        </div>
      ) : loading ? (
        <ul className="flex-1 space-y-2">
          {Array.from({ length: 6 }).map((_, i) => (
            <li key={i} className="skeleton h-11 rounded-xl" />
          ))}
        </ul>
      ) : empty ? (
        <div className="flex-1 flex items-center justify-center text-sm text-gray-500 dark:text-gray-400 min-h-[14rem]">
          暂无可计入榜单数据
        </div>
      ) : tab === 'need' ? (
        <ul className="flex-1 space-y-2">
          {topNeeds.map((r, i) => (
            <RankRow
              key={r.need_id}
              rank={i + 1}
              title={shortNeedId(r.need_id)}
              sub={r.repo_branch}
              pill={<RatioPill value={r.efficiency_ratio} />}
              onClick={() => navigate(`/needs/${encodeURIComponent(r.need_id)}`)}
            />
          ))}
        </ul>
      ) : (
        <ul className="flex-1 space-y-2">
          {topUsers.map((r, i) => (
            <RankRow
              key={r.user_id}
              rank={i + 1}
              title={resolveName(r.user_id)}
              sub={`合并需求 ${r.merged_need_count}`}
              pill={<RatioPill value={r.calendar_ratio} />}
              onClick={() => navigate(`/user/${encodeURIComponent(r.user_id)}`)}
            />
          ))}
        </ul>
      )}
    </div>
  )
}

function TabBtn({ active, onClick, label }: { active: boolean; onClick: () => void; label: string }) {
  return (
    <button
      type="button"
      role="tab"
      aria-selected={active}
      onClick={onClick}
      className={`px-3 py-1 rounded-md text-xs font-medium cursor-pointer transition-colors border-none ${
        active
          ? 'bg-white dark:bg-white/15 text-gray-900 dark:text-white shadow-sm'
          : 'bg-transparent text-gray-500 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white'
      }`}
    >
      {label}
    </button>
  )
}

function RankRow({
  rank,
  title,
  sub,
  pill,
  onClick,
}: {
  rank: number
  title: string
  sub: string
  pill: React.ReactNode
  onClick?: () => void
}) {
  const badge = rank <= 3 ? RANK_BADGE[rank - 1] : RANK_DEFAULT
  const clickable = !!onClick
  return (
    <li
      className={`flex items-center gap-3 rounded-xl px-2 py-1.5 hover:bg-white/40 dark:hover:bg-white/5 transition-colors ${
        clickable ? 'cursor-pointer focus:outline-none focus-visible:ring-2 focus-visible:ring-sky-400' : ''
      }`}
      onClick={onClick}
      role={clickable ? 'button' : undefined}
      tabIndex={clickable ? 0 : undefined}
      onKeyDown={clickable ? (e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), onClick?.()) : undefined}
      aria-label={clickable ? `${title}，点击查看详情` : undefined}
    >
      <span className={`shrink-0 w-6 h-6 rounded-full flex items-center justify-center text-xs font-bold tabular-nums ${badge}`}>
        {rank}
      </span>
      <div className="min-w-0 flex-1">
        <div className="text-sm font-medium text-gray-900 dark:text-white truncate" title={title}>
          {title}
        </div>
        <div className="text-xs text-gray-400 dark:text-gray-500 truncate" title={sub}>
          {sub}
        </div>
      </div>
      <span className="shrink-0">{pill}</span>
    </li>
  )
}
