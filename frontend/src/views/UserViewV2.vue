<template>
  <div class="kanban-native">
    <div class="kn-page">
      <header class="kn-header">
        <div class="kn-title-row">
          <div>
            <h1 class="kn-title">用户看板</h1>
            <p class="kn-subtitle">按用户聚合 v2 需求提效（数据源 user_productivity_v2，提效比为小数口径）。</p>
          </div>
        </div>
        <div class="kn-controls">
          <DateRangePicker v-model="dateRange" :clearable="false" />
          <el-input v-model="keyword" clearable placeholder="用户名/ID 过滤" style="width: 200px" />
          <el-button type="primary" :icon="Search" :loading="loading" @click="applyFilters">查询</el-button>
          <el-button :icon="Refresh" @click="resetFilters">重置</el-button>
        </div>
      </header>

      <section class="kn-metrics kn-metrics-4">
        <MetricCard label="用户数" :value="total" accent="var(--native-success)" />
        <MetricCard label="合并需求总数" :value="stats.merged" accent="var(--native-primary)" />
        <MetricCard label="Commit 总数" :value="stats.commits" accent="#8a4cf6" />
        <MetricCard label="日历提效中位" accent="var(--native-info)">
          <RatioPill :value="stats.medianRatio" />
        </MetricCard>
      </section>

      <section class="kn-panel">
        <div class="kn-panel-head">
          <span>用户列表</span>
          <span class="kn-panel-hint">点击用户查看周明细 / 需求 / 提交</span>
        </div>
        <div class="kn-table-wrap" v-loading="loading">
          <table class="kn-table">
            <thead>
              <tr>
                <th>用户</th>
                <th class="kn-num"><SortableTh field="merged_need_count" label="合并需求" numeric :active="sortField === 'merged_need_count'" :desc="sortDesc" @sort="onSort('merged_need_count')" /></th>
                <th class="kn-num"><SortableTh field="active_need_count" label="活跃" numeric :active="sortField === 'active_need_count'" :desc="sortDesc" @sort="onSort('active_need_count')" /></th>
                <th class="kn-num"><SortableTh field="abandoned_need_count" label="废弃" numeric :active="sortField === 'abandoned_need_count'" :desc="sortDesc" @sort="onSort('abandoned_need_count')" /></th>
                <th class="kn-num"><SortableTh field="actual_calendar_min" label="实际日历" numeric :active="sortField === 'actual_calendar_min'" :desc="sortDesc" @sort="onSort('actual_calendar_min')" /></th>
                <th class="kn-num"><SortableTh field="baseline_calendar_min" label="基线日历" numeric :active="sortField === 'baseline_calendar_min'" :desc="sortDesc" @sort="onSort('baseline_calendar_min')" /></th>
                <th><SortableTh field="calendar_ratio" label="日历提效" :active="sortField === 'calendar_ratio'" :desc="sortDesc" @sort="onSort('calendar_ratio')" /></th>
                <th class="kn-num"><SortableTh field="actual_work_min" label="实际工作量" numeric :active="sortField === 'actual_work_min'" :desc="sortDesc" @sort="onSort('actual_work_min')" /></th>
                <th><SortableTh field="work_ratio" label="工作量提效" :active="sortField === 'work_ratio'" :desc="sortDesc" @sort="onSort('work_ratio')" /></th>
                <th class="kn-num"><SortableTh field="commit_count" label="Commit" numeric :active="sortField === 'commit_count'" :desc="sortDesc" @sort="onSort('commit_count')" /></th>
                <th class="kn-num"><SortableTh field="commit_diff_lines" label="代码行" numeric :active="sortField === 'commit_diff_lines'" :desc="sortDesc" @sort="onSort('commit_diff_lines')" /></th>
                <th class="kn-num"><SortableTh field="week_count" label="活跃周" numeric :active="sortField === 'week_count'" :desc="sortDesc" @sort="onSort('week_count')" /></th>
                <th>置信</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="!filteredRows.length && !loading"><td colspan="13"><div class="kn-empty">暂无用户数据</div></td></tr>
              <tr v-for="row in pagedRows" :key="row.user_id" class="is-clickable" @click="goToDetail(row)">
                <td><button class="kn-link" @click.stop="goToDetail(row)">{{ shortName(row) }}</button></td>
                <td class="kn-num">{{ row.merged_need_count }}</td>
                <td class="kn-num">{{ row.active_need_count }}</td>
                <td class="kn-num">{{ row.abandoned_need_count }}</td>
                <td class="kn-num">{{ formatDuration(row.actual_calendar_min) }}</td>
                <td class="kn-num">{{ formatDuration(row.baseline_calendar_min) }}</td>
                <td><RatioPill :value="row.calendar_ratio" /></td>
                <td class="kn-num">{{ formatDuration(row.actual_work_min) }}</td>
                <td><RatioPill :value="row.work_ratio" /></td>
                <td class="kn-num">{{ row.commit_count }}</td>
                <td class="kn-num">{{ formatNumber(row.commit_diff_lines, 0) }}</td>
                <td class="kn-num">{{ row.week_count }}</td>
                <td>
                  <span v-if="row.confidence_limited" class="kn-tag kn-tag--warning" :title="confidenceReasonText(row.confidence_reason)">受限</span>
                  <span v-else class="kn-tag kn-tag--success">正常</span>
                  <div v-if="row.confidence_limited" class="kn-ellipsis kn-reason-hint" :title="confidenceReasonText(row.confidence_reason)">{{ confidenceReasonText(row.confidence_reason) }}</div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="kn-pagination">
          <el-pagination
            v-model:current-page="page"
            v-model:page-size="pageSize"
            :total="filteredRows.length"
            :page-sizes="[20, 50, 100]"
            layout="total, sizes, prev, pager, next"
            background
          />
        </div>
      </section>
    </div>
  </div>
