// 设置·系统配置：chat 服务 KV 配置（定时 ETL 开关/cron/绑定数据源、系统币种、默认汇率）。
// 后端是扁平 string→string KV（PUT /config FirstOrCreate 逐键 upsert），布尔/数字均以字符串存取。
import { useEffect, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { chatStats } from '@/api/endpoints'
import { useChatDatasources, useChatSystemConfig } from '@/api/queries'
import SettingsLayout, { BTN_PRIMARY, Field, INPUT_CLS, useChatEnabled } from './SettingsLayout'

const CURRENCY_OPTIONS = [
  { value: 'CNY', label: 'CNY（人民币）' },
  { value: 'USD', label: 'USD（美元）' },
  { value: 'EUR', label: 'EUR（欧元）' },
  { value: 'GBP', label: 'GBP（英镑）' },
  { value: 'JPY', label: 'JPY（日元）' },
]

export default function SystemConfig() {
  const enabled = useChatEnabled()
  const queryClient = useQueryClient()
  const { data: cfg, isLoading, error } = useChatSystemConfig(enabled)
  const { data: datasources } = useChatDatasources(enabled)

  const [etlEnabled, setEtlEnabled] = useState(false)
  const [cron, setCron] = useState('0 2 * * *')
  const [etlSource, setEtlSource] = useState('')
  const [currency, setCurrency] = useState('CNY')
  const [rate, setRate] = useState('7.2420')

  const [saving, setSaving] = useState(false)
  const [msg, setMsg] = useState<{ ok: boolean; text: string } | null>(null)

  // 配置加载后回填表单
  useEffect(() => {
    if (!cfg) return
    setEtlEnabled(cfg.daily_etl_enabled === 'true')
    setCron(cfg.daily_etl_cron || '0 2 * * *')
    setEtlSource(cfg.daily_etl_source || '')
    setCurrency(cfg.system_currency || 'CNY')
    setRate(cfg.default_exchange_rate || '7.2420')
  }, [cfg])

  async function handleSave() {
    const rateNum = Number(rate.trim())
    if (!Number.isFinite(rateNum) || rateNum <= 0) {
      setMsg({ ok: false, text: '默认汇率必须为正数' })
      return
    }
    if (etlEnabled && !etlSource) {
      setMsg({ ok: false, text: '启用定时 ETL 时必须选择绑定数据源' })
      return
    }
    setSaving(true)
    setMsg(null)
    try {
      await chatStats.updateConfig({
        daily_etl_enabled: etlEnabled ? 'true' : 'false',
        daily_etl_cron: cron.trim(),
        daily_etl_source: etlSource,
        system_currency: currency,
        default_exchange_rate: String(rateNum),
      })
      await queryClient.invalidateQueries({ queryKey: ['chat-system-config'] })
      setMsg({ ok: true, text: '配置已保存' })
    } catch (e: unknown) {
      setMsg({ ok: false, text: e instanceof Error ? e.message : '保存失败' })
    } finally {
      setSaving(false)
    }
  }

  return (
    <SettingsLayout>
      <section className="glass rounded-2xl p-5">
        <h2 className="text-sm font-semibold text-gray-700 dark:text-gray-200 mb-4">系统配置</h2>

        {error && (
          <div className="mb-4 text-sm text-rose-600 dark:text-rose-400">
            {(error as Error).message || '获取配置失败'}
          </div>
        )}

        {isLoading || !enabled ? (
          <div className="space-y-3 max-w-lg">
            {Array.from({ length: 5 }).map((_, i) => (
              <div key={i} className="skeleton h-9 rounded-lg" />
            ))}
          </div>
        ) : (
          <div className="space-y-4 max-w-lg">
            <label className="inline-flex items-center gap-1.5 text-sm text-gray-700 dark:text-gray-200 cursor-pointer select-none">
              <input
                type="checkbox"
                checked={etlEnabled}
                onChange={(e) => setEtlEnabled(e.target.checked)}
                className="accent-apple-blue cursor-pointer"
              />
              启用每日自动 ETL
            </label>

            <Field label="定时任务 Cron 表达式" hint="每天凌晨 2 点：0 2 * * *">
              <input type="text" value={cron} onChange={(e) => setCron(e.target.value)} placeholder="0 2 * * *" className={`${INPUT_CLS} font-mono`} />
            </Field>

            <Field label="定时任务数据源" hint="定时 ETL 绑定的数据源。留空则即使启用定时，也不会执行同步。">
              <select value={etlSource} onChange={(e) => setEtlSource(e.target.value)} className={INPUT_CLS}>
                <option value="">未绑定</option>
                {(datasources || []).map((d) => (
                  <option key={d.id} value={String(d.id)} disabled={!d.is_enabled}>
                    {d.name}（{d.source_type === 'postgres' ? 'PG' : 'ES'}）{d.is_enabled ? '' : ' - 未启用'}
                  </option>
                ))}
              </select>
            </Field>

            <Field label="系统币种" hint="价格存储和成本计算使用的基准币种。修改后新建的价格按新币种换算，已有价格不受影响。">
              <select value={currency} onChange={(e) => setCurrency(e.target.value)} className={INPUT_CLS}>
                {CURRENCY_OPTIONS.map((o) => (
                  <option key={o.value} value={o.value}>{o.label}</option>
                ))}
              </select>
            </Field>

            <Field label="默认汇率（外币兑换系统币种）" hint="新增模型价格时，非系统币种默认使用此汇率换算，可在价格编辑时单独修改。">
              <input type="number" step="0.0001" min="0.0001" value={rate} onChange={(e) => setRate(e.target.value)} placeholder="7.2420" className={INPUT_CLS} />
            </Field>

            <div className="flex items-center gap-3 pt-1">
              <button type="button" onClick={handleSave} disabled={saving} className={BTN_PRIMARY}>
                {saving ? '保存中...' : '保存配置'}
              </button>
              {msg && (
                <span className={`text-sm ${msg.ok ? 'text-emerald-600 dark:text-emerald-400' : 'text-rose-600 dark:text-rose-400'}`}>
                  {msg.text}
                </span>
              )}
            </div>
          </div>
        )}
      </section>
    </SettingsLayout>
  )
}
