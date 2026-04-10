## 实施

### 阶段 1：全局 ID path-safe 格式化

- [x] 1.1 新增 path-safe ID 转换工具函数
     【目标对象】`kbcli/raw_parser.go`（或新建 `kbcli/id_utils.go`）+ `backend/db.go`（或新建 `backend/id_utils.go`）
     【修改目的】提供统一的 path-safe ID 转换函数，供 kbcli 数据导入和后端数据迁移使用
     【修改方式】新增 `toPathSafeID(raw string) string` 函数
     【相关依赖】无（基础工具函数，被 1.2、1.3 依赖）
     【修改内容】
        - 算法：输入字符串 → 转小写 → 非(小写字母/数字/-/.)字符替换为 `-` → 合并连续 `-` → 去除首尾 `-`
        - 空字符串输入直接返回空字符串
        - 示例：`https://github.com/zgsm-ai/costrict.git#main` → `https-github.com-zgsm-ai-costrict.git-main`
        - 示例：`797e102c29:d:\projects\creditControl\credit-sentinel` → `797e102c29-d-projects-creditcontrol-credit-sentinel`
        - 在 kbcli 和 backend 两处都需要此函数（kbcli 用于新数据导入，backend 用于数据迁移和展示）

- [x] 1.2 kbcli 数据导入中 repo_id/project_id 改用 path-safe 格式
     【目标对象】`kbcli/raw_parser.go` + `kbcli/cmd_analyze.go`
     【修改目的】新导入的数据使用 path-safe 格式的 repo_id 和 project_id
     【修改方式】修改 `raw_parser.go` 的 ParseRawJSON 函数、修改 `cmd_analyze.go` 中 repoID 传递给 MapCommitDetailsToPG 的调用处
     【相关依赖】1.1
     【修改内容】
        - `raw_parser.go` ParseRawJSON 函数中 repo_id 生成处（约第139行）：如果值非空，调用 toPathSafeID 转换
        - `raw_parser.go` ParseRawJSON 函数中 project_id 生成处（约第144行）：`clientID[:10] + ":" + projectPath` 改为 `toPathSafeID(clientID[:10] + ":" + projectPath)`
        - `cmd_analyze.go` 第318行 `MapCommitDetailsToPG(gitResult.Commits, repoID, orgProvider)` 调用处：repoID 传入前先调用 `toPathSafeID(repoID)` 转换（注意：`pg_writer.go` 的 MapCommitDetailsToPG 函数内部直接使用传入的 repoID 参数，不需要在该函数内部再调用转换）

- [x] 1.3 数据库历史数据 repo_id/project_id 迁移
     【目标对象】`init_db.sql`
     【修改目的】将 costrict_tasks、costrict_commits、costrict_projects 中已有的 repo_id/project_id 转为 path-safe 格式
     【修改方式】在 init_db.sql 末尾追加 DO $$ BEGIN ... END $$ 迁移块，使用 PostgreSQL 的正则替换函数
     【相关依赖】1.1
     【修改内容】
        - 编写 PostgreSQL 函数或使用 regexp_replace 链式调用实现 path-safe 转换：先 lower()，再 regexp_replace 非法字符为 '-'，再 regexp_replace 连续 '-' 为单个 '-'，再 trim(both '-' from ...)
        - UPDATE costrict_tasks SET repo_id = path_safe(repo_id), project_id = path_safe(project_id) WHERE repo_id IS NOT NULL
        - UPDATE costrict_commits SET repo_id = path_safe(repo_id) WHERE repo_id IS NOT NULL
        - UPDATE costrict_projects SET repo_id = path_safe(repo_id) WHERE repo_id IS NOT NULL
        - 迁移语句需幂等（重复执行不报错，已转换的数据不会二次变化）
        - 手动执行 SQL 迁移并验证

### 阶段 2：costrict_commits 表字段变更

