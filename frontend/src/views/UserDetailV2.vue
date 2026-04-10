<template>
  <div class="kb-panel">
    <!-- title bar -->
    <el-card shadow="never">
      <div style="display: flex; align-items: center; justify-content: space-between">
        <div style="display: flex; align-items: center; gap: 12px">
          <el-button :icon="ArrowLeft" @click="router.back()">返回</el-button>
          <span style="font-size: 18px; font-weight: bold">用户详情:</span>
          <el-select
            v-model="currentUserId"
            filterable
            placeholder="选择用户"
            size="small"
            style="width: 180px"
            @change="onUserChange"
          >
            <el-option
              v-for="u in userList"
              :key="u.user_id"
              :label="u.user_name || u.user_id"
              :value="u.user_id"
            />
          </el-select>
        </div>
        <div style="display: flex; align-items: center; gap: 8px">
          <DateRangePicker v-model="dateRange" @change="onDateChange" size="small" />
          <el-select v-model="granularity" size="small" style="width: 90px" @change="onGranularityChange">
            <el-option label="天" value="day" />
            <el-option label="周" value="week" />
            <el-option label="月" value="month" />
            <el-option label="年" value="year" />
          </el-select>
        </div>
      </div>
    </el-card>

    <!-- summary metric cards -->
    <el-row :gutter="12" v-loading="loading" style="margin-top: 12px">
      <el-col :span="4" style="margin-bottom: 12px">
        <el-card shadow="never" class="kb-metric-card">
          <div class="kb-metric-label">总活跃天数</div>
          <div class="kb-metric-value">{{ summary?.day_count ?? '-' }}</div>
        </el-card>
      </el-col>
      <el-col :span="4" style="margin-bottom: 12px">
        <el-card shadow="never" class="kb-metric-card">
          <div class="kb-metric-label">总Task数</div>
          <div class="kb-metric-value">{{ summary?.task_count ?? '-' }}</div>
        </el-card>
      </el-col>
      <el-col :span="4" style="margin-bottom: 12px">
        <el-card shadow="never" class="kb-metric-card">
          <div class="kb-metric-label">总Commit数</div>
          <div class="kb-metric-value">{{ summary?.commit_count ?? '-' }}</div>
        </el-card>
      </el-col>
      <el-col :span="4" style="margin-bottom: 12px">
        <el-card shadow="never" class="kb-metric-card">
          <div class="kb-metric-label">Task提效比</div>
          <div class="kb-metric-value">
            <el-tag v-if="summary?.task_efficiency_ratio != null"
              :type="summary.task_efficiency_ratio >= 300 ? 'success' : summary.task_efficiency_ratio >= 150 ? 'primary' : 'info'"
              size="small">{{ summary.task_efficiency_ratio.toFixed(1) }}%</el-tag>
            <span v-else>-</span>
          </div>
        </el-card>
      </el-col>
      <el-col :span="4" style="margin-bottom: 12px">
        <el-card shadow="never" class="kb-metric-card">
          <div class="kb-metric-label">Commit提效比</div>
          <div class="kb-metric-value">
            <el-tag v-if="summary?.commit_efficiency_ratio != null"
              :type="summary.commit_efficiency_ratio >= 300 ? 'success' : summary.commit_efficiency_ratio >= 150 ? 'primary' : 'info'"
              size="small">{{ summary.commit_efficiency_ratio.toFixed(1) }}%</el-tag>
            <span v-else>-</span>
          </div>
        </el-card>
      </el-col>
      <el-col :span="4" style="margin-bottom: 12px">
        <el-card shadow="never" class="kb-metric-card">
          <div class="kb-metric-label">总费用</div>
          <div class="kb-metric-value">{{ summary?.cost != null ? fmtCostVal(summary.cost) : '-' }}</div>
        </el-card>
      </el-col>
    </el-row>

    <!-- commits table -->
    <el-card shadow="never" class="kb-table-card">
      <template #header><span>Commits 列表</span></template>
      <el-table :data="commits" stripe border v-loading="loading" style="width: 100%" empty-text="暂无数据">
        <el-table-column label="时间" min-width="140" sortable prop="period_label">
          <template #default="{ row }">{{ row.period_label }}</template>
        </el-table-column>
        <el-table-column label="Task数" width="90" align="right" sortable prop="task_count">
          <template #default="{ row }">
            <el-link v-if="row.task_count > 0" type="primary" @click="goToTaskList(row)">{{ row.task_count }}</el-link>
            <span v-else>0</span>
          </template>
        </el-table-column>
        <el-table-column label="代码量" width="90" align="right" sortable prop="commit_diff_lines">
          <template #default="{ row }">{{ row.commit_diff_lines ?? 0 }}</template>
        </el-table-column>
        <el-table-column label="实际耗时" width="110" align="right" sortable prop="commit_real_minutes">
          <template #default="{ row }">{{ formatDuration(row.commit_real_minutes) }}</template>
        </el-table-column>
        <el-table-column label="传统开发时长预估" width="150" align="right" sortable prop="commit_ancient_minutes">
          <template #default="{ row }">{{ formatDuration(row.commit_ancient_minutes) }}</template>
        </el-table-column>
        <el-table-column label="提效比" width="100" align="center" sortable prop="commit_efficiency_ratio">
          <template #default="{ row }">
            <el-tag v-if="row.commit_efficiency_ratio != null && row.commit_efficiency_ratio > 0"
              :type="row.commit_efficiency_ratio >= 300 ? 'success' : row.commit_efficiency_ratio >= 150 ? 'primary' : 'info'"
              size="small">{{ row.commit_efficiency_ratio.toFixed(1) }}%</el-tag>
            <el-tag v-else type="info" size="small">-</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="Tokens消耗" width="120" align="right" sortable :sort-method="(a, b) => ((a.upstream_tokens || 0) + (a.downstream_tokens || 0)) - ((b.upstream_tokens || 0) + (b.downstream_tokens || 0))">
          <template #default="{ row }">{{ fmtTokens(row.upstream_tokens, row.downstream_tokens) }}</template>
        </el-table-column>
        <el-table-column label="费用" width="100" align="right" sortable prop="cost">
          <template #default="{ row }">{{ fmtCostVal(row.cost) }}</template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- tasks table -->
    <el-card shadow="never" class="kb-table-card" style="margin-top: 12px">
      <template #header><span>Tasks 列表</span></template>
      <el-table :data="tasks" stripe border v-loading="loading" style="width: 100%" empty-text="暂无数据">
        <el-table-column label="时间" min-width="140" sortable prop="period_label">
          <template #default="{ row }">{{ row.period_label }}</template>
        </el-table-column>
        <el-table-column label="Commit数" width="90" align="right" sortable prop="commit_count">
          <template #default="{ row }">
            <el-link v-if="row.commit_count > 0" type="primary" @click="goToCommitList(row)">{{ row.commit_count }}</el-link>
            <span v-else>0</span>
          </template>
        </el-table-column>
        <el-table-column label="代码量" width="90" align="right" sortable prop="task_diff_lines">
          <template #default="{ row }">{{ row.task_diff_lines ?? 0 }}</template>
        </el-table-column>
        <el-table-column label="实际耗时" width="110" align="right" sortable prop="task_real_minutes">
          <template #default="{ row }">{{ formatDuration(row.task_real_minutes) }}</template>
        </el-table-column>
        <el-table-column label="传统开发时长预估" width="150" align="right" sortable prop="task_ancient_minutes">
          <template #default="{ row }">{{ formatDuration(row.task_ancient_minutes) }}</template>
        </el-table-column>
        <el-table-column label="提效比" width="100" align="center" sortable prop="task_efficiency_ratio">
          <template #default="{ row }">
            <el-tag v-if="row.task_efficiency_ratio != null && row.task_efficiency_ratio > 0"
              :type="row.task_efficiency_ratio >= 300 ? 'success' : row.task_efficiency_ratio >= 150 ? 'primary' : 'info'"
              size="small">{{ row.task_efficiency_ratio.toFixed(1) }}%</el-tag>
            <el-tag v-else type="info" size="small">-</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="Tokens消耗" width="120" align="right" sortable :sort-method="(a, b) => ((a.upstream_tokens || 0) + (a.downstream_tokens || 0)) - ((b.upstream_tokens || 0) + (b.downstream_tokens || 0))">
          <template #default="{ row }">{{ fmtTokens(row.upstream_tokens, row.downstream_tokens) }}</template>
        </el-table-column>
        <el-table-column label="费用" width="100" align="right" sortable prop="cost">
          <template #default="{ row }">{{ fmtCostVal(row.cost) }}</template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- charts -->
    <template v-if="commits.length > 0 || tasks.length > 0">
      <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 12px; margin-top: 12px">
        <el-card shadow="never"><div ref="chart1Ref" style="height:280px"></div></el-card>
        <el-card shadow="never"><div ref="chart2Ref" style="height:280px"></div></el-card>
        <el-card shadow="never"><div ref="chart3Ref" style="height:280px"></div></el-card>
        <el-card shadow="never"><div ref="chart4Ref" style="height:280px"></div></el-card>
      </div>
      <el-card shadow="never" style="margin-top: 12px">
        <div ref="chart5Ref" style="height:280px"></div>
      </el-card>
    </template>
  </div>
