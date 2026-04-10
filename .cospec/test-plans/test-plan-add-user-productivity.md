# 测试方案：add-user-productivity（用户生产力看板）

## 概述

本测试方案覆盖 `add-user-productivity` 变更新增的全部功能，包括 `user_productivity` 和 `user_groups` 两张表的 DB 层验证、7 个新 API 端点的集成测试。

**测试策略**：优先使用 API 级集成测试覆盖核心业务逻辑，通过 `httptest` + `gin.TestMode` 模式直接测试 HTTP handler，同时对数据库表结构和索引进行验证。每个测试用例通过 `defer DELETE` 自动清理测试数据，使用 2099 年日期避免与真实数据冲突。

**测试框架**：Go `testing` + `//go:build integration` 构建标签 + `httptest`

## 测试点列表

### DB 层测试

#### 1. UP-DB-01: user_productivity 表结构和索引验证
- **类型**: integration
- **描述**: 验证 user_productivity 表的 22 列类型正确，2 个索引存在
- **测试场景**:
  - 查询 `information_schema.columns` 验证每列的 `data_type`
  - 查询 `pg_indexes` 验证 `idx_user_productivity_user_id` 和 `idx_user_productivity_create_time` 索引存在
- **预期结果**: 所有列类型匹配，索引存在
- **测试函数**: `TestUserProductivityDB_TableAndIndexes`
- **测试用例文件**: `backend/user_productivity_integration_test.go`

#### 2. UP-DB-02: user_groups 表结构、默认值和索引验证
- **类型**: integration
- **描述**: 验证 user_groups 表的 5 列类型正确，默认值正确（`user_ids` 默认为 `[]`），索引存在
- **测试场景**:
  - 查询 `information_schema.columns` 验证列类型
  - 插入只含 `name` 的记录，验证 `user_ids` 默认值为 `[]`
  - 查询 `pg_indexes` 验证 `idx_user_groups_name` 索引存在
- **预期结果**: 列类型正确，`user_ids` 默认值为 `[]`，索引存在
- **测试函数**: `TestUserGroupDB_TableAndIndexes`
- **测试用例文件**: `backend/user_productivity_integration_test.go`

### API 层测试 — User Productivity

#### 3. UP-API-01: POST /api/v2/user-productivity/rebuild 正常聚合场景
- **类型**: integration
- **描述**: 端到端验证 rebuild 功能：插入 tasks + commits 测试数据 → 调用 rebuild API → 验证聚合结果和效率比计算
- **测试场景**:
  - 插入同一用户同一天的 2 条 task（diff_lines=100/50, tokens=5000+2000/3000+1500, cost=0.5/0.3, real_min=30/20, ancient_min=120/80）
  - 插入同一用户同一天的 1 条 commit（diff_lines=200, ancient_min=300, real_min=60）
  - 调用 `POST /api/v2/user-productivity/rebuild?startDate=20990115&endDate=20990115`
  - 验证响应 `{ status: "ok", count: >= 1 }`
  - 验证数据库聚合结果：task_diff_lines=150, upstream_tokens=7000, cost≈0.8
  - 验证效率比计算：task_efficiency_ratio=400 (200/50*100), commit_efficiency_ratio=500 (300/60*100)
- **预期结果**: 聚合数值正确，效率比计算正确，ON CONFLICT upsert 正常
- **测试函数**: `TestRebuildUserProductivity_Normal`
- **测试用例文件**: `backend/user_productivity_integration_test.go`

#### 4. UP-API-02: POST /api/v2/user-productivity/rebuild 参数校验
- **类型**: integration
- **描述**: 验证 rebuild API 的参数校验逻辑
- **测试场景**:
  - 不带任何参数 → 400
  - 只带 startDate → 400
  - startDate 格式错误 → 400
- **预期结果**: 所有异常情况返回 HTTP 400
- **测试函数**: `TestRebuildUserProductivity_MissingParams`
- **测试用例文件**: `backend/user_productivity_integration_test.go`

#### 5. UP-API-03: GET /api/v2/user-productivity 列表汇总 + 分页 + 日期过滤
- **类型**: integration
- **描述**: 验证汇总查询的 GROUP BY 聚合、分页结构、日期过滤功能
- **测试场景**:
  - 插入同一用户两天的 user_productivity 数据
  - 不带日期过滤查询：验证 `{ total, page, pageSize, data }` 结构，验证聚合 `day_count=2, task_count=3, task_diff_lines=150, task_efficiency_ratio=400`
  - 带日期过滤查询（只查一天）：验证 `day_count=1, task_count=2`
- **预期结果**: 汇总数值正确，分页字段完整，日期过滤有效
- **测试函数**: `TestListUserProductivitySummary_Normal`
- **测试用例文件**: `backend/user_productivity_integration_test.go`

