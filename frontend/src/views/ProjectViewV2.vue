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
        <el-input
          v-model="filterName"
          placeholder="项目名称"
          clearable
          style="width:180px"
          @input="onFilterChange"
        />
        <DateRangePicker
          v-model="filterStartRange"
          placeholder="开始时间范围"
          :clearable="true"
          @change="onFilterChange"
        />
        <DateRangePicker
          v-model="filterEndRange"
          placeholder="结束时间范围"
          :clearable="true"
          @change="onFilterChange"
        />
        <el-checkbox v-model="filterOngoing" @change="onFilterChange">仅显示尚未结束</el-checkbox>
      </div>
    </el-card>

    <!-- 表格区 -->
    <el-card v-if="!collapsed.table" shadow="never" style="margin-top:12px">
      <template #header>
        <div style="display:flex;align-items:center;justify-content:space-between">
          <span>项目列表</span>
          <div style="display:flex;gap:8px;align-items:center">
            <el-button type="primary" size="small" @click="showCreateDialog = true">+ 创建项目</el-button>
            <el-button link size="small" @click="toggleSection('table')">折叠</el-button>
          </div>
        </div>
      </template>
      <KbFilterTable
        ref="filterTableRef"
        bare
        :columns="columns"
        :data="filteredData"
        :loading="loading"
        :total="filteredData.length"
        :order="order"
        :page-sizes="[250, 500, 1000]"
        row-class-name="kb-clickable-row"
        @row-click="handleRowClick"
        @sort-change="onSortChange"
      >
        <template #cell-end_time_display="{ row }">
          <span v-if="row._ongoing" style="color:#67c23a;font-weight:500">尚未结束</span>
          <span v-else>{{ row._end_time_fmt }}</span>
        </template>
        <template #cell-efficiency_ratio="{ row }">
          <el-tag
            v-if="row.efficiency_ratio != null"
            :type="row.efficiency_ratio >= 300 ? 'success' : row.efficiency_ratio >= 150 ? 'primary' : 'info'"
            size="small"
          >{{ row.efficiency_ratio.toFixed(1) }}%</el-tag>
          <el-tag v-else type="info" size="small">-</el-tag>
        </template>
        <template #cell-actions="{ row }">
          <el-button type="danger" link size="small" @click.stop="handleDelete(row)">删除</el-button>
        </template>
      </KbFilterTable>
    </el-card>

    <!-- 图表区 -->
    <el-card v-if="!collapsed.charts && filteredData.length > 0" shadow="never" style="margin-top:12px">
      <template #header>
        <div style="display:flex;align-items:center;justify-content:space-between">
          <span>图表</span>
          <el-button link size="small" @click="toggleSection('charts')">折叠</el-button>
        </div>
      </template>
      <div style="display:grid;grid-template-columns:1fr 1fr;gap:12px">
        <div ref="chartEffRef" style="height:280px"></div>
        <div ref="chartCodeRef" style="height:280px"></div>
        <div ref="chartTimeRef" style="height:280px"></div>
        <div ref="chartPeopleRef" style="height:280px"></div>
        <div ref="chartCostRef" style="height:280px"></div>
      </div>
    </el-card>

    <!-- 创建项目对话框 -->
    <el-dialog v-model="showCreateDialog" title="创建项目" width="500px">
      <el-form label-width="80px">
        <el-form-item label="项目名称">
          <el-input v-model="createForm.name" placeholder="输入项目名称" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="createForm.description" type="textarea" :rows="3" placeholder="输入项目描述（可选）" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreateDialog = false">取消</el-button>
        <el-button type="primary" @click="handleCreate" :loading="creating">创建</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import KbFilterTable from '@/components/KbFilterTable.vue'
import CollapsedTagBar from '@/components/CollapsedTagBar.vue'
import DateRangePicker from '@/components/DateRangePicker.vue'
import { useChart } from '@/composables/useChart'
import { useCollapse } from '@/composables/useCollapse'
import { getProjects, createProject, deleteProject } from '@/api/es'
import { fmtCost, formatLocalTime, formatDuration } from '@/utils/formatters'
import { parseOrder } from '@/utils/sort'
import { ElMessage, ElMessageBox } from 'element-plus'

const router = useRouter()
const route = useRoute()
const loading = ref(false)
const tableData = ref([])
const filterTableRef = ref(null)

