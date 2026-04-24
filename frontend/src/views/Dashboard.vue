<template>
  <div class="kb-panel dashboard">
    <!-- 筛选区 -->
    <el-card class="kb-filter-card" shadow="never">
      <div class="kb-filter-row">
        <el-date-picker
          v-model="dateRange"
          type="daterange"
          range-separator="至"
          start-placeholder="开始日期"
          end-placeholder="结束日期"
          value-format="YYYY-MM-DD"
          style="width: 300px"
          :clearable="false"
        />
        <el-tabs v-model="activeTab" class="index-tabs" @tab-change="handleTabChange">
          <el-tab-pane label="Request 数据" name="request" />
          <el-tab-pane label="Task 聚合" name="aggregate" />
        </el-tabs>
        <el-radio-group
          v-if="activeTab === 'aggregate'"
          v-model="dimension"
          style="margin-left: 16px"
          @change="handleDimensionChange"
        >
          <el-radio-button value="project">按项目</el-radio-button>
          <el-radio-button value="user">按用户</el-radio-button>
          <el-radio-button value="org1">组织1</el-radio-button>
          <el-radio-button value="org2">组织2</el-radio-button>
          <el-radio-button value="org3">组织3</el-radio-button>
          <el-radio-button value="org4">组织4</el-radio-button>
        </el-radio-group>
        <el-button type="primary" :icon="Search" :loading="loading" @click="fetchData" style="margin-left: 16px">
          查询
        </el-button>
        <el-button
          v-if="activeTab === 'aggregate' && selectedRows.length > 0"
          type="warning"
          @click="showCreateGroupDialog"
          style="margin-left: 8px"
        >
          创建虚拟组（已选{{ selectedRows.length }}项）
        </el-button>
      </div>
    </el-card>

    <!-- 数据表格区 -->
    <el-card class="kb-table-card table-card" shadow="never">
      <el-table
        :data="tableData"
        stripe
        border
        v-loading="loading"
        style="width: 100%"
        :max-height="tableMaxHeight"
        @selection-change="handleSelectionChange"
      >
        <!-- Request 模式列 -->
        <template v-if="activeTab === 'request'">
          <el-table-column prop="@timestamp" label="时间" width="180" :formatter="fmtTimestamp" />
          <el-table-column prop="user_name" label="用户名" width="120" />
          <el-table-column prop="project_id" label="项目ID" width="200" show-overflow-tooltip />
          <el-table-column prop="model" label="模型" width="120" />
          <el-table-column prop="sender" label="发送者" width="80" />
          <el-table-column prop="user_in_chars" label="输入字符数" width="100" align="right" />
          <el-table-column prop="assistant_out_code_lines" label="代码行数" width="100" align="right" />
          <el-table-column prop="api_process_time" label="处理时长(ms)" width="120" align="right" />
          <el-table-column prop="api_ttft" label="首token时延(ms)" width="120" align="right" />
          <el-table-column prop="api_in_tokens" label="输入tokens" width="100" align="right" />
          <el-table-column prop="api_out_tokens" label="输出tokens" width="100" align="right" />
          <el-table-column prop="api_cost" label="费用(元)" width="100" align="right" :formatter="fmtCost" />
        </template>

        <!-- Task 聚合模式列 -->
        <template v-if="activeTab === 'aggregate'">
          <el-table-column type="selection" width="50" />
          <el-table-column prop="key" label="项目/用户/组织 ID" width="250" show-overflow-tooltip />
          <el-table-column prop="user_in_chars" label="输入字符数" width="120" align="right" sortable />
          <el-table-column prop="code_lines" label="代码行数" width="120" align="right" sortable />
          <el-table-column prop="api_count" label="API 次数" width="100" align="right" sortable />
          <el-table-column prop="api_cost" label="API 费用(元)" width="120" align="right" :formatter="fmtCost" sortable />
          <el-table-column prop="api_in_tokens" label="输入 tokens" width="120" align="right" sortable />
          <el-table-column prop="api_out_tokens" label="输出 tokens" width="120" align="right" sortable />
          <el-table-column prop="task_count" label="任务数量" width="100" align="right" sortable />
          <el-table-column prop="ai_estimated_days" label="AI预估人天" width="120" align="right" :formatter="fmtDays" sortable />
          <el-table-column prop="start_time" label="开始时间" width="180" :formatter="(row, col, val) => formatLocalTime(val)" />
          <el-table-column prop="end_time" label="结束时间" width="180" :formatter="(row, col, val) => formatLocalTime(val)" />
          <el-table-column prop="lead_time" label="前置时长" width="120" align="right" :formatter="fmtMsToMin" sortable />
          <el-table-column prop="process_time" label="处理时长" width="120" align="right" :formatter="fmtMsToMin" sortable />
        </template>
      </el-table>

      <!-- 分页 -->
      <div class="kb-pagination">
        <el-pagination
          v-model:current-page="page"
          v-model:page-size="pageSize"
          :page-sizes="[20, 50, 100, 200]"
          :total="total"
          layout="total, sizes, prev, pager, next, jumper"
          @size-change="handleSizeChange"
          @current-change="handlePageChange"
        />
      </div>
    </el-card>

    <!-- 创建虚拟组弹窗 -->
    <el-dialog v-model="groupDialogVisible" title="创建虚拟组" width="400px">
      <el-form>
        <el-form-item label="虚拟组名称">
          <el-input v-model="groupName" placeholder="请输入虚拟组名称" />
        </el-form-item>
        <el-form-item label="当前维度">
          <el-tag>{{ dimension }}</el-tag>
        </el-form-item>
        <el-form-item label="已选成员">
          <div>
            <el-tag v-for="row in selectedRows" :key="row.key" style="margin: 2px">{{ row.key }}</el-tag>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="groupDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="creatingGroup" @click="handleCreateGroup">确定创建</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Search } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { getRequests, getAggregate, createVirtualGroup } from '@/api/es'
