<template>
  <div class="kb-panel">
    <!-- 标题栏 -->
    <el-card shadow="never">
      <div style="display: flex; align-items: center; gap: 12px">
        <el-button :icon="ArrowLeft" @click="router.back()">返回</el-button>
        <span style="font-size: 18px; font-weight: bold">Task 详情</span>
        <el-button type="warning" size="small" @click="showManualDialog = true">人工调整</el-button>
      </div>
    </el-card>

    <!-- 基础信息 -->
    <el-card shadow="never" header="基础信息" v-loading="loading">
      <el-descriptions :column="3" border>
        <el-descriptions-item label="Task ID">{{ task.task_id || '-' }}</el-descriptions-item>
        <el-descriptions-item label="任务描述" :span="2">{{ task.title || '-' }}</el-descriptions-item>
        <el-descriptions-item label="用户">
          <el-link v-if="task.user_id" type="primary" @click="router.push('/user/' + encodeURIComponent(task.user_id))">{{ task.user_name || task.user_id }}</el-link>
          <span v-else>{{ task.user_name || '-' }}</span>
        </el-descriptions-item>
        <el-descriptions-item label="仓库">
          <el-link v-if="repoDisplay !== '-'" type="primary" @click="router.push('/repo/' + encodeURIComponent(task.repo_addr) + '/' + encodeURIComponent(task.repo_branch))">{{ repoDisplay }}</el-link>
          <span v-else>-</span>
        </el-descriptions-item>
        <el-descriptions-item label="工作目录">
          <el-link v-if="task.work_dir_id" type="primary" @click="router.push('/workdir/' + encodeURIComponent(task.work_dir_id))">{{ task.work_dir || task.work_dir_id }}</el-link>
          <span v-else>-</span>
        </el-descriptions-item>
        <el-descriptions-item label="开始时间">{{ formatLocalTime(task.start_time) }}</el-descriptions-item>
        <el-descriptions-item label="结束时间">{{ formatLocalTime(task.end_time) }}</el-descriptions-item>
        <el-descriptions-item label="系统">{{ task.client_os ? (task.client_os + ' ' + (task.client_os_version || '')) : '-' }}</el-descriptions-item>
        <el-descriptions-item label="客户端">{{ task.client_ide ? (task.client_ide + ' ' + (task.client_version || '')) : '-' }}</el-descriptions-item>
        <el-descriptions-item label="模式">{{ task.caller || '-' }}</el-descriptions-item>
      </el-descriptions>
    </el-card>

    <!-- 度量信息 -->
    <el-card shadow="never" header="度量信息">
      <el-descriptions :column="3" border>
        <el-descriptions-item label="生成代码量">
          {{ task.diff_lines ?? '-' }} 行
          <el-link type="primary" :href="getTaskFileUrl('summary')" target="_blank" style="margin-left: 8px">查看详情</el-link>
        </el-descriptions-item>
        <el-descriptions-item label="实际耗时">
          <template v-if="task.task_real_minutes_manual != null">
            {{ formatDuration(task.task_real_minutes_manual) }}
            <el-tooltip v-if="task.task_real_minutes_reason_manual" :content="task.task_real_minutes_reason_manual" placement="top" :show-after="200" popper-class="reason-tooltip">
              <el-icon style="margin-left: 4px; color: #E6A23C; cursor: help; vertical-align: middle;"><QuestionFilled /></el-icon>
            </el-tooltip>
            <span v-if="task.task_real_minutes != null || task.task_real_minutes_reason" style="margin-left: 8px; color: #C0C4CC; text-decoration: line-through; font-size: 12px;">
              {{ task.task_real_minutes != null ? formatDuration(task.task_real_minutes) : '(AI未出值)' }}
            </span>
            <el-tooltip v-if="task.task_real_minutes_reason" :content="task.task_real_minutes_reason" placement="top" :show-after="200" popper-class="reason-tooltip">
              <el-icon style="margin-left: 2px; color: #C0C4CC; cursor: help; vertical-align: middle; font-size: 12px;"><QuestionFilled /></el-icon>
            </el-tooltip>
          </template>
          <template v-else>
            {{ formatDuration(task.task_real_minutes) }}
            <el-tooltip v-if="task.task_real_minutes_reason" :content="task.task_real_minutes_reason" placement="top" :show-after="200" popper-class="reason-tooltip">
              <el-icon style="margin-left: 4px; color: #909399; cursor: help; vertical-align: middle;"><QuestionFilled /></el-icon>
            </el-tooltip>
          </template>
        </el-descriptions-item>
        <el-descriptions-item label="传统开发时长预估">
          <template v-if="task.task_ancient_minutes_manual != null">
            {{ formatDuration(task.task_ancient_minutes_manual) }}
            <el-tooltip v-if="task.task_ancient_minutes_reason_manual" :content="task.task_ancient_minutes_reason_manual" placement="top" :show-after="200" popper-class="reason-tooltip">
              <el-icon style="margin-left: 4px; color: #E6A23C; cursor: help; vertical-align: middle;"><QuestionFilled /></el-icon>
            </el-tooltip>
            <span v-if="task.task_ancient_minutes != null || task.task_ancient_minutes_reason" style="margin-left: 8px; color: #C0C4CC; text-decoration: line-through; font-size: 12px;">
              {{ task.task_ancient_minutes != null ? formatDuration(task.task_ancient_minutes) : '(AI未出值)' }}
            </span>
            <el-tooltip v-if="task.task_ancient_minutes_reason" :content="task.task_ancient_minutes_reason" placement="top" :show-after="200" popper-class="reason-tooltip">
              <el-icon style="margin-left: 2px; color: #C0C4CC; cursor: help; vertical-align: middle; font-size: 12px;"><QuestionFilled /></el-icon>
            </el-tooltip>
          </template>
          <template v-else>
            {{ formatDuration(task.task_ancient_minutes) }}
            <el-tooltip v-if="task.task_ancient_minutes_reason" :content="task.task_ancient_minutes_reason" placement="top" :show-after="200" popper-class="reason-tooltip">
              <el-icon style="margin-left: 4px; color: #909399; cursor: help; vertical-align: middle;"><QuestionFilled /></el-icon>
            </el-tooltip>
          </template>
        </el-descriptions-item>
        <el-descriptions-item label="API请求次数">{{ conversations.length || '-' }}</el-descriptions-item>
        <el-descriptions-item label="总Tokens">
          <el-tooltip :content="'上行: ' + totalUpstreamTokens.toLocaleString() + ' / 下行: ' + totalDownstreamTokens.toLocaleString()" placement="top">
            <span>{{ totalTokens > 0 ? totalTokens.toLocaleString() : '-' }}</span>
          </el-tooltip>
        </el-descriptions-item>
        <el-descriptions-item label="费用">{{ task.cost != null && task.cost > 0 ? fmtCostVal(task.cost) + ' 元' : (totalCostSum > 0 ? fmtCostVal(totalCostSum) + ' 元' : '-') }}</el-descriptions-item>
        <el-descriptions-item label="提效比例">
          <span :style="{ color: efficiencyColor, fontSize: '20px', fontWeight: 'bold' }">
            {{ task.efficiency_ratio != null ? Math.round(task.efficiency_ratio) + '%' : '-' }}
          </span>
        </el-descriptions-item>
      </el-descriptions>
    </el-card>

    <!-- 对话历史 -->
    <el-card v-if="conversations.length > 0" shadow="never">
      <template #header>
        <span>对话历史</span>
        <el-link type="primary" :href="getTaskFileUrl('conversation')" target="_blank" style="margin-left: 12px">查看原始数据</el-link>
      </template>
      <el-timeline>
        <template v-for="(item, idx) in timelineItems" :key="idx">
          <!-- 间隔节点 -->
          <el-timeline-item v-if="item.type === 'gap'" color="#C0C4CC" :timestamp="'间隔 ' + item.gapMinutes + ' 分钟（不计入耗时）'" placement="top">
          </el-timeline-item>
          <!-- 对话节点 -->
          <el-timeline-item v-else :color="item.isSegmentStart ? '#E6A23C' : '#67C23A'" :timestamp="formatLocalTime(item.conv.start_time)" placement="top">
            <el-card shadow="never">
              <!-- 时间行 -->
              <div style="color: #606266; font-size: 13px; margin-bottom: 6px">
                {{ formatLocalTime(item.conv.start_time) }} ~ {{ formatLocalTime(item.conv.end_time) }}
                | 耗时 {{ item.conv.process_time ?? '-' }} ms
                | TTFT {{ item.conv.process_ttft ?? '-' }} ms
              </div>
              <!-- 模式行 -->
              <div style="color: #909399; font-size: 13px; margin-bottom: 6px">
                {{ item.conv.prompt_mode || '-' }} | {{ item.conv.mode || '-' }} | {{ item.conv.model || '-' }}
                <el-tag v-if="item.conv.error_code" type="danger" size="small" style="margin-left: 8px">{{ item.conv.error_code }}: {{ item.conv.error_reason }}</el-tag>
              </div>
              <!-- 指标行 -->
              <div style="color: #909399; font-size: 13px; margin-bottom: 6px">
                上行 {{ item.conv.upstream_tokens ?? '-' }} | 下行 {{ item.conv.downstream_tokens ?? '-' }} | 费用 {{ fmtCostVal(item.conv.cost) }} | 代码 {{ item.conv.diff_lines ?? '-' }} 行
              </div>
              <!-- 输入行 -->
              <div v-if="item.conv.user_input">
                <pre class="task-detail-content">{{ getDisplayText(item.conv.user_input, item.originalIndex, 'input') }}</pre>
                <el-button v-if="item.conv.user_input.length > 200" text type="primary" size="small" @click="toggleExpand(item.originalIndex, 'input')">
                  {{ isExpanded(item.originalIndex, 'input') ? '收起' : '展开全文' }}
                </el-button>
              </div>
            </el-card>
          </el-timeline-item>
        </template>
      </el-timeline>
    </el-card>

    <el-empty v-if="!loading && conversations.length === 0 && task.task_id" description="暂无对话记录" />


    <el-dialog v-model="showManualDialog" title="人工调整" width="600px">
      <el-form label-width="160px">
        <el-form-item label="实际耗时(分钟)">
          <el-input-number v-model="manualForm.task_real_minutes_manual" :precision="2" :step="10" />
        </el-form-item>
        <el-form-item label="实际耗时理由">
          <el-input v-model="manualForm.task_real_minutes_reason_manual" type="textarea" :rows="2" />
        </el-form-item>
        <el-form-item label="传统开发时长预估(分钟)">
          <el-input-number v-model="manualForm.task_ancient_minutes_manual" :precision="2" :step="10" />
        </el-form-item>
        <el-form-item label="传统开发时长预估理由">
          <el-input v-model="manualForm.task_ancient_minutes_reason_manual" type="textarea" :rows="2" />
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
import { getTaskDetailV2, updateTaskManualV2 } from '@/api/es'
import { fmtCost, formatLocalTime, formatDuration } from '@/utils/formatters'
import { ElMessage } from 'element-plus'

