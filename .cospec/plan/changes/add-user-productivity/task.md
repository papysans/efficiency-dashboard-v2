## 实施

- [x] 1.1 新增 user_productivity 和 user_groups 建表语句
     【目标对象】`init_db_stat.sql`
     【修改目的】在 costrict_stat 数据库中创建 user_productivity 预聚合表和 user_groups 虚拟组表，为按 user+日期 维度的生产力数据存储提供基础
     【修改方式】在文件末尾追加两张表的 CREATE TABLE IF NOT EXISTS 语句及索引（新增）
     【相关依赖】现有的 tasks、commits 表结构（字段名和类型需与聚合源保持一致）
     【修改内容】
        - `user_productivity` 表：主键 `user_productivity_id`(VARCHAR, 格式为 user_id+日期拼接)，字段包含 create_time(TIMESTAMPTZ), user_id(VARCHAR 255), user_name(VARCHAR 255), task_ids(JSONB), work_dir_ids(JSONB), task_diff_lines(INT), upstream_tokens(BIGINT), downstream_tokens(BIGINT), cost(FLOAT8), task_real_minutes(FLOAT8), task_ancient_minutes(FLOAT8), task_efficiency_ratio(FLOAT8), commit_ids(JSONB), commit_diff_lines(INT), commit_ancient_minutes(FLOAT8), commit_real_ai_minutes(FLOAT8), commit_real_ancient_minutes(FLOAT8), commit_real_minutes(FLOAT8), commit_efficiency_ratio(FLOAT8), created_at(TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP), updated_at(TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP)
        - user_productivity 索引：user_id, create_time
        - `user_groups` 表：主键 `group_id`(UUID DEFAULT gen_random_uuid()), 字段 name(VARCHAR 500 NOT NULL), user_ids(JSONB DEFAULT '[]'), created_at(TIMESTAMPTZ DEFAULT NOW()), updated_at(TIMESTAMPTZ DEFAULT NOW())
        - user_groups 索引：name
        - 注意：UUID 生成和默认值风格参考现有 projects 表的写法

- [x] 1.2 新增 user_productivity DB 层 CRUD 函数
     【目标对象】`backend/db.go`
     【修改目的】提供 user_productivity 表的数据访问层，供 handler 调用
     【修改方式】在文件末尾新增结构体和函数，复用现有的 SQL 动态条件构造模式（参考 ListStatTasks 的 argIdx + conditions 模式）和 scan 辅助模式（参考 scanStatTask）
     【相关依赖】`statDB` 全局变量（handler 调用时传入）；现有的 `ListStatTasks`/`CountStatTasks` 函数签名模式；`rowScanner` 接口
     【修改内容】
        - 定义 `UserProductivity` 结构体（所有字段与表一一对应，JSONB 字段使用 json.RawMessage 类型，参考 StatCommit 的 TaskIDs 字段处理方式）
        - 定义 `userProductivitySelectColumns` 变量和 `scanUserProductivity` 辅助函数（参考 statTaskSelectColumns 和 scanStatTask 模式）
        - `UpsertUserProductivity(db *sql.DB, up *UserProductivity) error` — INSERT ON CONFLICT(user_productivity_id) DO UPDATE，使用 $1/$2 占位符风格，用于 rebuild 时写入
        - `ListUserProductivity(db *sql.DB, userId, startTime, endTime string, page, pageSize int) ([]UserProductivity, error)` — 按 user_id + 时间范围查询，使用 argIdx 动态条件构造，分页，ORDER BY create_time DESC
        - `CountUserProductivity(db *sql.DB, userId, startTime, endTime string) (int, error)` — 按相同条件计数
        - `DeleteUserProductivityByDate(db *sql.DB, startDate, endDate string) error` — 按 create_time 范围删除旧数据，用于 rebuild 前清理
        - 所有函数的错误处理使用 `fmt.Errorf("描述: %w", err)` 包装风格（与现有函数一致）

