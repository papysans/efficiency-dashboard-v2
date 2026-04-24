<template>
  <div class="kb-panel">
    <!-- title bar -->
    <el-card shadow="never">
      <div style="display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 8px">
        <div style="display: flex; align-items: center; gap: 8px; flex-wrap: wrap">
          <el-button :icon="ArrowLeft" @click="router.back()">返回</el-button>
          <span style="font-size: 16px; font-weight: bold; white-space: nowrap">组织详情</span>
          <!-- 级联组织选择器，与 URL orgPath 双向映射 -->
          <el-select v-model="filterOrg1" placeholder="一级组织" clearable size="small"
            style="width: 130px" @change="onOrg1Change">
            <el-option v-for="o in org1Options" :key="o" :label="o" :value="o" />
          </el-select>
          <el-select v-model="filterOrg2" placeholder="二级组织" clearable size="small"
            style="width: 130px" :disabled="!filterOrg1" @change="onOrg2Change">
            <el-option v-for="o in org2Options" :key="o" :label="o" :value="o" />
          </el-select>
          <el-select v-model="filterOrg3" placeholder="三级组织" clearable size="small"
            style="width: 130px" :disabled="!filterOrg2" @change="onOrg3Change">
            <el-option v-for="o in org3Options" :key="o" :label="o" :value="o" />
          </el-select>
          <el-select v-model="filterOrg4" placeholder="四级组织" clearable size="small"
            style="width: 130px" :disabled="!filterOrg3" @change="onOrg4Change">
            <el-option v-for="o in org4Options" :key="o" :label="o" :value="o" />
          </el-select>
        </div>
        <div style="display: flex; align-items: center; gap: 8px">
          <DateRangePicker v-model="dateRange" @change="onDateChange" size="small" />
          <el-select v-model="granularity" size="small" style="width: 90px" @change="fetchData">
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
          <div class="kb-metric-label">成员数</div>
          <div class="kb-metric-value">{{ summary?.user_count ?? '-' }}</div>
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
          <div class="kb-metric-label">Commit代码量</div>
          <div class="kb-metric-value">{{ summary?.commit_diff_lines ?? '-' }}</div>
        </el-card>
      </el-col>
      <el-col :span="4" style="margin-bottom: 12px">
        <el-card shadow="never" class="kb-metric-card">
          <div class="kb-metric-label">Task提效比</div>
          <div class="kb-metric-value">
            <el-tag v-if="summary?.task_efficiency_ratio > 0"
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
            <el-tag v-if="summary?.commit_efficiency_ratio > 0"
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

    <!-- members table -->
    <el-card v-if="!collapsed.members" shadow="never" class="kb-table-card" style="margin-top: 12px">
      <template #header>
        <div style="display:flex;align-items:center;justify-content:space-between">
          <span>用户列表</span>
          <el-button link size="small" @click="toggleSection('members')">折叠</el-button>
        </div>
      </template>
      <el-table :data="members" stripe border v-loading="loading" style="width: 100%" empty-text="暂无数据"
        :default-sort="{ prop: 'task_diff_lines', order: 'descending' }">
        <el-table-column label="用户名" min-width="110" fixed>
          <template #default="{ row }">
            <el-link type="primary" @click="goUser(row.user_id)">{{ row.user_name || row.user_id }}</el-link>
          </template>
        </el-table-column>
        <el-table-column prop="commit_diff_lines" label="Commit代码量" width="120" align="right" sortable />
        <el-table-column label="Commit实际耗时" width="130" align="right" sortable prop="commit_real_minutes">
          <template #default="{ row }">{{ formatDuration(row.commit_real_minutes) }}</template>
        </el-table-column>
        <el-table-column label="Commit提效比" width="120" align="center" sortable prop="commit_efficiency_ratio">
          <template #default="{ row }">
            <el-tag v-if="row.commit_efficiency_ratio > 0"
              :type="row.commit_efficiency_ratio >= 300 ? 'success' : row.commit_efficiency_ratio >= 150 ? 'primary' : 'info'"
              size="small">{{ row.commit_efficiency_ratio.toFixed(1) }}%</el-tag>
            <el-tag v-else type="info" size="small">-</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="task_diff_lines" label="Task代码量" width="110" align="right" sortable />
        <el-table-column label="Task实际耗时" width="120" align="right" sortable prop="task_real_minutes">
          <template #default="{ row }">{{ formatDuration(row.task_real_minutes) }}</template>
        </el-table-column>
        <el-table-column label="Task提效比" width="110" align="center" sortable prop="task_efficiency_ratio">
          <template #default="{ row }">
            <el-tag v-if="row.task_efficiency_ratio > 0"
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

    <!-- commits table -->
    <el-card v-if="!collapsed.commits" shadow="never" class="kb-table-card" style="margin-top: 12px">
      <template #header>
        <div style="display:flex;align-items:center;justify-content:space-between">
          <span>Commits 列表</span>
          <el-button link size="small" @click="toggleSection('commits')">折叠</el-button>
        </div>
      </template>
      <el-table :data="commits" stripe border v-loading="loading" style="width: 100%" empty-text="暂无数据">
        <el-table-column label="时间" min-width="140" sortable prop="period_label">
          <template #default="{ row }">{{ row.period_label }}</template>
        </el-table-column>
        <el-table-column label="Task数" width="80" align="right" sortable prop="task_count">
          <template #default="{ row }">{{ row.task_count ?? 0 }}</template>
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
            <el-tag v-if="row.commit_efficiency_ratio > 0"
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
    <el-card v-if="!collapsed.tasks" shadow="never" class="kb-table-card" style="margin-top: 12px">
      <template #header>
        <div style="display:flex;align-items:center;justify-content:space-between">
          <span>Tasks 列表</span>
          <el-button link size="small" @click="toggleSection('tasks')">折叠</el-button>
        </div>
      </template>
      <el-table :data="tasks" stripe border v-loading="loading" style="width: 100%" empty-text="暂无数据">
        <el-table-column label="时间" min-width="140" sortable prop="period_label">
          <template #default="{ row }">{{ row.period_label }}</template>
        </el-table-column>
        <el-table-column label="Commit数" width="90" align="right" sortable prop="commit_count">
          <template #default="{ row }">{{ row.commit_count ?? 0 }}</template>
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
            <el-tag v-if="row.task_efficiency_ratio > 0"
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

    <CollapsedTagBar :tags="collapsedSections" @expand="toggleSection" />

    <!-- charts -->
    <el-card v-if="!collapsed.charts && (commits.length > 0 || tasks.length > 0)" shadow="never" style="margin-top: 12px">
      <template #header>
        <div style="display:flex;align-items:center;justify-content:space-between">
          <span>图表</span>
          <el-button link size="small" @click="toggleSection('charts')">折叠</el-button>
        </div>
      </template>
      <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 12px">
        <div><div ref="chart1Ref" style="height:280px"></div></div>
        <div><div ref="chart2Ref" style="height:280px"></div></div>
        <div><div ref="chart3Ref" style="height:280px"></div></div>
        <div><div ref="chart4Ref" style="height:280px"></div></div>
      </div>
      <div style="margin-top: 12px"><div ref="chart5Ref" style="height:280px"></div></div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, nextTick, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft } from '@element-plus/icons-vue'
