## 实施

### 一、布局优化

- [x] 1.1 UserDetailV2.vue 标题行内嵌 DateRangePicker，移除独立 FilterBar 行
     【目标对象】`frontend/src/views/UserDetailV2.vue`
     【修改目的】减少空间占用，筛选条件与标题同行展示
     【修改方式】修改标题行 el-card 内的模板结构，参考 RepoDetailV2.vue 第5-18行的 header 布局
     【相关依赖】`DateRangePicker.vue`（`frontend/src/components/DateRangePicker.vue`）
     【修改内容】
        - 标题行 el-card 的外层 div 已有 `justify-content: space-between`，在右侧新增一个 div，内含 `<DateRangePicker v-model="dateRange" @change="fetchData" size="small" />`
        - 删除独立的 `<FilterBar v-model:dateRange="dateRange" @search="fetchData" style="margin-top: 12px" />` 整行
        - 删除 script 中 `import FilterBar from '@/components/FilterBar.vue'`，新增 `import DateRangePicker from '@/components/DateRangePicker.vue'`
        - 保留 `dateRange` 响应式变量和 `fetchData` 函数不变

- [x] 1.2 GroupView.vue 标题行内嵌 DateRangePicker，移除独立 FilterBar 行
     【目标对象】`frontend/src/views/GroupView.vue`
     【修改目的】与 UserDetailV2 保持一致的布局风格
     【修改方式】同任务 1.1，修改标题行 el-card 内的模板结构
     【相关依赖】`DateRangePicker.vue`（`frontend/src/components/DateRangePicker.vue`）
     【修改内容】
        - 标题行 el-card 的外层 div 已有 `justify-content: space-between`，在右侧新增一个 div，内含 `<DateRangePicker v-model="dateRange" @change="fetchData" size="small" />`
        - 删除独立的 `<FilterBar v-model:dateRange="dateRange" @search="fetchData" style="margin-top: 12px" />` 整行
        - 删除 script 中 `import FilterBar from '@/components/FilterBar.vue'`，新增 `import DateRangePicker from '@/components/DateRangePicker.vue'`
        - 保留 `dateRange` 响应式变量和 `fetchData` 函数不变

---

### 二、按天明细跳转联动

- [x] 2.1 UserDetailV2.vue 按天明细 Task数/Commit数 列添加跳转
     【目标对象】`frontend/src/views/UserDetailV2.vue`
     【修改目的】点击 Task数 跳转到 /task-v2 并带上日期和用户筛选，Commit数 同理跳转 /commit-v2
     【修改方式】修改模板中 Task数/Commit数 列的渲染方式，在 script 中新增两个点击处理函数
     【相关依赖】`useRouter`（已有，第133行 `const router = useRouter()`）
     【修改内容】
        - 新增 `handleTaskClick(row)` 函数：从 `row.create_time` 截取前10位日期，去掉连字符得到 `yyyyMMdd` 格式；从 `summary.value?.user_name` 取用户名；调用 `router.push` 跳转 `/task-v2`，携带 query 参数 `startDate`、`endDate`（均为当天日期）、`userName`
        - 新增 `handleCommitClick(row)` 函数：逻辑同上，跳转路径改为 `/commit-v2`
        - 按天明细表格中 Task数 列（当前第73-75行）：将 `<template #default>` 内容改为：当 `getArrayLength(row.task_ids) > 0` 时渲染 `<el-link type="primary" @click="handleTaskClick(row)">数值</el-link>`，否则直接显示 `0`
        - Commit数 列（当前第76-78行）：同上，数值来源 `getArrayLength(row.commit_ids)`，点击调用 `handleCommitClick(row)`

