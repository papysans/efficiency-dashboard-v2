// 组织详情页（OrgDetailV2 的 React + 玻璃拟态迁移）。
// 分区/列/口径/图表 1:1 按 research/pr3-user-repo-org.md §Org-7；⚠️ 百分比口径 → PercentPill（不 ×100）。
//
// 照搬陷阱（勿"修复"）：
//  ① 主数据走 /v2/orgs/detail（百分比口径）；级联选项走 getOrgV2(level,parent)，但 /v2/orgs 命中 native handler
//     **忽略 level/parent**，每次返回全部顶层 org → 四级级联在 native 下退化。React 照搬现状（不改成 listOrgV2）。
//  ② commit/task/user 跳转统一 encodeURIComponent（research 说现状未 encode，React 统一 encode 更稳，行为等价）。
// 图表 5 张（仅 commits/tasks 非空显示）；提效比趋势图 yAxis {value}%，**直接画原值不 ×100**（百分比口径）。
import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router'
import type { EChartsOption } from 'echarts'
import { getOrgV2 } from '@/api/endpoints'
import { useOrgDetail } from '@/api/queries'
import { useTheme } from '@/hooks/useTheme'
import type { CommitTimeSeriesItem, OrgMember, TaskTimeSeriesItem } from '@/api/types'
import { formatDuration, formatNumber } from '@/lib/formatters'
import { getDefaultDateRangeWide } from '@/lib/date'
import { getPalette } from '@/components/charts/chartTheme'
import { EChart } from '@/components/charts/EChart'
import { MetricCard } from '@/components/ui/MetricCard'
import { PercentPill } from '@/components/ui/PercentPill'
import { DateRangePicker } from '@/components/ui/DateRangePicker'

/** 费用单值（null → '-'，否则 2 位）。 */
function fmtCostVal(value: number | null | undefined): string {
  if (value == null) return '-'
  return Number(value).toFixed(2)
}

/** token K/M 缩写（对齐 Vue fmtTokens）。 */
function fmtTokens(up?: number | null, down?: number | null): string {
  const total = (up || 0) + (down || 0)
  if (total === 0) return '-'
  if (total >= 1_000_000) return `${(total / 1_000_000).toFixed(1)}M`
  if (total >= 1000) return `${(total / 1000).toFixed(1)}K`
  return String(total)
}

function normalizeDateQuery(value: string | null): string {
  if (!value) return ''
  const s = String(value).trim()
  if (/^\d{8}$/.test(s)) return `${s.slice(0, 4)}-${s.slice(4, 6)}-${s.slice(6, 8)}`
  if (/^\d{4}-\d{2}-\d{2}$/.test(s)) return s
  return ''
}

const GRANULARITY: Array<{ label: string; value: string }> = [
  { label: '天', value: 'day' },
  { label: '周', value: 'week' },
  { label: '月', value: 'month' },
  { label: '年', value: 'year' },
]

// 时序系列固定色（对齐 Vue）
const C_BLUE = '#409EFF'
const C_GREEN = '#67C23A'
const C_BLUE_LT = '#a0cfff'
const C_GREEN_LT = '#b3e19d'
const C_ORANGE = '#E6A23C'

const TH = 'px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_NUM = 'px-3 py-2 text-right font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_CENTER = 'px-3 py-2 text-center font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TD = 'px-3 py-2 align-middle text-gray-700 dark:text-gray-200'
const TD_NUM = 'px-3 py-2 align-middle text-right tabular-nums text-gray-700 dark:text-gray-200'

