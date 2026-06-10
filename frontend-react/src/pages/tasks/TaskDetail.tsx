// Task 详情页（TaskDetailV2 的 React + 玻璃拟态迁移）。
// 分区 1:1 按 research/pr2-task-pages.md §7；视觉换玻璃拟态。
//
// ⚠️ efficiency_ratio 百分比口径：详情大数字 Math.round()+'%'，不 ×100，用 percentTextClass 着色。
// manual 优先：度量区有 manual 时显示 manual 值 + 黄(?)理由 + 删除线原 AI 值 + 灰(?)AI 理由。
// time_segments 是死代码：会话时间线纯线性（无 gap/segment 分支）。
import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { useNavigate, useParams } from 'react-router'
import { useQueryClient } from '@tanstack/react-query'
import { useTaskDetail } from '@/api/queries'
import { getTaskFileUrl, updateTaskManualV2 } from '@/api/endpoints'
import type { Conversation, TaskListItem, UpdateTaskManualRequest } from '@/api/types'
import { useUserNameMap } from '@/hooks/useUserNameMap'
import { fmtCost, formatDuration, formatLocalTime } from '@/lib/formatters'
import { Tag } from '@/components/ui/Tag'
import { percentTextClass } from '@/components/ui/PercentPill'
import { Modal } from '@/components/ui/Modal'

const DISPLAY_LIMIT = 200

// 采集侧会把 harness 注入块（任务通知/系统提醒/环境详情等）混进 user_input，
// 它们不是用户提问。展示时剥掉，剥完为空说明整条是系统消息。
const NOISE_TAG_RE =
  /^<(task-notification|system-reminder|environment_details|local-command-stdout|local-command-caveat|command-name|command-message|command-args|file_content|workspace_diagnostics)>/

function extractUserQuestion(raw?: string): string {
  let s = (raw || '').trim()
  for (;;) {
    const m = s.match(NOISE_TAG_RE)
    if (!m) break
    const close = `</${m[1]}>`
    const idx = s.indexOf(close)
    if (idx === -1) return '' // 注入块未闭合 → 整条都是系统内容
    s = s.slice(idx + close.length).trim()
  }
  // 尾部附加的环境详情块一并剥掉
  return s.replace(/<environment_details>[\s\S]*?<\/environment_details>/g, '').trim()
}

