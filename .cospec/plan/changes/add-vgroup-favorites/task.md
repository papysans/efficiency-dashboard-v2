# 虚拟组与收藏后端功能

## 阶段 1：数据库建表

- [x] 1.1 在 `init_db.sql` 末尾追加 virtual_groups 和 favorites 两张表的 CREATE TABLE 语句
- [x] 1.2 执行追加的 SQL 在 PostgreSQL 中创建表

## 阶段 2：后端 API 实现

- [x] 2.1 新建 `backend/vgroup_handler.go`，实现虚拟组 CRUD（POST/GET/DELETE /api/virtual-groups，GET /api/virtual-groups/:id/aggregate）
- [x] 2.2 在同一文件中实现收藏 CRUD（POST/GET/DELETE /api/favorites）
- [x] 2.3 在 `backend/main.go` 中注册所有新路由

## 阶段 3：编译验证

- [x] 3.1 执行 `go build ./...` 确保编译通过

## 阶段 4：前端 API 层

- [x] 4.1 在 `frontend/src/api/es.js` 追加 7 个 API 函数：createVirtualGroup, getVirtualGroups, deleteVirtualGroup, getVirtualGroupAggregate, addFavorite, getFavorites, removeFavorite

## 阶段 5：Dashboard 聚合 Tab 多选 + 创建虚拟组

- [x] 5.1 Dashboard.vue 聚合 Tab 的 el-table 增加 type="selection" 多选列（仅 aggregate Tab 时显示）
- [x] 5.2 选中行 > 0 时，筛选区显示"创建虚拟组（已选N项）"按钮
- [x] 5.3 点击按钮弹出 el-dialog，输入虚拟组名称，调用 createVirtualGroup API，成功后 ElMessage 提示

## 阶段 6：ProjectPanel 收藏 + 虚拟组显示

- [x] 6.1 筛选区增加 el-switch "仅显示收藏"
- [x] 6.2 表格新增第一列：收藏星标图标（已收藏实心金色，未收藏空心，点击切换）
- [x] 6.3 虚拟组行显示 [虚拟组] el-tag type="warning" 标签 + 不同底色
- [x] 6.4 开启"仅显示收藏"时：加载收藏列表，过滤数据，虚拟组收藏调用聚合 API 获取数据混入表格

## 阶段 7：RepoPanel 收藏 + 虚拟组显示

- [x] 7.1 RepoPanel.vue 增加收藏开关、星标列、虚拟组标记、仅显示收藏逻辑（dimension=repo）

## 阶段 8：UserPanel 收藏 + 虚拟组显示

- [x] 8.1 UserPanel.vue 增加收藏开关、星标列、虚拟组标记、仅显示收藏逻辑（dimension=user）

## 阶段 9：OrgPanel 收藏 + 虚拟组显示

- [x] 9.1 OrgPanel.vue 增加收藏开关、星标列、虚拟组标记、仅显示收藏逻辑（dimension=org，需兼容层级下钻逻辑）

## 阶段 10：构建验证

- [x] 10.1 执行 `npm run build` 确保前端构建通过
