## 实施

- [x] 2.1 后端新增 Task 写入 API + v2 路由组
     【目标对象】`backend/task_handler_v2.go`（新建）+ `backend/main.go`
     【修改目的】提供 kbcli 写入 task 数据到 PG 的 HTTP 接口，并建立 v2 路由组供后续查询 API 复用
     【修改方式】新建 `task_handler_v2.go` 文件，定义 handler 函数；在 `main.go` 的 `main()` 函数中，现有 `api := r.Group("/api")` 路由块之后，新增 v2 路由组并注册写入+查询路由
     【相关依赖】`backend/db.go` 的 `UpsertCostrictTask()`、`BatchInsertCostrictTaskConversations()`、`CostrictTask`/`CostrictTaskConversation` 结构体；`backend/utils.go` 的 `ptrString()` 等指针辅助函数
     【修改内容】
        - 新建 `backend/task_handler_v2.go`，定义以下 handler：
        - `upsertTaskV2(c *gin.Context)`：POST `/api/v2/tasks`
          - 用 `c.ShouldBindJSON` 将请求体绑定到 `CostrictTask` 结构体
          - 绑定失败返回 HTTP 400 + `gin.H{"error": err.Error()}`（对齐仓库 handler 错误格式）
          - 调用 `UpsertCostrictTask(db, &task)` 写入 PG
          - DB 写入失败返回 HTTP 500 + `gin.H{"error": err.Error()}`
          - 成功返回 HTTP 200 + `gin.H{"status": "ok"}`
        - `batchUpsertConversationsV2(c *gin.Context)`：POST `/api/v2/tasks/conversations/batch`
          - 用 `c.ShouldBindJSON` 将请求体绑定到 `[]CostrictTaskConversation` 切片
          - 绑定失败返回 HTTP 400；空数组时直接返回 HTTP 200 + `gin.H{"status": "ok", "count": 0}`
          - 调用 `BatchInsertCostrictTaskConversations(db, convs)` 写入 PG
          - DB 写入失败返回 HTTP 500 + `gin.H{"error": err.Error()}`
          - 成功返回 HTTP 200 + `gin.H{"status": "ok", "count": len(convs)}`
        - 在 `main.go` 的 `main()` 函数中，现有 `api` 路由块（约第121-153行）之后，新增：
          - `v2 := api.Group("/v2")` 创建 v2 路由组
          - 注册写入路由：`v2.POST("/tasks", upsertTaskV2)`、`v2.POST("/tasks/conversations/batch", batchUpsertConversationsV2)`
          - 同时注册查询路由（任务 2.2 的两个 handler）：`v2.GET("/tasks", listTasksV2)`、`v2.GET("/tasks/:taskId", getTaskDetailV2)`

- [x] 2.2 后端新增 Task 查询 API
     【目标对象】`backend/task_handler_v2.go` + `backend/db.go`
     【修改目的】提供基于 PG 的 Task 列表（支持分页）和详情查询，供前端 UI 调用
     【修改方式】在 `task_handler_v2.go` 中新增查询 handler 函数；在 `db.go` 中修改 `ListCostrictTasks()` 函数增加分页支持和 count 查询
     【相关依赖】`backend/db.go` 的 `ListCostrictTasks()`、`GetCostrictTask()`、`ListCostrictTaskConversations()`；`backend/es_handler.go` 的 `getDefaultInt()` 分页辅助函数；`backend/constants.go` 的 `DefaultPageSize`
     【修改内容】
        - 修改 `backend/db.go` 的 `ListCostrictTasks()` 函数：
          - 增加 `page int, pageSize int` 参数（追加到现有参数之后）
          - 在 SQL 末尾追加 `LIMIT $N OFFSET $M` 分页子句
          - 新增配套 `CountCostrictTasks()` 函数：签名与 `ListCostrictTasks` 的过滤参数一致，返回 `(int, error)`，执行 `SELECT count(*) FROM costrict_tasks WHERE ...` 查询
        - 在 `task_handler_v2.go` 中新增：
        - `listTasksV2(c *gin.Context)`：GET `/api/v2/tasks`
          - 支持 query 参数：userId/repoId/projectId（可选过滤）+ startDate/endDate（YYYYMMDD 格式，必填）+ page/pageSize（可选，默认 1/50）
          - startDate/endDate 转换：用 `parseDateParam()` 解析为 `time.Time`，startDate 取当天 00:00:00，endDate 取当天 23:59:59，格式化为 RFC3339 传入 `ListCostrictTasks` 的 startTime/endTime 参数
          - 参数校验：startDate/endDate 缺失返回 HTTP 400 + `gin.H{"error": "startDate 和 endDate 为必填参数"}`（对齐现有 getTasks handler 风格）
          - 分页：用 `getDefaultInt(c, "page", 1)` 和 `getDefaultInt(c, "pageSize", DefaultPageSize)` 获取
          - 调用 `CountCostrictTasks` 获取 total，调用 `ListCostrictTasks` 获取分页数据
          - 返回 HTTP 200 + `gin.H{"total": N, "page": P, "pageSize": S, "data": [...]}`
        - `getTaskDetailV2(c *gin.Context)`：GET `/api/v2/tasks/:taskId`
          - 从 URL 参数获取 taskId：`c.Param("taskId")`
          - 调用 `GetCostrictTask(db, taskId)` 查询 task 详情
          - task 不存在返回 HTTP 404 + `gin.H{"error": "task not found"}`
          - 调用 `ListCostrictTaskConversations(db, taskId)` 查询关联 conversations
          - 返回 HTTP 200 + `gin.H{"task": task, "conversations": convs}`
          - DB 查询出错返回 HTTP 500 + `gin.H{"error": err.Error()}`

