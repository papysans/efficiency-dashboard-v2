// 主体×维度下钻共用壳（org/user/project/repo 四下钻共用）。
// 结构：主体标题 + 面包屑 + 可搜索对象选择器（聚合↔聚焦切换）+ <DimensionTabs entity/> + <Outlet/>。
// entity 由 router.tsx 的 entityRoute() 以 prop 传入（静态段 path 不产生路由参数，故走 prop）。
//
// 聚焦对象（focus object）单一数据源 = URL query ?object=<id>（深链/刷新保持，切维度 Tab 不丢，
// 因为切 Tab 只换 path 段不动 query）。维度内容（EfficiencyDimension 等）通过 useEntityFocus() 读 entity+object。
import { useCallback, useMemo } from 'react'
import { Outlet, useOutletContext, useSearchParams } from 'react-router'
import { DimensionTabs, type Entity } from '@/components/ui/DimensionTabs'
import { ObjectSelector } from '@/components/ui/ObjectSelector'
import { useEntityObjects, type EntityOption } from '@/hooks/useEntityObjects'

const ENTITY_TITLE: Record<Entity, string> = {
  org: '组织',
  user: '个人',
  project: '项目',
  repo: '仓库',
}

/** 维度内容从 Outlet context 读 entity + 当前聚焦对象。 */
export interface EntityFocusContext {
  entity: Entity
  /** 聚焦对象 id（空串=聚合态/全部）。 */
  object: string
  /** 聚焦对象显示名（聚合态为空）。 */
  objectLabel: string
}

export function useEntityFocus(): EntityFocusContext {
  return useOutletContext<EntityFocusContext>()
}

const OBJECT_KEY = 'object'

export default function EntityDimensionLayout({ entity }: { entity: Entity }) {
  const title = ENTITY_TITLE[entity]
  const [searchParams, setSearchParams] = useSearchParams()
  const object = searchParams.get(OBJECT_KEY) || ''

  const { options, loading } = useEntityObjects(entity)

  const objectLabel = useMemo<string>(() => {
    if (!object) return ''
    const hit = options.find((o: EntityOption) => o.value === object)
    return hit ? hit.label.trim() : object
  }, [object, options])

  // 写聚焦对象到 URL（保留其它 query；选「全部」→ 删 object 回聚合态）。
  // 进入聚焦同时清掉效率页的 ?sub（分布是聚合态的全局视图，聚焦后无意义）——退出聚焦回到「概览」而非意外停在分布。
  const onSelect = useCallback(
    (value: string) => {
      const next = new URLSearchParams(searchParams)
      if (value) {
        next.set(OBJECT_KEY, value)
        next.delete('sub')
      } else {
        next.delete(OBJECT_KEY)
      }
      setSearchParams(next, { replace: false })
    },
    [searchParams, setSearchParams],
  )

  const focus: EntityFocusContext = { entity, object, objectLabel }

  return (
    <div className="flex flex-col gap-5">
      {/* 顶部：主体标题 + 面包屑 + 对象选择器 */}
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="min-w-0">
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">{title}</h1>
          {/* 面包屑：主体 ›（聚焦时）对象名 */}
          <nav aria-label="面包屑" className="mt-1 text-xs text-gray-400 dark:text-gray-500 flex items-center gap-1.5 select-none">
            <button
              type="button"
              onClick={() => onSelect('')}
              disabled={!object}
              className={`bg-transparent border-none p-0 ${
                object ? 'cursor-pointer hover:text-apple-blue' : 'cursor-default text-gray-400 dark:text-gray-500'
              } focus:outline-none focus-visible:underline`}
            >
              {title}
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

        {/* 对象选择器：聚合↔聚焦 */}
        <div className="shrink-0">
          <ObjectSelector
            options={options}
            value={object}
            onChange={onSelect}
            loading={loading}
            allLabel={`全部${title}`}
            placeholder={`搜索${title}…`}
          />
        </div>
      </div>

      {/* 维度 Tab（切 Tab 只换 path 段，保留 ?object= query） */}
      <DimensionTabs entity={entity} />

      {/* 维度内容（通过 useEntityFocus 读 entity + object） */}
      <div>
        <Outlet context={focus} />
      </div>
    </div>
  )
}
