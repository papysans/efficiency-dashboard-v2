// 组织树右栏：dept-sync 部门成员花名册 + 看板 V2 指标（按 universal_id 左连）。
// 调 /v2/dept-tree/members?dept_id=&startDate=&endDate=。成员含「无看板数据」者（指标显示 —，挂「无活动」浅标）。
// 有看板数据的成员真名可点跳 /user/:universal_id（universal_id == 看板 user_id）。
// ⚠️ 提效比为小数口径 → RatioPill（×100），与 OrgList 一致。
import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router'
import { getDeptTreeMembersV2 } from '@/api/endpoints'
import type { DeptMember, DeptMembersSummary } from '@/api/types'
import { formatDuration, formatNumber } from '@/lib/formatters'
import { MetricCard } from '@/components/ui/MetricCard'
import { RatioPill } from '@/components/ui/RatioPill'
import { Tag } from '@/components/ui/Tag'

/** 费用单值（null → '-'，否则 2 位）。 */
function fmtCostVal(value: number | null | undefined): string {
  if (value == null) return '-'
  return Number(value).toFixed(2)
}

interface DeptMembersPanelProps {
  /** 选中的 dept-sync 部门 ID。空 → 占位提示。 */
  deptId: string
  /** 选中部门名（标题展示）。 */
  deptName: string
  /** [startDate, endDate]，YYYY-MM-DD。 */
  dateRange: [string, string]
}

const TH = 'px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_NUM = 'px-3 py-2 text-right font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_CENTER = 'px-3 py-2 text-center font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TD = 'px-3 py-2 align-middle text-gray-700 dark:text-gray-200'
const TD_NUM = 'px-3 py-2 align-middle text-right tabular-nums text-gray-700 dark:text-gray-200'
const DASH = <span className="text-gray-300 dark:text-gray-600">—</span>