- [x] 1.3 新增 user_groups DB 层 CRUD 函数
     【目标对象】`backend/db.go`
     【修改目的】提供 user_groups 表的数据访问层，供虚拟组 handler 调用
     【修改方式】在文件末尾新增结构体和函数（新增）
     【相关依赖】`statDB` 全局变量；`rowScanner` 接口
     【修改内容】
        - 定义 `UserGroup` 结构体（group_id string, name string, user_ids json.RawMessage, created_at *time.Time, updated_at *time.Time）
        - `CreateUserGroup(db *sql.DB, name string, userIDs []string) (*UserGroup, error)` — INSERT 后使用 RETURNING 返回新记录（参考 projects 表的 UUID 生成模式），user_ids 需转换为 JSON 字符串传入
        - `ListUserGroups(db *sql.DB) ([]UserGroup, error)` — 查询全部，ORDER BY created_at DESC
        - `GetUserGroup(db *sql.DB, groupId string) (*UserGroup, error)` — 按 group_id 查询单条，不存在返回 nil, nil（参考 GetStatTask 模式）
        - `DeleteUserGroup(db *sql.DB, groupId string) error` — 按 group_id 删除，检查 RowsAffected，不存在时返回错误

- [x] 1.4 新增 user_productivity_handler_v2.go 后端 handler
     【目标对象】`backend/user_productivity_handler_v2.go`（新文件）
     【修改目的】实现用户生产力的 rebuild、列表汇总、详情 API
     【修改方式】新建文件，package main，复用现有 handler 编码模式（参考 `user_handler_v2.go` 的日期参数解析和内存分页模式、`commit_handler_v2.go` 的批量查询模式）
     【相关依赖】`db.go` 中的 UserProductivity CRUD 函数、ListStatTasks、ListStatCommits；`utils.go` 的 `parseDateParam`；`es_handler.go` 的 `getDefaultInt`、`DefaultPageSize`；全局变量 `statDB`
     【修改内容】
        - `rebuildUserProductivity(c *gin.Context)` — POST handler：
          1. 接收 startDate/endDate 查询参数（YYYYMMDD 格式），使用 parseDateParam 解析并校验
          2. 先调用 DeleteUserProductivityByDate 清理指定日期范围的旧数据
          3. SQL 从 tasks 表按 user_id + DATE(start_time) GROUP BY 聚合：array_agg(task_id) as task_ids、array_agg(DISTINCT work_dir_id) as work_dir_ids、SUM(diff_lines) as task_diff_lines、SUM(upstream_tokens)、SUM(downstream_tokens)、SUM(cost)、SUM(task_real_minutes)、SUM(task_ancient_minutes)，同时取 MAX(user_name)
          4. SQL 从 commits 表按 user_id + DATE(commit_time) GROUP BY 聚合：array_agg(commit_id) as commit_ids、SUM(diff_lines) as commit_diff_lines、SUM(commit_ancient_minutes)、SUM(commit_real_ai_minutes)、SUM(commit_real_ancient_minutes)、SUM(commit_real_minutes)
          5. Go 内存合并：用 map[string]*UserProductivity（key 为 user_id+date 拼接）合并双表数据
          6. 计算 task_efficiency_ratio = task_ancient_minutes / task_real_minutes * 100（当 task_real_minutes > 0 时）；commit_efficiency_ratio 同理
          7. 在事务中批量 UpsertUserProductivity 写入，如果中途失败需 rollback 并返回错误
          8. 返回 { status: "ok", count: N }
          9. 边界处理：startDate/endDate 缺失时返回 400 错误；聚合结果为空时正常返回 count=0
        - `listUserProductivitySummary(c *gin.Context)` — GET handler：
          1. 接收 startDate/endDate 查询参数，使用 parseDateParam 解析，转换为 RFC3339 格式（参考 listUsersV2 中的日期处理）
          2. SQL 从 user_productivity 表按 user_id GROUP BY 聚合：MAX(user_name)、SUM 各数值指标、COUNT(*) as day_count
          3. 在 Go 中计算汇总 task_efficiency_ratio 和 commit_efficiency_ratio（加权计算）
          4. 使用 getDefaultInt 获取 page/pageSize，在内存中分页（参考 listUsersV2 的分页模式）
          5. 返回 { total, page, pageSize, data: [...] }
        - `getUserProductivityDetail(c *gin.Context)` — GET handler：
          1. 接收 userId 路径参数（c.Param）、startDate/endDate 查询参数
          2. 调用 ListUserProductivity 查询该用户的按天明细
          3. 在 Go 中计算汇总数据（SUM 各指标、整体 efficiency_ratio）
          4. 返回 { summary: {...}, daily: [...], total: N }
          5. 边界处理：userId 为空返回 400；无数据返回空 daily 数组和零值 summary

