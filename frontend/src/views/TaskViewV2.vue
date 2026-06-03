<template>
  <div class="kb-panel">
    <!-- AI 预估警告条 -->
    <div v-if="missingEstimateCount > 0" style="display: flex; align-items: center; gap: 12px; margin-bottom: 8px;">
      <el-alert type="warning" :closable="false" show-icon style="flex: 1">
        <template #title>
          有 {{ missingEstimateCount }} 条任务缺少「传统开发时长预估」数据
        </template>
      </el-alert>
      <el-button type="primary" :loading="estimating" @click="runEstimate">
        {{ estimating ? '估算中...' : 'AI 生成预估数据' }}
      </el-button>
    </div>

    <KbFilterTable
      ref="filterTableRef"
      :columns="columns"
      :data="tableData"
      :loading="loading"
      :total="total"
      :order="order"
      :show-selection="true"
      v-model:page="page"
      v-model:pageSize="pageSize"
      @selection-change="handleSelectionChange"
      @size-change="handleSizeChange"
      @page-change="handlePageChange"
      @filter-change="handleFilterChange"
      @sort-change="onSortChange"
    >
      <template #actions>
        <span style="color: #909399; font-size: 13px">已选 {{ selectedTasks.length }} 个 Task</span>
        <el-button type="primary" size="small" :disabled="selectedTasks.length === 0" @click="showAddToProjectDialog = true">
          添加到 Project
        </el-button>
      </template>
      <template #cell-task_id="{ row }">
        <el-link type="primary" @click.stop="router.push('/task/' + row.task_id)">{{ (row.task_id || '').substring(0, 6) }}</el-link>
      </template>
      <template #cell-user_name="{ row }">
        <el-link type="primary" @click.stop="goToUser(row.user_id, row.user_name)">{{ row.user_name }}</el-link>
      </template>
      <template #cell-org_display="{ row }">
        <el-link v-if="row.org1" type="primary" @click.stop="goToOrg(row)">
          {{ [row.org1, row.org2, row.org3, row.org4].filter(Boolean).join('/') }}
        </el-link>
        <span v-else>-</span>
      </template>

      <template #cell-efficiency_ratio="{ row }">
        <el-tag
          v-if="row.efficiency_ratio != null"
          :type="row.efficiency_ratio >= 300 ? 'success' : row.efficiency_ratio >= 150 ? 'primary' : 'info'"
          size="small"
        >
          {{ row.efficiency_ratio.toFixed(1) }}%
        </el-tag>
        <el-tag v-else type="info" size="small">-</el-tag>
      </template>
    </KbFilterTable>

    <!-- 添加到 Project 对话框 -->
    <el-dialog v-model="showAddToProjectDialog" title="添加到 Project" width="500px">
      <el-form label-width="100px">
        <el-form-item label="目标项目">
          <el-select v-model="selectedProjectId" placeholder="选择项目" style="width: 100%">
            <el-option v-for="p in projectList" :key="p.project_id" :label="p.name" :value="p.project_id" />
            <el-option label="+ 创建新 Project" value="__new__" />
          </el-select>
        </el-form-item>
        <template v-if="selectedProjectId === '__new__'">
          <el-form-item label="项目名称">
            <el-input v-model="newProjectName" placeholder="输入项目名称" />
          </el-form-item>
          <el-form-item label="项目描述">
            <el-input v-model="newProjectDesc" type="textarea" :rows="2" placeholder="输入项目描述（可选）" />
          </el-form-item>
        </template>
        <el-form-item label="Silica 权重">
          <el-input-number v-model="silicaWeight" :min="0" :max="1" :step="0.1" :precision="1" />
        </el-form-item>
        <el-form-item>
          <span style="color: #909399; font-size: 13px">已选 {{ selectedTasks.length }} 个 Task</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showAddToProjectDialog = false">取消</el-button>
        <el-button type="primary" @click="handleAddToProject" :loading="addingToProject">确认</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
