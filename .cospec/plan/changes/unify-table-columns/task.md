## 实施

- [x] 1.1 新增 BatchGetStatTasks 批量查询函数
     【目标对象】`backend/db.go`
     【修改目的】支持通过多个 task_id 一次性批量查询 StatTask，避免 N+1 查询
     【修改方式】在 `GetStatTask` 函数（第800行）附近新增 `BatchGetStatTasks` 函数
     【相关依赖】复用现有的 `scanStatTask` 函数和 `statTaskSelectColumns` 变量
     【修改内容】
        - 新增函数签名：`BatchGetStatTasks(db *sql.DB, taskIDs []string) (map[string]*StatTask, error)`
        - 接收 taskIDs 切片，返回 map[taskID]*StatTask
        - 边界处理：taskIDs 为空时直接返回空 map，不执行查询
        - 使用 `WHERE task_id IN ($1, $2, ...)` 动态构建占位符（$1, $2... 风格，与仓库其他查询一致）
        - 复用 `statTaskSelectColumns` 构建 SELECT 语句，用 `scanStatTask` 逐行扫描
        - 将结果按 task_id 放入 map 并返回
        - 查询失败时返回 `fmt.Errorf("批量查询 stat tasks 失败: %w", err)`，与 `GetStatTask` 错误风格一致

- [x] 1.2 listCommitsV2 响应中增加聚合的 cost/tokens 字段
     【目标对象】`backend/commit_handler_v2.go` 的 `listCommitsV2` 函数（第46-141行）
     【修改目的】为前端 Commit 表格提供费用和 Tokens 消耗数据
     【修改方式】在构建 results 循环之前，批量查询所有关联 tasks 并在循环中聚合
     【相关依赖】`backend/db.go` 的 `BatchGetStatTasks`；参考 `getCommitDetailV2` 第183-221行的聚合逻辑
     【修改内容】
        - 在第86行 results 构建之前，遍历 list 中所有 commit，解析 task_ids（json.Unmarshal），收集全部 taskID 到一个去重集合
        - 调用 `BatchGetStatTasks` 一次性批量查询所有关联的 StatTask
        - 在第88行 for 循环内，对每个 commit 解析 task_ids 和 task_ids_silica
        - 参考 `getCommitDetailV2` 的聚合逻辑：cost 直接累加（不乘 silica），tokens 乘以 silica 权重
          - cost: `sum(task.Cost)`（无 silica 加权，与 getCommitDetailV2 第201-203行一致）
          - upstream_tokens: `sum(int64(round(task.UpstreamTokens * silica)))`
          - downstream_tokens: `sum(int64(round(task.DownstreamTokens * silica)))`
        - 在 gin.H item（第89-116行）中添加 `"cost"`、`"upstream_tokens"`、`"downstream_tokens"` 三个字段
        - 解析 task_ids/task_ids_silica 失败时 log.Printf 记录并跳过（不中断请求），与现有错误处理风格一致

- [x] 1.3 listTasksV2 响应中补充 title 字段
     【目标对象】`backend/task_handler_v2.go` 的 `listTasksV2` 函数（第176-208行 gin.H 构建处）
     【修改目的】前端 Task 表格需要展示"说明"列，数据来自 task.Title
     【修改方式】在 gin.H map 构建中添加 title 字段
     【相关依赖】`StatTask` 结构体已有 `Title *string` 字段（db.go 第669行）
     【修改内容】
        - 在第178行 `"task_id": t.TaskID,` 后面添加一行：`"title": t.Title,`
        - 无额外边界处理需要，Title 为 *string 类型，nil 会序列化为 JSON null

