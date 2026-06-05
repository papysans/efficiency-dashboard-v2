// 组织详情页（OrgDetailV2 的 React + 玻璃拟态迁移）。
// 分区/列/口径/图表 1:1 按 research/pr3-user-repo-org.md §Org-7；⚠️ 百分比口径 → PercentPill（不 ×100）。
//
// 主体（summary/成员/时序表/图表）已抽到 OrgDetailPanel（与 OrgTree 右栏共用），本页只保留
// 标题栏 + 四级级联 + 粒度/日期，把 orgPath/dateRange/granularity 传给 panel。
//
// 照搬陷阱（勿"修复"）：
//  ① 级联选项走 getOrgV2(level,parent)，但 /v2/orgs 命中 native handler **忽略 level/parent**，每次返回
//     全部顶层 org → 四级级联在 native 下退化。React 照搬现状（不改成 listOrgV2）。
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router'
import { getOrgV2 } from '@/api/endpoints'
import { getDefaultDateRangeWide } from '@/lib/date'
import { DateRangePicker } from '@/components/ui/DateRangePicker'
import { OrgDetailPanel } from './OrgDetailPanel'

function normalizeDateQuery(value: string | null): string {
  if (!value) return ''
  const s = String(value).trim()
  if (/^\d{8}$/.test(s)) return `${s.slice(0, 4)}-${s.slice(4, 6)}-${s.slice(6, 8)}`
  if (/^\d{4}-\d{2}-\d{2}$/.test(s)) return s
  return ''
}

const GRANULARITY: Array<{ label: string; value: string }> = [
  { label: '天', value: 'day' },
  { label: '周', value: 'week' },
  { label: '月', value: 'month' },
  { label: '年', value: 'year' },
]

