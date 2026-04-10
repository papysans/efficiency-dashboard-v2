## 实施

- [x] 5.1 重写 API 层适配后端新路由
     【目标对象】`frontend/src/api/es.js`
     【修改目的】适配后端新路由（/api/indices 等），覆盖全部 18 个后端 API
     【修改方式】重写 es.js（或拆分为多个 API 模块文件）
     【修改内容】
        - 基础查询 API：getIndices(), getRequests(params), getTasks(params), getTasksSummary(params)
        - 聚合 API：getAggregate(params), getAggregateSummary(params)
        - 提效分析 API：getEfficiency(params), calculateEfficiency(params), correctEfficiency(params), getEfficiencyHistory(params), getEfficiencyFile(params), updateManualDays(params)
        - Git 分析 API：getGitAnalysis(params), triggerGitAnalysis(params), getGitCommits(params)
        - 关联分析 API：getTaskCommitMappings(params), getCodeAttribution(params), getCodeSourceStats(params)

- [x] 5.2 重构 Dashboard 页面
     【目标对象】`frontend/src/views/Dashboard.vue`
     【修改目的】适配新 API，改为 Request 数据 + Task 聚合数据双视图
     【修改方式】重写组件
     【修改内容】
        - Tab 1: Request 原始数据（调用 getRequests），表格列参照 a.md 7.1.1
        - Tab 2: Task 聚合数据（调用 getAggregate），支持维度切换（project/user/org1-4），表格列参照 a.md 7.1.2
        - 日期范围选择（默认最近 7 天）
        - 分页表格
        - URL 参数同步

- [x] 5.3 新增提效分析面板
     【目标对象】`frontend/src/views/EfficiencyPanel.vue`（新增）
     【修改目的】核心功能——展示 AI 提效分析数据
     【修改方式】新增组件
     【相关依赖】后端 /api/analysis/efficiency, /api/analysis/code-attribution
     【修改内容】
        - 维度切换：Project / Repo
        - 指标卡片区：AI 预估人天、实际 Lead Time、实际 Process Time、提效比例(Lead/Process)、投入成本、节省成本、ROI
        - 提效比例颜色标识：>100% 绿色，<100% 红色
        - 用户参与详情表：用户名、开始/结束时间、Lead Time、Process Time、占比
        - 代码来源统计：AI代码行数、人工代码行数、其他
        - 纠错入口：AI预估人天卡片上的编辑图标
        - 日期范围筛选
        - 项目/仓库 ID 输入或下拉选择

- [x] 5.4 重构 Project Panel
     【目标对象】`frontend/src/views/ProjectPanel.vue`
     【修改目的】改为项目列表 + 提效概览，点击可下钻到提效详情
     【修改方式】重写组件
     【修改内容】
        - 项目列表表格：从 getAggregate(dimension=project) 获取数据
        - 表格列：项目ID、任务数、API次数、代码行数、API费用、AI预估人天、处理时长
        - 点击项目行跳转到提效分析详情（复用 EfficiencyPanel 组件）
        - 保留 ECharts 图表（代码行数、费用对比等）

- [x] 5.5 新增 Repo Panel
     【目标对象】`frontend/src/views/RepoPanel.vue`（新增）
     【修改目的】仓库维度分析，含 Git 分析数据展示
     【修改方式】新增组件
     【修改内容】
        - 仓库列表：从 getAggregate(dimension=repo) 获取数据
        - Git 分析区：调用 getGitAnalysis，展示 commit 统计、贡献者、代码变更
        - Task-Commit 关联：调用 getTaskCommitMappings，展示关联关系表格
        - 代码来源分析：调用 getCodeAttribution，展示 AI/人工代码行数对比
        - 点击仓库可下钻到提效详情

