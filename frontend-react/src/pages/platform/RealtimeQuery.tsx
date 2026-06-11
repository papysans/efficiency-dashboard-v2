// 平台·明细点查 —— chat-indicator-statistics /stats/detail/query 的玻璃拟态重写（现场排查核心）。
// 一次性查询（无分页），SQL 层过滤，最多 100 条；行点击弹详情（全部字段，错误码醒目）。
// 时间发送本地壁钟时间 + 浏览器实际时区偏移（chat 侧按 RFC3339 解析，避免硬编码 +08:00）。
// datasource_id 不传 = 服务端自动取第一个启用的数据源（内网单源）。
import { useState, type ReactNode } from 'react'
import { useMutation } from '@tanstack/react-query'
import { chatStats } from '@/api/endpoints'
import { useGlobalConfig } from '@/api/queries'
import type { ChatDetailQueryReq, ChatDetailRow } from '@/api/types'
import { useUserNameMap } from '@/hooks/useUserNameMap'
import { Modal } from '@/components/ui/Modal'
import { Tag } from '@/components/ui/Tag'
import { formatLocalTime, formatNumber } from '@/lib/formatters'
import { ChatDisabledNotice } from '@/pages/settings/SettingsLayout'
import { ChatUserCell, isErrorCode, PlatformTabs } from './platformShared'

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
  start: string
  end: string
  universalId: string
  requestId: string
  model: string
  routedModel: string
  /** '' = 全部，'true' = 仅错误，'false' = 仅成功 */
  hasError: '' | 'true' | 'false'
  limit: number
  order: 'desc' | 'asc'
}

function defaultForm(): QueryForm {
  const end = new Date()
  const start = new Date(end.getTime() - 30 * 60_000)
  return {
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

const fmtMs = (v: number | null | undefined) => (v != null ? `${Number(v).toFixed(0)} ms` : '-')

export default function RealtimeQuery() {
  // 开关语义与设置区/态势页一致：未启用时整页提示，不让表单提交打到 503 的代理。
  const { data: gc } = useGlobalConfig()
  const chatDisabled = !!gc && gc.chat_stats_enabled !== true

  const [form, setForm] = useState<QueryForm>(defaultForm)
  const [validateMsg, setValidateMsg] = useState('')
  const [detailRow, setDetailRow] = useState<ChatDetailRow | null>(null)
  // universal_id 与看板 user_id 同源 → 结果表/详情弹层解析看板用户名并互链（失败自动回退）。
  const { resolveName } = useUserNameMap()

  // queries.ts 无明细点查 hook（按需触发的点查不适合 useQuery 缓存语义），页面内局部 useMutation。
  const query = useMutation({
    mutationFn: (body: ChatDetailQueryReq) => chatStats.queryDetail(body),
  })

  function setField<K extends keyof QueryForm>(key: K, value: QueryForm[K]) {
    setForm((f) => ({ ...f, [key]: value }))
  }

  function applyQuickRange(minutes: number) {
    const end = new Date()
    const start = new Date(end.getTime() - minutes * 60_000)
    setForm((f) => ({ ...f, start: toLocalInputValue(start), end: toLocalInputValue(end) }))
  }

  function submit() {
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
      start_time: toIsoWithOffset(form.start),
      end_time: toIsoWithOffset(form.end),
      limit: form.limit,
      order: form.order,
    }
    const universalId = form.universalId.trim()
    const requestId = form.requestId.trim()
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
    setForm(defaultForm())
    setValidateMsg('')
  }

  function onFieldKey(e: React.KeyboardEvent) {
    if (e.key === 'Enter') submit()
  }

  const rows = query.data?.items ?? []
  const total = query.data?.total ?? 0

  const header = (
    <header className="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white">明细查询</h1>
        <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
          按条件点查 LLM 请求明细（直查源库，最多返回 100 条），用于现场排查。
        </p>
      </div>
      <PlatformTabs />
    </header>
  )

  if (chatDisabled) {
    return (
      <div className="space-y-5">
        {header}
        <ChatDisabledNotice />
      </div>
    )
  }

  return (
    <div className="space-y-5">
      {header}

      {/* 查询条件 */}
      <section className="glass rounded-2xl p-5 space-y-3">
        <div className="flex flex-wrap items-center gap-2">
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
            disabled={query.isPending}
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
                    <td className={TD} colSpan={10}>
                      <div className="skeleton h-6 rounded" />
                    </td>
                  </tr>
                ))
              ) : rows.length === 0 ? (
                <tr>
                  <td colSpan={10}>
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
        maxWidth={760}
      >
        {detailRow && <RowDetail row={detailRow} resolveName={resolveName} />}
      </Modal>
    </div>
  )
}

// ---- 详情弹层 ----

function RowDetail({ row, resolveName }: { row: ChatDetailRow; resolveName: (userId?: string) => string }) {
  const hasErr = isErrorCode(row.error_code)
  return (
    <div className="space-y-4">
      {/* 错误醒目条 */}
      {hasErr && (
        <div className="rounded-xl px-4 py-3 text-sm bg-rose-50/70 dark:bg-rose-900/30 text-rose-700 dark:text-rose-300 font-medium">
          该请求出错，错误码：<span className="font-mono font-bold">{row.error_code}</span>
        </div>
      )}
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-x-6 gap-y-3">
        <Field label="ID" value={row.id} />
        <Field label="时间" value={formatLocalTime(row.ts)} />
        <Field label="Request ID" value={row.request_id} span2 mono />
        <Field label="Universal ID" value={row.universal_id} mono />
        <Field
          label="用户名"
          value={<ChatUserCell universalId={row.universal_id} chatUsername={row.username} resolveName={resolveName} />}
        />
        <Field label="User ID" value={row.user_id} span2 mono />
        <Field label="Model" value={row.model ? <Tag tone="primary">{row.model}</Tag> : null} />
        <Field label="Routed Model" value={row.routed_model ? <Tag tone="info">{row.routed_model}</Tag> : null} />
        <Field label="Mode" value={row.mode ? <Tag>{row.mode}</Tag> : null} />
        <Field
          label="错误码"
          value={hasErr ? <Tag tone="error" mono>{row.error_code}</Tag> : <Tag tone="success">OK</Tag>}
        />
        <Field label="Prompt Tokens" value={formatNumber(row.prompt_tokens)} />
        <Field label="Completion Tokens" value={formatNumber(row.completion_tokens)} />
        <Field label="Cache Tokens" value={formatNumber(row.cache_tokens)} />
        <Field label="Slow Chunk" value={row.slow_chunk} />
        <Field label="总耗时" value={fmtMs(row.duration)} />
        <Field label="首 Token 时延（TTFT）" value={fmtMs(row.first_token_duration)} />
        <Field label="System Tokens" value={formatNumber(row.system_tokens)} />
        <Field label="User Tokens" value={formatNumber(row.user_tokens)} />
        <Field label="Request Time" value={row.request_time ? formatLocalTime(row.request_time) : null} />
        <Field label="End Time" value={row.end_time ? formatLocalTime(row.end_time) : null} />
      </div>
    </div>
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
