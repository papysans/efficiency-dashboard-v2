// 平台·明细点查 —— chat-indicator-statistics /stats/detail/query 的玻璃拟态重写（现场排查核心）。
// 一次性查询（无分页），SQL 层过滤，最多 100 条；行点击弹详情（全部字段，错误码醒目）。
// 时间发送本地壁钟时间 + 浏览器实际时区偏移（chat 侧按 RFC3339 解析，避免硬编码 +08:00）。
// 数据源显式选择后传 datasource_id，避免多源环境下误查到服务端默认数据源。
import { useEffect, useMemo, useState, useCallback, type ReactNode } from 'react'
import { useMutation } from '@tanstack/react-query'
import { chatStats } from '@/api/endpoints'
import { useChatDatasources, useGlobalConfig } from '@/api/queries'
import type { ChatDetailQueryReq, ChatDetailRow, ChatLogPreviewResponse } from '@/api/types'
import { useUserNameMap } from '@/hooks/useUserNameMap'
import { Modal } from '@/components/ui/Modal'
import { Tag } from '@/components/ui/Tag'
import { formatLocalTime, formatNumber } from '@/lib/formatters'
import SettingsLayout, { ChatDisabledNotice } from '@/pages/settings/SettingsLayout'
import { ChatUserCell, isErrorCode } from './platformShared'

// ---- 时间工具（datetime-local <-> ISO 8601 带偏移） ----

function pad2(n: number): string {
  return String(n).padStart(2, '0')
}

