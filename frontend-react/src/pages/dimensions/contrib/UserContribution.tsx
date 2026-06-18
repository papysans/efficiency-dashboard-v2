// 「个人 · 贡献」维度内容（看板派生，不接平台 / chatStats）。
// 口径决策⑤：贡献用看板派生（提交 / 代码行 / 合并需求），平台 tokens=消耗量≠贡献，**故本组件零平台请求**。
//
// 两态由壳的聚焦对象（useEntityFocus().object）决定：
//   聚合态(object 空)：顶部 KPI（总合并需求 / 总代码行 / 总提交） + 用户贡献排行表
//     （用户 / 合并需求 / 代码行 / 提交 / AI 占比），点行下钻进 /user/:id 独立详情，或在壳里选对象进聚焦态。
//   聚焦态(object=某 userId)：复用 UserDetail（embedded），它已含个人合并需求 / 代码行 / 提交 / 周明细 / 关联 Need / Commit。
//
// 时间线：贡献维度在聚合态**无现成的「按周贡献」时序**（getAllUsersV2 是 用户×区间 聚合，无周桶）→ 走
//   DimensionTrend 的诚实空态（unavailable），不编造数据。聚焦态由 UserDetail 自带周趋势图，故聚焦态不再额外放顶部时间线。
//
// 数据：全局时间范围（useViewState().timeRange）→ getAllUsersV2（用户列表全量），与 UserList 同口径同获取方式。
//   用户名解析用 useUserNameMap（user_name 多为 UUID，用 commits 的 git_user_name 兜底）。
//   口径：ai_code_ratio / calendar_ratio / work_ratio 均为**小数口径** → RatioPill（×100）；merged/commit/diff 为计数 → formatNumber。
import { useMemo } from 'react'
import { useNavigate } from 'react-router'
import { useAllUsers } from '@/api/queries'
import type { UserV2Row } from '@/api/types'
import { useViewState } from '@/store/viewState'
import { useUserNameMap } from '@/hooks/useUserNameMap'
import { useEntityFocus } from '@/components/layout/EntityDimensionLayout'
import { MetricCard } from '@/components/ui/MetricCard'
import { RatioPill } from '@/components/ui/RatioPill'
import { DimensionTrend } from '@/components/executive/DimensionTrend'
import { ChartCard, EmptyHint } from '@/pages/platform/platformShared'
import { formatNumber } from '@/lib/formatters'
import { formatDateParam } from '@/lib/date'
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

/** 聚焦态壳：诚实说明 + UserDetail（embedded，复用其全部贡献明细 + 周趋势）。 */
function FocusedContribution({ object, objectLabel }: { object: string; objectLabel: string }) {
  const { timeRange } = useViewState()
  return (
    <div className="flex flex-col gap-4">
      <p className="text-xs text-gray-400 dark:text-gray-500">
        贡献口径 = <b className="font-medium text-gray-600 dark:text-gray-300">看板派生</b>（合并需求 / 代码行 / 提交）。
        平台（chat-stats）的 tokens 为消耗量≠贡献，故本维度不接入平台。下方为 {objectLabel || object} 的个人贡献明细与周趋势。
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
      {/* 时间线：贡献维度无现成的按周时序（用户列表是区间聚合，无周桶）→ 诚实空态，不编造数据。 */}
      <DimensionTrend
        rows={[]}
        unavailable
        title="贡献趋势"
        subtitle="个人贡献口径（看板派生）"
        unavailableNote="贡献维度暂无按周的时间线（用户列表按 用户×区间 聚合，无周桶）。进入具体用户可见其周明细与周趋势。"
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
