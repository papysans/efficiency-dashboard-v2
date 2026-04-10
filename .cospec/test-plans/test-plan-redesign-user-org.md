# 测试方案：redesign-user-org（User 和 Org 页面重设计）

## 概述

本次变更重新设计了 User 和 Org 页面，核心包括：
- 数据库 `user_groups` 表新增 `org_name` 字段
- 后端 `listUsersV2` 增加 org 字段、org 筛选参数、合并虚拟组数据
- 新增 `GET /api/v2/group` 组织详情接口
- 前端 `UserViewV2.vue` 重构（FilterBar + 新列顺序 + 虚拟组统一）
- 新建 `GroupView.vue` 组织详情页
- 路由新增 `/group`

测试策略：优先集成测试覆盖核心 API 行为，辅以静态检查验证文件结构和数据库 DDL。

---

## 测试点列表

### 1. 数据库：user_groups 表包含 org_name 字段

- **类型**: integration
- **描述**: 验证 `user_groups` 表已成功迁移，包含 `org_name VARCHAR(200)` 字段
- **测试场景**:
  - 查询 `information_schema.columns` 确认字段存在
  - 确认字段类型为 `character varying`，最大长度为 200
  - 确认默认值为空字符串 `''`
- **预期结果**: 查询返回 `org_name` 字段，数据类型 `character varying`，字符最大长度 200
- **测试用例文件**: `backend/user_group_handler_v2_integration_test.go`

**测试命令（PowerShell）**:
```powershell
$env:PGPASSWORD='1'; psql -U postgres -d costrict_stat -c "SELECT column_name, data_type, character_maximum_length, column_default FROM information_schema.columns WHERE table_name = 'user_groups' AND column_name = 'org_name';"
```

**预期输出**:
```
 column_name | data_type         | character_maximum_length | column_default
-------------+-------------------+--------------------------+----------------
 org_name    | character varying |                      200 | ''::character varying
```

---

### 2. 后端编译：backend/ 目录无编译错误

- **类型**: integration
- **描述**: 验证后端所有 Go 代码可以正常编译，无语法错误或类型错误
- **测试场景**: 在 `backend/` 目录执行 `go build ./...`
- **预期结果**: 命令退出码为 0，无错误输出
- **测试用例文件**: 无（编译验证）

**测试命令（PowerShell）**:
```powershell
cd D:\My\PubCode\kanban\backend; go build ./...
```

---

### 3. 后端 API：GET /api/v2/users 返回 org 相关字段

- **类型**: integration
- **描述**: 验证 `listUsersV2` 响应中每条用户记录包含 `org1/org2/org3/org4/org_display/is_virtual_group/org_name` 字段
- **测试场景**:
  - 调用 `GET /api/v2/users`（不带任何筛选参数）
  - 检查响应体 `data` 数组中普通用户记录的字段结构
  - 检查虚拟组记录（`is_virtual_group: true`）的字段结构
- **预期结果**:
  - 普通用户：`org1/org2/org3/org4` 为字符串（可为空），`org_display` 为非空层级用 `/` 拼接，`is_virtual_group: false`
  - 虚拟组：`is_virtual_group: true`，`org_display` 等于 `org_name`
- **测试用例文件**: `backend/user_handler_v2_integration_test.go`

**测试命令（服务运行时）**:
```powershell
Invoke-WebRequest -Uri "http://localhost:9990/api/v2/users?pageSize=5" -Method GET | Select-Object -ExpandProperty Content
```

---

### 4. 后端 API：GET /api/v2/users?org1=xxx 支持按组织筛选

- **类型**: integration
- **描述**: 验证 `listUsersV2` 支持 `org1/org2/org3/org4` 查询参数过滤用户
- **测试场景**:
  - 先查询全量用户，找到某个 `org1` 非空的用户
  - 再用该 `org1` 值作为参数请求，验证返回结果中所有普通用户的 `org1` 字段与参数一致
  - 验证虚拟组（`is_virtual_group: true`）不受 org 参数过滤，始终出现在结果中
- **预期结果**:
  - 所有普通用户行的 `org1` 字段 == 传入的 `org1` 参数值
  - 虚拟组行不受影响，仍出现在响应中
- **测试用例文件**: `backend/user_handler_v2_integration_test.go`

