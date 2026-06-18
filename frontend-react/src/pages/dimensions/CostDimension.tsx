// 「成本」维度内容（按 entity 分支）。口径决策①：user/org 两套并列卡，各自标源，口径别混（成本双源陷阱）：
//   ① AI 调用花费（平台·客观）= estimated_total_cost(¥) + tokens（shortToken）。
//   ② 人天成本（看板·折算）= 个人/部门人天成本**看板侧当前无数据 → 建设中占位**（不编造）。
//   project/repo → 平台无项目/仓库口径 → 看板费用**单卡**（KanbanCost，非双卡，显式注明）。
// 时间线 = 平台 AI 花费周序列（切窗）。聚合态=对象 AI 花费排行；聚焦态=该对象花费明细。
// 降级护栏：开关 false / 请求失败 → 平台卡显示「未接入平台」，看板人天卡照常（也是建设中）；不空页不抛错。
import { useEffect, useMemo, useState } from 'react'
import { useGlobalConfig } from '@/api/queries'
import { useViewState } from '@/store/viewState'
import { useUserNameMap } from '@/hooks/useUserNameMap'
import { useEntityFocus } from '@/components/layout/EntityDimensionLayout'
import { MetricCard } from '@/components/ui/MetricCard'
import { formatNumber } from '@/lib/formatters'
import { ChartCard, ChatUserCell, EmptyHint, shortToken } from '@/pages/platform/platformShared'
import { DirectMembersNote, PlatformNotConnected, PlatformWeekTrend, TruncationNote } from './platformDimShared'
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
import { KanbanCost } from './kanbanDerived'

const TH = 'px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_NUM = 'px-3 py-2 text-right font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TD = 'px-3 py-2 align-middle text-gray-700 dark:text-gray-200 whitespace-nowrap'
const TD_NUM = 'px-3 py-2 align-middle text-right tabular-nums text-gray-700 dark:text-gray-200'
const INPUT_CLS =
  'glass rounded-lg px-3 py-1.5 text-sm bg-transparent text-gray-900 dark:text-white placeholder-gray-400 ' +
  'focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue'

const fmtYuan = (v: number | null | undefined) => (v != null ? `¥${Number(v).toFixed(2)}` : '-')

/** 「人天成本（看板·折算）」卡 —— 个人/部门人天成本看板侧当前无数据，建设中占位（与平台¥口径分开标注）。 */
function PersonDayCostCard({ subject = '个人' }: { subject?: string }) {
  return (
    <section className="glass rounded-2xl p-5 flex flex-col">
      <div className="flex items-center justify-between mb-3 gap-3">
        <h2 className="text-sm font-semibold text-gray-700 dark:text-gray-200">人天成本（看板·折算）</h2>
        <span className="text-xs text-gray-400 dark:text-gray-500">人力折算 · 建设中</span>
      </div>
      <div className="flex-1 flex flex-col items-center justify-center text-center py-8 gap-2">
        <svg className="w-9 h-9 text-gray-300 dark:text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
        <p className="text-sm text-gray-500 dark:text-gray-400">{subject}人天成本数据建设中</p>
        <p className="text-xs text-gray-400 dark:text-gray-500 max-w-xs">
          口径 = 人力投入折算（人天 × 单价），与平台¥（Token 调用花费）不同源，请勿混用。
        </p>
      </div>
    </section>
  )
}

