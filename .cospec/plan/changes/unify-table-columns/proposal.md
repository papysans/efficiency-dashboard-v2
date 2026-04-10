# 变更：统一 Commit/Task 表格列结构

## 原因
三个页面（commit-v2、task-v2、repo详情）的表格列结构不一致，缺少关键字段（ID、费用、Tokens消耗），需要统一为相同的10列结构，便于对比和阅读。

## 变更内容
- 统一三个页面的 Commit/Task 表格为相同的10列：ID、时间、用户、说明、代码行数、实际耗时、传统开发时长预估、提效比、费用、Tokens消耗
- 后端 `listCommitsV2` 接口增加从关联 Tasks 聚合的 cost、tokens 数据
- 后端 `listTasksV2` 接口补充返回遗漏的 title 字段
- 后端 `getRepoDetailV2` 接口为每个 commit 附加聚合的 cost/tokens 数据
- 前端三个页面的表格列定义统一调整

## 影响
- **受影响的规范**：Commit 列表、Task 列表、仓库详情
- **受影响的代码**：
    - `backend/db.go`: 新增 `BatchGetStatTasks` 批量查询函数
    - `backend/commit_handler_v2.go:listCommitsV2`: 聚合关联 tasks 的 cost/tokens 到 commit 响应
    - `backend/task_handler_v2.go:listTasksV2`: gin.H map 中补充 title 字段
    - `backend/repo_handler_v2.go:getRepoDetailV2`: 为 commits 附加聚合的 cost/tokens
    - `frontend/src/views/CommitViewV2.vue`: 列定义改为统一10列
    - `frontend/src/views/TaskViewV2.vue`: 列定义改为统一10列
    - `frontend/src/views/RepoDetailV2.vue`: Commits/Tasks 表格列统一为10列并对齐
