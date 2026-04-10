# 变更：添加用户生产力看板功能

## 原因
项目经理和个人需要从"用户维度"查看 AI Coding 的产出和提效数据。现有的 `/user-v2` 页面只展示基础统计（task数/commit数/费用），缺乏按天粒度的生产力指标（实际耗时、传统耗时、提效比等），无法满足"看到 AI Coding 提效效果"的核心目标。

## 变更内容
1. **新增 `user_productivity` 表**：在 `costrict_stat` 数据库中新建预聚合表，按 user+日期 维度存储每日生产力数据，包含 task 维度指标（task_diff_lines、tokens、cost、实际耗时、传统耗时、提效比）和 commit 维度指标（commit_diff_lines、各类耗时、提效比）
2. **新增后端 API**：
   - `POST /api/v2/user-productivity/rebuild` — 从 tasks 和 commits 表按 user+日期聚合，写入 user_productivity 表
   - `GET /api/v2/user-productivity` — 用户生产力列表（按 user_id 归并聚合）
   - `GET /api/v2/user-productivity/:userId` — 用户生产力详情（按天明细 + 汇总卡片）
3. **新增前端页面**：
   - `/productivity` — UserProductivity 列表页，以 user_id 视角展示汇总生产力数据，支持多选用户创建虚拟组
   - `/productivity/:userId` — UserProductivityDetail 详情页，顶部汇总卡片 + 按天明细表格，支持日期范围筛选
4. **新增虚拟组功能**：
   - `user_groups` 表存储虚拟组定义（名称 + user_id 列表）
   - `POST/GET/DELETE /api/v2/user-groups` CRUD API
   - `GET /api/v2/user-groups/:groupId` — 实时聚合组内用户的 user_productivity 数据
   - `/productivity/group/:groupId` — 虚拟组详情页，展示组内汇总 + 成员明细

## 影响
- **受影响的规范**：用户数据展示、导航结构
- **受影响的代码**：
  - `init_db_stat.sql`: 新增 `user_productivity` 和 `user_groups` 两张表的建表语句
  - `backend/db.go`: 新增 user_productivity 和 user_groups 的 CRUD 函数
  - `backend/user_productivity_handler.go`（新文件）: rebuild、列表、详情 API handler
  - `backend/user_group_handler.go`（新文件）: 虚拟组 CRUD + 聚合 API handler
  - `backend/main.go`: 注册新路由
  - `frontend/src/api/es.js`: 新增 API 调用函数
  - `frontend/src/router/index.js`: 新增路由定义
  - `frontend/src/views/UserProductivityView.vue`（新文件）: 用户生产力列表页
  - `frontend/src/views/UserProductivityDetail.vue`（新文件）: 用户生产力详情页
  - `frontend/src/views/UserGroupDetail.vue`（新文件）: 虚拟组详情页
