// 设置·数据源：chat 指标源库（PostgreSQL / Elasticsearch）增改删 + 连接测试。
// ⚠️ 连接测试失败也是 HTTP 200，必须看返回体 success/message（见 design.md §2.3 / ChatDatasourceTestResult）。
import { useEffect, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { chatStats } from '@/api/endpoints'
import { useChatDatasources } from '@/api/queries'
import type { ChatDatasource, ChatDatasourceUpsert } from '@/api/types'
import { Modal } from '@/components/ui/Modal'
import { Tag } from '@/components/ui/Tag'
import SettingsLayout, {
  BTN_DANGER,
  BTN_GLASS,
  BTN_PRIMARY,
  Field,
  INPUT_CLS,
  LINK_BTN,
  LINK_BTN_DANGER,
  TD,
  TH,
  useChatEnabled,
} from './SettingsLayout'

const SSL_MODES = ['disable', 'require', 'verify-ca', 'verify-full']

function typeLabel(t: string): string {
  const m: Record<string, string> = { postgres: 'PG', elasticsearch: 'ES', loki: 'Loki', dept_api: '部门API', log_storage: '日志存储' }
  return m[t] || t
}

function tagTone(t: string): 'primary' | 'warning' | 'success' | 'error' {
  if (t === 'postgres') return 'primary'
  if (t === 'elasticsearch') return 'warning'
  if (t === 'loki') return 'success'
  return 'error'
}

/** 解析 config_json 中的主机/地址信息，无 config_json 时回退到扁平字段。 */
function dsHost(r: ChatDatasource): string {
  if (r.config_json) {
    try {
      const cfg = JSON.parse(r.config_json)
      switch (r.source_type) {
        case 'postgres': return `${cfg.host || r.pg_host || '-'}:${cfg.port ?? r.pg_port ?? '-'}`
        case 'elasticsearch': return JSON.stringify(cfg.hosts || [])
        case 'loki': return cfg.url || r.loki_url || '-'
        case 'dept_api': return cfg.base_url || '-'
        case 'log_storage': return cfg.storage === 's3' && cfg.s3 ? cfg.s3.endpoint : cfg.root_dir || '-'
      }
    } catch { /* fallthrough */ }
  }
  if (r.source_type === 'postgres') return `${r.pg_host || '-'}:${r.pg_port ?? '-'}`
  if (r.source_type === 'loki') return r.loki_url || '-'
  return r.es_hosts || '-'
}

/** 解析 config_json 中的库/索引信息。 */
function dsDb(r: ChatDatasource): string {
  if (r.config_json) {
    try {
      const cfg = JSON.parse(r.config_json)
      switch (r.source_type) {
        case 'postgres': return cfg.database || r.pg_database || '-'
        case 'elasticsearch': return cfg.index || r.es_index || '-'
        case 'loki': {
          const qs = cfg.queries || []
          return qs.length ? qs.map((q: { name: string }) => q.name).filter(Boolean).join(', ') || `${qs.length} 个预设` : '-'
        }
        case 'dept_api': return '-'
        case 'log_storage': return cfg.storage === 's3' && cfg.s3 ? cfg.s3.bucket : `max ${cfg.max_size_mb || 5}MB`
      }
    } catch { /* fallthrough */ }
  }
  if (r.source_type === 'postgres') return r.pg_database || '-'
  if (r.source_type === 'loki') {
    try {
      const qs = JSON.parse(r.loki_queries || '[]')
      if (Array.isArray(qs) && qs.length) return qs.map((q: { name: string }) => q.name).filter(Boolean).join(', ') || `${qs.length} 个预设`
    } catch { /* ignore */ }
    return '-'
  }
  return r.es_index || '-'
}

type TestState = { loading?: boolean; ok?: boolean; text?: string }

export default function Datasources() {
  const enabled = useChatEnabled()
  const queryClient = useQueryClient()
  const { data: rows, isLoading, error } = useChatDatasources(enabled)

  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<ChatDatasource | null>(null)
  const [pendingDelete, setPendingDelete] = useState<ChatDatasource | null>(null)
  const [deleting, setDeleting] = useState(false)
  const [deleteErr, setDeleteErr] = useState('')
  // 每行连接测试状态（id → 状态）
  const [testState, setTestState] = useState<Record<number, TestState>>({})

  async function invalidate() {
    await queryClient.invalidateQueries({ queryKey: ['chat-datasources'] })
  }

  async function handleSubmit(payload: ChatDatasourceUpsert) {
    if (editing) await chatStats.updateDatasource(editing.id, payload)
    else await chatStats.createDatasource(payload)
    setModalOpen(false)
    await invalidate()
  }

  async function confirmDelete() {
    if (!pendingDelete) return
    setDeleting(true)
    setDeleteErr('')
    try {
      await chatStats.deleteDatasource(pendingDelete.id)
      setPendingDelete(null)
      await invalidate()
    } catch (e: unknown) {
      setDeleteErr(e instanceof Error ? e.message : '删除失败')
    } finally {
      setDeleting(false)
    }
  }

  async function handleTest(id: number) {
    setTestState((s) => ({ ...s, [id]: { loading: true } }))
    try {
      const res = await chatStats.testDatasource(id)
      // 失败也是 HTTP 200，按返回体 success 判定
      setTestState((s) => ({
        ...s,
        [id]: res.success
          ? { ok: true, text: `${res.message}（${res.ping_ms}ms）` }
          : { ok: false, text: res.message || '连接失败' },
      }))
    } catch (e: unknown) {
      setTestState((s) => ({ ...s, [id]: { ok: false, text: e instanceof Error ? e.message : '测试失败' } }))
    }
  }

  const list = rows || []

  return (
    <SettingsLayout>
      <section className="glass rounded-2xl overflow-hidden">
        <div className="flex items-center justify-between px-5 py-3 border-b border-gray-200/50 dark:border-white/10">
          <span className="text-sm font-semibold text-gray-700 dark:text-gray-200">源数据源（{list.length}）</span>
          <button type="button" onClick={() => { setEditing(null); setModalOpen(true) }} className={BTN_PRIMARY}>
            新增数据源
          </button>
        </div>

        {error && (
          <div className="px-5 py-2 text-sm text-rose-600 dark:text-rose-400 bg-rose-50/50 dark:bg-rose-900/20">
            {(error as Error).message || '获取数据源列表失败'}
          </div>
        )}

        <div className="overflow-x-auto">
          <table className="w-full text-sm border-collapse">
            <thead>
              <tr className="border-b border-gray-200/50 dark:border-white/10">
                <th className={`${TH} min-w-[140px]`}>名称</th>
                <th className={TH}>类型</th>
                <th className={`${TH} min-w-[180px]`}>主机</th>
                <th className={TH}>库/索引</th>
                <th className={TH}>启用</th>
                <th className={`${TH} min-w-[180px]`}>连接测试</th>
                <th className={`${TH} text-center`}>操作</th>
              </tr>
            </thead>
            <tbody>
              {isLoading || !enabled ? (
                Array.from({ length: 3 }).map((_, i) => (
                  <tr key={i} className="border-b border-gray-100/50 dark:border-white/5">
                    <td className={TD} colSpan={7}>
                      <div className="skeleton h-6 rounded" />
                    </td>
                  </tr>
                ))
              ) : list.length === 0 ? (
                <tr>
                  <td colSpan={7}>
                    <div className="py-12 text-center text-sm text-gray-400 dark:text-gray-500">
                      暂无数据源，点击「新增数据源」配置
                    </div>
                  </td>
                </tr>
              ) : (
                list.map((r) => {
                  const ts = testState[r.id]
                  return (
                    <tr key={r.id} className="border-b border-gray-100/50 dark:border-white/5 hover:bg-apple-blue/5 dark:hover:bg-white/5 transition-colors">
                      <td className={TD}>
                        <span className="font-medium text-gray-900 dark:text-white">{r.name}</span>
                      </td>
                      <td className={TD}>
                        <Tag tone={tagTone(r.source_type)}>
                          {typeLabel(r.source_type)}
                        </Tag>
                      </td>
                      <td className={TD}>
                        <div className="max-w-[260px] truncate font-mono text-xs" title={dsHost(r)}>
                          {dsHost(r)}
                        </div>
                      </td>
                      <td className={TD}>{dsDb(r)}</td>
                      <td className={TD}>
                        <Tag tone={r.is_enabled ? 'success' : 'error'}>{r.is_enabled ? '是' : '否'}</Tag>
                      </td>
                      <td className={TD}>
                        {ts?.loading ? (
                          <span className="text-xs text-gray-400">测试中...</span>
                        ) : ts?.text ? (
                          <span className={`text-xs ${ts.ok ? 'text-emerald-600 dark:text-emerald-400' : 'text-rose-600 dark:text-rose-400'}`} title={ts.text}>
                            {ts.text}
                          </span>
                        ) : (
                          <span className="text-xs text-gray-400">-</span>
                        )}
                      </td>
                      <td className="px-3 py-2 align-middle text-center whitespace-nowrap">
                        <div className="inline-flex items-center gap-2">
                          <button type="button" className={LINK_BTN} onClick={() => { setEditing(r); setModalOpen(true) }}>编辑</button>
                          <button type="button" className={LINK_BTN} disabled={ts?.loading} onClick={() => handleTest(r.id)}>
                            测试连接
                          </button>
                          <button type="button" className={LINK_BTN_DANGER} onClick={() => setPendingDelete(r)}>删除</button>
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

      <DatasourceModal open={modalOpen} editing={editing} onClose={() => setModalOpen(false)} onSubmit={handleSubmit} />

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
            确定要删除数据源「{pendingDelete?.name}」吗？此操作不可撤销。
          </p>
        </div>
      </Modal>
    </SettingsLayout>
  )
}

// ---- 新增/编辑弹层 ----

function DatasourceModal({
  open,
  editing,
  onClose,
  onSubmit,
}: {
  open: boolean
  editing: ChatDatasource | null
  onClose: () => void
  onSubmit: (payload: ChatDatasourceUpsert) => Promise<void>
}) {
  const [name, setName] = useState('')
  const [sourceType, setSourceType] = useState('postgres')
  const [isEnabled, setIsEnabled] = useState(true)
  // PG
  const [pgHost, setPgHost] = useState('')
  const [pgPort, setPgPort] = useState('5432')
  const [pgDatabase, setPgDatabase] = useState('')
  const [pgSchema, setPgSchema] = useState('')
  const [pgTable, setPgTable] = useState('')
  const [pgUsername, setPgUsername] = useState('')
  const [pgPassword, setPgPassword] = useState('')
  const [pgSslMode, setPgSslMode] = useState('disable')
  // ES
  const [esHosts, setEsHosts] = useState('')
  const [esIndex, setEsIndex] = useState('')
  const [esUsername, setEsUsername] = useState('')
  const [esPassword, setEsPassword] = useState('')
  const [esScrollDuration, setEsScrollDuration] = useState('')
  const [esVerifyCerts, setEsVerifyCerts] = useState(true)
  // Loki
  const [lokiUrl, setLokiUrl] = useState('')
  const [lokiUsername, setLokiUsername] = useState('')
  const [lokiPassword, setLokiPassword] = useState('')
  const [lokiTenantId, setLokiTenantId] = useState('')
  const [lokiVerifyCerts, setLokiVerifyCerts] = useState(true)
  const [lokiQueries, setLokiQueries] = useState<Array<{ name: string; label_selector: string }>>([])
  // dept_api
  const [deptBaseUrl, setDeptBaseUrl] = useState('')
  const [deptQueryKey, setDeptQueryKey] = useState('')
  const [deptTimeout, setDeptTimeout] = useState('15')
  // log_storage
  const [lsStorage, setLsStorage] = useState('disk')
  const [lsRootDir, setLsRootDir] = useState('')
  const [lsMaxSizeMb, setLsMaxSizeMb] = useState('5')
  const [lsS3Endpoint, setLsS3Endpoint] = useState('')
  const [lsS3Bucket, setLsS3Bucket] = useState('')
  const [lsS3Region, setLsS3Region] = useState('')
  const [lsS3AccessKey, setLsS3AccessKey] = useState('')
  const [lsS3SecretKey, setLsS3SecretKey] = useState('')
  const [lsS3SessionToken, setLsS3SessionToken] = useState('')
  const [lsS3UseSSL, setLsS3UseSSL] = useState(true)
  const [lsS3InsecureSkipVerify, setLsS3InsecureSkipVerify] = useState(false)

  const [notes, setNotes] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [err, setErr] = useState('')

  useEffect(() => {
    if (!open) return
    setErr('')
    setName(editing?.name || '')
    setSourceType(editing?.source_type || 'postgres')
    setIsEnabled(editing ? editing.is_enabled : true)
    // 解析 config_json 回填
    let cfg: Record<string, unknown> | null = null
    if (editing?.config_json) {
      try { cfg = JSON.parse(editing.config_json) } catch { /* ignore */ }
    }
    setPgHost(editing?.pg_host || (cfg as Record<string, string>)?.host || '')
    setPgPort(editing?.pg_port != null ? String(editing.pg_port) : (cfg as Record<string, number>)?.port != null ? String((cfg as Record<string, number>).port) : '5432')
    setPgDatabase(editing?.pg_database || (cfg as Record<string, string>)?.database || '')
    setPgSchema(editing?.pg_schema || (cfg as Record<string, string>)?.schema || '')
    setPgTable(editing?.pg_table || (cfg as Record<string, string>)?.table || '')
    setPgUsername(editing?.pg_username || (cfg as Record<string, string>)?.username || '')
    setPgPassword(editing?.pg_password || '')
    setPgSslMode(editing?.pg_ssl_mode || (cfg as Record<string, string>)?.ssl_mode || 'disable')
    setEsHosts(editing?.es_hosts || (cfg ? JSON.stringify((cfg as Record<string, unknown[]>).hosts || []) : ''))
    setEsIndex(editing?.es_index || (cfg as Record<string, string>)?.index || '')
    setEsUsername(editing?.es_username || (cfg as Record<string, string>)?.username || '')
    setEsPassword(editing?.es_password || '')
    setEsScrollDuration(editing?.es_scroll_duration || (cfg as Record<string, string>)?.scroll_duration || '')
    setEsVerifyCerts(editing?.es_verify_certs ?? (cfg as Record<string, boolean>)?.verify_certs ?? true)
    // Loki
    setLokiUrl(editing?.loki_url || (cfg as Record<string, string>)?.url || '')
    setLokiUsername(editing?.loki_username || (cfg as Record<string, string>)?.username || '')
    setLokiPassword(editing?.loki_password || '')
    setLokiTenantId(editing?.loki_tenant_id || (cfg as Record<string, string>)?.tenant_id || '')
    setLokiVerifyCerts(editing?.loki_verify_certs ?? (cfg as Record<string, boolean>)?.verify_certs ?? true)
    setLokiQueries(
      (cfg && Array.isArray((cfg as Record<string, unknown[]>).queries) ? (cfg as Record<string, unknown[]>).queries : []) as Array<{ name: string; label_selector: string }> ||
      (editing?.loki_queries ? (() => { try { return JSON.parse(editing.loki_queries || '[]') } catch { return [] } })() : []),
    )
    // dept_api
    setDeptBaseUrl((cfg as Record<string, string>)?.base_url || '')
    setDeptQueryKey((cfg as Record<string, string>)?.query_key || '')
    setDeptTimeout((cfg as Record<string, number>)?.timeout != null ? String((cfg as Record<string, number>).timeout) : '15')
    // log_storage
    setLsStorage((cfg as Record<string, string>)?.storage || 'disk')
    setLsRootDir((cfg as Record<string, string>)?.root_dir || '')
    setLsMaxSizeMb((cfg as Record<string, number>)?.max_size_mb != null ? String((cfg as Record<string, number>).max_size_mb) : '5')
    const s3 = (cfg as Record<string, Record<string, unknown>>)?.s3
    setLsS3Endpoint((s3?.endpoint as string) || '')
    setLsS3Bucket((s3?.bucket as string) || '')
    setLsS3Region((s3?.region as string) || '')
    setLsS3AccessKey((s3?.access_key as string) || '')
    setLsS3SecretKey('')
    setLsS3SessionToken((s3?.session_token as string) || '')
    setLsS3UseSSL((s3?.use_ssl as boolean) ?? true)
    setLsS3InsecureSkipVerify((s3?.insecure_skip_verify as boolean) ?? false)
    setNotes(editing?.notes || '')
  }, [open, editing])

  async function handleSubmit() {
    const n = name.trim()
    if (!n) {
      setErr('请输入名称')
      return
    }
    const payload: ChatDatasourceUpsert = {
      name: n,
      source_type: sourceType,
      is_enabled: isEnabled,
      notes: notes.trim() ? notes.trim() : null,
    }
    // 构建 config_json
    const cfg = buildConfigJson()
    if (cfg) payload.config_json = JSON.stringify(cfg)

    if (sourceType === 'postgres') {
      const port = Number(pgPort.trim())
      payload.pg_host = pgHost.trim() || null
      payload.pg_port = Number.isFinite(port) && port > 0 ? port : null
      payload.pg_database = pgDatabase.trim() || null
      payload.pg_schema = pgSchema.trim() || null
      payload.pg_table = pgTable.trim() || null
      payload.pg_username = pgUsername.trim() || null
      payload.pg_password = pgPassword || null
      payload.pg_ssl_mode = pgSslMode || null
    } else if (sourceType === 'elasticsearch') {
      payload.es_hosts = esHosts.trim() || null
      payload.es_index = esIndex.trim() || null
      payload.es_username = esUsername.trim() || null
      payload.es_password = esPassword || null
      payload.es_scroll_duration = esScrollDuration.trim() || null
      payload.es_verify_certs = esVerifyCerts
    } else if (sourceType === 'loki') {
      payload.loki_url = lokiUrl.trim() || null
      payload.loki_username = lokiUsername.trim() || null
      payload.loki_password = lokiPassword || null
      payload.loki_tenant_id = lokiTenantId.trim() || null
      payload.loki_verify_certs = lokiVerifyCerts
      payload.loki_queries = JSON.stringify(lokiQueries.filter(q => q.name || q.label_selector))
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

  function buildConfigJson(): Record<string, unknown> | null {
    switch (sourceType) {
      case 'postgres':
        return { host: pgHost.trim() || '127.0.0.1', port: Number(pgPort.trim()) || 5432, database: pgDatabase.trim() || 'user_indicator', schema: pgSchema.trim() || 'public', table: pgTable.trim() || 'chat_metrics', username: pgUsername.trim() || 'postgres', password: pgPassword || '', ssl_mode: pgSslMode || 'disable' }
      case 'elasticsearch': {
        let hosts: string[] = []
        try { hosts = JSON.parse(esHosts.trim() || '[]') } catch { hosts = [] }
        return { hosts, username: esUsername.trim() || '', password: esPassword || '', index: esIndex.trim() || 'costrict_chat_metrics_v3', verify_certs: esVerifyCerts, scroll_duration: esScrollDuration.trim() || '5m' }
      }
      case 'loki': {
        const queries = lokiQueries.filter(q => q && (q.name || q.label_selector))
        return { url: lokiUrl.trim() || '', username: lokiUsername.trim() || '', password: lokiPassword || '', tenant_id: lokiTenantId.trim() || '', verify_certs: lokiVerifyCerts, queries }
      }
      case 'dept_api':
        return { base_url: deptBaseUrl.trim() || '', query_key: deptQueryKey.trim() || '', timeout: Number(deptTimeout.trim()) || 15 }
      case 'log_storage': {
        const c: Record<string, unknown> = { storage: lsStorage, root_dir: lsRootDir.trim() || '', max_size_mb: Number(lsMaxSizeMb.trim()) || 5 }
        if (lsStorage === 's3') {
          c.s3 = { endpoint: lsS3Endpoint.trim() || '', use_ssl: lsS3UseSSL, insecure_skip_verify: lsS3InsecureSkipVerify, bucket: lsS3Bucket.trim() || '', region: lsS3Region.trim() || '', access_key: lsS3AccessKey.trim() || '', secret_key: lsS3SecretKey || '', session_token: lsS3SessionToken.trim() || '' }
        }
        return c
      }
      default: return null
    }
  }

  return (
    <Modal
      open={open}
      title={editing ? `编辑数据源 — ${editing.name}` : '新增数据源'}
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
        <Field label="名称">
          <input type="text" value={name} onChange={(e) => setName(e.target.value)} placeholder="如 生产PG、ES集群" className={INPUT_CLS} />
        </Field>
        <div className="grid grid-cols-2 gap-3">
          <Field label="数据源类型">
            <select value={sourceType} onChange={(e) => setSourceType(e.target.value)} className={INPUT_CLS}>
              <option value="postgres">PostgreSQL</option>
              <option value="elasticsearch">Elasticsearch</option>
              <option value="loki">Loki（链路日志）</option>
              <option value="dept_api">部门查询 API</option>
              <option value="log_storage">日志存储（预览）</option>
            </select>
          </Field>
          <label className="flex items-end pb-2 gap-1.5 text-sm text-gray-600 dark:text-gray-300 cursor-pointer select-none">
            <input type="checkbox" checked={isEnabled} onChange={(e) => setIsEnabled(e.target.checked)} className="accent-apple-blue cursor-pointer" />
            启用
          </label>
        </div>

        {sourceType === 'postgres' && (
          <>
            <div className="grid grid-cols-2 gap-3">
              <Field label="主机">
                <input type="text" value={pgHost} onChange={(e) => setPgHost(e.target.value)} placeholder="127.0.0.1" className={INPUT_CLS} />
              </Field>
              <Field label="端口">
                <input type="number" value={pgPort} onChange={(e) => setPgPort(e.target.value)} placeholder="5432" className={INPUT_CLS} />
              </Field>
            </div>
            <div className="grid grid-cols-3 gap-3">
              <Field label="数据库">
                <input type="text" value={pgDatabase} onChange={(e) => setPgDatabase(e.target.value)} placeholder="user_indicator" className={INPUT_CLS} />
              </Field>
              <Field label="Schema">
                <input type="text" value={pgSchema} onChange={(e) => setPgSchema(e.target.value)} placeholder="public" className={INPUT_CLS} />
              </Field>
              <Field label="表名">
                <input type="text" value={pgTable} onChange={(e) => setPgTable(e.target.value)} placeholder="chat_metrics" className={INPUT_CLS} />
              </Field>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <Field label="用户名">
                <input type="text" value={pgUsername} onChange={(e) => setPgUsername(e.target.value)} className={INPUT_CLS} />
              </Field>
              <Field label="密码">
                <input type="password" value={pgPassword} onChange={(e) => setPgPassword(e.target.value)} autoComplete="new-password" className={INPUT_CLS} />
              </Field>
            </div>
            <Field label="SSL 模式">
              <select value={pgSslMode} onChange={(e) => setPgSslMode(e.target.value)} className={INPUT_CLS}>
                {SSL_MODES.map((m) => (
                  <option key={m} value={m}>{m}</option>
                ))}
              </select>
            </Field>
          </>
        )}

        {sourceType === 'elasticsearch' && (
          <>
            <Field label="ES 地址" hint='JSON 数组：["https://host:9200"]'>
              <textarea rows={2} value={esHosts} onChange={(e) => setEsHosts(e.target.value)} placeholder='["https://192.168.1.1:9200"]' className={`${INPUT_CLS} resize-y font-mono`} />
            </Field>
            <Field label="索引名">
              <input type="text" value={esIndex} onChange={(e) => setEsIndex(e.target.value)} placeholder="costrict_chat_metrics_v3" className={INPUT_CLS} />
            </Field>
            <div className="grid grid-cols-2 gap-3">
              <Field label="用户名">
                <input type="text" value={esUsername} onChange={(e) => setEsUsername(e.target.value)} className={INPUT_CLS} />
              </Field>
              <Field label="密码">
                <input type="password" value={esPassword} onChange={(e) => setEsPassword(e.target.value)} autoComplete="new-password" className={INPUT_CLS} />
              </Field>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <Field label="Scroll 保持时间">
                <input type="text" value={esScrollDuration} onChange={(e) => setEsScrollDuration(e.target.value)} placeholder="5m" className={INPUT_CLS} />
              </Field>
              <label className="flex items-end pb-2 gap-1.5 text-sm text-gray-600 dark:text-gray-300 cursor-pointer select-none" title="关闭后可连接自签名证书的 ES 节点">
                <input type="checkbox" checked={esVerifyCerts} onChange={(e) => setEsVerifyCerts(e.target.checked)} className="accent-apple-blue cursor-pointer" />
                验证 SSL 证书
              </label>
            </div>
          </>
        )}

        {sourceType === 'loki' && (
          <>
            <Field label="Loki 地址" hint="如 http://loki:3100（不要带 /loki/api 前缀）">
              <input type="text" value={lokiUrl} onChange={(e) => setLokiUrl(e.target.value)} placeholder="http://loki:3100" className={INPUT_CLS} />
            </Field>
            <Field label="查询预设" hint="每个预设独立配置 label 选择器，链路日志抽屉里可下拉切换">
              <div className="space-y-2">
                {lokiQueries.map((q, i) => (
                  <div key={i} className="flex gap-2 items-start">
                    <input type="text" value={q.name} onChange={(e) => { const cp = [...lokiQueries]; cp[i] = { ...cp[i], name: e.target.value }; setLokiQueries(cp) }} placeholder="预设名称" className={`${INPUT_CLS} w-40`} />
                    <input type="text" value={q.label_selector} onChange={(e) => { const cp = [...lokiQueries]; cp[i] = { ...cp[i], label_selector: e.target.value }; setLokiQueries(cp) }} placeholder='app="chat-rag",env="prod"' className={`${INPUT_CLS} flex-1`} />
                    <button type="button" onClick={() => setLokiQueries(lokiQueries.filter((_, j) => j !== i))} className="text-rose-500 hover:text-rose-700 text-lg leading-none mt-1.5">&times;</button>
                  </div>
                ))}
                <button type="button" onClick={() => setLokiQueries([...lokiQueries, { name: '', label_selector: '' }])} className="text-xs text-apple-blue hover:underline">
                  + 添加查询预设
                </button>
              </div>
            </Field>
            <div className="grid grid-cols-2 gap-3">
              <Field label="用户名（可选）">
                <input type="text" value={lokiUsername} onChange={(e) => setLokiUsername(e.target.value)} className={INPUT_CLS} />
              </Field>
              <Field label="密码（可选）">
                <input type="password" value={lokiPassword} onChange={(e) => setLokiPassword(e.target.value)} autoComplete="new-password" className={INPUT_CLS} />
              </Field>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <Field label="Tenant ID（多租户可选）">
                <input type="text" value={lokiTenantId} onChange={(e) => setLokiTenantId(e.target.value)} className={INPUT_CLS} />
              </Field>
              <label className="flex items-end pb-2 gap-1.5 text-sm text-gray-600 dark:text-gray-300 cursor-pointer select-none">
                <input type="checkbox" checked={lokiVerifyCerts} onChange={(e) => setLokiVerifyCerts(e.target.checked)} className="accent-apple-blue cursor-pointer" />
                验证 SSL 证书
              </label>
            </div>
          </>
        )}

        {sourceType === 'dept_api' && (
          <>
            <Field label="API 基础地址" hint="costrict-dept-info 服务地址">
              <input type="text" value={deptBaseUrl} onChange={(e) => setDeptBaseUrl(e.target.value)} placeholder="https://dept-api.example.com" className={INPUT_CLS} />
            </Field>
            <Field label="查询密钥 (query_key)">
              <input type="password" value={deptQueryKey} onChange={(e) => setDeptQueryKey(e.target.value)} autoComplete="new-password" placeholder="认证密钥" className={INPUT_CLS} />
            </Field>
            <Field label="超时(秒)">
              <input type="number" value={deptTimeout} onChange={(e) => setDeptTimeout(e.target.value)} min={1} max={120} className={INPUT_CLS} />
            </Field>
          </>
        )}

        {sourceType === 'log_storage' && (
          <>
            <Field label="存储方式">
              <select value={lsStorage} onChange={(e) => setLsStorage(e.target.value)} className={INPUT_CLS}>
                <option value="disk">本地磁盘 (disk)</option>
                <option value="s3">S3 / MinIO (s3)</option>
              </select>
            </Field>
            <Field label="根目录/前缀" hint={lsStorage === 'disk' ? '日志文件根目录' : 'S3 object key 前缀'}>
              <input type="text" value={lsRootDir} onChange={(e) => setLsRootDir(e.target.value)} placeholder={lsStorage === 'disk' ? '/data/logs' : 'chat-logs/'} className={INPUT_CLS} />
            </Field>
            <Field label="预览大小阈值 (MB)">
              <input type="number" value={lsMaxSizeMb} onChange={(e) => setLsMaxSizeMb(e.target.value)} min={1} max={50} className={INPUT_CLS} />
            </Field>
            {lsStorage === 's3' && (
              <>
                <Field label="S3 Endpoint" hint="如 192.168.1.1:9000 或 https://s3.example.com">
                  <input type="text" value={lsS3Endpoint} onChange={(e) => setLsS3Endpoint(e.target.value)} placeholder="192.168.1.1:9000" className={INPUT_CLS} />
                </Field>
                <div className="grid grid-cols-2 gap-3">
                  <Field label="Bucket">
                    <input type="text" value={lsS3Bucket} onChange={(e) => setLsS3Bucket(e.target.value)} placeholder="chat-rag" className={INPUT_CLS} />
                  </Field>
                  <Field label="Region">
                    <input type="text" value={lsS3Region} onChange={(e) => setLsS3Region(e.target.value)} placeholder="us-east-1" className={INPUT_CLS} />
                  </Field>
                </div>
                <div className="grid grid-cols-2 gap-3">
                  <Field label="Access Key">
                    <input type="text" value={lsS3AccessKey} onChange={(e) => setLsS3AccessKey(e.target.value)} className={INPUT_CLS} />
                  </Field>
                  <Field label="Secret Key">
                    <input type="password" value={lsS3SecretKey} onChange={(e) => setLsS3SecretKey(e.target.value)} autoComplete="new-password" className={INPUT_CLS} />
                  </Field>
                </div>
                <Field label="Session Token（可选）">
                  <input type="text" value={lsS3SessionToken} onChange={(e) => setLsS3SessionToken(e.target.value)} className={INPUT_CLS} />
                </Field>
                <div className="grid grid-cols-2 gap-3">
                  <label className="flex items-end pb-2 gap-1.5 text-sm text-gray-600 dark:text-gray-300 cursor-pointer select-none">
                    <input type="checkbox" checked={lsS3UseSSL} onChange={(e) => setLsS3UseSSL(e.target.checked)} className="accent-apple-blue cursor-pointer" />
                    使用 SSL
                  </label>
                  <label className="flex items-end pb-2 gap-1.5 text-sm text-gray-600 dark:text-gray-300 cursor-pointer select-none">
                    <input type="checkbox" checked={lsS3InsecureSkipVerify} onChange={(e) => setLsS3InsecureSkipVerify(e.target.checked)} className="accent-apple-blue cursor-pointer" />
                    跳过证书验证
                  </label>
                </div>
              </>
            )}
          </>
        )}

        <Field label="备注">
          <textarea rows={2} value={notes} onChange={(e) => setNotes(e.target.value)} className={`${INPUT_CLS} resize-y`} />
        </Field>
      </div>
    </Modal>
  )
}
