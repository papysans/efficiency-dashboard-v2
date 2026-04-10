## 实施

### 阶段 1: 数据库变更

- [x] 1.1 在 costrict_stat 库新建 commits 表
     【目标对象】`init_db_stat.sql`
     【修改目的】在 costrict_stat 库中创建 commits 表，与 tasks 表风格对齐
     【修改方式】在文件末尾追加 commits 表 DDL
     【相关依赖】参考现有 `tasks` 表的字段命名风格（无 costrict_ 前缀，使用 work_path）
     【修改内容】
        - 新建 `commits` 表，commit_id 为单字段主键（全局唯一），不含 repo_id 字段：
          `commit_id VARCHAR(500) PRIMARY KEY`
          `commit_time TIMESTAMPTZ`
          `repo_addr TEXT`, `repo_branch VARCHAR(500)`
          `git_user_name VARCHAR(255)`, `git_user_email VARCHAR(255)`
          `user_id VARCHAR(255)`, `user_name VARCHAR(255)`, `client_id VARCHAR(255)`
          `work_path TEXT`（原 project_path）
          `diff_lines INT`
          `commit_ancient_minutes FLOAT8`, `commit_ancient_minutes_reason TEXT`
          `commit_ancient_minutes_manual FLOAT8`, `commit_ancient_minutes_reason_manual TEXT`
          `task_ids JSONB`, `task_ids_silica JSONB`
          `commit_real_ai_minutes FLOAT8`, `commit_real_ancient_minutes FLOAT8`
          `commit_real_minutes FLOAT8`, `commit_real_minutes_reason TEXT`
          `commit_real_minutes_manual FLOAT8`, `commit_real_minutes_reason_manual TEXT`
          `created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP`
          `updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP`
        - 创建索引：`idx_commits_repo_addr` ON commits(repo_addr)
        - 创建索引：`idx_commits_repo_addr_branch` ON commits(repo_addr, repo_branch)
        - 创建索引：`idx_commits_user_id` ON commits(user_id)
        - 创建索引：`idx_commits_commit_time` ON commits(commit_time)

- [x] 1.2 清理 report 库的 costrict_* 表
     【目标对象】`init_db.sql`
     【修改目的】移除 report 库中不再使用的 costrict_ 系列表的 DDL 和迁移语句
     【修改方式】删除原始 CREATE TABLE/索引/迁移语句，并在文件末尾追加 DROP TABLE 保底
     【相关依赖】无
     【修改内容】
        - 删除 `costrict_tasks` 的 CREATE TABLE 语句（L228-258 附近）及相关索引
        - 删除 `costrict_task_conversations` 的 CREATE TABLE 语句及相关索引
        - 删除 `costrict_commits` 的 CREATE TABLE 语句（L279-306 附近）及相关索引
        - 删除 `costrict_projects` 的 CREATE TABLE 语句（L308-330 附近）及相关索引
        - 删除所有 `costrict_*` 相关的迁移语句（L380-475 附近的 ALTER TABLE/函数/UPDATE 等）
        - 在文件末尾追加 DROP TABLE 迁移保底：
          `DROP TABLE IF EXISTS costrict_projects CASCADE;`
          `DROP TABLE IF EXISTS costrict_commits CASCADE;`
          `DROP TABLE IF EXISTS costrict_tasks CASCADE;`
          `DROP TABLE IF EXISTS costrict_task_conversations CASCADE;`

- [x] 1.3 清理 seed_data.sql 中的 costrict_projects 测试数据
     【目标对象】`seed_data.sql`
     【修改目的】移除已删表的测试数据
     【修改方式】删除 INSERT INTO costrict_projects 相关行
     【相关依赖】无
     【修改内容】
        - 删除所有 `INSERT INTO costrict_projects` 语句

### 阶段 2: 后端数据层 — 移除 Costrict* 系列，新建 StatCommit

