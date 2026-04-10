# 测试方案：refactor-repo-pages — Repo 列表与详情页重构

> 生成日期：2026-04-08
> 覆盖模块：`backend/repo_handler_v2.go`、`backend/db.go (ListRepoAggregates)`、`frontend/src/views/RepoViewV2.vue`、`frontend/src/views/RepoDetailV2.vue`、`frontend/src/router/index.js`

---

## 概述

本测试方案覆盖 `refactor-repo-pages` 变更的 5 个测试域：

1. **后端集成测试**（Go `//go:build integration`）：验证 `ListRepoAggregates` 的 `task_count` / `efficiency_ratio` 聚合逻辑，以及 `getRepoDetailV2` 的 reason 汇总逻辑
2. **前端静态分析测试**（vitest `node` 环境）：通过 `readFileSync` 读取 `.vue` 文件内容，用 regex/includes 验证列定义、组件使用、reason 渲染、路由配置、导入依赖

测试策略：
- 后端：复用 `task_handler_v2_integration_test.go` 的 `testDB` + `setupTestRouter` 模式，使用 `httptest` + `gin.TestMode`
- 前端：复用 `commit-view-structure.test.js` 的文件内容静态分析模式

共 **14 个测试点**，约 **52 个测试断言**。

---

## 运行测试

### 后端集成测试

```powershell
cd D:\My\PubCode\kanban\backend
go test -tags=integration -v -run TestRepo ./...
```

### 前端静态分析测试

```powershell
cd D:\My\PubCode\kanban\frontend
npm run test
```

---

## 测试点列表

---

### 测试域 1：Backend ListRepoAggregates task_count / efficiency_ratio

#### 1. ListRepoAggregates task_count 聚合正确性
- **类型**: integration
- **描述**: 验证 `ListRepoAggregates` 函数的 `task_count` 字段能正确处理各种 `task_ids` 边界情况
- **测试场景**:
  1. 插入测试 commit，`task_ids` 为有效 JSON 数组 `["t1","t2","t3"]`
  2. 插入测试 commit，`task_ids` 为 `NULL`
  3. 插入测试 commit，`task_ids` 为 `"null"` 字符串
  4. 插入测试 commit，`task_ids` 为 `"[]"` 空数组
  5. 调用 `ListRepoAggregates(db, "", "")` 查询
  6. 找到测试 repo 的聚合行，验证 `task_count`
- **预期结果**: `task_count` = 3（只有有效 JSON 数组的元素被计数）
- **断言数**: 3
- **测试用例文件**: `backend/repo_handler_v2_integration_test.go`

#### 2. ListRepoAggregates efficiency_ratio 计算正确性
- **类型**: integration
- **描述**: 验证 `efficiency_ratio` 在不同 `sum_ancient_minutes` / `sum_real_minutes` 组合下的计算结果
- **测试场景**:
  1. 插入 commit：`commit_ancient_minutes=480`，`commit_real_minutes=120` → ratio = (480/120)*100 = 400.0
  2. 插入 commit：`commit_ancient_minutes=100`，`commit_real_minutes=NULL` → ratio 应为 nil（real=0 无法除）
  3. 插入 commit：两者均为 NULL → ratio 应为 nil
  4. 调用 `ListRepoAggregates` 查询并验证每组结果
- **预期结果**: 正常情况 ratio=400.0；real 为 0 或 NULL 时 ratio=nil
- **断言数**: 5
- **测试用例文件**: `backend/repo_handler_v2_integration_test.go`

#### 3. ListRepoAggregates 日期过滤
- **类型**: integration
- **描述**: 验证传入 `startTime` / `endTime` 参数时正确过滤 commits
- **测试场景**:
  1. 插入 2 条 commit：一条 commit_time 在 2025-01-15，一条在 2025-06-15
  2. 用 startTime=2025-01-01, endTime=2025-03-01 查询
  3. 验证只返回 1 月的 commit 聚合结果
- **预期结果**: 只包含时间范围内的 commit 聚合
- **断言数**: 3
- **测试用例文件**: `backend/repo_handler_v2_integration_test.go`

---

### 测试域 2：Backend getRepoDetailV2 reason 汇总

