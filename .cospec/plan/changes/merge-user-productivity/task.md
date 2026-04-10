## 实施

- [x] 1.1 重写后端 listUsersV2，改为从 user_productivity 表聚合
     【目标对象】`backend/user_handler_v2.go` → `listUsersV2` 函数（当前第15行，已有初步实现需核查并完善）
     【修改目的】将用户列表 API 的数据源从 tasks+commits 双表实时聚合改为从 user_productivity 预聚合表读取，统一数据源
     【修改方式】重写 `listUsersV2` 函数（完整替换函数体），直接在函数内构造 SQL，不依赖 db.go 的 ORM 封装（参考现有 listUserProductivitySummary 的直接 SQL 模式）
     【相关依赖】`statDB` 全局变量（`backend/main.go` 第54行）；`parseDateParam` 工具函数；`getDefaultInt` 工具函数；`DefaultPageSize` 常量
     【修改内容】
        - 函数签名保持 `func listUsersV2(c *gin.Context)`
        - 解析 startDate/endDate 查询参数（YYYYMMDD 格式）：调用 `parseDateParam`，endDate 转换时需 `+23:59:59`，最终转为 RFC3339 格式
        - 构造 SQL：`SELECT user_id, COALESCE(MAX(user_name), '') as user_name, COUNT(*) as day_count, COALESCE(SUM(jsonb_array_length(task_ids)), 0) as task_count, COALESCE(SUM(jsonb_array_length(commit_ids)), 0) as commit_count, COALESCE(SUM(task_diff_lines), 0) as task_diff_lines, COALESCE(SUM(commit_diff_lines), 0) as commit_diff_lines, COALESCE(SUM(upstream_tokens), 0) as upstream_tokens, COALESCE(SUM(downstream_tokens), 0) as downstream_tokens, COALESCE(SUM(cost), 0) as cost, COALESCE(SUM(task_real_minutes), 0) as task_real_minutes, COALESCE(SUM(task_ancient_minutes), 0) as task_ancient_minutes, COALESCE(SUM(commit_real_minutes), 0) as commit_real_minutes, COALESCE(SUM(commit_ancient_minutes), 0) as commit_ancient_minutes FROM user_productivity [WHERE ...] GROUP BY user_id`
        - WHERE 条件通过 `$1`, `$2` 占位符动态拼接（有日期参数时加 `create_time >= $N` / `create_time <= $N`）
        - 扫描结果后在 Go 中计算效率比：`taskEffRatio = math.Round(taskAncientMin / taskRealMin * 100)`（仅当 taskRealMin > 0），commitEffRatio 同理
        - 返回数组每项包含：user_id, user_name, day_count, task_count, commit_count, task_diff_lines, commit_diff_lines, upstream_tokens, downstream_tokens, cost, task_real_minutes, task_ancient_minutes, task_efficiency_ratio, commit_real_minutes, commit_ancient_minutes, commit_efficiency_ratio
        - 内存分页：`getDefaultInt(c, "page", 1)` / `getDefaultInt(c, "pageSize", DefaultPageSize)`，切片后返回
        - 返回格式：`gin.H{"total": total, "page": page, "pageSize": pageSize, "data": pagedSlice}`
        - 错误处理：SQL 查询失败、rows.Scan 失败、rows.Err() 均返回 500 + 中文错误信息（参考现有风格：`gin.H{"error": "查询 user_productivity 聚合失败: " + err.Error()}`）

