# 测试方案：add-virtual-project（虚拟项目管理功能）

## 概述

本测试方案覆盖 `add-virtual-project` 变更的全部功能，包括：
1. 数据库 `projects` 表结构验证
2. 后端 10 个 API 端点的集成测试（CRUD + 聚合 + 冲突检测）
3. 前端文件结构验证
4. 旧代码残留清理验证

**测试策略**：以集成测试为主，通过 HTTP API 层面的测试覆盖 handler → DB CRUD → 数据校验的完整链路。数据库表结构单独验证。前端和清理项采用文件存在性/内容搜索验证。

**测试框架**：Go 标准 `testing` 包 + `httptest` + `gin.TestMode`，使用 `//go:build integration` 构建标签。

**运行命令**：
```bash
cd backend
go test -tags integration -run TestProject -v
```

---

## 测试点列表

### 分类 1：后端编译验证（已通过 ✅）

> 此项为前置条件，已确认通过。

---

### 分类 2：数据库表结构验证

#### TP-DB-01: projects 表列结构完整性
- **类型**: integration
- **描述**: 验证 `projects` 表存在且包含全部 27 个字段，每个字段的数据类型正确
- **测试场景**: 查询 `information_schema.columns`，逐个校验列名和 `data_type`
- **预期结果**: 全部 27 个字段存在，类型匹配（UUID, VARCHAR, TEXT, JSONB, TIMESTAMPTZ, BIGINT, FLOAT8）
- **Go 测试函数**: `TestProjectDB_TableColumnsExist`
- **测试用例文件**: `backend/project_handler_v2_integration_test.go`

#### TP-DB-02: projects 表默认值
- **类型**: integration
- **描述**: 仅插入 `name` 字段，验证 JSONB 字段默认为 `[]`，数值字段默认为 0
- **测试场景**: INSERT 只含 name，SELECT 查看 repos/task_ids/task_ids_silica/upstream_tokens/downstream_tokens/cost
- **预期结果**: repos=`[]`, task_ids=`[]`, task_ids_silica=`[]`, upstream_tokens=0, downstream_tokens=0, cost=0
- **Go 测试函数**: `TestProjectDB_DefaultValues`
- **测试用例文件**: `backend/project_handler_v2_integration_test.go`

#### TP-DB-03: projects 表索引存在
- **类型**: integration
- **描述**: 验证 `idx_projects_name` 和 `idx_projects_updated_at` 索引存在
- **测试场景**: 查询 `pg_indexes` 表
- **预期结果**: 两个索引均存在
- **Go 测试函数**: `TestProjectDB_IndexesExist`
- **测试用例文件**: `backend/project_handler_v2_integration_test.go`

---

### 分类 3：后端 API 集成测试

#### TP-API-01: POST /api/v2/projects 正常创建
- **类型**: integration
- **描述**: 以合法 name + description 创建项目，验证返回有效 UUID 和正确的 name
- **测试场景**: POST 请求 body `{"name":"test-xxx","description":"..."}`
- **预期结果**: HTTP 200, 返回 JSON 含 `project_id`（36位UUID格式）和 `name` 匹配
- **Go 测试函数**: `TestProjectCreate_Normal`
- **测试用例文件**: `backend/project_handler_v2_integration_test.go`

#### TP-API-02: POST /api/v2/projects 空 name 校验
- **类型**: integration
- **描述**: name 为空字符串时应返回 400 错误
- **测试场景**: POST 请求 body `{"name":"","description":"should fail"}`
- **预期结果**: HTTP 400
- **Go 测试函数**: `TestProjectCreate_EmptyName`
- **测试用例文件**: `backend/project_handler_v2_integration_test.go`

#### TP-API-03: GET /api/v2/projects 列表含计算字段
- **类型**: integration
- **描述**: 创建项目后设置 repos/task_ids/minutes 数据，验证列表 API 返回 repo_count、task_count、efficiency_ratio
- **测试场景**: 
  1. 创建项目
  2. 直接更新 DB 设置 task_ids=["a","b"], repos=[{...}], ancient=480, real=120
  3. GET /api/v2/projects
- **预期结果**: HTTP 200, data 数组中目标项目含 repo_count=1, task_count=2, efficiency_ratio=400.0
- **Go 测试函数**: `TestProjectList_ComputedFields`
- **测试用例文件**: `backend/project_handler_v2_integration_test.go`

