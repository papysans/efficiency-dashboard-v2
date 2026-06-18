// 「成本」维度内容（按 entity 分支）。口径决策①：user/org 两套并列卡，各自标源，口径别混（成本双源陷阱）：
//   ① AI 调用花费（平台·客观）= estimated_total_cost(¥) + tokens（shortToken）。
//   ② 人天成本（看板·折算）= 个人/部门人天成本**看板侧当前无数据 → 建设中占位**（不编造）。
//   project/repo → 平台无项目/仓库口径 → 看板费用**单卡**（KanbanCost，非双卡，显式注明）。
// 时间线 = 平台 AI 花费周序列（切窗）。聚合态=对象 AI 花费排行；聚焦态=该对象花费明细。
// 降级护栏：开关 false / 请求失败 → 平台卡显示「未接入平台」，看板人天卡照常（也是建设中）；不空页不抛错。
import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router'
import { useDeptCostTree } from './platformDeptCostTree'
import type { CostTreeNode } from './costTreeRollup'
import { useAllUsers, useGlobalConfig } from '@/api/queries'
import type { UserV2Row } from '@/api/types'
import { useViewState } from '@/store/viewState'
import { useUserNameMap } from '@/hooks/useUserNameMap'
import { useEntityFocus } from '@/components/layout/EntityDimensionLayout'
import { MetricCard } from '@/components/ui/MetricCard'
import { formatNumber } from '@/lib/formatters'
import { formatDateParam } from '@/lib/date'
import { ChartCard, ChatUserCell, EmptyHint, shortToken } from '@/pages/platform/platformShared'
import { DirectMembersNote, PlatformNotConnected, PlatformWeekTrend, TruncationNote, useDeptFocus } from './platformDimShared'
import {
  useUserRanking,
  useUserRankingFocused,
  useUserWeekSeries,
  pickFocusedRow,
  type ChatUserRankingRow,
} from './platformUserData'
import {
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

/**
 * 「成本效率 · 单位产出成本」卡（个人维度，替代无数据的人天建设中卡）。
 * 跨源口径：分子 = 平台 AI 花费(¥, estimated_total_cost)；分母 = 看板产出（合并需求 / 生成代码行）。
 *   ¥/需求 = 平台AI花费 ÷ 看板合并需求数
 *   ¥/千行 = 平台AI花费 ÷ 看板生成代码行 ×1000
 * 两源按 universal_id == 看板 user_id 关联（同源，已实测）。聚合=总量÷总量；聚焦=该用户÷该用户。
 * 分母 0 或缺数据 → 显 '-'（不编造）。明确标注跨源（平台¥=Token 调用花费，看板=代码侧产出）。
 */
function CostEfficiencyCard({
  cost,
  mergedNeeds,
  diffLines,
  loading,
  caption,
  emptyHint,
}: {
  cost: number | null
  mergedNeeds: number | null
  diffLines: number | null
  loading: boolean
  caption: string
  emptyHint?: string
}) {
  const perNeed = cost != null && mergedNeeds != null && mergedNeeds > 0 ? cost / mergedNeeds : null
  const perKLoc = cost != null && diffLines != null && diffLines > 0 ? (cost / diffLines) * 1000 : null
  const noData = !loading && perNeed == null && perKLoc == null

  return (
    <section className="glass rounded-2xl p-5 flex flex-col" style={{ borderLeft: '3px solid #0071e3' }}>
      <div className="flex items-center justify-between mb-3 gap-3">
        <h2 className="text-sm font-semibold text-gray-700 dark:text-gray-200">成本效率（单位产出成本）</h2>
        <span className="text-xs text-gray-400 dark:text-gray-500">{caption} · 跨源</span>
      </div>
      {loading ? (
        <div className="grid grid-cols-2 gap-3">
          {Array.from({ length: 2 }).map((_, i) => (
            <div key={i} className="skeleton h-20 rounded-xl" />
          ))}
        </div>
      ) : noData ? (
        <div className="flex-1 flex items-center justify-center py-8 text-center text-sm text-gray-500 dark:text-gray-400">
          {emptyHint ?? '所选区间内无可计算的产出（合并需求 / 代码行为 0 或缺数据）。'}
        </div>
      ) : (
        <div className="grid grid-cols-2 gap-3">
          <MetricCard
            label="¥ / 完成需求"
            value={perNeed != null ? fmtYuan(perNeed) : '-'}
            hint="平台AI花费 ÷ 看板合并需求数"
          />
          <MetricCard
            label="¥ / 千行生成代码"
            value={perKLoc != null ? fmtYuan(perKLoc) : '-'}
            hint="平台AI花费 ÷ 看板生成代码行 ×1000"
          />
        </div>
      )}
      <p className="mt-3 text-xs text-gray-400 dark:text-gray-500">
        跨源口径：分子 = <b className="text-gray-600 dark:text-gray-300">平台 AI 花费</b>（Token 调用花费），
        分母 = <b className="text-gray-600 dark:text-gray-300">看板产出</b>（合并需求 / 代码行），两源别混读。
      </p>
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
  // 个人：第二卡 = 成本效率（依赖平台¥分子）；部门：保留人天建设中卡（org 成本分支不动）。
  const SecondCard =
    entity === 'org' ? (
      <PersonDayCostCard subject={subject} />
    ) : (
      // 成本效率卡的分子是平台¥，平台未就绪/未接入 → 优雅占位（与平台卡同语气）。
      <section className="glass rounded-2xl p-5">
        <h2 className="text-sm font-semibold text-gray-700 dark:text-gray-200 mb-3">成本效率（单位产出成本）</h2>
        <PlatformNotConnected reason={configResolved ? 'disabled' : 'error'} />
      </section>
    )

  if (!configResolved) {
    return (
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <div className="skeleton h-48 rounded-2xl" />
        {entity === 'org' ? <PersonDayCostCard subject={subject} /> : <div className="skeleton h-48 rounded-2xl" />}
      </div>
    )
  }

  // 降级：开关 false → 平台卡占位，但看板侧第二卡按主体照常（org 人天 / 个人成本效率占位，平台缺位不连累看板侧布局）。
  if (!chatEnabled) {
    return (
      <div className="flex flex-col gap-5">
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          <section className="glass rounded-2xl p-5">
            <h2 className="text-sm font-semibold text-gray-700 dark:text-gray-200 mb-3">AI 调用花费（平台·客观）</h2>
            <PlatformNotConnected reason="disabled" />
          </section>
          {SecondCard}
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
  const navigate = useNavigate()
  // 排行行下钻 → 看板用户详情（universal_id 与看板 user_id 同源）。
  const goToUser = (universalId: string) => navigate(`/user/${encodeURIComponent(universalId)}`)

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

  // 成本效率卡的看板侧产出（分母）：区间全量用户行（与 UserContribution 同口径同获取方式）。
  //   聚合 = 全量求和；聚焦 = 取 user_id == object 的那行（universal_id 与看板 user_id 同源）。
  const kanbanUsersQ = useAllUsers({ startDate: formatDateParam(start), endDate: formatDateParam(end) })
  const kanbanOutput = useMemo(() => {
    const rows: UserV2Row[] = kanbanUsersQ.data ?? []
    if (focused) {
      const row = rows.find((r) => r.user_id === object) ?? null
      return {
        mergedNeeds: row ? row.merged_need_count : null,
        diffLines: row ? row.commit_diff_lines : null,
      }
    }
    return {
      mergedNeeds: rows.reduce((s, r) => s + (r.merged_need_count || 0), 0),
      diffLines: rows.reduce((s, r) => s + (r.commit_diff_lines || 0), 0),
    }
  }, [kanbanUsersQ.data, focused, object])

  // 趋势：AI 花费(¥)周序列。聚焦=该用户 estimated_total_cost；聚合=整窗 cost。
  const trendSeries = useMemo(() => {
    const values = focused
      ? series.points.map((pt) => +(pt.row?.estimated_total_cost ?? 0).toFixed(2))
      : series.windows.map((w) => +(series.aggByKey.get(w.key)?.cost ?? 0).toFixed(2))
    return [{ name: 'AI 花费', color: '#af52de', values }]
  }, [focused, series.points, series.windows, series.aggByKey])

  // 平台请求失败 → 平台两张卡都区域占位（成本效率卡的分子=平台¥，平台缺位则该卡也优雅占位）。
  const platformError = (!focused && rankQ.error) || (focused && focusQ.error)
  const platformErrMsg = ((rankQ.error || focusQ.error) as Error | undefined)?.message

  const rows = rankQ.data?.data ?? []

  // 成本效率卡分子（平台 AI 花费）：与相邻 PlatformCostCard 同口径 —— 聚焦=该用户；聚合=当前 Top50 合计。
  const platformCost = focused
    ? focusedRow?.estimated_total_cost ?? null
    : rows.reduce((s, r) => s + (r.estimated_total_cost || 0), 0)
  const costEffLoading = (focused ? focusQ.isLoading : rankQ.isFetching) || kanbanUsersQ.isLoading

  return (
    <div className="flex flex-col gap-5">
      {/* 双卡口径标注（顶部 KPI 行：平台 AI 花费汇总 ‖ 成本效率·单位产出成本） */}
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
        {platformError ? (
          <section className="glass rounded-2xl p-5">
            <h2 className="text-sm font-semibold text-gray-700 dark:text-gray-200 mb-3">成本效率（单位产出成本）</h2>
            <PlatformNotConnected reason="error" detail={platformErrMsg} />
          </section>
        ) : (
          <CostEfficiencyCard
            cost={platformCost}
            mergedNeeds={kanbanOutput.mergedNeeds}
            diffLines={kanbanOutput.diffLines}
            loading={costEffLoading}
            caption={focused ? objectLabel || object : 'Top50 合计 ÷ 区间产出'}
            emptyHint={
              focused
                ? '该用户在所选区间内无看板产出（合并需求 / 代码行为 0 或未关联）。'
                : undefined
            }
          />
        )}
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
          sub={`区间聚合 · Top 50${rankQ.data?.total != null ? ` · 共 ${formatNumber(rankQ.data.total)} 人` : ''} · 点行下钻`}
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
          <CostRankingTable rows={rows} loading={rankQ.isFetching} resolveName={resolveName} onRowClick={goToUser} />
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
                  <td className={TD_NUM}>{u.estimated_total_cost.toFixed(2)}</td>
                  <td className={TD_NUM} title={formatNumber(u.sum_total_tokens)}>
                    {shortToken(u.sum_total_tokens)}
                  </td>
                  <td className={TD_NUM} title={formatNumber(u.sum_cache_tokens)}>
                    {shortToken(u.sum_cache_tokens)}
                  </td>
                  <td className={TD_NUM}>{formatNumber(u.total_requests)}</td>
                </tr>
              )
            })
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
  // 部门行下钻 → 写 ?object=<dept_id> 进聚焦态。
  const goDept = useDeptFocus()

  const series = useDeptWeekSeries({ startDate: start, endDate: end, deptId: focused ? object : undefined }, true)
  // 聚合态：成本树（整棵 dept-tree 各部门子树成本 rollup，替代一级部门平铺排行）。聚焦态不需要 → 不拉。
  const treeQ = useDeptCostTree({ startDate: start, endDate: end }, !focused)
  const focusQ = useDeptPlatformFocused({ startDate: start, endDate: end, deptId: object }, focused)

  const trendSeries = useMemo(
    () => [
      { name: 'AI 花费', color: '#af52de', values: series.windows.map((w) => +(series.aggByKey.get(w.key)?.cost ?? 0).toFixed(2)) },
    ],
    [series.windows, series.aggByKey],
  )

  const platformError = (!focused && treeQ.error) || (focused && focusQ.error)
  const platformErrMsg = (!focused ? treeQ.error : focusQ.error) ?? undefined

  // 聚合态 KPI 卡：树根（公司）子树合计 = 全树平台 AI 花费总额。
  const aggCost = useMemo(() => treeQ.nodes.reduce((s, n) => s + n.subtreeCost, 0), [treeQ.nodes])
  const aggActive = useMemo(() => treeQ.nodes.reduce((s, n) => s + n.subtreeActive, 0), [treeQ.nodes])

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
        ) : focused ? (
          <DeptPlatformCostCard focusedAgg={focusQ.agg} loading={focusQ.loading} objectLabel={objectLabel || object} />
        ) : (
          // 聚合态 KPI：树根（公司）子树合计 = 全树平台 AI 花费总额（含所有子部门 rollup）。
          <DeptTreeCostCard cost={aggCost} activeMembers={aggActive} loading={treeQ.loading} />
        )}
        <PersonDayCostCard subject="部门" />
      </div>

      {!platformError && (
        <PlatformWeekTrend
          title="AI 花费趋势（平台·部门聚合）"
          subtitle={focused ? `部门 · ${objectLabel || object} · 按周 · 仅直属成员` : '全部部门 · 按周 · 仅直属成员'}
          windows={series.windows}
          series={trendSeries}
          loading={series.loading}
          error={series.error}
          hasAny={series.hasAny}
          yFmt={(v) => `¥${shortToken(v)}`}
        />
      )}

      {/* P1-2：直属成本由全量排行 Top 500 命中求和，区间真实人数更大时漏算排行外成员 → 子树成本整体偏小，醒目标注。 */}
      {!focused && !platformError && treeQ.truncated && <TruncationNote total={treeQ.rankingTotal} />}

      {!focused && !platformError && (
        <ChartCard
          title="部门 AI 花费成本树（平台·子树递归 rollup）"
          sub={`部门层级 · 节点 = 其子树所有成员平台 AI 花费合计 · 点节点下钻 · 拉取 ${treeQ.deptRequestCount} 个部门花名册`}
        >
          <CostTree nodes={treeQ.nodes} loading={treeQ.loading} onSelect={goDept} />
        </ChartCard>
      )}
    </div>
  )
}

