# 测试方案：unify-commit-pages — Commit 页面统一重构

> 生成日期：2026-04-08
> 覆盖模块：`frontend/src/views/CommitDetailV2.vue`、`frontend/src/views/CommitViewV2.vue`
> 参考模板：`frontend/src/views/TaskDetailV2.vue`、`frontend/src/views/TaskViewV2.vue`

---

## 概述

本测试方案覆盖 `unify-commit-pages` 变更的前端逻辑和结构一致性，采用 **纯函数测试 + 文件内容静态分析** 策略，所有测试在 vitest `node` 环境下运行（无 DOM / 无组件渲染）：

1. **纯函数逻辑测试**：覆盖 `getEffectiveAncient` / `getEffectiveReal` 辅助函数（需从 SFC 提取）和 `efficiencyColor` 计算逻辑
2. **列定义结构验证**：通过读取文件内容验证 `columns` 数组的列数、过滤器类型、特殊属性
3. **模板结构一致性检查**：通过文件内容字符串匹配，验证 Commit 页面遵循 Task 页面的结构模式
4. **导入依赖完整性**：验证两个文件的所有 `@/` 路径导入均解析到存在的模块文件

共 **10 个测试点**，约 **32 个测试断言**。

---

## 运行测试

```powershell
cd D:\My\PubCode\kanban\frontend
npm run test
```

---

## 测试实施说明

### 函数不可直接导入的处理

CommitViewV2.vue 和 CommitDetailV2.vue 中的辅助函数（`getEffectiveAncient`、`getEffectiveReal`、`efficiencyColor`）定义在 `<script setup>` 内部，无法直接 import。有两种测试策略：

- **策略 A（推荐）**：将可测试的纯函数提取到 `frontend/src/utils/commit-helpers.js`，在测试中直接导入。例如：
  ```js
  // commit-helpers.js
  export function getEffectiveAncient(row) {
    return row.commit_ancient_minutes_manual ?? row.commit_ancient_minutes ?? null
  }
  export function getEffectiveReal(row) {
    return row.commit_real_minutes_manual ?? row.commit_real_minutes ?? null
  }
  export function getEfficiencyColor(ratio) {
    if (ratio == null) return '#909399'
    if (ratio >= 300) return '#67C23A'
    if (ratio >= 150) return '#409EFF'
    return '#909399'
  }
  ```

- **策略 B（备选）**：在测试文件中直接复制函数逻辑进行测试。这种方式更脆弱，需要在源代码变更时同步更新测试。

### 文件内容静态分析

测试点 4、5 通过 `node:fs` 的 `readFileSync` 读取 `.vue` 文件内容为字符串，使用正则或 `includes` 进行结构检查。这种方式不需要 DOM 环境。

---

## 测试点列表

### 1. getEffectiveAncient — manual 优先取值

- **类型**: unit
- **描述**: 验证古法预估取值逻辑：`commit_ancient_minutes_manual ?? commit_ancient_minutes ?? null`
- **测试场景与预期结果**:

| ID | 输入 (row) | 预期结果 | 说明 |
|----|-----------|---------|------|
| 1.1 | `{ commit_ancient_minutes_manual: 120, commit_ancient_minutes: 60 }` | `120` | manual 值存在，返回 manual |
| 1.2 | `{ commit_ancient_minutes_manual: null, commit_ancient_minutes: 60 }` | `60` | manual 为 null，回退到 original |
| 1.3 | `{ commit_ancient_minutes_manual: null, commit_ancient_minutes: null }` | `null` | 都为 null，返回 null |
| 1.4 | `{ commit_ancient_minutes_manual: 0, commit_ancient_minutes: 60 }` | `0` | manual 为 0（falsy 但 ?? 不触发），返回 0 |
| 1.5 | `{ commit_ancient_minutes: 45 }` | `45` | manual 为 undefined，?? 触发回退到 original |

- **测试用例文件**: `frontend/src/utils/commit-helpers.test.js`

### 2. getEffectiveReal — manual 优先取值

- **类型**: unit
- **描述**: 验证实际耗时取值逻辑：`commit_real_minutes_manual ?? commit_real_minutes ?? null`
- **测试场景与预期结果**:

