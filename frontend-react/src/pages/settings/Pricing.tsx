// 设置·模型价格：model_pricing CRUD（对照 chat 侧 Pricing.jsx 交互逻辑，玻璃拟态重写）。
// per-token 单价存储为「每 token」，界面统一按「每 1M tokens」展示/录入（×/÷ 1_000_000）。
// 非系统币种按汇率换算为系统币种存储（换算在 chat 后端做，前端只传 currency + exchange_rate + 原币种单价）。
import { useEffect, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { chatStats } from '@/api/endpoints'
import { useChatPricing, useChatSystemConfig } from '@/api/queries'
import type { ModelPricing, ModelPricingUpsert } from '@/api/types'
import { Modal } from '@/components/ui/Modal'
import { Tag, type TagTone } from '@/components/ui/Tag'
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
import { currencySymbol as sym } from '@/lib/formatters'

// 1 token ↔ 1M tokens 转换因子
const M = 1_000_000

const CURRENCY_OPTIONS = ['CNY', 'USD', 'EUR', 'GBP', 'JPY']

const MODE_OPTIONS = [
  { value: 'token', label: 'Token 计价' },
  { value: 'request', label: '请求次数计价' },
  { value: 'hybrid', label: '混合计价' },
]
const MODE_TONE: Record<string, TagTone> = { token: 'primary', request: 'warning', hybrid: 'info' }

/** per-token 单价 → 每 1M tokens 显示。 */
function fmtPerM(v: number | null | undefined, currency: string): string {
  if (v == null) return '-'
  return `${sym(currency)}${(v * M).toFixed(4)}`
}

/** date 字段防御性取 YYYY-MM-DD（后端可能回 ISO 带时间）。 */
function dateOnly(v: string | null | undefined): string {
  return (v || '').slice(0, 10)
}

/** 计费公式说明（对照 chat 侧 buildFormulaDetail）。 */
function buildFormulaDetail(r: ModelPricing): { title: string; formula: string; note: string } {
  const hasCache = r.cache_price_per_token != null && r.cache_price_per_token > 0
  if (r.pricing_mode === 'token') {
    if (hasCache) {
      return {
        title: 'Token 计价（含缓存折扣）',
        formula:
          '费用 = (prompt_tokens − cache_tokens) × 输入单价\n    + cache_tokens × 缓存单价\n    + completion_tokens × 输出单价',
        note: '缓存命中的 token 享受折扣价（通常为输入价 50% 或更低），非缓存命中部分按正常输入单价计费。',
      }
    }
    return {
      title: 'Token 计价（无缓存折扣）',
      formula: '费用 = prompt_tokens × 输入单价\n    + completion_tokens × 输出单价',
      note: '缓存单价为 0 或未设置，所有输入 token 均按输入单价全价计费。',
    }
  }
  if (r.pricing_mode === 'request') {
    return {
      title: '按请求次数计价',
      formula: '费用 = 请求次数 × 每次请求单价',
      note: '每次调用按固定单价计费，与 token 消耗量无关。',
    }
  }
  if (r.pricing_mode === 'hybrid') {
    return {
      title: hasCache ? '混合计价（Token + 请求次数，含缓存折扣）' : '混合计价（Token + 请求次数）',
      formula: hasCache
        ? '费用 = (prompt_tokens − cache_tokens) × 输入单价\n    + cache_tokens × 缓存单价\n    + completion_tokens × 输出单价\n    + 请求次数 × 每次请求单价'
        : '费用 = prompt_tokens × 输入单价\n    + completion_tokens × 输出单价\n    + 请求次数 × 每次请求单价',
      note: '同时按 token 消耗和请求次数两种维度计费。',
    }
  }
  return { title: '未知计价方式', formula: '-', note: '' }
}

export default function Pricing() {
  const enabled = useChatEnabled()
  const queryClient = useQueryClient()
  const { data: rows, isLoading, error } = useChatPricing(enabled)
  const { data: sysCfg } = useChatSystemConfig(enabled)

  const systemCurrency = sysCfg?.system_currency || 'CNY'
  const defaultRate = Number(sysCfg?.default_exchange_rate) > 0 ? Number(sysCfg?.default_exchange_rate) : 7.242

  // 新增/编辑弹层（editing=null 表示新增）
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<ModelPricing | null>(null)
  // 详情（计费公式）弹层
  const [detailRecord, setDetailRecord] = useState<ModelPricing | null>(null)
  // 删除确认
  const [pendingDelete, setPendingDelete] = useState<ModelPricing | null>(null)
  const [deleting, setDeleting] = useState(false)
  const [deleteErr, setDeleteErr] = useState('')

  async function invalidate() {
    await queryClient.invalidateQueries({ queryKey: ['chat-pricing'] })
  }

  async function handleSubmit(payload: ModelPricingUpsert) {
    if (editing) await chatStats.updatePricing(editing.id, payload)
    else await chatStats.createPricing(payload)
    setModalOpen(false)
    await invalidate()
  }

  async function confirmDelete() {
    if (!pendingDelete) return
    setDeleting(true)
    setDeleteErr('')
    try {
      await chatStats.deletePricing(pendingDelete.id)
      setPendingDelete(null)
      await invalidate()
    } catch (e: unknown) {
      setDeleteErr(e instanceof Error ? e.message : '删除失败')
    } finally {
      setDeleting(false)
    }
  }

  const list = rows || []

  return (
    <SettingsLayout>
      <section className="glass rounded-2xl overflow-hidden">
        <div className="flex flex-wrap items-center justify-between gap-2 px-5 py-3 border-b border-gray-200/50 dark:border-white/10">
          <span className="text-sm font-semibold text-gray-700 dark:text-gray-200">模型价格（{list.length}）</span>
          <div className="flex items-center gap-3">
            <span className="text-xs text-gray-400 dark:text-gray-500 hidden sm:inline">
              非 {systemCurrency} 货币按汇率换算为 {systemCurrency} 存储，原始价格保留参考
            </span>
            <button type="button" onClick={() => { setEditing(null); setModalOpen(true) }} className={BTN_PRIMARY}>
              新增价格
            </button>
          </div>
        </div>

        <div className="px-5 py-3 text-xs leading-5 text-emerald-700 dark:text-emerald-300 bg-emerald-50/60 dark:bg-emerald-900/20 border-b border-gray-200/50 dark:border-white/10">
          <strong>计价逻辑说明（点击「详情」查看该模型的完整公式）：</strong>
          <ul className="list-disc pl-5 mt-1 space-y-0.5">
            <li>token：<code>prompt_tokens × 输入单价 + completion_tokens × 输出单价</code>（设缓存单价后缓存命中部分按折扣价）</li>
            <li>request：<code>请求次数 × 每次请求单价</code></li>
            <li>hybrid：token 部分成本 + <code>请求次数 × 每次请求单价</code></li>
          </ul>
        </div>

        {error && (
          <div className="px-5 py-2 text-sm text-rose-600 dark:text-rose-400 bg-rose-50/50 dark:bg-rose-900/20">
            {(error as Error).message || '获取价格列表失败'}
          </div>
        )}

        <div className="overflow-x-auto">
          <table className="w-full text-sm border-collapse">
            <thead>
              <tr className="border-b border-gray-200/50 dark:border-white/10">
                <th className={`${TH} min-w-[160px]`}>模型名</th>
                <th className={TH}>计价模式</th>
                <th className={TH_NUM}>输入单价（{sym(systemCurrency)}/1M）</th>
                <th className={TH_NUM}>输出单价（{sym(systemCurrency)}/1M）</th>
                <th className={TH_NUM}>缓存单价（{sym(systemCurrency)}/1M）</th>
                <th className={TH_NUM}>请求单价</th>
                <th className={TH}>生效日期</th>
                <th className={TH}>失效日期</th>
                <th className={TH}>原始货币</th>
                <th className={`${TH} min-w-[120px]`}>备注</th>
                <th className={`${TH} text-center`}>操作</th>
              </tr>
            </thead>
            <tbody>
              {isLoading || !enabled ? (
                Array.from({ length: 4 }).map((_, i) => (
                  <tr key={i} className="border-b border-gray-100/50 dark:border-white/5">
                    <td className={TD} colSpan={11}>
                      <div className="skeleton h-6 rounded" />
                    </td>
                  </tr>
                ))
              ) : list.length === 0 ? (
                <tr>
                  <td colSpan={11}>
                    <div className="py-12 text-center text-sm text-gray-400 dark:text-gray-500">
                      暂无价格数据，点击「新增价格」开始
                    </div>
                  </td>
                </tr>
              ) : (
                list.map((r) => (
                  <tr key={r.id} className="border-b border-gray-100/50 dark:border-white/5 hover:bg-apple-blue/5 dark:hover:bg-white/5 transition-colors">
                    <td className={TD}>
                      <span className="font-medium text-gray-900 dark:text-white">{r.model_name}</span>
                    </td>
                    <td className={TD}>
                      <Tag tone={MODE_TONE[r.pricing_mode] || 'neutral'}>{r.pricing_mode}</Tag>
                    </td>
                    <td className={TD_NUM}>{fmtPerM(r.input_price_per_token, systemCurrency)}</td>
                    <td className={TD_NUM}>{fmtPerM(r.output_price_per_token, systemCurrency)}</td>
                    <td className={TD_NUM}>
                      {r.cache_price_per_token == null
                        ? '-'
                        : r.cache_price_per_token === 0
                          ? <span className="text-gray-400">{sym(systemCurrency)}0（不启用）</span>
                          : fmtPerM(r.cache_price_per_token, systemCurrency)}
                    </td>
                    <td className={TD_NUM}>{r.request_price != null ? `${sym(systemCurrency)}${r.request_price}` : '-'}</td>
                    <td className={TD}>{dateOnly(r.effective_date) || '-'}</td>
                    <td className={TD}>
                      {r.end_date ? dateOnly(r.end_date) : <Tag tone="success">永久有效</Tag>}
                    </td>
                    <td className={TD}>
                      {r.original_currency ? (
                        <span title={`原始货币 ${r.original_currency}，汇率 ${r.exchange_rate ?? '-'}`}>
                          <Tag tone="primary">{r.original_currency}</Tag>
                          <span className="text-xs text-gray-400 ml-1">汇率 {r.exchange_rate ?? '-'}</span>
                        </span>
                      ) : (
                        <span className="text-gray-400">-</span>
                      )}
                    </td>
                    <td className={TD}>
                      <div className="max-w-[160px] truncate text-gray-500 dark:text-gray-400" title={r.notes || ''}>
                        {r.notes || '-'}
                      </div>
                    </td>
                    <td className="px-3 py-2 align-middle text-center whitespace-nowrap">
                      <div className="inline-flex items-center gap-2">
                        <button type="button" className={LINK_BTN} onClick={() => setDetailRecord(r)}>详情</button>
                        <button type="button" className={LINK_BTN} onClick={() => { setEditing(r); setModalOpen(true) }}>编辑</button>
                        <button type="button" className={LINK_BTN_DANGER} onClick={() => setPendingDelete(r)}>删除</button>
                      </div>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </section>

      <PricingModal
        open={modalOpen}
        editing={editing}
        systemCurrency={systemCurrency}
        defaultRate={defaultRate}
        onClose={() => setModalOpen(false)}
        onSubmit={handleSubmit}
      />

      {/* 计费公式详情 */}
      <Modal
        open={!!detailRecord}
        title={`模型价格详情 — ${detailRecord?.model_name || ''}`}
        maxWidth={560}
        onClose={() => setDetailRecord(null)}
        footer={
          <button type="button" className={BTN_GLASS} onClick={() => setDetailRecord(null)}>关闭</button>
        }
      >
        {detailRecord && (() => {
          const d = buildFormulaDetail(detailRecord)
          return (
            <div className="space-y-3 text-sm">
              <dl className="grid grid-cols-2 gap-x-4 gap-y-2">
                <DetailItem label="计价模式"><Tag tone={MODE_TONE[detailRecord.pricing_mode] || 'neutral'}>{detailRecord.pricing_mode}</Tag></DetailItem>
                <DetailItem label="货币">
                  {detailRecord.original_currency
                    ? `${detailRecord.original_currency} → ${systemCurrency}（汇率 ${detailRecord.exchange_rate ?? '-'}）`
                    : systemCurrency}
                </DetailItem>
                <DetailItem label={`输入单价（${sym(systemCurrency)}/1M）`}>{fmtPerM(detailRecord.input_price_per_token, systemCurrency)}</DetailItem>
                <DetailItem label={`输出单价（${sym(systemCurrency)}/1M）`}>{fmtPerM(detailRecord.output_price_per_token, systemCurrency)}</DetailItem>
                <DetailItem label={`缓存单价（${sym(systemCurrency)}/1M）`}>
                  {detailRecord.cache_price_per_token == null
                    ? '-'
                    : detailRecord.cache_price_per_token === 0
                      ? `${sym(systemCurrency)}0（不启用折扣）`
                      : fmtPerM(detailRecord.cache_price_per_token, systemCurrency)}
                </DetailItem>
                <DetailItem label="请求单价">
                  {detailRecord.request_price != null ? `${sym(systemCurrency)}${detailRecord.request_price}` : '-'}
                </DetailItem>
                {detailRecord.original_currency && (
                  <>
                    <DetailItem label={`原始输入价（${detailRecord.original_currency}/1M）`}>
                      {fmtPerM(detailRecord.original_input_price, detailRecord.original_currency)}
                    </DetailItem>
                    <DetailItem label={`原始输出价（${detailRecord.original_currency}/1M）`}>
                      {fmtPerM(detailRecord.original_output_price, detailRecord.original_currency)}
                    </DetailItem>
                  </>
                )}
                <DetailItem label="生效日期">{dateOnly(detailRecord.effective_date) || '-'}</DetailItem>
                <DetailItem label="失效日期">{detailRecord.end_date ? dateOnly(detailRecord.end_date) : '永久有效'}</DetailItem>
                {detailRecord.notes && (
                  <div className="col-span-2">
                    <DetailItem label="备注">{detailRecord.notes}</DetailItem>
                  </div>
                )}
              </dl>
              <div className="glass rounded-xl p-4">
                <div className="font-semibold text-gray-900 dark:text-white mb-2">{d.title}</div>
                <pre className="text-xs whitespace-pre-wrap rounded-lg bg-gray-100/70 dark:bg-white/5 p-3 text-apple-blue mb-2">{d.formula}</pre>
                {d.note && <p className="text-xs text-gray-500 dark:text-gray-400">{d.note}</p>}
                <p className="text-xs text-gray-400 dark:text-gray-500 mt-2">
                  字段说明：prompt_tokens = 输入总 token（含缓存命中部分），cache_tokens = 缓存命中 token，completion_tokens = 输出 token
                </p>
              </div>
            </div>
          )
        })()}
      </Modal>

      {/* 删除确认 */}
      <Modal
        open={!!pendingDelete}
        title="确认删除"
        maxWidth={420}
        onClose={() => setPendingDelete(null)}
        footer={
          <>
            <button type="button" className={BTN_GLASS} onClick={() => setPendingDelete(null)}>取消</button>
            <button type="button" className={BTN_DANGER} disabled={deleting} onClick={confirmDelete}>
              {deleting ? '删除中...' : '删除'}
            </button>
          </>
        }
      >
        <div className="space-y-2">
          {deleteErr && <div className="text-sm text-rose-600 dark:text-rose-400">{deleteErr}</div>}
          <p className="text-sm text-gray-700 dark:text-gray-200">
            确定要删除「{pendingDelete?.model_name}」（生效日期 {dateOnly(pendingDelete?.effective_date)}）的价格记录吗？此操作不可撤销。
          </p>
        </div>
      </Modal>
    </SettingsLayout>
  )
}

function DetailItem({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <dt className="text-xs text-gray-500 dark:text-gray-400">{label}</dt>
      <dd className="text-sm text-gray-900 dark:text-white mt-0.5">{children}</dd>
    </div>
  )
}

// ---- 新增/编辑弹层 ----

function PricingModal({
  open,
  editing,
  systemCurrency,
  defaultRate,
  onClose,
  onSubmit,
}: {
  open: boolean
  editing: ModelPricing | null
  systemCurrency: string
  defaultRate: number
  onClose: () => void
  onSubmit: (payload: ModelPricingUpsert) => Promise<void>
}) {
  const [modelName, setModelName] = useState('')
  const [mode, setMode] = useState('token')
  const [currency, setCurrency] = useState(systemCurrency)
  const [exchangeRate, setExchangeRate] = useState('')
  // 单价均按「每 1M tokens」录入（字符串态，提交时 ÷ M）
  const [inputM, setInputM] = useState('')
  const [outputM, setOutputM] = useState('')
  const [cacheM, setCacheM] = useState('')
  const [requestPrice, setRequestPrice] = useState('')
  const [effectiveDate, setEffectiveDate] = useState('')
  const [endDate, setEndDate] = useState('')
  const [notes, setNotes] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [err, setErr] = useState('')

  useEffect(() => {
    if (!open) return
    setErr('')
    if (!editing) {
      setModelName('')
      setMode('token')
      setCurrency(systemCurrency)
      setExchangeRate(String(defaultRate))
      setInputM('')
      setOutputM('')
      setCacheM('')
      setRequestPrice('')
      setEffectiveDate('')
      setEndDate('')
      setNotes('')
      return
    }
    // 编辑：有原始货币按原始价格回填（所见即所录），否则回填系统币种价格
    setModelName(editing.model_name)
    setMode(editing.pricing_mode || 'token')
    const cur = editing.original_currency || systemCurrency
    setCurrency(cur)
    setExchangeRate(editing.exchange_rate != null ? String(editing.exchange_rate) : String(defaultRate))
    const inp = editing.original_currency ? editing.original_input_price : editing.input_price_per_token
    const out = editing.original_currency ? editing.original_output_price : editing.output_price_per_token
    const cache = editing.original_currency ? editing.original_cache_price : editing.cache_price_per_token
    const req = editing.original_currency ? editing.original_request_price : editing.request_price
    setInputM(inp != null ? String(inp * M) : '')
    setOutputM(out != null ? String(out * M) : '')
    setCacheM(cache != null ? String(cache * M) : '')
    setRequestPrice(req != null ? String(req) : '')
    setEffectiveDate(dateOnly(editing.effective_date))
    setEndDate(editing.end_date ? dateOnly(editing.end_date) : '')
    setNotes(editing.notes || '')
  }, [open, editing, systemCurrency, defaultRate])

  const showToken = mode === 'token' || mode === 'hybrid'
  const showRequest = mode === 'request' || mode === 'hybrid'
  const isNonSystemCurrency = currency !== systemCurrency

  function numOrNull(s: string): number | null {
    const t = s.trim()
    if (t === '') return null
    const n = Number(t)
    return Number.isFinite(n) ? n : null
  }

  async function handleSubmit() {
    const name = modelName.trim()
    if (!name) {
      setErr('请输入模型名')
      return
    }
    if (!effectiveDate) {
      setErr('请选择生效日期')
      return
    }
    const rate = numOrNull(exchangeRate)
    if (isNonSystemCurrency && (rate == null || rate <= 0)) {
      setErr('非系统币种必须填写有效汇率')
      return
    }
    const inputPerM = showToken ? numOrNull(inputM) : null
    const outputPerM = showToken ? numOrNull(outputM) : null
    const cachePerM = showToken ? numOrNull(cacheM) : null

    const payload: ModelPricingUpsert = {
      model_name: name,
      pricing_mode: mode,
      input_price_per_token: inputPerM != null ? inputPerM / M : null,
      output_price_per_token: outputPerM != null ? outputPerM / M : null,
      cache_price_per_token: cachePerM != null ? cachePerM / M : null,
      request_price: showRequest ? numOrNull(requestPrice) : null,
      currency,
      exchange_rate: isNonSystemCurrency ? rate : null,
      // 原始价格由 chat 后端按 currency+exchange_rate 换算时生成，前端不传
      original_currency: null,
      original_input_price: null,
      original_output_price: null,
      original_cache_price: null,
      original_request_price: null,
      effective_date: effectiveDate,
      end_date: endDate ? endDate : null,
      notes: notes.trim() ? notes.trim() : null,
    }

    setSubmitting(true)
    setErr('')
    try {
      await onSubmit(payload)
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : '保存失败')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Modal
      open={open}
      title={editing ? `修改模型价格 — ${editing.model_name}` : '新增模型价格'}
      maxWidth={560}
      onClose={onClose}
      footer={
        <>
          <button type="button" className={BTN_GLASS} onClick={onClose}>取消</button>
          <button type="button" className={BTN_PRIMARY} disabled={submitting} onClick={handleSubmit}>
            {submitting ? '保存中...' : '保存'}
          </button>
        </>
      }
    >
      <div className="space-y-3">
        {err && <div className="text-sm text-rose-600 dark:text-rose-400">{err}</div>}
        <Field label="模型名">
          <input type="text" value={modelName} onChange={(e) => setModelName(e.target.value)} placeholder="如 deepseek-v3" className={INPUT_CLS} />
        </Field>
        <Field label="计价方案">
          <select value={mode} onChange={(e) => setMode(e.target.value)} className={INPUT_CLS}>
            {MODE_OPTIONS.map((o) => (
              <option key={o.value} value={o.value}>{o.label}</option>
            ))}
          </select>
        </Field>
        <div className="grid grid-cols-2 gap-3">
          <Field label="原始货币">
            <select value={currency} onChange={(e) => setCurrency(e.target.value)} className={INPUT_CLS}>
              {CURRENCY_OPTIONS.map((c) => (
                <option key={c} value={c}>{c}</option>
              ))}
            </select>
          </Field>
          {isNonSystemCurrency && (
            <Field label={`1 ${currency} 兑换 ${systemCurrency} 汇率`} hint={`系统默认：${defaultRate}`}>
              <input type="number" step="0.0001" min="0.0001" value={exchangeRate} onChange={(e) => setExchangeRate(e.target.value)} className={INPUT_CLS} />
            </Field>
          )}
        </div>

        {showToken && (
          <>
            <Field label={`输入 Token 单价（${sym(currency)}/1M tokens）`} hint="用于 prompt_tokens 中非缓存命中部分的计费">
              <input type="number" step="0.01" min="0" value={inputM} onChange={(e) => setInputM(e.target.value)} placeholder="如 2.00（每百万 tokens）" className={INPUT_CLS} />
            </Field>
            <Field label={`输出 Token 单价（${sym(currency)}/1M tokens）`} hint="用于 completion_tokens 的计费">
              <input type="number" step="0.01" min="0" value={outputM} onChange={(e) => setOutputM(e.target.value)} placeholder="如 8.00（每百万 tokens）" className={INPUT_CLS} />
            </Field>
            <Field
              label={`缓存 Token 单价（${sym(currency)}/1M tokens）`}
              hint="缓存命中 token 按此折扣价计费；留空或填 0 则不启用折扣，按输入单价全价计费"
            >
              <input type="number" step="0.01" min="0" value={cacheM} onChange={(e) => setCacheM(e.target.value)} placeholder="如 0.50（可选，输入价的 50%）" className={INPUT_CLS} />
            </Field>
          </>
        )}

        {showRequest && (
          <Field label={`每次请求单价（${sym(currency)}）`}>
            <input type="number" step="0.01" value={requestPrice} onChange={(e) => setRequestPrice(e.target.value)} placeholder="如 0.05" className={INPUT_CLS} />
          </Field>
        )}

        <div className="grid grid-cols-2 gap-3">
          <Field label="生效日期">
            <input type="date" value={effectiveDate} onChange={(e) => setEffectiveDate(e.target.value)} className={INPUT_CLS} />
          </Field>
          <Field label="失效日期" hint="留空表示永久有效">
            <input type="date" value={endDate} onChange={(e) => setEndDate(e.target.value)} className={INPUT_CLS} />
          </Field>
        </div>

        <Field label="备注">
          <textarea rows={2} value={notes} onChange={(e) => setNotes(e.target.value)} placeholder="价格来源、变更说明" className={`${INPUT_CLS} resize-y`} />
        </Field>

        {isNonSystemCurrency && (
          <div className="text-xs rounded-lg px-3 py-2 bg-amber-50/70 dark:bg-amber-900/20 text-amber-700 dark:text-amber-300">
            价格将自动按汇率 <strong>{exchangeRate || defaultRate}</strong> 将 {currency} 换算为 {systemCurrency} 存储并用于计算，原始价格保留参考。
          </div>
        )}
      </div>
    </Modal>
  )
}
