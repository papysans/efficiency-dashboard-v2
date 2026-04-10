# 变更：合并用户生产力页面到用户页面

## 原因
当前用户相关功能分散在两套页面（`/user-v2` 基础统计 + `/productivity` 生产力提效），数据来源也是两套（users API 实时聚合 vs user-productivity API 预聚合表），导致体验割裂。需要合并为统一的 `/user-v2` 入口，参考 TaskViewV2 的 KbFilterTable 实现方式，并确保有数据可演示。

## 变更内容
1. **重写 UserViewV2.vue**：参考 TaskViewV2 使用 KbFilterTable 组件，数据源改为 user-productivity API，增加 rebuild 按钮、多选创建虚拟组功能、虚拟组列表区域
2. **重写 UserDetailV2.vue**：合并 UserProductivityDetail 的能力（汇总卡片 + 按天明细表格 + 趋势图），数据源改为 user-productivity detail API
3. **调整 UserGroupDetail.vue**：路由从 `/productivity/group/:groupId` 改为 `/user/group/:groupId`，内部跳转链接改为 `/user/xxx`
4. **移除 productivity 独立页面**：删除 UserProductivityView.vue 和 UserProductivityDetail.vue
5. **更新路由**：移除 `/productivity` 相关路由，新增 `/user/group/:groupId`
6. **移除导航菜单"生产力"项**
7. **后端合并**：将 `listUsersV2` 改为从 user_productivity 表聚合（替代双表实时聚合），将 `getUserDetailV2` 改为返回 user_productivity 按天明细 + 汇总；移除 `user_productivity_handler_v2.go` 中重复的列表/详情 handler（保留 rebuild）
8. **生成演示数据**：调用 rebuild API 确保有数据

## 影响
- **受影响的代码**：
  - `backend/user_handler_v2.go`: 重写 `listUsersV2` 和 `getUserDetailV2`，改为读取 user_productivity 表
  - `backend/main.go`: 移除 `/v2/user-productivity` 列表和详情路由（保留 rebuild），移除 `/v2/user-productivity/:userId`
  - `frontend/src/views/UserViewV2.vue`: 重写为 KbFilterTable 模式
  - `frontend/src/views/UserDetailV2.vue`: 重写为汇总卡片 + 按天明细
  - `frontend/src/views/UserGroupDetail.vue`: 路由跳转链接更新
  - `frontend/src/views/UserProductivityView.vue`: 删除
  - `frontend/src/views/UserProductivityDetail.vue`: 删除
  - `frontend/src/router/index.js`: 路由调整
  - `frontend/src/App.vue`: 移除"生产力"菜单项
  - `frontend/src/api/es.js`: 清理 API 函数
