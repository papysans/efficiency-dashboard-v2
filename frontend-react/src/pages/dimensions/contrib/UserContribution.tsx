// 「个人 · 贡献」维度内容（看板派生，不接平台 / chatStats）。
// 口径决策⑤：贡献用看板派生（提交 / 代码行 / 合并需求），平台 tokens=消耗量≠贡献，**故本组件零平台请求**。
//
// 两态由壳的聚焦对象（useEntityFocus().object）决定：
//   聚合态(object 空)：顶部 KPI（总合并需求 / 总代码行 / 总提交） + 用户贡献排行表
//     （用户 / 合并需求 / 代码行 / 提交 / AI 占比），点行下钻进 /user/:id 独立详情，或在壳里选对象进聚焦态。
//   聚焦态(object=某 userId)：复用 UserDetail（embedded），它已含个人合并需求 / 代码行 / 提交 / 周明细 / 关联 Need / Commit。
//
// 时间线：贡献维度**有现成的「按周贡献」时序**——/v2/efficiency 是 用户×周 聚合行（UserProductivityV2），
//   每行带 week_start + 贡献计数（merged_need_count / commit_diff_lines / commit_count）。本组件复用它画
//   按周贡献趋势（ContributionTrend，多系列：合并需求 / 提交 / 代码行）。
//     聚合态(无 object)：拉全量周行（不传 userId）→ 按 week_start 分桶**求和** → 每周公司级贡献趋势。
//     聚焦态(object=userId)：拉该用户周行（传 userId）→ 该用户按周贡献趋势（与 UserDetail 自带周趋势互补，口径一致）。
//   不复用 DimensionTrend：那是 efficiency_ratio **平均/百分比/单系列/单 Y 轴** 口径，硬塞计数会破坏其 6 个调用方；
//   贡献是 **求和/计数/多系列**，且 commit_diff_lines 量级远大于需求/提交数 → 需双 Y 轴。故用 platformShared 同源的
//   低层原语（getPalette + EChart + ISO 周工具）自建一个计数趋势，空态文案与 DimensionTrend 对齐（积累中 / 单周提示）。
//
// 数据：全局时间范围（useViewState().timeRange）→ getAllUsersV2（用户列表全量），与 UserList 同口径同获取方式。
//   用户名解析用 useUserNameMap（user_name 多为 UUID，用 commits 的 git_user_name 兜底）。
//   口径：ai_code_ratio / calendar_ratio / work_ratio 均为**小数口径** → RatioPill（×100）；merged/commit/diff 为计数 → formatNumber。
import { useMemo } from 'react'
import { useNavigate } from 'react-router'
import * as echarts from 'echarts'
import type { EChartsOption } from 'echarts'
import { useAllUsers, useEfficiencyV2 } from '@/api/queries'
import type { UserV2Row, UserProductivityV2 } from '@/api/types'
import { useViewState } from '@/store/viewState'
import { useUserNameMap } from '@/hooks/useUserNameMap'
import { useEntityFocus } from '@/components/layout/EntityDimensionLayout'
import { useTheme } from '@/hooks/useTheme'
import { MetricCard } from '@/components/ui/MetricCard'
import { RatioPill } from '@/components/ui/RatioPill'
import { EChart } from '@/components/charts/EChart'
import { getPalette } from '@/components/charts/chartTheme'
import { ChartCard, EmptyHint } from '@/pages/platform/platformShared'
// 注：贡献趋势是求和/计数/多系列 + 双 Y 轴，与 DimensionTrend（平均/百分比/单系列）口径不同 →
//   不复用 DimensionTrend，下方自建 ContributionTrend（同源低层原语 EChart + getPalette + ISO 周工具）。
import { formatNumber } from '@/lib/formatters'
import { formatDateParam } from '@/lib/date'
import { isoWeekOf, weekLabel } from '@/lib/week'
import { sortRows } from '@/lib/sort'
import UserDetail from '@/pages/users/UserDetail'

