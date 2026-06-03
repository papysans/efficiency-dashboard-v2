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
      @size-change="handleSizeChange"
      @page-change="handlePageChange"
      @filter-change="handleFilterChange"
      @sort-change="onSortChange"
    >
      <template #cell-commit_id="{ row }">
        <el-link type="primary" @click.stop="router.push('/commit/' + row.commit_id)">{{ (row.commit_id || '').substring(0, 8) }}</el-link>
      </template>
      <template #cell-org_display="{ row }">
        <el-link v-if="row.org_display" type="primary" @click.stop="goToOrg(row)">{{ row.org_display }}</el-link>
        <span v-else>-</span>
      </template>
      <template #cell-user_name="{ row }">
        <el-link type="primary" @click.stop="goToUser(row.user_id, row.user_name)">{{ row.user_name || '-' }}</el-link>
      </template>
      <template #cell-repo_addr="{ row }">
        <el-tooltip v-if="row.repo_addr" :content="row.repo_addr + '/' + (row.repo_branch || '-')" placement="top" :show-after="400">
          <el-link type="primary" class="repo-link-rtl" @click.stop="handleRepoClick(row)">
            {{ row.repo_addr }}/{{ row.repo_branch || '-' }}
          </el-link>
        </el-tooltip>
        <span v-else>-</span>
      </template>
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
import { getCommitsV2 } from '@/api/es'
import { fmtCost, formatDuration, formatLocalTime } from '@/utils/formatters'
import { getDefaultDateRangeWide } from '@/utils/date'
import { getEffectiveAncient, getEffectiveReal } from '@/utils/commit-helpers'
import { parseOrder } from '@/utils/sort'

const router = useRouter()
const route = useRoute()
const filterTableRef = ref(null)

function parseDateRange(startDate, endDate) {
  if (startDate && endDate) {
    const fmt = s => s.slice(0, 4) + '-' + s.slice(4, 6) + '-' + s.slice(6, 8)
    return [fmt(startDate), fmt(endDate)]
  }
  return getDefaultDateRangeWide()
}

function syncUrlToControls() {
  const { startDate, endDate, userName, org1, org2, org3, org4, order: orderQuery } = route.query
  serverDateRange = parseDateRange(startDate, endDate)
  order.value = typeof orderQuery === 'string' ? orderQuery : undefined
  filterTableRef.value?.setFilter('commit_time', serverDateRange)
  filterTableRef.value?.setFilter('user_name', userName ? String(userName).trim() : '')
  const orgVal = { org1: org1 || '', org2: org2 || '', org3: org3 || '', org4: org4 || '' }
  if (orgVal.org1 || orgVal.org2 || orgVal.org3 || orgVal.org4) {
    filterTableRef.value?.setFilter('org_display', orgVal)
  } else {
    filterTableRef.value?.setFilter('org_display', null)
  }
}

let _ignoreRouteWatch = false

function updateUrl() {
  const [start, end] = serverDateRange
  const currentUserName = filterTableRef.value?.getFilter('user_name') || ''
  const orgFilter = filterTableRef.value?.getFilter('org_display') || {}
  const query = {
    startDate: start.replace(/-/g, ''),
    endDate: end.replace(/-/g, ''),
  }
  if (currentUserName) query.userName = currentUserName
  if (orgFilter.org1) query.org1 = orgFilter.org1
  if (orgFilter.org2) query.org2 = orgFilter.org2
  if (orgFilter.org3) query.org3 = orgFilter.org3
  if (orgFilter.org4) query.org4 = orgFilter.org4
  if (order.value) query.order = order.value
  _ignoreRouteWatch = true
  router.replace({ query }).finally(() => { _ignoreRouteWatch = false })
}