- [x] 1.4 getRepoDetailV2 为 commits 附加聚合 cost/tokens
     【目标对象】`backend/repo_handler_v2.go` 的 `getRepoDetailV2` 函数（第80-205行）
     【修改目的】仓库详情页的 Commit 表格需要展示费用和 Tokens 数据
     【修改方式】在步骤5返回结果前，将 commits 从直接序列化 `[]StatCommit` 改为构建 `[]gin.H` 列表，附加聚合字段
     【相关依赖】已有的 `tasks` 变量（步骤2第126-133行获取的 `[]StatTask` 切片），复用任务 1.2 中相同的聚合逻辑
     【修改内容】
        - 在步骤2之后，用 tasks 切片构建 `taskMap := map[string]*StatTask`，遍历 tasks 以 TaskID 为 key
        - 在步骤5返回之前，遍历 commits，对每个 commit 解析 task_ids/task_ids_silica（复用 1.2 的逻辑）
        - 按权重聚合：cost 直接累加，tokens 乘以 silica（与任务 1.2 一致）
        - 构建 `commitItems []gin.H`，每项包含原始 commit 的全部字段（通过 JSON 序列化/反序列化或手动构建 gin.H）加上 `"cost"`、`"upstream_tokens"`、`"downstream_tokens"`
        - 将步骤5响应中的 `"commits": commits` 改为 `"commits": commitItems`
        - 解析 task_ids/task_ids_silica 失败时 log.Printf 记录并跳过（不中断请求）

- [x] 1.5 CommitViewV2.vue 表格列改为统一10列
     【目标对象】`frontend/src/views/CommitViewV2.vue` 的 columns 数组（第43-121行）及 template 部分（第1-28行）
     【修改目的】统一 Commit 表格列结构为10列，新增 ID/费用/Tokens 列，去掉"仓库"列
     【修改方式】替换 columns 数组定义，并在 template 中新增 commit_id 的插槽
     【相关依赖】`@/utils/formatters` 的 `fmtCost`、`formatDuration`；`@/utils/commit-helpers` 的 `getEffectiveAncient`、`getEffectiveReal`
     【修改内容】
        - 在 import 语句中添加 `fmtCost`（从 `@/utils/formatters` 导入，当前只导入了 `formatDuration`）
        - 将 columns 替换为以下10列（按顺序）：
          1. `commit_id` - "Commit ID"，minWidth:100，showOverflowTooltip，slotName: 'commit_id'
          2. `commit_time` - "时间"，minWidth:150，formatter 截取前16位，filter: date+serverSide
          3. `user_name` - "用户"，minWidth:90，filter: search-select
          4. `comment` - "说明"，minWidth:200，showOverflowTooltip，filter: text
          5. `diff_lines` - "代码行数"，minWidth:90，align:right，filter: number
          6. `commit_real_minutes` - "实际耗时"，minWidth:100，align:right，formatter 用 formatDuration+getEffectiveReal，filter: number（valueGetter: getEffectiveReal）
          7. `commit_ancient_minutes` - "传统开发时长预估"，minWidth:140，align:right，formatter 用 formatDuration+getEffectiveAncient，filter: number（valueGetter: getEffectiveAncient）
          8. `efficiency_ratio` - "提效比"，minWidth:90，align:center，slotName: 'efficiency_ratio'，filter: number
          9. `cost` - "费用"，minWidth:80，align:right，formatter: fmtCost，filter: number
          10. `_tokens` - "Tokens消耗"，minWidth:110，align:right，formatter 计算 upstream_tokens+downstream_tokens 并显示（为0时显示'-'），filter: number（valueGetter 取合计值）
        - 去掉原来的 `repo_addr`（"仓库"）列
        - 在 template 中 `<KbFilterTable>` 内新增 `#cell-commit_id` 插槽，显示前8位短 hash 的 el-link（点击跳转到 /commit/:id），参考 RepoDetailV2.vue 第72-74行的写法