**测试命令（服务运行时）**:
```powershell
# 先获取全量，找到 org1 值
$resp = Invoke-WebRequest -Uri "http://localhost:9990/api/v2/users?pageSize=100" -Method GET | Select-Object -ExpandProperty Content | ConvertFrom-Json
$org1Val = ($resp.data | Where-Object { $_.org1 -ne "" } | Select-Object -First 1).org1
Write-Host "Testing org1 filter with: $org1Val"

# 按 org1 筛选
Invoke-WebRequest -Uri "http://localhost:9990/api/v2/users?org1=$org1Val&pageSize=100" -Method GET | Select-Object -ExpandProperty Content
```

---

### 5. 后端 API：GET /api/v2/group 返回正确结构

- **类型**: integration
- **描述**: 验证新增的 `getGroupDetailV2` 接口返回 `{ org_path, summary, daily, members }` 结构
- **测试场景**:
  - 调用 `GET /api/v2/group`（不带任何参数，匹配所有用户）
  - 调用 `GET /api/v2/group?org1=xxx`（带有效的 org1 参数）
  - 调用 `GET /api/v2/group?org1=不存在的组织`（无匹配用户）
- **预期结果**:
  - 正常情况：响应包含 `org_path`（字符串）、`summary`（含 `task_diff_lines/commit_diff_lines/task_efficiency_ratio/commit_efficiency_ratio/cost` 等字段）、`daily`（数组）、`members`（数组，每项含 `user_id/user_name/task_diff_lines/commit_diff_lines/task_efficiency_ratio/commit_efficiency_ratio`）
  - 无匹配用户：返回 `{ org_path: "xxx", summary: {}, daily: [], members: [] }`
- **测试用例文件**: `backend/org_handler_v2_integration_test.go`

**测试命令（服务运行时）**:
```powershell
# 不带参数
Invoke-WebRequest -Uri "http://localhost:9990/api/v2/group" -Method GET | Select-Object -ExpandProperty Content | ConvertFrom-Json | Select-Object org_path, @{n='summary_keys';e={$_.summary.PSObject.Properties.Name}}, @{n='daily_count';e={$_.daily.Count}}, @{n='members_count';e={$_.members.Count}}

# 带 org1 参数（替换为实际存在的 org1 值）
Invoke-WebRequest -Uri "http://localhost:9990/api/v2/group?org1=技术部" -Method GET | Select-Object -ExpandProperty Content

# 不存在的组织
Invoke-WebRequest -Uri "http://localhost:9990/api/v2/group?org1=__不存在的组织__" -Method GET | Select-Object -ExpandProperty Content
```

---

### 6. 后端 API：POST /api/v2/user-groups 支持 org_name 字段

- **类型**: integration
- **描述**: 验证创建虚拟组时可传入 `org_name` 字段，且返回结果中包含该字段
- **测试场景**:
  - 正常场景：传入 `{ name, org_name, user_ids }` 创建虚拟组，检查返回的 `org_name` 字段
  - 边界场景：不传 `org_name` 字段，验证默认值为空字符串
  - 异常场景：不传 `name` 或 `user_ids` 为空，验证返回 400 错误
- **预期结果**:
  - 传入 `org_name` 时，响应 JSON 中 `org_name` 等于传入值
  - 不传 `org_name` 时，响应 JSON 中 `org_name` 为 `""`
  - 缺少必填字段时，返回 HTTP 400
- **测试用例文件**: `backend/user_group_handler_v2_integration_test.go`

**测试命令（服务运行时）**:
```powershell
# 正常创建（含 org_name）
$body = '{"name":"测试虚拟组","org_name":"技术架构组织","user_ids":["test-user-001"]}'
$resp = Invoke-WebRequest -Uri "http://localhost:9990/api/v2/user-groups" -Method POST -ContentType "application/json" -Body $body | Select-Object -ExpandProperty Content | ConvertFrom-Json
Write-Host "group_id: $($resp.group_id), org_name: $($resp.org_name)"

# 清理：删除刚创建的组
Invoke-WebRequest -Uri "http://localhost:9990/api/v2/user-groups/$($resp.group_id)" -Method DELETE

# 不传 org_name
$body2 = '{"name":"测试虚拟组2","user_ids":["test-user-001"]}'
$resp2 = Invoke-WebRequest -Uri "http://localhost:9990/api/v2/user-groups" -Method POST -ContentType "application/json" -Body $body2 | Select-Object -ExpandProperty Content | ConvertFrom-Json
Write-Host "org_name default: '$($resp2.org_name)'"
Invoke-WebRequest -Uri "http://localhost:9990/api/v2/user-groups/$($resp2.group_id)" -Method DELETE

# 缺少必填字段（应返回 400）
try {
  Invoke-WebRequest -Uri "http://localhost:9990/api/v2/user-groups" -Method POST -ContentType "application/json" -Body '{"name":"仅名称"}'
} catch {
  Write-Host "Expected error: $($_.Exception.Response.StatusCode)"
}
```