- [x] 2.1 在 db.go 中新建 StatCommit struct 和 CRUD 函数
     【目标对象】`backend/db.go`
     【修改目的】新建操作 costrict_stat.commits 表的数据模型和 CRUD
     【修改方式】在 StatTask 相关代码块附近新增
     【相关依赖】`init_db_stat.sql` 中的 commits 表定义（1.1）
     【修改内容】
        - 新建 `StatCommit` struct，字段与 commits 表对齐：
          不含 ID（无自增 id）、不含 RepoID 字段
          使用 `WorkPath *string json:"work_path"`（原 ProjectPath）
          commit_id 为 string 非指针（主键）
          风格参考 `StatTask`
        - 新建 `statCommitSelectColumns` 变量
        - 新建 `scanStatCommit` 函数（处理 JSONB 字段 task_ids/task_ids_silica）
        - 新建 `UpsertStatCommit(db *sql.DB, c *StatCommit) error` — INSERT ON CONFLICT(commit_id) DO UPDATE
        - 新建 `GetStatCommitByID(db *sql.DB, commitID string) (*StatCommit, error)` — 仅凭 commitId 查询
        - 新建 `ListStatCommits(db *sql.DB, repoAddr, repoBranch, userID, startTime, endTime string, page, pageSize int) ([]StatCommit, error)` — 支持 repo_addr+repo_branch+userId+时间范围筛选
        - 新建 `CountStatCommits(db *sql.DB, repoAddr, repoBranch, userID, startTime, endTime string) (int, error)`
        - 新建 `BatchUpsertStatCommits(db *sql.DB, commits []StatCommit) error` — 事务批量 upsert
        - 新建 `UpdateStatCommitManual(db *sql.DB, commitID string, ancientManual *float64, ancientReasonManual *string, realManual *float64, realReasonManual *string) error`
        - 新建 `UpdateStatCommitTaskAssoc(db *sql.DB, commitID string, taskIDs, taskIDsSilica json.RawMessage, realMinutes *float64, realAIMinutes *float64, realAncientMinutes *float64, realReason *string) error`
        - 新建 `ListRepoAggregates(db *sql.DB, startTime, endTime string) ([]map[string]interface{}, error)` — 从 commits 按 (repo_addr, repo_branch) GROUP BY 聚合，返回 repo_addr, repo_branch, commit_count, start_time(MIN), end_time(MAX), sum(commit_ancient_minutes), sum(commit_real_minutes)
        - 新建 `ListBranchesByRepoAddr(db *sql.DB, repoAddr string) ([]string, error)` — SELECT DISTINCT repo_branch

- [x] 2.2 从 db.go 中移除所有 Costrict* struct 和 CRUD
     【目标对象】`backend/db.go`
     【修改目的】清理已迁移/废弃的代码
     【修改方式】删除代码块
     【相关依赖】2.1 完成后再执行
     【修改内容】
        - 删除 `CostrictTask` struct（L154 附近）及相关：`costrictTaskSelectColumns`, `scanCostrictTask`, `UpsertCostrictTask`, `GetCostrictTask`, `ListCostrictTasks`, `CountCostrictTasks`, `DeleteCostrictTask`, `UpdateCostrictTaskManual`
        - 删除 `CostrictTaskConversation` struct（L189 附近）及相关：`costrictTaskConversationSelectColumns`, `scanCostrictTaskConversation`, `InsertCostrictTaskConversation`, `ListCostrictTaskConversations`, `BatchInsertCostrictTaskConversations`
        - 删除 `CostrictCommit` struct（L214 附近）及相关：`costrictCommitSelectColumns`, `scanCostrictCommit`, `UpsertCostrictCommit`, `GetCostrictCommit`, `ListCostrictCommits`, `CountCostrictCommits`, `BatchUpsertCostrictCommits`, `DeleteCostrictCommit`, `UpdateCostrictCommitManual`, `UpdateCostrictCommitTaskAssoc`
        - 删除 `CostrictProject` struct（L245 附近）及相关：`costrictProjectSelectColumns`, `scanCostrictProject`, `UpsertCostrictProject`, `GetCostrictProject`, `ListCostrictProjects`, `UpdateCostrictProjectManual`, `jsonRawToString`

- [x] 2.3 删除 project_associator.go
     【目标对象】`backend/project_associator.go`
     【修改目的】移除不再使用的 project 关联逻辑（AssociateProjectByRepo, AssociateAllProjects, listDistinctRepoIDs, calcSimpleSilica, estimateSilicaWithAI 等）
     【修改方式】删除整个文件
     【相关依赖】2.2 完成后
     【修改内容】
        - 删除整个 `project_associator.go` 文件
        - 确认无残留引用（`triggerProjectAssociation` 在 3.2 中被移除）

### 阶段 3: 后端 Handler 层重构