- [x] 2.2 TaskViewV2.vue 从 URL query 参数初始化日期和用户筛选
     【目标对象】`frontend/src/views/TaskViewV2.vue`
     【修改目的】支持从 UserDetailV2 跳转时携带的 startDate/endDate/userName 参数自动初始化筛选
     【修改方式】修改 import 语句补充 useRoute，修改现有 onMounted 函数（第367-371行）以读取 URL query 参数
     【相关依赖】`useRoute`（vue-router；当前第85行只有 `import { useRouter } from 'vue-router'`，需补充 useRoute）
     【修改内容】
        - 修改 import 行：将 `import { useRouter } from 'vue-router'` 改为 `import { useRouter, useRoute } from 'vue-router'`，并在 router 声明后新增 `const route = useRoute()`
        - 修改 `onMounted` 函数：在调用 `filterTableRef.value?.setFilter` 之前，先读取 `route.query` 中的 `startDate`、`endDate`、`userName`；若 `startDate` 和 `endDate` 均存在，则将其从 `yyyyMMdd` 格式转换为 `yyyy-MM-dd` 格式，赋值给 `serverDateRange`
        - 日期初始化后，调用 `filterTableRef.value?.setFilter('start_time', serverDateRange)` 同步到筛选组件，再 `await fetchData()`
        - fetchData 完成后，若 `userName` 存在，调用 `filterTableRef.value?.setFilter('user_name', String(userName))` 设置用户筛选
        - 若 URL 中无 query 参数，则保持原有默认日期逻辑不变

- [x] 2.3 CommitViewV2.vue 从 URL query 参数初始化日期和用户筛选
     【目标对象】`frontend/src/views/CommitViewV2.vue`
     【修改目的】支持从 UserDetailV2 跳转时携带的 startDate/endDate/userName 参数自动初始化筛选
     【修改方式】修改 import 语句补充 useRoute，修改现有 onMounted 函数（第225-229行）以读取 URL query 参数
     【相关依赖】`useRoute`（vue-router；当前第35行只有 `import { useRouter } from 'vue-router'`，需补充 useRoute）
     【修改内容】
        - 修改 import 行：将 `import { useRouter } from 'vue-router'` 改为 `import { useRouter, useRoute } from 'vue-router'`，并在 router 声明后新增 `const route = useRoute()`
        - 修改 `onMounted` 函数：逻辑与任务 2.2 完全相同，区别在于日期筛选字段为 `commit_time`（对应 `filterTableRef.value?.setFilter('commit_time', serverDateRange)`）
        - fetchData 完成后，若 `userName` 存在，调用 `filterTableRef.value?.setFilter('user_name', String(userName))` 设置用户筛选
        - 若 URL 中无 query 参数，则保持原有默认日期逻辑不变

---

### 三、图表重设计

- [x] 3.1 UserDetailV2.vue 图表区域重写为5个独立图表
     【目标对象】`frontend/src/views/UserDetailV2.vue`
     【修改目的】提供多维度数据可视化（数量/代码量/耗时对比/费用/提效比），替换现有单一折线图
     【修改方式】将现有单图 el-card（第116-118行）替换为5个 el-card 的网格布局，每个图用独立的 useChart 实例
     【相关依赖】`useChart`（`frontend/src/composables/useChart.js`）；`useChart(containerRef)` 接受 ref 参数，需先声明 `const chartNRef = ref(null)` 再传入
     【修改内容】
        - **模板**：删除现有 `<el-card v-if="daily.length > 0" ...>` 单图卡片，替换为：前4个图用 `display:grid; grid-template-columns:1fr 1fr; gap:12px; margin-top:12px` 的网格容器包裹4个 el-card（每个内含高度280px的 div ref），第5个图单独一个全宽 el-card（`margin-top:12px`）；整体用 `v-if="daily.length > 0"` 控制显示
        - **JS — 声明**：删除旧的 `trendChartRef` 和 `const { setOption: setTrendOption } = useChart(trendChartRef)`；新增5个 ref（`chart1Ref` 至 `chart5Ref`）和对应5个 useChart 实例，每个实例解构出 `setChartNOption`（N为1-5）
        - **图1 — Task数 & Commit数（分组柱状图）**：新增 `updateChart1()` 函数；对 `daily.value` 按 `create_time` 升序排序，x轴为日期（取前10位），Task数系列数据用 `getArrayLength(d.task_ids)` 计算，Commit数系列数据用 `getArrayLength(d.commit_ids)` 计算；Task数颜色 `#409EFF`，Commit数颜色 `#67C23A`；图表标题「Task数 & Commit数」
        - **图2 — 代码行数（分组柱状图）**：新增 `updateChart2()` 函数；结构同图1，字段改为 `task_diff_lines`（蓝 `#409EFF`）和 `commit_diff_lines`（绿 `#67C23A`）；图表标题「代码行数」
        - **图3 — 耗时对比（分组柱状图，4系列）**：新增 `updateChart3()` 函数；4个系列：Task传统耗时（`task_ancient_minutes`，`#a0cfff` 浅蓝）、Task实际耗时（`task_real_minutes`，`#409EFF` 深蓝）、Commit传统耗时（`commit_ancient_minutes`，`#b3e19d` 浅绿）、Commit实际耗时（`commit_real_minutes`，`#67C23A` 深绿）；tooltip formatter 将分钟值通过 `formatDuration` 转换显示；图表标题「耗时对比」
        - **图4 — 费用（单色柱状图）**：新增 `updateChart4()` 函数；字段 `cost`，颜色 `#E6A23C`（橙）；tooltip formatter 显示4位小数（`val.toFixed(4) + ' 元'`）；图表标题「费用」
        - **图5 — 提效比趋势（双折线图）**：将现有 `updateTrendChart()` 函数改名为 `updateChart5()`，将 `setTrendOption` 替换为 `setChart5Option`，其余逻辑（字段 `task_efficiency_ratio` 蓝线、`commit_efficiency_ratio` 绿线、y轴 `'{value}%'`）保持不变
        - **统一调用**：新增 `updateCharts()` 函数，依次调用 `updateChart1()` 至 `updateChart5()`；将 `fetchData` 中原来调用 `updateTrendChart()` 的地方改为调用 `updateCharts()`

