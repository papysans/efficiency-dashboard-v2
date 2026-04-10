# 变更：Dashboard + 首页改造（Phase 8）

## 原因
所有数据管道和视图页面已就绪（Phase 1-7），但首页（Home.vue）和 Dashboard 仍基于旧的 ES 数据模式。需要改造首页为全局概览 Dashboard，展示关键汇总指标（总 project 数/user 数/task 数/commit 数/总费用/AI 预估人天），并提供快速导航到各视图。

## 变更内容
- **后端新增 Dashboard 汇总 API**：GET `/api/v2/dashboard/summary`（全局汇总指标）
- **改造首页 Home.vue**：从简单的导航卡片改为带数据指标的 Dashboard 概览页
- **导航菜单整理**：整合 v1/v2 菜单，v2 页面作为主要导航

## 影响
- **受影响的代码**：
  - `backend/dashboard_handler_v2.go`（新建）：Dashboard 汇总 API
  - `backend/main.go`：注册路由
  - `frontend/src/api/es.js`：新增 API 函数
  - `frontend/src/views/Home.vue`：改造为 Dashboard 概览
  - `frontend/src/App.vue`：整理导航菜单
