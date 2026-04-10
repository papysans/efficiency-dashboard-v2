## 实施

### 阶段 1：数据层（DB schema + Go struct + SQL）

- [x] 1.1 数据库 schema 变更：重命名字段 + 新增字段
     【目标对象】`init_db.sql`
     【修改目的】将 ai_estimated_ancient_days/reason 重命名为 task_ancient_minutes/reason，新增 task_real_minutes 系列字段和 manual 字段，为后续后端/前端修改提供数据基础
     【修改方式】在 init_db.sql 末尾新增 DO $$ BEGIN ... END $$ 块添加 ALTER TABLE 语句（使用 IF EXISTS 防止重复执行），同时修改 CREATE TABLE 语句中的字段名保持一致
     【相关依赖】无，本任务为最底层依赖
     【修改内容】
        - costrict_tasks 表：RENAME COLUMN `ai_estimated_ancient_days` -> `task_ancient_minutes`，RENAME COLUMN `ai_estimated_ancient_reason` -> `task_ancient_minutes_reason`
        - costrict_tasks 表：ADD COLUMN `task_ancient_minutes_manual` DECIMAL(10,2)，ADD COLUMN `task_ancient_minutes_reason_manual` TEXT
        - costrict_tasks 表：ADD COLUMN `task_real_minutes` DECIMAL(10,2)，ADD COLUMN `task_real_minutes_reason` TEXT
        - costrict_tasks 表：ADD COLUMN `task_real_minutes_manual` DECIMAL(10,2)，ADD COLUMN `task_real_minutes_reason_manual` TEXT
        - costrict_tasks 表：UPDATE SET task_ancient_minutes = NULL（清零旧数据，因为单位从人天变为分钟）
        - costrict_commits 表：RENAME COLUMN `ai_estimated_ancient_days` -> `task_ancient_minutes`，RENAME COLUMN `ai_estimated_ancient_reason` -> `task_ancient_minutes_reason`
        - costrict_projects 表：RENAME COLUMN `ai_estimated_ancient_days` -> `task_ancient_minutes`，RENAME COLUMN `ai_estimated_ancient_reason` -> `task_ancient_minutes_reason`，RENAME COLUMN `ai_estimated_ancient_days_manual` -> `task_ancient_minutes_manual`，RENAME COLUMN `ai_estimated_ancient_reason_manual` -> `task_ancient_minutes_reason_manual`
        - 同步修改 CREATE TABLE 语句中的字段名，使 init_db.sql 中 CREATE TABLE 与 ALTER 保持一致
        - 手动执行 SQL 完成数据库迁移

- [x] 1.2 后端 Go struct 字段重命名 + 新增字段
     【目标对象】`backend/db.go`
     【修改目的】Go struct 与数据库字段保持一致
     【修改方式】修改 CostrictTask、CostrictCommit、CostrictProject 三个结构体的字段定义
     【相关依赖】1.1 完成后
     【修改内容】
        - CostrictTask 结构体（约第154行）：将 `AIEstimatedAncientDays *float64 json:"ai_estimated_ancient_days"` 改为 `TaskAncientMinutes *float64 json:"task_ancient_minutes"`
        - CostrictTask 结构体：将 `AIEstimatedAncientReason *string json:"ai_estimated_ancient_reason"` 改为 `TaskAncientMinutesReason *string json:"task_ancient_minutes_reason"`
        - CostrictTask 结构体：新增 `TaskAncientMinutesManual *float64 json:"task_ancient_minutes_manual"`
        - CostrictTask 结构体：新增 `TaskAncientMinutesReasonManual *string json:"task_ancient_minutes_reason_manual"`
        - CostrictTask 结构体：新增 `TaskRealMinutes *float64 json:"task_real_minutes"`
        - CostrictTask 结构体：新增 `TaskRealMinutesReason *string json:"task_real_minutes_reason"`
        - CostrictTask 结构体：新增 `TaskRealMinutesManual *float64 json:"task_real_minutes_manual"`
        - CostrictTask 结构体：新增 `TaskRealMinutesReasonManual *string json:"task_real_minutes_reason_manual"`
        - CostrictCommit 结构体（约第209行）：将 AIEstimatedAncientDays/Reason 改为 TaskAncientMinutes/Reason
        - CostrictProject 结构体（约第230行）：将 AIEstimatedAncientDays -> TaskAncientMinutes，AIEstimatedAncientReason -> TaskAncientMinutesReason，AIEstimatedAncientDaysManual -> TaskAncientMinutesManual，AIEstimatedAncientReasonManual -> TaskAncientMinutesReasonManual

