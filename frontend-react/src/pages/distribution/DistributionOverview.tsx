// 提效分布总览页：全量 Need 的双口径提效比分布 + 数据质量诊断。
// 数据源：useAllNeeds 拉两次（默认 kept + outlierOnly excluded）合并 = 看板口径内全量行；
// 分桶/统计全在前端（lib/distribution.ts），口径切换 + 手调粒度即时重算，不重新查询后端。
// URL query 单一数据源：startDate/endDate/caliber/bins（省默认值）。
import { useMemo } from 'react'
import { useNavigate, useSearchParams } from 'react-router'
import type { EChartsOption } from 'echarts'
import { useAllNeeds, useAllUsers, useProjectList, useRepos } from '@/api/queries'
import type { NeedsV2Summary, ProjectListItem, RepoListItem, UserV2Row } from '@/api/types'
import { useTheme } from '@/hooks/useTheme'
import { useUserNameMap } from '@/hooks/useUserNameMap'
import { useViewState } from '@/store/viewState'
import { formatDateParam } from '@/lib/date'
import { formatV2Ratio, formatNumber } from '@/lib/formatters'
import { sortRows } from '@/lib/sort'
import {
  computeDistribution,
  computeExclusionReasons,
  computeLocBands,
  computeQuantiles,
  GRANULARITY_PRESETS,
  MIN_BINS,
  MAX_BINS,
  type Caliber,
  type DistributionResult,
  type LocBand,
  type ReasonCount,
} from '@/lib/distribution'
import { Glass } from '@/components/ui/Glass'
import { MetricCard } from '@/components/ui/MetricCard'
import { Tag } from '@/components/ui/Tag'
import { RatioPill } from '@/components/ui/RatioPill'
import { PercentPill } from '@/components/ui/PercentPill'
import { EChart } from '@/components/charts/EChart'
import { getPalette, type ChartTheme } from '@/components/charts/chartTheme'

const DEFAULT_BINS = 6
const EXCLUDED_COLOR = '#fbbf24' // warn amber：被隔离
const REASON_COLOR = '#f87171' // neg rose：剔除原因条

// getAllNeedsV2 翻页上限 = MAX_PAGES(50) × PAGE_SIZE(200) = 1 万条/路；命中即被静默截断。
const PAGE_FETCH_LIMIT = 10000

/** ⑤ 排行维度 Tab。 */
type RankTab = 'user' | 'repo' | 'project'
const DEFAULT_TAB: RankTab = 'user'
const RANK_TOP_N = 10

// 日期来自全局 timeRange（顶部统一 picker），不再进 URL；URL 只持有 caliber/bins/tab（及壳的 object/sub）。
interface DistState {
  caliber: Caliber
  bins: number
  tab: RankTab
}

function readState(sp: URLSearchParams): DistState {
  const caliber: Caliber = sp.get('caliber') === 'work' ? 'work' : 'calendar'
  let bins = Number(sp.get('bins'))
  if (!Number.isFinite(bins) || bins <= 0) bins = DEFAULT_BINS
  bins = Math.max(MIN_BINS, Math.min(MAX_BINS, Math.round(bins)))
  const tabParam = sp.get('tab')
  const tab: RankTab = tabParam === 'repo' || tabParam === 'project' ? tabParam : DEFAULT_TAB
  return { caliber, bins, tab }
}

/** 把分布自身的 caliber/bins/tab 合并进现有 URL（保留壳的 object/sub/全局态；省默认值则删除该键）。 */
function applyToParams(sp: URLSearchParams, s: DistState): URLSearchParams {
  const next = new URLSearchParams(sp)
  if (s.caliber !== 'calendar') next.set('caliber', s.caliber)
  else next.delete('caliber')
  if (s.bins !== DEFAULT_BINS) next.set('bins', String(s.bins))
  else next.delete('bins')
  if (s.tab !== DEFAULT_TAB) next.set('tab', s.tab)
  else next.delete('tab')
  return next
}

