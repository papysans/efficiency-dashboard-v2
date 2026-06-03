<template>
  <div class="kb-panel">
    <KbFilterTable
      ref="filterTableRef"
      :columns="columns"
      :data="tableData"
      :loading="loading"
      :total="total"
      :order="order"
      v-model:page="page"
      v-model:pageSize="pageSize"
      row-class-name="kb-clickable-row"
      @row-click="handleRowClick"
      @size-change="handleSizeChange"
      @page-change="handlePageChange"
      @filter-change="handleFilterChange"
      @sort-change="onSortChange"
    >
      <template #cell-efficiency_ratio="{ row }">
        <el-tag
          v-if="row.efficiency_ratio != null"
          :type="row.efficiency_ratio >= 300 ? 'success' : row.efficiency_ratio >= 150 ? 'primary' : 'info'"
          size="small"
        >
          {{ row.efficiency_ratio.toFixed(1) }}%
        </el-tag>
        <el-tag v-else type="info" size="small">-</el-tag>
      </template>
    </KbFilterTable>
  </div>
</template>

<script setup>
import { ref, onMounted, nextTick, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import KbFilterTable from '@/components/KbFilterTable.vue'
import { getReposV2 } from '@/api/es'
import { formatDuration, formatLocalTime } from '@/utils/formatters'
import { getDefaultDateRangeWide } from '@/utils/date'
import { parseOrder } from '@/utils/sort'

const router = useRouter()
const route = useRoute()
const filterTableRef = ref(null)

// 列定义
const columns = [
  {
    prop: 'repo_addr',
    label: '仓库地址',
    minWidth: 300,
    showOverflowTooltip: true,
    filter: { type: 'text' },
  },
  {
    prop: 'repo_branch',
    label: '分支',
    minWidth: 100,
    filter: { type: 'multi-select' },
  },
  {
    prop: 'commit_count',
    label: 'Commit数',
    minWidth: 100,
    align: 'right',
    sortField: 'commitCount',
    filter: { type: 'number' },
  },
  {
    prop: 'task_count',
    label: 'Task数',
    minWidth: 100,
    align: 'right',
    sortField: 'taskCount',
    filter: { type: 'number' },
  },
  {
    prop: 'sum_ancient_minutes',
    label: '传统开发时长预估',
    minWidth: 120,
    align: 'right',
    // 单元格显示即 row.sum_ancient_minutes → 客户端按显示值排（与提效比/实际耗时口径统一，所见即所排）
    clientSort: true,
    sortValue: (row) => row.sum_ancient_minutes,
    formatter: (row, col, val) => formatDuration(val),
    filter: { type: 'number' },
  },
  {
    prop: 'sum_real_minutes',
    label: '实际耗时',
    minWidth: 120,
    align: 'right',
    clientSort: true,
    sortValue: (row) => row.sum_real_minutes,
    formatter: (row, col, val) => formatDuration(val),
    filter: { type: 'number' },
  },
  {
    prop: 'efficiency_ratio',
    label: '提效比',
    minWidth: 110,
    align: 'center',
    // 显示走封顶后的 efficiency_ratio，与后端裸 SQL 排序口径不同 → 客户端按显示值排
    clientSort: true,
    sortValue: (row) => row.efficiency_ratio,
    slotName: 'efficiency_ratio',
    filter: { type: 'number', shortcuts: [
      { label: '> 100%', value: { min: 100 } },
      { label: '> 200%', value: { min: 200 } },
      { label: '> 300%', value: { min: 300 } },
    ]},
  },
  {
    prop: 'start_time',
    label: '开始时间',
    minWidth: 150,
    sortField: 'startTime',
    formatter: (row, col, val) => formatLocalTime(val),
    filter: { type: 'date', serverSide: true },
  },
]

const loading = ref(false)
const tableData = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(250)
const order = ref(typeof route.query.order === 'string' ? route.query.order : undefined)

let serverDateRange = getDefaultDateRangeWide()
let _ignoreRouteWatch = false

function parseDateRange(startDate, endDate) {
  if (startDate && endDate) {
    const fmt = s => s.slice(0, 4) + '-' + s.slice(4, 6) + '-' + s.slice(6, 8)
    return [fmt(startDate), fmt(endDate)]
  }
  return getDefaultDateRangeWide()
}

function syncUrlToControls() {
  const { startDate, endDate, order: orderQuery } = route.query
  serverDateRange = parseDateRange(startDate, endDate)
  order.value = typeof orderQuery === 'string' ? orderQuery : undefined
  filterTableRef.value?.setFilter('start_time', serverDateRange)
}

function updateUrl() {
  const [start, end] = serverDateRange
  const query = {
    startDate: start.replace(/-/g, ''),
    endDate: end.replace(/-/g, ''),
  }
  if (order.value) query.order = order.value
  _ignoreRouteWatch = true
  router.replace({ query }).finally(() => { _ignoreRouteWatch = false })
}

// 仅当 order 命中服务端列（声明了 sortField）时才把 order 下发给后端。
function serverOrderParam() {
  const f = parseOrder(order.value)
  if (!f) return null
  const serverFields = columns.filter(c => c.sortField).map(c => c.sortField)
  return serverFields.includes(f.field) ? order.value : null
}

async function fetchData() {
  if (!serverDateRange || serverDateRange.length !== 2) return
  loading.value = true
  try {
    const serverOrder = serverOrderParam()
    const params = {
      startDate: serverDateRange[0].replace(/-/g, ''),
      endDate: serverDateRange[1].replace(/-/g, ''),
      page: page.value,
      pageSize: pageSize.value,
      ...(serverOrder ? { order: serverOrder } : {}),
    }
    const result = await getReposV2(params)
    const data = result.data || result
    tableData.value = data.data || []
    total.value = data.total || 0
  } catch {
    tableData.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

function handleRowClick(row) {
  router.push({ path: '/repo/' + encodeURIComponent(row.repo_addr) + '/' + encodeURIComponent(row.repo_branch) })
}

function handleFilterChange(allFilters) {
  const dateVal = allFilters.start_time
  if (dateVal && dateVal.length === 2) {
    serverDateRange = [dateVal[0], dateVal[1]]
  } else if (!dateVal) {
    serverDateRange = getDefaultDateRangeWide()
  }
  page.value = 1
  updateUrl()
  fetchData()
}

function onSortChange(payload) {
  order.value = payload.order
  updateUrl()
  // 仅服务端列需要重新取数；客户端列本地排序，无需请求。
  if (payload.server) {
    page.value = 1
    fetchData()
  }
}

function handleSizeChange() {
  page.value = 1
  fetchData()
}

function handlePageChange() {
  fetchData()
}

watch(() => route.query, async () => {
  if (_ignoreRouteWatch) return
  await nextTick()
  syncUrlToControls()
  await fetchData()
}, { deep: true })

onMounted(async () => {
  await nextTick()
  syncUrlToControls()
  await fetchData()
})
</script>
