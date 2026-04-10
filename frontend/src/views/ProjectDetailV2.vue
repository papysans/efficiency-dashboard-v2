<template>
  <div class="kb-panel">
    <!-- 标题栏 -->
    <el-card shadow="never">
      <div style="display: flex; align-items: center; justify-content: space-between">
        <div style="display: flex; align-items: center; gap: 12px">
          <el-button :icon="ArrowLeft" @click="router.back()">返回</el-button>
          <div>
            <span style="font-size: 18px; font-weight: bold">{{ project.name || '项目详情' }}</span>
            <div v-if="project.description" style="font-size: 13px; color: #909399; margin-top: 4px">{{ project.description }}</div>
          </div>
        </div>
        <div style="display: flex; gap: 8px">
          <el-button type="warning" size="small" @click="showManualDialog = true">人工调整</el-button>
          <el-button type="primary" size="small" @click="showEditDialog = true">编辑</el-button>
        </div>
      </div>
    </el-card>

    <!-- 基础信息 -->
    <el-card shadow="never" header="基础信息" v-loading="loading">
      <el-descriptions :column="3" border>
        <el-descriptions-item label="项目ID">{{ project.project_id || '-' }}</el-descriptions-item>
        <el-descriptions-item label="起始时间">
          <template v-if="project.start_time_manual">
            {{ formatLocalTime(project.start_time_manual) }}
            <span v-if="project.start_time" style="margin-left: 8px; color: #C0C4CC; text-decoration: line-through; font-size: 12px;">
              {{ formatLocalTime(project.start_time) }}
            </span>
          </template>
          <template v-else>
            {{ formatLocalTime(project.start_time) }}
          </template>
        </el-descriptions-item>
        <el-descriptions-item label="结束时间">
          <template v-if="project.end_time_manual">
            {{ formatLocalTime(project.end_time_manual) }}
            <span v-if="project.end_time" style="margin-left: 8px; color: #C0C4CC; text-decoration: line-through; font-size: 12px;">
              {{ formatLocalTime(project.end_time) }}
            </span>
          </template>
          <template v-else>
            {{ formatLocalTime(project.end_time) }}
          </template>
        </el-descriptions-item>
        <el-descriptions-item label="Repo数">{{ project.repos ? project.repos.length : 0 }}</el-descriptions-item>
        <el-descriptions-item label="Task数">{{ tasks.length }}</el-descriptions-item>
        <el-descriptions-item label="Commit数">{{ commits.length }}</el-descriptions-item>
        <el-descriptions-item label="参与人数">{{ userCount }}</el-descriptions-item>
      </el-descriptions>
    </el-card>

    <!-- 度量信息 -->
    <el-card shadow="never" header="度量信息">
      <el-descriptions :column="3" border>
        <!-- 传统开发预估 -->
        <el-descriptions-item label="传统开发预估">
          <template v-if="project.project_ancient_minutes_manual != null">
            {{ formatDuration(project.project_ancient_minutes_manual) }}
            <el-tooltip :content="`汇聚项目内所有 Task 和 Commit 的传统开发预估时间之和${project.project_ancient_minutes_reason_manual ? '：' + project.project_ancient_minutes_reason_manual : ''}`" placement="top" :show-after="200" popper-class="reason-tooltip">
              <el-icon style="margin-left: 4px; color: #E6A23C; cursor: help; vertical-align: middle;"><QuestionFilled /></el-icon>
            </el-tooltip>
            <span v-if="project.project_ancient_minutes != null" style="margin-left: 8px; color: #C0C4CC; text-decoration: line-through; font-size: 12px;">
              {{ formatDuration(project.project_ancient_minutes) }}
            </span>
            <el-tooltip :content="`汇聚项目内所有 Task 和 Commit 的传统开发预估时间之和${project.project_ancient_minutes_reason ? '：' + project.project_ancient_minutes_reason : ''}`" placement="top" :show-after="200" popper-class="reason-tooltip">
              <el-icon style="margin-left: 2px; color: #C0C4CC; cursor: help; vertical-align: middle; font-size: 12px;"><QuestionFilled /></el-icon>
            </el-tooltip>
          </template>
          <template v-else>
            {{ formatDuration(project.project_ancient_minutes) }}
            <el-tooltip :content="`汇聚项目内所有 Task 和 Commit 的传统开发预估时间之和${project.project_ancient_minutes_reason ? '：' + project.project_ancient_minutes_reason : ''}`" placement="top" :show-after="200" popper-class="reason-tooltip">
              <el-icon style="margin-left: 4px; color: #909399; cursor: help; vertical-align: middle;"><QuestionFilled /></el-icon>
            </el-tooltip>
          </template>
        </el-descriptions-item>

        <!-- 实际处理耗时 -->
        <el-descriptions-item label="实际处理耗时">
          <template v-if="project.project_real_process_minutes_manual != null">
            {{ formatDuration(project.project_real_process_minutes_manual) }}
            <el-tooltip :content="`汇聚项目内所有 Task 和 Commit 的实际 AI 处理耗时之和（不含等待时间）${project.project_real_process_minutes_reason_manual ? '：' + project.project_real_process_minutes_reason_manual : ''}`" placement="top" :show-after="200" popper-class="reason-tooltip">
              <el-icon style="margin-left: 4px; color: #E6A23C; cursor: help; vertical-align: middle;"><QuestionFilled /></el-icon>
            </el-tooltip>
            <span v-if="project.project_real_process_minutes != null" style="margin-left: 8px; color: #C0C4CC; text-decoration: line-through; font-size: 12px;">
              {{ formatDuration(project.project_real_process_minutes) }}
            </span>
            <el-tooltip :content="`汇聚项目内所有 Task 和 Commit 的实际 AI 处理耗时之和（不含等待时间）${project.project_real_process_minutes_reason ? '：' + project.project_real_process_minutes_reason : ''}`" placement="top" :show-after="200" popper-class="reason-tooltip">
              <el-icon style="margin-left: 2px; color: #C0C4CC; cursor: help; vertical-align: middle; font-size: 12px;"><QuestionFilled /></el-icon>
            </el-tooltip>
          </template>
          <template v-else>
            {{ formatDuration(project.project_real_process_minutes) }}
            <el-tooltip :content="`汇聚项目内所有 Task 和 Commit 的实际 AI 处理耗时之和（不含等待时间）${project.project_real_process_minutes_reason ? '：' + project.project_real_process_minutes_reason : ''}`" placement="top" :show-after="200" popper-class="reason-tooltip">
              <el-icon style="margin-left: 4px; color: #909399; cursor: help; vertical-align: middle;"><QuestionFilled /></el-icon>
            </el-tooltip>
          </template>
        </el-descriptions-item>

        <!-- 项目周期 -->
        <el-descriptions-item label="项目周期">
          <template v-if="project.project_real_lead_minutes_manual != null">
            {{ formatDuration(project.project_real_lead_minutes_manual) }}
            <el-tooltip v-if="project.project_real_lead_minutes_reason_manual" :content="project.project_real_lead_minutes_reason_manual" placement="top" :show-after="200" popper-class="reason-tooltip">
              <el-icon style="margin-left: 4px; color: #E6A23C; cursor: help; vertical-align: middle;"><QuestionFilled /></el-icon>
            </el-tooltip>
            <span v-if="project.project_real_lead_minutes != null" style="margin-left: 8px; color: #C0C4CC; text-decoration: line-through; font-size: 12px;">
              {{ formatDuration(project.project_real_lead_minutes) }}
            </span>
            <el-tooltip v-if="project.project_real_lead_minutes_reason" :content="project.project_real_lead_minutes_reason" placement="top" :show-after="200" popper-class="reason-tooltip">
              <el-icon style="margin-left: 2px; color: #C0C4CC; cursor: help; vertical-align: middle; font-size: 12px;"><QuestionFilled /></el-icon>
            </el-tooltip>
          </template>
          <template v-else>
            {{ formatDuration(project.project_real_lead_minutes) }}
            <el-tooltip v-if="project.project_real_lead_minutes_reason" :content="project.project_real_lead_minutes_reason" placement="top" :show-after="200" popper-class="reason-tooltip">
              <el-icon style="margin-left: 4px; color: #909399; cursor: help; vertical-align: middle;"><QuestionFilled /></el-icon>
            </el-tooltip>
          </template>
        </el-descriptions-item>

        <el-descriptions-item label="总Tokens">
          {{ totalTokens > 0 ? totalTokens.toLocaleString() : '-' }}
          <el-tooltip :content="`上行 Tokens（用户输入）：${(project.upstream_tokens || 0).toLocaleString()}，下行 Tokens（AI 输出）：${(project.downstream_tokens || 0).toLocaleString()}`" placement="top" :show-after="200" popper-class="reason-tooltip">
            <el-icon style="margin-left: 4px; color: #909399; cursor: help; vertical-align: middle;"><QuestionFilled /></el-icon>
          </el-tooltip>
        </el-descriptions-item>
        <el-descriptions-item label="总费用">{{ project.cost != null && project.cost > 0 ? fmtCostVal(project.cost) + ' 元' : '-' }}</el-descriptions-item>
        <el-descriptions-item label="生成代码量">
          {{ totalCodeLines > 0 ? totalCodeLines.toLocaleString() + ' 行' : '-' }}
          <el-tooltip content="项目内所有 Commit 的代码变更行数（diff_lines）之和" placement="top" :show-after="200" popper-class="reason-tooltip">
            <el-icon style="margin-left: 4px; color: #909399; cursor: help; vertical-align: middle;"><QuestionFilled /></el-icon>
          </el-tooltip>
        </el-descriptions-item>
        <el-descriptions-item label="实际耗时">
          {{ actualWorkDays != null ? actualWorkDays.toFixed(2) + ' 人天' : '-' }}
          <el-tooltip content="等同于实际处理耗时，以人天（8小时/天）为单位展示，来源：汇聚项目内所有 Task 和 Commit 的实际 AI 处理耗时之和" placement="top" :show-after="200" popper-class="reason-tooltip">
            <el-icon style="margin-left: 4px; color: #909399; cursor: help; vertical-align: middle;"><QuestionFilled /></el-icon>
          </el-tooltip>
        </el-descriptions-item>

        <!-- 实际人天代码量 -->
        <el-descriptions-item label="实际人天代码量">
          {{ actualLinesPerDay != null ? actualLinesPerDay.toFixed(0) + ' 行/人天' : '-' }}
          <el-tooltip content="生成代码量 ÷ 实际耗时（人天），反映 AI 辅助下实际的代码产出效率" placement="top" :show-after="200" popper-class="reason-tooltip">
            <el-icon style="margin-left: 4px; color: #909399; cursor: help; vertical-align: middle;"><QuestionFilled /></el-icon>
          </el-tooltip>
        </el-descriptions-item>

        <!-- 传统开发人天代码量 -->
        <el-descriptions-item label="传统开发人天代码量">
          {{ traditionalLinesPerDay != null ? traditionalLinesPerDay.toFixed(0) + ' 行/人天' : '-' }}
          <el-tooltip :content="`生成代码量 ÷ 传统开发预估（人天），可与企业传统基准（${traditionalDevLinesPerDay} 行/人天）对比验证预估合理性`" placement="top" :show-after="200" popper-class="reason-tooltip">
            <el-icon style="margin-left: 4px; color: #909399; cursor: help; vertical-align: middle;"><QuestionFilled /></el-icon>
          </el-tooltip>
        </el-descriptions-item>

        <!-- 开发提效比 -->
        <el-descriptions-item label="开发提效比">
          <span :style="{ color: getEfficiencyColor(devEfficiencyRatio != null ? devEfficiencyRatio * 100 : null), fontSize: '20px', fontWeight: 'bold' }">
            {{ devEfficiencyRatio != null ? Math.round(devEfficiencyRatio * 100) + '%' : '-' }}
          </span>
          <el-tooltip content="传统开发预估 ÷ 实际耗时，反映 AI 工具在纯开发环节的提效倍数" placement="top" :show-after="200" popper-class="reason-tooltip">
            <el-icon style="margin-left: 4px; color: #909399; cursor: help; vertical-align: middle;"><QuestionFilled /></el-icon>
          </el-tooltip>
        </el-descriptions-item>

        <!-- 端到端提效比 -->
        <el-descriptions-item label="端到端提效比">
          <span :style="{ color: getEfficiencyColor(e2eEfficiencyRatio != null ? e2eEfficiencyRatio * 100 : null), fontSize: '20px', fontWeight: 'bold' }">
            {{ e2eEfficiencyRatio != null ? Math.round(e2eEfficiencyRatio * 100) + '%' : '-' }}
          </span>
          <el-tooltip content="传统开发预估 ÷ 项目周期（含等待、评审等），反映整个项目流程的端到端提效倍数" placement="top" :show-after="200" popper-class="reason-tooltip">
            <el-icon style="margin-left: 4px; color: #909399; cursor: help; vertical-align: middle;"><QuestionFilled /></el-icon>
          </el-tooltip>
        </el-descriptions-item>
      </el-descriptions>
    </el-card>

    <!-- 用户视角 -->
    <el-card shadow="never">
      <template #header><span>用户视角 ({{ userStats.length }})</span></template>
      <el-table :data="userStats" style="width: 100%" empty-text="暂无数据" :default-sort="{ prop: 'commit_diff_lines', order: 'descending' }">
        <el-table-column prop="user_name" label="用户" min-width="100" />
        <el-table-column prop="task_count" label="Task数" width="90" align="right" sortable />
        <el-table-column prop="commit_count" label="Commit数" width="100" align="right" sortable />
        <el-table-column prop="commit_diff_lines" label="代码行数" width="100" align="right" sortable />
        <el-table-column label="Task传统预估" width="130" align="right" sortable prop="task_ancient_minutes">
          <template #default="{ row }">{{ formatDuration(row.task_ancient_minutes) }}</template>
        </el-table-column>
        <el-table-column label="Task实际耗时" width="130" align="right" sortable prop="task_real_minutes">
          <template #default="{ row }">{{ formatDuration(row.task_real_minutes) }}</template>
        </el-table-column>
        <el-table-column label="Task提效比" width="110" align="center" sortable prop="task_efficiency_ratio">
          <template #default="{ row }">
            <el-tag v-if="row.task_efficiency_ratio > 0"
              :type="row.task_efficiency_ratio >= 300 ? 'success' : row.task_efficiency_ratio >= 150 ? 'primary' : 'info'"
              size="small">{{ row.task_efficiency_ratio.toFixed(1) }}%</el-tag>
            <el-tag v-else type="info" size="small">-</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="Commit传统预估" width="140" align="right" sortable prop="commit_ancient_minutes">
          <template #default="{ row }">{{ formatDuration(row.commit_ancient_minutes) }}</template>
        </el-table-column>
        <el-table-column label="Commit实际耗时" width="140" align="right" sortable prop="commit_real_minutes">
          <template #default="{ row }">{{ formatDuration(row.commit_real_minutes) }}</template>
        </el-table-column>
        <el-table-column label="Commit提效比" width="120" align="center" sortable prop="commit_efficiency_ratio">
          <template #default="{ row }">
            <el-tag v-if="row.commit_efficiency_ratio > 0"
              :type="row.commit_efficiency_ratio >= 300 ? 'success' : row.commit_efficiency_ratio >= 150 ? 'primary' : 'info'"
              size="small">{{ row.commit_efficiency_ratio.toFixed(1) }}%</el-tag>
            <el-tag v-else type="info" size="small">-</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="费用" width="100" align="right" sortable prop="cost">
          <template #default="{ row }">{{ row.cost > 0 ? fmtCostVal(row.cost) : '-' }}</template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Repos 配置 -->
    <el-card shadow="never">
      <template #header>
        <div style="display: flex; align-items: center; justify-content: space-between">
          <span>Repos ({{ project.repos ? project.repos.length : 0 }})</span>
          <el-button type="primary" :icon="Plus" size="small" @click="openAddRepoDialog">添加 Repo</el-button>
        </div>
      </template>
      <el-table :data="project.repos || []" style="width: 100%" empty-text="暂无 Repo 配置">
        <el-table-column label="仓库地址" min-width="200" show-overflow-tooltip sortable prop="repo_addr">
          <template #default="{ row }">
            <el-link type="primary" @click="router.push('/repo/' + encodeURIComponent(row.repo_addr) + (row.repo_branch ? '/' + encodeURIComponent(row.repo_branch) : ''))">{{ row.repo_addr }}</el-link>
          </template>
        </el-table-column>
        <el-table-column prop="repo_branch" label="分支" min-width="100" sortable />
        <el-table-column label="开始时间" min-width="150" sortable prop="start_time" :formatter="(row) => formatLocalTime(row.start_time)" />
        <el-table-column label="结束时间" min-width="150" sortable prop="end_time" :formatter="(row) => formatLocalTime(row.end_time)" />
        <el-table-column label="白名单commits" min-width="120" align="right" sortable :sort-method="(a, b) => (a.include_only_commits?.length || 0) - (b.include_only_commits?.length || 0)">
          <template #default="{ row }">{{ row.include_only_commits ? row.include_only_commits.length : 0 }}</template>
        </el-table-column>
        <el-table-column label="排除commits" min-width="120" align="right" sortable :sort-method="(a, b) => (a.exclude_commits?.length || 0) - (b.exclude_commits?.length || 0)">
          <template #default="{ row }">{{ row.exclude_commits ? row.exclude_commits.length : 0 }}</template>
        </el-table-column>
        <el-table-column label="操作" width="120" align="center">
          <template #default="{ row, $index }">
            <el-button type="primary" link size="small" @click="openEditRepoDialog(row, $index)">编辑</el-button>
            <el-button type="danger" link size="small" @click="handleRemoveRepo($index)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Tasks 列表 -->
    <el-card shadow="never">
      <template #header>
        <div style="display: flex; align-items: center; justify-content: space-between">
          <span>Tasks ({{ tasks.length }})</span>
          <el-button type="primary" :icon="Plus" size="small" @click="openAddTaskDialog">添加 Task</el-button>
        </div>
      </template>
      <el-table :data="tasks" style="width: 100%" row-class-name="kb-clickable-row" @row-click="(row) => router.push('/task/' + row.task_id)" empty-text="暂无数据">
        <el-table-column label="Task ID" min-width="100" show-overflow-tooltip sortable prop="task_id">
          <template #default="{ row }">
            <el-link type="primary" @click.stop="router.push('/task/' + row.task_id)">{{ (row.task_id || '').substring(0, 8) }}</el-link>
          </template>
        </el-table-column>
        <el-table-column prop="user_name" label="用户" min-width="90" sortable />
        <el-table-column label="开始时间" min-width="150" sortable prop="start_time" :formatter="(row) => formatLocalTime(row.start_time)" />
        <el-table-column label="传统预估" min-width="120" align="right" sortable :sort-method="(a, b) => (a.task_ancient_minutes_manual ?? a.task_ancient_minutes ?? 0) - (b.task_ancient_minutes_manual ?? b.task_ancient_minutes ?? 0)">
          <template #default="{ row }">{{ formatDuration(row.task_ancient_minutes_manual ?? row.task_ancient_minutes) }}</template>
        </el-table-column>
        <el-table-column label="实际耗时" min-width="120" align="right" sortable :sort-method="(a, b) => (a.task_real_minutes_manual ?? a.task_real_minutes ?? 0) - (b.task_real_minutes_manual ?? b.task_real_minutes ?? 0)">
          <template #default="{ row }">{{ formatDuration(row.task_real_minutes_manual ?? row.task_real_minutes) }}</template>
        </el-table-column>
        <el-table-column label="Silica权重" min-width="100" align="right" sortable :sort-method="(a, b) => (a.silica ?? 1.0) - (b.silica ?? 1.0)">
          <template #default="{ row }">{{ row.silica ?? 1.0 }}</template>
        </el-table-column>
        <el-table-column label="费用" min-width="80" align="right" sortable prop="cost">
          <template #default="{ row }">{{ row.cost != null && row.cost > 0 ? fmtCostVal(row.cost) : '-' }}</template>
        </el-table-column>
        <el-table-column label="操作" width="120" align="center">
          <template #default="{ row }">
            <el-button type="primary" link size="small" @click.stop="openEditSilicaDialog(row)">编辑</el-button>
            <el-button type="danger" link size="small" @click.stop="handleRemoveTask(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Commits 列表 -->
    <el-card shadow="never">
      <template #header><span>Commits ({{ commits.length }})</span></template>
      <el-table :data="commits" style="width: 100%" row-class-name="kb-clickable-row" @row-click="(row) => router.push('/commit/' + row.commit_id)" empty-text="暂无数据">
        <el-table-column label="Commit ID" min-width="100" show-overflow-tooltip sortable prop="commit_id">
          <template #default="{ row }">
            <el-link type="primary" @click.stop="router.push('/commit/' + row.commit_id)">{{ (row.commit_id || '').substring(0, 8) }}</el-link>
          </template>
        </el-table-column>
        <el-table-column prop="user_name" label="用户" min-width="90" sortable />
        <el-table-column label="时间" min-width="150" sortable prop="commit_time" :formatter="(row) => formatLocalTime(row.commit_time)" />
        <el-table-column prop="comment" label="说明" min-width="200" show-overflow-tooltip />
        <el-table-column prop="diff_lines" label="代码行数" min-width="90" align="right" sortable />
        <el-table-column label="传统预估" min-width="120" align="right" sortable :sort-method="(a, b) => (a.commit_ancient_minutes_manual ?? a.commit_ancient_minutes ?? 0) - (b.commit_ancient_minutes_manual ?? b.commit_ancient_minutes ?? 0)">
          <template #default="{ row }">{{ formatDuration(row.commit_ancient_minutes_manual ?? row.commit_ancient_minutes) }}</template>
        </el-table-column>
        <el-table-column label="实际耗时" min-width="120" align="right" sortable :sort-method="(a, b) => (a.commit_real_minutes_manual ?? a.commit_real_minutes ?? 0) - (b.commit_real_minutes_manual ?? b.commit_real_minutes ?? 0)">
          <template #default="{ row }">{{ formatDuration(row.commit_real_minutes_manual ?? row.commit_real_minutes) }}</template>
        </el-table-column>
        <el-table-column label="硅含量" min-width="90" align="center" sortable :sort-by="['silica']">
          <template #default="{ row }">
            <el-tag v-if="row.silica != null" :type="row.silica >= 80 ? 'success' : row.silica >= 50 ? 'primary' : 'info'" size="small">
              {{ row.silica.toFixed(1) }}%
            </el-tag>
            <el-tag v-else type="info" size="small">-</el-tag>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 人工调整对话框 -->
    <el-dialog v-model="showManualDialog" title="人工调整" width="650px">
      <el-form label-width="180px">
        <el-form-item label="传统开发预估(分钟)">
          <el-input-number v-model="manualForm.project_ancient_minutes_manual" :precision="2" :step="10" />
        </el-form-item>
        <el-form-item label="传统开发预估理由">
          <el-input v-model="manualForm.project_ancient_minutes_reason_manual" type="textarea" :rows="2" />
        </el-form-item>
        <el-form-item label="实际处理耗时(分钟)">
          <el-input-number v-model="manualForm.project_real_process_minutes_manual" :precision="2" :step="10" />
        </el-form-item>
        <el-form-item label="实际处理耗时理由">
          <el-input v-model="manualForm.project_real_process_minutes_reason_manual" type="textarea" :rows="2" />
        </el-form-item>
        <el-form-item label="项目周期(分钟)">
          <el-input-number v-model="manualForm.project_real_lead_minutes_manual" :precision="2" :step="10" />
        </el-form-item>
        <el-form-item label="项目周期理由">
          <el-input v-model="manualForm.project_real_lead_minutes_reason_manual" type="textarea" :rows="2" />
        </el-form-item>
        <el-form-item label="开始时间">
          <el-date-picker v-model="manualForm.start_time_manual" type="datetime" placeholder="选择开始时间" style="width: 100%" />
        </el-form-item>
        <el-form-item label="结束时间">
          <el-date-picker v-model="manualForm.end_time_manual" type="datetime" placeholder="选择结束时间" style="width: 100%" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showManualDialog = false">取消</el-button>
        <el-button type="primary" @click="submitManual" :loading="manualSubmitting">保存</el-button>
      </template>
    </el-dialog>

    <!-- 编辑对话框 -->
    <el-dialog v-model="showEditDialog" title="编辑项目" width="500px">
      <el-form label-width="80px">
        <el-form-item label="项目名称">
          <el-input v-model="editForm.name" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="editForm.description" type="textarea" :rows="3" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showEditDialog = false">取消</el-button>
        <el-button type="primary" @click="submitEdit" :loading="editSubmitting">保存</el-button>
      </template>
    </el-dialog>

    <!-- 添加/编辑 Repo 对话框 -->
    <el-dialog v-model="showRepoDialog" :title="editingRepoIndex >= 0 ? '编辑 Repo' : '添加 Repo'" width="550px">
      <el-form label-width="120px">
        <el-form-item label="仓库地址" required>
          <el-select v-model="repoForm.repo_addr" filterable allow-create placeholder="请选择或输入仓库地址" style="width: 100%">
            <el-option v-for="addr in allRepoAddrs" :key="addr" :label="addr" :value="addr" />
          </el-select>
        </el-form-item>
        <el-form-item label="分支" required>
          <el-select v-model="repoForm.repo_branch" filterable allow-create :disabled="!repoForm.repo_addr" placeholder="请选择或输入分支名称" style="width: 100%">
            <el-option v-for="branch in allBranches" :key="branch" :label="branch" :value="branch" />
          </el-select>
        </el-form-item>
        <el-form-item label="时间范围">
          <template v-if="repoForm.end_time_is_now">
            <div style="display:flex; align-items:center; gap:8px; width:100%">
              <el-date-picker v-model="repoForm.date_range[0]" type="date" placeholder="开始时间" style="flex:1" />
              <span style="color:#606266; white-space:nowrap">至 now</span>
            </div>
          </template>
          <template v-else>
            <el-date-picker v-model="repoForm.date_range" type="daterange" range-separator="至" start-placeholder="开始时间" end-placeholder="结束时间" style="width: 100%" />
          </template>
        </el-form-item>
        <el-form-item label="仅包含指定 Commits">
          <el-switch v-model="repoForm.whitelist_mode" />
        </el-form-item>
        <el-form-item v-if="!repoForm.whitelist_mode" label="排除 Commits">
          <el-select v-model="repoForm.exclude_commits" multiple allow-create filterable placeholder="输入要排除的 commits" style="width: 100%" />
        </el-form-item>
        <el-form-item v-else label="包含 Commits">
          <el-select v-model="repoForm.include_only_commits" multiple allow-create filterable placeholder="输入要包含的 commits" style="width: 100%" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showRepoDialog = false">取消</el-button>
        <el-button type="primary" @click="submitRepo" :loading="repoSubmitting">确认</el-button>
      </template>
    </el-dialog>

    <!-- 添加 Task 对话框 -->
    <el-dialog v-model="showTaskDialog" title="添加 Task" width="500px">
      <el-form label-width="100px">
        <el-form-item label="Task IDs" required>
          <el-select
            v-model="taskForm.task_ids"
            multiple
            filterable
            remote
            reserve-keyword
            :remote-method="searchTasks"
            :loading="taskSearchLoading"
            placeholder="搜索 task（输入用户名/work_dir/task_id）"
            style="width: 100%"
          >
            <el-option
              v-for="task in taskOptions"
              :key="task.task_id"
              :value="task.task_id"
              :label="(task.task_id || '').substring(0, 8) + (task.title ? ' | ' + task.title : '')"
            >
              <div style="display:flex; justify-content:space-between; font-size:13px;">
                <span>{{ (task.task_id || '').substring(0, 12) }}...</span>
                <span style="color:#909399; margin-left:12px;">{{ task.user_name }} | {{ task.title || task.work_dir }}</span>
              </div>
            </el-option>
          </el-select>
        </el-form-item>
        <el-form-item label="Silica 权重">
          <el-input-number v-model="taskForm.silica" :precision="2" :step="0.1" :min="0" style="width: 100%" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showTaskDialog = false">取消</el-button>
        <el-button type="primary" @click="submitTask" :loading="taskSubmitting">确认</el-button>
      </template>
    </el-dialog>

    <!-- 编辑 Silica 对话框 -->
    <el-dialog v-model="showEditSilicaDialog" title="编辑 Silica 权重" width="400px">
      <el-form label-width="100px">
        <el-form-item label="Silica 权重">
          <el-input-number v-model="silicaForm.silica" :precision="2" :step="0.1" :min="0" style="width: 100%" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showEditSilicaDialog = false">取消</el-button>
        <el-button type="primary" @click="submitSilicaEdit" :loading="silicaSubmitting">确认</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ArrowLeft, QuestionFilled, Plus } from '@element-plus/icons-vue'
