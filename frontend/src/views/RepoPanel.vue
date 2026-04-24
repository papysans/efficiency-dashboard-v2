<template>
  <div class="kb-panel">
    <!-- 筛选区 -->
    <el-card class="kb-filter-card" shadow="never">
      <div class="kb-filter-row">
        <DimensionSelect
          v-model="filterRepoId"
          dimension="repo"
          :start-date="dateRange?.[0] || ''"
          :end-date="dateRange?.[1] || ''"
          placeholder="搜索仓库 ID..."
          style="width: 320px; margin-right: 16px"
          @change="handleQuery"
        />
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
        <el-button type="primary" :icon="Search" :loading="loading" @click="handleQuery" style="margin-left: 16px">
          查询
        </el-button>
        <el-switch
          v-model="showFavoritesOnly"
          active-text="仅显示收藏"
          style="margin-left: 16px"
          @change="handleFavoritesToggle"
        />
      </div>
    </el-card>

    <!-- 空状态 -->
    <el-empty v-if="!loading && tableData.length === 0" description="请选择日期范围后点击查询" />

    <!-- 仓库列表表格 -->
    <el-card v-if="tableData.length > 0" shadow="never" class="kb-table-card">
      <el-table
        :data="tableData"
        stripe
        border
        v-loading="loading"
        style="width: 100%"
        :row-class-name="getRowClassName"
        highlight-current-row
        @row-click="handleRowClick"
      >
        <el-table-column label="" width="50" align="center">
          <template #default="{ row }">
            <el-icon
              :size="18"
              :color="isFavorited(row) ? '#E6A23C' : '#C0C4CC'"
              style="cursor: pointer"
              @click.stop="toggleFavorite(row)"
            >
              <StarFilled v-if="isFavorited(row)" />
              <Star v-else />
            </el-icon>
          </template>
        </el-table-column>
        <el-table-column prop="key" label="仓库ID" width="250" show-overflow-tooltip>
          <template #default="{ row }">
            <el-tag v-if="row._isVirtualGroup" type="warning" size="small" style="margin-right: 6px">虚拟组</el-tag>
            <el-link type="primary" :underline="false" @click.stop="goToEfficiency(row)">{{ row.key }}</el-link>
          </template>
        </el-table-column>
        <el-table-column prop="task_count" label="任务数" width="100" align="right" sortable />
        <el-table-column prop="api_count" label="API次数" width="100" align="right" sortable />
        <el-table-column prop="code_lines" label="代码行数" width="120" align="right" sortable />
        <el-table-column prop="api_cost" label="API费用" width="120" align="right" :formatter="fmtCost" sortable />
        <el-table-column prop="ai_estimated_days" label="AI预估人天" width="120" align="right" :formatter="fmtDays" sortable />
        <el-table-column prop="process_time" label="处理时长" width="120" align="right" :formatter="fmtMsToMin" sortable />
      </el-table>
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

    <!-- Git 分析区（选中仓库后展示） -->
    <template v-if="selectedRepoId">
      <!-- Commit 统计卡片 -->
      <el-row :gutter="12">
        <el-col :span="6" style="margin-bottom: 12px">
          <el-card shadow="never" class="kb-metric-card">
            <div class="kb-metric-label">总 Commit 数</div>
            <div class="kb-metric-value">{{ gitData?.total_commits ?? '-' }}</div>
          </el-card>
        </el-col>
        <el-col :span="6" style="margin-bottom: 12px">
          <el-card shadow="never" class="kb-metric-card">
            <div class="kb-metric-label">贡献者数</div>
            <div class="kb-metric-value">{{ gitData?.contributor_count ?? '-' }}</div>
          </el-card>
        </el-col>
        <el-col :span="6" style="margin-bottom: 12px">
          <el-card shadow="never" class="kb-metric-card">
            <div class="kb-metric-label">代码新增行数</div>
            <div class="kb-metric-value">{{ gitData?.lines_added ?? '-' }}</div>
          </el-card>
        </el-col>
        <el-col :span="6" style="margin-bottom: 12px">
          <el-card shadow="never" class="kb-metric-card">
            <div class="kb-metric-label">代码删除行数</div>
            <div class="kb-metric-value">{{ gitData?.lines_deleted ?? '-' }}</div>
          </el-card>
        </el-col>
      </el-row>

      <!-- 贡献者列表 -->
      <el-card v-if="gitData?.contributors && gitData.contributors.length > 0" shadow="never" style="margin-bottom: 12px">
        <template #header><span>贡献者列表</span></template>
        <el-table :data="gitData.contributors" stripe border style="width: 100%">
          <el-table-column prop="name" label="贡献者" width="200" />
          <el-table-column prop="commits" label="Commit 数" width="120" align="right" sortable />
          <el-table-column prop="lines_added" label="新增行数" width="120" align="right" sortable />
          <el-table-column prop="lines_deleted" label="删除行数" width="120" align="right" sortable />
        </el-table>
      </el-card>

      <!-- Task-Commit 关联表格 -->
      <el-card shadow="never" style="margin-bottom: 12px">
        <template #header><span>Task-Commit 关联</span></template>
        <el-empty v-if="taskCommitData.length === 0" description="暂无 Task-Commit 关联数据" />
        <el-table v-else :data="taskCommitData" stripe border style="width: 100%">
          <el-table-column prop="commit_hash" label="Commit Hash" width="180" show-overflow-tooltip />
          <el-table-column prop="author" label="作者" width="150" />
          <el-table-column prop="time" label="时间" width="180" :formatter="(row, col, val) => formatLocalTime(val)" />
          <el-table-column prop="task_id" label="Task ID" width="200" show-overflow-tooltip />
          <el-table-column prop="relation_type" label="关联类型" width="120" />
          <el-table-column prop="code_source" label="代码来源" width="120" />
        </el-table>
      </el-card>

      <!-- 代码来源分析 -->
      <el-card shadow="never" style="margin-bottom: 12px">
        <template #header><span>代码来源分析</span></template>
        <el-row :gutter="12">
          <el-col :span="14">
            <el-table :data="codeAttrData" stripe border style="width: 100%">
              <el-table-column prop="source" label="来源" width="150" />
              <el-table-column prop="lines" label="代码行数" width="120" align="right" />
              <el-table-column prop="percentage" label="占比(%)" width="120" align="right" />
            </el-table>
          </el-col>
          <el-col :span="10">
            <div ref="pieChartRef" class="kb-chart-container"></div>
          </el-col>
        </el-row>
      </el-card>
    </template>
  </div>
