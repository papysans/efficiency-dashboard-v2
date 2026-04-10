# 变更：重新设计 User 和 Org 页面，统一用户与虚拟组管理

## 原因
当前用户列表（user-v2）缺少组织维度信息，虚拟组与用户分离展示，且缺乏日期范围筛选能力；组织（org-v2）与用户（user-v2）的筛选体验不一致，详情页设计也不统一。

## 变更内容

### 数据库
- `user_groups` 表新增 `org_name VARCHAR(200)` 字段，支持虚拟组设置自定义组织名称

### 后端
- `listUsersV2`：响应中增加 `org1/org2/org3/org4` 字段（从内存 orgMappings 查），支持 `org1/org2/org3/org4` 筛选参数，并将虚拟组数据（聚合成员统计）合并到响应列表（`is_virtual_group: true` 标记）
- `createUserGroup` / `listUserGroups`：支持 `org_name` 字段的读写
- 新增 `GET /api/v2/group` 接口：按 `org1/org2/org3/org4/startDate/endDate` 参数，返回该组织的 `summary + daily + members`（复用 orgMappings + user_productivity 聚合逻辑）

### 前端
- **`UserViewV2.vue` 重构**：
  - 添加 `FilterBar`（含日期范围选择器 + 4级联组织 Select，复用 OrgViewV2 逻辑）
  - 列顺序调整为：组织、用户名、commit代码量、commit实际耗时、commit提效比、task代码量、task实际耗时、task提效比、token消耗、费用
  - 虚拟组统一到用户列表（用标签/背景色区分，移除独立虚拟组卡片）
  - 新建虚拟组弹窗新增 `org_name` 字段
  - 「组织」列点击跳转 `/group?org1=xxx&org2=xxx`
  - 「用户名」列点击跳转 `/user/:userId`（保持现有逻辑）
- **新建 `GroupView.vue`**：组织详情页，展示统计指标卡 + 提效比趋势图 + 成员列表（设计与 UserDetailV2 一致）
- **`UserDetailV2.vue` 微调**：提取共享统计面板逻辑，与 GroupView 保持一致的展示风格
- **`router/index.js`**：新增 `/group` 路由

## 影响

- **受影响的规范**：用户管理、组织管理、虚拟组管理
- **受影响的代码**：
  - `init_db_stat.sql`：user_groups 表 DDL 变更（新增 org_name 列）
  - `backend/user_group_handler_v2.go`：createUserGroup/listUserGroups 支持 org_name
  - `backend/user_handler_v2.go`：listUsersV2 增加 org 字段、筛选参数、合并虚拟组
  - `backend/org_handler_v2.go`（或新文件）：新增 `/api/v2/group` 接口
  - `backend/main.go`（或路由注册文件）：注册新路由
  - `frontend/src/views/UserViewV2.vue`：整体重构
  - `frontend/src/views/UserDetailV2.vue`：微调统一样式
  - `frontend/src/views/GroupView.vue`：新建
  - `frontend/src/router/index.js`：新增 /group 路由