</template>

<script setup>
import { ref, onMounted, nextTick, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft } from '@element-plus/icons-vue'
import DateRangePicker from '@/components/DateRangePicker.vue'
import { useChart } from '@/composables/useChart'
import { getUserDetailV2, getUsersV2 } from '@/api/es'
import { fmtCost, formatDuration } from '@/utils/formatters'
import { getDefaultDateRangeWide } from '@/utils/date'

const route = useRoute()
const router = useRouter()

const loading = ref(false)
const summary = ref(null)
const commits = ref([])
const tasks = ref([])
const daily = ref([])
const dateRange = ref(getDefaultDateRangeWide())
const granularity = ref('day')
const userList = ref([])
const currentUserId = ref('')

let _ignoreRouteWatch = false

function syncUrlToControls() {
  const { startDate, endDate } = route.query
  if (startDate && endDate) {
    const fmt = s => s.slice(0, 4) + '-' + s.slice(4, 6) + '-' + s.slice(6, 8)
    dateRange.value = [fmt(startDate), fmt(endDate)]
  }
  currentUserId.value = route.params.userId || ''
}

async function loadUserList() {
  try {
    const result = await getUsersV2({ pageSize: 1000 })
    const data = result.data || result
    userList.value = (Array.isArray(data) ? data : data.data || []).filter(u => u.user_id)
  } catch {
    userList.value = []
  }
}