- [x] 2.3 kbcli 新增 PG 写入模块
     【目标对象】`kbcli/pg_writer.go`（新建）
     【修改目的】实现 TaskDoc/RawDoc → CostrictTask/CostrictTaskConversation 的数据映射，并通过 backend HTTP API 批量写入 PG
     【修改方式】新建文件，复用 `kbcli/db_writer.go` 的 `BackendClient` HTTP 封装模式（POST + JSON body + 错误处理）
     【相关依赖】`kbcli/raw_parser.go` 的 `RawDoc` 结构体；`kbcli/task_builder.go` 的 `TaskDoc` 结构体；`kbcli/db_writer.go` 的 `BackendClient` 及其 HTTP 调用模式；`backend/db.go` 的 `CostrictTask`/`CostrictTaskConversation` 字段定义（JSON tag 需对齐）
     【修改内容】
        - 定义 `PGTaskData` 结构体，JSON tag 必须与后端 `CostrictTask` 的 `ShouldBindJSON` 反序列化字段名一一对齐（参照 `backend/db.go` 第154-180行 `CostrictTask` 的字段），包含：task_id, user_id, user_name, client_id, ide, version, os, os_version, caller, repo_addr, repo_branch, repo_id, project_path, project_id, start_time, end_time, upstream_tokens, downstream_tokens, cost, diff_lines, ai_estimated_ancient_days, ai_estimated_ancient_reason
        - 定义 `PGConversationData` 结构体，JSON tag 对齐 `CostrictTaskConversation`（参照 `backend/db.go` 第183-206行），包含：task_id, request_id, sender, prompt_mode, mode, model, start_time, end_time, process_time, process_ttft, upstream_tokens, downstream_tokens, cost, request_content, response_content, user_input, diff, diff_lines, error_code, error_reason
        - 实现 `MapTaskDocToPG(taskDoc TaskDoc, rawDocs []RawDoc) *PGTaskData`：
          - 基本字段从 TaskDoc 直接映射：task_id, user_id, user_name, client_id, caller, project_path, project_id
          - ide = TaskDoc.ClientIDE, version = TaskDoc.ClientVersion, os = TaskDoc.ClientOS
          - os_version：TaskDoc/RawDoc 无此字段，设为空字符串
          - start_time = TaskDoc.APIRequestTime, end_time = TaskDoc.APIEndTime
          - upstream_tokens = TaskDoc.APIInTokens, downstream_tokens = TaskDoc.APIOutTokens
          - cost = TaskDoc.APICost
          - diff_lines = TaskDoc.AssistantOutCodeLines
          - ai_estimated_ancient_days = TaskDoc.AIEstimatedDays, ai_estimated_ancient_reason = TaskDoc.AIEstimatedReason
          - repo_addr 和 repo_branch：从 TaskDoc.RepoID 按 "#" 分割，若含 "#" 则 repo_addr=前半部分, repo_branch=后半部分；若不含 "#" 则 repo_addr=RepoID, repo_branch=""
          - repo_id = TaskDoc.RepoID（原始值，不分割）
        - 实现 `MapRawDocsToConversations(taskID string, rawDocs []RawDoc) []PGConversationData`：
          - 对每个 RawDoc 映射：
            - task_id = 传入的 taskID, request_id = RawDoc.RequestID
            - sender = RawDoc.Sender, prompt_mode = RawDoc.PromptMode, mode = RawDoc.Mode, model = RawDoc.Model
            - start_time = RawDoc.APIRequestTime, end_time = RawDoc.APIEndTime
            - process_time = RawDoc.APIProcessTime, process_ttft = RawDoc.APITtft
            - upstream_tokens = RawDoc.APIInTokens, downstream_tokens = RawDoc.APIOutTokens
            - cost = RawDoc.APICost
            - user_input：参考 `task_content.go` 的 `ExtractTaskContent` 函数中内联的用户输入提取逻辑（第84-97行）：读取 RawDoc.SourcePath 对应的原始 JSON，解析 params.llm_params.messages 最后一条的 content，提取 `<user_message>...</user_message>` 标签内的文本
            - diff / diff_lines：参考 `task_content.go` 的 `ExtractTaskContent` 函数中的 code_outputs 提取逻辑（第99-120行）：从原始 JSON 的 response_content.tool_calls 中提取 write_to_file/apply_diff 的内容，拼接为 diff 文本；diff_lines 用 `raw_parser.go` 的 `countDiffReplaceLines()` 计算
            - request_content / response_content：暂不填充（设为 nil），避免存储过大的原始请求/响应内容
            - error_code / error_reason：RawDoc 无对应字段，设为 nil
        - 实现 `(c *BackendClient) SaveTaskToPG(task *PGTaskData) error`：
          - POST `c.BaseURL + "/api/v2/tasks"`，Content-Type: application/json
          - 复用 `db_writer.go` 的 HTTP 调用模式：json.Marshal → HTTPClient.Post → 检查 StatusCode → 非 200 读取 body 返回错误
        - 实现 `(c *BackendClient) SaveConversationsToPG(convs []PGConversationData) error`：
          - POST `c.BaseURL + "/api/v2/tasks/conversations/batch"`
          - 空 convs 切片直接返回 nil，不发 HTTP 请求
          - 同样复用 `db_writer.go` 的 HTTP 调用错误处理模式

