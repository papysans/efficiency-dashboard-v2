<template>
  <div class="kanban-native">
    <div class="kn-page">
      <header class="kn-header">
        <div class="kn-title-row">
          <div>
            <h1 class="kn-title">组织看板</h1>
            <p class="kn-subtitle">按组织聚合 v2 需求提效（数据源 user_productivity_v2）。</p>
          </div>
        </div>
        <div class="kn-controls">
          <DateRangePicker v-model="dateRange" :clearable="false" />
          <el-button type="primary" :icon="Search" :loading="loading" @click="applyFilters">查询</el-button>
        </div>
      </header>

      <div v-if="noOrgMapping" class="kn-note">
        ⚠ 当前数据集缺少完整的用户↔组织映射（user_org 多数为空），未映射用户已归入「未分组」。组织维度仅供参考。
      </div>

      <section class="kn-metrics kn-metrics-4">
        <MetricCard label="组织数" :value="rows.length" accent="var(--native-primary)" />
        <MetricCard label="覆盖用户" :value="stats.users" accent="var(--native-success)" />
        <MetricCard label="合并需求总数" :value="stats.merged" accent="#8a4cf6" />
        <MetricCard label="整体日历提效" accent="var(--native-info)">
          <RatioPill :value="stats.overallRatio" />
        </MetricCard>
      </section>

      <section class="kn-panel">
        <div class="kn-panel-head"><span>组织列表</span></div>
        <div class="kn-table-wrap" v-loading="loading">
          <table class="kn-table">
            <thead>
              <tr>
                <th><SortableTh field="org_name" label="组织" :active="sortField === 'org_name'" :desc="sortDesc" @sort="onSort('org_name')" /></th>
                <th class="kn-num"><SortableTh field="user_count" label="用户数" numeric :active="sortField === 'user_count'" :desc="sortDesc" @sort="onSort('user_count')" /></th>
                <th class="kn-num"><SortableTh field="merged_need_count" label="合并需求" numeric :active="sortField === 'merged_need_count'" :desc="sortDesc" @sort="onSort('merged_need_count')" /></th>
                <th class="kn-num"><SortableTh field="actual_calendar_min" label="实际日历" numeric :active="sortField === 'actual_calendar_min'" :desc="sortDesc" @sort="onSort('actual_calendar_min')" /></th>
                <th class="kn-num"><SortableTh field="baseline_calendar_min" label="基线日历" numeric :active="sortField === 'baseline_calendar_min'" :desc="sortDesc" @sort="onSort('baseline_calendar_min')" /></th>
                <th><SortableTh field="calendar_ratio" label="日历提效" :active="sortField === 'calendar_ratio'" :desc="sortDesc" @sort="onSort('calendar_ratio')" /></th>
                <th><SortableTh field="work_ratio" label="工作量提效" :active="sortField === 'work_ratio'" :desc="sortDesc" @sort="onSort('work_ratio')" /></th>
                <th class="kn-num"><SortableTh field="commit_count" label="Commit" numeric :active="sortField === 'commit_count'" :desc="sortDesc" @sort="onSort('commit_count')" /></th>
                <th class="kn-num"><SortableTh field="commit_diff_lines" label="代码行" numeric :active="sortField === 'commit_diff_lines'" :desc="sortDesc" @sort="onSort('commit_diff_lines')" /></th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="!rows.length && !loading"><td colspan="9"><div class="kn-empty">暂无组织数据</div></td></tr>
              <tr v-for="row in sortedRows" :key="row.org_name">
                <td>{{ row.org_name }}</td>
                <td class="kn-num">{{ row.user_count }}</td>
                <td class="kn-num">{{ row.merged_need_count }}</td>
                <td class="kn-num">{{ formatDuration(row.actual_calendar_min) }}</td>
                <td class="kn-num">{{ formatDuration(row.baseline_calendar_min) }}</td>
                <td><RatioPill :value="row.calendar_ratio" /></td>
                <td><RatioPill :value="row.work_ratio" /></td>
                <td class="kn-num">{{ row.commit_count }}</td>
                <td class="kn-num">{{ formatNumber(row.commit_diff_lines, 0) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup>
defineOptions({ name: 'OrgViewV2' })
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Search } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import DateRangePicker from '@/components/DateRangePicker.vue'
import MetricCard from '@/components/native/MetricCard.vue'
import RatioPill from '@/components/native/RatioPill.vue'
import SortableTh from '@/components/native/SortableTh.vue'
import { getOrgV2 } from '@/api/es'
import { formatDuration, formatNumber } from '@/utils/formatters'
import { formatDateParam, getDefaultDateRangeWide } from '@/utils/date'
import { parseOrder, toOrder, sortRows } from '@/utils/sort'

const route = useRoute()
const router = useRouter()

const loading = ref(false)
const rows = ref([])
const noOrgMapping = ref(false)
const dateRange = ref(getDefaultDateRangeWide())

// 客户端排序：org native handler 不消费 order，全量数据已在前端，就地排序。
const order = ref('')
const parsedOrder = computed(() => parseOrder(order.value))
const sortField = computed(() => parsedOrder.value?.field || '')
const sortDesc = computed(() => parsedOrder.value?.desc || false)

// field → 行取值函数；数值列返回 number，文本列返回字符串。sortRows 保证 null/缺值恒沉底 + 稳定。
const NUMERIC_FIELDS = new Set([
  'user_count', 'merged_need_count', 'actual_calendar_min', 'baseline_calendar_min',
  'calendar_ratio', 'work_ratio', 'commit_count', 'commit_diff_lines',
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
  if (!p) return rows.value
  return sortRows(rows.value, getterFor(p.field), p.desc)
})

// 列头三态循环：无→升→降→无。把 order 合并进 URL。
function onSort(field) {
  const p = parsedOrder.value
  let next
  if (!p || p.field !== field) next = toOrder(field, false)
  else if (!p.desc) next = toOrder(field, true)
  else next = undefined
  order.value = next || ''
  const q = { ...route.query }
  if (order.value) q.order = order.value
  else delete q.order
  router.replace({ query: q })
}

const stats = computed(() => {
  const actual = rows.value.reduce((s, r) => s + (r.actual_calendar_min || 0), 0)
  const baseline = rows.value.reduce((s, r) => s + (r.baseline_calendar_min || 0), 0)
  return {
    users: rows.value.reduce((s, r) => s + (r.user_count || 0), 0),
    merged: rows.value.reduce((s, r) => s + (r.merged_need_count || 0), 0),
    overallRatio: actual > 0 ? (baseline - actual) / actual : null,
  }
})

function normalizeDateQuery(value) {
  if (!value) return ''
  const s = String(value).trim()
  if (/^\d{8}$/.test(s)) return `${s.slice(0, 4)}-${s.slice(4, 6)}-${s.slice(6, 8)}`
  if (/^\d{4}-\d{2}-\d{2}$/.test(s)) return s
  return ''
}

async function fetchData() {
  loading.value = true
  try {
    const [start, end] = dateRange.value
    const res = await getOrgV2({ startDate: formatDateParam(start), endDate: formatDateParam(end) })
    const data = res.data || res
    rows.value = data.data || []
    noOrgMapping.value = !!data.no_org_mapping
  } catch (err) {
    ElMessage.error(err?.response?.data?.error || err?.message || '获取组织列表失败')
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

onMounted(() => {
  const start = normalizeDateQuery(route.query.startDate)
  const end = normalizeDateQuery(route.query.endDate)
  dateRange.value = start && end ? [start, end] : getDefaultDateRangeWide()
  order.value = typeof route.query.order === 'string' ? route.query.order : ''
  fetchData()
})
</script>

<style scoped>
.kn-note {
  border: 1px solid color-mix(in oklab, var(--native-warning) 40%, transparent);
  background: var(--native-warning-soft);
  color: oklch(46.4% 0.085 62);
  border-radius: var(--native-radius-md);
  padding: 0.7rem 1rem;
  font-size: 0.85rem;
}
</style>