#### TP-API-04: GET /api/v2/projects/:projectId 详情 + 404
- **类型**: integration
- **描述**: 获取已存在项目的详情（含 project/commits/tasks/efficiency_ratio 字段）；获取不存在 ID 返回 404
- **测试场景**: 
  - 正常: 创建项目后 GET 详情
  - 404: GET 不存在的 UUID
- **预期结果**: 
  - 正常: HTTP 200, 响应含 project/commits/tasks/efficiency_ratio 四个顶层字段
  - 404: HTTP 404
- **Go 测试函数**: `TestProjectDetail_Normal`, `TestProjectDetail_NotFound`
- **测试用例文件**: `backend/project_handler_v2_integration_test.go`

#### TP-API-05: PUT /api/v2/projects/:projectId 更新项目
- **类型**: integration
- **描述**: 更新项目的 name/description/repos/task_ids/task_ids_silica，验证数据库写入正确；对不存在 ID 返回 404
- **测试场景**: 
  - 正常: 创建→PUT 更新→GET 数据库验证
  - 404: PUT 不存在 UUID
- **预期结果**: 
  - 正常: HTTP 200, DB 中 name/description/repos/task_ids 已更新
  - 404: HTTP 404
- **Go 测试函数**: `TestProjectUpdate_Normal`, `TestProjectUpdate_NotFound`
- **测试用例文件**: `backend/project_handler_v2_integration_test.go`

#### TP-API-06: DELETE /api/v2/projects/:projectId 删除项目
- **类型**: integration
- **描述**: 删除已存在项目验证 GetProject 返回 nil；删除不存在项目返回错误
- **测试场景**: 
  - 正常: 创建→DELETE→GetProject 验证
  - 不存在: DELETE 不存在 UUID
- **预期结果**: 
  - 正常: HTTP 200, GetProject 返回 nil
  - 不存在: HTTP 500（DeleteProject 返回 error）
- **Go 测试函数**: `TestProjectDelete_Normal`, `TestProjectDelete_NotFound`
- **测试用例文件**: `backend/project_handler_v2_integration_test.go`

#### TP-API-07: PUT /api/v2/projects/:projectId/manual 手动修正字段
- **类型**: integration
- **描述**: 
  - 合法字段（project_ancient_minutes_manual 等 8 个 allowed 字段）更新成功
  - 非法字段（如 name）被拒绝
- **测试场景**: 
  - 合法: PUT manual 端点，传入 allowed 字段
  - 非法: PUT manual 端点，传入 `{"name":"hacked"}`
- **预期结果**: 
  - 合法: HTTP 200, DB 中 manual 字段已更新
  - 非法: HTTP 500（UpdateProjectManual 返回 "不允许更新字段" 错误），name 未变
- **Go 测试函数**: `TestProjectManualUpdate_ValidFields`, `TestProjectManualUpdate_DisallowedField`
- **测试用例文件**: `backend/project_handler_v2_integration_test.go`

#### TP-API-08: POST /api/v2/projects/:projectId/tasks 添加 tasks 去重
- **类型**: integration
- **描述**: 分两次添加 task_ids，验证去重逻辑（重复 ID 不追加，silica 保留首次值）；对不存在项目返回 404
- **测试场景**: 
  1. 第一次: task_ids=["t1","t2"], silica=[1.0,0.8]
  2. 第二次: task_ids=["t2","t3"], silica=[0.5,0.9]
  3. 验证结果: task_ids=["t1","t2","t3"], silica=[1.0,0.8,0.9]
- **预期结果**: task_ids 长度=3，t2 的 silica 保持 0.8
- **Go 测试函数**: `TestProjectAddTasks_Deduplication`, `TestProjectAddTasks_NotFound`
- **测试用例文件**: `backend/project_handler_v2_integration_test.go`

#### TP-API-09: Repo 添加与删除 + 索引越界
- **类型**: integration
- **描述**: 
  - 添加 repo 过滤规则后验证 repos 数组增长
  - 按 index 删除 repo 后验证剩余正确
  - index 越界（空 repos 删 index=0, 负数 index）返回 400
- **测试场景**: 
  - 添加: POST repos 端点
  - 删除: 预设 2 个 repos → DELETE index=0 → 验证只剩第二个
  - 越界: DELETE index=0 (repos 为空), DELETE index=-1
- **预期结果**: 
  - 添加: HTTP 200, repos 长度+1
  - 删除: HTTP 200, 正确移除目标 repo
  - 越界: HTTP 400
- **Go 测试函数**: `TestProjectAddRepo_Normal`, `TestProjectRemoveRepo_Normal`, `TestProjectRemoveRepo_IndexOutOfBounds`
- **测试用例文件**: `backend/project_handler_v2_integration_test.go`

