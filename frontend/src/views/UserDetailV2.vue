<template>
  <div class="kanban-native">
    <div class="kn-page" v-loading="loading">
      <header class="kn-header">
        <button class="kn-back" @click="router.back()"><span>&lt;</span><span>返回</span></button>
        <div class="kn-title-row">
          <div>
            <h1 class="kn-title">用户详情</h1>
            <p class="kn-subtitle is-mono">{{ summary.user_name || summary.user_id || userId }}</p>
          </div>
          <div class="kn-controls">
            <DateRangePicker v-model="dateRange" :clearable="false" @change="fetchData" />
          </div>
        </div>
      </header>

      <section class="kn-metrics">
        <MetricCard label="合并需求" :value="summary.merged_need_count ?? 0" accent="var(--native-primary)" />
        <MetricCard label="日历提效" accent="var(--native-success)">
          <RatioPill :value="summary.calendar_ratio" />
        </MetricCard>
        <MetricCard label="工作量提效" accent="var(--native-info)">
          <RatioPill :value="summary.work_ratio" />
        </MetricCard>
        <MetricCard label="实际日历" :value="formatDuration(summary.actual_calendar_min)" accent="var(--native-warning)" />
        <MetricCard label="基线日历" :value="formatDuration(summary.baseline_calendar_min)" accent="var(--native-warning)" />
        <MetricCard label="Commit / 代码行" :value="`${summary.commit_count ?? 0} / ${formatNumber(summary.commit_diff_lines, 0)}`" accent="#8a4cf6" />
      </section>

      <div class="kn-grid-2">
        <section class="kn-panel">
          <div class="kn-panel-head"><span>周明细</span><span class="kn-panel-hint">{{ weeks.length }} 周</span></div>
          <div class="kn-table-wrap">
            <table class="kn-table">
              <thead>
                <tr><th>周起始</th><th class="kn-num">合并</th><th class="kn-num">活跃</th><th>日历提效</th><th>工作量提效</th><th class="kn-num">Commit</th><th>置信</th></tr>
              </thead>
              <tbody>
                <tr v-if="!weeks.length"><td colspan="7"><div class="kn-empty">暂无周数据</div></td></tr>
                <tr v-for="w in weeks" :key="w.week_start">
                  <td>{{ fmtDate(w.week_start) }}</td>
                  <td class="kn-num">{{ w.merged_need_count }}</td>
                  <td class="kn-num">{{ w.active_need_count }}</td>
                  <td><RatioPill :value="w.efficiency_ratio" /></td>
                  <td><RatioPill :value="w.work_efficiency_ratio" /></td>
                  <td class="kn-num">{{ w.commit_count }}</td>
                  <td>
                    <span v-if="w.confidence_limited" class="kn-tag kn-tag--warning">受限</span>
                    <span v-else class="kn-tag kn-tag--success">正常</span>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>
        <NativeChart :option="weeklyChart" empty="暂无周趋势" />
      </div>

      <section class="kn-panel">
        <div class="kn-panel-head"><span>关联需求 Need</span><span class="kn-panel-hint">{{ needs.length }} 个</span></div>
        <div class="kn-table-wrap">
          <table class="kn-table">
            <thead>
              <tr><th>Need</th><th>状态</th><th>仓库</th><th>分支</th><th class="kn-num">实际日历</th><th>日历提效</th><th>工作量提效</th></tr>
            </thead>
            <tbody>
              <tr v-if="!needs.length"><td colspan="7"><div class="kn-empty">暂无 Need</div></td></tr>
              <tr v-for="n in needs" :key="n.need_id" class="is-clickable" @click="goNeed(n)">
                <td><button class="kn-link" @click.stop="goNeed(n)">{{ shortId(n.need_id, 16) }}</button></td>
                <td><span class="kn-tag" :class="statusClass(n.status)">{{ n.status || '-' }}</span></td>
                <td><div class="kn-ellipsis" :title="n.repo_addr">{{ n.repo_addr || '-' }}</div></td>
                <td><div class="kn-ellipsis" :title="n.repo_branch">{{ n.repo_branch || '-' }}</div></td>
                <td class="kn-num">{{ formatDuration(n.total_calendar_min) }}</td>
                <td><RatioPill :value="n.efficiency_ratio" /></td>
                <td><RatioPill :value="n.work_efficiency_ratio" /></td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section class="kn-panel">
        <div class="kn-panel-head"><span>最近 Commit</span><span class="kn-panel-hint">{{ commits.length }} 条</span></div>
        <div class="kn-table-wrap">
          <table class="kn-table">
            <thead>
              <tr><th>Commit</th><th>提交时间</th><th>仓库</th><th class="kn-num">代码行</th><th>说明</th></tr>
            </thead>
            <tbody>
              <tr v-if="!commits.length"><td colspan="5"><div class="kn-empty">暂无 Commit</div></td></tr>
              <tr v-for="c in commits" :key="c.commit_id">
                <td><button class="kn-link" @click="router.push('/commit/' + encodeURIComponent(c.commit_id))">{{ shortId(c.commit_id, 10) }}</button></td>
                <td>{{ formatLocalTime(c.commit_time) }}</td>
                <td><div class="kn-ellipsis" :title="c.repo_addr">{{ c.repo_addr || '-' }}</div></td>
                <td class="kn-num">{{ formatNumber(c.diff_lines, 0) }}</td>
                <td><div class="kn-ellipsis" :title="c.comment">{{ c.comment || '-' }}</div></td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup>
