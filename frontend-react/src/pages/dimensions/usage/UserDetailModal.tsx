// 用户使用详情弹窗：部门聚合 → 本部门人员 → 点击用户行弹出。
// 对接 /stats/users/:uid/detail + /:uid/trend，参考 PlatformOverview UserDetailModal 融合展示风格。
// 时间范围继承当前 UsageKanban 面板的选择。
import { useMemo } from 'react'
import { Modal } from '@/components/ui/Modal'
import { EChart } from '@/components/charts/EChart'
import { getPalette } from '@/components/charts/chartTheme'
import { useTheme } from '@/hooks/useTheme'
import { useUserNameMap } from '@/hooks/useUserNameMap'
import { fmtCost, formatNumber } from '@/lib/formatters'
import { ChartCard, PIE_COLORS, EmptyHint, multiAreaOption, shortToken, useZeroRequestFilter, ZeroRequestToggle } from '@/pages/platform/platformShared'
import { useUsageUserDetail, useUsageUserTrend } from './usageData'

const PCT = (v: number | null | undefined) => (v == null || !Number.isFinite(v) ? '-' : `${v.toFixed(1)}%`)
const TH = 'px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_NUM = 'px-3 py-2 text-right font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TD = 'px-3 py-2 align-middle text-gray-700 dark:text-gray-200 whitespace-nowrap'
const TD_NUM = 'px-3 py-2 align-middle text-right tabular-nums text-gray-700 dark:text-gray-200 whitespace-nowrap'
const KPI_CARD = 'glass rounded-xl p-3'

function shortDate(s: string): string { return s.length >= 10 ? s.slice(5, 10) : s }
const fmtMs = (v: number | null | undefined) => (v != null ? `${Number(v).toFixed(0)} ms` : '-')

