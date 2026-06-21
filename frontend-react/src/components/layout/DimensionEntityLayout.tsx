// 维度 × 主体 下钻共用壳（usage/efficiency/cost/contribution 四维共用）。
// 结构：维度标题(H1) + 面包屑(维度 › 主体 › 聚焦对象) + 可搜索对象选择器(聚合↔聚焦) + <EntityTabs/> + <Outlet/>。
// dim 由 router 的 dimensionRoute() 以 prop 传入；entity 来自 URL param（/:dim/:entity），脏值回退默认主体。
//
// 聚焦对象（focus object）单一数据源 = URL query ?object=<id>（深链/刷新保持）。切主体 Tab 会丢 object
// （EntityTabs 不带 query，故意，换 id 空间），但切维度（顶部导航）保留 entity+object（AppShell 动态链接）。
// 维度内容（EfficiencyDimension 等）通过 useEntityFocus() 读 entity+object —— 对轴翻转无感。
import { useCallback, useMemo, useState } from 'react'
import { Navigate, Outlet, useParams, useSearchParams } from 'react-router'
import { EntityTabs } from '@/components/ui/EntityTabs'
import { ObjectSelector } from '@/components/ui/ObjectSelector'
import { CreateProjectModal } from '@/components/projects/CreateProjectModal'
import { useEntityObjects, type EntityOption } from '@/hooks/useEntityObjects'
import {
  DEFAULT_ENTITY,
  DIMENSION_LABEL,
  ENTITY_LABEL,
  isEntity,
  type Dimension,
  type Entity,
  type EntityFocusContext,
} from '@/components/layout/matrix'

const OBJECT_KEY = 'object'

// 外层只做 entity 脏值守卫：仅 useParams 一个 Hook 无条件调用，非法即在任何数据 Hook 之前重定向，
// 避免脏值路由（/usage/garbage）白跑一次 useEntityObjects 取数。校验通过后交给 Shell 渲染真内容。
export default function DimensionEntityLayout({ dim }: { dim: Dimension }) {
  const { entity } = useParams<{ entity: string }>()
  if (!isEntity(entity)) return <Navigate to={`/${dim}/${DEFAULT_ENTITY}`} replace />
  return <DimensionEntityShell dim={dim} entity={entity} />
}

function DimensionEntityShell({ dim, entity }: { dim: Dimension; entity: Entity }) {
  const [searchParams, setSearchParams] = useSearchParams()
  const object = searchParams.get(OBJECT_KEY) || ''

  const { options, loading } = useEntityObjects(entity)

  // 「创建项目」仅在主体 Tab=项目 时显示（任意维度页下）。
  const [createOpen, setCreateOpen] = useState(false)

  const objectLabel = useMemo<string>(() => {
    if (!object) return ''
    const hit = options.find((o: EntityOption) => o.value === object)
    return hit ? hit.label.trim() : object
  }, [object, options])

  // 写聚焦对象到 URL（保留其它 query）。切换聚焦（进或出）一律清掉效率页的 ?sub ——
  // 分布是聚合态全局视图，聚焦后无意义；退出聚焦也应回「概览」而非停在分布。
  const onSelect = useCallback(
    (value: string) => {
      const next = new URLSearchParams(searchParams)
      if (value) next.set(OBJECT_KEY, value)
      else next.delete(OBJECT_KEY)
      next.delete('sub')
      setSearchParams(next, { replace: false })
    },
    [searchParams, setSearchParams],
  )

  const dimTitle = DIMENSION_LABEL[dim]
  const entityTitle = ENTITY_LABEL[entity]
  const focus: EntityFocusContext = { entity, object, objectLabel }

  return (
    <div className="flex flex-col gap-5">
      {/* 顶部：维度标题 + 面包屑(维度 › 主体 › 对象) + 对象选择器 */}
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="min-w-0">
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">{dimTitle}</h1>
          {/* 面包屑：维度 › 主体 ›（聚焦时）对象名。点主体名可清聚焦回聚合态。 */}
          <nav aria-label="面包屑" className="mt-1 text-xs text-gray-400 dark:text-gray-500 flex items-center gap-1.5 select-none">
            <span className="text-gray-400 dark:text-gray-500">{dimTitle}</span>
            <span aria-hidden="true">›</span>
            <button
              type="button"
              onClick={() => onSelect('')}
              disabled={!object}
              className={`bg-transparent border-none p-0 ${
                object ? 'cursor-pointer hover:text-apple-blue' : 'cursor-default text-gray-400 dark:text-gray-500'
              } focus:outline-none focus-visible:underline`}
            >
              {entityTitle}
            </button>
            {object && (
              <>
                <span aria-hidden="true">›</span>
                <span className="text-gray-600 dark:text-gray-300 truncate max-w-[18rem]" title={objectLabel}>
                  {objectLabel || object}
                </span>
              </>
            )}
          </nav>
        </div>

        {/* 对象选择器（聚合↔聚焦）+「创建项目」按钮（仅主体 Tab=项目） */}
        <div className="shrink-0 flex items-center gap-2">
          {entity === 'project' && (
            <button
              type="button"
              onClick={() => setCreateOpen(true)}
              className="inline-flex items-center gap-1.5 bg-apple-blue hover:bg-apple-blue-hover text-white rounded-lg px-3 py-1.5 text-sm font-medium cursor-pointer transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue"
            >
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
              </svg>
              创建项目
            </button>
          )}
          <ObjectSelector
            options={options}
            value={object}
            onChange={onSelect}
            loading={loading}
            allLabel={`全部${entityTitle}`}
            placeholder={`搜索${entityTitle}…`}
          />
        </div>
      </div>

      {/* 主体 Tab（切主体换 id 空间，清 ?object=） */}
      <EntityTabs dim={dim} />

      {/* 维度内容（通过 useEntityFocus 读 entity + object） */}
      <div>
        <Outlet context={focus} />
      </div>

      {entity === 'project' && (
        <CreateProjectModal open={createOpen} onClose={() => setCreateOpen(false)} />
      )}
    </div>
  )
}