| ID | 输入 (row) | 预期结果 | 说明 |
|----|-----------|---------|------|
| 2.1 | `{ commit_real_minutes_manual: 90, commit_real_minutes: 30 }` | `90` | manual 值存在，返回 manual |
| 2.2 | `{ commit_real_minutes_manual: null, commit_real_minutes: 30 }` | `30` | manual 为 null，回退 |
| 2.3 | `{ commit_real_minutes_manual: null, commit_real_minutes: null }` | `null` | 都为 null |
| 2.4 | `{ commit_real_minutes_manual: 0, commit_real_minutes: 30 }` | `0` | manual 为 0，?? 不回退 |
| 2.5 | `{ commit_real_minutes: 15 }` | `15` | manual undefined → 回退 |

- **测试用例文件**: `frontend/src/utils/commit-helpers.test.js`

### 3. efficiencyColor — 提效比颜色逻辑

- **类型**: unit
- **描述**: 验证 CommitDetailV2 中 `efficiencyColor` 计算逻辑的颜色分级
- **测试场景与预期结果**:

| ID | 输入 (ratio) | 预期颜色 | 颜色含义 |
|----|-------------|---------|---------|
| 3.1 | `null` | `#909399` | 灰色（无数据） |
| 3.2 | `undefined` | `#909399` | 灰色（== null 成立） |
| 3.3 | `300` | `#67C23A` | 绿色（>= 300 边界） |
| 3.4 | `500` | `#67C23A` | 绿色（远超阈值） |
| 3.5 | `150` | `#409EFF` | 蓝色（>= 150 边界） |
| 3.6 | `299` | `#409EFF` | 蓝色（< 300 但 >= 150） |
| 3.7 | `100` | `#909399` | 灰色（< 150） |
| 3.8 | `0` | `#909399` | 灰色（零值） |

- **关键验证**: 边界值 `300` 归入绿色，`299` 归入蓝色；边界值 `150` 归入蓝色，`149` 归入灰色
- **测试用例文件**: `frontend/src/utils/commit-helpers.test.js`

### 4. CommitViewV2 列定义结构验证

- **类型**: unit（静态分析）
- **描述**: 通过读取 CommitViewV2.vue 文件内容，验证 `columns` 数组的结构正确性
- **测试场景与预期结果**:

| ID | 检查项 | 预期结果 |
|----|-------|---------|
| 4.1 | 列数 | 恰好 7 列（user_name, repo_addr, commit_time, diff_lines, commit_ancient_minutes, commit_real_minutes, efficiency_ratio） |
| 4.2 | 每列必备属性 | 每列都有 `prop`、`label`、`filter` 属性 |
| 4.3 | 过滤器类型序列 | 依次为：search-select, text, date, number, number, number, number |
| 4.4 | user_name 列 | `filter: { type: 'search-select' }` |
| 4.5 | repo_addr 列 | `filter: { type: 'text' }` |
| 4.6 | commit_time 列 | `filter: { type: 'date', serverSide: true }` |
| 4.7 | efficiency_ratio 列 | 有 `slotName: 'efficiency_ratio'` |
| 4.8 | commit_ancient_minutes 列 | filter 中有 `valueGetter: getEffectiveAncient` |
| 4.9 | commit_real_minutes 列 | filter 中有 `valueGetter: getEffectiveReal` |

- **实施方式**: 在测试中用 `readFileSync` 读取文件内容，用正则或字符串匹配验证
- **测试用例文件**: `frontend/src/views/__tests__/commit-view-structure.test.js`

### 5. CommitDetailV2 模板结构一致性

- **类型**: unit（静态分析）
- **描述**: 验证 CommitDetailV2 遵循 TaskDetailV2 的双卡片布局模式
- **测试场景与预期结果**:

| ID | 检查项 | 预期结果 |
|----|-------|---------|
| 5.1 | 基础信息卡片 | 文件包含 `header="基础信息"` |
| 5.2 | 度量信息卡片 | 文件包含 `header="度量信息"` |
| 5.3 | el-descriptions 属性 | 文件包含 `:column="3" border` |
| 5.4 | user_id 导航链接 | 文件包含 `commit.user_id` 和 `router.push('/user/'` |

- **实施方式**: 读取文件内容字符串，使用 `includes` 检查关键模板片段
- **测试用例文件**: `frontend/src/views/__tests__/commit-detail-structure.test.js`