- [x] 1.5 新增 user_group_handler_v2.go 后端 handler
     【目标对象】`backend/user_group_handler_v2.go`（新文件）
     【修改目的】实现虚拟组的 CRUD 和组内用户生产力聚合查询 API
     【修改方式】新建文件，package main，复用现有 handler 编码模式（参考 `project_handler_v2.go` 的 CRUD 模式）
     【相关依赖】`db.go` 中的 UserGroup CRUD 函数和 ListUserProductivity 函数；`utils.go` 的 `parseDateParam`；全局变量 `statDB`
     【修改内容】
        - `createUserGroupHandler(c *gin.Context)` — POST handler：接收 { name, user_ids } JSON body（使用 ShouldBindJSON），调用 CreateUserGroup，返回创建的 group 对象。边界处理：name 为空或 user_ids 为空返回 400
        - `listUserGroupsHandler(c *gin.Context)` — GET handler：调用 ListUserGroups 返回全部，返回 { data: [...] }
        - `deleteUserGroupHandler(c *gin.Context)` — DELETE handler：从路径参数获取 groupId（c.Param），调用 DeleteUserGroup。不存在返回 404
        - `getUserGroupDetailHandler(c *gin.Context)` — GET handler：
          1. 从路径参数获取 groupId，调用 GetUserGroup 获取组定义，不存在返回 404
          2. 接收 startDate/endDate 查询参数
          3. 遍历组内 user_ids，对每个用户从 user_productivity 表查询并汇总数据（调用 ListUserProductivity）
          4. 计算组级汇总（SUM 各指标、加权 efficiency_ratio）
          5. 返回 { group: {...}, summary: {...}, members: [...] }

- [x] 1.6 注册新 API 路由
     【目标对象】`backend/main.go`
     【修改目的】将新 handler 注册到 V2 路由组
     【修改方式】在 `v2 := api.Group("/v2")` 代码块内，Projects 路由注册之后追加新路由定义（修改）
     【相关依赖】任务 1.4 的 handler 函数（rebuildUserProductivity、listUserProductivitySummary、getUserProductivityDetail）；任务 1.5 的 handler 函数（createUserGroupHandler、listUserGroupsHandler、deleteUserGroupHandler、getUserGroupDetailHandler）
     【修改内容】
        - 添加注释 `// User Productivity`
        - `v2.POST("/user-productivity/rebuild", rebuildUserProductivity)`
        - `v2.GET("/user-productivity", listUserProductivitySummary)`
        - `v2.GET("/user-productivity/:userId", getUserProductivityDetail)`
        - 添加注释 `// User Groups`
        - `v2.POST("/user-groups", createUserGroupHandler)`
        - `v2.GET("/user-groups", listUserGroupsHandler)`
        - `v2.DELETE("/user-groups/:groupId", deleteUserGroupHandler)`
        - `v2.GET("/user-groups/:groupId", getUserGroupDetailHandler)`

- [x] 1.7 新增前端 API 调用函数
     【目标对象】`frontend/src/api/es.js`
     【修改目的】新增 user-productivity 和 user-groups 的 API 调用函数
     【修改方式】在文件末尾追加 export 函数（新增），复用现有的 `request` 调用模式和命名风格（如 `getTasksV2`、`createProject` 等）
     【相关依赖】文件顶部已导入的 `request` axios 实例（`import request from './index'`）
     【修改内容】
        - `rebuildUserProductivity(params)` — `request({ url: '/v2/user-productivity/rebuild', method: 'post', params })`
        - `getUserProductivityList(params)` — `request({ url: '/v2/user-productivity', method: 'get', params })`
        - `getUserProductivityDetail(userId, params)` — `request({ url: \`/v2/user-productivity/${userId}\`, method: 'get', params })`
        - `createUserGroup(data)` — `request({ url: '/v2/user-groups', method: 'post', data })`
        - `getUserGroups()` — `request({ url: '/v2/user-groups', method: 'get' })`
        - `deleteUserGroup(groupId)` — `request({ url: \`/v2/user-groups/${groupId}\`, method: 'delete' })`
        - `getUserGroupDetail(groupId, params)` — `request({ url: \`/v2/user-groups/${groupId}\`, method: 'get', params })`

