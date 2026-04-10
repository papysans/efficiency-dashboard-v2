## 实施

- [x] 1.1 提取公共 CSS 到全局样式文件
     【目标对象】`frontend/src/style.css`
     【修改目的】消除 5-6 个组件中重复的 scoped CSS（filter-card/filter-row/table-card/pagination-wrapper/clickable-row/virtual-group-row/chart-container/metric-card/metric-label/metric-value），统一各面板视觉风格
     【修改方式】在 style.css 末尾新增公共样式类定义
     【相关依赖】无，此任务为后续 1.7/1.8 任务的前置依赖
     【修改内容】
        - 添加 `.kb-panel` 基础面板样式（padding: 16px; display: flex; flex-direction: column; gap: 16px），对标现有各面板根 div 样式
        - 添加 `.kb-filter-card :deep(.el-card__body)` 筛选卡片样式（padding: 16px）
        - 添加 `.kb-filter-row` 筛选行样式（display: flex; align-items: center; flex-wrap: wrap; gap: 8px）
        - 添加 `.kb-table-card :deep(.el-card__body)` 表格卡片样式（padding: 16px）
        - 添加 `.kb-pagination` 分页容器样式（display: flex; justify-content: flex-end; margin-top: 16px）
        - 添加 `:deep(.kb-clickable-row)` 可点击行样式（cursor: pointer）
        - 添加 `:deep(.kb-vgroup-row)` 虚拟组行高亮样式（background-color: #fdf6ec !important）
        - 添加 `.kb-chart-container` 图表容器样式（width: 100%; height: 350px），注意 EfficiencyPanel 饼图高度为 300px 需保留局部覆盖
        - 添加 `.kb-metric-card :deep(.el-card__body)` 指标卡片样式（padding: 16px）
        - 添加 `.kb-metric-label` 指标标签样式（font-size: 13px; color: #909399）
        - 添加 `.kb-metric-value` 指标数值样式（font-size: 24px; font-weight: bold; margin-top: 8px; color: #303133）
        - 添加 `.kb-charts-area` 图表区域布局样式（display: flex; flex-direction: column）
        - 所有样式值必须从现有组件 scoped style 中提取，不发明新样式

- [x] 1.2 美化 Home 页面
     【目标对象】`frontend/src/views/Home.vue`
     【修改目的】为卡片添加差异化颜色，提升视觉层次，增强可区分度
     【修改方式】修改 `<template>` 中6个 `el-card` 组件的样式绑定，修改 `<style scoped>` 中的 `.nav-card` 样式
     【相关依赖】无
     【修改内容】
        - 注意：现有代码 `.nav-card` 已有 `margin-bottom: 20px`（第88行），无需重复添加行间距
        - 为6张卡片分别设置不同的左边框色（通过 style 绑定 `border-left: 3px solid <color>`），每个面板使用不同主题色以增强可区分度
        - 可选：适当增大 `.card-header` 图标尺寸（从 24 调整到 28）和字体大小
        - 保留现有的 hover 动画效果不变

- [x] 1.3 EfficiencyPanel 添加代码行数指标和 URL 参数恢复
     【目标对象】`frontend/src/views/EfficiencyPanel.vue`
     【修改目的】1) 补全代码行数指标到指标卡片区；2) 支持从其他面板跳转后自动恢复参数并查询，无需手动操作
     【修改方式】修改 `<template>` 第40-92行指标卡片区（新增一个 el-col 指标卡片）；修改 `<script setup>` 中第372-377行 `onMounted` 钩子（添加 URL 参数恢复逻辑）；将第211-218行饼图手动管理替换为 useChart composable
     【相关依赖】`frontend/src/composables/useChart.js` 的 `useChart()`；`frontend/src/composables/useUrlSync.js`（参考其 restoreFromUrl 模式，但 EfficiencyPanel 有 dimension+id 的特殊参数，需自行实现恢复逻辑）
     【修改内容】
        - 在第二行指标卡片区（第93-133行 el-row 内）添加一个"产出代码行数"指标卡片，数据来源：`effData.value?.actual_time?.total_code_lines`，使用现有 fmtDays 类似的格式化方式，无数据时显示 '-'
        - 在 `onMounted` 中读取 `route.query` 的 `dimension`、`id`、`startDate`、`endDate` 参数
        - 如果 URL 中 `id` 参数存在，自动设置 `dimension`（ref 变量名为 `dimension`）、`dimensionId`（ref 变量名为 `dimensionId`）、`dateRange` 并调用 `fetchData()`
        - 注意边界：若 URL 只有部分参数（如只有 id 没有日期），则不自动查询，仅恢复已有参数
        - 将饼图管理从手动 `echarts.init`/`dispose`/`resize`（第211-218行、第291-296行、第368-370行、第379-383行）替换为 `useChart(pieChartRef)` composable，参考 ProjectPanel.vue 第170-174行的用法
        - 替换后移除 `import * as echarts from 'echarts'`（第187行）、手动 resize 监听（第368-370行、第376-377行）和 `onUnmounted` 中的 dispose 逻辑（第379-383行）

