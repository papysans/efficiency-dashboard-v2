<template>
  <div class="kb-panel">
    <CollapsedTagBar :tags="collapsedTags" @expand="toggleSection" />

    <!-- 条件筛选区 -->
    <div v-if="!collapsed.filter">
      <FilterBar
        v-model:dateRange="dateRange"
        :show-org="true"
        :org-value="orgValue"
        @change="onFilterChange"
      >
        <el-select v-model="granularity" style="width:90px" @change="onGranularityChange">
          <el-option label="天" value="day" />
          <el-option label="周" value="week" />
          <el-option label="月" value="month" />
          <el-option label="年" value="year" />
        </el-select>
        <el-button link size="small" @click="toggleSection('filter')" style="margin-left:4px">折叠</el-button>
      </FilterBar>
    </div>

    <!-- 用户列表区 -->
    <el-card v-if="!collapsed.table" shadow="never" style="margin-top: 12px">
      <template #header>
        <div style="display:flex;align-items:center;justify-content:space-between">
          <span>用户列表</span>
          <el-button link size="small" @click="toggleSection('table')">折叠</el-button>
        </div>
      </template>
      <KbFilterTable
        ref="filterTableRef"
        :columns="columns"
        :data="tableData"
        :loading="loading"
        :total="total"
        v-model:page="page"
        v-model:pageSize="pageSize"
        @size-change="handleSizeChange"
        @page-change="handlePageChange"
        @filter-change="handleFilterChange"
      >
        <template #cell-org_display="{ row }">
          <el-link v-if="row.org_display" type="primary" @click.stop="handleOrgClick(row)">
            {{ row.org_display }}
          </el-link>
          <span v-else>-</span>
        </template>
        <template #cell-user_name="{ row }">
          <el-link type="primary" @click.stop="handleUserClick(row)">{{ row.user_name || row.user_id }}</el-link>
        </template>
        <template #cell-task_efficiency_ratio="{ row }">
          <el-tag
            v-if="row.task_efficiency_ratio != null"
            :type="row.task_efficiency_ratio >= 300 ? 'success' : row.task_efficiency_ratio >= 150 ? 'primary' : 'info'"
            size="small"
          >{{ row.task_efficiency_ratio.toFixed(1) }}%</el-tag>
          <el-tag v-else type="info" size="small">-</el-tag>
        </template>
        <template #cell-commit_efficiency_ratio="{ row }">
          <el-tag
            v-if="row.commit_efficiency_ratio != null"
            :type="row.commit_efficiency_ratio >= 300 ? 'success' : row.commit_efficiency_ratio >= 150 ? 'primary' : 'info'"
            size="small"
          >{{ row.commit_efficiency_ratio.toFixed(1) }}%</el-tag>
          <el-tag v-else type="info" size="small">-</el-tag>
        </template>
      </KbFilterTable>
    </el-card>

    <!-- 图表区 -->
    <el-card v-if="!collapsed.charts && seriesData.length > 0" shadow="never" style="margin-top: 12px">
      <template #header>
        <div style="display:flex;align-items:center;justify-content:space-between">
          <span>图表</span>
          <el-button link size="small" @click="toggleSection('charts')">折叠</el-button>
        </div>
      </template>
      <div style="display:grid;grid-template-columns:1fr 1fr;gap:12px">
        <div ref="chartCountsRef" style="height:260px"></div>
        <div ref="chartCodeRef" style="height:260px"></div>
        <div ref="chartTimeRef" style="height:260px"></div>
        <div ref="chartEffRef" style="height:260px"></div>
        <div ref="chartTokensRef" style="height:260px"></div>
        <div ref="chartCostRef" style="height:260px"></div>
      </div>
    </el-card>

  </div>
</template>