- [x] 2.4 kbcli reindex 命令增加 PG 写入步骤
     【目标对象】`kbcli/cmd_reindex.go`
     【修改目的】在现有 reindex task 步骤中同步写入 PG，使 rawdata 数据同时落库 ES 和 PG
     【修改方式】修改 `reindexTask()` 函数，在现有 for 循环（第209-240行）之后、BulkIndex ES 写入（第265行）之前，新增一个 for 循环遍历 taskDocs 写入 PG；修改 `runReindex()` 函数初始化 BackendClient
     【相关依赖】`kbcli/pg_writer.go` 的 `MapTaskDocToPG()`、`MapRawDocsToConversations()`、`SaveTaskToPG()`、`SaveConversationsToPG()`；`kbcli/db_writer.go` 的 `BackendClient`、`NewBackendClient()`；`kbcli/config.go` 的 `Config.BackendURL`
     【修改内容】
        - 在 `runReindex()` 函数中（第13行开始），初始化 BackendClient：
          - 调用 `NewBackendClient(config.BackendURL)` 创建客户端实例
          - 将 backendClient 传递给 `reindexDate()` 和 `reindexTask()` 函数（需修改函数签名，增加 `backendClient *BackendClient` 参数）
        - 在 `reindexTask()` 函数中，现有 for 循环（处理 taskDocs 提取 content/AI估时，第209-240行）结束之后，BulkIndex（第265行）之前，新增 PG 写入逻辑：
          - 新增 for 循环遍历 taskDocs：
            - 调用 `MapTaskDocToPG(taskDoc, rawDocsByTask[taskDoc.TaskID])` 生成 PGTaskData
            - 调用 `backendClient.SaveTaskToPG(pgTask)` 写入 task
            - 调用 `MapRawDocsToConversations(taskDoc.TaskID, rawDocsByTask[taskDoc.TaskID])` 生成 conversations
            - 调用 `backendClient.SaveConversationsToPG(convs)` 写入 conversations
          - 错误处理：PG 写入失败不中断流程，仅用 `fmt.Printf("警告: ...")` 打印日志（对齐 reindexTask 中现有的警告日志风格），继续处理下一个 task
          - 打印 PG 写入统计：成功数 / 总数

- [x] 2.5 从现有 rawdata 执行回填验证
     【目标对象】kbcli 命令行 + backend 服务 + PG 数据库
     【修改目的】验证管道端到端正确性，将现有 rawdata 数据回填到 PG
     【修改方式】运行 kbcli reindex 命令处理至少 1 天的数据，然后通过 API 和 SQL 验证
     【相关依赖】任务 2.1-2.4 全部完成，backend 服务运行中
     【修改内容】
        - 确保 backend 服务启动（`go run .` 在 backend/ 目录）
        - 执行 `kbcli reindex --date=20260331 --step=task`（或选择有数据的日期）
        - 通过 SQL 验证数据已写入：`SELECT count(*) FROM costrict_tasks;` 和 `SELECT count(*) FROM costrict_task_conversations;`
        - 通过 API 验证查询正常：`GET /api/v2/tasks?startDate=20260301&endDate=20260401`
        - 抽查 1-2 条 task 详情：`GET /api/v2/tasks/:taskId`，验证 conversations 关联正确（conversations 数量与 rawDocs 数量匹配）
        - 验证字段映射正确：对比 PG 中 costrict_tasks 的 start_time/end_time/cost 等字段与 ES 中对应 task 的 api_request_time/api_end_time/api_cost 一致
