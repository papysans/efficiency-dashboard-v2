// Commit 详情页（CommitDetailV2 的 React + 玻璃拟态迁移）。
// 分区 1:1 按 research/pr4-project-commit-workdir.md §1.2；视觉换玻璃拟态。
//
// ⚠️ efficiency_ratio 百分比口径：详情大数字 Math.round()+'%'，不 ×100，用 percentTextClass 着色。
// manual 优先：度量区有 manual 时显示 manual 值 + 黄(?)理由 + 删除线原 AI 值 + 灰(?)AI 理由。
// silica 字段在前端展示为 AI 代码占比；输入为 0~1 小数口径。
import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { useNavigate, useParams } from 'react-router'
import { useQueryClient } from '@tanstack/react-query'
import { useCommitDetail } from '@/api/queries'
import { updateCommitManualV2 } from '@/api/endpoints'
import type { CommitDetail as CommitDetailType, RelatedTask, UpdateCommitManualRequest } from '@/api/types'
import { useUserNameMap } from '@/hooks/useUserNameMap'
import { fmtCost, formatDuration, formatLocalTime, formatV2Ratio } from '@/lib/formatters'
import { Tag } from '@/components/ui/Tag'
import { percentTextClass } from '@/components/ui/PercentPill'
import { Modal } from '@/components/ui/Modal'

/** 关联 Task AI 代码占比 tag tone（0~1 小数口径）。 */
function relatedSilicaTone(v: number): 'success' | 'primary' | 'info' {
  if (v >= 0.8) return 'success'
  if (v >= 0.5) return 'primary'
  return 'info'
}

