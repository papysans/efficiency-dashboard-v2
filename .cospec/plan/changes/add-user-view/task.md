## 实施

- [x] 6.1 后端新增 User 聚合查询 API
     【目标对象】`backend/user_handler_v2.go`（新建）+ `backend/main.go`
     【修改目的】从 PG 的 costrict_tasks 和 costrict_commits 中聚合用户列表和用户维度统计
     【修改方式】新建 handler 文件，编写 SQL 聚合查询，注册路由
     【相关依赖】`backend/db.go` 的 ListCostrictTasks/ListCostrictCommits，全局变量 db
     【修改内容】
        - `listUsersV2(c *gin.Context)`：GET `/api/v2/users`，支持 startDate/endDate + page/pageSize
          - 执行 SQL 聚合：从 costrict_tasks 按 user_id 分组，统计 task_count, total_cost, total_upstream_tokens, total_downstream_tokens, min(start_time), max(end_time), 去重 repo_id 数量(project_count)
          - 同时从 costrict_commits 按 user_id 统计 commit_count, total_diff_lines
          - 合并两个结果集，返回用户列表
          - 返回格式 `{"total":N,"page":P,"pageSize":S,"data":[{user_id, user_name, task_count, commit_count, project_count, total_cost, total_tokens, total_diff_lines, first_active, last_active}]}`
        - `getUserDetailV2(c *gin.Context)`：GET `/api/v2/users/:userId`
          - 查询该用户的所有 tasks（ListCostrictTasks 按 userId 过滤）
          - 查询该用户的所有 commits（ListCostrictCommits 按 userId 过滤）
          - 从 tasks 中提取去重的 repo_id 列表 → 查询对应的 projects（GetCostrictProject）
          - 返回 `{"user":{user_id, user_name, ...统计}, "tasks":[...], "commits":[...], "projects":[...]}`
        - 在 `main.go` 的 v2 路由组注册：`v2.GET("/users", listUsersV2)` 和 `v2.GET("/users/:userId", getUserDetailV2)`

- [x] 6.2 前端新增 User v2 API 函数
     【目标对象】`frontend/src/api/es.js`
     【修改目的】封装后端 User v2 API 调用
     【修改方式】在文件末尾追加新函数
     【修改内容】
        - `getUsersV2(params)` — GET `/v2/users`，params: { startDate, endDate, page, pageSize }
        - `getUserDetailV2(userId)` — GET `/v2/users/${userId}`

- [x] 6.3 新建 User 视图页面
     【目标对象】`frontend/src/views/UserViewV2.vue`（新建）
     【修改目的】实现从用户维度查看参与项目、tasks、commits 的完整视图
     【修改方式】新建 Vue SFC 文件，参考 ProjectViewV2.vue 的行点击展开详情模式
     【相关依赖】任务 6.2 的 v2 API 函数；composables: useChart；utils: formatters, chart, date；style.css 公共类名
     【修改内容】
        **页面布局**：
        ```
        kb-panel
          ├── kb-filter-card（筛选区）
          │     └── el-date-picker(daterange) + el-button(查询)
          ├── kb-table-card（用户列表表格）
          │     ├── el-table 列: user_name / user_id / 参与项目数 / task数 / commit数 / 总费用 / 总diff行数 / 活跃时间范围
          │     └── kb-pagination
          ├── 详情区（v-if="selectedUser"）
          │     ├── 标题行: "用户详情: {user_name}" + 关闭按钮
          │     ├── 4个指标卡片: 参与项目数 / Task数 / Commit数 / 总费用
          │     ├── 参与项目列表(el-table): repo_id / 角色(task数) / 硅比例均值 / 费用 — 行可点击跳转到 /project-v2
          │     ├── Task列表(el-table): task_id / repo / start_time / end_time / cost / diff_lines
          │     ├── Commit列表(el-table): commit_id / repo / commit_time / diff_lines / message
          │     └── 贡献趋势图: 按日/周的 task 和 commit 数量趋势线图
          └── kb-charts-area（总览图表）
                ├── 费用分布(按用户) — 横向柱状图
                └── 代码产出(按用户) — 横向柱状图
        ```
        **交互逻辑**：
        - 页面加载 → getUsersV2 获取列表
        - 行点击 → getUserDetailV2(userId) → 展开详情
        - 参与项目行点击 → router.push('/project-v2') 跳转到项目视图
        - Task 行可跳转到 task 详情

- [x] 6.4 路由和导航更新 + 构建验证
     【目标对象】`frontend/src/router/index.js` + `frontend/src/App.vue` + frontend/
     【修改目的】将 User 视图集成到导航并验证构建
     【修改内容】
        - router/index.js：新增 `{ path: '/user-v2', name: 'UserV2', component: () => import('../views/UserViewV2.vue') }`
        - App.vue：导航菜单新增"用户(v2)"条目
        - backend/ 目录 `go build ./...` 编译通过
        - frontend/ 目录 `npm run build` 构建通过

- [x] add-user-view | task: 6.3-fix-1 UserViewV2.vue 详情区补全遗漏功能
     【目标对象】`frontend/src/views/UserViewV2.vue`
     【修改目的】补全代码审查发现的 3 处实现遗漏
     【修改方式】修改 UserViewV2.vue 的模板和脚本部分
     【修改内容】
        1. **参与项目列表增加 3 列**：
           - 新增 `projectStats` computed，从 `detailData.tasks` 按 `repo_id` 分组统计每个项目的 task_count 和 cost_sum，从 `detailData.projects` 的 `task_ids_silica` 计算硅比例均值
           - 参与项目表格 `:data` 改为 `projectStats`
           - 增加列：角色(task数) / 硅比例均值 / 费用
        2. **Commit 列表增加 message 列**：
           - 在 Commit 表格末尾添加 `<el-table-column prop="message" label="提交信息" min-width="300" show-overflow-tooltip />`（与 ProjectViewV2 对齐）
        3. **新增贡献趋势图**：
           - 在 Commit 列表之后、详情卡片结束之前添加趋势图
           - 新增 `trendChartRef` + `useChart` 实例
           - 新增 `updateTrendChart()` 函数：从 `detailData.tasks` 按 `start_time` 日期分组统计每日 task 数，从 `detailData.commits` 按 `commit_time` 分组统计每日 commit 数，生成双折线图（x 轴=日期，两条线=task 数/commit 数）
           - 在 `handleRowClick` 成功获取详情后调用 `updateTrendChart()`
     【构建验证】修改完成后 `npm run build` 必须通过
