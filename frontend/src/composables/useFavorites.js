import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { getFavorites, addFavorite, removeFavorite, getVirtualGroupAggregate, getAggregate } from '@/api/es'

/**
 * 收藏逻辑 Composable
 * @param {string|Ref<string>|() => string} dimension - 维度标识，支持响应式
 */
export function useFavorites(dimension) {
  const favorites = ref([])
  const showFavoritesOnly = ref(false)

  // 获取当前 dimension 值的辅助函数
  function getDimension() {
    // 支持: 字符串、ref、getter 函数
    if (typeof dimension === 'function') return dimension()
    if (dimension && typeof dimension === 'object' && 'value' in dimension) return dimension.value
    return dimension
  }

  function isFavorited(row) {
    return favorites.value.some(f => f.item_key === row.key)
  }

  async function toggleFavorite(row) {
    const fav = favorites.value.find(f => f.item_key === row.key)
    if (fav) {
      try {
        await removeFavorite(fav.id)
        favorites.value = favorites.value.filter(f => f.id !== fav.id)
        ElMessage.success('已取消收藏')
        if (showFavoritesOnly.value) {
          // 调用者需要自行处理收藏过滤刷新
        }
      } catch (err) {
        console.error('取消收藏失败:', err)
        ElMessage.error('取消收藏失败')
      }
    } else {
      try {
        const dim = getDimension()
        const res = await addFavorite({
          dimension: dim,
          item_key: row.key,
          display_name: row.key,
        })
        const newFav = res.data || res
        favorites.value.push(newFav)
        ElMessage.success('已收藏')
      } catch (err) {
        console.error('添加收藏失败:', err)
        ElMessage.error('添加收藏失败')
      }
    }
    return showFavoritesOnly.value  // 返回是否需要刷新收藏过滤
  }

  async function loadFavorites() {
    try {
      const dim = getDimension()
      const res = await getFavorites({ dimension: dim })
      const data = res.data || res
      favorites.value = data.items || data || []
    } catch (err) {
      console.error('加载收藏失败:', err)
      favorites.value = []
    }
  }

  /**
   * 应用收藏过滤
   * @param {Object} dateParams - { startDate: 'YYYYMMDD', endDate: 'YYYYMMDD' }
   * @param {Object} [extraParams] - 额外的查询参数（如 OrgPanel 的 drillFilter）
   * @returns {Promise<{items: Array, total: number}>} 过滤后的数据
   */
  async function applyFavoritesFilter(dateParams, extraParams = {}) {
    const dim = getDimension()
    // 获取全部聚合数据
    const params = { dimension: dim, ...dateParams, ...extraParams, page: 1, pageSize: 9999 }
    const result = await getAggregate(params)
    const data = result.data || result
    const allItems = data.items || data.hits || []

    // 收藏的 key 集合（排除虚拟组）
    const favKeys = new Set(favorites.value.filter(f => !f.virtual_group_id).map(f => f.item_key))
    const filtered = allItems.filter(item => favKeys.has(item.key))

    // 虚拟组收藏项
    const vgFavs = favorites.value.filter(f => f.virtual_group_id)
    const vgPromises = vgFavs.map(async (f) => {
      try {
        const res = await getVirtualGroupAggregate(f.virtual_group_id, dateParams)
        const vgData = res.data || res
        return { ...vgData, key: f.item_key, _isVirtualGroup: true }
      } catch {
        return { key: f.item_key, _isVirtualGroup: true }
      }
    })
    const vgItems = await Promise.all(vgPromises)

    const items = [...vgItems, ...filtered]
    return { items, total: items.length }
  }

  return {
    favorites,
    showFavoritesOnly,
    isFavorited,
    toggleFavorite,
    loadFavorites,
    applyFavoritesFilter,
  }
}
