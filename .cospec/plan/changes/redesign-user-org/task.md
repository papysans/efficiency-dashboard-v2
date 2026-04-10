## 实施

### 一、数据库变更

- [x] 1.1 user_groups 表新增 org_name 字段
     【目标对象】`init_db_stat.sql`（user_groups 表 DDL）；migration 脚本写入 `tools/migrate-user-groups-org-name/main.go`
     【修改目的】支持虚拟组设置自定义组织名称
     【修改方式】在 `CREATE TABLE IF NOT EXISTS user_groups` 的 `name` 字段后新增一行；新建 migration 工具执行 ALTER TABLE
     【相关依赖】`init_db_stat.sql`（DDL 同步）
     【修改内容】
        - `init_db_stat.sql`：在 `name VARCHAR(500) NOT NULL,` 后新增 `org_name VARCHAR(200) DEFAULT '',`
        - 新建 `tools/migrate-user-groups-org-name/main.go`，连接 costrict_stat 数据库，执行：`ALTER TABLE user_groups ADD COLUMN IF NOT EXISTS org_name VARCHAR(200) DEFAULT '';`
        - 执行该 migration 工具完成数据库变更

---

### 二、后端变更

- [x] 2.1 UserGroup struct 及 CRUD 函数支持 org_name 字段
     【目标对象】`backend/db.go`（UserGroup struct 第 1967-1973 行；CreateUserGroup 第 1978 行；ListUserGroups 第 2001 行）
     【修改目的】Go struct 与数据库字段同步，CRUD 函数支持 org_name 读写
     【修改方式】修改 struct 定义、INSERT SQL、SELECT SQL 及对应 Scan
     【相关依赖】任务 1.1（数据库已新增 org_name 列）
     【修改内容】
        - `UserGroup` struct 新增：`OrgName string \`json:"org_name"\``
        - `CreateUserGroup` 函数签名新增 `orgName string` 参数；INSERT SQL 中加入 `org_name`，VALUES 中对应传入 `orgName`；Scan 结果加入 `OrgName`
        - `ListUserGroups` 函数：SELECT 中加入 `org_name`，Scan 中加入 `&g.OrgName`

- [x] 2.2 createUserGroupHandler 支持 org_name 字段
     【目标对象】`backend/user_group_handler_v2.go`
     【修改目的】创建虚拟组时可传入 org_name
     【修改方式】修改请求体 struct 解析和 CreateUserGroup 调用
     【相关依赖】任务 2.1（CreateUserGroup 函数签名已更新）
     【修改内容】
        - 请求体 struct 新增 `OrgName string \`json:"org_name"\`` 字段
        - 调用 `CreateUserGroup(statDB, req.Name, req.OrgName, userIDs)` 时传入 `req.OrgName`

- [x] 2.3 listUsersV2 增加 org 字段、org 筛选参数、合并虚拟组数据
     【目标对象】`backend/user_handler_v2.go`
     【修改目的】用户列表响应增加组织信息，支持按组织筛选，虚拟组与用户统一列表展示
     【修改方式】修改 listUsersV2 handler：扩展响应 struct、新增查询参数处理、追加虚拟组聚合逻辑
     【相关依赖】`orgMappings`（全局 map，`backend/org_handler_v2.go`）、`ListUserGroups`、`ListUserProductivity`（`backend/db.go`）
     【修改内容】
        - 响应 struct 新增字段：`Org1, Org2, Org3, Org4 string`、`OrgDisplay string`（非空层级用"/"拼接）、`IsVirtualGroup bool`、`OrgName string`（虚拟组专用）
        - 查询参数新增：`org1, org2, org3, org4 string`（可选，逐级匹配非空参数）
        - **普通用户处理**：现有 SQL 聚合逻辑不变；聚合结果遍历时，从 `orgMappings[uid]` 填充 `Org1/2/3/4`，拼接 `OrgDisplay`（跳过空层级）；若 org1/2/3/4 查询参数非空，则在内存中过滤不匹配的用户行
        - **虚拟组处理**：调用 `ListUserGroups(statDB)` 获取所有虚拟组；对每个虚拟组，解析 `user_ids`，对每个成员 uid 调用 `ListUserProductivity(statDB, uid, startTime, endTime, 1, 100000)` 获取明细，在内存中聚合成员级汇总（参照 `getUserGroupDetailHandler` 的聚合逻辑）；将聚合结果追加到响应列表末尾，`IsVirtualGroup=true`，`OrgDisplay=group.OrgName`，`OrgName=group.OrgName`
        - 虚拟组不参与 org1/2/3/4 参数过滤（始终显示）

