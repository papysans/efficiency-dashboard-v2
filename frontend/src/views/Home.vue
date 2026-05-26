<template>
  <div class="kanban-native">
    <div class="kn-home" v-loading="loading">
      <!-- header -->
      <header class="kn-home-head">
        <div>
          <h1 class="kn-home-title">AI Coding 指标看板</h1>
          <p class="kn-home-sub">实时追踪 AI 辅助开发效率，量化提效价值</p>
        </div>
        <div class="kn-home-controls">
          <DateRangePicker v-model="dateRange" :clearable="false" @change="fetchData" />
          <el-button type="primary" :icon="Search" :loading="loading" @click="fetchData">查询</el-button>
        </div>
      </header>

      <!-- metrics + efficiency hero -->
      <section class="kn-home-grid">
        <div class="kn-home-metrics">
          <article
            v-for="m in metrics"
            :key="m.key"
            class="kn-home-card"
            :class="{ 'is-live': m.route }"
            :style="{ '--tone': m.tone }"
            @click="m.route && $router.push(m.route)"
          >
            <div class="kn-home-card-top">
              <span class="kn-home-card-label">{{ m.label }}</span>
              <span class="kn-home-card-icon">
                <el-icon :size="20"><component :is="m.icon" /></el-icon>
              </span>
            </div>
            <div class="kn-home-card-value">{{ m.value }}</div>
            <div class="kn-home-card-hint">{{ m.hint }}</div>
          </article>
        </div>

        <section class="kn-hero">
          <div class="kn-hero-top">
            <div class="kn-hero-label">综合日历提效</div>
            <div class="kn-hero-ratio" :class="ratioToneClass">{{ fmtRatio(summary.need_calendar_ratio) }}</div>
            <div class="kn-hero-desc">基于可计入需求（{{ summary.eligible_needs ?? 0 }} 个）的日历口径加权；工作量提效 {{ fmtRatio(summary.need_work_ratio) }}</div>
          </div>
          <div class="kn-hero-stats">
            <div class="kn-hero-stat">
              <span class="kn-hero-stat-label">节省时间</span>
              <span class="kn-hero-stat-value">{{ days(savedMinutes) }}</span>
            </div>
            <div class="kn-hero-stat">
              <span class="kn-hero-stat-label">基线预估</span>
              <span class="kn-hero-stat-value">{{ days(summary.need_baseline_calendar_min) }}</span>
            </div>
            <div class="kn-hero-stat">
              <span class="kn-hero-stat-label">实际耗时</span>
              <span class="kn-hero-stat-value">{{ days(summary.need_actual_calendar_min) }}</span>
            </div>
          </div>
          <span class="kn-hero-unit">单位：人天（8h/人天）</span>
        </section>
      </section>

      <!-- nav -->
      <section class="kn-home-nav-section">
        <h2 class="kn-home-nav-title">功能导航</h2>
        <div class="kn-home-nav-grid">
          <article
            v-for="n in navItems"
            :key="n.route"
            class="kn-home-nav-card"
            :style="{ '--tone': n.tone }"
            @click="$router.push(n.route)"
          >
            <span class="kn-home-nav-icon">
              <el-icon :size="22"><component :is="n.icon" /></el-icon>
            </span>
            <div class="kn-home-nav-text">
              <div class="kn-home-nav-name">{{ n.title }}</div>
              <div class="kn-home-nav-desc">{{ n.desc }}</div>
            </div>
            <el-icon class="kn-home-nav-arrow" :size="16"><ArrowRight /></el-icon>
          </article>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import {
  Search, Monitor, User, Document, Connection, DataAnalysis,
  OfficeBuilding, Timer, Folder, ArrowRight, Histogram,
} from '@element-plus/icons-vue'
import { getDashboardSummary } from '@/api/es'
import { getDefaultDateRangeWide } from '@/utils/date'
import { formatNumber, formatV2Ratio } from '@/utils/formatters'
import DateRangePicker from '@/components/DateRangePicker.vue'

const dateRange = ref(getDefaultDateRangeWide())
const loading = ref(false)
const summary = ref({})

function fmtInt(val) {
  if (val == null) return '-'
  return formatNumber(val, 0)
}

function fmtRatio(val) {
  return formatV2Ratio(val)
}

function days(min) {
  if (min == null || min <= 0) return '-'
  return (Number(min) / 480).toFixed(1)
}

const savedMinutes = computed(() => {
  const b = summary.value.need_baseline_calendar_min || 0
  const a = summary.value.need_actual_calendar_min || 0
  return Math.max(0, b - a)
})

const ratioToneClass = computed(() => {
  const r = Number(summary.value.need_calendar_ratio)
  if (!Number.isFinite(r)) return 'is-neutral'
  if (r < 0) return 'is-neg'
  return 'is-pos'
})

