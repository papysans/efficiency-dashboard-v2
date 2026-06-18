// 平台运维三级页：AI 服务健康度（成功率 / 错误率 / 时延，平台客观采集）。
// 由原「质量」维度迁来——质量维度已从主体×维度矩阵移除（口径上 AI 服务健康度非代码质量，
// 真代码质量暂无数据源）。这块有真实数据、对运维有用，故归到「设置 › 平台运维」下保留。
// 仅用户级口径：聚合健康度 KPI + 错误率周趋势 + 用户健康度排行（点行下钻看板用户详情）。
// 部门级 rollup 视图未随迁（依赖 dept-tree，且 chat 离线时不可用），需要时再补。
// 时间范围走全局 viewState（AppShell 顶部统一时间选择器，设置页同样生效）。
import { useMemo } from 'react'
import { useNavigate } from 'react-router'
import { useViewState } from '@/store/viewState'
import { useUserNameMap } from '@/hooks/useUserNameMap'
import { MetricCard } from '@/components/ui/MetricCard'
import { formatNumber } from '@/lib/formatters'
import { ChartCard, ChatUserCell, EmptyHint } from '@/pages/platform/platformShared'
import SettingsLayout from '@/pages/settings/SettingsLayout'
import { PlatformNotConnected, PlatformWeekTrend, TruncationNote } from '@/pages/dimensions/platformDimShared'
import {
  useUserRanking,
  useUserWeekSeries,
  errorRateOf,
  rowErrorRate,
  type ChatUserRankingRow,
} from '@/pages/dimensions/platformUserData'

const TH = 'px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_NUM = 'px-3 py-2 text-right font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TD = 'px-3 py-2 align-middle text-gray-700 dark:text-gray-200 whitespace-nowrap'
const TD_NUM = 'px-3 py-2 align-middle text-right tabular-nums text-gray-700 dark:text-gray-200'

const fmtMs = (v: number | null | undefined) => (v != null ? `${Number(v).toFixed(0)} ms` : '-')
const fmtPct = (v: number | null | undefined) => (v != null ? `${(v * 100).toFixed(2)}%` : '-')

/** 口径提示条：明确当前 = AI 服务健康度（平台客观采集），区别于代码质量。 */
function CaliberNotice() {
  return (
    <div
      className="glass rounded-xl px-4 py-3 flex items-start gap-2 text-sm border-l-4"
      style={{ borderLeftColor: '#ff9500' }}
      role="note"
    >
      <svg className="w-5 h-5 shrink-0 text-amber-500" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
      </svg>
      <span className="text-gray-600 dark:text-gray-300">
        本页 = <b className="text-gray-900 dark:text-white">AI 服务健康度</b>（成功率 / 错误率 / 时延，平台侧客观采集），
        反映模型服务可用性，<b>不是代码质量</b>。
      </span>
    </div>
  )
}

export default function PlatformHealth() {
  // SettingsLayout 在 chat_stats_enabled=false 时只渲染 ChatDisabledNotice，body 的 hook 不会执行。
  return (
    <SettingsLayout>
      <HealthBody />
    </SettingsLayout>
  )
}

function HealthBody() {
  const { timeRange } = useViewState()
  const [start, end] = timeRange
  const { resolveName } = useUserNameMap()
  const navigate = useNavigate()
  // 排行行下钻 → 看板用户详情（universal_id 与看板 user_id 同源）。
  const goToUser = (universalId: string) => navigate(`/user/${encodeURIComponent(universalId)}`)

  const series = useUserWeekSeries({ startDate: start, endDate: end }, true)
  // 健康度排行：按错误率倒序（最不健康在前，便于盯问题用户）。
  const rankQ = useUserRanking({ startDate: start, endDate: end, sortBy: 'error_rate', pageSize: 50 }, true)

  // 趋势：错误率周序列（百分比），整窗 errorRate（统一口径 error/(success+error)）。
  const trendSeries = useMemo(() => {
    const values = series.windows.map((w) => +(((series.aggByKey.get(w.key)?.errorRate ?? 0) * 100).toFixed(2)))
    return [{ name: '错误率', color: '#ff3b30', values }]
  }, [series.windows, series.aggByKey])

  if (rankQ.error) {
    return (
      <div className="flex flex-col gap-5">
        <CaliberNotice />
        <PlatformNotConnected reason="error" detail={(rankQ.error as Error)?.message} />
      </div>
    )
  }

  const rows = rankQ.data?.data ?? []

  return (
    <div className="flex flex-col gap-5">
      <CaliberNotice />

      <PlatformWeekTrend
        title="错误率趋势（AI 服务健康度）"
        subtitle="全部用户 · 按周"
        windows={series.windows}
        series={trendSeries}
        loading={series.loading}
        error={series.error}
        hasAny={series.hasAny}
        yFmt={(v) => `${v}%`}
      />

      <AggregateHealthKpis rows={rows} />
      {/* 错误率周序列按 Top 500 拉窗求和，区间真实人数更大时趋势被截断 → 醒目标注。 */}
      {series.truncated && <TruncationNote total={series.maxWindowTotal} />}
      <ChartCard title="用户健康度排行（AI 服务）" sub="区间聚合 · Top 50 · 按错误率倒序 · 点行下钻">
        <HealthRankingTable rows={rows} loading={rankQ.isFetching} resolveName={resolveName} onRowClick={goToUser} />
      </ChartCard>
    </div>
  )
}