export default function OrgDetail() {
  const { orgPath: orgPathRaw } = useParams<{ orgPath: string }>()
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()

  // 路由 param 由 React Router 已 decode；按 '/' 拆级。
  const parts = useMemo(() => (orgPathRaw || '').split('/').filter(Boolean), [orgPathRaw])
  const [org1, setOrg1] = useState(parts[0] || '')
  const [org2, setOrg2] = useState(parts[1] || '')
  const [org3, setOrg3] = useState(parts[2] || '')
  const [org4, setOrg4] = useState(parts[3] || '')

  // URL param 变化时回填级联
  useEffect(() => {
    setOrg1(parts[0] || '')
    setOrg2(parts[1] || '')
    setOrg3(parts[2] || '')
    setOrg4(parts[3] || '')
  }, [parts])

  const [granularity, setGranularity] = useState('day')
  const [org1Options, setOrg1Options] = useState<string[]>([])
  const [org2Options, setOrg2Options] = useState<string[]>([])
  const [org3Options, setOrg3Options] = useState<string[]>([])
  const [org4Options, setOrg4Options] = useState<string[]>([])

  const dateRange = useMemo<[string, string]>(() => {
    const start = normalizeDateQuery(searchParams.get('startDate'))
    const end = normalizeDateQuery(searchParams.get('endDate'))
    if (start && end) return [start, end]
    return getDefaultDateRangeWide()
  }, [searchParams])

  const dateParams = useMemo(
    () => ({ startDate: dateRange[0].replace(/-/g, ''), endDate: dateRange[1].replace(/-/g, '') }),
    [dateRange],
  )

  const currentOrgPath = useMemo(() => [org1, org2, org3, org4].filter(Boolean).join('/'), [org1, org2, org3, org4])

  // 级联选项：getOrgV2(忽略 level/parent，返回全部顶层 org_name) —— 照搬 Vue native 退化行为。
  const loadOrgOptions = useCallback(async (): Promise<string[]> => {
    try {
      const res = await getOrgV2(dateParams)
      return (res.data || []).map((r) => r.org_name).filter(Boolean)
    } catch {
      return []
    }
  }, [dateParams])

  // 初始加载一级选项（及已选层级的下级选项）
  useEffect(() => {
    let aborted = false
    loadOrgOptions().then((opts) => {
      if (aborted) return
      setOrg1Options(opts)
      if (org1) setOrg2Options(opts)
      if (org2) setOrg3Options(opts)
      if (org3) setOrg4Options(opts)
    })
    return () => {
      aborted = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [loadOrgOptions])

  // 同步 orgPath 到 URL 并刷新（清空下级 + 加载下级选项由各 onChange 处理）。
  const syncUrlAndFetch = useCallback(
    (path: string) => {
      const q = new URLSearchParams({ startDate: dateParams.startDate, endDate: dateParams.endDate })
      navigate({ pathname: `/org/${encodeURIComponent(path)}`, search: `?${q.toString()}` }, { replace: true })
    },
    [dateParams, navigate],
  )

  async function onOrg1Change(val: string) {
    setOrg1(val)
    setOrg2('')
    setOrg3('')
    setOrg4('')
    if (val) setOrg2Options(await loadOrgOptions())
    syncUrlAndFetch([val].filter(Boolean).join('/'))
  }
  async function onOrg2Change(val: string) {
    setOrg2(val)
    setOrg3('')
    setOrg4('')
    if (val) setOrg3Options(await loadOrgOptions())
    syncUrlAndFetch([org1, val].filter(Boolean).join('/'))
  }
  async function onOrg3Change(val: string) {
    setOrg3(val)
    setOrg4('')
    if (val) setOrg4Options(await loadOrgOptions())
    syncUrlAndFetch([org1, org2, val].filter(Boolean).join('/'))
  }
  function onOrg4Change(val: string) {
    setOrg4(val)
    syncUrlAndFetch([org1, org2, org3, val].filter(Boolean).join('/'))
  }

  function onDateChange(range: [string, string]) {
    const next = new URLSearchParams(searchParams)
    next.set('startDate', range[0].replace(/-/g, ''))
    next.set('endDate', range[1].replace(/-/g, ''))
    setSearchParams(next, { replace: true })
  }

  const selectCls =
    'glass rounded-lg px-2 py-1.5 text-sm bg-transparent cursor-pointer text-gray-700 dark:text-gray-200 ' +
    'focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue disabled:opacity-50 min-w-[120px]'

  return (
    <div className="space-y-5">
      {/* 标题栏 + 级联 + 粒度 */}
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-3 min-w-0">
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
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">组织详情</h1>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <select value={org1} onChange={(e) => onOrg1Change(e.target.value)} className={selectCls} aria-label="一级组织">
            <option value="">一级组织</option>
            {org1Options.map((o) => (
              <option key={o} value={o}>{o}</option>
            ))}
          </select>
          <select value={org2} onChange={(e) => onOrg2Change(e.target.value)} disabled={!org1} className={selectCls} aria-label="二级组织">
            <option value="">二级组织</option>
            {org2Options.map((o) => (
              <option key={o} value={o}>{o}</option>
            ))}
          </select>
          <select value={org3} onChange={(e) => onOrg3Change(e.target.value)} disabled={!org2} className={selectCls} aria-label="三级组织">
            <option value="">三级组织</option>
            {org3Options.map((o) => (
              <option key={o} value={o}>{o}</option>
            ))}
          </select>
          <select value={org4} onChange={(e) => onOrg4Change(e.target.value)} disabled={!org3} className={selectCls} aria-label="四级组织">
            <option value="">四级组织</option>
            {org4Options.map((o) => (
              <option key={o} value={o}>{o}</option>
            ))}
          </select>
          <DateRangePicker value={dateRange} onChange={onDateChange} />
          <select value={granularity} onChange={(e) => setGranularity(e.target.value)} className={`${selectCls} min-w-[80px]`} aria-label="粒度">
            {GRANULARITY.map((g) => (
              <option key={g.value} value={g.value}>{g.label}</option>
            ))}
          </select>
        </div>
      </header>

      <OrgDetailPanel orgPath={currentOrgPath} dateRange={dateRange} granularity={granularity} />
    </div>
  )
}
