// 个人详情视角：对接 /stats/users/:uid/detail + /:uid/trend。
// 覆盖个人口径：总输入/输出 Token、总请求次数、成功率/失败率、各模型用量与占比（需求 2/3 个人视角）。
import { useMemo } from 'react'
import { ChartCard, EmptyHint, PIE_COLORS, multiAreaOption, shortToken, useZeroRequestFilter, ZeroRequestToggle } from '@/pages/platform/platformShared'
import { MetricCard } from '@/components/ui/MetricCard'
import { EChart } from '@/components/charts/EChart'
import { useTheme } from '@/hooks/useTheme'
import { getPalette } from '@/components/charts/chartTheme'
import { useUserNameMap } from '@/hooks/useUserNameMap'
import { fmtCost, formatNumber } from '@/lib/formatters'
import { useUsageUserDetail, useUsageUserTrend } from './usageData'

const PCT = (v: number | null | undefined) => (v == null || !Number.isFinite(v) ? '-' : `${v.toFixed(1)}%`)
const TD_NUM = 'px-3 py-2 text-right align-middle tabular-nums text-gray-700 dark:text-gray-200 whitespace-nowrap'

export function MemberDetail({ uid, start, end }: { uid: string; start: string; end: string }) {
  const { theme } = useTheme()
  const p = getPalette(theme)
  const { resolveName } = useUserNameMap()
  const detailQ = useUsageUserDetail(uid, start, end)
  const trendQ = useUsageUserTrend(uid, start, end)
  // 各模型用量：默认隐藏 0 请求模型，开关可展开（hook 必须在所有早返回之前调用）。
  const { showZero, setShowZero, visible: visibleModels, hiddenCount } = useZeroRequestFilter(detailQ.data?.models)

  const labels = useMemo(() => (trendQ.data || []).map((t) => t.date), [trendQ.data])
  const reqOpt = useMemo(
    () =>
      multiAreaOption(
        p,
        labels,
        [{ name: '请求量', color: '#ff9500', data: (trendQ.data || []).map((t) => t.total_requests ?? 0) }],
        { yFmt: (v) => shortToken(v) },
      ),
    [p, labels, trendQ.data],
  )
  const tokenOpt = useMemo(
    () =>
      multiAreaOption(
        p,
        labels,
        [
          { name: '输入 Token', color: '#0071e3', data: (trendQ.data || []).map((t) => t.sum_prompt_tokens ?? 0) },
          { name: '输出 Token', color: '#af52de', data: (trendQ.data || []).map((t) => t.sum_completion_tokens ?? 0) },
        ],
        { yFmt: (v) => shortToken(v) },
      ),
    [p, labels, trendQ.data],
  )

  if (!uid) {
    return <div className="glass rounded-2xl p-10 text-center text-sm text-gray-400 dark:text-gray-500">未选择成员</div>
  }

  if (detailQ.error) {
    return (
      <div className="glass rounded-2xl p-10 text-center text-sm text-rose-600 dark:text-rose-400">
        加载个人详情失败：{(detailQ.error as Error).message}
      </div>
    )
  }

  const d = detailQ.data
  const u = d?.user_detail
  const depts = d?.departments || []
  const displayName = resolveName(uid) || u?.username || uid

  if (detailQ.isLoading || !u) {
    return (
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        {Array.from({ length: 8 }).map((_, i) => (
          <div key={i} className="skeleton h-24 rounded-2xl" />
        ))}
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-4">
      {/* 个人 KPI */}
      <ChartCard title={`${displayName} · 个人使用`} sub={`${start} ~ ${end}`}>
        <div className="mb-3 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-gray-500 dark:text-gray-400">
          {u.universal_id && <span>用户 ID: <code className="text-gray-700 dark:text-gray-200 font-mono">{u.universal_id}</code></span>}
          {u.username && <span>用户名: <span className="text-gray-700 dark:text-gray-200">{u.username}</span></span>}
        </div>
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
          <MetricCard label="总请求" value={formatNumber(u.total_requests)} />
          <MetricCard label="成功率" value={PCT(u.success_rate)} tone="pos" />
          <MetricCard label="失败率" value={PCT(u.error_rate)} tone={u.error_rate > 5 ? 'neg' : 'neutral'} />
          <MetricCard label="总会话数" value={formatNumber(u.total_sessions)} />
          <MetricCard label="总输入 Token" value={shortToken(u.sum_prompt_tokens)} hint={formatNumber(u.sum_prompt_tokens)} />
          <MetricCard label="总输出 Token" value={shortToken(u.sum_completion_tokens)} hint={formatNumber(u.sum_completion_tokens)} />
          <MetricCard label="总 Token" value={shortToken(u.sum_total_tokens)} hint={formatNumber(u.sum_total_tokens)} accent={p.brand} />
          <MetricCard label="活跃天数" value={formatNumber(u.active_days)} />
          <MetricCard label="平均耗时" value={u.avg_duration_ms ? `${(u.avg_duration_ms / 1000).toFixed(1)}s` : '-'} />
          <MetricCard label="预估花费" value={u.estimated_total_cost ? fmtCost(u.estimated_total_cost) : '-'} />
        </div>
        {depts.length > 0 && (
          <div className="mt-3 flex flex-wrap items-center gap-2 text-xs">
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
      </ChartCard>

      {/* 各模型用量（个人） */}
      <ChartCard
        title="各模型使用"
        sub="按实际命中模型拆分"
        extra={<ZeroRequestToggle showZero={showZero} onToggle={setShowZero} hiddenCount={hiddenCount} />}
      >
        {visibleModels.length ? (
          <div className="overflow-x-auto">
            <table className="w-full text-sm border-collapse">
              <thead>
                <tr className="border-b border-gray-200/50 dark:border-white/10 text-gray-500 dark:text-gray-400">
                  <th className="px-3 py-2 text-left whitespace-nowrap">模型</th>
                  <th className={TD_NUM.replace('text-right align-middle', 'text-right font-semibold')}>请求次数</th>
                  <th className="px-3 py-2 text-right font-semibold whitespace-nowrap">请求占比</th>
                  <th className="px-3 py-2 text-right font-semibold whitespace-nowrap">输入 Token</th>
                  <th className="px-3 py-2 text-right font-semibold whitespace-nowrap">输出 Token</th>
                  <th className="px-3 py-2 text-right font-semibold whitespace-nowrap">消耗占比</th>
                  <th className="px-3 py-2 text-right font-semibold whitespace-nowrap">成功率</th>
                </tr>
              </thead>
              <tbody>
                {visibleModels.map((m, i) => (
                  <tr key={m.model || i} className="border-b border-gray-100/50 dark:border-white/5">
                    <td className="px-3 py-2 text-gray-700 dark:text-gray-200">
                      <span className="inline-flex items-center gap-2">
                        <span className="w-2.5 h-2.5 rounded-full" style={{ background: PIE_COLORS[i % PIE_COLORS.length] }} />
                        <span className="truncate max-w-[180px]" title={m.model}>{m.model || '-'}</span>
                      </span>
                    </td>
                    <td className={TD_NUM}>{formatNumber(m.request_count)}</td>
                    <td className={TD_NUM}>{PCT(m.request_pct)}</td>
                    <td className={TD_NUM} title={formatNumber(m.prompt_tokens)}>{shortToken(m.prompt_tokens)}</td>
                    <td className={TD_NUM} title={formatNumber(m.completion_tokens)}>{shortToken(m.completion_tokens)}</td>
                    <td className={TD_NUM}>{PCT(m.token_pct)}</td>
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

      {/* 按天趋势 */}
      {trendQ.isLoading ? (
        <div className="h-[240px] skeleton rounded-2xl" />
      ) : trendQ.data && trendQ.data.length ? (
        <>
          <ChartCard title="请求量趋势（按天）">
            <EChart option={reqOpt} height={240} />
          </ChartCard>
          <ChartCard title="Token 消耗趋势（按天）">
            <EChart option={tokenOpt} height={240} />
          </ChartCard>
        </>
      ) : (
        <ChartCard title="使用趋势（按天）">
          <EmptyHint />
        </ChartCard>
      )}
    </div>
  )
}
