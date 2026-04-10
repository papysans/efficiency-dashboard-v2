# 测试方案：enhance-user-detail-charts

## 概述

本次变更为纯前端改动，涉及4个核心功能点：

1. **布局优化**：`UserDetailV2.vue` 和 `GroupView.vue` 移除独立 FilterBar 行，将 `DateRangePicker` 内嵌到标题行右侧（flex justify-content: space-between）
2. **图表重设计**：两个页面的单一折线图替换为5个独立 ECharts 图表（2×2网格 + 1全宽），分别展示 Task数&Commit数、代码行数、耗时对比、费用、提效比趋势
3. **跳转联动**：`UserDetailV2` 按天明细表格中 Task数/Commit数 列添加 `el-link`，点击跳转到 `/task-v2` 或 `/commit-v2`，并携带日期和用户名 query 参数
4. **URL 初始化**：`TaskViewV2` 和 `CommitViewV2` 在 `onMounted` 时读取 URL query 参数（`startDate`/`endDate`/`userName`）并初始化筛选器

测试策略：优先使用 PowerShell 静态代码检查验证代码结构正确性，辅以浏览器手动验证交互行为。无后端修改，不需要集成测试。

---

## 测试点列表

### 1. 布局验证：UserDetailV2.vue 标题行内嵌 DateRangePicker，无独立 FilterBar

- **类型**: integration（静态文件检查）
- **描述**: 验证 `UserDetailV2.vue` 已移除 `FilterBar` 组件引用，标题行使用 flex 布局在右侧嵌入 `DateRangePicker`
- **测试场景**:
  - 检查文件中不包含 `FilterBar` 字符串
  - 检查文件包含 `DateRangePicker` 组件
  - 检查标题行使用 `justify-content: space-between` 布局
  - 检查 `DateRangePicker` 绑定了 `@change="fetchData"`
- **预期结果**: 无 `FilterBar` 引用，有 `DateRangePicker` 且绑定 `@change="fetchData"`，布局为 space-between
- **测试用例文件**: 无（静态检查）

**测试命令（PowerShell）**:
```powershell
$file = "D:\My\PubCode\kanban\frontend\src\views\UserDetailV2.vue"
$content = Get-Content $file -Raw

# 不应包含 FilterBar
if ($content -notmatch "FilterBar") { Write-Host "✓ 无 FilterBar 引用" -ForegroundColor Green } else { Write-Host "✗ 仍有 FilterBar 引用" -ForegroundColor Red }

# 应包含 DateRangePicker 且绑定 @change
if ($content -match 'DateRangePicker.*@change="fetchData"') { Write-Host "✓ DateRangePicker 绑定 @change=fetchData" -ForegroundColor Green } else { Write-Host "✗ DateRangePicker 未正确绑定" -ForegroundColor Red }

# 应包含 space-between 布局
if ($content -match "space-between") { Write-Host "✓ 标题行使用 space-between 布局" -ForegroundColor Green } else { Write-Host "✗ 缺少 space-between 布局" -ForegroundColor Red }

# 应包含返回按钮和用户名标题
if ($content -match "router\.back\(\)" -and $content -match "用户详情") { Write-Host "✓ 标题行包含返回按钮和用户名" -ForegroundColor Green } else { Write-Host "✗ 标题行结构不完整" -ForegroundColor Red }
```

---

### 2. 布局验证：GroupView.vue 标题行内嵌 DateRangePicker，无独立 FilterBar

- **类型**: integration（静态文件检查）
- **描述**: 验证 `GroupView.vue` 与 `UserDetailV2.vue` 相同的布局结构，标题行右侧内嵌 `DateRangePicker`
- **测试场景**:
  - 检查文件中不包含 `FilterBar` 字符串
  - 检查文件包含 `DateRangePicker` 组件且绑定 `@change="fetchData"`
  - 检查标题行使用 `justify-content: space-between` 布局
- **预期结果**: 无 `FilterBar`，有 `DateRangePicker` 绑定 change 事件，布局一致
- **测试用例文件**: 无（静态检查）