const metrics = computed(() => [
  {
    key: 'repos',
    label: '总仓库数',
    value: fmtInt(summary.value.total_repos),
    hint: `分支 ${fmtInt(summary.value.total_branchs)} 个`,
    tone: '#0053dc',
    icon: Monitor,
    route: '/repo-v2',
  },
  {
    key: 'users',
    label: '总用户数',
    value: fmtInt(summary.value.total_users),
    hint: '参与提交的贡献者',
    tone: '#18a957',
    icon: User,
    route: '/user-v2',
  },
  {
    key: 'needs',
    label: '需求 Need',
    value: fmtInt(summary.value.total_needs),
    hint: `已合并 ${fmtInt(summary.value.merged_needs)} · 可计入 ${fmtInt(summary.value.eligible_needs)}`,
    tone: '#2e86ab',
    icon: DataAnalysis,
    route: '/needs-v2',
  },
  {
    key: 'commits',
    label: '总 Commit 数',
    value: fmtInt(summary.value.total_commits),
    hint: `代码行 ${fmtInt(summary.value.total_commit_lines)}`,
    tone: '#8a4cf6',
    icon: Connection,
    route: '/commit-v2',
  },
  {
    key: 'lines',
    label: '总代码行',
    value: fmtInt(summary.value.total_commit_lines),
    hint: 'commit 净改动行数',
    tone: '#ff7a00',
    icon: Histogram,
    route: null,
  },
])

const navItems = [
  { route: '/needs-v2', title: '需求视图', desc: '按需求边界查看 V2 提效', icon: DataAnalysis, tone: '#2e86ab' },
  { route: '/user-v2', title: '用户视图', desc: '按用户维度查看指标数据', icon: User, tone: '#18a957' },
  { route: '/org-v2', title: '组织视图', desc: '按组织维度查看指标数据', icon: OfficeBuilding, tone: '#b188ef' },
  { route: '/repo-v2', title: '仓库视图', desc: '按仓库维度查看指标数据', icon: Monitor, tone: '#0053dc' },
  { route: '/commit-v2', title: '提交视图', desc: '按提交维度查看指标数据', icon: Connection, tone: '#8a4cf6' },
  { route: '/task-v2', title: '任务视图', desc: '查看 AI 任务详情与耗时', icon: Timer, tone: '#f0b93f' },
  { route: '/project-v2', title: '项目视图', desc: '虚拟项目管理与提效分析', icon: Folder, tone: '#ff7a00' },
]

async function fetchData() {
  if (!dateRange.value || dateRange.value.length !== 2) return
  loading.value = true
  try {
    const params = {
      startDate: dateRange.value[0].replace(/-/g, ''),
      endDate: dateRange.value[1].replace(/-/g, ''),
    }
    const result = await getDashboardSummary(params)
    summary.value = result.data || result || {}
  } catch {
    summary.value = {}
  } finally {
    loading.value = false
  }
}

onMounted(fetchData)
</script>

<style scoped>
.kn-home {
  position: relative;
  z-index: 1;
  display: flex;
  flex-direction: column;
  gap: 1.75rem;
  padding: clamp(1.25rem, 3vw, 3rem);
}

.kn-home-head {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  justify-content: space-between;
  gap: 1rem;
}
.kn-home-title {
  margin: 0;
  font-size: clamp(1.9rem, 3.2vw, 2.6rem);
  font-weight: 600;
  letter-spacing: -0.05em;
  color: var(--native-foreground);
}
.kn-home-sub { margin: 0.4rem 0 0; font-size: 0.95rem; color: var(--native-muted); }
.kn-home-controls { display: flex; align-items: center; gap: 0.625rem; }

/* metrics + hero */
.kn-home-grid {
  display: grid;
  gap: 1.25rem;
  grid-template-columns: 1fr;
}
@media (min-width: 1200px) {
  .kn-home-grid { grid-template-columns: minmax(0, 1.15fr) minmax(24rem, 0.85fr); }
}
.kn-home-metrics {
  display: grid;
  gap: 1rem;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}
@media (min-width: 900px) { .kn-home-metrics { grid-template-columns: repeat(3, minmax(0, 1fr)); } }