- [x] 2.4 新增 GET /api/v2/group 接口（组织详情）
     【目标对象】`backend/org_handler_v2.go`（新增 `getGroupDetailV2` 函数）；`backend/main.go`（注册路由）
     【修改目的】为前端 /group 页面提供数据：组织汇总统计 + 按天趋势 + 成员列表
     【修改方式】在 org_handler_v2.go 末尾新增 handler 函数；在 main.go v2 路由组中注册
     【相关依赖】`orgMappings`、`ListUserProductivity`（`backend/db.go`）、`parseDateParam`
     【修改内容】
        - 新增 `getGroupDetailV2` handler，接收参数：`org1, org2, org3, org4, startDate, endDate`
        - 内部逻辑：
          1. 按 org1/org2/org3/org4 从 orgMappings 筛选该组织下的所有用户（复用 `filterUsersByParent` 或直接遍历 orgMappings 逐级匹配）
          2. 对每个用户调用 `ListUserProductivity(statDB, uid, startTime, endTime, 1, 100000)` 获取按天明细
          3. 按日期（`create_time` 截断到天，格式 `YYYY-MM-DD`）聚合所有用户的数据：daily 数组每项为 `{ date, task_diff_lines, commit_diff_lines, task_real_minutes, task_ancient_minutes, commit_real_minutes, commit_ancient_minutes, upstream_tokens, downstream_tokens, cost }`，task_efficiency_ratio 和 commit_efficiency_ratio 在 daily 中按当天聚合值重新计算（ancient/real 之比）
          4. 汇总 summary（对所有用户所有天的数据求和，效率比 = 总 ancient / 总 real * 100）
          5. 按用户维度汇总 members 列表（每个用户的汇总指标，含 user_id、user_name、org 字段）
          6. 返回 `{ org_path, summary, daily: []DailyItem, members: []MemberItem }`
        - `main.go` v2 路由组新增：`v2.GET("/group", getGroupDetailV2)`

---

### 三、前端 API 层变更

- [x] 3.1 新增/更新前端 API 函数
     【目标对象】`frontend/src/api/es.js`（根据已有 getUserGroups、createUserGroup 等函数所在文件）
     【修改目的】对接新增和修改的后端接口
     【修改方式】在现有 API 函数旁边新增/修改
     【修改内容】
        - `listUsersV2(params)` 函数：确认 params 对象透传，调用方可传入 `org1/org2/org3/org4` 字段（无需修改函数本身，确认已使用 `params` 对象透传即可）
        - 新增 `getGroupDetail(params)` 函数：`request({ url: '/v2/group', method: 'get', params })`，params 含 `org1/org2/org3/org4/startDate/endDate`

---

### 四、前端 UserViewV2.vue 重构

