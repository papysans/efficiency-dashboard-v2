<template>
  <div class="kb-panel efficiency-panel">
    <!-- 筛选区 -->
    <el-card class="kb-filter-card" shadow="never">
      <div class="kb-filter-row">
        <el-radio-group v-model="dimension" style="margin-right: 16px">
          <el-radio-button value="work_dir">Work Dir</el-radio-button>
          <el-radio-button value="repo">Repo</el-radio-button>
        </el-radio-group>
        <DimensionSelect
          v-model="dimensionId"
          :dimension="dimension"
          :start-date="dateRange?.[0] || ''"
          :end-date="dateRange?.[1] || ''"
          :placeholder="dimension === 'work_dir' ? '搜索工作目录 ID...' : '搜索仓库 ID...'"
          style="width: 320px"
          @change="fetchData"
        />
        <DateRangePicker v-model="dateRange" :clearable="false" style="margin-left: 16px" />
        <el-button type="primary" :icon="Search" :loading="loading" @click="fetchData" style="margin-left: 16px">
          查询
        </el-button>
      </div>
    </el-card>

    <!-- 空状态 -->
    <el-empty v-if="!loading && !effData" description="请输入 ID 并选择日期范围后点击查询" />

    <template v-if="effData">
      <!-- 指标卡片区 -->
      <el-row :gutter="12">
        <el-col :span="6" style="margin-bottom: 12px">
          <el-card shadow="never" class="kb-metric-card">
            <div class="metric-header">
              <span class="kb-metric-label">AI 预估人天</span>
              <el-icon class="metric-edit" @click="openCorrectionDialog"><Edit /></el-icon>
            </div>
            <div class="kb-metric-value">
              {{ fmtDays(effData.ai_estimated?.corrected_days ?? effData.ai_estimated?.raw_days) }}
              <span v-if="fmtDays(effData.ai_estimated?.corrected_days ?? effData.ai_estimated?.raw_days) !== '-'"> 天</span>
            </div>
            <el-tag v-if="effData.ai_estimated?.is_corrected" size="small" type="success" style="margin-top: 4px">
              已人工校准
            </el-tag>
            <div v-if="effData.ai_estimated?.reasons?.length" class="reason-section">
              <el-collapse>
                <el-collapse-item title="AI 预估理由">
                  <ul class="reason-list">
                    <li v-for="(r, idx) in effData.ai_estimated.reasons" :key="idx">{{ r }}</li>
                  </ul>
                </el-collapse-item>
              </el-collapse>
            </div>
          </el-card>
        </el-col>
        <el-col :span="6" style="margin-bottom: 12px">
          <el-card shadow="never" class="kb-metric-card">
            <div class="kb-metric-label">实际 Lead Time</div>
            <div class="kb-metric-value">
              {{ msToDays(effData.actual_time?.total_lead_time_ms) }}
              <span v-if="!String(msToDays(effData.actual_time?.total_lead_time_ms)).includes('分钟') && msToDays(effData.actual_time?.total_lead_time_ms) !== '-'"> 天</span>
            </div>
          </el-card>
        </el-col>
        <el-col :span="6" style="margin-bottom: 12px">
          <el-card shadow="never" class="kb-metric-card">
            <div class="kb-metric-label">实际 Process Time</div>
            <div class="kb-metric-value">
              {{ msToDays(effData.actual_time?.total_process_time_ms) }}
              <span v-if="!String(msToDays(effData.actual_time?.total_process_time_ms)).includes('分钟') && msToDays(effData.actual_time?.total_process_time_ms) !== '-'"> 天</span>
            </div>
          </el-card>
        </el-col>
        <el-col :span="6" style="margin-bottom: 12px">
          <el-card shadow="never" class="kb-metric-card">
            <div class="kb-metric-label">提效比例(Lead)</div>
            <div class="kb-metric-value" :style="{ color: ratioColor(effData.efficiency?.ratio_lead) }">
              {{ fmtRatio(effData.efficiency?.ratio_lead) }}
              <span v-if="fmtRatio(effData.efficiency?.ratio_lead) !== '-'">%</span>
            </div>
          </el-card>
        </el-col>
      </el-row>
      <el-row :gutter="12">
        <el-col :span="6" style="margin-bottom: 12px">
          <el-card shadow="never" class="kb-metric-card">
            <div class="kb-metric-label">提效比例(Process)</div>
            <div class="kb-metric-value" :style="{ color: ratioColor(effData.efficiency?.ratio_process) }">
              {{ fmtRatio(effData.efficiency?.ratio_process) }}
              <span v-if="fmtRatio(effData.efficiency?.ratio_process) !== '-'">%</span>
            </div>
            <div v-if="effData.efficiency?.reason" class="reason-text">
              {{ effData.efficiency.reason }}
            </div>
          </el-card>
        </el-col>
        <el-col :span="6" style="margin-bottom: 12px">
          <el-card shadow="never" class="kb-metric-card">
            <div class="kb-metric-label">投入成本</div>
            <div class="kb-metric-value">
              {{ fmtCost(effData.cost?.api_cost) }}
              <span v-if="fmtCost(effData.cost?.api_cost) !== '-'"> 元</span>
            </div>
          </el-card>
        </el-col>
        <el-col :span="6" style="margin-bottom: 12px">
          <el-card shadow="never" class="kb-metric-card">
            <div class="kb-metric-label">节省成本</div>
            <div class="kb-metric-value">
              {{ fmtCost(effData.cost?.saving) }}
              <span v-if="fmtCost(effData.cost?.saving) !== '-'"> 元</span>
            </div>
          </el-card>
        </el-col>
        <el-col :span="6" style="margin-bottom: 12px">
          <el-card shadow="never" class="kb-metric-card">
            <div class="kb-metric-label">ROI</div>
            <div class="kb-metric-value">
              {{ fmtRatio(effData.cost?.roi) }}
              <span v-if="fmtRatio(effData.cost?.roi) !== '-'">%</span>
            </div>
          </el-card>
        </el-col>
      </el-row>
      <el-row :gutter="12">
        <el-col :span="6" style="margin-bottom: 12px">
          <el-card shadow="never" class="kb-metric-card">
            <div class="kb-metric-label">产出代码行数</div>
            <div class="kb-metric-value">{{ fmtCodeLines(effData.actual_time?.total_code_lines) }}</div>
          </el-card>
        </el-col>
      </el-row>

      <!-- 用户参与详情表 -->
      <el-card shadow="never" style="margin-bottom: 12px">
        <template #header><span>用户参与详情</span></template>
        <el-table :data="userTableData" stripe border style="width: 100%" show-summary :summary-method="getUserSummary">
          <el-table-column prop="user_name" label="用户名" width="150" />
          <el-table-column prop="start_time" label="开始时间" width="180" :formatter="(row, col, val) => formatLocalTime(val)" />
          <el-table-column prop="end_time" label="结束时间" width="180" :formatter="(row, col, val) => formatLocalTime(val)" />
          <el-table-column prop="lead_time_days" label="Lead Time(天)" width="140" align="right" sortable />
          <el-table-column prop="process_time_days" label="Process Time(天)" width="150" align="right" sortable />
          <el-table-column prop="percentage" label="占比(%)" width="100" align="right" sortable />
        </el-table>
      </el-card>

      <!-- 代码来源统计 -->
      <el-card shadow="never" style="margin-bottom: 12px">
        <template #header><span>代码来源统计</span></template>
        <el-row :gutter="12">
          <el-col :span="14">
            <el-table :data="codeSourceData" stripe border style="width: 100%">
              <el-table-column prop="source" label="来源" width="150" />
              <el-table-column prop="commit_count" label="Commit数" width="120" align="right" sortable />
              <el-table-column prop="code_lines" label="代码行数" width="120" align="right" sortable />
              <el-table-column prop="estimated_days" label="预估人天" width="120" align="right" sortable />
              <el-table-column prop="percentage" label="占比(%)" width="100" align="right" sortable />
            </el-table>
          </el-col>
          <el-col :span="10">
            <div ref="pieChartRef" class="kb-chart-container"></div>
          </el-col>
        </el-row>
      </el-card>
    </template>

    <!-- 纠错对话框 -->
    <CorrectionDialog
      v-model:visible="correctionVisible"
      :dimension="dimension"
      :dimension-id="dimensionId"
      :raw-days="effData?.ai_estimated?.raw_days"
      :corrected-days="effData?.ai_estimated?.corrected_days"
      :is-corrected="effData?.ai_estimated?.is_corrected"
      :start-date="dateRange?.[0]?.replace(/-/g, '') || ''"
      :end-date="dateRange?.[1]?.replace(/-/g, '') || ''"
      @corrected="fetchData"
    />
  </div>
