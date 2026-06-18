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
// 时间线：合并需求按 ISO 周（/v2/dept-tree/trend，EntityWeeklyTrend metric=needs=need_count）。
//   聚焦态=单部门整棵子树成员；聚合态=全公司（dept-trend dept_id 空→后端默认公司根）。时间范围读全局 useViewState().timeRange。
//
// 本地降级：dept-tree/ranking 端点本地常 503（statDB 未连 / dept-sync 未配置）→ error 分支优雅占位
//   （参考 OrgTree 左树 error 分支），不白屏不崩。
//
// ⚠️ 口径：合并需求/代码行/提交为计数 → formatNumber。AI占比属使用/渗透维（codex 实锤本表混入），
//   按「贡献=合并需求/代码行/提交/贡献者」口径已从本排行删除；提效比同属效率维亦不在此展示。
import { useCallback, useMemo } from 'react'
import { useSearchParams } from 'react-router'
import { useDeptRanking, useDeptTrend } from '@/api/queries'
import { useViewState } from '@/store/viewState'
import { formatDateParam } from '@/lib/date'
import { useEntityFocus } from '@/components/layout/EntityDimensionLayout'
import { EntityWeeklyTrend } from '@/components/executive/EntityWeeklyTrend'
import { MetricCard } from '@/components/ui/MetricCard'
import { ChartCard, EmptyHint } from '@/pages/platform/platformShared'
import { DirectMembersNote } from '@/pages/dimensions/platformDimShared'
import { DeptMembersPanel } from '@/pages/orgs/DeptMembersPanel'
import { formatNumber } from '@/lib/formatters'
import { sortRows } from '@/lib/sort'
import type { DeptRankingItem } from '@/api/types'

const TH = 'px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_NUM = 'px-3 py-2 text-right font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TD = 'px-3 py-2 align-middle text-gray-700 dark:text-gray-200 whitespace-nowrap'
const TD_NUM = 'px-3 py-2 align-middle text-right tabular-nums text-gray-700 dark:text-gray-200'

export default function OrgContribution() {
  const { object, objectLabel, entity } = useEntityFocus()
  const { timeRange } = useViewState()
  const focused = object !== ''

  return (
    <div className="flex flex-col gap-5">
      {/* 页首：贡献时间线（合并需求按 ISO 周）。聚焦态=单部门整棵子树；聚合态=全公司（dept-trend 默认公司根）。
          均走 /v2/dept-tree/trend（user_productivity_v2 周表守恒聚合）。 */}
      <DeptContribTrend object={object} objectLabel={objectLabel} timeRange={timeRange} />

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

/** 聚焦态贡献时间线：该部门整棵子树成员周表按 ISO 周的「合并需求」时序（/v2/dept-tree/trend，need_count）。
 *  贡献维度主指标=合并需求（与下方排行/KPI 同口径）；loading/error/空态由 EntityWeeklyTrend 内部统一处理。
 *  日期用 YYYYMMDD（与 dept-tree/members、ranking 端点一致）。 */
function DeptContribTrend({
  object,
  objectLabel,
  timeRange,
}: {
  object: string
  objectLabel: string
  timeRange: [string, string]
}) {
  const dateParams = useMemo(
    () => ({ startDate: formatDateParam(timeRange[0]), endDate: formatDateParam(timeRange[1]) }),
    [timeRange],
  )
  // object 空 = 聚合态(全公司)：deptId 传空，后端默认公司根 → 全公司整树周趋势。
  const q = useDeptTrend({ deptId: object, ...dateParams })
  return (
    <EntityWeeklyTrend
      points={q.data?.data}
      loading={q.isLoading}
      error={q.error ? (q.error as Error).message : null}
      title="贡献趋势"
      subtitle={
        object
          ? `部门 · ${objectLabel || object} · 按 ISO 周（子树成员合并需求）`
          : '全公司 · 按 ISO 周（各部门子树成员合并需求）'
      }
      metric="needs"
    />
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