import { getProjectDetail, updateProjectManual, updateProject, removeRepoFromProject, addRepoToProject, addTasksToProject, removeTasksFromProject, updateTaskSilicaInProject, getReposV2, getTasksV2, getGlobalConfig } from '@/api/es'
import { fmtCost, formatLocalTime, formatDuration } from '@/utils/formatters'
import { getEfficiencyColor } from '@/utils/commit-helpers'
import { ElMessage, ElMessageBox } from 'element-plus'

const router = useRouter()
const route = useRoute()

const loading = ref(false)
const project = ref({})
const tasks = ref([])
const commits = ref([])
const userCount = ref(0)

function fmtCostVal(val) {
  return fmtCost(null, null, val)
}

const totalTokens = computed(() => {
  return (project.value.upstream_tokens || 0) + (project.value.downstream_tokens || 0)
})

const totalCodeLines = computed(() => {
  return commits.value.reduce((sum, row) => sum + (row.diff_lines || 0), 0)
})

const actualWorkDays = computed(() => {
  const minutes = project.value.project_real_process_minutes_manual ?? project.value.project_real_process_minutes
  if (minutes == null || minutes <= 0) return null
  return minutes / 480
})

const globalConfig = ref({ traditional_dev_lines_per_day: 100 })

const traditionalDevLinesPerDay = computed(() => globalConfig.value.traditional_dev_lines_per_day || 100)

