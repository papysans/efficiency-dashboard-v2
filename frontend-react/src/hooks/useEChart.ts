import { useEffect, useRef } from 'react'
import * as echarts from 'echarts'
import type { EChartsOption, EChartsType } from 'echarts'

/**
 * ECharts 生命周期管理（移植自 Vue composables/useChart.js 思路，改为 React ref + useEffect）：
 * init / setOption / dispose + window resize 监听。
 * option 变化时 setOption(notMerge)；theme 变化通过传入不同 option 触发重设。
 */
export function useEChart(option: EChartsOption) {
  const containerRef = useRef<HTMLDivElement | null>(null)
  const instanceRef = useRef<EChartsType | null>(null)

  // init / dispose（仅挂载卸载）
  useEffect(() => {
    if (!containerRef.current) return
    const chart = echarts.init(containerRef.current)
    instanceRef.current = chart
    const onResize = () => chart.resize()
    window.addEventListener('resize', onResize)
    return () => {
      window.removeEventListener('resize', onResize)
      chart.dispose()
      instanceRef.current = null
    }
  }, [])

  // option 更新（notMerge 全量替换，亮暗切换/数据变化都覆盖干净）
  useEffect(() => {
    instanceRef.current?.setOption(option, true)
  }, [option])

  return containerRef
}