**测试命令（PowerShell）**:
```powershell
$file = "D:\My\PubCode\kanban\frontend\src\views\GroupView.vue"
$content = Get-Content $file -Raw

if ($content -notmatch "FilterBar") { Write-Host "✓ 无 FilterBar 引用" -ForegroundColor Green } else { Write-Host "✗ 仍有 FilterBar 引用" -ForegroundColor Red }
if ($content -match 'DateRangePicker.*@change="fetchData"') { Write-Host "✓ DateRangePicker 绑定 @change=fetchData" -ForegroundColor Green } else { Write-Host "✗ DateRangePicker 未正确绑定" -ForegroundColor Red }
if ($content -match "space-between") { Write-Host "✓ 标题行使用 space-between 布局" -ForegroundColor Green } else { Write-Host "✗ 缺少 space-between 布局" -ForegroundColor Red }
if ($content -match "router\.back\(\)" -and $content -match "组织详情") { Write-Host "✓ 标题行包含返回按钮和组织名" -ForegroundColor Green } else { Write-Host "✗ 标题行结构不完整" -ForegroundColor Red }
```

---

### 3. 图表结构验证：UserDetailV2.vue 包含5个独立图表引用

- **类型**: integration（静态文件检查）
- **描述**: 验证 `UserDetailV2.vue` 包含 `chart1Ref`~`chart5Ref` 五个图表容器引用，以及对应的5个 `updateChartX` 函数
- **测试场景**:
  - 检查 `chart1Ref` ~ `chart5Ref` 均存在（模板 ref 和 script ref 定义）
  - 检查 `updateChart1` ~ `updateChart5` 函数均存在
  - 检查图表渲染条件：`v-if="daily.length > 0"`
  - 检查图表布局：前4个使用 `grid-template-columns: 1fr 1fr`，第5个独立全宽
- **预期结果**: 5个图表引用和函数均存在，渲染条件正确，布局结构正确
- **测试用例文件**: 无（静态检查）

**测试命令（PowerShell）**:
```powershell
$file = "D:\My\PubCode\kanban\frontend\src\views\UserDetailV2.vue"
$content = Get-Content $file -Raw

# 检查5个图表 ref
1..5 | ForEach-Object {
  if ($content -match "chart${_}Ref") { Write-Host "✓ chart${_}Ref 存在" -ForegroundColor Green } else { Write-Host "✗ chart${_}Ref 缺失" -ForegroundColor Red }
}

# 检查5个 updateChart 函数
1..5 | ForEach-Object {
  if ($content -match "function updateChart${_}") { Write-Host "✓ updateChart${_} 函数存在" -ForegroundColor Green } else { Write-Host "✗ updateChart${_} 函数缺失" -ForegroundColor Red }
}

# 检查渲染条件
if ($content -match 'v-if="daily\.length > 0"') { Write-Host "✓ 图表渲染条件正确" -ForegroundColor Green } else { Write-Host "✗ 图表渲染条件缺失" -ForegroundColor Red }

# 检查2x2网格布局
if ($content -match "grid-template-columns: 1fr 1fr") { Write-Host "✓ 2x2网格布局存在" -ForegroundColor Green } else { Write-Host "✗ 2x2网格布局缺失" -ForegroundColor Red }
```

---

### 4. 图表内容验证：5个图表标题和系列配置正确

- **类型**: integration（静态文件检查）
- **描述**: 验证每个图表的标题文本和系列类型符合设计要求：图表1/2/3/4为柱状图，图表5为折线图
- **测试场景**:
  - 图表1：标题 "Task数 & Commit数"，两个 bar 系列（Task数/Commit数）
  - 图表2：标题 "代码行数"，两个 bar 系列（Task代码行数/Commit代码行数）
  - 图表3：标题 "耗时对比"，四个 bar 系列（Task传统/实际耗时、Commit传统/实际耗时）
  - 图表4：标题 "费用"，一个 bar 系列
  - 图表5：标题 "提效比趋势"，两个 line 系列（Task提效比/Commit提效比），Y轴有 `%` 格式化
- **预期结果**: 所有图表标题和系列类型配置正确
- **测试用例文件**: 无（静态检查）

