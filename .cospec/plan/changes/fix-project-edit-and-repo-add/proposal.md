# 变更：修复Project编辑数据丢失 & Repo详情页添加到Project下拉选择

## 原因
1. Project编辑对话框提交时只发送 name/description，后端全量覆盖导致 repos/task_ids/task_ids_silica 被清空
2. Repo详情页"添加到Project"对话框中"目标Project"下拉选择功能已存在，但用户反映无法正常使用（需确认是否为 getProjects API 返回数据结构解析问题）

## 变更内容
- 修复 `submitEdit()` 函数：提交时携带 `project.value` 中的 repos/task_ids/task_ids_silica 原始数据，防止全量覆盖时丢失
- 排查并修复 `openAddToProject()` 中 projectList 数据解析，确保下拉列表能正确展示已有 Project

## 影响
- **受影响的规范**：项目管理
- **受影响的代码**：
    - `frontend/src/views/ProjectDetailV2.vue`：`submitEdit()` 补全 repos/task_ids/task_ids_silica 字段
    - `frontend/src/views/RepoDetailV2.vue`：确认 `openAddToProject()` 中 projectList 解析逻辑正确