export default function CommitDetail() {
  const { commitId } = useParams<{ commitId: string }>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { data, isLoading, error } = useCommitDetail(commitId)
  // user_name 可能是 UUID，统一走姓名映射（与列表页同口径）。
  const { resolveName } = useUserNameMap()

  // 顶层 efficiency_ratio 覆盖进 commit（§1.2，顶层优先）。
  const commit: CommitDetailType = useMemo(() => {
    const c = (data?.commit || { commit_id: commitId || '' }) as CommitDetailType
    if (data?.efficiency_ratio != null) return { ...c, efficiency_ratio: data.efficiency_ratio }
    return c
  }, [data, commitId])

  const relatedTasks: RelatedTask[] = data?.related_tasks || []
  const totalCost = data?.total_cost ?? 0
  const upstream = data?.upstream_tokens ?? 0
  const downstream = data?.downstream_tokens ?? 0
  const totalTokens = upstream + downstream
  const silica = data?.silica ?? commit.silica
  const ratio = commit.efficiency_ratio

  const [modalOpen, setModalOpen] = useState(false)

  // realMinutesExplain（§1.2）：有 reason 用之；否则若有关联 task → Σ(real × AI 代码占比)；无 task → 无关联 Task。
  const realMinutesExplain = useMemo(() => {
    if (commit.commit_real_minutes_reason) return commit.commit_real_minutes_reason
    if (relatedTasks.length > 0) {
      const parts = relatedTasks.map(
        (t) => `${formatDuration(t.task_real_minutes)} × ${((t.silica ?? 0) * 100).toFixed(0)}%`,
      )
      return `计算方式：Σ(Task实际耗时 × AI 代码占比)\n${parts.join(' + ')}`
    }
    return '无关联 Task'
  }, [commit.commit_real_minutes_reason, relatedTasks])

  async function submitManual(body: UpdateCommitManualRequest) {
    await updateCommitManualV2(commitId as string, body)
    setModalOpen(false)
    await queryClient.invalidateQueries({ queryKey: ['commit-detail', commitId] })
  }

  if (error) {
    return (
      <div className="glass rounded-2xl p-8 text-center text-sm text-rose-600 dark:text-rose-400">
        {(error as Error).message || '获取 Commit 详情失败'}
      </div>
    )
  }

  return (
    <div className={`space-y-5 ${isLoading ? 'opacity-60 pointer-events-none' : ''}`}>
      {/* ① 标题栏 */}
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-3">
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
          <h1 className="text-xl font-bold text-gray-900 dark:text-white">Commit 详情</h1>
        </div>
        <button
          type="button"
          onClick={() => setModalOpen(true)}
          className="inline-flex items-center gap-1.5 bg-amber-500 hover:bg-amber-600 text-white rounded-lg px-3 py-1.5 text-sm font-medium cursor-pointer transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-amber-400"
        >
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
          </svg>
          人工调整
        </button>
      </header>

      {/* ② 基础信息 */}
      <Panel title="基础信息">
        <KvGrid>
          <Kv label="Commit ID" mono>{commit.commit_id || '-'}</Kv>
          <Kv label="用户">
            {commit.user_id ? (
              <LinkBtn onClick={() => navigate(`/user/${encodeURIComponent(commit.user_id as string)}`)}>
                {resolveName(commit.user_id)}
              </LinkBtn>
            ) : (
              (commit.user_name && resolveName(commit.user_name)) || '-'
            )}
          </Kv>
          <Kv label="Git 用户">
            {commit.git_user_name ? `${commit.git_user_name}${commit.git_user_email ? ` <${commit.git_user_email}>` : ''}` : '-'}
          </Kv>
          <Kv label="仓库">
            {commit.repo_addr ? (
              <LinkBtn
                onClick={() =>
                  navigate(
                    `/repo/${encodeURIComponent(commit.repo_addr as string)}/${encodeURIComponent(commit.repo_branch || 'main')}`,
                  )
                }
              >
                {commit.repo_addr}#{commit.repo_branch || ''}
              </LinkBtn>
            ) : (
              '-'
            )}
          </Kv>
          <Kv label="分支">{commit.repo_branch || '-'}</Kv>
          <Kv label="提交时间">{formatLocalTime(commit.commit_time)}</Kv>
          <Kv label="提交说明" wide>{commit.comment || '-'}</Kv>
        </KvGrid>
      </Panel>

      {/* ③ 度量信息 */}
      <Panel title="度量信息">
        <KvGrid>
          <Kv label="生成代码量">{commit.diff_lines ?? '-'} 行</Kv>
          <Kv label="实际耗时">
            <ManualValue
              manual={commit.commit_real_minutes_manual}
              manualReason={commit.commit_real_minutes_reason_manual}
              original={commit.commit_real_minutes}
              originalReason={realMinutesExplain}
            />
          </Kv>
          <Kv label="传统开发时长预估">
            <ManualValue
              manual={commit.commit_ancient_minutes_manual}
              manualReason={commit.commit_ancient_minutes_reason_manual}
              original={commit.commit_ancient_minutes}
              originalReason={commit.commit_ancient_minutes_reason}
            />
          </Kv>
          <Kv label="提效比例">
            <span className={`text-xl font-bold tabular-nums ${percentTextClass(ratio)}`}>
              {ratio != null ? `${Math.round(ratio)}%` : '-'}
            </span>
          </Kv>
          <Kv
            label="AI 代码占比"
            title="commit 中由 AI Task 生成的代码占比，基于关联 Task 的 diff 行数加权计算"
          >
            {silica != null ? (
              <span className="text-base font-bold text-emerald-600 dark:text-emerald-400 tabular-nums">{formatV2Ratio(silica)}</span>
            ) : (
              <span className="text-gray-400 dark:text-gray-500">-</span>
            )}
          </Kv>
          <Kv label="总Tokens" title={`上行 ${upstream} / 下行 ${downstream}`}>
            {totalTokens > 0 ? totalTokens.toLocaleString() : '-'}
          </Kv>
          <Kv label="费用">{totalCost > 0 ? `${fmtCost(totalCost)} 元` : '-'}</Kv>
        </KvGrid>
      </Panel>

      {/* ④ 关联 Tasks */}
      <Panel title="关联 Tasks" hint={`${relatedTasks.length} 个`}>
        {relatedTasks.length === 0 ? (
          <div className="py-6 text-center text-sm text-gray-400 dark:text-gray-500">暂无关联 Task</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm border-collapse">
              <thead>
                <tr className="border-b border-gray-200/50 dark:border-white/10">
                  <th className={TH}>Task ID</th>
                  <th className={TH}>用户</th>
                  <th className={TH}>开始时间</th>
                  <th className={TH_NUM}>代码行数</th>
                  <th className={TH_NUM}>实际耗时</th>
                  <th className={TH_CENTER}>AI 代码占比</th>
                  <th className={TH_NUM}>费用</th>
                </tr>
              </thead>
              <tbody>
                {relatedTasks.map((t) => (
                  <tr key={t.task_id} className="border-b border-gray-100/50 dark:border-white/5 hover:bg-apple-blue/5 dark:hover:bg-white/5 transition-colors">
                    <td className={TD}>
                      <button
                        type="button"
                        className="font-mono text-apple-blue hover:text-apple-blue-hover cursor-pointer bg-transparent border-none p-0 max-w-[200px] truncate inline-block align-bottom focus:outline-none focus-visible:underline"
                        title={t.task_id}
                        onClick={() => navigate(`/task/${encodeURIComponent(t.task_id)}`)}
                      >
                        {t.task_id}
                      </button>
                    </td>
                    <td className={TD}>{t.user_name ? resolveName(t.user_name) : '-'}</td>
                    <td className={TD}>{formatLocalTime(t.start_time)}</td>
                    <td className={TD_NUM}>{t.diff_lines ?? '-'}</td>
                    <td className={TD_NUM}>{formatDuration(t.task_real_minutes)}</td>
                    <td className="px-3 py-2 align-middle text-center">
                      {t.silica != null ? (
                        <Tag tone={relatedSilicaTone(t.silica)}>{formatV2Ratio(t.silica)}</Tag>
                      ) : (
                        '-'
                      )}
                    </td>
                    <td className={TD_NUM}>{t.cost != null && t.cost > 0 ? t.cost.toFixed(2) : '-'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Panel>

      {/* 人工调整 Modal */}
      <ManualModal open={modalOpen} commit={commit} onClose={() => setModalOpen(false)} onSubmit={submitManual} />
    </div>
  )
}

// ---- manual 优先显示（实际耗时 / 传统预估 通用，§1.2）----
function ManualValue({
  manual,
  manualReason,
  original,
  originalReason,
}: {
  manual?: number | null
  manualReason?: string
  original?: number | null
  originalReason?: string
}) {
  if (manual != null) {
    return (
      <span className="inline-flex items-center gap-1.5 flex-wrap">
        <span>{formatDuration(manual)}</span>
        {manualReason && <ReasonMark reason={manualReason} tone="warning" />}
        <span className="line-through text-gray-400 dark:text-gray-500">
          {original != null ? formatDuration(original) : '(AI未出值)'}
        </span>
        {originalReason && <ReasonMark reason={originalReason} tone="muted" />}
      </span>
    )
  }
  return (
    <span className="inline-flex items-center gap-1.5">
      <span>{formatDuration(original)}</span>
      {originalReason && <ReasonMark reason={originalReason} tone="muted" />}
    </span>
  )
}

/** (?) 图标 + reason tooltip（warning 黄 / muted 灰）。 */
function ReasonMark({ reason, tone }: { reason: string; tone: 'warning' | 'muted' }) {
  const color = tone === 'warning' ? 'text-amber-500' : 'text-gray-400'
  return (
    <span className={`${color} cursor-help inline-flex align-middle`} title={reason} aria-label={reason}>
      <svg className="w-3.5 h-3.5" fill="currentColor" viewBox="0 0 24 24" aria-hidden="true">
        <path d="M12 2a10 10 0 100 20 10 10 0 000-20zm0 15a1 1 0 110-2 1 1 0 010 2zm1.07-7.75l-.9.92c-.5.51-.67.95-.67 1.83h-2v-.5c0-.66.27-1.26.67-1.67l1.24-1.26c.37-.36.59-.86.59-1.41a2 2 0 10-4 0H6a4 4 0 118 0c0 .73-.3 1.4-.83 1.99z" />
      </svg>
    </span>
  )
}

// ---- 人工调整对话框（§1.2）----
function ManualModal({
  open,
  commit,
  onClose,
  onSubmit,
}: {
  open: boolean
  commit: CommitDetailType
  onClose: () => void
  onSubmit: (body: UpdateCommitManualRequest) => Promise<void>
}) {
  const [ancient, setAncient] = useState('')
  const [ancientReason, setAncientReason] = useState('')
  const [real, setReal] = useState('')
  const [realReason, setRealReason] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [err, setErr] = useState('')

  // 打开时预填：manual 优先，否则裸值（§1.2）。
  useEffect(() => {
    if (!open) return
    const a = commit.commit_ancient_minutes_manual ?? commit.commit_ancient_minutes ?? null
    const r = commit.commit_real_minutes_manual ?? commit.commit_real_minutes ?? null
    setAncient(a == null ? '' : String(a))
    setAncientReason(commit.commit_ancient_minutes_reason_manual || '')
    setReal(r == null ? '' : String(r))
    setRealReason(commit.commit_real_minutes_reason_manual || '')
    setErr('')
  }, [open, commit])

  async function handleSubmit() {
    setSubmitting(true)
    setErr('')
    try {
      await onSubmit({
        commit_ancient_minutes_manual: ancient === '' ? null : Number(ancient),
        commit_ancient_minutes_reason_manual: ancientReason,
        commit_real_minutes_manual: real === '' ? null : Number(real),
        commit_real_minutes_reason_manual: realReason,
      })
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : '保存失败')
    } finally {
      setSubmitting(false)
    }
  }

  const inputCls =
    'glass rounded-lg px-3 py-1.5 text-sm w-full bg-transparent text-gray-900 dark:text-white ' +
    'focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue'

  return (
    <Modal
      open={open}
      title="人工调整"
      onClose={onClose}
      footer={
        <>
          <button
            type="button"
            onClick={onClose}
            className="glass rounded-lg px-4 py-1.5 text-sm text-gray-700 dark:text-gray-200 cursor-pointer hover:text-apple-blue transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue"
          >
            取消
          </button>
          <button
            type="button"
            onClick={handleSubmit}
            disabled={submitting}
            className="bg-apple-blue hover:bg-apple-blue-hover text-white rounded-lg px-4 py-1.5 text-sm font-medium cursor-pointer transition-colors disabled:opacity-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue"
          >
            {submitting ? '保存中...' : '保存'}
          </button>
        </>
      }
    >
      <div className="space-y-3">
        {err && <div className="text-sm text-rose-600 dark:text-rose-400">{err}</div>}
        <Field label="传统开发时长预估（分钟）">
          <input type="number" step={10} value={ancient} onChange={(e) => setAncient(e.target.value)} className={inputCls} />
        </Field>
        <Field label="传统开发时长预估理由">
          <textarea rows={2} value={ancientReason} onChange={(e) => setAncientReason(e.target.value)} className={`${inputCls} resize-y`} />
        </Field>
        <Field label="实际耗时（分钟）">
          <input type="number" step={10} value={real} onChange={(e) => setReal(e.target.value)} className={inputCls} />
        </Field>
        <Field label="实际耗时理由">
          <textarea rows={2} value={realReason} onChange={(e) => setRealReason(e.target.value)} className={`${inputCls} resize-y`} />
        </Field>
      </div>
    </Modal>
  )
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="block">
      <span className="block text-xs text-gray-500 dark:text-gray-400 mb-1">{label}</span>
      {children}
    </label>
  )
}

// ---- 玻璃布局子组件（与 TaskDetail 同风格）----
const TH = 'px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_NUM = 'px-3 py-2 text-right font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_CENTER = 'px-3 py-2 text-center font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TD = 'px-3 py-2 align-middle text-gray-700 dark:text-gray-200'
const TD_NUM = 'px-3 py-2 align-middle text-right tabular-nums text-gray-700 dark:text-gray-200'

function Panel({
  title,
  hint,
  children,
}: {
  title: string
  hint?: string
  children: ReactNode
}) {
  return (
    <section className="glass rounded-2xl overflow-hidden">
      <div className="flex items-center justify-between px-5 py-3 border-b border-gray-200/50 dark:border-white/10">
        <span className="text-sm font-semibold text-gray-700 dark:text-gray-200">{title}</span>
        {hint ? <span className="text-xs text-gray-400 dark:text-gray-500">{hint}</span> : null}
      </div>
      <div className="p-5">{children}</div>
    </section>
  )
}

function KvGrid({ children }: { children: ReactNode }) {
  return <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-x-6 gap-y-3">{children}</div>
}

function Kv({ label, children, wide = false, mono = false, title }: { label: string; children: ReactNode; wide?: boolean; mono?: boolean; title?: string }) {
  return (
    <div className={`flex flex-col gap-0.5 ${wide ? 'sm:col-span-2 lg:col-span-3' : ''}`}>
      <span className="text-xs text-gray-400 dark:text-gray-500" title={title}>
        {label}
        {title && (
          <span className="ml-1 text-gray-300 dark:text-gray-600 cursor-help align-middle" aria-hidden="true">
            <svg className="w-3 h-3 inline" fill="currentColor" viewBox="0 0 24 24">
              <path d="M12 2a10 10 0 100 20 10 10 0 000-20zm0 15a1 1 0 110-2 1 1 0 010 2zm1.07-7.75l-.9.92c-.5.51-.67.95-.67 1.83h-2v-.5c0-.66.27-1.26.67-1.67l1.24-1.26c.37-.36.59-.86.59-1.41a2 2 0 10-4 0H6a4 4 0 118 0c0 .73-.3 1.4-.83 1.99z" />
            </svg>
          </span>
        )}
      </span>
      <span className={`text-sm text-gray-800 dark:text-gray-100 break-words ${mono ? 'font-mono' : ''}`}>{children}</span>
    </div>
  )
}

function LinkBtn({ onClick, children }: { onClick: () => void; children: ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="text-apple-blue hover:text-apple-blue-hover cursor-pointer bg-transparent border-none p-0 text-left break-all focus:outline-none focus-visible:underline"
    >
      {children}
    </button>
  )
}