**测试命令（PowerShell）**:
```powershell
$file = "D:\My\PubCode\kanban\frontend\src\views\UserDetailV2.vue"
$content = Get-Content $file -Raw

$chartChecks = @(
  @{ Pattern = "Task数 & Commit数"; Desc = "图表1标题" },
  @{ Pattern = "代码行数"; Desc = "图表2标题" },
  @{ Pattern = "耗时对比"; Desc = "图表3标题" },
  @{ Pattern = "费用"; Desc = "图表4标题" },
  @{ Pattern = "提效比趋势"; Desc = "图表5标题" },
  @{ Pattern = "type: 'bar'"; Desc = "柱状图系列" },
  @{ Pattern = "type: 'line'"; Desc = "折线图系列" },
  @{ Pattern = "'{value}%'"; Desc = "Y轴百分比格式" },
  @{ Pattern = "Task传统耗时"; Desc = "图表3-4系列之一" },
  @{ Pattern = "Commit实际耗时"; Desc = "图表3-4系列之一" }
)

foreach ($check in $chartChecks) {
  if ($content -match [regex]::Escape($check.Pattern)) {
    Write-Host "✓ $($check.Desc): $($check.Pattern)" -ForegroundColor Green
  } else {
    Write-Host "✗ $($check.Desc) 缺失: $($check.Pattern)" -ForegroundColor Red
  }
}
```

---

### 5. 跳转联动：UserDetailV2 按天明细 Task数列 el-link 代码结构正确

- **类型**: integration（静态文件检查）
- **描述**: 验证 `UserDetailV2.vue` 中 Task数列使用 `el-link` 且绑定 `handleTaskClick`，仅在 `task_ids` 有数据时显示链接
- **测试场景**:
  - 检查 `el-link` 绑定 `@click="handleTaskClick(row)"`
  - 检查 `v-if="getArrayLength(row.task_ids) > 0"` 条件渲染
  - 检查 `handleTaskClick` 函数存在，且包含日期格式转换（`replace(/-/g, '')`）和 router.push 到 `/task-v2`
  - 检查 query 参数包含 `startDate`、`endDate`、`userName`
- **预期结果**: el-link 条件渲染正确，handleTaskClick 函数逻辑完整
- **测试用例文件**: 无（静态检查）

**测试命令（PowerShell）**:
```powershell
$file = "D:\My\PubCode\kanban\frontend\src\views\UserDetailV2.vue"
$content = Get-Content $file -Raw

$checks = @(
  @{ Pattern = "handleTaskClick(row)"; Desc = "Task列绑定 handleTaskClick" },
  @{ Pattern = "getArrayLength(row.task_ids) > 0"; Desc = "Task列条件渲染" },
  @{ Pattern = "handleCommitClick(row)"; Desc = "Commit列绑定 handleCommitClick" },
  @{ Pattern = "getArrayLength(row.commit_ids) > 0"; Desc = "Commit列条件渲染" },
  @{ Pattern = "function handleTaskClick"; Desc = "handleTaskClick 函数定义" },
  @{ Pattern = "function handleCommitClick"; Desc = "handleCommitClick 函数定义" },
  @{ Pattern = "/task-v2"; Desc = "跳转路径 /task-v2" },
  @{ Pattern = "/commit-v2"; Desc = "跳转路径 /commit-v2" },
  @{ Pattern = "replace(/-/g, '')"; Desc = "日期格式转换（去除连字符）" },
  @{ Pattern = "startDate: date, endDate: date, userName"; Desc = "query 参数结构" }
)

foreach ($check in $checks) {
  if ($content -match [regex]::Escape($check.Pattern)) {
    Write-Host "✓ $($check.Desc)" -ForegroundColor Green
  } else {
    Write-Host "✗ 缺失: $($check.Desc)" -ForegroundColor Red
  }
}
```

---

### 6. URL 初始化：TaskViewV2.vue onMounted 读取 query 参数逻辑正确