---

### 7. 后端 API：GET /api/v2/users 列表中虚拟组数据合并

- **类型**: integration
- **描述**: 验证 `listUsersV2` 响应的 `data` 数组末尾包含虚拟组记录，且虚拟组记录具有正确的标记字段
- **测试场景**:
  - 先创建一个虚拟组（含 `org_name`）
  - 调用 `GET /api/v2/users` 获取列表
  - 在 `data` 中找到 `is_virtual_group: true` 的记录
  - 验证该记录的 `org_display` 等于虚拟组的 `org_name`，`user_name` 等于虚拟组名称
  - 清理：删除测试虚拟组
- **预期结果**: 虚拟组出现在列表末尾，`is_virtual_group: true`，`org_display == org_name`，`org1/org2/org3/org4` 均为 `""`
- **测试用例文件**: `backend/user_handler_v2_integration_test.go`

---

### 8. 后端 API：GET /api/v2/group 日期参数过滤

- **类型**: integration
- **描述**: 验证 `getGroupDetailV2` 支持 `startDate/endDate` 参数，按日期范围过滤统计数据
- **测试场景**:
  - 调用时传入合法日期范围（`startDate=20250101&endDate=20250131`）
  - 调用时传入非法日期格式（`startDate=invalid`）
- **预期结果**:
  - 合法日期：正常返回，`daily` 中的日期均在指定范围内
  - 非法日期：返回 HTTP 400，错误信息包含 "startDate 格式错误"
- **测试用例文件**: `backend/org_handler_v2_integration_test.go`

**测试命令（服务运行时）**:
```powershell
# 合法日期
Invoke-WebRequest -Uri "http://localhost:9990/api/v2/group?startDate=20250101&endDate=20250131" -Method GET | Select-Object -ExpandProperty Content | ConvertFrom-Json | Select-Object org_path, @{n='daily_count';e={$_.daily.Count}}

# 非法日期（应返回 400）
try {
  Invoke-WebRequest -Uri "http://localhost:9990/api/v2/group?startDate=invalid" -Method GET
} catch {
  Write-Host "Expected 400: $($_.Exception.Response.StatusCode)"
}
```

---

### 9. 前端文件：GroupView.vue 存在且包含关键内容

- **类型**: integration（静态文件检查）
- **描述**: 验证 `frontend/src/views/GroupView.vue` 文件已创建，且包含组织详情页的关键组件
- **测试场景**:
  - 检查文件存在性
  - 检查文件包含 `FilterBar`、`getGroupDetail`、`org_path`、成员列表 `el-table` 等关键内容
- **预期结果**: 文件存在，包含所有关键标识符
- **测试用例文件**: 无（文件检查）

**测试命令（PowerShell）**:
```powershell
# 文件存在性
$file = "D:\My\PubCode\kanban\frontend\src\views\GroupView.vue"
if (Test-Path $file) { Write-Host "✓ GroupView.vue 存在" } else { Write-Host "✗ GroupView.vue 不存在" }

# 关键内容检查
$content = Get-Content $file -Raw
$checks = @("FilterBar", "getGroupDetail", "org_path", "members", "el-table", "router.back()", "commit_efficiency_ratio", "task_efficiency_ratio")
foreach ($check in $checks) {
  if ($content -match [regex]::Escape($check)) {
    Write-Host "✓ 包含: $check"
  } else {
    Write-Host "✗ 缺少: $check"
  }
}
```

---

### 10. 前端文件：router/index.js 包含 /group 路由

- **类型**: integration（静态文件检查）
- **描述**: 验证 `frontend/src/router/index.js` 包含 `/group` 路由配置，且指向 `GroupView.vue`
- **测试场景**:
  - 检查路由文件中是否存在 `path: '/group'`
  - 检查是否引用了 `GroupView.vue`
- **预期结果**: 路由文件包含 `/group` 路由，组件为 `GroupView.vue`
- **测试用例文件**: 无（文件检查）