/** 双口径堆叠直方图 option（kept 蓝 + excluded 黄堆叠；barOption 工厂不支持堆叠，故手写）。 */
function histogramOption(theme: ChartTheme, result: DistributionResult): EChartsOption {
  const p = getPalette(theme)
  const labels = result.histogram.map((b) => b.label)
  const kept = result.histogram.map((b) => b.kept)
  const excluded = result.histogram.map((b) => b.excluded)
  return {
    animation: true,
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' },
      backgroundColor: p.tooltipBg,
      borderColor: p.tooltipBorder,
      borderWidth: 1,
      textStyle: { color: p.tooltipText },
    },
    legend: { data: ['计入', '隔离'], top: 4, textStyle: { color: p.textColor } },
    grid: { left: '3%', right: '4%', top: 40, bottom: 24, containLabel: true },
    xAxis: {
      type: 'category',
      data: labels,
      axisLine: { lineStyle: { color: p.axisColor } },
      axisLabel: { color: p.textColor, fontSize: 11, hideOverlap: true, rotate: labels.length > 10 ? 30 : 0 },
    },
    yAxis: {
      type: 'value',
      axisLabel: { color: p.textColor },
      splitLine: { lineStyle: { color: p.splitLineColor } },
    },
    series: [
      { name: '计入', type: 'bar', stack: 'total', data: kept, itemStyle: { color: p.brand } },
      { name: '隔离', type: 'bar', stack: 'total', data: excluded, itemStyle: { color: EXCLUDED_COLOR } },
    ],
  }
}

/**
 * 横向条形 option（诊断模块用：降序 + 柱上「数值 (占比%)」标签）。
 * barOption 工厂只做纵向单系列，故手写。items 已按 value 降序由调用方传入。
 */
function horizontalBarOption(
  theme: ChartTheme,
  items: Array<{ label: string; value: number }>,
  color: string,
): EChartsOption {
  const p = getPalette(theme)
  const denom = items.reduce((s, it) => s + it.value, 0)
  // ECharts y 轴自下而上，反转数组使最大值显示在顶部。
  const ordered = [...items].reverse()
  return {
    animation: true,
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' },
      backgroundColor: p.tooltipBg,
      borderColor: p.tooltipBorder,
      borderWidth: 1,
      textStyle: { color: p.tooltipText },
      formatter: (params: unknown) => {
        const arr = params as Array<{ name: string; value: number }>
        const it = arr[0]
        const pct = denom > 0 ? ((it.value / denom) * 100).toFixed(1) : '0.0'
        return `${it.name}<br/>${formatNumber(it.value)} 个 · ${pct}%`
      },
    },
    grid: { left: '3%', right: '12%', top: 8, bottom: 8, containLabel: true },
    xAxis: {
      type: 'value',
      axisLine: { lineStyle: { color: p.axisColor } },
      axisLabel: { color: p.textColor, fontSize: 11 },
      splitLine: { lineStyle: { color: p.splitLineColor } },
    },
    yAxis: {
      type: 'category',
      data: ordered.map((it) => it.label),
      axisLine: { lineStyle: { color: p.axisColor } },
      axisLabel: { color: p.textColor, fontSize: 11 },
    },
    series: [
      {
        type: 'bar',
        data: ordered.map((it) => it.value),
        itemStyle: { color, borderRadius: [0, 4, 4, 0] },
        barWidth: '60%',
        label: {
          show: true,
          position: 'right',
          color: p.textColor,
          fontSize: 11,
          formatter: (param: { value?: unknown }) => {
            const v = Number(param.value) || 0
            const pct = denom > 0 ? ((v / denom) * 100).toFixed(1) : '0.0'
            return `${formatNumber(v)} (${pct}%)`
          },
        },
      },
    ],
  }
}

function medTone(v: number | null): 'pos' | 'neg' | 'neutral' {
  if (v == null) return 'neutral'
  return v < 0 ? 'neg' : 'pos'
}

