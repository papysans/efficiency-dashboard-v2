// 成本·子部门对比（PK）视角：一次取 /cost/sub-departments 列出当前部门全部子部门（团队），
// 横向对比各团队费用/占比/人均/输入输出/活跃用户/Token；并配「各团队每日费用」折线 + 团队费用构成饼图。
// include_children 控制统计是否含各子部门更深层（孙部门）。
// 点行 → onSelectDept(childId) 切到该子部门的聚合视图。
import { useMemo, useState } from 'react'
import type { EChartsOption } from 'echarts'
import { useTheme } from '@/hooks/useTheme'
import { getPalette } from '@/components/charts/chartTheme'
import { ChartCard, EmptyHint, PIE_COLORS, baseTooltip, shortToken } from '@/pages/platform/platformShared'
import { useGranularity, GranularityToggle } from '../granularity'
import { buildBuckets, GRANULARITY_CN } from '@/lib/timeBucket'
import { SortableTh } from '@/components/ui/SortableTh'
import { EChart } from '@/components/charts/EChart'
import { fmtCost, formatNumber } from '@/lib/formatters'
import { useCostSubDepts, useCostTeamTrend, useCostTeamComposition } from './costData'
import type { CostSubDeptItem, CostTeamTrendSeries } from './costTypes'

const PCT = (v: number | null | undefined) => (v == null || !Number.isFinite(v) ? '-' : `${v.toFixed(1)}%`)
const TH = 'px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_NUM = 'px-3 py-2 text-right font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TD = 'px-3 py-2 align-middle text-gray-700 dark:text-gray-200 whitespace-nowrap'
const TD_NUM = 'px-3 py-2 text-right align-middle tabular-nums text-gray-700 dark:text-gray-200 whitespace-nowrap'

type SortKey = 'total_cost' | 'cost_pct' | 'active_users' | 'input_cost' | 'output_cost'

/** 团队人均费用：active_users>0 才算，否则 null。 */
function perUserCost(it: CostSubDeptItem): number | null {
  return it.active_users > 0 ? it.total_cost / it.active_users : null
}