import DateRangePicker from '@/components/DateRangePicker.vue'
import CollapsedTagBar from '@/components/CollapsedTagBar.vue'
import { useChart } from '@/composables/useChart'
import { useCollapse } from '@/composables/useCollapse'
import { getOrgDetailV2, getOrgV2 } from '@/api/es'
import { fmtCost, formatDuration } from '@/utils/formatters'
import { getDefaultDateRangeWide } from '@/utils/date'

const route = useRoute()
const router = useRouter()

const { collapsed, collapsedTags: collapsedSections, toggle: toggleSection, load: loadCollapsed } = useCollapse(
  'org_detail_collapsed',
  { members: '用户列表', commits: 'Commits', tasks: 'Tasks', charts: '图表' },
  (key) => { if (key === 'charts') updateCharts() }
)

const loading = ref(false)
const summary = ref(null)
const commits = ref([])
const tasks = ref([])
const members = ref([])
const dateRange = ref(getDefaultDateRangeWide())
const granularity = ref('day')

// 级联组织选择器状态
const filterOrg1 = ref('')
const filterOrg2 = ref('')
const filterOrg3 = ref('')
const filterOrg4 = ref('')
const org1Options = ref([])
const org2Options = ref([])
const org3Options = ref([])
const org4Options = ref([])

// 当前有效的 orgPath（由选中的各级 org 拼接而成）
const currentOrgPath = computed(() => {
  return [filterOrg1.value, filterOrg2.value, filterOrg3.value, filterOrg4.value]
    .filter(Boolean).join('/')
})

// 用于标题展示的最末级名称
const orgName = computed(() => {
  const parts = currentOrgPath.value.split('/').filter(Boolean)
  return parts[parts.length - 1] || ''
})

// ---- 级联加载 ----
async function loadOrgOptions(level, parent) {
  try {
    const params = { level, parent: parent || '' }
    const result = await getOrgV2(params)
    const data = result.data || result
    return (data.data || []).map(d => d.org_name)
  } catch {
    return []
  }
}