const ancientWorkDays = computed(() => {
  const minutes = project.value.project_ancient_minutes_manual ?? project.value.project_ancient_minutes
  if (minutes == null || minutes <= 0) return null
  return minutes / 480
})

const leadWorkDays = computed(() => {
  const minutes = project.value.project_real_lead_minutes_manual ?? project.value.project_real_lead_minutes
  if (minutes == null || minutes <= 0) return null
  return minutes / 480
})

const actualLinesPerDay = computed(() => {
  if (!actualWorkDays.value || actualWorkDays.value <= 0) return null
  return totalCodeLines.value / actualWorkDays.value
})

const traditionalLinesPerDay = computed(() => {
  if (!ancientWorkDays.value || ancientWorkDays.value <= 0) return null
  return totalCodeLines.value / ancientWorkDays.value
})

const devEfficiencyRatio = computed(() => {
  if (!actualWorkDays.value || actualWorkDays.value <= 0) return null
  if (!ancientWorkDays.value) return null
  return ancientWorkDays.value / actualWorkDays.value
})

const e2eEfficiencyRatio = computed(() => {
  if (!leadWorkDays.value || leadWorkDays.value <= 0) return null
  if (!ancientWorkDays.value) return null
  return ancientWorkDays.value / leadWorkDays.value
})

