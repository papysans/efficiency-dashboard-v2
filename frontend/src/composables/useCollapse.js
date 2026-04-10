import { ref, computed, nextTick } from 'vue'

/**
 * 折叠状态管理 composable
 * @param {string} storageKey - localStorage 持久化 key
 * @param {Record<string, string>} sectionLabels - { sectionKey: '显示名称' }
 * @param {Function} [onExpand] - 展开某 section 后的回调 (key) => void
 */
export function useCollapse(storageKey, sectionLabels, onExpand) {
  const keys = Object.keys(sectionLabels)
  const initial = Object.fromEntries(keys.map(k => [k, false]))
  const collapsed = ref({ ...initial })

  function load() {
    try {
      const saved = localStorage.getItem(storageKey)
      if (saved) {
        const parsed = JSON.parse(saved)
        keys.forEach(k => {
          if (k in parsed) collapsed.value[k] = parsed[k]
        })
      }
    } catch {}
  }

  function save() {
    try { localStorage.setItem(storageKey, JSON.stringify(collapsed.value)) } catch {}
  }

  function toggle(key) {
    collapsed.value[key] = !collapsed.value[key]
    save()
    if (!collapsed.value[key]) {
      nextTick(() => nextTick(() => onExpand?.(key)))
    }
  }

  function expand(key) {
    if (collapsed.value[key]) toggle(key)
  }

  const collapsedTags = computed(() =>
    keys
      .filter(k => collapsed.value[k])
      .map(k => ({ key: k, label: sectionLabels[k] }))
  )

  return { collapsed, collapsedTags, toggle, expand, load }
}