- [x] 1.3 后端 SQL 查询/写入语句全量更新
     【目标对象】`backend/db.go`
     【修改目的】所有涉及 ai_estimated_ancient_days/reason 的 SQL 语句和 scan 函数改为新字段名，新增字段的 SELECT/INSERT/UPDATE
     【修改方式】修改以下函数：costrictTaskSelectColumns（约第413行）、scanCostrictTask（约第419行）、UpsertCostrictTask（约第835行）、GetCostrictTask（约第867行）、ListCostrictTasks（约第884行）；costrictCommitSelectColumns（约第449行）、scanCostrictCommit（约第454行）、UpsertCostrictCommit（约第1094行）、BatchUpsertCostrictCommits（约第1231行）；costrictProjectSelectColumns（约第465行）、scanCostrictProject（约第473行）、UpsertCostrictProject（约第1292行）、UpdateCostrictProjectManual（约第1388行）
     【相关依赖】1.2 完成后
     【修改内容】
        - `costrictTaskSelectColumns` 中将 `ai_estimated_ancient_days, ai_estimated_ancient_reason` 替换为 `task_ancient_minutes, task_ancient_minutes_reason`，追加新字段列：`task_ancient_minutes_manual, task_ancient_minutes_reason_manual, task_real_minutes, task_real_minutes_reason, task_real_minutes_manual, task_real_minutes_reason_manual`
        - `scanCostrictTask` 中对应增加 Scan 参数匹配新字段
        - `UpsertCostrictTask` 的 INSERT/ON CONFLICT DO UPDATE SQL 中：将 `ai_estimated_ancient_days` -> `task_ancient_minutes`、`ai_estimated_ancient_reason` -> `task_ancient_minutes_reason`，新增6个字段的 INSERT/UPDATE
        - `GetCostrictTask`、`ListCostrictTasks` 通过 costrictTaskSelectColumns 自动生效
        - `costrictCommitSelectColumns` 中 `ai_estimated_ancient_days, ai_estimated_ancient_reason` -> `task_ancient_minutes, task_ancient_minutes_reason`
        - `scanCostrictCommit` 对应更新
        - `UpsertCostrictCommit` SQL 中所有 `ai_estimated_ancient_days/reason` -> `task_ancient_minutes/reason`
        - `BatchUpsertCostrictCommits` SQL 中同样替换所有 `ai_estimated_ancient_days/reason` -> `task_ancient_minutes/reason`
        - `costrictProjectSelectColumns` 中 4 个 ancient 字段全部替换为对应的 minutes 字段
        - `scanCostrictProject` 对应更新
        - `UpsertCostrictProject` SQL 中 4 个 ancient 字段全部替换
        - `UpdateCostrictProjectManual` SQL 中 `ai_estimated_ancient_days_manual` -> `task_ancient_minutes_manual`，`ai_estimated_ancient_reason_manual` -> `task_ancient_minutes_reason_manual`
        - 新增 `UpdateCostrictTaskManual(db *sql.DB, taskID string, realManual, realReasonManual, ancientManual, ancientReasonManual)` 函数：UPDATE costrict_tasks SET 四个 manual 字段 WHERE task_id = $1，返回 error

- [x] 1.4 后端配置新增 task_real_minutes 计算参数
     【目标对象】`backend/config.yaml` + `backend/main.go`
     【修改目的】添加 task_real_minutes 的全局可配置参数（gap 阈值和延长值）
     【修改方式】在 main.go 的 Config 结构体（约第25行）中新增嵌套结构体字段，在 loadConfig 函数（约第49行）中设置默认值；在 config.yaml 末尾新增配置段
     【相关依赖】无
     【修改内容】
        - config.yaml 新增 `task_real_minutes` 配置段，包含 `gap_threshold_minutes: 30` 和 `extension_minutes: 5`
        - main.go 的 Config struct 新增 TaskRealMinutes 嵌套结构体，包含 GapThresholdMinutes int 和 ExtensionMinutes int
        - loadConfig 中设置默认值：GapThresholdMinutes=30，ExtensionMinutes=5（在文件不存在或未配置时使用）