- [x] 1.4 所有面板表格添加 sortable
     【目标对象】`frontend/src/views/Dashboard.vue`、`frontend/src/views/ProjectPanel.vue`、`frontend/src/views/RepoPanel.vue`、`frontend/src/views/UserPanel.vue`、`frontend/src/views/OrgPanel.vue`、`frontend/src/views/EfficiencyPanel.vue`
     【修改目的】所有表格均无排序功能，用户无法按指标排序查看
     【修改方式】为各文件中 `el-table-column` 的数值型列添加 `sortable` 属性
     【相关依赖】无
     【修改内容】
        - Dashboard（第58-90行）：aggregate 模式下的数值列添加 `sortable`：user_in_chars、code_lines、api_count、api_cost、api_in_tokens、api_out_tokens、task_count、ai_estimated_days、lead_time、process_time
        - ProjectPanel（第47-73行）：数值列添加 `sortable`：task_count、api_count、code_lines、api_cost、ai_estimated_days、process_time
        - RepoPanel（第42-77行）：仓库列表表格数值列添加 `sortable`：task_count、api_count、code_lines、api_cost、ai_estimated_days、process_time；贡献者表格（第124-129行）数值列添加 `sortable`：commits、lines_added、lines_deleted
        - UserPanel（第39-72行）：数值列添加 `sortable`：task_count、code_lines、api_cost、ai_estimated_days、process_time（注意：api_count 列在任务 1.5 中添加时一并加 sortable）
        - OrgPanel（第49-83行）：数值列添加 `sortable`：task_count、api_count、code_lines、api_cost、ai_estimated_days、process_time
        - EfficiencyPanel（第138-145行）：用户参与详情表数值列添加 `sortable`：lead_time_days、process_time_days、percentage；代码来源统计表（第153-158行）数值列添加 `sortable`：commit_count、code_lines、estimated_days、percentage
        - 排序策略：所有面板均使用服务端分页（API 传递 page/pageSize），但当前页数据量有限，使用 Element Plus 内置 `sortable`（前端排序当前页数据）即可满足需求；不使用 `sortable="custom"`

- [x] 1.5 UserPanel 补充 API 次数列和扩展图表
     【目标对象】`frontend/src/views/UserPanel.vue`
     【修改目的】UserPanel 比 ProjectPanel 少了 api_count 列、Token 图表、API 次数图表、处理时长图表，需对齐
     【修改方式】修改 `<template>` 第67-71行表格列区（新增 api_count 列）；修改第88-101行图表区（新增3个图表卡片）；修改 `<script setup>` 中 `updateCharts` 函数（第155-164行）添加新图表数据绑定；新增3个 useChart 实例和对应的 DOM ref
     【相关依赖】`frontend/src/utils/chart.js` 的 `createBarOption` 和 `createDualBarOption`；`frontend/src/composables/useChart.js` 的 `useChart()`
     【修改内容】
        - 在表格第67行 task_count 列之后添加 `<el-table-column prop="api_count" label="API次数" width="100" align="right" sortable />`
        - 在 import 中添加 `createDualBarOption`（当前只导入了 `createBarOption`，参考 ProjectPanel 第133行）
        - 新增3个 DOM ref：`chartTokensRef`、`chartApiCountRef`、`chartProcessTimeRef`
        - 新增3个 useChart 实例：`setTokensOption`、`setApiCountOption`、`setProcessTimeOption`（参考 ProjectPanel 第170-174行）
        - 图表区扩展为与 ProjectPanel 一致的5图表布局：第一行代码行数+API费用，第二行Token使用量+API调用次数，第三行处理时长（全宽）
        - 在 `updateCharts` 函数中添加3组数据映射和 setOption 调用，参考 ProjectPanel 第192-200行的实现模式

- [x] 1.6 OrgPanel 扩展图表
     【目标对象】`frontend/src/views/OrgPanel.vue`
     【修改目的】OrgPanel 只有2个图表（代码行数、API费用），比 ProjectPanel 少了 Token/API次数/处理时长图表，需对齐
     【修改方式】修改 `<template>` 第99-112行图表区（新增2个 el-row 包含3个图表卡片）；修改 `<script setup>` 中新增3个 DOM ref 和 useChart 实例；修改 `updateCharts` 函数（第173-182行）添加新图表数据绑定
     【相关依赖】`frontend/src/utils/chart.js` 的 `createBarOption` 和 `createDualBarOption`（当前只导入了 `createBarOption`，需补充导入 `createDualBarOption`）；`frontend/src/composables/useChart.js` 的 `useChart()`
     【修改内容】
        - 在 import 中补充 `createDualBarOption`（当前第125行只导入了 `createBarOption`）
        - 新增3个 DOM ref：`chartTokensRef`、`chartApiCountRef`、`chartProcessTimeRef`
        - 新增3个 useChart 实例：`setTokensOption`、`setApiCountOption`、`setProcessTimeOption`
        - 图表区扩展为与 ProjectPanel 一致的5图表布局：第一行代码行数+API费用（已有），第二行Token使用量+API调用次数（新增），第三行处理时长（新增，全宽）
        - 在 `updateCharts` 函数中添加3组数据映射和 setOption 调用，参考 ProjectPanel 第192-200行的实现模式（标题改为"按组织"）

