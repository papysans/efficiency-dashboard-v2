/** shimmer 骨架占位块，宽高用 className 控制 */
export function Skeleton({ className = '' }: { className?: string }) {
  return <div className={`skeleton ${className}`} />
}