- **类型**: integration（静态文件检查）
- **描述**: 验证 `TaskViewV2.vue` 的 `onMounted` 中正确读取 `route.query.startDate/endDate/userName`，并做格式转换后设置筛选器
- **测试场景**:
  - 检查 `useRoute` 引入
  - 检查 `onMounted` 中读取 `route.query` 的 `startDate`、`endDate`、`userName`
  - 检查日期格式转换函数（`YYYYMMDD` → `YYYY-MM-DD`：`s.slice(0,4) + '-' + s.slice(4,6) + '-' + s.slice(6,8)`）
  - 检查 `filterTableRef.value?.setFilter('start_time', serverDateRange)` 调用
  - 检查 `filterTableRef.value?.setFilter('user_name', String(userName))` 调用
  - 检查 `if (startDate && endDate)` 双参数校验（缺一不初始化）
- **预期结果**: 所有逻辑结构完整，格式转换正确
- **测试用例文件**: 无（静态检查）

**测试命令（PowerShell）**:
```powershell
$file = "D:\My\PubCode\kanban\frontend\src\views\TaskViewV2.vue"
$content = Get-Content $file -Raw

$checks = @(
  @{ Pattern = "useRoute"; Desc = "引入 useRoute" },
  @{ Pattern = "route.query"; Desc = "读取 route.query" },
  @{ Pattern = "startDate, endDate, userName"; Desc = "解构 startDate/endDate/userName" },
  @{ Pattern = "s.slice(0, 4) + '-' + s.slice(4, 6) + '-' + s.slice(6, 8)"; Desc = "日期格式转换" },
  @{ Pattern = "setFilter('start_time'"; Desc = "设置日期筛选器" },
  @{ Pattern = "setFilter('user_name'"; Desc = "设置用户名筛选器" },
  @{ Pattern = "String(userName)"; Desc = "userName 转字符串" },
  @{ Pattern = "if (startDate && endDate)"; Desc = "双参数校验" }
)

foreach ($check in $checks) {
  if ($content -match [regex]::Escape($check.Pattern)) {
    Write-Host "✓ $($check.Desc)" -ForegroundColor Green
  } else {
    Write-Host "✗ 缺失: $($check.Desc)" -ForegroundColor Red
  }
}
```

---

### 7. URL 初始化：CommitViewV2.vue onMounted 读取 query 参数逻辑正确

- **类型**: integration（静态文件检查）
- **描述**: 验证 `CommitViewV2.vue` 与 `TaskViewV2.vue` 相同的 URL 初始化逻辑，但筛选字段使用 `commit_time`
- **测试场景**:
  - 检查 `useRoute` 引入
  - 检查 `onMounted` 中读取 `route.query.startDate/endDate/userName`
  - 检查日期格式转换
  - 检查 `setFilter('commit_time', ...)` 调用（注意：不是 `start_time`）
  - 检查 `setFilter('user_name', String(userName))` 调用
- **预期结果**: 逻辑与 TaskViewV2 一致，筛选字段正确使用 `commit_time`
- **测试用例文件**: 无（静态检查）

**测试命令（PowerShell）**:
```powershell
$file = "D:\My\PubCode\kanban\frontend\src\views\CommitViewV2.vue"
$content = Get-Content $file -Raw

$checks = @(
  @{ Pattern = "useRoute"; Desc = "引入 useRoute" },
  @{ Pattern = "startDate, endDate, userName"; Desc = "解构 query 参数" },
  @{ Pattern = "s.slice(0, 4) + '-' + s.slice(4, 6) + '-' + s.slice(6, 8)"; Desc = "日期格式转换" },
  @{ Pattern = "setFilter('commit_time'"; Desc = "设置 commit_time 筛选器（而非 start_time）" },
  @{ Pattern = "setFilter('user_name'"; Desc = "设置用户名筛选器" },
  @{ Pattern = "String(userName)"; Desc = "userName 转字符串" },
  @{ Pattern = "if (startDate && endDate)"; Desc = "双参数校验" }
)

foreach ($check in $checks) {
  if ($content -match [regex]::Escape($check.Pattern)) {
    Write-Host "✓ $($check.Desc)" -ForegroundColor Green
  } else {
    Write-Host "✗ 缺失: $($check.Desc)" -ForegroundColor Red
  }
}

# 反向验证：CommitViewV2 不应使用 start_time 作为日期筛选字段
if ($content -notmatch "setFilter\('start_time'") { Write-Host "✓ 未误用 start_time 字段" -ForegroundColor Green } else { Write-Host "✗ 错误使用了 start_time（应为 commit_time）" -ForegroundColor Red }
```

