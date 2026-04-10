# 测试方案：统一三页面表格为10列结构 (unify-table-columns)

## 概述

本次变更将 CommitViewV2、TaskViewV2、RepoDetailV2 三个页面统一为相同的 10 列表格结构：
**ID → 时间 → 用户 → 说明 → 代码行数 → 实际耗时 → 传统开发时长预估 → 提效比 → 费用 → Tokens消耗**

测试策略分两层：
1. **后端集成测试**（Go + 真实数据库）：验证 API 返回的新字段（cost、upstream_tokens、downstream_tokens、title）正确聚合
2. **前端结构验证**（Vitest 静态分析）：验证三个 Vue 文件的列定义结构、列数、列顺序、minWidth 一致性

总计 **18 个测试点**，约 **48 个断言**。

---

## 测试点列表

---

### 后端集成测试（Go, build tag: integration）

---

### TP-01: BatchGetStatTasks 空输入返回空 map

- **类型**: integration
- **描述**: 调用 `BatchGetStatTasks(db, []string{})` 传入空切片，验证返回空 map 且无错误
- **测试场景**:
  1. 获取测试数据库连接
  2. 调用 `BatchGetStatTasks(tdb, []string{})`
  3. 检查返回值
- **预期结果**:
  - `err == nil`
  - `len(result) == 0`
- **断言数**: 2
- **测试用例文件**: `backend/unify_columns_integration_test.go`
- **实现备注**: 直接调用函数，不需要插入测试数据。参考 `task_handler_v2_integration_test.go` 中的 `testDB(t)` 模式。

---

### TP-02: BatchGetStatTasks 有效 taskIDs 返回正确 map

- **类型**: integration
- **描述**: 插入带有 cost、upstream_tokens、downstream_tokens、title 的测试 task 记录，验证批量查询返回完整字段
- **测试场景**:
  1. 插入 2 条 tasks 记录，分别设置 `cost=0.05`、`upstream_tokens=1000`、`downstream_tokens=500`、`title='测试任务A'` 和 `cost=0.10`、`upstream_tokens=2000`、`downstream_tokens=800`、`title='测试任务B'`
  2. 调用 `BatchGetStatTasks(tdb, []string{"test-batch-001", "test-batch-002"})`
  3. 验证 map 长度和各字段值
  4. 清理测试数据（defer DELETE）
- **预期结果**:
  - `err == nil`
  - `len(result) == 2`
  - `result["test-batch-001"].Cost != nil && *result["test-batch-001"].Cost == 0.05`
  - `result["test-batch-001"].UpstreamTokens != nil && *result["test-batch-001"].UpstreamTokens == 1000`
  - `result["test-batch-001"].DownstreamTokens != nil && *result["test-batch-001"].DownstreamTokens == 500`
  - `result["test-batch-001"].Title != nil && *result["test-batch-001"].Title == "测试任务A"`
- **断言数**: 6
- **测试用例文件**: `backend/unify_columns_integration_test.go`
- **实现备注**: 使用唯一前缀如 `test-batch-unify-` + timestamp 防止 ID 冲突。INSERT 语句需包含 `cost`, `upstream_tokens`, `downstream_tokens`, `title` 列。参考 StatTask 结构体（db.go:639-672）。

---

### TP-03: BatchGetStatTasks 不存在的 taskIDs 返回空 map

- **类型**: integration
- **描述**: 传入一组不存在的 taskID，验证返回空 map 且无错误
- **测试场景**:
  1. 调用 `BatchGetStatTasks(tdb, []string{"nonexistent-xyz-001", "nonexistent-xyz-002"})`
  2. 检查返回值
- **预期结果**:
  - `err == nil`
  - `len(result) == 0`
- **断言数**: 2
- **测试用例文件**: `backend/unify_columns_integration_test.go`
- **实现备注**: 不需要插入任何测试数据。

---

### TP-04: listCommitsV2 响应包含 cost/upstream_tokens/downstream_tokens 字段