/** Date → datetime-local 输入值（本地时区，'YYYY-MM-DDTHH:mm'）。 */
function toLocalInputValue(d: Date): string {
  return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}T${pad2(d.getHours())}:${pad2(d.getMinutes())}`
}

/** datetime-local 值 → 'YYYY-MM-DDTHH:mm:ss±HH:mm'（补秒 + 浏览器实际时区偏移）。 */
function toIsoWithOffset(v: string): string {
  const d = new Date(v)
  const off = -d.getTimezoneOffset()
  const sign = off >= 0 ? '+' : '-'
  const abs = Math.abs(off)
  const base = v.length === 16 ? `${v}:00` : v
  return `${base}${sign}${pad2(Math.floor(abs / 60))}:${pad2(abs % 60)}`
}

// ---- 常量 ----

const QUICK_RANGES = [
  { label: '近30分钟', minutes: 30 },
  { label: '近1小时', minutes: 60 },
  { label: '近3小时', minutes: 180 },
  { label: '近6小时', minutes: 360 },
  { label: '近12小时', minutes: 720 },
  { label: '近24小时', minutes: 1440 },
]

interface QueryForm {
  datasourceId: string
  start: string
  end: string
  universalId: string
  userId: string
  userName: string
  requestId: string
  model: string
  routedModel: string
  /** '' = 全部，'true' = 仅错误，'false' = 仅成功 */
  hasError: '' | 'true' | 'false'
  limit: number
  order: 'desc' | 'asc'
}

function defaultForm(datasourceId = ''): QueryForm {
  const end = new Date()
  const start = new Date(end.getTime() - 30 * 60_000)
  return {
  userId: '',
  userName: '',
    datasourceId,
    start: toLocalInputValue(start),
    end: toLocalInputValue(end),
    universalId: '',
    requestId: '',
    model: '',
    routedModel: '',
    hasError: '',
    limit: 100,
    order: 'desc',
  }
}

const TH = 'px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_NUM = 'px-3 py-2 text-right font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TD = 'px-3 py-2 align-middle text-gray-700 dark:text-gray-200 whitespace-nowrap'
const TD_NUM = 'px-3 py-2 align-middle text-right tabular-nums text-gray-700 dark:text-gray-200'

const INPUT_CLS =
  'glass rounded-lg px-3 py-1.5 text-sm bg-transparent text-gray-900 dark:text-white placeholder-gray-400 ' +
  'focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue'
const BTN_SECONDARY =
  'glass rounded-lg px-3 py-1 text-xs text-gray-700 dark:text-gray-200 cursor-pointer hover:text-apple-blue ' +
  'transition-colors disabled:opacity-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue'

const fmtMs = (v: number | null | undefined) => (v != null ? `${Number(v).toFixed(0)} ms` : '-')
const fmtFloat = (v: number | null | undefined, digits = 2) => (v != null ? Number(v).toFixed(digits) : '-')

interface LogPreviewState {
  open: boolean
  loading: boolean
  data: ChatLogPreviewResponse | null
  error: string
  path: string
}

export default function RealtimeQuery() {
  // 开关语义与设置区/态势页一致：未启用时整页提示，不让表单提交打到 503 的代理。
  const { data: gc } = useGlobalConfig()
  const chatEnabled = gc?.chat_stats_enabled === true
  const chatDisabled = !!gc && !chatEnabled
  const { data: datasources, isLoading: dsLoading, error: dsError } = useChatDatasources(chatEnabled)
  const enabledDatasources = useMemo(() => (datasources || []).filter((d) => d.is_enabled), [datasources])

  const [form, setForm] = useState<QueryForm>(defaultForm)
  const [validateMsg, setValidateMsg] = useState('')
  const [detailRow, setDetailRow] = useState<ChatDetailRow | null>(null)
  const [logPreview, setLogPreview] = useState<LogPreviewState>({
    open: false,
    loading: false,
    data: null,
    error: '',
    path: '',
  })
  // universal_id 与看板 user_id 同源 → 结果表/详情弹层解析看板用户名并互链（失败自动回退）。
  const { resolveName } = useUserNameMap()

  // queries.ts 无明细点查 hook（按需触发的点查不适合 useQuery 缓存语义），页面内局部 useMutation。
  const query = useMutation({
    mutationFn: (body: ChatDetailQueryReq) => chatStats.queryDetail(body),
  })

  useEffect(() => {
    if (!form.datasourceId && enabledDatasources.length > 0) {
      setForm((f) => ({ ...f, datasourceId: String(enabledDatasources[0].id) }))
    }
  }, [form.datasourceId, enabledDatasources])

  function setField<K extends keyof QueryForm>(key: K, value: QueryForm[K]) {
    setForm((f) => ({ ...f, [key]: value }))
  }

  function applyQuickRange(minutes: number) {
    const end = new Date()
    const start = new Date(end.getTime() - minutes * 60_000)
    setForm((f) => ({ ...f, start: toLocalInputValue(start), end: toLocalInputValue(end) }))
  }

  function submit() {
    if (!form.datasourceId) {
      setValidateMsg('请选择数据源')
      return
    }
    if (!form.start || !form.end) {
      setValidateMsg('请选择查询起止时间')
      return
    }
    const startMs = new Date(form.start).getTime()
    const endMs = new Date(form.end).getTime()
    if (Number.isNaN(startMs) || Number.isNaN(endMs)) {
      setValidateMsg('时间格式无效')
      return
    }
    if (startMs >= endMs) {
      setValidateMsg('开始时间必须早于结束时间')
      return
    }
    setValidateMsg('')

    // 所有文本输入提交前 .trim()（CLAUDE.md 输入处理规范）。
    const body: ChatDetailQueryReq = {
      datasource_id: form.datasourceId,
      start_time: toIsoWithOffset(form.start),
      end_time: toIsoWithOffset(form.end),
      limit: form.limit,
      order: form.order,
    }
    const universalId = form.universalId.trim()
    const requestId = form.requestId.trim()
    const userId = form.userId.trim()
    const userName = form.userName.trim()
    if (userId) body.user_id = userId
    if (userName) body.username = userName
    const model = form.model.trim()
    const routedModel = form.routedModel.trim()
    if (universalId) body.universal_id = universalId
    if (requestId) body.request_id = requestId
    if (model) body.model = model
    if (routedModel) body.routed_model = routedModel
    if (form.hasError === 'true') body.has_error = true
    else if (form.hasError === 'false') body.has_error = false

    query.mutate(body)
  }

  function resetForm() {
    setForm(defaultForm(form.datasourceId || (enabledDatasources[0] ? String(enabledDatasources[0].id) : '')))
    setValidateMsg('')
  }

  function onFieldKey(e: React.KeyboardEvent) {
    if (e.key === 'Enter') submit()
  }

  async function previewLog(localLogPath: string | null | undefined) {
    const path = (localLogPath || '').trim()
    if (!path) return
    setLogPreview({ open: true, loading: true, data: null, error: '', path })
    try {
      const data = await chatStats.previewLog({ local_log_path: path })
      setLogPreview({ open: true, loading: false, data, error: '', path })
    } catch (e: unknown) {
      setLogPreview({
        open: true,
        loading: false,
        data: null,
        error: e instanceof Error ? e.message : '日志预览失败',
        path,
      })
    }
  }

  const rows = query.data?.items ?? []
  const total = query.data?.total ?? 0

  const header = (
    <header>
      <h2 className="text-lg font-semibold text-gray-900 dark:text-white">明细查询</h2>
      <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
        按条件点查 LLM 请求明细（直查源库，最多返回 100 条），用于现场排查。
      </p>
    </header>
  )

  if (chatDisabled) {
    return (
      <SettingsLayout>
        <div className="space-y-5">
          {header}
          <ChatDisabledNotice />
        </div>
      </SettingsLayout>
    )
  }

  return (
    <SettingsLayout>
      <div className="space-y-5">
        {header}

        {/* 查询条件 */}
      <section className="glass rounded-2xl p-5 space-y-3">
        <div className="flex flex-wrap items-center gap-2">
          <label className="text-sm text-gray-600 dark:text-gray-300" htmlFor="rq-datasource">
            数据源
          </label>
          <select
            id="rq-datasource"
            value={form.datasourceId}
            onChange={(e) => setField('datasourceId', e.target.value)}
            disabled={dsLoading || query.isPending}
            className={`${INPUT_CLS} min-w-[240px] cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed`}
            aria-label="数据源"
          >
            {dsLoading ? (
              <option value="">正在加载数据源...</option>
            ) : enabledDatasources.length === 0 ? (
              <option value="">暂无可用数据源</option>
            ) : (
              <>
                <option value="">请选择数据源</option>
                {(datasources || []).map((d) => (
                  <option key={d.id} value={String(d.id)} disabled={!d.is_enabled}>
                    {d.name}（{d.source_type === 'postgres' ? 'PG' : 'ES'}）{d.is_enabled ? '' : ' - 未启用'}
                  </option>
                ))}
              </>
            )}
          </select>
          <label className="text-sm text-gray-600 dark:text-gray-300" htmlFor="rq-start">
            时间范围
          </label>
          <input
            id="rq-start"
            type="datetime-local"
            value={form.start}
            onChange={(e) => setField('start', e.target.value)}
            className={INPUT_CLS}
            aria-label="开始时间"
          />
          <span className="text-gray-400">~</span>
          <input
            type="datetime-local"
            value={form.end}
            onChange={(e) => setField('end', e.target.value)}
            className={INPUT_CLS}
            aria-label="结束时间"
          />
          <span className="text-xs text-gray-400 dark:text-gray-500 ml-1">快捷：</span>
          {QUICK_RANGES.map((r) => (
            <button
              key={r.minutes}
              type="button"
              onClick={() => applyQuickRange(r.minutes)}
              className="px-2 py-1 rounded-md text-xs text-gray-600 dark:text-gray-300 hover:text-apple-blue hover:bg-white/50 dark:hover:bg-white/10 cursor-pointer transition-colors bg-transparent border-none focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue"
            >
              {r.label}
            </button>
          ))}
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <input
            value={form.universalId}
            onChange={(e) => setField('universalId', e.target.value)}
            onKeyDown={onFieldKey}
            placeholder="Universal ID（精确）"
            className={`${INPUT_CLS} w-[200px]`}
          />
          <input
            value={form.userId}
            onChange={(e) => setField('userId', e.target.value)}
            onKeyDown={onFieldKey}
            placeholder="User ID（精确）"
            className={`${INPUT_CLS} w-[170px]`}
          />
          <input
            value={form.userName}
            onChange={(e) => setField('userName', e.target.value)}
            onKeyDown={onFieldKey}
            placeholder="用户名（精确）"
            className={`${INPUT_CLS} w-[170px]`}
          />
          <input
            value={form.requestId}
            onChange={(e) => setField('requestId', e.target.value)}
            onKeyDown={onFieldKey}
            placeholder="Request ID（精确）"
            className={`${INPUT_CLS} w-[260px]`}
          />
          <input
            value={form.model}
            onChange={(e) => setField('model', e.target.value)}
            onKeyDown={onFieldKey}
            placeholder="Model（精确）"
            className={`${INPUT_CLS} w-[160px]`}
          />
          <input
            value={form.routedModel}
            onChange={(e) => setField('routedModel', e.target.value)}
            onKeyDown={onFieldKey}
            placeholder="Routed Model（精确）"
            className={`${INPUT_CLS} w-[170px]`}
          />
          <select
            value={form.hasError}
            onChange={(e) => setField('hasError', e.target.value as QueryForm['hasError'])}
            className={`${INPUT_CLS} cursor-pointer`}
            aria-label="是否存在错误"
          >
            <option value="">全部请求</option>
            <option value="true">仅错误</option>
            <option value="false">仅成功</option>
          </select>
          <select
            value={form.limit}
            onChange={(e) => setField('limit', Number(e.target.value))}
            className={`${INPUT_CLS} cursor-pointer`}
            aria-label="最多返回条数"
          >
            <option value={10}>最多 10 条</option>
            <option value={50}>最多 50 条</option>
            <option value={100}>最多 100 条</option>
          </select>
          <select
            value={form.order}
            onChange={(e) => setField('order', e.target.value as QueryForm['order'])}
            className={`${INPUT_CLS} cursor-pointer`}
            aria-label="排序方向"
          >
            <option value="desc">时间倒序</option>
            <option value="asc">时间正序</option>
          </select>
          <button
            type="button"
            onClick={submit}
            disabled={query.isPending || dsLoading || enabledDatasources.length === 0}
            className="bg-apple-blue hover:bg-apple-blue-hover text-white rounded-lg px-4 py-1.5 text-sm font-medium cursor-pointer transition-colors disabled:opacity-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue"
          >
            {query.isPending ? '查询中…' : '查询'}
          </button>
          <button
            type="button"
            onClick={resetForm}
            className="glass rounded-lg px-4 py-1.5 text-sm text-gray-700 dark:text-gray-200 cursor-pointer hover:text-apple-blue transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue"
          >
            重置
          </button>
        </div>

        {validateMsg && <div className="text-sm text-amber-600 dark:text-amber-400">{validateMsg}</div>}
        {dsError && (
          <div className="text-sm text-rose-600 dark:text-rose-400">
            {(dsError as Error).message || '获取数据源失败'}
          </div>
        )}
      </section>

      {/* 查询结果 */}
      <section className="glass rounded-2xl overflow-hidden">
        <div className="flex items-center justify-between px-5 py-3 border-b border-gray-200/50 dark:border-white/10">
          <span className="text-sm font-semibold text-gray-700 dark:text-gray-200">
            查询结果
            {query.isSuccess && (
              <span className="ml-2 text-xs font-normal text-gray-400 dark:text-gray-500">共 {total} 条记录</span>
            )}
          </span>
          <span className="text-xs text-gray-400 dark:text-gray-500">点击行查看全部字段</span>
        </div>

        {query.error && (
          <div className="px-5 py-2 text-sm text-rose-600 dark:text-rose-400 bg-rose-50/50 dark:bg-rose-900/20">
            {(query.error as Error).message}
          </div>
        )}

        <div className="overflow-x-auto">
          <table className="w-full text-sm border-collapse">
            <thead>
              <tr className="border-b border-gray-200/50 dark:border-white/10">
                <th className={TH}>时间</th>
                <th className={TH}>Request ID</th>
                <th className={TH}>Universal ID</th>
                <th className={TH}>User ID</th>
                <th className={TH}>用户名</th>
                <th className={TH}>Model</th>
                <th className={TH}>Routed</th>
                <th className={TH}>状态</th>
                <th className={TH_NUM}>输入 Token</th>
                <th className={TH_NUM}>输出 Token</th>
                <th className={TH_NUM}>耗时</th>
              </tr>
            </thead>
            <tbody>
              {query.isPending ? (
                Array.from({ length: 6 }).map((_, i) => (
                  <tr key={i} className="border-b border-gray-100/50 dark:border-white/5">
                    <td className={TD} colSpan={11}>
                      <div className="skeleton h-6 rounded" />
                    </td>
                  </tr>
                ))
              ) : rows.length === 0 ? (
                <tr>
                  <td colSpan={11}>
                    <div className="py-12 text-center text-sm text-gray-400 dark:text-gray-500">
                      {query.isSuccess ? '未查询到符合条件的记录' : '设置查询条件后点击「查询」'}
                    </div>
                  </td>
                </tr>
              ) : (
                // 行 key 禁用 id：ES 数据源所有行 id=0（自增 id 仅 PG 源有），会撞 key。
                // request_id 为雪花/UUIDv7（唯一性可靠），拼 index 兜底。
                rows.map((row, idx) => (
                  <tr
                    key={`${row.request_id || 'row'}-${idx}`}
                    onClick={() => setDetailRow(row)}
                    className="border-b border-gray-100/50 dark:border-white/5 cursor-pointer hover:bg-apple-blue/5 dark:hover:bg-white/5 transition-colors"
                  >
                    <td className={TD}>{formatLocalTime(row.ts)}</td>
                    <td className={`${TD} font-mono text-xs`}>
                      <div className="max-w-[260px] truncate" title={row.request_id}>
                        {row.request_id || '-'}
                      </div>
                    </td>
                    <td className={`${TD} font-mono text-xs`}>
                      <div className="max-w-[150px] truncate" title={row.universal_id ?? undefined}>
                        {row.universal_id || '-'}
                      </div>
                    </td>
                    <td className={`${TD} font-mono text-xs`}>
                      <div className="max-w-[150px] truncate" title={row.user_id ?? undefined}>
                        {row.user_id || '-'}
                      </div>
                    </td>
                    <td className={TD}>
                      <div className="max-w-[160px] truncate">
                        <ChatUserCell
                          universalId={row.universal_id}
                          chatUsername={row.username}
                          resolveName={resolveName}
                        />
                      </div>
                    </td>
                    <td className={TD}>{row.model ? <Tag tone="primary">{row.model}</Tag> : '-'}</td>
                    <td className={TD}>{row.routed_model ? <Tag tone="info">{row.routed_model}</Tag> : '-'}</td>
                    <td className={TD}>
                      {isErrorCode(row.error_code) ? (
                        <Tag tone="error" mono title={`错误码 ${row.error_code}`}>
                          {row.error_code}
                        </Tag>
                      ) : (
                        <Tag tone="success">OK</Tag>
                      )}
                    </td>
                    <td className={TD_NUM}>{formatNumber(row.prompt_tokens)}</td>
                    <td className={TD_NUM}>{formatNumber(row.completion_tokens)}</td>
                    <td className={TD_NUM}>{fmtMs(row.duration)}</td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </section>

      {/* 行详情弹层（全部字段）。标题不用 id 标识行：ES 源所有行 id=0，改用 request_id。 */}
      <Modal
        open={!!detailRow}
        title={`请求详情 · ${detailRow?.request_id || '-'}`}
        onClose={() => setDetailRow(null)}
        maxWidth={980}
      >
        {detailRow && <RowDetail row={detailRow} resolveName={resolveName} onPreviewLog={previewLog} />}
      </Modal>

      <Modal
        open={logPreview.open}
        title="日志预览"
        onClose={() => setLogPreview({ open: false, loading: false, data: null, error: '', path: '' })}
        maxWidth={1180}
      >
        {logPreview.loading ? (
          <div className="space-y-3">
            <div className="skeleton h-6 rounded-lg" />
            <div className="skeleton h-64 rounded-xl" />
          </div>
        ) : logPreview.error ? (
          <div className="rounded-xl px-4 py-3 text-sm bg-rose-50/70 dark:bg-rose-900/30 text-rose-700 dark:text-rose-300">{logPreview.error}</div>
        ) : logPreview.data ? (
          logPreview.data.previewable ? (
            <LogPreviewContent preview={logPreview.data} fallbackPath={logPreview.path} />
          ) : (
            <div className="space-y-4">
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-x-6 gap-y-3">
                <Field label="文件名" value={logPreview.data.file_name} />
                <Field label="大小" value={formatBytes(getPreviewSizeBytes(logPreview.data, ''))} />
                <Field label="路径" value={logPreview.data.path || logPreview.path} span2 mono />
              </div>
              <div className={`rounded-xl px-4 py-3 text-sm ${logPreview.data.exceeded ? 'bg-amber-50/70 dark:bg-amber-900/30 text-amber-700 dark:text-amber-300' : 'bg-blue-50/70 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300'}`}>
                {logPreview.data.message || '该文件不支持在线预览'}
              </div>
            </div>
          )
        ) : null}
      </Modal>
      </div>
    </SettingsLayout>
  )
}

// ---- 详情弹层 ----

function RowDetail({
  row,
  resolveName,
  onPreviewLog,
}: {
  row: ChatDetailRow
  resolveName: (userId?: string) => string
  onPreviewLog: (localLogPath: string | null | undefined) => void
}) {
  const hasErr = isErrorCode(row.error_code)
  const localLogPath = (row.local_log_path || '').trim()
  return (
    <div className="space-y-5">
      {/* 错误醒目条 */}
      {hasErr && (
        <div className="rounded-xl px-4 py-3 text-sm bg-rose-50/70 dark:bg-rose-900/30 text-rose-700 dark:text-rose-300 font-medium">
          该请求出错，错误码：<span className="font-mono font-bold">{row.error_code}</span>
        </div>
      )}

      <DetailSection title="基础信息">
        <Field label="ID" value={row.id} />
        <Field
          label="状态"
          value={hasErr ? <Tag tone="error" mono>{row.error_code}</Tag> : <Tag tone="success">OK</Tag>}
        />
        <Field label="Request ID" value={row.request_id} span2 mono />
        <Field label="Universal ID" value={row.universal_id} mono />
        <Field label="User ID" value={row.user_id} mono />
        <Field
          label="用户名"
          value={<ChatUserCell universalId={row.universal_id} chatUsername={row.username} resolveName={resolveName} />}
        />
        <Field label="错误码" value={row.error_code} mono />
      </DetailSection>

      <DetailSection title="模型与标签">
        <Field label="Model" value={row.model ? <Tag tone="primary">{row.model}</Tag> : null} />
        <Field label="Routed Model" value={row.routed_model ? <Tag tone="info">{row.routed_model}</Tag> : null} />
        <Field label="Mode" value={row.mode ? <Tag>{row.mode}</Tag> : null} />
        <Field label="Client Version" value={row.client_version} mono />
        <Field label="Task ID" value={row.task_id} span2 mono />
      </DetailSection>

      <DetailSection title="Token 指标">
        <Field label="Prompt Tokens" value={formatNumber(row.prompt_tokens)} />
        <Field label="Completion Tokens" value={formatNumber(row.completion_tokens)} />
        <Field label="Cache Tokens" value={formatNumber(row.cache_tokens)} />
        <Field label="Retry Num" value={formatNumber(row.retry_num)} />
        <Field label="System Tokens" value={formatNumber(row.system_tokens)} />
        <Field label="User Tokens" value={formatNumber(row.user_tokens)} />
        <Field label="Processed System Tokens" value={formatNumber(row.processed_system_tokens)} />
        <Field label="Processed User Tokens" value={formatNumber(row.processed_user_tokens)} />
      </DetailSection>

      <DetailSection title="性能指标">
        <Field label="Duration" value={fmtMs(row.duration)} />
        <Field label="TTFT" value={fmtMs(row.first_token_duration)} />
        <Field label="Slow Chunk" value={formatNumber(row.slow_chunk)} />
        <Field label="Chunk/s" value={fmtFloat(row.chunk_per_second)} />
        <Field label="Token Output Time" value={fmtMs(row.token_output_time)} />
        <Field label="Token Output Speed" value={fmtFloat(row.token_output_speed)} />
        <Field label="Token Output Speed E2E" value={fmtFloat(row.token_output_speed_e2e)} />
      </DetailSection>

      <DetailSection title="时间链路">
        <Field label="TS" value={formatLocalTime(row.ts)} />
        <Field label="Created At" value={row.created_at ? formatLocalTime(row.created_at) : null} />
        <Field label="Request Time" value={row.request_time ? formatLocalTime(row.request_time) : null} />
        <Field label="Forward Request Time" value={row.forward_request_time ? formatLocalTime(row.forward_request_time) : null} />
        <Field label="End Time" value={row.end_time ? formatLocalTime(row.end_time) : null} />
      </DetailSection>

      <DetailSection title="日志">
        <Field
          label="Local Log Path"
          value={
            localLogPath ? (
              <div className="flex flex-wrap items-center gap-2">
                <span className="font-mono break-all">{localLogPath}</span>
                <button type="button" className={BTN_SECONDARY} onClick={() => onPreviewLog(localLogPath)}>
                  预览
                </button>
              </div>
            ) : null
          }
          span2
        />
      </DetailSection>
    </div>
  )
}

function DetailSection({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section>
      <h3 className="text-xs font-semibold text-gray-500 dark:text-gray-400 mb-3">{title}</h3>
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-x-6 gap-y-3">{children}</div>
    </section>
  )
}

function Field({
  label,
  value,
  span2 = false,
  mono = false,
}: {
  label: string
  value: ReactNode
  span2?: boolean
  mono?: boolean
}) {
  const display = value == null || value === '' ? '-' : value
  return (
    <div className={span2 ? 'sm:col-span-2' : ''}>
      <div className="text-xs text-gray-400 dark:text-gray-500 mb-0.5">{label}</div>
      <div className={`text-sm text-gray-800 dark:text-gray-100 ${mono ? 'font-mono break-all' : ''}`}>{display}</div>
    </div>
  )
}


// ---- 格式化辅助函数 ----

const JSON_FORMAT_MAX_BYTES = 1024 * 1024
const LOG_PREVIEW_RENDER_MAX_CHARS = 2 * 1024 * 1024

function formatBytes(bytes: number): string {
  const value = Number(bytes)
  if (!Number.isFinite(value) || value < 0) return '-'
  if (value < 1024) return `${value} B`
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`
  return `${(value / 1024 / 1024).toFixed(2)} MB`
}