export default function CostDimension() {
  const { entity, object, objectLabel } = useEntityFocus()
  const { timeRange } = useViewState()
  const { data: gc, isLoading: gcLoading } = useGlobalConfig()
  const configResolved = !!gc && !gcLoading
  const chatEnabled = gc?.chat_stats_enabled === true
  const focused = object !== ''

  // project/repo：平台无项目/仓库口径 → 看板费用单卡（不发 chat 请求，不看开关）。
  if (entity === 'project' || entity === 'repo') {
    return <KanbanCost entity={entity} object={object} objectLabel={objectLabel} focused={focused} timeRange={timeRange} />
  }

  const subject = entity === 'org' ? '部门' : '个人'

  if (!configResolved) {
    return (
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <div className="skeleton h-48 rounded-2xl" />
        <PersonDayCostCard subject={subject} />
      </div>
    )
  }

  // 降级：开关 false → 平台卡占位，但人天卡照常渲染（双卡口径分开，平台缺位不连累看板侧）。
  if (!chatEnabled) {
    return (
      <div className="flex flex-col gap-5">
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          <section className="glass rounded-2xl p-5">
            <h2 className="text-sm font-semibold text-gray-700 dark:text-gray-200 mb-3">AI 调用花费（平台·客观）</h2>
            <PlatformNotConnected reason="disabled" />
          </section>
          <PersonDayCostCard subject={subject} />
        </div>
      </div>
    )
  }

  if (entity === 'org') {
    return <OrgCostContent object={object} objectLabel={objectLabel} focused={focused} timeRange={timeRange} />
  }

  return <CostContent object={object} objectLabel={objectLabel} focused={focused} timeRange={timeRange} />
}

