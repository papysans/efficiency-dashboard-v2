## 实施

- [x] 5.1 前端新增 v2 API 函数
     【目标对象】`frontend/src/api/es.js`
     【修改目的】封装后端 v2 API 调用，供新视图页面使用
     【修改方式】在文件末尾新增 export 函数，遵循现有 API 函数模式（如 `getAggregate` 使用 `request({ url, method, params })` 格式）
     【相关依赖】`frontend/src/api/index.js` 导出的 axios 实例 `request`（文件顶部已有 `import request from './index'`）
     【修改内容】
        - `getTasksV2(params)` — GET `/v2/tasks`，params: { userId, repoId, projectId, startDate, endDate, page, pageSize }
        - `getTaskDetailV2(taskId)` — GET `/v2/tasks/${taskId}`，taskId 为路径参数
        - `getCommitsV2(params)` — GET `/v2/commits`，params: { repoId, userId, startDate, endDate, page, pageSize }
        - `getCommitDetailV2(commitId, repoId)` — GET `/v2/commits/${commitId}`，repoId 作为 query params 传递（后端通过 `c.Query("repoId")` 获取）
        - `getProjectsV2(params)` — GET `/v2/projects`，params: { startDate, endDate, page, pageSize }
        - `getProjectDetailV2(repoId)` — GET `/v2/projects/${repoId}`，repoId 为路径参数（后端通过 `c.Param("repoId")` 获取）
        - `triggerProjectAssociation(repoId)` — POST `/v2/projects/associate`，repoId 作为可选 query params 传递（后端通过 `c.Query("repoId")` 获取）
        - `updateProjectManualV2(repoId, data)` — PUT `/v2/projects/${repoId}/manual`，repoId 为路径参数，data 为 JSON body

- [x] 5.2 新建 Project 视图页面
     【目标对象】`frontend/src/views/ProjectViewV2.vue`（新建）
     【修改目的】实现设计文档要求的 Project 视图：从 project 看到参与者、关联 commits、关联 tasks、硅比例
     【修改方式】新建 Vue SFC 文件，参考 `RepoPanel.vue` 的行点击展开详情模式 + `ProjectPanel.vue` 的筛选/表格/图表结构
     【相关依赖】
        - 任务 5.1 中新增的 v2 API 函数（`getProjectsV2`, `getProjectDetailV2`, `triggerProjectAssociation`）
        - `@/composables/useChart`（图表实例管理）、`@/composables/useUrlSync`（URL 参数同步）
        - `@/utils/formatters`（`fmtCost`, `fmtDays`）、`@/utils/chart`（`createBarOption`, `createDualBarOption`）、`@/utils/date`（`getDefaultDateRange`）
        - `frontend/src/style.css` 的公共 kb- 类名（已全局引入，无需在组件中导入）
     【修改内容】
        **页面布局结构**：
        ```
        kb-panel
          ├── kb-filter-card（筛选区）
          │     └── el-date-picker(daterange) + el-button(查询) + el-button(触发关联分析)
          ├── kb-table-card（Project 列表表格）
          │     ├── el-table 列: repo_id / task数 / commit数 / 总tokens / 总费用 / AI预估人天 / 时间范围
          │     └── kb-pagination
          ├── 详情区（v-if="selectedProject"）— 参考 RepoPanel 的展开模式
          │     ├── 标题行: "项目详情: {repo_id}" + 关闭按钮
          │     ├── 4个指标卡片(kb-metric-card): 关联Task数 / 关联Commit数 / 总费用 / AI预估人天
          │     ├── 参与者列表(el-table): user_name / user_id / task数 / 贡献commits
          │     ├── Task列表(el-table): task_id / user_name / start_time / end_time / cost / diff_lines / silica(硅比例)
          │     ├── Commit列表(el-table): commit_id / git_user_name / commit_time / diff_lines / message
          │     └── 硅比例可视化: 横向柱状图(每个task的silica值)，使用 createBarOption 生成
          └── kb-charts-area（总览图表）
                ├── 费用分布(按project) — 横向柱状图，使用 createBarOption
                └── Token使用量(按project) — 双柱图，使用 createDualBarOption
        ```
        **交互逻辑**：
        - 页面加载 → 调用 `getProjectsV2` 获取列表 → 填充表格和图表
        - 行点击 → `selectedProject = row` → 调用 `getProjectDetailV2(row.repo_id)` → 展开详情区
        - 详情 API 返回结构为 `{ project, tasks, commits }`，tasks 和 commits 已展开为完整记录数组，直接用于渲染详情区子表格
        - 参与者列表：从返回的 `tasks` 数组中按 `user_id` 分组聚合，计算每人的 task 数和 cost 总和
        - task 列表中的 silica 列：从 `project.task_ids_silica` 数组按索引与 `project.task_ids` 对应，匹配到各 task 记录
        - "触发关联分析"按钮 → 调用 `triggerProjectAssociation()` → 成功后刷新列表
        - 日期格式：UI 层 `YYYY-MM-DD`（el-date-picker value-format），传 API 时 `.replace(/-/g, '')` 转为 `YYYYMMDD`
        - 错误处理：API 调用统一用 try/catch，错误由 `api/index.js` 响应拦截器弹出 ElMessage，组件内 catch 中重置对应数据为空数组/null
        **数据格式处理**：
        - `project.task_ids` 是 JSONB 数组（后端返回 JSON array），直接使用
        - `project.task_ids_silica` 是 JSONB 数组，值为 float
        - 参与者聚合：遍历详情中的 tasks 数组，按 user_id 分组计算 task 数和 cost