- [x] 1.8 新增前端路由定义
     【目标对象】`frontend/src/router/index.js`
     【修改目的】注册生产力相关页面的路由
     【修改方式】在 routes 数组中、现有路由定义之后追加新路由（修改）
     【相关依赖】新建的 Vue 组件文件（UserProductivityView.vue、UserProductivityDetail.vue、UserGroupDetail.vue）
     【修改内容】
        - `{ path: '/productivity', name: 'Productivity', component: () => import('@/views/UserProductivityView.vue') }`
        - `{ path: '/productivity/:userId', name: 'ProductivityDetail', component: () => import('@/views/UserProductivityDetail.vue') }`
        - `{ path: '/productivity/group/:groupId', name: 'ProductivityGroupDetail', component: () => import('@/views/UserGroupDetail.vue') }`
        - 注意：`/productivity/group/:groupId` 需定义在 `/productivity/:userId` 之前，避免 `group` 被解析为 userId 路径参数

- [x] 1.9 新增导航菜单入口
     【目标对象】`frontend/src/App.vue`
     【修改目的】在顶部导航菜单中添加"生产力"入口
     【修改方式】在 `<el-menu>` 内现有 `<el-menu-item>` 列表末尾追加一项（修改）
     【相关依赖】路由 `/productivity`（任务 1.8）
     【修改内容】
        - 在 `<el-menu-item index="/commit-v2">提交</el-menu-item>` 之后追加：`<el-menu-item index="/productivity">生产力</el-menu-item>`

- [x] 1.10 新增 UserProductivityView.vue 用户生产力列表页
     【目标对象】`frontend/src/views/UserProductivityView.vue`（新文件）
     【修改目的】以 user_id 视角展示汇总生产力数据，支持多选用户创建虚拟组
     【修改方式】新建 Vue 文件（`<script setup>` + `<template>` + `<style scoped>`），复用 `UserViewV2.vue` 的布局模式（日期筛选 + el-table），复用 `TaskViewV2.vue` 的多选操作模式
     【相关依赖】`FilterBar` 组件；`useChart` composable、`createBarOption` 图表工具；`getDefaultDateRangeWide` 日期工具；`fmtCost`、`formatDuration` 格式化工具；API 函数 `getUserProductivityList`、`rebuildUserProductivity`、`createUserGroup`、`getUserGroups`、`deleteUserGroup`
     【修改内容】
        - 页面顶部：日期范围选择器（el-date-picker，初始值使用 getDefaultDateRangeWide()）+ "刷新数据"按钮（调用 rebuildUserProductivity API，触发后重新加载列表）
        - 数据表格（el-table，支持 @selection-change 多选和 @row-click 行点击）：
          列：用户名, 活跃天数(day_count), Task数(task_count), Commit数(commit_count), Task代码行数(task_diff_lines), Task提效比(task_efficiency_ratio), Commit提效比(commit_efficiency_ratio), 总费用(cost, 使用 fmtCost 格式化), Tokens消耗(upstream_tokens+downstream_tokens)
          提效比列使用 el-tag 颜色标签（>=300 success 绿色, >=150 primary 蓝色, else info 灰色）
        - 行点击通过 router.push 跳转到 `/productivity/:userId`
        - 多选操作栏：选中多个用户后在表格上方显示"创建虚拟组"按钮，点击弹出 el-dialog 输入组名，确认后调用 createUserGroup API
        - 虚拟组列表区域：在表格下方展示已创建的虚拟组列表（调用 getUserGroups 获取），每项显示组名+成员数+删除按钮，点击组名跳转到 `/productivity/group/:groupId`
        - 底部图表：使用 useChart 渲染提效比分布柱状图（x轴用户名，y轴提效比）

