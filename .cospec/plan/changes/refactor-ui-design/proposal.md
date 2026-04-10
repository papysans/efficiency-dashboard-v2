# 变更：UI重构 - 美化设计、补全功能、增强互操作性

## 原因
当前 UI 存在以下问题：1) CSS 样式大量重复（5-6个组件各自定义相同样式）；2) EfficiencyPanel 缺少代码行数指标和 URL 参数恢复；3) 所有表格无排序功能；4) Home 页面布局第二行卡片无上间距；5) 各面板图表展示不一致（User/Org 比 Project 少多个图表）。

## 变更内容
### UI 统一与美化
- 提取公共 CSS 到全局样式文件，消除 scoped 样式中的大量重复
- 统一各面板图表数量和展示维度
- Home 页面布局修正（卡片间距、颜色差异化）
- 所有表格列添加 sortable 支持

### 功能补全
- EfficiencyPanel 添加代码行数（total_code_lines）指标卡片
- EfficiencyPanel 从 URL query 恢复 dimension/id/dateRange 并自动查询
- UserPanel 补充 API 次数列（与其他面板一致）

### 互操作性增强
- EfficiencyPanel 接收 URL 参数后自动触发查询（从其他面板跳转后无需手动操作）

## 影响
- **受影响的代码**：
  - `frontend/src/style.css`: 添加公共面板样式类
  - `frontend/src/views/Home.vue`: 布局美化、卡片颜色差异化
  - `frontend/src/views/EfficiencyPanel.vue`: 添加代码行数指标、URL参数恢复、使用useChart管理饼图
  - `frontend/src/views/Dashboard.vue`: 表格列添加sortable
  - `frontend/src/views/ProjectPanel.vue`: 使用公共CSS类、表格列添加sortable
  - `frontend/src/views/RepoPanel.vue`: 使用公共CSS类、表格列添加sortable、使用useChart管理饼图
  - `frontend/src/views/UserPanel.vue`: 使用公共CSS类、表格列添加sortable、补充API次数列和图表
  - `frontend/src/views/OrgPanel.vue`: 使用公共CSS类、表格列添加sortable、补充图表