### 6. CommitViewV2 组件使用一致性

- **类型**: unit（静态分析）
- **描述**: 验证 CommitViewV2 使用 KbFilterTable 且不使用已废弃的旧组件/composable
- **测试场景与预期结果**:

| ID | 检查项 | 预期结果 |
|----|-------|---------|
| 6.1 | 使用 KbFilterTable | 文件包含 `import KbFilterTable from '@/components/KbFilterTable.vue'` |
| 6.2 | 不导入 FilterBar | 文件不包含 `FilterBar` |
| 6.3 | 不导入 useChart | 文件不包含 `useChart` |
| 6.4 | 不导入 useUrlSync | 文件不包含 `useUrlSync` |
| 6.5 | 导入 getDefaultDateRangeWide | 文件包含 `import { getDefaultDateRangeWide } from '@/utils/date'` |
| 6.6 | handleFilterChange 读 commit_time | 文件包含 `allFilters.commit_time` 且不包含 `allFilters.start_time` |

- **实施方式**: 读取文件内容字符串进行正向和反向匹配
- **测试用例文件**: `frontend/src/views/__tests__/commit-view-structure.test.js`

### 7. CommitDetailV2 导入依赖完整性

- **类型**: unit（静态分析）
- **描述**: 验证 CommitDetailV2 的所有 `@/` 导入路径对应的模块文件存在
- **测试场景与预期结果**:

| ID | 导入路径 | 预期 |
|----|---------|------|
| 7.1 | `@/api/es` | `frontend/src/api/es.js` 存在 |
| 7.2 | `@/utils/formatters` | `frontend/src/utils/formatters.js` 存在 |

- **额外验证**: 从 `@/api/es` 导入的 `getCommitDetailV2` 和 `updateCommitManualV2` 在 es.js 中有定义
- **实施方式**: `readFileSync` 读取文件获取导入语句，`existsSync` 验证文件存在
- **测试用例文件**: `frontend/src/views/__tests__/commit-detail-structure.test.js`

### 8. CommitViewV2 导入依赖完整性

- **类型**: unit（静态分析）
- **描述**: 验证 CommitViewV2 的所有 `@/` 导入路径对应的模块文件存在
- **测试场景与预期结果**:

| ID | 导入路径 | 预期 |
|----|---------|------|
| 8.1 | `@/components/KbFilterTable.vue` | `frontend/src/components/KbFilterTable.vue` 存在 |
| 8.2 | `@/api/es` | `frontend/src/api/es.js` 存在 |
| 8.3 | `@/utils/formatters` | `frontend/src/utils/formatters.js` 存在 |
| 8.4 | `@/utils/date` | `frontend/src/utils/date.js` 存在 |

- **额外验证**: 从各模块导入的具体函数（`getCommitsV2`、`formatDuration`、`getDefaultDateRangeWide`）在对应文件中有 `export` 定义
- **实施方式**: `readFileSync` 读取文件获取导入语句，`existsSync` 验证文件存在
- **测试用例文件**: `frontend/src/views/__tests__/commit-view-structure.test.js`

### 9. CommitViewV2 效率标签颜色一致性

- **类型**: unit（静态分析）
- **描述**: 验证列表页 efficiency_ratio 插槽中的颜色阈值与详情页 efficiencyColor 一致
- **测试场景与预期结果**:

| ID | 检查项 | 预期结果 |
|----|-------|---------|
| 9.1 | 列表页 el-tag 类型判断 | 文件包含 `row.efficiency_ratio >= 300 ? 'success'` 和 `row.efficiency_ratio >= 150 ? 'primary'` |
| 9.2 | 阈值一致性 | 列表页的 300/150 阈值与详情页 efficiencyColor 中的 300/150 阈值一致 |

- **实施方式**: 分别读取两个文件内容，用正则提取阈值并比对
- **测试用例文件**: `frontend/src/views/__tests__/commit-view-structure.test.js`

### 10. CommitViewV2 与 TaskViewV2 模式对齐验证

- **类型**: unit（静态分析）
- **描述**: 验证 CommitViewV2 遵循 TaskViewV2 的相同代码模式（KbFilterTable 使用方式、事件处理）
- **测试场景与预期结果**:

| ID | 检查项 | 预期结果 |
|----|-------|---------|
| 10.1 | KbFilterTable 绑定事件 | CommitViewV2 包含 `@row-click`、`@size-change`、`@page-change`、`@filter-change` |
| 10.2 | KbFilterTable v-model 属性 | CommitViewV2 包含 `v-model:page` 和 `v-model:pageSize` |
| 10.3 | efficiency_ratio 插槽模式 | CommitViewV2 的 `#cell-efficiency_ratio` 模板与 TaskViewV2 结构相同（el-tag + toFixed(1)%） |

- **实施方式**: 读取两个文件内容，逐项比对关键模式
- **测试用例文件**: `frontend/src/views/__tests__/commit-view-structure.test.js`

---

## 关键考虑事项

1. **`??` vs `||` 语义差异**：`getEffectiveAncient`/`getEffectiveReal` 使用 nullish coalescing (`??`)，而非逻辑或 (`||`)。这意味着 `0`（falsy 但非 nullish）会被保留为有效值。测试点 1.4 和 2.4 专门验证了这一行为。

2. **`== null` 覆盖 undefined**：`efficiencyColor` 中 `ratio == null` 使用宽松等于，同时匹配 `null` 和 `undefined`。测试点 3.1 和 3.2 分别验证了这两种情况。

3. **SFC 内部函数不可导入**：Vue 3 `<script setup>` 中定义的函数是模块私有的，不会暴露给外部。测试策略推荐提取到独立工具文件（策略 A），以保证测试的可维护性。如果不提取，可在测试文件中复制逻辑进行验证（策略 B），但需标注同步风险。

4. **文件内容静态分析的局限性**：基于字符串匹配的结构验证不能捕获运行时行为错误（如错误的事件绑定顺序、响应式失效等）。这些测试仅验证代码结构的正确性，不替代手动或 E2E 测试。

5. **颜色阈值一致性**：CommitViewV2（列表页 el-tag type）和 CommitDetailV2（详情页 efficiencyColor）使用相同的 300/150 阈值，但表达方式不同（一个是 el-tag type 字符串，一个是十六进制颜色值）。测试点 9 专门交叉验证两者的一致性。

6. **handleFilterChange 字段名差异**：CommitViewV2 使用 `allFilters.commit_time`，而 TaskViewV2 使用 `allFilters.start_time`。这是正确的设计差异（Commit 表使用 commit_time 字段，Task 表使用 start_time 字段），测试点 6.6 验证了此差异。

7. **vitest 环境限制**：配置为 `environment: 'node'`，没有安装 `@vue/test-utils` 或 `jsdom`。所有测试必须是纯 JS 逻辑测试或文件系统操作，不能进行组件挂载或 DOM 操作。

8. **与已有测试的关系**：`formatters.test.js` 已覆盖 `formatLocalTime` 和 `formatDuration`，本方案不重复测试这些函数。本方案聚焦于 Commit 页面特有的逻辑和结构。

---

## 测试用例文件清单

| 文件路径 | 类型 | 覆盖测试点 | 预计断言数 |
|---------|------|-----------|----------|
| `frontend/src/utils/commit-helpers.test.js` | Vitest 纯函数测试 | 测试点 1（getEffectiveAncient）+ 测试点 2（getEffectiveReal）+ 测试点 3（efficiencyColor） | 18 |
| `frontend/src/views/__tests__/commit-view-structure.test.js` | Vitest 文件内容分析 | 测试点 4（列定义）+ 测试点 6（组件使用）+ 测试点 8（导入）+ 测试点 9（颜色一致）+ 测试点 10（模式对齐） | ~18 |
| `frontend/src/views/__tests__/commit-detail-structure.test.js` | Vitest 文件内容分析 | 测试点 5（模板结构）+ 测试点 7（导入） | ~6 |

**合计：约 42 个测试断言，覆盖 10 个测试点。**

### 前置工作

若采用策略 A（推荐），需在编写测试前：
1. 创建 `frontend/src/utils/commit-helpers.js`，从 CommitViewV2.vue 和 CommitDetailV2.vue 中提取 `getEffectiveAncient`、`getEffectiveReal`、`getEfficiencyColor` 三个纯函数
2. 修改 CommitViewV2.vue 和 CommitDetailV2.vue，改为从 `commit-helpers.js` 导入这些函数
