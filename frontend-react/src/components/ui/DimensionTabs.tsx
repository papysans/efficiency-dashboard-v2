// 维度 Tab（4 维：使用/效率/成本/贡献）。
// NavLink → /:entity/:dim（dim=usage|efficiency|cost|contribution）；当前高亮 dim 来自 URL。
// 每维是否点亮/灰显由 per-entity 配置决定（见 ENTITY_DIM_ENABLED）。当前 4 维全主体均点亮。
// 「质量」维已移除：AI 服务健康度（非代码质量）下沉到「设置 › 平台运维 › AI 服务健康度」，
//   真代码质量暂无数据源。
// 风格对齐玻璃拟态：选中态 bg-apple-blue 白字，未选中 hover 浅底；灰显态 not-allowed、不可点。
// ⚠️ 切 Tab 必须带上当前 ?object=（聚焦对象在 query 里），否则换维度会丢聚焦态 → 用 useLocation().search 续传。
import { NavLink, useLocation } from 'react-router'

export type Dimension = 'usage' | 'efficiency' | 'cost' | 'contribution'
export type Entity = 'org' | 'user' | 'project' | 'repo'

export const DIMENSIONS: ReadonlyArray<{ key: Dimension; label: string }> = [
  { key: 'usage', label: '使用' },
  { key: 'efficiency', label: '效率' },
  { key: 'cost', label: '成本' },
  { key: 'contribution', label: '贡献' },
] as const

// per-entity 每维是否点亮（false = 灰显/建设中，不可点）。当前 4 维对全主体均点亮（保留机制以备后续维度灰显）。
const ENTITY_DIM_ENABLED: Record<Entity, Record<Dimension, boolean>> = {
  org: { usage: true, efficiency: true, cost: true, contribution: true },
  user: { usage: true, efficiency: true, cost: true, contribution: true },
  project: { usage: true, efficiency: true, cost: true, contribution: true },
  repo: { usage: true, efficiency: true, cost: true, contribution: true },
}

/** 某主体某维度是否点亮（路由守卫/内容兜底可复用）。 */
export function isDimensionEnabled(entity: Entity, dim: Dimension): boolean {
  return ENTITY_DIM_ENABLED[entity]?.[dim] ?? false
}

export function DimensionTabs({ entity }: { entity: Entity }) {
  const enabled = ENTITY_DIM_ENABLED[entity]
  const { search } = useLocation()
  return (
    <div className="flex flex-wrap items-center gap-1" role="tablist" aria-label="维度">
      {DIMENSIONS.map(({ key, label }) => {
        const on = enabled?.[key] ?? false
        if (!on) {
          return (
            <span
              key={key}
              role="tab"
              aria-disabled="true"
              title="该维度数据建设中"
              className="px-3 py-1.5 rounded-lg text-sm font-medium select-none text-gray-300 dark:text-gray-600 cursor-not-allowed"
            >
              {label}
            </span>
          )
        }
        return (
          <NavLink
            key={key}
            to={`/${entity}/${key}${search}`}
            role="tab"
            className={({ isActive }) =>
              `px-3 py-1.5 rounded-lg text-sm font-medium no-underline transition-colors cursor-pointer ${
                isActive
                  ? 'bg-apple-blue text-white'
                  : 'text-gray-600 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white hover:bg-white/50 dark:hover:bg-white/10'
              }`
            }
          >
            {label}
          </NavLink>
        )
      })}
    </div>
  )
}