const H2 = 'text-sm font-semibold text-gray-700 dark:text-gray-200'

/** 分段控件（口径 / 粒度切换；单页使用，不抽全局）。 */
function Segmented<T extends string | number>({
  value,
  options,
  onChange,
}: {
  value: T
  options: Array<{ label: string; value: T }>
  onChange: (v: T) => void
}) {
  return (
    <div className="inline-flex glass rounded-lg p-0.5 gap-0.5">
      {options.map((o) => (
        <button
          key={String(o.value)}
          type="button"
          onClick={() => onChange(o.value)}
          className={`px-3 py-1 rounded-md text-sm font-medium cursor-pointer transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue ${
            o.value === value
              ? 'bg-apple-blue text-white'
              : 'text-gray-600 dark:text-gray-300 hover:text-apple-blue'
          }`}
        >
          {o.label}
        </button>
      ))}
    </div>
  )
}

/** 诊断单图：横向条形 + 数值/占比标签；全 0 时给友好提示不画空图。 */
function DiagnosticBar({
  theme,
  title,
  hint,
  items,
  color,
  loading,
}: {
  theme: ChartTheme
  title: string
  hint: string
  items: Array<{ label: string; value: number }>
  color: string
  loading: boolean
}) {
  const sum = items.reduce((s, it) => s + it.value, 0)
  const option = useMemo(() => horizontalBarOption(theme, items, color), [theme, items, color])
  return (
    <div>
      <div className="flex items-baseline justify-between mb-2">
        <h3 className="text-sm font-medium text-gray-600 dark:text-gray-300">{title}</h3>
        <span className="text-xs text-gray-400 dark:text-gray-500">{hint}</span>
      </div>
      {loading ? (
        <div className="skeleton h-44 rounded-xl" />
      ) : sum === 0 ? (
        <div className="h-44 flex items-center justify-center text-sm text-gray-400 dark:text-gray-500">
          该时间段暂无数据
        </div>
      ) : (
        <EChart option={option} height={176} />
      )}
    </div>
  )
}

/** ⑤ 排行单条：排名徽章 + 名称(可点跳详情) + 进度条 + 口径胶囊。 */
function RankBar({
  rank,
  title,
  sub,
  ratio,
  maxRatio,
  pill,
  onClick,
}: {
  rank: number
  title: string
  sub?: string
  ratio: number
  maxRatio: number
  pill: React.ReactNode
  onClick: () => void
}) {
  // 进度条长度按当前 Tab 内最大值归一（同 Tab 同口径，长度可比）；负值不画条。
  const pct = maxRatio > 0 ? Math.max(0, Math.min(100, (ratio / maxRatio) * 100)) : 0
  return (
    <li>
      <button
        type="button"
        onClick={onClick}
        className="w-full flex items-center gap-3 rounded-xl px-2 py-1.5 text-left bg-transparent border-none cursor-pointer hover:bg-white/40 dark:hover:bg-white/5 transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue"
      >
        <span className="shrink-0 w-6 text-right text-xs font-bold tabular-nums text-gray-400 dark:text-gray-500">
          {rank}
        </span>
        <div className="min-w-0 flex-1">
          <div className="flex items-center justify-between gap-2">
            <span className="text-sm font-medium text-gray-900 dark:text-white truncate" title={title}>
              {title}
            </span>
            <span className="shrink-0">{pill}</span>
          </div>
          <div className="mt-1 h-1.5 rounded-full bg-gray-200/70 dark:bg-white/10 overflow-hidden">
            <div className="h-full rounded-full bg-apple-blue transition-[width]" style={{ width: `${pct}%` }} />
          </div>
          {sub && (
            <div className="text-xs text-gray-400 dark:text-gray-500 truncate mt-0.5" title={sub}>
              {sub}
            </div>
          )}
        </div>
      </button>
    </li>
  )
}

