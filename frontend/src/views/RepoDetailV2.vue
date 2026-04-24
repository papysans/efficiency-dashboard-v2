<template>
  <div class="kb-panel">
    <!-- 标题栏 -->
    <el-card shadow="never">
      <div style="display: flex; align-items: center; justify-content: space-between">
        <div style="display: flex; align-items: center; gap: 12px">
          <el-button :icon="ArrowLeft" @click="router.back()">返回</el-button>
          <span style="font-size: 18px; font-weight: bold">仓库详情</span>
        </div>
        <div style="display: flex; align-items: center; gap: 10px">
          <el-select v-model="currentBranch" placeholder="选择分支" style="width: 180px" @change="handleBranchChange">
            <el-option v-for="b in branches" :key="b" :label="b" :value="b" />
          </el-select>
          <span style="width: 1px; height: 20px; background: #dcdfe6; flex-shrink: 0"></span>
          <DateRangePicker v-model="dateRange" @change="onDateChange" size="small" />
          <el-button type="primary" size="small" @click="openAddToProject">添加到 Project</el-button>
        </div>
      </div>
    </el-card>

    <!-- 基础信息 -->
    <el-card shadow="never" header="基础信息" v-loading="loading">
      <el-descriptions :column="3" border>
        <el-descriptions-item label="仓库地址">{{ repoAddr || '-' }}</el-descriptions-item>
        <el-descriptions-item label="分支">{{ currentBranch || '-' }}</el-descriptions-item>
        <el-descriptions-item label="活跃时间">{{ activityRange }}</el-descriptions-item>
        <el-descriptions-item label="提交数">{{ commits.length }}</el-descriptions-item>
        <el-descriptions-item label="任务数">{{ tasks.length }}</el-descriptions-item>
        <el-descriptions-item label="总 Tokens">{{ totalTokens.toLocaleString() }}</el-descriptions-item>
      </el-descriptions>
    </el-card>

    <!-- 度量信息 -->
    <el-card shadow="never" header="度量信息（基于 Commits 汇总）">
      <el-descriptions :column="3" border>
        <el-descriptions-item label="传统开发时长预估">
          <span style="font-size: 18px; font-weight: bold; color: #E6A23C">
            {{ efficiency.repo_ancient_minutes != null ? formatDuration(efficiency.repo_ancient_minutes) : '-' }}
          </span>
          <el-tooltip v-if="efficiency.repo_ancient_minutes_reason" :content="efficiency.repo_ancient_minutes_reason" placement="top" :show-after="200" popper-class="reason-tooltip">
            <el-icon class="metric-reason"><QuestionFilled /></el-icon>
          </el-tooltip>
        </el-descriptions-item>
        <el-descriptions-item label="实际耗时">
          <span style="font-size: 18px; font-weight: bold; color: #409EFF">
            {{ efficiency.repo_real_minutes != null ? formatDuration(efficiency.repo_real_minutes) : '-' }}
          </span>
          <el-tooltip v-if="efficiency.repo_real_minutes_reason" :content="efficiency.repo_real_minutes_reason" placement="top" :show-after="200" popper-class="reason-tooltip">
            <el-icon class="metric-reason"><QuestionFilled /></el-icon>
          </el-tooltip>
        </el-descriptions-item>
        <el-descriptions-item label="提效比">
          <span :style="{ fontSize: '20px', fontWeight: 'bold', color: efficiencyColor }">
            {{ efficiency.efficiency_ratio != null ? Math.round(efficiency.efficiency_ratio) + '%' : '-' }}
          </span>
        </el-descriptions-item>
        <el-descriptions-item label="代码行数">{{ totalDiffLines.toLocaleString() }} 行</el-descriptions-item>
        <el-descriptions-item label="总费用（Tasks）">{{ totalCost > 0 ? totalCost.toFixed(2) + ' 元' : '-' }}</el-descriptions-item>
        <el-descriptions-item label="贡献者">{{ contributorCount }} 人</el-descriptions-item>
      </el-descriptions>
    </el-card>

    <!-- Commits 表格 -->
    <el-card shadow="never" v-loading="loading">
      <template #header><span>Commits ({{ commits.length }})</span></template>
      <el-table :data="commits" style="width: 100%" row-class-name="kb-clickable-row" @row-click="handleCommitClick" empty-text="暂无数据">
        <el-table-column label="Commit ID" min-width="100" show-overflow-tooltip sortable prop="commit_id">
          <template #default="{ row }">
            <el-link type="primary" @click.stop="router.push('/commit/' + row.commit_id)">{{ (row.commit_id || '').substring(0, 8) }}</el-link>
          </template>
        </el-table-column>
        <el-table-column label="时间" min-width="150" sortable prop="commit_time" :formatter="(row, col, val) => formatLocalTime(row.commit_time)" />
        <el-table-column prop="git_user_name" label="用户" min-width="90" sortable />
        <el-table-column prop="comment" label="说明" min-width="200" show-overflow-tooltip />
        <el-table-column prop="diff_lines" label="代码行数" min-width="90" align="right" sortable />
        <el-table-column label="实际耗时" min-width="100" align="right" sortable :sort-method="(a, b) => (a.commit_real_minutes_manual ?? a.commit_real_minutes ?? 0) - (b.commit_real_minutes_manual ?? b.commit_real_minutes ?? 0)">
          <template #default="{ row }">{{ formatDuration(row.commit_real_minutes_manual ?? row.commit_real_minutes) }}</template>
        </el-table-column>
        <el-table-column label="传统开发时长预估" min-width="140" align="right" sortable :sort-method="(a, b) => (a.commit_ancient_minutes_manual ?? a.commit_ancient_minutes ?? 0) - (b.commit_ancient_minutes_manual ?? b.commit_ancient_minutes ?? 0)">
          <template #default="{ row }">{{ formatDuration(row.commit_ancient_minutes_manual ?? row.commit_ancient_minutes) }}</template>
        </el-table-column>
        <el-table-column label="硅含量" min-width="90" align="center" sortable prop="silica">
          <template #default="{ row }">
            <el-tag v-if="row.silica != null" :type="row.silica >= 80 ? 'success' : row.silica >= 50 ? 'primary' : 'info'" size="small">
              {{ row.silica.toFixed(1) }}%
            </el-tag>
            <el-tag v-else type="info" size="small">-</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="提效比" min-width="90" align="center" sortable :sort-method="(a, b) => commitEffRatio(a) - commitEffRatio(b)">
          <template #default="{ row }">
            <template v-if="commitEffRatio(row) > 0">
              <el-tag :type="commitEffRatio(row) >= 300 ? 'success' : commitEffRatio(row) >= 150 ? 'primary' : 'info'" size="small">
                {{ commitEffRatio(row).toFixed(1) }}%
              </el-tag>
            </template>
            <el-tag v-else type="info" size="small">-</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="费用" min-width="80" align="right" sortable prop="cost">
          <template #default="{ row }">{{ row.cost != null && row.cost > 0 ? row.cost.toFixed(2) : '-' }}</template>
        </el-table-column>
        <el-table-column label="Tokens消耗" min-width="110" align="right" sortable :sort-method="(a, b) => ((a.upstream_tokens || 0) + (a.downstream_tokens || 0)) - ((b.upstream_tokens || 0) + (b.downstream_tokens || 0))">
          <template #default="{ row }">{{ (row.upstream_tokens || 0) + (row.downstream_tokens || 0) > 0 ? ((row.upstream_tokens || 0) + (row.downstream_tokens || 0)).toLocaleString() : '-' }}</template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Tasks 表格 -->
    <el-card v-if="tasks.length > 0" shadow="never">
      <template #header><span>Tasks ({{ tasks.length }})</span></template>
      <el-table :data="tasks" style="width: 100%" row-class-name="kb-clickable-row" @row-click="(row) => router.push('/task/' + row.task_id)" empty-text="暂无数据">
        <el-table-column label="Task ID" min-width="100" show-overflow-tooltip sortable prop="task_id">
          <template #default="{ row }">
            <el-link type="primary" @click.stop="router.push('/task/' + row.task_id)">{{ (row.task_id || '').substring(0, 8) }}</el-link>
          </template>
        </el-table-column>
        <el-table-column label="时间" min-width="150" sortable prop="start_time" :formatter="(row, col, val) => formatLocalTime(row.start_time)" />
        <el-table-column prop="user_name" label="用户" min-width="90" sortable />
        <el-table-column prop="title" label="说明" min-width="200" show-overflow-tooltip />
        <el-table-column prop="diff_lines" label="代码行数" min-width="90" align="right" sortable />
        <el-table-column label="实际耗时" min-width="100" align="right" sortable :sort-method="(a, b) => (a.task_real_minutes_manual ?? a.task_real_minutes ?? 0) - (b.task_real_minutes_manual ?? b.task_real_minutes ?? 0)">
          <template #default="{ row }">{{ formatDuration(row.task_real_minutes_manual ?? row.task_real_minutes) }}</template>
        </el-table-column>
        <el-table-column label="传统开发时长预估" min-width="140" align="right" sortable :sort-method="(a, b) => (a.task_ancient_minutes_manual ?? a.task_ancient_minutes ?? 0) - (b.task_ancient_minutes_manual ?? b.task_ancient_minutes ?? 0)">
          <template #default="{ row }">{{ formatDuration(row.task_ancient_minutes_manual ?? row.task_ancient_minutes) }}</template>
        </el-table-column>
        <el-table-column label="提效比" min-width="90" align="center" sortable :sort-method="(a, b) => taskEffRatio(a) - taskEffRatio(b)">
          <template #default="{ row }">
            <template v-if="taskEffRatio(row) > 0">
              <el-tag :type="taskEffRatio(row) >= 300 ? 'success' : taskEffRatio(row) >= 150 ? 'primary' : 'info'" size="small">
                {{ taskEffRatio(row).toFixed(1) }}%
              </el-tag>
            </template>
            <el-tag v-else type="info" size="small">-</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="费用" min-width="80" align="right" sortable prop="cost">
          <template #default="{ row }">{{ row.cost != null && row.cost > 0 ? row.cost.toFixed(2) : '-' }}</template>
        </el-table-column>
        <el-table-column label="Tokens消耗" min-width="110" align="right" sortable :sort-method="(a, b) => ((a.upstream_tokens || 0) + (a.downstream_tokens || 0)) - ((b.upstream_tokens || 0) + (b.downstream_tokens || 0))">
          <template #default="{ row }">{{ (row.upstream_tokens || 0) + (row.downstream_tokens || 0) > 0 ? ((row.upstream_tokens || 0) + (row.downstream_tokens || 0)).toLocaleString() : '-' }}</template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 添加到 Project 对话框 -->
    <el-dialog v-model="showAddToProjectDialog" title="添加到 Project" width="750px">
      <el-form label-width="100px">
        <el-form-item label="目标 Project">
          <el-select v-model="selectedProjectId" placeholder="选择 Project" style="width: 100%">
            <el-option label="+ 新建 Project" value="__new__" />
            <el-option v-for="p in projectList" :key="p.project_id" :label="p.name" :value="p.project_id" />
          </el-select>
        </el-form-item>
        <template v-if="selectedProjectId === '__new__'">
          <el-form-item label="名称">
            <el-input v-model="newProjectName" placeholder="Project 名称" />
          </el-form-item>
          <el-form-item label="描述">
            <el-input v-model="newProjectDesc" placeholder="Project 描述（可选）" />
          </el-form-item>
        </template>
        <el-form-item label="时间范围">
          <el-date-picker v-model="addProjectDateRange" type="daterange" range-separator="~" start-placeholder="开始" end-placeholder="结束" value-format="YYYY-MM-DD" style="width: 100%" />
        </el-form-item>
        <el-form-item label="白名单模式">
          <el-switch v-model="whitelistMode" active-text="仅包含指定 Commits" />
        </el-form-item>
      </el-form>
      <template v-if="whitelistMode">
        <el-table ref="whitelistTableRef" :data="commits" style="width: 100%" max-height="300" @selection-change="handleWhitelistChange">
          <el-table-column type="selection" width="45" />
          <el-table-column label="Commit ID" width="100">
            <template #default="{ row }">{{ (row.commit_id || '').substring(0, 8) }}</template>
          </el-table-column>
          <el-table-column prop="comment" label="说明" min-width="180" show-overflow-tooltip />
          <el-table-column prop="git_user_name" label="用户" width="90" />
          <el-table-column label="时间" width="150">
            <template #default="{ row }">{{ formatLocalTime(row.commit_time) }}</template>
          </el-table-column>
          <el-table-column prop="diff_lines" label="代码行数" width="90" align="right" />
        </el-table>
      </template>
      <el-alert v-if="conflicts.length > 0" type="warning" :closable="false" style="margin-top: 12px">
        <template #title>以下 Commits 已属于其他 Project：</template>
        <div v-for="c in conflicts" :key="c.commit_id" style="font-size: 12px">
          {{ (c.commit_id || '').substring(0, 8) }} → {{ c.project_name }}
        </div>
      </el-alert>
      <template #footer>
        <el-button @click="showAddToProjectDialog = false">取消</el-button>
        <el-button v-if="conflicts.length > 0 && conflictsChecked" type="warning" :loading="addingToProject" @click="doAddToProject">仍然添加</el-button>
        <el-button v-else type="primary" :loading="addingToProject" @click="confirmAddToProject">确认</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ArrowLeft, QuestionFilled } from '@element-plus/icons-vue'