#### 4. getRepoDetailV2 返回 reason 字段
- **类型**: integration
- **描述**: 验证 `GET /api/v2/repos/detail` 响应的 `efficiency` 对象包含 `repo_ancient_minutes_reason` 和 `repo_real_minutes_reason` 字段
- **测试场景**:
  1. 插入测试 commit，设置 `commit_ancient_minutes_reason='ancient原因'`，`commit_real_minutes_reason='real原因'`
  2. 发起 `GET /api/v2/repos/detail?repoAddr=test-repo`
  3. 解析响应 JSON，验证 efficiency 子对象包含两个 reason 字段
- **预期结果**: `efficiency.repo_ancient_minutes_reason` 包含 "ancient原因"，`efficiency.repo_real_minutes_reason` 包含 "real原因"
- **断言数**: 5
- **测试用例文件**: `backend/repo_handler_v2_integration_test.go`

#### 5. getRepoDetailV2 reason 优先级：manual > auto
- **类型**: integration
- **描述**: 验证当 commit 同时有自动和手动 reason 时，手动 reason 优先
- **测试场景**:
  1. 插入测试 commit，设置 `commit_ancient_minutes_reason='自动原因'` 且 `commit_ancient_minutes_reason_manual='手动原因'`
  2. 发起 `GET /api/v2/repos/detail?repoAddr=test-repo-priority`
  3. 验证 reason 字段使用手动原因而非自动原因
- **预期结果**: `efficiency.repo_ancient_minutes_reason` 包含 "手动原因" 而非 "自动原因"
- **断言数**: 3
- **测试用例文件**: `backend/repo_handler_v2_integration_test.go`

---

### 测试域 3：Frontend RepoViewV2 KbFilterTable

#### 6. RepoViewV2 列定义：恰好 8 列且 props 正确
- **类型**: unit (静态分析)
- **描述**: 验证 `columns` 数组包含恰好 8 个列定义，且 prop 序列正确
- **测试场景**: 读取 `RepoViewV2.vue` 文件内容，正则提取所有 `prop: 'xxx'`
- **预期结果**: props = `['repo_addr', 'repo_branch', 'commit_count', 'task_count', 'sum_ancient_minutes', 'sum_real_minutes', 'efficiency_ratio', 'start_time']`
- **断言数**: 2
- **测试用例文件**: `frontend/src/views/__tests__/repo-view-structure.test.js`

#### 7. RepoViewV2 过滤器类型序列正确
- **类型**: unit (静态分析)
- **描述**: 验证各列的 `filter.type` 按序为 text, search-select, number, number, number, number, number, date
- **测试场景**: 正则提取所有 `filter: { type: 'xxx'` 中的类型值
- **预期结果**: 类型序列 = `['text', 'search-select', 'number', 'number', 'number', 'number', 'number', 'date']`
- **断言数**: 1
- **测试用例文件**: `frontend/src/views/__tests__/repo-view-structure.test.js`

#### 8. RepoViewV2 KbFilterTable 组件绑定完整性
- **类型**: unit (静态分析)
- **描述**: 验证 KbFilterTable 组件绑定了所有必要属性和事件
- **测试场景**: 检查模板内容包含 ref, :columns, :data, :loading, v-model:page, v-model:pageSize, @row-click, @size-change, @page-change, @filter-change
- **预期结果**: 全部存在
- **断言数**: 8
- **测试用例文件**: `frontend/src/views/__tests__/repo-view-structure.test.js`

#### 9. RepoViewV2 efficiency_ratio 插槽颜色逻辑
- **类型**: unit (静态分析)
- **描述**: 验证 `#cell-efficiency_ratio` 插槽内的 el-tag 颜色阈值判断正确
- **测试场景**: 验证文件内容包含 `#cell-efficiency_ratio`，300→success，150→primary，toFixed(1)% 格式
- **预期结果**: 阈值 300 → success，阈值 150 → primary，使用 toFixed(1)
- **断言数**: 4
- **测试用例文件**: `frontend/src/views/__tests__/repo-view-structure.test.js`

---

### 测试域 4：Frontend RepoDetailV2 reason 显示

#### 10. RepoDetailV2 reason 条件渲染
- **类型**: unit (静态分析)
- **描述**: 验证 RepoDetailV2 中 ancient 和 real 两个 reason 都有 `v-if` 条件渲染与 `el-tooltip` 包裹
- **测试场景**: 检查文件内容包含两组 `v-if="efficiency.repo_xxx_minutes_reason"` + `<el-tooltip` 组合
- **预期结果**: 存在 `v-if="efficiency.repo_ancient_minutes_reason"` 和 `v-if="efficiency.repo_real_minutes_reason"`，均使用 `el-tooltip`
- **断言数**: 4
- **测试用例文件**: `frontend/src/views/__tests__/repo-view-structure.test.js`

