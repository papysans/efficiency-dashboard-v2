# 变更：新增 Commit 列表/详情页 + 字段重命名 + 全局 ID path-safe 格式化

## 原因
项目缺少独立的 Commit 视图页面，commit 数据仅嵌入在项目详情中展示。需要新增 commit 列表页和详情页，同时将 commit 表中的预估字段改为 commit_ 前缀以语义化，并预留 task 关联和实际耗时字段。此外 repo_id/project_id 当前格式包含冒号、反斜杠等特殊字符，不适合 URL 传递和目录创建，需全局改为 path-safe 格式。

## 变更内容

### 1. 全局 ID path-safe 格式化
- repo_id 和 project_id 改为只包含小写字母、数字、`-`、`.` 的 path-safe 格式
- 转换规则：原始值转小写，非(小写字母/数字/-/.)的字符替换为 `-`，去除首尾 `-`，合并连续 `-`
- 示例：`https://github.com/zgsm-ai/costrict.git#main` → `https-github.com-zgsm-ai-costrict.git-main`
- 示例：`797e102c29:d:\projects\creditControl\credit-sentinel` → `797e102c29-d-projects-creditcontrol-credit-sentinel`
- 涉及：kbcli 数据导入逻辑修改 + 历史数据迁移 SQL + 所有依赖 repo_id/project_id 的关联逻辑

### 2. costrict_commits 表字段变更
- 重命名：`task_ancient_minutes` → `commit_ancient_minutes`，`task_ancient_minutes_reason` → `commit_ancient_minutes_reason`
- 新增字段（预留）：`commit_ancient_minutes_manual`、`commit_ancient_minutes_reason_manual`、`task_ids`(JSONB)、`task_ids_silica`(JSONB)、`commit_real_minutes`、`commit_real_minutes_reason`、`commit_real_minutes_manual`、`commit_real_minutes_reason_manual`

### 3. 后端 commit API 增强
- commit 详情 API 增加返回关联的 task 简要信息（如果 task_ids 有数据）
- 新增 PUT `/api/v2/commits/:commitId/manual` 人工修正 API
- commit_real_minutes 计算逻辑（当 task_ids 和 task_ids_silica 有数据时）

### 4. 前端新增 Commit 列表页（CommitViewV2.vue）
- 参考 TaskViewV2.vue 的结构：FilterBar + el-table + 分页 + 图表
- 表格列：commit_id、用户、仓库、提交时间、Diff行数、古法预估、实际耗时
- 支持按日期范围、repoId、userId 过滤
- 行点击跳转详情

### 5. 前端新增 Commit 详情页（CommitDetailV2.vue）
- 参考 TaskDetailV2.vue 的结构
- 元信息卡片：commit_id、用户、仓库、分支、提交时间、Diff行数、古法预估、实际耗时、提效比
- 关联 Tasks 区域（如果 task_ids 有数据则展示关联的 task 列表，可点击跳转 task 详情）
- 人工调整对话框

### 6. 路由和导航
- 新增路由：`/commit-v2`（列表）、`/commit/:commitId`（详情，query 带 repoId）
- 侧边菜单新增"提交"入口

### 7. 更新设计文档v2.md#commit数据

## 影响
- **受影响的代码**：
  - `init_db.sql`：ALTER TABLE 字段重命名+新增，历史数据 repo_id/project_id 迁移
  - `backend/db.go`：CostrictCommit struct 重命名+新增字段，SQL 更新，新增 manual 函数
  - `backend/commit_handler_v2.go`：详情 API 增强，新增 manual handler，commit_real_minutes 计算
  - `backend/main.go`：新增路由
  - `kbcli/raw_parser.go`：repo_id/project_id 生成逻辑改为 path-safe
  - `kbcli/pg_writer.go`：commit 字段映射更新
  - `frontend/src/views/CommitViewV2.vue`：新建
  - `frontend/src/views/CommitDetailV2.vue`：新建
  - `frontend/src/router/index.js`：新增路由
  - `frontend/src/api/es.js`：新增 API
  - `frontend/src/App.vue` 或布局组件：菜单新增"提交"
  - `设计文档v2.md`：更新 commit 数据章节
