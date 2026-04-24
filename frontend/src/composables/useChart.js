import { onMounted, onUnmounted, shallowRef } from 'vue'
import * as echarts from 'echarts'

// 全局 resize 管理：所有 useChart 实例共享一个 resize 监听器
const chartInstances = new Set()
let resizeListenerRegistered = false

function handleGlobalResize() {
  chartInstances.forEach(instance => {
    instance?.resize()
  })
}

/**
 * ECharts 生命周期管理 Composable
 * @param {Ref<HTMLElement>} containerRef - 图表容器 DOM ref
 * @returns {{ chart: ShallowRef, setOption: Function, dispose: Function }}
 */
export function useChart(containerRef) {
  const chart = shallowRef(null)

  function init() {
    if (containerRef.value && !chart.value) {
      chart.value = echarts.init(containerRef.value)
      chartInstances.add(chart.value)
    }
  }

  function setOption(option) {
    if (containerRef.value) {
      // v-if 重建后 DOM 节点已换，旧实例需要先销毁再重建
      if (chart.value && chart.value.getDom() !== containerRef.value) {
        dispose()
      }
      if (!chart.value) {
        init()
      }
    }
    chart.value?.setOption(option)
  }

  function dispose() {
    if (chart.value) {
      chartInstances.delete(chart.value)
      chart.value.dispose()
      chart.value = null
    }
  }

  onMounted(() => {
    // 注册全局 resize 监听（只注册一次）
    if (!resizeListenerRegistered) {
      window.addEventListener('resize', handleGlobalResize)
      resizeListenerRegistered = true
    }
  })

  onUnmounted(() => {
    dispose()
    // 当所有图表实例都销毁后，移除全局 resize 监听
    if (chartInstances.size === 0 && resizeListenerRegistered) {
      window.removeEventListener('resize', handleGlobalResize)
      resizeListenerRegistered = false
    }
  })

  return { chart, setOption, dispose }
}