function onUserChange(userId) {
  _ignoreRouteWatch = true
  router.replace({
    params: { userId },
    query: route.query,
  }).finally(() => { _ignoreRouteWatch = false })
  fetchData()
}

function updateUrl() {
  if (!dateRange.value || dateRange.value.length !== 2) return
  const query = {
    startDate: dateRange.value[0].replace(/-/g, ''),
    endDate: dateRange.value[1].replace(/-/g, ''),
  }
  _ignoreRouteWatch = true
  router.replace({ query }).finally(() => { _ignoreRouteWatch = false })
}

const chart1Ref = ref(null)
const chart2Ref = ref(null)
const chart3Ref = ref(null)
const chart4Ref = ref(null)
const chart5Ref = ref(null)
const { setOption: setChart1Option } = useChart(chart1Ref)
const { setOption: setChart2Option } = useChart(chart2Ref)
const { setOption: setChart3Option } = useChart(chart3Ref)
const { setOption: setChart4Option } = useChart(chart4Ref)
const { setOption: setChart5Option } = useChart(chart5Ref)

function fmtCostVal(val) {
  return fmtCost(null, null, val)
}

function getPeriodDateRange(row) {
  const key = row.period_key || ''
  const g = granularity.value
  let start = '', end = ''
  if (g === 'day') {
    start = end = key
  } else if (g === 'week') {
    const m = key.match(/^(\d{4})-W(\d{2})$/)
    if (m) {
      const y = parseInt(m[1]), w = parseInt(m[2])
      const jan4 = new Date(y, 0, 4)
      const dayNum = jan4.getDay() || 7
      const monday = new Date(jan4)
      monday.setDate(jan4.getDate() - dayNum + 1 + (w - 1) * 7)
      const sunday = new Date(monday)
      sunday.setDate(monday.getDate() + 6)
      start = monday.toISOString().slice(0, 10)
      end = sunday.toISOString().slice(0, 10)
    }
  } else if (g === 'month') {
    start = key + '-01'
    const [y, mo] = key.split('-').map(Number)
    const lastDay = new Date(y, mo, 0).getDate()
    end = key + '-' + String(lastDay).padStart(2, '0')
  } else if (g === 'year') {
    start = key + '-01-01'
    end = key + '-12-31'
  }
  return { start, end }
}

