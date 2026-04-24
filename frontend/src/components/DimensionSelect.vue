<template>
  <el-select
    v-model="selected"
    :placeholder="placeholder"
    filterable
    remote
    :remote-method="remoteSearch"
    :loading="loading"
    clearable
    :allow-create="allowCreate"
    default-first-option
    style="width: 100%"
    @change="handleChange"
    @clear="handleClear"
  >
    <el-option
      v-for="item in options"
      :key="item"
      :label="item"
      :value="item"
    />
  </el-select>
</template>

<script setup>
import { ref, watch, onMounted } from 'vue'
import { getAggregateKeys } from '@/api/es'

const props = defineProps({
  modelValue: { type: String, default: '' },
  dimension: { type: String, required: true },
  startDate: { type: String, default: '' },
  endDate: { type: String, default: '' },
  placeholder: { type: String, default: '输入关键词搜索...' },
  allowCreate: { type: Boolean, default: true },
})

const emit = defineEmits(['update:modelValue', 'change'])

const selected = ref(props.modelValue)
const options = ref([])
const loading = ref(false)
let allKeys = []

// 同步外部 v-model
watch(() => props.modelValue, (val) => { selected.value = val })

// 日期或维度变化时重新加载全部 keys
watch(() => [props.dimension, props.startDate, props.endDate], () => {
  loadAllKeys()
}, { immediate: false })

onMounted(() => {
  loadAllKeys()
})

async function loadAllKeys() {
  if (!props.startDate || !props.endDate || !props.dimension) return
  loading.value = true
  try {
    const res = await getAggregateKeys({
      dimension: props.dimension,
      startDate: props.startDate.replace(/-/g, ''),
      endDate: props.endDate.replace(/-/g, ''),
    })
    const data = res.data || res
    allKeys = data.keys || []
    options.value = allKeys.slice(0, 50) // 默认显示前50条
  } catch (e) {
    console.error('加载维度列表失败:', e)
    allKeys = []
    options.value = []
  } finally {
    loading.value = false
  }
}

function remoteSearch(query) {
  if (query) {
    const q = query.toLowerCase()
    options.value = allKeys.filter(k => k.toLowerCase().includes(q)).slice(0, 50)
  } else {
    options.value = allKeys.slice(0, 50)
  }
}

function handleChange(val) {
  emit('update:modelValue', val)
  emit('change', val)
}

function handleClear() {
  emit('update:modelValue', '')
  emit('change', '')
}
</script>
