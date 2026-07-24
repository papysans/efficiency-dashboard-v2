// 效率维度 · 用户聚合态「纯效率排行」（批次2）。
// 替代原 <UserList/>（13 列大表，混了 AI占比/代码行/提交/费用 等他维串味列）。
// 本表只展示效率字段：日历提效比 / 人力(工作量)提效比 / 节省(人天) / 合并需求。
// 取数与「效率·分布」同源（useAllUsers，翻页拉全，绕过服务端切片截断）。
//
// 口径约定：
//  - calendar_ratio / work_ratio 为**小数口径**（0.25 => 25%）→ RatioPill / formatV2Ratio。
//  - 顶部「平均提效比」走守恒口径：Σbaseline / Σactual 加权（UserV2Row 暴露 per-user
//    baseline_*_min / actual_*_min 合计字段，可守恒），不取算术均值。
//  - 节省(人天)：日历 ÷1440、工作 ÷480；总节省卡用日历口径（baseline-actual）÷1440。
import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router'
import { useAllUsers } from '@/api/queries'
import type { UserV2Row } from '@/api/types'
import { useUserNameMap } from '@/hooks/useUserNameMap'
import { formatDateParam } from '@/lib/date'
import { formatNumber, formatV2Ratio } from '@/lib/formatters'
import { glossaryTip } from '@/lib/glossary'
import { parseOrder, sortRows, toOrder } from '@/lib/sort'
import { RatioPill } from '@/components/ui/RatioPill'
import { SortableTh } from '@/components/ui/SortableTh'
import { MetricCard } from '@/components/ui/MetricCard'
import { ChartCard, EmptyHint } from '@/pages/platform/platformShared'

/** 日历分钟 → 人天（÷1440），保留 1 位小数。 */
const CALENDAR_DAY_MIN = 1440

/** 显示名截断（>20 字截断加 …）。入参为已解析的真实用户名。 */
function shortName(name: string): string {
  const n = name || '-'
  return n.length > 20 ? `${n.slice(0, 20)}…` : n
}

// 仅效率列参与排序（snake_case，纯客户端，不传后端）。
const NUMERIC_FIELDS = new Set<string>(['calendar_ratio', 'work_ratio', 'calendar_saved_days', 'merged_need_count', 'silica'])

/** 该用户日历节省人天 = (baseline_calendar - actual_calendar) / 1440；非正 → null 沉底。 */
function calendarSavedDays(row: UserV2Row): number | null {
  const saved = (Number(row.baseline_calendar_min) || 0) - (Number(row.actual_calendar_min) || 0)
  return saved > 0 ? saved / CALENDAR_DAY_MIN : null
}

/** 排序取值器：calendar_saved_days 为派生列（单独算）；其余数值列 Number（非有限 → null 沉底）。 */
function getterFor(field: string): (row: UserV2Row) => unknown {
  if (field === 'calendar_saved_days') return (row) => calendarSavedDays(row)
  if (NUMERIC_FIELDS.has(field)) {
    return (row) => {
      const v = (row as unknown as Record<string, unknown>)[field]
      const num = Number(v)
      return Number.isFinite(num) ? num : null
    }
  }
  return (row) => String((row as unknown as Record<string, unknown>)[field] ?? '')
}

const TH = 'px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_NUM = 'px-3 py-2 text-right font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TD = 'px-3 py-2 align-middle text-gray-700 dark:text-gray-200'
const TD_NUM = 'px-3 py-2 align-middle text-right tabular-nums text-gray-700 dark:text-gray-200'