const { collapsed, collapsedTags: collapsedSections, toggle: toggleSection, load: loadCollapsed } = useCollapse(
  'project_view_v2_collapsed',
  { filter: '筛选条件', table: '项目列表', charts: '图表' },
  (key) => { if (key === 'charts') nextTick(() => updateCharts()) }
)

const filterName = ref('')
const filterStartRange = ref(null)
const filterEndRange = ref(null)
const filterOngoing = ref(false)
const order = ref(typeof route.query.order === 'string' ? route.query.order : undefined)

// 创建对话框
const showCreateDialog = ref(false)
const creating = ref(false)
const createForm = ref({ name: '', description: '' })

const columns = [
  {
    prop: 'name',
    label: '项目名称',
    minWidth: 200,
  },
  {
    prop: 'start_time',
    label: '开始时间',
    minWidth: 150,
    formatter: (row) => formatLocalTime(row.start_time_manual ?? row.start_time),
  },
  {
    prop: 'end_time_display',
    label: '结束时间',
    minWidth: 150,
    slotName: 'end_time_display',
  },
  {
    prop: 'user_count',
    label: '人数',
    minWidth: 80,
    align: 'right',
    sortField: 'userCount',
  },
  {
    prop: 'repo_count',
    label: 'Repo数',
    minWidth: 90,
    align: 'right',
    sortField: 'repoCount',
  },
  {
    prop: 'task_count',
    label: 'Task数',
    minWidth: 90,
    align: 'right',
    sortField: 'taskCount',
  },
  {
    prop: 'total_code_lines',
    label: '生成代码量',
    minWidth: 110,
    align: 'right',
    sortField: 'totalCodeLines',
    formatter: (row, col, val) => val > 0 ? val.toLocaleString() + ' 行' : '-',
  },
  {
    prop: 'actual_lines_per_day',
    label: '实际人天代码量',
    minWidth: 130,
    align: 'right',
    sortField: 'actualLinesPerDay',
    formatter: (row, col, val) => val != null ? Math.round(val).toLocaleString() + ' 行/人天' : '-',
  },
  {
    prop: 'cost',
    label: '费用',
    minWidth: 100,
    align: 'right',
    sortField: 'cost',
    formatter: fmtCost,
  },
  {
    prop: 'project_real_lead_minutes',
    label: '项目周期',
    minWidth: 120,
    align: 'right',
    sortable: false,
    formatter: (row) => formatDuration(row.project_real_lead_minutes_manual ?? row.project_real_lead_minutes),
  },
  {
    prop: 'project_ancient_minutes',
    label: '传统开发预估',
    minWidth: 130,
    align: 'right',
    // 显示走 manual 覆盖值 → 客户端按显示值排
    clientSort: true,
    sortValue: (row) => row.project_ancient_minutes_manual ?? row.project_ancient_minutes ?? null,
    formatter: (row) => formatDuration(row.project_ancient_minutes_manual ?? row.project_ancient_minutes),
  },
  {
    prop: 'project_real_process_minutes',
    label: '实际耗时',
    minWidth: 120,
    align: 'right',
    clientSort: true,
    sortValue: (row) => row.project_real_process_minutes_manual ?? row.project_real_process_minutes ?? null,
    formatter: (row) => formatDuration(row.project_real_process_minutes_manual ?? row.project_real_process_minutes),
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
  },
  {
    prop: '_actions',
    label: '操作',
    width: 80,
    align: 'center',
    sortable: false,
    slotName: 'actions',
  },
]

// 图表 refs
const chartEffRef = ref(null)
const chartCodeRef = ref(null)
const chartTimeRef = ref(null)
const chartPeopleRef = ref(null)
const chartCostRef = ref(null)

const { setOption: setEffOption } = useChart(chartEffRef)
const { setOption: setCodeOption } = useChart(chartCodeRef)
const { setOption: setTimeOption } = useChart(chartTimeRef)
const { setOption: setPeopleOption } = useChart(chartPeopleRef)
const { setOption: setCostOption } = useChart(chartCostRef)

let _ignoreRouteWatch = false

function parseDateStr(s) {
  if (!s || s.length < 8) return null
  return s.slice(0, 4) + '-' + s.slice(4, 6) + '-' + s.slice(6, 8)
}

