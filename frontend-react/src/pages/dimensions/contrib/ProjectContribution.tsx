// 「项目 · 贡献」维度（看板派生）—— 平台(chat-stats)源无项目维度，贡献用看板派生：
//   贡献 = 提交产出的代码量 + 完成的需求 + 参与的贡献者人数（tokens 是消耗量非贡献，不进）。
// 两态（与 EfficiencyDimension / kanbanDerived 一致）：
//   聚合态(object 空)：项目贡献排行表（Need 数 / 贡献者 / 生成代码）+ KPI 卡。
//   聚焦态(选定 projectId)：复用 ProjectDetail（embedded，壳保留面包屑），其「贡献者」「组成·Needs」
//     两块即贡献明细，无需重写守恒派生逻辑。
// 时间线：项目无按周的时间序列数据（周表按 用户×周 聚合，无项目维度）→ DimensionTrend 走诚实空态，不编造。
// 口径：need_* 为小数口径 → RatioPill（AI 占比同样小数口径，绝不用 PercentPill 百分比口径）。
import { useMemo } from 'react'
import { useNavigate } from 'react-router'
import { useProjectList } from '@/api/queries'
import { useViewState } from '@/store/viewState'
import { useEntityFocus } from '@/components/layout/EntityDimensionLayout'
import { DimensionTrend } from '@/components/executive/DimensionTrend'
import { MetricCard } from '@/components/ui/MetricCard'
import { RatioPill } from '@/components/ui/RatioPill'
import { ChartCard, EmptyHint } from '@/pages/platform/platformShared'
import { formatNumber } from '@/lib/formatters'
import { sortRows } from '@/lib/sort'
import ProjectDetail from '@/pages/projects/ProjectDetail'
import type { ProjectListItem } from '@/api/types'

const TH = 'px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_NUM = 'px-3 py-2 text-right font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_CENTER = 'px-3 py-2 text-center font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TD = 'px-3 py-2 align-middle text-gray-700 dark:text-gray-200'
const TD_NUM = 'px-3 py-2 align-middle text-right tabular-nums text-gray-700 dark:text-gray-200'

/** 平台口径缺位提示：贡献用看板派生（平台无项目维度，tokens=消耗量非贡献故不进）。 */
function ContribCaliberNote() {
  return (
    <p className="text-xs text-gray-400 dark:text-gray-500">
      贡献为<b className="font-medium">看板派生口径</b>（完成的需求 / 生成代码 / 贡献者）。平台（chat-stats）源无项目维度，
      且 tokens 是消耗量非贡献，故贡献维度不接入平台。
    </p>
  )
}

export default function ProjectContribution() {
  const { object } = useEntityFocus()
  const { timeRange } = useViewState()
  const focused = object !== ''

  return (
    <div className="flex flex-col gap-5">
      {/* 页首：贡献无按周时间序列 → 诚实空态（不编造趋势）。 */}
      <DimensionTrend
        rows={[]}
        unavailable
        title="贡献趋势"
        subtitle="项目口径"
        unavailableNote="项目暂无按周的贡献时间线（周表按 用户×周 聚合，无项目维度）。下方为看板派生的贡献排行与明细。"
      />

      {focused ? (
        <div className="flex flex-col gap-4">
          <ContribCaliberNote />
          {/* 聚焦态：复用 ProjectDetail（含「贡献者」「组成·Needs」两块=贡献明细），壳保留面包屑。 */}
          <ProjectDetail projectIdProp={object} embedded />
        </div>
      ) : (
        <ProjectContribAggregate />
      )}
      {/* timeRange 由全局 store 提供（与其它维度一致）；项目贡献当前无时间维聚合，预留对齐签名。 */}
      <span className="sr-only">{timeRange[0]}~{timeRange[1]}</span>
    </div>
  )
}