- [x] 1.2 重写后端 getUserDetailV2，改为返回 user_productivity 按天明细
     【目标对象】`backend/user_handler_v2.go` → `getUserDetailV2` 函数（当前第150行，已有初步实现需核查并完善）
     【修改目的】将用户详情 API 改为返回 user_productivity 按天明细 + 汇总数据，替代原来的全量 tasks/commits 列表
     【修改方式】重写 `getUserDetailV2` 函数（完整替换函数体），调用 `ListUserProductivity` 获取按天明细，在 Go 中聚合汇总
     【相关依赖】`backend/db.go` 中的 `ListUserProductivity(db *sql.DB, userID, startTime, endTime string, page, pageSize int) ([]UserProductivity, error)` 函数；`statDB` 全局变量；`parseDateParam` 工具函数；`encoding/json` 包（用于 json.Unmarshal 统计 task_ids/commit_ids 数量）
     【修改内容】
        - 函数签名保持 `func getUserDetailV2(c *gin.Context)`
        - 从路径参数获取 `userId := c.Param("userId")`
        - 解析 startDate/endDate 可选查询参数（同 1.1 方式，endDate 需 +23:59:59）
        - 调用 `ListUserProductivity(statDB, userID, startTime, endTime, 1, 10000)` 获取按天明细（按天最多不超过 10000 条）
        - 在 Go 中遍历 daily 数组计算汇总：dayCount = len(daily)；对每条记录 json.Unmarshal task_ids/commit_ids 数组长度累加到 taskCount/commitCount；累加各 float64/int/int64 字段（需判断指针非 nil）；从 daily[0].UserName 提取 userName（若 daily 为空则为空字符串）
        - 计算效率比：taskEffRatio（仅当 taskRealMin > 0）、commitEffRatio（仅当 commitRealMin > 0），使用 `math.Round`
        - 构造 summary：包含 user_id, user_name, day_count, task_count, commit_count, task_diff_lines, commit_diff_lines, upstream_tokens, downstream_tokens, cost, task_real_minutes, task_ancient_minutes, task_efficiency_ratio, commit_real_minutes, commit_ancient_minutes, commit_efficiency_ratio
        - 返回格式：`gin.H{"summary": summary, "daily": daily, "total": dayCount}`
        - 错误处理：ListUserProductivity 失败返回 500 + 中文错误信息

- [x] 1.3 清理后端路由，移除独立的 user-productivity 列表/详情路由，迁移 rebuild 路由
     【目标对象】`backend/main.go` → `main` 函数内 V2 路由组注册部分（第228-231行）
     【修改目的】移除已合并到 users API 的独立路由，将 rebuild 路由从 `/user-productivity/rebuild` 迁移到 `/users/rebuild`
     【修改方式】修改 `main` 函数中 `v2` 路由组的路由注册代码（新增、修改、删除路由条目）
     【相关依赖】1.1 和 1.2 的 handler（`listUsersV2`、`getUserDetailV2` 已在 user_handler_v2.go 中实现）；`rebuildUserProductivity` handler（保留在 user_productivity_handler_v2.go 中）
     【修改内容】
        - 删除第230行：`v2.GET("/user-productivity", listUserProductivitySummary)`（已由 listUsersV2 替代）
        - 删除第231行：`v2.GET("/user-productivity/:userId", getUserProductivityDetail)`（已由 getUserDetailV2 替代）
        - 将第229行 `v2.POST("/user-productivity/rebuild", rebuildUserProductivity)` 修改为 `v2.POST("/users/rebuild", rebuildUserProductivity)`（handler 函数名不变，只改路由路径）
        - 已有的 `v2.GET("/users", listUsersV2)` 和 `v2.GET("/users/:userId", getUserDetailV2)` 保持不变
        - `v2.POST("/user-groups", ...)` 等 user-groups 路由保持不变
        - 注意：`/users/rebuild` 须注册在 `/users/:userId` 之前，否则 rebuild 会被 :userId 动态路由拦截

- [x] 1.4 清理后端 user_productivity_handler_v2.go 中已合并的函数
     【目标对象】`backend/user_productivity_handler_v2.go` → `listUserProductivitySummary` 函数（第237-371行）和 `getUserProductivityDetail` 函数（第374-488行）
     【修改目的】移除已被 user_handler_v2.go 接管的列表和详情 handler，避免重复代码
     【修改方式】删除 `listUserProductivitySummary` 和 `getUserProductivityDetail` 两个函数的完整定义（含函数注释），保留 `rebuildUserProductivity` 函数（第16-234行）
     【相关依赖】1.3 中路由已移除对这两个函数的引用，删除后不会编译报错；`rebuildUserProductivity` 函数仍使用 `pq`、`fmt`、`strings` 等 import，需确认删除后 import 列表仍有效
     【修改内容】
        - 删除 `// listUserProductivitySummary GET /api/v2/user-productivity` 注释及 `listUserProductivitySummary` 函数完整定义（第236-371行）
        - 删除 `// getUserProductivityDetail GET /api/v2/user-productivity/:userId` 注释及 `getUserProductivityDetail` 函数完整定义（第373-488行）
        - 保留文件顶部 `package main` 及 import 块（`fmt`、`strings`、`encoding/json`、`math`、`net/http`、`time`、`gin`、`pq` 均被 `rebuildUserProductivity` 使用，无需改动）
        - 保留 `rebuildUserProductivity` 函数完整不变

