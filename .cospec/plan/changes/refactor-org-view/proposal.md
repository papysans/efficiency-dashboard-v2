# 变更：Org 视图改造（Phase 7）

## 原因
现有 OrgPanel.vue（364行）基于 ES 聚合查询，只能展示 task 维度的统计。新的 PG 数据模型包含了 task↔commit↔project 的完整关联链，Org 视图需要增强：在组织层级下钻时展示该组织下的项目列表、用户贡献详情和 commit 统计，实现"从 org 看到项目和人员"的多维视角。

## 变更内容
- **后端新增 Org 聚合 API**：GET `/api/v2/orgs`（基于 org_mapping + PG 数据的组织层级聚合）
- **新建 OrgViewV2.vue**：在现有层级下钻基础上，增加该组织下的项目列表、用户列表和 commit 统计
- **路由和导航更新**

## 影响
- **受影响的代码**：
  - `backend/org_handler_v2.go`（新建）：Org 聚合查询
  - `backend/main.go`：v2 路由组注册
  - `frontend/src/api/es.js`：新增 org v2 API
  - `frontend/src/views/OrgViewV2.vue`（新建）：Org 视图页面
  - `frontend/src/router/index.js` + `App.vue`：路由和导航
