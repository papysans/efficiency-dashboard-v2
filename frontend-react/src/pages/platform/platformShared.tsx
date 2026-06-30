// 平台（chat-indicator-statistics 代理）三个子页的共享小件：子页 tab、token 缩写、错误码判断、
// 饼图色板、universal_id → 看板用户互链单元格、ECharts option 工厂与图表卡片。
// 仅平台页内部复用，不进 src/api/ 或全局组件（避免与并行任务冲突）。
import { useMemo, useState, type ReactNode } from 'react'
import { Link } from 'react-router'
import * as echarts from 'echarts'
import type { EChartsOption } from 'echarts'
import type { ChartPalette } from '@/components/charts/chartTheme'

/** 饼图/多系列色板（对齐 chat 侧 Apple 系配色，主色换看板 Apple Blue）。 */
export const PIE_COLORS = [
  '#0071e3',
  '#34c759',
  '#ff9500',
  '#ff3b30',
  '#af52de',
  '#5856d6',
  '#5ac8fa',
  '#ff2d55',
  '#8e8e93',
  '#ffd60a',
]

/** token 数缩写：1.2K / 3.4M / 1.05B。空/非数 => '-'。 */
export function shortToken(v: number | null | undefined): string {
  if (v == null || !Number.isFinite(Number(v))) return '-'
  const n = Number(v)
  if (n >= 1e9) return `${(n / 1e9).toFixed(2)}B`
  if (n >= 1e6) return `${(n / 1e6).toFixed(1)}M`
  if (n >= 1e3) return `${(n / 1e3).toFixed(1)}K`
  return String(n)
}

/** chat 侧错误判定口径：error_code 非空且非 '0' 才算错误（对齐其 Query.jsx isError）。 */
export function isErrorCode(code: string | null | undefined): boolean {
  return !!code && code !== '0'
}

/**
 * universal_id → 看板用户互链单元格。
 * chat.universal_id 与看板 user_id 同源（research/t6-universal-id-verify.md 实测结论），
 * 故复用 useUserNameMap 的 resolveName：命中（返回值 ≠ 原 id）→ 显示看板用户名并 Link 到 /user/:userId；
 * 解析不到 → 回退 chat 侧 username，再退截断 UUID（前 8 位）。
 * 映射加载失败/未就绪时 resolveName 原样返回 id，自然落入回退分支，不阻塞主数据渲染。
 */
export function ChatUserCell({
  universalId,
  chatUsername,
  resolveName,
}: {
  universalId: string | null | undefined
  chatUsername: string | null | undefined
  resolveName: (userId?: string) => string
}) {
  const uid = universalId || ''
  const resolved = uid ? resolveName(uid) : ''
  if (uid && resolved && resolved !== uid && resolved !== '-') {
    return (
      <Link
        to={`/user/${encodeURIComponent(uid)}`}
        onClick={(e) => e.stopPropagation()}
        className="text-apple-blue hover:text-apple-blue-hover no-underline focus:outline-none focus-visible:underline"
        title={`${resolved} · 查看看板用户详情`}
      >
        {resolved}
      </Link>
    )
  }
  const fallback = (chatUsername || '').trim() || (uid ? `${uid.slice(0, 8)}…` : '')
  return fallback ? <span title={uid || undefined}>{fallback}</span> : <span>-</span>
}

// ---- ECharts option 工厂 + 图表卡片（态势页 / 总览页共用） ----

function rgba(hex: string, alpha: number): string {
  const r = parseInt(hex.slice(1, 3), 16)
  const g = parseInt(hex.slice(3, 5), 16)
  const b = parseInt(hex.slice(5, 7), 16)
  return `rgba(${r},${g},${b},${alpha})`
}

export function baseTooltip(p: ChartPalette) {
  return {
    backgroundColor: p.tooltipBg,
    borderColor: p.tooltipBorder,
    borderWidth: 1,
    textStyle: { color: p.tooltipText },
  }
}

export interface AreaSeries {
  name: string
  color: string
  data: number[]
}

/**
 * 折线+渐变面积图（分钟/趋势通用）。yFmt 控制 y 轴刻度格式（token 缩写 / 百分比 / 金额）。
 * headers 提供时（按周/月聚合），tooltip 头部用 headers[dataIndex]（日期范围）替代 x 轴标签，
 * 各系列值按 yFmt 格式化。
 */