- [x] 1.5 重写 UserViewV2.vue，参考 TaskViewV2 使用 KbFilterTable
     【目标对象】`frontend/src/views/UserViewV2.vue`（完整重写）
     【修改目的】重写用户列表页，统一使用 KbFilterTable 组件展示生产力汇总数据（含效率比、费用等），并新增 rebuild 按钮和虚拟组管理功能
     【修改方式】完全重写文件，使用 `<script setup>` + KbFilterTable 组件（参考 TaskViewV2.vue 的 columns 数组定义模式和 handleFilterChange 处理 serverSide 日期筛选的方式）
     【相关依赖】`frontend/src/components/KbFilterTable.vue`；`frontend/src/composables/useChart.js`；`frontend/src/utils/chart.js` 的 `createBarOption`；`frontend/src/utils/formatters.js` 的 `fmtCost`；`frontend/src/utils/date.js` 的 `getDefaultDateRangeWide`；`frontend/src/api/es.js` 的 `getUsersV2`、`rebuildUsersV2`（1.9 新增）、`createUserGroup`、`getUserGroups`、`deleteUserGroup`
     【修改内容】
        - 顶部操作栏（flex 布局，margin-bottom: 8px）：
          - 左侧：el-button type="primary" 调用 `rebuildUsersV2`，显示 loading 状态，按钮文字"刷新数据"
          - 右侧（showSelection 为 true 时显示）：`已选 X 个用户` 文字 + "创建虚拟组" el-button，点击弹出 el-dialog（输入组名 → 调用 `createUserGroup({ name, member_ids: selectedUsers.map(u => u.user_id) })`）
        - KbFilterTable 配置（参考 TaskViewV2 columns 数组写法）：
          - columns 列定义：user_name（filter: {type:'text'}）、user_id（filter: {type:'text'}）、day_count（filter: {type:'number'}）、task_count（filter: {type:'number'}）、commit_count（filter: {type:'number'}）、task_diff_lines（label:'Task代码行数'，filter: {type:'number'}）、commit_diff_lines（label:'Commit代码行数'，filter: {type:'number'}）、task_efficiency_ratio（slotName: 'task_efficiency_ratio'，filter: {type:'number'}）、commit_efficiency_ratio（slotName: 'commit_efficiency_ratio'，filter: {type:'number'}）、cost（formatter: fmtCost，filter: {type:'number'}）、_tokens（label:'Tokens消耗'，valueGetter: row => row.upstream_tokens + row.downstream_tokens，filter: {type:'number'}）
          - date serverSide filter：绑定 prop `create_time`，filter: {type:'date', serverSide: true}；`handleFilterChange` 中检测 `allFilters.create_time` 变化时重新请求 API（同 TaskViewV2 的模式）
          - 效率比 slot（参考 UserGroupDetail.vue 的 el-tag 模式）：`>=300 type="success"`、`>=150 type="primary"`、否则 `type="info"`，显示 `xx.x%`
          - :show-selection="true"，@selection-change 更新 selectedUsers ref
          - @row-click 调用 `router.push('/user/' + row.user_id)`
          - @filter-change="handleFilterChange"，@page-change / @size-change 触发 fetchData
        - fetchData 函数：从 serverDateRange 构造 startDate/endDate（YYYYMMDD 格式），调用 `getUsersV2(params)`，更新 tableData/total
        - 虚拟组列表区域（KbFilterTable 之后）：调用 `getUserGroups()` 获取列表，el-table 展示组名+成员数+删除按钮；行点击跳转 `/user/group/` + g.group_id；删除调用 `deleteUserGroup(groupId)` 后刷新列表
        - 底部提效比分布柱状图：`useChart` + `createBarOption`，数据为 `tableData.value.map(d => ({ name: d.user_name || d.user_id, value: d.task_efficiency_ratio || 0 }))`
        - onMounted：初始化 serverDateRange = getDefaultDateRangeWide()，同步到 KbFilterTable 的 date filter（参考 TaskViewV2 的 filterTableRef.value?.setFilter）后调用 fetchData