export default function TaskDetail() {
  const { taskId } = useParams<{ taskId: string }>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { data, isLoading, error } = useTaskDetail(taskId)
  // task user_name 多为 UUID，用 commits 的 git_user_name 解析真实名。
  const { resolveName } = useUserNameMap()

  // 合并：顶层 efficiency_ratio 覆盖进 task（§7.1）。
  const task: TaskListItem = useMemo(() => {
    const t = (data?.task || { task_id: taskId || '' }) as TaskListItem
    if (data?.efficiency_ratio != null) return { ...t, efficiency_ratio: data.efficiency_ratio }
    return t
  }, [data, taskId])
  const conversations: Conversation[] = data?.conversations || []

  const [modalOpen, setModalOpen] = useState(false)
  const [expanded, setExpanded] = useState<Set<string>>(new Set())

  // 前端聚合（基于 conversations，§7.4）
  const totalUpstreamTokens = useMemo(() => conversations.reduce((s, c) => s + (c.upstream_tokens || 0), 0), [conversations])
  const totalDownstreamTokens = useMemo(() => conversations.reduce((s, c) => s + (c.downstream_tokens || 0), 0), [conversations])
  const totalTokens = totalUpstreamTokens + totalDownstreamTokens
  const totalCostSum = useMemo(() => conversations.reduce((s, c) => s + (c.cost || 0), 0), [conversations])

  const repoDisplay = task.repo_addr && task.repo_branch ? `${task.repo_addr}#${task.repo_branch}` : '-'
  const ratio = task.efficiency_ratio

  async function submitManual(body: UpdateTaskManualRequest) {
    await updateTaskManualV2(taskId as string, body)
    setModalOpen(false)
    await queryClient.invalidateQueries({ queryKey: ['task-detail', taskId] })
  }

  function toggleExpand(key: string) {
    setExpanded((prev) => {
      const next = new Set(prev)
      if (next.has(key)) next.delete(key)
      else next.add(key)
      return next
    })
  }

  if (error) {
    return (
      <div className="glass rounded-2xl p-8 text-center text-sm text-rose-600 dark:text-rose-400">
        {(error as Error).message || '获取 Task 详情失败'}
      </div>
    )
  }

  return (
    <div className={`space-y-5 ${isLoading ? 'opacity-60 pointer-events-none' : ''}`}>
      {/* 标题栏 */}
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
          <h1 className="text-xl font-bold text-gray-900 dark:text-white">Task 详情</h1>
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

      {/* ① 基础信息 */}
      <Panel title="基础信息">
        <KvGrid>
          <Kv label="Task ID" mono>{task.task_id || '-'}</Kv>
          <Kv label="任务描述" wide>{task.title || '-'}</Kv>
          <Kv label="用户">
            {task.user_id ? (
              <LinkBtn onClick={() => navigate(`/user/${encodeURIComponent(task.user_id as string)}`)}>
                {resolveName(task.user_id)}
              </LinkBtn>
            ) : (
              task.user_name || '-'
            )}
          </Kv>
          <Kv label="仓库">
            {repoDisplay !== '-' ? (
              <LinkBtn
                onClick={() =>
                  navigate(`/repo/${encodeURIComponent(task.repo_addr as string)}/${encodeURIComponent(task.repo_branch as string)}`)
                }
              >
                {repoDisplay}
              </LinkBtn>
            ) : (
              '-'
            )}
          </Kv>
          <Kv label="工作目录">
            {task.work_dir_id ? (
              <LinkBtn onClick={() => navigate(`/workdir/${encodeURIComponent(task.work_dir_id as string)}`)}>
                {task.work_dir || task.work_dir_id}
              </LinkBtn>
            ) : (
              '-'
            )}
          </Kv>
          <Kv label="开始时间">{formatLocalTime(task.start_time)}</Kv>
          <Kv label="结束时间">{formatLocalTime(task.end_time)}</Kv>
          <Kv label="系统">{task.client_os ? `${task.client_os} ${task.client_os_version || ''}`.trim() : '-'}</Kv>
          <Kv label="客户端">{task.client_ide ? `${task.client_ide} ${task.client_version || ''}`.trim() : '-'}</Kv>
          <Kv label="模式">{task.caller || '-'}</Kv>
        </KvGrid>
      </Panel>

      {/* ② 度量信息 */}
      <Panel title="度量信息">
        <KvGrid>
          <Kv label="生成代码量">
            <span className="inline-flex items-center gap-2">
              {task.diff_lines ?? '-'} 行
              <FileLink href={getTaskFileUrl('summary', task.task_id, task.start_time)}>查看详情</FileLink>
            </span>
          </Kv>
          <Kv label="实际耗时">
            <ManualValue
              manual={task.task_real_minutes_manual}
              manualReason={task.task_real_minutes_reason_manual}
              original={task.task_real_minutes}
              originalReason={task.task_real_minutes_reason}
            />
          </Kv>
          <Kv label="传统开发时长预估">
            <ManualValue
              manual={task.task_ancient_minutes_manual}
              manualReason={task.task_ancient_minutes_reason_manual}
              original={task.task_ancient_minutes}
              originalReason={task.task_ancient_minutes_reason}
            />
          </Kv>
          <Kv label="API请求次数">{conversations.length || '-'}</Kv>
          <Kv label="总Tokens" title={`上行 ${totalUpstreamTokens} / 下行 ${totalDownstreamTokens}`}>
            {totalTokens > 0 ? totalTokens.toLocaleString() : '-'}
          </Kv>
          <Kv label="费用">
            {(task.cost ?? 0) > 0 ? `${fmtCost(task.cost)} 元` : totalCostSum > 0 ? `${fmtCost(totalCostSum)} 元` : '-'}
          </Kv>
          <Kv label="提效比例">
            <span className={`text-xl font-bold tabular-nums ${percentTextClass(ratio)}`}>
              {ratio != null ? `${Math.round(ratio)}%` : '-'}
            </span>
          </Kv>
        </KvGrid>
      </Panel>

      {/* ③ 对话历史（纯线性时间线，无 gap） */}
      {conversations.length > 0 && (
        <Panel
          title="对话历史"
          hint=""
          rightSlot={<FileLink href={getTaskFileUrl('conversation', task.task_id, task.start_time)}>查看原始数据</FileLink>}
        >
          <ol className="relative border-l-2 border-gray-200/60 dark:border-white/10 ml-2 space-y-4">
            {conversations.map((conv, idx) => {
              const key = `${idx}_conv`
              const isExpanded = expanded.has(key)
              const input = conv.user_input || ''
              // 用户提问优先展示；剥完系统注入块为空 → 整条是系统消息，默认折叠原文。
              const question = extractUserQuestion(input)
              const isSystemOnly = !!input && !question
              const text = question || input
              const truncated = text.length > DISPLAY_LIMIT && !isExpanded
              const shown = truncated ? `${text.substring(0, DISPLAY_LIMIT)}...` : text
              return (
                <li key={conv.id ?? idx} className="ml-5">
                  <span className="absolute -left-[7px] w-3 h-3 rounded-full bg-emerald-500 ring-2 ring-white dark:ring-[#0a0a0f]" aria-hidden="true" />
                  <div className="text-xs text-gray-400 dark:text-gray-500 mb-1">{formatLocalTime(conv.start_time)}</div>
                  <div className="glass rounded-xl p-3 space-y-1.5">
                    {question && (
                      <pre className="whitespace-pre-wrap break-all bg-gray-50/80 dark:bg-white/5 rounded-lg px-3 py-2 text-xs leading-relaxed text-gray-700 dark:text-gray-200 max-h-[600px] overflow-y-auto m-0">
                        {shown}
                      </pre>
                    )}
                    {isSystemOnly && (
                      <div className="text-xs text-gray-400 dark:text-gray-500">
                        系统消息（非用户提问）
                        {isExpanded && (
                          <pre className="whitespace-pre-wrap break-all bg-gray-50/80 dark:bg-white/5 rounded-lg px-3 py-2 mt-1.5 text-xs leading-relaxed text-gray-500 dark:text-gray-400 max-h-[600px] overflow-y-auto m-0">
                            {shown}
                          </pre>
                        )}
                      </div>
                    )}
                    {!input && <div className="text-xs text-gray-400 dark:text-gray-500">（无用户输入）</div>}
                    {(text.length > DISPLAY_LIMIT || isSystemOnly) && (
                      <button
                        type="button"
                        onClick={() => toggleExpand(key)}
                        className="text-xs text-apple-blue hover:text-apple-blue-hover cursor-pointer bg-transparent border-none p-0 focus:outline-none focus-visible:underline"
                      >
                        {isExpanded ? '收起' : isSystemOnly ? '展开原文' : '展开全文'}
                      </button>
                    )}
                    <div className="flex flex-wrap items-center gap-2 text-xs text-gray-400 dark:text-gray-500">
                      <span>{conv.model || conv.mode || '-'}</span>
                      <span>耗时 {conv.process_time ?? '-'} ms</span>
                      <span>上行 {conv.upstream_tokens ?? '-'} / 下行 {conv.downstream_tokens ?? '-'}</span>
                      <span>费用 {fmtCost(conv.cost) || '0.00'}</span>
                      <span>代码 {conv.diff_lines ?? '-'} 行</span>
                      {conv.error_code && (
                        <Tag tone="error">
                          {conv.error_code}: {conv.error_reason}
                        </Tag>
                      )}
                    </div>
                  </div>
                </li>
              )
            })}
          </ol>
        </Panel>
      )}

      {!isLoading && conversations.length === 0 && task.task_id && (
        <Panel title="对话历史">
          <div className="py-6 text-center text-sm text-gray-400 dark:text-gray-500">暂无对话记录</div>
        </Panel>
      )}

      {/* ④ 人工调整 Modal */}
      <ManualModal open={modalOpen} task={task} onClose={() => setModalOpen(false)} onSubmit={submitManual} />
    </div>
  )
}