defineOptions({ name: 'TaskViewV2' })
import { ref, computed, onMounted, nextTick, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import KbFilterTable from '@/components/KbFilterTable.vue'
import { getTasksV2, estimateAncientMinutes, getProjects, createProject, addTasksToProject } from '@/api/es'
import { fmtCost, formatDuration, formatLocalTime } from '@/utils/formatters'
import { getDefaultDateRangeWide } from '@/utils/date'
import { parseOrder } from '@/utils/sort'
import { ElMessage } from 'element-plus'

const router = useRouter()
const route = useRoute()
const filterTableRef = ref(null)

function parseDateRange(startDate, endDate) {
  if (startDate && endDate) {
    const fmt = s => s.slice(0, 4) + '-' + s.slice(4, 6) + '-' + s.slice(6, 8)
    return [fmt(startDate), fmt(endDate)]
  }
  return getDefaultDateRangeWide()
}

function syncUrlToControls() {
  const { startDate, endDate, userName, org1, org2, org3, org4, order: orderQuery } = route.query
  serverDateRange = parseDateRange(startDate, endDate)
  order.value = typeof orderQuery === 'string' ? orderQuery : undefined
  filterTableRef.value?.setFilter('start_time', serverDateRange)
  filterTableRef.value?.setFilter('user_name', userName ? String(userName).trim() : '')
  const orgVal = { org1: org1 || '', org2: org2 || '', org3: org3 || '', org4: org4 || '' }
  if (orgVal.org1 || orgVal.org2 || orgVal.org3 || orgVal.org4) {
    filterTableRef.value?.setFilter('org_display', orgVal)
  } else {
    filterTableRef.value?.setFilter('org_display', null)
  }
}

function updateUrl() {
  const [start, end] = serverDateRange
  const currentUserName = filterTableRef.value?.getFilter('user_name') || ''
  const orgFilter = filterTableRef.value?.getFilter('org_display') || {}
  const query = {
    startDate: start.replace(/-/g, ''),
    endDate: end.replace(/-/g, ''),
  }
  if (currentUserName) query.userName = currentUserName
  if (orgFilter.org1) query.org1 = orgFilter.org1
  if (orgFilter.org2) query.org2 = orgFilter.org2
  if (orgFilter.org3) query.org3 = orgFilter.org3
  if (orgFilter.org4) query.org4 = orgFilter.org4
  if (order.value) query.order = order.value
  _ignoreRouteWatch = true
  router.replace({ query }).finally(() => { _ignoreRouteWatch = false })
}

/** 取 manual 优先值 */
function fmtRealMinutes(row, col, value) {
  return formatDuration(row.task_real_minutes_manual ?? row.task_real_minutes)
}
function fmtAncientMinutes(row, col, value) {
  return formatDuration(row.task_ancient_minutes_manual ?? row.task_ancient_minutes)
}
/** 取 manual 优先的数值，用于排序和筛选 */
function getEffectiveReal(row) {
  return row.task_real_minutes_manual ?? row.task_real_minutes ?? null
}
function getEffectiveAncient(row) {
  return row.task_ancient_minutes_manual ?? row.task_ancient_minutes ?? null
}

// 列定义
const columns = [
  {
    prop: 'task_id',
    label: 'Task ID',
    width: 90,
    slotName: 'task_id',
  },
  {
    prop: 'start_time',
    label: '时间',
    width: 175,
    sortField: 'startTime',
    formatter: (row, col, val) => formatLocalTime(val),
    filter: { type: 'date', serverSide: true },
  },
  {
    prop: 'org_display',
    label: '组织',
    minWidth: 200,
    showOverflowTooltip: true,
    slotName: 'org_display',
    filter: { type: 'cascade-org' },
  },
  {
    prop: 'user_name',
    label: '用户',
    width: 90,
    slotName: 'user_name',
    filter: { type: 'multi-select' },
  },
  {
    prop: 'title',
    label: '说明',
    minWidth: 120,
    showOverflowTooltip: true,
    filter: { type: 'text' },
  },
  {
    prop: 'diff_lines',
    label: '代码量',
    width: 85,
    align: 'right',
    sortField: 'diffLines',
    filter: { type: 'number', shortcuts: [
      { label: '> 0', value: { min: 1 } },
      { label: '> 50', value: { min: 50 } },
      { label: '> 200', value: { min: 200 } },
    ]},
  },
  {
    prop: 'task_real_minutes',
    label: '实际耗时',
    width: 110,
    align: 'right',
    // 显示走 getEffectiveReal（manual 覆盖）→ 客户端按显示值排
    clientSort: true,
    sortValue: getEffectiveReal,
    formatter: fmtRealMinutes,
    filter: { type: 'number', valueGetter: getEffectiveReal, shortcuts: [
      { label: '> 0', value: { min: 0.1 } },
      { label: '> 30min', value: { min: 30 } },
      { label: '> 1h', value: { min: 60 } },
    ]},
  },
  {
    prop: 'task_ancient_minutes',
    label: '传统耗时预估',
    width: 115,
    align: 'right',
    // 显示走 getEffectiveAncient（manual 覆盖）→ 客户端按显示值排
    clientSort: true,
    sortValue: getEffectiveAncient,
    formatter: fmtAncientMinutes,
    filter: { type: 'number', valueGetter: getEffectiveAncient, shortcuts: [
      { label: '> 0', value: { min: 0.1 } },
      { label: '> 30min', value: { min: 30 } },
      { label: '> 1h', value: { min: 60 } },
    ]},
  },
  {
    prop: 'efficiency_ratio',
    label: '提效比',
    width: 85,
    align: 'center',
    // 显示走 CalcEfficiencyRatioManual（封顶/覆盖）→ 客户端按显示值排
    clientSort: true,
    sortValue: (row) => row.efficiency_ratio,
    slotName: 'efficiency_ratio',
    filter: { type: 'number', shortcuts: [
      { label: '> 100%', value: { min: 100 } },
      { label: '> 200%', value: { min: 200 } },
      { label: '> 300%', value: { min: 300 } },
    ]},
  },
  {
    prop: '_tokens',
    label: 'Tokens消耗',
    width: 100,
    align: 'right',
    formatter: (row) => {
      const total = (row.upstream_tokens || 0) + (row.downstream_tokens || 0)
      return total > 0 ? total.toLocaleString() : '-'
    },
    filter: { type: 'number', valueGetter: (row) => (row.upstream_tokens || 0) + (row.downstream_tokens || 0), shortcuts: [
      { label: '> 0', value: { min: 1 } },
      { label: '> 10k', value: { min: 10000 } },
      { label: '> 100k', value: { min: 100000 } },
    ]},
  },
  {
    prop: 'cost',
    label: '费用',
    width: 75,
    align: 'right',
    sortField: 'cost',
    formatter: fmtCost,
    filter: { type: 'number', shortcuts: [
      { label: '> 0', value: { min: 0.001 } },
      { label: '> 0.01', value: { min: 0.01 } },
      { label: '> 0.1', value: { min: 0.1 } },
    ]},
  },
]

const loading = ref(false)
const tableData = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(250)
const order = ref(typeof route.query.order === 'string' ? route.query.order : undefined)

let serverDateRange = getDefaultDateRangeWide()

let _ignoreRouteWatch = false

// 仅当 order 命中服务端列（声明了 sortField）时才把 order 下发给后端。
function serverOrderParam() {
  const f = parseOrder(order.value)
  if (!f) return null
  const serverFields = columns.filter(c => c.sortField).map(c => c.sortField)
  return serverFields.includes(f.field) ? order.value : null
}

async function fetchData() {
  if (!serverDateRange || serverDateRange.length !== 2) return
  loading.value = true
  try {
    const serverOrder = serverOrderParam()
    const params = {
      startDate: serverDateRange[0].replace(/-/g, ''),
      endDate: serverDateRange[1].replace(/-/g, ''),
      page: page.value,
      pageSize: pageSize.value,
      ...(serverOrder ? { order: serverOrder } : {}),
    }
    const result = await getTasksV2(params)
    const data = result.data || result
    tableData.value = data.data || []
    total.value = data.total || 0
  } catch {
    tableData.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

function goToOrg(row) {
  if (!row.org1) return
  const [start, end] = serverDateRange
  const orgPath = [row.org1, row.org2, row.org3, row.org4].filter(Boolean).join('/')
  const query = {
    startDate: start.replace(/-/g, ''),
    endDate: end.replace(/-/g, ''),
  }
  if (row.org1) query.org1 = row.org1
  if (row.org2) query.org2 = row.org2
  if (row.org3) query.org3 = row.org3
  if (row.org4) query.org4 = row.org4
  router.push({ path: '/org/' + encodeURIComponent(orgPath), query })
}

function goToUser(userId, userName) {
  if (!userId && !userName) return
  const [start, end] = serverDateRange
  router.push({
    path: '/user/' + (userId || userName),
    query: {
      startDate: start.replace(/-/g, ''),
      endDate: end.replace(/-/g, ''),
    },
  })
}


// 缺少传统开发时长预估的任务数
const missingEstimateCount = computed(() => {
  return tableData.value.filter(t =>
    t.task_ancient_minutes == null && t.task_ancient_minutes_manual == null
  ).length
})

const estimating = ref(false)

async function runEstimate() {
  estimating.value = true
  try {
    const result = await estimateAncientMinutes()
    const data = result.data || result
    ElMessage.success(`估算完成：成功 ${data.success || 0}/${data.total || 0} 条`)
    await fetchData()
  } catch (e) {
    ElMessage.error('估算失败: ' + (e.message || e))
  } finally {
    estimating.value = false
  }
}

// === 多选 & 添加到 Project ===
const selectedTasks = ref([])

function handleSelectionChange(val) {
  selectedTasks.value = val
}

const showAddToProjectDialog = ref(false)
const projectList = ref([])
const selectedProjectId = ref('')
const newProjectName = ref('')
const newProjectDesc = ref('')
const silicaWeight = ref(1.0)
const addingToProject = ref(false)

async function loadProjects() {
  try {
    const result = await getProjects()
    const data = result.data || result
    projectList.value = data.data || data || []
  } catch {
    projectList.value = []
  }
}

// Watch dialog open to load projects
watch(showAddToProjectDialog, (val) => {
  if (val) {
    loadProjects()
    selectedProjectId.value = ''
    newProjectName.value = ''
    newProjectDesc.value = ''
    silicaWeight.value = 1.0
  }
})

async function handleAddToProject() {
  let projectId = selectedProjectId.value
  if (!projectId) {
    ElMessage.warning('请选择目标项目')
    return
  }
  addingToProject.value = true
  try {
    if (projectId === '__new__') {
      if (!newProjectName.value.trim()) {
        ElMessage.warning('请输入项目名称')
        addingToProject.value = false
        return
      }
      const res = await createProject({ name: newProjectName.value.trim(), description: newProjectDesc.value.trim() })
      const data = res.data || res
      projectId = data.project_id
    }
    await addTasksToProject(projectId, {
      task_ids: selectedTasks.value.map(t => t.task_id),
      task_ids_silica: selectedTasks.value.map(() => silicaWeight.value),
    })
    ElMessage.success('添加成功')
    showAddToProjectDialog.value = false
    selectedTasks.value = []
  } catch (e) {
    ElMessage.error('添加失败: ' + (e.message || e))
  } finally {
    addingToProject.value = false
  }
}

function handleFilterChange(allFilters) {
  const dateVal = allFilters.start_time
  if (dateVal && dateVal.length === 2) {
    serverDateRange = [dateVal[0], dateVal[1]]
  } else if (!dateVal) {
    serverDateRange = getDefaultDateRangeWide()
  }
  page.value = 1
  updateUrl()
  fetchData()
}

function onSortChange(payload) {
  order.value = payload.order
  updateUrl()
  // 仅服务端列需要重新取数；客户端列本地排序，无需请求。
  if (payload.server) {
    page.value = 1
    fetchData()
  }
}

function handleSizeChange() {
  page.value = 1
  fetchData()
}

function handlePageChange() {
  fetchData()
}

watch(() => route.query, async (query) => {
  if (_ignoreRouteWatch) return
  await nextTick()
  syncUrlToControls()
  await fetchData()
}, { deep: true })

onMounted(async () => {
  await nextTick()
  syncUrlToControls()
  await fetchData()
})
</script>