/** ⑤ 排行列表通用壳：loading 骨架 / error / empty / 渲染。 */
function RankList({
  loading,
  error,
  empty,
  children,
}: {
  loading: boolean
  error: Error | null
  empty: boolean
  children: React.ReactNode
}) {
  if (error) {
    return (
      <div className="flex items-center justify-center text-sm text-rose-600 dark:text-rose-400 min-h-[18rem]">
        加载失败：{error.message}
      </div>
    )
  }
  if (loading) {
    return (
      <ul className="space-y-2">
        {Array.from({ length: 6 }).map((_, i) => (
          <li key={i} className="skeleton h-12 rounded-xl" />
        ))}
      </ul>
    )
  }
  if (empty) {
    return (
      <div className="flex items-center justify-center text-sm text-gray-400 dark:text-gray-500 min-h-[18rem]">
        该时间段暂无可计入排行的数据
      </div>
    )
  }
  return <ul className="space-y-2">{children}</ul>
}

/** ⑤ 用户 Top 排行（小数口径 → RatioPill；点击跳 /user/{id}）。 */
function UserRanking({ startDate, endDate }: { startDate: string; endDate: string }) {
  const navigate = useNavigate()
  const { resolveName } = useUserNameMap()
  const q = useAllUsers({ startDate, endDate })
  const top = useMemo<UserV2Row[]>(() => {
    const rows = (q.data ?? []).filter((r) => r.calendar_ratio != null && Number.isFinite(Number(r.calendar_ratio)))
    return sortRows(rows, (r) => r.calendar_ratio, true).slice(0, RANK_TOP_N)
  }, [q.data])
  const maxRatio = top.length > 0 ? Number(top[0].calendar_ratio) : 0
  return (
    <RankList loading={q.isLoading} error={(q.error as Error) ?? null} empty={top.length === 0}>
      {top.map((r, i) => (
        <RankBar
          key={r.user_id}
          rank={i + 1}
          title={resolveName(r.user_id)}
          sub={`合并需求 ${formatNumber(r.merged_need_count)}`}
          ratio={Number(r.calendar_ratio)}
          maxRatio={maxRatio}
          pill={<RatioPill value={r.calendar_ratio} />}
          onClick={() => navigate(`/user/${encodeURIComponent(r.user_id)}`)}
        />
      ))}
    </RankList>
  )
}

/** ⑤ 仓库 Top 排行（百分比口径 → PercentPill；点击跳 /repo/{addr}/{branch}）。 */
function RepoRanking({ startDate, endDate }: { startDate: string; endDate: string }) {
  const navigate = useNavigate()
  // pageSize 拉大一次性取回，客户端排序取 Top10（对齐 RepoList 客户端排序口径）。
  const q = useRepos({ startDate, endDate, page: 1, pageSize: 1000 })
  const top = useMemo<RepoListItem[]>(() => {
    const rows = (q.data?.data ?? []).filter(
      (r) => r.efficiency_ratio != null && Number.isFinite(Number(r.efficiency_ratio)),
    )
    return sortRows(rows, (r) => r.efficiency_ratio, true).slice(0, RANK_TOP_N)
  }, [q.data])
  const maxRatio = top.length > 0 ? Number(top[0].efficiency_ratio) : 0
  return (
    <RankList loading={q.isLoading} error={(q.error as Error) ?? null} empty={top.length === 0}>
      {top.map((r, i) => (
        <RankBar
          key={`${r.repo_addr}#${r.repo_branch}`}
          rank={i + 1}
          title={r.repo_addr || '-'}
          sub={`${r.repo_branch || '-'} · Commit ${formatNumber(r.commit_count)}`}
          ratio={Number(r.efficiency_ratio)}
          maxRatio={maxRatio}
          pill={<PercentPill value={r.efficiency_ratio} />}
          onClick={() =>
            navigate(`/repo/${encodeURIComponent(r.repo_addr)}/${encodeURIComponent(r.repo_branch || '')}`)
          }
        />
      ))}
    </RankList>
  )
}

