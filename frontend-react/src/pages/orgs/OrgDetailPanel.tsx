// 组织详情主体（可复用面板）：按 orgPath + dateRange 调 /v2/orgs/detail，渲染
// summary 6 卡 + 用户列表 + Commits/Tasks 时序表 + 5 张图表。
// 由 OrgDetail.tsx（整页，带级联）和 OrgTree.tsx（右栏，配左树）共用，避免复制。
// ⚠️ 百分比口径 → PercentPill（不 ×100）；提效比趋势图 yAxis {value}% 直接画原值。
import { useMemo } from 'react'
import { useNavigate } from 'react-router'
import type { EChartsOption } from 'echarts'
import { useOrgDetail } from '@/api/queries'
import { useTheme } from '@/hooks/useTheme'
import { useUserNameMap } from '@/hooks/useUserNameMap'
import type { CommitTimeSeriesItem, OrgMember, TaskTimeSeriesItem } from '@/api/types'
import { formatDuration, formatNumber } from '@/lib/formatters'
import { getPalette } from '@/components/charts/chartTheme'
import { EChart } from '@/components/charts/EChart'
import { MetricCard } from '@/components/ui/MetricCard'
import { PercentPill } from '@/components/ui/PercentPill'
import { Panel, TH, TH_NUM, TH_CENTER, TD, TD_NUM, fmtCostVal, fmtTokens } from './orgDetailShared'

// 时序系列固定色（对齐 Vue）
const C_BLUE = '#409EFF'
const C_GREEN = '#67C23A'
const C_BLUE_LT = '#a0cfff'
const C_GREEN_LT = '#b3e19d'
const C_ORANGE = '#E6A23C'

interface OrgDetailPanelProps {
  /** "/" 分隔的组织路径，如 "深信服科技股份有限公司/研发体系/Costrict研发部/开发组"。空 → 占位提示。 */
  orgPath: string
  /** [startDate, endDate]，YYYY-MM-DD。 */
  dateRange: [string, string]
  /** 时间粒度（day/week/month/year）。 */
  granularity: string
}

