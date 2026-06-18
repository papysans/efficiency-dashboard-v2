// 「质量」维度内容（按 entity 分支接入平台「AI 服务健康度」初版）：user / org。
// ⚠️ 口径决策②：平台 error_rate 是「AI 服务错误率」（服务侧客观采集），不是代码质量。
//   页内固定一条醒目提示：当前口径=AI服务健康度，非代码质量；代码质量维度建设中。
//   KPI = 成功率 / 错误率 / 平均时延；时间线 = 错误率（或成功率）周序列（切窗）。
//   user 聚合态=各用户健康度排行；org 聚合态=各部门健康度排行（直属成员聚合）；聚焦态=该对象健康度 KPI。
// 降级护栏：chat_stats_enabled=false 或请求失败 → PlatformNotConnected。
//   project/repo 的质量维度无平台错误率口径 → 路由层走 QualityComingSoon（本组件不处理，防御性回退）。
import { useMemo } from 'react'
import { useNavigate } from 'react-router'
import { useGlobalConfig } from '@/api/queries'
import { useViewState } from '@/store/viewState'
import { useUserNameMap } from '@/hooks/useUserNameMap'
import { useEntityFocus } from '@/components/layout/EntityDimensionLayout'
import { MetricCard } from '@/components/ui/MetricCard'
import { formatNumber } from '@/lib/formatters'
import { ChartCard, ChatUserCell, EmptyHint } from '@/pages/platform/platformShared'
import { DimSkeleton, DirectMembersNote, PlatformNotConnected, PlatformWeekTrend, TruncationNote, useDeptFocus } from './platformDimShared'
import {
  useUserRanking,
  useUserRankingFocused,
  useUserWeekSeries,
  pickFocusedRow,
  errorRateOf,
  rowErrorRate,
  type ChatUserRankingRow,
} from './platformUserData'
import {
  useDeptPlatformRanking,
  useDeptPlatformFocused,
  useDeptWeekSeries,
  type DeptPlatformAgg,
} from './platformDeptData'

const TH = 'px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_NUM = 'px-3 py-2 text-right font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TD = 'px-3 py-2 align-middle text-gray-700 dark:text-gray-200 whitespace-nowrap'
const TD_NUM = 'px-3 py-2 align-middle text-right tabular-nums text-gray-700 dark:text-gray-200'

const fmtMs = (v: number | null | undefined) => (v != null ? `${Number(v).toFixed(0)} ms` : '-')
const fmtPct = (v: number | null | undefined) => (v != null ? `${(v * 100).toFixed(2)}%` : '-')

/** 口径醒目提示条（质量维度顶部固定，明确非代码质量）。 */
function CaliberNotice() {
  return (
    <div
      className="glass rounded-xl px-4 py-3 flex items-start gap-2 text-sm border-l-4"
      style={{ borderLeftColor: '#ff9500' }}
      role="note"
    >
      <svg className="w-5 h-5 shrink-0 text-amber-500" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
      </svg>
      <span className="text-gray-600 dark:text-gray-300">
        当前口径 = <b className="text-gray-900 dark:text-white">AI 服务健康度</b>（成功率 / 错误率 / 时延，平台客观采集），
        <b>非代码质量</b>。代码质量维度建设中。
      </span>
    </div>
  )
}

export default function QualityDimension() {
  const { entity, object, objectLabel } = useEntityFocus()
  const { timeRange } = useViewState()
  const { data: gc, isLoading: gcLoading } = useGlobalConfig()
  const configResolved = !!gc && !gcLoading
  const chatEnabled = gc?.chat_stats_enabled === true
  const focused = object !== ''

  // project/repo 质量维度无平台错误率口径（路由层为 QualityComingSoon）；防御性回退占位。
  if (entity !== 'user' && entity !== 'org') {
    return <PlatformNotConnected reason="disabled" detail="该主体的质量维度暂未接入平台数据。" />
  }
  if (!configResolved) {
    return (
      <div className="flex flex-col gap-5">
        <CaliberNotice />
        <DimSkeleton />
      </div>
    )
  }
  if (!chatEnabled) {
    return (
      <div className="flex flex-col gap-5">
        <CaliberNotice />
        <PlatformNotConnected reason="disabled" />
      </div>
    )
  }

  if (entity === 'org') {
    return <OrgQualityContent object={object} objectLabel={objectLabel} focused={focused} timeRange={timeRange} />
  }

  return (
    <QualityContent object={object} objectLabel={objectLabel} focused={focused} timeRange={timeRange} />
  )
}