export default function EfficiencyUserRanking({ timeRange }: { timeRange: [string, string] }) {
  const navigate = useNavigate()
  const { resolveName } = useUserNameMap()

  // 默认按日历提效比降序（null 沉底）。三态循环：无→升→降→无。
  const [order, setOrder] = useState<string>(toOrder('calendar_ratio', true) ?? '')
  const parsedOrder = useMemo(() => parseOrder(order), [order])

  const dateParams = useMemo(
    () => ({ startDate: formatDateParam(timeRange[0]), endDate: formatDateParam(timeRange[1]) }),
    [timeRange],
  )

  const { data, isLoading, error } = useAllUsers(dateParams)
  const rows = useMemo<UserV2Row[]>(() => data ?? [], [data])

  const sortedRows = useMemo(() => {
    if (!parsedOrder) return rows
    return sortRows(rows, getterFor(parsedOrder.field), parsedOrder.desc)
  }, [rows, parsedOrder])

  // 顶部 KPI（守恒/合计口径，与下方列表同口径）。
  const kpi = useMemo(() => {
    let baseCal = 0
    let actCal = 0
    let baseWork = 0
    let actWork = 0
    let savedCalMin = 0
    for (const r of rows) {
      const bc = Number(r.baseline_calendar_min) || 0
      const ac = Number(r.actual_calendar_min) || 0
      baseCal += bc
      actCal += ac
      if (bc - ac > 0) savedCalMin += bc - ac
      baseWork += Number(r.baseline_work_min) || 0
      actWork += Number(r.actual_work_min) || 0
    }
    // 守恒加权提效比 = Σbaseline / Σactual（口径同 db.go 守恒聚合）。actual<=0 → null。
    const weighted = (base: number, act: number) => (act > 0 ? base / act : null)
    return {
      userCount: rows.length,
      avgCalRatio: weighted(baseCal, actCal),
      avgWorkRatio: weighted(baseWork, actWork),
      savedCalDays: savedCalMin / CALENDAR_DAY_MIN,
    }
  }, [rows])

  // 三态循环：无→升→降→无。换列从升序开始。
  function onSortChange(field: string) {
    const cur = parsedOrder
    if (!cur || cur.field !== field) setOrder(toOrder(field, false) ?? '')
    else if (!cur.desc) setOrder(toOrder(field, true) ?? '')
    else setOrder('')
  }

  const isSortActive = (field: string) => parsedOrder?.field === field
  const isSortDesc = (field: string) => parsedOrder?.field === field && parsedOrder?.desc === true

  function goToDetail(row: UserV2Row) {
    if (!row?.user_id) return
    navigate({
      pathname: `/user/${encodeURIComponent(row.user_id)}`,
      search: `?${new URLSearchParams(dateParams).toString()}`,
    })
  }

  return (
    <div className="space-y-5">
      {/* KPI 卡组（守恒/合计口径） */}
      <section className="grid grid-cols-2 lg:grid-cols-4 gap-3">
        <MetricCard label="用户数" value={formatNumber(kpi.userCount)} accent="#0071e3" />
        <MetricCard
          label="平均日历提效比"
          value={formatV2Ratio(kpi.avgCalRatio)}
          tip="守恒加权口径：Σ传统周期预估 ÷ Σ实际周期（小数口径），非算术均值"
          tone={kpi.avgCalRatio != null && kpi.avgCalRatio < 0 ? 'neg' : 'pos'}
        />
        <MetricCard
          label="平均人力提效比"
          value={formatV2Ratio(kpi.avgWorkRatio)}
          tip="守恒加权口径：Σ传统人力预估 ÷ Σ实际人力（小数口径），非算术均值"
          tone={kpi.avgWorkRatio != null && kpi.avgWorkRatio < 0 ? 'neg' : 'pos'}
        />
        <MetricCard
          label="总节省（人天）"
          value={kpi.savedCalDays > 0 ? formatNumber(kpi.savedCalDays, 1) : '-'}
          hint="Σ(传统周期预估 − 实际周期) ÷ 1440（仅计正节省）"
        />
      </section>

      {/* 排行表（只效率字段） */}
      <ChartCard title="用户效率排行" sub="纯效率口径 · 按日历提效比降序（点行下钻个人详情）">
        {error ? (
          <div className="px-1 py-2 text-sm text-rose-600 dark:text-rose-400">
            加载失败：{(error as Error).message}
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm border-collapse">
              <thead>
                <tr className="border-b border-gray-200/50 dark:border-white/10">
                  <th className={TH_NUM}>#</th>
                  <th className={TH}>用户</th>
                  <th className={TH}>
                    <SortableTh field="calendar_ratio" label="日历提效比" active={isSortActive('calendar_ratio')} desc={isSortDesc('calendar_ratio')} onSort={onSortChange} />
                  </th>
                  <th className={TH}>
                    <SortableTh field="work_ratio" label="人力提效比" active={isSortActive('work_ratio')} desc={isSortDesc('work_ratio')} onSort={onSortChange} />
                  </th>
                  <th className={TH} title={glossaryTip('silica')}>
                    <SortableTh field="silica" label="含硅量" active={isSortActive('silica')} desc={isSortDesc('silica')} onSort={onSortChange} />
                  </th>
                  <th className={TH_NUM}>
                    <span className="inline-flex justify-end w-full">
                      <SortableTh field="calendar_saved_days" label="节省（人天）" numeric active={isSortActive('calendar_saved_days')} desc={isSortDesc('calendar_saved_days')} onSort={onSortChange} />
                    </span>
                  </th>
                  <th className={TH_NUM}>
                    <span className="inline-flex justify-end w-full">
                      <SortableTh field="merged_need_count" label="合并需求" numeric active={isSortActive('merged_need_count')} desc={isSortDesc('merged_need_count')} onSort={onSortChange} />
                    </span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {isLoading ? (
                  Array.from({ length: 8 }).map((_, i) => (
                    <tr key={i} className="border-b border-gray-100/50 dark:border-white/5">
                      <td className={TD} colSpan={7}>
                        <div className="skeleton h-6 rounded" />
                      </td>
                    </tr>
                  ))
                ) : sortedRows.length === 0 ? (
                  <tr>
                    <td colSpan={7}>
                      <EmptyHint compact />
                    </td>
                  </tr>
                ) : (
                  sortedRows.map((row, i) => {
                    const saved = calendarSavedDays(row)
                    return (
                      <tr
                        key={row.user_id}
                        onClick={() => goToDetail(row)}
                        className="border-b border-gray-100/50 dark:border-white/5 cursor-pointer hover:bg-apple-blue/5 dark:hover:bg-white/5 transition-colors"
                      >
                        <td className={TD_NUM}>{i + 1}</td>
                        <td className={TD}>
                          <button
                            type="button"
                            className="text-apple-blue hover:text-apple-blue-hover cursor-pointer bg-transparent border-none p-0 focus:outline-none focus-visible:underline"
                            title={resolveName(row.user_id)}
                            onClick={(e) => {
                              e.stopPropagation()
                              goToDetail(row)
                            }}
                          >
                            {shortName(resolveName(row.user_id))}
                          </button>
                        </td>
                        <td className={TD}><RatioPill value={row.calendar_ratio} /></td>
                        <td className={TD}><RatioPill value={row.work_ratio} /></td>
                        <td className={TD}><RatioPill value={row.silica} /></td>
                        <td className={TD_NUM}>{saved != null ? formatNumber(saved, 1) : '-'}</td>
                        <td className={TD_NUM}>{formatNumber(row.merged_need_count)}</td>
                      </tr>
                    )
                  })
                )}
              </tbody>
            </table>
          </div>
        )}
      </ChartCard>
    </div>
  )
}
