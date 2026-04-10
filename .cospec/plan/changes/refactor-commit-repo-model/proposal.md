# 变更：统一迁移至 costrict_stat 库，重构 Commit/Repo 数据模型

## 原因
1. 统一数据存储：将所有业务表统一到 `costrict_stat` 数据库（密码 1），清理 `report` 库中的 `costrict_*` 系列冗余表
2. 将 repo 概念从静态数据库表（`costrict_projects`）改为基于 `commits` 表的动态聚合视图——"没有真正的 repo 表"
3. commit_id 从复合唯一键 `(commit_id, repo_id)` 改为单字段主键 `commit_id`（全局唯一）
4. 同步字段重命名（`project_path` → `work_path`）和路由优化（`/repo/:repoId` → `/repo/:repoAddr/:repoBranch?`）
5. repo 详情页增强：分支下拉切换、时间段筛选、效率评估汇总、commits/tasks 列表下钻

## 变更内容

### 1. 数据库变更
- **costrict_stat 库**：新建 `commits` 表（DDL 追加到 `init_db_stat.sql`），字段命名风格与已有 `tasks` 表对齐（无 `costrict_` 前缀，使用 `work_path` 而非 `project_path`）
- **commit_id 为单字段主键**（全局唯一），不再需要 `repo_id` 组合
- **commits 表不含 repo_id 字段**（repo_addr + repo_branch 即可定位仓库）
- **report 库**：从 `init_db.sql` 中清理 `costrict_commits`、`costrict_tasks`、`costrict_task_conversations`、`costrict_projects` 的 CREATE TABLE/索引/迁移语句，添加 DROP TABLE 迁移保底

### 2. 后端变更
- **移除所有 `Costrict*` struct 及 CRUD**：`CostrictTask`、`CostrictTaskConversation`、`CostrictCommit`、`CostrictProject` 全部从 `db.go` 中删除
- **移除** `project_associator.go` 文件
- **新建 `StatCommit` struct 及 CRUD**（在 `db.go` 中），操作 `statDB` 的 `commits` 表，字段与需求定义对齐
- **所有 handler 统一使用 `statDB`**：
  - `commit_handler_v2.go`：从 `db` + `CostrictCommit` 改为 `statDB` + `StatCommit`；移除 repoId 必填参数
  - `user_handler_v2.go`：从 `ListCostrictTasks/ListCostrictCommits` 改为 `ListStatTasks` + 新的 `ListStatCommits`
  - `dashboard_handler_v2.go`：从 `costrict_projects` 计数改为 `commits` 表聚合
  - `org_handler_v2.go`：SQL 查询从 `costrict_*` 表迁移到 `tasks/commits` 表
- **移除 V2 路由中 projects 相关端点**
- **重构** `listReposV2` — 从 `commits` 表动态聚合 `DISTINCT(repo_addr, repo_branch)`
- **重构** `getRepoDetailV2` — 接受 `repoAddr`+`repoBranch`+`startDate`+`endDate` 参数，实时聚合效率评估（repo_ancient_minutes, repo_real_minutes, efficiency_ratio 等），含人工校准字段
- **重构** `getCommitDetailV2` — 移除 `repoId` 必填参数，仅凭 `commitId` 查询
- **新增** `listRepoBranchesV2` — 获取指定仓库的分支列表
- **repo 级别效率数据**为动态聚合计算结果，不持久化到数据库；repo 的人工校准通过各 commit 的 manual 字段间接实现

### 3. 前端变更
- **路由**：`/repo/:repoId` → `/repo/:repoAddr/:repoBranch?`，移除 `/project/:projectId`
- **重构** Repo 列表页 — 移除"触发关联分析"按钮，表格展示 repo_addr/repo_branch
- **重构** Repo 详情页 — 分支下拉切换、时间范围筛选、效率评估指标卡片（古法预估/实际耗时/提效比）、commits 列表（可跳转 /commit/:commitId）、tasks 列表（从 commits.task_ids 下钻获取，可跳转 /task/:taskId）、人工校准
- **更新** Commit 详情页 — 移除 repoId 依赖，通过 /commit/:commitId 访问
- **更新** API 函数 — 移除 project 相关，更新 repo/commit 签名
- **更新** 所有视图中的 repo 跳转链接

## 影响
- **受影响的规范**：数据存储统一、Repo 动态聚合、Commit 路由简化
- **受影响的代码**：
    - `init_db_stat.sql`: 新建 `commits` 表 DDL
    - `init_db.sql`: 清理所有 `costrict_*` 表 DDL，添加 DROP TABLE 迁移
    - `seed_data.sql`: 清理 `costrict_projects` 测试数据
    - `backend/db.go`: 移除 `Costrict*` struct/CRUD，新建 `StatCommit` struct/CRUD
    - `backend/project_associator.go`: 删除整个文件
    - `backend/project_handler_v2.go`: 移除 project handler，重写 repo handler（含 repo 级别效率聚合和人工校准）
    - `backend/commit_handler_v2.go`: 改用 `statDB` + `StatCommit`，移除 repoId 依赖
    - `backend/dashboard_handler_v2.go`: 改用 `commits` 表聚合
    - `backend/user_handler_v2.go`: 改用 `statDB` + `StatCommit`/`StatTask`
    - `backend/org_handler_v2.go`: SQL 查询从 `costrict_*` 迁移到 `tasks/commits`
    - `backend/main.go`: 更新路由注册
    - `kbcli/pg_writer.go`: 适配 StatCommit 结构（移除 RepoID，ProjectPath→WorkPath）
    - `frontend/src/router/index.js`: 路由变更
    - `frontend/src/api/es.js`: API 函数变更
    - `frontend/src/views/ProjectViewV2.vue`: 重构 repo 列表页
    - `frontend/src/views/ProjectDetailV2.vue`: 重构 repo 详情页（分支切换+时间筛选+效率评估+commits/tasks 列表+人工校准）
    - `frontend/src/views/CommitDetailV2.vue`: 更新 commit 详情页
    - `frontend/src/views/CommitViewV2.vue`: 更新 commit 列表页
    - `frontend/src/views/WorkDirDetailV2.vue`: 更新工作目录详情页
    - `frontend/src/views/TaskDetailV2.vue`: 更新仓库链接跳转
    - `frontend/src/views/UserDetailV2.vue`: 更新仓库链接跳转
    - `frontend/src/views/OrgDetailV2.vue`: 更新仓库链接跳转