.kn-home-card {
  display: flex;
  flex-direction: column;
  min-height: 10.5rem;
  border-radius: 20px;
  border: 1px solid color-mix(in oklab, var(--native-border) 40%, var(--native-bg));
  background: var(--native-panel);
  padding: 1.15rem 1.25rem;
  box-shadow: var(--native-shadow-sm);
  transition: transform 0.2s ease;
}
.kn-home-card.is-live { cursor: pointer; }
.kn-home-card.is-live:hover { transform: translateY(-3px); box-shadow: var(--native-shadow-md); }
.kn-home-card-top { display: flex; align-items: flex-start; justify-content: space-between; gap: 1rem; }
.kn-home-card-label { font-size: 0.95rem; font-weight: 500; letter-spacing: -0.02em; color: var(--native-foreground); }
.kn-home-card-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 2.8rem;
  height: 2.8rem;
  border-radius: 9999px;
  color: var(--tone);
  background: color-mix(in oklab, var(--tone) 12%, var(--native-bg));
}
.kn-home-card-value {
  margin-top: auto;
  padding-top: 0.75rem;
  font-size: 2.8rem;
  line-height: 1;
  font-weight: 600;
  letter-spacing: -0.06em;
  color: var(--tone);
  font-variant-numeric: tabular-nums;
}
.kn-home-card-hint { margin-top: 0.75rem; font-size: 0.9rem; color: var(--native-muted); }

/* efficiency hero */
.kn-hero {
  position: relative;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  gap: 2rem;
  overflow: hidden;
  border-radius: 22px;
  border: 1px solid color-mix(in oklab, var(--native-primary) 25%, var(--native-bg));
  background: linear-gradient(115deg, color-mix(in oklab, var(--native-primary) 8%, var(--native-panel)) 42%, color-mix(in oklab, var(--native-primary) 16%, var(--native-panel)) 100%);
  padding: 1.5rem 1.5rem 2.5rem;
  box-shadow: var(--native-shadow-md);
}
.kn-hero-label { font-size: 0.95rem; font-weight: 500; color: var(--native-foreground); }
.kn-hero-ratio {
  margin-top: 1.5rem;
  font-size: clamp(2.6rem, 5vw, 4rem);
  line-height: 1;
  font-weight: 600;
  letter-spacing: -0.08em;
  font-variant-numeric: tabular-nums;
}
.kn-hero-ratio.is-pos { color: var(--native-primary); }
.kn-hero-ratio.is-neg { color: var(--native-error); }
.kn-hero-ratio.is-neutral { color: var(--native-muted); }
.kn-hero-desc { margin-top: 0.9rem; max-width: 28rem; font-size: 0.85rem; line-height: 1.5; color: var(--native-muted); }
.kn-hero-stats {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  border-top: 1px solid var(--native-border);
  padding-top: 1.25rem;
}
.kn-hero-stat { display: flex; flex-direction: column; gap: 0.5rem; padding: 0 1.1rem; }
.kn-hero-stat:first-child { padding-left: 0; }
.kn-hero-stat + .kn-hero-stat { border-left: 1px solid var(--native-border); }
.kn-hero-stat-label { font-size: 0.9rem; color: var(--native-foreground); }
.kn-hero-stat-value { font-size: 2rem; line-height: 1; font-weight: 600; letter-spacing: -0.06em; color: var(--native-foreground); font-variant-numeric: tabular-nums; }
.kn-hero-unit { position: absolute; right: 1.5rem; bottom: 1rem; font-size: 0.78rem; color: var(--native-muted); }

/* nav */
.kn-home-nav-section { display: flex; flex-direction: column; gap: 1rem; }
.kn-home-nav-title { margin: 0; font-size: 1.5rem; font-weight: 600; letter-spacing: -0.04em; color: var(--native-foreground); }
.kn-home-nav-grid {
  display: grid;
  gap: 1rem;
  grid-template-columns: repeat(auto-fill, minmax(min(100%, 18rem), 1fr));
}
.kn-home-nav-card {
  display: flex;
  align-items: center;
  gap: 1rem;
  min-height: 6.5rem;
  border-radius: 18px;
  border: 1px solid color-mix(in oklab, var(--native-border) 40%, var(--native-bg));
  background: var(--native-panel);
  padding: 1.1rem 1.25rem;
  cursor: pointer;
  box-shadow: var(--native-shadow-sm);
  transition: transform 0.2s ease;
}
.kn-home-nav-card:hover { transform: translateY(-3px); box-shadow: var(--native-shadow-md); }
.kn-home-nav-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  width: 3rem;
  height: 3rem;
  border-radius: 14px;
  color: var(--tone);
  background: color-mix(in oklab, var(--tone) 12%, var(--native-bg));
}
.kn-home-nav-text { flex: 1; min-width: 0; }
.kn-home-nav-name { font-size: 1.05rem; font-weight: 500; letter-spacing: -0.02em; color: var(--native-foreground); }
.kn-home-nav-desc { margin-top: 0.25rem; font-size: 0.85rem; color: var(--native-muted); }
.kn-home-nav-arrow { color: var(--native-dim); transition: transform 0.2s ease; }
.kn-home-nav-card:hover .kn-home-nav-arrow { transform: translateX(3px); color: var(--native-foreground); }
</style>