---

### 8. 浏览器验证：UserDetailV2 布局和图表渲染（有数据时）

- **类型**: e2e（手动浏览器验证）
- **描述**: 在浏览器中访问有数据的用户详情页，验证 DateRangePicker 位置、5个图表渲染、表格 el-link 显示
- **测试场景**:
  1. 访问 `http://localhost:8880/user-v2`，找到一个有数据的用户，点击进入详情
  2. 验证标题行：左侧为"返回"按钮和用户名，右侧为 DateRangePicker（无独立筛选行）
  3. 等待数据加载完成（loading 消失）
  4. 验证按天明细表格中 Task数/Commit数 列：有数据的行显示蓝色 el-link，无数据的行显示 "0"
  5. 验证5个图表区域渲染（页面底部）：前4个图表2列布局，第5个全宽
  6. 验证图表标题：Task数&Commit数 / 代码行数 / 耗时对比 / 费用 / 提效比趋势
- **预期结果**:
  - 标题行右侧有 DateRangePicker，无独立筛选行
  - 有 task_ids 数据的行显示蓝色 el-link
  - 5个图表均正确渲染，有图例和坐标轴
- **测试用例文件**: 无（手动验证）

**测试步骤**:
```
1. 浏览器打开 http://localhost:8880/user-v2
2. 点击任意用户名进入 /user/:userId 详情页
3. 观察页面顶部 el-card：
   - ✓ 左侧：返回按钮 + "用户详情: {用户名}"
   - ✓ 右侧：DateRangePicker 日期选择器
   - ✓ 无第二行独立筛选区域
4. 等待数据加载，观察按天明细表格：
   - ✓ Task数列：有数据行显示蓝色 el-link，点击不报错
   - ✓ Commit数列：有数据行显示蓝色 el-link，点击不报错
   - ✓ 无数据行显示 "0"（普通文字）
5. 向下滚动，观察图表区域：
   - ✓ 第一排：2个图表并排（Task数&Commit数 / 代码行数）
   - ✓ 第二排：2个图表并排（耗时对比 / 费用）
   - ✓ 第三排：1个全宽图表（提效比趋势）
   - ✓ 每个图表有标题、图例、坐标轴、数据柱/线
```

---

### 9. 浏览器验证：Task数 el-link 跳转联动

- **类型**: e2e（手动浏览器验证）
- **描述**: 点击按天明细中 Task数 el-link，验证跳转到 `/task-v2` 且 URL 包含正确的 query 参数，且 TaskViewV2 筛选器已初始化
- **测试场景**:
  1. 在 UserDetailV2 按天明细中，找到 Task数 > 0 的行（显示 el-link）
  2. 记录该行的日期（如 2025-01-15）和当前用户名
  3. 点击 el-link
  4. 验证跳转后 URL 格式
  5. 验证 TaskViewV2 日期筛选器和用户名筛选器已初始化
- **预期结果**:
  - URL 变为 `/task-v2?startDate=20250115&endDate=20250115&userName=xxx`（日期格式 YYYYMMDD，无连字符）
  - TaskViewV2 的日期筛选器显示对应日期范围
  - TaskViewV2 的用户名筛选器显示对应用户名
  - 表格数据已按筛选条件加载
- **测试用例文件**: 无（手动验证）

**测试步骤**:
```
1. 在 UserDetailV2 按天明细找到 Task数 > 0 的行
2. 记录日期（如 "2025-01-15"）和用户名（如 "zhangsan"）
3. 点击 Task数 列的蓝色数字链接
4. 浏览器地址栏检查：
   - ✓ 路径为 /task-v2
   - ✓ startDate=20250115（YYYYMMDD 格式，无连字符）
   - ✓ endDate=20250115（与 startDate 相同，同一天）
   - ✓ userName=zhangsan
5. TaskViewV2 页面加载后检查：
   - ✓ 日期筛选器显示 "2025-01-15  To  2025-01-15"
   - ✓ 用户名筛选器显示 "zhangsan"
   - ✓ 表格数据已过滤（仅显示该用户该天的 Task）
```