const userStats = computed(() => {
  const map = {}
  for (const t of tasks.value) {
    const name = t.user_name || '未知'
    if (!map[name]) map[name] = { user_name: name, task_count: 0, commit_count: 0, commit_diff_lines: 0, task_ancient_minutes: 0, task_real_minutes: 0, commit_ancient_minutes: 0, commit_real_minutes: 0, cost: 0 }
    map[name].task_count++
    map[name].task_ancient_minutes += (t.task_ancient_minutes_manual ?? t.task_ancient_minutes) || 0
    map[name].task_real_minutes += (t.task_real_minutes_manual ?? t.task_real_minutes) || 0
    map[name].cost += t.cost || 0
  }
  for (const c of commits.value) {
    const name = c.user_name || '未知'
    if (!map[name]) map[name] = { user_name: name, task_count: 0, commit_count: 0, commit_diff_lines: 0, task_ancient_minutes: 0, task_real_minutes: 0, commit_ancient_minutes: 0, commit_real_minutes: 0, cost: 0 }
    map[name].commit_count++
    map[name].commit_diff_lines += c.diff_lines || 0
    map[name].commit_ancient_minutes += (c.commit_ancient_minutes_manual ?? c.commit_ancient_minutes) || 0
    map[name].commit_real_minutes += (c.commit_real_minutes_manual ?? c.commit_real_minutes) || 0
    map[name].cost += c.cost || 0
  }
  return Object.values(map).map(u => {
    u.task_efficiency_ratio = u.task_real_minutes > 0 ? (u.task_ancient_minutes / u.task_real_minutes) * 100 : 0
    u.commit_efficiency_ratio = u.commit_real_minutes > 0 ? (u.commit_ancient_minutes / u.commit_real_minutes) * 100 : 0
    return u
  })
})

