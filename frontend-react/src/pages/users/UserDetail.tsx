// 用户详情页（UserDetailV2 的 React + 玻璃拟态迁移）。
// 分区/列/口径 1:1 按 research/pr3-user-repo-org.md §User-2；小数口径 → RatioPill（×100）。
// 周趋势图：日历提效手动 ×100 喂给图表（小数口径），merged_need_count 用 bar。
import { useMemo, type ReactNode } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router'
import { useUserDetail } from '@/api/queries'
import { useTheme } from '@/hooks/useTheme'
import { useUserNameMap } from '@/hooks/useUserNameMap'
import type { NeedCommit, NeedsV2Summary, UserProductivityV2 } from '@/api/types'
import { formatDuration, formatLocalTime, formatNumber } from '@/lib/formatters'
import { getDefaultDateRangeWide } from '@/lib/date'
import { MetricCard } from '@/components/ui/MetricCard'
import { RatioPill } from '@/components/ui/RatioPill'
import { Tag, type TagTone } from '@/components/ui/Tag'
import { EChart } from '@/components/charts/EChart'
import { barOption } from '@/components/charts/barOption'

function shortId(value?: string | null, size = 8): string {
  if (!value) return '-'
  return String(value).slice(0, size)
}

function statusClass(status?: string): TagTone {
  if (status === 'merged') return 'success'
  if (status === 'active') return 'primary'
  return 'neutral'
}

