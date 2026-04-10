# 测试方案：enhance-task-v2-reports（任务视图 org 字段 + 用户报表）

## 概述

本次变更涉及 6 个文件，核心功能为：
1. **后端**：`listTasksV2` API 为每条 task 补充 org1-org4 组织字段（从全局 orgMappings 查找）
2. **前端**：TaskViewV2 新增组织列（cascade-org 级联筛选）+ 报表按钮
3. **前端**：KbFilterTable 组件新增 cascade-org 筛选类型
4. **前端**：新增 TaskUserReport 页面（级联组织筛选、6 个指标卡、6 个图表）
5. **前端**：新增 `/task-v2/report/user` 路由

测试策略采用**集成测试优先**：后端以 httptest + 真实 DB 的集成测试覆盖 org 字段逻辑的 3 个分支；前端以 vitest 源码结构测试验证组件配置和页面完整性。共 7 个测试点。

## 测试点列表

### TP-01: listTasksV2 返回 org 字段 — UserID 在 orgMappings 中
- **类型**: integration
- **描述**: UserID 存在且能在全局 orgMappings 中找到映射时，API 返回对应的 org1-org4 值
- **测试场景**:
  1. 插入一条 task（带 user_id）到 DB
  2. 设置 orgMappings 包含该 user_id 的映射（Org1=研发中心, Org2=平台部, Org3=基础架构组, Org4=云原生团队）
  3. GET /api/v2/tasks?startDate=xxx&endDate=xxx
  4. 在响应 data 数组中查找该 task
- **预期结果**: 该 task 的 org1="研发中心", org2="平台部", org3="基础架构组", org4="云原生团队"
- **测试用例文件**: `backend/task_org_fields_integration_test.go` → `TestListTasksV2_OrgFields_UserInMapping`

### TP-02: listTasksV2 返回 org 字段 — UserID 不在 orgMappings 中
- **类型**: integration
- **描述**: UserID 存在但 orgMappings 中无对应记录时，org 字段应为空字符串
- **测试场景**:
  1. 插入一条 task（带 user_id="unknown-user"）到 DB
  2. orgMappings 中不包含该 user_id
  3. GET /api/v2/tasks?startDate=xxx&endDate=xxx
- **预期结果**: 该 task 的 org1="", org2="", org3="", org4=""
- **测试用例文件**: `backend/task_org_fields_integration_test.go` → `TestListTasksV2_OrgFields_UserNotInMapping`

### TP-03: listTasksV2 返回 org 字段 — UserID 为 nil
- **类型**: integration
- **描述**: task 的 user_id 为 NULL（*string 指针为 nil）时，org 字段应为空字符串
- **测试场景**:
  1. 插入一条 task（不设 user_id，即 NULL）到 DB
  2. GET /api/v2/tasks?startDate=xxx&endDate=xxx
- **预期结果**: 该 task 的 org1="", org2="", org3="", org4=""
- **测试用例文件**: `backend/task_org_fields_integration_test.go` → `TestListTasksV2_OrgFields_UserIDNil`

### TP-04: TaskViewV2 org 列配置正确
- **类型**: unit（vitest 源码结构测试）
- **描述**: 验证 TaskViewV2.vue 包含 org_display 列定义，使用 cascade-org 筛选，且自定义插槽正确拼接 org1-org4
- **测试场景**:
  1. 读取 TaskViewV2.vue 源码
  2. 检查 prop: 'org_display' 列存在
  3. 检查 filter: { type: 'cascade-org' }
  4. 检查 #cell-org_display 插槽及 org1-org4 拼接逻辑
- **预期结果**: 所有断言通过
- **测试用例文件**: `frontend/src/views/__tests__/enhance-task-v2-reports.test.js` → `TP-04`

### TP-05: KbFilterTable 支持 cascade-org 筛选类型
- **类型**: unit（vitest 源码结构测试）
- **描述**: 验证 KbFilterTable.vue 的 cascade-org 筛选面板渲染、过滤逻辑、标签显示
- **测试场景**:
  1. 读取 KbFilterTable.vue 源码
  2. 检查 cascade-org 类型判断存在
  3. 检查 4 个级联 select（org1~org4）
  4. 检查 filteredData 中 cascade-org 过滤逻辑按 org1→org4 逐级匹配
  5. 检查 activeFilterTags 中 cascade-org 显示 parts.join('/')
- **预期结果**: 所有断言通过
- **测试用例文件**: `frontend/src/views/__tests__/enhance-task-v2-reports.test.js` → `TP-05`

### TP-06: TaskUserReport 页面结构完整
- **类型**: unit（vitest 源码结构测试）
- **描述**: 验证 TaskUserReport.vue 包含 DateRangePicker、4 级组织筛选、6 个汇总指标卡、6 个图表 ref、正确调用 API
- **测试场景**:
  1. 读取 TaskUserReport.vue 源码
  2. 检查 DateRangePicker、filterOrg1-4、6 个指标卡文案、6 个 chartRef、getUsersV2/getOrgV2 调用
  3. 检查 avgEfficiencyRatio 计算公式 (ancient/real)*100
- **预期结果**: 所有断言通过
- **测试用例文件**: `frontend/src/views/__tests__/enhance-task-v2-reports.test.js` → `TP-06`

### TP-07: 路由配置包含 /task-v2/report/user
- **类型**: unit（vitest 源码结构测试）
- **描述**: 验证路由文件中包含新路由且指向 TaskUserReport 组件
- **测试场景**:
  1. 读取 router/index.js 源码
  2. 检查 /task-v2/report/user 路径存在
  3. 检查路由组件引用 TaskUserReport
- **预期结果**: 所有断言通过
- **测试用例文件**: `frontend/src/views/__tests__/enhance-task-v2-reports.test.js` → `TP-07`

## 关键考虑事项

- **orgMappings 全局变量副作用**：后端测试中需要备份/恢复全局 `orgMappings`，避免影响其他测试用例
- **测试数据清理**：后端测试通过 `defer DELETE` 确保测试数据不残留
- **前端测试环境限制**：vitest 配置为 `environment: 'node'`，无法挂载 Vue 组件，因此采用源码文本匹配方式验证结构
- **已有测试不重复**：`unify_columns_integration_test.go` 已测试 listTasksV2 的 title/cost/tokens 字段，本次仅测试新增的 org 字段
- **keep-alive 无法自动化测试**：App.vue 的 keep-alive 缓存行为需要在浏览器中手动验证

## 测试用例文件清单

- `backend/task_org_fields_integration_test.go` — 后端集成测试（TP-01 ~ TP-03）
- `frontend/src/views/__tests__/enhance-task-v2-reports.test.js` — 前端结构测试（TP-04 ~ TP-07）

## 执行命令

### 后端集成测试
```powershell
cd backend
go test -tags integration -run "TestListTasksV2_OrgFields" -v -count=1
```

### 前端 vitest
```powershell
cd frontend
npx vitest run src/views/__tests__/enhance-task-v2-reports.test.js
```