function AggregateHealthKpis({ rows }: { rows: ChatUserRankingRow[] }) {
  const sum = (fn: (r: ChatUserRankingRow) => number) => rows.reduce((s, r) => s + (fn(r) || 0), 0)
  const total = sum((r) => r.total_requests)
  const success = sum((r) => r.success_requests)
  const errors = sum((r) => r.error_requests)
  // 统一口径：error/(success+error)（不含 total，对 total 含不含错误都稳健）。
  const errorRate = errorRateOf(success, errors)
  const successRate = errorRate != null ? 1 - errorRate : null
  // 平均时延：按请求数加权（更接近真实体验）。
  const weightedDur = sum((r) => (r.avg_duration_ms || 0) * (r.total_requests || 0))
  const avgDur = total > 0 ? weightedDur / total : null
  return (
    <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
      <MetricCard label="成功率" value={fmtPct(successRate)} tone="pos" hint="1 − 错误率" />
      <MetricCard label="错误率" value={fmtPct(errorRate)} tone={(errorRate ?? 0) > 0.05 ? 'neg' : 'neutral'} hint={`错误请求 ${formatNumber(errors)}`} />
      <MetricCard label="平均时延" value={fmtMs(avgDur)} hint="按请求数加权" />
      <MetricCard label="总请求（Top50）" value={formatNumber(total)} />
    </div>
  )
}

function HealthRankingTable({
  rows,
  loading,
  resolveName,
  onRowClick,
}: {
  rows: ChatUserRankingRow[]
  loading: boolean
  resolveName: (id?: string) => string
  onRowClick: (universalId: string) => void
}) {
  return (
    <div className="overflow-x-auto max-h-[520px] overflow-y-auto">
      <table className="w-full text-sm border-collapse">
        <thead className="sticky top-0 bg-white/70 dark:bg-gray-900/70 backdrop-blur">
          <tr className="border-b border-gray-200/50 dark:border-white/10">
            <th className={TH_NUM}>排名</th>
            <th className={TH}>用户</th>
            <th className={TH_NUM}>请求数</th>
            <th className={TH_NUM}>错误率</th>
            <th className={TH_NUM}>错误请求</th>
            <th className={TH_NUM}>平均时延</th>
          </tr>
        </thead>
        <tbody>
          {loading && rows.length === 0 ? (
            <tr>
              <td colSpan={6} className="py-10 text-center text-sm text-gray-400 dark:text-gray-500">
                加载中…
              </td>
            </tr>
          ) : rows.length === 0 ? (
            <tr>
              <td colSpan={6}>
                <EmptyHint compact />
              </td>
            </tr>
          ) : (
            rows.map((u, i) => {
              // 统一口径：error/(success+error)（与聚合/周序列同一公式），不用后端 u.error_rate。
              const errorRate = rowErrorRate(u)
              const uid = u.universal_id || ''
              return (
                <tr
                  key={uid || i}
                  onClick={uid ? () => onRowClick(uid) : undefined}
                  className={`border-b border-gray-100/50 dark:border-white/5 ${uid ? 'cursor-pointer hover:bg-apple-blue/5 dark:hover:bg-white/5 transition-colors' : ''}`}
                >
                  <td className={TD_NUM}>{i + 1}</td>
                  <td className={TD}>
                    <div className="max-w-[200px] truncate">
                      <ChatUserCell universalId={u.universal_id} chatUsername={u.username} resolveName={resolveName} />
                    </div>
                  </td>
                  <td className={TD_NUM}>{formatNumber(u.total_requests)}</td>
                  <td className={`${TD_NUM} ${(errorRate ?? 0) > 0.05 ? 'text-rose-600 dark:text-rose-400' : ''}`}>
                    {fmtPct(errorRate)}
                  </td>
                  <td className={TD_NUM}>{formatNumber(u.error_requests)}</td>
                  <td className={TD_NUM}>{fmtMs(u.avg_duration_ms)}</td>
                </tr>
              )
            })
          )}
        </tbody>
      </table>
    </div>
  )
}