async function onOrg1Change(val) {
  filterOrg2.value = ''; filterOrg3.value = ''; filterOrg4.value = ''
  org2Options.value = []; org3Options.value = []; org4Options.value = []
  if (val) org2Options.value = await loadOrgOptions('org2', val)
  syncUrlAndFetch()
}

async function onOrg2Change(val) {
  filterOrg3.value = ''; filterOrg4.value = ''
  org3Options.value = []; org4Options.value = []
  if (val) org3Options.value = await loadOrgOptions('org3', filterOrg1.value + '/' + val)
  syncUrlAndFetch()
}

async function onOrg3Change(val) {
  filterOrg4.value = ''
  org4Options.value = []
  if (val) org4Options.value = await loadOrgOptions('org4', filterOrg1.value + '/' + filterOrg2.value + '/' + val)
  syncUrlAndFetch()
}

function onOrg4Change() {
  syncUrlAndFetch()
}

function onDateChange() {
  syncUrlAndFetch()
}

// 将当前选中的 orgPath 同步到 URL，并触发数据刷新
function syncUrlAndFetch() {
  const newOrgPath = currentOrgPath.value
  const query = {}
  if (dateRange.value && dateRange.value.length === 2) {
    query.startDate = dateRange.value[0].replace(/-/g, '')
    query.endDate = dateRange.value[1].replace(/-/g, '')
  }
  if (newOrgPath) {
    router.replace({
      params: { orgPath: encodeURIComponent(newOrgPath) },
      query,
    })
  }
  fetchData()
}

// 从 URL orgPath 反向初始化各级选择器（依次加载下级选项）
async function initFromUrl() {
  const raw = decodeURIComponent(route.params.orgPath || '')
  const parts = raw.split('/').filter(Boolean)

  // 始终加载一级
  org1Options.value = await loadOrgOptions('org1', '')

  if (parts.length >= 1) {
    filterOrg1.value = parts[0]
    org2Options.value = await loadOrgOptions('org2', parts[0])
  }
  if (parts.length >= 2) {
    filterOrg2.value = parts[1]
    org3Options.value = await loadOrgOptions('org3', parts.slice(0, 2).join('/'))
  }
  if (parts.length >= 3) {
    filterOrg3.value = parts[2]
    org4Options.value = await loadOrgOptions('org4', parts.slice(0, 3).join('/'))
  }
  if (parts.length >= 4) {
    filterOrg4.value = parts[3]
  }
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

function fmtTokens(up, down) {
  const total = (up || 0) + (down || 0)
  if (total === 0) return '-'
  if (total >= 1000000) return (total / 1000000).toFixed(1) + 'M'
  if (total >= 1000) return (total / 1000).toFixed(1) + 'K'
  return String(total)
}

function formatOrgPath(row) {
  return [row.org1, row.org2, row.org3, row.org4].filter(Boolean).join(' / ')
}

function goUser(userId) {
  const start = dateRange.value?.[0]?.replace(/-/g, '') || ''
  const end = dateRange.value?.[1]?.replace(/-/g, '') || ''
  router.push({ path: `/user/${userId}`, query: { startDate: start, endDate: end } })
}

function getPeriodLabels(data) {
  return data.map(d => d.period_label)
}

function updateChart1() {
  const cData = commits.value
  if (cData.length === 0) return
  const labels = getPeriodLabels(cData)
  setChart1Option({
    title: { text: 'Task数 & Commit数', left: 'center', textStyle: { fontSize: 14, fontWeight: 'bold' } },
    tooltip: { trigger: 'axis' },
    legend: { data: ['Task数', 'Commit数'], top: '8%' },
    grid: { left: '5%', right: '5%', top: '20%', bottom: '10%', containLabel: true },
    xAxis: { type: 'category', data: labels, axisLabel: { rotate: 45, fontSize: 11 } },
    yAxis: { type: 'value' },
    series: [
      { name: 'Task数', type: 'bar', data: tasks.value.map(d => d.task_count || 0), itemStyle: { color: '#409EFF' } },
      { name: 'Commit数', type: 'bar', data: cData.map(d => d.commit_count || 0), itemStyle: { color: '#67C23A' } },
    ]
  })
}

function updateChart2() {
  const cData = commits.value
  if (cData.length === 0) return
  const labels = getPeriodLabels(cData)
  setChart2Option({
    title: { text: '代码行数', left: 'center', textStyle: { fontSize: 14, fontWeight: 'bold' } },
    tooltip: { trigger: 'axis' },
    legend: { data: ['Task代码行数', 'Commit代码行数'], top: '8%' },
    grid: { left: '5%', right: '5%', top: '20%', bottom: '10%', containLabel: true },
    xAxis: { type: 'category', data: labels, axisLabel: { rotate: 45, fontSize: 11 } },
    yAxis: { type: 'value' },
    series: [
      { name: 'Task代码行数', type: 'bar', data: tasks.value.map(d => d.task_diff_lines || 0), itemStyle: { color: '#409EFF' } },
      { name: 'Commit代码行数', type: 'bar', data: cData.map(d => d.commit_diff_lines || 0), itemStyle: { color: '#67C23A' } },
    ]
  })
}

function updateChart3() {
  const cData = commits.value
  const tData = tasks.value
  if (cData.length === 0 && tData.length === 0) return
  const labels = getPeriodLabels(cData.length > 0 ? cData : tData)
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
    legend: { data: ['Task传统耗时', 'Commit传统耗时', 'Task实际耗时', 'Commit实际耗时'], top: '8%' },
    grid: { left: '5%', right: '5%', top: '25%', bottom: '10%', containLabel: true },
    xAxis: { type: 'category', data: labels, axisLabel: { rotate: 45, fontSize: 11 } },
    yAxis: { type: 'value' },
    series: [
      {
        name: 'Task传统耗时',
        type: 'bar',
        stack: 'ancient',
        data: tData.map(d => d.task_ancient_minutes || 0),
        itemStyle: { color: '#409EFF' }
      },
      {
        name: 'Commit传统耗时',
        type: 'bar',
        stack: 'ancient',
        data: cData.map(d => d.commit_ancient_minutes || 0),
        itemStyle: { color: '#67C23A' }
      },
      {
        name: 'Task实际耗时',
        type: 'bar',
        stack: 'real',
        data: tData.map(d => d.task_real_minutes || 0),
        itemStyle: { color: '#a0cfff' }
      },
      {
        name: 'Commit实际耗时',
        type: 'bar',
        stack: 'real',
        data: cData.map(d => d.commit_real_minutes || 0),
        itemStyle: { color: '#b3e19d' }
      },
    ]
  })
}