function getPreviewSizeBytes(preview: ChatLogPreviewResponse | null, content: string): number {
  const sizeBytes = Number(preview?.size_bytes)
  if (Number.isFinite(sizeBytes) && sizeBytes >= 0) return sizeBytes
  const sizeMB = Number(preview?.size_mb)
  if (Number.isFinite(sizeMB) && sizeMB >= 0) return Math.round(sizeMB * 1024 * 1024)
  return content ? content.length : 0
}

function isJsonPreview(preview: ChatLogPreviewResponse | null, content: string): boolean {
  const name = `${preview?.file_name || ''} ${preview?.path || ''}`.toLowerCase()
  if (name.includes('.json')) return true
  const first = (content || '').trimStart().charAt(0)
  return first === '{' || first === '['
}

// ---- 日志预览内容增强组件 ----

function LogPreviewContent({ preview, fallbackPath }: { preview: ChatLogPreviewResponse; fallbackPath: string }) {
  const [mode, setMode] = useState<'raw' | 'formatted'>('raw')
  const content = preview?.content || ''
  const sizeBytes = getPreviewSizeBytes(preview, content)
  const jsonLike = isJsonPreview(preview, content)
  const canFormatJson = jsonLike && sizeBytes <= JSON_FORMAT_MAX_BYTES

  const formatted = useMemo(() => {
    if (mode !== 'formatted' || !canFormatJson || !content) {
      return { content, error: null as string | null }
    }
    try {
      return {
        content: JSON.stringify(JSON.parse(content), null, 2),
        error: null,
      }
    } catch (e) {
      return {
        content,
        error: `JSON 解析失败，已显示原文：${e instanceof Error ? e.message : String(e)}`,
      }
    }
  }, [canFormatJson, content, mode])

  const displayedContent = formatted.content
  const renderLimited = displayedContent.length > LOG_PREVIEW_RENDER_MAX_CHARS
  const renderedContent = renderLimited
    ? displayedContent.slice(0, LOG_PREVIEW_RENDER_MAX_CHARS)
    : displayedContent
  const isFormatted = mode === 'formatted' && !formatted.error

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(displayedContent)
    } catch {
      // 静默失败
    }
  }

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-x-6 gap-y-3">
        <Field label="文件名" value={preview.file_name || '-'} />
        <Field label="大小" value={formatBytes(sizeBytes)} />
        <Field label="路径" span2 mono value={
          <span
            className="break-all cursor-pointer"
            onClick={() => {
              const path = preview.path || fallbackPath
              navigator.clipboard.writeText(path).catch(() => {})
            }}
            title={preview.path || fallbackPath}
          >
            {preview.path || fallbackPath}
          </span>
        } />
      </div>

      {jsonLike && !canFormatJson && (
        <div className="rounded-xl px-4 py-3 text-sm bg-blue-50/70 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300">
          文件较大，已关闭浏览器端 JSON 格式化；{formatBytes(sizeBytes)} 的内容按原文预览。
        </div>
      )}
      {formatted.error && (
        <div className="rounded-xl px-4 py-3 text-sm bg-amber-50/70 dark:bg-amber-900/30 text-amber-700 dark:text-amber-300">
          {formatted.error}
        </div>
      )}
      {renderLimited && (
        <div className="rounded-xl px-4 py-3 text-sm bg-amber-50/70 dark:bg-amber-900/30 text-amber-700 dark:text-amber-300">
          内容较大，预览区仅渲染前 {formatBytes(LOG_PREVIEW_RENDER_MAX_CHARS)}，避免页面卡顿。
        </div>
      )}

      <div className="rounded-xl overflow-hidden border border-gray-200/50 dark:border-white/10">
        <div className="flex items-center justify-between gap-3 px-3 py-2 bg-white/80 dark:bg-white/5 border-b border-gray-200/50 dark:border-white/10 min-h-[44px]">
          <div className="flex items-center gap-2">
            <span className={`inline-flex items-center rounded-md px-2 py-0.5 text-xs font-medium ${
              jsonLike
                ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300'
                : 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400'
            }`}>
              {jsonLike ? 'JSON' : 'TEXT'}
            </span>
            <span className="text-xs text-gray-400 dark:text-gray-500">
              {isFormatted ? '格式化预览' : '原文预览'}
            </span>
          </div>
          <div className="flex items-center gap-2">
            {jsonLike && (
              <div className="glass rounded-lg p-0.5 inline-flex text-xs">
                <button
                  type="button"
                  onClick={() => setMode('raw')}
                  className={`px-2 py-0.5 rounded-md transition-colors cursor-pointer ${
                    mode === 'raw' ? 'bg-white dark:bg-white/20 text-gray-900 dark:text-white shadow-sm' : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200'
                  }`}
                >
                  原文
                </button>
                <button
                  type="button"
                  disabled={!canFormatJson}
                  onClick={() => setMode('formatted')}
                  className={`px-2 py-0.5 rounded-md transition-colors cursor-pointer ${
                    mode === 'formatted'
                      ? 'bg-white dark:bg-white/20 text-gray-900 dark:text-white shadow-sm'
                      : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200'
                  } disabled:opacity-40 disabled:cursor-not-allowed`}
                >
                  格式化
                </button>
              </div>
            )}
            <button
              type="button"
              onClick={handleCopy}
              className="glass rounded-lg px-2 py-1 text-xs text-gray-600 dark:text-gray-300 cursor-pointer hover:text-apple-blue transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue"
            >
              复制
            </button>
          </div>
        </div>
        {isFormatted ? (
          <JsonFoldingView text={renderedContent} />
        ) : (
          <pre
            className="m-0 px-4 py-3 overflow-auto text-xs leading-relaxed font-mono"
            style={{
              maxHeight: 600,
              background: '#F8FAFC',
              color: '#111827',
              whiteSpace: 'pre',
              wordBreak: 'normal',
              tabSize: 2,
            }}
          >
            {renderedContent}
          </pre>
        )}
      </div>
    </div>
  )
}