export function multiAreaOption(
  p: ChartPalette,
  times: string[],
  series: AreaSeries[],
  opts: { yFmt?: (v: number) => string; yMax?: number; headers?: string[] } = {},
): EChartsOption {
  const headers = opts.headers
  const valFmt = opts.yFmt ?? ((v: number) => String(v))
  return {
    animation: true,
    grid: { left: 8, right: 16, top: series.length > 1 ? 36 : 24, bottom: 8, containLabel: true },
    tooltip: {
      trigger: 'axis',
      ...baseTooltip(p),
      ...(headers
        ? {
            formatter: (params: unknown) => {
              const arr = params as { dataIndex: number; seriesName: string; value: number; marker: string; axisValue: string }[]
              const head = headers[arr[0]?.dataIndex] ?? arr[0]?.axisValue ?? ''
              const body = arr.map((it) => `${it.marker}${it.seriesName}: ${valFmt(it.value)}`).join('<br/>')
              return `${head}<br/>${body}`
            },
          }
        : {}),
    },
    legend:
      series.length > 1
        ? { top: 0, left: 'center', textStyle: { color: p.textColor }, itemWidth: 14, itemHeight: 8 }
        : undefined,
    xAxis: {
      type: 'category',
      data: times,
      boundaryGap: false,
      axisLine: { lineStyle: { color: p.axisColor } },
      axisLabel: { color: p.textColor, hideOverlap: true },
      axisTick: { show: false },
    },
    yAxis: {
      type: 'value',
      max: opts.yMax,
      axisLabel: { color: p.textColor, formatter: opts.yFmt },
      splitLine: { lineStyle: { color: p.splitLineColor } },
    },
    series: series.map((s) => ({
      name: s.name,
      type: 'line',
      smooth: true,
      symbol: 'none',
      data: s.data,
      lineStyle: { color: s.color, width: 2 },
      itemStyle: { color: s.color },
      areaStyle: {
        color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
          { offset: 0, color: rgba(s.color, 0.25) },
          { offset: 1, color: rgba(s.color, 0) },
        ]),
      },
    })),
  }
}

/** 玻璃拟态图表/表格卡片。extra 放在标题行右侧（如模型筛选下拉）。 */
export function ChartCard({
  title,
  sub,
  extra,
  children,
}: {
  title: ReactNode
  sub?: string
  extra?: ReactNode
  children: ReactNode
}) {
  return (
    <section className="glass rounded-2xl p-5 hover:shadow-lg transition-shadow">
      <div className="flex items-center justify-between gap-3 mb-3">
        <h2 className="text-sm font-semibold text-gray-700 dark:text-gray-200">{title}</h2>
        {sub && !extra && <span className="text-xs text-gray-400 dark:text-gray-500">{sub}</span>}
        {extra && (
          <div className="flex items-center gap-2">
            {sub && <span className="text-xs text-gray-400 dark:text-gray-500">{sub}</span>}
            {extra}
          </div>
        )}
      </div>
      {children}
    </section>
  )
}

export function EmptyHint({ compact = false }: { compact?: boolean }) {
  return (
    <div
      className={`flex items-center justify-center text-sm text-gray-400 dark:text-gray-500 ${
        compact ? 'py-10' : 'h-[260px]'
      }`}
    >
      暂无数据
    </div>
  )
}

/**
 * 模型用量/成本表统一过滤：默认隐藏 request_count=0 的模型，开关可展开。
 * items 来自 react-query（稳定引用），visible 随 showZero/items 记忆化。
 */
export function useZeroRequestFilter<T extends { request_count: number }>(items: T[] | undefined) {
  const [showZero, setShowZero] = useState(false)
  const visible = useMemo(
    () => (items ? (showZero ? items : items.filter((m) => m.request_count > 0)) : []),
    [items, showZero],
  )
  const hiddenCount = useMemo(
    () => (items ? items.reduce((n, m) => n + (m.request_count > 0 ? 0 : 1), 0) : 0),
    [items],
  )
  return { showZero, setShowZero, visible, hiddenCount }
}

/** 配合 useZeroRequestFilter，渲染在 ChartCard 的 extra 槽；无 0 请求模型时不渲染。 */
export function ZeroRequestToggle({
  showZero,
  onToggle,
  hiddenCount,
}: {
  showZero: boolean
  onToggle: (v: boolean) => void
  hiddenCount: number
}) {
  if (hiddenCount === 0 && !showZero) return null
  return (
    <label className="inline-flex items-center gap-1.5 text-xs text-gray-500 dark:text-gray-400 cursor-pointer select-none whitespace-nowrap">
      <input
        type="checkbox"
        checked={showZero}
        onChange={(e) => onToggle(e.target.checked)}
        className="accent-apple-blue"
      />
      {showZero ? '含 0 请求模型' : `显示 0 请求模型${hiddenCount ? ` (${hiddenCount})` : ''}`}
    </label>
  )
}

// 注：原 PlatformTabs（平台三子页切换 tab）已移除——平台页归到设置下后，
// 子页切换由 SettingsLayout 的「平台运维」分组导航统一承担。
