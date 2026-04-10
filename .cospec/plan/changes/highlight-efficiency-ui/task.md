## 实施

### 阶段 1：后端支持

- [x] 1.1 Dashboard API 返回提效聚合数据
     【目标对象】`backend/dashboard_handler_v2.go`
     【修改目的】首页需要展示总实际耗时、平均提效比，当前 dashboard API 不返回这些字段
     【修改方式】修改 `getDashboardSummary` 函数中 SQL 1（taskQuery）的聚合查询及返回 JSON
     【相关依赖】无新增依赖，复用已有 `db` 和现有查询结构
     【修改内容】
        - 在 taskQuery 的 SELECT 中新增两个聚合字段：
          - `COALESCE(SUM(COALESCE(task_real_minutes_manual, task_real_minutes)), 0) as total_real_minutes`（优先取 manual 值，与详情页一致）
          - `AVG(CASE WHEN COALESCE(task_real_minutes_manual, task_real_minutes) > 0 AND COALESCE(task_ancient_minutes_manual, task_ancient_minutes) > 0 THEN COALESCE(task_ancient_minutes_manual, task_ancient_minutes) / COALESCE(task_real_minutes_manual, task_real_minutes) * 100 END) as avg_efficiency_ratio`
        - 新增两个 Go 变量（`var totalRealMinutes, avgEfficiencyRatio float64`）并加入 `Scan` 参数列表
        - 在返回 JSON（`c.JSON(http.StatusOK, gin.H{...})`）中新增 `"total_real_minutes": totalRealMinutes` 和 `"avg_efficiency_ratio": avgEfficiencyRatio`
        - 注意 `avg_efficiency_ratio` 可能为 NULL（所有 task 均无有效数据时），Scan 需用 `*float64` 或 `sql.NullFloat64` 处理
        - 编译验证

- [x] 1.2 Task 列表 API 返回 efficiency_ratio
     【目标对象】`backend/task_handler_v2.go`
     【修改目的】Task 列表页需要直接展示每个 task 的提效比，当前列表 API 不返回 efficiency_ratio
     【修改方式】修改 `listTasksV2` 函数，在返回列表前为每个 task 计算 efficiency_ratio
     【相关依赖】复用 `getTaskDetailV2` 中已有的 efficiency_ratio 计算逻辑（优先取 manual 值）
     【修改内容】
        - 在 `listTasksV2` 取到 `list` 后、返回 JSON 前，遍历 list 为每条记录计算 efficiency_ratio
        - 计算逻辑同 `getTaskDetailV2`：effectiveAncient = COALESCE(manual, original)，effectiveReal = COALESCE(manual, original)，ratio = ancient/real * 100
        - 由于 `CostrictTask` 结构体无 efficiency_ratio 字段，有两种方案：
          - 方案A：将 list 转为 `[]gin.H`，每条记录追加 `"efficiency_ratio"` 字段
          - 方案B：新增一个包含 efficiency_ratio 的响应结构体
        - 选择与仓库现有风格一致的方案（参考 `getTaskDetailV2` 的做法，倾向方案A）
        - 当 ancient 或 real 为 0 或 nil 时，efficiency_ratio 返回 null

### 阶段 2：首页改进

- [x] 2.1 首页提效指标突出
     【目标对象】`frontend/src/views/Home.vue`
     【修改目的】让提效成为首页核心视觉焦点，用提效指标替代非核心的 Token/代码行数指标
     【修改方式】修改 `<template>` 中第二行 `el-row`（"指标卡片区 - 第二行"注释处）的 4 个 `el-col` 内容；在第二行后新增第三行 `el-row`；修改快速导航区的 `el-row` 布局
     【相关依赖】任务 1.1 返回的 `total_real_minutes`、`avg_efficiency_ratio`；已有的 `formatDuration` 工具函数（已 import）
     【修改内容】
        - 第二行 4 个指标卡片调整为：
          - 卡片1：总费用（保持不变）
          - 卡片2：**总实际耗时**（替换"总Token数"），值 = `formatDurationVal(summary.total_real_minutes)`，图标 `Timer`，颜色 `#1ABC9C`
          - 卡片3：**总节省时间**（替换"总代码行数"），值 = `formatDurationVal((summary.total_task_ancient_minutes || 0) - (summary.total_real_minutes || 0))`，图标 `TrendCharts`（或复用 `EditPen`），颜色翠绿 `#1ABC9C`
          - 卡片4：古法预估（保持不变）
        - 新增第三行：**平均提效比**横幅卡片
          - `el-row` + 单个 `el-col :span="24"`
          - 卡片样式：绿色背景渐变（`background: linear-gradient(135deg, #67C23A, #1ABC9C)`），白色大字体（36px），居中显示如"平均提效比 320%"
          - 值 = `summary.avg_efficiency_ratio`，格式化为保留一位小数 + '%'；无数据时显示'-'
        - 快速导航区：
          - 将 `el-col :span="8"` 改为 `:span="6"`（4列布局）
          - 新增第四个导航卡片："提交视图"，图标 `Connection`，颜色 `#F56C6C`，跳转 `/commit-v2`
        - 节省时间需处理负值边界：若 ancient < real，显示为 0 或加特殊标记
        - **UI 突出设计要点**：平均提效比横幅是整个首页最核心的视觉元素，必须确保它在视觉层次上高于所有其他卡片——通过渐变背景色、大字体、full-width 横幅实现"一眼看到提效成果"的效果