/** 组织详情可复用面板。 */
export function OrgDetailPanel({ orgPath, dateRange, granularity }: OrgDetailPanelProps) {
  const navigate = useNavigate()
  const { theme } = useTheme()
  // members 的 user_name 多为 UUID，用 commits 的 git_user_name 解析真实名。
  const { resolveName } = useUserNameMap()

  const dateParams = useMemo(
    () => ({ startDate: dateRange[0].replace(/-/g, ''), endDate: dateRange[1].replace(/-/g, '') }),
    [dateRange],
  )

  const { data, isLoading, error } = useOrgDetail({ orgPath, ...dateParams, granularity })

  const summary = data?.summary
  const members: OrgMember[] = useMemo(() => data?.members || [], [data?.members])
  const commits: CommitTimeSeriesItem[] = useMemo(() => data?.commits || [], [data?.commits])
  const tasks: TaskTimeSeriesItem[] = useMemo(() => data?.tasks || [], [data?.tasks])

  function goUser(userId: string) {
    if (!userId) return
    navigate({
      pathname: `/user/${encodeURIComponent(userId)}`,
      search: `?startDate=${dateParams.startDate}&endDate=${dateParams.endDate}`,
    })
  }

  // ---- 图表 option（5 张，仅 commits/tasks 非空显示）----
  const p = useMemo(() => getPalette(theme), [theme])
  const labels = useMemo(
    () => (commits.length ? commits : tasks).map((d) => d.period_label),
    [commits, tasks],
  )

  const baseAxis = useMemo(
    () => ({
      xAxis: {
        type: 'category' as const,
        data: labels,
        axisLine: { lineStyle: { color: p.axisColor } },
        axisLabel: { rotate: 45, fontSize: 11, color: p.textColor },
      },
      yAxis: {
        type: 'value' as const,
        axisLabel: { color: p.textColor },
        splitLine: { lineStyle: { color: p.splitLineColor } },
      },
      grid: { left: '5%', right: '5%', top: '24%', bottom: '12%', containLabel: true },
      tooltip: {
        trigger: 'axis' as const,
        backgroundColor: p.tooltipBg,
        borderColor: p.tooltipBorder,
        borderWidth: 1,
        textStyle: { color: p.tooltipText },
      },
    }),
    [labels, p],
  )

  const titleStyle = useMemo(
    () => ({ top: 8, left: 'center' as const, textStyle: { fontSize: 14, fontWeight: 'bold' as const, color: p.textColor } }),
    [p],
  )
  const legendStyle = useMemo(() => ({ top: 36, textStyle: { color: p.textColor } }), [p])

  const chart1: EChartsOption = useMemo(
    () => ({
      ...baseAxis,
      title: { ...titleStyle, text: 'Task数 & Commit数' },
      legend: { ...legendStyle, data: ['Task数', 'Commit数'] },
      series: [
        { name: 'Task数', type: 'bar', data: tasks.map((d) => d.task_count || 0), itemStyle: { color: C_BLUE } },
        { name: 'Commit数', type: 'bar', data: commits.map((d) => d.commit_count || 0), itemStyle: { color: C_GREEN } },
      ],
    }),
    [baseAxis, titleStyle, legendStyle, tasks, commits],
  )

  const chart2: EChartsOption = useMemo(
    () => ({
      ...baseAxis,
      title: { ...titleStyle, text: '代码行数' },
      legend: { ...legendStyle, data: ['Task代码行数', 'Commit代码行数'] },
      series: [
        { name: 'Task代码行数', type: 'bar', data: tasks.map((d) => d.task_diff_lines || 0), itemStyle: { color: C_BLUE } },
        { name: 'Commit代码行数', type: 'bar', data: commits.map((d) => d.commit_diff_lines || 0), itemStyle: { color: C_GREEN } },
      ],
    }),
    [baseAxis, titleStyle, legendStyle, tasks, commits],
  )

  const chart3: EChartsOption = useMemo(
    () => ({
      ...baseAxis,
      title: { ...titleStyle, text: '耗时对比' },
      legend: { ...legendStyle, data: ['Task传统耗时', 'Commit传统耗时', 'Task实际耗时', 'Commit实际耗时'] },
      tooltip: {
        ...baseAxis.tooltip,
        formatter: (items: unknown) => {
          const rows = (Array.isArray(items) ? items : [items]) as Array<{ axisValue: string; marker: string; seriesName: string; value: number }>
          return rows.reduce(
            (txt, item, i) => `${txt}${i === 0 ? `${item.axisValue}<br/>` : ''}${item.marker}${item.seriesName}: ${formatDuration(Number(item.value || 0))}<br/>`,
            '',
          )
        },
      },
      series: [
        { name: 'Task传统耗时', type: 'bar', stack: 'ancient', data: tasks.map((d) => d.task_ancient_minutes || 0), itemStyle: { color: C_BLUE } },
        { name: 'Commit传统耗时', type: 'bar', stack: 'ancient', data: commits.map((d) => d.commit_ancient_minutes || 0), itemStyle: { color: C_GREEN } },
        { name: 'Task实际耗时', type: 'bar', stack: 'real', data: tasks.map((d) => d.task_real_minutes || 0), itemStyle: { color: C_BLUE_LT } },
        { name: 'Commit实际耗时', type: 'bar', stack: 'real', data: commits.map((d) => d.commit_real_minutes || 0), itemStyle: { color: C_GREEN_LT } },
      ],
    }),
    [baseAxis, titleStyle, legendStyle, tasks, commits],
  )

  const chart4: EChartsOption = useMemo(
    () => ({
      ...baseAxis,
      title: { ...titleStyle, text: '费用' },
      legend: { ...legendStyle, data: ['费用'] },
      series: [{ name: '费用', type: 'bar', data: commits.map((d) => d.cost || 0), itemStyle: { color: C_ORANGE } }],
    }),
    [baseAxis, titleStyle, legendStyle, commits],
  )

  // 提效比趋势：百分比口径，直接画原值，yAxis {value}%
  const chart5: EChartsOption = useMemo(
    () => ({
      ...baseAxis,
      title: { ...titleStyle, text: '提效比趋势' },
      legend: { ...legendStyle, data: ['Task提效比', 'Commit提效比'] },
      yAxis: { ...baseAxis.yAxis, axisLabel: { formatter: '{value}%', color: p.textColor } },
      series: [
        { name: 'Task提效比', type: 'line', smooth: true, data: tasks.map((d) => d.task_efficiency_ratio || 0), itemStyle: { color: C_BLUE } },
        { name: 'Commit提效比', type: 'line', smooth: true, data: commits.map((d) => d.commit_efficiency_ratio || 0), itemStyle: { color: C_GREEN } },
      ],
    }),
    [baseAxis, titleStyle, legendStyle, tasks, commits, p],
  )

  const hasChartData = commits.length > 0 || tasks.length > 0

  if (!orgPath) {
    return (
      <div className="glass rounded-2xl p-8 text-center text-sm text-gray-400 dark:text-gray-500">请选择组织以查看详情</div>
    )
  }

  if (error) {
    return (
      <div className="glass rounded-2xl p-8 text-center text-sm text-rose-600 dark:text-rose-400">
        {(error as Error).message || '获取组织详情失败'}
      </div>
    )
  }

  return (
    <div className={`space-y-5 ${isLoading ? 'opacity-60 pointer-events-none' : ''}`}>
      {/* 6 张汇总卡 */}
      <section className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-3">
        <MetricCard label="成员数" value={formatNumber(summary?.user_count ?? 0)} />
        <MetricCard label="Task代码量" value={formatNumber(summary?.task_diff_lines ?? 0)} />
        <MetricCard label="Commit代码量" value={formatNumber(summary?.commit_diff_lines ?? 0)} />
        <MetricCard label="Task提效比" value={<PercentPill value={summary?.task_efficiency_ratio} />} />
        <MetricCard label="Commit提效比" value={<PercentPill value={summary?.commit_efficiency_ratio} />} />
        <MetricCard label="总费用" value={fmtCostVal(summary?.cost)} />
      </section>

      {/* 用户列表 */}
      <Panel title="用户列表" hint={`${members.length} 人`}>
        <table className="w-full text-sm border-collapse">
          <thead>
            <tr className="border-b border-gray-200/50 dark:border-white/10">
              <th className={TH}>用户名</th>
              <th className={TH_NUM}>Commit代码量</th>
              <th className={TH_NUM}>Commit实际耗时</th>
              <th className={TH_CENTER}>Commit提效比</th>
              <th className={TH_NUM}>Task代码量</th>
              <th className={TH_NUM}>Task实际耗时</th>
              <th className={TH_CENTER}>Task提效比</th>
              <th className={TH_NUM}>Tokens消耗</th>
              <th className={TH_NUM}>费用</th>
            </tr>
          </thead>
          <tbody>
            {!members.length ? (
              <tr>
                <td colSpan={9}>
                  <div className="py-8 text-center text-sm text-gray-400 dark:text-gray-500">暂无成员</div>
                </td>
              </tr>
            ) : (
              members.map((m) => (
                <tr
                  key={m.user_id}
                  onClick={() => goUser(m.user_id)}
                  className="border-b border-gray-100/50 dark:border-white/5 cursor-pointer hover:bg-apple-blue/5 dark:hover:bg-white/5 transition-colors"
                >
                  <td className={TD}>
                    <button
                      type="button"
                      className="text-apple-blue hover:text-apple-blue-hover cursor-pointer bg-transparent border-none p-0 focus:outline-none focus-visible:underline"
                      title={resolveName(m.user_id)}
                      onClick={(e) => {
                        e.stopPropagation()
                        goUser(m.user_id)
                      }}
                    >
                      {resolveName(m.user_id)}
                    </button>
                  </td>
                  <td className={TD_NUM}>{formatNumber(m.commit_diff_lines, 0)}</td>
                  <td className={TD_NUM}>{formatDuration(m.commit_real_minutes)}</td>
                  <td className="px-3 py-2 align-middle text-center"><PercentPill value={m.commit_efficiency_ratio} /></td>
                  <td className={TD_NUM}>{formatNumber(m.task_diff_lines, 0)}</td>
                  <td className={TD_NUM}>{formatDuration(m.task_real_minutes)}</td>
                  <td className="px-3 py-2 align-middle text-center"><PercentPill value={m.task_efficiency_ratio} /></td>
                  <td className={TD_NUM}>{fmtTokens(m.upstream_tokens, m.downstream_tokens)}</td>
                  <td className={TD_NUM}>{fmtCostVal(m.cost)}</td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </Panel>

      {/* Commits 时序表 */}
      <Panel title="Commits 列表" hint={`${commits.length} 期`}>
        <table className="w-full text-sm border-collapse">
          <thead>
            <tr className="border-b border-gray-200/50 dark:border-white/10">
              <th className={TH}>时间</th>
              <th className={TH_NUM}>Commit数</th>
              <th className={TH_NUM}>代码量</th>
              <th className={TH_NUM}>实际耗时</th>
              <th className={TH_NUM}>传统开发时长预估</th>
              <th className={TH_CENTER}>提效比</th>
              <th className={TH_NUM}>Tokens消耗</th>
              <th className={TH_NUM}>费用</th>
            </tr>
          </thead>
          <tbody>
            {!commits.length ? (
              <tr>
                <td colSpan={8}>
                  <div className="py-8 text-center text-sm text-gray-400 dark:text-gray-500">暂无 Commit 数据</div>
                </td>
              </tr>
            ) : (
              commits.map((c) => (
                <tr key={c.period_key} className="border-b border-gray-100/50 dark:border-white/5">
                  <td className={TD}>{c.period_label}</td>
                  <td className={TD_NUM}>{c.commit_count}</td>
                  <td className={TD_NUM}>{formatNumber(c.commit_diff_lines, 0)}</td>
                  <td className={TD_NUM}>{formatDuration(c.commit_real_minutes)}</td>
                  <td className={TD_NUM}>{formatDuration(c.commit_ancient_minutes)}</td>
                  <td className="px-3 py-2 align-middle text-center"><PercentPill value={c.commit_efficiency_ratio} /></td>
                  <td className={TD_NUM}>{fmtTokens(c.upstream_tokens, c.downstream_tokens)}</td>
                  <td className={TD_NUM}>{fmtCostVal(c.cost)}</td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </Panel>

      {/* Tasks 时序表 */}
      <Panel title="Tasks 列表" hint={`${tasks.length} 期`}>
        <table className="w-full text-sm border-collapse">
          <thead>
            <tr className="border-b border-gray-200/50 dark:border-white/10">
              <th className={TH}>时间</th>
              <th className={TH_NUM}>Task数</th>
              <th className={TH_NUM}>代码量</th>
              <th className={TH_NUM}>实际耗时</th>
              <th className={TH_NUM}>传统开发时长预估</th>
              <th className={TH_CENTER}>提效比</th>
              <th className={TH_NUM}>Tokens消耗</th>
              <th className={TH_NUM}>费用</th>
            </tr>
          </thead>
          <tbody>
            {!tasks.length ? (
              <tr>
                <td colSpan={8}>
                  <div className="py-8 text-center text-sm text-gray-400 dark:text-gray-500">暂无 Task 数据</div>
                </td>
              </tr>
            ) : (
              tasks.map((t) => (
                <tr key={t.period_key} className="border-b border-gray-100/50 dark:border-white/5">
                  <td className={TD}>{t.period_label}</td>
                  <td className={TD_NUM}>{t.task_count}</td>
                  <td className={TD_NUM}>{formatNumber(t.task_diff_lines, 0)}</td>
                  <td className={TD_NUM}>{formatDuration(t.task_real_minutes)}</td>
                  <td className={TD_NUM}>{formatDuration(t.task_ancient_minutes)}</td>
                  <td className="px-3 py-2 align-middle text-center"><PercentPill value={t.task_efficiency_ratio} /></td>
                  <td className={TD_NUM}>{fmtTokens(t.upstream_tokens, t.downstream_tokens)}</td>
                  <td className={TD_NUM}>{fmtCostVal(t.cost)}</td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </Panel>

      {/* 图表区（5 张，仅 commits/tasks 非空显示） */}
      {hasChartData && (
        <>
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
            <div className="glass rounded-2xl p-4"><EChart option={chart1} height={280} className="w-full" /></div>
            <div className="glass rounded-2xl p-4"><EChart option={chart2} height={280} className="w-full" /></div>
            <div className="glass rounded-2xl p-4"><EChart option={chart3} height={280} className="w-full" /></div>
            <div className="glass rounded-2xl p-4"><EChart option={chart4} height={280} className="w-full" /></div>
          </div>
          <div className="glass rounded-2xl p-4"><EChart option={chart5} height={280} className="w-full" /></div>
        </>
      )}
    </div>
  )
}