// ---- manual 优先显示（实际耗时 / 传统预估 通用，§7.4）----
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

// ---- 人工调整对话框（§7.6）----
function ManualModal({
  open,
  task,
  onClose,
  onSubmit,
}: {
  open: boolean
  task: TaskListItem
  onClose: () => void
  onSubmit: (body: UpdateTaskManualRequest) => Promise<void>
}) {
  const [real, setReal] = useState<string>('')
  const [realReason, setRealReason] = useState('')
  const [ancient, setAncient] = useState<string>('')
  const [ancientReason, setAncientReason] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [err, setErr] = useState('')

  // 打开时预填：manual 优先，否则原 AI 值（§7.6）。
  useEffect(() => {
    if (!open) return
    const r = task.task_real_minutes_manual ?? task.task_real_minutes ?? null
    const a = task.task_ancient_minutes_manual ?? task.task_ancient_minutes ?? null
    setReal(r == null ? '' : String(r))
    setRealReason(task.task_real_minutes_reason_manual || '')
    setAncient(a == null ? '' : String(a))
    setAncientReason(task.task_ancient_minutes_reason_manual || '')
    setErr('')
  }, [open, task])

  async function handleSubmit() {
    setSubmitting(true)
    setErr('')
    try {
      await onSubmit({
        task_real_minutes_manual: real === '' ? null : Number(real),
        task_real_minutes_reason_manual: realReason,
        task_ancient_minutes_manual: ancient === '' ? null : Number(ancient),
        task_ancient_minutes_reason_manual: ancientReason,
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
        <Field label="实际耗时（分钟）">
          <input type="number" step={10} value={real} onChange={(e) => setReal(e.target.value)} className={inputCls} />
        </Field>
        <Field label="实际耗时理由">
          <textarea rows={2} value={realReason} onChange={(e) => setRealReason(e.target.value)} className={`${inputCls} resize-y`} />
        </Field>
        <Field label="传统开发时长预估（分钟）">
          <input type="number" step={10} value={ancient} onChange={(e) => setAncient(e.target.value)} className={inputCls} />
        </Field>
        <Field label="传统开发时长预估理由">
          <textarea rows={2} value={ancientReason} onChange={(e) => setAncientReason(e.target.value)} className={`${inputCls} resize-y`} />
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

// ---- 玻璃布局子组件（与 NeedDetail 同风格）----
function Panel({
  title,
  hint,
  rightSlot,
  children,
}: {
  title: string
  hint?: string
  rightSlot?: ReactNode
  children: ReactNode
}) {
  return (
    <section className="glass rounded-2xl overflow-hidden">
      <div className="flex items-center justify-between px-5 py-3 border-b border-gray-200/50 dark:border-white/10">
        <span className="text-sm font-semibold text-gray-700 dark:text-gray-200">{title}</span>
        {rightSlot ?? (hint ? <span className="text-xs text-gray-400 dark:text-gray-500">{hint}</span> : null)}
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
    <div className={`flex flex-col gap-0.5 ${wide ? 'sm:col-span-2 lg:col-span-2' : ''}`}>
      <span className="text-xs text-gray-400 dark:text-gray-500">{label}</span>
      <span className={`text-sm text-gray-800 dark:text-gray-100 break-words ${mono ? 'font-mono' : ''}`} title={title}>
        {children}
      </span>
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

/** 任务文件外链：href 为空时不渲染（无 start_time）。 */
function FileLink({ href, children }: { href: string; children: ReactNode }) {
  if (!href) return null
  return (
    <a
      href={href}
      target="_blank"
      rel="noreferrer"
      className="text-xs text-apple-blue hover:text-apple-blue-hover no-underline hover:underline cursor-pointer focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue rounded"
    >
      {children}
    </a>
  )
}