function CostContent({
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

  const [searchInput, setSearchInput] = useState('')
  const [search, setSearch] = useState('')
  useEffect(() => {
    const id = window.setTimeout(() => setSearch(searchInput.trim()), 400)
    return () => window.clearTimeout(id)
  }, [searchInput])

  const series = useUserWeekSeries({ startDate: start, endDate: end, universalId: focused ? object : undefined }, true)
  const rankQ = useUserRanking({ startDate: start, endDate: end, sortBy: 'estimated_total_cost', search, pageSize: 50 }, !focused)
  const focusQ = useUserRankingFocused({ startDate: start, endDate: end, universalId: object }, focused)
  const focusedRow = focused ? pickFocusedRow(focusQ.data, object) : null

  // 趋势：AI 花费(¥)周序列。聚焦=该用户 estimated_total_cost；聚合=整窗 cost。
  const trendSeries = useMemo(() => {
    const values = focused
      ? series.points.map((pt) => +(pt.row?.estimated_total_cost ?? 0).toFixed(2))
      : series.windows.map((w) => +(series.aggByKey.get(w.key)?.cost ?? 0).toFixed(2))
    return [{ name: 'AI 花费', color: '#af52de', values }]
  }, [focused, series.points, series.windows, series.aggByKey])

  // 平台请求失败 → 平台卡区域占位，但人天卡照常。
  const platformError = (!focused && rankQ.error) || (focused && focusQ.error)
  const platformErrMsg = ((rankQ.error || focusQ.error) as Error | undefined)?.message

  const rows = rankQ.data?.data ?? []

  return (
    <div className="flex flex-col gap-5">
      {/* 双卡口径标注（顶部 KPI 行：平台 AI 花费汇总 + 人天建设中） */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        {platformError ? (
          <section className="glass rounded-2xl p-5">
            <h2 className="text-sm font-semibold text-gray-700 dark:text-gray-200 mb-3">AI 调用花费（平台·客观）</h2>
            <PlatformNotConnected reason="error" detail={platformErrMsg} />
          </section>
        ) : (
          <PlatformCostCard
            focused={focused}
            focusedRow={focusedRow}
            rows={rows}
            loading={focused ? focusQ.isLoading : rankQ.isFetching}
            objectLabel={objectLabel || object}
          />
        )}
        <PersonDayCostCard />
      </div>

      {/* 平台 AI 花费时间线 */}
      {!platformError && (
        <PlatformWeekTrend
          title="AI 花费趋势（平台·客观）"
          subtitle={focused ? `个人 · ${objectLabel || object} · 按周` : '全部用户 · 按周'}
          windows={series.windows}
          series={trendSeries}
          loading={series.loading}
          error={series.error}
          hasAny={series.hasAny}
          yFmt={(v) => `¥${shortToken(v)}`}
        />
      )}

      {/* P1-2：AI 花费周序列按 Top 500 拉窗求和，区间真实人数更大时趋势被截断 → 醒目标注。 */}
      {!focused && !platformError && series.truncated && <TruncationNote total={series.maxWindowTotal} />}

      {/* 聚合态：用户 AI 花费排行；聚焦态在卡内已给明细 */}
      {!focused && !platformError && (
        <ChartCard
          title="用户 AI 花费排行（平台·客观）"
          sub={`区间聚合 · Top 50${rankQ.data?.total != null ? ` · 共 ${formatNumber(rankQ.data.total)} 人` : ''}`}
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
          <CostRankingTable rows={rows} loading={rankQ.isFetching} resolveName={resolveName} />
        </ChartCard>
      )}
    </div>
  )
}

function PlatformCostCard({
  focused,
  focusedRow,
  rows,
  loading,
  objectLabel,
}: {
  focused: boolean
  focusedRow: ChatUserRankingRow | null
  rows: ChatUserRankingRow[]
  loading: boolean
  objectLabel: string
}) {
  // 聚焦 = 该用户；聚合 = 当前 Top50 合计。
  const cost = focused ? focusedRow?.estimated_total_cost ?? null : rows.reduce((s, r) => s + (r.estimated_total_cost || 0), 0)
  const tokens = focused ? focusedRow?.sum_total_tokens ?? null : rows.reduce((s, r) => s + (r.sum_total_tokens || 0), 0)
  const cacheTokens = focused ? focusedRow?.sum_cache_tokens ?? null : rows.reduce((s, r) => s + (r.sum_cache_tokens || 0), 0)

  return (
    <section className="glass rounded-2xl p-5 flex flex-col" style={{ borderLeft: '3px solid #af52de' }}>
      <div className="flex items-center justify-between mb-3 gap-3">
        <h2 className="text-sm font-semibold text-gray-700 dark:text-gray-200">AI 调用花费（平台·客观）</h2>
        <span className="text-xs text-gray-400 dark:text-gray-500">
          {focused ? objectLabel : 'Top50 合计'} · Token 调用花费
        </span>
      </div>
      {loading ? (
        <div className="grid grid-cols-3 gap-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="skeleton h-20 rounded-xl" />
          ))}
        </div>
      ) : focused && !focusedRow ? (
        <div className="flex-1 flex items-center justify-center py-8 text-sm text-gray-500 dark:text-gray-400">
          该用户在所选区间内无平台花费记录。
        </div>
      ) : (
        <div className="grid grid-cols-3 gap-3">
          <MetricCard label="AI 花费" value={fmtYuan(cost)} hint="estimated_total_cost" />
          <MetricCard label="总 Token" value={shortToken(tokens)} hint={formatNumber(tokens)} />
          <MetricCard label="缓存 Token" value={shortToken(cacheTokens)} hint={formatNumber(cacheTokens)} />
        </div>
      )}
    </section>
  )
}