#### TP-API-10: POST /api/v2/projects/check-conflicts 冲突检测
- **类型**: integration
- **描述**: 
  - 无冲突: 传入不存在的 commit_ids → conflicts 为空
  - 有冲突: 在 DB 插入 commit + 创建关联该 repo 的项目 → 检测到冲突
- **测试场景**: 
  - 无冲突: POST check-conflicts，commit_ids 为不存在的 ID
  - 有冲突: 
    1. INSERT commit (commit_id, repo_addr, repo_branch, commit_time)
    2. 创建 project 并 UPDATE repos 指向该 repo
    3. POST check-conflicts 传入该 commit_id
- **预期结果**: 
  - 无冲突: HTTP 200, conflicts 为空/null
  - 有冲突: HTTP 200, conflicts 数组包含 {commit_id, project_id, project_name}
- **Go 测试函数**: `TestProjectCheckConflicts_NoConflict`, `TestProjectCheckConflicts_WithConflict`
- **测试用例文件**: `backend/project_handler_v2_integration_test.go`

---

### 分类 4：前端文件结构验证

#### TP-FE-01: ProjectViewV2.vue 存在且结构合法
- **类型**: e2e (文件验证)
- **描述**: 确认 `frontend/src/views/ProjectViewV2.vue` 存在，包含 `<template>`、`<script>` 标签
- **测试场景**: 检查文件存在 + 内容包含 Vue SFC 结构标签
- **预期结果**: 文件存在，包含 `<template>` 和 `<script setup>`
- **验证命令**: 
  ```powershell
  Test-Path frontend/src/views/ProjectViewV2.vue
  rg "<template>" frontend/src/views/ProjectViewV2.vue
  rg "<script" frontend/src/views/ProjectViewV2.vue
  ```

#### TP-FE-02: ProjectDetailV2.vue 存在且结构合法
- **类型**: e2e (文件验证)
- **描述**: 确认 `frontend/src/views/ProjectDetailV2.vue` 存在，包含 `<template>`、`<script>` 标签
- **测试场景**: 同 TP-FE-01
- **预期结果**: 文件存在，包含 `<template>` 和 `<script setup>`
- **验证命令**: 
  ```powershell
  Test-Path frontend/src/views/ProjectDetailV2.vue
  rg "<template>" frontend/src/views/ProjectDetailV2.vue
  ```

#### TP-FE-03: es.js 包含全部 10 个 Project API 方法
- **类型**: e2e (文件验证)
- **描述**: 确认 `frontend/src/api/es.js` 包含所有 project 相关的 API 封装方法
- **测试场景**: 搜索 es.js 中的函数名
- **预期结果**: 包含以下 10 个导出函数：createProject, getProjects, getProjectDetail, updateProject, deleteProject, updateProjectManual, addTasksToProject, addRepoToProject, removeRepoFromProject, checkProjectConflicts
- **验证命令**: 
  ```powershell
  rg "export const (createProject|getProjects|getProjectDetail|updateProject|deleteProject|updateProjectManual|addTasksToProject|addRepoToProject|removeRepoFromProject|checkProjectConflicts)" frontend/src/api/es.js
  ```

#### TP-FE-04: 路由已注册 project-v2 和 project/:projectId
- **类型**: e2e (文件验证)
- **描述**: 确认 `frontend/src/router/index.js` 包含 project 相关路由
- **测试场景**: 搜索 router 文件
- **预期结果**: 包含 `/project-v2` → ProjectViewV2 和 `/project/:projectId` → ProjectDetailV2
- **验证命令**: 
  ```powershell
  rg "project" frontend/src/router/index.js
  ```

---

### 分类 5：旧代码残留清理验证

#### TP-CLEAN-01: ProjectPanel.vue 已删除
- **类型**: e2e (文件验证)
- **描述**: 确认 `frontend/src/views/ProjectPanel.vue` 不存在
- **测试场景**: 检查文件是否存在
- **预期结果**: 文件不存在
- **验证命令**: 
  ```powershell
  Test-Path frontend/src/views/ProjectPanel.vue  # 应返回 False
  ```

#### TP-CLEAN-02: V2 后端代码无 project_count 残留
- **类型**: e2e (代码搜索)
- **描述**: 确认 V2 handler 代码中已将 `project_count` 重命名为 `work_dir_count`
- **测试场景**: 在后端 Go 文件中搜索 `project_count`
- **预期结果**: 无匹配结果
- **验证命令**: 
  ```powershell
  rg "project_count" backend/ -g "*.go"  # 应无结果
  ```