#### 6. UP-API-04: GET /api/v2/user-productivity/:userId 详情查询
- **类型**: integration
- **描述**: 验证用户详情接口的 summary/daily/total 返回结构和效率比计算
- **测试场景**:
  - 正常查询：插入一条 user_productivity 数据，验证 `{ summary, daily, total }` 结构
  - 验证 summary 中 `day_count=1, task_diff_lines=100, task_efficiency_ratio=400, commit_efficiency_ratio=500`
  - 查询不存在的用户：返回 200 + 空结果 `total=0`
- **预期结果**: 详情数据正确，效率比重新计算正确，不存在用户返回空结果
- **测试函数**: `TestGetUserProductivityDetail_Normal`, `TestGetUserProductivityDetail_EmptyUserId`
- **测试用例文件**: `backend/user_productivity_integration_test.go`

### API 层测试 — User Groups

#### 7. UP-API-05: POST /api/v2/user-groups 创建用户组 + 参数校验
- **类型**: integration
- **描述**: 验证创建用户组正常流程和参数校验
- **测试场景**:
  - 正常创建：发送 `{ name, user_ids }` → 验证返回 UUID 格式 group_id、name、user_ids
  - 缺少 name（空字符串）→ 400
  - 缺少 user_ids → 400
  - 空 user_ids 数组 → 400
- **预期结果**: 正常创建返回完整对象，异常参数返回 400
- **测试函数**: `TestCreateUserGroup_Normal`, `TestCreateUserGroup_MissingParams`
- **测试用例文件**: `backend/user_productivity_integration_test.go`

#### 8. UP-API-06: GET /api/v2/user-groups 列表查询
- **类型**: integration
- **描述**: 验证用户组列表接口返回 `{ data: [...] }` 结构
- **测试场景**:
  - 先通过 DB 函数创建一个测试组
  - 调用 GET 列表接口
  - 验证 data 数组中包含创建的组
- **预期结果**: 列表包含测试组，字段完整
- **测试函数**: `TestListUserGroups_Normal`
- **测试用例文件**: `backend/user_productivity_integration_test.go`

#### 9. UP-API-07: DELETE /api/v2/user-groups/:groupId 删除 + 404
- **类型**: integration
- **描述**: 验证删除用户组正常流程和不存在返回 404
- **测试场景**:
  - 正常删除：创建组 → DELETE → 验证返回 `{ status: "ok" }` → 验证数据库中已删除
  - 删除不存在的组 → 404
- **预期结果**: 正常删除成功，不存在返回 404
- **测试函数**: `TestDeleteUserGroup_Normal`, `TestDeleteUserGroup_NotFound`
- **测试用例文件**: `backend/user_productivity_integration_test.go`

#### 10. UP-API-08: GET /api/v2/user-groups/:groupId 组详情 + 成员汇总
- **类型**: integration
- **描述**: 验证组详情接口的 `{ group, summary, members }` 结构、组级汇总计算和成员级数据
- **测试场景**:
  - 创建包含 2 个用户的组，为两个用户各插入 user_productivity 数据
  - 调用 GET 组详情接口
  - 验证 members 数组长度为 2
  - 验证组级汇总：`day_count=2, task_count=3, task_diff_lines=300, cost≈1.7, task_efficiency_ratio=400`
  - 查询不存在的组 → 404
- **预期结果**: 组详情返回完整结构，组级汇总正确累加各成员数据，效率比正确，不存在返回 404
- **测试函数**: `TestGetUserGroupDetail_Normal`, `TestGetUserGroupDetail_NotFound`
- **测试用例文件**: `backend/user_productivity_integration_test.go`

## 关键考虑事项

- **测试数据隔离**: 所有测试使用 2099 年日期和带时间戳后缀的 ID，避免与真实数据冲突。每个测试用 `defer DELETE` 清理数据
- **效率比计算验证**: `task_efficiency_ratio = round(task_ancient_minutes / task_real_minutes * 100)`，在 rebuild、list summary、detail、group detail 四个接口中均验证
- **rebuild 核心逻辑**: 测试覆盖了 tasks 聚合 + commits 聚合 + 合并写入 + ON CONFLICT upsert + 效率比计算 的完整流程
- **分页**: list summary 使用内存分页，测试验证了 page/pageSize/total 字段的存在性
- **组级汇总**: group detail 需要遍历组内所有用户的 productivity 数据并累加，测试验证了多用户跨成员的汇总正确性
- **边界条件**: 覆盖了参数缺失、格式错误、不存在的资源（404）、空数组等边界场景
- **全局变量 statDB**: 测试通过 `setupUserProductivityTestRouter` 设置全局 `statDB` 变量，与现有测试模式一致

## 测试用例文件清单

- `backend/user_productivity_integration_test.go`

## 运行命令

```powershell
# 在 backend/ 目录下运行所有 user productivity 相关测试
go test -tags integration -run "TestUserProductivity|TestUserGroup|TestRebuild|TestListUserProductivity|TestGetUserProductivity|TestCreateUserGroup|TestListUserGroups|TestDeleteUserGroup|TestGetUserGroupDetail" -v -count=1

# 运行全部集成测试
go test -tags integration -v -count=1 ./...
```