- [x] 4.1 添加 FilterBar（日期范围 + 4级联组织 Select）
     【目标对象】`frontend/src/views/UserViewV2.vue`
     【修改目的】在用户列表上方添加日期范围筛选和组织级联筛选，复用 OrgViewV2 的实现模式
     【修改方式】在现有模板 KbFilterTable 上方插入 FilterBar；JS 中新增响应式变量和级联加载函数
     【相关依赖】`FilterBar.vue`（`frontend/src/components/FilterBar.vue`）、`getOrgV2` API（已有）、`getDefaultDateRangeWide`（`frontend/src/utils/date.js`）
     【修改内容】
        - 模板：在 KbFilterTable 上方插入 `<FilterBar v-model:dateRange="dateRange" @search="handleQuery">`，slot 内放 4 个级联 el-select（org1~org4，样式和逻辑完全复用 OrgViewV2.vue 中的写法）
        - JS 新增响应式变量：`dateRange`（初始值 `getDefaultDateRangeWide()`）、`filterOrg1/2/3/4`（初始 `''`）、`org2Options/org3Options/org4Options`（初始 `[]`）
        - JS 新增级联加载函数：`onOrg1Change`、`onOrg2Change`、`onOrg3Change`（逻辑完全复用 OrgViewV2.vue 中的对应函数）
        - `handleQuery` 函数：重置 `page.value = 1`，调用 `fetchData()`
        - `fetchData` 中将 `dateRange` 转为 `startDate/endDate`（`YYYYMMDD` 格式），将 `filterOrg1/2/3/4` 非空值作为查询参数传给后端

- [x] 4.2 修改列定义（字段顺序 + 组织列 + 跳转逻辑）
     【目标对象】`frontend/src/views/UserViewV2.vue`
     【修改目的】按需求调整列顺序，新增组织列，组织列和用户名列点击跳转对应页面
     【修改方式】修改 columns 数组定义，新增 handleOrgClick 函数
     【修改内容】
        - 列顺序调整为：组织、用户名、commit代码量、commit实际耗时、commit提效比、task代码量、task实际耗时、task提效比、token消耗、费用
        - 新增「组织」列（prop: `org_display`，slotName: `org_display`）：自定义渲染为 `el-link`，点击调用 `handleOrgClick(row)`；真实用户跳转 `/group?org1=xxx&org2=xxx&org3=xxx&org4=xxx`（仅传入非空的 org 层级参数）；虚拟组行的组织列不可点击（无跳转，仅显示 org_name 文字）
        - 「用户名」列（slotName: `user_name`）：真实用户跳转 `/user/:userId`；虚拟组跳转 `/user/group/:groupId`（groupId 来自 row.group_id）
        - 移除旧的 `create_time` 服务端日期列（日期筛选已由 FilterBar 承担）

- [x] 4.3 虚拟组统一到用户列表，移除独立虚拟组卡片
     【目标对象】`frontend/src/views/UserViewV2.vue`
     【修改目的】虚拟组与用户在同一列表展示，移除底部独立的虚拟组卡片区域
     【修改方式】删除虚拟组卡片模板区域；调整 fetchData 直接使用后端合并后的列表数据
     【修改内容】
        - 删除模板中的虚拟组卡片区域（`el-card` + `v-for="g in groups"` 遍历块）
        - 删除 `groups` 响应式变量及 `getUserGroups()` 调用
        - `fetchData` 响应数据直接赋值给 `tableData`（后端已在 data 列表末尾合并虚拟组，`is_virtual_group: true`）
        - KbFilterTable 上设置 `:row-class-name="getRowClass"`，`getRowClass` 函数：`row.is_virtual_group ? 'virtual-group-row' : ''`
        - CSS 新增：`.virtual-group-row { background-color: #f0f9eb; }` 以及 `.virtual-group-row:hover > td { background-color: #e1f3d8 !important; }`

- [x] 4.4 新建虚拟组弹窗新增 org_name 字段
     【目标对象】`frontend/src/views/UserViewV2.vue`
     【修改目的】创建虚拟组时可填写组织名称
     【修改方式】在现有创建虚拟组弹窗的 el-form 中新增一个表单项
     【修改内容】
        - 弹窗 el-form 中新增「组织名称」el-form-item（非必填），内含 `el-input v-model="groupOrgName" placeholder="如：技术架构组织"`
        - 新增响应式变量 `const groupOrgName = ref('')`
        - 创建时调用：`createUserGroup({ name: groupName.value, org_name: groupOrgName.value, user_ids: selectedUsers.map(r => r.user_id) })`
        - 弹窗关闭/重置时：`groupOrgName.value = ''`

