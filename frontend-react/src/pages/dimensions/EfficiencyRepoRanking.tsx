// 「效率」维度 · 仓库聚合态的纯效率排行（批次2，替代原 <RepoList/> 的 13/9 列大表）。
// 只展示效率字段：提效比 / 节省(人天) / Commit数。删掉串味列（AI占比属使用维、Task/费用属各自维度）。
//
// ⚠️ 口径：RepoListItem.efficiency_ratio 是**百分比口径**（300=300%，绝不再 ×100），用 PercentPill。
//   平均提效比为**守恒口径**：Σ(古法预估 - 实际) / Σ实际 × 100（用每行 sum_ancient_minutes/sum_real_minutes 合计推导），
//   非各行提效比的算术均值——本维度行类型已暴露 per-对象 ancient/real 合计，故可守恒，不需等后端。
//   节省(人天)=Σ(古法预估min - 实际min) / 480（工作口径 8h/人天）。
//
// 取数：useRepos({page:1,pageSize:1000})，与仓库分布同源（整仓跨分支聚合）。按提效比降序。
import { useMemo } from 'react'
import { useNavigate } from 'react-router'
import { useRepos } from '@/api/queries'
import type { RepoListItem } from '@/api/types'
import { formatDateParam } from '@/lib/date'
import { formatNumber } from '@/lib/formatters'
import { sortRows } from '@/lib/sort'
import { MetricCard } from '@/components/ui/MetricCard'
import { PercentPill } from '@/components/ui/PercentPill'

const WORK_MIN_PER_DAY = 480 // 工作口径：8h/人天

const TH = 'px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_NUM = 'px-3 py-2 text-right font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_CENTER = 'px-3 py-2 text-center font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TD = 'px-3 py-2 align-middle text-gray-700 dark:text-gray-200'
const TD_NUM = 'px-3 py-2 align-middle text-right tabular-nums text-gray-700 dark:text-gray-200'

/** 节省人天：工作分钟差 / 480，保留 1 位。 */
function savedPersonDays(ancientMin: number | null | undefined, realMin: number | null | undefined): number | null {
  const a = Number(ancientMin)
  const r = Number(realMin)
  if (!Number.isFinite(a) || !Number.isFinite(r)) return null
  return (a - r) / WORK_MIN_PER_DAY
}