const router = useRouter()
const route = useRoute()

const loading = ref(false)
const task = ref({})
const conversations = ref([])
const expandState = ref({})

function fmtCostVal(val) {
  return fmtCost(null, null, val)
}

function getTaskFileUrl(type) {
  if (!task.value.start_time) return ''
  const d = new Date(task.value.start_time)
  const date = d.getFullYear() + '-' + String(d.getMonth() + 1).padStart(2, '0') + '-' + String(d.getDate()).padStart(2, '0')
  return `/api/v2/tasks/file?type=${type}&taskId=${task.value.task_id}&date=${date}`
}

const repoDisplay = computed(() => {
  if (task.value.repo_addr && task.value.repo_branch) {
    return task.value.repo_addr + '#' + task.value.repo_branch
  }
  if (task.value.repo_id && task.value.repo_id !== task.value.work_dir_id) {
    return task.value.repo_id
  }
  return '-'
})

const totalTokens = computed(() => {
  return conversations.value.reduce((s, c) => s + (c.upstream_tokens || 0) + (c.downstream_tokens || 0), 0)
})

const totalUpstreamTokens = computed(() => {
  return conversations.value.reduce((s, c) => s + (c.upstream_tokens || 0), 0)
})

const totalDownstreamTokens = computed(() => {
  return conversations.value.reduce((s, c) => s + (c.downstream_tokens || 0), 0)
})

