<template>
  <div class="kb-panel">
    <CollapsedTagBar :tags="collapsedSections" @expand="toggleSection" />

    <!-- 筛选区 -->
    <el-card v-if="!collapsed.filter" class="kb-filter-card" shadow="never">
      <template #header>
        <div style="display:flex;align-items:center;justify-content:space-between">
          <span>筛选条件</span>
          <el-button link size="small" @click="toggleSection('filter')">折叠</el-button>
        </div>
      </template>
      <div class="kb-filter-row">
        <el-select
          v-model="filterOrg1" placeholder="一级组织" clearable
          style="width:140px" @change="onOrg1Change"
        >
          <el-option v-for="o in org1Options" :key="o" :label="o" :value="o" />
        </el-select>
        <el-select
          v-model="filterOrg2" placeholder="二级组织" clearable
          style="width:140px" :disabled="!filterOrg1" @change="onOrg2Change"
        >
          <el-option v-for="o in org2Options" :key="o" :label="o" :value="o" />
        </el-select>
        <el-select
          v-model="filterOrg3" placeholder="三级组织" clearable
          style="width:140px" :disabled="!filterOrg2" @change="onOrg3Change"
        >
          <el-option v-for="o in org3Options" :key="o" :label="o" :value="o" />
        </el-select>
        <el-select
          v-model="filterOrg4" placeholder="四级组织" clearable
          style="width:140px" :disabled="!filterOrg3" @change="onOrg4Change"
        >
          <el-option v-for="o in org4Options" :key="o" :label="o" :value="o" />
        </el-select>
        <DateRangePicker v-model="dateRange" :clearable="false" @change="onDateChange" />
        <el-select v-model="granularity" style="width:90px" @change="onGranularityChange">
          <el-option label="天" value="day" />
          <el-option label="周" value="week" />
          <el-option label="月" value="month" />
          <el-option label="年" value="year" />
        </el-select>
      </div>
    </el-card>

    <!-- 表格区 -->
    <el-card v-if="!collapsed.table" shadow="never" style="margin-top: 12px">
      <template #header>
        <div style="display:flex;align-items:center;justify-content:space-between">
          <span>组织列表</span>
          <el-button link size="small" @click="toggleSection('table')">折叠</el-button>
        </div>
      </template>
      <KbFilterTable
        ref="filterTableRef"
        bare
        :columns="columns"
        :data="tableData"
        :loading="loading"
        :total="tableData.length"
      >
        <template #cell-org_name="{ row }">
          <el-link type="primary" @click.stop="goOrgDetail(row)">{{ row.org_name }}</el-link>
        </template>
        <template #cell-user_count="{ row }">
          <el-link v-if="row.user_count > 0" type="primary" @click.stop="goUserList(row)">{{ row.user_count }}</el-link>
          <span v-else>0</span>
        </template>
        <template #cell-task_count="{ row }">
          <el-link v-if="row.task_count > 0" type="primary" @click.stop="goTaskList(row)">{{ row.task_count }}</el-link>
          <span v-else>0</span>
        </template>
        <template #cell-commit_count="{ row }">
          <el-link v-if="row.commit_count > 0" type="primary" @click.stop="goCommitList(row)">{{ row.commit_count }}</el-link>
          <span v-else>0</span>
        </template>
        <template #cell-task_efficiency_ratio="{ row }">
          <el-tag v-if="row.task_efficiency_ratio > 0"
            :type="row.task_efficiency_ratio >= 300 ? 'success' : row.task_efficiency_ratio >= 150 ? 'primary' : 'info'"
            size="small">{{ row.task_efficiency_ratio.toFixed(1) }}%</el-tag>
          <el-tag v-else type="info" size="small">-</el-tag>
        </template>
        <template #cell-commit_efficiency_ratio="{ row }">
          <el-tag v-if="row.commit_efficiency_ratio > 0"
            :type="row.commit_efficiency_ratio >= 300 ? 'success' : row.commit_efficiency_ratio >= 150 ? 'primary' : 'info'"
            size="small">{{ row.commit_efficiency_ratio.toFixed(1) }}%</el-tag>
          <el-tag v-else type="info" size="small">-</el-tag>
        </template>
      </KbFilterTable>
    </el-card>

    <!-- 图表区域 -->
    <el-card v-if="!collapsed.charts && seriesData.length > 0" shadow="never" style="margin-top: 12px">
      <template #header>
        <div style="display:flex;align-items:center;justify-content:space-between">
          <span>图表</span>
          <el-button link size="small" @click="toggleSection('charts')">折叠</el-button>
        </div>
      </template>
      <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 12px">
        <div ref="chartMembersRef" style="height:260px"></div>
        <div ref="chartCountsRef" style="height:260px"></div>
        <div ref="chartCodeRef" style="height:260px"></div>
        <div ref="chartEffRef" style="height:260px"></div>
        <div ref="chartTokensRef" style="height:260px"></div>
        <div ref="chartCostRef" style="height:260px"></div>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted, nextTick, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import KbFilterTable from '@/components/KbFilterTable.vue'