**测试命令（PowerShell）**:
```powershell
$routerFile = "D:\My\PubCode\kanban\frontend\src\router\index.js"
$content = Get-Content $routerFile -Raw
if ($content -match "path: '/group'") { Write-Host "✓ /group 路由存在" } else { Write-Host "✗ /group 路由缺失" }
if ($content -match "GroupView\.vue") { Write-Host "✓ 引用 GroupView.vue" } else { Write-Host "✗ 未引用 GroupView.vue" }
if ($content -match "name: 'GroupView'") { Write-Host "✓ 路由名称 GroupView 存在" } else { Write-Host "✗ 路由名称缺失" }
```

---

### 11. 前端文件：UserViewV2.vue 包含 FilterBar 和组织列

- **类型**: integration（静态文件检查）
- **描述**: 验证 `UserViewV2.vue` 已重构，包含 FilterBar 筛选区、组织列（`org_display`）、虚拟组行样式、`org_name` 字段
- **测试场景**:
  - 检查 `FilterBar` 组件引用
  - 检查 `org_display` 列定义
  - 检查 `is_virtual_group` 行样式逻辑
  - 检查 `groupOrgName` 响应式变量（虚拟组弹窗 org_name 字段）
  - 检查 `handleOrgClick` 函数（组织列点击跳转 /group）
- **预期结果**: 所有关键内容均存在
- **测试用例文件**: 无（文件检查）

**测试命令（PowerShell）**:
```powershell
$file = "D:\My\PubCode\kanban\frontend\src\views\UserViewV2.vue"
$content = Get-Content $file -Raw
$checks = @(
  "FilterBar",
  "org_display",
  "is_virtual_group",
  "virtual-group-row",
  "groupOrgName",
  "handleOrgClick",
  "filterOrg1",
  "filterOrg2",
  "filterOrg3",
  "filterOrg4",
  "/group"
)
foreach ($check in $checks) {
  if ($content -match [regex]::Escape($check)) {
    Write-Host "✓ 包含: $check"
  } else {
    Write-Host "✗ 缺少: $check"
  }
}
```

---

### 12. 集成测试：运行已有集成测试套件不报错

- **类型**: integration
- **描述**: 确保本次变更未破坏已有集成测试
- **测试场景**: 在 `backend/` 目录运行所有集成测试
- **预期结果**: 所有测试通过，无 FAIL
- **测试用例文件**: `backend/*_integration_test.go`

**测试命令（PowerShell）**:
```powershell
cd D:\My\PubCode\kanban\backend; go test -tags integration -v -count=1 ./...
```

---

## 关键考虑事项

1. **org_mapping.csv 依赖**：`listUsersV2` 和 `getGroupDetailV2` 均依赖 `orgMappings` 全局变量（从 `org_mapping.csv` 加载）。若 CSV 文件不存在或为空，org 字段将全部为空字符串，org 筛选将无法过滤任何用户。测试时需确认 `org_mapping.csv` 已存在且有数据。

2. **虚拟组 org_name 与 org_display 的一致性**：虚拟组记录中 `org_display` 应等于 `org_name`。若 `org_name` 为空字符串，`org_display` 也为空字符串，前端组织列显示 `-`。

3. **org 筛选不影响虚拟组**：根据代码实现，org1/org2/org3/org4 筛选参数只对普通用户生效，虚拟组始终追加到结果末尾，不受过滤。测试时需验证此行为。

4. **效率比计算精度**：`task_efficiency_ratio` 和 `commit_efficiency_ratio` 使用 `math.Round` 取整，测试时注意浮点比较使用 `>=` 而非精确等于。

5. **分页与虚拟组位置**：虚拟组追加在内存分页之前，因此虚拟组始终出现在最后一页。测试时若页面较小，虚拟组可能不在第一页。

6. **数据库迁移验证**：`init_db_stat.sql` 中已包含 `org_name` 字段，但现有数据库可能通过 migration 工具添加。需验证实际数据库中字段存在，而非仅检查 DDL 文件。

7. **前端构建验证**：静态文件检查无法替代实际运行测试。若有条件，建议在 `frontend/` 目录执行 `npm run build` 验证无编译错误。

---

## 测试用例文件清单

> 注：以下为建议新增的集成测试文件，需在 `backend/` 目录下创建。

- `backend/user_handler_v2_integration_test.go` — 覆盖测试点 3、4、7
- `backend/user_group_handler_v2_integration_test.go` — 覆盖测试点 1、6
- `backend/org_handler_v2_integration_test.go` — 覆盖测试点 5、8

