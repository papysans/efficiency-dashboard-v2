# 变更：构建 Task 数据管道（Phase 2）

## 原因
Phase 1 已建好 PG 数据模型（costrict_tasks + costrict_task_conversations），但数据仍然只存储在 ES 中。需要建立从 rawdata 原始 JSON 文件解析并写入 PG 的完整数据管道，同时提供基于 PG 的 Task 查询 API，为后续 commit/project 关联和 UI 视图提供数据基础。

## 变更内容
- **后端新增 Task 写入 API**：POST `/api/v2/tasks`（upsert task）、POST `/api/v2/tasks/conversations/batch`（批量写入对话记录）
- **后端新增 Task 查询 API**：GET `/api/v2/tasks`（列表，支持多维度过滤+分页）、GET `/api/v2/tasks/:taskId`（详情+关联conversations）
- **kbcli 新增 PG 写入模块**：`pg_writer.go` 实现 RawDoc/TaskDoc → CostrictTask/CostrictTaskConversation 的字段映射 + 通过 backend API 写入 PG
- **kbcli reindex 命令增加 PG 写入**：在现有 task 步骤中同步写入 PG（并行于 ES 写入）
- 从现有 rawdata 回填数据到 PG

## 影响
- **受影响的代码**：
  - `backend/main.go`：新增 v2 路由组注册
  - `backend/task_handler_v2.go`（新建）：4 个新 handler 函数
  - `kbcli/pg_writer.go`（新建）：PG 写入客户端 + 数据映射
  - `kbcli/cmd_reindex.go`：reindexTask 函数中增加 PG 写入步骤
