// 项目详情页（纯 Need(branch) 口径重设计）—— Data-Dense Dashboard × 玻璃拟态。
// 三块：① 核心指标(KPI 卡) ② 组成·Needs(主数据表，可勾选纳入/排除) ③ 贡献者(从 Needs 守恒派生)。
// 「项目 = 一组 Need」：所有指标从已选干净 Need 派生。已移除 v1 遗留——古法 commit 度量 / Tasks 管理 /
// Commits 明细 / 旧用户视角。Repo 配置降级为「Need 来源规则」(repo[/分支])。
import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { useNavigate, useParams } from 'react-router'
import { useQueryClient } from '@tanstack/react-query'
import {
  addRepoToProject,
  deleteProject,
  getNeedRepoOptions,
  removeRepoFromProject,
  updateProject,
  updateProjectNeedSelection,
} from '@/api/endpoints'
import { useProjectDetail, useProjectNeeds } from '@/api/queries'
import type { NeedRepoOption, ProjectModel, ProjectNeedItem, ProjectRepo } from '@/api/types'
import { fmtCost, formatV2Ratio } from '@/lib/formatters'
import { useUserNameMap } from '@/hooks/useUserNameMap'
import { Tag } from '@/components/ui/Tag'
import { RatioPill } from '@/components/ui/RatioPill'
import { MetricCard } from '@/components/ui/MetricCard'
import { Modal } from '@/components/ui/Modal'

const WORK_MIN_PER_DAY = 480

function isZeroTime(s: string | null | undefined): boolean {
  return !s || String(s).startsWith('0001-')
}
function fmtDate(s: string | null | undefined): string {
  return isZeroTime(s) ? '—' : String(s).slice(0, 10)
}
/** 小数口径提效比的 KPI 卡着色：正绿 / 负红 / 空中性。 */
function ratioTone(r: number | null | undefined): 'pos' | 'neg' | 'neutral' {
  if (r == null || !Number.isFinite(r)) return 'neutral'
  return r >= 0 ? 'pos' : 'neg'
}
/** 客户端镜像 efficiencyV2Ratio：actual>0 才出比值（分子分母守恒后相除）。 */
function v2ratio(baseline: number, actual: number): number | null {
  return actual > 0 ? (baseline - actual) / actual : null
}

interface Contributor {
  user_id: string
  needCount: number
  loc: number
  aiLoc: number
  baseCal: number
  actCal: number
  baseWork: number
  actWork: number
  calRatio: number | null
  workRatio: number | null
  aiRatio: number | null
}

