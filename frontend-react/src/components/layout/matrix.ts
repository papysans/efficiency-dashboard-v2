// 主体 × 维度矩阵的共享真源：类型、有序列表、点亮矩阵、聚焦 context。
// IA = 维度优先：一级导航选维度（使用/效率/成本/贡献），内层 Tab 选主体（组织/个人/项目/仓库）。
// 维度组件通过 useEntityFocus() 读 {entity, object}、内部按 entity 字符串分支，不依赖 URL 路径段顺序
// —— 故「主体↔维度」轴翻转时维度组件零逻辑改动，仅本模块成为新的导入锚点。
import { useOutletContext } from 'react-router'

export type Entity = 'org' | 'user' | 'project' | 'repo'
export type Dimension = 'usage' | 'efficiency' | 'cost' | 'contribution'

/** 有序主体列表（内层 Tab 渲染顺序；首项=默认落位）。 */
export const ENTITIES: ReadonlyArray<{ key: Entity; label: string }> = [
  { key: 'org', label: '组织' },
  { key: 'user', label: '个人' },
  { key: 'project', label: '项目' },
  { key: 'repo', label: '仓库' },
] as const

/** 有序维度列表（一级导航渲染顺序；首项=裸主体重定向的默认维度）。
 *  「质量」维已移除：AI 服务健康度（非代码质量）下沉到「设置 › 平台运维」。 */
export const DIMENSIONS: ReadonlyArray<{ key: Dimension; label: string }> = [
  { key: 'usage', label: '使用' },
  { key: 'efficiency', label: '效率' },
  { key: 'cost', label: '成本' },
  { key: 'contribution', label: '贡献' },
] as const

/** 维度页默认落位的主体（决策：组织=全公司视角）。 */
export const DEFAULT_ENTITY: Entity = 'org'
/** 裸主体/旧链回退时的默认维度（与旧「index→usage」对称）。 */
export const DEFAULT_DIMENSION: Dimension = 'usage'

// 标签真源唯一收敛到 ENTITIES/DIMENSIONS（有序列表），record 由其派生，避免双写漂移。
export const ENTITY_LABEL = Object.fromEntries(ENTITIES.map((e) => [e.key, e.label])) as Record<Entity, string>
export const DIMENSION_LABEL = Object.fromEntries(DIMENSIONS.map((d) => [d.key, d.label])) as Record<Dimension, string>

// per-cell 点亮矩阵（false = 灰显/建设中，不可点）。当前 4×4 全亮，保留机制以备后续灰显。
const ENABLED: Record<Entity, Record<Dimension, boolean>> = {
  org: { usage: true, efficiency: true, cost: true, contribution: true },
  user: { usage: true, efficiency: true, cost: true, contribution: true },
  project: { usage: true, efficiency: true, cost: true, contribution: true },
  repo: { usage: true, efficiency: true, cost: true, contribution: true },
}

/** 某主体某维度是否点亮（Tab 灰显/路由守卫可复用）。 */
export function isEnabled(entity: Entity, dim: Dimension): boolean {
  return ENABLED[entity]?.[dim] ?? false
}

/** URL param 的 entity 段守卫（脏值如 /usage/garbage 时回退默认主体）。 */
export function isEntity(v: string | undefined): v is Entity {
  return v === 'org' || v === 'user' || v === 'project' || v === 'repo'
}

// 维度页路径形如 /:dim/:entity。维度段白名单从 DIMENSIONS 派生（单一真源，避免散落硬编码正则）。
const DIM_PATH_RE = new RegExp(`^/(?:${DIMENSIONS.map((d) => d.key).join('|')})/([^/]+)`)

/** 若 pathname 是维度页 /:dim/:entity，返回其合法主体 entity；否则 null（总览/详情页等）。 */
export function entityFromPath(pathname: string): Entity | null {
  const m = pathname.match(DIM_PATH_RE)
  return m && isEntity(m[1]) ? m[1] : null
}

/** 当前是否为「组织主体」维度页（/:dim/org）——ScrollToTop 用于组织树内导航不滚顶。 */
export function isOrgEntityPath(pathname: string): boolean {
  return entityFromPath(pathname) === 'org'
}

/** 维度内容从 Outlet context 读 entity + 当前聚焦对象（单一数据源 = ?object=）。 */
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