### 阶段 2：后端业务逻辑（计算 + API）

- [x] 2.1 后端实现 task_real_minutes 实时计算逻辑
     【目标对象】`backend/task_handler_v2.go`
     【修改目的】根据 conversation 时间序列计算任务实际耗时（分钟）
     【修改方式】新增 `TimeSegment` 结构体和 `calculateTaskRealMinutes` 函数，在 `getTaskDetailV2` 函数（约第92行）中调用
     【相关依赖】1.3、1.4 完成后
     【修改内容】
        - 新增 `TimeSegment` 结构体：`Start time.Time, End time.Time, ConvCount int`
        - 新增 `calculateTaskRealMinutes(conversations []CostrictTaskConversation, gapThreshold int, extensionMin int) (float64, string, []TimeSegment)` 函数
        - 算法逻辑：
          1. 过滤掉 start_time 为 nil 的 conversation
          2. 如果有效 conversation 为 0，返回 (0, "无有效对话", nil)
          3. 如果只有 1 条，返回 (extensionMin, reason, 单个片段)
          4. 按 start_time 升序排序
          5. 遍历排序后的时间列表，计算相邻间隔
          6. 间隔 <= gapThreshold 分钟：归入当前连续片段
          7. 间隔 > gapThreshold 分钟：结束当前片段（最后一个时间点 + extensionMinutes），开始新片段
          8. 最后一个片段的结束时间 = 最后一个时间点 + extensionMinutes
          9. 累加所有片段的时长得到 task_real_minutes
          10. 生成 reason 字符串描述各片段起止时间
        - 在 `getTaskDetailV2` 中：获取 conversations 后调用 calculateTaskRealMinutes，将结果写入 task 对象，并使用 goroutine 异步 UPDATE 到 DB（异步失败仅 log.Printf 记录错误，不影响 API 响应）
        - 返回 JSON 中增加 `time_segments` 字段供前端可视化

- [x] 2.2 后端新增 task manual 更新 API + 路由注册
     【目标对象】`backend/task_handler_v2.go` + `backend/main.go`
     【修改目的】支持前端提交 task_real_minutes_manual 和 task_ancient_minutes_manual 的修正值
     【修改方式】在 task_handler_v2.go 中新增 `updateTaskManualV2` handler 函数；在 main.go 的 v2 路由组（约第160行）中注册新路由
     【相关依赖】1.3 完成后（依赖 UpdateCostrictTaskManual 函数）
     【修改内容】
        - task_handler_v2.go 新增 `updateTaskManualV2(c *gin.Context)` handler
        - 请求体解析：`task_real_minutes_manual, task_real_minutes_reason_manual, task_ancient_minutes_manual, task_ancient_minutes_reason_manual`
        - 从 URL 参数获取 taskId：`c.Param("taskId")`
        - 校验 taskId 非空，调用 1.3 中新增的 `UpdateCostrictTaskManual` 函数
        - 错误处理：参数绑定失败返回 400，DB 操作失败返回 500，成功返回 `{"status": "ok"}`
        - main.go 中 v2 路由组新增：`v2.PUT("/tasks/:taskId/manual", updateTaskManualV2)`

- [x] 2.3 后端 API 返回增加 efficiency_ratio 计算字段
     【目标对象】`backend/task_handler_v2.go`
     【修改目的】在 task 详情 API 返回中增加提效比字段
     【修改方式】在 `getTaskDetailV2` 函数的 JSON 返回部分（约第111行 `c.JSON`）中增加计算逻辑
     【相关依赖】2.1 完成后
     【修改内容】
        - 获取 task_ancient_minutes 有效值：优先使用 task_ancient_minutes_manual，为 nil 则使用 task_ancient_minutes
        - 获取 task_real_minutes 有效值：优先使用 task_real_minutes_manual，为 nil 则使用 task_real_minutes
        - 计算 efficiency_ratio：如果 real > 0 且 ancient > 0，则 efficiency_ratio = (ancient / real) * 100；否则 efficiency_ratio 为 nil（不输出该字段或输出 null）
        - 将 efficiency_ratio 附加到返回 JSON 中

### 阶段 3：kbcli 数据导入层修改

