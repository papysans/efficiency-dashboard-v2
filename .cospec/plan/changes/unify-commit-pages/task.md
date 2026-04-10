## 实施

- [x] 1.1 重构 CommitDetailV2.vue 布局为基础信息+度量信息双卡片结构
     【目标对象】`frontend/src/views/CommitDetailV2.vue`
     【修改目的】将基础信息和度量信息拆分为两个独立的 el-card，去掉提效指标大卡片，与 TaskDetailV2 保持一致的设计风格
     【修改方式】修改 template 中现有的 el-card + el-descriptions 结构，删除提效指标 el-row 区块，修改 script 中 `commitEfficiencyColor` computed
     【相关依赖】`frontend/src/views/TaskDetailV2.vue` 作为参照模板（第12-96行的基础信息+度量信息双卡片结构）
     【修改内容】
        - 将现有第2个 el-card（第13-67行）中的 el-descriptions 拆分为两个 el-card：
          * 基础信息卡片（el-card header="基础信息"）保留：Commit ID、用户、Git 用户<email>、仓库（保留现有 el-link 跳转）、分支、提交时间
          * 其中「用户」字段改为 el-link，点击跳转到 `/user/:userId`（参照 TaskDetailV2 第17-18行：有 user_id 时显示 el-link，否则显示纯文本）
        - 新增度量信息卡片（el-card header="度量信息"），包含新的 el-descriptions：
          * Diff行数（原样迁移）
          * 古法预估（保留现有 manual 优先 + tooltip + 删除线逻辑，原样迁移第26-43行的 template）
          * 实际耗时（保留现有 manual 优先 + tooltip + 删除线逻辑，原样迁移第46-63行的 template）
          * 提效比：使用彩色大字样式（参照 TaskDetailV2 第91-93行：span + :style="{ color: efficiencyColor, fontSize: '20px', fontWeight: 'bold' }"）
        - 删除整个「提效指标卡片」区块（第69-141行的 el-row 含3个 el-col 大卡片）
        - 将 `commitEfficiencyColor` computed（第204-210行）重命名为 `efficiencyColor`，逻辑不变
        - 删除 scoped style 中的 `.dashboard-metric-card`、`.metric-label`、`.metric-value` 相关 CSS（第277-293行），这些样式仅被删除的大卡片使用

- [x] 1.2 用 KbFilterTable 重写 CommitViewV2.vue 列表页
     【目标对象】`frontend/src/views/CommitViewV2.vue`
     【修改目的】复用 KbFilterTable 组件替换 FilterBar+el-table+echarts 图表，与 TaskViewV2 保持一致的交互体验
     【修改方式】重写整个文件的 template 和 script 部分，参照 TaskViewV2.vue 的完整实现模式
     【相关依赖】`frontend/src/components/KbFilterTable.vue`、`frontend/src/views/TaskViewV2.vue` 作为参照模板、`frontend/src/api/es.js` 的 `getCommitsV2`
     【修改内容】
        template 部分：
        - 删除 FilterBar 组件及其 dateRange 绑定（第4-6行）
        - 删除汇总指标卡片区域（第8-36行的 el-row 含4个 el-col 大卡片）
        - 删除原生 el-table 及分页（第39-87行），替换为 KbFilterTable 组件
        - 删除 echarts 图表区域（第89-103行的 kb-charts-area）
        - KbFilterTable 绑定方式参照 TaskViewV2 第15-27行：传入 columns、data、loading、total、v-model:page、v-model:pageSize，监听 row-click、size-change、page-change、filter-change 事件
        - 添加 efficiency_ratio 列的自定义 slot（参照 TaskViewV2 第28-37行的 `#cell-efficiency_ratio` slot，用 el-tag 渲染，>=300 success / >=150 primary / 其他 info）
        script 部分 — 删除以下内容：
        - 删除 FilterBar 组件导入（第110行）
        - 删除 useChart、useUrlSync 导入及其实例化（第111-112行、第165-174行）
        - 删除 createBarOption、createDualBarOption 导入（第116行）
        - 删除 searchKeyword ref 及 filteredCommits computed（第141-149行）
        - 删除 totalAncientMinutes、totalRealMinutes、avgEfficiencyRatio computed（第151-162行）
        - 删除 sortByEfficiency、fmtDuration 函数（第120-128行）
        - 删除 updateOverviewCharts 函数（第212-229行）
        - 删除 syncToUrl、restoreFromUrl 相关（第165-167行、第234行、第248行）
        - 删除 diffChartRef、ancientChartRef ref（第170-171行）
        script 部分 — 新增/修改以下内容：
        - 新增 KbFilterTable 组件导入，新增 filterTableRef ref
        - 新增 formatDuration 导入（从 `@/utils/formatters`）
        - 新增两个 manual 优先取值辅助函数（参照 TaskViewV2 第55-67行模式）：
          * `getEffectiveAncient(row)` 返回 `row.commit_ancient_minutes_manual ?? row.commit_ancient_minutes ?? null`
          * `getEffectiveReal(row)` 返回 `row.commit_real_minutes_manual ?? row.commit_real_minutes ?? null`
        - 定义 columns 数组（参照 TaskViewV2 第70-181行模式）：
          * user_name: label "用户", filter type search-select
          * repo_addr: label "仓库", showOverflowTooltip: true, filter type text
          * commit_time: label "提交时间", formatter 截取前16字符显示, filter type date + serverSide: true
          * diff_lines: label "Diff行数", align right, filter type number, shortcuts: >0(min:1), >50(min:50), >200(min:200)
          * commit_ancient_minutes: label "古法预估", align right, sortMethod 使用 getEffectiveAncient, formatter 调用 formatDuration(getEffectiveAncient(row)), filter type number + valueGetter: getEffectiveAncient, shortcuts: >0(min:0.1), >30min(min:30), >1h(min:60)
          * commit_real_minutes: label "实际耗时", align right, sortMethod 使用 getEffectiveReal, formatter 调用 formatDuration(getEffectiveReal(row)), filter type number + valueGetter: getEffectiveReal, shortcuts: >0(min:0.1), >30min(min:30), >1h(min:60)
          * efficiency_ratio: label "提效比", align center, sortMethod 按 efficiency_ratio 排序, slotName: 'efficiency_ratio', filter type number, shortcuts: >100%(min:100), >200%(min:200), >300%(min:300)
        - 默认 pageSize 设为 250（KbFilterTable 默认 pageSizes 已是 [250, 500, 1000] 无需额外传入）
        - 新增 serverDateRange 变量，初始值 getDefaultDateRangeWide()（参照 TaskViewV2 第189行）
        - 修改 fetchData 函数：使用 serverDateRange 作为日期参数（替换原 dateRange ref），去掉 nextTick + updateOverviewCharts 调用
        - 新增 handleFilterChange(allFilters) 函数（参照 TaskViewV2 第240-254行）：从 allFilters.commit_time 读取日期范围，与 serverDateRange 比较，有变化则更新 serverDateRange 并重新 fetchData
        - handleRowClick 保持不变：跳转 `/commit/:commit_id`
        - 修改 onMounted：await nextTick() 后调用 filterTableRef.value?.setFilter('commit_time', serverDateRange) 设置默认日期筛选，然后 fetchData（参照 TaskViewV2 第265-269行）
        style 部分：
        - 删除 scoped style 中的 `.dashboard-metric-card`、`.metric-label`、`.metric-value` 相关 CSS（第257-273行），这些样式仅被删除的汇总卡片使用
