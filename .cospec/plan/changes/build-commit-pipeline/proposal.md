# 变更：构建 Commit 数据管道（Phase 3）

## 原因
Phase 2 完成了 Task 数据管道（rawdata→PG），但 Commit 数据仍仅存在于 Git 分析结果 JSON 文件中，未入库 PG。需要将 Git 分析产生的 Commit 数据写入 PG costrict_commits 表，并提供 Commit 查询 API，为 Phase 4 的 project 关联引擎和后续 UI 视图提供数据基础。

## 变更内容
- **后端新增 Commit 写入 API**：POST `/api/v2/commits`（upsert commit）、POST `/api/v2/commits/batch`（批量写入）
- **后端新增 Commit 查询 API**：GET `/api/v2/commits`（列表，支持 repoId/userId/时间过滤+分页）、GET `/api/v2/commits/:commitId`（详情）
- **kbcli git 分析流程增加 PG 写入**：在 `runAnalyzeGit` 函数中，Git 分析完成后将 CommitDetail 映射为 CostrictCommit 并写入 PG
- **kbcli 新增 commit PG 写入方法**：在 `pg_writer.go` 中增加 Commit 数据映射和写入函数

## 影响
- **受影响的代码**：
  - `backend/commit_handler_v2.go`（新建）：4 个新 handler 函数
  - `backend/main.go`：v2 路由组注册 commit 路由
  - `kbcli/pg_writer.go`：新增 Commit 映射和写入方法
  - `kbcli/cmd_analyze.go`：`runAnalyzeGit` 函数中增加 PG 写入步骤
