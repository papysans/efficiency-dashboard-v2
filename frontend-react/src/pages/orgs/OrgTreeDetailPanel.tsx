// 组织树右栏详情（V2 口径）：调 /v2/orgs/tree-detail（复用后端 aggregateUsersV2），
// 渲染 summary 指标卡 + 成员表。与 OrgDetailPanel（V1，聚合自空 user_productivity）不同，本面板数据真实可用。
// ⚠️ 小数口径 → RatioPill（×100），与 OrgList 一致；成员显示名来自 user_org.user_name（dept-sync 真实名）。
import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router'
import { getOrgTreeDetailV2 } from '@/api/endpoints'
import type { OrgTreeDetailSummary, UserV2Row } from '@/api/types'
import { formatDuration, formatNumber } from '@/lib/formatters'
import { MetricCard } from '@/components/ui/MetricCard'
import { RatioPill } from '@/components/ui/RatioPill'
import { Tag } from '@/components/ui/Tag'
import { fmtCostVal } from './orgDetailShared'

interface OrgTreeDetailPanelProps {
  /** "/" 分隔的组织路径，如 "深信服科技股份有限公司/研发体系/Costrict研发部/开发组"。空 → 占位提示。 */
  orgPath: string
  /** [startDate, endDate]，YYYY-MM-DD。 */
  dateRange: [string, string]
}

const TH = 'px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_NUM = 'px-3 py-2 text-right font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_CENTER = 'px-3 py-2 text-center font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TD = 'px-3 py-2 align-middle text-gray-700 dark:text-gray-200'
const TD_NUM = 'px-3 py-2 align-middle text-right tabular-nums text-gray-700 dark:text-gray-200'

/** 组织树右栏 V2 详情面板。 */
export function OrgTreeDetailPanel({ orgPath, dateRange }: OrgTreeDetailPanelProps) {
  const navigate = useNavigate()

  const dateParams = useMemo(
    () => ({ startDate: dateRange[0].replace(/-/g, ''), endDate: dateRange[1].replace(/-/g, '') }),
    [dateRange],
  )

  const [summary, setSummary] = useState<OrgTreeDetailSummary | null>(null)
  const [members, setMembers] = useState<UserV2Row[]>([])
  const [loading, setLoading] = useState(false)
  const [errMsg, setErrMsg] = useState('')

  useEffect(() => {
    if (!orgPath) {
      setSummary(null)
      setMembers([])
      return
    }
    let aborted = false
    setLoading(true)
    setErrMsg('')
    getOrgTreeDetailV2({ orgPath, ...dateParams })
      .then((res) => {
        if (aborted) return
        setSummary(res.summary)
        setMembers(res.members || [])
      })
      .catch((err: unknown) => {
        if (aborted) return
        setSummary(null)
        setMembers([])
        setErrMsg(err instanceof Error ? err.message : '获取组织详情失败')
      })
      .finally(() => {
        if (!aborted) setLoading(false)
      })
    return () => {
      aborted = true
    }
  }, [orgPath, dateParams])

  function goUser(userId: string) {
    if (!userId) return
    navigate({
      pathname: `/user/${encodeURIComponent(userId)}`,
      search: `?startDate=${dateParams.startDate}&endDate=${dateParams.endDate}`,
    })
  }

  if (!orgPath) {
    return (
      <div className="glass rounded-2xl p-8 text-center text-sm text-gray-400 dark:text-gray-500">请选择组织以查看详情</div>
    )
  }

  if (errMsg) {
    return <div className="glass rounded-2xl p-8 text-center text-sm text-rose-600 dark:text-rose-400">{errMsg}</div>
  }

  return (
    <div className={`space-y-5 ${loading ? 'opacity-60 pointer-events-none' : ''}`}>
      {/* summary 指标卡（8 张）— 小数口径 → RatioPill（×100），与 OrgList 一致 */}
      <section className="grid grid-cols-2 md:grid-cols-4 gap-3">
        <MetricCard label="用户数" value={formatNumber(summary?.user_count ?? 0)} />
        <MetricCard label="合并需求" value={formatNumber(summary?.merged_need_count ?? 0)} />
        <MetricCard label="实际日历" value={formatDuration(summary?.actual_calendar_min)} />
        <MetricCard label="日历提效" value={<RatioPill value={summary?.calendar_ratio} />} />
        <MetricCard label="工作量提效" value={<RatioPill value={summary?.work_ratio} />} />
        <MetricCard label="Commit" value={formatNumber(summary?.commit_count ?? 0)} />
        <MetricCard label="代码行" value={formatNumber(summary?.commit_diff_lines ?? 0)} />
        <MetricCard label="费用" value={fmtCostVal(summary?.cost)} />
      </section>

      {/* 成员表 */}
      <section className="glass rounded-2xl overflow-hidden">
        <div className="flex items-center justify-between px-5 py-3 border-b border-gray-200/50 dark:border-white/10">
          <span className="text-sm font-semibold text-gray-700 dark:text-gray-200">成员列表</span>
          <span className="text-xs text-gray-400 dark:text-gray-500">{members.length} 人 · 提效比按百分比展示（小数口径 ×100）</span>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-sm border-collapse">
            <thead>
              <tr className="border-b border-gray-200/50 dark:border-white/10">
                <th className={TH}>成员</th>
                <th className={TH_NUM}>合并需求</th>
                <th className={TH_NUM}>实际日历</th>
                <th className={TH_CENTER}>日历提效</th>
                <th className={TH_CENTER}>工作量提效</th>
                <th className={TH_NUM}>Commit</th>
                <th className={TH_NUM}>代码行</th>
              </tr>
            </thead>
            <tbody>
              {loading ? (
                Array.from({ length: 4 }).map((_, i) => (
                  <tr key={i} className="border-b border-gray-100/50 dark:border-white/5">
                    <td className={TD} colSpan={7}>
                      <div className="skeleton h-6 rounded" />
                    </td>
                  </tr>
                ))
              ) : !members.length ? (
                <tr>
                  <td colSpan={7}>
                    <div className="py-8 text-center text-sm text-gray-400 dark:text-gray-500">暂无成员</div>
                  </td>
                </tr>
              ) : (
                members.map((m) => (
                  <tr
                    key={m.user_id}
                    onClick={() => goUser(m.user_id)}
                    className="border-b border-gray-100/50 dark:border-white/5 cursor-pointer hover:bg-apple-blue/5 dark:hover:bg-white/5 transition-colors"
                  >
                    <td className={TD}>
                      <span className="inline-flex items-center gap-1.5">
                        <button
                          type="button"
                          className="text-apple-blue hover:text-apple-blue-hover cursor-pointer bg-transparent border-none p-0 focus:outline-none focus-visible:underline"
                          title={m.user_name}
                          onClick={(e) => {
                            e.stopPropagation()
                            goUser(m.user_id)
                          }}
                        >
                          {m.user_name || m.user_id}
                        </button>
                        {m.confidence_limited && (
                          <Tag tone="warning" title={m.confidence_reason || '数据置信度不足'}>受限</Tag>
                        )}
                      </span>
                    </td>
                    <td className={TD_NUM}>{m.merged_need_count}</td>
                    <td className={TD_NUM}>{formatDuration(m.actual_calendar_min)}</td>
                    <td className="px-3 py-2 align-middle text-center"><RatioPill value={m.calendar_ratio} /></td>
                    <td className="px-3 py-2 align-middle text-center"><RatioPill value={m.work_ratio} /></td>
                    <td className={TD_NUM}>{m.commit_count}</td>
                    <td className={TD_NUM}>{formatNumber(m.commit_diff_lines, 0)}</td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  )
}
