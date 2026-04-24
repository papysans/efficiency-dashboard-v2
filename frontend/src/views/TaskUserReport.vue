<template>
  <div class="kb-panel">
    <!-- 筛选区 -->
    <FilterBar
      v-model:dateRange="dateRange"
      :show-org="true"
      :org-value="orgValue"
      @change="onFilterChange"
    >
      <el-button :icon="ArrowLeft" @click="router.back()">返回</el-button>
    </FilterBar>

    <el-empty v-if="!loading && users.length === 0" description="暂无数据" />

    <template v-if="users.length > 0">
      <!-- 汇总指标卡 -->
      <el-row :gutter="12" style="margin-top: 12px">
        <el-col :span="4">
          <el-card shadow="never" class="kb-metric-card">
            <div class="kb-metric-label">总Task数</div>
            <div class="kb-metric-value">
              <el-link type="primary" @click="goToTaskList">{{ totalTaskCount }}</el-link>
            </div>
          </el-card>
        </el-col>
        <el-col :span="4">
          <el-card shadow="never" class="kb-metric-card">
            <div class="kb-metric-label">总Commit数</div>
            <div class="kb-metric-value">
              <el-link type="primary" @click="goToCommitList">{{ totalCommitCount }}</el-link>
            </div>
          </el-card>
        </el-col>
        <el-col :span="4">
          <el-card shadow="never" class="kb-metric-card">
            <div class="kb-metric-label">总代码行数</div>
            <div class="kb-metric-value">{{ totalDiffLines.toLocaleString() }}</div>
          </el-card>
        </el-col>
        <el-col :span="4">
          <el-card shadow="never" class="kb-metric-card">
            <div class="kb-metric-label">总传统耗时</div>
            <div class="kb-metric-value">{{ formatDuration(totalAncientMinutes) }}</div>
          </el-card>
        </el-col>
        <el-col :span="4">
          <el-card shadow="never" class="kb-metric-card">
            <div class="kb-metric-label">总实际耗时</div>
            <div class="kb-metric-value">{{ formatDuration(totalRealMinutes) }}</div>
          </el-card>
        </el-col>
        <el-col :span="4">
          <el-card shadow="never" class="kb-metric-card">
            <div class="kb-metric-label">总费用</div>
            <div class="kb-metric-value">{{ totalCost.toFixed(4) }}</div>
          </el-card>
        </el-col>
      </el-row>

      <!-- 6个图表 -->
      <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 12px; margin-top: 12px">
        <el-card shadow="never"><div ref="chart1Ref" style="height:360px"></div></el-card>
        <el-card shadow="never"><div ref="chart2Ref" style="height:360px"></div></el-card>
        <el-card shadow="never"><div ref="chart3Ref" style="height:360px"></div></el-card>
        <el-card shadow="never"><div ref="chart4Ref" style="height:360px"></div></el-card>
        <el-card shadow="never"><div ref="chart5Ref" style="height:360px"></div></el-card>
        <el-card shadow="never"><div ref="chart6Ref" style="height:360px"></div></el-card>
      </div>
    </template>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft } from '@element-plus/icons-vue'
import FilterBar from '@/components/FilterBar.vue'
import { getUsersV2 } from '@/api/es'
import { useChart } from '@/composables/useChart'
import { createBarOption, createDualBarOption } from '@/utils/chart'
import { formatDuration } from '@/utils/formatters'
import { getDefaultDateRangeWide } from '@/utils/date'

const route = useRoute()
const router = useRouter()

const loading = ref(false)
const users = ref([])
const dateRange = ref(getDefaultDateRangeWide())

// 组织筛选（由 FilterBar 内部管理，这里只记录用于传参）
const filterOrg1 = ref('')
const filterOrg2 = ref('')
const filterOrg3 = ref('')
const filterOrg4 = ref('')

// 传给 FilterBar 的初始 orgValue
const orgValue = computed(() => ({
  org1: filterOrg1.value,
  org2: filterOrg2.value,
  org3: filterOrg3.value,
  org4: filterOrg4.value,
}))

// 图表
const chart1Ref = ref(null)
const chart2Ref = ref(null)
const chart3Ref = ref(null)
const chart4Ref = ref(null)
const chart5Ref = ref(null)
const chart6Ref = ref(null)
const { setOption: setChart1 } = useChart(chart1Ref)
const { setOption: setChart2 } = useChart(chart2Ref)
const { setOption: setChart3 } = useChart(chart3Ref)
const { setOption: setChart4 } = useChart(chart4Ref)
const { setOption: setChart5 } = useChart(chart5Ref)
const { setOption: setChart6 } = useChart(chart6Ref)