export default function OrgDetail() {
  const { orgPath: orgPathRaw } = useParams<{ orgPath: string }>()
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const { theme } = useTheme()

  // 路由 param 由 React Router 已 decode；按 '/' 拆级。
  const parts = useMemo(() => (orgPathRaw || '').split('/').filter(Boolean), [orgPathRaw])
  const [org1, setOrg1] = useState(parts[0] || '')
  const [org2, setOrg2] = useState(parts[1] || '')
  const [org3, setOrg3] = useState(parts[2] || '')
  const [org4, setOrg4] = useState(parts[3] || '')

  // URL param 变化时回填级联
  useEffect(() => {
    setOrg1(parts[0] || '')
    setOrg2(parts[1] || '')
    setOrg3(parts[2] || '')
    setOrg4(parts[3] || '')
  }, [parts])

  const [granularity, setGranularity] = useState('day')
  const [org1Options, setOrg1Options] = useState<string[]>([])
  const [org2Options, setOrg2Options] = useState<string[]>([])
  const [org3Options, setOrg3Options] = useState<string[]>([])
  const [org4Options, setOrg4Options] = useState<string[]>([])

  const dateRange = useMemo<[string, string]>(() => {
    const start = normalizeDateQuery(searchParams.get('startDate'))
    const end = normalizeDateQuery(searchParams.get('endDate'))
    if (start && end) return [start, end]
    return getDefaultDateRangeWide()
  }, [searchParams])

  const dateParams = useMemo(
    () => ({ startDate: dateRange[0].replace(/-/g, ''), endDate: dateRange[1].replace(/-/g, '') }),
    [dateRange],
  )

  const currentOrgPath = useMemo(() => [org1, org2, org3, org4].filter(Boolean).join('/'), [org1, org2, org3, org4])

  const { data, isLoading, error } = useOrgDetail({
    orgPath: currentOrgPath,
    ...dateParams,
    granularity,
  })

  const summary = data?.summary
  const members: OrgMember[] = useMemo(() => data?.members || [], [data?.members])
  const commits: CommitTimeSeriesItem[] = useMemo(() => data?.commits || [], [data?.commits])
  const tasks: TaskTimeSeriesItem[] = useMemo(() => data?.tasks || [], [data?.tasks])

  // 级联选项：getOrgV2(忽略 level/parent，返回全部顶层 org_name) —— 照搬 Vue native 退化行为。
  const loadOrgOptions = useCallback(async (): Promise<string[]> => {
    try {
      const res = await getOrgV2(dateParams)
      return (res.data || []).map((r) => r.org_name).filter(Boolean)
    } catch {
      return []
    }
  }, [dateParams])

  // 初始加载一级选项（及已选层级的下级选项）
  useEffect(() => {
    let aborted = false
    loadOrgOptions().then((opts) => {
      if (aborted) return
      setOrg1Options(opts)
      if (org1) setOrg2Options(opts)
      if (org2) setOrg3Options(opts)
      if (org3) setOrg4Options(opts)
    })
    return () => {
      aborted = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [loadOrgOptions])

  // 同步 orgPath 到 URL 并刷新（清空下级 + 加载下级选项由各 onChange 处理）。
  const syncUrlAndFetch = useCallback(
    (path: string) => {
      const q = new URLSearchParams({ startDate: dateParams.startDate, endDate: dateParams.endDate })
      navigate({ pathname: `/org/${encodeURIComponent(path)}`, search: `?${q.toString()}` }, { replace: true })
    },
    [dateParams, navigate],
  )

  async function onOrg1Change(val: string) {
    setOrg1(val)
    setOrg2('')
    setOrg3('')
    setOrg4('')
    if (val) setOrg2Options(await loadOrgOptions())
    syncUrlAndFetch([val].filter(Boolean).join('/'))
  }
  async function onOrg2Change(val: string) {
    setOrg2(val)
    setOrg3('')
    setOrg4('')
    if (val) setOrg3Options(await loadOrgOptions())
    syncUrlAndFetch([org1, val].filter(Boolean).join('/'))
  }
  async function onOrg3Change(val: string) {
    setOrg3(val)
    setOrg4('')
    if (val) setOrg4Options(await loadOrgOptions())
    syncUrlAndFetch([org1, org2, val].filter(Boolean).join('/'))
  }
  function onOrg4Change(val: string) {
    setOrg4(val)
    syncUrlAndFetch([org1, org2, org3, val].filter(Boolean).join('/'))
  }

  function onDateChange(range: [string, string]) {
    const next = new URLSearchParams(searchParams)
    next.set('startDate', range[0].replace(/-/g, ''))
    next.set('endDate', range[1].replace(/-/g, ''))
    setSearchParams(next, { replace: true })
  }

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

  if (error) {
    return (
      <div className="glass rounded-2xl p-8 text-center text-sm text-rose-600 dark:text-rose-400">
        {(error as Error).message || '获取组织详情失败'}
      </div>
    )
  }

  const selectCls =
    'glass rounded-lg px-2 py-1.5 text-sm bg-transparent cursor-pointer text-gray-700 dark:text-gray-200 ' +
    'focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue disabled:opacity-50 min-w-[120px]'

  return (
    <div className={`space-y-5 ${isLoading ? 'opacity-60 pointer-events-none' : ''}`}>
      {/* 标题栏 + 级联 + 粒度 */}
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-3 min-w-0">
          <button
            type="button"
            onClick={() => navigate(-1)}
            className="inline-flex items-center gap-1 text-sm text-gray-500 dark:text-gray-400 hover:text-apple-blue cursor-pointer bg-transparent border-none p-0 transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue rounded"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
            </svg>
            返回
          </button>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">组织详情</h1>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <select value={org1} onChange={(e) => onOrg1Change(e.target.value)} className={selectCls} aria-label="一级组织">
            <option value="">一级组织</option>
            {org1Options.map((o) => (
              <option key={o} value={o}>{o}</option>
            ))}
          </select>
          <select value={org2} onChange={(e) => onOrg2Change(e.target.value)} disabled={!org1} className={selectCls} aria-label="二级组织">
            <option value="">二级组织</option>
            {org2Options.map((o) => (
              <option key={o} value={o}>{o}</option>
            ))}
          </select>
          <select value={org3} onChange={(e) => onOrg3Change(e.target.value)} disabled={!org2} className={selectCls} aria-label="三级组织">
            <option value="">三级组织</option>
            {org3Options.map((o) => (
              <option key={o} value={o}>{o}</option>
            ))}
          </select>
          <select value={org4} onChange={(e) => onOrg4Change(e.target.value)} disabled={!org3} className={selectCls} aria-label="四级组织">
            <option value="">四级组织</option>
            {org4Options.map((o) => (
              <option key={o} value={o}>{o}</option>
            ))}
          </select>
          <DateRangePicker value={dateRange} onChange={onDateChange} />
          <select value={granularity} onChange={(e) => setGranularity(e.target.value)} className={`${selectCls} min-w-[80px]`} aria-label="粒度">
            {GRANULARITY.map((g) => (
              <option key={g.value} value={g.value}>{g.label}</option>
            ))}
          </select>
        </div>
      </header>

      {!currentOrgPath ? (
        <div className="glass rounded-2xl p-8 text-center text-sm text-gray-400 dark:text-gray-500">请选择组织以查看详情</div>
      ) : (
        <>
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
                          onClick={(e) => {
                            e.stopPropagation()
                            goUser(m.user_id)
                          }}
                        >
                          {m.user_name || m.user_id}
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
        </>
      )}
    </div>
  )
}

function Panel({ title, hint, children }: { title: string; hint?: string; children: ReactNode }) {
  return (
    <section className="glass rounded-2xl overflow-hidden">
      <div className="flex items-center justify-between px-5 py-3 border-b border-gray-200/50 dark:border-white/10">
        <span className="text-sm font-semibold text-gray-700 dark:text-gray-200">{title}</span>
        {hint && <span className="text-xs text-gray-400 dark:text-gray-500">{hint}</span>}
      </div>
      <div className="overflow-x-auto p-1">{children}</div>
    </section>
  )
}