- [x] 1.6 重写 UserDetailV2.vue，合并按天明细和汇总卡片
     【目标对象】`frontend/src/views/UserDetailV2.vue`（完整重写）
     【修改目的】重写用户详情页，合并 UserProductivityDetail 的汇总卡片 + 按天明细表格 + 趋势图能力，数据源改为 getUserDetailV2（现在返回 {summary, daily, total}）
     【修改方式】完全重写文件，使用 `<script setup>`，整体结构参考现有 UserGroupDetail.vue（标题栏+汇总卡片+表格+图表）
     【相关依赖】`frontend/src/composables/useChart.js`；`frontend/src/utils/formatters.js` 的 `fmtCost`、`formatDuration`；`frontend/src/utils/date.js` 的 `getDefaultDateRangeWide`；`frontend/src/api/es.js` 的 `getUserDetailV2`（1.9 中更新签名后支持 params）
     【修改内容】
        - 标题栏：el-button 返回（`router.back()`）+ `用户详情: {{ summary?.user_name || '-' }}`
        - 日期范围筛选：el-date-picker（v-model 绑定 dateRange，初始化为 getDefaultDateRangeWide()），@change 触发 fetchData；调用时将 dateRange 转换为 startDate/endDate（YYYYMMDD 格式）传给 `getUserDetailV2(userId, { startDate, endDate })`
        - 顶部汇总卡片（el-row + el-col，class="kb-metric-card"，参考 UserGroupDetail.vue 的卡片样式）：
          总活跃天数(summary.day_count)、总Task数(summary.task_count)、总Commit数(summary.commit_count)、Task提效比（el-tag 颜色：>=300 success / >=150 primary / else info，显示 summary.task_efficiency_ratio.toFixed(1)+'%'）、Commit提效比（同上，用 summary.commit_efficiency_ratio）、总费用（fmtCost 格式化 summary.cost）
        - 中间按天明细表格（el-table，:data="daily" 数组，stripe border）：
          列：日期（prop: create_time，formatter: val => val?.substring(0,10)）、Task数（prop: task_count，通过后端返回字段，若后端返回 task_ids jsonb 则前端用 row.task_ids?.length 或直接展示后端计算好的数值）、Commit数（prop: commit_count 同理）、Task代码行数(task_diff_lines)、Commit代码行数(commit_diff_lines)、Task实际耗时(formatter: formatDuration(row.task_real_minutes))、Task传统耗时(formatter: formatDuration(row.task_ancient_minutes))、Task提效比(el-tag 颜色渲染)、Commit实际耗时(commit_real_minutes)、Commit传统耗时(commit_ancient_minutes)、Commit提效比(el-tag 颜色渲染)、费用(fmtCost)
          注意：daily 数组中每条 UserProductivity 记录的 task_count/commit_count 字段由 ListUserProductivity 返回的是 jsonb 原始数据，前端需通过 row.task_ids 字段（Array 类型）取 .length，或后端在 ListUserProductivity 中已转换（需查看 db.go 的 UserProductivity 结构体）
        - 底部趋势图（useChart 双折线）：xAxis 为 `daily.map(d => d.create_time?.substring(0,10))`，series: [Task提效比按天、Commit提效比按天]，数值分别取 daily[i].task_efficiency_ratio / daily[i].commit_efficiency_ratio

- [x] 1.7 更新 UserGroupDetail.vue 路由跳转
     【目标对象】`frontend/src/views/UserGroupDetail.vue` → `handleRowClick` 函数（第125-127行）和 `handleDelete` 函数（第129-145行）
     【修改目的】将虚拟组详情页的内部跳转链接从 `/productivity/xxx` 改为 `/user/xxx`，删除后返回路径从 `/productivity` 改为 `/user-v2`
     【修改方式】修改两处 router.push 调用（小范围修改，不改其他逻辑）
     【相关依赖】路由 `/user/:userId`（router/index.js 第8行）和 `/user-v2`（router/index.js 第7行）
     【修改内容】
        - 第126行 `handleRowClick`：将 `router.push('/productivity/' + row.user_id)` 改为 `router.push('/user/' + row.user_id)`
        - 第139行 `handleDelete` 成功后：将 `router.push('/productivity')` 改为 `router.push('/user-v2')`
        - 其余代码保持不变（包括第7行 `router.back()` 返回按钮，保留原逻辑）