/** 聚合态：项目贡献排行（Need 数 / 贡献者 / 生成代码）+ KPI 卡。点行下钻 → /project/:projectId 独立详情。 */
function ProjectContribAggregate() {
  const navigate = useNavigate()
  const { data, isLoading, error } = useProjectList()
  const rows = useMemo<ProjectListItem[]>(() => data?.data ?? [], [data])

  function goToProject(row: ProjectListItem) {
    if (!row?.project_id) return
    navigate(`/project/${encodeURIComponent(row.project_id)}`)
  }

  // 按生成代码量降序（null 沉底）——贡献维度看产出代码量。
  const ranked = useMemo(() => sortRows(rows, (r) => r.need_total_loc_net, true), [rows])

  const kpi = useMemo(() => {
    const projects = rows.length
    const needs = rows.reduce((s, r) => s + (r.need_total_count || 0), 0)
    const eligible = rows.reduce((s, r) => s + (r.need_eligible_count || 0), 0)
    const contributors = rows.reduce((s, r) => s + (r.user_count || 0), 0)
    const loc = rows.reduce((s, r) => s + (r.need_total_loc_net || 0), 0)
    return { projects, needs, eligible, contributors, loc }
  }, [rows])

  if (error) {
    return (
      <div className="glass rounded-2xl p-8 text-center text-sm text-rose-600 dark:text-rose-400">
        {(error as Error).message || '获取项目贡献数据失败'}
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-5">
      <ContribCaliberNote />
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        <MetricCard label="项目数" value={formatNumber(kpi.projects)} />
        <MetricCard
          label="完成需求"
          value={formatNumber(kpi.eligible)}
          hint={`候选 ${formatNumber(kpi.needs)}`}
          tip="合格(干净)需求之和；候选=看板口径全量需求。"
        />
        <MetricCard
          label="贡献者(累计)"
          value={formatNumber(kpi.contributors)}
          hint="各项目人数合计(可重复)"
          tip="同一人可能多账号(工号去重待后续)，跨项目累计可重复计数。"
        />
        <MetricCard
          label="生成代码(合计)"
          value={kpi.loc > 0 ? `${formatNumber(kpi.loc)} 行` : '-'}
          tip="各项目干净需求净 LOC 之和。"
        />
      </div>
      <ChartCard title="项目贡献排行（看板派生）" sub="按生成代码量倒序 · 点行下钻">
        <div className="overflow-x-auto">
          <table className="w-full text-sm border-collapse">
            <thead>
              <tr className="border-b border-gray-200/50 dark:border-white/10">
                <th className={TH_NUM}>排名</th>
                <th className={`${TH} min-w-[200px]`}>项目</th>
                <th className={TH_NUM}>生成代码</th>
                <th className={TH_NUM}>完成 / 候选需求</th>
                <th className={TH_NUM} title="同一人可能多账号(工号去重待后续)">贡献者</th>
                <th className={TH_CENTER}>AI 占比</th>
              </tr>
            </thead>
            <tbody>
              {isLoading ? (
                Array.from({ length: 6 }).map((_, i) => (
                  <tr key={i} className="border-b border-gray-100/50 dark:border-white/5">
                    <td colSpan={6} className="px-3 py-2">
                      <div className="skeleton h-6 rounded" />
                    </td>
                  </tr>
                ))
              ) : ranked.length === 0 ? (
                <tr>
                  <td colSpan={6}>
                    <EmptyHint compact />
                  </td>
                </tr>
              ) : (
                ranked.map((r, i) => (
                  <tr
                    key={r.project_id}
                    onClick={() => goToProject(r)}
                    className="border-b border-gray-100/50 dark:border-white/5 cursor-pointer hover:bg-apple-blue/5 dark:hover:bg-white/5 transition-colors"
                  >
                    <td className={TD_NUM}>{i + 1}</td>
                    <td className={TD}>
                      <button
                        type="button"
                        className="max-w-[240px] truncate text-left font-medium text-apple-blue hover:text-apple-blue-hover bg-transparent border-none p-0 cursor-pointer focus:outline-none focus-visible:underline"
                        title={r.name}
                        onClick={(e) => {
                          e.stopPropagation()
                          goToProject(r)
                        }}
                      >
                        {r.name || '-'}
                      </button>
                    </td>
                    <td className={TD_NUM}>
                      {r.need_total_loc_net && r.need_total_loc_net > 0 ? `${formatNumber(r.need_total_loc_net)} 行` : '-'}
                    </td>
                    <td className={TD_NUM} title="完成(合格) / 候选需求">
                      {r.need_eligible_count ?? 0}{' '}
                      <span className="text-gray-400 dark:text-gray-500">/ {r.need_total_count ?? 0}</span>
                    </td>
                    <td className={TD_NUM}>{formatNumber(r.user_count)}</td>
                    <td className="px-3 py-2 align-middle text-center">
                      <RatioPill value={r.need_ai_code_ratio ?? null} />
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