- [x] 3.1 kbcli 数据导入修正：repo_id 生成逻辑
     【目标对象】`kbcli/raw_parser.go`
     【修改目的】确保 project_id 和 repo_id 不再使用相同的值，repo_id 在无 repo 信息时置空
     【修改方式】修改 `ParseRawJSON` 函数（约第107行）中 repo_id 的赋值逻辑
     【相关依赖】无
     【修改内容】
        - 当前逻辑：repo 变量为空字符串，projectID 在 repo 为空时使用 clientID[:10] + ":" + projectPath 格式，但 repo_id 也被设为同样的空值
        - 修改：保持 repo_id 为空字符串（当前 rawdata JSON 中不含 git repo 信息），确保 doc.RepoID 不会被错误赋值为 projectID 格式的数据
        - 验证 rawJSON 结构中是否存在 git remote/repo 信息字段作为后续扩展预留

- [x] 3.2 kbcli 字段映射更新（pg_writer + task_builder + task_content + ai_estimator）
     【目标对象】`kbcli/pg_writer.go` + `kbcli/task_builder.go` + `kbcli/task_content.go` + `kbcli/ai_estimator.go`
     【修改目的】将所有 AIEstimatedAncientDays/Reason 和 AIEstimatedDays/Reason 字段重命名为 TaskAncientMinutes/Reason，并更新 AI 估算的 prompt 和输出解析
     【修改方式】修改 PGTaskData struct（pg_writer.go 约第16行）、PGCommitData struct（pg_writer.go 约第241行）、MapTaskDocToPG 函数（pg_writer.go 约第70行）；修改 TaskDoc struct（task_builder.go 约第6行）；修改 TaskContentFile struct（task_content.go 约第24行）；修改 ai_estimator.go 中的 defaultEstimationPrompt（约第57行）和 EstimateTaskDays 的返回值解析（约第165行）
     【相关依赖】1.1 完成后
     【修改内容】
        - pg_writer.go 的 PGTaskData struct：`AIEstimatedAncientDays` -> `TaskAncientMinutes`，`AIEstimatedAncientReason` -> `TaskAncientMinutesReason`
        - pg_writer.go 的 PGCommitData struct：同样重命名
        - pg_writer.go 的 MapTaskDocToPG 函数（约第90行）：`AIEstimatedAncientDays: ptrFloat64(taskDoc.AIEstimatedDays)` -> `TaskAncientMinutes: ptrFloat64(taskDoc.TaskAncientMinutes)` 等
        - task_builder.go 的 TaskDoc struct：`AIEstimatedDays float64` -> `TaskAncientMinutes float64`，`AIEstimatedReason string` -> `TaskAncientMinutesReason string`，json tag 同步更新
        - task_content.go 的 TaskContentFile struct：`AIEstimatedDays` -> `TaskAncientMinutes`，`AIEstimatedReason` -> `TaskAncientMinutesReason`
        - ai_estimator.go 的 defaultEstimationPrompt：将 prompt 中的"人天"改为"分钟"，输出 JSON 字段名从 `ai_estimated_days/ai_estimated_reason` 改为 `task_ancient_minutes/task_ancient_minutes_reason`
        - ai_estimator.go 的 EstimateTaskDays 函数（建议重命名为 EstimateTaskMinutes）：解析 JSON 结果的 struct 字段名更新，验证范围从 0-1000 人天调整为 0-100000 分钟
        - ai_estimator.go 的 UpdateTaskContentWithEstimation 函数：字段赋值更新

### 阶段 4：后端聚合查询字段更新