/** 周起始本地日期（YYYY-MM-DD），取本地时间前 10 位等价（对齐 Vue fmtDate）。 */
function fmtWeek(weekStart?: string): string {
  if (!weekStart) return '-'
  const d = new Date(weekStart)
  if (Number.isNaN(d.getTime())) return '-'
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

function normalizeDateQuery(value: string | null): [string, string] | null {
  const toIso = (s: string | null): string => {
    if (!s) return ''
    const t = s.trim()
    if (/^\d{8}$/.test(t)) return `${t.slice(0, 4)}-${t.slice(4, 6)}-${t.slice(6, 8)}`
    if (/^\d{4}-\d{2}-\d{2}$/.test(t)) return t
    return ''
  }
  const start = toIso(value)
  return start ? [start, ''] : null
}

const TH = 'px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_NUM = 'px-3 py-2 text-right font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TD = 'px-3 py-2 align-middle text-gray-700 dark:text-gray-200'
const TD_NUM = 'px-3 py-2 align-middle text-right tabular-nums text-gray-700 dark:text-gray-200'

/**
 * UserDetail 既是独立路由页（/user/:userId，从 URL 取 userId/日期），
 * 也可被「个人·效率」聚焦态壳内嵌（传 userIdProp + dateRangeProp，隐藏返回按钮/标题外框）。
 */
interface UserDetailProps {
  userIdProp?: string
  dateRangeProp?: [string, string]
  /** 嵌入壳内时设 true：去掉「返回」按钮与外层标题，避免与壳的面包屑/标题重复。 */
  embedded?: boolean
}

export default function UserDetail({ userIdProp, dateRangeProp, embedded = false }: UserDetailProps = {}) {
  const routeParams = useParams<{ userId: string }>()
  const userId = userIdProp ?? routeParams.userId
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const { theme } = useTheme()
  // 用 commits 的 git_user_name 把 user_id(UUID) 解析为真实名。
  const { resolveName } = useUserNameMap()

  // 日期：嵌入态用 prop（全局 timeRange）；独立页取 URL（YYYYMMDD/YYYY-MM-DD）→ ISO；缺则近 90 天。
  const dateRange = useMemo<[string, string]>(() => {
    if (dateRangeProp) return dateRangeProp
    const startRaw = normalizeDateQuery(searchParams.get('startDate'))
    const endRaw = normalizeDateQuery(searchParams.get('endDate'))
    if (startRaw && endRaw) return [startRaw[0], endRaw[0]]
    return getDefaultDateRangeWide()
  }, [searchParams, dateRangeProp])

  const params = useMemo(
    () => ({ startDate: dateRange[0].replace(/-/g, ''), endDate: dateRange[1].replace(/-/g, '') }),
    [dateRange],
  )

  const { data, isLoading, error } = useUserDetail(userId, params)

  const summary = data?.summary
  const weeks: UserProductivityV2[] = useMemo(() => data?.weeks || [], [data?.weeks])
  const needs: NeedsV2Summary[] = data?.needs || []
  const commits: NeedCommit[] = data?.commits || []

  // 周趋势：按 week_start 升序；series1=line 日历提效(×100)、series2=bar 合并需求。
  const weeklyChartOption = useMemo(() => {
    if (!weeks.length) return null
    const sorted = [...weeks].sort((a, b) => new Date(a.week_start).getTime() - new Date(b.week_start).getTime())
    const labels = sorted.map((w) => fmtWeek(w.week_start))
    const ratioData = sorted.map((w) => Number((w.efficiency_ratio ?? 0)) * 100)
    const mergedData = sorted.map((w) => Number(w.merged_need_count ?? 0))
    return barOption(
      theme,
      '周日历提效（%）',
      labels,
      [
        { name: '日历提效', data: ratioData, type: 'line' },
        { name: '合并需求', data: mergedData, type: 'bar' },
      ],
      { titleSize: 14 },
    )
  }, [weeks, theme])

  if (error) {
    return (
      <div className="glass rounded-2xl p-8 text-center text-sm text-rose-600 dark:text-rose-400">
        {(error as Error).message || '获取用户详情失败'}
      </div>
    )
  }

  return (
    <div className={`space-y-5 ${isLoading ? 'opacity-60 pointer-events-none' : ''}`}>
      {/* Header（嵌入壳内时省略：壳已有标题/面包屑/对象选择器） */}
      {!embedded && (
        <header className="space-y-3">
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
          <div className="min-w-0">
            <h1 className="text-2xl font-bold text-gray-900 dark:text-white">用户详情</h1>
            <p className="text-sm text-gray-500 dark:text-gray-400 mt-1 font-mono break-all">
              {resolveName(summary?.user_id || userId)}
            </p>
          </div>
        </header>
      )}

      {/* 6 张 MetricCard */}
      <section className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-3">
        <MetricCard label="合并需求" value={formatNumber(summary?.merged_need_count ?? 0)} />
        <MetricCard label="日历提效" value={<RatioPill value={summary?.calendar_ratio} />} />
        <MetricCard label="人力提效" value={<RatioPill value={summary?.work_ratio} />} />
        <MetricCard label="实际周期" value={formatDuration(summary?.actual_calendar_min)} />
        <MetricCard label="传统周期预估" value={formatDuration(summary?.baseline_calendar_min)} />
        <MetricCard
          label="Commit / 代码行"
          value={`${summary?.commit_count ?? 0} / ${formatNumber(summary?.commit_diff_lines, 0)}`}
        />
      </section>

      {/* 周明细 + 周趋势 */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <Panel title="周明细" hint={`${weeks.length} 周`} bodyClass="overflow-x-auto">
          <table className="w-full text-sm border-collapse">
            <thead>
              <tr className="border-b border-gray-200/50 dark:border-white/10">
                <th className={TH}>周起始</th>
                <th className={TH_NUM}>合并</th>
                <th className={TH_NUM}>活跃</th>
                <th className={TH}>日历提效</th>
                <th className={TH}>人力提效</th>
                <th className={TH_NUM}>Commit</th>
                <th className={TH}>置信</th>
              </tr>
            </thead>
            <tbody>
              {!weeks.length ? (
                <tr>
                  <td colSpan={7}>
                    <div className="py-6 text-center text-sm text-gray-400 dark:text-gray-500">暂无周数据</div>
                  </td>
                </tr>
              ) : (
                weeks.map((w) => (
                  <tr key={w.user_productivity_v2_id || w.week_start} className="border-b border-gray-100/50 dark:border-white/5">
                    <td className={TD}>{fmtWeek(w.week_start)}</td>
                    <td className={TD_NUM}>{w.merged_need_count}</td>
                    <td className={TD_NUM}>{w.active_need_count}</td>
                    <td className={TD}><RatioPill value={w.efficiency_ratio} /></td>
                    <td className={TD}><RatioPill value={w.work_efficiency_ratio} /></td>
                    <td className={TD_NUM}>{w.commit_count}</td>
                    <td className={TD}>{w.confidence_limited ? <Tag tone="warning">受限</Tag> : <Tag tone="success">正常</Tag>}</td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </Panel>
        <div className="glass rounded-2xl p-4 min-h-[280px] flex items-center justify-center">
          {weeklyChartOption ? (
            <EChart option={weeklyChartOption} height={280} className="w-full" />
          ) : (
            <div className="text-sm text-gray-400 dark:text-gray-500">暂无周趋势数据</div>
          )}
        </div>
      </div>

      {/* 关联需求 Need */}
      <Panel title="关联需求 Need" hint={`${needs.length} 个`} bodyClass="overflow-x-auto">
        <table className="w-full text-sm border-collapse">
          <thead>
            <tr className="border-b border-gray-200/50 dark:border-white/10">
              <th className={TH}>Need</th>
              <th className={TH}>状态</th>
              <th className={TH}>仓库</th>
              <th className={TH}>分支</th>
              <th className={TH_NUM}>实际周期</th>
              <th className={TH}>日历提效</th>
              <th className={TH}>人力提效</th>
            </tr>
          </thead>
          <tbody>
            {!needs.length ? (
              <tr>
                <td colSpan={7}>
                  <div className="py-6 text-center text-sm text-gray-400 dark:text-gray-500">暂无 Need</div>
                </td>
              </tr>
            ) : (
              needs.map((n) => (
                <tr key={n.need_id} className="border-b border-gray-100/50 dark:border-white/5">
                  <td className={TD}>
                    <button
                      type="button"
                      onClick={() => navigate(`/needs/${encodeURIComponent(n.need_id)}`)}
                      className="font-mono text-apple-blue hover:text-apple-blue-hover cursor-pointer bg-transparent border-none p-0 focus:outline-none focus-visible:underline"
                      title={n.need_id}
                    >
                      {shortId(n.need_id, 16)}
                    </button>
                  </td>
                  <td className={TD}><Tag tone={statusClass(n.status)}>{n.status || '-'}</Tag></td>
                  <td className={TD}><div className="max-w-[240px] truncate" title={n.repo_addr}>{n.repo_addr || '-'}</div></td>
                  <td className={TD}><div className="max-w-[160px] truncate" title={n.repo_branch}>{n.repo_branch || '-'}</div></td>
                  <td className={TD_NUM}>{formatDuration(n.total_calendar_min)}</td>
                  <td className={TD}><RatioPill value={n.efficiency_ratio} /></td>
                  <td className={TD}><RatioPill value={n.work_efficiency_ratio} /></td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </Panel>

      {/* 最近 Commit */}
      <Panel title="最近 Commit" hint={`${commits.length} 条`} bodyClass="overflow-x-auto">
        <table className="w-full text-sm border-collapse">
          <thead>
            <tr className="border-b border-gray-200/50 dark:border-white/10">
              <th className={TH}>Commit</th>
              <th className={TH}>提交时间</th>
              <th className={TH}>仓库</th>
              <th className={TH_NUM}>代码行</th>
              <th className={TH}>说明</th>
            </tr>
          </thead>
          <tbody>
            {!commits.length ? (
              <tr>
                <td colSpan={5}>
                  <div className="py-6 text-center text-sm text-gray-400 dark:text-gray-500">暂无 Commit</div>
                </td>
              </tr>
            ) : (
              commits.map((c) => (
                <tr key={c.commit_id} className="border-b border-gray-100/50 dark:border-white/5">
                  <td className={TD}>
                    <button
                      type="button"
                      onClick={() => navigate(`/commit/${encodeURIComponent(c.commit_id)}`)}
                      className="font-mono text-apple-blue hover:text-apple-blue-hover cursor-pointer bg-transparent border-none p-0 focus:outline-none focus-visible:underline"
                      title={c.commit_id}
                    >
                      {shortId(c.commit_id, 10)}
                    </button>
                  </td>
                  <td className={TD}>{formatLocalTime(c.commit_time)}</td>
                  <td className={TD}><div className="max-w-[240px] truncate" title={c.repo_addr as string}>{(c.repo_addr as string) || '-'}</div></td>
                  <td className={TD_NUM}>{formatNumber(c.diff_lines, 0)}</td>
                  <td className={TD}><div className="max-w-[280px] truncate" title={c.comment}>{c.comment || '-'}</div></td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </Panel>
    </div>
  )
}

function Panel({ title, hint, bodyClass = '', children }: { title: string; hint?: string; bodyClass?: string; children: ReactNode }) {
  return (
    <section className="glass rounded-2xl overflow-hidden">
      <div className="flex items-center justify-between px-5 py-3 border-b border-gray-200/50 dark:border-white/10">
        <span className="text-sm font-semibold text-gray-700 dark:text-gray-200">{title}</span>
        {hint && <span className="text-xs text-gray-400 dark:text-gray-500">{hint}</span>}
      </div>
      <div className={`p-5 ${bodyClass}`}>{children}</div>
    </section>
  )
}
