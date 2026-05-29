<template>
  <div class="kanban-native">
    <div class="kn-page" v-loading="loading">
      <!-- header -->
      <header class="kn-header">
        <button class="kn-back" @click="router.back()"><span>&lt;</span><span>返回</span></button>
        <div class="kn-title-row">
          <div>
            <h1 class="kn-title">需求 Need 详情</h1>
            <p class="kn-subtitle is-mono">{{ need.need_id || '-' }}</p>
          </div>
          <div class="kn-pills">
            <span v-if="need.status" class="kn-tag" :class="statusTagClass(need.status)">{{ need.status }}</span>
            <span v-if="need.confidence_level" class="kn-tag" :class="confidenceTagClass(need.confidence_level)">效率置信 {{ need.confidence_level }}</span>
            <span class="kn-tag" :class="need.coverage_eligible ? 'kn-tag--success' : 'kn-tag--neutral'">{{ need.coverage_eligible ? '可计入' : '未计入' }}</span>
            <span v-if="need.outlier_flag" class="kn-tag kn-tag--error">异常样本</span>
          </div>
        </div>
      </header>

      <!-- metric cards -->
      <section class="kn-metrics">
        <MetricCard label="日历提效" accent="var(--native-success)" :hint="formatBand">
          <RatioPill :value="need.efficiency_ratio" />
        </MetricCard>
        <MetricCard label="工作量提效" accent="var(--native-info)">
          <RatioPill :value="need.work_efficiency_ratio" />
        </MetricCard>
        <MetricCard label="实际日历" :value="formatDuration(need.total_calendar_min)" accent="var(--native-primary)" />
        <MetricCard label="基线日历" :value="formatDuration(need.baseline_calendar_min)" accent="var(--native-primary)" />
        <MetricCard label="实际工作量" :value="formatDuration(need.total_active_work_corrected_min)" accent="var(--native-warning)" />
        <MetricCard label="融合基线工作量" :value="formatDuration(need.baseline_fused_work_min)" accent="var(--native-warning)" />
      </section>

      <!-- 基础信息 -->
      <section class="kn-panel">
        <div class="kn-panel-head"><span>基础信息</span></div>
        <div class="kn-panel-body">
          <div class="kn-kv">
            <div class="kn-kv-item"><span class="kn-kv-label">边界来源</span><span class="kn-kv-value">{{ need.boundary_source || '-' }}</span></div>
            <div class="kn-kv-item"><span class="kn-kv-label">边界置信</span><span class="kn-kv-value"><span class="kn-tag" :class="confidenceTagClass(need.boundary_confidence)">{{ need.boundary_confidence || '-' }}</span></span></div>
            <div class="kn-kv-item is-wide"><span class="kn-kv-label">边界标识</span><span class="kn-kv-value is-mono">{{ need.boundary_key || '-' }}</span></div>
            <div class="kn-kv-item is-wide"><span class="kn-kv-label">仓库</span><span class="kn-kv-value is-mono">{{ need.repo_addr || '-' }}</span></div>
            <div class="kn-kv-item"><span class="kn-kv-label">分支</span><span class="kn-kv-value is-mono">{{ need.repo_branch || '-' }}</span></div>
            <div class="kn-kv-item"><span class="kn-kv-label">主用户</span><span class="kn-kv-value">{{ need.primary_user_id || '-' }}</span></div>
            <div class="kn-kv-item"><span class="kn-kv-label">协作人数</span><span class="kn-kv-value">{{ contributorCount }}</span></div>
            <div class="kn-kv-item"><span class="kn-kv-label">开始时间</span><span class="kn-kv-value">{{ formatLocalTime(need.dev_start_ts) }}</span></div>
            <div class="kn-kv-item"><span class="kn-kv-label">结束时间</span><span class="kn-kv-value">{{ formatLocalTime(need.dev_end_ts) }}</span></div>
            <div class="kn-kv-item"><span class="kn-kv-label">合并时间</span><span class="kn-kv-value">{{ formatLocalTime(need.merge_ts) }}</span></div>
            <div class="kn-kv-item"><span class="kn-kv-label">开发跨度</span><span class="kn-kv-value">{{ formatDuration(need.dev_duration_min) }}</span></div>
            <div class="kn-kv-item"><span class="kn-kv-label">等待评审</span><span class="kn-kv-value">{{ formatDuration(need.wait_for_review_min) }}</span></div>
            <div class="kn-kv-item"><span class="kn-kv-label">团队画像</span><span class="kn-kv-value">{{ need.team_profile_used || '-' }}</span></div>
            <div class="kn-kv-item is-wide">
              <span class="kn-kv-label">说明</span>
              <span class="kn-kv-value">
                <span v-if="!reasonItems.length" class="is-muted">-</span>
                <span v-else class="kn-pills">
                  <span v-for="(r, i) in reasonItems" :key="i" class="kn-tag" :class="reasonTagClass(r.tone)" :title="r.hint">{{ r.label }}</span>
                </span>
              </span>
            </div>
          </div>
        </div>
      </section>

      <!-- 提效与基线 -->
      <div class="kn-grid-2">
        <section class="kn-panel">
          <div class="kn-panel-head"><span>基线组成（工作量，分钟）</span></div>
          <div class="kn-panel-body kn-table-wrap">
            <table class="kn-table">
              <thead>
                <tr><th>来源</th><th class="kn-num">思考</th><th class="kn-num">执行</th><th class="kn-num">验证</th><th class="kn-num">合计</th><th>说明</th></tr>
              </thead>
              <tbody>
                <tr v-for="r in baselineRows" :key="r.name">
                  <td>{{ r.name }}</td>
                  <td class="kn-num">{{ fmtMin(r.think) }}</td>
                  <td class="kn-num">{{ fmtMin(r.exec) }}</td>
                  <td class="kn-num">{{ fmtMin(r.verify) }}</td>
                  <td class="kn-num">{{ fmtMin(r.total) }}</td>
                  <td><div class="kn-ellipsis" :title="reasonHints(r.reason)">{{ r.reason ? reasonSummary(r.reason) : '-' }}</div></td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>
        <NativeChart :option="baselineChart" empty="暂无基线数据" />
      </div>

      <!-- 阶段工作量 -->
      <div class="kn-grid-2">
        <section class="kn-panel">
          <div class="kn-panel-head"><span>阶段工作量</span></div>
          <div class="kn-panel-body">
            <div class="kn-kv">
              <div class="kn-kv-item"><span class="kn-kv-label">思考</span><span class="kn-kv-value">{{ formatDuration(need.total_think_min) }}</span></div>
              <div class="kn-kv-item"><span class="kn-kv-label">执行</span><span class="kn-kv-value">{{ formatDuration(need.total_exec_min) }}</span></div>
              <div class="kn-kv-item"><span class="kn-kv-label">验证</span><span class="kn-kv-value">{{ formatDuration(need.total_verify_min) }}</span></div>
              <div class="kn-kv-item"><span class="kn-kv-label">其他</span><span class="kn-kv-value">{{ formatDuration(need.total_other_min) }}</span></div>
              <div class="kn-kv-item"><span class="kn-kv-label">会话活跃人工</span><span class="kn-kv-value">{{ formatDuration(need.total_session_active_person_min) }}</span></div>
              <div class="kn-kv-item"><span class="kn-kv-label">未覆盖人工估算</span><span class="kn-kv-value">{{ formatDuration(need.estimate_uncovered_human_min) }}</span></div>
            </div>
          </div>
        </section>
        <NativeChart :option="stageChart" empty="暂无阶段数据" />
      </div>

      <!-- 代码与质量信号 -->
      <section class="kn-panel">
        <div class="kn-panel-head">
          <span>代码与质量信号</span>
          <span v-if="qualityReason" class="kn-panel-hint" :title="reasonHints(qualityReason)">{{ reasonSummary(qualityReason) }}</span>
        </div>
        <div class="kn-panel-body">
          <div class="kn-kv">
            <div class="kn-kv-item"><span class="kn-kv-label">净代码行</span><span class="kn-kv-value">{{ fmtInt(need.total_loc_net) }}</span></div>
            <div class="kn-kv-item"><span class="kn-kv-label">改动文件</span><span class="kn-kv-value">{{ fmtInt(need.total_files_touched) }}</span></div>
            <div class="kn-kv-item"><span class="kn-kv-label">提交数</span><span class="kn-kv-value">{{ fmtInt(need.commit_count) }}</span></div>
            <div class="kn-kv-item"><span class="kn-kv-label">AI 代码占比</span><span class="kn-kv-value">{{ fmtPct(need.ai_code_ratio) }}</span></div>
            <div class="kn-kv-item"><span class="kn-kv-label">AI 覆盖行</span><span class="kn-kv-value">{{ fmtInt(need.ai_covered_loc) }}</span></div>
            <div class="kn-kv-item"><span class="kn-kv-label">未覆盖行</span><span class="kn-kv-value">{{ fmtInt(need.uncovered_loc) }}</span></div>
            <div class="kn-kv-item"><span class="kn-kv-label">未覆盖工作占比</span><span class="kn-kv-value">{{ fmtPct(need.uncovered_work_ratio) }}</span></div>
            <div class="kn-kv-item"><span class="kn-kv-label">硅含量</span><span class="kn-kv-value">{{ fmtPct(need.silica) }}</span></div>
            <div class="kn-kv-item"><span class="kn-kv-label">Churn 比</span><span class="kn-kv-value">{{ fmtPct(need.churn_ratio) }}</span></div>
            <div class="kn-kv-item"><span class="kn-kv-label">重复率</span><span class="kn-kv-value">{{ fmtPct(need.duplication_ratio) }}</span></div>
            <div class="kn-kv-item"><span class="kn-kv-label">回退次数 / 回退率</span><span class="kn-kv-value">{{ fmtInt(need.revert_count) }} / {{ fmtPct(need.revert_rate) }}</span></div>
            <div class="kn-kv-item"><span class="kn-kv-label">生成后删除率</span><span class="kn-kv-value">{{ fmtPct(need.post_generation_deletion_ratio) }}</span></div>
          </div>
          <div class="kn-pills" style="margin-top: 1rem">
            <span class="kn-tag" :class="signalTagClass(need.feature_dependency_risk)">特性依赖风险: {{ need.feature_dependency_risk || '未知' }}</span>
            <span class="kn-tag" :class="signalTagClass(need.silica_signal)">硅含量信号: {{ need.silica_signal || '未知' }}</span>
            <span class="kn-tag" :class="signalTagClass(need.ai_code_ratio_signal)">AI 占比信号: {{ need.ai_code_ratio_signal || '未知' }}</span>
            <span class="kn-tag" :class="signalTagClass(need.uncovered_work_signal)">未覆盖工作信号: {{ need.uncovered_work_signal || '未知' }}</span>
          </div>
        </div>
      </section>

      <!-- 改动文件 -->
      <section class="kn-panel">
        <div class="kn-panel-head"><span>改动文件</span><span class="kn-panel-hint">{{ needFiles.length }} 个</span></div>
        <div class="kn-panel-body">
          <div v-if="!needFiles.length" class="kn-empty">暂无改动文件</div>
          <div v-else class="kn-pills">
            <span v-for="f in needFiles" :key="f" class="kn-tag kn-tag--neutral is-mono" :title="f">{{ f }}</span>
          </div>
        </div>
      </section>

      <!-- 关联 Sessions -->
      <section class="kn-panel">
        <div class="kn-panel-head"><span>关联 Sessions</span><span class="kn-panel-hint">{{ sessions.length }} 个</span></div>
        <div class="kn-table-wrap">
          <table class="kn-table">
            <thead>
              <tr>
                <th>Session</th><th>用户</th><th>开始</th><th>结束</th>
                <th class="kn-num">活跃工作量</th><th class="kn-num">思考</th><th class="kn-num">执行</th><th class="kn-num">验证</th>
                <th>阶段置信</th><th>摘要</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="!sessions.length"><td colspan="10"><div class="kn-empty">暂无 Session</div></td></tr>
              <tr v-for="s in sessions" :key="s.session_id">
                <td class="is-mono">{{ shortId(s.session_id) }}</td>
                <td><div class="kn-ellipsis" :title="s.user_id">{{ s.user_id || '-' }}</div></td>
                <td>{{ formatLocalTime(s.session_start_ts) }}</td>
                <td>{{ formatLocalTime(s.session_end_ts) }}</td>
                <td class="kn-num">{{ formatDuration(s.total_active_min) }}</td>
                <td class="kn-num">{{ formatDuration(s.think_active_min) }}</td>
                <td class="kn-num">{{ formatDuration(s.exec_active_min) }}</td>
                <td class="kn-num">{{ formatDuration(s.verify_active_min) }}</td>
                <td><span class="kn-tag" :class="confidenceTagClass(s.stage_confidence)">{{ s.stage_confidence || '-' }}</span></td>
                <td><div class="kn-ellipsis" :title="s.summary">{{ s.summary || '-' }}</div></td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <!-- 关联 Commits -->
      <section class="kn-panel">
        <div class="kn-panel-head"><span>关联 Commits</span><span class="kn-panel-hint">{{ commits.length }} 个</span></div>
        <div class="kn-table-wrap">
          <table class="kn-table">
            <thead>
              <tr><th>Commit</th><th>提交时间</th><th>用户</th><th class="kn-num">代码行</th><th class="kn-num">硅含量</th><th>提交说明</th><th>改动文件</th></tr>
            </thead>
            <tbody>
              <tr v-if="!commits.length"><td colspan="7"><div class="kn-empty">暂无 Commit</div></td></tr>
              <tr v-for="c in commits" :key="c.commit_id">
                <td><button class="kn-link" @click="router.push('/commit/' + encodeURIComponent(c.commit_id))">{{ shortId(c.commit_id, 10) }}</button></td>
                <td>{{ formatLocalTime(c.commit_time) }}</td>
                <td><div class="kn-ellipsis" :title="c.user_name">{{ c.user_name || '-' }}</div></td>
                <td class="kn-num">{{ fmtInt(c.diff_lines) }}</td>
                <td class="kn-num">{{ fmtPct(c.silica) }}</td>
                <td><div class="kn-ellipsis" :title="c.comment">{{ c.comment || '-' }}</div></td>
                <td>
                  <div v-if="commitFiles(c).length" class="kn-ellipsis is-mono" style="max-width: 360px" :title="commitFiles(c).join('\n')">
                    <strong>{{ commitFiles(c).length }}</strong> · {{ commitFiles(c).join('  ·  ') }}
                  </div>
                  <span v-else class="is-muted">-</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup>
