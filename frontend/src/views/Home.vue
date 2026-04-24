<template>
  <div class="home-page" v-loading="loading">
    <!-- Hero Header -->
    <div class="hero-header">
      <div class="hero-bg"></div>
      <div class="hero-content">
        <div class="hero-title-area">
          <h1 class="hero-title">AI Coding 指标看板</h1>
          <p class="hero-subtitle">实时追踪 AI 辅助开发效率，量化提效价值</p>
        </div>
        <div class="hero-filter">
          <DateRangePicker v-model="dateRange" :clearable="false" />
          <el-button type="primary" :icon="Search" :loading="loading" @click="fetchData" size="large">
            查询
          </el-button>
        </div>
      </div>
    </div>

    <!-- Core Metrics -->
    <div class="section-container">
      <div class="metrics-grid">
        <div
          v-for="metric in coreMetrics"
          :key="metric.key"
          class="metric-card"
          :style="{ '--accent': metric.color }"
          @click="metric.route && $router.push(metric.route)"
          :class="{ clickable: metric.route }"
        >
          <div class="metric-card-inner">
            <div class="metric-icon-wrap">
              <el-icon :size="22" :style="{ color: metric.color }">
                <component :is="metric.icon" />
              </el-icon>
            </div>
            <div class="metric-info">
              <div class="metric-label">{{ metric.label }}</div>
              <div class="metric-value" :style="{ color: metric.color }">{{ metric.value }}</div>
            </div>
          </div>
          <div class="metric-card-bar" :style="{ background: metric.color }"></div>
        </div>
      </div>
    </div>

    <!-- Efficiency Banner -->
    <div class="section-container">
      <div class="efficiency-section" :style="efficiencyStyle">
        <div class="efficiency-left">
          <div class="efficiency-label">综合提效比</div>
          <div class="efficiency-desc">AI 辅助 vs 传统开发时长对比</div>
        </div>
        <div class="efficiency-center">
          <div class="efficiency-ratio">{{ formatEfficiencyRatio(summary.avg_efficiency_ratio) }}</div>
        </div>
        <div class="efficiency-right">
          <div class="efficiency-stat">
            <span class="stat-label">节省时间</span>
            <span class="stat-value">{{ formatSavedTime() }}</span>
          </div>
          <div class="efficiency-divider"></div>
          <div class="efficiency-stat">
            <span class="stat-label">传统预估</span>
            <span class="stat-value">{{ formatDurationVal(summary.total_task_ancient_minutes) }}</span>
          </div>
          <div class="efficiency-divider"></div>
          <div class="efficiency-stat">
            <span class="stat-label">实际耗时</span>
            <span class="stat-value">{{ formatDurationVal(summary.total_real_minutes) }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Navigation -->
    <div class="section-container">
      <div class="section-header">
        <span class="section-title">功能导航</span>
        <span class="section-line"></span>
      </div>
      <div class="nav-grid">
        <div
          v-for="nav in navItems"
          :key="nav.route"
          class="nav-card"
          :style="{ '--nav-color': nav.color }"
          @click="$router.push(nav.route)"
        >
          <div class="nav-icon-wrap">
            <el-icon :size="28" :style="{ color: nav.color }">
              <component :is="nav.icon" />
            </el-icon>
          </div>
          <div class="nav-title">{{ nav.title }}</div>
          <div class="nav-desc">{{ nav.desc }}</div>
          <div class="nav-arrow">
            <el-icon :size="14" color="#c0c4cc"><ArrowRight /></el-icon>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import {
  Search, Monitor, User, Document, Connection, Coin,
  Timer, OfficeBuilding, TrendCharts, Folder, ArrowRight
} from '@element-plus/icons-vue'
import { getDashboardSummary } from '@/api/es'
import { getDefaultDateRangeWide } from '@/utils/date'
import { formatDuration } from '@/utils/formatters'
import DateRangePicker from '@/components/DateRangePicker.vue'

const dateRange = ref(getDefaultDateRangeWide())
const loading = ref(false)
const summary = ref({})

function formatCost(val) {
  if (val == null) return '-'
  return Number(val).toFixed(2)
}

function formatDurationVal(val) {
  if (val == null) return '-'
  return formatDuration(val)
}

function formatSavedTime() {
  const ancient = summary.value.total_task_ancient_minutes || 0
  const real = summary.value.total_real_minutes || 0
  const saved = ancient - real
  if (saved <= 0 || ancient === 0) return '-'
  return formatDuration(saved)
}

function formatEfficiencyRatio(val) {
  if (val == null || val === 0) return '-'
  return val.toFixed(1) + '%'
}

const coreMetrics = computed(() => [
  {
    key: 'repos',
    label: '总仓库数',
    value: summary.value.total_repos ?? '-',
    color: '#409EFF',
    icon: Monitor,
    route: '/repo-v2',
  },
  {
    key: 'users',
    label: '总用户数',
    value: summary.value.total_users ?? '-',
    color: '#67C23A',
    icon: User,
    route: '/user-v2',
  },
  {
    key: 'tasks',
    label: '总 Task 数',
    value: summary.value.total_tasks ?? '-',
    color: '#E6A23C',
    icon: Document,
    route: '/task-v2',
  },
  {
    key: 'commits',
    label: '总 Commit 数',
    value: summary.value.total_commits ?? '-',
    color: '#F56C6C',
    icon: Connection,
    route: '/commit-v2',
  },
  {
    key: 'cost',
    label: '总费用（元）',
    value: formatCost(summary.value.total_cost),
    color: '#9B59B6',
    icon: Coin,
    route: null,
  },
])

const navItems = [
  { route: '/repo-v2', title: '仓库视图', desc: '按仓库维度查看指标数据', icon: Monitor, color: '#409EFF' },
  { route: '/user-v2', title: '用户视图', desc: '按用户维度查看指标数据', icon: User, color: '#67C23A' },
  { route: '/org-v2', title: '组织视图', desc: '按组织维度查看指标数据', icon: OfficeBuilding, color: '#E6A23C' },
  { route: '/commit-v2', title: '提交视图', desc: '按提交维度查看指标数据', icon: Connection, color: '#F56C6C' },
  { route: '/task-v2', title: '任务视图', desc: '查看 AI 任务详情与耗时', icon: Timer, color: '#1ABC9C' },
  { route: '/project-v2', title: '项目视图', desc: '虚拟项目管理与提效分析', icon: Folder, color: '#F39C12' },
]

const efficiencyStyle = computed(() => {
  const val = summary.value.avg_efficiency_ratio
  if (val == null || val === 0) {
    return { background: 'linear-gradient(135deg, #636e72 0%, #2d3436 100%)' }
  }
  if (val >= 300) {
    return { background: 'linear-gradient(135deg, #00b894 0%, #00cec9 50%, #0984e3 100%)' }
  }
  if (val >= 150) {
    return { background: 'linear-gradient(135deg, #0984e3 0%, #6c5ce7 100%)' }
  }
  return { background: 'linear-gradient(135deg, #fdcb6e 0%, #e17055 100%)' }
})

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
  } catch (err) {
    summary.value = {}
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchData()
})
</script>