#### 11. RepoDetailV2 metric-reason CSS 类
- **类型**: unit (静态分析)
- **描述**: 验证 RepoDetailV2 定义了 `.metric-reason` CSS 类且在模板中使用
- **测试场景**: 检查 `<style` 部分包含 `.metric-reason` 定义，模板中包含 `class="metric-reason"`
- **预期结果**: CSS 定义和模板使用均存在
- **断言数**: 2
- **测试用例文件**: `frontend/src/views/__tests__/repo-view-structure.test.js`

---

### 测试域 5：Router 配置与文件重命名一致性

#### 12. Router 导入路径指向正确的重命名文件
- **类型**: unit (静态分析)
- **描述**: 验证 `router/index.js` 中 Repo 相关路由导入的是 `RepoViewV2.vue` 和 `RepoDetailV2.vue`（而非旧名称 ProjectViewV2 / ProjectDetailV2）
- **测试场景**: 读取 `router/index.js`，验证包含新文件名导入，不包含旧文件名
- **预期结果**: 包含 `RepoViewV2.vue` 和 `RepoDetailV2.vue`，不包含 `ProjectViewV2` 和 `ProjectDetailV2`
- **断言数**: 4
- **测试用例文件**: `frontend/src/views/__tests__/repo-view-structure.test.js`

#### 13. RepoViewV2 和 RepoDetailV2 导入依赖完整性
- **类型**: unit (静态分析)
- **描述**: 验证两个 Vue 文件中所有 `@/` 路径导入均解析到实际存在的文件
- **测试场景**: 提取所有 `from '@/xxx'` 导入路径，逐一检查文件是否存在
- **预期结果**: 所有导入文件均存在（KbFilterTable.vue, FilterBar.vue, api/es.js, utils/formatters.js, utils/date.js）
- **断言数**: 7
- **测试用例文件**: `frontend/src/views/__tests__/repo-view-structure.test.js`

#### 14. RepoViewV2 与 TaskViewV2 模式对齐
- **类型**: unit (静态分析)
- **描述**: 验证 RepoViewV2 与 TaskViewV2 使用相同的 KbFilterTable 插槽和格式化模式
- **测试场景**: 读取 TaskViewV2.vue，对比两者的 `#cell-efficiency_ratio` 插槽、el-tag 使用、toFixed(1) 格式
- **预期结果**: 两者模式一致
- **断言数**: 4
- **测试用例文件**: `frontend/src/views/__tests__/repo-view-structure.test.js`

---

## 关键考虑事项

- 后端测试依赖本地 PostgreSQL 数据库 `costrict_stat`，测试数据在测试结束后通过 `defer` 清理
- 后端测试的 `repo_addr` 使用唯一的测试前缀（如 `test-repo-agg-`）避免与真实数据冲突
- 前端测试纯静态分析，不依赖 DOM 渲染，vitest 在 `node` 环境下运行
- `efficiency_ratio` 阈值 300/150 需要在列表页和详情页保持一致
- `task_count` 的 SQL `jsonb_array_length` 需要正确处理 NULL / "null" / "[]" 边界
- reason 字段的优先级逻辑（manual > auto）在 handler 层实现，需要通过集成测试覆盖

---

## 测试用例文件清单

- `backend/repo_handler_v2_integration_test.go` — 后端集成测试（测试点 1-5，约 19 个断言）
- `frontend/src/views/__tests__/repo-view-structure.test.js` — 前端静态分析测试（测试点 6-14，约 36 个断言）

---

## 测试点统计

| 测试域 | 测试点数 | 断言数 |
|--------|---------|--------|
| 1. ListRepoAggregates 聚合逻辑 | 3 | 11 |
| 2. getRepoDetailV2 reason 汇总 | 2 | 8 |
| 3. RepoViewV2 KbFilterTable | 4 | 15 |
| 4. RepoDetailV2 reason 显示 | 2 | 6 |
| 5. Router 与文件重命名 | 3 | 15 |
| **合计** | **14** | **55** |