function goToTaskList(row) {
  const { start, end } = getPeriodDateRange(row)
  const query = {}
  if (start && end) {
    query.startDate = start.replace(/-/g, '')
    query.endDate = end.replace(/-/g, '')
  }
  if (summary.value?.user_name) {
    query.userName = summary.value.user_name
  }
  router.push({ path: '/task-v2', query })
}

function goToCommitList(row) {
  const { start, end } = getPeriodDateRange(row)
  const query = {}
  if (start && end) {
    query.startDate = start.replace(/-/g, '')
    query.endDate = end.replace(/-/g, '')
  }
  if (summary.value?.user_name) {
    query.userName = summary.value.user_name
  }
  router.push({ path: '/commit-v2', query })
}

function fmtTokens(up, down) {
  const total = (up || 0) + (down || 0)
  if (total === 0) return '-'
  if (total >= 1000000) return (total / 1000000).toFixed(1) + 'M'
  if (total >= 1000) return (total / 1000).toFixed(1) + 'K'
  return String(total)
}

function getPeriodLabels(data) {
  return data.map(d => d.period_label)
}

function updateChart1() {
  const data = commits.value
  if (data.length === 0) return
  const labels = getPeriodLabels(data)
  setChart1Option({
    title: { text: 'Task数 & Commit数', left: 'center', textStyle: { fontSize: 14, fontWeight: 'bold' } },
    tooltip: { trigger: 'axis' },
    legend: { data: ['Task数', 'Commit数'], top: '8%' },
    grid: { left: '5%', right: '5%', top: '20%', bottom: '10%', containLabel: true },
    xAxis: { type: 'category', data: labels, axisLabel: { rotate: 45, fontSize: 11 } },
    yAxis: { type: 'value' },
    series: [
      { name: 'Task数', type: 'bar', data: tasks.value.map(d => d.task_count || 0), itemStyle: { color: '#409EFF' } },
      { name: 'Commit数', type: 'bar', data: data.map(d => d.commit_count || 0), itemStyle: { color: '#67C23A' } },
    ]
  })
}

function updateChart2() {
  const data = commits.value
  if (data.length === 0) return
  const labels = getPeriodLabels(data)
  setChart2Option({
    title: { text: '代码行数', left: 'center', textStyle: { fontSize: 14, fontWeight: 'bold' } },
    tooltip: { trigger: 'axis' },
    legend: { data: ['Task代码行数', 'Commit代码行数'], top: '8%' },
    grid: { left: '5%', right: '5%', top: '20%', bottom: '10%', containLabel: true },
    xAxis: { type: 'category', data: labels, axisLabel: { rotate: 45, fontSize: 11 } },
    yAxis: { type: 'value' },
    series: [
      { name: 'Task代码行数', type: 'bar', data: tasks.value.map(d => d.task_diff_lines || 0), itemStyle: { color: '#409EFF' } },
      { name: 'Commit代码行数', type: 'bar', data: data.map(d => d.commit_diff_lines || 0), itemStyle: { color: '#67C23A' } },
    ]
  })
}

