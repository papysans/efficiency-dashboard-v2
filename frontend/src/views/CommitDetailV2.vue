<template>
  <div class="kb-panel">
    <!-- 标题栏 -->
    <el-card shadow="never">
      <div style="display: flex; align-items: center; gap: 12px">
        <el-button :icon="ArrowLeft" @click="router.back()">返回</el-button>
        <span style="font-size: 18px; font-weight: bold">Commit 详情</span>
        <el-button type="warning" size="small" @click="showManualDialog = true">人工调整</el-button>
      </div>
    </el-card>

    <!-- 基础信息 -->
    <el-card shadow="never" header="基础信息" v-loading="loading">
      <el-descriptions :column="3" border>
        <el-descriptions-item label="Commit ID">{{ commit.commit_id || '-' }}</el-descriptions-item>
        <el-descriptions-item label="用户">
          <el-link v-if="commit.user_id" type="primary" @click="router.push('/user/' + encodeURIComponent(commit.user_id))">{{ commit.user_name || commit.user_id }}</el-link>
          <span v-else>{{ commit.user_name || '-' }}</span>
        </el-descriptions-item>
        <el-descriptions-item label="Git 用户">{{ commit.git_user_name ? commit.git_user_name + ' <' + (commit.git_user_email || '') + '>' : '-' }}</el-descriptions-item>
        <el-descriptions-item label="仓库">
          <el-link v-if="commit.repo_addr" type="primary" @click="router.push('/repo/' + encodeURIComponent(commit.repo_addr) + '/' + encodeURIComponent(commit.repo_branch))">{{ commit.repo_addr }}{{ commit.repo_branch ? '#' + commit.repo_branch : '' }}</el-link>
          <span v-else>-</span>
        </el-descriptions-item>
        <el-descriptions-item label="分支">{{ commit.repo_branch || '-' }}</el-descriptions-item>
        <el-descriptions-item label="提交时间">{{ formatLocalTime(commit.commit_time) }}</el-descriptions-item>
        <el-descriptions-item label="提交说明" :span="3">{{ commit.comment || '-' }}</el-descriptions-item>
      </el-descriptions>
    </el-card>

    <!-- 度量信息 -->
    <el-card shadow="never" header="度量信息">
      <el-descriptions :column="3" border>
        <el-descriptions-item label="生成代码量">
          {{ commit.diff_lines ?? '-' }} 行
        </el-descriptions-item>
        <el-descriptions-item label="实际耗时">
          <template v-if="commit.commit_real_minutes_manual != null">
            {{ formatDuration(commit.commit_real_minutes_manual) }}
            <el-tooltip v-if="commit.commit_real_minutes_reason_manual" :content="commit.commit_real_minutes_reason_manual" placement="top" :show-after="200" popper-class="reason-tooltip">
              <el-icon style="margin-left: 4px; color: #E6A23C; cursor: help; vertical-align: middle;"><QuestionFilled /></el-icon>
            </el-tooltip>
            <span v-if="commit.commit_real_minutes != null || commit.commit_real_minutes_reason" style="margin-left: 8px; color: #C0C4CC; text-decoration: line-through; font-size: 12px;">
              {{ commit.commit_real_minutes != null ? formatDuration(commit.commit_real_minutes) : '(AI未出值)' }}
            </span>
            <el-tooltip v-if="commit.commit_real_minutes_reason" :content="commit.commit_real_minutes_reason" placement="top" :show-after="200" popper-class="reason-tooltip">
              <el-icon style="margin-left: 2px; color: #C0C4CC; cursor: help; vertical-align: middle; font-size: 12px;"><QuestionFilled /></el-icon>
            </el-tooltip>
          </template>
          <template v-else>
            {{ formatDuration(commit.commit_real_minutes) }}
            <el-tooltip :content="realMinutesExplain" placement="top" :show-after="200" popper-class="reason-tooltip">
              <el-icon style="margin-left: 4px; color: #909399; cursor: help; vertical-align: middle;"><QuestionFilled /></el-icon>
            </el-tooltip>
          </template>
        </el-descriptions-item>
        <el-descriptions-item label="传统开发时长预估">
          <template v-if="commit.commit_ancient_minutes_manual != null">
            {{ formatDuration(commit.commit_ancient_minutes_manual) }}
            <el-tooltip v-if="commit.commit_ancient_minutes_reason_manual" :content="commit.commit_ancient_minutes_reason_manual" placement="top" :show-after="200" popper-class="reason-tooltip">
              <el-icon style="margin-left: 4px; color: #E6A23C; cursor: help; vertical-align: middle;"><QuestionFilled /></el-icon>
            </el-tooltip>
            <span v-if="commit.commit_ancient_minutes != null || commit.commit_ancient_minutes_reason" style="margin-left: 8px; color: #C0C4CC; text-decoration: line-through; font-size: 12px;">
              {{ commit.commit_ancient_minutes != null ? formatDuration(commit.commit_ancient_minutes) : '(AI未出值)' }}
            </span>
            <el-tooltip v-if="commit.commit_ancient_minutes_reason" :content="commit.commit_ancient_minutes_reason" placement="top" :show-after="200" popper-class="reason-tooltip">
              <el-icon style="margin-left: 2px; color: #C0C4CC; cursor: help; vertical-align: middle; font-size: 12px;"><QuestionFilled /></el-icon>
            </el-tooltip>
          </template>
          <template v-else>
            {{ formatDuration(commit.commit_ancient_minutes) }}
            <el-tooltip v-if="commit.commit_ancient_minutes_reason" :content="commit.commit_ancient_minutes_reason" placement="top" :show-after="200" popper-class="reason-tooltip">
              <el-icon style="margin-left: 4px; color: #909399; cursor: help; vertical-align: middle;"><QuestionFilled /></el-icon>
            </el-tooltip>
          </template>
        </el-descriptions-item>
        <el-descriptions-item label="提效比例">
          <span :style="{ color: efficiencyColor, fontSize: '20px', fontWeight: 'bold' }">
            {{ commit.efficiency_ratio != null ? Math.round(commit.efficiency_ratio) + '%' : '-' }}
          </span>
        </el-descriptions-item>
        <el-descriptions-item label="硅含量">
          <span v-if="silica != null" style="font-size: 16px; font-weight: bold; color: #67C23A">
            {{ silica }}%
          </span>
          <span v-else style="color: #909399">-</span>
          <el-tooltip content="commit 中由 AI Task 生成的代码占比，基于关联 Task 的 diff 行数加权计算" placement="top" :show-after="200">
            <el-icon style="margin-left: 4px; color: #909399; cursor: help; vertical-align: middle;"><QuestionFilled /></el-icon>
          </el-tooltip>
        </el-descriptions-item>
        <el-descriptions-item label="总Tokens">
          <el-tooltip :content="'上行: ' + upstreamTokens.toLocaleString() + ' / 下行: ' + downstreamTokens.toLocaleString()" placement="top">
            <span>{{ (upstreamTokens + downstreamTokens) > 0 ? (upstreamTokens + downstreamTokens).toLocaleString() : '-' }}</span>
          </el-tooltip>
        </el-descriptions-item>
        <el-descriptions-item label="费用">
          {{ totalCost > 0 ? totalCost.toFixed(2) + ' 元' : '-' }}
        </el-descriptions-item>
      </el-descriptions>
    </el-card>

    <!-- 关联 Tasks -->
    <el-card v-if="relatedTasks && relatedTasks.length > 0" shadow="never">
      <template #header><span>关联 Tasks</span></template>
      <el-table :data="relatedTasks" style="width: 100%">
        <el-table-column prop="task_id" label="Task ID" min-width="200" show-overflow-tooltip>
          <template #default="{ row }">
            <el-link type="primary" @click="router.push('/task/' + row.task_id)">{{ row.task_id }}</el-link>
          </template>
        </el-table-column>
        <el-table-column prop="user_name" label="用户" min-width="90" />
        <el-table-column prop="start_time" label="开始时间" min-width="140" :formatter="(row, col, val) => formatLocalTime(val)" />
        <el-table-column prop="diff_lines" label="代码行数" min-width="90" align="right" />
        <el-table-column prop="task_real_minutes" label="实际耗时" min-width="100" align="right" :formatter="(row, col, val) => formatDuration(val)" />
        <el-table-column label="硅含量" min-width="80" align="center">
          <template #default="{ row }">
            <el-tag v-if="row.silica != null" :type="row.silica >= 0.8 ? 'success' : row.silica >= 0.5 ? 'primary' : 'info'" size="small">
              {{ (row.silica * 100).toFixed(1) }}%
            </el-tag>
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column label="费用" min-width="80" align="right">
          <template #default="{ row }">{{ row.cost != null && row.cost > 0 ? row.cost.toFixed(2) : '-' }}</template>
        </el-table-column>
      </el-table>
    </el-card>
    <el-empty v-else-if="!loading" description="暂无关联 Task" />


    <el-dialog v-model="showManualDialog" title="人工调整" width="600px">
      <el-form label-width="160px">
        <el-form-item label="传统开发时长预估(分钟)">
          <el-input-number v-model="manualForm.commit_ancient_minutes_manual" :precision="2" :step="10" />
        </el-form-item>
        <el-form-item label="传统开发时长预估理由">
          <el-input v-model="manualForm.commit_ancient_minutes_reason_manual" type="textarea" :rows="2" />
        </el-form-item>
        <el-form-item label="实际耗时(分钟)">
          <el-input-number v-model="manualForm.commit_real_minutes_manual" :precision="2" :step="10" />
        </el-form-item>
        <el-form-item label="实际耗时理由">
          <el-input v-model="manualForm.commit_real_minutes_reason_manual" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showManualDialog = false">取消</el-button>
        <el-button type="primary" @click="submitManual" :loading="manualSubmitting">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ArrowLeft, QuestionFilled } from '@element-plus/icons-vue'
