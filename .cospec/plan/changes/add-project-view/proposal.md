# 变更：新增 Project 视图 UI（Phase 5）

## 原因
Phase 1-4 已完成数据管道和关联引擎（PG 中有 tasks/commits/projects 数据 + v2 API），但前端仍使用旧的 ES 聚合查询模式。需要构建基于 PG v2 API 的全新 Project 视图，实现设计文档要求的"从 project 可以看到参与者、关联 commits、关联 tasks 和硅比例"的核心 UI 交互。

## 变更内容
- **前端新增 v2 API 函数**：在 `api/es.js` 中新增调用 v2 接口的函数（tasks/commits/projects）
- **新建 ProjectViewV2.vue**：全新的 Project 视图页面，采用 RepoPanel 的"行点击展开详情"模式，展示 project 列表→点击展开参与者/commits/tasks/硅比例
- **路由和导航更新**：新增路由、调整菜单显示

## 影响
- **受影响的代码**：
  - `frontend/src/api/es.js`：新增 v2 API 调用函数
  - `frontend/src/views/ProjectViewV2.vue`（新建）：Project 视图页面
  - `frontend/src/router/index.js`：新增路由
  - `frontend/src/App.vue`：导航菜单新增条目