---

## 一键验证脚本（PowerShell）

以下脚本可在不启动后端服务的情况下，完成静态验证部分：

```powershell
Write-Host "=== redesign-user-org 变更验证 ===" -ForegroundColor Cyan

# 1. 数据库字段验证
Write-Host "`n[1] 数据库 user_groups.org_name 字段" -ForegroundColor Yellow
$env:PGPASSWORD='1'; psql -U postgres -d costrict_stat -c "SELECT column_name, data_type, character_maximum_length FROM information_schema.columns WHERE table_name = 'user_groups' AND column_name = 'org_name';"

# 2. 后端编译
Write-Host "`n[2] 后端编译" -ForegroundColor Yellow
Push-Location D:\My\PubCode\kanban\backend
go build ./...
if ($LASTEXITCODE -eq 0) { Write-Host "✓ 编译成功" -ForegroundColor Green } else { Write-Host "✗ 编译失败" -ForegroundColor Red }
Pop-Location

# 3. 前端文件检查
Write-Host "`n[3] 前端文件检查" -ForegroundColor Yellow

$groupViewFile = "D:\My\PubCode\kanban\frontend\src\views\GroupView.vue"
if (Test-Path $groupViewFile) { Write-Host "✓ GroupView.vue 存在" -ForegroundColor Green } else { Write-Host "✗ GroupView.vue 不存在" -ForegroundColor Red }

$routerFile = "D:\My\PubCode\kanban\frontend\src\router\index.js"
$routerContent = Get-Content $routerFile -Raw
if ($routerContent -match "path: '/group'") { Write-Host "✓ /group 路由存在" -ForegroundColor Green } else { Write-Host "✗ /group 路由缺失" -ForegroundColor Red }
if ($routerContent -match "GroupView\.vue") { Write-Host "✓ 路由引用 GroupView.vue" -ForegroundColor Green } else { Write-Host "✗ 路由未引用 GroupView.vue" -ForegroundColor Red }

$userViewFile = "D:\My\PubCode\kanban\frontend\src\views\UserViewV2.vue"
$userViewContent = Get-Content $userViewFile -Raw
$userViewChecks = @("FilterBar", "org_display", "is_virtual_group", "virtual-group-row", "groupOrgName", "handleOrgClick", "/group")
foreach ($check in $userViewChecks) {
  if ($userViewContent -match [regex]::Escape($check)) {
    Write-Host "✓ UserViewV2.vue 包含: $check" -ForegroundColor Green
  } else {
    Write-Host "✗ UserViewV2.vue 缺少: $check" -ForegroundColor Red
  }
}

# 4. init_db_stat.sql DDL 检查
Write-Host "`n[4] init_db_stat.sql DDL 检查" -ForegroundColor Yellow
$sqlFile = "D:\My\PubCode\kanban\init_db_stat.sql"
$sqlContent = Get-Content $sqlFile -Raw
if ($sqlContent -match "org_name VARCHAR\(200\)") { Write-Host "✓ init_db_stat.sql 包含 org_name 字段" -ForegroundColor Green } else { Write-Host "✗ init_db_stat.sql 缺少 org_name 字段" -ForegroundColor Red }

Write-Host "`n=== 验证完成 ===" -ForegroundColor Cyan
```

---

## 集成测试代码（新增文件）

### backend/user_group_handler_v2_integration_test.go

```go
//go:build integration

package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserGroupOrgName_DBSchema(t *testing.T) {
	initTestDB(t)
	var colName, dataType string
	var maxLen int
	err := statDB.QueryRow(`
		SELECT column_name, data_type, character_maximum_length
		FROM information_schema.columns
		WHERE table_name = 'user_groups' AND column_name = 'org_name'
	`).Scan(&colName, &dataType, &maxLen)
	require.NoError(t, err, "org_name 字段应存在于 user_groups 表")
	assert.Equal(t, "org_name", colName)
	assert.Equal(t, "character varying", dataType)
	assert.Equal(t, 200, maxLen)
}