- [x] 1.11 新增 UserProductivityDetail.vue 用户生产力详情页
     【目标对象】`frontend/src/views/UserProductivityDetail.vue`（新文件）
     【修改目的】展示单个用户的按天生产力明细和汇总
     【修改方式】新建 Vue 文件（`<script setup>` + `<template>` + `<style scoped>`），复用 `UserDetailV2.vue` 的指标卡片模式 + el-table 表格模式
     【相关依赖】`useChart` composable；`formatDuration`、`fmtCost` 格式化工具；`getDefaultDateRangeWide` 日期工具；API 函数 `getUserProductivityDetail`；`useRoute`、`useRouter`
     【修改内容】
        - 标题栏：el-page-header 返回按钮（router.back()）+ "用户生产力: {user_name}"
        - 日期范围筛选：el-date-picker（初始值 getDefaultDateRangeWide()），变更后重新请求 API
        - 顶部汇总卡片（el-row + el-col，参考 UserDetailV2 的 kb-metric-card 样式）：
          总活跃天数、总Task数、总Commit数、加权Task提效比(%)、加权Commit提效比(%)、总费用(fmtCost)
        - 中间：按天明细表格（el-table）：
          列：日期(create_time), Task数(task_ids 数组长度), Commit数(commit_ids 数组长度), Task代码行数(task_diff_lines), Commit代码行数(commit_diff_lines), Task实际耗时(task_real_minutes, formatDuration), Task传统耗时(task_ancient_minutes, formatDuration), Task提效比(el-tag 颜色标签), Commit实际耗时(commit_real_minutes, formatDuration), Commit传统耗时(commit_ancient_minutes, formatDuration), Commit提效比(el-tag 颜色标签), 费用(cost, fmtCost)
        - 底部趋势图：使用 useChart 渲染双轴折线图（x轴日期，左y轴 Task提效比，右y轴 Commit提效比）

- [x] 1.12 新增 UserGroupDetail.vue 虚拟组详情页
     【目标对象】`frontend/src/views/UserGroupDetail.vue`（新文件）
     【修改目的】展示虚拟组的汇总数据和成员明细
     【修改方式】新建 Vue 文件（`<script setup>` + `<template>` + `<style scoped>`），复用 UserProductivityDetail 的卡片模式
     【相关依赖】`formatDuration`、`fmtCost` 格式化工具；`getDefaultDateRangeWide` 日期工具；API 函数 `getUserGroupDetail`、`deleteUserGroup`；`useRoute`、`useRouter`
     【修改内容】
        - 标题栏：el-page-header 返回按钮 + "虚拟组: {group_name}" + el-button 删除按钮（调用 deleteUserGroup 后 router.push('/productivity') 返回列表）
        - 日期范围筛选：el-date-picker，变更后重新请求
        - 顶部汇总卡片（el-row + el-col）：组级汇总 — 总Task数、总Commit数、加权Task提效比、加权Commit提效比、总费用、成员数
        - 成员明细表格（el-table，@row-click 跳转）：
           列：用户名, 活跃天数, Task数, Commit数, Task提效比(el-tag), Commit提效比(el-tag), 费用(fmtCost)
           行点击通过 router.push 跳转到 `/productivity/:userId`

- [x] add-user-productivity | task: 1.11-fix-1 修复 UserProductivityDetail.vue 费用列缺少 prop
     【目标对象】`frontend/src/views/UserProductivityDetail.vue`
     【修改目的】修复按天明细表格中"费用"列因缺少 `prop="cost"` 导致 fmtCost formatter 无法获取值、费用始终显示为空的 BUG
     【修改方式】在第 112 行的 `<el-table-column label="费用" ...>` 中添加 `prop="cost"` 属性
     【修改内容】将 `<el-table-column label="费用" width="100" align="right" :formatter="fmtCost" />` 改为 `<el-table-column prop="cost" label="费用" width="100" align="right" :formatter="fmtCost" />`