- [x] 5.3 路由和导航更新
     【目标对象】`frontend/src/router/index.js` + `frontend/src/App.vue`
     【修改目的】将新 Project 视图集成到应用导航中
     【修改方式】在 `router/index.js` 的 `routes` 数组中新增路由条目；在 `App.vue` 的 `<el-menu>` 中新增 `<el-menu-item>`
     【相关依赖】任务 5.2 新建的 `frontend/src/views/ProjectViewV2.vue`
     【修改内容】
        - `router/index.js`：在 routes 数组中（`/project-panel` 路由之后）新增 `{ path: '/project-v2', name: 'ProjectV2', component: () => import('@/views/ProjectViewV2.vue') }`，注意使用 `@/views/` 前缀与现有路由风格保持一致（现有路由使用 `@/views/` 格式）
        - `App.vue`：在 `<el-menu-item index="/project-panel">项目</el-menu-item>` 之后新增 `<el-menu-item index="/project-v2">项目(v2)</el-menu-item>`

- [x] add-project-view | task: 5.2-fix-1 参与者列表第4列修复
     【目标对象】`frontend/src/views/ProjectViewV2.vue`
     【修改目的】将参与者列表第4列从"贡献费用"(total_cost) 修正为"贡献commits"（commit 数量），与 task.md 第39行规格一致
     【修改方式】修改 participants computed 逻辑和模板中参与者表格的第4列
     【修改内容】
        - participants computed：增加对 `detailData.commits` 数组的遍历，按 `user_name` 与 tasks 中的 `user_name` 匹配，统计每个参与者的 commit 数量（commit_count）
        - 模板中参与者表格：将第4列 `prop="total_cost" label="贡献费用" :formatter="fmtCost"` 改为 `prop="commit_count" label="贡献commits"`

- [x] 5.4 前端构建验证
     【目标对象】`frontend/` 目录
     【修改目的】验证新页面编译通过且可正常渲染
     【修改方式】执行构建命令验证，不修改代码
     【相关依赖】任务 5.1、5.2、5.3 的所有变更
     【修改内容】
        - 在 `frontend/` 目录执行 `npm run build` 确认无编译错误
         - 启动 dev server（`npm run dev`），在浏览器访问 `/project-v2`
         - 确认页面可渲染，API 调用无报错（需要 backend 服务运行中）

- [x] add-project-view | task: 5.2-fix-2 修复 listProjectsV2 返回数据格式 + 前端字段名
     【目标对象】`backend/project_handler_v2.go` + `frontend/src/views/ProjectViewV2.vue`
     【修改目的】
       1. 后端 listProjectsV2 当前直接返回原始 CostrictProject 对象，前端表格期望 task_count/commit_count/total_tokens/total_cost/ai_estimated_days/min_time/max_time 等计算字段
       2. 前端 ProjectViewV2.vue 第 270 行用 `data.items` 但后端返回 `data.data` 
     【修改方式】
       1. 在 `backend/project_handler_v2.go` 的 `listProjectsV2` 函数中，将 `[]CostrictProject` 转换为聚合响应对象列表，每个对象包含前端表格需要的字段
       2. 在 `frontend/src/views/ProjectViewV2.vue` 第 270 行将 `data.items` 改为 `data.data`
     【修改内容】
        - `backend/project_handler_v2.go`：在 listProjectsV2 函数中（分页切片 pagedSlice 之后），遍历 pagedSlice 构造响应数组。每个元素为 map/struct，包含：`repo_id`(=p.RepoID)、`task_count`(从 p.TaskIDs 解析 JSON 数组计算 len)、`commit_count`(从 p.CommitIDs 解析 JSON 数组计算 len)、`total_tokens`(p.UpstreamTokens + p.DownstreamTokens，注意指针判空)、`total_cost`(p.Cost)、`ai_estimated_days`(p.AIEstimatedAncientDays)、`min_time`(p.StartTime 格式化为 YYYY-MM-DD)、`max_time`(p.EndTime 格式化为 YYYY-MM-DD)。将 `"data": pagedSlice` 改为 `"data": items`（items 为构造的聚合数组）
        - `frontend/src/views/ProjectViewV2.vue` 第 270 行：将 `tableData.value = data.items || []` 改为 `tableData.value = data.data || []`