- **类型**: integration
- **描述**: 插入一条 commit（带 task_ids + task_ids_silica）和对应的 task（带 cost/tokens），调用 GET /api/v2/commits 验证聚合结果
- **测试场景**:
  1. 插入 1 条 task：`task_id="test-commit-tokens-t1"`, `cost=0.08`, `upstream_tokens=5000`, `downstream_tokens=2000`
  2. 插入 1 条 commit：`task_ids=["test-commit-tokens-t1"]`, `task_ids_silica=[0.6]`, `commit_time` 在测试日期范围内
  3. 创建 gin 路由并注册 `listCommitsV2`，需要设置全局 `statDB` 变量
  4. GET `/api/v2/commits?startDate=YYYY-MM-DD&endDate=YYYY-MM-DD`
  5. 解析 JSON 响应中 `data` 数组的第一条
  6. 清理测试数据
- **预期结果**:
  - HTTP 200
  - `data[0]["cost"]` 为 float64，值 == 0.08（task cost 直接累加，不乘 silica）
  - `data[0]["upstream_tokens"]` 为 int64，值 == `round(5000 * 0.6)` = 3000
  - `data[0]["downstream_tokens"]` 为 int64，值 == `round(2000 * 0.6)` = 1200
- **断言数**: 4
- **测试用例文件**: `backend/unify_columns_integration_test.go`
- **实现备注**: 需要创建 `setupCommitTestRouter(t)` 辅助函数，参考 `setupRepoTestRouter` 模式（repo_handler_v2_integration_test.go:18-36）。路由注册 `v2.GET("/commits", listCommitsV2)`。注意 cost 是直接从 task.Cost 累加，tokens 需要乘以 silica 比例。

---

### TP-05: listCommitsV2 无关联 task 时 cost/tokens 为零值

- **类型**: integration
- **描述**: 插入一条 task_ids 为空的 commit，验证 cost=0, upstream_tokens=0, downstream_tokens=0
- **测试场景**:
  1. 插入 1 条 commit：`task_ids=NULL` 或 `task_ids='[]'`
  2. GET `/api/v2/commits?startDate=...&endDate=...`
  3. 检查 data[0] 中的 cost/tokens 字段
- **预期结果**:
  - `data[0]["cost"]` == 0（float64 零值）
  - `data[0]["upstream_tokens"]` == 0（int64 零值）
  - `data[0]["downstream_tokens"]` == 0（int64 零值）
- **断言数**: 3
- **测试用例文件**: `backend/unify_columns_integration_test.go`
- **实现备注**: 验证空 task_ids 场景下不会 panic 且零值正确返回。

---

### TP-06: listTasksV2 响应包含 title 字段

- **类型**: integration
- **描述**: 插入带 title 的 task 记录，调用 GET /api/v2/tasks 验证 title 字段存在
- **测试场景**:
  1. 插入 1 条 task：`title='统一列结构测试'`, `start_time` 在测试日期范围内
  2. 创建 gin 路由注册 `listTasksV2`
  3. GET `/api/v2/tasks?startDate=YYYY-MM-DD&endDate=YYYY-MM-DD`
  4. 解析 data 数组查找匹配的 task
- **预期结果**:
  - HTTP 200
  - 匹配 task 的 `title` 字段 == `"统一列结构测试"`
  - `cost` 字段存在（可能为 nil 或 float64）
  - `upstream_tokens` 字段存在
  - `downstream_tokens` 字段存在
- **断言数**: 5
- **测试用例文件**: `backend/unify_columns_integration_test.go`
- **实现备注**: 需要 `setupTaskListTestRouter(t)` 辅助函数。注意 listTasksV2 需要 startDate/endDate 参数（task_handler_v2.go:150-155）。task 表中的 cost/upstream_tokens/downstream_tokens 是直接字段（不需要聚合），参考 task_handler_v2.go:197。

---

### TP-07: getRepoDetailV2 commits 数组包含 cost/tokens 字段

- **类型**: integration
- **描述**: 使用已有的 `setupRepoTestRouter`，插入 commit+task 数据，验证 repo detail 接口的 commits 数组中每项都包含 cost/upstream_tokens/downstream_tokens
- **测试场景**:
  1. 插入 1 条 task：`cost=0.15`, `upstream_tokens=8000`, `downstream_tokens=3000`
  2. 插入 1 条 commit：`task_ids=["test-repo-detail-t1"]`, `task_ids_silica=[1.0]`
  3. GET `/api/v2/repos/detail?repoAddr=test-repo-xxx`
  4. 解析 `commits` 数组