- [x] 2.1 数据库 schema：commits 表字段重命名+新增
     【目标对象】`init_db.sql`
     【修改目的】commit 表字段改为 commit_ 前缀以语义化，新增 manual 字段和 task 关联字段
     【修改方式】在 init_db.sql 的迁移块中新增 ALTER TABLE 语句，同步修改 CREATE TABLE 定义
     【相关依赖】1.3（应在 path-safe 迁移之后执行）
     【修改内容】
        - RENAME COLUMN `task_ancient_minutes` → `commit_ancient_minutes`（需 IF EXISTS 判断幂等）
        - RENAME COLUMN `task_ancient_minutes_reason` → `commit_ancient_minutes_reason`
        - ADD COLUMN `commit_ancient_minutes_manual` DECIMAL(10,2)
        - ADD COLUMN `commit_ancient_minutes_reason_manual` TEXT
        - ADD COLUMN `task_ids` JSONB — 关联的 task_id 数组
        - ADD COLUMN `task_ids_silica` JSONB — 对应的硅比例数组
        - ADD COLUMN `commit_real_minutes` DECIMAL(10,2) — 实际耗时(分钟)
        - ADD COLUMN `commit_real_minutes_reason` TEXT — 计算逻辑说明(最大200字符)
        - ADD COLUMN `commit_real_minutes_manual` DECIMAL(10,2)
        - ADD COLUMN `commit_real_minutes_reason_manual` TEXT
        - 同步修改 CREATE TABLE 中 costrict_commits 的列定义（将 task_ancient_minutes/reason 改为 commit_ancient_minutes/reason，并新增上述列）
        - 执行 SQL 迁移

- [x] 2.2 后端 Go struct 和 SQL 全量更新
     【目标对象】`backend/db.go`
     【修改目的】CostrictCommit struct 与数据库字段保持一致
     【修改方式】修改 CostrictCommit struct（约第214-233行）、costrictCommitSelectColumns（约第461行）、scanCostrictCommit（约第466行）、UpsertCostrictCommit（约第1119行）、BatchUpsertCostrictCommits（约第1255行），新增 UpdateCostrictCommitManual 和 UpdateCostrictCommitTaskAssoc 函数
     【相关依赖】2.1
     【修改内容】
        - CostrictCommit struct（约第214行）：
          - `TaskAncientMinutes *float64 json:"task_ancient_minutes"` → `CommitAncientMinutes *float64 json:"commit_ancient_minutes"`
          - `TaskAncientMinutesReason *string json:"task_ancient_minutes_reason"` → `CommitAncientMinutesReason *string json:"commit_ancient_minutes_reason"`
          - 新增字段：CommitAncientMinutesManual(*float64)、CommitAncientMinutesReasonManual(*string)、TaskIDs(json.RawMessage)、TaskIDsSilica(json.RawMessage)、CommitRealMinutes(*float64)、CommitRealMinutesReason(*string)、CommitRealMinutesManual(*float64)、CommitRealMinutesReasonManual(*string)
        - costrictCommitSelectColumns（约第461行）：将 `task_ancient_minutes, task_ancient_minutes_reason` 改为 `commit_ancient_minutes, commit_ancient_minutes_reason`，并追加新增字段列
        - scanCostrictCommit（约第466行）：增加新字段的 Scan 参数，TaskIDs/TaskIDsSilica 需 jsonb 扫描处理（参考 scanCostrictProject 的 JSONB 处理方式）
        - UpsertCostrictCommit（约第1119行）：SQL 中 `task_ancient_minutes` → `commit_ancient_minutes`，`task_ancient_minutes_reason` → `commit_ancient_minutes_reason`；注意 task_ids/silica 和 manual 字段不在 upsert 中更新
        - BatchUpsertCostrictCommits（约第1255行）：同上 SQL 字段名更新
        - 新增 `UpdateCostrictCommitManual(db, commitID, repoID, manual_fields...)` 函数：UPDATE commit_ancient_minutes_manual/reason + commit_real_minutes_manual/reason WHERE commit_id=$1 AND repo_id=$2
        - 新增 `UpdateCostrictCommitTaskAssoc(db, commitID, repoID, taskIDs, taskIDsSilica, realMinutes, realMinutesReason)` 函数：UPDATE task_ids/task_ids_silica/commit_real_minutes/commit_real_minutes_reason

### 阶段 3：后端 commit API 增强