### 阶段 3：列表页改进

- [x] 3.1 Task 列表页新增提效列 + 图表改进
     【目标对象】`frontend/src/views/TaskViewV2.vue`
     【修改目的】列表一览中直接看到实际耗时和提效比，图表聚焦提效对比
     【修改方式】在 `<el-table>` 中"古法预估"列后新增两列；替换 `updateOverviewCharts` 函数中的图表配置；新增 `import { createDualBarOption }` 导入
     【相关依赖】任务 1.2 返回的 `efficiency_ratio` 字段；`@/utils/chart` 中已有的 `createBarOption` 和 `createDualBarOption`；`@/utils/formatters` 中已有的 `formatDuration`
     【修改内容】
        - 表格新增列1："实际耗时"
          - prop=`task_real_minutes`，width=100，align=right，sortable，`:formatter="fmtDuration"`
        - 表格新增列2："提效比"
          - 使用 `<template #default="{ row }">` 自定义渲染
          - 从后端返回的 `row.efficiency_ratio` 取值（已由任务 1.2 提供）
          - 显示为 `el-tag`：≥300 → type="success"（绿色）；≥150 → type="primary"（蓝色）；<150 → type="info"（灰色）
          - 值为 null/undefined 时显示 `el-tag type="info"` 内容为"-"
          - 文字格式：保留一位小数 + '%'，如"320.0%"
        - **UI 突出设计要点**：提效比列应在表格中最醒目——el-tag 彩色徽章与其他纯文本列形成强对比，使用户扫视表格时首先注意到提效数据
        - 图表改造（替换 `updateOverviewCharts` 函数体）：
          - 左图（复用 `costChartRef` ref 变量和 `setCostOption`）：调用 `createBarOption` 生成"提效比分布（按Task）"，数据 = `data.map(t => ({ name: t.task_id, value: t.efficiency_ratio || 0 }))`，颜色 `#67C23A`，按提效比从大到小排列
          - 右图（复用 `diffChartRef` ref 变量和 `setDiffOption`）：调用 `createDualBarOption` 生成"古法预估 vs 实际耗时"，data1 = ancient_minutes 数组，data2 = real_minutes 数组，label1="古法预估"，label2="实际耗时"，color1=`#E6A23C`（橙），color2=`#409EFF`（蓝）
        - import 行新增 `createDualBarOption`：`import { createBarOption, createDualBarOption } from '@/utils/chart'`

- [x] 3.2 Commit 列表页新增提效列 + 汇总卡片 + 图表改进
     【目标对象】`frontend/src/views/CommitViewV2.vue`
     【修改目的】列表页提供提效概览，新增提效比列和汇总指标
     【修改方式】在 `<el-table>` 的"实际耗时"列后新增"提效比"列；在 FilterBar 和 `el-card.kb-table-card` 之间新增汇总卡片 `el-row`；替换 `updateOverviewCharts` 函数体；新增 computed 属性
     【相关依赖】`@/utils/chart` 中已有的 `createBarOption` 和 `createDualBarOption`；`@/utils/formatters` 中已有的 `formatDuration`
     【修改内容】
        - 表格新增"提效比"列（在"实际耗时"列后）：
          - 使用 `<template #default="{ row }">` 自定义渲染
          - 前端计算：`commit_ancient_minutes > 0 && commit_real_minutes > 0` 时，`ratio = (commit_ancient_minutes / commit_real_minutes) * 100`
          - el-tag 颜色逻辑同任务 3.1（≥300 success、≥150 primary、<150 info）
          - 值为 null 或分母为 0 时显示"-"
        - FilterBar 和表格之间新增 4 个汇总卡片（`el-row :gutter="16"` + 4 个 `el-col :span="6"`）：
          - 卡片1：总Commit数 = `filteredCommits.length`
          - 卡片2：总古法预估 = 新增 `computed` 属性 `totalAncientMinutes`（对 `filteredCommits` 求和 `commit_ancient_minutes`），用 `formatDuration` 显示
          - 卡片3：总实际耗时 = 新增 `computed` 属性 `totalRealMinutes`（对 `filteredCommits` 求和 `commit_real_minutes`），用 `formatDuration` 显示
          - 卡片4：平均提效比 = 新增 `computed` 属性 `avgEfficiencyRatio`（= totalAncientMinutes / totalRealMinutes * 100），用百分比显示 + 动态颜色（同 el-tag 颜色规则）
          - 卡片样式复用 `dashboard-metric-card` 风格（border-left 色带 + hover 上浮），需在 `<style scoped>` 中添加或复用 Home.vue 的样式
        - 图表改造（替换 `updateOverviewCharts` 函数体）：
          - 左图（复用 `diffChartRef`/`setDiffOption`）：调用 `createBarOption` 生成"提效比分布（按Commit）"，数据 = 前端计算每个 commit 的 ratio，按提效比从大到小排列
          - 右图（复用 `ancientChartRef`/`setAncientOption`）：调用 `createDualBarOption` 生成"古法预估 vs 实际耗时"，data1=ancient，data2=real，色橙+蓝
        - import 行新增 `createDualBarOption`：`import { createBarOption, createDualBarOption } from '@/utils/chart'`
        - **UI 突出设计要点**：汇总卡片中"平均提效比"使用与首页一致的醒目配色（绿色系），在 4 张卡片中视觉权重最高；提效比列的 el-tag 彩色徽章同 Task 列表

