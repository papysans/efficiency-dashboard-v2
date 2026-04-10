# 测试方案：enhance-task-detail — Task 详情页增强

> 生成日期：2026-04-05
> 覆盖模块：`backend/task_handler_v2.go`、`backend/db.go`、`frontend/src/utils/formatters.js`、`frontend/src/views/TaskDetailV2.vue`

---

## 概述

本测试方案覆盖 `enhance-task-detail` 变更的全部核心功能，采用**集成测试优先 + 单元测试补充**策略：

1. **后端单元测试**（纯函数，无需 DB）：覆盖 `calculateTaskRealMinutes` 核心算法和 `efficiency_ratio` 计算逻辑
2. **前端单元测试**（vitest）：覆盖 `formatLocalTime` 和 `formatDuration` 格式化函数
3. **后端集成测试**（需 DB + HTTP）：覆盖 `UpdateCostrictTaskManual` DB 函数、API 端点、数据库 schema 验证
4. **静态检查**（手动/脚本）：验证前端代码中旧字段名已彻底清除

共 **10 个测试点**，**38 个测试函数**。

---

## 运行测试

### 后端单元测试（无需外部依赖）

```powershell
cd D:\My\PubCode\kanban\backend
go test -run "TestCalculateTaskRealMinutes|TestEfficiencyRatio" -v
```

### 后端集成测试（需要 PostgreSQL）

```powershell
cd D:\My\PubCode\kanban\backend
go test -tags=integration -run "TestUpdateCostrictTaskManual|TestGetTaskDetailV2|TestUpdateTaskManualV2|TestDBSchema" -v
```

### 前端单元测试

```powershell
cd D:\My\PubCode\kanban\frontend
# 首次运行需安装 vitest
npm install
npm run test
```

### 前端旧字段名检查（手动）

```powershell
cd D:\My\PubCode\kanban\frontend
# 检查旧字段名引用（结果应为空）
rg "ai_estimated_ancient_days" src/ -g "*.vue" -g "*.js"
rg "ai_estimated_days" src/ -g "*.vue" -g "*.js"
rg "total_cost" src/ -g "*.vue"
rg "model_name" src/ -g "*.vue"
rg "total_tokens" src/ -g "*.vue"
```

---

## 测试点列表

### 1. calculateTaskRealMinutes 核心算法

- **类型**: unit
- **描述**: 测试基于对话时间序列的实际工作时长计算，包含片段合并、断开、排序、extension 追加等逻辑
- **测试场景**:
  - 0 条有效对话（全部 start_time 为 nil）→ 返回 (0, "无有效对话", nil)
  - 空切片 → 返回 (0, "无有效对话", nil)
  - 1 条有效对话 → 返回 (extensionMin, reason, 单个片段)
  - 1 条有效 + 多条 nil → 同上
  - 2 条连续对话（间隔 20 分钟 < 30 分钟阈值）→ 1 个片段，25 分钟
  - 2 条对话间隔 60 分钟 → 断开为 2 个片段，各 5 分钟
  - 恰好 30 分钟间隔（边界值）→ 归入同一片段，35 分钟
  - 30 分钟 1 秒间隔 → 断开为 2 个片段
  - 乱序输入 → 排序后正确计算
  - 3 个时间段（5 条对话）→ 45 分钟
  - 自定义 gapThreshold=10, extension=3 → 14 分钟
- **预期结果**: 各子用例的 totalMinutes、segments 数量、ConvCount 均符合预期
- **测试用例文件**: `backend/task_real_minutes_test.go`

### 2. efficiency_ratio 计算逻辑

- **类型**: unit
- **描述**: 测试提效比计算，包含 manual 值优先、nil 处理、零值处理
- **测试场景**:
  - 正常情况：ancient=480, real=120 → ratio=400%
  - manual 优先：ancientManual=960, realManual=120 → 800%（忽略非 manual 值）
  - ancient 为 nil → ratio 为 nil
  - real=0 → ratio 为 nil（除零保护）
  - real 为 nil → ratio 为 nil
  - both nil → ratio 为 nil
  - ancient=0 → ratio 为 nil
  - 仅 ancientManual 存在 → 使用 manual 值计算
- **预期结果**: 各场景的 ratio 值或 nil 状态符合预期
- **测试用例文件**: `backend/task_real_minutes_test.go`

### 3. formatLocalTime 前端函数

- **类型**: unit
- **描述**: 测试 ISO 8601 时间字符串转本地格式 YYYY-MM-DD HH:mm:ss
- **测试场景**:
  - 正常 UTC ISO 字符串 → 返回格式化的本地时间
  - 带时区偏移的 ISO 字符串 → 正确转换
  - null / undefined / 空字符串 → 返回 '-'
  - 无效字符串 "not-a-date" → 返回 '-'
- **预期结果**: 有效时间返回 YYYY-MM-DD HH:mm:ss 格式，无效输入返回 '-'
- **测试用例文件**: `frontend/src/utils/formatters.test.js`

### 4. formatDuration 前端函数

- **类型**: unit
- **描述**: 测试分钟数自适应显示：分钟 / 小时分钟 / 人天
- **测试场景**:
  - null/0/undefined → '-'
  - 30 → "30分钟"，1 → "1分钟"，59 → "59分钟"
  - 60 → "1小时"
  - 90 → "1小时30分钟"
  - 120 → "2小时"
  - 480 → "8小时"（边界值，<= 480 用小时）
  - 481 → "1.0人天"（> 480 切换人天）
  - 960 → "2.0人天"
  - 小数四舍五入
- **预期结果**: 各输入值的格式化输出符合预期
- **测试用例文件**: `frontend/src/utils/formatters.test.js`