- [x] 3.1 commit 详情 API 增强
     【目标对象】`backend/commit_handler_v2.go`
     【修改目的】详情返回关联 task 信息、计算提效比
     【修改方式】修改 getCommitDetailV2 函数（约第91行）
     【相关依赖】2.2
     【修改内容】
        - 读取 commit 后，如果 commit.TaskIDs 有数据（非 nil 且非空数组），解析 JSON 数组获取 task_id 列表
        - 查询关联 task 简要信息：SELECT task_id, user_name, start_time, task_real_minutes FROM costrict_tasks WHERE task_id = ANY($1)
        - 计算 efficiency_ratio：
          - effective_ancient = commit.CommitAncientMinutesManual ?? commit.CommitAncientMinutes
          - effective_real = commit.CommitRealMinutesManual ?? commit.CommitRealMinutes
          - 两者都有值时：efficiency_ratio = effective_ancient / effective_real * 100，保留1位小数
          - 任一为 nil 时：efficiency_ratio = nil
        - 返回 JSON 从 `gin.H{"commit": commit}` 改为 `gin.H{"commit": commit, "related_tasks": relatedTasks, "efficiency_ratio": ratio}`

- [x] 3.2 新增 commit manual 更新 API
     【目标对象】`backend/commit_handler_v2.go` + `backend/main.go`
     【修改目的】支持前端提交人工修正值
     【修改方式】在 commit_handler_v2.go 中新增 updateCommitManualV2 handler 函数；在 main.go 的 v2 路由组（约第176行之后）新增路由注册
     【相关依赖】2.2
     【修改内容】
        - handler 函数：updateCommitManualV2
        - 路由：PUT `/api/v2/commits/:commitId/manual`
        - 请求体 struct：commit_ancient_minutes_manual(*float64)、commit_ancient_minutes_reason_manual(*string)、commit_real_minutes_manual(*float64)、commit_real_minutes_reason_manual(*string)
        - 需要 commitId(path param) + repoId(query param) 两个参数定位 commit
        - 参数校验：repoId 不能为空
        - 调用 UpdateCostrictCommitManual 写入数据库
        - main.go 新增：`v2.PUT("/commits/:commitId/manual", updateCommitManualV2)`（放在现有 `v2.GET("/commits/:commitId", getCommitDetailV2)` 之后）

### 阶段 4：kbcli commit 字段映射更新

- [x] 4.1 kbcli commit 估时字段全量重命名（task_ancient_minutes → commit_ancient_minutes）
     【目标对象】`kbcli/pg_writer.go` + `kbcli/git_analyzer.go` + `kbcli/cmd_analyze.go` + `kbcli/db_writer.go` + `kbcli/es_mappings.go`
     【修改目的】commit 维度的估时字段从 task_ 前缀全部改为 commit_ 前缀，保持与数据库 schema 一致
     【修改方式】修改以下文件中的 struct 字段名、JSON tag 和变量引用
     【相关依赖】2.1
     【修改内容】
        - `pg_writer.go` PGCommitData struct（约第254-255行）：
          - `TaskAncientMinutes *float64` → `CommitAncientMinutes *float64`
          - `TaskAncientMinutesReason *string` → `CommitAncientMinutesReason *string`
          - JSON tag 也同步更新
        - `pg_writer.go` MapCommitDetailsToPG 函数（约第259行）：字段赋值从 TaskAncientMinutes 改为 CommitAncientMinutes（如果有赋值处）
        - `git_analyzer.go` EstimateFromGit 函数中的 AI prompt 模板（约第258-261行）：
          - JSON 输出示例从 `"task_ancient_minutes": 270, "task_ancient_minutes_reason": "..."` 改为 `"commit_ancient_minutes": 270, "commit_ancient_minutes_reason": "..."`
        - `git_analyzer.go` 内部 aiResult struct（约第352-353行）：
          - `TaskAncientMinutes float64 json:"task_ancient_minutes"` → `CommitAncientMinutes float64 json:"commit_ancient_minutes"`
          - `TaskAncientMinutesReason string json:"task_ancient_minutes_reason"` → `CommitAncientMinutesReason string json:"commit_ancient_minutes_reason"`
          - 后续引用处（约第359-363行）：`aiResult.TaskAncientMinutes` → `aiResult.CommitAncientMinutes`，同理 Reason
        - `cmd_analyze.go`（约第272-273行）：
          - `output["task_ancient_minutes"] = minutes` → `output["commit_ancient_minutes"] = minutes`
          - `output["task_ancient_minutes_reason"] = reason` → `output["commit_ancient_minutes_reason"] = reason`
        - `cmd_analyze.go`（约第305行）：
          - `output["task_ancient_minutes"].(float64)` → `output["commit_ancient_minutes"].(float64)`
        - `db_writer.go`（约第43行）：
          - `body["task_ancient_minutes_from_git"] = *aiEstDays` → `body["commit_ancient_minutes_from_git"] = *aiEstDays`
          - 注意：后端 `/api/analysis/git/analyze` 接口也需要同步接受新字段名（检查 `backend/git_handler.go` 中是否引用此字段名）
        - `es_mappings.go` commit ES mapping（约第79-80行）：
          - `"task_ancient_minutes": { "type": "float" }` → `"commit_ancient_minutes": { "type": "float" }`
          - `"task_ancient_minutes_reason": { "type": "text" }` → `"commit_ancient_minutes_reason": { "type": "text" }`