### 阶段 4：详情页改进

- [x] 4.1 Task 详情页提效卡片替换
     【目标对象】`frontend/src/views/TaskDetailV2.vue`
     【修改目的】统计摘要区域突出提效数据（古法预估/实际耗时/提效比），替代现有的请求数/Tokens/费用
     【修改方式】修改 `<template>` 中"统计摘要"注释处的 `el-row`（第36~55行）的 3 个 `el-col` 内容
     【相关依赖】已有的 `task.efficiency_ratio`（由 `loadData` 从后端 `getTaskDetailV2` 获取并合并）；`formatDuration` 已 import
     【修改内容】
        - 原3卡片"总请求数/总Tokens/总费用"替换为：
          - 卡片1：**古法预估**
            - label="古法预估"，值 = `formatDuration(task.task_ancient_minutes)`
            - 样式：`border-left: 4px solid #E6A23C`（橙色）
          - 卡片2：**实际耗时**
            - label="实际耗时"，值 = `formatDuration(task.task_real_minutes)`
            - 样式：`border-left: 4px solid #409EFF`（蓝色）
          - 卡片3：**提效比**
            - label="提效比"，值 = `task.efficiency_ratio != null ? task.efficiency_ratio.toFixed(1) + '%' : '-'`
            - 字体更大（28px），动态颜色：≥300 绿色 `#67C23A`、≥150 蓝色 `#409EFF`、其他灰色 `#909399`
            - 样式：`border-left: 4px solid` + 动态颜色（同文字颜色）
        - 移除 `v-if="conversations.length > 0"` 条件（提效卡片不依赖对话历史，即使无对话也应显示 task 级数据）
        - 可选：在 3 个大卡片下方新增一行小字辅助信息显示原有的"总请求数 | 总Tokens | 总费用"，但 proposal 未要求保留，故不实现

- [x] 4.2 Commit 详情页提效卡片新增
     【目标对象】`frontend/src/views/CommitDetailV2.vue`
     【修改目的】详情页突出提效数据，在元信息和关联Tasks之间新增醒目的提效指标
     【修改方式】在 `<template>` 中元信息 `el-card`（el-descriptions）之后、关联 Tasks `el-card` 之前，新增一个 `el-row` 包含 3 个 `el-col`
     【相关依赖】已有的 `commit.efficiency_ratio`（由 `loadData` 从后端 `getCommitDetailV2` 获取并合并）；`formatDuration` 已 import
     【修改内容】
        - 新增 `el-row :gutter="12"` 包含 3 个 `el-col :span="8"`：
          - 卡片1：**古法预估**
            - label="古法预估"，值 = `formatDuration(commit.commit_ancient_minutes)`
            - 样式：`border-left: 4px solid #E6A23C`（橙色），`dashboard-metric-card` 风格（hover 上浮）
          - 卡片2：**实际耗时**
            - label="实际耗时"，值 = `formatDuration(commit.commit_real_minutes)`
            - 样式：`border-left: 4px solid #409EFF`（蓝色）
          - 卡片3：**提效比**
            - label="提效比"，值 = `commit.efficiency_ratio != null ? commit.efficiency_ratio.toFixed(1) + '%' : '-'`
            - 动态颜色：≥300 绿色 `#67C23A`、≥150 蓝色 `#409EFF`、其他灰色 `#909399`
            - 字体更大（28px）
        - 在 `<style scoped>` 中新增卡片样式（`dashboard-metric-card` 风格：border-left 色带 + hover translateY(-4px)），与 Home.vue 一致
        - 不替代 `<el-empty description="暂无关联 Task" />`，该空状态提示保持不变