- [x] 3.1 重构 commit_handler_v2.go
     【目标对象】`backend/commit_handler_v2.go`
     【修改目的】改用 statDB + StatCommit，移除 repoId 必填依赖，commit_id 为单字段查询
     【修改方式】修改全部 5 个函数实现
     【相关依赖】`StatCommit` struct 及 CRUD（2.1）
     【修改内容】
        - `upsertCommitV2`：`CostrictCommit` → `StatCommit`，`UpsertCostrictCommit(db, ...)` → `UpsertStatCommit(statDB, ...)`
        - `batchUpsertCommitsV2`：`[]CostrictCommit` → `[]StatCommit`，`BatchUpsertCostrictCommits(db, ...)` → `BatchUpsertStatCommits(statDB, ...)`
        - `listCommitsV2`：query 参数从 `repoId` 改为 `repoAddr` + `repoBranch`；`CountCostrictCommits(db, ...)` → `CountStatCommits(statDB, ...)`；`ListCostrictCommits(db, ...)` → `ListStatCommits(statDB, ...)`
        - `getCommitDetailV2`：移除 repoId 必填校验（L96-100 删除），`GetCostrictCommit(db, commitID, repoID)` → `GetStatCommitByID(statDB, commitID)`；`GetCostrictTask(db, taskID)` → `GetStatTask(statDB, taskID)`；异步写回改为 `UpdateStatCommitTaskAssoc(statDB, commitID, ...)`（移除 repoID 参数）
        - `updateCommitManualV2`：移除 repoId 必填校验（L210-214 删除），`UpdateCostrictCommitManual(db, commitId, repoId, ...)` → `UpdateStatCommitManual(statDB, commitId, ...)`

- [x] 3.2 重构 project_handler_v2.go — 移除 project handler，重写 repo handler
     【目标对象】`backend/project_handler_v2.go`
     【修改目的】移除所有 project 相关 handler，重写 repo 列表和详情 handler 为 commits 动态聚合
     【修改方式】重写整个文件
     【相关依赖】`StatCommit`, `StatTask`, `ListRepoAggregates`, `ListBranchesByRepoAddr`, `ListStatCommits`（2.1）
     【修改内容】
        - 删除全部 project 相关 handler：`triggerProjectAssociation`, `listProjectsV2`, `getProjectDetailV2`, `getProjectDetailByQueryV2`, `projectDetailResponse`, `updateProjectManualV2`, `getProjectDetailByProjectIdV2`
        - 删除辅助函数：`extractUniqueRepoIds`, `sumTaskCost`, `sumTaskTokens`, `countDistinctUsers`, `sumTaskAIDays`
        - 重写 `listReposV2`：调用 `ListRepoAggregates(statDB, startTime, endTime)` 从 commits 动态聚合，返回 repo_addr, repo_branch, commit_count, start_time, end_time, sum(commit_ancient_minutes), sum(commit_real_minutes) 等，支持分页
        - 重写 `getRepoDetailV2`：接受 query 参数 `repoAddr` + `repoBranch` + `startDate` + `endDate`
          1. 调用 `ListStatCommits(statDB, repoAddr, repoBranch, "", startTime, endTime, 1, 10000)` 获取 commits
          2. 从 commits 的 task_ids 字段解析去重所有 taskID，批量调用 `GetStatTask(statDB, taskID)` 获取关联 tasks
          3. 实时计算 repo 级别效率评估：
             - `repo_ancient_minutes = sum(commit_ancient_minutes_manual ?? commit_ancient_minutes)`
             - `repo_real_minutes = sum(commit_real_minutes_manual ?? commit_real_minutes)`
             - `efficiency_ratio = (repo_ancient_minutes / repo_real_minutes) * 100`（两者都 > 0 时）
             - `repo_ancient_minutes_reason`：汇总说明
             - `repo_real_minutes_reason`：汇总说明
          4. 调用 `ListBranchesByRepoAddr(statDB, repoAddr)` 获取分支列表
          5. 返回 JSON：`{ branches, commits, tasks, efficiency: {repo_ancient_minutes, repo_real_minutes, efficiency_ratio, ...}, summary }`
        - 新增 `listRepoBranchesV2`：GET /api/v2/repos/branches?repoAddr=xxx，调用 `ListBranchesByRepoAddr`

- [x] 3.3 更新 dashboard_handler_v2.go
     【目标对象】`backend/dashboard_handler_v2.go`
     【修改目的】将所有 SQL 查询从 report 库的 costrict_* 表迁移到 costrict_stat 库的 tasks/commits 表
     【修改方式】修改 `getDashboardSummary` 函数中的 SQL 查询及其数据库连接
     【相关依赖】costrict_stat 库的 tasks 表（已存在）和 commits 表（1.1）
     【修改内容】
        - SQL1（task 统计）：表名 `costrict_tasks` → `tasks`；`db.QueryRow(...)` → `statDB.QueryRow(...)`
        - SQL2（commit 统计）：表名 `costrict_commits` → `commits`；`db.QueryRow(...)` → `statDB.QueryRow(...)`
        - SQL3（repo 统计）：`SELECT COUNT(*) FROM costrict_projects` → `SELECT COUNT(*) FROM (SELECT DISTINCT repo_addr, repo_branch FROM commits WHERE repo_addr IS NOT NULL AND repo_addr != '') sub`；`db.QueryRow(...)` → `statDB.QueryRow(...)`