// ============================ 成本树（聚合态）：可展开/折叠的部门层级，节点显子树成本¥ ============================
/**
 * 复用 OrgTree 的树渲染范式（缩进 + 展开按钮 + 懒渲染折叠子树），但聚焦成本：每节点显**子树成本¥**（递归 rollup）。
 * 不直接复用 OrgTree（它绑定 DeptMembersPanel/选中态/URL 探索语义，且每行无成本列）→ 在此建一个聚焦成本的版本。
 * 展开态用本地 state；点节点名 → onSelect(dept_id) 写 ?object= 进聚焦态（复用 useDeptFocus）。
 */
function CostTree({
  nodes,
  loading,
  onSelect,
}: {
  nodes: CostTreeNode[]
  loading: boolean
  onSelect: (deptId: string) => void
}) {
  // 默认展开第一层（根/一级部门），其余折叠 —— 与 OrgTree 初始态一致语气。
  const initialOpen = useMemo(() => {
    const s = new Set<string>()
    for (const n of nodes) if (n.deptId) s.add(n.deptId)
    return s
  }, [nodes])
  const [expanded, setExpanded] = useState<Set<string>>(initialOpen)
  // 树到达/切换时重置默认展开（仅当当前为空，避免覆盖用户手动展开）。
  useEffect(() => {
    setExpanded((prev) => (prev.size > 0 ? prev : initialOpen))
  }, [initialOpen])

  const toggle = (id: string) =>
    setExpanded((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })

  // 同层按子树成本倒序（与原排行倒序口径一致）。
  const sorted = useMemo(() => nodes.slice().sort((a, b) => b.subtreeCost - a.subtreeCost), [nodes])

  if (loading && nodes.length === 0) {
    return (
      <div className="space-y-2 p-2">
        {Array.from({ length: 6 }).map((_, i) => (
          <div key={i} className="skeleton h-8 rounded" />
        ))}
      </div>
    )
  }
  if (nodes.length === 0) {
    return <EmptyHint compact />
  }

  return (
    <div className="max-h-[560px] overflow-y-auto">
      <ul role="tree" aria-label="部门成本树" className="list-none m-0 p-0">
        {sorted.map((n) => (
          <CostTreeRow key={n.deptId} node={n} expanded={expanded} onToggle={toggle} onSelect={onSelect} />
        ))}
      </ul>
    </div>
  )
}