</template>

<script setup>
import { ref, onMounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { Search } from '@element-plus/icons-vue'
import { Star, StarFilled } from '@element-plus/icons-vue'
import { useChart } from '@/composables/useChart'
import { getAggregate, getGitAnalysis, getTaskCommitMappings, getCodeAttribution } from '@/api/es'
import { fmtCost, fmtDays, fmtMsToMin, formatLocalTime } from '@/utils/formatters'
import { getDefaultDateRange } from '@/utils/date'
import { useFavorites } from '@/composables/useFavorites'
import { useUrlSync } from '@/composables/useUrlSync'
import DimensionSelect from '@/components/DimensionSelect.vue'

const router = useRouter()

// 筛选状态
const dateRange = ref([])
const loading = ref(false)

// 仓库列表
const tableData = ref([])
const filterRepoId = ref('')
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)

// 收藏
const { favorites, showFavoritesOnly, isFavorited, toggleFavorite: _toggleFavorite, loadFavorites, applyFavoritesFilter } = useFavorites('repo')

async function toggleFavorite(row) {
  const needRefresh = await _toggleFavorite(row)
  if (needRefresh) {
    await handleFavoritesToggle(true)
  }
}

// URL 同步
const { syncToUrl, restoreFromUrl } = useUrlSync([
  { key: 'dateRange', ref: dateRange, type: 'dateRange' },
])

// 选中仓库
const selectedRepoId = ref('')

// Git 分析数据
const gitData = ref(null)
const taskCommitData = ref([])
const codeAttrData = ref([])