import { getCommitDetailV2, updateCommitManualV2 } from '@/api/es'
import { formatLocalTime, formatDuration } from '@/utils/formatters'
import { getEfficiencyColor } from '@/utils/commit-helpers'
import { ElMessage } from 'element-plus'

const router = useRouter()
const route = useRoute()

const loading = ref(false)
const commit = ref({})
const relatedTasks = ref([])
const silica = ref(null)
const totalCost = ref(0)
const upstreamTokens = ref(0)
const downstreamTokens = ref(0)

const efficiencyColor = computed(() => getEfficiencyColor(commit.value.efficiency_ratio))

// 实际耗时计算说明
const realMinutesExplain = computed(() => {
  if (commit.value.commit_real_minutes_reason) return commit.value.commit_real_minutes_reason
  const tasks = relatedTasks.value
  if (!tasks || tasks.length === 0) return '无关联 Task'
  const parts = tasks.map(t => {
    const real = t.task_real_minutes ?? 0
    const s = t.silica ?? 0
    return `${formatDuration(real)} × ${(s * 100).toFixed(0)}%`
  })
  return '计算方式：Σ(Task实际耗时 × 硅含量)\n' + parts.join(' + ')
})

// 人工调整
const showManualDialog = ref(false)
const manualSubmitting = ref(false)
const manualForm = ref({
  commit_ancient_minutes_manual: null,
  commit_ancient_minutes_reason_manual: '',
  commit_real_minutes_manual: null,
  commit_real_minutes_reason_manual: ''
})

