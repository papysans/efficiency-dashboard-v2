// Need 详情页（NeedDetailV2 的 React + 玻璃拟态迁移）。
// 9 分区 1:1 按 research/pr2-need-pages.md §3；视觉换玻璃拟态。
// 口径坑：fmtMin=formatNumber(分钟整数)、fmtPct 把 0 当「-」、fmtInt 只把 null 当「-」、
//        验证用 formatVerifyMin、need_id/commit_id 跳转 encodeURIComponent。
import { Fragment, useMemo, useState, type ReactNode } from 'react'
import { useNavigate, useParams } from 'react-router'
import { useNeedDetail } from '@/api/queries'
import { useTheme } from '@/hooks/useTheme'
import type { NeedBaselineComponents, NeedCommit, NeedDetail as NeedDetailModel, NeedSession } from '@/api/types'
import {
  formatDuration,
  formatLocalTime,
  formatNumber,
  formatV2Ratio,
  formatVerifyMin,
  STAGE_ESTIMATE_TIP,
  VERIFY_UNAVAILABLE_TIP,
} from '@/lib/formatters'
import { parseReason, reasonHints, reasonSummary } from '@/lib/reasonText'
import {
  ACTUAL_CALENDAR_TIP,
  ACTUAL_WORK_TIP,
  BASELINE_CALENDAR_TIP,
  CALENDAR_RATIO_TIP,
  FUSED_BASELINE_WORK_TIP,
  WORK_RATIO_TIP,
} from '@/lib/needMetricTips'
import { MetricCard } from '@/components/ui/MetricCard'
import { RatioPill } from '@/components/ui/RatioPill'
import { Tag, type TagTone } from '@/components/ui/Tag'
import { EChart } from '@/components/charts/EChart'
import { barOption } from '@/components/charts/barOption'

// accent 色（替代 Vue 的 --native-* 变量），对齐玻璃拟态语义色。
const ACCENT = {
  success: '#34d399',
  info: '#60a5fa',
  primary: '#0071e3',
  warning: '#fbbf24',
}

// ---- 纯函数（移植 NeedDetailV2）----

function asFileList(value: unknown): string[] {
  if (Array.isArray(value)) return value.filter(Boolean) as string[]
  if (typeof value === 'string') {
    const s = value.trim()
    if (!s || s === '[]' || s === 'null') return []
    try {
      const arr = JSON.parse(s)
      return Array.isArray(arr) ? arr.filter(Boolean) : []
    } catch {
      return []
    }
  }
  return []
}

function shortId(value?: string | null, size = 8): string {
  if (!value) return '-'
  return String(value).slice(0, size)
}

function statusTagClass(status?: string): TagTone {
  if (status === 'merged') return 'success'
  if (status === 'active') return 'primary'
  return 'neutral'
}

function confidenceTagClass(level?: string | null): TagTone {
  if (level === 'high') return 'success'
  if (level === 'medium') return 'warning'
  if (level === 'low') return 'info'
  if (level === 'very_low') return 'error'
  return 'neutral'
}

function signalTagClass(signal?: string): TagTone {
  const s = String(signal || '').toLowerCase()
  if (s === 'ok' || s === 'low') return 'success'
  if (s === 'medium' || s === 'warn' || s === 'warning') return 'warning'
  if (s === 'high' || s === 'risk' || s === 'bad') return 'error'
  return 'neutral'
}

function reasonTagClass(tone: string): TagTone {
  if (tone === 'error') return 'error'
  if (tone === 'warning') return 'warning'
  if (tone === 'info') return 'info'
  return 'neutral'
}

// 基线表用分钟整数（formatNumber），非 formatDuration
function fmtMin(value: number | null | undefined): string {
  if (value == null) return '-'
  return formatNumber(value, 0)
}
// fmtInt 只把 null 当「-」
function fmtInt(value: number | null | undefined): string {
  if (value == null) return '-'
  return formatNumber(value, 0)
}
// fmtPct 把 0 也当「-」
function fmtPct(value: number | null | undefined): string {
  if (value == null || value === 0) return '-'
  return formatV2Ratio(value)
}