- [x] 1.6 TaskViewV2.vue 表格列改为统一10列
     【目标对象】`frontend/src/views/TaskViewV2.vue` 的 columns 数组（第70-181行）及 template 部分（第1-39行）
     【修改目的】统一 Task 表格列结构为10列，新增 ID/说明列，合并 Tokens 列
     【修改方式】替换 columns 数组定义，并在 template 中新增 task_id 的插槽
     【相关依赖】`@/utils/formatters` 的 `fmtCost`、`formatDuration`（已导入）；本文件已有 `fmtRealMinutes`、`fmtAncientMinutes`、`getEffectiveReal`、`getEffectiveAncient` 辅助函数
     【修改内容】
        - 将 columns 替换为以下10列（按顺序）：
          1. `task_id` - "Task ID"，minWidth:100，showOverflowTooltip，slotName: 'task_id'
          2. `start_time` - "时间"，minWidth:150，formatter 截取前16位，filter: date+serverSide
          3. `user_name` - "用户"，minWidth:90，filter: search-select
          4. `title` - "说明"，minWidth:200，showOverflowTooltip，filter: text
          5. `diff_lines` - "代码行数"，minWidth:90，align:right，filter: number
          6. `task_real_minutes` - "实际耗时"，minWidth:100，align:right，formatter 用 fmtRealMinutes，filter: number（valueGetter: getEffectiveReal）
          7. `task_ancient_minutes` - "传统开发时长预估"，minWidth:140，align:right，formatter 用 fmtAncientMinutes，filter: number（valueGetter: getEffectiveAncient）
          8. `efficiency_ratio` - "提效比"，minWidth:90，align:center，slotName: 'efficiency_ratio'，filter: number
          9. `cost` - "费用"，minWidth:80，align:right，formatter: fmtCost，filter: number
          10. `_tokens` - "Tokens消耗"，minWidth:110，align:right，formatter 计算 upstream_tokens+downstream_tokens 并显示（为0时显示'-'），filter: number（valueGetter 取合计值）
        - 去掉原来的 `work_dir`（"工作目录"）列和 `caller`（"模式"）列
        - 去掉原来的 `upstream_tokens`（"上行Tokens"）和 `downstream_tokens`（"下行Tokens"）两列，合并为一列 `_tokens`
        - 在 template 中 `<KbFilterTable>` 内新增 `#cell-task_id` 插槽，显示前8位短 hash 的 el-link（点击跳转到 /task/:id），参考 CommitViewV2 的 commit_id 插槽写法

- [x] 1.7 RepoDetailV2.vue Commits/Tasks 表格统一为10列并对齐
     【目标对象】`frontend/src/views/RepoDetailV2.vue` 的 Commits el-table（第70-96行）和 Tasks el-table（第102-131行）
     【修改目的】统一 Commit/Task 表格列结构为10列，两表列宽完全一致以实现上下对齐
     【修改方式】重写两个 el-table 内的 el-table-column 定义，新增费用和 Tokens 列
     【相关依赖】`@/utils/formatters` 的 `fmtCost`（当前未导入，需新增导入）；已导入的 `formatDuration`、`formatLocalTime`
     【修改内容】
        - 在 import 语句（第141行）中从 `@/utils/formatters` 补充导入 `fmtCost`
        - Commits 表格（替换第70-96行的 el-table-column，10列）：
          1. Commit ID - minWidth:100，已有（保持第71-75行）
          2. 时间 - minWidth:150，formatter 用 formatLocalTime（修改原 minWidth:140 为150）
          3. 用户(git_user_name) - minWidth:90（保持）
          4. 说明(comment) - minWidth:200（修改原 minWidth:180 为200）
          5. 代码行数 - minWidth:90（保持）
          6. 实际耗时 - minWidth:100（保持）
          7. 传统开发时长预估 - minWidth:140（保持）
          8. 提效比 - minWidth:90（保持）
          9. 费用 - minWidth:80（新增列，数据来自后端 task 1.4 聚合的 cost 字段，用 `row.cost != null && row.cost > 0 ? row.cost.toFixed(4) : '-'` 格式化）
          10. Tokens消耗 - minWidth:110（新增列，显示 upstream_tokens+downstream_tokens 合计，为0时显示'-'）
        - Tasks 表格（替换第102-131行的 el-table-column，10列，列宽与 Commits 表完全一致）：
          1. Task ID - minWidth:100（保持第103-107行）
          2. 时间 - minWidth:150（修改原 minWidth:140 为150）
          3. 用户(user_name) - minWidth:90（保持）
          4. 说明(title) - minWidth:200（新增列，显示 task.title，showOverflowTooltip）
          5. 代码行数 - minWidth:90（保持）
          6. 实际耗时 - minWidth:100（保持）
          7. 传统开发时长预估 - minWidth:140（保持）
          8. 提效比 - minWidth:90（保持）
          9. 费用 - minWidth:80（保持，修改原 minWidth 为80）
          10. Tokens消耗 - minWidth:110（新增列，显示 upstream_tokens+downstream_tokens 合计，为0时显示'-'）
        - 去掉 Tasks 表格原来的 `caller`（"模式"）列（第108行）
        - 两表格的 minWidth 完全一致，确保上下对齐
        - 列顺序调整：原 Commits 表的"说明"和"时间"位置交换（"时间"移到第2列，"说明"移到第4列）；原 Tasks 表的"时间"也移到第2列
