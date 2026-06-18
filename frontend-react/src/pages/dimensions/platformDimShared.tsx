// 个人维度（user）平台数据接入的共享 UI 小件：
//   - PlatformNotConnected：chat_stats_enabled=false 或平台请求失败时的优雅占位（降级护栏一等公民）。
//   - PlatformWeekTrend：把周窗口序列（lib/weekWindows 切窗 + chatStats 各窗查询）渲染为按周折线，
//     复用 platformShared 的 multiAreaOption / ChartCard / EmptyHint，对齐玻璃拟态。
// 仅个人维度内部复用。
import { useCallback, useMemo } from 'react'
import { useSearchParams } from 'react-router'
import { useTheme } from '@/hooks/useTheme'
import { EChart } from '@/components/charts/EChart'
import { getPalette } from '@/components/charts/chartTheme'
import { ChartCard, EmptyHint, multiAreaOption, type AreaSeries } from '@/pages/platform/platformShared'
import { weekWindowLabel, type WeekWindow } from '@/lib/weekWindows'

/**
 * 部门排行行下钻 → 写 ?object=<dept_id> 进 org 主体聚焦态（与 OrgContribution.goDept 同源范式）。
 * 部门没有独立详情路由，故下钻=切到壳的聚焦态（保留其它 query，清 dept_id/sub）。
 * 供 Usage/Quality/Cost 三个平台 org 维度复用，避免三处复制。
 */