export default function ProjectDetail() {
  const { projectId } = useParams<{ projectId: string }>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { data, isLoading, error } = useProjectDetail(projectId)
  const { data: needsData } = useProjectNeeds(projectId)
  const { resolveName } = useUserNameMap()

  const project = data?.project
  const repos = useMemo<ProjectRepo[]>(() => project?.repos || [], [project])
  const projectNeeds = useMemo<ProjectNeedItem[]>(() => needsData?.data || [], [needsData])

  const [needBusy, setNeedBusy] = useState<string | null>(null)
  const [needErr, setNeedErr] = useState('')
  const [editOpen, setEditOpen] = useState(false)
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [sourceAddOpen, setSourceAddOpen] = useState(false)
  const [removeSource, setRemoveSource] = useState<{ index: number; repo: ProjectRepo } | null>(null)

  const reload = useCallback(async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ['project-detail', projectId] }),
      queryClient.invalidateQueries({ queryKey: ['project-needs', projectId] }),
    ])
  }, [queryClient, projectId])

  // 贡献者：从「已选(非 excluded)」Need 客户端守恒派生，按口径分别只计干净 Need（与后端 agg 同口径）。
  const contributors = useMemo<Contributor[]>(() => {
    const m = new Map<string, Contributor>()
    for (const n of projectNeeds) {
      if (n.excluded) continue
      const uid = n.primary_user_id || '未知'
      let c = m.get(uid)
      if (!c) {
        c = { user_id: uid, needCount: 0, loc: 0, aiLoc: 0, baseCal: 0, actCal: 0, baseWork: 0, actWork: 0, calRatio: null, workRatio: null, aiRatio: null }
        m.set(uid, c)
      }
      c.needCount += 1
      if (n.coverage_eligible && !n.calendar_outlier_flag) {
        c.baseCal += n.baseline_calendar_min || 0
        c.actCal += n.total_calendar_min || 0
      }
      if (n.coverage_eligible && !n.work_outlier_flag) {
        c.baseWork += n.baseline_fused_work_min || 0
        c.actWork += n.total_active_work_corrected_min || 0
      }
      if (n.coverage_eligible && !n.outlier_flag && (n.total_loc_net || 0) > 0) {
        c.loc += n.total_loc_net || 0
        c.aiLoc += n.ai_covered_loc || 0
      }
    }
    const rows = Array.from(m.values())
    for (const c of rows) {
      c.calRatio = v2ratio(c.baseCal, c.actCal)
      c.workRatio = v2ratio(c.baseWork, c.actWork)
      c.aiRatio = c.loc > 0 ? c.aiLoc / c.loc : null
    }
    rows.sort((a, b) => b.needCount - a.needCount || b.loc - a.loc)
    return rows
  }, [projectNeeds])

  const toggleNeed = useCallback(
    async (n: ProjectNeedItem) => {
      setNeedBusy(n.need_id)
      setNeedErr('')
      try {
        await updateProjectNeedSelection(projectId as string, {
          repo_addr: n.repo_addr,
          repo_branch: n.repo_branch,
          need_id: n.need_id,
          excluded: !n.excluded,
        })
        await reload()
      } catch (e: unknown) {
        setNeedErr(e instanceof Error ? e.message : '更新 Need 勾选失败')
      } finally {
        setNeedBusy(null)
      }
    },
    [reload, projectId],
  )

  async function confirmDelete() {
    setDeleting(true)
    try {
      await deleteProject(projectId as string)
      await queryClient.invalidateQueries({ queryKey: ['project-list'] })
      navigate('/project-v2')
    } finally {
      setDeleting(false)
    }
  }

  async function doRemoveSource() {
    if (!removeSource) return
    await removeRepoFromProject(projectId as string, removeSource.index)
    setRemoveSource(null)
    await reload()
  }

  if (error) {
    return (
      <div className="glass rounded-2xl p-8 text-center text-sm text-rose-600 dark:text-rose-400">
        {(error as Error).message || '获取项目详情失败'}
      </div>
    )
  }

  const dateRange =
    project && !isZeroTime(project.start_time_manual ?? project.start_time)
      ? `${fmtDate(project.start_time_manual ?? project.start_time)} ~ ${
          isZeroTime(project.end_time_manual ?? project.end_time) ? '至今' : fmtDate(project.end_time_manual ?? project.end_time)
        }`
      : '—'

  const calR = data?.need_calendar_efficiency_ratio
  const workR = data?.need_work_efficiency_ratio
  const actualPersonDays = data?.need_actual_work_min != null ? data.need_actual_work_min / WORK_MIN_PER_DAY : null
  const calPersonDays = data?.need_actual_calendar_min != null ? data.need_actual_calendar_min / WORK_MIN_PER_DAY : null

  return (
    <div className={`space-y-5 ${isLoading ? 'opacity-60 pointer-events-none' : ''}`}>
      {/* ① 标题栏 */}
      <header className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex items-start gap-3 min-w-0">
          <BackButton onClick={() => navigate(-1)} />
          <div className="min-w-0">
            <h1 className="text-xl font-bold text-gray-900 dark:text-white truncate">{project?.name || '项目详情'}</h1>
            {project?.description && <p className="text-sm text-gray-500 dark:text-gray-400 line-clamp-2">{project.description}</p>}
            <div className="mt-1.5 flex flex-wrap items-center gap-2 text-xs text-gray-400 dark:text-gray-500">
              <MetaChip>{dateRange}</MetaChip>
              <MetaChip>{data?.need_total_count ?? projectNeeds.length} Needs</MetaChip>
              <MetaChip>{contributors.length} 人</MetaChip>
            </div>
          </div>
        </div>
        <div className="flex items-center gap-2 shrink-0">
          <button
            type="button"
            onClick={() => setSourceAddOpen(true)}
            className="inline-flex items-center gap-1.5 bg-apple-blue hover:bg-apple-blue-hover text-white rounded-lg px-3 py-1.5 text-sm font-medium cursor-pointer transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
            </svg>
            添加来源
          </button>
          <button
            type="button"
            onClick={() => setEditOpen(true)}
            className="inline-flex items-center gap-1.5 glass text-gray-700 dark:text-gray-200 rounded-lg px-3 py-1.5 text-sm font-medium cursor-pointer transition-colors hover:text-apple-blue focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue"
          >
            编辑
          </button>
          <button
            type="button"
            onClick={() => setDeleteOpen(true)}
            className="inline-flex items-center gap-1.5 glass text-rose-500 rounded-lg px-3 py-1.5 text-sm font-medium cursor-pointer transition-colors hover:text-rose-600 focus:outline-none focus-visible:ring-2 focus-visible:ring-rose-400"
          >
            删除
          </button>
        </div>
      </header>

      {/* ② 核心指标（Need/branch 口径，守恒聚合，只计干净 Need） */}
      <section>
        <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-3">
          <MetricCard
            label="日历提效比"
            value={calR != null ? formatV2Ratio(calR) : '—'}
            tone={ratioTone(calR)}
            accent="#0071e3"
            tip="（基线日历时间 − 实际日历时间）÷ 实际日历时间；分子分母守恒，仅计干净 Need。业务主口径。"
          />
          <MetricCard
            label="工作量提效比"
            value={workR != null ? formatV2Ratio(workR) : '—'}
            tone={ratioTone(workR)}
            tip="（融合基线工时 − 实际活跃工时）÷ 实际活跃工时。内部下钻口径。"
          />
          <MetricCard
            label="AI 代码占比"
            value={data?.need_ai_code_ratio != null ? formatV2Ratio(data.need_ai_code_ratio) : '—'}
            tip="Σ ai_covered_loc ÷ Σ total_loc_net（干净 Need）。"
          />
          <MetricCard
            label="实际工时"
            value={actualPersonDays != null ? `${actualPersonDays.toFixed(1)} 人天` : '—'}
            hint={calPersonDays != null ? `日历跨度 ${calPersonDays.toFixed(1)} 人天` : undefined}
            tip="干净 Need 的实际活跃工时之和（÷480 折人天）。"
          />
          <MetricCard
            label="生成代码"
            value={data?.need_total_loc_net != null ? `${data.need_total_loc_net.toLocaleString()} 行` : '—'}
            tip="干净 Need 净 LOC 之和。"
          />
          <MetricCard
            label="合格 / 候选 Need"
            value={`${data?.need_eligible_count ?? 0} / ${data?.need_total_count ?? 0}`}
            hint={`自动剔除 ${data?.need_excluded_count ?? 0}`}
            tip="合格=已选且 coverage_eligible 且非 outlier；候选=看板口径全量；自动剔除=日历 outlier。"
          />
          <MetricCard
            label="费用"
            value={data?.need_cost != null && data.need_cost > 0 ? `¥${fmtCost(data.need_cost)}` : '¥0'}
            hint={`tokens 上 ${Math.round((data?.need_upstream_tokens ?? 0) / 1000)}k · 下 ${Math.round((data?.need_downstream_tokens ?? 0) / 1000)}k`}
            tip="干净 Need 会话的 token 成本之和（只计 coverage_eligible 且非 outlier，与其他卡同口径；按 session 去重；源数据缺 cost 时为 ¥0，tokens 仍真实）。"
          />
        </div>
      </section>

      {/* ③ 组成 · Needs（主角：来源规则 + 逐个勾选纳入/排除） */}
      <Panel
        title="组成 · Needs"
        hint={`候选 ${data?.need_total_count ?? projectNeeds.length} · 合格 ${data?.need_eligible_count ?? 0}`}
      >
        {/* 来源规则 chips */}
        <div className="mb-3 flex flex-wrap items-center gap-2">
          <span className="text-xs text-gray-400 dark:text-gray-500">Need 来源：</span>
          {repos.length === 0 ? (
            <span className="text-xs text-gray-400 dark:text-gray-500">未配置（点「添加来源」按仓库/分支纳入 Need）</span>
          ) : (
            repos.map((r, i) => (
              <span
                key={`${r.repo_addr}#${r.repo_branch}#${i}`}
                className="inline-flex items-center gap-1.5 rounded-full bg-apple-blue/10 dark:bg-white/10 px-2.5 py-1 text-xs text-gray-700 dark:text-gray-200"
              >
                <span className="font-mono truncate max-w-[220px]" title={`${r.repo_addr}${r.repo_branch ? ` @ ${r.repo_branch}` : ''}`}>
                  {shortRepo(r.repo_addr)}
                  {r.repo_branch ? ` @ ${r.repo_branch}` : ' @ 全部分支'}
                </span>
                <button
                  type="button"
                  onClick={() => setRemoveSource({ index: i, repo: r })}
                  aria-label={`移除来源 ${r.repo_addr}`}
                  className="text-gray-400 hover:text-rose-500 cursor-pointer bg-transparent border-none p-0 leading-none transition-colors focus:outline-none focus-visible:text-rose-500"
                >
                  <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              </span>
            ))
          )}
        </div>

        {needErr && <div className="mb-3 text-sm text-rose-600 dark:text-rose-400">{needErr}</div>}
        {(needsData?.stale_count ?? 0) > 0 && (
          <div className="mb-3 rounded-lg bg-amber-50 dark:bg-amber-900/20 px-3 py-2 text-xs text-amber-700 dark:text-amber-300">
            ⚠️ 配置中有 {needsData?.stale_count} 个已勾选/排除的 Need 因重算已失效（need_id 漂移），不再影响聚合；如需清理可移除对应来源后重加。
          </div>
        )}

        {projectNeeds.length === 0 ? (
          <Empty>候选池内暂无 Need（先在上方「添加来源」配置特性分支仓库）</Empty>
        ) : (
          <TableWrap>
            <thead>
              <tr className="border-b border-gray-200/50 dark:border-white/10">
                <th className={TH_CENTER}>纳入</th>
                <th className={TH}>Need</th>
                <th className={TH}>分支</th>
                <th className={TH}>边界源</th>
                <th className={TH_CENTER}>日历提效比</th>
                <th className={TH_CENTER}>工作量提效比</th>
                <th className={TH_CENTER}>AI占比</th>
                <th className={TH_CENTER}>状态</th>
                <th className={TH_NUM}>代码行</th>
              </tr>
            </thead>
            <tbody>
              {projectNeeds.map((n) => (
                <tr
                  key={n.need_id}
                  className={`border-b border-gray-100/50 dark:border-white/5 transition-colors ${n.excluded ? 'opacity-40' : 'hover:bg-apple-blue/5 dark:hover:bg-white/5'}`}
                >
                  <td className="px-3 py-2 align-middle text-center">
                    <input
                      type="checkbox"
                      checked={!n.excluded}
                      disabled={needBusy === n.need_id}
                      onChange={() => toggleNeed(n)}
                      className="w-4 h-4 accent-apple-blue cursor-pointer disabled:opacity-50"
                      aria-label={n.excluded ? `纳入 Need ${n.repo_branch}` : `排除 Need ${n.repo_branch}`}
                    />
                  </td>
                  <td className={TD}>
                    <LinkBtn onClick={() => navigate(`/needs/${encodeURIComponent(n.need_id)}`)}>
                      <span className="font-mono break-all" title={n.need_id}>
                        {n.need_id.length > 30 ? `${n.need_id.slice(0, 30)}…` : n.need_id}
                      </span>
                    </LinkBtn>
                  </td>
                  <td className={TD}>{n.repo_branch || '-'}</td>
                  <td className={TD}><span className="text-xs text-gray-400 dark:text-gray-500">{n.boundary_source}</span></td>
                  <td className="px-3 py-2 align-middle text-center"><RatioPill value={n.efficiency_ratio} /></td>
                  <td className="px-3 py-2 align-middle text-center"><RatioPill value={n.work_efficiency_ratio} /></td>
                  <td className="px-3 py-2 align-middle text-center"><RatioPill value={n.ai_code_ratio ?? null} /></td>
                  <td className="px-3 py-2 align-middle text-center"><NeedStatusTag n={n} /></td>
                  <td className={TD_NUM}>{n.total_loc_net != null ? n.total_loc_net.toLocaleString() : '-'}</td>
                </tr>
              ))}
            </tbody>
          </TableWrap>
        )}
      </Panel>

      {/* ④ 贡献者（从已选干净 Need 守恒派生） */}
      <Panel title="贡献者" hint={`${contributors.length} 人`}>
        {contributors.length === 0 ? (
          <Empty>暂无已选 Need 的贡献者</Empty>
        ) : (
          <TableWrap>
            <thead>
              <tr className="border-b border-gray-200/50 dark:border-white/10">
                <th className={TH}>用户</th>
                <th className={TH_NUM}>Needs</th>
                <th className={TH_CENTER}>日历提效比</th>
                <th className={TH_CENTER}>工作量提效比</th>
                <th className={TH_CENTER}>AI占比</th>
                <th className={TH_NUM}>代码行</th>
              </tr>
            </thead>
            <tbody>
              {contributors.map((c) => (
                <tr key={c.user_id} className="border-b border-gray-100/50 dark:border-white/5 hover:bg-apple-blue/5 dark:hover:bg-white/5 transition-colors">
                  <td className={TD} title={resolveName(c.user_id)}>
                    {c.user_id && c.user_id !== '未知' ? (
                      <LinkBtn onClick={() => navigate(`/user/${encodeURIComponent(c.user_id)}`)}>{resolveName(c.user_id)}</LinkBtn>
                    ) : (
                      resolveName(c.user_id)
                    )}
                  </td>
                  <td className={TD_NUM}>{c.needCount}</td>
                  <td className="px-3 py-2 align-middle text-center"><RatioPill value={c.calRatio} /></td>
                  <td className="px-3 py-2 align-middle text-center"><RatioPill value={c.workRatio} /></td>
                  <td className="px-3 py-2 align-middle text-center"><RatioPill value={c.aiRatio} /></td>
                  <td className={TD_NUM}>{c.loc.toLocaleString()}</td>
                </tr>
              ))}
            </tbody>
          </TableWrap>
        )}
      </Panel>

      {/* ---- Dialogs ---- */}
      {project && (
        <>
          <EditModal open={editOpen} project={project} repos={repos} onClose={() => setEditOpen(false)} onSaved={async () => { setEditOpen(false); await reload() }} projectId={projectId as string} />
          <SourceModal open={sourceAddOpen} existingRepos={repos} onClose={() => setSourceAddOpen(false)} onSaved={async () => { setSourceAddOpen(false); await reload() }} projectId={projectId as string} />
        </>
      )}

      <ConfirmModal
        open={deleteOpen}
        title="确认删除"
        message={`确定要删除项目「${project?.name || ''}」吗？此操作不可撤销。`}
        confirmLabel="删除"
        loading={deleting}
        onClose={() => setDeleteOpen(false)}
        onConfirm={confirmDelete}
      />
      <ConfirmModal
        open={!!removeSource}
        title="移除来源"
        message={`确定要移除 Need 来源「${removeSource?.repo.repo_addr || ''}${removeSource?.repo.repo_branch ? ` @ ${removeSource.repo.repo_branch}` : ''}」吗？该来源下的 Need 将不再计入本项目。`}
        confirmLabel="移除"
        onClose={() => setRemoveSource(null)}
        onConfirm={doRemoveSource}
      />
    </div>
  )
}

