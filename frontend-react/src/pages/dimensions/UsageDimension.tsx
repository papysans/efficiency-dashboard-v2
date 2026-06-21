// 「使用」维度内容（按 entity 分支）：
//   user → 平台金源（请求/活跃天/会话/用户数）。universal_id == 看板 user_id。
//   org  → 平台经部门映射聚合（dept-sync 直属成员 universal_id 集合 × 平台排行求和；非递归=仅直属成员）。
//   project/repo → **平台填不了这俩**（平台源无项目/仓库字段）→ 走看板派生口径（AI 渗透/贡献者 / AI 代码占比）。
// 降级护栏：chat_stats_enabled=false 或请求失败 → PlatformNotConnected（user/org），绝不发请求/不空页。
//   project/repo 是看板派生，不受平台开关影响，照常渲染。
import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router'
import { useGlobalConfig } from '@/api/queries'
import { useViewState } from '@/store/viewState'
import { useUserNameMap } from '@/hooks/useUserNameMap'
import { useEntityFocus } from '@/components/layout/matrix'
import { MetricCard } from '@/components/ui/MetricCard'
import { formatNumber } from '@/lib/formatters'
import { ChartCard, ChatUserCell, EmptyHint, shortToken } from '@/pages/platform/platformShared'
import {
  DimSkeleton,
  DirectMembersNote,
  PlatformFullVolumeHeadline,
  PlatformNotConnected,
  PlatformWeekTrend,
  TruncationNote,
  useDeptFocus,
  type WeekSeriesSpec,
} from './platformDimShared'
import {
  useUserRanking,
  useUserRankingFocused,
  useUserWeekSeries,
  pickFocusedRow,
  type ChatUserRankingRow,
} from './platformUserData'
import {
  useDeptPlatformRanking,
  useDeptPlatformFocused,
  useDeptWeekSeries,
  type DeptPlatformAgg,
} from './platformDeptData'
import { KanbanUsage } from './kanbanDerived'

const TH = 'px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_NUM = 'px-3 py-2 text-right font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TD = 'px-3 py-2 align-middle text-gray-700 dark:text-gray-200 whitespace-nowrap'
const TD_NUM = 'px-3 py-2 align-middle text-right tabular-nums text-gray-700 dark:text-gray-200'
const INPUT_CLS =
  'glass rounded-lg px-3 py-1.5 text-sm bg-transparent text-gray-900 dark:text-white placeholder-gray-400 ' +
  'focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue'

export default function UsageDimension() {
  const { entity, object, objectLabel } = useEntityFocus()
  const { timeRange } = useViewState()
  const { data: gc, isLoading: gcLoading } = useGlobalConfig()
  // config 未就绪前不判「未接入」（避免内网平台已启用却闪一下占位）；就绪后 chat_stats_enabled 为准。
  const configResolved = !!gc && !gcLoading
  const chatEnabled = gc?.chat_stats_enabled === true
  const focused = object !== ''

  // project/repo：平台填不了 → 看板派生口径（不看 chat 开关、不发 chat 请求）。
  if (entity === 'project' || entity === 'repo') {
    return <KanbanUsage entity={entity} object={object} objectLabel={objectLabel} focused={focused} timeRange={timeRange} />
  }

  // 以下 user/org 走平台。config 加载中：轻骨架，不发请求、不误判降级。
  if (!configResolved) {
    return <DimSkeleton />
  }
  // 降级护栏：开关 false → 不发任何请求，直接优雅占位。
  if (!chatEnabled) {
    return <PlatformNotConnected reason="disabled" />
  }

  if (entity === 'org') {
    return <OrgUsage object={object} objectLabel={objectLabel} focused={focused} timeRange={timeRange} />
  }

  return <UserUsage object={object} objectLabel={objectLabel} focused={focused} timeRange={timeRange} />
}