// 手动修正
const showManualDialog = ref(false)
const manualSubmitting = ref(false)
const manualForm = ref({})

watch(showManualDialog, (val) => {
  if (val) {
    const p = project.value
    manualForm.value = {
      project_ancient_minutes_manual: p.project_ancient_minutes_manual ?? p.project_ancient_minutes ?? null,
      project_ancient_minutes_reason_manual: p.project_ancient_minutes_reason_manual || '',
      project_real_process_minutes_manual: p.project_real_process_minutes_manual ?? p.project_real_process_minutes ?? null,
      project_real_process_minutes_reason_manual: p.project_real_process_minutes_reason_manual || '',
      project_real_lead_minutes_manual: p.project_real_lead_minutes_manual ?? p.project_real_lead_minutes ?? null,
      project_real_lead_minutes_reason_manual: p.project_real_lead_minutes_reason_manual || '',
      start_time_manual: p.start_time_manual || null,
      end_time_manual: p.end_time_manual || null,
    }
  }
})

async function submitManual() {
  manualSubmitting.value = true
  try {
    await updateProjectManual(route.params.projectId, manualForm.value)
    ElMessage.success('人工调整已保存')
    showManualDialog.value = false
    await loadData()
  } catch (e) {
    ElMessage.error('保存失败: ' + (e.response?.data?.error || e.message))
  } finally {
    manualSubmitting.value = false
  }
}

