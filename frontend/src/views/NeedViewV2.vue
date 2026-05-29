<template>
  <div class="kanban-native">
    <div class="kn-page">
      <!-- header -->
      <header class="kn-header">
        <div class="kn-title-row">
          <div>
            <h1 class="kn-title">需求 Need 提效</h1>
            <p class="kn-subtitle">按需求边界度量提效比，日历提效为最终业务口径，工作量提效用于诊断。</p>
          </div>
        </div>
        <div class="kn-controls">
          <DateRangePicker v-model="dateRange" :clearable="false" />
          <el-input v-model="filters.repoAddr" clearable placeholder="仓库地址" style="width: 200px" @keyup.enter="applyFilters" />
          <el-input v-model="filters.repoBranch" clearable placeholder="分支" style="width: 160px" @keyup.enter="applyFilters" />
          <el-input v-model="filters.userId" clearable placeholder="用户 ID" style="width: 150px" @keyup.enter="applyFilters" />
          <el-select v-model="filters.status" clearable placeholder="状态" style="width: 120px">
            <el-option label="merged" value="merged" />
          </el-select>
          <el-select v-model="filters.boundarySource" clearable placeholder="边界来源" style="width: 140px">
            <el-option label="commit" value="commit" />
            <el-option label="branch" value="branch" />
            <el-option label="session" value="session" />
            <el-option label="manual" value="manual" />
          </el-select>
          <el-select v-model="filters.boundaryConfidence" clearable placeholder="边界置信" style="width: 130px">
            <el-option label="high" value="high" />
            <el-option label="medium" value="medium" />
            <el-option label="low" value="low" />
            <el-option label="very_low" value="very_low" />
          </el-select>
          <el-select v-model="filters.confidenceLevel" clearable placeholder="效率置信" style="width: 130px">
            <el-option label="high" value="high" />
            <el-option label="medium" value="medium" />
            <el-option label="low" value="low" />
          </el-select>
          <el-checkbox v-model="filters.outlierOnly">仅异常</el-checkbox>
          <el-checkbox v-model="filters.includeAll" title="放开看板口径：显示 active 未交付 + 主干分支 + 全部需求">显示全部</el-checkbox>
          <el-button type="primary" :icon="Search" :loading="loading" @click="applyFilters">查询</el-button>
          <el-button :icon="Refresh" @click="resetFilters">重置</el-button>
        </div>
      </header>

      <!-- summary metrics (当前结果集) -->
      <section class="kn-metrics kn-metrics-4">
        <MetricCard label="Need 总数" :value="total" accent="var(--native-primary)" />
        <MetricCard label="本页可计入" :value="pageStats.eligible" accent="var(--native-success)" />
        <MetricCard label="本页异常" :value="pageStats.outliers" accent="var(--native-error)" />
        <MetricCard label="本页日历提效中位" accent="var(--native-info)">
          <RatioPill :value="pageStats.medianRatio" />
        </MetricCard>
      </section>

      <!-- table -->
      <section class="kn-panel">
        <div class="kn-panel-head">
          <span>Need 列表</span>
          <span class="kn-panel-hint">提效比按百分比展示（小数口径 ×100）</span>
        </div>
        <div class="kn-table-wrap" v-loading="loading">
          <table class="kn-table">
            <thead>
              <tr>
                <th>Need ID</th>
                <th>日历提效</th>
                <th>工作量提效</th>
                <th>状态</th>
                <th>边界来源</th>
                <th>边界置信</th>
                <th>仓库</th>
                <th>分支</th>
                <th>主用户</th>
                <th class="kn-num">实际日历</th>
                <th class="kn-num">基线日历</th>
                <th>日历区间</th>
                <th class="kn-num">实际工作量</th>
                <th class="kn-num">基线工作量</th>
                <th class="kn-num">思考</th>
                <th class="kn-num">执行</th>
                <th class="kn-num">验证</th>
                <th>效率置信</th>
                <th>质量</th>
                <th>说明</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="!tableData.length && !loading">
                <td colspan="20"><div class="kn-empty">暂无 Need 数据</div></td>
              </tr>
              <tr
                v-for="row in tableData"
                :key="row.need_id"
                :class="'is-clickable'"
                @click="goToDetail(row)"
              >
                <td>
                  <button class="kn-link" @click.stop="goToDetail(row)">{{ shortNeedId(row.need_id) }}</button>
                </td>
                <td><RatioPill :value="row.efficiency_ratio" /></td>
                <td><RatioPill :value="row.work_efficiency_ratio" /></td>
                <td><span class="kn-tag" :class="statusTagClass(row.status)">{{ row.status || '-' }}</span></td>
                <td>{{ row.boundary_source || '-' }}</td>
                <td><span class="kn-tag" :class="confidenceTagClass(row.boundary_confidence)">{{ row.boundary_confidence || '-' }}</span></td>
                <td><div class="kn-ellipsis" :title="row.repo_addr">{{ row.repo_addr || '-' }}</div></td>
                <td><div class="kn-ellipsis" :title="row.repo_branch">{{ row.repo_branch || '-' }}</div></td>
                <td><div class="kn-ellipsis" :title="row.primary_user_id">{{ row.primary_user_id || '-' }}</div></td>
                <td class="kn-num">{{ formatDuration(row.total_calendar_min) }}</td>
                <td class="kn-num">{{ formatDuration(row.baseline_calendar_min) }}</td>
                <td class="kn-num">{{ formatBand(row) }}</td>
                <td class="kn-num">{{ formatDuration(row.total_active_work_corrected_min) }}</td>
                <td class="kn-num">{{ formatDuration(row.baseline_fused_work_min) }}</td>
                <td class="kn-num">{{ formatDuration(row.total_think_min) }}</td>
                <td class="kn-num">{{ formatDuration(row.total_exec_min) }}</td>
                <td class="kn-num">{{ formatDuration(row.total_verify_min) }}</td>
                <td><span class="kn-tag" :class="confidenceTagClass(row.confidence_level)">{{ row.confidence_level || '-' }}</span></td>
                <td>
                  <span v-if="row.outlier_flag" class="kn-tag kn-tag--error">异常</span>
                  <span v-else-if="row.coverage_eligible" class="kn-tag kn-tag--success">可计入</span>
                  <span v-else class="kn-tag kn-tag--neutral">未计入</span>
                </td>
                <td><div class="kn-ellipsis" :title="reasonHints(row.reason)">{{ reasonSummary(row.reason) }}</div></td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="kn-pagination">
          <el-pagination
            v-model:current-page="page"
            v-model:page-size="pageSize"
            :total="total"
            :page-sizes="[20, 50, 100, 200]"
            layout="total, sizes, prev, pager, next"
            background
            @size-change="handleSizeChange"
            @current-change="handlePageChange"
          />
        </div>
      </section>
    </div>
  </div>
