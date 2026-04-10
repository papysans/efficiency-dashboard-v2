# 变更：重构 PG 数据模型（Phase 1）

## 原因
现有数据库 schema（project_metrics、repo_metrics 等 6 张聚合指标表）是面向"分析结果展示"设计的，无法承载设计文档中要求的 task→commit→project 三层数据关联体系和硅比例追踪。需要新建核心数据表作为后续所有阶段（数据管道、关联引擎、UI）的基础。

## 变更内容
- 新建 `costrict_tasks` 表：存储 task 汇总数据（对应设计文档"服务端 task_summary"，排除 diff 大字段）
- 新建 `costrict_task_conversations` 表：存储 task 的对话明细（对应设计文档"task_conversation.jsonl"的每一行）
- 新建 `costrict_commits` 表：存储 commit 数据（对应设计文档"commit_id.json"，排除 diff 大字段）
- 新建 `costrict_projects` 表：存储 project 关联聚合数据（含 task_ids/commit_ids/silica 等 JSONB 字段及人工调整字段）
- 后端新增 Go struct 定义和基础 CRUD 函数（Upsert/Get/List）
- 构造测试种子数据（5 用户、3 仓库、15 个 task、12 个 commit、4 个 project，模拟真实场景）
- 保留现有旧表（project_metrics、repo_metrics 等）不删除，确保现有功能不中断

## 影响
- **受影响的规范**：数据模型、数据管道
- **受影响的代码**：
  - `init_db.sql`：新增 4 张表的 DDL + 索引
  - `backend/db.go`：新增 4 个 Go struct + scan 辅助函数 + CRUD 函数
  - `seed_data.sql`（新建）：测试数据种子脚本
