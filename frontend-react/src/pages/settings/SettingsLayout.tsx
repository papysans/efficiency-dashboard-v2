// 设置区共用骨架：标题 + 四页 tab（价格/数据源/同步任务/系统配置），玻璃拟态。
// chat_stats_enabled=false（backend 未配 chat_stats.base_url）时不渲染子页内容，显示未启用提示；
// 各子页配合用 useChatEnabled() 作为 React Query 的 enabled，避免向 503 的代理发请求。
import type { ReactNode } from 'react'
import { NavLink } from 'react-router'
import { useGlobalConfig } from '@/api/queries'

const TABS = [
  { to: '/settings/pricing', label: '模型价格' },
  { to: '/settings/datasources', label: '数据源' },
  { to: '/settings/sync', label: '同步任务' },
  { to: '/settings/config', label: '系统配置' },
]

// 平台运维三级页（原顶部「平台」一级入口已撤；矩阵已把平台客观数据铺进业务页，
// 这些原始监控页保留为运维用途，归到设置下）。仅 chat_stats_enabled 时显示。
const PLATFORM_TABS = [
  { to: '/settings/platform/overview', label: '平台总览' },
  { to: '/settings/platform/health', label: 'AI 服务健康度' },
  { to: '/settings/platform/realtime', label: '实时态势' },
  { to: '/settings/platform/realtime/query', label: '明细查询' },
]

// ---- 共用样式常量（对齐 ProjectList 等现有页面的玻璃拟态惯例） ----
export const INPUT_CLS =
  'glass rounded-lg px-3 py-1.5 text-sm w-full bg-transparent text-gray-900 dark:text-white ' +
  'placeholder-gray-400 focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue'
export const BTN_PRIMARY =
  'bg-apple-blue hover:bg-apple-blue-hover text-white rounded-lg px-4 py-1.5 text-sm font-medium cursor-pointer ' +
  'transition-colors disabled:opacity-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue'
export const BTN_GLASS =
  'glass rounded-lg px-4 py-1.5 text-sm text-gray-700 dark:text-gray-200 cursor-pointer hover:text-apple-blue ' +
  'transition-colors disabled:opacity-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue'
export const BTN_DANGER =
  'bg-rose-500 hover:bg-rose-600 text-white rounded-lg px-4 py-1.5 text-sm font-medium cursor-pointer ' +
  'transition-colors disabled:opacity-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-rose-400'
export const LINK_BTN =
  'text-apple-blue hover:underline cursor-pointer bg-transparent border-none p-0 text-sm transition-colors ' +
  'focus:outline-none focus-visible:underline'
export const LINK_BTN_DANGER =
  'text-rose-500 hover:text-rose-600 cursor-pointer bg-transparent border-none p-0 text-sm transition-colors ' +
  'focus:outline-none focus-visible:underline'

export const TH = 'px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
export const TH_NUM = 'px-3 py-2 text-right font-semibold text-gray-500 dark:text-gray-400 whitespace-nowrap'
export const TD = 'px-3 py-2 align-middle text-gray-700 dark:text-gray-200'
export const TD_NUM = 'px-3 py-2 align-middle text-right tabular-nums text-gray-700 dark:text-gray-200'

/** 表单字段（label + 控件 + 可选说明），对齐 ProjectList 的 Field 写法。 */
export function Field({ label, hint, children }: { label: string; hint?: string; children: ReactNode }) {
  return (
    <label className="block">
      <span className="block text-xs text-gray-500 dark:text-gray-400 mb-1">{label}</span>
      {children}
      {hint && <span className="block text-xs text-gray-400 dark:text-gray-500 mt-1">{hint}</span>}
    </label>
  )
}

/** chat 代理是否启用（/v2/config 的 chat_stats_enabled）。未加载完成时返回 false（查询保持 idle）。 */
export function useChatEnabled(): boolean {
  const { data } = useGlobalConfig()
  return data?.chat_stats_enabled === true
}

/** chat 代理未启用时的整页提示（设置区与平台两子页共用，保持三处开关语义一致）。 */
export function ChatDisabledNotice() {
  return (
    <div className="glass rounded-2xl p-10 text-center">
      <h2 className="text-xl font-semibold text-gray-900 dark:text-white mb-2">平台指标服务未启用</h2>
      <p className="text-sm text-gray-500 dark:text-gray-400">
        后端未配置 chat_stats.base_url，无法访问 chat-indicator-statistics 服务。请在 backend 配置中填写后重启。
      </p>
    </div>
  )
}

function SettingsTab({ to, label, end }: { to: string; label: string; end?: boolean }) {
  return (
    <NavLink
      to={to}
      end={end}
      className={({ isActive }) =>
        `px-3 py-1.5 rounded-lg text-sm font-medium transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-apple-blue ${
          isActive
            ? 'bg-apple-blue text-white'
            : 'glass text-gray-600 dark:text-gray-300 hover:text-apple-blue'
        }`
      }
    >
      {label}
    </NavLink>
  )
}

export default function SettingsLayout({ children }: { children: ReactNode }) {
  const { data: gc } = useGlobalConfig()
  const disabled = !!gc && gc.chat_stats_enabled !== true
  const chatEnabled = gc?.chat_stats_enabled === true

  return (
    <div className="space-y-5">
      <header className="space-y-3">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">设置</h1>
          <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
            平台客观指标（chat-indicator-statistics）管理：模型价格、源数据源、ETL 同步任务与系统配置。
          </p>
        </div>
        <nav aria-label="设置子页" className="flex flex-wrap items-center gap-2">
          {TABS.map((t) => (
            <SettingsTab key={t.to} to={t.to} label={t.label} />
          ))}

          {/* 平台运维分组（顶部「平台」一级入口已撤，归到设置下）。仅 chat_stats_enabled 时显示。 */}
          {chatEnabled && (
            <>
              <span
                className="mx-1 self-stretch w-px bg-gray-200 dark:bg-white/10"
                aria-hidden="true"
              />
              <span className="text-xs font-semibold text-gray-400 dark:text-gray-500 px-1">
                平台运维
              </span>
              {PLATFORM_TABS.map((t) => (
                // 实时态势 /realtime 与明细查询 /realtime/query 路径前缀重叠 → /realtime 用 end 精确匹配。
                <SettingsTab
                  key={t.to}
                  to={t.to}
                  label={t.label}
                  end={t.to === '/settings/platform/realtime'}
                />
              ))}
            </>
          )}
        </nav>
      </header>

      {disabled ? <ChatDisabledNotice /> : children}
    </div>
  )
}