/**
 * ⑤ 项目 Top 排行（小数口径 → RatioPill；点击跳 /project/{id}）。无分页、与日期无关。
 * 用 need_calendar_efficiency_ratio（小数口径，与项目列表/详情页同源）——古法 efficiency_ratio
 * 是百分比口径已不再展示，用它会让从本页点进详情数字对不上。
 */
function ProjectRanking() {
  const navigate = useNavigate()
  const q = useProjectList()
  const top = useMemo<ProjectListItem[]>(() => {
    const rows = (q.data?.data ?? []).filter(
      (r) =>
        r.need_calendar_efficiency_ratio != null &&
        Number.isFinite(Number(r.need_calendar_efficiency_ratio)),
    )
    return sortRows(rows, (r) => r.need_calendar_efficiency_ratio, true).slice(0, RANK_TOP_N)
  }, [q.data])
  const maxRatio = top.length > 0 ? Number(top[0].need_calendar_efficiency_ratio) : 0
  return (
    <RankList loading={q.isLoading} error={(q.error as Error) ?? null} empty={top.length === 0}>
      {top.map((r, i) => (
        <RankBar
          key={r.project_id}
          rank={i + 1}
          title={r.name || r.project_id}
          sub={r.repo_count != null ? `仓库 ${formatNumber(r.repo_count)} 个` : undefined}
          ratio={Number(r.need_calendar_efficiency_ratio)}
          maxRatio={maxRatio}
          pill={<RatioPill value={r.need_calendar_efficiency_ratio} />}
          onClick={() => navigate(`/project/${encodeURIComponent(r.project_id)}`)}
        />
      ))}
    </RankList>
  )
}

