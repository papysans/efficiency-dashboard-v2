// 平台·实时态势 —— chat-indicator-statistics /stats/realtime 的玻璃拟态重写（design §2.2）。
// 服务端 10 秒全局限频：取数完成（成功或失败）后，刷新/换档按钮统一 10s 倒计时禁用，
// 避免必然 400；若仍撞限频（多人同时刷），错误条直接展示 chat 侧友好文案「请 N 秒后再试」。
// datasource_id 不传 = 服务端自动取第一个启用的数据源（内网单源，省一次 /datasources 请求）。
import { useEffect, useMemo, useState } from 'react'
import * as echarts from 'echarts'
import type { EChartsOption } from 'echarts'
import { useChatRealtime, useGlobalConfig } from '@/api/queries'
import { useTheme } from '@/hooks/useTheme'
import { useUserNameMap } from '@/hooks/useUserNameMap'
import { EChart } from '@/components/charts/EChart'
import { getPalette, type ChartPalette } from '@/components/charts/chartTheme'
import { formatNumber } from '@/lib/formatters'
import { ChatDisabledNotice } from '@/pages/settings/SettingsLayout'
import { ChatUserCell, PIE_COLORS, PlatformTabs, shortToken } from './platformShared'

type Range = '30m' | '1h' | '3h'

const RANGE_OPTIONS: Array<{ value: Range; label: string }> = [
  { value: '30m', label: '近 30 分钟' },
  { value: '1h', label: '近 1 小时' },
  { value: '3h', label: '近 3 小时' },
]

/** chat 侧 /stats/realtime 全局限频窗口（10 秒一次）。 */
const COOLDOWN_MS = 10_000

// ---- option 工厂（纯函数，useMemo 按 data/theme 重建） ----

function rgba(hex: string, alpha: number): string {
  const r = parseInt(hex.slice(1, 3), 16)
  const g = parseInt(hex.slice(3, 5), 16)
  const b = parseInt(hex.slice(5, 7), 16)
  return `rgba(${r},${g},${b},${alpha})`
}

function baseTooltip(p: ChartPalette) {
  return {
    backgroundColor: p.tooltipBg,
    borderColor: p.tooltipBorder,
    borderWidth: 1,
    textStyle: { color: p.tooltipText },
  }
}

interface AreaSeries {
  name: string
  color: string
  data: number[]
}

