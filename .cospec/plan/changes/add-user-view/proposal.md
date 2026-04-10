# 变更：新增 User 视图（Phase 6）

## 原因
设计文档要求"从 user 可以看到他参与的 project"等多视角互通。Phase 5 完成了 Project 视图（从 project 看参与者），现在需要构建 User 视图，实现反向查看：从用户维度看到其参与的所有项目、提交的 commits、创建的 tasks、贡献统计等。

## 变更内容
- **后端新增 User 聚合查询 API**：GET `/api/v2/users`（从 tasks/commits 聚合用户列表）、GET `/api/v2/users/:userId`（用户详情：参与的 projects、tasks、commits）
- **新建 UserViewV2.vue**：用户列表 + 行点击展开详情（参与项目列表、task/commit 列表、贡献统计图表）
- **路由和导航更新**

## 影响
- **受影响的代码**：
  - `backend/user_handler_v2.go`（新建）：User 聚合查询 handler
  - `backend/main.go`：v2 路由组注册 user 路由
  - `frontend/src/api/es.js`：新增 user v2 API 函数
  - `frontend/src/views/UserViewV2.vue`（新建）：User 视图页面
  - `frontend/src/router/index.js`：新增路由
  - `frontend/src/App.vue`：导航菜单新增条目