const TH = 'px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_NUM = 'px-3 py-2 text-right font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_CENTER = 'px-3 py-2 text-center font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TD = 'px-3 py-2 align-middle text-gray-700 dark:text-gray-200 whitespace-nowrap'
const TD_NUM = 'px-3 py-2 align-middle text-right tabular-nums text-gray-700 dark:text-gray-200'

/** 显示名截断（>20 字 …），入参为已解析真实名（对齐 UserList shortName）。 */
function shortName(name: string): string {
  const n = name || '-'
  return n.length > 20 ? `${n.slice(0, 20)}…` : n
}

// ---- 按周贡献趋势（看板派生 · /v2/efficiency 周表）----

interface ContribWeekPoint {
  key: string
  label: string
  monday: number
  merged: number
  commits: number
  diffLines: number
}

/** 周表行（user×week）按 ISO 周分桶**求和**贡献计数（合并需求 / 提交 / 代码行）。 */
function aggregateContribByWeek(rows: UserProductivityV2[]): ContribWeekPoint[] {
  const buckets = new Map<string, ContribWeekPoint>()
  for (const r of rows) {
    const wk = isoWeekOf(r.week_start)
    if (!wk) continue
    const cur =
      buckets.get(wk.key) ||
      { key: wk.key, label: weekLabel(wk.monday), monday: wk.monday.getTime(), merged: 0, commits: 0, diffLines: 0 }
    cur.merged += r.merged_need_count || 0
    cur.commits += r.commit_count || 0
    cur.diffLines += r.commit_diff_lines || 0
    buckets.set(wk.key, cur)
  }
  return Array.from(buckets.values()).sort((a, b) => a.monday - b.monday)
}

/**
 * 通用「按周贡献趋势」卡片（看板派生，多系列计数）。
 * 量级问题：commit_diff_lines 远大于合并需求 / 提交数 → 代码行走**第二 Y 轴**，避免压扁需求/提交两条线。
 * 空态文案与 DimensionTrend 对齐（unavailable / 积累中 / 单周提示），不编造数据。
 */