const totalCostSum = computed(() => {
  return conversations.value.reduce((s, c) => s + (c.cost || 0), 0)
})

const efficiencyColor = computed(() => {
  const ratio = task.value.efficiency_ratio
  if (ratio == null) return '#909399'
  if (ratio >= 300) return '#67C23A'
  if (ratio >= 150) return '#409EFF'
  return '#909399'
})

const timelineItems = computed(() => {
  const convs = conversations.value
  const segments = task.value.time_segments || []
  if (!convs.length) return []

  const items = []
  let lastSegIdx = -1

  for (let i = 0; i < convs.length; i++) {
    const conv = convs[i]
    let segIdx = 0
    if (segments.length > 0 && conv.start_time) {
      const ct = new Date(conv.start_time).getTime()
      for (let s = 0; s < segments.length; s++) {
        const segStart = new Date(segments[s].start).getTime()
        const segEnd = new Date(segments[s].end).getTime()
        if (ct >= segStart && ct <= segEnd) {
          segIdx = s
          break
        }
        if (ct > segEnd && s < segments.length - 1) continue
        segIdx = s
      }
    }

    const isNewSegment = segIdx !== lastSegIdx && lastSegIdx !== -1

    if (isNewSegment && segments.length > 0) {
      const prevEnd = new Date(segments[lastSegIdx].end)
      const currStart = new Date(segments[segIdx].start)
      const gapMinutes = Math.round((currStart - prevEnd) / 60000)
      items.push({
        type: 'gap',
        gapMinutes,
      })
    }

    items.push({
      type: 'conv',
      conv,
      originalIndex: i,
      segmentIndex: segIdx,
      isSegmentStart: isNewSegment,
    })

    lastSegIdx = segIdx
  }

  return items
})

