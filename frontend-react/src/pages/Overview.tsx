import { useMemo, type ReactNode } from 'react'
import { useDashboardSummary, useDashboardTrends } from '@/api/queries'
import { useViewState } from '@/store/viewState'
import { formatDateParam } from '@/lib/date'
import { formatNumber, formatV2Ratio } from '@/lib/formatters'
import { glossaryTip } from '@/lib/glossary'
import { MetricCard } from '@/components/ui/MetricCard'
import { MetricScorecard } from '@/components/executive/MetricScorecard'
import { HeroSaving } from '@/components/executive/HeroSaving'
import { TrendCard } from '@/components/executive/TrendCard'
import { TopRankCard } from '@/components/executive/TopRankCard'
import { DeptPKCard } from '@/components/executive/DeptPKCard'
import { PlatformObjectiveCard } from '@/components/executive/PlatformObjectiveCard'

// 高管提效总览大屏。Bento 12 列网格 + 玻璃拟态 + 卡片 staggered 渐入。
// ① Hero 省人天/AI花费/净节省/提效 ② 平台客观指标(AI 真实花费/请求/Token,平台采集口径独立) ③ 使用/贡献/AI占比 速览(同款卡,等高)
// ④ 周提效趋势 ⑤ 部门PK + Top榜 ⑥ 规模概览。
// ③ 不含成本/质量/效率：成本与 Hero 全平台 AI 花费口径不同易混淆、质量本轮无数据、效率与 Hero 综合提效重复。
// ② 平台块降级护栏：chat_stats_enabled=false / 请求失败 → 轻提示，不空页、不崩、不影响其余区块。
export default function Overview() {
  // 全局时间范围（顶部统一 DateRangePicker），切日期联动总览各卡。
  const { timeRange } = useViewState()
  const [startDate, endDate] = useMemo(
    () => [formatDateParam(timeRange[0]), formatDateParam(timeRange[1])],
    [timeRange],
  )

  return (
    <div className="grid grid-cols-12 gap-4 lg:gap-6">
      {/* Row1 Hero */}
      <Cell index={0} className="col-span-12">
        <HeroSaving startDate={startDate} endDate={endDate} />
      </Cell>

      {/* Row2 平台客观指标（AI 调用真实花费/请求/Token，口径独立于上方看板派生 Hero；未接入/失败时优雅降级） */}
      <Cell index={1} className="col-span-12">
        <PlatformObjectiveCard />
      </Cell>

      {/* Row3 使用/贡献/AI占比 速览（同款卡，三张等高） */}
      <Cell index={2} className="col-span-12">
        <ScorecardStrip startDate={startDate} endDate={endDate} />
      </Cell>

      {/* Row4 AI 渗透率（单独卡：渗透 / 覆盖 / 切散缺口） */}
      <Cell index={3} className="col-span-12">
        <AIPenetrationCard startDate={startDate} endDate={endDate} />
      </Cell>

      {/* Row5 周提效趋势（整宽） */}
      <Cell index={4} className="col-span-12">
        <TrendCard startDate={startDate} endDate={endDate} />
      </Cell>

      {/* Row6 部门 PK + Top 榜（需求 / 人） */}
      <Cell index={5} className="col-span-12 lg:col-span-6">
        <DeptPKCard startDate={startDate} endDate={endDate} />
      </Cell>
      <Cell index={6} className="col-span-12 lg:col-span-6">
        <TopRankCard startDate={startDate} endDate={endDate} />
      </Cell>

      {/* Row7 规模概览 */}
      <Cell index={7} className="col-span-12">
        <CountsCard startDate={startDate} endDate={endDate} />
      </Cell>
    </div>
  )
}

/** staggered 渐入容器：每个卡片延迟 i*80ms（reduce-motion 由 index.css 媒体查询禁用）。 */
function Cell({ index, className = '', children }: { index: number; className?: string; children: ReactNode }) {
  return (
    <div
      className={`${className} animate-[fade-in-up_.5s_ease-out_both]`}
      style={{ animationDelay: `${index * 80}ms` }}
    >
      {children}
    </div>
  )
}