function ContributionTrend({
  rows,
  loading = false,
  error = null,
  title = '贡献趋势',
  subtitle,
}: {
  rows: UserProductivityV2[] | undefined
  loading?: boolean
  error?: string | null
  title?: string
  subtitle?: string
}) {
  const { theme } = useTheme()
  const points = useMemo(() => aggregateContribByWeek(rows ?? []), [rows])

  const option = useMemo<EChartsOption>(() => {
    const p = getPalette(theme)
    const COLOR_MERGED = '#0071e3' // 合并需求（主色）
    const COLOR_COMMITS = '#34c759' // 提交
    const COLOR_DIFF = '#ff9500' // 代码行（第二 Y 轴）
    return {
      animation: true,
      grid: { left: 8, right: 16, top: 36, bottom: 8, containLabel: true },
      tooltip: { trigger: 'axis', backgroundColor: p.tooltipBg, borderColor: p.tooltipBorder, borderWidth: 1, textStyle: { color: p.tooltipText } },
      legend: { top: 0, left: 'center', textStyle: { color: p.textColor }, itemWidth: 14, itemHeight: 8 },
      xAxis: {
        type: 'category',
        data: points.map((pt) => pt.label),
        boundaryGap: false,
        axisLine: { lineStyle: { color: p.axisColor } },
        axisLabel: { color: p.textColor, hideOverlap: true },
        axisTick: { show: false },
      },
      yAxis: [
        {
          type: 'value',
          name: '需求 / 提交',
          nameTextStyle: { color: p.textColor, fontSize: 11 },
          axisLabel: { color: p.textColor },
          splitLine: { lineStyle: { color: p.splitLineColor } },
        },
        {
          type: 'value',
          name: '代码行',
          nameTextStyle: { color: p.textColor, fontSize: 11 },
          axisLabel: { color: p.textColor, formatter: (v: number) => formatNumber(v, 0) },
          splitLine: { show: false },
        },
      ],
      series: [
        {
          name: '合并需求',
          type: 'line',
          yAxisIndex: 0,
          smooth: true,
          symbol: 'circle',
          symbolSize: 6,
          data: points.map((pt) => pt.merged),
          lineStyle: { color: COLOR_MERGED, width: 3 },
          itemStyle: { color: COLOR_MERGED },
          areaStyle: {
            color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
              { offset: 0, color: p.areaTop },
              { offset: 1, color: p.areaBottom },
            ]),
          },
        },
        {
          name: '提交',
          type: 'line',
          yAxisIndex: 0,
          smooth: true,
          symbol: 'circle',
          symbolSize: 6,
          data: points.map((pt) => pt.commits),
          lineStyle: { color: COLOR_COMMITS, width: 2 },
          itemStyle: { color: COLOR_COMMITS },
        },
        {
          name: '代码行',
          type: 'line',
          yAxisIndex: 1,
          smooth: true,
          symbol: 'circle',
          symbolSize: 6,
          data: points.map((pt) => pt.diffLines),
          lineStyle: { color: COLOR_DIFF, width: 2, type: 'dashed' },
          itemStyle: { color: COLOR_DIFF },
        },
      ],
    }
  }, [points, theme])

  return (
    <div className="glass rounded-2xl p-5 md:p-6 min-h-[20rem] hover:shadow-lg transition-shadow flex flex-col">
      <div className="flex items-center justify-between mb-4 gap-3">
        <h2 className="text-sm font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide">{title}</h2>
        {subtitle && <span className="text-xs text-gray-400 dark:text-gray-500 text-right">{subtitle}</span>}
      </div>

      {error ? (
        <TrendCentered>加载失败：{error}</TrendCentered>
      ) : loading ? (
        <div className="flex-1 skeleton rounded-xl min-h-[16rem]" />
      ) : points.length < 2 ? (
        <TrendCentered>
          <div className="flex flex-col items-center gap-2 text-center">
            <TrendIcon />
            <p className="text-sm text-gray-500 dark:text-gray-400">趋势数据积累中（当前样本较少）</p>
            {points.length === 1 && (
              <p className="text-xs text-gray-400 dark:text-gray-500">
                本期数据集中在单周（{points[0].label} 起 · 合并需求 {formatNumber(points[0].merged)} · 提交{' '}
                {formatNumber(points[0].commits)} · 代码行 {formatNumber(points[0].diffLines, 0)}）
              </p>
            )}
          </div>
        </TrendCentered>
      ) : (
        <div className="flex-1">
          <EChart option={option} height={260} />
        </div>
      )}
    </div>
  )
}

function TrendCentered({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex-1 flex items-center justify-center text-sm text-gray-500 dark:text-gray-400 min-h-[16rem]">
      {children}
    </div>
  )
}

function TrendIcon() {
  return (
    <svg className="w-10 h-10 text-gray-300 dark:text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M3 13.125C3 12.504 3.504 12 4.125 12h2.25c.621 0 1.125.504 1.125 1.125v6.75C7.5 20.496 6.996 21 6.375 21h-2.25A1.125 1.125 0 013 19.875v-6.75zM9.75 8.625c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125v11.25c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V8.625zM16.5 4.125c0-.621.504-1.125 1.125-1.125h2.25C20.496 3 21 3.504 21 4.125v15.75c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V4.125z" />
    </svg>
  )
}

export default function UserContribution() {
  const { object, objectLabel } = useEntityFocus()
  const focused = object !== ''

  // 聚焦态：复用 UserDetail（embedded）—— 个人贡献明细（合并需求/代码行/提交/周趋势/关联 Need·Commit）由它给出。
  // 壳已有标题/面包屑/对象选择器，embedded 去掉返回与外标题。聚焦态不另放顶部时间线（UserDetail 自带周趋势）。
  if (focused) {
    return <FocusedContribution object={object} objectLabel={objectLabel} />
  }
  return <AggregateContribution />
}