import { fmtCost, fmtDays, fmtMsToMin, formatLocalTime } from '@/utils/formatters'
import { getDefaultDateRange } from '@/utils/date'

const route = useRoute()
const router = useRouter()

// 筛选状态
const dateRange = ref([])
const activeTab = ref('request')
const dimension = ref('project')

// 表格状态
const tableData = ref([])
const loading = ref(false)
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const tableMaxHeight = ref(600)

// 防抖定时器
let debounceTimer = null

// 多选 & 虚拟组
const selectedRows = ref([])
const groupDialogVisible = ref(false)
const groupName = ref('')
const creatingGroup = ref(false)

// 格式化时间戳
function fmtTimestamp(row, col, value) {
  if (!value) return ''
  const d = new Date(value)
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  const h = String(d.getHours()).padStart(2, '0')
  const min = String(d.getMinutes()).padStart(2, '0')
  const s = String(d.getSeconds()).padStart(2, '0')
  return `${y}-${m}-${day} ${h}:${min}:${s}`
}

// 同步 URL 参数
function syncUrl() {
  const query = { ...route.query }
  if (dateRange.value && dateRange.value.length === 2) {
    query.startDate = dateRange.value[0]
    query.endDate = dateRange.value[1]
  }
  query.tab = activeTab.value
  query.dimension = dimension.value
  router.replace({ query })
}

// 从 URL 恢复状态
function restoreFromUrl() {
  const q = route.query
  if (q.startDate && q.endDate) {
    dateRange.value = [q.startDate, q.endDate]
  }
  if (q.tab === 'request' || q.tab === 'aggregate') {
    activeTab.value = q.tab
  }
  if (['project', 'user', 'org1', 'org2', 'org3', 'org4'].includes(q.dimension)) {
    dimension.value = q.dimension
  }
}

// 请求数据
async function fetchData() {
  if (!dateRange.value || dateRange.value.length !== 2) return

  loading.value = true
  try {
    const params = {
      startDate: dateRange.value[0].replace(/-/g, ''),
      endDate: dateRange.value[1].replace(/-/g, ''),
      page: page.value,
      pageSize: pageSize.value,
    }

    let result
    if (activeTab.value === 'request') {
      result = await getRequests(params)
    } else {
      params.dimension = dimension.value
      result = await getAggregate(params)
    }

    const data = result.data || result
    tableData.value = data.hits || data.items || []
    total.value = data.total || 0
  } catch (err) {
    console.error('查询数据失败:', err)
    tableData.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

// 防抖查询
function debouncedFetch() {
  clearTimeout(debounceTimer)
  debounceTimer = setTimeout(() => {
    page.value = 1
    fetchData()
  }, 300)
}

// 事件处理
function handleSelectionChange(rows) {
  selectedRows.value = rows
}

function showCreateGroupDialog() {
  groupName.value = ''
  groupDialogVisible.value = true
}

async function handleCreateGroup() {
  const name = groupName.value.trim()
  if (!name) {
    ElMessage.warning('请输入虚拟组名称')
    return
  }
  creatingGroup.value = true
  try {
    await createVirtualGroup({
      name,
      dimension: dimension.value,
      member_keys: selectedRows.value.map(r => r.key),
    })
    ElMessage.success('虚拟组创建成功')
    groupDialogVisible.value = false
    selectedRows.value = []
  } catch (err) {
    console.error('创建虚拟组失败:', err)
    ElMessage.error('创建虚拟组失败')
  } finally {
    creatingGroup.value = false
  }
}

function handleTabChange() {
  syncUrl()
  debouncedFetch()
}

function handleDimensionChange() {
  syncUrl()
  debouncedFetch()
}

function handleSizeChange() {
  page.value = 1
  fetchData()
}

function handlePageChange() {
  fetchData()
}

// 计算表格最大高度
function calcTableHeight() {
  tableMaxHeight.value = window.innerHeight - 220
}

onMounted(() => {
  // 先从 URL 恢复状态
  restoreFromUrl()

  // 如果没有日期范围，设置默认值
  if (!dateRange.value || dateRange.value.length !== 2) {
    dateRange.value = getDefaultDateRange()
  }

  calcTableHeight()
  window.addEventListener('resize', calcTableHeight)

  // 首次加载数据
  fetchData()
})

onUnmounted(() => {
  clearTimeout(debounceTimer)
  window.removeEventListener('resize', calcTableHeight)
})
</script>

<style scoped>
.dashboard {
  height: 100%;
}

.kb-filter-card {
  flex-shrink: 0;
}

.index-tabs {
  margin-left: 16px;
}

.index-tabs :deep(.el-tabs__header) {
  margin-bottom: 0;
}

.index-tabs :deep(.el-tabs__nav-wrap::after) {
  display: none;
}

.table-card {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.kb-table-card :deep(.el-card__body) {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.kb-pagination {
  flex-shrink: 0;
}
</style>