- [x] 3.4 更新 user_handler_v2.go
     【目标对象】`backend/user_handler_v2.go`
     【修改目的】将 SQL 查询和 CRUD 调用从 report 库迁移到 costrict_stat 库
     【修改方式】修改 `listUsersV2` 和 `getUserDetailV2` 两个函数
     【相关依赖】`StatCommit`, `ListStatCommits`, `ListStatTasks`（2.1）
     【修改内容】
        - `listUsersV2`：
          1. SQL1（task 查询）：表名 `costrict_tasks` → `tasks`；`db.Query(...)` → `statDB.Query(...)`
          2. SQL2（commit 查询）：表名 `costrict_commits` → `commits`；`db.Query(...)` → `statDB.Query(...)`
        - `getUserDetailV2`：
          1. `ListCostrictTasks(db, ...)` → `ListStatTasks(statDB, ...)`（注意参数签名变化：ListStatTasks 使用 workDirID 替代 repoID/projectID）
          2. `ListCostrictCommits(db, "", userID, ...)` → `ListStatCommits(statDB, "", "", userID, ...)`
          3. 移除 `GetCostrictProject` 调用
          4. 从 commits 聚合 repo 信息：遍历 commits 提取 DISTINCT (repo_addr, repo_branch) 作为 repos
          5. 响应中 `"projects"` 字段改为 `"repos"`

- [x] 3.5 更新 org_handler_v2.go
     【目标对象】`backend/org_handler_v2.go`
     【修改目的】将 SQL 查询从 report 库迁移到 costrict_stat 库
     【修改方式】修改 `listOrgV2` 和 `getOrgDetailV2` 两个函数中的 SQL 查询及数据库连接
     【相关依赖】costrict_stat 库的 tasks/commits 表
     【修改内容】
        - `listOrgV2`：
          1. SQL1（task 查询）：表名 `costrict_tasks` → `tasks`；`db.Query(...)` → `statDB.Query(...)`
          2. SQL2（commit 查询）：表名 `costrict_commits` → `commits`；`db.Query(...)` → `statDB.Query(...)`
        - `getOrgDetailV2`：
          1. SQL1（task）：`costrict_tasks` → `tasks`；`db.Query(...)` → `statDB.Query(...)`
          2. SQL2（commit）：`costrict_commits` → `commits`；`db.Query(...)` → `statDB.Query(...)`
          3. SQL3（repo task 聚合）：`costrict_tasks` → `tasks`；`db.Query(...)` → `statDB.Query(...)`；聚合键从 `repo_id` 改为 `(repo_addr, repo_branch)`
          4. SQL4（repo commit 聚合）：`costrict_commits` → `commits`；`db.Query(...)` → `statDB.Query(...)`；聚合键从 `repo_id` 改为 `(repo_addr, repo_branch)`

- [x] 3.6 更新 main.go 路由注册
     【目标对象】`backend/main.go`
     【修改目的】移除 project 路由，新增 repo branches 路由
     【修改方式】修改 V2 路由组（L185-218）中的路由注册代码
     【相关依赖】3.2 中新增的 handler
     【修改内容】
        - 移除 V2 路由中 projects 相关端点（L203-208）：`/projects/associate`, `/projects`, `/projects/detail`, `/projects/by-project-id`, `/projects/:repoId`, `/projects/:repoId/manual`
        - 新增 `v2.GET("/repos/branches", listRepoBranchesV2)`

- [x] 3.7 修复 task_real_minutes_test.go 编译错误
     【目标对象】`backend/task_real_minutes_test.go`
     【修改目的】CostrictTaskConversation 被删除后测试需适配
     【修改方式】修改类型引用
     【相关依赖】2.2 完成后
     【修改内容】
        - 将所有 `CostrictTaskConversation` 替换为 `StatTaskConversation`