/** 组织树右栏部门成员面板。 */
export function DeptMembersPanel({ deptId, deptName, dateRange }: DeptMembersPanelProps) {
  const navigate = useNavigate()

  const dateParams = useMemo(
    () => ({ startDate: dateRange[0].replace(/-/g, ''), endDate: dateRange[1].replace(/-/g, '') }),
    [dateRange],
  )

  const [summary, setSummary] = useState<DeptMembersSummary | null>(null)
  const [members, setMembers] = useState<DeptMember[]>([])
  const [loading, setLoading] = useState(false)
  const [errMsg, setErrMsg] = useState('')

  useEffect(() => {
    if (!deptId) {
      setSummary(null)
      setMembers([])
      return
    }
    let aborted = false
    setLoading(true)
    setErrMsg('')
    getDeptTreeMembersV2({ deptId, ...dateParams })
      .then((res) => {
        if (aborted) return
        setSummary(res.summary)
        setMembers(res.members || [])
      })
      .catch((err: unknown) => {
        if (aborted) return
        setSummary(null)
        setMembers([])
        setErrMsg(err instanceof Error ? err.message : '获取部门成员失败')
      })
      .finally(() => {
        if (!aborted) setLoading(false)
      })
    return () => {
      aborted = true
    }
  }, [deptId, dateParams])

  function goUser(universalId: string) {
    if (!universalId) return
    navigate({
      pathname: `/user/${encodeURIComponent(universalId)}`,
      search: `?startDate=${dateParams.startDate}&endDate=${dateParams.endDate}`,
    })
  }

  if (!deptId) {
    return (
      <div className="glass rounded-2xl p-8 text-center text-sm text-gray-400 dark:text-gray-500">请在左树选择部门以查看成员</div>
    )
  }

  if (errMsg) {
    return <div className="glass rounded-2xl p-8 text-center text-sm text-rose-600 dark:text-rose-400">{errMsg}</div>
  }

  return (
    <div className={`space-y-5 ${loading ? 'opacity-60 pointer-events-none' : ''}`}>
      {/* summary 指标卡 — 小数口径 → RatioPill（×100） */}
      <section className="grid grid-cols-2 md:grid-cols-4 gap-3">
        <MetricCard label="成员数" value={formatNumber(summary?.member_count ?? 0)} hint={summary ? `${summary.kanban_member_count} 人有看板数据` : undefined} />
        <MetricCard label="合并需求" value={formatNumber(summary?.merged_need_count ?? 0)} />
        <MetricCard label="实际周期" value={formatDuration(summary?.actual_calendar_min)} />
        <MetricCard label="日历提效" value={<RatioPill value={summary?.calendar_ratio} />} />
        <MetricCard label="人力提效" value={<RatioPill value={summary?.work_ratio} />} />
        <MetricCard label="AI 代码占比" value={<RatioPill value={summary?.ai_code_ratio} />} />
        <MetricCard label="Commit" value={formatNumber(summary?.commit_count ?? 0)} />
        <MetricCard label="代码行" value={formatNumber(summary?.commit_diff_lines ?? 0)} />
        <MetricCard label="费用" value={fmtCostVal(summary?.cost)} />
      </section>

      {/* 成员表 */}
      <section className="glass rounded-2xl overflow-hidden">
        <div className="flex items-center justify-between px-5 py-3 border-b border-gray-200/50 dark:border-white/10">
          <span className="text-sm font-semibold text-gray-700 dark:text-gray-200">{deptName || '部门'} · 成员花名册</span>
          <span className="text-xs text-gray-400 dark:text-gray-500">
            {members.length} 人（直属）· 按可计入需求汇总
          </span>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-sm border-collapse">
            <thead>
              <tr className="border-b border-gray-200/50 dark:border-white/10">
                <th className={TH}>成员</th>
                <th className={TH}>工号</th>
                <th className={TH}>职级</th>
                <th className={TH_NUM}>合并需求</th>
                <th className={TH_NUM}>实际周期</th>
                <th className={TH_CENTER}>日历提效</th>
                <th className={TH_CENTER}>人力提效</th>
                <th className={TH_CENTER}>AI 代码占比</th>
                <th className={TH_NUM}>Commit</th>
                <th className={TH_NUM}>代码行</th>
              </tr>
            </thead>
            <tbody>
              {loading ? (
                Array.from({ length: 4 }).map((_, i) => (
                  <tr key={i} className="border-b border-gray-100/50 dark:border-white/5">
                    <td className={TD} colSpan={10}>
                      <div className="skeleton h-6 rounded" />
                    </td>
                  </tr>
                ))
              ) : !members.length ? (
                <tr>
                  <td colSpan={10}>
                    <div className="py-8 text-center text-sm text-gray-400 dark:text-gray-500">
                      该部门无直属成员（可能是中间部门，请下钻到子部门）
                    </div>
                  </td>
                </tr>
              ) : (
                members.map((m) => {
                  const clickable = m.has_kanban_data && !!m.universal_id
                  return (
                    <tr
                      key={m.emp_no || m.universal_id}
                      onClick={clickable ? () => goUser(m.universal_id) : undefined}
                      className={`border-b border-gray-100/50 dark:border-white/5 transition-colors ${
                        clickable ? 'cursor-pointer hover:bg-apple-blue/5 dark:hover:bg-white/5' : ''
                      }`}
                    >
                      <td className={TD}>
                        <span className="inline-flex items-center gap-1.5">
                          {clickable ? (
                            <button
                              type="button"
                              className="text-apple-blue hover:text-apple-blue-hover cursor-pointer bg-transparent border-none p-0 focus:outline-none focus-visible:underline"
                              title={m.real_name}
                              onClick={(e) => {
                                e.stopPropagation()
                                goUser(m.universal_id)
                              }}
                            >
                              {m.real_name || m.emp_no}
                            </button>
                          ) : (
                            <span className="text-gray-700 dark:text-gray-200" title={m.real_name}>
                              {m.real_name || m.emp_no || '—'}
                            </span>
                          )}
                          {!m.has_kanban_data && (
                            <Tag tone="neutral" title="该成员在所选时间窗口内无看板活动数据">无活动</Tag>
                          )}
                        </span>
                      </td>
                      <td className={TD}>{m.emp_no || DASH}</td>
                      <td className={TD}>{m.position || DASH}</td>
                      <td className={TD_NUM}>{m.has_kanban_data ? formatNumber(m.merged_need_count) : DASH}</td>
                      <td className={TD_NUM}>{m.has_kanban_data ? formatDuration(m.actual_calendar_min) : DASH}</td>
                      <td className="px-3 py-2 align-middle text-center">
                        {m.has_kanban_data ? <RatioPill value={m.calendar_ratio} /> : DASH}
                      </td>
                      <td className="px-3 py-2 align-middle text-center">
                        {m.has_kanban_data ? <RatioPill value={m.work_ratio} /> : DASH}
                      </td>
                      <td className="px-3 py-2 align-middle text-center">
                        {m.has_kanban_data ? <RatioPill value={m.ai_code_ratio} /> : DASH}
                      </td>
                      <td className={TD_NUM}>{m.has_kanban_data ? formatNumber(m.commit_count) : DASH}</td>
                      <td className={TD_NUM}>{m.has_kanban_data ? formatNumber(m.commit_diff_lines, 0) : DASH}</td>
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