// ============================ 个人（user）：平台金源 ============================
function UserUsage({
  object,
  objectLabel,
  focused,
  timeRange,
}: {
  object: string
  objectLabel: string
  focused: boolean
  timeRange: [string, string]
}) {
  const [start, end] = timeRange
  const { resolveName } = useUserNameMap()
  const navigate = useNavigate()
  // 排行行下钻 → 看板用户详情（universal_id 与看板 user_id 同源，与 ChatUserCell 互链同址）。
  const goToUser = (universalId: string) => navigate(`/user/${encodeURIComponent(universalId)}`)

  const [searchInput, setSearchInput] = useState('')
  const [search, setSearch] = useState('')
  useEffect(() => {
    const id = window.setTimeout(() => setSearch(searchInput.trim()), 400)
    return () => window.clearTimeout(id)
  }, [searchInput])

  const series = useUserWeekSeries({ startDate: start, endDate: end, universalId: focused ? object : undefined }, true)
  const rankQ = useUserRanking({ startDate: start, endDate: end, sortBy: 'total_requests', search, pageSize: 50 }, !focused)
  const focusQ = useUserRankingFocused({ startDate: start, endDate: end, universalId: object }, focused)
  const focusedRow = focused ? pickFocusedRow(focusQ.data, object) : null

  // 聚合态：请求量(几十万) 与 活跃用户(几百) 量级悬殊 → 活跃用户走右轴独立刻度（防贴底直线）。
  //   聚焦态两条都是该用户量级（请求/会话），同左轴即可。
  const trendSeries = useMemo<WeekSeriesSpec[]>(() => {
    if (focused) {
      return [
        { name: '请求量', color: '#ff9500', values: series.points.map((pt) => pt.row?.total_requests ?? 0) },
        { name: '会话数', color: '#0071e3', values: series.points.map((pt) => pt.row?.unique_task_count ?? 0) },
      ]
    }
    return [
      { name: '请求量', color: '#ff9500', values: series.windows.map((w) => series.aggByKey.get(w.key)?.totalRequests ?? 0) },
      { name: '活跃用户', color: '#34c759', axis: 'right', values: series.windows.map((w) => series.aggByKey.get(w.key)?.activeUsers ?? 0) },
    ]
  }, [focused, series.points, series.windows, series.aggByKey])

  const fatalError = (!focused && rankQ.error) || (focused && focusQ.error)
  if (fatalError) {
    const msg = ((rankQ.error || focusQ.error) as Error)?.message
    return <PlatformNotConnected reason="error" detail={msg} />
  }

  const rows = rankQ.data?.data ?? []
  const total = rankQ.data?.total

  return (
    <div className="flex flex-col gap-5">
      {!focused && <PlatformFullVolumeHeadline start={start} end={end} />}
      <PlatformWeekTrend
        title="使用趋势（平台）"
        subtitle={focused ? `个人 · ${objectLabel || object} · 按周` : '全部用户 · 按周'}
        windows={series.windows}
        series={trendSeries}
        loading={series.loading}
        error={series.error}
        hasAny={series.hasAny}
        yFmt={(v) => shortToken(v)}
        // 聚合态右轴 = 活跃用户（计数，独立刻度，整数缩写避免和左轴请求量混读）。
        rightYFmt={(v) => formatNumber(v)}
      />

      {focused ? (
        <FocusedUsage row={focusedRow} loading={focusQ.isLoading} objectLabel={objectLabel || object} />
      ) : (
        <>
          <AggregateKpis rows={rows} total={total} />
          {/* P1-2：周序列按 Top 500 拉窗求和，区间真实人数更大时趋势/活跃成员被截断 → 醒目标注。 */}
          {series.truncated && <TruncationNote total={series.maxWindowTotal} />}
          <ChartCard
            title="用户使用排行（平台）"
            sub={`区间聚合 · Top 50${total != null ? ` · 共 ${formatNumber(total)} 人` : ''}`}
            extra={
              <input
                type="search"
                value={searchInput}
                onChange={(e) => setSearchInput(e.target.value)}
                placeholder="搜索 ID / 用户名"
                className={INPUT_CLS}
                aria-label="搜索用户"
              />
            }
          >
            <UsageRankingTable rows={rows} loading={rankQ.isFetching} resolveName={resolveName} onRowClick={goToUser} />
          </ChartCard>
        </>
      )}
    </div>
  )
}