</template>

<script setup>
import { ref, onMounted, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Search, Edit } from '@element-plus/icons-vue'
import { useChart } from '@/composables/useChart'
import { ElMessage } from 'element-plus'
import { getEfficiency, getCodeSourceStats } from '@/api/es'
import { getDefaultDateRange } from '@/utils/date'
import { formatLocalTime } from '@/utils/formatters'
import CorrectionDialog from '@/components/CorrectionDialog.vue'
import DimensionSelect from '@/components/DimensionSelect.vue'
import DateRangePicker from '@/components/DateRangePicker.vue'

const route = useRoute()
const router = useRouter()

// 筛选状态
const dimension = ref('work_dir')
const dimensionId = ref('')
const dateRange = ref([])
const loading = ref(false)

// 数据
const effData = ref(null)
const codeSourceData = ref([])

// 纠错对话框
const correctionVisible = ref(false)

// 图表
const pieChartRef = ref(null)
const { setOption: setPieOption } = useChart(pieChartRef)

// 格式化函数
function fmtDays(val) {
  if (val == null || val === 0) return '-'
  const v = Number(val)
  if (v < 0.1) return v.toFixed(2)
  return v.toFixed(1)
}

function msToDays(ms) {
  if (ms == null || ms === 0) return '-'
  const days = Number(ms) / 28800000
  if (days < 0.01) {
    // 小于 0.01 天（约 5 分钟）显示分钟
    const mins = Math.round(Number(ms) / 60000)
    return mins + ' 分钟'
  }
  if (days < 0.1) return days.toFixed(2)
  return days.toFixed(1)
}