- **预期结果**:
  - HTTP 200
  - `commits[0]["cost"]` == 0.15
  - `commits[0]["upstream_tokens"]` == 8000
  - `commits[0]["downstream_tokens"]` == 3000
- **断言数**: 4
- **测试用例文件**: `backend/unify_columns_integration_test.go`
- **实现备注**: 复用 `setupRepoTestRouter(t)` 已有模式。repo_handler_v2.go:236-238 是验证目标代码。silica=1.0 时 tokens 不需要比例换算，直接等于原值。

---

### TP-08: getRepoDetailV2 多 task 聚合 cost/tokens 正确

- **类型**: integration
- **描述**: 一条 commit 关联 2 个 task（不同 silica 比例），验证 cost 累加正确、tokens 按比例聚合
- **测试场景**:
  1. 插入 task-A: `cost=0.10`, `upstream_tokens=10000`, `downstream_tokens=4000`
  2. 插入 task-B: `cost=0.20`, `upstream_tokens=6000`, `downstream_tokens=2000`
  3. 插入 commit: `task_ids=["task-A","task-B"]`, `task_ids_silica=[0.5, 0.8]`
  4. GET `/api/v2/repos/detail?repoAddr=...`
- **预期结果**:
  - `commits[0]["cost"]` == 0.10 + 0.20 = 0.30（cost 直接累加，不乘 silica）
  - `commits[0]["upstream_tokens"]` == round(10000\*0.5) + round(6000\*0.8) = 5000 + 4800 = 9800
  - `commits[0]["downstream_tokens"]` == round(4000\*0.5) + round(2000\*0.8) = 2000 + 1600 = 3600
- **断言数**: 3
- **测试用例文件**: `backend/unify_columns_integration_test.go`
- **实现备注**: 这是核心聚合逻辑验证。注意 cost 是直接累加（不乘 silica），而 tokens 需要乘以对应的 silica 值后取 Round（commit_handler_v2.go:128-144, repo_handler_v2.go:218-234）。

---

### 前端结构验证测试（Vitest 静态分析）

---

### TP-09: CommitViewV2 恰好 10 列且顺序正确

- **类型**: unit (static analysis)
- **描述**: 用正则从 CommitViewV2.vue 文件中提取所有 `prop: 'xxx'`，验证恰好 10 列且顺序与统一规范一致
- **测试场景**:
  1. 读取 CommitViewV2.vue 文件内容
  2. 用正则 `/prop:\s*'(\w+)'/g` 提取所有 prop 值
  3. 验证数组长度和顺序
- **预期结果**:
  - `props.length === 10`
  - `props` === `['commit_id', 'commit_time', 'user_name', 'comment', 'diff_lines', 'commit_real_minutes', 'commit_ancient_minutes', 'efficiency_ratio', 'cost', '_tokens']`
- **断言数**: 2
- **测试用例文件**: `frontend/src/views/__tests__/unify-columns-structure.test.js`
- **实现备注**: 参考现有 commit-view-structure.test.js 中的正则提取模式（原测试验证 7 列，此测试更新为 10 列）。注意：此测试需要**替换**原 commit-view-structure.test.js 中的"恰好 7 列"断言（测试点 4.1），或在新文件中覆盖。

---

### TP-10: CommitViewV2 有 #cell-commit_id 插槽且导入 fmtCost

- **类型**: unit (static analysis)
- **描述**: 验证 CommitViewV2 模板中有 #cell-commit_id 插槽（用于短 ID 显示），且 script 中导入了 fmtCost
- **测试场景**:
  1. 检查文件内容包含 `#cell-commit_id`
  2. 检查文件内容包含 `fmtCost` 的导入
  3. 检查 cost 列使用了 `formatter: fmtCost`
- **预期结果**:
  - 文件包含 `#cell-commit_id`
  - 文件匹配 `import.*fmtCost.*from '@/utils/formatters'`
  - 文件匹配 `prop:\s*'cost'` 附近有 `formatter:\s*fmtCost`
- **断言数**: 3
- **测试用例文件**: `frontend/src/views/__tests__/unify-columns-structure.test.js`
- **实现备注**: fmtCost 在 CommitViewV2.vue:38 导入，在 line 129 作为 cost 列的 formatter。