</template>

<script setup>
defineOptions({ name: 'UserViewV2' })
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Search, Refresh } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import DateRangePicker from '@/components/DateRangePicker.vue'
import MetricCard from '@/components/native/MetricCard.vue'
import RatioPill from '@/components/native/RatioPill.vue'
import SortableTh from '@/components/native/SortableTh.vue'
import { getUsersV2 } from '@/api/es'
import { formatDuration, formatNumber } from '@/utils/formatters'
import { formatDateParam, getDefaultDateRangeWide } from '@/utils/date'
import { parseOrder, toOrder, sortRows } from '@/utils/sort'

const route = useRoute()
const router = useRouter()

const loading = ref(false)
const rows = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(50)
const keyword = ref('')
const dateRange = ref(getDefaultDateRangeWide())

const filteredRows = computed(() => {
  const kw = keyword.value.trim().toLowerCase()
  if (!kw) return rows.value
  return rows.value.filter(r => `${r.user_name || ''}${r.user_id || ''}`.toLowerCase().includes(kw))
})

// 客户端排序：order 走 URL（'field' 升 / '-field' 降 / 空 无），后端 native handler 不消费 order。
const order = ref('')
const parsedOrder = computed(() => parseOrder(order.value))
const sortField = computed(() => parsedOrder.value?.field || '')
const sortDesc = computed(() => parsedOrder.value?.desc || false)

// field → 行取值函数；数值列返回 number，文本列返回字符串。sortRows 保证 null/缺值恒沉底 + 稳定。
const NUMERIC_FIELDS = new Set([
  'merged_need_count', 'active_need_count', 'abandoned_need_count',
  'actual_calendar_min', 'baseline_calendar_min', 'calendar_ratio',
  'actual_work_min', 'work_ratio', 'commit_count', 'commit_diff_lines', 'week_count',
])
function getterFor(field) {
  if (NUMERIC_FIELDS.has(field)) {
    return (row) => {
      const v = row[field]
      if (v == null || v === '') return null
      const n = Number(v)
      return Number.isFinite(n) ? n : null
    }
  }
  return (row) => {
    const v = row[field]
    return v == null || v === '' ? null : String(v)
  }
}

const sortedRows = computed(() => {
  const p = parsedOrder.value
  if (!p) return filteredRows.value
  return sortRows(filteredRows.value, getterFor(p.field), p.desc)
})