/**
 * 速览卡（同款 MetricScorecard，三张等高）：使用人数 / 贡献净增行 / AI 代码占比。
 * 当期值取 dashboard/summary（组织级全局口径）；使用/贡献的 sparkline+环比取 dashboard/trends 周序列。
 * AI 占比无周序列（周表无 AI 覆盖行数据）→ 不画 sparkline（卡内留高占位，与另两张等高）。
 * 纯展示、不跳转；ⓘ 看口径。
 * 已移除：成本（与 Hero 全平台 AI 花费口径不同，易混淆）、效率（与 Hero 综合日历提效重复）、质量（本轮无数据）。
 */
function ScorecardStrip({ startDate, endDate }: { startDate: string; endDate: string }) {
  const summaryQ = useDashboardSummary({ startDate, endDate })
  const trendsQ = useDashboardTrends({ startDate, endDate })
  const s = summaryQ.data
  const points = trendsQ.data?.points ?? []
  const compare = trendsQ.data?.compare ?? {}
  const loading = summaryQ.isLoading || trendsQ.isLoading

  if (summaryQ.error) {
    return (
      <div className="glass rounded-2xl p-5 text-sm text-rose-600 dark:text-rose-400">
        加载失败：{(summaryQ.error as Error).message}
      </div>
    )
  }

  return (
    <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 lg:gap-4">
      <MetricScorecard
        label="使用人数"
        value={s ? formatNumber(s.total_users_v2) : null}
        hint={s ? `需求 ${formatNumber(s.merged_needs)} 已合并` : undefined}
        tip={glossaryTip('active_users')}
        series={points.map((p) => p.active_users)}
        delta={compare.usage}
        accent="#0071e3"
        loading={loading}
      />
      <MetricScorecard
        label="贡献行数"
        value={s ? `${formatNumber(s.total_commit_lines)}` : null}
        hint={s ? `AI ${formatV2Ratio(s.ai_code_ratio)} · 净增行` : undefined}
        tip={glossaryTip('commit_diff_lines')}
        series={points.map((p) => p.commit_diff_lines)}
        delta={compare.contribution}
        accent="#5e5ce6"
        loading={loading}
      />
      <MetricScorecard
        label="AI 代码占比"
        value={s ? formatV2Ratio(s.ai_code_ratio) : null}
        hint={s ? `可计入 ${formatNumber(s.eligible_needs)}/${formatNumber(s.total_needs)} 需求` : undefined}
        tip={glossaryTip('ai_code_ratio')}
        series={[]}
        accent="#00c7be"
        loading={loading}
      />
    </div>
  )
}

/**
 * 底部规模概览卡：复用 PR0 的总仓库/用户/需求/Commit/代码行 MetricCard 网格。
 * hint 只写"这张卡独有的信息"，口径细节走 ⓘ（glossaryTip），避免六张卡互相复述：
 * - 总 Commit 的 hint 原为「代码行 N」，与相邻「总代码行」是同一个数 → 改为均行/次（派生信息）。
 * - 总用户数(工具口径,tasks 去重) vs 活跃用户V2(需求口径,有可计入需求) 两者都是"人数"，
 *   原 hint「参与提交的贡献者」/「需求口径参与者」既分不出差别、前者也与后端主口径不符。
 */