---

### TP-11: CommitViewV2 Tokens列正确聚合 upstream + downstream

- **类型**: unit (static analysis)
- **描述**: 验证 `_tokens` 列的 formatter 逻辑正确计算 upstream_tokens + downstream_tokens 之和
- **测试场景**:
  1. 检查文件包含 `prop: '_tokens'`
  2. 检查 `_tokens` 列的 formatter 包含 `upstream_tokens` 和 `downstream_tokens` 的加法
  3. 检查使用 `toLocaleString()` 格式化
- **预期结果**:
  - 文件包含 `prop: '_tokens'`
  - 文件匹配 `(row.upstream_tokens || 0) + (row.downstream_tokens || 0)` 模式
  - _tokens 列 formatter 中包含 `toLocaleString()`
- **断言数**: 3
- **测试用例文件**: `frontend/src/views/__tests__/unify-columns-structure.test.js`
- **实现备注**: 参考 CommitViewV2.vue:141-143 的实际实现。

---

### TP-12: TaskViewV2 恰好 10 列且顺序正确

- **类型**: unit (static analysis)
- **描述**: 验证 TaskViewV2.vue 的列定义恰好 10 列，顺序与统一规范一致
- **测试场景**:
  1. 读取 TaskViewV2.vue 文件内容
  2. 用正则提取所有 prop 值
  3. 验证数组长度和顺序
- **预期结果**:
  - `props.length === 10`
  - `props` === `['task_id', 'start_time', 'user_name', 'title', 'diff_lines', 'task_real_minutes', 'task_ancient_minutes', 'efficiency_ratio', 'cost', '_tokens']`
- **断言数**: 2
- **测试用例文件**: `frontend/src/views/__tests__/unify-columns-structure.test.js`
- **实现备注**: 参考 TaskViewV2.vue:73-178 的实际列定义。

---

### TP-13: TaskViewV2 有 #cell-task_id 插槽且无旧列

- **类型**: unit (static analysis)
- **描述**: 验证 TaskViewV2 有 task_id 的链接插槽，且不包含已移除的 work_dir、mode、单独的 upstream_tokens/downstream_tokens 列
- **测试场景**:
  1. 检查 `#cell-task_id` 存在
  2. 检查文件不包含 `prop: 'work_dir'`
  3. 检查文件不包含 `prop: 'mode'`
  4. 检查文件不包含 `prop: 'upstream_tokens'`（作为独立列定义）
  5. 检查文件不包含 `prop: 'downstream_tokens'`（作为独立列定义）
- **预期结果**:
  - 文件包含 `#cell-task_id`
  - 文件不匹配 `prop:\s*'work_dir'`
  - 文件不匹配 `prop:\s*'mode'`
  - 文件不匹配 `prop:\s*'upstream_tokens'`
  - 文件不匹配 `prop:\s*'downstream_tokens'`
- **断言数**: 5
- **测试用例文件**: `frontend/src/views/__tests__/unify-columns-structure.test.js`
- **实现备注**: upstream_tokens 和 downstream_tokens 作为数据字段仍然存在于 API 响应中，但在列定义中不应作为独立列，而是被合并到 `_tokens` 虚拟列中。

---

### TP-14: TaskViewV2 cost 列使用 fmtCost、_tokens 列聚合正确

- **类型**: unit (static analysis)
- **描述**: 验证 TaskViewV2 的 cost 列使用 fmtCost 格式化，_tokens 列正确聚合
- **测试场景**:
  1. 检查导入包含 fmtCost
  2. 检查 cost 列使用 `formatter: fmtCost`
  3. 检查 _tokens 列包含 upstream + downstream 聚合逻辑
- **预期结果**:
  - 文件匹配 `import.*fmtCost.*from '@/utils/formatters'`
  - 文件中 `prop: 'cost'` 附近有 `formatter:\s*fmtCost`
  - 文件中 `prop: '_tokens'` 附近有 `upstream_tokens.*downstream_tokens` 聚合模式
- **断言数**: 3
- **测试用例文件**: `frontend/src/views/__tests__/unify-columns-structure.test.js`
- **实现备注**: 参考 TaskViewV2.vue:50 的 fmtCost 导入和 line 156 的 formatter 使用。