import CollapsedTagBar from '@/components/CollapsedTagBar.vue'
import DateRangePicker from '@/components/DateRangePicker.vue'
import { useChart } from '@/composables/useChart'
import { useCollapse } from '@/composables/useCollapse'
import { getOrgV2 } from '@/api/es'
import { fmtCost } from '@/utils/formatters'
import { getDefaultDateRangeWide } from '@/utils/date'

const router = useRouter()
const route = useRoute()
const filterTableRef = ref(null)

const { collapsed, collapsedTags: collapsedSections, toggle: toggleSection, load: loadCollapsed } = useCollapse(
  'org_view_v2_collapsed',
  { filter: '筛选条件', table: '组织列表', charts: '图表' },
  (key) => { if (key === 'charts') updateCharts() }
)

const currentLevel = ref('org1')
const dateRange = ref(getDefaultDateRangeWide())
const granularity = ref('day')

const filterOrg1 = ref('')
const filterOrg2 = ref('')
const filterOrg3 = ref('')
const filterOrg4 = ref('')
const org1Options = ref([])
const org2Options = ref([])
const org3Options = ref([])
const org4Options = ref([])

async function loadOrgOptions(level, parent) {
  try {
    const dr = dateRange.value
    const params = { level, parent: parent || '' }
    if (dr && dr.length === 2) {
      params.startDate = dr[0].replace(/-/g, '')
      params.endDate = dr[1].replace(/-/g, '')
    }
    const result = await getOrgV2(params)
    const data = result.data || result
    return (data.data || []).map(d => d.org_name)
  } catch {
    return []
  }
}

async function onOrg1Change(val) {
  filterOrg2.value = ''
  filterOrg3.value = ''
  filterOrg4.value = ''
  org2Options.value = []
  org3Options.value = []
  org4Options.value = []
  if (val) org2Options.value = await loadOrgOptions('org2', val)
  currentLevel.value = resolveLevelFromFilters()
  updateUrl()
  fetchData()
}

async function onOrg2Change(val) {
  filterOrg3.value = ''
  filterOrg4.value = ''
  org3Options.value = []
  org4Options.value = []
  if (val) org3Options.value = await loadOrgOptions('org3', filterOrg1.value + '/' + val)
  currentLevel.value = resolveLevelFromFilters()
  updateUrl()
  fetchData()
}

async function onOrg3Change(val) {
  filterOrg4.value = ''
  org4Options.value = []
  if (val) org4Options.value = await loadOrgOptions('org4', filterOrg1.value + '/' + filterOrg2.value + '/' + val)
  currentLevel.value = resolveLevelFromFilters()
  updateUrl()
  fetchData()
}

function onOrg4Change() {
  currentLevel.value = resolveLevelFromFilters()
  updateUrl()
  fetchData()
}

function onDateChange() {
  updateUrl()
  fetchData()
}

const loading = ref(false)
const tableData = ref([])
const seriesData = ref([])
const allPeriods = ref([])

let _ignoreRouteWatch = false

const chartMembersRef = ref(null)
const chartCountsRef = ref(null)
const chartCodeRef = ref(null)
const chartEffRef = ref(null)
const chartTokensRef = ref(null)
const chartCostRef = ref(null)

const { setOption: setMembersOption } = useChart(chartMembersRef)
const { setOption: setCountsOption } = useChart(chartCountsRef)
const { setOption: setCodeOption } = useChart(chartCodeRef)
const { setOption: setEffOption } = useChart(chartEffRef)
const { setOption: setTokensOption } = useChart(chartTokensRef)
const { setOption: setCostOption } = useChart(chartCostRef)

