// 统一占位卡：项目/仓库的「质量」维度数据建设中（无代码质量数据源）。
// 玻璃拟态，文案明确区分「代码质量」与平台「AI 服务健康度」（见 prd 质量维度口径决策②）。
export function QualityComingSoon() {
  return (
    <div className="glass rounded-2xl p-10 text-center">
      <h2 className="text-xl font-semibold text-gray-900 dark:text-white mb-2">质量维度数据建设中</h2>
      <p className="text-sm text-gray-500 dark:text-gray-400">
        代码质量维度的数据来源正在建设中，敬请期待。
      </p>
    </div>
  )
}
