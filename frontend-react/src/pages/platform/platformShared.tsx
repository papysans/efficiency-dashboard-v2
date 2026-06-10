// 平台（chat-indicator-statistics 代理）两个子页的共享小件：子页 tab、token 缩写、错误码判断、
// 饼图色板、universal_id → 看板用户互链单元格。
// 仅平台页内部复用，不进 src/api/ 或全局组件（避免与并行任务冲突）。
import { Link, NavLink } from 'react-router'

/** 饼图/多系列色板（对齐 chat 侧 Apple 系配色，主色换看板 Apple Blue）。 */
export const PIE_COLORS = [
  '#0071e3',
  '#34c759',
  '#ff9500',
  '#ff3b30',
  '#af52de',
  '#5856d6',
  '#5ac8fa',
  '#ff2d55',
  '#8e8e93',
  '#ffd60a',
]

/** token 数缩写：1.2K / 3.4M / 1.05B。空/非数 => '-'。 */
export function shortToken(v: number | null | undefined): string {
  if (v == null || !Number.isFinite(Number(v))) return '-'
  const n = Number(v)
  if (n >= 1e9) return `${(n / 1e9).toFixed(2)}B`
  if (n >= 1e6) return `${(n / 1e6).toFixed(1)}M`
  if (n >= 1e3) return `${(n / 1e3).toFixed(1)}K`
  return String(n)
}

/** chat 侧错误判定口径：error_code 非空且非 '0' 才算错误（对齐其 Query.jsx isError）。 */
export function isErrorCode(code: string | null | undefined): boolean {
  return !!code && code !== '0'
}

/**
 * universal_id → 看板用户互链单元格。
 * chat.universal_id 与看板 user_id 同源（research/t6-universal-id-verify.md 实测结论），
 * 故复用 useUserNameMap 的 resolveName：命中（返回值 ≠ 原 id）→ 显示看板用户名并 Link 到 /user/:userId；
 * 解析不到 → 回退 chat 侧 username，再退截断 UUID（前 8 位）。
 * 映射加载失败/未就绪时 resolveName 原样返回 id，自然落入回退分支，不阻塞主数据渲染。
 */
export function ChatUserCell({
  universalId,
  chatUsername,
  resolveName,
}: {
  universalId: string | null | undefined
  chatUsername: string | null | undefined
  resolveName: (userId?: string) => string
}) {
  const uid = universalId || ''
  const resolved = uid ? resolveName(uid) : ''
  if (uid && resolved && resolved !== uid && resolved !== '-') {
    return (
      <Link
        to={`/user/${encodeURIComponent(uid)}`}
        onClick={(e) => e.stopPropagation()}
        className="text-apple-blue hover:text-apple-blue-hover no-underline focus:outline-none focus-visible:underline"
        title={`${resolved} · 查看看板用户详情`}
      >
        {resolved}
      </Link>
    )
  }
  const fallback = (chatUsername || '').trim() || (uid ? `${uid.slice(0, 8)}…` : '')
  return fallback ? <span title={uid || undefined}>{fallback}</span> : <span>-</span>
}

const TABS = [
  { to: '/platform/realtime', label: '实时态势', end: true },
  { to: '/platform/realtime/query', label: '明细查询', end: false },
]

/** 实时态势 / 明细查询 两子页切换 tab（玻璃药丸样式）。 */
export function PlatformTabs() {
  return (
    <nav className="glass rounded-xl p-1 inline-flex items-center gap-1" aria-label="平台子页切换">
      {TABS.map((t) => (
        <NavLink
          key={t.to}
          to={t.to}
          end={t.end}
          className={({ isActive }) =>
            `px-4 py-1.5 rounded-lg text-sm font-medium no-underline transition-colors ${
              isActive
                ? 'bg-apple-blue text-white'
                : 'text-gray-600 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white hover:bg-white/50 dark:hover:bg-white/10'
            }`
          }
        >
          {t.label}
        </NavLink>
      ))}
    </nav>
  )
}