#### TP-CLEAN-03: V2 前端代码无 project_count 残留
- **类型**: e2e (代码搜索)
- **描述**: 确认前端 Vue 文件中已将 `project_count` 重命名为 `work_dir_count`
- **测试场景**: 在前端 Vue 文件中搜索 `project_count`
- **预期结果**: 无匹配结果
- **验证命令**: 
  ```powershell
  rg "project_count" frontend/src/ -g "*.vue"  # 应无结果
  ```

---

## 关键考虑事项

1. **异步聚合计算**: `updateProjectV2`、`addTasksToProjectV2`、`addRepoToProjectV2`、`removeRepoFromProjectV2` 中的 `recalculateProjectAggregates` 在 goroutine 中异步执行。集成测试仅验证 HTTP 响应的同步部分（status + 直接 DB 写入），不验证异步聚合结果。若需测试聚合正确性，需直接调用 `recalculateProjectAggregates` 并等待完成。

2. **测试数据隔离**: 每个测试用例使用 timestamp 后缀生成唯一标识符，通过 `defer DELETE` 清理测试数据，避免测试间干扰。

3. **全局变量依赖**: handler 函数依赖全局变量 `statDB`，测试中通过 `setupProjectTestRouter` 统一设置。`testDB(t)` 辅助函数已在 `task_handler_v2_integration_test.go` 中定义，不可重复定义。

4. **DeleteProject 的 404 行为**: 当前实现中 `deleteProjectV2` 不预检查项目是否存在，直接调用 `DeleteProject` 并在 rows==0 时返回 error，handler 返回 500 而非 404。测试用例据此设计预期为 HTTP 500。

5. **UpdateProjectManual 的安全性**: 该函数使用白名单机制（`allowed` map），只允许更新 8 个 manual 字段。测试覆盖了合法字段更新成功和非法字段被拒绝两种场景。

6. **前端验证局限**: 前端测试仅验证文件存在性和关键内容，不验证运行时渲染。完整的前端验证需启动 dev server 后进行 E2E 测试。

---

## 测试用例文件清单

- `backend/project_handler_v2_integration_test.go` — 16 个 Go 集成测试函数，覆盖 DB 表结构 + 全部 10 个 API 端点

### Go 测试函数与测试点映射

| Go 测试函数 | 测试点 ID | 说明 |
|---|---|---|
| `TestProjectDB_TableColumnsExist` | TP-DB-01 | 27 列存在且类型正确 |
| `TestProjectDB_DefaultValues` | TP-DB-02 | JSONB/数值字段默认值 |
| `TestProjectDB_IndexesExist` | TP-DB-03 | 2 个索引存在 |
| `TestProjectCreate_Normal` | TP-API-01 | 正常创建返回 UUID |
| `TestProjectCreate_EmptyName` | TP-API-02 | 空 name → 400 |
| `TestProjectList_ComputedFields` | TP-API-03 | 列表含 repo_count/task_count/efficiency_ratio |
| `TestProjectDetail_Normal` | TP-API-04 | 详情含 project/commits/tasks |
| `TestProjectDetail_NotFound` | TP-API-04 | 不存在 → 404 |
| `TestProjectUpdate_Normal` | TP-API-05 | 更新 name/desc/repos/task_ids |
| `TestProjectUpdate_NotFound` | TP-API-05 | 不存在 → 404 |
| `TestProjectDelete_Normal` | TP-API-06 | 删除成功 |
| `TestProjectDelete_NotFound` | TP-API-06 | 不存在 → 500 |
| `TestProjectManualUpdate_ValidFields` | TP-API-07 | 合法 manual 字段更新 |
| `TestProjectManualUpdate_DisallowedField` | TP-API-07 | 非法字段被拒绝 |
| `TestProjectAddTasks_Deduplication` | TP-API-08 | task_ids 去重追加 |
| `TestProjectAddTasks_NotFound` | TP-API-08 | 项目不存在 → 404 |
| `TestProjectAddRepo_Normal` | TP-API-09 | 添加 repo |
| `TestProjectRemoveRepo_Normal` | TP-API-09 | 删除 repo 正常 |
| `TestProjectRemoveRepo_IndexOutOfBounds` | TP-API-09 | index 越界 → 400 |
| `TestProjectCheckConflicts_NoConflict` | TP-API-10 | 无冲突 |
| `TestProjectCheckConflicts_WithConflict` | TP-API-10 | 有冲突检测 |