---

### TP-15: RepoDetailV2 Commits 表恰好 10 列

- **类型**: unit (static analysis)
- **描述**: 统计 RepoDetailV2 中 Commits 表格（第一个 `<el-table>`）的 `<el-table-column>` 数量，验证恰好 10 列
- **测试场景**:
  1. 读取 RepoDetailV2.vue
  2. 提取 Commits 表格区域（从 `Commits (` 到第一个 `</el-table>`）
  3. 用正则 `/<el-table-column/g` 统计数量
  4. 验证列标签顺序
- **预期结果**:
  - Commits 区域有恰好 10 个 `<el-table-column>`
  - 标签序列包含：`Commit ID`, `时间`, `用户`, `说明`, `代码行数`, `实际耗时`, `传统开发时长预估`, `提效比`, `费用`, `Tokens消耗`
- **断言数**: 2
- **测试用例文件**: `frontend/src/views/__tests__/unify-columns-structure.test.js`
- **实现备注**: RepoDetailV2 使用原生 `<el-table-column>` 而非 KbFilterTable columns 数组，需要用不同的正则策略。参考 RepoDetailV2.vue:70-102 的 Commits 表。

---

### TP-16: RepoDetailV2 Tasks 表恰好 10 列

- **类型**: unit (static analysis)
- **描述**: 统计 RepoDetailV2 中 Tasks 表格（第二个 `<el-table>`）的 `<el-table-column>` 数量，验证恰好 10 列
- **测试场景**:
  1. 提取 Tasks 表格区域（从 `Tasks (` 到对应的 `</el-table>`）
  2. 用正则统计 `<el-table-column>` 数量
  3. 验证列标签顺序
- **预期结果**:
  - Tasks 区域有恰好 10 个 `<el-table-column>`
  - 标签序列包含：`Task ID`, `时间`, `用户`, `说明`, `代码行数`, `实际耗时`, `传统开发时长预估`, `提效比`, `费用`, `Tokens消耗`
- **断言数**: 2
- **测试用例文件**: `frontend/src/views/__tests__/unify-columns-structure.test.js`
- **实现备注**: 参考 RepoDetailV2.vue:108-140 的 Tasks 表。

---

### TP-17: RepoDetailV2 两表 minWidth 一致

- **类型**: unit (static analysis)
- **描述**: 验证 Commits 和 Tasks 两张表的 10 列 minWidth 值完全相同：100, 150, 90, 200, 90, 100, 140, 90, 80, 110
- **测试场景**:
  1. 分别从 Commits 和 Tasks 区域提取所有 `min-width="xxx"` 值
  2. 比较两组值完全一致
  3. 验证具体数值序列
- **预期结果**:
  - Commits 表 minWidth 序列 === `[100, 150, 90, 200, 90, 100, 140, 90, 80, 110]`
  - Tasks 表 minWidth 序列 === `[100, 150, 90, 200, 90, 100, 140, 90, 80, 110]`
  - 两组序列相等
- **断言数**: 3
- **测试用例文件**: `frontend/src/views/__tests__/unify-columns-structure.test.js`
- **实现备注**: RepoDetailV2 使用 `min-width="xxx"` 属性（HTML 模板语法），而 CommitViewV2/TaskViewV2 使用 `minWidth: xxx`（JS 列定义语法）。用正则 `min-width="(\d+)"` 提取。

---

### TP-18: 三页面列标签一致性验证

- **类型**: unit (static analysis)
- **描述**: 验证三个页面的 10 列中文标签（label）完全一致（除了第一列 ID 名称和第四列说明的差异外）
- **测试场景**:
  1. 从 CommitViewV2 提取 columns 数组的 label 值
  2. 从 TaskViewV2 提取 columns 数组的 label 值
  3. 从 RepoDetailV2 提取两个 el-table 的 label 值
  4. 比较公共列标签（第2-10列）
- **预期结果**:
  - CommitViewV2 标签：`['Commit ID', '时间', '用户', '说明', '代码行数', '实际耗时', '传统开发时长预估', '提效比', '费用', 'Tokens消耗']`
  - TaskViewV2 标签：`['Task ID', '时间', '用户', '说明', '代码行数', '实际耗时', '传统开发时长预估', '提效比', '费用', 'Tokens消耗']`
  - 第 2-10 列标签三页完全一致