function CountsCard({ startDate, endDate }: { startDate: string; endDate: string }) {
  const { data, isLoading, error } = useDashboardSummary({ startDate, endDate })
  // 均行/次：总代码行 ÷ 总 commit 数（commit 为 0 时不显示）
  const linesPerCommit = data && data.total_commits > 0 ? data.total_commit_lines / data.total_commits : null

  return (
    <div className="glass rounded-2xl p-5 md:p-6 hover:shadow-lg transition-shadow flex flex-col">
      <h2 className="text-sm font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide mb-4">规模概览</h2>
      {error ? (
        <div className="flex-1 flex items-center justify-center text-sm text-rose-600 dark:text-rose-400 min-h-[14rem]">
          加载失败：{(error as Error).message}
        </div>
      ) : isLoading || !data ? (
        <div className="grid grid-cols-2 sm:grid-cols-3 gap-3 flex-1">
          {Array.from({ length: 6 }).map((_, i) => (
            <div key={i} className="skeleton h-20 rounded-2xl" />
          ))}
        </div>
      ) : (
        <div className="grid grid-cols-2 sm:grid-cols-3 gap-3">
          <MetricCard
            label="总仓库数"
            value={formatNumber(data.total_repos)}
            hint={`分支 ${formatNumber(data.total_branchs)} 个`}
            tip={glossaryTip('repo_scale')}
          />
          <MetricCard
            label="总用户数"
            value={formatNumber(data.total_users)}
            hint="用过 AI 工具的人"
            tip={glossaryTip('tool_users')}
          />
          <MetricCard
            label="需求"
            value={formatNumber(data.total_needs)}
            hint={`已合并 ${formatNumber(data.merged_needs)} · 可计入 ${formatNumber(data.eligible_needs)}`}
            tip={glossaryTip('merged_need')}
          />
          <MetricCard
            label="总 Commit"
            value={formatNumber(data.total_commits)}
            hint={linesPerCommit != null ? `均 ${formatNumber(linesPerCommit, 0)} 行/次` : undefined}
            tip={glossaryTip('commit_count')}
          />
          <MetricCard
            label="总代码行"
            value={formatNumber(data.total_commit_lines)}
            hint="增删相抵后的净行数"
            tip={glossaryTip('commit_diff_lines')}
          />
          <MetricCard
            label="活跃用户(V2)"
            value={formatNumber(data.total_users_v2)}
            hint="有可计入需求的人"
            tip={glossaryTip('active_users')}
          />
        </div>
      )}
    </div>
  )
}

/**
 * AI 渗透率卡（单独）：渗透率(作者实际在用 AI 的需求占比，含被 need 边界切散到别仓库/别分支的) +
 * 数据覆盖率(看板能直接关联到 AI 会话的占比) + 切散缺口(用了 AI 但被切散、未进计算的)。
 * 口径与需求列表折叠一致；完整勘探见 docs/2026-06-16-needs-ai-code-ratio-dash-investigation.md。
 */
function AIPenetrationCard({ startDate, endDate }: { startDate: string; endDate: string }) {
  const { data, isLoading, error } = useDashboardSummary({ startDate, endDate })
  const pen = data?.ai_penetration_rate ?? null
  const cov = data?.ai_coverage_rate ?? null
  const gap = pen != null && cov != null ? pen - cov : null

  return (
    <div className="glass rounded-2xl p-5 md:p-6 hover:shadow-lg transition-shadow flex flex-col">
      <h2 className="text-sm font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide mb-4">AI 渗透率</h2>
      {error ? (
        <div className="flex-1 flex items-center justify-center text-sm text-rose-600 dark:text-rose-400 min-h-[7rem]">
          加载失败：{(error as Error).message}
        </div>
      ) : isLoading || !data ? (
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 flex-1">
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="skeleton h-20 rounded-2xl" />
          ))}
        </div>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
          <MetricCard label="AI 渗透率" value={formatV2Ratio(pen)} hint="作者实际在用 AI 的需求占比（含被切散）" />
          <MetricCard label="数据覆盖率" value={formatV2Ratio(cov)} hint="看板能直接关联到 AI 会话的占比" />
          <MetricCard label="切散缺口" value={formatV2Ratio(gap)} hint="用了 AI 但被 need 边界切散、未进计算" />
        </div>
      )}
    </div>
  )
}