const columns = [
  {
    prop: 'org_name',
    label: '组织',
    minWidth: 160,
    showOverflowTooltip: true,
    slotName: 'org_name',
    filter: { type: 'text' },
  },
  {
    prop: 'user_count',
    label: '成员数',
    width: 80,
    align: 'right',
    sortable: true,
    slotName: 'user_count',
  },
  {
    prop: 'task_count',
    label: 'Task数',
    width: 85,
    align: 'right',
    sortable: true,
    slotName: 'task_count',
  },
  {
    prop: 'task_diff_lines',
    label: 'Task代码量',
    width: 105,
    align: 'right',
    sortable: true,
  },
  {
    prop: 'task_efficiency_ratio',
    label: 'Task提效比',
    width: 105,
    align: 'center',
    sortable: true,
    slotName: 'task_efficiency_ratio',
  },
  {
    prop: 'commit_count',
    label: 'Commit数',
    width: 95,
    align: 'right',
    sortable: true,
    slotName: 'commit_count',
  },
  {
    prop: 'commit_diff_lines',
    label: 'Commit代码量',
    width: 115,
    align: 'right',
    sortable: true,
  },
  {
    prop: 'commit_efficiency_ratio',
    label: 'Commit提效比',
    width: 115,
    align: 'center',
    sortable: true,
    slotName: 'commit_efficiency_ratio',
  },
  {
    prop: 'total_tokens',
    label: 'Token消耗',
    width: 105,
    align: 'right',
    sortable: true,
    formatter: (row, col, val) => val > 0 ? val.toLocaleString() : '-',
  },
  {
    prop: 'total_cost',
    label: '总费用',
    width: 90,
    align: 'right',
    sortable: true,
    formatter: fmtCost,
  },
]

function parseDateRange(startDate, endDate) {
  if (startDate && endDate) {
    const fmt = s => s.slice(0, 4) + '-' + s.slice(4, 6) + '-' + s.slice(6, 8)
    return [fmt(startDate), fmt(endDate)]
  }
  return getDefaultDateRangeWide()
}

function resolveLevelFromFilters() {
  if (filterOrg4.value) return 'org4'
  if (filterOrg3.value) return 'org4'
  if (filterOrg2.value) return 'org3'
  if (filterOrg1.value) return 'org2'
  return 'org1'
}

function getCurrentParent() {
  const level = currentLevel.value
  const parts = []
  if (level === 'org2' && filterOrg1.value) parts.push(filterOrg1.value)
  else if (level === 'org3') {
    if (filterOrg1.value) parts.push(filterOrg1.value)
    if (filterOrg2.value) parts.push(filterOrg2.value)
  } else if (level === 'org4') {
    if (filterOrg1.value) parts.push(filterOrg1.value)
    if (filterOrg2.value) parts.push(filterOrg2.value)
    if (filterOrg3.value) parts.push(filterOrg3.value)
  }
  return parts.join('/')
}

