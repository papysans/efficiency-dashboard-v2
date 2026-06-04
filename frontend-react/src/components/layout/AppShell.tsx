import { Outlet, NavLink, Link } from 'react-router'
import { useTheme } from '@/hooks/useTheme'

const navItems = [
  { to: '/', label: '总览', end: true },
  { to: '/needs-v2', label: '需求' },
  { to: '/task-v2', label: '任务' },
  { to: '/user-v2', label: '用户' },
  { to: '/repo-v2', label: '仓库' },
  { to: '/org-v2', label: '组织' },
  { to: '/project-v2', label: '项目' },
  { to: '/commit-v2', label: '提交' },
]

export default function AppShell() {
  const { theme, toggle } = useTheme()

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
            效能看板
          </Link>
          <div className="hidden md:flex items-center gap-1">
            {navItems.map(({ to, label, end }) => (
              <NavLink
                key={to}
                to={to}
                end={end}
                className={({ isActive }) =>
                  `px-3 py-1.5 rounded-lg text-sm font-medium no-underline transition-colors ${
                    isActive
                      ? 'bg-apple-blue text-white'
                      : 'text-gray-600 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white hover:bg-white/50 dark:hover:bg-white/10'
                  }`
                }
              >
                {label}
              </NavLink>
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

      <main className="max-w-[1920px] mx-auto px-4 sm:px-6 py-8">
        <Outlet />
      </main>
    </div>
  )
}