---

### 10. 浏览器验证：Commit数 el-link 跳转联动 + 边界条件

- **类型**: e2e（手动浏览器验证）
- **描述**: 点击 Commit数 el-link 验证跳转到 `/commit-v2`；同时验证边界条件：仅提供 startDate 无 endDate 时不初始化日期筛选
- **测试场景**:
  - **正常场景**：点击 Commit数 el-link，验证跳转到 `/commit-v2?startDate=YYYYMMDD&endDate=YYYYMMDD&userName=xxx`，CommitViewV2 筛选器已初始化
  - **边界场景1**：直接访问 `/task-v2?startDate=20250115`（无 endDate），验证日期筛选器使用默认范围（不被 startDate 覆盖）
  - **边界场景2**：直接访问 `/commit-v2?startDate=20250101&endDate=20250131&userName=123`（userName 为数字字符串），验证用户名筛选器显示 "123"（字符串化）
- **预期结果**:
  - 正常场景：CommitViewV2 日期和用户名筛选器均已初始化
  - 边界场景1：日期筛选器使用默认范围（不因单独 startDate 而变化）
  - 边界场景2：用户名筛选器正确显示数字字符串
- **测试用例文件**: 无（手动验证）

**测试步骤**:
```
# 正常场景
1. 在 UserDetailV2 找到 Commit数 > 0 的行，点击 el-link
2. 验证 URL: /commit-v2?startDate=YYYYMMDD&endDate=YYYYMMDD&userName=xxx
3. 验证 CommitViewV2 的 commit_time 筛选器和用户名筛选器已初始化

# 边界场景1：缺少 endDate
4. 浏览器直接访问: http://localhost:8880/task-v2?startDate=20250115
5. 验证：日期筛选器显示默认范围（不是 2025-01-15，因为 endDate 缺失）

# 边界场景2：userName 为数字
6. 浏览器直接访问: http://localhost:8880/commit-v2?startDate=20250101&endDate=20250131&userName=123
7. 验证：用户名筛选器显示 "123"（字符串，不报错）
```

---

## 关键考虑事项

1. **图表渲染条件**：5个图表使用 `v-if="daily.length > 0"` 条件渲染。若当前日期范围内无数据，图表区域不会出现在 DOM 中，这是正确行为。测试时需确保选择有数据的日期范围。

2. **日期格式转换**：`handleTaskClick`/`handleCommitClick` 将 `YYYY-MM-DD` 转换为 `YYYYMMDD`（去除连字符）作为 URL query 参数；`TaskViewV2`/`CommitViewV2` 的 `onMounted` 做反向转换（`YYYYMMDD` → `YYYY-MM-DD`）。两端格式必须一致。

3. **getArrayLength 函数**：支持数组和 JSON 字符串两种格式的 `task_ids`/`commit_ids`，解析失败返回 0。el-link 的显示条件 `> 0` 确保只有实际有数据时才显示链接。

4. **setFilter 调用顺序**：`TaskViewV2` 和 `CommitViewV2` 在 `onMounted` 中先设置日期筛选（`setFilter`），再 `fetchData`，最后设置用户名筛选（在 fetchData 之后）。这个顺序确保数据加载时已有日期约束，用户名筛选为前端过滤。

5. **GroupView 图表字段差异**：`GroupView` 的 `daily` 数据字段名与 `UserDetailV2` 有差异（如 `d.date` vs `d.create_time`，`d.task_count` vs `getArrayLength(d.task_ids)`）。静态检查时需注意两个文件各自的字段引用是否与后端 API 返回一致。

6. **前端构建验证**：所有静态检查完成后，建议执行 `npm run build` 确认无编译错误（变更概述已标注构建成功）。

---

## 测试用例文件清单

> 本次变更为纯前端改动，测试方式为静态代码检查 + 浏览器手动验证，无需新增自动化测试文件。

- 静态检查脚本（内嵌于测试点中，使用 PowerShell 执行）
- 浏览器手动验证步骤（测试点 8、9、10）

---

## 一键静态验证脚本（PowerShell）

