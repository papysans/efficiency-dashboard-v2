## 实施

- [x] 1.1 后端 listTasksV2 补充 org 字段
     【目标对象】`backend/task_handler_v2.go`
     【修改目的】在任务列表响应中附加组织信息，使前端可显示组织列并支持级联筛选
     【修改方式】在 `listTasksV2` 函数的 for 循环内（第176-226行，构建每条 item 的 `gin.H` 时），新增 org1-org4 字段赋值
     【相关依赖】`backend/org_handler_v2.go` 的 `orgMappings` 全局变量（`map[string]*OrgMapping`）和 `OrgMapping` 结构体
     【修改内容】
        - 在 item 构建逻辑中（`results[i] = item` 之前），先判断 `t.UserID != nil`，再通过 `orgMappings[*t.UserID]` 查找 OrgMapping 指针
        - 若找到映射，在 item map 中添加 `"org1": mapping.Org1`, `"org2": mapping.Org2`, `"org3": mapping.Org3`, `"org4": mapping.Org4`
        - 若 `t.UserID` 为 nil 或无映射数据，则四个字段填空字符串 `""`

- [x] 1.2 App.vue 添加 keep-alive 保持 TaskViewV2 状态
     【目标对象】`frontend/src/App.vue`
     【修改目的】从 task-v2 跳转到 task/:id 再返回时保留筛选条件和滚动位置
     【修改方式】修改第31行的 `<router-view />` 标签，改为带 v-slot 的 keep-alive 包裹形式
     【相关依赖】需配合任务 1.3 中 TaskViewV2 声明组件名称才能生效
     【修改内容】
        - 将第31行 `<router-view />` 替换为：`<router-view v-slot="{ Component }"><keep-alive :include="['TaskViewV2']"><component :is="Component" /></keep-alive></router-view>`
        - 仅 include TaskViewV2，不影响其他页面的正常销毁和重建

- [x] 1.3 TaskViewV2 声明组件名称以支持 keep-alive
     【目标对象】`frontend/src/views/TaskViewV2.vue`
     【修改目的】配合 App.vue 中 keep-alive 的 `:include="['TaskViewV2']"` 匹配，使组件能被缓存
     【修改方式】在 `<script setup>` 块顶部（第87行 import 语句之前）添加 `defineOptions` 宏调用
     【相关依赖】Vue 3.4+ 原生支持 `defineOptions`（项目 vue 版本为 ^3.4.0，无需额外插件）
     【修改内容】
        - 在 `<script setup>` 块内第一行添加 `defineOptions({ name: 'TaskViewV2' })`
        - 此宏在编译时展开，不需要 import

- [x] 1.4 KbFilterTable 新增 cascade-org 筛选类型
     【目标对象】`frontend/src/components/KbFilterTable.vue`
     【修改目的】支持在表头 popover 中显示4级组织级联筛选，供任务列表的"组织"列使用
     【修改方式】扩展 KbFilterTable 的筛选类型，在现有 text/search-select/number/enum/date 之外新增 `cascade-org` 类型，需修改模板和多个函数
     【相关依赖】`frontend/src/api/es.js` 的 `getOrgV2` 函数；级联逻辑参考 `frontend/src/views/UserViewV2.vue` 的 `loadOrgOptions`/`onOrg1Change`/`onOrg2Change`/`onOrg3Change` 实现模式
     【修改内容】
        - 导入 `getOrgV2` API
        - 在 `<script>` 中添加级联数据管理：`cascadeOrg` reactive 对象（含 org1/org2/org3/org4 的 value 和 options 数组），以及 `loadCascadeOrgOptions(level, parent)` 异步函数（调用 `getOrgV2({ level, parent })` 返回选项名称数组，参考 UserViewV2.vue 第258-270行的 `loadOrgOptions` 实现）
        - 级联联动逻辑：选择 org1 时清空 org2-4 值和选项并加载 org2 选项；选择 org2 时清空 org3-4 并加载 org3 选项（parent 为 `org1/org2` 拼接）；选择 org3 时清空 org4 并加载 org4 选项（parent 为 `org1/org2/org3` 拼接）
        - **模板**：在表头 popover 的 date 类型模板块之后（约第215行）、`</el-popover>` 之前，增加 `cascade-org` 类型的 UI 面板：宽度400px的 popover，内含4个 `el-select`（一级组织→四级组织），竖向排列，每级 clearable，下级在上级未选时 disabled
        - **模板**：在条件栏 tag 弹出编辑面板（虚拟定位 popover，约第36-84行）的 date 类型之后，增加 `cascade-org` 类型的编辑 UI（同上述4个级联 select）
        - **`getPopoverWidth` 函数**（第337行）：增加 `if (col.filter.type === 'cascade-org') return 400`
        - **`onPopoverShow` 函数**（第379行）：增加 `cascade-org` 分支，从 `filters[col.prop]` 读取 `{ org1, org2, org3, org4 }` 对象恢复到 `cascadeOrg` 的 value 中，并加载对应层级的 options
        - **`applyFilter` 函数**（第420行）：增加 `cascade-org` 分支，将 `cascadeOrg` 的 org1-4 value 打包为 `{ org1, org2, org3, org4 }` 对象写入 `filters[col.prop]`；若四个值均为空则 delete
        - **`resetFilter` 函数**（第452行）：增加 `cascade-org` 分支，清空 `cascadeOrg` 所有 value 和 options，delete `filters[col.prop]`
        - **`setFilter` 函数**（第491行）：增加 `cascade-org` 分支，接受 `{ org1, org2, org3, org4 }` 对象写入 `filters[prop]`
        - **`filteredData` computed**（第521行）：增加 `cascade-org` 类型的过滤逻辑，从 `filters[col.prop]` 取出 `{ org1, org2, org3, org4 }` 对象，对每级有值的字段匹配 `row.org1`/`row.org2`/`row.org3`/`row.org4`，全部匹配才保留
        - **`activeFilterTags` computed**（第575行）：增加 `cascade-org` 分支，判断 `filters[col.prop]` 为对象且至少有一个 orgN 有值时，display 为各级有值的 org 用 `/` 拼接