function QualityContent({
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
  // 排行行下钻 → 看板用户详情（universal_id 与看板 user_id 同源）。
  const goToUser = (universalId: string) => navigate(`/user/${encodeURIComponent(universalId)}`)

  const series = useUserWeekSeries({ startDate: start, endDate: end, universalId: focused ? object : undefined }, true)
  // 健康度排行：按错误率倒序（最不健康在前，便于盯问题用户）。
  const rankQ = useUserRanking({ startDate: start, endDate: end, sortBy: 'error_rate', pageSize: 50 }, !focused)
  const focusQ = useUserRankingFocused({ startDate: start, endDate: end, universalId: object }, focused)
  const focusedRow = focused ? pickFocusedRow(focusQ.data, object) : null

  // 趋势：错误率周序列（百分比）。统一口径 error/(success+error)：聚焦=该用户单行重算；聚合=整窗 errorRate（已统一）。
  const trendSeries = useMemo(() => {
    const values = focused
      ? series.points.map((pt) => +(((pt.row ? rowErrorRate(pt.row) ?? 0 : 0) * 100).toFixed(2)))
      : series.windows.map((w) => +(((series.aggByKey.get(w.key)?.errorRate ?? 0) * 100).toFixed(2)))
    return [{ name: '错误率', color: '#ff3b30', values }]
  }, [focused, series.points, series.windows, series.aggByKey])

  const fatalError = (!focused && rankQ.error) || (focused && focusQ.error)
  if (fatalError) {
    const msg = ((rankQ.error || focusQ.error) as Error)?.message
    return (
      <div className="flex flex-col gap-5">
        <CaliberNotice />
        <PlatformNotConnected reason="error" detail={msg} />
      </div>
    )
  }

  const rows = rankQ.data?.data ?? []

  return (
    <div className="flex flex-col gap-5">
      <CaliberNotice />

      <PlatformWeekTrend
        title="错误率趋势（AI 服务健康度）"
        subtitle={focused ? `个人 · ${objectLabel || object} · 按周` : '全部用户 · 按周'}
        windows={series.windows}
        series={trendSeries}
        loading={series.loading}
        error={series.error}
        hasAny={series.hasAny}
        yFmt={(v) => `${v}%`}
      />

      {focused ? (
        <FocusedHealth row={focusedRow} loading={focusQ.isLoading} objectLabel={objectLabel || object} />
      ) : (
        <>
          <AggregateHealthKpis rows={rows} />
          {/* P1-2：错误率周序列按 Top 500 拉窗求和，区间真实人数更大时趋势被截断 → 醒目标注。 */}
          {series.truncated && <TruncationNote total={series.maxWindowTotal} />}
          <ChartCard title="用户健康度排行（AI 服务）" sub="区间聚合 · Top 50 · 按错误率倒序 · 点行下钻">
            <HealthRankingTable rows={rows} loading={rankQ.isFetching} resolveName={resolveName} onRowClick={goToUser} />
          </ChartCard>
        </>
      )}
    </div>
  )
}

function AggregateHealthKpis({ rows }: { rows: ChatUserRankingRow[] }) {
  const sum = (fn: (r: ChatUserRankingRow) => number) => rows.reduce((s, r) => s + (fn(r) || 0), 0)
  const total = sum((r) => r.total_requests)
  const success = sum((r) => r.success_requests)
  const errors = sum((r) => r.error_requests)
  // 统一口径：error/(success+error)（不含 total，对 total 含不含错误都稳健）。
  const errorRate = errorRateOf(success, errors)
  const successRate = errorRate != null ? 1 - errorRate : null
  // 平均时延：按请求数加权（更接近真实体验）。
  const weightedDur = sum((r) => (r.avg_duration_ms || 0) * (r.total_requests || 0))
  const avgDur = total > 0 ? weightedDur / total : null
  return (
    <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
      <MetricCard label="成功率" value={fmtPct(successRate)} tone="pos" hint="1 − 错误率" />
      <MetricCard label="错误率" value={fmtPct(errorRate)} tone={(errorRate ?? 0) > 0.05 ? 'neg' : 'neutral'} hint={`错误请求 ${formatNumber(errors)}`} />
      <MetricCard label="平均时延" value={fmtMs(avgDur)} hint="按请求数加权" />
      <MetricCard label="总请求（Top50）" value={formatNumber(total)} />
    </div>
  )
}

function FocusedHealth({
  row,
  loading,
  objectLabel,
}: {
  row: ChatUserRankingRow | null
  loading: boolean
  objectLabel: string
}) {
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
        该用户在所选区间内无平台调用记录（{objectLabel}）。
      </div>
    )
  }
  // 统一口径：用 success/error 重算（与聚合/周序列同一公式），不再用后端 row.error_rate。
  const errorRate = rowErrorRate(row)
  const successRate = errorRate != null ? 1 - errorRate : null
  return (
    <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
      <MetricCard label="成功率" value={fmtPct(successRate)} tone="pos" />
      <MetricCard label="错误率" value={fmtPct(errorRate)} tone={(errorRate ?? 0) > 0.05 ? 'neg' : 'neutral'} hint={`错误请求 ${formatNumber(row.error_requests)}`} />
      <MetricCard label="平均时延" value={fmtMs(row.avg_duration_ms)} />
      <MetricCard label="最大时延" value={fmtMs(row.max_duration_ms)} />
    </div>
  )
}