export function CostCompareView({
  deptId,
  start,
  end,
  includeChildren,
  onSelectDept,
}: {
  deptId: string
  start: string
  end: string
  includeChildren: boolean
  onSelectDept: (deptId: string) => void
}) {
  const { theme } = useTheme()
  const p = useMemo(() => getPalette(theme), [theme])

  const q = { deptId, start, end, includeChildren }
  const subDepts = useCostSubDepts(q)
  const teamTrend = useCostTeamTrend(q)
  const teamComposition = useCostTeamComposition(q)

  // 趋势粒度（随区间重置默认）。
  const { gran, setGran, options: granOptions } = useGranularity(start, end)

  const [sortBy, setSortBy] = useState<SortKey>('total_cost')
  const [sortOrder, setSortOrder] = useState<'asc' | 'desc'>('desc')

  // items 前端排序即可
  const items = useMemo(() => {
    const arr = (subDepts.data?.items ?? []).slice()
    arr.sort((a, b) => {
      const diff = Number(a[sortBy] ?? 0) - Number(b[sortBy] ?? 0)
      return sortOrder === 'desc' ? -diff : diff
    })
    return arr
  }, [subDepts.data, sortBy, sortOrder])

  const totals = useMemo(() => {
    const sum = (k: keyof CostSubDeptItem) => items.reduce((s, it) => s + Number(it[k] ?? 0), 0)
    const activeUsers = sum('active_users')
    return {
      total_cost: sum('total_cost'),
      input_cost: sum('input_cost'),
      output_cost: sum('output_cost'),
      active_users: activeUsers,
      total_tokens: sum('total_tokens'),
      per_user: activeUsers > 0 ? sum('total_cost') / activeUsers : null,
    }
  }, [items])

  const handleSort = (field: string) => {
    const f = field as SortKey
    if (sortBy === f) setSortOrder((o) => (o === 'desc' ? 'asc' : 'desc'))
    else {
      setSortBy(f)
      setSortOrder('desc')
    }
  }

  // ---- 各团队每日费用折线：多 series 按 date 并集分桶，桶内费用求和（可加） ----
  const trendSeries = (teamTrend.data?.series ?? []) as CostTeamTrendSeries[]
  const trendOpt = useMemo<EChartsOption | null>(() => {
    if (!trendSeries.length) return null
    const dateSet = new Set(trendSeries.flatMap((s) => s.data.map((d) => d.date)))
    const buckets = buildBuckets(Array.from(dateSet), gran, { start, end })
    if (!buckets.length) return null
    const labels = buckets.map((b) => b.label)
    const headers = buckets.map((b) => b.rangeText)
    const seriesData = (s: CostTeamTrendSeries) => {
      const m = new Map(s.data.map((d) => [d.date, Number(d.total_cost) || 0]))
      return buckets.map((b) => b.dates.reduce((acc, d) => acc + (m.get(d) ?? 0), 0))
    }
    return {
      animation: true,
      grid: { left: 8, right: 16, top: 36, bottom: 8, containLabel: true },
      tooltip: {
        trigger: 'axis',
        ...baseTooltip(p),
        formatter: (params: unknown) => {
          const arr = params as { dataIndex: number; seriesName: string; value: number; marker: string; axisValue: string }[]
          const head = headers[arr[0]?.dataIndex] ?? arr[0]?.axisValue ?? ''
          return `${head}<br/>${arr.map((it) => `${it.marker}${it.seriesName}: ¥${fmtCost(it.value)}`).join('<br/>')}`
        },
      },
      legend: { top: 0, left: 'center', textStyle: { color: p.textColor }, itemWidth: 14, itemHeight: 8 },
      xAxis: {
        type: 'category',
        data: labels,
        boundaryGap: false,
        axisLine: { lineStyle: { color: p.axisColor } },
        axisLabel: { color: p.textColor, hideOverlap: true },
        axisTick: { show: false },
      },
      yAxis: {
        type: 'value',
        axisLabel: { color: p.textColor, formatter: (v: number) => `¥${shortToken(v)}` },
        splitLine: { lineStyle: { color: p.splitLineColor } },
      },
      series: trendSeries.map((s) => ({
        name: s.dept_name,
        type: 'line',
        smooth: true,
        symbol: 'none',
        data: seriesData(s),
        lineStyle: { width: 2 },
      })),
    }
  }, [trendSeries, p, gran, start, end])

  // ---- 团队费用构成饼图 ----
  const compItems = teamComposition.data?.items ?? []
  const pieOpt = useMemo<EChartsOption | null>(() => {
    if (!compItems.length) return null
    return {
      tooltip: { trigger: 'item', ...baseTooltip(p), formatter: '{b}: ¥{c} ({d}%)' },
      legend: { type: 'scroll', bottom: 0, textStyle: { color: p.textColor } },
      series: [
        {
          type: 'pie',
          radius: ['38%', '68%'],
          center: ['50%', '46%'],
          itemStyle: { borderColor: p.tooltipBg, borderWidth: 2 },
          label: { color: p.textColor },
          data: compItems.map((it, i) => ({
            name: it.dept_name,
            value: it.total_cost,
            itemStyle: { color: PIE_COLORS[i % PIE_COLORS.length] },
          })),
        },
      ],
    }
  }, [compItems, p])

  // ---- 渲染 ----
  const subLabel = `${items.length} 个子部门 · ${includeChildren ? '含各子部门下级' : '仅各子部门直属'} · 点行下钻`

  return (
    <div className="space-y-5">
      {/* 子部门成本对比表 */}
      <ChartCard title="子部门成本对比" sub={subLabel}>
        <div className="overflow-x-auto">
          <table className="w-full text-sm border-collapse">
            <thead>
              <tr className="border-b border-gray-200/50 dark:border-white/10">
                <th className={TH_NUM}>#</th>
                <th className={TH}>子部门</th>
                <th className={TH_NUM}>
                  <SortableTh field="total_cost" label="费用" numeric active={sortBy === 'total_cost'} desc={sortOrder === 'desc'} onSort={handleSort} />
                </th>
                <th className={TH_NUM}>
                  <SortableTh field="cost_pct" label="费用占比" numeric active={sortBy === 'cost_pct'} desc={sortOrder === 'desc'} onSort={handleSort} />
                </th>
                <th className={TH_NUM}>团队人均</th>
                <th className={TH_NUM}>
                  <SortableTh field="input_cost" label="输入费用" numeric active={sortBy === 'input_cost'} desc={sortOrder === 'desc'} onSort={handleSort} />
                </th>
                <th className={TH_NUM}>
                  <SortableTh field="output_cost" label="输出费用" numeric active={sortBy === 'output_cost'} desc={sortOrder === 'desc'} onSort={handleSort} />
                </th>
                <th className={TH_NUM}>
                  <SortableTh field="active_users" label="活跃用户" numeric active={sortBy === 'active_users'} desc={sortOrder === 'desc'} onSort={handleSort} />
                </th>
                <th className={TH_NUM}>总 Token</th>
              </tr>
            </thead>
            <tbody>
              {subDepts.isLoading && !items.length ? (
                <tr>
                  <td colSpan={9} className="py-10 text-center text-sm text-gray-400 dark:text-gray-500">加载中…</td>
                </tr>
              ) : !items.length ? (
                <tr>
                  <td colSpan={9} className="py-10 text-center text-sm text-gray-400 dark:text-gray-500">该部门下无子部门</td>
                </tr>
              ) : (
                items.map((it, i) => {
                  const pu = perUserCost(it)
                  return (
                    <tr
                      key={it.dept_id}
                      onClick={() => onSelectDept(it.dept_id)}
                      className="border-b border-gray-100/50 dark:border-white/5 cursor-pointer hover:bg-apple-blue/5 dark:hover:bg-white/5 transition-colors"
                    >
                      <td className={TD_NUM}>{i + 1}</td>
                      <td className={TD}>
                        <span
                          className="max-w-[220px] truncate inline-block align-middle"
                          style={{ color: '#af52de' }}
                          title={it.dept_name}
                        >
                          {it.dept_name}
                        </span>
                      </td>
                      <td className={TD_NUM}>¥{fmtCost(it.total_cost)}</td>
                      <td className={TD_NUM}>{PCT(it.cost_pct)}</td>
                      <td className={TD_NUM}>{pu == null ? '-' : `¥${fmtCost(pu)}`}</td>
                      <td className={TD_NUM}>¥{fmtCost(it.input_cost)}</td>
                      <td className={TD_NUM}>¥{fmtCost(it.output_cost)}</td>
                      <td className={TD_NUM}>{formatNumber(it.active_users)}</td>
                      <td className={TD_NUM} title={formatNumber(it.total_tokens)}>{shortToken(it.total_tokens)}</td>
                    </tr>
                  )
                })
              )}
            </tbody>
            {items.length > 0 && (
              <tfoot>
                <tr className="border-t border-gray-200/70 dark:border-white/10 font-semibold text-gray-600 dark:text-gray-300">
                  <td className={TD_NUM} colSpan={2}>
                    合计
                  </td>
                  <td className={TD_NUM}>¥{fmtCost(totals.total_cost)}</td>
                  <td className={TD_NUM}>100%</td>
                  <td className={TD_NUM}>{totals.per_user == null ? '-' : `¥${fmtCost(totals.per_user)}`}</td>
                  <td className={TD_NUM}>¥{fmtCost(totals.input_cost)}</td>
                  <td className={TD_NUM}>¥{fmtCost(totals.output_cost)}</td>
                  <td className={TD_NUM}>{formatNumber(totals.active_users)}</td>
                  <td className={TD_NUM} title={formatNumber(totals.total_tokens)}>{shortToken(totals.total_tokens)}</td>
                </tr>
              </tfoot>
            )}
          </table>
        </div>
      </ChartCard>

      {/* 各团队费用趋势折线 */}
      <ChartCard title={`各团队费用趋势（${GRANULARITY_CN[gran]}）`} sub="折线（多团队对齐，缺数据补 0）" extra={<GranularityToggle value={gran} options={granOptions} onChange={setGran} />}>
        {trendOpt ? <EChart option={trendOpt} height={300} /> : <EmptyHint />}
      </ChartCard>

      {/* 团队费用构成占比饼图 */}
      <ChartCard title="团队费用构成" sub="各团队费用占比">
        {pieOpt ? <EChart option={pieOpt} height={300} /> : <EmptyHint />}
      </ChartCard>
    </div>
  )
}