- [x] add-project-view | task: 5.2-fix-3 修复 listProjectsV2 过滤空 repo_id 的记录
     【目标对象】`backend/project_handler_v2.go`
     【修改目的】数据库存在 repo_id 为空字符串的垃圾数据，导致测试 1.5 中 `data[0].repo_id` 为空。需在 handler 层过滤掉空 RepoID 的记录
     【修改方式】在 `listProjectsV2` 函数中，调用 `ListCostrictProjects` 之后、分页之前，过滤掉 `RepoID == ""` 的记录
     【修改内容】
        - 在 `all, err := ListCostrictProjects(db, startTime, endTime)` 之后，添加过滤逻辑：遍历 all，只保留 `p.RepoID != ""` 的记录到新切片 filtered，然后将 all 替换为 filtered

- [x] add-project-view | task: 5.2-fix-4 新增 /projects/detail 路由支持 query 参数
     【目标对象】`backend/project_handler_v2.go` + `backend/main.go`
     【修改目的】测试用 GET `/api/v2/projects/detail?repoId=xxx` 获取项目详情，但后端只有路径参数路由 `/projects/:repoId`。需新增静态路由 `/projects/detail` 支持 query 参数
     【修改方式】
       1. 在 `project_handler_v2.go` 新增 `getProjectDetailByQueryV2` handler 函数，从 `c.Query("repoId")` 读取参数，复用与 `getProjectDetailV2` 相同的查询逻辑
       2. 在 `main.go` 的路由注册中，在 `v2.GET("/projects/:repoId", ...)` 之前注册 `v2.GET("/projects/detail", getProjectDetailByQueryV2)`（Gin 静态路由优先于参数路由）
     【修改内容】
        - `backend/project_handler_v2.go`：新增 `getProjectDetailByQueryV2` 函数，从 `c.Query("repoId")` 获取 repoId，若为空返回 400 错误，否则复用 `getProjectDetailV2` 中的查询逻辑（GetCostrictProject → 解析 task_ids/commit_ids → 返回 project/tasks/commits）。为避免代码重复，可提取公共函数 `projectDetailResponse(c *gin.Context, repoID string)`
        - `backend/main.go`：在 v2 路由组中 `v2.GET("/projects", listProjectsV2)` 之后、`v2.GET("/projects/:repoId", getProjectDetailV2)` 之前，注册 `v2.GET("/projects/detail", getProjectDetailByQueryV2)`

- [x] add-project-view | task: 5.2-fix-5 前端 ProjectViewV2 默认日期范围改为 90 天
     【目标对象】`frontend/src/views/ProjectViewV2.vue`
     【修改目的】默认日期范围 7 天（getDefaultDateRange）太短，seed 数据的 project start_time 均在 30 天以前，导致前端列表 0 行
     【修改方式】在 `onMounted` 中将 `dateRange.value = getDefaultDateRange()` 改为使用更宽的日期范围（最近 90 天）
     【修改内容】
        - 在 `ProjectViewV2.vue` 的 `onMounted` 回调中，将 `dateRange.value = getDefaultDateRange()` 替换为内联计算 90 天范围的逻辑：`const end = new Date(); const start = new Date(); start.setDate(start.getDate() - 89); const fmt = (d) => { ... }; dateRange.value = [fmt(start), fmt(end)]`
        - 或者更简洁地，在 `@/utils/date.js` 中新增 `getDefaultDateRangeWide(days = 90)` 函数，在 ProjectViewV2.vue 中调用它。但考虑最小修改原则，直接在 `onMounted` 中修改参数即可