defineOptions({ name: 'NeedDetailV2' })
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import MetricCard from '@/components/native/MetricCard.vue'
import RatioPill from '@/components/native/RatioPill.vue'
import NativeChart from '@/components/native/NativeChart.vue'
import { getNeedDetailV2 } from '@/api/es'
import { formatDuration, formatLocalTime, formatV2Ratio, formatNumber } from '@/utils/formatters'
import { parseReason, reasonSummary, reasonHints } from '@/utils/reasonText'
import { kanbanChart } from '@/utils/kanbanChart'

const route = useRoute()
const router = useRouter()

const loading = ref(false)
const need = ref({})
const sessions = ref([])
const commits = ref([])
const baselineComponents = ref({})
const qualitySignals = ref({})
const confidenceSignals = ref({})

const formatBand = computed(() => {
  if (need.value.efficiency_band_low == null && need.value.efficiency_band_high == null) return ''
  return `区间 ${formatV2Ratio(need.value.efficiency_band_low)} ~ ${formatV2Ratio(need.value.efficiency_band_high)}`
})

const contributorCount = computed(() => {
  const c = need.value.contributor_user_ids
  if (Array.isArray(c)) return c.length
  return '-'
})

const qualityReason = computed(() => qualitySignals.value?.reason || '')

const reasonItems = computed(() => parseReason(need.value.reason))

