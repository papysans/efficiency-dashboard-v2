<template>
  <div class="kb-panel">
    <!-- 标题栏 -->
    <el-card shadow="never">
      <div style="display: flex; align-items: center; gap: 12px">
        <el-button :icon="ArrowLeft" @click="router.back()">返回</el-button>
        <span style="font-size: 18px; font-weight: bold">工作目录详情: {{ detailData?.repo_addr || detailData?.repo_id || '-' }}</span>
      </div>
    </el-card>

    <!-- 仓库概览 -->
    <el-card shadow="never" v-loading="loading">
      <el-descriptions :column="3" border>
        <el-descriptions-item label="仓库地址">{{ detailData?.repo_addr || '-' }}</el-descriptions-item>
        <el-descriptions-item label="分支">{{ detailData?.repo_branch || '-' }}</el-descriptions-item>
        <el-descriptions-item label="用户数">{{ detailData?.summary?.user_count ?? '-' }}</el-descriptions-item>
        <el-descriptions-item label="关联Task数">{{ detailData?.summary?.task_count ?? '-' }}</el-descriptions-item>
        <el-descriptions-item label="关联Commit数">{{ detailData?.summary?.commit_count ?? '-' }}</el-descriptions-item>
        <el-descriptions-item label="总费用">{{ detailData?.summary?.total_cost != null ? fmtCostVal(detailData.summary.total_cost) : '-' }}</el-descriptions-item>
        <el-descriptions-item label="传统开发时长预估">{{ detailData?.summary?.task_ancient_minutes != null ? formatDuration(detailData.summary.task_ancient_minutes) : '-' }}</el-descriptions-item>
      </el-descriptions>
    </el-card>

    <!-- Commit 列表 -->
    <el-card v-if="detailData?.commits && detailData.commits.length > 0" shadow="never">
      <template #header><span>Commit 列表</span></template>
      <el-table :data="detailData.commits" stripe border style="width: 100%" row-key="commit_id">
        <el-table-column type="expand">
          <template #default="{ row }">
            <div style="padding: 12px 20px">
              <!-- 硅含量分析理由 -->
              <div v-if="row.silica_reason" style="margin-bottom: 12px">
                <strong>硅含量分析理由：</strong>
                <span>{{ row.silica_reason }}</span>
              </div>
              <!-- 关联 Tasks 子表格 -->
              <div v-if="row.matched_tasks && row.matched_tasks.length > 0">
                <strong>关联 Tasks：</strong>
                <el-table :data="row.matched_tasks" stripe border size="small" style="margin-top: 8px">
                  <el-table-column prop="task_id" label="Task ID" min-width="200">
                    <template #default="{ row: taskRow }">
                      <el-link type="primary" @click.stop="router.push('/task/' + taskRow.task_id)">{{ taskRow.task_id }}</el-link>
                    </template>
                  </el-table-column>
                  <el-table-column prop="user_name" label="用户" width="150" />
                  <el-table-column label="硅比例" width="120" align="right">
                    <template #default="{ row: taskRow }">
                      <el-progress :percentage="Math.round((taskRow.silica || 0) * 100)" :color="silicaColor(taskRow.silica || 0)" :stroke-width="10" :show-text="true" style="width: 100px" />
                    </template>
                  </el-table-column>
                </el-table>
              </div>
              <div v-else style="color: #909399;">暂无关联 Task</div>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="commit_id" label="Commit ID" width="180" show-overflow-tooltip sortable />
        <el-table-column prop="git_user_name" label="提交者" width="150" sortable />
        <el-table-column prop="commit_time" label="提交时间" width="180" show-overflow-tooltip sortable :formatter="(row, col, val) => formatLocalTime(val)" />
        <el-table-column prop="diff_lines" label="Diff行数" width="100" align="right" sortable />
        <el-table-column label="硅含量" width="150" align="center" sortable :sort-method="(a, b) => (a.silica || 0) - (b.silica || 0)">
          <template #default="{ row }">
            <el-progress :percentage="Math.round((row.silica || 0) * 100)" :color="silicaColor(row.silica || 0)" :stroke-width="10" :show-text="true" style="width: 120px" />
          </template>
        </el-table-column>
        <el-table-column label="关联Task数" width="100" align="right" sortable :sort-method="(a, b) => (a.matched_tasks?.length || 0) - (b.matched_tasks?.length || 0)">
          <template #default="{ row }">
            {{ row.matched_tasks ? row.matched_tasks.length : 0 }}
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 参与者列表 -->
    <el-card v-if="participants.length > 0" shadow="never">
      <template #header><span>参与者列表</span></template>
      <el-table :data="participants" stripe border style="width: 100%" empty-text="暂无数据">
        <el-table-column prop="user_name" label="用户名" width="200">
          <template #default="{ row }">
            <el-link type="primary" @click.stop="router.push('/user/' + row.user_id)">{{ row.user_name || row.user_id }}</el-link>
          </template>
        </el-table-column>
        <el-table-column prop="task_count" label="Task数" width="100" align="right" sortable />
        <el-table-column prop="commit_count" label="Commit数" width="120" align="right" sortable />
      </el-table>
    </el-card>

    <!-- 硅比例图表 -->
    <el-card v-if="silicaChartData.length > 0" shadow="never">
      <div ref="silicaChartRef" class="kb-chart-container"></div>
    </el-card>

  </div>
