<template>
  <div class="kb-panel">
    <!-- title bar -->
    <el-card shadow="never">
      <div style="display: flex; align-items: center; justify-content: space-between">
        <div style="display: flex; align-items: center; gap: 12px">
          <el-button :icon="ArrowLeft" @click="router.back()">返回</el-button>
          <span style="font-size: 18px; font-weight: bold">组织详情: {{ orgTitle }}</span>
        </div>
        <DateRangePicker v-model="dateRange" @change="fetchData" size="small" />
      </div>
    </el-card>

    <!-- summary metric cards -->
    <el-row :gutter="12" v-loading="loading" style="margin-top: 12px">
      <el-col :span="4" style="margin-bottom: 12px">
        <el-card shadow="never" class="kb-metric-card">
          <div class="kb-metric-label">Commit代码量</div>
          <div class="kb-metric-value">{{ summary?.commit_diff_lines ?? '-' }}</div>
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
          <div class="kb-metric-label">Task代码量</div>
          <div class="kb-metric-value">{{ summary?.task_diff_lines ?? '-' }}</div>
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
          <div class="kb-metric-label">Token消耗</div>
          <div class="kb-metric-value">{{ summary ? ((summary.upstream_tokens || 0) + (summary.downstream_tokens || 0)).toLocaleString() : '-' }}</div>
        </el-card>
      </el-col>
      <el-col :span="4" style="margin-bottom: 12px">
        <el-card shadow="never" class="kb-metric-card">
          <div class="kb-metric-label">总费用</div>
          <div class="kb-metric-value">{{ summary?.cost != null ? fmtCostVal(summary.cost) : '-' }}</div>
        </el-card>
      </el-col>
    </el-row>

    <!-- charts -->
    <template v-if="daily.length > 0">
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

    <!-- members table -->
    <el-card shadow="never" class="kb-table-card" style="margin-top: 12px">
      <template #header><span>成员列表</span></template>
      <el-table :data="members" stripe border v-loading="loading" style="width: 100%" empty-text="暂无数据">
        <el-table-column label="用户名" min-width="120">
          <template #default="{ row }">
            <el-link type="primary" @click="router.push('/user/' + row.user_id)">{{ row.user_name || row.user_id }}</el-link>
          </template>
        </el-table-column>
        <el-table-column prop="commit_diff_lines" label="Commit代码量" min-width="120" align="right" />
        <el-table-column label="Commit提效比" min-width="120" align="center">
          <template #default="{ row }">
            <el-tag v-if="row.commit_efficiency_ratio != null"
              :type="row.commit_efficiency_ratio >= 300 ? 'success' : row.commit_efficiency_ratio >= 150 ? 'primary' : 'info'"
              size="small">{{ row.commit_efficiency_ratio.toFixed(1) }}%</el-tag>
            <el-tag v-else type="info" size="small">-</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="task_diff_lines" label="Task代码量" min-width="110" align="right" />
        <el-table-column label="Task提效比" min-width="110" align="center">
          <template #default="{ row }">
            <el-tag v-if="row.task_efficiency_ratio != null"
              :type="row.task_efficiency_ratio >= 300 ? 'success' : row.task_efficiency_ratio >= 150 ? 'primary' : 'info'"
              size="small">{{ row.task_efficiency_ratio.toFixed(1) }}%</el-tag>
            <el-tag v-else type="info" size="small">-</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="费用" min-width="90" align="right">
          <template #default="{ row }">{{ fmtCostVal(row.cost) }}</template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft } from '@element-plus/icons-vue'
import DateRangePicker from '@/components/DateRangePicker.vue'
import { useChart } from '@/composables/useChart'
import { getGroupDetail } from '@/api/es'
import { fmtCost, formatDuration } from '@/utils/formatters'
import { getDefaultDateRangeWide } from '@/utils/date'

const route = useRoute()
const router = useRouter()

const loading = ref(false)
const summary = ref(null)
const daily = ref([])
const members = ref([])
const dateRange = ref(getDefaultDateRangeWide())

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

// 从 route.query 读取 org1/org2/org3/org4，拼接标题
const orgTitle = computed(() => {
  const parts = [route.query.org1, route.query.org2, route.query.org3, route.query.org4].filter(v => v)
  return parts.join(' / ') || '未知组织'
})

function fmtCostVal(val) {
  return fmtCost(null, null, val)
}

function updateChart1() {
  const data = daily.value
  if (data.length === 0) return
  const sorted = [...data].sort((a, b) => (a.date || '').localeCompare(b.date || ''))
  const dates = sorted.map(d => d.date)
  setChart1Option({
    title: { text: 'Task数 & Commit数', left: 'center', textStyle: { fontSize: 14, fontWeight: 'bold' } },
    tooltip: { trigger: 'axis' },
    legend: { data: ['Task数', 'Commit数'], top: '8%' },
    grid: { left: '5%', right: '5%', top: '20%', bottom: '10%', containLabel: true },
    xAxis: { type: 'category', data: dates, axisLabel: { rotate: 45, fontSize: 11 } },
    yAxis: { type: 'value' },
    series: [
      { name: 'Task数', type: 'bar', data: sorted.map(d => d.task_count ?? 0), itemStyle: { color: '#409EFF' } },
      { name: 'Commit数', type: 'bar', data: sorted.map(d => d.commit_count ?? 0), itemStyle: { color: '#67C23A' } },
    ]
  })
}