### 5. UpdateCostrictTaskManual DB 函数

- **类型**: integration
- **描述**: 测试数据库层面的人工修正字段更新，需要连接 PostgreSQL
- **测试场景**:
  - 正常更新 4 个 manual 字段 → 全部写入成功
  - 不存在的 taskID → 返回错误
  - 部分字段为 nil → 允许清空（设为 NULL）
- **预期结果**: 正常更新后 SELECT 验证字段值正确；不存在的 task 返回错误
- **测试用例文件**: `backend/task_handler_v2_integration_test.go`

### 6. GET /api/v2/tasks/:taskId API 返回新字段

- **类型**: integration
- **描述**: 验证任务详情 API 响应包含所有新增字段
- **测试场景**:
  - 存在的 task → 响应包含 task, conversations, time_segments, efficiency_ratio
  - task 对象中包含 task_real_minutes, task_ancient_minutes
  - 不存在的 task → 404
- **预期结果**: 200 响应 JSON 结构完整，新字段均存在
- **测试用例文件**: `backend/task_handler_v2_integration_test.go`

### 7. PUT /api/v2/tasks/:taskId/manual API

- **类型**: integration
- **描述**: 验证人工修正更新 API
- **测试场景**:
  - 正常 JSON 提交 → 200, {"status": "ok"}
  - 无效 JSON → 400
- **预期结果**: 正常请求返回 200，无效请求返回 400
- **测试用例文件**: `backend/task_handler_v2_integration_test.go`

### 8. 数据库 schema — 新字段存在验证

- **类型**: integration
- **描述**: 查询 information_schema 验证 costrict_tasks 表的 8 个新字段均已创建
- **测试场景**: 查询 information_schema.columns 验证以下字段存在：
  - task_real_minutes, task_real_minutes_reason
  - task_real_minutes_manual, task_real_minutes_reason_manual
  - task_ancient_minutes, task_ancient_minutes_reason
  - task_ancient_minutes_manual, task_ancient_minutes_reason_manual
- **预期结果**: 8 个字段均存在
- **测试用例文件**: `backend/task_handler_v2_integration_test.go`

### 9. 数据库 schema — 旧字段不存在验证

- **类型**: integration
- **描述**: 验证已重命名/删除的旧字段不再存在于 costrict_tasks 表中
- **测试场景**: 查询 information_schema.columns 验证以下字段不存在：
  - ai_estimated_ancient_days
  - ai_estimated_ancient_reason
- **预期结果**: 旧字段不存在
- **测试用例文件**: `backend/task_handler_v2_integration_test.go`

### 10. 前端旧字段名引用检查

- **类型**: 静态检查（手动）
- **描述**: 在前端 .vue 和 .js 文件中搜索已废弃的旧字段名，确认不存在错误引用
- **测试场景**: 使用 rg 搜索以下模式：
  - `ai_estimated_ancient_days` 在 src/ 中
  - `ai_estimated_days` 在 src/ 中
  - `total_cost` 在 .vue 文件中（应使用 cost 字段）
  - `model_name` 在 .vue 文件中
  - `total_tokens` 在 .vue 文件中
- **预期结果**: 搜索结果为空（不存在错误引用）
- **验证方式**: 在 PowerShell 中手动执行搜索命令

---

## 关键考虑事项

1. **calculateTaskRealMinutes 的 gap 判断使用 `<=`**：恰好等于 gapThreshold 的间隔会被合并到同一片段（代码 `gap <= gapDur`），超过才断开。边界值测试验证了这一点。

2. **efficiency_ratio 的除零保护**：代码要求 `*effectiveReal > 0 && *effectiveAncient > 0` 才计算，避免除零和无意义的 0/x 场景。

3. **manual 值优先级**：getTaskDetailV2 中，如果 manual 值存在，则覆盖自动计算值用于 efficiency_ratio 计算。测试用例 2.2 和 2.8 验证了这一逻辑。

4. **异步更新 task_real_minutes**：getTaskDetailV2 使用 goroutine 异步写回计算结果到数据库。集成测试中不直接验证异步写入（有竞态条件），而是通过验证 API 响应中的值间接确认。

5. **vitest 环境配置**：前端项目需新增 vitest 依赖（已更新 package.json）。vite.config.js 中添加了 `test.environment: 'node'` 配置。

6. **集成测试使用 build tag**：后端集成测试使用 `//go:build integration` 标签，不会在普通 `go test` 中运行，需显式指定 `-tags=integration`。

7. **测试数据清理**：集成测试中每个测试函数都使用 `defer` 清理插入的测试数据，避免污染数据库。

8. **formatDuration 的 480 分钟边界**：`<= 480` 使用小时制，`> 480` 切换人天。480 分钟恰好是 8 小时 = 1 个工作日。

---

## 测试用例文件清单

| 文件 | 类型 | 覆盖测试点 | 测试函数数 |
|------|------|-----------|-----------|
| `backend/task_real_minutes_test.go` | Go 单元测试 | 测试点 1（算法）+ 测试点 2（ratio） | 19 |
| `frontend/src/utils/formatters.test.js` | Vitest 单元测试 | 测试点 3（formatLocalTime）+ 测试点 4（formatDuration） | 16 |
| `backend/task_handler_v2_integration_test.go` | Go 集成测试 | 测试点 5-9（DB + API + schema） | 8 |

**合计：43 个自动化测试函数，覆盖 10 个测试点（其中测试点 10 为手动静态检查）。**

### 文件路径

- `backend/task_real_minutes_test.go`
- `backend/task_handler_v2_integration_test.go`
- `frontend/src/utils/formatters.test.js`