- [x] 1.5 TaskViewV2 增加组织列定义
     【目标对象】`frontend/src/views/TaskViewV2.vue`
     【修改目的】在任务表格中显示组织信息并支持级联筛选
     【修改方式】在 columns 数组中 `user_name` 列定义（第130-136行）之后插入新的列对象
     【相关依赖】依赖任务 1.1 后端提供 org1-org4 字段；依赖任务 1.4 KbFilterTable 支持 `cascade-org` 筛选类型
     【修改内容】
        - 在 `user_name` 列定义之后、`title` 列之前插入新列对象：prop 为 `'org_display'`，label 为 `'组织'`，minWidth 160，showOverflowTooltip true
        - formatter 函数：`(row) => [row.org1, row.org2, row.org3, row.org4].filter(Boolean).join('/') || '-'`
        - filter 配置：`{ type: 'cascade-org' }`
        - 注意：`cascade-org` 是客户端筛选，不设置 `serverSide`，因此 `handleFilterChange` 无需额外处理

- [x] 1.6 TaskViewV2 增加报表按钮
     【目标对象】`frontend/src/views/TaskViewV2.vue`
     【修改目的】在操作栏添加"组织报表""用户报表""时间报表"按钮，用户报表跳转新页面并携带筛选条件
     【修改方式】修改 template 中 `#actions` 插槽内容（第30-35行），并在 `<script setup>` 中新增 `goUserReport` 函数
     【相关依赖】`vue-router` 的 `useRouter`（已导入）；`ElMessage`（已导入）；KbFilterTable 的 `getFilter` 方法（已通过 `filterTableRef` expose）
     【修改内容】
        - 在 template `#actions` 插槽中，在"已选 N 个 Task"文字之前添加3个按钮：
          - `<el-button size="small" @click="ElMessage.info('敬请期待')">组织报表</el-button>`
          - `<el-button size="small" type="success" @click="goUserReport">用户报表</el-button>`
          - `<el-button size="small" @click="ElMessage.info('敬请期待')">时间报表</el-button>`
        - 新增 `goUserReport` 函数：
          - 从 `serverDateRange` 获取 startDate/endDate（已在组件中定义，转换为 yyyyMMdd 格式）
          - 从 `filterTableRef.value?.getFilter('org_display')` 获取级联筛选值（返回 `{ org1, org2, org3, org4 }` 对象或 null）
          - 构建 query 对象：`{ startDate, endDate, org1, org2, org3, org4 }`（过滤掉空值）
          - 调用 `router.push({ path: '/task-v2/report/user', query })`
        - 注意 actions 插槽可见性：当前 KbFilterTable 第4行条件为 `activeFilterTags.length > 0 || $slots.actions`，actions 插槽有内容即会显示条件栏，报表按钮始终存在因此条件栏始终可见，无需额外调整

- [x] 1.7 新增用户报表路由
     【目标对象】`frontend/src/router/index.js`
     【修改目的】注册用户报表页面路由，使 `/task-v2/report/user` 可访问
     【修改方式】在 routes 数组中 task-v2 路由（第13行）之后新增一条路由配置
     【相关依赖】`frontend/src/views/TaskUserReport.vue`（任务 1.8 新建）
     【修改内容】
        - 在第13行 `{ path: '/task-v2', ... }` 之后添加：`{ path: '/task-v2/report/user', name: 'TaskUserReport', component: () => import('@/views/TaskUserReport.vue') }`
        - 注意：此路由必须在 `/task-v2` 之后、`/task/:taskId` 之前，避免路径被 `:taskId` 参数捕获