export function useDeptFocus(): (deptId: string) => void {
  const [searchParams, setSearchParams] = useSearchParams()
  return useCallback(
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
}

/**
 * 平台未接入 / 请求失败时的统一优雅占位（玻璃拟态卡）。
 * reason='disabled'（开关 false）与 reason='error'（请求失败）文案不同，但都不空页、不抛错。
 * 复用 ChatDisabledNotice 的语义但更轻（内嵌在维度内容里，壳/Tab 仍在）。
 */
export function PlatformNotConnected({
  reason = 'disabled',
  detail,
}: {
  reason?: 'disabled' | 'error'
  detail?: string
}) {
  const title = reason === 'error' ? '平台数据暂不可用' : '未接入平台数据'
  const body =
    reason === 'error'
      ? '平台指标服务请求失败（网络或上游异常）。本维度依赖平台客观采集，恢复后将自动展示。'
      : '当前环境未启用平台指标服务（chat_stats_enabled=false），本维度依赖平台（chat-indicator-statistics）客观采集数据。配置平台源后将自动展示。'
  return (
    <div className="glass rounded-2xl p-10 text-center">
      <div className="mx-auto mb-3 w-10 h-10 text-gray-300 dark:text-gray-600">
        <svg fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={1.5}
            d="M3.055 11H5a2 2 0 012 2v1a2 2 0 002 2 2 2 0 012 2v2.945M8 3.935V5.5A2.5 2.5 0 0010.5 8h.5a2 2 0 012 2 2 2 0 104 0 2 2 0 012-2h1.064M15 20.488V18a2 2 0 012-2h3.064M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
          />
        </svg>
      </div>
      <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-2">{title}</h2>
      <p className="mx-auto max-w-md text-sm text-gray-500 dark:text-gray-400">{body}</p>
      {detail && <p className="mt-2 text-xs text-gray-400 dark:text-gray-500">{detail}</p>}
    </div>
  )
}

/**
 * 截断脚注（P1-2）：聚合按周/全量排行只拉 Top AGG_PAGE_SIZE 名，区间真实人数更大时数据被截断。
 * 对齐 Usage 排行「共 N 人」风格，醒目标注「基于 Top 500，区间共 X」。total 未知则只说基于 Top 500。
 */
export function TruncationNote({
  total,
  pageSize = 500,
  className = '',
}: {
  total?: number
  pageSize?: number
  className?: string
}) {
  return (
    <p className={`text-xs text-amber-600 dark:text-amber-400 flex items-center gap-1 ${className}`} role="note">
      <svg className="w-3.5 h-3.5 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
      </svg>
      <span>
        数据基于 Top {pageSize.toLocaleString()}
        {total != null ? `，区间共 ${total.toLocaleString()} 人` : ''}（超出部分未计入，聚合值偏小）
      </span>
    </p>
  )
}

/**
 * 直属成员标注（P1-3）：组织平台维度的**聚焦态**只统计该部门直属成员（dept-tree/members 非递归），
 * 选含子部门的父部门会偏小。在 org 聚焦态 render 醒目加一行「仅直属成员，子部门未计入」。
 * 共享组件，供 Usage/Quality/Cost/Contribution 四处 org 聚焦复用，避免四处复制文案。
 * 固定文案（不判断是否真有子部门，统一提醒口径）。
 */
export function DirectMembersNote({ className = '' }: { className?: string }) {
  return (
    <div
      className={`glass rounded-xl px-4 py-2.5 flex items-start gap-2 text-xs border-l-4 ${className}`}
      style={{ borderLeftColor: '#ff9500' }}
      role="note"
    >
      <svg className="w-4 h-4 shrink-0 text-amber-500 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01M5.07 19H19a2 2 0 001.74-2.99l-7-12a2 2 0 00-3.48 0l-7 12A2 2 0 005.07 19z" />
      </svg>
      <span className="text-gray-600 dark:text-gray-300">
        <b className="text-gray-900 dark:text-white">仅直属成员，子部门未计入</b>。该部门花名册为<b>直属成员</b>（dept-tree/members 非递归），
        选含子部门的父部门，聚合值会偏小。
      </span>
    </div>
  )
}

/** config 加载中的轻骨架（趋势卡 + KPI 行占位），避免误判降级闪占位。 */
export function DimSkeleton() {
  return (
    <div className="flex flex-col gap-5">
      <div className="skeleton h-72 rounded-2xl" />
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        {Array.from({ length: 4 }).map((_, i) => (
          <div key={i} className="skeleton h-24 rounded-2xl" />
        ))}
      </div>
    </div>
  )
}

/** 一条周序列（label = MM/DD 周一；name/color 来自调用方）。 */
export interface WeekSeriesSpec {
  name: string
  color: string
  /** 与 windows 一一对应的数值（缺数据点用 0 或 null 由调用方决定）。 */
  values: number[]
}

/**
 * 按周折线趋势卡（个人维度时间线复用）。windows 来自 sliceWeekWindows；series 各点与 windows 对齐。
 * 数据全空（hasAny=false）时走诚实空态（与 DimensionTrend 的「积累中」一致语气），不画空图。
 */
export function PlatformWeekTrend({
  title,
  subtitle,
  windows,
  series,
  loading = false,
  error = null,
  hasAny,
  yFmt,
  yMax,
}: {
  title: string
  subtitle?: string
  windows: WeekWindow[]
  series: WeekSeriesSpec[]
  loading?: boolean
  error?: string | null
  hasAny: boolean
  yFmt?: (v: number) => string
  yMax?: number
}) {
  const { theme } = useTheme()
  const p = useMemo(() => getPalette(theme), [theme])
  const labels = windows.map((w) => weekWindowLabel(w.monday))
  const areaSeries: AreaSeries[] = series.map((s) => ({ name: s.name, color: s.color, data: s.values }))
  const option = useMemo(
    () => multiAreaOption(p, labels, areaSeries, { yFmt, yMax }),
    // labels/areaSeries 由 windows/series 派生，依赖原始入参即可
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [p, windows, series, yFmt, yMax],
  )

  return (
    <ChartCard title={title} sub={subtitle}>
      {error ? (
        <div className="flex items-center justify-center h-[260px] text-sm text-rose-600 dark:text-rose-400">
          加载失败：{error}
        </div>
      ) : loading ? (
        <div className="skeleton rounded-xl h-[260px]" />
      ) : !hasAny || windows.length === 0 ? (
        <EmptyHint />
      ) : (
        <EChart option={option} height={260} />
      )}
    </ChartCard>
  )
}