以下脚本可在不启动浏览器的情况下，完成所有静态代码结构验证：

```powershell
Write-Host "=== enhance-user-detail-charts 变更静态验证 ===" -ForegroundColor Cyan

# ---- 1. UserDetailV2.vue 布局验证 ----
Write-Host "`n[1] UserDetailV2.vue 布局" -ForegroundColor Yellow
$udv2 = Get-Content "D:\My\PubCode\kanban\frontend\src\views\UserDetailV2.vue" -Raw
if ($udv2 -notmatch "FilterBar") { Write-Host "  ✓ 无 FilterBar 引用" -ForegroundColor Green } else { Write-Host "  ✗ 仍有 FilterBar 引用" -ForegroundColor Red }
if ($udv2 -match 'DateRangePicker.*@change="fetchData"') { Write-Host "  ✓ DateRangePicker 绑定 @change" -ForegroundColor Green } else { Write-Host "  ✗ DateRangePicker 绑定缺失" -ForegroundColor Red }
if ($udv2 -match "space-between") { Write-Host "  ✓ space-between 布局" -ForegroundColor Green } else { Write-Host "  ✗ space-between 布局缺失" -ForegroundColor Red }

# ---- 2. GroupView.vue 布局验证 ----
Write-Host "`n[2] GroupView.vue 布局" -ForegroundColor Yellow
$gv = Get-Content "D:\My\PubCode\kanban\frontend\src\views\GroupView.vue" -Raw
if ($gv -notmatch "FilterBar") { Write-Host "  ✓ 无 FilterBar 引用" -ForegroundColor Green } else { Write-Host "  ✗ 仍有 FilterBar 引用" -ForegroundColor Red }
if ($gv -match 'DateRangePicker.*@change="fetchData"') { Write-Host "  ✓ DateRangePicker 绑定 @change" -ForegroundColor Green } else { Write-Host "  ✗ DateRangePicker 绑定缺失" -ForegroundColor Red }

# ---- 3. 5个图表结构验证 ----
Write-Host "`n[3] UserDetailV2.vue 图表结构" -ForegroundColor Yellow
1..5 | ForEach-Object {
  $n = $_
  if ($udv2 -match "chart${n}Ref" -and $udv2 -match "function updateChart${n}") {
    Write-Host "  ✓ chart${n}: ref + updateChart${n} 均存在" -ForegroundColor Green
  } else {
    Write-Host "  ✗ chart${n}: 缺少 ref 或 updateChart 函数" -ForegroundColor Red
  }
}
if ($udv2 -match 'v-if="daily\.length > 0"') { Write-Host "  ✓ 图表渲染条件 daily.length > 0" -ForegroundColor Green } else { Write-Host "  ✗ 图表渲染条件缺失" -ForegroundColor Red }
if ($udv2 -match "grid-template-columns: 1fr 1fr") { Write-Host "  ✓ 2x2 网格布局" -ForegroundColor Green } else { Write-Host "  ✗ 2x2 网格布局缺失" -ForegroundColor Red }

# ---- 4. 图表标题和系列类型 ----
Write-Host "`n[4] 图表标题和系列类型" -ForegroundColor Yellow
@("Task数 & Commit数", "代码行数", "耗时对比", "费用", "提效比趋势") | ForEach-Object {
  if ($udv2 -match [regex]::Escape($_)) { Write-Host "  ✓ 图表标题: $_" -ForegroundColor Green } else { Write-Host "  ✗ 图表标题缺失: $_" -ForegroundColor Red }
}
if ($udv2 -match "type: 'bar'") { Write-Host "  ✓ 柱状图系列存在" -ForegroundColor Green } else { Write-Host "  ✗ 柱状图系列缺失" -ForegroundColor Red }
if ($udv2 -match "type: 'line'") { Write-Host "  ✓ 折线图系列存在" -ForegroundColor Green } else { Write-Host "  ✗ 折线图系列缺失" -ForegroundColor Red }