- [x] 1.8 创建用户报表页面
     【目标对象】`frontend/src/views/TaskUserReport.vue`（新建）
     【修改目的】以用户为维度展示 Task 报表分析面板，包含汇总指标卡和6个图表
     【修改方式】新建 Vue SFC 组件，参考 `GroupView.vue` 的 dashboard 布局模式 和 `UserViewV2.vue` 的级联筛选模式
     【相关依赖】
        - `frontend/src/api/es.js` 的 `getUsersV2`（获取用户聚合数据）, `getOrgV2`（加载组织级联选项）
        - `frontend/src/composables/useChart.js` 的 `useChart`（ECharts 生命周期管理）
        - `frontend/src/utils/chart.js` 的 `createBarOption`（横向柱状图）, `createDualBarOption`（双柱对比图）
        - `frontend/src/utils/formatters.js` 的 `fmtCost`（费用格式化，4位小数）, `formatDuration`（时长格式化，分钟→人天）
        - `frontend/src/components/FilterBar.vue`（筛选区容器）
        - `frontend/src/components/DateRangePicker.vue`（日期范围选择，已由 FilterBar 内部引入）
     【修改内容】
        - **筛选区**：使用 FilterBar 组件（`v-model:dateRange` + `@search`），在默认插槽中放置4个 `el-select`（一级→四级组织，竖向级联联动，参考 UserViewV2.vue 第4-17行模板和第258-304行级联逻辑），在 FilterBar 左侧额外放置返回按钮（`@click="router.back()"`）
        - **级联数据**：声明 `filterOrg1/2/3/4` ref、`org1/2/3/4Options` ref、`loadOrgOptions(level, parent)` 异步函数（调用 `getOrgV2({ level, parent, startDate, endDate })`，返回 `(data.data || []).map(d => d.org_name)`）、`onOrg1Change`/`onOrg2Change`/`onOrg3Change` 联动函数（清空下级并加载下级选项，与 UserViewV2.vue 完全相同的模式）
        - **onMounted**：从 `route.query` 读取 startDate/endDate（yyyyMMdd 格式，转为 yyyy-MM-dd 赋给 dateRange）和 org1-org4（赋给 filterOrgN），加载 org1 选项（调用 `loadOrgOptions('org1', '')`），若有 org1 值则依次加载 org2/org3/org4 选项，最后调用 fetchData
        - **fetchData**：调用 `getUsersV2({ startDate, endDate, org1, org2, org3, org4, pageSize: 9999 })`（startDate/endDate 为 yyyyMMdd 格式），从返回的 `data.data` 数组中过滤掉 `is_virtual_group === true` 的记录
        - **汇总指标卡**（1行6个 `el-col :span="4"`，参考 GroupView.vue 第15-65行的 `kb-metric-card` 布局）：
          - 总Task数：`users.reduce((s, u) => s + (u.task_count || 0), 0)`
          - 总代码行数：`sum(task_diff_lines)`
          - 总传统耗时：`sum(task_ancient_minutes)`，用 `formatDuration` 显示
          - 总实际耗时：`sum(task_real_minutes)`，用 `formatDuration` 显示
          - 总费用：`sum(cost)`，显示为 `$` + 4位小数
          - 平均提效比：`totalAncient / totalReal * 100`，当 totalReal > 0 时显示为 `xx.x%`，否则显示 `-`
        - **6个图表数据准备**：将过滤后的用户数组转换为 `createBarOption` 所需的 `Array<{name: string, value: number}>` 格式，其中 name 为 `user_name`，value 为对应指标值。双柱图需分别构建 ancientData（`{name: user_name, value: task_ancient_minutes}`）和 realData（`{name: user_name, value: task_real_minutes}`）两个数组
        - **6个图表**（3行2列 grid 布局，每行2个 `el-col :span="12"`）：
          a) Task数 - `createBarOption('Task数（按用户）', taskCountData, '#409EFF')`
          b) 代码行数 - `createBarOption('代码行数（按用户）', diffLinesData, '#67C23A')`
          c) 传统耗时 vs 实际耗时 - `createDualBarOption('耗时对比（按用户）', ancientData, realData, '传统耗时', '实际耗时', '#E6A23C', '#409EFF')`
          d) 费用 - `createBarOption('费用（按用户）', costData, '#E6A23C', v => Number(v).toFixed(4))`
          e) Token消耗 - `createBarOption('Token消耗（按用户）', tokenData, '#909399', v => Number(v).toLocaleString())`
          f) 提效比 - `createBarOption('Task提效比（按用户）', ratioData, '#67C23A', v => v.toFixed(1) + '%')`
        - **每个图表**声明 `ref(null)` 容器引用，调用 `useChart(chartNRef)` 获取 `setOption` 函数（参考 GroupView.vue 第136-140行），在 fetchData 成功后调用各 `setOption(createBarOption(...))`
        - **样式**：外层使用 `kb-panel` class 容器；图表容器使用 `kb-chart-container` class（`height: 280px`），参考 GroupView.vue 的样式定义
