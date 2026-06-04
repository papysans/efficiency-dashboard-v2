import { useEffect, useRef, useState } from 'react'

/** 是否启用动效（尊重 prefers-reduced-motion） */
function motionAllowed(): boolean {
  if (typeof window === 'undefined' || !window.matchMedia) return true
  return !window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

/**
 * 数字滚动动效：从 0 缓动到 target（requestAnimationFrame，默认 1.2s ease-out）。
 * prefers-reduced-motion: reduce 时直接返回终值、不滚动（无障碍要求）。
 */
export function useCountUp(target: number, duration = 1200): number {
  const [value, setValue] = useState(() => (motionAllowed() ? 0 : target))
  const rafRef = useRef<number | null>(null)

  useEffect(() => {
    if (!motionAllowed() || !Number.isFinite(target)) {
      setValue(Number.isFinite(target) ? target : 0)
      return
    }
    const from = 0
    const start = performance.now()
    const tick = (now: number) => {
      const t = Math.min(1, (now - start) / duration)
      const eased = 1 - Math.pow(1 - t, 3) // ease-out cubic
      setValue(from + (target - from) * eased)
      if (t < 1) {
        rafRef.current = requestAnimationFrame(tick)
      } else {
        setValue(target)
      }
    }
    rafRef.current = requestAnimationFrame(tick)
    return () => {
      if (rafRef.current != null) cancelAnimationFrame(rafRef.current)
    }
  }, [target, duration])

  return value
}