function syncUrlToControls() {
  const q = route.query
  filterName.value = q.name ? String(q.name).trim() : ''
  filterOngoing.value = q.ongoing === '1'
  order.value = typeof q.order === 'string' ? q.order : undefined
  filterStartRange.value = (q.startFrom && q.startTo)
    ? [parseDateStr(q.startFrom), parseDateStr(q.startTo)]
    : null
  filterEndRange.value = (q.endFrom && q.endTo)
    ? [parseDateStr(q.endFrom), parseDateStr(q.endTo)]
    : null
}

function updateUrl() {
  const query = {}
  if (filterName.value) query.name = filterName.value
  if (filterOngoing.value) query.ongoing = '1'
  if (filterStartRange.value && filterStartRange.value.length === 2) {
    query.startFrom = filterStartRange.value[0].replace(/-/g, '')
    query.startTo = filterStartRange.value[1].replace(/-/g, '')
  }
  if (filterEndRange.value && filterEndRange.value.length === 2) {
    query.endFrom = filterEndRange.value[0].replace(/-/g, '')
    query.endTo = filterEndRange.value[1].replace(/-/g, '')
  }
  if (order.value) query.order = order.value
  _ignoreRouteWatch = true
  router.replace({ query }).finally(() => { _ignoreRouteWatch = false })
}

function onFilterChange() {
  updateUrl()
  nextTick(() => updateCharts())
}

function enrichData(list) {
  return (list || []).map(item => {
    const endTime = item.end_time_manual ?? item.end_time
    const ongoing = !endTime
    return {
      ...item,
      _ongoing: ongoing,
      _end_time_fmt: ongoing ? '' : formatLocalTime(endTime),
    }
  })
}

const filteredData = computed(() => {
  let data = tableData.value
  const name = filterName.value.trim().toLowerCase()
  if (name) data = data.filter(r => (r.name || '').toLowerCase().includes(name))
  if (filterOngoing.value) data = data.filter(r => r._ongoing)
  if (filterStartRange.value && filterStartRange.value.length === 2) {
    const [from, to] = filterStartRange.value
    data = data.filter(r => {
      const st = r.start_time_manual ?? r.start_time
      if (!st) return false
      const d = st.slice(0, 10)
      return d >= from && d <= to
    })
  }
  if (filterEndRange.value && filterEndRange.value.length === 2) {
    const [from, to] = filterEndRange.value
    data = data.filter(r => {
      if (r._ongoing) return false
      const et = r.end_time_manual ?? r.end_time
      if (!et) return false
      const d = et.slice(0, 10)
      return d >= from && d <= to
    })
  }
  return data
})

// 仅当 order 命中服务端列（声明了 sortField）时才把 order 下发给后端。
function serverOrderParam() {
  const f = parseOrder(order.value)
  if (!f) return null
  const serverFields = columns.filter(c => c.sortField).map(c => c.sortField)
  return serverFields.includes(f.field) ? order.value : null
}

async function fetchData() {
  loading.value = true
  try {
    const serverOrder = serverOrderParam()
    const result = await getProjects(serverOrder ? { order: serverOrder } : undefined)
    const data = result.data || result
    tableData.value = enrichData(data.data || data || [])
    await nextTick()
    updateCharts()
  } catch {
    tableData.value = []
  } finally {
    loading.value = false
  }
}

function onSortChange(payload) {
  order.value = payload.order
  updateUrl()
  // 仅服务端列需要重新取数；客户端列本地排序，无需请求。
  if (payload.server) {
    fetchData()
  }
}

function handleRowClick(row) {
  router.push('/project/' + row.project_id)
}

function makeBarOption(title, categories, seriesList) {
  return {
    title: { text: title, left: 'center', textStyle: { fontSize: 13, fontWeight: 'bold' } },
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
    legend: { data: seriesList.map(s => s.name), top: '8%', type: 'scroll' },
    grid: { left: '5%', right: '5%', top: '22%', bottom: '15%', containLabel: true },
    xAxis: { type: 'category', data: categories, axisLabel: { rotate: 30, fontSize: 11, overflow: 'truncate', width: 80 } },
    yAxis: { type: 'value' },
    series: seriesList.map(s => ({
      name: s.name,
      type: 'bar',
      data: s.data,
    })),
  }
}