- [x] 3.2 GroupView.vue 图表区域重写为5个独立图表
     【目标对象】`frontend/src/views/GroupView.vue`
     【修改目的】与 UserDetailV2 保持一致的图表展示风格，数据来源为 daily 数组（后端 getGroupDetailV2 按日期聚合）
     【修改方式】将现有单图 el-card（第67-69行）替换为5个 el-card 的网格布局，逻辑结构与任务 3.1 相同
     【相关依赖】`useChart`（`frontend/src/composables/useChart.js`）；daily 字段与 UserDetailV2 的差异见修改内容
     【修改内容】
        - **模板**：与任务 3.1 完全相同的5图网格结构；整体用 `v-if="daily.length > 0"` 控制显示；注意5图网格应插入在原有单图位置（成员列表表格之前）
        - **JS — 声明**：删除旧的 `trendChartRef` 和 `setTrendOption`；新增5个 ref 和对应5个 useChart 实例，方式同任务 3.1
        - **日期字段差异**：GroupView 的 daily 每项用 `d.date`（字符串 `YYYY-MM-DD`，无需截取），而非 `d.create_time`；排序时也按 `d.date` 升序
        - **图1 — Task数 & Commit数**：GroupView 的 daily 是后端聚合数据，可能没有 `task_ids`/`commit_ids` 数组字段，而是直接提供 `task_count`/`commit_count` 计数字段；实现时优先使用 `d.task_count ?? 0` 和 `d.commit_count ?? 0`（不调用 `getArrayLength`，GroupView 中也无需引入该函数）
        - **图2 — 代码行数**：字段 `task_diff_lines` 和 `commit_diff_lines`，与 UserDetailV2 相同
        - **图3 — 耗时对比**：4个系列字段与 UserDetailV2 完全相同（`task_ancient_minutes`、`task_real_minutes`、`commit_ancient_minutes`、`commit_real_minutes`）；GroupView 已有 `formatDuration` 的 import（需确认，若无则补充）
        - **图4 — 费用**：字段 `cost`，与 UserDetailV2 相同
        - **图5 — 提效比趋势**：将现有 `updateTrendChart()` 改名为 `updateChart5()`，替换 `setTrendOption` 为 `setChart5Option`；日期数据源改为 `data.map(d => d.date)`（现有代码第141行已是此写法，保持不变）
        - **统一调用**：新增 `updateCharts()` 函数，依次调用5个 update 函数；将 `fetchData` 中原来调用 `updateTrendChart()` 的地方改为调用 `updateCharts()`
