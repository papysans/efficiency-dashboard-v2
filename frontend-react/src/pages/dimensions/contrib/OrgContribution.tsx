// 组织 · 贡献维度（看板派生）。贡献 = 部门交付物（合并需求 / 代码行 / 提交），平台源无部门/贡献字段，
// 故全程走看板派生口径（与 prd 矩阵「组织×贡献 = 板 部门贡献榜」一致，不发任何 chatStats 请求）。
//
// 数据源：/v2/dept-tree/ranking（useDeptRanking）—— 一次聚合返回 parent 各直接子部门整棵子树汇总
//   （DeptRankingItem.summary 复用 DeptMembersSummary 口径：merged_need_count / commit_diff_lines /
//   commit_count / member_count / kanban_member_count）。聚焦态用 /v2/dept-tree/members（DeptMembersPanel）
//   拿该部门直属成员贡献明细（仅直属，非递归）。
//
// 两态（由壳的 useEntityFocus().object 决定）：
//   聚合态(object 空)：贡献 KPI 卡（整体）+ 部门贡献 PK 榜（各直接子部门，点行下钻 → 写 ?object= 进聚焦）。
//   聚焦态(object=dept_id)：该部门成员贡献明细（DeptMembersPanel，含按成员的合并需求/代码行/提交）。
//
// 时间线：dept-sync 排行端点无「按周」时序（与 EfficiencyDimension 的 org-focus 一致）→ 走 DimensionTrend
//   的 unavailable 诚实空态，不编造贡献趋势。时间范围读全局 useViewState().timeRange。
//
// 本地降级：dept-tree/ranking 端点本地常 503（statDB 未连 / dept-sync 未配置）→ error 分支优雅占位
//   （参考 OrgTree 左树 error 分支），不白屏不崩。
//
// ⚠️ 口径：summary 提效比/AI占比为小数口径 → RatioPill（×100）；合并需求/代码行/提交为计数 → formatNumber。
//   贡献维度不用 PercentPill（无百分比直填字段）。
import { useCallback, useMemo } from 'react'
import { useSearchParams } from 'react-router'
import { useDeptRanking } from '@/api/queries'
import { useViewState } from '@/store/viewState'
import { useEntityFocus } from '@/components/layout/EntityDimensionLayout'
import { DimensionTrend } from '@/components/executive/DimensionTrend'
import { MetricCard } from '@/components/ui/MetricCard'
import { RatioPill } from '@/components/ui/RatioPill'
import { ChartCard, EmptyHint } from '@/pages/platform/platformShared'
import { DirectMembersNote } from '@/pages/dimensions/platformDimShared'
import { DeptMembersPanel } from '@/pages/orgs/DeptMembersPanel'
import { formatNumber } from '@/lib/formatters'
import { sortRows } from '@/lib/sort'
import type { DeptRankingItem } from '@/api/types'

const TH = 'px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_NUM = 'px-3 py-2 text-right font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_CENTER = 'px-3 py-2 text-center font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TD = 'px-3 py-2 align-middle text-gray-700 dark:text-gray-200 whitespace-nowrap'
const TD_NUM = 'px-3 py-2 align-middle text-right tabular-nums text-gray-700 dark:text-gray-200'

export default function OrgContribution() {
  const { object, objectLabel, entity } = useEntityFocus()
  const { timeRange } = useViewState()
  const focused = object !== ''

  return (
    <div className="flex flex-col gap-5">
      {/* 页首：贡献时间线 —— 排行端点无按周时序 → 诚实空态（不编造）。 */}
      <DimensionTrend
        rows={[]}
        unavailable
        title="贡献趋势"
        subtitle={focused ? `部门 · ${objectLabel || object}` : '全公司 · 按部门'}
        unavailableNote="按部门的贡献周趋势建设中（部门排行端点为区间聚合，无按周时序）。下方为看板派生的部门贡献排行 / 成员明细。"
      />

      {focused ? (
        // 聚焦态：复用 DeptMembersPanel 给该部门成员贡献明细（含合并需求/代码行/提交，仅直属成员，非递归）。
        // P1-3：聚焦态只统计直属成员，子部门未计入 → 醒目标注（与平台 org 维度一致复用 DirectMembersNote）。
        <>
          <DirectMembersNote />
          <DeptMembersPanel deptId={object} deptName={objectLabel || object} dateRange={timeRange} />
        </>
      ) : (
        <DeptContributionAggregate entity={entity} timeRange={timeRange} />
      )}
    </div>
  )
}

