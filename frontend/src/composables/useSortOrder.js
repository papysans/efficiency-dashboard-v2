import { ref, computed } from 'vue'
import { parseOrder } from '@/utils/sort.js'

/**
 * 列表排序 order 的状态 + 从 URL 回填的 composable（native 页用）。
 *
 * order 走 URL query.order（'field' 升 / '-field' 降 / 无 = 清除）。本 composable
 * 只负责 order 的响应式状态与解析，写 URL / watch 防回环由各页既有 updateUrl /
 * ignoreRouteWatch 机制负责（与 NeedViewV2 现有模式一致）。
 *
 * @param {import('vue-router').RouteLocationNormalizedLoaded} route
 * @returns {{ order: import('vue').Ref<string>, parsed: import('vue').ComputedRef<{field:string,desc:boolean}|null>, syncFromRoute: () => void }}
 */
export function useSortOrder(route) {
  const order = ref('')

  const parsed = computed(() => parseOrder(order.value))

  // 从当前 route.query.order 回填 ref（onMounted / route 变化时调用）。
  function syncFromRoute() {
    const o = route.query.order
    order.value = typeof o === 'string' ? o : ''
  }

  return { order, parsed, syncFromRoute }
}