func TestCreateUserGroup_WithOrgName(t *testing.T) {
	initTestDB(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/v2/user-groups", createUserGroupHandler)

	body := `{"name":"test-group-orgname","org_name":"技术架构组织","user_ids":["test-uid-001"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v2/user-groups", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "技术架构组织", resp["org_name"])
	groupID := resp["group_id"].(string)

	// 清理
	t.Cleanup(func() {
		statDB.Exec("DELETE FROM user_groups WHERE group_id = $1", groupID)
	})
}

func TestCreateUserGroup_WithoutOrgName(t *testing.T) {
	initTestDB(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/v2/user-groups", createUserGroupHandler)

	body := `{"name":"test-group-no-orgname","user_ids":["test-uid-002"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v2/user-groups", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "", resp["org_name"], "未传 org_name 时默认应为空字符串")
	groupID := resp["group_id"].(string)

	t.Cleanup(func() {
		statDB.Exec("DELETE FROM user_groups WHERE group_id = $1", groupID)
	})
}

func TestCreateUserGroup_MissingRequired(t *testing.T) {
	initTestDB(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/v2/user-groups", createUserGroupHandler)

	// 缺少 user_ids
	body := `{"name":"test-group-missing-ids"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v2/user-groups", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
```

### backend/user_handler_v2_integration_test.go

```go
//go:build integration

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListUsersV2_OrgFields(t *testing.T) {
	initTestDB(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v2/users", listUsersV2)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/users?pageSize=10", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	data, ok := resp["data"].([]interface{})
	require.True(t, ok, "data 应为数组")

	// 检查每条记录包含 org 相关字段
	for _, item := range data {
		row := item.(map[string]interface{})
		assert.Contains(t, row, "org1", "响应应包含 org1 字段")
		assert.Contains(t, row, "org2", "响应应包含 org2 字段")
		assert.Contains(t, row, "org3", "响应应包含 org3 字段")
		assert.Contains(t, row, "org4", "响应应包含 org4 字段")
		assert.Contains(t, row, "org_display", "响应应包含 org_display 字段")
		assert.Contains(t, row, "is_virtual_group", "响应应包含 is_virtual_group 字段")
		assert.Contains(t, row, "org_name", "响应应包含 org_name 字段")
	}
}

func TestListUsersV2_VirtualGroupMerged(t *testing.T) {
	initTestDB(t)

	// 创建测试虚拟组
	group, err := CreateUserGroup(statDB, "test-vgroup-merge", "测试虚拟组织", []string{"test-uid-vg"})
	require.NoError(t, err)
	t.Cleanup(func() {
		statDB.Exec("DELETE FROM user_groups WHERE group_id = $1", group.GroupID)
	})

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v2/users", listUsersV2)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/users?pageSize=1000", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	data := resp["data"].([]interface{})
	var found bool
	for _, item := range data {
		row := item.(map[string]interface{})
		if row["is_virtual_group"] == true && row["user_name"] == "test-vgroup-merge" {
			found = true
			assert.Equal(t, "测试虚拟组织", row["org_display"], "虚拟组 org_display 应等于 org_name")
			assert.Equal(t, "测试虚拟组织", row["org_name"])
			assert.Equal(t, "", row["org1"])
			assert.Equal(t, "", row["org2"])
			assert.Equal(t, "", row["org3"])
			assert.Equal(t, "", row["org4"])
		}
	}
	assert.True(t, found, "用户列表中应包含虚拟组记录")
}
```

### backend/org_handler_v2_integration_test.go（新增 group 接口测试）

```go
//go:build integration

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetGroupDetailV2_Structure(t *testing.T) {
	initTestDB(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v2/group", getGroupDetailV2)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/group", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Contains(t, resp, "org_path", "响应应包含 org_path 字段")
	assert.Contains(t, resp, "summary", "响应应包含 summary 字段")
	assert.Contains(t, resp, "daily", "响应应包含 daily 字段")
	assert.Contains(t, resp, "members", "响应应包含 members 字段")
}

func TestGetGroupDetailV2_NoMatchReturnsEmpty(t *testing.T) {
	initTestDB(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v2/group", getGroupDetailV2)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/group?org1=__不存在的组织__", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Equal(t, "__不存在的组织__", resp["org_path"])
	daily, ok := resp["daily"].([]interface{})
	assert.True(t, ok)
	assert.Empty(t, daily, "无匹配用户时 daily 应为空数组")
	members, ok := resp["members"].([]interface{})
	assert.True(t, ok)
	assert.Empty(t, members, "无匹配用户时 members 应为空数组")
}

func TestGetGroupDetailV2_InvalidDate(t *testing.T) {
	initTestDB(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v2/group", getGroupDetailV2)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/group?startDate=invalid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
```