function CostTreeRow({
  node,
  expanded,
  onToggle,
  onSelect,
}: {
  node: CostTreeNode
  expanded: Set<string>
  onToggle: (id: string) => void
  onSelect: (deptId: string) => void
}) {
  const isOpen = expanded.has(node.deptId)
  // 同层子节点按子树成本倒序。
  const children = useMemo(
    () => node.children.slice().sort((a, b) => b.subtreeCost - a.subtreeCost),
    [node.children],
  )

  return (
    <li role="treeitem" aria-expanded={node.hasChildren ? isOpen : undefined}>
      <div
        className="flex items-center gap-1 rounded-lg pr-2 py-1.5 transition-colors text-gray-700 dark:text-gray-200 hover:bg-white/50 dark:hover:bg-white/10"
        style={{ paddingLeft: `${node.depth * 16 + 8}px` }}
      >
        {node.hasChildren && node.children.length > 0 ? (
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation()
              onToggle(node.deptId)
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
          onClick={() => node.deptId && onSelect(node.deptId)}
          disabled={!node.deptId}
          className="flex-1 min-w-0 inline-flex items-center justify-between gap-3 text-left text-sm bg-transparent border-none p-0 cursor-pointer text-inherit focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue rounded disabled:cursor-default"
        >
          <span className="truncate font-medium" title={node.deptName}>{node.deptName}</span>
          <span className="shrink-0 inline-flex items-center gap-2 tabular-nums">
            <span
              className="text-xs px-1.5 py-0.5 rounded-full bg-gray-100 dark:bg-white/10 text-gray-500 dark:text-gray-400"
              title={`子树成员 ${node.subtreeMembers} 人，其中活跃 ${node.subtreeActive} 人`}
            >
              {node.subtreeActive}/{node.subtreeMembers}人
            </span>
            <span className="font-semibold text-gray-900 dark:text-white" title={`子树合计 ¥${node.subtreeCost.toFixed(2)}`}>
              ¥{shortToken(node.subtreeCost)}
            </span>
          </span>
        </button>
      </div>
      {/* 懒渲染：折叠节点不渲染子节点 DOM（大树不一次性铺 DOM）。 */}
      {node.hasChildren && isOpen && children.length > 0 ? (
        <ul role="group" className="list-none m-0 p-0">
          {children.map((ch) => (
            <CostTreeRow key={ch.deptId} node={ch} expanded={expanded} onToggle={onToggle} onSelect={onSelect} />
          ))}
        </ul>
      ) : null}
    </li>
  )
}

/** 聚合态 KPI 卡：成本树根合计（全树平台 AI 花费总额）。 */
function DeptTreeCostCard({ cost, activeMembers, loading }: { cost: number; activeMembers: number; loading: boolean }) {
  return (
    <section className="glass rounded-2xl p-5 flex flex-col" style={{ borderLeft: '3px solid #af52de' }}>
      <div className="flex items-center justify-between mb-3 gap-3">
        <h2 className="text-sm font-semibold text-gray-700 dark:text-gray-200">AI 调用花费（平台·全树合计）</h2>
        <span className="text-xs text-gray-400 dark:text-gray-500">全部部门子树 rollup · Token 调用花费</span>
      </div>
      {loading ? (
        <div className="grid grid-cols-2 gap-3">
          {Array.from({ length: 2 }).map((_, i) => (
            <div key={i} className="skeleton h-20 rounded-xl" />
          ))}
        </div>
      ) : (
        <div className="grid grid-cols-2 gap-3">
          <MetricCard label="AI 花费合计" value={fmtYuan(cost)} hint="全树 estimated_total_cost rollup" />
          <MetricCard label="活跃成员" value={formatNumber(activeMembers)} hint="区间内有平台记录的成员数" />
        </div>
      )}
    </section>
  )
}

/** 组织聚焦态 KPI 卡：单部门直属成员平台花费合计（成本树点节点 → ?object= → 此卡）。 */
function DeptPlatformCostCard({
  focusedAgg,
  loading,
  objectLabel,
}: {
  focusedAgg: DeptPlatformAgg | null
  loading: boolean
  objectLabel: string
}) {
  const cost = focusedAgg?.estimatedTotalCost ?? null
  const tokens = focusedAgg?.sumTotalTokens ?? null
  const cacheTokens = focusedAgg?.sumCacheTokens ?? null

  return (
    <section className="glass rounded-2xl p-5 flex flex-col" style={{ borderLeft: '3px solid #af52de' }}>
      <div className="flex items-center justify-between mb-3 gap-3">
        <h2 className="text-sm font-semibold text-gray-700 dark:text-gray-200">AI 调用花费（平台·部门聚合）</h2>
        <span className="text-xs text-gray-400 dark:text-gray-500">{objectLabel} · 仅直属成员</span>
      </div>
      {loading ? (
        <div className="grid grid-cols-3 gap-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="skeleton h-20 rounded-xl" />
          ))}
        </div>
      ) : !focusedAgg || focusedAgg.activePlatformUsers === 0 ? (
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
