// 创建项目弹窗（可复用）。原为 ProjectList.tsx 的局部组件，抽取为共享组件，
// 供项目列表页与「项目」维度壳（DimensionEntityLayout）共用，任意子维度都能新建项目。
//
// 自带提交逻辑（createProject）+ 成功后失效项目相关 query（列表/选择器即时刷新），
// 通过 onCreated 回调把新建项目的返回交给调用方（如跳转/二次刷新），无 toast 基建时即关弹窗+刷新。
import { useEffect, useState, type ReactNode } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { createProject } from '@/api/endpoints'
import type { CreateProjectResponse } from '@/api/types'
import { Modal } from '@/components/ui/Modal'

const inputCls =
  'glass rounded-lg px-3 py-1.5 text-sm w-full bg-transparent text-gray-900 dark:text-white ' +
  'focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue'

export interface CreateProjectModalProps {
  open: boolean
  onClose: () => void
  /** 创建成功后回调（已自动失效项目 query）；调用方可用返回值跳转或二次刷新。 */
  onCreated?: (res: CreateProjectResponse) => void
}

export function CreateProjectModal({ open, onClose, onCreated }: CreateProjectModalProps) {
  const queryClient = useQueryClient()
  const [name, setName] = useState('')
  const [desc, setDesc] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [err, setErr] = useState('')

  useEffect(() => {
    if (!open) return
    setName('')
    setDesc('')
    setErr('')
  }, [open])

  async function handleSubmit() {
    if (!name.trim()) {
      setErr('请输入项目名称')
      return
    }
    setSubmitting(true)
    setErr('')
    try {
      const res = await createProject({ name: name.trim(), description: desc.trim() })
      // 失效项目列表（含 ['project-list', params] 各变体）+ 对象选择器复用的同一 query，
      // 让新项目立即出现在排行/选择器里。
      await queryClient.invalidateQueries({ queryKey: ['project-list'] })
      onClose()
      onCreated?.(res)
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : '创建失败')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Modal
      open={open}
      title="创建项目"
      maxWidth={500}
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
            {submitting ? '创建中...' : '创建'}
          </button>
        </>
      }
    >
      <div className="space-y-3">
        {err && <div className="text-sm text-rose-600 dark:text-rose-400">{err}</div>}
        <Field label="项目名称">
          <input type="text" value={name} onChange={(e) => setName(e.target.value)} className={inputCls} />
        </Field>
        <Field label="描述（可选）">
          <textarea rows={3} value={desc} onChange={(e) => setDesc(e.target.value)} className={`${inputCls} resize-y`} />
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