function updateChart3() {
  const data = commits.value
  if (data.length === 0) return
  const labels = getPeriodLabels(data)
  setChart3Option({
    title: { text: '耗时对比', left: 'center', textStyle: { fontSize: 14, fontWeight: 'bold' } },
    tooltip: {
      trigger: 'axis',
      formatter(params) {
        let s = params[0].axisValue + '<br/>'
        params.forEach(p => { s += p.marker + p.seriesName + ': ' + formatDuration(p.value) + '<br/>' })
        return s
      }
    },
    legend: { data: ['Task传统耗时', 'Task实际耗时', 'Commit传统耗时', 'Commit实际耗时'], top: '8%' },
    grid: { left: '5%', right: '5%', top: '25%', bottom: '10%', containLabel: true },
    xAxis: { type: 'category', data: labels, axisLabel: { rotate: 45, fontSize: 11 } },
    yAxis: { type: 'value' },
    series: [
      { name: 'Task传统耗时', type: 'bar', data: tasks.value.map(d => d.task_ancient_minutes || 0), itemStyle: { color: '#a0cfff' } },
      { name: 'Task实际耗时', type: 'bar', data: tasks.value.map(d => d.task_real_minutes || 0), itemStyle: { color: '#409EFF' } },
      { name: 'Commit传统耗时', type: 'bar', data: data.map(d => d.commit_ancient_minutes || 0), itemStyle: { color: '#b3e19d' } },
      { name: 'Commit实际耗时', type: 'bar', data: data.map(d => d.commit_real_minutes || 0), itemStyle: { color: '#67C23A' } },
    ]
  })
}

function updateChart4() {
  const data = commits.value
  if (data.length === 0) return
  const labels = getPeriodLabels(data)
  setChart4Option({
    title: { text: '费用', left: 'center', textStyle: { fontSize: 14, fontWeight: 'bold' } },
    tooltip: {
      trigger: 'axis',
      formatter(params) {
        const val = params[0]?.value ?? 0
        return params[0].axisValue + '<br/>' + params[0].marker + '费用: ' + val.toFixed(2) + ' 元'
      }
    },
    grid: { left: '5%', right: '5%', top: '15%', bottom: '10%', containLabel: true },
    xAxis: { type: 'category', data: labels, axisLabel: { rotate: 45, fontSize: 11 } },
    yAxis: { type: 'value' },
    series: [
      { name: '费用', type: 'bar', data: data.map(d => d.cost || 0), itemStyle: { color: '#E6A23C' } },
    ]
  })
}

function updateChart5() {
  const cData = commits.value
  const tData = tasks.value
  if (cData.length === 0 && tData.length === 0) return
  const labels = getPeriodLabels(cData.length > 0 ? cData : tData)
  setChart5Option({
    title: { text: '提效比趋势', left: 'center', textStyle: { fontSize: 14, fontWeight: 'bold' } },
    tooltip: { trigger: 'axis' },
    legend: { data: ['Task提效比', 'Commit提效比'], top: '8%' },
    grid: { left: '5%', right: '5%', top: '20%', bottom: '10%', containLabel: true },
    xAxis: { type: 'category', data: labels, axisLabel: { rotate: 45, fontSize: 11 } },
    yAxis: { type: 'value', axisLabel: { formatter: '{value}%' } },
    series: [
      { name: 'Task提效比', type: 'line', data: tData.map(d => d.task_efficiency_ratio || 0), smooth: true, itemStyle: { color: '#409EFF' } },
      { name: 'Commit提效比', type: 'line', data: cData.map(d => d.commit_efficiency_ratio || 0), smooth: true, itemStyle: { color: '#67C23A' } },
    ]
  })
}

function updateCharts() {
  updateChart1()
  updateChart2()
  updateChart3()
  updateChart4()
  updateChart5()
}

function onDateChange() {
  updateUrl()
  fetchData()
}

function onGranularityChange() {
  fetchData()
}

async function fetchData() {
  const userId = currentUserId.value || route.params.userId
  if (!userId) return
  if (!dateRange.value || dateRange.value.length !== 2) return

  loading.value = true
  try {
    const params = {
      startDate: dateRange.value[0].replace(/-/g, ''),
      endDate: dateRange.value[1].replace(/-/g, ''),
      granularity: granularity.value,
    }
    const result = await getUserDetailV2(userId, params)
    const data = result.data || result
    summary.value = data.summary || null
    daily.value = data.daily || []
    commits.value = data.commits || []
    tasks.value = data.tasks || []

    await nextTick()
    updateCharts()
  } catch {
    summary.value = null
    daily.value = []
    commits.value = []
    tasks.value = []
  } finally {
    loading.value = false
  }
}

watch(() => route.query, async (query) => {
  if (_ignoreRouteWatch) return
  syncUrlToControls()
  await fetchData()
}, { deep: true })

onMounted(() => {
  syncUrlToControls()
  loadUserList()
  fetchData()
})
</script>