export function UserDetailModal({
  uid,
  username: usernameProp,
  start,
  end,
  onClose,
}: {
  uid: string
  username: string
  start: string
  end: string
  onClose: () => void
}) {
  const { theme } = useTheme()
  const p = useMemo(() => getPalette(theme), [theme])
  const { resolveName } = useUserNameMap()

  const detailQ = useUsageUserDetail(uid, start, end)
  const trendQ = useUsageUserTrend(uid, start, end)

  const { showZero, setShowZero, visible: visibleModels, hiddenCount } = useZeroRequestFilter(detailQ.data?.models)

  const u = detailQ.data?.user_detail
  const depts = detailQ.data?.departments || []
  const trendData = trendQ.data || []
  const displayName = resolveName(uid) || usernameProp || u?.username || uid

  // ---- 汇总区间合计（从 trend 数据累加） ----
  const total = useMemo(() => trendData.reduce((s, r) => ({
    requests: s.requests + (r.total_requests || 0),
    tokens: s.tokens + (r.sum_total_tokens || 0),
    cost: s.cost + (r.estimated_total_cost || 0),
    prompt: s.prompt + (r.sum_prompt_tokens || 0),
    completion: s.completion + (r.sum_completion_tokens || 0),
    cache: s.cache + (r.sum_cache_tokens || 0),
    sessions: s.sessions + (r.unique_task_count || 0),
    errors: s.errors + (r.error_requests || 0),
    avgDuration: s.avgDuration + (r.avg_duration_ms || 0) * (r.total_requests || 0),
    avgTTFT: s.avgTTFT + (r.avg_first_token_duration_ms || 0) * (r.total_requests || 0),
  }), { requests: 0, tokens: 0, cost: 0, prompt: 0, completion: 0, cache: 0, sessions: 0, errors: 0, avgDuration: 0, avgTTFT: 0 }), [trendData])

  // ---- 合并模型偏好（从每日 model_preference JSON 累加） ----
  const mergedModelPref = useMemo(() => {
    const merged: Record<string, number> = {}
    for (const d of trendData) {
      if (!d.model_preference) continue
      try {
        const prefs = JSON.parse(d.model_preference)
        for (const [model, count] of Object.entries(prefs)) {
          merged[model] = (merged[model] || 0) + (count as number)
        }
      } catch { /* ignore */ }
    }
    return Object.keys(merged).length > 0 ? merged : null
  }, [trendData])

  // ---- KPI 卡片数据 ----
  const ukpis = [
    { title: '总请求', value: formatNumber(u?.total_requests ?? total.requests) },
    { title: '总输入 Token', value: shortToken(u?.sum_prompt_tokens ?? total.prompt), full: formatNumber(u?.sum_prompt_tokens ?? total.prompt) },
    { title: '总输出 Token', value: shortToken(u?.sum_completion_tokens ?? total.completion), full: formatNumber(u?.sum_completion_tokens ?? total.completion) },
    { title: '总 Token', value: shortToken(u?.sum_total_tokens ?? total.tokens), full: formatNumber(u?.sum_total_tokens ?? total.tokens) },
    { title: '成功率', value: PCT(u?.success_rate), tone: 'pos' as const },
    { title: '失败率', value: PCT(u?.error_rate), tone: (u?.error_rate ?? 0) > 5 ? 'neg' as const : 'neutral' as const },
    { title: '活跃天数', value: formatNumber(u?.active_days ?? 0) },
    { title: '会话数', value: formatNumber(u?.total_sessions ?? total.sessions) },
    { title: '平均 TTFT', value: total.requests > 0 ? fmtMs(total.avgTTFT / total.requests) : (u?.avg_ttft_ms ? fmtMs(u.avg_ttft_ms) : '-') },
    { title: '平均时延', value: total.requests > 0 ? fmtMs(total.avgDuration / total.requests) : (u?.avg_duration_ms ? fmtMs(u.avg_duration_ms) : '-') },
    { title: '预估花费', value: u?.estimated_total_cost ? fmtCost(u.estimated_total_cost) : total.cost ? `¥${total.cost.toFixed(2)}` : '-' },
    { title: '缓存 Token', value: shortToken(u?.sum_cache_tokens ?? total.cache), full: formatNumber(u?.sum_cache_tokens ?? total.cache) },
  ]

  // ---- 模型偏好饼图 ----
  const modelPrefPieOpt = useMemo(() => {
    if (!mergedModelPref) return null
    const entries = Object.entries(mergedModelPref).sort((a, b) => b[1] - a[1])
    return {
      tooltip: { trigger: 'item' as const },
      legend: { bottom: 0, textStyle: { fontSize: 10 } },
      series: [{
        type: 'pie' as const, radius: ['45%', '72%'], center: ['50%', '45%'],
        label: { formatter: '{b}\n{d}%', fontSize: 10 },
        data: entries.map(([name, value], i) => ({ name, value, itemStyle: { color: PIE_COLORS[i % PIE_COLORS.length] } })),
      }],
    }
  }, [mergedModelPref])

  // ---- 趋势图 ----
  const labels = useMemo(() => trendData.map((t) => t.date), [trendData])

  const reqTrendOpt = useMemo(
    () => multiAreaOption(p, labels, [{ name: '请求量', color: '#ff9500', data: trendData.map((t) => t.total_requests ?? 0) }], { yFmt: (v: number) => shortToken(v) }),
    [p, labels, trendData],
  )
  const tokenTrendOpt = useMemo(
    () => multiAreaOption(p, labels, [
      { name: '输入 Token', color: '#0071e3', data: trendData.map((t) => t.sum_prompt_tokens ?? 0) },
      { name: '输出 Token', color: '#af52de', data: trendData.map((t) => t.sum_completion_tokens ?? 0) },
    ], { yFmt: (v: number) => shortToken(v) }),
    [p, labels, trendData],
  )

  // ---- 成本趋势图 ----
  const costTrendOpt = useMemo(() => multiAreaOption(p, labels, [
    { name: '总成本', color: '#ff3b30', data: trendData.map((t) => +(t.estimated_total_cost || 0).toFixed(2)) },
    { name: '输入成本', color: '#0071e3', data: trendData.map((t) => +(t.estimated_input_cost || 0).toFixed(2)) },
    { name: '输出成本', color: '#34c759', data: trendData.map((t) => +(t.estimated_output_cost || 0).toFixed(2)) },
  ], { yFmt: (v: number) => `¥${shortToken(v)}` }), [p, labels, trendData])

  const isLoading = detailQ.isLoading || trendQ.isLoading
  const hasError = detailQ.error || trendQ.error

  return (
    <Modal
      open={true}
      title={`${displayName} · 个人使用详情`}
      maxWidth={1100}
      onClose={onClose}
      footer={<button type="button" className="glass rounded-lg px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-white/10 cursor-pointer border-none" onClick={onClose}>关闭</button>}
    >
      <div className="space-y-4 max-h-[70vh] overflow-y-auto pr-1">
        {/* 子标题 */}
        <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-gray-500 dark:text-gray-400">
          {u?.universal_id && <span>用户 ID: <code className="text-gray-700 dark:text-gray-200 font-mono">{u.universal_id}</code></span>}
          {u?.username && <span>用户名: <span className="text-gray-700 dark:text-gray-200">{u.username}</span></span>}
          <span>{start} ~ {end}</span>
        </div>

        {isLoading ? (
          <div className="py-12 text-center text-sm text-gray-400">加载中...</div>
        ) : hasError ? (
          <div className="py-12 text-center text-sm text-rose-600 dark:text-rose-400">
            加载失败：{(hasError as Error).message}
          </div>
        ) : !u && trendData.length === 0 ? (
          <div className="py-12 text-center text-sm text-gray-400">暂无数据</div>
        ) : (
          <>
            {/* KPI 卡片 3 行 x 4 */}
            <div className="grid grid-cols-4 gap-3">
              {ukpis.slice(0, 4).map((k) => (
                <div key={k.title} className={KPI_CARD}>
                  <div className="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase">{k.title}</div>
                  <div className={`text-xl font-bold tabular-nums text-gray-900 dark:text-white ${k.tone === 'pos' ? 'text-emerald-600 dark:text-emerald-400' : k.tone === 'neg' ? 'text-rose-600 dark:text-rose-400' : ''}`} title={k.full}>{k.value}</div>
                </div>
              ))}
            </div>
            <div className="grid grid-cols-4 gap-3">
              {ukpis.slice(4, 8).map((k) => (
                <div key={k.title} className={KPI_CARD}>
                  <div className="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase">{k.title}</div>
                  <div className={`text-xl font-bold tabular-nums text-gray-900 dark:text-white ${k.tone === 'pos' ? 'text-emerald-600 dark:text-emerald-400' : k.tone === 'neg' ? 'text-rose-600 dark:text-rose-400' : ''}`} title={k.full}>{k.value}</div>
                </div>
              ))}
            </div>
            <div className="grid grid-cols-4 gap-3">
              {ukpis.slice(8, 12).map((k) => (
                <div key={k.title} className={KPI_CARD}>
                  <div className="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase">{k.title}</div>
                  <div className={`text-xl font-bold tabular-nums text-gray-900 dark:text-white ${k.tone === 'pos' ? 'text-emerald-600 dark:text-emerald-400' : k.tone === 'neg' ? 'text-rose-600 dark:text-rose-400' : ''}`} title={k.full}>{k.value}</div>
                </div>
              ))}
            </div>

            {/* 部门归属 */}
            {depts.length > 0 && (
              <div className="flex flex-wrap items-center gap-2 text-xs">
                <span className="text-gray-400 dark:text-gray-500">部门归属：</span>
                {depts.map((dp) => (
                  <span
                    key={dp.dept_id}
                    className={`px-2 py-0.5 rounded-full ${dp.is_main ? 'bg-apple-blue/15 text-apple-blue' : 'bg-gray-100 dark:bg-white/10 text-gray-500 dark:text-gray-400'}`}
                    title={dp.is_main ? '主部门' : '兼职部门'}
                  >
                    {dp.dept_name}
                    {dp.is_main ? ' · 主' : ''}
                  </span>
                ))}
              </div>
            )}

            {/* 模型偏好 + 各模型使用 */}
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
              {modelPrefPieOpt && (
                <ChartCard title="模型偏好（使用次数）">
                  <EChart option={modelPrefPieOpt} height={280} />
                </ChartCard>
              )}
              <ChartCard
                title="各模型使用"
                sub="按实际命中模型拆分"
                extra={<ZeroRequestToggle showZero={showZero} onToggle={setShowZero} hiddenCount={hiddenCount} />}
              >
                {visibleModels.length ? (
                  <div className="overflow-x-auto max-h-[280px] overflow-y-auto">
                    <table className="w-full text-sm border-collapse">
                      <thead className="sticky top-0 bg-white/70 dark:bg-gray-900/70 backdrop-blur">
                        <tr className="border-b border-gray-200/50 dark:border-white/10 text-gray-500 dark:text-gray-400">
                          <th className="px-3 py-2 text-left whitespace-nowrap">模型</th>
                          <th className={TH_NUM}>请求</th>
                          <th className={TH_NUM}>占比</th>
                          <th className={TH_NUM}>输入</th>
                          <th className={TH_NUM}>输出</th>
                          <th className={TH_NUM}>成功率</th>
                        </tr>
                      </thead>
                      <tbody>
                        {visibleModels.map((m, i) => (
                          <tr key={m.model || i} className="border-b border-gray-100/50 dark:border-white/5">
                            <td className="px-3 py-2 text-gray-700 dark:text-gray-200">
                              <span className="inline-flex items-center gap-2">
                                <span className="w-2.5 h-2.5 rounded-full" style={{ background: PIE_COLORS[i % PIE_COLORS.length] }} />
                                <span className="truncate max-w-[140px]" title={m.model}>{m.model || '-'}</span>
                              </span>
                            </td>
                            <td className={TD_NUM}>{formatNumber(m.request_count)}</td>
                            <td className={TD_NUM}>{PCT(m.request_pct)}</td>
                            <td className={TD_NUM} title={formatNumber(m.prompt_tokens)}>{shortToken(m.prompt_tokens)}</td>
                            <td className={TD_NUM} title={formatNumber(m.completion_tokens)}>{shortToken(m.completion_tokens)}</td>
                            <td className={TD_NUM}>{PCT(m.success_rate)}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                ) : (
                  <EmptyHint compact />
                )}
              </ChartCard>
            </div>

            {/* 趋势图：请求+Token + 成本 */}
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
              <ChartCard title="请求量 & Token 趋势">
                <EChart option={reqTrendOpt} height={260} />
              </ChartCard>
              <ChartCard title="Token 消耗趋势（按天）">
                <EChart option={tokenTrendOpt} height={260} />
              </ChartCard>
              <ChartCard title="成本变化趋势">
                <EChart option={costTrendOpt} height={260} />
              </ChartCard>
            </div>

            {/* 每日明细表 */}
            {trendData.length > 0 && (
              <div className="overflow-x-auto max-h-[360px] overflow-y-auto">
                <table className="w-full text-sm border-collapse">
                  <thead className="sticky top-0 bg-white/70 dark:bg-gray-900/70 backdrop-blur">
                    <tr className="border-b border-gray-200/50 dark:border-white/10">
                      <th className={TH}>日期</th>
                      <th className={TH_NUM}>请求</th>
                      <th className={TH_NUM}>输入Token</th>
                      <th className={TH_NUM}>输出Token</th>
                      <th className={TH_NUM}>缓存Token</th>
                      <th className={TH_NUM}>成本</th>
                      <th className={TH_NUM}>会话</th>
                      <th className={TH_NUM}>TTFT</th>
                      <th className={TH_NUM}>时延</th>
                    </tr>
                  </thead>
                  <tbody>
                    {trendData.map((d, i) => (
                      <tr key={d.date || i} className="border-b border-gray-100/50 dark:border-white/5">
                        <td className={TD}>{shortDate(d.date)}</td>
                        <td className={TD_NUM}>{formatNumber(d.total_requests)}</td>
                        <td className={TD_NUM} title={formatNumber(d.sum_prompt_tokens)}>{shortToken(d.sum_prompt_tokens)}</td>
                        <td className={TD_NUM} title={formatNumber(d.sum_completion_tokens)}>{shortToken(d.sum_completion_tokens)}</td>
                        <td className={TD_NUM} title={formatNumber(d.sum_cache_tokens)}>{shortToken(d.sum_cache_tokens)}</td>
                        <td className={TD_NUM}>¥{Number(d.estimated_total_cost || 0).toFixed(2)}</td>
                        <td className={TD_NUM}>{formatNumber(d.unique_task_count)}</td>
                        <td className={TD_NUM}>{fmtMs(d.avg_first_token_duration_ms)}</td>
                        <td className={TD_NUM}>{fmtMs(d.avg_duration_ms)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </>
        )}
      </div>
    </Modal>
  )
}