// touched_files 经后端 StringJSON.MarshalJSON 已是数组；个别情况下可能是 JSON 字符串，统一归一。
function asFileList(value) {
  if (Array.isArray(value)) return value.filter(Boolean)
  if (typeof value === 'string') {
    const s = value.trim()
    if (!s || s === '[]' || s === 'null') return []
    try {
      const arr = JSON.parse(s)
      return Array.isArray(arr) ? arr.filter(Boolean) : []
    } catch {
      return []
    }
  }
  return []
}

const needFiles = computed(() => asFileList(need.value.touched_files))
function commitFiles(c) {
  return asFileList(c?.touched_files)
}

function reasonTagClass(tone) {
  if (tone === 'error') return 'kn-tag--error'
  if (tone === 'warning') return 'kn-tag--warning'
  if (tone === 'info') return 'kn-tag--info'
  return 'kn-tag--neutral'
}

const baselineRows = computed(() => {
  const b = baselineComponents.value || {}
  return [
    { name: '算法基线', think: b.algo_think_min, exec: b.algo_exec_min, verify: b.algo_verify_min, total: b.algo_total_min, reason: '' },
    { name: '相似锚点 kNN', think: null, exec: null, verify: null, total: b.anchor_knn_min, reason: b.anchor_knn_reason || '' },
    { name: 'LLM 估算', think: b.llm_think_min, exec: b.llm_exec_min, verify: b.llm_verify_min, total: b.llm_total_min, reason: b.llm_reason || b.llm_confidence || '' },
    { name: '融合工作量', think: null, exec: null, verify: null, total: b.fused_work_min, reason: '' },
    { name: '离散工作量', think: null, exec: null, verify: null, total: b.spread_work_min, reason: '' },
    { name: '日历基线', think: null, exec: null, verify: null, total: b.calendar_min, reason: b.team_work_density == null ? '' : `团队工作密度 ${b.team_work_density}` },
  ]
})

