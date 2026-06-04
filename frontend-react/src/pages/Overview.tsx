import { useMemo, type ReactNode } from 'react'
import { useDashboardSummary } from '@/api/queries'
import { getDefaultDateRangeWide, formatDateParam } from '@/lib/date'
import { formatNumber } from '@/lib/formatters'
import { MetricCard } from '@/components/ui/MetricCard'
import { HeroSaving } from '@/components/executive/HeroSaving'
import { TrendCard } from '@/components/executive/TrendCard'
import { AdoptionCard } from '@/components/executive/AdoptionCard'
import { TopRankCard } from '@/components/executive/TopRankCard'

// 高管提效总览大屏（PR1）。Bento 12 列网格 + 玻璃拟态 + 卡片 staggered 渐入。
// 四件套：① Hero 省人天&ROI ② 提效趋势 ③ Top 提效榜（需求/人）④ AI 采用度环形。
export default function Overview() {
  const [startDate, endDate] = useMemo(() => {
    const [s, e] = getDefaultDateRangeWide()
    return [formatDateParam(s), formatDateParam(e)]
  }, [])

  return (
    <div className="grid grid-cols-12 gap-4 lg:gap-6">
      {/* Row1 Hero */}
      <Cell index={0} className="col-span-12">
        <HeroSaving startDate={startDate} endDate={endDate} />
      </Cell>

      {/* Row2 趋势 + 采用度 */}
      <Cell index={1} className="col-span-12 lg:col-span-8">
        <TrendCard startDate={startDate} endDate={endDate} />
      </Cell>
      <Cell index={2} className="col-span-12 lg:col-span-4">
        <AdoptionCard startDate={startDate} endDate={endDate} />
      </Cell>

      {/* Row3 Top 榜（需求 / 人） + 规模概览 */}
      <Cell index={3} className="col-span-12 lg:col-span-6">
        <TopRankCard startDate={startDate} endDate={endDate} />
      </Cell>
      <Cell index={4} className="col-span-12 lg:col-span-6">
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

/** 底部规模概览卡：复用 PR0 的总仓库/用户/需求/Commit/代码行 MetricCard 网格。 */
function CountsCard({ startDate, endDate }: { startDate: string; endDate: string }) {
  const { data, isLoading, error } = useDashboardSummary({ startDate, endDate })

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
          <MetricCard label="总仓库数" value={formatNumber(data.total_repos)} hint={`分支 ${formatNumber(data.total_branchs)} 个`} />
          <MetricCard label="总用户数" value={formatNumber(data.total_users)} hint="参与提交的贡献者" />
          <MetricCard
            label="需求 Need"
            value={formatNumber(data.total_needs)}
            hint={`已合并 ${formatNumber(data.merged_needs)} · 可计入 ${formatNumber(data.eligible_needs)}`}
          />
          <MetricCard label="总 Commit" value={formatNumber(data.total_commits)} hint={`代码行 ${formatNumber(data.total_commit_lines)}`} />
          <MetricCard label="总代码行" value={formatNumber(data.total_commit_lines)} hint="commit 净改动行数" />
          <MetricCard label="活跃用户(V2)" value={formatNumber(data.total_users_v2)} hint="需求口径参与者" />
        </div>
      )}
    </div>
  )
}