- [x] 4.1 后端 dashboard 和 project 聚合查询字段更新
     【目标对象】`backend/dashboard_handler_v2.go` + `backend/project_handler_v2.go`
     【修改目的】SQL 聚合查询中的 ai_estimated_ancient_days 改为 task_ancient_minutes
     【修改方式】修改 getDashboardSummary 函数（dashboard_handler_v2.go 约第42行）、listProjectsV2 函数（project_handler_v2.go 约第63行）、getProjectDetailByQueryV2/getProjectDetailByProjectIdV2 等函数中的 SQL 以及 sumTaskAIDays 辅助函数
     【相关依赖】1.1 完成后
     【修改内容】
        - dashboard_handler_v2.go 的 getDashboardSummary 函数中：`SUM(ai_estimated_ancient_days)` -> `SUM(task_ancient_minutes)`，变量名 totalAIDays 可保留或更新
        - project_handler_v2.go 的 listProjectsV2 函数中：`SUM(ai_estimated_ancient_days) as ai_estimated_days` -> `SUM(task_ancient_minutes) as ai_estimated_days`（注意别名保持向后兼容或同步更新前端）
        - project_handler_v2.go 约第344行的 SQL：`p.ai_estimated_ancient_days` -> `p.task_ancient_minutes`
        - project_handler_v2.go 中 sumTaskAIDays 辅助函数：内部读取的字段从 AIEstimatedAncientDays 改为 TaskAncientMinutes
        - project_handler_v2.go 的 JSON 返回中 `"ai_estimated_days"` 键名考虑是否需要改为 `"task_ancient_minutes"`（需与前端同步）

### 阶段 5：前端修改

- [x] 5.1 前端时间格式化与时长工具函数
     【目标对象】`frontend/src/utils/formatters.js`
     【修改目的】提供统一的时间本地化格式化函数和分钟自适应显示函数
     【修改方式】在已有的 formatters.js 文件中新增两个导出函数
     【相关依赖】无
     【修改内容】
        - 新增 `formatLocalTime(isoStr)` 函数：将 ISO 8601/RFC3339 字符串转为本地时间格式 `YYYY-MM-DD HH:mm:ss`，输入为空或无效时返回 '-'
        - 新增 `formatDuration(minutes)` 函数：根据分钟数自适应显示，规则为：小于 60 分钟显示"X分钟"；60-480 分钟显示"X小时Y分钟"；大于 480 分钟（1人天=8小时=480分钟）显示"X.X人天"；输入为 null/undefined/0 时返回 '-'

- [x] 5.2 前端 API 调用新增 task manual 更新函数
     【目标对象】`frontend/src/api/es.js`
     【修改目的】新增调用 PUT /api/v2/tasks/:taskId/manual 的 API 函数
     【修改方式】在 es.js 文件末尾新增一个导出函数
     【相关依赖】2.2 完成后（后端路由已注册）
     【修改内容】
        - 新增 `export function updateTaskManualV2(taskId, data)` 函数
        - 调用 `request({ url: '/v2/tasks/${taskId}/manual', method: 'put', data })`
        - 风格与已有的 `updateProjectManualV2` 函数保持一致

- [x] 5.3 前端 Task 详情页字段修正与增强
     【目标对象】`frontend/src/views/TaskDetailV2.vue`
     【修改目的】修复前后端字段名不匹配、添加新指标展示、时间本地化、仓库/Project 分离、人工修正入口
     【修改方式】修改 template 中的数据绑定和 script setup 中的 import/计算逻辑
     【相关依赖】5.1、5.2、2.1（后端返回 time_segments）、2.3（后端返回 efficiency_ratio）完成后
     【修改内容】
        - import 新增：从 `@/utils/formatters` 导入 `formatLocalTime` 和 `formatDuration`；从 `@/api/es` 导入 `updateTaskManualV2`
        - 修复字段名 Bug（template 部分）：
          - 第22行 `task.total_cost` -> `task.cost`（后端 CostrictTask 的 JSON tag 是 "cost"）
          - 第24行 `task.ai_estimated_days` -> `task.task_ancient_minutes`，显示改用 `formatDuration(task.task_ancient_minutes)`
          - 第60行 `conv.model_name` -> `conv.model`（后端 CostrictTaskConversation 的 JSON tag 是 "model"）
          - 第84行 `conv.total_tokens` -> `(conv.upstream_tokens || 0) + (conv.downstream_tokens || 0)`（后端没有 total_tokens 字段）
        - script 中 totalTokens computed（第118行）：`c.total_tokens` -> `(c.upstream_tokens || 0) + (c.downstream_tokens || 0)`
        - 时间本地化（template 部分）：
          - 第20行 task.start_time 用 `formatLocalTime(task.start_time)` 格式化
          - 第21行 task.end_time 用 `formatLocalTime(task.end_time)` 格式化
          - 第54行 el-timeline-item 的 :timestamp 用 `formatLocalTime(conv.start_time)` 格式化
        - 元信息区域字段调整（template 中 el-descriptions 部分）：
          - 「仓库」字段改为显示 `task.repo_addr + '#' + task.repo_branch`（为空时显示 task.repo_id），链接跳转到 `/repo/` + encodeURIComponent(task.repo_id)
          - 新增「Project」字段，显示 task.project_id
          - 「AI预估人天」改为「古法预估」，显示 formatDuration(task.task_ancient_minutes)
          - 新增「实际耗时」，显示 formatDuration(task.task_real_minutes)
          - 新增「提效比」，显示 efficiency_ratio（从 API 返回的 efficiency_ratio 字段，百分比格式如 "235.5%"）
        - 新增「编辑」按钮和 el-dialog 人工修正对话框（参考 ProjectDetailV2.vue 的 manualForm 做法）：
          - 编辑 task_real_minutes_manual、task_real_minutes_reason_manual、task_ancient_minutes_manual、task_ancient_minutes_reason_manual
          - 提交时调用 updateTaskManualV2(taskId, data)
          - 提交成功后刷新页面数据