const stageChart = computed(() => {
  const n = need.value
  const data = [n.total_think_min, n.total_exec_min, n.total_verify_min, n.total_other_min].map(v => Number(v || 0))
  if (data.every(v => v === 0)) return null
  return kanbanChart('阶段工作量分布', ['思考', '执行', '验证', '其他'], [{ name: '活跃工作量（分钟）', data }], {
    titleSize: 14,
    format: v => formatDuration(v),
  })
})

const baselineChart = computed(() => {
  const b = baselineComponents.value || {}
  const n = need.value
  const items = [
    ['实际工作量', n.total_active_work_corrected_min],
    ['算法', b.algo_total_min],
    ['锚点 kNN', b.anchor_knn_min],
    ['LLM', b.llm_total_min],
    ['融合', b.fused_work_min],
  ]
  const data = items.map(([, v]) => Number(v || 0))
  if (data.every(v => v === 0)) return null
  return kanbanChart('工作量基线对比', items.map(([k]) => k), [{ name: '分钟', data }], {
    titleSize: 14,
    format: v => formatDuration(v),
  })
})

async function loadData() {
  const needId = route.params.needId
  if (!needId) return
  loading.value = true
  try {
    const res = await getNeedDetailV2(needId)
    const data = res.data || res
    need.value = data.need || {}
    sessions.value = data.sessions || data.stage_metrics || []
    commits.value = data.commits || []
    baselineComponents.value = data.baseline_components || {}
    qualitySignals.value = data.quality_signals || {}
    confidenceSignals.value = data.confidence_signals || {}
  } catch (err) {
    ElMessage.error(err?.response?.data?.error || err?.message || '获取 Need 详情失败')
  } finally {
    loading.value = false
  }
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

function signalTagClass(signal) {
  const s = String(signal || '').toLowerCase()
  if (s === 'ok' || s === 'low') return 'kn-tag--success'
  if (s === 'medium' || s === 'warn' || s === 'warning') return 'kn-tag--warning'
  if (s === 'high' || s === 'risk' || s === 'bad') return 'kn-tag--error'
  return 'kn-tag--neutral'
}

function fmtMin(value) {
  if (value == null) return '-'
  return formatNumber(value, 0)
}

function fmtInt(value) {
  if (value == null) return '-'
  return formatNumber(value, 0)
}

function fmtPct(value) {
  if (value == null || value === 0) return '-'
  return formatV2Ratio(value)
}

function shortId(value, size = 8) {
  if (!value) return '-'
  return String(value).slice(0, size)
}

watch(() => route.params.needId, loadData)
onMounted(loadData)
</script>