- [x] 1.7 RepoPanel 使用公共 CSS 和 useChart
     【目标对象】`frontend/src/views/RepoPanel.vue`
     【修改目的】RepoPanel 的饼图手动管理 echarts 实例（init/dispose/resize），未使用项目已有的 useChart composable；scoped CSS 与其他面板大量重复
     【修改方式】修改 `<script setup>` 中第216-217行饼图变量、第332-352行饼图函数、第386-390行 onUnmounted、第371-374行 resize 监听，替换为 useChart composable；修改 `<template>` 和 `<style scoped>` 中的 class 名称为公共 CSS 类
     【相关依赖】`frontend/src/composables/useChart.js` 的 `useChart()`；任务 1.1 定义的公共 CSS 类
     【修改内容】
        - 移除 `import * as echarts from 'echarts'`（第171行），添加 `import { useChart } from '@/composables/useChart'`
        - 将 `let pieChart = null`（第217行）替换为 `const { setOption: setPieOption } = useChart(pieChartRef)`
        - 将 `initPieChart()` 和 `updatePieChart()` 合并简化：直接调用 `setPieOption(option)` 即可，useChart 内部会自动 init
        - 移除 `onUnmounted` 中的手动 dispose（第386-390行）和 `handleResize` 函数（第372-374行）及其事件监听（第381行、第389行）
        - 将 template 中 `class="repo-panel"` 改为 `class="kb-panel"`，`class="filter-card"` 改为 `class="kb-filter-card"`，以此类推
        - 删除 scoped style 中与公共样式重复的定义（.repo-panel、.filter-card、.filter-row、.table-card、.clickable-row、.virtual-group-row、.pagination-wrapper、.metric-card、.metric-label、.metric-value、.chart-container）
        - 保留 RepoPanel 特有的 scoped 样式（如果有）

- [x] 1.8 各面板使用公共 CSS 类
     【目标对象】`frontend/src/views/Dashboard.vue`、`frontend/src/views/ProjectPanel.vue`、`frontend/src/views/UserPanel.vue`、`frontend/src/views/OrgPanel.vue`、`frontend/src/views/EfficiencyPanel.vue`
     【修改目的】各面板 scoped 样式与全局公共样式重复，需统一使用 1.1 定义的公共 CSS 类以消除重复
     【修改方式】在各文件的 `<template>` 中将现有 class 名称替换为公共 CSS 类名，在 `<style scoped>` 中删除与公共样式重复的定义
     【相关依赖】任务 1.1 定义的公共 CSS 类
     【修改内容】
        - Dashboard.vue：将 `class="dashboard"` 替换为 `class="kb-panel"`（注意 Dashboard 有额外的 `height: 100%` 需保留为局部样式）；将 `class="filter-card"` 替换为 `class="kb-filter-card"`；将 `class="filter-row"` 替换为 `class="kb-filter-row"`；将 `class="table-card"` 替换为 `class="kb-table-card"`；将 `class="pagination-wrapper"` 替换为 `class="kb-pagination"`；删除 scoped 中对应的重复定义；保留 Dashboard 特有样式（.index-tabs、.table-card flex 布局等）
        - ProjectPanel.vue：同理替换 class 名称（project-panel→kb-panel、filter-card→kb-filter-card、filter-row→kb-filter-row、table-card→kb-table-card、pagination-wrapper→kb-pagination、clickable-row→kb-clickable-row、virtual-group-row→kb-vgroup-row、charts-area→kb-charts-area、chart-container→kb-chart-container）；删除重复 scoped 定义
        - UserPanel.vue：同理替换（user-panel→kb-panel 等）；删除重复 scoped 定义
        - OrgPanel.vue：同理替换（org-panel→kb-panel 等）；保留 OrgPanel 特有的面包屑样式（.breadcrumb-card、.breadcrumb-link）
        - EfficiencyPanel.vue：同理替换（efficiency-panel→kb-panel 等）；保留 EfficiencyPanel 特有样式（.metric-header、.metric-edit、.reason-section、.reason-list、.reason-text 及提效比率颜色逻辑）
        - 注意：EfficiencyPanel 的 gap 为 12px（其余面板为 16px），需保留局部覆盖或在 kb-panel 基础上追加 class