// 汇总指标
const totalTaskCount = computed(() => users.value.reduce((s, u) => s + (u.task_count || 0), 0))
const totalCommitCount = computed(() => users.value.reduce((s, u) => s + (u.commit_count || 0), 0))
const totalDiffLines = computed(() => users.value.reduce((s, u) => s + (u.task_diff_lines || 0), 0))
const totalAncientMinutes = computed(() => users.value.reduce((s, u) => s + (u.task_ancient_minutes || 0), 0))
const totalRealMinutes = computed(() => users.value.reduce((s, u) => s + (u.task_real_minutes || 0), 0))
const totalCost = computed(() => users.value.reduce((s, u) => s + (u.cost || 0), 0))
const avgEfficiencyRatio = computed(() => {
  const real = totalRealMinutes.value
  const ancient = totalAncientMinutes.value
  return real > 0 ? (ancient / real) * 100 : 0
})

/** FilterBar @change 回调 */
function onFilterChange({ dateRange: dr, org1, org2, org3, org4 }) {
  dateRange.value = dr
  filterOrg1.value = org1 || ''
  filterOrg2.value = org2 || ''
  filterOrg3.value = org3 || ''
  filterOrg4.value = org4 || ''
  fetchData()
}

function buildOrgQuery() {
  const query = {}
  if (dateRange.value && dateRange.value.length === 2) {
    query.startDate = dateRange.value[0].replace(/-/g, '')
    query.endDate = dateRange.value[1].replace(/-/g, '')
  }
  if (filterOrg1.value) query.org1 = filterOrg1.value
  if (filterOrg2.value) query.org2 = filterOrg2.value
  if (filterOrg3.value) query.org3 = filterOrg3.value
  if (filterOrg4.value) query.org4 = filterOrg4.value
  return query
}

function goToTaskList() {
  router.push({ path: '/task-v2', query: buildOrgQuery() })
}

function goToCommitList() {
  router.push({ path: '/commit-v2', query: buildOrgQuery() })
}

async function fetchData() {
  if (!dateRange.value || dateRange.value.length !== 2) return
  loading.value = true
  try {
    const params = {
      startDate: dateRange.value[0].replace(/-/g, ''),
      endDate: dateRange.value[1].replace(/-/g, ''),
      pageSize: 9999,
    }
    if (filterOrg1.value) params.org1 = filterOrg1.value
    if (filterOrg2.value) params.org2 = filterOrg2.value
    if (filterOrg3.value) params.org3 = filterOrg3.value
    if (filterOrg4.value) params.org4 = filterOrg4.value

    const result = await getUsersV2(params)
    const data = result.data || result
    users.value = (data.data || []).filter(u => !u.is_virtual_group)

    await nextTick()
    if (users.value.length > 0) updateCharts()
  } catch {
    users.value = []
  } finally {
    loading.value = false
  }
}

function updateCharts() {
  const list = users.value

  const taskCountData = list.map(u => ({ name: u.user_name || u.user_id, value: u.task_count || 0 }))
  setChart1(createBarOption('Task数（按用户）', taskCountData, '#409EFF'))

  const diffLinesData = list.map(u => ({ name: u.user_name || u.user_id, value: u.task_diff_lines || 0 }))
  setChart2(createBarOption('代码行数（按用户）', diffLinesData, '#67C23A'))

  const ancientData = list.map(u => ({ name: u.user_name || u.user_id, value: u.task_ancient_minutes || 0 }))
  const realData = list.map(u => ({ name: u.user_name || u.user_id, value: u.task_real_minutes || 0 }))
  setChart3(createDualBarOption('耗时对比（按用户）', ancientData, realData, '传统耗时', '实际耗时', '#E6A23C', '#409EFF'))

  const costData = list.map(u => ({ name: u.user_name || u.user_id, value: u.cost || 0 }))
  setChart4(createBarOption('费用（按用户）', costData, '#E6A23C', v => Number(v).toFixed(4)))

  const tokenData = list.map(u => ({ name: u.user_name || u.user_id, value: (u.upstream_tokens || 0) + (u.downstream_tokens || 0) }))
  setChart5(createBarOption('Token消耗（按用户）', tokenData, '#909399', v => Number(v).toLocaleString()))

  const ratioData = list.map(u => ({ name: u.user_name || u.user_id, value: u.task_efficiency_ratio || 0 }))
  setChart6(createBarOption('Task提效比（按用户）', ratioData, '#67C23A', v => v.toFixed(1) + '%'))
}

onMounted(async () => {
  const { startDate, endDate, org1, org2, org3, org4 } = route.query
  if (startDate && endDate) {
    const fmt = s => s.slice(0, 4) + '-' + s.slice(4, 6) + '-' + s.slice(6, 8)
    dateRange.value = [fmt(startDate), fmt(endDate)]
  }
  if (org1) filterOrg1.value = org1
  if (org2) filterOrg2.value = org2
  if (org3) filterOrg3.value = org3
  if (org4) filterOrg4.value = org4
  await fetchData()
})
</script>
