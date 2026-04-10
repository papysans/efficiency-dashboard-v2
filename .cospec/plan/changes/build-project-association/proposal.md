# 变更：构建 Project 关联引擎（Phase 4）

## 原因
Phase 2-3 已将 Task 和 Commit 数据存入 PG，但两者之间缺乏关联关系。设计文档核心需求是：通过 commit↔task 的 diff 对比分析建立 project 关联，计算硅比例（silica），并生成 project 聚合记录，支持人工调整。这是整个系统的核心价值——量化 AI 编程对项目的实际贡献。

## 变更内容
- **后端新增 Project 关联引擎 API**：POST `/api/v2/projects/associate`（触发关联分析）、GET `/api/v2/projects`（列表）、GET `/api/v2/projects/:repoId`（详情，含关联的 tasks/commits/silica）、PUT `/api/v2/projects/:repoId/manual`（人工调整）
- **后端新增关联逻辑**：在 backend 中实现 commit↔task 的初步关联（基于 repo_id + user_id + 时间窗口），复用 kbcli 中 `git_task_matcher.go` 已有的匹配算法思路
- **AI 硅比例分析**：调用 AI API 对比 task.diff 和 commit.diff 的相似度，得出每个 task 对 commit 的代码贡献比例
- **Project 聚合生成**：将关联结果汇总为 costrict_projects 记录（task_ids/commit_ids/silica 数组 + 加权统计）

## 影响
- **受影响的代码**：
  - `backend/project_handler_v2.go`（新建）：Project API handler
  - `backend/project_associator.go`（新建）：关联引擎核心逻辑
  - `backend/main.go`：v2 路由组注册 project 路由
