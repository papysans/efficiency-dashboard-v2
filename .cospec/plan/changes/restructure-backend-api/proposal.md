# 变更：重构后端 API（移除 Stat 查询，新增 Task/聚合/提效分析/纠错 API，引入 PostgreSQL）

## 原因
当前后端只有 4 个 API（索引列表、Request 查询、Stat 查询、Stat 汇总），全部基于 Stat 物理索引查询。Change 1 已移除 Stat 层，新增 Task 层。后端需要：移除 Stat 相关 API，新增 Task 查询 API，实现实时维度聚合（从 Task 表 ES Aggregation），引入 PostgreSQL 存储提效分析元信息表，实现提效分析和纠错 API。

## 变更内容
- 移除 Stat 相关 API：删除 `/api/es/stat-data` 和 `/api/es/stat-summary` handler
- 新增 Request 查询 API：`GET /api/requests`（保留原有功能，路由路径调整）
- 新增 Task 查询 API：`GET /api/tasks`、`GET /api/tasks/summary`
- 新增维度聚合 API：`GET /api/aggregate`（实时从 Task 表 ES Aggregation，支持 project/repo/user/org1-4 维度）
- 引入 PostgreSQL：创建 `init_db.sql`（project_metrics、repo_metrics、correction_history 表），后端新增 PG 连接
- 新增提效分析 API：`GET /api/analysis/efficiency`、`POST /api/analysis/efficiency/calculate`、`PUT /api/analysis/efficiency/correct`
- 新增纠错历史 API：`GET /api/analysis/efficiency/history`
- 新增分析文件下载 API：`GET /api/analysis/efficiency/file`
- 路由整理：从 `/api/es/` 统一改为 `/api/` 前缀
- **BREAKING**：前端需要适配新路由

## 影响
- **受影响的代码**：
    - `backend/main.go`: 路由注册重构，新增 PG 连接初始化
    - `backend/es_handler.go`: 移除 getStatData/getStatSummary，保留并重命名 getIndices/getRawData
    - `backend/task_handler.go`: **新增文件**，Task 查询和聚合 handler
    - `backend/aggregate_handler.go`: **新增文件**，实时维度聚合 handler
    - `backend/analysis_handler.go`: **新增文件**，提效分析和纠错 handler
    - `backend/db.go`: **新增文件**，PostgreSQL 连接和查询
    - `backend/config.go`: 新增 PG 配置
    - `backend/config.yaml`: 新增 database 配置块
    - `init_db.sql`: **新增文件**，数据库建表脚本
    - `backend/es_client.go`: 新增排序支持