---

### 五、新建 GroupView.vue（组织详情页）

- [x] 5.1 新建 GroupView.vue
     【目标对象】`frontend/src/views/GroupView.vue`（新建文件）
     【修改目的】展示某组织节点的统计数据，布局和交互设计与 UserDetailV2.vue 保持一致
     【相关依赖】`getGroupDetail` API（任务 3.1）、`FilterBar.vue`、`useChart`（`frontend/src/composables/useChart.js`）、`formatDuration`、`fmtCost`（`frontend/src/utils/formatters.js`）、`getDefaultDateRangeWide`
     【修改内容】
        - 从 `route.query` 读取 `org1/org2/org3/org4`，拼接面包屑标题（过滤空值后用" / "连接）
        - 顶部 el-card：返回按钮（`router.back()`）+ 组织路径标题
        - FilterBar：`v-model:dateRange` + `@search="fetchData"`，日期变化时重新请求
        - 统计指标卡（`el-row :gutter="12"`，6个 `el-col :span="4"`）：commit代码量（formatDuration）、commit提效比（el-tag 颜色）、task代码量、task提效比、token消耗、费用（fmtCost）
        - 趋势图（`useChart` + ECharts 双折线）：参照 `UserDetailV2.vue` 中 `updateTrendChart` 的实现方式，在 `GroupView.vue` 中独立实现同逻辑的 `updateTrendChart` 函数（x轴为 daily[].date，两条线为 task_efficiency_ratio 和 commit_efficiency_ratio）
        - 成员列表（el-table）：用户名列（`el-link` 点击跳转 `/user/:userId`）、commit代码量、commit提效比（el-tag）、task代码量、task提效比（el-tag）、费用；提效比颜色规则：`>=300` success / `>=150` primary / 其他 info

- [x] 5.2 UserDetailV2.vue 微调，与 GroupView 保持一致的展示风格
     【目标对象】`frontend/src/views/UserDetailV2.vue`
     【修改目的】统一两个详情页的视觉风格，确保指标卡布局、趋势图样式、提效比颜色规则一致
     【修改方式】对照 GroupView.vue 的实现，微调 UserDetailV2 中不一致的部分
     【修改内容】
        - 统计指标卡：若当前不是 6 列布局（`el-col :span="4"`），调整为与 GroupView 一致的 6 列布局
        - 提效比 el-tag 颜色规则：确认与 GroupView 使用相同的判断逻辑（`>=300` success / `>=150` primary / 其他 info）
        - 趋势图：确认 x轴日期格式、折线颜色与 GroupView 一致

- [x] 5.3 新增 /group 路由
     【目标对象】`frontend/src/router/index.js`
     【修改目的】注册 GroupView 路由，使 /group?org1=xxx 可访问
     【修改方式】在 `/org-v2` 路由附近新增一条路由配置
     【修改内容】
        - 新增：`{ path: '/group', name: 'GroupView', component: () => import('../views/GroupView.vue') }`

---

### 代码审查修复

- [x] redesign-user-org | task: 2.1-fix-1 GetUserGroup 函数补充 org_name 字段
     【目标对象】`backend/db.go`（GetUserGroup 函数，第2025-2042行）
     【修改目的】GetUserGroup 的 SELECT 语句缺少 org_name 字段，导致通过 getUserGroupDetailHandler 获取虚拟组详情时 group.OrgName 始终为空字符串
     【修改方式】在 SELECT 语句中加入 org_name，在 Scan 中加入 &g.OrgName
     【修改内容】
        - SELECT 改为：`SELECT group_id, name, org_name, user_ids, created_at, updated_at`
        - Scan 改为：`.Scan(&g.GroupID, &g.Name, &g.OrgName, &rawUserIDs, &g.CreatedAt, &g.UpdatedAt)`