### 阶段 5：Playwright 端到端测试

- [ ] 5.1 编写提效信息突出展示验证测试
     【目标对象】`frontend/e2e/highlight-efficiency.spec.js`（新建文件）
     【修改目的】自动化验证全站提效信息是否按预期醒目展示，确保改动不遗漏、不回归
     【修改方式】新建 Playwright 测试文件，参照 `e2e/final-v2.spec.js` 的风格编写测试用例
     【相关依赖】所有前端任务（2.1/3.1/3.2/4.1/4.2）完成后执行；`playwright.config.js` 已配置 `baseURL: http://localhost:8880`
     【修改内容】
        - 测试用例1：**首页提效横幅可见且有数据**
          - 访问 `/`，等待 networkidle
          - 断言第三行平均提效比横幅存在（通过文本"平均提效比"定位或通过渐变背景色 CSS 定位）
          - 断言横幅内数值非'-'且包含'%'
          - 断言"总实际耗时"和"总节省时间"卡片存在且有数据
          - 断言"提交视图"导航卡片存在
          - 截图 `test-results/eff-01-home.png`
        - 测试用例2：**Task 列表页提效比列和图表**
          - 访问 `/task-v2`，等待数据加载
          - 断言表格存在"实际耗时"和"提效比"表头
          - 断言至少有一个 `el-tag` 类型为 success/primary/info 显示提效比
          - 断言图表区域存在（`.kb-charts-area`）
          - 截图 `test-results/eff-02-task-list.png`
        - 测试用例3：**Commit 列表页汇总卡片和提效比列**
          - 访问 `/commit-v2`，等待数据加载
          - 断言 4 个汇总卡片存在（总Commit数、总古法预估、总实际耗时、平均提效比）
          - 断言表格存在"提效比"表头且有 el-tag 彩色徽章
          - 截图 `test-results/eff-03-commit-list.png`
        - 测试用例4：**Task 详情页提效卡片替换**
          - 先通过 API 获取一个 task_id，访问 `/task/:taskId`
          - 断言统计摘要区域显示"古法预估"、"实际耗时"、"提效比"（而非"总请求数"、"总Tokens"、"总费用"）
          - 断言提效比卡片有动态颜色（检查 style 或 class）
          - 截图 `test-results/eff-04-task-detail.png`
        - 测试用例5：**Commit 详情页提效卡片新增**
          - 先通过 API 获取一个 commit_id 和 repo_id，访问 `/commit/:commitId?repoId=xxx`
          - 断言元信息和关联Tasks之间存在 3 个提效卡片（古法预估、实际耗时、提效比）
          - 截图 `test-results/eff-05-commit-detail.png`
        - 测试用例6：**全站无 JS 错误无 API 500**
          - 依次访问 `/`、`/task-v2`、`/commit-v2`，监听 `pageerror` 和 response status >= 500
          - 断言无 JS 错误、无 API 500

- [ ] 5.2 运行 Playwright 测试并验证通过
     【目标对象】`frontend/` 目录
     【修改目的】确保所有测试用例通过，提效 UI 改动符合预期
     【修改方式】在 `frontend/` 目录执行 `npx playwright test e2e/highlight-efficiency.spec.js`
     【相关依赖】任务 5.1 完成；前后端服务已启动（`localhost:8880`）
     【修改内容】
        - 执行测试命令，检查所有用例是否通过
        - 若有失败：根据截图和错误信息定位问题，回到对应任务修复后重新测试
        - 检查 `test-results/` 目录下截图，人工确认提效信息在视觉上确实突出醒目

### 阶段 6：Bug 修复

- [x] 6.1 修复 Task 详情页仓库和 Project 显示相同值
     【目标对象】`frontend/src/views/TaskDetailV2.vue`
     【修改内容】修改 `repoDisplay` computed，当 `repo_id === project_id` 时返回 `-`；模板 `v-if` 条件改为 `repoDisplay !== '-'`

- [x] 6.2 Task 详情页 reason 字段通过 tooltip 展示
     【目标对象】`frontend/src/views/TaskDetailV2.vue`
     【修改内容】import `QuestionFilled` 图标；在 el-descriptions 和提效摘要卡片的"古法预估"/"实际耗时"旁添加 `el-tooltip` + `QuestionFilled` 问号图标，仅当 reason 有值时显示；添加全局 `.reason-tooltip { max-width: 400px }` 样式

- [x] 6.3 Commit 详情页 reason 字段通过 tooltip 展示
     【目标对象】`frontend/src/views/CommitDetailV2.vue`
     【修改内容】同 6.2，对 `commit_ancient_minutes_reason` 和 `commit_real_minutes_reason` 添加 tooltip 问号图标