function updateChart2() {
  const data = daily.value
  if (data.length === 0) return
  const sorted = [...data].sort((a, b) => (a.date || '').localeCompare(b.date || ''))
  const dates = sorted.map(d => d.date)
  setChart2Option({
    title: { text: '代码行数', left: 'center', textStyle: { fontSize: 14, fontWeight: 'bold' } },
    tooltip: { trigger: 'axis' },
    legend: { data: ['Task代码行数', 'Commit代码行数'], top: '8%' },
    grid: { left: '5%', right: '5%', top: '20%', bottom: '10%', containLabel: true },
    xAxis: { type: 'category', data: dates, axisLabel: { rotate: 45, fontSize: 11 } },
    yAxis: { type: 'value' },
    series: [
      { name: 'Task代码行数', type: 'bar', data: sorted.map(d => d.task_diff_lines || 0), itemStyle: { color: '#409EFF' } },
      { name: 'Commit代码行数', type: 'bar', data: sorted.map(d => d.commit_diff_lines || 0), itemStyle: { color: '#67C23A' } },
    ]
  })
}

function updateChart3() {
  const data = daily.value
  if (data.length === 0) return
  const sorted = [...data].sort((a, b) => (a.date || '').localeCompare(b.date || ''))
  const dates = sorted.map(d => d.date)
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
    xAxis: { type: 'category', data: dates, axisLabel: { rotate: 45, fontSize: 11 } },
    yAxis: { type: 'value' },
    series: [
      { name: 'Task传统耗时', type: 'bar', data: sorted.map(d => d.task_ancient_minutes || 0), itemStyle: { color: '#a0cfff' } },
      { name: 'Task实际耗时', type: 'bar', data: sorted.map(d => d.task_real_minutes || 0), itemStyle: { color: '#409EFF' } },
      { name: 'Commit传统耗时', type: 'bar', data: sorted.map(d => d.commit_ancient_minutes || 0), itemStyle: { color: '#b3e19d' } },
      { name: 'Commit实际耗时', type: 'bar', data: sorted.map(d => d.commit_real_minutes || 0), itemStyle: { color: '#67C23A' } },
    ]
  })
}

function updateChart4() {
  const data = daily.value
  if (data.length === 0) return
  const sorted = [...data].sort((a, b) => (a.date || '').localeCompare(b.date || ''))
  const dates = sorted.map(d => d.date)
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
    xAxis: { type: 'category', data: dates, axisLabel: { rotate: 45, fontSize: 11 } },
    yAxis: { type: 'value' },
    series: [
      { name: '费用', type: 'bar', data: sorted.map(d => d.cost || 0), itemStyle: { color: '#E6A23C' } },
    ]
  })
}

function updateChart5() {
  const data = daily.value
  if (data.length === 0) return
  const sorted = [...data].sort((a, b) => (a.date || '').localeCompare(b.date || ''))
  const dates = sorted.map(d => d.date)
  setChart5Option({
    title: { text: '提效比趋势', left: 'center', textStyle: { fontSize: 14, fontWeight: 'bold' } },
    tooltip: { trigger: 'axis' },
    legend: { data: ['Task提效比', 'Commit提效比'], top: '8%' },
    grid: { left: '5%', right: '5%', top: '20%', bottom: '10%', containLabel: true },
    xAxis: { type: 'category', data: dates, axisLabel: { rotate: 45, fontSize: 11 } },
    yAxis: { type: 'value', axisLabel: { formatter: '{value}%' } },
    series: [
      { name: 'Task提效比', type: 'line', data: sorted.map(d => d.task_efficiency_ratio || 0), smooth: true, itemStyle: { color: '#409EFF' } },
      { name: 'Commit提效比', type: 'line', data: sorted.map(d => d.commit_efficiency_ratio || 0), smooth: true, itemStyle: { color: '#67C23A' } },
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

async function fetchData() {
  if (!dateRange.value || dateRange.value.length !== 2) return
  loading.value = true
  try {
    const params = {
      startDate: dateRange.value[0].replace(/-/g, ''),
      endDate: dateRange.value[1].replace(/-/g, ''),
    }
    if (route.query.org1) params.org1 = route.query.org1
    if (route.query.org2) params.org2 = route.query.org2
    if (route.query.org3) params.org3 = route.query.org3
    if (route.query.org4) params.org4 = route.query.org4

    const result = await getGroupDetail(params)
    const data = result.data || result
    summary.value = data.summary || null
    daily.value = data.daily || []
    members.value = data.members || []

    await nextTick()
    updateCharts()
  } catch {
    summary.value = null
    daily.value = []
    members.value = []
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  const { startDate, endDate } = route.query
  if (startDate && endDate) {
    const fmt = s => s.slice(0, 4) + '-' + s.slice(4, 6) + '-' + s.slice(6, 8)
    dateRange.value = [fmt(startDate), fmt(endDate)]
  }
  fetchData()
})
</script>