function AggregateKpis({ rows, total }: { rows: ChatUserRankingRow[]; total?: number }) {
  const sum = (fn: (r: ChatUserRankingRow) => number) => rows.reduce((s, r) => s + (fn(r) || 0), 0)
  const requests = sum((r) => r.total_requests)
  const sessions = sum((r) => r.unique_task_count)
  return (
    <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
      <MetricCard label="活跃用户" value={total != null ? formatNumber(total) : formatNumber(rows.length)} hint="区间内有平台调用的用户" />
      <MetricCard label="总请求（Top50）" value={formatNumber(requests)} hint="当前排行页合计" />
      <MetricCard label="会话数（Top50）" value={formatNumber(sessions)} hint="unique_task_count 合计" />
      <MetricCard label="人均请求（Top50）" value={rows.length ? formatNumber(Math.round(requests / rows.length)) : '-'} />
    </div>
  )
}

function FocusedUsage({ row, loading, objectLabel }: { row: ChatUserRankingRow | null; loading: boolean; objectLabel: string }) {
  if (loading) {
    return (
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        {Array.from({ length: 4 }).map((_, i) => (
          <div key={i} className="skeleton h-24 rounded-2xl" />
        ))}
      </div>
    )
  }
  if (!row) {
    return (
      <div className="glass rounded-2xl p-10 text-center text-sm text-gray-500 dark:text-gray-400">
        该用户在所选区间内无平台使用记录（{objectLabel}）。
      </div>
    )
  }
  return (
    <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
      <MetricCard label="总请求" value={formatNumber(row.total_requests)} />
      <MetricCard label="活跃天数" value={formatNumber(row.active_days)} />
      <MetricCard label="会话数" value={formatNumber(row.unique_task_count)} hint="unique_task_count" />
      <MetricCard label="总 Token" value={shortToken(row.sum_total_tokens)} hint={formatNumber(row.sum_total_tokens)} />
    </div>
  )
}

function UsageRankingTable({
  rows,
  loading,
  resolveName,
  onRowClick,
}: {
  rows: ChatUserRankingRow[]
  loading: boolean
  resolveName: (id?: string) => string
  onRowClick: (universalId: string) => void
}) {
  return (
    <div className="overflow-x-auto max-h-[520px] overflow-y-auto">
      <table className="w-full text-sm border-collapse">
        <thead className="sticky top-0 bg-white/70 dark:bg-gray-900/70 backdrop-blur">
          <tr className="border-b border-gray-200/50 dark:border-white/10">
            <th className={TH_NUM}>排名</th>
            <th className={TH}>用户</th>
            <th className={TH_NUM}>请求数</th>
            <th className={TH_NUM}>会话数</th>
            <th className={TH_NUM}>活跃天数</th>
            <th className={TH_NUM}>总 Token</th>
          </tr>
        </thead>
        <tbody>
          {loading && rows.length === 0 ? (
            <tr>
              <td colSpan={6} className="py-10 text-center text-sm text-gray-400 dark:text-gray-500">加载中…</td>
            </tr>
          ) : rows.length === 0 ? (
            <tr>
              <td colSpan={6}>
                <EmptyHint compact />
              </td>
            </tr>
          ) : (
            rows.map((u, i) => {
              const uid = u.universal_id || ''
              return (
                <tr
                  key={uid || i}
                  onClick={uid ? () => onRowClick(uid) : undefined}
                  className={`border-b border-gray-100/50 dark:border-white/5 ${uid ? 'cursor-pointer hover:bg-apple-blue/5 dark:hover:bg-white/5 transition-colors' : ''}`}
                >
                  <td className={TD_NUM}>{i + 1}</td>
                  <td className={TD}>
                    <div className="max-w-[200px] truncate">
                      <ChatUserCell universalId={u.universal_id} chatUsername={u.username} resolveName={resolveName} />
                    </div>
                  </td>
                  <td className={TD_NUM}>{formatNumber(u.total_requests)}</td>
                  <td className={TD_NUM}>{formatNumber(u.unique_task_count)}</td>
                  <td className={TD_NUM}>{formatNumber(u.active_days)}</td>
                  <td className={TD_NUM} title={formatNumber(u.sum_total_tokens)}>
                    {shortToken(u.sum_total_tokens)}
                  </td>
                </tr>
              )
            })
          )}
        </tbody>
      </table>
    </div>
  )
}