import { getRepoDetailV2New, getRepoBranches, getProjects, createProject, addRepoToProject, checkProjectConflicts } from '@/api/es'
import { ElMessage } from 'element-plus'
import { fmtCost, formatDuration, formatLocalTime } from '@/utils/formatters'
import { getDefaultDateRangeWide } from '@/utils/date'
import DateRangePicker from '@/components/DateRangePicker.vue'

const router = useRouter()
const route = useRoute()

const loading = ref(false)
const dateRange = ref([])

const repoAddr = computed(() => route.params.repoAddr ? decodeURIComponent(route.params.repoAddr) : '')
const currentBranch = ref('')
const branches = ref([])

const commits = ref([])
const tasks = ref([])
const efficiency = ref({})

// 添加到 Project 对话框
const showAddToProjectDialog = ref(false)
const projectList = ref([])
const selectedProjectId = ref('')
const newProjectName = ref('')
const newProjectDesc = ref('')
const addProjectDateRange = ref(null)
const whitelistMode = ref(false)
const whitelistCommits = ref([])
const addingToProject = ref(false)
const conflictsChecked = ref(false)
const conflicts = ref([])
const whitelistTableRef = ref(null)

function fmtDate(d) {
  const pad = n => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

// 计算属性
const efficiencyColor = computed(() => {
  const ratio = efficiency.value.efficiency_ratio
  if (ratio == null) return '#909399'
  if (ratio >= 300) return '#67C23A'
  if (ratio >= 150) return '#409EFF'
  return '#909399'
})

const totalDiffLines = computed(() => {
  return commits.value.reduce((s, c) => s + (c.diff_lines || 0), 0)
})

const contributorCount = computed(() => {
  const names = new Set()
  commits.value.forEach(c => { if (c.git_user_name) names.add(c.git_user_name) })
  tasks.value.forEach(t => { if (t.user_name) names.add(t.user_name) })
  return names.size
})

const totalTokens = computed(() => {
  return tasks.value.reduce((s, t) => s + (t.upstream_tokens || 0) + (t.downstream_tokens || 0), 0)
})

const totalCost = computed(() => {
  return tasks.value.reduce((s, t) => s + (t.cost || 0), 0)
})

const activityRange = computed(() => {
  const times = commits.value
    .map(c => c.commit_time)
    .filter(Boolean)
    .map(t => new Date(t).getTime())
  if (times.length === 0) return '-'
  const min = new Date(Math.min(...times))
  const max = new Date(Math.max(...times))
  return `${fmtDate(min)} ~ ${fmtDate(max)}`
})

function commitEffRatio(row) {
  const ancient = row.commit_ancient_minutes_manual ?? row.commit_ancient_minutes
  const real = row.commit_real_minutes_manual ?? row.commit_real_minutes
  if (ancient > 0 && real > 0) return (ancient / real) * 100
  return 0
}

function taskEffRatio(row) {
  const ancient = row.task_ancient_minutes_manual ?? row.task_ancient_minutes
  const real = row.task_real_minutes_manual ?? row.task_real_minutes
  if (ancient > 0 && real > 0) return (ancient / real) * 100
  return 0
}

function handleCommitClick(row) {
  router.push('/commit/' + row.commit_id)
}

function onDateChange() {
  if (dateRange.value && dateRange.value.length === 2) {
    router.replace({
      query: {
        startDate: dateRange.value[0].replace(/-/g, ''),
        endDate: dateRange.value[1].replace(/-/g, ''),
      }
    })
  }
  loadData()
}

function handleBranchChange(branch) {
  const query = {}
  if (dateRange.value && dateRange.value.length === 2) {
    query.startDate = dateRange.value[0].replace(/-/g, '')
    query.endDate = dateRange.value[1].replace(/-/g, '')
  }
  router.push({ path: '/repo/' + encodeURIComponent(repoAddr.value) + '/' + encodeURIComponent(branch), query })
}

async function loadBranches() {
  if (!repoAddr.value) return
  try {
    const result = await getRepoBranches(repoAddr.value)
    const data = result.data || result
    branches.value = data.branches || data || []
  } catch {
    branches.value = []
  }
}

async function loadData() {
  if (!repoAddr.value) return
  loading.value = true
  try {
    const params = {}
    if (dateRange.value && dateRange.value.length === 2) {
      params.startDate = dateRange.value[0].replace(/-/g, '')
      params.endDate = dateRange.value[1].replace(/-/g, '')
    }
    const result = await getRepoDetailV2New(repoAddr.value, currentBranch.value, params)
    const data = result.data || result
    commits.value = data.commits || []
    tasks.value = data.tasks || []
    efficiency.value = data.efficiency || {}
    if (data.branches && data.branches.length > 0) {
      branches.value = data.branches
    }
  } catch {
    commits.value = []
    tasks.value = []
    efficiency.value = {}
  } finally {
    loading.value = false
  }
}

// 添加到 Project
async function openAddToProject() {
  showAddToProjectDialog.value = true
  selectedProjectId.value = ''
  newProjectName.value = ''
  newProjectDesc.value = ''
  addProjectDateRange.value = null
  whitelistMode.value = false
  whitelistCommits.value = []
  conflictsChecked.value = false
  conflicts.value = []
  try {
    const result = await getProjects()
    projectList.value = result.data?.data || []
  } catch {
    projectList.value = []
  }
}

function handleWhitelistChange(selection) {
  whitelistCommits.value = selection
}

function getTargetCommitIds() {
  if (whitelistMode.value) {
    return whitelistCommits.value.map(c => c.commit_id)
  }
  let list = commits.value
  if (addProjectDateRange.value && addProjectDateRange.value.length === 2) {
    const start = addProjectDateRange.value[0]
    const end = addProjectDateRange.value[1]
    list = list.filter(c => {
      const d = c.commit_time ? c.commit_time.substring(0, 10) : ''
      return d >= start && d <= end
    })
  }
  return list.map(c => c.commit_id)
}

async function confirmAddToProject() {
  if (!selectedProjectId.value) {
    ElMessage.warning('请选择目标 Project')
    return
  }
  if (selectedProjectId.value === '__new__' && !newProjectName.value.trim()) {
    ElMessage.warning('请输入 Project 名称')
    return
  }
  if (!conflictsChecked.value) {
    const commitIds = getTargetCommitIds()
    if (commitIds.length === 0) {
      ElMessage.warning('没有可添加的 Commits')
      return
    }
    try {
      addingToProject.value = true
      const result = await checkProjectConflicts({ commit_ids: commitIds })
      const data = result.data || result
      conflicts.value = data.conflicts || []
      conflictsChecked.value = true
      if (conflicts.value.length > 0) {
        addingToProject.value = false
        return
      }
    } catch {
      ElMessage.error('冲突检测失败')
      addingToProject.value = false
      return
    }
  }
  await doAddToProject()
}

async function doAddToProject() {
  try {
    addingToProject.value = true
    let projectId = selectedProjectId.value
    if (projectId === '__new__') {
      const res = await createProject({ name: newProjectName.value.trim(), description: newProjectDesc.value.trim() })
      const data = res.data || res
      projectId = data.project_id
    }
    const repoFilter = {
      repo_addr: repoAddr.value,
      repo_branch: currentBranch.value,
      start_time: (addProjectDateRange.value && addProjectDateRange.value[0]) || null,
      end_time: (addProjectDateRange.value && addProjectDateRange.value[1]) || null,
      include_only_commits: whitelistMode.value ? whitelistCommits.value.map(c => c.commit_id) : [],
      exclude_commits: []
    }
    await addRepoToProject(projectId, repoFilter)
    ElMessage.success('添加成功')
    showAddToProjectDialog.value = false
  } catch {
    ElMessage.error('添加失败')
  } finally {
    addingToProject.value = false
  }
}

onMounted(async () => {
  const branchFromRoute = route.params.repoBranch ? decodeURIComponent(route.params.repoBranch) : ''
  currentBranch.value = branchFromRoute
  // 从 URL query 参数初始化日期范围
  const { startDate, endDate } = route.query
  if (startDate && endDate) {
    const fmt = s => s.slice(0, 4) + '-' + s.slice(4, 6) + '-' + s.slice(6, 8)
    dateRange.value = [fmt(startDate), fmt(endDate)]
  } else {
    dateRange.value = getDefaultDateRangeWide()
  }
  await Promise.all([loadBranches(), loadData()])
  if (!currentBranch.value && branches.value.length > 0) {
    currentBranch.value = branches.value[0]
  }
})

watch(() => route.params.repoBranch, (newBranch) => {
  if (newBranch) {
    currentBranch.value = decodeURIComponent(newBranch)
    loadData()
  }
})

watch(() => route.query, (query) => {
  const { startDate, endDate } = query
  if (startDate && endDate) {
    const fmt = s => s.slice(0, 4) + '-' + s.slice(4, 6) + '-' + s.slice(6, 8)
    const newRange = [fmt(startDate), fmt(endDate)]
    if (newRange[0] !== dateRange.value[0] || newRange[1] !== dateRange.value[1]) {
      dateRange.value = newRange
      loadData()
    }
  }
}, { deep: true })
</script>

<style>
.reason-tooltip {
  max-width: 400px !important;
}
.metric-reason {
  margin-left: 4px;
  color: #909399;
  cursor: help;
  vertical-align: middle;
}
</style>
