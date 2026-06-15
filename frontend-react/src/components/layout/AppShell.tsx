import { Outlet, NavLink, Link, useLocation } from 'react-router'
import { useGlobalConfig } from '@/api/queries'
import { useTheme } from '@/hooks/useTheme'

// 两段式导航：「效能」= 现有 8 项；「平台」= chat-indicator-statistics 客观指标（平台指标/设置）。
// 「平台」组仅在 /v2/config 的 chat_stats_enabled === true 时渲染（false/未配置/请求中一律隐藏）。
// match = 高亮前缀：平台指标/设置各有多个子页（/platform/*、/settings/*），单入口按前缀保持高亮。
const navGroups = [
  {
    group: '效能',
    items: [
      { to: '/', label: '总览', end: true },
      { to: '/distribution-v2', label: '分布' },
      { to: '/needs-v2', label: '需求' },
      // 「任务」(/task-v2)、「仓库」(/repo-v2) 暂从导航隐藏：数据缺失、单独入口意义不大。
      // 路由仍保留（其他页面有互链跳转到任务/仓库详情），后续数据补齐后可恢复。
      { to: '/user-v2', label: '用户' },
      { to: '/org-tree-v2', label: '组织' },
      { to: '/project-v2', label: '项目' },
      { to: '/commit-v2', label: '提交' },
    ],
  },
  {
    group: '平台',
    items: [
      { to: '/platform/overview', label: '平台指标', match: '/platform' },
      { to: '/settings/pricing', label: '设置', match: '/settings' },
    ],
  },
] as const

export default function AppShell() {
  const { theme, toggle } = useTheme()
  const { pathname } = useLocation()
  const { data: globalConfig } = useGlobalConfig()
  const dashboardTitle = `${globalConfig?.dashboard_title_prefix ?? ''}效能看板`
  const chatStatsEnabled = globalConfig?.chat_stats_enabled === true
  const visibleGroups = navGroups.filter((g) => g.group !== '平台' || chatStatsEnabled)

  return (
    <div className="relative min-h-screen">
      {/* 背景渐变光球 */}
      <div className="bg-orb bg-orb-purple" />
      <div className="bg-orb bg-orb-blue" />
      <div className="bg-orb bg-orb-pink" />

      {/* 玻璃导航，贴顶 */}
      <nav className="glass sticky top-0 z-50 px-6 py-3 flex items-center justify-between rounded-none border-x-0 border-t-0">
        <div className="flex items-center gap-8">
          <Link to="/" className="text-xl font-bold text-gray-900 dark:text-white no-underline tracking-tight">
            {dashboardTitle}
          </Link>
          <div className="hidden md:flex items-center gap-1">
            {visibleGroups.map((group, gi) => (
              <div key={group.group} className="flex items-center gap-1">
                {gi > 0 && <span className="mx-2 h-4 w-px bg-gray-300 dark:bg-white/20" aria-hidden="true" />}
                {/* 单组时不显示组标签（chat 未启用时导航与原先完全一致） */}
                {visibleGroups.length > 1 && (
                  <span className="mr-1 text-[11px] tracking-wider text-gray-400 dark:text-gray-500 select-none">
                    {group.group}
                  </span>
                )}
                {group.items.map((item) => (
                  <NavLink
                    key={item.to}
                    to={item.to}
                    end={'end' in item ? item.end : undefined}
                    className={({ isActive }) =>
                      `px-3 py-1.5 rounded-lg text-sm font-medium no-underline transition-colors ${
                        isActive || ('match' in item && pathname.startsWith(item.match))
                          ? 'bg-apple-blue text-white'
                          : 'text-gray-600 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white hover:bg-white/50 dark:hover:bg-white/10'
                      }`
                    }
                  >
                    {item.label}
                  </NavLink>
                ))}
              </div>
            ))}
          </div>
        </div>

        <button
          onClick={toggle}
          className="px-2.5 py-1.5 rounded-lg text-sm text-gray-600 dark:text-gray-300 hover:bg-white/50 dark:hover:bg-white/10 transition-colors border-none cursor-pointer bg-transparent"
          aria-label="切换主题"
        >
          {theme === 'light' ? (
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z" />
            </svg>
          ) : (
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" />
            </svg>
          )}
        </button>
      </nav>

      <main className="max-w-[90%] mx-auto px-4 sm:px-6 py-8">
        <Outlet />
      </main>
    </div>
  )
}
