// 内层主体 Tab（维度优先 IA）：在某维度页内切主体（组织/个人/项目/仓库）→ /:dim/:entity。
// ⚠️ 切主体即换 id 空间（org dept_id ≠ user_id），NavLink 故意**不带** ?object= —— 清掉聚焦/分布子 tab，
//    回到该主体的聚合态。这与旧 DimensionTabs「切维度保留 object」正好相反，是轴翻转后跟着改的核心逻辑。
// 灰显沿用 isEnabled(entity, dim)（当前 4×4 全亮，保留机制）。风格对齐玻璃拟态。
import { NavLink, useLocation } from 'react-router'
import { ENTITIES, isEnabled, type Dimension } from '@/components/layout/matrix'

export function EntityTabs({ dim }: { dim: Dimension }) {
  const { pathname } = useLocation()
  return (
    <div className="flex flex-wrap items-center gap-1" role="tablist" aria-label="主体">
      {ENTITIES.map(({ key, label }) => {
        const on = isEnabled(key, dim)
        if (!on) {
          return (
            <span
              key={key}
              role="tab"
              aria-selected={false}
              aria-disabled="true"
              title="该主体此维度数据建设中"
              className="px-3 py-1.5 rounded-lg text-sm font-medium select-none text-gray-300 dark:text-gray-600 cursor-not-allowed"
            >
              {label}
            </span>
          )
        }
        // active 单一来源（pathname === /:dim/:entity）同时驱动视觉态与 aria-selected，
        // 对齐项目既有 role="tab" 规范（TopRankCard 的 TabBtn）——屏幕阅读器据此判断当前激活项。
        const to = `/${dim}/${key}`
        const active = pathname === to
        return (
          <NavLink
            key={key}
            to={to}
            role="tab"
            aria-selected={active}
            className={`px-3 py-1.5 rounded-lg text-sm font-medium no-underline transition-colors cursor-pointer ${
              active
                ? 'bg-apple-blue text-white'
                : 'text-gray-600 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white hover:bg-white/50 dark:hover:bg-white/10'
            }`}
          >
            {label}
          </NavLink>
        )
      })}
    </div>
  )
}