// 编辑对话框
const showEditDialog = ref(false)
const editSubmitting = ref(false)
const editForm = ref({ name: '', description: '' })

watch(showEditDialog, (val) => {
  if (val) {
    editForm.value = {
      name: project.value.name || '',
      description: project.value.description || '',
    }
  }
})

async function submitEdit() {
  if (!editForm.value.name.trim()) {
    ElMessage.warning('项目名称不能为空')
    return
  }
  editSubmitting.value = true
  try {
    await updateProject(route.params.projectId, {
      name: editForm.value.name.trim(),
      description: editForm.value.description.trim(),
      repos: project.value.repos,
      task_ids: project.value.task_ids,
      task_ids_silica: project.value.task_ids_silica,
    })
    ElMessage.success('编辑已保存')
    showEditDialog.value = false
    await loadData()
  } catch (e) {
    ElMessage.error('保存失败: ' + (e.response?.data?.error || e.message))
  } finally {
    editSubmitting.value = false
  }
}

// Repo 对话框
const showRepoDialog = ref(false)
const repoSubmitting = ref(false)
const editingRepoIndex = ref(-1)
const repoForm = ref({
  repo_addr: '',
  repo_branch: '',
  date_range: [],
  end_time_is_now: false,
  whitelist_mode: false,
  exclude_commits: [],
  include_only_commits: []
})
const allRepos = ref([])
const allBranches = ref([])