function CostRankingTable({
  rows,
  loading,
  resolveName,
}: {
  rows: ChatUserRankingRow[]
  loading: boolean
  resolveName: (id?: string) => string
}) {
  return (
    <div className="overflow-x-auto max-h-[520px] overflow-y-auto">
      <table className="w-full text-sm border-collapse">
        <thead className="sticky top-0 bg-white/70 dark:bg-gray-900/70 backdrop-blur">
          <tr className="border-b border-gray-200/50 dark:border-white/10">
            <th className={TH_NUM}>排名</th>
            <th className={TH}>用户</th>
            <th className={TH_NUM}>AI 花费（¥）</th>
            <th className={TH_NUM}>总 Token</th>
            <th className={TH_NUM}>缓存 Token</th>
            <th className={TH_NUM}>请求数</th>
          </tr>
        </thead>
        <tbody>
          {loading && rows.length === 0 ? (
            <tr>
              <td colSpan={6} className="py-10 text-center text-sm text-gray-400 dark:text-gray-500">
                加载中…
              </td>
            </tr>
          ) : rows.length === 0 ? (
            <tr>
              <td colSpan={6}>
                <EmptyHint compact />
              </td>
            </tr>
          ) : (
            rows.map((u, i) => (
              <tr key={u.universal_id || i} className="border-b border-gray-100/50 dark:border-white/5">
                <td className={TD_NUM}>{i + 1}</td>
                <td className={TD}>
                  <div className="max-w-[200px] truncate">
                    <ChatUserCell universalId={u.universal_id} chatUsername={u.username} resolveName={resolveName} />
                  </div>
                </td>
                <td className={TD_NUM}>{u.estimated_total_cost.toFixed(2)}</td>
                <td className={TD_NUM} title={formatNumber(u.sum_total_tokens)}>
                  {shortToken(u.sum_total_tokens)}
                </td>
                <td className={TD_NUM} title={formatNumber(u.sum_cache_tokens)}>
                  {shortToken(u.sum_cache_tokens)}
                </td>
                <td className={TD_NUM}>{formatNumber(u.total_requests)}</td>
              </tr>
            ))
          )}
        </tbody>
      </table>
    </div>
  )
}

// ============================ 组织（org）：平台部门聚合 AI 花费 ‖ 部门人天（建设中） ============================
function OrgCostContent({
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

  const series = useDeptWeekSeries({ startDate: start, endDate: end, deptId: focused ? object : undefined }, true)
  const rankQ = useDeptPlatformRanking({ startDate: start, endDate: end }, !focused)
  const focusQ = useDeptPlatformFocused({ startDate: start, endDate: end, deptId: object }, focused)

  const trendSeries = useMemo(
    () => [
      { name: 'AI 花费', color: '#af52de', values: series.windows.map((w) => +(series.aggByKey.get(w.key)?.cost ?? 0).toFixed(2)) },
    ],
    [series.windows, series.aggByKey],
  )

  const platformError = (!focused && rankQ.error) || (focused && focusQ.error)
  const platformErrMsg = (!focused ? rankQ.error : focusQ.error) ?? undefined

  const items = (rankQ.items ?? []).slice().sort((a, b) => b.estimatedTotalCost - a.estimatedTotalCost)

  return (
    <div className="flex flex-col gap-5">
      {/* P1-3：org 聚焦态只统计直属成员，子部门未计入 → 醒目标注。 */}
      {focused && <DirectMembersNote />}

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        {platformError ? (
          <section className="glass rounded-2xl p-5">
            <h2 className="text-sm font-semibold text-gray-700 dark:text-gray-200 mb-3">AI 调用花费（平台·部门聚合）</h2>
            <PlatformNotConnected reason="error" detail={platformErrMsg ?? undefined} />
          </section>
        ) : (
          <DeptPlatformCostCard
            focused={focused}
            focusedAgg={focusQ.agg}
            items={items}
            loading={focused ? focusQ.loading : rankQ.loading}
            objectLabel={objectLabel || object}
          />
        )}
        <PersonDayCostCard subject="部门" />
      </div>

      {!platformError && (
        <PlatformWeekTrend
          title="AI 花费趋势（平台·部门聚合）"
          subtitle={focused ? `部门 · ${objectLabel || object} · 按周 · 仅直属成员` : '全部一级部门 · 按周 · 仅直属成员'}
          windows={series.windows}
          series={trendSeries}
          loading={series.loading}
          error={series.error}
          hasAny={series.hasAny}
          yFmt={(v) => `¥${shortToken(v)}`}
        />
      )}

      {/* P1-2：部门聚合按 Top 500 全量排行命中求和，区间真实人数更大时漏算排行外成员 → 醒目标注。 */}
      {!focused && !platformError && rankQ.truncated && <TruncationNote total={rankQ.rankingTotal} />}

      {!focused && !platformError && (
        <ChartCard title="部门 AI 花费排行（平台·直属成员聚合）" sub="区间聚合 · 一级部门 · 按花费倒序">
          <DeptCostRankingTable items={items} loading={rankQ.loading} />
        </ChartCard>
      )}
    </div>
  )
}

function DeptPlatformCostCard({
  focused,
  focusedAgg,
  items,
  loading,
  objectLabel,
}: {
  focused: boolean
  focusedAgg: DeptPlatformAgg | null
  items: DeptPlatformAgg[]
  loading: boolean
  objectLabel: string
}) {
  const cost = focused ? focusedAgg?.estimatedTotalCost ?? null : items.reduce((s, it) => s + (it.estimatedTotalCost || 0), 0)
  const tokens = focused ? focusedAgg?.sumTotalTokens ?? null : items.reduce((s, it) => s + (it.sumTotalTokens || 0), 0)
  const cacheTokens = focused ? focusedAgg?.sumCacheTokens ?? null : items.reduce((s, it) => s + (it.sumCacheTokens || 0), 0)

  return (
    <section className="glass rounded-2xl p-5 flex flex-col" style={{ borderLeft: '3px solid #af52de' }}>
      <div className="flex items-center justify-between mb-3 gap-3">
        <h2 className="text-sm font-semibold text-gray-700 dark:text-gray-200">AI 调用花费（平台·部门聚合）</h2>
        <span className="text-xs text-gray-400 dark:text-gray-500">{focused ? objectLabel : '全部部门合计'} · 仅直属成员</span>
      </div>
      {loading ? (
        <div className="grid grid-cols-3 gap-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="skeleton h-20 rounded-xl" />
          ))}
        </div>
      ) : focused && (!focusedAgg || focusedAgg.activePlatformUsers === 0) ? (
        <div className="flex-1 flex items-center justify-center py-8 text-sm text-gray-500 dark:text-gray-400">
          该部门直属成员在所选区间内无平台花费记录。
        </div>
      ) : (
        <div className="grid grid-cols-3 gap-3">
          <MetricCard label="AI 花费" value={fmtYuan(cost)} hint="estimated_total_cost 合计" />
          <MetricCard label="总 Token" value={shortToken(tokens)} hint={formatNumber(tokens)} />
          <MetricCard label="缓存 Token" value={shortToken(cacheTokens)} hint={formatNumber(cacheTokens)} />
        </div>
      )}
    </section>
  )
}