# ---- 5. 跳转联动代码 ----
Write-Host "`n[5] 跳转联动代码" -ForegroundColor Yellow
if ($udv2 -match "handleTaskClick") { Write-Host "  ✓ handleTaskClick 存在" -ForegroundColor Green } else { Write-Host "  ✗ handleTaskClick 缺失" -ForegroundColor Red }
if ($udv2 -match "handleCommitClick") { Write-Host "  ✓ handleCommitClick 存在" -ForegroundColor Green } else { Write-Host "  ✗ handleCommitClick 缺失" -ForegroundColor Red }
if ($udv2 -match "replace\(/-/g, ''\)") { Write-Host "  ✓ 日期格式转换（去连字符）" -ForegroundColor Green } else { Write-Host "  ✗ 日期格式转换缺失" -ForegroundColor Red }
if ($udv2 -match "/task-v2" -and $udv2 -match "/commit-v2") { Write-Host "  ✓ 跳转路径 /task-v2 和 /commit-v2" -ForegroundColor Green } else { Write-Host "  ✗ 跳转路径缺失" -ForegroundColor Red }
if ($udv2 -match "startDate: date, endDate: date, userName") { Write-Host "  ✓ query 参数结构正确" -ForegroundColor Green } else { Write-Host "  ✗ query 参数结构不完整" -ForegroundColor Red }

# ---- 6. TaskViewV2 URL 初始化 ----
Write-Host "`n[6] TaskViewV2.vue URL 初始化" -ForegroundColor Yellow
$tv2 = Get-Content "D:\My\PubCode\kanban\frontend\src\views\TaskViewV2.vue" -Raw
if ($tv2 -match "route\.query") { Write-Host "  ✓ 读取 route.query" -ForegroundColor Green } else { Write-Host "  ✗ 未读取 route.query" -ForegroundColor Red }
if ($tv2 -match "if \(startDate && endDate\)") { Write-Host "  ✓ 双参数校验" -ForegroundColor Green } else { Write-Host "  ✗ 双参数校验缺失" -ForegroundColor Red }
if ($tv2 -match "setFilter\('start_time'") { Write-Host "  ✓ setFilter start_time" -ForegroundColor Green } else { Write-Host "  ✗ setFilter start_time 缺失" -ForegroundColor Red }
if ($tv2 -match "setFilter\('user_name'") { Write-Host "  ✓ setFilter user_name" -ForegroundColor Green } else { Write-Host "  ✗ setFilter user_name 缺失" -ForegroundColor Red }
if ($tv2 -match "String\(userName\)") { Write-Host "  ✓ userName 转字符串" -ForegroundColor Green } else { Write-Host "  ✗ userName 未转字符串" -ForegroundColor Red }

# ---- 7. CommitViewV2 URL 初始化 ----
Write-Host "`n[7] CommitViewV2.vue URL 初始化" -ForegroundColor Yellow
$cv2 = Get-Content "D:\My\PubCode\kanban\frontend\src\views\CommitViewV2.vue" -Raw
if ($cv2 -match "route\.query") { Write-Host "  ✓ 读取 route.query" -ForegroundColor Green } else { Write-Host "  ✗ 未读取 route.query" -ForegroundColor Red }
if ($cv2 -match "if \(startDate && endDate\)") { Write-Host "  ✓ 双参数校验" -ForegroundColor Green } else { Write-Host "  ✗ 双参数校验缺失" -ForegroundColor Red }
if ($cv2 -match "setFilter\('commit_time'") { Write-Host "  ✓ setFilter commit_time（正确字段）" -ForegroundColor Green } else { Write-Host "  ✗ setFilter commit_time 缺失" -ForegroundColor Red }
if ($cv2 -notmatch "setFilter\('start_time'") { Write-Host "  ✓ 未误用 start_time 字段" -ForegroundColor Green } else { Write-Host "  ✗ 错误使用了 start_time（应为 commit_time）" -ForegroundColor Red }
if ($cv2 -match "setFilter\('user_name'") { Write-Host "  ✓ setFilter user_name" -ForegroundColor Green } else { Write-Host "  ✗ setFilter user_name 缺失" -ForegroundColor Red }

Write-Host "`n=== 静态验证完成 ===" -ForegroundColor Cyan
Write-Host "提示：浏览器交互验证（测试点 8/9/10）需手动执行，访问 http://localhost:8880" -ForegroundColor Gray
```