/** 聚合态：部门贡献 PK 榜（各直接子部门整棵子树汇总）+ 贡献 KPI 卡。点行下钻 → 写 ?object= 进聚焦。 */
function DeptContributionAggregate({ entity, timeRange }: { entity: string; timeRange: [string, string] }) {
  const [searchParams, setSearchParams] = useSearchParams()

  // 与 DeptMembersPanel 一致用 YYYYMMDD（排行端点 startDate/endDate 透传 aggregateUsersV2，口径同 members）。
  const dateParams = useMemo(
    () => ({ startDate: timeRange[0].replace(/-/g, ''), endDate: timeRange[1].replace(/-/g, '') }),
    [timeRange],
  )
  // parentDeptId 留空 → 后端取配置根，排「全公司一级部门」PK。
  const { data, isLoading, error } = useDeptRanking(dateParams)

  const items = useMemo<DeptRankingItem[]>(() => data?.items ?? [], [data])

  // 贡献维度按「合并需求」降序作 PK 主榜（null 沉底）。
  const ranked = useMemo(() => sortRows(items, (it) => it.summary.merged_need_count, true), [items])

  const kpi = useMemo(() => {
    const depts = items.length
    const mergedNeeds = items.reduce((s, it) => s + (it.summary.merged_need_count || 0), 0)
    const codeLines = items.reduce((s, it) => s + (it.summary.commit_diff_lines || 0), 0)
    const commits = items.reduce((s, it) => s + (it.summary.commit_count || 0), 0)
    return { depts, mergedNeeds, codeLines, commits }
  }, [items])

  // 选定部门 → 写 ?object=（与壳 onSelect / OrgTree select 同源），保留其它 query，清 ?sub。
  // ⚠️ Rules of Hooks：必须放在任何条件 return（如下方 error 分支）之前，保证所有渲染路径 Hook 数量/顺序一致。
  const goDept = useCallback(
    (deptId: string) => {
      if (!deptId) return
      const next = new URLSearchParams(searchParams)
      next.set('object', deptId)
      next.delete('dept_id')
      next.delete('sub')
      setSearchParams(next, { replace: false })
    },
    [searchParams, setSearchParams],
  )

  // 本地降级：ranking 端点 503（statDB 未连 / dept-sync 未配）→ 优雅占位（不白屏不崩，参考 OrgTree error 分支）。
  if (error) {
    return (
      <div className="flex flex-col gap-4">
        <DerivedCaliberNote />
        <div className="glass rounded-2xl p-8 text-center text-sm text-rose-600 dark:text-rose-400">
          {(error as Error).message || '获取部门贡献排行失败'}
          <p className="mt-2 text-xs text-gray-400 dark:text-gray-500">
            部门排行依赖 dept-sync 服务（本地环境常未配置 → 503）。请在已接入部门同步的环境查看。
          </p>
        </div>
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-5">
      <DerivedCaliberNote />

      {/* 贡献 KPI 卡（整体）—— 计数口径 → formatNumber，不用 RatioPill/PercentPill。 */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        <MetricCard label="参与部门" value={formatNumber(kpi.depts)} hint="一级子部门（整棵子树汇总）" />
        <MetricCard label="合并需求" value={formatNumber(kpi.mergedNeeds)} hint="各部门 merged_need 合计" />
        <MetricCard label="代码行" value={formatNumber(kpi.codeLines)} hint="各部门 commit 净代码行合计" />
        <MetricCard label="提交数" value={formatNumber(kpi.commits)} hint="各部门 commit 合计" />
      </div>

      <ChartCard title="部门贡献 PK 榜（看板派生）" sub="按合并需求倒序 · 各一级子部门整棵子树汇总">
        <div className="overflow-x-auto max-h-[560px] overflow-y-auto">
          <table className="w-full text-sm border-collapse">
            <thead className="sticky top-0 bg-white/70 dark:bg-gray-900/70 backdrop-blur">
              <tr className="border-b border-gray-200/50 dark:border-white/10">
                <th className={TH_NUM}>排名</th>
                <th className={TH}>部门</th>
                <th className={TH_NUM}>合并需求</th>
                <th className={TH_NUM}>代码行</th>
                <th className={TH_NUM}>提交数</th>
                <th className={TH_NUM}>活跃成员</th>
                <th className={TH_CENTER}>AI 代码占比</th>
              </tr>
            </thead>
            <tbody>
              {isLoading ? (
                <SkeletonRows cols={7} />
              ) : ranked.length === 0 ? (
                <tr>
                  <td colSpan={7}>
                    <EmptyHint compact />
                  </td>
                </tr>
              ) : (
                ranked.map((it, i) => (
                  <tr
                    key={it.dept_id}
                    onClick={() => goDept(it.dept_id)}
                    className="border-b border-gray-100/50 dark:border-white/5 cursor-pointer hover:bg-apple-blue/5 dark:hover:bg-white/5 transition-colors"
                  >
                    <td className={TD_NUM}>{i + 1}</td>
                    <td className={TD}>
                      <button
                        type="button"
                        className="max-w-[260px] truncate text-left font-medium text-apple-blue hover:text-apple-blue-hover bg-transparent border-none p-0 cursor-pointer focus:outline-none focus-visible:underline"
                        title={it.dept_name}
                        onClick={(e) => {
                          e.stopPropagation()
                          goDept(it.dept_id)
                        }}
                      >
                        {it.dept_name || '-'}
                      </button>
                    </td>
                    <td className={TD_NUM}>{formatNumber(it.summary.merged_need_count)}</td>
                    <td className={TD_NUM}>
                      {it.summary.commit_diff_lines > 0 ? `${formatNumber(it.summary.commit_diff_lines)} 行` : '-'}
                    </td>
                    <td className={TD_NUM}>{formatNumber(it.summary.commit_count)}</td>
                    <td className={TD_NUM}>
                      {formatNumber(it.summary.kanban_member_count)}
                      <span className="text-gray-400 dark:text-gray-500"> / {formatNumber(it.summary.member_count)}</span>
                    </td>
                    <td className="px-3 py-2 align-middle text-center">
                      <RatioPill value={it.summary.ai_code_ratio ?? null} />
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </ChartCard>

      <p className="text-xs text-gray-400 dark:text-gray-500">
        「活跃成员 / 总成员」= 在所选时间窗口内有看板数据的成员数 / dept-sync 花名册直属人数。点部门行下钻该部门成员贡献明细。
        {entity !== 'org' && '（当前主体非组织，仍按部门维度展示贡献排行）'}
      </p>
    </div>
  )
}

/** 看板派生口径说明（贡献全程看板派生，平台源无部门/贡献字段，避免误以为平台数据）。 */
function DerivedCaliberNote() {
  return (
    <p className="text-xs text-gray-400 dark:text-gray-500">
      贡献 = 部门交付物（合并需求 / 代码行 / 提交），为<b className="font-medium">看板派生口径</b>。
      平台（chat-stats）源无部门 / 贡献字段，故此维不接平台数据。
    </p>
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