watch(showManualDialog, (val) => {
  if (val) {
    manualForm.value.commit_ancient_minutes_manual = commit.value.commit_ancient_minutes_manual || commit.value.commit_ancient_minutes || null
    manualForm.value.commit_ancient_minutes_reason_manual = commit.value.commit_ancient_minutes_reason_manual || ''
    manualForm.value.commit_real_minutes_manual = commit.value.commit_real_minutes_manual || commit.value.commit_real_minutes || null
    manualForm.value.commit_real_minutes_reason_manual = commit.value.commit_real_minutes_reason_manual || ''
  }
})

async function submitManual() {
  manualSubmitting.value = true
  try {
    const commitId = route.params.commitId
    await updateCommitManualV2(commitId, {
      commit_ancient_minutes_manual: manualForm.value.commit_ancient_minutes_manual,
      commit_ancient_minutes_reason_manual: manualForm.value.commit_ancient_minutes_reason_manual,
      commit_real_minutes_manual: manualForm.value.commit_real_minutes_manual,
      commit_real_minutes_reason_manual: manualForm.value.commit_real_minutes_reason_manual
    })
    ElMessage.success('人工调整已保存')
    showManualDialog.value = false
    await loadData()
  } catch (e) {
    ElMessage.error('保存失败: ' + e.message)
  } finally {
    manualSubmitting.value = false
  }
}

async function loadData() {
  const commitId = route.params.commitId
  if (!commitId) return

  loading.value = true
  try {
    const result = await getCommitDetailV2(commitId)
    const data = result.data || result
    const c = data.commit || {}
    if (data.efficiency_ratio != null) c.efficiency_ratio = data.efficiency_ratio
    commit.value = c
    relatedTasks.value = data.related_tasks || []
    silica.value = data.silica ?? null
    totalCost.value = data.total_cost ?? 0
    upstreamTokens.value = data.upstream_tokens ?? 0
    downstreamTokens.value = data.downstream_tokens ?? 0
  } catch (err) {
    commit.value = {}
    relatedTasks.value = []
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadData()
})
</script>

<style scoped>
</style>

<style>
.reason-tooltip {
  max-width: 400px !important;
}
</style>