// ============================ 组织（org）：平台经部门映射聚合 ============================
function OrgUsage({
  object,
  objectLabel,
  focused,
  timeRange,
}: {
  object: string
  objectLabel: string
  focused: boolean
  timeRange: [string, string]
}) {
  const [start, end] = timeRange
  // 部门行下钻 → 写 ?object=<dept_id> 进聚焦态（org 主体下钻范式，与 OrgContribution.goDept 一致）。
  const goDept = useDeptFocus()

  const series = useDeptWeekSeries({ startDate: start, endDate: end, deptId: focused ? object : undefined }, true)
  const rankQ = useDeptPlatformRanking({ startDate: start, endDate: end }, !focused)
  const focusQ = useDeptPlatformFocused({ startDate: start, endDate: end, deptId: object }, focused)

  // 请求量(部门聚合，量级大) vs 活跃成员(几十人) 量级悬殊 → 活跃成员走右轴独立刻度（防贴底直线）。
  const trendSeries = useMemo<WeekSeriesSpec[]>(
    () => [
      { name: '请求量', color: '#ff9500', values: series.windows.map((w) => series.aggByKey.get(w.key)?.totalRequests ?? 0) },
      { name: '活跃成员', color: '#34c759', axis: 'right', values: series.windows.map((w) => series.aggByKey.get(w.key)?.activeUsers ?? 0) },
    ],
    [series.windows, series.aggByKey],
  )

  const fatalError = (!focused && rankQ.error) || (focused && focusQ.error)
  if (fatalError) {
    const msg = (!focused ? rankQ.error : focusQ.error) ?? undefined
    return <PlatformNotConnected reason="error" detail={msg ?? undefined} />
  }

  const items = (rankQ.items ?? []).slice().sort((a, b) => b.totalRequests - a.totalRequests)

  return (
    <div className="flex flex-col gap-5">
      <DirectMembersNotice />

      {!focused && <PlatformFullVolumeHeadline start={start} end={end} />}

      <PlatformWeekTrend
        title="使用趋势（平台·部门聚合）"
        subtitle={focused ? `部门 · ${objectLabel || object} · 按周 · 仅直属成员` : '全部一级部门 · 按周 · 仅直属成员'}
        windows={series.windows}
        series={trendSeries}
        loading={series.loading}
        error={series.error}
        hasAny={series.hasAny}
        yFmt={(v) => shortToken(v)}
        // 右轴 = 活跃成员（计数，独立刻度）。
        rightYFmt={(v) => formatNumber(v)}
      />

      {focused ? (
        <>
          <DirectMembersNote />
          <FocusedDeptUsage agg={focusQ.agg} loading={focusQ.loading} objectLabel={objectLabel || object} />
        </>
      ) : (
        <>
          <DeptAggregateKpis items={items} />
          {/* P1-2：部门聚合按 Top 500 全量排行命中求和，区间真实人数更大时漏算排行外成员 → 醒目标注。 */}
          {rankQ.truncated && <TruncationNote total={rankQ.rankingTotal} />}
          <ChartCard title="部门使用排行（平台·直属成员聚合）" sub="区间聚合 · 一级部门 · 按请求量倒序 · 点行下钻">
            <DeptUsageRankingTable items={items} loading={rankQ.loading} onRowClick={goDept} />
          </ChartCard>
        </>
      )}
    </div>
  )
}

/** 部门口径诚实标注（dept-tree/members 非递归 → 仅直属成员）。 */
function DirectMembersNotice() {
  return (
    <div className="glass rounded-xl px-4 py-2.5 flex items-start gap-2 text-xs border-l-4" style={{ borderLeftColor: '#0071e3' }} role="note">
      <svg className="w-4 h-4 shrink-0 text-sky-500 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
      </svg>
      <span className="text-gray-600 dark:text-gray-300">
        平台无部门维度，本视图按 <b>universal_id → 部门花名册</b> 在看板侧聚合；花名册仅含<b>直属成员</b>（非递归，未含子部门 rollup）。
      </span>
    </div>
  )
}

function DeptAggregateKpis({ items }: { items: DeptPlatformAgg[] }) {
  const sum = (fn: (it: DeptPlatformAgg) => number) => items.reduce((s, it) => s + (fn(it) || 0), 0)
  const requests = sum((it) => it.totalRequests)
  const sessions = sum((it) => it.uniqueTaskCount)
  const active = sum((it) => it.activePlatformUsers)
  return (
    <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
      <MetricCard label="一级部门数" value={formatNumber(items.length)} />
      <MetricCard label="活跃成员" value={formatNumber(active)} hint="区间内有平台调用的直属成员" />
      <MetricCard label="总请求" value={formatNumber(requests)} />
      <MetricCard label="会话数" value={formatNumber(sessions)} hint="unique_task_count 合计" />
    </div>
  )
}