/** Need 干净度标签：不合格 / 日历异常 / 工作量异常 / 干净。
 * 「日历异常」对应核心指标「自动剔除」(日历口径)；「工作量异常」仅从工作量口径剔除、不计入「自动剔除」。 */
function NeedStatusTag({ n }: { n: ProjectNeedItem }) {
  if (!n.coverage_eligible) return <Tag tone="info" title="未交付或低置信，不计入提效比">不合格</Tag>
  if (n.calendar_outlier_flag) return <Tag tone="warning" title={n.reason || '日历口径异常，从日历提效比剔除'}>日历异常</Tag>
  if (n.work_outlier_flag) return <Tag tone="warning" title={n.reason || '工作量口径异常，仅从工作量提效比剔除'}>工作量异常</Tag>
  return <Tag tone="success">干净</Tag>
}

/** repo 地址压缩展示（去协议前缀、保留尾部可辨识段）。 */
function shortRepo(addr: string): string {
  const s = addr.replace(/^https?:\/\//, '').replace(/^git@/, '').replace(/\.git$/, '')
  return s.length > 28 ? `…${s.slice(-28)}` : s
}

/** repo 地址拆成「主名 + 路径」：取最后一段做主名，其余做灰字路径（仓库选择器用）。 */
function repoDisplay(addr: string): { name: string; path: string } {
  const s = addr.replace(/^https?:\/\//, '').replace(/^git@/, '').replace(/\.git$/, '')
  const segs = s.split(/[/:]/).filter(Boolean)
  const name = segs.length ? segs[segs.length - 1] : s
  const path = segs.slice(0, -1).join('/')
  return { name, path }
}

// ============ 编辑项目 dialog ============
function EditModal({
  open,
  project,
  repos,
  projectId,
  onClose,
  onSaved,
}: {
  open: boolean
  project: ProjectModel
  repos: ProjectRepo[]
  projectId: string
  onClose: () => void
  onSaved: () => Promise<void>
}) {
  const [name, setName] = useState('')
  const [desc, setDesc] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [err, setErr] = useState('')

  useEffect(() => {
    if (!open) return
    setName(project.name || '')
    setDesc(project.description || '')
    setErr('')
  }, [open, project])

  async function handleSubmit() {
    if (!name.trim()) {
      setErr('请输入项目名称')
      return
    }
    setSubmitting(true)
    setErr('')
    try {
      // ⚠️ 必须回传 repos 原值，否则后端清空（task_ids 已不属项目模型，后端忽略）。
      await updateProject(projectId, {
        name: name.trim(),
        description: (desc || '').trim(),
        repos,
      })
      await onSaved()
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : '保存失败')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <FormModal open={open} title="编辑项目" maxWidth={500} submitting={submitting} onClose={onClose} onSubmit={handleSubmit}>
      {err && <div className="text-sm text-rose-600 dark:text-rose-400">{err}</div>}
      <Field label="项目名称">
        <input type="text" value={name} onChange={(e) => setName(e.target.value)} className={INPUT} />
      </Field>
      <Field label="描述">
        <textarea rows={3} value={desc} onChange={(e) => setDesc(e.target.value)} className={`${INPUT} resize-y`} />
      </Field>
    </FormModal>
  )
}

// ============ 添加 Need 来源 dialog（仓库选择器：needs 同源、多选 + 可选分支细化） ============
// 取代旧的「手填 git 地址」：列表来自 /need-repo-options（与候选池同口径同源），勾选必命中。
type BranchMode = 'all' | 'specific'
interface RepoSelection {
  mode: BranchMode
  branches: Set<string>
}

function SourceModal({
  open,
  projectId,
  existingRepos,
  onClose,
  onSaved,
}: {
  open: boolean
  projectId: string
  existingRepos: ProjectRepo[]
  onClose: () => void
  onSaved: () => Promise<void>
}) {
  const [options, setOptions] = useState<NeedRepoOption[]>([])
  const [loading, setLoading] = useState(false)
  const [search, setSearch] = useState('')
  const [selection, setSelection] = useState<Map<string, RepoSelection>>(new Map())
  const [submitting, setSubmitting] = useState(false)
  const [err, setErr] = useState('')

  useEffect(() => {
    if (!open) return
    setSearch('')
    setSelection(new Map())
    setErr('')
    setLoading(true)
    getNeedRepoOptions()
      .then((res) => setOptions(res.data || []))
      .catch(() => setErr('加载仓库列表失败'))
      .finally(() => setLoading(false))
  }, [open])

  // 已配置来源（精确 repo_addr）标记"已添加"，避免重复勾选；仅 UI 提示，重复添加也无害。
  const existingAddrs = useMemo(() => new Set(existingRepos.map((r) => r.repo_addr)), [existingRepos])

  const filtered = useMemo(() => {
    const kw = search.trim().toLowerCase()
    if (!kw) return options
    return options.filter((o) => o.repo_addr.toLowerCase().includes(kw))
  }, [options, search])

  function toggleRepo(addr: string) {
    setSelection((prev) => {
      const next = new Map(prev)
      if (next.has(addr)) next.delete(addr)
      else next.set(addr, { mode: 'all', branches: new Set() })
      return next
    })
  }
  function setMode(addr: string, mode: BranchMode) {
    setSelection((prev) => {
      const next = new Map(prev)
      const cur = next.get(addr) || { mode: 'all', branches: new Set<string>() }
      next.set(addr, { ...cur, mode })
      return next
    })
  }
  function toggleBranch(addr: string, branch: string) {
    setSelection((prev) => {
      const next = new Map(prev)
      const cur = next.get(addr) || { mode: 'specific', branches: new Set<string>() }
      const bs = new Set(cur.branches)
      if (bs.has(branch)) bs.delete(branch)
      else bs.add(branch)
      next.set(addr, { mode: 'specific', branches: bs })
      return next
    })
  }

  const selectedCount = selection.size

  async function handleSubmit() {
    if (selectedCount === 0) {
      setErr('请至少勾选一个仓库')
      return
    }
    for (const [addr, sel] of selection) {
      if (sel.mode === 'specific' && sel.branches.size === 0) {
        setErr(`仓库「${repoDisplay(addr).name}」选了"指定分支"但未勾选任何分支`)
        return
      }
    }
    setSubmitting(true)
    setErr('')
    try {
      // 顺序逐条写：addRepoToProject 在后端是 read→append→write（无事务/加锁），并发会丢失更新
      // （都读同一份初始 repos，后写覆盖先写），故按序 await 让每条读到上一条的结果。
      for (const [addr, sel] of selection) {
        const branches = sel.mode === 'all' ? [''] : Array.from(sel.branches)
        for (const b of branches) {
          await addRepoToProject(projectId, {
            repo_addr: addr,
            repo_branch: b,
            start_time: null,
            end_time: null,
            exclude_commits: [],
            include_only_commits: [],
          })
        }
      }
      await onSaved()
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : '添加失败')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <FormModal
      open={open}
      title="添加 Need 来源"
      maxWidth={560}
      submitting={submitting}
      submitLabel={`加入${selectedCount ? ` (${selectedCount})` : ''}`}
      onClose={onClose}
      onSubmit={handleSubmit}
    >
      {err && <div className="text-sm text-rose-600 dark:text-rose-400">{err}</div>}
      <input
        type="text"
        value={search}
        onChange={(e) => setSearch(e.target.value)}
        className={INPUT}
        placeholder="🔍 搜索仓库名"
        aria-label="搜索仓库名"
      />
      <div className="max-h-[360px] overflow-y-auto -mx-1 px-1 space-y-1">
        {loading ? (
          <div className="py-6 text-center text-sm text-gray-400 dark:text-gray-500">加载中…</div>
        ) : filtered.length === 0 ? (
          <div className="py-6 text-center text-sm text-gray-400 dark:text-gray-500">无可选仓库</div>
        ) : (
          filtered.map((o) => {
            const sel = selection.get(o.repo_addr)
            const already = existingAddrs.has(o.repo_addr)
            const disp = repoDisplay(o.repo_addr)
            return (
              <div key={o.repo_addr} className="rounded-lg border border-gray-200/60 dark:border-white/10">
                <label className="flex items-center gap-2.5 px-3 py-2 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={!!sel}
                    disabled={already}
                    onChange={() => toggleRepo(o.repo_addr)}
                    className="w-4 h-4 accent-apple-blue cursor-pointer disabled:opacity-40"
                  />
                  <span className="min-w-0 flex-1 truncate">
                    <span className="font-medium text-gray-900 dark:text-white">{disp.name}</span>
                    {disp.path && <span className="ml-1.5 text-xs text-gray-400 dark:text-gray-500">{disp.path}</span>}
                    {already && <span className="ml-1.5 text-xs text-emerald-600 dark:text-emerald-400">已添加</span>}
                  </span>
                  <span className="shrink-0 text-xs text-gray-400 dark:text-gray-500 tabular-nums">
                    {o.need_count} Need · {fmtDate(o.last_active)}
                  </span>
                </label>
                {sel && (
                  <div className="px-3 pb-2.5 pl-9 space-y-1.5">
                    <div className="flex items-center gap-3 text-xs text-gray-600 dark:text-gray-300">
                      <label className="inline-flex items-center gap-1 cursor-pointer">
                        <input type="radio" name={`bm-${o.repo_addr}`} checked={sel.mode === 'all'} onChange={() => setMode(o.repo_addr, 'all')} className="accent-apple-blue cursor-pointer" />
                        全部特性分支
                      </label>
                      <label className="inline-flex items-center gap-1 cursor-pointer">
                        <input type="radio" name={`bm-${o.repo_addr}`} checked={sel.mode === 'specific'} onChange={() => setMode(o.repo_addr, 'specific')} className="accent-apple-blue cursor-pointer" />
                        指定分支
                      </label>
                    </div>
                    {sel.mode === 'specific' && (
                      <div className="flex flex-wrap gap-1.5 pt-0.5">
                        {o.branches.length === 0 ? (
                          <span className="text-xs text-gray-400 dark:text-gray-500">该仓库无特性分支</span>
                        ) : (
                          o.branches.map((b) => {
                            const on = sel.branches.has(b.repo_branch)
                            return (
                              <button
                                type="button"
                                key={b.repo_branch}
                                onClick={() => toggleBranch(o.repo_addr, b.repo_branch)}
                                className={`rounded-full px-2 py-0.5 text-xs border transition-colors cursor-pointer focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue ${on ? 'bg-apple-blue text-white border-apple-blue' : 'border-gray-300 dark:border-white/15 text-gray-600 dark:text-gray-300 hover:border-apple-blue'}`}
                              >
                                {b.repo_branch} <span className={on ? 'text-white/70' : 'text-gray-400'}>{b.need_count}</span>
                              </button>
                            )
                          })
                        )}
                      </div>
                    )}
                  </div>
                )}
              </div>
            )
          })
        )}
      </div>
      <p className="text-xs text-gray-400 dark:text-gray-500">
        勾选仓库即纳入其全部特性分支（已交付、非主干）的 Need；可改"指定分支"细选。加入后在下方列表逐个勾选/排除。
      </p>
    </FormModal>
  )
}

// ============ 通用子组件 ============
const INPUT =
  'glass rounded-lg px-3 py-1.5 text-sm w-full bg-transparent text-gray-900 dark:text-white ' +
  'focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue'

const TH = 'px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_NUM = 'px-3 py-2 text-right font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TH_CENTER = 'px-3 py-2 text-center font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
const TD = 'px-3 py-2 align-middle text-gray-700 dark:text-gray-200'
const TD_NUM = 'px-3 py-2 align-middle text-right tabular-nums text-gray-700 dark:text-gray-200'

function MetaChip({ children }: { children: ReactNode }) {
  return <span className="inline-flex items-center rounded-full bg-gray-100 dark:bg-white/10 px-2 py-0.5">{children}</span>
}

function BackButton({ onClick }: { onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="inline-flex items-center gap-1 text-sm text-gray-500 dark:text-gray-400 hover:text-apple-blue cursor-pointer bg-transparent border-none p-0 mt-1 transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue rounded"
    >
      <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
      </svg>
      返回
    </button>
  )
}

function Panel({ title, hint, action, children }: { title: string; hint?: string; action?: ReactNode; children: ReactNode }) {
  return (
    <section className="glass rounded-2xl overflow-hidden">
      <div className="flex items-center justify-between px-5 py-3 border-b border-gray-200/50 dark:border-white/10">
        <div className="flex items-center gap-2">
          <span className="text-sm font-semibold text-gray-700 dark:text-gray-200">{title}</span>
          {hint ? <span className="text-xs text-gray-400 dark:text-gray-500">{hint}</span> : null}
        </div>
        {action}
      </div>
      <div className="p-5">{children}</div>
    </section>
  )
}

function TableWrap({ children }: { children: ReactNode }) {
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm border-collapse">{children}</table>
    </div>
  )
}

function Empty({ children }: { children: ReactNode }) {
  return <div className="py-6 text-center text-sm text-gray-400 dark:text-gray-500">{children}</div>
}

function LinkBtn({ onClick, children }: { onClick: () => void; children: ReactNode }) {
  return (
    <button
      type="button"
      onClick={(e) => { e.stopPropagation(); onClick() }}
      className="text-apple-blue hover:text-apple-blue-hover cursor-pointer bg-transparent border-none p-0 text-left break-all focus:outline-none focus-visible:underline"
    >
      {children}
    </button>
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

/** 表单 Modal 壳（取消/提交 footer）。 */
function FormModal({
  open,
  title,
  maxWidth,
  submitting,
  submitLabel = '保存',
  onClose,
  onSubmit,
  children,
}: {
  open: boolean
  title: string
  maxWidth?: number
  submitting: boolean
  submitLabel?: string
  onClose: () => void
  onSubmit: () => void
  children: ReactNode
}) {
  return (
    <Modal
      open={open}
      title={title}
      maxWidth={maxWidth}
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
            onClick={onSubmit}
            disabled={submitting}
            className="bg-apple-blue hover:bg-apple-blue-hover text-white rounded-lg px-4 py-1.5 text-sm font-medium cursor-pointer transition-colors disabled:opacity-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue"
          >
            {submitting ? '保存中...' : submitLabel}
          </button>
        </>
      }
    >
      <div className="space-y-3">{children}</div>
    </Modal>
  )
}

/** 确认 Modal（删除/移除 通用）。 */
function ConfirmModal({
  open,
  title,
  message,
  confirmLabel,
  loading = false,
  onClose,
  onConfirm,
}: {
  open: boolean
  title: string
  message: string
  confirmLabel: string
  loading?: boolean
  onClose: () => void
  onConfirm: () => void | Promise<void>
}) {
  const [busy, setBusy] = useState(false)
  async function handle() {
    setBusy(true)
    try {
      await onConfirm()
    } finally {
      setBusy(false)
    }
  }
  const disabled = busy || loading
  return (
    <Modal
      open={open}
      title={title}
      maxWidth={420}
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
            onClick={handle}
            disabled={disabled}
            className="bg-rose-500 hover:bg-rose-600 text-white rounded-lg px-4 py-1.5 text-sm font-medium cursor-pointer transition-colors disabled:opacity-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-rose-400"
          >
            {disabled ? '处理中...' : confirmLabel}
          </button>
        </>
      }
    >
      <p className="text-sm text-gray-700 dark:text-gray-200">{message}</p>
    </Modal>
  )
}
