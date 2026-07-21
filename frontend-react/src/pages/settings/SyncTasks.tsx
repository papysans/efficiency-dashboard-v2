// 设置·同步任务：提交 ETL 同步（起止时间 + 数据源 + 强制覆盖）+ 任务列表（进度/重试/取消）。
// 有进行中任务（pending/running/retrying）时每 5s 失效 ['chat-sync-tasks'] 轮询刷新。
import { useEffect, useMemo, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { chatStats } from '@/api/endpoints'
import { useChatDatasources, useChatSyncTasks } from '@/api/queries'
import type { ChatSyncTask } from '@/api/types'
import { formatDateTimeShort } from '@/lib/formatters'
import { Modal } from '@/components/ui/Modal'
import { Tag, type TagTone } from '@/components/ui/Tag'
import { formatShanghaiDayRange, toShanghaiSyncRange } from './syncDates'
import SettingsLayout, {
  BTN_DANGER,
  BTN_GLASS,
  BTN_PRIMARY,
  Field,
  INPUT_CLS,
  LINK_BTN,
  LINK_BTN_DANGER,
  TD,
  TD_NUM,
  TH,
  TH_NUM,
  useChatEnabled,
} from './SettingsLayout'

const STATUS_TONE: Record<string, TagTone> = {
  pending: 'neutral',
  running: 'info',
  completed: 'success',
  failed: 'error',
  retrying: 'warning',
}

const ACTIVE_STATUSES = new Set(['pending', 'running', 'retrying'])

function progressPercent(t: ChatSyncTask): number {
  if (!t.total_gaps) return t.status === 'completed' ? 100 : 0
  return Math.round((t.completed_gaps / t.total_gaps) * 100)
}

export default function SyncTasks() {
  const enabled = useChatEnabled()
  const queryClient = useQueryClient()
  const { data, isLoading, error, refetch } = useChatSyncTasks(enabled)
  const { data: datasources } = useChatDatasources(enabled)

  const tasks = useMemo(() => data?.tasks || [], [data])
  const enabledSources = useMemo(() => (datasources || []).filter((d) => d.is_enabled), [datasources])

  // ---- 提交同步 ----
  const [sourceId, setSourceId] = useState('')
  const [startDate, setStartDate] = useState('')
  const [endDate, setEndDate] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [submitMsg, setSubmitMsg] = useState<{ ok: boolean; text: string } | null>(null)

  // 数据源加载后默认选第一个启用的
  useEffect(() => {
    if (!sourceId && enabledSources.length > 0) setSourceId(String(enabledSources[0].id))
  }, [sourceId, enabledSources])

  async function handleSubmit() {
    const range = toShanghaiSyncRange(startDate, endDate)
    if (!range) {
      setSubmitMsg({ ok: false, text: '请选择有效的开始和结束日期，且开始日期不能晚于结束日期' })
      return
    }
    if (!sourceId) {
      setSubmitMsg({ ok: false, text: '请选择数据源' })
      return
    }
    setSubmitting(true)
    setSubmitMsg(null)
    try {
      const res = await chatStats.submitSyncTask({
        ...range,
        source_id: Number(sourceId),
        force: false,
      })
      setSubmitMsg({
        ok: true,
        text: `同步任务已提交（数据源：${res?.source_name || sourceId}）`,
      })
      setStartDate('')
      setEndDate('')
      await queryClient.invalidateQueries({ queryKey: ['chat-sync-tasks'] })
    } catch (e: unknown) {
      setSubmitMsg({ ok: false, text: e instanceof Error ? e.message : '提交失败' })
    } finally {
      setSubmitting(false)
    }
  }

  // ---- 进行中任务轮询（5s） ----
  const hasActive = useMemo(() => tasks.some((t) => ACTIVE_STATUSES.has(t.status)), [tasks])
  useEffect(() => {
    if (!hasActive || !enabled) return
    const timer = window.setInterval(() => {
      queryClient.invalidateQueries({ queryKey: ['chat-sync-tasks'] })
    }, 5000)
    return () => window.clearInterval(timer)
  }, [hasActive, enabled, queryClient])

  // ---- 重试 / 取消（带确认） ----
  const [pendingAction, setPendingAction] = useState<{ type: 'retry' | 'cancel'; task: ChatSyncTask } | null>(null)
  const [acting, setActing] = useState(false)
  const [actionErr, setActionErr] = useState('')

  async function confirmAction() {
    if (!pendingAction) return
    setActing(true)
    setActionErr('')
    try {
      if (pendingAction.type === 'retry') await chatStats.retrySyncTask(pendingAction.task.task_id)
      else await chatStats.cancelSyncTask(pendingAction.task.task_id)
      setPendingAction(null)
      await queryClient.invalidateQueries({ queryKey: ['chat-sync-tasks'] })
    } catch (e: unknown) {
      setActionErr(e instanceof Error ? e.message : '操作失败')
    } finally {
      setActing(false)
    }
  }

  return (
    <SettingsLayout>
      {/* 提交同步 */}
      <section className="glass rounded-2xl p-5 space-y-3">
        <h2 className="text-sm font-semibold text-gray-700 dark:text-gray-200">发起数据同步</h2>
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
          <Field label="数据源">
            <select value={sourceId} onChange={(e) => setSourceId(e.target.value)} className={INPUT_CLS}>
              {enabledSources.length === 0 && <option value="">暂无启用的数据源</option>}
              {enabledSources.map((d) => (
                <option key={d.id} value={String(d.id)}>
                  {d.name}（{d.source_type === 'postgres' ? 'PG' : 'ES'}）
                </option>
              ))}
            </select>
          </Field>
          <Field label="开始日期（含）">
            <input type="date" value={startDate} max={endDate || undefined} onChange={(e) => setStartDate(e.target.value)} className={INPUT_CLS} />
          </Field>
          <Field label="结束日期（含）">
            <input type="date" value={endDate} min={startDate || undefined} onChange={(e) => setEndDate(e.target.value)} className={INPUT_CLS} />
          </Field>
          <div className="flex items-end">
            <button type="button" onClick={handleSubmit} disabled={submitting} className={BTN_PRIMARY}>
              {submitting ? '提交中...' : '开始同步'}
            </button>
          </div>
        </div>
        <p className="text-xs text-gray-500 dark:text-gray-400">按北京时间同步，开始和结束日期均包含。</p>
        {submitMsg && (
          <div className={`text-sm ${submitMsg.ok ? 'text-emerald-600 dark:text-emerald-400' : 'text-rose-600 dark:text-rose-400'}`}>
            {submitMsg.text}
          </div>
        )}
      </section>

      {/* 任务列表 */}
      <section className="glass rounded-2xl overflow-hidden">
        <div className="flex items-center justify-between px-5 py-3 border-b border-gray-200/50 dark:border-white/10">
          <span className="text-sm font-semibold text-gray-700 dark:text-gray-200">
            同步任务（{tasks.length}）{hasActive && <span className="ml-2 text-xs font-normal text-apple-blue">进行中任务自动刷新</span>}
          </span>
          <button type="button" onClick={() => refetch()} className={BTN_GLASS}>
            刷新
          </button>
        </div>

        {error && (
          <div className="px-5 py-2 text-sm text-rose-600 dark:text-rose-400 bg-rose-50/50 dark:bg-rose-900/20">
            {(error as Error).message || '获取同步任务失败'}
          </div>
        )}

        <div className="overflow-x-auto">
          <table className="w-full text-sm border-collapse">
            <thead>
              <tr className="border-b border-gray-200/50 dark:border-white/10">
                <th className={TH}>任务ID</th>
                <th className={TH}>状态</th>
                <th className={TH}>数据源</th>
                <th className={`${TH} min-w-[140px]`}>进度</th>
                <th className={`${TH} min-w-[160px]`}>请求范围</th>
                <th className={TH_NUM}>处理行数</th>
                <th className={TH_NUM}>写入行数</th>
                <th className={`${TH} min-w-[120px]`}>错误信息</th>
                <th className={`${TH} text-center`}>操作</th>
              </tr>
            </thead>
            <tbody>
              {isLoading || !enabled ? (
                Array.from({ length: 4 }).map((_, i) => (
                  <tr key={i} className="border-b border-gray-100/50 dark:border-white/5">
                    <td className={TD} colSpan={9}>
                      <div className="skeleton h-6 rounded" />
                    </td>
                  </tr>
                ))
              ) : tasks.length === 0 ? (
                <tr>
                  <td colSpan={9}>
                    <div className="py-12 text-center text-sm text-gray-400 dark:text-gray-500">暂无同步任务</div>
                  </td>
                </tr>
              ) : (
                tasks.map((t) => {
                  const isActive = ACTIVE_STATUSES.has(t.status)
                  const pct = progressPercent(t)
                  return (
                    <tr key={t.task_id} className="border-b border-gray-100/50 dark:border-white/5 hover:bg-apple-blue/5 dark:hover:bg-white/5 transition-colors">
                      <td className={TD}>
                        <span className="font-mono text-xs" title={t.task_id}>{t.task_id.slice(0, 12)}...</span>
                      </td>
                      <td className={TD}>
                        <Tag tone={STATUS_TONE[t.status] || 'neutral'}>{t.status}</Tag>
                      </td>
                      <td className={TD}>{t.source_name || '-'}</td>
                      <td className={TD}>
                        <div className="flex items-center gap-2">
                          <div className="flex-1 h-1.5 rounded-full bg-gray-200/70 dark:bg-white/10 overflow-hidden" role="progressbar" aria-valuenow={pct} aria-valuemin={0} aria-valuemax={100}>
                            <div
                              className={`h-full rounded-full transition-all ${t.status === 'failed' ? 'bg-rose-500' : t.status === 'completed' ? 'bg-emerald-500' : 'bg-apple-blue'}`}
                              style={{ width: `${pct}%` }}
                            />
                          </div>
                          <span className="text-xs tabular-nums text-gray-500 dark:text-gray-400 whitespace-nowrap">
                            {t.total_gaps ? `${t.completed_gaps}/${t.total_gaps}` : `${pct}%`}
                          </span>
                        </div>
                      </td>
                      <td className={TD}>
                        <span className="text-xs whitespace-nowrap">
                          {formatShanghaiDayRange(t.req_start_time, t.req_end_time) ||
                            `${formatDateTimeShort(t.req_start_time)} ~ ${formatDateTimeShort(t.req_end_time)}`}
                        </span>
                      </td>
                      <td className={TD_NUM}>{t.total_rows_processed?.toLocaleString() ?? '-'}</td>
                      <td className={TD_NUM}>{t.total_rows_written?.toLocaleString() ?? '-'}</td>
                      <td className={TD}>
                        {t.error_message ? (
                          <div className="max-w-[180px] truncate text-xs text-rose-600 dark:text-rose-400" title={t.error_message}>
                            {t.error_message}
                          </div>
                        ) : (
                          <span className="text-xs text-gray-400">-</span>
                        )}
                      </td>
                      <td className="px-3 py-2 align-middle text-center whitespace-nowrap">
                        <div className="inline-flex items-center gap-2">
                          {t.status === 'failed' && (
                            <button type="button" className={LINK_BTN} onClick={() => setPendingAction({ type: 'retry', task: t })}>
                              重试
                            </button>
                          )}
                          {isActive && (
                            <button type="button" className={LINK_BTN_DANGER} onClick={() => setPendingAction({ type: 'cancel', task: t })}>
                              停止
                            </button>
                          )}
                          {!isActive && t.status !== 'failed' && <span className="text-xs text-gray-400">-</span>}
                        </div>
                      </td>
                    </tr>
                  )
                })
              )}
            </tbody>
          </table>
        </div>
      </section>

      {/* 重试/取消确认 */}
      <Modal
        open={!!pendingAction}
        title={pendingAction?.type === 'retry' ? '确认重试' : '确认停止'}
        maxWidth={420}
        onClose={() => setPendingAction(null)}
        footer={
          <>
            <button type="button" className={BTN_GLASS} onClick={() => setPendingAction(null)}>取消</button>
            <button
              type="button"
              className={pendingAction?.type === 'retry' ? BTN_PRIMARY : BTN_DANGER}
              disabled={acting}
              onClick={confirmAction}
            >
              {acting ? '处理中...' : pendingAction?.type === 'retry' ? '重试' : '停止任务'}
            </button>
          </>
        }
      >
        <div className="space-y-2">
          {actionErr && <div className="text-sm text-rose-600 dark:text-rose-400">{actionErr}</div>}
          <p className="text-sm text-gray-700 dark:text-gray-200">
            {pendingAction?.type === 'retry'
              ? `确定重试任务 ${pendingAction?.task.task_id.slice(0, 12)}... 吗？`
              : `确定停止任务 ${pendingAction?.task.task_id.slice(0, 12)}... 吗？任务将标记为失败。`}
          </p>
        </div>
      </Modal>
    </SettingsLayout>
  )
}