- [x] 5.4 前端对话历史时间片段可视化
     【目标对象】`frontend/src/views/TaskDetailV2.vue`
     【修改目的】在对话历史 el-timeline 中可视化连续/断开的时间片段
     【修改方式】修改 template 中 el-timeline 部分和 script setup 中新增计算逻辑
     【相关依赖】2.1（后端返回 time_segments）、5.1
     【修改内容】
        - 在 script setup 中新增 computed 函数，利用后端返回的 `time_segments` 数据为每个 conversation 标记所属片段索引
        - el-timeline-item 动态设置 :color 属性：
          - 同一连续片段内的对话：绿色节点（`#67C23A`）
          - 断开后的第一个对话（即新片段的首个对话）：橙色节点（`#E6A23C`）
        - 在断开处（两个片段之间）插入一个特殊的 el-timeline-item，显示「间隔 XX 分钟（不计入耗时）」，用灰色节点
        - 在对话历史 card 的 header 旁显示耗时汇总：「实际耗时：formatDuration(task.task_real_minutes)」

- [x] 5.5 前端其他页面字段名同步更新
     【目标对象】`frontend/src/views/TaskViewV2.vue` + `frontend/src/views/ProjectDetailV2.vue` + `frontend/src/views/ProjectViewV2.vue` + `frontend/src/views/Home.vue`
     【修改目的】所有引用旧字段名 ai_estimated_ancient_days 的页面同步更新为 task_ancient_minutes
     【修改方式】修改各文件中的 template 数据绑定和 script 中的字段引用
     【相关依赖】5.1（依赖 formatDuration）、4.1（后端返回字段名已更新）
     【修改内容】
        - TaskViewV2.vue 第28行：`prop="ai_estimated_ancient_days"` -> `prop="task_ancient_minutes"`，label 从"AI预估人天"改为"古法预估"，添加 :formatter 使用 formatDuration
        - ProjectDetailV2.vue 的 manualForm 相关（约第100、103、196、197、204、205、218、219行）：所有 `ai_estimated_ancient_days_manual` -> `task_ancient_minutes_manual`，`ai_estimated_ancient_reason_manual` -> `task_ancient_minutes_reason_manual`
        - ProjectViewV2.vue 第27行：`prop="ai_estimated_days"` 以及 label 更新（取决于后端 4.1 的 JSON 别名是否变更）
        - Home.vue 第99行：`summary.total_ai_estimated_days` 的显示（取决于后端 4.1 的 JSON 别名是否变更）

### 阶段 6：文档

- [x] 6.1 输出设计文档v2.md
     【目标对象】`设计文档v2.md`（新建）
     【修改目的】根据本次变更输出更新后的 task_summary.json 数据结构定义
     【修改方式】新建 markdown 文件
     【相关依赖】所有前述任务完成后
     【修改内容】
        - 基于现有设计文档（如有），更新 task_summary.json 部分
        - 体现所有新增/重命名的字段：task_ancient_minutes/reason（含 _manual）、task_real_minutes/reason（含 _manual）
        - 体现 efficiency_ratio 的计算逻辑和优先级规则（_manual 优先）
        - 体现 task_real_minutes 的算法说明（gap_threshold、extension、时间片段）
        - 体现 time_segments 的数据结构