/** 折线+渐变面积图（分钟趋势通用）。yFmt 控制 y 轴刻度格式（token 缩写 / 百分比）。 */
function multiAreaOption(
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

/** 环形分布图（模型分布 / Auto 路由细分）。 */
function donutOption(p: ChartPalette, items: Array<{ name: string; value: number }>): EChartsOption {
  return {
    animation: true,
    color: PIE_COLORS,
    tooltip: { trigger: 'item', ...baseTooltip(p) },
    series: [
      {
        type: 'pie',
        radius: ['42%', '68%'],
        center: ['50%', '52%'],
        data: items,
        label: { color: p.textColor, formatter: '{b} {d}%' },
        labelLine: { lineStyle: { color: p.axisColor } },
        itemStyle: { borderWidth: 0 },
      },
    ],
  }
}

// ---- 样式常量（对齐 NeedList 表格类） ----

const TH = 'px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_NUM = 'px-3 py-2 text-right font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TD = 'px-3 py-2 align-middle text-gray-700 dark:text-gray-200'
const TD_NUM = 'px-3 py-2 align-middle text-right tabular-nums text-gray-700 dark:text-gray-200'

export default function RealtimeReport() {
  const { theme } = useTheme()
  const [range, setRange] = useState<Range>('30m')
  // 开关语义与设置区一致：chat_stats_enabled !== true 时 hooks 不发请求、整页显示未启用提示
  // （导航虽已隐藏「平台」组，但直达 URL 仍可进本页）。
  const { data: gc } = useGlobalConfig()
  const chatEnabled = gc?.chat_stats_enabled === true
  const chatDisabled = !!gc && !chatEnabled
  const q = useChatRealtime({ range }, chatEnabled)
  const data = q.data
  // universal_id 与看板 user_id 同源 → Top 用户表解析看板用户名并互链（加载失败自动回退，不阻塞）。
  const { resolveName } = useUserNameMap()

  // 10s 限频倒计时：以最近一次取数完成时刻（成功/失败取较新者）起算。
  const lastFetchedAt = Math.max(q.dataUpdatedAt || 0, q.errorUpdatedAt || 0)
  const [cooldown, setCooldown] = useState(0)
  useEffect(() => {
    if (!lastFetchedAt) return
    const tick = () => setCooldown(Math.max(0, Math.ceil((lastFetchedAt + COOLDOWN_MS - Date.now()) / 1000)))
    tick()
    const id = window.setInterval(tick, 500)
    return () => window.clearInterval(id)
  }, [lastFetchedAt])

  const locked = q.isFetching || cooldown > 0

  const p = useMemo(() => getPalette(theme), [theme])

  const tokenTrendOpt = useMemo(() => {
    const items = data?.token_trend ?? []
    return multiAreaOption(
      p,
      items.map((i) => i.time),
      [
        { name: '输入 Token', color: '#0071e3', data: items.map((i) => i.prompt_tokens) },
        { name: '输出 Token', color: '#34c759', data: items.map((i) => i.completion_tokens) },
        { name: '缓存 Token', color: '#5ac8fa', data: items.map((i) => i.cache_tokens) },
      ],
      { yFmt: (v) => shortToken(v) },
    )
  }, [data, p])

  const cacheRateOpt = useMemo(() => {
    const items = data?.cache_hit_rate ?? []
    return multiAreaOption(
      p,
      items.map((i) => i.time),
      [{ name: '缓存命中率', color: '#34c759', data: items.map((i) => i.rate) }],
      { yFmt: (v) => `${v}%`, yMax: 100 },
    )
  }, [data, p])

  const requestTrendOpt = useMemo(() => {
    const items = data?.request_trend ?? []
    return multiAreaOption(
      p,
      items.map((i) => i.time),
      [{ name: '请求量', color: '#ff9500', data: items.map((i) => i.request_count) }],
    )
  }, [data, p])

  const modelPieOpt = useMemo(
    () => donutOption(p, (data?.model_requests ?? []).map((m) => ({ name: m.model, value: m.request_count }))),
    [data, p],
  )

  const autoRouterOpt = useMemo(
    () =>
      donutOption(
        p,
        (data?.auto_router_breakdown ?? []).map((m) => ({ name: m.routed_model, value: m.request_count })),
      ),
    [data, p],
  )

  const summary = data?.summary
  const errRequests = summary?.total_error_requests ?? 0
  const kpis: Array<{ title: string; value: string; full?: string; alert?: boolean }> = [
    { title: '请求量', value: formatNumber(summary?.total_requests) },
    { title: '活跃用户', value: formatNumber(summary?.total_users) },
    {
      title: '输入 Token',
      value: shortToken(summary?.total_prompt_tokens),
      full: formatNumber(summary?.total_prompt_tokens),
    },
    {
      title: '输出 Token',
      value: shortToken(summary?.total_completion_tokens),
      full: formatNumber(summary?.total_completion_tokens),
    },
    {
      title: '缓存 Token',
      value: shortToken(summary?.total_cache_tokens),
      full: formatNumber(summary?.total_cache_tokens),
    },
    { title: '错误请求', value: formatNumber(summary?.total_error_requests), alert: errRequests > 0 },
    { title: '实时费用', value: summary?.total_cost != null ? `¥${summary.total_cost.toFixed(2)}` : '-' },
  ]

  const updatedAt = q.dataUpdatedAt
    ? new Date(q.dataUpdatedAt).toLocaleTimeString('zh-CN', { hour12: false })
    : ''

  const rangeBtn = (active: boolean) =>
    `px-3 py-1.5 rounded-lg text-sm font-medium cursor-pointer transition-colors disabled:opacity-50 disabled:cursor-not-allowed focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue ${
      active
        ? 'bg-apple-blue text-white'
        : 'glass text-gray-600 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white'
    }`

  const header = (
    <header className="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white">平台实时态势</h1>
        <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
          直查源库的 LLM 请求实时聚合：token / 成本 / 错误 / 模型分布。服务端限频 10 秒一次。
        </p>
      </div>
      <PlatformTabs />
    </header>
  )

  if (chatDisabled) {
    return (
      <div className="space-y-5">
        {header}
        <ChatDisabledNotice />
      </div>
    )
  }

  return (
    <div className="space-y-5">
      {header}

      {/* 工具栏：range 切换 + 手动刷新（限频倒计时） */}
      <div className="flex flex-wrap items-center gap-2">
        {RANGE_OPTIONS.map((o) => (
          <button
            key={o.value}
            type="button"
            onClick={() => setRange(o.value)}
            disabled={locked && o.value !== range}
            title={locked && o.value !== range ? `限频 10 秒，${cooldown} 秒后可切换` : undefined}
            className={rangeBtn(o.value === range)}
            aria-pressed={o.value === range}
          >
            {o.label}
          </button>
        ))}
        <button
          type="button"
          onClick={() => q.refetch()}
          disabled={locked}
          className="bg-apple-blue hover:bg-apple-blue-hover text-white rounded-lg px-4 py-1.5 text-sm font-medium cursor-pointer transition-colors disabled:opacity-50 disabled:cursor-not-allowed focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue"
        >
          {q.isFetching ? '刷新中…' : cooldown > 0 ? `刷新（${cooldown}s）` : '刷新'}
        </button>
        {updatedAt && <span className="text-xs text-gray-400 dark:text-gray-500">更新于 {updatedAt}</span>}
      </div>

      {q.error && (
        <div className="glass rounded-xl px-4 py-3 text-sm text-rose-600 dark:text-rose-400 bg-rose-50/50 dark:bg-rose-900/20">
          {(q.error as Error).message}
        </div>
      )}

      {q.isLoading ? (
        <ReportSkeleton />
      ) : data ? (
        <>
          {/* KPI 卡行 */}
          <div className="grid grid-cols-2 sm:grid-cols-4 xl:grid-cols-7 gap-3">
            {kpis.map((k) => (
              <div key={k.title} className="glass rounded-2xl p-4 hover:shadow-lg transition-shadow">
                <div className="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide mb-1">
                  {k.title}
                </div>
                <div
                  className={`text-2xl font-bold tabular-nums ${
                    k.alert ? 'text-rose-600 dark:text-rose-400' : 'text-gray-900 dark:text-white'
                  }`}
                  title={k.full}
                >
                  {k.value}
                </div>
              </div>
            ))}
          </div>

          {/* 趋势图 */}
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
            <ChartCard title="Token 趋势" sub="每分钟 · 输入 / 输出 / 缓存">
              {(data.token_trend ?? []).length > 0 ? <EChart option={tokenTrendOpt} height={280} /> : <EmptyHint />}
            </ChartCard>
            <ChartCard title="缓存命中率趋势" sub="每分钟 · cache / prompt">
              {(data.cache_hit_rate ?? []).length > 0 ? <EChart option={cacheRateOpt} height={280} /> : <EmptyHint />}
            </ChartCard>
            <ChartCard title="请求量趋势" sub="每分钟">
              {(data.request_trend ?? []).length > 0 ? <EChart option={requestTrendOpt} height={260} /> : <EmptyHint />}
            </ChartCard>
            <ChartCard title="模型请求分布" sub="含 auto">
              {(data.model_requests ?? []).length > 0 ? <EChart option={modelPieOpt} height={260} /> : <EmptyHint />}
            </ChartCard>
          </div>

          {/* Auto 路由细分（无 auto 流量时整卡隐藏，对齐 chat 侧行为） */}
          {(data.auto_router_breakdown ?? []).length > 0 && (
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
              <ChartCard title="Auto 路由细分" sub="auto 请求实际路由到的模型">
                <EChart option={autoRouterOpt} height={260} />
              </ChartCard>
              <ChartCard title="Auto 路由明细">
                <table className="w-full text-sm border-collapse">
                  <thead>
                    <tr className="border-b border-gray-200/50 dark:border-white/10">
                      <th className={TH}>路由模型</th>
                      <th className={TH_NUM}>请求数</th>
                      <th className={TH_NUM}>占比</th>
                    </tr>
                  </thead>
                  <tbody>
                    {data.auto_router_breakdown.map((r) => (
                      <tr key={r.routed_model} className="border-b border-gray-100/50 dark:border-white/5">
                        <td className={TD}>{r.routed_model || '-'}</td>
                        <td className={TD_NUM}>{formatNumber(r.request_count)}</td>
                        <td className={TD_NUM}>{r.percentage.toFixed(1)}%</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </ChartCard>
            </div>
          )}

          {/* 模型详情表 */}
          <ChartCard title="模型详情">
            <div className="overflow-x-auto">
              <table className="w-full text-sm border-collapse">
                <thead>
                  <tr className="border-b border-gray-200/50 dark:border-white/10">
                    <th className={TH}>模型</th>
                    <th className={TH_NUM}>请求数</th>
                    <th className={TH_NUM}>用户数</th>
                    <th className={TH_NUM}>输入 Token</th>
                    <th className={TH_NUM}>输出 Token</th>
                    <th className={TH_NUM}>费用（¥）</th>
                  </tr>
                </thead>
                <tbody>
                  {(data.model_requests ?? []).length === 0 ? (
                    <tr>
                      <td colSpan={6}>
                        <EmptyHint compact />
                      </td>
                    </tr>
                  ) : (
                    data.model_requests.map((m) => (
                      <tr key={m.model} className="border-b border-gray-100/50 dark:border-white/5">
                        <td className={TD}>{m.model || '-'}</td>
                        <td className={TD_NUM}>{formatNumber(m.request_count)}</td>
                        <td className={TD_NUM}>{formatNumber(m.user_count)}</td>
                        <td className={TD_NUM} title={formatNumber(m.prompt_tokens)}>
                          {shortToken(m.prompt_tokens)}
                        </td>
                        <td className={TD_NUM} title={formatNumber(m.completion_tokens)}>
                          {shortToken(m.completion_tokens)}
                        </td>
                        <td className={TD_NUM}>{m.total_cost != null ? m.total_cost.toFixed(2) : '-'}</td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </ChartCard>

          {/* Top 用户表 */}
          <ChartCard title="请求量 Top 50 用户" sub="按请求数倒序">
            <div className="overflow-x-auto max-h-[480px] overflow-y-auto">
              <table className="w-full text-sm border-collapse">
                <thead className="sticky top-0 bg-white/70 dark:bg-gray-900/70 backdrop-blur">
                  <tr className="border-b border-gray-200/50 dark:border-white/10">
                    <th className={TH_NUM}>排名</th>
                    <th className={TH}>Universal ID</th>
                    <th className={TH}>用户名</th>
                    <th className={TH_NUM}>请求数</th>
                    <th className={TH_NUM}>输入 Token</th>
                    <th className={TH_NUM}>输出 Token</th>
                  </tr>
                </thead>
                <tbody>
                  {(data.top_users ?? []).length === 0 ? (
                    <tr>
                      <td colSpan={6}>
                        <EmptyHint compact />
                      </td>
                    </tr>
                  ) : (
                    data.top_users.map((u, i) => (
                      <tr key={u.universal_id || i} className="border-b border-gray-100/50 dark:border-white/5">
                        <td className={TD_NUM}>{i + 1}</td>
                        <td className={`${TD} font-mono text-xs`}>{u.universal_id || '-'}</td>
                        <td className={TD}>
                          <div className="max-w-[180px] truncate">
                            <ChatUserCell
                              universalId={u.universal_id}
                              chatUsername={u.username}
                              resolveName={resolveName}
                            />
                          </div>
                        </td>
                        <td className={TD_NUM}>{formatNumber(u.request_count)}</td>
                        <td className={TD_NUM} title={formatNumber(u.prompt_tokens)}>
                          {shortToken(u.prompt_tokens)}
                        </td>
                        <td className={TD_NUM} title={formatNumber(u.completion_tokens)}>
                          {shortToken(u.completion_tokens)}
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </ChartCard>
        </>
      ) : !q.error ? (
        <div className="glass rounded-2xl p-10 text-center text-sm text-gray-400 dark:text-gray-500">
          暂无数据，点击「刷新」开始查询
        </div>
      ) : null}
    </div>
  )
}

function ChartCard({ title, sub, children }: { title: string; sub?: string; children: React.ReactNode }) {
  return (
    <section className="glass rounded-2xl p-5 hover:shadow-lg transition-shadow">
      <div className="flex items-center justify-between mb-3">
        <h2 className="text-sm font-semibold text-gray-700 dark:text-gray-200">{title}</h2>
        {sub && <span className="text-xs text-gray-400 dark:text-gray-500">{sub}</span>}
      </div>
      {children}
    </section>
  )
}

function EmptyHint({ compact = false }: { compact?: boolean }) {
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

function ReportSkeleton() {
  return (
    <div className="space-y-5">
      <div className="grid grid-cols-2 sm:grid-cols-4 xl:grid-cols-7 gap-3">
        {Array.from({ length: 7 }).map((_, i) => (
          <div key={i} className="skeleton h-20 rounded-2xl" />
        ))}
      </div>
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        {Array.from({ length: 4 }).map((_, i) => (
          <div key={i} className="skeleton h-72 rounded-2xl" />
        ))}
      </div>
    </div>
  )
}