async function loadAllRepos() {
  try {
    const result = await getReposV2({ pageSize: 1000 })
    allRepos.value = result.data?.data || result.data || []
    const addr = repoForm.value.repo_addr
    if (addr) {
      allBranches.value = allRepos.value
        .filter(r => r.repo_addr === addr)
        .map(r => r.repo_branch)
        .filter(Boolean)
    }
  } catch (e) {
    allRepos.value = []
  }
}

const allRepoAddrs = computed(() => {
  const addrs = []
  const seen = new Set()
  for (const r of allRepos.value) {
    const addr = r.repo_addr
    if (addr && !seen.has(addr)) {
      seen.add(addr)
      addrs.push(addr)
    }
  }
  return addrs
})

watch(() => repoForm.value.repo_addr, (addr, oldAddr) => {
  if (addr) {
    allBranches.value = allRepos.value
      .filter(r => r.repo_addr === addr)
      .map(r => r.repo_branch)
      .filter(Boolean)
  } else {
    allBranches.value = []
  }
  if (oldAddr && addr !== oldAddr) {
    repoForm.value.repo_branch = ''
  }
})

async function openAddRepoDialog() {
  editingRepoIndex.value = -1
  const startTime = project.value.start_time_manual || project.value.start_time || null
  const endTime = project.value.end_time_manual || project.value.end_time || null
  repoForm.value = {
    repo_addr: '',
    repo_branch: '',
    date_range: startTime && endTime ? [startTime, endTime] : (startTime ? [startTime] : []),
    end_time_is_now: !endTime,
    whitelist_mode: false,
    exclude_commits: [],
    include_only_commits: []
  }
  await loadAllRepos()
  showRepoDialog.value = true
}

async function openEditRepoDialog(row, index) {
  editingRepoIndex.value = index
  repoForm.value = {
    repo_addr: row.repo_addr || '',
    repo_branch: row.repo_branch || '',
    date_range: row.start_time && row.end_time ? [row.start_time, row.end_time] : [],
    whitelist_mode: row.include_only_commits && row.include_only_commits.length > 0,
    exclude_commits: row.exclude_commits || [],
    include_only_commits: row.include_only_commits || []
  }
  await loadAllRepos()
  showRepoDialog.value = true
}