export default function EfficiencyRepoRanking({ timeRange }: { timeRange: [string, string] }) {
  const navigate = useNavigate()

  const startDate = formatDateParam(timeRange[0])
  const endDate = formatDateParam(timeRange[1])
  const { data, isLoading, error } = useRepos({ startDate, endDate, page: 1, pageSize: 1000 })

  const rows = useMemo<RepoListItem[]>(() => data?.data ?? [], [data])

  // 提效比降序（百分比口径，null 沉底）。
  const sorted = useMemo(() => sortRows(rows, (r: RepoListItem) => r.efficiency_ratio, true), [rows])

  // 守恒聚合：用每行古法/实际分钟合计推导平均提效比与总节省（不取行提效比算术均值）。
  const agg = useMemo(() => {
    let sumAncient = 0
    let sumReal = 0
    for (const r of rows) {
      const a = Number(r.sum_ancient_minutes)
      const rl = Number(r.sum_real_minutes)
      if (Number.isFinite(a)) sumAncient += a
      if (Number.isFinite(rl)) sumReal += rl
    }
    const avgRatioPct = sumReal > 0 ? ((sumAncient - sumReal) / sumReal) * 100 : null
    const savedDays = (sumAncient - sumReal) / WORK_MIN_PER_DAY
    return { repoCount: rows.length, avgRatioPct, savedDays }
  }, [rows])

  function goToRepo(addr: string) {
    if (!addr) return
    // 整仓口径：分支留空，下钻进整仓详情（详情页内再切分支）。对齐 kanbanDerived useRepoNav。
    navigate(`/repo/${encodeURIComponent(addr)}/${encodeURIComponent('')}`)
  }

  return (
    <div className="space-y-5">
      {/* KPI 卡组（守恒口径） */}
      <div className="glass rounded-2xl p-5 md:p-6 space-y-4">
        <div className="flex items-center justify-between gap-3">
          <h2 className="text-sm font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide">仓库提效概览</h2>
          <span className="text-xs text-gray-400 dark:text-gray-500 text-right">
            整仓跨分支聚合 · 提效比为百分比口径（300=300%）
          </span>
        </div>

        {error ? (
          <div className="text-sm text-rose-600 dark:text-rose-400">加载失败：{(error as Error).message}</div>
        ) : isLoading ? (
          <div className="grid grid-cols-2 md:grid-cols-3 gap-3">
            {Array.from({ length: 3 }).map((_, i) => (
              <div key={i} className="skeleton h-20 rounded-2xl" />
            ))}
          </div>
        ) : (
          <div className="grid grid-cols-2 md:grid-cols-3 gap-3">
            <MetricCard label="仓库数" value={formatNumber(agg.repoCount)} accent="#0071e3" />
            <MetricCard
              label="平均提效比"
              value={agg.avgRatioPct == null ? '-' : `${agg.avgRatioPct.toFixed(1)}%`}
              tip="守恒口径：Σ(古法预估 − 实际) / Σ实际 ×100（按古法/实际分钟合计加权，非各仓提效比算术均值）"
              tone={agg.avgRatioPct != null && agg.avgRatioPct < 0 ? 'neg' : 'pos'}
            />
            <MetricCard
              label="总节省"
              value={`${formatNumber(agg.savedDays, 1)} 人天`}
              hint="Σ(古法预估 − 实际) / 480（工作口径 8h/人天）"
            />
          </div>
        )}
      </div>

      {/* 纯效率排行表 */}
      <section className="glass rounded-2xl overflow-hidden">
        <div className="flex items-center justify-between px-5 py-3 border-b border-gray-200/50 dark:border-white/10">
          <span className="text-sm font-semibold text-gray-700 dark:text-gray-200">仓库效率排行</span>
          <span className="text-xs text-gray-400 dark:text-gray-500">按提效比降序 · 仅效率字段</span>
        </div>

        <div className="overflow-x-auto">
          <table className="w-full text-sm border-collapse">
            <thead>
              <tr className="border-b border-gray-200/50 dark:border-white/10">
                <th className={`${TH_NUM} w-16`}>排名</th>
                <th className={`${TH} min-w-[300px]`}>仓库地址</th>
                <th className={TH_CENTER}>提效比</th>
                <th className={TH_NUM}>节省(人天)</th>
                <th className={TH_NUM}>Commit数</th>
              </tr>
            </thead>
            <tbody>
              {isLoading ? (
                Array.from({ length: 8 }).map((_, i) => (
                  <tr key={i} className="border-b border-gray-100/50 dark:border-white/5">
                    <td className={TD} colSpan={5}>
                      <div className="skeleton h-6 rounded" />
                    </td>
                  </tr>
                ))
              ) : error ? (
                <tr>
                  <td colSpan={5}>
                    <div className="py-12 text-center text-sm text-rose-600 dark:text-rose-400">
                      加载失败：{(error as Error).message}
                    </div>
                  </td>
                </tr>
              ) : sorted.length === 0 ? (
                <tr>
                  <td colSpan={5}>
                    <div className="py-12 text-center text-sm text-gray-400 dark:text-gray-500">暂无仓库数据</div>
                  </td>
                </tr>
              ) : (
                sorted.map((row, i) => {
                  const saved = savedPersonDays(row.sum_ancient_minutes, row.sum_real_minutes)
                  return (
                    <tr
                      key={row.repo_addr}
                      onClick={() => goToRepo(row.repo_addr)}
                      className="border-b border-gray-100/50 dark:border-white/5 cursor-pointer hover:bg-apple-blue/5 dark:hover:bg-white/5 transition-colors"
                    >
                      <td className={`${TD_NUM} text-gray-400 dark:text-gray-500`}>{i + 1}</td>
                      <td className={TD}>
                        <div className="max-w-[360px] truncate" title={row.repo_addr}>
                          {row.repo_addr || '-'}
                        </div>
                      </td>
                      <td className="px-3 py-2 align-middle text-center">
                        <PercentPill value={row.efficiency_ratio} />
                      </td>
                      <td className={TD_NUM}>{saved == null ? '-' : `${formatNumber(saved, 1)} 人天`}</td>
                      <td className={TD_NUM}>{formatNumber(row.commit_count)}</td>
                    </tr>
                  )
                })
              )}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  )
}