function FocusedDeptUsage({ agg, loading, objectLabel }: { agg: DeptPlatformAgg | null; loading: boolean; objectLabel: string }) {
  if (loading) {
    return (
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        {Array.from({ length: 4 }).map((_, i) => (
          <div key={i} className="skeleton h-24 rounded-2xl" />
        ))}
      </div>
    )
  }
  if (!agg || agg.activePlatformUsers === 0) {
    return (
      <div className="glass rounded-2xl p-10 text-center text-sm text-gray-500 dark:text-gray-400">
        该部门直属成员在所选区间内无平台使用记录（{objectLabel}）。
      </div>
    )
  }
  return (
    <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
      <MetricCard label="活跃成员" value={`${formatNumber(agg.activePlatformUsers)} / ${formatNumber(agg.memberCount)}`} hint="有平台调用 / 直属成员" />
      <MetricCard label="总请求" value={formatNumber(agg.totalRequests)} />
      <MetricCard label="会话数" value={formatNumber(agg.uniqueTaskCount)} hint="unique_task_count" />
      <MetricCard label="总 Token" value={shortToken(agg.sumTotalTokens)} hint={formatNumber(agg.sumTotalTokens)} />
    </div>
  )
}

function DeptUsageRankingTable({
  items,
  loading,
  onRowClick,
}: {
  items: DeptPlatformAgg[]
  loading: boolean
  onRowClick: (deptId: string) => void
}) {
  return (
    <div className="overflow-x-auto max-h-[520px] overflow-y-auto">
      <table className="w-full text-sm border-collapse">
        <thead className="sticky top-0 bg-white/70 dark:bg-gray-900/70 backdrop-blur">
          <tr className="border-b border-gray-200/50 dark:border-white/10">
            <th className={TH_NUM}>排名</th>
            <th className={TH}>部门</th>
            <th className={TH_NUM}>活跃成员</th>
            <th className={TH_NUM}>请求数</th>
            <th className={TH_NUM}>会话数</th>
            <th className={TH_NUM}>总 Token</th>
          </tr>
        </thead>
        <tbody>
          {loading && items.length === 0 ? (
            <tr>
              <td colSpan={6} className="py-10 text-center text-sm text-gray-400 dark:text-gray-500">加载中…</td>
            </tr>
          ) : items.length === 0 ? (
            <tr>
              <td colSpan={6}>
                <EmptyHint compact />
              </td>
            </tr>
          ) : (
            items.map((it, i) => (
              <tr
                key={it.deptId || i}
                onClick={it.deptId ? () => onRowClick(it.deptId) : undefined}
                className={`border-b border-gray-100/50 dark:border-white/5 ${it.deptId ? 'cursor-pointer hover:bg-apple-blue/5 dark:hover:bg-white/5 transition-colors' : ''}`}
              >
                <td className={TD_NUM}>{i + 1}</td>
                <td className={TD}>
                  {it.deptId ? (
                    <button
                      type="button"
                      className="max-w-[220px] truncate text-left font-medium text-apple-blue hover:text-apple-blue-hover bg-transparent border-none p-0 cursor-pointer focus:outline-none focus-visible:underline"
                      title={it.deptName}
                      onClick={(e) => {
                        e.stopPropagation()
                        onRowClick(it.deptId)
                      }}
                    >
                      {it.deptName}
                    </button>
                  ) : (
                    <div className="max-w-[220px] truncate" title={it.deptName}>{it.deptName}</div>
                  )}
                </td>
                <td className={TD_NUM}>
                  {formatNumber(it.activePlatformUsers)}
                  <span className="text-gray-400 dark:text-gray-500"> / {formatNumber(it.memberCount)}</span>
                </td>
                <td className={TD_NUM}>{formatNumber(it.totalRequests)}</td>
                <td className={TD_NUM}>{formatNumber(it.uniqueTaskCount)}</td>
                <td className={TD_NUM} title={formatNumber(it.sumTotalTokens)}>
                  {shortToken(it.sumTotalTokens)}
                </td>
              </tr>
            ))
          )}
        </tbody>
      </table>
    </div>
  )
}
