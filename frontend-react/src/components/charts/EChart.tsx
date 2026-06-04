import type { CSSProperties } from 'react'
import type { EChartsOption } from 'echarts'
import { useEChart } from '@/hooks/useEChart'

interface EChartProps {
  option: EChartsOption
  height?: number | string
  className?: string
}

/** 玻璃拟态环境下的 ECharts 容器：传入 option 即渲染，自动管理实例与 resize。 */
export function EChart({ option, height = 280, className = '' }: EChartProps) {
  const ref = useEChart(option)
  const style: CSSProperties = { height: typeof height === 'number' ? `${height}px` : height, width: '100%' }
  return <div ref={ref} className={className} style={style} />
}