const FILE_PREVIEW_N = 24

export default function NeedDetail() {
  const { needId } = useParams<{ needId: string }>()
  const navigate = useNavigate()
  const { theme } = useTheme()
  const { data, isLoading, error } = useNeedDetail(needId)

  const need: NeedDetailModel = (data?.need as NeedDetailModel) || ({ need_id: needId || '' } as NeedDetailModel)
  const sessions: NeedSession[] = data?.sessions || data?.stage_metrics || []
  const commits: NeedCommit[] = data?.commits || []
  const baseline: NeedBaselineComponents = data?.baseline_components || {}
  const qualityReason = (data?.quality_signals?.reason as string) || ''

  const [needFilesExpanded, setNeedFilesExpanded] = useState(false)
  const [expandedCommits, setExpandedCommits] = useState<Set<string>>(new Set())

  const reasonItems = useMemo(() => parseReason(need.reason), [need.reason])
  const needFiles = useMemo(() => asFileList(need.touched_files), [need.touched_files])
  const visibleNeedFiles = needFilesExpanded ? needFiles : needFiles.slice(0, FILE_PREVIEW_N)

  const contributorCount = Array.isArray(need.contributor_user_ids) ? need.contributor_user_ids.length : '-'

  // 详情版 band（带「区间」前缀，空时 ''）
  const bandHint = useMemo(() => {
    if (need.efficiency_band_low == null && need.efficiency_band_high == null) return ''
    return `区间 ${formatV2Ratio(need.efficiency_band_low)} ~ ${formatV2Ratio(need.efficiency_band_high)}`
  }, [need.efficiency_band_low, need.efficiency_band_high])

  const baselineRows = useMemo(
    () => [
      { name: '算法基线', think: baseline.algo_think_min, exec: baseline.algo_exec_min, verify: baseline.algo_verify_min, total: baseline.algo_total_min, reason: '' },
      { name: '相似锚点 kNN', think: null, exec: null, verify: null, total: baseline.anchor_knn_min, reason: baseline.anchor_knn_reason || '' },
      { name: 'LLM 估算', think: baseline.llm_think_min, exec: baseline.llm_exec_min, verify: baseline.llm_verify_min, total: baseline.llm_total_min, reason: baseline.llm_reason || baseline.llm_confidence || '' },
      { name: '融合工作量', think: null, exec: null, verify: null, total: baseline.fused_work_min, reason: '' },
      { name: '离散工作量', think: null, exec: null, verify: null, total: baseline.spread_work_min, reason: '' },
      { name: '日历基线', think: null, exec: null, verify: null, total: baseline.calendar_min, reason: baseline.team_work_density == null ? '' : `团队工作密度 ${baseline.team_work_density}` },
    ],
    [baseline],
  )

  const baselineChartOption = useMemo(() => {
    const items: Array<[string, number | null | undefined]> = [
      ['实际工作量', need.total_active_work_corrected_min],
      ['算法', baseline.algo_total_min],
      ['锚点 kNN', baseline.anchor_knn_min],
      ['LLM', baseline.llm_total_min],
      ['融合', baseline.fused_work_min],
    ]
    const seriesData = items.map(([, v]) => Number(v || 0))
    if (seriesData.every((v) => v === 0)) return null
    return barOption(
      theme,
      '工作量基线对比',
      items.map(([k]) => k),
      [{ name: '分钟', data: seriesData }],
      { titleSize: 14, format: (v) => formatDuration(v) },
    )
  }, [need.total_active_work_corrected_min, baseline, theme])

  const stageChartOption = useMemo(() => {
    const seriesData = [need.total_think_min, need.total_exec_min, need.total_verify_min, need.total_other_min].map((v) => Number(v || 0))
    if (seriesData.every((v) => v === 0)) return null
    return barOption(theme, '阶段工作量分布', ['思考', '执行', '验证', '其他'], [{ name: '活跃工作量（分钟）', data: seriesData }], {
      titleSize: 14,
      format: (v) => formatDuration(v),
    })
  }, [need.total_think_min, need.total_exec_min, need.total_verify_min, need.total_other_min, theme])

  function toggleCommitFiles(id: string) {
    setExpandedCommits((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  if (error) {
    return (
      <div className="glass rounded-2xl p-8 text-center text-sm text-rose-600 dark:text-rose-400">
        {(error as Error).message || '获取 Need 详情失败'}
      </div>
    )
  }

  return (
    <div className={`space-y-5 ${isLoading ? 'opacity-60 pointer-events-none' : ''}`}>
      {/* ① Header */}
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
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="min-w-0">
            <h1 className="text-2xl font-bold text-gray-900 dark:text-white">需求 Need 详情</h1>
            <p className="text-sm text-gray-500 dark:text-gray-400 mt-1 font-mono break-all">{need.need_id || '-'}</p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            {need.status && <Tag tone={statusTagClass(need.status)}>{need.status}</Tag>}
            {need.confidence_level && <Tag tone={confidenceTagClass(need.confidence_level)}>效率置信 {need.confidence_level}</Tag>}
            <Tag tone={need.coverage_eligible ? 'success' : 'neutral'}>{need.coverage_eligible ? '可计入' : '未计入'}</Tag>
            {need.outlier_flag && <Tag tone="error">异常样本</Tag>}
          </div>
        </div>
      </header>

      {/* ② 提效指标卡 */}
      <section className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-3">
        <MetricCard label="日历提效" accent={ACCENT.success} hint={bandHint || undefined} tip={CALENDAR_RATIO_TIP} value={<RatioPill value={need.efficiency_ratio} />} />
        <MetricCard label="工作量提效" accent={ACCENT.info} tip={WORK_RATIO_TIP} value={<RatioPill value={need.work_efficiency_ratio} />} />
        <MetricCard label="实际日历" accent={ACCENT.primary} tip={ACTUAL_CALENDAR_TIP} value={formatDuration(need.total_calendar_min)} />
        <MetricCard label="基线日历" accent={ACCENT.primary} tip={BASELINE_CALENDAR_TIP} value={formatDuration(need.baseline_calendar_min)} />
        <MetricCard label="实际工作量" accent={ACCENT.warning} tip={ACTUAL_WORK_TIP} value={formatDuration(need.total_active_work_corrected_min)} />
        <MetricCard label="融合基线工作量" accent={ACCENT.warning} tip={FUSED_BASELINE_WORK_TIP} value={formatDuration(need.baseline_fused_work_min)} />
      </section>

      {/* ③ 基础信息 */}
      <Panel title="基础信息">
        <KvGrid>
          <Kv label="边界来源">{need.boundary_source || '-'}</Kv>
          <Kv label="边界置信"><Tag tone={confidenceTagClass(need.boundary_confidence)}>{need.boundary_confidence || '-'}</Tag></Kv>
          <Kv label="边界标识" wide mono>{need.boundary_key || '-'}</Kv>
          <Kv label="仓库" wide mono>{need.repo_addr || '-'}</Kv>
          <Kv label="分支" mono>{need.repo_branch || '-'}</Kv>
          <Kv label="主用户">{need.primary_user_id || '-'}</Kv>
          <Kv label="协作人数">{contributorCount}</Kv>
          <Kv label="开始时间">{formatLocalTime(need.dev_start_ts)}</Kv>
          <Kv label="结束时间">{formatLocalTime(need.dev_end_ts)}</Kv>
          <Kv label="合并时间">{formatLocalTime(need.merge_ts)}</Kv>
          <Kv label="开发跨度">{formatDuration(need.dev_duration_min)}</Kv>
          <Kv label="等待评审">{formatDuration(need.wait_for_review_min)}</Kv>
          <Kv label="团队画像">{need.team_profile_used || '-'}</Kv>
          <Kv label="说明" wide>
            {!reasonItems.length ? (
              <span className="text-gray-400">-</span>
            ) : (
              <span className="flex flex-wrap gap-1.5">
                {reasonItems.map((r, i) => (
                  <Tag key={i} tone={reasonTagClass(r.tone)} title={r.hint}>
                    {r.label}
                  </Tag>
                ))}
              </span>
            )}
          </Kv>
        </KvGrid>
      </Panel>

      {/* ④ 基线分解 + 图表 */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <Panel title="基线组成（工作量，分钟）" bodyClass="overflow-x-auto">
          <table className="w-full text-sm border-collapse">
            <thead>
              <tr className="border-b border-gray-200/50 dark:border-white/10">
                <th className="px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400">来源</th>
                <th className="px-3 py-2 text-right font-semibold text-gray-500 dark:text-gray-400">思考</th>
                <th className="px-3 py-2 text-right font-semibold text-gray-500 dark:text-gray-400">执行</th>
                <th className="px-3 py-2 text-right font-semibold text-gray-500 dark:text-gray-400">验证</th>
                <th className="px-3 py-2 text-right font-semibold text-gray-500 dark:text-gray-400">合计</th>
                <th className="px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400">说明</th>
              </tr>
            </thead>
            <tbody>
              {baselineRows.map((r) => (
                <tr key={r.name} className="border-b border-gray-100/50 dark:border-white/5">
                  <td className="px-3 py-2 text-gray-700 dark:text-gray-200">{r.name}</td>
                  <td className="px-3 py-2 text-right tabular-nums text-gray-700 dark:text-gray-200">{fmtMin(r.think)}</td>
                  <td className="px-3 py-2 text-right tabular-nums text-gray-700 dark:text-gray-200">{fmtMin(r.exec)}</td>
                  <td className="px-3 py-2 text-right tabular-nums text-gray-700 dark:text-gray-200">{fmtMin(r.verify)}</td>
                  <td className="px-3 py-2 text-right tabular-nums text-gray-700 dark:text-gray-200">{fmtMin(r.total)}</td>
                  <td className="px-3 py-2 text-gray-700 dark:text-gray-200">
                    <div className="max-w-[200px] truncate" title={reasonHints(r.reason)}>{r.reason ? reasonSummary(r.reason) : '-'}</div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </Panel>
        <ChartPanel option={baselineChartOption} empty="暂无基线数据" />
      </div>

      {/* ⑤ 阶段工作量 + 图表 */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <Panel title="阶段工作量">
          <KvGrid>
            <Kv label="思考" title={STAGE_ESTIMATE_TIP}>{formatDuration(need.total_think_min)}</Kv>
            <Kv label="执行" title={STAGE_ESTIMATE_TIP}>{formatDuration(need.total_exec_min)}</Kv>
            <Kv label="验证" title={VERIFY_UNAVAILABLE_TIP}>{formatVerifyMin(need.total_verify_min)}</Kv>
            <Kv label="其他">{formatDuration(need.total_other_min)}</Kv>
            <Kv label="会话活跃人工">{formatDuration(need.total_session_active_person_min)}</Kv>
            <Kv label="未覆盖人工估算">{formatDuration(need.estimate_uncovered_human_min)}</Kv>
          </KvGrid>
          <p className="mt-3 text-xs text-gray-400 dark:text-gray-500">验证：采集未覆盖（{VERIFY_UNAVAILABLE_TIP}）。思考 / 执行为粗略估算口径。</p>
        </Panel>
        <div className="flex flex-col gap-2">
          <ChartPanel option={stageChartOption} empty="暂无阶段数据" />
          <p className="text-xs text-gray-400 dark:text-gray-500 px-1">验证：采集未覆盖，图中“验证 0”非真实结论。</p>
        </div>
      </div>

      {/* ⑥ 代码与质量信号 */}
      <Panel
        title="代码与质量信号"
        hint={qualityReason ? reasonSummary(qualityReason) : undefined}
        hintTitle={qualityReason ? reasonHints(qualityReason) : undefined}
      >
        <KvGrid>
          <Kv label="净代码行">{fmtInt(need.total_loc_net)}</Kv>
          <Kv label="改动文件">{fmtInt(need.total_files_touched)}</Kv>
          <Kv label="提交数">{fmtInt(need.commit_count)}</Kv>
          <Kv label="AI 代码占比">{fmtPct(need.ai_code_ratio)}</Kv>
          <Kv label="AI 覆盖行">{fmtInt(need.ai_covered_loc)}</Kv>
          <Kv label="未覆盖行">{fmtInt(need.uncovered_loc)}</Kv>
          <Kv label="未覆盖工作占比">{fmtPct(need.uncovered_work_ratio)}</Kv>
          <Kv label="硅含量">{fmtPct(need.silica)}</Kv>
          <Kv label="Churn 比">{fmtPct(need.churn_ratio)}</Kv>
          <Kv label="重复率">{fmtPct(need.duplication_ratio)}</Kv>
          <Kv label="回退次数 / 回退率">{fmtInt(need.revert_count)} / {fmtPct(need.revert_rate)}</Kv>
          <Kv label="生成后删除率">{fmtPct(need.post_generation_deletion_ratio)}</Kv>
        </KvGrid>
        <div className="flex flex-wrap gap-1.5 mt-4">
          <Tag tone={signalTagClass(need.feature_dependency_risk)}>特性依赖风险: {need.feature_dependency_risk || '未知'}</Tag>
          <Tag tone={signalTagClass(need.silica_signal)}>硅含量信号: {need.silica_signal || '未知'}</Tag>
          <Tag tone={signalTagClass(need.ai_code_ratio_signal)}>AI 占比信号: {need.ai_code_ratio_signal || '未知'}</Tag>
          <Tag tone={signalTagClass(need.uncovered_work_signal)}>未覆盖工作信号: {need.uncovered_work_signal || '未知'}</Tag>
        </div>
      </Panel>

      {/* ⑦ 改动文件 */}
      <Panel title="改动文件" hint={`${needFiles.length} 个`}>
        {!needFiles.length ? (
          <div className="py-6 text-center text-sm text-gray-400 dark:text-gray-500">暂无改动文件</div>
        ) : (
          <>
            <div className="flex flex-wrap gap-1.5">
              {visibleNeedFiles.map((f) => (
                <Tag key={f} tone="neutral" mono title={f}>
                  {f}
                </Tag>
              ))}
            </div>
            {needFiles.length > FILE_PREVIEW_N && (
              <button
                type="button"
                onClick={() => setNeedFilesExpanded((e) => !e)}
                className="mt-2 text-sm text-apple-blue hover:text-apple-blue-hover cursor-pointer bg-transparent border-none p-0 focus:outline-none focus-visible:underline"
              >
                {needFilesExpanded ? '收起' : `展开全部（${needFiles.length}）`}
              </button>
            )}
          </>
        )}
      </Panel>

      {/* ⑧ 关联 Sessions */}
      <Panel title="关联 Sessions" hint={`${sessions.length} 个`} bodyClass="overflow-x-auto">
        <table className="w-full text-sm border-collapse">
          <thead>
            <tr className="border-b border-gray-200/50 dark:border-white/10 text-left text-gray-500 dark:text-gray-400">
              <th className="px-3 py-2 font-semibold">Session</th>
              <th className="px-3 py-2 font-semibold">用户</th>
              <th className="px-3 py-2 font-semibold">开始</th>
              <th className="px-3 py-2 font-semibold">结束</th>
              <th className="px-3 py-2 font-semibold text-right">活跃工作量</th>
              <th className="px-3 py-2 font-semibold text-right" title={STAGE_ESTIMATE_TIP}>思考</th>
              <th className="px-3 py-2 font-semibold text-right" title={STAGE_ESTIMATE_TIP}>执行</th>
              <th className="px-3 py-2 font-semibold text-right">
                <span className="inline-flex items-center gap-1 justify-end">验证 <InfoMark tip={VERIFY_UNAVAILABLE_TIP} /></span>
              </th>
              <th className="px-3 py-2 font-semibold">阶段置信</th>
              <th className="px-3 py-2 font-semibold">摘要</th>
            </tr>
          </thead>
          <tbody>
            {!sessions.length ? (
              <tr>
                <td colSpan={10}>
                  <div className="py-6 text-center text-sm text-gray-400 dark:text-gray-500">暂无 Session</div>
                </td>
              </tr>
            ) : (
              sessions.map((s) => (
                <tr key={s.session_id} className="border-b border-gray-100/50 dark:border-white/5">
                  <td className="px-3 py-2 font-mono text-gray-700 dark:text-gray-200">{shortId(s.session_id)}</td>
                  <td className="px-3 py-2 text-gray-700 dark:text-gray-200"><div className="max-w-[220px] truncate" title={s.user_id}>{s.user_id || '-'}</div></td>
                  <td className="px-3 py-2 text-gray-700 dark:text-gray-200">{formatLocalTime(s.session_start_ts)}</td>
                  <td className="px-3 py-2 text-gray-700 dark:text-gray-200">{formatLocalTime(s.session_end_ts)}</td>
                  <td className="px-3 py-2 text-right tabular-nums text-gray-700 dark:text-gray-200">{formatDuration(s.total_active_min)}</td>
                  <td className="px-3 py-2 text-right tabular-nums text-gray-700 dark:text-gray-200">{formatDuration(s.think_active_min)}</td>
                  <td className="px-3 py-2 text-right tabular-nums text-gray-700 dark:text-gray-200">{formatDuration(s.exec_active_min)}</td>
                  <td className="px-3 py-2 text-right tabular-nums text-gray-700 dark:text-gray-200" title={VERIFY_UNAVAILABLE_TIP}>{formatVerifyMin(s.verify_active_min)}</td>
                  <td className="px-3 py-2"><Tag tone={confidenceTagClass(s.stage_confidence)}>{s.stage_confidence || '-'}</Tag></td>
                  <td className="px-3 py-2 text-gray-700 dark:text-gray-200"><div className="max-w-[280px] truncate" title={s.summary}>{s.summary || '-'}</div></td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </Panel>

      {/* ⑨ 关联 Commits */}
      <Panel title="关联 Commits" hint={`${commits.length} 个`} bodyClass="overflow-x-auto">
        <table className="w-full text-sm border-collapse">
          <thead>
            <tr className="border-b border-gray-200/50 dark:border-white/10 text-left text-gray-500 dark:text-gray-400">
              <th className="px-3 py-2 font-semibold">Commit</th>
              <th className="px-3 py-2 font-semibold">提交时间</th>
              <th className="px-3 py-2 font-semibold">用户</th>
              <th className="px-3 py-2 font-semibold text-right">代码行</th>
              <th className="px-3 py-2 font-semibold text-right">硅含量</th>
              <th className="px-3 py-2 font-semibold">提交说明</th>
              <th className="px-3 py-2 font-semibold">改动文件</th>
            </tr>
          </thead>
          <tbody>
            {!commits.length ? (
              <tr>
                <td colSpan={7}>
                  <div className="py-6 text-center text-sm text-gray-400 dark:text-gray-500">暂无 Commit</div>
                </td>
              </tr>
            ) : (
              commits.map((c) => {
                const files = asFileList(c.touched_files)
                const expanded = expandedCommits.has(c.commit_id)
                return (
                  <Fragment key={c.commit_id}>
                    <tr className="border-b border-gray-100/50 dark:border-white/5">
                      <td className="px-3 py-2">
                        <button
                          type="button"
                          onClick={() => navigate(`/commit/${encodeURIComponent(c.commit_id)}`)}
                          className="font-mono text-apple-blue hover:text-apple-blue-hover cursor-pointer bg-transparent border-none p-0 focus:outline-none focus-visible:underline"
                        >
                          {shortId(c.commit_id, 10)}
                        </button>
                      </td>
                      <td className="px-3 py-2 text-gray-700 dark:text-gray-200">{formatLocalTime(c.commit_time)}</td>
                      <td className="px-3 py-2 text-gray-700 dark:text-gray-200"><div className="max-w-[180px] truncate" title={c.user_name}>{c.user_name || '-'}</div></td>
                      <td className="px-3 py-2 text-right tabular-nums text-gray-700 dark:text-gray-200">{fmtInt(c.diff_lines)}</td>
                      <td className="px-3 py-2 text-right tabular-nums text-gray-700 dark:text-gray-200">{fmtPct(c.silica)}</td>
                      <td className="px-3 py-2 text-gray-700 dark:text-gray-200"><div className="max-w-[280px] truncate" title={c.comment}>{c.comment || '-'}</div></td>
                      <td className="px-3 py-2">
                        {files.length ? (
                          <button
                            type="button"
                            onClick={() => toggleCommitFiles(c.commit_id)}
                            className="text-apple-blue hover:text-apple-blue-hover cursor-pointer bg-transparent border-none p-0 focus:outline-none focus-visible:underline"
                          >
                            {expanded ? '收起' : `${files.length} 个文件`}
                          </button>
                        ) : (
                          <span className="text-gray-400">-</span>
                        )}
                      </td>
                    </tr>
                    {files.length > 0 && expanded && (
                      <tr className="border-b border-gray-100/50 dark:border-white/5">
                        <td colSpan={7} className="px-3 py-2">
                          <div className="flex flex-wrap gap-1.5">
                            {files.map((f) => (
                              <Tag key={f} tone="neutral" mono title={f}>
                                {f}
                              </Tag>
                            ))}
                          </div>
                        </td>
                      </tr>
                    )}
                  </Fragment>
                )
              })
            )}
          </tbody>
        </table>
      </Panel>
    </div>
  )
}

// ---- 玻璃布局子组件 ----

function Panel({
  title,
  hint,
  hintTitle,
  bodyClass = '',
  children,
}: {
  title: string
  hint?: string
  hintTitle?: string
  bodyClass?: string
  children: ReactNode
}) {
  return (
    <section className="glass rounded-2xl overflow-hidden">
      <div className="flex items-center justify-between px-5 py-3 border-b border-gray-200/50 dark:border-white/10">
        <span className="text-sm font-semibold text-gray-700 dark:text-gray-200">{title}</span>
        {hint && (
          <span className="text-xs text-gray-400 dark:text-gray-500 truncate max-w-[50%]" title={hintTitle}>
            {hint}
          </span>
        )}
      </div>
      <div className={`p-5 ${bodyClass}`}>{children}</div>
    </section>
  )
}

function KvGrid({ children }: { children: ReactNode }) {
  return <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-x-6 gap-y-3">{children}</div>
}

function Kv({ label, children, wide = false, mono = false, title }: { label: string; children: ReactNode; wide?: boolean; mono?: boolean; title?: string }) {
  return (
    <div className={`flex flex-col gap-0.5 ${wide ? 'col-span-2 md:col-span-3 lg:col-span-4' : ''}`}>
      <span className="text-xs text-gray-400 dark:text-gray-500">{label}</span>
      <span className={`text-sm text-gray-800 dark:text-gray-100 break-words ${mono ? 'font-mono' : ''}`} title={title}>
        {children}
      </span>
    </div>
  )
}

/** 图表玻璃卡：option 为 null 时显示空态。 */
function ChartPanel({ option, empty }: { option: ReturnType<typeof barOption> | null; empty: string }) {
  return (
    <div className="glass rounded-2xl p-4 min-h-[280px] flex items-center justify-center">
      {option ? (
        <EChart option={option} height={280} className="w-full" />
      ) : (
        <div className="text-sm text-gray-400 dark:text-gray-500">{empty}</div>
      )}
    </div>
  )
}

function InfoMark({ tip }: { tip: string }) {
  return (
    <span className="text-gray-400 cursor-help inline-flex align-middle" title={tip} aria-label={tip}>
      <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
      </svg>
    </span>
  )
}
