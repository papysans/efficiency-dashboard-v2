<template>
  <div class="kb-panel">
    <!-- 筛选区 -->
    <el-card class="kb-filter-card" shadow="never">
      <div class="kb-filter-row">
        <DimensionSelect
          v-model="filterUserId"
          dimension="user"
          :start-date="dateRange?.[0] || ''"
          :end-date="dateRange?.[1] || ''"
          placeholder="搜索用户..."
          style="width: 280px; margin-right: 16px"
          @change="handleQuery"
        />
        <el-date-picker
          v-model="dateRange"
          type="daterange"
          range-separator="至"
          start-placeholder="开始日期"
          end-placeholder="结束日期"
          value-format="YYYY-MM-DD"
          style="width: 300px"
          :clearable="false"
        />
        <el-button type="primary" :icon="Search" :loading="loading" @click="handleQuery" style="margin-left: 16px">
          查询
        </el-button>
        <el-switch
          v-model="showFavoritesOnly"
          active-text="仅显示收藏"
          style="margin-left: 16px"
          @change="handleFavoritesToggle"
        />
      </div>
    </el-card>

    <!-- 用户列表表格 -->
    <el-card shadow="never" class="kb-table-card">
        <el-table
          :data="users"
          stripe
          border
          v-loading="loading"
          style="width: 100%"
          :row-class-name="getRowClassName"
          @row-click="handleRowClick"
        >
          <el-table-column label="" width="50" align="center">
            <template #default="{ row }">
              <el-icon
                :size="18"
                :color="isFavorited(row) ? '#E6A23C' : '#C0C4CC'"
                style="cursor: pointer"
                @click.stop="toggleFavorite(row)"
              >
                <StarFilled v-if="isFavorited(row)" />
                <Star v-else />
              </el-icon>
            </template>
          </el-table-column>
          <el-table-column prop="key" label="用户名" min-width="200" show-overflow-tooltip>
            <template #default="{ row }">
              <el-tag v-if="row._isVirtualGroup" type="warning" size="small" style="margin-right: 6px">虚拟组</el-tag>
              <span>{{ row.key }}</span>
            </template>
          </el-table-column>
          <el-table-column prop="task_count" label="任务数" width="100" align="right" sortable />
          <el-table-column prop="api_count" label="API次数" width="100" align="right" sortable />
          <el-table-column prop="code_lines" label="代码行数" width="120" align="right" sortable />
          <el-table-column prop="api_cost" label="API费用" width="120" align="right" :formatter="fmtCost" sortable />
          <el-table-column prop="ai_estimated_days" label="AI预估人天" width="120" align="right" :formatter="fmtDays" sortable />
          <el-table-column prop="process_time" label="处理时长" width="120" align="right" :formatter="fmtMsToMin" sortable />
        </el-table>

        <div class="kb-pagination">
          <el-pagination
            v-model:current-page="page"
            v-model:page-size="pageSize"
            :page-sizes="[20, 50, 100, 200]"
            :total="total"
            layout="total, sizes, prev, pager, next, jumper"
            @size-change="handleSizeChange"
            @current-change="handlePageChange"
          />
        </div>
      </el-card>

      <!-- 图表区域 -->
      <div class="kb-charts-area">
        <el-row :gutter="12">
          <el-col :span="12" style="margin-bottom: 12px">
            <el-card shadow="never">
              <div ref="chartCodeLinesRef" class="kb-chart-container"></div>
            </el-card>
          </el-col>
          <el-col :span="12" style="margin-bottom: 12px">
            <el-card shadow="never">
              <div ref="chartApiCostRef" class="kb-chart-container"></div>
            </el-card>
          </el-col>
        </el-row>
        <el-row :gutter="12">
          <el-col :span="12" style="margin-bottom: 12px">
            <el-card shadow="never">
              <div ref="chartTokensRef" class="kb-chart-container"></div>
            </el-card>
          </el-col>
          <el-col :span="12" style="margin-bottom: 12px">
            <el-card shadow="never">
              <div ref="chartApiCountRef" class="kb-chart-container"></div>
            </el-card>
          </el-col>
        </el-row>
        <el-row :gutter="12">
          <el-col :span="24" style="margin-bottom: 12px">
            <el-card shadow="never">
              <div ref="chartProcessTimeRef" class="kb-chart-container"></div>
            </el-card>
          </el-col>
        </el-row>
      </div>
  </div>
</template>

<script setup>
import { ref, onMounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { Search } from '@element-plus/icons-vue'
import { Star, StarFilled } from '@element-plus/icons-vue'
import { getAggregate } from '@/api/es'
import { fmtCost, fmtDays, fmtMsToMin } from '@/utils/formatters'
import { getDefaultDateRange } from '@/utils/date'
import { createBarOption, createDualBarOption } from '@/utils/chart'
import { useFavorites } from '@/composables/useFavorites'
import { useUrlSync } from '@/composables/useUrlSync'
import { useChart } from '@/composables/useChart'
import DimensionSelect from '@/components/DimensionSelect.vue'

const router = useRouter()

// 筛选状态
const dateRange = ref([])
const loading = ref(false)
const users = ref([])
const filterUserId = ref('')

// 收藏
const { favorites, showFavoritesOnly, isFavorited, toggleFavorite: _toggleFavorite, loadFavorites, applyFavoritesFilter } = useFavorites('user')

async function toggleFavorite(row) {
  const needRefresh = await _toggleFavorite(row)
  if (needRefresh) {
    await handleFavoritesToggle(true)
  }
}

// 分页状态
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)

