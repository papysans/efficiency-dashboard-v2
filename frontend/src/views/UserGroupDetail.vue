<template>
  <div class="kb-panel">
    <!-- 标题栏 -->
    <el-card shadow="never">
      <div style="display: flex; align-items: center; justify-content: space-between">
        <div style="display: flex; align-items: center; gap: 12px">
          <el-button :icon="ArrowLeft" @click="router.back()">返回</el-button>
          <span style="font-size: 18px; font-weight: bold">虚拟组: {{ groupData?.group?.name || '-' }}</span>
        </div>
        <el-button type="danger" size="small" @click="handleDelete">删除此组</el-button>
      </div>
    </el-card>

    <!-- 日期范围筛选 -->
    <FilterBar v-model:dateRange="dateRange" @search="fetchData" />

    <!-- 顶部汇总卡片 -->
    <el-row :gutter="12" v-loading="loading">
      <el-col :span="4" style="margin-bottom: 12px">
        <el-card shadow="never" class="kb-metric-card">
          <div class="kb-metric-label">成员数</div>
          <div class="kb-metric-value">{{ groupData?.members?.length ?? '-' }}</div>
        </el-card>
      </el-col>
      <el-col :span="4" style="margin-bottom: 12px">
        <el-card shadow="never" class="kb-metric-card">
          <div class="kb-metric-label">总Task数</div>
          <div class="kb-metric-value">{{ groupData?.summary?.task_count ?? '-' }}</div>
        </el-card>
      </el-col>
      <el-col :span="4" style="margin-bottom: 12px">
        <el-card shadow="never" class="kb-metric-card">
          <div class="kb-metric-label">总Commit数</div>
          <div class="kb-metric-value">{{ groupData?.summary?.commit_count ?? '-' }}</div>
        </el-card>
      </el-col>
      <el-col :span="4" style="margin-bottom: 12px">
        <el-card shadow="never" class="kb-metric-card">
          <div class="kb-metric-label">加权Task提效比</div>
          <div class="kb-metric-value">{{ groupData?.summary?.task_efficiency_ratio != null ? groupData.summary.task_efficiency_ratio.toFixed(1) + '%' : '-' }}</div>
        </el-card>
      </el-col>
      <el-col :span="4" style="margin-bottom: 12px">
        <el-card shadow="never" class="kb-metric-card">
          <div class="kb-metric-label">加权Commit提效比</div>
          <div class="kb-metric-value">{{ groupData?.summary?.commit_efficiency_ratio != null ? groupData.summary.commit_efficiency_ratio.toFixed(1) + '%' : '-' }}</div>
        </el-card>
      </el-col>
      <el-col :span="4" style="margin-bottom: 12px">
        <el-card shadow="never" class="kb-metric-card">
          <div class="kb-metric-label">总费用</div>
          <div class="kb-metric-value">{{ groupData?.summary?.cost != null ? fmtCostVal(groupData.summary.cost) : '-' }}</div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 成员明细表格 -->
    <el-card shadow="never" class="kb-table-card">
      <template #header><span>成员明细</span></template>
      <el-table
        :data="groupData?.members || []"
        stripe
        border
        v-loading="loading"
        style="width: 100%"
        row-class-name="kb-clickable-row"
        @row-click="handleRowClick"
        empty-text="暂无数据"
      >
        <el-table-column prop="user_name" label="用户名" min-width="150" show-overflow-tooltip />
        <el-table-column prop="day_count" label="活跃天数" width="100" align="right" sortable />
        <el-table-column prop="task_count" label="Task数" width="90" align="right" sortable />
        <el-table-column prop="commit_count" label="Commit数" width="100" align="right" sortable />
        <el-table-column prop="task_efficiency_ratio" label="Task提效比" width="110" align="center" sortable>
          <template #default="{ row }">
            <el-tag
              v-if="row.task_efficiency_ratio != null"
              :type="row.task_efficiency_ratio >= 300 ? 'success' : row.task_efficiency_ratio >= 150 ? 'primary' : 'info'"
              size="small"
            >
              {{ row.task_efficiency_ratio.toFixed(1) }}%
            </el-tag>
            <el-tag v-else type="info" size="small">-</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="commit_efficiency_ratio" label="Commit提效比" width="120" align="center" sortable>
          <template #default="{ row }">
            <el-tag
              v-if="row.commit_efficiency_ratio != null"
              :type="row.commit_efficiency_ratio >= 300 ? 'success' : row.commit_efficiency_ratio >= 150 ? 'primary' : 'info'"
              size="small"
            >
              {{ row.commit_efficiency_ratio.toFixed(1) }}%
            </el-tag>
            <el-tag v-else type="info" size="small">-</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="cost" label="费用" width="100" align="right" :formatter="fmtCost" sortable />
      </el-table>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft } from '@element-plus/icons-vue'
import FilterBar from '@/components/FilterBar.vue'
import { getUserGroupDetail, deleteUserGroup } from '@/api/es'
import { fmtCost } from '@/utils/formatters'
import { getDefaultDateRangeWide } from '@/utils/date'
import { ElMessage, ElMessageBox } from 'element-plus'

const route = useRoute()
const router = useRouter()

const loading = ref(false)
const groupData = ref(null)
const dateRange = ref(getDefaultDateRangeWide())

function fmtCostVal(val) {
  return fmtCost(null, null, val)
}

function handleRowClick(row) {
  router.push('/user/' + row.user_id)
}

async function handleDelete() {
  try {
    await ElMessageBox.confirm('确定要删除此虚拟组吗？删除后不可恢复。', '确认删除', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    })
    const groupId = route.params.groupId
    await deleteUserGroup(groupId)
    ElMessage.success('删除成功')
    router.push('/user-v2')
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error('删除失败')
    }
  }
}

async function fetchData() {
  const groupId = route.params.groupId
  if (!groupId) return
  if (!dateRange.value || dateRange.value.length !== 2) return

  loading.value = true
  try {
    const params = {
      startDate: dateRange.value[0].replace(/-/g, ''),
      endDate: dateRange.value[1].replace(/-/g, ''),
    }
    const result = await getUserGroupDetail(groupId, params)
    groupData.value = result.data || result
  } catch (err) {
    groupData.value = null
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchData()
})
</script>