// ---- JSON 语法高亮与折叠 ----

interface FoldRegion {
  startLine: number
  endLine: number
  bracket: '{' | '['
}

function findFoldRegions(lines: string[]): FoldRegion[] {
  const regions: FoldRegion[] = []
  const stack: { start: number; indent: number; bracket: '{' | '[' }[] = []

  for (let i = 0; i < lines.length; i++) {
    const m = lines[i].match(/\S/)
    if (!m) continue
    const indent = m.index!
    const trimmed = lines[i].trim()
    if (!trimmed) continue

    const first = trimmed[0]
    if (first === '}' || first === ']') {
      if (stack.length > 0 && stack[stack.length - 1].indent === indent) {
        const top = stack.pop()!
        regions.push({ startLine: top.start, endLine: i, bracket: top.bracket })
      }
      continue
    }

    const last = trimmed[trimmed.length - 1]
    if (last === '{' || last === '[') {
      if (i + 1 < lines.length) {
        const nextM = lines[i + 1].match(/\S/)
        if (nextM && (nextM.index ?? 0) > indent) {
          stack.push({ start: i, indent, bracket: last as '{' | '[' })
        }
      }
    }
  }
  return regions
}

function highlightJsonLine(line: string): React.ReactNode {
  const parts: React.ReactNode[] = []
  let i = 0
  let key = 0

  while (i < line.length) {
    // whitespace
    if (line[i] === ' ') {
      const start = i
      while (i < line.length && line[i] === ' ') i++
      parts.push(line.slice(start, i))
      continue
    }

    // string (handle escaped quotes)
    if (line[i] === '"') {
      const start = i
      i++
      while (i < line.length && line[i] !== '"') {
        if (line[i] === '\\') i++
        i++
      }
      if (i < line.length) i++
      const str = line.slice(start, i)

      // check if this is a property key (followed by :) → check after whitespace
      let j = i
      while (j < line.length && line[j] === ' ') j++
      const isKey = j < line.length && line[j] === ':'

      parts.push(
        <span key={key++} style={{ color: isKey ? '#0550AE' : '#0A3069' }}>
          {str}
        </span>,
      )
      continue
    }

    // : ,
    if (line[i] === ':' || line[i] === ',') {
      parts.push(<span key={key++} style={{ color: '#24292F' }}>{line[i]}</span>)
      i++
      continue
    }

    // brackets
    if (line[i] === '{' || line[i] === '}' || line[i] === '[' || line[i] === ']') {
      parts.push(<span key={key++} style={{ color: '#24292F' }}>{line[i]}</span>)
      i++
      continue
    }

    // number
    if (line[i] === '-' || (line[i] >= '0' && line[i] <= '9')) {
      const start = i
      if (line[i] === '-') i++
      while (i < line.length && line[i] >= '0' && line[i] <= '9') i++
      if (i < line.length && line[i] === '.') {
        i++
        while (i < line.length && line[i] >= '0' && line[i] <= '9') i++
      }
      if (i < line.length && (line[i] === 'e' || line[i] === 'E')) {
        i++
        if (i < line.length && (line[i] === '+' || line[i] === '-')) i++
        while (i < line.length && line[i] >= '0' && line[i] <= '9') i++
      }
      parts.push(<span key={key++} style={{ color: '#0550AE' }}>{line.slice(start, i)}</span>)
      continue
    }

    // true / false / null
    if (line.slice(i, i + 4) === 'true') {
      parts.push(<span key={key++} style={{ color: '#CF222E' }}>true</span>)
      i += 4
      continue
    }
    if (line.slice(i, i + 5) === 'false') {
      parts.push(<span key={key++} style={{ color: '#CF222E' }}>false</span>)
      i += 5
      continue
    }
    if (line.slice(i, i + 4) === 'null') {
      parts.push(<span key={key++} style={{ color: '#CF222E' }}>null</span>)
      i += 4
      continue
    }

    // fallback: emit one char
    parts.push(line[i])
    i++
  }

  return parts.length > 0 ? <>{parts}</> : line
}