- [x] 5.6 新增 User Panel
     【目标对象】`frontend/src/views/UserPanel.vue`（新增）
     【修改目的】用户维度的 AI 编码活动展示
     【修改方式】新增组件
     【修改内容】
        - 用户列表：从 getAggregate(dimension=user) 获取数据
        - 表格列：用户名、任务数、代码行数、API费用、AI预估人天、处理时长
        - 个人提效统计

- [x] 5.7 新增 Org Panel
     【目标对象】`frontend/src/views/OrgPanel.vue`（新增）
     【修改目的】组织层级导航和提效对比
     【修改方式】新增组件
     【修改内容】
        - 层级导航：org1 → org2 → org3 → org4，逐级下钻
        - 从 getAggregate(dimension=org1|org2|org3|org4) 获取数据
        - 各单元提效对比表格

- [x] 5.8 实现纠错功能 UI
     【目标对象】`frontend/src/components/CorrectionDialog.vue`（新增）
     【修改目的】提供 AI 预估人天纠错的对话框组件
     【修改方式】新增组件
     【修改内容】
        - el-dialog 弹窗：原始值(只读) + 纠正值(输入框) + 纠正原因(必填文本域) + 操作人(自动填充)
        - 调用 correctEfficiency API 提交纠错
        - 纠正后指标卡标注"已人工校准"
        - 查看纠错历史链接（调用 getEfficiencyHistory）

- [x] 5.9 更新路由和导航
     【目标对象】`frontend/src/router/index.js` + `frontend/src/App.vue` + `frontend/src/views/Home.vue`
     【修改目的】注册新页面路由，更新导航菜单和首页卡片
     【修改方式】修改
     【修改内容】
        - router/index.js 新增路由：/efficiency, /repo-panel, /user-panel, /org-panel
        - App.vue 导航菜单新增：提效分析、仓库面板、用户面板、组织面板
        - Home.vue 首页卡片新增对应入口

- [x] 5.10 构建验证
     【目标对象】`frontend/`
     【修改内容】
        - npm run build 构建通过
        - npm run dev 启动开发服务器
        - 页面可访问（即使后端未启动也不报错，显示空数据/加载状态）

- [x] restructure-frontend | task: 5.8-fix-1 修复纠错对话框 API 参数与后端不匹配问题
     【目标对象】`frontend/src/components/CorrectionDialog.vue` + `frontend/src/views/EfficiencyPanel.vue`
     【修改目的】修复 CorrectionDialog 纠错功能无法正常工作的 3 个 BUG
     【修改内容】
        - 修复1（严重）：correctEfficiency API 调用中 `operator` 字段名改为 `by`（后端 CorrectionRequest 结构体 json tag 为 "by"）
        - 修复2（严重）：CorrectionDialog 新增 `startDate`/`endDate` props，在 correctEfficiency API 调用中传入（后端要求这两个参数）
        - 修复3（中等）：操作人字段增加必填校验（后端要求 by 不能为空），placeholder 改为"请输入操作人（必填）"
        - EfficiencyPanel.vue 给 CorrectionDialog 组件传入 :start-date 和 :end-date props（取自当前 dateRange 日期范围，格式为 YYYYMMDD）

- [x] 6.1 新建 TaskViewV2.vue（Task 列表页 /task-v2）
     【目标对象】`frontend/src/views/TaskViewV2.vue`（新建）
     【修改目的】新增 Task 列表页，与 UserViewV2/ProjectViewV2 风格一致
     【修改内容】
        - 筛选区：el-date-picker(daterange, 默认90天) + el-input(搜索task_id/用户名, width:220px, clearable) + el-button(查询)
        - 表格列：Task ID(prop:task_id, show-overflow-tooltip, width:220) | 用户(prop:user_name) | 项目(prop:project_id, show-overflow-tooltip) | 模式(prop:caller, width:80) | 开始时间(prop:start_time, formatter截取前16字符, width:150) | 费用(prop:cost, formatter保留4位) | Diff行数(prop:diff_lines, width:80) | AI预估人天(prop:ai_estimated_ancient_days, width:100)
        - 行点击：router.push('/task/' + row.task_id)
        - 搜索：前端 computed 过滤（task_id 或 user_name 包含关键词）
        - 图表：费用分布(按Task) + Diff行数分布(按Task)，使用 createBarOption
        - API：调用 getTasksV2({ startDate, endDate, page, pageSize })
        - 参考 UserViewV2.vue 的代码结构