</template>

<script setup>
defineOptions({ name: 'NeedViewV2' })
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Search, Refresh } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import DateRangePicker from '@/components/DateRangePicker.vue'
import MetricCard from '@/components/native/MetricCard.vue'
import RatioPill from '@/components/native/RatioPill.vue'
import { getNeedsV2 } from '@/api/es'
import { formatDuration, formatV2Ratio } from '@/utils/formatters'
import { reasonSummary, reasonHints } from '@/utils/reasonText'
import { formatDateParam, getDefaultDateRangeWide } from '@/utils/date'

const route = useRoute()
const router = useRouter()

const loading = ref(false)
const tableData = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const dateRange = ref(getDefaultDateRangeWide())
const filters = reactive({
  repoAddr: '',
  repoBranch: '',
  userId: '',
  status: '',
  boundarySource: '',
  boundaryConfidence: '',
  confidenceLevel: '',
  outlierOnly: false,
  includeAll: false,
})

let ignoreRouteWatch = false

const pageStats = computed(() => {
  const rows = tableData.value
  const ratios = rows
    .map(r => Number(r.efficiency_ratio))
    .filter(n => Number.isFinite(n))
    .sort((a, b) => a - b)
  const median = ratios.length ? ratios[Math.floor((ratios.length - 1) / 2)] : null
  return {
    eligible: rows.filter(r => r.coverage_eligible).length,
    outliers: rows.filter(r => r.outlier_flag).length,
    medianRatio: median,
  }
})

function normalizeDateQuery(value) {
  if (!value) return ''
  const s = String(value).trim()
  if (/^\d{8}$/.test(s)) return `${s.slice(0, 4)}-${s.slice(4, 6)}-${s.slice(6, 8)}`
  if (/^\d{4}-\d{2}-\d{2}$/.test(s)) return s
  return ''
}

