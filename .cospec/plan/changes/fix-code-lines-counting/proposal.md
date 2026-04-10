# 变更：修复代码行数统计与提取前端公共层

## 原因
`apply_diff` 工具的代码行数统计因参数 key 错误（读取 `"content"` 而非 `"diff"`）导致所有 apply_diff 产出记为 0 行，严重低估实际代码产出。同时前端存在约 800 行跨组件重复代码，阻碍后续 UI 重构。

## 变更内容
### Phase 1: 修复代码行数统计
- 修复 `raw_parser.go:calcOutCodeLines()` 对 `apply_diff` 的参数 key 读取（`"diff"` 替代 `"content"`）
- 实现 `apply_diff` 的 REPLACE 部分行数提取（只统计 `=======` 到 `>>>>>>> REPLACE` 之间的新增代码行）
- 修复 `task_content.go:ExtractTaskContent()` 的代码内容提取逻辑
- 修正 `raw_parser_test.go` 中 TP-25 测试数据（使用 `"diff"` key 和真实 diff 格式）

### Phase 2: 提取前端公共层
- 提取 `src/utils/formatters.js`（fmtCost/fmtDays/fmtMsToMin/fmtNumber）
- 提取 `src/utils/date.js`（getDefaultDateRange/formatDateParam）
- 提取 `src/utils/chart.js`（createBarOption/createDualBarOption/truncateName/统一图表主题）
- 提取 `src/composables/useFavorites.js`（收藏逻辑4处复用）
- 提取 `src/composables/useUrlSync.js`（URL同步5处复用）
- 提取 `src/composables/useChart.js`（ECharts生命周期管理4处复用）
- 添加 axios 响应拦截器统一错误处理
- 各面板组件引用公共模块，删除重复代码

## 影响
- **受影响的代码**：
  - `kbcli/raw_parser.go`: 修复 `calcOutCodeLines()` 的 apply_diff 分支
  - `kbcli/task_content.go`: 修复代码内容提取的 apply_diff 分支
  - `kbcli/raw_parser_test.go`: 修正测试数据
  - `frontend/src/utils/formatters.js`: 新增，公共格式化函数
  - `frontend/src/utils/date.js`: 新增，日期工具函数
  - `frontend/src/utils/chart.js`: 新增，图表工具函数
  - `frontend/src/composables/useFavorites.js`: 新增，收藏逻辑composable
  - `frontend/src/composables/useUrlSync.js`: 新增，URL同步composable
  - `frontend/src/composables/useChart.js`: 新增，ECharts生命周期composable
  - `frontend/src/api/index.js`: 添加响应拦截器
  - `frontend/src/views/Dashboard.vue`: 引用公共模块，删除重复代码
  - `frontend/src/views/EfficiencyPanel.vue`: 引用公共模块，删除重复代码
  - `frontend/src/views/ProjectPanel.vue`: 引用公共模块，删除重复代码
  - `frontend/src/views/RepoPanel.vue`: 引用公共模块，删除重复代码
  - `frontend/src/views/UserPanel.vue`: 引用公共模块，删除重复代码
  - `frontend/src/views/OrgPanel.vue`: 引用公共模块，删除重复代码