function makeBarOptionPct(title, categories, seriesList) {
  return {
    ...makeBarOption(title, categories, seriesList),
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' },
      formatter(params) {
        let str = params[0].axisValue + '<br/>'
        params.forEach(p => { str += p.marker + p.seriesName + ': ' + (p.value ?? 0).toFixed(1) + '%<br/>' })
        return str
      }
    },
    yAxis: { type: 'value', axisLabel: { formatter: '{value}%' } },
  }
}

function updateCharts() {
  if (collapsed.value.charts) return
  const data = filteredData.value
  if (!data.length) return

  const names = data.map(r => r.name)

  // a) 效率图：提效比
  setEffOption(makeBarOptionPct('提效比（按项目）', names, [
    {
      name: '提效比',
      data: data.map(r => r.efficiency_ratio ?? null),
    },
  ]))

  // b) 代码量图：生成代码量 + 实际人天代码量
  setCodeOption(makeBarOption('代码量（按项目）', names, [
    {
      name: '生成代码量（行）',
      data: data.map(r => r.total_code_lines || 0),
    },
    {
      name: '实际人天代码量（行/人天）',
      data: data.map(r => r.actual_lines_per_day != null ? Math.round(r.actual_lines_per_day) : null),
    },
  ]))

  // c) 时间图：传统开发预估 + 实际耗时 + 项目周期（转换为天）
  const toDay = m => m != null ? Math.round(m / 480 * 10) / 10 : null
  setTimeOption({
    ...makeBarOption('时间对比（人天，按项目）', names, [
      { name: '传统开发预估', data: data.map(r => toDay(r.project_ancient_minutes_manual ?? r.project_ancient_minutes)) },
      { name: '实际耗时', data: data.map(r => toDay(r.project_real_process_minutes_manual ?? r.project_real_process_minutes)) },
      { name: '项目周期', data: data.map(r => toDay(r.project_real_lead_minutes_manual ?? r.project_real_lead_minutes)) },
    ]),
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' },
      formatter(params) {
        let str = params[0].axisValue + '<br/>'
        params.forEach(p => { str += p.marker + p.seriesName + ': ' + (p.value ?? '-') + ' 人天<br/>' })
        return str
      }
    },
  })

  // d) 人员规模图：人数 + Task数 + Commit数（暂无，用Repo数代替）
  setPeopleOption(makeBarOption('人员与规模（按项目）', names, [
    { name: '人数', data: data.map(r => r.user_count || 0) },
    { name: 'Task数', data: data.map(r => r.task_count || 0) },
    { name: 'Repo数', data: data.map(r => r.repo_count || 0) },
  ]))

  // e) 费用图
  setCostOption({
    ...makeBarOption('费用（按项目）', names, [
      { name: '费用（元）', data: data.map(r => r.cost || 0) },
    ]),
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' },
      formatter(params) {
        let str = params[0].axisValue + '<br/>'
        params.forEach(p => { str += p.marker + p.seriesName + ': ' + Number(p.value || 0).toFixed(2) + ' 元<br/>' })
        return str
      }
    },
  })
}

async function handleCreate() {
  if (!createForm.value.name.trim()) {
    ElMessage.warning('请输入项目名称')
    return
  }
  creating.value = true
  try {
    await createProject({
      name: createForm.value.name.trim(),
      description: (createForm.value.description || '').trim(),
    })
    ElMessage.success('创建成功')
    showCreateDialog.value = false
    createForm.value = { name: '', description: '' }
    await fetchData()
  } catch (e) {
    ElMessage.error('创建失败: ' + (e.response?.data?.error || e.message))
  } finally {
    creating.value = false
  }
}

async function handleDelete(row) {
  try {
    await ElMessageBox.confirm(`确定要删除项目「${row.name}」吗？`, '确认删除', { type: 'warning' })
    await deleteProject(row.project_id)
    ElMessage.success('删除成功')
    await fetchData()
  } catch (e) {
    if (e !== 'cancel') ElMessage.error('删除失败: ' + (e.response?.data?.error || e.message || e))
  }
}

watch(() => route.query, async () => {
  if (_ignoreRouteWatch) return
  const prevOrder = order.value
  syncUrlToControls()
  if (order.value !== prevOrder) {
    await fetchData()
    return
  }
  await nextTick()
  updateCharts()
}, { deep: true })

watch(filteredData, () => {
  nextTick(() => updateCharts())
})

onMounted(() => {
  loadCollapsed()
  syncUrlToControls()
  fetchData()
})
</script>