</template>

<script setup>
import { ref, computed, onMounted, nextTick } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ArrowLeft } from '@element-plus/icons-vue'
import { useChart } from '@/composables/useChart'
import { getRepoDetailV2New } from '@/api/es'
import { fmtCost, fmtDays, formatDuration, formatLocalTime } from '@/utils/formatters'
import { createBarOption } from '@/utils/chart'

const router = useRouter()
const route = useRoute()

const loading = ref(false)
const detailData = ref(null)

const workDirId = computed(() => {
  if (route.params.workDirId) return decodeURIComponent(route.params.workDirId)
  return ''
})

function fmtCostVal(val) {
  return fmtCost(null, null, val)
}

function fmtDaysVal(val) {
  return fmtDays(null, null, val)
}

function silicaColor(silica) {
  const pct = silica * 100
  if (pct >= 80) return '#67C23A'
  if (pct >= 50) return '#409EFF'
  return '#E6A23C'
}

// 参与者聚合
const participants = computed(() => {
  if (!detailData.value?.tasks) return []
  const map = {}
  detailData.value.tasks.forEach(t => {
    const uid = t.user_id || 'unknown'
    if (!map[uid]) map[uid] = { user_id: uid, user_name: t.user_name || uid, task_count: 0, commit_count: 0 }
    map[uid].task_count++
  })
  if (detailData.value?.commits) {
    const nameToUid = {}
    detailData.value.tasks.forEach(t => {
      if (t.user_name) nameToUid[t.user_name] = t.user_id || 'unknown'
    })
    detailData.value.commits.forEach(c => {
      const gitName = c.git_user_name
      const uid = nameToUid[gitName]
      if (uid && map[uid]) {
        map[uid].commit_count++
      }
    })
  }
  return Object.values(map)
})

// 硅比例图表数据
const silicaChartData = computed(() => {
  if (!detailData.value?.silica_entries) return []
  return detailData.value.silica_entries
    .filter(e => e && e.silica != null)
    .map(e => ({ name: e.task_id, value: e.silica }))
})

const silicaChartRef = ref(null)
const { setOption: setSilicaOption } = useChart(silicaChartRef)

async function loadDetail(id) {
  loading.value = true
  try {
    const result = await getRepoDetailV2New(id, '', {})
    detailData.value = result.data || result

    await nextTick()
    if (silicaChartData.value.length > 0) {
      setSilicaOption(createBarOption(
        '硅比例（按Task）',
        silicaChartData.value,
        '#409EFF',
        (v) => (v * 100).toFixed(1) + '%'
      ))
    }
  } catch (err) {
    detailData.value = null
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  if (workDirId.value) {
    loadDetail(workDirId.value)
  }
})
</script>
