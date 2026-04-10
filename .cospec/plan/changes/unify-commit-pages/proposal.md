# 变更：统一 Commit 页面设计风格与 Task 页面一致

## 原因
Commit 详情页和 Commit 列表页与对应的 Task 页面在设计风格、组件使用上不统一，导致用户体验不一致。需要参照 Task 页面的成熟设计模式对 Commit 页面进行统一改造。

## 变更内容
### Commit 详情页（CommitDetailV2.vue）
- 去掉独立的「提效指标大卡片」（3列彩色大字卡片），将度量信息整合到 el-descriptions 中
- 基础信息和度量信息拆分为两个独立的 el-card（与 TaskDetailV2 一致）
- 基础信息中的「用户」字段添加 el-link 跳转到用户详情页
- 度量信息采用 el-descriptions 展示：Diff行数、古法预估（manual优先+tooltip）、实际耗时（manual优先+tooltip）、提效比（彩色大字）
- 保留「关联 Tasks」表格（Commit 独有的内容区域）
- 保留「人工调整」对话框

### Commit-v2 列表页（CommitViewV2.vue）
- 用 KbFilterTable 组件替换 FilterBar + 原生 el-table
- 去掉汇总指标卡片（4列大字卡片）
- 去掉 echarts 图表（提效比分布 + 古法vs实际）
- 去掉 FilterBar、useChart、useUrlSync 依赖
- 列定义参照 TaskViewV2 的模式：列头筛选（search-select/text/date/number/enum）
- 默认分页提升至 250 条
- 点击行跳转到 /commit/:commitId 详情页

## 影响
- **受影响的规范**：Commit 详情展示、Commit 列表展示
- **受影响的代码**：
    - `frontend/src/views/CommitDetailV2.vue`: 重构布局为 基础信息卡片 + 度量信息卡片 + 关联Tasks表格，去掉提效指标大卡片
    - `frontend/src/views/CommitViewV2.vue`: 用 KbFilterTable 替换 FilterBar+el-table，去掉汇总卡片和图表
