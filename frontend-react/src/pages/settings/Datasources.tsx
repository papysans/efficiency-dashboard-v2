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
                        <Tag tone={r.source_type === 'postgres' ? 'primary' : 'warning'}>
                          {r.source_type === 'postgres' ? 'PG' : 'ES'}
                        </Tag>
                      </td>
                      <td className={TD}>
                        <div className="max-w-[260px] truncate font-mono text-xs" title={r.source_type === 'postgres' ? `${r.pg_host}:${r.pg_port}` : r.es_hosts || ''}>
                          {r.source_type === 'postgres' ? `${r.pg_host || '-'}:${r.pg_port ?? '-'}` : r.es_hosts || '-'}
                        </div>
                      </td>
                      <td className={TD}>{r.source_type === 'postgres' ? r.pg_database || '-' : r.es_index || '-'}</td>
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

  const [notes, setNotes] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [err, setErr] = useState('')

  useEffect(() => {
    if (!open) return
    setErr('')
    setName(editing?.name || '')
    setSourceType(editing?.source_type || 'postgres')
    setIsEnabled(editing ? editing.is_enabled : true)
    setPgHost(editing?.pg_host || '')
    setPgPort(editing?.pg_port != null ? String(editing.pg_port) : '5432')
    setPgDatabase(editing?.pg_database || '')
    setPgSchema(editing?.pg_schema || '')
    setPgTable(editing?.pg_table || '')
    setPgUsername(editing?.pg_username || '')
    setPgPassword(editing?.pg_password || '')
    setPgSslMode(editing?.pg_ssl_mode || 'disable')
    setEsHosts(editing?.es_hosts || '')
    setEsIndex(editing?.es_index || '')
    setEsUsername(editing?.es_username || '')
    setEsPassword(editing?.es_password || '')
    setEsScrollDuration(editing?.es_scroll_duration || '')
    setEsVerifyCerts(editing?.es_verify_certs ?? true)
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
    } else {
      payload.es_hosts = esHosts.trim() || null
      payload.es_index = esIndex.trim() || null
      payload.es_username = esUsername.trim() || null
      payload.es_password = esPassword || null
      payload.es_scroll_duration = esScrollDuration.trim() || null
      payload.es_verify_certs = esVerifyCerts
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

        <Field label="备注">
          <textarea rows={2} value={notes} onChange={(e) => setNotes(e.target.value)} className={`${INPUT_CLS} resize-y`} />
        </Field>
      </div>
    </Modal>
  )
}
