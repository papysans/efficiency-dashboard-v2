import { useRoute, useRouter } from 'vue-router'

/**
 * URL 同步 Composable
 * @param {Array<{key: string, ref: Ref, type: 'string'|'dateRange'|'enum', enums?: string[]}>} paramDefs
 *   - key: URL query 参数名
 *   - ref: Vue ref 对象
 *   - type: 'dateRange' 表示 dateRange ref 需要拆分为 startDate/endDate 两个 query 参数
 *           'string' 表示直接映射
 *           'enum' 表示只接受 enums 列表中的值
 */
export function useUrlSync(paramDefs) {
  const route = useRoute()
  const router = useRouter()

  /**
   * 将当前参数同步到 URL query
   */
  function syncToUrl() {
    const query = { ...route.query }
    for (const def of paramDefs) {
      if (def.type === 'dateRange') {
        const val = def.ref.value
        if (val && val.length === 2) {
          query.startDate = val[0]
          query.endDate = val[1]
        }
      } else {
        // string 或 enum
        if (def.ref.value != null && def.ref.value !== '') {
          query[def.key] = def.ref.value
        }
      }
    }
    router.replace({ query })
  }

  /**
   * 从 URL query 恢复参数
   * @returns {boolean} 是否恢复了任何参数
   */
  function restoreFromUrl() {
    const q = route.query
    let restored = false
    for (const def of paramDefs) {
      if (def.type === 'dateRange') {
        if (q.startDate && q.endDate) {
          def.ref.value = [q.startDate, q.endDate]
          restored = true
        }
      } else if (def.type === 'enum') {
        if (q[def.key] && def.enums && def.enums.includes(q[def.key])) {
          def.ref.value = q[def.key]
          restored = true
        }
      } else {
        // string
        if (q[def.key]) {
          def.ref.value = q[def.key]
          restored = true
        }
      }
    }
    return restored
  }

  return { syncToUrl, restoreFromUrl }
}