function getOrgPath(orgName) {
  const parent = getCurrentParent()
  return parent ? parent + '/' + orgName : orgName
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
  currentLevel.value = resolveLevelFromFilters()
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

async function fetchData() {
  if (!dateRange.value || dateRange.value.length !== 2) return
  loading.value = true
  try {
    const params = {
      level: currentLevel.value,
      parent: getCurrentParent(),
      startDate: dateRange.value[0].replace(/-/g, ''),
      endDate: dateRange.value[1].replace(/-/g, ''),
      granularity: granularity.value,
    }
    const result = await getOrgV2(params)
    const data = result.data || result
    tableData.value = data.data || []
    seriesData.value = data.series || []
    allPeriods.value = data.periods || []
    await nextTick()
    if (seriesData.value.length > 0) updateCharts()
  } catch {
    tableData.value = []
    seriesData.value = []
    allPeriods.value = []
  } finally {
    loading.value = false
  }
}

function makeLineOption(title, seriesList) {
  return {
    title: { text: title, left: 'center', textStyle: { fontSize: 14, fontWeight: 'bold' } },
    tooltip: { trigger: 'axis' },
    legend: { data: seriesList.map(s => s.name), top: '8%', type: 'scroll' },
    grid: { left: '5%', right: '5%', top: '22%', bottom: '10%', containLabel: true },
    xAxis: { type: 'category', data: allPeriods.value, axisLabel: { rotate: 45, fontSize: 11 } },
    yAxis: { type: 'value' },
    series: seriesList.map(s => ({
      name: s.name,
      type: 'line',
      smooth: true,
      data: s.data,
    })),
  }
}

function makeLineOptionPct(title, seriesList) {
  return {
    title: { text: title, left: 'center', textStyle: { fontSize: 14, fontWeight: 'bold' } },
    tooltip: { trigger: 'axis', formatter(params) {
      let s = params[0].axisValue + '<br/>'
      params.forEach(p => { s += p.marker + p.seriesName + ': ' + p.value.toFixed(1) + '%<br/>' })
      return s
    }},
    legend: { data: seriesList.map(s => s.name), top: '8%', type: 'scroll' },
    grid: { left: '5%', right: '5%', top: '22%', bottom: '10%', containLabel: true },
    xAxis: { type: 'category', data: allPeriods.value, axisLabel: { rotate: 45, fontSize: 11 } },
    yAxis: { type: 'value', axisLabel: { formatter: '{value}%' } },
    series: seriesList.map(s => ({
      name: s.name,
      type: 'line',
      smooth: true,
      data: s.data,
    })),
  }
}

function updateCharts() {
  const series = seriesData.value
  const periods = allPeriods.value
  if (!series.length || !periods.length) return

  const getPoints = (orgSeries, field) =>
    orgSeries.points.map(p => p[field] ?? 0)

  // a) 成员数
  setMembersOption(makeLineOption('成员数（按组织）', series.map(s => ({
    name: s.org_name,
    data: getPoints(s, 'user_count'),
  }))))

  // b) Task数 & Commit数（每个 org 两条线）
  const countSeries = []
  series.forEach(s => {
    countSeries.push({ name: s.org_name + ' Task数', data: getPoints(s, 'task_count') })
    countSeries.push({ name: s.org_name + ' Commit数', data: getPoints(s, 'commit_count') })
  })
  setCountsOption(makeLineOption('Task数 & Commit数（按组织）', countSeries))

  // c) Task代码量 & Commit代码量
  const codeSeries = []
  series.forEach(s => {
    codeSeries.push({ name: s.org_name + ' Task代码量', data: getPoints(s, 'task_diff_lines') })
    codeSeries.push({ name: s.org_name + ' Commit代码量', data: getPoints(s, 'commit_diff_lines') })
  })
  setCodeOption(makeLineOption('代码量（按组织）', codeSeries))

  // d) Task提效比 & Commit提效比
  const effSeries = []
  series.forEach(s => {
    effSeries.push({ name: s.org_name + ' Task提效比', data: getPoints(s, 'task_efficiency_ratio') })
    effSeries.push({ name: s.org_name + ' Commit提效比', data: getPoints(s, 'commit_efficiency_ratio') })
  })
  setEffOption(makeLineOptionPct('提效比（按组织）', effSeries))

  // e) Token消耗
  setTokensOption(makeLineOption('Token消耗（按组织）', series.map(s => ({
    name: s.org_name,
    data: getPoints(s, 'total_tokens'),
  }))))

  // f) 总费用
  setCostOption({
    ...makeLineOption('总费用（按组织）', series.map(s => ({
      name: s.org_name,
      data: getPoints(s, 'total_cost'),
    }))),
    tooltip: {
      trigger: 'axis',
      formatter(params) {
        let str = params[0].axisValue + '<br/>'
        params.forEach(p => { str += p.marker + p.seriesName + ': ' + Number(p.value).toFixed(2) + ' 元<br/>' })
        return str
      }
    },
  })
}

function goOrgDetail(row) {
  const orgPath = getOrgPath(row.org_name)
  const [start, end] = dateRange.value
  router.push({
    path: '/org/' + encodeURIComponent(orgPath),
    query: {
      startDate: start.replace(/-/g, ''),
      endDate: end.replace(/-/g, ''),
    },
  })
}

function buildOrgJumpQuery(row) {
  const orgPath = getOrgPath(row.org_name)
  const parts = orgPath.split('/')
  const [start, end] = dateRange.value
  const query = {
    startDate: start.replace(/-/g, ''),
    endDate: end.replace(/-/g, ''),
  }
  if (parts[0]) query.org1 = parts[0]
  if (parts[1]) query.org2 = parts[1]
  if (parts[2]) query.org3 = parts[2]
  if (parts[3]) query.org4 = parts[3]
  return query
}

function goUserList(row) {
  router.push({ path: '/user-v2', query: buildOrgJumpQuery(row) })
}

function goTaskList(row) {
  router.push({ path: '/task-v2', query: buildOrgJumpQuery(row) })
}

function goCommitList(row) {
  router.push({ path: '/commit-v2', query: buildOrgJumpQuery(row) })
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
  org1Options.value = await loadOrgOptions('org1', '')
  if (filterOrg1.value) org2Options.value = await loadOrgOptions('org2', filterOrg1.value)
  if (filterOrg2.value) org3Options.value = await loadOrgOptions('org3', filterOrg1.value + '/' + filterOrg2.value)
  if (filterOrg3.value) org4Options.value = await loadOrgOptions('org4', filterOrg1.value + '/' + filterOrg2.value + '/' + filterOrg3.value)
  await fetchData()
})
</script>