### 阶段 5：前端页面

- [x] 5.1 前端 API 新增 commit manual 函数
     【目标对象】`frontend/src/api/es.js`
     【修改目的】新增 commit manual 更新 API 调用函数
     【修改方式】在 es.js 末尾（约第173行后）新增 export 函数
     【相关依赖】3.2
     【修改内容】
        - 新增 `export function updateCommitManualV2(commitId, repoId, data)` → `request({ url: \`/v2/commits/${commitId}/manual\`, method: 'put', data, params: { repoId } })`
        - 注意：`getCommitsV2` 和 `getCommitDetailV2` 已存在（第115-121行），无需新增

- [x] 5.2 新增 Commit 列表页 CommitViewV2.vue
     【目标对象】`frontend/src/views/CommitViewV2.vue`（新建）
     【修改目的】参考 TaskViewV2.vue 创建 commit 列表页
     【修改方式】新建 Vue 文件，复用 TaskViewV2.vue 的整体结构（FilterBar + el-table + 分页 + 图表）
     【相关依赖】5.1
     【修改内容】
        - FilterBar：日期范围 + 搜索框（placeholder="搜索 Commit ID/用户名"）+ 查询按钮
        - el-table 表格列：
          - commit_id（截短前8位，show-overflow-tooltip）
          - 用户（user_name）
          - 仓库（repo_id，show-overflow-tooltip）
          - 提交时间（commit_time，使用 formatLocalTime）
          - Diff行数（diff_lines，align="right"，sortable）
          - 古法预估（commit_ancient_minutes，使用 formatDuration，align="right"，sortable）
          - 实际耗时（commit_real_minutes，使用 formatDuration，align="right"，sortable）
        - 分页：el-pagination，与 TaskViewV2.vue 一致的配置（page-sizes=[20,50,100,200]）
        - 行点击跳转：`/commit/${row.commit_id}?repoId=${encodeURIComponent(row.repo_id)}`
        - 数据加载：调用 `getCommitsV2({ startDate, endDate, page, pageSize })`（已存在的 API）
        - 前端搜索过滤：基于 searchKeyword 对 commit_id/user_name 过滤，输入值需 .trim()
        - 复用已有的 `formatLocalTime`、`formatDuration` 等工具函数（从 composables 或 utils 导入）

- [x] 5.3 新增 Commit 详情页 CommitDetailV2.vue
     【目标对象】`frontend/src/views/CommitDetailV2.vue`（新建）
     【修改目的】参考 TaskDetailV2.vue 创建 commit 详情页
     【修改方式】新建 Vue 文件，复用 TaskDetailV2.vue 的整体结构（标题栏 + 元信息卡片 + 人工调整 dialog）
     【相关依赖】5.1、3.1
     【修改内容】
        - 路由参数获取：commitId 从 route.params 获取，repoId 从 route.query 获取
        - 标题栏 el-card：返回按钮（router.back()）+ "Commit 详情" 标题 + 人工调整按钮（el-button type="warning"）
        - 元信息卡片 el-descriptions（:column="3" border）：
          - commit_id、用户(user_name)、Git用户(git_user_name + git_user_email)
          - 仓库(repo_id，el-link 可跳转 /repo/:repoId)、分支(repo_branch)、提交时间(formatLocalTime(commit_time))
          - Diff行数(diff_lines)、古法预估(formatDuration(commit_ancient_minutes))、实际耗时(formatDuration(commit_real_minutes))、提效比(efficiency_ratio + '%')
        - 关联 Tasks 区域：
          - 条件渲染：v-if="relatedTasks && relatedTasks.length > 0"
          - el-table 展示：task_id（el-link 可跳转 /task/:taskId）、user_name、start_time、task_real_minutes(formatDuration)
        - 人工调整 el-dialog：
          - 表单字段：commit_ancient_minutes_manual(el-input-number)、commit_ancient_minutes_reason_manual(el-input textarea)、commit_real_minutes_manual(el-input-number)、commit_real_minutes_reason_manual(el-input textarea)
          - 打开时预填：优先取 _manual 值，fallback 到原始值
          - 提交调用 updateCommitManualV2(commitId, repoId, manualForm)，成功后刷新数据
        - loadData：调用 getCommitDetailV2(commitId, repoId) → 从响应中提取 commit、related_tasks、efficiency_ratio

