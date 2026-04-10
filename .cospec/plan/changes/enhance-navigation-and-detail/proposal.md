# 变更：增强导航互通、Task详情页、修复数据问题（Phase 9）

## 原因
Playwright 测试 25/25 全部通过，但分析发现以下核心问题：
1. **task_ids_silica 数据格式不一致**：associator 写入的是 `[{task_id, silica}]` 对象数组，seed_data 写的是字符串数组，前端假设是纯数值数组 — 三者互不匹配
2. **org_mapping.csv 与测试数据 user_id 不匹配**：org_mapping 中是 UUID，seed_data 中是 user-001~005
3. **缺少 Task 详情页**：无法查看对话历史（user_input/response_content 链路）
4. **跨页面导航缺失**：project 详情中的参与者无法点击跳转到 user 视图，反之亦然
5. **人工调整 UI 缺失**：project 的 manual 字段无操作入口
6. **搜索过滤缺失**：列表页无搜索框

## 变更内容
- 统一 task_ids_silica 数据格式为 `[{task_id, silica}]` 对象数组，前端全部适配
- 补全 org_mapping.csv 测试数据（覆盖 seed_data 的 5 个用户）
- 新建 Task 详情页（TaskDetailV2.vue）：查看对话历史、关联 project
- 所有视图增加跨页面点击跳转（user→project、project→user、task 可点击查看详情）
- Project 详情增加人工调整入口
- Project/User 列表增加搜索过滤

## 影响
- **受影响的代码**：
  - `seed_data.sql`：修复 task_ids_silica 格式 + 补全 org_mapping
  - `org_mapping.csv`：补充 user-001~005 的映射
  - `frontend/src/views/TaskDetailV2.vue`（新建）：Task 详情页
  - `frontend/src/views/ProjectViewV2.vue`：修复硅比例读取 + 增加跨页面跳转 + 人工调整
  - `frontend/src/views/UserViewV2.vue`：修复硅比例 + 增加跨页面跳转
  - `frontend/src/views/OrgViewV2.vue`：增加跨页面跳转参数
  - `frontend/src/router/index.js`：新增 task 详情路由
  - `frontend/src/App.vue`：菜单微调
  - `frontend/src/api/es.js`：新增 API（如有需要）
