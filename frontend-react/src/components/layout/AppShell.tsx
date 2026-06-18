import { useState } from 'react'
import { Outlet, NavLink, Link, useLocation } from 'react-router'
import { useGlobalConfig } from '@/api/queries'
import { useTheme } from '@/hooks/useTheme'
import { useViewState } from '@/store/viewState'
import { DateRangePicker } from '@/components/ui/DateRangePicker'

// 主体×维度矩阵 IA（A 主体优先）：一级导航选「谁」，进下钻后页内一排维度 Tab 选「看什么」。
// 一级 6 项：总览 / 组织 / 个人 / 项目 / 仓库 / 需求。平台/设置进右侧工具区（齿轮 + 条件平台）。
// 时间范围提升为全局（绑定 viewState store），放右侧工具区，切维度/切主体保持不变。
// 高亮：NavLink isActive + 前缀匹配（match，如 /org/* 整段高亮，进 /org/efficiency 仍高亮「组织」）。
interface NavItem {
  to: string
  label: string
  /** 精确匹配（仅总览 /）。 */
  end?: boolean
  /** 前缀匹配段（如 /org → /org/efficiency 仍高亮）。 */
  match?: string
}

const navItems: NavItem[] = [
  { to: '/', label: '总览', end: true },
  { to: '/org', label: '组织', match: '/org' },
  { to: '/user', label: '个人', match: '/user' },
  { to: '/project', label: '项目', match: '/project' },
  { to: '/repo', label: '仓库', match: '/repo' },
  { to: '/needs-v2', label: '需求', match: '/needs' },
]

function navLinkClass(isActive: boolean) {
  return `px-3 py-1.5 rounded-lg text-sm font-medium no-underline transition-colors ${
    isActive
      ? 'bg-apple-blue text-white'
      : 'text-gray-600 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white hover:bg-white/50 dark:hover:bg-white/10'
  }`
}

export default function AppShell() {
  const { theme, toggle } = useTheme()
  const { pathname } = useLocation()
  const { data: globalConfig } = useGlobalConfig()
  const { timeRange, setTimeRange } = useViewState()
  const [drawerOpen, setDrawerOpen] = useState(false)

  const dashboardTitle = `${globalConfig?.dashboard_title_prefix ?? ''}效能看板`

  // 前缀匹配高亮：/org/efficiency 落到 /org 段。需求页（/needs-v2、/needs/:id）按 /needs 前缀高亮。
  const isItemActive = (item: NavItem) => {
    if (item.end) return pathname === item.to
    if (pathname === item.to) return true
    return item.match != null && (pathname === item.match || pathname.startsWith(`${item.match}/`))
  }

  return (
    <div className="relative min-h-screen">
      {/* 背景渐变光球 */}
      <div className="bg-orb bg-orb-purple" />
      <div className="bg-orb bg-orb-blue" />
      <div className="bg-orb bg-orb-pink" />

      {/* 玻璃导航，贴顶 */}
      <nav className="glass sticky top-0 z-50 px-6 py-3 flex items-center justify-between rounded-none border-x-0 border-t-0">
        <div className="flex items-center gap-8">
          {/* 移动端汉堡 */}
          <button
            onClick={() => setDrawerOpen(true)}
            className="md:hidden px-2 py-1.5 rounded-lg text-gray-600 dark:text-gray-300 hover:bg-white/50 dark:hover:bg-white/10 transition-colors border-none cursor-pointer bg-transparent"
            aria-label="打开导航菜单"
          >
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h16" />
            </svg>
          </button>

          <Link to="/" className="text-xl font-bold text-gray-900 dark:text-white no-underline tracking-tight">
            {dashboardTitle}
          </Link>

          {/* 桌面一级导航 */}
          <div className="hidden md:flex items-center gap-1">
            {navItems.map((item) => (
              <NavLink key={item.to} to={item.to} className={() => navLinkClass(isItemActive(item))}>
                {item.label}
              </NavLink>
            ))}
          </div>
        </div>

        {/* 右侧工具区：全局时间范围 + 设置 + 主题。
            注：原「平台」一级入口已撤——平台客观数据已按主体×维度铺进业务页，
            原始平台监控页下沉到「设置 › 平台运维」三级页（条件 chat_stats_enabled）。 */}
        <div className="flex items-center gap-2">
          <div className="hidden sm:block">
            <DateRangePicker value={timeRange} onChange={setTimeRange} />
          </div>

          <NavLink
            to="/settings/pricing"
            className={() =>
              `px-2.5 py-1.5 rounded-lg transition-colors border-none cursor-pointer bg-transparent ${
                pathname.startsWith('/settings')
                  ? 'text-apple-blue'
                  : 'text-gray-600 dark:text-gray-300 hover:bg-white/50 dark:hover:bg-white/10'
              } no-underline`
            }
            aria-label="设置"
            title="设置"
          >
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
            </svg>
          </NavLink>

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
        </div>
      </nav>

      {/* 移动端抽屉 */}
      {drawerOpen && (
        <div className="md:hidden fixed inset-0 z-[60]" role="dialog" aria-modal="true" aria-label="导航菜单">
          <div
            className="absolute inset-0 bg-black/30 backdrop-blur-sm"
            onClick={() => setDrawerOpen(false)}
            aria-hidden="true"
          />
          <div className="glass absolute left-0 top-0 h-full w-64 p-5 flex flex-col gap-2 rounded-none border-y-0 border-l-0">
            <div className="flex items-center justify-between mb-2">
              <span className="text-lg font-bold text-gray-900 dark:text-white">{dashboardTitle}</span>
              <button
                onClick={() => setDrawerOpen(false)}
                className="px-2 py-1 rounded-lg text-gray-500 dark:text-gray-400 hover:bg-white/50 dark:hover:bg-white/10 transition-colors border-none cursor-pointer bg-transparent"
                aria-label="关闭导航菜单"
              >
                <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>

            <div className="sm:hidden mb-2">
              <DateRangePicker value={timeRange} onChange={setTimeRange} />
            </div>

            {navItems.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                onClick={() => setDrawerOpen(false)}
                className={() => navLinkClass(isItemActive(item))}
              >
                {item.label}
              </NavLink>
            ))}

            <span className="mt-2 h-px bg-gray-200 dark:bg-white/10" aria-hidden="true" />

            {/* 平台运维已下沉到「设置 › 平台运维」三级页，不再单列一级入口。 */}
            <NavLink
              to="/settings/pricing"
              onClick={() => setDrawerOpen(false)}
              className={() => navLinkClass(pathname.startsWith('/settings'))}
            >
              设置
            </NavLink>
          </div>
        </div>
      )}

      <main className="max-w-[90%] mx-auto px-4 sm:px-6 py-8">
        <Outlet />
      </main>
    </div>
  )
}