/** 聚焦态壳：贡献专属周趋势（顶部，该用户）+ 诚实说明 + UserDetail（embedded，复用其全部贡献明细）。 */
function FocusedContribution({ object, objectLabel }: { object: string; objectLabel: string }) {
  const { timeRange } = useViewState()
  const dateParams = useMemo(
    () => ({ startDate: formatDateParam(timeRange[0]), endDate: formatDateParam(timeRange[1]), userId: object }),
    [timeRange, object],
  )
  // 该用户周表行（/v2/efficiency 支持 userId 过滤）→ 顶部贡献周趋势，与聚合态口径一致。
  const trendQ = useEfficiencyV2(dateParams, object !== '')
  return (
    <div className="flex flex-col gap-4">
      <ContributionTrend
        rows={trendQ.data?.data}
        loading={trendQ.isLoading}
        error={trendQ.error ? (trendQ.error as Error).message : null}
        subtitle={`个人 · ${objectLabel || object} · 按 ISO 周 · 看板派生`}
      />
      <p className="text-xs text-gray-400 dark:text-gray-500">
        贡献口径 = <b className="font-medium text-gray-600 dark:text-gray-300">看板派生</b>（合并需求 / 代码行 / 提交）。
        平台（chat-stats）的 tokens 为消耗量≠贡献，故本维度不接入平台。下方为 {objectLabel || object} 的个人贡献明细。
      </p>
      <UserDetail userIdProp={object} dateRangeProp={timeRange} embedded />
    </div>
  )
}