const pagedRows = computed(() => {
  const start = (page.value - 1) * pageSize.value
  return sortedRows.value.slice(start, start + pageSize.value)
})

// 列头三态循环：无→升→降→无。排序时回到第一页，并把 order 合并进 URL。
function onSort(field) {
  const p = parsedOrder.value
  let next
  if (!p || p.field !== field) next = toOrder(field, false)
  else if (!p.desc) next = toOrder(field, true)
  else next = undefined
  order.value = next || ''
  page.value = 1
  router.replace({ query: buildQuery() })
}

function buildQuery() {
  const q = { ...route.query }
  if (order.value) q.order = order.value
  else delete q.order
  return q
}

const stats = computed(() => {
  const data = filteredRows.value
  const ratios = data.map(r => Number(r.calendar_ratio)).filter(n => Number.isFinite(n)).sort((a, b) => a - b)
  return {
    merged: data.reduce((s, r) => s + (r.merged_need_count || 0), 0),
    commits: data.reduce((s, r) => s + (r.commit_count || 0), 0),
    medianRatio: ratios.length ? ratios[Math.floor((ratios.length - 1) / 2)] : null,
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
  const start = normalizeDateQuery(route.query.startDate)
  const end = normalizeDateQuery(route.query.endDate)
  dateRange.value = start && end ? [start, end] : getDefaultDateRangeWide()
  order.value = typeof route.query.order === 'string' ? route.query.order : ''
}

async function fetchData() {
  loading.value = true
  try {
    const [start, end] = dateRange.value
    const res = await getUsersV2({ startDate: formatDateParam(start), endDate: formatDateParam(end), pageSize: 1000 })
    const data = res.data || res
    rows.value = data.data || []
    total.value = data.total || rows.value.length
    page.value = 1
  } catch (err) {
    ElMessage.error(err?.response?.data?.error || err?.message || '获取用户列表失败')
  } finally {
    loading.value = false
  }
}

async function applyFilters() {
  const [start, end] = dateRange.value
  const query = { startDate: formatDateParam(start), endDate: formatDateParam(end) }
  if (order.value) query.order = order.value
  await router.replace({ query })
  fetchData()
}

function resetFilters() {
  dateRange.value = getDefaultDateRangeWide()
  keyword.value = ''
  order.value = ''
  router.replace({ query: { ...route.query, order: undefined } })
  fetchData()
}

function goToDetail(row) {
  if (!row?.user_id) return
  const [start, end] = dateRange.value
  router.push({ path: `/user/${encodeURIComponent(row.user_id)}`, query: { startDate: formatDateParam(start), endDate: formatDateParam(end) } })
}

function shortName(row) {
  const name = row.user_name || row.user_id || '-'
  return name.length > 20 ? `${name.slice(0, 20)}…` : name
}

// 把 user_productivity_v2 的受限原因码翻译成可读文案，供"受限"tag 展示/悬浮。
function confidenceReasonText(reason) {
  if (!reason) return '受限：数据置信度不足'
  const rules = [
    [/no_eligible_baseline/, () => '无可计入需求（没有 merged + 高/中置信 + 可测日历 的需求）'],
    [/high_confidence_ratio=([0-9.]+)/, m => `高置信需求工作量占比过低（${(Number(m[1]) * 100).toFixed(1)}%）`],
    [/low_unreported_ratio=([0-9.]+)/, m => `低/未上报需求工作量占比过高（${(Number(m[1]) * 100).toFixed(1)}%）`],
  ]
  const parts = []
  reason.split(';').map(s => s.trim()).filter(Boolean).forEach(tok => {
    let hit = false
    for (const [re, label] of rules) {
      const m = tok.match(re)
      if (m) { parts.push(label(m)); hit = true; break }
    }
    if (!hit) parts.push(tok)
  })
  return '受限原因：' + parts.join('；')
}

watch(() => keyword.value, () => { page.value = 1 })

onMounted(() => {
  syncFromRoute()
  fetchData()
})
</script>

<style scoped>
.kn-reason-hint {
  max-width: 200px;
  margin-top: 2px;
  font-size: 0.72rem;
  line-height: 1.2;
  color: var(--native-muted);
}
</style>