// 列定义（按需求顺序）
const columns = [
  {
    prop: 'commit_id',
    label: 'Commit ID',
    minWidth: 88,
    showOverflowTooltip: true,
    slotName: 'commit_id',
  },
  {
    prop: 'commit_time',
    label: '时间',
    minWidth: 170,
    sortField: 'commitTime',
    formatter: (row, col, val) => formatLocalTime(val),
    filter: { type: 'date', serverSide: true },
  },
  {
    prop: 'org_display',
    label: '组织',
    minWidth: 150,
    showOverflowTooltip: true,
    slotName: 'org_display',
    filter: { type: 'cascade-org' },
  },
  {
    prop: 'user_name',
    label: '用户',
    minWidth: 100,
    slotName: 'user_name',
    filter: { type: 'multi-select' },
  },
  {
    prop: 'comment',
    label: '说明',
    minWidth: 120,
    showOverflowTooltip: true,
    filter: { type: 'text' },
  },
  {
    prop: 'repo_addr',
    label: '仓库',
    minWidth: 120,
    showOverflowTooltip: true,
    slotName: 'repo_addr',
    filter: { type: 'multi-select' },
  },
  {
    prop: 'diff_lines',
    label: '代码量',
    minWidth: 80,
    align: 'right',
    sortField: 'diffLines',
    filter: { type: 'number', shortcuts: [
      { label: '> 0', value: { min: 1 } },
      { label: '> 50', value: { min: 50 } },
      { label: '> 200', value: { min: 200 } },
    ]},
  },
  {
    prop: 'commit_real_minutes',
    label: '实际耗时',
    minWidth: 95,
    align: 'right',
    // 显示走 getEffectiveReal（manual 覆盖），与后端裸列排序口径不同 → 客户端按显示值排
    clientSort: true,
    sortValue: getEffectiveReal,
    formatter: (row) => formatDuration(getEffectiveReal(row)),
    filter: { type: 'number', valueGetter: getEffectiveReal, shortcuts: [
      { label: '> 0', value: { min: 0.1 } },
      { label: '> 30min', value: { min: 30 } },
      { label: '> 1h', value: { min: 60 } },
    ]},
  },
  {
    prop: 'commit_ancient_minutes',
    label: '传统耗时预估',
    minWidth: 110,
    align: 'right',
    // 显示走 getEffectiveAncient（manual 覆盖）→ 客户端按显示值排
    clientSort: true,
    sortValue: getEffectiveAncient,
    formatter: (row) => formatDuration(getEffectiveAncient(row)),
    filter: { type: 'number', valueGetter: getEffectiveAncient, shortcuts: [
      { label: '> 0', value: { min: 0.1 } },
      { label: '> 30min', value: { min: 30 } },
      { label: '> 1h', value: { min: 60 } },
    ]},
  },
  {
    prop: 'efficiency_ratio',
    label: '提效比',
    minWidth: 85,
    align: 'center',
    // 显示走 CalcEfficiencyRatioManual（封顶/覆盖），与后端裸 SQL 排序口径不同 → 客户端按显示值排
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
    prop: '_tokens',
    label: 'Tokens消耗',
    minWidth: 100,
    align: 'right',
    formatter: (row) => {
      const total = (row.upstream_tokens || 0) + (row.downstream_tokens || 0)
      return total > 0 ? total.toLocaleString() : '-'
    },
    filter: { type: 'number', valueGetter: (row) => (row.upstream_tokens || 0) + (row.downstream_tokens || 0), shortcuts: [
      { label: '> 0', value: { min: 1 } },
      { label: '> 10k', value: { min: 10000 } },
      { label: '> 100k', value: { min: 100000 } },
    ]},
  },
  {
    prop: 'cost',
    label: '费用',
    minWidth: 75,
    align: 'right',
    sortField: 'cost',
    formatter: fmtCost,
    filter: { type: 'number', shortcuts: [
      { label: '> 0', value: { min: 0.001 } },
      { label: '> 0.01', value: { min: 0.01 } },
      { label: '> 0.1', value: { min: 0.1 } },
    ]},
  },
]

const loading = ref(false)
const tableData = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(250)
const order = ref(typeof route.query.order === 'string' ? route.query.order : undefined)

let serverDateRange = getDefaultDateRangeWide()

// 仅当 order 命中服务端列（声明了 sortField）时才把 order 下发给后端。
// 客户端列（clientSort）的 order 不下发，避免后端 400 或错误口径。
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
    // 将 cascade-org filter 值传给后端
    const orgFilter = filterTableRef.value?.getFilter('org_display') || {}
    if (orgFilter.org1) params.org1 = orgFilter.org1
    if (orgFilter.org2) params.org2 = orgFilter.org2
    if (orgFilter.org3) params.org3 = orgFilter.org3
    if (orgFilter.org4) params.org4 = orgFilter.org4

    const result = await getCommitsV2(params)
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

function goToOrg(row) {
  if (!row.org1) return
  const orgPath = [row.org1, row.org2, row.org3, row.org4].filter(Boolean).join('/')
  const [start, end] = serverDateRange
  router.push({
    path: '/org/' + encodeURIComponent(orgPath),
    query: {
      startDate: start.replace(/-/g, ''),
      endDate: end.replace(/-/g, ''),
    },
  })
}

function goToUser(userId, userName) {
  if (!userId && !userName) return
  const [start, end] = serverDateRange
  router.push({
    path: '/user/' + (userId || userName),
    query: {
      startDate: start.replace(/-/g, ''),
      endDate: end.replace(/-/g, ''),
    },
  })
}

function handleRepoClick(row) {
  if (!row.repo_addr) return
  const path = '/repo/' + encodeURIComponent(row.repo_addr) + '/' + encodeURIComponent(row.repo_branch || 'main')
  router.push({
    path,
    query: {
      startDate: serverDateRange[0].replace(/-/g, ''),
      endDate: serverDateRange[1].replace(/-/g, ''),
    }
  })
}

function handleFilterChange(allFilters) {
  const dateVal = allFilters.commit_time
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
  // 仅服务端列需要重新取数（回到第 1 页全局排序）；客户端列本地排序，无需请求。
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

watch(() => route.query, async (query) => {
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

<style scoped>
.repo-link-rtl {
  direction: rtl;
  unicode-bidi: plaintext;
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 100%;
}
</style>
