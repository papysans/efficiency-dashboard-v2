<template>
  <el-dialog
    :model-value="visible"
    title="AI 预估人天纠错"
    width="520px"
    @update:model-value="$emit('update:visible', $event)"
    @open="onOpen"
  >
    <el-form :model="form" label-width="90px">
      <el-form-item label="原始值">
        <el-input :model-value="rawDays != null ? Number(rawDays).toFixed(1) : '-'" disabled />
      </el-form-item>
      <el-form-item label="纠正值">
        <el-input-number v-model="form.value" :min="0" :precision="1" style="width: 100%" />
      </el-form-item>
      <el-form-item label="纠正原因">
        <el-input v-model="form.reason" type="textarea" :rows="3" placeholder="请输入纠正原因（必填）" />
      </el-form-item>
      <el-form-item label="操作人">
        <el-input v-model="form.operator" placeholder="请输入操作人（必填）" />
      </el-form-item>
    </el-form>

    <el-divider />

    <div class="history-section">
      <div class="history-title">纠错历史</div>
      <div v-loading="historyLoading">
        <el-table v-if="historyList.length > 0" :data="historyList" stripe border size="small" style="width: 100%">
          <el-table-column prop="created_at" label="时间" width="180" :formatter="(row, col, val) => formatLocalTime(val)" />
          <el-table-column prop="field" label="字段" width="100" />
          <el-table-column prop="old_value" label="旧值" width="80" align="right" />
          <el-table-column prop="new_value" label="新值" width="80" align="right" />
          <el-table-column prop="reason" label="原因" min-width="120" show-overflow-tooltip />
          <el-table-column prop="operator" label="操作人" width="90" />
        </el-table>
        <el-empty v-else-if="!historyLoading" description="暂无纠错记录" />
      </div>
    </div>

    <template #footer>
      <el-button @click="$emit('update:visible', false)">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="handleSubmit">确认提交</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { correctEfficiency, getEfficiencyHistory } from '@/api/es'
import { formatLocalTime } from '@/utils/formatters'

const props = defineProps({
  visible: { type: Boolean, default: false },
  dimension: { type: String, default: '' },
  dimensionId: { type: String, default: '' },
  rawDays: { type: Number, default: null },
  correctedDays: { type: Number, default: null },
  isCorrected: { type: Boolean, default: false },
  startDate: { type: String, default: '' },
  endDate: { type: String, default: '' },
})

const emit = defineEmits(['update:visible', 'corrected'])

const form = ref({ value: 0, reason: '', operator: '' })
const submitting = ref(false)
const historyLoading = ref(false)
const historyList = ref([])

function onOpen() {
  form.value = {
    value: props.correctedDays ?? props.rawDays ?? 0,
    reason: '',
    operator: '',
  }
  fetchHistory()
}

async function fetchHistory() {
  historyLoading.value = true
  try {
    const res = await getEfficiencyHistory({ dimension: props.dimension, id: props.dimensionId })
    historyList.value = res.data || res || []
    if (!Array.isArray(historyList.value)) historyList.value = []
  } catch (err) {
    console.error('获取纠错历史失败:', err)
    historyList.value = []
  } finally {
    historyLoading.value = false
  }
}

async function handleSubmit() {
  if (!form.value.reason?.trim()) {
    ElMessage.warning('请填写纠正原因')
    return
  }
  if (!form.value.operator?.trim()) {
    ElMessage.warning('请填写操作人')
    return
  }
  submitting.value = true
  try {
    await correctEfficiency({
      dimension: props.dimension,
      id: props.dimensionId,
      startDate: props.startDate,
      endDate: props.endDate,
      field: 'ai_estimated_days',
      value: form.value.value,
      reason: form.value.reason.trim(),
      by: form.value.operator.trim(),
    })
    ElMessage.success('纠错提交成功')
    emit('update:visible', false)
    emit('corrected')
  } catch (err) {
    console.error('纠错失败:', err)
    ElMessage.error('纠错失败')
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.history-section {
  margin-top: 4px;
}

.history-title {
  font-size: 14px;
  font-weight: bold;
  color: #303133;
  margin-bottom: 12px;
}
</style>