function updateChart4() {
  const cData = commits.value
  if (cData.length === 0) return
  const labels = getPeriodLabels(cData)
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
      { name: '费用', type: 'bar', data: cData.map(d => d.cost || 0), itemStyle: { color: '#E6A23C' } },
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

async function fetchData() {
  if (!dateRange.value || dateRange.value.length !== 2) return
  const orgPath = currentOrgPath.value
  if (!orgPath) return

  loading.value = true
  try {
    const params = {
      org_path: orgPath,
      startDate: dateRange.value[0].replace(/-/g, ''),
      endDate: dateRange.value[1].replace(/-/g, ''),
      granularity: granularity.value,
    }
    const result = await getOrgDetailV2(params)
    const data = result.data || result
    summary.value = data.summary || null
    commits.value = data.commits || []
    tasks.value = data.tasks || []
    members.value = data.members || []

    await nextTick()
    updateCharts()
  } catch {
    summary.value = null
    commits.value = []
    tasks.value = []
    members.value = []
  } finally {
    loading.value = false
  }
}

watch(() => route.query, async (query) => {
  const { startDate, endDate } = query
  if (startDate && endDate) {
    const fmt = s => s.slice(0, 4) + '-' + s.slice(4, 6) + '-' + s.slice(6, 8)
    dateRange.value = [fmt(startDate), fmt(endDate)]
    fetchData()
  }
}, { deep: true })

onMounted(async () => {
  loadCollapsed()
  const { startDate, endDate, org1, org2, org3, org4 } = route.query
  if (startDate && endDate) {
    const fmt = s => s.slice(0, 4) + '-' + s.slice(4, 6) + '-' + s.slice(6, 8)
    dateRange.value = [fmt(startDate), fmt(endDate)]
  }
  if (org1 || org2 || org3 || org4) {
    org1Options.value = await loadOrgOptions('org1', '')
    if (org1) {
      filterOrg1.value = String(org1)
      org2Options.value = await loadOrgOptions('org2', String(org1))
    }
    if (org2) {
      filterOrg2.value = String(org2)
      org3Options.value = await loadOrgOptions('org3', [org1, org2].filter(Boolean).join('/'))
    }
    if (org3) {
      filterOrg3.value = String(org3)
      org4Options.value = await loadOrgOptions('org4', [org1, org2, org3].filter(Boolean).join('/'))
    }
    if (org4) {
      filterOrg4.value = String(org4)
    }
    fetchData()
  } else {
    await initFromUrl()
    fetchData()
  }
})
</script>

<style scoped>
</style>
