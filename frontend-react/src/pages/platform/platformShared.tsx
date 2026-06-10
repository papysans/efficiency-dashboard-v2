// 平台（chat-indicator-statistics 代理）三个子页的共享小件：子页 tab、token 缩写、错误码判断、
// 饼图色板、universal_id → 看板用户互链单元格、ECharts option 工厂与图表卡片。
// 仅平台页内部复用，不进 src/api/ 或全局组件（避免与并行任务冲突）。
import type { ReactNode } from 'react'
import { Link, NavLink } from 'react-router'
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

/** 折线+渐变面积图（分钟/按日趋势通用）。yFmt 控制 y 轴刻度格式（token 缩写 / 百分比 / 金额）。 */
export function multiAreaOption(
  p: ChartPalette,
  times: string[],
  series: AreaSeries[],
  opts: { yFmt?: (v: number) => string; yMax?: number } = {},
): EChartsOption {
  return {
    animation: true,
    grid: { left: 8, right: 16, top: series.length > 1 ? 36 : 24, bottom: 8, containLabel: true },
    tooltip: { trigger: 'axis', ...baseTooltip(p) },
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
  title: string
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

const TABS = [
  { to: '/platform/overview', label: '平台总览', end: true },
  { to: '/platform/realtime', label: '实时态势', end: true },
  { to: '/platform/realtime/query', label: '明细查询', end: false },
]

/** 平台总览 / 实时态势 / 明细查询 三子页切换 tab（玻璃药丸样式）。 */
export function PlatformTabs() {
  return (
    <nav className="glass rounded-xl p-1 inline-flex items-center gap-1" aria-label="平台子页切换">
      {TABS.map((t) => (
        <NavLink
          key={t.to}
          to={t.to}
          end={t.end}
          className={({ isActive }) =>
            `px-4 py-1.5 rounded-lg text-sm font-medium no-underline transition-colors ${
              isActive
                ? 'bg-apple-blue text-white'
                : 'text-gray-600 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white hover:bg-white/50 dark:hover:bg-white/10'
            }`
          }
        >
          {t.label}
        </NavLink>
      ))}
    </nav>
  )
}