- [x] 3.8 修复 task_handler_v2_integration_test.go 编译错误
     【目标对象】`backend/task_handler_v2_integration_test.go`
     【修改目的】Costrict* 函数被删除后测试需适配
     【修改方式】修改函数调用和 SQL 表名
     【相关依赖】2.2 完成后
     【修改内容】
        - `UpdateCostrictTaskManual` → `UpdateStatTaskManual`
        - `GetCostrictTask` → `GetStatTask`
        - 所有 `costrict_tasks` 表名改为 `tasks`
        - `testDB` 函数连接的数据库从 `report` 改为 `costrict_stat`

- [x] 3.9 更新 kbcli/pg_writer.go
     【目标对象】`kbcli/pg_writer.go`
     【修改目的】适配后端 StatCommit 结构变更：移除 RepoID 字段，ProjectPath → WorkPath
     【修改方式】修改 `PGCommitData` struct（L241-256）和 `MapCommitDetailsToPG` 函数（L258-294）
     【相关依赖】后端 StatCommit 结构（2.1），后端 batchUpsertCommitsV2 handler（3.1）
     【修改内容】
        - `PGCommitData` struct：移除 `RepoID *string` 字段，将 `ProjectPath *string json:"ProjectPath"` 改为 `WorkPath *string json:"WorkPath"`
        - `MapCommitDetailsToPG` 函数：移除 `pg.RepoID = ptrString(repoID)` 赋值（L267），将 `ProjectPath` 相关逻辑改为 `WorkPath`

### 阶段 4: 前端变更

- [x] 4.1 更新前端路由
     【目标对象】`frontend/src/router/index.js`
     【修改目的】更新 repo 详情路由，移除 project 路由
     【修改方式】修改 routes 数组
     【相关依赖】无
     【修改内容】
        - L6：`/repo/:repoId` → `/repo/:repoAddr/:repoBranch?`（repoBranch 为可选参数）
        - L15：移除 `/project/:projectId` 路由

- [x] 4.2 更新前端 API 函数
     【目标对象】`frontend/src/api/es.js`
     【修改目的】移除 project 相关 API，更新 repo/commit 签名
     【修改方式】修改/删除函数定义
     【相关依赖】后端 API 变更（阶段 3）
     【修改内容】
        - 删除 `getProjectsV2`（L123-125）, `getProjectDetailV2`（L127-129）, `triggerProjectAssociation`（L131-133）, `updateProjectManualV2`（L135-137）, `getProjectDetailByProjectId`（L159-161）
        - 更新 `getRepoDetailV2New`（L167-169）：参数从 `(repoId)` 改为 `(repoAddr, repoBranch, params)`，传 `{ repoAddr, repoBranch, ...params }`
        - 新增 `getRepoBranches(repoAddr)` — GET `/v2/repos/branches?repoAddr=xxx`
        - 更新 `getCommitDetailV2`（L119-121）：移除 repoId 参数，改为 `(commitId)` 无额外 params
        - 更新 `updateCommitManualV2`（L175-177）：移除 repoId 参数，从 `(commitId, repoId, data)` 改为 `(commitId, data)`

- [x] 4.3 重构 Repo 列表页
     【目标对象】`frontend/src/views/ProjectViewV2.vue`
     【修改目的】移除"触发关联分析"，适配新路由跳转
     【修改方式】修改组件模板、script 中的 import 和函数
     【相关依赖】API 变更（4.2）
     【修改内容】
        - 移除"触发关联分析"按钮和 `handleTriggerAssociation` 函数
        - 移除 `triggerProjectAssociation` import
        - 行点击跳转：从 `'/repo/' + encodeURIComponent(row.repo_id)` 改为 `'/repo/' + encodeURIComponent(row.repo_addr) + '/' + encodeURIComponent(row.repo_branch)`
        - 表格列：`repo_id` 改为展示 `repo_addr` + `repo_branch`