// 图表 DOM ref
const chartCodeLinesRef = ref(null)
const chartApiCostRef = ref(null)
const chartTokensRef = ref(null)
const chartApiCountRef = ref(null)
const chartProcessTimeRef = ref(null)

// 图表实例（使用 useChart composable）
const { setOption: setCodeLinesOption } = useChart(chartCodeLinesRef)
const { setOption: setApiCostOption } = useChart(chartApiCostRef)
const { setOption: setTokensOption } = useChart(chartTokensRef)
const { setOption: setApiCountOption } = useChart(chartApiCountRef)
const { setOption: setProcessTimeOption } = useChart(chartProcessTimeRef)

// URL 同步
const { syncToUrl, restoreFromUrl } = useUrlSync([
  { key: 'dateRange', ref: dateRange, type: 'dateRange' },
])

function updateCharts() {
  const data = users.value
  if (!data || data.length === 0) return

  const codeLinesData = data.map(d => ({ name: d.key, value: d.code_lines || 0 }))
  setCodeLinesOption(createBarOption('代码行数（按用户）', codeLinesData, '#409EFF', (v) => `${v} 行`))

  const apiCostData = data.map(d => ({ name: d.key, value: d.api_cost || 0 }))
  setApiCostOption(createBarOption('API 费用（按用户）', apiCostData, '#E6A23C', (v) => `${Number(v).toFixed(2)} 元`))

  const inTokensData = data.map(d => ({ name: d.key, value: d.api_in_tokens || 0 }))
  const outTokensData = data.map(d => ({ name: d.key, value: d.api_out_tokens || 0 }))
  setTokensOption(createDualBarOption('Token 使用量（按用户）', inTokensData, outTokensData, '输入 Tokens', '输出 Tokens', '#67C23A', '#F56C6C'))

  const apiCountData = data.map(d => ({ name: d.key, value: d.api_count || 0 }))
  setApiCountOption(createBarOption('API 调用次数（按用户）', apiCountData, '#909399', (v) => `${v} 次`))

  const processTimeData = data.map(d => ({ name: d.key, value: Math.round((d.process_time || 0) / 60000) }))
  setProcessTimeOption(createBarOption('处理时长（按用户，分钟）', processTimeData, '#409EFF', (v) => `${v} 分钟`))
}

function handleRowClick(row) {
  router.push({
    path: '/efficiency',
    query: {
      dimension: 'user',
      id: row.key,
      startDate: dateRange.value[0],
      endDate: dateRange.value[1]
    }
  })
}

// 行样式
function getRowClassName({ row }) {
  const cls = []
  if (row._isVirtualGroup) cls.push('kb-vgroup-row')
  cls.push('kb-clickable-row')
  return cls.join(' ')
}

// 收藏开关变化
async function handleFavoritesToggle(val) {
  if (val) {
    await loadFavorites()
    loading.value = true
    try {
      const dateParams = {
        startDate: dateRange.value[0].replace(/-/g, ''),
        endDate: dateRange.value[1].replace(/-/g, ''),
      }
      const result = await applyFavoritesFilter(dateParams)
      users.value = result.items
      total.value = result.total

      await nextTick()
      if (users.value.length > 0) {
        updateCharts()
      }
    } catch (err) {
      // 拦截器已处理错误提示
    } finally {
      loading.value = false
    }
  } else {
    fetchData()
  }
}

async function fetchData() {
  if (!dateRange.value || dateRange.value.length !== 2) return

  loading.value = true
  try {
    const params = {
      dimension: 'user',
      startDate: dateRange.value[0].replace(/-/g, ''),
      endDate: dateRange.value[1].replace(/-/g, ''),
      page: page.value,
      pageSize: pageSize.value
    }
    const result = await getAggregate(params)
    const data = result.data || result
    users.value = data.items || data.hits || []
    total.value = data.total || 0

    // 同步加载收藏列表（用于星标显示）
    if (favorites.value.length === 0) {
      loadFavorites()
    }

    await nextTick()
    if (users.value.length > 0) {
      updateCharts()
    }
  } catch (err) {
    console.error('查询数据失败:', err)
    users.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

function handleQuery() {
  page.value = 1
  syncToUrl()
  fetchData()
}

function handleSizeChange() {
  page.value = 1
  fetchData()
}

function handlePageChange() {
  fetchData()
}

onMounted(() => {
  restoreFromUrl()
  if (!dateRange.value || dateRange.value.length !== 2) {
    dateRange.value = getDefaultDateRange()
  }
  fetchData()
  loadFavorites()
})
</script>