// 饼图
const pieChartRef = ref(null)
const { setOption: setPieOption } = useChart(pieChartRef)

// 请求仓库列表
async function fetchData() {
  if (!dateRange.value || dateRange.value.length !== 2) return

  loading.value = true
  selectedRepoId.value = ''
  gitData.value = null
  taskCommitData.value = []
  codeAttrData.value = []

  try {
    const params = {
      dimension: 'repo',
      startDate: dateRange.value[0].replace(/-/g, ''),
      endDate: dateRange.value[1].replace(/-/g, ''),
      page: page.value,
      pageSize: pageSize.value,
    }
    const result = await getAggregate(params)
    const data = result.data || result
    tableData.value = data.items || data.hits || []
    total.value = data.total || 0

    if (favorites.value.length === 0) {
      loadFavorites()
    }
  } catch (err) {
    console.error('查询数据失败:', err)
    tableData.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

// 点击仓库行
async function handleRowClick(row) {
  selectedRepoId.value = row.key

  const dateParams = {
    startDate: dateRange.value[0].replace(/-/g, ''),
    endDate: dateRange.value[1].replace(/-/g, ''),
  }

  try {
    const [gitResult, tcResult, caResult] = await Promise.all([
      getGitAnalysis({ repo_id: row.key, ...dateParams }),
      getTaskCommitMappings({ repo_id: row.key, ...dateParams }),
      getCodeAttribution({ repo_id: row.key, ...dateParams }),
    ])

    gitData.value = gitResult.data || gitResult
    const tcData = tcResult.data || tcResult
    taskCommitData.value = Array.isArray(tcData) ? tcData : (tcData?.items || [])
    const caData = caResult.data || caResult
    codeAttrData.value = Array.isArray(caData) ? caData : (caData?.items || [])

    await nextTick()
    updatePieChart()
  } catch (err) {
    console.error('获取仓库详情失败:', err)
    gitData.value = null
    taskCommitData.value = []
    codeAttrData.value = []
  }
}

// 下钻到提效详情
function goToEfficiency(row) {
  router.push({
    path: '/efficiency',
    query: {
      dimension: 'repo',
      id: row.key,
      startDate: dateRange.value[0],
      endDate: dateRange.value[1],
    },
  })
}

// 行样式
function getRowClassName({ row }) {
  const cls = []
  if (row._isVirtualGroup) cls.push('kb-vgroup-row')
  cls.push('kb-clickable-row')
  return cls.join(' ')
}

// 收藏开关变化
async function handleFavoritesToggle(val) {
  if (val) {
    await loadFavorites()
    loading.value = true
    try {
      const dateParams = {
        startDate: dateRange.value[0].replace(/-/g, ''),
        endDate: dateRange.value[1].replace(/-/g, ''),
      }
      const result = await applyFavoritesFilter(dateParams)
      tableData.value = result.items
      total.value = result.total
    } catch (err) {
      // 拦截器已处理错误提示
    } finally {
      loading.value = false
    }
  } else {
    fetchData()
  }
}

// 饼图
function updatePieChart() {
  if (codeAttrData.value.length === 0) return
  setPieOption({
    title: { text: '代码来源占比', left: 'center', textStyle: { fontSize: 14, fontWeight: 'bold' } },
    tooltip: { trigger: 'item', formatter: '{b}: {c} 行 ({d}%)' },
    series: [{
      type: 'pie',
      radius: '60%',
      data: codeAttrData.value.map(d => ({
        name: d.source || '未知',
        value: d.lines || 0,
      })),
    }],
  })
}

// 查询按钮
function handleQuery() {
  page.value = 1
  syncToUrl()
  fetchData()
}

// 分页
function handleSizeChange() {
  page.value = 1
  fetchData()
}

function handlePageChange() {
  fetchData()
}

onMounted(() => {
  restoreFromUrl()
  if (!dateRange.value || dateRange.value.length !== 2) {
    dateRange.value = getDefaultDateRange()
  }
  fetchData()
  loadFavorites()
})
</script>

<style scoped>
.kb-chart-container {
  height: 300px;
}
</style>
