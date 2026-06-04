/** 通用占位页 —— 让 24 条路由在 PR0 阶段可点不 404，后续 PR 逐个替换为真实页面。 */
export default function Placeholder({ title }: { title: string }) {
  return (
    <div className="glass rounded-2xl p-10 text-center">
      <h2 className="text-xl font-semibold text-gray-900 dark:text-white mb-2">{title}</h2>
      <p className="text-sm text-gray-500 dark:text-gray-400">该页面将在后续 PR 迁移到 React（玻璃拟态）。</p>
    </div>
  )
}
