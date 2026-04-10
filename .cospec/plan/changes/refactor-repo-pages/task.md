## 实施

- [x] 0.1 后端文件重命名：project_handler_v2.go → repo_handler_v2.go
     【目标对象】`backend/project_handler_v2.go`
     【修改目的】将 "project" 命名空间释放出来供后续使用，统一用 "repo" 命名后端 handler 文件
     【修改方式】重命名文件，不修改文件内部内容（函数名 listReposV2/getRepoDetailV2/listRepoBranchesV2 已经是 repo 语义，无需改动）
     【相关依赖】`backend/main.go` 行207-209 的路由注册（引用函数名不变，Go 同包内无需修改 import）
     【修改内容】
        - 将 `backend/project_handler_v2.go` 重命名为 `backend/repo_handler_v2.go`
        - 确认 Go 同包编译无需修改其他文件（函数名和包名均不变）

- [x] 0.2 前端文件重命名：ProjectViewV2.vue → RepoViewV2.vue
     【目标对象】`frontend/src/views/ProjectViewV2.vue`
     【修改目的】将 "project" 命名空间释放出来供后续使用，统一用 "repo" 命名前端视图文件
     【修改方式】重命名文件
     【相关依赖】`frontend/src/router/index.js` 行5 的 import 路径
     【修改内容】
        - 将 `frontend/src/views/ProjectViewV2.vue` 重命名为 `frontend/src/views/RepoViewV2.vue`
        - 修改 `frontend/src/router/index.js` 行5：将 `import('@/views/ProjectViewV2.vue')` 改为 `import('@/views/RepoViewV2.vue')`

- [x] 0.3 前端文件重命名：ProjectDetailV2.vue → RepoDetailV2.vue
     【目标对象】`frontend/src/views/ProjectDetailV2.vue`
     【修改目的】将 "project" 命名空间释放出来供后续使用，统一用 "repo" 命名前端视图文件
     【修改方式】重命名文件
     【相关依赖】`frontend/src/router/index.js` 行6 的 import 路径
     【修改内容】
        - 将 `frontend/src/views/ProjectDetailV2.vue` 重命名为 `frontend/src/views/RepoDetailV2.vue`
        - 修改 `frontend/src/router/index.js` 行6：将 `import('@/views/ProjectDetailV2.vue')` 改为 `import('@/views/RepoDetailV2.vue')`

- [x] 1.1 后端 `ListRepoAggregates` 增加 task_count 和 efficiency_ratio 聚合字段
     【目标对象】`backend/db.go` 的 `ListRepoAggregates` 函数（行1370-1428）
     【修改目的】在聚合查询中增加 task_count（commit 关联的 task 总数）和 efficiency_ratio（提效比），供列表页展示
     【修改方式】修改现有 SQL 查询语句、Scan 变量、item map 构建逻辑
     【相关依赖】commits 表的 task_ids JSONB 字段（可能为 NULL、"null"、"[]"）
     【修改内容】
        - SQL 增加聚合列：`SUM(CASE WHEN task_ids IS NOT NULL AND task_ids::text NOT IN ('null', '[]') THEN jsonb_array_length(task_ids) ELSE 0 END) AS task_count`
        - 行1409-1412 的 Scan 变量区增加 `var taskCount int`
        - 行1413 的 rows.Scan 调用中增加 `&taskCount`
        - 行1416-1424 的 item map 中添加 `"task_count": taskCount`
        - 在 item map 构建后（行1424附近），增加 efficiency_ratio 计算：当 sumAncient != nil 且 sumReal != nil 且 *sumReal > 0 时，计算 `(*sumAncient / *sumReal) * 100`，四舍五入到一位小数，存入 item["efficiency_ratio"]；否则 item["efficiency_ratio"] = nil

- [x] 1.2 后端 `listReposV2` handler 验证新字段透传
     【目标对象】`backend/repo_handler_v2.go`（重命名后）的 `listReposV2` 函数（行12-76）
     【修改目的】确保 handler 将 ListRepoAggregates 新增的 task_count 和 efficiency_ratio 字段正确透传给前端
     【修改方式】在现有时间格式化循环（行42-53）后补充 nil 安全处理，无需新建函数
     【相关依赖】任务 1.1 中 `ListRepoAggregates` 返回的 item map 新增字段
     【修改内容】
        - ListRepoAggregates 已在 item map 中返回 task_count 和 efficiency_ratio，handler 通过 gin.H{"data": pagedSlice} 自动序列化透传，无需额外映射
        - 在行42-53的时间格式化 for 循环中，对 efficiency_ratio 做 nil 检查：若值为 nil，保留 nil（前端会显示 "-"）；非 nil 时保留原值
        - 验证 JSON 序列化后 task_count 为整数、efficiency_ratio 为浮点数或 null