function HealthRankingTable({
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
            <th className={TH_NUM}>错误率</th>
            <th className={TH_NUM}>错误请求</th>
            <th className={TH_NUM}>平均时延</th>
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
            rows.map((u, i) => {
              // 统一口径：error/(success+error)（与聚合/聚焦/周序列同一公式），不用后端 u.error_rate。
              const errorRate = rowErrorRate(u)
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
                  <td className={`${TD_NUM} ${(errorRate ?? 0) > 0.05 ? 'text-rose-600 dark:text-rose-400' : ''}`}>
                    {fmtPct(errorRate)}
                  </td>
                  <td className={TD_NUM}>{formatNumber(u.error_requests)}</td>
                  <td className={TD_NUM}>{fmtMs(u.avg_duration_ms)}</td>
                </tr>
              )
            })
          )}
        </tbody>
      </table>
    </div>
  )
}

// ============================ 组织（org）：平台部门聚合 AI 服务健康度 ============================
function OrgQualityContent({
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
  // 部门行下钻 → 写 ?object=<dept_id> 进聚焦态。
  const goDept = useDeptFocus()

  const series = useDeptWeekSeries({ startDate: start, endDate: end, deptId: focused ? object : undefined }, true)
  const rankQ = useDeptPlatformRanking({ startDate: start, endDate: end }, !focused)
  const focusQ = useDeptPlatformFocused({ startDate: start, endDate: end, deptId: object }, focused)

  const trendSeries = useMemo(() => {
    const values = series.windows.map((w) => +(((series.aggByKey.get(w.key)?.errorRate ?? 0) * 100).toFixed(2)))
    return [{ name: '错误率', color: '#ff3b30', values }]
  }, [series.windows, series.aggByKey])

  const fatalError = (!focused && rankQ.error) || (focused && focusQ.error)
  if (fatalError) {
    const msg = (!focused ? rankQ.error : focusQ.error) ?? undefined
    return (
      <div className="flex flex-col gap-5">
        <CaliberNotice />
        <PlatformNotConnected reason="error" detail={msg ?? undefined} />
      </div>
    )
  }

  // 健康度排行：按错误率倒序（最不健康在前）。
  const items = (rankQ.items ?? []).slice().sort((a, b) => (b.errorRate ?? -Infinity) - (a.errorRate ?? -Infinity))

  return (
    <div className="flex flex-col gap-5">
      <CaliberNotice />

      <PlatformWeekTrend
        title="错误率趋势（AI 服务健康度·部门聚合）"
        subtitle={focused ? `部门 · ${objectLabel || object} · 按周 · 仅直属成员` : '全部一级部门 · 按周 · 仅直属成员'}
        windows={series.windows}
        series={trendSeries}
        loading={series.loading}
        error={series.error}
        hasAny={series.hasAny}
        yFmt={(v) => `${v}%`}
      />

      {focused ? (
        <>
          <DirectMembersNote />
          <FocusedDeptHealth agg={focusQ.agg} loading={focusQ.loading} objectLabel={objectLabel || object} />
        </>
      ) : (
        <>
          <DeptHealthKpis items={items} />
          {/* P1-2：部门聚合按 Top 500 全量排行命中求和，区间真实人数更大时漏算排行外成员 → 醒目标注。 */}
          {rankQ.truncated && <TruncationNote total={rankQ.rankingTotal} />}
          <ChartCard title="部门健康度排行（AI 服务·直属成员聚合）" sub="区间聚合 · 一级部门 · 按错误率倒序 · 点行下钻">
            <DeptHealthRankingTable items={items} loading={rankQ.loading} onRowClick={goDept} />
          </ChartCard>
        </>
      )}
    </div>
  )
}

function DeptHealthKpis({ items }: { items: DeptPlatformAgg[] }) {
  const totalReq = items.reduce((s, it) => s + (it.totalRequests || 0), 0)
  const errors = items.reduce((s, it) => s + (it.errorRequests || 0), 0)
  const denom = totalReq + errors
  const errorRate = denom > 0 ? errors / denom : null
  const successRate = errorRate != null ? 1 - errorRate : null
  // 平均时延：按请求数加权。
  const weightedDur = items.reduce((s, it) => s + (it.avgDurationMs || 0) * (it.totalRequests || 0), 0)
  const avgDur = totalReq > 0 ? weightedDur / totalReq : null
  return (
    <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
      <MetricCard label="成功率" value={fmtPct(successRate)} tone="pos" hint="1 − 错误率" />
      <MetricCard label="错误率" value={fmtPct(errorRate)} tone={(errorRate ?? 0) > 0.05 ? 'neg' : 'neutral'} hint={`错误请求 ${formatNumber(errors)}`} />
      <MetricCard label="平均时延" value={fmtMs(avgDur)} hint="按请求数加权" />
      <MetricCard label="总请求" value={formatNumber(totalReq)} />
    </div>
  )
}

function FocusedDeptHealth({ agg, loading, objectLabel }: { agg: DeptPlatformAgg | null; loading: boolean; objectLabel: string }) {
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
        该部门直属成员在所选区间内无平台调用记录（{objectLabel}）。
      </div>
    )
  }
  const successRate = agg.errorRate != null ? 1 - agg.errorRate : null
  return (
    <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
      <MetricCard label="成功率" value={fmtPct(successRate)} tone="pos" />
      <MetricCard label="错误率" value={fmtPct(agg.errorRate)} tone={(agg.errorRate ?? 0) > 0.05 ? 'neg' : 'neutral'} hint={`错误请求 ${formatNumber(agg.errorRequests)}`} />
      <MetricCard label="平均时延" value={fmtMs(agg.avgDurationMs)} />
      <MetricCard label="活跃成员" value={`${formatNumber(agg.activePlatformUsers)} / ${formatNumber(agg.memberCount)}`} hint="有平台调用 / 直属成员" />
    </div>
  )
}

function DeptHealthRankingTable({
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
            <th className={TH_NUM}>请求数</th>
            <th className={TH_NUM}>错误率</th>
            <th className={TH_NUM}>错误请求</th>
            <th className={TH_NUM}>平均时延</th>
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
                <td className={TD_NUM}>{formatNumber(it.totalRequests)}</td>
                <td className={`${TD_NUM} ${(it.errorRate ?? 0) > 0.05 ? 'text-rose-600 dark:text-rose-400' : ''}`}>
                  {fmtPct(it.errorRate)}
                </td>
                <td className={TD_NUM}>{formatNumber(it.errorRequests)}</td>
                <td className={TD_NUM}>{fmtMs(it.avgDurationMs)}</td>
              </tr>
            ))
          )}
        </tbody>
      </table>
    </div>
  )
}