function JsonFoldingView({ text }: { text: string }) {
  const [foldedLines, setFoldedLines] = useState<Set<number>>(new Set())
  const lines = useMemo(() => text.split('\n'), [text])
  const regions = useMemo(() => findFoldRegions(lines), [lines])

  const toggleFold = useCallback((startLine: number) => {
    setFoldedLines((prev) => {
      const next = new Set(prev)
      if (next.has(startLine)) next.delete(startLine)
      else next.add(startLine)
      return next
    })
  }, [])

  const regionByStart = useMemo(() => new Map(regions.map((r) => [r.startLine, r])), [regions])

  // lines hidden by a fold (between start+1 and end-1 inclusive)
  const hiddenLines = useMemo(() => {
    const h = new Set<number>()
    for (const r of regions) {
      if (foldedLines.has(r.startLine)) {
        for (let i = r.startLine + 1; i < r.endLine; i++) h.add(i)
      }
    }
    return h
  }, [regions, foldedLines])

  return (
    <div
      className="overflow-auto font-mono text-xs leading-relaxed"
      style={{ maxHeight: 600, background: '#F8FAFC', tabSize: 2 }}
    >
      {lines.map((line, idx) => {
        if (hiddenLines.has(idx)) return null

        const region = regionByStart.get(idx)
        const isFolded = region && foldedLines.has(idx)
        const foldCount = region ? region.endLine - region.startLine - 1 : 0

        return (
          <div
            key={idx}
            className="flex hover:bg-black/[0.02] dark:hover:bg-white/[0.04]"
          >
            {/* gutter (fold button) */}
            <div
              className="flex-none w-5 text-center select-none leading-relaxed"
              style={{ userSelect: 'none' }}
            >
              {region && (
                <button
                  type="button"
                  onClick={() => toggleFold(idx)}
                  className="inline-flex items-center justify-center w-4 h-4 text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 cursor-pointer bg-transparent border-none p-0 text-xs leading-none"
                  title={isFolded ? '展开' : '折叠'}
                  aria-label={isFolded ? '展开' : '折叠'}
                >
                  {isFolded ? '\u25B6' : '\u25BC'}
                </button>
              )}
            </div>
            {/* line content */}
            <div className="flex-1 whitespace-pre">
              {isFolded ? (
                <span
                  className="cursor-pointer text-gray-400 dark:text-gray-500"
                  onClick={() => toggleFold(idx)}
                  title={`${foldCount} 行`}
                >
                  {highlightJsonLine(line)}
                  {'  // +'}{foldCount}{' items'}
                </span>
              ) : (
                highlightJsonLine(line)
              )}
            </div>
          </div>
        )
      })}
    </div>
  )
}