- [x] 1.3 后端 `getRepoDetailV2` 增加 reason 汇总
     【目标对象】`backend/repo_handler_v2.go`（重命名后）的 `getRepoDetailV2` 函数（行78-184）
     【修改目的】在效率评估结果中增加 reason 说明字段，将各 commit 的 reason 文本汇总为 repo 级别说明
     【修改方式】在步骤3的循环（行136-152）中收集 reason 文本，新增两个字符串变量拼接结果
     【相关依赖】`StatCommit` 结构体（行1012-1039）的 `CommitAncientMinutesReason`、`CommitAncientMinutesReasonManual`、`CommitRealMinutesReason`、`CommitRealMinutesReasonManual` 字段
     【修改内容】
        - 在行135（var repoAncientMinutes... 之前）声明两个 string slice：`var ancientReasons, realReasons []string`
        - 在行136-152的循环中，对每个 commit：
          - ancient reason 取值逻辑：manual 优先（CommitAncientMinutesReasonManual），其次 CommitAncientMinutesReason
          - real reason 取值逻辑：manual 优先（CommitRealMinutesReasonManual），其次 CommitRealMinutesReason
          - 若 reason 非 nil 且非空字符串，拼接格式 "commitID前8位: reason文本" 追加到对应 slice
        - 循环结束后，用 "; " 连接 ancientReasons 和 realReasons 生成汇总字符串
        - 在行174-178的 efficiency 响应 gin.H 中，增加两个字段：
          - `"repo_ancient_minutes_reason": strings.Join(ancientReasons, "; ")`
          - `"repo_real_minutes_reason": strings.Join(realReasons, "; ")`
        - 边界处理：若所有 commit 均无 reason，则返回空字符串 ""

- [x] 1.4 前端 Repo 列表页重写为 KbFilterTable
     【目标对象】`frontend/src/views/RepoViewV2.vue`（重命名后，全文重写）
     【修改目的】使用 KbFilterTable 组件替代 FilterBar + 手写 el-table，统一与 TaskViewV2 的交互模式，去掉图表区域
     【修改方式】参考 `frontend/src/views/TaskViewV2.vue` 的结构和模式全文重写
     【相关依赖】`frontend/src/components/KbFilterTable.vue`、`frontend/src/api/es.js` 的 `getReposV2`、`frontend/src/utils/formatters.js` 的 `formatDuration` / `fmtCost`、`frontend/src/utils/date.js` 的 `getDefaultDateRangeWide`
     【修改内容】
        - template 部分：移除 FilterBar、el-table、el-pagination、图表区域（costChartRef/tokenChartRef），替换为 `<KbFilterTable>` 组件，包含 ref/columns/data/loading/total/v-model:page/v-model:pageSize 等 props 和 row-click/size-change/page-change/filter-change 事件
        - template 中定义 efficiency_ratio 的自定义 slot（`#cell-efficiency_ratio`），使用 el-tag 显示颜色：>= 300 为 success，>= 150 为 primary，其余为 info，与 TaskViewV2 一致
        - script 部分移除旧依赖导入：FilterBar、useChart、useUrlSync、createBarOption、createDualBarOption
        - script 部分新增导入：KbFilterTable
        - 定义 columns 数组，列定义如下：
          - `repo_addr`（仓库地址）：minWidth 300，showOverflowTooltip: true，filter type `text`
          - `repo_branch`（分支）：minWidth 100，filter type `search-select`
          - `commit_count`（Commit数）：minWidth 100，align right，filter type `number`
          - `task_count`（Task数）：minWidth 100，align right，filter type `number`
          - `sum_ancient_minutes`（古法预估）：minWidth 120，align right，formatter 用 `(row, col, val) => formatDuration(val)`，filter type `number`
          - `sum_real_minutes`（实际耗时）：minWidth 120，align right，formatter 用 `(row, col, val) => formatDuration(val)`，filter type `number`
          - `efficiency_ratio`（提效比）：minWidth 110，align center，slotName: 'efficiency_ratio'，filter type `number`，shortcuts: [> 100%, > 200%, > 300%]
          - `start_time`（开始时间）：minWidth 150，filter type `date`，serverSide: true
        - 数据管理：`tableData`/`total`/`page`/`pageSize` ref 变量，pageSize 默认值 250
        - `serverDateRange` 变量（let，非 ref），初始值 `getDefaultDateRangeWide()`
        - `fetchData` 函数：将 serverDateRange 格式化为 startDate/endDate 参数，调用 getReposV2 API
        - `handleFilterChange(allFilters)`：检测 start_time 日期筛选变化时更新 serverDateRange 并重新请求（与 TaskViewV2 逻辑一致）
        - `handleRowClick(row)`：跳转 `/repo/${encodeURIComponent(row.repo_addr)}/${encodeURIComponent(row.repo_branch)}`
        - `onMounted`：通过 filterTableRef.setFilter('start_time', serverDateRange) 设置初始日期筛选，调用 fetchData
        - 移除 style 中的图表相关样式（如 kb-chart-container）

- [x] 1.5 前端 Repo 详情页效率评估增加 reason 展示
     【目标对象】`frontend/src/views/RepoDetailV2.vue`（重命名后，行19-41 效率评估卡片区域）
     【修改目的】在效率评估的古法预估和实际耗时卡片下方增加 reason 说明文字，提供评估依据的可读性
     【修改方式】在现有 metric-value div 下方各新增一个 reason 文本 div
     【相关依赖】任务 1.3 后端 `getRepoDetailV2` 返回的 `efficiency.repo_ancient_minutes_reason`、`efficiency.repo_real_minutes_reason` 字段
     【修改内容】
        - 行24（古法预估卡片 metric-value div）下方新增 div：条件渲染 `v-if="efficiency.repo_ancient_minutes_reason"`，样式为小字灰色（font-size: 12px, color: #999, margin-top: 4px），文本内容 `efficiency.repo_ancient_minutes_reason`，超长时 CSS text-overflow: ellipsis 截断，外层加 el-tooltip 显示完整内容
        - 行30（实际耗时卡片 metric-value div）下方新增 div：同上模式，显示 `efficiency.repo_real_minutes_reason`
        - 新增 scoped CSS class（如 `.metric-reason`），定义小字灰色样式：font-size 12px、color #999、margin-top 4px、overflow hidden、text-overflow ellipsis、white-space nowrap