async function submitRepo() {
  if (!repoForm.value.repo_addr.trim()) {
    ElMessage.warning('仓库地址不能为空')
    return
  }
  if (!repoForm.value.repo_branch.trim()) {
    ElMessage.warning('分支不能为空')
    return
  }

  repoSubmitting.value = true
  try {
    const data = {
      repo_addr: repoForm.value.repo_addr.trim(),
      repo_branch: repoForm.value.repo_branch.trim(),
      start_time: repoForm.value.date_range && repoForm.value.date_range[0] ? repoForm.value.date_range[0] : null,
      end_time: repoForm.value.end_time_is_now ? null : (repoForm.value.date_range && repoForm.value.date_range[1] ? repoForm.value.date_range[1] : null),
      exclude_commits: repoForm.value.whitelist_mode ? [] : repoForm.value.exclude_commits,
      include_only_commits: repoForm.value.whitelist_mode ? repoForm.value.include_only_commits : []
    }

    const projectId = route.params.projectId

    // 如果是编辑模式，先删除旧的
    if (editingRepoIndex.value >= 0) {
      await removeRepoFromProject(projectId, editingRepoIndex.value)
    }

    await addRepoToProject(projectId, data)
    ElMessage.success(editingRepoIndex.value >= 0 ? '编辑成功' : '添加成功')
    showRepoDialog.value = false
    await loadData()
  } catch (e) {
    ElMessage.error((editingRepoIndex.value >= 0 ? '编辑' : '添加') + '失败: ' + (e.response?.data?.error || e.message || e))
  } finally {
    repoSubmitting.value = false
  }
}

// Task 对话框
const showTaskDialog = ref(false)
const taskSubmitting = ref(false)
const taskForm = ref({
  task_ids: [],
  silica: 1.0
})
const taskOptions = ref([])
const taskSearchLoading = ref(false)

async function searchTasks(query) {
  taskSearchLoading.value = true
  try {
    const today = new Date()
    const endDate = today.toISOString().slice(0, 10).replace(/-/g, '')
    const startDate = '20250101'
    const result = await getTasksV2({ pageSize: 50, startDate, endDate })
    const data = result.data?.data || result.data || []
    if (query && query.trim()) {
      const q = query.trim().toLowerCase()
      taskOptions.value = data.filter(t =>
        (t.task_id || '').toLowerCase().includes(q) ||
        (t.user_name || '').toLowerCase().includes(q) ||
        (t.work_dir || '').toLowerCase().includes(q) ||
        (t.title || '').toLowerCase().includes(q)
      )
    } else {
      taskOptions.value = data
    }
  } catch (e) {
    taskOptions.value = []
  } finally {
    taskSearchLoading.value = false
  }
}

async function openAddTaskDialog() {
  taskForm.value = {
    task_ids: [],
    silica: 1.0
  }
  showTaskDialog.value = true
  await searchTasks('')
}

async function submitTask() {
  if (!taskForm.value.task_ids || taskForm.value.task_ids.length === 0) {
    ElMessage.warning('请至少输入一个 Task ID')
    return
  }

  taskSubmitting.value = true
  try {
    const taskIds = taskForm.value.task_ids.map(id => id.trim()).filter(id => id)
    const data = {
      task_ids: taskIds,
      task_ids_silica: taskIds.map(() => taskForm.value.silica)
    }
    await addTasksToProject(route.params.projectId, data)
    ElMessage.success('添加成功')
    showTaskDialog.value = false
    await loadData()
  } catch (e) {
    ElMessage.error('添加失败: ' + (e.response?.data?.error || e.message || e))
  } finally {
    taskSubmitting.value = false
  }
}

async function handleRemoveTask(row) {
  try {
    await ElMessageBox.confirm('确定要删除此 Task 吗？', '确认删除', { type: 'warning' })
    await removeTasksFromProject(route.params.projectId, { task_ids: [row.task_id] })
    ElMessage.success('删除成功')
    await loadData()
  } catch (e) {
    if (e !== 'cancel') ElMessage.error('删除失败: ' + (e.response?.data?.error || e.message || e))
  }
}

// Edit Silica 对话框
const showEditSilicaDialog = ref(false)
const silicaSubmitting = ref(false)
const silicaForm = ref({
  task_id: '',
  silica: 1.0
})

function openEditSilicaDialog(row) {
  silicaForm.value = {
    task_id: row.task_id,
    silica: row.silica ?? 1.0
  }
  showEditSilicaDialog.value = true
}

async function submitSilicaEdit() {
  silicaSubmitting.value = true
  try {
    await updateTaskSilicaInProject(route.params.projectId, {
      task_id: silicaForm.value.task_id,
      silica: silicaForm.value.silica
    })
    ElMessage.success('修改成功')
    showEditSilicaDialog.value = false
    await loadData()
  } catch (e) {
    ElMessage.error('修改失败: ' + (e.response?.data?.error || e.message || e))
  } finally {
    silicaSubmitting.value = false
  }
}

// 删除 repo
async function handleRemoveRepo(index) {
  try {
    await ElMessageBox.confirm('确定要删除此 Repo 配置吗？', '确认删除', { type: 'warning' })
    await removeRepoFromProject(route.params.projectId, index)
    ElMessage.success('删除成功')
    await loadData()
  } catch (e) {
    if (e !== 'cancel') ElMessage.error('删除失败: ' + (e.response?.data?.error || e.message || e))
  }
}

// 数据加载
async function loadData() {
  const projectId = route.params.projectId
  if (!projectId) return
  loading.value = true
  try {
    const result = await getProjectDetail(projectId)
    const data = result.data || result
    project.value = data.project || data || {}
    tasks.value = data.tasks || []
    commits.value = data.commits || []
    userCount.value = data.user_count || 0
  } catch (e) {
    project.value = {}
    tasks.value = []
    commits.value = []
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadData()
  getGlobalConfig().then(res => {
    globalConfig.value = res.data || res
  }).catch(() => {})
})
</script>

<style>
.reason-tooltip {
  max-width: 400px !important;
}
</style>