<style scoped>
.home-page {
  min-height: 100%;
  background: #f0f2f5;
}

/* Hero */
.hero-header {
  position: relative;
  overflow: hidden;
  background: linear-gradient(135deg, #1a1f36 0%, #2d3561 60%, #1e3a5f 100%);
  padding: 32px 32px 36px;
}

.hero-bg {
  position: absolute;
  inset: 0;
  background:
    radial-gradient(ellipse at 20% 50%, rgba(64, 158, 255, 0.15) 0%, transparent 60%),
    radial-gradient(ellipse at 80% 20%, rgba(103, 194, 58, 0.1) 0%, transparent 50%);
  pointer-events: none;
}

.hero-content {
  position: relative;
  max-width: 1200px;
  margin: 0 auto;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 32px;
  flex-wrap: wrap;
}

.hero-title {
  font-size: 28px;
  font-weight: 700;
  color: #fff;
  margin: 0 0 6px 0;
  letter-spacing: 0.5px;
}

.hero-subtitle {
  font-size: 14px;
  color: rgba(255, 255, 255, 0.6);
  margin: 0;
}

.hero-filter {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}

/* Section */
.section-container {
  max-width: 1200px;
  margin: 0 auto;
  padding: 20px 32px 0;
}

.section-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
}

.section-title {
  font-size: 15px;
  font-weight: 600;
  color: #303133;
  white-space: nowrap;
}