function syncFromRoute() {
  const q = route.query
  const start = normalizeDateQuery(q.startDate)
  const end = normalizeDateQuery(q.endDate)
  dateRange.value = start && end ? [start, end] : getDefaultDateRangeWide()
  page.value = Number(q.page) > 0 ? Number(q.page) : 1
  pageSize.value = Number(q.pageSize) > 0 ? Math.min(Number(q.pageSize), 200) : 20
  filters.repoAddr = q.repoAddr ? String(q.repoAddr).trim() : ''
  filters.repoBranch = q.repoBranch ? String(q.repoBranch).trim() : ''
  filters.userId = q.userId ? String(q.userId).trim() : ''
  filters.status = q.status ? String(q.status).trim() : ''
  filters.boundarySource = q.boundarySource ? String(q.boundarySource).trim() : ''
  filters.boundaryConfidence = q.boundaryConfidence ? String(q.boundaryConfidence).trim() : ''
  filters.confidenceLevel = q.confidenceLevel ? String(q.confidenceLevel).trim() : ''
  filters.outlierOnly = q.outlierOnly === 'true'
  filters.includeAll = q.includeAll === 'true'
}

function buildQuery() {
  const [start, end] = dateRange.value
  const query = {
    startDate: formatDateParam(start),
    endDate: formatDateParam(end),
  }
  if (page.value !== 1) query.page = String(page.value)
  if (pageSize.value !== 20) query.pageSize = String(pageSize.value)
  Object.entries(filters).forEach(([key, value]) => {
    if (key === 'outlierOnly' || key === 'includeAll') {
      if (value) query[key] = 'true'
      return
    }
    const trimmed = String(value || '').trim()
    if (trimmed) query[key] = trimmed
  })
  return query
}

function buildParams() {
  return { ...buildQuery(), page: page.value, pageSize: pageSize.value }
}

async function updateUrl() {
  ignoreRouteWatch = true
  try {
    await router.replace({ query: buildQuery() })
  } finally {
    ignoreRouteWatch = false
  }
}

async function fetchData() {
  loading.value = true
  try {
    const res = await getNeedsV2(buildParams())
    const data = res.data || res
    tableData.value = data.data || []
    total.value = data.total || 0
  } catch (err) {
    ElMessage.error(err?.response?.data?.error || err?.message || '获取 Need 列表失败')
  } finally {
    loading.value = false
  }
}

async function applyFilters() {
  page.value = 1
  await updateUrl()
  fetchData()
}

async function resetFilters() {
  dateRange.value = getDefaultDateRangeWide()
  page.value = 1
  pageSize.value = 20
  Object.keys(filters).forEach(key => {
    filters[key] = (key === 'outlierOnly' || key === 'includeAll') ? false : ''
  })
  await updateUrl()
  fetchData()
}

async function handleSizeChange() {
  page.value = 1
  await updateUrl()
  fetchData()
}

async function handlePageChange() {
  await updateUrl()
  fetchData()
}

function goToDetail(row) {
  if (!row?.need_id) return
  router.push({ path: `/needs/${encodeURIComponent(row.need_id)}`, query: buildQuery() })
}

function shortNeedId(value) {
  if (!value) return '-'
  const s = String(value)
  return s.length > 18 ? `${s.slice(0, 18)}…` : s
}

function statusTagClass(status) {
  if (status === 'merged') return 'kn-tag--success'
  if (status === 'active') return 'kn-tag--primary'
  return 'kn-tag--neutral'
}

function confidenceTagClass(level) {
  if (level === 'high') return 'kn-tag--success'
  if (level === 'medium') return 'kn-tag--warning'
  if (level === 'low') return 'kn-tag--info'
  if (level === 'very_low') return 'kn-tag--error'
  return 'kn-tag--neutral'
}

function formatBand(row) {
  if (row.efficiency_band_low == null && row.efficiency_band_high == null) return '-'
  return `${formatV2Ratio(row.efficiency_band_low)} ~ ${formatV2Ratio(row.efficiency_band_high)}`
}

watch(
  () => route.query,
  () => {
    if (ignoreRouteWatch) return
    syncFromRoute()
    fetchData()
  },
)

onMounted(() => {
  syncFromRoute()
  fetchData()
})
</script>