- **断言数**: 3
- **测试用例文件**: `frontend/src/views/__tests__/unify-columns-structure.test.js`
- **实现备注**: CommitViewV2 和 TaskViewV2 用 `label:\s*'([^']+)'` 提取；RepoDetailV2 用 `label="([^"]+)"` 提取。注意第一列分别是 `Commit ID` / `Task ID`，第四列分别是 `说明` / `说明`（TaskViewV2 中对应 `title` 字段但 label 也是"说明"）。

---

## 关键考虑事项

1. **后端测试数据隔离**: 所有测试用例必须使用唯一前缀（含 timestamp）作为 ID，并在 defer 中清理。参考现有 `repo_handler_v2_integration_test.go` 中 `repoAddr := "test-repo-xxx-" + fmt.Sprintf("%d", time.Now().UnixNano())` 的模式。

2. **statDB vs db 全局变量**: 后端 handler 中使用的是 `statDB` 全局变量（连接 `costrict_stat` 数据库），而部分旧测试用 `db` 变量。新测试统一使用 `statDB`，参考 `setupRepoTestRouter` 模式。testDB() 辅助函数连接 `costrict_stat` 数据库（DSN: `dbname=costrict_stat`）。

3. **cost 累加 vs tokens 按比例聚合**: 在 commit 层面聚合 task 数据时，cost 是直接累加所有关联 task 的 cost 值；而 upstream_tokens 和 downstream_tokens 需要乘以对应的 silica 比例后取 `math.Round()`。这是核心聚合逻辑，TP-04 和 TP-08 重点覆盖。

4. **前端测试使用 node 环境**: Vitest 配置为 `node` 环境（无 DOM），所以只能做静态文件分析，不能 mount 组件。使用 `readFileSync` 读取 `.vue` 文件后用正则验证结构。

5. **现有测试点冲突**: commit-view-structure.test.js 中的测试点 4.1 断言"恰好 7 列"，与变更后的 10 列冲突。新测试应**替换或更新**原 4.1 断言。建议：在新测试文件中覆盖列数验证，并在实施时更新旧测试文件中的断言。

6. **RepoDetailV2 的两种列提取方式**: CommitViewV2 和 TaskViewV2 用 JS 对象 `{ prop: 'xxx', label: 'xxx', minWidth: xxx }` 定义列，而 RepoDetailV2 用 HTML 模板 `<el-table-column label="xxx" min-width="xxx">` 定义列。正则需要分别适配。

7. **listCommitsV2 的日期参数**: API 要求 `startDate` 和 `endDate` 查询参数（格式 `YYYY-MM-DD`）。测试插入数据时需确保 `commit_time` 落在请求的日期范围内。

---

## 测试用例文件清单

- `backend/unify_columns_integration_test.go` — 后端集成测试（TP-01 ~ TP-08），8 个测试函数，29 个断言
- `frontend/src/views/__tests__/unify-columns-structure.test.js` — 前端结构验证（TP-09 ~ TP-18），10 个 describe/it 块，28 个断言

**总计**: 18 个测试点，约 57 个断言

---

## 与现有测试的关系

| 现有测试文件 | 影响 |
|---|---|
| `frontend/src/views/__tests__/commit-view-structure.test.js` | **需更新** 测试点 4.1 的"恰好 7 列"断言改为 10 列，props 数组需更新。新增列 `commit_id`, `cost`, `_tokens` |
| `backend/task_handler_v2_integration_test.go` | **无冲突** — 测试 UpdateStatTaskManual 和 getTaskDetailV2，不涉及列表接口 |
| `backend/repo_handler_v2_integration_test.go` | **无冲突** — 测试 ListRepoAggregates 和 reason 字段，新测试覆盖不同的 cost/tokens 聚合逻辑 |
| `frontend/src/views/__tests__/repo-view-structure.test.js` | **无冲突** — 测试 RepoViewV2（列表页），不涉及 RepoDetailV2 的列结构 |
| `frontend/src/utils/commit-helpers.test.js` | **无冲突** — 测试 helper 函数逻辑 |