- [x] 1.8 更新前端路由和导航菜单
     【目标对象】`frontend/src/router/index.js`（第18-20行）和 `frontend/src/App.vue`（第22行）
     【修改目的】移除 productivity 相关路由，新增 `/user/group/:groupId` 虚拟组路由，移除导航菜单"生产力"项
     【修改方式】修改路由数组（删除条目、新增条目），修改 App.vue 菜单列表（删除条目）
     【相关依赖】1.5~1.7 的页面组件；`UserGroupDetail.vue` 已存在
     【修改内容】
        - router/index.js：删除第18行 `{ path: '/productivity', ... }`
        - router/index.js：删除第19行 `{ path: '/productivity/group/:groupId', ... }`
        - router/index.js：删除第20行 `{ path: '/productivity/:userId', ... }`
        - router/index.js：在 `/user/:userId`（第8行）之前新增 `{ path: '/user/group/:groupId', name: 'UserGroupDetail', component: () => import('@/views/UserGroupDetail.vue') }`（必须在 `/user/:userId` 前面，避免动态段拦截）
        - App.vue：删除第22行 `<el-menu-item index="/productivity">生产力</el-menu-item>`

- [x] 1.9 更新前端 API 函数
     【目标对象】`frontend/src/api/es.js` → `getUserDetailV2` 函数（第127-129行）、`rebuildUserProductivity` / `getUserProductivityList` / `getUserProductivityDetail`（第184-186行）
     【修改目的】更新 API 函数签名以支持日期参数，将 rebuild 路由从 `/v2/user-productivity/rebuild` 改为 `/v2/users/rebuild`，移除已废弃的 user-productivity 列表/详情函数
     【修改方式】修改现有函数签名和 URL，新增 `rebuildUsersV2` 函数，删除废弃函数
     【相关依赖】后端路由变更（1.3）；1.5 中调用 `rebuildUsersV2`；1.6 中调用 `getUserDetailV2(userId, params)`
     【修改内容】
        - 修改第127-129行 `getUserDetailV2`：将函数签名从 `getUserDetailV2(userId)` 改为 `getUserDetailV2(userId, params)`，URL 保持 `/v2/users/${userId}`，添加 params 传参：`request({ url: \`/v2/users/${userId}\`, method: 'get', params })`
        - 在第184行之前（User Productivity API 注释块中）新增：`export const rebuildUsersV2 = (params) => request({ url: '/v2/users/rebuild', method: 'post', params })`
        - 删除第184行：`export const rebuildUserProductivity = (params) => request({ url: '/v2/user-productivity/rebuild', method: 'post', params })`
        - 删除第185行：`export const getUserProductivityList = (params) => request({ url: '/v2/user-productivity', method: 'get', params })`
        - 删除第186行：`export const getUserProductivityDetail = (userId, params) => request({ url: \`/v2/user-productivity/${userId}\`, method: 'get', params })`
        - 保留第188-192行 `createUserGroup`/`getUserGroups`/`deleteUserGroup`/`getUserGroupDetail` 函数不变
        - `getUsersV2(params)`（第123-125行）保持不变

- [x] 1.10 删除废弃的前端页面文件
     【目标对象】`frontend/src/views/UserProductivityView.vue` 和 `frontend/src/views/UserProductivityDetail.vue`
     【修改目的】移除已被 UserViewV2.vue 和 UserDetailV2.vue 接管的独立生产力页面，消除死代码
     【修改方式】删除这两个文件（执行前确认 router/index.js 中对这两个文件的 import 引用已在 1.8 中移除）
     【相关依赖】1.8 中路由已移除对这两个文件的懒加载引用
     【修改内容】
        - 删除 `frontend/src/views/UserProductivityView.vue`
        - 删除 `frontend/src/views/UserProductivityDetail.vue`
        - 执行方式：`Remove-Item -Path "frontend/src/views/UserProductivityView.vue" -Force` 和 `Remove-Item -Path "frontend/src/views/UserProductivityDetail.vue" -Force`