- [x] 5.4 路由和菜单注册
     【目标对象】`frontend/src/router/index.js` + `frontend/src/App.vue`
     【修改目的】新增 commit 路由和导航菜单项
     【修改方式】在 router/index.js 的 routes 数组中新增两条路由；在 App.vue 的 el-menu 中新增菜单项
     【相关依赖】5.2、5.3
     【修改内容】
        - router/index.js 的 routes 数组（约第12行 task 路由之后）新增：
          - `{ path: '/commit-v2', name: 'CommitV2', component: () => import('@/views/CommitViewV2.vue') }`
          - `{ path: '/commit/:commitId', name: 'CommitDetail', component: () => import('@/views/CommitDetailV2.vue') }`
        - App.vue el-menu（约第19行"任务"之后）新增：
          - `<el-menu-item index="/commit-v2">提交</el-menu-item>`

- [x] 5.5 编译验证
     【目标对象】`backend/` + `frontend/`
     【修改目的】确保所有字段重命名后编译通过，无遗漏引用
     【修改方式】执行编译命令
     【相关依赖】2.2、4.1、5.4
     【修改内容】
        - 后端编译：在 backend/ 目录执行 `go build ./...`，确保无编译错误
        - 前端编译：在 frontend/ 目录执行 `npm run build`，确保无编译错误
        - 注意：costrict_tasks 表和 costrict_projects 表中的 `task_ancient_minutes` 字段不需要改名（它们本身就是 task 维度的字段），只有 costrict_commits 表中的 `task_ancient_minutes` 需要改为 `commit_ancient_minutes`
        - 注意：`dashboard_handler_v2.go` 第42行的 `SUM(task_ancient_minutes)` 查的是 costrict_tasks 表，不需要修改
        - 注意：`project_handler_v2.go` 第63行/第344行的 `task_ancient_minutes` 分别查的是 costrict_tasks 和 costrict_projects 表，不需要修改

### 阶段 6：文档

- [ ] 6.1 更新设计文档v2.md#commit数据
     【目标对象】`设计文档v2.md`
     【修改目的】更新 commit 数据章节，包含所有新增/重命名字段的含义和计算方法
     【修改方式】修改 `设计文档v2.md` 中 commit 数据相关章节
     【相关依赖】2.1、3.1、3.2
     【修改内容】
        - 更新 costrict_commits 表字段定义（含注释说明每个字段的含义和计算方法）
        - 说明 commit_ancient_minutes 的 AI 预估逻辑（基于 Git 统计 + commit 消息调用 AI 估时）
        - 说明 commit_real_minutes 的计算方法：sum(task_real_minutes[i] * task_ids_silica[i])
        - 说明 task_ids 关联条件
        - 说明 _manual 字段的用途和优先级（manual 值优先于自动计算值）
        - 说明 repo_id/project_id 的 path-safe 格式转换规则及示例

### 审查修复

- [x] add-commit-v2-page | task: 3.1-fix-1 efficiency_ratio 保留1位小数
     【目标对象】`backend/commit_handler_v2.go`
     【修改目的】task.md 3.1 要求 efficiency_ratio 保留1位小数，当前未做四舍五入处理
     【修改方式】在 getCommitDetailV2 函数中，efficiency_ratio 计算后使用 math.Round(ratio*10)/10 保留1位小数；import 中添加 "math"
     【修改内容】
        - 第4行 import 中添加 "math"
        - 第164行 `ratio := (*effectiveAncient / *effectiveReal) * 100` 之后添加 `ratio = math.Round(ratio*10) / 10`