/** 聚合态：KPI（总合并需求/总代码行/总提交） + 用户贡献排行表（点行下钻 /user/:id）。 */
function AggregateContribution() {
  const navigate = useNavigate()
  const { timeRange } = useViewState()
  const { resolveName } = useUserNameMap()

  const dateParams = useMemo(
    () => ({ startDate: formatDateParam(timeRange[0]), endDate: formatDateParam(timeRange[1]) }),
    [timeRange],
  )
  // 与 UserList 同获取方式：翻页拉全（绕过服务端切片截断），全量在客户端排序/汇总。
  const { data, isLoading, error } = useAllUsers(dateParams)
  const rows = useMemo<UserV2Row[]>(() => data ?? [], [data])

  // 按周贡献趋势：/v2/efficiency 不传 userId = 全量 用户×周 行 → 按周求和（与排行/KPI 互补，趋势是时序维）。
  const trendQ = useEfficiencyV2(dateParams)

  // 贡献排行：按合并需求倒序（null 沉底）—— 贡献维度以「交付的需求」为首要排序键。
  const ranked = useMemo(() => sortRows(rows, (r) => r.merged_need_count, true), [rows])

  // KPI：总合并需求 / 总代码行 / 总提交（+ 贡献人数做上下文）。
  const kpi = useMemo(() => {
    const contributors = rows.length
    const merged = rows.reduce((s, r) => s + (r.merged_need_count || 0), 0)
    const diffLines = rows.reduce((s, r) => s + (r.commit_diff_lines || 0), 0)
    const commits = rows.reduce((s, r) => s + (r.commit_count || 0), 0)
    return { contributors, merged, diffLines, commits }
  }, [rows])

  function goToDetail(row: UserV2Row) {
    if (!row?.user_id) return
    navigate({
      pathname: `/user/${encodeURIComponent(row.user_id)}`,
      search: `?${new URLSearchParams(dateParams).toString()}`,
    })
  }

  if (error) {
    return (
      <div className="glass rounded-2xl p-8 text-center text-sm text-rose-600 dark:text-rose-400">
        {(error as Error).message || '获取个人贡献数据失败'}
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-5">
      {/* 时间线：复用 /v2/efficiency 周表（用户×周）按周求和 → 公司级按周贡献趋势（看板派生）。 */}
      <ContributionTrend
        rows={trendQ.data?.data}
        loading={trendQ.isLoading}
        error={trendQ.error ? (trendQ.error as Error).message : null}
        subtitle="全部用户 · 按 ISO 周 · 看板派生"
      />

      <p className="text-xs text-gray-400 dark:text-gray-500">
        贡献口径 = <b className="font-medium text-gray-600 dark:text-gray-300">看板派生</b>（合并需求 / 代码行 / 提交）。
        平台（chat-stats）的 tokens 为消耗量≠贡献，故本维度不接入平台。
      </p>

      {/* KPI */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        <MetricCard label="贡献人数" value={formatNumber(kpi.contributors)} hint="可计入贡献的用户数" />
        <MetricCard label="合并需求总数" value={formatNumber(kpi.merged)} />
        <MetricCard label="代码行总数" value={formatNumber(kpi.diffLines)} hint="commit diff 行合计" />
        <MetricCard label="提交总数" value={formatNumber(kpi.commits)} />
      </div>

      {/* 排行表 */}
      <ChartCard title="个人贡献排行（看板派生）" sub="按合并需求倒序，点行查看个人明细">
        <div className="overflow-x-auto max-h-[560px] overflow-y-auto">
          <table className="w-full text-sm border-collapse">
            <thead className="sticky top-0 bg-white/70 dark:bg-gray-900/70 backdrop-blur">
              <tr className="border-b border-gray-200/50 dark:border-white/10">
                <th className={TH_NUM}>排名</th>
                <th className={TH}>用户</th>
                <th className={TH_NUM}>合并需求</th>
                <th className={TH_NUM}>代码行</th>
                <th className={TH_NUM}>提交</th>
                <th className={TH_CENTER}>AI 代码占比</th>
              </tr>
            </thead>
            <tbody>
              {isLoading ? (
                <SkeletonRows cols={6} />
              ) : ranked.length === 0 ? (
                <tr>
                  <td colSpan={6}>
                    <EmptyHint compact />
                  </td>
                </tr>
              ) : (
                ranked.map((r, i) => (
                  <tr
                    key={r.user_id}
                    onClick={() => goToDetail(r)}
                    className="border-b border-gray-100/50 dark:border-white/5 cursor-pointer hover:bg-apple-blue/5 dark:hover:bg-white/5 transition-colors"
                  >
                    <td className={TD_NUM}>{i + 1}</td>
                    <td className={TD}>
                      <button
                        type="button"
                        className="text-apple-blue hover:text-apple-blue-hover cursor-pointer bg-transparent border-none p-0 focus:outline-none focus-visible:underline"
                        title={resolveName(r.user_id)}
                        onClick={(e) => {
                          e.stopPropagation()
                          goToDetail(r)
                        }}
                      >
                        {shortName(resolveName(r.user_id))}
                      </button>
                    </td>
                    <td className={TD_NUM}>{formatNumber(r.merged_need_count)}</td>
                    <td className={TD_NUM}>{formatNumber(r.commit_diff_lines, 0)}</td>
                    <td className={TD_NUM}>{formatNumber(r.commit_count)}</td>
                    <td className="px-3 py-2 align-middle text-center">
                      <RatioPill value={r.ai_code_ratio ?? null} />
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </ChartCard>
    </div>
  )
}

function SkeletonRows({ cols }: { cols: number }) {
  return (
    <>
      {Array.from({ length: 6 }).map((_, i) => (
        <tr key={i} className="border-b border-gray-100/50 dark:border-white/5">
          <td colSpan={cols} className="px-3 py-2">
            <div className="skeleton h-6 rounded" />
          </td>
        </tr>
      ))}
    </>
  )
}