function DeptCostRankingTable({ items, loading }: { items: DeptPlatformAgg[]; loading: boolean }) {
  return (
    <div className="overflow-x-auto max-h-[520px] overflow-y-auto">
      <table className="w-full text-sm border-collapse">
        <thead className="sticky top-0 bg-white/70 dark:bg-gray-900/70 backdrop-blur">
          <tr className="border-b border-gray-200/50 dark:border-white/10">
            <th className={TH_NUM}>排名</th>
            <th className={TH}>部门</th>
            <th className={TH_NUM}>AI 花费（¥）</th>
            <th className={TH_NUM}>总 Token</th>
            <th className={TH_NUM}>活跃成员</th>
            <th className={TH_NUM}>请求数</th>
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
              <tr key={it.deptId || i} className="border-b border-gray-100/50 dark:border-white/5">
                <td className={TD_NUM}>{i + 1}</td>
                <td className={TD}>
                  <div className="max-w-[220px] truncate" title={it.deptName}>{it.deptName}</div>
                </td>
                <td className={TD_NUM}>{(it.estimatedTotalCost || 0).toFixed(2)}</td>
                <td className={TD_NUM} title={formatNumber(it.sumTotalTokens)}>
                  {shortToken(it.sumTotalTokens)}
                </td>
                <td className={TD_NUM}>{formatNumber(it.activePlatformUsers)}</td>
                <td className={TD_NUM}>{formatNumber(it.totalRequests)}</td>
              </tr>
            ))
          )}
        </tbody>
      </table>
    </div>
  )
}