- [x] 4.4 重构 Repo 详情页
     【目标对象】`frontend/src/views/ProjectDetailV2.vue`
     【修改目的】支持分支切换、时间筛选、效率评估展示、commits/tasks 列表下钻
     【修改方式】重写组件，适配需求中的 repo 详情页规格
     【相关依赖】`getRepoDetailV2New`, `getRepoBranches` API（4.2）
     【修改内容】
        - 从路由获取 repoAddr + repoBranch（`route.params.repoAddr` + `route.params.repoBranch`）
        - 移除人工调整对话框中的 `updateProjectManualV2` 调用
        - 添加分支下拉（el-select），通过 `getRepoBranches(repoAddr)` 获取分支列表，选择时路由跳转到 `/repo/:repoAddr/:repoBranch`
        - 添加 FilterBar 时间范围选择（startDate/endDate），筛选 start_time 和 end_time
        - 效率评估指标卡片（3 个）：
          1. 古法预估（repo_ancient_minutes）— 橙色
          2. 实际耗时（repo_real_minutes）— 蓝色
          3. 提效比（efficiency_ratio）— 动态颜色（≥300% 绿色，≥150% 蓝色，其他灰色）
        - Commits 表格：列出筛选时间段内的所有 commits，commit_id 可点击跳转 `/commit/:commitId`
        - Tasks 表格：从 commits.task_ids 下钻获取去重后的 tasks 列表，task_id 可点击跳转 `/task/:taskId`
        - 说明：repo 级别效率数据为动态聚合，无独立 manual 端点；用户可通过各 commit 详情页的人工校准间接影响 repo 汇总值

- [x] 4.5 更新 Commit 详情页
     【目标对象】`frontend/src/views/CommitDetailV2.vue`
     【修改目的】移除 repoId 依赖，commit 详情仅凭 commitId 访问
     【修改方式】修改组件中的 API 调用和路由跳转
     【相关依赖】API 变更（4.2）
     【修改内容】
        - loadData：`getCommitDetailV2(commitId, repoId)` 改为 `getCommitDetailV2(commitId)`，移除 `const repoId = route.query.repoId`
        - submitManual：`updateCommitManualV2(commitId, repoId, ...)` 改为 `updateCommitManualV2(commitId, ...)`
        - 仓库链接跳转：从 `'/repo/' + encodeURIComponent(commit.repo_id)` 改为 `'/repo/' + encodeURIComponent(commit.repo_addr) + '/' + encodeURIComponent(commit.repo_branch)`

- [x] 4.6 更新 Commit 列表页
     【目标对象】`frontend/src/views/CommitViewV2.vue`
     【修改目的】移除 repo_id 依赖，更新行点击跳转
     【修改方式】修改组件模板和跳转函数
     【相关依赖】API 变更（4.2）
     【修改内容】
        - 表格列：`prop="repo_id"` 改为显示 `repo_addr + "#" + repo_branch`
        - `handleRowClick`：从 `{ path: '/commit/' + row.commit_id, query: { repoId: row.repo_id } }` 改为 `{ path: '/commit/' + row.commit_id }`，移除 query 中的 repoId

- [x] 4.7 更新 WorkDirDetailV2.vue
     【目标对象】`frontend/src/views/WorkDirDetailV2.vue`
     【修改目的】移除已删除 API 依赖，适配新 API 参数
     【修改方式】修改组件的 import 和 API 调用
     【相关依赖】API 变更（4.2）
     【修改内容】
        - 移除 `updateProjectManualV2` import 和调用
        - 适配 `getRepoDetailV2New` 新参数格式（repoAddr, repoBranch 替代 repoId）

- [x] 4.8 更新 TaskDetailV2.vue
     【目标对象】`frontend/src/views/TaskDetailV2.vue`
     【修改目的】仓库链接跳转从 repo_id 改为 repo_addr/repo_branch
     【修改方式】修改组件模板中的路由跳转
     【相关依赖】路由变更（4.1）
     【修改内容】
        - 仓库链接跳转：从 `'/repo/' + encodeURIComponent(task.repo_id)` 改为 `'/repo/' + encodeURIComponent(task.repo_addr) + '/' + encodeURIComponent(task.repo_branch)`

- [x] 4.9 更新 UserDetailV2.vue
     【目标对象】`frontend/src/views/UserDetailV2.vue`
     【修改目的】仓库跳转从 repo_id 改为 repo_addr/repo_branch，适配后端返回格式
     【修改方式】修改组件模板和数据处理逻辑
     【相关依赖】后端 getUserDetailV2 变更（3.4），路由变更（4.1）
     【修改内容】
        - 表格列中 `repo_id` 改为 `repo_addr`
        - 仓库链接跳转改为新路由格式
        - 数据聚合逻辑适配后端返回的 repos 格式

- [x] 4.10 更新 OrgDetailV2.vue
     【目标对象】`frontend/src/views/OrgDetailV2.vue`
     【修改目的】仓库跳转从 repo_id 改为 repo_addr/repo_branch
     【修改方式】修改组件模板中的表格列和路由跳转
     【相关依赖】路由变更（4.1）
     【修改内容】
        - 表格列 `prop="repo_id"` 改为 `prop="repo_addr"`
        - 仓库链接跳转改为新路由格式
