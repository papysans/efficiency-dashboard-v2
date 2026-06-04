// 用户组（虚拟组）详情页（UserGroupDetail 的 React + 玻璃拟态迁移）。
// 分区/列/口径 1:1 按 research/pr3-user-repo-org.md §User-3；⚠️ 百分比口径 → PercentPill（不 ×100）。
// 后端无 user-groups 列表端点，本页只能通过直链/路由访问；做好空/404 态。删除后跳回 /user-v2。
import { useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router'
import { useUserGroupDetail } from '@/api/queries'
import { deleteUserGroup } from '@/api/endpoints'
import type { UserGroupMember } from '@/api/types'
import { fmtCost, formatNumber } from '@/lib/formatters'
import { getDefaultDateRangeWide } from '@/lib/date'
import { MetricCard } from '@/components/ui/MetricCard'
import { PercentPill } from '@/components/ui/PercentPill'
import { DateRangePicker } from '@/components/ui/DateRangePicker'
import { Modal } from '@/components/ui/Modal'

/** 费用单值（对齐 Vue fmtCostVal：null → '-'，否则 2 位）。 */
function fmtCostVal(value: number | null | undefined): string {
  if (value == null) return '-'
  return fmtCost(value)
}

const TH = 'px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_NUM = 'px-3 py-2 text-right font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_CENTER = 'px-3 py-2 text-center font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TD = 'px-3 py-2 align-middle text-gray-700 dark:text-gray-200'
const TD_NUM = 'px-3 py-2 align-middle text-right tabular-nums text-gray-700 dark:text-gray-200'
const TD_CENTER = 'px-3 py-2 align-middle text-center text-gray-700 dark:text-gray-200'

export default function UserGroupDetail() {
  const { groupId } = useParams<{ groupId: string }>()
  const navigate = useNavigate()

  const [dateRange, setDateRange] = useState<[string, string]>(getDefaultDateRangeWide())
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [deleting, setDeleting] = useState(false)

  const params = useMemo(
    () => ({ startDate: dateRange[0].replace(/-/g, ''), endDate: dateRange[1].replace(/-/g, '') }),
    [dateRange],
  )

  const { data, isLoading, error } = useUserGroupDetail(groupId, params)

  const group = data?.group
  const summary = data?.summary
  const members: UserGroupMember[] = useMemo(() => data?.members || [], [data?.members])

  async function handleDelete() {
    if (!groupId) return
    setDeleting(true)
    try {
      await deleteUserGroup(groupId)
      setConfirmOpen(false)
      navigate('/user-v2')
    } catch {
      // 错误已由 client 拦截器转成 Error message，这里只复位删除态
      setDeleting(false)
    }
  }

  return (
    <div className={`space-y-5 ${isLoading ? 'opacity-60 pointer-events-none' : ''}`}>
      {/* 标题栏 */}
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-3 min-w-0">
          <button
            type="button"
            onClick={() => navigate(-1)}
            className="inline-flex items-center gap-1 text-sm text-gray-500 dark:text-gray-400 hover:text-apple-blue cursor-pointer bg-transparent border-none p-0 transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue rounded"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
            </svg>
            返回
          </button>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white truncate">虚拟组: {group?.name || '-'}</h1>
        </div>
        <div className="flex items-center gap-2">
          <DateRangePicker value={dateRange} onChange={setDateRange} />
          <button
            type="button"
            onClick={() => setConfirmOpen(true)}
            disabled={!groupId}
            className="rounded-lg px-4 py-1.5 text-sm font-medium cursor-pointer transition-colors bg-rose-600 hover:bg-rose-500 text-white disabled:opacity-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-rose-500"
          >
            删除此组
          </button>
        </div>
      </header>

      {error && (
        <div className="glass rounded-2xl p-8 text-center text-sm text-rose-600 dark:text-rose-400">
          {(error as Error).message || '获取用户组详情失败'}
        </div>
      )}

      {!error && (
        <>
          {/* 6 张汇总卡 */}
          <section className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-3">
            <MetricCard label="成员数" value={formatNumber(members.length)} />
            <MetricCard label="总 Task 数" value={formatNumber(summary?.task_count ?? 0)} />
            <MetricCard label="总 Commit 数" value={formatNumber(summary?.commit_count ?? 0)} />
            <MetricCard label="加权 Task 提效比" value={<PercentPill value={summary?.task_efficiency_ratio} />} />
            <MetricCard label="加权 Commit 提效比" value={<PercentPill value={summary?.commit_efficiency_ratio} />} />
            <MetricCard label="总费用" value={fmtCostVal(summary?.cost)} />
          </section>

          {/* 成员明细 */}
          <section className="glass rounded-2xl overflow-hidden">
            <div className="flex items-center justify-between px-5 py-3 border-b border-gray-200/50 dark:border-white/10">
              <span className="text-sm font-semibold text-gray-700 dark:text-gray-200">成员明细</span>
              <span className="text-xs text-gray-400 dark:text-gray-500">提效比为百分比口径（300=300%）</span>
            </div>
            <div className="overflow-x-auto">
              <table className="w-full text-sm border-collapse">
                <thead>
                  <tr className="border-b border-gray-200/50 dark:border-white/10">
                    <th className={TH}>用户名</th>
                    <th className={TH_NUM}>活跃天数</th>
                    <th className={TH_NUM}>Task数</th>
                    <th className={TH_NUM}>Commit数</th>
                    <th className={TH_CENTER}>Task提效比</th>
                    <th className={TH_CENTER}>Commit提效比</th>
                    <th className={TH_NUM}>费用</th>
                  </tr>
                </thead>
                <tbody>
                  {isLoading ? (
                    Array.from({ length: 5 }).map((_, i) => (
                      <tr key={i} className="border-b border-gray-100/50 dark:border-white/5">
                        <td className={TD} colSpan={7}>
                          <div className="skeleton h-6 rounded" />
                        </td>
                      </tr>
                    ))
                  ) : !members.length ? (
                    <tr>
                      <td colSpan={7}>
                        <div className="py-12 text-center text-sm text-gray-400 dark:text-gray-500">暂无数据</div>
                      </td>
                    </tr>
                  ) : (
                    members.map((m) => (
                      <tr
                        key={m.user_id}
                        onClick={() => navigate(`/user/${encodeURIComponent(m.user_id)}`)}
                        className="border-b border-gray-100/50 dark:border-white/5 cursor-pointer hover:bg-apple-blue/5 dark:hover:bg-white/5 transition-colors"
                      >
                        <td className={TD}>
                          <div className="max-w-[260px] truncate" title={m.user_name}>{m.user_name || '-'}</div>
                        </td>
                        <td className={TD_NUM}>{m.day_count}</td>
                        <td className={TD_NUM}>{m.task_count}</td>
                        <td className={TD_NUM}>{m.commit_count}</td>
                        <td className={TD_CENTER}><PercentPill value={m.task_efficiency_ratio} /></td>
                        <td className={TD_CENTER}><PercentPill value={m.commit_efficiency_ratio} /></td>
                        <td className={TD_NUM}>{fmtCostVal(m.cost)}</td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </section>
        </>
      )}

      {/* 删除确认 */}
      <Modal
        open={confirmOpen}
        title="删除虚拟组"
        onClose={() => !deleting && setConfirmOpen(false)}
        maxWidth={420}
        footer={
          <>
            <button
              type="button"
              onClick={() => setConfirmOpen(false)}
              disabled={deleting}
              className="glass rounded-lg px-4 py-1.5 text-sm text-gray-700 dark:text-gray-200 cursor-pointer hover:text-apple-blue transition-colors disabled:opacity-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue"
            >
              取消
            </button>
            <button
              type="button"
              onClick={handleDelete}
              disabled={deleting}
              className="rounded-lg px-4 py-1.5 text-sm font-medium cursor-pointer transition-colors bg-rose-600 hover:bg-rose-500 text-white disabled:opacity-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-rose-500"
            >
              {deleting ? '删除中…' : '确认删除'}
            </button>
          </>
        }
      >
        <p className="text-sm text-gray-600 dark:text-gray-300">
          确认删除虚拟组「{group?.name || groupId}」？此操作不可撤销。
        </p>
      </Modal>
    </div>
  )
}
