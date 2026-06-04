import type { ReactNode, HTMLAttributes } from 'react'

interface GlassProps extends HTMLAttributes<HTMLDivElement> {
  children: ReactNode
}

/** 玻璃拟态容器：.glass + 大圆角，可叠加 className */
export function Glass({ children, className = '', ...rest }: GlassProps) {
  return (
    <div className={`glass rounded-2xl ${className}`} {...rest}>
      {children}
    </div>
  )
}
