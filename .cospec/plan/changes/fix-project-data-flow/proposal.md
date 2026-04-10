# 变更：修复 Project 数据流 + UI 关键体验（P0+P1）

## 原因
逐页 UI 审查发现：Project 视图只显示 seed_data 的 4 个 project，真实导入的 58 个 task 无法在 Project 维度展示；Project 详情指标全为空。根本原因是 listProjectsV2 只查 costrict_projects 表（依赖关联引擎预计算），而真实 task 数据的 repo_id 格式（project_id）与 costrict_projects 不匹配。

## 变更内容
- **P0-1: Project 列表改为从 tasks 表实时聚合**：listProjectsV2 直接 SQL 聚合 costrict_tasks 按 project_id 分组，返回每个项目的 task_count/cost/tokens/时间范围，不再仅依赖 costrict_projects 表
- **P0-2: Project 详情改为从 tasks/commits 表实时查询**：projectDetailResponse 按 project_id 查 tasks，按 repo_id 查 commits，实时返回
- **P0-3: Project 视图前端适配**：表格 key 从 repo_id 改为 project_id，详情数据源适配
- **P1-1: 时间格式化**：User 列表活跃时间显示为 YYYY-MM-DD
- **P1-2: 空数据提示**：所有表格添加 empty-text
- **P1-3: Dashboard 筛选区样式**：筛选区独立于指标卡片

## 影响
- `backend/project_handler_v2.go`：重写 listProjectsV2 和 projectDetailResponse
- `frontend/src/views/ProjectViewV2.vue`：表格字段适配
- `frontend/src/views/UserViewV2.vue`：时间格式化
- `frontend/src/views/Home.vue`：筛选区样式
- 所有 v2 Vue 页面：el-table 添加 empty-text