.section-line {
  flex: 1;
  height: 1px;
  background: #e4e7ed;
}

/* Metrics Grid */
.metrics-grid {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 14px;
}

.metric-card {
  background: #fff;
  border-radius: 10px;
  overflow: hidden;
  position: relative;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.06);
  transition: transform 0.2s, box-shadow 0.2s;
}

.metric-card.clickable {
  cursor: pointer;
}

.metric-card.clickable:hover {
  transform: translateY(-3px);
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.1);
}

.metric-card-inner {
  padding: 18px 16px 14px;
  display: flex;
  align-items: flex-start;
  gap: 12px;
}

.metric-icon-wrap {
  width: 42px;
  height: 42px;
  border-radius: 10px;
  background: color-mix(in srgb, var(--accent) 10%, transparent);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.metric-info {
  flex: 1;
  min-width: 0;
}

.metric-label {
  font-size: 12px;
  color: #909399;
  margin-bottom: 6px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.metric-value {
  font-size: 26px;
  font-weight: 700;
  line-height: 1;
}

.metric-card-bar {
  height: 3px;
  width: 100%;
}

/* Efficiency Section */
.efficiency-section {
  border-radius: 12px;
  padding: 24px 32px;
  display: flex;
  align-items: center;
  gap: 32px;
  color: #fff;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.15);
}

.efficiency-left {
  flex-shrink: 0;
}

.efficiency-label {
  font-size: 16px;
  font-weight: 600;
  margin-bottom: 4px;
}

.efficiency-desc {
  font-size: 12px;
  opacity: 0.7;
}

.efficiency-center {
  flex: 1;
  text-align: center;
}

.efficiency-ratio {
  font-size: 52px;
  font-weight: 800;
  line-height: 1;
  text-shadow: 0 2px 10px rgba(0, 0, 0, 0.2);
}

.efficiency-right {
  display: flex;
  align-items: center;
  gap: 24px;
  flex-shrink: 0;
}

.efficiency-stat {
  text-align: center;
}

.stat-label {
  display: block;
  font-size: 11px;
  opacity: 0.7;
  margin-bottom: 4px;
}

.stat-value {
  display: block;
  font-size: 18px;
  font-weight: 600;
}

.efficiency-divider {
  width: 1px;
  height: 36px;
  background: rgba(255, 255, 255, 0.25);
}

/* Nav Grid */
.nav-grid {
  display: grid;
  grid-template-columns: repeat(6, 1fr);
  gap: 12px;
  padding-bottom: 24px;
}

.nav-card {
  background: #fff;
  border-radius: 10px;
  padding: 20px 16px 16px;
  cursor: pointer;
  transition: transform 0.2s, box-shadow 0.2s;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.06);
  position: relative;
  border-top: 3px solid var(--nav-color);
}

.nav-card:hover {
  transform: translateY(-3px);
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.1);
}

.nav-icon-wrap {
  width: 48px;
  height: 48px;
  border-radius: 12px;
  background: color-mix(in srgb, var(--nav-color) 10%, transparent);
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 12px;
}

.nav-title {
  font-size: 14px;
  font-weight: 600;
  color: #303133;
  margin-bottom: 4px;
}

.nav-desc {
  font-size: 12px;
  color: #909399;
  line-height: 1.4;
}

.nav-arrow {
  position: absolute;
  bottom: 14px;
  right: 14px;
  opacity: 0;
  transition: opacity 0.2s;
}

.nav-card:hover .nav-arrow {
  opacity: 1;
}
</style>
