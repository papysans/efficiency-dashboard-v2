<template>
  <el-card class="kb-filter-card" shadow="never">
    <div class="kb-filter-row">
      <!-- 四级组织级联（按需显示） -->
      <template v-if="showOrg">
        <el-select
          v-model="localOrg1" placeholder="一级组织" clearable
          style="width:140px" @change="onOrg1Change"
        >
          <el-option v-for="o in org1Options" :key="o" :label="o" :value="o" />
        </el-select>
        <el-select
          v-model="localOrg2" placeholder="二级组织" clearable
          style="width:140px" :disabled="!localOrg1" @change="onOrg2Change"
        >
          <el-option v-for="o in org2Options" :key="o" :label="o" :value="o" />
        </el-select>
        <el-select
          v-model="localOrg3" placeholder="三级组织" clearable
          style="width:140px" :disabled="!localOrg2" @change="onOrg3Change"
        >
          <el-option v-for="o in org3Options" :key="o" :label="o" :value="o" />
        </el-select>
        <el-select
          v-model="localOrg4" placeholder="四级组织" clearable
          style="width:140px" :disabled="!localOrg3" @change="onOrg4Change"
        >
          <el-option v-for="o in org4Options" :key="o" :label="o" :value="o" />
        </el-select>
      </template>

      <DateRangePicker v-model="localDateRange" :clearable="false" @change="onDateChange" />

      <!-- 额外自定义控件 -->
      <slot />
    </div>
  </el-card>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import DateRangePicker from './DateRangePicker.vue'
import { getOrgV2 } from '@/api/es'

const props = defineProps({
  /** 日期范围，格式 ['YYYY-MM-DD', 'YYYY-MM-DD'] */
  dateRange: { type: Array, required: true },
  /** 是否显示组织级联选择器 */
  showOrg: { type: Boolean, default: false },
  /** 初始组织值 { org1, org2, org3, org4 } */
  orgValue: { type: Object, default: () => ({}) },
})

const emit = defineEmits([
  'update:dateRange',
  /** 日期或组织任意条件变更时触发，payload: { dateRange, org1, org2, org3, org4 } */
  'change',
])

// ── 日期 ────────────────────────────────────────────────────
const localDateRange = computed({
  get: () => props.dateRange,
  set: (val) => emit('update:dateRange', val),
})

function onDateChange() {
  emitChange()
}

// ── 组织 ────────────────────────────────────────────────────
const localOrg1 = ref('')
const localOrg2 = ref('')
const localOrg3 = ref('')
const localOrg4 = ref('')
const org1Options = ref([])
const org2Options = ref([])
const org3Options = ref([])
const org4Options = ref([])

async function loadOrgOptions(level, parent) {
  try {
    const params = { level, parent: parent || '' }
    const dr = props.dateRange
    if (dr && dr.length === 2) {
      params.startDate = dr[0].replace(/-/g, '')
      params.endDate = dr[1].replace(/-/g, '')
    }
    const result = await getOrgV2(params)
    const data = result.data || result
    return (data.data || []).map(d => d.org_name)
  } catch {
    return []
  }
}

async function onOrg1Change(val) {
  localOrg2.value = ''
  localOrg3.value = ''
  localOrg4.value = ''
  org2Options.value = []
  org3Options.value = []
  org4Options.value = []
  if (val) org2Options.value = await loadOrgOptions('org2', val)
  emitChange()
}

async function onOrg2Change(val) {
  localOrg3.value = ''
  localOrg4.value = ''
  org3Options.value = []
  org4Options.value = []
  if (val) org3Options.value = await loadOrgOptions('org3', localOrg1.value + '/' + val)
  emitChange()
}

async function onOrg3Change(val) {
  localOrg4.value = ''
  org4Options.value = []
  if (val) org4Options.value = await loadOrgOptions('org4', localOrg1.value + '/' + localOrg2.value + '/' + val)
  emitChange()
}

function onOrg4Change() {
  emitChange()
}

// ── emit ────────────────────────────────────────────────────
function emitChange() {
  emit('change', {
    dateRange: localDateRange.value,
    org1: localOrg1.value,
    org2: localOrg2.value,
    org3: localOrg3.value,
    org4: localOrg4.value,
  })
}

// ── 从父组件传入的 orgValue 初始化（支持外部写入） ──────────
watch(() => props.orgValue, async (val) => {
  if (!val) return
  const { org1, org2, org3, org4 } = val
  localOrg1.value = org1 || ''
  localOrg2.value = org2 || ''
  localOrg3.value = org3 || ''
  localOrg4.value = org4 || ''
  // 恢复下级选项
  if (localOrg1.value) org2Options.value = await loadOrgOptions('org2', localOrg1.value)
  if (localOrg2.value) org3Options.value = await loadOrgOptions('org3', localOrg1.value + '/' + localOrg2.value)
  if (localOrg3.value) org4Options.value = await loadOrgOptions('org4', localOrg1.value + '/' + localOrg2.value + '/' + localOrg3.value)
}, { deep: true, immediate: false })

/** 暴露给父组件：重新加载 org1 选项（如日期变更后）*/
async function reloadOrg1() {
  org1Options.value = await loadOrgOptions('org1', '')
}

onMounted(async () => {
  if (!props.showOrg) return
  // 加载 org1
  org1Options.value = await loadOrgOptions('org1', '')
  // 恢复初始值（如果 orgValue 有值）
  const { org1, org2, org3, org4 } = props.orgValue || {}
  if (org1) {
    localOrg1.value = org1
    org2Options.value = await loadOrgOptions('org2', org1)
  }
  if (org2) {
    localOrg2.value = org2
    org3Options.value = await loadOrgOptions('org3', org1 + '/' + org2)
  }
  if (org3) {
    localOrg3.value = org3
    org4Options.value = await loadOrgOptions('org4', org1 + '/' + org2 + '/' + org3)
  }
  if (org4) localOrg4.value = org4
})

defineExpose({ reloadOrg1, localOrg1, localOrg2, localOrg3, localOrg4 })
</script>