- [x] 6.2 Repo 列表页路径从 /project-v2 改为 /repo-v2
     【目标对象】`frontend/src/router/index.js` + `frontend/src/App.vue` + `frontend/src/views/Home.vue`
     【修改内容】
        - router/index.js：path: '/project-v2' → '/repo-v2'
        - App.vue 菜单：index="/project-v2" → index="/repo-v2"
        - Home.vue 中所有 /project-v2 跳转链接 → /repo-v2

- [x] 6.3 清除旧版路由，router/index.js 只保留 9 条
     【目标对象】`frontend/src/router/index.js`
     【修改内容】删除以下路由：
        - /dashboard → Dashboard.vue
        - /project-panel → ProjectPanel.vue
        - /efficiency → EfficiencyPanel.vue
        - /repo-panel → RepoPanel.vue
        - /user-panel → UserPanel.vue
        - /org-panel → OrgPanel.vue
        - /project/:projectId → ProjectDetailV2.vue
     保留最终 9 条路由：
        - / → Home.vue
        - /repo-v2 → ProjectViewV2.vue
        - /repo/:repoId → ProjectDetailV2.vue
        - /user-v2 → UserViewV2.vue
        - /user/:userId → UserDetailV2.vue
        - /org-v2 → OrgViewV2.vue
        - /org/:orgPath → OrgDetailV2.vue
        - /task-v2 → TaskViewV2.vue（新增）
        - /task/:taskId → TaskDetailV2.vue

- [x] 6.4 App.vue 导航菜单清理
     【目标对象】`frontend/src/App.vue`
     【修改内容】删除"更多"子菜单，主导航改为5项：首页(/) | 仓库(/repo-v2) | 用户(/user-v2) | 组织(/org-v2) | 任务(/task-v2)

- [x] 6.5 提取 FilterBar 通用筛选组件
     【目标对象】`frontend/src/components/FilterBar.vue`（新建）
     【修改内容】
        - 模板：el-card.kb-filter-card > div.kb-filter-row > el-date-picker(v-model dateRange, daterange) + slot(额外筛选控件) + el-button(查询)
        - Props：dateRange (Array, required)，支持 v-model:dateRange
        - Emits：update:dateRange, search
     然后在以下页面替换为 FilterBar：
        - ProjectViewV2.vue（slot 放搜索框+触发关联分析按钮）
        - UserViewV2.vue（slot 放搜索框）
        - OrgViewV2.vue（slot 放级联下拉）
        - TaskViewV2.vue（slot 放搜索框）

- [x] 6.6 Home.vue 默认日期范围改为 90 天
     【目标对象】`frontend/src/views/Home.vue`
     【修改内容】import 改用 getDefaultDateRangeWide，dateRange 默认值改为 getDefaultDateRangeWide()

- [x] 6.7 构建验证
     【目标对象】`frontend/`
     【修改内容】npm run build 通过

- [x] restructure-frontend | task: 6.2-fix-1 修复 TaskDetailV2.vue 中残留的 /project-v2 死链接
     【目标对象】`frontend/src/views/TaskDetailV2.vue`
     【修改目的】由于 /project-v2 路由已在 6.3 中被删除，TaskDetailV2.vue 第17行仍引用 /project-v2 导致死链接
     【修改内容】
        - 将第17行 `router.push({ path: '/project-v2', query: { repoId: task.repo_id } })` 改为 `router.push('/repo/' + encodeURIComponent(task.repo_id))`，直接跳转到仓库详情页
