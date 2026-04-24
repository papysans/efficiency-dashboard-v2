<template>
  <div class="drp-wrapper">
    <el-popover
      v-model:visible="popoverVisible"
      trigger="manual"
      placement="bottom-start"
      :width="620"
      popper-class="drp-popover"
      @click-outside="popoverVisible = false"
    >
      <template #reference>
        <div class="drp-trigger" :class="[`drp-trigger--${size}`]" @click="popoverVisible = !popoverVisible">
          <el-icon class="drp-icon-calendar"><Calendar /></el-icon>
          <span class="drp-display" :class="{ 'drp-placeholder': !displayText }">
            {{ displayText || placeholder }}
          </span>
          <el-icon v-if="clearable && modelValue?.length === 2" class="drp-icon-clear" @click.stop="clearValue"><CircleClose /></el-icon>
        </div>
      </template>

      <!-- Popover 内容 -->
      <div class="drp-panel">
        <!-- 左侧快捷按钮 -->
        <div class="drp-shortcuts">
          <button
            v-for="sc in shortcuts"
            :key="sc.label"
            class="drp-shortcut-btn"
            :class="{ 'is-active': activeShortcut === sc.label }"
            @click="applyShortcut(sc)"
          >
            {{ sc.label }}
          </button>
        </div>
        <!-- 右侧日历 -->
        <div class="drp-calendar">
          <el-date-picker
            v-model="internalValue"
            type="daterange"
            :inline="true"
            @change="onCalendarChange"
          />
        </div>
      </div>
      <!-- 底部 -->
      <div class="drp-footer">
        <span class="drp-footer-text">{{ displayText || placeholder }}</span>
        <el-icon v-if="clearable && modelValue?.length === 2" class="drp-footer-clear" @click="clearValue"><CircleClose /></el-icon>
      </div>
    </el-popover>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { Calendar, CircleClose } from '@element-plus/icons-vue'

const props = defineProps({
  modelValue: { type: Array, default: null },
  clearable: { type: Boolean, default: false },
  placeholder: { type: String, default: '选择日期范围' },
  size: { type: String, default: 'default' },
})

const emit = defineEmits(['update:modelValue', 'change'])

const popoverVisible = ref(false)
const activeShortcut = ref('')

const shortcuts = [
  { label: 'Today', days: 0 },
  { label: '1 day ago', days: 1 },
  { label: '3 days ago', days: 3 },
  { label: '1 week ago', days: 7 },
  { label: '1 month ago', days: 30 },
  { label: '3 months ago', days: 90 },
]

function fmtDate(d) {
  const pad = n => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

const displayText = computed(() => {
  if (!props.modelValue || props.modelValue.length !== 2) return ''
  return `${props.modelValue[0]}  To  ${props.modelValue[1]}`
})

const internalValue = computed({
  get: () => {
    if (!props.modelValue || props.modelValue.length !== 2) return null
    return [new Date(props.modelValue[0]), new Date(props.modelValue[1])]
  },
  set: () => {},
})

function applyShortcut(sc) {
  activeShortcut.value = sc.label
  const end = new Date()
  const start = new Date()
  if (sc.days === 0) {
    start.setHours(0, 0, 0, 0)
  } else {
    start.setDate(start.getDate() - sc.days)
  }
  const val = [fmtDate(start), fmtDate(end)]
  emit('update:modelValue', val)
  emit('change', val)
  popoverVisible.value = false
}

function onCalendarChange(val) {
  if (!val || val.length !== 2) return
  activeShortcut.value = ''
  const result = [fmtDate(val[0]), fmtDate(val[1])]
  emit('update:modelValue', result)
  emit('change', result)
  popoverVisible.value = false
}

function clearValue() {
  activeShortcut.value = ''
  emit('update:modelValue', null)
  emit('change', null)
}
</script>

<style scoped>
.drp-wrapper {
  display: inline-block;
}

.drp-trigger {
  border: 1px solid #dcdfe6;
  border-radius: 4px;
  padding: 0 12px;
  height: 32px;
  display: flex;
  align-items: center;
  cursor: pointer;
  background: #fff;
  min-width: 240px;
  gap: 8px;
  transition: border-color 0.2s;
}

.drp-trigger:hover {
  border-color: #c0c4cc;
}

.drp-trigger--small {
  height: 28px;
}

.drp-trigger--large {
  height: 40px;
}

.drp-icon-calendar {
  color: #909399;
  flex-shrink: 0;
}

.drp-display {
  flex: 1;
  color: #606266;
  font-size: 14px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.drp-placeholder {
  color: #c0c4cc;
}

.drp-icon-clear {
  color: #c0c4cc;
  cursor: pointer;
  flex-shrink: 0;
  transition: color 0.2s;
}

.drp-icon-clear:hover {
  color: #909399;
}

.drp-panel {
  display: flex;
  gap: 0;
}

.drp-shortcuts {
  width: 120px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  padding: 8px 0;
  border-right: 1px solid #ebeef5;
}

.drp-shortcut-btn {
  width: 100%;
  text-align: left;
  padding: 8px 12px;
  border: none;
  background: transparent;
  cursor: pointer;
  font-size: 13px;
  color: #606266;
  transition: background 0.15s, color 0.15s;
  border-radius: 0;
}

.drp-shortcut-btn:hover {
  background: #f5f7fa;
  color: #409eff;
}

.drp-shortcut-btn.is-active {
  background: #409eff;
  color: #fff;
}

.drp-calendar {
  flex: 1;
  padding: 8px;
}

.drp-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-top: 1px solid #ebeef5;
  padding: 8px 12px;
  margin-top: 4px;
}

.drp-footer-text {
  color: #606266;
  font-size: 13px;
}

.drp-footer-clear {
  color: #c0c4cc;
  cursor: pointer;
  transition: color 0.2s;
}

.drp-footer-clear:hover {
  color: #909399;
}
</style>