defineOptions({ name: 'UserDetailV2' })
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import DateRangePicker from '@/components/DateRangePicker.vue'
import MetricCard from '@/components/native/MetricCard.vue'
import RatioPill from '@/components/native/RatioPill.vue'
import NativeChart from '@/components/native/NativeChart.vue'
import { getUserDetailV2 } from '@/api/es'
import { formatDuration, formatLocalTime, formatNumber } from '@/utils/formatters'
import { formatDateParam, getDefaultDateRangeWide } from '@/utils/date'
import { kanbanChart } from '@/utils/kanbanChart'

const route = useRoute()
const router = useRouter()

const loading = ref(false)
const summary = ref({})
const weeks = ref([])
const needs = ref([])
const commits = ref([])
const dateRange = ref(getDefaultDateRangeWide())

const userId = computed(() => route.params.userId)

const weeklyChart = computed(() => {
  if (!weeks.value.length) return null
  const ordered = [...weeks.value].sort((a, b) => String(a.week_start).localeCompare(String(b.week_start)))
  const labels = ordered.map(w => fmtDate(w.week_start))
  return kanbanChart('周日历提效（%）', labels, [
    { name: '日历提效', type: 'line', data: ordered.map(w => Number(((w.efficiency_ratio ?? 0) * 100).toFixed(1))) },
    { name: '合并需求', type: 'bar', data: ordered.map(w => Number(w.merged_need_count || 0)) },
  ], { titleSize: 14 })
})

function normalizeDateQuery(value) {
  if (!value) return ''
  const s = String(value).trim()
  if (/^\d{8}$/.test(s)) return `${s.slice(0, 4)}-${s.slice(4, 6)}-${s.slice(6, 8)}`
  if (/^\d{4}-\d{2}-\d{2}$/.test(s)) return s
  return ''
}

async function fetchData() {
  if (!userId.value) return
  loading.value = true
  try {
    const [start, end] = dateRange.value
    const res = await getUserDetailV2(userId.value, { startDate: formatDateParam(start), endDate: formatDateParam(end) })
    const data = res.data || res
    summary.value = data.summary || {}
    weeks.value = data.weeks || []
    needs.value = data.needs || []
    commits.value = data.commits || []
  } catch (err) {
    ElMessage.error(err?.response?.data?.error || err?.message || '获取用户详情失败')
  } finally {
    loading.value = false
  }
}

function fmtDate(value) {
  if (!value) return '-'
  return formatLocalTime(value).slice(0, 10)
}

function shortId(value, size = 10) {
  if (!value) return '-'
  return String(value).length > size ? `${String(value).slice(0, size)}…` : String(value)
}

function statusClass(status) {
  if (status === 'merged') return 'kn-tag--success'
  if (status === 'active') return 'kn-tag--primary'
  return 'kn-tag--neutral'
}

function goNeed(n) {
  if (!n?.need_id) return
  router.push('/needs/' + encodeURIComponent(n.need_id))
}

watch(() => route.params.userId, fetchData)

onMounted(() => {
  const start = normalizeDateQuery(route.query.startDate)
  const end = normalizeDateQuery(route.query.endDate)
  dateRange.value = start && end ? [start, end] : getDefaultDateRangeWide()
  fetchData()
})
</script>