function fmtRatio(val) {
  if (val == null || val === 0) return '-'
  const v = Number(val)
  if (v > 99999) return '>99999'
  return v.toFixed(1)
}

function fmtCost(val) {
  if (val == null || val === 0) return '-'
  return Number(val).toFixed(2)
}

function ratioColor(val) {
  if (val == null) return ''
  return val >= 100 ? '#67C23A' : '#F56C6C'
}

function fmtCodeLines(val) {
  if (val == null || val === 0) return '-'
  return val.toLocaleString()
}

// 用户表格数据（转换毫秒为天）
const userTableData = ref([])

function buildUserTable() {
  const users = effData.value?.actual_time?.users || []
  userTableData.value = users.map(u => ({
    user_name: u.user_name,
    start_time: u.start_time,
    end_time: u.end_time,
    lead_time_days: (Number(u.lead_time_ms || 0) / 28800000).toFixed(1),
    process_time_days: (Number(u.process_time_ms || 0) / 28800000).toFixed(1),
    percentage: u.percentage,
  }))
}

function getUserSummary({ columns, data }) {
  const sums = []
  columns.forEach((col, idx) => {
    if (idx === 0) {
      sums[idx] = '合计'
      return
    }
    if (col.property === 'lead_time_days' || col.property === 'process_time_days') {
      const total = data.reduce((sum, row) => sum + Number(row[col.property] || 0), 0)
      sums[idx] = total.toFixed(1)
    } else if (col.property === 'percentage') {
      const total = data.reduce((sum, row) => sum + Number(row[col.property] || 0), 0)
      sums[idx] = total
    } else {
      sums[idx] = ''
    }
  })
  return sums
}

// 纠错对话框
function openCorrectionDialog() {
  correctionVisible.value = true
}

// 饼图
function updatePieChart() {
  if (codeSourceData.value.length === 0) return
  const sourceLabels = {
    ai_current: 'AI(当前)',
    human: '人工',
    ai_other: 'AI(其他)',
    unknown: '未知',
  }
  setPieOption({
    title: { text: '代码来源占比', left: 'center', textStyle: { fontSize: 14, fontWeight: 'bold' } },
    tooltip: { trigger: 'item', formatter: '{b}: {c} 行 ({d}%)' },
    series: [{
      type: 'pie',
      radius: '60%',
      data: codeSourceData.value.map(d => ({
        name: sourceLabels[d.source] || d.source,
        value: d.code_lines || 0,
      })),
    }],
  })
}

// 请求数据
async function fetchData() {
  const id = dimensionId.value.trim()
  if (!id) {
    ElMessage.warning('请输入项目/仓库 ID')
    return
  }
  if (!dateRange.value || dateRange.value.length !== 2) return

  loading.value = true
  try {
    const params = {
      dimension: dimension.value,
      id,
      startDate: dateRange.value[0].replace(/-/g, ''),
      endDate: dateRange.value[1].replace(/-/g, ''),
    }

    // 提效分析数据（必须）
    const effResult = await getEfficiency(params)
    effData.value = effResult.data || effResult

    // 代码来源统计（仅 repo 维度才有，project 维度忽略失败）
    codeSourceData.value = []
    try {
      const codeResult = await getCodeSourceStats(params)
      const codeData = codeResult.data || codeResult
      codeSourceData.value = codeData?.items || (Array.isArray(codeData) ? codeData : [])
    } catch (e) {
      // 非 repo 维度可能无此数据，静默忽略
    }

    buildUserTable()

    await nextTick()
    updatePieChart()
  } catch (err) {
    console.error('查询数据失败:', err)
    effData.value = null
    codeSourceData.value = []
    userTableData.value = []
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  if (!dateRange.value || dateRange.value.length !== 2) {
    dateRange.value = getDefaultDateRange()
  }

  // 从 URL 恢复参数
  const query = route.query
  if (query.dimension) dimension.value = query.dimension
  if (query.id) dimensionId.value = query.id
  if (query.startDate && query.endDate) {
    dateRange.value = [query.startDate, query.endDate]
  }

  // 如果有完整的 dimension+id 参数，自动查询
  if (dimensionId.value) {
    fetchData()
  }
})
</script>

<style scoped>
.efficiency-panel {
  gap: 12px;
}

.metric-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.metric-edit {
  cursor: pointer;
  color: #409eff;
  font-size: 16px;
}

.metric-edit:hover {
  color: #337ecc;
}

.kb-chart-container {
  height: 300px;
}

.reason-section {
  margin-top: 8px;
}

.reason-section :deep(.el-collapse-item__header) {
  height: 28px;
  line-height: 28px;
  font-size: 12px;
  color: #909399;
}

.reason-section :deep(.el-collapse-item__content) {
  padding-bottom: 4px;
}

.reason-list {
  margin: 0;
  padding-left: 16px;
  font-size: 12px;
  color: #606266;
  line-height: 1.6;
}

.reason-text {
  margin-top: 8px;
  font-size: 12px;
  color: #606266;
  line-height: 1.5;
}
</style>