export default function DistributionOverview() {
  const { theme } = useTheme()
  const [sp, setSp] = useSearchParams()
  // 全局时间范围（顶部统一 DateRangePicker）——本页不再有自己的日期 picker。
  const { timeRange } = useViewState()
  const state = useMemo(() => readState(sp), [sp])
  const { caliber, bins, tab } = state
  const [startStr, endStr] = timeRange
  const caliberLabel = caliber === 'calendar' ? '日历' : '人力'

  // 拉两次合并：默认(NOT outlier)=kept，outlierOnly=excluded；都在看板口径内（不传 includeAll）。
  const startParam = formatDateParam(startStr)
  const endParam = formatDateParam(endStr)
  const keptQ = useAllNeeds({ startDate: startParam, endDate: endParam })
  const exclQ = useAllNeeds({ startDate: startParam, endDate: endParam, outlierOnly: true })

  // 隐含契约：kept 集(默认 NOT outlier_flag) ∪ outlierOnly 集 = 看板口径内全部行（不传 includeAll）。
  // 合并后前端再按当前口径 flag（calendar_outlier_flag / work_outlier_flag）重分 kept/excluded，
  // 故 outlier_flag 仅用于「取全集」，不直接当口径判据。正确性依赖两路都未被翻页上限截断
  // —— 任一路命中 PAGE_FETCH_LIMIT 即由下方 truncated 横幅兜底提示。
  const rows = useMemo<NeedsV2Summary[]>(() => {
    const seen = new Set<string>()
    const out: NeedsV2Summary[] = []
    for (const r of [...(keptQ.data ?? []), ...(exclQ.data ?? [])]) {
      if (r && r.need_id && !seen.has(r.need_id)) {
        seen.add(r.need_id)
        out.push(r)
      }
    }
    return out
  }, [keptQ.data, exclQ.data])

  const loading = keptQ.isLoading || exclQ.isLoading
  const error = (keptQ.error ?? exclQ.error) as Error | null

  // 手调粒度/切口径即时重算：依赖 caliber/bins 的 useMemo，不触发请求（queryKey 仅含日期）。
  const dist = useMemo(() => computeDistribution(rows, caliber, bins), [rows, caliber, bins])
  const calQ = useMemo(() => computeQuantiles(rows, 'calendar'), [rows])
  const workQ = useMemo(() => computeQuantiles(rows, 'work'), [rows])
  const curQ = caliber === 'calendar' ? calQ : workQ
  const option = useMemo(() => histogramOption(theme, dist), [theme, dist])

  // ④ 诊断：剔除原因(3 类，按隔离总数占比) + LOC 速率分档(4 档)；纯函数，随 rows 重算。
  const reasonItems = useMemo(
    () =>
      sortRows(computeExclusionReasons(rows), (r: ReasonCount) => r.count, true).map((r) => ({
        label: r.label,
        value: r.count,
      })),
    [rows],
  )
  const locItems = useMemo(
    () =>
      sortRows(computeLocBands(rows), (b: LocBand) => b.count, true).map((b) => ({
        label: b.label,
        value: b.count,
      })),
    [rows],
  )

  // 截断探针：kept / excluded 任一路命中翻页上限即说明被静默截断（不能比 serverTotal——
  // total 只是 kept 总数，而 rows 是 kept∪excluded，比较会恒为 false）。
  const truncated =
    (keptQ.data?.length ?? 0) >= PAGE_FETCH_LIMIT || (exclQ.data?.length ?? 0) >= PAGE_FETCH_LIMIT

  const latest = useMemo(() => {
    let m = ''
    for (const r of rows) if (r.dev_end_ts && r.dev_end_ts > m) m = r.dev_end_ts
    return m
  }, [rows])

  const total = dist.keptCount + dist.excludedCount
  const exclPct = total > 0 ? (dist.excludedCount / total) * 100 : 0

  function commit(next: DistState) {
    setSp(applyToParams(sp, next), { replace: true })
  }

  return (
    <div className="space-y-5">
      {/* header */}
      <header className="space-y-3">
        <p className="text-sm text-gray-500 dark:text-gray-400">
          全量 Need 的提效比分布与数据质量诊断。可切换日历 / 人力口径、手调分档粒度，即时重算（不重新查询）。
        </p>
        <div className="flex flex-wrap items-center gap-3">
          <div className="flex items-center gap-1.5">
            <span className="text-xs text-gray-400 dark:text-gray-500">口径</span>
            <Segmented<Caliber>
              value={caliber}
              options={[
                { label: '日历', value: 'calendar' },
                { label: '人力', value: 'work' },
              ]}
              onChange={(c) => commit({ ...state, caliber: c })}
            />
          </div>
          <div className="flex items-center gap-1.5">
            <span className="text-xs text-gray-400 dark:text-gray-500">粒度</span>
            <Segmented<number>
              value={bins}
              options={GRANULARITY_PRESETS.map((g) => ({ label: `${g.label} ${g.bins}`, value: g.bins }))}
              onChange={(b) => commit({ ...state, bins: b })}
            />
          </div>
        </div>
      </header>

      {error && (
        <div className="text-sm text-rose-600 dark:text-rose-400 bg-rose-50/50 dark:bg-rose-900/20 rounded-lg px-4 py-2">
          加载失败：{error.message}
        </div>
      )}

      {truncated && (
        <div className="flex items-center gap-2 text-sm bg-amber-50/60 dark:bg-amber-900/20 rounded-lg px-4 py-2">
          <Tag tone="warning">数据量较大</Tag>
          <span className="text-amber-700 dark:text-amber-300">
            单次拉取已达上限（约 1 万条/路），结果可能不完整，请缩小日期范围。
          </span>
        </div>
      )}

      {/* ① 健康横幅 */}
      <div className="grid grid-cols-2 md:grid-cols-5 gap-3">
        <MetricCard label="计入 Need" value={formatNumber(dist.keptCount)} hint={`${caliberLabel}口径`} accent="#0071e3" />
        <MetricCard
          label="被隔离 Need"
          value={formatNumber(dist.excludedCount)}
          hint={`占 ${exclPct.toFixed(1)}%`}
          tone={exclPct > 20 ? 'neg' : 'neutral'}
          accent={EXCLUDED_COLOR}
        />
        <MetricCard label="日历中位提效" value={formatV2Ratio(calQ.median)} tone={medTone(calQ.median)} />
        <MetricCard label="人力中位提效" value={formatV2Ratio(workQ.median)} tone={medTone(workQ.median)} />
        <MetricCard label="数据更新至" value={latest ? latest.slice(0, 10) : '—'} hint="最新 Need 完成日" />
      </div>

      {/* ② 双口径提效比分布直方图 */}
      <Glass className="p-5">
        <div className="flex items-center justify-between mb-2">
          <h2 className={H2}>提效比分布 · {caliberLabel}口径</h2>
          <span className="text-xs text-gray-400 dark:text-gray-500">
            蓝=计入 / 黄=隔离 · 共 {formatNumber(total)} 个 Need
          </span>
        </div>
        {loading ? (
          <div className="skeleton h-[320px] rounded-xl" />
        ) : total === 0 ? (
          <div className="h-[320px] flex items-center justify-center text-sm text-gray-400 dark:text-gray-500">
            该时间段暂无可计入的 Need 数据
          </div>
        ) : (
          <EChart option={option} height={320} />
        )}
      </Glass>

      {/* ③ 分位数（当前口径） */}
      <Glass className="p-5">
        <h2 className={H2}>
          分位数 · {caliberLabel}口径
          <span className="ml-2 text-xs font-normal text-gray-400 dark:text-gray-500">
            计入 {formatNumber(curQ.count)} 个
          </span>
        </h2>
        <div className="grid grid-cols-3 gap-3 mt-3">
          <MetricCard label="P25" value={formatV2Ratio(curQ.p25)} tone={medTone(curQ.p25)} />
          <MetricCard label="中位数" value={formatV2Ratio(curQ.median)} tone={medTone(curQ.median)} />
          <MetricCard label="P75" value={formatV2Ratio(curQ.p75)} tone={medTone(curQ.p75)} />
        </div>
      </Glass>

      {/* ④ 数据质量诊断：左=剔除原因(3 类，占隔离总数) / 右=LOC 速率分档(4 档) */}
      <Glass className="p-5">
        <div className="flex items-center justify-between mb-3">
          <h2 className={H2}>数据质量诊断</h2>
          <span className="text-xs text-gray-400 dark:text-gray-500">降序 · 柱上为「数量 (占比%)」</span>
        </div>
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
          <DiagnosticBar
            theme={theme}
            title="剔除原因"
            hint={`占被隔离 ${formatNumber(dist.excludedCount)} 个`}
            items={reasonItems}
            color={REASON_COLOR}
            loading={loading}
          />
          <DiagnosticBar
            theme={theme}
            title="LOC 速率分档"
            hint="行/分钟 · 计入候选"
            items={locItems}
            color={getPalette(theme).brand}
            loading={loading}
          />
        </div>
      </Glass>

      {/* ⑤ 维度横向对比 Top 排行（Tab：用户/仓库/项目；口径分开标注，点击进详情） */}
      <Glass className="p-5">
        <div className="flex flex-wrap items-center justify-between gap-2 mb-3">
          <h2 className={H2}>维度横向对比 Top {RANK_TOP_N}</h2>
          <div className="flex items-center gap-2">
            <span className="text-xs text-gray-400 dark:text-gray-500">
              {tab === 'repo' ? '百分比口径' : '小数口径'}
            </span>
            <Segmented<RankTab>
              value={tab}
              options={[
                { label: '用户', value: 'user' },
                { label: '仓库', value: 'repo' },
                { label: '项目', value: 'project' },
              ]}
              onChange={(t) => commit({ ...state, tab: t })}
            />
          </div>
        </div>
        {tab === 'user' && <UserRanking startDate={startParam} endDate={endParam} />}
        {tab === 'repo' && <RepoRanking startDate={startParam} endDate={endParam} />}
        {tab === 'project' && <ProjectRanking />}
      </Glass>
    </div>
  )
}
