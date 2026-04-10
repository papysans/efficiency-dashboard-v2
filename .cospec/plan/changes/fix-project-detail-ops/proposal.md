# 变更：修复 Project 详情页操作能力 + 添加顶部导航

## 原因
1. App.vue 全局顶部导航栏缺少"项目"入口，用户无法从任何页面直接跳转到项目列表
2. ProjectDetailV2.vue 详情页中 Repos 区域缺少"添加"和"编辑"功能，Tasks 区域缺少"添加""删除""编辑 silica"功能

## 变更内容
- App.vue 顶部导航栏新增"项目"菜单项
- 后端新增 `removeTasksFromProject` 和 `updateTaskSilicaInProject` API
- ProjectDetailV2.vue Repos 区域：添加"添加 Repo"按钮 + 编辑对话框
- ProjectDetailV2.vue Tasks 区域：添加"添加 Task"按钮 + 删除按钮 + 编辑 silica 权重

## 影响
- **受影响的代码**：
  - `frontend/src/App.vue`: 新增导航菜单项
  - `backend/project_handler_v2.go`: 新增 removeTasksFromProjectV2、updateTaskSilicaInProjectV2 handler
  - `backend/main.go`: 注册新路由
  - `frontend/src/api/es.js`: 新增 API 方法
  - `frontend/src/views/ProjectDetailV2.vue`: 添加 Repos/Tasks 的增删改操作