function expandKey(idx, type) {
  return idx + '_' + type
}

function isExpanded(idx, type) {
  return !!expandState.value[expandKey(idx, type)]
}

function toggleExpand(idx, type) {
  const key = expandKey(idx, type)
  expandState.value[key] = !expandState.value[key]
}

function getDisplayText(text, idx, type) {
  if (!text) return ''
  if (text.length <= 200 || isExpanded(idx, type)) return text
  return text.substring(0, 200) + '...'
}

// 人工调整
const showManualDialog = ref(false)
const manualSubmitting = ref(false)
const manualForm = ref({
  task_real_minutes_manual: null,
  task_real_minutes_reason_manual: '',
  task_ancient_minutes_manual: null,
  task_ancient_minutes_reason_manual: ''
})

watch(showManualDialog, (val) => {
  if (val) {
    manualForm.value.task_real_minutes_manual = task.value.task_real_minutes_manual || task.value.task_real_minutes || null
    manualForm.value.task_real_minutes_reason_manual = task.value.task_real_minutes_reason_manual || ''
    manualForm.value.task_ancient_minutes_manual = task.value.task_ancient_minutes_manual || task.value.task_ancient_minutes || null
    manualForm.value.task_ancient_minutes_reason_manual = task.value.task_ancient_minutes_reason_manual || ''
  }
})

async function submitManual() {
  manualSubmitting.value = true
  try {
    await updateTaskManualV2(route.params.taskId, {
      task_real_minutes_manual: manualForm.value.task_real_minutes_manual,
      task_real_minutes_reason_manual: manualForm.value.task_real_minutes_reason_manual,
      task_ancient_minutes_manual: manualForm.value.task_ancient_minutes_manual,
      task_ancient_minutes_reason_manual: manualForm.value.task_ancient_minutes_reason_manual
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
  const taskId = route.params.taskId
  if (!taskId) return

  loading.value = true
  try {
    const result = await getTaskDetailV2(taskId)
    const data = result.data || result
    const t = data.task || {}
    // efficiency_ratio 和 time_segments 在顶层返回，合并到 task 对象中
    if (data.efficiency_ratio != null) t.efficiency_ratio = data.efficiency_ratio
    if (data.time_segments) t.time_segments = data.time_segments
    task.value = t
    conversations.value = data.conversations || []
  } catch (err) {
    task.value = {}
    conversations.value = []
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadData()
})
</script>

<style scoped>
.task-detail-content {
  white-space: pre-wrap;
  word-break: break-all;
  background: #f5f7fa;
  padding: 8px 12px;
  border-radius: 4px;
  font-size: 13px;
  line-height: 1.6;
  margin: 0;
  max-height: 600px;
  overflow-y: auto;
}
</style>

<style>
.reason-tooltip {
  max-width: 400px !important;
}
</style>