<script setup>
import { ref, computed, onMounted, nextTick, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import KbFilterTable from '@/components/KbFilterTable.vue'
import FilterBar from '@/components/FilterBar.vue'
import CollapsedTagBar from '@/components/CollapsedTagBar.vue'
import { getUsersV2, createUserGroup } from '@/api/es'
import { fmtCost, formatDuration } from '@/utils/formatters'
import { getDefaultDateRangeWide } from '@/utils/date'
import { useChart } from '@/composables/useChart'
import { useCollapse } from '@/composables/useCollapse'
import { ElMessage } from 'element-plus'

const router = useRouter()
const route = useRoute()
const filterTableRef = ref(null)

const columns = [
  {
    prop: 'org_display',
    label: '组织',
    minWidth: 160,
    slotName: 'org_display',
    filter: { type: 'cascade-org' },
  },
  {
    prop: 'user_name',
    label: '用户名',
    minWidth: 120,
    slotName: 'user_name',
    filter: { type: 'multi-select' },
  },
  {
    prop: 'task_count',
    label: 'Task数',
    minWidth: 85,
    align: 'right',
    sortable: true,
  },
  {
    prop: 'commit_count',
    label: 'Commit数',
    minWidth: 95,
    align: 'right',
    sortable: true,
  },
  {
    prop: 'task_diff_lines',
    label: 'Task代码量',
    minWidth: 110,
    align: 'right',
    sortable: true,
    filter: { type: 'number', shortcuts: [
      { label: '> 0', value: { min: 1 } },
      { label: '> 100', value: { min: 100 } },
      { label: '> 500', value: { min: 500 } },
    ]},
  },
  {
    prop: 'commit_diff_lines',
    label: 'Commit代码量',
    minWidth: 120,
    align: 'right',
    sortable: true,
    filter: { type: 'number', shortcuts: [
      { label: '> 0', value: { min: 1 } },
      { label: '> 100', value: { min: 100 } },
      { label: '> 500', value: { min: 500 } },
    ]},
  },
  {
    prop: 'task_real_minutes',
    label: 'Task实际耗时',
    minWidth: 120,
    align: 'right',
    sortable: true,
    formatter: (row) => row.task_real_minutes ? row.task_real_minutes.toFixed(1) + ' min' : '-',
  },
  {
    prop: 'commit_real_minutes',
    label: 'Commit实际耗时',
    minWidth: 130,
    align: 'right',
    sortable: true,
    formatter: (row) => row.commit_real_minutes ? row.commit_real_minutes.toFixed(1) + ' min' : '-',
  },
  {
    prop: 'task_efficiency_ratio',
    label: 'Task提效比',
    minWidth: 110,
    align: 'center',
    sortable: true,
    slotName: 'task_efficiency_ratio',
    filter: { type: 'number', shortcuts: [
      { label: '> 100%', value: { min: 100 } },
      { label: '> 200%', value: { min: 200 } },
      { label: '> 300%', value: { min: 300 } },
    ]},
  },
  {
    prop: 'commit_efficiency_ratio',
    label: 'Commit提效比',
    minWidth: 120,
    align: 'center',
    sortable: true,
    slotName: 'commit_efficiency_ratio',
    filter: { type: 'number', shortcuts: [
      { label: '> 100%', value: { min: 100 } },
      { label: '> 200%', value: { min: 200 } },
      { label: '> 300%', value: { min: 300 } },
    ]},
  },
  {
    prop: '_tokens',
    label: 'Tokens消耗',
    minWidth: 110,
    align: 'right',
    sortable: true,
    formatter: (row) => {
      const total = (row.upstream_tokens || 0) + (row.downstream_tokens || 0)
      return total > 0 ? total.toLocaleString() : '-'
    },
  },
  {
    prop: 'cost',
    label: '费用',
    minWidth: 80,
    align: 'right',
    sortable: true,
    formatter: fmtCost,
  },
]

const loading = ref(false)
const tableData = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(50)
const granularity = ref('day')
const dateRange = ref(getDefaultDateRangeWide())

const filterOrg1 = ref('')
const filterOrg2 = ref('')
const filterOrg3 = ref('')
const filterOrg4 = ref('')

const orgValue = computed(() => ({
  org1: filterOrg1.value,
  org2: filterOrg2.value,
  org3: filterOrg3.value,
  org4: filterOrg4.value,
}))

let _ignoreRouteWatch = false

const seriesData = ref([])
const allPeriods = ref([])

// ---- 折叠状态 ----
const { collapsed, collapsedTags, toggle: toggleSection, load: loadCollapsed } = useCollapse(
  'user_view_v2_collapsed',
  { filter: '条件筛选', table: '用户列表', charts: '图表' },
  (key) => { if (key === 'charts' && seriesData.value.length > 0) updateCharts() }
)

// ---- Charts ----
const chartCountsRef = ref(null)
const chartCodeRef = ref(null)
const chartTimeRef = ref(null)
const chartEffRef = ref(null)
const chartTokensRef = ref(null)
const chartCostRef = ref(null)

const { setOption: setCountsOption } = useChart(chartCountsRef)
const { setOption: setCodeOption } = useChart(chartCodeRef)
const { setOption: setTimeOption } = useChart(chartTimeRef)
const { setOption: setEffOption } = useChart(chartEffRef)
const { setOption: setTokensOption } = useChart(chartTokensRef)
const { setOption: setCostOption } = useChart(chartCostRef)

function parseDateRange(startDate, endDate) {
  if (startDate && endDate) {
    const fmt = s => s.slice(0, 4) + '-' + s.slice(4, 6) + '-' + s.slice(6, 8)
    return [fmt(startDate), fmt(endDate)]
  }
  return getDefaultDateRangeWide()
}

function onFilterChange({ dateRange: dr, org1, org2, org3, org4 }) {
  dateRange.value = dr
  filterOrg1.value = org1 || ''
  filterOrg2.value = org2 || ''
  filterOrg3.value = org3 || ''
  filterOrg4.value = org4 || ''
  page.value = 1
  updateUrl()
  fetchData()
}

function onGranularityChange() {
  updateUrl()
  fetchData()
}

function syncUrlToControls() {
  const { startDate, endDate, org1, org2, org3, org4, granularity: g } = route.query
  dateRange.value = parseDateRange(startDate, endDate)
  filterOrg1.value = org1 || ''
  filterOrg2.value = org2 || ''
  filterOrg3.value = org3 || ''
  filterOrg4.value = org4 || ''
  if (g) granularity.value = g
}

function updateUrl() {
  const [start, end] = dateRange.value
  const query = {
    startDate: start.replace(/-/g, ''),
    endDate: end.replace(/-/g, ''),
    granularity: granularity.value,
  }
  if (filterOrg1.value) query.org1 = filterOrg1.value
  if (filterOrg2.value) query.org2 = filterOrg2.value
  if (filterOrg3.value) query.org3 = filterOrg3.value
  if (filterOrg4.value) query.org4 = filterOrg4.value
  _ignoreRouteWatch = true
  router.replace({ query }).finally(() => { _ignoreRouteWatch = false })
}

function makeBarOption(title, seriesList) {
  return {
    title: { text: title, left: 'center', textStyle: { fontSize: 13, fontWeight: 'bold' } },
    tooltip: { trigger: 'axis' },
    legend: { data: seriesList.map(s => s.name), top: '8%', type: 'scroll' },
    grid: { left: '5%', right: '5%', top: '22%', bottom: '10%', containLabel: true },
    xAxis: { type: 'category', data: allPeriods.value, axisLabel: { rotate: 45, fontSize: 11 } },
    yAxis: { type: 'value' },
    series: seriesList.map(s => ({
      name: s.name,
      type: 'bar',
      data: s.data,
    })),
  }
}

function makeBarOptionPct(title, seriesList) {
  return {
    title: { text: title, left: 'center', textStyle: { fontSize: 13, fontWeight: 'bold' } },
    tooltip: {
      trigger: 'axis',
      formatter(params) {
        let str = params[0].axisValue + '<br/>'
        params.forEach(p => { str += p.marker + p.seriesName + ': ' + p.value.toFixed(1) + '%<br/>' })
        return str
      },
    },
    legend: { data: seriesList.map(s => s.name), top: '8%', type: 'scroll' },
    grid: { left: '5%', right: '5%', top: '22%', bottom: '10%', containLabel: true },
    xAxis: { type: 'category', data: allPeriods.value, axisLabel: { rotate: 45, fontSize: 11 } },
    yAxis: { type: 'value', axisLabel: { formatter: '{value}%' } },
    series: seriesList.map(s => ({
      name: s.name,
      type: 'bar',
      data: s.data,
    })),
  }
}

function getFilteredSeries() {
  const filtered = filterTableRef.value?.filteredData
  if (!filtered || filtered.length === seriesData.value.length) return seriesData.value
  const userNames = new Set(filtered.map(r => r.user_name))
  return seriesData.value.filter(s => userNames.has(s.user_name))
}

function updateCharts() {
  const series = getFilteredSeries()
  const periods = allPeriods.value
  if (!series.length || !periods.length) return

  const getPoints = (userSeries, field) =>
    userSeries.points.map(p => p[field] ?? 0)

  const countSeries = []
  series.forEach(s => {
    countSeries.push({ name: s.user_name + ' Task数', data: getPoints(s, 'task_count') })
    countSeries.push({ name: s.user_name + ' Commit数', data: getPoints(s, 'commit_count') })
  })
  setCountsOption(makeBarOption('Task数 & Commit数', countSeries))

  const codeSeries = []
  series.forEach(s => {
    codeSeries.push({ name: s.user_name + ' Task代码量', data: getPoints(s, 'task_diff_lines') })
    codeSeries.push({ name: s.user_name + ' Commit代码量', data: getPoints(s, 'commit_diff_lines') })
  })
  setCodeOption(makeBarOption('Task代码量 & Commit代码量', codeSeries))

  const timeSeries = []
  series.forEach(s => {
    timeSeries.push({ name: s.user_name + ' Task传统耗时', data: getPoints(s, 'task_ancient_minutes') })
    timeSeries.push({ name: s.user_name + ' Commit传统耗时', data: getPoints(s, 'commit_ancient_minutes') })
    timeSeries.push({ name: s.user_name + ' Task实际耗时', data: getPoints(s, 'task_real_minutes') })
    timeSeries.push({ name: s.user_name + ' Commit实际耗时', data: getPoints(s, 'commit_real_minutes') })
  })
  setTimeOption({
    ...makeBarOption('传统耗时 & 实际耗时（min）', timeSeries),
    tooltip: {
      trigger: 'axis',
      formatter(params) {
        let str = params[0].axisValue + '<br/>'
        params.forEach(p => { str += p.marker + p.seriesName + ': ' + formatDuration(p.value) + '<br/>' })
        return str
      },
    },
  })

  const effSeries = []
  series.forEach(s => {
    effSeries.push({ name: s.user_name + ' Task提效比', data: getPoints(s, 'task_efficiency_ratio') })
    effSeries.push({ name: s.user_name + ' Commit提效比', data: getPoints(s, 'commit_efficiency_ratio') })
  })
  setEffOption(makeBarOptionPct('Task提效比 & Commit提效比', effSeries))

  setTokensOption(makeBarOption('Token消耗', series.map(s => ({
    name: s.user_name,
    data: getPoints(s, 'total_tokens'),
  }))))

  setCostOption({
    ...makeBarOption('总费用', series.map(s => ({
      name: s.user_name,
      data: getPoints(s, 'total_cost'),
    }))),
    tooltip: {
      trigger: 'axis',
      formatter(params) {
        let str = params[0].axisValue + '<br/>'
        params.forEach(p => { str += p.marker + p.seriesName + ': ' + Number(p.value).toFixed(2) + ' 元<br/>' })
        return str
      },
    },
  })
}

function handleOrgClick(row) {
  const orgPath = [row.org1, row.org2, row.org3, row.org4].filter(Boolean).join('/')
  if (!orgPath) return
  const query = {}
  if (row.org1) query.org1 = row.org1
  if (row.org2) query.org2 = row.org2
  if (row.org3) query.org3 = row.org3
  if (row.org4) query.org4 = row.org4
  router.push({ path: '/org/' + encodeURIComponent(orgPath), query })
}

function handleUserClick(row) {
  const [start, end] = dateRange.value
  router.push({
    path: '/user/' + row.user_id,
    query: {
      startDate: start.replace(/-/g, ''),
      endDate: end.replace(/-/g, ''),
    },
  })
}

function handleFilterChange() {
  page.value = 1
  if (seriesData.value.length > 0 && !collapsed.value.charts) {
    nextTick(() => updateCharts())
  }
}

function handleSizeChange() {
  page.value = 1
  fetchData()
}

function handlePageChange() {
  fetchData()
}

async function fetchData() {
  if (!dateRange.value || dateRange.value.length !== 2) return
  loading.value = true
  try {
    const params = {
      startDate: dateRange.value[0].replace(/-/g, ''),
      endDate: dateRange.value[1].replace(/-/g, ''),
      page: page.value,
      pageSize: pageSize.value,
      granularity: granularity.value,
    }
    if (filterOrg1.value) params.org1 = filterOrg1.value
    if (filterOrg2.value) params.org2 = filterOrg2.value
    if (filterOrg3.value) params.org3 = filterOrg3.value
    if (filterOrg4.value) params.org4 = filterOrg4.value

    const result = await getUsersV2(params)
    const data = result.data || result
    tableData.value = data.data || []
    total.value = data.total || 0
    seriesData.value = data.series || []
    allPeriods.value = data.periods || []

    await nextTick()
    if (seriesData.value.length > 0 && !collapsed.value.charts) updateCharts()
  } catch {
    tableData.value = []
    total.value = 0
    seriesData.value = []
    allPeriods.value = []
  } finally {
    loading.value = false
  }
}

watch(() => route.query, async () => {
  if (_ignoreRouteWatch) return
  await nextTick()
  syncUrlToControls()
  await fetchData()
}, { deep: true })

onMounted(async () => {
  loadCollapsed()
  await nextTick()
  syncUrlToControls()
  await fetchData()
})
</script>


